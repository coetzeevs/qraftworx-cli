package gemini

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/coetzeevs/qraftworx-cli/internal/cost"
	"github.com/coetzeevs/qraftworx-cli/internal/logging"
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
	CostTracker   *cost.Tracker
	Logger        *slog.Logger
}

// RunLoop executes the Gemini tool loop:
//  1. Call PreCallGate (if cost tracker is set) before Generate
//  2. Call Generate with initial parts
//  3. If response is text, return it
//  4. If response is function call(s), execute them via executor
//  5. Send function responses back to Gemini
//  6. Repeat until text response or max iterations
//  7. Record usage after each Generate (if cost tracker is set)
//  8. Log InteractionLog for every interaction
func RunLoop(ctx context.Context, client *Client, executor ToolExecutor, parts []Part, cfg LoopConfig) (*GenerateContentResult, error) {
	if cfg.MaxIterations <= 0 {
		cfg.MaxIterations = 10
	}

	totalStart := time.Now()
	currentParts := parts
	var allToolCalls []logging.ToolCallLog
	var totalTokensSent, totalTokensRecvd int
	var totalCostUSD float64
	var userPrompt string

	// Extract user prompt from the first text part for logging
	for _, p := range parts {
		if tp, ok := p.(TextPart); ok {
			userPrompt = tp.Text
			break
		}
	}

	for i := 0; i < cfg.MaxIterations; i++ {
		// S6: Pre-call budget gate
		if cfg.CostTracker != nil {
			if err := cfg.CostTracker.PreCallGate(100_000); err != nil {
				logInteraction(cfg.Logger, userPrompt, allToolCalls, totalTokensSent, totalTokensRecvd, totalCostUSD, totalStart, err)
				return nil, fmt.Errorf("loop iteration %d: %w", i, err)
			}
		}

		result, err := client.Generate(ctx, currentParts)
		if err != nil {
			logInteraction(cfg.Logger, userPrompt, allToolCalls, totalTokensSent, totalTokensRecvd, totalCostUSD, totalStart, err)
			return nil, fmt.Errorf("loop iteration %d: %w", i, err)
		}

		// Track tokens from this iteration
		promptTok := 0
		candidateTok := 0
		if result.Usage != nil {
			promptTok = result.Usage.PromptTokens
			candidateTok = result.Usage.CandidateTokens
			totalTokensSent += promptTok
			totalTokensRecvd += candidateTok
		}

		// Record usage after Generate
		if cfg.CostTracker != nil {
			if result.Usage != nil {
				if recordErr := cfg.CostTracker.RecordUsage(promptTok, candidateTok); recordErr != nil {
					if cfg.Logger != nil {
						cfg.Logger.Error("failed to record usage", "error", recordErr)
					}
				}
				totalCostUSD += cost.EstimateCost(promptTok, candidateTok)
			} else {
				// nil usage -> signal max cost with negative promptTokens
				if recordErr := cfg.CostTracker.RecordUsage(-1, 0); recordErr != nil {
					if cfg.Logger != nil {
						cfg.Logger.Error("failed to record usage", "error", recordErr)
					}
				}
				totalCostUSD += cost.EstimateCost(cost.MaxCostPromptTokens, cost.MaxCostCandidateTokens)
			}
		}

		// Text response terminates the loop
		if result.Text != "" && len(result.FunctionCalls) == 0 {
			logInteraction(cfg.Logger, userPrompt, allToolCalls, totalTokensSent, totalTokensRecvd, totalCostUSD, totalStart, nil)
			return result, nil
		}

		// No function calls and no text — unexpected, return what we have
		if len(result.FunctionCalls) == 0 {
			logInteraction(cfg.Logger, userPrompt, allToolCalls, totalTokensSent, totalTokensRecvd, totalCostUSD, totalStart, nil)
			return result, nil
		}

		// Execute each function call and build response parts
		var responseParts []Part
		for _, fc := range result.FunctionCalls {
			toolStart := time.Now()
			argsJSON, _ := json.Marshal(fc.Args)
			toolResult, execErr := executor.Execute(ctx, fc.Name, argsJSON)
			toolDuration := time.Since(toolStart)

			tcLog := logging.ToolCallLog{
				Name:     fc.Name,
				Summary:  summarizeToolCall(fc.Name, fc.Args),
				Duration: toolDuration,
			}
			if execErr != nil {
				tcLog.Error = execErr.Error()
			}
			allToolCalls = append(allToolCalls, tcLog)

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

	logInteraction(cfg.Logger, userPrompt, allToolCalls, totalTokensSent, totalTokensRecvd, totalCostUSD, totalStart, ErrMaxIterations)
	return nil, ErrMaxIterations
}

// logInteraction logs a structured InteractionLog if a logger is configured.
func logInteraction(logger *slog.Logger, prompt string, toolCalls []logging.ToolCallLog, tokensSent, tokensRecvd int, costUSD float64, start time.Time, loopErr error) {
	if logger == nil {
		return
	}

	errMsg := ""
	if loopErr != nil {
		errMsg = loopErr.Error()
	}

	logger.Info("interaction",
		"user_prompt", prompt,
		"tokens_sent", tokensSent,
		"tokens_received", tokensRecvd,
		"tool_calls_count", len(toolCalls),
		"total_latency_ms", time.Since(start).Milliseconds(),
		"cost_usd", costUSD,
		"error", errMsg,
	)
}

// summarizeToolCall creates a sanitized summary of a tool call (S4).
// Uses tool name + arg keys only, never raw values.
func summarizeToolCall(name string, args map[string]any) string {
	if len(args) == 0 {
		return fmt.Sprintf("called %s with no arguments", name)
	}
	keys := make([]string, 0, len(args))
	for k := range args {
		keys = append(keys, k)
	}
	return fmt.Sprintf("called %s with keys: %v", name, keys)
}
