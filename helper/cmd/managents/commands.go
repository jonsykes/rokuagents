package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/tonylook/managents/helper/internal/agent"
	"github.com/tonylook/managents/helper/internal/app"
	"github.com/tonylook/managents/helper/internal/demo"
	"github.com/tonylook/managents/helper/internal/detect"
	"github.com/tonylook/managents/helper/internal/detect/claude"
	"github.com/tonylook/managents/helper/internal/detect/codex"
	"github.com/tonylook/managents/helper/internal/detect/opencode"
	"github.com/tonylook/managents/helper/internal/device"
	"github.com/tonylook/managents/helper/internal/process"
	"github.com/tonylook/managents/helper/internal/protocol"
)

// defaultSources wires the detectors to this computer. claudeWindow forces the
// Claude context limit in tokens (0 = infer).
func defaultSources(home string, claudeWindow int) []detect.Source {
	procs := process.System{}
	claudeSource := claude.NewSource(home, procs)
	claudeSource.ContextWindow = claudeWindow
	return []detect.Source{
		claudeSource,
		&opencode.Source{Processes: procs, Store: opencode.SQLiteStore{Path: opencode.DefaultDatabasePath(home)}},
		codex.NewSource(home, procs),
	}
}

func runCommand(ctx context.Context, inv *invocation) error {
	flags := inv.flags()
	port := portFlag(flags)
	logLevel := flags.String("log-level", "info", "debug, info, warn or error")
	logFile := flags.String("log-file", "",
		"append the log to this file instead of stderr; past 1 MiB it moves to FILE.1 at the next start")
	claudeWindow := claudeContextFlag(flags)
	if err := inv.parse(flags); err != nil {
		return err
	}
	if _, err := parseLogLevel(*logLevel); err != nil {
		return err // before a log file is created
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	logOutput := inv.stderr
	if *logFile != "" {
		file, err := openLogFile(*logFile, maxLogSize)
		if err != nil {
			return err
		}
		defer file.Close()
		logOutput = file
	}
	logger, err := newLogger(logOutput, *logLevel)
	if err != nil {
		return err
	}

	displays := device.NewManager(logger, *port)
	defer displays.Close()
	_, displays.AvailableFirmware = embeddedFirmware()
	logger.Info(startedMessage, "version", currentVersion())
	return app.NewRunner(defaultSources(home, *claudeWindow), displays, logger).Run(ctx)
}

func statusCommand(ctx context.Context, inv *invocation) error {
	flags := inv.flags()
	asJSON := flags.Bool("json", false, "print the protocol frame that would be sent")
	claudeWindow := claudeContextFlag(flags)
	if err := inv.parse(flags); err != nil {
		return err
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}

	now := time.Now()
	sessions, detectErr := detect.All(ctx, now, defaultSources(home, *claudeWindow))
	cards := agent.Arrange(sessions)
	if *asJSON {
		line, err := protocol.Encode(protocol.NewState(cards, now))
		if err != nil {
			return err
		}
		_, err = inv.stdout.Write(line)
		return errors.Join(err, detectErr)
	}
	return errors.Join(printStatus(inv.stdout, cards, now, home), detectErr)
}

// printStatus prints one line per card, in display order. The folder is the
// session's working directory with the home directory shortened to ~: the
// card name alone may not tell two folders apart, and the output is often
// pasted into bug reports.
func printStatus(w io.Writer, cards []agent.Card, now time.Time, home string) error {
	if len(cards) == 0 {
		_, err := fmt.Fprintln(w, "no open agent sessions")
		return err
	}
	table := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	fmt.Fprintln(table, "NAME\tAGENT\tSTATUS\tAGE\tCONTEXT\tFOLDER")
	for _, c := range cards {
		fmt.Fprintf(table, "%s\t%s\t%s\t%s\t%s\t%s\n", c.Name, c.Kind, c.Status,
			c.Age(now).Truncate(time.Second), contextText(c.Context), shortPath(c.Dir, home))
	}
	return table.Flush()
}

func contextText(usage *agent.ContextUsage) string {
	switch {
	case usage == nil:
		return "-"
	case usage.Limit > 0:
		return fmt.Sprintf("%dk/%dk", usage.Used/1000, usage.Limit/1000)
	default:
		return fmt.Sprintf("%dk", usage.Used/1000)
	}
}

// shortPath writes path relative to home as ~/...
func shortPath(path, home string) string {
	if home == "" {
		return path
	}
	rel, err := filepath.Rel(home, path)
	switch {
	case err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)):
		return path
	case rel == ".":
		return "~"
	default:
		return "~" + string(filepath.Separator) + rel
	}
}

