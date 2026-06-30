package product

import (
	"context"
	"errors"
	"testing"

	"github.com/csw/console/services/admin-api/internal/app/dto"
	"github.com/csw/console/services/admin-api/internal/domain/common"
	domainproduct "github.com/csw/console/services/admin-api/internal/domain/product"
)

func ptrStr(s string) *string { return &s }
func ptrI64(v int64) *int64   { return &v }
func ptrBool(v bool) *bool    { return &v }

func errCode(t *testing.T, err error) string {
	t.Helper()
	var ae *Error
	if !errors.As(err, &ae) {
		t.Fatalf("期望 *Error，got %T: %v", err, err)
	}
	return ae.Code
}

func newProductSvc() (*ProductService, *memStore, *spyAudit) {
	store := newMemStore()
	audit := &spyAudit{}
	svc := NewProductService(store, audit, common.EnvSandbox, fixedNow)
	return svc, store, audit
}

// ───────────────────────── CreateProduct：金额归一化 ─────────────────────────

func TestCreateProduct_USD_NormalizesMajorAmount(t *testing.T) {
	svc, _, audit := newProductSvc()
	view, err := svc.CreateProduct(context.Background(), dto.CreateProductCmd{
		GameID:       "100001",
		ProductID:    "com.demo.gold",
		ProductName:  "Gold Pack",
		BaseCurrency: "usd",
		BaseAmount:   ptrStr("4.999"),
		PriceID:      "price_gold",
	})
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if view.BaseAmountMinor != 500 {
		t.Fatalf("USD 4.999 应归一为 500 minor, got %d", view.BaseAmountMinor)
	}
	if view.BaseAmountDisplay != "5.00" {
		t.Fatalf("display 应反算为 5.00, got %q", view.BaseAmountDisplay)
	}
	if view.BaseCurrency != "USD" {
		t.Fatalf("币种应大写归一, got %q", view.BaseCurrency)
	}
	if view.Env != "sandbox" {
		t.Fatalf("env 应取运行环境 sandbox, got %q", view.Env)
	}
	// S7：写审计 product.create。
	if len(audit.entries) != 1 || audit.entries[0].Action != "product.create" {
		t.Fatalf("应写一条 product.create 审计, got %+v", audit.entries)
	}
}

func TestCreateProduct_JPY_NoDecimalHalfUp(t *testing.T) {
	svc, _, _ := newProductSvc()
	view, err := svc.CreateProduct(context.Background(), dto.CreateProductCmd{
		GameID:       "100001",
		ProductID:    "com.demo.jpy",
		ProductName:  "JPY Pack",
		BaseCurrency: "JPY",
		BaseAmount:   ptrStr("120.5"),
		PriceID:      "price_jpy",
	})
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if view.BaseAmountMinor != 121 {
		t.Fatalf("JPY 120.5 应 half_up 归一为 121, got %d", view.BaseAmountMinor)
	}
}

func TestCreateProduct_BelowMin_Rejected(t *testing.T) {
	svc, _, _ := newProductSvc()
	_, err := svc.CreateProduct(context.Background(), dto.CreateProductCmd{
		GameID:       "100001",
		ProductID:    "com.demo.tiny",
		ProductName:  "Tiny",
		BaseCurrency: "USD",
		BaseAmount:   ptrStr("0.001"), // 归一为 0 minor < min=1
		PriceID:      "price_tiny",
	})
	if err == nil || errCode(t, err) != codeValidation {
		t.Fatalf("低于最小值应 VALIDATION_FAILED, got %v", err)
	}
}

func TestCreateProduct_CurrencyNotSupported(t *testing.T) {
	svc, _, _ := newProductSvc()
	// EUR 不在 specs。
	_, err := svc.CreateProduct(context.Background(), dto.CreateProductCmd{
		GameID:       "100001",
		ProductID:    "com.demo.eur",
		ProductName:  "Euro",
		BaseCurrency: "EUR",
		BaseAmountMinor: ptrI64(500),
		PriceID:      "price_eur",
	})
	if err == nil || errCode(t, err) != codeCurrency {
		t.Fatalf("未知币种应 CURRENCY_NOT_SUPPORTED, got %v", err)
	}
}

