package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	pluginapp "github.com/csw/console/services/admin-api/internal/app/plugin"
	"github.com/jackc/pgx/v5"
)

type GameChannelPluginRepo struct{ db DBTX }

func (r *GameChannelPluginRepo) GetGameChannel(ctx context.Context, id int64) (pluginapp.GameChannelContext, error) {
	var out pluginapp.GameChannelContext
	err := r.db.QueryRow(ctx, `
SELECT id, market_code, channel_id_ref, hidden
FROM game_channels WHERE id = $1`, id).Scan(&out.ID, &out.Market, &out.ChannelID, &out.Hidden)
	if err != nil {
		return pluginapp.GameChannelContext{}, mapErr(err)
	}
	return out, nil
}

func (r *GameChannelPluginRepo) GetByID(ctx context.Context, id int64) (pluginapp.GameChannelPluginConfig, error) {
	row := r.db.QueryRow(ctx, `
SELECT id, game_channel_id_ref, plugin_id_ref, enabled, config_json, config_status, last_check_at, last_check_message, updated_at
FROM game_channel_plugin_configs
WHERE id = $1`, id)
	return scanGameChannelPluginConfig(row)
}

func (r *GameChannelPluginRepo) GetByGameChannelAndPlugin(ctx context.Context, gameChannelID, pluginIDRef int64) (*pluginapp.GameChannelPluginConfig, error) {
	row := r.db.QueryRow(ctx, `
SELECT id, game_channel_id_ref, plugin_id_ref, enabled, config_json, config_status, last_check_at, last_check_message, updated_at
FROM game_channel_plugin_configs
WHERE game_channel_id_ref = $1 AND plugin_id_ref = $2`, gameChannelID, pluginIDRef)
	cfg, err := scanGameChannelPluginConfig(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) || errors.Is(err, mapErr(errNoRows())) {
			return nil, nil
		}
		return nil, err
	}
	return &cfg, nil
}

func (r *GameChannelPluginRepo) ListByGameChannel(ctx context.Context, gameChannelID int64) ([]pluginapp.GameChannelPluginConfig, error) {
	rows, err := r.db.Query(ctx, `
SELECT id, game_channel_id_ref, plugin_id_ref, enabled, config_json, config_status, last_check_at, last_check_message, updated_at
FROM game_channel_plugin_configs
WHERE game_channel_id_ref = $1`, gameChannelID)
	if err != nil {
		return nil, mapErr(err)
	}
	defer rows.Close()
	out := []pluginapp.GameChannelPluginConfig{}
	for rows.Next() {
		cfg, err := scanGameChannelPluginConfig(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, cfg)
	}
	return out, mapErr(rows.Err())
}

func (r *GameChannelPluginRepo) Upsert(ctx context.Context, cfg pluginapp.GameChannelPluginConfig) (pluginapp.GameChannelPluginConfig, error) {
	raw, err := json.Marshal(cfg.ConfigJSON)
	if err != nil {
		return pluginapp.GameChannelPluginConfig{}, fmt.Errorf("marshal config_json: %w", err)
	}
	err = r.db.QueryRow(ctx, `
INSERT INTO game_channel_plugin_configs (
  game_channel_id_ref, plugin_id_ref, enabled, config_json, config_status, last_check_at, last_check_message
) VALUES ($1,$2,$3,$4::jsonb,$5, NOW(), $6)
ON CONFLICT (game_channel_id_ref, plugin_id_ref) DO UPDATE SET
  enabled = EXCLUDED.enabled,
  config_json = EXCLUDED.config_json,
  config_status = EXCLUDED.config_status,
  last_check_at = EXCLUDED.last_check_at,
  last_check_message = EXCLUDED.last_check_message,
  updated_at = NOW()
RETURNING id, updated_at`,
		cfg.GameChannelIDRef, cfg.PluginIDRef, cfg.Enabled, string(raw), cfg.ConfigStatus, cfg.LastCheckMessage,
	).Scan(&cfg.ID, &cfg.UpdatedAt)
	if err != nil {
		return pluginapp.GameChannelPluginConfig{}, mapErr(err)
	}
	return cfg, nil
}

func scanGameChannelPluginConfig(row interface{ Scan(...any) error }) (pluginapp.GameChannelPluginConfig, error) {
	var cfg pluginapp.GameChannelPluginConfig
	var raw []byte
	err := row.Scan(
		&cfg.ID, &cfg.GameChannelIDRef, &cfg.PluginIDRef, &cfg.Enabled,
		&raw, &cfg.ConfigStatus, &cfg.LastCheckAt, &cfg.LastCheckMessage, &cfg.UpdatedAt,
	)
	if err != nil {
		return pluginapp.GameChannelPluginConfig{}, mapErr(err)
	}
	cfg.ConfigJSON = map[string]any{}
	if len(raw) > 0 {
		if err := json.Unmarshal(raw, &cfg.ConfigJSON); err != nil {
			return pluginapp.GameChannelPluginConfig{}, fmt.Errorf("decode config_json: %w", err)
		}
	}
	return cfg, nil
}
