package dto

import (
	"encoding/json"
	"time"

	"github.com/nanirise/heart-echo/backend/internal/model"
	"github.com/nanirise/heart-echo/backend/pkg/errcode"
	"github.com/nanirise/heart-echo/backend/pkg/jwt"
)

// RegisterRequest 是 POST /auth/register 的请求体，三个字段与契约 §3.1 逐条对应。
// 刻意不声明 id / avatarUrl / createdAt：请求体里没有的字段，就没有被前端塞进来的可能。
type RegisterRequest struct {
	Username string `json:"username" binding:"required,min=3,max=20,alphanum"`
	// max=100 对齐 users.email 的 varchar(100)：不设上限的话，超长邮箱会一路走到
	// 数据库报 22001（值太长），错误码变成 5003「数据库操作失败」而不是 4001。
	Email string `json:"email"    binding:"required,email,max=100"`
	// printascii 限定可打印 ASCII（\x20-\x7E，含空格），是给 bcrypt 用的约束：
	// bcrypt 对超过 72 字节的密码**直接返回 ErrPasswordTooLong**（x/crypto v0.55.0
	// 的 GenerateFromPassword，不是截断），hashPassword 会把它包成 5000。
	// max=32 就是拦这一步的闸——把 5000 换成 4001，不是防截断（那里不截断）。
	// min/max 数的是字符数，ASCII 下字符数 = 字节数，32 个字符最多 32 字节，离 72 有大余量。
	Password string `json:"password" binding:"required,min=8,max=32,printascii"`
}

// LoginRequest 是 POST /auth/login 的请求体。
// 只校验非空，不套注册那套长度与字符集规则：长短不对的密码和写错的密码没有区别，
// 多写一层只会让输错的人拿到 4001「参数校验失败」这种看不懂的提示。
type LoginRequest struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
}

// RefreshRequest 是 POST /auth/refresh 的请求体。
type RefreshRequest struct {
	RefreshToken string `json:"refreshToken" binding:"required"`
}

// UserResponse 是注册 / 登录响应里嵌的那个 user 对象（契约 §3.1）。
// 比契约 §3.4 的 GET /user/profile 少一个 createdAt —— §3.1 就只有这四个，
// 多给一个字段属于契约漂移，要加得先广播。
//
// 单独定义而不是直接序列化 model.User：这个类型里根本没有 PasswordHash 字段，
// 就算有人把 model.User 上的 json:"-" 改掉，密码哈希也漏不出来。
type UserResponse struct {
	ID        uint64  `json:"id"`
	Username  string  `json:"username"`
	Email     string  `json:"email"`
	AvatarURL *string `json:"avatarUrl"`
}

// NewUserResponse 把用户实体转成响应结构。不传实体直接序列化，是上一条注释说的那道防线。
func NewUserResponse(u *model.User) UserResponse {
	return UserResponse{
		ID:        u.ID,
		Username:  u.Username,
		Email:     u.Email,
		AvatarURL: u.AvatarURL,
	}
}

// AuthResponse 是 register / login 的响应 data（契约 §3.1 / §3.2）。
//
// 内嵌 jwt.TokenPair 而不是重抄一遍两个 token 字段：字段名与 json tag 只在
// pkg/jwt 定义一次，不会出现"签发的"和"契约里写的"两处对不上。
// 内嵌（匿名）字段会被提升成 AuthResponse 自己的字段，序列化后是平铺的
// {accessToken, refreshToken, user}，不是嵌套对象。
type AuthResponse struct {
	jwt.TokenPair
	User UserResponse `json:"user"`
}

// UpdateProfileRequest 是 PUT /user/profile 的请求体（契约 §3.5）。
// 两个字段都可选，但"可选"的含义不同，所以类型不同。
type UpdateProfileRequest struct {
	// nil = 没传（不改）；非 nil = 传了值（改成它）
	Username *string `json:"username" binding:"omitempty,min=3,max=20,alphanum"`

	// 用 json.RawMessage 而非 *string：要区分三态（契约 §3.5 允许传 null 清空头像）。
	// *string 下"没传"和"传了 null"都是 nil，用户点"清空头像"会被当成"没传"，
	// 而这个 bug 在只测 {"username":"x"} 时看不出来。
	AvatarURL json.RawMessage `json:"avatarUrl"`
}

// ParseAvatarURL 把三态解析成「要不要改、改成什么」。
// present=false 表示请求里没有这个字段，调用方应保持原值不动。
func (r *UpdateProfileRequest) ParseAvatarURL() (value *string, present bool, err error) {
	if len(r.AvatarURL) == 0 {
		return nil, false, nil // 没传
	}
	if string(r.AvatarURL) == "null" {
		return nil, true, nil // 明确清空：要改，改成 nil
	}
	var s string
	if err := json.Unmarshal(r.AvatarURL, &s); err != nil {
		// 传了数字 / 对象 / 布尔：契约只允许字符串或 null
		return nil, false, errcode.New(errcode.ErrInvalidParams)
	}
	// 契约 §3.5：最长 255，对齐 users.avatar_url 的 varchar(255)。
	// 不拦的话超长会走到数据库报 22001，用户拿到 5003 而不是 4001。
	if len(s) > 255 {
		return nil, false, errcode.New(errcode.ErrInvalidParams)
	}
	return &s, true, nil
}

// ChangePasswordRequest 是 PUT /user/password 的请求体（契约 §3.6）。
// newPassword 的规则与注册**逐条相同**——两处不一致的话，会出现
// "注册时能用的密码，改密码时说它不合法"。
type ChangePasswordRequest struct {
	// 只 required，不套长度与字符集：理由同 LoginRequest。
	// 老密码格式不对和密码错，对用户来说是同一件事，多校验一层只会
	// 让他拿到 4001 而不是 4015，反而看不出是密码错了。
	OldPassword string `json:"oldPassword" binding:"required"`
	NewPassword string `json:"newPassword" binding:"required,min=8,max=32,printascii"`
}

// ProfileResponse 是 GET /user/profile 的响应 data（契约 §3.4）。
//
// 内嵌 UserResponse 而不是重抄那 4 个字段：契约 §3.1 与 §3.4 共有的字段
// 只定义一次，不会出现"注册响应里叫 username、资料响应里叫 userName"。
// 内嵌是匿名的，序列化后是平铺的 {id, username, email, avatarUrl, createdAt}，
// 不是嵌套的 {"UserResponse": {...}}。
type ProfileResponse struct {
	UserResponse
	CreatedAt time.Time `json:"createdAt"`
}

// NewProfileResponse 把用户实体转成资料响应。复用 NewUserResponse，
// 不重写那 4 个字段的赋值——重写就等于把防线又抄了一遍。
func NewProfileResponse(u *model.User) ProfileResponse {
	return ProfileResponse{
		UserResponse: NewUserResponse(u),
		CreatedAt:    u.CreatedAt,
	}
}
