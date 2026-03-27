# Implementation Status

This document maps each phase from the [Implementation Plan](../execution/implementation-plan.md) to what was actually built, listing key files, test counts, security findings addressed, and deviations from the original plan.

**Last updated:** 2026-03-27
**Total test functions:** 209

---

## Phase 0: Project Bootstrap

**Status:** Complete

### Key Files Created

| File | Purpose |
|---|---|
| `go.mod` | Module `github.com/coetzeevs/qraftworx-cli`, Go 1.26.1 |
| `go.sum` | Dependency checksums |
| `cmd/qraft/main.go` | CLI entry point with Cobra root command |
| `.golangci.yml` | Linter config (errcheck, govet, staticcheck, gosec, gocritic, etc.) |
| `.pre-commit-config.yaml` | Pre-commit hooks |
| `.goreleaser.yml` | Release configuration |
| `.github/workflows/` | CI pipeline |
| `.gitleaks.toml` | Secret detection config |
| `.gitignore` | Binary, test artifact, secret exclusions |
| `.env.example` | Template for environment variables |
| `Makefile` | Build, test, lint, CI targets |
| `CLAUDE.md` | Project instructions for AI assistants |

### Dependencies Pinned

| Dependency | Version | Purpose |
|---|---|---|
| `github.com/coetzeevs/cerebro` | v1.2.0 | Persistent memory (brain/ package) |
| `google.golang.org/genai` | v1.51.0 | Gemini Go SDK |
| `github.com/spf13/cobra` | v1.10.2 | CLI framework |
| `github.com/eclipse/paho.mqtt.golang` | v1.5.1 | MQTT client |
| `github.com/BurntSushi/toml` | v1.6.0 | TOML config parsing |

### Security Findings Addressed

- **S12 (Supply chain):** `govulncheck` in CI, dependency pinning in go.sum
- **N1 (Hardcoded keys):** `gitleaks` in pre-commit pipeline

### Test Count: 0 (bootstrap phase, no production code)

---

## Phase 1: CerebroClient Wrapper + SafePath

**Status:** Complete

### Key Files Created

| File | Purpose |
|---|---|
| `internal/safepath/safepath.go` | SafePath opaque type with New(), NewOutput(), String() |
| `internal/safepath/safepath_test.go` | 12 tests: traversal, symlinks, relative paths, multiple bases, output paths |
| `internal/cerebro/client.go` | Client wrapping brain.Brain with project + global store |
| `internal/cerebro/client_test.go` | 17 tests: open, Add/List, Get/Edges, Stats, global, missing DB |

### Security Findings Addressed

- **S9 (SafePath type):** Defined in this phase, used by all subsequent phases
- **S10 (Timeouts):** CerebroClient methods accept context.Context

### Test Count: 29

### Deviations from Plan

- `NewOutput()` added to SafePath for validating output files that do not yet exist (needed by ffmpeg capture/transcode)
- `NewClientFromPath()` and `NewClientForTest()` added for cross-package test support
- `NewClientWithGlobal()` separated from `NewClient()` for clearer API
- `AddToGlobal()` and `ListGlobal()` methods added for global brain operations

---

## Phase 2: Gemini Client

**Status:** Complete

### Key Files Created

| File | Purpose |
|---|---|
| `internal/gemini/client.go` | Client with Generate(), Part types, option pattern |
| `internal/gemini/client_test.go` | 14 tests: text, function calls, usage, retry, timeout, unregistered tools |
| `internal/gemini/retry.go` | classifyError(), withRetry() with exponential backoff |
| `internal/gemini/tools.go` | BuildToolDeclarations(), ToolNames(), schemaFromMap() |
| `internal/gemini/tools_test.go` | 3 tests: declaration building, empty input, name extraction |

### Security Findings Addressed

- **S10 (Timeouts):** context.WithTimeout on every GenerateContent call
- **S6 (Cost control, partial):** Missing UsageMetadata treated as MaxCostUsage sentinel
- **Architectural rec #4:** Reject function calls naming unregistered tools

