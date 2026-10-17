package main

import (
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"
)

func TestOpenLogFileRotatesPastTheLimit(t *testing.T) {
	path := filepath.Join(t.TempDir(), "managents.log")
	write := func(content string) {
		t.Helper()
		file, err := openLogFile(path, 10)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := file.WriteString(content); err != nil {
			t.Fatal(err)
		}
		if err := file.Close(); err != nil {
			t.Fatal(err)
		}
	}
	read := func(path string) string {
		t.Helper()
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		return string(data)
	}

	write("first\n")  // 6 bytes: created
	write("second\n") // 13 bytes after this: appended, still under the limit at open
	if got := read(path); got != "first\nsecond\n" {
		t.Errorf("log = %q, want both runs appended", got)
	}
	write("third\n") // over the limit at open: rotated first
	if got, old := read(path), read(path+".1"); got != "third\n" || old != "first\nsecond\n" {
		t.Errorf("log = %q, previous = %q; want the third run alone, the first two in .1", got, old)
	}

	if runtime.GOOS != "windows" {
		info, err := os.Stat(path)
		if err != nil {
			t.Fatal(err)
		}
		if perm := info.Mode().Perm(); perm != 0o600 {
			t.Errorf("log permissions %v, want 0600", perm)
		}
	}
}

func TestSummarizeLog(t *testing.T) {
	log := strings.Join([]string{
		`time=2026-10-09T09:00:00.000+02:00 level=INFO msg="managents started" version=0.1.0`,
		`time=2026-10-09T09:00:01.000+02:00 level=WARN msg="session detection failed" err="old run"`,
		`time=2026-10-09T10:00:00.000+02:00 level=INFO msg="managents started" version=0.2.0`,
		`time=2026-10-09T10:00:02.000+02:00 level=INFO msg="display connected" port=/dev/cu.usbserial-10`,
		`time=2026-10-09T10:00:02.100+02:00 level=WARN msg="display firmware is out of date" firmware=0.0.9`,
		`time=2026-10-09T10:30:00.000+02:00 level=INFO msg="display disconnected" port=/dev/cu.usbserial-10`,
		`time=2026-10-09T11:00:02.000+02:00 level=INFO msg="display connected" port=/dev/cu.usbserial-10`,
		`time=2026-10-09T11:00:02.100+02:00 level=WARN msg="display firmware is out of date" firmware=0.0.9`,
		`time=2026-10-09T11:05:00.000+02:00 level=ERROR msg="encoding state failed" err=boom`,
		"",
	}, "\n")
	lines := strings.Split(log, "\n")

	tests := []struct {
		name         string
		log          string
		tail         int
		maxWarnings  int
		wantWarnings []string
		wantTail     []string
	}{
		{"since the latest start, repeats once, latest kept", log, 3, 5,
			[]string{lines[7], lines[8]}, lines[6:9]},
		{"at most maxWarnings, the latest", log, 1, 1, []string{lines[8]}, lines[8:9]},
		{"shorter than the tail", strings.Join(lines[:2], "\n"), 5, 5, []string{lines[1]}, lines[:2]},
		{"no start line: the whole log", lines[1], 5, 5, []string{lines[1]}, lines[1:2]},
		{"empty", "", 5, 5, nil, nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := summarizeLog(tt.log, tt.tail, tt.maxWarnings)
			if !slices.Equal(got.warnings, tt.wantWarnings) {
				t.Errorf("warnings:\n%s\nwant:\n%s", strings.Join(got.warnings, "\n"), strings.Join(tt.wantWarnings, "\n"))
			}
			if !slices.Equal(got.tail, tt.wantTail) {
				t.Errorf("tail:\n%s\nwant:\n%s", strings.Join(got.tail, "\n"), strings.Join(tt.wantTail, "\n"))
			}
		})
	}
}

func TestReadLogOfAServiceThatNeverRan(t *testing.T) {
	summary, err := readLog(filepath.Join(t.TempDir(), "missing.log"), 5, 5)
	if err != nil || summary.warnings != nil || summary.tail != nil {
		t.Errorf("readLog = %+v, %v; want an empty summary", summary, err)
	}
}
