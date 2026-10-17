package app

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/tonylook/managents/helper/internal/agent"
	"github.com/tonylook/managents/helper/internal/detect"
	"github.com/tonylook/managents/helper/internal/protocol"
)

type fakeSource struct {
	name     string
	sessions []agent.Session
	err      error
}

func (s *fakeSource) Name() string { return s.name }
func (s *fakeSource) Sessions(context.Context, time.Time) ([]agent.Session, error) {
	return s.sessions, s.err
}

type fakeDisplays struct {
	connected   int
	discoveries int
	frames      []protocol.State
}

func (d *fakeDisplays) Discover(time.Time) { d.discoveries++ }
func (d *fakeDisplays) Count() int         { return d.connected }
func (d *fakeDisplays) Broadcast(line []byte) {
	var state protocol.State
	if err := json.Unmarshal(line, &state); err != nil {
		panic(err)
	}
	d.frames = append(d.frames, state)
}

type harness struct {
	now      time.Time
	source   *fakeSource
	displays *fakeDisplays
	runner   *Runner
}

func newHarness() *harness {
	h := &harness{
		now:      time.Date(2026, 10, 9, 12, 0, 0, 0, time.UTC),
		source:   &fakeSource{name: "fake"},
		displays: &fakeDisplays{connected: 1},
	}
	h.runner = NewRunner([]detect.Source{h.source}, h.displays, quietLogger())
	h.runner.Clock = func() time.Time { return h.now }
	h.runner.spawn = func(f func()) { f() } // discovery passes finish within the step
	return h
}

func quietLogger() *slog.Logger { return slog.New(slog.NewTextHandler(io.Discard, nil)) }

func (h *harness) step(advance time.Duration) {
	h.now = h.now.Add(advance)
	h.runner.Step(context.Background())
}

func working(id string) agent.Session {
	return agent.Session{ID: id, Kind: agent.KindClaude, Dir: "/w/" + id, Status: agent.StatusWorking}
}

func TestSendsImmediatelyThenKeepAlive(t *testing.T) {
	h := newHarness()
	h.source.sessions = []agent.Session{working("a")}

	h.step(0)
	h.step(time.Second)
	if len(h.displays.frames) != 1 {
		t.Fatalf("frames after 1s = %d, want 1 (no change, keep-alive not due)", len(h.displays.frames))
	}
	h.step(time.Second)
	if len(h.displays.frames) != 2 {
		t.Fatalf("frames after 2s = %d, want 2 (keep-alive)", len(h.displays.frames))
	}
}

func TestSendsOnChangeWithoutWaiting(t *testing.T) {
	h := newHarness()
	h.source.sessions = []agent.Session{working("a")}
	h.step(0)

	h.source.sessions = []agent.Session{working("a"), working("b")}
	h.step(time.Second)

	if len(h.displays.frames) != 2 || len(h.displays.frames[1].Agents) != 2 {
		t.Fatalf("frames = %+v, want a second frame with two agents", h.displays.frames)
	}
}

func TestNoFramesWithoutDisplays(t *testing.T) {
	h := newHarness()
	h.displays.connected = 0
	h.step(0)
	if len(h.displays.frames) != 0 || h.displays.discoveries != 1 {
		t.Errorf("frames=%d discoveries=%d", len(h.displays.frames), h.displays.discoveries)
	}
}

func TestNewDisplayGetsFrameAtOnce(t *testing.T) {
	h := newHarness()
	h.step(0)
	h.displays.connected = 0
	h.step(time.Second)
	h.displays.connected = 1
	h.step(500 * time.Millisecond)
	if len(h.displays.frames) != 2 {
		t.Errorf("frames = %d, want 2", len(h.displays.frames))
	}
}

func TestKeepAliveToleratesEarlyTicks(t *testing.T) {
	h := newHarness()
	h.source.sessions = []agent.Session{working("a")}

	for range 20 {
		h.step(999 * time.Millisecond) // a ticker may fire slightly early
	}

	if len(h.displays.frames) != 10 || h.displays.discoveries != 10 {
		t.Errorf("frames=%d discoveries=%d over 20 ticks, want 10 each (every second tick)",
			len(h.displays.frames), h.displays.discoveries)
	}
}

