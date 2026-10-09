package host

import (
	"bufio"
	"fmt"
	"io/fs"
	"net"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"syscall"
	"time"
)

var (
	turnstileFSRoot        = ""
	turnstileNow           = time.Now
	turnstileSocketPaths   = waylandSocketPaths
	turnstileSocketPeerPID = waylandSocketPeerPID
)

type TurnstileFinding struct {
	ID      string
	Problem string
	Fix     string
}

func turnstilePath(path string) string {
	if turnstileFSRoot == "" || !filepath.IsAbs(path) {
		return path
	}
	return filepath.Join(turnstileFSRoot, strings.TrimPrefix(path, string(filepath.Separator)))
}

func (a *App) SessionEnsure(scope string) int {
	init, err := a.Init()
	if err != nil {
		return a.failf("%v", err)
	}
	if init == Systemd {
		return ExitOK
	}
	switch scope {
	case "--system":
		if a.cfg.UID != 0 {
			return a.failf("session ensure --system must run as root")
		}
		return a.ensureTurnstileSystem()
	case "--user":
		return a.ensureTurnstileUser()
	default:
		return ExitUsage
	}
}

func (a *App) ensureTurnstileSystem() int {
	for _, name := range []string{"dbus", "turnstiled"} {
		target := turnstilePath(filepath.Join("/etc/sv", name))
		link := turnstilePath(filepath.Join("/etc/runit/runsvdir/default", name))
		changed, err := ensureExactLink(target, link)
		if err != nil {
			return a.failf("enable %s: %v", name, err)
		}
		if changed {
			fmt.Fprintf(a.cfg.Stdout, "enabled system service %s\n", name)
		}
	}
	conf := turnstilePath("/etc/turnstile/turnstiled.conf")
	changed, err := ensureTurnstileConfig(conf)
	if err != nil {
		return a.failf("update %s: %v", conf, err)
	}
	if changed {
		fmt.Fprintf(a.cfg.Stdout, "updated %s\n", conf)
	}
	pam := turnstilePath("/etc/pam.d/system-login")
	changed, err = ensureTurnstilePAM(pam)
	if err != nil {
		return a.failf("update %s: %v", pam, err)
	}
	if changed {
		fmt.Fprintf(a.cfg.Stdout, "updated %s\n", pam)
	}
	return ExitOK
}

func (a *App) ensureTurnstileUser() int {
	service := filepath.Join(a.home(), ".config", "service")
	links := []struct{ path, target string }{
		{filepath.Join(service, "dbus", "run"), turnstilePath("/usr/share/examples/turnstile/dbus.run")},
		{filepath.Join(service, "dbus", "check"), turnstilePath("/usr/share/examples/turnstile/dbus.check")},
	}
	for _, link := range links {
		changed, err := ensureExactLink(link.target, link.path)
		if err != nil {
			return a.failf("link %s: %v", link.path, err)
		}
		if changed {
			fmt.Fprintf(a.cfg.Stdout, "linked %s\n", link.path)
		}
	}
	ready := filepath.Join(service, "turnstile-ready", "conf")
	changed, err := ensureCoreService(ready, "dbus")
	if err != nil {
		return a.failf("update %s: %v", ready, err)
	}
	if changed {
		fmt.Fprintf(a.cfg.Stdout, "updated %s\n", ready)
	}
	envdir := a.envDir()
	if info, err := os.Stat(envdir); os.IsNotExist(err) {
		if err := os.MkdirAll(envdir, 0o700); err != nil {
			return a.failf("create %s: %v", envdir, err)
		}
		fmt.Fprintf(a.cfg.Stdout, "created %s\n", envdir)
	} else if err != nil || !info.IsDir() {
		return a.failf("%s is not a directory", envdir)
	}
	return ExitOK
}

func ensureExactLink(target, path string) (bool, error) {
	if current, err := os.Readlink(path); err == nil && current == target {
		return false, nil
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return false, err
	}
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return false, err
	}
	if err := os.Symlink(target, path); err != nil {
		return false, err
	}
	return true, nil
}

