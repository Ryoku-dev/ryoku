package main

import (
	"encoding/json"
	"errors"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"golang.org/x/sys/unix"

	wm "ryoku-wm"
)

// nightlight.go owns the night light so its state is a pushed topic instead of
// a shell poll. The on/off truth is the provider's backend process, named by its
// caps (gamma lives in the compositor, so a dead process is the only honest
// "off"); the temperature is the state file ryoku-cmd-nightlight writes. The
// watcher inotifies the state directory and rescans /proc on every change, so a
// toggle from the keybind, the script, the Hub, or a kill all reach QML without
// anything polling. Toggling from QML rides the same script: an intent is a user
// action, not a poll, and the script stays the single writer of the temp file.
// The marker is the user's declared intent, so a hotplug that quietly takes the
// warm gamma away from the running backend (dock/undock, lid, VT resume: the
// compositor forgets which client owns each output's colour, and the client is
// never told) is healed without asking the user: the daemon re-runs the on
// intent, whose kill-then-spawn claims every present output again
// (rearmOnOutputs).

const nlDefaultTemp = 4000

// nlRearmGrace is how long the connector set must hold still before the
// hotplug re-claim fires: a docking sequence renegotiates outputs repeatedly,
// and each move pushes the re-claim back so only the settled set is claimed.
// A var so the hotplug tests shorten the wait instead of sitting through it.
var nlRearmGrace = 4 * time.Second

const nightlightFallbackReconcile = 30 * time.Second

// errNightlightUnavailable is what the intents return when the active compositor
// has no night-light capability, so the caller reports the absence by name.
var errNightlightUnavailable = errors.New("night light is not available on this desktop")

// errNightlightSchedule rejects an unknown schedule mode by name so a UI bug
// cannot silently disable the schedule the user picked.
var errNightlightSchedule = errors.New("night light schedule mode must be off, sun or clock")

type nightlightState struct {
	topic    *stateTopic
	wmc      *wm.Client
	stateDir string
	tempFile string
	quit     <-chan struct{}
	workers  sync.WaitGroup
	// process caches the backend's comm name once the provider answers caps. It
	// stays empty until then, and backend() re-asks while it is empty, so a
	// provider that only came up after the daemon started is still picked up
	// rather than leaving the night light dead for the daemon's whole life.
	mu      sync.Mutex
	process string

	// schedMu guards the schedule: mode ("off" | "sun" | "clock"), the margin
	// in minutes applied around each sunrise/sunset edge, and the clock
	// window's start/stop minutes from midnight (used only by the clock mode).
	// schedLastDesire is the last state the schedule asked for, so a manual
	// toggle is honoured until the next edge instead of being fought every
	// tick.
	schedMu         sync.Mutex
	schedMode       string
	schedMarginMin  int
	schedStartMin   int
	schedStopMin    int
	schedFile       string
	schedLastDesire bool
	schedKick       chan struct{}

	// rearmMu guards the hotplug bookkeeping: outputsSeen says the wm watcher
	// has reported a connector set at least once (the first set only primes, it
	// is not a hotplug), outputsSig is that set (the enabled output names, so
	// docking moves it while focus and workspace frames leave it alone), and
	// healTimer is the single pending re-claim: a docking sequence keeps moving
	// the set and keeps pushing the re-claim back, so only a settled set is
	// claimed.
	rearmMu     sync.Mutex
	outputsSeen bool
	outputsSig  string
	healTimer   *time.Timer
}

// nightlightPaths derives the script's state files from XDG_STATE_HOME. The
// script hardcodes the same defaults, so the two stay in lockstep by naming.
func nightlightPaths() (dir, temp string) {
	base := os.Getenv("XDG_STATE_HOME")
	if base == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", ""
		}
		base = filepath.Join(home, ".local", "state")
	}
	return base, filepath.Join(base, "ryoku-nightlight")
}

