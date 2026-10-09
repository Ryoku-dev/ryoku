package host

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
)

type transientOptions struct {
	scope   bool
	slice   string
	props   []string
	env     []string
	command []string
}

func parseTransient(args []string) (transientOptions, bool) {
	var out transientOptions
	for len(args) > 0 {
		switch args[0] {
		case "--":
			out.command = args[1:]
			return out, len(out.command) > 0
		case "--scope":
			out.scope, args = true, args[1:]
		case "--slice":
			if len(args) < 2 {
				return out, false
			}
			out.slice, args = args[1], args[2:]
		case "--prop":
			if len(args) < 2 || !strings.Contains(args[1], "=") {
				return out, false
			}
			out.props, args = append(out.props, args[1]), args[2:]
		case "--env":
			if len(args) < 2 || envName(args[1]) == "" || !strings.Contains(args[1], "=") {
				return out, false
			}
			out.env, args = append(out.env, args[1]), args[2:]
		default:
			return out, false
		}
	}
	return out, false
}

func validJobName(name string) bool {
	return name != "" && name != "." && name != ".." && !strings.ContainsAny(name, "/\\")
}

func (a *App) Transient(args []string) int {
	if len(args) < 2 || !validJobName(args[1]) {
		return ExitUsage
	}
	verb, name := args[0], args[1]
	init, err := a.Init()
	if err != nil {
		return a.failf("%v", err)
	}
	switch verb {
	case "start":
		opts, ok := parseTransient(args[2:])
		if !ok {
			return ExitUsage
		}
		if init == Systemd {
			return a.startSystemdTransient(name, opts)
		}
		return a.startRunitTransient(name, opts)
	case "stop", "is-active":
		if len(args) != 2 {
			return ExitUsage
		}
		if init == Systemd {
			return a.systemdTransientState(verb, name)
		}
		return a.runitTransientState(verb, name)
	default:
		return ExitUsage
	}
}

func (a *App) transientDir() string         { return filepath.Join(a.runtimeDir(), "ryoku", "transient") }
func (a *App) pidPath(name string) string   { return filepath.Join(a.transientDir(), name+".pid") }
func (a *App) scopePath(name string) string { return filepath.Join(a.transientDir(), name+".scope") }

func (a *App) startSystemdTransient(name string, opts transientOptions) int {
	args := []string{"--user", "--unit=" + name}
	if opts.scope {
		args = append(args, "--scope")
	}
	if opts.slice != "" {
		args = append(args, "--slice="+opts.slice)
	}
	for _, prop := range opts.props {
		args = append(args, "-p", prop)
	}
	for _, env := range opts.env {
		args = append(args, "-E", env)
	}
	if !opts.scope {
		args = append(args, "--collect")
	}
	args = append(args, "--quiet", "--")
	args = append(args, opts.command...)
	if opts.scope {
		if err := os.MkdirAll(a.transientDir(), 0o700); err != nil {
			return ExitFailure
		}
		if err := os.WriteFile(a.scopePath(name), nil, 0o600); err != nil {
			return ExitFailure
		}
	}
	return commandExit(a.run("systemd-run", args...))
}

func (a *App) systemdTransientState(verb, name string) int {
	suffix := ".service"
	if _, err := os.Stat(a.scopePath(name)); err == nil {
		suffix = ".scope"
	}
	args := []string{"--user", verb}
	if verb == "is-active" {
		args = append(args, "--quiet")
	}
	args = append(args, name+suffix)
	result := a.run("systemctl", args...)
	if result.Code == 0 {
		if verb == "stop" {
			_ = os.Remove(a.scopePath(name))
		}
		return ExitOK
	}
	if result.Code == 3 {
		return ExitFalse
	}
	if result.Code == 4 || result.Code == 5 {
		return ExitAbsent
	}
	return ExitFailure
}

func (a *App) startRunitTransient(name string, opts transientOptions) int {
	if opts.scope {
		result := a.cfg.Runner.Run(Command{Name: opts.command[0], Args: opts.command[1:], Env: opts.env, Stdin: a.cfg.Stdin, Stdout: a.cfg.Stdout, Stderr: a.cfg.Stderr})
		return commandExit(result)
	}
	if err := os.MkdirAll(a.transientDir(), 0o700); err != nil {
		return ExitFailure
	}
	result := a.cfg.Runner.Run(Command{Name: opts.command[0], Args: opts.command[1:], Env: opts.env, Detached: true, AppendPath: filepath.Join(a.transientDir(), name+".log")})
	if result.Code != 0 || result.PID <= 0 {
		return ExitFailure
	}
	if err := os.WriteFile(a.pidPath(name), []byte(strconv.Itoa(result.PID)), 0o600); err != nil {
		_ = a.cfg.Runner.SignalGroup(result.PID, syscall.SIGTERM)
		return ExitFailure
	}
	return ExitOK
}

func (a *App) transientPID(name string) (int, int) {
	body, err := os.ReadFile(a.pidPath(name))
	if err != nil {
		return 0, ExitAbsent
	}
	pid, err := strconv.Atoi(strings.TrimSpace(string(body)))
	if err != nil || pid <= 0 {
		_ = os.Remove(a.pidPath(name))
		return 0, ExitFalse
	}
	if !a.cfg.Runner.Alive(pid) {
		_ = os.Remove(a.pidPath(name))
		return 0, ExitFalse
	}
	return pid, ExitOK
}

func (a *App) runitTransientState(verb, name string) int {
	pid, code := a.transientPID(name)
	if code != ExitOK {
		return code
	}
	if verb == "is-active" {
		return ExitOK
	}
	if err := a.cfg.Runner.SignalGroup(pid, syscall.SIGTERM); err != nil {
		return ExitFailure
	}
	if err := os.Remove(a.pidPath(name)); err != nil && !os.IsNotExist(err) {
		return ExitFailure
	}
	return ExitOK
}
