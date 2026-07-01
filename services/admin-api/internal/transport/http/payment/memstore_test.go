package payment

import (
	"context"
	"encoding/base64"
	"errors"
	"sort"
	"sync"

	adminapp "github.com/csw/console/services/admin-api/internal/app/admin"
	paymentapp "github.com/csw/console/services/admin-api/internal/app/payment"
	"github.com/csw/console/services/admin-api/internal/domain/accountauth"
)

type paymentMemStore struct {
	mu    sync.Mutex
	state *paymentMemState
}

type paymentMemState struct {
	failReplaceAfterDelete bool
	seqRouteID             int64

	gamesByCode  map[string]int64
	gameCodeByID map[int64]string

	payWaysByCode map[string]paymentapp.PayWayRef
	payWayMeta    map[int64]struct {
		id   string
		name string
		kind string
	}

	providersByCode  map[string]paymentapp.ProviderRef
	providerCodeByID map[int64]string

	subjectsByCode map[string]paymentapp.SubjectRef

	merchantByCode map[string]paymentapp.MerchantAccountRef
	merchantByID   map[int64]paymentapp.MerchantAccountRef

	channelsByCode map[string]struct {
		id      int64
		enabled bool
	}
	channelCodeByID map[int64]string

	packagesByGameAndCode map[int64]map[string]struct {
		id      int64
		enabled bool
	}
	packageCodeByID map[int64]string

	merchantAccounts []paymentapp.MerchantAccountDTO
	routesByGameID   map[int64][]storedRoute
}

type storedRoute struct {
	id int64
	paymentapp.RouteRecord
}

func newPaymentMemStore() *paymentMemStore {
	st := &paymentMemState{
		seqRouteID:       100,
		gamesByCode:      map[string]int64{"100001": 1},
		gameCodeByID:     map[int64]string{1: "100001"},
		payWaysByCode:    map[string]paymentapp.PayWayRef{},
		payWayMeta:       map[int64]struct{ id, name, kind string }{},
		providersByCode:  map[string]paymentapp.ProviderRef{},
		providerCodeByID: map[int64]string{},
		subjectsByCode:   map[string]paymentapp.SubjectRef{},
		merchantByCode:   map[string]paymentapp.MerchantAccountRef{},
		merchantByID:     map[int64]paymentapp.MerchantAccountRef{},
		channelsByCode: map[string]struct {
			id      int64
			enabled bool
		}{},
		channelCodeByID: map[int64]string{},
		packagesByGameAndCode: map[int64]map[string]struct {
			id      int64
			enabled bool
		}{},
		packageCodeByID: map[int64]string{},
		routesByGameID:  map[int64][]storedRoute{},
	}

	st.payWaysByCode["credit_card"] = paymentapp.PayWayRef{RowID: 1, PayWayID: "credit_card", Enabled: true}
	st.payWayMeta[1] = struct{ id, name, kind string }{id: "credit_card", name: "信用卡", kind: "card"}
	st.payWaysByCode["paypal"] = paymentapp.PayWayRef{RowID: 2, PayWayID: "paypal", Enabled: true}
	st.payWayMeta[2] = struct{ id, name, kind string }{id: "paypal", name: "PayPal", kind: "wallet"}

	st.providersByCode["airwallex"] = paymentapp.ProviderRef{RowID: 1, ProviderID: "airwallex", Enabled: true}
	st.providerCodeByID[1] = "airwallex"
	st.providersByCode["payermax"] = paymentapp.ProviderRef{RowID: 2, ProviderID: "payermax", Enabled: true}
	st.providerCodeByID[2] = "payermax"

	st.subjectsByCode["hk_entity"] = paymentapp.SubjectRef{RowID: 1, SubjectID: "hk_entity", Enabled: true}
	st.merchantByCode["merchant_aw_main"] = paymentapp.MerchantAccountRef{
		RowID: 1, MerchantAccountID: "merchant_aw_main", ProviderID: "airwallex", Enabled: true,
	}
	st.merchantByID[1] = st.merchantByCode["merchant_aw_main"]
	st.merchantByCode["merchant_pm_main"] = paymentapp.MerchantAccountRef{
		RowID: 2, MerchantAccountID: "merchant_pm_main", ProviderID: "payermax", Enabled: true,
	}
	st.merchantByID[2] = st.merchantByCode["merchant_pm_main"]

	st.channelsByCode["googleplay"] = struct {
		id      int64
		enabled bool
	}{id: 11, enabled: true}
	st.channelCodeByID[11] = "googleplay"
	st.packagesByGameAndCode[1] = map[string]struct {
		id      int64
		enabled bool
	}{
		"pkg.jp": {id: 21, enabled: true},
	}
	st.packageCodeByID[21] = "pkg.jp"

	st.merchantAccounts = []paymentapp.MerchantAccountDTO{
		{
			MerchantAccountID: "merchant_aw_main",
			ProviderID:        "airwallex",
			SubjectID:         "hk_entity",
			MerchantID:        "AW-001",
			MerchantName:      "Airwallex Main",
			ConfigJSON:        map[string]any{"apiBase": "https://api.example.com"},
			Secret:            "plaintext-never-exposed",
			Enabled:           true,
		},
	}
	st.routesByGameID[1] = []storedRoute{{
		id: 100,
		RouteRecord: paymentapp.RouteRecord{
			MarketCode:           "GLOBAL",
			CountryCode:          "*",
			Currency:             "*",
			PayWayIDRef:          1,
			ProviderIDRef:        1,
			MerchantAccountIDRef: 1,
			Priority:             100,
			Enabled:              true,
		},
	}}
	return &paymentMemStore{state: st}
}

