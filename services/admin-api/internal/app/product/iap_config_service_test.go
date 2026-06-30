package product

import (
	"context"
	"testing"

	"github.com/csw/console/services/admin-api/internal/app/dto"
	"github.com/csw/console/services/admin-api/internal/domain/common"
	domainproduct "github.com/csw/console/services/admin-api/internal/domain/product"
)

func newIAPSvc() (*IAPConfigService, *memStore, *spyCrypto, *spyAudit) {
	store := newMemStore()
	crypto := &spyCrypto{}
	audit := &spyAudit{}
	svc := NewIAPConfigService(store, crypto, spyFile{}, audit, fixedNow)
	// 渠道 google 有 enabled 模板；game_channel 9001 属 google；包 7001 属 game_channel 9001。
	store.state.templates["google"] = iapTemplate()
	store.state.gameChannels[9001] = "google"
	store.state.packages[7001] = pkgInfo{gameID: "100001", packageCode: "pkg-a", channelID: "google", gameChannelID: 9001}
	return svc, store, crypto, audit
}

// ───────────────────────── PutGameChannelConfig ─────────────────────────

func TestPutIAPConfig_FullValid_EncryptsAndMasks(t *testing.T) {
	svc, store, crypto, audit := newIAPSvc()
	view, err := svc.PutGameChannelConfig(context.Background(), dto.UpsertIAPConfigCmd{
		GameChannelID: 9001,
		Enabled:       ptrBool(true),
		ConfigJSON:    map[string]any{"appId": "app-123", "privateKey": "super-secret"},
	})
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if view.ConfigStatus != "valid" {
		t.Fatalf("齐全配置应 valid, got %s", view.ConfigStatus)
	}
	// S8：响应密文恒 masked，绝不回明文。
	if view.ConfigJSON["privateKey"] != maskedValue {
		t.Fatalf("响应 privateKey 应 masked, got %v", view.ConfigJSON["privateKey"])
	}
	// 落库为密文（非明文、非 masked）。
	stored := store.state.iapConfigs[9001]
	if stored.ConfigJSON["privateKey"] == "super-secret" || stored.ConfigJSON["privateKey"] == maskedValue {
		t.Fatalf("落库应为密文, got %v", stored.ConfigJSON["privateKey"])
	}
	if crypto.encryptCalls != 1 {
		t.Fatalf("应加密 1 次, got %d", crypto.encryptCalls)
	}
	if len(audit.entries) != 1 || audit.entries[0].Action != "iap.config.update" {
		t.Fatalf("应写一条 iap.config.update 审计, got %+v", audit.entries)
	}
}

func TestPutIAPConfig_EnableWithoutSecret_Rejected(t *testing.T) {
	svc, _, _, _ := newIAPSvc()
	// 启用但缺密文 privateKey → invalid → 拒绝 enabled=true。
	_, err := svc.PutGameChannelConfig(context.Background(), dto.UpsertIAPConfigCmd{
		GameChannelID: 9001,
		Enabled:       ptrBool(true),
		ConfigJSON:    map[string]any{"appId": "app-123"},
	})
	if err == nil || errCode(t, err) != codeValidation {
		t.Fatalf("启用缺必填密文应 VALIDATION_FAILED, got %v", err)
	}
}

func TestPutIAPConfig_DisabledInvalidPersisted(t *testing.T) {
	svc, store, _, _ := newIAPSvc()
	// enabled=false 时允许持久化 invalid（CR 修复语义；对齐 §3.4）。
	view, err := svc.PutGameChannelConfig(context.Background(), dto.UpsertIAPConfigCmd{
		GameChannelID: 9001,
		Enabled:       ptrBool(false),
		ConfigJSON:    map[string]any{"appId": "app-123"}, // 缺 privateKey
	})
	if err != nil {
		t.Fatalf("disabled 持久化 invalid 不应报错: %v", err)
	}
	if view.ConfigStatus != "invalid" {
		t.Fatalf("缺密文应 invalid, got %s", view.ConfigStatus)
	}
	if store.state.iapConfigs[9001].ConfigStatus != common.ConfigStatusInvalid {
		t.Fatalf("落库 config_status 应 invalid")
	}
}

func TestPutIAPConfig_EmptyConfigIsEmptyStatus(t *testing.T) {
	svc, _, _, _ := newIAPSvc()
	view, err := svc.PutGameChannelConfig(context.Background(), dto.UpsertIAPConfigCmd{
		GameChannelID: 9001,
		Enabled:       ptrBool(false),
		ConfigJSON:    map[string]any{},
	})
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if view.ConfigStatus != "empty" {
		t.Fatalf("空配置应 empty, got %s", view.ConfigStatus)
	}
}

func TestPutIAPConfig_NilConfigRejected(t *testing.T) {
	svc, _, _, _ := newIAPSvc()
	_, err := svc.PutGameChannelConfig(context.Background(), dto.UpsertIAPConfigCmd{
		GameChannelID: 9001,
		ConfigJSON:    nil,
	})
	if err == nil || errCode(t, err) != codeValidation {
		t.Fatalf("configJson 缺失应 VALIDATION_FAILED, got %v", err)
	}
}

