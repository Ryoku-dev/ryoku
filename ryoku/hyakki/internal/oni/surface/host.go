package surface

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"image"
	"math"
	"testing/fstest"
	"time"
	"unicode/utf8"

	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/palette"

	"ryoku-rashin/internal/client"
	"ryoku-rashin/internal/oni/feature"
	"ryoku-rashin/internal/oni/motion"
	"ryoku-rashin/internal/oni/placement"
	"ryoku-rashin/internal/oni/profile"
	"ryoku-rashin/internal/oni/render"
	"ryoku-rashin/internal/oni/render/desktoplook"
	"ryoku-rashin/internal/oni/sound"
	"ryoku-rashin/internal/oni/spec"
	"ryoku-rashin/internal/oni/strand"
	"ryoku-rashin/internal/protocol"
)

// callTimeout bounds one daemon call made for the user's gesture.
const callTimeout = 5 * time.Second

// ErrPresenterRefused means the daemon would not let this process present:
// Oni is off, or another presenter holds the screen.
var ErrPresenterRefused = errors.New("oni refused the presenter")

// Options configure Run.
type Options struct {
	Lab    Lab
	Dial   Dialer
	Ledger *Ledger
}

// Run presents Oni until ctx ends, Oni is turned off or the display closes.
func Run(ctx context.Context, opts Options) error {
	sp, err := spec.Load()
	if err != nil {
		return err
	}
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	h := newHost(ctx, cancel, sp, opts)
	be, err := connectWayland(h)
	if err != nil {
		return err
	}
	defer be.close()
	if err := be.required(); err != nil {
		return err
	}
	h.be = be
	return h.run()
}

type flight struct {
	start time.Time
	dur   time.Duration
	path  func(t float64) image.Point
}

type host struct {
	ctx      context.Context
	cancel   context.CancelFunc
	sp       *spec.OniSpec
	eng      *motion.Engine
	led      *Ledger
	lab      Lab
	dial     Dialer
	be       backend
	exit     error
	quitting bool

	link       Link
	linking    bool
	everOpened bool
	retried    bool
	dormant    bool
	afterOpen  []func(Link)

	state        string
	daemonHidden bool
	motionF      feature.MotionFrame
	place        placement.Place
	size         string

	char          *render.Character
	charID        string
	look          render.Look
	desktopReduce bool

	out     placement.Output
	haveOut bool
	origin  image.Point
	// placed: origin is where the character was last put; placementSeen:
	// the daemon's placement has arrived once
	placed, placementSeen bool
	shown                 bool
	img                   *image.NRGBA
	pose                  motion.Pose
	dirty                 bool

	clk      clock
	timer    *time.Timer
	timerGen int
	fl       *flight
	grip     grip
	pointer  *spec.Point

	strandHost
	perchHost
	keysHost
	guideHost
	handsHost

	player   *sound.Player
	openSink func() (sound.Sink, error)
	// voice is the last voice phase the daemon sent, "" until one arrives.
	voice string
	// level is the last level frame, held only while the voice phase is
	// listening or speaking; it feeds motion.Input.Mic and Speech
	level feature.LevelFrame
	// micMark: the voice phase keeps the microphone open between presses,
	// so the mark shows; markOnly: it shows alone, Oni being hidden
	micMark, markOnly bool
}

func newHost(ctx context.Context, cancel context.CancelFunc, sp *spec.OniSpec, opts Options) *host {
	h := &host{
		ctx: ctx, cancel: cancel, sp: sp,
		eng: motion.New(sp),
		strandHost: strandHost{
			strd: strand.New(sp),
		},
		openSink: opts.Lab.soundSink(),
		led:      opts.Ledger,
		lab:      opts.Lab,
		dial:     opts.Dial,
		state:    "idle",
		motionF:  feature.MotionFrame{Reduce: "follow", Courtesy: true},
		place:    placement.Place{Corner: placement.DefaultCorner, Outputs: map[string]placement.Saved{}},
		size:     "normal",
		keysHost: keysHost{startKeys: opts.Lab.startKeys()},
	}
	h.clk.halt()
	return h
}

func (h *host) run() error {
	mech := courtesyTopLayer
	if h.be.foreignToplevels() {
		mech = courtesyForeign
	}
	h.led.Add(Entry{Ev: EvStart, Reason: mech})
	h.setDesktop(palette.ReadDesktop())
	go palette.WatchDesktop(h.ctx, func(d palette.Desktop) {
		h.be.post(func() { h.setDesktop(d) })
	})
	loaded := profile.Default()
	h.setCharacter(loaded.Profile.ID, loaded.Profile, loaded.Files)
	h.connect(true)
	err := h.be.run(h.ctx)
	h.stopTimer()
	h.clearNotice("exit")
	h.dropGlow("exit")
	h.stopKeys()
	if h.link != nil {
		h.link.Close()
	}
	if !h.quitting {
		reason := "signal"
		if err != nil {
			reason = "error"
		}
		h.led.Add(Entry{Ev: EvExit, Reason: reason})
	}
	if h.exit != nil {
		return h.exit
	}
	return err
}

func (h *host) quit(reason string, err error) {
	h.led.Add(Entry{Ev: EvExit, Reason: reason})
	h.exit, h.quitting = err, true
	h.cancel()
}

// --- daemon link ---

func (h *host) connect(start bool) {
	if h.linking {
		return
	}
	h.linking = true
	go func() {
		l, err := h.dial(h.ctx, start)
		var res protocol.OniOpenResult
		if err == nil {
			res, err = l.Open(h.ctx)
		}
		h.be.post(func() { h.opened(l, res, err) })
	}()
}

