package handler

import (
	"encoding/json"

	"github.com/gin-gonic/gin"
	"github.com/gin-gonic/gin/binding"

	"github.com/nanirise/heart-echo/backend/internal/dto"
	"github.com/nanirise/heart-echo/backend/internal/service"
	"github.com/nanirise/heart-echo/backend/pkg/errcode"
	"github.com/nanirise/heart-echo/backend/pkg/response"
)

// AuthHandler 是认证与用户资料共 6 个端点的 HTTP 层：绑定 → 调 service → 写响应。
//
// 六个端点分属两个路由组，判断依据只有一条：**这个端点的用户身份从哪里来**。
//   - /auth/register|login|refresh 三个：注册登录时用户还没有身份，刷新的身份
//     来自请求体里那枚已验签的 token。它们不读 Context，挂在免鉴权的 api 组上。
//   - /user/profile|password 三个：改的是"谁"完全由令牌决定，挂在 protected 组上，
//     从 Context 取 userID。
//
// 六个端点共用一个 handler 而不是拆成两个：它们背后是同一个 AuthService，
// 拆开只会多一个包装同名方法的空壳，装配点也要多一行。
type AuthHandler struct {
	authService *service.AuthService
}

// NewAuthHandler 构造 handler。
func NewAuthHandler(authService *service.AuthService) *AuthHandler {
	return &AuthHandler{authService: authService}
}

// RegisterAuthRoutes 由 router.go 汇总调用，挂**免鉴权**的三个认证端点。
//
// ⚠️ 挂在**免鉴权的 api 组**上，不是 protected：契约 §1 的白名单里就有这三个端点。
// 挂到 protected 上，等于要求"先登录才能登录"，前端会卡在登录页出不去。
func RegisterAuthRoutes(rg *gin.RouterGroup, h *AuthHandler) {
	g := rg.Group("/auth")
	{
		g.POST("/register", h.Register)
		g.POST("/login", h.Login)
		g.POST("/refresh", h.Refresh)
	}
}

// RegisterUserRoutes 由 router.go 汇总调用，挂**需鉴权**的三个用户资料端点。
//
// ⚠️ 与上面相反，必须挂在 protected 组上（契约 §3.4-3.6 三个端点都要求登录）。
// 这里不靠 currentUserID 兜底：挂错组时它取不到值只返回 4010，
// 接口表现看着"正常"，实际上任何人都能裸请求走到 service。
func RegisterUserRoutes(rg *gin.RouterGroup, h *AuthHandler) {
	g := rg.Group("/user")
	{
		g.GET("/profile", h.GetProfile)
		g.PUT("/profile", h.UpdateProfile)
		g.PUT("/password", h.ChangePassword)
	}
}

// Register 处理 POST /auth/register。
func (h *AuthHandler) Register(c *gin.Context) {
	var req dto.RegisterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		// 上抛的必须是 BizError：原始绑定错误不是 BizError，会被 BizErrorHandler 归成 5000。
		// 原始 err 只进日志、不返回前端 —— 它会把校验规则的细节吐给调用方。
		_ = c.Error(errcode.New(errcode.ErrInvalidParams))
		return
	}

	resp, err := h.authService.Register(c.Request.Context(), &req)
	if err != nil {
		_ = c.Error(err)
		return
	}
	response.Success(c, *resp)
}

// Login 处理 POST /auth/login。
func (h *AuthHandler) Login(c *gin.Context) {
	var req dto.LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		_ = c.Error(errcode.New(errcode.ErrInvalidParams))
		return
	}

	resp, err := h.authService.Login(c.Request.Context(), &req)
	if err != nil {
		_ = c.Error(err)
		return
	}
	response.Success(c, *resp)
}

// Refresh 处理 POST /auth/refresh。
//
// 响应 data 与上面两个不同：契约 §3.3 只给一对令牌，不带 user ——
// 刷新拿到的用户信息不保证是最新的，契约刻意不让它成为"读用户信息"的第二个入口。
func (h *AuthHandler) Refresh(c *gin.Context) {
	var req dto.RefreshRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		_ = c.Error(errcode.New(errcode.ErrInvalidParams))
		return
	}

	pair, err := h.authService.Refresh(c.Request.Context(), req.RefreshToken)
	if err != nil {
		_ = c.Error(err)
		return
	}
	response.Success(c, *pair)
}

