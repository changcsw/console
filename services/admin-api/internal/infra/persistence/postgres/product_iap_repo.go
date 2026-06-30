package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"

	"github.com/csw/console/services/admin-api/internal/domain/accountauth"
	"github.com/csw/console/services/admin-api/internal/domain/common"
	domainproduct "github.com/csw/console/services/admin-api/internal/domain/product"
)

type GameChannelIAPConfigRepo struct{ db DBTX }

func (r *GameChannelIAPConfigRepo) GetChannelInfo(ctx context.Context, gameChannelID int64) (string, error) {
	var channelID string
	err := r.db.QueryRow(ctx, `
SELECT ch.channel_id
FROM game_channels gc
JOIN platform.channels ch ON ch.id = gc.channel_id_ref
WHERE gc.id=$1`, gameChannelID).Scan(&channelID)
	if err != nil {
		return "", mapErr(err)
	}
	return channelID, nil
}

func (r *GameChannelIAPConfigRepo) GetByGameChannelID(ctx context.Context, gameChannelID int64) (domainproduct.IAPConfig, bool, error) {
	row := r.db.QueryRow(ctx, `
SELECT id, game_channel_id_ref, enabled, config_json, config_status, last_check_at, last_check_message, created_at, updated_at
FROM game_channel_iap_configs WHERE game_channel_id_ref=$1`, gameChannelID)
	item, ok, err := scanIAPConfig(row, true)
	if err != nil {
		return domainproduct.IAPConfig{}, false, mapErr(err)
	}
	return item, ok, nil
}

func (r *GameChannelIAPConfigRepo) UpsertByGameChannelID(ctx context.Context, gameChannelID int64, cfg domainproduct.IAPConfig) (domainproduct.IAPConfig, error) {
	raw, err := marshalJSON(cfg.ConfigJSON)
	if err != nil {
		return domainproduct.IAPConfig{}, err
	}
	row := r.db.QueryRow(ctx, `
INSERT INTO game_channel_iap_configs (game_channel_id_ref, enabled, config_json, config_status, last_check_at, last_check_message)
VALUES ($1,$2,$3::jsonb,$4,$5,$6)
ON CONFLICT (game_channel_id_ref)
DO UPDATE SET enabled=EXCLUDED.enabled, config_json=EXCLUDED.config_json, config_status=EXCLUDED.config_status,
              last_check_at=EXCLUDED.last_check_at, last_check_message=EXCLUDED.last_check_message, updated_at=NOW()
RETURNING id, game_channel_id_ref, enabled, config_json, config_status, last_check_at, last_check_message, created_at, updated_at`,
		gameChannelID, cfg.Enabled, raw, string(cfg.ConfigStatus), cfg.LastCheckAt, cfg.LastCheckMessage)
	item, _, err := scanIAPConfig(row, true)
	if err != nil {
		return domainproduct.IAPConfig{}, mapErr(err)
	}
	return item, nil
}

type ChannelPackageIAPOverrideRepo struct{ db DBTX }

func (r *ChannelPackageIAPOverrideRepo) GetByPackageID(ctx context.Context, packageID int64) (domainproduct.IAPConfig, bool, error) {
	row := r.db.QueryRow(ctx, `
SELECT id, package_id_ref, enabled, config_json, config_status, last_check_at, last_check_message, created_at, updated_at
FROM channel_package_iap_overrides WHERE package_id_ref=$1`, packageID)
	item, ok, err := scanIAPConfig(row, false)
	if err != nil {
		return domainproduct.IAPConfig{}, false, mapErr(err)
	}
	return item, ok, nil
}

func (r *ChannelPackageIAPOverrideRepo) UpsertByPackageID(ctx context.Context, packageID int64, cfg domainproduct.IAPConfig) (domainproduct.IAPConfig, error) {
	raw, err := marshalJSON(cfg.ConfigJSON)
	if err != nil {
		return domainproduct.IAPConfig{}, err
	}
	row := r.db.QueryRow(ctx, `
INSERT INTO channel_package_iap_overrides (package_id_ref, enabled, config_json, config_status, last_check_at, last_check_message)
VALUES ($1,$2,$3::jsonb,$4,$5,$6)
ON CONFLICT (package_id_ref)
DO UPDATE SET enabled=EXCLUDED.enabled, config_json=EXCLUDED.config_json, config_status=EXCLUDED.config_status,
              last_check_at=EXCLUDED.last_check_at, last_check_message=EXCLUDED.last_check_message, updated_at=NOW()
RETURNING id, package_id_ref, enabled, config_json, config_status, last_check_at, last_check_message, created_at, updated_at`,
		packageID, cfg.Enabled, raw, string(cfg.ConfigStatus), cfg.LastCheckAt, cfg.LastCheckMessage)
	item, _, err := scanIAPConfig(row, false)
	if err != nil {
		return domainproduct.IAPConfig{}, mapErr(err)
	}
	return item, nil
}

