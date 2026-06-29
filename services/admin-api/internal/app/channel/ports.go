// Package channel 是渠道与渠道实例（GameMarketChannel）的应用层：编排、事务、唯一性/兼容性校验、
// 复制清空敏感字段、隐藏/恢复、审计写入。command/query 方法集中在 ChannelService；DTO 见 internal/app/dto。
// 仓储端口在此定义，由 infra 实现（依赖方向向内）。
package channel

import (
	"context"
	"net/http"
	"time"

	adminapp "github.com/csw/console/services/admin-api/internal/app/admin"
	"github.com/csw/console/services/admin-api/internal/app/dto"
	domainchannel "github.com/csw/console/services/admin-api/internal/domain/channel"
)

// Error 携带统一错误码/HTTP 状态/消息/明细的应用层错误。
type Error struct {
	Status  int
	Code    string
	Message string
	Details []any
}

func (e *Error) Error() string { return e.Message }

// 错误码常量（00 §7.4 全局 + 模块私有 MARKET_CHANNEL_INCOMPATIBLE）。
const (
	codeValidation   = "VALIDATION_FAILED"
	codeConflict     = "CONFLICT"
	codeNotFound     = "NOT_FOUND"
	codeIncompatible = "MARKET_CHANNEL_INCOMPATIBLE"
)

func validationErr(msg string, details ...any) *Error {
	if details == nil {
		details = []any{}
	}
	return &Error{Status: http.StatusBadRequest, Code: codeValidation, Message: msg, Details: details}
}

func conflictErr(msg string) *Error {
	return &Error{Status: http.StatusConflict, Code: codeConflict, Message: msg, Details: []any{}}
}

func notFoundErr(msg string) *Error {
	return &Error{Status: http.StatusNotFound, Code: codeNotFound, Message: msg, Details: []any{}}
}

func incompatibleErr(msg string) *Error {
	return &Error{Status: http.StatusBadRequest, Code: codeIncompatible, Message: msg, Details: []any{}}
}

// fieldDetail 统一字段级校验明细（details: [{field, reason}]）。
func fieldDetail(field, reason string) any {
	return map[string]string{"field": field, "reason": reason}
}

// GameChannelPatch 编辑渠道实例的列级补丁（nil 表示不改）。仅支持 enabled/remark。
type GameChannelPatch struct {
	Enabled *bool
	Remark  *string
}

// PackagePatch 编辑渠道包的列级补丁（nil 表示不改）。
type PackagePatch struct {
	PackageName          *string
	BundleID             *string
	InheritChannelConfig *bool
	Enabled              *bool
	OverrideJSON         map[string]any // nil 不改
}

// ChannelRepository 平台级渠道主数据/策略读仓储（platform.channels / platform.channel_policies）。
type ChannelRepository interface {
	// ListChannelsWithPolicy 列出全部渠道主数据 + 策略（候选列表，按 sort 升序）。
	ListChannelsWithPolicy(ctx context.Context) ([]domainchannel.ChannelWithPolicy, error)
	// GetChannelByChannelID 按业务键取单个渠道主数据 + 策略（不存在返回 adminapp.ErrNotFound）。
	GetChannelByChannelID(ctx context.Context, channelID string) (domainchannel.ChannelWithPolicy, error)
}

// GameChannelRepository 渠道实例窄仓储（game_channels）。SQL 不写 schema 前缀、不带 env 谓词；
// env 由连接固定的 search_path 决定。region 通过 JOIN platform.channels 实时取出（不落 game_channels）。
type GameChannelRepository interface {
	// ResolveGameRowID 把对外 game_id 解析为 games.id（不存在返回 adminapp.ErrNotFound）。
	ResolveGameRowID(ctx context.Context, gameID string) (int64, error)
	// ExistsInstance 判断 (gameIDRef, market, channelID) 实例是否已存在。
	ExistsInstance(ctx context.Context, gameIDRef int64, market, channelID string) (bool, error)
	// FindInstance 取某 (gameIDRef, market, channelID) 实例（复制来源校验，不存在返回 adminapp.ErrNotFound）。
	FindInstance(ctx context.Context, gameIDRef int64, market, channelID string) (domainchannel.GameMarketChannel, error)
	// Insert 落库渠道实例，返回装配后的聚合（含 id/时间戳/region）。inst 须已带 GameIDRef/ChannelIDRef。
	Insert(ctx context.Context, inst domainchannel.GameMarketChannel) (domainchannel.GameMarketChannel, error)
	// GetByID 按 game_channels.id 取实例聚合（JOIN games/channels 装配 gameId/channelId/region）。
	GetByID(ctx context.Context, id int64) (domainchannel.GameMarketChannel, error)
	// List 分页/过滤实例列表（按 gameId + 可选 market/channelId/compatible/hidden/configStatus）。
	List(ctx context.Context, q dto.ListMarketChannelsQuery) ([]domainchannel.GameMarketChannel, int, error)
	// UpdateBasics 更新 enabled/remark（nil 不改）。
	UpdateBasics(ctx context.Context, id int64, patch GameChannelPatch) error
	// Hide 置 hidden=true 并记录操作人/时间。
	Hide(ctx context.Context, id int64, by string, at time.Time) error
	// Unhide 置 hidden=false 并清隐藏操作人/时间。
	Unhide(ctx context.Context, id int64) error
}

// ChannelPackageRepository 渠道包窄仓储（channel_packages）。
type ChannelPackageRepository interface {
	ListByGameChannel(ctx context.Context, gameChannelID int64) ([]domainchannel.ChannelPackage, error)
	ExistsPackageCode(ctx context.Context, gameChannelID int64, code string) (bool, error)
	InsertPackage(ctx context.Context, pkg domainchannel.ChannelPackage) (domainchannel.ChannelPackage, error)
	GetPackageByID(ctx context.Context, id int64) (domainchannel.ChannelPackage, error)
	UpdatePackage(ctx context.Context, id int64, patch PackagePatch) error
}

// Repositories 一组仓储句柄（绑定到 pool 或某事务连接）。
type Repositories struct {
	Channels     ChannelRepository
	GameChannels GameChannelRepository
	Packages     ChannelPackageRepository
}

// TxManager 提供事务边界，跨聚合写编排在 app 层用 InTx 包裹（01 §4.2）。
type TxManager interface {
	Repositories() Repositories
	InTx(ctx context.Context, fn func(Repositories) error) error
}

// AuditSink / AuditEntry 复用 auth 应用层端口，保持审计写入一致（00 §8）。
type (
	AuditSink  = adminapp.AuditSink
	AuditEntry = adminapp.AuditEntry
)
