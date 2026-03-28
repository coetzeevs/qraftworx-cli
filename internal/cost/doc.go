// Package cost tracks Gemini API spend with a file-persisted daily counter
// and pre-call budget gate.
//
// # Key Types
//
//   - Tracker: the main cost tracker. Manages a daily budget with pre-call
//     gating, usage recording, and threshold warnings.
//   - ErrBudgetExhausted: sentinel error returned when a call would exceed
//     the daily budget.
//   - EstimateCost: utility function to calculate USD cost from token counts.
//
// # Architecture Role
//
// The cost package integrates with the Gemini tool loop. Before each Generate
// call, RunLoop calls PreCallGate to verify the estimated cost would not
// exceed the daily budget. After each call, RecordUsage accumulates actual
// token usage to the file-persisted counter.
//
// The daily counter is stored as a JSON file with a date field. When the date
// no longer matches today (UTC), the counter resets to zero. This provides
// automatic daily budget renewal without cron jobs.
//
// # Cost Model
//
// Costs are estimated using Gemini 2.5 Flash approximate pricing:
//   - Prompt tokens: $0.075 per 1M tokens
//   - Candidate tokens: $0.30 per 1M tokens
//
// PreCallGate conservatively estimates all tokens as candidate tokens
// (most expensive) to avoid underestimation.
//
// # Security Considerations
//
//   - S6 (Cost Control): PreCallGate runs before every API call, not after.
//     This prevents budget overruns from being discovered only post-call.
//   - S6 (Missing Metadata): When promptTokens is negative (signaling absent
//     UsageMetadata), RecordUsage substitutes MaxCostPromptTokens (1M) and
//     MaxCostCandidateTokens (1M) to prevent cost tracking bypass.
//   - S6 (Concurrency): The Tracker uses a sync.Mutex for thread safety.
//     The counter file is written with 0600 permissions.
//
// # Testing
//
// Tests inject nowFunc for time control (daily reset verification). Concurrent
// safety is verified with 20 goroutines making 10 calls each. Counter files
// use t.TempDir(). Warning threshold logging uses a bytes.Buffer slog sink.
package cost
