package main

// gpupending.go: whether a graphics change still waits on the user. A graphics
// mode is read by the compositor when it starts, so a change made in this
// session is pending until the next sign-in; the display wiring (the MUX) is
// pending until a restart, which ryoku-gpu-mux reports itself. The mode marker
// is keyed to the running compositor instance, so it survives closing Settings
// and clears itself once a new session starts.

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"syscall"

	wm "ryoku-wm"
)

// gpuModeApplies: the running window manager reads the render-card pin the mode
// writes; one that picks its own card has no pin file and ignores the mode
func gpuModeApplies() bool {
	return wm.GpuPinFile(wm.Detect().Name) != ""
}

type gpuModeMarker struct {
	From    string `json:"from"`
	To      string `json:"to"`
	AQ      string `json:"aq"`
	Session string `json:"session"`
}

func gpuModeMarkerPath() string {
	return filepath.Join(xdgHome("XDG_STATE_HOME", ".local/state"), "ryoku", "gpu-mode-pending.json")
}

// gpuSessionID names the running compositor instance: the boot plus the
// Wayland socket's inode and change time, which a new compositor recreates
func gpuSessionID() string {
	boot := strings.TrimSpace(readTrim("/proc/sys/kernel/random/boot_id"))
	display := os.Getenv("WAYLAND_DISPLAY")
	if display == "" {
		return boot
	}
	if !filepath.IsAbs(display) {
		display = filepath.Join(os.Getenv("XDG_RUNTIME_DIR"), display)
	}
	var st syscall.Stat_t
	if err := syscall.Stat(display, &st); err != nil {
		return boot
	}
	return fmt.Sprintf("%s:%d:%d.%d", boot, st.Ino, st.Ctim.Sec, st.Ctim.Nsec)
}

func readGpuModeMarker(path string) (gpuModeMarker, bool) {
	var m gpuModeMarker
	b, err := os.ReadFile(path)
	if err != nil {
		return m, false
	}
	if json.Unmarshal(b, &m) != nil || m.To == "" {
		return m, false
	}
	return m, true
}

// nextGpuModeMarker is the pure step: the marker after a change from prev to
// mode in session, given the marker on disk. nil means nothing is pending,
// because the session already runs the mode just chosen.
func nextGpuModeMarker(old gpuModeMarker, haveOld bool, prev, mode, aq, session string) *gpuModeMarker {
	from := prev
	if haveOld && old.Session == session {
		from = old.From
	}
	if mode == from {
		return nil
	}
	return &gpuModeMarker{From: from, To: mode, AQ: aq, Session: session}
}

func recordGpuModeChange(prev, mode string) error {
	path := gpuModeMarkerPath()
	old, have := readGpuModeMarker(path)
	next := nextGpuModeMarker(old, have, prev, mode, readAQValue(gpuLuaPath()), gpuSessionID())
	if next == nil {
		if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	b, err := json.Marshal(next)
	if err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, b, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

// gpuModePending: the marker still describes this session and the pin file
// still holds what was written; anything else is stale and goes
func gpuModePending(m gpuModeMarker, have bool, session, aq string) bool {
	return have && m.Session == session && m.AQ == aq
}

func gpuModeLabel(mode string) string {
	switch mode {
	case "performance":
		return "Performance"
	case "passthrough":
		return "Passthrough"
	}
	return "Hybrid"
}

func printGpuModePending() error {
	path := gpuModeMarkerPath()
	m, have := readGpuModeMarker(path)
	if !gpuModePending(m, have, gpuSessionID(), readAQValue(gpuLuaPath())) {
		if have {
			_ = os.Remove(path)
		}
		return nil
	}
	return printJSON(map[string][]string{"args": {gpuModeLabel(m.To)}})
}

func printGpuMuxPending() error {
	st, err := readMuxState()
	if err != nil || !st.Capable || !st.RebootPend {
		return nil
	}
	label := "Hybrid"
	if st.Mode == "discrete" {
		label = "Discrete"
	}
	return printJSON(map[string][]string{"args": {label}})
}
