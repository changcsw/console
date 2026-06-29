package game

import (
	"context"
	"errors"
	"io"
	"strings"

	adminapp "github.com/csw/console/services/admin-api/internal/app/admin"
	"github.com/csw/console/services/admin-api/internal/app/dto"
	"github.com/csw/console/services/admin-api/internal/domain/common"
	domaingame "github.com/csw/console/services/admin-api/internal/domain/game"
)

const maskedSecret = "masked"

// GameService 游戏主数据的读/写用例。
// 依赖 TxManager（仓储+事务）/ 随机源（game_secret）/ AuditSink / 运行环境 env。
// ctx 携带按当前 env 钉死 search_path 的连接，env 不作显式写入参数（01 §4.4）。
type GameService struct {
	tx    TxManager
	rand  io.Reader
	audit AuditSink
	env   common.Environment
}

// NewGameService 构造服务。
func NewGameService(tx TxManager, rand io.Reader, audit AuditSink, env common.Environment) *GameService {
	return &GameService{tx: tx, rand: rand, audit: audit, env: env}
}

// ===== query =====

// ListGames 游戏分页列表（轻量摘要，不含 gameSecret）。
func (s *GameService) ListGames(ctx context.Context, q dto.ListGamesQuery) (dto.Page[dto.GameListItem], error) {
	if len(q.Keyword) > 64 {
		return dto.Page[dto.GameListItem]{}, validationErr("keyword 长度不能超过 64", fieldDetail("keyword", "maxLen 64"))
	}
	if q.Status != "" && !domaingame.IsValidGameStatus(common.GameStatus(q.Status)) {
		return dto.Page[dto.GameListItem]{}, validationErr("status 非法", fieldDetail("status", "enum"))
	}
	if q.MarketCode != "" && !domaingame.IsValidMarket(q.MarketCode) {
		return dto.Page[dto.GameListItem]{}, validationErr("marketCode 非法", fieldDetail("marketCode", "enum"))
	}

	page, pageSize := normalizePage(q.Page, q.PageSize)
	q.Page, q.PageSize = page, pageSize

	games, total, err := s.tx.Repositories().Games.ListGames(ctx, q)
	if err != nil {
		return dto.Page[dto.GameListItem]{}, err
	}
	items := make([]dto.GameListItem, 0, len(games))
	for i := range games {
		items = append(items, toListItem(games[i]))
	}
	return dto.Page[dto.GameListItem]{Items: items, Page: page, PageSize: pageSize, Total: total}, nil
}

// GetGame 游戏详情（完整聚合，gameSecret 脱敏）。
func (s *GameService) GetGame(ctx context.Context, gameID string) (dto.GameDetail, error) {
	g, err := s.load(ctx, gameID)
	if err != nil {
		return dto.GameDetail{}, err
	}
	return s.toDetail(g, true), nil
}

// ===== command =====

