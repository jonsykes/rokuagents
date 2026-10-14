package opencode

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/tonylook/managents/helper/internal/agent"
	"github.com/tonylook/managents/helper/internal/process"
)

var now = time.Date(2026, 10, 9, 16, 30, 0, 0, time.UTC)

type fakeProcesses []process.Info

func (f fakeProcesses) FindByName(_ context.Context, name string) ([]process.Info, error) {
	if name != "opencode" {
		return nil, nil
	}
	return f, nil
}

var errStore = errors.New("no such column: data")

// fakeStore serves canned history per folder. messageErr and partErr, when
// set, are returned for every folder.
type fakeStore struct {
	messages   map[string]Message
	parts      map[string]ToolPart
	messageErr error
	partErr    error
}

func (f fakeStore) LastMessage(_ context.Context, dir string) (Message, bool, error) {
	m, ok := f.messages[dir]
	return m, ok && f.messageErr == nil, f.messageErr
}

func (f fakeStore) LastToolPart(_ context.Context, dir string) (ToolPart, bool, error) {
	p, ok := f.parts[dir]
	return p, ok && f.partErr == nil, f.partErr
}

func TestStatusOf(t *testing.T) {
	running := now.Add(-time.Hour) // when the opencode process started
	tests := []struct {
		name    string
		msg     Message
		started time.Time
		want    agent.Status
	}{
		{"reply streaming", Message{Role: "assistant", UpdatedAt: now}, running, agent.StatusWorking},
		{"reply finished", Message{Role: "assistant", Completed: true, UpdatedAt: now}, running, agent.StatusWaiting},
		{"reply failed", Message{Role: "assistant", ErrorName: "APIError", UpdatedAt: now}, running, agent.StatusError},
		{"reply aborted", Message{Role: "assistant", ErrorName: abortedError, UpdatedAt: now}, running, agent.StatusWaiting},
		{"reply interrupted (OpenCode 2)", Message{Role: "assistant", ErrorName: "aborted", UpdatedAt: now}, running, agent.StatusWaiting},
		{"prompt just sent", Message{Role: "user", UpdatedAt: now.Add(-10 * time.Second)}, running, agent.StatusWorking},
		{"prompt long ago", Message{Role: "user", UpdatedAt: now.Add(-10 * time.Minute)}, running, agent.StatusWaiting},
		{"dormant", Message{Role: "assistant", Completed: true, UpdatedAt: now.Add(-3 * time.Hour)}, running, agent.StatusIdle},
		{"an old error is idle", Message{Role: "assistant", ErrorName: "APIError", UpdatedAt: now.Add(-3 * time.Hour)}, running, agent.StatusIdle},
		{"reply left by a previous run", Message{Role: "assistant", UpdatedAt: now.Add(-2 * time.Minute)}, now.Add(-time.Minute), agent.StatusWaiting},
		{"prompt left by a previous run", Message{Role: "user", UpdatedAt: now.Add(-10 * time.Second)}, now.Add(-5 * time.Second), agent.StatusWaiting},
		{"error left by a previous run", Message{Role: "assistant", ErrorName: "APIError", UpdatedAt: now.Add(-2 * time.Minute)}, now.Add(-time.Minute), agent.StatusWaiting},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := statusOf(tt.msg, tt.started, now); got != tt.want {
				t.Errorf("statusOf = %s, want %s", got, tt.want)
			}
		})
	}
}

func TestSessionsOnePerFolder(t *testing.T) {
	started := now.Add(-time.Hour)
	source := &Source{
		Processes: fakeProcesses{
			{PID: 10, Dir: "/w/api", StartedAt: started},
			{PID: 11, Dir: "/w/api", StartedAt: started.Add(-time.Minute)},
			{PID: 12, Dir: "", StartedAt: started},
			{PID: 13, Dir: "/w/web", StartedAt: started},
		},
		Store: fakeStore{messages: map[string]Message{
			"/w/api": {Role: "assistant", UpdatedAt: now.Add(-time.Second), TurnStartedAt: now.Add(-5 * time.Second)},
		}},
	}

	sessions, err := source.Sessions(context.Background(), now)
	if err != nil {
		t.Fatal(err)
	}

	if len(sessions) != 2 {
		t.Fatalf("got %d sessions, want 2: %+v", len(sessions), sessions)
	}
	api, web := sessions[0], sessions[1]
	if api.ID != "opencode:11" || api.Status != agent.StatusWorking {
		t.Errorf("api session = %+v, want the oldest process, working", api)
	}
	if web.Status != agent.StatusWaiting || !web.Since.Equal(started) {
		t.Errorf("web session without history = %+v", web)
	}
}

