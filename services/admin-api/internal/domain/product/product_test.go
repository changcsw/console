package product

import (
	"errors"
	"strings"
	"testing"

	"github.com/csw/console/services/admin-api/internal/domain/accountauth"
	"github.com/csw/console/services/admin-api/internal/domain/common"
)

// 纯逻辑单测：NormalizeOverrideField / ResolveEffectiveIDs / DeriveConfigStatus / MergeIAPConfig。无 IO。
// 红线：product_id(≤128) 与 price_id(≤64) 两维独立、长度上限不同、禁止互填。

func TestNormalizeOverrideField_DefaultClearsOverride(t *testing.T) {
	// default 模式：无论传入什么 override 一律清空。
	mode, override, err := NormalizeOverrideField(common.OverrideModeDefault, "残值应被清空", MaxProductIDLen)
	if err != nil {
		t.Fatalf("default 不应报错: %v", err)
	}
	if mode != common.OverrideModeDefault || override != "" {
		t.Fatalf("default 应强制清空 override, got mode=%s override=%q", mode, override)
	}
}

func TestNormalizeOverrideField_EmptyModeTreatedAsDefault(t *testing.T) {
	mode, override, err := NormalizeOverrideField("", "x", MaxPriceIDLen)
	if err != nil || mode != common.OverrideModeDefault || override != "" {
		t.Fatalf("空 mode 应按 default 处理, got mode=%s override=%q err=%v", mode, override, err)
	}
}

func TestNormalizeOverrideField_OverrideRequiresValue(t *testing.T) {
	if _, _, err := NormalizeOverrideField(common.OverrideModeOverride, "   ", MaxProductIDLen); !errors.Is(err, ErrOverrideRequired) {
		t.Fatalf("override 模式空值应返回 ErrOverrideRequired, got %v", err)
	}
}

func TestNormalizeOverrideField_OverrideTrimsAndKeeps(t *testing.T) {
	mode, override, err := NormalizeOverrideField(common.OverrideModeOverride, "  sku-001  ", MaxProductIDLen)
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if mode != common.OverrideModeOverride || override != "sku-001" {
		t.Fatalf("override 应 trim 并保留, got mode=%s override=%q", mode, override)
	}
}

func TestNormalizeOverrideField_LengthLimits(t *testing.T) {
	// product_id 上限 128：129 拒绝、128 通过。
	if _, _, err := NormalizeOverrideField(common.OverrideModeOverride, strings.Repeat("a", 129), MaxProductIDLen); err == nil {
		t.Fatalf("productId override 超 128 应报错")
	}
	if _, ov, err := NormalizeOverrideField(common.OverrideModeOverride, strings.Repeat("a", 128), MaxProductIDLen); err != nil || len(ov) != 128 {
		t.Fatalf("productId override 等于 128 应通过, err=%v len=%d", err, len(ov))
	}
	// price_id 上限 64：65 拒绝、64 通过。两维上限不同。
	if _, _, err := NormalizeOverrideField(common.OverrideModeOverride, strings.Repeat("b", 65), MaxPriceIDLen); err == nil {
		t.Fatalf("priceId override 超 64 应报错")
	}
	if _, ov, err := NormalizeOverrideField(common.OverrideModeOverride, strings.Repeat("b", 64), MaxPriceIDLen); err != nil || len(ov) != 64 {
		t.Fatalf("priceId override 等于 64 应通过, err=%v len=%d", err, len(ov))
	}
	// price_id 维度下，长度 65（productId 合法但 priceId 非法）必须被各自上限拦截，验证不可混用。
	if _, _, err := NormalizeOverrideField(common.OverrideModeOverride, strings.Repeat("c", 100), MaxPriceIDLen); err == nil {
		t.Fatalf("priceId 维度 100 字符应被 64 上限拒绝（两维不可混）")
	}
}

func TestNormalizeOverrideField_InvalidMode(t *testing.T) {
	if _, _, err := NormalizeOverrideField(common.OverrideMode("bogus"), "x", MaxProductIDLen); err == nil {
		t.Fatalf("非法 mode 应报错")
	}
}

func TestResolveEffectiveIDs_BothDefaultFallbackBase(t *testing.T) {
	eff := ResolveEffectiveIDs("base.product", "base.price", ChannelProduct{
		ProductIDMode: common.OverrideModeDefault,
		PriceIDMode:   common.OverrideModeDefault,
	})
	if eff.ProductID != "base.product" || eff.PriceID != "base.price" {
		t.Fatalf("两维 default 应回退基准, got %+v", eff)
	}
	if len(eff.Warnings) != 0 {
		t.Fatalf("default 不应产生 warning, got %v", eff.Warnings)
	}
}