func (h *host) opened(l Link, res protocol.OniOpenResult, err error) {
	h.linking = false
	if err != nil {
		h.talkWant = false
		if l != nil {
			l.Close()
		}
		if !h.everOpened {
			if errors.Is(err, client.ErrDisabled) {
				h.quit("oni-off", err)
			} else {
				h.quit("error", err)
			}
			return
		}
		h.goDormant()
		return
	}
	if openResult(res) == openExit {
		h.talkWant = false
		l.Close()
		reason := res.Reason
		if reason == "" {
			reason = "refused"
		}
		h.quit(reason, fmt.Errorf("%w: %s", ErrPresenterRefused, reason))
		return
	}
	h.link, h.everOpened, h.dormant = l, true, false
	for _, raw := range res.Frames {
		h.frame(l, raw)
	}
	go func() {
		for raw := range l.Frames() {
			h.be.post(func() { h.frame(l, raw) })
		}
		<-l.Done()
		h.be.post(func() { h.lost(l) })
	}()
	pending := h.afterOpen
	h.afterOpen = nil
	for _, fn := range pending {
		fn(l)
	}
	h.talkOpened(l)
}

func (h *host) lost(l Link) {
	if l != h.link {
		return
	}
	h.link = nil
	l.Close()
	// the daemon cancels a pending seq when its presenter goes, and the
	// microphone is unknown until a fresh snapshot says idle
	h.tearStrand(tearDormant, "")
	// the daemon detaches a presenter that leaves, and forgets its rects
	h.hideHighlight(highlightHidden)
	h.hoverNext, h.hoverBusy = nil, false
	h.forgetOverlay()
	h.dropGuide(pickLost)
	h.clearNotice(pickLost)
	// the daemon stops a Hands task when its presenter goes, and sends the
	// approvals still waiting again to the next one that opens
	h.approvals = nil
	h.endHands(pickLost, "")
	if h.dropPerch(detachLost) {
		h.reposition(true)
	}
	h.voice = ""
	h.quietLevels()
	// the microphone belongs to the daemon, so it closed with it
	h.setMicMark("")
	if h.player != nil {
		h.player.Gate().LinkLost()
	}
	if onLinkLost(h.state, h.retried) == LossRetry {
		h.retried = true
		h.led.Add(Entry{Ev: EvRedial, Reason: "retry"})
		h.connect(false)
		return
	}
	h.goDormant()
}

func (h *host) goDormant() {
	h.dormant = true
	h.led.Add(Entry{Ev: EvDormant, State: h.state})
}

// withLink runs fn on the open link, or redials (starting the daemon, since
// the user asked) and runs it once the link opens.
func (h *host) withLink(fn func(Link)) {
	if h.link != nil {
		fn(h.link)
		return
	}
	h.afterOpen = append(h.afterOpen, fn)
	if !h.linking {
		h.retried = false
		h.led.Add(Entry{Ev: EvRedial, Reason: "click"})
		h.connect(true)
	}
}

func (h *host) call(fn func(ctx context.Context) error) {
	go func() {
		ctx, cancel := context.WithTimeout(h.ctx, callTimeout)
		defer cancel()
		fn(ctx)
	}()
}

func (h *host) interact(kind string) {
	h.withLink(func(l Link) {
		h.call(func(ctx context.Context) error { return l.Interact(ctx, kind) })
	})
}

func (h *host) frame(l Link, raw json.RawMessage) {
	if l != h.link {
		return
	}
	p, err := feature.DecodeFrame(raw)
	if err != nil {
		h.led.Add(Entry{Ev: EvFrame, Reason: "invalid"})
		return
	}
	e := Entry{Ev: EvFrame, Kind: p.Kind()}
	switch f := p.(type) {
	case *feature.GestureFrame:
		e.Seq = f.Seq
	case *feature.PlacementFrame:
		e.Seq = f.Seq
	case *feature.PerchFrame:
		e.Seq = f.Seq
	case *feature.StrandTargetFrame:
		e.Seq = f.Seq
	case *feature.StrandRetractFrame:
		e.Seq = f.Seq
	case *feature.BubbleFrame:
		e.Seq = f.Seq
	case *feature.GuideFrame:
		e.Seq = f.Seq
	case *feature.PickFrame:
		e.Seq = f.Seq
	}
	if v, ok := p.(*feature.VoiceFrame); ok {
		// the phase and source only: a voice frame's words stay off the
		// ledger
		e.Phase, e.Source = v.Phase, v.Source
	}
	h.led.Add(e)
	if exitFrame(p) {
		h.tearStrand(tearOff, "")
		h.tearPerch(detachOff)
		h.quit("oni-off", nil)
		return
	}
	switch f := p.(type) {
	case *feature.StateFrame:
		h.daemonHidden, _ = hiddenFrame(p)
		h.setState(f.State, f.Reason)
		h.updateVisible()
	case *feature.CharacterFrame:
		h.loadCharacter(f.ID)
	case *feature.GestureFrame:
		h.play(f.ID)
	case *feature.MotionFrame:
		h.motionF = *f
		h.setChord(f.Chord)
		if h.player != nil {
			h.player.Gate().Enable(f.Sounds)
		}
		h.wakeStrand()
		h.updateVisible()
		h.kick()
		if h.hands != nil {
			h.syncGlow(time.Now())
		}
	case *feature.PlacementFrame:
		h.applyPlacement(f)
	case *feature.PerchFrame:
		h.perchFrame(f)
	case *feature.StrandTargetFrame:
		h.strandTarget(f)
	case *feature.StrandRetractFrame:
		h.strandRetract(f.Seq)
	case *feature.BubbleFrame:
		h.bubbleFrame(f)
	case *feature.NoticeFrame:
		h.noticeFrame(f)
	case *feature.GuideFrame:
		h.guideFrame(f)
	case *feature.ConsentFrame:
		if handsFeature(f.Feature) {
			h.handoffFrame(f)
		} else {
			h.offerFrame(f)
		}
	case *feature.ConsentAnswerFrame:
		h.offerAnswered(f.ID)
		h.handoffAnswered(f.ID)
	case *feature.HandsFrame:
		h.handsFrame(f)
	case *feature.ApprovalFrame:
		h.approvalFrame(f)
	case *feature.PickFrame:
		h.pickFrame(f)
	case *feature.VoiceFrame:
		h.voice = f.Phase
		h.setMicMark(f.Phase)
		if h.player != nil {
			h.player.Gate().Voice(f.Phase)
		}
		if !levelPhase(f.Phase) {
			h.quietLevels()
		}
	case *feature.LevelFrame:
		// a level outside listening or speaking is stale: the phase that
		// carried it has ended
		if levelPhase(h.voice) && h.level != *f {
			h.level = *f
			h.kick()
		}
	}
}