func TestCreateProduct_CurrencyDisabled(t *testing.T) {
	svc, _, _ := newProductSvc()
	// ABC 存在但 enabled=false。
	_, err := svc.CreateProduct(context.Background(), dto.CreateProductCmd{
		GameID:       "100001",
		ProductID:    "com.demo.abc",
		ProductName:  "ABC",
		BaseCurrency: "ABC",
		BaseAmountMinor: ptrI64(500),
		PriceID:      "price_abc",
	})
	if err == nil || errCode(t, err) != codeCurrency {
		t.Fatalf("disabled 币种应 CURRENCY_NOT_SUPPORTED, got %v", err)
	}
}

func TestCreateProduct_AmountInconsistent(t *testing.T) {
	svc, _, _ := newProductSvc()
	// baseAmount(4.999→500) 与 baseAmountMinor(600) 不一致。
	_, err := svc.CreateProduct(context.Background(), dto.CreateProductCmd{
		GameID:          "100001",
		ProductID:       "com.demo.mix",
		ProductName:     "Mix",
		BaseCurrency:    "USD",
		BaseAmount:      ptrStr("4.999"),
		BaseAmountMinor: ptrI64(600),
		PriceID:         "price_mix",
	})
	if err == nil || errCode(t, err) != codeValidation {
		t.Fatalf("amount 与 minor 不一致应 VALIDATION_FAILED, got %v", err)
	}
}

func TestCreateProduct_PriceIDTooLong(t *testing.T) {
	svc, _, _ := newProductSvc()
	long := make([]byte, 65)
	for i := range long {
		long[i] = 'a'
	}
	_, err := svc.CreateProduct(context.Background(), dto.CreateProductCmd{
		GameID:       "100001",
		ProductID:    "com.demo.long",
		ProductName:  "Long",
		BaseCurrency: "USD",
		BaseAmountMinor: ptrI64(500),
		PriceID:      string(long),
	})
	if err == nil || errCode(t, err) != codeValidation {
		t.Fatalf("priceId 超 64 应 VALIDATION_FAILED, got %v", err)
	}
}

func TestCreateProduct_Conflict(t *testing.T) {
	svc, _, _ := newProductSvc()
	cmd := dto.CreateProductCmd{
		GameID:          "100001",
		ProductID:       "com.demo.dup",
		ProductName:     "Dup",
		BaseCurrency:    "USD",
		BaseAmountMinor: ptrI64(500),
		PriceID:         "price_dup",
	}
	if _, err := svc.CreateProduct(context.Background(), cmd); err != nil {
		t.Fatalf("首次创建应成功: %v", err)
	}
	_, err := svc.CreateProduct(context.Background(), cmd)
	if err == nil || errCode(t, err) != codeConflict {
		t.Fatalf("同 (game, productId) 重复应 CONFLICT, got %v", err)
	}
}

// ───────────────────────── UpdateProduct ─────────────────────────

func TestUpdateProduct_ReNormalizesAmount(t *testing.T) {
	svc, _, _ := newProductSvc()
	if _, err := svc.CreateProduct(context.Background(), dto.CreateProductCmd{
		GameID: "100001", ProductID: "com.demo.upd", ProductName: "Upd",
		BaseCurrency: "USD", BaseAmountMinor: ptrI64(500), PriceID: "p1",
	}); err != nil {
		t.Fatalf("seed create: %v", err)
	}
	view, err := svc.UpdateProduct(context.Background(), dto.UpdateProductCmd{
		GameID: "100001", ProductID: "com.demo.upd",
		BaseAmount: ptrStr("9.999"), // → 1000
	})
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if view.BaseAmountMinor != 1000 {
		t.Fatalf("更新金额应重新归一为 1000, got %d", view.BaseAmountMinor)
	}
}

func TestUpdateProduct_NotFound(t *testing.T) {
	svc, _, _ := newProductSvc()
	_, err := svc.UpdateProduct(context.Background(), dto.UpdateProductCmd{
		GameID: "100001", ProductID: "ghost", ProductName: ptrStr("x"),
	})
	if err == nil || errCode(t, err) != codeNotFound {
		t.Fatalf("不存在商品应 NOT_FOUND, got %v", err)
	}
}

// ───────────────────────── PutPackageProducts：包级全量 upsert + 删未出现项 ─────────────────────────

