package plugin

import (
	"context"
	"errors"
	"strconv"
	"strings"

	adminapp "github.com/csw/console/services/admin-api/internal/app/admin"
	"github.com/csw/console/services/admin-api/internal/app/dto"
	"github.com/csw/console/services/admin-api/internal/domain/common"
	domainplugin "github.com/csw/console/services/admin-api/internal/domain/plugin"
)

type Service struct {
	tx     TxManager
	cipher Cipher
	audit  AuditSink
	env    common.Environment
}

func NewService(tx TxManager, cipher Cipher, audit AuditSink, env common.Environment) *Service {
	return &Service{tx: tx, cipher: cipher, audit: audit, env: env}
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

func mapWriteErr(err error, msg string) error {
	var appErr *Error
	if errors.As(err, &appErr) {
		return appErr
	}
	if errors.Is(err, adminapp.ErrConflict) {
		return conflictErr(msg)
	}
	if errors.Is(err, adminapp.ErrNotFound) {
		return notFoundErr("resource not found")
	}
	return err
}

func boolOr(v *bool, def bool) bool {
	if v != nil {
		return *v
	}
	return def
}

func cloneMap(src map[string]any) map[string]any {
	if src == nil {
		return map[string]any{}
	}
	dst := make(map[string]any, len(src))
	for k, v := range src {
		dst[k] = v
	}
	return dst
}

func (s *Service) writeAudit(ctx context.Context, action, resourceType string, resourceID int64, detail map[string]any) error {
	if s.audit == nil {
		return nil
	}
	actor := int64(0)
	if ac, ok := adminapp.AuthContextFrom(ctx); ok {
		actor = ac.UserID
	}
	return s.audit.Write(ctx, AuditEntry{
		ActorID:      actor,
		Action:       action,
		ResourceType: resourceType,
		ResourceID:   strconv.FormatInt(resourceID, 10),
		Detail:       detail,
	})
}

func templateFrom(tpl *FeaturePluginTemplate) domainplugin.ConfigTemplate {
	if tpl == nil {
		return domainplugin.ConfigTemplate{}
	}
	return domainplugin.ConfigTemplate{
		FormSchema:      tpl.FormSchema,
		SecretFields:    tpl.SecretFields,
		FileFields:      tpl.FileFields,
		ValidationRules: tpl.ValidationRules,
	}
}

func maskSecrets(config map[string]any, secretFields []string) map[string]any {
	out := cloneMap(config)
	for _, key := range secretFields {
		if _, ok := out[key]; ok {
			out[key] = "masked"
		}
	}
	return out
}

func (s *Service) encryptSecrets(config map[string]any, secretFields []string, old map[string]any) (map[string]any, error) {
	out := cloneMap(config)
	for _, key := range secretFields {
		v, ok := out[key]
		if !ok {
			continue
		}
		str, ok := v.(string)
		if !ok {
			continue
		}
		str = strings.TrimSpace(str)
		if str == "" {
			delete(out, key)
			continue
		}
		if str == "masked" || str == "******" {
			if old != nil {
				if prev, ok := old[key]; ok {
					out[key] = prev
					continue
				}
			}
			delete(out, key)
			continue
		}
		if s.cipher == nil {
			return nil, validationErr("密文字段加密器未配置")
		}
		enc, err := s.cipher.Encrypt(str)
		if err != nil {
			return nil, validationErr("密文字段加密失败")
		}
		out[key] = enc
	}
	return out, nil
}

func makeConfigView(cfg GameChannelPluginConfig, meta FeaturePluginMeta, tpl *FeaturePluginTemplate, gc GameChannelContext) dto.ChannelPluginItemView {
	flags := domainplugin.ResolveRuntimeFlags(gc.Hidden, domainplugin.ValidatePluginRegionCompatibility(gc.Market, meta.Region), cfg.Enabled, common.ConfigStatus(cfg.ConfigStatus))
	view := dto.ChannelPluginItemView{
		ID:                      cfg.ID,
		PluginID:                meta.PluginID,
		PluginName:              meta.Name,
		Region:                  meta.Region,
		Required:                meta.Required,
		Selectable:              meta.Selectable,
		Locked:                  meta.Locked,
		Enabled:                 cfg.Enabled,
		ConfigJSON:              maskSecrets(cfg.ConfigJSON, secretFieldsOf(tpl)),
		ConfigStatus:            cfg.ConfigStatus,
		LastCheckMessage:        cfg.LastCheckMessage,
		LastCheckAt:             cfg.LastCheckAt,
		IncludedInRuntimeConfig: flags.IncludedInRuntimeConfig,
		IncludedInSnapshot:      flags.IncludedInSnapshot,
		IncludedInSync:          flags.IncludedInSync,
		Template:                templateView(tpl),
	}
	if !cfg.UpdatedAt.IsZero() {
		t := cfg.UpdatedAt
		view.UpdatedAt = &t
	}
	return view
}

func templateView(tpl *FeaturePluginTemplate) dto.FeaturePluginTemplateView {
	if tpl == nil {
		return dto.FeaturePluginTemplateView{
			FormSchemaJSON:      []domainplugin.TemplateField{},
			SecretFieldsJSON:    []string{},
			FileFieldsJSON:      []domainplugin.FileField{},
			ValidationRulesJSON: map[string]domainplugin.ValidationRule{},
		}
	}
	secretFields := tpl.SecretFields
	if secretFields == nil {
		secretFields = []string{}
	}
	formSchema := tpl.FormSchema
	if formSchema == nil {
		formSchema = []domainplugin.TemplateField{}
	}
	fileFields := tpl.FileFields
	if fileFields == nil {
		fileFields = []domainplugin.FileField{}
	}
	rules := tpl.ValidationRules
	if rules == nil {
		rules = map[string]domainplugin.ValidationRule{}
	}
	return dto.FeaturePluginTemplateView{
		TemplateVersion:     tpl.TemplateVersion,
		FormSchemaJSON:      formSchema,
		SecretFieldsJSON:    secretFields,
		FileFieldsJSON:      fileFields,
		ValidationRulesJSON: rules,
	}
}

func secretFieldsOf(tpl *FeaturePluginTemplate) []string {
	if tpl == nil {
		return nil
	}
	return tpl.SecretFields
}
