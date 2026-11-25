package daemon

import (
	"context"
	"errors"
	"os"
	"slices"
	"strings"
	"sync"
	"time"

	i18n "ryoku-i18n"
	"ryoku-rashin/internal/oni/capture"
	"ryoku-rashin/internal/oni/feature"
	"ryoku-rashin/internal/oni/guide"
	"ryoku-rashin/internal/oni/presence"
	"ryoku-rashin/internal/oni/spec"
	"ryoku-rashin/internal/oni/strand"
	"ryoku-rashin/internal/oni/window"
	"ryoku-rashin/internal/protocol"
)

// Oni perches only while a presenter draws it and the user put it on a
// window. Nothing here runs at rest: the desktop is first read when a perch
// or a hover asks, the window watch and the geometry poll run only while a
// perch is out, and both end with it.

// oniWindowRead bounds one read of the desktop's windows.
const oniWindowRead = 3 * time.Second

// oniPerch is the perched window as the daemon follows it.
type oniPerch struct {
	mu sync.Mutex
	// provider reads the desktop; found on first use, kept while presence
	// runs
	provider window.Provider
	tracker  window.Tracker
	// window is the perched window as last read; empty ID when detached
	window window.Window
	// gen counts perches, so a watch for an older one changes nothing
	gen  uint64
	stop context.CancelFunc
	// resting: the perched Oni sleeps, so nothing is polled; polling is
	// the window the provider was last told to sample
	resting bool
	polling window.ID
	// overlay is the presenter's own rects per output, for masking; trail
	// is each output's latest reports, newest last, for oniMasks
	overlay map[string][]window.Box
	trail   map[string][]overlayReport
	// moved is when the presenter was last told to move a part it reports
	// (the strand drawn back); reports from before it no longer say where
	// the presenter is. reported is closed at the next report, waking a
	// capture that waits for one.
	moved    time.Time
	reported chan struct{}
}

// overlayReport is one report of the presenter's rects on an output.
type overlayReport struct {
	at    time.Time
	rects []window.Box
}

// A part the presenter moves, the card gliding to its new place or the
// strand drawing back, is reported as it is committed, a frame or more
// before the screen shows it there. A capture therefore waits until the
// reports held still for oniOverlayQuiet, at most oniOverlayWait; one that
// cannot wait that long masks every place reported in the last
// oniOverlayTrail as well. Right after the daemon told the presenter to
// move (oniMoved), the reports before it held still only because the
// presenter has not drawn the move yet, so a capture first waits for a
// report from after it.
const (
	oniOverlayQuiet = 150 * time.Millisecond
	oniOverlayWait  = 1500 * time.Millisecond
	oniOverlayTrail = 300 * time.Millisecond
)

// oniMoved notes that the presenter was just told, at at, to move a part
// whose rects it reports.
func (d *Daemon) oniMoved(at time.Time) {
	o := &d.oni.perch
	o.mu.Lock()
	defer o.mu.Unlock()
	if at.After(o.moved) {
		o.moved = at
	}
}

// oniMasks is what a capture of output paints out: the presenter's own
// rects once they held still after its latest move, or, when they would
// not before oniOverlayWait or ctx ends, those and every place reported in
// the last oniOverlayTrail.
func (d *Daemon) oniMasks(ctx context.Context, output string) []window.Box {
	o := &d.oni.perch
	deadline := time.Now().Add(oniOverlayWait)
	for {
		o.mu.Lock()
		now := time.Now()
		trail := o.trail[output]
		var last time.Time
		if len(trail) > 0 {
			last = trail[len(trail)-1].at
		}
		// a report from after the latest move; a presenter that never
		// reported on output has nothing on the way
		fresh := len(trail) == 0 || last.After(o.moved)
		still := fresh && (len(trail) == 0 || now.Sub(last) >= oniOverlayQuiet)
		if still || !now.Before(deadline) || ctx.Err() != nil {
			masks := slices.Clone(o.overlay[output])
			if !still {
				for _, r := range trail {
					if now.Sub(r.at) <= oniOverlayTrail {
						masks = append(masks, r.rects...)
					}
				}
			}
			o.mu.Unlock()
			return masks
		}
		wait := deadline.Sub(now)
		if fresh {
			wait = min(oniOverlayQuiet-now.Sub(last), wait)
		}
		if o.reported == nil {
			o.reported = make(chan struct{})
		}
		reported := o.reported
		o.mu.Unlock()
		t := time.NewTimer(wait)
		select {
		case <-t.C:
		case <-reported:
			t.Stop()
		case <-ctx.Done():
			t.Stop()
		}
	}
}

