package postgres

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/csw/console/services/admin-api/internal/app/command"
	domainsync "github.com/csw/console/services/admin-api/internal/domain/sync"
)

type SyncRepo struct {
	db DBTX
}

func (r *SyncRepo) ResolveGameExists(ctx context.Context, schema, gameID string) (bool, error) {
	var exists bool
	query := fmt.Sprintf("SELECT EXISTS(SELECT 1 FROM %s.games WHERE game_id=$1)", safeSchema(schema))
	if err := r.db.QueryRow(ctx, query, strings.TrimSpace(gameID)).Scan(&exists); err != nil {
		return false, mapErr(err)
	}
	return exists, nil
}

func (r *SyncRepo) CreateJob(ctx context.Context, in command.CreateSyncJobInput) (int64, error) {
	var id int64
	err := r.db.QueryRow(ctx, `
INSERT INTO platform.sync_jobs
  (game_id_ref, source_env, target_env, source_hash, target_hash_before, include_deletes, operator_id, operator_note, status)
VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)
RETURNING id`,
		in.GameID, in.SourceEnv, in.TargetEnv, in.SourceHash, in.TargetHashBefore, in.IncludeDeletes, in.OperatorID, in.OperatorNote, string(in.Status),
	).Scan(&id)
	if err != nil {
		return 0, mapErr(err)
	}
	return id, nil
}

func (r *SyncRepo) AddItems(ctx context.Context, jobID int64, items []command.SyncJobItemInput) error {
	for _, item := range items {
		sandboxJSON, _ := json.Marshal(item.SandboxValue)
		prodJSON, _ := json.Marshal(item.ProductionValue)
		_, err := r.db.Exec(ctx, `
INSERT INTO platform.sync_job_items
  (sync_job_id_ref, section, entity_type, entity_key, op, field_name, sandbox_value_json, production_value_json, masked, applied)
VALUES ($1,$2,$3,$4,$5,$6,$7::jsonb,$8::jsonb,$9,$10)`,
			jobID, string(item.Section), item.EntityType, item.EntityKey, string(item.Op), item.FieldName, string(sandboxJSON), string(prodJSON), item.Masked, item.Applied,
		)
		if err != nil {
			return mapErr(err)
		}
	}
	return nil
}

func (r *SyncRepo) ListJobsByGame(ctx context.Context, gameID string, page, pageSize int, status string) ([]domainsync.JobItem, int, error) {
	where := "WHERE game_id_ref=$1"
	args := []any{gameID}
	if status != "" {
		where += " AND status=$2"
		args = append(args, status)
	}
	var total int
	countSQL := "SELECT COUNT(*) FROM platform.sync_jobs " + where
	if err := r.db.QueryRow(ctx, countSQL, args...).Scan(&total); err != nil {
		return nil, 0, mapErr(err)
	}
	args = append(args, pageSize, (page-1)*pageSize)
	listSQL := `
SELECT id, game_id_ref, source_env, target_env, status, include_deletes, operator_id, operator_note, source_hash, target_hash_before, target_hash_after, executed_at, created_at
FROM platform.sync_jobs ` + where + `
ORDER BY created_at DESC, id DESC
LIMIT $` + fmt.Sprintf("%d", len(args)-1) + ` OFFSET $` + fmt.Sprintf("%d", len(args))
	rows, err := r.db.Query(ctx, listSQL, args...)
	if err != nil {
		return nil, 0, mapErr(err)
	}
	defer rows.Close()
	items := make([]domainsync.JobItem, 0)
	for rows.Next() {
		var item domainsync.JobItem
		if err := rows.Scan(
			&item.SyncJobID, &item.GameID, &item.SourceEnv, &item.TargetEnv, &item.Status, &item.IncludeDeletes,
			&item.OperatorID, &item.OperatorNote, &item.SourceHash, &item.TargetHashBefore, &item.TargetHashAfter, &item.ExecutedAt, &item.CreatedAt,
		); err != nil {
			return nil, 0, err
		}
		items = append(items, item)
	}
	return items, total, mapErr(rows.Err())
}

func (r *SyncRepo) UpdateJobResult(ctx context.Context, jobID int64, status domainsync.SyncJobStatus, targetHashAfter, operatorNote string, executedAt *time.Time) error {
	_, err := r.db.Exec(ctx, `
UPDATE platform.sync_jobs
SET status=$2, target_hash_after=$3, operator_note=CASE WHEN $4 <> '' THEN $4 ELSE operator_note END, executed_at=$5, updated_at=NOW()
WHERE id=$1`,
		jobID, string(status), targetHashAfter, operatorNote, executedAt,
	)
	return mapErr(err)
}

func (r *SyncRepo) MarkItemsApplied(ctx context.Context, jobID int64, selected []domainsync.Section, includeDeletes bool) (map[domainsync.Section]domainsync.DiffSummary, []domainsync.ExecuteSkippedDelete, error) {
	sections := make([]string, 0, len(selected))
	for _, sec := range selected {
		sections = append(sections, string(sec))
	}
	if len(sections) == 0 {
		return map[domainsync.Section]domainsync.DiffSummary{}, nil, nil
	}
	_, err := r.db.Exec(ctx, `
UPDATE platform.sync_job_items
SET applied=TRUE, updated_at=NOW()
WHERE sync_job_id_ref=$1
  AND section = ANY($2)
  AND ($3 OR op <> 'delete')`, jobID, sections, includeDeletes)
	if err != nil {
		return nil, nil, mapErr(err)
	}
	rows, err := r.db.Query(ctx, `
SELECT section, op, COUNT(*)
FROM platform.sync_job_items
WHERE sync_job_id_ref=$1 AND applied=TRUE
GROUP BY section, op`, jobID)
	if err != nil {
		return nil, nil, mapErr(err)
	}
	defer rows.Close()
	summary := map[domainsync.Section]domainsync.DiffSummary{}
	for rows.Next() {
		var section string
		var op string
		var c int
		if err := rows.Scan(&section, &op, &c); err != nil {
			return nil, nil, err
		}
		s := summary[domainsync.Section(section)]
		switch op {
		case string(domainsync.OpAdd):
			s.Add = c
		case string(domainsync.OpUpdate):
			s.Update = c
		case string(domainsync.OpDelete):
			s.Delete = c
		}
		summary[domainsync.Section(section)] = s
	}
	if err := mapErr(rows.Err()); err != nil {
		return nil, nil, err
	}
	skipped := []domainsync.ExecuteSkippedDelete{}
	if !includeDeletes {
		srows, qErr := r.db.Query(ctx, `
SELECT section, entity_key
FROM platform.sync_job_items
WHERE sync_job_id_ref=$1 AND op='delete' AND section = ANY($2)`, jobID, sections)
		if qErr != nil {
			return nil, nil, mapErr(qErr)
		}
		defer srows.Close()
		for srows.Next() {
			var section, entityKey string
			if err := srows.Scan(&section, &entityKey); err != nil {
				return nil, nil, err
			}
			skipped = append(skipped, domainsync.ExecuteSkippedDelete{
				Section:   domainsync.Section(section),
				EntityKey: entityKey,
				Reason:    "include_deletes=false",
			})
		}
		if err := mapErr(srows.Err()); err != nil {
			return nil, nil, err
		}
	}
	return summary, skipped, nil
}

func (r *SyncRepo) IsNonceConsumed(ctx context.Context, nonce string) (bool, int64, *time.Time, error) {
	rows, err := r.db.Query(ctx, `SELECT sync_job_id_ref, consumed_at FROM platform.sync_consumed_tokens WHERE nonce=$1`, nonce)
	if err != nil {
		return false, 0, nil, mapErr(err)
	}
	defer rows.Close()
	if !rows.Next() {
		return false, 0, nil, nil
	}
	var jobID int64
	var consumedAt time.Time
	if err := rows.Scan(&jobID, &consumedAt); err != nil {
		return false, 0, nil, err
	}
	return true, jobID, &consumedAt, nil
}

func (r *SyncRepo) ConsumeNonce(ctx context.Context, nonce string, syncJobID int64) error {
	_, err := r.db.Exec(ctx, `
INSERT INTO platform.sync_consumed_tokens (nonce, sync_job_id_ref)
VALUES ($1,$2)`, nonce, syncJobID)
	return mapErr(err)
}