// startNightlight registers the `nightlight` topic and its intents, then starts
// the watcher. The intents are registered unconditionally: backend() resolves
// the process name lazily from caps, so a provider that only answers after the
// daemon started still gets a working toggle. When no backend is ever named the
// intents return errNightlightUnavailable and the frame stays off.
func (d *daemon) startNightlight() {
	dir, temp := nightlightPaths()
	n := &nightlightState{
		topic:          d.registerTopic("nightlight"),
		wmc:            d.wmc,
		stateDir:       dir,
		tempFile:       temp,
		quit:           d.quit,
		schedFile:      filepath.Join(dir, "ryoku-nightlight-schedule.json"),
		schedMode:      nlSchedOff,
		schedMarginMin: nlDefaultMarginMin,
		schedStartMin:  nlDefaultClockStart,
		schedStopMin:   nlDefaultClockStop,
		schedKick:      make(chan struct{}, 1),
	}
	d.nightlight = n
	n.loadSchedule()

	d.registerCall("nightlight.toggle", func(json.RawMessage) (any, error) {
		return nil, n.intent("toggle")
	})
	d.registerCall("nightlight.set", func(raw json.RawMessage) (any, error) {
		var a struct {
			On          *bool   `json:"on"`
			Temperature int     `json:"temperature"`
			Schedule    *string `json:"schedule"`
			MarginMin   *int    `json:"marginMin"`
			StartAt     *int    `json:"startAt"`
			StopAt      *int    `json:"stopAt"`
		}
		if err := json.Unmarshal(raw, &a); err != nil {
			return nil, err
		}
		// The schedule and the manual toggle are independent writes: a UI that
		// only flips on/off never sends schedule fields, and setting the
		// schedule never overrides the user's current on/off choice. The
		// schedule field may ride along with a time edit (the Hub sends the
		// live mode with every clock change) or be absent (a plain margin
		// update), so the bounds are applied through the same call.
		if a.Schedule != nil || a.StartAt != nil || a.StopAt != nil {
			mode := n.scheduleMode()
			if a.Schedule != nil {
				mode = *a.Schedule
			}
			if err := n.setSchedule(mode, a.MarginMin, a.StartAt, a.StopAt); err != nil {
				return nil, err
			}
			n.tickSchedule()
			n.publish(n.running())
			if a.On == nil {
				return map[string]any{"ok": true, "schedule": n.scheduleMode()}, nil
			}
		}
		if a.On != nil && *a.On {
			if a.Temperature > 0 {
				return nil, n.intent("on", strconv.Itoa(a.Temperature))
			}
			return nil, n.intent("on")
		}
		if a.On != nil {
			return nil, n.intent("off")
		}
		return map[string]any{"ok": true, "schedule": n.scheduleMode()}, nil
	})

	if dir == "" {
		n.publish(n.running())
		return
	}
	n.workers.Add(2)
	go func() { defer n.workers.Done(); n.watch() }()
	go func() { defer n.workers.Done(); n.runSchedule() }()
}

// backend resolves the provider's night-light process name from caps and caches
// it once caps answers. While it is still empty it re-asks: a Client that probed
// before the provider was up now caches no failure, so a later probe carries the
// real name. Empty means this desktop has no night light.
func (n *nightlightState) backend() string {
	n.mu.Lock()
	defer n.mu.Unlock()
	if n.process != "" {
		return n.process
	}
	if n.wmc == nil {
		return ""
	}
	if caps, err := n.wmc.Caps(); err == nil {
		n.process = caps.NightLightProcess
	}
	return n.process
}

// intent runs the script for a user action and then publishes the fresh state
// itself, so the caller's next frame already carries the completed toggle
// instead of waiting for the watcher to wake and settle. run waits for the
// script, and the script waits for the provider to spawn the backend, so
// running() is truthful by the time run returns. A script error still moved the
// world (the marker may be gone), so the frame is republished either way. With
// no backend named it refuses by name and leaves the frame off.
func (n *nightlightState) intent(args ...string) error {
	if n.backend() == "" {
		n.publish(false)
		return errNightlightUnavailable
	}
	err := n.run(args...)
	n.publish(n.running())
	return err
}

// run execs ryoku-cmd-nightlight with the verb and waits, so the caller's reply
// reflects a completed toggle. The script republishes nothing itself; the
// marker and temp writes land in the state dir, which wakes the watcher to push
// the new frame.
func (n *nightlightState) run(args ...string) error {
	bin, err := exec.LookPath("ryoku-cmd-nightlight")
	if err != nil {
		return err
	}
	cmd := exec.Command(bin, args...)
	cmd.SysProcAttr = &syscall.SysProcAttr{Pdeathsig: syscall.SIGKILL}
	out, err := cmd.CombinedOutput()
	if err != nil {
		log.Printf("ryoku-shell: nightlight %v: %v: %s", args, err, strings.TrimSpace(string(out)))
	}
	return err
}

