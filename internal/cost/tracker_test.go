package cost

import (
	"bytes"
	"log/slog"
	"math"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

// Task 4.7: PreCallGate allows under budget.

func TestTracker_PreCallGate_UnderBudget(t *testing.T) {
	dir := t.TempDir()
	counterPath := filepath.Join(dir, "counter.json")

	tracker := NewTracker(1.00, 0.8, counterPath)

	// Small estimate, well under budget
	err := tracker.PreCallGate(1000)
	if err != nil {
		t.Fatalf("expected nil error for under-budget call, got: %v", err)
	}
}

// Task 4.8: PreCallGate blocks over budget.

func TestTracker_PreCallGate_OverBudget(t *testing.T) {
	dir := t.TempDir()
	counterPath := filepath.Join(dir, "counter.json")

	// Very small budget: $0.0001
	tracker := NewTracker(0.0001, 0.8, counterPath)

	// First, record some usage to exhaust budget
	err := tracker.RecordUsage(1000, 1000)
	if err != nil {
		t.Fatalf("RecordUsage: %v", err)
	}

	// Now the gate should block
	err = tracker.PreCallGate(100000)
	if err != ErrBudgetExhausted {
		t.Fatalf("expected ErrBudgetExhausted, got: %v", err)
	}
}

func TestTracker_PreCallGate_ExactlyAtBudget(t *testing.T) {
	dir := t.TempDir()
	counterPath := filepath.Join(dir, "counter.json")

	// Budget that would be exactly equaled by the estimated tokens
	// estimatedCost = 1_000_000 * CandidatePricePerToken = 0.3
	tracker := NewTracker(0.3, 0.8, counterPath)

	// 0 + 0.3 == 0.3, not > 0.3, so should be allowed
	err := tracker.PreCallGate(1_000_000)
	if err != nil {
		t.Fatalf("expected nil (at budget, not over), got: %v", err)
	}
}

// Task 4.9: RecordUsage accumulates.

func TestTracker_RecordUsage_Accumulates(t *testing.T) {
	dir := t.TempDir()
	counterPath := filepath.Join(dir, "counter.json")

	tracker := NewTracker(10.0, 0.8, counterPath)

	promptTokens := 1000
	candidateTokens := 500

	// Record three times
	for i := 0; i < 3; i++ {
		if err := tracker.RecordUsage(promptTokens, candidateTokens); err != nil {
			t.Fatalf("RecordUsage call %d: %v", i, err)
		}
	}

	spend, err := tracker.TodaySpend()
	if err != nil {
		t.Fatalf("TodaySpend: %v", err)
	}

	singleCost := EstimateCost(promptTokens, candidateTokens)
	expectedSpend := singleCost * 3

	if math.Abs(spend-expectedSpend) > 0.000001 {
		t.Errorf("spend = %f, want %f", spend, expectedSpend)
	}
}

// Task 4.10: File-locked counter concurrent safety.

func TestTracker_FileLock_ConcurrentSafety(t *testing.T) {
	dir := t.TempDir()
	counterPath := filepath.Join(dir, "counter.json")

	tracker := NewTracker(100.0, 0.9, counterPath)

	promptTokens := 100
	candidateTokens := 50

	const goroutines = 20
	const callsPerGoroutine = 10

	var wg sync.WaitGroup
	wg.Add(goroutines)
	for i := 0; i < goroutines; i++ {
		go func() {
			defer wg.Done()
			for j := 0; j < callsPerGoroutine; j++ {
				if err := tracker.RecordUsage(promptTokens, candidateTokens); err != nil {
					t.Errorf("RecordUsage: %v", err)
				}
			}
		}()
	}
	wg.Wait()

	spend, err := tracker.TodaySpend()
	if err != nil {
		t.Fatalf("TodaySpend: %v", err)
	}

	singleCost := EstimateCost(promptTokens, candidateTokens)
	expectedTotal := singleCost * float64(goroutines*callsPerGoroutine)

	if math.Abs(spend-expectedTotal) > 0.000001 {
		t.Errorf("concurrent spend = %f, want %f (diff: %e)", spend, expectedTotal, math.Abs(spend-expectedTotal))
	}
}

// Task 4.11: Daily reset at midnight UTC.

func TestTracker_DailyReset(t *testing.T) {
	dir := t.TempDir()
	counterPath := filepath.Join(dir, "counter.json")

	// Set time to yesterday
	yesterday := time.Date(2026, 3, 26, 23, 0, 0, 0, time.UTC)
	origNow := nowFunc
	nowFunc = func() time.Time { return yesterday }
	defer func() { nowFunc = origNow }()

	tracker := NewTracker(10.0, 0.8, counterPath)

	// Record some usage "yesterday"
	if err := tracker.RecordUsage(10000, 5000); err != nil {
		t.Fatalf("RecordUsage yesterday: %v", err)
	}

	yesterdaySpend, err := tracker.TodaySpend()
	if err != nil {
		t.Fatalf("TodaySpend yesterday: %v", err)
	}
	if yesterdaySpend == 0 {
		t.Fatal("expected non-zero spend yesterday")
	}

	// Advance to today
	today := time.Date(2026, 3, 27, 1, 0, 0, 0, time.UTC)
	nowFunc = func() time.Time { return today }

	// Spend should be reset
	todaySpend, err := tracker.TodaySpend()
	if err != nil {
		t.Fatalf("TodaySpend today: %v", err)
	}
	if todaySpend != 0 {
		t.Errorf("expected zero spend after midnight reset, got %f", todaySpend)
	}
}

// Task 4.12: nil UsageMetadata = max cost (signaled by negative promptTokens).

func TestTracker_RecordUsage_NilUsage(t *testing.T) {
	dir := t.TempDir()
	counterPath := filepath.Join(dir, "counter.json")

	tracker := NewTracker(1000.0, 0.8, counterPath)

	// Signal absent usage with negative promptTokens
	if err := tracker.RecordUsage(-1, 0); err != nil {
		t.Fatalf("RecordUsage(-1, 0): %v", err)
	}

	spend, err := tracker.TodaySpend()
	if err != nil {
		t.Fatalf("TodaySpend: %v", err)
	}

	maxCost := EstimateCost(MaxCostPromptTokens, MaxCostCandidateTokens)
	if math.Abs(spend-maxCost) > 0.000001 {
		t.Errorf("spend = %f, want maxCost %f", spend, maxCost)
	}

	// Verify it's a significant amount (not zero)
	if spend < 0.1 {
		t.Errorf("max cost should be substantial, got %f", spend)
	}
}

// Task 4.13: Warn threshold logging.

func TestTracker_WarnThreshold(t *testing.T) {
	dir := t.TempDir()
	counterPath := filepath.Join(dir, "counter.json")

	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug}))

	// Budget $0.001, warn at 50%
	tracker := NewTracker(0.001, 0.5, counterPath)
	tracker.SetLogger(logger)

	// Record enough usage to exceed warn threshold
	// Candidate cost: 10000 * 0.0000003 = 0.003, which is > 0.001 * 0.5 = 0.0005
	if err := tracker.RecordUsage(5000, 10000); err != nil {
		t.Fatalf("RecordUsage: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "daily spend approaching budget") {
		t.Errorf("expected warning about budget, got: %s", output)
	}
}

