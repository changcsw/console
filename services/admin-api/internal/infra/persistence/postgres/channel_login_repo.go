package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	channellogin "github.com/csw/console/services/admin-api/internal/app/channellogin"
	domainchannel "github.com/csw/console/services/admin-api/internal/domain/channel"
	"github.com/csw/console/services/admin-api/internal/domain/common"

	"github.com/jackc/pgx/v5"
)

// ChannelLoginTemplateRepo 平台级渠道登录模板读仓储（platform.channel_login_templates）。
// 简单模板表（00 §4.4.1）：运行时取该渠道 enabled=TRUE 的最新 template_version；不走三态机。
type ChannelLoginTemplateRepo struct{ db DBTX }

const channelLoginTemplateSelect = `
SELECT id, channel_id_ref, template_version,
       form_schema_json, secret_fields_json, file_fields_json, validation_rules_json,
       enabled, created_at, updated_at
FROM platform.channel_login_templates`

// GetPublishedByChannel 取该渠道 enabled=TRUE 的最新 template_version；不存在返回 (nil, nil)。
func (r *ChannelLoginTemplateRepo) GetPublishedByChannel(ctx context.Context, channelIDRef int64) (*domainchannel.ChannelLoginTemplate, error) {
	row := r.db.QueryRow(ctx, channelLoginTemplateSelect+
		` WHERE channel_id_ref = $1 AND enabled = TRUE ORDER BY template_version DESC LIMIT 1`, channelIDRef)
	return scanChannelLoginTemplate(row)
}

// GetByChannelVersion 取该渠道指定 template_version（显式校验版本）；不存在返回 (nil, nil)。
func (r *ChannelLoginTemplateRepo) GetByChannelVersion(ctx context.Context, channelIDRef int64, version string) (*domainchannel.ChannelLoginTemplate, error) {
	row := r.db.QueryRow(ctx, channelLoginTemplateSelect+
		` WHERE channel_id_ref = $1 AND template_version = $2 LIMIT 1`, channelIDRef, version)
	return scanChannelLoginTemplate(row)
}

func scanChannelLoginTemplate(row pgx.Row) (*domainchannel.ChannelLoginTemplate, error) {
	var (
		tpl                                 domainchannel.ChannelLoginTemplate
		formRaw, secretRaw, fileRaw, ruleRaw []byte
	)
	if err := row.Scan(
		&tpl.ID, &tpl.ChannelIDRef, &tpl.TemplateVersion,
		&formRaw, &secretRaw, &fileRaw, &ruleRaw,
		&tpl.Enabled, &tpl.CreatedAt, &tpl.UpdatedAt,
	); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, mapErr(err)
	}
	tpl.FormSchema = []domainchannel.ChannelLoginFormField{}
	tpl.SecretFields = []string{}
	tpl.FileFields = []domainchannel.ChannelLoginFileField{}
	tpl.ValidationRules = map[string]domainchannel.ChannelLoginValidationRule{}
	if len(formRaw) > 0 {
		if err := json.Unmarshal(formRaw, &tpl.FormSchema); err != nil {
			return nil, fmt.Errorf("decode form_schema_json: %w", err)
		}
	}
	if len(secretRaw) > 0 {
		if err := json.Unmarshal(secretRaw, &tpl.SecretFields); err != nil {
			return nil, fmt.Errorf("decode secret_fields_json: %w", err)
		}
	}
	if len(fileRaw) > 0 {
		if err := json.Unmarshal(fileRaw, &tpl.FileFields); err != nil {
			return nil, fmt.Errorf("decode file_fields_json: %w", err)
		}
	}
	if len(ruleRaw) > 0 {
		if err := json.Unmarshal(ruleRaw, &tpl.ValidationRules); err != nil {
			return nil, fmt.Errorf("decode validation_rules_json: %w", err)
		}
	}
	return &tpl, nil
}