func (r *SyncRepo) LoadSectionEntities(ctx context.Context, sourceSchema, targetSchema, gameID string, section domainsync.Section) ([]domainsync.EntityRecord, []domainsync.EntityRecord, map[string]struct{}, error) {
	src, masked, err := r.loadSection(ctx, sourceSchema, gameID, section)
	if err != nil {
		return nil, nil, nil, err
	}
	dst, _, err := r.loadSection(ctx, targetSchema, gameID, section)
	if err != nil {
		return nil, nil, nil, err
	}
	return src, dst, masked, nil
}

func (r *SyncRepo) ApplySection(ctx context.Context, section domainsync.Section, gameID string, includeDeletes bool) error {
	switch section {
	case domainsync.SectionGame:
		return r.applyGame(ctx, gameID)
	case domainsync.SectionMarkets:
		return r.applyMarkets(ctx, gameID, includeDeletes)
	case domainsync.SectionLegal:
		return r.applyLegal(ctx, gameID, includeDeletes)
	case domainsync.SectionChannels:
		return r.applyChannels(ctx, gameID, includeDeletes)
	case domainsync.SectionPackages:
		return r.applyPackages(ctx, gameID, includeDeletes)
	case domainsync.SectionProducts:
		return r.applyProducts(ctx, gameID, includeDeletes)
	case domainsync.SectionCashier:
		return r.applyCashier(ctx, gameID, includeDeletes)
	case domainsync.SectionPayments:
		return r.applyPayments(ctx, gameID, includeDeletes)
	case domainsync.SectionConfig:
		return r.applyConfig(ctx, gameID, includeDeletes)
	default:
		return fmt.Errorf("unknown section: %s", section)
	}
}

func (r *SyncRepo) loadSection(ctx context.Context, schema, gameID string, section domainsync.Section) ([]domainsync.EntityRecord, map[string]struct{}, error) {
	switch section {
	case domainsync.SectionGame:
		return r.loadGame(ctx, schema, gameID)
	case domainsync.SectionMarkets:
		return r.loadMarkets(ctx, schema, gameID)
	case domainsync.SectionLegal:
		return r.loadLegal(ctx, schema, gameID)
	case domainsync.SectionChannels:
		return r.loadChannels(ctx, schema, gameID)
	case domainsync.SectionPackages:
		return r.loadPackages(ctx, schema, gameID)
	case domainsync.SectionProducts:
		return r.loadProducts(ctx, schema, gameID)
	case domainsync.SectionCashier:
		return r.loadCashier(ctx, schema, gameID)
	case domainsync.SectionPayments:
		return r.loadPayments(ctx, schema, gameID)
	case domainsync.SectionConfig:
		return r.loadConfig(ctx, schema, gameID)
	default:
		return nil, nil, fmt.Errorf("unknown section: %s", section)
	}
}

func (r *SyncRepo) loadGame(ctx context.Context, schema, gameID string) ([]domainsync.EntityRecord, map[string]struct{}, error) {
	rows, err := r.db.Query(ctx, fmt.Sprintf(`
SELECT game_id, game_secret, name, alias, icon_url, default_market_code, status
FROM %s.games
WHERE game_id=$1`, safeSchema(schema)), gameID)
	if err != nil {
		return nil, nil, mapErr(err)
	}
	defer rows.Close()
	out := []domainsync.EntityRecord{}
	for rows.Next() {
		fields := map[string]any{}
		var gameID, gameSecret, name, alias, iconURL, defaultMarket, status string
		if err := rows.Scan(&gameID, &gameSecret, &name, &alias, &iconURL, &defaultMarket, &status); err != nil {
			return nil, nil, err
		}
		fields["game_id"] = gameID
		fields["game_secret"] = gameSecret
		fields["name"] = name
		fields["alias"] = alias
		fields["icon_url"] = iconURL
		fields["default_market_code"] = defaultMarket
		fields["status"] = status
		out = append(out, domainsync.EntityRecord{EntityType: "game", EntityKey: gameID, Fields: fields})
	}
	// game_secret 以密文/原值参与 hash（基线复核），但 preview diff 恒 masked。
	return out, map[string]struct{}{"game_secret": {}}, mapErr(rows.Err())
}

func (r *SyncRepo) loadMarkets(ctx context.Context, schema, gameID string) ([]domainsync.EntityRecord, map[string]struct{}, error) {
	rows, err := r.db.Query(ctx, fmt.Sprintf(`
SELECT gm.market_code, gm.is_default, gm.enabled, gm.default_locale
FROM %s.game_markets gm
JOIN %s.games g ON g.id=gm.game_id_ref
WHERE g.game_id=$1 AND gm.enabled=TRUE
ORDER BY gm.market_code ASC`, safeSchema(schema), safeSchema(schema)), gameID)
	if err != nil {
		return nil, nil, mapErr(err)
	}
	defer rows.Close()
	out := []domainsync.EntityRecord{}
	for rows.Next() {
		var market string
		var isDefault, enabled bool
		var locale string
		if err := rows.Scan(&market, &isDefault, &enabled, &locale); err != nil {
			return nil, nil, err
		}
		out = append(out, domainsync.EntityRecord{
			EntityType: "game_market",
			EntityKey:  market,
			Fields: map[string]any{
				"market_code":    market,
				"is_default":     isDefault,
				"enabled":        enabled,
				"default_locale": locale,
			},
		})
	}
	return out, map[string]struct{}{}, mapErr(rows.Err())
}

func (r *SyncRepo) loadLegal(ctx context.Context, schema, gameID string) ([]domainsync.EntityRecord, map[string]struct{}, error) {
	rows, err := r.db.Query(ctx, fmt.Sprintf(`
SELECT scope_type, scope_value, terms_url, privacy_url, delete_account_url
FROM %s.game_legal_links gl
JOIN %s.games g ON g.id=gl.game_id_ref
WHERE g.game_id=$1
ORDER BY scope_type ASC, scope_value ASC`, safeSchema(schema), safeSchema(schema)), gameID)
	if err != nil {
		return nil, nil, mapErr(err)
	}
	defer rows.Close()
	out := []domainsync.EntityRecord{}
	for rows.Next() {
		var scopeType, scopeValue, terms, privacy, delURL string
		if err := rows.Scan(&scopeType, &scopeValue, &terms, &privacy, &delURL); err != nil {
			return nil, nil, err
		}
		key := scopeType + ":" + scopeValue
		out = append(out, domainsync.EntityRecord{
			EntityType: "game_legal_link",
			EntityKey:  key,
			Fields: map[string]any{
				"scope_type":         scopeType,
				"scope_value":        scopeValue,
				"terms_url":          terms,
				"privacy_url":        privacy,
				"delete_account_url": delURL,
			},
		})
	}
	return out, map[string]struct{}{}, mapErr(rows.Err())
}

