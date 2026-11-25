package presence

import (
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"sync"
	"testing"
	"time"

	"github.com/godbus/dbus/v5"
	"ryoku-rashin/internal/oni/feature"
	"ryoku-rashin/internal/protocol"
)

type fakeTimer struct {
	clock *fakeClock
	at    time.Time
	f     func()
	done  bool
}

func (t *fakeTimer) Stop() bool {
	t.clock.mu.Lock()
	defer t.clock.mu.Unlock()
	was := !t.done
	t.done = true
	return was
}

type fakeClock struct {
	mu     sync.Mutex
	now    time.Time
	timers []*fakeTimer
}

func (c *fakeClock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.now
}

func (c *fakeClock) AfterFunc(d time.Duration, f func()) Timer {
	c.mu.Lock()
	defer c.mu.Unlock()
	t := &fakeTimer{clock: c, at: c.now.Add(d), f: f}
	c.timers = append(c.timers, t)
	return t
}

// Advance moves the clock and fires every timer that came due.
func (c *fakeClock) Advance(d time.Duration) {
	c.mu.Lock()
	c.now = c.now.Add(d)
	var due []*fakeTimer
	for _, t := range c.timers {
		if !t.done && !t.at.After(c.now) {
			t.done = true
			due = append(due, t)
		}
	}
	c.mu.Unlock()
	for _, t := range due {
		t.f()
	}
}

// Armed counts the timers still waiting.
func (c *fakeClock) Armed() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	n := 0
	for _, t := range c.timers {
		if !t.done {
			n++
		}
	}
	return n
}

type recorder struct {
	mu     sync.Mutex
	frames []string
}

func (r *recorder) emit(t *testing.T) func(feature.Payload) {
	return func(p feature.Payload) {
		b, err := feature.EncodeFrame(p)
		if err != nil {
			t.Errorf("the service sent a frame the contract refuses: %v", err)
			return
		}
		r.mu.Lock()
		r.frames = append(r.frames, string(b))
		r.mu.Unlock()
	}
}

// take returns the frames sent since the last take.
func (r *recorder) take() []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := r.frames
	r.frames = nil
	return out
}

func encode(t *testing.T, ps ...feature.Payload) []string {
	t.Helper()
	out := []string{}
	for _, p := range ps {
		b, err := feature.EncodeFrame(p)
		if err != nil {
			t.Fatal(err)
		}
		out = append(out, string(b))
	}
	return out
}

func kinds(ps []feature.Payload) []string {
	var out []string
	for _, p := range ps {
		out = append(out, p.Kind())
	}
	return out
}

func placementIn(t *testing.T, ps []feature.Payload) *feature.PlacementFrame {
	t.Helper()
	for _, p := range ps {
		if f, ok := p.(*feature.PlacementFrame); ok {
			return f
		}
	}
	t.Fatalf("no placement frame in %q", kinds(ps))
	return nil
}

func isolate(t *testing.T) {
	t.Helper()
	t.Setenv("XDG_STATE_HOME", filepath.Join(t.TempDir(), "state"))
}

func on() protocol.OniSettings {
	s := protocol.DefaultOniSettings()
	s.Master = true
	return s
}

var moth = feature.CharacterFrame{ID: "testmoth", Name: "Moth (test)"}

func started(t *testing.T) (*Service, *fakeClock, *recorder) {
	t.Helper()
	isolate(t)
	clock := &fakeClock{now: time.Unix(1_800_000_000, 0)}
	rec := &recorder{}
	s := New(clock, rec.emit(t))
	t.Cleanup(s.Close)
	s.Character(moth)
	s.Settings(on())
	rec.take()
	return s, clock, rec
}

func TestSnapshotIsStateCharacterMotionPlacementVoiceInOrder(t *testing.T) {
	s, _, _ := started(t)
	snap := s.Snapshot()
	if got, want := kinds(snap), []string{feature.KindState, feature.KindCharacter, feature.KindMotion, feature.KindPlacement, feature.KindVoice}; !slices.Equal(got, want) {
		t.Fatalf("snapshot kinds = %q, want %q", got, want)
	}
	if st := snap[0].(*feature.StateFrame); st.State != "idle" || st.Reason != "" {
		t.Fatalf("state = %+v", st)
	}
	if c := snap[1].(*feature.CharacterFrame); !reflect.DeepEqual(*c, moth) {
		t.Fatalf("character = %+v", c)
	}
	if m := snap[2].(*feature.MotionFrame); *m != (feature.MotionFrame{Reduce: protocol.OniReduceFollow, Courtesy: true}) {
		t.Fatalf("motion = %+v", m)
	}
	if p := snap[3].(*feature.PlacementFrame); p.Corner != DefaultCorner || p.Size != protocol.OniSizeNormal || len(p.Outputs) != 0 {
		t.Fatalf("placement = %+v", p)
	}
	if v := snap[4].(*feature.VoiceFrame); *v != (feature.VoiceFrame{Phase: VoiceIdle}) {
		t.Fatalf("voice = %+v; nothing listens in this build", v)
	}
}

