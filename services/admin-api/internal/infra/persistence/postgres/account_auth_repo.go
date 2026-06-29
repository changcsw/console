package postgres

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	accountauthapp "github.com/csw/console/services/admin-api/internal/app/accountauth"
	domainaa "github.com/csw/console/services/admin-api/internal/domain/accountauth"
	"github.com/csw/console/services/admin-api/internal/domain/common"
)

type AccountAuthRepo struct{ db DBTX }

func (r *AccountAuthRepo) ListTypeCatalog(ctx context.Context) ([]accountauthapp.TypeCatalogItem, error) {
	rows, err := r.db.Query(ctx, `
SELECT t.id, t.auth_type_id, t.auth_type_name, t.enabled, t.sort,
       tmp.template_version, tmp.form_schema_json, tmp.secret_fields_json, tmp.file_fields_json, tmp.validation_rules_json
FROM platform.account_auth_types t
LEFT JOIN LATERAL (
  SELECT template_version, form_schema_json, secret_fields_json, file_fields_json, validation_rules_json
  FROM platform.account_auth_templates at
  WHERE at.auth_type_id_ref = t.id AND at.enabled = TRUE
  ORDER BY at.template_version DESC
  LIMIT 1
) tmp ON TRUE
ORDER BY t.sort, t.id`)
	if err != nil {
		return nil, mapErr(err)
	}
	defer rows.Close()

	out := []accountauthapp.TypeCatalogItem{}
	for rows.Next() {
		item, err := scanTypeCatalogItem(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, mapErr(rows.Err())
}

func (r *AccountAuthRepo) ListChannelPolicies(ctx context.Context, channelID string) ([]accountauthapp.ChannelTypePolicy, error) {
	rows, err := r.db.Query(ctx, `
SELECT t.auth_type_id, cat.default_enabled, cat.locked
FROM platform.channels ch
JOIN platform.channel_account_auth_types cat ON cat.channel_id_ref = ch.id
JOIN platform.account_auth_types t ON t.id = cat.auth_type_id_ref
WHERE ch.channel_id = $1
ORDER BY cat.sort, t.sort, t.id`, channelID)
	if err != nil {
		return nil, mapErr(err)
	}
	defer rows.Close()

	out := []accountauthapp.ChannelTypePolicy{}
	for rows.Next() {
		var item accountauthapp.ChannelTypePolicy
		if err := rows.Scan(&item.AuthTypeID, &item.DefaultEnabled, &item.Locked); err != nil {
			return nil, mapErr(err)
		}
		out = append(out, item)
	}
	if len(out) == 0 {
		// channel 不存在或无配置均按 not found 处理，避免吞掉错误输入。
		var exists bool
		if err := r.db.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM platform.channels WHERE channel_id=$1)`, channelID).Scan(&exists); err != nil {
			return nil, mapErr(err)
		}
		if !exists {
			return nil, mapErr(errNoRows())
		}
	}
	return out, mapErr(rows.Err())
}

func (r *AccountAuthRepo) ResolveGameRowID(ctx context.Context, gameID string) (int64, error) {
	var id int64
	if err := r.db.QueryRow(ctx, `SELECT id FROM games WHERE game_id=$1`, gameID).Scan(&id); err != nil {
		return 0, mapErr(err)
	}
	return id, nil
}

func (r *AccountAuthRepo) ListAllowedTypesByGame(ctx context.Context, gameIDRef int64) ([]accountauthapp.GameAllowedType, error) {
	rows, err := r.db.Query(ctx, `
SELECT t.id, t.auth_type_id,
       BOOL_OR(cat.default_enabled) AS default_enabled,
       BOOL_OR(cat.locked)          AS locked,
       tmp.template_version, tmp.form_schema_json, tmp.secret_fields_json, tmp.file_fields_json, tmp.validation_rules_json
FROM game_channels gc
JOIN platform.channels ch ON ch.id = gc.channel_id_ref
JOIN platform.channel_policies cp ON cp.channel_id_ref = ch.id
JOIN platform.channel_account_auth_types cat ON cat.channel_id_ref = gc.channel_id_ref
JOIN platform.account_auth_types t ON t.id = cat.auth_type_id_ref AND t.enabled = TRUE
LEFT JOIN LATERAL (
  SELECT template_version, form_schema_json, secret_fields_json, file_fields_json, validation_rules_json
  FROM platform.account_auth_templates at
  WHERE at.auth_type_id_ref = t.id AND at.enabled = TRUE
  ORDER BY at.template_version DESC
  LIMIT 1
) tmp ON TRUE
WHERE gc.game_id_ref = $1 AND gc.enabled = TRUE AND gc.hidden = FALSE
  AND cp.login_mode = 'account_system'
GROUP BY t.id, t.auth_type_id, tmp.template_version, tmp.form_schema_json, tmp.secret_fields_json, tmp.file_fields_json, tmp.validation_rules_json
ORDER BY t.id`, gameIDRef)
	if err != nil {
		return nil, mapErr(err)
	}
	defer rows.Close()

	out := []accountauthapp.GameAllowedType{}
	for rows.Next() {
		var item accountauthapp.GameAllowedType
		var formRaw, secretRaw, fileRaw, ruleRaw []byte
		if err := rows.Scan(
			&item.AuthTypeIDRef, &item.AuthTypeID, &item.DefaultEnabled, &item.Locked,
			&item.Template.TemplateVersion, &formRaw, &secretRaw, &fileRaw, &ruleRaw,
		); err != nil {
			return nil, mapErr(err)
		}
		tpl, err := decodeTemplate(item.Template.TemplateVersion, formRaw, secretRaw, fileRaw, ruleRaw)
		if err != nil {
			return nil, err
		}
		item.Template = tpl
		out = append(out, item)
	}
	return out, mapErr(rows.Err())
}

func (r *AccountAuthRepo) ListGameConfigs(ctx context.Context, gameIDRef int64) ([]accountauthapp.GameConfigItem, error) {
	rows, err := r.db.Query(ctx, `
SELECT auth_type_id_ref, enabled, config_json, config_status, last_check_at, last_check_message
FROM game_account_auth_configs
WHERE game_id_ref=$1
ORDER BY auth_type_id_ref`, gameIDRef)
	if err != nil {
		return nil, mapErr(err)
	}
	defer rows.Close()

	out := []accountauthapp.GameConfigItem{}
	for rows.Next() {
		var item accountauthapp.GameConfigItem
		var cfgRaw []byte
		var status string
		if err := rows.Scan(&item.AuthTypeIDRef, &item.Enabled, &cfgRaw, &status, &item.LastCheckAt, &item.LastCheckMessage); err != nil {
			return nil, mapErr(err)
		}
		if err := json.Unmarshal(cfgRaw, &item.ConfigJSON); err != nil {
			return nil, fmt.Errorf("decode config_json: %w", err)
		}
		item.ConfigStatus = common.ConfigStatus(status)
		out = append(out, item)
	}
	return out, mapErr(rows.Err())
}

func (r *AccountAuthRepo) ReplaceGameConfigs(ctx context.Context, gameIDRef int64, items []accountauthapp.GameConfigUpsert) error {
	if _, err := r.db.Exec(ctx, `DELETE FROM game_account_auth_configs WHERE game_id_ref=$1`, gameIDRef); err != nil {
		return mapErr(err)
	}
	for _, item := range items {
		cfg := item.ConfigJSON
		if cfg == nil {
			cfg = map[string]any{}
		}
		raw, err := json.Marshal(cfg)
		if err != nil {
			return fmt.Errorf("marshal config_json: %w", err)
		}
		if _, err := r.db.Exec(ctx, `
INSERT INTO game_account_auth_configs (
  game_id_ref, auth_type_id_ref, enabled, config_json, config_status, last_check_at, last_check_message
) VALUES ($1,$2,$3,$4::jsonb,$5,$6,$7)`,
			gameIDRef, item.AuthTypeIDRef, item.Enabled, string(raw), string(item.ConfigStatus), item.LastCheckAt, item.LastCheckMessage,
		); err != nil {
			return mapErr(err)
		}
	}
	return nil
}

func scanTypeCatalogItem(rows interface {
	Scan(dest ...any) error
}) (accountauthapp.TypeCatalogItem, error) {
	var item accountauthapp.TypeCatalogItem
	var formRaw, secretRaw, fileRaw, ruleRaw []byte
	if err := rows.Scan(
		&item.AuthTypeIDRef, &item.AuthTypeID, &item.AuthTypeName, &item.Enabled, &item.Sort,
		&item.Template.TemplateVersion, &formRaw, &secretRaw, &fileRaw, &ruleRaw,
	); err != nil {
		return accountauthapp.TypeCatalogItem{}, mapErr(err)
	}
	tpl, err := decodeTemplate(item.Template.TemplateVersion, formRaw, secretRaw, fileRaw, ruleRaw)
	if err != nil {
		return accountauthapp.TypeCatalogItem{}, err
	}
	item.Template = tpl
	return item, nil
}

func decodeTemplate(version string, formRaw, secretRaw, fileRaw, ruleRaw []byte) (domainaa.Template, error) {
	tpl := domainaa.Template{
		TemplateVersion: version,
		FormSchema:      []domainaa.FormField{},
		SecretFields:    []string{},
		FileFields:      []domainaa.FileField{},
		ValidationRules: map[string]domainaa.ValidationRule{},
	}
	if strings.TrimSpace(version) == "" {
		return tpl, nil
	}
	if len(formRaw) > 0 {
		if err := json.Unmarshal(formRaw, &tpl.FormSchema); err != nil {
			return domainaa.Template{}, fmt.Errorf("decode form_schema_json: %w", err)
		}
	}
	if len(secretRaw) > 0 {
		if err := json.Unmarshal(secretRaw, &tpl.SecretFields); err != nil {
			return domainaa.Template{}, fmt.Errorf("decode secret_fields_json: %w", err)
		}
	}
	if len(fileRaw) > 0 {
		if err := json.Unmarshal(fileRaw, &tpl.FileFields); err != nil {
			return domainaa.Template{}, fmt.Errorf("decode file_fields_json: %w", err)
		}
	}
	if len(ruleRaw) > 0 {
		if err := json.Unmarshal(ruleRaw, &tpl.ValidationRules); err != nil {
			return domainaa.Template{}, fmt.Errorf("decode validation_rules_json: %w", err)
		}
	}
	return tpl, nil
}
