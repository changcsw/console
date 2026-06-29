package games

import (
	"context"
	"errors"
	"sort"

	accountauthapp "github.com/csw/console/services/admin-api/internal/app/accountauth"
	adminapp "github.com/csw/console/services/admin-api/internal/app/admin"
	domainaa "github.com/csw/console/services/admin-api/internal/domain/accountauth"
)

// account-auth 模块（13）进程内 L3 httptest 的内存装配：
// 实现 accountauthapp.TxManager + Repository（内存仓储）、spy Cipher（真实"加密语义"+调用计数）。
// InTx 通过克隆/回填实现真实回滚语义，等价覆盖 S10（整体替换中途失败 → 整体回滚）。
// 平台目录/渠道允许集合/游戏配置均在内存内 seed，不依赖真实 PG（与 games memStore 同口径）。

// ───────────────────────── 模板工厂 ─────────────────────────

func emptyTemplate() domainaa.Template {
	return domainaa.Template{TemplateVersion: "v1"}
}

// googleTemplate 与 migrations/000006 seed 的 google 模板一致：
// secretFields=[clientSecret]，clientId/clientSecret/redirectUri 必填，clientId.minLen=1，redirectUri.format=url。
func googleTemplate() domainaa.Template {
	minOne := 1
	return domainaa.Template{
		TemplateVersion: "v1",
		FormSchema: []domainaa.FormField{
			{Key: "clientId", Label: "Client ID", Component: "input", Required: true, Order: 10, Scope: "both"},
			{Key: "clientSecret", Label: "Client Secret", Component: "password", Required: true, Order: 20, Scope: "server"},
			{Key: "redirectUri", Label: "Redirect URI", Component: "input", Required: true, Order: 30, Scope: "both"},
		},
		SecretFields: []string{"clientSecret"},
		ValidationRules: map[string]domainaa.ValidationRule{
			"clientId":    {MinLen: &minOne},
			"redirectUri": {Format: "url"},
		},
	}
}

// auth_type_id_ref 常量（与 seed 同源 sort 顺序，仅测试内引用）。
const (
	refGuest  int64 = 1
	refPhone  int64 = 2
	refGoogle int64 = 4
	refApple  int64 = 5
	refLine   int64 = 7
)

// ───────────────────────── 内存状态 ─────────────────────────

type aaState struct {
	types       []accountauthapp.TypeCatalogItem
	channelPol  map[string][]accountauthapp.ChannelTypePolicy
	gameRowID   map[string]int64                                  // 对外 gameID -> 内部 rowID
	allowed     map[int64][]accountauthapp.GameAllowedType        // rowID -> 允许集合
	configs     map[int64]map[int64]accountauthapp.GameConfigItem // rowID -> authTypeIDRef -> 配置
	failReplace bool                                              // S10：强制 ReplaceGameConfigs 失败
}

func newAAState() *aaState {
	return &aaState{
		types: []accountauthapp.TypeCatalogItem{
			{AuthTypeIDRef: refGuest, AuthTypeID: "guest", AuthTypeName: "游客", Enabled: true, Sort: 10, Template: emptyTemplate()},
			{AuthTypeIDRef: refPhone, AuthTypeID: "phone", AuthTypeName: "手机号", Enabled: true, Sort: 20, Template: emptyTemplate()},
			{AuthTypeIDRef: refGoogle, AuthTypeID: "google", AuthTypeName: "Google 登录", Enabled: true, Sort: 40, Template: googleTemplate()},
		},
		channelPol: map[string][]accountauthapp.ChannelTypePolicy{
			"ch_account": {
				{AuthTypeID: "guest", DefaultEnabled: true, Locked: false},
				{AuthTypeID: "google", DefaultEnabled: false, Locked: false},
				{AuthTypeID: "line", DefaultEnabled: false, Locked: true},
			},
		},
		gameRowID: map[string]int64{"100001": 1},
		allowed: map[int64][]accountauthapp.GameAllowedType{
			1: {
				{AuthTypeIDRef: refGuest, AuthTypeID: "guest", DefaultEnabled: false, Locked: false, Template: emptyTemplate()},
				{AuthTypeIDRef: refPhone, AuthTypeID: "phone", DefaultEnabled: true, Locked: false, Template: emptyTemplate()},
				{AuthTypeIDRef: refGoogle, AuthTypeID: "google", DefaultEnabled: false, Locked: false, Template: googleTemplate()},
				{AuthTypeIDRef: refApple, AuthTypeID: "apple", DefaultEnabled: false, Locked: false, Template: domainaa.Template{}}, // 无可用模板（TemplateVersion=""）
				{AuthTypeIDRef: refLine, AuthTypeID: "line", DefaultEnabled: false, Locked: true, Template: emptyTemplate()},
			},
		},
		configs: map[int64]map[int64]accountauthapp.GameConfigItem{},
	}
}

