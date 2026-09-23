package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"strconv"
	"strings"
	"time"

	wm "ryoku-wm"
)

// Two properties must survive any rewrite here:
//
// A focus change emits straight from the event line with no compositor query,
// because the shell daemon resolves a surface target on every keypress from it.
//
// Other changes are debounced into a refresh of only the affected sections,
// so a burst costs one query pass instead of a storm.

// Half a 60 Hz frame: folds a burst, never a visible stale beat.
const resyncDebounce = 8 * time.Millisecond

const reconnectBackoff = 500 * time.Millisecond

// runWatch streams every frame kind, or only the ones named in args. A narrowed
// watch skips the reads it does not need, which keeps a burst cheap for a
// consumer that only wants one kind.
func runWatch(args []string) error {
	want := map[wm.FrameKind]bool{}
	for _, a := range args {
		want[wm.FrameKind(a)] = true
	}
	wants := func(k wm.FrameKind) bool { return len(want) == 0 || want[k] }
	return streamWatch(wants)
}

func streamWatch(wants func(wm.FrameKind) bool) error {
	enc := json.NewEncoder(stdout)
	emit := func(f wm.Frame) {
		// Flush per frame: the consumer is a live shell, and an unflushed focus
		// frame is a bar that never updates.
		if enc.Encode(f) != nil || stdout.Flush() != nil {
			os.Exit(0)
		}
	}

	for {
		ensureLiveSignature()
		path := eventSocketPath()
		if path == "" {
			time.Sleep(reconnectBackoff)
			continue
		}
		conn, err := net.Dial("unix", path)
		if err != nil {
			time.Sleep(reconnectBackoff)
			continue
		}
		// focusedmon only fires on change, so a single-monitor session would
		// never populate the cache from events alone.
		consume(conn, emit, wants)
		_ = conn.Close()
		time.Sleep(reconnectBackoff)
	}
}

func consume(conn net.Conn, emit func(wm.Frame), wants func(wm.FrameKind) bool) {
	state := newWatchState(emit, wants)
	state.refresh(refreshAll)
	emit(wm.Frame{Kind: wm.FrameReady})
	pending := time.NewTimer(time.Hour)
	if !pending.Stop() {
		<-pending.C
	}
	defer pending.Stop()

	done := make(chan struct{})
	defer close(done)
	lines := make(chan string, 256)
	go func() {
		defer close(lines)
		scan := bufio.NewScanner(conn)
		for scan.Scan() {
			select {
			case lines <- scan.Text():
			case <-done:
				return
			}
		}
	}()

	armed := false
	var dirty refreshMask
	for {
		select {
		case line, ok := <-lines:
			if !ok {
				return
			}
			if mon, isFocus := parseFocusedMon(line); isFocus && wants(wm.FrameFocus) {
				emit(wm.Frame{Kind: wm.FrameFocus, FocusedOutput: mon})
			}
			if name, removed := parseMonitorRemoved(line); removed && wants(wm.FrameFocus) {
				// A removed output must not linger until the debounce: the next
				// keybind would target a dead monitor.
				emit(wm.Frame{Kind: wm.FrameFocus, FocusedOutput: focusedFallback(name)})
			}
			dirty |= state.event(line)
			if dirty == 0 {
				continue
			}
			if !armed {
				armed = true
				pending.Reset(resyncDebounce)
			}
		case <-pending.C:
			armed = false
			state.refresh(dirty)
			dirty = 0
		}
	}
}

type hyprMonitor struct {
	ID                    int      `json:"id"`
	Name                  string   `json:"name"`
	Width                 int      `json:"width"`
	Height                int      `json:"height"`
	Scale                 float64  `json:"scale"`
	Focused               bool     `json:"focused"`
	Make                  string   `json:"make"`
	Model                 string   `json:"model"`
	PhysicalWidth         int      `json:"physicalWidth"`
	Disabled              bool     `json:"disabled"`
	X                     int      `json:"x"`
	Y                     int      `json:"y"`
	RefreshRate           float64  `json:"refreshRate"`
	Transform             int      `json:"transform"`
	VRR                   bool     `json:"vrr"`
	AvailableModes        []string `json:"availableModes"`
	MirrorOf              string   `json:"mirrorOf"`
	ColorManagementPreset string   `json:"colorManagementPreset"`
	SdrBrightness         float64  `json:"sdrBrightness"`
	ActiveWorkspace       struct {
		ID   int    `json:"id"`
		Name string `json:"name"`
	} `json:"activeWorkspace"`
}

// Read once per resync and threaded through, so workspaces and windows do not
// each re-query the monitor list to resolve a name.
func readMonitors() ([]hyprMonitor, error) {
	out, err := ctl("monitors", "all", "-j")
	if err != nil {
		return nil, err
	}
	var mons []hyprMonitor
	if err := json.Unmarshal(out, &mons); err != nil {
		return nil, err
	}
	return mons, nil
}

