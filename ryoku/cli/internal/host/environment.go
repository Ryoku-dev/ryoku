package host

import (
	"os"
	"path/filepath"
	"strings"
)

func (a *App) ServiceEnv(args []string) int {
	if len(args) == 0 {
		return ExitUsage
	}
	all := len(args) == 1 && args[0] == "--all"
	if !all {
		for _, arg := range args {
			if arg == "--all" || envName(arg) == "" {
				return ExitUsage
			}
		}
	}
	init, err := a.Init()
	if err != nil {
		return a.failf("%v", err)
	}
	if init == Systemd {
		return commandExit(a.run("dbus-update-activation-environment", append([]string{"--systemd"}, args...)...))
	}
	if err := os.MkdirAll(a.envDir(), 0o700); err != nil {
		return ExitFailure
	}
	entries := args
	if all {
		entries = filteredEnvironment()
	}
	for _, entry := range entries {
		name, value := splitEnvironment(entry, a.getenv)
		if name == "" {
			return ExitUsage
		}
		if err := os.WriteFile(filepath.Join(a.envDir(), name), []byte(value), 0o600); err != nil {
			return ExitFailure
		}
	}
	return commandExit(a.run("dbus-update-activation-environment", args...))
}

func envName(entry string) string {
	name := strings.SplitN(entry, "=", 2)[0]
	if name == "" {
		return ""
	}
	for i, r := range name {
		if !(r == '_' || r >= 'A' && r <= 'Z' || r >= 'a' && r <= 'z' || i > 0 && r >= '0' && r <= '9') {
			return ""
		}
	}
	return name
}

func splitEnvironment(entry string, getenv func(string) string) (string, string) {
	parts := strings.SplitN(entry, "=", 2)
	name := envName(entry)
	if len(parts) == 2 {
		return name, parts[1]
	}
	return name, getenv(name)
}

func filteredEnvironment() []string {
	out := make([]string, 0, len(os.Environ()))
	for _, entry := range os.Environ() {
		name := envName(entry)
		if name == "PWD" || name == "OLDPWD" || name == "SHLVL" || name == "_" || strings.HasPrefix(name, "TURNSTILE_") {
			continue
		}
		out = append(out, entry)
	}
	return out
}

func (a *App) Inhibit(args []string) int {
	if len(args) == 0 {
		return ExitUsage
	}
	binary := ""
	if a.cfg.Runner.LookPath("systemd-inhibit") {
		binary = "systemd-inhibit"
	} else if a.cfg.Runner.LookPath("elogind-inhibit") {
		binary = "elogind-inhibit"
	}
	if binary == "" {
		return ExitAbsent
	}
	return commandExit(a.run(binary, args...))
}