// levelPhase reports a voice phase the daemon sends levels in.
func levelPhase(phase string) bool {
	return phase == feature.VoiceListening || phase == feature.VoiceSpeaking
}

// quietLevels drops the held levels when their phase or the link ends.
func (h *host) quietLevels() {
	if h.level != (feature.LevelFrame{}) {
		h.level = feature.LevelFrame{}
		h.kick()
	}
}

// --- engine ---

func (h *host) setState(id, reason string) {
	if id == "sleeping" {
		h.tearStrand(tearAsleep, feature.SkipHidden)
	}
	h.state = id
	h.eng.SetState(id, reason)
	h.led.Add(Entry{Ev: EvState, State: id, Reason: reason})
	h.kick()
}

func (h *host) play(id string) {
	h.eng.Play(id)
	h.led.Add(Entry{Ev: EvGesture, Gesture: id})
	h.sound("gesture-" + id)
	h.kick()
}

func (h *host) reduced() bool {
	switch h.motionF.Reduce {
	case "on":
		return true
	case "off":
		return false
	}
	return h.desktopReduce
}

func (h *host) input() motion.Input {
	in := motion.Input{Reduced: h.reduced(), BatterySaver: h.lab.batterySaver(h.motionF.BatterySaver), Mic: h.level.Mic, Speech: h.level.Speech}
	if h.pointer != nil && h.haveOut {
		ext := h.extent()
		c := spec.Point{X: float64(h.origin.X) + float64(ext.X)/2, Y: float64(h.origin.Y) + float64(ext.Y)/2}
		in.Pointer = &spec.Point{X: h.pointer.X - c.X, Y: h.pointer.Y - c.Y}
	}
	if h.grip.dragging {
		in.Drag = &motion.Drag{Velocity: h.grip.vel}
	}
	return in
}

// kick steps at once when nothing is in flight to pick the event up.
func (h *host) kick() {
	if h.shown && h.be != nil && h.clk.idle() {
		h.step(time.Now())
	}
}

func (h *host) step(now time.Time) {
	dt := time.Duration(float64(h.clk.dt(now)) * h.lab.TimeScale)
	in := h.input()
	tick := h.eng.Step(dt, in)
	h.pose = tick.Pose
	moved := h.advanceFlight(now)
	e := Entry{Ev: EvStep, DT: new(dt.Seconds()), Input: in, Commit: new(tick.Redraw), Pose: tick.Pose}
	prev := h.strandF.Phase
	st, stepped := h.stepStrand(dt, in, &e)
	// the thread holds the motion tier while it is out, whatever the
	// character's own tier
	running := stepped && !st.Stop
	fps, next := tick.FPS, tick.Next
	if running {
		fps = max(fps, st.FPS)
		if next <= 0 || (st.Next > 0 && st.Next < next) {
			next = st.Next
		}
	}
	e.FPS = fps
	h.led.Add(e)
	if stepped {
		h.strandStepped(prev, st)
	}
	// a fading chip or highlight steps at the motion tier until it is there
	fading := h.stepFaders(dt)
	if fading {
		if motion := time.Second / 60; next <= 0 || next > motion {
			next = motion
		}
	}
	stop := tick.Stop && h.fl == nil && !h.grip.dragging && !running && !fading
	redraw := tick.Redraw || moved || h.dirty || h.grip.dragging
	committed := false
	if redraw {
		committed = h.commit(!stop, fps)
	}
	if h.strandDirty {
		if h.commitStrand(!stop && !committed, fps) && !stop {
			committed = true
		}
		// a strand frame the compositor could not take yet keeps the
		// clock running until it can
		stop = stop && !h.strandDirty
	}
	h.commitOverlays()
	h.reportOverlay()
	if h.fl != nil || h.grip.dragging {
		next = 0
	}
	if wait := h.clk.stepped(now, stop, committed, next); wait > 0 {
		h.armTimer(wait)
	}
	if stop {
		h.led.Add(Entry{Ev: EvStop})
	}
}

func (h *host) frameDone(now time.Time) {
	step, wait := h.clk.frameDone(now)
	switch {
	case step:
		h.step(now)
	case wait > 0:
		h.armTimer(wait)
	}
}

func (h *host) armTimer(d time.Duration) {
	h.stopTimer()
	gen := h.timerGen
	h.timer = time.AfterFunc(d, func() {
		h.be.post(func() {
			if gen == h.timerGen && h.clk.timerFired() {
				h.step(time.Now())
			}
		})
	})
}

func (h *host) stopTimer() {
	h.timerGen++
	if h.timer != nil {
		h.timer.Stop()
		h.timer = nil
	}
}

// --- drawing ---

func (h *host) sizeFactor() float64 {
	if f, ok := h.sp.Size(h.size); ok {
		return f
	}
	return 1
}

// extent is the sprite size at the character's own scale.
func (h *host) extent() image.Point {
	if h.char == nil {
		return image.Point{}
	}
	return h.char.Extent(h.sizeFactor())
}