func (s *paymentMemStore) cloneState() *paymentMemState {
	c := *s.state
	c.gamesByCode = cloneMapStringInt64(s.state.gamesByCode)
	c.gameCodeByID = cloneMapInt64String(s.state.gameCodeByID)
	c.payWaysByCode = cloneMapPayWayRef(s.state.payWaysByCode)
	c.payWayMeta = map[int64]struct{ id, name, kind string }{}
	for k, v := range s.state.payWayMeta {
		c.payWayMeta[k] = v
	}
	c.providersByCode = cloneMapProviderRef(s.state.providersByCode)
	c.providerCodeByID = cloneMapInt64String(s.state.providerCodeByID)
	c.subjectsByCode = cloneMapSubjectRef(s.state.subjectsByCode)
	c.merchantByCode = cloneMapMerchantRef(s.state.merchantByCode)
	c.merchantByID = cloneMapMerchantRefByID(s.state.merchantByID)
	c.channelsByCode = map[string]struct {
		id      int64
		enabled bool
	}{}
	for k, v := range s.state.channelsByCode {
		c.channelsByCode[k] = v
	}
	c.channelCodeByID = cloneMapInt64String(s.state.channelCodeByID)
	c.packagesByGameAndCode = map[int64]map[string]struct {
		id      int64
		enabled bool
	}{}
	for gameID, pkgs := range s.state.packagesByGameAndCode {
		c.packagesByGameAndCode[gameID] = map[string]struct {
			id      int64
			enabled bool
		}{}
		for code, info := range pkgs {
			c.packagesByGameAndCode[gameID][code] = info
		}
	}
	c.packageCodeByID = cloneMapInt64String(s.state.packageCodeByID)
	c.merchantAccounts = make([]paymentapp.MerchantAccountDTO, len(s.state.merchantAccounts))
	copy(c.merchantAccounts, s.state.merchantAccounts)
	c.routesByGameID = map[int64][]storedRoute{}
	for k, rows := range s.state.routesByGameID {
		cp := make([]storedRoute, len(rows))
		copy(cp, rows)
		c.routesByGameID[k] = cp
	}
	return &c
}

func (s *paymentMemStore) Repository() paymentapp.Repository {
	s.mu.Lock()
	defer s.mu.Unlock()
	return &paymentMemRepo{st: s.state}
}

func (s *paymentMemStore) InTx(_ context.Context, fn func(paymentapp.Repository) error) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	clone := s.cloneState()
	if err := fn(&paymentMemRepo{st: clone}); err != nil {
		return err
	}
	s.state = clone
	return nil
}

type paymentMemRepo struct {
	st *paymentMemState
}

func (r *paymentMemRepo) ListPayWays(_ context.Context, _ paymentapp.ListFilter) ([]paymentapp.PayWayDTO, int, error) {
	items := make([]paymentapp.PayWayDTO, 0, len(r.st.payWaysByCode))
	for _, ref := range r.st.payWaysByCode {
		meta := r.st.payWayMeta[ref.RowID]
		items = append(items, paymentapp.PayWayDTO{
			PayWayID: meta.id, PayWayName: meta.name, PayWayType: meta.kind, Enabled: ref.Enabled,
		})
	}
	sort.Slice(items, func(i, j int) bool { return items[i].PayWayID < items[j].PayWayID })
	return items, len(items), nil
}

