package main

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	wm "ryoku-wm"
)

// component is a Quickshell config the daemon keeps alive. Persistent components
// start with the daemon and are restarted if they exit; the rest are started the
// first time a command needs them.
type component struct {
	name       string
	persistent bool
}

var components = []component{
	// One consolidated instance renders every surface in-process (bar, backdrop,
	//	launcher, visualizer, widgets, overview), replacing the old
	// per-surface Quickshell configs. Always-on, as the pill frame was.
	{"shell", true},
}

func reloadCoverPath() string {
	if shellDir != "" {
		return filepath.Join(shellDir, "scripts", "ryoku-reload-cover")
	}
	if exe, err := os.Executable(); err == nil {
		return filepath.Join(filepath.Dir(exe), "ryoku-reload-cover")
	}
	return "ryoku-reload-cover"
}

var reloadCoverOutput = func(ctx context.Context) ([]byte, error) {
	return exec.CommandContext(ctx, reloadCoverPath(), "begin").Output()
}

var reloadCoverBegin = func() string {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	out, err := reloadCoverOutput(ctx)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

func beginReloadCover() string {
	return reloadCoverBegin()
}

// parseDisabledComponents pulls the "disabledComponents" string array from a
// performance.json body. shell is never returned: it is the whole in-process
// desktop, so turning it off is not offered. A malformed or absent list
// disables nothing.
func parseDisabledComponents(b []byte) map[string]bool {
	var m struct {
		Disabled []string `json:"disabledComponents"`
	}
	out := map[string]bool{}
	if json.Unmarshal(b, &m) != nil {
		return out
	}
	for _, name := range m.Disabled {
		if name != "shell" {
			out[name] = true
		}
	}
	return out
}

// componentDisabled reports whether a component is turned off via
// performance.json's disabledComponents array. Disabled components never start,
// at boot or on demand. shell is always on; a missing file disables nothing.
func componentDisabled(name string) bool {
	if name == "shell" {
		return false
	}
	// ryoku: with surfaces consolidated into the single shell, per-surface disable
	// is now internal to the shell; performance.json's disabledComponents no longer
	// maps to separate processes, so this only ever guards components that no
	// longer exist.
	b, err := os.ReadFile(perfPath())
	if err != nil {
		return false
	}
	return parseDisabledComponents(b)[name]
}

type daemon struct {
	mu           sync.Mutex
	sup          map[string]bool      // components that already have a supervisor goroutine
	proc         map[string]*exec.Cmd // current live process per component
	paintSig     chan struct{}        // coalescing wake for the palette/border worker
	stageSig     chan struct{}        // coalescing wake for the unified stage worker
	stageForce   atomic.Bool          // a pending forced regenerate (effect/quality change, refresh, re-cut)
	stageGen     atomic.Bool          // a pending enable: reuse an existing cut, else generate
	stageBusy    atomic.Bool          // a cut/inpaint is in flight (for the status/topic)
	ledsSig      chan struct{}        // coalescing wake for the OpenRGB worker
	widgetSig    chan struct{}        // coalescing wake for the widget-occupancy gate
	quit         chan struct{}
	closed       bool
	ln           net.Listener
	lock         *os.File // exclusive single-daemon guard, held until exit
	failMu       sync.Mutex
	lastFail     map[string]string // component -> last line it died with
	voiceMu      sync.Mutex        // serializes voice (Super+`) toggles
	voiceOn      bool              // dictation active; guarded by voiceMu
	voiceStop    chan struct{}     // reaps the live voxtype state stream; nil when none
	prompter     *prompter         // GNOME keyring system prompter (nil when unavailable)
	wmc          *wm.Client        // sole path to the compositor
	wmMu         sync.Mutex        // guards the compositor state the wm watcher keeps warm
	activeMon    string            // focused output, kept warm by watchWindowManager
	wmOutputs    []wm.Output
	wmWorkspaces []wm.Workspace
	wmWindows    []wm.Window
	wmKbdLayout  string   // active xkb layout, kept warm by watchWindowManager
	wmKbdList    []string // configured layouts, in switch order
	wmOverview   bool     // the compositor's native overview is open (niri)
	wmReady      bool
	wmVersions   map[string]int // frame kind -> publishes since daemon start
	wmTopic      *stateTopic
	gateMu       sync.Mutex               // guards gateWant / gateWake
	gateWant     map[string]bool          // component -> may run now (absent = yes)
	gateWake     map[string]chan struct{} // wakes a parked supervisor when its gate opens
	parkMu       sync.Mutex               // guards hiddenSince
	hiddenSince  map[string]time.Time     // parkable palette -> when it last went hidden (absent = shown)
	topicsMu     sync.Mutex               // guards topics
	topics       map[string]*stateTopic   // subsystem name -> pub/sub state topic
	callsMu      sync.Mutex               // guards calls
	calls        map[string]callFunc      // "topic.method" -> control handler
	clip         *clipState               // clipboard history state (nil until started)
	tray         *trayState               // system tray watcher/host state (nil until started)
	ryoWallMu    sync.Mutex               // guards ryoWall
	ryoWall      ryogamiFrame             // last wallpaper frame seen from ryogami; feeds the stage worker
	polkit       *polkitAgent             // PolicyKit1 authentication agent (nil until started)
	settings     *settingsStore           // shell.json store (nil until startSettings); theme apply patches through it
	pp           *powerProfilesState      // power-profiles-daemon bus state; nil until startPowerProfiles
	keypress     *keypressManager         // evdev key stream; opens devices only while the overlay is enabled
}

func runDaemon() error {
	// The provider (not this daemon) binds to the live compositor. Its opaque
	// per-session instance handle is the only thing the take-over needs.
	wmc := wm.Open()
	path := sockPath()
	if c, err := net.DialTimeout("unix", path, 300*time.Millisecond); err == nil {
		c.Close()
		// Take over only a provably stale incumbent: one bound to a different
		// compositor instance than ours (a previous session's). A same-session
		// incumbent, an unidentified one (older binary, ok=false), or our own
		// missing instance are left alone, so a genuine double-start refuses.
		caps, _ := wmc.Caps()
		incInstance, ok := daemonSignature(path)
		if !shouldTakeOver(caps.Instance, incInstance, ok) {
			return fmt.Errorf("a daemon is already running at %s", path)
		}
		quitStaleDaemon(path)
	}
	// A lock, not just the socket: a runtime dir cleaned under a live daemon (or
	// a socket file removed by hand) makes the dial above succeed at nothing, and
	// two daemons each supervising a desktop is the duplicate-shell bug. The lock
	// lives for the life of the process and is released by exit, so a crash never
	// wedges the next start.
	lock, err := holdDaemonLock(path + ".lock")
	if err != nil {
		return err
	}
	_ = os.Remove(path)
	// The control socket drives session-scoped actions; keep it owner-only so a
	// second local user can't connect. net.Listen would otherwise leave it at
	// the ambient umask (0755 at the usual 022 -> unconnectable by others, but
	// that is luck, not policy). Forcing the umask around Listen makes the
	// socket 0700 atomically, with no world-visible window to chmod after.
	old := syscall.Umask(0o077)
	ln, err := net.Listen("unix", path)
	syscall.Umask(old)
	if err != nil {
		return err
	}

	d := &daemon{
		sup:         map[string]bool{},
		proc:        map[string]*exec.Cmd{},
		paintSig:    make(chan struct{}, 1),
		stageSig:    make(chan struct{}, 1),
		ledsSig:     make(chan struct{}, 1),
		widgetSig:   make(chan struct{}, 1),
		quit:        make(chan struct{}),
		gateWant:    map[string]bool{},
		gateWake:    map[string]chan struct{}{},
		hiddenSince: map[string]time.Time{},
		lastFail:    map[string]string{},
		wmc:         wmc,
	}
	d.ln = ln
	d.lock = lock // held for the process lifetime: closing it would free the guard

	go func() {
		sig := make(chan os.Signal, 1)
		signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
		<-sig
		d.signalQuit()
	}()

	setupQmlImportPath()

	go d.runtimeJanitor()
	d.bootstrap()

	for {
		conn, err := ln.Accept()
		if err != nil {
			select {
			case <-d.quit:
				d.shutdown()
				_ = os.Remove(path)
				return nil
			default:
				continue
			}
		}
		go d.handle(conn)
	}
}

// shouldTakeOver reports whether a daemon starting now should displace the
// incumbent on the control socket. It takes over only a provably stale
// incumbent: one whose compositor instance handle (ok) differs from ours
// (mySig). A same-session incumbent, an unidentified one (older binary,
// ok=false), or our own missing handle (mySig=="") leave it in place, so a
// genuine double-start still refuses.
func shouldTakeOver(mySig, incSig string, ok bool) bool {
	return ok && mySig != "" && incSig != mySig
}

// daemonSignature asks the daemon at path for its compositor instance handle. ok
// is false when the query fails or the reply is an error (an older daemon that
// predates the verb), so the caller treats the incumbent as unidentified and
// does not displace it. An empty handle from a current daemon is a valid answer
// (ok=true, sig="").
func daemonSignature(path string) (sig string, ok bool) {
	conn, err := net.DialTimeout("unix", path, 300*time.Millisecond)
	if err != nil {
		return "", false
	}
	defer conn.Close()
	_ = conn.SetDeadline(time.Now().Add(time.Second))
	if _, err := fmt.Fprintln(conn, "signature"); err != nil {
		return "", false
	}
	buf := make([]byte, 4096)
	n, _ := conn.Read(buf)
	resp := strings.TrimSpace(string(buf[:n]))
	if strings.HasPrefix(resp, "err ") {
		return "", false
	}
	return resp, true
}

// quitStaleDaemon tells the incumbent to quit and waits, bounded, for it to
// release the control socket so this daemon can bind. The incumbent reaps its
// own supervised quickshell children on quit, so the takeover strands no process
// holding a single-instance lock.
func quitStaleDaemon(path string) {
	if conn, err := net.DialTimeout("unix", path, 300*time.Millisecond); err == nil {
		_ = conn.SetDeadline(time.Now().Add(time.Second))
		fmt.Fprintln(conn, "quit")
		conn.Close()
	}
	for range 30 {
		conn, err := net.DialTimeout("unix", path, 100*time.Millisecond)
		if err != nil {
			return
		}
		conn.Close()
		time.Sleep(100 * time.Millisecond)
	}
}

// setupQmlImportPath puts the active shell config root and home-installed
// Ryoku plugins on the import path inherited by supervised Quickshell
// processes. External Store scenes import stable modules from the config root;
// dev sessions point at the checkout instead of a materialized copy.
func setupQmlImportPath() {
	home, err := os.UserHomeDir()
	if err != nil {
		return
	}
	configRoot := os.Getenv("XDG_CONFIG_HOME")
	if configRoot == "" {
		configRoot = filepath.Join(home, ".config")
	}
	shellRoot := filepath.Join(configRoot, "quickshell")
	if shellDir != "" {
		shellRoot = filepath.Join(shellDir, "quickshell")
	}
	dirs := []string{shellRoot}
	exe, _ := os.Executable()
	if strings.HasPrefix(exe, home+string(os.PathSeparator)) {
		dirs = append(dirs, filepath.Join(home, ".local", "lib", "qt6", "qml"))
	} else if _, err := os.Stat("/usr/lib/qt6/qml/Ryoku/Blobs/qmldir"); err != nil {
		dirs = append(dirs, filepath.Join(home, ".local", "lib", "qt6", "qml"))
	}
	prefix := strings.Join(dirs, string(os.PathListSeparator))
	for _, name := range []string{"QML2_IMPORT_PATH", "QML_IMPORT_PATH"} {
		value := prefix
		if current := os.Getenv(name); current != "" {
			value += string(os.PathListSeparator) + current
		}
		_ = os.Setenv(name, value)
	}
}

// bootstrap brings the shell up: the settings store (sole writer of shell.json,
// served over the settings topic), the clipboard history and its selection
// watcher, the tray host, the keyring prompter, the theme workers, the in-shell
// wallpaper surface and the first wallpaper, then the persistent Quickshell
// components.
func (d *daemon) bootstrap() {
	d.startSettings()
	d.startKeypress()
	d.startClipboard()
	d.startTray()
	d.startWeather()
	d.startMusic()
	d.startCalendar()
	d.startPowerProfiles()
	d.startNetwork()
	d.startNightlight()
	d.startOsd()
	d.prompter = startKeyringPrompter()
	if d.prompter != nil {
		d.prompter.mon = d.activeMonitor
	}
	d.startSession()
	d.startPolkit()
	d.startUpdates()
	go d.paintWorker()
	d.startStage()
	go d.watchRyogami()
	go d.watchMatugenKnobs()
	go d.ledsWorker()
	d.startWM()
	go d.watchAudio()
	go d.watchPowerSounds()
	go d.watchAutoPowerSaver()
	go d.widgetGateWorker()
	go d.idlePark()
	d.startSleepWake()
	go d.startComponents()
}

// holdDaemonLock takes the exclusive single-daemon lock, retrying briefly: a
// stale daemon we just asked to quit may still be letting go of it. The returned
// file must stay open for as long as the daemon runs.
func holdDaemonLock(path string) (*os.File, error) {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return nil, err
	}
	deadline := time.Now().Add(3 * time.Second)
	for {
		if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err == nil {
			return f, nil
		}
		if time.Now().After(deadline) {
			f.Close()
			return nil, fmt.Errorf("another ryoku-shell daemon holds %s", path)
		}
		time.Sleep(100 * time.Millisecond)
	}
}

