package service

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/nanirise/heart-echo/backend/pkg/errcode"
)

// TestParseSchedule 钉住 Go ↔ Python 的内部 JSON 契约：字段是 snake_case，
// 与前端 camelCase 不同。三个出口必须能在 Go 侧区分开。
func TestParseSchedule(t *testing.T) {
	cases := []struct {
		name        string
		response    string
		hasIntent   bool
		needClarify bool
		content     string
		remindAt    string // RFC3339；空表示不应解析出时间
	}{
		{
			name:     "陈述句没有提醒意图",
			response: `{"has_intent":false}`,
		},
		{
			name:        "有意图但需要澄清",
			response:    `{"has_intent":true,"need_clarify":true}`,
			hasIntent:   true,
			needClarify: true,
		},
		{
			name:      "解析成功",
			response:  `{"has_intent":true,"need_clarify":false,"content":"开会","remind_at":"2026-10-20T15:00:00+08:00"}`,
			hasIntent: true,
			content:   "开会",
			remindAt:  "2026-10-20T15:00:00+08:00",
		},
		{
			name:      "remind_at 为空按未提供处理",
			response:  `{"has_intent":true,"need_clarify":false,"content":"开会","remind_at":""}`,
			hasIntent: true,
			content:   "开会",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method != http.MethodPost || r.URL.Path != "/schedule/parse" {
					t.Errorf("请求应为 POST /schedule/parse，实际 %s %s", r.Method, r.URL.Path)
				}
				if got := r.Header.Get("X-Internal-Token"); got != "test-token" {
					t.Errorf("X-Internal-Token 应为 test-token，实际 %q", got)
				}
				var body struct {
					Message string `json:"message"`
				}
				if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
					t.Errorf("请求体解析失败: %v", err)
				}
				if body.Message != "用户原话" {
					t.Errorf("message 应为用户原话，实际 %q", body.Message)
				}
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(tc.response))
			}))
			defer server.Close()

			client := &aiHTTPClient{
				baseURL: server.URL,
				token:   "test-token",
				http:    server.Client(),
			}
			got, err := client.ParseSchedule(context.Background(), "用户原话")
			if err != nil {
				t.Fatalf("ParseSchedule 不应失败: %v", err)
			}

			if got.HasIntent != tc.hasIntent {
				t.Errorf("HasIntent 应为 %v，实际 %v", tc.hasIntent, got.HasIntent)
			}
			if got.NeedClarify != tc.needClarify {
				t.Errorf("NeedClarify 应为 %v，实际 %v", tc.needClarify, got.NeedClarify)
			}
			if got.Content != tc.content {
				t.Errorf("Content 应为 %q，实际 %q", tc.content, got.Content)
			}

			if tc.remindAt == "" {
				if got.RemindAt != nil {
					t.Errorf("RemindAt 应为 nil，实际 %v", got.RemindAt)
				}
				return
			}
			want, err := time.Parse(time.RFC3339, tc.remindAt)
			if err != nil {
				t.Fatalf("测试用例的 remindAt 不合法: %v", err)
			}
			if got.RemindAt == nil || !got.RemindAt.Equal(want) {
				t.Errorf("RemindAt 应为 %v，实际 %v", want, got.RemindAt)
			}
		})
	}
}

