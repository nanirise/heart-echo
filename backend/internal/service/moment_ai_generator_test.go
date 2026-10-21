package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"go.uber.org/zap"

	"github.com/nanirise/heart-echo/backend/internal/config"
	"github.com/nanirise/heart-echo/backend/internal/model"
)

type fakeMomentAIClient struct {
	content  string
	err      error
	called   bool
	received MomentRequest
}

func (f *fakeMomentAIClient) GenerateMoment(
	_ context.Context, req MomentRequest,
) (string, error) {
	f.called = true
	f.received = req
	return f.content, f.err
}

type fakeContentGenerator struct {
	result   Generated
	err      error
	called   bool
	received GenerateInput
}

func (f *fakeContentGenerator) Generate(
	_ context.Context, in GenerateInput,
) (Generated, error) {
	f.called = true
	f.received = in
	return f.result, f.err
}

func testGenerateInput() GenerateInput {
	return GenerateInput{
		Persona: model.Persona{
			ID:              7,
			Name:            "小回",
			PersonalityDesc: "温柔",
			SpeakingStyle:   "简短",
		},
		Context: "用户：今天好累",
	}
}

func TestAIGeneratorUsesAIContent(t *testing.T) {
	ai := &fakeMomentAIClient{content: " 今天风很轻。 "}
	fallback := &fakeContentGenerator{result: Generated{Content: "模板内容"}}
	generator := NewAIGenerator(ai, fallback, zap.NewNop())

	got, err := generator.Generate(context.Background(), testGenerateInput())
	if err != nil {
		t.Fatalf("Generate 不应失败: %v", err)
	}
	if got.Content != "今天风很轻。" {
		t.Errorf("应返回 AI 正文（已去首尾空白），实际 %q", got.Content)
	}
	if fallback.called {
		t.Error("AI 成功时不应调用模板兜底")
	}
	if ai.received.Name != "小回" || ai.received.Context != "用户：今天好累" {
		t.Errorf("应把人设与上下文传给 AI，实际 %+v", ai.received)
	}
}

func TestAIGeneratorFallsBackOnError(t *testing.T) {
	ai := &fakeMomentAIClient{err: errors.New("boom")}
	fallback := &fakeContentGenerator{result: Generated{Content: "模板内容"}}
	generator := NewAIGenerator(ai, fallback, zap.NewNop())

	got, err := generator.Generate(context.Background(), testGenerateInput())
	if err != nil {
		t.Fatalf("AI 失败时应降级而不是返回错误: %v", err)
	}
	if got.Content != "模板内容" {
		t.Errorf("应返回模板正文，实际 %q", got.Content)
	}
	if !fallback.called {
		t.Error("AI 失败时应调用模板兜底")
	}
}

func TestAIGeneratorFallsBackOnEmptyContent(t *testing.T) {
	ai := &fakeMomentAIClient{content: "   "}
	fallback := &fakeContentGenerator{result: Generated{Content: "模板内容"}}
	generator := NewAIGenerator(ai, fallback, zap.NewNop())

	got, err := generator.Generate(context.Background(), testGenerateInput())
	if err != nil {
		t.Fatalf("空正文时应降级而不是返回错误: %v", err)
	}
	if got.Content != "模板内容" {
		t.Errorf("空正文应触发模板兜底，实际 %q", got.Content)
	}
	if !fallback.called {
		t.Error("空正文时应调用模板兜底")
	}
}

type blockingMomentAIClient struct{}

func (blockingMomentAIClient) GenerateMoment(
	ctx context.Context, _ MomentRequest,
) (string, error) {
	<-ctx.Done()
	return "", ctx.Err()
}

func TestNewMomentGeneratorDefaultsToAI(t *testing.T) {
	// 漏配 MOMENT_GENERATOR 时按默认 ai 处理，不能静默退回模板；
	// 显式 ai 与空值必须走同一条分支。
	for _, mode := range []string{"", config.MomentGeneratorAI} {
		t.Run("mode="+mode, func(t *testing.T) {
			ai := &fakeMomentAIClient{content: "AI 正文"}
			generator := NewMomentGenerator(mode, ai, zap.NewNop())

			if _, ok := generator.(*AIGenerator); !ok {
				t.Fatalf("mode=%q 应返回 *AIGenerator，实际 %T", mode, generator)
			}
			got, err := generator.Generate(context.Background(), testGenerateInput())
			if err != nil {
				t.Fatalf("Generate 不应失败: %v", err)
			}
			if got.Content != "AI 正文" {
				t.Errorf("应返回 AI 正文，实际 %q", got.Content)
			}
			if !ai.called {
				t.Error("默认档位应调用 ai-service")
			}
		})
	}
}

func TestNewMomentGeneratorTemplateModeSkipsAI(t *testing.T) {
	ai := &fakeMomentAIClient{content: "不应被使用"}
	generator := NewMomentGenerator(config.MomentGeneratorTemplate, ai, zap.NewNop())

	if _, ok := generator.(*TemplateGenerator); !ok {
		t.Fatalf("template 档应返回 *TemplateGenerator，实际 %T", generator)
	}
	if _, err := generator.Generate(context.Background(), testGenerateInput()); err != nil {
		t.Fatalf("模板档不应依赖外部服务: %v", err)
	}
	if ai.called {
		t.Error("template 档不应调用 ai-service")
	}
}

func TestAIGeneratorFallsBackOnTimeout(t *testing.T) {
	fallback := &fakeContentGenerator{result: Generated{Content: "模板内容"}}
	generator := NewAIGenerator(blockingMomentAIClient{}, fallback, zap.NewNop())
	generator.timeout = time.Millisecond

	got, err := generator.Generate(context.Background(), testGenerateInput())
	if err != nil {
		t.Fatalf("超时时应降级而不是返回错误: %v", err)
	}
	if got.Content != "模板内容" {
		t.Errorf("超时应触发模板兜底，实际 %q", got.Content)
	}
	if !fallback.called {
		t.Error("超时时应调用模板兜底")
	}
}
