package platformchannel

import (
	"context"
	"errors"
	"slices"
	"strconv"
	"strings"

	adminapp "github.com/csw/console/services/admin-api/internal/app/admin"
	"github.com/csw/console/services/admin-api/internal/app/dto"
	domainchannel "github.com/csw/console/services/admin-api/internal/domain/channel"
	"github.com/csw/console/services/admin-api/internal/domain/common"
)

// Service 平台渠道主数据与渠道模版的读/写用例（系统管理员维护，权限码 platform_channel.* / channel_template.*）。
type Service struct {
	tx    TxManager
	audit AuditSink
}

// NewService 构造服务。
func NewService(tx TxManager, audit AuditSink) *Service {
	return &Service{tx: tx, audit: audit}
}

// ===== 渠道主数据 =====

// ListChannels 平台渠道分页列表（GET /platform/channels）。
func (s *Service) ListChannels(ctx context.Context, q dto.ListPlatformChannelsQuery) (dto.Page[dto.PlatformChannelView], error) {
	empty := dto.Page[dto.PlatformChannelView]{}
	if q.Region != "" && !domainchannel.ChannelRegion(q.Region).IsKnown() {
		return empty, validationErr("发行市场非法", fieldDetail("region", "enum"))
	}
	if q.ChannelType != "" && !domainchannel.IsValidChannelType(q.ChannelType) {
		return empty, validationErr("channelType 非法", fieldDetail("channelType", "enum"))
	}
	page, pageSize := normalizePage(q.Page, q.PageSize)
	q.Page, q.PageSize = page, pageSize

	rows, total, err := s.tx.Repositories().Channels.List(ctx, q)
	if err != nil {
		return empty, err
	}
	items := make([]dto.PlatformChannelView, 0, len(rows))
	for i := range rows {
		items = append(items, toChannelView(rows[i]))
	}
	return dto.Page[dto.PlatformChannelView]{Items: items, Page: page, PageSize: pageSize, Total: total}, nil
}

// GetChannel 平台渠道详情（GET /platform/channels/{channelId}）。
func (s *Service) GetChannel(ctx context.Context, channelID string) (dto.PlatformChannelView, error) {
	row, err := s.loadChannel(ctx, strings.TrimSpace(channelID))
	if err != nil {
		return dto.PlatformChannelView{}, err
	}
	return toChannelView(row), nil
}

// CreateChannel 新建平台渠道 + 策略（POST /platform/channels）。
func (s *Service) CreateChannel(ctx context.Context, cmd dto.CreatePlatformChannelCmd) (dto.PlatformChannelView, error) {
	zero := dto.PlatformChannelView{}
	channelID := strings.TrimSpace(cmd.ChannelID)
	if issues := domainchannel.ValidateChannelID(channelID); len(issues) > 0 {
		return zero, validationErr(issues[0].Message, issueDetails(issues)...)
	}
	ch := domainchannel.Channel{
		ChannelID:   channelID,
		ChannelName: strings.TrimSpace(cmd.ChannelName),
		ChannelType: cmd.ChannelType,
		Region:      domainchannel.ChannelRegion(cmd.Region),
		Enabled:     boolOr(cmd.Enabled, true),
		Sort:        intOr(cmd.Sort, 0),
	}
	policy := domainchannel.ChannelPolicy{
		LoginMode:     common.LoginMode(cmd.LoginMode),
		PaymentMode:   common.PaymentMode(cmd.PaymentMode),
		LoginLocked:   boolOr(cmd.LoginLocked, false),
		PaymentLocked: boolOr(cmd.PaymentLocked, false),
	}
	issues := append(domainchannel.ValidateChannelMaster(ch), domainchannel.ValidateChannelPolicy(policy)...)
	if len(issues) > 0 {
		return zero, validationErr(issues[0].Message, issueDetails(issues)...)
	}

	err := s.tx.InTx(ctx, func(repos Repositories) error {
		if _, err := repos.Channels.GetByChannelID(ctx, channelID); err == nil {
			return conflictErr("渠道 ID 已存在：" + channelID)
		} else if !errors.Is(err, adminapp.ErrNotFound) {
			return err
		}
		return repos.Channels.Insert(ctx, ch, policy)
	})
	if err != nil {
		return zero, mapWriteErr(err, "渠道 ID 已存在："+channelID)
	}
	if err := s.writeAudit(ctx, "platform_channel.create", "platform_channel", channelID, map[string]any{
		"channelId": channelID, "channelType": ch.ChannelType, "region": string(ch.Region),
	}); err != nil {
		return zero, err
	}
	return s.GetChannel(ctx, channelID)
}