func ensureTurnstileConfig(path string) (bool, error) {
	body, err := os.ReadFile(path)
	if err != nil && !os.IsNotExist(err) {
		return false, err
	}
	lines := fileLines(body)
	changed := false
	for _, key := range []struct{ name, value string }{{"manage_rundir", "no"}, {"export_dbus_address", "yes"}} {
		found := false
		for index, line := range lines {
			parts := strings.SplitN(line, "=", 2)
			if len(parts) != 2 || strings.TrimSpace(parts[0]) != key.name {
				continue
			}
			found = true
			if strings.TrimSpace(parts[1]) != key.value {
				lines[index] = key.name + " = " + key.value
				changed = true
			}
			break
		}
		if !found {
			if len(lines) == 1 && lines[0] == "" {
				lines = nil
			}
			lines = append(lines, key.name+" = "+key.value)
			changed = true
		}
	}
	if !changed {
		return false, nil
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return false, err
	}
	return true, os.WriteFile(path, []byte(strings.Join(lines, "\n")+"\n"), 0o644)
}

func ensureTurnstilePAM(path string) (bool, error) {
	body, err := os.ReadFile(path)
	if err != nil {
		return false, err
	}
	if pamTurnstilePresent(body) {
		return false, nil
	}
	info, err := os.Stat(path)
	if err != nil {
		return false, err
	}
	backup := path + ".ryoku-bak"
	if _, err := os.Stat(backup); os.IsNotExist(err) {
		if err := os.WriteFile(backup, body, info.Mode().Perm()); err != nil {
			return false, err
		}
	} else if err != nil {
		return false, err
	}
	lines := fileLines(body)
	insert := -1
	lastSession := -1
	for index, line := range lines {
		fields := strings.Fields(line)
		if len(fields) == 0 || strings.TrimPrefix(fields[0], "-") != "session" {
			continue
		}
		lastSession = index
		if insert < 0 && strings.Contains(line, "pam_elogind.so") {
			insert = index
		}
	}
	if insert < 0 {
		insert = lastSession + 1
	}
	entry := "-session optional pam_turnstile.so"
	lines = append(lines, "")
	copy(lines[insert+1:], lines[insert:])
	lines[insert] = entry
	return true, os.WriteFile(path, []byte(strings.Join(lines, "\n")+"\n"), info.Mode().Perm())
}

func pamTurnstilePresent(body []byte) bool {
	for _, line := range fileLines(body) {
		fields := strings.Fields(line)
		if len(fields) == 0 || strings.TrimPrefix(fields[0], "-") != "session" {
			continue
		}
		for _, field := range fields[1:] {
			if field == "pam_turnstile.so" {
				return true
			}
		}
	}
	return false
}

func ensureCoreService(path, service string) (bool, error) {
	body, err := os.ReadFile(path)
	if err != nil && !os.IsNotExist(err) {
		return false, err
	}
	lines := fileLines(body)
	for index, line := range lines {
		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 || strings.TrimSpace(parts[0]) != "core_services" {
			continue
		}
		value := strings.Trim(strings.TrimSpace(parts[1]), "\"'")
		names := strings.Fields(value)
		for _, name := range names {
			if name == service {
				return false, nil
			}
		}
		names = append(names, service)
		lines[index] = `core_services="` + strings.Join(names, " ") + `"`
		return true, writeLines(path, lines)
	}
	if len(lines) == 1 && lines[0] == "" {
		lines = nil
	}
	lines = append(lines, `core_services="`+service+`"`)
	return true, writeLines(path, lines)
}

func writeLines(path string, lines []string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(strings.Join(lines, "\n")+"\n"), 0o644)
}

func fileLines(body []byte) []string {
	value := strings.TrimRight(string(body), "\n")
	if value == "" {
		return nil
	}
	return strings.Split(value, "\n")
}

func (a *App) SessionCheck() int {
	init, err := a.Init()
	if err != nil {
		return a.failf("%v", err)
	}
	if init == Systemd {
		return ExitOK
	}
	findings := a.turnstileFindings()
	for _, finding := range findings {
		fmt.Fprintf(a.cfg.Stdout, "%s: %s -> %s\n", finding.ID, finding.Problem, finding.Fix)
	}
	if len(findings) > 0 {
		return ExitFalse
	}
	return ExitOK
}