// sprite is where the character draws and at what size factor: its place,
// or the pose a region pen gives it while the pen rides the strand's tip.
func (h *host) sprite() (origin image.Point, sf float64, ext image.Point) {
	sf = h.sizeFactor()
	if h.char == nil {
		return h.origin, sf, image.Point{}
	}
	if h.charOver == nil {
		return h.origin, sf, h.char.Extent(sf)
	}
	sf *= h.charOver.Scale
	ext = h.char.Extent(sf)
	c := h.charOver.Pos
	at := image.Pt(int(math.Round(c.X-float64(ext.X)/2)), int(math.Round(c.Y-float64(ext.Y)/2)))
	return h.out.Rect.Min.Add(at), sf, ext
}

func (h *host) margin() int { return h.sp.Placement.CornerMarginPx }

func (h *host) commit(arm bool, fps int) bool {
	if !h.haveOut || h.char == nil {
		return false
	}
	origin, sf, ext := h.sprite()
	if ext.X <= 0 || ext.Y <= 0 {
		return false
	}
	dpr := h.be.scale(h.out.ID)
	phys := image.Pt(int(math.Ceil(float64(ext.X)*dpr)), int(math.Ceil(float64(ext.Y)*dpr)))
	if h.img == nil || h.img.Bounds().Size() != phys {
		h.img = image.NewNRGBA(image.Rectangle{Max: phys})
	}
	painted := h.char.Draw(h.img, h.pose, h.look, sf, dpr)
	mark := h.paintMicMark(ext, dpr)
	input := image.Rect(
		int(math.Floor(float64(painted.Min.X)/dpr)), int(math.Floor(float64(painted.Min.Y)/dpr)),
		int(math.Ceil(float64(painted.Max.X)/dpr)), int(math.Ceil(float64(painted.Max.Y)/dpr)),
	).Intersect(image.Rectangle{Max: ext})
	local := origin.Sub(h.out.Rect.Min)
	at, committed, err := h.be.present(presentation{output: h.out.ID, origin: local, size: ext, img: h.img, input: input, arm: arm, grab: h.grip.pressed})
	if err != nil {
		h.led.Add(Entry{Ev: EvCommit, Reason: "error", Output: h.out.ID})
		return false
	}
	if !committed {
		return false
	}
	h.dirty = false
	e := Entry{Ev: EvCommit, Rect: rectOf(image.Rectangle{Min: at, Max: at.Add(ext)}), Pose: h.pose, Output: h.out.ID, Scale: dpr, FPS: fps, State: h.state, Mark: mark}
	if o := h.charOver; o != nil {
		e.Char = &CharInfo{X: o.Pos.X, Y: o.Pos.Y, Scale: o.Scale}
	}
	h.led.Add(e)
	return true
}

func (h *host) setDesktop(d palette.Desktop) {
	h.look = desktoplook.FromDesktop(d)
	h.desktopReduce = d.ReduceMotion
	h.dirty = true
	h.commitMarkOnly()
	h.strandDirty = h.strandDirty || h.strandShown
	h.hl.dirty = h.hl.dirty || h.hl.shown
	h.chip.dirty = h.chip.dirty || h.chip.shown
	h.card.dirty = h.card.dirty || h.card.shown
	h.pill.f.dirty = h.pill.f.dirty || h.pill.f.shown
	h.handoff.f.dirty = h.handoff.f.dirty || h.handoff.f.shown
	h.notice.f.dirty = h.notice.f.dirty || h.notice.f.shown
	h.wakeStrand()
	h.kick()
	if h.hands != nil {
		for _, g := range h.glow.outs {
			g.dirty = true
		}
		h.syncGlow(time.Now())
	}
}

func (h *host) loadCharacter(id string) {
	if id == h.charID {
		return
	}
	if id == profile.ShippedID {
		loaded := profile.Default()
		h.setCharacter(id, loaded.Profile, loaded.Files)
		return
	}
	h.withLink(func(l Link) {
		go func() {
			ctx, cancel := context.WithTimeout(h.ctx, callTimeout)
			defer cancel()
			a, err := l.Assets(ctx, id)
			var loaded profile.Loaded
			if err == nil {
				loaded, err = loadAssets(id, a)
			}
			h.be.post(func() {
				if err != nil {
					h.led.Add(Entry{Ev: EvFrame, Kind: feature.KindCharacter, Reason: "assets-failed"})
					return
				}
				h.setCharacter(id, loaded.Profile, a.Files)
			})
		}()
	})
}

// loadAssets validates a character's bytes again here: the surface draws
// only what the profile loader accepts.
func loadAssets(id string, a protocol.OniCharacterAssets) (profile.Loaded, error) {
	if a.ID != id {
		return profile.Loaded{}, fmt.Errorf("assets for %q answered %q", id, a.ID)
	}
	fsys := fstest.MapFS{}
	for name, data := range a.Files {
		fsys[id+"/"+name] = &fstest.MapFile{Data: data, Mode: 0o644}
	}
	loaded, _ := profile.Load(fsys, id)
	if !loaded.Valid {
		return profile.Loaded{}, fmt.Errorf("character %q is not valid", id)
	}
	return loaded, nil
}

func (h *host) setCharacter(id string, p profile.Profile, files map[string][]byte) {
	c, err := render.Load(p, files, h.sp)
	if err != nil {
		h.led.Add(Entry{Ev: EvFrame, Kind: feature.KindCharacter, Reason: "render-failed"})
		return
	}
	first := h.char == nil
	h.char, h.charID = c, id
	if !first {
		h.eng.Reset()
		h.led.Add(Entry{Ev: EvReset})
	}
	h.dirty = true
	h.newPlayer(p)
	h.reposition(false)
}

