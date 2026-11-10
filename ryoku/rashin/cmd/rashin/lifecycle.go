package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	i18n "ryoku-i18n"
	"ryoku-rashin/internal/client"
	"ryoku-rashin/internal/config"
	"ryoku-rashin/internal/paths"
	"ryoku-rashin/internal/protocol"
)

func init() {
	register(&Command{Name: "on", Group: "lifecycle", Summary: "Turn Rashin on.", Run: runOn})
	register(&Command{Name: "off", Group: "lifecycle", Summary: "Turn Rashin off.", Run: runOff})
	register(&Command{Name: "status", Group: "lifecycle", Usage: "[--json]", Summary: "Show Rashin status without starting it.", Run: runStatus})
	register(&Command{Name: "ensure", Group: "lifecycle", Summary: "Make sure the daemon is ready.", Run: runEnsure})
	register(&Command{Name: "migrate", Group: "lifecycle", Summary: "Migrate Rashin 1 settings and remove old state.", Run: runMigrate})
}

func runOn(args []string) int {
	if len(args) != 0 {
		return usageError("on")
	}
	if err := config.SetEnabled(true); err != nil {
		return report(err)
	}
	if systemdPresent() {
		if host, err := exec.LookPath("ryoku-host"); err == nil {
			if err := runQuiet(host, "svc", "--user", "enable", "--now", "rashind.socket"); err != nil {
				return report(err)
			}
			if err := runQuiet(host, "svc", "--user", "enable", "--now", "ryoku-ai-usage.timer"); err != nil {
				return report(err)
			}
		} else {
			if err := runQuiet("systemctl", "--user", "enable", "--now", "rashind.socket", "ryoku-ai-usage.timer"); err != nil {
				return report(err)
			}
		}
	} else if host, err := exec.LookPath("ryoku-host"); err == nil {
		_ = runQuiet(host, "svc", "--user", "enable", "--now", "ryoku-ai-usage")
	}
	fmt.Fprintln(os.Stdout, i18n.T("Rashin is on."))
	return 0
}