// pollGeometry is the window whose geometry the provider should sample:
// only a perched window that is floating, and only while Oni is awake. A
// tiled window moves with the layout, on the events the watch already reads
// (a split dragged by hand is the exception, as in the Ryoku seam); a
// floating one can move silently (Hyprland publishes no move event), which
// only a sample sees. The caller holds o.mu.
func (o *oniPerch) pollGeometry() window.ID {
	if o.window.Floating && !o.resting {
		return o.window.ID
	}
	return ""
}

// poll tells p when pollGeometry changes. The caller holds o.mu.
func (o *oniPerch) poll(p window.Provider) {
	if want := o.pollGeometry(); p != nil && want != o.polling {
		o.polling = want
		p.Geometry(want)
	}
}

// oniWindows is where Oni reads the desktop's windows.
func (d *Daemon) oniWindows() (window.Provider, error) {
	if d.deps.OniWindows != nil {
		return d.deps.OniWindows()
	}
	env := os.Environ()
	return window.Detect(env, window.Exec(env))
}

// oniCapture is how Oni pictures one window of p's desktop.
func (d *Daemon) oniCapture(p window.Provider) capture.Service {
	if d.deps.OniCapture != nil {
		return d.deps.OniCapture(p)
	}
	return capture.New(capture.Exec(os.Environ()), oniCapturePaths(os.Getenv), capture.Layout(p.Snapshot))
}

// oniCapturePaths is the whole capture ladder, or, in the Oni lab only
// (HYAKKI_ONI_LAB=1), the comma list HYAKKI_ONI_CAPTURE_PATHS names, so a
// proof can hide a path. A name the ladder does not know allows nothing.
func oniCapturePaths(getenv func(string) string) []capture.Path {
	list := getenv("HYAKKI_ONI_CAPTURE_PATHS")
	if getenv("HYAKKI_ONI_LAB") != "1" || list == "" {
		return capture.Ladder
	}
	var paths []capture.Path
	for _, name := range strings.Split(list, ",") {
		p := capture.Path(strings.TrimSpace(name))
		if !slices.Contains(capture.Ladder, p) {
			return nil
		}
		paths = append(paths, p)
	}
	return paths
}

// oniProvider is the running presence's desktop reader, found on first use.
func (d *Daemon) oniProvider() (window.Provider, error) {
	o := &d.oni.perch
	o.mu.Lock()
	defer o.mu.Unlock()
	if o.provider != nil {
		return o.provider, nil
	}
	p, err := d.oniWindows()
	if err != nil {
		return nil, err
	}
	o.provider = p
	return p, nil
}

// oniCaller refuses a perch, detach or overlay call from anyone but the app,
// the CLI or, where presenter allows it, the surface drawing Oni. The MCP
// bridge passes too: mcpGate already held it to the tools turned on, and
// none of them perches, detaches or draws. A connection that opened the
// oni event as an observer only watches.
func (d *Daemon) oniCaller(c *connection, presenter bool) error {
	refused := protocol.Errorf(protocol.CodeForbidden, "%s", i18n.T("Only the Hyakki app, the hyakki command or the surface drawing Oni can do this."))
	d.oni.clientsMu.Lock()
	isPresenter := d.oni.presenter == c
	client := d.oni.clients[c]
	d.oni.clientsMu.Unlock()
	if isPresenter {
		if presenter {
			return nil
		}
		return refused
	}
	if client != nil && client.renderer == protocol.OniRendererObserver {
		return refused
	}
	c.mu.RLock()
	kind := c.kind
	c.mu.RUnlock()
	if c.session == "" && (c.app || kind == protocol.ClientCLI || kind == protocol.ClientMCP) {
		return nil
	}
	return refused
}

