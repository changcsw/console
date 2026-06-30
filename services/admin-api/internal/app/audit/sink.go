package audit

import (
	"context"
	"log/slog"

	adminapp "github.com/csw/console/services/admin-api/internal/app/admin"
	"github.com/csw/console/services/admin-api/internal/domain/common"
)

// SinkAdapter 兼容历史模块使用的 adminapp.AuditSink 端口。
type SinkAdapter struct {
	svc    Service
	logger *slog.Logger
}

func NewSinkAdapter(svc Service, logger *slog.Logger) *SinkAdapter {
	return &SinkAdapter{svc: svc, logger: logger}
}

func (s *SinkAdapter) Write(ctx context.Context, entry adminapp.AuditEntry) error {
	if s == nil || s.svc == nil {
		return nil
	}
	err := s.svc.Write(ctx, SecretAwareAuditInput{
		AuditWriteInput: AuditWriteInput{
			ActorID:      entry.ActorID,
			Action:       entry.Action,
			ResourceType: entry.ResourceType,
			ResourceID:   entry.ResourceID,
			Detail: common.AuditDetail{
				Extra: entry.Detail,
			},
		},
		SecretKeys: append([]string(nil), entry.SecretKeys...),
	})
	if err != nil && s.logger != nil {
		s.logger.Error("audit explicit write failed", "action", entry.Action, "resourceType", entry.ResourceType, "resourceID", entry.ResourceID, "err", err)
	}
	return err
}
