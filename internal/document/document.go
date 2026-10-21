package document

import (
	"errors"
	"fmt"
)

type Limits struct {
	MaxActions   int
	MaxXLSXCells int
}

var (
	// ErrInvalid marks errors caused by the request contents.
	ErrInvalid = errors.New("invalid request")
	// ErrLimit marks requests that exceed a configured resource limit.
	ErrLimit = errors.New("limit exceeded")

	errUnknownAction = errors.New("unknown action type")
)

type ActionError struct {
	Index int
	Type  string
	Err   error
}

func (e *ActionError) Error() string {
	return fmt.Sprintf("action %d (%s): %v", e.Index, e.Type, e.Err)
}

func (e *ActionError) Unwrap() []error {
	return []error{ErrInvalid, e.Err}
}

func checkActionCount(n int, limits Limits) error {
	if n > limits.MaxActions {
		return fmt.Errorf("%w: %d actions, at most %d allowed", ErrLimit, n, limits.MaxActions)
	}
	return nil
}

func valueOr(value, fallback string) string {
	if value == "" {
		return fallback
	}
	return value
}
