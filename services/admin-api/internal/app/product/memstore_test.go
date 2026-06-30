package product

import (
	"context"
	"errors"
	"sort"
	"strings"
	"time"

	adminapp "github.com/csw/console/services/admin-api/internal/app/admin"
	"github.com/csw/console/services/admin-api/internal/domain/accountauth"
	"github.com/csw/console/services/admin-api/internal/domain/common"
	domainproduct "github.com/csw/console/services/admin-api/internal/domain/product"
)

// product 模块（16）进程内 L2 等价装配：内存 TxManager + 全部窄仓储 + spy crypto/file/audit。
// InTx 通过克隆/回填实现真实回滚语义，等价覆盖 S10（包级 ReplaceByPackage 中途失败 → 整体回滚）。
// 不依赖真实 PG；金额归一化、两维独立解析、IAP merge/脱敏/config_status 走真实领域逻辑。

// ───────────────────────── currency specs ─────────────────────────

func specUSD() common.CurrencySpec {
	return common.CurrencySpec{CurrencyCode: "USD", DecimalPlaces: 2, MinAmountMinor: 1, RoundingMode: "half_up", Enabled: true}
}
func specJPY() common.CurrencySpec {
	return common.CurrencySpec{CurrencyCode: "JPY", DecimalPlaces: 0, MinAmountMinor: 1, RoundingMode: "half_up", Enabled: true}
}
func specDisabled() common.CurrencySpec {
	return common.CurrencySpec{CurrencyCode: "ABC", DecimalPlaces: 2, MinAmountMinor: 1, RoundingMode: "half_up", Enabled: false}
}

// iapTemplate 含一个密文字段 privateKey + 必填普通字段 appId，验脱敏/状态。
func iapTemplate() accountauth.Template {
	return accountauth.Template{
		TemplateVersion: "v1",
		FormSchema:      []accountauth.FormField{{Key: "appId", Required: true}},
		SecretFields:    []string{"privateKey"},
	}
}

// ───────────────────────── 内存状态 ─────────────────────────

type pkgInfo struct {
	gameID        string
	packageCode   string
	channelID     string
	gameChannelID int64
}

type memState struct {
	products     map[int64]domainproduct.Product          // ref -> product
	productSeq   int64                                     // ID 自增
	channelProds map[int64][]domainproduct.ChannelProduct  // packageID -> mappings
	cpSeq        int64                                     // channel_products ID 自增
	iapConfigs   map[int64]domainproduct.IAPConfig         // gameChannelID -> config
	pkgOverrides map[int64]domainproduct.IAPConfig         // packageID -> override
	specs        map[string]common.CurrencySpec            // currencyCode -> spec
	packages     map[int64]pkgInfo                         // packageID -> info
	templates    map[string]accountauth.Template           // channelID -> enabled latest template
	gameChannels map[int64]string                          // gameChannelID -> channelID
	failReplace  bool                                      // S10：强制 ReplaceByPackage 失败
}

func newMemState() *memState {
	return &memState{
		products:     map[int64]domainproduct.Product{},
		channelProds: map[int64][]domainproduct.ChannelProduct{},
		iapConfigs:   map[int64]domainproduct.IAPConfig{},
		pkgOverrides: map[int64]domainproduct.IAPConfig{},
		specs: map[string]common.CurrencySpec{
			"USD": specUSD(),
			"JPY": specJPY(),
			"ABC": specDisabled(),
		},
		packages:     map[int64]pkgInfo{},
		templates:    map[string]accountauth.Template{},
		gameChannels: map[int64]string{},
	}
}

