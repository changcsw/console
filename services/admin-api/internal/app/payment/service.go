package payment

import (
	"context"
	"encoding/json"
	"errors"
	"regexp"
	"sort"
	"strings"
	"time"

	adminapp "github.com/csw/console/services/admin-api/internal/app/admin"
	"github.com/csw/console/services/admin-api/internal/domain/accountauth"
	domainpayment "github.com/csw/console/services/admin-api/internal/domain/payment"
)

var subjectIDRegex = regexp.MustCompile(`^[a-z0-9_]{1,64}$`)

type Service struct {
	tx     TxManager
	crypto CryptoService
	audit  AuditSink
	now    nowFunc
}

func NewService(tx TxManager, crypto CryptoService, audit AuditSink, now nowFunc) *Service {
	if now == nil {
		now = time.Now
	}
	return &Service{tx: tx, crypto: crypto, audit: audit, now: now}
}

func (s *Service) ListPayWays(ctx context.Context, filter ListFilter) ([]PayWayDTO, int, error) {
	return s.tx.Repository().ListPayWays(ctx, normalizeFilter(filter))
}

func (s *Service) ListProviders(ctx context.Context, filter ListFilter) ([]ProviderDTO, int, error) {
	return s.tx.Repository().ListProviders(ctx, normalizeFilter(filter))
}

func (s *Service) ListBillingSubjects(ctx context.Context, filter ListFilter) ([]BillingSubjectDTO, int, error) {
	return s.tx.Repository().ListBillingSubjects(ctx, normalizeFilter(filter))
}

func (s *Service) ListMerchantAccounts(ctx context.Context, filter ListFilter) ([]MerchantAccountDTO, int, error) {
	items, total, err := s.tx.Repository().ListMerchantAccounts(ctx, normalizeFilter(filter))
	if err != nil {
		return nil, 0, mapRepoErr(err)
	}
	for i := range items {
		items[i].Secret = maskedValue
	}
	return items, total, nil
}

func (s *Service) GetProviderTemplate(ctx context.Context, providerID string) (accountauth.Template, error) {
	providerID = strings.TrimSpace(providerID)
	if providerID == "" {
		return accountauth.Template{}, validationErr("providerId 必填", fieldDetail("providerId", "required"))
	}
	provider, err := s.tx.Repository().ResolveProvider(ctx, providerID)
	if err != nil {
		return accountauth.Template{}, mapRepoErr(err)
	}
	tpl, err := s.tx.Repository().GetLatestProviderTemplate(ctx, provider.RowID)
	if err != nil {
		return accountauth.Template{}, mapRepoErr(err)
	}
	return tpl, nil
}

func (s *Service) GetGameRoutes(ctx context.Context, gameID string) (GameRoutesDTO, error) {
	gameID = strings.TrimSpace(gameID)
	if gameID == "" {
		return GameRoutesDTO{}, validationErr("gameId 必填", fieldDetail("gameId", "required"))
	}
	rows, err := s.tx.Repository().ListGameRoutes(ctx, gameID)
	if err != nil {
		return GameRoutesDTO{}, mapRepoErr(err)
	}
	grouped := make(map[string]*RouteGroupDTO, len(rows))
	groupOrder := make([]string, 0)
	for _, row := range rows {
		group, ok := grouped[row.PayWayID]
		if !ok {
			group = &RouteGroupDTO{
				PayWayID:   row.PayWayID,
				PayWayName: row.PayWayName,
				PayWayType: row.PayWayType,
			}
			grouped[row.PayWayID] = group
			groupOrder = append(groupOrder, row.PayWayID)
		}
		hasDisabled := !row.PayWayEnabled || !row.ProviderEnabled || !row.MerchantEnabled || !row.ChannelEnabled || !row.PackageEnabled
		group.Routes = append(group.Routes, RouteItemDTO{
			ID: row.ID,
			Selector: RouteSelectorDTO{
				PackageCode: wildcardToNullable(row.PackageCode),
				ChannelID:   wildcardToNullable(row.ChannelID),
				MarketCode:  selectorDisplayCode(row.MarketCode),
				CountryCode: selectorDisplayCode(row.CountryCode),
				Currency:    selectorDisplayCode(row.Currency),
			},
			ProviderID:             row.ProviderID,
			MerchantAccountID:      row.MerchantAccountID,
			Priority:               row.Priority,
			Enabled:                row.Enabled,
			HasDisabledReference:   hasDisabled,
			PayWayEnabled:          row.PayWayEnabled,
			ProviderEnabled:        row.ProviderEnabled,
			MerchantAccountEnabled: row.MerchantEnabled,
			ChannelEnabled:         row.ChannelEnabled,
			PackageEnabled:         row.PackageEnabled,
		})
	}

	groups := make([]RouteGroupDTO, 0, len(groupOrder))
	for _, payWayID := range groupOrder {
		group := grouped[payWayID]
		sort.SliceStable(group.Routes, func(i, j int) bool {
			if group.Routes[i].Priority != group.Routes[j].Priority {
				return group.Routes[i].Priority < group.Routes[j].Priority
			}
			return group.Routes[i].ID < group.Routes[j].ID
		})
		groups = append(groups, *group)
	}
	env := ""
	if ac, ok := adminapp.AuthContextFrom(ctx); ok {
		env = string(ac.Environment)
	}
	return GameRoutesDTO{GameID: gameID, Env: env, Groups: groups}, nil
}

