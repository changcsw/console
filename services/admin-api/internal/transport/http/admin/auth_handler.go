package admin

import (
	"net/http"

	adminapp "github.com/csw/console/services/admin-api/internal/app/admin"
	"github.com/csw/console/services/admin-api/internal/app/dto"
	"github.com/csw/console/services/admin-api/internal/transport/http/httpx"
)

type loginRequest struct {
	UserName string `json:"userName"`
	Password string `json:"password"`
}

type refreshRequest struct {
	RefreshToken string `json:"refreshToken"`
}

type logoutRequest struct {
	RefreshToken string `json:"refreshToken"`
}

type feishuCallbackRequest struct {
	Code        string `json:"code"`
	State       string `json:"state"`
	RedirectURI string `json:"redirectUri"`
}

// Login POST /auth/login（公开）。
func (h *Handler) Login(w http.ResponseWriter, r *http.Request) {
	var req loginRequest
	if err := decodeJSON(r, &req); err != nil {
		badRequest(w)
		return
	}
	result, err := h.auth.Login(r.Context(), dto.LoginCmd{UserName: req.UserName, Password: req.Password})
	if err != nil {
		httpx.WriteAppError(w, err)
		return
	}
	httpx.WriteData(w, http.StatusOK, result)
}

// Refresh POST /auth/refresh（公开，凭 refresh）。
func (h *Handler) Refresh(w http.ResponseWriter, r *http.Request) {
	var req refreshRequest
	if err := decodeJSON(r, &req); err != nil {
		badRequest(w)
		return
	}
	pair, err := h.auth.Refresh(r.Context(), dto.RefreshCmd{RefreshToken: req.RefreshToken})
	if err != nil {
		httpx.WriteAppError(w, err)
		return
	}
	httpx.WriteData(w, http.StatusOK, pair)
}

// Logout POST /auth/logout（需登录）。
func (h *Handler) Logout(w http.ResponseWriter, r *http.Request) {
	var req logoutRequest
	if err := decodeJSON(r, &req); err != nil {
		badRequest(w)
		return
	}
	actor := int64(0)
	if ac, ok := adminapp.AuthContextFrom(r.Context()); ok {
		actor = ac.UserID
	}
	if err := h.auth.Logout(r.Context(), actor, dto.LogoutCmd{RefreshToken: req.RefreshToken}); err != nil {
		httpx.WriteAppError(w, err)
		return
	}
	httpx.WriteData(w, http.StatusOK, map[string]any{"loggedOut": true})
}

// FeishuCallback POST /auth/feishu/callback（公开）。
func (h *Handler) FeishuCallback(w http.ResponseWriter, r *http.Request) {
	var req feishuCallbackRequest
	if err := decodeJSON(r, &req); err != nil {
		badRequest(w)
		return
	}
	result, err := h.auth.FeishuCallback(r.Context(), dto.FeishuCallbackCmd{
		Code: req.Code, State: req.State, RedirectURI: req.RedirectURI,
	})
	if err != nil {
		httpx.WriteAppError(w, err)
		return
	}
	httpx.WriteData(w, http.StatusOK, result)
}

// Me GET /me（需登录，无需特定权限码）。
func (h *Handler) Me(w http.ResponseWriter, r *http.Request) {
	ac, ok := adminapp.AuthContextFrom(r.Context())
	if !ok {
		httpx.WriteError(w, http.StatusUnauthorized, httpx.CodeUnauthenticated, "未认证")
		return
	}
	view, err := h.auth.Me(r.Context(), ac.UserID)
	if err != nil {
		httpx.WriteAppError(w, err)
		return
	}
	httpx.WriteData(w, http.StatusOK, view)
}