func (r *SyncRepo) loadChannels(ctx context.Context, schema, gameID string) ([]domainsync.EntityRecord, map[string]struct{}, error) {
	sc := safeSchema(schema)
	masked := map[string]struct{}{"config_json": {}}
	out := []domainsync.EntityRecord{}
	rows, err := r.db.Query(ctx, fmt.Sprintf(`
SELECT gc.market_code, ch.channel_id, gc.enabled, gc.hidden, gc.config_status, gc.remark
FROM %s.game_channels gc
JOIN %s.games g ON g.id=gc.game_id_ref
JOIN platform.channels ch ON ch.id=gc.channel_id_ref
WHERE g.game_id=$1
  AND gc.hidden=FALSE
  AND gc.enabled=TRUE
  AND gc.config_status='valid'
  AND ch.enabled=TRUE
ORDER BY gc.market_code ASC, ch.channel_id ASC`, sc, sc), gameID)
	if err != nil {
		return nil, nil, mapErr(err)
	}
	for rows.Next() {
		var marketCode, channelID, status, remark string
		var enabled, hidden bool
		if err := rows.Scan(&marketCode, &channelID, &enabled, &hidden, &status, &remark); err != nil {
			rows.Close()
			return nil, nil, err
		}
		out = append(out, domainsync.EntityRecord{
			EntityType: "game_channel",
			EntityKey:  marketCode + "/" + channelID,
			Fields: map[string]any{
				"market_code":   marketCode,
				"channel_id":    channelID,
				"enabled":       enabled,
				"hidden":        hidden,
				"config_status": status,
				"remark":        remark,
			},
		})
	}
	if err := mapErr(rows.Err()); err != nil {
		rows.Close()
		return nil, nil, err
	}
	rows.Close()

	// 渠道实例登录 / IAP 单实例配置（唯一键 = market/channel）
	for _, sub := range []struct {
		table      string
		entityType string
	}{
		{"game_channel_login_configs", "game_channel_login_config"},
		{"game_channel_iap_configs", "game_channel_iap_config"},
	} {
		crows, cerr := r.db.Query(ctx, fmt.Sprintf(`
SELECT gc.market_code, ch.channel_id, cfg.enabled, cfg.config_status, cfg.config_json
FROM %s.%s cfg
JOIN %s.game_channels gc ON gc.id=cfg.game_channel_id_ref
JOIN %s.games g ON g.id=gc.game_id_ref
JOIN platform.channels ch ON ch.id=gc.channel_id_ref
WHERE g.game_id=$1
  AND gc.hidden=FALSE AND gc.enabled=TRUE AND gc.config_status='valid' AND ch.enabled=TRUE
  AND cfg.enabled=TRUE AND cfg.config_status='valid'
ORDER BY gc.market_code ASC, ch.channel_id ASC`, sc, sub.table, sc, sc), gameID)
		if cerr != nil {
			return nil, nil, mapErr(cerr)
		}
		for crows.Next() {
			var marketCode, channelID, cfgStatus string
			var enabled bool
			var cfgRaw []byte
			if err := crows.Scan(&marketCode, &channelID, &enabled, &cfgStatus, &cfgRaw); err != nil {
				crows.Close()
				return nil, nil, err
			}
			out = append(out, domainsync.EntityRecord{
				EntityType: sub.entityType,
				EntityKey:  marketCode + "/" + channelID,
				Fields: map[string]any{
					"market_code":   marketCode,
					"channel_id":    channelID,
					"enabled":       enabled,
					"config_status": cfgStatus,
					"config_json":   decodeJSONB(cfgRaw),
				},
			})
		}
		if err := mapErr(crows.Err()); err != nil {
			crows.Close()
			return nil, nil, err
		}
		crows.Close()
	}

	// 渠道实例功能插件配置（唯一键 = market/channel/plugin_id）
	prows, perr := r.db.Query(ctx, fmt.Sprintf(`
SELECT gc.market_code, ch.channel_id, fp.plugin_id, pc.enabled, pc.config_status, pc.config_json
FROM %s.game_channel_plugin_configs pc
JOIN %s.game_channels gc ON gc.id=pc.game_channel_id_ref
JOIN %s.games g ON g.id=gc.game_id_ref
JOIN platform.channels ch ON ch.id=gc.channel_id_ref
JOIN platform.feature_plugins fp ON fp.id=pc.plugin_id_ref
WHERE g.game_id=$1
  AND gc.hidden=FALSE AND gc.enabled=TRUE AND gc.config_status='valid' AND ch.enabled=TRUE
  AND pc.enabled=TRUE AND pc.config_status='valid'
ORDER BY gc.market_code ASC, ch.channel_id ASC, fp.plugin_id ASC`, sc, sc, sc), gameID)
	if perr != nil {
		return nil, nil, mapErr(perr)
	}
	for prows.Next() {
		var marketCode, channelID, pluginID, cfgStatus string
		var enabled bool
		var cfgRaw []byte
		if err := prows.Scan(&marketCode, &channelID, &pluginID, &enabled, &cfgStatus, &cfgRaw); err != nil {
			prows.Close()
			return nil, nil, err
		}
		out = append(out, domainsync.EntityRecord{
			EntityType: "game_channel_plugin_config",
			EntityKey:  marketCode + "/" + channelID + "/" + pluginID,
			Fields: map[string]any{
				"market_code":   marketCode,
				"channel_id":    channelID,
				"plugin_id":     pluginID,
				"enabled":       enabled,
				"config_status": cfgStatus,
				"config_json":   decodeJSONB(cfgRaw),
			},
		})
	}
	if err := mapErr(prows.Err()); err != nil {
		prows.Close()
		return nil, nil, err
	}
	prows.Close()

	return out, masked, nil
}

func (r *SyncRepo) loadPackages(ctx context.Context, schema, gameID string) ([]domainsync.EntityRecord, map[string]struct{}, error) {
	sc := safeSchema(schema)
	masked := map[string]struct{}{"config_json": {}, "override_json": {}}
	out := []domainsync.EntityRecord{}
	rows, err := r.db.Query(ctx, fmt.Sprintf(`
SELECT gc.market_code, ch.channel_id, cp.package_code, cp.package_name, cp.bundle_id, cp.inherit_channel_config, cp.enabled, cp.override_json
FROM %s.channel_packages cp
JOIN %s.game_channels gc ON gc.id=cp.game_channel_id_ref
JOIN %s.games g ON g.id=gc.game_id_ref
JOIN platform.channels ch ON ch.id=gc.channel_id_ref
WHERE g.game_id=$1 AND cp.enabled=TRUE
ORDER BY gc.market_code ASC, ch.channel_id ASC, cp.package_code ASC`, sc, sc, sc), gameID)
	if err != nil {
		return nil, nil, mapErr(err)
	}
	for rows.Next() {
		var market, channelID, packageCode, packageName, bundleID string
		var inherit, enabled bool
		var overrideRaw []byte
		if err := rows.Scan(&market, &channelID, &packageCode, &packageName, &bundleID, &inherit, &enabled, &overrideRaw); err != nil {
			rows.Close()
			return nil, nil, err
		}
		out = append(out, domainsync.EntityRecord{
			EntityType: "channel_package",
			EntityKey:  market + "/" + channelID + "/" + packageCode,
			Fields: map[string]any{
				"market_code":            market,
				"channel_id":             channelID,
				"package_code":           packageCode,
				"package_name":           packageName,
				"bundle_id":              bundleID,
				"inherit_channel_config": inherit,
				"enabled":                enabled,
				"override_json":          decodeJSONB(overrideRaw),
			},
		})
	}
	if err := mapErr(rows.Err()); err != nil {
		rows.Close()
		return nil, nil, err
	}
	rows.Close()

	// 渠道包 IAP 覆盖（唯一键 = market/channel/package_code）
	irows, ierr := r.db.Query(ctx, fmt.Sprintf(`
SELECT gc.market_code, ch.channel_id, cp.package_code, o.enabled, o.config_status, o.config_json
FROM %s.channel_package_iap_overrides o
JOIN %s.channel_packages cp ON cp.id=o.package_id_ref
JOIN %s.game_channels gc ON gc.id=cp.game_channel_id_ref
JOIN %s.games g ON g.id=gc.game_id_ref
JOIN platform.channels ch ON ch.id=gc.channel_id_ref
WHERE g.game_id=$1 AND cp.enabled=TRUE AND o.enabled=TRUE AND o.config_status='valid'
ORDER BY gc.market_code ASC, ch.channel_id ASC, cp.package_code ASC`, sc, sc, sc, sc), gameID)
	if ierr != nil {
		return nil, nil, mapErr(ierr)
	}
	for irows.Next() {
		var market, channelID, packageCode, cfgStatus string
		var enabled bool
		var cfgRaw []byte
		if err := irows.Scan(&market, &channelID, &packageCode, &enabled, &cfgStatus, &cfgRaw); err != nil {
			irows.Close()
			return nil, nil, err
		}
		out = append(out, domainsync.EntityRecord{
			EntityType: "channel_package_iap_override",
			EntityKey:  market + "/" + channelID + "/" + packageCode,
			Fields: map[string]any{
				"market_code":   market,
				"channel_id":    channelID,
				"package_code":  packageCode,
				"enabled":       enabled,
				"config_status": cfgStatus,
				"config_json":   decodeJSONB(cfgRaw),
			},
		})
	}
	if err := mapErr(irows.Err()); err != nil {
		irows.Close()
		return nil, nil, err
	}
	irows.Close()

	// 渠道包功能插件覆盖（唯一键 = market/channel/package_code/plugin_id）
	prows, perr := r.db.Query(ctx, fmt.Sprintf(`
SELECT gc.market_code, ch.channel_id, cp.package_code, fp.plugin_id, o.inherit_channel_config, o.enabled, o.config_status, o.config_json
FROM %s.channel_package_plugin_overrides o
JOIN %s.channel_packages cp ON cp.id=o.package_id_ref
JOIN %s.game_channels gc ON gc.id=cp.game_channel_id_ref
JOIN %s.games g ON g.id=gc.game_id_ref
JOIN platform.channels ch ON ch.id=gc.channel_id_ref
JOIN platform.feature_plugins fp ON fp.id=o.plugin_id_ref
WHERE g.game_id=$1 AND cp.enabled=TRUE AND o.enabled=TRUE AND o.config_status='valid'
ORDER BY gc.market_code ASC, ch.channel_id ASC, cp.package_code ASC, fp.plugin_id ASC`, sc, sc, sc, sc), gameID)
	if perr != nil {
		return nil, nil, mapErr(perr)
	}
	for prows.Next() {
		var market, channelID, packageCode, pluginID, cfgStatus string
		var inherit, enabled bool
		var cfgRaw []byte
		if err := prows.Scan(&market, &channelID, &packageCode, &pluginID, &inherit, &enabled, &cfgStatus, &cfgRaw); err != nil {
			prows.Close()
			return nil, nil, err
		}
		out = append(out, domainsync.EntityRecord{
			EntityType: "channel_package_plugin_override",
			EntityKey:  market + "/" + channelID + "/" + packageCode + "/" + pluginID,
			Fields: map[string]any{
				"market_code":            market,
				"channel_id":             channelID,
				"package_code":           packageCode,
				"plugin_id":              pluginID,
				"inherit_channel_config": inherit,
				"enabled":                enabled,
				"config_status":          cfgStatus,
				"config_json":            decodeJSONB(cfgRaw),
			},
		})
	}
	if err := mapErr(prows.Err()); err != nil {
		prows.Close()
		return nil, nil, err
	}
	prows.Close()

	return out, masked, nil
}