func devicesCommand(_ context.Context, inv *invocation) error {
	if err := inv.parse(inv.flags()); err != nil {
		return err
	}
	ports, err := device.USBEnumerator{IncludeUnknown: true}.Candidates()
	if err != nil {
		return err
	}
	if len(ports) == 0 {
		fmt.Fprintln(inv.stdout, "no USB serial ports found (is the cable a data cable?)")
		return nil
	}
	_, available := embeddedFirmware()
	for _, name := range ports {
		display, err := device.Connect(device.SerialOpener{}, name, device.DefaultHandshakeTimeout)
		var info protocol.Hello
		if err == nil {
			info = display.Info
			display.Close()
		}
		fmt.Fprintln(inv.stdout, describePort(name, info, err, available))
	}
	return nil
}

// describePort is the devices line of a port, given what connecting to it
// returned and the firmware version this helper can flash ("" for none).
func describePort(name string, info protocol.Hello, err error, available string) string {
	switch {
	case errors.Is(err, device.ErrPortBusy):
		return name + "\tin use by another program (the managents service?)"
	case errors.Is(err, device.ErrNotADisplay):
		return name + "\tnot a managents display (new board? run: managents flash)"
	case err != nil:
		return name + "\terror: " + strings.TrimPrefix(err.Error(), name+": ")
	}
	line := fmt.Sprintf("%s\tmanagents display: board %s, firmware %s, %dx%d", name, info.Board, info.FW, info.W, info.H)
	switch {
	case protocol.OlderThan(info.FW, protocol.MinFirmware):
		line += fmt.Sprintf("\n\tfirmware older than %s, update it with: managents flash", protocol.MinFirmware)
	case protocol.OlderThan(info.FW, available):
		line += fmt.Sprintf("\n\tfirmware %s available: run managents flash", available)
	}
	if info.Proto > protocol.Version {
		line += "\n\tthe display speaks a newer protocol: update managents"
	}
	return line
}

func demoCommand(ctx context.Context, inv *invocation) error {
	flags := inv.flags()
	port := portFlag(flags)
	hold := flags.Duration("hold", 6*time.Second, "how long each demo screen stays up")
	if err := inv.parse(flags); err != nil {
		return err
	}
	if *hold <= 0 {
		return inv.usageError(flags, "invalid value %v for -hold: must be positive", *hold)
	}
	logger, err := newLogger(inv.stderr, "info")
	if err != nil {
		return err
	}
	displays := device.NewManager(logger, *port)
	defer displays.Close()
	source := &demo.Source{Start: time.Now(), Hold: *hold, Logger: logger}
	logger.Info("demo started; scenes show on every connected display (Ctrl-C to stop)", "hold", *hold)
	return app.NewRunner([]detect.Source{source}, displays, logger).Run(ctx)
}

func portFlag(flags *flag.FlagSet) *string {
	return flags.String("port", "", "use this serial port only, e.g. /dev/cu.usbserial-10 or COM3 (default: auto-detect)")
}

func claudeContextFlag(flags *flag.FlagSet) *int {
	return flags.Int("claude-context", 0,
		"Claude context window in tokens, e.g. 1000000 (default: 200k, or 1M once a session exceeds 200k)")
}
