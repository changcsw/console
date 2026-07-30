package postgres

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"

	"github.com/csw/console/services/admin-api/internal/app/dto"
	platformchannelapp "github.com/csw/console/services/admin-api/internal/app/platformchannel"
	domainchannel "github.com/csw/console/services/admin-api/internal/domain/channel"
	"github.com/csw/console/services/admin-api/internal/domain/common"
)

// PlatformChannelRepo 平台渠道主数据/策略读写仓储（platform.channels / platform.channel_policies）。
// 平台表显式写 platform 前缀：这些表全环境共享，不随 search_path 的 env schema 变化。
type PlatformChannelRepo struct{ db DBTX }

const platformChannelSelect = `
SELECT c.id, c.channel_id, c.channel_name, c.channel_type, c.region, c.enabled, c.sort, c.updated_at,
       COALESCE(p.login_mode, 'account_system'), COALESCE(p.payment_mode, 'hybrid'),
       COALESCE(p.login_locked, FALSE), COALESCE(p.payment_locked, FALSE),
       (SELECT COUNT(*) FROM platform.channel_login_templates t WHERE t.channel_id_ref = c.id),
       (SELECT COUNT(*) FROM platform.channel_iap_templates t WHERE t.channel_id_ref = c.id)
FROM platform.channels c
LEFT JOIN platform.channel_policies p ON p.channel_id_ref = c.id`

// List 分页列出渠道（含策略与模版版本数），按 sort、id 升序。
func (r *PlatformChannelRepo) List(ctx context.Context, q dto.ListPlatformChannelsQuery) ([]platformchannelapp.PlatformChannelRow, int, error) {
	where := []string{}
	args := []any{}
	if kw := strings.TrimSpace(q.Keyword); kw != "" {
		args = append(args, "%"+kw+"%")
		where = append(where, fmt.Sprintf("(c.channel_id ILIKE $%d OR c.channel_name ILIKE $%d)", len(args), len(args)))
	}
	if q.Region != "" {
		args = append(args, q.Region)
		where = append(where, fmt.Sprintf("c.region = $%d", len(args)))
	}
	if q.ChannelType != "" {
		args = append(args, q.ChannelType)
		where = append(where, fmt.Sprintf("c.channel_type = $%d", len(args)))
	}
	if q.Enabled != nil {
		args = append(args, *q.Enabled)
		where = append(where, fmt.Sprintf("c.enabled = $%d", len(args)))
	}
	clause := ""
	if len(where) > 0 {
		clause = " WHERE " + strings.Join(where, " AND ")
	}

	var total int
	if err := r.db.QueryRow(ctx, `SELECT COUNT(*) FROM platform.channels c`+clause, args...).Scan(&total); err != nil {
		return nil, 0, mapErr(err)
	}

	args = append(args, q.PageSize, (q.Page-1)*q.PageSize)
	sql := platformChannelSelect + clause +
		fmt.Sprintf(" ORDER BY c.sort ASC, c.id ASC LIMIT $%d OFFSET $%d", len(args)-1, len(args))
	rows, err := r.db.Query(ctx, sql, args...)
	if err != nil {
		return nil, 0, mapErr(err)
	}
	defer rows.Close()
	out := []platformchannelapp.PlatformChannelRow{}
	for rows.Next() {
		row, err := scanPlatformChannelRow(rows)
		if err != nil {
			return nil, 0, mapErr(err)
		}
		out = append(out, row)
	}
	return out, total, mapErr(rows.Err())
}

// GetByChannelID 按业务键取单行。
func (r *PlatformChannelRepo) GetByChannelID(ctx context.Context, channelID string) (platformchannelapp.PlatformChannelRow, error) {
	row := r.db.QueryRow(ctx, platformChannelSelect+` WHERE c.channel_id = $1`, channelID)
	out, err := scanPlatformChannelRow(row)
	if err != nil {
		return platformchannelapp.PlatformChannelRow{}, mapErr(err)
	}
	return out, nil
}