func TestTheBackgroundServerIsNotASession(t *testing.T) {
	started := now.Add(-time.Hour)
	tests := []struct {
		name string
		args []string
		card bool
	}{
		{"terminal", []string{"opencode"}, true},
		{"a prompt that says serve", []string{"opencode", "run", "serve"}, true},
		{"not readable", nil, true},
		{"service", []string{"/home/ann/.opencode/bin/opencode", "serve", "--service"}, false},
		{"service after a flag", []string{"opencode", "--print-logs", "serve"}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			source := &Source{
				Processes: fakeProcesses{{PID: 1, Dir: "/home/ann", Args: tt.args, StartedAt: started}},
				Store:     fakeStore{},
			}

			sessions, err := source.Sessions(context.Background(), now)

			if err != nil || (len(sessions) == 1) != tt.card {
				t.Errorf("sessions = %+v, %v; want a card: %v", sessions, err, tt.card)
			}
		})
	}
}

func TestWorkingAgeCountsFromThePrompt(t *testing.T) {
	prompt := now.Add(-4 * time.Minute)
	source := &Source{
		Processes: fakeProcesses{{PID: 1, Dir: "/w/api", StartedAt: now.Add(-time.Hour)}},
		Store: fakeStore{messages: map[string]Message{
			// The third reply of the turn, still streaming.
			"/w/api": {Role: "assistant", UpdatedAt: now.Add(-time.Second), TurnStartedAt: prompt},
		}},
	}

	sessions, err := source.Sessions(context.Background(), now)
	if err != nil {
		t.Fatal(err)
	}
	if s := sessions[0]; s.Status != agent.StatusWorking || s.Age(now) != 4*time.Minute {
		t.Errorf("session = %+v, want working for 4m", s)
	}
}

func TestReplyFromAPreviousRunIsNotWorking(t *testing.T) {
	source := &Source{
		Processes: fakeProcesses{{PID: 1, Dir: "/w/api", StartedAt: now.Add(-time.Minute)}},
		Store: fakeStore{messages: map[string]Message{
			// Never completed: the previous opencode in this folder was killed.
			"/w/api": {Role: "assistant", UpdatedAt: now.Add(-time.Hour), TurnStartedAt: now.Add(-time.Hour)},
		}},
	}

	sessions, err := source.Sessions(context.Background(), now)
	if err != nil {
		t.Fatal(err)
	}
	if sessions[0].Status != agent.StatusWaiting {
		t.Errorf("status = %s, want waiting", sessions[0].Status)
	}
}

func TestStaleRunningToolMeansWaiting(t *testing.T) {
	source := &Source{
		Processes: fakeProcesses{{PID: 1, Dir: "/w/api", StartedAt: now}},
		Store: fakeStore{
			messages: map[string]Message{"/w/api": {Role: "assistant", UpdatedAt: now}},
			parts:    map[string]ToolPart{"/w/api": {State: "running", UpdatedAt: now.Add(-time.Minute)}},
		},
	}

	sessions, err := source.Sessions(context.Background(), now)
	if err != nil {
		t.Fatal(err)
	}
	if sessions[0].Status != agent.StatusWaiting {
		t.Errorf("status = %s, want waiting", sessions[0].Status)
	}
}

func TestStoreErrorsAreReportedButSessionsShown(t *testing.T) {
	working := map[string]Message{
		"/w/api": {Role: "assistant", UpdatedAt: now, TurnStartedAt: now},
		"/w/web": {Role: "assistant", UpdatedAt: now, TurnStartedAt: now},
	}
	tests := []struct {
		name  string
		store fakeStore
		want  agent.Status
	}{
		{"messages unreadable", fakeStore{messages: working, messageErr: errStore}, agent.StatusWaiting},
		{"tool calls unreadable", fakeStore{messages: working, partErr: errStore}, agent.StatusWorking},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			source := &Source{
				Processes: fakeProcesses{
					{PID: 1, Dir: "/w/api", StartedAt: now.Add(-time.Hour)},
					{PID: 2, Dir: "/w/web", StartedAt: now.Add(-time.Hour)},
				},
				Store: tt.store,
			}

			sessions, err := source.Sessions(context.Background(), now)

			if !errors.Is(err, errStore) {
				t.Errorf("err = %v, want the store error", err)
			}
			if len(sessions) != 2 {
				t.Fatalf("got %d sessions, want both folders shown: %+v", len(sessions), sessions)
			}
			for _, s := range sessions {
				if s.Status != tt.want {
					t.Errorf("%s: status = %s, want %s", s.Dir, s.Status, tt.want)
				}
			}
		})
	}
}