func cloneConfigItem(it accountauthapp.GameConfigItem) accountauthapp.GameConfigItem {
	cp := it
	cp.ConfigJSON = cloneStrAnyMap(it.ConfigJSON)
	if it.LastCheckAt != nil {
		t := *it.LastCheckAt
		cp.LastCheckAt = &t
	}
	return cp
}

func cloneStrAnyMap(in map[string]any) map[string]any {
	out := make(map[string]any, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}

func (s *aaState) clone() *aaState {
	c := &aaState{
		types:       s.types,
		channelPol:  s.channelPol,
		gameRowID:   s.gameRowID,
		allowed:     s.allowed,
		failReplace: s.failReplace,
		configs:     map[int64]map[int64]accountauthapp.GameConfigItem{},
	}
	for rowID, byRef := range s.configs {
		m := map[int64]accountauthapp.GameConfigItem{}
		for ref, it := range byRef {
			m[ref] = cloneConfigItem(it)
		}
		c.configs[rowID] = m
	}
	return c
}

// ───────────────────────── TxManager + Repository ─────────────────────────

type aaStore struct{ state *aaState }

func newAAStore() *aaStore { return &aaStore{state: newAAState()} }

func (s *aaStore) Repository() accountauthapp.Repository { return &aaRepo{s.state} }

func (s *aaStore) InTx(ctx context.Context, fn func(accountauthapp.Repository) error) error {
	clone := s.state.clone()
	if err := fn(&aaRepo{clone}); err != nil {
		return err // 丢弃 clone = 回滚
	}
	s.state = clone
	return nil
}

// configFor 仓储状态断言辅助：返回某游戏某认证类型已落库的 config_json 值（测试内只读）。
func (s *aaStore) configFor(rowID, authTypeIDRef int64) (accountauthapp.GameConfigItem, bool) {
	byRef, ok := s.state.configs[rowID]
	if !ok {
		return accountauthapp.GameConfigItem{}, false
	}
	it, ok := byRef[authTypeIDRef]
	return it, ok
}

type aaRepo struct{ st *aaState }

func (r *aaRepo) ListTypeCatalog(_ context.Context) ([]accountauthapp.TypeCatalogItem, error) {
	return r.st.types, nil
}

func (r *aaRepo) ListChannelPolicies(_ context.Context, channelID string) ([]accountauthapp.ChannelTypePolicy, error) {
	items, ok := r.st.channelPol[channelID]
	if !ok {
		return nil, adminapp.ErrNotFound
	}
	return items, nil
}

func (r *aaRepo) ResolveGameRowID(_ context.Context, gameID string) (int64, error) {
	rowID, ok := r.st.gameRowID[gameID]
	if !ok {
		return 0, adminapp.ErrNotFound
	}
	return rowID, nil
}

func (r *aaRepo) ListAllowedTypesByGame(_ context.Context, gameIDRef int64) ([]accountauthapp.GameAllowedType, error) {
	return r.st.allowed[gameIDRef], nil
}

func (r *aaRepo) ListGameConfigs(_ context.Context, gameIDRef int64) ([]accountauthapp.GameConfigItem, error) {
	byRef := r.st.configs[gameIDRef]
	out := make([]accountauthapp.GameConfigItem, 0, len(byRef))
	for _, it := range byRef {
		out = append(out, cloneConfigItem(it))
	}
	// 稳定序：按 authTypeIDRef 排序，保证断言可重复。
	sort.Slice(out, func(i, j int) bool { return out[i].AuthTypeIDRef < out[j].AuthTypeIDRef })
	return out, nil
}

// ReplaceGameConfigs 整体替换：删除该游戏旧配置 → 写入新集合（last-writer-wins）。
func (r *aaRepo) ReplaceGameConfigs(_ context.Context, gameIDRef int64, items []accountauthapp.GameConfigUpsert) error {
	if r.st.failReplace {
		return errors.New("forced replace failure (S10)")
	}
	byRef := map[int64]accountauthapp.GameConfigItem{}
	for _, up := range items {
		byRef[up.AuthTypeIDRef] = accountauthapp.GameConfigItem{
			AuthTypeIDRef:    up.AuthTypeIDRef,
			Enabled:          up.Enabled,
			ConfigJSON:       cloneStrAnyMap(up.ConfigJSON),
			ConfigStatus:     up.ConfigStatus,
			LastCheckAt:      up.LastCheckAt,
			LastCheckMessage: up.LastCheckMessage,
		}
	}
	r.st.configs[gameIDRef] = byRef
	return nil
}

// ───────────────────────── spy Cipher ─────────────────────────

// spyCipher 模拟密文位加密：Encrypt(plain)="enc:"+plain，并计数调用次数。
// 用于断言「只改 enabled / 只改非 secret / 留空 / masked 提交」时不重新加密（密文恒定）。
type spyCipher struct{ encryptCalls int }

func (c *spyCipher) Encrypt(plain string) (string, error) {
	c.encryptCalls++
	return "enc:" + plain, nil
}