func (s *Service) CreateBillingSubject(ctx context.Context, cmd CreateBillingSubjectCommand) (BillingSubjectDTO, error) {
	cmd.SubjectID = strings.TrimSpace(cmd.SubjectID)
	cmd.SubjectName = strings.TrimSpace(cmd.SubjectName)
	cmd.LegalEntityName = strings.TrimSpace(cmd.LegalEntityName)
	if !subjectIDRegex.MatchString(cmd.SubjectID) {
		return BillingSubjectDTO{}, validationErr("subjectId 非法", fieldDetail("subjectId", "pattern"))
	}
	if cmd.SubjectName == "" || len(cmd.SubjectName) > 128 {
		return BillingSubjectDTO{}, validationErr("subjectName 非法", fieldDetail("subjectName", "length"))
	}
	if cmd.LegalEntityName == "" || len(cmd.LegalEntityName) > 255 {
		return BillingSubjectDTO{}, validationErr("legalEntityName 非法", fieldDetail("legalEntityName", "length"))
	}

	out, err := s.tx.Repository().CreateBillingSubject(ctx, BillingSubjectRecord{
		SubjectID:       cmd.SubjectID,
		SubjectName:     cmd.SubjectName,
		LegalEntityName: cmd.LegalEntityName,
		Enabled:         cmd.Enabled,
	})
	if err != nil {
		return BillingSubjectDTO{}, mapRepoErr(err)
	}
	s.writeAudit(ctx, AuditEntry{
		Action:       "billing_subject.create",
		ResourceType: "billing_subject",
		ResourceID:   out.SubjectID,
		Detail: map[string]any{
			"subjectId":   out.SubjectID,
			"subjectName": out.SubjectName,
		},
	})
	return out, nil
}

