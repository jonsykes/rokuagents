package process

import (
	"context"
	"time"

	"github.com/shirou/gopsutil/v4/process"
	"golang.org/x/sys/unix"
)

// On macOS one sysctl returns the name and start time of every process. It
// is about a hundred times cheaper than querying each process in turn, which
// matters for a loop that lists processes every second all day.

func startTime(_ context.Context, pid int) (time.Time, bool) {
	k, err := unix.SysctlKinfoProc("kern.proc.pid", pid)
	if err != nil {
		return time.Time{}, false // no such process
	}
	return time.Unix(k.Proc.P_starttime.Unix()), true
}

func findByName(ctx context.Context, name string) ([]Info, error) {
	all, err := unix.SysctlKinfoProcSlice("kern.proc.all")
	if err != nil {
		return nil, err
	}
	var found []Info
	for i := range all {
		k := &all[i]
		// p_comm keeps the first 16 bytes (MAXCOMLEN) of the executable name,
		// enough for the names the detectors look for.
		if unix.ByteSliceToString(k.Proc.P_comm[:]) != name {
			continue
		}
		pid := int(k.Proc.P_pid)
		dir, args := inspect(ctx, pid)
		found = append(found, Info{PID: pid, Dir: dir, Args: args, StartedAt: time.Unix(k.Proc.P_starttime.Unix())})
	}
	return found, nil
}

// inspect returns the working directory and command line of a process, each
// empty if it is not readable.
func inspect(ctx context.Context, pid int) (string, []string) {
	p, err := process.NewProcessWithContext(ctx, int32(pid))
	if err != nil {
		return "", nil
	}
	dir, _ := p.CwdWithContext(ctx)
	args, _ := p.CmdlineSliceWithContext(ctx)
	return dir, args
}
