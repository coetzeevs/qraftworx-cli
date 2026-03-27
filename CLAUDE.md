# QraftWorx CLI

Go CLI for AI-powered content automation. Uses Gemini as reasoning engine, Cerebro as persistent memory. Targets Linux (amd64/arm64) for Raspberry Pi + 3D printer + camera workflows.

## Development

```bash
# Build (CGO required for SQLite)
CGO_ENABLED=1 go build -trimpath -o bin/qraft ./cmd/qraft

# Test (full suite with race detector)
CGO_ENABLED=1 go test ./... -race -coverprofile=coverage.out -covermode=atomic

# Test (short, skips ffmpeg integration tests)
CGO_ENABLED=1 go test ./... -race -short

# Lint
golangci-lint run --config=.golangci.yml

# All CI checks locally
make ci
```

## Key Patterns

- **CGO required**: Cerebro dependency (mattn/go-sqlite3 + sqlite-vec). Always set `CGO_ENABLED=1`.
- **TDD (strict)**: Write failing test first. Red -> Green -> Refactor. No production code without a covering test.
- **SafePath**: All filesystem operations use `SafePath` type (`internal/safepath/`), never raw strings. Validates absolute path, cleans traversals, resolves symlinks, checks allowed bases.
- **Compile-time tools**: All tools registered at compile time in `internal/tools/`. No dynamic loading, no directory scanning.
- **Secrets in env vars**: `GEMINI_API_KEY` and all API keys via environment variables only. Never in config files, logs, or memory nodes.
- **Pre-commit hooks**: Install with `pre-commit install`. Runs lint, gitleaks.
- **Confirmation gates**: Hardware, media capture, and upload tools require user confirmation. Non-TTY = default-deny.
- **Log scrubbing**: All log output passes through `SecretScrubber` which redacts fields matching `api_key`, `token`, `secret`, `authorization`, `credential`, `password`.

## Architecture

See `docs/architecture/design/qraft-architecture.md` for the full spec.

### Packages

| Package | Purpose | Key Types |
|---|---|---|
| `cmd/qraft/` | CLI entry point (Cobra). `init` subcommand scaffolds `~/.qraftworx/`. | `newInitCmd()` |
| `internal/cerebro/` | Cerebro `brain/` wrapper. Sole interface between Qraft and Cerebro. | `Client`, `NewClient()`, `NewClientWithGlobal()` |
| `internal/gemini/` | Gemini API client with retry, timeout, tool loop. | `Client`, `GenerateContentResult`, `FunctionCall`, `Part`, `TextPart`, `FunctionResponsePart`, `RunLoop()` |
| `internal/hydrator/` | Context assembly: memory search + sensor polling + token budgeting. | `Hydrator`, `HydratedContext`, `FormatForGemini()` |
| `internal/tools/` | Tool interface + all implementations. Registry for compile-time registration. | `Tool` (interface), `Registry`, `MemoryAddTool`, `MemorySearchTool`, `CaptureMediaTool`, `ProcessVideoTool`, `UploadTool`, `FFmpegBuilder`, `YouTubeUploader`, `TikTokUploader` |
| `internal/executor/` | Tool execution with confirmation gates, panic recovery, error caps. | `Executor`, `ConfirmFunc`, `ErrToolNotFound`, `ErrNonInteractiveDenied`, `ErrUserDenied`, `ErrTooManyErrors` |
| `internal/sensors/` | MQTT subscriber + Moonraker HTTP poller. Concurrent aggregation. | `SensorProvider` (interface), `Poller`, `MQTTSensor`, `MoonrakerSensor`, `PrinterState`, `ValueSchema` |
| `internal/config/` | TOML config loading + multi-error validation + defaults. | `Config`, `GeminiConfig`, `SensorConfig`, `MediaConfig`, `CostConfig`, `Duration` |
| `internal/logging/` | Structured JSON logging with secret scrubbing. | `Logger`, `SecretScrubber`, `InteractionLog`, `ToolCallLog` |
| `internal/cost/` | Daily API budget tracking with pre-call gate. | `Tracker`, `ErrBudgetExhausted`, `EstimateCost()` |
| `internal/safepath/` | Filesystem boundary enforcement via opaque path type. | `SafePath`, `New()`, `NewOutput()` |

