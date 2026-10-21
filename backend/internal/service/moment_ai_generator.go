package service

import (
	"context"
	"errors"
	"strings"
	"time"

	"go.uber.org/zap"

	"github.com/nanirise/heart-echo/backend/internal/config"
)

// momentAIGenerateTimeout 是单个人设调用 ai-service 的业务上限。
// 底层共享 Transport 的 ResponseHeaderTimeout（10s）可能更早触发；
// 两者都会落到模板兜底，不会拖住整轮定时任务。
const momentAIGenerateTimeout = 20 * time.Second

var errEmptyMomentContent = errors.New("ai-service returned empty moment content")

// MomentAIClient 是 AIGenerator 需要的那部分 AIClient。
// 收窄成单方法接口，单测只需实现 GenerateMoment，不必伪造整套 AIClient。
type MomentAIClient interface {
	GenerateMoment(ctx context.Context, req MomentRequest) (string, error)
}

// AIGenerator 是阶段二实现：优先让 ai-service 结合人设与近期互动生成；
// 任何失败（网络、超时、非 200、空正文）都回退 fallback，保证动态不断更。
type AIGenerator struct {
	client   MomentAIClient
	fallback ContentGenerator
	timeout  time.Duration
	logger   *zap.Logger
}

// NewMomentGenerator 按 MOMENT_GENERATOR 选实现。
//
// 只有显式的 template 才停用 AI；未配置或空值一律按默认 ai 处理，
// 避免环境变量漏配时朋友圈静默退回模板。
func NewMomentGenerator(
	mode string, aiClient MomentAIClient, logger *zap.Logger,
) ContentGenerator {
	if mode == config.MomentGeneratorTemplate {
		return NewTemplateGenerator()
	}
	return NewAIGenerator(aiClient, NewTemplateGenerator(), logger)
}

// NewAIGenerator 构造阶段二生成器；fallback 通常传 NewTemplateGenerator()。
func NewAIGenerator(
	client MomentAIClient, fallback ContentGenerator, logger *zap.Logger,
) *AIGenerator {
	return &AIGenerator{
		client:   client,
		fallback: fallback,
		timeout:  momentAIGenerateTimeout,
		logger:   logger,
	}
}

// Generate 先调 AI，失败降级模板。
//
// 返回的 error 只可能来自模板本身（例如 ctx 已取消）；AI 侧的错误一律被
// 降级吞掉 —— 定时任务不该因为一次模型抖动就漏掉一条动态。
func (g *AIGenerator) Generate(ctx context.Context, in GenerateInput) (Generated, error) {
	if err := ctx.Err(); err != nil {
		return Generated{}, err
	}

	aiCtx, cancel := context.WithTimeout(ctx, g.timeout)
	defer cancel()

	content, err := g.client.GenerateMoment(aiCtx, MomentRequest{
		Name:            in.Persona.Name,
		PersonalityDesc: in.Persona.PersonalityDesc,
		SpeakingStyle:   in.Persona.SpeakingStyle,
		Context:         in.Context,
	})
	if err == nil {
		content = strings.TrimSpace(content)
		if content != "" {
			return Generated{Content: content}, nil
		}
		// 200 + 空正文同样是失败：落库会让朋友圈出现一条空白动态。
		err = errEmptyMomentContent
	}

	g.logger.Warn("moment ai generation failed, falling back to template",
		zap.Uint64("personaID", in.Persona.ID),
		zap.Error(err))
	return g.fallback.Generate(ctx, in)
}
