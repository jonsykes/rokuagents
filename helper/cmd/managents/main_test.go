package main

import (
	"bytes"
	"context"
	"runtime/debug"
	"strings"
	"testing"
)

func TestRunExitCodes(t *testing.T) {
	tests := []struct {
		args       []string
		wantCode   int
		wantStdout string // a part of stdout ("" = stdout must be empty)
		wantStderr string // a part of stderr ("" = stderr must be empty)
	}{
		{nil, exitOK, "Commands:", ""},
		{[]string{"help"}, exitOK, "Commands:", ""},
		{[]string{"-h"}, exitOK, "Commands:", ""},
		{[]string{"--help"}, exitOK, "Commands:", ""},
		{[]string{"version"}, exitOK, "managents dev", ""},
		{[]string{"--version"}, exitOK, "managents dev", ""},
		{[]string{"status", "-h"}, exitOK, "", "managents status: print the agent sessions"},
		{[]string{"service", "--help"}, exitOK, "", "Actions:"},
		{[]string{"service", "stop", "-h"}, exitOK, "", "managents service stop: stop the service"},
		{[]string{"bogus"}, exitUsage, "", `unknown command "bogus"`},
		{[]string{"--port", "/dev/x"}, exitUsage, "", `unknown command "--port"`},
		{[]string{"status", "--bogus"}, exitUsage, "", "flag provided but not defined: -bogus"},
		{[]string{"status", "extra"}, exitUsage, "", `unexpected argument "extra"`},
		{[]string{"devices", "extra"}, exitUsage, "", `unexpected argument "extra"`},
		{[]string{"version", "extra"}, exitUsage, "", `unexpected argument "extra"`},
		{[]string{"--version", "extra"}, exitUsage, "", `unexpected argument "extra"`},
		{[]string{"demo", "--hold", "0s"}, exitUsage, "", "must be positive"},
		{[]string{"service"}, exitUsage, "", "Actions:"},
		{[]string{"service", "bogus"}, exitUsage, "", `unknown service action "bogus"`},
		{[]string{"service", "status", "extra"}, exitUsage, "", `unexpected argument "extra"`},
		{[]string{"run", "--log-level", "loud"}, exitFailure, "", `managents: invalid log level "loud"`},
	}
	for _, tt := range tests {
		t.Run(strings.Join(tt.args, " "), func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			code := run(context.Background(), tt.args, &stdout, &stderr)
			if code != tt.wantCode {
				t.Errorf("exit code %d, want %d", code, tt.wantCode)
			}
			for _, out := range []struct {
				name, got, want string
			}{{"stdout", stdout.String(), tt.wantStdout}, {"stderr", stderr.String(), tt.wantStderr}} {
				if out.want == "" && out.got != "" || !strings.Contains(out.got, out.want) {
					t.Errorf("%s = %q, want it to contain %q", out.name, out.got, out.want)
				}
			}
			if n := strings.Count(stderr.String(), tt.wantStderr); tt.wantStderr != "" && n > 1 {
				t.Errorf("stderr repeats %q %d times", tt.wantStderr, n)
			}
		})
	}
}

func TestHelpListsEveryCommand(t *testing.T) {
	var stdout bytes.Buffer
	run(context.Background(), []string{"help"}, &stdout, &bytes.Buffer{})
	for _, cmd := range commands() {
		if !strings.Contains(stdout.String(), "  "+cmd.name+" ") {
			t.Errorf("help does not list %q:\n%s", cmd.name, stdout.String())
		}
	}
}

func TestResolveVersion(t *testing.T) {
	built := func(v string) *debug.BuildInfo { return &debug.BuildInfo{Main: debug.Module{Version: v}} }
	tests := []struct {
		name   string
		linked string
		info   *debug.BuildInfo
		want   string
	}{
		{"release build", "v0.2.0", built("(devel)"), "v0.2.0"},
		{"go install of a tag", "dev", built("v0.2.0"), "v0.2.0"},
		{"go install of a commit", "dev", built("v0.0.0-20261009120000-abcdef123456"), "v0.0.0-20261009120000-abcdef123456"},
		{"go build in a checkout", "dev", built("(devel)"), "dev"},
		{"no build info", "dev", nil, "dev"},
	}
	for _, tt := range tests {
		if got := resolveVersion(tt.linked, tt.info); got != tt.want {
			t.Errorf("%s: %q, want %q", tt.name, got, tt.want)
		}
	}
}
