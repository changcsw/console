package postgres

import (
	"context"

	"github.com/csw/console/services/admin-api/internal/domain/common"
)

type ProductChannelPackageRepo struct{ db DBTX }

func (r *ProductChannelPackageRepo) GetPackageGameAndChannel(ctx context.Context, packageID int64) (gameID string, packageCode string, channelID string, gameChannelID int64, err error) {
	err = r.db.QueryRow(ctx, `
SELECT g.game_id, cp.package_code, ch.channel_id, gc.id
FROM channel_packages cp
JOIN game_channels gc ON gc.id = cp.game_channel_id_ref
JOIN games g ON g.id = gc.game_id_ref
JOIN platform.channels ch ON ch.id = gc.channel_id_ref
WHERE cp.id=$1`, packageID).Scan(&gameID, &packageCode, &channelID, &gameChannelID)
	if err != nil {
		return "", "", "", 0, mapErr(err)
	}
	return gameID, packageCode, channelID, gameChannelID, nil
}

func (r *ProductChannelPackageRepo) BelongsToGame(ctx context.Context, packageID int64, gameID string) (bool, error) {
	var exists bool
	err := r.db.QueryRow(ctx, `
SELECT EXISTS(
  SELECT 1
  FROM channel_packages cp
  JOIN game_channels gc ON gc.id = cp.game_channel_id_ref
  JOIN games g ON g.id = gc.game_id_ref
  WHERE cp.id=$1 AND g.game_id=$2
)`, packageID, gameID).Scan(&exists)
	return exists, mapErr(err)
}

type CurrencySpecRepo struct{ db DBTX }

// NewCurrencySpecRepo 绑定连接池/事务，供平台级币种字典只读用例使用。
func NewCurrencySpecRepo(db DBTX) *CurrencySpecRepo { return &CurrencySpecRepo{db: db} }

// ListEnabled 列出已启用的 currency_specs（平台级，只读），按 currency_code 升序稳定输出。
func (r *CurrencySpecRepo) ListEnabled(ctx context.Context) ([]common.CurrencySpec, error) {
	rows, err := r.db.Query(ctx, `
SELECT currency_code, currency_name, decimal_places, min_amount_minor, rounding_mode, enabled
FROM platform.currency_specs
WHERE enabled = TRUE
ORDER BY currency_code`)
	if err != nil {
		return nil, mapErr(err)
	}
	defer rows.Close()
	specs := make([]common.CurrencySpec, 0)
	for rows.Next() {
		var spec common.CurrencySpec
		if err := rows.Scan(&spec.CurrencyCode, &spec.CurrencyName, &spec.DecimalPlaces, &spec.MinAmountMinor, &spec.RoundingMode, &spec.Enabled); err != nil {
			return nil, mapErr(err)
		}
		specs = append(specs, spec)
	}
	if err := rows.Err(); err != nil {
		return nil, mapErr(err)
	}
	return specs, nil
}

func (r *CurrencySpecRepo) GetByCode(ctx context.Context, currencyCode string) (common.CurrencySpec, error) {
	var spec common.CurrencySpec
	spec.CurrencyCode = currencyCode
	err := r.db.QueryRow(ctx, `
SELECT currency_code, decimal_places, min_amount_minor, rounding_mode, enabled
FROM platform.currency_specs
WHERE currency_code=$1`, currencyCode).
		Scan(&spec.CurrencyCode, &spec.DecimalPlaces, &spec.MinAmountMinor, &spec.RoundingMode, &spec.Enabled)
	if err != nil {
		return common.CurrencySpec{}, mapErr(err)
	}
	return spec, nil
}
