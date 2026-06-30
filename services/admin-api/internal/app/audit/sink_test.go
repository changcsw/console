package audit

import (
	"context"
	"errors"
	"testing"

	adminapp "github.com/csw/console/services/admin-api/internal/app/admin"
	"github.com/csw/console/services/admin-api/internal/domain/common"
)

// SinkAdapter 是历史模块（admin/game/channel/account-auth）经 adminapp.AuditSink
// 写审计的适配器。本测试覆盖两条红线：①显式写失败 error 必须冒泡（触发业务事务回滚）；
// ②AuditEntry.SecretKeys 必须透传到 SecretAwareAuditInput，确保 detail 脱敏生效。

func TestSinkAdapter_PropagatesWriteError(t *testing.T) {
	repo := &fakeRepo{insertErr: errors.New("db down")}
	sink := NewSinkAdapter(newService(repo, common.EnvSandbox), nil)
	err := sink.Write(context.Background(), adminapp.AuditEntry{
		Action: "game.publish", ResourceType: "game", ResourceID: "g1",
	})
	if err == nil {
		t.Fatalf("expected explicit write error to propagate for tx rollback")
	}
}

func TestSinkAdapter_PassesSecretKeysForMasking(t *testing.T) {
	repo := &fakeRepo{}
	sink := NewSinkAdapter(newService(repo, common.EnvSandbox), nil)
	err := sink.Write(context.Background(), adminapp.AuditEntry{
		Action: "account_auth.update", ResourceType: "game", ResourceID: "g1",
		Detail:     map[string]any{"apiKey": "plain", "keep": "v"},
		SecretKeys: []string{"apiKey"},
	})
	if err != nil {
		t.Fatalf("Write: %v", err)
	}
	if len(repo.inserted) != 1 {
		t.Fatalf("expected one insert, got %d", len(repo.inserted))
	}
	extra := repo.inserted[0].Detail.Extra
	if extra["apiKey"] != "masked" {
		t.Fatalf("secret key not masked via SecretKeys: %v", extra["apiKey"])
	}
	if extra["keep"] != "v" {
		t.Fatalf("non-secret field should stay visible: %v", extra["keep"])
	}
}

func TestSinkAdapter_NilSafe(t *testing.T) {
	var sink *SinkAdapter
	if err := sink.Write(context.Background(), adminapp.AuditEntry{}); err != nil {
		t.Fatalf("nil sink should be no-op, got %v", err)
	}
}
