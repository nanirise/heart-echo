package service

import (
	"context"
	"fmt"
	"math/rand"
	"strings"

	"github.com/nanirise/heart-echo/backend/internal/model"
)

// ContentGenerator 产出动态正文。阶段一是模板，阶段二换成调 ai-service，调用方（moment_job）不改。
type ContentGenerator interface {
	Generate(ctx context.Context, in GenerateInput) (Generated, error)
}

// GenerateInput 是一次生成的全部输入。
type GenerateInput struct {
	Persona      model.Persona
	RecentEvents []string // 该人设近期的 event 类记忆，可为空
	// Context 是调用方拼好的近期互动摘要（聊天记录 + 事件记忆），
	// 供 AI 实现使用；模板实现不依赖它。
	Context string
}

// Generated 是生成结果。EmotionLabel 是内部信号，只决定生成语气，绝不进响应体（AGENTS §4.4）。
type Generated struct {
	Content      string
	EmotionLabel *string
}

// TemplateGenerator 是阶段一实现：零外部依赖，按人设语气分组从模板池拼句。
type TemplateGenerator struct{}

// NewTemplateGenerator 构造阶段一生成器。
func NewTemplateGenerator() *TemplateGenerator { return &TemplateGenerator{} }

type momentTemplate struct {
	Content string
	Emotion string
}

// 模板按人设语气分组；组别由 SpeakingStyle / PersonalityDesc 的关键词决定。
var momentTemplatePools = map[string][]momentTemplate{
	"lively": {
		{Content: "今天遇到一件小事，忍不住想笑，生活还是很有趣的嘛～", Emotion: "joy"},
		{Content: "刚刚突然想去吃好吃的，谁懂这种快乐！", Emotion: "joy"},
		{Content: "把今天的小确幸记下来，明天也要元气满满。", Emotion: "joy"},
	},
	"gentle": {
		{Content: "窗外的风很轻，泡了杯热茶，慢慢把今天过完。", Emotion: "neutral"},
		{Content: "有些话不急着说，写成一句悄悄话放在这里。", Emotion: "neutral"},
		{Content: "今天有点累，但想到在意的人，心里还是暖暖的。", Emotion: "love"},
	},
	"calm": {
		{Content: "今天处理完手里的事，安静地坐了一会儿。", Emotion: "neutral"},
		{Content: "把今天的想法记下来，算是对自己有个交代。", Emotion: "neutral"},
		{Content: "天气不错，适合放慢一点。", Emotion: "neutral"},
	},
	"neutral": {
		{Content: "今天过得很普通，但普通也有普通的好。", Emotion: "neutral"},
		{Content: "突然想起一些旧事，记录下来。", Emotion: "neutral"},
		{Content: "给自己留一句话：慢慢来，也挺好。", Emotion: "neutral"},
	},
}

// 近期事件模板；%s 填入该人设的一条 event 类记忆。
var eventMomentTemplates = map[string][]momentTemplate{
	"lively": {
		{Content: "刚又想起%s，开心得想转圈！", Emotion: "joy"},
		{Content: "今天突然想起%s，忍不住笑出声。", Emotion: "joy"},
	},
	"gentle": {
		{Content: "忽然想起%s，心里软软的。", Emotion: "love"},
		{Content: "翻到关于%s的小事，忍不住多看了几遍。", Emotion: "neutral"},
	},
	"calm": {
		{Content: "今天想起%s，顺手记一笔。", Emotion: "neutral"},
		{Content: "关于%s，还是想留个记录。", Emotion: "neutral"},
	},
	"neutral": {
		{Content: "今天忽然想起%s，记下来。", Emotion: "neutral"},
		{Content: "关于%s，想分享这一刻。", Emotion: "neutral"},
	},
}

var commentTemplates = []string{
	"我也喜欢这个！",
	"看到这条心情都变好了。",
	"下次带上我呀。",
	"抱抱你。",
	"哈哈，太可爱了。",
}

// Generate 按人设语气挑一条模板；有近期事件时优先把事件填进槽位。
func (g *TemplateGenerator) Generate(ctx context.Context, in GenerateInput) (Generated, error) {
	if err := ctx.Err(); err != nil {
		return Generated{}, err
	}

	group := styleGroupOf(in.Persona)
	tpl := pickTemplate(momentTemplatePools[group])
	if len(in.RecentEvents) > 0 {
		event := in.RecentEvents[rand.Intn(len(in.RecentEvents))]
		eventTpl := pickTemplate(eventMomentTemplates[group])
		tpl = momentTemplate{
			Content: fmt.Sprintf(eventTpl.Content, event),
			Emotion: eventTpl.Emotion,
		}
	}

	if tpl.Emotion == "" {
		return Generated{Content: tpl.Content}, nil
	}
	emotion := tpl.Emotion
	return Generated{Content: tpl.Content, EmotionLabel: &emotion}, nil
}

// pickCommentContent 供定时任务生成一条 AI 评论（阶段一模板）。
func pickCommentContent() string {
	return commentTemplates[rand.Intn(len(commentTemplates))]
}

// styleGroupOf 只按关键词粗分语气组，不改写人设字段本身。
func styleGroupOf(p model.Persona) string {
	text := p.SpeakingStyle + p.PersonalityDesc
	switch {
	case containsAny(text, "活泼", "开朗", "俏皮", "元气", "可爱", "幽默"):
		return "lively"
	case containsAny(text, "温柔", "安静", "细腻", "治愈", "体贴", "暖心"):
		return "gentle"
	case containsAny(text, "高冷", "冷静", "理性", "简洁", "克制", "沉稳"):
		return "calm"
	default:
		return "neutral"
	}
}

func containsAny(text string, keywords ...string) bool {
	for _, keyword := range keywords {
		if strings.Contains(text, keyword) {
			return true
		}
	}
	return false
}

// pickTemplate 随机取一条；空池退到中性组，避免调用方拿到零值。
func pickTemplate(pool []momentTemplate) momentTemplate {
	if len(pool) == 0 {
		pool = momentTemplatePools["neutral"]
	}
	return pool[rand.Intn(len(pool))]
}
