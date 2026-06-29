package cashier

import (
	"context"
	"errors"
	"fmt"
	"math/big"
	"strconv"
	"strings"
	"time"

	adminapp "github.com/csw/console/services/admin-api/internal/app/admin"
	"github.com/csw/console/services/admin-api/internal/app/command"
	domaincashier "github.com/csw/console/services/admin-api/internal/domain/cashier"
	"github.com/csw/console/services/admin-api/internal/domain/common"
)

type Service struct {
	tx    TxManager
	audit AuditSink
	now   func() time.Time
}

func NewService(tx TxManager, audit AuditSink, now func() time.Time) *Service {
	if now == nil {
		now = time.Now
	}
	return &Service{tx: tx, audit: audit, now: now}
}

func (s *Service) ListTemplates(ctx context.Context, page, pageSize int) ([]domaincashier.PriceTemplate, int, error) {
	page, pageSize = normalizePage(page, pageSize)
	return s.tx.Repository().ListTemplates(ctx, page, pageSize)
}

func (s *Service) CreateTemplate(ctx context.Context, input CreateTemplateInput) (domaincashier.PriceTemplate, error) {
	input.TemplateID = strings.TrimSpace(input.TemplateID)
	input.TemplateName = strings.TrimSpace(input.TemplateName)
	if input.TemplateID == "" {
		return domaincashier.PriceTemplate{}, validationErr("templateId 必填", fieldDetail("templateId", "required"))
	}
	if input.TemplateName == "" {
		return domaincashier.PriceTemplate{}, validationErr("templateName 必填", fieldDetail("templateName", "required"))
	}
	if input.FXSyncMode != string(common.FXSyncModeManualConfirm) && input.FXSyncMode != string(common.FXSyncModeAutoApply) {
		return domaincashier.PriceTemplate{}, validationErr("fxSyncMode 非法", fieldDetail("fxSyncMode", "manual_confirm/auto_apply"))
	}
	if input.FXSyncSchedule != "monthly" && input.FXSyncSchedule != "quarterly" {
		return domaincashier.PriceTemplate{}, validationErr("fxSyncSchedule 非法", fieldDetail("fxSyncSchedule", "monthly/quarterly"))
	}
	created, err := s.tx.Repository().CreateTemplate(ctx, domaincashier.PriceTemplate{
		TemplateID:     input.TemplateID,
		TemplateName:   input.TemplateName,
		FXSyncEnabled:  input.FXSyncEnabled,
		FXSyncMode:     input.FXSyncMode,
		FXSyncSchedule: input.FXSyncSchedule,
		Status:         "draft",
	})
	if err != nil {
		return domaincashier.PriceTemplate{}, mapRepoErr(err, "templateId 冲突")
	}
	s.writeAudit(ctx, "cashier.template.create", "cashier_template", strconv.FormatInt(created.ID, 10), map[string]any{
		"templateId": created.TemplateID,
	})
	return created, nil
}

func (s *Service) GetTemplate(ctx context.Context, templateID string) (domaincashier.PriceTemplate, []domaincashier.TemplateVersionRecord, error) {
	tpl, err := s.tx.Repository().GetTemplateByTemplateID(ctx, templateID)
	if err != nil {
		return domaincashier.PriceTemplate{}, nil, mapRepoErr(err, "template 不存在")
	}
	versions, err := s.tx.Repository().ListVersions(ctx, tpl.ID)
	return tpl, versions, err
}

