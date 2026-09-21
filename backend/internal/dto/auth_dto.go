package dto

import (
	"github.com/nanirise/heart-echo/backend/internal/model"
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