func perchRefusal(code, reason, message string) error {
	return &protocol.Error{Code: code, Message: message, Reason: reason}
}

// oniPerchCall perches Oni on the window p names, or hit-tests for a hover.
func (d *Daemon) oniPerchCall(c *connection, p protocol.OniPerchParams) (protocol.OniPerchResult, error) {
	if err := d.oniCaller(c, true); err != nil {
		return protocol.OniPerchResult{}, err
	}
	svc := d.oni.service.Load()
	settings := protocol.EffectiveOni(d.Settings())
	d.oni.clientsMu.Lock()
	drawn := d.oni.presenter != nil
	d.oni.clientsMu.Unlock()
	switch {
	case !settings.Master:
		return protocol.OniPerchResult{}, perchRefusal(protocol.CodeDisabled, feature.PerchOniOff, i18n.T("Oni is off."))
	case settings.Hidden:
		return protocol.OniPerchResult{}, perchRefusal(protocol.CodeUnavailable, feature.PerchHidden, i18n.T("Oni is hidden, so it cannot perch."))
	case svc == nil || !drawn:
		return protocol.OniPerchResult{}, perchRefusal(protocol.CodeUnavailable, feature.PerchPresenterLost, i18n.T("Nothing is drawing Oni, so it cannot perch."))
	}
	provider, err := d.oniProvider()
	if err != nil {
		return protocol.OniPerchResult{}, perchRefusal(protocol.CodeUnavailable, feature.PerchNoGeometry, i18n.Tf("Oni cannot read the windows on this desktop: %v.", err))
	}
	ctx, cancel := context.WithTimeout(context.Background(), oniWindowRead)
	snap, err := provider.Snapshot(ctx)
	cancel()
	if err != nil {
		return protocol.OniPerchResult{}, perchRefusal(protocol.CodeUnavailable, feature.PerchNoGeometry, i18n.Tf("Oni cannot read the windows on this desktop: %v.", err))
	}
	w, found := oniFind(snap, p)
	reason, message := perchable(w, found)
	if p.Hover {
		result := protocol.OniPerchResult{Reason: reason}
		if found {
			result.Window, result.App, result.Title, result.Output = string(w.ID), oniText(w.App, 128), oniText(w.Title, 256), w.Output
			result.Box = rectOf(w.Box)
		}
		return result, nil
	}
	if reason != "" {
		code := protocol.CodeUnavailable
		if reason == feature.PerchClosed {
			code = protocol.CodeNotFound
		}
		return protocol.OniPerchResult{}, perchRefusal(code, reason, message)
	}
	frame := feature.PerchFrame{
		Window: string(w.ID),
		App:    oniText(w.App, 128),
		Title:  oniText(w.Title, 256),
		Output: w.Output,
		Box:    boxOf(*w.Box),
		Corner: oniCorner(svc, w, p, snap.Outputs),
	}
	seq, err := svc.Perch(frame)
	if err != nil {
		return protocol.OniPerchResult{}, presenceRefusal(err)
	}
	d.oniFollowPerch(svc, provider, w, snap, seq)
	return protocol.OniPerchResult{Seq: seq, Window: frame.Window, App: frame.App, Title: frame.Title, Output: frame.Output, Corner: frame.Corner, Box: rectOf(w.Box)}, nil
}

// oniFollowPerch follows w once presence perched on it as seq, unless that
// perch already ended or another took its place.
func (d *Daemon) oniFollowPerch(svc *presence.Service, provider window.Provider, w window.Window, snap window.Snapshot, seq uint64) {
	o := &d.oni.perch
	o.mu.Lock()
	defer o.mu.Unlock()
	if p, ok := svc.Perched(); !ok || p.Seq != seq {
		return
	}
	d.oniFollow(svc, provider, w, snap)
}

// oniFind is the window p names, or the one under its point.
func oniFind(snap window.Snapshot, p protocol.OniPerchParams) (window.Window, bool) {
	if p.Window != "" {
		for _, w := range snap.Windows {
			if string(w.ID) == p.Window {
				return w, true
			}
		}
		return window.Window{}, false
	}
	i := slices.IndexFunc(snap.Outputs, func(o window.Output) bool { return o.Name == p.Output })
	if i < 0 {
		return window.Window{}, false
	}
	o := snap.Outputs[i]
	var t window.Tracker
	return t.HitTest(spec.Point{X: float64(o.X) + p.X, Y: float64(o.Y) + p.Y}, snap)
}