// TestExtractMemories 钉住 Go ↔ Python 的记忆提取契约：字段是 snake_case，
// 且记忆候选里没有归属字段（user_id / persona_id 只能由 Go 侧补）。
func TestExtractMemories(t *testing.T) {
	const response = `{"memories":[{"memory_type":"fact","content":"用户养了一只叫豆豆的猫","importance_score":0.85}],"profile_updates":{"occupation":"程序员","interests":["跑步","猫"]}}`

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/memory/extract" {
			t.Errorf("请求应为 POST /memory/extract，实际 %s %s", r.Method, r.URL.Path)
		}
		if got := r.Header.Get("X-Internal-Token"); got != "test-token" {
			t.Errorf("X-Internal-Token 应为 test-token，实际 %q", got)
		}
		var body struct {
			Message string `json:"message"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Errorf("请求体解析失败: %v", err)
		}
		if body.Message != "我养了一只叫豆豆的猫" {
			t.Errorf("message 应为用户原话，实际 %q", body.Message)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(response))
	}))
	defer server.Close()

	client := &aiHTTPClient{baseURL: server.URL, token: "test-token", http: server.Client()}
	got, err := client.ExtractMemories(context.Background(), "我养了一只叫豆豆的猫")
	if err != nil {
		t.Fatalf("ExtractMemories 不应失败: %v", err)
	}

	if len(got.Memories) != 1 {
		t.Fatalf("应返回 1 条候选记忆，实际 %d", len(got.Memories))
	}
	candidate := got.Memories[0]
	if candidate.MemoryType != "fact" || candidate.Content != "用户养了一只叫豆豆的猫" ||
		candidate.ImportanceScore != 0.85 {
		t.Errorf("候选记忆字段解析错误: %+v", candidate)
	}
	const wantProfile = `{"occupation":"程序员","interests":["跑步","猫"]}`
	if string(got.ProfileUpdates) != wantProfile {
		t.Errorf("ProfileUpdates 应原样保留 JSON\n实际: %s\n期望: %s",
			got.ProfileUpdates, wantProfile)
	}
}

// TestStoreMemories 钉住 Go → Python 的向量写入契约：snake_case、归属 ID 在顶层，
// 记忆 ID 必须是 PG 主键（Python 拿它当 Chroma 文档 ID 做幂等 upsert）。
func TestStoreMemories(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/memory/store" {
			t.Errorf("请求应为 POST /memory/store，实际 %s %s", r.Method, r.URL.Path)
		}
		if got := r.Header.Get("X-Internal-Token"); got != "test-token" {
			t.Errorf("X-Internal-Token 应为 test-token，实际 %q", got)
		}

		var body struct {
			UserID    uint64            `json:"user_id"`
			PersonaID uint64            `json:"persona_id"`
			Memories  []MemoryStoreItem `json:"memories"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Errorf("请求体解析失败: %v", err)
		}
		if body.UserID != 7 || body.PersonaID != 3 {
			t.Errorf("归属字段应为 user_id=7 persona_id=3，实际 %+v", body)
		}
		if len(body.Memories) != 1 {
			t.Fatalf("应发送 1 条记忆，实际 %d", len(body.Memories))
		}
		memory := body.Memories[0]
		if memory.ID != 101 || memory.MemoryType != "fact" ||
			memory.Content != "用户养了一只叫豆豆的猫" || memory.ImportanceScore != 0.85 {
			t.Errorf("记忆字段错误: %+v", memory)
		}

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"stored":1}`))
	}))
	defer server.Close()

	client := &aiHTTPClient{baseURL: server.URL, token: "test-token", http: server.Client()}
	stored, err := client.StoreMemories(context.Background(), 7, 3, []MemoryStoreItem{
		{
			ID:              101,
			MemoryType:      "fact",
			Content:         "用户养了一只叫豆豆的猫",
			ImportanceScore: 0.85,
		},
	})
	if err != nil {
		t.Fatalf("StoreMemories 不应失败: %v", err)
	}
	if stored != 1 {
		t.Errorf("stored 应为 1，实际 %d", stored)
	}
}

// TestStoreMemoriesUnavailable 锁定非 200 的归类：调用方只记 warn，
// 但错误码仍应是 5002（AI 服务不可用），不能混成 5000。
func TestStoreMemoriesUnavailable(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	client := &aiHTTPClient{baseURL: server.URL, token: "test-token", http: server.Client()}

	_, err := client.StoreMemories(context.Background(), 7, 3, []MemoryStoreItem{
		{ID: 101, MemoryType: "fact", Content: "用户养了一只猫", ImportanceScore: 0.85},
	})
	if err == nil {
		t.Fatal("非 200 应返回错误")
	}
	var bizErr *errcode.BizError
	if !errors.As(err, &bizErr) || bizErr.Code != errcode.ErrAIUnavailable {
		t.Fatalf("错误码应为 %d，实际 %v", errcode.ErrAIUnavailable, err)
	}
}

// TestGenerateMoment 钉住 Go → Python 的朋友圈生成契约：字段 snake_case，
// 上下文由 Go 侧拼好，响应只取 content。
func TestGenerateMoment(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/moment/generate" {
			t.Errorf("请求应为 POST /moment/generate，实际 %s %s", r.Method, r.URL.Path)
		}
		if got := r.Header.Get("X-Internal-Token"); got != "test-token" {
			t.Errorf("X-Internal-Token 应为 test-token，实际 %q", got)
		}
		raw, err := io.ReadAll(r.Body)
		if err != nil {
			t.Errorf("读取请求体失败: %v", err)
		}
		// 按 Python Pydantic 模型的字段名逐字校验：模型是 extra="forbid"，
		// 拼错或多数一个字段都会 422。解到 MomentRequest 自身发现不了标签写错。
		var keys map[string]json.RawMessage
		if err := json.Unmarshal(raw, &keys); err != nil {
			t.Errorf("请求体解析失败: %v", err)
		}
		wantKeys := []string{"name", "personality_desc", "speaking_style", "context"}
		if len(keys) != len(wantKeys) {
			t.Errorf("请求体应恰好 %d 个字段，实际 %d 个: %s", len(wantKeys), len(keys), raw)
		}
		for _, key := range wantKeys {
			if _, ok := keys[key]; !ok {
				t.Errorf("请求体缺少字段 %q: %s", key, raw)
			}
		}

		var body MomentRequest
		if err := json.Unmarshal(raw, &body); err != nil {
			t.Errorf("请求体解析失败: %v", err)
		}
		if body.Name != "小回" || body.PersonalityDesc != "温柔" ||
			body.SpeakingStyle != "简短" || body.Context != "用户：今天好累" {
			t.Errorf("请求字段错误: %+v", body)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"content":"今天风很轻。"}`))
	}))
	defer server.Close()

	client := &aiHTTPClient{baseURL: server.URL, token: "test-token", http: server.Client()}
	got, err := client.GenerateMoment(context.Background(), MomentRequest{
		Name:            "小回",
		PersonalityDesc: "温柔",
		SpeakingStyle:   "简短",
		Context:         "用户：今天好累",
	})
	if err != nil {
		t.Fatalf("GenerateMoment 不应失败: %v", err)
	}
	if got != "今天风很轻。" {
		t.Errorf("content 应为 %q，实际 %q", "今天风很轻。", got)
	}
}

