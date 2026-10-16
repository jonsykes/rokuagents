// Command managents streams the status of local AI coding agents (Claude Code,
// OpenCode, Codex) to a managents USB display.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"runtime/debug"
	"syscall"
	"text/tabwriter"
)

// version is set at build time: -ldflags "-X main.version=v0.1.0".
var version = "dev"

// command is one "managents <name>" subcommand.
type command struct {
	name    string
	summary string // one line, shown by help and -h
	run     func(ctx context.Context, inv *invocation) error
}

// commands lists the subcommands in the order help shows them.
func commands() []command {
	return []command{
		{"run", "stream agent status to every connected display (what the service runs)", runCommand},
		{"status", "print the agent sessions detected on this computer", statusCommand},
		{"devices", "list USB serial ports and identify managents displays", devicesCommand},
		{"service", "run managents in the background from login: install, uninstall, start, stop, status", serviceCommand},
		{"flash", "install the display firmware on a connected board (embedded in this program)", flashCommand},
		{"demo", "cycle demo screens on the connected displays", demoCommand},
		{"version", "print the version", versionCommand},
	}
}

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	code := run(ctx, os.Args[1:], os.Stdout, os.Stderr)
	stop()
	os.Exit(code)
}

// Exit codes of run.
const (
	exitOK      = 0
	exitFailure = 1
	exitUsage   = 2 // the command line is wrong
)

// run executes the command line args and returns the exit code.
func run(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		printUsage(stdout)
		return exitOK
	}
	switch args[0] {
	case "help", "-h", "-help", "--help":
		printUsage(stdout)
		return exitOK
	case "-version", "--version":
		args = append([]string{"version"}, args[1:]...)
	}
	for _, cmd := range commands() {
		if cmd.name == args[0] {
			inv := &invocation{name: cmd.name, summary: cmd.summary, args: args[1:], stdout: stdout, stderr: stderr}
			return exitCode(cmd.run(ctx, inv), stderr)
		}
	}
	fmt.Fprintf(stderr, "managents: unknown command %q\nRun 'managents help' for the list of commands.\n", args[0])
	return exitUsage
}

// exitCode reports err and maps it to an exit code.
func exitCode(err error, stderr io.Writer) int {
	switch {
	case err == nil, errors.Is(err, flag.ErrHelp):
		return exitOK
	case errors.Is(err, errUsage):
		return exitUsage // already reported, with the command's usage
	default:
		fmt.Fprintln(stderr, "managents:", err)
		return exitFailure
	}
}

func printUsage(w io.Writer) {
	fmt.Fprint(w, "managents: AI agent status on a USB desk display\n\nUsage:\n  managents <command> [flags]\n\nCommands:\n")
	table := tabwriter.NewWriter(w, 0, 0, 3, ' ', 0)
	for _, cmd := range commands() {
		fmt.Fprintf(table, "  %s\t%s\n", cmd.name, cmd.summary)
	}
	table.Flush()
	fmt.Fprint(w, "\nRun 'managents <command> -h' for the flags of a command.\n"+
		"Get started: https://github.com/tonylook/managents#get-started\n")
}

// errUsage marks a command line that was rejected and already reported.
var errUsage = errors.New("usage error")

// invocation is one run of a command: its arguments and output streams.
type invocation struct {
	name, summary  string
	args           []string
	stdout, stderr io.Writer
}

// flags returns the command's flag set. Its errors and -h output go to
// stderr, after a line naming the command.
func (inv *invocation) flags() *flag.FlagSet {
	flags := flag.NewFlagSet(inv.name, flag.ContinueOnError)
	flags.SetOutput(inv.stderr)
	flags.Usage = func() {
		fmt.Fprintf(inv.stderr, "managents %s: %s\n", inv.name, inv.summary)
		hasFlags := false
		flags.VisitAll(func(*flag.Flag) { hasFlags = true })
		if hasFlags {
			fmt.Fprintln(inv.stderr, "\nFlags:")
			flags.PrintDefaults()
		}
	}
	return flags
}

// parse parses the command's arguments with flags. The command takes no
// positional arguments.
func (inv *invocation) parse(flags *flag.FlagSet) error {
	if err := flags.Parse(inv.args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return err
		}
		return errUsage // the flag package printed the error and the usage
	}
	if flags.NArg() > 0 {
		return inv.usageError(flags, "unexpected argument %q", flags.Arg(0))
	}
	return nil
}

// usageError reports a wrong command line with the command's usage.
func (inv *invocation) usageError(flags *flag.FlagSet, format string, args ...any) error {
	fmt.Fprintf(inv.stderr, format+"\n", args...)
	flags.Usage()
	return errUsage
}

func versionCommand(_ context.Context, inv *invocation) error {
	if err := inv.parse(inv.flags()); err != nil {
		return err
	}
	_, err := fmt.Fprintln(inv.stdout, "managents", currentVersion())
	return err
}

func currentVersion() string {
	info, _ := debug.ReadBuildInfo()
	return resolveVersion(version, info)
}

// resolveVersion is the version set at build time or, for a plain go build
// or go install, the module version Go recorded.
func resolveVersion(linked string, info *debug.BuildInfo) string {
	if linked != "dev" || info == nil {
		return linked
	}
	if v := info.Main.Version; v != "" && v != "(devel)" {
		return v
	}
	return linked
}