// startupStagger spaces the persistent components' cold starts at login so a
// handful of Quickshell processes do not contend for the GPU and CPU in the same
// frame (the boot-contention burst iNiR calls out).
const startupStagger = 250 * time.Millisecond

// reapStrays removes quickshell instances left by a previous daemon and returns
// how many were left. A daemon killed rather than asked to quit orphans its
// surfaces (KillMode=process), Quickshell allows a second instance of one config,
// and a leftover that ignores SIGTERM used to survive the session, so this waits
// for the process to go and SIGKILLs what does not. It runs before every start,
// since a stray can appear at any point.
func (d *daemon) reapStrays() int {
	strays := d.strayPids()
	if len(strays) == 0 {
		return 0
	}
	signalAll(strays, syscall.SIGTERM)
	strays = waitGone(strays, 3*time.Second)
	if len(strays) > 0 {
		// SIGTERM was ignored or the process is stuck in a syscall: take it out,
		// because a second live shell is worse than a hard kill.
		signalAll(strays, syscall.SIGKILL)
		strays = waitGone(strays, time.Second)
	}
	return len(strays)
}

// strayPids lists live quickshell processes that render a component this daemon
// owns but are not its own children.
func (d *daemon) strayPids() []int {
	entries, err := os.ReadDir("/proc")
	if err != nil {
		return nil
	}
	mine := d.ownedPids()
	self := os.Getpid()
	var out []int
	for _, e := range entries {
		pid, err := strconv.Atoi(e.Name())
		if err != nil || pid == self || mine[pid] {
			continue
		}
		raw, err := os.ReadFile(filepath.Join("/proc", e.Name(), "cmdline"))
		if err != nil {
			continue
		}
		argv := strings.Split(strings.TrimRight(string(raw), "\x00"), "\x00")
		for _, c := range components {
			if selectsComponent(argv, qsSelect(c.name)) {
				out = append(out, pid)
				break
			}
		}
	}
	return out
}

