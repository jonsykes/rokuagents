package service

import (
	"bufio"
	"encoding/xml"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// launchctl exit statuses the launchd adapter tells apart.
const (
	launchctlNoSuchProcess = 3   // bootout: the job is not loaded
	launchctlIOError       = 5   // bootstrap: often the previous instance still exiting
	launchctlNotFound      = 113 // print, bootout: no such service in the domain
)

// The bootstrap retry covers a job that bootout has not finished stopping.
const (
	bootstrapAttempts   = 10
	bootstrapRetryDelay = 200 * time.Millisecond
)

// launchd manages a LaunchAgent in the user's GUI domain. The log file gets
// the program's log (--log-file) and launchd's capture of its stderr, so a
// panic lands next to the log lines that led to it.
type launchd struct {
	label     string
	args      []string // after the executable
	plistPath string
	logPath   string
	domain    string // gui/<uid>
	cmd       Commander
	sleep     func(time.Duration)
}

func newLaunchd(env Env, cmd Commander) *launchd {
	logPath := filepath.Join(env.Home, "Library", "Logs", "managents.log")
	return &launchd{
		label:     Label,
		args:      []string{"run", "--log-file", logPath},
		plistPath: filepath.Join(env.Home, "Library", "LaunchAgents", Label+".plist"),
		logPath:   logPath,
		domain:    fmt.Sprintf("gui/%d", env.UID),
		cmd:       cmd,
		sleep:     time.Sleep,
	}
}

// service is the job's launchctl service target.
func (l *launchd) service() string { return l.domain + "/" + l.label }

func (l *launchd) Install(executable string) error {
	plist, err := render("launchd.plist.tmpl", job{
		Label:   l.label,
		Program: append([]string{executable}, l.args...),
		LogPath: l.logPath,
	})
	if err != nil {
		return err
	}
	if err := writeFileAtomic(l.plistPath, plist, 0o644); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(l.logPath), 0o755); err != nil {
		return err // neither launchd nor the helper creates it
	}
	if err := l.bootout(); err != nil {
		return err
	}
	// enable clears a "disabled" mark left by an earlier launchctl disable.
	if _, err := l.cmd.Run("launchctl", "enable", l.service()); err != nil {
		return err
	}
	return l.bootstrap()
}

func (l *launchd) Uninstall() error {
	if err := l.bootout(); err != nil {
		return err
	}
	return removeFiles(l.plistPath, l.logPath, l.logPath+".1")
}

func (l *launchd) Start() error {
	state, err := l.Status()
	switch {
	case err != nil:
		return err
	case !state.Installed:
		return ErrNotInstalled
	case state.Loaded:
		return nil // launchd keeps a loaded job running
	}
	return l.bootstrap()
}

func (l *launchd) Stop() error {
	if err := l.requireInstalled(); err != nil {
		return err
	}
	return l.bootout()
}

func (l *launchd) Status() (State, error) {
	installed, err := exists(l.plistPath)
	if err != nil {
		return State{}, err
	}
	out, err := l.cmd.Run("launchctl", "print", l.service())
	var state State
	switch exitCode(err) {
	case 0:
		state = parseLaunchctlPrint(out)
	case launchctlNotFound:
	default:
		return State{}, err
	}
	state.Installed, state.File, state.LogPath = installed, l.plistPath, l.logPath
	return state, nil
}

func (l *launchd) requireInstalled() error {
	installed, err := exists(l.plistPath)
	if err == nil && !installed {
		return ErrNotInstalled
	}
	return err
}

// bootout unloads the job, which stops it; a job that is not loaded is fine.
func (l *launchd) bootout() error {
	_, err := l.cmd.Run("launchctl", "bootout", l.service())
	if code := exitCode(err); code == launchctlNoSuchProcess || code == launchctlNotFound {
		return nil
	}
	return err
}

// bootstrap loads the job, which starts it (RunAtLoad). launchd refuses while
// a booted-out instance is still exiting, so that error is retried briefly.
// It also refuses, with the same error, a job switched off in System Settings.
func (l *launchd) bootstrap() error {
	for attempt := 1; ; attempt++ {
		_, err := l.cmd.Run("launchctl", "bootstrap", l.domain, l.plistPath)
		switch {
		case exitCode(err) != launchctlIOError:
			return err
		case attempt == bootstrapAttempts:
			return fmt.Errorf("%w; if managents is switched off in System Settings > General > "+
				"Login Items & Extensions, switch it on and try again", err)
		}
		l.sleep(bootstrapRetryDelay)
	}
}

// parseLaunchctlPrint reads the top-level "key = value" lines of
// `launchctl print <service>`. Nested blocks (indented further) describe other
// things and are skipped. The format is not documented; unknown lines are
// ignored.
func parseLaunchctlPrint(out string) State {
	state := State{Loaded: true}
	scanner := bufio.NewScanner(strings.NewReader(out))
	for scanner.Scan() {
		line := scanner.Text()
		if !strings.HasPrefix(line, "\t") || strings.HasPrefix(line, "\t\t") {
			continue
		}
		key, value, ok := strings.Cut(strings.TrimSpace(line), " = ")
		if !ok {
			continue
		}
		switch key {
		case "state":
			state.Running = value == "running"
		case "pid":
			state.PID, _ = strconv.Atoi(value)
		case "program":
			state.Program = value
		case "last exit code":
			if value != "(never exited)" {
				state.LastExit = value
			}
		}
	}
	return state
}

// xmlEscape makes s safe as XML character data.
func xmlEscape(s string) (string, error) {
	var out strings.Builder
	err := xml.EscapeText(&out, []byte(s))
	return out.String(), err
}