func (s *Service) CreateVersion(ctx context.Context, input CreateVersionInput) (domaincashier.TemplateVersionRecord, error) {
	if input.SourceType == domaincashier.SourceTypeCopyPublished || input.SourceType == domaincashier.SourceTypeCopyArchived {
		if input.SourceVersion <= 0 {
			return domaincashier.TemplateVersionRecord{}, validationErr("copy 时 sourceVersion 必填", fieldDetail("sourceVersion", "required"))
		}
	}
	var created domaincashier.TemplateVersionRecord
	err := s.tx.InTx(ctx, func(repo CashierTemplateRepository) error {
		tpl, err := repo.GetTemplateByTemplateID(ctx, input.TemplateID)
		if err != nil {
			return mapRepoErr(err, "template 不存在")
		}
		sourceType := input.SourceType
		if sourceType == "" {
			sourceType = domaincashier.SourceTypeManual
		}
		nextVersion, err := repo.NextVersion(ctx, tpl.ID)
		if err != nil {
			return err
		}
		if input.SourceVersion > 0 {
			source, err := repo.GetVersionByTemplateAndVersion(ctx, tpl.ID, input.SourceVersion)
			if err != nil {
				return mapRepoErr(err, "sourceVersion 不存在")
			}
			if source.Status != domaincashier.StatusPublished && source.Status != domaincashier.StatusArchived {
				return versionStateErr("仅允许从 published/archived 复制")
			}
			switch source.Status {
			case domaincashier.StatusPublished:
				sourceType = domaincashier.SourceTypeCopyPublished
			case domaincashier.StatusArchived:
				sourceType = domaincashier.SourceTypeCopyArchived
			}
			created, err = repo.CreateVersion(ctx, domaincashier.TemplateVersionRecord{
				TemplateIDRef: tpl.ID,
				Version:       nextVersion,
				SourceType:    sourceType,
				Status:        domaincashier.StatusDraft,
				Checksum:      "",
			})
			if err != nil {
				return mapRepoErr(err, "version 冲突")
			}
			if _, err = repo.CopyRows(ctx, source.ID, created.ID); err != nil {
				return err
			}
			return nil
		}
		created, err = repo.CreateVersion(ctx, domaincashier.TemplateVersionRecord{
			TemplateIDRef: tpl.ID,
			Version:       nextVersion,
			SourceType:    sourceType,
			Status:        domaincashier.StatusDraft,
			Checksum:      "",
		})
		if err != nil {
			return mapRepoErr(err, "version 冲突")
		}
		return nil
	})
	return created, err
}

func (s *Service) CopyToDraft(ctx context.Context, templateID string, sourceVersion int) (domaincashier.TemplateVersion, error) {
	var out domaincashier.TemplateVersion
	err := s.tx.InTx(ctx, func(repo CashierTemplateRepository) error {
		tpl, err := repo.GetTemplateByTemplateID(ctx, templateID)
		if err != nil {
			return mapRepoErr(err, "template 不存在")
		}
		sourceRec, err := repo.GetVersionByTemplateAndVersion(ctx, tpl.ID, sourceVersion)
		if err != nil {
			return mapRepoErr(err, "sourceVersion 不存在")
		}
		if sourceRec.Status != domaincashier.StatusPublished && sourceRec.Status != domaincashier.StatusArchived {
			return versionStateErr("仅允许从 published/archived 复制")
		}
		next, err := repo.NextVersion(ctx, tpl.ID)
		if err != nil {
			return err
		}
		draft := command.BuildDraftFromTemplateVersion(domaincashier.TemplateVersion{
			TemplateID: templateID,
			Version:    sourceVersion,
			Status:     sourceRec.Status,
		}, next)
		created, err := repo.CreateVersion(ctx, domaincashier.TemplateVersionRecord{
			TemplateIDRef: tpl.ID,
			Version:       draft.Version,
			SourceType:    draft.SourceType,
			Status:        domaincashier.StatusDraft,
			Checksum:      "",
		})
		if err != nil {
			return mapRepoErr(err, "version 冲突")
		}
		if _, err := repo.CopyRows(ctx, sourceRec.ID, created.ID); err != nil {
			return err
		}
		out = domaincashier.TemplateVersion{
			TemplateID: templateID,
			Version:    created.Version,
			Status:     created.Status,
			SourceType: draft.SourceType,
		}
		return nil
	})
	return out, err
}

func (s *Service) ListRows(ctx context.Context, templateID string, version int) ([]domaincashier.PriceRow, error) {
	record, err := s.loadVersion(ctx, templateID, version)
	if err != nil {
		return nil, err
	}
	return s.tx.Repository().ListRows(ctx, record.ID)
}