// full adds the editor detail (position, rotation, VRR, physical modes) the
// display page needs and the lean watch frame omits.
func monitorOutputs(mons []hyprMonitor, full bool) []wm.Output {
	outputs := make([]wm.Output, 0, len(mons))
	for _, m := range mons {
		o := wm.Output{
			Name:            m.Name,
			Width:           m.Width,
			Height:          m.Height,
			Scale:           m.Scale,
			Focused:         m.Focused,
			ActiveWorkspace: workspaceKey(m.ActiveWorkspace.ID, m.ActiveWorkspace.Name),
			Make:            m.Make,
			Model:           m.Model,
			PhysicalWidth:   m.PhysicalWidth,
			Disabled:        m.Disabled,
		}
		if full {
			o.X = m.X
			o.Y = m.Y
			o.Transform = m.Transform
			o.VRR = m.VRR
			modes := make([]string, 0, len(m.AvailableModes))
			for _, am := range m.AvailableModes {
				modes = append(modes, hyprModeString(am))
			}
			o.Modes = modes
			o.Mode = currentHyprMode(m, modes)
			o.Mirror = mirrorName(m.MirrorOf)
			o.ColorMode = hyprColorMode(m.ColorManagementPreset)
			o.SdrBrightness = m.SdrBrightness
		}
		outputs = append(outputs, o)
	}
	return outputs
}

// hyprModeString strips Hyprland's trailing "Hz" so an advertised mode reads the
// same as niri's "WxH@rate" and round-trips cleanly back to hl.monitor.
func hyprModeString(s string) string {
	s = strings.TrimSpace(s)
	if n := len(s); n >= 2 && strings.EqualFold(s[n-2:], "Hz") {
		s = strings.TrimSpace(s[:n-2])
	}
	return s
}

// currentHyprMode picks the advertised mode matching the monitor's live width,
// height and refresh, so the editor's current mode is one the picker also lists.
func currentHyprMode(m hyprMonitor, modes []string) string {
	prefix := fmt.Sprintf("%dx%d@", m.Width, m.Height)
	want := int(m.RefreshRate + 0.5)
	for _, s := range modes {
		if !strings.HasPrefix(s, prefix) {
			continue
		}
		if f, err := strconv.ParseFloat(s[len(prefix):], 64); err == nil && int(f+0.5) == want {
			return s
		}
	}
	return fmt.Sprintf("%dx%d@%d", m.Width, m.Height, want)
}

// mirrorName drops Hyprland's "none" sentinel so an un-mirrored output reads as
// the empty string the neutral shape uses.
func mirrorName(s string) string {
	if s == "" || s == "none" {
		return ""
	}
	return s
}

// hyprColorMode folds Hyprland's colour-management preset onto the neutral colour
// mode the editor shows: an HDR preset is hdr, sRGB/auto/none are srgb, anything
// else is a wide-gamut profile.
func hyprColorMode(preset string) string {
	switch preset {
	case "hdr", "hdredid":
		return "hdr"
	case "", "srgb", "auto":
		return "srgb"
	}
	return "wide"
}

func focusedName(mons []hyprMonitor) string {
	for _, m := range mons {
		if m.Focused {
			return m.Name
		}
	}
	return ""
}

func readWorkspaces(mons []hyprMonitor) []wm.Workspace {
	out, err := ctl("workspaces", "-j")
	if err != nil {
		return nil
	}
	var raw []struct {
		ID            int    `json:"id"`
		Name          string `json:"name"`
		Monitor       string `json:"monitor"`
		Windows       int    `json:"windows"`
		HasFullscreen bool   `json:"hasfullscreen"`
		TiledLayout   string `json:"tiledLayout"`
	}
	if json.Unmarshal(out, &raw) != nil {
		return nil
	}
	// Active is per output, not per session, so it comes from the monitor list.
	active := make(map[string]bool, len(mons))
	for _, m := range mons {
		if k := workspaceKey(m.ActiveWorkspace.ID, m.ActiveWorkspace.Name); k != "" {
			active[k] = true
		}
	}
	list := make([]wm.Workspace, 0, len(raw))
	for _, w := range raw {
		key := workspaceKey(w.ID, w.Name)
		list = append(list, wm.Workspace{
			ID:     key,
			Name:   w.Name,
			Output: w.Monitor,
			Active: active[key],
			// A negative id is how Hyprland marks a scratchpad workspace.
			Special:    w.ID < 0 || strings.HasPrefix(w.Name, "special:"),
			Windows:    w.Windows,
			Fullscreen: w.HasFullscreen,
			Layout:     w.TiledLayout,
		})
	}
	return list
}