// UpdateChannel 编辑平台渠道 + 策略（PATCH /platform/channels/{channelId}）。
// channelType / region 创建后不可改：region 决定 market 兼容性，改动会让既有渠道实例集体失配。
func (s *Service) UpdateChannel(ctx context.Context, cmd dto.UpdatePlatformChannelCmd) (dto.PlatformChannelView, error) {
	zero := dto.PlatformChannelView{}
	channelID := strings.TrimSpace(cmd.ChannelID)
	row, err := s.loadChannel(ctx, channelID)
	if err != nil {
		return zero, err
	}

	master := ChannelMasterPatch{Enabled: cmd.Enabled, Sort: cmd.Sort}
	if cmd.ChannelName != nil {
		name := strings.TrimSpace(*cmd.ChannelName)
		master.ChannelName = &name
	}
	merged := row.Channel
	if master.ChannelName != nil {
		merged.ChannelName = *master.ChannelName
	}
	if master.Sort != nil {
		merged.Sort = *master.Sort
	}
	if master.Enabled != nil {
		merged.Enabled = *master.Enabled
	}

	policy := ChannelPolicyPatch{
		LoginMode:     cmd.LoginMode,
		PaymentMode:   cmd.PaymentMode,
		LoginLocked:   cmd.LoginLocked,
		PaymentLocked: cmd.PaymentLocked,
	}
	mergedPolicy := row.Policy
	if policy.LoginMode != nil {
		mergedPolicy.LoginMode = common.LoginMode(*policy.LoginMode)
	}
	if policy.PaymentMode != nil {
		mergedPolicy.PaymentMode = common.PaymentMode(*policy.PaymentMode)
	}
	if policy.LoginLocked != nil {
		mergedPolicy.LoginLocked = *policy.LoginLocked
	}
	if policy.PaymentLocked != nil {
		mergedPolicy.PaymentLocked = *policy.PaymentLocked
	}

	issues := append(domainchannel.ValidateChannelMaster(merged), domainchannel.ValidateChannelPolicy(mergedPolicy)...)
	if len(issues) > 0 {
		return zero, validationErr(issues[0].Message, issueDetails(issues)...)
	}

	changed := changedChannelFields(master, policy)
	if len(changed) == 0 {
		return toChannelView(row), nil
	}
	err = s.tx.InTx(ctx, func(repos Repositories) error {
		if master.ChannelName != nil || master.Enabled != nil || master.Sort != nil {
			if err := repos.Channels.UpdateMaster(ctx, channelID, master); err != nil {
				return err
			}
		}
		if policy.LoginMode != nil || policy.PaymentMode != nil || policy.LoginLocked != nil || policy.PaymentLocked != nil {
			return repos.Channels.UpsertPolicy(ctx, channelID, policy)
		}
		return nil
	})
	if err != nil {
		return zero, mapWriteErr(err, "渠道更新冲突")
	}
	if err := s.writeAudit(ctx, "platform_channel.update", "platform_channel", channelID, map[string]any{
		"channelId": channelID, "fields": changed,
	}); err != nil {
		return zero, err
	}
	return s.GetChannel(ctx, channelID)
}

