package errors

import (
	"context"
	"log/slog"
	"net/http"
	"remaster/shared/logger"

	"github.com/gin-gonic/gin"
	"google.golang.org/genproto/googleapis/rpc/errdetails"
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

// ---- HTTP error handling ----
func (eh *ErrorHandler) HandleGinError(c *gin.Context, err error) {
	requestLogger := logger.FromContext(c.Request.Context(), eh.logger)

	appErr, ok := err.(*AppError)
	if !ok {
		eh.logError(c.Request.Context(), requestLogger, err)
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Success: false,
			Error:   "Internal server error",
			Code:    "INTERNAL_ERROR",
		})
		return
	}

	eh.logError(c.Request.Context(), requestLogger, appErr)
	c.JSON(appErr.StatusCode, ErrorResponse{
		Success: false,
		Error:   appErr.Message,
		Code:    appErr.Code,
		Details: appErr.Details,
	})
}

// ---- gRPC → gRPC ----
func (eh *ErrorHandler) HandleGrpcError(ctx context.Context, err error) error {
	requestLogger := logger.FromContext(ctx, eh.logger)

	appErr, ok := err.(*AppError)
	if !ok {
		requestLogger.ErrorContext(ctx, "non-app error", slog.Any("error", err))
		return status.Error(codes.Internal, "Internal server error")
	}

	eh.logError(ctx, requestLogger, appErr)

	grpcCode := ErrorType.mapToGrpcCode(appErr.Type)
	st := status.New(grpcCode, appErr.Message)

	detail := &errdetails.ErrorInfo{
		Reason:   appErr.Code,
		Metadata: appErr.Details,
	}
	stWithDetails, err2 := st.WithDetails(detail)
	if err2 != nil {
		return st.Err()
	}
	return stWithDetails.Err()
}

// ---- gRPC → HTTP (для API Gateway) ----
func (eh *ErrorHandler) HandleGrpcToHttp(c *gin.Context, err error) {
	requestLogger := logger.FromContext(c.Request.Context(), eh.logger)

	st, ok := status.FromError(err)
	if !ok {
		requestLogger.ErrorContext(c.Request.Context(), "failed to convert gRPC error to status",
			slog.Any("error", err))

		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Success: false,
			Error:   "Internal server error",
			Code:    "INTERNAL_ERROR",
		})
		return
	}

	httpStatus := grpcToHTTP(st.Code())
	resp := ErrorResponse{
		Success: false,
		Error:   st.Message(),
		Code:    st.Code().String(),
	}

	for _, d := range st.Details() {
		switch info := d.(type) {
		case *errdetails.ErrorInfo:
			if info.Reason != "" {
				resp.Code = info.Reason
			}
			if len(info.Metadata) > 0 {
				resp.Details = info.Metadata
			}
		}
	}

	c.JSON(httpStatus, resp)
}

func (eh *ErrorHandler) logError(ctx context.Context, logger *slog.Logger, err error) {
	if appErr, ok := err.(*AppError); ok {
		logLevel := slog.LevelWarn
		if appErr.Type == ErrorTypeInternal || appErr.Type == ErrorTypeDatabase {
			logLevel = slog.LevelError
		}

		logAttrs := []slog.Attr{
			slog.String("error_type", string(appErr.Type.String())),
			slog.String("error_code", appErr.Code),
			slog.Any("details", appErr.Details),
		}

		if appErr.Cause != nil {
			logAttrs = append(logAttrs, slog.String("cause", appErr.Cause.Error()))
		}

		logger.LogAttrs(ctx, logLevel, appErr.Message, logAttrs...)
		return
	}

	logger.ErrorContext(ctx, "Unexpected error occurred", slog.Any("error", err))
}
