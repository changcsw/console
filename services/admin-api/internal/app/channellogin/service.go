package channellogin

import (
	"context"
	"errors"
	"slices"
	"strconv"
	"strings"
	"time"

	adminapp "github.com/csw/console/services/admin-api/internal/app/admin"
	"github.com/csw/console/services/admin-api/internal/app/dto"
	"github.com/csw/console/services/admin-api/internal/domain/channel"
	"github.com/csw/console/services/admin-api/internal/domain/common"
)

type Service struct {
	tx     TxManager
	cipher Cipher
	files  FileStore
	audit  AuditSink
	now    func() time.Time
	env    common.Environment
}

func NewService(tx TxManager, cipher Cipher, files FileStore, audit AuditSink, now func() time.Time, env common.Environment) *Service {
	if now == nil {
		now = time.Now
	}
	return &Service{tx: tx, cipher: cipher, files: files, audit: audit, now: now, env: env}
}

func (s *Service) GetLoginConfig(ctx context.Context, gameChannelID int64) (dto.ChannelLoginView, error) {
	inst, policy, tpl, err := s.loadContext(ctx, gameChannelID, "")
	if err != nil {
		return dto.ChannelLoginView{}, err
	}
	cfg, err := s.tx.Repositories().Configs.GetByGameChannel(ctx, gameChannelID)
	if err != nil {
		return dto.ChannelLoginView{}, mapLoadErr(err, "读取渠道登录配置失败")
	}
	if cfg == nil {
		cfg = &channel.ChannelLoginConfig{
			GameChannelIDRef: gameChannelID,
			Enabled:          false,
			ConfigJSON:       map[string]any{},
			ConfigStatus:     common.ConfigStatusEmpty,
		}
	}
	return s.toView(inst, policy, tpl, cfg), nil
}

func (s *Service) UpsertLoginConfig(ctx context.Context, cmd dto.UpsertChannelLoginConfigCmd) (dto.ChannelLoginView, error) {
	inst, policy, tpl, err := s.loadContext(ctx, cmd.GameChannelID, cmd.TemplateVersion)
	if err != nil {
		return dto.ChannelLoginView{}, err
	}
	repos := s.tx.Repositories()
	existing, err := repos.Configs.GetByGameChannel(ctx, cmd.GameChannelID)
	if err != nil {
		return dto.ChannelLoginView{}, mapLoadErr(err, "读取渠道登录配置失败")
	}
	if existing == nil {
		existing = &channel.ChannelLoginConfig{
			GameChannelIDRef: cmd.GameChannelID,
			ConfigJSON:       map[string]any{},
			ConfigStatus:     common.ConfigStatusEmpty,
		}
	}

	enabled := false
	if cmd.Enabled != nil {
		enabled = *cmd.Enabled
	}
	inputConfig := cloneMap(cmd.ConfigJSON)
	logicalConfig := cloneMap(inputConfig)
	secretChanged := map[string]bool{}
	for _, key := range tpl.SecretFields {
		raw, ok := logicalConfig[key]
		if !ok {
			continue
		}
		str, ok := raw.(string)
		if !ok {
			continue
		}
		if channel.SecretMaskedValue == str || channel.SecretMaskedAlias == str {
			if prev, exists := existing.ConfigJSON[key]; exists {
				// 哨兵=保留原密文（未修改）：校验前先用存量值替换哨兵，使该字段按
				// “已存在且合法值”参与必填/validation_rules 校验，避免字面 "******"(len=6)
				// 命中 minLen/pattern 被误判 invalid（与 account-auth 校验前替换哨兵一致）。
				logicalConfig[key] = prev
				inputConfig[key] = prev
			} else {
				// 无存量密文时哨兵视为未填 → 必填校验缺失 → invalid，避免 ****** 明文落库。
				delete(logicalConfig, key)
				delete(inputConfig, key)
			}
			continue
		}
		secretChanged[key] = true
	}
	for _, field := range tpl.FileFields {
		raw, ok := inputConfig[field.Key]
		if !ok {
			continue
		}
		ref, ok := raw.(string)
		if !ok {
			continue
		}
		ref = strings.TrimSpace(ref)
		if ref == "" {
			continue
		}
		if s.files != nil {
			normalized, ferr := s.files.NormalizeReference(ctx, ref)
			if ferr != nil {
				return dto.ChannelLoginView{}, validationErr("文件字段处理失败", ValidationDetail{
					Field: field.Key, Rule: "file", Message: ferr.Error(),
				})
			}
			inputConfig[field.Key] = normalized
		} else {
			inputConfig[field.Key] = ref
		}
	}

	status, message, issues := channel.ValidateLoginConfigAgainstTemplate(logicalConfig, *tpl)
	if status == common.ConfigStatusEmpty && inst.CopiedFromMarket != "" {
		status = common.ConfigStatusInvalid
		message = channel.CopiedMissingFieldsMessage
	}
	checkAt := (*time.Time)(nil)
	if status != common.ConfigStatusEmpty {
		now := s.now()
		checkAt = &now
	}
	if status == common.ConfigStatusValid {
		message = ""
	}
	if status == common.ConfigStatusEmpty {
		message = ""
	}

	storedConfig, err := s.encryptSecrets(inputConfig, tpl.SecretFields, existing.ConfigJSON, secretChanged)
	if err != nil {
		return dto.ChannelLoginView{}, err
	}
	next := &channel.ChannelLoginConfig{
		ID:               existing.ID,
		GameChannelIDRef: cmd.GameChannelID,
		Enabled:          enabled,
		ConfigJSON:       storedConfig,
		ConfigStatus:     status,
		LastCheckAt:      checkAt,
		LastCheckMessage: message,
	}

	if err := s.tx.InTx(ctx, func(txRepos Repositories) error {
		return txRepos.Configs.Upsert(ctx, next)
	}); err != nil {
		return dto.ChannelLoginView{}, mapWriteErr(err)
	}
	view := s.toView(inst, policy, tpl, next)

	if len(issues) > 0 {
		details := make([]any, 0, len(issues))
		for _, issue := range issues {
			details = append(details, ValidationDetail{
				Field: issue.Field, Rule: issue.Rule, Message: issue.Message,
			})
		}
		return view, validationErr(messageOrFallback(message, "渠道登录配置校验失败"), details...)
	}
	s.writeAudit(ctx, inst, existing, next, tpl, secretChanged)
	return view, nil
}

