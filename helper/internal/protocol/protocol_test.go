package protocol

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/santhosh-tekuri/jsonschema/v6"

	"github.com/tonylook/managents/helper/internal/agent"
)

// The shared contract lives at the repository root, next to the firmware.
const protocolDir = "../../../protocol"

var now = time.Date(2026, 10, 9, 16, 30, 0, 0, time.FixedZone("CEST", 2*3600))

func compileSchema(t *testing.T, name string) *jsonschema.Schema {
	t.Helper()
	compiler := jsonschema.NewCompiler()
	schema, err := compiler.Compile(filepath.Join(protocolDir, "schema", name))
	if err != nil {
		t.Fatal(err)
	}
	return schema
}

func validate(t *testing.T, schema *jsonschema.Schema, line []byte) error {
	t.Helper()
	value, err := jsonschema.UnmarshalJSON(bytes.NewReader(line))
	if err != nil {
		return err
	}
	return schema.Validate(value)
}

func fixtureLines(t *testing.T, pattern string) []string {
	t.Helper()
	paths, err := filepath.Glob(filepath.Join(protocolDir, "fixtures", pattern))
	if err != nil || len(paths) == 0 {
		t.Fatalf("no fixtures match %s", pattern)
	}
	var lines []string
	for _, path := range paths {
		f, err := os.Open(path)
		if err != nil {
			t.Fatal(err)
		}
		scanner := bufio.NewScanner(f)
		for scanner.Scan() {
			if line := strings.TrimSpace(scanner.Text()); line != "" {
				lines = append(lines, line)
			}
		}
		f.Close()
	}
	return lines
}

func sampleCards() []agent.Card {
	return []agent.Card{
		{
			Name: "managents",
			Session: agent.Session{
				ID: "claude:1", Kind: agent.KindClaude, Dir: "/w/managents", Status: agent.StatusWorking,
				Since: now.Add(-42 * time.Second), Context: &agent.ContextUsage{Used: 88000, Limit: 200000},
			},
		},
		{
			Name: "core",
			Session: agent.Session{
				ID: "opencode:2", Kind: agent.KindOpenCode, Dir: "/w/core", Status: agent.StatusIdle,
				Since: now.Add(-3 * time.Hour), Context: &agent.ContextUsage{Used: 1000},
			},
		},
	}
}

func TestValidFixturesMatchSchema(t *testing.T) {
	schema := compileSchema(t, "host-message.schema.json")
	for _, line := range fixtureLines(t, "valid/*.jsonl") {
		if err := validate(t, schema, []byte(line)); err != nil {
			t.Errorf("valid fixture rejected: %s\n%v", line, err)
		}
	}
}

func TestInvalidFixturesFailSchema(t *testing.T) {
	schema := compileSchema(t, "host-message.schema.json")
	for _, line := range fixtureLines(t, "invalid/*.jsonl") {
		if err := validate(t, schema, []byte(line)); err == nil {
			t.Errorf("invalid fixture accepted: %s", line)
		}
	}
}

func TestNewStateMatchesSchema(t *testing.T) {
	schema := compileSchema(t, "host-message.schema.json")
	line, err := Encode(NewState(sampleCards(), now))
	if err != nil {
		t.Fatal(err)
	}
	if err := validate(t, schema, line); err != nil {
		t.Errorf("NewState output violates the schema: %v\n%s", err, line)
	}
}

func TestNewStateContent(t *testing.T) {
	state := NewState(sampleCards(), now)

	if state.Now != now.Unix() || state.TZ != 7200 {
		t.Errorf("now/tz = %d/%d", state.Now, state.TZ)
	}
	first, second := state.Agents[0], state.Agents[1]
	if first.Age != 42 || first.Status != "working" || first.Ctx.Used != 88000 || *first.Ctx.Limit != 200000 {
		t.Errorf("first agent = %+v", first)
	}
	if second.Status != "idle" || second.Ctx.Limit != nil {
		t.Errorf("second agent = %+v (limit must be null when unknown)", second)
	}
}

func TestNewStateCapsAgents(t *testing.T) {
	cards := make([]agent.Card, MaxAgents+3)
	for i := range cards {
		cards[i] = agent.Card{Name: "a", Session: agent.Session{ID: "claude:1", Kind: agent.KindClaude, Status: agent.StatusWaiting}}
	}
	state := NewState(cards, now)
	if len(state.Agents) != MaxAgents || state.More != 3 {
		t.Errorf("agents=%d more=%d, want %d and 3", len(state.Agents), state.More, MaxAgents)
	}
}

