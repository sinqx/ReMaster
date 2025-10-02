package server

import (
	"context"
	"log/slog"
	"runtime/debug"
	"time"

	"github.com/google/uuid"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	"remaster/shared/logger"
)

// RecoveryUnary — catches panics, logs the stack, and returns an internal status (without panic details).
func RecoveryUnary(logger *slog.Logger) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (resp any, err error) {
		defer func() {
			if r := recover(); r != nil {
				stackBytes := debug.Stack()
				logger.Error("gRPC panic recovered",
					"method", info.FullMethod,
					"panic", r,
					"stack", string(stackBytes),
				)
				err = status.Error(codes.Internal, "internal server error")
			}
		}()
		return handler(ctx, req)
	}
}

// stream version
func RecoveryStream(logger *slog.Logger) grpc.StreamServerInterceptor {
	return func(srv any, ss grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) (err error) {
		defer func() {
			if r := recover(); r != nil {
				stackBytes := debug.Stack()
				logger.Error("gRPC stream panic recovered",
					"method", info.FullMethod,
					"panic", r,
					"stack", string(stackBytes),
				)
				err = status.Error(codes.Internal, "internal server error")
			}
		}()
		return handler(srv, ss)
	}
}

// CorrelationUnary — takes x-correlation-id or correlation-id headers and creates a correlation id.
func CorrelationUnary(baseLogger *slog.Logger) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		md, _ := metadata.FromIncomingContext(ctx)
		var cid string
		if v := md.Get("x-correlation-id"); len(v) > 0 {
			cid = v[0]
		} else if v := md.Get("correlation-id"); len(v) > 0 {
			cid = v[0]
		}
		if cid == "" {
			cid = uuid.New().String()
		}

		// create request logger and inject into context
		reqLogger := baseLogger.With(slog.String("correlation_id", cid))
		ctx = logger.ToContext(ctx, reqLogger)
		return handler(ctx, req)
	}
}

// LoggingUnary - logs gRPC calls and their duration. without body and auth headers
func LoggingUnary(baseLogger *slog.Logger) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (resp any, err error) {
		start := time.Now()
		reqLogger := logger.FromContext(ctx, baseLogger)

		resp, err = handler(ctx, req)
		duration := time.Since(start)

		if err != nil {
			reqLogger.LogAttrs(ctx, slog.LevelError, "gRPC call failed",
				slog.String("method", info.FullMethod),
				slog.Duration("duration", duration),
				slog.Any("error", err),
			)
		} else {
			reqLogger.LogAttrs(ctx, slog.LevelInfo, "gRPC call completed",
				slog.String("method", info.FullMethod),
				slog.Duration("duration", duration),
			)
		}
		return resp, err
	}
}
