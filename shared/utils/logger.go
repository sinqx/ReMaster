package logger

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"time"

	config "remaster/shared"

	"github.com/fatih/color" // For colored output
)

// Custom attributes for logging context
type logContextKey string

const (
	serviceKey   logContextKey = "service"
	requestIDKey logContextKey = "request_id"
)

// PrettyHandler is a custom handler for pretty output
type PrettyHandler struct {
	handler slog.Handler
}

func (h PrettyHandler) Handle(ctx context.Context, r slog.Record) error {
	// Define color based on level
	var levelColor func(format string, a ...interface{}) string
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

	// Get time with color
	timeColor := color.New(color.FgWhite, color.Faint).SprintFunc()
	timestamp := timeColor(r.Time.Format(time.RFC3339))

	// Get service and request_id from context if available
	service := ctx.Value(serviceKey)
	requestID := ctx.Value(requestIDKey)

	// Format message
	msg := r.Message
	if service != nil {
		msg = fmt.Sprintf("[%s] %s", service, msg)
	}
	if requestID != nil {
		msg = fmt.Sprintf("%s [req:%s]", msg, requestID)
	}

	// Collect attributes
	var attrs []string
	r.Attrs(func(a slog.Attr) bool {
		// Skip system attributes if needed
		if a.Key == "time" || a.Key == "level" {
			return true
		}
		attrs = append(attrs, fmt.Sprintf("%s=%v", a.Key, a.Value.Any()))
		return true
	})
	attrStr := strings.Join(attrs, " ")

	// Form the final string
	output := fmt.Sprintf("%s %s %s %s",
		timestamp,
		levelColor(r.Level.String()),
		msg,
		attrStr,
	)

	// Output to stdout
	fmt.Fprintln(os.Stdout, output)
	return nil
}

// New creates a new logger with a custom handler
func New(cfg config.LogConfig) *slog.Logger {
	// Set logging level
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

	// Configure base handler
	opts := &slog.HandlerOptions{
		Level: level,
	}

	// Create custom handler
	handler := PrettyHandler{
		handler: slog.NewTextHandler(os.Stdout, opts),
	}

	// Wrap in slog.Logger
	logger := slog.New(handler)

	// Add global context (optional)
	return logger.With(
		slog.String("app", "remaster"),
	)
}

// WithService adds the service name to the logger context
func WithService(logger *slog.Logger, service string) *slog.Logger {
	return logger.With(slog.String(string(serviceKey), service))
}

// WithRequestID adds the request ID to the logger context
func WithRequestID(logger *slog.Logger, requestID string) *slog.Logger {
	return logger.With(slog.String(string(requestIDKey), requestID))
}

func (h PrettyHandler) Enabled(ctx context.Context, level slog.Level) bool {
	return h.handler.Enabled(ctx, level)
}
func (h PrettyHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return PrettyHandler{handler: h.handler.WithAttrs(attrs)}
}
func (h PrettyHandler) WithGroup(name string) slog.Handler {
	return PrettyHandler{handler: h.handler.WithGroup(name)}
}
