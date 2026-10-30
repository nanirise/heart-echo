// 情绪回写的归属条件只有真实 SQL 能验证：WHERE 少一个 user_id / persona_id，
// 单测里看不出任何区别，线上却会改到别人的消息（AGENTS §4.3 红线）。
// 连不上库时按 testTx 的约定 Skip。

package repository

import (
	"context"
	"math"
	"testing"

	"github.com/nanirise/heart-echo/backend/internal/model"
)

func TestUpdateEmotion(t *testing.T) {
	tx := testTx(t)
	ctx := context.Background()

	me := seedUser(t, tx, "emotion_me")
	other := seedUser(t, tx, "emotion_other")
	mine := seedPersona(t, tx, me.ID, "我的")
	theirs := seedPersona(t, tx, other.ID, "别人的")

	mineMessage := &model.ChatMessage{
		UserID: me.ID, PersonaID: mine.ID, Role: model.RoleUser, Content: "我很难过",
	}
	if err := tx.Create(mineMessage).Error; err != nil {
		t.Fatalf("播种消息失败: %v", err)
	}
	otherMessage := &model.ChatMessage{
		UserID: other.ID, PersonaID: theirs.ID, Role: model.RoleUser, Content: "我很难过",
	}
	if err := tx.Create(otherMessage).Error; err != nil {
		t.Fatalf("播种消息失败: %v", err)
	}

	repo := NewMessageRepo(tx)
	if err := repo.UpdateEmotion(
		ctx, me.ID, mine.ID, mineMessage.ID, "sadness", 0.7,
	); err != nil {
		t.Fatalf("回写情绪失败: %v", err)
	}

	var got model.ChatMessage
	if err := tx.First(&got, mineMessage.ID).Error; err != nil {
		t.Fatalf("读回消息失败: %v", err)
	}
	if got.EmotionLabel == nil || *got.EmotionLabel != "sadness" {
		t.Fatalf("emotion_label 应为 sadness，实际 %v", got.EmotionLabel)
	}
	if got.EmotionScore == nil || math.Abs(*got.EmotionScore-0.7) > 1e-9 {
		t.Fatalf("emotion_score 应为 0.7，实际 %v", got.EmotionScore)
	}

	// 拿自己的 user/persona 去改别人的消息：必须一行都不命中。
	if err := repo.UpdateEmotion(
		ctx, me.ID, mine.ID, otherMessage.ID, "joy", 0.9,
	); err != nil {
		t.Fatalf("越权回写不应报错（应静默不命中）: %v", err)
	}

	var untouched model.ChatMessage
	if err := tx.First(&untouched, otherMessage.ID).Error; err != nil {
		t.Fatalf("读回消息失败: %v", err)
	}
	if untouched.EmotionLabel != nil || untouched.EmotionScore != nil {
		t.Fatalf("别人的消息不应被更新，实际 label=%v score=%v",
			untouched.EmotionLabel, untouched.EmotionScore)
	}
}
