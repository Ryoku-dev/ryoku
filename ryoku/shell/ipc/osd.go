package main

import (
	"encoding/json"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"golang.org/x/sys/unix"
)

// osd.go feeds the brightness OSD. The volume and mic OSDs read PipeWire
// directly in QML, where Ryoku already owns the audio graph, so their change
// events and value are observable without the daemon. Brightness has no such
// QML-observable source: the media keys and the `brightness` verb both drive
// ryoku-cmd-brightness straight to the backlight, so the daemon watches the
// primary backlight and pushes a fraction plus a monotonically rising sequence
// to the `osd` topic. QML shows the brightness OSD when the sequence advances
// and binds `value` to the slider; the bucket->icon mapping lives once in QML
// beside the volume and mic buckets. Contract 12 sec 3, sec 9.

type osdState struct {
	topic *stateTopic
	seq   int
}

// backlightLevelPath is where the last user-set panel level lives across
// reboots. The kernel drives every backlight to its hardware default at boot
// and nothing else persists the level, so a plain reboot erased it (#199).
// The watcher below is the one place every writer (media keys, the slider,
// the brightness verb) converges, so it owns both halves: save on change,
// restore before the session's first read.
func backlightLevelPath() string {
	return filepath.Join(stateDir(), "ryoku", "backlight")
}

// Idle dimming must not overwrite or restore the user's persistent level.
func idleBrightnessActive() bool {
	dir := os.Getenv("RYOKU_IDLE_BRIGHTNESS_STATE_DIR")
	if dir == "" {
		runtimeDir := os.Getenv("XDG_RUNTIME_DIR")
		if runtimeDir == "" {
			runtimeDir = os.TempDir()
		}
		dir = filepath.Join(runtimeDir, "ryoku-idle-brightness")
	}
	_, err := os.Stat(filepath.Join(dir, "state"))
	return err == nil
}

// saveBacklight records the raw sysfs level. Best-effort: a read-only state
// dir costs persistence, not the session.
func saveBacklight(level int) {
	dir := filepath.Join(stateDir(), "ryoku")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return
	}
	_ = os.WriteFile(backlightLevelPath(), []byte(strconv.Itoa(level)), 0o644)
}

func saveObservedBacklight(level int) {
	if idleBrightnessActive() {
		return
	}
	saveBacklight(level)
}

// restoreBacklight pushes the saved level onto the device before the watcher
// opens it, so the first published value is the restored one and the OSD and
// sliders agree with the screen. A level outside [1,max] (the panel changed
// shape, the file is stale) is dropped rather than trusted; a device that
// rejects the write (no seat ACL on an unusual box) is left at the kernel's
// value instead of fought.
func restoreBacklight(dev string, max int) {
	if max <= 0 {
		return
	}
	if idleBrightnessActive() {
		return
	}
	b, err := os.ReadFile(backlightLevelPath())
	if err != nil {
		return
	}
	want, err := strconv.Atoi(strings.TrimSpace(string(b)))
	if err != nil || want < 1 || want > max {
		return
	}
	if cur, ok := readSysInt(filepath.Join(dev, "brightness")); ok && cur == want {
		return
	}
	_ = os.WriteFile(filepath.Join(dev, "brightness"), []byte(strconv.Itoa(want)), 0o644)
}

// backlightDevice picks the primary backlight through ryoku-hw-backlight, the
// one selector that names the device driving the connected panel: a first-entry
// pick can land on a phantom nvidia_0 beside the real EC or amdgpu device, and
// then the OSD watches a device the key handler never writes (#176). Falls
// back to the first entry when the helper is missing or names nothing.
func backlightDevice() string {
	const base = "/sys/class/backlight"
	if out, err := exec.Command("ryoku-hw-backlight").Output(); err == nil {
		if name := strings.TrimSpace(string(out)); name != "" {
			if _, err := os.Stat(filepath.Join(base, name)); err == nil {
				return filepath.Join(base, name)
			}
		}
	}
	ents, err := os.ReadDir(base)
	if err != nil || len(ents) == 0 {
		return ""
	}
	return filepath.Join(base, ents[0].Name())
}

// readSysInt reads a single integer sysfs attribute.
func readSysInt(path string) (int, bool) {
	b, err := os.ReadFile(path)
	if err != nil {
		return 0, false
	}
	n, err := strconv.Atoi(strings.TrimSpace(string(b)))
	if err != nil {
		return 0, false
	}
	return n, true
}

// startOsd registers the `osd` topic and, when a backlight exists, starts the
// watcher goroutine that republishes on every brightness change.
func (d *daemon) startOsd() {
	o := &osdState{topic: d.registerTopic("osd")}
	o.publish(0)
	if dev := backlightDevice(); dev != "" {
		go o.watchBacklight(dev)
	}
}

// publish writes the current osd frame. The topic drops a byte-identical frame,
// so republishing an unchanged value never wakes a QML binding.
func (o *osdState) publish(value float64) {
	frame, _ := json.Marshal(map[string]any{
		"brightness": map[string]any{"seq": o.seq, "value": value},
	})
	o.topic.publish(frame)
}

