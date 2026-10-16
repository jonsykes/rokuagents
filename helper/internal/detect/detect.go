// Package detect finds the agent sessions open on this computer.
//
// Each agent product has its own Source (see the claude, opencode and codex
// subpackages). Sources only read session metadata — status, working
// directory, timestamps, token counts — never conversation content.
package detect

import (
	"context"
	"errors"
	"time"

	"github.com/tonylook/managents/helper/internal/agent"
)

// Source detects the open sessions of one agent product.
type Source interface {
	Name() string
	Sessions(ctx context.Context, now time.Time) ([]agent.Session, error)
}

// All queries every source and merges the results. A failing source does not
// hide the others: its error is returned alongside whatever was found.
func All(ctx context.Context, now time.Time, sources []Source) ([]agent.Session, error) {
	var sessions []agent.Session
	var errs []error
	for _, source := range sources {
		found, err := source.Sessions(ctx, now)
		if err != nil {
			errs = append(errs, &SourceError{Source: source.Name(), Err: err})
		}
		sessions = append(sessions, found...)
	}
	return sessions, errors.Join(errs...)
}

// SourceError attributes a detection failure to its source.
type SourceError struct {
	Source string
	Err    error
}

func (e *SourceError) Error() string { return e.Source + ": " + e.Err.Error() }
func (e *SourceError) Unwrap() error { return e.Err }
