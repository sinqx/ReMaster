package errors

import (
	"errors" // stdlib для As/Is
	"fmt"
	"net/http"
)

type ErrorType int

const (
	ErrorTypeValidation ErrorType = iota
	ErrorTypeNotFound
	ErrorTypeConflict
	ErrorTypeUnauthorized
	ErrorTypeForbidden
	ErrorTypeBadRequest
	ErrorTypeRateLimit
	ErrorTypeInternal
	ErrorTypeDatabase
)

type AppError struct {
	Type       ErrorType
	Code       string
	Message    string
	Cause      error
	StatusCode int
	Details    map[string]string
}

func (e *AppError) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("%s: %v", e.Message, e.Cause)
	}
	return e.Message
}

func (e *AppError) Unwrap() error {
	return e.Cause
}

type ValidationError struct{ AppError }
type ConflictError struct{ AppError }
type DatabaseError struct{ AppError }
type InternalError struct{ AppError }
type NotFoundError struct{ AppError }
type UnauthorizedError struct{ AppError }
type ForbiddenError struct{ AppError }
type BadRequestError struct{ AppError }
type RateLimitError struct{ AppError }

func NewValidationError(msg string, details map[string]string) *ValidationError {
	return &ValidationError{AppError{
		Type:       ErrorTypeValidation,
		Code:       "VALIDATION_ERROR",
		Message:    msg,
		StatusCode: http.StatusBadRequest,
		Details:    details,
	}}
}

func NewConflictError(msg string, cause error) *ConflictError {
	return &ConflictError{AppError{
		Type:       ErrorTypeConflict,
		Code:       "CONFLICT_ERROR",
		Message:    msg,
		Cause:      cause,
		StatusCode: http.StatusConflict,
	}}
}

func NewDatabaseError(msg string, cause error) *DatabaseError {
	return &DatabaseError{AppError{
		Type:       ErrorTypeDatabase,
		Code:       "DATABASE_ERROR",
		Message:    msg,
		Cause:      cause,
		StatusCode: http.StatusInternalServerError,
	}}
}

func NewInternalError(msg string, cause error) *InternalError {
	return &InternalError{AppError{
		Type:       ErrorTypeInternal,
		Code:       "INTERNAL_ERROR",
		Message:    msg,
		Cause:      cause,
		StatusCode: http.StatusInternalServerError,
	}}
}

func NewNotFoundError(msg string) *NotFoundError {
	return &NotFoundError{AppError{
		Type:       ErrorTypeNotFound,
		Code:       "NOT_FOUND",
		Message:    msg,
		StatusCode: http.StatusNotFound,
	}}
}

func NewUnauthorizedError(msg string) *UnauthorizedError {
	return &UnauthorizedError{AppError{
		Type:       ErrorTypeUnauthorized,
		Code:       "UNAUTHORIZED",
		Message:    msg,
		StatusCode: http.StatusUnauthorized,
	}}
}

func NewForbiddenError(msg string) *ForbiddenError {
	return &ForbiddenError{AppError{
		Type:       ErrorTypeForbidden,
		Code:       "FORBIDDEN",
		Message:    msg,
		StatusCode: http.StatusForbidden,
	}}
}

func NewBadRequestError(msg string) *BadRequestError {
	return &BadRequestError{AppError{
		Type:       ErrorTypeBadRequest,
		Code:       "BAD_REQUEST",
		Message:    msg,
		StatusCode: http.StatusBadRequest,
	}}
}

func NewRateLimitError(msg string) *RateLimitError {
	return &RateLimitError{AppError{
		Type:       ErrorTypeRateLimit,
		Code:       "RATE_LIMIT_EXCEEDED",
		Message:    msg,
		StatusCode: http.StatusTooManyRequests,
	}}
}

func IsValidationError(err error) bool {
	var target *ValidationError
	return errors.As(err, &target)
}

func IsNotFoundError(err error) bool {
	var target *NotFoundError
	return errors.As(err, &target)
}

func IsConflictError(err error) bool {
	var target *ConflictError
	return errors.As(err, &target)
}

func IsUnauthorizedError(err error) bool {
	var target *UnauthorizedError
	return errors.As(err, &target)
}

func IsForbiddenError(err error) bool {
	var target *ForbiddenError
	return errors.As(err, &target)
}

func IsBadRequestError(err error) bool {
	var target *BadRequestError
	return errors.As(err, &target)
}

func IsRateLimitError(err error) bool {
	var target *RateLimitError
	return errors.As(err, &target)
}

func IsDatabaseError(err error) bool {
	var target *DatabaseError
	return errors.As(err, &target)
}

func IsInternalError(err error) bool {
	var target *InternalError
	return errors.As(err, &target)
}
