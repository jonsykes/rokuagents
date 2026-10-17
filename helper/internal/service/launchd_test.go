package service

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

const (
	testExecutable = "/Users/ann/.local/bin/managents"
	plistName      = "com.github.tonylook.managents.plist"
)

// newTestLaunchd returns the launchd Manager of a user whose home is a
// temporary directory, with launchctl faked by cmd.
func newTestLaunchd(t *testing.T, cmd *fakeCommander) (*launchd, string) {
	t.Helper()
	home := t.TempDir()
	l := newLaunchd(Env{GOOS: "darwin", Home: home, UID: 501}, cmd)
	l.sleep = func(time.Duration) {}
	return l, home
}

// notLoaded answers launchctl like a domain without the job.
func notLoaded(argv []string) (string, error) {
	switch argv[1] {
	case "bootout":
		return "Boot-out failed: 3: No such process", exitWith(launchctlNoSuchProcess)
	case "print":
		return "Could not find service", exitWith(launchctlNotFound)
	}
	return "", nil
}

func TestPlistGolden(t *testing.T) {
	plist, err := render("launchd.plist.tmpl", job{
		Label:   Label,
		Program: []string{testExecutable, "run", "--log-file", "/Users/ann/Library/Logs/managents.log"},
		LogPath: "/Users/ann/Library/Logs/managents.log",
	})
	if err != nil {
		t.Fatal(err)
	}
	assertGolden(t, "managents.plist", plist)
}

// TestPlistIsValid has plutil check a plist whose paths need XML escaping.
func TestPlistIsValid(t *testing.T) {
	plutil, err := exec.LookPath("plutil")
	if err != nil {
		t.Skip("plutil not found (macOS only)")
	}
	plist, err := render("launchd.plist.tmpl", job{
		Label:   Label,
		Program: []string{`/Users/ann/A&B <tools>/managents`, "run"},
		LogPath: `/Users/ann/Library/Logs/"quoted".log`,
	})
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), plistName)
	if err := os.WriteFile(path, plist, 0o644); err != nil {
		t.Fatal(err)
	}
	if out, err := exec.Command(plutil, "-lint", path).CombinedOutput(); err != nil {
		t.Errorf("plutil -lint: %v\n%s\n%s", err, out, plist)
	}
	out, err := exec.Command(plutil, "-extract", "ProgramArguments.0", "raw", path).Output()
	if got := strings.TrimSpace(string(out)); err != nil || got != `/Users/ann/A&B <tools>/managents` {
		t.Errorf("ProgramArguments.0 = %q (%v)", got, err)
	}
}

func TestLaunchdInstall(t *testing.T) {
	cmd := &fakeCommander{reply: notLoaded}
	l, home := newTestLaunchd(t, cmd)
	plistPath := filepath.Join(home, "Library", "LaunchAgents", plistName)
	logPath := filepath.Join(home, "Library", "Logs", "managents.log")

	if err := l.Install(testExecutable); err != nil {
		t.Fatal(err)
	}
	assertCalls(t, cmd,
		"launchctl bootout gui/501/com.github.tonylook.managents",
		"launchctl enable gui/501/com.github.tonylook.managents",
		"launchctl bootstrap gui/501 "+plistPath)
	want, err := render("launchd.plist.tmpl", job{
		Label:   Label,
		Program: []string{testExecutable, "run", "--log-file", logPath},
		LogPath: logPath,
	})
	if err != nil {
		t.Fatal(err)
	}
	first := readFile(t, plistPath)
	if first != string(want) {
		t.Errorf("plist:\n%s\nwant:\n%s", first, want)
	}

	// Installing again, with the job now loaded, does the same.
	cmd.reply = nil
	if err := l.Install(testExecutable); err != nil {
		t.Fatal(err)
	}
	assertCalls(t, cmd,
		"launchctl bootout gui/501/com.github.tonylook.managents",
		"launchctl enable gui/501/com.github.tonylook.managents",
		"launchctl bootstrap gui/501 "+plistPath)
	if again := readFile(t, plistPath); again != first {
		t.Errorf("re-install changed the plist:\n%s", again)
	}
}

func TestLaunchdInstallFailsWhenLaunchctlDoes(t *testing.T) {
	cmd := &fakeCommander{reply: func(argv []string) (string, error) {
		if argv[1] == "bootout" {
			return "Boot-out failed: 1: Operation not permitted", exitWith(1)
		}
		return "", nil
	}}
	l, _ := newTestLaunchd(t, cmd)
	if err := l.Install(testExecutable); exitCode(err) != 1 {
		t.Errorf("Install = %v, want the bootout error", err)
	}
	assertCalls(t, cmd, "launchctl bootout gui/501/com.github.tonylook.managents")
}

