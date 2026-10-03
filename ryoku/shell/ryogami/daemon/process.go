package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"sync"
	"syscall"
)

// managedProcess supervises the picker while it is in use or inside its short keep-warm window.
type managedProcess struct {
	mu     sync.Mutex
	cmd    *exec.Cmd
	qml    string
	gpuEnv func() []string
	gpu    []string
}

func headless() bool { return os.Getenv("RYOGAMI_HEADLESS") != "" }

// gpuEnv is read at each spawn so a changed GPU preference applies on the next start.
func newPickerProcess(gpuEnv func() []string) *managedProcess {
	return &managedProcess{qml: resolveShellQML(), gpuEnv: gpuEnv}
}

// resolveShellQML finds the picker entry point: env override first (the dev
// deploy sets it on the unit), then the packaged default.
func resolveShellQML() string {
	if p := os.Getenv("RYOGAMI_SHELL_QML"); p != "" {
		return p
	}
	if p := "shell.qml"; fileExists(p) {
		abs, err := filepath.Abs(p)
		if err == nil {
			return abs
		}
	}
	return "/usr/share/ryogami/shell.qml"
}

func (m *managedProcess) running() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.runningLocked()
}

func (m *managedProcess) runningLocked() bool {
	if m.cmd == nil || m.cmd.Process == nil {
		return false
	}
	if m.cmd.ProcessState != nil {
		m.cmd = nil
		return false
	}
	return true
}

// launch starts the picker; extraEnv rides on top of the daemon's environment.
func (m *managedProcess) launch(extraEnv ...string) {
	if headless() {
		return
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.runningLocked() {
		return
	}
	m.clearLeftover()
	// The picker watches the shell's palette file; a directory that does not exist yet
	// cannot be watched, and on a first login the shell may not have created it.
	_ = os.MkdirAll(filepath.Dir(themeColorsPath()), 0o755)
	cmd := exec.Command("quickshell", "-p", m.qml)
	var gpu []string
	if m.gpuEnv != nil {
		gpu = m.gpuEnv()
	}
	cmd.Env = pickerEnv(append(extraEnv, gpu...)...)
	cmd.Stdin = nil
	if log := managedLogPath("picker"); log != "" {
		if f, err := os.OpenFile(log, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644); err == nil {
			cmd.Stdout = f
			cmd.Stderr = f
		}
	}
	if err := cmd.Start(); err != nil {
		fmt.Fprintf(os.Stderr, "ryogami: launch picker: %v\n", err)
		return
	}
	m.cmd = cmd
	m.gpu = gpu
	go func() { _ = cmd.Wait() }()
}

// clearLeftover stops a picker an earlier daemon of this session left holding the
// surface: the unit keeps children across restarts, and the previous release's
// picker ran from this same path. Matching the runtime dir leaves a picker that
// another session runs from the path alone.
func (m *managedProcess) clearLeftover() {
	runtime := os.Getenv("XDG_RUNTIME_DIR")
	procs, err := os.ReadDir("/proc")
	if err != nil {
		return
	}
	for _, p := range procs {
		pid, err := strconv.Atoi(p.Name())
		if err != nil {
			continue
		}
		raw, err := os.ReadFile(filepath.Join("/proc", p.Name(), "cmdline"))
		if err != nil {
			continue
		}
		args := strings.Split(strings.TrimRight(string(raw), "\x00"), "\x00")
		if filepath.Base(args[0]) != "quickshell" || !slices.Contains(args, m.qml) {
			continue
		}
		env, err := os.ReadFile(filepath.Join("/proc", p.Name(), "environ"))
		if err != nil || !slices.Contains(strings.Split(string(env), "\x00"), "XDG_RUNTIME_DIR="+runtime) {
			continue
		}
		_ = syscall.Kill(pid, syscall.SIGTERM)
	}
}

// The GPU is fixed at process start, so a hidden picker is swapped off-screen when the preference changes.
func (m *managedProcess) refreshIfStale() {
	if m.gpuEnv == nil || !m.running() {
		return
	}
	m.mu.Lock()
	stale := !slices.Equal(m.gpu, m.gpuEnv())
	m.mu.Unlock()
	if stale {
		m.kill()
		m.launch()
	}
}

// The daemon spawns quickshell itself and a session does not always export these paths.
func pickerImportDirs() string {
	dirs := []string{filepath.Join(home(), ".local", "lib", "qt6", "qml"), "/usr/lib/qt6/qml"}
	return strings.Join(dirs, string(os.PathListSeparator))
}

// Prepended so the Ryoku.Ryogami module resolves ahead of any inherited path.
func pickerEnv(extra ...string) []string {
	dirs := pickerImportDirs()
	base := os.Environ()
	out := make([]string, 0, len(base)+len(extra)+2)
	var haveImport, have2 bool
	for _, kv := range base {
		switch {
		case strings.HasPrefix(kv, "QML_IMPORT_PATH="):
			out = append(out, "QML_IMPORT_PATH="+prependList(dirs, kv[len("QML_IMPORT_PATH="):]))
			haveImport = true
		case strings.HasPrefix(kv, "QML2_IMPORT_PATH="):
			out = append(out, "QML2_IMPORT_PATH="+prependList(dirs, kv[len("QML2_IMPORT_PATH="):]))
			have2 = true
		default:
			out = append(out, kv)
		}
	}
	if !haveImport {
		out = append(out, "QML_IMPORT_PATH="+dirs)
	}
	if !have2 {
		out = append(out, "QML2_IMPORT_PATH="+dirs)
	}
	return append(out, extra...)
}

func prependList(prefix, existing string) string {
	if existing == "" {
		return prefix
	}
	return prefix + string(os.PathListSeparator) + existing
}

func (m *managedProcess) kill() {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.cmd != nil && m.cmd.Process != nil {
		_ = m.cmd.Process.Kill()
		m.cmd = nil
	}
}

// ensure reports whether a warm instance is up, cold-starting a visible one
// when it is not. extra rides on that cold start, so a request still lands on
// a picker that was not there to hear the event.
func (m *managedProcess) ensure(extra ...string) bool {
	if m.running() {
		return true
	}
	m.launch(append([]string{"RYOGAMI_START_VISIBLE=1"}, extra...)...)
	return false
}

func (d *daemon) refreshWorkshopOnPickerStart() {
	if !d.featureSteam() {
		return
	}
	d.rescan(false)
	d.broadcast("ryogami.workshop.changed", map[string]interface{}{})
}

// A warm picker hears the event; a cold one starts straight onto the folio.
func (d *daemon) openSettings(tab string) {
	if d.ui.ensure("RYOGAMI_START_SETTINGS=" + tab) {
		d.broadcast("ryogami.wall.settings", map[string]interface{}{"tab": tab})
	} else {
		go d.refreshWorkshopOnPickerStart()
	}
}

func managedLogPath(label string) string {
	base := os.Getenv("XDG_CACHE_HOME")
	if base == "" {
		base = filepath.Join(home(), ".cache")
	}
	dir := filepath.Join(base, "ryogami")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return ""
	}
	return filepath.Join(dir, label+".log")
}