### Test Count: 17

### Deviations from Plan

- Tests use injectable `generateFunc` and `sleepFn` instead of `httptest.Server` -- cleaner, faster, more focused
- `APIError` type with `StatusCode()` method for retry classification
- `FunctionResponsePart` added as a Part implementation for tool loop responses
- `WithRegisteredToolNames()` option added for tool name validation

---

## Phase 3: Single-Pass Loop + Tool Interface + Executor + Hydrator

**Status:** Complete

### Key Files Created

| File | Purpose |
|---|---|
| `internal/tools/tool.go` | Tool interface + ToolPermission struct |
| `internal/tools/registry.go` | Registry with Register/Get/All, duplicate detection |
| `internal/tools/registry_test.go` | 4 tests: register, get, all, duplicate panics |
| `internal/tools/memory.go` | MemoryAddTool + MemorySearchTool |
| `internal/tools/memory_test.go` | 8 tests: name, execute, bad args, empty content/query |
| `internal/executor/executor.go` | Executor with confirmation gates, panic recovery, error caps |
| `internal/executor/executor_test.go` | 8 tests: non-TTY, not found, no-confirm, approved, denied, panic, error cap, reset |
| `internal/executor/confirm.go` | stripControlChars() with ANSI regex |
| `internal/executor/confirm_test.go` | 3 tests: ANSI removal, non-printable, normal text |
| `internal/hydrator/hydrator.go` | Hydrator with Hydrate(), token budgeting, memory truncation |
| `internal/hydrator/hydrator_test.go` | 5 tests: basic, truncation, no memories, with/without sensors |
| `internal/hydrator/format.go` | FormatForGemini(), sanitizeMemoryContent() |
| `internal/hydrator/format_test.go` | 4 tests: delimiters, sanitization, truncation, no memories |
| `internal/gemini/loop.go` | RunLoop() with cost integration and interaction logging |
| `internal/gemini/loop_test.go` | 5 tests: text, function call+text, max iterations, tool error, nil executor |

### Security Findings Addressed

- **S1 (Confirmation gate, CRITICAL):** ANSI stripping, TTY detection, default-deny non-interactive
- **S3 (Indirect prompt injection, HIGH):** Memory delimiters, anti-injection preamble, content length cap, pattern detection

### Test Count: 37

### Deviations from Plan

- CLI wiring (Tasks 3.23-3.24) deferred; `init` subcommand built in Phase 8 instead
- `WithSensors()` added to Hydrator for sensor poller injection
- `summarizeToolCall()` produces sanitized summaries (name + arg keys only)
- `logInteraction()` integrated directly into RunLoop

---

## Phase 4: Observability -- Logging + Cost Tracking

**Status:** Complete

### Key Files Created

| File | Purpose |
|---|---|
| `internal/logging/logging.go` | Logger with 0600 file perms, secret scrubbing |
| `internal/logging/logging_test.go` | 7 tests: perms, invalid path, JSON, secrets, nil file, slog, all levels |
| `internal/logging/scrubber.go` | SecretScrubber (slog.Handler), IsSensitiveKey() |
| `internal/logging/scrubber_test.go` | 13 tests: each sensitive pattern, case insensitive, WithAttrs, WithGroup, Enabled, JSON output |
| `internal/logging/interaction.go` | InteractionLog + ToolCallLog structs |
| `internal/logging/interaction_test.go` | 6 tests: marshal round-trip, error field, omit empty, no raw args, summary string |
| `internal/cost/tracker.go` | Tracker with PreCallGate, RecordUsage, TodaySpend, file counter |
| `internal/cost/tracker_test.go` | 11 tests: under/over/exact budget, accumulation, concurrency, daily reset, nil usage, warn threshold |

### Security Findings Addressed

- **S4 (Log security, HIGH):** File permissions 0600, SecretScrubber, no raw args in logs
- **S6 (Cost control, HIGH):** Pre-call budget gate, file-locked counter, absent metadata = max cost