func TestMasterOffSendsOniOffAndSnapshotsOnlyTheState(t *testing.T) {
	s, clock, rec := started(t)
	off := on()
	off.Master = false
	s.Settings(off)
	offFrame := &feature.StateFrame{State: "sleeping", Reason: ReasonOff}
	if got := rec.take(); !slices.Equal(got, encode(t, offFrame)) {
		t.Fatalf("turning Oni off sent %q", got)
	}
	if got := s.Snapshot(); !slices.Equal(encode(t, got...), encode(t, offFrame)) {
		t.Fatalf("snapshot while off = %q", encode(t, got...))
	}
	if clock.Armed() != 0 {
		t.Fatal("a timer is armed while Oni is off")
	}
	s.Interact(protocol.OniInteractWake)
	s.Activity(Activity{Session: "ask-1", State: protocol.StateWorking})
	s.Activity(Activity{Session: "ask-1", State: protocol.StateIdle})
	if got := rec.take(); len(got) != 0 {
		t.Fatalf("Oni off still sent %q", got)
	}
	s.Settings(on())
	if got := kinds(decodeAll(t, rec.take())); !slices.Equal(got, []string{feature.KindState, feature.KindCharacter, feature.KindMotion, feature.KindPlacement, feature.KindVoice}) {
		t.Fatalf("turning Oni on sent %q", got)
	}
}

func decodeAll(t *testing.T, frames []string) []feature.Payload {
	t.Helper()
	var out []feature.Payload
	for _, f := range frames {
		p, err := feature.DecodeFrame([]byte(f))
		if err != nil {
			t.Fatal(err)
		}
		out = append(out, p)
	}
	return out
}

func TestSleepsAfterTheQuietSpellAndWakes(t *testing.T) {
	s, clock, rec := started(t)
	if clock.Armed() != 1 {
		t.Fatalf("%d timers armed while awake and idle", clock.Armed())
	}
	clock.Advance(SleepAfter - time.Millisecond)
	if got := rec.take(); len(got) != 0 {
		t.Fatalf("sent %q before the quiet spell ended", got)
	}
	clock.Advance(time.Millisecond)
	want := encode(t, &feature.GestureFrame{Seq: 1, ID: "sleep"}, &feature.StateFrame{State: "sleeping"})
	if got := rec.take(); !slices.Equal(got, want) {
		t.Fatalf("sleep sent %q, want %q", got, want)
	}
	if clock.Armed() != 0 {
		t.Fatal("a sleeping Oni still has a timer armed")
	}
	clock.Advance(10 * SleepAfter)
	if got := rec.take(); len(got) != 0 {
		t.Fatalf("a sleeping Oni sent %q", got)
	}
	s.Interact(protocol.OniInteractWake)
	want = encode(t, &feature.GestureFrame{Seq: 2, ID: "wake"}, &feature.StateFrame{State: "idle"})
	if got := rec.take(); !slices.Equal(got, want) {
		t.Fatalf("wake sent %q, want %q", got, want)
	}
	if clock.Armed() != 1 {
		t.Fatal("waking did not start the quiet spell over")
	}
	s.Interact(protocol.OniInteractWake)
	if got := rec.take(); len(got) != 0 {
		t.Fatalf("waking an awake Oni sent %q", got)
	}
}

func TestPokingPutsOffSleep(t *testing.T) {
	s, clock, rec := started(t)
	clock.Advance(SleepAfter / 2)
	s.Interact(protocol.OniInteractPoke)
	clock.Advance(SleepAfter/2 + time.Second)
	if got := rec.take(); len(got) != 0 {
		t.Fatalf("Oni slept %q though it was poked", got)
	}
	clock.Advance(SleepAfter)
	if got := kinds(decodeAll(t, rec.take())); !slices.Equal(got, []string{feature.KindGesture, feature.KindState}) {
		t.Fatalf("no sleep after a full quiet spell: %q", got)
	}
}

