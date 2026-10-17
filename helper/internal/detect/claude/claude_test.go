package claude

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/tonylook/managents/helper/internal/agent"
	"github.com/tonylook/managents/helper/internal/protocol"
)

var now = time.Date(2026, 10, 9, 16, 30, 0, 0, time.UTC)

// fakeProcesses maps pid -> start time.
type fakeProcesses map[int]time.Time

func (f fakeProcesses) StartTime(_ context.Context, pid int) (time.Time, bool) {
	t, ok := f[pid]
	return t, ok
}

type fixture struct {
	t      *testing.T
	home   string
	procs  fakeProcesses
	source *Source
}

func newFixture(t *testing.T) *fixture {
	t.Helper()
	t.Setenv("CLAUDE_CONFIG_DIR", "") // the developer's own setting must not leak in
	home := t.TempDir()
	procs := fakeProcesses{}
	return &fixture{t: t, home: home, procs: procs, source: NewSource(home, procs)}
}

// addSession writes a registry record for a running process.
func (f *fixture) addSession(pid int, fields map[string]any) {
	f.t.Helper()
	started := now.Add(-time.Hour)
	f.procs[pid] = started.Add(400 * time.Millisecond)
	record := map[string]any{
		"pid":       pid,
		"sessionId": "session-" + strconv.Itoa(pid),
		"cwd":       "/work/project",
		"startedAt": started.UnixMilli(),
		"procStart": started.Format(procStartLayout),
		"status":    "idle",
	}
	for k, v := range fields {
		record[k] = v
	}
	writeJSON(f.t, filepath.Join(f.source.SessionsDir, strconv.Itoa(pid)+".json"), record)
}

func (f *fixture) addTranscript(sessionID string, entries ...map[string]any) {
	f.t.Helper()
	var lines []string
	for _, e := range entries {
		data, err := json.Marshal(e)
		if err != nil {
			f.t.Fatal(err)
		}
		lines = append(lines, string(data))
	}
	path := filepath.Join(f.source.ProjectsDir, "-work-project", sessionID+".jsonl")
	writeFile(f.t, path, strings.Join(lines, "\n")+"\n")
}

func (f *fixture) sessions() []agent.Session {
	f.t.Helper()
	sessions, err := f.source.Sessions(context.Background(), now)
	if err != nil {
		f.t.Fatal(err)
	}
	return sessions
}

func TestStatusMapping(t *testing.T) {
	recent := now.Add(-time.Minute).UnixMilli()
	hoursAgo := now.Add(-3 * time.Hour).UnixMilli()
	tests := []struct {
		name        string
		fields      map[string]any
		endsInError bool
		want        agent.Status
	}{
		{"busy is working", map[string]any{"status": "busy", "statusUpdatedAt": recent}, false, agent.StatusWorking},
		{"busy is never error", map[string]any{"status": "busy", "statusUpdatedAt": recent}, true, agent.StatusWorking},
		{"idle prompt is waiting", map[string]any{"statusUpdatedAt": recent}, false, agent.StatusWaiting},
		{"untouched for hours is idle", map[string]any{"statusUpdatedAt": hoursAgo}, false, agent.StatusIdle},
		{"turn ended in an API error", map[string]any{"statusUpdatedAt": recent}, true, agent.StatusError},
		{"an old error is idle", map[string]any{"statusUpdatedAt": hoursAgo}, true, agent.StatusIdle},
		{"permission prompt is waiting", map[string]any{"status": "waiting", "waitingFor": "permission prompt", "statusUpdatedAt": recent}, false, agent.StatusWaiting},
		{"a prompt never turns idle", map[string]any{"status": "waiting", "statusUpdatedAt": hoursAgo}, false, agent.StatusWaiting},
		{"waitingFor wins over busy", map[string]any{"status": "busy", "waitingFor": "permission prompt"}, false, agent.StatusWaiting},
		{"empty waitingFor is ignored", map[string]any{"status": "busy", "waitingFor": ""}, false, agent.StatusWorking},
		{"null waitingFor is ignored", map[string]any{"status": "busy", "waitingFor": nil}, false, agent.StatusWorking},
		{"unknown status counts as idle", map[string]any{"status": "compacting", "statusUpdatedAt": recent}, false, agent.StatusWaiting},
	}
	for i, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := newFixture(t)
			f.addSession(100+i, tt.fields)
			if tt.endsInError {
				f.addTranscript("session-"+strconv.Itoa(100+i), map[string]any{"type": "assistant", "isApiErrorMessage": true})
			}
			sessions := f.sessions()
			if len(sessions) != 1 {
				t.Fatalf("got %d sessions, want 1", len(sessions))
			}
			if sessions[0].Status != tt.want {
				t.Errorf("status = %s, want %s", sessions[0].Status, tt.want)
			}
		})
	}
}

