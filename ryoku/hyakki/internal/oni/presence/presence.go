// Package presence is the daemon's side of Oni on screen: the state the
// sessions put Oni in, the gestures that mark a turn ending, the sleep after
// a quiet spell and the wake that follows, where Oni stands, and the strand
// targets whose landing the daemon waits on. It decides what Oni shows and
// never reads a character, so a character cannot change it; the daemon hands
// it the character frame to pass on.
package presence

import (
	"slices"
	"sync"
	"time"

	"ryoku-rashin/internal/oni/feature"
	"ryoku-rashin/internal/oni/spec"
	"ryoku-rashin/internal/protocol"
)

// Clock is the time the service reads and the timer it arms; tests pass a
// fake one.
type Clock interface {
	Now() time.Time
	AfterFunc(d time.Duration, f func()) Timer
}

// Timer is an armed AfterFunc.
type Timer interface {
	Stop() bool
}

type systemClock struct{}

func (systemClock) Now() time.Time { return time.Now() }

func (systemClock) AfterFunc(d time.Duration, f func()) Timer { return time.AfterFunc(d, f) }

// SystemClock is the wall clock.
func SystemClock() Clock { return systemClock{} }

// Activity is one session's state as the daemon last reported it. Tool is
// whether its running turn is in a tool rather than the model.
type Activity struct {
	Session string
	State   protocol.SessionState
	Tool    bool
}

// Reasons a state frame carries: oni-off tells a renderer to exit, hidden to
// unmap.
const (
	ReasonOff    = protocol.OniOpenOff
	ReasonHidden = "hidden"
)

// SleepAfter is how long Oni stays awake with nothing happening, the spec's
// sleep.idleMs. The spec is embedded and its loader tested, so it loads; the
// number written here is that same value.
var SleepAfter = func() time.Duration {
	if s, err := spec.Load(); err == nil && s.Sleep.IdleMs > 0 {
		return time.Duration(s.Sleep.IdleMs * float64(time.Millisecond))
	}
	return 90 * time.Second
}()

// LevelEvery is the shortest gap between two level frames, one over the
// spec's voice.levels.maxHz; voicePhases are the spec's voice phases, the
// state each derives and whether it shows the mic mark. Without the spec no
// phase derives a state and a level goes out at most once a second.
var LevelEvery, voicePhases = func() (time.Duration, map[string]spec.VoicePhase) {
	s, err := spec.Load()
	if err != nil || s.Voice.Levels.MaxHz <= 0 {
		return time.Second, map[string]spec.VoicePhase{}
	}
	return time.Duration(float64(time.Second) / s.Voice.Levels.MaxHz), s.Voice.Phases
}()

// Service holds Oni's presence while a renderer or observer has the oni
// event open. emit receives every frame in order, called with the service's
// lock held, so it must not call back into the service.
type Service struct {
	mu    sync.Mutex
	clock Clock
	emit  func(feature.Payload)

	settings  protocol.OniSettings
	character *feature.CharacterFrame
	sessions  map[string]Activity
	failed    bool
	asleep    bool
	saver     bool
	// voice is the microphone's phase as the voice pipeline last reported
	// it through Voice; idle until then
	voice feature.VoiceFrame
	// levelAt is when the last level frame went out
	levelAt time.Time
	// guide is the running guided walkthrough's frame as the daemon last
	// sent it through Guide; nil while no guide runs
	guide *feature.GuideFrame
	// hands is the running Hands task's frame as the daemon last sent it
	// through Hands, and handsCard the handoff card it waits on; nil while
	// no task runs
	hands     *feature.HandsFrame
	handsCard *feature.ConsentFrame
	// App-only approval disclosures, never included in general snapshots.
	appApprovals [3]appApproval

	timer Timer
	armed uint64

	strand strand
	perch  perch

	gestureSeq uint64
	placeSeq   uint64
	place      Place
	file       string

	// what the frames sent so far said
	last struct {
		on     bool
		state  feature.StateFrame
		motion feature.MotionFrame
		size   string
	}
	closed bool
}

