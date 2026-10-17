package demo

import (
	"bytes"
	"context"
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/tonylook/managents/helper/internal/agent"
	"github.com/tonylook/managents/helper/internal/protocol"
)

var start = time.Date(2026, 10, 9, 12, 0, 0, 0, time.UTC)

func TestSourceCyclesScenes(t *testing.T) {
	source := &Source{Start: start, Hold: 6 * time.Second}
	scenes := Scenes(start)
	tests := []struct {
		at        time.Duration
		wantScene int
	}{
		{-time.Second, 0}, // a clock that stepped back
		{0, 0},
		{5 * time.Second, 0},
		{6 * time.Second, 1},
		{time.Duration(len(scenes)) * 6 * time.Second, 0},
		{time.Duration(len(scenes)+2)*6*time.Second + time.Second, 2},
	}
	for _, tt := range tests {
		scene, n := source.SceneAt(start.Add(tt.at))
		if n%len(scenes) != tt.wantScene || scene.Title != scenes[tt.wantScene].Title {
			t.Errorf("at %+v: scene %d %q, want %d %q", tt.at, n, scene.Title, tt.wantScene, scenes[tt.wantScene].Title)
		}
	}
}

func TestAgesAdvanceWithinAScene(t *testing.T) {
	source := &Source{Start: start, Hold: time.Minute}
	at := func(d time.Duration) time.Duration {
		sessions, err := source.Sessions(context.Background(), start.Add(d))
		if err != nil {
			t.Fatal(err)
		}
		return sessions[0].Age(start.Add(d))
	}
	if early, late := at(time.Second), at(30*time.Second); late-early != 29*time.Second {
		t.Errorf("age went from %v to %v, want 29 s more", early, late)
	}
	if next := at(time.Minute + time.Second); next != at(time.Second) {
		t.Errorf("next scene starts at age %v, want the same as the first one", next)
	}
}

func TestSourceLogsEachSceneOnce(t *testing.T) {
	var log bytes.Buffer
	source := &Source{Start: start, Hold: 6 * time.Second, Logger: slog.New(slog.NewTextHandler(&log, nil))}
	for d := time.Duration(0); d < 12*time.Second; d += time.Second {
		if _, err := source.Sessions(context.Background(), start.Add(d)); err != nil {
			t.Fatal(err)
		}
	}
	if got := strings.Count(log.String(), "msg=showing"); got != 2 {
		t.Errorf("%d scene lines in 2 scenes:\n%s", got, log.String())
	}
}

func TestEverySceneFitsTheProtocol(t *testing.T) {
	for _, scene := range Scenes(start) {
		cards := agent.Arrange(scene.Sessions)
		state := protocol.NewState(cards, start)
		if _, err := protocol.Encode(state); err != nil {
			t.Errorf("%s: %v", scene.Title, err)
		}
		if len(state.Agents)+state.More != len(cards) {
			t.Errorf("%s: %d agents + %d more, want %d", scene.Title, len(state.Agents), state.More, len(cards))
		}
		ids := make(map[string]bool)
		for _, s := range scene.Sessions {
			if ids[s.ID] {
				t.Errorf("%s: id %s used twice", scene.Title, s.ID)
			}
			ids[s.ID] = true
		}
	}
}

// TestScenesCoverTheDisplay checks that the demo shows what it is for: every
// status and agent, contexts with and without a known limit, an empty screen,
// full pages and a second page.
func TestScenesCoverTheDisplay(t *testing.T) {
	statuses := make(map[agent.Status]bool)
	kinds := make(map[agent.Kind]bool)
	counts := make(map[int]bool)
	var knownLimit, unknownLimit, noContext bool
	for _, scene := range Scenes(start) {
		counts[len(scene.Sessions)] = true
		for _, s := range scene.Sessions {
			statuses[s.Status], kinds[s.Kind] = true, true
			switch {
			case s.Context == nil:
				noContext = true
			case s.Context.Limit == 0:
				unknownLimit = true
			default:
				knownLimit = true
			}
		}
	}
	for _, status := range []agent.Status{agent.StatusWorking, agent.StatusWaiting, agent.StatusError, agent.StatusIdle} {
		if !statuses[status] {
			t.Errorf("no %s session", status)
		}
	}
	if !kinds[agent.KindClaude] || !kinds[agent.KindOpenCode] || !kinds[agent.KindCodex] {
		t.Errorf("agents shown: %v, want all three", kinds)
	}
	if !knownLimit || !unknownLimit || !noContext {
		t.Errorf("context: limit known %v, unknown %v, none %v; want all three", knownLimit, unknownLimit, noContext)
	}
	for _, n := range []int{0, 1, 7, 9, 14} {
		if !counts[n] {
			t.Errorf("no scene with %d sessions", n)
		}
	}
}

func TestFoldersThatShareANameGetDistinctNames(t *testing.T) {
	for _, scene := range Scenes(start) {
		if !strings.HasPrefix(scene.Title, "seven agents") {
			continue
		}
		names := make(map[string]bool)
		for _, c := range agent.Arrange(scene.Sessions) {
			names[c.Name] = true
		}
		for _, want := range []string{"web-shop #1", "web-shop #2", "client/api", "server/api"} {
			if !names[want] {
				t.Errorf("no card named %q in %v", want, names)
			}
		}
		return
	}
	t.Fatal("no seven-agent scene")
}
