package gemini

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
)

// ErrMaxIterations is returned when the tool loop exceeds its iteration cap.
var ErrMaxIterations = errors.New("gemini: tool loop exceeded maximum iterations")

// ToolExecutor executes a tool by name with JSON args.
// This is satisfied by executor.Executor.
type ToolExecutor interface {
	Execute(ctx context.Context, toolName string, args json.RawMessage) (any, error)
}

// LoopConfig configures the tool loop.
type LoopConfig struct {
	MaxIterations int // default: 10
}

// RunLoop executes the Gemini tool loop:
//  1. Call Generate with initial parts
//  2. If response is text, return it
//  3. If response is function call(s), execute them via executor
//  4. Send function responses back to Gemini
//  5. Repeat until text response or max iterations
func RunLoop(ctx context.Context, client *Client, executor ToolExecutor, parts []Part, cfg LoopConfig) (*GenerateContentResult, error) {
	if cfg.MaxIterations <= 0 {
		cfg.MaxIterations = 10
	}

	currentParts := parts

	for i := 0; i < cfg.MaxIterations; i++ {
		result, err := client.Generate(ctx, currentParts)
		if err != nil {
			return nil, fmt.Errorf("loop iteration %d: %w", i, err)
		}

		// Text response terminates the loop
		if result.Text != "" && len(result.FunctionCalls) == 0 {
			return result, nil
		}

		// No function calls and no text — unexpected, return what we have
		if len(result.FunctionCalls) == 0 {
			return result, nil
		}

		// Execute each function call and build response parts
		var responseParts []Part
		for _, fc := range result.FunctionCalls {
			argsJSON, _ := json.Marshal(fc.Args)
			toolResult, execErr := executor.Execute(ctx, fc.Name, argsJSON)

			response := make(map[string]any)
			if execErr != nil {
				response["error"] = execErr.Error()
			} else {
				response["result"] = toolResult
			}

			responseParts = append(responseParts, FunctionResponsePart{
				Name:     fc.Name,
				Response: response,
			})
		}

		currentParts = responseParts
	}

	return nil, ErrMaxIterations
}