// --- placement ---

func (h *host) applyPlacement(f *feature.PlacementFrame) {
	h.size = f.Size
	outs := make(map[string]placement.Saved, len(f.Outputs))
	for _, o := range f.Outputs {
		outs[o.ID] = placement.Saved{FX: o.FX, FY: o.FY}
	}
	h.place = placement.Place{Corner: f.Corner, Outputs: outs, Active: f.Active}
	h.dirty = true
	if !h.grip.pressed {
		// the first placement is where the character already belongs, not
		// somewhere it travels to
		h.reposition(h.placementSeen)
	}
	h.placementSeen = true
}

// reposition puts the character where the placement says on the output it
// belongs on, or, while perched, by the perch's corner. It flies there,
// unless motion is reduced or it was never placed: from where it is, or,
// when its output shrank or went away, from the nearest point of the output
// it now belongs on, so the flight is seen.
func (h *host) reposition(fly bool) {
	if h.be == nil || h.char == nil {
		return
	}
	o, ok := placement.Choose(h.be.outputs(), h.place, "")
	p := h.perch
	if p != nil {
		if po, on := h.perchOutput(p); on {
			o, ok = po, true
		} else {
			p = nil
		}
	}
	if !ok {
		h.haveOut = false
		h.updateVisible()
		return
	}
	ext := h.extent()
	var saved *placement.Saved
	if s, ok := h.place.Outputs[o.ID]; ok && h.place.Corner == "" {
		saved = &s
	}
	target := placement.Resolve(o, ext, h.place.Corner, saved, h.margin())
	if p != nil {
		target = h.seat(o, p)
	}
	// a new output, or the same one with a new size, position or scale, is a
	// display change: record it before the move it causes
	if !h.haveOut || o.ID != h.out.ID || o.Rect != h.out.Rect || o.Scale != h.out.Scale {
		// the thread's target was placed on the output as it was
		if h.out.ID != "" && (o.ID != h.out.ID || o.Rect != h.out.Rect) {
			h.tearStrand(tearOutput, feature.SkipOffOutput)
		}
		h.led.Add(Entry{Ev: EvOutput, Output: o.ID, Scale: o.Scale, Rect: rectOf(image.Rectangle{Min: target.Sub(o.Rect.Min), Max: target.Sub(o.Rect.Min).Add(ext)})})
	}
	from := target
	if h.placed {
		from = placement.Clamp(o, h.origin, ext, 0)
	}
	h.placed = true
	if from == target || !fly || h.reduced() {
		h.fl = nil
		h.origin = target
	} else {
		centre := image.Pt(o.Rect.Min.X+o.Rect.Dx()/2, o.Rect.Min.Y+o.Rect.Dy()/2)
		dur, path := placement.Flight(from, target, centre, h.sp.Flight)
		h.origin = from
		h.fl = nil
		if dur > 0 {
			h.fl = &flight{start: time.Now(), dur: dur, path: path}
			h.sound("flight-skate")
		} else {
			h.origin = target
		}
	}
	h.out, h.haveOut = o, true
	h.dirty = true
	h.updateVisible()
	if h.fl == nil {
		h.flightDone()
	}
	h.kick()
}

func (h *host) advanceFlight(now time.Time) bool {
	if h.fl == nil {
		return false
	}
	t := float64(now.Sub(h.fl.start)) * h.lab.TimeScale / float64(h.fl.dur)
	p := h.fl.path(t)
	if t >= 1 {
		h.fl = nil
		h.sound("flight-land")
		h.flightDone()
	}
	moved := p != h.origin
	h.origin = p
	return moved
}

// --- visibility ---

func (h *host) updateVisible() {
	covered := h.haveOut && h.be != nil && h.be.covered(h.out.ID)
	show, reason := visibility(h.daemonHidden, h.motionF.Courtesy, covered, h.haveOut && h.char != nil)
	switch {
	case show && !h.shown:
		h.shown = true
		h.led.Add(Entry{Ev: EvShow, Output: h.out.ID})
		h.clk.halt()
		h.dirty = true
		h.kick()
	case !show && h.shown:
		skipped := feature.SkipHidden
		if reason == hideCourtesy {
			skipped = feature.SkipCovered
		}
		h.tearStrand(reason, skipped)
		h.shown = false
		h.be.hide()
		h.clk.halt()
		h.stopTimer()
		h.grip.release()
		h.pointer = nil
		h.led.Add(Entry{Ev: EvHide, Reason: reason, Output: h.out.ID})
		h.hideCard()
		h.clearNotice(reason)
		h.hideHands()
		// nothing perched survives a hide: a courtesy hide is the
		// presenter's own, so it tells the daemon; the daemon's hide and a
		// lost output detach there already
		perched := h.perch != nil
		switch reason {
		case hideCourtesy:
			h.tearPerch(detachHidden)
			if l := h.link; perched && l != nil {
				h.call(func(ctx context.Context) error { return l.Detach(ctx, detachHidden) })
			}
		case hideOutput:
			h.tearPerch(detachOutput)
		default:
			h.tearPerch(detachHidden)
		}
		if perched && h.haveOut {
			h.reposition(false)
		}
	}
	h.syncMarkOnly()
}

// --- sink ---

func (h *host) outputsChanged() {
	if h.grip.pressed {
		for _, o := range h.be.outputs() {
			if o.ID == h.out.ID {
				return
			}
		}
		h.grip.release()
	}
	h.reposition(true)
	if h.pickArmed {
		h.syncPick(pickOutputs)
	}
	if h.hands != nil {
		h.syncGlow(time.Now())
	}
}

func (h *host) scaleChanged() {
	h.dirty = true
	h.commitMarkOnly()
	if h.hands != nil {
		h.syncGlow(time.Now())
	}
	if h.clk.armed {
		return
	}
	h.kick()
}

