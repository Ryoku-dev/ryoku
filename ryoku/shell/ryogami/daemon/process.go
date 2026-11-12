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
	"time"
)

const (
	pickerReleaseSetting = "performance.releaseAfterHideSeconds"
	pickerReconcileWait  = 5 * time.Minute
	pickerStopGrace      = 2 * time.Second
)

// managedProcess supervises the picker and serializes every stop and start.
type managedProcess struct {
	mu           sync.Mutex
	cmd          *exec.Cmd
	done         chan struct{}
	stopping     bool
	command      func(string, ...string) *exec.Cmd
	qml          string
	gpuEnv       func() []string
	idleGrace    func() time.Duration
	gpu          []string
	ready        bool
	startVisible bool
	subscriberID uint64
	pending      pickerPendingEvent
	hidden       bool
	hiddenSince  time.Time
	busy         bool
	parkWake     chan struct{}
}

type pickerPendingEvent struct {
	name string
	data map[string]interface{}
}

func headless() bool { return os.Getenv("RYOGAMI_HEADLESS") != "" }

// Runtime settings are read for every park decision; GPU preference is read
// for every spawn.
func newPickerProcess(gpuEnv func() []string, idleGrace func() time.Duration) *managedProcess {
	return &managedProcess{
		qml:       resolveShellQML(),
		gpuEnv:    gpuEnv,
		idleGrace: idleGrace,
		command:   exec.Command,
		parkWake:  make(chan struct{}, 1),
	}
}

// resolveShellQML finds the picker entry point: an explicit immutable override
// first, then the entry installed in this user's runtime XDG data home, then the
// packaged default.
func resolveShellQML() string {
	if p := os.Getenv("RYOGAMI_SHELL_QML"); p != "" {
		return resolvePath(p)
	}
	if p := "shell.qml"; fileExists(p) {
		abs, err := filepath.Abs(p)
		if err == nil {
			return abs
		}
	}
	if p := filepath.Join(dataHome(), "ryogami", "shell.qml"); fileExists(p) {
		return p
	}
	return "/usr/share/ryogami/shell.qml"
}

func pickerParkDue(since time.Time, hidden, busy bool, grace time.Duration, now time.Time) bool {
	return grace > 0 && hidden && !busy && now.Sub(since) >= grace
}

func (d *daemon) pickerIdleGrace() time.Duration {
	seconds := d.settingNumber(pickerReleaseSetting)
	if seconds <= 0 {
		return 0
	}
	return time.Duration(seconds * float64(time.Second))
}

func (m *managedProcess) idleGraceNow() time.Duration {
	if m.idleGrace == nil {
		return 0
	}
	grace := m.idleGrace()
	if grace < 0 {
		return 0
	}
	return grace
}

func init() {
	watchSetting(pickerReleaseSetting, func(d *daemon, key string, _ interface{}) {
		if key == pickerReleaseSetting && d.ui != nil {
			d.ui.kickPark()
		}
	})
}

func (m *managedProcess) clearRuntimeLocked() {
	m.ready = false
	m.startVisible = false
	m.subscriberID = 0
	m.pending = pickerPendingEvent{}
	m.busy = false
}

func (m *managedProcess) running() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.runningLocked()
}

func (m *managedProcess) runningLocked() bool {
	return m.cmd != nil && m.cmd.Process != nil && m.cmd.ProcessState == nil
}

