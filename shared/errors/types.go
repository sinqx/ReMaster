package errors

import (
	"fmt"
	"net/http"
)

type ErrorType string

const (
	ErrorTypeValidation   ErrorType = "validation"
	ErrorTypeNotFound     ErrorType = "not_found"
	ErrorTypeConflict     ErrorType = "conflict"
	ErrorTypeUnauthorized ErrorType = "unauthorized"
	ErrorTypeForbidden    ErrorType = "forbidden"
	ErrorTypeInternal     ErrorType = "internal"
	ErrorTypeBadRequest   ErrorType = "bad_request"
	ErrorTypeRateLimit    ErrorType = "rate_limit"
)

type AppError struct {
	Type       ErrorType         `json:"type"`
	Message    string            `json:"message"`
	Code       string            `json:"code"`
	StatusCode int               `json:"status_code"`
	Details    map[string]string `json:"details,omitempty"`
	Cause      error             `json:"-"`
}

func (e *AppError) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("%s: %v", e.Message, e.Cause)
	}
	return e.Message
}

func NewValidationError(message string, details map[string]string) *AppError {
	return &AppError{
		Type:       ErrorTypeValidation,
		Message:    message,
		Code:       "VALIDATION_FAILED",
		StatusCode: http.StatusUnprocessableEntity,
		Details:    details,
	}
}

func NewNotFoundError(message string) *AppError {
	return &AppError{
		Type:       ErrorTypeNotFound,
		Message:    message,
		Code:       "NOT_FOUND",
		StatusCode: http.StatusNotFound,
	}
}

func NewConflictError(message string) *AppError {
	return &AppError{
		Type:       ErrorTypeConflict,
		Message:    message,
		Code:       "CONFLICT",
		StatusCode: http.StatusConflict,
	}
}

func NewUnauthorizedError(message string) *AppError {
	return &AppError{
		Type:       ErrorTypeUnauthorized,
		Message:    message,
		Code:       "UNAUTHORIZED",
		StatusCode: http.StatusUnauthorized,
	}
}

func NewForbiddenError(message string) *AppError {
	return &AppError{
		Type:       ErrorTypeForbidden,
		Message:    message,
		Code:       "FORBIDDEN",
		StatusCode: http.StatusForbidden,
	}
}

func NewInternalError(message string, cause error) *AppError {
	return &AppError{
		Type:       ErrorTypeInternal,
		Message:    message,
		Code:       "INTERNAL_ERROR",
		StatusCode: http.StatusInternalServerError,
		Cause:      cause,
	}
}

func NewBadRequestError(message string) *AppError {
	return &AppError{
		Type:       ErrorTypeBadRequest,
		Message:    message,
		Code:       "BAD_REQUEST",
		StatusCode: http.StatusBadRequest,
	}
}

func NewRateLimitError(message string) *AppError {
	return &AppError{
		Type:       ErrorTypeRateLimit,
		Message:    message,
		Code:       "RATE_LIMIT_EXCEEDED",
		StatusCode: http.StatusTooManyRequests,
	}
}

func IsValidationError(err error) bool {
	if appErr, ok := err.(*AppError); ok {
		return appErr.Type == ErrorTypeValidation
	}
	return false
}

func IsNotFoundError(err error) bool {
	if appErr, ok := err.(*AppError); ok {
		return appErr.Type == ErrorTypeNotFound
	}
	return false
}

func IsConflictError(err error) bool {
	if appErr, ok := err.(*AppError); ok {
		return appErr.Type == ErrorTypeConflict
	}
	return false
}

func IsUnauthorizedError(err error) bool {
	if appErr, ok := err.(*AppError); ok {
		return appErr.Type == ErrorTypeUnauthorized
	}
	return false
}