func (h *host) configured() {
	h.clk.halt()
	h.stopTimer()
	h.dirty = true
	// a new sprite surface took the strand's and the overlays with it
	h.strandDirty = h.strandDirty || h.strandShown
	h.strandShown = false
	h.lostOverlays()
	h.commitMarkOnly()
	h.kick()
}

func (h *host) surfaceClosed() {
	h.clk.halt()
	h.stopTimer()
	h.haveOut = false
	h.reposition(false)
}

func (h *host) coveredChanged() { h.updateVisible() }

func (h *host) pointerMotion(p spec.Point) {
	h.pointer = &p
	if h.grip.pressed {
		started, dragging, pos := h.grip.move(p, time.Now())
		if started {
			h.fl = nil
			h.led.Add(Entry{Ev: EvDrag, Output: h.out.ID})
			h.interact("drag")
			// pulling the character off its window takes the perch down
			h.detach(detachDragged)
		}
		if dragging {
			h.origin = placement.Clamp(h.out, pos, h.extent(), h.margin())
			h.hover(p)
		}
	}
	h.kick()
}

func (h *host) pointerLeave() {
	if !h.grip.pressed {
		h.pointer = nil
		h.kick()
	}
}

func (h *host) button(pressed bool) {
	if pressed {
		if h.pointer != nil && h.haveOut {
			h.grip.press(h.origin, *h.pointer, time.Now())
		}
		return
	}
	if !h.grip.pressed {
		return
	}
	if h.grip.release() {
		h.drop()
	} else {
		h.click()
	}
	h.kick()
}

func (h *host) drop() {
	fr := placement.Fraction(h.out, h.origin, h.extent())
	h.hideHighlight(highlightDrop)
	h.hoverNext = nil
	h.led.Add(Entry{Ev: EvDrop, Output: h.out.ID, FX: new(fr.FX), FY: new(fr.FY)})
	h.play("settle")
	h.dropOn(fr)
}

func (h *host) click() {
	switch {
	case h.link == nil:
		h.interact("wake")
	case h.state == "sleeping":
		h.interact("wake")
	default:
		h.play("poke")
		h.interact("poke")
	}
}

// --- strand ---

// Teardown reasons beyond the hide reasons, and why a retract happened.
const (
	tearOff       = "off"
	tearAsleep    = "asleep"
	tearDormant   = "dormant"
	tearOutput    = hideOutput
	tearHome      = "home"
	retractDaemon = "daemon"
	retractSkip   = "superseded"
)

type bubbleReq struct {
	seq  uint64
	text string
}

// strandHost is the host's half of the thread: the shared engine, the
// target it is on, the frame it last gave and the arrival owed the daemon.
type strandHost struct {
	strd *strand.Engine
	// strandRun: the engine needs steps; it stops at home and at the end
	// of a hold, and wakes on a target, a retract or a motion change
	strandRun   bool
	strandF     strand.Frame
	strandDirty bool // strandF is not on screen yet
	strandShown bool // the strand surface holds a buffer
	strandImg   *image.NRGBA
	target      *strand.Target
	topSeq      uint64 // the newest seq the engine took
	awaiting    uint64 // the seq whose arrival the daemon is owed, 0 none
	landed      uint64 // the seq whose tip last landed
	charOver    *strand.CharPose
	bubble      *bubbleReq
}

// resolveTarget turns the daemon's fractions into output-local px: of the
// perched window, else of the character's output. A perch on another
// output cannot be reached from this one.
func (h *host) resolveTarget(f *feature.StrandTargetFrame) (t strand.Target, skip string) {
	ref := strand.Rect{W: float64(h.out.Rect.Dx()), H: float64(h.out.Rect.Dy())}
	if p := h.perch; p != nil {
		ref = p.box
		frame := p.box
		t.Frame = &frame
		if p.output != "" && p.output != h.out.ID {
			skip = feature.SkipOffOutput
		}
	}
	t.Seq, t.Quiet = f.Seq, f.Quiet
	t.Box = strand.Rect{X: ref.X + f.X*ref.W, Y: ref.Y + f.Y*ref.H, W: f.W * ref.W, H: f.H * ref.H}
	switch f.Mode {
	case feature.StrandModeLasso:
		t.Mode = strand.ModeLasso
	case feature.StrandModeRegion:
		t.Mode = strand.ModeRegion
	case feature.StrandModePerch:
		t.Mode = strand.ModePerch
		if p := h.perch; p != nil {
			t.Corner = p.corner
		} else if skip == "" {
			skip = feature.SkipHidden
		}
	}
	if r := f.Region; r != nil {
		t.Region = &strand.Rect{X: r.X, Y: r.Y, W: r.W, H: r.H}
	}
	return t, skip
}

func (h *host) strandTarget(f *feature.StrandTargetFrame) {
	if f.Seq <= h.topSeq {
		return
	}
	t, skip := h.resolveTarget(f)
	if skip == "" && !h.shown {
		covered := h.haveOut && h.be != nil && h.be.covered(h.out.ID)
		skip = feature.SkipHidden
		if _, reason := visibility(h.daemonHidden, h.motionF.Courtesy, covered, h.haveOut && h.char != nil); reason == hideCourtesy {
			skip = feature.SkipCovered
		}
	}
	if skip != "" {
		// never handed to the engine, so recorded as a skip, not a target
		h.led.Add(Entry{Ev: EvStrandSkip, Seq: t.Seq, Skipped: skip, Target: &t})
		h.awaiting = 0
		if h.target != nil && h.strd.Out() {
			h.strd.Retract(h.target.Seq)
			h.led.Add(Entry{Ev: EvStrandRetract, Seq: h.target.Seq, Reason: retractSkip})
			h.bubble = nil
			h.strandRun = true
			h.kick()
		}
		h.report(t.Seq, skip)
		return
	}
	h.topSeq = t.Seq
	h.awaiting = t.Seq
	if t.Mode == strand.ModePerch && h.fl != nil {
		// the thread curls once the character stands by the corner
		h.heldPerch = &t
		return
	}
	h.giveTarget(t)
	h.kick()
}