// ===== 渠道模版 =====

// ListTemplates 列出某渠道的模版版本（GET /platform/channels/{channelId}/templates）。
// kind 为空表示登录 + IAP 全部返回。
func (s *Service) ListTemplates(ctx context.Context, channelID, kind string) ([]dto.ChannelTemplateView, error) {
	row, err := s.loadChannel(ctx, strings.TrimSpace(channelID))
	if err != nil {
		return nil, err
	}
	kinds := []domainchannel.ChannelTemplateKind{domainchannel.ChannelTemplateKindLogin, domainchannel.ChannelTemplateKindIAP}
	if kind != "" {
		k := domainchannel.ChannelTemplateKind(kind)
		if !domainchannel.IsValidChannelTemplateKind(k) {
			return nil, validationErr("kind 非法", fieldDetail("kind", "login/iap"))
		}
		kinds = []domainchannel.ChannelTemplateKind{k}
	}

	out := []dto.ChannelTemplateView{}
	repos := s.tx.Repositories()
	for _, k := range kinds {
		items, err := repos.Templates.ListByChannel(ctx, k, row.Channel.ID)
		if err != nil {
			return nil, err
		}
		// 仓储按 template_version 降序返回，运行时取「enabled 的最新版本」，故首个 enabled 即生效版本。
		effectiveFound := false
		for i := range items {
			items[i].ChannelID = row.Channel.ChannelID
			effective := false
			if !effectiveFound && items[i].Enabled {
				effective, effectiveFound = true, true
			}
			out = append(out, toTemplateView(items[i], effective))
		}
	}
	return out, nil
}

// GetTemplate 取单个模版版本（GET /platform/channel-templates/{kind}/{templateId}）。
func (s *Service) GetTemplate(ctx context.Context, kind string, templateID int64) (dto.ChannelTemplateView, error) {
	k, err := parseKind(kind)
	if err != nil {
		return dto.ChannelTemplateView{}, err
	}
	tpl, err := s.tx.Repositories().Templates.GetByID(ctx, k, templateID)
	if err != nil {
		return dto.ChannelTemplateView{}, mapLoadErr(err, "渠道模版不存在")
	}
	return s.withEffectiveFlag(ctx, tpl)
}

// CreateTemplate 新建模版版本（POST /platform/channels/{channelId}/templates）。
func (s *Service) CreateTemplate(ctx context.Context, cmd dto.CreateChannelTemplateCmd) (dto.ChannelTemplateView, error) {
	zero := dto.ChannelTemplateView{}
	k, err := parseKind(cmd.Kind)
	if err != nil {
		return zero, err
	}
	row, err := s.loadChannel(ctx, strings.TrimSpace(cmd.ChannelID))
	if err != nil {
		return zero, err
	}
	tpl := domainchannel.ChannelTemplate{
		Kind:            k,
		ChannelIDRef:    row.Channel.ID,
		ChannelID:       row.Channel.ChannelID,
		TemplateVersion: strings.TrimSpace(cmd.TemplateVersion),
		FormSchema:      toDomainFormSchema(cmd.FormSchema),
		SecretFields:    trimStrings(cmd.SecretFields),
		FileFields:      toDomainFileFields(cmd.FileFields),
		ValidationRules: toDomainRules(cmd.ValidationRules),
		Enabled:         boolOr(cmd.Enabled, true),
	}
	if issues := domainchannel.ValidateChannelTemplate(tpl); len(issues) > 0 {
		return zero, validationErr(issues[0].Message, issueDetails(issues)...)
	}

	var created domainchannel.ChannelTemplate
	err = s.tx.InTx(ctx, func(repos Repositories) error {
		inserted, err := repos.Templates.Insert(ctx, tpl)
		if err != nil {
			return err
		}
		created = inserted
		return nil
	})
	if err != nil {
		return zero, mapWriteErr(err, "该渠道下模版版本 "+tpl.TemplateVersion+" 已存在")
	}
	created.ChannelID = row.Channel.ChannelID
	if err := s.writeAudit(ctx, "channel_template.create", "channel_template", itoa(created.ID), map[string]any{
		"kind": string(k), "channelId": row.Channel.ChannelID, "templateVersion": created.TemplateVersion,
	}); err != nil {
		return zero, err
	}
	return s.withEffectiveFlag(ctx, created)
}

