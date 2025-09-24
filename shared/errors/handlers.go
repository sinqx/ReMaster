package errors

import (
	"context"
	"errors"
	"log/slog"
	"net/http"

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
	Success bool   `json:"success"`
	Error   string `json:"error"`
	Code    string `json:"code"`
	Details any    `json:"details,omitempty"`
}

// ---- HTTP error handling ----
func (eh *ErrorHandler) HandleGinError(c *gin.Context, err error) {
	appErr, ok := err.(*AppError)
	if !ok {
		eh.logError(c.Request.Context(), eh.logger, err)
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Success: false,
			Error:   "Internal server error",
			Code:    "INTERNAL_ERROR",
		})
		return
	}

	eh.logError(c.Request.Context(), eh.logger, appErr)
	c.JSON(appErr.StatusCode, ErrorResponse{
		Success: false,
		Error:   appErr.Message,
		Code:    appErr.Code,
		Details: appErr.Details,
	})
}

// ---- gRPC → gRPC ----
func (eh *ErrorHandler) HandleGrpcError(err error) error {
	var appErr *AppError
	if errors.As(err, &appErr) {
		return status.Error(appErr.Type.ToGrpcCode(), appErr.Message)
	}
	// fallback
	return status.Error(codes.Internal, "Internal server error")
}

func (eh *ErrorHandler) HandleHttpError(c *gin.Context, err error) {
	var appErr *AppError
	if errors.As(err, &appErr) {
		eh.logError(c.Request.Context(), eh.logger, err)

		response := ErrorResponse{
			Success: false,
			Error:   appErr.Message,
			Code:    appErr.Code,
			Details: appErr.Details,
		}

		c.JSON(appErr.StatusCode, response)
		return
	}

	eh.logger.ErrorContext(c.Request.Context(),
		"Unknown error occurred",
		slog.Any("error", err),
	)

	response := ErrorResponse{
		Success: false,
		Error:   "Internal server error",
		Code:    "INTERNAL_ERROR",
	}

	c.JSON(http.StatusInternalServerError, response)
}

// ---- gRPC → HTTP (для API Gateway) ----
func (eh *ErrorHandler) HandleGrpcToHttp(c *gin.Context, err error) {
	st, ok := status.FromError(err)
	if !ok {
		eh.logger.ErrorContext(c.Request.Context(),
			"failed to convert gRPC error to status",
			slog.Any("error", err),
		)

		resp := ErrorResponse{
			Success: false,
			Error:   "Internal server error",
			Code:    "INTERNAL_ERROR",
		}

		c.JSON(http.StatusInternalServerError, resp)
		return
	}

	httpStatus := GrpcToHTTP(st.Code())
	resp := ErrorResponse{
		Success: false,
		Error:   st.Message(),
		Code:    st.Code().String(),
	}

	details := make(map[string]any)
	for _, d := range st.Details() {
		switch info := d.(type) {
		case *errdetails.ErrorInfo:
			if info.Reason != "" {
				resp.Code = info.Reason
			}
			if len(info.Metadata) > 0 {
				for k, v := range info.Metadata {
					details[k] = v
				}
			}
		case *errdetails.BadRequest:
			violations := make(map[string]string)
			for _, violation := range info.FieldViolations {
				violations[violation.Field] = violation.Description
			}
			if len(violations) > 0 {
				details["field_violations"] = violations
			}
		case *errdetails.ResourceInfo:
			details["resource_type"] = info.ResourceType
			details["resource_name"] = info.ResourceName
		}
	}

	if len(details) > 0 {
		resp.Details = details
	}

	logLevel := slog.LevelInfo
	if httpStatus >= 500 {
		logLevel = slog.LevelError
	} else if httpStatus >= 400 {
		logLevel = slog.LevelWarn
	}

	eh.logger.Log(c.Request.Context(), logLevel,
		"gRPC error converted to HTTP",
		slog.String("grpc_code", st.Code().String()),
		slog.Int("http_status", httpStatus),
		slog.String("message", st.Message()),
	)

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
