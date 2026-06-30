package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	pluginapp "github.com/csw/console/services/admin-api/internal/app/plugin"
	domainplugin "github.com/csw/console/services/admin-api/internal/domain/plugin"
	"github.com/jackc/pgx/v5"
)

type FeaturePluginRepo struct{ db DBTX }

func (r *FeaturePluginRepo) ListByChannel(ctx context.Context, channelIDRef int64) ([]pluginapp.FeaturePluginMeta, error) {
	rows, err := r.db.Query(ctx, `
SELECT fp.id, fp.plugin_id, fp.plugin_name, fp.region, fp.enabled,
       cfp.required, cfp.selectable, cfp.locked, cfp.default_enabled
FROM platform.channel_feature_plugins cfp
JOIN platform.feature_plugins fp ON fp.id = cfp.plugin_id_ref
WHERE cfp.channel_id_ref = $1
ORDER BY fp.sort ASC, fp.id ASC`, channelIDRef)
	if err != nil {
		return nil, mapErr(err)
	}
	defer rows.Close()
	out := []pluginapp.FeaturePluginMeta{}
	for rows.Next() {
		var item pluginapp.FeaturePluginMeta
		if err := rows.Scan(&item.ID, &item.PluginID, &item.Name, &item.Region, &item.Enabled, &item.Required, &item.Selectable, &item.Locked, &item.DefaultEnabled); err != nil {
			return nil, mapErr(err)
		}
		out = append(out, item)
	}
	return out, mapErr(rows.Err())
}

func (r *FeaturePluginRepo) GetByPluginID(ctx context.Context, pluginID string) (pluginapp.FeaturePluginMeta, error) {
	var item pluginapp.FeaturePluginMeta
	err := r.db.QueryRow(ctx, `
SELECT id, plugin_id, plugin_name, region, enabled
FROM platform.feature_plugins
WHERE plugin_id = $1`, pluginID).Scan(
		&item.ID, &item.PluginID, &item.Name, &item.Region, &item.Enabled,
	)
	if err != nil {
		return pluginapp.FeaturePluginMeta{}, mapErr(err)
	}
	return item, nil
}

func (r *FeaturePluginRepo) GetLatestTemplate(ctx context.Context, pluginIDRef int64) (*pluginapp.FeaturePluginTemplate, error) {
	row := r.db.QueryRow(ctx, `
SELECT plugin_id_ref, template_version, secret_fields_json, form_schema_json, file_fields_json, validation_rules_json
FROM platform.feature_plugin_templates
WHERE plugin_id_ref = $1 AND enabled = TRUE
ORDER BY template_version DESC
LIMIT 1`, pluginIDRef)
	var (
		tpl                             pluginapp.FeaturePluginTemplate
		secretRaw, formRaw, fileRaw, vr []byte
	)
	if err := row.Scan(&tpl.PluginIDRef, &tpl.TemplateVersion, &secretRaw, &formRaw, &fileRaw, &vr); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, mapErr(err)
	}
	if len(secretRaw) > 0 {
		if err := json.Unmarshal(secretRaw, &tpl.SecretFields); err != nil {
			return nil, fmt.Errorf("decode secret_fields_json: %w", err)
		}
	}
	if len(formRaw) > 0 {
		if err := json.Unmarshal(formRaw, &tpl.FormSchema); err != nil {
			return nil, fmt.Errorf("decode form_schema_json: %w", err)
		}
	}
	if len(fileRaw) > 0 {
		if err := json.Unmarshal(fileRaw, &tpl.FileFields); err != nil {
			return nil, fmt.Errorf("decode file_fields_json: %w", err)
		}
	}
	tpl.ValidationRules = map[string]domainplugin.ValidationRule{}
	if len(vr) > 0 {
		if err := json.Unmarshal(vr, &tpl.ValidationRules); err != nil {
			return nil, fmt.Errorf("decode validation_rules_json: %w", err)
		}
	}
	return &tpl, nil
}
