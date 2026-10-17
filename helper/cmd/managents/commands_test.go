package main

import (
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/tonylook/managents/helper/internal/agent"
	"github.com/tonylook/managents/helper/internal/device"
	"github.com/tonylook/managents/helper/internal/protocol"
)

func TestPrintStatus(t *testing.T) {
	now := time.Date(2026, 10, 9, 12, 0, 0, 0, time.UTC)
	home := filepath.FromSlash("/home/ann")
	inHome := filepath.FromSlash("/home/ann/src/atlas-api")
	elsewhere := filepath.FromSlash("/srv/web-shop")
	cards := []agent.Card{
		{Name: "atlas-api", Session: agent.Session{Kind: agent.KindClaude, Dir: inHome, Status: agent.StatusWorking,
			Since: now.Add(-12 * time.Second), Context: &agent.ContextUsage{Used: 88000, Limit: 200000}}},
		{Name: "web-shop", Session: agent.Session{Kind: agent.KindOpenCode, Dir: elsewhere, Status: agent.StatusWaiting,
			Since: now.Add(-5*time.Minute - 300*time.Millisecond)}},
	}
	want := fmt.Sprintf(""+
		"NAME       AGENT     STATUS   AGE   CONTEXT   FOLDER\n"+
		"atlas-api  claude    working  12s   88k/200k  %s\n"+
		"web-shop   opencode  waiting  5m0s  -         %s\n",
		filepath.FromSlash("~/src/atlas-api"), elsewhere)

	var out strings.Builder
	if err := printStatus(&out, cards, now, home); err != nil {
		t.Fatal(err)
	}
	if out.String() != want {
		t.Errorf("got:\n%s\nwant:\n%s", out.String(), want)
	}

	out.Reset()
	if err := printStatus(&out, nil, now, home); err != nil {
		t.Fatal(err)
	}
	if out.String() != "no open agent sessions\n" {
		t.Errorf("without sessions: %q", out.String())
	}
}

func TestContextText(t *testing.T) {
	tests := []struct {
		usage *agent.ContextUsage
		want  string
	}{
		{nil, "-"},
		{&agent.ContextUsage{Used: 88400, Limit: 200000}, "88k/200k"},
		{&agent.ContextUsage{Used: 477000, Limit: 1000000}, "477k/1000k"},
		{&agent.ContextUsage{Used: 31000}, "31k"}, // limit unknown
		{&agent.ContextUsage{Used: 999, Limit: 200000}, "0k/200k"},
	}
	for _, tt := range tests {
		if got := contextText(tt.usage); got != tt.want {
			t.Errorf("contextText(%+v) = %q, want %q", tt.usage, got, tt.want)
		}
	}
}

func TestShortPath(t *testing.T) {
	home := filepath.FromSlash("/home/ann")
	tests := []struct{ path, want string }{
		{"/home/ann/src/api", "~/src/api"},
		{"/home/ann", "~"},
		{"/home/anna/src", "/home/anna/src"},
		{"/srv/api", "/srv/api"},
	}
	for _, tt := range tests {
		if got := shortPath(filepath.FromSlash(tt.path), home); got != filepath.FromSlash(tt.want) {
			t.Errorf("shortPath(%q) = %q, want %q", tt.path, got, tt.want)
		}
	}
}

func TestDescribePort(t *testing.T) {
	const port = "/dev/cu.usbserial-10"
	hello := protocol.Hello{V: 1, T: "hello", Device: "managents", FW: "0.1.0", Board: "e32r40t", W: 480, H: 320, Proto: 1}
	old, newer, dev, behind := hello, hello, hello, hello
	old.FW = "0.0.9"
	newer.Proto = protocol.Version + 1
	dev.FW = "f91cdc5"
	behind.FW = "0.2.0"
	tests := []struct {
		name      string
		info      protocol.Hello
		err       error
		available string // the firmware this helper can flash
		want      string
	}{
		{"display", hello, nil, "",
			port + "\tmanagents display: board e32r40t, firmware 0.1.0, 480x320"},
		{"old firmware", old, nil, "0.3.0",
			port + "\tmanagents display: board e32r40t, firmware 0.0.9, 480x320\n" +
				"\tfirmware older than " + protocol.MinFirmware + ", update it with: managents flash"},
		{"newer protocol", newer, nil, "",
			port + "\tmanagents display: board e32r40t, firmware 0.1.0, 480x320\n" +
				"\tthe display speaks a newer protocol: update managents"},
		{"development build", dev, nil, "0.3.0",
			port + "\tmanagents display: board e32r40t, firmware f91cdc5, 480x320"},
		{"update available", behind, nil, "0.3.0",
			port + "\tmanagents display: board e32r40t, firmware 0.2.0, 480x320\n" +
				"\tfirmware 0.3.0 available: run managents flash"},
		{"up to date", behind, nil, "0.2.0",
			port + "\tmanagents display: board e32r40t, firmware 0.2.0, 480x320"},
		{"busy", protocol.Hello{}, fmt.Errorf("%s: %w", port, device.ErrPortBusy), "",
			port + "\tin use by another program (the managents service?)"},
		{"not a display", protocol.Hello{}, fmt.Errorf("%s: %w", port, device.ErrNotADisplay), "",
			port + "\tnot a managents display (new board? run: managents flash)"},
		{"other error", protocol.Hello{}, fmt.Errorf("%s: %w", port, errors.New("permission denied")), "",
			port + "\terror: permission denied"},
	}
	for _, tt := range tests {
		if got := describePort(port, tt.info, tt.err, tt.available); got != tt.want {
			t.Errorf("%s:\n%q\nwant\n%q", tt.name, got, tt.want)
		}
	}
}