// slowDisplays is one connected display whose discovery passes block until
// the test releases them, like a pass probing a port that never answers.
type slowDisplays struct {
	release chan struct{}
	frames  int
}

func (d *slowDisplays) Discover(time.Time) { <-d.release }
func (d *slowDisplays) Count() int         { return 1 }
func (d *slowDisplays) Broadcast([]byte)   { d.frames++ }

// TestSlowDiscoveryDoesNotDelayFrames replays 70 s of slightly early ticks
// while every discovery pass takes the 3 s of a handshake that times out. The
// display gives up after 6 s without a frame, so the 2 s keep-alive rhythm
// must hold.
func TestSlowDiscoveryDoesNotDelayFrames(t *testing.T) {
	const passDuration, tick = 3 * time.Second, 999 * time.Millisecond
	displays := &slowDisplays{release: make(chan struct{})}
	defer close(displays.release)
	now := time.Date(2026, 10, 9, 12, 0, 0, 0, time.UTC)
	runner := NewRunner(nil, displays, quietLogger())
	runner.Clock = func() time.Time { return now }

	var passStart time.Time
	inFlight, passes := false, 0
	passDone := make(chan struct{}, 1) // lets a pass still running at the end exit
	spawn := runner.spawn              // NewRunner's: a Runner that waits for its passes must fail
	runner.spawn = func(pass func()) {
		if inFlight {
			t.Error("a discovery pass started while another was running")
		}
		inFlight, passStart = true, now
		passes++
		spawn(func() { pass(); passDone <- struct{}{} })
	}

	finished := make(chan time.Duration)
	go func() {
		var lastFrame time.Time
		var longestGap time.Duration
		for end := now.Add(70 * time.Second); now.Before(end); now = now.Add(tick) {
			sent := displays.frames
			runner.Step(context.Background())
			if displays.frames > sent {
				if !lastFrame.IsZero() {
					longestGap = max(longestGap, now.Sub(lastFrame))
				}
				lastFrame = now
			}
			if inFlight && now.Sub(passStart) >= passDuration {
				displays.release <- struct{}{}
				<-passDone
				inFlight = false
			}
		}
		finished <- longestGap
	}()

	select {
	case longestGap := <-finished:
		if longestGap > 2100*time.Millisecond {
			t.Errorf("longest gap between frames = %v, want about 2 s", longestGap)
		}
		if passes < 10 {
			t.Errorf("discovery passes = %d, want one starting soon after the previous one ends", passes)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("Step waited for a discovery pass to finish")
	}
}

func TestRunStopsWhenCancelled(t *testing.T) {
	h := newHarness()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	if err := h.runner.Run(ctx); err != nil {
		t.Fatal(err)
	}
	if len(h.displays.frames) != 1 {
		t.Errorf("frames = %d, want 1 (one step, then stop)", len(h.displays.frames))
	}
}

func TestFailingSourceStillSendsTheRest(t *testing.T) {
	h := newHarness()
	broken := &fakeSource{name: "broken", err: errors.New("boom")}
	h.runner.Sources = append(h.runner.Sources, broken)
	h.source.sessions = []agent.Session{working("a")}

	h.step(0)

	if len(h.displays.frames) != 1 || len(h.displays.frames[0].Agents) != 1 {
		t.Errorf("frames = %+v", h.displays.frames)
	}
}

func TestFingerprintIgnoresClockAndAges(t *testing.T) {
	base := protocol.State{V: 1, T: "state", Now: 100, Agents: []protocol.AgentEntry{{ID: "a", Status: "working", Age: 1}}}
	later := base
	later.Now = 200
	later.Agents = []protocol.AgentEntry{{ID: "a", Status: "working", Age: 101}}
	changed := base
	changed.Agents = []protocol.AgentEntry{{ID: "a", Status: "waiting", Age: 1}}

	if Fingerprint(base) != Fingerprint(later) {
		t.Error("clock/age ticks must not count as a change")
	}
	if Fingerprint(base) == Fingerprint(changed) {
		t.Error("a status change must change the fingerprint")
	}
	if base.Agents[0].Age != 1 {
		t.Error("Fingerprint modified its input")
	}
}