// ChannelLoginConfigRepo 渠道实例登录配置仓储（game_channel_login_configs，业务表，靠 search_path）。
type ChannelLoginConfigRepo struct{ db DBTX }

// GetByGameChannel 按 game_channel_id_ref 取配置；不存在返回 (nil, nil)。
func (r *ChannelLoginConfigRepo) GetByGameChannel(ctx context.Context, gameChannelID int64) (*domainchannel.ChannelLoginConfig, error) {
	row := r.db.QueryRow(ctx, `
SELECT id, game_channel_id_ref, enabled, config_json, config_status, last_check_at, last_check_message, created_at, updated_at
FROM game_channel_login_configs
WHERE game_channel_id_ref = $1`, gameChannelID)

	var (
		cfg    domainchannel.ChannelLoginConfig
		cfgRaw []byte
		status string
	)
	if err := row.Scan(
		&cfg.ID, &cfg.GameChannelIDRef, &cfg.Enabled, &cfgRaw, &status,
		&cfg.LastCheckAt, &cfg.LastCheckMessage, &cfg.CreatedAt, &cfg.UpdatedAt,
	); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, mapErr(err)
	}
	cfg.ConfigJSON = map[string]any{}
	if len(cfgRaw) > 0 {
		if err := json.Unmarshal(cfgRaw, &cfg.ConfigJSON); err != nil {
			return nil, fmt.Errorf("decode config_json: %w", err)
		}
	}
	cfg.ConfigStatus = common.ConfigStatus(status)
	return &cfg, nil
}

// Upsert 按 (game_channel_id_ref) upsert（唯一键冲突即更新）。
func (r *ChannelLoginConfigRepo) Upsert(ctx context.Context, cfg *domainchannel.ChannelLoginConfig) error {
	config := cfg.ConfigJSON
	if config == nil {
		config = map[string]any{}
	}
	raw, err := json.Marshal(config)
	if err != nil {
		return fmt.Errorf("marshal config_json: %w", err)
	}
	_, err = r.db.Exec(ctx, `
INSERT INTO game_channel_login_configs (
  game_channel_id_ref, enabled, config_json, config_status, last_check_at, last_check_message
) VALUES ($1, $2, $3::jsonb, $4, $5, $6)
ON CONFLICT (game_channel_id_ref) DO UPDATE SET
  enabled            = EXCLUDED.enabled,
  config_json        = EXCLUDED.config_json,
  config_status      = EXCLUDED.config_status,
  last_check_at      = EXCLUDED.last_check_at,
  last_check_message = EXCLUDED.last_check_message,
  updated_at         = NOW()`,
		cfg.GameChannelIDRef, cfg.Enabled, string(raw), string(cfg.ConfigStatus), cfg.LastCheckAt, cfg.LastCheckMessage,
	)
	return mapErr(err)
}

// channelLoginPolicyAdapter 复用 ChannelRepo，适配 channellogin.ChannelPolicyRepository。
type channelLoginPolicyAdapter struct{ repo *ChannelRepo }

// GetByChannelID 按渠道业务键取策略（login_mode / login_locked）。
func (a channelLoginPolicyAdapter) GetByChannelID(ctx context.Context, channelID string) (domainchannel.ChannelPolicy, error) {
	cwp, err := a.repo.GetChannelByChannelID(ctx, channelID)
	if err != nil {
		return domainchannel.ChannelPolicy{}, err
	}
	return cwp.Policy, nil
}

// 接口符合性编译期断言。
var (
	_ channellogin.ChannelLoginTemplateRepository = (*ChannelLoginTemplateRepo)(nil)
	_ channellogin.ChannelLoginConfigRepository   = (*ChannelLoginConfigRepo)(nil)
	_ channellogin.ChannelPolicyRepository        = channelLoginPolicyAdapter{}
	_ channellogin.GameChannelRepository          = (*GameChannelRepo)(nil)
)
