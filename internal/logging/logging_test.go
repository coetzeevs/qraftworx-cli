package logging

import (
	"encoding/json"
	"log/slog"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// Task 4.1: Logger creates file with 0600 permissions.

func TestNewLogger_FilePermissions(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("file permissions not testable on Windows")
	}

	dir := t.TempDir()
	logPath := filepath.Join(dir, "test.log")

	logger, err := NewLogger(logPath, slog.LevelDebug)
	if err != nil {
		t.Fatalf("NewLogger: %v", err)
	}
	defer func() {
		if closeErr := logger.Close(); closeErr != nil {
			t.Errorf("Close: %v", closeErr)
		}
	}()

	info, err := os.Stat(logPath)
	if err != nil {
		t.Fatalf("stat: %v", err)
	}

	perm := info.Mode().Perm()
	if perm != 0o600 {
		t.Errorf("file permissions = %o, want 0600", perm)
	}
}

func TestNewLogger_InvalidPath(t *testing.T) {
	_, err := NewLogger("/nonexistent/dir/test.log", slog.LevelDebug)
	if err == nil {
		t.Fatal("expected error for invalid path")
	}
}

// Task 4.3: Logger writes structured JSON.

func TestLogger_WritesJSON(t *testing.T) {
	dir := t.TempDir()
	logPath := filepath.Join(dir, "test.log")

	logger, err := NewLogger(logPath, slog.LevelDebug)
	if err != nil {
		t.Fatalf("NewLogger: %v", err)
	}

	logger.Info("hello world", "key", "value")

	if closeErr := logger.Close(); closeErr != nil {
		t.Fatalf("Close: %v", closeErr)
	}

	data, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}

	var m map[string]any
	if err := json.Unmarshal(data, &m); err != nil {
		t.Fatalf("output is not valid JSON: %v\nraw: %s", err, string(data))
	}

	if msg, ok := m["msg"].(string); !ok || msg != "hello world" {
		t.Errorf("msg = %v, want 'hello world'", m["msg"])
	}
	if val, ok := m["key"].(string); !ok || val != "value" {
		t.Errorf("key = %v, want 'value'", m["key"])
	}
}

// Task 4.4: Logger scrubs secrets in output.

func TestLogger_ScrubsSecrets(t *testing.T) {
	dir := t.TempDir()
	logPath := filepath.Join(dir, "test.log")

	logger, err := NewLogger(logPath, slog.LevelDebug)
	if err != nil {
		t.Fatalf("NewLogger: %v", err)
	}

	logger.Info("request",
		"api_key", "sk-real-key-123",
		"user", "alice",
		"authorization", "Bearer tok-xyz",
	)

	if closeErr := logger.Close(); closeErr != nil {
		t.Fatalf("Close: %v", closeErr)
	}

	data, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}

	out := string(data)
	if strings.Contains(out, "sk-real-key-123") {
		t.Errorf("api_key value leaked in log output: %s", out)
	}
	if strings.Contains(out, "Bearer tok-xyz") {
		t.Errorf("authorization value leaked in log output: %s", out)
	}
	if !strings.Contains(out, "alice") {
		t.Errorf("expected 'alice' in output: %s", out)
	}
}

func TestLogger_Close_NilFile(t *testing.T) {
	l := &Logger{}
	if err := l.Close(); err != nil {
		t.Errorf("Close on nil file: %v", err)
	}
}

func TestLogger_Slog(t *testing.T) {
	dir := t.TempDir()
	logPath := filepath.Join(dir, "test.log")

	logger, err := NewLogger(logPath, slog.LevelDebug)
	if err != nil {
		t.Fatalf("NewLogger: %v", err)
	}
	defer func() {
		if closeErr := logger.Close(); closeErr != nil {
			t.Errorf("Close: %v", closeErr)
		}
	}()

	if logger.Slog() == nil {
		t.Error("Slog() returned nil")
	}
}

func TestLogger_AllLevels(t *testing.T) {
	dir := t.TempDir()
	logPath := filepath.Join(dir, "test.log")

	logger, err := NewLogger(logPath, slog.LevelDebug)
	if err != nil {
		t.Fatalf("NewLogger: %v", err)
	}

	logger.Debug("debug msg")
	logger.Info("info msg")
	logger.Warn("warn msg")
	logger.Error("error msg")

	if closeErr := logger.Close(); closeErr != nil {
		t.Fatalf("Close: %v", closeErr)
	}

	data, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}

	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	if len(lines) != 4 {
		t.Fatalf("expected 4 log lines, got %d: %s", len(lines), string(data))
	}

	for _, line := range lines {
		var m map[string]any
		if err := json.Unmarshal([]byte(line), &m); err != nil {
			t.Errorf("invalid JSON line: %s", line)
		}
	}
}