### Test Count: 37

### Deviations from Plan

- Cost Tracker uses `sync.Mutex` (in-process) rather than file locking, with injectable `nowFunc` for testing
- `RecordUsage` takes `(promptTokens, candidateTokens int)` instead of `*gemini.UsageMetadata` to avoid circular dependency
- Negative `promptTokens` signals absent usage metadata (convention)
- `SetLogger()` method on Tracker for warning output injection

---

## Phase 5: Sensor Integration (MQTT + HTTP Polling)

**Status:** Complete

### Key Files Created

| File | Purpose |
|---|---|
| `internal/sensors/sensors.go` | SensorProvider interface |
| `internal/sensors/poller.go` | Poller with concurrent PollAll() and timeout |
| `internal/sensors/poller_test.go` | 5 tests: multiple sensors, timeout skips slow, all down, no providers, error sensor |
| `internal/sensors/mqtt.go` | MQTTSensor + MQTTConfig + ValueSchema + FieldSpec |
| `internal/sensors/mqtt_test.go` | 10 tests: plaintext rejection, secure acceptance, subscribe/cache, no data, schema validation, injection rejection, empty fields, invalid JSON, name |
| `internal/sensors/moonraker.go` | MoonrakerSensor + PrinterState with typed validation |
| `internal/sensors/moonraker_test.go` | 11 tests: parse status, timeout, range validation (4 sub-tests), unreachable, invalid enum, valid states, ToMap, name, close, bad JSON, non-200 |
| `internal/sensors/testdata/moonraker_status.json` | Fixture for Moonraker response parsing |

### Security Findings Addressed

- **S5 (MQTT transport, HIGH):** TLS required by default, AllowInsecure opt-in, config-level validation
- **S3 (Data injection, HIGH):** ValueSchema validation, typed PrinterState, range checks
- **S10 (Timeouts, MEDIUM):** http.Client.Timeout on Moonraker, context.WithTimeout on Poller

### Test Count: 26

### Deviations from Plan

- `MQTTClient` interface extracted for testability (Connect, Subscribe, Disconnect, IsConnected)
- `ValueSchema` supports four types: float64, int, string, enum -- with range checks and enum validation
- `PrinterState` enum uses typed `PrinterStateEnum` instead of free string
- Moonraker sensor returns `nil, nil` for all graceful failure modes (unreachable, non-200, bad JSON)

---

## Phase 6: Media Tools

**Status:** Complete

### Key Files Created

| File | Purpose |
|---|---|
| `internal/tools/ffmpeg.go` | FFmpegBuilder + codec allowlist + parameter clamping |
| `internal/tools/ffmpeg_test.go` | 12 tests: codec validation, FPS clamping, duration clamping, resolution validation, capture frame, transcode |
| `internal/tools/ffmpeg_integration_test.go` | 1 test: real ffmpeg execution (guarded by testing.Short) |
| `internal/tools/capture.go` | CaptureMediaTool (frame capture with SafePath) |
| `internal/tools/capture_test.go` | 8 tests: name, parameters, confirmation, permissions, bad args, empty filename, path traversal, execute |
| `internal/tools/video.go` | ProcessVideoTool (ffmpeg transcoding with SafePath) |
| `internal/tools/video_test.go` | 9 tests: name, parameters, no confirmation, bad args, empty fields, input traversal, output traversal, invalid codec, execute |

### Security Findings Addressed

- **S2 (Command injection, CRITICAL):** SafePath for all paths, exec.CommandContext with arg slices, device from config only, codec allowlist, parameter clamping

### Test Count: 30

### Deviations from Plan

- `FFmpegBuilder` is a dedicated type rather than utility functions, encapsulating binary validation and allowed base directories
- `NewOutput()` added to SafePath specifically for ffmpeg output files that do not exist yet
- `TranscodeOpts` struct groups all transcoding parameters

---

## Phase 7: Upload Tools

**Status:** Complete

### Key Files Created