func TestSessionFields(t *testing.T) {
	f := newFixture(t)
	changed := now.Add(-42 * time.Second)
	f.addSession(4242, map[string]any{"cwd": "/work/agent-lights", "statusUpdatedAt": changed.UnixMilli()})

	s := f.sessions()[0]

	if s.ID != "claude:4242" || s.Kind != agent.KindClaude || s.Dir != "/work/agent-lights" {
		t.Errorf("unexpected identity: %+v", s)
	}
	if got := s.Age(now); got != 42*time.Second {
		t.Errorf("age = %v, want 42s", got)
	}
}

func TestSkipsStaleAndBackgroundRecords(t *testing.T) {
	f := newFixture(t)
	f.addSession(1, nil)
	f.addSession(2, map[string]any{"spare": true})
	f.addSession(3, map[string]any{"parkedJobId": "job-1"})
	f.addSession(4, nil)
	delete(f.procs, 4) // process exited, file left behind
	f.addSession(5, nil)
	f.procs[5] = now // pid reused by a newer process
	writeFile(t, filepath.Join(f.source.SessionsDir, "6.json"), "{not json")
	record, err := os.ReadFile(filepath.Join(f.source.SessionsDir, "1.json"))
	if err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(f.source.SessionsDir, "1-backup.json"), string(record)) // not a registry file name

	sessions := f.sessions()

	if len(sessions) != 1 || sessions[0].ID != "claude:1" {
		t.Errorf("sessions = %+v, want only claude:1", sessions)
	}
}

func TestErrorAndContextFromTranscript(t *testing.T) {
	f := newFixture(t)
	f.addSession(7, map[string]any{"sessionId": "abc", "statusUpdatedAt": now.UnixMilli()})
	f.addTranscript("abc",
		map[string]any{"type": "user", "message": map[string]any{"content": "secret prompt"}},
		map[string]any{"type": "assistant", "message": map[string]any{"usage": map[string]any{
			"input_tokens": 10, "cache_creation_input_tokens": 1000, "cache_read_input_tokens": 87000,
		}}},
		map[string]any{"type": "assistant", "isSidechain": true, "message": map[string]any{"usage": map[string]any{
			"input_tokens": 5,
		}}},
		map[string]any{"type": "assistant", "isApiErrorMessage": true},
	)

	s := f.sessions()[0]

	if s.Status != agent.StatusError {
		t.Errorf("status = %s, want error", s.Status)
	}
	if s.Context == nil || s.Context.Used != 88010 || s.Context.Limit != StandardContextWindow {
		t.Errorf("context = %+v, want 88010 of the standard window", s.Context)
	}
}

func TestTranscriptEndings(t *testing.T) {
	reply := func(tokens int) map[string]any {
		return map[string]any{"type": "assistant", "message": map[string]any{"usage": map[string]any{"cache_read_input_tokens": tokens}}}
	}
	// Claude Code records an API error as a synthetic assistant message with
	// zero usage, then appends bookkeeping entries after it.
	apiError := map[string]any{"type": "assistant", "isApiErrorMessage": true, "message": map[string]any{
		"model": "<synthetic>", "usage": map[string]any{"input_tokens": 0, "cache_creation_input_tokens": 0, "cache_read_input_tokens": 0},
	}}
	bookkeeping := []map[string]any{
		{"type": "system", "subtype": "turn_duration"}, {"type": "last-prompt"}, {"type": "ai-title"},
		{"type": "mode"}, {"type": "permission-mode"},
	}
	prompt := map[string]any{"type": "user", "message": map[string]any{"content": "next question"}}
	subagentError := map[string]any{"type": "assistant", "isSidechain": true, "isApiErrorMessage": true}

	tests := []struct {
		name       string
		entries    []map[string]any
		wantStatus agent.Status
		wantUsed   int
	}{
		{"API error followed by bookkeeping", append([]map[string]any{reply(90_000), apiError}, bookkeeping...), agent.StatusError, 90_000},
		{"API error keeps the last real context", []map[string]any{reply(42_000), apiError}, agent.StatusError, 42_000},
		{"a new prompt clears the error", []map[string]any{reply(90_000), apiError, prompt}, agent.StatusWaiting, 90_000},
		{"a subagent error is not the session's", []map[string]any{reply(1000), subagentError}, agent.StatusWaiting, 1000},
		{"finished turn followed by bookkeeping", append([]map[string]any{reply(5000)}, bookkeeping...), agent.StatusWaiting, 5000},
	}
	for i, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := newFixture(t)
			id := "ending-" + strconv.Itoa(i)
			f.addSession(200+i, map[string]any{"sessionId": id, "statusUpdatedAt": now.UnixMilli()})
			f.addTranscript(id, tt.entries...)

			s := f.sessions()[0]

			if s.Status != tt.wantStatus {
				t.Errorf("status = %s, want %s", s.Status, tt.wantStatus)
			}
			if s.Context == nil || s.Context.Used != tt.wantUsed {
				t.Errorf("context = %+v, want %d used", s.Context, tt.wantUsed)
			}
		})
	}
}

