# Qraft CLI -- Engineering Standards, CI, and Testing Strategy

**Status:** Draft
**Date:** 2026-03-27
**Companion to:** [implementation-plan.md](implementation-plan.md)
**Repository:** github.com/coetzeevs/qraftworx-cli

---

## 1. CI Pipeline

### `.github/workflows/ci.yml`

```yaml
name: CI

on:
  push:
    branches:
      - main
      - "feature/**"
      - "feat/**"
      - "fix/**"
      - "architecture/**"
  pull_request:
    branches:
      - main

permissions:
  contents: read

env:
  CGO_ENABLED: "1"

jobs:
  lint:
    name: Lint
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with:
          go-version-file: go.mod
      - name: Install C compiler (CGO for brain/)
        run: sudo apt-get update && sudo apt-get install -y gcc
      - name: golangci-lint
        uses: golangci/golangci-lint-action@v7
        with:
          version: v2.11

  test:
    name: Test
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with:
          go-version-file: go.mod
      - name: Install system dependencies
        run: |
          sudo apt-get update
          sudo apt-get install -y gcc ffmpeg
      - name: Run tests
        run: go test ./... -race -coverprofile=coverage.out -covermode=atomic
      - name: Show coverage
        run: go tool cover -func=coverage.out
      - name: Enforce minimum coverage
        run: |
          TOTAL=$(go tool cover -func=coverage.out | grep total | awk '{print $3}' | tr -d '%')
          echo "Total coverage: ${TOTAL}%"
          if [ "$(echo "$TOTAL < 70.0" | bc)" -eq 1 ]; then
            echo "::error::Coverage ${TOTAL}% is below 70% threshold"
            exit 1
          fi

  test-short:
    name: Test (short, no ffmpeg)
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with:
          go-version-file: go.mod
      - name: Install C compiler
        run: sudo apt-get update && sudo apt-get install -y gcc
      - name: Run short tests
        run: go test ./... -race -short

  govulncheck:
    name: Vulnerability Check
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with:
          go-version-file: go.mod
      - name: Install govulncheck
        run: go install golang.org/x/vuln/cmd/govulncheck@latest
      - name: Run govulncheck
        run: govulncheck ./...

  goreleaser-check:
    name: GoReleaser Check
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
        with:
          fetch-depth: 0
      - uses: actions/setup-go@v5
        with:
          go-version-file: go.mod
      - name: Install C compiler
        run: sudo apt-get update && sudo apt-get install -y gcc
      - uses: goreleaser/goreleaser-action@v6
        with:
          version: "~> v2"
          args: check
```

### `.github/workflows/release.yml`

```yaml
name: Release

on:
  push:
    tags:
      - "v*"

permissions:
  contents: write

jobs:
  release:
    name: Release
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
        with:
          fetch-depth: 0
      - uses: actions/setup-go@v5
        with:
          go-version-file: go.mod
      - name: Install cross-compilation toolchains
        run: |
          sudo apt-get update
          sudo apt-get install -y gcc gcc-aarch64-linux-gnu
      - uses: goreleaser/goreleaser-action@v6
        with:
          version: "~> v2"
          args: release --clean
        env:
          GITHUB_TOKEN: ${{ secrets.GITHUB_TOKEN }}
```

### Design Notes

- **CGO is required** everywhere because Cerebro's `brain/` uses `mattn/go-sqlite3` + `sqlite-vec`.
- **ffmpeg in CI:** The `test` job installs ffmpeg for integration tests. The `test-short` job skips these (guarded by `testing.Short()`).
- **No live Gemini API calls in CI.** All Gemini tests use recorded HTTP fixtures via `httptest.Server`.
- **Coverage threshold:** 70% minimum, enforced in CI. Raise to 80% after Phase 4.

---

## 2. Pre-commit Hooks

### `.pre-commit-config.yaml`

```yaml
repos:
  - repo: https://github.com/golangci/golangci-lint
    rev: v2.11.2
    hooks:
      - id: golangci-lint
        args: [--config=.golangci.yml]

  - repo: https://github.com/dnephin/pre-commit-golang
    rev: v0.5.1
    hooks:
      - id: go-fmt
      - id: go-mod-tidy

  - repo: https://github.com/gitleaks/gitleaks
    rev: v8.24.0
    hooks:
      - id: gitleaks

  - repo: local
    hooks:
      - id: go-build
        name: go build
        entry: go build ./...
        language: system
        pass_filenames: false
        types: [go]

      - id: go-test
        name: go test
        entry: go test ./... -race -short
        language: system
        pass_filenames: false
        types: [go]
```

### `.gitleaks.toml`

```toml
title = "Qraft gitleaks config"

[extend]
useDefault = true

[[rules]]
id = "gemini-api-key"
description = "Google AI / Gemini API Key"
regex = '''AIza[0-9A-Za-z\-_]{35}'''
tags = ["key", "google"]

[[allowlist]]
paths = [
    '''testdata/.*\.json''',
    '''testdata/.*\.golden''',
]
```