| File | Purpose |
|---|---|
| `internal/tools/upload.go` | UploadTool + Uploader interface + rate limiting |
| `internal/tools/upload_test.go` | 7 tests: name, confirmation, bad args, missing fields, rate limit, path validation, unknown platform |
| `internal/tools/youtube.go` | YouTubeUploader (multipart upload with OAuth) |
| `internal/tools/youtube_test.go` | 1 test: upload round-trip with httptest.Server |
| `internal/tools/tiktok.go` | TikTokUploader (Content Posting API) |
| `internal/tools/tiktok_test.go` | 1 test: upload round-trip with httptest.Server |
| `internal/tools/mime.go` | ValidateMediaFile (header-based MIME check) |
| `internal/tools/mime_test.go` | 4 tests: valid video, valid image, invalid type, non-existent file |

### Security Findings Addressed

- **S7 (Upload hardening, HIGH):** SafePath path validation, MIME header check, rate limiting (1/hour), confirmation required

### Test Count: 13

### Deviations from Plan

- `Uploader` interface extracted for platform-agnostic upload logic
- `UploadMetadata` and `UploadResult` types standardize cross-platform upload data
- Privacy defaults to "private" for both YouTube and TikTok
- Rate limiting uses time window (1 hour) rather than per-interaction count

---

## Phase 8: Configuration + CLI

**Status:** Complete

### Key Files Created

| File | Purpose |
|---|---|
| `internal/config/config.go` | Config struct, TOML loading, Duration type |
| `internal/config/config_test.go` | 5 tests: valid config, missing file, invalid syntax, duration parsing |
| `internal/config/validate.go` | Multi-error validation with defaults |
| `internal/config/validate_test.go` | 9 tests: missing model, invalid log level, negative budget, MQTT plaintext, multi-error, defaults, missing model fixture |
| `internal/config/testdata/valid.toml` | Valid config fixture |
| `internal/config/testdata/invalid_syntax.toml` | Invalid TOML fixture |
| `internal/config/testdata/missing_model.toml` | Missing required field fixture |
| `internal/config/testdata/mqtt_plaintext.toml` | Plaintext MQTT fixture |
| `cmd/qraft/init.go` | `qraft init` subcommand |
| `cmd/qraft/init_test.go` | 6 tests: creates config dir, writes default config, creates log dir, does not overwrite, warns no API key, no warning when set |

### Security Findings Addressed

- **S4 (File permissions):** Config directory 0700, config file 0600, log directory 0700
- **S5 (MQTT validation):** Config-level validation rejects plaintext MQTT without opt-in

### Test Count: 20

### Deviations from Plan

- `qraft init` implemented as a subcommand rather than auto-init on first run
- Init function uses dependency injection (homeDir, lookupEnv, writer) for testability
- Default config includes all sections with sensible values

---

## Summary

| Phase | Name | Status | Test Count | Key Security Findings |
|---|---|---|---|---|
| 0 | Project Bootstrap | Complete | 0 | S12, N1 |
| 1 | CerebroClient + SafePath | Complete | 29 | S9, S10 |
| 2 | Gemini Client | Complete | 17 | S6 (partial), S10, Rec #4 |
| 3 | Loop + Tools + Executor + Hydrator | Complete | 37 | S1, S3 |
| 4 | Logging + Cost Tracking | Complete | 37 | S4, S6 |
| 5 | Sensor Integration | Complete | 26 | S5, S3, S10 |
| 6 | Media Tools | Complete | 30 | S2 |
| 7 | Upload Tools | Complete | 13 | S7 |
| 8 | Configuration + CLI | Complete | 20 | S4, S5 |
| **Total** | | **All Complete** | **209** | **S1-S10, S12 addressed** |

### Findings Not Yet Addressed

| Finding | Severity | Status | Notes |
|---|---|---|---|
| S11 | MEDIUM | Partial | Media retention cleanup not automated |
| S13 | LOW | Not implemented | Log hash chain for tamper detection |
