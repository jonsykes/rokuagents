package codex

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/tonylook/managents/helper/internal/agent"
	"github.com/tonylook/managents/helper/internal/process"
	"github.com/tonylook/managents/helper/internal/protocol"
)

var now = time.Date(2026, 10, 9, 16, 30, 0, 0, time.UTC)

// fakeProcesses serves the codex processes and, separately, their children.
type fakeProcesses struct {
	codex       []process.Info
	children    []process.Info
	childrenErr error
	asked       *int // counts Children calls when set
}

func (f fakeProcesses) FindByName(_ context.Context, name string) ([]process.Info, error) {
	if name != "codex" {
		return nil, nil
	}
	return f.codex, nil
}

func (f fakeProcesses) Children(_ context.Context, parents []int) ([]process.Info, error) {
	if f.asked != nil {
		*f.asked++
	}
	var found []process.Info
	for _, c := range f.children {
		for _, p := range parents {
			if c.PID/100 == p { // a child's pid is its parent's times 100 plus n
				found = append(found, c)
			}
		}
	}
	return found, f.childrenErr
}

var errStore = errors.New("database is locked")

// fakeStore serves the rollout of each folder.
type fakeStore struct {
	rollouts map[string]string
	err      error
}

func (f fakeStore) LastThread(_ context.Context, dir string) (Thread, bool, error) {
	path, ok := f.rollouts[dir]
	return Thread{RolloutPath: path}, ok && f.err == nil, f.err
}

// line is one rollout entry at time at.
func line(at time.Time, kind string, payload map[string]any) map[string]any {
	return map[string]any{"timestamp": at.Format(time.RFC3339Nano), "type": kind, "payload": payload}
}

func event(at time.Time, name string, fields ...any) map[string]any {
	payload := map[string]any{"type": name}
	for i := 0; i+1 < len(fields); i += 2 {
		payload[fields[i].(string)] = fields[i+1]
	}
	return line(at, "event_msg", payload)
}

func item(at time.Time, kind string) map[string]any {
	return line(at, "response_item", map[string]any{"type": kind, "content": "x", "arguments": "{}"})
}

func tokens(at time.Time, used, window int) map[string]any {
	return event(at, "token_count", "info", map[string]any{
		"total_token_usage":    map[string]any{"total_tokens": used * 3},
		"last_token_usage":     map[string]any{"input_tokens": used - 5, "output_tokens": 5, "total_tokens": used},
		"model_context_window": window,
	})
}