// CreateGame 创建游戏：校验 → 生成 gameId/gameSecret → 落当前 schema（games + 默认市场）→ 写审计。
// 仅创建响应一次性返回明文 gameSecret（SecretMasked=false）。
func (s *GameService) CreateGame(ctx context.Context, cmd dto.CreateGameCmd) (dto.GameDetail, error) {
	name := strings.TrimSpace(cmd.Name)
	if name == "" || len(name) > 128 {
		return dto.GameDetail{}, validationErr("name 必填且不超过 128", fieldDetail("name", "1-128"))
	}
	alias := strings.TrimSpace(cmd.Alias)
	if !domaingame.IsValidAlias(alias) {
		return dto.GameDetail{}, validationErr("alias 非法", fieldDetail("alias", "1-64, ^[a-zA-Z0-9_-]+$"))
	}
	if len(cmd.IconURL) > 512 || !domaingame.IsValidOptionalURL(cmd.IconURL) {
		return dto.GameDetail{}, validationErr("iconUrl 非法", fieldDetail("iconUrl", "maxLen 512, url"))
	}
	dmc := cmd.DefaultMarketCode
	if dmc != "" && !domaingame.IsValidMarket(dmc) {
		return dto.GameDetail{}, validationErr("defaultMarketCode 非法", fieldDetail("defaultMarketCode", "enum"))
	}
	status := common.GameStatus(cmd.Status)
	if cmd.Status == "" {
		status = common.GameStatusDraft
	} else if !domaingame.IsValidGameStatus(status) {
		return dto.GameDetail{}, validationErr("status 非法", fieldDetail("status", "enum"))
	}
	for _, mc := range cmd.Markets {
		if !domaingame.IsValidMarket(mc) {
			return dto.GameDetail{}, validationErr("markets 含非法 market", fieldDetail("markets", "enum"))
		}
	}

	markets, resolvedDMC := domaingame.ApplyDefaultMarket(cmd.Markets, dmc)

	var created domaingame.Game
	err := s.tx.InTx(ctx, func(repos Repositories) error {
		exists, err := repos.Games.ExistsAlias(ctx, alias, "")
		if err != nil {
			return err
		}
		if exists {
			return conflictErr("alias already exists")
		}
		seq, err := repos.Games.NextGameIDSeq(ctx)
		if err != nil {
			return err
		}
		gameID := domaingame.GenerateGameID(s.env, seq)
		secret, err := domaingame.GenerateGameSecret(s.rand)
		if err != nil {
			return err
		}
		g := domaingame.Game{
			GameID:            gameID,
			GameSecret:        secret,
			Name:              name,
			Alias:             alias,
			IconURL:           cmd.IconURL,
			DefaultMarketCode: resolvedDMC,
			Status:            status,
			Markets:           markets,
		}
		saved, err := repos.Games.InsertGame(ctx, g)
		if err != nil {
			return err
		}
		created = saved
		return nil
	})
	if err != nil {
		return dto.GameDetail{}, mapWriteErr(err, "alias already exists")
	}

	s.writeAudit(ctx, "game.create", created.GameID, map[string]any{
		"name": name, "alias": alias, "defaultMarketCode": resolvedDMC,
	})
	return s.toDetail(created, false), nil
}

// UpdateGame 编辑基础信息（部分更新；不可改 gameId/gameSecret，不接受切 env）。
func (s *GameService) UpdateGame(ctx context.Context, cmd dto.UpdateGameCmd) (dto.GameDetail, error) {
	current, err := s.load(ctx, cmd.GameID)
	if err != nil {
		return dto.GameDetail{}, err
	}

	patch := GamePatch{}
	if cmd.Name != nil {
		name := strings.TrimSpace(*cmd.Name)
		if name == "" || len(name) > 128 {
			return dto.GameDetail{}, validationErr("name 必填且不超过 128", fieldDetail("name", "1-128"))
		}
		patch.Name = &name
	}
	if cmd.Alias != nil {
		alias := strings.TrimSpace(*cmd.Alias)
		if !domaingame.IsValidAlias(alias) {
			return dto.GameDetail{}, validationErr("alias 非法", fieldDetail("alias", "1-64, ^[a-zA-Z0-9_-]+$"))
		}
		if alias != current.Alias {
			exists, err := s.tx.Repositories().Games.ExistsAlias(ctx, alias, cmd.GameID)
			if err != nil {
				return dto.GameDetail{}, err
			}
			if exists {
				return dto.GameDetail{}, conflictErr("alias already exists")
			}
		}
		patch.Alias = &alias
	}
	if cmd.IconURL != nil {
		if len(*cmd.IconURL) > 512 || !domaingame.IsValidOptionalURL(*cmd.IconURL) {
			return dto.GameDetail{}, validationErr("iconUrl 非法", fieldDetail("iconUrl", "maxLen 512, url"))
		}
		patch.IconURL = cmd.IconURL
	}
	if cmd.Status != nil {
		st := common.GameStatus(*cmd.Status)
		if !domaingame.IsValidGameStatus(st) {
			return dto.GameDetail{}, validationErr("status 非法", fieldDetail("status", "enum"))
		}
		patch.Status = cmd.Status
	}
	if cmd.DefaultMarketCode != nil {
		dmc := *cmd.DefaultMarketCode
		if !domaingame.IsValidMarket(dmc) {
			return dto.GameDetail{}, validationErr("defaultMarketCode 非法", fieldDetail("defaultMarketCode", "enum"))
		}
		if !isEnabledMarket(current.Markets, dmc) {
			return dto.GameDetail{}, validationErr("defaultMarketCode must be an enabled market", fieldDetail("defaultMarketCode", "must be an enabled market"))
		}
		patch.DefaultMarketCode = &dmc
	}

	// games 列更新 + default_market_code 同步回写 game_markets.is_default 必须原子（单事务）。
	if err := s.tx.InTx(ctx, func(repos Repositories) error {
		return repos.Games.UpdateGame(ctx, cmd.GameID, patch)
	}); err != nil {
		return dto.GameDetail{}, mapWriteErr(err, "alias already exists")
	}

	s.writeAudit(ctx, "game.update", cmd.GameID, map[string]any{"fields": changedFields(patch)})
	return s.GetGame(ctx, cmd.GameID)
}

