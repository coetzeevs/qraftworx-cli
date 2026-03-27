package gemini

import (
	"context"
	"errors"
	"fmt"
	"os"
	"time"

	"google.golang.org/genai"
)

// Part represents a content part in a prompt.
type Part interface {
	toGenaiPart() *genai.Part
}

// TextPart is a text content part.
type TextPart struct{ Text string }

func (p TextPart) toGenaiPart() *genai.Part {
	return &genai.Part{Text: p.Text}
}

// FunctionResponsePart sends a tool result back to Gemini.
type FunctionResponsePart struct {
	Name     string
	Response map[string]any
}

func (p FunctionResponsePart) toGenaiPart() *genai.Part {
	return &genai.Part{
		FunctionResponse: &genai.FunctionResponse{
			Name:     p.Name,
			Response: p.Response,
		},
	}
}

// GenerateContentResult wraps the API response for Qraft's needs.
type GenerateContentResult struct {
	Text          string
	FunctionCalls []FunctionCall
	Usage         *UsageMetadata
	FinishReason  string
}

// FunctionCall represents a single tool invocation requested by Gemini.
type FunctionCall struct {
	Name string
	Args map[string]any
}

// UsageMetadata tracks token usage for cost accounting.
type UsageMetadata struct {
	PromptTokens    int
	CandidateTokens int
	TotalTokens     int
}

// MaxCostUsage is a sentinel for missing usage metadata (S6).
// Treat absent usageMetadata as maximum possible cost.
var MaxCostUsage = &UsageMetadata{
	PromptTokens:    1_000_000,
	CandidateTokens: 1_000_000,
	TotalTokens:     2_000_000,
}

// generateFunc is the signature for the underlying API call.
type generateFunc func(ctx context.Context, model string, contents []*genai.Content, config *genai.GenerateContentConfig) (*genai.GenerateContentResponse, error)

// Client wraps the Gemini API for GenerateContent calls with tool support.
type Client struct {
	model           string
	registeredTools map[string]bool
	maxRetries      int
	timeout         time.Duration
	tools           []*genai.Tool
	generateFn      generateFunc
	sleepFn         func(time.Duration) // time.Sleep in production, injectable for tests
}

// Option configures the Client.
type Option func(*Client)

// WithTimeout sets the per-request timeout. Default: 30s.
func WithTimeout(d time.Duration) Option {
	return func(c *Client) { c.timeout = d }
}

// WithMaxRetries sets the maximum retry count. Default: 3.
func WithMaxRetries(n int) Option {
	return func(c *Client) { c.maxRetries = n }
}

// WithTools sets the Gemini tool declarations.
func WithTools(tools []*genai.Tool) Option {
	return func(c *Client) { c.tools = tools }
}

// WithRegisteredToolNames registers tool names for validation.
// Generate rejects function calls naming unregistered tools.
func WithRegisteredToolNames(names []string) Option {
	return func(c *Client) {
		for _, n := range names {
			c.registeredTools[n] = true
		}
	}
}

// NewClient creates a Gemini client. Reads GEMINI_API_KEY from env.
// Returns error if key is missing.
func NewClient(model string, opts ...Option) (*Client, error) {
	apiKey := os.Getenv("GEMINI_API_KEY")
	if apiKey == "" {
		return nil, errors.New("gemini: GEMINI_API_KEY environment variable not set")
	}

	gc, err := genai.NewClient(context.Background(), &genai.ClientConfig{
		APIKey:  apiKey,
		Backend: genai.BackendGeminiAPI,
	})
	if err != nil {
		return nil, fmt.Errorf("gemini: creating client: %w", err)
	}

	c := &Client{
		model:           model,
		registeredTools: make(map[string]bool),
		maxRetries:      3,
		timeout:         30 * time.Second,
		generateFn:      gc.Models.GenerateContent,
		sleepFn:         time.Sleep,
	}
	for _, opt := range opts {
		opt(c)
	}
	return c, nil
}

// Generate sends a prompt to Gemini and returns the result.
// Handles retries on 429/500/503 with exponential backoff.
// Enforces context timeout.
func (c *Client) Generate(ctx context.Context, parts []Part) (*GenerateContentResult, error) {
	genaiParts := make([]*genai.Part, len(parts))
	for i, p := range parts {
		genaiParts[i] = p.toGenaiPart()
	}
	contents := []*genai.Content{{Parts: genaiParts, Role: "user"}}

	var config *genai.GenerateContentConfig
	if len(c.tools) > 0 {
		config = &genai.GenerateContentConfig{Tools: c.tools}
	}

	ctx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()

	resp, err := withRetry(ctx, c.maxRetries, c.sleepFn, func(ctx context.Context) (*genai.GenerateContentResponse, error) {
		return c.generateFn(ctx, c.model, contents, config)
	})
	if err != nil {
		return nil, err
	}

	return c.parseResponse(resp)
}

func (c *Client) parseResponse(resp *genai.GenerateContentResponse) (*GenerateContentResult, error) {
	result := &GenerateContentResult{}

	// S6: missing usage metadata = max cost
	if resp.UsageMetadata != nil {
		result.Usage = &UsageMetadata{
			PromptTokens:    int(resp.UsageMetadata.PromptTokenCount),
			CandidateTokens: int(resp.UsageMetadata.CandidatesTokenCount),
			TotalTokens:     int(resp.UsageMetadata.TotalTokenCount),
		}
	} else {
		result.Usage = MaxCostUsage
	}

	if len(resp.Candidates) == 0 {
		return result, nil
	}

	candidate := resp.Candidates[0]
	result.FinishReason = string(candidate.FinishReason)

	if candidate.Content == nil {
		return result, nil
	}

	for _, part := range candidate.Content.Parts {
		if part.Text != "" {
			result.Text += part.Text
		}
		if part.FunctionCall != nil {
			if len(c.registeredTools) > 0 && !c.registeredTools[part.FunctionCall.Name] {
				return nil, fmt.Errorf("gemini: model requested unregistered tool %q", part.FunctionCall.Name)
			}
			result.FunctionCalls = append(result.FunctionCalls, FunctionCall{
				Name: part.FunctionCall.Name,
				Args: part.FunctionCall.Args,
			})
		}
	}

	return result, nil
}