// Insert 落库渠道主数据 + 策略（同事务内保证一对一）。
func (r *PlatformChannelRepo) Insert(ctx context.Context, ch domainchannel.Channel, policy domainchannel.ChannelPolicy) error {
	var id int64
	err := r.db.QueryRow(ctx, `
INSERT INTO platform.channels (channel_id, channel_name, channel_type, region, enabled, sort)
VALUES ($1, $2, $3, $4, $5, $6)
RETURNING id`,
		ch.ChannelID, ch.ChannelName, ch.ChannelType, string(ch.Region), ch.Enabled, ch.Sort,
	).Scan(&id)
	if err != nil {
		return mapErr(err)
	}
	_, err = r.db.Exec(ctx, `
INSERT INTO platform.channel_policies (channel_id_ref, login_mode, payment_mode, login_locked, payment_locked)
VALUES ($1, $2, $3, $4, $5)`,
		id, string(policy.LoginMode), string(policy.PaymentMode), policy.LoginLocked, policy.PaymentLocked,
	)
	return mapErr(err)
}

// UpdateMaster 更新主数据可变列（channel_id / channel_type / region 不可改）。
func (r *PlatformChannelRepo) UpdateMaster(ctx context.Context, channelID string, patch platformchannelapp.ChannelMasterPatch) error {
	sets := []string{}
	args := []any{}
	if patch.ChannelName != nil {
		args = append(args, *patch.ChannelName)
		sets = append(sets, fmt.Sprintf("channel_name = $%d", len(args)))
	}
	if patch.Enabled != nil {
		args = append(args, *patch.Enabled)
		sets = append(sets, fmt.Sprintf("enabled = $%d", len(args)))
	}
	if patch.Sort != nil {
		args = append(args, *patch.Sort)
		sets = append(sets, fmt.Sprintf("sort = $%d", len(args)))
	}
	if len(sets) == 0 {
		return nil
	}
	sets = append(sets, "updated_at = NOW()")
	args = append(args, channelID)
	sql := `UPDATE platform.channels SET ` + strings.Join(sets, ", ") +
		fmt.Sprintf(" WHERE channel_id = $%d", len(args))
	_, err := r.db.Exec(ctx, sql, args...)
	return mapErr(err)
}

// UpsertPolicy 更新策略；该渠道尚无策略行时插入（保持渠道与策略一对一）。
func (r *PlatformChannelRepo) UpsertPolicy(ctx context.Context, channelID string, patch platformchannelapp.ChannelPolicyPatch) error {
	_, err := r.db.Exec(ctx, `
INSERT INTO platform.channel_policies (channel_id_ref, login_mode, payment_mode, login_locked, payment_locked)
SELECT c.id,
       COALESCE($2::varchar, 'account_system'),
       COALESCE($3::varchar, 'hybrid'),
       COALESCE($4::boolean, FALSE),
       COALESCE($5::boolean, FALSE)
FROM platform.channels c
WHERE c.channel_id = $1
ON CONFLICT (channel_id_ref) DO UPDATE SET
  login_mode     = COALESCE($2::varchar, channel_policies.login_mode),
  payment_mode   = COALESCE($3::varchar, channel_policies.payment_mode),
  login_locked   = COALESCE($4::boolean, channel_policies.login_locked),
  payment_locked = COALESCE($5::boolean, channel_policies.payment_locked),
  updated_at     = NOW()`,
		channelID, patch.LoginMode, patch.PaymentMode, patch.LoginLocked, patch.PaymentLocked,
	)
	return mapErr(err)
}

func scanPlatformChannelRow(row interface{ Scan(...any) error }) (platformchannelapp.PlatformChannelRow, error) {
	var (
		out                            platformchannelapp.PlatformChannelRow
		region, loginMode, paymentMode string
		loginLocked, paymentLocked     bool
	)
	if err := row.Scan(
		&out.Channel.ID, &out.Channel.ChannelID, &out.Channel.ChannelName, &out.Channel.ChannelType,
		&region, &out.Channel.Enabled, &out.Channel.Sort, &out.UpdatedAt,
		&loginMode, &paymentMode, &loginLocked, &paymentLocked,
		&out.LoginTemplateCount, &out.IAPTemplateCount,
	); err != nil {
		return platformchannelapp.PlatformChannelRow{}, err
	}
	out.Channel.Region = domainchannel.ChannelRegion(region)
	out.Policy = domainchannel.ChannelPolicy{
		ChannelIDRef:  out.Channel.ID,
		LoginMode:     common.LoginMode(loginMode),
		PaymentMode:   common.PaymentMode(paymentMode),
		LoginLocked:   loginLocked,
		PaymentLocked: paymentLocked,
	}
	return out, nil
}

