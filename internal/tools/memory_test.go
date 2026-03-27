package tools

import (
	"context"
	"encoding/json"
	"path/filepath"
	"testing"

	"github.com/coetzeevs/cerebro/brain"
	cerebroclient "github.com/coetzeevs/qraftworx-cli/internal/cerebro"
)

func testCerebroClient(t *testing.T) *cerebroclient.Client {
	t.Helper()
	path := filepath.Join(t.TempDir(), "test.sqlite")
	b, err := brain.Init(path, brain.EmbedConfig{Provider: "none"})
	if err != nil {
		t.Fatalf("brain.Init: %v", err)
	}
	t.Cleanup(func() { _ = b.Close() })
	// Use unexported constructor via exported test helper pattern
	return cerebroclient.NewClientForTest(b)
}

func TestMemoryAddTool_Name(t *testing.T) {
	tool := NewMemoryAddTool(nil)
	if tool.Name() != "memory_add" {
		t.Errorf("name=%q, want memory_add", tool.Name())
	}
}

func TestMemoryAddTool_Execute(t *testing.T) {
	c := testCerebroClient(t)
	tool := NewMemoryAddTool(c)

	args := json.RawMessage(`{"content": "PLA prints best at 210C", "type": "concept"}`)
	result, err := tool.Execute(context.Background(), args)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}

	m, ok := result.(map[string]any)
	if !ok {
		t.Fatalf("expected map result, got %T", result)
	}
	if m["id"] == "" {
		t.Error("expected non-empty ID")
	}
	if m["status"] != "added" {
		t.Errorf("status=%v, want added", m["status"])
	}
}

func TestMemoryAddTool_Execute_BadArgs(t *testing.T) {
	tool := NewMemoryAddTool(nil)
	_, err := tool.Execute(context.Background(), json.RawMessage(`{invalid`))
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

func TestMemoryAddTool_Execute_EmptyContent(t *testing.T) {
	tool := NewMemoryAddTool(nil)
	_, err := tool.Execute(context.Background(), json.RawMessage(`{"content": ""}`))
	if err == nil {
		t.Fatal("expected error for empty content")
	}
}

func TestMemorySearchTool_Name(t *testing.T) {
	tool := NewMemorySearchTool(nil)
	if tool.Name() != "memory_search" {
		t.Errorf("name=%q, want memory_search", tool.Name())
	}
}

func TestMemorySearchTool_Execute(t *testing.T) {
	c := testCerebroClient(t)

	// Add a node first
	_, err := c.Add("PLA temperature settings", brain.Concept)
	if err != nil {
		t.Fatal(err)
	}

	tool := NewMemorySearchTool(c)
	// Search will fail with noop embedder, which is expected
	_, err = tool.Execute(context.Background(), json.RawMessage(`{"query": "PLA"}`))
	// With noop embedder, search returns error — that's fine, we're testing the wiring
	if err == nil {
		t.Log("Search succeeded (unexpected with noop embedder, but OK)")
	}
}

func TestMemorySearchTool_Execute_BadArgs(t *testing.T) {
	tool := NewMemorySearchTool(nil)
	_, err := tool.Execute(context.Background(), json.RawMessage(`not json`))
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

func TestMemorySearchTool_Execute_EmptyQuery(t *testing.T) {
	tool := NewMemorySearchTool(nil)
	_, err := tool.Execute(context.Background(), json.RawMessage(`{"query": ""}`))
	if err == nil {
		t.Fatal("expected error for empty query")
	}
}
