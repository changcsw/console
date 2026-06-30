package audit

import (
	"context"
	"sync/atomic"

	"github.com/csw/console/services/admin-api/internal/domain/common"
)

type ctxKey int

const ctxAuditMeta ctxKey = iota

type writeMarker struct {
	written atomic.Bool
}

type contextMeta struct {
	ActorID int64
	Env     common.Environment
	Request *common.AuditRequestMeta
	marker  *writeMarker
}

// InjectRequestContext 注入审计请求上下文（供中间件调用）。
func InjectRequestContext(ctx context.Context, actorID int64, env common.Environment, req common.AuditRequestMeta) context.Context {
	return withContextMeta(ctx, contextMeta{
		ActorID: actorID,
		Env:     env,
		Request: &req,
	})
}

func withContextMeta(ctx context.Context, meta contextMeta) context.Context {
	if meta.marker == nil {
		meta.marker = &writeMarker{}
	}
	return context.WithValue(ctx, ctxAuditMeta, &meta)
}

func fromContext(ctx context.Context) contextMeta {
	meta, ok := ctx.Value(ctxAuditMeta).(*contextMeta)
	if !ok || meta == nil {
		return contextMeta{}
	}
	return *meta
}

func markWritten(ctx context.Context) {
	meta, ok := ctx.Value(ctxAuditMeta).(*contextMeta)
	if !ok || meta == nil || meta.marker == nil {
		return
	}
	meta.marker.written.Store(true)
}

func IsWritten(ctx context.Context) bool {
	meta, ok := ctx.Value(ctxAuditMeta).(*contextMeta)
	if !ok || meta == nil || meta.marker == nil {
		return false
	}
	return meta.marker.written.Load()
}
