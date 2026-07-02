package dashboard

import (
	"context"
	"slices"
	"strings"
	"time"

	adminapp "github.com/csw/console/services/admin-api/internal/app/admin"
	"github.com/csw/console/services/admin-api/internal/app/dto"
	channeldomain "github.com/csw/console/services/admin-api/internal/domain/channel"
	"github.com/csw/console/services/admin-api/internal/domain/common"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

const (
	defaultRange = "7d"
	defaultTopN  = 5
	maxTopN      = 20
)

type SummaryService interface {
	Summary(ctx context.Context, params dto.DashboardSummaryParams) (dto.DashboardSummary, error)
}

type QueryService struct {
	pool *pgxpool.Pool
}

func NewQueryService(pool *pgxpool.Pool) *QueryService {
	return &QueryService{pool: pool}
}

func (s *QueryService) Summary(ctx context.Context, params dto.DashboardSummaryParams) (dto.DashboardSummary, error) {
	ac, ok := adminapp.AuthContextFrom(ctx)
	if !ok {
		return dto.DashboardSummary{}, validationErr("未认证上下文")
	}
	normalized, window, err := normalizeSummaryParams(params)
	if err != nil {
		return dto.DashboardSummary{}, err
	}

	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly})
	if err != nil {
		return dto.DashboardSummary{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	generatedAt, err := loadGeneratedAt(ctx, tx)
	if err != nil {
		return dto.DashboardSummary{}, err
	}
	timeRange := dto.DashboardTimeRange{
		Range: normalized.Range,
		Since: generatedAt.Add(-window),
		Until: generatedAt,
	}

	env := string(ac.Environment)
	out := dto.DashboardSummary{
		Environment: env,
		GeneratedAt: generatedAt,
		TimeRange:   timeRange,
		FXReview: dto.DashboardFXReviewMetric{
			Permitted: false,
			EnvScoped: false,
			Link: dto.DashboardMetricLink{
				Route: "/cashier",
				Query: map[string]any{"tab": "fx-review", "status": "pending_review"},
			},
			TopItems: []dto.DashboardFXReviewItem{},
		},
		ConfigIssues: dto.DashboardConfigIssuesMetric{
			Permitted: false,
			EnvScoped: true,
			BySource:  defaultSourceCounts(),
			Link: dto.DashboardMetricLink{
				Route: "/games",
				Query: map[string]any{"configStatus": "invalid"},
			},
			TopItems: []dto.DashboardConfigIssueItem{},
		},
		RecentSyncJobs: dto.DashboardRecentSyncMetric{
			Permitted: false,
			EnvScoped: true,
			Window:    timeRange,
			ByStatus:  dto.DashboardSyncJobStatus{},
			Link: dto.DashboardMetricLink{
				Route: "/games",
				Query: map[string]any{"tab": "sync-history", "targetEnv": env},
			},
			TopItems: []dto.DashboardSyncJobItem{},
		},
		PendingSnapshots: dto.DashboardPendingSnapMetric{
			Permitted: false,
			EnvScoped: true,
			Link: dto.DashboardMetricLink{
				Route: "/games",
				Query: map[string]any{"tab": "snapshots", "status": "draft"},
			},
			TopItems: []dto.DashboardSnapshotTopItem{},
		},
		ChannelInstanceIssues: dto.DashboardChannelIssueMetric{
			Permitted: false,
			EnvScoped: true,
			Link: dto.DashboardMetricLink{
				Route: "/games",
				Query: map[string]any{"tab": "channels", "issue": "hidden,incompatible"},
			},
			TopItems: []dto.DashboardChannelIssueTopItem{},
		},
	}

	if ac.HasPermission("cashier.read") {
		metric, mErr := s.loadFXReview(ctx, tx, normalized.WithTopItems, normalized.TopN)
		if mErr != nil {
			return dto.DashboardSummary{}, mErr
		}
		out.FXReview = metric
		out.FXReview.Permitted = true
	}
	if ac.HasPermission("channel.read") && ac.HasPermission("game.read") {
		metric, mErr := s.loadConfigIssues(ctx, tx, normalized.WithTopItems, normalized.TopN)
		if mErr != nil {
			return dto.DashboardSummary{}, mErr
		}
		out.ConfigIssues = metric
		out.ConfigIssues.Permitted = true
	}
	if ac.HasPermission("channel.read") {
		channelMetric, cErr := s.loadChannelIssues(ctx, tx, normalized.WithTopItems, normalized.TopN)
		if cErr != nil {
			return dto.DashboardSummary{}, cErr
		}
		out.ChannelInstanceIssues = channelMetric
		out.ChannelInstanceIssues.Permitted = true
	}
	if ac.HasPermission("sync.preview") {
		metric, mErr := s.loadRecentSyncJobs(ctx, tx, env, timeRange, normalized.WithTopItems, normalized.TopN)
		if mErr != nil {
			return dto.DashboardSummary{}, mErr
		}
		out.RecentSyncJobs = metric
		out.RecentSyncJobs.Permitted = true
	}
	if ac.HasPermission("snapshot.read") {
		metric, mErr := s.loadPendingSnapshots(ctx, tx, normalized.WithTopItems, normalized.TopN)
		if mErr != nil {
			return dto.DashboardSummary{}, mErr
		}
		out.PendingSnapshots = metric
		out.PendingSnapshots.Permitted = true
	}

	if err := tx.Commit(ctx); err != nil {
		return dto.DashboardSummary{}, err
	}
	return out, nil
}

func normalizeSummaryParams(in dto.DashboardSummaryParams) (dto.DashboardSummaryParams, time.Duration, error) {
	out := in
	out.Range = strings.TrimSpace(out.Range)
	if out.Range == "" {
		out.Range = defaultRange
	}
	window, ok := rangeToDuration(out.Range)
	if !ok {
		return dto.DashboardSummaryParams{}, 0, validationErr("range 非法，仅支持 24h/7d/30d/90d")
	}
	if out.TopN <= 0 {
		out.TopN = defaultTopN
	} else if out.TopN > maxTopN {
		return dto.DashboardSummaryParams{}, 0, validationErr("topN 超出范围，允许 1..20")
	}
	return out, window, nil
}

func rangeToDuration(input string) (time.Duration, bool) {
	switch strings.TrimSpace(strings.ToLower(input)) {
	case "24h":
		return 24 * time.Hour, true
	case "7d":
		return 7 * 24 * time.Hour, true
	case "30d":
		return 30 * 24 * time.Hour, true
	case "90d":
		return 90 * 24 * time.Hour, true
	default:
		return 0, false
	}
}

func loadGeneratedAt(ctx context.Context, tx pgx.Tx) (time.Time, error) {
	var ts time.Time
	err := tx.QueryRow(ctx, `SELECT NOW()`).Scan(&ts)
	return ts.UTC(), err
}

func defaultSourceCounts() []dto.DashboardConfigIssueCount {
	return []dto.DashboardConfigIssueCount{
		{Source: "account_auth", InvalidCount: 0},
		{Source: "channel_login", InvalidCount: 0},
		{Source: "channel_iap", InvalidCount: 0},
		{Source: "package_iap_override", InvalidCount: 0},
		{Source: "plugin_config", InvalidCount: 0},
		{Source: "package_plugin_override", InvalidCount: 0},
	}
}

func (s *QueryService) loadFXReview(ctx context.Context, tx pgx.Tx, withTopItems bool, topN int) (dto.DashboardFXReviewMetric, error) {
	out := dto.DashboardFXReviewMetric{
		Permitted: false,
		EnvScoped: false,
		Link: dto.DashboardMetricLink{
			Route: "/cashier",
			Query: map[string]any{"tab": "fx-review", "status": "pending_review"},
		},
		TopItems: []dto.DashboardFXReviewItem{},
	}
	if err := tx.QueryRow(ctx, `SELECT COUNT(*) FROM platform.cashier_fx_sync_runs WHERE status='pending_review'`).Scan(&out.PendingReviewCount); err != nil {
		return dto.DashboardFXReviewMetric{}, err
	}
	if !withTopItems {
		return out, nil
	}
	rows, err := tx.Query(ctx, `
SELECT r.id, t.template_id, t.template_name, r.triggered_at
FROM platform.cashier_fx_sync_runs r
JOIN platform.cashier_price_templates t ON t.id=r.template_id_ref
WHERE r.status='pending_review'
ORDER BY r.triggered_at DESC, r.id DESC
LIMIT $1`, topN)
	if err != nil {
		return dto.DashboardFXReviewMetric{}, err
	}
	defer rows.Close()
	for rows.Next() {
		var item dto.DashboardFXReviewItem
		if err := rows.Scan(&item.RunID, &item.TemplateID, &item.TemplateName, &item.TriggeredAt); err != nil {
			return dto.DashboardFXReviewMetric{}, err
		}
		out.TopItems = append(out.TopItems, item)
	}
	return out, rows.Err()
}

func (s *QueryService) loadConfigIssues(ctx context.Context, tx pgx.Tx, withTopItems bool, topN int) (dto.DashboardConfigIssuesMetric, error) {
	out := dto.DashboardConfigIssuesMetric{
		Permitted: false,
		EnvScoped: true,
		BySource:  defaultSourceCounts(),
		Link: dto.DashboardMetricLink{
			Route: "/games",
			Query: map[string]any{"configStatus": "invalid"},
		},
		TopItems: []dto.DashboardConfigIssueItem{},
	}
	queries := map[string]string{
		"account_auth":            `SELECT COUNT(*) FROM game_account_auth_configs WHERE config_status='invalid'`,
		"channel_login":           `SELECT COUNT(*) FROM game_channel_login_configs WHERE config_status='invalid'`,
		"channel_iap":             `SELECT COUNT(*) FROM game_channel_iap_configs WHERE config_status='invalid'`,
		"package_iap_override":    `SELECT COUNT(*) FROM channel_package_iap_overrides WHERE config_status='invalid'`,
		"plugin_config":           `SELECT COUNT(*) FROM game_channel_plugin_configs WHERE config_status='invalid'`,
		"package_plugin_override": `SELECT COUNT(*) FROM channel_package_plugin_overrides WHERE config_status='invalid'`,
	}
	bySource := map[string]int64{}
	for source, sql := range queries {
		var count int64
		if err := tx.QueryRow(ctx, sql).Scan(&count); err != nil {
			return dto.DashboardConfigIssuesMetric{}, err
		}
		bySource[source] = count
		out.InvalidTotal += count
	}
	for i := range out.BySource {
		out.BySource[i].InvalidCount = bySource[out.BySource[i].Source]
	}
	if !withTopItems {
		return out, nil
	}

	rows, err := tx.Query(ctx, `
SELECT source, game_id_ref, target, COALESCE(last_check_message,'')
FROM (
  SELECT 'account_auth'::text AS source, a.game_id_ref, COALESCE(t.auth_type_id,'') AS target, a.last_check_message, a.last_check_at
  FROM game_account_auth_configs a
  LEFT JOIN platform.account_auth_types t ON t.id=a.auth_type_id_ref
  WHERE a.config_status='invalid'
  UNION ALL
  SELECT 'channel_login', gc.game_id_ref, c.channel_id, l.last_check_message, l.last_check_at
  FROM game_channel_login_configs l
  JOIN game_channels gc ON gc.id=l.game_channel_id_ref
  JOIN platform.channels c ON c.id=gc.channel_id_ref
  WHERE l.config_status='invalid'
  UNION ALL
  SELECT 'channel_iap', gc.game_id_ref, c.channel_id, i.last_check_message, i.last_check_at
  FROM game_channel_iap_configs i
  JOIN game_channels gc ON gc.id=i.game_channel_id_ref
  JOIN platform.channels c ON c.id=gc.channel_id_ref
  WHERE i.config_status='invalid'
  UNION ALL
  SELECT 'package_iap_override', gc.game_id_ref, CONCAT(c.channel_id, '/', cp.package_code), i.last_check_message, i.last_check_at
  FROM channel_package_iap_overrides i
  JOIN channel_packages cp ON cp.id=i.package_id_ref
  JOIN game_channels gc ON gc.id=cp.game_channel_id_ref
  JOIN platform.channels c ON c.id=gc.channel_id_ref
  WHERE i.config_status='invalid'
  UNION ALL
  SELECT 'plugin_config', gc.game_id_ref, c.channel_id, p.last_check_message, p.last_check_at
  FROM game_channel_plugin_configs p
  JOIN game_channels gc ON gc.id=p.game_channel_id_ref
  JOIN platform.channels c ON c.id=gc.channel_id_ref
  WHERE p.config_status='invalid'
  UNION ALL
  SELECT 'package_plugin_override', gc.game_id_ref, CONCAT(c.channel_id, '/', cp.package_code), p.last_check_message, p.last_check_at
  FROM channel_package_plugin_overrides p
  JOIN channel_packages cp ON cp.id=p.package_id_ref
  JOIN game_channels gc ON gc.id=cp.game_channel_id_ref
  JOIN platform.channels c ON c.id=gc.channel_id_ref
  WHERE p.config_status='invalid'
) t
ORDER BY last_check_at DESC NULLS LAST
LIMIT $1`, topN)
	if err != nil {
		return dto.DashboardConfigIssuesMetric{}, err
	}
	defer rows.Close()

	type raw struct {
		source    string
		gameIDRef int64
		target    string
		message   string
	}
	rawItems := make([]raw, 0, topN)
	ids := make([]int64, 0, topN)
	for rows.Next() {
		var r raw
		if err := rows.Scan(&r.source, &r.gameIDRef, &r.target, &r.message); err != nil {
			return dto.DashboardConfigIssuesMetric{}, err
		}
		rawItems = append(rawItems, r)
		ids = append(ids, r.gameIDRef)
	}
	if err := rows.Err(); err != nil {
		return dto.DashboardConfigIssuesMetric{}, err
	}
	meta, err := loadGameMetaByRowIDs(ctx, tx, ids)
	if err != nil {
		return dto.DashboardConfigIssuesMetric{}, err
	}
	for _, r := range rawItems {
		item := dto.DashboardConfigIssueItem{
			Source:           r.source,
			Target:           r.target,
			LastCheckMessage: r.message,
		}
		if m, ok := meta[r.gameIDRef]; ok {
			item.GameID = m.GameID
			item.GameName = m.Name
		}
		out.TopItems = append(out.TopItems, item)
	}
	return out, nil
}

func (s *QueryService) loadRecentSyncJobs(ctx context.Context, tx pgx.Tx, env string, timeRange dto.DashboardTimeRange, withTopItems bool, topN int) (dto.DashboardRecentSyncMetric, error) {
	out := dto.DashboardRecentSyncMetric{
		Permitted: false,
		EnvScoped: true,
		Window:    timeRange,
		ByStatus:  dto.DashboardSyncJobStatus{},
		Link: dto.DashboardMetricLink{
			Route: "/games",
			Query: map[string]any{"tab": "sync-history", "targetEnv": env},
		},
		TopItems: []dto.DashboardSyncJobItem{},
	}
	rows, err := tx.Query(ctx, `
SELECT status, COUNT(*)
FROM platform.sync_jobs
WHERE target_env=$1 AND created_at >= $2
GROUP BY status`, env, timeRange.Since)
	if err != nil {
		return dto.DashboardRecentSyncMetric{}, err
	}
	defer rows.Close()
	for rows.Next() {
		var status string
		var count int64
		if err := rows.Scan(&status, &count); err != nil {
			return dto.DashboardRecentSyncMetric{}, err
		}
		switch strings.ToLower(status) {
		case "previewed":
			out.ByStatus.Previewed = count
		case "succeeded":
			out.ByStatus.Succeeded = count
		case "failed":
			out.ByStatus.Failed = count
		}
	}
	if err := rows.Err(); err != nil {
		return dto.DashboardRecentSyncMetric{}, err
	}
	out.Total = out.ByStatus.Previewed + out.ByStatus.Succeeded + out.ByStatus.Failed
	var lastFailedAt *time.Time
	if err := tx.QueryRow(ctx, `
SELECT MAX(COALESCE(executed_at, created_at))
FROM platform.sync_jobs
WHERE target_env=$1 AND status='failed' AND created_at >= $2`, env, timeRange.Since).Scan(&lastFailedAt); err != nil {
		return dto.DashboardRecentSyncMetric{}, err
	}
	out.LastFailedAt = lastFailedAt

	if !withTopItems {
		return out, nil
	}
	topRows, err := tx.Query(ctx, `
SELECT id, game_id_ref, status, COALESCE(executed_at, created_at) AS executed_at
FROM platform.sync_jobs
WHERE target_env=$1 AND created_at >= $2
ORDER BY COALESCE(executed_at, created_at) DESC, id DESC
LIMIT $3`, env, timeRange.Since, topN)
	if err != nil {
		return dto.DashboardRecentSyncMetric{}, err
	}
	defer topRows.Close()
	type raw struct {
		jobID      int64
		gameID     string
		status     string
		executedAt time.Time
	}
	rawItems := make([]raw, 0, topN)
	gameIDs := make([]string, 0, topN)
	for topRows.Next() {
		var item raw
		if err := topRows.Scan(&item.jobID, &item.gameID, &item.status, &item.executedAt); err != nil {
			return dto.DashboardRecentSyncMetric{}, err
		}
		rawItems = append(rawItems, item)
		gameIDs = append(gameIDs, item.gameID)
	}
	if err := topRows.Err(); err != nil {
		return dto.DashboardRecentSyncMetric{}, err
	}
	nameMap, err := loadGameNameByGameIDs(ctx, tx, gameIDs)
	if err != nil {
		return dto.DashboardRecentSyncMetric{}, err
	}
	for _, item := range rawItems {
		out.TopItems = append(out.TopItems, dto.DashboardSyncJobItem{
			JobID:      item.jobID,
			GameID:     item.gameID,
			GameName:   nameMap[item.gameID],
			Status:     item.status,
			ExecutedAt: item.executedAt,
		})
	}
	return out, nil
}

func (s *QueryService) loadPendingSnapshots(ctx context.Context, tx pgx.Tx, withTopItems bool, topN int) (dto.DashboardPendingSnapMetric, error) {
	out := dto.DashboardPendingSnapMetric{
		Permitted: false,
		EnvScoped: true,
		Link: dto.DashboardMetricLink{
			Route: "/games",
			Query: map[string]any{"tab": "snapshots", "status": "draft"},
		},
		TopItems: []dto.DashboardSnapshotTopItem{},
	}
	if err := tx.QueryRow(ctx, `SELECT COUNT(*) FROM game_config_snapshots WHERE status='draft'`).Scan(&out.DraftCount); err != nil {
		return dto.DashboardPendingSnapMetric{}, err
	}
	if !withTopItems {
		return out, nil
	}
	rows, err := tx.Query(ctx, `
SELECT id, game_id_ref, config_version, generated_at
FROM game_config_snapshots
WHERE status='draft'
ORDER BY generated_at DESC, id DESC
LIMIT $1`, topN)
	if err != nil {
		return dto.DashboardPendingSnapMetric{}, err
	}
	defer rows.Close()
	type raw struct {
		snapshotID    int64
		gameIDRef     int64
		configVersion string
		generatedAt   time.Time
	}
	rawItems := make([]raw, 0, topN)
	gameIDs := make([]int64, 0, topN)
	for rows.Next() {
		var item raw
		if err := rows.Scan(&item.snapshotID, &item.gameIDRef, &item.configVersion, &item.generatedAt); err != nil {
			return dto.DashboardPendingSnapMetric{}, err
		}
		rawItems = append(rawItems, item)
		gameIDs = append(gameIDs, item.gameIDRef)
	}
	if err := rows.Err(); err != nil {
		return dto.DashboardPendingSnapMetric{}, err
	}
	meta, err := loadGameMetaByRowIDs(ctx, tx, gameIDs)
	if err != nil {
		return dto.DashboardPendingSnapMetric{}, err
	}
	for _, item := range rawItems {
		top := dto.DashboardSnapshotTopItem{
			SnapshotID:    item.snapshotID,
			ConfigVersion: item.configVersion,
			GeneratedAt:   item.generatedAt,
		}
		if m, ok := meta[item.gameIDRef]; ok {
			top.GameID = m.GameID
			top.GameName = m.Name
		}
		out.TopItems = append(out.TopItems, top)
	}
	return out, nil
}

func (s *QueryService) loadChannelIssues(ctx context.Context, tx pgx.Tx, withTopItems bool, topN int) (dto.DashboardChannelIssueMetric, error) {
	out := dto.DashboardChannelIssueMetric{
		Permitted: false,
		EnvScoped: true,
		Link: dto.DashboardMetricLink{
			Route: "/games",
			Query: map[string]any{"tab": "channels", "issue": "hidden,incompatible"},
		},
		TopItems: []dto.DashboardChannelIssueTopItem{},
	}
	if err := tx.QueryRow(ctx, `SELECT COUNT(*) FROM game_channels WHERE hidden=TRUE`).Scan(&out.HiddenCount); err != nil {
		return dto.DashboardChannelIssueMetric{}, err
	}
	if err := tx.QueryRow(ctx, `
SELECT COUNT(*)
FROM game_channels gc
JOIN platform.channels c ON c.id=gc.channel_id_ref
WHERE (gc.market_code='CN' AND c.region <> 'domestic')
   OR (gc.market_code <> 'CN' AND c.region <> 'overseas')`).Scan(&out.IncompatibleCount); err != nil {
		return dto.DashboardChannelIssueMetric{}, err
	}
	if !withTopItems {
		return out, nil
	}
	rows, err := tx.Query(ctx, `
SELECT gc.id, gc.game_id_ref, c.channel_id, c.region, gc.market_code, gc.hidden, gc.updated_at
FROM game_channels gc
JOIN platform.channels c ON c.id=gc.channel_id_ref
WHERE gc.hidden=TRUE
   OR (gc.market_code='CN' AND c.region <> 'domestic')
   OR (gc.market_code <> 'CN' AND c.region <> 'overseas')
ORDER BY gc.updated_at DESC, gc.id DESC
LIMIT $1`, topN*2)
	if err != nil {
		return dto.DashboardChannelIssueMetric{}, err
	}
	defer rows.Close()
	type raw struct {
		gameChannelID int64
		gameIDRef     int64
		channelID     string
		region        string
		marketCode    string
		hidden        bool
	}
	rawItems := make([]raw, 0, topN*2)
	gameIDs := make([]int64, 0, topN*2)
	for rows.Next() {
		var item raw
		var _updatedAt time.Time
		if err := rows.Scan(&item.gameChannelID, &item.gameIDRef, &item.channelID, &item.region, &item.marketCode, &item.hidden, &_updatedAt); err != nil {
			return dto.DashboardChannelIssueMetric{}, err
		}
		rawItems = append(rawItems, item)
		gameIDs = append(gameIDs, item.gameIDRef)
	}
	if err := rows.Err(); err != nil {
		return dto.DashboardChannelIssueMetric{}, err
	}
	meta, err := loadGameMetaByRowIDs(ctx, tx, gameIDs)
	if err != nil {
		return dto.DashboardChannelIssueMetric{}, err
	}
	for _, item := range rawItems {
		if len(out.TopItems) >= topN {
			break
		}
		base := dto.DashboardChannelIssueTopItem{
			GameChannelID: item.gameChannelID,
			ChannelID:     item.channelID,
			MarketCode:    strings.ToUpper(item.marketCode),
		}
		if m, ok := meta[item.gameIDRef]; ok {
			base.GameID = m.GameID
			base.GameName = m.Name
		}
		if item.hidden {
			top := base
			top.Issue = "hidden"
			out.TopItems = append(out.TopItems, top)
			if len(out.TopItems) >= topN {
				break
			}
		}
		if !channeldomain.IsCompatible(common.Market(item.marketCode), channeldomain.ChannelRegion(item.region)) {
			top := base
			top.Issue = "incompatible"
			out.TopItems = append(out.TopItems, top)
		}
	}
	if len(out.TopItems) > topN {
		out.TopItems = out.TopItems[:topN]
	}
	return out, nil
}

type gameMeta struct {
	GameID string
	Name   string
}

func loadGameMetaByRowIDs(ctx context.Context, tx pgx.Tx, ids []int64) (map[int64]gameMeta, error) {
	ids = uniqueInt64(ids)
	out := make(map[int64]gameMeta, len(ids))
	if len(ids) == 0 {
		return out, nil
	}
	rows, err := tx.Query(ctx, `SELECT id, game_id, name FROM games WHERE id = ANY($1)`, ids)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var id int64
		var meta gameMeta
		if err := rows.Scan(&id, &meta.GameID, &meta.Name); err != nil {
			return nil, err
		}
		out[id] = meta
	}
	return out, rows.Err()
}

