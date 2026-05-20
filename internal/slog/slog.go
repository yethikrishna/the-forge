// Package slog provides structured logging wrappers around log/slog
// with convenient defaults and Forge-specific formatting.
//
// The Forge burns bright — its logs should too.
package slog

import (
	"log/slog"
	"os"
)

// Level aliases for convenience.
const (
	LevelDebug = slog.LevelDebug
	LevelInfo  = slog.LevelInfo
	LevelWarn  = slog.LevelWarn
	LevelError = slog.LevelError
)

// Logger wraps slog.Logger with Forge defaults.
type Logger struct {
	*slog.Logger
}

// Default returns the default Forge logger.
var Default = New()

// New creates a new Logger with the given options.
// If no handlers are specified, uses a text handler writing to stderr.
func New(opts ...Option) *Logger {
	cfg := &config{
		level:  slog.LevelInfo,
		writer: os.Stderr,
		json:   false,
	}
	for _, o := range opts {
		o(cfg)
	}

	var handler slog.Handler
	if cfg.json {
		handler = slog.NewJSONHandler(cfg.writer, &slog.HandlerOptions{
			Level: cfg.level,
		})
	} else {
		handler = slog.NewTextHandler(cfg.writer, &slog.HandlerOptions{
			Level: cfg.level,
		})
	}

	return &Logger{Logger: slog.New(handler)}
}

type config struct {
	level  slog.Level
	writer interface {
		Write([]byte) (int, error)
	}
	json bool
}

// Option configures a Logger.
type Option func(*config)

// WithLevel sets the minimum log level.
func WithLevel(level slog.Level) Option {
	return func(c *config) { c.level = level }
}

// WithWriter sets the log output writer.
func WithWriter(w interface {
	Write([]byte) (int, error)
}) Option {
	return func(c *config) { c.writer = w }
}

// WithJSON enables JSON log output.
func WithJSON() Option {
	return func(c *config) { c.json = true }
}

// Debug logs at debug level.
func (l *Logger) Debug(msg string, args ...any) {
	l.Logger.Debug(msg, args...)
}

// Info logs at info level.
func (l *Logger) Info(msg string, args ...any) {
	l.Logger.Info(msg, args...)
}

// Warn logs at warn level.
func (l *Logger) Warn(msg string, args ...any) {
	l.Logger.Warn(msg, args...)
}

// Error logs at error level.
func (l *Logger) Error(msg string, args ...any) {
	l.Logger.Error(msg, args...)
}

// With returns a new Logger with the given attributes added.
func (l *Logger) With(args ...any) *Logger {
	return &Logger{Logger: l.Logger.With(args...)}
}

// WithGroup returns a new Logger with the given group name.
func (l *Logger) WithGroup(name string) *Logger {
	return &Logger{Logger: l.Logger.WithGroup(name)}
}

// SetDefault sets the global default logger.
func SetDefault(l *Logger) {
	Default = l
	slog.SetDefault(l.Logger)
}

// Package-level convenience functions that delegate to Default.

func Debug(msg string, args ...any) { Default.Debug(msg, args...) }
func Info(msg string, args ...any)  { Default.Info(msg, args...) }
func Warn(msg string, args ...any)  { Default.Warn(msg, args...) }
func Error(msg string, args ...any) { Default.Error(msg, args...) }
