package main

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log/slog"
	"os"
	"slices"
	"strconv"
	"strings"
)

// maxLogSize is the size past which run --log-file starts a new log. Under
// the service, a program that keeps crashing is restarted every 10 s and
// would otherwise grow the log without bound.
const maxLogSize = 1 << 20

// startedMessage is logged first by every run, so the log can be read from
// the latest start.
const startedMessage = "managents started"

// newLogger logs to w at the given level.
func newLogger(w io.Writer, level string) (*slog.Logger, error) {
	l, err := parseLogLevel(level)
	if err != nil {
		return nil, err
	}
	return slog.New(slog.NewTextHandler(w, &slog.HandlerOptions{Level: l})), nil
}

func parseLogLevel(level string) (slog.Level, error) {
	var l slog.Level
	if err := l.UnmarshalText([]byte(level)); err != nil {
		return 0, fmt.Errorf("invalid log level %q (use debug, info, warn, error)", level)
	}
	return l, nil
}

// openLogFile opens path for appending, creating it readable by the user
// only. A file larger than limit is first renamed to path.1, replacing the
// previous one. Under launchd, which opened path as stderr before the program
// started, a panic in a run that rotated the log therefore lands in path.1.
func openLogFile(path string, limit int64) (*os.File, error) {
	if info, err := os.Stat(path); err == nil && info.Size() > limit {
		if err := os.Rename(path, path+".1"); err != nil {
			return nil, err
		}
	}
	return os.OpenFile(path, os.O_WRONLY|os.O_APPEND|os.O_CREATE, 0o600)
}

// logSummary is what `service status` shows of the service's log.
type logSummary struct {
	warnings []string // distinct warnings and errors since the latest start, oldest first
	tail     []string // the last lines
}

// readLog summarises the log at path; a missing log is empty.
func readLog(path string, tailLines, maxWarnings int) (logSummary, error) {
	data, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return logSummary{}, nil
	}
	if err != nil {
		return logSummary{}, err
	}
	return summarizeLog(string(data), tailLines, maxWarnings), nil
}

// summarizeLog picks the last tailLines lines of log, and the last
// maxWarnings distinct WARN and ERROR lines since the latest start. Lines
// that differ only in their time count as one, the latest kept.
func summarizeLog(log string, tailLines, maxWarnings int) logSummary {
	lines := strings.Split(strings.TrimRight(log, "\n"), "\n")
	if lines[0] == "" {
		return logSummary{}
	}
	started := "msg=" + strconv.Quote(startedMessage)
	since := 0
	for i, line := range lines {
		if strings.Contains(line, started) {
			since = i
		}
	}

	var warnings, keys []string
	for _, line := range lines[since:] {
		if !strings.Contains(line, " level=WARN ") && !strings.Contains(line, " level=ERROR ") {
			continue
		}
		key := withoutTime(line)
		if i := slices.Index(keys, key); i >= 0 {
			keys, warnings = slices.Delete(keys, i, i+1), slices.Delete(warnings, i, i+1)
		}
		keys, warnings = append(keys, key), append(warnings, line)
	}
	return logSummary{
		warnings: warnings[max(len(warnings)-maxWarnings, 0):],
		tail:     lines[max(len(lines)-tailLines, 0):],
	}
}

// withoutTime drops the leading time=... attribute of a log line.
func withoutTime(line string) string {
	if !strings.HasPrefix(line, "time=") {
		return line
	}
	_, rest, _ := strings.Cut(line, " ")
	return rest
}
