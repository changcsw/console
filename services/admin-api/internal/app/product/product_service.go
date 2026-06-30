package product

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"

	adminapp "github.com/csw/console/services/admin-api/internal/app/admin"
	"github.com/csw/console/services/admin-api/internal/app/dto"
	"github.com/csw/console/services/admin-api/internal/domain/common"
	domainproduct "github.com/csw/console/services/admin-api/internal/domain/product"
)

type ProductService struct {
	tx    TxManager
	audit AuditSink
	env   common.Environment
	now   nowFunc
}

func NewProductService(tx TxManager, audit AuditSink, env common.Environment, now nowFunc) *ProductService {
	if now == nil {
		now = time.Now
	}
	return &ProductService{tx: tx, audit: audit, env: env, now: now}
}

func (s *ProductService) ListProducts(ctx context.Context, q dto.ListProductsQuery) (dto.Page[dto.ProductView], error) {
	if strings.TrimSpace(q.GameID) == "" {
		return dto.Page[dto.ProductView]{}, validationErr("gameId 不能为空", fieldDetail("gameId", "required"))
	}
	if len(strings.TrimSpace(q.Keyword)) > 128 {
		return dto.Page[dto.ProductView]{}, validationErr("keyword 长度不能超过 128", fieldDetail("keyword", "maxLen 128"))
	}
	page, pageSize := normalizePage(q.Page, q.PageSize)
	items, total, err := s.tx.Repositories().Products.ListByGame(ctx, q.GameID, q.Keyword, q.Enabled, page, pageSize, q.Sort)
	if err != nil {
		return dto.Page[dto.ProductView]{}, mapReadErr(err, "game or products not found")
	}
	result := make([]dto.ProductView, 0, len(items))
	for _, item := range items {
		spec, err := s.tx.Repositories().CurrencySpecs.GetByCode(ctx, item.BaseCurrency)
		if err != nil {
			return dto.Page[dto.ProductView]{}, mapReadErr(err, "currency not found")
		}
		result = append(result, toProductView(item, spec, s.env))
	}
	return dto.Page[dto.ProductView]{Items: result, Page: page, PageSize: pageSize, Total: total}, nil
}

func (s *ProductService) CreateProduct(ctx context.Context, cmd dto.CreateProductCmd) (dto.ProductView, error) {
	create := domainproduct.Product{
		GameID:       strings.TrimSpace(cmd.GameID),
		ProductID:    strings.TrimSpace(cmd.ProductID),
		ProductName:  strings.TrimSpace(cmd.ProductName),
		BaseCurrency: strings.ToUpper(strings.TrimSpace(cmd.BaseCurrency)),
		PriceID:      strings.TrimSpace(cmd.PriceID),
		Enabled:      true,
	}
	if cmd.Enabled != nil {
		create.Enabled = *cmd.Enabled
	}
	if err := validateProductIdentity(create.ProductID, create.PriceID); err != nil {
		return dto.ProductView{}, err
	}
	if create.ProductName == "" || len(create.ProductName) > 128 {
		return dto.ProductView{}, validationErr("productName 必填且不超过 128", fieldDetail("productName", "1-128"))
	}
	spec, amountMinor, err := s.normalizeAmount(ctx, create.BaseCurrency, cmd.BaseAmountMinor, cmd.BaseAmount)
	if err != nil {
		return dto.ProductView{}, err
	}
	create.BaseAmountMinor = amountMinor
	item, err := s.tx.Repositories().Products.Create(ctx, create)
	if err != nil {
		return dto.ProductView{}, mapWriteErr(err, "product already exists")
	}
	s.writeAudit(ctx, "product.create", create.GameID+"|"+create.ProductID, map[string]any{
		"productId": create.ProductID,
		"priceId":   create.PriceID,
		"enabled":   create.Enabled,
	})
	return toProductView(item, spec, s.env), nil
}