// New starts a service with Oni off until Settings says otherwise, standing
// where it was last placed.
func New(clock Clock, emit func(feature.Payload)) *Service {
	s := &Service{clock: clock, emit: emit, sessions: map[string]Activity{}, file: PlacementFile()}
	s.place = loadPlace(s.file)
	s.settings = protocol.DefaultOniSettings()
	s.voice = feature.VoiceFrame{Phase: VoiceIdle}
	s.last.state = *s.stateFrame()
	return s
}

// Settings applies Oni's effective settings (protocol.EffectiveOni). Turning
// Oni off cancels the pending strand target and drops the running guide's
// frame; turning it off or hiding it lets go of the perched window.
func (s *Service) Settings(o protocol.OniSettings) {
	s.mu.Lock()
	defer s.unlock()
	if s.closed {
		return
	}
	switch {
	case !o.Master:
		s.detach(feature.PerchOniOff)
	case o.Hidden:
		s.detach(feature.PerchHidden)
	}
	wasShown := s.shown()
	s.settings = o
	if s.shown() && !wasShown {
		s.asleep = false
	}
	if !s.settings.Master {
		s.strand.out = 0
		s.cancelPending(CancelOff)
		s.guide = nil
		s.hands, s.handsCard = nil, nil
		s.appApprovals = [3]appApproval{}
	}
	s.publish()
	s.rearm()
}

// Character sets the frame the daemon built for the character in use.
func (s *Service) Character(c feature.CharacterFrame) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return
	}
	same := s.character != nil && s.character.ID == c.ID && s.character.Name == c.Name && slices.Equal(s.character.Fallbacks, c.Fallbacks)
	frame := c
	frame.Fallbacks = slices.Clone(c.Fallbacks)
	s.character = &frame
	if !same && s.last.on {
		s.send(s.characterFrame())
	}
}

// Activity records one session's state. A turn that ends plays done, one
// that fails plays error, and any of it wakes a sleeping Oni.
func (s *Service) Activity(a Activity) {
	s.mu.Lock()
	defer s.unlock()
	if s.closed {
		return
	}
	prev, running := s.sessions[a.Session]
	if running && prev == a {
		return
	}
	gesture := ""
	switch a.State {
	case protocol.StateWorking, protocol.StateWaiting:
		s.sessions[a.Session] = a
		s.failed = false
	case protocol.StateFailed:
		if !running {
			return
		}
		delete(s.sessions, a.Session)
		gesture = "error"
		s.failed = true
	default:
		if !running {
			return
		}
		delete(s.sessions, a.Session)
		gesture = "done"
	}
	s.wake()
	if gesture != "" {
		s.gesture(gesture)
	}
	s.publish()
	s.rearm()
}

// Interact is a poke, wake, drag or drop the presenter reports. Each wakes
// a sleeping Oni, settles a shown error and starts the quiet spell over.
func (s *Service) Interact(kind string) {
	s.mu.Lock()
	defer s.unlock()
	if s.closed || !s.shown() {
		return
	}
	s.failed = false
	s.wake()
	s.publish()
	s.rearm()
}

// VoiceIdle is the voice phase with the microphone closed.
const VoiceIdle = feature.VoiceIdle

// Voice records the voice pipeline's phase, the one place the microphone's
// state comes from: anything but idle means the microphone is live, and the
// renderer stays silent. A frame goes out when the phase, its source or its
// text changes, before anything the phase sets off, so the renderer's sound
// gate closes first. A phase the spec maps to a state (listening,
// thinking for transcribing, speaking) wakes a sleeping Oni and shows that
// state; armed shows none but the mic mark. An invalid frame, a dictation's
// words among them, is refused and changes nothing.
func (s *Service) Voice(v feature.VoiceFrame) error {
	if _, err := feature.EncodeFrame(&v); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.unlock()
	if s.closed || v == s.voice {
		return nil
	}
	s.voice = v
	if s.last.on {
		s.send(s.voiceFrame())
	}
	if s.voiceState() != "" {
		s.wake()
	}
	s.publish()
	s.rearm()
	return nil
}

