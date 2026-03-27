# Qraft CLI Architecture Review

Companion to [qraft-cli-support.md](qraft-cli-support.md). Synthesized from independent reviews by principal architect and tech lead roles against Cerebro's established architecture, ADRs, and engineering standards.

**Verdict: REQUEST_CHANGES** -- The proposal has sound high-level instincts but contains several technically unsound recommendations, significant gaps, and no engineering rigor around testing, security, or operations.

---

## 1. What the Proposal Gets Right

- **Separation of concerns.** Cerebro as "Hippocampus" (storage) and Qraft as "Prefrontal Cortex" (cognition) correctly respects ADR-006 (Model B).
- **Python for the orchestrator.** Legitimate choice given the hobbyist library ecosystem (GPIO, MQTT, Moonraker) and the google-genai SDK's automatic function calling.
- **Physical safety guardrails.** Correctly identifies that LLM-directed hardware control needs confirmation gates.

---

## 2. Critical Issues (Must Fix)

### 2.1 CFFI/Shared Object Bridge -- Do Not Build

The proposal suggests "a CFFI/Shared Object bridge since it's Go" over subprocess. **This is the single most dangerous recommendation in the proposal.**

- Cerebro is a CLI-first tool. The Cobra commands and `--format json` output *are* the public API contract. CFFI would bypass this boundary entirely.
- Go's CGO + Python's CFFI creates a two-layer FFI chain (Python -> C -> Go -> C -> SQLite). Debugging crashes in this stack is miserable.
- The Go runtime is not designed to be embedded in a long-running Python process. Goroutine scheduling, GC pauses, and signal handling all conflict with Python's own runtime.
- Cross-compilation for the shared object (which must bundle Go runtime + SQLite + sqlite-vec) is a build-system nightmare.
- You lose process isolation. A crash in sqlite-vec takes down the Python process.

**Decision required:** Use subprocess with `--format json`. This is what Claude Code already does successfully. The CLI *is* the contract.

```python
class CerebroClient:
    def __init__(self, binary: str = "cerebro", db: str | None = None):
        self._binary = binary
        self._db = db

    def recall(self, query: str, limit: int = 5) -> list[Memory]:
        result = subprocess.run(
            [self._binary, "recall", query, "--limit", str(limit), "--format", "json"],
            capture_output=True, text=True, timeout=10
        )
        if result.returncode != 0:
            raise CerebroError(result.stderr)
        return [Memory.from_dict(m) for m in json.loads(result.stdout)]
```

If read latency becomes a measured bottleneck, consider direct SQLite reads (Python stdlib `sqlite3`) for the hydration path only -- the schema is the contract. Note: sqlite-vec virtual tables may require `Connection.load_extension()` with the native extension.

### 2.2 Security Architecture -- Absent

**Dynamic tool loading + LLM-directed execution + physical hardware = highest risk.**

The `~/qraftworx/tools/` directory scanner with no sandboxing means:
- Any file in that directory becomes executable code that Gemini can invoke with the user's full system permissions.
- Any process that can write to that directory can inject arbitrary code.
- No signature verification, no allowlist, no capability declaration.

The `[Y/n]` confirmation is insufficient for an agentic loop. If Qraft runs unattended (the point of automation), nobody is there to confirm.

**Required before implementation:**
- Tools must declare capabilities: `name`, `description`, `parameter schema` (JSON Schema), `required permissions` (network, GPIO, filesystem), `physical-safety classification` (safe / requires-confirmation).
- Start with hardcoded tools in a typed Python module, not directory scanning. A decorator-based registry (`@tool(name="check_printer", requires_confirmation=True)`) is cleaner.
- Directory scanning can come later as a plugin mechanism after the permission model is proven.
- Rate limiting on physical actuations.
- Kill switch that does not depend on software being healthy (hardware interlock).
- API key storage: where do Gemini keys live? File permissions? Environment variables? Not addressed at all.
- The `search_docs` scraping tool lets Gemini choose what URLs to fetch. This is a prompt-injection vector (malicious URLs in retrieved memories).

### 2.3 Multi-Writer Memory Conflict

Both Claude Code (via hooks) and Qraft (via Gemini) writing to the same Cerebro SQLite store is a data integrity risk with no coordination mechanism.

- Cerebro uses SQLite WAL mode with a 5-second busy timeout. Concurrent subprocess writes compete for the lock.
- The daily consolidation job has Gemini rewriting memory history while Claude Code's GC hook may also be running. No ordering guarantee.
- Two agents with different cognition models making independent decisions about memory importance creates incoherent state.

**Decision required:** Define which agent owns which store, or establish a write coordination protocol (e.g., store-per-agent, or a locking convention).

