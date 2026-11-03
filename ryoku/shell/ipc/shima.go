package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync/atomic"
	"syscall"
	"time"
)

func shimaStateHome() string {
	if path := os.Getenv("XDG_STATE_HOME"); path != "" {
		return path
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".local", "state")
}

var activeShimaRoot atomic.Value

type desktopRun struct {
	root        string
	shima       bool
	key         string
	ready       string
	intentional atomic.Bool
	failed      atomic.Bool
}

func (d *daemon) desktopSelection() (string, string) {
	style := ""
	if d.settings != nil {
		d.settings.mu.Lock()
		style, _ = d.settings.raw["barStyle"].(string)
		d.settings.mu.Unlock()
	}
	if style != "iris" {
		d.mu.Lock()
		d.shimaBlocked = false
		d.mu.Unlock()
		if validBarStyleID(style) && style != "" {
			d.mu.Lock()
			d.shimaPrevious = style
			d.mu.Unlock()
		}
		return "native", ""
	}
	d.mu.Lock()
	blocked := d.shimaBlocked
	d.mu.Unlock()
	if blocked {
		return "native", ""
	}
	// Keep the vendored iRiS scene as the default until native Arch sessions
	// validate the standalone upstream shell. No existing installation changes.
	if os.Getenv("RYOKU_SHIMA_UPSTREAM") != "1" || os.Getenv("NIRI_SOCKET") == "" {
		return "native", ""
	}
	path := os.Getenv("RYOKU_SHIMA_LAUNCHER")
	if path == "" {
		path, _ = exec.LookPath("ryoku-shima")
	}
	if path == "" {
		return "native", ""
	}
	path, err := filepath.EvalSymlinks(path)
	if err != nil {
		return "native", ""
	}
	return path, path
}

func (d *daemon) desktopCommand() (*exec.Cmd, *desktopRun, error) {
	key, launcher := d.desktopSelection()
	run := &desktopRun{key: key, shima: launcher != ""}
	if !run.shima {
		activeShimaRoot.Store("")
		cmd := exec.Command("qs", qsSelect("shell")...)
		cmd.Env = qsEnv()
		return cmd, run, nil
	}
	out, err := exec.Command(launcher, "--root").Output()
	if err != nil {
		return nil, run, fmt.Errorf("Shima runtime discovery: %w", err)
	}
	root := strings.TrimSpace(string(out))
	if !filepath.IsAbs(root) {
		return nil, run, fmt.Errorf("Shima runtime path must be absolute")
	}
	if _, err := os.Stat(filepath.Join(root, "shell.qml")); err != nil {
		return nil, run, err
	}
	dir := os.Getenv("XDG_RUNTIME_DIR")
	if dir == "" {
		return nil, run, fmt.Errorf("Shima requires XDG_RUNTIME_DIR")
	}
	file, err := os.CreateTemp(dir, "ryoku-shima-ready-")
	if err != nil {
		return nil, run, err
	}
	run.ready = file.Name()
	file.Close()
	cmd := exec.Command(launcher)
	cmd.Env = append(qsEnv(), "RYOKU_SHIMA_MANAGED=1", "RYOKU_SHIMA_READY_FILE="+run.ready)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	run.root = root
	activeShimaRoot.Store(root)
	return cmd, run, nil
}

func shimaReady(path string, pid int) bool {
	b, err := os.ReadFile(path)
	return err == nil && strings.TrimSpace(string(b)) == strconv.Itoa(pid)
}

func terminateDesktop(cmd *exec.Cmd, group bool, exited <-chan struct{}) {
	signal := func(sig syscall.Signal) {
		if group {
			_ = syscall.Kill(-cmd.Process.Pid, sig)
		} else {
			_ = cmd.Process.Signal(sig)
		}
	}
	signal(syscall.SIGTERM)
	go func() {
		select {
		case <-exited:
		case <-time.After(2 * time.Second):
			signal(syscall.SIGKILL)
		}
	}()
}

func (d *daemon) watchDesktop(cmd *exec.Cmd, run *desktopRun, exited <-chan struct{}, timeout time.Duration) {
	tick := time.NewTicker(100 * time.Millisecond)
	defer tick.Stop()
	deadline := time.Now().Add(timeout)
	ready := !run.shima
	for {
		select {
		case <-exited:
			return
		case <-d.quit:
			run.intentional.Store(true)
			terminateDesktop(cmd, run.shima, exited)
			return
		case <-tick.C:
			wanted, _ := d.desktopSelection()
			if wanted != run.key {
				run.intentional.Store(true)
				terminateDesktop(cmd, run.shima, exited)
				return
			}
			if !ready {
				ready = shimaReady(run.ready, cmd.Process.Pid)
				if ready {
					go recordBootOK(exited)
				}
				if !ready && time.Now().After(deadline) {
					run.failed.Store(true)
					terminateDesktop(cmd, true, exited)
					return
				}
			}
		}
	}
}

func (d *daemon) restoreAfterShima(run *desktopRun, cmd *exec.Cmd) {
	if !run.shima {
		return
	}
	defer os.Remove(run.ready)
	cleanupShimaWindows(run.root, run.ready)
	// Process children used for shell watchers must not survive the root.
	_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGTERM)
	select {
	case <-d.quit:
		return
	default:
	}
	if run.intentional.Load() {
		return
	}
	if status, ok := cmd.ProcessState.Sys().(syscall.WaitStatus); ok &&
		status.Signaled() && status.Signal() == syscall.SIGTERM && !run.failed.Load() {
		return
	}
	d.fallbackShima("Shima exited or failed readiness")
}

