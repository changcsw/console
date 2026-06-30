package channels

import (
	"context"

	adminapp "github.com/csw/console/services/admin-api/internal/app/admin"
	channelloginapp "github.com/csw/console/services/admin-api/internal/app/channellogin"
	"github.com/csw/console/services/admin-api/internal/domain/channel"
)

// memLoginStore 实现 channelloginapp.TxManager（内存仓储，进程内 L3 用）。
// 单表写（game_channel_login_configs）：InTx 在 configs 快照上执行 fn，失败整体回滚（验 S10）。
type memLoginStore struct{ state *memLoginState }

type memLoginState struct {
	instances map[int64]channel.GameMarketChannel    // gameChannelID -> 实例
	policies  map[string]channel.ChannelPolicy       // channelID(string) -> 策略
	templates map[int64]channel.ChannelLoginTemplate // channelIDRef -> enabled 最新模板
	configs   map[int64]*channel.ChannelLoginConfig  // gameChannelID -> 配置
	seq       int64
	// 失败注入：S10 回滚 / S5 冲突。
	failUpsert   bool
	failConflict bool
	upsertCalls  int
}

func newMemLoginStore() *memLoginStore {
	return &memLoginStore{state: &memLoginState{
		instances: map[int64]channel.GameMarketChannel{},
		policies:  map[string]channel.ChannelPolicy{},
		templates: map[int64]channel.ChannelLoginTemplate{},
		configs:   map[int64]*channel.ChannelLoginConfig{},
	}}
}

func (s *memLoginStore) repos(st *memLoginState) channelloginapp.Repositories {
	return channelloginapp.Repositories{
		GameChannels: &memLoginGameChannels{st: st},
		Policies:     &memLoginPolicies{st: st},
		Templates:    &memLoginTemplates{st: st},
		Configs:      &memLoginConfigs{st: st},
	}
}

func (s *memLoginStore) Repositories() channelloginapp.Repositories { return s.repos(s.state) }

func (s *memLoginStore) InTx(ctx context.Context, fn func(channelloginapp.Repositories) error) error {
	// 快照 configs（深拷贝），失败回滚。
	snapshot := make(map[int64]*channel.ChannelLoginConfig, len(s.state.configs))
	for k, v := range s.state.configs {
		cp := *v
		cp.ConfigJSON = cloneAnyMapTest(v.ConfigJSON)
		snapshot[k] = &cp
	}
	if err := fn(s.repos(s.state)); err != nil {
		s.state.configs = snapshot
		return err
	}
	return nil
}

// ───────────────────────── 仓储实现 ─────────────────────────

type memLoginGameChannels struct{ st *memLoginState }

func (r *memLoginGameChannels) GetByID(_ context.Context, id int64) (channel.GameMarketChannel, error) {
	inst, ok := r.st.instances[id]
	if !ok {
		return channel.GameMarketChannel{}, adminapp.ErrNotFound
	}
	return inst, nil
}

type memLoginPolicies struct{ st *memLoginState }

func (r *memLoginPolicies) GetByChannelID(_ context.Context, channelID string) (channel.ChannelPolicy, error) {
	p, ok := r.st.policies[channelID]
	if !ok {
		return channel.ChannelPolicy{}, adminapp.ErrNotFound
	}
	return p, nil
}

type memLoginTemplates struct{ st *memLoginState }

func (r *memLoginTemplates) GetPublishedByChannel(_ context.Context, channelIDRef int64) (*channel.ChannelLoginTemplate, error) {
	tpl, ok := r.st.templates[channelIDRef]
	if !ok || !tpl.Enabled {
		return nil, nil // 无模板：service 映射为 400 VALIDATION_FAILED（无模板）
	}
	cp := tpl
	return &cp, nil
}

func (r *memLoginTemplates) GetByChannelVersion(_ context.Context, channelIDRef int64, version string) (*channel.ChannelLoginTemplate, error) {
	tpl, ok := r.st.templates[channelIDRef]
	if !ok || tpl.TemplateVersion != version {
		return nil, nil
	}
	cp := tpl
	return &cp, nil
}

type memLoginConfigs struct{ st *memLoginState }

func (r *memLoginConfigs) GetByGameChannel(_ context.Context, gameChannelID int64) (*channel.ChannelLoginConfig, error) {
	cfg, ok := r.st.configs[gameChannelID]
	if !ok {
		return nil, nil
	}
	cp := *cfg
	cp.ConfigJSON = cloneAnyMapTest(cfg.ConfigJSON)
	return &cp, nil
}

func (r *memLoginConfigs) Upsert(_ context.Context, cfg *channel.ChannelLoginConfig) error {
	r.st.upsertCalls++
	if r.st.failConflict {
		return adminapp.ErrConflict
	}
	if r.st.failUpsert {
		return context.DeadlineExceeded // 任意非哨兵错误 → 映射 500
	}
	stored := *cfg
	stored.ConfigJSON = cloneAnyMapTest(cfg.ConfigJSON)
	if stored.ID == 0 {
		r.st.seq++
		stored.ID = r.st.seq
		cfg.ID = stored.ID
	}
	r.st.configs[cfg.GameChannelIDRef] = &stored
	return nil
}

// ───────────────────────── seed helpers ─────────────────────────

func (st *memLoginState) seedInstance(id, channelIDRef int64, channelID, market, copiedFrom string) {
	st.instances[id] = channel.GameMarketChannel{
		ID:               id,
		GameID:           "100001",
		Market:           market,
		ChannelIDRef:     channelIDRef,
		ChannelID:        channelID,
		CopiedFromMarket: copiedFrom,
	}
	if id > st.seq {
		st.seq = id
	}
}

func cloneAnyMapTest(in map[string]any) map[string]any {
	if in == nil {
		return map[string]any{}
	}
	out := make(map[string]any, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}
