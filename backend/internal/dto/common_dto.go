package dto

// 分页参数的缺省值与上限。契约 §1：page 从 1 开始，pageSize 默认 20、上限 100。
const (
	DefaultPage     = 1
	DefaultPageSize = 20
	MaxPageSize     = 100
)

// PageResult 是全项目唯一的分页响应结构，五处分页端点（persona / chat-message /
// user-memory / moments / schedules）共用。不要另建第二个。
type PageResult[T any] struct {
	List     []T   `json:"list"`
	Total    int64 `json:"total"`
	Page     int   `json:"page"`
	PageSize int   `json:"pageSize"`
}

// ClampPage 把分页参数收敛到合法区间：page<1 → 1；pageSize<1 → 20；pageSize>100 → 100。
// 是"收敛"不是"报错"——分页参数不产生错误码，写成 4001 属于契约漂移。
func ClampPage(page, pageSize int) (int, int) {
	if page < DefaultPage {
		page = DefaultPage
	}
	if pageSize < 1 {
		pageSize = DefaultPageSize
	}
	if pageSize > MaxPageSize {
		pageSize = MaxPageSize
	}
	return page, pageSize
}
