package dto

import "time"

// MomentItem 是契约 §8 的 Moment 实体，8 个字段。
// personaName / liked / commentCount 都是派生字段，不是 ai_moments 的列（ai-moment spec §3.5）。
type MomentItem struct {
	ID           uint64    `json:"id"`
	PersonaID    uint64    `json:"personaId"`
	PersonaName  string    `json:"personaName"`
	Content      string    `json:"content"`
	LikeCount    int       `json:"likeCount"`
	Liked        bool      `json:"liked"`
	CommentCount int64     `json:"commentCount"`
	CreatedAt    time.Time `json:"createdAt"`
}

// CommentItem 是契约 §8 的 Comment 实体。personaId 与 userId 恰好一个非空，
// 必须用指针：值类型会把 null 序列化成 0。
type CommentItem struct {
	ID         uint64    `json:"id"`
	MomentID   uint64    `json:"momentId"`
	PersonaID  *uint64   `json:"personaId"`
	UserID     *uint64   `json:"userId"`
	AuthorName string    `json:"authorName"`
	Content    string    `json:"content"`
	CreatedAt  time.Time `json:"createdAt"`
}

// CreateCommentRequest 是 POST /moments/:id/comments 的请求体。
type CreateCommentRequest struct {
	Content string `json:"content" binding:"required"`
}

// LikeResponse 是 POST /moments/:id/like 的响应 data。
type LikeResponse struct {
	LikeCount int  `json:"likeCount"`
	Liked     bool `json:"liked"`
}