func (r *paymentMemRepo) ListProviders(_ context.Context, _ paymentapp.ListFilter) ([]paymentapp.ProviderDTO, int, error) {
	items := []paymentapp.ProviderDTO{
		{ProviderID: "airwallex", ProviderName: "Airwallex", ProviderKind: "aggregator", Enabled: true},
		{ProviderID: "payermax", ProviderName: "PayerMax", ProviderKind: "aggregator", Enabled: true},
	}
	return items, len(items), nil
}

func (r *paymentMemRepo) ListBillingSubjects(_ context.Context, _ paymentapp.ListFilter) ([]paymentapp.BillingSubjectDTO, int, error) {
	items := []paymentapp.BillingSubjectDTO{{SubjectID: "hk_entity", SubjectName: "HK Entity", LegalEntityName: "HK Entity Ltd", Enabled: true}}
	return items, len(items), nil
}

func (r *paymentMemRepo) CreateBillingSubject(_ context.Context, in paymentapp.BillingSubjectRecord) (paymentapp.BillingSubjectDTO, error) {
	if _, exists := r.st.subjectsByCode[in.SubjectID]; exists {
		return paymentapp.BillingSubjectDTO{}, adminapp.ErrConflict
	}
	row := int64(len(r.st.subjectsByCode) + 10)
	r.st.subjectsByCode[in.SubjectID] = paymentapp.SubjectRef{RowID: row, SubjectID: in.SubjectID, Enabled: in.Enabled}
	return paymentapp.BillingSubjectDTO{
		SubjectID: in.SubjectID, SubjectName: in.SubjectName, LegalEntityName: in.LegalEntityName, Enabled: in.Enabled,
	}, nil
}

func (r *paymentMemRepo) ListMerchantAccounts(_ context.Context, _ paymentapp.ListFilter) ([]paymentapp.MerchantAccountDTO, int, error) {
	out := make([]paymentapp.MerchantAccountDTO, len(r.st.merchantAccounts))
	copy(out, r.st.merchantAccounts)
	return out, len(out), nil
}

func (r *paymentMemRepo) CreateMerchantAccount(_ context.Context, in paymentapp.MerchantAccountRecord) (paymentapp.MerchantAccountDTO, error) {
	if _, exists := r.st.merchantByCode[in.MerchantAccountID]; exists {
		return paymentapp.MerchantAccountDTO{}, adminapp.ErrConflict
	}
	rowID := int64(len(r.st.merchantByCode) + 100)
	ref := paymentapp.MerchantAccountRef{
		RowID: rowID, MerchantAccountID: in.MerchantAccountID, ProviderID: in.ProviderID, Enabled: in.Enabled,
	}
	r.st.merchantByCode[in.MerchantAccountID] = ref
	r.st.merchantByID[rowID] = ref
	dto := paymentapp.MerchantAccountDTO{
		MerchantAccountID: in.MerchantAccountID,
		ProviderID:        in.ProviderID,
		SubjectID:         in.SubjectID,
		MerchantID:        in.MerchantID,
		MerchantName:      in.MerchantName,
		ConfigJSON:        in.ConfigJSON,
		Secret:            in.SecretCiphertext,
		Enabled:           in.Enabled,
	}
	r.st.merchantAccounts = append(r.st.merchantAccounts, dto)
	return dto, nil
}

func (r *paymentMemRepo) GetLatestProviderTemplate(_ context.Context, providerRowID int64) (accountauth.Template, error) {
	// airwallex(rowID=1) 提供一份 enabled 最新模板；其余 provider 无模板（触发前端降级路径）。
	if providerRowID == 1 {
		return accountauth.Template{
			TemplateVersion: "3",
			FormSchema: []accountauth.FormField{
				{Key: "merchant_ref", Label: "Merchant Ref", Component: "input", Required: true, Order: 1},
				{Key: "api_key", Label: "API Key", Component: "password", Required: true, Order: 2},
			},
			SecretFields: []string{"api_key"},
			FileFields:   []accountauth.FileField{},
			ValidationRules: map[string]accountauth.ValidationRule{
				"merchant_ref": {Required: true},
			},
		}, nil
	}
	return accountauth.Template{}, adminapp.ErrNotFound
}

