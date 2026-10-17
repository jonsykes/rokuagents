package service

import (
	"errors"
	"os/exec"
	"strings"
)

// Exec is the Commander that runs real programs.
type Exec struct{}

// Run implements Commander.
func (Exec) Run(name string, args ...string) (string, error) {
	out, err := exec.Command(name, args...).CombinedOutput()
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		return string(out), &ExitError{
			Command: strings.Join(append([]string{name}, args...), " "),
			Code:    exitErr.ExitCode(),
			Output:  string(out),
		}
	}
	return string(out), err
}