// ReplaceMarkets 全量覆盖市场：校验自洽 → 删除保护 → 落库并回写 default_market_code → 写审计。
func (s *GameService) ReplaceMarkets(ctx context.Context, cmd dto.ReplaceMarketsCmd) (dto.GameDetail, error) {
	current, err := s.load(ctx, cmd.GameID)
	if err != nil {
		return dto.GameDetail{}, err
	}

	markets := make([]domaingame.GameMarket, 0, len(cmd.Markets))
	defaultCode := ""
	for _, m := range cmd.Markets {
		if !domaingame.IsValidMarket(m.MarketCode) {
			return dto.GameDetail{}, validationErr("markets 含非法 market", fieldDetail("markets.marketCode", "enum"))
		}
		locale := m.DefaultLocale
		if locale == "" {
			locale = "en-US"
		}
		if !domaingame.IsValidLocale(locale) {
			return dto.GameDetail{}, validationErr("defaultLocale 非法", fieldDetail("markets.defaultLocale", "locale format"))
		}
		if m.IsDefault {
			defaultCode = m.MarketCode
		}
		markets = append(markets, domaingame.GameMarket{
			MarketCode:    m.MarketCode,
			IsDefault:     m.IsDefault,
			Enabled:       m.Enabled,
			DefaultLocale: locale,
		})
	}
	if err := domaingame.ValidateMarkets(markets, defaultCode); err != nil {
		return dto.GameDetail{}, marketRuleErr(err)
	}

	// 删除保护 + 全量覆盖 + 回写 default_market_code 必须原子（单事务，避免 diff/校验与写入之间的 TOCTOU）。
	// channel 未落地前 CountChannelsByMarket 恒 0，删除保护降级为「不可移除当前默认市场」。
	newCodes := map[string]bool{}
	for _, m := range markets {
		newCodes[m.MarketCode] = true
	}
	if err := s.tx.InTx(ctx, func(repos Repositories) error {
		for _, cur := range current.Markets {
			if newCodes[cur.MarketCode] {
				continue
			}
			cnt, err := repos.Games.CountChannelsByMarket(ctx, cmd.GameID, cur.MarketCode)
			if err != nil {
				return err
			}
			if cnt > 0 {
				return conflictErr("cannot remove market with existing channels")
			}
			if cur.IsDefault {
				return conflictErr("cannot remove the default market")
			}
		}
		return repos.Games.ReplaceMarkets(ctx, cmd.GameID, markets, defaultCode)
	}); err != nil {
		return dto.GameDetail{}, mapWriteErr(err, "market conflict")
	}

	s.writeAudit(ctx, "game.markets.update", cmd.GameID, map[string]any{"defaultMarketCode": defaultCode, "count": len(markets)})
	return s.GetGame(ctx, cmd.GameID)
}

