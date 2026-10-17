package service

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func newTestSystemd(t *testing.T, cmd *fakeCommander) (*systemd, string) {
	t.Helper()
	configDir := t.TempDir()
	return newSystemd(Env{GOOS: "linux", Home: "/home/ann", ConfigDir: configDir, UID: 1000}, cmd),
		filepath.Join(configDir, "systemd", "user", "managents.service")
}

func TestUnitGolden(t *testing.T) {
	unit, err := render("systemd.service.tmpl", job{Program: []string{"/home/ann/.local/bin/managents", "run"}})
	if err != nil {
		t.Fatal(err)
	}
	assertGolden(t, "managents.service", unit)
}

func TestExecStartQuoting(t *testing.T) {
	tests := []struct {
		program []string
		want    string
	}{
		{[]string{"/home/ann/.local/bin/managents", "run"}, `"/home/ann/.local/bin/managents" "run"`},
		{[]string{"/home/ann/my tools/managents"}, `"/home/ann/my tools/managents"`},
		{[]string{`/home/ann/"q"/back\slash`}, `"/home/ann/\"q\"/back\\slash"`},
		{[]string{"/home/ann/100%/$HOME/managents"}, `"/home/ann/100%%/$$HOME/managents"`},
	}
	for _, tt := range tests {
		if got := execStart(tt.program); got != tt.want {
			t.Errorf("execStart(%q) = %s, want %s", tt.program, got, tt.want)
		}
	}
}

func TestSystemdInstall(t *testing.T) {
	cmd := &fakeCommander{}
	s, unitPath := newTestSystemd(t, cmd)
	want, err := render("systemd.service.tmpl", job{Program: []string{"/home/ann/.local/bin/managents", "run"}})
	if err != nil {
		t.Fatal(err)
	}
	for range 2 { // installing again does the same
		if err := s.Install("/home/ann/.local/bin/managents"); err != nil {
			t.Fatal(err)
		}
		assertCalls(t, cmd,
			"systemctl --user daemon-reload",
			"systemctl --user enable managents.service",
			"systemctl --user restart managents.service")
		if got := readFile(t, unitPath); got != string(want) {
			t.Errorf("unit:\n%s\nwant:\n%s", got, want)
		}
	}
}

func TestSystemdUninstall(t *testing.T) {
	cmd := &fakeCommander{}
	s, unitPath := newTestSystemd(t, cmd)
	if err := s.Uninstall(); err != nil {
		t.Fatal(err)
	}
	assertCalls(t, cmd) // nothing installed, nothing to do

	if err := s.Install("/home/ann/.local/bin/managents"); err != nil {
		t.Fatal(err)
	}
	cmd.takeCalls()
	if err := s.Uninstall(); err != nil {
		t.Fatal(err)
	}
	assertCalls(t, cmd,
		"systemctl --user disable --now managents.service",
		"systemctl --user daemon-reload")
	assertGone(t, unitPath)
}

func TestSystemdStartAndStop(t *testing.T) {
	cmd := &fakeCommander{}
	s, _ := newTestSystemd(t, cmd)
	if err := s.Start(); !errors.Is(err, ErrNotInstalled) {
		t.Errorf("Start before Install = %v, want ErrNotInstalled", err)
	}
	if err := s.Stop(); !errors.Is(err, ErrNotInstalled) {
		t.Errorf("Stop before Install = %v, want ErrNotInstalled", err)
	}
	assertCalls(t, cmd)

	if err := s.Install("/home/ann/.local/bin/managents"); err != nil {
		t.Fatal(err)
	}
	cmd.takeCalls()
	if err := s.Stop(); err != nil {
		t.Fatal(err)
	}
	if err := s.Start(); err != nil {
		t.Fatal(err)
	}
	assertCalls(t, cmd, "systemctl --user stop managents.service", "systemctl --user start managents.service")
}

func TestSystemdStatus(t *testing.T) {
	running, err := os.ReadFile(filepath.Join("testdata", "systemctl-show-running.txt"))
	if err != nil {
		t.Fatal(err)
	}
	cmd := &fakeCommander{reply: func([]string) (string, error) { return string(running), nil }}
	s, unitPath := newTestSystemd(t, cmd)
	if err := s.Install("/home/ann/.local/bin/managents"); err != nil {
		t.Fatal(err)
	}
	cmd.takeCalls()

	state, err := s.Status()
	if err != nil {
		t.Fatal(err)
	}
	assertCalls(t, cmd, "systemctl --user show managents.service "+
		"-p LoadState,ActiveState,MainPID,ExecMainStatus,ExecMainExitTimestamp,ExecStart")
	want := State{Installed: true, Loaded: true, Running: true, PID: 4242,
		Program: "/home/ann/.local/bin/managents", File: unitPath}
	if state != want {
		t.Errorf("%+v, want %+v", state, want)
	}
}

func TestParseSystemctlShow(t *testing.T) {
	tests := []struct {
		file string
		want State
	}{
		{"systemctl-show-running.txt", State{Loaded: true, Running: true, PID: 4242, Program: "/home/ann/.local/bin/managents"}},
		{"systemctl-show-crashing.txt", State{Loaded: true, LastExit: "1", Program: "/home/ann/.local/bin/managents"}},
		{"systemctl-show-not-found.txt", State{}},
	}
	for _, tt := range tests {
		out, err := os.ReadFile(filepath.Join("testdata", tt.file))
		if err != nil {
			t.Fatal(err)
		}
		if got := parseSystemctlShow(string(out)); got != tt.want {
			t.Errorf("%s: %+v, want %+v", tt.file, got, tt.want)
		}
	}
}

func TestSerialAccessHint(t *testing.T) {
	debian := map[string]string{"dialout": "20", "uucp": "10"}
	arch := map[string]string{"uucp": "987"}
	tests := []struct {
		name     string
		groups   map[string]string // the groups that exist: name -> gid
		memberOf []string
		want     string // the group the hint adds, "" = no hint
	}{
		{"Debian, not a member", debian, []string{"1000", "27"}, "dialout"},
		{"Debian, in dialout", debian, []string{"1000", "20"}, ""},
		{"Debian, only in uucp", debian, []string{"1000", "10"}, "dialout"},
		{"Arch, not a member", arch, []string{"1000"}, "uucp"},
		{"Arch, in uucp", arch, []string{"1000", "987"}, ""},
		{"neither group exists", map[string]string{}, []string{"1000"}, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			lookup := func(name string) (string, bool) { gid, ok := tt.groups[name]; return gid, ok }
			got := serialAccessHint("ann", tt.memberOf, lookup)
			want := ""
			if tt.want != "" {
				want = "Your user may not be allowed to open the display's serial port. Fix it with:\n" +
					"  sudo usermod -aG " + tt.want + " ann\nthen log out and back in."
			}
			if got != want {
				t.Errorf("hint = %q, want %q", got, want)
			}
		})
	}
}