// ownedPids: the quickshell children this daemon is supervising right now, so a
// reap never shoots its own live surface.
func (d *daemon) ownedPids() map[int]bool {
	d.mu.Lock()
	defer d.mu.Unlock()
	out := make(map[int]bool, len(d.proc))
	for _, c := range d.proc {
		if c != nil && c.Process != nil {
			out[c.Process.Pid] = true
		}
	}
	return out
}

// selectsComponent reports whether argv is a quickshell instance rendering the
// component sel names. Both selector forms count: a packaged daemon looks for
// "-c shell" while a checkout's leftover reads "-p .../quickshell/shell", and
// matching only its own form is why the reap used to miss the strays that matter.
func selectsComponent(argv, sel []string) bool {
	if len(argv) < 3 || len(sel) < 2 {
		return false
	}
	switch filepath.Base(argv[0]) {
	case "qs", "quickshell":
	default:
		return false
	}
	// a client invocation (`qs -c shell ipc call ...`) talks to an instance, it
	// is not one; only a bare selector runs a config.
	name := filepath.Base(sel[1])
	for i := 1; i < len(argv)-1; i++ {
		switch argv[i] {
		case "-c", "--config":
			if argv[i+1] == name && i+2 == len(argv) {
				return true
			}
		case "-p", "--path":
			if filepath.Base(argv[i+1]) == name && i+2 == len(argv) {
				return true
			}
		}
	}
	return false
}

