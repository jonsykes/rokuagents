package service

import (
	"bufio"
	"os/user"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
)

const unitName = "managents.service"

// systemd manages a systemd user unit. The helper logs to stderr, which the
// journal keeps and rotates: journalctl --user -u managents.
type systemd struct {
	unitPath string
	args     []string // after the executable
	cmd      Commander
}

func newSystemd(env Env, cmd Commander) *systemd {
	return &systemd{
		unitPath: filepath.Join(env.ConfigDir, "systemd", "user", unitName),
		args:     []string{"run"},
		cmd:      cmd,
	}
}

func (s *systemd) systemctl(args ...string) (string, error) {
	return s.cmd.Run("systemctl", append([]string{"--user"}, args...)...)
}

func (s *systemd) Install(executable string) error {
	unit, err := render("systemd.service.tmpl", job{Program: append([]string{executable}, s.args...)})
	if err != nil {
		return err
	}
	if err := writeFileAtomic(s.unitPath, unit, 0o644); err != nil {
		return err
	}
	for _, args := range [][]string{{"daemon-reload"}, {"enable", unitName}, {"restart", unitName}} {
		if _, err := s.systemctl(args...); err != nil {
			return err
		}
	}
	return nil
}

func (s *systemd) Uninstall() error {
	installed, err := exists(s.unitPath)
	if err != nil || !installed {
		return err
	}
	if _, err := s.systemctl("disable", "--now", unitName); err != nil {
		return err
	}
	if err := removeFiles(s.unitPath); err != nil {
		return err
	}
	_, err = s.systemctl("daemon-reload")
	return err
}

func (s *systemd) Start() error { return s.control("start") }
func (s *systemd) Stop() error  { return s.control("stop") }

func (s *systemd) control(verb string) error {
	installed, err := exists(s.unitPath)
	switch {
	case err != nil:
		return err
	case !installed:
		return ErrNotInstalled
	}
	_, err = s.systemctl(verb, unitName)
	return err
}

func (s *systemd) Status() (State, error) {
	installed, err := exists(s.unitPath)
	if err != nil {
		return State{}, err
	}
	out, err := s.systemctl("show", unitName,
		"-p", "LoadState,ActiveState,MainPID,ExecMainStatus,ExecMainExitTimestamp,ExecStart")
	if err != nil {
		return State{}, err
	}
	state := parseSystemctlShow(out)
	state.Installed, state.File = installed, s.unitPath
	return state, nil
}

// parseSystemctlShow reads the key=value lines of `systemctl show`.
func parseSystemctlShow(out string) State {
	values := make(map[string]string)
	scanner := bufio.NewScanner(strings.NewReader(out))
	for scanner.Scan() {
		if key, value, ok := strings.Cut(scanner.Text(), "="); ok {
			values[key] = value
		}
	}
	state := State{
		Loaded:  values["LoadState"] == "loaded",
		Running: values["ActiveState"] == "active",
		Program: execStartPath(values["ExecStart"]),
	}
	if state.Running {
		state.PID, _ = strconv.Atoi(values["MainPID"])
	}
	if values["ExecMainExitTimestamp"] != "" {
		state.LastExit = values["ExecMainStatus"]
	}
	return state
}

// execStartPath extracts the program from systemctl's ExecStart property,
// "{ path=/usr/bin/x ; argv[]=/usr/bin/x run ; ... }".
func execStartPath(property string) string {
	_, rest, ok := strings.Cut(property, "path=")
	if !ok {
		return ""
	}
	path, _, _ := strings.Cut(rest, " ;")
	return path
}

// execStart quotes a command line for ExecStart: each word in double quotes,
// with the characters systemd would expand (% specifiers, $ variables)
// escaped.
func execStart(program []string) string {
	words := make([]string, len(program))
	for i, word := range program {
		word = strings.NewReplacer(`\`, `\\`, `"`, `\"`, "%", "%%", "$", "$$").Replace(word)
		words[i] = `"` + word + `"`
	}
	return strings.Join(words, " ")
}

// serialGroups own USB serial ports on common distributions, the first one
// that exists wins: dialout on Debian, Ubuntu and Fedora (which also have a
// uucp group), uucp on Arch.
var serialGroups = []string{"dialout", "uucp"}

// SerialAccessHint returns how to let the current user open serial ports,
// or "" when they already can (or it cannot be told).
func SerialAccessHint() string {
	current, err := user.Current()
	if err != nil {
		return ""
	}
	memberOf, err := current.GroupIds()
	if err != nil {
		return ""
	}
	return serialAccessHint(current.Username, memberOf, func(name string) (string, bool) {
		group, err := user.LookupGroup(name)
		if err != nil {
			return "", false
		}
		return group.Gid, true
	})
}

// serialAccessHint is SerialAccessHint for a user in the groups memberOf
// (group ids); lookup returns the id of a group name if it exists.
func serialAccessHint(userName string, memberOf []string, lookup func(name string) (gid string, ok bool)) string {
	for _, name := range serialGroups {
		gid, ok := lookup(name)
		switch {
		case !ok:
			continue
		case slices.Contains(memberOf, gid):
			return ""
		}
		return "Your user may not be allowed to open the display's serial port. Fix it with:\n" +
			"  sudo usermod -aG " + name + " " + userName + "\n" +
			"then log out and back in."
	}
	return ""
}
