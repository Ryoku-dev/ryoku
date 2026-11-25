package window

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"time"

	"ryoku-rashin/internal/oni/spec"
)

// Hyprland, read over its request socket (the request `hyprctl -j` would
// send, written straight to the socket, so a read starts no process) and its
// event socket (socket2, which only ever sends) for change notices. Hyprland
// publishes no move or resize event, so the perched window's geometry is
// sampled while Geometry names it: quickly right after it changed or an event
// named it, then less often while it holds still.

// hyprPollFast is the sample period right after the perched window changed
// or an event named it: the Ryoku seam's period for moving windows.
const hyprPollFast = 250 * time.Millisecond

// hyprPollSlow caps the backoff: each sample that finds the perched window
// where it was doubles the period up to this.
const hyprPollSlow = 2 * time.Second

// hyprDebounce folds a burst of events into one read pass.
const hyprDebounce = 8 * time.Millisecond

// hyprRequestTimeout bounds one request on the request socket.
const hyprRequestTimeout = time.Second

type hyprClient struct {
	Address   string     `json:"address"`
	Mapped    bool       `json:"mapped"`
	Hidden    bool       `json:"hidden"`
	At        [2]float64 `json:"at"`
	Size      [2]float64 `json:"size"`
	Workspace struct {
		ID int `json:"id"`
	} `json:"workspace"`
	Floating       bool            `json:"floating"`
	Monitor        int             `json:"monitor"`
	Class          string          `json:"class"`
	Title          string          `json:"title"`
	PID            int             `json:"pid"`
	Fullscreen     json.RawMessage `json:"fullscreen"`
	FocusHistoryID int             `json:"focusHistoryID"`
}

// fullscreen reads both Hyprland forms: the old boolean, and the newer mode
// number where 1 is maximized and 2 or 3 is fullscreen.
func (c hyprClient) fullscreen() bool {
	var b bool
	if json.Unmarshal(c.Fullscreen, &b) == nil {
		return b
	}
	var n int
	return json.Unmarshal(c.Fullscreen, &n) == nil && n >= 2
}

type hyprMonitor struct {
	ID              int     `json:"id"`
	Name            string  `json:"name"`
	Width           int     `json:"width"`
	Height          int     `json:"height"`
	X               int     `json:"x"`
	Y               int     `json:"y"`
	Scale           float64 `json:"scale"`
	Transform       int     `json:"transform"`
	Disabled        bool    `json:"disabled"`
	ActiveWorkspace struct {
		ID int `json:"id"`
	} `json:"activeWorkspace"`
	SpecialWorkspace struct {
		ID int `json:"id"`
	} `json:"specialWorkspace"`
}

// logical is the monitor's size in the global logical layout: the mode
// divided by scale, swapped when the panel is rotated a quarter turn.
func (m hyprMonitor) logical() (int, int) {
	scale := m.Scale
	if scale <= 0 {
		scale = 1
	}
	w := int(math.Round(float64(m.Width) / scale))
	h := int(math.Round(float64(m.Height) / scale))
	if m.Transform%2 == 1 {
		w, h = h, w
	}
	return w, h
}

// timer arms one tick after d and returns its stop. Tests inject one to see
// exactly when, and after how long, the next sample is due.
type timer func(d time.Duration) (<-chan time.Time, func())

func realTimer(d time.Duration) (<-chan time.Time, func()) {
	t := time.NewTimer(d)
	return t.C, func() { t.Stop() }
}

type hyprland struct {
	run      Runner
	events   func(ctx context.Context) (io.ReadCloser, error)
	newTimer timer

	mu sync.Mutex
	// target is the window whose geometry is sampled; empty while no perch
	// is out
	target ID
	kick   chan struct{}
}

func newHyprland(run Runner, events func(ctx context.Context) (io.ReadCloser, error)) *hyprland {
	return &hyprland{run: guard(run), events: events, newTimer: realTimer, kick: make(chan struct{}, 1)}
}