func (s *Service) loadContext(ctx context.Context, gameChannelID int64, templateVersion string) (
	channel.GameMarketChannel, channel.ChannelPolicy, *channel.ChannelLoginTemplate, error,
) {
	repos := s.tx.Repositories()
	inst, err := repos.GameChannels.GetByID(ctx, gameChannelID)
	if err != nil {
		return channel.GameMarketChannel{}, channel.ChannelPolicy{}, nil, mapLoadErr(err, "gameChannel 不存在")
	}
	policy, err := repos.Policies.GetByChannelID(ctx, inst.ChannelID)
	if err != nil {
		return channel.GameMarketChannel{}, channel.ChannelPolicy{}, nil, mapLoadErr(err, "渠道策略不存在")
	}
	if policy.LoginMode != common.LoginModeChannelOnly {
		return channel.GameMarketChannel{}, channel.ChannelPolicy{}, nil, validationErr("该渠道非 channel_only，不能读写渠道登录配置")
	}
	var tpl *channel.ChannelLoginTemplate
	if strings.TrimSpace(templateVersion) == "" {
		tpl, err = repos.Templates.GetPublishedByChannel(ctx, inst.ChannelIDRef)
	} else {
		tpl, err = repos.Templates.GetByChannelVersion(ctx, inst.ChannelIDRef, templateVersion)
	}
	if err != nil {
		return channel.GameMarketChannel{}, channel.ChannelPolicy{}, nil, mapLoadErr(err, "渠道登录模板不存在")
	}
	if tpl == nil {
		return channel.GameMarketChannel{}, channel.ChannelPolicy{}, nil, validationErr("渠道登录模板不存在")
	}
	return inst, policy, tpl, nil
}

func (s *Service) encryptSecrets(
	input map[string]any,
	secretFields []string,
	existing map[string]any,
	changed map[string]bool,
) (map[string]any, error) {
	out := cloneMap(input)
	for _, key := range secretFields {
		raw, ok := out[key]
		if !ok {
			continue
		}
		str, ok := raw.(string)
		if !ok {
			continue
		}
		str = strings.TrimSpace(str)
		if str == "" {
			delete(out, key)
			continue
		}
		if !changed[key] {
			if prev, ok := existing[key]; ok {
				out[key] = prev
			}
			continue
		}
		if s.cipher == nil {
			return nil, validationErr("密文字段加密器未配置", ValidationDetail{
				Field: key, Rule: "encrypt", Message: "缺少 AES-GCM 配置",
			})
		}
		encrypted, err := s.cipher.Encrypt(str)
		if err != nil {
			return nil, validationErr("密文字段加密失败", ValidationDetail{
				Field: key, Rule: "encrypt", Message: err.Error(),
			})
		}
		out[key] = encrypted
	}
	return out, nil
}