func (r *paymentMemRepo) ResolveGameRowID(_ context.Context, gameID string) (int64, error) {
	if row, ok := r.st.gamesByCode[gameID]; ok {
		return row, nil
	}
	return 0, adminapp.ErrNotFound
}

func (r *paymentMemRepo) ResolvePayWay(_ context.Context, payWayID string) (paymentapp.PayWayRef, error) {
	if row, ok := r.st.payWaysByCode[payWayID]; ok {
		return row, nil
	}
	return paymentapp.PayWayRef{}, adminapp.ErrNotFound
}

func (r *paymentMemRepo) ResolveProvider(_ context.Context, providerID string) (paymentapp.ProviderRef, error) {
	if row, ok := r.st.providersByCode[providerID]; ok {
		return row, nil
	}
	return paymentapp.ProviderRef{}, adminapp.ErrNotFound
}

func (r *paymentMemRepo) ResolveSubject(_ context.Context, subjectID string) (paymentapp.SubjectRef, error) {
	if row, ok := r.st.subjectsByCode[subjectID]; ok {
		return row, nil
	}
	return paymentapp.SubjectRef{}, adminapp.ErrNotFound
}

func (r *paymentMemRepo) ResolveMerchantAccount(_ context.Context, merchantAccountID string) (paymentapp.MerchantAccountRef, error) {
	if row, ok := r.st.merchantByCode[merchantAccountID]; ok {
		return row, nil
	}
	return paymentapp.MerchantAccountRef{}, adminapp.ErrNotFound
}

func (r *paymentMemRepo) ResolveChannel(_ context.Context, channelID string) (int64, bool, error) {
	if row, ok := r.st.channelsByCode[channelID]; ok {
		return row.id, row.enabled, nil
	}
	return 0, false, adminapp.ErrNotFound
}

func (r *paymentMemRepo) ResolvePackage(_ context.Context, gameRowID int64, packageCode string) (int64, bool, error) {
	if pkgs, ok := r.st.packagesByGameAndCode[gameRowID]; ok {
		if row, ok := pkgs[packageCode]; ok {
			return row.id, row.enabled, nil
		}
	}
	return 0, false, adminapp.ErrNotFound
}

func (r *paymentMemRepo) ReplaceGameRoutes(_ context.Context, gameRowID int64, routes []paymentapp.RouteRecord) error {
	r.st.routesByGameID[gameRowID] = nil
	if r.st.failReplaceAfterDelete {
		return errors.New("forced replace error")
	}
	rows := make([]storedRoute, 0, len(routes))
	for _, route := range routes {
		r.st.seqRouteID++
		rows = append(rows, storedRoute{id: r.st.seqRouteID, RouteRecord: route})
	}
	r.st.routesByGameID[gameRowID] = rows
	return nil
}

func (r *paymentMemRepo) ListGameRoutes(_ context.Context, gameID string) ([]paymentapp.GameRouteRecord, error) {
	gameRowID, ok := r.st.gamesByCode[gameID]
	if !ok {
		return nil, adminapp.ErrNotFound
	}
	rows := r.st.routesByGameID[gameRowID]
	out := make([]paymentapp.GameRouteRecord, 0, len(rows))
	for _, row := range rows {
		pw := r.st.payWayMeta[row.PayWayIDRef]
		payWayEnabled := true
		for _, ref := range r.st.payWaysByCode {
			if ref.RowID == row.PayWayIDRef {
				payWayEnabled = ref.Enabled
				break
			}
		}
		providerEnabled := true
		for _, ref := range r.st.providersByCode {
			if ref.RowID == row.ProviderIDRef {
				providerEnabled = ref.Enabled
				break
			}
		}
		merchant := r.st.merchantByID[row.MerchantAccountIDRef]
		channelEnabled := true
		if row.ChannelIDRef != nil {
			channelEnabled = false
			for _, info := range r.st.channelsByCode {
				if info.id == *row.ChannelIDRef {
					channelEnabled = info.enabled
					break
				}
			}
		}
		packageEnabled := true
		if row.PackageIDRef != nil {
			packageEnabled = false
			for _, pkgs := range r.st.packagesByGameAndCode {
				for _, info := range pkgs {
					if info.id == *row.PackageIDRef {
						packageEnabled = info.enabled
					}
				}
			}
		}
		out = append(out, paymentapp.GameRouteRecord{
			ID:                row.id,
			GameID:            gameID,
			PayWayID:          pw.id,
			PayWayName:        pw.name,
			PayWayType:        pw.kind,
			PackageCode:       routeNullableCode(row.PackageIDRef, r.st.packageCodeByID),
			ChannelID:         routeNullableCode(row.ChannelIDRef, r.st.channelCodeByID),
			MarketCode:        row.MarketCode,
			CountryCode:       row.CountryCode,
			Currency:          row.Currency,
			ProviderID:        r.st.providerCodeByID[row.ProviderIDRef],
			MerchantAccountID: merchant.MerchantAccountID,
			Priority:          row.Priority,
			Enabled:           row.Enabled,
			PayWayEnabled:     payWayEnabled,
			ProviderEnabled:   providerEnabled,
			MerchantEnabled:   merchant.Enabled,
			ChannelEnabled:    channelEnabled,
			PackageEnabled:    packageEnabled,
		})
	}
	return out, nil
}