func TestSessionsSetTheState(t *testing.T) {
	s, clock, rec := started(t)
	step := func(a Activity, want ...feature.Payload) {
		t.Helper()
		s.Activity(a)
		if got := rec.take(); !slices.Equal(got, encode(t, want...)) {
			t.Fatalf("%+v sent %q, want %q", a, got, encode(t, want...))
		}
	}
	step(Activity{Session: "ask-1", State: protocol.StateWorking}, &feature.StateFrame{State: "thinking"})
	if clock.Armed() != 0 {
		t.Fatal("Oni may sleep while a turn runs")
	}
	step(Activity{Session: "ask-1", State: protocol.StateWorking})
	step(Activity{Session: "ask-1", State: protocol.StateWorking, Tool: true}, &feature.StateFrame{State: "working"})
	step(Activity{Session: "agent-2", State: protocol.StateWaiting}, &feature.StateFrame{State: "ready"})
	step(Activity{Session: "agent-2", State: protocol.StateIdle}, &feature.GestureFrame{Seq: 1, ID: "done"}, &feature.StateFrame{State: "working"})
	step(Activity{Session: "ask-1", State: protocol.StateFailed}, &feature.GestureFrame{Seq: 2, ID: "error"}, &feature.StateFrame{State: "error"})
	step(Activity{Session: "ask-1", State: protocol.StateFailed})
	step(Activity{Session: "never-ran", State: protocol.StateIdle})
	if clock.Armed() != 1 {
		t.Fatal("the quiet spell did not start once every turn ended")
	}
	s.Interact(protocol.OniInteractPoke)
	if got := rec.take(); !slices.Equal(got, encode(t, &feature.StateFrame{State: "idle"})) {
		t.Fatalf("a poke after an error sent %q", got)
	}
	clock.Advance(SleepAfter)
	rec.take()
	step(Activity{Session: "ask-3", State: protocol.StateWorking}, &feature.GestureFrame{Seq: 4, ID: "wake"}, &feature.StateFrame{State: "thinking"})
}

func TestHiddenUnmapsAndArmsNothing(t *testing.T) {
	s, clock, rec := started(t)
	hidden := on()
	hidden.Hidden = true
	s.Settings(hidden)
	if got := rec.take(); !slices.Equal(got, encode(t, &feature.StateFrame{State: "sleeping", Reason: ReasonHidden})) {
		t.Fatalf("hiding sent %q", got)
	}
	if clock.Armed() != 0 {
		t.Fatal("a hidden Oni has a timer armed")
	}
	s.Settings(on())
	if got := rec.take(); !slices.Equal(got, encode(t, &feature.StateFrame{State: "idle"})) {
		t.Fatalf("showing sent %q", got)
	}
}

func TestPresentationSettingsSendTheirFrames(t *testing.T) {
	s, _, rec := started(t)
	next := on()
	next.ReduceMotion = protocol.OniReduceOn
	next.Courtesy = false
	s.Settings(next)
	if got := rec.take(); !slices.Equal(got, encode(t, &feature.MotionFrame{Reduce: protocol.OniReduceOn})) {
		t.Fatalf("reduce motion sent %q", got)
	}
	next.Size = protocol.OniSizeBig
	s.Settings(next)
	got := decodeAll(t, rec.take())
	if len(got) != 1 || got[0].(*feature.PlacementFrame).Size != protocol.OniSizeBig {
		t.Fatalf("size sent %+v", got)
	}
	s.Character(moth)
	if got := rec.take(); len(got) != 0 {
		t.Fatalf("the same character sent %q", got)
	}
	other := feature.CharacterFrame{ID: "partial", Name: "Moth (test)", Fallbacks: []protocol.OniFallback{{Slot: "animations.states.listening", Reason: "missing", Used: "static-pose"}}}
	s.Character(other)
	if got := rec.take(); !slices.Equal(got, encode(t, &other)) {
		t.Fatalf("a new character sent %q", got)
	}
}

func TestPlacementPersistsAcrossARestart(t *testing.T) {
	s, _, rec := started(t)
	if err := s.Place(protocol.OniPlaceParams{Output: "DP-1", FX: 0.25, FY: 0.75}); err != nil {
		t.Fatal(err)
	}
	if err := s.Place(protocol.OniPlaceParams{Output: "HDMI-A-1", FX: 1, FY: 0}); err != nil {
		t.Fatal(err)
	}
	if err := s.Place(protocol.OniPlaceParams{Output: "DP-1", FX: 0.5, FY: 0.5}); err != nil {
		t.Fatal(err)
	}
	want := feature.PlacementFrame{Corner: "", Size: protocol.OniSizeNormal, Outputs: []feature.PlacementOutput{{ID: "DP-1", FX: 0.5, FY: 0.5}, {ID: "HDMI-A-1", FX: 1, FY: 0}}, Active: "DP-1"}
	sent := decodeAll(t, rec.take())
	if len(sent) != 3 {
		t.Fatalf("placing sent %d frames", len(sent))
	}
	last := *sent[2].(*feature.PlacementFrame)
	last.Seq = 0
	if !reflect.DeepEqual(last, want) {
		t.Fatalf("placement = %+v, want %+v", last, want)
	}
	for _, bad := range []protocol.OniPlaceParams{{Output: "", Corner: ""}, {Output: "DP-1", FX: 2}, {Corner: "middle"}} {
		if err := s.Place(bad); err == nil {
			t.Fatalf("%+v was saved", bad)
		}
	}
	s.Close()

	again := New(&fakeClock{}, nil)
	again.Settings(on())
	snap := again.Snapshot()
	got := *placementIn(t, snap)
	got.Seq = 0
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("after a restart placement = %+v, want %+v", got, want)
	}
	if err := again.Place(protocol.OniPlaceParams{Corner: protocol.OniCornerTopLeft}); err != nil {
		t.Fatal(err)
	}
	again.Close()
	p, err := ReadPlace(PlacementFile())
	if err != nil {
		t.Fatal(err)
	}
	if p.Corner != protocol.OniCornerTopLeft || p.Active != "DP-1" || len(p.Outputs) != 2 {
		t.Fatalf("a pin lost the saved spots: %+v", p)
	}
}

