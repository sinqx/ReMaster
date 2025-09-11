package errors

import (
	"context"
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type ErrorHandler struct {
	logger *slog.Logger
}

func NewErrorHandler(logger *slog.Logger) *ErrorHandler {
	return &ErrorHandler{
		logger: logger,
	}
}

// Response structure HTTP 
type ErrorResponse struct {
	Success bool              `json:"success"`
	Error   string            `json:"error"`
	Code    string            `json:"code"`
	Details map[string]string `json:"details,omitempty"`
}

// HTTP Error Handlers
func (eh *ErrorHandler) HandleGinError(c *gin.Context, err error) {
	if appErr, ok := err.(*AppError); ok {
		eh.logError(c.Request.Context(), appErr, c.Request.Method, c.Request.URL.RequestURI())

		response := ErrorResponse{
			Success: false,
			Error:   appErr.Message,
			Code:    appErr.Code,
			Details: appErr.Details,
		}

		c.JSON(appErr.StatusCode, response)
		return
	}

	eh.logError(c.Request.Context(), err, c.Request.Method, c.Request.URL.RequestURI())

	response := ErrorResponse{
		Success: false,
		Error:   "Internal server error",
		Code:    "INTERNAL_ERROR",
	}

	c.JSON(http.StatusInternalServerError, response)
}

// gRPC Error Handlers
func (eh *ErrorHandler) HandleGrpcError(ctx context.Context, err error) error {
	if appErr, ok := err.(*AppError); ok {
		eh.logError(ctx, appErr, "gRPC", "")

		grpcCode := eh.mapToGrpcCode(appErr.Type)
		return status.Error(grpcCode, appErr.Message)
	}

	eh.logError(ctx, err, "gRPC", "")
	return status.Error(codes.Internal, "Internal server error")
}

func (eh *ErrorHandler) mapToGrpcCode(errorType ErrorType) codes.Code {
	switch errorType {
	case ErrorTypeValidation:
		return codes.InvalidArgument
	case ErrorTypeNotFound:
		return codes.NotFound
	case ErrorTypeConflict:
		return codes.AlreadyExists
	case ErrorTypeUnauthorized:
		return codes.Unauthenticated
	case ErrorTypeForbidden:
		return codes.PermissionDenied
	case ErrorTypeBadRequest:
		return codes.InvalidArgument
	case ErrorTypeRateLimit:
		return codes.ResourceExhausted
	default:
		return codes.Internal
	}
}

func (eh *ErrorHandler) logError(ctx context.Context, err error, method, uri string) {
	if appErr, ok := err.(*AppError); ok {
		if appErr.Type == ErrorTypeInternal {
			eh.logger.Error("Internal error occurred",
				"error", err.Error(),
				"type", appErr.Type,
				"code", appErr.Code,
				"method", method,
				"uri", uri,
				"cause", appErr.Cause,
			)
		} else {
			eh.logger.Warn("User error occurred",
				"error", err.Error(),
				"type", appErr.Type,
				"code", appErr.Code,
				"method", method,
				"uri", uri,
			)
		}
		return
	}

	eh.logger.Error("Unexpected error occurred",
		"error", err.Error(),
		"method", method,
		"uri", uri,
	)
}

func (eh *ErrorHandler) GinErrorMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Next()

		if len(c.Errors) > 0 {
			err := c.Errors.Last().Err
			eh.HandleGinError(c, err)
		}
	}
}
