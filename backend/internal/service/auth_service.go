package service

import (
	"context"
	"errors"
	"sync"

	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"

	"github.com/nanirise/heart-echo/backend/internal/config"
	"github.com/nanirise/heart-echo/backend/internal/dto"
	"github.com/nanirise/heart-echo/backend/internal/model"
	"github.com/nanirise/heart-echo/backend/internal/repository"
	"github.com/nanirise/heart-echo/backend/pkg/errcode"
	"github.com/nanirise/heart-echo/backend/pkg/jwt"
)

// bcryptCost 是密码哈希的 cost。
// 写死 10 而不是用 bcrypt.DefaultCost：库哪天改默认值，我们的密码强度不该跟着变。
// 也不往上调：cost 每 +1 耗时翻倍，登录是高频接口，10 是技术文档 §4.8 定的值。
const bcryptCost = 10

// dummyHash 是"用户不存在"那条路径专用的假哈希，第一次用到时才生成。
//
// 不写死在源码里有两个原因：硬编码的 bcrypt 串会被密钥扫描工具当成泄露的凭据报出来；
// 而且它必须与 bcryptCost 保持一致，写死就等于同一个事实存了两份。
var dummyHash = sync.OnceValue(func() []byte {
	h, err := bcrypt.GenerateFromPassword([]byte("heart-echo-dummy-password"), bcryptCost)
	if err != nil {
		// GenerateFromPassword 只在 cost 越界时报错，而 bcryptCost 是编译期常量，走不到这里。
		// panic 而不是忽略：忽略会让假哈希变成 nil，比对瞬间返回、计时防护静默失效——
		// 那比直接崩掉更难查。
		panic("service: 生成本地假哈希失败: " + err.Error())
	}
	return h
})

// AuthService 是认证链路的业务逻辑：注册 / 登录 / 刷新令牌。
// 只依赖 context.Context，不感知 *gin.Context。
type AuthService struct {
	userRepo *repository.UserRepo
	jwtCfg   config.JWTConfig
}

// NewAuthService 构造服务，并组装它依赖的仓储。
func NewAuthService(db *gorm.DB, jwtCfg config.JWTConfig) *AuthService {
	return &AuthService{
		userRepo: repository.NewUserRepo(db),
		jwtCfg:   jwtCfg,
	}
}

// Register 创建账号并直接签发令牌（契约 §3.1 的"注册即登录"）。
func (s *AuthService) Register(ctx context.Context, req *dto.RegisterRequest) (*dto.AuthResponse, error) {
	if err := s.ensureUnique(ctx, req.Username, req.Email); err != nil {
		return nil, err
	}

	hash, err := hashPassword(req.Password)
	if err != nil {
		return nil, errcode.Wrap(errcode.ErrInternal, err)
	}

	u := &model.User{
		Username:     req.Username,
		Email:        req.Email,
		PasswordHash: hash,
	}

	if err := s.userRepo.Create(ctx, u); err != nil {
		// 并发注册时两个请求会一起通过上面的先查，最后由唯一索引拦下——
		// 这里必须翻译成与先查相同的错误码，否则并发场景返回 5003、单发返回 4004，
		// 同一种错误两种表现，前端分支会走错。
		switch {
		case errors.Is(err, repository.ErrDuplicateUsername):
			return nil, errcode.New(errcode.ErrUsernameTaken)
		case errors.Is(err, repository.ErrDuplicateEmail):
			return nil, errcode.New(errcode.ErrEmailExists)
		}
		return nil, errcode.Wrap(errcode.ErrDBFailed, err)
	}

	return s.issueTokens(u)
}

// Login 校验凭据并签发令牌。
//
// 用户不存在与密码错误一律返回 4013，两个原因不分开——
// 分开就等于提供了一个用户名探测器，攻击者能问出"这个用户名注册过没有"。
func (s *AuthService) Login(ctx context.Context, req *dto.LoginRequest) (*dto.AuthResponse, error) {
	u, err := s.userRepo.FindByUsername(ctx, req.Username)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			// 计时侧信道：用户不存在时直接返回，响应会明显快于"用户存在但密码错"
			//（bcrypt cost 10 约几十毫秒），靠响应耗时照样能枚举用户名。
			// 对假哈希空跑一次比对，把两条路径的耗时拉平——
			// 错误码统一了但耗时没统一，等于把防枚举做了个样子。
			_ = bcrypt.CompareHashAndPassword(dummyHash(), []byte(req.Password))
			return nil, errcode.New(errcode.ErrPasswordWrong)
		}
		return nil, errcode.Wrap(errcode.ErrDBFailed, err)
	}

	if err := bcrypt.CompareHashAndPassword([]byte(u.PasswordHash), []byte(req.Password)); err != nil {
		return nil, errcode.New(errcode.ErrPasswordWrong)
	}

	return s.issueTokens(u)
}