// launch starts the resident picker; extraEnv rides on top of the daemon's
// environment (RYOGAMI_START_VISIBLE=1 makes a cold instance show itself, the
// recovery path when the resident one has died).
func (m *managedProcess) launch(extraEnv ...string) {
	if headless() {
		return
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.launchLocked(extraEnv...)
}

func (m *managedProcess) launchLocked(extraEnv ...string) {
	if m.stopping || m.runningLocked() {
		return
	}
	m.clearRuntimeLocked()
	m.clearLeftover()
	// The picker watches the shell's palette file; a directory that does not exist yet
	// cannot be watched, and on a first login the shell may not have created it.
	_ = os.MkdirAll(filepath.Dir(themeColorsPath()), 0o755)
	command := m.command
	if command == nil {
		command = exec.Command
	}
	cmd := command("quickshell", "-p", m.qml)
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
	done := make(chan struct{})
	m.cmd = cmd
	m.done = done
	m.gpu = gpu
	m.startVisible = slices.Contains(extraEnv, "RYOGAMI_START_VISIBLE=1")
	go func() {
		_ = cmd.Wait()
		m.mu.Lock()
		if m.cmd == cmd {
			m.cmd = nil
			m.done = nil
			m.stopping = false
			m.clearRuntimeLocked()
		}
		close(done)
		m.mu.Unlock()
	}()
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

func (m *managedProcess) kickPark() {
	select {
	case m.parkWake <- struct{}{}:
	default:
	}
}

func (m *managedProcess) updateState(visible, busy bool) {
	m.mu.Lock()
	if visible {
		m.hidden = false
		m.hiddenSince = time.Time{}
	} else if !m.hidden {
		m.hidden = true
		m.hiddenSince = time.Now()
	}
	m.busy = busy
	m.mu.Unlock()
	m.kickPark()
}

func (m *managedProcess) idlePark() {
	timer := time.NewTimer(0)
	defer timer.Stop()
	for {
		now := time.Now()
		grace := m.idleGraceNow()
		wait := pickerReconcileWait

		m.mu.Lock()
		hidden, busy, since := m.hidden, m.busy, m.hiddenSince
		m.mu.Unlock()

		if pickerParkDue(since, hidden, busy, grace, now) {
			m.parkIfDue(now)
			continue
		}
		if grace > 0 && hidden && !busy {
			if left := grace - now.Sub(since); left < wait {
				wait = left
			}
		}
		if wait < 0 {
			wait = 0
		}
		if !timer.Stop() {
			select {
			case <-timer.C:
			default:
			}
		}
		timer.Reset(wait)
		select {
		case <-m.parkWake:
		case <-timer.C:
		}
	}
}

func (m *managedProcess) beginStopLocked(graceful bool) (*exec.Cmd, <-chan struct{}) {
	if m.cmd == nil || m.cmd.Process == nil {
		return nil, nil
	}
	cmd, done := m.cmd, m.done
	if graceful {
		if !m.stopping {
			m.stopping = true
			_ = cmd.Process.Signal(syscall.SIGTERM)
		}
	} else {
		m.stopping = true
		_ = cmd.Process.Kill()
	}
	return cmd, done
}

func (m *managedProcess) waitForStop(cmd *exec.Cmd, done <-chan struct{}, graceful bool) {
	if cmd == nil || done == nil {
		return
	}
	if !graceful {
		<-done
		return
	}
	timer := time.NewTimer(pickerStopGrace)
	defer timer.Stop()
	select {
	case <-done:
		return
	case <-timer.C:
		m.mu.Lock()
		if m.cmd == cmd {
			_ = cmd.Process.Kill()
		}
		m.mu.Unlock()
		<-done
	}
}

func (m *managedProcess) parkIfDue(now time.Time) {
	grace := m.idleGraceNow()
	m.mu.Lock()
	if !pickerParkDue(m.hiddenSince, m.hidden, m.busy, grace, now) {
		m.mu.Unlock()
		return
	}
	m.hidden = false
	m.hiddenSince = time.Time{}
	cmd, done := m.beginStopLocked(true)
	m.mu.Unlock()
	m.waitForStop(cmd, done, true)
}

// The GPU is fixed at process start, so a hidden picker is swapped off-screen when the preference changes.
func (m *managedProcess) refreshIfStale() {
	if m.gpuEnv == nil {
		return
	}
	m.mu.Lock()
	if !m.hidden || m.stopping || !m.runningLocked() || slices.Equal(m.gpu, m.gpuEnv()) {
		m.mu.Unlock()
		return
	}
	m.hidden = false
	m.hiddenSince = time.Time{}
	cmd, done := m.beginStopLocked(true)
	m.mu.Unlock()
	m.waitForStop(cmd, done, true)
	m.launch()
}

// The daemon spawns quickshell itself and a session does not always export these paths.
func pickerImportDirs() string {
	dirs := []string{
		filepath.Join(home(), ".local", "lib", "qt6", "qml"),
		"/usr/lib/qt6/qml",
		"/usr/lib64/qt6/qml",
	}
	return strings.Join(dirs, string(os.PathListSeparator))
}

// pickerDiskCacheEnv keeps the picker off Qt's on-disk shader and pipeline
// caches. Every quickshell process shares one cache file, and on NVIDIA's
// OpenGL driver programs reloaded from it drew the picker's text scrambled.
// The picker can be restarted after an idle release, so the driver caches stay
// disabled even though each cold open recompiles its pipeline.
//
// Video previews decode in software, as the shell's own clips do: on hybrid
// NVIDIA boxes hardware-decoded frames (NVDEC, shared into GL through CUDA)
// tore the whole picker into shards while the download browser played live
// wallpaper previews. The previews are small loops, so software decode is cheap.
var pickerDiskCacheEnv = []string{
	"QSG_RHI_DISABLE_DISK_CACHE=1",
	"QT_DISABLE_SHADER_DISK_CACHE=1",
	"QT_MEDIA_BACKEND=ffmpeg",
	"QT_FFMPEG_DECODING_HW_DEVICE_TYPES=",
}

// Import dirs are prepended so the Ryoku.Ryogami module resolves ahead of any
// inherited path.
func pickerEnv(extra ...string) []string {
	dirs := pickerImportDirs()
	base := os.Environ()
	out := make([]string, 0, len(base)+len(extra)+len(pickerDiskCacheEnv)+2)
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
	out = append(out, pickerDiskCacheEnv...)
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
	cmd, done := m.beginStopLocked(false)
	m.mu.Unlock()
	m.waitForStop(cmd, done, false)
}

// ensure reports whether the picker is ready to receive an event. A request
// arriving during a graceful park waits for that process to exit before one
// cold visible start; a request arriving while a start connects is retained.
func (m *managedProcess) ensure(extra ...string) bool {
	for {
		m.mu.Lock()
		m.hidden = false
		m.hiddenSince = time.Time{}
		if m.stopping {
			done := m.done
			if done == nil {
				m.stopping = false
				m.mu.Unlock()
				continue
			}
			m.mu.Unlock()
			m.kickPark()
			<-done
			continue
		}
		if m.runningLocked() {
			if m.ready {
				m.mu.Unlock()
				m.kickPark()
				return true
			}
			pending := pickerPendingFor(extra)
			if pending.name != "ryogami.wall.toggle" || !m.startVisible {
				m.pending = pending
			}
			m.mu.Unlock()
			m.kickPark()
			return false
		}
		if !headless() {
			m.launchLocked(append([]string{"RYOGAMI_START_VISIBLE=1"}, extra...)...)
		}
		m.mu.Unlock()
		m.kickPark()
		return false
	}
}

func (m *managedProcess) subscribed(id uint64, deliver func(string, map[string]interface{})) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.pending.name != "" {
		deliver(m.pending.name, m.pending.data)
		m.pending = pickerPendingEvent{}
	}
	m.subscriberID = id
	m.ready = true
}

func (m *managedProcess) unsubscribed(id uint64) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.subscriberID == id {
		m.subscriberID = 0
		m.ready = false
	}
}

