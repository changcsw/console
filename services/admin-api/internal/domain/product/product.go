package product

import (
	"errors"
	"strings"
	"time"

	"github.com/csw/console/services/admin-api/internal/domain/accountauth"
	"github.com/csw/console/services/admin-api/internal/domain/common"
)

const (
	MaxProductIDLen = 128
	MaxPriceIDLen   = 64
)

type Product struct {
	ID              int64
	GameIDRef       int64
	GameID          string
	ProductID       string
	ProductName     string
	BaseAmountMinor int64
	BaseCurrency    string
	PriceID         string
	Enabled         bool
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

type ChannelProduct struct {
	ID                int64
	ProductIDRef      int64
	PackageIDRef      int64
	ProductIDMode     common.OverrideMode
	ProductIDOverride string
	PriceIDMode       common.OverrideMode
	PriceIDOverride   string
	Enabled           bool
	CreatedAt         time.Time
	UpdatedAt         time.Time
}

type IAPConfig struct {
	ID               int64
	GameChannelIDRef int64
	PackageIDRef     int64
	Enabled          bool
	ConfigJSON       map[string]any
	ConfigStatus     common.ConfigStatus
	LastCheckAt      *time.Time
	LastCheckMessage string
	CreatedAt        time.Time
	UpdatedAt        time.Time
}

type EffectiveIDs struct {
	ProductID string
	PriceID   string
	Warnings  []string
}

var (
	ErrInvalidProductIDMode = errors.New("product_id_mode must be default/override")
	ErrInvalidPriceIDMode   = errors.New("price_id_mode must be default/override")
	ErrOverrideRequired     = errors.New("override value is required when mode=override")
)

// NormalizeOverrideField 归一化 mode+override 组合：default 强制清空，override 要求非空。
func NormalizeOverrideField(mode common.OverrideMode, override string, maxLen int) (common.OverrideMode, string, error) {
	trimmed := strings.TrimSpace(override)
	switch mode {
	case "", common.OverrideModeDefault:
		return common.OverrideModeDefault, "", nil
	case common.OverrideModeOverride:
		if trimmed == "" {
			return "", "", ErrOverrideRequired
		}
		if len(trimmed) > maxLen {
			return "", "", errors.New("override value too long")
		}
		return common.OverrideModeOverride, trimmed, nil
	default:
		return "", "", errors.New("invalid override mode")
	}
}

// ResolveEffectiveIDs 按两组独立覆盖规则计算生效 product_id / price_id。
func ResolveEffectiveIDs(baseProductID, basePriceID string, item ChannelProduct) EffectiveIDs {
	effective := EffectiveIDs{ProductID: baseProductID, PriceID: basePriceID}
	if item.ProductIDMode == common.OverrideModeOverride {
		if strings.TrimSpace(item.ProductIDOverride) != "" {
			effective.ProductID = strings.TrimSpace(item.ProductIDOverride)
		} else {
			effective.Warnings = append(effective.Warnings, "product_id override empty; fallback to base")
		}
	}
	if item.PriceIDMode == common.OverrideModeOverride {
		if strings.TrimSpace(item.PriceIDOverride) != "" {
			effective.PriceID = strings.TrimSpace(item.PriceIDOverride)
		} else {
			effective.Warnings = append(effective.Warnings, "price_id override empty; fallback to base")
		}
	}
	return effective
}

// DeriveConfigStatus 推导模板驱动配置状态（empty/invalid/valid）。
func DeriveConfigStatus(config map[string]any, tpl accountauth.Template) (common.ConfigStatus, string) {
	if len(config) == 0 {
		return common.ConfigStatusEmpty, ""
	}
	status, msg := accountauth.ValidateConfigAgainstTemplate(config, tpl)
	return status, msg
}

// MergeIAPConfig 顶层键覆盖：override 同名键覆盖 base。
func MergeIAPConfig(base, override map[string]any) map[string]any {
	out := map[string]any{}
	for k, v := range base {
		out[k] = v
	}
	for k, v := range override {
		out[k] = v
	}
	return out
}
