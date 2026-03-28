# QraftWorx CLI

QraftWorx CLI (`qraft`) is a Go command-line tool for AI-powered content automation. It uses Google Gemini as its reasoning engine and [Cerebro](https://github.com/coetzeevs/cerebro) as persistent memory to automate content creation workflows -- reading 3D printer telemetry, capturing video/image feeds from cameras, processing media with ffmpeg, and uploading to platforms like YouTube and TikTok.

## Architecture

```
  User prompt
       |
       v
 +-----------+     +----------+     +-----+
 | Hydrator  |---->| Gemini   |---->| LLM |
 | (context  |     | Client   |     |     |
 |  assembly)|     | (API +   |     +-----+
 +-----------+     |  retry)  |        |
   |      |        +----------+        |
   |      |             ^              v
   v      v             |        +-----------+
+------+ +------+       |        | Executor  |
|Cerebro| |Sensor|       |        | (confirm  |
|Memory | |Poller|       |        |  gates)   |
+------+ +------+       |        +-----------+
                         |              |
                         |              v
                    FunctionResponse  +----------------+
                         ^            | Tool Registry  |
                         |            |                |
                         +------------| memory_add     |
                                      | memory_search  |
                                      | capture_media  |
                                      | process_video  |
                                      | upload_media   |
                                      +----------------+
```

**Data flow:** The Hydrator assembles context from Cerebro memories and sensor state, then sends a hydrated prompt to Gemini. If Gemini requests tool calls, the Executor runs them through confirmation gates and the Tool Registry, then sends results back to Gemini. This loop continues until Gemini produces a text response or the iteration cap is reached.

## Features

| Feature | Description |
|---|---|
| **Persistent Memory** | Cerebro-backed semantic memory with vector search, stored in SQLite |
| **Gemini Tool Loop** | Explicit function-calling loop with retry, timeout, and iteration caps |
| **Sensor Integration** | MQTT subscriber and Moonraker HTTP poller for 3D printer telemetry |
| **Media Capture** | ffmpeg-based frame/video capture from V4L2 devices |
| **Video Processing** | ffmpeg transcoding with codec allowlists and parameter clamping |
| **Platform Upload** | YouTube and TikTok upload with OAuth, MIME validation, and rate limiting |
| **Cost Controls** | Daily budget tracking with pre-call gates and file-persisted counters |
| **Security Hardening** | SafePath filesystem boundaries, log scrubbing, confirmation gates, ANSI stripping |
| **Structured Logging** | JSON logging with secret redaction and interaction audit trail |
| **TOML Configuration** | Validated configuration with sensible defaults and multi-error reporting |

## Prerequisites

- **Go 1.24+** (module requires go 1.26.1 in go.mod)
- **C compiler** (CGO required for SQLite via `mattn/go-sqlite3` and `sqlite-vec`)
- **ffmpeg** (optional; required for media capture and video processing tools)
- **MQTT broker** (optional; required for MQTT sensor integration)

## Installation

### From source

```bash
git clone https://github.com/coetzeevs/qraftworx-cli.git
cd qraftworx-cli
CGO_ENABLED=1 go build -trimpath -o bin/qraft ./cmd/qraft
```

### Using `go install`

```bash
CGO_ENABLED=1 go install github.com/coetzeevs/qraftworx-cli/cmd/qraft@latest
```

## Quick Start

```bash
# 1. Set your Gemini API key
export GEMINI_API_KEY="your-key-here"

# 2. Initialize the configuration directory
qraft init

# 3. Edit config if needed
$EDITOR ~/.qraftworx/config.toml
```

The `init` command creates `~/.qraftworx/` with:
- `config.toml` -- default configuration (will not overwrite existing)
- `logs/` -- directory for structured JSON logs

## Configuration

Configuration lives at `~/.qraftworx/config.toml`. All sections have sensible defaults.

```toml
[gemini]
model = "gemini-2.5-pro"
max_tokens = 8192
max_tool_iterations = 10
timeout = "30s"

[cerebro]
project_dir = "~/.qraftworx/cerebro"

[sensors.moonraker]
type = "moonraker"
url = "http://localhost:7125"
poll_timeout = "5s"

[sensors.mqtt]
type = "mqtt"
broker_url = "tls://broker.example.com:8883"
topic = "printer/telemetry"
ca_cert = "~/.qraftworx/certs/ca.crt"
client_cert = "~/.qraftworx/certs/client.crt"
client_key = "~/.qraftworx/certs/client.key"

[media.webcam]
type = "v4l2"
device = "/dev/video0"
resolution = "1920x1080"

[media.gopro]
type = "http"
url = "http://gopro.local"

[logging]
path = "~/.qraftworx/logs"
level = "info"    # debug | info | warn | error

[cost]
daily_budget_usd = 5.00
warn_threshold_usd = 4.00
```

### Environment Variables

| Variable | Required | Description |
|---|---|---|
| `GEMINI_API_KEY` | Yes | Google Gemini API key |
| `YOUTUBE_OAUTH_TOKEN` | No | YouTube upload OAuth2 token |
| `TIKTOK_ACCESS_TOKEN` | No | TikTok Content Posting API token |

## Project Structure

```
qraftworx-cli/
├── cmd/qraft/                  CLI entry point (Cobra)
│   ├── main.go                 Root command setup
│   └── init.go                 `qraft init` subcommand
├── internal/
│   ├── cerebro/                Cerebro brain/ wrapper
│   │   └── client.go           Project + global brain access
│   ├── gemini/                 Gemini API client
│   │   ├── client.go           GenerateContent with retry/timeout
│   │   ├── retry.go            Exponential backoff (429/500/503)
│   │   ├── tools.go            Tool declaration builder
│   │   └── loop.go             Tool execution loop with cost tracking
│   ├── hydrator/               Context assembly
│   │   ├── hydrator.go         Memory search + sensor polling + token budgeting
│   │   └── format.go           Gemini-compatible part formatting (S3 sanitization)
│   ├── tools/                  Tool interface + implementations
│   │   ├── tool.go             Tool interface and ToolPermission
│   │   ├── registry.go         Compile-time tool registry
│   │   ├── memory.go           memory_add, memory_search
│   │   ├── capture.go          capture_media (ffmpeg frame capture)
│   │   ├── video.go            process_video (ffmpeg transcoding)
│   │   ├── ffmpeg.go           FFmpegBuilder (validated command construction)
│   │   ├── upload.go           upload_media (platform dispatcher + rate limiting)
│   │   ├── youtube.go          YouTube multipart upload
│   │   ├── tiktok.go           TikTok Content Posting API upload
│   │   └── mime.go             MIME type validation for uploads
│   ├── executor/               Tool execution engine
│   │   ├── executor.go         Confirmation gates, panic recovery, error caps
│   │   └── confirm.go          ANSI stripping, control char sanitization
│   ├── sensors/                Sensor polling
│   │   ├── sensors.go          SensorProvider interface
│   │   ├── poller.go           Concurrent multi-sensor aggregator
│   │   ├── mqtt.go             MQTT subscriber with schema validation
│   │   └── moonraker.go        Moonraker HTTP API client (Klipper)
│   ├── config/                 Configuration
│   │   ├── config.go           TOML loading + type definitions
│   │   └── validate.go         Multi-error validation with defaults
│   ├── logging/                Structured logging
│   │   ├── logging.go          JSON file logger (0600 perms)
│   │   ├── scrubber.go         Secret field redaction (slog.Handler)
│   │   └── interaction.go      InteractionLog + ToolCallLog types
│   ├── cost/                   API cost tracking
│   │   └── tracker.go          Daily budget, pre-call gate, file counter
│   └── safepath/               Filesystem boundary enforcement
│       └── safepath.go         SafePath opaque type (symlink-aware)
├── docs/architecture/          Design and execution documents
├── .golangci.yml               Linter configuration
├── .pre-commit-config.yaml     Pre-commit hooks
├── .goreleaser.yml             Release configuration
├── Makefile                    Build, test, lint, CI targets
└── go.mod                      Module definition
```

## Development

### Build

```bash
# Build binary
make build
# or directly:
CGO_ENABLED=1 go build -trimpath -o bin/qraft ./cmd/qraft
```

### Test

```bash
# Full test suite with race detector
make test

# Short tests (skips ffmpeg integration tests)
make test-short

# Run specific package tests
CGO_ENABLED=1 go test ./internal/safepath/... -race
```

### Lint

```bash
# Run all linters
make lint
# or:
golangci-lint run --config=.golangci.yml
```

### All CI Checks

```bash
make ci    # runs: lint, test, govulncheck
```

### Pre-commit Hooks

```bash
make hooks    # installs pre-commit hooks
```

### Coverage

```bash
make cover-html    # generates and opens HTML coverage report
```

## Security Model

QraftWorx has a defense-in-depth security model addressing 13 security findings (S1-S13).

### Key Security Properties

- **SafePath type** (`internal/safepath/`): All filesystem operations use an opaque `SafePath` type that validates paths are absolute, cleaned, symlink-resolved, and within allowed base directories. Tools accept `SafePath`, never raw strings.

- **Confirmation gates** (`internal/executor/`): Tools that control hardware, capture media, or upload content require explicit user confirmation. Non-TTY environments default-deny all confirmation-required tools. Tool arguments are stripped of ANSI escape sequences before display.

- **Secret management**: API keys are read from environment variables only, never stored in config files or logs. The `SecretScrubber` slog handler redacts fields containing `api_key`, `token`, `secret`, `authorization`, `credential`, or `password`.

- **MQTT transport security** (`internal/sensors/mqtt.go`): Plaintext MQTT (`tcp://`) is rejected by default. Requires explicit `allow_insecure_mqtt = true` opt-in. TLS/SSL endpoints are required.

- **ffmpeg command injection prevention** (`internal/tools/ffmpeg.go`): Device paths come from config only, never from LLM arguments. Commands use `exec.CommandContext` with separate argument slices. Codecs are validated against an allowlist. FPS and duration are clamped to safe ranges.

- **Cost controls** (`internal/cost/`): Pre-call budget gate prevents API calls when daily budget would be exceeded. Missing usage metadata is treated as maximum possible cost. File-persisted daily counter resets at midnight UTC.

- **Upload hardening** (`internal/tools/upload.go`): Files must be within the configured media directory (symlink-resolved). MIME types are validated by reading file headers. Rate limited to 1 upload per hour window.

For the full security model, see [SECURITY.md](SECURITY.md).

## Architecture Documentation

- [Architecture Specification](docs/architecture/design/qraft-architecture.md) -- full system design, security assessment, decision log
- [Implementation Plan](docs/architecture/execution/implementation-plan.md) -- TDD-ordered phase breakdown
- [Implementation Status](docs/architecture/design/IMPLEMENTATION-STATUS.md) -- what was built per phase
- [Engineering Standards](docs/architecture/execution/engineering-standards.md) -- coding conventions

## License

This project is licensed under the MIT License. See [LICENSE](LICENSE) for details.
