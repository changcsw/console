package postgres

import (
	"context"
	"encoding/json"
	"fmt"
	"slices"
	"strconv"
	"strings"
	"time"

	snapshotapp "github.com/csw/console/services/admin-api/internal/app/snapshot"
	"github.com/csw/console/services/admin-api/internal/domain/common"
	domainsnapshot "github.com/csw/console/services/admin-api/internal/domain/snapshot"
)

type SnapshotRepo struct {
	db DBTX
}

func (r *SnapshotRepo) ResolveGameRowID(ctx context.Context, gameID string) (int64, error) {
	var id int64
	err := r.db.QueryRow(ctx, `SELECT id FROM games WHERE game_id=$1`, strings.TrimSpace(gameID)).Scan(&id)
	if err != nil {
		return 0, mapErr(err)
	}
	return id, nil
}

func (r *SnapshotRepo) LoadValidData(ctx context.Context, gameIDRef int64, gameID string, generatedAt time.Time) (domainsnapshot.ValidDataView, []string, error) {
	legalLinks, err := r.loadLegalLinks(ctx, gameIDRef)
	if err != nil {
		return domainsnapshot.ValidDataView{}, nil, err
	}
	accountAuth, err := r.loadAccountAuth(ctx, gameIDRef)
	if err != nil {
		return domainsnapshot.ValidDataView{}, nil, err
	}
	products, err := r.loadProducts(ctx, gameIDRef)
	if err != nil {
		return domainsnapshot.ValidDataView{}, nil, err
	}
	cashier, err := r.loadCashierProfile(ctx, gameIDRef)
	if err != nil {
		return domainsnapshot.ValidDataView{}, nil, err
	}
	channels, err := r.loadChannels(ctx, gameIDRef)
	if err != nil {
		return domainsnapshot.ValidDataView{}, nil, err
	}
	payWays, err := r.loadPayWays(ctx, gameIDRef)
	if err != nil {
		return domainsnapshot.ValidDataView{}, nil, err
	}

	return domainsnapshot.ValidDataView{
		GameID:      gameID,
		GameIDRef:   gameIDRef,
		GeneratedAt: generatedAt,
		LegalLinks:  legalLinks,
		AccountAuth: accountAuth,
		Products:    products,
		Cashier:     cashier,
		Channels:    channels,
	}, payWays, nil
}

func (r *SnapshotRepo) CreateSnapshot(ctx context.Context, in snapshotapp.CreateSnapshotInput) (domainsnapshot.ConfigSnapshot, error) {
	payload, err := json.Marshal(in.ConfigJSON)
	if err != nil {
		return domainsnapshot.ConfigSnapshot{}, err
	}
	var row domainsnapshot.ConfigSnapshot
	var status string
	var rawConfig []byte
	err = r.db.QueryRow(ctx, `
INSERT INTO game_config_snapshots
  (game_id_ref, config_schema_version, config_version, config_json, file_name, file_hash, storage_key, status, generated_at)
VALUES ($1,$2,$3,$4::jsonb,$5,$6,$7,'draft',$8)
RETURNING id, game_id_ref, config_schema_version, config_version, config_json, file_name, file_hash, storage_key, status, generated_at, published_at, created_at, updated_at`,
		in.GameIDRef, in.ConfigSchemaVersion, in.ConfigVersion, string(payload), in.FileName, in.FileHash, in.StorageKey, in.GeneratedAt,
	).Scan(
		&row.ID, &row.GameIDRef, &row.ConfigSchemaVersion, &row.ConfigVersion, &rawConfig, &row.FileName, &row.FileHash,
		&row.StorageKey, &status, &row.GeneratedAt, &row.PublishedAt, &row.CreatedAt, &row.UpdatedAt,
	)
	if err != nil {
		return domainsnapshot.ConfigSnapshot{}, mapErr(err)
	}
	row.Status = domainsnapshot.SnapshotStatus(status)
	row.ConfigJSON = map[string]any{}
	if len(rawConfig) > 0 {
		if err := json.Unmarshal(rawConfig, &row.ConfigJSON); err != nil {
			return domainsnapshot.ConfigSnapshot{}, err
		}
	}
	return row, nil
}

