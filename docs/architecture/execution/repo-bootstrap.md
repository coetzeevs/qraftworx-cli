# Qraft CLI -- Repository Bootstrap Guide

**Status:** Draft
**Date:** 2026-03-27
**Companion to:** [implementation-plan.md](implementation-plan.md), [engineering-standards.md](engineering-standards.md)

---

## 1. Repository Decision: Separate Repo

**Qraft lives in a new repository: `github.com/coetzeevs/qraftworx-cli`**

### Rationale

| Concern | Separate Repo | Monorepo (extend Cerebro) |
|---|---|---|
| **Separation of concerns** | Cerebro is a memory library. Qraft is an orchestrator. Different release cadences, different users. | Bloats Cerebro with Gemini SDK, MQTT, ffmpeg, platform API deps. |
| **Dependency direction** | Qraft depends on Cerebro (one-way). Clean. | Circular risk: Cerebro internal types leaking into Qraft, or Qraft concerns bleeding into brain/. |
| **Go module boundary** | Separate `go.mod`. Cerebro releases are tagged; Qraft pins a specific version. | Single `go.mod` with all dependencies. Every `go get` pulls Gemini SDK even for Cerebro-only users. |
| **CI isolation** | Qraft CI installs ffmpeg, MQTT broker, etc. Cerebro CI stays lean. | One CI pipeline for everything. Slow, complex, fragile. |
| **ADR-006 compliance** | Cerebro remains pure storage (Model B). Qraft is cognition. Physical repo boundary enforces the architectural boundary. | Temptation to "just add one more function" to brain/ for Qraft's convenience. |

### How Qraft Consumes Cerebro

```go
// qraftworx/go.mod
module github.com/coetzeevs/qraftworx-cli

go 1.24.0

require (
    github.com/coetzeevs/cerebro v1.1.1  // pinned to latest tagged release
)
```

**Version will update** to the release created by Task 0.1 (after `brain/types.go` is added to Cerebro).

Qraft imports `brain/` only (after Task 0.1 re-exports types):

```go
import "github.com/coetzeevs/cerebro/brain"

// Use re-exported types directly:
// brain.Node, brain.ScoredNode, brain.NodeType, brain.Episode, etc.
```

No `internal/store` import needed. Task 0.1 (Section 4) re-exports all required types from the `brain/` package.

---

## 2. Carrying Over Design Context

The design and architecture documents live in the Cerebro repo today. They must be copied to the new repo so context remains available to the agent and developers.

### Step-by-step

```bash
# 1. Create the new repo
mkdir -p ~/projects/agentic/qraftworx
cd ~/projects/agentic/qraftworx
git init

# 2. Copy ALL design and architecture docs
mkdir -p docs/architecture
cp -r ~/projects/agentic/cerebro/docs/architecture/qraft-cli/* docs/architecture/

# 3. Verify the structure
tree docs/
# Expected:
# docs/
# └── architecture/
#     ├── design/
#     │   ├── nvs/
#     │   │   ├── ADR-NVS-001-vision-daemon-process-model.md
#     │   │   ├── ADR-NVS-002-frame-sharing-mechanism.md
#     │   │   ├── HANDOFF.md
#     │   │   ├── nvs-architecture.md
#     │   │   └── nvs-interfaces.go
#     │   ├── nvs-feature-spec.md
#     │   ├── qraft-architecture.md        <-- primary architecture spec
#     │   ├── qraft-cli-review.md          <-- architect + tech lead critique
#     │   ├── qraft-cli-support.md         <-- original proposal (historical)
#     │   └── qraft-vision-feature.md      <-- original NVS request (historical)
#     └── execution/
#         ├── implementation-plan.md        <-- TDD phase plan (this plan)
#         ├── engineering-standards.md      <-- CI, testing, tooling
#         └── repo-bootstrap.md            <-- this document

# 4. Initialize Go module
go mod init github.com/coetzeevs/qraftworx-cli

# 5. Add dependencies (pin to specific versions, never use @latest)
go get github.com/coetzeevs/cerebro@v1.1.1  # update to post-Task-0.1 release tag
go get google.golang.org/genai@v1.x.x       # pin to latest stable at time of bootstrap
go get github.com/spf13/cobra@v1.10.2
go get github.com/eclipse/paho.mqtt.golang@v1.x.x  # pin to latest stable at time of bootstrap
go get github.com/BurntSushi/toml@v1.x.x            # pin to latest stable at time of bootstrap
go get golang.org/x/term@v0.x.x                     # pin to latest stable at time of bootstrap

# 6. Create directory scaffold (Phase 0, Task 0.4)
mkdir -p cmd/qraft
mkdir -p internal/{cerebro,gemini,hydrator,tools,executor,sensors,config,logging,cost,safepath}
```

