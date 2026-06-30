package product

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"time"

	adminapp "github.com/csw/console/services/admin-api/internal/app/admin"
	"github.com/csw/console/services/admin-api/internal/app/dto"
	"github.com/csw/console/services/admin-api/internal/domain/accountauth"
	"github.com/csw/console/services/admin-api/internal/domain/common"
	domainproduct "github.com/csw/console/services/admin-api/internal/domain/product"
)

type IAPConfigService struct {
	tx     TxManager
	crypto CryptoService
	file   FileService
	audit  AuditSink
	now    nowFunc
}

func NewIAPConfigService(tx TxManager, crypto CryptoService, file FileService, audit AuditSink, now nowFunc) *IAPConfigService {
	if now == nil {
		now = time.Now
	}
	return &IAPConfigService{tx: tx, crypto: crypto, file: file, audit: audit, now: now}
}

func (s *IAPConfigService) GetGameChannelConfig(ctx context.Context, gameChannelID int64) (dto.GameChannelIAPConfigView, error) {
	channelID, err := s.tx.Repositories().GameChannelIAP.GetChannelInfo(ctx, gameChannelID)
	if err != nil {
		return dto.GameChannelIAPConfigView{}, mapReadErr(err, "game channel not found")
	}
	tpl, err := s.tx.Repositories().ChannelIAPTemplates.GetLatestEnabledByChannelID(ctx, channelID)
	if err != nil {
		return dto.GameChannelIAPConfigView{}, mapReadErr(err, "iap template not found")
	}
	cfg, ok, err := s.tx.Repositories().GameChannelIAP.GetByGameChannelID(ctx, gameChannelID)
	if err != nil {
		return dto.GameChannelIAPConfigView{}, mapReadErr(err, "iap config load failed")
	}
	if !ok {
		cfg = domainproduct.IAPConfig{
			GameChannelIDRef: gameChannelID,
			Enabled:          false,
			ConfigJSON:       map[string]any{},
			ConfigStatus:     common.ConfigStatusEmpty,
			LastCheckMessage: "",
		}
	}
	return dto.GameChannelIAPConfigView{
		GameChannelID: gameChannelID,
		ChannelID:     channelID,
		Template:      toTemplateView(tpl),
		Config:        toIAPConfigView(maskConfig(cfg.ConfigJSON, tpl.SecretFields), cfg),
	}, nil
}

func (s *IAPConfigService) PutGameChannelConfig(ctx context.Context, cmd dto.UpsertIAPConfigCmd) (dto.IAPConfigView, error) {
	channelID, err := s.tx.Repositories().GameChannelIAP.GetChannelInfo(ctx, cmd.GameChannelID)
	if err != nil {
		return dto.IAPConfigView{}, mapReadErr(err, "game channel not found")
	}
	tpl, err := s.tx.Repositories().ChannelIAPTemplates.GetLatestEnabledByChannelID(ctx, channelID)
	if err != nil {
		return dto.IAPConfigView{}, mapReadErr(err, "iap template not found")
	}
	current, _, err := s.tx.Repositories().GameChannelIAP.GetByGameChannelID(ctx, cmd.GameChannelID)
	if err != nil {
		return dto.IAPConfigView{}, mapReadErr(err, "iap config load failed")
	}
	config, status, message, checkAt, err := s.normalizeIAPConfig(tpl, cmd.ConfigJSON, current.ConfigJSON)
	if err != nil {
		return dto.IAPConfigView{}, err
	}
	enabled := false
	if cmd.Enabled != nil {
		enabled = *cmd.Enabled
	} else if current.ID > 0 {
		enabled = current.Enabled
	}
	if err := rejectEnableWhenNotValid(enabled, status, message); err != nil {
		return dto.IAPConfigView{}, err
	}
	next := domainproduct.IAPConfig{
		GameChannelIDRef: cmd.GameChannelID,
		Enabled:          enabled,
		ConfigJSON:       config,
		ConfigStatus:     status,
		LastCheckAt:      checkAt,
		LastCheckMessage: message,
	}
	saved, err := s.tx.Repositories().GameChannelIAP.UpsertByGameChannelID(ctx, cmd.GameChannelID, next)
	if err != nil {
		return dto.IAPConfigView{}, mapWriteErr(err, "save iap config failed")
	}
	s.writeAudit(ctx, "iap.config.update", fmt.Sprintf("%d", cmd.GameChannelID), map[string]any{
		"configStatus": saved.ConfigStatus,
		"enabled":      saved.Enabled,
	})
	return toIAPConfigView(maskConfig(saved.ConfigJSON, tpl.SecretFields), saved), nil
}