// giveTarget hands t to the engine. It may run inside a step, so it does
// not step; a caller outside one kicks.
func (h *host) giveTarget(t strand.Target) {
	h.strd.Target(t)
	h.led.Add(Entry{Ev: EvStrandTarget, Seq: t.Seq, Target: &t})
	h.target = &t
	if t.Mode == strand.ModePerch {
		pt := t
		h.perchT = &pt
	}
	h.bubble = nil
	h.strandRun = true
}

func (h *host) strandRetract(seq uint64) {
	if h.perchT != nil && h.perchT.Seq == seq {
		h.perchT, h.reperch = nil, false
	}
	if t := h.heldPerch; t != nil && t.Seq == seq {
		h.heldPerch = nil
		if h.awaiting == seq {
			h.awaiting = 0
		}
		return
	}
	if h.target == nil || h.target.Seq != seq {
		return
	}
	h.strd.Retract(seq)
	h.led.Add(Entry{Ev: EvStrandRetract, Seq: seq, Reason: retractDaemon})
	if h.awaiting == seq {
		h.awaiting = 0
	}
	h.bubble = nil
	h.strandRun = true
	h.kick()
}

// wakeStrand gives a thread that is out a step after a change it must see
// (reduced motion, the look).
func (h *host) wakeStrand() {
	if h.strd.Out() {
		h.strandRun = true
	}
}

// anchor is where the thread leaves the drawn character, output-local.
func (h *host) anchor() spec.Point {
	o, sf, _ := h.sprite()
	a := h.char.Anchor(h.pose, sf)
	l := o.Sub(h.out.Rect.Min)
	return spec.Point{X: float64(l.X) + a.X, Y: float64(l.Y) + a.Y}
}

func (h *host) strandEnv(in motion.Input) strand.Env {
	w, ht := float64(h.out.Rect.Dx()), float64(h.out.Rect.Dy())
	env := strand.Env{Centre: spec.Point{X: w / 2, Y: ht / 2}, Screen: strand.Rect{W: w, H: ht}, Reduced: in.Reduced, BatterySaver: in.BatterySaver}
	if h.char != nil {
		env.Anchor = h.anchor()
	}
	return env
}

// stepStrand steps the thread in the host's clock and records the call on
// the step's ledger line, as the strand replay reads it.
func (h *host) stepStrand(dt time.Duration, in motion.Input, e *Entry) (strand.Tick, bool) {
	if !h.strandRun || !h.haveOut || h.char == nil {
		return strand.Tick{}, false
	}
	env := h.strandEnv(in)
	tk := h.strd.Step(dt, env)
	e.StrandEnv, e.StrandCommit = &env, new(tk.Redraw)
	if tk.Redraw {
		e.Strand = strandInfo(tk.Frame)
		h.strandDirty = true
	}
	h.strandF = tk.Frame
	h.carry(tk.Frame.Char)
	if tk.Stop {
		h.strandRun = false
	}
	return tk, true
}

// carry moves the character with a region pen, and back when it lets go.
func (h *host) carry(c *strand.CharPose) {
	if (c == nil) == (h.charOver == nil) && (c == nil || *c == *h.charOver) {
		return
	}
	h.charOver = nil
	if c != nil {
		cp := *c
		h.charOver = &cp
	}
	h.dirty = true
}

// strandStepped acts on what a step did: sounds, the arrival owed, and
// forgetting a target once the thread is home.
func (h *host) strandStepped(prev strand.Phase, tk strand.Tick) {
	f := tk.Frame
	h.strandSounds(prev, f)
	if f.Arrived {
		h.strandArrived(f)
	}
	if !h.strd.Out() {
		h.target, h.bubble = nil, nil
		h.strandHome()
	}
}

// strandArrived reports the landing (or the engine's skip) exactly once
// for the seq the daemon waits on.
func (h *host) strandArrived(f strand.Frame) {
	if f.Skipped != "" {
		h.led.Add(Entry{Ev: EvStrandSkip, Seq: f.Seq, Skipped: f.Skipped})
	} else {
		h.led.Add(Entry{Ev: EvStrandArrive, Seq: f.Seq})
		h.landed = f.Seq
		h.placeBubble()
	}
	if f.Seq != 0 && f.Seq == h.awaiting {
		h.awaiting = 0
		h.report(f.Seq, f.Skipped)
	}
}

// report tells the daemon about seq, on the open link only: a lost link
// already cancelled it there.
func (h *host) report(seq uint64, skipped string) {
	l := h.link
	if l == nil || seq == 0 {
		return
	}
	h.call(func(ctx context.Context) error { return l.StrandArrived(ctx, seq, skipped) })
}