func (r *SyncRepo) loadProducts(ctx context.Context, schema, gameID string) ([]domainsync.EntityRecord, map[string]struct{}, error) {
	rows, err := r.db.Query(ctx, fmt.Sprintf(`
SELECT p.product_id, p.product_name, p.base_amount_minor, p.base_currency, p.price_id, p.enabled
FROM %s.products p
JOIN %s.games g ON g.id=p.game_id_ref
WHERE g.game_id=$1 AND p.enabled=TRUE
ORDER BY p.product_id ASC`, safeSchema(schema), safeSchema(schema)), gameID)
	if err != nil {
		return nil, nil, mapErr(err)
	}
	defer rows.Close()
	out := []domainsync.EntityRecord{}
	for rows.Next() {
		var productID, productName, currency, priceID string
		var amount int64
		var enabled bool
		if err := rows.Scan(&productID, &productName, &amount, &currency, &priceID, &enabled); err != nil {
			return nil, nil, err
		}
		out = append(out, domainsync.EntityRecord{
			EntityType: "product",
			EntityKey:  productID,
			Fields: map[string]any{
				"product_id":        productID,
				"product_name":      productName,
				"base_amount_minor": amount,
				"base_currency":     currency,
				"price_id":          priceID,
				"enabled":           enabled,
			},
		})
	}
	if err := mapErr(rows.Err()); err != nil {
		rows.Close()
		return nil, nil, err
	}
	rows.Close()

	// channel_products（唯一键 = market/channel/package_code/product_id）
	crows, cerr := r.db.Query(ctx, fmt.Sprintf(`
SELECT gc.market_code, ch.channel_id, cp.package_code, p.product_id,
       cprod.product_id_mode, cprod.product_id_override, cprod.price_id_mode, cprod.price_id_override, cprod.enabled
FROM %s.channel_products cprod
JOIN %s.products p ON p.id=cprod.product_id_ref
JOIN %s.channel_packages cp ON cp.id=cprod.package_id_ref
JOIN %s.game_channels gc ON gc.id=cp.game_channel_id_ref
JOIN %s.games g ON g.id=gc.game_id_ref
JOIN platform.channels ch ON ch.id=gc.channel_id_ref
WHERE g.game_id=$1 AND cprod.enabled=TRUE AND p.enabled=TRUE AND cp.enabled=TRUE
ORDER BY gc.market_code ASC, ch.channel_id ASC, cp.package_code ASC, p.product_id ASC`,
		safeSchema(schema), safeSchema(schema), safeSchema(schema), safeSchema(schema), safeSchema(schema)), gameID)
	if cerr != nil {
		return nil, nil, mapErr(cerr)
	}
	for crows.Next() {
		var market, channelID, packageCode, productID, pidMode, pidOverride, priceMode, priceOverride string
		var enabled bool
		if err := crows.Scan(&market, &channelID, &packageCode, &productID, &pidMode, &pidOverride, &priceMode, &priceOverride, &enabled); err != nil {
			crows.Close()
			return nil, nil, err
		}
		out = append(out, domainsync.EntityRecord{
			EntityType: "channel_product",
			EntityKey:  market + "/" + channelID + "/" + packageCode + "/" + productID,
			Fields: map[string]any{
				"market_code":         market,
				"channel_id":          channelID,
				"package_code":        packageCode,
				"product_id":          productID,
				"product_id_mode":     pidMode,
				"product_id_override": pidOverride,
				"price_id_mode":       priceMode,
				"price_id_override":   priceOverride,
				"enabled":             enabled,
			},
		})
	}
	if err := mapErr(crows.Err()); err != nil {
		crows.Close()
		return nil, nil, err
	}
	crows.Close()

	return out, map[string]struct{}{}, nil
}

func (r *SyncRepo) loadCashier(ctx context.Context, schema, gameID string) ([]domainsync.EntityRecord, map[string]struct{}, error) {
	sc := safeSchema(schema)
	out := []domainsync.EntityRecord{}
	rows, err := r.db.Query(ctx, fmt.Sprintf(`
SELECT p.template_id_ref, p.applied_template_version_id, p.snapshot_checksum
FROM %s.game_cashier_profiles p
JOIN %s.games g ON g.id=p.game_id_ref
WHERE g.game_id=$1`, sc, sc), gameID)
	if err != nil {
		return nil, nil, mapErr(err)
	}
	for rows.Next() {
		var templateID, versionID int64
		var checksum string
		if err := rows.Scan(&templateID, &versionID, &checksum); err != nil {
			rows.Close()
			return nil, nil, err
		}
		out = append(out, domainsync.EntityRecord{
			EntityType: "game_cashier_profile",
			EntityKey:  gameID,
			Fields: map[string]any{
				"template_id_ref":             templateID,
				"applied_template_version_id": versionID,
				"snapshot_checksum":           checksum,
			},
		})
	}
	if err := mapErr(rows.Err()); err != nil {
		rows.Close()
		return nil, nil, err
	}
	rows.Close()

	// 价格覆盖（唯一键 = country/region/currency/price_id）
	orows, oerr := r.db.Query(ctx, fmt.Sprintf(`
SELECT o.country_code, o.region_code, o.currency, o.price_id,
       o.pre_tax_amount_minor, o.tax_rate::text, o.tax_amount_minor, o.after_tax_amount_minor, o.reason, o.effective_at
FROM %s.game_cashier_price_overrides o
JOIN %s.games g ON g.id=o.game_id_ref
WHERE g.game_id=$1
ORDER BY o.country_code ASC, o.region_code ASC, o.currency ASC, o.price_id ASC`, sc, sc), gameID)
	if oerr != nil {
		return nil, nil, mapErr(oerr)
	}
	for orows.Next() {
		var country, region, currency, priceID, taxRate, reason string
		var preTax, taxAmount, afterTax int64
		var effectiveAt time.Time
		if err := orows.Scan(&country, &region, &currency, &priceID, &preTax, &taxRate, &taxAmount, &afterTax, &reason, &effectiveAt); err != nil {
			orows.Close()
			return nil, nil, err
		}
		out = append(out, domainsync.EntityRecord{
			EntityType: "game_cashier_price_override",
			EntityKey:  country + "/" + region + "/" + currency + "/" + priceID,
			Fields: map[string]any{
				"country_code":           country,
				"region_code":            region,
				"currency":               currency,
				"price_id":               priceID,
				"pre_tax_amount_minor":   preTax,
				"tax_rate":               taxRate,
				"tax_amount_minor":       taxAmount,
				"after_tax_amount_minor": afterTax,
				"reason":                 reason,
				"effective_at":           effectiveAt.UTC().Format(time.RFC3339),
			},
		})
	}
	if err := mapErr(orows.Err()); err != nil {
		orows.Close()
		return nil, nil, err
	}
	orows.Close()

	return out, map[string]struct{}{}, nil
}

