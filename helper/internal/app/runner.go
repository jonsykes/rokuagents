// Package app is the helper's use case: keep every connected display in sync
// with the agent sessions open on this computer.
package app

import (
	"context"
	"log/slog"
	"sync/atomic"
	"time"

	"github.com/tonylook/managents/helper/internal/agent"
	"github.com/tonylook/managents/helper/internal/detect"
	"github.com/tonylook/managents/helper/internal/protocol"
)

// Displays is where frames go (implemented by device.Manager). Discover may
// take seconds and runs concurrently with Broadcast and Count, but never
// concurrently with itself.
type Displays interface {
	Discover(now time.Time)
	Broadcast(line []byte)
	Count() int
}

// Runner polls the sources and streams state frames to the displays.
type Runner struct {
	Sources  []detect.Source
	Displays Displays
	Logger   *slog.Logger
	Clock    func() time.Time

	// PollInterval is how often sessions are detected; a change is sent at the
	// next poll. KeepAlive is the longest gap between two frames (the display
	// gives up after 6 s of silence). DiscoverInterval paces port scanning.
	PollInterval     time.Duration
	KeepAlive        time.Duration
	DiscoverInterval time.Duration

	lastSent         time.Time
	lastFingerprint  string
	lastDiscovery    time.Time
	lastErrorMessage string

	// spawn starts a discovery pass in the background; discovering is set
	// while one runs, so that passes never overlap.
	spawn       func(func())
	discovering atomic.Bool
}

// NewRunner returns a Runner with the protocol's default timings.
func NewRunner(sources []detect.Source, displays Displays, logger *slog.Logger) *Runner {
	return &Runner{
		Sources:          sources,
		Displays:         displays,
		Logger:           logger,
		Clock:            time.Now,
		PollInterval:     time.Second,
		KeepAlive:        2 * time.Second,
		DiscoverInterval: 2 * time.Second,
		spawn:            func(f func()) { go f() },
	}
}

// Run loops until ctx is cancelled.
func (r *Runner) Run(ctx context.Context) error {
	ticker := time.NewTicker(r.PollInterval)
	defer ticker.Stop()
	for {
		r.Step(ctx)
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
		}
	}
}

// Step runs one poll: start a discovery pass if due, detect sessions, and send
// a frame if something changed or the keep-alive is due. Discovery runs in the
// background because probing a port that never answers takes seconds, and the
// connected displays must keep getting frames meanwhile.
func (r *Runner) Step(ctx context.Context) {
	now := r.Clock()
	if r.due(now, r.lastDiscovery, r.DiscoverInterval) && r.discovering.CompareAndSwap(false, true) {
		r.lastDiscovery = now
		r.spawn(func() {
			defer r.discovering.Store(false)
			r.Displays.Discover(now)
		})
	}
	if r.Displays.Count() == 0 {
		r.lastSent = time.Time{} // a newly connected display gets a frame at once
		return
	}

	state := r.Snapshot(ctx, now)
	fingerprint := Fingerprint(state)
	if fingerprint == r.lastFingerprint && !r.due(now, r.lastSent, r.KeepAlive) {
		return
	}
	line, err := protocol.Encode(state)
	if err != nil {
		r.Logger.Error("encoding state failed", "err", err)
		return
	}
	r.Displays.Broadcast(line)
	r.lastSent = now
	r.lastFingerprint = fingerprint
}

// due reports whether something last done at last is due again at now. Half a
// poll of slack absorbs ticker jitter: a tick that fires a little early must
// not postpone the action by a whole poll.
func (r *Runner) due(now, last time.Time, every time.Duration) bool {
	return last.IsZero() || now.Sub(last) >= every-r.PollInterval/2
}

// Snapshot detects the open sessions and builds the frame for them. Sources
// that fail are logged (once per distinct error) and skipped.
func (r *Runner) Snapshot(ctx context.Context, now time.Time) protocol.State {
	sessions, err := detect.All(ctx, now, r.Sources)
	r.reportDetectionError(err)
	return protocol.NewState(agent.Arrange(sessions), now)
}

func (r *Runner) reportDetectionError(err error) {
	message := ""
	if err != nil {
		message = err.Error()
	}
	if message != r.lastErrorMessage && message != "" {
		r.Logger.Warn("session detection failed", "err", err)
	}
	r.lastErrorMessage = message
}
