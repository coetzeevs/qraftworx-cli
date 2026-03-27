package logging

import (
	"context"
	"log/slog"
	"strings"
)

// sensitivePatterns are the field name substrings that trigger redaction (S4).
var sensitivePatterns = []string{
	"api_key",
	"token",
	"secret",
	"authorization",
	"credential",
	"password",
}

// redactedValue replaces sensitive field values in log output.
const redactedValue = "[REDACTED]"

// SecretScrubber redacts sensitive fields from log records.
// Fields whose keys contain api_key, token, secret, authorization,
// credential, or password are replaced with "[REDACTED]" (S4).
type SecretScrubber struct {
	patterns []string
	next     slog.Handler
}

// NewSecretScrubber wraps a slog.Handler, redacting sensitive attributes.
func NewSecretScrubber(next slog.Handler) *SecretScrubber {
	return &SecretScrubber{
		patterns: sensitivePatterns,
		next:     next,
	}
}

// Enabled delegates to the wrapped handler.
func (s *SecretScrubber) Enabled(ctx context.Context, level slog.Level) bool {
	return s.next.Enabled(ctx, level)
}

// Handle scrubs sensitive attributes before delegating to the wrapped handler.
func (s *SecretScrubber) Handle(ctx context.Context, r slog.Record) error { //nolint:gocritic // slog.Handler interface requires value receiver for Record
	scrubbed := slog.NewRecord(r.Time, r.Level, r.Message, r.PC)
	r.Attrs(func(a slog.Attr) bool {
		scrubbed.AddAttrs(s.scrubAttr(a))
		return true
	})
	return s.next.Handle(ctx, scrubbed)
}

// WithAttrs returns a new handler with pre-scrubbed attributes.
func (s *SecretScrubber) WithAttrs(attrs []slog.Attr) slog.Handler {
	scrubbed := make([]slog.Attr, len(attrs))
	for i, a := range attrs {
		scrubbed[i] = s.scrubAttr(a)
	}
	return &SecretScrubber{
		patterns: s.patterns,
		next:     s.next.WithAttrs(scrubbed),
	}
}

// WithGroup returns a new handler with the given group.
func (s *SecretScrubber) WithGroup(name string) slog.Handler {
	return &SecretScrubber{
		patterns: s.patterns,
		next:     s.next.WithGroup(name),
	}
}

// scrubAttr redacts the value if the key matches a sensitive pattern.
func (s *SecretScrubber) scrubAttr(a slog.Attr) slog.Attr {
	key := strings.ToLower(a.Key)
	for _, p := range s.patterns {
		if strings.Contains(key, p) {
			return slog.String(a.Key, redactedValue)
		}
	}
	// Handle group attrs recursively
	if a.Value.Kind() == slog.KindGroup {
		attrs := a.Value.Group()
		scrubbed := make([]slog.Attr, len(attrs))
		for i, ga := range attrs {
			scrubbed[i] = s.scrubAttr(ga)
		}
		return slog.Attr{Key: a.Key, Value: slog.GroupValue(scrubbed...)}
	}
	return a
}

// IsSensitiveKey reports whether a field name should be redacted.
func IsSensitiveKey(key string) bool {
	lower := strings.ToLower(key)
	for _, p := range sensitivePatterns {
		if strings.Contains(lower, p) {
			return true
		}
	}
	return false
}