func cloneStrAny(in map[string]any) map[string]any {
	if in == nil {
		return map[string]any{}
	}
	out := make(map[string]any, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}

func cloneIAP(c domainproduct.IAPConfig) domainproduct.IAPConfig {
	cp := c
	cp.ConfigJSON = cloneStrAny(c.ConfigJSON)
	if c.LastCheckAt != nil {
		t := *c.LastCheckAt
		cp.LastCheckAt = &t
	}
	return cp
}

func (s *memState) clone() *memState {
	c := &memState{
		products:     map[int64]domainproduct.Product{},
		productSeq:   s.productSeq,
		channelProds: map[int64][]domainproduct.ChannelProduct{},
		cpSeq:        s.cpSeq,
		iapConfigs:   map[int64]domainproduct.IAPConfig{},
		pkgOverrides: map[int64]domainproduct.IAPConfig{},
		specs:        s.specs,
		packages:     s.packages,
		templates:    s.templates,
		gameChannels: s.gameChannels,
		failReplace:  s.failReplace,
	}
	for k, v := range s.products {
		c.products[k] = v
	}
	for k, v := range s.channelProds {
		cp := make([]domainproduct.ChannelProduct, len(v))
		copy(cp, v)
		c.channelProds[k] = cp
	}
	for k, v := range s.iapConfigs {
		c.iapConfigs[k] = cloneIAP(v)
	}
	for k, v := range s.pkgOverrides {
		c.pkgOverrides[k] = cloneIAP(v)
	}
	return c
}

// ───────────────────────── TxManager + Repositories ─────────────────────────

type memStore struct {
	state *memState
}

func newMemStore() *memStore { return &memStore{state: newMemState()} }

func (s *memStore) Repositories() Repositories {
	return Repositories{
		Products:            &memProductRepo{s.state},
		ChannelProducts:     &memChannelProductRepo{s.state},
		Packages:            &memPackageRepo{s.state},
		CurrencySpecs:       &memCurrencyRepo{s.state},
		GameChannelIAP:      &memGameChannelIAPRepo{s.state},
		PackageIAPOverrides: &memPackageOverrideRepo{s.state},
		ChannelIAPTemplates: &memTemplateRepo{s.state},
	}
}

func (s *memStore) InTx(ctx context.Context, fn func(Repositories) error) error {
	clone := s.state.clone()
	repos := Repositories{
		Products:            &memProductRepo{clone},
		ChannelProducts:     &memChannelProductRepo{clone},
		Packages:            &memPackageRepo{clone},
		CurrencySpecs:       &memCurrencyRepo{clone},
		GameChannelIAP:      &memGameChannelIAPRepo{clone},
		PackageIAPOverrides: &memPackageOverrideRepo{clone},
		ChannelIAPTemplates: &memTemplateRepo{clone},
	}
	if err := fn(repos); err != nil {
		return err // 丢弃 clone = 回滚
	}
	s.state = clone
	return nil
}

// ───────────────────────── ProductRepository ─────────────────────────

type memProductRepo struct{ st *memState }

func (r *memProductRepo) ListByGame(_ context.Context, gameID, keyword string, enabled *bool, page, pageSize int, _ string) ([]domainproduct.Product, int, error) {
	matched := []domainproduct.Product{}
	for _, p := range r.st.products {
		if p.GameID != gameID {
			continue
		}
		if keyword != "" && !strings.Contains(p.ProductID, keyword) && !strings.Contains(p.ProductName, keyword) {
			continue
		}
		if enabled != nil && p.Enabled != *enabled {
			continue
		}
		matched = append(matched, p)
	}
	sort.Slice(matched, func(i, j int) bool { return matched[i].ID > matched[j].ID })
	total := len(matched)
	start := (page - 1) * pageSize
	if start >= total {
		return []domainproduct.Product{}, total, nil
	}
	end := start + pageSize
	if end > total {
		end = total
	}
	return matched[start:end], total, nil
}

func (r *memProductRepo) Create(_ context.Context, item domainproduct.Product) (domainproduct.Product, error) {
	for _, p := range r.st.products {
		if p.GameID == item.GameID && p.ProductID == item.ProductID {
			return domainproduct.Product{}, adminapp.ErrConflict
		}
	}
	r.st.productSeq++
	item.ID = r.st.productSeq
	now := time.Unix(1700000000, 0).UTC()
	item.CreatedAt, item.UpdatedAt = now, now
	r.st.products[item.ID] = item
	return item, nil
}

func (r *memProductRepo) GetByGameAndProductID(_ context.Context, gameID, productID string) (domainproduct.Product, error) {
	for _, p := range r.st.products {
		if p.GameID == gameID && p.ProductID == productID {
			return p, nil
		}
	}
	return domainproduct.Product{}, adminapp.ErrNotFound
}

func (r *memProductRepo) Update(_ context.Context, gameID, productID string, patch ProductPatch) (domainproduct.Product, error) {
	for id, p := range r.st.products {
		if p.GameID == gameID && p.ProductID == productID {
			if patch.ProductName != nil {
				p.ProductName = *patch.ProductName
			}
			if patch.BaseAmountMinor != nil {
				p.BaseAmountMinor = *patch.BaseAmountMinor
			}
			if patch.BaseCurrency != nil {
				p.BaseCurrency = *patch.BaseCurrency
			}
			if patch.PriceID != nil {
				p.PriceID = *patch.PriceID
			}
			if patch.Enabled != nil {
				p.Enabled = *patch.Enabled
			}
			r.st.products[id] = p
			return p, nil
		}
	}
	return domainproduct.Product{}, adminapp.ErrNotFound
}

func (r *memProductRepo) ListByIDs(_ context.Context, gameID string, productIDs []string) ([]domainproduct.Product, error) {
	want := map[string]bool{}
	for _, id := range productIDs {
		want[id] = true
	}
	out := []domainproduct.Product{}
	for _, p := range r.st.products {
		if p.GameID == gameID && want[p.ProductID] {
			out = append(out, p)
		}
	}
	return out, nil
}

// ───────────────────────── ChannelProductRepository ─────────────────────────

type memChannelProductRepo struct{ st *memState }

func (r *memChannelProductRepo) ListByPackage(_ context.Context, packageID int64) ([]domainproduct.ChannelProduct, error) {
	items := r.st.channelProds[packageID]
	out := make([]domainproduct.ChannelProduct, len(items))
	copy(out, items)
	return out, nil
}

// ReplaceByPackage 全量替换：删除该包旧映射 → 写入新集合（删除未出现项语义）。
func (r *memChannelProductRepo) ReplaceByPackage(_ context.Context, packageID int64, items []domainproduct.ChannelProduct) error {
	if r.st.failReplace {
		return errors.New("forced replace failure (S10)")
	}
	stored := make([]domainproduct.ChannelProduct, 0, len(items))
	for _, it := range items {
		r.st.cpSeq++
		it.ID = r.st.cpSeq
		it.PackageIDRef = packageID
		stored = append(stored, it)
	}
	if len(stored) == 0 {
		delete(r.st.channelProds, packageID)
		return nil
	}
	r.st.channelProds[packageID] = stored
	return nil
}

// ───────────────────────── ChannelPackageRepository ─────────────────────────

type memPackageRepo struct{ st *memState }

func (r *memPackageRepo) GetPackageGameAndChannel(_ context.Context, packageID int64) (string, string, string, int64, error) {
	info, ok := r.st.packages[packageID]
	if !ok {
		return "", "", "", 0, adminapp.ErrNotFound
	}
	return info.gameID, info.packageCode, info.channelID, info.gameChannelID, nil
}

func (r *memPackageRepo) BelongsToGame(_ context.Context, packageID int64, gameID string) (bool, error) {
	info, ok := r.st.packages[packageID]
	if !ok {
		return false, adminapp.ErrNotFound
	}
	return info.gameID == gameID, nil
}

// ───────────────────────── CurrencySpecRepository ─────────────────────────

type memCurrencyRepo struct{ st *memState }

func (r *memCurrencyRepo) GetByCode(_ context.Context, code string) (common.CurrencySpec, error) {
	spec, ok := r.st.specs[code]
	if !ok {
		return common.CurrencySpec{}, adminapp.ErrNotFound
	}
	return spec, nil
}

// ───────────────────────── GameChannelIapConfigRepository ─────────────────────────

type memGameChannelIAPRepo struct{ st *memState }

func (r *memGameChannelIAPRepo) GetByGameChannelID(_ context.Context, gameChannelID int64) (domainproduct.IAPConfig, bool, error) {
	cfg, ok := r.st.iapConfigs[gameChannelID]
	if !ok {
		return domainproduct.IAPConfig{}, false, nil
	}
	return cloneIAP(cfg), true, nil
}

func (r *memGameChannelIAPRepo) UpsertByGameChannelID(_ context.Context, gameChannelID int64, cfg domainproduct.IAPConfig) (domainproduct.IAPConfig, error) {
	cfg.ID = gameChannelID
	cfg.GameChannelIDRef = gameChannelID
	r.st.iapConfigs[gameChannelID] = cloneIAP(cfg)
	return cloneIAP(cfg), nil
}

func (r *memGameChannelIAPRepo) GetChannelInfo(_ context.Context, gameChannelID int64) (string, error) {
	ch, ok := r.st.gameChannels[gameChannelID]
	if !ok {
		return "", adminapp.ErrNotFound
	}
	return ch, nil
}

// ───────────────────────── ChannelPackageIapOverrideRepository ─────────────────────────

type memPackageOverrideRepo struct{ st *memState }

func (r *memPackageOverrideRepo) GetByPackageID(_ context.Context, packageID int64) (domainproduct.IAPConfig, bool, error) {
	cfg, ok := r.st.pkgOverrides[packageID]
	if !ok {
		return domainproduct.IAPConfig{}, false, nil
	}
	return cloneIAP(cfg), true, nil
}

func (r *memPackageOverrideRepo) UpsertByPackageID(_ context.Context, packageID int64, cfg domainproduct.IAPConfig) (domainproduct.IAPConfig, error) {
	cfg.ID = packageID
	cfg.PackageIDRef = packageID
	r.st.pkgOverrides[packageID] = cloneIAP(cfg)
	return cloneIAP(cfg), nil
}

// ───────────────────────── ChannelIapTemplateRepository ─────────────────────────

type memTemplateRepo struct{ st *memState }

func (r *memTemplateRepo) GetLatestEnabledByChannelID(_ context.Context, channelID string) (accountauth.Template, error) {
	tpl, ok := r.st.templates[channelID]
	if !ok {
		return accountauth.Template{}, adminapp.ErrNotFound
	}
	return tpl, nil
}

// ───────────────────────── spy crypto / file / audit ─────────────────────────

// spyCrypto 模拟密文加密：Encrypt(plain)="enc:"+plain，并计数调用次数。
// 用于断言「masked/留空/仅 toggle 提交」时不重新加密（密文恒定）。
type spyCrypto struct{ encryptCalls int }

func (c *spyCrypto) Encrypt(plain string) (string, error) {
	c.encryptCalls++
	return "enc:" + plain, nil
}

// spyFile 文件引用规范化：原样返回（最小实现），用于挂入 FileService 端口。
type spyFile struct{}

func (spyFile) NormalizeReference(value string) (string, error) { return value, nil }

// spyAudit 记录所有审计写入，供断言 action / detail 脱敏。
type spyAudit struct{ entries []adminapp.AuditEntry }

func (a *spyAudit) Write(_ context.Context, entry adminapp.AuditEntry) error {
	a.entries = append(a.entries, entry)
	return nil
}

func fixedNow() time.Time { return time.Unix(1700000100, 0).UTC() }
