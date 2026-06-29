package accountauth

import (
	"context"
	"errors"
	"slices"
	"strings"
	"time"

	adminapp "github.com/csw/console/services/admin-api/internal/app/admin"
	"github.com/csw/console/services/admin-api/internal/app/dto"
	domainaa "github.com/csw/console/services/admin-api/internal/domain/accountauth"
	"github.com/csw/console/services/admin-api/internal/domain/common"
)

const maskedValue = "masked"

type Service struct {
	tx     TxManager
	cipher Cipher
	audit  AuditSink
	now    func() time.Time
}

func NewService(tx TxManager, cipher Cipher, audit AuditSink, now func() time.Time) *Service {
	if now == nil {
		now = time.Now
	}
	return &Service{tx: tx, cipher: cipher, audit: audit, now: now}
}

func (s *Service) ListTypes(ctx context.Context) ([]dto.AccountAuthTypeView, error) {
	items, err := s.tx.Repository().ListTypeCatalog(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]dto.AccountAuthTypeView, 0, len(items))
	for _, item := range items {
		out = append(out, dto.AccountAuthTypeView{
			AuthTypeID:   item.AuthTypeID,
			AuthTypeName: item.AuthTypeName,
			Enabled:      item.Enabled,
			Sort:         item.Sort,
			Template:     toTemplateView(item.Template),
		})
	}
	return out, nil
}

func (s *Service) ListChannelTypes(ctx context.Context, channelID string) ([]dto.ChannelAccountAuthTypeView, error) {
	channelID = strings.TrimSpace(channelID)
	if channelID == "" {
		return nil, validationErr("channelId 不能为空", fieldDetail("channelId", "required"))
	}
	items, err := s.tx.Repository().ListChannelPolicies(ctx, channelID)
	if err != nil {
		return nil, mapLoadErr(err, "channel not found")
	}
	out := make([]dto.ChannelAccountAuthTypeView, 0, len(items))
	for _, item := range items {
		out = append(out, dto.ChannelAccountAuthTypeView{
			AuthTypeID:     item.AuthTypeID,
			DefaultEnabled: item.DefaultEnabled,
			Locked:         item.Locked,
		})
	}
	return out, nil
}

func (s *Service) GetGameConfigs(ctx context.Context, gameID string) ([]dto.GameAccountAuthConfigView, error) {
	gameIDRef, allowed, existing, err := s.loadGameState(ctx, gameID)
	if err != nil {
		return nil, err
	}
	_ = gameIDRef
	return s.mergeViews(allowed, existing), nil
}

func (s *Service) ReplaceGameConfigs(ctx context.Context, cmd dto.ReplaceGameAccountAuthConfigsCmd) ([]dto.GameAccountAuthConfigView, error) {
	_, allowed, existing, err := s.loadGameState(ctx, cmd.GameID)
	if err != nil {
		return nil, err
	}
	allowedByID := map[string]GameAllowedType{}
	for _, item := range allowed {
		allowedByID[item.AuthTypeID] = item
	}
	for _, item := range cmd.Items {
		if _, ok := allowedByID[item.AuthTypeID]; !ok {
			return nil, typeNotAllowedErr("authTypeId 不在该游戏渠道允许集合中: " + item.AuthTypeID)
		}
	}

	reqByID := map[string]dto.ReplaceGameAccountAuthConfigItem{}
	for _, item := range cmd.Items {
		reqByID[item.AuthTypeID] = item
	}
	existingByRef := map[int64]GameConfigItem{}
	for _, e := range existing {
		existingByRef[e.AuthTypeIDRef] = e
	}

	upserts := make([]GameConfigUpsert, 0, len(allowed))
	now := s.now()
	for _, allow := range allowed {
		req, hasReq := reqByID[allow.AuthTypeID]
		ex, hasExisting := existingByRef[allow.AuthTypeIDRef]

		enabled := false
		config := map[string]any{}
		if hasExisting {
			enabled = ex.Enabled
			config = cloneAnyMap(ex.ConfigJSON)
		}
		if hasReq {
			if req.Enabled != nil {
				enabled = *req.Enabled
			}
			if req.ConfigJSON != nil {
				config = mergeConfigWithExisting(req.ConfigJSON, ex.ConfigJSON, allow.Template)
			}
		} else if !hasExisting && allow.DefaultEnabled {
			enabled = true
		}
		if allow.Locked {
			if hasExisting {
				enabled = ex.Enabled
				config = cloneAnyMap(ex.ConfigJSON)
			} else {
				enabled = true
			}
		}

		status := common.ConfigStatusEmpty
		message := ""
		var checkAt *time.Time
		if enabled || len(config) > 0 {
			if allow.Template.TemplateVersion == "" {
				return nil, templateNotFoundErr("认证类型缺少可用模板: " + allow.AuthTypeID)
			}
			effectiveConfig, err := s.encryptSecrets(config, allow.Template.SecretFields, ex.ConfigJSON)
			if err != nil {
				return nil, err
			}
			status, message = domainaa.ValidateConfigAgainstTemplate(effectiveConfig, allow.Template)
			if status == common.ConfigStatusValid {
				t := now
				checkAt = &t
			}
			config = effectiveConfig
		}
		upserts = append(upserts, GameConfigUpsert{
			AuthTypeIDRef:    allow.AuthTypeIDRef,
			Enabled:          enabled,
			ConfigJSON:       config,
			ConfigStatus:     status,
			LastCheckAt:      checkAt,
			LastCheckMessage: message,
		})
	}

	gameIDRef, err := s.tx.Repository().ResolveGameRowID(ctx, cmd.GameID)
	if err != nil {
		return nil, mapLoadErr(err, "game not found")
	}
	if err := s.tx.InTx(ctx, func(repo Repository) error {
		return repo.ReplaceGameConfigs(ctx, gameIDRef, upserts)
	}); err != nil {
		return nil, mapWriteErr(err)
	}
	s.writeAudit(ctx, cmd.GameID, allowed, existingByRef, upserts)

	_, allowedAfter, existingAfter, err := s.loadGameState(ctx, cmd.GameID)
	if err != nil {
		return nil, err
	}
	return s.mergeViews(allowedAfter, existingAfter), nil
}