func (s *IAPConfigService) GetPackageOverride(ctx context.Context, packageID int64) (dto.PackageIAPOverrideView, error) {
	gameID, packageCode, channelID, gameChannelID, err := s.tx.Repositories().Packages.GetPackageGameAndChannel(ctx, packageID)
	if err != nil {
		return dto.PackageIAPOverrideView{}, mapReadErr(err, "package not found")
	}
	_ = gameID
	tpl, err := s.tx.Repositories().ChannelIAPTemplates.GetLatestEnabledByChannelID(ctx, channelID)
	if err != nil {
		return dto.PackageIAPOverrideView{}, mapReadErr(err, "iap template not found")
	}
	baseCfg, _, err := s.tx.Repositories().GameChannelIAP.GetByGameChannelID(ctx, gameChannelID)
	if err != nil {
		return dto.PackageIAPOverrideView{}, mapReadErr(err, "base iap config load failed")
	}
	override, ok, err := s.tx.Repositories().PackageIAPOverrides.GetByPackageID(ctx, packageID)
	if err != nil {
		return dto.PackageIAPOverrideView{}, mapReadErr(err, "package iap override load failed")
	}
	if !ok {
		override = domainproduct.IAPConfig{
			PackageIDRef:     packageID,
			Enabled:          false,
			ConfigJSON:       map[string]any{},
			ConfigStatus:     common.ConfigStatusEmpty,
			LastCheckMessage: "",
		}
	}
	return dto.PackageIAPOverrideView{
		PackageID:   packageID,
		PackageCode: packageCode,
		ChannelID:   channelID,
		Template:    toTemplateView(tpl),
		BaseConfig:  toIAPConfigView(maskConfig(baseCfg.ConfigJSON, tpl.SecretFields), baseCfg),
		Override:    toIAPConfigView(maskConfig(override.ConfigJSON, tpl.SecretFields), override),
	}, nil
}

func (s *IAPConfigService) PutPackageOverride(ctx context.Context, cmd dto.UpsertPackageIAPOverrideCmd) (dto.IAPConfigView, error) {
	_, _, channelID, _, err := s.tx.Repositories().Packages.GetPackageGameAndChannel(ctx, cmd.PackageID)
	if err != nil {
		return dto.IAPConfigView{}, mapReadErr(err, "package not found")
	}
	tpl, err := s.tx.Repositories().ChannelIAPTemplates.GetLatestEnabledByChannelID(ctx, channelID)
	if err != nil {
		return dto.IAPConfigView{}, mapReadErr(err, "iap template not found")
	}
	current, _, err := s.tx.Repositories().PackageIAPOverrides.GetByPackageID(ctx, cmd.PackageID)
	if err != nil {
		return dto.IAPConfigView{}, mapReadErr(err, "package iap override load failed")
	}
	config, status, message, checkAt, err := s.normalizeIAPConfig(tpl, cmd.ConfigJSON, current.ConfigJSON)
	if err != nil {
		return dto.IAPConfigView{}, err
	}
	enabled := false
	if cmd.Enabled != nil {
		enabled = *cmd.Enabled
	} else if current.ID > 0 {
		enabled = current.Enabled
	}
	if err := rejectEnableWhenNotValid(enabled, status, message); err != nil {
		return dto.IAPConfigView{}, err
	}
	next := domainproduct.IAPConfig{
		PackageIDRef:     cmd.PackageID,
		Enabled:          enabled,
		ConfigJSON:       config,
		ConfigStatus:     status,
		LastCheckAt:      checkAt,
		LastCheckMessage: message,
	}
	saved, err := s.tx.Repositories().PackageIAPOverrides.UpsertByPackageID(ctx, cmd.PackageID, next)
	if err != nil {
		return dto.IAPConfigView{}, mapWriteErr(err, "save package iap override failed")
	}
	s.writeAudit(ctx, "iap.override.update", fmt.Sprintf("%d", cmd.PackageID), map[string]any{
		"configStatus": saved.ConfigStatus,
		"enabled":      saved.Enabled,
	})
	return toIAPConfigView(maskConfig(saved.ConfigJSON, tpl.SecretFields), saved), nil
}