func (a *App) turnstileFindings() []TurnstileFinding {
	var findings []TurnstileFinding
	add := func(id, problem, fix string) { findings = append(findings, TurnstileFinding{id, problem, fix}) }
	for _, name := range []string{"dbus", "turnstiled"} {
		link := turnstilePath(filepath.Join("/etc/runit/runsvdir/default", name))
		want := turnstilePath(filepath.Join("/etc/sv", name))
		if target, err := os.Readlink(link); err != nil || target != want {
			add("turnstile-boot-"+name, name+" is not enabled at boot", "sudo ryoku-host session ensure --system")
		}
	}
	if a.probe("pgrep", "-x", "turnstiled").Code != 0 {
		add("turnstile-not-running", "turnstiled is not running", "enable and start turnstiled")
	}
	conf := turnstilePath("/etc/turnstile/turnstiled.conf")
	values := readAssignments(conf)
	if values["manage_rundir"] != "no" {
		add("turnstile-rundir", "Turnstile manages /run/user instead of elogind", "set manage_rundir = no")
	}
	if values["export_dbus_address"] != "yes" {
		add("turnstile-dbus-export", "Turnstile does not export the session bus address", "set export_dbus_address = yes")
	}
	pam := turnstilePath("/etc/pam.d/system-login")
	if body, err := os.ReadFile(pam); err != nil || !pamTurnstilePresent(body) {
		add("turnstile-pam", "system-login does not open a Turnstile session", "sudo ryoku-host session ensure --system")
	}
	service := filepath.Join(a.home(), ".config", "service")
	for _, name := range []string{"run", "check"} {
		want := turnstilePath(filepath.Join("/usr/share/examples/turnstile", "dbus."+name))
		if target, err := os.Readlink(filepath.Join(service, "dbus", name)); err != nil || target != want {
			add("turnstile-user-dbus-"+name, "the user D-Bus "+name+" link is missing", "ryoku-host session ensure --user")
		}
	}
	ready := filepath.Join(service, "turnstile-ready", "conf")
	if !coreServicePresent(ready, "dbus") {
		add("turnstile-ready-dbus", "turnstile-ready does not wait for D-Bus", "ryoku-host session ensure --user")
	}
	if info, err := os.Stat(a.envDir()); err != nil || !info.IsDir() {
		add("turnstile-user-env", "the Turnstile service environment directory is missing", "ryoku-host session ensure --user")
	}
	for _, path := range a.wrapperFiles(true) {
		if !fileHasSessionWrapper(path) {
			continue
		}
		fix := "ryoku-host session fix-wrappers"
		if strings.HasPrefix(path, turnstilePath("/usr/share/wayland-sessions")+string(filepath.Separator)) {
			fix = "replace the package-owned session file with an unwrapped entry"
		}
		add("turnstile-wrapper", path+" starts the session through a second D-Bus wrapper", fix)
	}
	findings = append(findings, a.sessionProcessFindings()...)
	sort.SliceStable(findings, func(i, j int) bool {
		if findings[i].ID == findings[j].ID {
			return findings[i].Problem < findings[j].Problem
		}
		return findings[i].ID < findings[j].ID
	})
	return findings
}

func readAssignments(path string) map[string]string {
	values := map[string]string{}
	file, err := os.Open(path)
	if err != nil {
		return values
	}
	defer file.Close()
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		parts := strings.SplitN(scanner.Text(), "=", 2)
		if len(parts) == 2 {
			values[strings.TrimSpace(parts[0])] = strings.Trim(strings.TrimSpace(parts[1]), "\"'")
		}
	}
	return values
}

func coreServicePresent(path, wanted string) bool {
	value := readAssignments(path)["core_services"]
	for _, name := range strings.Fields(value) {
		if name == wanted {
			return true
		}
	}
	return false
}

