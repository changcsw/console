package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	pluginapp "github.com/csw/console/services/admin-api/internal/app/plugin"
	"github.com/jackc/pgx/v5"
)

type ChannelPackagePluginRepo struct{ db DBTX }

func (r *ChannelPackagePluginRepo) GetPackage(ctx context.Context, id int64) (pluginapp.ChannelPackageContext, error) {
	var out pluginapp.ChannelPackageContext
	err := r.db.QueryRow(ctx, `
SELECT cp.id, cp.game_channel_id_ref, gc.market_code
FROM channel_packages cp
JOIN game_channels gc ON gc.id = cp.game_channel_id_ref
WHERE cp.id = $1`, id).Scan(&out.ID, &out.GameChannel, &out.MarketCode)
	if err != nil {
		return pluginapp.ChannelPackageContext{}, mapErr(err)
	}
	return out, nil
}

func (r *ChannelPackagePluginRepo) GetByID(ctx context.Context, id int64) (pluginapp.ChannelPackagePluginOverride, error) {
	row := r.db.QueryRow(ctx, `
SELECT id, package_id_ref, plugin_id_ref, inherit_channel_config, enabled, config_json, config_status, last_check_at, last_check_message, updated_at
FROM channel_package_plugin_overrides
WHERE id = $1`, id)
	return scanPackagePluginOverride(row)
}

func (r *ChannelPackagePluginRepo) GetByPackageAndPlugin(ctx context.Context, packageID, pluginIDRef int64) (*pluginapp.ChannelPackagePluginOverride, error) {
	row := r.db.QueryRow(ctx, `
SELECT id, package_id_ref, plugin_id_ref, inherit_channel_config, enabled, config_json, config_status, last_check_at, last_check_message, updated_at
FROM channel_package_plugin_overrides
WHERE package_id_ref = $1 AND plugin_id_ref = $2`, packageID, pluginIDRef)
	it, err := scanPackagePluginOverride(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) || errors.Is(err, mapErr(errNoRows())) {
			return nil, nil
		}
		return nil, err
	}
	return &it, nil
}

func (r *ChannelPackagePluginRepo) ListByPackage(ctx context.Context, packageID int64) ([]pluginapp.ChannelPackagePluginOverride, error) {
	rows, err := r.db.Query(ctx, `
SELECT id, package_id_ref, plugin_id_ref, inherit_channel_config, enabled, config_json, config_status, last_check_at, last_check_message, updated_at
FROM channel_package_plugin_overrides
WHERE package_id_ref = $1`, packageID)
	if err != nil {
		return nil, mapErr(err)
	}
	defer rows.Close()
	out := []pluginapp.ChannelPackagePluginOverride{}
	for rows.Next() {
		it, err := scanPackagePluginOverride(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, it)
	}
	return out, mapErr(rows.Err())
}

func (r *ChannelPackagePluginRepo) Upsert(ctx context.Context, cfg pluginapp.ChannelPackagePluginOverride) (pluginapp.ChannelPackagePluginOverride, error) {
	raw, err := json.Marshal(cfg.ConfigJSON)
	if err != nil {
		return pluginapp.ChannelPackagePluginOverride{}, fmt.Errorf("marshal config_json: %w", err)
	}
	err = r.db.QueryRow(ctx, `
INSERT INTO channel_package_plugin_overrides (
  package_id_ref, plugin_id_ref, inherit_channel_config, enabled, config_json, config_status, last_check_at, last_check_message
) VALUES ($1,$2,$3,$4,$5::jsonb,$6,NOW(),$7)
ON CONFLICT (package_id_ref, plugin_id_ref) DO UPDATE SET
  inherit_channel_config = EXCLUDED.inherit_channel_config,
  enabled = EXCLUDED.enabled,
  config_json = EXCLUDED.config_json,
  config_status = EXCLUDED.config_status,
  last_check_at = EXCLUDED.last_check_at,
  last_check_message = EXCLUDED.last_check_message,
  updated_at = NOW()
RETURNING id, updated_at`,
		cfg.PackageIDRef, cfg.PluginIDRef, cfg.InheritChannelConfig, cfg.Enabled, string(raw), cfg.ConfigStatus, cfg.LastCheckMessage,
	).Scan(&cfg.ID, &cfg.UpdatedAt)
	if err != nil {
		return pluginapp.ChannelPackagePluginOverride{}, mapErr(err)
	}
	return cfg, nil
}

func scanPackagePluginOverride(row interface{ Scan(...any) error }) (pluginapp.ChannelPackagePluginOverride, error) {
	var it pluginapp.ChannelPackagePluginOverride
	var raw []byte
	err := row.Scan(
		&it.ID, &it.PackageIDRef, &it.PluginIDRef, &it.InheritChannelConfig, &it.Enabled,
		&raw, &it.ConfigStatus, &it.LastCheckAt, &it.LastCheckMessage, &it.UpdatedAt,
	)
	if err != nil {
		return pluginapp.ChannelPackagePluginOverride{}, mapErr(err)
	}
	it.ConfigJSON = map[string]any{}
	if len(raw) > 0 {
		if err := json.Unmarshal(raw, &it.ConfigJSON); err != nil {
			return pluginapp.ChannelPackagePluginOverride{}, fmt.Errorf("decode config_json: %w", err)
		}
	}
	return it, nil
}