func seedPackageProducts(t *testing.T, svc *ProductService, store *memStore) {
	t.Helper()
	store.state.packages[7001] = pkgInfo{gameID: "100001", packageCode: "pkg-a", channelID: "google", gameChannelID: 9001}
	for _, p := range []dto.CreateProductCmd{
		{GameID: "100001", ProductID: "sku.a", ProductName: "A", BaseCurrency: "USD", BaseAmountMinor: ptrI64(500), PriceID: "price_a"},
		{GameID: "100001", ProductID: "sku.b", ProductName: "B", BaseCurrency: "USD", BaseAmountMinor: ptrI64(600), PriceID: "price_b"},
	} {
		if _, err := svc.CreateProduct(context.Background(), p); err != nil {
			t.Fatalf("seed product %s: %v", p.ProductID, err)
		}
	}
}

func TestPutPackageProducts_FullUpsertThenDeleteMissing(t *testing.T) {
	svc, store, _ := newProductSvc()
	seedPackageProducts(t, svc, store)

	// 第一次：声明 a + b。
	_, err := svc.PutPackageProducts(context.Background(), dto.PutPackageProductsCmd{
		PackageID: 7001,
		Items: []dto.PutPackageProductItem{
			{ProductID: "sku.a"},
			{ProductID: "sku.b", ProductIDMode: "override", ProductIDOverride: "store-sku-b"},
		},
	})
	if err != nil {
		t.Fatalf("first put: %v", err)
	}
	if got := len(store.state.channelProds[7001]); got != 2 {
		t.Fatalf("应存 2 条映射, got %d", got)
	}

	// 第二次：只声明 a → b 被删除（删除未出现项语义）。
	views, err := svc.PutPackageProducts(context.Background(), dto.PutPackageProductsCmd{
		PackageID: 7001,
		Items:     []dto.PutPackageProductItem{{ProductID: "sku.a"}},
	})
	if err != nil {
		t.Fatalf("second put: %v", err)
	}
	if got := len(store.state.channelProds[7001]); got != 1 {
		t.Fatalf("全量覆盖后应只剩 1 条（b 删除）, got %d", got)
	}
	if len(views) != 1 || views[0].ProductID != "sku.a" {
		t.Fatalf("返回应只含 sku.a, got %+v", views)
	}
}

func TestPutPackageProducts_EmptyClearsAll(t *testing.T) {
	svc, store, _ := newProductSvc()
	seedPackageProducts(t, svc, store)
	if _, err := svc.PutPackageProducts(context.Background(), dto.PutPackageProductsCmd{
		PackageID: 7001,
		Items:     []dto.PutPackageProductItem{{ProductID: "sku.a"}},
	}); err != nil {
		t.Fatalf("seed mapping: %v", err)
	}
	if _, err := svc.PutPackageProducts(context.Background(), dto.PutPackageProductsCmd{PackageID: 7001, Items: nil}); err != nil {
		t.Fatalf("empty put: %v", err)
	}
	if got := len(store.state.channelProds[7001]); got != 0 {
		t.Fatalf("空 items 应清空全部映射, got %d", got)
	}
}

func TestPutPackageProducts_EffectiveTwoDimensionsIndependent(t *testing.T) {
	svc, store, _ := newProductSvc()
	seedPackageProducts(t, svc, store)
	views, err := svc.PutPackageProducts(context.Background(), dto.PutPackageProductsCmd{
		PackageID: 7001,
		Items: []dto.PutPackageProductItem{
			// product 覆盖、price 回退基准。
			{ProductID: "sku.a", ProductIDMode: "override", ProductIDOverride: "store-a"},
		},
	})
	if err != nil {
		t.Fatalf("put: %v", err)
	}
	v := views[0]
	if v.Effective.ProductID != "store-a" {
		t.Fatalf("product 覆盖应生效, got %q", v.Effective.ProductID)
	}
	if v.Effective.PriceID != "price_a" {
		t.Fatalf("price 维 default 应回退基准 price_a, got %q", v.Effective.PriceID)
	}
	// 落库 default 维 override 已清空。
	stored := store.state.channelProds[7001][0]
	if stored.PriceIDMode != common.OverrideModeDefault || stored.PriceIDOverride != "" {
		t.Fatalf("price default 维落库 override 应清空, got mode=%s ov=%q", stored.PriceIDMode, stored.PriceIDOverride)
	}
}