// ChannelTemplateAdminRepo 渠道模版读写仓储。两张模版表结构同构，按 kind 选表。
type ChannelTemplateAdminRepo struct{ db DBTX }

// templateTable 按 kind 解析落库表名（仅内部枚举映射，不接受外部字符串拼接）。
func templateTable(kind domainchannel.ChannelTemplateKind) (string, error) {
	switch kind {
	case domainchannel.ChannelTemplateKindLogin:
		return "platform.channel_login_templates", nil
	case domainchannel.ChannelTemplateKindIAP:
		return "platform.channel_iap_templates", nil
	default:
		return "", fmt.Errorf("unknown channel template kind: %q", kind)
	}
}

const templateColumns = `t.id, t.channel_id_ref, c.channel_id, t.template_version,
       t.form_schema_json, t.secret_fields_json, t.file_fields_json, t.validation_rules_json,
       t.enabled, t.created_at, t.updated_at`

// ListByChannel 列出该渠道某类模版的全部版本（按 template_version 降序，与运行时取版本口径一致）。
func (r *ChannelTemplateAdminRepo) ListByChannel(
	ctx context.Context, kind domainchannel.ChannelTemplateKind, channelIDRef int64,
) ([]domainchannel.ChannelTemplate, error) {
	table, err := templateTable(kind)
	if err != nil {
		return nil, err
	}
	rows, err := r.db.Query(ctx, `SELECT `+templateColumns+` FROM `+table+` t
JOIN platform.channels c ON c.id = t.channel_id_ref
WHERE t.channel_id_ref = $1
ORDER BY t.template_version DESC, t.id DESC`, channelIDRef)
	if err != nil {
		return nil, mapErr(err)
	}
	defer rows.Close()
	out := []domainchannel.ChannelTemplate{}
	for rows.Next() {
		tpl, err := scanChannelTemplate(rows, kind)
		if err != nil {
			return nil, mapErr(err)
		}
		out = append(out, tpl)
	}
	return out, mapErr(rows.Err())
}

// GetByID 取单个模版版本。
func (r *ChannelTemplateAdminRepo) GetByID(
	ctx context.Context, kind domainchannel.ChannelTemplateKind, id int64,
) (domainchannel.ChannelTemplate, error) {
	table, err := templateTable(kind)
	if err != nil {
		return domainchannel.ChannelTemplate{}, err
	}
	row := r.db.QueryRow(ctx, `SELECT `+templateColumns+` FROM `+table+` t
JOIN platform.channels c ON c.id = t.channel_id_ref
WHERE t.id = $1`, id)
	tpl, err := scanChannelTemplate(row, kind)
	if err != nil {
		return domainchannel.ChannelTemplate{}, mapErr(err)
	}
	return tpl, nil
}

// Insert 落库新版本。
func (r *ChannelTemplateAdminRepo) Insert(
	ctx context.Context, tpl domainchannel.ChannelTemplate,
) (domainchannel.ChannelTemplate, error) {
	table, err := templateTable(tpl.Kind)
	if err != nil {
		return domainchannel.ChannelTemplate{}, err
	}
	form, secret, file, rules, err := marshalTemplateJSON(tpl)
	if err != nil {
		return domainchannel.ChannelTemplate{}, err
	}
	out := tpl
	err = r.db.QueryRow(ctx, `INSERT INTO `+table+` (
  channel_id_ref, template_version, form_schema_json, secret_fields_json, file_fields_json, validation_rules_json, enabled
) VALUES ($1, $2, $3::jsonb, $4::jsonb, $5::jsonb, $6::jsonb, $7)
RETURNING id, created_at, updated_at`,
		tpl.ChannelIDRef, tpl.TemplateVersion, form, secret, file, rules, tpl.Enabled,
	).Scan(&out.ID, &out.CreatedAt, &out.UpdatedAt)
	if err != nil {
		return domainchannel.ChannelTemplate{}, mapErr(err)
	}
	return out, nil
}

