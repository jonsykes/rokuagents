package process

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

func TestStartTimeOfThisProcess(t *testing.T) {
	started, running := System{}.StartTime(context.Background(), os.Getpid())

	if !running {
		t.Fatal("this process is reported as not running")
	}
	if age := time.Since(started); age < 0 || age > time.Hour {
		t.Errorf("started %v ago, want a moment ago", age)
	}
}

func TestStartTimeOfAnExitedProcess(t *testing.T) {
	child := exec.Command(os.Args[0], "-test.run=^$") // this test binary, running no tests
	if err := child.Run(); err != nil {
		t.Fatal(err)
	}

	_, running := System{}.StartTime(context.Background(), child.Process.Pid)

	if running {
		t.Errorf("pid %d of an exited process is reported as running", child.Process.Pid)
	}
}

func TestFindByNameFindsThisProcess(t *testing.T) {
	name := filepath.Base(os.Args[0])
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if wd, err = filepath.EvalSymlinks(wd); err != nil {
		t.Fatal(err)
	}

	found, err := System{}.FindByName(context.Background(), name)
	if err != nil {
		t.Fatal(err)
	}

	for _, p := range found {
		if p.PID != os.Getpid() {
			continue
		}
		if filepath.Clean(p.Dir) != wd {
			t.Errorf("Dir = %q, want %q", p.Dir, wd)
		}
		if age := time.Since(p.StartedAt); age < 0 || age > time.Hour {
			t.Errorf("started %v ago, want a moment ago", age)
		}
		if len(p.Args) == 0 || filepath.Base(p.Args[0]) != name {
			t.Errorf("Args = %q, want the command line starting with %q", p.Args, name)
		}
		return
	}
	t.Errorf("FindByName(%q) = %+v, want this process (pid %d) among them", name, found, os.Getpid())
}

func TestFindByNameWithoutMatches(t *testing.T) {
	found, err := System{}.FindByName(context.Background(), "no-such-process")
	if err != nil || len(found) != 0 {
		t.Errorf("FindByName = %+v, %v; want nothing", found, err)
	}
}
