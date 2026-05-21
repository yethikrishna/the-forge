// Package flog provides formatted logging with simple Printf-style API
// and level control. Lightweight alternative to slog for quick logging.
package flog

import (
	"fmt"
	"io"
	"log"
	"os"
	"sync"
)

// Level represents a log level.
type Level int

const (
	LevelDebug Level = iota
	LevelInfo
	LevelWarn
	LevelError
	LevelSilent
)

// Logger provides formatted logging at different levels.
type Logger struct {
	mu     sync.Mutex
	level  Level
	prefix string
	debug  *log.Logger
	info   *log.Logger
	warn   *log.Logger
	err    *log.Logger
	writer io.Writer
}

// Default is the package-level default logger.
var Default = New()

// New creates a new Logger writing to stderr.
func New() *Logger {
	return NewWithWriter(os.Stderr)
}

// NewWithWriter creates a new Logger writing to the given writer.
func NewWithWriter(w io.Writer) *Logger {
	l := &Logger{
		level:  LevelInfo,
		prefix: "",
		writer: w,
	}
	l.debug = log.New(w, "[DEBUG] ", log.Ltime)
	l.info = log.New(w, "[INFO]  ", log.Ltime)
	l.warn = log.New(w, "[WARN]  ", log.Ltime)
	l.err = log.New(w, "[ERROR] ", log.Ltime|log.Lshortfile)
	return l
}

// SetLevel sets the minimum log level.
func (l *Logger) SetLevel(level Level) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.level = level
}

// SetPrefix sets a prefix for all log messages.
func (l *Logger) SetPrefix(prefix string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.prefix = prefix
}

// Debugf logs a formatted debug message.
func (l *Logger) Debugf(format string, args ...any) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.level <= LevelDebug {
		l.debug.Printf(l.prefix+format, args...)
	}
}

// Infof logs a formatted info message.
func (l *Logger) Infof(format string, args ...any) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.level <= LevelInfo {
		l.info.Printf(l.prefix+format, args...)
	}
}

// Warnf logs a formatted warning message.
func (l *Logger) Warnf(format string, args ...any) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.level <= LevelWarn {
		l.warn.Printf(l.prefix+format, args...)
	}
}

// Errorf logs a formatted error message.
func (l *Logger) Errorf(format string, args ...any) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.level <= LevelError {
		l.err.Printf(l.prefix+format, args...)
	}
}

// Debug logs a debug message.
func (l *Logger) Debug(args ...any) {
	l.Debugf("%s", fmt.Sprint(args...))
}

// Info logs an info message.
func (l *Logger) Info(args ...any) {
	l.Infof("%s", fmt.Sprint(args...))
}

// Warn logs a warning message.
func (l *Logger) Warn(args ...any) {
	l.Warnf("%s", fmt.Sprint(args...))
}

// Error logs an error message.
func (l *Logger) Error(args ...any) {
	l.Errorf("%s", fmt.Sprint(args...))
}

// Package-level convenience functions.

func SetLevel(level Level)        { Default.SetLevel(level) }
func SetPrefix(prefix string)     { Default.SetPrefix(prefix) }
func Debugf(format string, args ...any) { Default.Debugf(format, args...) }
func Infof(format string, args ...any)  { Default.Infof(format, args...) }
func Warnf(format string, args ...any)  { Default.Warnf(format, args...) }
func Errorf(format string, args ...any) { Default.Errorf(format, args...) }
func Debug(args ...any)                 { Default.Debug(args...) }
func Info(args ...any)                  { Default.Info(args...) }
func Warn(args ...any)                  { Default.Warn(args...) }
func Error(args ...any)                 { Default.Error(args...) }