func TestTracker_WarnThreshold_NoWarningBelowThreshold(t *testing.T) {
	dir := t.TempDir()
	counterPath := filepath.Join(dir, "counter.json")

	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug}))

	// Budget $100, warn at 80%
	tracker := NewTracker(100.0, 0.8, counterPath)
	tracker.SetLogger(logger)

	// Very small usage, well under threshold
	if err := tracker.RecordUsage(10, 5); err != nil {
		t.Fatalf("RecordUsage: %v", err)
	}

	output := buf.String()
	if strings.Contains(output, "daily spend approaching budget") {
		t.Errorf("did not expect warning below threshold, got: %s", output)
	}
}

func TestTracker_TodaySpend_NoFile(t *testing.T) {
	dir := t.TempDir()
	counterPath := filepath.Join(dir, "nonexistent.json")

	tracker := NewTracker(10.0, 0.8, counterPath)

	spend, err := tracker.TodaySpend()
	if err != nil {
		t.Fatalf("TodaySpend: %v", err)
	}
	if spend != 0 {
		t.Errorf("expected zero spend for missing file, got %f", spend)
	}
}

func TestTracker_EstimateCost(t *testing.T) {
	// 1M * 0.000000075 + 1M * 0.0000003 = 0.075 + 0.3 = 0.375
	cost := EstimateCost(1_000_000, 1_000_000)
	expected := 0.375
	if math.Abs(cost-expected) > 0.001 {
		t.Errorf("EstimateCost = %f, want ~%f", cost, expected)
	}
}