// commitStrand puts strandF on the strand surface, or unmaps it once the
// thread is home. It reports whether it committed.
func (h *host) commitStrand(arm bool, fps int) bool {
	f := h.strandF
	if len(f.Strokes) == 0 || !(f.Bbox.W > 0) || !(f.Bbox.H > 0) {
		h.strandDirty = false
		h.hideStrandSurface(tearHome)
		return false
	}
	if !h.haveOut {
		return false
	}
	rect := image.Rect(
		int(math.Floor(f.Bbox.X)), int(math.Floor(f.Bbox.Y)),
		int(math.Ceil(f.Bbox.X+f.Bbox.W)), int(math.Ceil(f.Bbox.Y+f.Bbox.H)),
	)
	dpr := h.be.strandScale(h.out.ID, h.sp.Strand.Stroke.DPRCap)
	phys := image.Pt(int(math.Ceil(float64(rect.Dx())*dpr)), int(math.Ceil(float64(rect.Dy())*dpr)))
	if h.strandImg == nil || h.strandImg.Bounds().Size() != phys {
		h.strandImg = image.NewNRGBA(image.Rectangle{Max: phys})
	}
	render.PaintStrand(h.strandImg, rect.Min, f, h.look, dpr)
	committed, err := h.be.presentStrand(strandPresentation{output: h.out.ID, rect: rect, img: h.strandImg, arm: arm})
	if err != nil {
		h.strandDirty = false
		h.led.Add(Entry{Ev: EvCommit, Part: PartStrand, Reason: "error", Output: h.out.ID})
		return false
	}
	if !committed {
		return false
	}
	// the curl a perch keeps on its window's corner points at no control:
	// its padded bounds mask it, and hold still while it breathes
	if t := h.target; t != nil && t.Seq == f.Seq && t.Mode == strand.ModePerch {
		h.strandMask.cover(rect)
	} else {
		h.strandMask.update(f, h.sp.Strand.Stroke.OutlinePx, strandMaskMax, h.strandShown)
	}
	h.strandDirty, h.strandShown = false, true
	h.led.Add(Entry{Ev: EvCommit, Part: PartStrand, Rect: rectOf(rect), Strand: strandInfo(f), Output: h.out.ID, Scale: dpr, FPS: fps})
	return true
}

// hideStrandSurface unmaps the strand surface and drops its buffer, so a
// thread at home holds no memory.
func (h *host) hideStrandSurface(reason string) {
	h.strandImg = nil
	if !h.strandShown {
		return
	}
	h.be.hideStrand()
	h.strandShown = false
	h.led.Add(Entry{Ev: EvHide, Part: PartStrand, Reason: reason, Output: h.out.ID})
}

// tearStrand takes the thread down at once when nothing can show it: the
// arrival owed is reported as skipped where the link lives (skipped empty
// reports nothing), and the engine lands home in one recorded step so no
// later step animates it.
func (h *host) tearStrand(reason, skipped string) {
	if seq := h.awaiting; seq != 0 {
		h.awaiting = 0
		if skipped != "" && h.link != nil {
			h.led.Add(Entry{Ev: EvStrandSkip, Seq: seq, Skipped: skipped})
			h.report(seq, skipped)
		}
	}
	if h.strd.Out() {
		if h.target != nil {
			h.strd.Retract(h.target.Seq)
			h.led.Add(Entry{Ev: EvStrandRetract, Seq: h.target.Seq, Reason: reason})
		}
		h.settleStrand()
	}
	h.target, h.bubble = nil, nil
	h.heldPerch, h.perchT, h.reperch = nil, nil, false
	h.carry(nil)
	h.strandRun, h.strandDirty = false, false
	h.hideStrandSurface(reason)
}

// settleStrand is a zero-length step of both engines with the strand's
// motion reduced, which lands a retracted thread home at once. It is
// recorded as a step so both ledger replays make the same calls.
func (h *host) settleStrand() {
	in := h.input()
	tick := h.eng.Step(0, in)
	h.pose = tick.Pose
	if tick.Redraw {
		h.dirty = true
	}
	env := h.strandEnv(in)
	env.Reduced = true
	st := h.strd.Step(0, env)
	h.strandF = st.Frame
	e := Entry{Ev: EvStep, DT: new(0.0), Input: in, Commit: new(tick.Redraw), Pose: tick.Pose, FPS: tick.FPS,
		StrandEnv: &env, StrandCommit: new(st.Redraw)}
	if st.Redraw {
		e.Strand = strandInfo(st.Frame)
	}
	h.led.Add(e)
}

func (h *host) bubbleFrame(f *feature.BubbleFrame) {
	if h.target == nil || !h.strd.Out() {
		h.led.Add(Entry{Ev: EvBubble, Seq: f.Seq, Reason: "no-strand"})
		return
	}
	h.bubble = &bubbleReq{seq: f.Seq, text: f.Text}
	if h.landed == h.target.Seq {
		h.placeBubble()
	}
}

// placeBubble records where the bubble's card goes by the landed tip: never
// over the target (for a ring, over the ring round it).
func (h *host) placeBubble() {
	b, t := h.bubble, h.target
	if b == nil || t == nil {
		return
	}
	h.bubble = nil
	against := t.Box
	if t.Mode != strand.ModePoint {
		g := h.sp.Strand.Lasso.GapPx
		against = strand.Rect{X: against.X - g, Y: against.Y - g, W: against.W + 2*g, H: against.H + 2*g}
	}
	screen := strand.Rect{W: float64(h.out.Rect.Dx()), H: float64(h.out.Rect.Dy())}
	r := strand.BubbleRect(against, h.strandF.Tip, h.strandF.Approach(), bubbleSize(b.text, h.sp.Strand.Bubble), screen, h.sp)
	h.led.Add(Entry{Ev: EvBubble, Seq: b.seq, Box: &r, Target: t})
	if !t.Quiet {
		h.sound("bubble-open")
	}
}

// bubbleSize estimates the card for text until the surface draws text: an
// average advance per rune, wrapped at the card's width, one em of padding.
func bubbleSize(text string, b spec.Bubble) spec.Point {
	pad := b.TextPx
	inner := b.MaxWidthPx - 2*pad
	run := float64(utf8.RuneCountInString(text)) * 0.55 * b.TextPx
	lines := max(1, math.Ceil(run/inner))
	return spec.Point{X: math.Min(run, inner) + 2*pad, Y: lines*b.TextPx*1.4 + b.KickerPx + 2*pad}
}