// GetProfile 处理 GET /user/profile。
//
// 没有路径参数、没有查询参数：读的永远是"令牌里那个人"，
// 契约 §3.4 刻意不给 userId 入参，越权入口就不存在。
func (h *AuthHandler) GetProfile(c *gin.Context) {
	userID, ok := currentUserID(c)
	if !ok {
		_ = c.Error(errcode.New(errcode.ErrUnauthorized))
		return
	}

	resp, err := h.authService.GetProfile(c.Request.Context(), userID)
	if err != nil {
		_ = c.Error(err)
		return
	}
	response.Success(c, *resp)
}

// UpdateProfile 处理 PUT /user/profile。
//
// 这里用 bindStrictJSON 而不是 c.ShouldBindJSON —— 它是全项目唯一一个
// "部分更新"端点，多传的字段名会被静默丢掉（理由见 bindStrictJSON 的注释）。
// 其余校验交给 dto.UpdateProfileRequest 自己的 binding tag 与 ParseAvatarURL：
// username 的空串、avatarUrl 的三态都在 service 里处理，handler 不重复判断。
func (h *AuthHandler) UpdateProfile(c *gin.Context) {
	userID, ok := currentUserID(c)
	if !ok {
		_ = c.Error(errcode.New(errcode.ErrUnauthorized))
		return
	}

	var req dto.UpdateProfileRequest
	if err := bindStrictJSON(c, &req); err != nil {
		_ = c.Error(errcode.New(errcode.ErrInvalidParams))
		return
	}

	resp, err := h.authService.UpdateProfile(c.Request.Context(), userID, &req)
	if err != nil {
		_ = c.Error(err)
		return
	}
	response.Success(c, *resp)
}

// ChangePassword 处理 PUT /user/password。
//
// 响应 data 是 null（契约 §3.6），所以泛型参数要显式写成 any ——
// Success(c, nil) 推不出 T，编译不过。
func (h *AuthHandler) ChangePassword(c *gin.Context) {
	userID, ok := currentUserID(c)
	if !ok {
		_ = c.Error(errcode.New(errcode.ErrUnauthorized))
		return
	}

	var req dto.ChangePasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		_ = c.Error(errcode.New(errcode.ErrInvalidParams))
		return
	}

	if err := h.authService.ChangePassword(c.Request.Context(), userID, &req); err != nil {
		_ = c.Error(err)
		return
	}
	response.Success[any](c, nil)
}

// bindStrictJSON 解析请求体，遇到**不认识的字段名**直接报错；
// 其余行为与 c.ShouldBindJSON 一致（解码 + 跑 binding tag 校验）。
//
// 为什么不直接用 c.ShouldBindJSON：它内部走 encoding/json 的默认解码，
// 多出来的字段会被**静默丢掉**。对 PUT /user/profile 这种"部分更新"接口，
// 那会让两种完全不同的情况表现成同一个结果：
//   - 用户什么都没想改（发了 {}）
//   - 调用方把字段名拼错了（发了 {"user_name":"新名字"}）
//
// 两者都是"什么字段都没传进去"→ 什么都不改 → 200「保存成功」。
// 拼错名字的人拿到的是"成功"，只能自己去猜哪里不对。
//
// 只有这一个端点用严格模式，不推广到其他端点：注册 / 登录 / 改密的字段全是必填，
// 拼错名字会让 required 直接拦下，本来就不会静默通过。
// 将来若出现第二个"部分更新"端点（例如 PUT /personas/:id），把这个函数提上去共用，
// 不要抄一份——两处的严格程度必须一致，否则又是"同一种错两种表现"。
func bindStrictJSON(c *gin.Context, dst any) error {
	dec := json.NewDecoder(c.Request.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(dst); err != nil {
		return err
	}
	// 校验这一步不能省：上面的 Decode 只负责把 JSON 填进结构体，
	// binding tag（min=3 / max=20 / alphanum）要显式再跑一遍。
	// 用的是 gin 生产里那个 validator 实例，不是自己 new 的替身。
	return binding.Validator.ValidateStruct(dst)
}