func TestPlacementFileIsReadStrictly(t *testing.T) {
	isolate(t)
	file := PlacementFile()
	if err := os.MkdirAll(filepath.Dir(file), 0o700); err != nil {
		t.Fatal(err)
	}
	good := `{"version":1,"corner":"","outputs":[{"id":"DP-1","fx":0.1,"fy":0.9}],"active":"DP-1"}`
	if err := os.WriteFile(file, []byte(good), 0o600); err != nil {
		t.Fatal(err)
	}
	if p, err := ReadPlace(file); err != nil || p.Active != "DP-1" || len(p.Outputs) != 1 {
		t.Fatalf("good file = %+v, %v", p, err)
	}
	for _, bad := range []string{
		`{"version":2,"corner":"","outputs":[]}`,
		`{"version":1.0,"corner":"","outputs":[]}`,
		`{"corner":"","outputs":[]}`,
		`{"version":1,"outputs":[]}`,
		`{"version":1,"corner":"","outputs":null}`,
		`{"version":1,"corner":"","outputs":[],"size":"big"}`,
		`{"version":1,"Corner":"tl","outputs":[]}`,
		`{"version":1,"corner":"tl","corner":"br","outputs":[]}`,
		`{"version":1,"corner":"","outputs":[{"id":"DP-1","fx":1.5,"fy":0}]}`,
		`{"version":1,"corner":"","outputs":[{"id":"DP-1","fy":0}]}`,
		`{"version":1,"corner":"","outputs":[]} {}`,
		`[]`,
	} {
		if err := os.WriteFile(file, []byte(bad), 0o600); err != nil {
			t.Fatal(err)
		}
		if p, err := ReadPlace(file); err == nil {
			t.Errorf("%s read as %+v", bad, p)
		}
		s := New(&fakeClock{}, nil)
		s.Settings(on())
		snap := s.Snapshot()
		if p := placementIn(t, snap); p.Corner != DefaultCorner || len(p.Outputs) != 0 {
			t.Errorf("%s was used as %+v", bad, p)
		}
		s.Close()
	}
}

func TestPowerSaverReachesTheMotionFrame(t *testing.T) {
	s, _, rec := started(t)
	s.Power(true)
	if got := rec.take(); !slices.Equal(got, encode(t, &feature.MotionFrame{Reduce: protocol.OniReduceFollow, BatterySaver: true, Courtesy: true})) {
		t.Fatalf("the saver profile sent %q", got)
	}
	s.Power(true)
	if got := rec.take(); len(got) != 0 {
		t.Fatalf("an unchanged profile sent %q", got)
	}
	if m := s.Snapshot()[2].(*feature.MotionFrame); !m.BatterySaver {
		t.Fatalf("snapshot motion = %+v", m)
	}
}

func TestPowerProfileSignalsAreReadExactly(t *testing.T) {
	const iface = "net.hadess.PowerProfiles"
	changed := func(body ...any) *dbus.Signal {
		return &dbus.Signal{Name: "org.freedesktop.DBus.Properties.PropertiesChanged", Body: body}
	}
	if p, ok := changedProfile(changed(iface, map[string]dbus.Variant{"ActiveProfile": dbus.MakeVariant("power-saver")}, []string{}), iface); !ok || p != powerSaverProfile {
		t.Fatalf("profile = %q, %v", p, ok)
	}
	for _, sig := range []*dbus.Signal{
		changed("org.example.Other", map[string]dbus.Variant{"ActiveProfile": dbus.MakeVariant("power-saver")}, []string{}),
		changed(iface, map[string]dbus.Variant{"PerformanceDegraded": dbus.MakeVariant("")}, []string{}),
		changed(iface, map[string]dbus.Variant{"ActiveProfile": dbus.MakeVariant(3)}, []string{}),
		changed(iface),
		{Name: "org.example.Other.Signal", Body: []any{iface, map[string]dbus.Variant{}}},
	} {
		if p, ok := changedProfile(sig, iface); ok {
			t.Errorf("%+v read as %q", sig, p)
		}
	}
}