// perchable says why Oni cannot perch on w, or nothing when it can.
func perchable(w window.Window, found bool) (reason, message string) {
	switch {
	case !found:
		return feature.PerchClosed, i18n.T("There is no open window there for Oni to perch on.")
	case w.Minimized || !w.Visible:
		return feature.PerchHidden, i18n.T("That window is not on screen, so Oni cannot perch on it.")
	case w.Box == nil:
		why := w.Unplaced
		if why == "" {
			why = i18n.T("this desktop does not say where the window is")
		}
		return feature.PerchNoGeometry, i18n.Tf("Oni cannot perch there: %s.", why)
	}
	if b := boxOf(*w.Box); b.W <= 0 || b.H <= 0 {
		return feature.PerchNoGeometry, i18n.T("Oni cannot perch there: the window has no size.")
	}
	return "", ""
}

// oniCorner is the window's top corner nearer to Oni, by the strand's own
// rule: nearer to the drop point, or to where Oni stands on that output,
// passing over a corner whose curl would leave the output.
func oniCorner(svc *presence.Service, w window.Window, p protocol.OniPerchParams, outputs []window.Output) string {
	frame := strand.Rect{X: w.Box.X, Y: w.Box.Y, W: w.Box.W, H: w.Box.H}
	var screen strand.Rect
	centre := spec.Point{X: frame.X + frame.W/2, Y: frame.Y}
	if i := slices.IndexFunc(outputs, func(o window.Output) bool { return o.Name == w.Output }); i >= 0 {
		o := outputs[i]
		screen = strand.Rect{W: float64(o.W), H: float64(o.H)}
		centre = spec.Point{X: screen.W / 2, Y: screen.H / 2}
	}
	from := centre
	switch {
	case p.Output != "":
		from = spec.Point{X: p.X, Y: p.Y}
	case screen.W > 0:
		if fx, fy, ok := svc.Spot(w.Output); ok {
			from = spec.Point{X: fx * screen.W, Y: fy * screen.H}
		}
	}
	s, err := spec.Load()
	if err != nil {
		s, screen = &spec.OniSpec{}, strand.Rect{}
	}
	return strand.PerchCorner(s, frame, from, centre, screen)
}

// oniFollow starts following w: the window watch and the geometry poll run
// from now until the perch ends. A question about the window before is
// dropped with its picture, and a walkthrough kept for its app may be
// hinted. The caller holds d.oni.perch.mu.
func (d *Daemon) oniFollow(svc *presence.Service, provider window.Provider, w window.Window, snap window.Snapshot) {
	d.oniAskEnd()
	d.guideUnwindOff(w.ID)
	o := &d.oni.perch
	if o.stop != nil {
		o.stop()
	}
	o.gen++
	gen := o.gen
	o.window = w
	o.tracker = window.Tracker{}
	o.tracker.Track(w.ID)
	o.tracker.Update(snap)
	ctx, stop := context.WithCancel(context.Background())
	o.stop = stop
	o.poll(provider)
	go func() {
		_ = provider.Watch(ctx, func(s window.Snapshot) { d.oniWindowsChanged(svc, gen, s) })
	}()
	d.hintPerched(w)
}