type ChannelIAPTemplateRepo struct{ db DBTX }

func (r *ChannelIAPTemplateRepo) GetLatestEnabledByChannelID(ctx context.Context, channelID string) (accountauth.Template, error) {
	var (
		version                string
		formRaw, secretRaw     []byte
		fileRaw, validationRaw []byte
	)
	err := r.db.QueryRow(ctx, `
SELECT t.template_version, t.form_schema_json, t.secret_fields_json, t.file_fields_json, t.validation_rules_json
FROM platform.channel_iap_templates t
JOIN platform.channels ch ON ch.id = t.channel_id_ref
WHERE ch.channel_id=$1 AND t.enabled=TRUE
ORDER BY t.template_version DESC
LIMIT 1`, channelID).Scan(&version, &formRaw, &secretRaw, &fileRaw, &validationRaw)
	if err != nil {
		return accountauth.Template{}, mapErr(err)
	}
	return decodeIAPTemplate(version, formRaw, secretRaw, fileRaw, validationRaw)
}

func scanIAPConfig(row interface{ Scan(...any) error }, gameChannel bool) (domainproduct.IAPConfig, bool, error) {
	var (
		item   domainproduct.IAPConfig
		raw    []byte
		status string
	)
	if gameChannel {
		if err := row.Scan(&item.ID, &item.GameChannelIDRef, &item.Enabled, &raw, &status, &item.LastCheckAt, &item.LastCheckMessage, &item.CreatedAt, &item.UpdatedAt); err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return domainproduct.IAPConfig{}, false, nil
			}
			return domainproduct.IAPConfig{}, false, err
		}
	} else {
		if err := row.Scan(&item.ID, &item.PackageIDRef, &item.Enabled, &raw, &status, &item.LastCheckAt, &item.LastCheckMessage, &item.CreatedAt, &item.UpdatedAt); err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return domainproduct.IAPConfig{}, false, nil
			}
			return domainproduct.IAPConfig{}, false, err
		}
	}
	item.ConfigJSON = map[string]any{}
	if len(raw) > 0 {
		if err := json.Unmarshal(raw, &item.ConfigJSON); err != nil {
			return domainproduct.IAPConfig{}, false, fmt.Errorf("decode config_json: %w", err)
		}
	}
	item.ConfigStatus = common.ConfigStatus(status)
	return item, true, nil
}

func decodeIAPTemplate(version string, formRaw, secretRaw, fileRaw, validationRaw []byte) (accountauth.Template, error) {
	tpl := accountauth.Template{
		TemplateVersion: version,
		FormSchema:      []accountauth.FormField{},
		SecretFields:    []string{},
		FileFields:      []accountauth.FileField{},
		ValidationRules: map[string]accountauth.ValidationRule{},
	}
	if len(formRaw) > 0 {
		if err := json.Unmarshal(formRaw, &tpl.FormSchema); err != nil {
			return accountauth.Template{}, fmt.Errorf("decode form_schema_json: %w", err)
		}
	}
	if len(secretRaw) > 0 {
		if err := json.Unmarshal(secretRaw, &tpl.SecretFields); err != nil {
			return accountauth.Template{}, fmt.Errorf("decode secret_fields_json: %w", err)
		}
	}
	if len(fileRaw) > 0 {
		if err := json.Unmarshal(fileRaw, &tpl.FileFields); err != nil {
			return accountauth.Template{}, fmt.Errorf("decode file_fields_json: %w", err)
		}
	}
	if len(validationRaw) > 0 {
		if err := json.Unmarshal(validationRaw, &tpl.ValidationRules); err != nil {
			return accountauth.Template{}, fmt.Errorf("decode validation_rules_json: %w", err)
		}
	}
	return tpl, nil
}

func marshalJSON(value map[string]any) (string, error) {
	if len(value) == 0 {
		return "{}", nil
	}
	raw, err := json.Marshal(value)
	if err != nil {
		return "", err
	}
	return string(raw), nil
}
