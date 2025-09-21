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
	appErr, ok := err.(*AppError)
	if !ok {
		// Fallback для non-AppError
		eh.logError(c.Request.Context(), err, c.Request.Method, c.Request.URL.RequestURI())
		response := ErrorResponse{
			Success: false,
			Error:   "Internal server error",
			Code:    "INTERNAL_ERROR",
		}
		c.JSON(http.StatusInternalServerError, response)
		return
	}

	eh.logError(c.Request.Context(), appErr, c.Request.Method, c.Request.URL.RequestURI())

	response := ErrorResponse{
		Success: false,
		Error:   appErr.Message,
		Code:    appErr.Code,
		Details: appErr.Details, // Используем из AppError
	}

	c.JSON(appErr.StatusCode, response)
}

// gRPC Error Handlers
func (eh *ErrorHandler) HandleGrpcError(ctx context.Context, err error) error {
	appErr, ok := err.(*AppError)
	if !ok {
		eh.logError(ctx, err, "gRPC", "")
		return status.Error(codes.Internal, "Internal server error")
	}

	eh.logError(ctx, appErr, "gRPC", "")

	grpcCode := eh.mapToGrpcCode(appErr.Type)
	return status.Error(grpcCode, appErr.Message)
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
	case ErrorTypeDatabase: // Добавил
		return codes.Internal
	default:
		return codes.Internal
	}
}

func (eh *ErrorHandler) HandleGrpcToHttp(c *gin.Context, err error) {
	st, ok := status.FromError(err)
	if !ok {
		eh.logger.Error("non-grpc error", "error", err)
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Success: false,
			Error:   "Internal server error",
			Code:    "INTERNAL_ERROR",
		})
		return
	}

	httpStatus := eh.grpcToHTTP(st.Code())

	details := map[string]string{}
	if appErr, ok := err.(*AppError); ok {
		details = appErr.Details
	} else if st.Code() == codes.AlreadyExists {
		details["field"] = "email" // example
	} else if st.Code() == codes.InvalidArgument {
		details["validation"] = st.Message()
	}

	eh.logger.Error("grpc error", "code", st.Code(), "message", st.Message(), "path", c.Request.URL.Path)

	c.JSON(httpStatus, ErrorResponse{
		Success: false,
		Error:   st.Message(),
		Code:    st.Code().String(),
		Details: details,
	})
}

func (eh *ErrorHandler) logError(ctx context.Context, err error, method, uri string) {
	if appErr, ok := err.(*AppError); ok {
		logLevel := slog.LevelWarn
		if appErr.Type == ErrorTypeInternal || appErr.Type == ErrorTypeDatabase {
			logLevel = slog.LevelError
		}
		eh.logger.Log(ctx, logLevel, "Error occurred",
			"error", err.Error(),
			"type", appErr.Type,
			"code", appErr.Code,
			"method", method,
			"uri", uri,
			"cause", appErr.Cause,
			"details", appErr.Details, // Добавил логирование details
		)
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

func (eh *ErrorHandler) grpcToHTTP(code codes.Code) int {
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