### Pre-commit vs CI Parity

| Check | Pre-commit | CI |
|---|---|---|
| `gofmt` | yes | yes (via golangci-lint) |
| `go mod tidy` | yes | implicit (build fails if dirty) |
| `go build` | yes | yes (implicit in test) |
| `golangci-lint` | yes (same config) | yes (same config) |
| `go test -race -short` | yes | yes (`test-short` job) |
| `go test -race` (full) | no (too slow) | yes (`test` job) |
| `gitleaks` | yes | no (secrets in CI managed via GitHub) |
| `govulncheck` | no (too slow) | yes |
| `goreleaser check` | no | yes |
| Coverage threshold | no | yes (70%) |

---

## 3. Testing Patterns

### 3.1 Gemini Client: Record/Replay

Use `httptest.Server` with golden file fixtures. No third-party recorder library.

```go
func testGeminiServer(t *testing.T, fixtureName string) *httptest.Server {
    t.Helper()
    fixture, _ := os.ReadFile(filepath.Join("testdata", fixtureName))
    srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        w.Header().Set("Content-Type", "application/json")
        w.Write(fixture)
    }))
    t.Cleanup(srv.Close)
    return srv
}
```

For the tool loop, use an interface-level mock:

```go
type Generator interface {
    GenerateContent(ctx context.Context, parts ...genai.Part) (*genai.GenerateContentResponse, error)
}
```

Unit tests inject a `Generator` stub returning pre-built responses with `FunctionCall` parts.

### 3.2 brain/ Integration: Real SQLite

```go
func testBrain(t *testing.T) *brain.Brain {
    t.Helper()
    path := filepath.Join(t.TempDir(), "test.sqlite")
    b, err := brain.Init(path, brain.EmbedConfig{Provider: "none"})
    if err != nil { t.Fatalf("brain.Init: %v", err) }
    t.Cleanup(func() { _ = b.Close() })
    return b
}
```

Use `Provider: "none"` for unit tests (disables embedding, all CRUD works). No mocking `brain/`.

### 3.3 Executor / Confirmation Gates

Inject TTY check and stdin as dependencies:

```go
type Executor struct {
    isTerminal func(fd int) bool  // production: term.IsTerminal
    stdin      io.Reader           // production: os.Stdin
    stderr     io.Writer           // production: os.Stderr
}
```

Tests inject `func(int) bool { return false }` for non-TTY and `strings.NewReader("y\n")` for stdin.

### 3.4 HTTP Sensors: `httptest.Server`

Each sensor client test starts a local HTTP server returning fixture JSON. Test both success and timeout paths.

### 3.5 MQTT Sensors: In-Process Mock Broker

Use `github.com/mochi-mqtt/server/v2` as an embeddable test broker:

```go
func testBroker(t *testing.T) (*mqtt.Server, string) {
    broker := mqtt.New(nil)
    _ = broker.AddHook(new(auth.AllowHook), nil)
    ln, _ := net.Listen("tcp", "127.0.0.1:0")
    tcp := listeners.NewTCP(listeners.Config{ID: "test", Address: ln.Addr().String()})
    _ = broker.AddListenerWithListener(tcp, ln)
    go broker.Serve()
    t.Cleanup(func() { _ = broker.Close() })
    return broker, "tcp://" + ln.Addr().String()
}
```

### 3.6 ffmpeg Subprocess

**Unit tests:** Verify `FFmpegBuilder` produces correct argument slices. No subprocess execution.

```go
func TestFFmpegBuilder_CaptureFrame_Args(t *testing.T) {
    cmd := builder.CaptureFrame(device, output)
    expected := []string{"-f", "v4l2", "-i", "/dev/video0", "-frames:v", "1", "-y", "/tmp/out.jpg"}
    if !slices.Equal(cmd.Args[1:], expected) {
        t.Errorf("args mismatch: got %v, want %v", cmd.Args[1:], expected)
    }
}
```

**Integration tests** (guarded by `if testing.Short() { t.Skip() }`): Use `ffmpeg -f lavfi -i testsrc=...` as synthetic input.

### 3.7 SafePath Edge Cases

Test: traversal (`../../`), symlinks outside base, relative paths, valid paths. Document TOCTOU race (double-validate at construction AND before exec).

### 3.8 Cost Tracker: File-Locked Concurrent Access

Spawn 10 goroutines each calling `RecordUsage(0.10)`. Assert final total is `$1.00`. Uses `syscall.Flock`.

### 3.9 Structured Logging

Inject `io.Writer` (production: file, test: `bytes.Buffer`). Assert JSON is valid and secrets are redacted:

```go
func TestLogger_ToolCallRedactsSecrets(t *testing.T) {
    var buf bytes.Buffer
    logger := NewLogger(LogConfig{Writer: &buf})
    logger.LogToolCall(ToolCallLog{Args: `{"api_key": "AIzaSyBad"}`})
    if strings.Contains(buf.String(), "AIzaSyBad") {
        t.Fatal("API key leaked")
    }
}
```

---