// HyprlandIPC is how the Hyprland provider reaches the compositor: Requests
// serves the allowlisted `hyprctl` reads, Events opens the event socket.
type HyprlandIPC struct {
	Requests Runner
	Events   func(ctx context.Context) (io.ReadCloser, error)
}

// HyprlandSockets is the running Hyprland's request and event sockets
// named by env.
func HyprlandSockets(env []string) HyprlandIPC {
	return HyprlandIPC{Requests: HyprlandRequests(env), Events: HyprlandEvents(env)}
}

// hyprDial connects to one of the running Hyprland's sockets named by env:
// under XDG_RUNTIME_DIR, or under /tmp where older releases put them.
func hyprDial(env []string, file string) func(ctx context.Context) (net.Conn, error) {
	sig := lookup(env, EnvHyprlandInstance)
	runtime := lookup(env, "XDG_RUNTIME_DIR")
	return func(ctx context.Context) (net.Conn, error) {
		if sig == "" || strings.ContainsAny(sig, "/\x00") {
			return nil, fmt.Errorf("window: no Hyprland instance signature")
		}
		var d net.Dialer
		var last error
		for _, dir := range []string{runtime, "/tmp"} {
			if dir == "" {
				continue
			}
			c, err := d.DialContext(ctx, "unix", filepath.Join(dir, "hypr", sig, file))
			if err == nil {
				return c, nil
			}
			last = err
		}
		return nil, last
	}
}

// hyprSnapshotBatch is the one request a snapshot makes: Hyprland's batch of
// the monitor and client lists, answered on one connection, so a sample
// wakes the compositor once instead of twice.
const hyprSnapshotBatch = "j/monitors;j/clients"

// HyprlandRequests serves the allowlisted `hyprctl -j VERB` and `hyprctl
// --batch` reads on the running Hyprland's request socket: the same request
// hyprctl sends, with no process started. Any other argv is refused before
// anything is dialled.
func HyprlandRequests(env []string) Runner {
	dial := hyprDial(env, ".socket.sock")
	return func(ctx context.Context, name string, args ...string) ([]byte, error) {
		if name != CommandHyprland || len(args) != 2 || !Allowed(name, args...) {
			return nil, refuse(name, args)
		}
		var req string
		switch args[0] {
		case "-j":
			req = "j/" + args[1]
		case "--batch":
			req = "[[BATCH]]" + args[1]
		default:
			return nil, refuse(name, args)
		}
		ctx, cancel := context.WithTimeout(ctx, hyprRequestTimeout)
		defer cancel()
		c, err := dial(ctx)
		if err != nil {
			return nil, err
		}
		defer c.Close()
		if dl, ok := ctx.Deadline(); ok {
			_ = c.SetDeadline(dl)
		}
		if _, err := io.WriteString(c, req); err != nil {
			return nil, fmt.Errorf("hyprland %s: %w", req, err)
		}
		if u, ok := c.(*net.UnixConn); ok {
			_ = u.CloseWrite()
		}
		out, err := io.ReadAll(c)
		if err != nil {
			return nil, fmt.Errorf("hyprland %s: %w", req, err)
		}
		return out, nil
	}
}

// HyprlandEvents dials the running Hyprland's event socket named by env.
func HyprlandEvents(env []string) func(ctx context.Context) (io.ReadCloser, error) {
	dial := hyprDial(env, ".socket2.sock")
	return func(ctx context.Context) (io.ReadCloser, error) {
		return dial(ctx)
	}
}

func (p *hyprland) Caps() Caps {
	return Caps{Provider: ProviderHyprland, Geometry: GeomAll, Events: p.events != nil, Pointer: true}
}