func signalAll(pids []int, sig syscall.Signal) {
	for _, pid := range pids {
		if p, err := os.FindProcess(pid); err == nil {
			_ = p.Signal(sig)
		}
	}
}

// waitGone returns the pids still alive after up to d, polling so a process that
// exits at once costs nothing.
func waitGone(pids []int, d time.Duration) []int {
	deadline := time.Now().Add(d)
	for {
		left := pids[:0:0]
		for _, pid := range pids {
			if pidAlive(pid) {
				left = append(left, pid)
			}
		}
		if len(left) == 0 || time.Now().After(deadline) {
			return left
		}
		pids = left
		time.Sleep(50 * time.Millisecond)
	}
}

// pidAlive: signal 0 probes without delivering. A zombie still answers, so the
// /proc state is checked too; a reaped child of ours is gone either way.
func pidAlive(pid int) bool {
	p, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	if err := p.Signal(syscall.Signal(0)); err != nil {
		return false
	}
	st, err := os.ReadFile(filepath.Join("/proc", strconv.Itoa(pid), "stat"))
	if err != nil {
		return false
	}
	if i := strings.LastIndexByte(string(st), ')'); i > 0 {
		fields := strings.Fields(string(st)[i+1:])
		if len(fields) > 0 && fields[0] == "Z" {
			return false
		}
	}
	return true
}

// startComponents brings the persistent components up one at a time, pill first,
// leaving startupStagger between each. ensure is idempotent, so a keybind that
// needs a component before its turn still starts it at once.
func (d *daemon) startComponents() {
	d.reapStrays()
	for _, c := range components {
		if !startsAtBoot(c) {
			continue
		}
		d.ensure(c.name)
		select {
		case <-d.quit:
			return
		case <-time.After(startupStagger):
		}
	}
}

// startsAtBoot reports whether a component renders at login. Persistent
// components always do; on-demand palettes wait for their first keybind. A
// disabled component never starts.
func startsAtBoot(c component) bool {
	if componentDisabled(c.name) {
		return false
	}
	return c.persistent
}

// ensure guarantees a supervisor goroutine exists for a component.
func (d *daemon) ensure(name string) {
	if componentDisabled(name) {
		return
	}
	d.mu.Lock()
	if d.sup[name] {
		d.mu.Unlock()
		return
	}
	d.sup[name] = true
	d.mu.Unlock()
	go d.supervise(name)
}

// jemallocConf tunes the allocator Quickshell links. jemalloc defaults narenas
// to 4*ncpu (64 on a 16-thread box) and only returns freed pages to the OS on
// later allocation activity, so an idle shell that stops allocating keeps every
// dirty page mapped as RSS. Two arenas is ample for an event-driven GUI, and a
// background thread purges on the decay schedule even while idle. Each supervised
// qs process pays this once, so five of them stop hoarding a dozen arenas of
// freed heap apiece.
const jemallocConf = "narenas:2,background_thread:true,dirty_decay_ms:5000,muzzy_decay_ms:5000"

// qsEnv is the environment for a supervised quickshell process: the daemon's own
// env (which carries the QML import path setupQmlImportPath exports) plus the
// jemalloc tuning, unless the user already pinned MALLOC_CONF.
func qsEnv() []string {
	env := os.Environ()
	if os.Getenv("MALLOC_CONF") == "" {
		env = append(env, "MALLOC_CONF="+jemallocConf)
	}
	return env
}

// surfaceLog is where a supervised surface's stderr goes, one file per component
// so a crash on start is still readable after the respawn.
func surfaceLog(name string) string {
	dir := filepath.Join(stateDir(), "ryoku", "surfaces")
	_ = os.MkdirAll(dir, 0o755)
	return filepath.Join(dir, name+".log")
}

func tailLine(path string) string {
	b, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	lines := strings.Split(strings.TrimRight(string(b), "\n"), "\n")
	for i := len(lines) - 1; i >= 0; i-- {
		if l := strings.TrimSpace(lines[i]); l != "" {
			return l
		}
	}
	return ""
}

// tailLines returns the last n lines of the file joined by newlines, or "" if it
// cannot be read. Used to attach a dying surface's output to a crash log without
// copying the whole file.
func tailLines(path string, n int) string {
	b, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	lines := strings.Split(strings.TrimRight(string(b), "\n"), "\n")
	if len(lines) > n {
		lines = lines[len(lines)-n:]
	}
	return strings.Join(lines, "\n")
}

