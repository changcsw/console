// Package game 是游戏主数据（Game Core）的应用层：编排、事务、唯一性/自洽校验、game_id/secret 生成、审计写入。
// command/query 方法集中在 GameService；DTO 见 internal/app/dto。仓储端口在此定义，由 infra 实现（依赖方向向内）。
package game

import (
	"context"
	"net/http"

	adminapp "github.com/csw/console/services/admin-api/internal/app/admin"
	"github.com/csw/console/services/admin-api/internal/app/dto"
	domaingame "github.com/csw/console/services/admin-api/internal/domain/game"
)

// Error 携带统一错误码/HTTP 状态/消息/明细的应用层错误（仅用全局错误码，不新增，见 game compact）。
type Error struct {
	Status  int
	Code    string
	Message string
	Details []any
}

func (e *Error) Error() string { return e.Message }

// 全局错误码常量（00 §7.4），与 transport/http/httpx 对齐。
const (
	codeValidation = "VALIDATION_FAILED"
	codeConflict   = "CONFLICT"
	codeNotFound   = "NOT_FOUND"
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

// fieldDetail 统一字段级校验明细（details: [{field, reason}]）。
func fieldDetail(field, reason string) any {
	return map[string]string{"field": field, "reason": reason}
}

// GamePatch 编辑基础信息的列级补丁（nil 表示不改）。
type GamePatch struct {
	Name              *string
	Alias             *string
	IconURL           *string
	Status            *string
	DefaultMarketCode *string // 非空时同步回写 game_markets.is_default
}

// GameRepository 窄仓储：单聚合 CRUD + compact 列出的必要查询。
// SQL 不写 schema 前缀、不带 env 谓词；env 由连接固定的 search_path 决定（01 §4.2/§4.4）。
type GameRepository interface {
	// NextGameIDSeq 返回当前 schema 内 game_id 的最大数字序号（空表返回 0）。
	NextGameIDSeq(ctx context.Context) (int64, error)
	// ExistsAlias 判断 alias 是否已存在；excludeGameID 非空时排除该游戏（变更查重）。
	ExistsAlias(ctx context.Context, alias, excludeGameID string) (bool, error)
	// InsertGame 同事务插入 games + 其市场集合，返回装配后的聚合（含 ID/时间戳）。
	InsertGame(ctx context.Context, g domaingame.Game) (domaingame.Game, error)
	// GetGameByGameID 按对外 game_id 装配完整聚合（games + markets + legalLinks）。
	GetGameByGameID(ctx context.Context, gameID string) (domaingame.Game, error)
	// ListGames 分页列表（每项含市场集合用于摘要）。
	ListGames(ctx context.Context, q dto.ListGamesQuery) ([]domaingame.Game, int, error)
	// UpdateGame 更新 games 列级字段；patch.DefaultMarketCode 非空时同步 game_markets.is_default。
	UpdateGame(ctx context.Context, gameID string, patch GamePatch) error
	// ReplaceMarkets 全量覆盖市场并回写 games.default_market_code（事务内）。
	ReplaceMarkets(ctx context.Context, gameID string, markets []domaingame.GameMarket, defaultMarketCode string) error
	// ReplaceLegalLinks 全量覆盖法务链接（事务内）。
	ReplaceLegalLinks(ctx context.Context, gameID string, links []domaingame.GameLegalLink) error
	// CountChannelsByMarket 统计某 market 下的渠道实例数（删除保护，跨模块只读，同 schema）。
	CountChannelsByMarket(ctx context.Context, gameID, marketCode string) (int, error)
}

// Repositories 一组仓储句柄（绑定到 pool 或某事务连接）。
type Repositories struct {
	Games GameRepository
}

// TxManager 提供事务边界，跨聚合写编排在 app 层用 InTx 包裹（01 §4.2）。
type TxManager interface {
	Repositories() Repositories
	InTx(ctx context.Context, fn func(Repositories) error) error
}

// AuditSink / AuditEntry 复用 auth 应用层端口，保持审计写入一致（00 §8）。
type AuditSink = adminapp.AuditSink

// AuditEntry 审计记录别名。
type AuditEntry = adminapp.AuditEntry
