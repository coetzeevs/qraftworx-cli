package logging

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"strings"
	"testing"
)

// Task 4.2: SecretScrubber redacts sensitive fields.

func TestSecretScrubber_RedactsAPIKey(t *testing.T) {
	var buf bytes.Buffer
	base := slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})
	scrubber := NewSecretScrubber(base)
	logger := slog.New(scrubber)

	logger.Info("test", "api_key", "sk-secret-123")

	out := buf.String()
	if strings.Contains(out, "sk-secret-123") {
		t.Errorf("api_key value leaked: %s", out)
	}
	if !strings.Contains(out, redactedValue) {
		t.Errorf("expected %s in output: %s", redactedValue, out)
	}
}

func TestSecretScrubber_RedactsToken(t *testing.T) {
	var buf bytes.Buffer
	base := slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})
	scrubber := NewSecretScrubber(base)
	logger := slog.New(scrubber)

	logger.Info("test", "auth_token", "tok-abc-456")

	out := buf.String()
	if strings.Contains(out, "tok-abc-456") {
		t.Errorf("token value leaked: %s", out)
	}
	if !strings.Contains(out, redactedValue) {
		t.Errorf("expected %s in output: %s", redactedValue, out)
	}
}

func TestSecretScrubber_RedactsSecret(t *testing.T) {
	var buf bytes.Buffer
	base := slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})
	scrubber := NewSecretScrubber(base)
	logger := slog.New(scrubber)

	logger.Info("test", "client_secret", "mysecret")

	out := buf.String()
	if strings.Contains(out, "mysecret") {
		t.Errorf("secret value leaked: %s", out)
	}
}

func TestSecretScrubber_RedactsAuthorization(t *testing.T) {
	var buf bytes.Buffer
	base := slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})
	scrubber := NewSecretScrubber(base)
	logger := slog.New(scrubber)

	logger.Info("test", "authorization", "Bearer xyz")

	out := buf.String()
	if strings.Contains(out, "Bearer xyz") {
		t.Errorf("authorization value leaked: %s", out)
	}
}

func TestSecretScrubber_RedactsCredential(t *testing.T) {
	var buf bytes.Buffer
	base := slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})
	scrubber := NewSecretScrubber(base)
	logger := slog.New(scrubber)

	logger.Info("test", "credential", "cred-xyz")

	out := buf.String()
	if strings.Contains(out, "cred-xyz") {
		t.Errorf("credential value leaked: %s", out)
	}
}

func TestSecretScrubber_RedactsPassword(t *testing.T) {
	var buf bytes.Buffer
	base := slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})
	scrubber := NewSecretScrubber(base)
	logger := slog.New(scrubber)

	logger.Info("test", "password", "hunter2")

	out := buf.String()
	if strings.Contains(out, "hunter2") {
		t.Errorf("password value leaked: %s", out)
	}
}

func TestSecretScrubber_PreservesNormalFields(t *testing.T) {
	var buf bytes.Buffer
	base := slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})
	scrubber := NewSecretScrubber(base)
	logger := slog.New(scrubber)

	logger.Info("test", "user", "alice", "request_id", "abc-123")

	out := buf.String()
	if !strings.Contains(out, "alice") {
		t.Errorf("expected 'alice' in output: %s", out)
	}
	if !strings.Contains(out, "abc-123") {
		t.Errorf("expected 'abc-123' in output: %s", out)
	}
	if strings.Contains(out, redactedValue) {
		t.Errorf("did not expect %s for normal fields: %s", redactedValue, out)
	}
}

func TestSecretScrubber_CaseInsensitive(t *testing.T) {
	var buf bytes.Buffer
	base := slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})
	scrubber := NewSecretScrubber(base)
	logger := slog.New(scrubber)

	logger.Info("test", "API_KEY", "should-be-redacted")

	out := buf.String()
	if strings.Contains(out, "should-be-redacted") {
		t.Errorf("case-insensitive match failed, value leaked: %s", out)
	}
}

func TestSecretScrubber_WithAttrs(t *testing.T) {
	var buf bytes.Buffer
	base := slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})
	scrubber := NewSecretScrubber(base)
	logger := slog.New(scrubber).With("api_key", "pre-set-key")

	logger.Info("test")

	out := buf.String()
	if strings.Contains(out, "pre-set-key") {
		t.Errorf("pre-set api_key leaked via WithAttrs: %s", out)
	}
}

func TestSecretScrubber_WithGroup(t *testing.T) {
	var buf bytes.Buffer
	base := slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})
	scrubber := NewSecretScrubber(base)
	logger := slog.New(scrubber).WithGroup("auth")

	logger.Info("test", "token", "group-token-val")

	out := buf.String()
	if strings.Contains(out, "group-token-val") {
		t.Errorf("token leaked in group: %s", out)
	}
}

func TestSecretScrubber_Enabled(t *testing.T) {
	var buf bytes.Buffer
	base := slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelWarn})
	scrubber := NewSecretScrubber(base)

	if scrubber.Enabled(context.Background(), slog.LevelDebug) {
		t.Error("expected debug to be disabled at warn level")
	}
	if !scrubber.Enabled(context.Background(), slog.LevelWarn) {
		t.Error("expected warn to be enabled at warn level")
	}
}

func TestIsSensitiveKey(t *testing.T) {
	tests := []struct {
		key  string
		want bool
	}{
		{"api_key", true},
		{"API_KEY", true},
		{"auth_token", true},
		{"client_secret", true},
		{"authorization", true},
		{"credential", true},
		{"password", true},
		{"user", false},
		{"request_id", false},
		{"model", false},
	}
	for _, tt := range tests {
		got := IsSensitiveKey(tt.key)
		if got != tt.want {
			t.Errorf("IsSensitiveKey(%q) = %v, want %v", tt.key, got, tt.want)
		}
	}
}

func TestSecretScrubber_OutputIsValidJSON(t *testing.T) {
	var buf bytes.Buffer
	base := slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})
	scrubber := NewSecretScrubber(base)
	logger := slog.New(scrubber)

	logger.Info("test", "api_key", "sk-123", "user", "alice")

	var m map[string]any
	if err := json.Unmarshal(buf.Bytes(), &m); err != nil {
		t.Fatalf("output is not valid JSON: %v\noutput: %s", err, buf.String())
	}
}