func (s *Service) UpsertRows(ctx context.Context, templateID string, version int, rows []UpsertPriceRowInput) ([]domaincashier.PriceRow, error) {
	var persisted []domaincashier.PriceRow
	err := s.tx.InTx(ctx, func(repo CashierTemplateRepository) error {
		tpl, err := repo.GetTemplateByTemplateID(ctx, templateID)
		if err != nil {
			return mapRepoErr(err, "template 不存在")
		}
		versionRecord, err := repo.GetVersionByTemplateAndVersion(ctx, tpl.ID, version)
		if err != nil {
			return mapRepoErr(err, "version 不存在")
		}
		if versionRecord.Status != domaincashier.StatusDraft {
			return versionStateErr("published 版本只读，需 copy-to-draft")
		}
		normRows := make([]domaincashier.PriceRow, 0, len(rows))
		for idx, row := range rows {
			norm, err := normalizeRow(ctx, repo, row)
			if err != nil {
				var appErr *Error
				if errors.As(err, &appErr) {
					return appErr
				}
				return validationErr(fmt.Sprintf("rows[%d] 非法: %v", idx, err))
			}
			normRows = append(normRows, norm)
		}
		if err := repo.ReplaceRows(ctx, versionRecord.ID, normRows); err != nil {
			return err
		}
		persisted, err = repo.ListRows(ctx, versionRecord.ID)
		return err
	})
	return persisted, err
}

func (s *Service) PublishVersion(ctx context.Context, templateID string, version int) error {
	return s.tx.InTx(ctx, func(repo CashierTemplateRepository) error {
		tpl, err := repo.GetTemplateByTemplateID(ctx, templateID)
		if err != nil {
			return mapRepoErr(err, "template 不存在")
		}
		target, err := repo.GetVersionByTemplateAndVersion(ctx, tpl.ID, version)
		if err != nil {
			return mapRepoErr(err, "version 不存在")
		}
		if !domaincashier.CanTransition(target.Status, domaincashier.StatusPublished) {
			return versionStateErr("仅 draft 可发布")
		}
		now := s.now()
		oldPublished, err := repo.GetPublishedVersion(ctx, tpl.ID)
		if err != nil {
			return err
		}
		if oldPublished != nil && oldPublished.ID != target.ID {
			if !domaincashier.CanTransition(oldPublished.Status, domaincashier.StatusArchived) {
				return versionStateErr("已发布版本状态非法")
			}
			if err := repo.ArchiveVersion(ctx, oldPublished.ID, now); err != nil {
				return err
			}
		}
		if err := repo.PublishVersion(ctx, target.ID, now); err != nil {
			return err
		}
		s.writeAudit(ctx, "cashier.version.publish", "cashier_template_version", strconv.FormatInt(target.ID, 10), map[string]any{
			"templateId": templateID,
			"version":    version,
		})
		return nil
	})
}

func (s *Service) TriggerFXSyncRun(ctx context.Context, templateID string) (FXSyncRunView, error) {
	var view FXSyncRunView
	err := s.tx.InTx(ctx, func(repo CashierTemplateRepository) error {
		tpl, err := repo.GetTemplateByTemplateID(ctx, templateID)
		if err != nil {
			return mapRepoErr(err, "template 不存在")
		}
		nextVersion, err := repo.NextVersion(ctx, tpl.ID)
		if err != nil {
			return err
		}
		candidate, err := repo.CreateVersion(ctx, domaincashier.TemplateVersionRecord{
			TemplateIDRef: tpl.ID,
			Version:       nextVersion,
			SourceType:    domaincashier.SourceTypeFXAuto,
			AutoGenerated: true,
			Status:        domaincashier.StatusDraft,
			Checksum:      "",
		})
		if err != nil {
			return err
		}
		copied := 0
		sourceVersion := 0
		if published, err := repo.GetPublishedVersion(ctx, tpl.ID); err != nil {
			return err
		} else if published != nil {
			sourceVersion = published.Version
			copied, err = repo.CopyRows(ctx, published.ID, candidate.ID)
			if err != nil {
				return err
			}
		}
		run, err := repo.CreateFXSyncRun(ctx, domaincashier.FXSyncRun{
			TemplateIDRef:         tpl.ID,
			CandidateVersionIDRef: candidate.ID,
			Status:                domaincashier.FXRunPendingReview,
			DiffSummaryJSON: map[string]any{
				"sourceVersion":    sourceVersion,
				"candidateVersion": candidate.Version,
				"copiedRows":       copied,
			},
			ReviewNote: "",
		})
		if err != nil {
			return err
		}
		if tpl.FXSyncMode == string(common.FXSyncModeAutoApply) {
			if err := s.approveRunInTx(ctx, repo, run.ID, "auto_apply", 0); err != nil {
				return err
			}
			// auto_apply 下同事务已 approve→applied，重新读取 run 使响应 status 与落库一致（P4）。
			reloaded, err := repo.GetFXSyncRun(ctx, run.ID)
			if err != nil {
				return err
			}
			run = reloaded
		}
		view = FXSyncRunView{Run: run, CandidateVersion: candidate.Version}
		return nil
	})
	return view, err
}