// rearmOnOutputs is the hotplug hook the daemon's wm frame pump calls for
// every output frame. The marker is the user's declared intent: while it is
// set, a connector set that MOVES (dock, undock, lid, a VT resume that
// re-enumerates) re-claims the gamma, because a compositor that reprograms
// its outputs during hotplug drops the running backend's claim on them
// without telling it, and the client cannot know to re-assert. A settled
// set that never moved is untouched, so normal frames (focus, workspace,
// activity) never churn the backend. The re-claim is debounced on the set
// holding still for nlRearmGrace: docking renegotiates repeatedly, and only
// the settled sequence pays one respawn. The script restarts the backend
// (kill-then-spawn), which binds a control for every output present now;
// outputs wlsunset sees without an event also re-bind on their own.
func (n *nightlightState) rearmOnOutputs(outputs []wm.Output) {
	sig := outputSetSig(outputs)
	n.rearmMu.Lock()
	changed := n.outputsSeen && sig != n.outputsSig
	n.outputsSeen, n.outputsSig = true, sig
	n.rearmMu.Unlock()
	if !changed || !n.markerEnabled() {
		return
	}
	log.Printf("ryoku-shell: night light: connector set changed, re-claiming the warm gamma")
	n.rearmMu.Lock()
	if n.healTimer != nil {
		n.healTimer.Stop()
	}
	n.healTimer = time.AfterFunc(nlRearmGrace, n.healNow)
	n.rearmMu.Unlock()
}

// healNow is the debounced re-claim: re-run the on intent if the user's
// marker is still set. The intent is idempotent by design: the script writes
// the same files and the provider replaces the backend, so a heal is
// indistinguishable from the user pressing the same toggle on.
func (n *nightlightState) healNow() {
	if !n.markerEnabled() {
		return
	}
	if err := n.intent("on"); err != nil && !errors.Is(err, errNightlightUnavailable) {
		log.Printf("ryoku-shell: night light: hotplug re-claim failed: %v", err)
	}
}

// outputSetSig names the enabled connector set: one token per enabled output
// name. Names only: mode, scale, position and focus churn constantly and prove
// nothing about which client owns the gamma.
func outputSetSig(outputs []wm.Output) string {
	names := make([]string, 0, len(outputs))
	for _, o := range outputs {
		if !o.Disabled {
			names = append(names, o.Name)
		}
	}
	sort.Strings(names)
	return strings.Join(names, ",")
}

// watch publishes the current state once, then waits on the state directory
// while the backend is off and on its pidfd plus the directory while it runs.
// A backend started by hand cannot be discovered while off because it changes
// neither source; Ryoku's script remains the supported owner of that process.
func (n *nightlightState) watch() {
	stopFD, stop, err := quitEventFD(n.quit)
	if err != nil {
		return
	}
	defer func() { stop(); unix.Close(stopFD) }()
	on, pid := n.runningPID()
	temp := n.savedTemp()
	n.publish(on)

	fd, err := unix.InotifyInit1(unix.IN_CLOEXEC | unix.IN_NONBLOCK)
	if err != nil {
		n.loopNoWatch(on, temp)
		return
	}
	defer unix.Close(fd)
	if _, err := unix.InotifyAddWatch(fd, n.stateDir,
		unix.IN_CLOSE_WRITE|unix.IN_CREATE|unix.IN_DELETE|unix.IN_MOVED_TO); err != nil {
		log.Printf("ryoku-shell: nightlight watch failed, using slow reconciliation: %v", err)
		n.loopNoWatch(on, temp)
		return
	}

	buf := make([]byte, 4096)
	tempName := filepath.Base(n.tempFile)
	for {
		pidfd := -1
		if pid > 0 {
			pidfd, _ = unix.PidfdOpen(pid, 0)
		}
		pfds := []unix.PollFd{{Fd: int32(fd), Events: unix.POLLIN | unix.POLLERR}}
		timeout := -1
		if pidfd >= 0 {
			pfds = append(pfds, unix.PollFd{Fd: int32(pidfd), Events: unix.POLLIN | unix.POLLERR | unix.POLLHUP})
		} else if on || n.markerEnabled() {
			timeout = int(nightlightFallbackReconcile / time.Millisecond)
		}
		quitIndex := len(pfds)
		pfds = append(pfds, unix.PollFd{Fd: int32(stopFD), Events: unix.POLLIN})
		nready, err := unix.Poll(pfds, timeout)
		if pidfd >= 0 {
			unix.Close(pidfd)
		}
		if err != nil {
			if err == unix.EINTR {
				continue
			}
			return
		}
		if pfds[quitIndex].Revents != 0 {
			return
		}
		stateChanged := false
		if pfds[0].Revents != 0 {
			for {
				count, err := unix.Read(fd, buf)
				if err != nil {
					break
				}
				for _, event := range decodeFileEvents(buf[:count]) {
					if event.mask&unix.IN_Q_OVERFLOW != 0 ||
						event.name == tempName || event.name == "ryoku-nightlight-enabled" {
						stateChanged = true
					}
				}
			}
		}
		pidExited := pidfd >= 0 && len(pfds) > 1 && pfds[1].Revents != 0
		if !stateChanged && !pidExited && nready > 0 {
			continue
		}

		var next bool
		if stateChanged || pidExited || (!on && n.markerEnabled()) {
			next, pid = n.settle()
		} else {
			next, pid = n.runningPID()
		}
		nextTemp := n.savedTemp()
		if next != on || nextTemp != temp {
			on, temp = next, nextTemp
			n.publish(on)
		}
	}
}