### 2.4 Embedding Provider Mismatch in Offline Mode

The Gemini -> Ollama fallback silently breaks vector search if the providers use different embedding dimensions.

- Voyage: 1024-dim vectors.
- Ollama nomic-embed-text: 768-dim vectors.
- You cannot search a 1024-dim `vec_nodes` table with a 768-dim query vector. This is a hard failure, not graceful degradation.

**Decision required:** Qraft must only use Ollama for embeddings if Cerebro was initialized with Ollama. Switching mid-session is architecturally impossible without a re-embedding migration.

---

## 3. Architectural Concerns (Should Fix)

### 3.1 Ollama as "Search Intent Generator" -- Remove This Layer

The proposal adds a local LLM call to "rewrite" the user's query before sending it to Cerebro. This is unnecessary indirection.

**Latency math:**
| Step | Without rewriting | With rewriting |
|------|-------------------|----------------|
| Ollama inference (query rewrite) | -- | 500-2000ms |
| Cerebro embedding | 50-100ms | 50-100ms |
| Cerebro vector search | 5-20ms | 5-20ms |
| **Total** | **55-120ms** | **555-2120ms** |

That is a 5-20x latency multiplier for marginal retrieval improvement. Cerebro already does semantic search with composite scoring (relevance 0.35 + importance 0.25 + recency 0.25 + structural 0.15). The raw query gets embedded and matched directly.

If retrieval quality is insufficient, the fix is to improve the embedding model or tune scoring weights -- not add an inference step. If query expansion is needed, use Gemini (already being called) rather than a separate Ollama invocation.

### 3.2 Memory Consolidation -- Wrong Primitives

The proposal says: use `cerebro supersede` to replace 20 episode nodes with 1 reflection.

**Problems:**
- `supersede` creates a one-to-one replacement with a `supersedes` edge. It is the wrong primitive for 20-to-1 consolidation.
- Cerebro's GC evaluates only `active` nodes. If GC runs before consolidation, episodes may be archived before consolidation reaches them.
- Automated LLM summarization creates a lossy compression feedback loop: summaries of summaries over weeks.

**Correct approach:**
1. Create one new reflection node with the consolidated content.
2. `cerebro mark-consolidated` the 20 source episodes (sets status to `consolidated`, excluded from GC evaluation).
3. `cerebro edge` from the reflection to each episode (relation: `consolidates`). Preserves graph structure and originals for audit.
4. Run consolidation *before* GC, not independently.
5. Add a `consolidation_generation` metadata field to cap re-summarization depth.
6. Require human approval for consolidation, at least initially.

### 3.3 The "Port to Go Later" Advice -- A Trap

> "Once the logic is 'frozen' and stable, you can port the core loop into Go."

This will never happen:
- The logic will never be "frozen" in an actively evolving hobby system.
- Language migrations are rewrites, not ports. Different libraries, different error handling, different concurrency models.
- It deprioritizes engineering quality in Python ("it's temporary anyway") while ensuring the rewrite never occurs. Result: a permanent prototype.

**Decision required:** Either commit to Python with full engineering rigor (testing, typing, linting, CI), or start in Go and accept the SDK tradeoff. Manual function calling in Go is more explicit, which is arguably *better* for a system that controls physical hardware.

### 3.4 Context Window Management -- Missing

The hydrator has no token budget. Cerebro recall returns up to 20 nodes (type-stratified). Add hardware telemetry, search results, and system instructions, and the total context can easily exceed useful limits. The proposal does not address truncation or prioritization.

### 3.5 "Thinking Level" Routing -- Under-Specified

Who classifies `"Hello" -> Minimal` vs `"Debug ESC logic" -> High`? If Qraft uses heuristics (keyword matching), it is fragile. If it asks Gemini to self-classify, it wastes a round-trip. If user-specified, it is friction. Start with a simple CLI flag (`qraft --think high "debug this"`) rather than auto-classification.

### 3.6 Offline Fallback -- Overly Ambitious

Do not try to make Ollama a drop-in replacement for Gemini. They have different function-calling protocols, different capabilities, and different output formats. A 7B model on a Raspberry Pi will not reliably execute multi-step tool-use chains.

**Better approach:** Degraded mode = thin CLI over Cerebro commands directly. `qraft remember "X"` -> `cerebro add`, `qraft recall "X"` -> `cerebro recall`. No LLM needed for basic memory operations. Reserve Ollama for conversational responses only, with explicit caveats.

---

## 4. Missing Concerns

