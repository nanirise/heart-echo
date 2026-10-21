package service

import (
	"context"
	"testing"
	"time"

	"go.uber.org/zap"
	"gorm.io/gorm"

	"github.com/nanirise/heart-echo/backend/internal/model"
)

// fakeMomentJobRepo 只实现定时任务用到的仓储方法；频率限制相关的返回值由用例设定。
type fakeMomentJobRepo struct {
	todayCount      int64
	lastMoment      *time.Time
	recentTimeCalls int
	created         []*model.AIMoment
}

func (f *fakeMomentJobRepo) ListAllPersonasForJob(
	_ context.Context,
) ([]model.Persona, error) {
	return nil, nil
}

func (f *fakeMomentJobRepo) CountTodayMoments(
	_ context.Context, _, _ uint64,
) (int64, error) {
	return f.todayCount, nil
}

func (f *fakeMomentJobRepo) GetRecentMomentTime(
	_ context.Context, _ uint64,
) (*time.Time, error) {
	f.recentTimeCalls++
	return f.lastMoment, nil
}

func (f *fakeMomentJobRepo) ListRecentEventsByPersona(
	_ context.Context, _, _ uint64, _ int,
) ([]string, error) {
	return nil, nil
}

func (f *fakeMomentJobRepo) ListRecentMessagesByPersona(
	_ context.Context, _, _ uint64, _ int,
) ([]model.ChatMessage, error) {
	return nil, nil
}

func (f *fakeMomentJobRepo) CreateMoment(
	_ context.Context, m *model.AIMoment,
) error {
	f.created = append(f.created, m)
	return nil
}

func (f *fakeMomentJobRepo) ListOtherPersonasInSameAccount(
	_ context.Context, _, _ uint64,
) ([]model.Persona, error) {
	return nil, nil
}

func (f *fakeMomentJobRepo) CreateComment(
	_ context.Context, _ *gorm.DB, _ *model.MomentComment,
) error {
	return nil
}

func newTestMomentJob(repo momentJobRepo, generator ContentGenerator) *MomentJob {
	return &MomentJob{
		repo:      repo,
		generator: generator,
		interval:  30 * time.Minute,
		logger:    zap.NewNop(),
	}
}

func testMomentPersona() *model.Persona {
	return &model.Persona{ID: 7, UserID: 3, Name: "小回"}
}

func TestGenerateForPersonaSkipsAtDailyLimit(t *testing.T) {
	repo := &fakeMomentJobRepo{todayCount: momentDailyLimit}
	generator := &fakeContentGenerator{result: Generated{Content: "不应生成"}}
	job := newTestMomentJob(repo, generator)

	if err := job.generateForPersona(context.Background(), testMomentPersona()); err != nil {
		t.Fatalf("达到日上限应跳过而不是报错: %v", err)
	}
	if generator.called {
		t.Error("达到日上限后不应调用生成器")
	}
	if len(repo.created) != 0 {
		t.Error("达到日上限后不应落库")
	}
	// 日上限必须排在最小间隔判断之前：否则还要多查一次 MAX(created_at)
	if repo.recentTimeCalls != 0 {
		t.Errorf("日上限命中后不应再查最近发布时间，实际查询 %d 次", repo.recentTimeCalls)
	}
}

func TestGenerateForPersonaSkipsWithinMinInterval(t *testing.T) {
	recent := time.Now().Add(-2 * time.Hour)
	repo := &fakeMomentJobRepo{todayCount: 2, lastMoment: &recent}
	generator := &fakeContentGenerator{result: Generated{Content: "不应生成"}}
	job := newTestMomentJob(repo, generator)

	if err := job.generateForPersona(context.Background(), testMomentPersona()); err != nil {
		t.Fatalf("未满最小间隔应跳过而不是报错: %v", err)
	}
	if generator.called {
		t.Error("距上一条不足 3 小时时不应调用生成器")
	}
	if len(repo.created) != 0 {
		t.Error("距上一条不足 3 小时时不应落库")
	}
}

func TestGenerateForPersonaGeneratesOutsideMinInterval(t *testing.T) {
	last := time.Now().Add(-3*time.Hour - 30*time.Minute)
	repo := &fakeMomentJobRepo{todayCount: 2, lastMoment: &last}
	generator := &fakeContentGenerator{result: Generated{Content: "今天风很轻。"}}
	job := newTestMomentJob(repo, generator)

	if err := job.generateForPersona(context.Background(), testMomentPersona()); err != nil {
		t.Fatalf("满足频率限制时应正常生成: %v", err)
	}
	if !generator.called {
		t.Error("距上一条 3.5 小时且今日未满 5 条时应调用生成器")
	}
	if len(repo.created) != 1 {
		t.Fatalf("应落库 1 条动态，实际 %d 条", len(repo.created))
	}
	if repo.created[0].Content != "今天风很轻。" {
		t.Errorf("落库正文应为生成结果，实际 %q", repo.created[0].Content)
	}
}

func TestBuildMomentContextOrdersOldestFirstAndLabelsRoles(t *testing.T) {
	// 仓储语义是最新在前；拼 prompt 时必须反转成时间正序。
	messages := []model.ChatMessage{
		{Role: model.RoleAssistant, Content: "今天也辛苦啦"},
		{Role: model.RoleUser, Content: "我养了一只猫"},
	}

	got := buildMomentContext("小回", messages, []string{"用户养了一只叫豆豆的猫"})
	want := "最近聊天记录：\n" +
		"用户：我养了一只猫\n" +
		"小回：今天也辛苦啦\n" +
		"你记得关于用户的事：\n" +
		"- 用户养了一只叫豆豆的猫"

	if got != want {
		t.Errorf("context 顺序或标签错误\n实际:\n%s\n期望:\n%s", got, want)
	}
}

func TestBuildMomentContextSkipsEmptyContent(t *testing.T) {
	got := buildMomentContext(
		"小回",
		[]model.ChatMessage{{Role: model.RoleUser, Content: "   "}},
		nil,
	)
	if got != "" {
		t.Errorf("空消息不应产出上下文，实际 %q", got)
	}
}

func TestBuildMomentContextFallsBackToSecondPersonSpeaker(t *testing.T) {
	got := buildMomentContext(
		"",
		[]model.ChatMessage{{Role: model.RoleAssistant, Content: "早点睡"}},
		nil,
	)
	if got != "最近聊天记录：\n你：早点睡" {
		t.Errorf("人设名为空时 assistant 标签应退化为「你」，实际 %q", got)
	}
}