// UpdateTemplate 编辑模版版本（PATCH /platform/channel-templates/{kind}/{templateId}）。
// 四件套按整体替换合并：入参为 nil 的部分保留原值。
func (s *Service) UpdateTemplate(ctx context.Context, cmd dto.UpdateChannelTemplateCmd) (dto.ChannelTemplateView, error) {
	zero := dto.ChannelTemplateView{}
	k, err := parseKind(cmd.Kind)
	if err != nil {
		return zero, err
	}
	current, err := s.tx.Repositories().Templates.GetByID(ctx, k, cmd.TemplateID)
	if err != nil {
		return zero, mapLoadErr(err, "渠道模版不存在")
	}

	merged := current
	changed := []string{}
	if cmd.FormSchema != nil {
		merged.FormSchema = toDomainFormSchema(cmd.FormSchema)
		changed = append(changed, "formSchemaJson")
	}
	if cmd.SecretFields != nil {
		merged.SecretFields = trimStrings(cmd.SecretFields)
		changed = append(changed, "secretFieldsJson")
	}
	if cmd.FileFields != nil {
		merged.FileFields = toDomainFileFields(cmd.FileFields)
		changed = append(changed, "fileFieldsJson")
	}
	if cmd.ValidationRules != nil {
		merged.ValidationRules = toDomainRules(cmd.ValidationRules)
		changed = append(changed, "validationRulesJson")
	}
	if cmd.Enabled != nil {
		merged.Enabled = *cmd.Enabled
		changed = append(changed, "enabled")
	}
	if len(changed) == 0 {
		return s.withEffectiveFlag(ctx, current)
	}
	if issues := domainchannel.ValidateChannelTemplate(merged); len(issues) > 0 {
		return zero, validationErr(issues[0].Message, issueDetails(issues)...)
	}

	if err := s.tx.Repositories().Templates.Replace(ctx, merged); err != nil {
		return zero, mapWriteErr(err, "渠道模版更新冲突")
	}
	if err := s.writeAudit(ctx, "channel_template.update", "channel_template", itoa(merged.ID), map[string]any{
		"kind": string(k), "channelId": merged.ChannelID, "templateVersion": merged.TemplateVersion, "fields": changed,
	}); err != nil {
		return zero, err
	}
	return s.withEffectiveFlag(ctx, merged)
}

// ===== helpers =====

func (s *Service) loadChannel(ctx context.Context, channelID string) (PlatformChannelRow, error) {
	if channelID == "" {
		return PlatformChannelRow{}, validationErr("channelId 必填", fieldDetail("channelId", "required"))
	}
	row, err := s.tx.Repositories().Channels.GetByChannelID(ctx, channelID)
	if err != nil {
		return PlatformChannelRow{}, mapLoadErr(err, "渠道不存在")
	}
	return row, nil
}

// withEffectiveFlag 重新读取同渠道同类模版，判定 tpl 是否为当前生效版本后出视图。
// 仓储按 template_version 降序返回，运行时取「enabled 的最新版本」，故首个 enabled 即生效版本。
func (s *Service) withEffectiveFlag(ctx context.Context, tpl domainchannel.ChannelTemplate) (dto.ChannelTemplateView, error) {
	items, err := s.tx.Repositories().Templates.ListByChannel(ctx, tpl.Kind, tpl.ChannelIDRef)
	if err != nil {
		return dto.ChannelTemplateView{}, err
	}
	effective := false
	for _, item := range items {
		if tpl.ChannelID == "" {
			tpl.ChannelID = item.ChannelID
		}
		if item.Enabled {
			effective = item.ID == tpl.ID
			break
		}
	}
	return toTemplateView(tpl, effective), nil
}

