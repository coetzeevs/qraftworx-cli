// Package cerebro wraps the Cerebro brain/ library with the subset of operations
// QraftWorx needs. It is the sole interface between Qraft and the Cerebro persistent
// memory system -- no other package imports brain/ directly.
//
// # Key Types
//
//   - Client: wraps brain.Brain with project and optional global store access.
//     Provides Search, Add, List, Get, AddEdge, and Stats operations.
//   - NewClient: opens a project brain by directory path.
//   - NewClientWithGlobal: opens both project and global brains for merged search.
//   - NewClientFromPath: opens a brain directly from a SQLite file path (for testing).
//   - NewClientForTest: creates a Client from an existing brain.Brain instance.
//
// # Architecture Role
//
// The cerebro package sits at the bottom of the dependency chain. The Hydrator
// queries it for relevant memories during context assembly, and the memory tools
// (memory_add, memory_search) use it to persist and retrieve knowledge.
//
// The Client supports both project-scoped and global brains. When a global brain
// is configured, SearchWithGlobal merges results from both stores. Memory nodes
// are typed (Episode, Concept, Procedure, Reflection) and support functional
// options like WithImportance and WithSubtype.
//
// # Security Considerations
//
// The cerebro package accepts context.Context for timeout propagation (S10).
// Memory content is treated as untrusted data by the hydrator, which applies
// length caps and injection detection before forwarding to Gemini (S3).
//
// # Testing
//
// Tests use brain.Init with a noop embedder (Provider: "none") in t.TempDir().
// Vector search requires embeddings, so Search tests verify error handling
// rather than result correctness. Add/List/Get/Stats are tested with real
// SQLite databases.
package cerebro