// supervise runs `qs -c <name>` and restarts it whenever it exits, backing off if
// it dies immediately so a broken config does not spin the CPU.
func (d *daemon) supervise(name string) {
	backoff := time.Second
	for {
		select {
		case <-d.quit:
			return
		default:
		}
		// park while a gate keeps this component unloaded (the visualiser
		// audio-unload). a fresh open wakes us; the timeout is a safety re-check.
		for !d.gateAllows(name) {
			select {
			case <-d.quit:
				return
			case <-d.gateWaitCh(name):
			case <-time.After(5 * time.Second):
			}
		}
		// Never start a second copy of a surface: anything already rendering this
		// component is a leftover (our own child is tracked, so it is excluded),
		// and it goes before we draw over it.
		d.reapStrays()
		cmd := exec.Command("qs", qsSelect(name)...)
		cmd.Env = qsEnv()
		// Keep the surface's own output: a config that fails to load reports on
		// stdout and a loader failure on stderr, and without both a black screen
		// has no reason attached anywhere.
		logPath := surfaceLog(name)
		logFile, err := os.Create(logPath)
		if err == nil {
			cmd.Stdout = logFile
			cmd.Stderr = logFile
		}
		if err := cmd.Start(); err != nil {
			time.Sleep(backoff)
			backoff = capDur(backoff*2, 30*time.Second)
			continue
		}
		d.mu.Lock()
		d.proc[name] = cmd
		d.mu.Unlock()
		if parkable(name) {
			d.markHidden(name)
		}
		exited := make(chan struct{})
		if name == "shell" {
			// A shell that stays up past its crash window is the desktop coming
			// back after a boot; record it for the boot guard (ryoku boot-guard),
			// which reverts an update whose next boots never get here.
			go recordBootOK(exited)
		}

		start := time.Now()
		_ = cmd.Wait()
		close(exited)
		if name == "shell" && d.keypress != nil {
			d.keypress.configure(false, d.keypress.currentMode())
		}
		if logFile != nil {
			logFile.Close()
		}
		if name == "shell" {
			// On the shell going down, attach the reason so the next crash
			// report from a user carries it: the exit status, the signal that
			// killed it, and the tail of what the surface printed before it went.
			signal := "none"
			if ws, ok := cmd.ProcessState.Sys().(syscall.WaitStatus); ok && ws.Signaled() {
				signal = ws.Signal().String()
			}
			fmt.Fprintf(os.Stderr, "shell exited: %s (signal: %s)\n",
				cmd.ProcessState.String(), signal)
			if tail := tailLines(logPath, 20); tail != "" {
				fmt.Fprintf(os.Stderr, "shell stderr tail:\n%s\n", tail)
			}
		}

		d.mu.Lock()
		delete(d.proc, name)
		d.mu.Unlock()

		select {
		case <-d.quit:
			return
		default:
		}
		if time.Since(start) < 3*time.Second {
			if line := tailLine(logPath); line != "" {
				d.failMu.Lock()
				d.lastFail[name] = line
				d.failMu.Unlock()
				fmt.Fprintf(os.Stderr, "%s died at once: %s\n", name, line)
			}
			// Back off exponentially so a crash loop cannot spin the CPU.
			backoff = capDur(backoff*2, 30*time.Second)
			time.Sleep(backoff)
		} else {
			// Healthy run that exited (a reload or SIGTERM): respawn at once so
			// the surface does not blink out for a backoff interval.
			backoff = time.Second
		}
	}
}

// gateAllows reports whether the supervisor may (re)start name now. Any
// component without a gate defaults to true, so gating is strictly opt-in and a
// cleared gate never blocks a start.
func (d *daemon) gateAllows(name string) bool {
	d.gateMu.Lock()
	defer d.gateMu.Unlock()
	w, ok := d.gateWant[name]
	return !ok || w
}

// gateWaitCh returns name's wake channel, creating it on first use so a parked
// supervisor can block until its gate opens.
func (d *daemon) gateWaitCh(name string) chan struct{} {
	d.gateMu.Lock()
	defer d.gateMu.Unlock()
	ch := d.gateWake[name]
	if ch == nil {
		ch = make(chan struct{}, 1)
		d.gateWake[name] = ch
	}
	return ch
}

// setGate opens or closes a component's run gate. Opening wakes a parked
// supervisor; closing SIGTERMs the live process so its supervisor parks instead
// of respawning. Only real state changes act.
func (d *daemon) setGate(name string, want bool) {
	d.gateMu.Lock()
	prev, ok := d.gateWant[name]
	d.gateWant[name] = want
	ch := d.gateWake[name]
	if ch == nil {
		ch = make(chan struct{}, 1)
		d.gateWake[name] = ch
	}
	d.gateMu.Unlock()
	if ok && prev == want {
		return
	}
	if want {
		select {
		case ch <- struct{}{}:
		default:
		}
		return
	}
	d.mu.Lock()
	cmd := d.proc[name]
	d.mu.Unlock()
	if cmd != nil && cmd.Process != nil {
		_ = cmd.Process.Signal(syscall.SIGTERM)
	}
}

func (d *daemon) signalQuit() {
	d.mu.Lock()
	defer d.mu.Unlock()
	if !d.closed {
		d.closed = true
		close(d.quit)
		if d.ln != nil {
			_ = d.ln.Close()
		}
	}
}

// shutdown stops the supervised Quickshell processes and does not return until
// they are gone.
//
// Returning early is what left a live surface behind: systemd is KillMode=process
// (so a user's apps survive a shell reload) and would restart us straight away,
// and the replacement daemon then drew a second desktop on top of the one still
// running. SIGTERM first so Quickshell can bow out cleanly, SIGKILL if it will
// not.
func (d *daemon) shutdown() {
	if d.keypress != nil {
		d.keypress.configure(false, "all")
	}
	d.mu.Lock()
	pids := make([]int, 0, len(d.proc))
	for _, c := range d.proc {
		if c != nil && c.Process != nil {
			pids = append(pids, c.Process.Pid)
		}
	}
	d.mu.Unlock()
	if len(pids) == 0 {
		return
	}
	signalAll(pids, syscall.SIGTERM)
	if left := waitGone(pids, 3*time.Second); len(left) > 0 {
		signalAll(left, syscall.SIGKILL)
		waitGone(left, time.Second)
	}
}