// oniWindowsChanged follows the perched window through one new snapshot:
// a move or resize sends a new perch frame, a close, hide or lost output
// detaches, and focus moves the perch only when the setting says so.
func (d *Daemon) oniWindowsChanged(svc *presence.Service, gen uint64, s window.Snapshot) {
	o := &d.oni.perch
	o.mu.Lock()
	if gen != o.gen || o.window.ID == "" {
		o.mu.Unlock()
		return
	}
	changes := o.tracker.Update(s)
	perched := o.window
	detach, update, follow := "", false, window.Window{}
	for _, ch := range changes {
		switch ch.Kind {
		case window.ChangeClosed:
			detach = feature.PerchClosed
		case window.ChangeHidden:
			detach = feature.PerchHidden
		case window.ChangeFocus:
			follow = ch.Window
		default:
			perched = ch.Window
			update = true
		}
	}
	if detach == "" {
		switch {
		case !slices.ContainsFunc(s.Outputs, func(out window.Output) bool { return out.Name == perched.Output }):
			detach = feature.PerchOutputGone
		case !update:
		case perched.Box == nil:
			detach = feature.PerchNoGeometry
		case perched.Minimized || !perched.Visible:
			detach = feature.PerchHidden
		}
	}
	if detach == "" && update {
		o.window = perched
	}
	// a window can float or tile without moving: the poll follows its
	// floating state on every read, not only on a change
	if i := slices.IndexFunc(s.Windows, func(w window.Window) bool { return w.ID == o.window.ID }); i >= 0 && detach == "" {
		o.window.Floating = s.Windows[i].Floating
	}
	if detach == "" {
		o.poll(o.provider)
	}
	o.mu.Unlock()
	switch {
	case detach != "":
		svc.Detach(detach)
		return
	case update:
		svc.PerchUpdate(*boxOf(*perched.Box), perched.Output)
	}
	// focus moving on the desktop, a dialog or a menu window opening or
	// closing, is the user acting on a guide's step: it advances like Next,
	// and the settle watch then says whether the window changed
	if follow.ID != "" {
		d.guideAdvance(guide.ByChange)
	}
	// focus coming back to the perched window may bring a hint for its app
	if follow.ID != "" && follow.ID == perched.ID {
		d.hintPerched(perched)
	}
	if follow.ID == "" || follow.ID == perched.ID || !protocol.EffectiveOni(d.Settings()).PerchFollowsFocus {
		return
	}
	if reason, _ := perchable(follow, true); reason != "" {
		return
	}
	o.mu.Lock()
	current, provider := o.gen, o.provider
	o.mu.Unlock()
	if gen != current || provider == nil {
		return
	}
	frame := feature.PerchFrame{
		Window: string(follow.ID),
		App:    oniText(follow.App, 128),
		Title:  oniText(follow.Title, 256),
		Output: follow.Output,
		Box:    boxOf(*follow.Box),
		Corner: oniCorner(svc, follow, protocol.OniPerchParams{}, s.Outputs),
	}
	seq, err := svc.Perch(frame)
	if err != nil {
		return
	}
	d.oniFollowPerch(svc, provider, follow, s, seq)
}

// oniRest stops the geometry poll while a perched Oni sleeps; waking turns
// it back on (for a floating window) and catches up with where the window
// went meanwhile.
func (d *Daemon) oniRest(resting bool) {
	o := &d.oni.perch
	o.mu.Lock()
	o.resting = resting
	provider, gen, perched := o.provider, o.gen, o.window.ID != ""
	o.poll(provider)
	o.mu.Unlock()
	if !perched || provider == nil {
		return
	}
	svc := d.oni.service.Load()
	if resting || svc == nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), oniWindowRead)
	defer cancel()
	if snap, err := provider.Snapshot(ctx); err == nil {
		d.oniWindowsChanged(svc, gen, snap)
	}
}

// oniDetached ends the window watch, the geometry poll and any picture
// waiting to be sent, whatever ended the perch.
func (d *Daemon) oniDetached(reason string) {
	o := &d.oni.perch
	o.mu.Lock()
	o.gen++
	o.window = window.Window{}
	o.tracker = window.Tracker{}
	if o.stop != nil {
		o.stop()
		o.stop = nil
	}
	if o.provider != nil && o.polling != "" {
		o.provider.Geometry("")
	}
	o.polling = ""
	o.mu.Unlock()
	d.oniAskEnd()
	d.guideUnwind(reason)
}

// oniPerchReset forgets the desktop reader and the overlay when presence
// stops. The caller holds d.oni.mu.
func (d *Daemon) oniPerchReset() {
	o := &d.oni.perch
	o.mu.Lock()
	defer o.mu.Unlock()
	o.gen++
	o.resting = false
	if o.stop != nil {
		o.stop()
		o.stop = nil
	}
	if o.provider != nil {
		o.provider.Geometry("")
	}
	o.provider, o.window, o.tracker, o.overlay, o.polling = nil, window.Window{}, window.Tracker{}, nil, ""
}