func readWindows(mons []hyprMonitor) []wm.Window {
	out, err := ctl("clients", "-j")
	if err != nil {
		return nil
	}
	var raw []struct {
		Address   string `json:"address"`
		Class     string `json:"class"`
		Title     string `json:"title"`
		Monitor   int    `json:"monitor"`
		Floating  bool   `json:"floating"`
		At        []int  `json:"at"`
		Size      []int  `json:"size"`
		Workspace struct {
			ID   int    `json:"id"`
			Name string `json:"name"`
		} `json:"workspace"`
		FocusHistoryID int `json:"focusHistoryID"`
	}
	if json.Unmarshal(out, &raw) != nil {
		return nil
	}
	// The client list reports a monitor index while everything else uses names.
	names := make(map[int]string, len(mons))
	for _, m := range mons {
		names[m.ID] = m.Name
	}
	list := make([]wm.Window, 0, len(raw))
	for _, c := range raw {
		w := wm.Window{
			ID:         c.Address,
			AppID:      c.Class,
			Title:      c.Title,
			Workspace:  workspaceKey(c.Workspace.ID, c.Workspace.Name),
			Output:     names[c.Monitor],
			FocusOrder: c.FocusHistoryID,
			Floating:   c.Floating,
		}
		if len(c.At) == 2 {
			w.X, w.Y = c.At[0], c.At[1]
		}
		if len(c.Size) == 2 {
			w.Width, w.Height = c.Size[0], c.Size[1]
		}
		list = append(list, w)
	}
	return list
}

// readKeyboard returns the active layout and the loaded set. The main keyboard
// is the first with a usable active keymap; per-device layouts are not surfaced
// because the bar shows one indicator.
//
// Hyprland reports the transient sentinel "ERROR" as a keyboard's active_keymap
// while its xkb state is mid-transition (a virtual device coming and going, as
// dictation does), and no further activelayout event follows once it settles, so
// publishing it latches the bar on "ERROR" forever. Treat it like an empty
// keymap: skip that device, and if none resolves, return "" so the caller keeps
// the last-good layout instead of a value the compositor never confirmed.
func readKeyboard() (string, []string) {
	out, err := ctl("devices", "-j")
	if err != nil {
		return "", nil
	}
	var devs struct {
		Keyboards []struct {
			ActiveKeymap string `json:"active_keymap"`
			Layout       string `json:"layout"`
			Main         bool   `json:"main"`
		} `json:"keyboards"`
	}
	if json.Unmarshal(out, &devs) != nil {
		return "", nil
	}
	for _, k := range devs.Keyboards {
		if !k.Main || !usableKeymap(k.ActiveKeymap) {
			continue
		}
		var all []string
		for _, l := range strings.Split(k.Layout, ",") {
			if l = strings.TrimSpace(l); l != "" {
				all = append(all, l)
			}
		}
		return k.ActiveKeymap, all
	}
	return "", nil
}

// usableKeymap reports whether an active_keymap is a real layout rather than the
// empty string or Hyprland's transient "ERROR" transition sentinel.
func usableKeymap(s string) bool {
	s = strings.TrimSpace(s)
	return s != "" && !strings.EqualFold(s, "error")
}

// runState is one snapshot for callers that ask once and exit.
func runState() error {
	if !live() {
		return fmt.Errorf("state: no live Hyprland session")
	}
	mons, err := readMonitors()
	if err != nil {
		return err
	}
	active, all := readKeyboard()
	snap := wm.Snapshot{
		FocusedOutput:   focusedName(mons),
		Outputs:         monitorOutputs(mons, true),
		Workspaces:      readWorkspaces(mons),
		Windows:         readWindows(mons),
		KeyboardLayout:  active,
		KeyboardLayouts: all,
	}
	enc := json.NewEncoder(stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(snap)
}

// The name is what a dispatcher accepts and what a user recognises; the id is
// the fallback for an unnamed workspace.
func workspaceKey(id int, name string) string {
	if name != "" {
		return name
	}
	if id == 0 {
		return ""
	}
	return strconv.Itoa(id)
}

// Queried, not guessed: Hyprland refocuses a survivor on unplug, and naming the
// wrong one would send the next surface to a monitor that is gone.
func focusedFallback(gone string) string {
	mons, err := readMonitors()
	if err != nil {
		return ""
	}
	if focused := focusedName(mons); focused != gone {
		return focused
	}
	return ""
}

// focusedmonv2 is rejected on purpose: matching v1 exactly means a future
// Hyprland that drops it fails in the tests instead of going silently stale.
func parseFocusedMon(line string) (string, bool) {
	ev, data, ok := strings.Cut(line, ">>")
	if !ok || ev != "focusedmon" {
		return "", false
	}
	mon, _, _ := strings.Cut(data, ",")
	if mon == "" {
		return "", false
	}
	return mon, true
}

func parseMonitorRemoved(line string) (string, bool) {
	ev, data, ok := strings.Cut(line, ">>")
	if !ok || ev != "monitorremoved" {
		return "", false
	}
	name := strings.TrimSpace(data)
	if name == "" {
		return "", false
	}
	return name, true
}