func (d *daemon) fallbackShima(reason string) {
	if d.settings != nil {
		d.settings.mu.Lock()
		style, _ := d.settings.raw["barStyle"].(string)
		d.settings.mu.Unlock()
		if style != "iris" {
			return
		}
	}
	activeShimaRoot.Store("")
	d.mu.Lock()
	d.shimaBlocked = true
	previous := d.shimaPrevious
	d.mu.Unlock()
	if previous == "" || previous == "iris" {
		previous = "qsbar"
	}
	if d.settings != nil {
		value, _ := json.Marshal(previous)
		if err := d.settings.patch("barStyle", value); err != nil {
			fmt.Fprintf(os.Stderr, "Shima fallback could not persist: %v\n", err)
		}
	}
	fmt.Fprintf(os.Stderr, "%s; restoring %s\n", reason, previous)
}

func writeShimaFamily(path, family string, panels []string) error {
	if family != "iris" && family != "ii" && family != "waffle" {
		return fmt.Errorf("unknown Shima panel family")
	}
	info, err := os.Lstat(path)
	if err != nil {
		return err
	}
	if !info.Mode().IsRegular() {
		return fmt.Errorf("Shima config must be a regular writable file")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	var config map[string]any
	if err := json.Unmarshal(data, &config); err != nil || config == nil {
		return fmt.Errorf("invalid Shima configuration")
	}
	merge := func(key string) {
		values, _ := config[key].([]any)
		seen := map[string]bool{}
		for _, value := range values {
			if name, ok := value.(string); ok {
				seen[name] = true
			}
		}
		for _, panel := range panels {
			if !seen[panel] {
				values = append(values, panel)
				seen[panel] = true
			}
		}
		config[key] = values
	}
	config["panelFamily"] = family
	merge("enabledPanels")
	merge("knownPanels")
	file, err := os.CreateTemp(filepath.Dir(path), ".shima-family-")
	if err != nil {
		return err
	}
	defer os.Remove(file.Name())
	if err := file.Chmod(info.Mode().Perm()); err != nil {
		file.Close()
		return err
	}
	err = json.NewEncoder(file).Encode(config)
	if err == nil {
		err = file.Sync()
	}
	closeErr := file.Close()
	if err != nil {
		return err
	}
	if closeErr != nil {
		return closeErr
	}
	return os.Rename(file.Name(), path)
}

func (d *daemon) changeShimaFamily(family string, panels []string) error {
	d.desktopChangeMu.Lock()
	defer d.desktopChangeMu.Unlock()
	if _, launcher := d.desktopSelection(); launcher == "" {
		return fmt.Errorf("managed Shima is not selected")
	}
	if family != "iris" && family != "ii" && family != "waffle" {
		return fmt.Errorf("unknown Shima panel family")
	}
	d.mu.Lock()
	cmd := d.proc["shell"]
	d.mu.Unlock()
	if cmd == nil || cmd.Process == nil {
		return fmt.Errorf("Shima is not running")
	}
	d.setGate("shell", false)
	defer d.setGate("shell", true)
	deadline := time.Now().Add(5 * time.Second)
	stopped := false
	for time.Now().Before(deadline) {
		d.mu.Lock()
		stopped = d.proc["shell"] != cmd
		d.mu.Unlock()
		if stopped {
			break
		}
		if time.Until(deadline) < 3*time.Second {
			_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
		}
		time.Sleep(30 * time.Millisecond)
	}
	if !stopped {
		return fmt.Errorf("Shima did not stop; configuration preserved")
	}
	path := filepath.Join(ryokuConfigDir(), "shima", "config.json")
	return writeShimaFamily(path, family, panels)
}

func ownedShimaWindow(argv []string, environ, root, token string) bool {
	if len(argv) < 3 || root == "" || token == "" {
		return false
	}
	exe := strings.TrimSuffix(strings.TrimPrefix(filepath.Base(argv[0]), "."), "-wrapped")
	if exe != "qs" && exe != "quickshell" {
		return false
	}
	marked := false
	for _, kv := range strings.Split(environ, "\x00") {
		if kv == "RYOKU_SHIMA_READY_FILE="+token {
			marked = true
			break
		}
	}
	if !marked {
		return false
	}
	for i := 1; i < len(argv)-1; i++ {
		if argv[i] == "-p" || argv[i] == "--path" {
			path := filepath.Clean(argv[i+1])
			return (path == root || strings.HasPrefix(path, root+string(os.PathSeparator))) && instanceTail(argv[i+2:])
		}
	}
	return false
}

func cleanupShimaWindows(root, token string) {
	entries, err := os.ReadDir("/proc")
	if err != nil {
		return
	}
	for _, entry := range entries {
		pid, err := strconv.Atoi(entry.Name())
		if err != nil || pid == os.Getpid() {
			continue
		}
		dir := filepath.Join("/proc", entry.Name())
		args, err := os.ReadFile(filepath.Join(dir, "cmdline"))
		if err != nil {
			continue
		}
		env, err := os.ReadFile(filepath.Join(dir, "environ"))
		if err != nil {
			continue
		}
		argv := strings.Split(strings.TrimRight(string(args), "\x00"), "\x00")
		if ownedShimaWindow(argv, string(env), root, token) {
			_ = syscall.Kill(pid, syscall.SIGTERM)
		}
	}
}
