package host

import (
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"
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
func (a *App) runsvPath(name string) string { return filepath.Join(a.transientDir(), name+".runsv") }
func (a *App) runitTransientPath(name string) string {
	return filepath.Join(a.userServiceDir(), name)
}

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
	if !a.cfg.Runner.LookPath("sv") {
		return a.startDetachedRunitTransient(name, opts)
	}

	service := a.runitTransientPath(name)
	if _, err := os.Stat(a.runsvPath(name)); err == nil {
		_ = a.runitTransientState("stop", name)
	}
	if _, err := os.Stat(service); err == nil {
		return ExitFailure
	} else if !os.IsNotExist(err) {
		return ExitFailure
	}
	if err := os.MkdirAll(a.transientDir(), 0o700); err != nil {
		return ExitFailure
	}
	if err := a.writeRunitTransientService(service, name, opts); err != nil {
		return ExitFailure
	}
	if !a.waitRunitSupervisor(service) {
		_ = os.RemoveAll(service)
		return a.startDetachedRunitTransient(name, opts)
	}

	if err := os.WriteFile(a.runsvPath(name), nil, 0o600); err != nil {
		_ = os.RemoveAll(service)
		return ExitFailure
	}
	if err := os.Remove(filepath.Join(service, "down")); err != nil && !os.IsNotExist(err) {
		_ = a.removeSupervisedRunitTransient(name)
		return ExitFailure
	}
	if result := a.run("sv", "-w", "5", "up", service); result.Code != 0 {
		_ = a.removeSupervisedRunitTransient(name)
		return ExitFailure
	}
	running, code := a.runitStatus(service)
	if code != ExitOK || !running {
		_ = a.removeSupervisedRunitTransient(name)
		return ExitFailure
	}
	pid, ok := a.waitRunitPID(service)
	if !ok {
		_ = a.removeSupervisedRunitTransient(name)
		return ExitFailure
	}
	if err := os.WriteFile(a.pidPath(name), []byte(strconv.Itoa(pid)), 0o600); err != nil {
		_ = a.removeSupervisedRunitTransient(name)
		return ExitFailure
	}
	return ExitOK
}

func (a *App) startDetachedRunitTransient(name string, opts transientOptions) int {
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

func shellQuote(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "'\\''") + "'"
}

func (a *App) writeRunitTransientService(service, name string, opts transientOptions) error {
	root := filepath.Dir(service)
	if err := os.MkdirAll(root, 0o700); err != nil {
		return err
	}
	staging, err := os.MkdirTemp(root, "."+name+".")
	if err != nil {
		return err
	}
	defer os.RemoveAll(staging)
	if err := os.Chmod(staging, 0o700); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(staging, "down"), nil, 0o600); err != nil {
		return err
	}
	cwd, err := os.Getwd()
	if err != nil {
		return err
	}
	var run strings.Builder
	run.WriteString("#!/bin/sh\ncd ")
	run.WriteString(shellQuote(cwd))
	run.WriteString(" || exit 111\nexec chpst -P env -i --")
	for _, entry := range append(os.Environ(), opts.env...) {
		run.WriteByte(' ')
		run.WriteString(shellQuote(entry))
	}
	for _, arg := range opts.command {
		run.WriteByte(' ')
		run.WriteString(shellQuote(arg))
	}
	run.WriteString(" 2>&1\n")
	if err := os.WriteFile(filepath.Join(staging, "run"), []byte(run.String()), 0o700); err != nil {
		return err
	}
	finish := "#!/bin/sh\n: > ./down\nexec sv down .\n"
	if err := os.WriteFile(filepath.Join(staging, "finish"), []byte(finish), 0o700); err != nil {
		return err
	}
	logDir := filepath.Join(staging, "log")
	if err := os.Mkdir(logDir, 0o700); err != nil {
		return err
	}
	logRun := "#!/bin/sh\nexec cat >> " + shellQuote(filepath.Join(a.transientDir(), name+".log")) + " 2>&1\n"
	if err := os.WriteFile(filepath.Join(logDir, "run"), []byte(logRun), 0o700); err != nil {
		return err
	}
	return os.Rename(staging, service)
}

func (a *App) waitRunitSupervisor(service string) bool {
	deadline := time.Now().Add(6 * time.Second)
	for {
		if a.probe("sv", "status", service).Code == 0 {
			return true
		}
		if !time.Now().Before(deadline) {
			return false
		}
		time.Sleep(100 * time.Millisecond)
	}
}

func (a *App) waitRunitPID(service string) (int, bool) {
	deadline := time.Now().Add(time.Second)
	for {
		body, err := os.ReadFile(filepath.Join(service, "supervise", "pid"))
		pid, parseErr := strconv.Atoi(strings.TrimSpace(string(body)))
		if err == nil && parseErr == nil && pid > 0 && a.cfg.Runner.Alive(pid) {
			return pid, true
		}
		if !time.Now().Before(deadline) {
			return 0, false
		}
		time.Sleep(25 * time.Millisecond)
	}
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

func (a *App) removeSupervisedRunitTransient(name string) error {
	var first error
	for _, path := range []string{a.runitTransientPath(name), a.runsvPath(name), a.pidPath(name)} {
		if err := os.RemoveAll(path); err != nil && first == nil {
			first = err
		}
	}
	return first
}

func (a *App) runitTransientState(verb, name string) int {
	if _, err := os.Stat(a.runsvPath(name)); err == nil {
		service := a.runitTransientPath(name)
		if verb == "is-active" {
			running, code := a.runitStatus(service)
			if code != ExitOK {
				return code
			}
			if !running {
				return ExitFalse
			}
			return ExitOK
		}
		pid, _ := a.transientPID(name)
		var signalErr error
		if pid > 0 {
			signalErr = a.cfg.Runner.SignalGroup(pid, syscall.SIGTERM)
			if errors.Is(signalErr, syscall.ESRCH) {
				signalErr = nil
			}
		}
		result := a.run("sv", "-w", "5", "down", service)
		if err := a.removeSupervisedRunitTransient(name); err != nil {
			return ExitFailure
		}
		if signalErr != nil || (result.Code != 0 && pid <= 0) {
			return ExitFailure
		}
		return ExitOK
	}

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
