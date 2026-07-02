package dashboard

import "net/http"

const (
	CodeValidation = "VALIDATION_FAILED"
	CodeInternal   = "INTERNAL"
)

type Error struct {
	Status  int
	Code    string
	Message string
	Details []any
}

func (e *Error) Error() string { return e.Message }

func validationErr(message string, details ...any) *Error {
	if details == nil {
		details = []any{}
	}
	return &Error{
		Status:  http.StatusBadRequest,
		Code:    CodeValidation,
		Message: message,
		Details: details,
	}
}