func (r *SyncRepo) loadPayments(ctx context.Context, schema, gameID string) ([]domainsync.EntityRecord, map[string]struct{}, error) {
	rows, err := r.db.Query(ctx, fmt.Sprintf(`
SELECT COALESCE(ch.channel_id,'*'), COALESCE(cp.package_code,'*'), market_code, country_code, currency, pw.pay_way_id, pr.priority, pr.enabled
FROM %s.payment_routes pr
JOIN %s.games g ON g.id=pr.game_id_ref
JOIN platform.pay_ways pw ON pw.id=pr.pay_way_id_ref
LEFT JOIN platform.channels ch ON ch.id=pr.channel_id_ref
LEFT JOIN %s.channel_packages cp ON cp.id=pr.package_id_ref
WHERE g.game_id=$1 AND pr.enabled=TRUE
ORDER BY pw.pay_way_id ASC, pr.priority ASC`, safeSchema(schema), safeSchema(schema), safeSchema(schema)), gameID)
	if err != nil {
		return nil, nil, mapErr(err)
	}
	defer rows.Close()
	out := []domainsync.EntityRecord{}
	for rows.Next() {
		var channelID, packageCode, market, country, currency, payWay string
		var priority int
		var enabled bool
		if err := rows.Scan(&channelID, &packageCode, &market, &country, &currency, &payWay, &priority, &enabled); err != nil {
			return nil, nil, err
		}
		key := strings.Join([]string{payWay, packageCode, channelID, market, country, currency}, "/")
		out = append(out, domainsync.EntityRecord{
			EntityType: "payment_route",
			EntityKey:  key,
			Fields: map[string]any{
				"pay_way_id":   payWay,
				"package_code": packageCode,
				"channel_id":   channelID,
				"market_code":  market,
				"country_code": country,
				"currency":     currency,
				"priority":     priority,
				"enabled":      enabled,
			},
		})
	}
	return out, map[string]struct{}{}, mapErr(rows.Err())
}

func (r *SyncRepo) loadConfig(ctx context.Context, schema, gameID string) ([]domainsync.EntityRecord, map[string]struct{}, error) {
	rows, err := r.db.Query(ctx, fmt.Sprintf(`
SELECT config_version, file_hash
FROM %s.game_config_snapshots s
JOIN %s.games g ON g.id=s.game_id_ref
WHERE g.game_id=$1 AND s.status='published'
ORDER BY s.published_at DESC, s.id DESC`, safeSchema(schema), safeSchema(schema)), gameID)
	if err != nil {
		return nil, nil, mapErr(err)
	}
	defer rows.Close()
	out := []domainsync.EntityRecord{}
	for rows.Next() {
		var version, hash string
		if err := rows.Scan(&version, &hash); err != nil {
			return nil, nil, err
		}
		out = append(out, domainsync.EntityRecord{
			EntityType: "config_snapshot",
			EntityKey:  version,
			Fields: map[string]any{
				"config_version": version,
				"file_hash":      hash,
			},
		})
	}
	return out, map[string]struct{}{}, mapErr(rows.Err())
}

func (r *SyncRepo) applyGame(ctx context.Context, gameID string) error {
	_, err := r.db.Exec(ctx, `
INSERT INTO production.games (game_id, game_secret, name, alias, icon_url, default_market_code, status)
SELECT game_id, game_secret, name, alias, icon_url, default_market_code, status
FROM sandbox.games
WHERE game_id=$1
ON CONFLICT (game_id) DO UPDATE SET
  game_secret=EXCLUDED.game_secret,
  name=EXCLUDED.name,
  alias=EXCLUDED.alias,
  icon_url=EXCLUDED.icon_url,
  default_market_code=EXCLUDED.default_market_code,
  status=EXCLUDED.status,
  updated_at=NOW()`, gameID)
	return mapErr(err)
}

func (r *SyncRepo) applyMarkets(ctx context.Context, gameID string, includeDeletes bool) error {
	_, err := r.db.Exec(ctx, `
INSERT INTO production.game_markets (game_id_ref, market_code, is_default, enabled, default_locale)
SELECT pg.id, sm.market_code, sm.is_default, sm.enabled, sm.default_locale
FROM sandbox.game_markets sm
JOIN sandbox.games sg ON sg.id=sm.game_id_ref
JOIN production.games pg ON pg.game_id=sg.game_id
WHERE sg.game_id=$1 AND sm.enabled=TRUE
ON CONFLICT (game_id_ref, market_code) DO UPDATE SET
  is_default=EXCLUDED.is_default,
  enabled=EXCLUDED.enabled,
  default_locale=EXCLUDED.default_locale,
  updated_at=NOW()`, gameID)
	if err != nil {
		return mapErr(err)
	}
	if includeDeletes {
		_, err = r.db.Exec(ctx, `
DELETE FROM production.game_markets pm
USING production.games pg
WHERE pm.game_id_ref=pg.id
  AND pg.game_id=$1
  AND NOT EXISTS (
    SELECT 1 FROM sandbox.game_markets sm
    JOIN sandbox.games sg ON sg.id=sm.game_id_ref
    WHERE sg.game_id=$1 AND sm.market_code=pm.market_code AND sm.enabled=TRUE
  )`, gameID)
	}
	return mapErr(err)
}

func (r *SyncRepo) applyLegal(ctx context.Context, gameID string, includeDeletes bool) error {
	_, err := r.db.Exec(ctx, `
INSERT INTO production.game_legal_links (game_id_ref, scope_type, scope_value, terms_url, privacy_url, delete_account_url)
SELECT pg.id, sl.scope_type, sl.scope_value, sl.terms_url, sl.privacy_url, sl.delete_account_url
FROM sandbox.game_legal_links sl
JOIN sandbox.games sg ON sg.id=sl.game_id_ref
JOIN production.games pg ON pg.game_id=sg.game_id
WHERE sg.game_id=$1
ON CONFLICT (game_id_ref, scope_type, scope_value) DO UPDATE SET
  terms_url=EXCLUDED.terms_url,
  privacy_url=EXCLUDED.privacy_url,
  delete_account_url=EXCLUDED.delete_account_url,
  updated_at=NOW()`, gameID)
	if err != nil {
		return mapErr(err)
	}
	if includeDeletes {
		_, err = r.db.Exec(ctx, `
DELETE FROM production.game_legal_links pl
USING production.games pg
WHERE pl.game_id_ref=pg.id
  AND pg.game_id=$1
  AND NOT EXISTS (
    SELECT 1 FROM sandbox.game_legal_links sl
    JOIN sandbox.games sg ON sg.id=sl.game_id_ref
    WHERE sg.game_id=$1 AND sl.scope_type=pl.scope_type AND sl.scope_value=pl.scope_value
  )`, gameID)
	}
	return mapErr(err)
}