func (s *Service) loadGameState(ctx context.Context, gameID string) (int64, []GameAllowedType, []GameConfigItem, error) {
	gameID = strings.TrimSpace(gameID)
	if gameID == "" {
		return 0, nil, nil, validationErr("gameId 不能为空", fieldDetail("gameId", "required"))
	}
	repo := s.tx.Repository()
	gameIDRef, err := repo.ResolveGameRowID(ctx, gameID)
	if err != nil {
		return 0, nil, nil, mapLoadErr(err, "game not found")
	}
	allowed, err := repo.ListAllowedTypesByGame(ctx, gameIDRef)
	if err != nil {
		return 0, nil, nil, err
	}
	existing, err := repo.ListGameConfigs(ctx, gameIDRef)
	if err != nil {
		return 0, nil, nil, err
	}
	return gameIDRef, allowed, existing, nil
}

func (s *Service) mergeViews(allowed []GameAllowedType, existing []GameConfigItem) []dto.GameAccountAuthConfigView {
	existingByRef := map[int64]GameConfigItem{}
	for _, item := range existing {
		existingByRef[item.AuthTypeIDRef] = item
	}
	out := make([]dto.GameAccountAuthConfigView, 0, len(allowed))
	for _, item := range allowed {
		cfg, ok := existingByRef[item.AuthTypeIDRef]
		if !ok {
			cfg = GameConfigItem{
				AuthTypeIDRef:    item.AuthTypeIDRef,
				Enabled:          item.DefaultEnabled,
				ConfigJSON:       map[string]any{},
				ConfigStatus:     common.ConfigStatusEmpty,
				LastCheckMessage: "",
			}
		}
		if item.Locked {
			cfg.Enabled = true
		}
		out = append(out, dto.GameAccountAuthConfigView{
			AuthTypeID:       item.AuthTypeID,
			Enabled:          cfg.Enabled,
			ConfigJSON:       maskConfig(cfg.ConfigJSON, item.Template.SecretFields),
			ConfigStatus:     string(cfg.ConfigStatus),
			LastCheckAt:      cfg.LastCheckAt,
			LastCheckMessage: cfg.LastCheckMessage,
		})
	}
	return out
}

func (s *Service) encryptSecrets(config map[string]any, secretFields []string, existing map[string]any) (map[string]any, error) {
	out := cloneAnyMap(config)
	for _, key := range secretFields {
		raw, ok := out[key]
		if !ok {
			continue
		}
		str, ok := raw.(string)
		if !ok {
			continue
		}
		if strings.TrimSpace(str) == "" {
			if prev, hasPrev := existing[key]; hasPrev {
				out[key] = prev
			}
			continue
		}
		if str == maskedValue {
			if prev, hasPrev := existing[key]; hasPrev {
				out[key] = prev
			}
			continue
		}
		if prev, hasPrev := existing[key]; hasPrev && str == prev {
			continue
		}
		if s.cipher == nil {
			return nil, encryptionErr("secret 字段加密器未配置")
		}
		encrypted, err := s.cipher.Encrypt(str)
		if err != nil {
			return nil, encryptionErr("secret 字段加密失败")
		}
		out[key] = encrypted
	}
	return out, nil
}

func maskConfig(config map[string]any, secretFields []string) map[string]any {
	out := cloneAnyMap(config)
	for _, key := range secretFields {
		if _, ok := out[key]; ok {
			out[key] = maskedValue
		}
	}
	return out
}