func TestResolveEffectiveIDs_IndependentOverride(t *testing.T) {
	// 仅 price 覆盖，product 回退基准——体现两维独立。
	eff := ResolveEffectiveIDs("base.product", "base.price", ChannelProduct{
		ProductIDMode:   common.OverrideModeDefault,
		PriceIDMode:     common.OverrideModeOverride,
		PriceIDOverride: "price-override",
	})
	if eff.ProductID != "base.product" {
		t.Fatalf("product 维 default 应回退基准, got %q", eff.ProductID)
	}
	if eff.PriceID != "price-override" {
		t.Fatalf("price 维 override 应生效, got %q", eff.PriceID)
	}

	// 仅 product 覆盖，price 回退基准。
	eff2 := ResolveEffectiveIDs("base.product", "base.price", ChannelProduct{
		ProductIDMode:     common.OverrideModeOverride,
		ProductIDOverride: "sku-override",
		PriceIDMode:       common.OverrideModeDefault,
	})
	if eff2.ProductID != "sku-override" || eff2.PriceID != "base.price" {
		t.Fatalf("product 覆盖 + price 回退应独立解析, got %+v", eff2)
	}
}

func TestResolveEffectiveIDs_OverrideEmptyDefensiveFallback(t *testing.T) {
	// override 模式但 override 空：防御性回退基准并记 warning（compact §生效解析 规则3）。
	eff := ResolveEffectiveIDs("base.product", "base.price", ChannelProduct{
		ProductIDMode:     common.OverrideModeOverride,
		ProductIDOverride: "",
		PriceIDMode:       common.OverrideModeOverride,
		PriceIDOverride:   "   ",
	})
	if eff.ProductID != "base.product" || eff.PriceID != "base.price" {
		t.Fatalf("override 空应防御回退基准, got %+v", eff)
	}
	if len(eff.Warnings) != 2 {
		t.Fatalf("两维 override 空各记 1 条 warning, got %v", eff.Warnings)
	}
}

func TestResolveEffectiveIDs_BothOverride(t *testing.T) {
	eff := ResolveEffectiveIDs("base.product", "base.price", ChannelProduct{
		ProductIDMode:     common.OverrideModeOverride,
		ProductIDOverride: "sku",
		PriceIDMode:       common.OverrideModeOverride,
		PriceIDOverride:   "price",
	})
	if eff.ProductID != "sku" || eff.PriceID != "price" {
		t.Fatalf("两维 override 应各自生效, got %+v", eff)
	}
}

func TestDeriveConfigStatus_EmptyConfig(t *testing.T) {
	status, msg := DeriveConfigStatus(map[string]any{}, accountauth.Template{TemplateVersion: "v1"})
	if status != common.ConfigStatusEmpty || msg != "" {
		t.Fatalf("空配置应为 empty, got status=%s msg=%q", status, msg)
	}
}

func TestDeriveConfigStatus_ClearedSecretMustBeInvalidNotEmpty(t *testing.T) {
	// 红线：复制创建清空 secret/file 的实例必须 invalid 不得 empty。
	// 配置含非密文字段（非空），但 secret 字段缺失 → 非空 config 走校验 → invalid。
	tpl := accountauth.Template{
		TemplateVersion: "v1",
		SecretFields:    []string{"privateKey"},
	}
	config := map[string]any{"clientId": "abc"} // privateKey 被清空缺失
	status, msg := DeriveConfigStatus(config, tpl)
	if status != common.ConfigStatusInvalid {
		t.Fatalf("缺密文字段的非空配置必须 invalid（不得 empty）, got %s", status)
	}
	if !strings.Contains(msg, "privateKey") {
		t.Fatalf("invalid 消息应标注缺失密文字段, got %q", msg)
	}
}

func TestDeriveConfigStatus_Valid(t *testing.T) {
	tpl := accountauth.Template{
		TemplateVersion: "v1",
		SecretFields:    []string{"privateKey"},
		FormSchema:      []accountauth.FormField{{Key: "clientId", Required: true}},
	}
	config := map[string]any{"clientId": "abc", "privateKey": "enc:xxx"}
	status, msg := DeriveConfigStatus(config, tpl)
	if status != common.ConfigStatusValid || msg != "" {
		t.Fatalf("齐全配置应 valid, got status=%s msg=%q", status, msg)
	}
}

func TestMergeIAPConfig_TopLevelOverride(t *testing.T) {
	base := map[string]any{"appId": "base-app", "region": "GLOBAL", "kept": "base"}
	override := map[string]any{"appId": "pkg-app", "region": "JP"}
	merged := MergeIAPConfig(base, override)
	if merged["appId"] != "pkg-app" || merged["region"] != "JP" {
		t.Fatalf("override 同名顶层键应覆盖 base, got %+v", merged)
	}
	if merged["kept"] != "base" {
		t.Fatalf("base 独有键应保留, got %+v", merged)
	}
	// 不修改入参。
	if base["appId"] != "base-app" {
		t.Fatalf("MergeIAPConfig 不应修改 base 入参")
	}
}

func TestMergeIAPConfig_EmptyOverrideKeepsBase(t *testing.T) {
	base := map[string]any{"appId": "base-app"}
	merged := MergeIAPConfig(base, map[string]any{})
	if merged["appId"] != "base-app" {
		t.Fatalf("空 override 应等于 base, got %+v", merged)
	}
}