func (s *Service) ListFXSyncRuns(ctx context.Context, templateID string) ([]FXSyncRunView, error) {
	repo := s.tx.Repository()
	tpl, err := repo.GetTemplateByTemplateID(ctx, templateID)
	if err != nil {
		return nil, mapRepoErr(err, "template 不存在")
	}
	runs, err := repo.ListFXSyncRuns(ctx, tpl.ID)
	if err != nil {
		return nil, err
	}
	out := make([]FXSyncRunView, 0, len(runs))
	for _, run := range runs {
		candidateVersion := 0
		if v, err := repo.GetVersionByID(ctx, run.CandidateVersionIDRef); err == nil {
			candidateVersion = v.Version
		}
		out = append(out, FXSyncRunView{Run: run, CandidateVersion: candidateVersion})
	}
	return out, nil
}

func (s *Service) ApproveFXSyncRun(ctx context.Context, runID int64, note string) error {
	reviewer := int64(0)
	if ac, ok := adminapp.AuthContextFrom(ctx); ok {
		reviewer = ac.UserID
	}
	return s.tx.InTx(ctx, func(repo CashierTemplateRepository) error {
		return s.approveRunInTx(ctx, repo, runID, note, reviewer)
	})
}

func (s *Service) IgnoreFXSyncRun(ctx context.Context, runID int64, note string) error {
	reviewer := int64(0)
	if ac, ok := adminapp.AuthContextFrom(ctx); ok {
		reviewer = ac.UserID
	}
	return s.tx.InTx(ctx, func(repo CashierTemplateRepository) error {
		run, err := repo.GetFXSyncRun(ctx, runID)
		if err != nil {
			return mapRepoErr(err, "run 不存在")
		}
		if run.Status != domaincashier.FXRunPendingReview {
			return versionStateErr("仅 pending_review 可忽略")
		}
		return repo.UpdateFXSyncRunReview(ctx, run.ID, domaincashier.FXRunIgnored, reviewer, s.now(), note)
	})
}

func (s *Service) approveRunInTx(ctx context.Context, repo CashierTemplateRepository, runID int64, note string, reviewer int64) error {
	run, err := repo.GetFXSyncRun(ctx, runID)
	if err != nil {
		return mapRepoErr(err, "run 不存在")
	}
	if run.Status != domaincashier.FXRunPendingReview {
		return versionStateErr("仅 pending_review 可审核")
	}
	candidate, err := repo.GetVersionByID(ctx, run.CandidateVersionIDRef)
	if err != nil {
		return mapRepoErr(err, "candidate version 不存在")
	}
	template, err := repo.GetTemplateByID(ctx, candidate.TemplateIDRef)
	if err != nil {
		return mapRepoErr(err, "template 不存在")
	}

	now := s.now()
	oldPublished, err := repo.GetPublishedVersion(ctx, candidate.TemplateIDRef)
	if err != nil {
		return err
	}
	if oldPublished != nil && oldPublished.ID != candidate.ID {
		if !domaincashier.CanTransition(oldPublished.Status, domaincashier.StatusArchived) {
			return versionStateErr("已发布版本状态非法")
		}
		if err := repo.ArchiveVersion(ctx, oldPublished.ID, now); err != nil {
			return err
		}
	}
	if !domaincashier.CanTransition(candidate.Status, domaincashier.StatusPublished) {
		return versionStateErr("候选版本状态非法")
	}
	if err := repo.PublishVersion(ctx, candidate.ID, now); err != nil {
		return err
	}
	if err := repo.UpdateFXSyncRunReview(ctx, run.ID, domaincashier.FXRunApplied, reviewer, now, note); err != nil {
		return err
	}
	s.writeAudit(ctx, "fx.approve", "cashier_fx_sync_run", strconv.FormatInt(run.ID, 10), map[string]any{
		"templateId": template.TemplateID,
		"runId":      run.ID,
		"note":       note,
	})
	return nil
}