func (s *Service) CreateMerchantAccount(ctx context.Context, cmd CreateMerchantAccountCommand) (MerchantAccountDTO, error) {
	cmd.MerchantAccountID = strings.TrimSpace(cmd.MerchantAccountID)
	cmd.ProviderID = strings.TrimSpace(cmd.ProviderID)
	cmd.SubjectID = strings.TrimSpace(cmd.SubjectID)
	cmd.MerchantID = strings.TrimSpace(cmd.MerchantID)
	cmd.MerchantName = strings.TrimSpace(cmd.MerchantName)
	if cmd.MerchantAccountID == "" || cmd.ProviderID == "" || cmd.SubjectID == "" || cmd.MerchantID == "" || cmd.MerchantName == "" {
		return MerchantAccountDTO{}, validationErr("merchant account 字段不完整", fieldDetail("merchantAccountId", "required"))
	}
	if len(cmd.ConfigJSON) == 0 {
		cmd.ConfigJSON = map[string]any{}
	}
	if cmd.Secrets == nil {
		cmd.Secrets = map[string]string{}
	}
	provider, err := s.tx.Repository().ResolveProvider(ctx, cmd.ProviderID)
	if err != nil {
		return MerchantAccountDTO{}, mapRepoErr(err)
	}
	if !provider.Enabled {
		return MerchantAccountDTO{}, validationErr("provider 已禁用", fieldDetail("providerId", "disabled"))
	}
	subject, err := s.tx.Repository().ResolveSubject(ctx, cmd.SubjectID)
	if err != nil {
		return MerchantAccountDTO{}, mapRepoErr(err)
	}
	if !subject.Enabled {
		return MerchantAccountDTO{}, validationErr("subject 已禁用", fieldDetail("subjectId", "disabled"))
	}

	tpl, err := s.tx.Repository().GetLatestProviderTemplate(ctx, provider.RowID)
	if err != nil && !errors.Is(err, adminapp.ErrNotFound) {
		return MerchantAccountDTO{}, mapRepoErr(err)
	}
	if err == nil {
		combined := cloneMap(cmd.ConfigJSON)
		for key, value := range cmd.Secrets {
			combined[key] = value
		}
		status, msg := accountauth.ValidateConfigAgainstTemplate(combined, tpl)
		if status == "invalid" {
			return MerchantAccountDTO{}, validationErr(msg, fieldDetail("configJson", "template_validation_failed"))
		}
	}

	if s.crypto == nil {
		return MerchantAccountDTO{}, validationErr("密文加密器未配置", fieldDetail("secrets", "crypto_unavailable"))
	}
	secretsPayload, err := json.Marshal(cmd.Secrets)
	if err != nil {
		return MerchantAccountDTO{}, validationErr("secrets 非法", fieldDetail("secrets", "invalid_json"))
	}
	ciphertext, err := s.crypto.Encrypt(string(secretsPayload))
	if err != nil {
		return MerchantAccountDTO{}, validationErr("密钥加密失败", fieldDetail("secrets", "encrypt_failed"))
	}

	out, err := s.tx.Repository().CreateMerchantAccount(ctx, MerchantAccountRecord{
		MerchantAccountID: cmd.MerchantAccountID,
		ProviderRowID:     provider.RowID,
		SubjectRowID:      subject.RowID,
		ProviderID:        provider.ProviderID,
		SubjectID:         subject.SubjectID,
		MerchantID:        cmd.MerchantID,
		MerchantName:      cmd.MerchantName,
		ConfigJSON:        cmd.ConfigJSON,
		SecretCiphertext:  ciphertext,
		Enabled:           cmd.Enabled,
	})
	if err != nil {
		return MerchantAccountDTO{}, mapRepoErr(err)
	}
	out.Secret = maskedValue

	secretKeys := make([]string, 0, len(cmd.Secrets))
	for k := range cmd.Secrets {
		secretKeys = append(secretKeys, k)
	}
	s.writeAudit(ctx, AuditEntry{
		Action:       "merchant_account.create",
		ResourceType: "merchant_account",
		ResourceID:   out.MerchantAccountID,
		Detail: map[string]any{
			"merchantAccountId": out.MerchantAccountID,
			"providerId":        out.ProviderID,
			"subjectId":         out.SubjectID,
			"merchantId":        out.MerchantID,
		},
		SecretKeys: secretKeys,
	})
	return out, nil
}

func (s *Service) SaveGameRoutes(ctx context.Context, gameID string, cmd SaveGameRoutesCommand) (GameRoutesDTO, error) {
	gameID = strings.TrimSpace(gameID)
	if gameID == "" {
		return GameRoutesDTO{}, validationErr("gameId 必填", fieldDetail("gameId", "required"))
	}

	err := s.tx.InTx(ctx, func(repo Repository) error {
		gameRowID, err := repo.ResolveGameRowID(ctx, gameID)
		if err != nil {
			return mapRepoErr(err)
		}
		writeRows := make([]RouteRecord, 0, len(cmd.Items))
		domainRows := make([]domainpayment.Route, 0, len(cmd.Items))
		for i, item := range cmd.Items {
			norm, route, itemErr := s.normalizeRouteItem(ctx, repo, gameRowID, item)
			if itemErr != nil {
				return validationErr("路由项校验失败", map[string]any{
					"index":  i,
					"reason": itemErr.Error(),
				})
			}
			writeRows = append(writeRows, norm)
			domainRows = append(domainRows, route)
		}
		if err := domainpayment.ValidateRouteSet(domainRows); err != nil {
			var conflict *domainpayment.RouteConflictError
			if errors.As(err, &conflict) {
				return routeConflictErr(conflict)
			}
			return validationErr(err.Error())
		}
		if err := repo.ReplaceGameRoutes(ctx, gameRowID, writeRows); err != nil {
			return mapRepoErr(err)
		}
		s.writeAudit(ctx, AuditEntry{
			Action:       "payment_route.update",
			ResourceType: "payment_route",
			ResourceID:   gameID,
			Detail: map[string]any{
				"gameId": gameID,
				"count":  len(writeRows),
			},
		})
		return nil
	})
	if err != nil {
		return GameRoutesDTO{}, err
	}
	return s.GetGameRoutes(ctx, gameID)
}