func (s *IAPConfigService) normalizeIAPConfig(tpl accountauth.Template, config map[string]any, existing map[string]any) (map[string]any, common.ConfigStatus, string, *time.Time, error) {
	if config == nil {
		return nil, common.ConfigStatusEmpty, "", nil, validationErr("configJson 必填", fieldDetail("configJson", "required"))
	}
	effective := mergeMissingSensitiveAndFiles(config, existing, tpl)
	withSecrets, err := s.encryptSecrets(effective, tpl.SecretFields, existing)
	if err != nil {
		return nil, common.ConfigStatusInvalid, "", nil, err
	}
	withFiles, err := s.normalizeFileRefs(withSecrets, tpl.FileFields)
	if err != nil {
		return nil, common.ConfigStatusInvalid, "", nil, err
	}
	status, message := domainproduct.DeriveConfigStatus(withFiles, tpl)
	var checkAt *time.Time
	if status == common.ConfigStatusValid {
		t := s.now()
		checkAt = &t
	}
	return withFiles, status, message, checkAt, nil
}

// rejectEnableWhenNotValid 显式启用时要求 config_status=valid（compact：缺必填不得 enabled=true）。
func rejectEnableWhenNotValid(enabled bool, status common.ConfigStatus, message string) error {
	if !enabled || status == common.ConfigStatusValid {
		return nil
	}
	reason := "config_not_valid"
	if strings.Contains(message, accountauth.MissingSecretOrFileMessage) {
		reason = "required_secret_or_file_missing"
	}
	return validationErr(message, fieldDetail("configJson", reason))
}

func (s *IAPConfigService) encryptSecrets(config map[string]any, secretFields []string, existing map[string]any) (map[string]any, error) {
	out := cloneMap(config)
	for _, key := range secretFields {
		raw, ok := out[key]
		if !ok {
			continue
		}
		str, ok := raw.(string)
		if !ok {
			continue
		}
		if strings.TrimSpace(str) == "" || str == maskedValue {
			if prev, hasPrev := existing[key]; hasPrev {
				out[key] = prev
			}
			continue
		}
		if s.crypto == nil {
			return nil, validationErr("密文字段加密器未配置", fieldDetail("configJson."+key, "crypto_unavailable"))
		}
		encrypted, err := s.crypto.Encrypt(str)
		if err != nil {
			return nil, validationErr("密文字段加密失败", fieldDetail("configJson."+key, "encrypt_failed"))
		}
		out[key] = encrypted
	}
	return out, nil
}

func (s *IAPConfigService) normalizeFileRefs(config map[string]any, fields []accountauth.FileField) (map[string]any, error) {
	if s.file == nil {
		return config, nil
	}
	out := cloneMap(config)
	for _, field := range fields {
		raw, ok := out[field.Key]
		if !ok {
			continue
		}
		str, ok := raw.(string)
		if !ok || strings.TrimSpace(str) == "" {
			continue
		}
		ref, err := s.file.NormalizeReference(str)
		if err != nil {
			return nil, validationErr("文件引用无效", fieldDetail("configJson."+field.Key, "invalid_file_ref"))
		}
		out[field.Key] = ref
	}
	return out, nil
}

