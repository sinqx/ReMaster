package logger

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"sort"
	"strings"
	"sync"

	config "remaster/shared"

	"github.com/fatih/color"
)

// Context keys for request tracing
const (
	CorrelationIDKey = "correlation_id"
	ServiceNameKey   = "service"
	RequestIDKey     = "request_id"
)

type PrettyHandler struct {
}

var (
	once     sync.Once
	instance *slog.Logger
)

func init() {
	color.NoColor = false
}

func Get(cfg config.LogConfig) *slog.Logger {
	once.Do(func() {
		instance = New(cfg)
	})
	return instance
}

func (h PrettyHandler) Handle(ctx context.Context, r slog.Record) error {
	var levelColor func(format string, a ...any) string
	switch r.Level {
	case slog.LevelDebug:
		levelColor = color.New(color.FgCyan).SprintfFunc()
	case slog.LevelInfo:
		levelColor = color.New(color.FgGreen).SprintfFunc()
	case slog.LevelWarn:
		levelColor = color.New(color.FgYellow).SprintfFunc()
	case slog.LevelError:
		levelColor = color.New(color.FgRed).SprintfFunc()
	default:
		levelColor = color.New(color.FgWhite).SprintfFunc()
	}

	timeColor := color.New(color.FgWhite, color.Faint).SprintFunc()
	timestamp := timeColor(r.Time.Format("2006/01/02 15:04:05"))

	var correlationID, service, requestID string
	attrs := map[string]any{}

	r.Attrs(func(a slog.Attr) bool {
		switch a.Key {
		case CorrelationIDKey:
			correlationID = a.Value.String()
		case ServiceNameKey:
			service = a.Value.String()
		case RequestIDKey:
			requestID = a.Value.String()
		case "time", "level":
		default:
			attrs[a.Key] = a.Value.Any()
		}
		return true
	})

	header := fmt.Sprintf("%s %s", timestamp, levelColor("%-5s", r.Level.String()))

	var contextParts []string
	if service != "" {
		contextParts = append(contextParts, fmt.Sprintf("svc:%s", service))
	}
	if correlationID != "" {
		contextParts = append(contextParts, fmt.Sprintf("cid:%s", correlationID))
	}
	if requestID != "" {
		contextParts = append(contextParts, fmt.Sprintf("rid:%s", requestID))
	}
	contextStr := ""
	if len(contextParts) > 0 {
		contextStr = "[" + strings.Join(contextParts, " ") + "] "
	}

	msg := fmt.Sprintf("%s %s%s", header, contextStr, r.Message)

	if len(attrs) > 0 {
		attrLines := []string{}
		for k, v := range attrs {
			attrLines = append(attrLines, fmt.Sprintf("    %-12s: %v", k, v))
		}
		sort.Strings(attrLines)
		msg += "\n" + strings.Join(attrLines, "\n")
	}

	fmt.Fprintln(os.Stdout, msg)
	return nil
}

func (h PrettyHandler) Enabled(ctx context.Context, level slog.Level) bool {
	return true
}

func (h PrettyHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return h
}

func (h PrettyHandler) WithGroup(name string) slog.Handler {
	return h
}

// New creates a new structured logger
func New(cfg config.LogConfig) *slog.Logger {
	var level slog.Level
	switch strings.ToLower(cfg.Level) {
	case "debug":
		level = slog.LevelDebug
	case "warn":
		level = slog.LevelWarn
	case "error":
		level = slog.LevelError
	default:
		level = slog.LevelInfo
	}

	opts := &slog.HandlerOptions{
		Level: level,
	}

	var handler slog.Handler
	if cfg.Format == "pretty" {
		handler = PrettyHandler{}
	} else {
		// Production JSON format
		handler = slog.NewJSONHandler(os.Stdout, opts)
	}

	return slog.New(handler).With(
		slog.String("app", "remaster"),
	)
}

// WithService adds service context to logger
func WithService(logger *slog.Logger, serviceName string) *slog.Logger {
	return logger.With(slog.String(ServiceNameKey, serviceName))
}

// WithCorrelationID adds correlation ID to logger
func WithCorrelationID(logger *slog.Logger, correlationID string) *slog.Logger {
	return logger.With(slog.String(CorrelationIDKey, correlationID))
}

// WithRequestID adds request ID to logger
func WithRequestID(logger *slog.Logger, requestID string) *slog.Logger {
	return logger.With(slog.String(RequestIDKey, requestID))
}

// FromContext extracts logger from context or returns default
func FromContext(ctx context.Context, defaultLogger *slog.Logger) *slog.Logger {
	if logger, ok := ctx.Value("logger").(*slog.Logger); ok {
		return logger
	}
	return defaultLogger
}

// ToContext adds logger to context
func ToContext(ctx context.Context, log *slog.Logger) context.Context {
	type contextKey string
	const loggerKey contextKey = "logger"
	return context.WithValue(ctx, loggerKey, log)
}
