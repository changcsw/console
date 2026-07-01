package snapshot

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strconv"
	"strings"
	"time"

	adminapp "github.com/csw/console/services/admin-api/internal/app/admin"
	"github.com/csw/console/services/admin-api/internal/domain/common"
	domainpayment "github.com/csw/console/services/admin-api/internal/domain/payment"
	domainsnapshot "github.com/csw/console/services/admin-api/internal/domain/snapshot"
)

type service struct {
	tx      TxManager
	payment PaymentResolver
	audit   AuditSink
	now     func() time.Time
}

func NewService(tx TxManager, payment PaymentResolver, audit AuditSink, now func() time.Time) Service {
	if now == nil {
		now = time.Now
	}
	return &service{tx: tx, payment: payment, audit: audit, now: now}
}

func (s *service) Generate(ctx context.Context, gameID string) (GenerateResult, error) {
	gameID = strings.TrimSpace(gameID)
	if gameID == "" {
		return GenerateResult{}, validationErr("gameId 必填")
	}

	generatedAt := s.now().UTC()
	view, err := s.loadValidView(ctx, gameID, generatedAt)
	if err != nil {
		return GenerateResult{}, err
	}
	if view.PaymentRoutes == nil {
		view.PaymentRoutes = map[common.Market][]domainsnapshot.ResolvedRoute{}
	}

	runtime := domainsnapshot.BuildRuntimeConfig(view)
	canonical, err := domainsnapshot.CanonicalJSON(runtime)
	if err != nil {
		return GenerateResult{}, err
	}
	hash := domainsnapshot.HashCanonicalJSON(canonical)
	version := domainsnapshot.BuildConfigVersion(generatedAt, hash)
	fileName := fmt.Sprintf("game_%s_%s.json", gameID, version)

	var configJSON map[string]any
	if err := json.Unmarshal(canonical, &configJSON); err != nil {
		return GenerateResult{}, err
	}

	row, err := s.tx.Repository().CreateSnapshot(ctx, CreateSnapshotInput{
		GameIDRef:           view.GameIDRef,
		ConfigSchemaVersion: domainsnapshot.ConfigSchemaVersion,
		ConfigVersion:       version,
		ConfigJSON:          configJSON,
		FileName:            fileName,
		FileHash:            hash,
		StorageKey:          "",
		GeneratedAt:         generatedAt,
	})
	if err != nil {
		return GenerateResult{}, mapRepoErr(err)
	}

	s.writeAudit(ctx, AuditEntry{
		Action:       "snapshot.generate",
		ResourceType: "game_config_snapshot",
		ResourceID:   strconv.FormatInt(row.ID, 10),
		Detail: map[string]any{
			"gameId":        gameID,
			"snapshotId":    row.ID,
			"configVersion": row.ConfigVersion,
			"fileHash":      row.FileHash,
		},
	})

	return GenerateResult{
		ID:            row.ID,
		ConfigVersion: row.ConfigVersion,
		FileHash:      row.FileHash,
		Status:        string(row.Status),
		GeneratedAt:   row.GeneratedAt,
	}, nil
}

func (s *service) List(ctx context.Context, gameID string, filter ListFilter) (SnapshotList, error) {
	gameID = strings.TrimSpace(gameID)
	if gameID == "" {
		return SnapshotList{}, validationErr("gameId 必填")
	}
	if filter.Page <= 0 {
		filter.Page = 1
	}
	if filter.PageSize <= 0 {
		filter.PageSize = 20
	}
	if filter.PageSize > 100 {
		filter.PageSize = 100
	}
	rows, total, err := s.tx.Repository().ListSnapshots(ctx, gameID, filter)
	if err != nil {
		return SnapshotList{}, mapRepoErr(err)
	}
	items := make([]SnapshotItem, 0, len(rows))
	for _, row := range rows {
		items = append(items, toSnapshotItem(row))
	}
	return SnapshotList{
		Items:    items,
		Page:     filter.Page,
		PageSize: filter.PageSize,
		Total:    total,
	}, nil
}

