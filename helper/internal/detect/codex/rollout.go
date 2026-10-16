package codex

import (
	"bytes"
	"encoding/json"
	"time"

	"github.com/tonylook/managents/helper/internal/agent"
	"github.com/tonylook/managents/helper/internal/detect"
)

// Only the end of a rollout is scanned: rolloutTail bytes, growing fourfold
// up to maxRolloutTail when a huge entry (a large tool output) hides the start
// of the last turn or its last token count.
const (
	rolloutTail    = 256 << 10
	maxRolloutTail = 4 << 20
)

// turnState is where the newest turn of a rollout is.
type turnState int

const (
	noTurn turnState = iota
	turnRunning
	turnEnded
)

// rolloutSummary is what the end of a rollout says about its thread.
type rolloutSummary struct {
	turn turnState
	// at is when the newest turn started (running) or ended (ended).
	at time.Time
	// failed: the newest turn ended in an error.
	failed bool
	// openCall is when the running turn's newest response item, a tool call
	// still without its output, was written; zero otherwise.
	openCall time.Time
	context  *agent.ContextUsage
}

// entry declares only the metadata fields we use. Message content, tool
// arguments and outputs, and error messages are never decoded into memory we
// keep, logged or sent anywhere.
type entry struct {
	Timestamp time.Time `json:"timestamp"`
	Type      string    `json:"type"` // event_msg, response_item, ...
	Payload   struct {
		Type  string   `json:"type"`  // task_started, function_call, ...
		Error presence `json:"error"` // task_complete of a failed turn
		Info  *struct {
			LastTokenUsage *struct {
				TotalTokens int `json:"total_tokens"`
			} `json:"last_token_usage"`
			ModelContextWindow int `json:"model_context_window"`
		} `json:"info"` // token_count
	} `json:"payload"`
}

// presence records whether a JSON field holds a value, without keeping it.
type presence bool

func (p *presence) UnmarshalJSON(data []byte) error {
	*p = presence(!bytes.Equal(bytes.TrimSpace(data), []byte("null")))
	return nil
}

// event is the type of an event_msg entry, or "".
func (e entry) event() string {
	if e.Type != "event_msg" {
		return ""
	}
	return e.Payload.Type
}

// isToolCall reports a model's request to run a tool; its output is a
// separate, later item.
func (e entry) isToolCall() bool {
	if e.Type != "response_item" {
		return false
	}
	switch e.Payload.Type {
	case "function_call", "custom_tool_call", "local_shell_call":
		return true
	}
	return false
}

// contextUsage is the context-window fill reported by a token count: the
// tokens of the last model call, out of the model's window.
func (e entry) contextUsage() *agent.ContextUsage {
	if e.event() != "token_count" || e.Payload.Info == nil || e.Payload.Info.LastTokenUsage == nil {
		return nil
	}
	used := e.Payload.Info.LastTokenUsage.TotalTokens
	if used <= 0 {
		return nil
	}
	return &agent.ContextUsage{Used: used, Limit: e.Payload.Info.ModelContextWindow}
}

// summarizeRollout reads the end of a rollout (JSON lines).
func summarizeRollout(path string) (rolloutSummary, error) {
	for size := int64(rolloutTail); ; size *= 4 {
		tail, whole, err := detect.ReadTail(path, size)
		if err != nil {
			return rolloutSummary{}, err
		}
		summary, complete := summarize(tail)
		if complete || whole || size >= maxRolloutTail {
			return summary, nil
		}
	}
}

// summarize scans the lines of tail backwards, and reports whether it found
// everything it looks for: the start of the newest turn and a token count.
//
// The newest turn event decides the state: task_started is a running turn,
// task_complete and turn_aborted (an interruption) an ended one. A turn
// failed when its task_complete carries an error, or, in older Codex
// versions, when an error event came before it.
func summarize(tail []byte) (rolloutSummary, bool) {
	var summary rolloutSummary
	turnStartFound, seenResponse := false, false
	for rest := tail; len(rest) > 0 && !(turnStartFound && summary.context != nil); {
		i := bytes.LastIndexByte(rest, '\n')
		line := rest[i+1:]
		rest = rest[:max(i, 0)]

		var e entry
		if len(bytes.TrimSpace(line)) == 0 || json.Unmarshal(line, &e) != nil {
			continue
		}
		if summary.context == nil {
			summary.context = e.contextUsage()
		}
		if turnStartFound {
			continue
		}
		switch event := e.event(); {
		case event == "task_started":
			turnStartFound = true
			if summary.turn == noTurn {
				summary.turn, summary.at = turnRunning, e.Timestamp
			}
		case summary.turn != noTurn:
			summary.failed = summary.failed || (summary.turn == turnEnded && event == "error")
		case event == "task_complete":
			summary.turn, summary.at, summary.failed = turnEnded, e.Timestamp, bool(e.Payload.Error)
		case event == "turn_aborted":
			summary.turn, summary.at = turnEnded, e.Timestamp
		case e.Type == "response_item" && !seenResponse:
			// Still inside the running turn, if the turn turns out to run.
			seenResponse = true
			if e.isToolCall() {
				summary.openCall = e.Timestamp
			}
		}
	}
	if summary.turn != turnRunning {
		summary.openCall = time.Time{}
	}
	return summary, turnStartFound && summary.context != nil
}