// Refresh 用 refresh token 换一对新令牌（契约 §3.3）。
//
// 所有失败路径都返回 4014：这个端点在契约里只有这一个错误码，
// 用户能做的事也只有一件——重新登录。
func (s *AuthService) Refresh(ctx context.Context, refreshToken string) (*jwt.TokenPair, error) {
	claims, err := jwt.ParseToken(refreshToken, s.jwtCfg.Secret)
	if err != nil {
		return nil, errcode.New(errcode.ErrRefreshInvalid)
	}
	// 拿 access token 来换令牌必须拒绝：放过它等于让 2 小时的 access 变成 7 天有效。
	if claims.TokenType != jwt.TokenTypeRefresh {
		return nil, errcode.New(errcode.ErrRefreshInvalid)
	}

	// 回查一次库。不是重新鉴权，是为了让新令牌里的 username 取当前值：
	// 用户改过用户名之后，令牌里带的是旧名字，而每次刷新都是从上一个令牌抄 claims，
	// 抄的是同一份旧数据，不查库这个错会一直传下去。
	// 顺带挡住"库里已经没有这个用户，旧令牌还在签新令牌"。
	u, err := s.userRepo.FindByID(ctx, claims.UserID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errcode.New(errcode.ErrRefreshInvalid)
		}
		return nil, errcode.Wrap(errcode.ErrDBFailed, err)
	}

	pair, err := s.signPair(u)
	if err != nil {
		return nil, errcode.Wrap(errcode.ErrInternal, err)
	}
	return pair, nil
}

// ensureUnique 是唯一性的"先查一次"，作用只是给出友好错误。
// 它拦不住并发——两个请求可以同时查到"不存在"然后一起插入，
// 真正的防线是 users 表上那两个唯一索引（见 repository.UserRepo.Create）。
func (s *AuthService) ensureUnique(ctx context.Context, username, email string) error {
	// 邮箱先查：两个都已被占用时固定以邮箱为准，顺序写死是为了结果可预期。
	if _, err := s.userRepo.FindByEmail(ctx, email); err == nil {
		return errcode.New(errcode.ErrEmailExists)
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		return errcode.Wrap(errcode.ErrDBFailed, err)
	}

	if _, err := s.userRepo.FindByUsername(ctx, username); err == nil {
		return errcode.New(errcode.ErrUsernameTaken)
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		return errcode.Wrap(errcode.ErrDBFailed, err)
	}

	return nil
}

// issueTokens 签发令牌并组装注册 / 登录的响应。
func (s *AuthService) issueTokens(u *model.User) (*dto.AuthResponse, error) {
	pair, err := s.signPair(u)
	if err != nil {
		return nil, errcode.Wrap(errcode.ErrInternal, err)
	}
	return &dto.AuthResponse{TokenPair: *pair, User: dto.NewUserResponse(u)}, nil
}

// signPair 签发一对令牌，用户名取调用方传进来的当前值。
func (s *AuthService) signPair(u *model.User) (*jwt.TokenPair, error) {
	return jwt.GenerateTokenPair(
		u.ID,
		u.Username,
		s.jwtCfg.Secret,
		s.jwtCfg.AccessTokenExpire,
		s.jwtCfg.RefreshTokenExpire,
	)
}

// hashPassword 生成 bcrypt 哈希。
// 接收明文密码是必要的——哈希必须现算，盐是随机的，同一个密码两次哈希结果不同。
func hashPassword(password string) (string, error) {
	h, err := bcrypt.GenerateFromPassword([]byte(password), bcryptCost)
	if err != nil {
		return "", err
	}
	return string(h), nil
}