func (r *SyncRepo) applyChannels(ctx context.Context, gameID string, includeDeletes bool) error {
	_, err := r.db.Exec(ctx, `
INSERT INTO production.game_channels (game_id_ref, channel_id_ref, market_code, enabled, hidden, config_status, remark)
SELECT pg.id, sc.channel_id_ref, sc.market_code, sc.enabled, sc.hidden, sc.config_status, sc.remark
FROM sandbox.game_channels sc
JOIN sandbox.games sg ON sg.id=sc.game_id_ref
JOIN production.games pg ON pg.game_id=sg.game_id
WHERE sg.game_id=$1 AND sc.hidden=FALSE AND sc.enabled=TRUE AND sc.config_status='valid'
ON CONFLICT (game_id_ref, market_code, channel_id_ref) DO UPDATE SET
  enabled=EXCLUDED.enabled,
  hidden=EXCLUDED.hidden,
  config_status=EXCLUDED.config_status,
  remark=EXCLUDED.remark,
  updated_at=NOW()`, gameID)
	if err != nil {
		return mapErr(err)
	}
	// 登录 / IAP 单实例配置 upsert（config_json 服务端搬运，密文不经明文）
	for _, table := range []string{"game_channel_login_configs", "game_channel_iap_configs"} {
		_, err = r.db.Exec(ctx, fmt.Sprintf(`
INSERT INTO production.%s (game_channel_id_ref, enabled, config_json, config_status, last_check_at, last_check_message)
SELECT pgc.id, scfg.enabled, scfg.config_json, scfg.config_status, scfg.last_check_at, scfg.last_check_message
FROM sandbox.%s scfg
JOIN sandbox.game_channels sgc ON sgc.id=scfg.game_channel_id_ref
JOIN sandbox.games sg ON sg.id=sgc.game_id_ref
JOIN production.games pg ON pg.game_id=sg.game_id
JOIN production.game_channels pgc ON pgc.game_id_ref=pg.id AND pgc.channel_id_ref=sgc.channel_id_ref AND pgc.market_code=sgc.market_code
WHERE sg.game_id=$1
  AND sgc.hidden=FALSE AND sgc.enabled=TRUE AND sgc.config_status='valid'
  AND scfg.enabled=TRUE AND scfg.config_status='valid'
ON CONFLICT (game_channel_id_ref) DO UPDATE SET
  enabled=EXCLUDED.enabled,
  config_json=EXCLUDED.config_json,
  config_status=EXCLUDED.config_status,
  last_check_at=EXCLUDED.last_check_at,
  last_check_message=EXCLUDED.last_check_message,
  updated_at=NOW()`, table, table), gameID)
		if err != nil {
			return mapErr(err)
		}
	}
	// 功能插件配置 upsert（唯一键含 plugin_id_ref，platform 共享）
	_, err = r.db.Exec(ctx, `
INSERT INTO production.game_channel_plugin_configs (game_channel_id_ref, plugin_id_ref, enabled, config_json, config_status, last_check_at, last_check_message)
SELECT pgc.id, spc.plugin_id_ref, spc.enabled, spc.config_json, spc.config_status, spc.last_check_at, spc.last_check_message
FROM sandbox.game_channel_plugin_configs spc
JOIN sandbox.game_channels sgc ON sgc.id=spc.game_channel_id_ref
JOIN sandbox.games sg ON sg.id=sgc.game_id_ref
JOIN production.games pg ON pg.game_id=sg.game_id
JOIN production.game_channels pgc ON pgc.game_id_ref=pg.id AND pgc.channel_id_ref=sgc.channel_id_ref AND pgc.market_code=sgc.market_code
WHERE sg.game_id=$1
  AND sgc.hidden=FALSE AND sgc.enabled=TRUE AND sgc.config_status='valid'
  AND spc.enabled=TRUE AND spc.config_status='valid'
ON CONFLICT (game_channel_id_ref, plugin_id_ref) DO UPDATE SET
  enabled=EXCLUDED.enabled,
  config_json=EXCLUDED.config_json,
  config_status=EXCLUDED.config_status,
  last_check_at=EXCLUDED.last_check_at,
  last_check_message=EXCLUDED.last_check_message,
  updated_at=NOW()`, gameID)
	if err != nil {
		return mapErr(err)
	}
	if includeDeletes {
		// 先删子配置（FK 安全），再删渠道实例本身。
		for _, table := range []string{"game_channel_login_configs", "game_channel_iap_configs", "game_channel_plugin_configs"} {
			_, err = r.db.Exec(ctx, fmt.Sprintf(`
DELETE FROM production.%s pcfg
USING production.game_channels pgc, production.games pg
WHERE pcfg.game_channel_id_ref=pgc.id
  AND pgc.game_id_ref=pg.id
  AND pg.game_id=$1
  AND NOT EXISTS (
    SELECT 1
    FROM sandbox.%s scfg
    JOIN sandbox.game_channels sgc ON sgc.id=scfg.game_channel_id_ref
    JOIN sandbox.games sg ON sg.id=sgc.game_id_ref
    WHERE sg.game_id=$1
      AND sgc.market_code=pgc.market_code
      AND sgc.channel_id_ref=pgc.channel_id_ref
      AND scfg.enabled=TRUE AND scfg.config_status='valid'
  )`, table, table), gameID)
			if err != nil {
				return mapErr(err)
			}
		}
		_, err = r.db.Exec(ctx, `
DELETE FROM production.game_channels pc
USING production.games pg, platform.channels ch
WHERE pc.game_id_ref=pg.id
  AND pc.channel_id_ref=ch.id
  AND pg.game_id=$1
  AND NOT EXISTS (
    SELECT 1 FROM sandbox.game_channels sc
    JOIN sandbox.games sg ON sg.id=sc.game_id_ref
    WHERE sg.game_id=$1
      AND sc.market_code=pc.market_code
      AND sc.channel_id_ref=pc.channel_id_ref
      AND sc.hidden=FALSE AND sc.enabled=TRUE AND sc.config_status='valid'
  )`, gameID)
	}
	return mapErr(err)
}

func (r *SyncRepo) applyPackages(ctx context.Context, gameID string, includeDeletes bool) error {
	_, err := r.db.Exec(ctx, `
INSERT INTO production.channel_packages (game_channel_id_ref, package_code, package_name, market_code, bundle_id, inherit_channel_config, enabled, override_json)
SELECT pgc.id, scp.package_code, scp.package_name, scp.market_code, scp.bundle_id, scp.inherit_channel_config, scp.enabled, scp.override_json
FROM sandbox.channel_packages scp
JOIN sandbox.game_channels sgc ON sgc.id=scp.game_channel_id_ref
JOIN sandbox.games sg ON sg.id=sgc.game_id_ref
JOIN production.games pg ON pg.game_id=sg.game_id
JOIN production.game_channels pgc ON pgc.game_id_ref=pg.id AND pgc.channel_id_ref=sgc.channel_id_ref AND pgc.market_code=sgc.market_code
WHERE sg.game_id=$1 AND scp.enabled=TRUE
ON CONFLICT (game_channel_id_ref, package_code) DO UPDATE SET
  package_name=EXCLUDED.package_name,
  market_code=EXCLUDED.market_code,
  bundle_id=EXCLUDED.bundle_id,
  inherit_channel_config=EXCLUDED.inherit_channel_config,
  enabled=EXCLUDED.enabled,
  override_json=EXCLUDED.override_json,
  updated_at=NOW()`, gameID)
	if err != nil {
		return mapErr(err)
	}
	// 渠道包 IAP 覆盖 upsert（唯一键 = package_id_ref）
	_, err = r.db.Exec(ctx, `
INSERT INTO production.channel_package_iap_overrides (package_id_ref, enabled, config_json, config_status, last_check_at, last_check_message)
SELECT pcp.id, so.enabled, so.config_json, so.config_status, so.last_check_at, so.last_check_message
FROM sandbox.channel_package_iap_overrides so
JOIN sandbox.channel_packages scp ON scp.id=so.package_id_ref
JOIN sandbox.game_channels sgc ON sgc.id=scp.game_channel_id_ref
JOIN sandbox.games sg ON sg.id=sgc.game_id_ref
JOIN production.games pg ON pg.game_id=sg.game_id
JOIN production.game_channels pgc ON pgc.game_id_ref=pg.id AND pgc.channel_id_ref=sgc.channel_id_ref AND pgc.market_code=sgc.market_code
JOIN production.channel_packages pcp ON pcp.game_channel_id_ref=pgc.id AND pcp.package_code=scp.package_code
WHERE sg.game_id=$1 AND scp.enabled=TRUE AND so.enabled=TRUE AND so.config_status='valid'
ON CONFLICT (package_id_ref) DO UPDATE SET
  enabled=EXCLUDED.enabled,
  config_json=EXCLUDED.config_json,
  config_status=EXCLUDED.config_status,
  last_check_at=EXCLUDED.last_check_at,
  last_check_message=EXCLUDED.last_check_message,
  updated_at=NOW()`, gameID)
	if err != nil {
		return mapErr(err)
	}
	// 渠道包功能插件覆盖 upsert（唯一键 = package_id_ref, plugin_id_ref）
	_, err = r.db.Exec(ctx, `
INSERT INTO production.channel_package_plugin_overrides (package_id_ref, plugin_id_ref, inherit_channel_config, enabled, config_json, config_status, last_check_at, last_check_message)
SELECT pcp.id, so.plugin_id_ref, so.inherit_channel_config, so.enabled, so.config_json, so.config_status, so.last_check_at, so.last_check_message
FROM sandbox.channel_package_plugin_overrides so
JOIN sandbox.channel_packages scp ON scp.id=so.package_id_ref
JOIN sandbox.game_channels sgc ON sgc.id=scp.game_channel_id_ref
JOIN sandbox.games sg ON sg.id=sgc.game_id_ref
JOIN production.games pg ON pg.game_id=sg.game_id
JOIN production.game_channels pgc ON pgc.game_id_ref=pg.id AND pgc.channel_id_ref=sgc.channel_id_ref AND pgc.market_code=sgc.market_code
JOIN production.channel_packages pcp ON pcp.game_channel_id_ref=pgc.id AND pcp.package_code=scp.package_code
WHERE sg.game_id=$1 AND scp.enabled=TRUE AND so.enabled=TRUE AND so.config_status='valid'
ON CONFLICT (package_id_ref, plugin_id_ref) DO UPDATE SET
  inherit_channel_config=EXCLUDED.inherit_channel_config,
  enabled=EXCLUDED.enabled,
  config_json=EXCLUDED.config_json,
  config_status=EXCLUDED.config_status,
  last_check_at=EXCLUDED.last_check_at,
  last_check_message=EXCLUDED.last_check_message,
  updated_at=NOW()`, gameID)
	if err != nil {
		return mapErr(err)
	}
	if includeDeletes {
		// 先删包覆盖子表（FK 安全），再删渠道包本身。
		_, err = r.db.Exec(ctx, `
DELETE FROM production.channel_package_iap_overrides po
USING production.channel_packages pcp, production.game_channels pgc, production.games pg
WHERE po.package_id_ref=pcp.id
  AND pcp.game_channel_id_ref=pgc.id
  AND pgc.game_id_ref=pg.id
  AND pg.game_id=$1
  AND NOT EXISTS (
    SELECT 1
    FROM sandbox.channel_package_iap_overrides so
    JOIN sandbox.channel_packages scp ON scp.id=so.package_id_ref
    JOIN sandbox.game_channels sgc ON sgc.id=scp.game_channel_id_ref
    JOIN sandbox.games sg ON sg.id=sgc.game_id_ref
    WHERE sg.game_id=$1
      AND sgc.market_code=pgc.market_code
      AND sgc.channel_id_ref=pgc.channel_id_ref
      AND scp.package_code=pcp.package_code
      AND scp.enabled=TRUE AND so.enabled=TRUE AND so.config_status='valid'
  )`, gameID)
		if err != nil {
			return mapErr(err)
		}
		_, err = r.db.Exec(ctx, `
DELETE FROM production.channel_package_plugin_overrides po
USING production.channel_packages pcp, production.game_channels pgc, production.games pg
WHERE po.package_id_ref=pcp.id
  AND pcp.game_channel_id_ref=pgc.id
  AND pgc.game_id_ref=pg.id
  AND pg.game_id=$1
  AND NOT EXISTS (
    SELECT 1
    FROM sandbox.channel_package_plugin_overrides so
    JOIN sandbox.channel_packages scp ON scp.id=so.package_id_ref
    JOIN sandbox.game_channels sgc ON sgc.id=scp.game_channel_id_ref
    JOIN sandbox.games sg ON sg.id=sgc.game_id_ref
    WHERE sg.game_id=$1
      AND sgc.market_code=pgc.market_code
      AND sgc.channel_id_ref=pgc.channel_id_ref
      AND scp.package_code=pcp.package_code
      AND so.plugin_id_ref=po.plugin_id_ref
      AND scp.enabled=TRUE AND so.enabled=TRUE AND so.config_status='valid'
  )`, gameID)
		if err != nil {
			return mapErr(err)
		}
		_, err = r.db.Exec(ctx, `
DELETE FROM production.channel_packages pcp
USING production.game_channels pgc, production.games pg
WHERE pcp.game_channel_id_ref=pgc.id
  AND pgc.game_id_ref=pg.id
  AND pg.game_id=$1
  AND NOT EXISTS (
    SELECT 1
    FROM sandbox.channel_packages scp
    JOIN sandbox.game_channels sgc ON sgc.id=scp.game_channel_id_ref
    JOIN sandbox.games sg ON sg.id=sgc.game_id_ref
    WHERE sg.game_id=$1
      AND sgc.market_code=pgc.market_code
      AND sgc.channel_id_ref=pgc.channel_id_ref
      AND scp.package_code=pcp.package_code
      AND scp.enabled=TRUE
  )`, gameID)
	}
	return mapErr(err)
}