| Concern | Status in Proposal | Required |
|---------|-------------------|----------|
| **Testing strategy** | Absent | pytest, mypy, ruff, integration tests against real cerebro binary, mock/replay for Gemini SDK |
| **Error handling** | Happy path only | Explicit error taxonomy per I/O boundary, timeouts on every external call, structured error types |
| **Observability** | Not mentioned | Structured logging (JSON lines) at every layer boundary: input, hydrated context, Gemini request/response, tool calls, Cerebro commands |
| **Cost management** | Not mentioned | Per-session token budget, daily/monthly API spend caps, cost-per-interaction logging |
| **Rate limiting** | Not mentioned | Gemini rate limits, backoff/throttle strategy for automatic function calling loops |
| **Configuration management** | Not mentioned | Config file format, schema, validation, defaults, environment overrides |
| **Deployment story** | Not mentioned | How is Qraft installed? How is the Cerebro binary version managed alongside? |
| **Version compatibility** | Not mentioned | `cerebro --version` check on init, schema contract between Python wrapper and CLI output |
| **Concurrency model** | Not mentioned | MQTT is async, Gemini is async, Ollama is async, hardware polling is async -- asyncio? threading? |
| **Graceful shutdown** | Not mentioned | Ctrl+C during function-calling loop mid-hardware-actuation |
| **State machine definition** | Mentioned but not defined | States, transitions, invalid transition handling, state persistence |
| **Pre-commit hooks / CI** | Not mentioned | Match Cerebro's standards: ruff, mypy, pytest, CI pipeline |
| **Commit conventions** | Not mentioned | Adopt Cerebro's conventional commits |
| **Backup before consolidation** | Not mentioned | Dry-run mode, rollback capability for memory rewriting |

---

## 5. Risk Assessment (Ordered by Severity)

| # | Risk | Severity | Mitigation |
|---|------|----------|------------|
| 1 | CFFI bridge creates deep coupling, fragile FFI chain | **Critical** | Use subprocess. Do not build CFFI. |
| 2 | Dynamic tool loading + LLM execution + physical hardware with no sandbox | **Critical** | Build permission model, hardcode tools first, add sandboxing |
| 3 | Embedding dimension mismatch in offline mode breaks vector search | **High** | Lock to single embedding provider per store; no mid-session switching |
| 4 | Multi-writer conflict (Claude Code + Qraft on same store) | **High** | Store-per-agent or write coordination protocol |
| 5 | Automated consolidation corrupts memory graph | **Medium** | Use mark-consolidated + edges, not supersede; cap consolidation generations |
| 6 | No error handling across five external dependencies | **Medium** | Error taxonomy, timeouts, circuit breakers |
| 7 | Scope creep (3 layers + Ollama rewriting + hardware + scraping + consolidation + fallback + tool registry) | **Medium** | Ruthless scope reduction; build incrementally |
| 8 | "Port to Go" creates permanent prototype mindset | **Medium** | Commit to one language with full engineering rigor |

---

## 6. Recommended Build Order

If proceeding with Python:

1. **`CerebroClient`** -- subprocess wrapper with JSON parsing, error handling, version check, timeout handling. This is the foundation. TDD from the first line.
2. **Single-pass loop** -- user input -> `cerebro recall` -> inject context -> Gemini API -> response. No tools, no hydration layers. Prove the loop works end-to-end.
3. **Hardcoded tool registry** -- 2-3 tools with typed function signatures: `add_memory`, `search_memory`, `get_stats`. Decorator-based registration, not directory scanning.
4. **Structured logging** -- JSON lines at every boundary. Debug-ability before features.
5. **Hardware integration tools** -- MQTT/sensor tools behind confirmation gates with declared permissions.
6. **Offline degraded mode** -- Direct CLI passthrough, no LLM reasoning.
7. **Dynamic tool registry** -- If and only if warranted by real usage.
8. **Consolidation** -- With correct primitives (mark-consolidated + edges), human approval, and backup.

---

## 7. Proposal Tone (Meta-Observation)

The proposal reads as a sales pitch rather than an engineering document. Phrases like "pivotal architectural crossroad," "expert breakdown," and "the Author's Path" optimize for persuasion over technical rigor. An architecture document should be dispassionate and precise. Several claims are stated as facts without evidence (e.g., "Gemini 3.1 Flash" -- this model version does not exist in current Gemini naming conventions).

---

## 8. Open Questions for Q

1. Will Qraft and Claude Code share the same Cerebro store, or should each agent have its own?
2. Is the Gemini SDK's automatic function calling a hard requirement, or would explicit tool invocation (possible in Go) be acceptable?
3. What is the target deployment environment -- desktop, Raspberry Pi, or both?
4. What is the acceptable latency budget for a single Qraft interaction?
5. Is offline mode a v1 requirement or a future nice-to-have?
6. What physical hardware will be controlled, and what are the actual safety implications of malfunction?
