// Package logging provides structured JSON logging with secret scrubbing
// for QraftWorx.
//
// # Key Types
//
//   - Logger: wraps slog.Logger with file management and secret scrubbing.
//     Creates log files with 0600 permissions (S4). Provides Info, Warn,
//     Error, Debug methods and Slog() for direct access.
//   - SecretScrubber: an slog.Handler that intercepts log records and redacts
//     sensitive field values before they reach the underlying handler.
//   - InteractionLog: structured record of one Qraft interaction (timestamp,
//     request ID, user prompt, token counts, tool calls, latency, cost, error).
//   - ToolCallLog: structured record of a single tool invocation (name,
//     sanitized summary, duration, error). Uses Summary string, never raw args.
//
// # Architecture Role
//
// The logging package provides observability across the entire system. Every
// Gemini interaction is recorded as a structured InteractionLog. The Logger
// is injected into the cost tracker and tool loop for warning and error output.
//
// The SecretScrubber is a key security component: it wraps the base slog handler
// and scans every log attribute for sensitive field names before they are written.
//
// # Secret Scrubbing (S4)
//
// The SecretScrubber redacts values of attributes whose keys (case-insensitive)
// contain any of these patterns:
//   - api_key
//   - token
//   - secret
//   - authorization
//   - credential
//   - password
//
// Redacted values are replaced with "[REDACTED]". Group attributes are processed
// recursively. The scrubber operates at the slog.Handler level, ensuring all
// log output is sanitized regardless of which code path produces it.
//
// # Security Considerations
//
//   - S4: Log files created with 0600 permissions (owner read/write only).
//   - S4: SecretScrubber prevents API keys and tokens from appearing in logs.
//   - S4: ToolCallLog.Summary is a sanitized string, not raw JSON args.
//   - IsSensitiveKey: exported utility for checking if a field name should be redacted.
//
// # Testing
//
// Logger tests verify file permissions, JSON output format, and secret redaction
// by writing to files in t.TempDir(). Scrubber tests use bytes.Buffer as the
// log sink and verify that sensitive values are replaced with [REDACTED] while
// normal values are preserved. WithAttrs and WithGroup paths are tested for
// complete coverage.
package logging