func (d *daemon) handle(conn net.Conn) {
	defer conn.Close()
	_ = conn.SetDeadline(time.Now().Add(30 * time.Second))
	r := bufio.NewReader(conn)
	line, err := r.ReadString('\n')
	if err != nil && line == "" {
		return
	}
	cmd := strings.TrimSpace(line)
	// The keyring island returns the typed secret on a second line so it never
	// reaches a command line (and thus world-readable /proc/<pid>/cmdline).
	if strings.HasPrefix(cmd, "keyring-respond") {
		secret, _ := r.ReadString('\n')
		fmt.Fprintln(conn, d.keyringRespond(cmd, strings.TrimRight(secret, "\r\n")))
		return
	}
	// Typed subsystem state rides the same socket: a long-lived subscription
	// stream, or a request/response control call. Both outlive the one-shot
	// command deadline, which they clear for themselves.
	if strings.HasPrefix(cmd, "subscribe ") {
		d.serveSubscription(conn, cmd)
		return
	}
	if strings.HasPrefix(cmd, "call ") {
		d.serveCalls(conn, r, cmd)
		return
	}
	// A clipboard capture helper streams new selection bytes after its header
	// line, so they never reach a command line.
	if strings.HasPrefix(cmd, "clip-ingest ") {
		fmt.Fprintln(conn, d.clipIngest(cmd, r))
		return
	}
	if strings.HasPrefix(cmd, "clip-copy ") {
		fmt.Fprintln(conn, d.clipCopy(cmd))
		return
	}
	fmt.Fprintln(conn, d.dispatch(cmd))
}

var surfaceCommands = map[string]string{
	// One bare kebab verb per shell surface, spelled to match its CustomShortcut
	// id, so a compositor keybind reaches any surface as `ryoku-shell <id>` where
	// no global-shortcuts protocol exists (niri). Flag surfaces land on ShellState
	// through the surface bus's style-independent consumer; frame-menu surfaces
	// land on the per-monitor FrameMenuManager. Both are the same transition a
	// CustomShortcut press runs in-process.
	"bar-toggle":         "barToggle",
	"launcher":           "launcher",
	"overview":           "overview",
	"visualizer":         "visualizer",
	"visualizer-overlay": "visualizer-overlay",
	"visualizer-place":   "visualizer-place",
	"quicksettings":      "quick-settings",
	"wallpaper-menu":     "wallpaper",
	"clipboard":          "clipboard",
	"stash":              "stash",
	"screenshot":         "quick-settings#capture",
	"compress":           "stash#compress",
	"install":            "stash#install",
	// Preserved aliases so nothing scripted today breaks: the menu-prefixed
	// spellings and the file-picker desktop entries (install-app.desktop,
	// compress-video.desktop) land on the same surfaces they always have.
	"menu screenshot":   "screenshot",
	"menu stash":        "stash",
	"menu app-launcher": "launcher",
}

// route resolves an IPC-style command to the single shell's IpcHandler config,
// target, and function it triggers. The consolidated shell renders every surface
// in-process, so a matched command always resolves to config and target "shell"
// and the openSurface entry point on its surface bus. ok is false for commands
// that need more than one IPC call (wallpaper, reload, status, ...).
func route(cmd string) (config, target, fn string, ok bool) {
	if _, ok := surfaceCommands[cmd]; ok {
		return "shell", "shell", "openSurface", true
	}
	if _, ok := menuID(cmd); ok {
		return "shell", "shell", "openSurface", true
	}
	return "", "", "", false
}

func validBarStyleID(id string) bool {
	if id == "" {
		return false
	}
	for _, r := range id {
		if (r < 'a' || r > 'z') && (r < '0' || r > '9') && r != '-' {
			return false
		}
	}
	return true
}

