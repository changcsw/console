package games

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"

	"github.com/csw/console/services/admin-api/internal/app/dto"
	"github.com/csw/console/services/admin-api/internal/transport/http/httpx"
)

type createProductRequest struct {
	ProductID       string  `json:"productId"`
	ProductName     string  `json:"productName"`
	BaseCurrency    string  `json:"baseCurrency"`
	BaseAmountMinor *int64  `json:"baseAmountMinor"`
	BaseAmount      *string `json:"baseAmount"`
	PriceID         string  `json:"priceId"`
	Enabled         *bool   `json:"enabled"`
}

type updateProductRequest struct {
	ProductName     *string `json:"productName"`
	BaseCurrency    *string `json:"baseCurrency"`
	BaseAmountMinor *int64  `json:"baseAmountMinor"`
	BaseAmount      *string `json:"baseAmount"`
	PriceID         *string `json:"priceId"`
	Enabled         *bool   `json:"enabled"`
}

type putPackageProductsRequest struct {
	Items []putPackageProductItem `json:"items"`
}

type putPackageProductItem struct {
	ProductID         string `json:"productId"`
	Enabled           *bool  `json:"enabled"`
	ProductIDMode     string `json:"productIdMode"`
	ProductIDOverride string `json:"productIdOverride"`
	PriceIDMode       string `json:"priceIdMode"`
	PriceIDOverride   string `json:"priceIdOverride"`
}

type putIAPConfigRequest struct {
	Enabled    *bool          `json:"enabled"`
	ConfigJSON map[string]any `json:"configJson"`
}

func (h *Handler) ListProducts(w http.ResponseWriter, r *http.Request) {
	if h.productSvc == nil {
		httpx.WriteError(w, http.StatusServiceUnavailable, httpx.CodeInternal, "product backend unavailable")
		return
	}
	q := r.URL.Query()
	page, pageSize := parsePage(q)
	enabled, err := parseOptionalBoolParam("enabled", q.Get("enabled"))
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, httpx.CodeValidation, err.Error(), map[string]string{"field": "enabled", "reason": "bool"})
		return
	}
	result, err := h.productSvc.ListProducts(r.Context(), dto.ListProductsQuery{
		GameID:   chi.URLParam(r, "gameId"),
		Keyword:  q.Get("keyword"),
		Enabled:  enabled,
		Page:     page,
		PageSize: pageSize,
		Sort:     q.Get("sort"),
	})
	if err != nil {
		writeError(w, err)
		return
	}
	httpx.WriteData(w, http.StatusOK, result)
}

func (h *Handler) CreateProduct(w http.ResponseWriter, r *http.Request) {
	if h.productSvc == nil {
		httpx.WriteError(w, http.StatusServiceUnavailable, httpx.CodeInternal, "product backend unavailable")
		return
	}
	var req createProductRequest
	if err := decodeJSON(r, &req); err != nil {
		badRequest(w)
		return
	}
	result, err := h.productSvc.CreateProduct(r.Context(), dto.CreateProductCmd{
		GameID:          chi.URLParam(r, "gameId"),
		ProductID:       req.ProductID,
		ProductName:     req.ProductName,
		BaseCurrency:    req.BaseCurrency,
		BaseAmountMinor: req.BaseAmountMinor,
		BaseAmount:      req.BaseAmount,
		PriceID:         req.PriceID,
		Enabled:         req.Enabled,
	})
	if err != nil {
		writeError(w, err)
		return
	}
	httpx.WriteData(w, http.StatusCreated, result)
}

func (h *Handler) UpdateProduct(w http.ResponseWriter, r *http.Request) {
	if h.productSvc == nil {
		httpx.WriteError(w, http.StatusServiceUnavailable, httpx.CodeInternal, "product backend unavailable")
		return
	}
	var req updateProductRequest
	if err := decodeJSON(r, &req); err != nil {
		badRequest(w)
		return
	}
	result, err := h.productSvc.UpdateProduct(r.Context(), dto.UpdateProductCmd{
		GameID:          r.URL.Query().Get("gameId"),
		ProductID:       chi.URLParam(r, "productId"),
		ProductName:     req.ProductName,
		BaseCurrency:    req.BaseCurrency,
		BaseAmountMinor: req.BaseAmountMinor,
		BaseAmount:      req.BaseAmount,
		PriceID:         req.PriceID,
		Enabled:         req.Enabled,
	})
	if err != nil {
		writeError(w, err)
		return
	}
	httpx.WriteData(w, http.StatusOK, result)
}

func (h *Handler) GetPackageProducts(w http.ResponseWriter, r *http.Request) {
	if h.productSvc == nil {
		httpx.WriteError(w, http.StatusServiceUnavailable, httpx.CodeInternal, "product backend unavailable")
		return
	}
	packageID, ok := parseID(w, r, "packageId")
	if !ok {
		return
	}
	items, err := h.productSvc.GetPackageProducts(r.Context(), packageID)
	if err != nil {
		writeError(w, err)
		return
	}
	httpx.WriteData(w, http.StatusOK, map[string]any{"items": items})
}