func (a *App) sessionProcessFindings() []TurnstileFinding {
	runtime := a.getenv("XDG_RUNTIME_DIR")
	if runtime == "" {
		return nil
	}
	entries, _ := os.ReadDir(turnstilePath("/proc"))
	uid := uint32(os.Getuid())
	dbusCount := 0
	var findings []TurnstileFinding
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		pid, err := strconv.Atoi(entry.Name())
		if err != nil {
			continue
		}
		base := filepath.Join(turnstilePath("/proc"), strconv.Itoa(pid))
		info, err := os.Stat(base)
		if err != nil {
			continue
		}
		stat, ok := info.Sys().(*syscall.Stat_t)
		if !ok || stat.Uid != uid {
			continue
		}
		comm := strings.TrimSpace(readText(filepath.Join(base, "comm")))
		cmdline := strings.ReplaceAll(readText(filepath.Join(base, "cmdline")), "\x00", " ")
		if comm == "dbus-daemon" && strings.Contains(cmdline, "--session") {
			dbusCount++
		}
	}
	if dbusCount > 1 {
		findings = append(findings, TurnstileFinding{"turnstile-extra-bus", fmt.Sprintf("%d user session buses are running", dbusCount), "remove session bus wrappers and log in again"})
	}
	bus := "unix:path=" + filepath.Join(runtime, "bus")
	seen := map[int]bool{}
	for _, socket := range turnstileSocketPaths(runtime, a.getenv("WAYLAND_DISPLAY")) {
		pid, err := turnstileSocketPeerPID(socket)
		if err != nil || pid <= 0 || seen[pid] {
			continue
		}
		seen[pid] = true
		base := filepath.Join(turnstilePath("/proc"), strconv.Itoa(pid))
		env := envMap(readText(filepath.Join(base, "environ")))
		if env["DBUS_SESSION_BUS_ADDRESS"] == bus {
			continue
		}
		name := strings.TrimSpace(readText(filepath.Join(base, "comm")))
		if name == "" {
			name = fmt.Sprintf("process %d", pid)
		}
		findings = append(findings, TurnstileFinding{"turnstile-compositor-bus", name + " is attached to the wrong session bus", "log out and start the session without dbus-run-session or dbus-launch"})
	}
	return findings
}

func waylandSocketPaths(runtime, preferred string) []string {
	paths, _ := filepath.Glob(filepath.Join(runtime, "wayland-*"))
	sockets := make([]string, 0, len(paths))
	for _, path := range paths {
		if strings.HasSuffix(path, ".lock") {
			continue
		}
		info, err := os.Lstat(path)
		if err == nil && info.Mode()&os.ModeSocket != 0 {
			sockets = append(sockets, path)
		}
	}
	sort.Strings(sockets)
	if preferred == "" {
		return sockets
	}
	wanted := preferred
	if !filepath.IsAbs(wanted) {
		wanted = filepath.Join(runtime, wanted)
	}
	for index, path := range sockets {
		if path != wanted {
			continue
		}
		copy(sockets[1:index+1], sockets[:index])
		sockets[0] = wanted
		break
	}
	return sockets
}

func waylandSocketPeerPID(path string) (int, error) {
	conn, err := net.DialUnix("unix", nil, &net.UnixAddr{Name: path, Net: "unix"})
	if err != nil {
		return 0, err
	}
	defer conn.Close()
	raw, err := conn.SyscallConn()
	if err != nil {
		return 0, err
	}
	var cred *syscall.Ucred
	var socketErr error
	if err := raw.Control(func(fd uintptr) {
		cred, socketErr = syscall.GetsockoptUcred(int(fd), syscall.SOL_SOCKET, syscall.SO_PEERCRED)
	}); err != nil {
		return 0, err
	}
	if socketErr != nil {
		return 0, socketErr
	}
	return int(cred.Pid), nil
}

func readText(path string) string { body, _ := os.ReadFile(path); return string(body) }
func envMap(body string) map[string]string {
	out := map[string]string{}
	for _, entry := range strings.Split(body, "\x00") {
		parts := strings.SplitN(entry, "=", 2)
		if len(parts) == 2 {
			out[parts[0]] = parts[1]
		}
	}
	return out
}