## 4. Test Data and Fixtures

```
internal/
  gemini/testdata/
    text-response.json              # Simple text reply
    tool-call-single.json           # Single function call
    tool-call-multi-turn.json       # Multi-turn tool loop
    error-rate-limit.json           # 429 response
    error-server.json               # 500 response
    malformed-function-call.json    # Invalid function_call
  sensors/testdata/
    moonraker/printer-printing.json
    moonraker/printer-idle.json
    gopro/status-recording.json
  media/testdata/
    frame-640x480.jpg               # Minimal valid JPEG
  config/testdata/
    valid.toml
    invalid_syntax.toml
    missing_model.toml
    mqtt_plaintext.toml
```

**Gemini fixture management:**
1. Record once manually with a real API key (developer utility, not committed).
2. Redact all API keys from request headers before committing.
3. Store as plain JSON, one file per scenario.

**Synthetic video:** Use `ffmpeg -f lavfi -i testsrc=...` directly in tests. Pre-generate a minimal JPEG for frame buffer tests and commit it.

---

## 5. Secret Management

### Local Development

```bash
# .env.example (committed)
GEMINI_API_KEY=

# .env (git-ignored, developer fills in)
GEMINI_API_KEY=AIza...
```

`.gitignore` entry:
```
.env
.env.*
!.env.example
```

Makefile `run` target loads `.env`:
```makefile
run:
	@test -f .env && export $$(grep -v '^#' .env | xargs) || true; \
	go run ./cmd/qraft $(ARGS)
```

### Rules

- CLI validates `GEMINI_API_KEY` at startup, fails with clear error if missing.
- Never log the API key. The Gemini client wrapper must implement `slog.LogValuer` to redact.
- Test code uses `"test-key-not-real"` as the API key.
- gitleaks in pre-commit prevents accidental commits.

### CI Secrets

No Gemini API key in CI. If future live integration tests are needed, use a `workflow_dispatch` workflow with a GitHub Environment requiring reviewer approval.

---

## 6. Code Quality Gates

### `.golangci.yml`

```yaml
version: "2"

formatters:
  enable:
    - gofmt

linters:
  enable:
    - errcheck
    - govet
    - staticcheck
    - unused
    - ineffassign
    - gosec
    - gocritic
    - misspell
    - bodyclose
    - noctx
    - exhaustive
    - sqlclosecheck

  settings:
    errcheck:
      check-type-assertions: true
    gocritic:
      enabled-tags:
        - diagnostic
        - style
        - performance
    gosec:
      excludes:
        - G301
        - G304
    exhaustive:
      default-signifies-exhaustive: true

  exclusions:
    rules:
      - linters: [gosec]
        text: "G201|G202"
      - linters: [gocritic]
        path: "_test\\.go$"
        text: "hugeParam"
```

**Why these linters:** `bodyclose` and `noctx` catch HTTP resource leaks. `exhaustive` catches missing enum cases (printer state, tool permissions, hardware tiers). `sqlclosecheck` catches unclosed SQL resources.

### `.goreleaser.yml`

```yaml
version: 2

builds:
  - main: ./cmd/qraft
    binary: qraft
    env:
      - CGO_ENABLED=1
    goos:
      - linux
    goarch:
      - amd64
      - arm64
    flags:
      - -trimpath
    ldflags:
      - -s -w
      - -X main.version={{.Version}}
      - -X main.commit={{.Commit}}
    overrides:
      - goos: linux
        goarch: arm64
        env:
          - CC=aarch64-linux-gnu-gcc

archives:
  - formats:
      - tar.gz
    name_template: "{{ .ProjectName }}_{{ .Version }}_{{ .Os }}_{{ .Arch }}"

changelog:
  disable: true

checksum:
  name_template: checksums.txt
```

### `Makefile`

```makefile
.PHONY: build test test-short lint fmt vet vuln clean run cover-html hooks ci

build:
	CGO_ENABLED=1 go build -trimpath -o bin/qraft ./cmd/qraft

test:
	CGO_ENABLED=1 go test ./... -race -coverprofile=coverage.out -covermode=atomic
	go tool cover -func=coverage.out

test-short:
	CGO_ENABLED=1 go test ./... -race -short

lint:
	golangci-lint run --config=.golangci.yml

fmt:
	gofmt -w .

vet:
	go vet ./...

vuln:
	govulncheck ./...

clean:
	rm -rf bin/ coverage.out

run:
	@test -f .env && export $$(grep -v '^#' .env | xargs) || true; \
	go run ./cmd/qraft $(ARGS)

cover-html: test
	go tool cover -html=coverage.out

hooks:
	pre-commit install
	@echo "Pre-commit hooks installed."

ci: lint test vuln
	@echo "All CI checks passed."
```

### `.gitignore`

```gitignore
# Binaries
bin/
qraft

# Test artifacts
coverage.out

# Secrets
.env
.env.*
!.env.example

# IDE
.idea/
.vscode/
*.swp

# OS
.DS_Store

# Media working directory
vault/
```
