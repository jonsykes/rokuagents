//go:build !darwin

package process

import (
	"context"
	"time"

	"github.com/shirou/gopsutil/v4/process"
)

func startTime(ctx context.Context, pid int) (time.Time, bool) {
	p, err := process.NewProcessWithContext(ctx, int32(pid))
	if err != nil {
		return time.Time{}, false
	}
	millis, err := p.CreateTimeWithContext(ctx)
	if err != nil {
		return time.Time{}, false
	}
	return time.UnixMilli(millis), true
}

func findByName(ctx context.Context, name string) ([]Info, error) {
	all, err := process.ProcessesWithContext(ctx)
	if err != nil {
		return nil, err
	}
	var found []Info
	for _, p := range all {
		pname, err := p.NameWithContext(ctx)
		if err != nil || pname != name {
			continue
		}
		info := Info{PID: int(p.Pid)}
		if dir, err := p.CwdWithContext(ctx); err == nil {
			info.Dir = dir
		}
		if args, err := p.CmdlineSliceWithContext(ctx); err == nil {
			info.Args = args
		}
		if millis, err := p.CreateTimeWithContext(ctx); err == nil {
			info.StartedAt = time.UnixMilli(millis)
		}
		found = append(found, info)
	}
	return found, nil
}
