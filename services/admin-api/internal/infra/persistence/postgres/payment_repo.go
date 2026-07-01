package postgres

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	paymentapp "github.com/csw/console/services/admin-api/internal/app/payment"
	"github.com/csw/console/services/admin-api/internal/domain/accountauth"
)

type PaymentRepo struct {
	db DBTX
}

func (r *PaymentRepo) ListPayWays(ctx context.Context, filter paymentapp.ListFilter) ([]paymentapp.PayWayDTO, int, error) {
	where := []string{"1=1"}
	args := []any{}
	idx := 1
	if filter.Enabled != nil {
		where = append(where, fmt.Sprintf("enabled=$%d", idx))
		args = append(args, *filter.Enabled)
		idx++
	}
	if filter.Type != "" {
		where = append(where, fmt.Sprintf("pay_way_type=$%d", idx))
		args = append(args, filter.Type)
		idx++
	}
	whereSQL := strings.Join(where, " AND ")
	var total int
	if err := r.db.QueryRow(ctx, fmt.Sprintf(`SELECT COUNT(*) FROM platform.pay_ways WHERE %s`, whereSQL), args...).Scan(&total); err != nil {
		return nil, 0, mapErr(err)
	}
	args = append(args, filter.PageSize, (filter.Page-1)*filter.PageSize)
	rows, err := r.db.Query(ctx, fmt.Sprintf(`
SELECT pay_way_id, pay_way_name, pay_way_type, enabled, sort
FROM platform.pay_ways
WHERE %s
ORDER BY sort ASC, pay_way_id ASC
LIMIT $%d OFFSET $%d`, whereSQL, idx, idx+1), args...)
	if err != nil {
		return nil, 0, mapErr(err)
	}
	defer rows.Close()
	out := make([]paymentapp.PayWayDTO, 0)
	for rows.Next() {
		var item paymentapp.PayWayDTO
		if err := rows.Scan(&item.PayWayID, &item.PayWayName, &item.PayWayType, &item.Enabled, &item.Sort); err != nil {
			return nil, 0, mapErr(err)
		}
		out = append(out, item)
	}
	return out, total, mapErr(rows.Err())
}

func (r *PaymentRepo) ListProviders(ctx context.Context, filter paymentapp.ListFilter) ([]paymentapp.ProviderDTO, int, error) {
	where := []string{"1=1"}
	args := []any{}
	idx := 1
	if filter.Enabled != nil {
		where = append(where, fmt.Sprintf("enabled=$%d", idx))
		args = append(args, *filter.Enabled)
		idx++
	}
	if filter.Kind != "" {
		where = append(where, fmt.Sprintf("provider_kind=$%d", idx))
		args = append(args, filter.Kind)
		idx++
	}
	whereSQL := strings.Join(where, " AND ")
	var total int
	if err := r.db.QueryRow(ctx, fmt.Sprintf(`SELECT COUNT(*) FROM platform.cashier_providers WHERE %s`, whereSQL), args...).Scan(&total); err != nil {
		return nil, 0, mapErr(err)
	}
	args = append(args, filter.PageSize, (filter.Page-1)*filter.PageSize)
	rows, err := r.db.Query(ctx, fmt.Sprintf(`
SELECT provider_id, provider_name, provider_kind, enabled, sort
FROM platform.cashier_providers
WHERE %s
ORDER BY sort ASC, provider_id ASC
LIMIT $%d OFFSET $%d`, whereSQL, idx, idx+1), args...)
	if err != nil {
		return nil, 0, mapErr(err)
	}
	defer rows.Close()
	out := make([]paymentapp.ProviderDTO, 0)
	for rows.Next() {
		var item paymentapp.ProviderDTO
		if err := rows.Scan(&item.ProviderID, &item.ProviderName, &item.ProviderKind, &item.Enabled, &item.Sort); err != nil {
			return nil, 0, mapErr(err)
		}
		out = append(out, item)
	}
	return out, total, mapErr(rows.Err())
}