func (s *Service) ResolveRoute(ctx context.Context, gameID string, input domainpayment.MatchInput) (domainpayment.RouteTarget, error) {
	gameID = strings.TrimSpace(gameID)
	input.PayWay = strings.TrimSpace(input.PayWay)
	if gameID == "" || input.PayWay == "" {
		return domainpayment.RouteTarget{}, notFoundErr("route 不存在")
	}
	rows, err := s.tx.Repository().ListEnabledRoutes(ctx, gameID, input.PayWay)
	if err != nil {
		return domainpayment.RouteTarget{}, mapRepoErr(err)
	}
	candidates := make([]domainpayment.Route, 0, len(rows))
	for _, row := range rows {
		if !row.ProviderEnabled || !row.MerchantEnabled {
			continue
		}
		candidates = append(candidates, domainpayment.Route{
			ID:              row.ID,
			GameID:          gameID,
			Package:         wildcardToEmpty(row.PackageCode),
			Channel:         wildcardToEmpty(row.ChannelID),
			Market:          wildcardToEmpty(row.MarketCode),
			Country:         wildcardToEmpty(row.CountryCode),
			Currency:        wildcardToEmpty(row.Currency),
			PayWay:          row.PayWayID,
			Provider:        row.ProviderID,
			MerchantAccount: row.MerchantAccountID,
			Priority:        row.Priority,
			Enabled:         row.Enabled,
		})
	}
	best, err := domainpayment.PickBestRoute(candidates, input)
	if err != nil {
		if errors.Is(err, domainpayment.ErrRouteNotFound) {
			return domainpayment.RouteTarget{}, notFoundErr("route 不存在")
		}
		return domainpayment.RouteTarget{}, err
	}
	return domainpayment.RouteTarget{
		Provider:        best.Provider,
		MerchantAccount: best.MerchantAccount,
	}, nil
}

func (s *Service) normalizeRouteItem(ctx context.Context, repo Repository, gameRowID int64, item SaveRouteItem) (RouteRecord, domainpayment.Route, error) {
	payWayID := strings.TrimSpace(item.PayWayID)
	providerID := strings.TrimSpace(item.ProviderID)
	merchantID := strings.TrimSpace(item.MerchantAccountID)
	if payWayID == "" || providerID == "" || merchantID == "" {
		return RouteRecord{}, domainpayment.Route{}, errors.New("payWayId/providerId/merchantAccountId 必填")
	}
	payWay, err := repo.ResolvePayWay(ctx, payWayID)
	if err != nil {
		return RouteRecord{}, domainpayment.Route{}, mapRepoErr(err)
	}
	if !payWay.Enabled {
		return RouteRecord{}, domainpayment.Route{}, errors.New("payWay 已禁用")
	}
	provider, err := repo.ResolveProvider(ctx, providerID)
	if err != nil {
		return RouteRecord{}, domainpayment.Route{}, mapRepoErr(err)
	}
	if !provider.Enabled {
		return RouteRecord{}, domainpayment.Route{}, errors.New("provider 已禁用")
	}
	merchant, err := repo.ResolveMerchantAccount(ctx, merchantID)
	if err != nil {
		return RouteRecord{}, domainpayment.Route{}, mapRepoErr(err)
	}
	if !merchant.Enabled {
		return RouteRecord{}, domainpayment.Route{}, errors.New("merchantAccount 已禁用")
	}
	if merchant.ProviderID != provider.ProviderID {
		return RouteRecord{}, domainpayment.Route{}, errors.New("merchantAccount 与 provider 不匹配")
	}

	packageCode := normalizeNullableString(item.Package)
	channelID := normalizeNullableString(item.Channel)
	marketCode := normalizeUpperNullableString(item.Market)
	countryCode := normalizeUpperNullableString(item.Country)
	currency := normalizeUpperNullableString(item.Currency)
	if !isAllowedMarket(marketCode) {
		return RouteRecord{}, domainpayment.Route{}, errors.New("marketCode 非法")
	}

	var packageIDRef *int64
	if packageCode != "" {
		ref, enabled, err := repo.ResolvePackage(ctx, gameRowID, packageCode)
		if err != nil {
			return RouteRecord{}, domainpayment.Route{}, mapRepoErr(err)
		}
		if !enabled {
			return RouteRecord{}, domainpayment.Route{}, errors.New("package 已禁用")
		}
		packageIDRef = &ref
	}

	var channelIDRef *int64
	if channelID != "" {
		ref, enabled, err := repo.ResolveChannel(ctx, channelID)
		if err != nil {
			return RouteRecord{}, domainpayment.Route{}, mapRepoErr(err)
		}
		if !enabled {
			return RouteRecord{}, domainpayment.Route{}, errors.New("channel 已禁用")
		}
		channelIDRef = &ref
	}

	priority := 100
	if item.Priority != nil {
		priority = *item.Priority
	}
	enabled := true
	if item.Enabled != nil {
		enabled = *item.Enabled
	}

	record := RouteRecord{
		MarketCode:           emptyToWildcard(marketCode),
		CountryCode:          emptyToWildcard(countryCode),
		Currency:             emptyToWildcard(currency),
		ChannelIDRef:         channelIDRef,
		PackageIDRef:         packageIDRef,
		PayWayIDRef:          payWay.RowID,
		ProviderIDRef:        provider.RowID,
		MerchantAccountIDRef: merchant.RowID,
		Priority:             priority,
		Enabled:              enabled,
	}
	route := domainpayment.Route{
		GameID:          "",
		Package:         packageCode,
		Channel:         channelID,
		Market:          marketCode,
		Country:         countryCode,
		Currency:        currency,
		PayWay:          payWay.PayWayID,
		Provider:        provider.ProviderID,
		MerchantAccount: merchant.MerchantAccountID,
		Priority:        priority,
		Enabled:         enabled,
	}
	return record, route, nil
}