func TestContextBehindAHugeToolResult(t *testing.T) {
	tests := []struct {
		name       string
		resultSize int
		wantUsed   int // 0: no context
	}{
		{"larger than the first window", 3 * transcriptTail, 77_000},
		{"larger than the largest window", maxTranscriptTail, 0},
	}
	for i, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := newFixture(t)
			id := "huge-" + strconv.Itoa(i)
			f.addSession(300+i, map[string]any{"sessionId": id, "statusUpdatedAt": now.UnixMilli()})
			f.addTranscript(id,
				map[string]any{"type": "assistant", "message": map[string]any{"usage": map[string]any{"input_tokens": 77_000}}},
				map[string]any{"type": "user", "toolUseResult": strings.Repeat("x", tt.resultSize)},
			)

			s := f.sessions()[0]

			switch {
			case tt.wantUsed == 0 && s.Context != nil:
				t.Errorf("context = %+v, want none: the scan must stay bounded", s.Context)
			case tt.wantUsed != 0 && (s.Context == nil || s.Context.Used != tt.wantUsed):
				t.Errorf("context = %+v, want %d used", s.Context, tt.wantUsed)
			}
		})
	}
}

func TestConversationContentNeverLeaks(t *testing.T) {
	const canary = "CANARY-7f3a"
	text := []any{map[string]any{"type": "text", "text": canary}}
	f := newFixture(t)
	f.addSession(9, map[string]any{"sessionId": "private", "statusUpdatedAt": now.UnixMilli(), "name": canary})
	f.addTranscript("private",
		map[string]any{"type": "user", "message": map[string]any{"role": "user", "content": canary}},
		map[string]any{"type": "assistant", "message": map[string]any{"content": text, "usage": map[string]any{"input_tokens": 1234}}},
		map[string]any{"type": "user", "toolUseResult": map[string]any{"stdout": canary},
			"message": map[string]any{"content": []any{map[string]any{"type": "tool_result", "content": canary}}}},
		map[string]any{"type": "assistant", "isApiErrorMessage": true, "message": map[string]any{"content": text}},
		map[string]any{"type": "ai-title", "aiTitle": canary},
		map[string]any{"type": "last-prompt", "lastPrompt": canary},
	)

	sessions := f.sessions()

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

func TestContextLimit(t *testing.T) {
	inferred := &Source{}
	if got := inferred.contextLimit(150_000); got != StandardContextWindow {
		t.Errorf("150k used: limit %d, want standard", got)
	}
	if got := inferred.contextLimit(240_000); got != ExtendedContextWindow {
		t.Errorf("240k used: limit %d, want extended (it cannot fit the standard window)", got)
	}
	if got := (&Source{ContextWindow: 1_000_000}).contextLimit(10_000); got != 1_000_000 {
		t.Errorf("configured window ignored: %d", got)
	}
}

func TestConfigDirectory(t *testing.T) {
	home := t.TempDir()
	custom := filepath.Join(t.TempDir(), "claude-config")
	tests := []struct {
		name, env, want string
	}{
		{"default", "", filepath.Join(home, ".claude")},
		{"CLAUDE_CONFIG_DIR", custom, custom},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("CLAUDE_CONFIG_DIR", tt.env)
			s := NewSource(home, fakeProcesses{})
			if s.SessionsDir != filepath.Join(tt.want, "sessions") || s.ProjectsDir != filepath.Join(tt.want, "projects") {
				t.Errorf("dirs = %q, %q, want under %q", s.SessionsDir, s.ProjectsDir, tt.want)
			}
		})
	}
}