// oniDetachCall lets go of the perched window.
func (d *Daemon) oniDetachCall(c *connection, p protocol.OniDetachParams) error {
	if err := d.oniCaller(c, true); err != nil {
		return err
	}
	reason := p.Reason
	if reason == "" {
		reason = feature.PerchUser
	}
	d.oni.clientsMu.Lock()
	presenter := d.oni.presenter == c
	d.oni.clientsMu.Unlock()
	if reason != feature.PerchUser && !presenter {
		return protocol.Errorf(protocol.CodeForbidden, "%s", i18n.T("Only the surface drawing Oni reports that Oni was dragged off or hidden."))
	}
	if svc := d.oni.service.Load(); svc != nil {
		svc.Detach(reason)
	}
	return nil
}

// oniIsPresenter says whether c is the surface drawing Oni.
func (d *Daemon) oniIsPresenter(c *connection) bool {
	d.oni.clientsMu.Lock()
	defer d.oni.clientsMu.Unlock()
	return d.oni.presenter == c
}

// oniOverlayCall keeps the presenter's own rects on one output, so a crop
// masks Oni out of the picture.
func (d *Daemon) oniOverlayCall(c *connection, p protocol.OniOverlayParams) error {
	d.oni.clientsMu.Lock()
	presenter := d.oni.presenter == c
	d.oni.clientsMu.Unlock()
	if !presenter {
		return protocol.Errorf(protocol.CodeForbidden, "%s", i18n.T("Only the surface drawing Oni reports where it draws."))
	}
	o := &d.oni.perch
	o.mu.Lock()
	defer o.mu.Unlock()
	rects := make([]window.Box, 0, len(p.Rects))
	for _, r := range p.Rects {
		rects = append(rects, window.Box{X: r.X, Y: r.Y, W: r.W, H: r.H})
	}
	now := time.Now()
	if o.trail == nil {
		o.trail = map[string][]overlayReport{}
	}
	trail := o.trail[p.Output]
	for len(trail) > 0 && now.Sub(trail[0].at) > oniOverlayTrail {
		trail = trail[1:]
	}
	o.trail[p.Output] = append(trail, overlayReport{at: now, rects: rects})
	if o.reported != nil {
		close(o.reported)
		o.reported = nil
	}
	if len(rects) == 0 {
		delete(o.overlay, p.Output)
		return nil
	}
	if o.overlay == nil {
		o.overlay = map[string][]window.Box{}
	}
	o.overlay[p.Output] = rects
	return nil
}

// presenceRefusal names why presence sent nothing.
func presenceRefusal(err error) error {
	switch {
	case errors.Is(err, presence.ErrOff):
		return perchRefusal(protocol.CodeDisabled, feature.PerchOniOff, i18n.T("Oni is off."))
	case errors.Is(err, presence.ErrHidden):
		return perchRefusal(protocol.CodeUnavailable, feature.PerchHidden, i18n.T("Oni is hidden, so it cannot perch."))
	case errors.Is(err, presence.ErrNoPresenter), errors.Is(err, presence.ErrClosed):
		return perchRefusal(protocol.CodeUnavailable, feature.PerchPresenterLost, i18n.T("Nothing is drawing Oni, so it cannot perch."))
	}
	return protocol.Errorf(protocol.CodeBadRequest, "%s", i18n.Tf("Oni cannot perch on that window: %v.", err))
}

func boxOf(b window.Box) *feature.Box {
	return &feature.Box{X: b.X, Y: b.Y, W: b.W, H: b.H}
}

func rectOf(b *window.Box) *protocol.OniRect {
	if b == nil {
		return nil
	}
	return &protocol.OniRect{X: b.X, Y: b.Y, W: b.W, H: b.H}
}

// oniText is s cut to limit runes with control and invisible formatting
// characters dropped, as a frame may carry it; empty when nothing is left.
func oniText(s string, limit int) string {
	out := make([]rune, 0, len(s))
	for _, r := range s {
		if len(out) == limit {
			break
		}
		if protocol.ValidOniText(string(r), 1, false) {
			out = append(out, r)
		}
	}
	return string(out)
}