func (h *Handler) PutPackageProducts(w http.ResponseWriter, r *http.Request) {
	if h.productSvc == nil {
		httpx.WriteError(w, http.StatusServiceUnavailable, httpx.CodeInternal, "product backend unavailable")
		return
	}
	packageID, ok := parseID(w, r, "packageId")
	if !ok {
		return
	}
	var req putPackageProductsRequest
	if err := decodeJSON(r, &req); err != nil {
		badRequest(w)
		return
	}
	items := make([]dto.PutPackageProductItem, 0, len(req.Items))
	for _, item := range req.Items {
		items = append(items, dto.PutPackageProductItem{
			ProductID:         item.ProductID,
			Enabled:           item.Enabled,
			ProductIDMode:     item.ProductIDMode,
			ProductIDOverride: item.ProductIDOverride,
			PriceIDMode:       item.PriceIDMode,
			PriceIDOverride:   item.PriceIDOverride,
		})
	}
	result, err := h.productSvc.PutPackageProducts(r.Context(), dto.PutPackageProductsCmd{
		PackageID: packageID,
		Items:     items,
	})
	if err != nil {
		writeError(w, err)
		return
	}
	httpx.WriteData(w, http.StatusOK, map[string]any{"items": result})
}

func (h *Handler) GetGameChannelIAPConfig(w http.ResponseWriter, r *http.Request) {
	if h.iapSvc == nil {
		httpx.WriteError(w, http.StatusServiceUnavailable, httpx.CodeInternal, "iap backend unavailable")
		return
	}
	gameChannelID, ok := parseID(w, r, "gameChannelId")
	if !ok {
		return
	}
	result, err := h.iapSvc.GetGameChannelConfig(r.Context(), gameChannelID)
	if err != nil {
		writeError(w, err)
		return
	}
	httpx.WriteData(w, http.StatusOK, result)
}

func (h *Handler) PutGameChannelIAPConfig(w http.ResponseWriter, r *http.Request) {
	if h.iapSvc == nil {
		httpx.WriteError(w, http.StatusServiceUnavailable, httpx.CodeInternal, "iap backend unavailable")
		return
	}
	gameChannelID, ok := parseID(w, r, "gameChannelId")
	if !ok {
		return
	}
	var req putIAPConfigRequest
	if err := decodeJSON(r, &req); err != nil {
		badRequest(w)
		return
	}
	result, err := h.iapSvc.PutGameChannelConfig(r.Context(), dto.UpsertIAPConfigCmd{
		GameChannelID: gameChannelID,
		Enabled:       req.Enabled,
		ConfigJSON:    req.ConfigJSON,
	})
	if err != nil {
		writeError(w, err)
		return
	}
	httpx.WriteData(w, http.StatusOK, result)
}

func (h *Handler) GetPackageIAPOverride(w http.ResponseWriter, r *http.Request) {
	if h.iapSvc == nil {
		httpx.WriteError(w, http.StatusServiceUnavailable, httpx.CodeInternal, "iap backend unavailable")
		return
	}
	packageID, ok := parseID(w, r, "packageId")
	if !ok {
		return
	}
	result, err := h.iapSvc.GetPackageOverride(r.Context(), packageID)
	if err != nil {
		writeError(w, err)
		return
	}
	httpx.WriteData(w, http.StatusOK, result)
}

func (h *Handler) PutPackageIAPOverride(w http.ResponseWriter, r *http.Request) {
	if h.iapSvc == nil {
		httpx.WriteError(w, http.StatusServiceUnavailable, httpx.CodeInternal, "iap backend unavailable")
		return
	}
	packageID, ok := parseID(w, r, "packageId")
	if !ok {
		return
	}
	var req putIAPConfigRequest
	if err := decodeJSON(r, &req); err != nil {
		badRequest(w)
		return
	}
	result, err := h.iapSvc.PutPackageOverride(r.Context(), dto.UpsertPackageIAPOverrideCmd{
		PackageID:  packageID,
		Enabled:    req.Enabled,
		ConfigJSON: req.ConfigJSON,
	})
	if err != nil {
		writeError(w, err)
		return
	}
	httpx.WriteData(w, http.StatusOK, result)
}

func parseOptionalBoolParam(name, raw string) (*bool, error) {
	if raw == "" {
		return nil, nil
	}
	v, err := strconv.ParseBool(raw)
	if err != nil {
		return nil, errors.New(name + " 非法")
	}
	return &v, nil
}

func parseID(w http.ResponseWriter, r *http.Request, name string) (int64, bool) {
	raw := chi.URLParam(r, name)
	id, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || id <= 0 {
		httpx.WriteError(w, http.StatusBadRequest, httpx.CodeValidation, name+" 非法",
			map[string]string{"field": name, "reason": "int64"})
		return 0, false
	}
	return id, true
}
