package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
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
	"ryoku-rashin/internal/migration"
	"ryoku-rashin/internal/paths"
	"ryoku-rashin/internal/protocol"
)

func init() {
	register(&Command{Name: "on", Group: "lifecycle", Usage: "[--if-unset]", Summary: "Turn Hyakki on.", Run: runOn})
	register(&Command{Name: "off", Group: "lifecycle", Summary: "Turn Hyakki off.", Run: runOff})
	register(&Command{Name: "status", Group: "lifecycle", Usage: "[--json]", Summary: "Show Hyakki status without starting it.", Run: runStatus})
	register(&Command{Name: "ensure", Group: "lifecycle", Summary: "Make sure the daemon is ready.", Run: runEnsure})
	register(&Command{Name: "migrate", Group: "lifecycle", Usage: "[--project <folder>|--restore <backup>]", Summary: "Relocate legacy app data or restore its backup.", Run: runMigrate})
}

func runOn(args []string) int {
	if len(args) > 1 || len(args) == 1 && args[0] != "--if-unset" {
		return usageError("on")
	}
	if len(args) == 1 {
		enabled, err := config.EnableIfUnset(enableNativeUserUnits)
		if err != nil {
			return report(err)
		}
		if !enabled {
			fmt.Fprintln(os.Stdout, i18n.T("Hyakki is off."))
			return 3
		}
	} else if err := config.SetEnabled(true); err != nil {
		return report(err)
	}
	if len(args) == 0 {
		if err := enableNativeUserUnits(); err != nil {
			return report(err)
		}
	}
	fmt.Fprintln(os.Stdout, i18n.T("Hyakki is on."))
	if runitPresent() {
		runitSteps(os.Stdout, true)
	}
	return 0
}

func enableNativeUserUnits() error {
	for _, unit := range []string{"oni.socket", "hyakki-usage.timer"} {
		if userUnitAvailable(unit) {
			if err := runQuiet("systemctl", "--user", "enable", "--now", unit); err != nil {
				return err
			}
		}
	}
	return nil
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
	var stopErr error
	for _, unit := range []string{"oni.socket", "oni.service", "hyakki-usage.timer", "hyakki-usage.service", "oni-routines.timer", "oni-routines.service"} {
		if userUnitAvailable(unit) {
			if err := runQuiet("systemctl", "--user", "disable", "--now", unit); err != nil {
				stopErr = errors.Join(stopErr, err)
			}
		}
	}
	terminateLockHolder()
	if stopErr != nil {
		return report(stopErr)
	}
	fmt.Fprintln(os.Stdout, i18n.T("Hyakki is off."))
	if runitPresent() {
		runitSteps(os.Stdout, false)
	}
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
	if len(args) == 2 && args[0] == "--restore" {
		if err := migration.Restore(args[1]); err != nil {
			return report(err)
		}
		fmt.Fprintln(os.Stdout, i18n.T("Restored the migration backup. Current Hyakki data was retained."))
		return 0
	}
	if len(args) != 0 && !(len(args) == 2 && args[0] == "--project") {
		return usageError("migrate [--project <folder>|--restore <backup>]")
	}
	if err := migration.Ensure(); err != nil {
		return report(err)
	}
	if len(args) == 2 {
		if err := migration.Project(args[1]); err != nil {
			return report(err)
		}
	}
	fmt.Fprintln(os.Stdout, i18n.T("The Hyakki data layout is current."))
	return 0
}

