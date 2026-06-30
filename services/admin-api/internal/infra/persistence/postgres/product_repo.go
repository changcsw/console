package postgres

import (
	"context"
	"fmt"
	"strings"

	productapp "github.com/csw/console/services/admin-api/internal/app/product"
	domainproduct "github.com/csw/console/services/admin-api/internal/domain/product"
)

type ProductRepo struct{ db DBTX }

const productSelect = `
SELECT p.id, p.game_id_ref, g.game_id, p.product_id, p.product_name, p.base_amount_minor, p.base_currency,
       p.price_id, p.enabled, p.created_at, p.updated_at
FROM products p
JOIN games g ON g.id = p.game_id_ref`

func (r *ProductRepo) ListByGame(ctx context.Context, gameID string, keyword string, enabled *bool, page, pageSize int, sort string) ([]domainproduct.Product, int, error) {
	where := []string{"g.game_id = $1"}
	args := []any{gameID}
	idx := 2
	if kw := strings.TrimSpace(keyword); kw != "" {
		where = append(where, fmt.Sprintf("(p.product_id ILIKE $%d OR p.product_name ILIKE $%d)", idx, idx))
		args = append(args, "%"+kw+"%")
		idx++
	}
	if enabled != nil {
		where = append(where, fmt.Sprintf("p.enabled = $%d", idx))
		args = append(args, *enabled)
		idx++
	}
	cond := strings.Join(where, " AND ")
	var total int
	if err := r.db.QueryRow(ctx, "SELECT COUNT(*) FROM products p JOIN games g ON g.id=p.game_id_ref WHERE "+cond, args...).Scan(&total); err != nil {
		return nil, 0, mapErr(err)
	}
	order := orderBy(sort, map[string]string{"updatedAt": "p.updated_at", "createdAt": "p.created_at", "productId": "p.product_id"}, "p.updated_at DESC")
	args = append(args, pageSize, (page-1)*pageSize)
	rows, err := r.db.Query(ctx, fmt.Sprintf("%s WHERE %s ORDER BY %s LIMIT $%d OFFSET $%d", productSelect, cond, order, idx, idx+1), args...)
	if err != nil {
		return nil, 0, mapErr(err)
	}
	defer rows.Close()
	items := []domainproduct.Product{}
	for rows.Next() {
		item, err := scanProduct(rows)
		if err != nil {
			return nil, 0, mapErr(err)
		}
		items = append(items, item)
	}
	return items, total, mapErr(rows.Err())
}

func (r *ProductRepo) Create(ctx context.Context, item domainproduct.Product) (domainproduct.Product, error) {
	err := r.db.QueryRow(ctx, `
INSERT INTO products (game_id_ref, product_id, product_name, base_amount_minor, base_currency, price_id, enabled)
VALUES ((SELECT id FROM games WHERE game_id=$1), $2,$3,$4,$5,$6,$7)
RETURNING id, game_id_ref, product_id, product_name, base_amount_minor, base_currency, price_id, enabled, created_at, updated_at`,
		item.GameID, item.ProductID, item.ProductName, item.BaseAmountMinor, item.BaseCurrency, item.PriceID, item.Enabled,
	).Scan(&item.ID, &item.GameIDRef, &item.ProductID, &item.ProductName, &item.BaseAmountMinor, &item.BaseCurrency, &item.PriceID, &item.Enabled, &item.CreatedAt, &item.UpdatedAt)
	if err != nil {
		return domainproduct.Product{}, mapErr(err)
	}
	item.GameID = strings.TrimSpace(item.GameID)
	return item, nil
}

func (r *ProductRepo) GetByGameAndProductID(ctx context.Context, gameID, productID string) (domainproduct.Product, error) {
	row := r.db.QueryRow(ctx, productSelect+" WHERE g.game_id=$1 AND p.product_id=$2", gameID, productID)
	item, err := scanProduct(row)
	if err != nil {
		return domainproduct.Product{}, mapErr(err)
	}
	return item, nil
}

func (r *ProductRepo) Update(ctx context.Context, gameID, productID string, patch productapp.ProductPatch) (domainproduct.Product, error) {
	sets := []string{}
	args := []any{}
	idx := 1
	if patch.ProductName != nil {
		sets = append(sets, fmt.Sprintf("product_name=$%d", idx))
		args = append(args, *patch.ProductName)
		idx++
	}
	if patch.BaseAmountMinor != nil {
		sets = append(sets, fmt.Sprintf("base_amount_minor=$%d", idx))
		args = append(args, *patch.BaseAmountMinor)
		idx++
	}
	if patch.BaseCurrency != nil {
		sets = append(sets, fmt.Sprintf("base_currency=$%d", idx))
		args = append(args, *patch.BaseCurrency)
		idx++
	}
	if patch.PriceID != nil {
		sets = append(sets, fmt.Sprintf("price_id=$%d", idx))
		args = append(args, *patch.PriceID)
		idx++
	}
	if patch.Enabled != nil {
		sets = append(sets, fmt.Sprintf("enabled=$%d", idx))
		args = append(args, *patch.Enabled)
		idx++
	}
	sets = append(sets, "updated_at=NOW()")
	args = append(args, gameID, productID)
	tag, err := r.db.Exec(ctx,
		fmt.Sprintf("UPDATE products SET %s WHERE game_id_ref=(SELECT id FROM games WHERE game_id=$%d) AND product_id=$%d", strings.Join(sets, ", "), idx, idx+1),
		args...)
	if err != nil {
		return domainproduct.Product{}, mapErr(err)
	}
	if tag.RowsAffected() == 0 {
		return domainproduct.Product{}, mapErr(errNoRows())
	}
	return r.GetByGameAndProductID(ctx, gameID, productID)
}

func (r *ProductRepo) ListByIDs(ctx context.Context, gameID string, productIDs []string) ([]domainproduct.Product, error) {
	if len(productIDs) == 0 {
		return []domainproduct.Product{}, nil
	}
	args := []any{gameID}
	holders := make([]string, 0, len(productIDs))
	for i, id := range productIDs {
		args = append(args, id)
		holders = append(holders, fmt.Sprintf("$%d", i+2))
	}
	query := fmt.Sprintf("%s WHERE g.game_id=$1 AND p.product_id IN (%s)", productSelect, strings.Join(holders, ","))
	rows, err := r.db.Query(ctx, query, args...)
	if err != nil {
		return nil, mapErr(err)
	}
	defer rows.Close()
	items := make([]domainproduct.Product, 0, len(productIDs))
	for rows.Next() {
		item, err := scanProduct(rows)
		if err != nil {
			return nil, mapErr(err)
		}
		items = append(items, item)
	}
	return items, mapErr(rows.Err())
}

func scanProduct(row interface{ Scan(...any) error }) (domainproduct.Product, error) {
	var item domainproduct.Product
	if err := row.Scan(&item.ID, &item.GameIDRef, &item.GameID, &item.ProductID, &item.ProductName, &item.BaseAmountMinor, &item.BaseCurrency, &item.PriceID, &item.Enabled, &item.CreatedAt, &item.UpdatedAt); err != nil {
		return domainproduct.Product{}, err
	}
	return item, nil
}