// loopNoWatch is the degraded watcher when inotify is unavailable.
func (n *nightlightState) loopNoWatch(on bool, temp int) {
	tick := time.NewTicker(nightlightFallbackReconcile)
	defer tick.Stop()
	for {
		select {
		case <-n.quit:
			return
		case <-tick.C:
		}
		next, nextTemp := n.running(), n.savedTemp()
		if next != on || nextTemp != temp {
			on, temp = next, nextTemp
			n.publish(on)
		}
	}
}

// settle resolves a state-dir wake, where the first /proc scan can lie: the
// script writes the enabled marker before the backend exists. It waits for the
// process truth to agree with that intent, while retaining the pid for the next
// event-driven wait.
func (n *nightlightState) settle() (bool, int) {
	want := n.markerEnabled()
	deadline := time.Now().Add(2 * time.Second)
	for {
		select {
		case <-n.quit:
			return false, 0
		default:
		}
		got, pid := n.runningPID()
		if got == want || time.Now().After(deadline) {
			return got, pid
		}
		time.Sleep(100 * time.Millisecond)
	}
}

// markerEnabled reports whether the script's enabled marker is present: the
// user's declared intent, which settle waits for /proc to catch up to. It sits
// beside the temp file, so it shares the watched state directory.
func (n *nightlightState) markerEnabled() bool {
	if n.stateDir == "" {
		return false
	}
	_, err := os.Stat(filepath.Join(n.stateDir, "ryoku-nightlight-enabled"))
	return err == nil
}

// publish writes the nightlight frame. The topic drops a byte-identical frame,
// so an unchanged state never wakes a binding.
func (n *nightlightState) publish(on bool) {
	if n.topic == nil {
		return
	}
	n.schedMu.Lock()
	margin, start, stop := n.schedMarginMin, n.schedStartMin, n.schedStopMin
	n.schedMu.Unlock()
	frame, err := json.Marshal(map[string]any{
		"on":          on,
		"temperature": n.savedTemp(),
		"schedule":    n.scheduleMode(),
		"marginMin":   margin,
		"startAt":     start,
		"stopAt":      stop,
	})
	if err != nil {
		return
	}
	n.topic.publish(frame)
}

// savedTemp reads the persisted temperature, defaulting to the script's default
// when no file exists or it is unparseable.
func (n *nightlightState) savedTemp() int {
	b, err := os.ReadFile(n.tempFile)
	if err != nil {
		return nlDefaultTemp
	}
	v, err := strconv.Atoi(strings.TrimSpace(string(b)))
	if err != nil || v <= 0 {
		return nlDefaultTemp
	}
	return v
}

// running scans /proc for the provider's night-light backend, named by caps.
func (n *nightlightState) running() bool {
	running, _ := n.runningPID()
	return running
}

func (n *nightlightState) runningPID() (bool, int) {
	proc := n.backend()
	if proc == "" {
		return false, 0
	}
	ents, err := os.ReadDir("/proc")
	if err != nil {
		return false, 0
	}
	for _, e := range ents {
		if procCommIs(e.Name(), proc) {
			pid, _ := strconv.Atoi(e.Name())
			return true, pid
		}
	}
	return false, 0
}

// procCommIs reports whether the pid's comm equals name. It skips anything that
// is not a numeric process directory, so the caller can hand it every /proc
// entry. comm truncates at 15 characters; hyprsunset fits, so an exact compare
// is right.
func procCommIs(pid, name string) bool {
	if _, err := strconv.Atoi(pid); err != nil {
		return false
	}
	b, err := os.ReadFile("/proc/" + pid + "/comm")
	if err != nil {
		return false
	}
	return strings.TrimSpace(string(b)) == name
}