func (s *ProductService) UpdateProduct(ctx context.Context, cmd dto.UpdateProductCmd) (dto.ProductView, error) {
	gameID := strings.TrimSpace(cmd.GameID)
	productID := strings.TrimSpace(cmd.ProductID)
	if gameID == "" || productID == "" {
		return dto.ProductView{}, validationErr("gameId/productId 不能为空", fieldDetail("gameId", "required"), fieldDetail("productId", "required"))
	}
	current, err := s.tx.Repositories().Products.GetByGameAndProductID(ctx, gameID, productID)
	if err != nil {
		return dto.ProductView{}, mapReadErr(err, "product not found")
	}
	patch := ProductPatch{}
	if cmd.ProductName != nil {
		name := strings.TrimSpace(*cmd.ProductName)
		if name == "" || len(name) > 128 {
			return dto.ProductView{}, validationErr("productName 必填且不超过 128", fieldDetail("productName", "1-128"))
		}
		patch.ProductName = &name
	}
	if cmd.PriceID != nil {
		priceID := strings.TrimSpace(*cmd.PriceID)
		if len(priceID) == 0 || len(priceID) > domainproduct.MaxPriceIDLen {
			return dto.ProductView{}, validationErr("priceId 非法", fieldDetail("priceId", "1-64"))
		}
		patch.PriceID = &priceID
	}
	if cmd.Enabled != nil {
		patch.Enabled = cmd.Enabled
	}
	baseCurrency := current.BaseCurrency
	if cmd.BaseCurrency != nil {
		baseCurrency = strings.ToUpper(strings.TrimSpace(*cmd.BaseCurrency))
		patch.BaseCurrency = &baseCurrency
	}
	if cmd.BaseAmount != nil || cmd.BaseAmountMinor != nil || cmd.BaseCurrency != nil {
		_, amountMinor, err := s.normalizeAmount(ctx, baseCurrency, cmd.BaseAmountMinor, cmd.BaseAmount)
		if err != nil {
			return dto.ProductView{}, err
		}
		patch.BaseAmountMinor = &amountMinor
	}
	updated, err := s.tx.Repositories().Products.Update(ctx, gameID, productID, patch)
	if err != nil {
		return dto.ProductView{}, mapWriteErr(err, "product update failed")
	}
	spec, err := s.tx.Repositories().CurrencySpecs.GetByCode(ctx, updated.BaseCurrency)
	if err != nil {
		return dto.ProductView{}, mapReadErr(err, "currency not found")
	}
	s.writeAudit(ctx, "product.update", gameID+"|"+productID, map[string]any{
		"updatedFields": changedProductFields(patch),
	})
	return toProductView(updated, spec, s.env), nil
}

func (s *ProductService) GetPackageProducts(ctx context.Context, packageID int64) ([]dto.PackageProductView, error) {
	gameID, _, _, _, err := s.tx.Repositories().Packages.GetPackageGameAndChannel(ctx, packageID)
	if err != nil {
		return nil, mapReadErr(err, "package not found")
	}
	mappings, err := s.tx.Repositories().ChannelProducts.ListByPackage(ctx, packageID)
	if err != nil {
		return nil, mapReadErr(err, "package products not found")
	}
	productsByID, err := s.loadProductsForMapping(ctx, gameID, mappings)
	if err != nil {
		return nil, err
	}
	views := make([]dto.PackageProductView, 0, len(mappings))
	for _, m := range mappings {
		base, ok := productsByID[m.ProductIDRef]
		if !ok {
			continue
		}
		effective := domainproduct.ResolveEffectiveIDs(base.ProductID, base.PriceID, m)
		views = append(views, dto.PackageProductView{
			ProductID:         base.ProductID,
			ProductName:       base.ProductName,
			Enabled:           m.Enabled,
			ProductIDMode:     string(m.ProductIDMode),
			ProductIDOverride: m.ProductIDOverride,
			PriceIDMode:       string(m.PriceIDMode),
			PriceIDOverride:   m.PriceIDOverride,
			Base: dto.PackageBaseView{
				ProductID:       base.ProductID,
				PriceID:         base.PriceID,
				BaseAmountMinor: base.BaseAmountMinor,
				BaseCurrency:    base.BaseCurrency,
			},
			Effective: dto.PackageIDPairView{ProductID: effective.ProductID, PriceID: effective.PriceID},
		})
	}
	return views, nil
}

