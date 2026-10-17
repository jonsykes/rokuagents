// Package service runs the helper in the background for the logged-in user:
// a LaunchAgent on macOS, a systemd user unit on Linux. Both start it at
// login and restart it if it exits.
//
// The plist and unit files are rendered from the templates next to this file.
// launchctl and systemctl are reached through a Commander, so everything but
// the commands themselves is tested without touching the system.
package service

import (
	"embed"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"text/template"
)

// Label names the LaunchAgent; the systemd unit is managents.service.
const Label = "com.github.tonylook.managents"

// ErrUnsupported is returned by New on systems without service support.
var ErrUnsupported = errors.New("the background service is not supported on this system yet")

// ErrNotInstalled is returned by Start and Stop before Install.
var ErrNotInstalled = errors.New("the managents service is not installed; install it with: managents service install")

// Manager installs and controls the service.
type Manager interface {
	// Install writes the service file for executable and (re)starts the
	// service. Running it again replaces an earlier installation.
	Install(executable string) error
	// Uninstall stops the service and removes its files, logs included.
	Uninstall() error
	// Start starts the installed service. It does nothing if it already runs.
	Start() error
	// Stop stops the service until the next login or Start.
	Stop() error
	// Status reports the service's state; not being installed is a state,
	// not an error.
	Status() (State, error)
}

// State is what the service manager reports about the service.
type State struct {
	Installed bool   // the plist or unit file exists
	Loaded    bool   // the service manager knows the service
	Running   bool   // its process runs
	PID       int    // 0 when not running
	LastExit  string // how the last run ended, "" if it never did
	Program   string // the executable the service manager starts, if it reports one
	File      string // the plist or unit file
	LogPath   string // the log file; "" when the system journal keeps the logs
}

// Commander runs a command and returns its combined output. A command that
// runs and fails is reported as an *ExitError.
type Commander interface {
	Run(name string, args ...string) (string, error)
}

// ExitError is a command that exited with a non-zero status.
type ExitError struct {
	Command string // the command line
	Code    int
	Output  string
}

func (e *ExitError) Error() string {
	return fmt.Sprintf("%s: exit status %d: %s", e.Command, e.Code, strings.TrimSpace(e.Output))
}

// exitCode is err's exit status, 0 for nil and -1 for an error that is not
// an *ExitError.
func exitCode(err error) int {
	var exitErr *ExitError
	switch {
	case err == nil:
		return 0
	case errors.As(err, &exitErr):
		return exitErr.Code
	default:
		return -1
	}
}

// Env is what the service needs to know about the user and the system.
type Env struct {
	GOOS      string
	Home      string
	ConfigDir string // os.UserConfigDir; systemd user units live below it
	TempDir   string
	UID       int
}

// CurrentEnv returns the Env of the running process.
func CurrentEnv() (Env, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return Env{}, err
	}
	configDir, err := os.UserConfigDir()
	if err != nil {
		return Env{}, err
	}
	return Env{GOOS: runtime.GOOS, Home: home, ConfigDir: configDir, TempDir: os.TempDir(), UID: os.Getuid()}, nil
}

// New returns the Manager for env's system.
func New(env Env, cmd Commander) (Manager, error) {
	switch env.GOOS {
	case "darwin":
		return newLaunchd(env, cmd), nil
	case "linux":
		return newSystemd(env, cmd), nil
	default:
		return nil, fmt.Errorf("%w (%s)", ErrUnsupported, env.GOOS)
	}
}

// CheckExecutable refuses to install a service for a program that would not
// start at the next login: a temporary build, a file in a folder that is
// cleaned up or that macOS keeps background services out of, or an install
// as root, whose service would not run in the user's session.
func CheckExecutable(env Env, executable string) error {
	const moveIt = "move it somewhere permanent, such as ~/.local/bin, and run it from there"
	if env.UID == 0 {
		return errors.New("install the service as your own user, not as root: it runs in your login session")
	}
	if strings.Contains(filepath.ToSlash(executable), "/go-build") {
		return fmt.Errorf("%s is a temporary build made by 'go run'; build or install managents first", executable)
	}
	for _, dir := range []string{env.TempDir, "/tmp", "/private/tmp"} {
		if within(executable, dir) {
			return fmt.Errorf("%s is in a temporary folder, which gets cleaned up; %s", executable, moveIt)
		}
	}
	if env.GOOS == "darwin" {
		for _, folder := range []string{"Desktop", "Documents", "Downloads"} {
			if within(executable, filepath.Join(env.Home, folder)) {
				return fmt.Errorf("macOS does not let background services run programs from ~/%s; %s", folder, moveIt)
			}
		}
	}
	return nil
}

// within reports whether path is inside dir.
func within(path, dir string) bool {
	if dir == "" {
		return false
	}
	rel, err := filepath.Rel(filepath.Clean(dir), filepath.Clean(path))
	return err == nil && rel != "." && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}

//go:embed *.tmpl
var templateFiles embed.FS

var templates = template.Must(template.New("").Funcs(template.FuncMap{
	"xml":       xmlEscape,
	"execStart": execStart,
}).ParseFS(templateFiles, "*.tmpl"))

// job is what the templates render.
type job struct {
	Label   string
	Program []string // the executable, then its arguments
	LogPath string
}

func render(name string, j job) ([]byte, error) {
	var out strings.Builder
	if err := templates.ExecuteTemplate(&out, name, j); err != nil {
		return nil, err
	}
	return []byte(out.String()), nil
}

// writeFileAtomic replaces path with data, so that the service manager never
// reads half a file.
func writeFileAtomic(path string, data []byte, perm fs.FileMode) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), "."+filepath.Base(path)+".*")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name()) // fails harmlessly after the rename
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Chmod(perm); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), path)
}

// removeFiles deletes paths; the ones that do not exist are skipped.
func removeFiles(paths ...string) error {
	var errs []error
	for _, path := range paths {
		if err := os.Remove(path); err != nil && !errors.Is(err, fs.ErrNotExist) {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}

// exists reports whether path exists.
func exists(path string) (bool, error) {
	_, err := os.Stat(path)
	if errors.Is(err, fs.ErrNotExist) {
		return false, nil
	}
	return err == nil, err
}
