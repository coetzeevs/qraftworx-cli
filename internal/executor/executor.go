package executor

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/coetzeevs/qraftworx-cli/internal/tools"
)

var (
	// ErrToolNotFound is returned when a requested tool is not in the registry.
	ErrToolNotFound = errors.New("executor: tool not found")

	// ErrNonInteractiveDenied is returned when a confirmation-required tool
	// is executed in a non-TTY environment (S1).
	ErrNonInteractiveDenied = errors.New("executor: confirmation-required tool denied in non-interactive mode")

	// ErrUserDenied is returned when the user declines confirmation.
	ErrUserDenied = errors.New("executor: user denied tool execution")

	// ErrTooManyErrors is returned when the per-interaction error cap is reached.
	ErrTooManyErrors = errors.New("executor: too many tool errors in this interaction")
)

// ConfirmFunc asks the user to confirm an action.
type ConfirmFunc func(toolName string, summary string) (bool, error)

// Executor runs tools with confirmation gates and permission checks.
type Executor struct {
	registry  *tools.Registry
	confirmFn ConfirmFunc
	isTTY     bool
	maxErrors int
	errCount  int
}

// Option configures the Executor.
type Option func(*Executor)

// WithConfirmFunc sets the confirmation function. Defaults to nil (auto-approve for no-confirm tools).
func WithConfirmFunc(fn ConfirmFunc) Option {
	return func(e *Executor) { e.confirmFn = fn }
}

// WithTTY sets whether the executor is running in a TTY.
func WithTTY(isTTY bool) Option {
	return func(e *Executor) { e.isTTY = isTTY }
}

// WithMaxErrors sets the per-interaction error cap.
func WithMaxErrors(n int) Option {
	return func(e *Executor) { e.maxErrors = n }
}

// New creates an Executor with the given registry and options.
func New(registry *tools.Registry, opts ...Option) *Executor {
	e := &Executor{
		registry:  registry,
		maxErrors: 3,
	}
	for _, opt := range opts {
		opt(e)
	}
	return e
}

// ResetErrors resets the per-interaction error counter.
func (e *Executor) ResetErrors() {
	e.errCount = 0
}

// Execute runs a tool by name with the given args.
func (e *Executor) Execute(ctx context.Context, toolName string, args json.RawMessage) (result any, err error) {
	if e.errCount >= e.maxErrors {
		return nil, ErrTooManyErrors
	}

	tool, ok := e.registry.Get(toolName)
	if !ok {
		e.errCount++
		return nil, fmt.Errorf("%w: %q", ErrToolNotFound, toolName)
	}

	// Panic recovery
	defer func() {
		if r := recover(); r != nil {
			e.errCount++
			err = fmt.Errorf("executor: tool %q panicked: %v", toolName, r)
			result = nil
		}
	}()

	// Confirmation gate (S1)
	if tool.RequiresConfirmation() {
		if !e.isTTY {
			e.errCount++
			return nil, ErrNonInteractiveDenied
		}
		if e.confirmFn != nil {
			summary := stripControlChars(string(args))
			approved, confirmErr := e.confirmFn(toolName, summary)
			if confirmErr != nil {
				e.errCount++
				return nil, fmt.Errorf("executor: confirmation error: %w", confirmErr)
			}
			if !approved {
				return nil, ErrUserDenied
			}
		}
	}

	result, err = tool.Execute(ctx, args)
	if err != nil {
		e.errCount++
	}
	return result, err
}