func userUnitAvailable(unit string) bool {
	systemctl, err := exec.LookPath("systemctl")
	if err != nil {
		return false
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	output, err := exec.CommandContext(ctx, systemctl, "--user", "show", "--property=LoadState", "--value", unit).Output()
	return err == nil && strings.TrimSpace(string(output)) == "loaded"
}
func daemonRunningWithoutDial() bool {
	if systemdPresent() {
		if systemctl, err := exec.LookPath("systemctl"); err == nil {
			if exec.Command(systemctl, "--user", "is-active", "--quiet", "oni.service").Run() == nil {
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

// initRoot is the root the init-system probes read; tests point it at a
// fixture tree.
var initRoot = "/"

// lookPath finds a program on PATH; tests replace it.
var lookPath = exec.LookPath

func systemdPresent() bool {
	_, err := os.Stat(filepath.Join(initRoot, "run/systemd/system"))
	return err == nil
}

// runitPresent reports a session supervised by runit rather than systemd:
// runit as process 1, as on Void, or runit's own /run/runit directory.
func runitPresent() bool {
	if systemdPresent() {
		return false
	}
	if comm, err := os.ReadFile(filepath.Join(initRoot, "proc/1/comm")); err == nil && strings.TrimSpace(string(comm)) == "runit" {
		return true
	}
	info, err := os.Stat(filepath.Join(initRoot, "run/runit"))
	return err == nil && info.IsDir()
}

// runitTemplates is where the packages install the dormant runit services.
const runitTemplates = "/usr/share/hyakki/runit"

// runitServices are the templates, in the order the steps name them.
var runitServices = []string{"oni", "oni-routines", "hyakki-usage"}

// runitSteps prints the commands that arm (on) or park (off) the optional
// runit services. It writes nothing: the user's service directory is
// theirs, and on Ryoku ryoku-host supervises the same services.
func runitSteps(w io.Writer, on bool) {
	if _, err := lookPath("ryoku-host"); err == nil {
		fmt.Fprintln(w, i18n.T("This session runs runit and Ryoku supervises Hyakki's services. No service was changed. To match, run:"))
		if on {
			fmt.Fprintln(w, "  ryoku-host svc --user enable --now hyakki-usage")
			return
		}
		fmt.Fprintln(w, "  ryoku-host svc --user disable --now hyakki-usage")
		fmt.Fprintln(w, "  ryoku-host svc --user disable --now oni-routines")
		return
	}
	dir := runitServiceDir()
	shown := `"$SVDIR"`
	if dir != "" {
		shown = shellQuote(dir)
	}
	if on {
		fmt.Fprintln(w, i18n.T("This session runs runit, so no service was changed. Clients start Oni when they need it. To supervise the usage refresh and the routine wake, run:"))
		if dir == "" {
			fmt.Fprintln(w, i18n.T("  (set SVDIR to the folder your user runsvdir watches first)"))
		}
		var missing []string
		for _, name := range []string{"oni-routines", "hyakki-usage"} {
			if dir == "" || !runitInstalled(dir, name) {
				missing = append(missing, runitTemplates+"/"+name)
			}
		}
		if len(missing) > 0 {
			fmt.Fprintf(w, "  cp -R %s %s/\n", strings.Join(missing, " "), shown)
		}
		fmt.Fprintf(w, "  sv up %s/oni-routines %s/hyakki-usage\n", shown, shown)
		return
	}
	var present []string
	for _, name := range runitServices {
		if dir == "" || runitInstalled(dir, name) {
			present = append(present, shown+"/"+name)
		}
	}
	if len(present) == 0 {
		fmt.Fprintln(w, i18n.Tf("This session runs runit. No Hyakki service is in %s, so there is nothing to park.", dir))
		return
	}
	fmt.Fprintln(w, i18n.T("This session runs runit, so no service was changed. To park the supervised services, run:"))
	if dir == "" {
		fmt.Fprintln(w, i18n.T("  (set SVDIR to the folder your user runsvdir watches first)"))
	}
	fmt.Fprintf(w, "  sv down %s\n", strings.Join(present, " "))
	fmt.Fprintln(w, i18n.T("Before removing the package, also run:"))
	fmt.Fprintf(w, "  rm -r %s\n", strings.Join(present, " "))
}

// runitServiceDir finds the user's runit service folder: SVDIR when set,
// else Turnstile's ~/.config/service or the classic ~/service. Empty when
// none exists.
func runitServiceDir() string {
	if dir := os.Getenv("SVDIR"); filepath.IsAbs(dir) {
		return filepath.Clean(dir)
	}
	var candidates []string
	if config, err := os.UserConfigDir(); err == nil {
		candidates = append(candidates, filepath.Join(config, "service"))
	}
	if home, err := os.UserHomeDir(); err == nil {
		candidates = append(candidates, filepath.Join(home, "service"))
	}
	for _, dir := range candidates {
		if info, err := os.Stat(dir); err == nil && info.IsDir() {
			return dir
		}
	}
	return ""
}

func runitInstalled(dir, name string) bool {
	info, err := os.Stat(filepath.Join(dir, name, "run"))
	return err == nil && info.Mode().IsRegular()
}

// shellQuote quotes a path for a copied command line.
func shellQuote(s string) string {
	if s != "" && strings.Trim(s, "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789/._-+") == "" {
		return s
	}
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}

func runQuiet(name string, args ...string) error {
	command := exec.Command(name, args...)
	command.Stdout = nil
	command.Stderr = os.Stderr
	return command.Run()
}
func usageError(usage string) int {
	fmt.Fprintln(os.Stderr, i18n.Tf("Usage: hyakki %s", usage))
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