func (s *Service) writeAudit(ctx context.Context, entry AuditEntry) {
	if s.audit == nil {
		return
	}
	if ac, ok := adminapp.AuthContextFrom(ctx); ok {
		entry.ActorID = ac.UserID
	}
	_ = s.audit.Write(ctx, entry)
}

func mapRepoErr(err error) error {
	var appErr *Error
	if errors.As(err, &appErr) {
		return appErr
	}
	if errors.Is(err, adminapp.ErrNotFound) {
		return notFoundErr("资源不存在")
	}
	if errors.Is(err, adminapp.ErrConflict) {
		return conflictErr("资源冲突")
	}
	return err
}

func normalizeFilter(in ListFilter) ListFilter {
	if in.Page <= 0 {
		in.Page = 1
	}
	if in.PageSize <= 0 {
		in.PageSize = 20
	}
	if in.PageSize > 100 {
		in.PageSize = 100
	}
	in.Type = strings.TrimSpace(in.Type)
	in.Kind = strings.TrimSpace(in.Kind)
	in.ProviderID = strings.TrimSpace(in.ProviderID)
	in.SubjectID = strings.TrimSpace(in.SubjectID)
	return in
}

func normalizeNullableString(v *string) string {
	if v == nil {
		return ""
	}
	trimmed := strings.TrimSpace(*v)
	if trimmed == "" || trimmed == "*" {
		return ""
	}
	return trimmed
}

func normalizeUpperNullableString(v *string) string {
	out := normalizeNullableString(v)
	if out == "" {
		return ""
	}
	return strings.ToUpper(out)
}

func emptyToWildcard(v string) string {
	if strings.TrimSpace(v) == "" {
		return "*"
	}
	return v
}

func wildcardToEmpty(v string) string {
	if strings.TrimSpace(v) == "*" {
		return ""
	}
	return v
}

func wildcardToNullable(v string) *string {
	trimmed := strings.TrimSpace(v)
	if trimmed == "" || trimmed == "*" {
		return nil
	}
	out := trimmed
	return &out
}

func selectorDisplayCode(v string) string {
	if strings.TrimSpace(v) == "" || strings.TrimSpace(v) == "*" {
		return "*"
	}
	return v
}

func cloneMap(in map[string]any) map[string]any {
	if len(in) == 0 {
		return map[string]any{}
	}
	out := make(map[string]any, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}

func isAllowedMarket(v string) bool {
	switch v {
	case "", "GLOBAL", "JP", "KR", "SEA", "HMT", "CN":
		return true
	default:
		return false
	}
}

func fieldDetail(field, reason string) any {
	return map[string]string{"field": field, "reason": reason}
}
