// Package logging provides structured logging capabilities using Go's log/slog package
// following best practices from the Better Stack Community Guide.
package logging

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"runtime"
	"time"

	"github.com/c0deZ3R0/go-sync-kit/errors"
)

// Logger is our wrapper around slog.Logger with additional convenience methods
type Logger struct {
	*slog.Logger
}

// Config holds logger configuration
type Config struct {
	Level       string `json:"level"`       // debug, info, warn, error
	Format      string `json:"format"`      // text, json
	AddSource   bool   `json:"add_source"`  // whether to add source code information
	Environment string `json:"environment"` // dev, prod, test
}

// Default configuration
var DefaultConfig = Config{
	Level:       "info",
	Format:      "json",
	AddSource:   true,
	Environment: "dev",
}

// Global logger instance
var defaultLogger *Logger

// LogValuer implementations for consistent representation of custom types
type Operation string

func (o Operation) LogValue() slog.Value {
	return slog.StringValue(string(o))
}

type Component string

func (c Component) LogValue() slog.Value {
	return slog.StringValue(string(c))
}

// SyncErrorValuer provides structured logging for SyncError
type SyncErrorValuer struct {
	*errors.SyncError
}

func (e SyncErrorValuer) LogValue() slog.Value {
	attrs := []slog.Attr{
		slog.String("operation", string(e.Op)),
		slog.String("component", e.Component),
		slog.String("code", string(e.Code)),
		slog.String("kind", string(e.Kind)),
		slog.Bool("retryable", e.Retryable),
		slog.String("error", e.Err.Error()),
	}
	
	// Add metadata if present
	if e.Metadata != nil {
		metadataAttrs := make([]slog.Attr, 0, len(e.Metadata))
		for k, v := range e.Metadata {
			metadataAttrs = append(metadataAttrs, slog.Any(k, v))
		}
		attrs = append(attrs, slog.Any("metadata", slog.GroupValue(metadataAttrs...)))
	}
	
	return slog.GroupValue(attrs...)
}

// NewLogger creates a new logger with the provided configuration
func NewLogger(config Config) *Logger {
	var level slog.Level
	switch config.Level {
	case "debug":
		level = slog.LevelDebug
	case "info":
		level = slog.LevelInfo
	case "warn":
		level = slog.LevelWarn
	case "error":
		level = slog.LevelError
	default:
		level = slog.LevelInfo
	}

	opts := &slog.HandlerOptions{
		Level:     level,
		AddSource: config.AddSource,
	}

	var handler slog.Handler
	if config.Format == "text" || config.Environment == "dev" {
		handler = slog.NewTextHandler(os.Stdout, opts)
	} else {
		handler = slog.NewJSONHandler(os.Stdout, opts)
	}

	return &Logger{Logger: slog.New(handler)}
}

// Init initializes the global logger with the provided configuration
func Init(config Config) {
	defaultLogger = NewLogger(config)
	slog.SetDefault(defaultLogger.Logger)
}

// Default returns the default logger instance
func Default() *Logger {
	if defaultLogger == nil {
		Init(DefaultConfig)
	}
	return defaultLogger
}

// WithOperation creates a child logger with operation context
func (l *Logger) WithOperation(op Operation) *Logger {
	return &Logger{Logger: l.With(slog.Any("operation", op))}
}

// WithComponent creates a child logger with component context
func (l *Logger) WithComponent(component Component) *Logger {
	return &Logger{Logger: l.With(slog.Any("component", component))}
}

// WithContext creates a child logger with key-value context
func (l *Logger) WithContext(ctx context.Context, attrs ...slog.Attr) *Logger {
	// Extract common context values
	contextAttrs := make([]any, 0, len(attrs)+2)
	
	// Add request ID if present
	if reqID := ctx.Value("request_id"); reqID != nil {
		contextAttrs = append(contextAttrs, slog.String("request_id", fmt.Sprintf("%v", reqID)))
	}
	
	// Add trace ID if present
	if traceID := ctx.Value("trace_id"); traceID != nil {
		contextAttrs = append(contextAttrs, slog.String("trace_id", fmt.Sprintf("%v", traceID)))
	}
	
	// Convert attrs to []any
	for _, attr := range attrs {
		contextAttrs = append(contextAttrs, attr)
	}
	
	return &Logger{Logger: l.With(contextAttrs...)}
}