// pickerPendingFor turns a cold-start environment into the event a starting
// picker receives once it subscribes, so either path lands on the same view.
func pickerPendingFor(extra []string) pickerPendingEvent {
	env := map[string]string{}
	for _, entry := range extra {
		if k, v, ok := strings.Cut(entry, "="); ok {
			env[k] = v
		}
	}
	if tab, ok := env["RYOGAMI_START_SETTINGS"]; ok {
		return pickerPendingEvent{name: "ryogami.wall.settings", data: settingsTarget(tab, env["RYOGAMI_START_SECTION"], env["RYOGAMI_START_CONTROL"])}
	}
	if name, ok := env["RYOGAMI_START_COLLECTION"]; ok {
		return pickerPendingEvent{name: "ryogami.wall.collection", data: map[string]interface{}{"name": name}}
	}
	return pickerPendingEvent{name: "ryogami.wall.toggle", data: map[string]interface{}{}}
}

func settingsTarget(tab, section, control string) map[string]interface{} {
	return map[string]interface{}{"tab": tab, "section": section, "control": control}
}

// A resident picker hears the event; a dead one cold-starts straight onto the folio.
func (d *daemon) openSettings(tab, section, control string) {
	if d.ui.ensure("RYOGAMI_START_SETTINGS="+tab, "RYOGAMI_START_SECTION="+section, "RYOGAMI_START_CONTROL="+control) {
		d.broadcast("ryogami.wall.settings", settingsTarget(tab, section, control))
	}
}

var pickerCollections = []string{"wallpapers", "themes", "rices", "workshop"}

// openCollection shows the picker on one library collection, cold-starting it there if needed.
func (d *daemon) openCollection(name string) error {
	if !slices.Contains(pickerCollections, name) {
		return fmt.Errorf("collection expects %s", strings.Join(pickerCollections, "|"))
	}
	if d.ui.ensure("RYOGAMI_START_COLLECTION=" + name) {
		d.broadcast("ryogami.wall.collection", map[string]interface{}{"name": name})
	}
	return nil
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
