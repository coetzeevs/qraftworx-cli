package executor

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/coetzeevs/qraftworx-cli/internal/tools"
)

type mockTool struct {
	name    string
	confirm bool
	execFn  func(ctx context.Context, args json.RawMessage) (any, error)
}

func (m mockTool) Name() string                      { return m.name }
func (m mockTool) Description() string               { return "mock" }
func (m mockTool) Parameters() map[string]any        { return nil }
func (m mockTool) RequiresConfirmation() bool        { return m.confirm }
func (m mockTool) Permissions() tools.ToolPermission { return tools.ToolPermission{} }
func (m mockTool) Execute(ctx context.Context, args json.RawMessage) (any, error) {
	if m.execFn != nil {
		return m.execFn(ctx, args)
	}
	return "executed", nil
}

func registryWith(tt ...mockTool) *tools.Registry {
	r := tools.NewRegistry()
	for _, t := range tt {
		r.Register(t)
	}
	return r
}

// Task 3.5
func TestExecutor_NonTTY_DeniesConfirmation(t *testing.T) {
	r := registryWith(mockTool{name: "hw", confirm: true})
	e := New(r, WithTTY(false))

	_, err := e.Execute(context.Background(), "hw", nil)
	if !errors.Is(err, ErrNonInteractiveDenied) {
		t.Fatalf("expected ErrNonInteractiveDenied, got: %v", err)
	}
}

// Task 3.6
func TestExecutor_Execute_ToolNotFound(t *testing.T) {
	r := tools.NewRegistry()
	e := New(r)

	_, err := e.Execute(context.Background(), "nonexistent", nil)
	if !errors.Is(err, ErrToolNotFound) {
		t.Fatalf("expected ErrToolNotFound, got: %v", err)
	}
}

// Task 3.7
func TestExecutor_Execute_NoConfirmation(t *testing.T) {
	r := registryWith(mockTool{name: "safe", confirm: false})
	e := New(r)

	result, err := e.Execute(context.Background(), "safe", nil)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if result != "executed" {
		t.Errorf("result=%v, want executed", result)
	}
}

// Task 3.8
func TestExecutor_Execute_RequiresConfirmation_Approved(t *testing.T) {
	r := registryWith(mockTool{name: "hw", confirm: true})
	e := New(r, WithTTY(true), WithConfirmFunc(func(_, _ string) (bool, error) {
		return true, nil
	}))

	result, err := e.Execute(context.Background(), "hw", nil)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if result != "executed" {
		t.Errorf("result=%v, want executed", result)
	}
}

func TestExecutor_Execute_RequiresConfirmation_Denied(t *testing.T) {
	r := registryWith(mockTool{name: "hw", confirm: true})
	e := New(r, WithTTY(true), WithConfirmFunc(func(_, _ string) (bool, error) {
		return false, nil
	}))

	_, err := e.Execute(context.Background(), "hw", nil)
	if !errors.Is(err, ErrUserDenied) {
		t.Fatalf("expected ErrUserDenied, got: %v", err)
	}
}

// Task 3.9
func TestExecutor_Execute_PanicRecovery(t *testing.T) {
	r := registryWith(mockTool{
		name: "panicker",
		execFn: func(_ context.Context, _ json.RawMessage) (any, error) {
			panic("boom")
		},
	})
	e := New(r)

	_, err := e.Execute(context.Background(), "panicker", nil)
	if err == nil {
		t.Fatal("expected error from panic recovery")
	}
	if !contains(err.Error(), "panicked") {
		t.Errorf("error should mention panic: %v", err)
	}
}

// Task 3.10
func TestExecutor_Execute_ErrorCountCap(t *testing.T) {
	r := registryWith(mockTool{
		name: "fail",
		execFn: func(_ context.Context, _ json.RawMessage) (any, error) {
			return nil, errors.New("tool failed")
		},
	})
	e := New(r, WithMaxErrors(3))

	// Exhaust error budget
	for i := 0; i < 3; i++ {
		_, _ = e.Execute(context.Background(), "fail", nil)
	}

	// Next call should return ErrTooManyErrors
	_, err := e.Execute(context.Background(), "fail", nil)
	if !errors.Is(err, ErrTooManyErrors) {
		t.Fatalf("expected ErrTooManyErrors, got: %v", err)
	}
}

func TestExecutor_ResetErrors(t *testing.T) {
	r := registryWith(mockTool{
		name: "fail",
		execFn: func(_ context.Context, _ json.RawMessage) (any, error) {
			return nil, errors.New("tool failed")
		},
	})
	e := New(r, WithMaxErrors(1))

	_, _ = e.Execute(context.Background(), "fail", nil)
	e.ResetErrors()

	// Should be able to execute again
	_, err := e.Execute(context.Background(), "fail", nil)
	if errors.Is(err, ErrTooManyErrors) {
		t.Fatal("should not be ErrTooManyErrors after reset")
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && searchString(s, substr)
}

func searchString(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
