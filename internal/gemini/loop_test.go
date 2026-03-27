package gemini

import (
	"context"
	"encoding/json"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"google.golang.org/genai"
)

type mockExecutor struct {
	execFn func(ctx context.Context, name string, args json.RawMessage) (any, error)
}

func (m *mockExecutor) Execute(ctx context.Context, name string, args json.RawMessage) (any, error) {
	return m.execFn(ctx, name, args)
}

// Task 3.19
func TestToolLoop_TextResponse(t *testing.T) {
	c := newTestClient(func(_ context.Context, _ string, _ []*genai.Content, _ *genai.GenerateContentConfig) (*genai.GenerateContentResponse, error) {
		return mockTextResponse("Hello!"), nil
	})

	result, err := RunLoop(context.Background(), c, nil, []Part{TextPart{Text: "hi"}}, LoopConfig{})
	if err != nil {
		t.Fatalf("RunLoop: %v", err)
	}
	if result.Text != "Hello!" {
		t.Errorf("text=%q, want Hello!", result.Text)
	}
}

// Task 3.20
func TestToolLoop_FunctionCallThenText(t *testing.T) {
	var callCount atomic.Int32
	c := newTestClient(func(_ context.Context, _ string, contents []*genai.Content, _ *genai.GenerateContentConfig) (*genai.GenerateContentResponse, error) {
		n := callCount.Add(1)
		if n == 1 {
			// First call: return function call
			return mockFuncCallResponse("memory_search", map[string]any{"query": "PLA"}), nil
		}
		// Second call: return text
		return mockTextResponse("PLA prints at 210C"), nil
	})
	c.registeredTools["memory_search"] = true

	exec := &mockExecutor{
		execFn: func(_ context.Context, name string, _ json.RawMessage) (any, error) {
			return map[string]any{"results": []string{"PLA at 210C"}}, nil
		},
	}

	result, err := RunLoop(context.Background(), c, exec, []Part{TextPart{Text: "PLA temp?"}}, LoopConfig{})
	if err != nil {
		t.Fatalf("RunLoop: %v", err)
	}
	if result.Text != "PLA prints at 210C" {
		t.Errorf("text=%q, want 'PLA prints at 210C'", result.Text)
	}
	if callCount.Load() != 2 {
		t.Errorf("generate calls=%d, want 2", callCount.Load())
	}
}

// Task 3.21
func TestToolLoop_MaxIterations(t *testing.T) {
	c := newTestClient(func(_ context.Context, _ string, _ []*genai.Content, _ *genai.GenerateContentConfig) (*genai.GenerateContentResponse, error) {
		// Always return a function call, never text
		return mockFuncCallResponse("looping_tool", map[string]any{}), nil
	})
	c.registeredTools["looping_tool"] = true

	exec := &mockExecutor{
		execFn: func(_ context.Context, _ string, _ json.RawMessage) (any, error) {
			return "ok", nil
		},
	}

	_, err := RunLoop(context.Background(), c, exec, []Part{TextPart{Text: "loop"}}, LoopConfig{MaxIterations: 3})
	if !errors.Is(err, ErrMaxIterations) {
		t.Fatalf("expected ErrMaxIterations, got: %v", err)
	}
}

// Task 3.22
func TestToolLoop_ToolError(t *testing.T) {
	var callCount atomic.Int32
	c := newTestClient(func(_ context.Context, _ string, contents []*genai.Content, _ *genai.GenerateContentConfig) (*genai.GenerateContentResponse, error) {
		n := callCount.Add(1)
		if n == 1 {
			return mockFuncCallResponse("failing_tool", map[string]any{}), nil
		}
		// After receiving error response, model returns text
		return mockTextResponse("I see the tool failed"), nil
	})
	c.registeredTools["failing_tool"] = true

	exec := &mockExecutor{
		execFn: func(_ context.Context, _ string, _ json.RawMessage) (any, error) {
			return nil, errors.New("tool broken")
		},
	}

	result, err := RunLoop(context.Background(), c, exec, []Part{TextPart{Text: "do it"}}, LoopConfig{})
	if err != nil {
		t.Fatalf("RunLoop: %v", err)
	}
	if result.Text != "I see the tool failed" {
		t.Errorf("text=%q", result.Text)
	}
}

// Verify loop doesn't need executor for text-only responses
func TestToolLoop_NilExecutor_TextOnly(t *testing.T) {
	c := newTestClient(func(_ context.Context, _ string, _ []*genai.Content, _ *genai.GenerateContentConfig) (*genai.GenerateContentResponse, error) {
		return mockTextResponse("simple"), nil
	})

	result, err := RunLoop(context.Background(), c, nil, []Part{TextPart{Text: "hi"}}, LoopConfig{})
	if err != nil {
		t.Fatal(err)
	}
	if result.Text != "simple" {
		t.Errorf("text=%q", result.Text)
	}
}

// newTestClient is already defined in client_test.go (same package),
// but we need to ensure the sleepFn is set for loop tests too.
func init() {
	// Ensure test helpers are available
	_ = time.Millisecond
}