func (s *ProductService) PutPackageProducts(ctx context.Context, cmd dto.PutPackageProductsCmd) ([]dto.PackageProductView, error) {
	if len(cmd.Items) == 0 {
		if err := s.tx.InTx(ctx, func(repos Repositories) error {
			return repos.ChannelProducts.ReplaceByPackage(ctx, cmd.PackageID, nil)
		}); err != nil {
			return nil, mapWriteErr(err, "replace package products failed")
		}
		return []dto.PackageProductView{}, nil
	}
	gameID, _, _, _, err := s.tx.Repositories().Packages.GetPackageGameAndChannel(ctx, cmd.PackageID)
	if err != nil {
		return nil, mapReadErr(err, "package not found")
	}
	productIDs := make([]string, 0, len(cmd.Items))
	seen := map[string]bool{}
	for i, item := range cmd.Items {
		id := strings.TrimSpace(item.ProductID)
		if id == "" {
			return nil, validationErr("productId 不能为空", fieldDetail(fmt.Sprintf("items[%d].productId", i), "required"))
		}
		if seen[id] {
			return nil, validationErr("items.productId 不可重复", fieldDetail(fmt.Sprintf("items[%d].productId", i), "duplicate"))
		}
		seen[id] = true
		productIDs = append(productIDs, id)
	}
	products, err := s.tx.Repositories().Products.ListByIDs(ctx, gameID, productIDs)
	if err != nil {
		return nil, mapReadErr(err, "products not found")
	}
	if len(products) != len(productIDs) {
		return nil, validationErr("存在不属于该包所属游戏的 productId", fieldDetail("items.productId", "not_in_game"))
	}
	productRefByID := map[string]int64{}
	productByRef := map[int64]domainproduct.Product{}
	for _, p := range products {
		productRefByID[p.ProductID] = p.ID
		productByRef[p.ID] = p
	}
	upserts := make([]domainproduct.ChannelProduct, 0, len(cmd.Items))
	for i, item := range cmd.Items {
		productMode, productOverride, err := domainproduct.NormalizeOverrideField(common.OverrideMode(item.ProductIDMode), item.ProductIDOverride, domainproduct.MaxProductIDLen)
		if err != nil {
			return nil, overrideValidationErr("productId", i, err)
		}
		priceMode, priceOverride, err := domainproduct.NormalizeOverrideField(common.OverrideMode(item.PriceIDMode), item.PriceIDOverride, domainproduct.MaxPriceIDLen)
		if err != nil {
			return nil, overrideValidationErr("priceId", i, err)
		}
		enabled := true
		if item.Enabled != nil {
			enabled = *item.Enabled
		}
		upserts = append(upserts, domainproduct.ChannelProduct{
			PackageIDRef:      cmd.PackageID,
			ProductIDRef:      productRefByID[strings.TrimSpace(item.ProductID)],
			ProductIDMode:     productMode,
			ProductIDOverride: productOverride,
			PriceIDMode:       priceMode,
			PriceIDOverride:   priceOverride,
			Enabled:           enabled,
		})
	}
	if err := s.tx.InTx(ctx, func(repos Repositories) error {
		return repos.ChannelProducts.ReplaceByPackage(ctx, cmd.PackageID, upserts)
	}); err != nil {
		return nil, mapWriteErr(err, "replace package products failed")
	}
	views := make([]dto.PackageProductView, 0, len(upserts))
	for _, item := range upserts {
		base := productByRef[item.ProductIDRef]
		effective := domainproduct.ResolveEffectiveIDs(base.ProductID, base.PriceID, item)
		views = append(views, dto.PackageProductView{
			ProductID:         base.ProductID,
			ProductName:       base.ProductName,
			Enabled:           item.Enabled,
			ProductIDMode:     string(item.ProductIDMode),
			ProductIDOverride: item.ProductIDOverride,
			PriceIDMode:       string(item.PriceIDMode),
			PriceIDOverride:   item.PriceIDOverride,
			Base: dto.PackageBaseView{
				ProductID:       base.ProductID,
				PriceID:         base.PriceID,
				BaseAmountMinor: base.BaseAmountMinor,
				BaseCurrency:    base.BaseCurrency,
			},
			Effective: dto.PackageIDPairView{ProductID: effective.ProductID, PriceID: effective.PriceID},
		})
	}
	s.writeAudit(ctx, "product.package_products.update", fmt.Sprintf("%d", cmd.PackageID), map[string]any{"items": len(upserts)})
	return views, nil
}

func (s *ProductService) normalizeAmount(ctx context.Context, currency string, amountMinor *int64, amount *string) (common.CurrencySpec, int64, error) {
	spec, err := s.tx.Repositories().CurrencySpecs.GetByCode(ctx, currency)
	if err != nil || !spec.Enabled {
		return common.CurrencySpec{}, 0, currencyErr(fmt.Sprintf("currency '%s' is not in currency_specs", currency))
	}
	if amountMinor == nil && amount == nil {
		return common.CurrencySpec{}, 0, validationErr("baseAmount/baseAmountMinor 至少传一个", fieldDetail("baseAmountMinor", "required"))
	}
	var minor int64
	if amountMinor != nil {
		minor = *amountMinor
	}
	if amount != nil {
		parsed, err := common.NormalizeMajorAmount(*amount, spec)
		if err != nil {
			return common.CurrencySpec{}, 0, validationErr("baseAmount 非法", fieldDetail("baseAmount", err.Error()))
		}
		if amountMinor != nil && parsed != *amountMinor {
			return common.CurrencySpec{}, 0, validationErr("baseAmount 与 baseAmountMinor 不一致", fieldDetail("baseAmount", "inconsistent"))
		}
		minor = parsed
	}
	normalized, err := common.NormalizeMinorAmount(minor, spec)
	if err != nil {
		return common.CurrencySpec{}, 0, validationErr("baseAmountMinor 小于最小值", fieldDetail("baseAmountMinor", err.Error()))
	}
	return spec, normalized, nil
}