// dispatch turns one command line into actions and returns "ok" or "err ...".
func (d *daemon) dispatch(line string) string {
	fields := strings.Fields(line)
	if len(fields) == 0 {
		return "err empty command"
	}
	cmd, args := fields[0], fields[1:]
	routeCmd := cmd
	switch cmd {
	case "menu":
		// menu close clears every open menu. App launcher and dedicated frame
		// surfaces keep their established menu commands.
		switch {
		case len(args) == 1 && args[0] == "close":
			return d.menuClose()
		case len(args) == 1 && (args[0] == "app-launcher" || args[0] == "screenshot" || args[0] == "stash"):
			routeCmd = line
		default:
			if _, ok := menuID(line); !ok {
				return "err menu: unknown or malformed id"
			}
			routeCmd = line
		}
	case "bar":
		// The data-layout verbs (list/catalog/move/show/hide/set/position/form/
		// defaults/settings) take a verb first; the reveal grammar takes an edge
		// first, so the first word disambiguates the two without collision.
		if len(args) >= 1 && barCLIVerbs[args[0]] {
			return d.barCLI(args)
		}
		// bar <edge|all> <toggle|reveal|hide> drives the bar reveal state.
		edge, action, ok := parseBarEdge(args)
		if !ok {
			return "err bar: expected <top|bottom|left|right|all> <toggle|reveal|hide>, or a widget verb (list|catalog|move|show|hide|set|position|form|defaults|settings)"
		}
		return d.barToggle(edge, action)
	case "dock":
		return d.dockCLI(args)
	}
	if config, target, fn, ok := route(routeCmd); ok {
		if componentDisabled(config) {
			// the user turned this component off; its keybind is a silent no-op
			// rather than a failed ipc call to a process that will never start.
			return "ok"
		}
		d.ensure(config)
		if config == "visualizer" || parkable(config) {
			// an explicit toggle must win over the idle-unload gate: reopen a
			// parked visualiser or palette so its supervisor respawns, then
			// ipcCall retries until the fresh instance answers.
			d.setGate(config, true)
		}
		mon := d.activeMonitor()
		if config == "shell" {
			if fn == "openSurface" {
				id, menu := menuID(routeCmd)
				if !menu {
					id = surfaceCommands[routeCmd]
				}
				return shellIpc(fn, mon, id)
			}
			return shellIpc(fn, mon)
		}
		return ipcCall(config, target, fn, mon)
	}

	switch cmd {
	case "voice":
		return d.voice()
	case "barstyle":
		if len(args) != 1 || !validBarStyleID(args[0]) {
			return "err barstyle: expected one product id"
		}
		if d.settings == nil {
			return "err barstyle: settings not ready"
		}
		value, _ := json.Marshal(args[0])
		if err := d.settings.patch("barStyle", value); err != nil {
			return "err barstyle: " + err.Error()
		}
		return "ok"
	case "lock":
		// lock status is the reference check (prints locked/unlocked, exit 0);
		// bare lock engages the session lock.
		if len(args) >= 1 && args[0] == "status" {
			if isLocked() {
				return "locked"
			}
			return "unlocked"
		}
		return lockSession()
	case "audio":
		if len(args) != 1 {
			return "err audio: expected up, down, or mute"
		}
		return d.audio(args[0])
	case "brightness":
		if len(args) != 1 {
			return "err brightness: expected up or down"
		}
		return d.brightness(args[0])
	case "hub":
		switch len(args) {
		case 1:
			return d.hub(args[0], "")
		case 2:
			return d.hub(args[0], args[1])
		}
		return "err hub: expected open [section] or close"
	case "stage":
		// One verb surface for Ryostage (docs/stage.md): set-effect records the
		// three-way effect per wallpaper; set-layer applies a layer's arrangement
		// flags (enabled/front/depth); add-layer / cut-layer / remove-layer manage
		// the manual layers; refresh forces a re-cut; cancel kills a running engine
		// child; clear drops the current wall's generated artifacts; status
		// snapshots the topic; models proxies the engine catalogue. QML normally
		// subscribes to the `stage` topic; these verbs carry intent the other way.
		if len(args) == 0 {
			return "err stage: expected a verb"
		}
		// Trailing path / json arguments may contain spaces, so recover them from
		// the raw line by fixed-field split instead of the whitespace args, which
		// would truncate at the first space.
		switch args[0] {
		case "set-effect":
			if len(args) < 2 {
				return "err stage set-effect: expected off|depth|parallax"
			}
			eff := stageEffect(args[1])
			if eff != stageEffectOff && eff != stageEffectDepth && eff != stageEffectParallax {
				return "err stage set-effect: expected off|depth|parallax"
			}
			d.stageSetEffect(eff)
			return "ok"
		case "refresh":
			d.stageForce.Store(true)
			d.scheduleStage()
			return "ok"
		case "cancel":
			stageCancel()
			return "ok"
		case "status":
			return d.stageStatusJSON()
		case "models":
			return stageModelsJSON()
		case "cut-layer":
			if len(args) < 2 {
				return "err stage cut-layer: expected a path"
			}
			path, err := d.stageCutLayer(restField(line, 3))
			if err != nil {
				return "err stage cut-layer: " + err.Error()
			}
			return "ok " + path
		case "set-layer":
			if len(args) < 3 {
				return "err stage set-layer: expected <index> <json>"
			}
			if err := d.stageSetLayer(args[1], restField(line, 4)); err != nil {
				return "err stage set-layer: " + err.Error()
			}
			return "ok"
		case "add-layer":
			if len(args) < 2 {
				return "err stage add-layer: expected a path"
			}
			path, err := d.stageAddLayer(restField(line, 3))
			if err != nil {
				return "err stage add-layer: " + err.Error()
			}
			return "ok " + path
		case "remove-layer":
			if len(args) < 2 {
				return "err stage remove-layer: expected an index"
			}
			if err := d.stageRemoveLayer(args[1]); err != nil {
				return "err stage remove-layer: " + err.Error()
			}
			return "ok"
		case "clear":
			d.stageClear()
			return "ok"
		}
		return "err stage: unknown verb " + args[0]
	case "theme":
		// Apply a colour scheme by writing theme.theme through the settings store
		// (the sole writer of shell.json), which validates the name, persists,
		// broadcasts, and schedules the retheme. `theme catalog` is served
		// client-side in main.go, so it never reaches here. Join the args so a
		// name with spaces ("Tokyo Night") survives the command split.
		if len(args) == 0 {
			return "err theme: expected a scheme name (or `catalog`)"
		}
		if args[0] == "remove" && len(args) >= 2 {
			// `theme remove <id>` uninstalls a downloaded scheme. Reset the
			// applied scheme to the Ryoku default first when it is the one being
			// removed, so the desktop never keeps a scheme whose files are gone;
			// the settings store drives the retheme off that write.
			id := strings.Join(args[1:], " ")
			if d.settings != nil && d.settings.themeName() == id {
				def, _ := json.Marshal("Default")
				if err := d.settings.patch("theme.theme", def); err != nil {
					return "err theme: " + err.Error()
				}
			}
			if err := removeUserTheme(id); err != nil {
				return "err theme: " + err.Error()
			}
			return "ok"
		}
		if d.settings == nil {
			return "err theme: settings not ready"
		}
		name, _ := json.Marshal(strings.Join(args, " "))
		if err := d.settings.patch("theme.theme", name); err != nil {
			return "err theme: " + err.Error()
		}
		return "ok"
	case "gtk":
		// A curated light/dark scheme is owned by the Hub: it writes colors.json
		// itself, so the paint worker idles and nothing here would land the
		// desktop's GTK settings for the new mode. The Hub calls this instead of
		// setting gsettings on its own, which keeps one writer for gtk-theme,
		// color-scheme and accent-color rather than two that can disagree.
		if len(args) != 2 || args[0] != "apply" {
			return "err gtk: expected `apply <light|dark>`"
		}
		if args[1] != "light" && args[1] != "dark" {
			return "err gtk: mode must be light or dark"
		}
		matugenReload(args[1])
		return "ok"
	case "reload":
		return d.reload()
	case "status":
		return d.status()
	case "ping":
		return "ok"
	case "signature":
		// Opaque per-session instance handle from the provider, string-compared
		// to tell a stale incumbent from a same-session double-start. Empty means
		// no live session.
		caps, _ := d.wmc.Caps()
		return caps.Instance
	case "quit":
		d.signalQuit()
		return "ok"
	case "plugin":
		// plugin <id> [toggle] -> toggle that plugin's frame popout. Reserved for
		// future per-host actions (show/hide); toggle is the default.
		if len(args) < 1 {
			return "err plugin: missing id"
		}
		d.ensure("shell")
		return shellIpc("openSurface", d.activeMonitor(), "plugin:"+args[0])
	case "plugins":
		// plugins reload -> the per-monitor PluginPopouts watch plugins.json and
		// re-discover on change, so a Settings save retunes live; this is a no-op
		// acknowledgement kept for an explicit force path.
		if len(args) >= 1 && args[0] == "reload" {
			return "ok"
		}
		return "err plugins: unknown action"
	case "state":
		// a parkable palette (launcher/overview) reporting its open state for the
		// idle-park worker: `state <name> <0|1>`. 1 shows (cancels the park grace),
		// 0 hides (starts it).
		if len(args) < 2 {
			return "err state: need <name> <0|1>"
		}
		d.setPaletteVisible(args[0], args[1] == "1")
		return "ok"
	case "sound":
		// an event cue from a config outside the daemon (ryoshot fires the
		// shutter on capture). The daemon owns the assets and playback; it only
		// accepts a known event name.
		if len(args) != 1 || !knownSound(args[0]) {
			return "err sound: expected a known event"
		}
		playSound(args[0])
		return "ok"

	default:
		return "err unknown command: " + cmd
	}
}

