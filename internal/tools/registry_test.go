package tools

import (
	"context"
	"encoding/json"
	"testing"
)

type stubTool struct {
	name    string
	confirm bool
}

func (s stubTool) Name() string                                              { return s.name }
func (s stubTool) Description() string                                       { return "stub" }
func (s stubTool) Parameters() map[string]any                                { return nil }
func (s stubTool) Execute(_ context.Context, _ json.RawMessage) (any, error) { return "ok", nil }
func (s stubTool) RequiresConfirmation() bool                                { return s.confirm }
func (s stubTool) Permissions() ToolPermission                               { return ToolPermission{} }

func TestRegistry_RegisterAndGet(t *testing.T) {
	r := NewRegistry()
	r.Register(stubTool{name: "test_tool"})

	tool, ok := r.Get("test_tool")
	if !ok {
		t.Fatal("expected tool to be found")
	}
	if tool.Name() != "test_tool" {
		t.Errorf("name=%q, want test_tool", tool.Name())
	}
}

func TestRegistry_Get_NotFound(t *testing.T) {
	r := NewRegistry()
	_, ok := r.Get("nonexistent")
	if ok {
		t.Fatal("expected not found")
	}
}

func TestRegistry_All(t *testing.T) {
	r := NewRegistry()
	r.Register(stubTool{name: "a"})
	r.Register(stubTool{name: "b"})

	all := r.All()
	if len(all) != 2 {
		t.Fatalf("expected 2 tools, got %d", len(all))
	}
}

func TestRegistry_Register_DuplicatePanics(t *testing.T) {
	r := NewRegistry()
	r.Register(stubTool{name: "dup"})

	defer func() {
		if recover() == nil {
			t.Fatal("expected panic on duplicate registration")
		}
	}()
	r.Register(stubTool{name: "dup"})
}