func (s *service) Publish(ctx context.Context, snapshotID int64) (SnapshotItem, error) {
	if snapshotID <= 0 {
		return SnapshotItem{}, validationErr("snapshotId 非法")
	}
	var out domainsnapshot.ConfigSnapshot
	err := s.tx.InTx(ctx, func(repo Repository) error {
		row, err := repo.GetSnapshot(ctx, snapshotID)
		if err != nil {
			return mapRepoErr(err)
		}
		if row.Status != domainsnapshot.StatusDraft {
			return versionStateErr("仅 draft 可发布")
		}
		published, err := repo.PublishSnapshot(ctx, snapshotID, s.now().UTC())
		if err != nil {
			mapped := mapRepoErr(err)
			var appErr *Error
			if errors.As(mapped, &appErr) && appErr.Code == CodeNotFound {
				if row2, getErr := repo.GetSnapshot(ctx, snapshotID); getErr == nil && row2.Status != domainsnapshot.StatusDraft {
					return versionStateErr("仅 draft 可发布")
				}
			}
			return mapped
		}
		out = published
		return nil
	})
	if err != nil {
		return SnapshotItem{}, err
	}

	s.writeAudit(ctx, AuditEntry{
		Action:       "snapshot.publish",
		ResourceType: "game_config_snapshot",
		ResourceID:   strconv.FormatInt(out.ID, 10),
		Detail: map[string]any{
			"snapshotId":    out.ID,
			"configVersion": out.ConfigVersion,
		},
	})
	return toSnapshotItem(out), nil
}

func (s *service) Download(ctx context.Context, snapshotID int64) (DownloadResult, error) {
	if snapshotID <= 0 {
		return DownloadResult{}, validationErr("snapshotId 非法")
	}
	row, err := s.tx.Repository().GetSnapshot(ctx, snapshotID)
	if err != nil {
		return DownloadResult{}, mapRepoErr(err)
	}
	body, err := domainsnapshot.CanonicalJSON(row.ConfigJSON)
	if err != nil {
		return DownloadResult{}, err
	}
	return DownloadResult{FileName: row.FileName, Body: body}, nil
}

func (s *service) loadValidView(ctx context.Context, gameID string, generatedAt time.Time) (domainsnapshot.ValidDataView, error) {
	gameIDRef, err := s.tx.Repository().ResolveGameRowID(ctx, gameID)
	if err != nil {
		return domainsnapshot.ValidDataView{}, mapRepoErr(err)
	}
	view, payWays, err := s.tx.Repository().LoadValidData(ctx, gameIDRef, gameID, generatedAt)
	if err != nil {
		return domainsnapshot.ValidDataView{}, mapRepoErr(err)
	}
	slices.Sort(payWays)

	routesByMarket := make(map[common.Market][]domainsnapshot.ResolvedRoute)
	for _, market := range []common.Market{
		common.MarketGlobal, common.MarketJP, common.MarketKR, common.MarketSEA, common.MarketHMT, common.MarketCN,
	} {
		routes := make([]domainsnapshot.ResolvedRoute, 0, len(payWays))
		for _, payWay := range payWays {
			if s.payment == nil {
				continue
			}
			target, err := s.payment.ResolveRoute(ctx, gameID, domainpayment.MatchInput{
				PayWay: payWay,
				Market: string(market),
			})
			if err != nil {
				if isNotFoundRoute(err) {
					continue
				}
				return domainsnapshot.ValidDataView{}, err
			}
			routes = append(routes, domainsnapshot.ResolvedRoute{
				PayWay:          payWay,
				Provider:        target.Provider,
				MerchantAccount: target.MerchantAccount,
			})
		}
		routesByMarket[market] = routes
	}
	view.PaymentRoutes = routesByMarket
	return view, nil
}

func toSnapshotItem(row domainsnapshot.ConfigSnapshot) SnapshotItem {
	return SnapshotItem{
		ID:            row.ID,
		ConfigVersion: row.ConfigVersion,
		Status:        string(row.Status),
		FileHash:      row.FileHash,
		GeneratedAt:   row.GeneratedAt,
		PublishedAt:   row.PublishedAt,
	}
}

func (s *service) writeAudit(ctx context.Context, entry AuditEntry) {
	if s.audit == nil {
		return
	}
	if ac, ok := adminapp.AuthContextFrom(ctx); ok {
		entry.ActorID = ac.UserID
	}
	_ = s.audit.Write(ctx, entry)
}

func mapRepoErr(err error) error {
	var appErr *Error
	if errors.As(err, &appErr) {
		return appErr
	}
	if errors.Is(err, adminapp.ErrNotFound) {
		return notFoundErr("资源不存在")
	}
	if errors.Is(err, adminapp.ErrConflict) {
		return conflictErr("资源冲突")
	}
	return err
}
