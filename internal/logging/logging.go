package logging

import (
	"fmt"
	"log/slog"
	"os"
)

// Logger wraps slog with Qraft-specific functionality:
// file creation with 0o600 permissions (S4) and secret scrubbing (S4).
type Logger struct {
	slog     *slog.Logger
	file     *os.File
	scrubber *SecretScrubber
}

// NewLogger creates a JSON logger writing to the given path.
// Creates the file with 0o600 permissions (S4).
func NewLogger(path string, level slog.Level) (*Logger, error) {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		return nil, fmt.Errorf("logging: creating log file: %w", err)
	}

	base := slog.NewJSONHandler(f, &slog.HandlerOptions{Level: level})
	scrubber := NewSecretScrubber(base)
	logger := slog.New(scrubber)

	return &Logger{
		slog:     logger,
		file:     f,
		scrubber: scrubber,
	}, nil
}

// Slog returns the underlying *slog.Logger for direct use.
func (l *Logger) Slog() *slog.Logger {
	return l.slog
}

// Close closes the underlying log file.
func (l *Logger) Close() error {
	if l.file != nil {
		return l.file.Close()
	}
	return nil
}

// Info logs at INFO level with secret scrubbing.
func (l *Logger) Info(msg string, args ...any) {
	l.slog.Info(msg, args...)
}

// Warn logs at WARN level with secret scrubbing.
func (l *Logger) Warn(msg string, args ...any) {
	l.slog.Warn(msg, args...)
}

// Error logs at ERROR level with secret scrubbing.
func (l *Logger) Error(msg string, args ...any) {
	l.slog.Error(msg, args...)
}

// Debug logs at DEBUG level with secret scrubbing.
func (l *Logger) Debug(msg string, args ...any) {
	l.slog.Debug(msg, args...)
}
