package postgres

import (
	"context"

	"github.com/csw/console/services/admin-api/internal/domain/common"
	domainproduct "github.com/csw/console/services/admin-api/internal/domain/product"
)

type ChannelProductRepo struct{ db DBTX }

func (r *ChannelProductRepo) ListByPackage(ctx context.Context, packageID int64) ([]domainproduct.ChannelProduct, error) {
	rows, err := r.db.Query(ctx, `
SELECT id, product_id_ref, package_id_ref, product_id_mode, product_id_override, price_id_mode, price_id_override,
       enabled, created_at, updated_at
FROM channel_products
WHERE package_id_ref=$1
ORDER BY id`, packageID)
	if err != nil {
		return nil, mapErr(err)
	}
	defer rows.Close()
	out := []domainproduct.ChannelProduct{}
	for rows.Next() {
		var item domainproduct.ChannelProduct
		var productMode, priceMode string
		if err := rows.Scan(&item.ID, &item.ProductIDRef, &item.PackageIDRef, &productMode, &item.ProductIDOverride, &priceMode, &item.PriceIDOverride, &item.Enabled, &item.CreatedAt, &item.UpdatedAt); err != nil {
			return nil, mapErr(err)
		}
		item.ProductIDMode = common.OverrideMode(productMode)
		item.PriceIDMode = common.OverrideMode(priceMode)
		out = append(out, item)
	}
	return out, mapErr(rows.Err())
}

func (r *ChannelProductRepo) ReplaceByPackage(ctx context.Context, packageID int64, items []domainproduct.ChannelProduct) error {
	if _, err := r.db.Exec(ctx, `DELETE FROM channel_products WHERE package_id_ref=$1`, packageID); err != nil {
		return mapErr(err)
	}
	for _, item := range items {
		if _, err := r.db.Exec(ctx, `
INSERT INTO channel_products (
  product_id_ref, package_id_ref, product_id_mode, product_id_override, price_id_mode, price_id_override, enabled
) VALUES ($1,$2,$3,$4,$5,$6,$7)`,
			item.ProductIDRef, packageID, string(item.ProductIDMode), item.ProductIDOverride, string(item.PriceIDMode), item.PriceIDOverride, item.Enabled,
		); err != nil {
			return mapErr(err)
		}
	}
	return nil
}