func (s *Service) loadVersion(ctx context.Context, templateID string, version int) (domaincashier.TemplateVersionRecord, error) {
	tpl, err := s.tx.Repository().GetTemplateByTemplateID(ctx, templateID)
	if err != nil {
		return domaincashier.TemplateVersionRecord{}, mapRepoErr(err, "template 不存在")
	}
	record, err := s.tx.Repository().GetVersionByTemplateAndVersion(ctx, tpl.ID, version)
	if err != nil {
		return domaincashier.TemplateVersionRecord{}, mapRepoErr(err, "version 不存在")
	}
	return record, nil
}

func normalizeRow(ctx context.Context, repo CashierTemplateRepository, in UpsertPriceRowInput) (domaincashier.PriceRow, error) {
	if strings.TrimSpace(in.CountryCode) == "" || strings.TrimSpace(in.Currency) == "" || strings.TrimSpace(in.PriceID) == "" {
		return domaincashier.PriceRow{}, fmt.Errorf("countryCode/currency/priceId 必填")
	}
	spec, err := repo.GetCurrencySpec(ctx, strings.ToUpper(strings.TrimSpace(in.Currency)))
	if err != nil {
		return domaincashier.PriceRow{}, currencyErr("currency not supported")
	}
	preTax, err := common.NormalizeAmountToMinor(in.PreTaxAmount, spec)
	if err != nil {
		return domaincashier.PriceRow{}, validationErr(err.Error())
	}
	taxRate := strings.TrimSpace(in.TaxRate)
	if taxRate == "" {
		taxRate = "0"
	}
	taxMinor, err := calcTaxAmount(preTax, taxRate)
	if err != nil {
		return domaincashier.PriceRow{}, err
	}
	return domaincashier.PriceRow{
		CountryCode:         strings.ToUpper(strings.TrimSpace(in.CountryCode)),
		RegionCode:          defaultString(strings.TrimSpace(in.RegionCode), "*"),
		Currency:            strings.ToUpper(strings.TrimSpace(in.Currency)),
		PriceID:             strings.TrimSpace(in.PriceID),
		PreTaxAmountMinor:   preTax,
		TaxRate:             taxRate,
		TaxAmountMinor:      taxMinor,
		AfterTaxAmountMinor: preTax + taxMinor,
		EffectiveAt:         in.EffectiveAt.UTC().Format(time.RFC3339),
	}, nil
}

func calcTaxAmount(preTaxMinor int64, taxRate string) (int64, error) {
	r := new(big.Rat)
	if _, ok := r.SetString(strings.TrimSpace(taxRate)); !ok {
		return 0, fmt.Errorf("invalid taxRate")
	}
	base := big.NewRat(preTaxMinor, 1)
	value := new(big.Rat).Mul(base, r)
	num := new(big.Int).Set(value.Num())
	den := new(big.Int).Set(value.Denom())
	q := new(big.Int)
	rem := new(big.Int)
	q.QuoRem(num, den, rem)
	if rem.Sign() != 0 {
		doubleRem := new(big.Int).Mul(new(big.Int).Abs(rem), big.NewInt(2))
		if doubleRem.Cmp(new(big.Int).Abs(den)) >= 0 {
			if num.Sign() >= 0 {
				q.Add(q, big.NewInt(1))
			} else {
				q.Sub(q, big.NewInt(1))
			}
		}
	}
	if !q.IsInt64() {
		return 0, fmt.Errorf("tax overflow")
	}
	return q.Int64(), nil
}

func (s *Service) writeAudit(ctx context.Context, action, resourceType, resourceID string, detail map[string]any) {
	if s.audit == nil {
		return
	}
	actor := int64(0)
	if ac, ok := adminapp.AuthContextFrom(ctx); ok {
		actor = ac.UserID
	}
	s.audit.Write(ctx, AuditEntry{
		ActorID: actor, Action: action, ResourceType: resourceType, ResourceID: resourceID, Detail: detail,
	})
}

func mapRepoErr(err error, notFoundMsg string) error {
	var appErr *Error
	if errors.As(err, &appErr) {
		return appErr
	}
	if errors.Is(err, adminapp.ErrNotFound) {
		return notFoundErr(notFoundMsg)
	}
	if errors.Is(err, adminapp.ErrConflict) {
		return conflictErr("资源冲突")
	}
	return err
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

func defaultString(v, def string) string {
	if v == "" {
		return def
	}
	return v
}
