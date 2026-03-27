# Qraft CLI -- TDD Implementation Plan

**Status:** Draft
**Date:** 2026-03-27
**Companion to:** [qraft-architecture.md](../design/qraft-architecture.md), [nvs-feature-spec.md](../design/nvs-feature-spec.md)
**Repository:** github.com/coetzeevs/qraftworx-cli

---

## Overview

This document expands the 8 architecture phases into concrete, TDD-ordered implementation tasks. Every task follows red-green-refactor: write a failing test, make it pass, clean up. Security findings are woven into the phase where they are structurally relevant, not bolted on after.

### Conventions

- **Test file naming:** `foo_test.go` alongside `foo.go`
- **Test function naming:** `TestTypeName_MethodName_Scenario`
- **Test fixtures:** `testdata/` directories per package
- **Temp state:** `t.TempDir()` for any file-based tests
- **Context7 verified:** Gemini Go SDK (`google.golang.org/genai`), Paho MQTT (`github.com/eclipse/paho.mqtt.golang`), BurntSushi TOML (`github.com/BurntSushi/toml`)

---

## Phase 0: Project Bootstrap

**Goal:** Empty, building, linted, CI-enabled repository with all dependencies wired.

### Tasks

| # | Task | Files and/or Folders Created and/or Updated | DoD |
|---|------|---------------|-----|
| 0.1 | **Cerebro: re-export store types from brain/** (see Section "Cerebro Prerequisite" below) | `brain/types.go` in **cerebro** repo | `brain.Episode`, `brain.Concept`, `brain.Procedure`, `brain.Reflection` and key types accessible to external modules. Tagged as new release. |
| 0.2 | `go mod init github.com/coetzeevs/qraftworx-cli` | `go.mod` | Module compiles |
| 0.3 | Add Cerebro dependency: `go get github.com/coetzeevs/cerebro@v1.1.1` (pin to latest release) | `go.mod`, `go.sum` | `go build ./...` succeeds |
| 0.4 | Add core dependencies (pinned versions, no `@latest`) | `go.mod`, `go.sum` | All imports resolve |
| 0.5 | Create directory scaffold (empty `.go` files with `package` declarations) | See tree below | `go build ./...` succeeds, `go vet ./...` clean |
| 0.6 | Configure golangci-lint | `.golangci.yml` | `golangci-lint run` passes |
| 0.7 | Configure pre-commit hooks | `.pre-commit-config.yaml` | `pre-commit run --all-files` passes |
| 0.8 | Create CI pipeline (GitHub Actions) | `.github/workflows/ci.yml` | Push triggers lint + test + govulncheck (S12) |
| 0.9 | Create GoReleaser config | `.goreleaser.yml` | `goreleaser check` passes, targets linux/amd64 + linux/arm64 |
| 0.10.1 | Add `gitleaks` to pre-commit (N1 remediation) | `.pre-commit-config.yaml`, `.gitleaks.toml` | Pre-commit rejects hardcoded API keys |
| 0.10.2 | Add `gitleaks` to pre-commit (N1 remediation) in `github.com/coetzeevs/cerebro` | `.pre-commit-config.yaml`, `.gitleaks.toml` | Pre-commit rejects hardcoded API keys |
| 0.11 | Create CLAUDE.md project instructions | `CLAUDE.md` | Documents TDD discipline, project conventions |
| 0.12 | Init cerebro for project development usage in qraftworx | `CLAUDE.md`, `.claude/` | Cerebro memory system configured for the new repo |
| 0.13 | Create `docs/architecture/qraft-cli/execution/` directory structure | `docs/architecture/qraft-cli/execution/` | Directory created with all subdirectories and files listed in the implementation plan |

### Cerebro Prerequisite (Task 0.1)

Before Qraft can import `brain/` from an external Go module, Cerebro must re-export types that appear in `brain/`'s public API but live in `internal/store/`. External modules cannot import `internal/` packages.

**In the Cerebro repo**, create `brain/types.go`:

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

Tag a new Cerebro release after this change (e.g., `v1.2.0`), then pin Qraft's `go.mod` to that version in Task 0.3.

### Directory Scaffold (Task 0.4)

```
qraftworx/
  cmd/qraft/
    main.go              // package main, empty main()
  internal/
    cerebro/
      client.go          // package cerebro
    gemini/
      client.go          // package gemini
    hydrator/
      hydrator.go        // package hydrator
    tools/
      tool.go            // package tools
    executor/
      executor.go        // package executor
    sensors/
      sensors.go         // package sensors
    config/
      config.go          // package config
    logging/
      logging.go         // package logging
    cost/
      cost.go            // package cost
    safepath/
      safepath.go        // package safepath
```

### Dependencies (Task 0.3)

```
github.com/coetzeevs/cerebro@v1.1.1       // brain/ package (pin to latest release; update after Task 0.1)
google.golang.org/genai@v1.x.x           // Gemini Go SDK (pin to latest stable at time of bootstrap)
github.com/spf13/cobra@v1.10.2           // CLI framework
github.com/eclipse/paho.mqtt.golang@v1.x // MQTT client (pin to latest stable at time of bootstrap)
github.com/BurntSushi/toml@v1.x.x        // TOML config (pin to latest stable at time of bootstrap)
golang.org/x/term@v0.x.x                 // TTY detection (pin to latest stable at time of bootstrap)
```

### CI Pipeline (Task 0.7)

```yaml
jobs:
  lint-test:
    steps:
      - golangci-lint run
      - go test ./... -race -coverprofile=coverage.out
      - govulncheck ./...     # S12: supply chain
      - go build -o /dev/null ./cmd/qraft
```

### Security Findings Addressed

- **S12 (supply chain):** `govulncheck` in CI, dependency pinning in `go.sum`
- **N1 (hardcoded keys):** `gitleaks` in pre-commit pipeline

### Acceptance Criteria

- [ ] Cerebro `brain/types.go` created and tagged as new release
- [ ] `go build ./cmd/qraft` produces a binary
- [ ] `go.mod` pins Cerebro to a specific tagged version (no `@latest`)
- [ ] All dependencies pinned to specific versions in `go.mod`
- [ ] `go test ./...` passes (no tests yet, but no compilation errors)
- [ ] `golangci-lint run` clean
- [ ] `pre-commit run --all-files` passes
- [ ] CI pipeline runs on push
- [ ] `govulncheck ./...` clean

### Estimated New Files: 18-20 (qraftworx) + 1 (cerebro: brain/types.go)

---

## Phase 1: CerebroClient Wrapper + SafePath

**Goal:** A Go package wrapping `brain.Brain` with the subset of operations Qraft needs, plus the `SafePath` type used across all subsequent phases.

### Key Types

```go
// internal/safepath/safepath.go

// SafePath is an opaque type representing a validated filesystem path.
// It guarantees the path:
//   - Is absolute
//   - Has been cleaned (no .., double slashes)
//   - Resolves within one of the allowed base directories
//   - Has had symlinks evaluated (via filepath.EvalSymlinks)
//
// SafePath values can only be constructed via New(), which performs all validation.
// Tools accept SafePath, never raw strings, for filesystem arguments.
type SafePath struct {
    resolved string // the validated, absolute, symlink-resolved path
}

// New validates and constructs a SafePath. Returns error if the path
// escapes the allowed bases or contains traversal sequences.
func New(raw string, allowedBases []string) (SafePath, error)

// String returns the resolved path.
func (p SafePath) String() string
```

```go
// internal/cerebro/client.go

// Client wraps brain.Brain with the operations Qraft needs.
// It is the sole interface between Qraft and Cerebro.
type Client struct {
    brain  *brain.Brain
    global *brain.Brain // nil if no global store
}

// NewClient opens a project brain (and optionally the global brain).
func NewClient(projectDir string) (*Client, error)

// Close closes all brain connections.
func (c *Client) Close() error

// Search returns scored memory nodes matching the query.
func (c *Client) Search(ctx context.Context, query string, limit int, threshold float64) ([]store.ScoredNode, error)

// Add stores a new memory node.
func (c *Client) Add(content string, nodeType store.NodeType, opts ...brain.AddOption) (string, error)

// List returns nodes matching filters.
func (c *Client) List(opts store.ListNodesOpts) ([]store.Node, error)

// Get retrieves a single node with edges.
func (c *Client) Get(id string) (*store.NodeWithEdges, error)

// Stats returns brain health metrics.
func (c *Client) Stats() (*store.Stats, error)
```

### Tasks (TDD Order)

| # | Task | Test File | Test Functions | Impl File | DoD |
|---|------|-----------|----------------|-----------|-----|
| 1.1 | SafePath: reject traversal | `internal/safepath/safepath_test.go` | `TestNew_RejectsTraversal`, `TestNew_RejectsRelativePath`, `TestNew_RejectsEscapingSymlink` | `internal/safepath/safepath.go` | Paths with `..` or symlinks outside allowed bases return error |
| 1.2 | SafePath: accept valid paths | `internal/safepath/safepath_test.go` | `TestNew_AcceptsAbsoluteWithinBase`, `TestNew_ResolvesSymlinksWithinBase` | `internal/safepath/safepath.go` | Valid paths return SafePath with resolved absolute path |
| 1.3 | SafePath: String() returns resolved | `internal/safepath/safepath_test.go` | `TestSafePath_String` | `internal/safepath/safepath.go` | String() output matches filepath.EvalSymlinks result |
| 1.4 | CerebroClient: open project brain | `internal/cerebro/client_test.go` | `TestNewClient_OpensProjectBrain` | `internal/cerebro/client.go` | Opens brain in t.TempDir(), closes cleanly |
| 1.5 | CerebroClient: Add + Search round-trip | `internal/cerebro/client_test.go` | `TestClient_AddAndSearch` | `internal/cerebro/client.go` | Add a node, search for it, verify returned. Uses noop embedder (search will use List fallback or require test embedder). |
| 1.6 | CerebroClient: Add with options | `internal/cerebro/client_test.go` | `TestClient_Add_WithImportance`, `TestClient_Add_WithSubtype` | `internal/cerebro/client.go` | Functional options pass through to brain.Add |
| 1.7 | CerebroClient: List by type | `internal/cerebro/client_test.go` | `TestClient_List_ByType` | `internal/cerebro/client.go` | List with type filter returns correct subset |
| 1.8 | CerebroClient: Get with edges | `internal/cerebro/client_test.go` | `TestClient_Get_WithEdges` | `internal/cerebro/client.go` | Add two nodes + edge, Get returns node with edges |
| 1.9 | CerebroClient: Stats | `internal/cerebro/client_test.go` | `TestClient_Stats` | `internal/cerebro/client.go` | Stats returns non-zero counts after adding nodes |
| 1.10 | CerebroClient: handles missing DB | `internal/cerebro/client_test.go` | `TestNewClient_MissingDB` | `internal/cerebro/client.go` | Returns structured error, does not panic |
| 1.11 | CerebroClient: global store search | `internal/cerebro/client_test.go` | `TestClient_SearchWithGlobal` | `internal/cerebro/client.go` | When global brain is non-nil, SearchWithGlobal merges results |

### Security Findings Addressed

- **S9 (SafePath type):** Defined in this phase. Every subsequent phase uses SafePath for filesystem operations instead of raw strings.
- **S10 (timeouts):** CerebroClient methods accept `context.Context` for timeout propagation.

### Acceptance Criteria

- [ ] `go test ./internal/safepath/... -race` passes (6+ tests)
- [ ] `go test ./internal/cerebro/... -race` passes (8+ tests)
- [ ] SafePath rejects `../`, symlinks outside base, relative paths
- [ ] CerebroClient opens brain, CRUD works, closes cleanly
- [ ] No direct brain/ import outside `internal/cerebro/` (enforced by code review)

### Estimated New Files: 4

- `internal/safepath/safepath.go`
- `internal/safepath/safepath_test.go`
- `internal/cerebro/client.go`
- `internal/cerebro/client_test.go`

---

## Phase 2: Gemini Client

**Goal:** HTTP client that calls `GenerateContent` with tool declarations, parses `FunctionCall` responses, handles retries with exponential backoff, and enforces timeouts.

### Key Types

```go
// internal/gemini/client.go

// Client wraps the Gemini API for GenerateContent calls with tool support.
type Client struct {
    genaiClient *genai.Client
    model       string
    tools       []*genai.Tool
    maxRetries  int
    timeout     time.Duration
}

// NewClient creates a Gemini client. Reads GEMINI_API_KEY from env.
// Returns error if key is missing.
func NewClient(model string, opts ...Option) (*Client, error)

// Option configures the Gemini client.
type Option func(*Client)
func WithTimeout(d time.Duration) Option
func WithMaxRetries(n int) Option
func WithTools(tools []*genai.Tool) Option

// GenerateContentResult wraps the API response for Qraft's needs.
type GenerateContentResult struct {
    Text          string              // text response (empty if function call)
    FunctionCalls []FunctionCall       // function calls requested by model
    Usage         *UsageMetadata       // token counts
    FinishReason  string
}

// FunctionCall represents a single tool invocation requested by Gemini.
type FunctionCall struct {
    Name string
    Args map[string]any
}

// UsageMetadata tracks token usage for cost accounting.
type UsageMetadata struct {
    PromptTokens     int
    CandidateTokens  int
    TotalTokens      int
}

// Part represents a content part (text or function response).
type Part interface{ isPart() }
type TextPart struct { Text string }
type FunctionResponsePart struct {
    Name     string
    Response map[string]any
}

// Generate sends a prompt (with optional parts) to Gemini and returns the result.
// Handles retries on 429/500/503 with exponential backoff.
// Enforces context timeout.
func (c *Client) Generate(ctx context.Context, parts []Part) (*GenerateContentResult, error)
```

### Tasks (TDD Order)

| # | Task | Test File | Test Functions | Impl File | DoD |
|---|------|-----------|----------------|-----------|-----|
| 2.1 | NewClient rejects missing API key | `internal/gemini/client_test.go` | `TestNewClient_MissingAPIKey` | `internal/gemini/client.go` | Returns error when GEMINI_API_KEY not set |
| 2.2 | Generate: text response | `internal/gemini/client_test.go` | `TestClient_Generate_TextResponse` | `internal/gemini/client.go` | With recorded response, parses text correctly. Uses httptest.Server to mock Gemini API. |
| 2.3 | Generate: function call response | `internal/gemini/client_test.go` | `TestClient_Generate_FunctionCall` | `internal/gemini/client.go` | Parses FunctionCall name + args from recorded response |
| 2.4 | Generate: multiple function calls | `internal/gemini/client_test.go` | `TestClient_Generate_MultipleFunctionCalls` | `internal/gemini/client.go` | Handles responses with >1 function call in parts |
| 2.5 | Generate: extracts usage metadata | `internal/gemini/client_test.go` | `TestClient_Generate_UsageMetadata` | `internal/gemini/client.go` | Token counts populated from response |
| 2.6 | Generate: missing usage treated as max cost | `internal/gemini/client_test.go` | `TestClient_Generate_MissingUsage` | `internal/gemini/client.go` | S6: absent usageMetadata -> returns sentinel max-cost value |
| 2.7 | Generate: retry on 429 | `internal/gemini/client_test.go` | `TestClient_Generate_Retry429` | `internal/gemini/retry.go` | Mock returns 429 twice then 200. Verifies 3 total attempts with backoff. |
| 2.8 | Generate: retry on 500/503 | `internal/gemini/client_test.go` | `TestClient_Generate_RetryServerError` | `internal/gemini/retry.go` | Mock returns 500 once then 200. Verifies retry-once behavior. |
| 2.9 | Generate: no retry on 400 | `internal/gemini/client_test.go` | `TestClient_Generate_NoRetryClientError` | `internal/gemini/retry.go` | 400 returns immediately, no retry |
| 2.10 | Generate: context timeout | `internal/gemini/client_test.go` | `TestClient_Generate_Timeout` | `internal/gemini/client.go` | S10: Context cancelled mid-request returns context.DeadlineExceeded |
| 2.11 | Generate: reject unregistered tool names | `internal/gemini/client_test.go` | `TestClient_Generate_RejectsUnregisteredTool` | `internal/gemini/client.go` | Security rec: FunctionCall with name not in tools -> error |
| 2.12 | Tool declarations: build from Tool interface | `internal/gemini/tools_test.go` | `TestBuildToolDeclarations` | `internal/gemini/tools.go` | Converts Tool interface list to genai.Tool slice |
| 2.13 | Record/replay test fixtures | `internal/gemini/testdata/` | N/A (test infrastructure) | N/A | JSON fixtures for recorded Gemini responses |

### Security Findings Addressed

- **S10 (timeouts):** `context.WithTimeout` on every GenerateContent call. Default 30s.
- **S6 (cost control, partial):** Missing usage metadata treated as maximum possible cost.
- **Architectural rec #4:** Reject function calls naming unregistered tools.

### Acceptance Criteria

- [ ] `go test ./internal/gemini/... -race` passes (12+ tests)
- [ ] Retry logic verified with mock HTTP server
- [ ] Timeout enforcement verified
- [ ] Unregistered tool name rejection verified
- [ ] No API key logged anywhere in test output
- [ ] All tests use recorded responses or httptest mocks (no live API calls)

### Estimated New Files: 6

- `internal/gemini/client.go`
- `internal/gemini/client_test.go`
- `internal/gemini/retry.go`
- `internal/gemini/tools.go`
- `internal/gemini/tools_test.go`
- `internal/gemini/testdata/*.json` (3-5 fixture files)

---

## Phase 3: Single-Pass Loop + Tool Interface + Executor + Hydrator

**Goal:** `qraft "prompt"` works end-to-end: hydrate context from Cerebro, call Gemini, execute tools, loop until text response. Two tools: `memory_add`, `memory_search`. Confirmation gate with full hardening.

### Key Types

```go
// internal/tools/tool.go

// Tool is the interface every Qraft tool must implement.
type Tool interface {
    Name() string
    Description() string
    Parameters() map[string]any
    Execute(ctx context.Context, args json.RawMessage) (any, error)
    RequiresConfirmation() bool
    Permissions() ToolPermission
}

// ToolPermission declares what capabilities a tool needs.
type ToolPermission struct {
    Network      bool
    FileSystem   bool
    Hardware     bool
    MediaCapture bool
    Upload       bool
}

// Registry holds all registered tools, keyed by name.
type Registry struct {
    tools map[string]Tool
}

func NewRegistry() *Registry
func (r *Registry) Register(t Tool)
func (r *Registry) Get(name string) (Tool, bool)
func (r *Registry) All() []Tool
```

```go
// internal/executor/executor.go

// Executor runs tools with confirmation gates and permission checks.
type Executor struct {
    registry      *tools.Registry
    confirmFn     ConfirmFunc          // injectable for testing
    isTTY         bool
    maxErrors     int                  // per-interaction error cap (default: 3)
}

// ConfirmFunc asks the user to confirm an action. Defaults to stdin.
type ConfirmFunc func(toolName string, summary string) (bool, error)

// Execute runs a tool by name with the given args.
func (e *Executor) Execute(ctx context.Context, toolName string, args json.RawMessage) (any, error)
```

```go
// internal/hydrator/hydrator.go

// Hydrator assembles context for Gemini calls.
type Hydrator struct {
    cerebro          *cerebro.Client
    maxContextTokens int
}

// HydratedContext is the assembled context for a Gemini call.
type HydratedContext struct {
    UserPrompt    string
    Memories      []store.ScoredNode
    SensorState   map[string]any       // nil until Phase 5
    SystemPrompt  string
    TokenEstimate int
}

// Hydrate builds context for a user prompt.
func (h *Hydrator) Hydrate(ctx context.Context, prompt string) (*HydratedContext, error)

// FormatForGemini converts HydratedContext into genai-compatible parts.
// Memories are wrapped in <memories> delimiters with anti-injection preamble (S3).
func (hc *HydratedContext) FormatForGemini() []gemini.Part
```

```go
// internal/executor/confirm.go

// stripControlChars removes all ANSI escape sequences and non-printable
// characters from the input. Used to sanitize tool args before display (S1).
func stripControlChars(s string) string

// defaultConfirmFn is the production confirmation implementation.
// It checks TTY status (S1), strips ANSI from display (S1),
// and requires typing the tool name for Hardware/Upload actions (S1).
func defaultConfirmFn(toolName string, summary string) (bool, error)
```

### Tasks (TDD Order)

| # | Task | Test File | Test Functions | Impl File | DoD |
|---|------|-----------|----------------|-----------|-----|
| **Tool Interface + Registry** | | | | | |
| 3.1 | Registry: register and retrieve | `internal/tools/registry_test.go` | `TestRegistry_RegisterAndGet`, `TestRegistry_Get_NotFound` | `internal/tools/registry.go` | Register tool, get by name, get unknown returns false |
| 3.2 | Registry: All() returns all tools | `internal/tools/registry_test.go` | `TestRegistry_All` | `internal/tools/registry.go` | Returns all registered tools |
| 3.3 | Registry: duplicate name panics | `internal/tools/registry_test.go` | `TestRegistry_Register_DuplicatePanics` | `internal/tools/registry.go` | Registering same name twice panics (compile-time catch) |
| **Confirmation Gate (S1)** | | | | | |
| 3.4 | stripControlChars: removes ANSI | `internal/executor/confirm_test.go` | `TestStripControlChars_RemovesANSI`, `TestStripControlChars_RemovesNonPrintable`, `TestStripControlChars_PreservesNormalText` | `internal/executor/confirm.go` | ANSI escape sequences stripped, normal text preserved |
| 3.5 | Non-TTY: default-deny | `internal/executor/executor_test.go` | `TestExecutor_NonTTY_DeniesConfirmation` | `internal/executor/executor.go` | S1: When not a TTY, tools requiring confirmation return ErrNonInteractiveDenied |
| 3.6 | Executor: tool not found | `internal/executor/executor_test.go` | `TestExecutor_Execute_ToolNotFound` | `internal/executor/executor.go` | Returns ErrToolNotFound |
| 3.7 | Executor: no-confirm tool executes directly | `internal/executor/executor_test.go` | `TestExecutor_Execute_NoConfirmation` | `internal/executor/executor.go` | Tool with RequiresConfirmation()=false runs without prompt |
| 3.8 | Executor: confirm-required tool prompts | `internal/executor/executor_test.go` | `TestExecutor_Execute_RequiresConfirmation_Approved`, `TestExecutor_Execute_RequiresConfirmation_Denied` | `internal/executor/executor.go` | Injectable confirmFn called, result respected |
| 3.9 | Executor: panic recovery | `internal/executor/executor_test.go` | `TestExecutor_Execute_PanicRecovery` | `internal/executor/executor.go` | Tool that panics -> recovered, returns error |
| 3.10 | Executor: error count cap | `internal/executor/executor_test.go` | `TestExecutor_Execute_ErrorCountCap` | `internal/executor/executor.go` | After maxErrors (3), returns ErrTooManyErrors |
| **Memory Tools** | | | | | |
| 3.11 | MemoryAddTool: implements Tool | `internal/tools/memory_test.go` | `TestMemoryAddTool_Name`, `TestMemoryAddTool_Execute` | `internal/tools/memory.go` | Adds node to cerebro, returns ID |
| 3.12 | MemorySearchTool: implements Tool | `internal/tools/memory_test.go` | `TestMemorySearchTool_Name`, `TestMemorySearchTool_Execute` | `internal/tools/memory.go` | Searches cerebro, returns results |
| 3.13 | Memory tools: bad args | `internal/tools/memory_test.go` | `TestMemoryAddTool_Execute_BadArgs`, `TestMemorySearchTool_Execute_BadArgs` | `internal/tools/memory.go` | Invalid JSON args return structured error |
| **Hydrator** | | | | | |
| 3.14 | Hydrator: basic hydration | `internal/hydrator/hydrator_test.go` | `TestHydrator_Hydrate_BasicPrompt` | `internal/hydrator/hydrator.go` | Returns HydratedContext with prompt and memories |
| 3.15 | Hydrator: token budget truncation | `internal/hydrator/hydrator_test.go` | `TestHydrator_Hydrate_TruncatesAtBudget` | `internal/hydrator/hydrator.go` | When memories exceed budget, lowest-scored truncated |
| 3.16 | Hydrator: empty search results | `internal/hydrator/hydrator_test.go` | `TestHydrator_Hydrate_NoMemories` | `internal/hydrator/hydrator.go` | Zero results is normal, proceeds with empty memories |
| 3.17 | FormatForGemini: memory delimiters | `internal/hydrator/format_test.go` | `TestHydratedContext_FormatForGemini_MemoryDelimiters` | `internal/hydrator/format.go` | S3: Memories wrapped in `<memories>` block with anti-injection preamble |
| 3.18 | FormatForGemini: sanitizes memory content | `internal/hydrator/format_test.go` | `TestHydratedContext_FormatForGemini_SanitizesContent` | `internal/hydrator/format.go` | S3: Content length capped, instruction-like patterns flagged |
| **Tool Loop** | | | | | |
| 3.19 | Tool loop: text response terminates | `internal/gemini/loop_test.go` | `TestToolLoop_TextResponse` | `internal/gemini/loop.go` | Single Generate call returns text -> loop exits |
| 3.20 | Tool loop: function call + response + text | `internal/gemini/loop_test.go` | `TestToolLoop_FunctionCallThenText` | `internal/gemini/loop.go` | Generate returns FunctionCall -> execute -> send FunctionResponse -> Generate returns text |
| 3.21 | Tool loop: max iterations cap | `internal/gemini/loop_test.go` | `TestToolLoop_MaxIterations` | `internal/gemini/loop.go` | After 10 iterations without text, returns ErrMaxIterations |
| 3.22 | Tool loop: tool error sent as function response | `internal/gemini/loop_test.go` | `TestToolLoop_ToolError` | `internal/gemini/loop.go` | Tool execution error is formatted as FunctionResponse error and sent back to Gemini |
| **CLI Wiring** | | | | | |
| 3.23 | Cobra root command | `cmd/qraft/main.go`, `cmd/qraft/root.go` | N/A (integration) | `cmd/qraft/root.go` | `qraft "prompt"` parses the positional arg |
| 3.24 | End-to-end: prompt -> response | `cmd/qraft/root_test.go` | `TestRootCommand_EndToEnd` | `cmd/qraft/root.go` | Integration test: mock Gemini + real cerebro in TempDir. Prompt in, text out. |

### Security Findings Addressed

- **S1 (confirmation gate, CRITICAL):** Tasks 3.4-3.5. ANSI stripping, TTY detection, default-deny in non-interactive mode.
- **S3 (indirect prompt injection, HIGH):** Tasks 3.17-3.18. Memory content wrapped in delimited blocks with anti-injection preamble. Content length capped.

### Acceptance Criteria

- [ ] `go test ./internal/tools/... -race` passes (8+ tests)
- [ ] `go test ./internal/executor/... -race` passes (7+ tests)
- [ ] `go test ./internal/hydrator/... -race` passes (5+ tests)
- [ ] `go test ./internal/gemini/... -race` passes (including loop tests)
- [ ] `qraft "hello"` with mocked Gemini produces a text response
- [ ] Non-TTY execution denies confirmation-required tools
- [ ] ANSI injection in tool args is stripped before display
- [ ] Tool loop terminates after max iterations

### Estimated New Files: 14

- `internal/tools/tool.go` (interface, already scaffolded)
- `internal/tools/registry.go`
- `internal/tools/registry_test.go`
- `internal/tools/memory.go`
- `internal/tools/memory_test.go`
- `internal/executor/executor.go`
- `internal/executor/executor_test.go`
- `internal/executor/confirm.go`
- `internal/executor/confirm_test.go`
- `internal/hydrator/hydrator.go`
- `internal/hydrator/hydrator_test.go`
- `internal/hydrator/format.go`
- `internal/hydrator/format_test.go`
- `internal/gemini/loop.go`
- `internal/gemini/loop_test.go`
- `cmd/qraft/root.go`
- `cmd/qraft/root_test.go`

---

## Phase 4: Observability -- Logging + Cost Tracking

**Goal:** Structured JSON logging at every boundary. Cost tracking with pre-call budget gate and file-locked daily counter.

### Key Types

```go
// internal/logging/logging.go

// Logger wraps slog with Qraft-specific functionality:
// file creation with 0600 permissions (S4) and secret scrubbing (S4).
type Logger struct {
    slog     *slog.Logger
    file     *os.File
    scrubber *SecretScrubber
}

// NewLogger creates a JSON logger writing to the given path.
// Creates the file with 0600 permissions (S4).
func NewLogger(path string, level slog.Level) (*Logger, error)

// SecretScrubber redacts sensitive fields from log records.
// Fields: api_key, token, secret, authorization, credential, password (S4).
type SecretScrubber struct {
    patterns []string
}

// InteractionLog is the structured record of one Qraft interaction.
type InteractionLog struct {
    Timestamp     time.Time       `json:"ts"`
    RequestID     string          `json:"request_id"`
    UserPrompt    string          `json:"user_prompt"`
    MemoriesUsed  int             `json:"memories_used"`
    SensorsPolled map[string]bool `json:"sensors_polled"`
    TokensSent    int             `json:"tokens_sent"`
    TokensRecvd   int            `json:"tokens_received"`
    ToolCalls     []ToolCallLog   `json:"tool_calls"`
    GeminiLatency time.Duration   `json:"gemini_latency_ms"`
    TotalLatency  time.Duration   `json:"total_latency_ms"`
    CostUSD       float64         `json:"cost_usd"`
    Error         string          `json:"error,omitempty"`
}

// ToolCallLog records a single tool invocation.
type ToolCallLog struct {
    Name     string        `json:"name"`
    Summary  string        `json:"summary"`  // sanitized, NOT raw args (S4)
    Duration time.Duration `json:"duration_ms"`
    Error    string        `json:"error,omitempty"`
}
```

```go
// internal/cost/tracker.go

// Tracker tracks Gemini API spend with a file-locked daily counter (S6).
type Tracker struct {
    dailyBudget   float64
    warnThreshold float64
    counterPath   string         // path to daily counter file
    mu            sync.Mutex
}

// NewTracker creates a cost tracker.
func NewTracker(budget, warn float64, counterPath string) *Tracker

// PreCallGate checks if the estimated cost would exceed the daily budget.
// Returns ErrBudgetExhausted if so (S6).
func (t *Tracker) PreCallGate(estimatedTokens int) error

// RecordUsage adds actual token usage to the daily counter.
// Uses file locking for cross-process safety (S6).
func (t *Tracker) RecordUsage(usage *gemini.UsageMetadata) error

// TodaySpend returns the current day's accumulated spend.
func (t *Tracker) TodaySpend() (float64, error)
```

### Tasks (TDD Order)

| # | Task | Test File | Test Functions | Impl File | DoD |
|---|------|-----------|----------------|-----------|-----|
| **Logging** | | | | | |
| 4.1 | Logger: creates file with 0600 | `internal/logging/logging_test.go` | `TestNewLogger_FilePermissions` | `internal/logging/logging.go` | S4: File created with mode 0600 |
| 4.2 | SecretScrubber: redacts sensitive fields | `internal/logging/scrubber_test.go` | `TestSecretScrubber_RedactsAPIKey`, `TestSecretScrubber_RedactsToken`, `TestSecretScrubber_PreservesNormalFields` | `internal/logging/scrubber.go` | S4: Fields containing "api_key", "token", "secret", "authorization", "credential", "password" are redacted to "[REDACTED]" |
| 4.3 | Logger: writes structured JSON | `internal/logging/logging_test.go` | `TestLogger_WritesJSON` | `internal/logging/logging.go` | Output is valid JSON lines |
| 4.4 | Logger: scrubs secrets in output | `internal/logging/logging_test.go` | `TestLogger_ScrubsSecrets` | `internal/logging/logging.go` | S4: Log output does not contain API keys even if passed as attrs |
| 4.5 | InteractionLog: serialization | `internal/logging/interaction_test.go` | `TestInteractionLog_MarshalJSON` | `internal/logging/interaction.go` | Round-trip JSON serialization preserves all fields |
| 4.6 | ToolCallLog: uses summary not raw args | `internal/logging/interaction_test.go` | `TestToolCallLog_NoRawArgs` | `internal/logging/interaction.go` | S4: No json.RawMessage field, only sanitized summary string |
| **Cost Tracking** | | | | | |
| 4.7 | Tracker: PreCallGate allows under budget | `internal/cost/tracker_test.go` | `TestTracker_PreCallGate_UnderBudget` | `internal/cost/tracker.go` | Returns nil when spend < budget |
| 4.8 | Tracker: PreCallGate blocks over budget | `internal/cost/tracker_test.go` | `TestTracker_PreCallGate_OverBudget` | `internal/cost/tracker.go` | S6: Returns ErrBudgetExhausted when estimated cost would exceed budget |
| 4.9 | Tracker: RecordUsage accumulates | `internal/cost/tracker_test.go` | `TestTracker_RecordUsage_Accumulates` | `internal/cost/tracker.go` | Multiple calls accumulate spend |
| 4.10 | Tracker: file-locked counter | `internal/cost/tracker_test.go` | `TestTracker_FileLock_ConcurrentSafety` | `internal/cost/tracker.go` | S6: Two goroutines writing concurrently produce correct total |
| 4.11 | Tracker: daily reset at midnight UTC | `internal/cost/tracker_test.go` | `TestTracker_DailyReset` | `internal/cost/tracker.go` | Counter file with yesterday's date is ignored |
| 4.12 | Tracker: absent usage metadata = max cost | `internal/cost/tracker_test.go` | `TestTracker_RecordUsage_NilUsage` | `internal/cost/tracker.go` | S6: nil UsageMetadata records maximum possible cost |
| 4.13 | Tracker: warn threshold logging | `internal/cost/tracker_test.go` | `TestTracker_WarnThreshold` | `internal/cost/tracker.go` | When spend > warnThreshold, logs warning |
| **Integration** | | | | | |
| 4.14 | Wire logging + cost into tool loop | N/A (wiring) | N/A | `internal/gemini/loop.go` (update) | Tool loop calls PreCallGate before Generate, records usage after, logs InteractionLog |

### Security Findings Addressed

- **S4 (log security, HIGH):** Tasks 4.1-4.6. File permissions 0600, secret scrubber, no raw args in logs.
- **S6 (cost control, HIGH):** Tasks 4.7-4.13. Pre-call budget gate, file-locked counter, absent metadata = max cost.

### Acceptance Criteria

- [ ] `go test ./internal/logging/... -race` passes (6+ tests)
- [ ] `go test ./internal/cost/... -race` passes (7+ tests)
- [ ] Log file created with 0600 permissions
- [ ] Secret values never appear in log output
- [ ] Budget gate prevents calls when daily limit exceeded
- [ ] Concurrent cost tracking produces correct totals
- [ ] Tool loop logs structured InteractionLog for every interaction

### Estimated New Files: 8

- `internal/logging/logging.go`
- `internal/logging/logging_test.go`
- `internal/logging/scrubber.go`
- `internal/logging/scrubber_test.go`
- `internal/logging/interaction.go`
- `internal/logging/interaction_test.go`
- `internal/cost/tracker.go`
- `internal/cost/tracker_test.go`

---

## Phase 5: Sensor Integration (MQTT + HTTP Polling)

**Goal:** Hydrator fetches live sensor state from MQTT broker and Moonraker HTTP API. MQTTS by default with authentication.

### Key Types

```go
// internal/sensors/sensors.go

// SensorProvider fetches current state from a sensor source.
type SensorProvider interface {
    // Name returns the sensor name (for logging and hydration context).
    Name() string

    // Poll fetches current state with the given timeout.
    // Returns nil, nil if the sensor is unreachable (graceful degradation).
    Poll(ctx context.Context) (map[string]any, error)

    // Close releases resources.
    Close() error
}

// Poller aggregates multiple sensor providers and polls them with timeout.
type Poller struct {
    providers []SensorProvider
    timeout   time.Duration
}

// PollAll queries all sensors within the timeout.
// Returns available data; unavailable sensors are logged and skipped.
func (p *Poller) PollAll(ctx context.Context) map[string]any
```

```go
// internal/sensors/mqtt.go

// MQTTSensor subscribes to an MQTT topic and caches the last message.
type MQTTSensor struct {
    name      string
    broker    string           // must be "ssl://" or "tls://" unless insecure allowed (S5)
    topic     string
    client    mqtt.Client
    lastValue atomic.Value     // cached last message
    schema    *ValueSchema     // S3: strict type validation
}

// MQTTConfig configures an MQTT sensor.
type MQTTConfig struct {
    Name           string
    BrokerURL      string   // S5: must be mqtts:// unless AllowInsecure
    Topic          string
    CACert         string   // S5: CA cert path
    ClientCert     string   // S5: client cert path
    ClientKey      string   // S5: client key path
    Username       string
    Password       string
    AllowInsecure  bool     // S5: requires explicit opt-in for plaintext
}

// ValueSchema defines expected types and ranges for MQTT values (S3).
type ValueSchema struct {
    Fields map[string]FieldSpec
}
type FieldSpec struct {
    Type    string   // "float64", "int", "string", "enum"
    Min     *float64 // numeric range
    Max     *float64
    Allowed []string // enum values
}
```

```go
// internal/sensors/moonraker.go

// MoonrakerSensor polls the Moonraker HTTP API for printer status.
type MoonrakerSensor struct {
    name    string
    baseURL string
    client  *http.Client // S10: with explicit timeout
}

// PrinterState is the validated, typed subset of Moonraker data (S3).
// Only typed fields are injected into the hydrator, never raw JSON.
type PrinterState struct {
    ExtruderTempC float64          `json:"extruder_temp_c"`
    BedTempC      float64          `json:"bed_temp_c"`
    PrintProgress float64          `json:"print_progress"`
    State         PrinterStateEnum `json:"state"`
    Filename      string           `json:"filename"`
}

type PrinterStateEnum string
const (
    PrinterIdle     PrinterStateEnum = "idle"
    PrinterPrinting PrinterStateEnum = "printing"
    PrinterPaused   PrinterStateEnum = "paused"
    PrinterError    PrinterStateEnum = "error"
)
```

### Tasks (TDD Order)

| # | Task | Test File | Test Functions | Impl File | DoD |
|---|------|-----------|----------------|-----------|-----|
| **MQTT (S5)** | | | | | |
| 5.1 | MQTTConfig: reject plaintext without opt-in | `internal/sensors/mqtt_test.go` | `TestMQTTConfig_RejectsPlaintext` | `internal/sensors/mqtt.go` | S5: `tcp://` URL without AllowInsecure=true returns error |
| 5.2 | MQTTConfig: accept MQTTS | `internal/sensors/mqtt_test.go` | `TestMQTTConfig_AcceptsMQTTS` | `internal/sensors/mqtt.go` | S5: `ssl://` or `tls://` URL accepted |
| 5.3 | MQTTSensor: subscribe and cache | `internal/sensors/mqtt_test.go` | `TestMQTTSensor_SubscribeAndCache` | `internal/sensors/mqtt.go` | Uses mock MQTT broker. Publish message, Poll returns it. |
| 5.4 | MQTTSensor: Poll returns nil on no data | `internal/sensors/mqtt_test.go` | `TestMQTTSensor_Poll_NoData` | `internal/sensors/mqtt.go` | Before any message, Poll returns nil (graceful) |
| 5.5 | MQTTSensor: value schema validation | `internal/sensors/mqtt_test.go` | `TestMQTTSensor_SchemaValidation` | `internal/sensors/mqtt.go` | S3: Values outside schema ranges are rejected |
| 5.6 | MQTTSensor: rejects string injection in numeric field | `internal/sensors/mqtt_test.go` | `TestMQTTSensor_RejectsInjection` | `internal/sensors/mqtt.go` | S3: String value in float64 field returns error |
| **Moonraker** | | | | | |
| 5.7 | MoonrakerSensor: parse status | `internal/sensors/moonraker_test.go` | `TestMoonrakerSensor_ParseStatus` | `internal/sensors/moonraker.go` | httptest.Server returns JSON, parses into PrinterState |
| 5.8 | MoonrakerSensor: HTTP timeout | `internal/sensors/moonraker_test.go` | `TestMoonrakerSensor_Timeout` | `internal/sensors/moonraker.go` | S10: Slow server -> context.DeadlineExceeded |
| 5.9 | MoonrakerSensor: validate ranges | `internal/sensors/moonraker_test.go` | `TestMoonrakerSensor_ValidateRanges` | `internal/sensors/moonraker.go` | S3: ExtruderTempC > 300 rejected. PrintProgress > 1.0 rejected. |
| 5.10 | MoonrakerSensor: unreachable is graceful | `internal/sensors/moonraker_test.go` | `TestMoonrakerSensor_Unreachable` | `internal/sensors/moonraker.go` | Connection refused -> returns nil, nil (logged, not fatal) |
| 5.11 | PrinterState: enum validation | `internal/sensors/moonraker_test.go` | `TestPrinterState_InvalidEnum` | `internal/sensors/moonraker.go` | S3: Unknown state string -> error |
| **Poller** | | | | | |
| 5.12 | Poller: aggregates multiple sensors | `internal/sensors/poller_test.go` | `TestPoller_PollAll_MultipleSensors` | `internal/sensors/poller.go` | Two mock providers, both polled, results merged |
| 5.13 | Poller: timeout skips slow sensor | `internal/sensors/poller_test.go` | `TestPoller_PollAll_TimeoutSkipsSlow` | `internal/sensors/poller.go` | One fast, one slow provider. Slow skipped, fast returned. |
| 5.14 | Poller: all sensors down | `internal/sensors/poller_test.go` | `TestPoller_PollAll_AllDown` | `internal/sensors/poller.go` | Returns empty map, logs warnings, does not error |
| **Hydrator Integration** | | | | | |
| 5.15 | Hydrator: includes sensor state | `internal/hydrator/hydrator_test.go` (update) | `TestHydrator_Hydrate_WithSensors` | `internal/hydrator/hydrator.go` (update) | HydratedContext.SensorState populated from poller |

### Security Findings Addressed

- **S5 (MQTT auth, HIGH):** Tasks 5.1-5.2. MQTTS required by default, plaintext requires explicit opt-in.
- **S3 (indirect injection, HIGH):** Tasks 5.5-5.6, 5.9, 5.11. Strict schema validation on all sensor data. Only typed values injected.
- **S10 (timeouts):** Tasks 5.8, 5.13. Explicit timeouts on HTTP calls and sensor polling.

### Acceptance Criteria

- [ ] `go test ./internal/sensors/... -race` passes (14+ tests)
- [ ] MQTT plaintext rejected without explicit opt-in
- [ ] Sensor values validated against schema (type, range, enum)
- [ ] Unreachable sensors are gracefully skipped
- [ ] Hydrator includes sensor state in context
- [ ] All HTTP calls have explicit timeout

### Estimated New Files: 8

- `internal/sensors/sensors.go` (interface)
- `internal/sensors/mqtt.go`
- `internal/sensors/mqtt_test.go`
- `internal/sensors/moonraker.go`
- `internal/sensors/moonraker_test.go`
- `internal/sensors/poller.go`
- `internal/sensors/poller_test.go`
- `internal/sensors/testdata/moonraker_status.json`

---

## Phase 6: Media Tools (FFmpegBuilder + capture_media + process_video)

**Goal:** ffmpeg-based media capture and processing behind SafePath validation and confirmation gates. FFmpegBuilder prevents command injection.

### Key Types

```go
// internal/tools/ffmpeg.go

// FFmpegBuilder constructs validated ffmpeg commands.
// All paths must be SafePath. All numeric params are typed and clamped.
// Never uses sh -c. Always exec.CommandContext with separate args (S2).
type FFmpegBuilder struct {
    binary     string        // validated at startup
    allowedIn  []SafePath    // allowed input paths (from config)
    allowedOut []SafePath    // allowed output dirs (from config)
}

// NewFFmpegBuilder validates the ffmpeg binary exists and returns a builder.
func NewFFmpegBuilder(binaryPath string, allowedIn, allowedOut []SafePath) (*FFmpegBuilder, error)

// CaptureFrame builds an ffmpeg command to capture a single frame.
func (b *FFmpegBuilder) CaptureFrame(ctx context.Context, device SafePath, output SafePath) *exec.Cmd

// Transcode builds an ffmpeg command to process a video file.
func (b *FFmpegBuilder) Transcode(ctx context.Context, input SafePath, output SafePath, opts TranscodeOpts) *exec.Cmd

// TranscodeOpts configures video transcoding. All values are clamped.
type TranscodeOpts struct {
    Codec      string        // validated against allowlist
    Resolution string        // validated WxH format
    FPS        int           // clamped to 1-60
    Duration   time.Duration // clamped to max 24h
}
```

```go
// internal/tools/capture.go

// CaptureMediaTool captures a frame or short video using ffmpeg.
type CaptureMediaTool struct {
    builder  *FFmpegBuilder
    workDir  SafePath         // validated output directory
}

// RequiresConfirmation returns true (hardware + media capture).
func (t *CaptureMediaTool) RequiresConfirmation() bool { return true }
```

```go
// internal/tools/video.go

// ProcessVideoTool runs ffmpeg transcoding on an existing file.
type ProcessVideoTool struct {
    builder *FFmpegBuilder
    workDir SafePath
}

// RequiresConfirmation returns false (no hardware, no upload).
func (t *ProcessVideoTool) RequiresConfirmation() bool { return false }
```

### Tasks (TDD Order)

| # | Task | Test File | Test Functions | Impl File | DoD |
|---|------|-----------|----------------|-----------|-----|
| **FFmpegBuilder (S2)** | | | | | |
| 6.1 | NewFFmpegBuilder: validates binary exists | `internal/tools/ffmpeg_test.go` | `TestNewFFmpegBuilder_ValidBinary`, `TestNewFFmpegBuilder_MissingBinary` | `internal/tools/ffmpeg.go` | Returns error if binary not found |
| 6.2 | CaptureFrame: correct arg slice | `internal/tools/ffmpeg_test.go` | `TestFFmpegBuilder_CaptureFrame_ArgSlice` | `internal/tools/ffmpeg.go` | S2: Verify exec.Cmd.Args has separate elements, no shell interpolation |
| 6.3 | CaptureFrame: only SafePath accepted | `internal/tools/ffmpeg_test.go` | `TestFFmpegBuilder_CaptureFrame_RequiresSafePath` | `internal/tools/ffmpeg.go` | S2: Compile-time enforcement -- method signature requires SafePath |
| 6.4 | Transcode: clamps FPS | `internal/tools/ffmpeg_test.go` | `TestFFmpegBuilder_Transcode_ClampsFPS` | `internal/tools/ffmpeg.go` | S2: FPS > 60 clamped to 60, FPS < 1 clamped to 1 |
| 6.5 | Transcode: validates codec allowlist | `internal/tools/ffmpeg_test.go` | `TestFFmpegBuilder_Transcode_RejectsUnknownCodec` | `internal/tools/ffmpeg.go` | S2: Codec not in allowlist returns error |
| 6.6 | Transcode: clamps duration | `internal/tools/ffmpeg_test.go` | `TestFFmpegBuilder_Transcode_ClampsDuration` | `internal/tools/ffmpeg.go` | Duration > 24h clamped to 24h |
| 6.7 | No sh -c anywhere | `internal/tools/ffmpeg_test.go` | `TestFFmpegBuilder_NoShellInvocation` | `internal/tools/ffmpeg.go` | S2: grep all Cmd constructions, verify no "sh", "-c" in args |
| **CaptureMediaTool** | | | | | |
| 6.8 | CaptureMediaTool: requires confirmation | `internal/tools/capture_test.go` | `TestCaptureMediaTool_RequiresConfirmation` | `internal/tools/capture.go` | Returns true |
| 6.9 | CaptureMediaTool: execute with mock | `internal/tools/capture_test.go` | `TestCaptureMediaTool_Execute` | `internal/tools/capture.go` | Uses testdata synthetic input. Verify output file created in workDir. |
| 6.10 | CaptureMediaTool: device from config only | `internal/tools/capture_test.go` | `TestCaptureMediaTool_DeviceFromConfig` | `internal/tools/capture.go` | S2: Device path comes from config, not from args JSON |
| 6.11 | CaptureMediaTool: args parsing | `internal/tools/capture_test.go` | `TestCaptureMediaTool_Execute_BadArgs` | `internal/tools/capture.go` | Invalid args return structured error |
| **ProcessVideoTool** | | | | | |
| 6.12 | ProcessVideoTool: does not require confirm | `internal/tools/video_test.go` | `TestProcessVideoTool_RequiresConfirmation` | `internal/tools/video.go` | Returns false |
| 6.13 | ProcessVideoTool: execute | `internal/tools/video_test.go` | `TestProcessVideoTool_Execute` | `internal/tools/video.go` | Transcodes test video file using ffmpeg -f lavfi synthetic source |
| 6.14 | ProcessVideoTool: input must be in workDir | `internal/tools/video_test.go` | `TestProcessVideoTool_Execute_RejectsPathOutsideWorkDir` | `internal/tools/video.go` | Input path not within allowed dirs returns error |
| **Integration** | | | | | |
| 6.15 | ffmpeg integration test | `internal/tools/ffmpeg_integration_test.go` | `TestFFmpeg_Integration_CaptureWithTestSrc` | N/A | Uses `ffmpeg -f lavfi -i testsrc=duration=1` -- skipped if ffmpeg not installed |

### Security Findings Addressed

- **S2 (command injection, CRITICAL):** Tasks 6.1-6.7. FFmpegBuilder enforces separate arg slices, SafePath for all paths, clamped numeric params, codec allowlist. No shell invocation.
- **S9 (SafePath):** Pervasive -- all file paths through SafePath type from Phase 1.

### Acceptance Criteria

- [ ] `go test ./internal/tools/... -race` passes (15+ tests including media)
- [ ] FFmpegBuilder never constructs `sh -c` commands
- [ ] All paths to ffmpeg are SafePath validated
- [ ] Numeric parameters are clamped to safe ranges
- [ ] CaptureMediaTool requires confirmation
- [ ] Integration test passes with synthetic ffmpeg source

### Estimated New Files: 8

- `internal/tools/ffmpeg.go`
- `internal/tools/ffmpeg_test.go`
- `internal/tools/capture.go`
- `internal/tools/capture_test.go`
- `internal/tools/video.go`
- `internal/tools/video_test.go`
- `internal/tools/ffmpeg_integration_test.go`
- `internal/tools/testdata/` (test fixtures)

---

## Phase 7: Upload Tools

**Goal:** TikTok/YouTube upload integration with path validation, MIME checking, rate limiting, and confirmation gates.

### Key Types

```go
// internal/tools/upload.go

// UploadTool uploads media to external platforms.
type UploadTool struct {
    mediaDir   SafePath              // S7: only files within this dir
    maxPerHour int                   // S7: rate limit (default: 1)
    platforms  map[string]Uploader
}

// Uploader is the interface for platform-specific upload logic.
type Uploader interface {
    Upload(ctx context.Context, file SafePath, metadata UploadMetadata) (*UploadResult, error)
    Platform() string
}

// UploadMetadata configures the upload.
type UploadMetadata struct {
    Title       string
    Description string
    Tags        []string
    Privacy     string // "private", "unlisted", "public"
}

// UploadResult is returned after a successful upload.
type UploadResult struct {
    Platform string `json:"platform"`
    URL      string `json:"url"`
    ID       string `json:"id"`
}

// RequiresConfirmation returns true (upload = data exfiltration).
func (t *UploadTool) RequiresConfirmation() bool { return true }
```

```go
// internal/tools/mime.go

// ValidateMediaFile checks that a file is a valid media file for upload (S7).
// Reads file header bytes, validates MIME type against allowlist.
// Rejects based on actual content, not file extension.
func ValidateMediaFile(path SafePath) (mimeType string, err error)
```

### Tasks (TDD Order)

| # | Task | Test File | Test Functions | Impl File | DoD |
|---|------|-----------|----------------|-----------|-----|
| **MIME Validation (S7)** | | | | | |
| 7.1 | ValidateMediaFile: accepts video/mp4 | `internal/tools/mime_test.go` | `TestValidateMediaFile_AcceptsMP4` | `internal/tools/mime.go` | Real MP4 header bytes pass validation |
| 7.2 | ValidateMediaFile: accepts image/jpeg | `internal/tools/mime_test.go` | `TestValidateMediaFile_AcceptsJPEG` | `internal/tools/mime.go` | JPEG magic bytes pass |
| 7.3 | ValidateMediaFile: rejects text file | `internal/tools/mime_test.go` | `TestValidateMediaFile_RejectsText` | `internal/tools/mime.go` | S7: text/plain file returns error |
| 7.4 | ValidateMediaFile: rejects by content not extension | `internal/tools/mime_test.go` | `TestValidateMediaFile_RejectsRenamedText` | `internal/tools/mime.go` | S7: .mp4 extension but text content -> rejected |
| **Path Validation (S7)** | | | | | |
| 7.5 | UploadTool: rejects path outside mediaDir | `internal/tools/upload_test.go` | `TestUploadTool_RejectsPathOutsideMediaDir` | `internal/tools/upload.go` | S7: filepath.EvalSymlinks -> not in mediaDir -> error |
| 7.6 | UploadTool: rejects symlink escape | `internal/tools/upload_test.go` | `TestUploadTool_RejectsSymlinkEscape` | `internal/tools/upload.go` | S7: Symlink pointing outside mediaDir -> error |
| **Rate Limiting (S7)** | | | | | |
| 7.7 | UploadTool: rate limit per interaction | `internal/tools/upload_test.go` | `TestUploadTool_RateLimitPerInteraction` | `internal/tools/upload.go` | S7: Second upload in same interaction returns error |
| **Upload Execution** | | | | | |
| 7.8 | UploadTool: requires confirmation | `internal/tools/upload_test.go` | `TestUploadTool_RequiresConfirmation` | `internal/tools/upload.go` | Returns true |
| 7.9 | UploadTool: execute with mock uploader | `internal/tools/upload_test.go` | `TestUploadTool_Execute_MockUploader` | `internal/tools/upload.go` | Mock Uploader receives correct path and metadata |
| 7.10 | UploadTool: platform not found | `internal/tools/upload_test.go` | `TestUploadTool_Execute_UnknownPlatform` | `internal/tools/upload.go` | Returns error for unregistered platform |
| 7.11 | UploadTool: timeout on upload | `internal/tools/upload_test.go` | `TestUploadTool_Execute_Timeout` | `internal/tools/upload.go` | S10: Context timeout -> error |
| **Platform Implementations** | | | | | |
| 7.12 | YouTubeUploader: request format | `internal/tools/youtube_test.go` | `TestYouTubeUploader_RequestFormat` | `internal/tools/youtube.go` | httptest.Server verifies correct OAuth + multipart format |
| 7.13 | TikTokUploader: request format | `internal/tools/tiktok_test.go` | `TestTikTokUploader_RequestFormat` | `internal/tools/tiktok.go` | httptest.Server verifies correct API format |

### Security Findings Addressed

- **S7 (upload validation, HIGH):** Tasks 7.1-7.7. Path validated within mediaDir after symlink resolution. MIME checked by header bytes. Max 1 upload per interaction.
- **S10 (timeouts):** Task 7.11. Upload calls have explicit context timeout.

### Acceptance Criteria

- [ ] `go test ./internal/tools/... -race` passes (including upload tests)
- [ ] Upload path must resolve within configured media directory
- [ ] MIME validation reads file header, not extension
- [ ] Rate limited to 1 upload per interaction
- [ ] Both platform uploaders use httptest mocks
- [ ] Upload tool requires confirmation

### Estimated New Files: 8

- `internal/tools/mime.go`
- `internal/tools/mime_test.go`
- `internal/tools/upload.go`
- `internal/tools/upload_test.go`
- `internal/tools/youtube.go`
- `internal/tools/youtube_test.go`
- `internal/tools/tiktok.go`
- `internal/tools/tiktok_test.go`

---

## Phase 8: Configuration

**Goal:** TOML config file loading, validation, and `qraft init` scaffolding command.

### Key Types

```go
// internal/config/config.go

// Config is the top-level configuration for Qraft.
// Loaded from ~/.qraftworx/config.toml.
type Config struct {
    Gemini  GeminiConfig            `toml:"gemini"`
    Cerebro CerebroConfig           `toml:"cerebro"`
    Sensors map[string]SensorConfig `toml:"sensors"`
    Media   MediaConfig             `toml:"media"`
    Logging LoggingConfig           `toml:"logging"`
    Cost    CostConfig              `toml:"cost"`
}

type GeminiConfig struct {
    Model             string        `toml:"model"`
    MaxTokens         int           `toml:"max_tokens"`
    MaxToolIterations int           `toml:"max_tool_iterations"`
    Timeout           Duration      `toml:"timeout"` // custom TOML duration
}

type CerebroConfig struct {
    ProjectDir string `toml:"project_dir"`
}

type SensorConfig struct {
    Type        string `toml:"type"`           // "moonraker", "mqtt"
    URL         string `toml:"url,omitempty"`
    BrokerURL   string `toml:"broker_url,omitempty"`
    Topic       string `toml:"topic,omitempty"`
    CACert      string `toml:"ca_cert,omitempty"`
    ClientCert  string `toml:"client_cert,omitempty"`
    ClientKey   string `toml:"client_key,omitempty"`
    AllowInsecure bool `toml:"allow_insecure_mqtt,omitempty"`
    PollTimeout Duration `toml:"poll_timeout"`
}

type MediaConfig struct {
    Webcam  DeviceConfig `toml:"webcam"`
    GoPro   DeviceConfig `toml:"gopro"`
}

type DeviceConfig struct {
    Type       string `toml:"type,omitempty"` // "v4l2", "http"
    Device     string `toml:"device,omitempty"`
    URL        string `toml:"url,omitempty"`
    Resolution string `toml:"resolution,omitempty"`
}

type LoggingConfig struct {
    Path  string `toml:"path"`
    Level string `toml:"level"` // "debug", "info", "warn", "error"
}

type CostConfig struct {
    DailyBudgetUSD   float64 `toml:"daily_budget_usd"`
    WarnThresholdUSD float64 `toml:"warn_threshold_usd"`
}

// Duration is a TOML-friendly wrapper around time.Duration.
type Duration struct {
    time.Duration
}
func (d *Duration) UnmarshalText(text []byte) error

// Load reads and validates config from the given path.
func Load(path string) (*Config, error)

// Validate checks all config values for correctness.
// Returns a multi-error with all validation failures.
func (c *Config) Validate() error
```

### Tasks (TDD Order)

| # | Task | Test File | Test Functions | Impl File | DoD |
|---|------|-----------|----------------|-----------|-----|
| **Config Loading** | | | | | |
| 8.1 | Load: valid config | `internal/config/config_test.go` | `TestLoad_ValidConfig` | `internal/config/config.go` | Parses testdata/valid.toml into Config struct |
| 8.2 | Load: missing file | `internal/config/config_test.go` | `TestLoad_MissingFile` | `internal/config/config.go` | Returns structured error |
| 8.3 | Load: invalid TOML syntax | `internal/config/config_test.go` | `TestLoad_InvalidSyntax` | `internal/config/config.go` | Returns parse error |
| 8.4 | Duration: parse | `internal/config/config_test.go` | `TestDuration_UnmarshalText` | `internal/config/config.go` | "30s", "2m", "1h" all parse correctly |
| **Validation** | | | | | |
| 8.5 | Validate: missing model | `internal/config/validate_test.go` | `TestValidate_MissingModel` | `internal/config/validate.go` | Error mentions gemini.model is required |
| 8.6 | Validate: invalid log level | `internal/config/validate_test.go` | `TestValidate_InvalidLogLevel` | `internal/config/validate.go` | Error for unknown log level |
| 8.7 | Validate: negative budget | `internal/config/validate_test.go` | `TestValidate_NegativeBudget` | `internal/config/validate.go` | Error for daily_budget_usd < 0 |
| 8.8 | Validate: MQTT plaintext without opt-in | `internal/config/validate_test.go` | `TestValidate_MQTTPlaintextWithoutOptIn` | `internal/config/validate.go` | S5: Error for tcp:// broker without allow_insecure_mqtt |
| 8.9 | Validate: multi-error accumulation | `internal/config/validate_test.go` | `TestValidate_MultipleErrors` | `internal/config/validate.go` | Multiple issues reported in one validation pass |
| 8.10 | Validate: all defaults populated | `internal/config/validate_test.go` | `TestValidate_Defaults` | `internal/config/validate.go` | Minimal valid config gets defaults (model, timeout, budget) |
| **qraft init** | | | | | |
| 8.11 | Init: creates config dir | `cmd/qraft/init_test.go` | `TestInitCommand_CreatesConfigDir` | `cmd/qraft/init.go` | Creates ~/.qraftworx/ with 0700 permissions |
| 8.12 | Init: writes default config | `cmd/qraft/init_test.go` | `TestInitCommand_WritesDefaultConfig` | `cmd/qraft/init.go` | Writes config.toml with sensible defaults |
| 8.13 | Init: creates log directory | `cmd/qraft/init_test.go` | `TestInitCommand_CreatesLogDir` | `cmd/qraft/init.go` | Creates ~/.qraftworx/logs/ with 0700 |
| 8.14 | Init: does not overwrite existing | `cmd/qraft/init_test.go` | `TestInitCommand_DoesNotOverwrite` | `cmd/qraft/init.go` | Existing config.toml is not clobbered, warns user |
| 8.15 | Init: validates GEMINI_API_KEY present | `cmd/qraft/init_test.go` | `TestInitCommand_WarnsNoAPIKey` | `cmd/qraft/init.go` | Prints warning if GEMINI_API_KEY not set |
| **Test Fixtures** | | | | | |
| 8.16 | Config test fixtures | `internal/config/testdata/` | N/A | N/A | valid.toml, invalid_syntax.toml, missing_model.toml, mqtt_plaintext.toml |

### Security Findings Addressed

- **S5 (MQTT auth):** Task 8.8. Config validation rejects plaintext MQTT without explicit opt-in.
- **S4 (file permissions):** Tasks 8.11, 8.13. Config and log directories created with 0700.

### Acceptance Criteria

- [ ] `go test ./internal/config/... -race` passes (10+ tests)
- [ ] `go test ./cmd/qraft/... -race` passes (init tests)
- [ ] Valid config loads and validates successfully
- [ ] Multi-error validation reports all issues at once
- [ ] `qraft init` creates directory structure with correct permissions
- [ ] Existing config not overwritten

### Estimated New Files: 10

- `internal/config/config.go`
- `internal/config/config_test.go`
- `internal/config/validate.go`
- `internal/config/validate_test.go`
- `cmd/qraft/init.go`
- `cmd/qraft/init_test.go`
- `internal/config/testdata/valid.toml`
- `internal/config/testdata/invalid_syntax.toml`
- `internal/config/testdata/missing_model.toml`
- `internal/config/testdata/mqtt_plaintext.toml`

---

## Dependency Graph

```
Phase 0 (Bootstrap)
    |
    v
Phase 1 (CerebroClient + SafePath) ----+
    |                                    |
    v                                    |
Phase 2 (Gemini Client) ------+         |
    |                          |         |
    v                          v         v
Phase 3 (Loop + Tools + Executor + Hydrator)
    |
    +-----> Phase 4 (Observability)
    |
    +-----> Phase 5 (Sensors) ---> feeds back into Hydrator
    |
    +-----> Phase 6 (Media) [requires SafePath from P1, FFmpegBuilder before impl]
    |
    +-----> Phase 7 (Upload) [requires SafePath from P1, Media from P6 for test fixtures]
    |
    +-----> Phase 8 (Config) [can start in parallel after P3, wires everything together]
```

**Critical path:** 0 -> 1 -> 2 -> 3 -> (4, 5, 6, 7, 8 in parallel or sequence)

Phases 4-8 depend on Phase 3 but not on each other, except:
- Phase 6 (Media) must come before Phase 7 (Upload) for test fixtures
- Phase 5 (Sensors) feeds into hydrator updates
- Phase 8 (Config) wires all prior phases together at the end

---

## Security Finding Coverage Matrix

| Finding | Severity | Phase | Tasks | Verified By |
|---------|----------|-------|-------|-------------|
| S1 (confirmation gate) | CRITICAL | 3 | 3.4-3.5 | TestStripControlChars_*, TestExecutor_NonTTY_* |
| S2 (ffmpeg injection) | CRITICAL | 6 | 6.1-6.7 | TestFFmpegBuilder_*, TestFFmpegBuilder_NoShellInvocation |
| S3 (prompt injection) | HIGH | 3, 5 | 3.17-3.18, 5.5-5.6, 5.9, 5.11 | TestHydratedContext_FormatForGemini_*, TestMQTTSensor_SchemaValidation |
| S4 (log security) | HIGH | 4 | 4.1-4.6 | TestNewLogger_FilePermissions, TestSecretScrubber_* |
| S5 (MQTT auth) | HIGH | 5, 8 | 5.1-5.2, 8.8 | TestMQTTConfig_RejectsPlaintext, TestValidate_MQTTPlaintextWithoutOptIn |
| S6 (cost control) | HIGH | 4 | 4.7-4.13 | TestTracker_PreCallGate_*, TestTracker_FileLock_* |
| S7 (upload validation) | HIGH | 7 | 7.1-7.7 | TestValidateMediaFile_*, TestUploadTool_RejectsPath* |
| S8 (GoPro) | MEDIUM | 5 | (MoonrakerSensor pattern) | Schema validation on HTTP responses |
| S9 (SafePath) | MEDIUM | 1 | 1.1-1.3 | TestNew_RejectsTraversal, TestNew_RejectsEscapingSymlink |
| S10 (timeouts) | MEDIUM | 2, 5, 7 | 2.10, 5.8, 7.11 | TestClient_Generate_Timeout, TestMoonrakerSensor_Timeout |
| S11 (media retention) | MEDIUM | 6 | (via SafePath + config) | Config-based retention policy |
| S12 (supply chain) | LOW | 0 | 0.7, 0.9 | CI: govulncheck, gitleaks |
| S13 (log integrity) | LOW | 4 | (future enhancement) | Not in v1 scope |
| N1 (hardcoded keys) | CRITICAL | 0 | 0.9 | gitleaks in pre-commit |

---

## Total Estimates

| Phase | New Files | New Tests (approx) |
|-------|-----------|-------------------|
| 0 (Bootstrap) | 18-20 | 0 |
| 1 (Cerebro + SafePath) | 4 | 11 |
| 2 (Gemini) | 6 + fixtures | 13 |
| 3 (Loop + Tools) | 17 | 24 |
| 4 (Observability) | 8 | 13 |
| 5 (Sensors) | 8 | 15 |
| 6 (Media) | 8 | 15 |
| 7 (Upload) | 8 | 13 |
| 8 (Config) | 10 | 16 |
| **Total** | **~87-89** | **~120** |
