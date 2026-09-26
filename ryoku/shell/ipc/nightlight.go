package main

import (
	"encoding/json"
	"errors"
	"log"
	"os"
	"os/exec"
	"path/filepath"
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

const nlDefaultTemp = 4000

// errNightlightUnavailable is what the intents return when the active compositor
// has no night-light capability, so the caller reports the absence by name.
var errNightlightUnavailable = errors.New("night light is not available on this desktop")

// errNightlightSchedule rejects an unknown schedule mode by name so a UI bug
// cannot silently disable the follow-the-sun window.
var errNightlightSchedule = errors.New("night light schedule mode must be off or sun")

type nightlightState struct {
	topic    *stateTopic
	wmc      *wm.Client
	stateDir string
	tempFile string
	// process caches the backend's comm name once the provider answers caps. It
	// stays empty until then, and backend() re-asks while it is empty, so a
	// provider that only came up after the daemon started is still picked up
	// rather than leaving the night light dead for the daemon's whole life.
	mu      sync.Mutex
	process string

	// schedMu guards the schedule: mode ("off" | "sun") and the margin in
	// minutes applied around each sunrise/sunset edge. schedLastDesire is the
	// last state the schedule asked for, so a manual toggle is honoured until
	// the next edge instead of being fought every tick.
	schedMu         sync.Mutex
	schedMode       string
	schedMarginMin  int
	schedFile       string
	schedLastDesire bool
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
		schedFile:      filepath.Join(dir, "ryoku-nightlight-schedule.json"),
		schedMode:      nlSchedOff,
		schedMarginMin: nlDefaultMarginMin,
	}
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
		}
		if err := json.Unmarshal(raw, &a); err != nil {
			return nil, err
		}
		// The schedule and the manual toggle are independent writes: a UI that
		// only flips on/off never sends schedule fields, and setting the
		// schedule never overrides the user's current on/off choice.
		if a.Schedule != nil {
			if err := n.setSchedule(*a.Schedule, a.MarginMin); err != nil {
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
	go n.watch()
	go n.runSchedule()
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

// watch publishes the current state once, then blocks in poll(2) on an inotify
// watch of the state directory: the marker create/remove and the temp write all
// land there, so every script-driven change pokes the watcher. Two wake reasons,
// two behaviours. On a state-dir event the first /proc scan can lie, because the
// script writes the enabled marker before the provider has spawned the backend,
// so the wake settles (see settle) before publishing. On the bounded poll
// timeout, a raw pkill from outside the script touched no watched file, so the
// expiry is a plain re-check: one scan. No timer, no subprocess.
func (n *nightlightState) watch() {
	on, temp := n.running(), n.savedTemp()
	n.publish(on)

	fd, err := unix.InotifyInit1(unix.IN_CLOEXEC | unix.IN_NONBLOCK)
	if err != nil {
		// No inotify: fall back to the settle re-check alone.
		n.loopNoWatch(on, temp)
		return
	}
	defer unix.Close(fd)
	if _, err := unix.InotifyAddWatch(fd, n.stateDir, unix.IN_CLOSE_WRITE|unix.IN_CREATE|unix.IN_DELETE|unix.IN_MOVED_TO); err != nil {
		log.Printf("ryoku-shell: nightlight watch failed: %v", err)
	}

	buf := make([]byte, 4096)
	for {
		pfds := []unix.PollFd{{Fd: int32(fd), Events: unix.POLLIN | unix.POLLERR}}
		// The bounded poll is the outside-kill re-check: a raw pkill touches no
		// watched file, so every expiry rescans /proc.
		nready, err := unix.Poll(pfds, 5000)
		if err != nil && err != unix.EINTR {
			return
		}
		// Drain the inotify queue so the next poll does not fire immediately.
		for {
			if _, err := unix.Read(fd, buf); err != nil {
				break
			}
		}
		var next bool
		if nready > 0 {
			// Woken by a state-dir write: do not trust the first scan, settle it.
			next = n.settle()
		} else {
			next = n.running()
		}
		nextTemp := n.savedTemp()
		if next != on || nextTemp != temp {
			on, temp = next, nextTemp
			n.publish(on)
		}
	}
}

// loopNoWatch is the degraded watcher when inotify is unavailable: a slow tick
// only, still fork-free.
func (n *nightlightState) loopNoWatch(on bool, temp int) {
	tick := time.NewTicker(5 * time.Second)
	defer tick.Stop()
	for range tick.C {
		next, nextTemp := n.running(), n.savedTemp()
		if next != on || nextTemp != temp {
			on, temp = next, nextTemp
			n.publish(on)
		}
	}
}

// settle resolves a state-dir wake, where the first /proc scan can lie: the
// script writes the enabled marker before `ryoku wm act nightlight.on` has
// spawned the backend, so a scan in that window sees no process and would
// publish off. Read the marker (the script's declared intent) once, then rescan
// /proc every 100 ms until running() agrees with it or 2 s elapse, and report
// the settled state. A bounded sleep loop on the watcher's own goroutine: no
// timer, no subprocess, no leak.
func (n *nightlightState) settle() bool {
	want := n.markerEnabled()
	deadline := time.Now().Add(2 * time.Second)
	for {
		got := n.running()
		if got == want || time.Now().After(deadline) {
			return got
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
	margin := n.schedMarginMin
	n.schedMu.Unlock()
	frame, err := json.Marshal(map[string]any{
		"on":          on,
		"temperature": n.savedTemp(),
		"schedule":    n.scheduleMode(),
		"marginMin":   margin,
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
// This is the fork-free pgrep: read each pid's comm (one small file per process,
// no exec) and compare the name. comm truncates at 15 characters; the backend
// names fit, so an exact compare is right. False when no backend is named.
func (n *nightlightState) running() bool {
	proc := n.backend()
	if proc == "" {
		return false
	}
	ents, err := os.ReadDir("/proc")
	if err != nil {
		return false
	}
	for _, e := range ents {
		if procCommIs(e.Name(), proc) {
			return true
		}
	}
	return false
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