### Key Interfaces

```go
// internal/tools/tool.go
type Tool interface {
    Name() string
    Description() string
    Parameters() map[string]any
    Execute(ctx context.Context, args json.RawMessage) (any, error)
    RequiresConfirmation() bool
    Permissions() ToolPermission
}

// internal/sensors/sensors.go
type SensorProvider interface {
    Name() string
    Poll(ctx context.Context) (map[string]any, error)
    Close() error
}

// internal/gemini/tools.go
type ToolDeclarer interface {
    Name() string
    Description() string
    Parameters() map[string]any
}

// internal/gemini/loop.go
type ToolExecutor interface {
    Execute(ctx context.Context, toolName string, args json.RawMessage) (any, error)
}
```

## Conventions

- Test fixtures in `testdata/` directories per package
- Use `t.TempDir()` for test databases and files
- Gemini tests use mock `generateFunc`, never live API
- ffmpeg integration tests guarded by `testing.Short()`
- Confirm gates: TTY detection injected via `WithTTY()` for testability
- Sensor tests use mock providers and `httptest.Server`
- MQTT tests use mock `MQTTClient` interface, no real broker
- Cost tracker tests inject `nowFunc` for time control

## Security Findings Addressed

| Finding | Severity | Mitigation |
|---|---|---|
| S1: Confirmation gate bypass | CRITICAL | ANSI stripping, non-TTY default-deny, injectable `ConfirmFunc` |
| S2: ffmpeg command injection | CRITICAL | `SafePath` for all paths, `exec.CommandContext` with arg slices, device from config only |
| S3: Indirect prompt injection | HIGH | Memory delimiters, anti-injection preamble, content length cap, injection pattern detection, typed sensor data |
| S4: Log file secrets | HIGH | 0600 file perms, `SecretScrubber` handler, sanitized tool summaries (no raw args) |
| S5: MQTT plaintext | HIGH | TLS required by default, `allow_insecure_mqtt` explicit opt-in, schema validation |
| S6: Cost control bypass | HIGH | Pre-call budget gate, file-locked counter, absent usage = max cost |
| S7: Upload path traversal | HIGH | `SafePath` validation, MIME header check, rate limiting (1/hour) |
| S9: Filesystem boundaries | MEDIUM | `SafePath` opaque type, symlink resolution, base directory enforcement |
| S10: HTTP timeouts | MEDIUM | `context.WithTimeout` on all external calls, configurable per-request timeout |
| S12: Supply chain | LOW | Dependency pinning, `govulncheck` in CI, `gitleaks` in pre-commit |

## Testing Patterns Per Package

- **cerebro**: Real `brain.Init()` with noop embedder in `t.TempDir()`. Tests Add/List/Get/Stats/Edges. Search requires embeddings so tested for error handling.
- **gemini**: Mock `generateFunc` injected into `Client`. No live API. `sleepFn` injected for fast retry tests. `atomic.Int32` for attempt counting.
- **hydrator**: Nil cerebro client (memories empty). Mock `SensorPoller` interface. Tests token budgeting and memory truncation.
- **tools**: `testCerebroClient()` helper with noop embedder. `safepath.New()` with `t.TempDir()` bases. `httptest.Server` for upload tests.
- **executor**: Mock `Tool` via struct with `execFn`. Injectable `ConfirmFunc`. Tests panic recovery, error caps, TTY gating.
- **sensors**: Mock `SensorProvider` with configurable delay/error. Mock `MQTTClient` with `simulateMessage()`. `httptest.Server` for Moonraker.
- **config**: Fixture TOML files in `testdata/`. Tests validation errors, defaults, duration parsing.
- **logging**: `bytes.Buffer` as log sink. Verifies JSON output, secret redaction, file permissions.
- **cost**: Injectable `nowFunc` for daily reset. `sync.WaitGroup` for concurrent safety. `t.TempDir()` for counter files.
- **safepath**: `t.TempDir()` bases with real files, symlinks, and traversal attempts.
