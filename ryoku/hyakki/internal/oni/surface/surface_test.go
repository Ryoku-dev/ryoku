package surface

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"image"
	"image/color"
	"io"
	"os"
	"strings"
	"testing"
	"time"

	"ryoku-rashin/internal/oni/feature"
	"ryoku-rashin/internal/oni/motion"
	"ryoku-rashin/internal/oni/profile"
	"ryoku-rashin/internal/oni/spec"
	"ryoku-rashin/internal/protocol"
)

func encode(t *testing.T, p feature.Payload) json.RawMessage {
	t.Helper()
	b, err := feature.EncodeFrame(p)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func TestLedgerWritesOneStampedJSONLinePerEvent(t *testing.T) {
	var buf bytes.Buffer
	now := time.Unix(100, 0)
	l := newLedger(&buf, "wayland-7", func() time.Time { return now })
	now = now.Add(1500 * time.Millisecond)
	l.Add(Entry{Ev: EvCommit, State: "thinking", Pose: motion.Pose{Scale: 1}, Rect: rectOf(image.Rect(10, 20, 74, 84)), Output: "HEADLESS-1", FPS: 60})
	l.Add(Entry{Ev: EvStep, DT: new(0.016), Commit: new(false), Input: motion.Input{Reduced: true}})
	var nilLedger *Ledger
	nilLedger.Add(Entry{Ev: EvStop})

	lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
	if len(lines) != 2 {
		t.Fatalf("lines = %d, want 2: %q", len(lines), buf.String())
	}
	var first map[string]any
	if err := json.Unmarshal([]byte(lines[0]), &first); err != nil {
		t.Fatal(err)
	}
	if first["t"] != 1.5 || first["ev"] != "commit" || first["display"] != "wayland-7" || first["output"] != "HEADLESS-1" || first["fps"] != 60.0 {
		t.Fatalf("commit line = %v", first)
	}
	if r, _ := first["rect"].([]any); len(r) != 4 || r[0] != 10.0 || r[1] != 20.0 || r[2] != 64.0 || r[3] != 64.0 {
		t.Fatalf("rect = %v, want [10 20 64 64]", first["rect"])
	}
	if _, ok := first["pose"].(map[string]any); !ok {
		t.Fatalf("pose = %v, want an object", first["pose"])
	}
	var second map[string]any
	if err := json.Unmarshal([]byte(lines[1]), &second); err != nil {
		t.Fatal(err)
	}
	if second["dt"] != 0.016 || second["commit"] != false || second["input"] == nil {
		t.Fatalf("step line = %v", second)
	}
	if _, ok := second["rect"]; ok {
		t.Fatalf("step line carries a rect: %v", second)
	}
}

func TestLabHooksNeedTheLabSwitch(t *testing.T) {
	env := map[string]string{"HYAKKI_ONI_LEDGER": "/tmp/x", "HYAKKI_ONI_TIMESCALE": "0.1", "HYAKKI_ONI_POWER": "saver", "HYAKKI_ONI_SCRIPT": "/tmp/s", "HYAKKI_ONI_PROFILE": "/tmp/p",
		"HYAKKI_ONI_SOUND_FILE": "/tmp/pcm", "HYAKKI_ONI_ARRIVALS": "/tmp/a"}
	get := func(k string) string { return env[k] }
	off, err := LabFromEnv(get)
	if err != nil || off.On || off.Ledger != "" || off.Script != "" || off.Profile != "" || off.SoundFile != "" || off.Arrivals != "" || off.TimeScale != 1 || off.batterySaver(false) {
		t.Fatalf("without HYAKKI_ONI_LAB = %+v, %v", off, err)
	}
	env["HYAKKI_ONI_LAB"] = "1"
	on, err := LabFromEnv(get)
	if err != nil || !on.On || on.Ledger != "/tmp/x" || on.Script != "/tmp/s" || on.Profile != "/tmp/p" || on.SoundFile != "/tmp/pcm" || on.Arrivals != "/tmp/a" || on.TimeScale != 0.1 || !on.batterySaver(false) {
		t.Fatalf("with HYAKKI_ONI_LAB = %+v, %v", on, err)
	}
	for k, v := range map[string]string{"HYAKKI_ONI_TIMESCALE": "2", "HYAKKI_ONI_POWER": "turbo"} {
		bad := map[string]string{"HYAKKI_ONI_LAB": "1", k: v}
		if _, err := LabFromEnv(func(k string) string { return bad[k] }); err == nil {
			t.Fatalf("%s=%s accepted", k, v)
		}
	}
	full := Lab{Power: PowerFull}
	if full.batterySaver(true) {
		t.Fatal("HYAKKI_ONI_POWER=full kept the daemon's saver tier")
	}
}

func TestScriptRejectsFramesTheDaemonCouldNotSend(t *testing.T) {
	for name, src := range map[string]string{
		"unknown state": `[{"atMs":0,"frame":{"kind":"state","data":{"state":"dancing"}}}]`,
		"both":          `{"atMs":0,"eof":true,"frame":{"kind":"state","data":{"state":"idle"}}}`,
		"neither":       `{"atMs":5}`,
		"negative":      `{"atMs":-1,"eof":true}`,
		"unknown key":   `{"atMs":0,"eof":true,"at":1}`,
	} {
		if _, err := LoadScript(strings.NewReader(src)); err == nil {
			t.Errorf("%s: accepted", name)
		}
	}
}

func next(t *testing.T, l Link) feature.Payload {
	t.Helper()
	select {
	case raw, ok := <-l.Frames():
		if !ok {
			t.Fatal("frames closed")
		}
		p, err := feature.DecodeFrame(raw)
		if err != nil {
			t.Fatal(err)
		}
		return p
	case <-time.After(5 * time.Second):
		t.Fatal("no frame")
	}
	return nil
}

func TestScriptedDaemonPlaysTheScriptAfterItsSnapshot(t *testing.T) {
	lines := []string{
		`{"atMs":30,"frame":` + string(encode(t, &feature.StateFrame{State: "thinking"})) + `}`,
		`{"atMs":10,"frame":` + string(encode(t, &feature.StateFrame{State: "listening"})) + `}`,
		`{"atMs":50,"frame":` + string(encode(t, &feature.StateFrame{State: "sleeping"})) + `}`,
	}
	entries, err := LoadScript(strings.NewReader(strings.Join(lines, "\n")))
	if err != nil {
		t.Fatal(err)
	}
	l, err := ScriptDialer(entries, "")(context.Background(), true)
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()
	res, err := l.Open(context.Background())
	if err != nil || !res.Presenter {
		t.Fatalf("open = %+v, %v", res, err)
	}
	var kinds []string
	for _, raw := range res.Frames {
		p, err := feature.DecodeFrame(raw)
		if err != nil {
			t.Fatal(err)
		}
		kinds = append(kinds, p.Kind())
	}
	if strings.Join(kinds, ",") != "state,character,motion,placement,voice" {
		t.Fatalf("snapshot = %v", kinds)
	}
	for _, want := range []string{"listening", "thinking", "sleeping"} {
		if s := next(t, l).(*feature.StateFrame); s.State != want {
			t.Fatalf("state = %q, want %q", s.State, want)
		}
	}
	if err := l.Interact(context.Background(), "wake"); err != nil {
		t.Fatal(err)
	}
	if g, ok := next(t, l).(*feature.GestureFrame); !ok || g.ID != "wake" {
		t.Fatalf("after wake: %#v", g)
	}
	if s, ok := next(t, l).(*feature.StateFrame); !ok || s.State != "idle" {
		t.Fatalf("after wake gesture: %#v", s)
	}
	if err := l.Place(context.Background(), protocol.OniPlaceParams{Output: "HEADLESS-1", FX: 0.25, FY: 0.75}); err != nil {
		t.Fatal(err)
	}
	p, ok := next(t, l).(*feature.PlacementFrame)
	if !ok || p.Corner != "" || p.Active != "HEADLESS-1" || len(p.Outputs) != 1 || p.Outputs[0].FX != 0.25 || p.Outputs[0].FY != 0.75 {
		t.Fatalf("placement after place = %#v", p)
	}
	a, err := l.Assets(context.Background(), profile.ShippedID)
	if err != nil || len(a.Files) == 0 || a.ID != profile.ShippedID {
		t.Fatalf("assets = %d files, %v", len(a.Files), err)
	}
}

func TestScriptedDaemonEOFEndsTheLink(t *testing.T) {
	entries, err := LoadScript(strings.NewReader(`[{"atMs":5,"eof":true}]`))
	if err != nil {
		t.Fatal(err)
	}
	l, _ := ScriptDialer(entries, "")(context.Background(), false)
	if _, err := l.Open(context.Background()); err != nil {
		t.Fatal(err)
	}
	select {
	case <-l.Done():
	case <-time.After(5 * time.Second):
		t.Fatal("eof entry did not end the link")
	}
	if !errors.Is(l.Err(), io.EOF) {
		t.Fatalf("err = %v, want EOF", l.Err())
	}
	if err := l.Interact(context.Background(), "poke"); err == nil {
		t.Fatal("a call after EOF succeeded")
	}
}

func TestPressBecomesADragOnlyPastTheThreshold(t *testing.T) {
	sp, err := spec.Load()
	if err != nil {
		t.Fatal(err)
	}
	limit := float64(sp.Placement.DragThresholdPx)
	now := time.Unix(0, 0)
	var g grip
	g.press(image.Pt(50, 50), spec.Point{X: 100, Y: 100}, now)
	if started, dragging, _ := g.move(spec.Point{X: 100 + limit, Y: 100}, now.Add(time.Millisecond)); started || dragging {
		t.Fatal("a move of exactly the threshold dragged")
	}
	started, dragging, pos := g.move(spec.Point{X: 101 + limit, Y: 100}, now.Add(2*time.Millisecond))
	if !started || !dragging || pos != image.Pt(51+int(limit), 50) {
		t.Fatalf("past threshold: started=%v dragging=%v pos=%v", started, dragging, pos)
	}
	if g.vel.X <= 0 {
		t.Fatalf("drag velocity = %v, want rightward", g.vel)
	}
	if started, dragging, pos = g.move(spec.Point{X: 120, Y: 130}, now.Add(3*time.Millisecond)); started || !dragging || pos != image.Pt(70, 80) {
		t.Fatalf("further move: started=%v dragging=%v pos=%v", started, dragging, pos)
	}
	if !g.release() {
		t.Fatal("release after a drag reported a click")
	}

	g.press(image.Pt(50, 50), spec.Point{X: 100, Y: 100}, now)
	g.move(spec.Point{X: 102, Y: 101}, now.Add(time.Millisecond))
	if g.release() {
		t.Fatal("a press inside the threshold reported a drag")
	}
	if _, dragging, _ := g.move(spec.Point{X: 200, Y: 200}, now); dragging {
		t.Fatal("moving without a press dragged")
	}
}

func TestLinkLossGoesDormantWhenAsleep(t *testing.T) {
	if onLinkLost("sleeping", false) != LossDormant {
		t.Fatal("an asleep presenter redialled after the daemon idled out")
	}
	if onLinkLost("thinking", false) != LossRetry {
		t.Fatal("an awake presenter did not try one quiet redial")
	}
	if onLinkLost("thinking", true) != LossDormant {
		t.Fatal("a second loss while awake kept redialling")
	}
}

func TestStoppedClockArmsNothing(t *testing.T) {
	t0 := time.Unix(10, 0)
	var c clock
	c.halt()
	if !c.idle() {
		t.Fatal("a halted clock refused an event")
	}
	if wait := c.stepped(t0, true, true, 0); wait != 0 || c.armed || c.waiting {
		t.Fatalf("stopped step: wait=%v armed=%v waiting=%v", wait, c.armed, c.waiting)
	}
	if step, wait := c.frameDone(t0.Add(time.Second)); step || wait != 0 {
		t.Fatal("a late frame callback restarted a stopped clock")
	}

	if wait := c.stepped(t0, false, true, 50*time.Millisecond); wait != 0 || !c.armed || c.idle() {
		t.Fatal("a committed step did not arm the frame callback")
	}
	step, wait := c.frameDone(t0.Add(16 * time.Millisecond))
	if step || wait != 34*time.Millisecond {
		t.Fatalf("early callback at a 20 fps tier: step=%v wait=%v", step, wait)
	}
	if !c.timerFired() {
		t.Fatal("the pacing timer did not step")
	}
	c.stepped(t0.Add(50*time.Millisecond), false, true, 16*time.Millisecond)
	if step, _ := c.frameDone(t0.Add(65 * time.Millisecond)); !step {
		t.Fatal("a callback within the slack did not step")
	}
	if wait := c.stepped(t0, false, false, 20*time.Millisecond); wait != 20*time.Millisecond || c.armed {
		t.Fatalf("running step without a commit: wait=%v armed=%v", wait, c.armed)
	}
}

func TestVisibilityOrder(t *testing.T) {
	cases := []struct {
		hidden, courtesy, covered, out bool
		show                           bool
		reason                         string
	}{
		{false, true, false, true, true, ""},
		{false, true, true, true, false, hideCourtesy},
		{false, false, true, true, true, ""},
		{true, true, true, true, false, hideHidden},
		{false, true, false, false, false, hideOutput},
	}
	for _, c := range cases {
		show, reason := visibility(c.hidden, c.courtesy, c.covered, c.out)
		if show != c.show || reason != c.reason {
			t.Errorf("visibility(%v,%v,%v,%v) = %v,%q want %v,%q", c.hidden, c.courtesy, c.covered, c.out, show, reason, c.show, c.reason)
		}
	}
}

func TestShmPixelsArePremultipliedBGRA(t *testing.T) {
	img := image.NewNRGBA(image.Rect(0, 0, 3, 1))
	img.SetNRGBA(0, 0, color.NRGBA{R: 200, G: 100, B: 50, A: 255})
	img.SetNRGBA(1, 0, color.NRGBA{R: 200, G: 100, B: 50, A: 128})
	img.SetNRGBA(2, 0, color.NRGBA{R: 200, G: 100, B: 50, A: 0})
	dst := make([]byte, 12)
	toARGB8888(dst, img)
	want := []byte{50, 100, 200, 255, 25, 50, 100, 128, 0, 0, 0, 0}
	if !bytes.Equal(dst, want) {
		t.Fatalf("pixels = %v, want %v", dst, want)
	}
}

func TestAssetsLoadOnlyAsTheRequestedValidCharacter(t *testing.T) {
	src, _ := profile.Load(os.DirFS("../profile/testdata"), "testmoth")
	if !src.Valid {
		t.Fatalf("fixture invalid: %+v", src.Report)
	}
	a := protocol.OniCharacterAssets{ID: "testmoth", Files: src.Files}
	got, err := loadAssets("testmoth", a)
	if err != nil || got.Profile.ID != "testmoth" {
		t.Fatalf("loadAssets = %+v, %v", got.Profile.ID, err)
	}
	if _, err := loadAssets("other", a); err == nil {
		t.Fatal("assets answered for another id were accepted")
	}
	broken := protocol.OniCharacterAssets{ID: "testmoth", Files: map[string][]byte{profile.ProfileFile: []byte("{")}}
	if _, err := loadAssets("testmoth", broken); err == nil {
		t.Fatal("an invalid profile was accepted")
	}
}