// Snapshot reads the monitor and client lists in one batched request. The
// batch answer is the two JSON documents in order, separated by blank lines.
func (p *hyprland) Snapshot(ctx context.Context) (Snapshot, error) {
	var mons []hyprMonitor
	var clients []hyprClient
	out, err := p.run(ctx, CommandHyprland, "--batch", hyprSnapshotBatch)
	if err != nil {
		return Snapshot{}, err
	}
	dec := json.NewDecoder(bytes.NewReader(out))
	if err := dec.Decode(&mons); err != nil {
		return Snapshot{}, fmt.Errorf("hyprctl monitors: %w", err)
	}
	if err := dec.Decode(&clients); err != nil {
		return Snapshot{}, fmt.Errorf("hyprctl clients: %w", err)
	}
	return hyprConvert(mons, clients), nil
}

func (p *hyprland) Geometry(id ID) {
	p.mu.Lock()
	p.target = id
	p.mu.Unlock()
	select {
	case p.kick <- struct{}{}:
	default:
	}
}

func (p *hyprland) geometryTarget() ID {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.target
}

// Watch reads one snapshot, then re-reads after each burst of events and,
// while Geometry names a window, on the adaptive geometry sample.
func (p *hyprland) Watch(ctx context.Context, fn func(Snapshot)) error {
	if p.events == nil {
		return ErrNoStream
	}
	rc, err := p.events(ctx)
	if err != nil {
		return err
	}
	stop := context.AfterFunc(ctx, func() { _ = rc.Close() })
	defer func() {
		stop()
		_ = rc.Close()
	}()
	lines := make(chan string, 64)
	readErr := make(chan error, 1)
	go func() {
		defer close(lines)
		scan := bufio.NewScanner(rc)
		for scan.Scan() {
			select {
			case lines <- scan.Text():
			case <-ctx.Done():
				return
			}
		}
		readErr <- scan.Err()
	}()

	e := emitter{fn: fn}
	var last Snapshot
	refresh := func() (bool, error) {
		s, err := p.Snapshot(ctx)
		if err != nil {
			if ctx.Err() != nil {
				return false, nil
			}
			return false, err
		}
		last = s
		e.emit(s)
		return true, nil
	}
	if _, err := refresh(); err != nil {
		return err
	}

	// the sampled window, its record at the last read, and the sample timer
	var (
		target   ID
		seen     Window
		wait     time.Duration
		tick     <-chan time.Time
		stopTick func()
	)
	schedule := func(d time.Duration) {
		if stopTick != nil {
			stopTick()
		}
		wait = d
		tick, stopTick = p.newTimer(d)
	}
	halt := func() {
		if stopTick != nil {
			stopTick()
		}
		tick, stopTick, wait = nil, nil, 0
	}
	defer halt()
	// moved records the target as last read and says whether it changed.
	moved := func() bool {
		w, _ := findWindow(last, target)
		if reflect.DeepEqual(w, seen) {
			return false
		}
		seen = w
		return true
	}
	arm := func() {
		id := p.geometryTarget()
		switch {
		case id == "":
			target = ""
			halt()
		case id != target:
			target = id
			moved()
			schedule(hyprPollFast)
		}
	}
	arm()

	debounce := time.NewTimer(time.Hour)
	debounce.Stop()
	defer debounce.Stop()
	// named is set when an event in the pending burst named the target
	named := false
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-p.kick:
			arm()
		case <-tick:
			ok, err := refresh()
			if err != nil {
				return err
			}
			if !ok {
				continue
			}
			if moved() {
				schedule(hyprPollFast)
			} else {
				schedule(min(2*wait, hyprPollSlow))
			}
		case <-debounce.C:
			ok, err := refresh()
			if err != nil {
				return err
			}
			if ok && target != "" && (moved() || named) {
				schedule(hyprPollFast)
			}
			named = false
		case line, ok := <-lines:
			if !ok {
				if ctx.Err() != nil {
					return nil
				}
				if err := <-readErr; err != nil {
					return err
				}
				return io.ErrUnexpectedEOF
			}
			if hyprRelevant(line) {
				named = named || (target != "" && hyprNames(line, target))
				debounce.Reset(hyprDebounce)
			}
		}
	}
}