func (s *Service) toView(
	inst channel.GameMarketChannel,
	policy channel.ChannelPolicy,
	tpl *channel.ChannelLoginTemplate,
	cfg *channel.ChannelLoginConfig,
) dto.ChannelLoginView {
	config := cloneMap(cfg.ConfigJSON)
	for _, key := range tpl.SecretFields {
		if _, ok := config[key]; ok {
			config[key] = channel.SecretMaskedValue
		}
	}
	return dto.ChannelLoginView{
		GameChannelID: inst.ID,
		Environment:   string(s.env),
		ChannelID:     inst.ChannelID,
		MarketCode:    inst.Market,
		LoginMode:     string(policy.LoginMode),
		LoginLocked:   policy.LoginLocked,
		Config: dto.ChannelLoginConfigView{
			Enabled:          cfg.Enabled,
			ConfigJSON:       config,
			ConfigStatus:     string(cfg.ConfigStatus),
			LastCheckAt:      cfg.LastCheckAt,
			LastCheckMessage: cfg.LastCheckMessage,
		},
		Template: dto.ChannelLoginTemplateView{
			TemplateVersion: tpl.TemplateVersion,
			FormSchemaJSON:  mapFormSchema(tpl.FormSchema),
			SecretFields:    slices.Clone(tpl.SecretFields),
			FileFields:      mapFileFields(tpl.FileFields),
			ValidationRules: mapValidationRules(tpl.ValidationRules),
		},
	}
}

func (s *Service) writeAudit(
	ctx context.Context,
	inst channel.GameMarketChannel,
	before *channel.ChannelLoginConfig,
	after *channel.ChannelLoginConfig,
	tpl *channel.ChannelLoginTemplate,
	secretChanged map[string]bool,
) {
	if s.audit == nil {
		return
	}
	actor := int64(0)
	if ac, ok := adminapp.AuthContextFrom(ctx); ok {
		actor = ac.UserID
	}
	detail := map[string]any{
		"before": map[string]any{
			"enabled":          before.Enabled,
			"configStatus":     string(before.ConfigStatus),
			"lastCheckMessage": before.LastCheckMessage,
		},
		"after": map[string]any{
			"enabled":          after.Enabled,
			"configStatus":     string(after.ConfigStatus),
			"lastCheckMessage": after.LastCheckMessage,
		},
		"templateVersion": tpl.TemplateVersion,
	}
	secretMeta := map[string]any{}
	for _, key := range tpl.SecretFields {
		secretMeta[key] = map[string]any{"changed": secretChanged[key]}
	}
	detail["secrets"] = secretMeta
	s.audit.Write(ctx, AuditEntry{
		ActorID:      actor,
		Action:       "channel.login_config.update",
		ResourceType: "game_channel_login_config",
		ResourceID:   itoa(inst.ID),
		Detail:       detail,
	})
}

func mapLoadErr(err error, msg string) error {
	var appErr *Error
	if errors.As(err, &appErr) {
		return appErr
	}
	if errors.Is(err, adminapp.ErrNotFound) {
		return notFoundErr(msg)
	}
	return err
}

func mapWriteErr(err error) error {
	var appErr *Error
	if errors.As(err, &appErr) {
		return appErr
	}
	if errors.Is(err, adminapp.ErrConflict) {
		return conflictErr("渠道登录配置冲突")
	}
	return err
}

func cloneMap(input map[string]any) map[string]any {
	if input == nil {
		return map[string]any{}
	}
	out := make(map[string]any, len(input))
	for k, v := range input {
		out[k] = v
	}
	return out
}

func mapFormSchema(fields []channel.ChannelLoginFormField) []any {
	out := make([]any, 0, len(fields))
	for _, item := range fields {
		out = append(out, map[string]any{
			"key":       item.Key,
			"label":     item.Label,
			"component": item.Component,
			"required":  item.Required,
			"order":     item.Order,
			"group":     item.Group,
		})
	}
	return out
}

func mapFileFields(fields []channel.ChannelLoginFileField) []any {
	out := make([]any, 0, len(fields))
	for _, item := range fields {
		entry := map[string]any{"key": item.Key}
		if len(item.Accept) > 0 {
			entry["accept"] = slices.Clone(item.Accept)
		}
		if item.MaxSizeKB != nil {
			entry["maxSizeKB"] = *item.MaxSizeKB
		}
		out = append(out, entry)
	}
	return out
}

func mapValidationRules(rules map[string]channel.ChannelLoginValidationRule) map[string]any {
	out := map[string]any{}
	for key, rule := range rules {
		entry := map[string]any{}
		if rule.Required {
			entry["required"] = true
		}
		if rule.MinLen != nil {
			entry["minLen"] = *rule.MinLen
		}
		if rule.MaxLen != nil {
			entry["maxLen"] = *rule.MaxLen
		}
		if rule.Min != nil {
			entry["min"] = *rule.Min
		}
		if rule.Max != nil {
			entry["max"] = *rule.Max
		}
		if rule.Pattern != "" {
			entry["pattern"] = rule.Pattern
		}
		if rule.Format != "" {
			entry["format"] = rule.Format
		}
		if len(rule.Enum) > 0 {
			entry["enum"] = slices.Clone(rule.Enum)
		}
		out[key] = entry
	}
	return out
}

func messageOrFallback(message, fallback string) string {
	if strings.TrimSpace(message) == "" {
		return fallback
	}
	return message
}

func itoa(v int64) string {
	return strconv.FormatInt(v, 10)
}