// LogError logs an error with stack trace information and structured attributes
func (l *Logger) LogError(ctx context.Context, err error, msg string, attrs ...slog.Attr) {
	allAttrs := make([]any, 0, len(attrs)+3)
	
	// Add error information
	if syncErr, ok := err.(*errors.SyncError); ok {
		allAttrs = append(allAttrs, slog.Any("sync_error", SyncErrorValuer{SyncError: syncErr}))
	} else {
		allAttrs = append(allAttrs, slog.String("error", err.Error()))
	}
	
	// Add stack trace information
	pc, file, line, ok := runtime.Caller(1)
	if ok {
		fn := runtime.FuncForPC(pc)
		allAttrs = append(allAttrs,
			slog.Group("caller",
				slog.String("file", file),
				slog.Int("line", line),
				slog.String("function", fn.Name()),
			),
		)
	}
	
	// Convert attrs to []any
	for _, attr := range attrs {
		allAttrs = append(allAttrs, attr)
	}
	
	l.ErrorContext(ctx, msg, allAttrs...)
}

// LogOperation logs the start and end of an operation with duration tracking
func (l *Logger) LogOperation(ctx context.Context, op Operation, component Component, fn func() error) error {
	start := time.Now()
	opLogger := l.WithOperation(op).WithComponent(component)
	
	opLogger.InfoContext(ctx, "operation started",
		slog.Time("start_time", start),
	)
	
	err := fn()
	duration := time.Since(start)
	
	if err != nil {
		opLogger.LogError(ctx, err, "operation failed",
			slog.Duration("duration", duration),
			slog.Bool("success", false),
		)
		return err
	}
	
	opLogger.InfoContext(ctx, "operation completed",
		slog.Duration("duration", duration),
		slog.Bool("success", true),
	)
	
	return nil
}

// Convenience methods that use the default logger
func Debug(msg string, attrs ...slog.Attr) {
	args := make([]any, len(attrs))
	for i, attr := range attrs {
		args[i] = attr
	}
	Default().Debug(msg, args...)
}

func Info(msg string, attrs ...slog.Attr) {
	args := make([]any, len(attrs))
	for i, attr := range attrs {
		args[i] = attr
	}
	Default().Info(msg, args...)
}

func Warn(msg string, attrs ...slog.Attr) {
	args := make([]any, len(attrs))
	for i, attr := range attrs {
		args[i] = attr
	}
	Default().Warn(msg, args...)
}

func Error(msg string, attrs ...slog.Attr) {
	args := make([]any, len(attrs))
	for i, attr := range attrs {
		args[i] = attr
	}
	Default().Error(msg, args...)
}

func DebugContext(ctx context.Context, msg string, attrs ...slog.Attr) {
	args := make([]any, len(attrs))
	for i, attr := range attrs {
		args[i] = attr
	}
	Default().DebugContext(ctx, msg, args...)
}

func InfoContext(ctx context.Context, msg string, attrs ...slog.Attr) {
	args := make([]any, len(attrs))
	for i, attr := range attrs {
		args[i] = attr
	}
	Default().InfoContext(ctx, msg, args...)
}

func WarnContext(ctx context.Context, msg string, attrs ...slog.Attr) {
	args := make([]any, len(attrs))
	for i, attr := range attrs {
		args[i] = attr
	}
	Default().WarnContext(ctx, msg, args...)
}

func ErrorContext(ctx context.Context, msg string, attrs ...slog.Attr) {
	args := make([]any, len(attrs))
	for i, attr := range attrs {
		args[i] = attr
	}
	Default().ErrorContext(ctx, msg, args...)
}

func LogError(ctx context.Context, err error, msg string, attrs ...slog.Attr) {
	Default().LogError(ctx, err, msg, attrs...)
}

func LogOperation(ctx context.Context, op Operation, component Component, fn func() error) error {
	return Default().LogOperation(ctx, op, component, fn)
}

func WithOperation(op Operation) *Logger {
	return Default().WithOperation(op)
}

func WithComponent(component Component) *Logger {
	return Default().WithComponent(component)
}

func WithContext(ctx context.Context, attrs ...slog.Attr) *Logger {
	return Default().WithContext(ctx, attrs...)
}