// ReplaceLegalLinks 全量覆盖法务链接：逐项校验 scope → 去重 → 落库 → 写审计。
func (s *GameService) ReplaceLegalLinks(ctx context.Context, cmd dto.ReplaceLegalLinksCmd) (dto.GameDetail, error) {
	if _, err := s.load(ctx, cmd.GameID); err != nil {
		return dto.GameDetail{}, err
	}

	links := make([]domaingame.GameLegalLink, 0, len(cmd.LegalLinks))
	seen := map[string]bool{}
	defaultCount := 0
	for _, l := range cmd.LegalLinks {
		normalized, err := domaingame.ValidateLegalScope(l.ScopeType, l.ScopeValue)
		if err != nil {
			return dto.GameDetail{}, legalScopeErr(err)
		}
		if l.ScopeType == string(common.LegalScopeDefault) {
			defaultCount++
			if defaultCount > 1 {
				return dto.GameDetail{}, conflictErr("duplicate legal link scope")
			}
		}
		for _, u := range []struct{ field, val string }{
			{"termsUrl", l.TermsURL}, {"privacyUrl", l.PrivacyURL}, {"deleteAccountUrl", l.DeleteAccountURL},
		} {
			if len(u.val) > 512 || !domaingame.IsValidOptionalURL(u.val) {
				return dto.GameDetail{}, validationErr(u.field+" 非法", fieldDetail("legalLinks."+u.field, "maxLen 512, url"))
			}
		}
		key := l.ScopeType + "|" + normalized
		if seen[key] {
			return dto.GameDetail{}, conflictErr("duplicate legal link scope")
		}
		seen[key] = true
		links = append(links, domaingame.GameLegalLink{
			ScopeType:        l.ScopeType,
			ScopeValue:       normalized,
			TermsURL:         l.TermsURL,
			PrivacyURL:       l.PrivacyURL,
			DeleteAccountURL: l.DeleteAccountURL,
		})
	}

	// 全量覆盖（delete + insert）必须原子（单事务）。
	if err := s.tx.InTx(ctx, func(repos Repositories) error {
		return repos.Games.ReplaceLegalLinks(ctx, cmd.GameID, links)
	}); err != nil {
		return dto.GameDetail{}, mapWriteErr(err, "duplicate legal link scope")
	}

	s.writeAudit(ctx, "game.legal.update", cmd.GameID, map[string]any{"count": len(links)})
	return s.GetGame(ctx, cmd.GameID)
}

// ===== helpers =====

func (s *GameService) load(ctx context.Context, gameID string) (domaingame.Game, error) {
	g, err := s.tx.Repositories().Games.GetGameByGameID(ctx, gameID)
	if err != nil {
		if errors.Is(err, adminapp.ErrNotFound) {
			return domaingame.Game{}, notFoundErr("game not found")
		}
		return domaingame.Game{}, err
	}
	return g, nil
}

func (s *GameService) writeAudit(ctx context.Context, action, gameID string, detail map[string]any) {
	if s.audit == nil {
		return
	}
	actor := int64(0)
	if ac, ok := adminapp.AuthContextFrom(ctx); ok {
		actor = ac.UserID
	}
	s.audit.Write(ctx, AuditEntry{ActorID: actor, Action: action, ResourceType: "game", ResourceID: gameID, Detail: detail})
}

func (s *GameService) toDetail(g domaingame.Game, masked bool) dto.GameDetail {
	secret := g.GameSecret
	if masked {
		secret = maskedSecret
	}
	d := dto.GameDetail{
		GameID:            g.GameID,
		Name:              g.Name,
		Alias:             g.Alias,
		IconURL:           g.IconURL,
		Status:            string(g.Status),
		DefaultMarketCode: g.DefaultMarketCode,
		GameSecret:        secret,
		SecretMasked:      masked,
		Environment:       string(s.env),
		Markets:           toMarketViews(g.Markets),
		LegalLinks:        toLegalViews(g.LegalLinks),
		CreatedAt:         g.CreatedAt,
		UpdatedAt:         g.UpdatedAt,
	}
	return d
}

func toMarketViews(markets []domaingame.GameMarket) []dto.GameMarketView {
	out := make([]dto.GameMarketView, 0, len(markets))
	for _, m := range markets {
		out = append(out, dto.GameMarketView{
			MarketCode: m.MarketCode, IsDefault: m.IsDefault, Enabled: m.Enabled, DefaultLocale: m.DefaultLocale,
		})
	}
	return out
}

