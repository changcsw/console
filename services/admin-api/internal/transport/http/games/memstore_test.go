package games

import (
	"context"
	"sort"
	"strconv"
	"strings"
	"time"

	adminapp "github.com/csw/console/services/admin-api/internal/app/admin"
	"github.com/csw/console/services/admin-api/internal/app/dto"
	gameapp "github.com/csw/console/services/admin-api/internal/app/game"
	"github.com/csw/console/services/admin-api/internal/domain/common"
	domaingame "github.com/csw/console/services/admin-api/internal/domain/game"
)

// memGameState 是 game 模块的内存数据快照，仅用于进程内 httptest 全链路覆盖
// （transport -> app -> domain），不依赖真实 PG。InTx 通过克隆/回填实现真实回滚语义，
// 等价覆盖 S10（多表写中途失败整体回滚）。
type memGameState struct {
	games map[string]*domaingame.Game // 按对外 game_id 键
	seqID int64                       // 内部主键自增

	// channelCounts: gameID -> marketCode -> 渠道实例数（删除保护用；channel 未落地默认 0）。
	channelCounts map[string]map[string]int
}

func newMemGameState() *memGameState {
	return &memGameState{
		games:         map[string]*domaingame.Game{},
		channelCounts: map[string]map[string]int{},
	}
}

func cloneGame(g *domaingame.Game) *domaingame.Game {
	cp := *g
	cp.Markets = append([]domaingame.GameMarket(nil), g.Markets...)
	cp.LegalLinks = append([]domaingame.GameLegalLink(nil), g.LegalLinks...)
	return &cp
}

func (s *memGameState) clone() *memGameState {
	c := newMemGameState()
	for k, v := range s.games {
		c.games[k] = cloneGame(v)
	}
	for gid, byMarket := range s.channelCounts {
		m := map[string]int{}
		for mc, n := range byMarket {
			m[mc] = n
		}
		c.channelCounts[gid] = m
	}
	c.seqID = s.seqID
	return c
}

// memStore 实现 gameapp.TxManager。
type memStore struct{ state *memGameState }

func newMemStore() *memStore { return &memStore{state: newMemGameState()} }

func (s *memStore) Repositories() gameapp.Repositories {
	return gameapp.Repositories{Games: &memGameRepo{s.state}}
}

func (s *memStore) InTx(ctx context.Context, fn func(gameapp.Repositories) error) error {
	clone := s.state.clone()
	if err := fn(gameapp.Repositories{Games: &memGameRepo{clone}}); err != nil {
		return err // 丢弃 clone = 回滚
	}
	s.state = clone
	return nil
}

// memGameRepo 实现 gameapp.GameRepository。
type memGameRepo struct{ st *memGameState }

func (r *memGameRepo) NextGameIDSeq(_ context.Context) (int64, error) {
	var max int64
	for gid := range r.st.games {
		if n, err := strconv.ParseInt(gid, 10, 64); err == nil && n > max {
			max = n
		}
	}
	return max, nil
}

func (r *memGameRepo) ExistsAlias(_ context.Context, alias, excludeGameID string) (bool, error) {
	for gid, g := range r.st.games {
		if g.Alias == alias && gid != excludeGameID {
			return true, nil
		}
	}
	return false, nil
}

func (r *memGameRepo) InsertGame(_ context.Context, g domaingame.Game) (domaingame.Game, error) {
	r.st.seqID++
	g.ID = r.st.seqID
	now := time.Now()
	g.CreatedAt, g.UpdatedAt = now, now
	r.st.games[g.GameID] = cloneGame(&g)
	return *cloneGame(&g), nil
}

func (r *memGameRepo) GetGameByGameID(_ context.Context, gameID string) (domaingame.Game, error) {
	g, ok := r.st.games[gameID]
	if !ok {
		return domaingame.Game{}, adminapp.ErrNotFound
	}
	return *cloneGame(g), nil
}

