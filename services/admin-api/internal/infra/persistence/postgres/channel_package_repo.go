package postgres

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/csw/console/services/admin-api/internal/app/channel"
	domainchannel "github.com/csw/console/services/admin-api/internal/domain/channel"
)

// ChannelPackageRepo 渠道包仓储（channel_packages，业务表，SQL 不带 schema 前缀，靠 search_path）。
type ChannelPackageRepo struct{ db DBTX }

const channelPackageSelect = `
SELECT id, game_channel_id_ref, package_code, package_name, market_code, bundle_id,
       inherit_channel_config, enabled, override_json, created_at, updated_at
FROM channel_packages`

// ListByGameChannel 列出某渠道实例下的渠道包（按 package_code 升序）。
func (r *ChannelPackageRepo) ListByGameChannel(ctx context.Context, gameChannelID int64) ([]domainchannel.ChannelPackage, error) {
	rows, err := r.db.Query(ctx, channelPackageSelect+` WHERE game_channel_id_ref = $1 ORDER BY package_code ASC`, gameChannelID)
	if err != nil {
		return nil, mapErr(err)
	}
	defer rows.Close()
	out := []domainchannel.ChannelPackage{}
	for rows.Next() {
		pkg, err := scanPackage(rows)
		if err != nil {
			return nil, mapErr(err)
		}
		out = append(out, pkg)
	}
	return out, mapErr(rows.Err())
}

// ExistsPackageCode 判断同实例下 package_code 是否已存在。
func (r *ChannelPackageRepo) ExistsPackageCode(ctx context.Context, gameChannelID int64, code string) (bool, error) {
	var exists bool
	err := r.db.QueryRow(ctx,
		`SELECT EXISTS(SELECT 1 FROM channel_packages WHERE game_channel_id_ref = $1 AND package_code = $2)`,
		gameChannelID, code,
	).Scan(&exists)
	return exists, mapErr(err)
}

// InsertPackage 落库渠道包，返回装配后的聚合（含 id/时间戳）。
func (r *ChannelPackageRepo) InsertPackage(ctx context.Context, pkg domainchannel.ChannelPackage) (domainchannel.ChannelPackage, error) {
	override, err := marshalOverride(pkg.OverrideJSON)
	if err != nil {
		return domainchannel.ChannelPackage{}, err
	}
	err = r.db.QueryRow(ctx,
		`INSERT INTO channel_packages
		   (game_channel_id_ref, package_code, package_name, market_code, bundle_id,
		    inherit_channel_config, enabled, override_json)
		 VALUES ($1,$2,$3,$4,$5,$6,$7,$8)
		 RETURNING id, created_at, updated_at`,
		pkg.GameChannelIDRef, pkg.PackageCode, pkg.PackageName, pkg.MarketCode, pkg.BundleID,
		pkg.InheritChannelConfig, pkg.Enabled, override,
	).Scan(&pkg.ID, &pkg.CreatedAt, &pkg.UpdatedAt)
	if err != nil {
		return domainchannel.ChannelPackage{}, mapErr(err)
	}
	return pkg, nil
}

// GetPackageByID 按 channel_packages.id 取渠道包。
func (r *ChannelPackageRepo) GetPackageByID(ctx context.Context, id int64) (domainchannel.ChannelPackage, error) {
	row := r.db.QueryRow(ctx, channelPackageSelect+` WHERE id = $1`, id)
	pkg, err := scanPackage(row)
	if err != nil {
		return domainchannel.ChannelPackage{}, mapErr(err)
	}
	return pkg, nil
}

// UpdatePackage 更新渠道包列级字段（nil 不改）。
func (r *ChannelPackageRepo) UpdatePackage(ctx context.Context, id int64, patch channel.PackagePatch) error {
	sets := []string{}
	args := []any{}
	idx := 1
	if patch.PackageName != nil {
		sets = append(sets, fmt.Sprintf("package_name = $%d", idx))
		args = append(args, *patch.PackageName)
		idx++
	}
	if patch.BundleID != nil {
		sets = append(sets, fmt.Sprintf("bundle_id = $%d", idx))
		args = append(args, *patch.BundleID)
		idx++
	}
	if patch.InheritChannelConfig != nil {
		sets = append(sets, fmt.Sprintf("inherit_channel_config = $%d", idx))
		args = append(args, *patch.InheritChannelConfig)
		idx++
	}
	if patch.Enabled != nil {
		sets = append(sets, fmt.Sprintf("enabled = $%d", idx))
		args = append(args, *patch.Enabled)
		idx++
	}
	if patch.OverrideJSON != nil {
		override, err := marshalOverride(patch.OverrideJSON)
		if err != nil {
			return err
		}
		sets = append(sets, fmt.Sprintf("override_json = $%d", idx))
		args = append(args, override)
		idx++
	}
	if len(sets) == 0 {
		return nil
	}
	sets = append(sets, "updated_at = NOW()")
	args = append(args, id)
	tag, err := r.db.Exec(ctx,
		fmt.Sprintf("UPDATE channel_packages SET %s WHERE id = $%d", strings.Join(sets, ", "), idx), args...)
	if err != nil {
		return mapErr(err)
	}
	if tag.RowsAffected() == 0 {
		return mapErr(errNoRows())
	}
	return nil
}

func scanPackage(row interface{ Scan(...any) error }) (domainchannel.ChannelPackage, error) {
	var (
		pkg      domainchannel.ChannelPackage
		override []byte
	)
	if err := row.Scan(
		&pkg.ID, &pkg.GameChannelIDRef, &pkg.PackageCode, &pkg.PackageName, &pkg.MarketCode, &pkg.BundleID,
		&pkg.InheritChannelConfig, &pkg.Enabled, &override, &pkg.CreatedAt, &pkg.UpdatedAt,
	); err != nil {
		return domainchannel.ChannelPackage{}, err
	}
	pkg.OverrideJSON = map[string]any{}
	if len(override) > 0 {
		if err := json.Unmarshal(override, &pkg.OverrideJSON); err != nil {
			return domainchannel.ChannelPackage{}, err
		}
	}
	return pkg, nil
}

// marshalOverride 把 override 载荷序列化为 jsonb 文本（nil/空 → {}）。
func marshalOverride(m map[string]any) (string, error) {
	if len(m) == 0 {
		return "{}", nil
	}
	b, err := json.Marshal(m)
	if err != nil {
		return "", err
	}
	return string(b), nil
}