### CLAUDE.md for the new repo

Create `CLAUDE.md` at the repo root with project conventions. This is the agent's primary instruction file.

```markdown
# QraftWorx

Go CLI for AI-powered content automation. Uses Gemini as reasoning engine, Cerebro as persistent memory.

## Development

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
```

---

## 3. Phase 0 Execution Checklist

After creating the repo and copying docs, execute these tasks to complete Phase 0:

```
[ ] Cerebro: create brain/types.go (re-export store types for external modules)
[ ] Cerebro: tag new release (e.g., v1.2.0) after brain/types.go lands
[ ] go mod init github.com/coetzeevs/qraftworx-cli
[ ] go get all dependencies pinned to specific versions (cerebro@v1.2.0, etc. -- never @latest)
[ ] Create directory scaffold with package declarations
[ ] Create cmd/qraft/main.go (empty main, package main)
[ ] go build ./cmd/qraft produces a binary
[ ] Create .golangci.yml (from engineering-standards.md)
[ ] golangci-lint run passes
[ ] Create .pre-commit-config.yaml (from engineering-standards.md)
[ ] Create .gitleaks.toml (from engineering-standards.md)
[ ] pre-commit install
[ ] pre-commit run --all-files passes
[ ] Create .github/workflows/ci.yml (from engineering-standards.md)
[ ] Create .github/workflows/release.yml (from engineering-standards.md)
[ ] Create .goreleaser.yml (from engineering-standards.md)
[ ] goreleaser check passes
[ ] Create .gitignore (from engineering-standards.md)
[ ] Create .env.example
[ ] Create Makefile (from engineering-standards.md)
[ ] Create CLAUDE.md
[ ] Create initial README.md (minimal: project name, one-line description, build instructions)
[ ] git add, initial commit, push to GitHub
[ ] Verify CI runs on push
[ ] govulncheck ./... clean in CI
```

---

## 4. Cerebro Changes Needed (Phase 0, Task 0.1 -- Prerequisite)

This is the **first task in Phase 0**, executed in the Cerebro repo before Qraft's `go.mod` is created.

### 4.1 Type Re-Export (Required)

Qraft needs access to types that appear in `brain/`'s public API signatures but live in `internal/store/`:

- `store.Node` -- returned by `brain.List()`
- `store.ScoredNode` -- returned by `brain.Search()`
- `store.NodeType` -- parameter to `brain.Add()`
- `store.ListNodesOpts` -- parameter to `brain.List()`
- `store.NodeWithEdges` -- returned by `brain.Get()`
- `store.Stats` -- returned by `brain.Stats()`

Go's module system prevents external modules from importing `internal/` packages. These types must be re-exported from `brain/`.

**Create `brain/types.go` in the Cerebro repo:**

```go
package brain

import "github.com/coetzeevs/cerebro/internal/store"

// Re-exported types for external consumers of the brain/ package.
type (
    Node          = store.Node
    ScoredNode    = store.ScoredNode
    NodeWithEdges = store.NodeWithEdges
    NodeType      = store.NodeType
    ListNodesOpts = store.ListNodesOpts
    Stats         = store.Stats
)

// Re-exported node type constants.
const (
    Episode    = store.Episode
    Concept    = store.Concept
    Procedure  = store.Procedure
    Reflection = store.Reflection
)
```

### 4.2 Tagged Release (Required)

After `brain/types.go` lands on main, tag a new Cerebro release. The latest release is currently `v1.1.1`. The new tag should be `v1.2.0` (minor bump for new public API surface).

Qraft's `go.mod` then pins to this exact tag:

```
go get github.com/coetzeevs/cerebro@v1.2.0
```

---

## 5. Document Reference Map

For the agent working in the qraftworx repo, here's where to find everything:

| Question | Document |
|---|---|
| What are we building? | `docs/architecture/design/qraft-architecture.md` |
| What are the security requirements? | `docs/architecture/design/qraft-architecture.md`, Appendix A |
| What's the NVS feature? | `docs/architecture/design/nvs-feature-spec.md` |
| What's the NVS detailed design? | `docs/architecture/design/nvs/nvs-architecture.md` |
| What order do we build in? | `docs/architecture/execution/implementation-plan.md` |
| What are the CI/testing standards? | `docs/architecture/execution/engineering-standards.md` |
| How was the repo set up? | `docs/architecture/execution/repo-bootstrap.md` (this doc) |
| What was the original proposal? | `docs/architecture/design/qraft-cli-support.md` (historical) |
| What was the critique? | `docs/architecture/design/qraft-cli-review.md` (historical) |