func TestMissingRegistryIsNotAnError(t *testing.T) {
	f := newFixture(t)
	if sessions := f.sessions(); len(sessions) != 0 {
		t.Errorf("sessions = %v, want none", sessions)
	}
}

func writeJSON(t *testing.T, path string, value any) {
	t.Helper()
	data, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	writeFile(t, path, string(data))
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}

func TestTitles(t *testing.T) {
	tests := []struct {
		name    string
		entries []map[string]any
		want    string
	}{
		{"none", []map[string]any{{"type": "user"}}, ""},
		{"generated", []map[string]any{{"type": "ai-title", "aiTitle": "Fix the build"}}, "Fix the build"},
		{"latest generated", []map[string]any{
			{"type": "ai-title", "aiTitle": "First"}, {"type": "ai-title", "aiTitle": "Second"},
		}, "Second"},
		{"custom wins over a later generated one", []map[string]any{
			{"type": "custom-title", "customTitle": "Payments refactor"}, {"type": "ai-title", "aiTitle": "Generated"},
		}, "Payments refactor"},
		{"cleared custom falls back", []map[string]any{
			{"type": "ai-title", "aiTitle": "Generated"},
			{"type": "custom-title", "customTitle": "Mine"}, {"type": "custom-title", "customTitle": ""},
		}, "Generated"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := newFixture(t)
			f.source.Titles = true
			f.addSession(7, nil)
			f.addTranscript("session-7", tt.entries...)
			sessions := f.sessions()
			if len(sessions) != 1 || sessions[0].Title != tt.want {
				t.Fatalf("sessions = %+v, want title %q", sessions, tt.want)
			}
		})
	}
}

func TestTitlesOffByDefault(t *testing.T) {
	f := newFixture(t)
	f.addSession(7, nil)
	f.addTranscript("session-7", map[string]any{"type": "custom-title", "customTitle": "Payments refactor"})
	if sessions := f.sessions(); len(sessions) != 1 || sessions[0].Title != "" {
		t.Fatalf("sessions = %+v, want no title unless asked", sessions)
	}
}

func TestTitleFollowsAppendsAndPartialLines(t *testing.T) {
	f := newFixture(t)
	f.source.Titles = true
	f.addSession(7, nil)
	f.addTranscript("session-7", map[string]any{"type": "ai-title", "aiTitle": "Old"})
	if got := f.sessions()[0].Title; got != "Old" {
		t.Fatalf("title = %q, want Old", got)
	}
	path := filepath.Join(f.source.ProjectsDir, "-work-project", "session-7.jsonl")
	file, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	if _, err := file.WriteString(`{"type":"custom-title","customTitle":"New"`); err != nil {
		t.Fatal(err)
	}
	if got := f.sessions()[0].Title; got != "Old" {
		t.Fatalf("title = %q while the line is partial, want Old", got)
	}
	if _, err := file.WriteString("}\n"); err != nil {
		t.Fatal(err)
	}
	if got := f.sessions()[0].Title; got != "New" {
		t.Fatalf("title = %q once the line is complete, want New", got)
	}
}

func TestTitlesCarryNoOtherContent(t *testing.T) {
	const canary = "CANARY-7f3a"
	f := newFixture(t)
	f.source.Titles = true
	f.addSession(9, map[string]any{"sessionId": "private", "statusUpdatedAt": now.UnixMilli()})
	f.addTranscript("private",
		map[string]any{"type": "user", "message": map[string]any{"role": "user", "content": canary + `-title"`}},
		map[string]any{"type": "last-prompt", "lastPrompt": canary},
		map[string]any{"type": "ai-title", "aiTitle": "Safe title"},
	)
	sessions := f.sessions()
	frame, err := protocol.Encode(protocol.NewState(agent.Arrange(sessions), now))
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(frame, []byte(canary)) || !bytes.Contains(frame, []byte("Safe title")) {
		t.Errorf("frame = %s, want the title and nothing else from the conversation", frame)
	}
}