func (r *paymentMemRepo) ListEnabledRoutes(_ context.Context, gameID, payWayID string) ([]paymentapp.ResolvedRouteRecord, error) {
	gameRowID, ok := r.st.gamesByCode[gameID]
	if !ok {
		return nil, adminapp.ErrNotFound
	}
	targetPayWay, ok := r.st.payWaysByCode[payWayID]
	if !ok {
		return []paymentapp.ResolvedRouteRecord{}, nil
	}
	rows := r.st.routesByGameID[gameRowID]
	out := make([]paymentapp.ResolvedRouteRecord, 0, len(rows))
	for _, row := range rows {
		if row.PayWayIDRef != targetPayWay.RowID || !row.Enabled {
			continue
		}
		merchant := r.st.merchantByID[row.MerchantAccountIDRef]
		out = append(out, paymentapp.ResolvedRouteRecord{
			ID:                row.id,
			PackageCode:       routeNullableCode(row.PackageIDRef, r.st.packageCodeByID),
			ChannelID:         routeNullableCode(row.ChannelIDRef, r.st.channelCodeByID),
			MarketCode:        row.MarketCode,
			CountryCode:       row.CountryCode,
			Currency:          row.Currency,
			PayWayID:          payWayID,
			ProviderID:        r.st.providerCodeByID[row.ProviderIDRef],
			MerchantAccountID: merchant.MerchantAccountID,
			Priority:          row.Priority,
			Enabled:           row.Enabled,
			ProviderEnabled:   true,
			MerchantEnabled:   merchant.Enabled,
		})
	}
	return out, nil
}

type fakeCrypto struct{}

func (fakeCrypto) Encrypt(plain string) (string, error) {
	return "enc:" + base64.StdEncoding.EncodeToString([]byte(plain)), nil
}

type fakeAudit struct {
	entries []paymentapp.AuditEntry
}

func (a *fakeAudit) Write(_ context.Context, entry paymentapp.AuditEntry) error {
	a.entries = append(a.entries, entry)
	return nil
}

func cloneMapStringInt64(in map[string]int64) map[string]int64 {
	out := make(map[string]int64, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}

func cloneMapInt64String(in map[int64]string) map[int64]string {
	out := make(map[int64]string, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}

func cloneMapPayWayRef(in map[string]paymentapp.PayWayRef) map[string]paymentapp.PayWayRef {
	out := make(map[string]paymentapp.PayWayRef, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}

func cloneMapProviderRef(in map[string]paymentapp.ProviderRef) map[string]paymentapp.ProviderRef {
	out := make(map[string]paymentapp.ProviderRef, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}

func cloneMapSubjectRef(in map[string]paymentapp.SubjectRef) map[string]paymentapp.SubjectRef {
	out := make(map[string]paymentapp.SubjectRef, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}

func cloneMapMerchantRef(in map[string]paymentapp.MerchantAccountRef) map[string]paymentapp.MerchantAccountRef {
	out := make(map[string]paymentapp.MerchantAccountRef, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}

func cloneMapMerchantRefByID(in map[int64]paymentapp.MerchantAccountRef) map[int64]paymentapp.MerchantAccountRef {
	out := make(map[int64]paymentapp.MerchantAccountRef, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}

func routeNullableCode(idRef *int64, byID map[int64]string) string {
	if idRef == nil {
		return "*"
	}
	if code, ok := byID[*idRef]; ok {
		return code
	}
	return "*"
}
