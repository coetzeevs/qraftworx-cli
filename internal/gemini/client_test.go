package gemini

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"google.golang.org/genai"
)

// newTestClient creates a Client with the given mock generateFn.
// Uses no-op sleep and no timeout pressure for fast tests.
func newTestClient(fn generateFunc) *Client {
	return &Client{
		model:           "test-model",
		registeredTools: make(map[string]bool),
		maxRetries:      3,
		timeout:         10 * time.Second,
		generateFn:      fn,
		sleepFn:         func(time.Duration) {}, // no-op for fast tests
	}
}

func mockTextResponse(text string) *genai.GenerateContentResponse {
	return &genai.GenerateContentResponse{
		Candidates: []*genai.Candidate{{
			Content: &genai.Content{
				Parts: []*genai.Part{{Text: text}},
				Role:  "model",
			},
			FinishReason: "STOP",
		}},
		UsageMetadata: &genai.GenerateContentResponseUsageMetadata{
			PromptTokenCount:     10,
			CandidatesTokenCount: 5,
			TotalTokenCount:      15,
		},
	}
}

func mockFuncCallResponse(name string, args map[string]any) *genai.GenerateContentResponse {
	return &genai.GenerateContentResponse{
		Candidates: []*genai.Candidate{{
			Content: &genai.Content{
				Parts: []*genai.Part{{
					FunctionCall: &genai.FunctionCall{Name: name, Args: args},
				}},
				Role: "model",
			},
			FinishReason: "STOP",
		}},
		UsageMetadata: &genai.GenerateContentResponseUsageMetadata{
			PromptTokenCount:     20,
			CandidatesTokenCount: 8,
			TotalTokenCount:      28,
		},
	}
}

// Task 2.1
func TestNewClient_MissingAPIKey(t *testing.T) {
	t.Setenv("GEMINI_API_KEY", "")
	_, err := NewClient("gemini-2.5-flash")
	if err == nil {
		t.Fatal("expected error for missing API key")
	}
}

// Task 2.2
func TestClient_Generate_TextResponse(t *testing.T) {
	c := newTestClient(func(_ context.Context, _ string, _ []*genai.Content, _ *genai.GenerateContentConfig) (*genai.GenerateContentResponse, error) {
		return mockTextResponse("Hello from Gemini"), nil
	})

	result, err := c.Generate(context.Background(), []Part{TextPart{Text: "hello"}})
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	if result.Text != "Hello from Gemini" {
		t.Errorf("text=%q, want %q", result.Text, "Hello from Gemini")
	}
	if result.FinishReason != "STOP" {
		t.Errorf("finish=%q, want STOP", result.FinishReason)
	}
}

// Task 2.3
func TestClient_Generate_FunctionCall(t *testing.T) {
	args := map[string]any{"query": "PLA temperature"}
	c := newTestClient(func(_ context.Context, _ string, _ []*genai.Content, _ *genai.GenerateContentConfig) (*genai.GenerateContentResponse, error) {
		return mockFuncCallResponse("memory_search", args), nil
	})
	c.registeredTools["memory_search"] = true

	result, err := c.Generate(context.Background(), []Part{TextPart{Text: "search"}})
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	if len(result.FunctionCalls) != 1 {
		t.Fatalf("function calls=%d, want 1", len(result.FunctionCalls))
	}
	fc := result.FunctionCalls[0]
	if fc.Name != "memory_search" {
		t.Errorf("name=%q, want memory_search", fc.Name)
	}
	if fc.Args["query"] != "PLA temperature" {
		t.Errorf("args[query]=%v, want PLA temperature", fc.Args["query"])
	}
}

