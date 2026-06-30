package audit

import (
	"context"
	"errors"
	"slices"
	"strings"
	"time"

	adminapp "github.com/csw/console/services/admin-api/internal/app/admin"
	"github.com/csw/console/services/admin-api/internal/domain/common"
)

var (
	ErrValidation = errors.New("validation failed")
	ErrNotFound   = errors.New("not found")
)

// Operator 审计操作者信息（join admin_users）。
type Operator struct {
	ID          int64
	UserName    string
	DisplayName string
}

// AuditLogItem 审计列表项（兼容前端分页/详情）。
type AuditLogItem struct {
	common.AuditLog
	Operator *Operator
}

// AuditWriteInput 显式写审计输入。
type AuditWriteInput struct {
	ActorID      int64
	Action       string
	ResourceType string
	ResourceID   string
	Env          common.Environment
	Detail       common.AuditDetail
}

// SecretAwareAuditInput 在写入前声明需递归脱敏的字段名。
type SecretAwareAuditInput struct {
	AuditWriteInput
	SecretKeys []string
}

// AuditQuery 审计查询条件。
type AuditQuery struct {
	ID              *int64
	Env             *common.Environment
	Action          *string
	ResourceType    *string
	ResourceID      *string
	Operator        *int64
	OperatorKeyword *string
	From            *time.Time
	To              *time.Time
	Keyword         *string
	Page            int
	PageSize        int
	SortDesc        bool
}

// AuditPage 分页结果。
type AuditPage struct {
	Items    []AuditLogItem
	Page     int
	PageSize int
	Total    int64
}

// Repository 为审计仓储端口。
type Repository interface {
	Insert(ctx context.Context, row common.AuditLog) error
	Query(ctx context.Context, q AuditQuery) ([]AuditLogItem, int64, error)
}

// Service 是审计应用服务。
type Service interface {
	Write(ctx context.Context, in SecretAwareAuditInput) error
	Query(ctx context.Context, q AuditQuery) (AuditPage, error)
}

type service struct {
	repo       Repository
	runtimeEnv common.Environment
}

func NewService(repo Repository, runtimeEnv common.Environment) Service {
	return &service{repo: repo, runtimeEnv: runtimeEnv}
}

func (s *service) Write(ctx context.Context, in SecretAwareAuditInput) error {
	action := strings.TrimSpace(in.Action)
	resourceType := strings.TrimSpace(in.ResourceType)
	resourceID := strings.TrimSpace(in.ResourceID)
	if action == "" || resourceType == "" || resourceID == "" {
		return ErrValidation
	}

	meta := fromContext(ctx)
	actorID := in.ActorID
	if actorID <= 0 {
		if meta.ActorID > 0 {
			actorID = meta.ActorID
		} else if ac, ok := adminapp.AuthContextFrom(ctx); ok {
			actorID = ac.UserID
		}
	}
	if actorID < 0 {
		actorID = 0
	}

	env := in.Env
	if env == "" {
		env = meta.Env
	}
	if env == "" {
		env = s.runtimeEnv
	}

	detail := sanitizeDetail(in.Detail, in.SecretKeys)
	if detail.Request == nil && meta.Request != nil {
		copyReq := *meta.Request
		detail.Request = &copyReq
	}
	if detail.Summary == "" {
		detail.Summary = defaultSummary(action, resourceID, detail.Changed)
	}
	detail.Changed = normalizeChanged(detail.Changed)

	row := common.AuditLog{
		ActorID:      actorID,
		Action:       action,
		ResourceType: resourceType,
		ResourceID:   resourceID,
		Env:          env,
		Detail:       detail,
	}
	if err := s.repo.Insert(ctx, row); err != nil {
		return err
	}
	markWritten(ctx)
	return nil
}

func (s *service) Query(ctx context.Context, q AuditQuery) (AuditPage, error) {
	if q.From != nil && q.To != nil && q.From.After(*q.To) {
		return AuditPage{}, ErrValidation
	}
	q.Page, q.PageSize = normalizePage(q.Page, q.PageSize)
	if !q.SortDesc {
		// false 表示正序，true 为默认倒序。保持显式值，不回退。
	}
	items, total, err := s.repo.Query(ctx, q)
	if err != nil {
		return AuditPage{}, err
	}
	if q.ID != nil && len(items) == 0 {
		return AuditPage{}, ErrNotFound
	}
	return AuditPage{
		Items:    items,
		Page:     q.Page,
		PageSize: q.PageSize,
		Total:    total,
	}, nil
}

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

func defaultSummary(action, resourceID string, changed []string) string {
	if len(changed) > 0 {
		return action + " " + resourceID + " fields: " + strings.Join(changed, ",")
	}
	return action + " " + resourceID
}

func normalizeChanged(changed []string) []string {
	if len(changed) == 0 {
		return nil
	}
	uniq := map[string]struct{}{}
	out := make([]string, 0, len(changed))
	for _, item := range changed {
		item = strings.TrimSpace(item)
		if item == "" {
			continue
		}
		if _, ok := uniq[item]; ok {
			continue
		}
		uniq[item] = struct{}{}
		out = append(out, item)
	}
	if len(out) == 0 {
		return nil
	}
	slices.Sort(out)
	return out
}

func sanitizeDetail(detail common.AuditDetail, secretKeys []string) common.AuditDetail {
	secretSet := map[string]struct{}{}
	for _, key := range secretKeys {
		key = strings.TrimSpace(strings.ToLower(key))
		if key != "" {
			secretSet[key] = struct{}{}
		}
	}
	detail.Before = sanitizeMap(detail.Before, secretSet)
	detail.After = sanitizeMap(detail.After, secretSet)
	detail.Extra = sanitizeMap(detail.Extra, secretSet)
	return detail
}

func sanitizeMap(in map[string]any, secretSet map[string]struct{}) map[string]any {
	if len(in) == 0 {
		return nil
	}
	out := make(map[string]any, len(in))
	for k, v := range in {
		if _, ok := secretSet[strings.ToLower(k)]; ok {
			out[k] = "masked"
			continue
		}
		out[k] = sanitizeValue(v, secretSet)
	}
	return out
}

func sanitizeValue(v any, secretSet map[string]struct{}) any {
	switch val := v.(type) {
	case map[string]any:
		return sanitizeMap(val, secretSet)
	case []any:
		out := make([]any, 0, len(val))
		for _, item := range val {
			out = append(out, sanitizeValue(item, secretSet))
		}
		return out
	default:
		return v
	}
}
