package channel

import (
	"errors"

	"github.com/csw/console/services/admin-api/internal/domain/common"
)

// 渠道包/隐藏相关纯规则错误（app 层据此映射统一错误码）。
var (
	// ErrCannotHideUnhealthy 不得隐藏 invalid/empty 状态实例（00 红线）。
	ErrCannotHideUnhealthy = errors.New("cannot hide a channel instance that is not valid")
	// ErrPackageMarketMismatch 渠道包 market 必须与所属实例一致。
	ErrPackageMarketMismatch = errors.New("package market must match its channel instance market")
)

// IsValidConfigStatus 校验配置状态枚举（00 §3.4）。
func IsValidConfigStatus(s common.ConfigStatus) bool {
	switch s {
	case common.ConfigStatusEmpty, common.ConfigStatusInvalid, common.ConfigStatusValid:
		return true
	default:
		return false
	}
}

// CanHide 隐藏前置规则（compact §隐藏/恢复 + 00 红线）：仅允许隐藏 valid 实例，
// 不得隐藏 invalid/empty（避免把"未配齐"误当"主动移出生效集"）。
func CanHide(status common.ConfigStatus) error {
	if status != common.ConfigStatusValid {
		return ErrCannotHideUnhealthy
	}
	return nil
}

// ValidatePackageMarket 渠道包 market 与所属实例 market 一致性校验（compact §渠道包）。
func ValidatePackageMarket(packageMarket, instanceMarket string) error {
	if packageMarket != instanceMarket {
		return ErrPackageMarketMismatch
	}
	return nil
}
