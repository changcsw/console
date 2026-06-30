package common

import "time"

// AuditLog 是审计日志值对象（写后只读）。
type AuditLog struct {
	ID           int64
	ActorID      int64
	Action       string
	ResourceType string
	ResourceID   string
	Env          Environment
	Detail       AuditDetail
	CreatedAt    time.Time
}

// AuditDetail 记录关键 before/after 与请求摘要。
type AuditDetail struct {
	Summary string            `json:"summary,omitempty"`
	Before  map[string]any    `json:"before,omitempty"`
	After   map[string]any    `json:"after,omitempty"`
	Changed []string          `json:"changed,omitempty"`
	Extra   map[string]any    `json:"extra,omitempty"`
	Request *AuditRequestMeta `json:"request,omitempty"`
}

// AuditRequestMeta 保存请求元信息，用于审计追踪。
type AuditRequestMeta struct {
	IP        string `json:"ip,omitempty"`
	UserAgent string `json:"userAgent,omitempty"`
	RequestID string `json:"requestId,omitempty"`
	Method    string `json:"method,omitempty"`
	Path      string `json:"path,omitempty"`
}
