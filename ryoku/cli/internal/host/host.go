// Package host hides init-system and package-manager differences behind one
// command contract shared by Ryoku's runtime components.
package host

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"syscall"
)

const (
	ExitOK          = 0
	ExitFailure     = 1
	ExitUsage       = 2
	ExitFalse       = 3
	ExitAbsent      = 4
	ExitNotProvided = 5
)

type Command struct {
	Name       string
	Args       []string
	Env        []string
	Stdin      io.Reader
	Stdout     io.Writer
	Stderr     io.Writer
	Detached   bool
	Replace    bool
	AppendPath string
}

type Result struct {
	Code   int
	Output string
	PID    int
}

type Runner interface {
	Run(Command) Result
	LookPath(string) bool
	SignalGroup(int, syscall.Signal) error
	Alive(int) bool
}

type ExecRunner struct{}

func (ExecRunner) LookPath(name string) bool {
	_, err := exec.LookPath(name)
	return err == nil
}

func (ExecRunner) Run(spec Command) Result {
	if spec.Replace {
		path, err := exec.LookPath(spec.Name)
		if err != nil {
			return Result{Code: ExitFailure}
		}
		env := os.Environ()
		if spec.Env != nil {
			env = append(env, spec.Env...)
		}
		if err := syscall.Exec(path, append([]string{spec.Name}, spec.Args...), env); err != nil {
			return Result{Code: ExitFailure}
		}
	}
	cmd := exec.Command(spec.Name, spec.Args...)
	if spec.Env != nil {
		cmd.Env = append(os.Environ(), spec.Env...)
	}
	cmd.Stdin = spec.Stdin
	cmd.Stdout = spec.Stdout
	cmd.Stderr = spec.Stderr
	if cmd.Stdout == nil && !spec.Detached {
		output, err := cmd.Output()
		if err == nil {
			return Result{Output: string(output)}
		}
		if exit, ok := err.(*exec.ExitError); ok {
			return Result{Code: exit.ExitCode(), Output: string(output)}
		}
		return Result{Code: ExitFailure}
	}
	if spec.Detached {
		cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
		cmd.Stdin = nil
		if spec.AppendPath != "" {
			if err := os.MkdirAll(filepath.Dir(spec.AppendPath), 0o700); err != nil {
				return Result{Code: ExitFailure}
			}
			log, err := os.OpenFile(spec.AppendPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
			if err != nil {
				return Result{Code: ExitFailure}
			}
			defer log.Close()
			cmd.Stdout, cmd.Stderr = log, log
		}
		if err := cmd.Start(); err != nil {
			return Result{Code: ExitFailure}
		}
		pid := cmd.Process.Pid
		_ = cmd.Process.Release()
		return Result{PID: pid}
	}
	if err := cmd.Run(); err != nil {
		if exit, ok := err.(*exec.ExitError); ok {
			return Result{Code: exit.ExitCode()}
		}
		return Result{Code: ExitFailure}
	}
	return Result{}
}

func (ExecRunner) SignalGroup(pid int, signal syscall.Signal) error {
	return syscall.Kill(-pid, signal)
}

func (ExecRunner) Alive(pid int) bool {
	return syscall.Kill(pid, 0) == nil
}

type Config struct {
	Runner            Runner
	Stdin             io.Reader
	Stdout            io.Writer
	Stderr            io.Writer
	Getenv            func(string) string
	SystemdRuntimeDir string
	UserServiceDir    string
	SystemServiceDir  string
	SystemBootDir     string
	SystemLiveDir     string
	TurnstileEnvDir   string
	RuntimeDir        string
	PackageTable      string
	ZoneinfoDir       string
	LocaltimePath     string
	RCConfPath        string
	InitLibDir        string
	PacmanConf        string
	PacmanSyncDir     string
	XBPSConfigDir     string
	XBPSShippedConfig string
	UIDSet            bool
	UID               int
}

type App struct {
	cfg Config
}

func New(cfg Config) *App {
	if cfg.Runner == nil {
		cfg.Runner = ExecRunner{}
	}
	if cfg.Stdin == nil {
		cfg.Stdin = os.Stdin
	}
	if cfg.Stdout == nil {
		cfg.Stdout = os.Stdout
	}
	if cfg.Stderr == nil {
		cfg.Stderr = os.Stderr
	}
	if cfg.Getenv == nil {
		cfg.Getenv = os.Getenv
	}
	if cfg.SystemdRuntimeDir == "" {
		cfg.SystemdRuntimeDir = "/run/systemd/system"
	}
	if cfg.SystemServiceDir == "" {
		cfg.SystemServiceDir = "/etc/sv"
	}
	if cfg.SystemBootDir == "" {
		cfg.SystemBootDir = "/etc/runit/runsvdir/default"
	}
	if cfg.SystemLiveDir == "" {
		cfg.SystemLiveDir = "/var/service"
	}
	if cfg.PackageTable == "" {
		cfg.PackageTable = "/usr/share/ryoku/packages/void.tsv"
	}
	if cfg.ZoneinfoDir == "" {
		cfg.ZoneinfoDir = "/usr/share/zoneinfo"
	}
	if cfg.LocaltimePath == "" {
		cfg.LocaltimePath = "/etc/localtime"
	}
	if cfg.RCConfPath == "" {
		cfg.RCConfPath = "/etc/rc.conf"
	}
	if cfg.InitLibDir == "" {
		cfg.InitLibDir = "/usr/lib/ryoku/runit"
	}
	if cfg.PacmanConf == "" {
		cfg.PacmanConf = "/etc/pacman.conf"
	}
	if cfg.PacmanSyncDir == "" {
		cfg.PacmanSyncDir = "/var/lib/pacman/sync"
	}
	if cfg.XBPSConfigDir == "" {
		cfg.XBPSConfigDir = "/etc/xbps.d"
	}
	if cfg.XBPSShippedConfig == "" {
		cfg.XBPSShippedConfig = "/usr/share/xbps.d/20-ryoku.conf"
	}
	if !cfg.UIDSet && cfg.UID == 0 {
		cfg.UID = os.Geteuid()
	}
	return &App{cfg: cfg}
}

// Default returns the production host seam backed by the current process and
// the host filesystem.
func Default() *App {
	return New(Config{})
}

func (a *App) getenv(key string) string { return a.cfg.Getenv(key) }

func (a *App) home() string {
	if home := a.getenv("HOME"); home != "" {
		return home
	}
	home, _ := os.UserHomeDir()
	return home
}

func (a *App) userServiceDir() string {
	if a.cfg.UserServiceDir != "" {
		return a.cfg.UserServiceDir
	}
	if dir := a.getenv("RYOKU_USER_SERVICE_DIR"); dir != "" {
		return dir
	}
	return filepath.Join(a.home(), ".config", "service")
}

func (a *App) envDir() string {
	if a.cfg.TurnstileEnvDir != "" {
		return a.cfg.TurnstileEnvDir
	}
	if dir := a.getenv("TURNSTILE_ENV_DIR"); dir != "" {
		return dir
	}
	return filepath.Join(a.home(), ".config", "service-env")
}

func (a *App) query(name string, args ...string) Result {
	return a.cfg.Runner.Run(Command{Name: name, Args: args, Stdin: a.cfg.Stdin})
}

func (a *App) probe(name string, args ...string) Result {
	return a.cfg.Runner.Run(Command{Name: name, Args: args, Stdin: a.cfg.Stdin, Stdout: io.Discard, Stderr: io.Discard})
}

func (a *App) runtimeDir() string {
	if a.cfg.RuntimeDir != "" {
		return a.cfg.RuntimeDir
	}
	if dir := a.getenv("XDG_RUNTIME_DIR"); dir != "" {
		return dir
	}
	return filepath.Join("/tmp", "ryoku-"+strconv.Itoa(os.Getuid()))
}

func (a *App) run(name string, args ...string) Result {
	return a.cfg.Runner.Run(Command{Name: name, Args: args, Stdin: a.cfg.Stdin, Stdout: a.cfg.Stdout, Stderr: a.cfg.Stderr})
}

func (a *App) failf(format string, args ...any) int {
	fmt.Fprintf(a.cfg.Stderr, "ryoku-host: "+format+"\n", args...)
	return ExitFailure
}

func commandExit(result Result) int {
	if result.Code == 0 {
		return ExitOK
	}
	return ExitFailure
}