func TestLaunchdBootstrapRetriesWhileTheOldInstanceExits(t *testing.T) {
	tests := []struct {
		name      string
		failures  int // bootstrap answers error 5 this many times
		wantCalls int
		wantErr   bool
	}{
		{"at once", 0, 1, false},
		{"after two retries", 2, 3, false},
		{"gives up", 100, bootstrapAttempts, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			bootstraps := 0
			cmd := &fakeCommander{reply: func(argv []string) (string, error) {
				if argv[1] != "bootstrap" {
					return notLoaded(argv)
				}
				if bootstraps++; bootstraps <= tt.failures {
					return "Bootstrap failed: 5: Input/output error", exitWith(launchctlIOError)
				}
				return "", nil
			}}
			l, _ := newTestLaunchd(t, cmd)
			slept := 0
			l.sleep = func(time.Duration) { slept++ }

			err := l.Install(testExecutable)
			if bootstraps != tt.wantCalls || slept != tt.wantCalls-1 || (err != nil) != tt.wantErr {
				t.Errorf("%d bootstraps, %d sleeps, err %v; want %d bootstraps, error %v",
					bootstraps, slept, err, tt.wantCalls, tt.wantErr)
			}
			if tt.wantErr && (exitCode(err) != launchctlIOError || !strings.Contains(err.Error(), "Login Items")) {
				t.Errorf("error %v, want launchctl's error 5 and the Login Items hint", err)
			}
		})
	}
}

func TestLaunchdUninstallRemovesThePlistAndTheLogs(t *testing.T) {
	cmd := &fakeCommander{reply: notLoaded}
	l, home := newTestLaunchd(t, cmd)
	if err := l.Install(testExecutable); err != nil {
		t.Fatal(err)
	}
	logPath := filepath.Join(home, "Library", "Logs", "managents.log")
	touch(t, logPath, logPath+".1")
	cmd.reply = nil
	cmd.takeCalls()

	if err := l.Uninstall(); err != nil {
		t.Fatal(err)
	}
	assertCalls(t, cmd, "launchctl bootout gui/501/com.github.tonylook.managents")
	assertGone(t, filepath.Join(home, "Library", "LaunchAgents", plistName), logPath, logPath+".1")

	// Nothing left to remove is not an error.
	cmd.reply = notLoaded
	if err := l.Uninstall(); err != nil {
		t.Errorf("second Uninstall: %v", err)
	}
}

func TestLaunchdStartAndStop(t *testing.T) {
	cmd := &fakeCommander{reply: notLoaded}
	l, home := newTestLaunchd(t, cmd)
	plistPath := filepath.Join(home, "Library", "LaunchAgents", plistName)
	if err := l.Start(); !errors.Is(err, ErrNotInstalled) {
		t.Errorf("Start before Install = %v, want ErrNotInstalled", err)
	}
	if err := l.Stop(); !errors.Is(err, ErrNotInstalled) {
		t.Errorf("Stop before Install = %v, want ErrNotInstalled", err)
	}
	assertCalls(t, cmd, "launchctl print gui/501/com.github.tonylook.managents")

	if err := l.Install(testExecutable); err != nil {
		t.Fatal(err)
	}
	cmd.takeCalls()
	if err := l.Stop(); err != nil {
		t.Fatal(err)
	}
	assertCalls(t, cmd, "launchctl bootout gui/501/com.github.tonylook.managents")
	if _, err := os.Stat(plistPath); err != nil {
		t.Errorf("Stop removed the plist: %v", err)
	}

	if err := l.Start(); err != nil {
		t.Fatal(err)
	}
	assertCalls(t, cmd,
		"launchctl print gui/501/com.github.tonylook.managents",
		"launchctl bootstrap gui/501 "+plistPath)

	cmd.reply = nil // loaded: launchd already keeps it running
	if err := l.Start(); err != nil {
		t.Fatal(err)
	}
	assertCalls(t, cmd, "launchctl print gui/501/com.github.tonylook.managents")
}

func TestLaunchdStatus(t *testing.T) {
	running, err := os.ReadFile(filepath.Join("testdata", "launchctl-print-running.txt"))
	if err != nil {
		t.Fatal(err)
	}
	cmd := &fakeCommander{reply: notLoaded}
	l, home := newTestLaunchd(t, cmd)
	plistPath := filepath.Join(home, "Library", "LaunchAgents", plistName)
	logPath := filepath.Join(home, "Library", "Logs", "managents.log")

	state, err := l.Status()
	if err != nil {
		t.Fatal(err)
	}
	if want := (State{File: plistPath, LogPath: logPath}); state != want {
		t.Errorf("not installed: %+v, want %+v", state, want)
	}

	if err := l.Install(testExecutable); err != nil {
		t.Fatal(err)
	}
	cmd.reply = func([]string) (string, error) { return string(running), nil }
	state, err = l.Status()
	if err != nil {
		t.Fatal(err)
	}
	want := State{Installed: true, Loaded: true, Running: true, PID: 4242, Program: testExecutable,
		File: plistPath, LogPath: logPath}
	if state != want {
		t.Errorf("running: %+v, want %+v", state, want)
	}

	cmd.reply = func([]string) (string, error) { return "", errors.New("launchctl: not found") }
	if _, err := l.Status(); err == nil {
		t.Error("Status hid a failing launchctl")
	}
}

func TestParseLaunchctlPrint(t *testing.T) {
	tests := []struct {
		file string
		want State
	}{
		{"launchctl-print-running.txt", State{Loaded: true, Running: true, PID: 4242, Program: testExecutable}},
		// launchd waits before restarting a job that keeps exiting; the pid
		// of a nested block is not the job's.
		{"launchctl-print-crashing.txt", State{Loaded: true, LastExit: "2", Program: testExecutable}},
	}
	for _, tt := range tests {
		out, err := os.ReadFile(filepath.Join("testdata", tt.file))
		if err != nil {
			t.Fatal(err)
		}
		if got := parseLaunchctlPrint(string(out)); got != tt.want {
			t.Errorf("%s: %+v, want %+v", tt.file, got, tt.want)
		}
	}
}
