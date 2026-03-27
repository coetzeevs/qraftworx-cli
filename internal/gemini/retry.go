package gemini

import (
	"context"
	"errors"
	"fmt"
	"time"

	"google.golang.org/genai"
)

// APIError represents an HTTP error from the Gemini API.
type APIError struct {
	Code    int
	Message string
}

func (e *APIError) Error() string   { return fmt.Sprintf("gemini: HTTP %d: %s", e.Code, e.Message) }
func (e *APIError) StatusCode() int { return e.Code }

// statusCoder is implemented by errors that carry an HTTP status code.
type statusCoder interface {
	StatusCode() int
}

// classifyError determines if an error is retryable and the delay before retry.
// 429: retryable with exponential backoff, up to maxRetries.
// 500/503: retryable once with 2s delay.
// All other errors: not retryable.
func classifyError(err error, attempt int) (retry bool, delay time.Duration) {
	var sc statusCoder
	if !errors.As(err, &sc) {
		return false, 0
	}
	switch sc.StatusCode() {
	case 429:
		return true, time.Duration(1<<min(attempt, 30)) * time.Second
	case 500, 503:
		if attempt == 0 {
			return true, 2 * time.Second
		}
		return false, 0
	default:
		return false, 0
	}
}

// withRetry wraps a generate call with retry logic.
func withRetry(ctx context.Context, maxRetries int, sleepFn func(time.Duration), fn func(context.Context) (*genai.GenerateContentResponse, error)) (*genai.GenerateContentResponse, error) {
	var lastErr error
	for attempt := 0; attempt <= maxRetries; attempt++ {
		resp, err := fn(ctx)
		if err == nil {
			return resp, nil
		}
		lastErr = err

		if attempt == maxRetries {
			break
		}

		retry, delay := classifyError(err, attempt)
		if !retry {
			return nil, err
		}

		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
			sleepFn(delay)
		}
	}
	return nil, lastErr
}
