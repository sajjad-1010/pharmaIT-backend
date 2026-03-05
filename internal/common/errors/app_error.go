package errors

import "net/http"

type AppError struct {
	Code       string      `json:"code"`
	Message    string      `json:"message"`
	Details    interface{} `json:"details,omitempty"`
	HTTPStatus int         `json:"-"`
}

func (e AppError) Error() string {
	return e.Message
}

func New(httpStatus int, code, message string, details interface{}) AppError {
	return AppError{
		Code:       code,
		Message:    message,
		Details:    details,
		HTTPStatus: httpStatus,
	}
}

func BadRequest(code, message string, details interface{}) AppError {
	return New(http.StatusBadRequest, code, message, details)
}

func Unauthorized(code, message string) AppError {
	return New(http.StatusUnauthorized, code, message, nil)
}

func Forbidden(code, message string) AppError {
	return New(http.StatusForbidden, code, message, nil)
}

func NotFound(code, message string) AppError {
	return New(http.StatusNotFound, code, message, nil)
}

func Conflict(code, message string, details interface{}) AppError {
	return New(http.StatusConflict, code, message, details)
}

func Internal(message string) AppError {
	return New(http.StatusInternalServerError, "INTERNAL_ERROR", message, nil)
}

