package logging

import (
	"context"
	"log/slog"
	"os"
	"strings"
)

// Environment types
const (
	EnvDevelopment = "development"
	EnvProduction  = "production"
	EnvTest        = "test"
)

// GetConfigFromEnv creates a logger configuration based on environment variables
func GetConfigFromEnv() Config {
	config := DefaultConfig

	// Get log level from environment
	if level := os.Getenv("LOG_LEVEL"); level != "" {
		config.Level = strings.ToLower(level)
	}

	// Get log format from environment
	if format := os.Getenv("LOG_FORMAT"); format != "" {
		config.Format = strings.ToLower(format)
	}

	// Get environment
	if env := os.Getenv("ENVIRONMENT"); env != "" {
		config.Environment = strings.ToLower(env)
	}

	// Get add source setting
	if addSource := os.Getenv("LOG_ADD_SOURCE"); addSource != "" {
		config.AddSource = strings.ToLower(addSource) == "true"
	}

	// Environment-specific defaults
	switch config.Environment {
	case EnvProduction:
		// Production: JSON format, INFO level, no source info for performance
		if config.Format == "" {
			config.Format = "json"
		}
		if config.Level == "" {
			config.Level = "info"
		}
		config.AddSource = false

	case EnvTest:
		// Test: Text format for readability, DEBUG level
		if config.Format == "" {
			config.Format = "text"
		}
		if config.Level == "" {
			config.Level = "debug"
		}
		config.AddSource = false

	case EnvDevelopment:
		// Development: Text format for readability, DEBUG level, source info
		if config.Format == "" {
			config.Format = "text"
		}
		if config.Level == "" {
			config.Level = "debug"
		}
		config.AddSource = true
	}

	return config
}

// CustomLevel defines a custom log level between existing ones
type CustomLevel slog.Level

// Custom levels between the standard ones
const (
	LevelTrace CustomLevel = CustomLevel(slog.LevelDebug - 4) // Even more verbose than debug
	LevelFatal CustomLevel = CustomLevel(slog.LevelError + 4) // More severe than error
)

// String returns the string representation of the custom level
func (l CustomLevel) String() string {
	switch l {
	case LevelTrace:
		return "TRACE"
	case LevelFatal:
		return "FATAL"
	default:
		return slog.Level(l).String()
	}
}

// Trace logs at trace level using the default logger
func Trace(msg string, attrs ...slog.Attr) {
	args := make([]any, len(attrs))
	for i, attr := range attrs {
		args[i] = attr
	}
	Default().Log(nil, slog.Level(LevelTrace), msg, args...)
}

// Fatal logs at fatal level and exits the program
func Fatal(msg string, attrs ...slog.Attr) {
	args := make([]any, len(attrs))
	for i, attr := range attrs {
		args[i] = attr
	}
	Default().Log(nil, slog.Level(LevelFatal), msg, args...)
	os.Exit(1)
}

// TraceContext logs at trace level with context using the default logger
func TraceContext(ctx context.Context, msg string, attrs ...slog.Attr) {
	args := make([]any, len(attrs))
	for i, attr := range attrs {
		args[i] = attr
	}
	Default().Log(ctx, slog.Level(LevelTrace), msg, args...)
}

// FatalContext logs at fatal level with context and exits the program
func FatalContext(ctx context.Context, msg string, attrs ...slog.Attr) {
	args := make([]any, len(attrs))
	for i, attr := range attrs {
		args[i] = attr
	}
	Default().Log(ctx, slog.Level(LevelFatal), msg, args...)
	os.Exit(1)
}

// DynamicLevelVar allows changing log level at runtime
type DynamicLevelVar struct {
	*slog.LevelVar
}

// NewDynamicLevelVar creates a new dynamic level variable
func NewDynamicLevelVar(initialLevel slog.Level) *DynamicLevelVar {
	levelVar := &slog.LevelVar{}
	levelVar.Set(initialLevel)
	return &DynamicLevelVar{LevelVar: levelVar}
}

// SetFromString sets the level from a string representation
func (d *DynamicLevelVar) SetFromString(level string) bool {
	switch strings.ToLower(level) {
	case "trace":
		d.Set(slog.Level(LevelTrace))
	case "debug":
		d.Set(slog.LevelDebug)
	case "info":
		d.Set(slog.LevelInfo)
	case "warn", "warning":
		d.Set(slog.LevelWarn)
	case "error":
		d.Set(slog.LevelError)
	case "fatal":
		d.Set(slog.Level(LevelFatal))
	default:
		return false
	}
	return true
}

// NewLoggerWithDynamicLevel creates a logger with dynamic level support
func NewLoggerWithDynamicLevel(config Config) (*Logger, *DynamicLevelVar) {
	levelVar := NewDynamicLevelVar(slog.LevelInfo)
	
	opts := &slog.HandlerOptions{
		Level:     levelVar.LevelVar,
		AddSource: config.AddSource,
	}

	var handler slog.Handler
	if config.Format == "text" || config.Environment == "dev" {
		handler = slog.NewTextHandler(os.Stdout, opts)
	} else {
		handler = slog.NewJSONHandler(os.Stdout, opts)
	}

	logger := &Logger{Logger: slog.New(handler)}
	return logger, levelVar
}