func toLegalViews(links []domaingame.GameLegalLink) []dto.GameLegalLinkView {
	out := make([]dto.GameLegalLinkView, 0, len(links))
	for _, l := range links {
		out = append(out, dto.GameLegalLinkView{
			ScopeType: l.ScopeType, ScopeValue: l.ScopeValue,
			TermsURL: l.TermsURL, PrivacyURL: l.PrivacyURL, DeleteAccountURL: l.DeleteAccountURL,
		})
	}
	return out
}

func toListItem(g domaingame.Game) dto.GameListItem {
	codes := make([]string, 0, len(g.Markets))
	for _, m := range g.Markets {
		codes = append(codes, m.MarketCode)
	}
	return dto.GameListItem{
		GameID:            g.GameID,
		Name:              g.Name,
		Alias:             g.Alias,
		IconURL:           g.IconURL,
		Status:            string(g.Status),
		DefaultMarketCode: g.DefaultMarketCode,
		MarketCodes:       codes,
		MarketCount:       len(codes),
		CreatedAt:         g.CreatedAt,
		UpdatedAt:         g.UpdatedAt,
	}
}

func isEnabledMarket(markets []domaingame.GameMarket, code string) bool {
	for _, m := range markets {
		if m.MarketCode == code {
			return m.Enabled
		}
	}
	return false
}

func changedFields(p GamePatch) []string {
	fields := []string{}
	if p.Name != nil {
		fields = append(fields, "name")
	}
	if p.Alias != nil {
		fields = append(fields, "alias")
	}
	if p.IconURL != nil {
		fields = append(fields, "iconUrl")
	}
	if p.Status != nil {
		fields = append(fields, "status")
	}
	if p.DefaultMarketCode != nil {
		fields = append(fields, "defaultMarketCode")
	}
	return fields
}

// normalizePage 归一化分页（00 §7.3：page>=1，pageSize 默认 20、最大 100）。
func normalizePage(page, pageSize int) (int, int) {
	if page < 1 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = 20
	}
	if pageSize > 100 {
		pageSize = 100
	}
	return page, pageSize
}

// mapWriteErr 把已是 *Error 的错误透传；DB 唯一冲突（adminapp.ErrConflict）映射为指定冲突消息；其它原样返回。
func mapWriteErr(err error, conflictMsg string) error {
	var appErr *Error
	if errors.As(err, &appErr) {
		return appErr
	}
	if errors.Is(err, adminapp.ErrConflict) {
		return conflictErr(conflictMsg)
	}
	if errors.Is(err, adminapp.ErrNotFound) {
		return notFoundErr("game not found")
	}
	return err
}

func marketRuleErr(err error) *Error {
	switch {
	case errors.Is(err, domaingame.ErrNotExactlyOneDefault):
		return validationErr("exactly one default market is required", fieldDetail("markets", "exactly one default"))
	case errors.Is(err, domaingame.ErrDefaultMarketNotEnabled):
		return validationErr("default market must be enabled", fieldDetail("markets", "default must be enabled"))
	case errors.Is(err, domaingame.ErrEmptyMarkets):
		return validationErr("markets 不能为空", fieldDetail("markets", "non-empty"))
	case errors.Is(err, domaingame.ErrDuplicateMarket):
		return validationErr("markets 含重复 market", fieldDetail("markets", "duplicate"))
	case errors.Is(err, domaingame.ErrInvalidMarket):
		return validationErr("markets 含非法 market", fieldDetail("markets", "enum"))
	default:
		return validationErr("markets 非法", fieldDetail("markets", "invalid"))
	}
}

func legalScopeErr(err error) *Error {
	switch {
	case errors.Is(err, domaingame.ErrInvalidScopeType):
		return validationErr("scopeType 非法", fieldDetail("legalLinks.scopeType", "default/market/locale"))
	case errors.Is(err, domaingame.ErrInvalidScopeValue):
		return validationErr("scopeValue 非法", fieldDetail("legalLinks.scopeValue", "by scopeType"))
	default:
		return validationErr("legalLinks 非法", fieldDetail("legalLinks", "invalid"))
	}
}