var sessionWrapperRE = regexp.MustCompile(`(^\s*Exec=|^\s*|(?:&&|\|\||;)\s*|\b(?:then|do|else)\s+)(exec\s+)?(?:dbus-run-session(?:\s+--)?|dbus-launch(?:\s+--(?:exit-with-session|sh-syntax))*)\s+(.+)$`)

func stripSessionWrappers(body string) (string, bool) {
	lines := strings.Split(body, "\n")
	changed := false
	for index, line := range lines {
		if strings.HasPrefix(strings.TrimSpace(line), "#") {
			continue
		}
		match := sessionWrapperRE.FindStringSubmatch(line)
		if match == nil {
			continue
		}
		start := strings.Index(line, match[0])
		lines[index] = line[:start] + match[1] + match[2] + match[3]
		changed = true
	}
	return strings.Join(lines, "\n"), changed
}

func fileHasSessionWrapper(path string) bool {
	body, err := os.ReadFile(path)
	if err != nil {
		return false
	}
	_, changed := stripSessionWrappers(string(body))
	return changed
}

func (a *App) SessionFixWrappers(args []string) int {
	init, err := a.Init()
	if err != nil {
		return a.failf("%v", err)
	}
	if init == Systemd {
		return ExitOK
	}
	backup := ""
	if len(args) == 2 && args[0] == "--backup-dir" {
		backup = args[1]
	} else if len(args) != 0 {
		return ExitUsage
	}
	if backup == "" {
		backup = filepath.Join(a.home(), ".local", "state", "ryoku", "session-backup", turnstileNow().UTC().Format("20060102T150405Z"))
	}
	for _, path := range a.wrapperFiles(false) {
		body, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		updated, changed := stripSessionWrappers(string(body))
		if !changed {
			continue
		}
		info, err := os.Stat(path)
		if err != nil || !userWritable(info) {
			continue
		}
		rel := strings.TrimPrefix(filepath.Clean(path), string(filepath.Separator))
		destination := filepath.Join(backup, rel)
		if err := os.MkdirAll(filepath.Dir(destination), 0o700); err != nil {
			return a.failf("backup %s: %v", path, err)
		}
		if err := os.WriteFile(destination, body, info.Mode().Perm()); err != nil {
			return a.failf("backup %s: %v", path, err)
		}
		if err := os.WriteFile(path, []byte(updated), info.Mode().Perm()); err != nil {
			return a.failf("rewrite %s: %v", path, err)
		}
		fmt.Fprintln(a.cfg.Stdout, path)
	}
	return ExitOK
}

func userWritable(info fs.FileInfo) bool {
	stat, ok := info.Sys().(*syscall.Stat_t)
	return ok && stat.Uid == uint32(os.Getuid()) && info.Mode().Perm()&0o200 != 0
}

func (a *App) wrapperFiles(includePackaged bool) []string {
	home := a.home()
	paths := []string{
		filepath.Join(home, ".bash_profile"), filepath.Join(home, ".profile"), filepath.Join(home, ".zprofile"), filepath.Join(home, ".zlogin"),
		filepath.Join(home, ".config", "fish", "config.fish"), filepath.Join(home, ".xinitrc"), filepath.Join(home, ".xsession"),
	}
	fish, _ := filepath.Glob(filepath.Join(home, ".config", "fish", "conf.d", "*.fish"))
	paths = append(paths, fish...)
	local, _ := filepath.Glob(filepath.Join(home, ".local", "share", "wayland-sessions", "*.desktop"))
	paths = append(paths, local...)
	usrLocal, _ := filepath.Glob(filepath.Join(turnstilePath("/usr/local/share/wayland-sessions"), "*.desktop"))
	paths = append(paths, usrLocal...)
	if includePackaged {
		packaged, _ := filepath.Glob(filepath.Join(turnstilePath("/usr/share/wayland-sessions"), "*.desktop"))
		paths = append(paths, packaged...)
	}
	sort.Strings(paths)
	return paths
}