func TestPutPackageProducts_OverrideRequiredWhenModeOverride(t *testing.T) {
	svc, store, _ := newProductSvc()
	seedPackageProducts(t, svc, store)
	_, err := svc.PutPackageProducts(context.Background(), dto.PutPackageProductsCmd{
		PackageID: 7001,
		Items: []dto.PutPackageProductItem{
			{ProductID: "sku.a", PriceIDMode: "override", PriceIDOverride: ""},
		},
	})
	if err == nil || errCode(t, err) != codeValidation {
		t.Fatalf("priceId override 模式空值应 VALIDATION_FAILED, got %v", err)
	}
	if got := len(store.state.channelProds[7001]); got != 0 {
		t.Fatalf("校验失败不应写入, got %d", got)
	}
}

func TestPutPackageProducts_DuplicateProductID(t *testing.T) {
	svc, store, _ := newProductSvc()
	seedPackageProducts(t, svc, store)
	_, err := svc.PutPackageProducts(context.Background(), dto.PutPackageProductsCmd{
		PackageID: 7001,
		Items:     []dto.PutPackageProductItem{{ProductID: "sku.a"}, {ProductID: "sku.a"}},
	})
	if err == nil || errCode(t, err) != codeValidation {
		t.Fatalf("items 内 productId 重复应 VALIDATION_FAILED, got %v", err)
	}
}

func TestPutPackageProducts_ProductNotInGame(t *testing.T) {
	svc, store, _ := newProductSvc()
	seedPackageProducts(t, svc, store)
	_, err := svc.PutPackageProducts(context.Background(), dto.PutPackageProductsCmd{
		PackageID: 7001,
		Items:     []dto.PutPackageProductItem{{ProductID: "sku.not-exist"}},
	})
	if err == nil || errCode(t, err) != codeValidation {
		t.Fatalf("productId 不属该游戏应 VALIDATION_FAILED, got %v", err)
	}
}

// S10：包级 ReplaceByPackage 中途失败 → 整体回滚，无部分写入。
func TestPutPackageProducts_TransactionRollback(t *testing.T) {
	svc, store, _ := newProductSvc()
	seedPackageProducts(t, svc, store)
	// 先写入一份基线。
	if _, err := svc.PutPackageProducts(context.Background(), dto.PutPackageProductsCmd{
		PackageID: 7001,
		Items:     []dto.PutPackageProductItem{{ProductID: "sku.a"}},
	}); err != nil {
		t.Fatalf("seed mapping: %v", err)
	}
	before := append([]domainproduct.ChannelProduct(nil), store.state.channelProds[7001]...)

	store.state.failReplace = true
	_, err := svc.PutPackageProducts(context.Background(), dto.PutPackageProductsCmd{
		PackageID: 7001,
		Items:     []dto.PutPackageProductItem{{ProductID: "sku.b"}},
	})
	if err == nil {
		t.Fatalf("强制失败下应返回错误")
	}
	after := store.state.channelProds[7001]
	if len(after) != len(before) || after[0].ProductIDRef != before[0].ProductIDRef {
		t.Fatalf("事务失败应整体回滚，映射保持不变, before=%+v after=%+v", before, after)
	}
}

func TestListProducts_Pagination(t *testing.T) {
	svc, store, _ := newProductSvc()
	seedPackageProducts(t, svc, store) // 2 products
	page, err := svc.ListProducts(context.Background(), dto.ListProductsQuery{GameID: "100001", Page: 1, PageSize: 1})
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if page.Total != 2 || len(page.Items) != 1 || page.PageSize != 1 {
		t.Fatalf("分页应 total=2/items=1/pageSize=1, got total=%d items=%d ps=%d", page.Total, len(page.Items), page.PageSize)
	}
}

func TestListProducts_PageSizeClampedTo100(t *testing.T) {
	svc, _, _ := newProductSvc()
	page, err := svc.ListProducts(context.Background(), dto.ListProductsQuery{GameID: "100001", Page: 1, PageSize: 99999})
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if page.PageSize != 100 {
		t.Fatalf("pageSize 超限应钳制为 100, got %d", page.PageSize)
	}
}
