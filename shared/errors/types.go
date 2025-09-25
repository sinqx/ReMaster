package errors

import (
	"errors"
	"fmt"
	"net/http"

	"google.golang.org/grpc/codes"
)

// Error types
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

// Universal application error
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

// Factories for different types of errors
func NewAppError(errType ErrorType, code, msg string, status int, cause error, details map[string]string) *AppError {
	return &AppError{
		Type:       errType,
		Code:       code,
		Message:    msg,
		Cause:      cause,
		StatusCode: status,
		Details:    details,
	}
}

func NewValidationError(msg string, details map[string]string) *AppError {
	return NewAppError(ErrorTypeValidation, "VALIDATION_ERROR", msg, http.StatusBadRequest, nil, details)
}

func NewConflictError(msg string, cause error) *AppError {
	return NewAppError(ErrorTypeConflict, "CONFLICT_ERROR", msg, http.StatusConflict, cause, nil)
}

func NewDatabaseError(msg string, cause error) *AppError {
	return NewAppError(ErrorTypeDatabase, "DATABASE_ERROR", msg, http.StatusInternalServerError, cause, nil)
}

func NewInternalError(msg string, cause error) *AppError {
	return NewAppError(ErrorTypeInternal, "INTERNAL_ERROR", msg, http.StatusInternalServerError, cause, nil)
}

func NewNotFoundError(msg string, cause error) *AppError {
	return NewAppError(ErrorTypeNotFound, "NOT_FOUND", msg, http.StatusNotFound, nil, nil)
}

func NewUnauthorizedError(msg string) *AppError {
	return NewAppError(ErrorTypeUnauthorized, "UNAUTHORIZED", msg, http.StatusUnauthorized, nil, nil)
}

func NewForbiddenError(msg string) *AppError {
	return NewAppError(ErrorTypeForbidden, "FORBIDDEN", msg, http.StatusForbidden, nil, nil)
}

func NewBadRequestError(msg string) *AppError {
	return NewAppError(ErrorTypeBadRequest, "BAD_REQUEST", msg, http.StatusBadRequest, nil, nil)
}

func NewRateLimitError(msg string) *AppError {
	return NewAppError(ErrorTypeRateLimit, "RATE_LIMIT_EXCEEDED", msg, http.StatusTooManyRequests, nil, nil)
}

func NewPermissionError(msg string) *AppError {
	return NewAppError(ErrorTypeForbidden, "PERMISSION_ERROR", msg, http.StatusForbidden, nil, nil)
}

// -------- Mapping --------

func (et ErrorType) String() string {
	switch et {
	case ErrorTypeValidation:
		return "validation"
	case ErrorTypeNotFound:
		return "not_found"
	case ErrorTypeConflict:
		return "conflict"
	case ErrorTypeUnauthorized:
		return "unauthorized"
	case ErrorTypeForbidden:
		return "forbidden"
	case ErrorTypeBadRequest:
		return "bad_request"
	case ErrorTypeRateLimit:
		return "rate_limit"
	case ErrorTypeInternal:
		return "internal"
	case ErrorTypeDatabase:
		return "database"
	default:
		return "unknown"
	}
}

// ErrorType -> gRPC Code
func (et ErrorType) ToGrpcCode() codes.Code {
	switch et {
	case ErrorTypeConflict:
		return codes.AlreadyExists
	case ErrorTypeNotFound:
		return codes.NotFound
	case ErrorTypeValidation, ErrorTypeBadRequest:
		return codes.InvalidArgument
	case ErrorTypeUnauthorized:
		return codes.Unauthenticated
	case ErrorTypeForbidden:
		return codes.PermissionDenied
	case ErrorTypeRateLimit:
		return codes.ResourceExhausted
	case ErrorTypeDatabase, ErrorTypeInternal:
		return codes.Internal
	default:
		return codes.Internal
	}
}

// gRPC Code -> HTTP Status
func GrpcToHTTP(code codes.Code) int {
	switch code {
	case codes.OK:
		return http.StatusOK
	case codes.Canceled:
		return http.StatusRequestTimeout
	case codes.InvalidArgument, codes.FailedPrecondition, codes.OutOfRange:
		return http.StatusBadRequest
	case codes.DeadlineExceeded:
		return http.StatusGatewayTimeout
	case codes.NotFound:
		return http.StatusNotFound
	case codes.AlreadyExists, codes.Aborted:
		return http.StatusConflict
	case codes.PermissionDenied:
		return http.StatusForbidden
	case codes.ResourceExhausted:
		return http.StatusTooManyRequests
	case codes.Unimplemented:
		return http.StatusNotImplemented
	case codes.Internal, codes.DataLoss, codes.Unknown:
		return http.StatusInternalServerError
	case codes.Unavailable:
		return http.StatusServiceUnavailable
	case codes.Unauthenticated:
		return http.StatusUnauthorized
	default:
		return http.StatusInternalServerError
	}
}

// -------- Helper --------

func AsAppError(err error) (*AppError, bool) {
	var appErr *AppError
	if errors.As(err, &appErr) {
		return appErr, true
	}
	return nil, false
}
