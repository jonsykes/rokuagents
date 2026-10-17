package claude

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/tonylook/managents/helper/internal/agent"
)

// jsonl joins entries into transcript lines.
func jsonl(t *testing.T, entries ...map[string]any) []byte {
	t.Helper()
	var lines []string
	for _, e := range entries {
		data, err := json.Marshal(e)
		if err != nil {
			t.Fatal(err)
		}
		lines = append(lines, string(data))
	}
	return []byte(strings.Join(lines, "\n") + "\n")
}

func prompt(text string) map[string]any {
	return map[string]any{"type": "user", "message": map[string]any{"role": "user", "content": text}}
}

func reply(blocks ...map[string]any) map[string]any {
	content := make([]any, len(blocks))
	for i, b := range blocks {
		content[i] = b
	}
	return map[string]any{"type": "assistant", "message": map[string]any{"content": content}}
}

func text(s string) map[string]any { return map[string]any{"type": "text", "text": s} }

func tool(name string, input map[string]any) map[string]any {
	return map[string]any{"type": "tool_use", "name": name, "input": input}
}

func result() map[string]any {
	return map[string]any{"type": "user", "message": map[string]any{"content": []any{
		map[string]any{"type": "tool_result", "content": "output"}}}}
}

func TestActivity(t *testing.T) {
	plan := tool(toolPlan, map[string]any{"items": []any{
		map[string]any{"content": "Read the code", "status": "in_progress"},
		map[string]any{"content": "Fix the webhook retries", "status": "pending"},
	}})
	tests := []struct {
		name     string
		entries  []map[string]any
		status   agent.Status
		approval bool
		want     string
	}{
		{"working: the bash description", []map[string]any{
			prompt("run the tests"), reply(tool("Bash", map[string]any{"command": "go test ./...", "description": "Run helper tests"})),
		}, agent.StatusWorking, false, "Run helper tests"},
		{"working: editing a file", []map[string]any{
			prompt("fix it"), reply(tool("Edit", map[string]any{"file_path": "/Users/ann/code/payments-api/retry.go"})),
		}, agent.StatusWorking, false, "Editing retry.go"},
		{"working: an MCP tool", []map[string]any{
			prompt("look"), reply(tool("mcp__github__list_issues", nil)),
		}, agent.StatusWorking, false, "Using github list issues"},
		{"working: the plan step beats the tool", []map[string]any{
			prompt("go"), reply(plan), result(), reply(tool("Read", map[string]any{"file_path": "/a/b.go"})),
		}, agent.StatusWorking, false, "Read the code"},
		{"working: the plan advances", []map[string]any{
			prompt("go"), reply(plan), result(), reply(tool(toolPlan, map[string]any{"advance": true})),
		}, agent.StatusWorking, false, "Fix the webhook retries"},
		{"working: the plan starts a step out of order", []map[string]any{
			prompt("go"), reply(plan), result(), reply(tool(toolPlan, map[string]any{"done": []any{1}, "start": 2})),
		}, agent.StatusWorking, false, "Fix the webhook retries"},
		{"working: a finished plan falls back to the tool", []map[string]any{
			prompt("go"), reply(plan), result(), reply(tool(toolWrapUp, map[string]any{"recap": "Done."})),
			prompt("next"), reply(tool("Grep", nil)),
		}, agent.StatusWorking, false, "Searching the code"},
		{"working: a terminal todo list", []map[string]any{
			prompt("go"), reply(tool(toolTodo, map[string]any{"todos": []any{
				map[string]any{"content": "Write tests", "activeForm": "Writing tests", "status": "in_progress"}}})),
		}, agent.StatusWorking, false, "Writing tests"},
		{"approval: what it wants to run", []map[string]any{
			prompt("ship it"), reply(text("Pushing now."), tool("Bash", map[string]any{"description": "Push the branch"})),
		}, agent.StatusWaiting, true, "Approve: Push the branch"},
		{"approval: a question tool", []map[string]any{
			prompt("plan it"), reply(tool(toolQuestion, map[string]any{"questions": []any{
				map[string]any{"question": "Which database should the cache use?"}}})),
		}, agent.StatusWaiting, true, "Which database should the cache use?"},
		{"waiting: the app's end-of-turn question", []map[string]any{
			prompt("review"), reply(text("All checks pass.")), reply(tool(toolWrapUp, map[string]any{
				"recap": "Tests pass.", "ask": "Should I merge PR 5?"})),
		}, agent.StatusWaiting, false, "Should I merge PR 5?"},
		{"waiting: the question a reply ends with", []map[string]any{
			prompt("thoughts?"), reply(text("I found two options.\n\n**Option A** is faster. Which one do you want? Let me know.")),
		}, agent.StatusWaiting, false, "Which one do you want?"},
		{"waiting: otherwise the last sentence", []map[string]any{
			prompt("thanks"), reply(text("The fix is in `retry.go`. See [the PR](https://example.com/1) for details.")),
		}, agent.StatusWaiting, false, "See the PR for details."},
		{"waiting: code blocks and tables are skipped", []map[string]any{
			prompt("show"), reply(text("Run this:\n```sh\nmake test\n```\n| a | b |\n|---|---|\nDone.")),
		}, agent.StatusWaiting, false, "Done."},
		{"waiting: nothing since the prompt", []map[string]any{
			reply(text("Old answer?")), prompt("new question"),
		}, agent.StatusWaiting, false, ""},
		{"idle: none", []map[string]any{
			prompt("x"), reply(text("Done?")),
		}, agent.StatusIdle, false, ""},
		{"subagents are ignored", []map[string]any{
			prompt("go"), reply(tool("Bash", map[string]any{"description": "Main thread"})),
			{"type": "assistant", "isSidechain": true, "message": map[string]any{"content": []any{
				tool("Bash", map[string]any{"description": "Subagent work"})}}},
		}, agent.StatusWorking, false, "Main thread"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := activity(jsonl(t, tt.entries...), tt.status, tt.approval); got != tt.want {
				t.Errorf("activity = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestActivityIsBounded(t *testing.T) {
	long := strings.Repeat("word ", 100) + "end?"
	got := activity(jsonl(t, prompt("x"), reply(text(long))), agent.StatusWaiting, false)
	if len(got) > MaxActivityBytes {
		t.Errorf("activity is %d bytes, want at most %d", len(got), MaxActivityBytes)
	}
}

func TestActivityToleratesACutFirstLine(t *testing.T) {
	tail := append([]byte(`ent":"cut"}`+"\n"), jsonl(t, prompt("x"), reply(tool("Grep", nil)))...)
	if got := activity(tail, agent.StatusWorking, false); got != "Searching the code" {
		t.Errorf("activity = %q", got)
	}
}

func TestActivityOffByDefault(t *testing.T) {
	f := newFixture(t)
	f.addSession(7, map[string]any{"status": "busy"})
	f.addTranscript("session-7", prompt("go"), reply(tool("Bash", map[string]any{"description": "Run helper tests"})))
	if s := f.sessions(); len(s) != 1 || s[0].Activity != "" {
		t.Fatalf("sessions = %+v, want no activity unless asked", s)
	}
	f.source.Activity = true
	if s := f.sessions(); len(s) != 1 || s[0].Activity != "Run helper tests" {
		t.Fatalf("sessions = %+v, want the activity once asked", s)
	}
}

func TestActivityNeverCarriesPromptsOrToolOutput(t *testing.T) {
	const canary = "CANARY-7f3a"
	f := newFixture(t)
	f.source.Activity = true
	f.addSession(9, map[string]any{"status": "busy"})
	f.addTranscript("session-9",
		prompt(canary), reply(tool("Bash", map[string]any{"command": canary, "description": "Run helper tests"})),
		map[string]any{"type": "user", "toolUseResult": map[string]any{"stdout": canary},
			"message": map[string]any{"content": []any{map[string]any{"type": "tool_result", "content": canary}}}},
	)
	s := f.sessions()
	if len(s) != 1 || strings.Contains(s[0].Activity, canary) || s[0].Activity != "Run helper tests" {
		t.Fatalf("sessions = %+v: activity must not carry the prompt, command or output", s)
	}
}