// writeRollout writes entries as a rollout and returns its path.
func writeRollout(t *testing.T, entries ...map[string]any) string {
	t.Helper()
	var buf bytes.Buffer
	for _, e := range entries {
		data, err := json.Marshal(e)
		if err != nil {
			t.Fatal(err)
		}
		buf.Write(data)
		buf.WriteByte('\n')
	}
	path := filepath.Join(t.TempDir(), "rollout.jsonl")
	if err := os.WriteFile(path, buf.Bytes(), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestSummarize(t *testing.T) {
	t0 := now.Add(-time.Minute)
	tests := []struct {
		name     string
		entries  []map[string]any
		turn     turnState
		at       time.Time
		failed   bool
		openCall time.Time
	}{
		{"no turn yet", []map[string]any{line(t0, "session_meta", map[string]any{"cwd": "/w/api"})}, noTurn, time.Time{}, false, time.Time{}},
		{"model replying", []map[string]any{
			event(t0, "task_started"), item(t0.Add(time.Second), "message"),
		}, turnRunning, t0, false, time.Time{}},
		{"tool call without output", []map[string]any{
			event(t0, "task_started"), item(t0.Add(time.Second), "reasoning"), item(t0.Add(2*time.Second), "custom_tool_call"),
			line(t0.Add(3*time.Second), "token_usage_record", map[string]any{}),
		}, turnRunning, t0, false, t0.Add(2 * time.Second)},
		{"tool call with output", []map[string]any{
			event(t0, "task_started"), item(t0.Add(time.Second), "function_call"), item(t0.Add(2*time.Second), "function_call_output"),
		}, turnRunning, t0, false, time.Time{}},
		{"turn complete", []map[string]any{
			event(t0, "task_started"), item(t0.Add(time.Second), "function_call"),
			event(t0.Add(2*time.Second), "task_complete", "error", nil),
		}, turnEnded, t0.Add(2 * time.Second), false, time.Time{}},
		{"turn failed", []map[string]any{
			event(t0, "task_started"), event(t0.Add(time.Second), "task_complete", "error", map[string]any{"message": "bad"}),
		}, turnEnded, t0.Add(time.Second), true, time.Time{}},
		{"turn failed (older Codex)", []map[string]any{
			event(t0, "task_started"), event(t0.Add(time.Second), "error", "message", "bad"),
			event(t0.Add(2*time.Second), "task_complete"),
		}, turnEnded, t0.Add(2 * time.Second), true, time.Time{}},
		{"an earlier turn failed", []map[string]any{
			event(t0, "task_started"), event(t0.Add(time.Second), "error", "message", "bad"),
			event(t0.Add(2*time.Second), "task_complete"), event(t0.Add(3*time.Second), "task_started"),
			event(t0.Add(4*time.Second), "task_complete"),
		}, turnEnded, t0.Add(4 * time.Second), false, time.Time{}},
		{"interrupted", []map[string]any{
			event(t0, "task_started"), item(t0.Add(time.Second), "function_call"),
			event(t0.Add(2*time.Second), "turn_aborted", "reason", "interrupted"),
		}, turnEnded, t0.Add(2 * time.Second), false, time.Time{}},
		{"next turn started", []map[string]any{
			event(t0, "task_started"), event(t0.Add(time.Second), "task_complete"), event(t0.Add(2*time.Second), "task_started"),
		}, turnRunning, t0.Add(2 * time.Second), false, time.Time{}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := summarizeRollout(writeRollout(t, tt.entries...))
			if err != nil {
				t.Fatal(err)
			}
			if got.turn != tt.turn || !got.at.Equal(tt.at) || got.failed != tt.failed || !got.openCall.Equal(tt.openCall) {
				t.Errorf("summary = %+v, want turn %d at %v, failed %v, open call %v", got, tt.turn, tt.at, tt.failed, tt.openCall)
			}
		})
	}
}

func TestContextFromTheLastTokenCount(t *testing.T) {
	t0 := now.Add(-time.Minute)
	path := writeRollout(t,
		event(t0, "task_started"), tokens(t0, 1000, 258400),
		event(t0.Add(time.Second), "task_complete"), event(t0.Add(2*time.Second), "task_started"),
		tokens(t0.Add(3*time.Second), 40000, 258400),
		event(t0.Add(4*time.Second), "token_count", "info", nil, "rate_limits", map[string]any{}),
	)

	got, err := summarizeRollout(path)

	if err != nil || got.context == nil || *got.context != (agent.ContextUsage{Used: 40000, Limit: 258400}) {
		t.Errorf("context = %+v, %v; want 40000 of 258400 from the last count with usage", got.context, err)
	}
}

func TestTurnStartBehindAHugeToolOutput(t *testing.T) {
	t0 := now.Add(-time.Minute)
	huge := line(t0.Add(time.Second), "response_item", map[string]any{
		"type": "function_call_output", "output": strings.Repeat("x", rolloutTail),
	})
	path := writeRollout(t, tokens(t0, 5000, 258400), event(t0, "task_started"), huge, item(t0.Add(2*time.Second), "function_call"))

	got, err := summarizeRollout(path)

	if err != nil || got.turn != turnRunning || !got.at.Equal(t0) || got.context == nil {
		t.Errorf("summary = %+v, %v; want the running turn and its context found past the output", got, err)
	}
}

func TestStatusOf(t *testing.T) {
	running := now.Add(-time.Hour) // when the codex process started
	ended := func(at time.Time, failed bool) rolloutSummary {
		return rolloutSummary{turn: turnEnded, at: at, failed: failed}
	}
	tests := []struct {
		name      string
		summary   rolloutSummary
		started   time.Time
		want      agent.Status
		wantSince time.Time
	}{
		{"no turn yet", rolloutSummary{}, running, agent.StatusWaiting, running},
		{"turn running", rolloutSummary{turn: turnRunning, at: now.Add(-time.Minute)}, running, agent.StatusWorking, now.Add(-time.Minute)},
		{"turn done", ended(now.Add(-time.Minute), false), running, agent.StatusWaiting, now.Add(-time.Minute)},
		{"turn failed", ended(now.Add(-time.Minute), true), running, agent.StatusError, now.Add(-time.Minute)},
		{"dormant", ended(now.Add(-3*time.Hour), false), now.Add(-4 * time.Hour), agent.StatusIdle, now.Add(-3 * time.Hour)},
		{"an old error is idle", ended(now.Add(-3*time.Hour), true), now.Add(-4 * time.Hour), agent.StatusIdle, now.Add(-3 * time.Hour)},
		{"resumed thread", ended(now.Add(-48*time.Hour), false), running, agent.StatusWaiting, running},
		{"turn left running by a previous run", rolloutSummary{turn: turnRunning, at: running.Add(-time.Minute)}, running, agent.StatusWaiting, running},
		{"error left by a previous run", ended(running.Add(-time.Minute), true), running, agent.StatusWaiting, running},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, since := statusOf(tt.summary, tt.started, now)
			if got != tt.want || !since.Equal(tt.wantSince) {
				t.Errorf("statusOf = %s since %v, want %s since %v", got, since, tt.want, tt.wantSince)
			}
		})
	}
}