// longCard has an id and a name longer than the display keeps.
func longCard() agent.Card {
	return agent.Card{
		Name: strings.Repeat("é", 100),
		Session: agent.Session{
			ID: "opencode:" + strings.Repeat("9", 100), Kind: agent.KindOpenCode, Status: agent.StatusWorking,
		},
	}
}

func TestNewStateTruncatesLongFields(t *testing.T) {
	entry := NewState([]agent.Card{longCard()}, now).Agents[0]

	if len(entry.ID) != MaxIDBytes {
		t.Errorf("id is %d bytes, want %d", len(entry.ID), MaxIDBytes)
	}
	if len(entry.Name) != MaxNameBytes || !utf8.ValidString(entry.Name) {
		t.Errorf("name %q, want %d bytes cut between characters", entry.Name, MaxNameBytes)
	}
}

func TestNewStateSendsNoPaths(t *testing.T) {
	card := agent.Card{Name: "api", Session: agent.Session{
		ID: "claude:1", Kind: agent.KindClaude, Dir: "/Users/ann/clients/api", Status: agent.StatusIdle,
	}}
	line, err := Encode(NewState([]agent.Card{card}, now))
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(line, []byte("/Users/ann")) || bytes.Contains(line, []byte(`"path"`)) {
		t.Errorf("the frame leaks the working directory: %s", line)
	}
}

func TestNewStateMovesAgentsToMoreAsLastResort(t *testing.T) {
	cards := make([]agent.Card, MaxAgents)
	for i := range cards {
		cards[i] = longCard()
	}
	state := NewState(cards, now)
	if _, err := Encode(state); err != nil {
		t.Fatal(err)
	}
	if state.More == 0 || len(state.Agents)+state.More != MaxAgents {
		t.Errorf("agents=%d more=%d", len(state.Agents), state.More)
	}
}

func TestEncodeRejectsOverlongLines(t *testing.T) {
	if _, err := Encode(strings.Repeat("x", MaxLineLength)); !errors.Is(err, ErrLineTooLong) {
		t.Errorf("err = %v, want ErrLineTooLong", err)
	}
}

func TestEmptyStateHasEmptyArray(t *testing.T) {
	line, err := Encode(NewState(nil, now))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(line, []byte(`"agents":[]`)) || line[len(line)-1] != '\n' {
		t.Errorf("line = %q", line)
	}
}

func TestParseDeviceHello(t *testing.T) {
	schema := compileSchema(t, "device-message.schema.json")
	line := []byte(`{"v":1,"t":"hello","device":"managents","fw":"0.1.0","board":"e32r40t","w":480,"h":320,"proto":1}`)
	if err := validate(t, schema, line); err != nil {
		t.Fatalf("sample device hello violates the schema: %v", err)
	}

	hello, ok := ParseDeviceHello(line)
	if !ok || hello.W != 480 || hello.Board != "e32r40t" {
		t.Errorf("ParseDeviceHello = %+v, %v", hello, ok)
	}
	for _, other := range []string{
		"ets Jun  8 2016 00:22:57",
		`{"v":1,"t":"hello"}`,
		`{"v":1,"t":"hello","device":"other"}`,
		`{"v":2,"t":"hello","device":"managents"}`,
	} {
		if _, ok := ParseDeviceHello([]byte(other)); ok {
			t.Errorf("accepted %q", other)
		}
	}
}

func TestTitlesGoBeforeCards(t *testing.T) {
	var cards []agent.Card
	for i := range MaxAgents {
		cards = append(cards, agent.Card{
			Session: agent.Session{ID: fmt.Sprintf("claude:%d", i), Kind: agent.KindClaude, Status: agent.StatusIdle,
				Since: now, Title: strings.Repeat("t", 150)},
			Name: fmt.Sprintf("project-%d", i),
		})
	}
	state := NewState(cards, now)
	if len(state.Agents) != MaxAgents || state.More != 0 {
		t.Fatalf("%d agents, %d more: titles must be dropped before any card", len(state.Agents), state.More)
	}
	if state.Agents[0].Title == "" || state.Agents[MaxAgents-1].Title != "" {
		t.Errorf("first title %q, last %q: the least active lose theirs first",
			state.Agents[0].Title, state.Agents[MaxAgents-1].Title)
	}
	if !fitsOneLine(state) {
		t.Error("frame does not fit one line")
	}
}