func (r *PaymentRepo) ListBillingSubjects(ctx context.Context, filter paymentapp.ListFilter) ([]paymentapp.BillingSubjectDTO, int, error) {
	where := []string{"1=1"}
	args := []any{}
	idx := 1
	if filter.Enabled != nil {
		where = append(where, fmt.Sprintf("enabled=$%d", idx))
		args = append(args, *filter.Enabled)
		idx++
	}
	whereSQL := strings.Join(where, " AND ")
	var total int
	if err := r.db.QueryRow(ctx, fmt.Sprintf(`SELECT COUNT(*) FROM platform.billing_subjects WHERE %s`, whereSQL), args...).Scan(&total); err != nil {
		return nil, 0, mapErr(err)
	}
	args = append(args, filter.PageSize, (filter.Page-1)*filter.PageSize)
	rows, err := r.db.Query(ctx, fmt.Sprintf(`
SELECT subject_id, subject_name, legal_entity_name, enabled
FROM platform.billing_subjects
WHERE %s
ORDER BY subject_id ASC
LIMIT $%d OFFSET $%d`, whereSQL, idx, idx+1), args...)
	if err != nil {
		return nil, 0, mapErr(err)
	}
	defer rows.Close()
	out := make([]paymentapp.BillingSubjectDTO, 0)
	for rows.Next() {
		var item paymentapp.BillingSubjectDTO
		if err := rows.Scan(&item.SubjectID, &item.SubjectName, &item.LegalEntityName, &item.Enabled); err != nil {
			return nil, 0, mapErr(err)
		}
		out = append(out, item)
	}
	return out, total, mapErr(rows.Err())
}

func (r *PaymentRepo) CreateBillingSubject(ctx context.Context, in paymentapp.BillingSubjectRecord) (paymentapp.BillingSubjectDTO, error) {
	var out paymentapp.BillingSubjectDTO
	err := r.db.QueryRow(ctx, `
INSERT INTO platform.billing_subjects (subject_id, subject_name, legal_entity_name, enabled)
VALUES ($1,$2,$3,$4)
RETURNING subject_id, subject_name, legal_entity_name, enabled`,
		in.SubjectID, in.SubjectName, in.LegalEntityName, in.Enabled,
	).Scan(&out.SubjectID, &out.SubjectName, &out.LegalEntityName, &out.Enabled)
	if err != nil {
		return paymentapp.BillingSubjectDTO{}, mapErr(err)
	}
	return out, nil
}

func (r *PaymentRepo) ListMerchantAccounts(ctx context.Context, filter paymentapp.ListFilter) ([]paymentapp.MerchantAccountDTO, int, error) {
	where := []string{"1=1"}
	args := []any{}
	idx := 1
	if filter.Enabled != nil {
		where = append(where, fmt.Sprintf("ma.enabled=$%d", idx))
		args = append(args, *filter.Enabled)
		idx++
	}
	if filter.ProviderID != "" {
		where = append(where, fmt.Sprintf("p.provider_id=$%d", idx))
		args = append(args, filter.ProviderID)
		idx++
	}
	if filter.SubjectID != "" {
		where = append(where, fmt.Sprintf("s.subject_id=$%d", idx))
		args = append(args, filter.SubjectID)
		idx++
	}
	whereSQL := strings.Join(where, " AND ")
	var total int
	if err := r.db.QueryRow(ctx, fmt.Sprintf(`
SELECT COUNT(*)
FROM platform.cashier_merchant_accounts ma
JOIN platform.cashier_providers p ON p.id=ma.provider_id_ref
JOIN platform.billing_subjects s ON s.id=ma.subject_id_ref
WHERE %s`, whereSQL), args...).Scan(&total); err != nil {
		return nil, 0, mapErr(err)
	}
	args = append(args, filter.PageSize, (filter.Page-1)*filter.PageSize)
	rows, err := r.db.Query(ctx, fmt.Sprintf(`
SELECT ma.merchant_account_id, p.provider_id, s.subject_id, ma.merchant_id, ma.merchant_name, ma.config_json, ma.enabled
FROM platform.cashier_merchant_accounts ma
JOIN platform.cashier_providers p ON p.id=ma.provider_id_ref
JOIN platform.billing_subjects s ON s.id=ma.subject_id_ref
WHERE %s
ORDER BY ma.merchant_account_id ASC
LIMIT $%d OFFSET $%d`, whereSQL, idx, idx+1), args...)
	if err != nil {
		return nil, 0, mapErr(err)
	}
	defer rows.Close()
	out := make([]paymentapp.MerchantAccountDTO, 0)
	for rows.Next() {
		var item paymentapp.MerchantAccountDTO
		var configRaw []byte
		if err := rows.Scan(
			&item.MerchantAccountID, &item.ProviderID, &item.SubjectID,
			&item.MerchantID, &item.MerchantName, &configRaw, &item.Enabled,
		); err != nil {
			return nil, 0, mapErr(err)
		}
		item.ConfigJSON = map[string]any{}
		if len(configRaw) > 0 {
			if err := json.Unmarshal(configRaw, &item.ConfigJSON); err != nil {
				return nil, 0, err
			}
		}
		out = append(out, item)
	}
	return out, total, mapErr(rows.Err())
}