func (r *SnapshotRepo) ListSnapshots(ctx context.Context, gameID string, filter snapshotapp.ListFilter) ([]domainsnapshot.ConfigSnapshot, int, error) {
	var total int
	if err := r.db.QueryRow(ctx, `
SELECT COUNT(*)
FROM game_config_snapshots s
JOIN games g ON g.id=s.game_id_ref
WHERE g.game_id=$1`, gameID).Scan(&total); err != nil {
		return nil, 0, mapErr(err)
	}
	rows, err := r.db.Query(ctx, `
SELECT s.id, s.game_id_ref, s.config_schema_version, s.config_version, s.config_json, s.file_name, s.file_hash, s.storage_key, s.status, s.generated_at, s.published_at, s.created_at, s.updated_at
FROM game_config_snapshots s
JOIN games g ON g.id=s.game_id_ref
WHERE g.game_id=$1
ORDER BY s.generated_at DESC, s.id DESC
LIMIT $2 OFFSET $3`, gameID, filter.PageSize, (filter.Page-1)*filter.PageSize)
	if err != nil {
		return nil, 0, mapErr(err)
	}
	defer rows.Close()
	out := make([]domainsnapshot.ConfigSnapshot, 0)
	for rows.Next() {
		item, err := scanSnapshot(rows)
		if err != nil {
			return nil, 0, err
		}
		out = append(out, item)
	}
	return out, total, mapErr(rows.Err())
}

func (r *SnapshotRepo) GetSnapshot(ctx context.Context, snapshotID int64) (domainsnapshot.ConfigSnapshot, error) {
	rows, err := r.db.Query(ctx, `
SELECT id, game_id_ref, config_schema_version, config_version, config_json, file_name, file_hash, storage_key, status, generated_at, published_at, created_at, updated_at
FROM game_config_snapshots
WHERE id=$1`, snapshotID)
	if err != nil {
		return domainsnapshot.ConfigSnapshot{}, mapErr(err)
	}
	defer rows.Close()
	if !rows.Next() {
		return domainsnapshot.ConfigSnapshot{}, mapErr(errNoRows())
	}
	item, err := scanSnapshot(rows)
	if err != nil {
		return domainsnapshot.ConfigSnapshot{}, err
	}
	return item, nil
}

func (r *SnapshotRepo) PublishSnapshot(ctx context.Context, snapshotID int64, publishedAt time.Time) (domainsnapshot.ConfigSnapshot, error) {
	rows, err := r.db.Query(ctx, `
UPDATE game_config_snapshots
SET status='published', published_at=$2, updated_at=NOW()
WHERE id=$1 AND status='draft'
RETURNING id, game_id_ref, config_schema_version, config_version, config_json, file_name, file_hash, storage_key, status, generated_at, published_at, created_at, updated_at`,
		snapshotID, publishedAt)
	if err != nil {
		return domainsnapshot.ConfigSnapshot{}, mapErr(err)
	}
	defer rows.Close()
	if !rows.Next() {
		return domainsnapshot.ConfigSnapshot{}, mapErr(errNoRows())
	}
	return scanSnapshot(rows)
}

func scanSnapshot(rows interface {
	Scan(dest ...any) error
}) (domainsnapshot.ConfigSnapshot, error) {
	var item domainsnapshot.ConfigSnapshot
	var status string
	var raw []byte
	if err := rows.Scan(
		&item.ID, &item.GameIDRef, &item.ConfigSchemaVersion, &item.ConfigVersion, &raw, &item.FileName, &item.FileHash,
		&item.StorageKey, &status, &item.GeneratedAt, &item.PublishedAt, &item.CreatedAt, &item.UpdatedAt,
	); err != nil {
		return domainsnapshot.ConfigSnapshot{}, err
	}
	item.Status = domainsnapshot.SnapshotStatus(status)
	item.ConfigJSON = map[string]any{}
	if len(raw) > 0 {
		if err := json.Unmarshal(raw, &item.ConfigJSON); err != nil {
			return domainsnapshot.ConfigSnapshot{}, err
		}
	}
	return item, nil
}