func (r *SyncRepo) applyProducts(ctx context.Context, gameID string, includeDeletes bool) error {
	_, err := r.db.Exec(ctx, `
INSERT INTO production.products (game_id_ref, product_id, product_name, base_amount_minor, base_currency, price_id, enabled)
SELECT pg.id, sp.product_id, sp.product_name, sp.base_amount_minor, sp.base_currency, sp.price_id, sp.enabled
FROM sandbox.products sp
JOIN sandbox.games sg ON sg.id=sp.game_id_ref
JOIN production.games pg ON pg.game_id=sg.game_id
WHERE sg.game_id=$1 AND sp.enabled=TRUE
ON CONFLICT (game_id_ref, product_id) DO UPDATE SET
  product_name=EXCLUDED.product_name,
  base_amount_minor=EXCLUDED.base_amount_minor,
  base_currency=EXCLUDED.base_currency,
  price_id=EXCLUDED.price_id,
  enabled=EXCLUDED.enabled,
  updated_at=NOW()`, gameID)
	if err != nil {
		return mapErr(err)
	}
	// channel_products upsert（依赖 production 已存在的 products + channel_packages）
	_, err = r.db.Exec(ctx, `
INSERT INTO production.channel_products (product_id_ref, package_id_ref, product_id_mode, product_id_override, price_id_mode, price_id_override, enabled)
SELECT pp.id, pcp.id, scprod.product_id_mode, scprod.product_id_override, scprod.price_id_mode, scprod.price_id_override, scprod.enabled
FROM sandbox.channel_products scprod
JOIN sandbox.products sp ON sp.id=scprod.product_id_ref
JOIN sandbox.channel_packages scp ON scp.id=scprod.package_id_ref
JOIN sandbox.game_channels sgc ON sgc.id=scp.game_channel_id_ref
JOIN sandbox.games sg ON sg.id=sgc.game_id_ref
JOIN production.games pg ON pg.game_id=sg.game_id
JOIN production.products pp ON pp.game_id_ref=pg.id AND pp.product_id=sp.product_id
JOIN production.game_channels pgc ON pgc.game_id_ref=pg.id AND pgc.channel_id_ref=sgc.channel_id_ref AND pgc.market_code=sgc.market_code
JOIN production.channel_packages pcp ON pcp.game_channel_id_ref=pgc.id AND pcp.package_code=scp.package_code
WHERE sg.game_id=$1 AND scprod.enabled=TRUE AND sp.enabled=TRUE AND scp.enabled=TRUE
ON CONFLICT (package_id_ref, product_id_ref) DO UPDATE SET
  product_id_mode=EXCLUDED.product_id_mode,
  product_id_override=EXCLUDED.product_id_override,
  price_id_mode=EXCLUDED.price_id_mode,
  price_id_override=EXCLUDED.price_id_override,
  enabled=EXCLUDED.enabled,
  updated_at=NOW()`, gameID)
	if err != nil {
		return mapErr(err)
	}
	if includeDeletes {
		// 先删 channel_products（FK 引用 products），再删 products。
		_, err = r.db.Exec(ctx, `
DELETE FROM production.channel_products pcprod
USING production.products pp, production.games pg
WHERE pcprod.product_id_ref=pp.id
  AND pp.game_id_ref=pg.id
  AND pg.game_id=$1
  AND NOT EXISTS (
    SELECT 1
    FROM sandbox.channel_products scprod
    JOIN sandbox.products sp ON sp.id=scprod.product_id_ref
    JOIN sandbox.channel_packages scp ON scp.id=scprod.package_id_ref
    JOIN sandbox.game_channels sgc ON sgc.id=scp.game_channel_id_ref
    JOIN sandbox.games sg ON sg.id=sgc.game_id_ref
    JOIN production.game_channels pgc ON pgc.game_id_ref=pg.id AND pgc.channel_id_ref=sgc.channel_id_ref AND pgc.market_code=sgc.market_code
    JOIN production.channel_packages pcp2 ON pcp2.game_channel_id_ref=pgc.id AND pcp2.package_code=scp.package_code
    WHERE sg.game_id=$1
      AND pcprod.package_id_ref=pcp2.id
      AND sp.product_id=pp.product_id
      AND scprod.enabled=TRUE AND sp.enabled=TRUE AND scp.enabled=TRUE
  )`, gameID)
		if err != nil {
			return mapErr(err)
		}
		_, err = r.db.Exec(ctx, `
DELETE FROM production.products pp
USING production.games pg
WHERE pp.game_id_ref=pg.id
  AND pg.game_id=$1
  AND NOT EXISTS (
    SELECT 1 FROM sandbox.products sp
    JOIN sandbox.games sg ON sg.id=sp.game_id_ref
    WHERE sg.game_id=$1 AND sp.product_id=pp.product_id AND sp.enabled=TRUE
  )`, gameID)
	}
	return mapErr(err)
}