// Task 2.4
func TestClient_Generate_MultipleFunctionCalls(t *testing.T) {
	c := newTestClient(func(_ context.Context, _ string, _ []*genai.Content, _ *genai.GenerateContentConfig) (*genai.GenerateContentResponse, error) {
		return &genai.GenerateContentResponse{
			Candidates: []*genai.Candidate{{
				Content: &genai.Content{
					Parts: []*genai.Part{
						{FunctionCall: &genai.FunctionCall{Name: "memory_search", Args: map[string]any{"q": "a"}}},
						{FunctionCall: &genai.FunctionCall{Name: "memory_add", Args: map[string]any{"content": "b"}}},
					},
					Role: "model",
				},
				FinishReason: "STOP",
			}},
			UsageMetadata: &genai.GenerateContentResponseUsageMetadata{TotalTokenCount: 10},
		}, nil
	})
	c.registeredTools["memory_search"] = true
	c.registeredTools["memory_add"] = true

	result, err := c.Generate(context.Background(), []Part{TextPart{Text: "do both"}})
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	if len(result.FunctionCalls) != 2 {
		t.Fatalf("function calls=%d, want 2", len(result.FunctionCalls))
	}
	if result.FunctionCalls[0].Name != "memory_search" {
		t.Errorf("fc[0].name=%q, want memory_search", result.FunctionCalls[0].Name)
	}
	if result.FunctionCalls[1].Name != "memory_add" {
		t.Errorf("fc[1].name=%q, want memory_add", result.FunctionCalls[1].Name)
	}
}

// Task 2.5
func TestClient_Generate_UsageMetadata(t *testing.T) {
	c := newTestClient(func(_ context.Context, _ string, _ []*genai.Content, _ *genai.GenerateContentConfig) (*genai.GenerateContentResponse, error) {
		return mockTextResponse("ok"), nil
	})

	result, err := c.Generate(context.Background(), []Part{TextPart{Text: "hi"}})
	if err != nil {
		t.Fatal(err)
	}
	if result.Usage == nil {
		t.Fatal("expected non-nil usage")
	}
	if result.Usage.PromptTokens != 10 {
		t.Errorf("prompt=%d, want 10", result.Usage.PromptTokens)
	}
	if result.Usage.CandidateTokens != 5 {
		t.Errorf("candidate=%d, want 5", result.Usage.CandidateTokens)
	}
	if result.Usage.TotalTokens != 15 {
		t.Errorf("total=%d, want 15", result.Usage.TotalTokens)
	}
}

// Task 2.6
func TestClient_Generate_MissingUsage(t *testing.T) {
	c := newTestClient(func(_ context.Context, _ string, _ []*genai.Content, _ *genai.GenerateContentConfig) (*genai.GenerateContentResponse, error) {
		return &genai.GenerateContentResponse{
			Candidates: []*genai.Candidate{{
				Content:      &genai.Content{Parts: []*genai.Part{{Text: "ok"}}, Role: "model"},
				FinishReason: "STOP",
			}},
			// UsageMetadata intentionally nil
		}, nil
	})

	result, err := c.Generate(context.Background(), []Part{TextPart{Text: "hi"}})
	if err != nil {
		t.Fatal(err)
	}
	if result.Usage != MaxCostUsage {
		t.Errorf("expected MaxCostUsage sentinel, got %+v", result.Usage)
	}
}

// Task 2.7
func TestClient_Generate_Retry429(t *testing.T) {
	var attempts atomic.Int32
	c := newTestClient(func(_ context.Context, _ string, _ []*genai.Content, _ *genai.GenerateContentConfig) (*genai.GenerateContentResponse, error) {
		n := attempts.Add(1)
		if n <= 2 {
			return nil, &APIError{Code: 429, Message: "rate limited"}
		}
		return mockTextResponse("ok"), nil
	})

	result, err := c.Generate(context.Background(), []Part{TextPart{Text: "hi"}})
	if err != nil {
		t.Fatalf("expected success after retries, got: %v", err)
	}
	if attempts.Load() != 3 {
		t.Errorf("attempts=%d, want 3", attempts.Load())
	}
	if result.Text != "ok" {
		t.Errorf("text=%q, want ok", result.Text)
	}
}

// Task 2.8
func TestClient_Generate_RetryServerError(t *testing.T) {
	var attempts atomic.Int32
	c := newTestClient(func(_ context.Context, _ string, _ []*genai.Content, _ *genai.GenerateContentConfig) (*genai.GenerateContentResponse, error) {
		n := attempts.Add(1)
		if n == 1 {
			return nil, &APIError{Code: 500, Message: "internal error"}
		}
		return mockTextResponse("recovered"), nil
	})

	result, err := c.Generate(context.Background(), []Part{TextPart{Text: "hi"}})
	if err != nil {
		t.Fatalf("expected success after retry, got: %v", err)
	}
	if attempts.Load() != 2 {
		t.Errorf("attempts=%d, want 2", attempts.Load())
	}
	if result.Text != "recovered" {
		t.Errorf("text=%q, want recovered", result.Text)
	}
}