func TestSessionsOnePerFolder(t *testing.T) {
	started := now.Add(-time.Hour)
	working := writeRollout(t, event(now.Add(-5*time.Second), "task_started"), tokens(now.Add(-time.Second), 30000, 258400))
	source := &Source{
		Processes: fakeProcesses{codex: []process.Info{
			{PID: 10, Dir: "/w/api", Args: []string{"codex"}, StartedAt: started},
			{PID: 11, Dir: "/w/api", Args: []string{"codex", "resume"}, StartedAt: started.Add(-time.Minute)},
			{PID: 12, Dir: "", Args: []string{"codex"}, StartedAt: started},
			{PID: 13, Dir: "/w/web", Args: []string{"codex"}, StartedAt: started},
		}},
		Store: fakeStore{rollouts: map[string]string{"/w/api": working}},
	}

	sessions, err := source.Sessions(context.Background(), now)
	if err != nil {
		t.Fatal(err)
	}

	if len(sessions) != 2 {
		t.Fatalf("got %d sessions, want 2: %+v", len(sessions), sessions)
	}
	api, web := sessions[0], sessions[1]
	if api.ID != "codex:11" || api.Kind != agent.KindCodex || api.Status != agent.StatusWorking ||
		!api.Since.Equal(now.Add(-5*time.Second)) || !api.StartedAt.Equal(started.Add(-time.Minute)) {
		t.Errorf("api session = %+v, want the oldest process, working since the turn started", api)
	}
	if api.Context == nil || *api.Context != (agent.ContextUsage{Used: 30000, Limit: 258400}) {
		t.Errorf("api context = %+v", api.Context)
	}
	if web.Status != agent.StatusWaiting || !web.Since.Equal(started) || web.Context != nil {
		t.Errorf("web session without history = %+v", web)
	}
}

func TestOnlySessionsGetACard(t *testing.T) {
	started := now.Add(-time.Hour)
	tests := []struct {
		name string
		args []string
		card bool
	}{
		{"terminal", []string{"codex"}, true},
		{"terminal with flags", []string{"/opt/homebrew/bin/codex", "--sandbox", "read-only"}, true},
		{"first prompt", []string{"codex", "fix the failing test"}, true},
		{"resume", []string{"codex", "resume", "--last"}, true},
		{"not readable", nil, true},
		{"app server", []string{"/Users/ann/.codex/packages/bin/codex", "app-server", "--listen", "unix://"}, false},
		{"app server daemon", []string{"codex", "app-server", "daemon", "pid-update-loop"}, false},
		{"mcp server", []string{"codex", "mcp-server"}, false},
		{"exec", []string{"codex", "exec", "--json", "review this"}, false},
		{"exec after a flag", []string{"codex", "--search", "e", "review this"}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			source := &Source{
				Processes: fakeProcesses{codex: []process.Info{{PID: 10, Dir: "/w/api", Args: tt.args, StartedAt: started}}},
				Store:     fakeStore{},
			}
			sessions, err := source.Sessions(context.Background(), now)
			if err != nil {
				t.Fatal(err)
			}
			if got := len(sessions) == 1; got != tt.card {
				t.Errorf("card = %v, want %v (%+v)", got, tt.card, sessions)
			}
		})
	}
}