func runOff(args []string) int {
	if len(args) != 0 {
		return usageError("off")
	}
	if err := config.SetEnabled(false); err != nil {
		return report(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if daemonRunningWithoutDial() {
		if conn, err := client.Dial(ctx, client.Options{Client: protocol.ClientCLI, Start: false}); err == nil {
			_ = conn.Call(ctx, protocol.MShutdown, protocol.Empty{}, &protocol.Empty{})
			conn.Close()
		}
	}
	if systemdPresent() {
		if host, err := exec.LookPath("ryoku-host"); err == nil {
			for _, unit := range []string{"rashind.socket", "rashind.service", "ryoku-ai-usage.timer"} {
				_ = runQuiet(host, "svc", "--user", "disable", "--now", unit)
			}
		} else {
			_ = runQuiet("systemctl", "--user", "disable", "--now", "rashind.socket", "rashind.service", "ryoku-ai-usage.timer")
		}
	} else if host, err := exec.LookPath("ryoku-host"); err == nil {
		_ = runQuiet(host, "svc", "--user", "disable", "--now", "ryoku-ai-usage")
	}
	terminateLockHolder()
	fmt.Fprintln(os.Stdout, i18n.T("Rashin is off."))
	return 0
}

func runStatus(args []string) int {
	jsonMode := len(args) == 1 && args[0] == "--json"
	if len(args) > 1 || len(args) == 1 && !jsonMode {
		return usageError("status [--json]")
	}
	settings, err := config.Load()
	if err != nil {
		return report(err)
	}
	status := protocol.Status{Enabled: settings.Enabled, Running: false, Version: version}
	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()
	if daemonRunningWithoutDial() {
		if conn, err := client.Dial(ctx, client.Options{Client: protocol.ClientCLI, Start: false}); err == nil {
			_ = conn.Call(ctx, protocol.MStatus, protocol.Empty{}, &status)
			conn.Close()
		}
	}
	if jsonMode {
		value := struct {
			protocol.Status
			Settings protocol.Settings `json:"settings"`
		}{Status: status, Settings: settings}
		return printJSON(value)
	}
	state := i18n.T("off")
	if settings.Enabled {
		state = i18n.T("on")
	}
	if status.Running {
		state = i18n.T("running")
	}
	fmt.Fprintln(os.Stdout, state)
	return 0
}

func runEnsure(args []string) int {
	if len(args) != 0 {
		return usageError("ensure")
	}
	settings, err := config.Load()
	if err != nil {
		return report(err)
	}
	if !settings.Enabled {
		return 3
	}
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
	defer cancel()
	conn, err := client.Dial(ctx, client.Options{Client: protocol.ClientCLI, Start: true})
	if err != nil {
		return report(err)
	}
	conn.Close()
	return 0
}

func runMigrate(args []string) int {
	if len(args) != 0 {
		return usageError("migrate")
	}
	raw, _ := os.ReadFile(paths.ConfigFile())
	var doc map[string]json.RawMessage
	_ = json.Unmarshal(raw, &doc)
	if len(raw) > 0 {
		if _, versioned := doc["version"]; !versioned {
			settings, err := config.Load()
			if err != nil {
				return report(err)
			}
			if err := config.Save(settings); err != nil {
				return report(err)
			}
			fmt.Fprintln(os.Stdout, i18n.T("Migrated Rashin settings."))
		}
	}
	for _, dir := range paths.LegacyStateDirs() {
		if removeIfExists(dir) {
			fmt.Fprintln(os.Stdout, i18n.Tf("Removed %s.", dir))
		}
	}
	legacyRuntime := filepath.Join(filepath.Dir(paths.RuntimeDir()), "ryoku-rashin")
	if removeIfExists(legacyRuntime) {
		fmt.Fprintln(os.Stdout, i18n.Tf("Removed %s.", legacyRuntime))
	}
	for _, unit := range []string{"ryoku-rashin.service", "ryoku-prowl.service"} {
		path := filepath.Join(paths.ConfigHome(), "systemd", "user", unit)
		content, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		text := strings.ToLower(string(content))
		if strings.Contains(text, "ryoku-rashin") || strings.Contains(text, "prowl") {
			if os.Remove(path) == nil {
				fmt.Fprintln(os.Stdout, i18n.Tf("Removed %s.", path))
			}
		}
	}
	for _, path := range legacyHarnessConfigs() {
		content, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		text := string(content)
		if strings.Contains(text, "127.0.0.1:8788") || strings.Contains(text, "localhost:8788") {
			fmt.Fprintln(os.Stdout, i18n.Tf("Update the old Prowl gateway address in %s.", path))
		}
	}
	return 0
}

func legacyHarnessConfigs() []string {
	home := paths.Home()
	return []string{filepath.Join(home, ".hermes/config.yaml"), filepath.Join(home, ".codex/config.toml"), filepath.Join(home, ".claude/settings.json"), filepath.Join(home, ".config/opencode/opencode.json"), filepath.Join(home, ".omp/agent/models.yml")}
}

func removeIfExists(path string) bool {
	if path == "" {
		return false
	}
	if _, err := os.Stat(path); err != nil {
		return false
	}
	return os.RemoveAll(path) == nil
}
func daemonRunningWithoutDial() bool {
	if systemdPresent() {
		if systemctl, err := exec.LookPath("systemctl"); err == nil {
			if exec.Command(systemctl, "--user", "is-active", "--quiet", "rashind.service").Run() == nil {
				return true
			}
		}
	}
	f, err := os.OpenFile(paths.DaemonLock(), os.O_RDWR, 0o600)
	if err != nil {
		return false
	}
	defer f.Close()
	if syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB) != nil {
		return true
	}
	_ = syscall.Flock(int(f.Fd()), syscall.LOCK_UN)
	return false
}

func systemdPresent() bool { _, err := os.Stat("/run/systemd/system"); return err == nil }
func runQuiet(name string, args ...string) error {
	command := exec.Command(name, args...)
	command.Stdout = nil
	command.Stderr = os.Stderr
	return command.Run()
}
func usageError(usage string) int {
	fmt.Fprintln(os.Stderr, i18n.Tf("Usage: rashin %s", usage))
	return 2
}
func printJSON(value any) int {
	enc := json.NewEncoder(os.Stdout)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(value); err != nil {
		return report(err)
	}
	return 0
}

func terminateLockHolder() {
	f, err := os.OpenFile(paths.DaemonLock(), os.O_RDWR, 0o600)
	if err != nil {
		return
	}
	defer f.Close()
	if syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB) == nil {
		_ = syscall.Flock(int(f.Fd()), syscall.LOCK_UN)
		return
	}
	b := make([]byte, 64)
	n, _ := f.ReadAt(b, 0)
	pid, _ := strconv.Atoi(strings.TrimSpace(string(b[:n])))
	if pid > 1 {
		if p, err := os.FindProcess(pid); err == nil {
			_ = p.Signal(syscall.SIGTERM)
		}
	}
}