func loadGameNameByGameIDs(ctx context.Context, tx pgx.Tx, gameIDs []string) (map[string]string, error) {
	gameIDs = uniqueString(gameIDs)
	out := make(map[string]string, len(gameIDs))
	if len(gameIDs) == 0 {
		return out, nil
	}
	rows, err := tx.Query(ctx, `SELECT game_id, name FROM games WHERE game_id = ANY($1)`, gameIDs)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var gameID, name string
		if err := rows.Scan(&gameID, &name); err != nil {
			return nil, err
		}
		out[gameID] = name
	}
	return out, rows.Err()
}

func uniqueInt64(input []int64) []int64 {
	if len(input) == 0 {
		return input
	}
	seen := make(map[int64]struct{}, len(input))
	out := make([]int64, 0, len(input))
	for _, item := range input {
		if _, ok := seen[item]; ok {
			continue
		}
		seen[item] = struct{}{}
		out = append(out, item)
	}
	slices.Sort(out)
	return out
}

func uniqueString(input []string) []string {
	if len(input) == 0 {
		return input
	}
	seen := make(map[string]struct{}, len(input))
	out := make([]string, 0, len(input))
	for _, item := range input {
		item = strings.TrimSpace(item)
		if item == "" {
			continue
		}
		if _, ok := seen[item]; ok {
			continue
		}
		seen[item] = struct{}{}
		out = append(out, item)
	}
	slices.Sort(out)
	return out
}