// 密文更新回归：提交 masked → 还原旧密文，不重新加密。
func TestPutIAPConfig_MaskedKeepsOldSecret(t *testing.T) {
	svc, store, crypto, _ := newIAPSvc()
	if _, err := svc.PutGameChannelConfig(context.Background(), dto.UpsertIAPConfigCmd{
		GameChannelID: 9001, Enabled: ptrBool(true),
		ConfigJSON: map[string]any{"appId": "app-123", "privateKey": "super-secret"},
	}); err != nil {
		t.Fatalf("seed: %v", err)
	}
	encBefore := store.state.iapConfigs[9001].ConfigJSON["privateKey"]
	callsBefore := crypto.encryptCalls

	// 二次提交 masked → 不应重新加密，密文保持不变。
	if _, err := svc.PutGameChannelConfig(context.Background(), dto.UpsertIAPConfigCmd{
		GameChannelID: 9001, Enabled: ptrBool(true),
		ConfigJSON: map[string]any{"appId": "app-123", "privateKey": maskedValue},
	}); err != nil {
		t.Fatalf("second put: %v", err)
	}
	if store.state.iapConfigs[9001].ConfigJSON["privateKey"] != encBefore {
		t.Fatalf("masked 提交应保留旧密文")
	}
	if crypto.encryptCalls != callsBefore {
		t.Fatalf("masked 提交不应重新加密, before=%d after=%d", callsBefore, crypto.encryptCalls)
	}
}

func TestGetIAPConfig_MasksSecret(t *testing.T) {
	svc, store, _, _ := newIAPSvc()
	store.state.iapConfigs[9001] = domainproduct.IAPConfig{
		GameChannelIDRef: 9001,
		Enabled:          true,
		ConfigJSON:       map[string]any{"appId": "app-123", "privateKey": "enc:super-secret"},
		ConfigStatus:     common.ConfigStatusValid,
	}
	view, err := svc.GetGameChannelConfig(context.Background(), 9001)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if view.Config.ConfigJSON["privateKey"] != maskedValue {
		t.Fatalf("读取密文应 masked, got %v", view.Config.ConfigJSON["privateKey"])
	}
	if view.ChannelID != "google" {
		t.Fatalf("channelId 应解析为 google, got %q", view.ChannelID)
	}
}

func TestGetIAPConfig_DefaultsWhenAbsent(t *testing.T) {
	svc, _, _, _ := newIAPSvc()
	view, err := svc.GetGameChannelConfig(context.Background(), 9001)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if view.Config.ConfigStatus != "empty" || view.Config.Enabled {
		t.Fatalf("无配置应回退 empty/disabled 默认, got %+v", view.Config)
	}
}

// ───────────────────────── PutPackageOverride ─────────────────────────

func TestPutPackageOverride_ValidMasksAndAudits(t *testing.T) {
	svc, store, _, audit := newIAPSvc()
	view, err := svc.PutPackageOverride(context.Background(), dto.UpsertPackageIAPOverrideCmd{
		PackageID:  7001,
		Enabled:    ptrBool(true),
		ConfigJSON: map[string]any{"appId": "pkg-app", "privateKey": "pkg-secret"},
	})
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if view.ConfigStatus != "valid" {
		t.Fatalf("齐全覆盖应 valid, got %s", view.ConfigStatus)
	}
	if view.ConfigJSON["privateKey"] != maskedValue {
		t.Fatalf("覆盖响应密文应 masked, got %v", view.ConfigJSON["privateKey"])
	}
	if store.state.pkgOverrides[7001].ConfigJSON["privateKey"] == "pkg-secret" {
		t.Fatalf("覆盖落库应为密文")
	}
	if len(audit.entries) != 1 || audit.entries[0].Action != "iap.override.update" {
		t.Fatalf("应写 iap.override.update 审计, got %+v", audit.entries)
	}
}

func TestGetPackageOverride_ReturnsBaseAndOverrideMasked(t *testing.T) {
	svc, store, _, _ := newIAPSvc()
	store.state.iapConfigs[9001] = domainproduct.IAPConfig{
		GameChannelIDRef: 9001, Enabled: true, ConfigStatus: common.ConfigStatusValid,
		ConfigJSON: map[string]any{"appId": "base-app", "privateKey": "enc:base"},
	}
	store.state.pkgOverrides[7001] = domainproduct.IAPConfig{
		PackageIDRef: 7001, Enabled: true, ConfigStatus: common.ConfigStatusValid,
		ConfigJSON: map[string]any{"appId": "ov-app", "privateKey": "enc:ov"},
	}
	view, err := svc.GetPackageOverride(context.Background(), 7001)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if view.BaseConfig.ConfigJSON["privateKey"] != maskedValue || view.Override.ConfigJSON["privateKey"] != maskedValue {
		t.Fatalf("base 与 override 密文都应 masked, got base=%v override=%v",
			view.BaseConfig.ConfigJSON["privateKey"], view.Override.ConfigJSON["privateKey"])
	}
	if view.PackageCode != "pkg-a" {
		t.Fatalf("packageCode 应 pkg-a, got %q", view.PackageCode)
	}
}