func (r *memGameRepo) ListGames(_ context.Context, q dto.ListGamesQuery) ([]domaingame.Game, int, error) {
	all := make([]*domaingame.Game, 0, len(r.st.games))
	for _, g := range r.st.games {
		if q.Keyword != "" {
			kw := strings.ToLower(q.Keyword)
			if !strings.Contains(strings.ToLower(g.Name), kw) &&
				!strings.Contains(strings.ToLower(g.Alias), kw) &&
				!strings.Contains(strings.ToLower(g.GameID), kw) {
				continue
			}
		}
		if q.Status != "" && string(g.Status) != q.Status {
			continue
		}
		if q.MarketCode != "" && !hasEnabledMarket(g.Markets, q.MarketCode) {
			continue
		}
		all = append(all, g)
	}
	// 默认 -updatedAt；同时间退化为 gameId 稳定序，保证分页可重复。
	sort.Slice(all, func(i, j int) bool {
		if !all[i].UpdatedAt.Equal(all[j].UpdatedAt) {
			return all[i].UpdatedAt.After(all[j].UpdatedAt)
		}
		return all[i].GameID < all[j].GameID
	})
	total := len(all)

	page, pageSize := q.Page, q.PageSize
	if page < 1 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = 20
	}
	start := (page - 1) * pageSize
	if start > total {
		start = total
	}
	end := start + pageSize
	if end > total {
		end = total
	}
	out := make([]domaingame.Game, 0, end-start)
	for _, g := range all[start:end] {
		out = append(out, *cloneGame(g))
	}
	return out, total, nil
}

func (r *memGameRepo) UpdateGame(_ context.Context, gameID string, patch gameapp.GamePatch) error {
	g, ok := r.st.games[gameID]
	if !ok {
		return adminapp.ErrNotFound
	}
	if patch.Name != nil {
		g.Name = *patch.Name
	}
	if patch.Alias != nil {
		g.Alias = *patch.Alias
	}
	if patch.IconURL != nil {
		g.IconURL = *patch.IconURL
	}
	if patch.Status != nil {
		g.Status = common.GameStatus(*patch.Status)
	}
	if patch.DefaultMarketCode != nil {
		dmc := *patch.DefaultMarketCode
		g.DefaultMarketCode = dmc
		for i := range g.Markets {
			g.Markets[i].IsDefault = g.Markets[i].MarketCode == dmc
		}
	}
	g.UpdatedAt = time.Now()
	return nil
}

func (r *memGameRepo) ReplaceMarkets(_ context.Context, gameID string, markets []domaingame.GameMarket, defaultMarketCode string) error {
	g, ok := r.st.games[gameID]
	if !ok {
		return adminapp.ErrNotFound
	}
	g.Markets = append([]domaingame.GameMarket(nil), markets...)
	g.DefaultMarketCode = defaultMarketCode
	g.UpdatedAt = time.Now()
	return nil
}

func (r *memGameRepo) ReplaceLegalLinks(_ context.Context, gameID string, links []domaingame.GameLegalLink) error {
	g, ok := r.st.games[gameID]
	if !ok {
		return adminapp.ErrNotFound
	}
	g.LegalLinks = append([]domaingame.GameLegalLink(nil), links...)
	g.UpdatedAt = time.Now()
	return nil
}

func (r *memGameRepo) CountChannelsByMarket(_ context.Context, gameID, marketCode string) (int, error) {
	if byMarket, ok := r.st.channelCounts[gameID]; ok {
		return byMarket[marketCode], nil
	}
	return 0, nil
}

func hasEnabledMarket(markets []domaingame.GameMarket, code string) bool {
	for _, m := range markets {
		if m.MarketCode == code && m.Enabled {
			return true
		}
	}
	return false
}

// fakeAudit 记录审计调用，供 S7/S8 断言。
type fakeAudit struct{ entries []gameapp.AuditEntry }

func (a *fakeAudit) Write(_ context.Context, e gameapp.AuditEntry) { a.entries = append(a.entries, e) }

func (a *fakeAudit) byAction(action string) (gameapp.AuditEntry, bool) {
	for _, e := range a.entries {
		if e.Action == action {
			return e, true
		}
	}
	return gameapp.AuditEntry{}, false
}
