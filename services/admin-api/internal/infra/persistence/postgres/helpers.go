package postgres

import (
	"strings"

	"github.com/jackc/pgx/v5"
)

func errNoRows() error { return pgx.ErrNoRows }

// orderBy 把 API 排序参数（如 -updatedAt）映射为 SQL ORDER BY 子句。
// allowed: API 字段名 -> 列名。非法/缺省返回 fallback。
func orderBy(sort string, allowed map[string]string, fallback string) string {
	sort = strings.TrimSpace(sort)
	if sort == "" {
		return fallback
	}
	dir := "ASC"
	field := sort
	if strings.HasPrefix(sort, "-") {
		dir = "DESC"
		field = sort[1:]
	} else if strings.HasPrefix(sort, "+") {
		field = sort[1:]
	}
	col, ok := allowed[field]
	if !ok {
		return fallback
	}
	return col + " " + dir
}