// Replace 整体覆盖四件套与 enabled（template_version 与所属渠道不可改）。
func (r *ChannelTemplateAdminRepo) Replace(ctx context.Context, tpl domainchannel.ChannelTemplate) error {
	table, err := templateTable(tpl.Kind)
	if err != nil {
		return err
	}
	form, secret, file, rules, err := marshalTemplateJSON(tpl)
	if err != nil {
		return err
	}
	tag, err := r.db.Exec(ctx, `UPDATE `+table+` SET
  form_schema_json      = $2::jsonb,
  secret_fields_json    = $3::jsonb,
  file_fields_json      = $4::jsonb,
  validation_rules_json = $5::jsonb,
  enabled               = $6,
  updated_at            = NOW()
WHERE id = $1`, tpl.ID, form, secret, file, rules, tpl.Enabled)
	if err != nil {
		return mapErr(err)
	}
	if tag.RowsAffected() == 0 {
		return mapErr(pgx.ErrNoRows)
	}
	return nil
}

func marshalTemplateJSON(tpl domainchannel.ChannelTemplate) (form, secret, file, rules string, err error) {
	formSchema := tpl.FormSchema
	if formSchema == nil {
		formSchema = []domainchannel.ChannelLoginFormField{}
	}
	secretFields := tpl.SecretFields
	if secretFields == nil {
		secretFields = []string{}
	}
	fileFields := tpl.FileFields
	if fileFields == nil {
		fileFields = []domainchannel.ChannelLoginFileField{}
	}
	validationRules := tpl.ValidationRules
	if validationRules == nil {
		validationRules = map[string]domainchannel.ChannelLoginValidationRule{}
	}
	formRaw, err := json.Marshal(formSchema)
	if err != nil {
		return "", "", "", "", fmt.Errorf("marshal form_schema_json: %w", err)
	}
	secretRaw, err := json.Marshal(secretFields)
	if err != nil {
		return "", "", "", "", fmt.Errorf("marshal secret_fields_json: %w", err)
	}
	fileRaw, err := json.Marshal(fileFields)
	if err != nil {
		return "", "", "", "", fmt.Errorf("marshal file_fields_json: %w", err)
	}
	rulesRaw, err := json.Marshal(validationRules)
	if err != nil {
		return "", "", "", "", fmt.Errorf("marshal validation_rules_json: %w", err)
	}
	return string(formRaw), string(secretRaw), string(fileRaw), string(rulesRaw), nil
}

func scanChannelTemplate(row interface{ Scan(...any) error }, kind domainchannel.ChannelTemplateKind) (domainchannel.ChannelTemplate, error) {
	var (
		tpl                                  domainchannel.ChannelTemplate
		formRaw, secretRaw, fileRaw, ruleRaw []byte
	)
	if err := row.Scan(
		&tpl.ID, &tpl.ChannelIDRef, &tpl.ChannelID, &tpl.TemplateVersion,
		&formRaw, &secretRaw, &fileRaw, &ruleRaw,
		&tpl.Enabled, &tpl.CreatedAt, &tpl.UpdatedAt,
	); err != nil {
		return domainchannel.ChannelTemplate{}, err
	}
	tpl.Kind = kind
	tpl.FormSchema = []domainchannel.ChannelLoginFormField{}
	tpl.SecretFields = []string{}
	tpl.FileFields = []domainchannel.ChannelLoginFileField{}
	tpl.ValidationRules = map[string]domainchannel.ChannelLoginValidationRule{}
	if len(formRaw) > 0 {
		if err := json.Unmarshal(formRaw, &tpl.FormSchema); err != nil {
			return domainchannel.ChannelTemplate{}, fmt.Errorf("decode form_schema_json: %w", err)
		}
	}
	if len(secretRaw) > 0 {
		if err := json.Unmarshal(secretRaw, &tpl.SecretFields); err != nil {
			return domainchannel.ChannelTemplate{}, fmt.Errorf("decode secret_fields_json: %w", err)
		}
	}
	if len(fileRaw) > 0 {
		if err := json.Unmarshal(fileRaw, &tpl.FileFields); err != nil {
			return domainchannel.ChannelTemplate{}, fmt.Errorf("decode file_fields_json: %w", err)
		}
	}
	if len(ruleRaw) > 0 {
		if err := json.Unmarshal(ruleRaw, &tpl.ValidationRules); err != nil {
			return domainchannel.ChannelTemplate{}, fmt.Errorf("decode validation_rules_json: %w", err)
		}
	}
	return tpl, nil
}

// 接口符合性编译期断言。
var (
	_ platformchannelapp.ChannelAdminRepository         = (*PlatformChannelRepo)(nil)
	_ platformchannelapp.ChannelTemplateAdminRepository = (*ChannelTemplateAdminRepo)(nil)
)