func (r *PaymentRepo) CreateMerchantAccount(ctx context.Context, in paymentapp.MerchantAccountRecord) (paymentapp.MerchantAccountDTO, error) {
	configPayload := "{}"
	if len(in.ConfigJSON) > 0 {
		raw, err := json.Marshal(in.ConfigJSON)
		if err != nil {
			return paymentapp.MerchantAccountDTO{}, err
		}
		configPayload = string(raw)
	}
	var out paymentapp.MerchantAccountDTO
	var configRaw []byte
	err := r.db.QueryRow(ctx, `
INSERT INTO platform.cashier_merchant_accounts
  (merchant_account_id, provider_id_ref, subject_id_ref, merchant_id, merchant_name, config_json, secret_ciphertext, enabled)
VALUES ($1,$2,$3,$4,$5,$6::jsonb,$7,$8)
RETURNING merchant_account_id, merchant_id, merchant_name, config_json, enabled`,
		in.MerchantAccountID, in.ProviderRowID, in.SubjectRowID, in.MerchantID, in.MerchantName, configPayload, in.SecretCiphertext, in.Enabled,
	).Scan(&out.MerchantAccountID, &out.MerchantID, &out.MerchantName, &configRaw, &out.Enabled)
	if err != nil {
		return paymentapp.MerchantAccountDTO{}, mapErr(err)
	}
	out.ProviderID = in.ProviderID
	out.SubjectID = in.SubjectID
	out.ConfigJSON = map[string]any{}
	if len(configRaw) > 0 {
		if err := json.Unmarshal(configRaw, &out.ConfigJSON); err != nil {
			return paymentapp.MerchantAccountDTO{}, err
		}
	}
	return out, nil
}

func (r *PaymentRepo) GetLatestProviderTemplate(ctx context.Context, providerRowID int64) (accountauth.Template, error) {
	var (
		versionRaw string
		formRaw    []byte
		secretRaw  []byte
		fileRaw    []byte
		rulesRaw   []byte
	)
	err := r.db.QueryRow(ctx, `
SELECT template_version, form_schema_json, secret_fields_json, file_fields_json, validation_rules_json
FROM platform.cashier_provider_templates
WHERE provider_id_ref=$1 AND enabled=TRUE
ORDER BY template_version DESC
LIMIT 1`, providerRowID).Scan(&versionRaw, &formRaw, &secretRaw, &fileRaw, &rulesRaw)
	if err != nil {
		return accountauth.Template{}, mapErr(err)
	}
	tpl := accountauth.Template{
		TemplateVersion: versionRaw,
		FormSchema:      []accountauth.FormField{},
		SecretFields:    []string{},
		FileFields:      []accountauth.FileField{},
		ValidationRules: map[string]accountauth.ValidationRule{},
	}
	if len(formRaw) > 0 {
		if err := json.Unmarshal(formRaw, &tpl.FormSchema); err != nil {
			return accountauth.Template{}, err
		}
	}
	if len(secretRaw) > 0 {
		if err := json.Unmarshal(secretRaw, &tpl.SecretFields); err != nil {
			return accountauth.Template{}, err
		}
	}
	if len(fileRaw) > 0 {
		if err := json.Unmarshal(fileRaw, &tpl.FileFields); err != nil {
			return accountauth.Template{}, err
		}
	}
	if len(rulesRaw) > 0 {
		if err := json.Unmarshal(rulesRaw, &tpl.ValidationRules); err != nil {
			return accountauth.Template{}, err
		}
	}
	return tpl, nil
}

func (r *PaymentRepo) ResolveGameRowID(ctx context.Context, gameID string) (int64, error) {
	var id int64
	if err := r.db.QueryRow(ctx, `SELECT id FROM games WHERE game_id=$1`, gameID).Scan(&id); err != nil {
		return 0, mapErr(err)
	}
	return id, nil
}

