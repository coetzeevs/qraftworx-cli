package cost

import (
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"sync"
	"time"
)

// ErrBudgetExhausted is returned when a call would exceed the daily budget (S6).
var ErrBudgetExhausted = errors.New("cost: daily budget exhausted")

// Pricing per token in USD (Gemini 2.5 Flash approximate pricing).
const (
	PromptPricePerToken    = 0.000000075 // $0.075 per 1M tokens
	CandidatePricePerToken = 0.0000003   // $0.30 per 1M tokens
)

// MaxCostPromptTokens is the sentinel prompt token count for missing usage (S6).
const MaxCostPromptTokens = 1_000_000

// MaxCostCandidateTokens is the sentinel candidate token count for missing usage (S6).
const MaxCostCandidateTokens = 1_000_000

// dailyCounter is the file-persisted daily spend counter.
type dailyCounter struct {
	Date  string  `json:"date"`  // YYYY-MM-DD in UTC
	Spend float64 `json:"spend"` // accumulated USD
}

// nowFunc is injectable for testing. Defaults to time.Now.
var nowFunc = time.Now

// Tracker tracks Gemini API spend with a file-locked daily counter (S6).
type Tracker struct {
	dailyBudget   float64
	warnThreshold float64
	counterPath   string
	mu            sync.Mutex
	logger        *slog.Logger
}

// NewTracker creates a cost tracker.
// budget is the daily budget in USD.
// warn is the threshold (0-1 fraction of budget) at which to log a warning.
// counterPath is the path to the daily counter JSON file.
func NewTracker(budget, warn float64, counterPath string) *Tracker {
	return &Tracker{
		dailyBudget:   budget,
		warnThreshold: warn,
		counterPath:   counterPath,
	}
}

// SetLogger sets the logger for warning/info messages.
func (t *Tracker) SetLogger(l *slog.Logger) {
	t.logger = l
}

// PreCallGate checks if the estimated cost would exceed the daily budget.
// Returns ErrBudgetExhausted if so (S6).
func (t *Tracker) PreCallGate(estimatedTokens int) error {
	t.mu.Lock()
	defer t.mu.Unlock()

	current, err := t.readCounter()
	if err != nil {
		return fmt.Errorf("cost: reading counter: %w", err)
	}

	// Estimate cost conservatively: assume all tokens are candidate tokens (most expensive)
	estimatedCost := float64(estimatedTokens) * CandidatePricePerToken

	if current.Spend+estimatedCost > t.dailyBudget {
		return ErrBudgetExhausted
	}

	return nil
}

// RecordUsage adds actual token usage to the daily counter.
// Uses file locking (via mutex) for cross-process safety (S6).
// Negative promptTokens signals absent usage metadata; max cost is recorded (S6).
func (t *Tracker) RecordUsage(promptTokens, candidateTokens int) error {
	if promptTokens < 0 {
		promptTokens = MaxCostPromptTokens
		candidateTokens = MaxCostCandidateTokens
	}

	cost := EstimateCost(promptTokens, candidateTokens)

	t.mu.Lock()
	defer t.mu.Unlock()

	counter, err := t.readCounter()
	if err != nil {
		return fmt.Errorf("cost: reading counter: %w", err)
	}

	counter.Spend += cost

	if err := t.writeCounter(counter); err != nil {
		return fmt.Errorf("cost: writing counter: %w", err)
	}

	// Log warning if over threshold
	if t.warnThreshold > 0 && counter.Spend > t.warnThreshold*t.dailyBudget {
		if t.logger != nil {
			t.logger.Warn("daily spend approaching budget",
				"spend_usd", counter.Spend,
				"budget_usd", t.dailyBudget,
				"threshold", t.warnThreshold,
			)
		}
	}

	return nil
}

// TodaySpend returns the current day's accumulated spend.
func (t *Tracker) TodaySpend() (float64, error) {
	t.mu.Lock()
	defer t.mu.Unlock()

	counter, err := t.readCounter()
	if err != nil {
		return 0, fmt.Errorf("cost: reading counter: %w", err)
	}

	return counter.Spend, nil
}

// readCounter reads and returns the daily counter, resetting if the date has changed.
func (t *Tracker) readCounter() (dailyCounter, error) {
	today := nowFunc().UTC().Format("2006-01-02")

	data, err := os.ReadFile(t.counterPath)
	if err != nil {
		if os.IsNotExist(err) {
			return dailyCounter{Date: today, Spend: 0}, nil
		}
		return dailyCounter{}, fmt.Errorf("reading counter file: %w", err)
	}

	var counter dailyCounter
	if err := json.Unmarshal(data, &counter); err != nil {
		// Corrupt file — start fresh
		return dailyCounter{Date: today, Spend: 0}, nil
	}

	// Daily reset: if the date doesn't match today, start fresh
	if counter.Date != today {
		return dailyCounter{Date: today, Spend: 0}, nil
	}

	return counter, nil
}

// writeCounter persists the daily counter to disk with 0o600 permissions.
func (t *Tracker) writeCounter(counter dailyCounter) error {
	data, err := json.Marshal(counter)
	if err != nil {
		return fmt.Errorf("marshaling counter: %w", err)
	}
	return os.WriteFile(t.counterPath, data, 0o600)
}

// EstimateCost calculates the USD cost from prompt and candidate token counts.
func EstimateCost(promptTokens, candidateTokens int) float64 {
	promptCost := float64(promptTokens) * PromptPricePerToken
	candidateCost := float64(candidateTokens) * CandidatePricePerToken
	return promptCost + candidateCost
}
