package service

import (
	"context"
	"errors"

	"go.uber.org/zap"
	"gorm.io/gorm"

	"github.com/nanirise/heart-echo/backend/internal/dto"
	"github.com/nanirise/heart-echo/backend/internal/model"
	"github.com/nanirise/heart-echo/backend/internal/repository"
	"github.com/nanirise/heart-echo/backend/pkg/errcode"
)

// scheduleNudgePrefix 必须保留 [nudge] 前缀：ai-service 的 system prompt 靠它识别
// "这不是用户刚手打的消息"。少了前缀，模型会把系统注入的提醒当成用户自己说的话。
const scheduleNudgePrefix = "[nudge] 到时间了："

// ScheduleService 日程的列表、取消与触发。
// 触发只有一条实现（trigger）：手动接口与定时任务都走它，满足契约 §10
// "手动触发必须复用定时任务同一段逻辑"。
type ScheduleService struct {
	repo        *repository.ScheduleRepo
	personaRepo *repository.PersonaRepo
	chat        *ChatService
}

// NewScheduleService 构造服务。chat 与主动消息共用同一个注入链路。
func NewScheduleService(db *gorm.DB, chat *ChatService) *ScheduleService {
	return &ScheduleService{
		repo:        repository.NewScheduleRepo(db),
		personaRepo: repository.NewPersonaRepo(db),
		chat:        chat,
	}
}

// ListSchedules 返回某个人设的日程分页。
//
// 先校验人设归属再查列表：失败一律 4043（契约 §10），不暴露该 persona_id 是否存在。
func (s *ScheduleService) ListSchedules(
	ctx context.Context, userID, personaID uint64, page, pageSize int,
) (*dto.PageResult[dto.ScheduleResponse], error) {
	if userID == 0 {
		return nil, errcode.New(errcode.ErrUnauthorized)
	}
	if personaID == 0 {
		return nil, errcode.New(errcode.ErrInvalidParams)
	}

	owned, err := s.personaRepo.ExistsOwnedByUser(ctx, userID, personaID)
	if err != nil {
		return nil, errcode.Wrap(errcode.ErrDBFailed, err)
	}
	if !owned {
		return nil, errcode.New(errcode.ErrPersonaNotFound)
	}

	page, pageSize = dto.ClampPage(page, pageSize)
	list, total, err := s.repo.ListByPersona(ctx, userID, personaID, (page-1)*pageSize, pageSize)
	if err != nil {
		return nil, errcode.Wrap(errcode.ErrDBFailed, err)
	}

	items := make([]dto.ScheduleResponse, 0, len(list))
	for i := range list {
		items = append(items, dto.NewScheduleResponse(&list[i]))
	}
	return &dto.PageResult[dto.ScheduleResponse]{
		List:     items,
		Total:    total,
		Page:     page,
		PageSize: pageSize,
	}, nil
}

// CancelSchedule 软删除一条日程。
//
// 越权与不存在都收敛到 4040（契约 §10），不返回 403：403 会暴露该 id 存在。
// 已取消的行再次删除仍返回成功——仓储层不限制 status，重复删除是幂等的。
func (s *ScheduleService) CancelSchedule(ctx context.Context, userID, scheduleID uint64) error {
	if userID == 0 {
		return errcode.New(errcode.ErrUnauthorized)
	}
	if scheduleID == 0 {
		return errcode.New(errcode.ErrNotFound)
	}

	affected, err := s.repo.Cancel(ctx, userID, scheduleID)
	if err != nil {
		return errcode.Wrap(errcode.ErrDBFailed, err)
	}
	if affected == 0 {
		return errcode.New(errcode.ErrNotFound)
	}
	return nil
}

// TriggerSchedule 手动触发一条日程的提醒，返回生成的助手消息。
//
// 契约的 POST /schedules/:id/trigger 没有 personaId 入参，persona_id 只能从行上取。
// 归属先由 FindOwned 的 user_id 锁死；下面再查一次 persona 归属，是为了把
// InjectNudge 可能返回的 4043 收敛成契约 §10 只列的 4040。
//
// 刻意不检查 remind_at <= NOW()：手动触发就是给演示/提前提醒用的，
// 到点条件只属于定时任务的 ListPendingDue。
func (s *ScheduleService) TriggerSchedule(
	ctx context.Context, userID, scheduleID uint64,
) (*NudgeResult, error) {
	if userID == 0 {
		return nil, errcode.New(errcode.ErrUnauthorized)
	}
	if scheduleID == 0 {
		return nil, errcode.New(errcode.ErrNotFound)
	}

	schedule, err := s.repo.FindOwned(ctx, userID, scheduleID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errcode.New(errcode.ErrNotFound)
		}
		return nil, errcode.Wrap(errcode.ErrDBFailed, err)
	}
	// cancelled 是软删除，sent 是已提醒过：两者都不再触发，统一按不存在处理。
	if schedule.Status != model.ScheduleStatusPending {
		return nil, errcode.New(errcode.ErrNotFound)
	}
	// 契约 §10 的触发端点只列 4040：persona 不归属（正常路径写不出这种行，
	// 只可能来自脏数据）也收敛到 4040，不让 InjectNudge 的 4043 漏出去。
	owned, err := s.personaRepo.ExistsOwnedByUser(ctx, userID, schedule.PersonaID)
	if err != nil {
		return nil, errcode.Wrap(errcode.ErrDBFailed, err)
	}
	if !owned {
		return nil, errcode.New(errcode.ErrNotFound)
	}
	return s.trigger(ctx, schedule)
}

// ListPendingDue 定时任务入口：返回已到期仍待提醒的日程（跨用户，不回给请求方）。
func (s *ScheduleService) ListPendingDue(
	ctx context.Context, limit int,
) ([]model.Schedule, error) {
	list, err := s.repo.ListPendingDue(ctx, limit)
	if err != nil {
		return nil, errcode.Wrap(errcode.ErrDBFailed, err)
	}
	return list, nil
}

// trigger 是手动触发与定时任务共用的唯一注入路径：
// 先走 InjectNudge（落 [nudge] 消息 + 生成回复），成功后再把状态置为 sent。
//
// 顺序不能反：先置 sent 再注入，一旦注入失败提醒就永久丢失；当前顺序是 at-least-once，
// 极端并发下可能重复提醒，但不会漏。
func (s *ScheduleService) trigger(
	ctx context.Context, schedule *model.Schedule,
) (*NudgeResult, error) {
	result, err := s.chat.InjectNudge(ctx, NudgeInput{
		UserID:    schedule.UserID,
		PersonaID: schedule.PersonaID,
		Content:   scheduleNudgePrefix + schedule.Content,
	})
	if err != nil {
		return nil, err
	}

	affected, err := s.repo.MarkSent(ctx, schedule.UserID, schedule.ID)
	if err != nil {
		return nil, errcode.Wrap(errcode.ErrDBFailed, err)
	}
	if affected == 0 {
		// 扫描后到注入前被用户取消 / 被另一路径触发：提醒已经发出，不能回滚，
		// 仍按成功返回，只留一条日志供排查。
		zap.L().Warn("schedule: reminder injected but row is no longer pending",
			zap.Uint64("scheduleID", schedule.ID),
			zap.Uint64("userID", schedule.UserID),
		)
	}
	return result, nil
}
