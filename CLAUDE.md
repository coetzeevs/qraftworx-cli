# QraftWorx

Go CLI for AI-powered content automation. Uses Gemini as reasoning engine, Cerebro as persistent memory.

## Development

```bash
# Build
go build -o bin/qraft ./cmd/qraft

# Test
go test ./... -race

# Test (short, no ffmpeg)
go test ./... -race -short

# Lint
golangci-lint run

# All CI checks locally
make ci
```

## Key Patterns

- **CGO required**: Cerebro dependency (mattn/go-sqlite3 + sqlite-vec)
- **TDD (strict)**: Write failing test first. Red -> Green -> Refactor. No production code without a covering test.
- **SafePath**: All filesystem operations use SafePath type (internal/safepath/), never raw strings
- **Compile-time tools**: All tools registered at compile time in internal/tools/. No dynamic loading.
- **Secrets in env vars**: GEMINI_API_KEY and all API keys via environment variables only. Never in config files, logs, or memory nodes.
- **Pre-commit hooks**: Install with `pre-commit install`

## Architecture

See docs/architecture/design/qraft-architecture.md for the full spec.

- cmd/qraft/          CLI entry point (Cobra)
- internal/cerebro/   Cerebro brain/ wrapper
- internal/gemini/    Gemini API client, tool loop, retry logic
- internal/hydrator/  Context assembly, token budgeting
- internal/tools/     Tool interface + implementations
- internal/executor/  Tool execution, confirmation gates, permission checks
- internal/sensors/   MQTT + HTTP sensor polling
- internal/config/    TOML config loading + validation
- internal/logging/   Structured JSON logging
- internal/cost/      API cost tracking
- internal/safepath/  SafePath type for filesystem boundary enforcement

## Conventions

- Test fixtures in testdata/ directories per package
- Use t.TempDir() for test databases and files
- Gemini tests use recorded responses (testdata/*.json), never live API
- ffmpeg integration tests guarded by testing.Short()
- Confirm gates: TTY detection injected for testability
