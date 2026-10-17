package claude

import (
	"bytes"
	"encoding/json"
	"errors"

	"github.com/tonylook/managents/helper/internal/agent"
	"github.com/tonylook/managents/helper/internal/detect"
)

// Only the end of a transcript is scanned: transcriptTail bytes, growing
// fourfold up to maxTranscriptTail when a huge entry (a large tool result)
// hides the last model call.
const (
	transcriptTail    = 256 << 10
	maxTranscriptTail = 4 << 20
)

var errNoEntries = errors.New("transcript has no readable entries")

type transcriptSummary struct {
	endsInError bool
	context     *agent.ContextUsage
}

// entry declares only the metadata fields we use. Message content is never
// decoded into memory we keep, logged or sent anywhere.
type entry struct {
	Type              string `json:"type"`
	IsSidechain       bool   `json:"isSidechain"`
	IsAPIErrorMessage bool   `json:"isApiErrorMessage"`
	Message           *struct {
		Usage *struct {
			InputTokens         int `json:"input_tokens"`
			CacheCreationTokens int `json:"cache_creation_input_tokens"`
			CacheReadTokens     int `json:"cache_read_input_tokens"`
		} `json:"usage"`
	} `json:"message"`
}

// isConversation excludes subagent traffic and the bookkeeping entries
// (system, last-prompt, ai-title, mode, ...) Claude Code appends after a turn.
func (e entry) isConversation() bool {
	return !e.IsSidechain && (e.Type == "user" || e.Type == "assistant")
}

// contextTokens is the context-window size of a model call. It is zero for
// entries without usage, and for synthetic messages such as API errors.
func (e entry) contextTokens() int {
	if e.Type != "assistant" || e.Message == nil || e.Message.Usage == nil {
		return 0
	}
	u := e.Message.Usage
	return u.InputTokens + u.CacheCreationTokens + u.CacheReadTokens
}

// summarizeTranscript reads the end of a session transcript (JSON lines) to
// find whether the conversation ends in an API error, and how many tokens the
// last main-thread model call had in its context window.
func summarizeTranscript(path string) (transcriptSummary, error) {
	for size := int64(transcriptTail); ; size *= 4 {
		tail, whole, err := detect.ReadTail(path, size)
		if err != nil {
			return transcriptSummary{}, err
		}
		summary, err := summarize(tail)
		if (err == nil && summary.context != nil) || whole || size >= maxTranscriptTail {
			return summary, err
		}
	}
}

// summarize scans the lines of tail backwards. The last conversation entry
// decides whether the session ended in an error; the context comes from the
// last model call that reported a usage.
func summarize(tail []byte) (transcriptSummary, error) {
	var summary transcriptSummary
	decided := false
	for rest := tail; len(rest) > 0; {
		i := bytes.LastIndexByte(rest, '\n')
		line := rest[i+1:]
		rest = rest[:max(i, 0)]

		var e entry
		if len(bytes.TrimSpace(line)) == 0 || json.Unmarshal(line, &e) != nil || !e.isConversation() {
			continue
		}
		if !decided {
			decided = true
			summary.endsInError = e.IsAPIErrorMessage
		}
		if used := e.contextTokens(); used > 0 {
			summary.context = &agent.ContextUsage{Used: used}
			break
		}
	}
	if !decided {
		return transcriptSummary{}, errNoEntries
	}
	return summary, nil
}