func (s *Service) writeAudit(ctx context.Context, action, resourceType, resourceID string, detail map[string]any) error {
	if s.audit == nil {
		return nil
	}
	actor := int64(0)
	if ac, ok := adminapp.AuthContextFrom(ctx); ok {
		actor = ac.UserID
	}
	return s.audit.Write(ctx, AuditEntry{
		ActorID: actor, Action: action, ResourceType: resourceType, ResourceID: resourceID, Detail: detail,
	})
}

func parseKind(kind string) (domainchannel.ChannelTemplateKind, error) {
	k := domainchannel.ChannelTemplateKind(strings.TrimSpace(kind))
	if !domainchannel.IsValidChannelTemplateKind(k) {
		return "", validationErr("kind 非法", fieldDetail("kind", "login/iap"))
	}
	return k, nil
}

func toChannelView(row PlatformChannelRow) dto.PlatformChannelView {
	return dto.PlatformChannelView{
		ChannelID:          row.Channel.ChannelID,
		ChannelName:        row.Channel.ChannelName,
		ChannelType:        row.Channel.ChannelType,
		Region:             string(row.Channel.Region),
		Enabled:            row.Channel.Enabled,
		Sort:               row.Channel.Sort,
		LoginMode:          string(row.Policy.LoginMode),
		PaymentMode:        string(row.Policy.PaymentMode),
		LoginLocked:        row.Policy.LoginLocked,
		PaymentLocked:      row.Policy.PaymentLocked,
		LoginTemplateCount: row.LoginTemplateCount,
		IAPTemplateCount:   row.IAPTemplateCount,
		UpdatedAt:          row.UpdatedAt,
	}
}

func toTemplateView(tpl domainchannel.ChannelTemplate, effective bool) dto.ChannelTemplateView {
	form := make([]any, 0, len(tpl.FormSchema))
	for _, f := range tpl.FormSchema {
		entry := map[string]any{
			"key": f.Key, "label": f.Label, "component": f.Component,
			"required": f.Required, "order": f.Order, "group": f.Group, "scope": f.Scope,
		}
		if f.Placeholder != "" {
			entry["placeholder"] = f.Placeholder
		}
		if len(f.Options) > 0 {
			options := make([]any, 0, len(f.Options))
			for _, opt := range f.Options {
				options = append(options, map[string]any{"label": opt.Label, "value": opt.Value})
			}
			entry["options"] = options
		}
		form = append(form, entry)
	}

	files := make([]any, 0, len(tpl.FileFields))
	for _, f := range tpl.FileFields {
		entry := map[string]any{"key": f.Key}
		if len(f.Accept) > 0 {
			entry["accept"] = slices.Clone(f.Accept)
		}
		if f.MaxSizeKB != nil {
			entry["maxSizeKB"] = *f.MaxSizeKB
		}
		files = append(files, entry)
	}

	rules := map[string]any{}
	for key, rule := range tpl.ValidationRules {
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
		rules[key] = entry
	}

	secrets := tpl.SecretFields
	if secrets == nil {
		secrets = []string{}
	}
	return dto.ChannelTemplateView{
		TemplateID:          tpl.ID,
		Kind:                string(tpl.Kind),
		ChannelID:           tpl.ChannelID,
		TemplateVersion:     tpl.TemplateVersion,
		FormSchemaJSON:      form,
		SecretFieldsJSON:    slices.Clone(secrets),
		FileFieldsJSON:      files,
		ValidationRulesJSON: rules,
		Enabled:             tpl.Enabled,
		Effective:           effective,
		CreatedAt:           tpl.CreatedAt,
		UpdatedAt:           tpl.UpdatedAt,
	}
}

