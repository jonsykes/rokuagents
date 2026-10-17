package main

import (
	"runtime"
	"strings"
	"testing"

	"github.com/tonylook/managents/helper/internal/service"
)

func TestPrintServiceStatus(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("the service is not supported on Windows; the expected paths are Unix ones")
	}
	mac := service.Env{GOOS: "darwin", Home: "/Users/ann"}
	linux := service.Env{GOOS: "linux", Home: "/home/ann"}
	installed := service.State{
		Installed: true, Loaded: true, Running: true, PID: 4242,
		Program: "/Users/ann/.local/bin/managents",
		File:    "/Users/ann/Library/LaunchAgents/com.github.tonylook.managents.plist",
		LogPath: "/Users/ann/Library/Logs/managents.log",
	}
	crashing := installed
	crashing.Running, crashing.PID, crashing.LastExit = false, 0, "2"
	stopped := installed
	stopped.Loaded, stopped.Running, stopped.PID, stopped.Program = false, false, 0, ""
	onLinux := service.State{
		Installed: true, Loaded: true, Program: "/home/ann/.local/bin/managents",
		File: "/home/ann/.config/systemd/user/managents.service",
	}
	log := logSummary{
		warnings: []string{`level=WARN msg="display firmware is out of date" firmware=0.0.9`},
		tail:     []string{`level=INFO msg="managents started"`, `level=INFO msg="display connected"`},
	}

	tests := []struct {
		name           string
		env            service.Env
		state          service.State
		programMissing bool
		log            logSummary
		want           string
	}{
		{"not installed", mac, service.State{File: installed.File, LogPath: installed.LogPath}, false, logSummary{}, `
managents service: not installed (install it with: managents service install)
`},
		{"running", mac, installed, false, log, `
managents service: running (pid 4242)
  program  /Users/ann/.local/bin/managents
  file     ~/Library/LaunchAgents/com.github.tonylook.managents.plist
  log      ~/Library/Logs/managents.log

Warnings since the latest start:
  level=WARN msg="display firmware is out of date" firmware=0.0.9

Latest log lines:
  level=INFO msg="managents started"
  level=INFO msg="display connected"
`},
		{"program gone", mac, crashing, true, logSummary{}, `
managents service: not running, last exit status 2 (see the log)
If it does not stay running, allow managents in System Settings > General > Login Items & Extensions.
  program  /Users/ann/.local/bin/managents (missing: install managents again, then run: managents service install)
  file     ~/Library/LaunchAgents/com.github.tonylook.managents.plist
  log      ~/Library/Logs/managents.log
`},
		{"stopped", mac, stopped, false, logSummary{}, `
managents service: stopped (start it with: managents service start)
If it does not stay running, allow managents in System Settings > General > Login Items & Extensions.
  program  -
  file     ~/Library/LaunchAgents/com.github.tonylook.managents.plist
  log      ~/Library/Logs/managents.log
`},
		{"stopped on Linux", linux, onLinux, false, logSummary{}, `
managents service: stopped (start it with: managents service start)
  program  /home/ann/.local/bin/managents
  file     ~/.config/systemd/user/managents.service
  log      journalctl --user -u managents
`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var out strings.Builder
			if err := printServiceStatus(&out, tt.env, tt.state, tt.programMissing, tt.log); err != nil {
				t.Fatal(err)
			}
			if want := strings.TrimPrefix(tt.want, "\n"); out.String() != want {
				t.Errorf("got:\n%s\nwant:\n%s", out.String(), want)
			}
		})
	}
}
