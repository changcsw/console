// Package httpx 提供统一响应包络与全局错误码映射（00 §7.2/§7.4）。
package httpx

import (
	"encoding/json"
	"errors"
	"net/http"

	adminapp "github.com/csw/console/services/admin-api/internal/app/admin"
)

// 全局错误码（00 §7.4）。
const (
	CodeUnauthenticated = "UNAUTHENTICATED"
	CodeForbidden       = "FORBIDDEN"
	CodeNotFound        = "NOT_FOUND"
	CodeValidation      = "VALIDATION_FAILED"
	CodeConflict        = "CONFLICT"
	CodeInternal        = "INTERNAL"
)

// ErrorBody 错误包络体。
type ErrorBody struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Details []any  `json:"details"`
}

type errorEnvelope struct {
	Error ErrorBody `json:"error"`
}

type dataEnvelope struct {
	Data any `json:"data"`
}

// WriteData 写成功包络 { "data": ... }。
func WriteData(w http.ResponseWriter, status int, data any) {
	writeJSON(w, status, dataEnvelope{Data: data})
}

// WriteError 写错误包络 { "error": {code,message,details} }。
func WriteError(w http.ResponseWriter, status int, code, message string, details ...any) {
	if details == nil {
		details = []any{}
	}
	writeJSON(w, status, errorEnvelope{Error: ErrorBody{Code: code, Message: message, Details: details}})
}

// WriteAppError 把 app 层哨兵错误映射为标准错误码与 HTTP 状态。
func WriteAppError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, adminapp.ErrUnauthenticated):
		WriteError(w, http.StatusUnauthorized, CodeUnauthenticated, "用户名或密码错误")
	case errors.Is(err, adminapp.ErrValidation):
		WriteError(w, http.StatusBadRequest, CodeValidation, "入参校验失败")
	case errors.Is(err, adminapp.ErrNotFound):
		WriteError(w, http.StatusNotFound, CodeNotFound, "资源不存在")
	case errors.Is(err, adminapp.ErrConflict):
		WriteError(w, http.StatusConflict, CodeConflict, "唯一性或状态冲突")
	default:
		WriteError(w, http.StatusInternalServerError, CodeInternal, "服务端内部错误")
	}
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}
