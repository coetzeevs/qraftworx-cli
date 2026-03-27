package hydrator

import (
	"context"
	"testing"

	"github.com/coetzeevs/cerebro/brain"
)

// Task 3.14
func TestHydrator_Hydrate_BasicPrompt(t *testing.T) {
	h := New(nil, 8192) // no cerebro client — memories will be empty

	hc, err := h.Hydrate(context.Background(), "hello world")
	if err != nil {
		t.Fatalf("Hydrate: %v", err)
	}
	if hc.UserPrompt != "hello world" {
		t.Errorf("prompt=%q, want %q", hc.UserPrompt, "hello world")
	}
	if hc.SystemPrompt == "" {
		t.Error("expected non-empty system prompt")
	}
	if len(hc.Memories) != 0 {
		t.Errorf("expected 0 memories without cerebro, got %d", len(hc.Memories))
	}
}

// Task 3.15
func TestHydrator_Hydrate_TruncatesAtBudget(t *testing.T) {
	h := New(nil, 10) // very tight budget

	hc := &HydratedContext{
		UserPrompt:   "hi",
		SystemPrompt: "system",
		Memories: []brain.ScoredNode{
			{Node: brain.Node{Content: "high score memory with lots of content that takes many tokens"}, Score: 0.9},
			{Node: brain.Node{Content: "medium score"}, Score: 0.5},
			{Node: brain.Node{Content: "low score"}, Score: 0.3},
		},
	}

	// Estimate exceeds budget — should truncate
	est := h.estimateTokens(hc)
	if est <= 10 {
		t.Skipf("estimate %d already within budget, adjust test", est)
	}

	hc.truncateMemories(10)
	// After truncation, some memories should be removed
	if hc.TokenEstimate > 10 {
		t.Errorf("token estimate %d still exceeds budget 10", hc.TokenEstimate)
	}
}

// Task 3.16
func TestHydrator_Hydrate_NoMemories(t *testing.T) {
	h := New(nil, 8192)

	hc, err := h.Hydrate(context.Background(), "test prompt")
	if err != nil {
		t.Fatal(err)
	}
	if len(hc.Memories) != 0 {
		t.Errorf("expected nil/empty memories, got %d", len(hc.Memories))
	}
}
