// Package protocol implements the managents serial protocol v1: JSON objects,
// one per line, exchanged with the display over USB serial.
// The specification is docs/protocol.md; firmware/lib/core is the other side.
package protocol

import (
	"encoding/json"
	"errors"
	"fmt"
	"time"
	"unicode/utf8"

	"github.com/tonylook/managents/helper/internal/agent"
)

// Version is the protocol version this helper speaks.
const Version = 1

// DeviceName is what a managents display reports in its hello message.
const DeviceName = "managents"

// MaxAgents is the most agents a state frame carries; the rest are counted in
// More. The display pages through them nine at a time.
const MaxAgents = 24

// MaxIDBytes bounds an agent id, because the display keeps no more of it.
// Longer ids are cut the way the display would cut them.
const MaxIDBytes = 48

// MaxNameBytes bounds a card name; the display truncates further to fit.
const MaxNameBytes = 64

// MaxLineLength is the longest line a display accepts, newline excluded.
const MaxLineLength = 4096

// State is the host -> device frame describing every open session.
type State struct {
	V      int          `json:"v"`
	T      string       `json:"t"`
	Now    int64        `json:"now"`
	TZ     int          `json:"tz"`
	Agents []AgentEntry `json:"agents"`
	More   int          `json:"more,omitempty"`
}

// AgentEntry is one card on the display. It carries the folder name but no
// path: full directories stay on this computer.
type AgentEntry struct {
	ID     string   `json:"id"`
	Kind   string   `json:"kind"`
	Name   string   `json:"name"`
	Status string   `json:"status"`
	Age    int64    `json:"age"`
	Ctx    *Context `json:"ctx,omitempty"`
	// Title is set only for network displays that asked for titles; the
	// display falls back to Name without it.
	Title string `json:"title,omitempty"`
	// Activity, like Title, is only for network displays that asked for it.
	Activity string `json:"activity,omitempty"`
}

// Context is a session's context-window usage. Limit is null when unknown.
type Context struct {
	Used  int  `json:"used"`
	Limit *int `json:"limit"`
}

// Hello is exchanged in both directions: as a probe from the host, and as the
// device's identification.
type Hello struct {
	V      int    `json:"v"`
	T      string `json:"t"`
	Device string `json:"device,omitempty"`
	FW     string `json:"fw,omitempty"`
	Board  string `json:"board,omitempty"`
	W      int    `json:"w,omitempty"`
	H      int    `json:"h,omitempty"`
	Proto  int    `json:"proto,omitempty"`
}

// NewState builds the frame for the given cards at time now. Cards beyond
// MaxAgents are counted in More.
func NewState(cards []agent.Card, now time.Time) State {
	_, offset := now.Zone()
	state := State{V: Version, T: "state", Now: now.Unix(), TZ: offset, Agents: []AgentEntry{}}
	for i, card := range cards {
		if i == MaxAgents {
			state.More = len(cards) - MaxAgents
			break
		}
		state.Agents = append(state.Agents, entryFor(card, now))
	}
	// Activity lines and titles are optional, so they go before cards do,
	// least active first.
	for i := len(state.Agents) - 1; i >= 0 && !fitsOneLine(state); i-- {
		state.Agents[i].Activity = ""
	}
	for i := len(state.Agents) - 1; i >= 0 && !fitsOneLine(state); i-- {
		state.Agents[i].Title = ""
	}
	for !fitsOneLine(state) && len(state.Agents) > 0 {
		state.Agents = state.Agents[:len(state.Agents)-1] // the least active go first
		state.More++
	}
	return state
}

func fitsOneLine(state State) bool {
	line, err := json.Marshal(state)
	return err == nil && len(line) <= MaxLineLength
}

func entryFor(card agent.Card, now time.Time) AgentEntry {
	entry := AgentEntry{
		ID:       truncateUTF8(card.ID, MaxIDBytes),
		Kind:     string(card.Kind),
		Name:     truncateUTF8(card.Name, MaxNameBytes),
		Status:   string(card.Status),
		Age:      int64(card.Age(now) / time.Second),
		Title:    card.Title,
		Activity: card.Activity,
	}
	if card.Context != nil {
		entry.Ctx = &Context{Used: card.Context.Used}
		if card.Context.Limit > 0 {
			limit := card.Context.Limit
			entry.Ctx.Limit = &limit
		}
	}
	return entry
}

// truncateUTF8 cuts s to at most n bytes without splitting a character.
func truncateUTF8(s string, n int) string {
	if len(s) <= n {
		return s
	}
	for n > 0 && !utf8.RuneStart(s[n]) {
		n--
	}
	return s[:n]
}

// HelloProbe is the host's hello.
func HelloProbe() Hello { return Hello{V: Version, T: "hello"} }

// Encode serialises a message as one protocol line, newline included.
func Encode(message any) ([]byte, error) {
	line, err := json.Marshal(message)
	if err != nil {
		return nil, err
	}
	if len(line) > MaxLineLength {
		return nil, fmt.Errorf("%w: %d bytes", ErrLineTooLong, len(line))
	}
	return append(line, '\n'), nil
}

// ErrLineTooLong is returned when a message cannot fit in one protocol line.
var ErrLineTooLong = errors.New("protocol line too long")

// ParseDeviceHello decodes a device -> host line and returns the hello it
// carries, or false if the line is anything else (boot log, other messages).
func ParseDeviceHello(line []byte) (Hello, bool) {
	var hello Hello
	if json.Unmarshal(line, &hello) != nil {
		return Hello{}, false
	}
	if hello.V != Version || hello.T != "hello" || hello.Device != DeviceName {
		return Hello{}, false
	}
	return hello, true
}