func (r *PaymentRepo) ResolvePayWay(ctx context.Context, payWayID string) (paymentapp.PayWayRef, error) {
	var out paymentapp.PayWayRef
	err := r.db.QueryRow(ctx, `
SELECT id, pay_way_id, enabled
FROM platform.pay_ways
WHERE pay_way_id=$1`, payWayID).Scan(&out.RowID, &out.PayWayID, &out.Enabled)
	if err != nil {
		return paymentapp.PayWayRef{}, mapErr(err)
	}
	return out, nil
}

func (r *PaymentRepo) ResolveProvider(ctx context.Context, providerID string) (paymentapp.ProviderRef, error) {
	var out paymentapp.ProviderRef
	err := r.db.QueryRow(ctx, `
SELECT id, provider_id, enabled
FROM platform.cashier_providers
WHERE provider_id=$1`, providerID).Scan(&out.RowID, &out.ProviderID, &out.Enabled)
	if err != nil {
		return paymentapp.ProviderRef{}, mapErr(err)
	}
	return out, nil
}

func (r *PaymentRepo) ResolveSubject(ctx context.Context, subjectID string) (paymentapp.SubjectRef, error) {
	var out paymentapp.SubjectRef
	err := r.db.QueryRow(ctx, `
SELECT id, subject_id, enabled
FROM platform.billing_subjects
WHERE subject_id=$1`, subjectID).Scan(&out.RowID, &out.SubjectID, &out.Enabled)
	if err != nil {
		return paymentapp.SubjectRef{}, mapErr(err)
	}
	return out, nil
}

func (r *PaymentRepo) ResolveMerchantAccount(ctx context.Context, merchantAccountID string) (paymentapp.MerchantAccountRef, error) {
	var out paymentapp.MerchantAccountRef
	err := r.db.QueryRow(ctx, `
SELECT ma.id, ma.merchant_account_id, p.provider_id, ma.enabled
FROM platform.cashier_merchant_accounts ma
JOIN platform.cashier_providers p ON p.id=ma.provider_id_ref
WHERE ma.merchant_account_id=$1`, merchantAccountID).
		Scan(&out.RowID, &out.MerchantAccountID, &out.ProviderID, &out.Enabled)
	if err != nil {
		return paymentapp.MerchantAccountRef{}, mapErr(err)
	}
	return out, nil
}

func (r *PaymentRepo) ResolveChannel(ctx context.Context, channelID string) (int64, bool, error) {
	var id int64
	var enabled bool
	err := r.db.QueryRow(ctx, `SELECT id, enabled FROM platform.channels WHERE channel_id=$1`, channelID).Scan(&id, &enabled)
	if err != nil {
		return 0, false, mapErr(err)
	}
	return id, enabled, nil
}

func (r *PaymentRepo) ResolvePackage(ctx context.Context, gameRowID int64, packageCode string) (int64, bool, error) {
	var (
		id      int64
		enabled bool
	)
	err := r.db.QueryRow(ctx, `
SELECT cp.id, cp.enabled
FROM channel_packages cp
JOIN game_channels gc ON gc.id = cp.game_channel_id_ref
WHERE gc.game_id_ref=$1 AND cp.package_code=$2`, gameRowID, packageCode).
		Scan(&id, &enabled)
	if err != nil {
		return 0, false, mapErr(err)
	}
	return id, enabled, nil
}

func (r *PaymentRepo) ReplaceGameRoutes(ctx context.Context, gameRowID int64, routes []paymentapp.RouteRecord) error {
	if _, err := r.db.Exec(ctx, `DELETE FROM payment_routes WHERE game_id_ref=$1`, gameRowID); err != nil {
		return mapErr(err)
	}
	for _, item := range routes {
		if _, err := r.db.Exec(ctx, `
INSERT INTO payment_routes
  (game_id_ref, market_code, country_code, currency, channel_id_ref, package_id_ref, pay_way_id_ref, provider_id_ref, merchant_account_id_ref, priority, enabled)
VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)`,
			gameRowID, item.MarketCode, item.CountryCode, item.Currency, item.ChannelIDRef, item.PackageIDRef,
			item.PayWayIDRef, item.ProviderIDRef, item.MerchantAccountIDRef, item.Priority, item.Enabled,
		); err != nil {
			return mapErr(err)
		}
	}
	return nil
}