func validateProductIdentity(productID, priceID string) error {
	if productID == "" || len(productID) > domainproduct.MaxProductIDLen {
		return validationErr("productId 非法", fieldDetail("productId", "1-128"))
	}
	if priceID == "" || len(priceID) > domainproduct.MaxPriceIDLen {
		return validationErr("priceId 非法", fieldDetail("priceId", "1-64"))
	}
	return nil
}

func (s *ProductService) loadProductsForMapping(ctx context.Context, gameID string, mappings []domainproduct.ChannelProduct) (map[int64]domainproduct.Product, error) {
	if len(mappings) == 0 {
		return map[int64]domainproduct.Product{}, nil
	}
	all, _, err := s.tx.Repositories().Products.ListByGame(ctx, gameID, "", nil, 1, 1000, "updated_at DESC")
	if err != nil {
		return nil, mapReadErr(err, "products not found")
	}
	out := map[int64]domainproduct.Product{}
	for _, p := range all {
		out[p.ID] = p
	}
	return out, nil
}

func toProductView(item domainproduct.Product, spec common.CurrencySpec, env common.Environment) dto.ProductView {
	return dto.ProductView{
		ID:                item.ID,
		Env:               string(env),
		GameID:            item.GameID,
		ProductID:         item.ProductID,
		ProductName:       item.ProductName,
		BaseAmountMinor:   item.BaseAmountMinor,
		BaseCurrency:      item.BaseCurrency,
		BaseAmountDisplay: common.FormatMinorAmount(item.BaseAmountMinor, spec.DecimalPlaces),
		PriceID:           item.PriceID,
		Enabled:           item.Enabled,
		CreatedAt:         item.CreatedAt,
		UpdatedAt:         item.UpdatedAt,
	}
}

func changedProductFields(p ProductPatch) []string {
	keys := []string{}
	if p.ProductName != nil {
		keys = append(keys, "productName")
	}
	if p.BaseCurrency != nil {
		keys = append(keys, "baseCurrency")
	}
	if p.BaseAmountMinor != nil {
		keys = append(keys, "baseAmountMinor")
	}
	if p.PriceID != nil {
		keys = append(keys, "priceId")
	}
	if p.Enabled != nil {
		keys = append(keys, "enabled")
	}
	slices.Sort(keys)
	return keys
}

func overrideValidationErr(dim string, index int, err error) *Error {
	field := fmt.Sprintf("items[%d].%sOverride", index, dim)
	msg := fmt.Sprintf("%sOverride is required when %sMode=override", dim, dim)
	if !errors.Is(err, domainproduct.ErrOverrideRequired) {
		msg = err.Error()
	}
	return validationErr(msg, fieldDetail(field, "invalid_override"))
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

func mapReadErr(err error, notFoundMsg string) error {
	var appErr *Error
	if errors.As(err, &appErr) {
		return appErr
	}
	if errors.Is(err, adminapp.ErrNotFound) {
		return notFoundErr(notFoundMsg)
	}
	return err
}

func mapWriteErr(err error, conflictMsg string) error {
	var appErr *Error
	if errors.As(err, &appErr) {
		return appErr
	}
	if errors.Is(err, adminapp.ErrConflict) {
		return conflictErr(conflictMsg)
	}
	if errors.Is(err, adminapp.ErrNotFound) {
		return notFoundErr("resource not found")
	}
	return err
}

func (s *ProductService) writeAudit(ctx context.Context, action, resourceID string, detail map[string]any) {
	if s.audit == nil {
		return
	}
	actor := int64(0)
	if ac, ok := adminapp.AuthContextFrom(ctx); ok {
		actor = ac.UserID
	}
	s.audit.Write(ctx, AuditEntry{
		ActorID:      actor,
		Action:       action,
		ResourceType: "product",
		ResourceID:   resourceID,
		Detail:       detail,
	})
}