// Backoff for reopening a backlight node that went away: a dGPU driver
// (nvidia) unregisters and re-registers its backlight across suspend, so the
// node comes back within a second or two of resume.
const (
	backlightRetryMin = time.Second
	backlightRetryMax = 30 * time.Second
)

// watchBacklight keeps a watcher on the primary backlight for the life of the
// daemon. The saved level is restored once, at startup. When the watched node
// disappears (the driver re-registered the device on resume) the watcher
// re-resolves the device and reopens it, backing off while it is missing,
// rather than polling a dead fd that reports POLLERR forever.
func (o *osdState) watchBacklight(dev string) {
	maxb, ok := readSysInt(filepath.Join(dev, "max_brightness"))
	if !ok || maxb <= 0 {
		return
	}
	restoreBacklight(dev, maxb)
	delay := backlightRetryMin
	for {
		if o.watchBacklightNode(dev, maxb) {
			delay = backlightRetryMin
		}
		time.Sleep(delay)
		delay = min(delay*2, backlightRetryMax)
		if dev = backlightDevice(); dev == "" {
			continue
		}
		if maxb, ok = readSysInt(filepath.Join(dev, "max_brightness")); !ok || maxb <= 0 {
			dev = ""
		}
	}
}

// watchBacklightNode blocks in poll(2) on the backlight's actual_brightness
// sysfs attribute, which the kernel wakes with POLLPRI on every brightness
// change (media keys, the brightness verb, or anything else). Each wake
// recomputes the fraction, bumps the sequence, and republishes. The first read
// is the current value and does NOT bump the sequence, so neither a fresh
// subscriber nor a reopen after resume flashes the OSD. Reading the attribute after each poll
// re-arms the notify. It returns once the node is gone (open or read fails),
// reporting whether it ever read the node.
func (o *osdState) watchBacklightNode(dev string, maxb int) bool {
	if dev == "" {
		return false
	}
	fd, err := unix.Open(filepath.Join(dev, "actual_brightness"), unix.O_RDONLY, 0)
	if err != nil {
		return false
	}
	defer unix.Close(fd)

	buf := make([]byte, 32)
	// Polling wakes on actual_brightness, but the value published (and saved)
	// is the linear `brightness` attribute: on a driver with a custom
	// brightness curve (amdgpu) actual_brightness is nonlinear, so publishing
	// it made the OSD read 7-88% for a 1-100% request (#176).
	publish := func() {
		if frac, raw, ok := readBrightnessFraction(dev, maxb); ok {
			o.publish(frac)
			saveObservedBacklight(raw)
		}
	}
	if _, err := readActual(fd, buf); err != nil {
		return false
	}
	// A reopen publishes too: the level may have moved while the node was
	// gone, and an unchanged frame is dropped by the topic anyway.
	publish()
	for {
		fds := []unix.PollFd{{Fd: int32(fd), Events: unix.POLLPRI | unix.POLLERR}}
		n, err := unix.Poll(fds, -1)
		if err != nil {
			if err == unix.EINTR {
				continue
			}
			return true
		}
		if n == 0 {
			continue
		}
		// Reading actual_brightness re-arms the POLLPRI notify. A failed read
		// leaves the condition pending, so poll would return at once forever:
		// the node is dead (ENODEV after the driver re-registered it), and
		// the caller reopens it.
		if _, err := readActual(fd, buf); err != nil {
			if err == errBacklightValue {
				continue
			}
			return true
		}
		o.seq++
		publish()
	}
}

// readBrightnessFraction returns the panel's linear level as a 0..1 fraction
// plus its raw attribute value. It reads `brightness`, not `actual_brightness`:
// on a driver with a custom brightness curve the two differ, and only the
// requested value is a linear percentage the OSD and restore path can use.
func readBrightnessFraction(dev string, maxb int) (float64, int, bool) {
	if maxb <= 0 {
		return 0, 0, false
	}
	raw, ok := readSysInt(filepath.Join(dev, "brightness"))
	if !ok {
		return 0, 0, false
	}
	return float64(raw) / float64(maxb), raw, true
}

// errBacklightValue marks a read that succeeded but held no integer: the
// notify is re-armed, so the watcher keeps polling the same fd.
var errBacklightValue = errors.New("backlight: unparsable level")

// readActual reads the backlight level via pread at offset 0, which both fetches
// the value and clears the pending poll condition on the sysfs file. A failed or
// empty read means the node is gone.
func readActual(fd int, buf []byte) (int, error) {
	n, err := unix.Pread(fd, buf, 0)
	if err != nil {
		return 0, err
	}
	if n <= 0 {
		return 0, io.ErrUnexpectedEOF
	}
	v, err := strconv.Atoi(strings.TrimSpace(string(buf[:n])))
	if err != nil {
		return 0, errBacklightValue
	}
	return v, nil
}