// Level passes the microphone and speech levels to the renderer while the
// voice phase is listening or speaking and Oni is shown, at most once per
// LevelEvery. A level that comes sooner, in another phase, or while Oni is
// off or hidden is dropped: no timer holds it. sent says whether it went
// out; an invalid level is refused.
func (s *Service) Level(l feature.LevelFrame) (sent bool, err error) {
	if _, err := feature.EncodeFrame(&l); err != nil {
		return false, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed || !s.last.on || !s.shown() {
		return false, nil
	}
	if s.voice.Phase != feature.VoiceListening && s.voice.Phase != feature.VoiceSpeaking {
		return false, nil
	}
	now := s.clock.Now()
	if !s.levelAt.IsZero() && now.Sub(s.levelAt) < LevelEvery {
		return false, nil
	}
	s.levelAt = now
	s.send(&l)
	return true, nil
}

// MicMark reports whether the presenter must show the persistent mic mark:
// the microphone is open between presses, for the wake phrase or open mic.
// No setting hides it.
func (s *Service) MicMark() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return voicePhases[s.voice.Phase].MicMark
}

// voiceState is the character state the voice phase derives, "" for none.
func (s *Service) voiceState() string {
	if p, ok := voicePhases[s.voice.Phase]; ok && p.State != spec.NoState {
		return p.State
	}
	return ""
}

// Snapshot is the frames a renderer starts from: state, character, motion,
// placement, the perch while one is attached, voice, the guide while one
// runs, and the Hands task and the handoff card it waits on while one runs,
// in that order, or only the state while Oni is off.
func (s *Service) Snapshot() []feature.Payload {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.snapshot()
}

// Open hands register the snapshot under the lock every frame is emitted
// under: no frame goes out between the snapshot and register's return, so a
// connection register adds sees every later frame and no earlier one. It
// must not call back into the service.
// appApprovals is true only for an authenticated app observer; the caller
// supplies that authorization. Reading a snapshot never grants answer authority.
func (s *Service) Open(appApprovals bool, register func(snapshot []feature.Payload)) {
	s.mu.Lock()
	defer s.mu.Unlock()
	snapshot := s.snapshot()
	if appApprovals {
		snapshot = append(snapshot, s.appApprovalFrames()...)
	}
	register(snapshot)
}

// Close stops the timers, cancels the pending strand target and lets go of
// the perched window; the service sends nothing more.
func (s *Service) Close() {
	s.mu.Lock()
	defer s.unlock()
	s.cancelPending(CancelClosed)
	if s.perch.frame != nil {
		s.perch.frame = nil
		s.perch.notes = append(s.perch.notes, feature.PerchPresenterLost)
	}
	s.closed = true
	s.appApprovals = [3]appApproval{}
	s.disarm()
}

// snapshot is state, character, motion, placement, the perch while one is
// attached, voice, the guide while one runs and the Hands task while one
// runs; a snapshot with none of them is the same as one before any existed.
func (s *Service) snapshot() []feature.Payload {
	state := s.stateFrame()
	if !s.settings.Master {
		return []feature.Payload{state}
	}
	out := []feature.Payload{state}
	if c := s.characterFrame(); c != nil {
		out = append(out, c)
	}
	out = append(out, s.motionFrame(), s.placementFrame())
	if p := s.perch.frame; p != nil {
		frame := *p
		box := *p.Box
		frame.Box = &box
		out = append(out, &frame)
	}
	out = append(out, s.voiceFrame())
	if g := s.guideFrame(); g != nil {
		out = append(out, g)
	}
	out = append(out, s.handsFrames()...)
	return out
}

func (s *Service) voiceFrame() *feature.VoiceFrame {
	v := s.voice
	return &v
}

func (s *Service) shown() bool { return s.settings.Master && !s.settings.Hidden }

// busy is a session running, a strand target awaited, the user talking to
// Oni, a guide in a phase that derives a state or a Hands task running:
// Oni stays awake.
func (s *Service) busy() bool {
	return len(s.sessions) > 0 || s.strand.pending != 0 || s.voiceState() != "" || s.guideState() != "" || s.handsBusy()
}

