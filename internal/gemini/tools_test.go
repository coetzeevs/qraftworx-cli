package gemini

import (
	"testing"
)

type mockTool struct {
	name        string
	description string
	parameters  map[string]any
}

func (m mockTool) Name() string               { return m.name }
func (m mockTool) Description() string        { return m.description }
func (m mockTool) Parameters() map[string]any { return m.parameters }

// Task 2.12
func TestBuildToolDeclarations(t *testing.T) {
	tools := []ToolDeclarer{
		mockTool{
			name:        "memory_search",
			description: "Search memory for relevant nodes",
			parameters: map[string]any{
				"query": map[string]any{
					"type":        "STRING",
					"description": "search query",
					"required":    true,
				},
				"limit": map[string]any{
					"type":        "INTEGER",
					"description": "max results",
				},
			},
		},
		mockTool{
			name:        "memory_add",
			description: "Add a memory node",
			parameters: map[string]any{
				"content": "STRING",
			},
		},
	}

	result := BuildToolDeclarations(tools)

	if len(result) != 1 {
		t.Fatalf("expected 1 tool group, got %d", len(result))
	}
	defs := result[0].FunctionDeclarations
	if len(defs) != 2 {
		t.Fatalf("expected 2 declarations, got %d", len(defs))
	}
	if defs[0].Name != "memory_search" {
		t.Errorf("decl[0].name=%q, want memory_search", defs[0].Name)
	}
	if defs[1].Name != "memory_add" {
		t.Errorf("decl[1].name=%q, want memory_add", defs[1].Name)
	}
	if defs[0].Description != "Search memory for relevant nodes" {
		t.Errorf("decl[0].description=%q", defs[0].Description)
	}
	if defs[0].Parameters == nil {
		t.Fatal("expected non-nil parameters schema")
	}
	if len(defs[0].Parameters.Required) != 1 || defs[0].Parameters.Required[0] != "query" {
		t.Errorf("required=%v, want [query]", defs[0].Parameters.Required)
	}
}

func TestBuildToolDeclarations_Empty(t *testing.T) {
	result := BuildToolDeclarations(nil)
	if result != nil {
		t.Errorf("expected nil for empty input, got %v", result)
	}
}

func TestToolNames(t *testing.T) {
	tools := []ToolDeclarer{
		mockTool{name: "a"},
		mockTool{name: "b"},
	}
	names := ToolNames(tools)
	if len(names) != 2 || names[0] != "a" || names[1] != "b" {
		t.Errorf("names=%v, want [a b]", names)
	}
}