func (r *PaymentRepo) ListGameRoutes(ctx context.Context, gameID string) ([]paymentapp.GameRouteRecord, error) {
	rows, err := r.db.Query(ctx, `
SELECT pr.id, g.game_id, pw.pay_way_id, pw.pay_way_name, pw.pay_way_type,
       COALESCE(cp.package_code, '*'), COALESCE(ch.channel_id, '*'),
       pr.market_code, pr.country_code, pr.currency,
       p.provider_id, ma.merchant_account_id,
       pr.priority, pr.enabled,
       pw.enabled, p.enabled, ma.enabled,
       COALESCE(ch.enabled, TRUE), COALESCE(cp.enabled, TRUE)
FROM payment_routes pr
JOIN games g ON g.id=pr.game_id_ref
JOIN platform.pay_ways pw ON pw.id=pr.pay_way_id_ref
JOIN platform.cashier_providers p ON p.id=pr.provider_id_ref
JOIN platform.cashier_merchant_accounts ma ON ma.id=pr.merchant_account_id_ref
LEFT JOIN channel_packages cp ON cp.id=pr.package_id_ref
LEFT JOIN platform.channels ch ON ch.id=pr.channel_id_ref
WHERE g.game_id=$1
ORDER BY pw.sort ASC, pw.pay_way_id ASC, pr.priority ASC, pr.id ASC`, gameID)
	if err != nil {
		return nil, mapErr(err)
	}
	defer rows.Close()
	out := make([]paymentapp.GameRouteRecord, 0)
	for rows.Next() {
		var item paymentapp.GameRouteRecord
		if err := rows.Scan(
			&item.ID, &item.GameID, &item.PayWayID, &item.PayWayName, &item.PayWayType,
			&item.PackageCode, &item.ChannelID, &item.MarketCode, &item.CountryCode, &item.Currency,
			&item.ProviderID, &item.MerchantAccountID, &item.Priority, &item.Enabled,
			&item.PayWayEnabled, &item.ProviderEnabled, &item.MerchantEnabled,
			&item.ChannelEnabled, &item.PackageEnabled,
		); err != nil {
			return nil, mapErr(err)
		}
		out = append(out, item)
	}
	return out, mapErr(rows.Err())
}

func (r *PaymentRepo) ListEnabledRoutes(ctx context.Context, gameID, payWayID string) ([]paymentapp.ResolvedRouteRecord, error) {
	rows, err := r.db.Query(ctx, `
SELECT pr.id,
       COALESCE(cp.package_code, '*'),
       COALESCE(ch.channel_id, '*'),
       pr.market_code, pr.country_code, pr.currency,
       pw.pay_way_id, p.provider_id, ma.merchant_account_id,
       pr.priority, pr.enabled, p.enabled, ma.enabled
FROM payment_routes pr
JOIN games g ON g.id=pr.game_id_ref
JOIN platform.pay_ways pw ON pw.id=pr.pay_way_id_ref
JOIN platform.cashier_providers p ON p.id=pr.provider_id_ref
JOIN platform.cashier_merchant_accounts ma ON ma.id=pr.merchant_account_id_ref
LEFT JOIN channel_packages cp ON cp.id=pr.package_id_ref
LEFT JOIN platform.channels ch ON ch.id=pr.channel_id_ref
WHERE g.game_id=$1 AND pw.pay_way_id=$2 AND pr.enabled=TRUE
ORDER BY pr.id ASC`, gameID, payWayID)
	if err != nil {
		return nil, mapErr(err)
	}
	defer rows.Close()
	out := make([]paymentapp.ResolvedRouteRecord, 0)
	for rows.Next() {
		var item paymentapp.ResolvedRouteRecord
		if err := rows.Scan(
			&item.ID, &item.PackageCode, &item.ChannelID, &item.MarketCode, &item.CountryCode, &item.Currency,
			&item.PayWayID, &item.ProviderID, &item.MerchantAccountID, &item.Priority, &item.Enabled,
			&item.ProviderEnabled, &item.MerchantEnabled,
		); err != nil {
			return nil, mapErr(err)
		}
		out = append(out, item)
	}
	return out, mapErr(rows.Err())
}