func TestToolCallWaitsUnlessACommandRuns(t *testing.T) {
	started := now.Add(-time.Hour)
	turn := now.Add(-time.Minute)
	call := now.Add(-30 * time.Second)
	tui := process.Info{PID: 10, Dir: "/w/api", Args: []string{"codex"}, StartedAt: started}
	server := process.Info{PID: 20, Dir: "/Users/ann", Args: []string{"codex", "app-server"}, StartedAt: started}
	tests := []struct {
		name     string
		call     time.Time
		children []process.Info
		want     agent.Status
	}{
		{"approval prompt", call, nil, agent.StatusWaiting},
		{"call just written", now.Add(-2 * time.Second), nil, agent.StatusWorking},
		{"command run by the TUI", call, []process.Info{{PID: 1001, Name: "zsh", Dir: "/w/api", StartedAt: call}}, agent.StatusWorking},
		{"command run by the app server", call, []process.Info{{PID: 2001, Name: "sleep", Dir: "/w/api/src", StartedAt: call.Add(time.Second)}}, agent.StatusWorking},
		{"command start rounded down", call, []process.Info{{PID: 1001, Name: "go", Dir: "/w/api", StartedAt: turn.Add(-time.Second)}}, agent.StatusWorking},
		{"long command polled by a later call", call, []process.Info{{PID: 2001, Name: "make", Dir: "/w/api", StartedAt: turn.Add(5 * time.Second)}}, agent.StatusWorking},
		{"command left by an earlier turn", call, []process.Info{{PID: 2001, Name: "sleep", Dir: "/w/api", StartedAt: turn.Add(-10 * time.Second)}}, agent.StatusWaiting},
		{"Codex's own helper", call, []process.Info{{PID: 1001, Name: "codex-code-mode-", Dir: "/w/api", StartedAt: call}}, agent.StatusWaiting},
		{"a server started with the session", call, []process.Info{{PID: 1001, Name: "node", Dir: "/w/api", StartedAt: started}}, agent.StatusWaiting},
		{"another session's command", call, []process.Info{{PID: 2001, Name: "zsh", Dir: "/w/api-v2", StartedAt: call}}, agent.StatusWaiting},
		{"cwd not readable", call, []process.Info{{PID: 2001, Name: "zsh", StartedAt: call}}, agent.StatusWaiting},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rollout := writeRollout(t, event(turn, "task_started"), item(tt.call, "function_call"))
			source := &Source{
				Processes: fakeProcesses{codex: []process.Info{tui, server}, children: tt.children},
				Store:     fakeStore{rollouts: map[string]string{"/w/api": rollout}},
			}
			sessions, err := source.Sessions(context.Background(), now)
			if err != nil {
				t.Fatal(err)
			}
			if len(sessions) != 1 || sessions[0].Status != tt.want {
				t.Fatalf("sessions = %+v, want one %s", sessions, tt.want)
			}
			if tt.want == agent.StatusWaiting && !sessions[0].Since.Equal(tt.call) {
				t.Errorf("waiting since %v, want since the call %v", sessions[0].Since, tt.call)
			}
		})
	}
}

func TestChildrenAreListedOncePerDetection(t *testing.T) {
	started := now.Add(-time.Hour)
	call := now.Add(-30 * time.Second)
	rollout := writeRollout(t, event(now.Add(-time.Minute), "task_started"), item(call, "function_call"))
	idle := writeRollout(t, event(now.Add(-time.Minute), "task_started"), event(call, "task_complete"))
	asked := 0
	source := &Source{
		Processes: fakeProcesses{asked: &asked, codex: []process.Info{
			{PID: 10, Dir: "/w/api", StartedAt: started},
			{PID: 11, Dir: "/w/web", StartedAt: started},
			{PID: 12, Dir: "/w/docs", StartedAt: started},
		}},
		Store: fakeStore{rollouts: map[string]string{"/w/api": rollout, "/w/web": rollout, "/w/docs": idle}},
	}

	if _, err := source.Sessions(context.Background(), now); err != nil {
		t.Fatal(err)
	}

	if asked != 1 {
		t.Errorf("children listed %d times, want once", asked)
	}
}