// wake plays wake when Oni is asleep; the caller publishes the new state.
func (s *Service) wake() {
	if s.asleep && s.shown() {
		s.asleep = false
		s.gesture("wake")
		s.rest(false)
	}
}

func (s *Service) gesture(id string) {
	if !s.shown() {
		return
	}
	s.gestureSeq++
	s.send(&feature.GestureFrame{Seq: s.gestureSeq, ID: id})
}

func (s *Service) state() string {
	if s.asleep {
		return "sleeping"
	}
	if st := s.voiceState(); st != "" {
		return st
	}
	if st := s.guideState(); st != "" {
		return st
	}
	waiting, working, tool := false, false, false
	for _, a := range s.sessions {
		switch a.State {
		case protocol.StateWaiting:
			waiting = true
		case protocol.StateWorking:
			working = true
			tool = tool || a.Tool
		}
	}
	switch {
	case waiting:
		return "ready"
	case tool:
		return "working"
	case working:
		return "thinking"
	case s.failed:
		return "error"
	}
	return "idle"
}

func (s *Service) stateFrame() *feature.StateFrame {
	switch {
	case !s.settings.Master:
		return &feature.StateFrame{State: "sleeping", Reason: ReasonOff}
	case s.settings.Hidden:
		return &feature.StateFrame{State: "sleeping", Reason: ReasonHidden}
	}
	return &feature.StateFrame{State: s.state()}
}

func (s *Service) characterFrame() *feature.CharacterFrame {
	if s.character == nil {
		return nil
	}
	c := *s.character
	c.Fallbacks = slices.Clone(c.Fallbacks)
	return &c
}

// motionFrame carries the talk chord only while voice is on, so a
// presenter's key reader runs only then.
func (s *Service) motionFrame() *feature.MotionFrame {
	m := &feature.MotionFrame{Reduce: s.settings.ReduceMotion, BatterySaver: s.saver, Courtesy: s.settings.Courtesy, Sounds: s.settings.Sounds}
	if s.settings.Voice {
		m.Chord = s.settings.Chord
	}
	return m
}

// publish sends what changed since the last frames. Turning Oni on sends
// every frame a renderer starts from, as Snapshot would.
func (s *Service) publish() {
	state := s.stateFrame()
	if !s.settings.Master {
		if s.last.on || s.last.state != *state {
			s.last.on, s.last.state = false, *state
			s.send(state)
		}
		return
	}
	motion := s.motionFrame()
	if !s.last.on {
		s.last.on, s.last.state, s.last.motion, s.last.size = true, *state, *motion, s.settings.Size
		for _, p := range s.snapshot() {
			s.send(p)
		}
		return
	}
	if s.last.state != *state {
		s.last.state = *state
		s.send(state)
	}
	if s.last.motion != *motion {
		s.last.motion = *motion
		s.send(motion)
	}
	if s.last.size != s.settings.Size {
		s.last.size = s.settings.Size
		s.send(s.placementFrame())
	}
}

// rearm starts the quiet spell over: Oni sleeps once nothing has happened
// for SleepAfter. Nothing is armed while Oni is off, hidden, asleep or busy,
// so a sleeping Oni costs nothing.
func (s *Service) rearm() {
	s.disarm()
	if s.closed || !s.shown() || s.asleep || s.busy() {
		return
	}
	s.armed++
	armed := s.armed
	s.timer = s.clock.AfterFunc(SleepAfter, func() { s.sleep(armed) })
}

func (s *Service) disarm() {
	if s.timer != nil {
		s.timer.Stop()
		s.timer = nil
	}
}

func (s *Service) sleep(armed uint64) {
	s.mu.Lock()
	defer s.unlock()
	if armed != s.armed || s.closed || !s.shown() || s.asleep || s.busy() {
		return
	}
	s.timer = nil
	s.failed = false
	s.gesture("sleep")
	s.asleep = true
	s.rest(true)
	s.publish()
}

func (s *Service) send(p feature.Payload) {
	if s.emit != nil {
		s.emit(p)
	}
}