func mergeMissingSensitiveAndFiles(req, existing map[string]any, tpl accountauth.Template) map[string]any {
	out := cloneMap(req)
	for _, key := range tpl.SecretFields {
		if _, ok := out[key]; !ok {
			if prev, has := existing[key]; has {
				out[key] = prev
			}
		}
	}
	for _, item := range tpl.FileFields {
		if _, ok := out[item.Key]; !ok {
			if prev, has := existing[item.Key]; has {
				out[item.Key] = prev
			}
		}
	}
	return out
}

func toTemplateView(tpl accountauth.Template) dto.TemplateView {
	formSchema := make([]any, 0, len(tpl.FormSchema))
	for _, f := range tpl.FormSchema {
		formSchema = append(formSchema, map[string]any{
			"key": f.Key, "label": f.Label, "component": f.Component, "required": f.Required, "order": f.Order, "scope": f.Scope,
		})
	}
	fileFields := make([]any, 0, len(tpl.FileFields))
	for _, f := range tpl.FileFields {
		item := map[string]any{"key": f.Key}
		if len(f.Accept) > 0 {
			item["accept"] = f.Accept
		}
		if f.MaxSizeKB > 0 {
			item["maxSizeKB"] = f.MaxSizeKB
		}
		fileFields = append(fileFields, item)
	}
	validation := map[string]any{}
	keys := make([]string, 0, len(tpl.ValidationRules))
	for k := range tpl.ValidationRules {
		keys = append(keys, k)
	}
	slices.Sort(keys)
	for _, key := range keys {
		rule := tpl.ValidationRules[key]
		item := map[string]any{}
		if rule.Required {
			item["required"] = true
		}
		if rule.MinLen != nil {
			item["minLen"] = *rule.MinLen
		}
		if rule.MaxLen != nil {
			item["maxLen"] = *rule.MaxLen
		}
		if rule.Pattern != "" {
			item["pattern"] = rule.Pattern
		}
		if rule.Format != "" {
			item["format"] = rule.Format
		}
		if len(rule.Enum) > 0 {
			item["enum"] = rule.Enum
		}
		validation[key] = item
	}
	return dto.TemplateView{
		TemplateVersion: tpl.TemplateVersion,
		FormSchema:      formSchema,
		SecretFields:    slices.Clone(tpl.SecretFields),
		FileFields:      fileFields,
		ValidationRules: validation,
	}
}

func toIAPConfigView(config map[string]any, cfg domainproduct.IAPConfig) dto.IAPConfigView {
	return dto.IAPConfigView{
		Enabled:          cfg.Enabled,
		ConfigStatus:     string(cfg.ConfigStatus),
		ConfigJSON:       config,
		LastCheckAt:      cfg.LastCheckAt,
		LastCheckMessage: cfg.LastCheckMessage,
	}
}

func maskConfig(config map[string]any, secretFields []string) map[string]any {
	out := cloneMap(config)
	for _, key := range secretFields {
		if _, ok := out[key]; ok {
			out[key] = maskedValue
		}
	}
	return out
}

func cloneMap(in map[string]any) map[string]any {
	if in == nil {
		return map[string]any{}
	}
	out := make(map[string]any, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}

func (s *IAPConfigService) writeAudit(ctx context.Context, action, resourceID string, detail map[string]any) {
	if s.audit == nil {
		return
	}
	actor := int64(0)
	if ac, ok := adminapp.AuthContextFrom(ctx); ok {
		actor = ac.UserID
	}
	s.audit.Write(ctx, AuditEntry{
		ActorID:      actor,
		Action:       action,
		ResourceType: "product",
		ResourceID:   resourceID,
		Detail:       detail,
	})
}
