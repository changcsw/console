// Package platformchannel 是「平台渠道主数据 + 渠道模版」的应用层（system 侧基础数据维护，00 §4.4）。
// 这一层的对象与游戏无关：系统管理员在此新增渠道、维护渠道策略与模版四件套；
// 游戏侧只在渠道实例上引用模版填参（见 app/channel、app/channellogin、app/product）。
package platformchannel

import (
	"context"
	"net/http"
	"time"

	adminapp "github.com/csw/console/services/admin-api/internal/app/admin"
	"github.com/csw/console/services/admin-api/internal/app/dto"
	domainchannel "github.com/csw/console/services/admin-api/internal/domain/channel"
)

// Error 携带统一错误码/HTTP 状态/消息/明细的应用层错误（00 §7.4）。
type Error struct {
	Status  int
	Code    string
	Message string
	Details []any
}

func (e *Error) Error() string { return e.Message }

const (
	codeValidation = "VALIDATION_FAILED"
	codeConflict   = "CONFLICT"
	codeNotFound   = "NOT_FOUND"
)

func validationErr(msg string, details ...any) *Error {
	if details == nil {
		details = []any{}
	}
	return &Error{Status: http.StatusBadRequest, Code: codeValidation, Message: msg, Details: details}
}

func conflictErr(msg string) *Error {
	return &Error{Status: http.StatusConflict, Code: codeConflict, Message: msg, Details: []any{}}
}

func notFoundErr(msg string) *Error {
	return &Error{Status: http.StatusNotFound, Code: codeNotFound, Message: msg, Details: []any{}}
}

// issueDetails 把领域校验明细转为统一 details（[{field, rule, message}]）。
func issueDetails(issues []domainchannel.ValidationIssue) []any {
	out := make([]any, 0, len(issues))
	for _, item := range issues {
		out = append(out, map[string]string{"field": item.Field, "rule": item.Rule, "message": item.Message})
	}
	return out
}

// PlatformChannelRow 平台渠道行：主数据 + 策略 + 各类模版版本数 + 最近更新时间。
type PlatformChannelRow struct {
	Channel            domainchannel.Channel
	Policy             domainchannel.ChannelPolicy
	LoginTemplateCount int
	IAPTemplateCount   int
	UpdatedAt          time.Time
}

// ChannelMasterPatch 渠道主数据列级补丁（nil 不改）。
// channel_id / channel_type / region 不在补丁内：前两者是身份，region 决定 market 兼容性口径，
// 改动会让已存在的渠道实例集体失配，故创建后不可改。
type ChannelMasterPatch struct {
	ChannelName *string
	Enabled     *bool
	Sort        *int
}

// ChannelPolicyPatch 渠道策略列级补丁（nil 不改）。
type ChannelPolicyPatch struct {
	LoginMode     *string
	PaymentMode   *string
	LoginLocked   *bool
	PaymentLocked *bool
}

// ChannelAdminRepository 平台渠道主数据/策略读写仓储（platform.channels / platform.channel_policies）。
type ChannelAdminRepository interface {
	// List 分页列出渠道（含策略与模版计数），按 sort 升序。
	List(ctx context.Context, q dto.ListPlatformChannelsQuery) ([]PlatformChannelRow, int, error)
	// GetByChannelID 按业务键取单行（不存在返回 adminapp.ErrNotFound）。
	GetByChannelID(ctx context.Context, channelID string) (PlatformChannelRow, error)
	// Insert 落库渠道主数据 + 策略（channel_id 冲突返回 adminapp.ErrConflict）。
	Insert(ctx context.Context, ch domainchannel.Channel, policy domainchannel.ChannelPolicy) error
	// UpdateMaster 更新主数据可变列。
	UpdateMaster(ctx context.Context, channelID string, patch ChannelMasterPatch) error
	// UpsertPolicy 更新策略（无行时插入，保证渠道与策略一对一）。
	UpsertPolicy(ctx context.Context, channelID string, patch ChannelPolicyPatch) error
}

// ChannelTemplateAdminRepository 渠道模版读写仓储。kind 决定落库表：
// login → platform.channel_login_templates；iap → platform.channel_iap_templates（两表同构）。
type ChannelTemplateAdminRepository interface {
	// ListByChannel 列出该渠道某类模版的全部版本（按 template_version 降序）。
	ListByChannel(ctx context.Context, kind domainchannel.ChannelTemplateKind, channelIDRef int64) ([]domainchannel.ChannelTemplate, error)
	// GetByID 取单个模版版本（不存在返回 adminapp.ErrNotFound）。
	GetByID(ctx context.Context, kind domainchannel.ChannelTemplateKind, id int64) (domainchannel.ChannelTemplate, error)
	// Insert 落库新版本（(channel_id_ref, template_version) 冲突返回 adminapp.ErrConflict）。
	Insert(ctx context.Context, tpl domainchannel.ChannelTemplate) (domainchannel.ChannelTemplate, error)
	// Replace 整体覆盖四件套与 enabled（tpl 为服务层合并校验后的完整状态）。
	Replace(ctx context.Context, tpl domainchannel.ChannelTemplate) error
}

// Repositories 一组仓储句柄（绑定到 pool 或某事务连接）。
type Repositories struct {
	Channels  ChannelAdminRepository
	Templates ChannelTemplateAdminRepository
}

// TxManager 提供事务边界（渠道 + 策略需同事务落库）。
type TxManager interface {
	Repositories() Repositories
	InTx(ctx context.Context, fn func(Repositories) error) error
}

// AuditSink / AuditEntry 复用 auth 应用层端口，保持审计写入一致（00 §8）。
type (
	AuditSink  = adminapp.AuditSink
	AuditEntry = adminapp.AuditEntry
)
