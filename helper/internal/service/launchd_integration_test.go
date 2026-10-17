//go:build darwin

package service

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// TestLaunchdForReal drives the real launchctl through a whole lifecycle. It
// loads a LaunchAgent into the user's session, so it runs only when asked:
//
//	MANAGENTS_LAUNCHD_IT=1 go test -run TestLaunchdForReal ./internal/service
//
// The agent has its own label and runs a shell that only sleeps, so it never
// meets an installed managents service or a display, and it is removed even
// when the test fails.
func TestLaunchdForReal(t *testing.T) {
	if os.Getenv("MANAGENTS_LAUNCHD_IT") != "1" {
		t.Skip("set MANAGENTS_LAUNCHD_IT=1 to load a test LaunchAgent")
	}
	dir := t.TempDir()
	l := newLaunchd(Env{GOOS: "darwin", Home: dir, UID: os.Getuid()}, Exec{})
	l.label = Label + ".integration-test"
	l.plistPath = filepath.Join(dir, l.label+".plist")
	l.logPath = filepath.Join(dir, "test.log")
	l.args = []string{"-c", "echo started >&2; exec sleep 600"}
	t.Cleanup(func() {
		if err := l.Uninstall(); err != nil {
			t.Errorf("cleanup: %v (remove it with: launchctl bootout %s)", err, l.service())
		}
	})

	if err := l.Install("/bin/sh"); err != nil {
		t.Fatal(err)
	}
	first := waitUntilRunning(t, l)
	if log, _ := os.ReadFile(l.logPath); !strings.Contains(string(log), "started") {
		t.Errorf("log = %q, want the job's stderr", log)
	}

	if err := l.Install("/bin/sh"); err != nil { // replaces the running job
		t.Fatal(err)
	}
	if again := waitUntilRunning(t, l); again.PID == first.PID {
		t.Errorf("re-install kept pid %d running", first.PID)
	}

	if err := l.Stop(); err != nil {
		t.Fatal(err)
	}
	if state, err := l.Status(); err != nil || state.Loaded || !state.Installed {
		t.Errorf("after Stop: %+v, %v; want installed and not loaded", state, err)
	}
	if err := l.Start(); err != nil {
		t.Fatal(err)
	}
	waitUntilRunning(t, l)

	if err := l.Uninstall(); err != nil {
		t.Fatal(err)
	}
	if state, err := l.Status(); err != nil || state.Installed || state.Loaded {
		t.Errorf("after Uninstall: %+v, %v", state, err)
	}
	assertGone(t, l.plistPath, l.logPath)
}

func waitUntilRunning(t *testing.T, l *launchd) State {
	t.Helper()
	var state State
	var err error
	for deadline := time.Now().Add(5 * time.Second); time.Now().Before(deadline); time.Sleep(100 * time.Millisecond) {
		if state, err = l.Status(); err == nil && state.Running && state.PID > 0 {
			return state
		}
	}
	t.Fatalf("not running after 5 s: %+v, %v", state, err)
	return state
}