// TestGenerateMomentUnavailable 锁定非 200 的归类：调用方据此回退模板，
// 错误码不能退化成 5000。
func TestGenerateMomentUnavailable(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer server.Close()

	client := &aiHTTPClient{baseURL: server.URL, token: "test-token", http: server.Client()}

	_, err := client.GenerateMoment(context.Background(), MomentRequest{Name: "小回"})
	if err == nil {
		t.Fatal("非 200 应返回错误")
	}
	var bizErr *errcode.BizError
	if !errors.As(err, &bizErr) || bizErr.Code != errcode.ErrAIUnavailable {
		t.Fatalf("错误码应为 %d，实际 %v", errcode.ErrAIUnavailable, err)
	}
}

// TestParseScheduleUnavailable 锁定非 200 的归类：调用方虽然只记日志，
// 但错误码不能退化成 5000，否则日志里分不清「AI 服务挂了」与「解析结果有问题」。
func TestParseScheduleUnavailable(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer server.Close()

	client := &aiHTTPClient{baseURL: server.URL, token: "test-token", http: server.Client()}

	_, err := client.ParseSchedule(context.Background(), "用户原话")
	if err == nil {
		t.Fatal("非 200 应返回错误")
	}
	var bizErr *errcode.BizError
	if !errors.As(err, &bizErr) || bizErr.Code != errcode.ErrAIUnavailable {
		t.Fatalf("错误码应为 %d，实际 %v", errcode.ErrAIUnavailable, err)
	}
}