func toDomainFormSchema(in []dto.TemplateFieldInput) []domainchannel.ChannelLoginFormField {
	out := make([]domainchannel.ChannelLoginFormField, 0, len(in))
	for _, f := range in {
		field := domainchannel.ChannelLoginFormField{
			Key:         strings.TrimSpace(f.Key),
			Label:       strings.TrimSpace(f.Label),
			Component:   strings.TrimSpace(f.Component),
			Required:    f.Required,
			Order:       f.Order,
			Group:       strings.TrimSpace(f.Group),
			Scope:       strings.TrimSpace(f.Scope),
			Placeholder: f.Placeholder,
		}
		for _, opt := range f.Options {
			field.Options = append(field.Options, domainchannel.FieldOption{Label: opt.Label, Value: opt.Value})
		}
		out = append(out, field)
	}
	return out
}

func toDomainFileFields(in []dto.TemplateFileFieldInput) []domainchannel.ChannelLoginFileField {
	out := make([]domainchannel.ChannelLoginFileField, 0, len(in))
	for _, f := range in {
		out = append(out, domainchannel.ChannelLoginFileField{
			Key:       strings.TrimSpace(f.Key),
			Accept:    trimStrings(f.Accept),
			MaxSizeKB: f.MaxSizeKB,
		})
	}
	return out
}

func toDomainRules(in map[string]dto.TemplateRuleInput) map[string]domainchannel.ChannelLoginValidationRule {
	out := map[string]domainchannel.ChannelLoginValidationRule{}
	for key, rule := range in {
		out[strings.TrimSpace(key)] = domainchannel.ChannelLoginValidationRule{
			Required: rule.Required,
			MinLen:   rule.MinLen,
			MaxLen:   rule.MaxLen,
			Min:      rule.Min,
			Max:      rule.Max,
			Pattern:  rule.Pattern,
			Format:   rule.Format,
			Enum:     trimStrings(rule.Enum),
		}
	}
	return out
}

func trimStrings(in []string) []string {
	out := make([]string, 0, len(in))
	for _, item := range in {
		out = append(out, strings.TrimSpace(item))
	}
	return out
}

func changedChannelFields(master ChannelMasterPatch, policy ChannelPolicyPatch) []string {
	fields := []string{}
	if master.ChannelName != nil {
		fields = append(fields, "channelName")
	}
	if master.Enabled != nil {
		fields = append(fields, "enabled")
	}
	if master.Sort != nil {
		fields = append(fields, "sort")
	}
	if policy.LoginMode != nil {
		fields = append(fields, "loginMode")
	}
	if policy.PaymentMode != nil {
		fields = append(fields, "paymentMode")
	}
	if policy.LoginLocked != nil {
		fields = append(fields, "loginLocked")
	}
	if policy.PaymentLocked != nil {
		fields = append(fields, "paymentLocked")
	}
	return fields
}

func fieldDetail(field, reason string) any {
	return map[string]string{"field": field, "reason": reason}
}

func boolOr(p *bool, def bool) bool {
	if p != nil {
		return *p
	}
	return def
}

func intOr(p *int, def int) int {
	if p != nil {
		return *p
	}
	return def
}

func itoa(v int64) string { return strconv.FormatInt(v, 10) }

// normalizePage 归一化分页（00 §7.3：page>=1，pageSize 默认 20、最大 100）。
func normalizePage(page, pageSize int) (int, int) {
	if page < 1 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = 20
	}
	if pageSize > 100 {
		pageSize = 100
	}
	return page, pageSize
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

func mapWriteErr(err error, conflictMsg string) error {
	var appErr *Error
	if errors.As(err, &appErr) {
		return appErr
	}
	if errors.Is(err, adminapp.ErrConflict) {
		return conflictErr(conflictMsg)
	}
	if errors.Is(err, adminapp.ErrNotFound) {
		return notFoundErr("渠道模版不存在")
	}
	return err
}
