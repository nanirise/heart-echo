package model

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"
)

// scheduleJSONKeys 契约 §10 的 Schedule 字段集，两条测试共用一份，避免两处各写一遍而漂移。
var scheduleJSONKeys = map[string]bool{
	"id":        true,
	"personaId": true,
	"content":   true,
	"remindAt":  true,
	"status":    true,
	"createdAt": true,
}

// TestScheduleJSONKeys 锁死 Schedule 序列化后的键集（恰好 6 个，与契约 §10 逐字一致）。
//
// 与 moment_like 那条不同：本表**有**契约实体，所以这是**契约校验**，不是结构探针。
// userId / sourceMessageId 是本表最容易抄错的两个——契约里没有它们，
// 而 moment_like.go 的 UserID 写的是 json:"userId"、user_memory.go 写的是 json:"-"：照抄哪一张都可能错（spec §2）。
//
// 断言用**恰好相等**而不是包含：多一个键也要报红。
func TestScheduleJSONKeys(t *testing.T) {
	raw, err := json.Marshal(Schedule{})
	if err != nil {
		t.Fatalf("序列化 Schedule 失败：%v", err)
	}

	var got map[string]any
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatalf("反序列化到 map 失败：%v", err)
	}

	for key := range got {
		if !scheduleJSONKeys[key] {
			t.Errorf("响应体多出字段 %q：要么是 userId / sourceMessageId 写成了能序列化的 tag，"+
				"要么是派生字段混进了 model（会被 AutoMigrate 建成真列）（spec §2 / §5 分组 C）", key)
		}
	}
	for key := range scheduleJSONKeys {
		if _, ok := got[key]; !ok {
			t.Errorf("响应体缺少字段 %q", key)
		}
	}
}

// TestScheduleJSONTags 查字段的 json tag 本身，而不是序列化结果——上面那条看**产物**，这条看**声明**。
//
// 为什么产物不够（实测，spec §5 分组 C 注入 13）：encoding/json 遇到**同名**字段会**静默丢弃全部**，不报错。
// 把 UserID 和 User 都写成 json:"userId" 时，输出与合规时一模一样，上面的测试照样绿。
// 静默丢字段是本仓库踩过的那类事故（tag 是字符串字面量，编译器和 vet 都不看内容），所以这里补声明的检查。
func TestScheduleJSONTags(t *testing.T) {
	claimed := map[string][]string{} // json 名 → 声明它的 Go 字段名（多个即撞名）
	for field := range reflect.TypeFor[Schedule]().Fields() {
		name := strings.Split(field.Tag.Get("json"), ",")[0]
		if name == "-" {
			continue
		}
		if name == "" {
			name = field.Name // 没写 tag 时 encoding/json 用 Go 字段名
		}
		claimed[name] = append(claimed[name], field.Name)
	}

	for name, fields := range claimed {
		if !scheduleJSONKeys[name] {
			t.Errorf("字段 %v 声明的 json 名是 %q，契约里没有这个键", fields, name)
		}
		if len(fields) > 1 {
			t.Errorf("字段 %v 共用 json 名 %q：encoding/json 会静默丢弃它们全部，不报错", fields, name)
		}
	}
	for name := range scheduleJSONKeys {
		if len(claimed[name]) == 0 {
			t.Errorf("契约字段 %q 没有任何字段声明它", name)
		}
	}
}