func (r *SnapshotRepo) loadLegalLinks(ctx context.Context, gameIDRef int64) ([]domainsnapshot.LegalLink, error) {
	rows, err := r.db.Query(ctx, `
SELECT scope_type, scope_value, terms_url, privacy_url, delete_account_url
FROM game_legal_links
WHERE game_id_ref=$1
ORDER BY scope_type ASC, scope_value ASC`, gameIDRef)
	if err != nil {
		return nil, mapErr(err)
	}
	defer rows.Close()
	out := make([]domainsnapshot.LegalLink, 0)
	for rows.Next() {
		var item domainsnapshot.LegalLink
		if err := rows.Scan(&item.ScopeType, &item.ScopeValue, &item.TermsURL, &item.PrivacyURL, &item.DeleteAccountURL); err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, mapErr(rows.Err())
}

func (r *SnapshotRepo) loadAccountAuth(ctx context.Context, gameIDRef int64) ([]domainsnapshot.AccountAuthItem, error) {
	rows, err := r.db.Query(ctx, `
SELECT t.auth_type_id, cfg.config_json, tpl.form_schema_json, tpl.secret_fields_json
FROM game_account_auth_configs cfg
JOIN platform.account_auth_types t ON t.id=cfg.auth_type_id_ref AND t.enabled=TRUE
JOIN LATERAL (
  SELECT form_schema_json, secret_fields_json
  FROM platform.account_auth_templates atpl
  WHERE atpl.auth_type_id_ref=t.id AND atpl.enabled=TRUE
  ORDER BY atpl.template_version DESC
  LIMIT 1
) tpl ON TRUE
WHERE cfg.game_id_ref=$1 AND cfg.enabled=TRUE AND cfg.config_status='valid'
ORDER BY t.auth_type_id ASC`, gameIDRef)
	if err != nil {
		return nil, mapErr(err)
	}
	defer rows.Close()

	out := make([]domainsnapshot.AccountAuthItem, 0)
	for rows.Next() {
		var (
			item      domainsnapshot.AccountAuthItem
			rawConfig []byte
			rawForm   []byte
			rawSecret []byte
		)
		if err := rows.Scan(&item.AuthTypeID, &rawConfig, &rawForm, &rawSecret); err != nil {
			return nil, err
		}
		if err := json.Unmarshal(rawConfig, &item.Config); err != nil {
			return nil, err
		}
		item.FormSchema = parseScopeFields(rawForm)
		item.SecretFields = parseStringArray(rawSecret)
		out = append(out, item)
	}
	return out, mapErr(rows.Err())
}

func (r *SnapshotRepo) loadProducts(ctx context.Context, gameIDRef int64) ([]domainsnapshot.ProductItem, error) {
	rows, err := r.db.Query(ctx, `
SELECT product_id, price_id, base_currency, base_amount_minor
FROM products
WHERE game_id_ref=$1 AND enabled=TRUE
ORDER BY product_id ASC`, gameIDRef)
	if err != nil {
		return nil, mapErr(err)
	}
	defer rows.Close()
	out := make([]domainsnapshot.ProductItem, 0)
	for rows.Next() {
		var item domainsnapshot.ProductItem
		if err := rows.Scan(&item.ProductID, &item.EffectivePriceID, &item.Currency, &item.AmountMinor); err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, mapErr(rows.Err())
}

func (r *SnapshotRepo) loadCashierProfile(ctx context.Context, gameIDRef int64) (*domainsnapshot.CashierProfile, error) {
	rows, err := r.db.Query(ctx, `
SELECT t.template_id, v.version, p.snapshot_checksum
FROM game_cashier_profiles p
JOIN platform.cashier_price_templates t ON t.id=p.template_id_ref
JOIN platform.cashier_price_template_versions v ON v.id=p.applied_template_version_id
WHERE p.game_id_ref=$1`, gameIDRef)
	if err != nil {
		return nil, mapErr(err)
	}
	defer rows.Close()
	if !rows.Next() {
		return nil, nil
	}
	var profile domainsnapshot.CashierProfile
	if err := rows.Scan(&profile.TemplateID, &profile.TemplateVersion, &profile.SnapshotChecksum); err != nil {
		return nil, err
	}
	return &profile, mapErr(rows.Err())
}

func (r *SnapshotRepo) loadChannels(ctx context.Context, gameIDRef int64) ([]domainsnapshot.ChannelInput, error) {
	rows, err := r.db.Query(ctx, `
SELECT gc.id, ch.channel_id, ch.region, gc.market_code, gc.hidden, gc.enabled, gc.config_status
FROM game_channels gc
JOIN platform.channels ch ON ch.id=gc.channel_id_ref AND ch.enabled=TRUE
WHERE gc.game_id_ref=$1
ORDER BY gc.market_code ASC, ch.channel_id ASC`, gameIDRef)
	if err != nil {
		return nil, mapErr(err)
	}
	defer rows.Close()

	channelIDs := make([]int64, 0)
	channels := make([]domainsnapshot.ChannelInput, 0)
	for rows.Next() {
		var (
			item       domainsnapshot.ChannelInput
			id         int64
			marketCode string
			status     string
		)
		if err := rows.Scan(&id, &item.ChannelID, &item.Region, &marketCode, &item.Hidden, &item.Enabled, &status); err != nil {
			return nil, err
		}
		item.Market = common.Market(strings.ToUpper(marketCode))
		item.ConfigStatus = common.ConfigStatus(status)
		channels = append(channels, item)
		channelIDs = append(channelIDs, id)
	}
	if err := mapErr(rows.Err()); err != nil {
		return nil, err
	}
	if len(channelIDs) == 0 {
		return channels, nil
	}

	loginMap, err := r.loadChannelLogin(ctx, channelIDs)
	if err != nil {
		return nil, err
	}
	iapMap, err := r.loadChannelIAP(ctx, channelIDs)
	if err != nil {
		return nil, err
	}
	pkgMap, err := r.loadPackages(ctx, channelIDs)
	if err != nil {
		return nil, err
	}
	pluginMap, err := r.loadPlugins(ctx, channelIDs)
	if err != nil {
		return nil, err
	}

	for i, id := range channelIDs {
		channels[i].Login = loginMap[id]
		channels[i].IAP = iapMap[id]
		channels[i].Packages = pkgMap[id]
		channels[i].Plugins = pluginMap[id]
	}
	return channels, nil
}

func (r *SnapshotRepo) loadChannelLogin(ctx context.Context, channelIDs []int64) (map[int64]*domainsnapshot.TemplateConfig, error) {
	idsArg := joinInt64(channelIDs)
	rows, err := r.db.Query(ctx, fmt.Sprintf(`
SELECT gc.id, cfg.enabled, cfg.config_status, cfg.config_json, tpl.form_schema_json, tpl.secret_fields_json
FROM game_channels gc
LEFT JOIN game_channel_login_configs cfg ON cfg.game_channel_id_ref=gc.id
LEFT JOIN LATERAL (
  SELECT form_schema_json, secret_fields_json
  FROM platform.channel_login_templates t
  WHERE t.channel_id_ref=gc.channel_id_ref AND t.enabled=TRUE
  ORDER BY t.template_version DESC
  LIMIT 1
) tpl ON TRUE
WHERE gc.id IN (%s)`, idsArg))
	if err != nil {
		return nil, mapErr(err)
	}
	defer rows.Close()
	out := make(map[int64]*domainsnapshot.TemplateConfig, len(channelIDs))
	for rows.Next() {
		var (
			id        int64
			enabled   *bool
			status    *string
			rawConfig []byte
			rawForm   []byte
			rawSecret []byte
		)
		if err := rows.Scan(&id, &enabled, &status, &rawConfig, &rawForm, &rawSecret); err != nil {
			return nil, err
		}
		if enabled == nil || status == nil {
			continue
		}
		cfg := &domainsnapshot.TemplateConfig{
			Enabled:      *enabled,
			ConfigStatus: common.ConfigStatus(*status),
			Config:       map[string]any{},
			FormSchema:   parseScopeFields(rawForm),
			SecretFields: parseStringArray(rawSecret),
		}
		if len(rawConfig) > 0 {
			if err := json.Unmarshal(rawConfig, &cfg.Config); err != nil {
				return nil, err
			}
		}
		out[id] = cfg
	}
	return out, mapErr(rows.Err())
}

func (r *SnapshotRepo) loadChannelIAP(ctx context.Context, channelIDs []int64) (map[int64]*domainsnapshot.TemplateConfig, error) {
	idsArg := joinInt64(channelIDs)
	rows, err := r.db.Query(ctx, fmt.Sprintf(`
SELECT gc.id, cfg.enabled, cfg.config_status, cfg.config_json, tpl.form_schema_json, tpl.secret_fields_json
FROM game_channels gc
LEFT JOIN game_channel_iap_configs cfg ON cfg.game_channel_id_ref=gc.id
LEFT JOIN LATERAL (
  SELECT form_schema_json, secret_fields_json
  FROM platform.channel_iap_templates t
  WHERE t.channel_id_ref=gc.channel_id_ref AND t.enabled=TRUE
  ORDER BY t.template_version DESC
  LIMIT 1
) tpl ON TRUE
WHERE gc.id IN (%s)`, idsArg))
	if err != nil {
		return nil, mapErr(err)
	}
	defer rows.Close()
	out := make(map[int64]*domainsnapshot.TemplateConfig, len(channelIDs))
	for rows.Next() {
		var (
			id        int64
			enabled   *bool
			status    *string
			rawConfig []byte
			rawForm   []byte
			rawSecret []byte
		)
		if err := rows.Scan(&id, &enabled, &status, &rawConfig, &rawForm, &rawSecret); err != nil {
			return nil, err
		}
		if enabled == nil || status == nil {
			continue
		}
		cfg := &domainsnapshot.TemplateConfig{
			Enabled:      *enabled,
			ConfigStatus: common.ConfigStatus(*status),
			Config:       map[string]any{},
			FormSchema:   parseScopeFields(rawForm),
			SecretFields: parseStringArray(rawSecret),
		}
		if len(rawConfig) > 0 {
			if err := json.Unmarshal(rawConfig, &cfg.Config); err != nil {
				return nil, err
			}
		}
		out[id] = cfg
	}
	return out, mapErr(rows.Err())
}

func (r *SnapshotRepo) loadPackages(ctx context.Context, channelIDs []int64) (map[int64][]domainsnapshot.PackageConfig, error) {
	idsArg := joinInt64(channelIDs)
	rows, err := r.db.Query(ctx, fmt.Sprintf(`
SELECT game_channel_id_ref, package_code, bundle_id, enabled
FROM channel_packages
WHERE game_channel_id_ref IN (%s)
ORDER BY game_channel_id_ref ASC, package_code ASC`, idsArg))
	if err != nil {
		return nil, mapErr(err)
	}
	defer rows.Close()
	out := make(map[int64][]domainsnapshot.PackageConfig, len(channelIDs))
	for rows.Next() {
		var (
			id   int64
			item domainsnapshot.PackageConfig
		)
		if err := rows.Scan(&id, &item.PackageCode, &item.BundleID, &item.Enabled); err != nil {
			return nil, err
		}
		out[id] = append(out[id], item)
	}
	return out, mapErr(rows.Err())
}

func (r *SnapshotRepo) loadPlugins(ctx context.Context, channelIDs []int64) (map[int64][]domainsnapshot.PluginConfig, error) {
	idsArg := joinInt64(channelIDs)
	rows, err := r.db.Query(ctx, fmt.Sprintf(`
SELECT cfg.game_channel_id_ref, fp.plugin_id, fp.plugin_name, fp.region, cfp.required,
       cfg.enabled, cfg.config_status, cfg.config_json, tpl.form_schema_json, tpl.secret_fields_json, EXTRACT(EPOCH FROM cfg.updated_at)::bigint
FROM game_channel_plugin_configs cfg
JOIN platform.feature_plugins fp ON fp.id=cfg.plugin_id_ref AND fp.enabled=TRUE
JOIN platform.channel_feature_plugins cfp ON cfp.plugin_id_ref=cfg.plugin_id_ref
JOIN game_channels gc ON gc.id=cfg.game_channel_id_ref AND gc.channel_id_ref=cfp.channel_id_ref
LEFT JOIN LATERAL (
  SELECT form_schema_json, secret_fields_json
  FROM platform.feature_plugin_templates t
  WHERE t.plugin_id_ref=cfg.plugin_id_ref AND t.enabled=TRUE
  ORDER BY t.template_version DESC
  LIMIT 1
) tpl ON TRUE
WHERE cfg.game_channel_id_ref IN (%s)
ORDER BY cfg.game_channel_id_ref ASC, fp.plugin_id ASC`, idsArg))
	if err != nil {
		return nil, mapErr(err)
	}
	defer rows.Close()
	out := make(map[int64][]domainsnapshot.PluginConfig, len(channelIDs))
	for rows.Next() {
		var (
			id        int64
			item      domainsnapshot.PluginConfig
			status    string
			rawConfig []byte
			rawForm   []byte
			rawSecret []byte
		)
		if err := rows.Scan(
			&id, &item.PluginID, &item.PluginName, &item.Region, &item.Required, &item.Enabled, &status,
			&rawConfig, &rawForm, &rawSecret, &item.UpdatedAtUnix,
		); err != nil {
			return nil, err
		}
		item.ConfigStatus = common.ConfigStatus(status)
		item.Config = map[string]any{}
		if len(rawConfig) > 0 {
			if err := json.Unmarshal(rawConfig, &item.Config); err != nil {
				return nil, err
			}
		}
		item.FormSchema = parseScopeFields(rawForm)
		item.SecretFields = parseStringArray(rawSecret)
		out[id] = append(out[id], item)
	}
	return out, mapErr(rows.Err())
}

func (r *SnapshotRepo) loadPayWays(ctx context.Context, gameIDRef int64) ([]string, error) {
	rows, err := r.db.Query(ctx, `
SELECT DISTINCT pw.pay_way_id
FROM payment_routes pr
JOIN platform.pay_ways pw ON pw.id=pr.pay_way_id_ref AND pw.enabled=TRUE
WHERE pr.game_id_ref=$1 AND pr.enabled=TRUE
ORDER BY pw.pay_way_id ASC`, gameIDRef)
	if err != nil {
		return nil, mapErr(err)
	}
	defer rows.Close()
	out := make([]string, 0)
	for rows.Next() {
		var payWay string
		if err := rows.Scan(&payWay); err != nil {
			return nil, err
		}
		out = append(out, payWay)
	}
	slices.Sort(out)
	return out, mapErr(rows.Err())
}

func parseScopeFields(raw []byte) []domainsnapshot.ScopeField {
	if len(raw) == 0 {
		return nil
	}
	var fields []domainsnapshot.ScopeField
	_ = json.Unmarshal(raw, &fields)
	return fields
}

func parseStringArray(raw []byte) []string {
	if len(raw) == 0 {
		return nil
	}
	var out []string
	_ = json.Unmarshal(raw, &out)
	return out
}

func joinInt64(ids []int64) string {
	out := make([]string, 0, len(ids))
	for _, id := range ids {
		out = append(out, strconv.FormatInt(id, 10))
	}
	return strings.Join(out, ",")
}
