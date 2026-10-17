package service

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

var update = flag.Bool("update", false, "rewrite the golden files in testdata")

// fakeCommander records every command and answers with reply.
type fakeCommander struct {
	calls [][]string
	reply func(argv []string) (string, error)
}

func (c *fakeCommander) Run(name string, args ...string) (string, error) {
	argv := append([]string{name}, args...)
	c.calls = append(c.calls, argv)
	if c.reply == nil {
		return "", nil
	}
	return c.reply(argv)
}

// takeCalls returns the commands run since the last call, one string each.
func (c *fakeCommander) takeCalls() []string {
	lines := make([]string, len(c.calls))
	for i, argv := range c.calls {
		lines[i] = strings.Join(argv, " ")
	}
	c.calls = nil
	return lines
}

func exitWith(code int) error { return &ExitError{Command: "fake", Code: code} }

func assertCalls(t *testing.T, cmd *fakeCommander, want ...string) {
	t.Helper()
	if got := cmd.takeCalls(); !slices.Equal(got, want) {
		t.Errorf("commands:\n  %s\nwant:\n  %s", strings.Join(got, "\n  "), strings.Join(want, "\n  "))
	}
}

// assertGolden compares got with testdata/name, or rewrites it with -update.
func assertGolden(t *testing.T, name string, got []byte) {
	t.Helper()
	path := filepath.Join("testdata", name)
	if *update {
		if err := os.WriteFile(path, got, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(want) {
		t.Errorf("%s differs from the render (go test -update rewrites it):\n%s", path, got)
	}
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func touch(t *testing.T, paths ...string) {
	t.Helper()
	for _, path := range paths {
		if err := os.WriteFile(path, []byte("x\n"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
}

func assertGone(t *testing.T, paths ...string) {
	t.Helper()
	for _, path := range paths {
		if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
			t.Errorf("%s still exists (%v)", path, err)
		}
	}
}

func TestNewPicksTheSystemServiceManager(t *testing.T) {
	tests := []struct {
		goos string
		want string // the Manager's type, "" = unsupported
	}{
		{"darwin", "*service.launchd"},
		{"linux", "*service.systemd"},
		{"windows", ""},
		{"freebsd", ""},
	}
	for _, tt := range tests {
		m, err := New(Env{GOOS: tt.goos, Home: "/home/ann", ConfigDir: "/home/ann/.config", UID: 1000}, &fakeCommander{})
		if tt.want == "" {
			if !errors.Is(err, ErrUnsupported) || !strings.Contains(err.Error(), tt.goos) {
				t.Errorf("%s: got %T, %v; want ErrUnsupported naming the system", tt.goos, m, err)
			}
			continue
		}
		if got := fmt.Sprintf("%T", m); got != tt.want || err != nil {
			t.Errorf("%s: got %s, %v; want %s", tt.goos, got, err, tt.want)
		}
	}
}

func TestCheckExecutable(t *testing.T) {
	darwin := Env{GOOS: "darwin", Home: "/Users/ann", TempDir: "/var/folders/xy/T", UID: 501}
	linux := Env{GOOS: "linux", Home: "/home/ann", TempDir: "/tmp", UID: 1000}
	root := darwin
	root.UID = 0

	tests := []struct {
		name    string
		env     Env
		exe     string
		wantErr string // "" = accepted
	}{
		{"installed by install.sh", darwin, "/Users/ann/.local/bin/managents", ""},
		{"Homebrew symlink", darwin, "/opt/homebrew/bin/managents", ""},
		{"Linux home", linux, "/home/ann/.local/bin/managents", ""},
		{"a folder that only starts like Desktop", darwin, "/Users/ann/Desktopper/managents", ""},
		{"Downloads is fine on Linux", linux, "/home/ann/Downloads/managents", ""},
		{"go run", darwin, "/var/folders/xy/T/go-build1234/b001/exe/managents", "'go run'"},
		{"go run with GOTMPDIR", linux, "/home/ann/gotmp/go-build99/b001/exe/managents", "'go run'"},
		{"temporary folder", darwin, "/var/folders/xy/T/managents", "temporary folder"},
		{"/tmp", darwin, "/tmp/managents", "temporary folder"},
		{"/private/tmp", darwin, "/private/tmp/managents", "temporary folder"},
		{"Downloads", darwin, "/Users/ann/Downloads/managents", "~/Downloads"},
		{"Desktop subfolder", darwin, "/Users/ann/Desktop/tools/managents", "~/Desktop"},
		{"Documents", darwin, "/Users/ann/Documents/managents", "~/Documents"},
		{"root", root, "/usr/local/bin/managents", "not as root"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			env := tt.env
			env.Home, env.TempDir = filepath.FromSlash(env.Home), filepath.FromSlash(env.TempDir)
			err := CheckExecutable(env, filepath.FromSlash(tt.exe))
			switch {
			case tt.wantErr == "" && err != nil:
				t.Errorf("refused: %v", err)
			case tt.wantErr != "" && (err == nil || !strings.Contains(err.Error(), tt.wantErr)):
				t.Errorf("got %v, want an error containing %q", err, tt.wantErr)
			}
		})
	}
}

func TestWriteFileAtomicReplacesTheFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "new", "dir", "file")
	for _, content := range []string{"first", "second"} {
		if err := writeFileAtomic(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
		if got := readFile(t, path); got != content {
			t.Errorf("file = %q, want %q", got, content)
		}
	}
	entries, err := os.ReadDir(filepath.Dir(path))
	if err != nil || len(entries) != 1 {
		t.Errorf("directory holds %d entries (%v), want only the file", len(entries), err)
	}
}
