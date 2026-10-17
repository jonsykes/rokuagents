package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"text/tabwriter"

	"github.com/tonylook/managents/helper/internal/service"
)

// serviceAction is one "managents service <name>" action.
type serviceAction struct {
	name, summary string
	run           func(inv *invocation, env service.Env, m service.Manager) error
}

func serviceActions() []serviceAction {
	return []serviceAction{
		{"install", "run managents now and at every login (again after an update)", serviceInstall},
		{"uninstall", "stop managents and remove the service and its logs", serviceUninstall},
		{"start", "start the installed service", serviceStart},
		{"stop", "stop the service until the next login, e.g. to free the display's port", serviceStop},
		{"status", "show whether the service runs, its warnings and its latest log lines", serviceStatus},
	}
}

func serviceCommand(_ context.Context, inv *invocation) error {
	if len(inv.args) == 0 {
		printServiceUsage(inv.stderr)
		return errUsage
	}
	switch inv.args[0] {
	case "help", "-h", "-help", "--help":
		printServiceUsage(inv.stderr)
		return flag.ErrHelp
	}
	for _, action := range serviceActions() {
		if action.name != inv.args[0] {
			continue
		}
		sub := &invocation{name: "service " + action.name, summary: action.summary, args: inv.args[1:],
			stdout: inv.stdout, stderr: inv.stderr}
		if err := sub.parse(sub.flags()); err != nil {
			return err
		}
		env, err := service.CurrentEnv()
		if err != nil {
			return err
		}
		m, err := service.New(env, service.Exec{})
		if errors.Is(err, service.ErrUnsupported) {
			return fmt.Errorf("%w; start 'managents run' at log on instead, e.g. with Task Scheduler", err)
		}
		if err != nil {
			return err
		}
		return action.run(sub, env, m)
	}
	fmt.Fprintf(inv.stderr, "unknown service action %q\n", inv.args[0])
	printServiceUsage(inv.stderr)
	return errUsage
}

func printServiceUsage(w io.Writer) {
	fmt.Fprint(w, "managents service <action>: run managents in the background from login\n\nActions:\n")
	table := tabwriter.NewWriter(w, 0, 0, 3, ' ', 0)
	for _, action := range serviceActions() {
		fmt.Fprintf(table, "  %s\t%s\n", action.name, action.summary)
	}
	table.Flush()
}

func serviceInstall(inv *invocation, env service.Env, m service.Manager) error {
	// The executable is not resolved through symlinks: a package manager's
	// link keeps working after an upgrade, its target may not.
	executable, err := os.Executable()
	if err != nil {
		return err
	}
	if executable, err = filepath.Abs(executable); err != nil {
		return err
	}
	if err := service.CheckExecutable(env, executable); err != nil {
		return err
	}
	if err := m.Install(executable); err != nil {
		return err
	}
	state, err := m.Status()
	if err != nil {
		return err
	}
	out := inv.stdout
	fmt.Fprintln(out, "managents now runs in the background and starts at login. Plug in the display.")
	fmt.Fprintf(out, "Service file: %s\nLogs: %s\n", shortPath(state.File, env.Home), logLocation(state, env.Home))
	switch env.GOOS {
	case "darwin":
		fmt.Fprintln(out, "macOS may notify you that a background item was added: that is managents.")
	case "linux":
		if hint := service.SerialAccessHint(); hint != "" {
			fmt.Fprintln(out, hint)
		}
	}
	return nil
}

func serviceUninstall(inv *invocation, _ service.Env, m service.Manager) error {
	if err := m.Uninstall(); err != nil {
		return err
	}
	_, err := fmt.Fprintln(inv.stdout, "managents no longer runs in the background; its service file and logs are removed.")
	return err
}

func serviceStart(inv *invocation, _ service.Env, m service.Manager) error {
	if err := m.Start(); err != nil {
		return err
	}
	_, err := fmt.Fprintln(inv.stdout, "managents service started.")
	return err
}

func serviceStop(inv *invocation, _ service.Env, m service.Manager) error {
	if err := m.Stop(); err != nil {
		return err
	}
	_, err := fmt.Fprintln(inv.stdout,
		"managents service stopped until the next login. Start it again with: managents service start")
	return err
}

// Lines of the log `service status` shows.
const (
	statusTailLines = 5
	statusWarnings  = 5
)

func serviceStatus(inv *invocation, env service.Env, m service.Manager) error {
	state, err := m.Status()
	if err != nil {
		return err
	}
	var log logSummary
	if state.LogPath != "" {
		if log, err = readLog(state.LogPath, statusTailLines, statusWarnings); err != nil {
			return err
		}
	}
	programMissing := false
	if state.Program != "" {
		_, err := os.Stat(state.Program)
		programMissing = errors.Is(err, fs.ErrNotExist)
	}
	return printServiceStatus(inv.stdout, env, state, programMissing, log)
}

// printServiceStatus prints the service state, then the warnings logged
// since its latest start (such as an out-of-date display firmware) and the
// latest log lines.
func printServiceStatus(w io.Writer, env service.Env, state service.State, programMissing bool, log logSummary) error {
	if !state.Installed && !state.Loaded {
		_, err := fmt.Fprintln(w, "managents service: not installed (install it with: managents service install)")
		return err
	}
	fmt.Fprintln(w, "managents service:", serviceHeadline(state))
	if !state.Running && env.GOOS == "darwin" {
		fmt.Fprintln(w, "If it does not stay running, allow managents in System Settings > General > Login Items & Extensions.")
	}

	table := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	program := state.Program
	switch {
	case program == "":
		program = "-"
	case programMissing:
		program += " (missing: install managents again, then run: managents service install)"
	}
	fmt.Fprintf(table, "  program\t%s\n", program)
	file := shortPath(state.File, env.Home)
	if !state.Installed {
		file += " (missing: run managents service install)"
	}
	fmt.Fprintf(table, "  file\t%s\n", file)
	fmt.Fprintf(table, "  log\t%s\n", logLocation(state, env.Home))
	if err := table.Flush(); err != nil {
		return err
	}

	for _, section := range []struct {
		title string
		lines []string
	}{
		{"Warnings since the latest start:", log.warnings},
		{"Latest log lines:", log.tail},
	} {
		if len(section.lines) == 0 {
			continue
		}
		fmt.Fprintf(w, "\n%s\n", section.title)
		for _, line := range section.lines {
			fmt.Fprintf(w, "  %s\n", line)
		}
	}
	return nil
}

func serviceHeadline(state service.State) string {
	switch {
	case state.Running:
		return fmt.Sprintf("running (pid %d)", state.PID)
	case state.LastExit != "":
		return "not running, last exit status " + state.LastExit + " (see the log)"
	default:
		return "stopped (start it with: managents service start)"
	}
}

// logLocation is where the service's log can be read.
func logLocation(state service.State, home string) string {
	if state.LogPath == "" {
		return "journalctl --user -u managents"
	}
	return shortPath(state.LogPath, home)
}