// findWindow is the window id in s, if s has it.
func findWindow(s Snapshot, id ID) (Window, bool) {
	for _, w := range s.Windows {
		if w.ID == id {
			return w, true
		}
	}
	return Window{}, false
}

// hyprNames is true when an event line is about window id: Hyprland's
// window events lead their data with the address, without its 0x.
func hyprNames(line string, id ID) bool {
	_, data, ok := strings.Cut(line, ">>")
	if !ok {
		return false
	}
	addr, _, _ := strings.Cut(data, ",")
	return addr != "" && addr == strings.TrimPrefix(string(id), "0x")
}

// hyprRelevant is true for the events that can change a window's identity,
// placement, focus, visibility or output.
func hyprRelevant(line string) bool {
	name, _, _ := strings.Cut(line, ">>")
	switch strings.TrimSuffix(name, "v2") {
	case "openwindow", "closewindow", "movewindow", "activewindow", "windowtitle",
		"fullscreen", "changefloatingmode", "minimized", "pin",
		"workspace", "focusedmon", "moveworkspace", "renameworkspace",
		"createworkspace", "destroyworkspace", "activespecial",
		"monitoradded", "monitorremoved", "moveintogroup", "moveoutofgroup":
		return true
	}
	return false
}

func (p *hyprland) Pointer(ctx context.Context) (spec.Point, bool, error) {
	out, err := p.run(ctx, CommandHyprland, "-j", "cursorpos")
	if err != nil {
		return spec.Point{}, false, err
	}
	var v struct {
		X float64 `json:"x"`
		Y float64 `json:"y"`
	}
	if err := json.Unmarshal(out, &v); err != nil {
		return spec.Point{}, false, fmt.Errorf("hyprctl cursorpos: %w", err)
	}
	return spec.Point{X: v.X, Y: v.Y}, true, nil
}

func hyprConvert(mons []hyprMonitor, clients []hyprClient) Snapshot {
	var s Snapshot
	byID := map[int]hyprMonitor{}
	visible := map[int]bool{}
	for _, m := range mons {
		if m.Disabled {
			continue
		}
		byID[m.ID] = m
		w, h := m.logical()
		s.Outputs = append(s.Outputs, Output{Name: m.Name, X: m.X, Y: m.Y, W: w, H: h, Scale: m.Scale})
		visible[m.ActiveWorkspace.ID] = true
		if m.SpecialWorkspace.ID != 0 {
			visible[m.SpecialWorkspace.ID] = true
		}
	}
	for _, c := range clients {
		if !c.Mapped || c.Address == "" {
			continue
		}
		w := Window{
			ID: ID(c.Address), App: c.Class, Title: c.Title, PID: c.PID,
			Floating: c.Floating, Fullscreen: c.fullscreen(),
			FocusOrder: c.FocusHistoryID, Focused: c.FocusHistoryID == 0,
			Visible: !c.Hidden && visible[c.Workspace.ID],
		}
		m, ok := byID[c.Monitor]
		if ok {
			w.Output = m.Name
		}
		switch {
		case !w.Visible:
			w.Unplaced = UnplacedOffView
		case ok && c.Size[0] > 0 && c.Size[1] > 0:
			w.Box = &Box{X: c.At[0] - float64(m.X), Y: c.At[1] - float64(m.Y), W: c.Size[0], H: c.Size[1]}
		default:
			w.Unplaced = UnplacedNoGeom
		}
		s.Windows = append(s.Windows, w)
	}
	sortWindows(s.Windows)
	return s
}

func lookup(env []string, key string) string {
	for i := len(env) - 1; i >= 0; i-- {
		if k, v, ok := strings.Cut(env[i], "="); ok && k == key {
			return v
		}
	}
	return ""
}