// voice handles the Super+` tap. With dictation running it toggles Voxtype's
// transcription and the pill's mic-wave together (first tap records and shows
// the wave; the next stops, transcribes, and hides it). With dictation off it
// just flashes an "off" note on the pill. Tap-to-toggle rides only the key-press
// edge: Hyprland won't deliver a release once the modifier lifts first, which
// would otherwise leave a hold-to-talk recording stuck on.
//
// Dictation can also end without a tap (Voxtype stops on silence or finishes
// transcribing), so the ON edge starts a state watcher that closes the surface
// when Voxtype reports idle (#244). The watcher is spawned before `record
// start` so it cannot miss the transition into recording.
func (d *daemon) voice() string {
	d.voiceMu.Lock()
	defer d.voiceMu.Unlock()
	if !dictationReady() {
		d.voiceOn = false
		d.ensure("shell")
		return shellIpc("openSurface", d.activeMonitor(), "voice-off")
	}
	d.voiceOn = !d.voiceOn
	if d.voiceOn {
		d.ensure("shell")
		stop := make(chan struct{})
		d.voiceStop = stop
		go d.watchVoice(stop)
		voxtypeRecord("start")
		return shellIpc("openSurface", d.activeMonitor(), "voice")
	}
	if d.voiceStop != nil {
		close(d.voiceStop)
		d.voiceStop = nil
	}
	voxtypeRecord("stop")
	return shellIpc("closeSurface", d.activeMonitor(), "voice")
}

// reload restarts every supervised component by terminating it; the supervisor
// brings it back. It waits for the result: answering "ok" while the surface dies
// on the same error is what made `ryoku reload` look like it does nothing on a
// black screen.
func (d *daemon) reload() string {
	_ = beginReloadCover()
	d.mu.Lock()
	was := make(map[string]int, len(d.proc))
	procs := make([]*exec.Cmd, 0, len(d.proc))
	for name, c := range d.proc {
		if c != nil && c.Process != nil {
			was[name] = c.Process.Pid
			procs = append(procs, c)
		}
	}
	d.mu.Unlock()
	for _, c := range procs {
		_ = c.Process.Signal(syscall.SIGTERM)
	}
	deadline := time.Now().Add(12 * time.Second)
	for _, c := range components {
		if !startsAtBoot(c) {
			continue
		}
		// A new pid, not just an entry: the map still holds the outgoing process
		// until its supervisor reaps it. And it has to stay up, because a surface
		// that cannot load starts and dies in a loop, which is the case this
		// reply exists for.
		fresh, freshAt := 0, time.Time{}
		for {
			d.mu.Lock()
			cmd := d.proc[c.name]
			d.mu.Unlock()
			pid := 0
			if cmd != nil && cmd.Process != nil {
				pid = cmd.Process.Pid
			}
			if pid != 0 && pid != was[c.name] {
				if pid != fresh {
					fresh, freshAt = pid, time.Now()
				}
			} else {
				fresh = 0
			}
			if fresh != 0 && time.Since(freshAt) > 3*time.Second {
				break
			}
			if time.Now().After(deadline) {
				d.failMu.Lock()
				why := d.lastFail[c.name]
				d.failMu.Unlock()
				if why == "" {
					why = "see " + surfaceLog(c.name)
				}
				return fmt.Sprintf("err reload: %s did not stay up: %s", c.name, why)
			}
			time.Sleep(150 * time.Millisecond)
		}
	}
	return "ok"
}

func (d *daemon) status() string {
	d.mu.Lock()
	defer d.mu.Unlock()
	var b strings.Builder
	for _, c := range components {
		state := "stopped"
		if _, ok := d.proc[c.name]; ok {
			state = "running"
		} else if d.sup[c.name] {
			state = "starting"
		}
		fmt.Fprintf(&b, "%s: %s\n", c.name, state)
	}
	return strings.TrimRight(b.String(), "\n")
}

func capDur(d, max time.Duration) time.Duration {
	if d > max {
		return max
	}
	return d
}
