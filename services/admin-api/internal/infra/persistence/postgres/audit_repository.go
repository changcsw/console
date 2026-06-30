package postgres

import (
	"context"
	"encoding/json"
	"strconv"
	"strings"

	auditapp "github.com/csw/console/services/admin-api/internal/app/audit"
	"github.com/csw/console/services/admin-api/internal/domain/common"
)

type AuditRepository struct {
	db DBTX
}

func NewAuditRepository(db DBTX) *AuditRepository { return &AuditRepository{db: db} }

func (r *AuditRepository) Insert(ctx context.Context, row common.AuditLog) error {
	detailJSON := []byte("{}")
	if !isZeroAuditDetail(row.Detail) {
		encoded, err := json.Marshal(row.Detail)
		if err != nil {
			return err
		}
		detailJSON = encoded
	}
	_, err := r.db.Exec(ctx, `
INSERT INTO audit_logs (actor_id, action, resource_type, resource_id, env, detail_json)
VALUES ($1, $2, $3, $4, $5, $6::jsonb)
`, row.ActorID, row.Action, row.ResourceType, row.ResourceID, row.Env, string(detailJSON))
	return err
}

func isZeroAuditDetail(detail common.AuditDetail) bool {
	return detail.Summary == "" &&
		len(detail.Before) == 0 &&
		len(detail.After) == 0 &&
		len(detail.Changed) == 0 &&
		len(detail.Extra) == 0 &&
		detail.Request == nil
}

func (r *AuditRepository) Query(ctx context.Context, q auditapp.AuditQuery) ([]auditapp.AuditLogItem, int64, error) {
	whereSQL, args := buildAuditWhere(q)

	countSQL := `
SELECT COUNT(1)
FROM audit_logs al
LEFT JOIN admin_users au ON al.actor_id = au.id
` + whereSQL
	var total int64
	if err := r.db.QueryRow(ctx, countSQL, args...).Scan(&total); err != nil {
		return nil, 0, err
	}

	orderBy := "al.created_at DESC, al.id DESC"
	if !q.SortDesc {
		orderBy = "al.created_at ASC, al.id ASC"
	}
	queryArgs := make([]any, 0, len(args)+2)
	queryArgs = append(queryArgs, args...)
	queryArgs = append(queryArgs, q.PageSize, (q.Page-1)*q.PageSize)

	rows, err := r.db.Query(ctx, `
SELECT
  al.id, al.actor_id, al.action, al.resource_type, al.resource_id, al.env, al.detail_json, al.created_at,
  au.id, au.user_name, au.display_name
FROM audit_logs al
LEFT JOIN admin_users au ON al.actor_id = au.id
`+whereSQL+`
ORDER BY `+orderBy+`
LIMIT $`+strconv.Itoa(len(args)+1)+` OFFSET $`+strconv.Itoa(len(args)+2), queryArgs...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	items := make([]auditapp.AuditLogItem, 0)
	for rows.Next() {
		var (
			item                                  auditapp.AuditLogItem
			detailRaw                             []byte
			envStr                                string
			operatorID                            *int64
			operatorUserName, operatorDisplayName *string
		)
		if err := rows.Scan(
			&item.ID, &item.ActorID, &item.Action, &item.ResourceType, &item.ResourceID, &envStr, &detailRaw, &item.CreatedAt,
			&operatorID, &operatorUserName, &operatorDisplayName,
		); err != nil {
			return nil, 0, err
		}
		item.Env = common.Environment(envStr)
		if len(detailRaw) > 0 {
			if err := json.Unmarshal(detailRaw, &item.Detail); err != nil {
				return nil, 0, err
			}
		}
		if operatorID != nil {
			item.Operator = &auditapp.Operator{
				ID:          *operatorID,
				UserName:    derefString(operatorUserName),
				DisplayName: derefString(operatorDisplayName),
			}
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}
	return items, total, nil
}

func buildAuditWhere(q auditapp.AuditQuery) (string, []any) {
	clauses := make([]string, 0, 10)
	args := make([]any, 0, 10)
	add := func(clause string, value any) {
		args = append(args, value)
		clauses = append(clauses, clause+" $"+strconv.Itoa(len(args)))
	}
	addDual := func(clause string, left, right any) {
		args = append(args, left)
		p1 := "$" + strconv.Itoa(len(args))
		args = append(args, right)
		p2 := "$" + strconv.Itoa(len(args))
		clauses = append(clauses, strings.ReplaceAll(strings.ReplaceAll(clause, "{1}", p1), "{2}", p2))
	}

	if q.ID != nil {
		add("al.id =", *q.ID)
	}
	if q.Env != nil && *q.Env != "" {
		add("al.env =", *q.Env)
	}
	if q.Action != nil && strings.TrimSpace(*q.Action) != "" {
		add("al.action =", strings.TrimSpace(*q.Action))
	}
	if q.ResourceType != nil && strings.TrimSpace(*q.ResourceType) != "" {
		add("al.resource_type =", strings.TrimSpace(*q.ResourceType))
	}
	if q.ResourceID != nil && strings.TrimSpace(*q.ResourceID) != "" {
		add("al.resource_id =", strings.TrimSpace(*q.ResourceID))
	}
	if q.Operator != nil {
		add("al.actor_id =", *q.Operator)
	} else if q.OperatorKeyword != nil && strings.TrimSpace(*q.OperatorKeyword) != "" {
		kw := "%" + strings.TrimSpace(*q.OperatorKeyword) + "%"
		addDual("(au.user_name ILIKE {1} OR au.display_name ILIKE {2})", kw, kw)
	}
	if q.From != nil {
		add("al.created_at >=", q.From.UTC())
	}
	if q.To != nil {
		add("al.created_at <=", q.To.UTC())
	}
	if q.Keyword != nil && strings.TrimSpace(*q.Keyword) != "" {
		kw := "%" + strings.TrimSpace(*q.Keyword) + "%"
		addDual("(al.resource_id ILIKE {1} OR COALESCE(al.detail_json->>'summary','') ILIKE {2})", kw, kw)
	}

	if len(clauses) == 0 {
		return "", args
	}
	return " WHERE " + strings.Join(clauses, " AND "), args
}

func derefString(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

var _ auditapp.Repository = (*AuditRepository)(nil)