func (r *SyncRepo) applyCashier(ctx context.Context, gameID string, includeDeletes bool) error {
	_, err := r.db.Exec(ctx, `
INSERT INTO production.game_cashier_profiles (game_id_ref, template_id_ref, applied_template_version_id, snapshot_checksum, applied_at)
SELECT pg.id, scp.template_id_ref, scp.applied_template_version_id, scp.snapshot_checksum, scp.applied_at
FROM sandbox.game_cashier_profiles scp
JOIN sandbox.games sg ON sg.id=scp.game_id_ref
JOIN production.games pg ON pg.game_id=sg.game_id
WHERE sg.game_id=$1
ON CONFLICT (game_id_ref) DO UPDATE SET
  template_id_ref=EXCLUDED.template_id_ref,
  applied_template_version_id=EXCLUDED.applied_template_version_id,
  snapshot_checksum=EXCLUDED.snapshot_checksum,
  applied_at=EXCLUDED.applied_at,
  updated_at=NOW()`, gameID)
	if err != nil {
		return mapErr(err)
	}
	// 价格覆盖 upsert（唯一键 = game_id_ref, country, region, currency, price_id）
	_, err = r.db.Exec(ctx, `
INSERT INTO production.game_cashier_price_overrides
  (game_id_ref, country_code, region_code, currency, price_id, pre_tax_amount_minor, tax_rate, tax_amount_minor, after_tax_amount_minor, reason, effective_at)
SELECT pg.id, so.country_code, so.region_code, so.currency, so.price_id, so.pre_tax_amount_minor, so.tax_rate, so.tax_amount_minor, so.after_tax_amount_minor, so.reason, so.effective_at
FROM sandbox.game_cashier_price_overrides so
JOIN sandbox.games sg ON sg.id=so.game_id_ref
JOIN production.games pg ON pg.game_id=sg.game_id
WHERE sg.game_id=$1
ON CONFLICT (game_id_ref, country_code, region_code, currency, price_id) DO UPDATE SET
  pre_tax_amount_minor=EXCLUDED.pre_tax_amount_minor,
  tax_rate=EXCLUDED.tax_rate,
  tax_amount_minor=EXCLUDED.tax_amount_minor,
  after_tax_amount_minor=EXCLUDED.after_tax_amount_minor,
  reason=EXCLUDED.reason,
  effective_at=EXCLUDED.effective_at,
  updated_at=NOW()`, gameID)
	if err != nil {
		return mapErr(err)
	}
	if includeDeletes {
		_, err = r.db.Exec(ctx, `
DELETE FROM production.game_cashier_price_overrides po
USING production.games pg
WHERE po.game_id_ref=pg.id
  AND pg.game_id=$1
  AND NOT EXISTS (
    SELECT 1 FROM sandbox.game_cashier_price_overrides so
    JOIN sandbox.games sg ON sg.id=so.game_id_ref
    WHERE sg.game_id=$1
      AND so.country_code=po.country_code
      AND so.region_code=po.region_code
      AND so.currency=po.currency
      AND so.price_id=po.price_id
  )`, gameID)
		if err != nil {
			return mapErr(err)
		}
		_, err = r.db.Exec(ctx, `
DELETE FROM production.game_cashier_profiles pcp
USING production.games pg
WHERE pcp.game_id_ref=pg.id
  AND pg.game_id=$1
  AND NOT EXISTS (
    SELECT 1 FROM sandbox.game_cashier_profiles scp
    JOIN sandbox.games sg ON sg.id=scp.game_id_ref
    WHERE sg.game_id=$1
  )`, gameID)
	}
	return mapErr(err)
}

func (r *SyncRepo) applyPayments(ctx context.Context, gameID string, includeDeletes bool) error {
	_, err := r.db.Exec(ctx, `
INSERT INTO production.payment_routes
  (game_id_ref, market_code, country_code, currency, channel_id_ref, package_id_ref, pay_way_id_ref, provider_id_ref, merchant_account_id_ref, priority, enabled)
SELECT
  pg.id, spr.market_code, spr.country_code, spr.currency,
  spr.channel_id_ref, pcp.id, spr.pay_way_id_ref, spr.provider_id_ref, spr.merchant_account_id_ref,
  spr.priority, spr.enabled
FROM sandbox.payment_routes spr
JOIN sandbox.games sg ON sg.id=spr.game_id_ref
JOIN production.games pg ON pg.game_id=sg.game_id
LEFT JOIN sandbox.channel_packages scp ON scp.id=spr.package_id_ref
LEFT JOIN sandbox.game_channels sgc ON sgc.id=scp.game_channel_id_ref
LEFT JOIN production.game_channels pgc ON pgc.game_id_ref=pg.id AND pgc.channel_id_ref=sgc.channel_id_ref AND pgc.market_code=sgc.market_code
LEFT JOIN production.channel_packages pcp ON pcp.game_channel_id_ref=pgc.id AND pcp.package_code=scp.package_code
WHERE sg.game_id=$1 AND spr.enabled=TRUE
ON CONFLICT (game_id_ref, pay_way_id_ref, COALESCE(package_id_ref, -1), COALESCE(channel_id_ref, -1), market_code, country_code, currency) WHERE enabled
DO UPDATE SET
  provider_id_ref=EXCLUDED.provider_id_ref,
  merchant_account_id_ref=EXCLUDED.merchant_account_id_ref,
  priority=EXCLUDED.priority,
  enabled=EXCLUDED.enabled,
  updated_at=NOW()`, gameID)
	if err != nil {
		return mapErr(err)
	}
	if includeDeletes {
		_, err = r.db.Exec(ctx, `
DELETE FROM production.payment_routes pr
USING production.games pg
WHERE pr.game_id_ref=pg.id
  AND pg.game_id=$1
  AND NOT EXISTS (
    SELECT 1
    FROM sandbox.payment_routes spr
    JOIN sandbox.games sg ON sg.id=spr.game_id_ref
    WHERE sg.game_id=$1
      AND spr.enabled=TRUE
      AND spr.market_code=pr.market_code
      AND spr.country_code=pr.country_code
      AND spr.currency=pr.currency
      AND COALESCE(spr.channel_id_ref,-1)=COALESCE(pr.channel_id_ref,-1)
      AND COALESCE(spr.package_id_ref,-1)=COALESCE(pr.package_id_ref,-1)
      AND spr.pay_way_id_ref=pr.pay_way_id_ref
      AND spr.provider_id_ref=pr.provider_id_ref
      AND spr.merchant_account_id_ref=pr.merchant_account_id_ref
  )`, gameID)
	}
	return mapErr(err)
}

func (r *SyncRepo) applyConfig(ctx context.Context, gameID string, includeDeletes bool) error {
	_, err := r.db.Exec(ctx, `
INSERT INTO production.game_config_snapshots
  (game_id_ref, config_schema_version, config_version, config_json, file_name, file_hash, storage_key, status, generated_at, published_at)
SELECT pg.id, scs.config_schema_version, scs.config_version, scs.config_json, scs.file_name, scs.file_hash, scs.storage_key, scs.status, scs.generated_at, scs.published_at
FROM sandbox.game_config_snapshots scs
JOIN sandbox.games sg ON sg.id=scs.game_id_ref
JOIN production.games pg ON pg.game_id=sg.game_id
WHERE sg.game_id=$1 AND scs.status='published'
ON CONFLICT (game_id_ref, config_version) DO UPDATE SET
  config_schema_version=EXCLUDED.config_schema_version,
  config_json=EXCLUDED.config_json,
  file_name=EXCLUDED.file_name,
  file_hash=EXCLUDED.file_hash,
  storage_key=EXCLUDED.storage_key,
  status=EXCLUDED.status,
  generated_at=EXCLUDED.generated_at,
  published_at=EXCLUDED.published_at,
  updated_at=NOW()`, gameID)
	if err != nil {
		return mapErr(err)
	}
	if includeDeletes {
		_, err = r.db.Exec(ctx, `
DELETE FROM production.game_config_snapshots pcs
USING production.games pg
WHERE pcs.game_id_ref=pg.id
  AND pg.game_id=$1
  AND NOT EXISTS (
    SELECT 1
    FROM sandbox.game_config_snapshots scs
    JOIN sandbox.games sg ON sg.id=scs.game_id_ref
    WHERE sg.game_id=$1
      AND scs.status='published'
      AND scs.config_version=pcs.config_version
  )`, gameID)
	}
	return mapErr(err)
}

// decodeJSONB 将 JSONB 原始字节解码为可确定性哈希/对比的值；空则返回空对象。
func decodeJSONB(raw []byte) any {
	if len(raw) == 0 {
		return map[string]any{}
	}
	var v any
	if err := json.Unmarshal(raw, &v); err != nil {
		return string(raw)
	}
	if v == nil {
		return map[string]any{}
	}
	return v
}

func safeSchema(schema string) string {
	switch strings.ToLower(strings.TrimSpace(schema)) {
	case "develop":
		return "develop"
	case "sandbox":
		return "sandbox"
	case "production":
		return "production"
	default:
		return "sandbox"
	}
}