// Task 2.9
func TestClient_Generate_NoRetryClientError(t *testing.T) {
	var attempts atomic.Int32
	c := newTestClient(func(_ context.Context, _ string, _ []*genai.Content, _ *genai.GenerateContentConfig) (*genai.GenerateContentResponse, error) {
		attempts.Add(1)
		return nil, &APIError{Code: 400, Message: "bad request"}
	})

	_, err := c.Generate(context.Background(), []Part{TextPart{Text: "hi"}})
	if err == nil {
		t.Fatal("expected error")
	}
	if attempts.Load() != 1 {
		t.Errorf("attempts=%d, want 1 (no retry)", attempts.Load())
	}
}

// Task 2.10
func TestClient_Generate_Timeout(t *testing.T) {
	c := newTestClient(func(ctx context.Context, _ string, _ []*genai.Content, _ *genai.GenerateContentConfig) (*genai.GenerateContentResponse, error) {
		<-ctx.Done()
		return nil, ctx.Err()
	})
	c.timeout = 50 * time.Millisecond

	_, err := c.Generate(context.Background(), []Part{TextPart{Text: "hi"}})
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("expected DeadlineExceeded, got: %v", err)
	}
}

// Task 2.11
func TestClient_Generate_RejectsUnregisteredTool(t *testing.T) {
	c := newTestClient(func(_ context.Context, _ string, _ []*genai.Content, _ *genai.GenerateContentConfig) (*genai.GenerateContentResponse, error) {
		return mockFuncCallResponse("unknown_tool", map[string]any{}), nil
	})
	c.registeredTools["memory_search"] = true // only this is registered

	_, err := c.Generate(context.Background(), []Part{TextPart{Text: "hi"}})
	if err == nil {
		t.Fatal("expected error for unregistered tool")
	}
}

func TestClient_Generate_NoToolValidationWhenNoneRegistered(t *testing.T) {
	c := newTestClient(func(_ context.Context, _ string, _ []*genai.Content, _ *genai.GenerateContentConfig) (*genai.GenerateContentResponse, error) {
		return mockFuncCallResponse("any_tool", map[string]any{}), nil
	})
	// No registered tools — validation skipped

	result, err := c.Generate(context.Background(), []Part{TextPart{Text: "hi"}})
	if err != nil {
		t.Fatalf("expected no error when no tools registered, got: %v", err)
	}
	if len(result.FunctionCalls) != 1 {
		t.Fatalf("function calls=%d, want 1", len(result.FunctionCalls))
	}
}

func TestClient_Generate_500RetriedOnlyOnce(t *testing.T) {
	var attempts atomic.Int32
	c := newTestClient(func(_ context.Context, _ string, _ []*genai.Content, _ *genai.GenerateContentConfig) (*genai.GenerateContentResponse, error) {
		attempts.Add(1)
		return nil, &APIError{Code: 500, Message: "server error"}
	})

	_, err := c.Generate(context.Background(), []Part{TextPart{Text: "hi"}})
	if err == nil {
		t.Fatal("expected error after exhausting retries")
	}
	// 500 retries once: attempt 0 (fail) -> retry -> attempt 1 (fail, no more 500 retries)
	if attempts.Load() != 2 {
		t.Errorf("attempts=%d, want 2 (initial + 1 retry for 500)", attempts.Load())
	}
}

func TestClient_Generate_EmptyCandidates(t *testing.T) {
	c := newTestClient(func(_ context.Context, _ string, _ []*genai.Content, _ *genai.GenerateContentConfig) (*genai.GenerateContentResponse, error) {
		return &genai.GenerateContentResponse{
			Candidates:    []*genai.Candidate{},
			UsageMetadata: &genai.GenerateContentResponseUsageMetadata{TotalTokenCount: 5},
		}, nil
	})

	result, err := c.Generate(context.Background(), []Part{TextPart{Text: "hi"}})
	if err != nil {
		t.Fatal(err)
	}
	if result.Text != "" {
		t.Errorf("expected empty text, got %q", result.Text)
	}
}
