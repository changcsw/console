package payment

import (
	"errors"
	"strings"
)

type Route struct {
	ID              int64
	GameID          string
	Package         string
	Channel         string
	Market          string
	Country         string
	Currency        string
	PayWay          string
	Provider        string
	MerchantAccount string
	Priority        int
	Enabled         bool
}

type MatchInput struct {
	PayWay   string
	Package  string
	Channel  string
	Market   string
	Country  string
	Currency string
}

type RouteTarget struct {
	Provider        string
	MerchantAccount string
}

var ErrRouteNotFound = errors.New("route not found")

type ConflictKind string

const (
	ConflictDuplicatePriority ConflictKind = "duplicate_priority"
	ConflictDuplicateSelector ConflictKind = "duplicate_selector"
)

type RouteConflictError struct {
	Kind     ConflictKind
	PayWay   string
	Priority int
	Selector string
	// 两条冲突路由在校验集合中的定位（供 handler 映射进 details 供前端行级高亮）。
	// LeftIndex/RightIndex 恒可用（提交序索引）；LeftID/RightID 仅在存量路由（非全量新增）时非零。
	LeftIndex  int
	RightIndex int
	LeftID     int64
	RightID    int64
}

func (e *RouteConflictError) Error() string {
	if e == nil {
		return "route conflict"
	}
	return "route conflict: " + string(e.Kind)
}

func normalizeSelector(value string) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" || trimmed == "*" {
		return "*"
	}
	return trimmed
}

func normalizeUpperSelector(value string) string {
	out := normalizeSelector(value)
	if out == "*" {
		return out
	}
	return strings.ToUpper(out)
}