func TestUnreadableHistoryStillShowsTheSession(t *testing.T) {
	started := now.Add(-time.Hour)
	call := now.Add(-30 * time.Second)
	errChildren := errors.New("sysctl failed")
	rollout := writeRollout(t, event(now.Add(-time.Minute), "task_started"), item(call, "function_call"))
	tests := []struct {
		name    string
		store   Store
		procs   fakeProcesses
		want    agent.Status
		wantErr error
	}{
		{"store error", fakeStore{err: errStore}, fakeProcesses{}, agent.StatusWaiting, errStore},
		{"rollout not written yet", fakeStore{rollouts: map[string]string{"/w/api": filepath.Join(t.TempDir(), "missing.jsonl")}},
			fakeProcesses{}, agent.StatusWaiting, nil},
		{"children not listed", fakeStore{rollouts: map[string]string{"/w/api": rollout}},
			fakeProcesses{childrenErr: errChildren}, agent.StatusWorking, errChildren},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.procs.codex = []process.Info{{PID: 10, Dir: "/w/api", StartedAt: started}}
			source := &Source{Processes: tt.procs, Store: tt.store}

			sessions, err := source.Sessions(context.Background(), now)

			if !errors.Is(err, tt.wantErr) || (tt.wantErr == nil && err != nil) {
				t.Errorf("err = %v, want %v", err, tt.wantErr)
			}
			if len(sessions) != 1 || sessions[0].Status != tt.want {
				t.Errorf("sessions = %+v, want one %s", sessions, tt.want)
			}
		})
	}
}

func TestConversationContentNeverLeaks(t *testing.T) {
	const canary = "CANARY-7f3a"
	t0 := now.Add(-time.Minute)
	text := []any{map[string]any{"type": "input_text", "text": canary}}
	rollout := writeRollout(t,
		line(t0, "session_meta", map[string]any{"id": "t1", "cwd": "/w/api", "base_instructions": map[string]any{"text": canary}}),
		event(t0, "task_started", "turn_id", canary),
		line(t0, "response_item", map[string]any{"type": "message", "role": "user", "content": text}),
		event(t0, "user_message", "message", canary),
		line(t0, "turn_context", map[string]any{"cwd": "/w/api", "user_instructions": canary}),
		line(t0, "response_item", map[string]any{"type": "reasoning", "summary": text, "encrypted_content": canary}),
		line(t0, "response_item", map[string]any{"type": "function_call", "name": "shell", "arguments": canary}),
		line(t0, "response_item", map[string]any{"type": "function_call_output", "output": canary}),
		event(t0, "agent_message", "message", canary),
		event(t0, "error", "message", canary),
		tokens(t0, 1234, 258400),
		event(t0.Add(time.Second), "task_complete", "last_agent_message", canary, "error", map[string]any{"message": canary}),
	)
	source := &Source{
		Processes: fakeProcesses{codex: []process.Info{{PID: 10, Dir: "/w/api", Args: []string{"codex", canary}, StartedAt: now.Add(-time.Hour)}}},
		Store:     fakeStore{rollouts: map[string]string{"/w/api": rollout}},
	}

	sessions, err := source.Sessions(context.Background(), now)
	if err != nil {
		t.Fatal(err)
	}

	if len(sessions) != 1 || sessions[0].Status != agent.StatusError || sessions[0].Context == nil {
		t.Fatalf("sessions = %+v, want the session read with its error and context", sessions)
	}
	detected, err := json.Marshal(sessions)
	if err != nil {
		t.Fatal(err)
	}
	frame, err := protocol.Encode(protocol.NewState(agent.Arrange(sessions), now))
	if err != nil {
		t.Fatal(err)
	}
	for what, data := range map[string][]byte{"sessions": detected, "frame": frame} {
		if bytes.Contains(data, []byte(canary)) {
			t.Errorf("conversation content leaked into the %s: %s", what, data)
		}
	}
}