func cloneAnyMap(in map[string]any) map[string]any {
	if in == nil {
		return map[string]any{}
	}
	out := make(map[string]any, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}

// mergeConfigWithExisting 请求局部 configJson 时，未提交的 secret/file 字段保留已有值（留空=不修改）。
func mergeConfigWithExisting(req, existing map[string]any, tpl domainaa.Template) map[string]any {
	out := cloneAnyMap(req)
	if existing == nil {
		return out
	}
	for _, key := range tpl.SecretFields {
		if _, ok := out[key]; !ok {
			if prev, hasPrev := existing[key]; hasPrev {
				out[key] = prev
			}
		}
	}
	for _, f := range tpl.FileFields {
		if _, ok := out[f.Key]; !ok {
			if prev, hasPrev := existing[f.Key]; hasPrev {
				out[f.Key] = prev
			}
		}
	}
	return out
}

func toTemplateView(tpl domainaa.Template) dto.AccountAuthTemplateView {
	formSchema := make([]any, 0, len(tpl.FormSchema))
	for _, item := range tpl.FormSchema {
		formSchema = append(formSchema, map[string]any{
			"key": item.Key, "label": item.Label, "component": item.Component,
			"required": item.Required, "order": item.Order, "scope": item.Scope,
		})
	}
	fileFields := make([]any, 0, len(tpl.FileFields))
	for _, item := range tpl.FileFields {
		fileFields = append(fileFields, map[string]any{"key": item.Key})
	}
	validation := map[string]any{}
	for key, rule := range tpl.ValidationRules {
		val := map[string]any{}
		if rule.Required {
			val["required"] = true
		}
		if rule.MinLen != nil {
			val["minLen"] = *rule.MinLen
		}
		if rule.MaxLen != nil {
			val["maxLen"] = *rule.MaxLen
		}
		if rule.Pattern != "" {
			val["pattern"] = rule.Pattern
		}
		if rule.Format != "" {
			val["format"] = rule.Format
		}
		if len(rule.Enum) > 0 {
			val["enum"] = slices.Clone(rule.Enum)
		}
		validation[key] = val
	}
	return dto.AccountAuthTemplateView{
		TemplateVersion: tpl.TemplateVersion,
		FormSchema:      formSchema,
		SecretFields:    slices.Clone(tpl.SecretFields),
		FileFields:      fileFields,
		ValidationRules: validation,
	}
}

func mapLoadErr(err error, notFoundMsg string) error {
	var appErr *Error
	if errors.As(err, &appErr) {
		return appErr
	}
	if errors.Is(err, adminapp.ErrNotFound) {
		return notFoundErr(notFoundMsg)
	}
	return err
}

// mapWriteErr 透传应用层 *Error；其余仓储错误原样返回（由 httpx 兜底为 INTERNAL）。
// 注意：本模块的写为「整体替换」（DELETE+INSERT，按 allowed 集合构造去重 upsert），
// 不做乐观并发检测，采用 last-writer-wins，因此不暴露业务级 CONFLICT（见 handoff CONFLICT 选择）。
func mapWriteErr(err error) error {
	var appErr *Error
	if errors.As(err, &appErr) {
		return appErr
	}
	return err
}

// writeAudit 写 game.account_auth.update 审计（00 §8）。
// detail_json 记录每项 authTypeId 与 enabled/configStatus 的 before/after；
// 绝不写入 configJson / secret（脱敏：仅记状态位，不记任何字段值）。
// 注：当前 wiring 注入 audit=nil（与 game/channel 一致的已知遗留，待 audit 模块统一接通），
// 届时本调用即可真正落 audit_logs，无需改动 service 层。
func (s *Service) writeAudit(ctx context.Context, gameID string, allowed []GameAllowedType, before map[int64]GameConfigItem, upserts []GameConfigUpsert) {
	if s.audit == nil {
		return
	}
	actor := int64(0)
	if ac, ok := adminapp.AuthContextFrom(ctx); ok {
		actor = ac.UserID
	}
	authTypeByRef := map[int64]string{}
	for _, a := range allowed {
		authTypeByRef[a.AuthTypeIDRef] = a.AuthTypeID
	}
	items := make([]map[string]any, 0, len(upserts))
	for _, up := range upserts {
		prev, hadPrev := before[up.AuthTypeIDRef]
		enabledBefore := false
		statusBefore := string(common.ConfigStatusEmpty)
		if hadPrev {
			enabledBefore = prev.Enabled
			statusBefore = string(prev.ConfigStatus)
		}
		items = append(items, map[string]any{
			"authTypeId":         authTypeByRef[up.AuthTypeIDRef],
			"enabledBefore":      enabledBefore,
			"enabledAfter":       up.Enabled,
			"configStatusBefore": statusBefore,
			"configStatusAfter":  string(up.ConfigStatus),
		})
	}
	s.audit.Write(ctx, AuditEntry{
		ActorID:      actor,
		Action:       "game.account_auth.update",
		ResourceType: "game",
		ResourceID:   gameID,
		Detail: map[string]any{
			"items": items,
		},
	})
}
