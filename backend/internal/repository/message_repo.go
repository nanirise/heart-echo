package repository

import (
	"context"

	"gorm.io/gorm"

	"github.com/nanirise/heart-echo/backend/internal/model"
)

// MessageRepo 消息表的数据访问。
// 三条链路（SSE 对话 / 主动消息 / 日程提醒）共用同一份 Create，
// 不要各自写一份 INSERT —— 两套写法在 created_at 与 last_message_at 上必然对不上。
//
// 归属条件一律做进 SQL 的 WHERE，不是先查出来再在 Go 里比：
// 「先查后判」只要漏掉那个 if 就是静默越权，做进 SQL 则最坏情况是查不到。
type MessageRepo struct {
	db *gorm.DB
}

// NewMessageRepo 构造仓储；db 只用于读路径，写路径由调用方传入事务句柄。
func NewMessageRepo(db *gorm.DB) *MessageRepo {
	return &MessageRepo{db: db}
}

// Create 插入一条消息，m.ID 由 BIGSERIAL 回填；CreatedAt 由 GORM 自动填。
//
// 写路径收事务句柄 tx 而不是用 r.db：消息落库与 personas.last_message_at
// 必须同成同败（chat-message §4 的不变量），拆成两条独立语句会出现
// 「消息进去了、会话列表的排序时间没动」这种半截状态。
//
// Create 时不要给 m.User / m.Persona 关联字段赋值，否则会连带写 users / personas 表。
func (r *MessageRepo) Create(ctx context.Context, tx *gorm.DB, m *model.ChatMessage) error {
	return tx.WithContext(ctx).Create(m).Error
}

// ListByPersona 按人设分页取消息，倒序（最新在前，前端自己反转成旧→新）。
//
// 两个条件一个都不能少：少 persona_id 会把人设的消息串到一起，
// 少 user_id 会跨用户（AGENTS §4.3 红线）。
func (r *MessageRepo) ListByPersona(
	ctx context.Context, userID, personaID uint64, offset, limit int,
) ([]model.ChatMessage, int64, error) {
	// 空切片起步而不是 nil：nil slice 会序列化成 null，前端 .map 直接崩。
	// page 超范围时返回的就是这个空列表 + 真实 total，仍是 200。
	list := make([]model.ChatMessage, 0)
	var total int64

	// Count 与 Find 的 WHERE 必须逐字相同，否则 total 与 list 对不上。
	// 写成全表 COUNT(*) 不只是数字错 —— 它会把「系统里一共有多少条消息」
	// 泄漏给任意登录用户，这是越权防线的一部分。
	err := r.db.WithContext(ctx).Model(&model.ChatMessage{}).
		Where("persona_id = ? AND user_id = ?", personaID, userID).
		Count(&total).Error
	if err != nil {
		return nil, 0, err
	}

	// id DESC 兜底不能省：created_at 是事务开始时间，同一事务里落的多条
	// 时间戳完全相同，只按它排会让翻页重复或漏项。
	err = r.db.WithContext(ctx).Model(&model.ChatMessage{}).
		Where("persona_id = ? AND user_id = ?", personaID, userID).
		Order("created_at DESC, id DESC").
		Offset(offset).Limit(limit).
		Find(&list).Error
	if err != nil {
		return nil, 0, err
	}
	return list, total, nil
}
