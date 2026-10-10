package main

import (
	"bufio"
	"encoding/json"
	"io"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// writeStageStub installs a fake ryostage engine (via RYOKU_STAGE_ENGINE) that
// records every invocation and, for cut/inpaint, writes the requested output.
// Behaviour is steered per-test through STAGE_STUB_* env vars the daemon
// inherits when it spawns the child. Returns the call-log path.
func writeStageStub(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	logf := filepath.Join(dir, "calls.log")
	bin := filepath.Join(dir, "ryostage-stub")
	const script = `#!/usr/bin/env bash
echo "$*" >> "$STAGE_STUB_LOG"
cmd="${1:-}"; shift || true
case "$cmd" in
  check)
    if [[ -n "${STAGE_STUB_MISSING:-}" ]]; then echo "${STAGE_STUB_CHECK_REASON:-missing}"; exit 1; fi
    echo available; exit 0 ;;
  models)
    echo '[{"id":"u2netp","tier":"draft","label":"Draft","installed":true}]'; exit 0 ;;
  cut)
    out="$2"
    if [[ -n "${STAGE_STUB_SLEEP:-}" ]]; then sleep "$STAGE_STUB_SLEEP"; fi
    if [[ -n "${STAGE_STUB_CUT_FAIL:-}" ]]; then echo "cut failed" >&2; exit 1; fi
    mkdir -p "$(dirname "$out")"; printf 'SUBJECTPNG' > "$out"; echo "$out"; exit 0 ;;
  inpaint)
    out="$3"
    if [[ -n "${STAGE_STUB_INPAINT_FAIL:-}" ]]; then echo "inpaint failed" >&2; exit 1; fi
    mkdir -p "$(dirname "$out")"; printf 'BGPNG' > "$out"; echo "$out"; exit 0 ;;
esac
exit 0
`
	if err := os.WriteFile(bin, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("RYOKU_STAGE_ENGINE", bin)
	t.Setenv("STAGE_STUB_LOG", logf)
	return logf
}

// countCalls returns how many logged engine invocations began with verb.
func countCalls(logf, verb string) int {
	b, err := os.ReadFile(logf)
	if err != nil {
		return 0
	}
	n := 0
	for _, line := range strings.Split(string(b), "\n") {
		if fields := strings.Fields(line); len(fields) > 0 && fields[0] == verb {
			n++
		}
	}
	return n
}

// stageHome wires a hermetic HOME + XDG tree and returns it.
func stageHome(t *testing.T) string {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_STATE_HOME", filepath.Join(home, ".local", "state"))
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
	t.Setenv("XDG_CACHE_HOME", filepath.Join(home, ".cache"))
	// An empty runtime dir: no ryogami socket, so depthClear/publish fail fast
	// and harmlessly instead of reaching the live daemon.
	t.Setenv("XDG_RUNTIME_DIR", t.TempDir())
	return home
}

func (d *daemon) showWall(pic string) {
	d.ryoWallMu.Lock()
	d.ryoWall = ryogamiFrame{Default: ryogamiFrameEntry{Path: pic}}
	d.ryoWallMu.Unlock()
}

// TestStageWallsRoundTrip pins the v2 registry contract: a saved registry loads
// back identically (effect + the three layer flags), the shape carries neither
// the retired mode/scene nor per-layer look knobs, and a missing or corrupt file
// loads with a non-nil map so callers never panic assigning in.
func TestStageWallsRoundTrip(t *testing.T) {
	home := stageHome(t)
	wall := filepath.Join(home, "w.png")

	reg := stageWalls{Current: wall, Walls: map[string]stageWall{}}
	reg.Walls[wall] = stageWall{
		Effect: stageEffectParallax,
		Layers: []stageLayer{
			{Out: "/a/subject.png", Label: "Subject", Enabled: true, Front: true, Depth: 0.5},
			{Out: "/a/layer-02.png", Label: "Layer 2", Enabled: false, Front: false, Depth: 0.9},
		},
	}
	if err := saveStageWalls(reg); err != nil {
		t.Fatal(err)
	}
	got := loadStageWalls()
	if got.Current != wall {
		t.Fatalf("current = %q, want %q", got.Current, wall)
	}
	w := got.Walls[wall]
	if w.Effect != stageEffectParallax {
		t.Fatalf("effect = %v, want parallax", w.Effect)
	}
	if len(w.Layers) != 2 {
		t.Fatalf("layers = %d, want 2", len(w.Layers))
	}
	if s := w.Layers[0]; s.Label != "Subject" || !s.Enabled || !s.Front || s.Depth != 0.5 {
		t.Fatalf("subject layer round-trip = %+v", s)
	}
	if l := w.Layers[1]; l.Enabled || l.Front || l.Depth != 0.9 {
		t.Fatalf("manual layer round-trip = %+v", l)
	}
	raw, _ := os.ReadFile(stageWallsPath())
	for _, gone := range []string{"\"mode\"", "\"scene\"", "opacity", "feather"} {
		if strings.Contains(string(raw), gone) {
			t.Fatalf("v2 registry still carries the retired %q: %s", gone, raw)
		}
	}

	// Normalisation after a decode failure: a garbage file still yields a
	// non-nil map.
	if err := os.WriteFile(stageWallsPath(), []byte("{not json"), 0o644); err != nil {
		t.Fatal(err)
	}
	if norm := loadStageWalls(); norm.Walls == nil {
		t.Fatal("corrupt registry did not normalise to a non-nil Walls map")
	}
}

// TestStageSwitchReusesWithoutGenerating pins the core rule: a wallpaper switch
// (a plain reconcile, neither force nor gen) reuses an existing cut instantly
// and never runs the engine -- not even the availability probe.
func TestStageSwitchReusesWithoutGenerating(t *testing.T) {
	home := stageHome(t)
	logf := writeStageStub(t)

	wall := filepath.Join(home, "w.png")
	writeFile(t, wall, "wp")
	subj := stageSubjectOut(wall)
	if err := os.MkdirAll(filepath.Dir(subj), 0o755); err != nil {
		t.Fatal(err)
	}
	writeFile(t, subj, "SUBJECT")
	newer := time.Now().Add(time.Hour)
	if err := os.Chtimes(subj, newer, newer); err != nil {
		t.Fatal(err)
	}
	reg := stageWalls{Walls: map[string]stageWall{wall: {Effect: stageEffectDepth}}}
	if err := saveStageWalls(reg); err != nil {
		t.Fatal(err)
	}

	d := &daemon{}
	d.showWall(wall)
	d.reconcileStage(false, false)

	if got := countCalls(logf, "cut") + countCalls(logf, "inpaint") + countCalls(logf, "check"); got != 0 {
		t.Fatalf("a switch made %d engine calls, want 0", got)
	}
	frame := d.buildStageFrame()
	if frame.Walls[wall].Subject != subj {
		t.Fatalf("reused subject not in frame: %+v", frame.Walls[wall])
	}
}

// TestStageSetEffectDepth pins the enable path: turning Depth on for an uncut
// wallpaper runs exactly one cut, publishes the subject in the topic frame, and
// folds it to ryogami over the unchanged `depth set` wire.
func TestStageSetEffectDepth(t *testing.T) {
	home := stageHome(t)
	logf := writeStageStub(t)

	// A fake ryogami to capture the pixel-lock fold.
	rt := os.Getenv("XDG_RUNTIME_DIR")
	ln, err := net.Listen("unix", filepath.Join(rt, "ryogami.sock"))
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	lines := make(chan string, 4)
	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			go func(c net.Conn) {
				defer c.Close()
				l, _ := bufio.NewReader(c).ReadString('\n')
				lines <- strings.TrimSpace(l)
				_, _ = io.WriteString(c, "ok\n")
			}(conn)
		}
	}()

	wall := filepath.Join(home, "w.png")
	writeFile(t, wall, "wp")
	d := &daemon{stageSig: make(chan struct{}, 1)}
	d.showWall(wall)

	d.stageSetEffect(stageEffectDepth)
	d.reconcileStage(d.stageForce.Swap(false), d.stageGen.Swap(false))

	if n := countCalls(logf, "cut"); n != 1 {
		t.Fatalf("cut called %d times, want exactly 1", n)
	}
	subj := stageSubjectOut(wall)
	if !isFile(subj) {
		t.Fatalf("subject.png not produced at %s", subj)
	}
	frame := d.buildStageFrame()
	if frame.Walls[wall].Effect != stageEffectDepth || frame.Walls[wall].Subject != subj {
		t.Fatalf("frame did not publish subject: %+v", frame.Walls[wall])
	}
	// layers[0] is always the subject slot.
	if len(frame.Walls[wall].Layers) == 0 || frame.Walls[wall].Layers[0].Label != "Subject" {
		t.Fatalf("frame layers[0] is not the subject: %+v", frame.Walls[wall].Layers)
	}
	select {
	case got := <-lines:
		body, ok := strings.CutPrefix(got, "depth set ")
		if !ok || !strings.Contains(body, subj) {
			t.Fatalf("ryogami fold = %q, want `depth set` carrying %s", got, subj)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("subject was never folded to ryogami")
	}
}

// TestStageSetEffectParallax pins the parallax enable: it runs a cut then an
// inpaint and publishes both the subject and the recoloured background.
func TestStageSetEffectParallax(t *testing.T) {
	home := stageHome(t)
	logf := writeStageStub(t)

	wall := filepath.Join(home, "w.png")
	writeFile(t, wall, "wp")
	d := &daemon{stageSig: make(chan struct{}, 1)}
	d.showWall(wall)

	d.stageSetEffect(stageEffectParallax)
	d.reconcileStage(d.stageForce.Swap(false), d.stageGen.Swap(false))

	if n := countCalls(logf, "cut"); n != 1 {
		t.Fatalf("cut called %d times, want 1", n)
	}
	if n := countCalls(logf, "inpaint"); n != 1 {
		t.Fatalf("inpaint called %d times, want 1", n)
	}
	subj := stageSubjectOut(wall)
	bg := stageBackgroundOut(wall)
	if !isFile(subj) || !isFile(bg) {
		t.Fatalf("subject=%v background=%v, want both", isFile(subj), isFile(bg))
	}
	w := d.buildStageFrame().Walls[wall]
	if w.Effect != stageEffectParallax || w.Subject != subj || w.Background != bg {
		t.Fatalf("frame did not publish parallax artifacts: %+v", w)
	}
}

// TestStageEffectSwitchNeverRecuts pins the spec's core rule that an effect
// switch never re-cuts: parallax then depth runs the engine at most once for the
// cut and once for the inpaint, and depth -> parallax -> depth runs nothing new.
func TestStageEffectSwitchNeverRecuts(t *testing.T) {
	home := stageHome(t)
	logf := writeStageStub(t)

	wall := filepath.Join(home, "w.png")
	writeFile(t, wall, "wp")
	d := &daemon{stageSig: make(chan struct{}, 1)}
	d.showWall(wall)

	apply := func(eff stageEffect) {
		d.stageSetEffect(eff)
		d.reconcileStage(d.stageForce.Swap(false), d.stageGen.Swap(false))
	}

	// First Parallax use: one cut + one inpaint. Switching to Depth adds neither.
	apply(stageEffectParallax)
	apply(stageEffectDepth)
	if n := countCalls(logf, "cut"); n != 1 {
		t.Fatalf("cut ran %d times across parallax->depth, want exactly 1", n)
	}
	if n := countCalls(logf, "inpaint"); n != 1 {
		t.Fatalf("inpaint ran %d times across parallax->depth, want exactly 1", n)
	}

	// depth -> parallax -> depth: the subject and backdrop already exist, so the
	// engine runs nothing new.
	apply(stageEffectParallax)
	apply(stageEffectDepth)
	if n := countCalls(logf, "cut"); n != 1 {
		t.Fatalf("cut ran %d times after depth->parallax->depth, want still 1", n)
	}
	if n := countCalls(logf, "inpaint"); n != 1 {
		t.Fatalf("inpaint ran %d times after depth->parallax->depth, want still 1", n)
	}
}

// TestStagePurgedArtifactsRetryOnWake pins the ghost-entry self-heal: a wall
// left ON whose artifacts vanished is re-cut by a plain wake (no force, no
// gen), because the registry says the user still wants the scene.
func TestStagePurgedArtifactsRetryOnWake(t *testing.T) {
	home := stageHome(t)
	logf := writeStageStub(t)

	wall := filepath.Join(home, "w.png")
	writeFile(t, wall, "wp")
	reg := stageWalls{Walls: map[string]stageWall{wall: {Effect: stageEffectDepth}}}
	if err := saveStageWalls(reg); err != nil {
		t.Fatal(err)
	}

	d := &daemon{stageSig: make(chan struct{}, 1)}
	d.showWall(wall)
	d.reconcileStage(false, false)

	if n := countCalls(logf, "cut"); n != 1 {
		t.Fatalf("a purged wall on a plain wake ran %d cuts, want the self-heal to run 1", n)
	}
	if !isFile(stageSubjectOut(wall)) {
		t.Fatal("the self-heal did not produce a subject")
	}
	if f := d.buildStageFrame(); f.Notice != "" {
		t.Fatalf("a healed cut still noticed %q, want silence", f.Notice)
	}
}

// TestStageBlockedCutNotices pins the loud failure: when the engine cannot cut
// (the tier's model is missing), the enable still records the effect but the
// frame carries the helper's reason instead of dying mute, and the next good
// reconcile clears it.
func TestStageBlockedCutNotices(t *testing.T) {
	home := stageHome(t)
	logf := writeStageStub(t)
	t.Setenv("STAGE_STUB_MISSING", "1")
	t.Setenv("STAGE_STUB_CHECK_REASON", "model u2netp missing")

	wall := filepath.Join(home, "w.png")
	writeFile(t, wall, "wp")
	d := &daemon{stageSig: make(chan struct{}, 1)}
	d.showWall(wall)

	d.stageSetEffect(stageEffectDepth)
	d.reconcileStage(d.stageForce.Swap(false), d.stageGen.Swap(false))

	if n := countCalls(logf, "cut"); n != 0 {
		t.Fatalf("an unavailable engine ran %d cuts, want 0", n)
	}
	f := d.buildStageFrame()
	if f.Walls[wall].Effect != stageEffectDepth {
		t.Fatalf("the enable did not record the effect: %+v", f.Walls[wall])
	}
	if f.Notice != "model u2netp missing" {
		t.Fatalf("notice = %q, want the helper's reason", f.Notice)
	}

	// The model lands: a retry cuts, and the notice clears.
	t.Setenv("STAGE_STUB_MISSING", "")
	d.reconcileStage(true, false)
	if n := countCalls(logf, "cut"); n != 1 {
		t.Fatalf("after the model appeared, cut ran %d times, want 1", n)
	}
	if f := d.buildStageFrame(); f.Notice != "" {
		t.Fatalf("notice = %q after a good reconcile, want cleared", f.Notice)
	}
}

// TestStageProbeAsksForTheTierModel pins the model-aware guard: a fine-tier
// box is probed for birefnet-general-lite, not the default draft model, so an
// installed fine tier no longer reads as "engine missing".
func TestStageProbeAsksForTheTierModel(t *testing.T) {
	home := stageHome(t)
	logf := writeStageStub(t)
	cfg := filepath.Join(home, ".config", "ryoku")
	if err := os.MkdirAll(cfg, 0o755); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(cfg, "stage.json"), `{"quality":"fine"}`)

	wall := filepath.Join(home, "w.png")
	writeFile(t, wall, "wp")
	reg := stageWalls{Walls: map[string]stageWall{wall: {Effect: stageEffectDepth}}}
	if err := saveStageWalls(reg); err != nil {
		t.Fatal(err)
	}

	d := &daemon{stageSig: make(chan struct{}, 1)}
	d.showWall(wall)
	d.reconcileStage(false, false)

	b, err := os.ReadFile(logf)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(b), "check birefnet-general-lite") {
		t.Fatalf("the probe did not ask for the fine-tier model, calls:\n%s", b)
	}
	if n := countCalls(logf, "cut"); n != 1 {
		t.Fatalf("a fine-tier box with the model available ran %d cuts, want 1", n)
	}
}

// TestStageClearTakesWallOff pins the clear verb's registry contract: the
// generated artifacts go, the wall records Plain, and the next plain wake does
// not re-cut (an off wall is no ghost for the self-heal).
func TestStageClearTakesWallOff(t *testing.T) {
	home := stageHome(t)
	logf := writeStageStub(t)

	wall := filepath.Join(home, "w.png")
	writeFile(t, wall, "wp")
	d := &daemon{stageSig: make(chan struct{}, 1)}
	d.showWall(wall)
	d.stageSetEffect(stageEffectDepth)
	d.reconcileStage(d.stageForce.Swap(false), d.stageGen.Swap(false))
	cuts := countCalls(logf, "cut")

	d.stageClear()
	d.reconcileStage(false, false)

	if isFile(stageSubjectOut(wall)) {
		t.Fatal("clear left subject.png behind")
	}
	if f := d.buildStageFrame(); f.Walls[wall].Effect != stageEffectOff {
		t.Fatalf("clear left the wall on %+v, want off", f.Walls[wall])
	}
	if n := countCalls(logf, "cut"); n != cuts {
		t.Fatalf("the wake after clear re-cut (%d cuts, was %d), want none", n, cuts)
	}
}

// TestStageCancelSignalsChild pins cancellation: a cut in flight is killed at
// its real child PID, so the reconcile returns and no subject is written.
func TestStageCancelSignalsChild(t *testing.T) {
	home := stageHome(t)
	writeStageStub(t)
	t.Setenv("STAGE_STUB_SLEEP", "5")

	wall := filepath.Join(home, "w.png")
	writeFile(t, wall, "wp")
	reg := stageWalls{Walls: map[string]stageWall{wall: {Effect: stageEffectDepth}}}
	if err := saveStageWalls(reg); err != nil {
		t.Fatal(err)
	}

	d := &daemon{stageSig: make(chan struct{}, 1)}
	d.showWall(wall)

	done := make(chan struct{})
	go func() {
		defer close(done)
		d.reconcileStage(false, true) // gen: runs the (sleeping) cut
	}()

	// Wait for the child to come up, then cancel it.
	deadline := time.Now().Add(4 * time.Second)
	for stageCutPID.Load() == 0 {
		if time.Now().After(deadline) {
			t.Fatal("cut child never started")
		}
		time.Sleep(10 * time.Millisecond)
	}
	stageCancel()

	select {
	case <-done:
	case <-time.After(4 * time.Second):
		t.Fatal("reconcile did not return after cancel")
	}
	if stageCutPID.Load() != 0 {
		t.Fatalf("cut PID not cleared after cancel: %d", stageCutPID.Load())
	}
	if isFile(stageSubjectOut(wall)) {
		t.Fatal("a cancelled cut still produced a subject")
	}
}

// TestStageQualityTiers pins the tier defaults, per-tier overrides, and the
// matting contract the engine calls depend on.
func TestStageQualityTiers(t *testing.T) {
	defaults := defaultStageModels()
	cases := []struct {
		name    string
		tier    string
		models  stageModels
		model   string
		matting bool
	}{
		{"draft default", "draft", defaults, "u2netp", false},
		{"standard default", "standard", defaults, "u2netp", true},
		{"fine default", "fine", defaults, "birefnet-general-lite", true},
		{"draft override", "draft", stageModels{Draft: "silueta"}, "silueta", false},
		{"standard override", "standard", stageModels{Standard: "birefnet-portrait"}, "birefnet-portrait", true},
		{"fine override", "fine", stageModels{Fine: "birefnet-general"}, "birefnet-general", true},
		{"unknown tier", "", stageModels{}, "u2netp", false},
	}
	for _, tc := range cases {
		got := stageQualityFor(tc.tier, tc.models)
		if got.model != tc.model || got.matting != tc.matting {
			t.Errorf("%s: stageQualityFor(%q) = %+v, want {%s %v}", tc.name, tc.tier, got, tc.model, tc.matting)
		}
	}
}

func TestStageConfigReadsModelOverride(t *testing.T) {
	home := stageHome(t)
	logf := writeStageStub(t)
	dir := filepath.Join(home, ".config", "ryoku")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(dir, "stage.json"), `{
  "quality": "fine",
  "models": {
    "draft": "silueta",
    "standard": "u2netp",
    "fine": "birefnet-portrait"
  }
}`)
	got := stageConfig()
	if got.model != "birefnet-portrait" || !got.matting {
		t.Fatalf("stageConfig() = %+v, want portrait with matting", got)
	}
	source := filepath.Join(home, "wall.png")
	out := filepath.Join(home, "subject.png")
	writeFile(t, source, "wall")
	if err := (&daemon{}).engineCut(source, out, got); err != nil {
		t.Fatal(err)
	}
	calls, err := os.ReadFile(logf)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(calls), "--model birefnet-portrait --matting") {
		t.Fatalf("cut did not use the persisted fine override: %s", calls)
	}
}

// TestStageEngineBinSeam pins the resolution order: the RYOKU_STAGE_ENGINE seam
// wins, and a bad RYOKU_SHELL_DIR falls back to the PATH name.
func TestStageEngineBinSeam(t *testing.T) {
	t.Setenv("RYOKU_STAGE_ENGINE", "/opt/ryostage")
	if got := stageEngineBin(); got != "/opt/ryostage" {
		t.Fatalf("engine bin = %q, want the RYOKU_STAGE_ENGINE seam", got)
	}
	t.Setenv("RYOKU_STAGE_ENGINE", "")
	t.Setenv("RYOKU_SHELL_DIR", "/dev/null")
	if got := stageEngineBin(); got != "ryostage" {
		t.Fatalf("engine bin with bad RYOKU_SHELL_DIR = %q, want ryostage", got)
	}
}

// TestStageManualLayers pins the manual-layer naming and ordering contract:
// layer-NN.png in order, subject.png and single-digit names excluded, each
// defaulting to on and behind the widgets.
func TestStageManualLayers(t *testing.T) {
	home := stageHome(t)
	wall := filepath.Join(home, "w.png")
	dir := stageWallDir(wall)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"layer-03.png", "layer-01.png", "layer-02.png", "subject.png", "notes.txt", "layer-1.png"} {
		writeFile(t, filepath.Join(dir, name), name)
	}
	layers := manualLayers(wall)
	if len(layers) != 3 {
		t.Fatalf("manual layers = %d, want 3 (subject.png and single-digit excluded)", len(layers))
	}
	wantLabel := []string{"Layer 1", "Layer 2", "Layer 3"}
	wantSuffix := []string{"layer-01.png", "layer-02.png", "layer-03.png"}
	for i, l := range layers {
		if l.Label != wantLabel[i] {
			t.Errorf("layer[%d] label = %v, want %q", i, l.Label, wantLabel[i])
		}
		if !strings.HasSuffix(l.Out, wantSuffix[i]) {
			t.Errorf("layer[%d] out = %v, want suffix %q", i, l.Out, wantSuffix[i])
		}
		if !l.Enabled || l.Front {
			t.Errorf("layer[%d] defaults = %+v, want enabled & behind", i, l)
		}
	}
}

// TestStageAddLayerNumbering pins add-layer's naming: the next free NN after the
// highest existing layer-NN.png, formatted two-digit.
func TestStageAddLayerNumbering(t *testing.T) {
	home := stageHome(t)
	wall := filepath.Join(home, "w.png")
	writeFile(t, wall, "wp")
	src := filepath.Join(home, "extra.png")
	writeFile(t, src, "X")

	d := &daemon{stageSig: make(chan struct{}, 1)}
	d.showWall(wall)

	dir := stageWallDir(wall)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	// A pre-existing layer-03.png: the next add must be layer-04.png (max+1).
	writeFile(t, filepath.Join(dir, "layer-03.png"), "L3")

	p1, err := d.stageAddLayer(src)
	if err != nil {
		t.Fatal(err)
	}
	if filepath.Base(p1) != "layer-04.png" {
		t.Fatalf("first add = %s, want layer-04.png", filepath.Base(p1))
	}
	p2, err := d.stageAddLayer(src)
	if err != nil {
		t.Fatal(err)
	}
	if filepath.Base(p2) != "layer-05.png" {
		t.Fatalf("second add = %s, want layer-05.png", filepath.Base(p2))
	}
}

// TestStageRemoveLayerRejectsSubject pins the guard: index 0 (the subject) is
// never removable, while a manual layer removes its file.
func TestStageRemoveLayerRejectsSubject(t *testing.T) {
	home := stageHome(t)
	wall := filepath.Join(home, "w.png")
	writeFile(t, wall, "wp")
	if err := saveStageWalls(stageWalls{Walls: map[string]stageWall{wall: {Effect: stageEffectDepth}}}); err != nil {
		t.Fatal(err)
	}

	d := &daemon{stageSig: make(chan struct{}, 1)}
	d.showWall(wall)

	if err := d.stageRemoveLayer("0"); err == nil {
		t.Fatal("remove-layer 0 must be rejected: the subject is not removable")
	}

	dir := stageWallDir(wall)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	layer := filepath.Join(dir, "layer-01.png")
	writeFile(t, layer, "L1")
	if err := d.stageRemoveLayer("1"); err != nil {
		t.Fatalf("remove-layer 1 (a manual layer) failed: %v", err)
	}
	if isFile(layer) {
		t.Fatal("remove-layer 1 did not delete the manual layer file")
	}
}

// TestStageRegistryMigrationV1toV2 pins the one-time registry fold: effect
// `subject` becomes `depth`, the scene order reduces to each layer's front flag
// (a layer after a widget token sits in front), `depthFactor` becomes `depth`,
// unknown per-layer knobs and mode/scene are dropped, a manual wall keeps its
// layer-NN.png after a prepended subject, the marker is written, and a second
// start is a no-op.
func TestStageRegistryMigrationV1toV2(t *testing.T) {
	home := stageHome(t)
	if err := os.MkdirAll(filepath.Join(stateDir(), "ryoku"), 0o755); err != nil {
		t.Fatal(err)
	}
	wpA := filepath.Join(home, "a.png") // depth (was "subject") + one manual layer
	wpB := filepath.Join(home, "b.png") // parallax manual: layers only, no subject entry
	writeFile(t, wpA, "A")
	writeFile(t, wpB, "B")

	subjA := stageSubjectOut(wpA)
	layA2 := filepath.Join(stageWallDir(wpA), "layer-02.png")
	layB1 := filepath.Join(stageWallDir(wpB), "layer-01.png")
	v1 := `{
      "current": "` + wpA + `",
      "walls": {
        "` + wpA + `": {
          "effect": "subject",
          "mode": "auto",
          "scene": ["wallpaper", "layer:1", "widget:clock", "layer:2"],
          "layers": [
            {"out":"` + subjA + `","label":"Subject","depthFactor":0.3,"opacity":0.8,"enabled":true},
            {"out":"` + layA2 + `","label":"Layer 2","depthFactor":0.7,"feather":0.1}
          ]
        },
        "` + wpB + `": {
          "effect": "parallax",
          "mode": "manual",
          "layers": [
            {"out":"` + layB1 + `","label":"Layer 1","depthFactor":0.4}
          ]
        }
      }
    }`
	writeFile(t, stageWallsPath(), v1)

	migrateStage()

	reg := loadStageWalls()
	a := reg.Walls[wpA]
	if a.Effect != stageEffectDepth {
		t.Fatalf("wpA effect = %v, want depth (was subject)", a.Effect)
	}
	if len(a.Layers) != 2 {
		t.Fatalf("wpA layers = %d, want 2", len(a.Layers))
	}
	// Scene reduction: layer:1 (subject, idx0) is before widget:clock -> behind;
	// layer:2 (idx1) is after it -> in front.
	if a.Layers[0].Front {
		t.Fatalf("subject listed before the widget should be behind: %+v", a.Layers[0])
	}
	if !a.Layers[1].Front {
		t.Fatalf("layer after the widget should be in front: %+v", a.Layers[1])
	}
	if a.Layers[0].Depth != 0.3 || a.Layers[1].Depth != 0.7 {
		t.Fatalf("depthFactor not folded to depth: %+v", a.Layers)
	}
	raw, _ := os.ReadFile(stageWallsPath())
	for _, gone := range []string{"depthFactor", "opacity", "feather", "\"mode\"", "\"scene\""} {
		if strings.Contains(string(raw), gone) {
			t.Fatalf("v2 registry still carries the retired %q: %s", gone, raw)
		}
	}

	// A manual wall keeps its layer-NN.png after a prepended subject slot.
	b := reg.Walls[wpB]
	if b.Effect != stageEffectParallax {
		t.Fatalf("wpB effect = %v, want parallax", b.Effect)
	}
	if len(b.Layers) != 2 {
		t.Fatalf("wpB layers = %d, want 2 (subject + kept manual)", len(b.Layers))
	}
	if filepath.Base(b.Layers[0].Out) != "subject.png" || b.Layers[0].Label != "Subject" {
		t.Fatalf("wpB layer[0] is not the prepended subject: %+v", b.Layers[0])
	}
	if filepath.Base(b.Layers[1].Out) != "layer-01.png" {
		t.Fatalf("wpB layer[1] is not the kept manual: %+v", b.Layers[1])
	}

	if !isFile(stageMigrationMarker()) {
		t.Fatal("v2 migration marker not written")
	}
	before, _ := os.ReadFile(stageWallsPath())
	migrateStage()
	after, _ := os.ReadFile(stageWallsPath())
	if string(before) != string(after) {
		t.Fatal("second migration mutated the registry (not a no-op)")
	}
}

// TestStageSettingsFoldV1toV2 pins the one-time stage.json fold: feather becomes
// edge, mouse:false folds to motion.amount subtle, lift/preset drop, idle/music
// default, quality/shadow/front carry, and an already-v2 file is left alone.
func TestStageSettingsFoldV1toV2(t *testing.T) {
	home := stageHome(t)
	cfg := filepath.Join(home, ".config", "ryoku")
	if err := os.MkdirAll(cfg, 0o755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(cfg, "stage.json")
	writeFile(t, path, `{"quality":"fine","feather":0.2,"lift":1,"shadow":0.3,"shadowAngle":45,`+
		`"motion":{"mouse":false,"sensitivity":2,"range":0.3,"wallpaper":0.2},`+
		`"preset":"deep","front":["clock"]}`)

	migrateStageSettings()

	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var got map[string]any
	if err := json.Unmarshal(b, &got); err != nil {
		t.Fatal(err)
	}
	if got["edge"] != 0.2 {
		t.Fatalf("feather did not fold to edge=0.2: %v", got["edge"])
	}
	if got["quality"] != "fine" || got["shadow"] != 0.3 {
		t.Fatalf("carried keys wrong: quality=%v shadow=%v", got["quality"], got["shadow"])
	}
	for _, gone := range []string{"feather", "lift", "preset"} {
		if _, ok := got[gone]; ok {
			t.Fatalf("v1-only key %q not dropped: %s", gone, b)
		}
	}
	motion, _ := got["motion"].(map[string]any)
	if motion["amount"] != "subtle" {
		t.Fatalf("mouse:false must fold to amount=subtle, got %v", motion["amount"])
	}
	if motion["idle"] != "none" || motion["music"] != false {
		t.Fatalf("motion idle/music not defaulted: %+v", motion)
	}
	for _, gone := range []string{"mouse", "sensitivity", "range", "wallpaper"} {
		if _, ok := motion[gone]; ok {
			t.Fatalf("motion sub-knob %q not dropped: %s", gone, b)
		}
	}
	front, _ := got["front"].([]any)
	if len(front) != 1 || front[0] != "clock" {
		t.Fatalf("front = %v, want [clock]", got["front"])
	}

	// An already-v2 stage.json is left byte-for-byte untouched.
	v2 := `{"quality":"draft","edge":0.1,"shadow":0,"shadowAngle":90,` +
		`"motion":{"amount":"strong","idle":"float","music":true},"front":[]}`
	writeFile(t, path, v2)
	migrateStageSettings()
	after, _ := os.ReadFile(path)
	if string(after) != v2 {
		t.Fatalf("already-v2 stage.json was rewritten:\n got %s\nwant %s", after, v2)
	}
}

// TestStageRyogamiFrameWake pins the bridge trigger: a frame showing a new
// wallpaper, one that lost its subject fold, or a live claim wakes the stage
// worker, while the frame our own publish produces (same sources, subject
// folded) stays quiet, so the publish-subscribe loop settles.
func TestStageRyogamiFrameWake(t *testing.T) {
	d := &daemon{stageSig: make(chan struct{}, 1)}
	woke := func() bool {
		select {
		case <-d.stageSig:
			return true
		default:
			return false
		}
	}
	feed := func(js string) { d.consumeRyogamiFrames(strings.NewReader(js + "\n")) }

	feed(`{"default":{"path":"/w/a.png"},"outputs":{}}`)
	if !woke() {
		t.Fatal("a new wallpaper did not wake the stage worker")
	}
	feed(`{"default":{"path":"/w/a.png","depth":"/d/a-subject.png"},"outputs":{}}`)
	if woke() {
		t.Fatal("our own subject fold woke the worker again")
	}
	feed(`{"default":{"path":"/w/a.png"},"outputs":{}}`)
	if !woke() {
		t.Fatal("a re-set that dropped the subject fold did not wake the worker")
	}
	feed(`{"default":{"path":"/w/a.png","live":true},"outputs":{}}`)
	if !woke() {
		t.Fatal("a live claim did not wake the worker")
	}
}

// TestStageLegacyMigrationV2 pins the pre-v1 fold a stable box takes jumping
// straight to v2: the retired Depth (depth-walls.json + Pictures/Depth) and
// Parallax (layers.pz + Pictures/Parallax) land as one v2 registry with
// artifacts moved by rename and a v2 stage.json, the marker is written, and a
// second start is a no-op.
func TestStageLegacyMigrationV2(t *testing.T) {
	home := stageHome(t)
	if err := os.MkdirAll(filepath.Join(stateDir(), "ryoku"), 0o755); err != nil {
		t.Fatal(err)
	}
	cfg := filepath.Join(home, ".config", "ryoku")
	if err := os.MkdirAll(cfg, 0o755); err != nil {
		t.Fatal(err)
	}

	wpA := filepath.Join(home, "walls", "a.png") // depth-only
	wpB := filepath.Join(home, "walls", "b.png") // parallax auto
	if err := os.MkdirAll(filepath.Dir(wpA), 0o755); err != nil {
		t.Fatal(err)
	}
	writeFile(t, wpA, "A")
	writeFile(t, wpB, "B")

	// Retired depth-walls.json (per-wall opt-in) + its cutout.
	writeFile(t, legacyDepthWallsPath(), `{"current":true,"walls":{"`+wpA+`":true}}`)
	depthPNG := filepath.Join(legacyDepthDir(), "a-depth.png")
	if err := os.MkdirAll(filepath.Dir(depthPNG), 0o755); err != nil {
		t.Fatal(err)
	}
	writeFile(t, depthPNG, "OLDCUT")

	// Retired parallax layers.pz + its folder. The scene lifts layer 2 in front
	// of the widgets; the subject (layer 1) stays behind.
	pbDir := filepath.Join(legacyParallaxDir(), "b")
	if err := os.MkdirAll(pbDir, 0o755); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(pbDir, "layer-01.png"), "OLDSUBJECT")
	writeFile(t, filepath.Join(pbDir, "layer-02.png"), "OLDLAYER2")
	writeFile(t, filepath.Join(pbDir, "background.png"), "OLDBG")
	lp := legacyParallaxWalls{
		Walls: map[string]legacyParallaxWall{
			wpB: {Enabled: true, Mode: "auto", Scene: []string{"wallpaper", "layer:1", "widget:clock", "layer:2"}},
		},
		Layers: map[string][]legacyParallaxLayer{
			wpB: {
				{Out: filepath.Join(pbDir, "layer-01.png"), Rev: 1, Label: "subject", Depth: 0.5},
				{Out: filepath.Join(pbDir, "layer-02.png"), Rev: 2, Label: "Layer 2", Depth: 0.7},
			},
		},
	}
	lpBytes, _ := json.Marshal(lp)
	writeFile(t, legacyLayersPath(), string(lpBytes))

	// Retired settings: parallax's model+matting wins the derived quality (fine),
	// and a user's front from depth.json survives.
	writeFile(t, filepath.Join(cfg, "depth.json"), `{"model":"u2netp","alphaMatting":false,"front":["clock"]}`)
	writeFile(t, filepath.Join(cfg, "parallax.json"), `{"mode":"auto","model":"birefnet-general-lite","alphaMatting":true}`)

	migrateStage()

	reg := loadStageWalls()
	if reg.Walls[wpA].Effect != stageEffectDepth {
		t.Fatalf("wpA effect = %v, want depth", reg.Walls[wpA].Effect)
	}
	if a := reg.Walls[wpA]; len(a.Layers) == 0 ||
		filepath.Base(a.Layers[0].Out) != "subject.png" || a.Layers[0].Label != "Subject" {
		t.Fatalf("wpA layers[0] is not the subject: %+v", a.Layers)
	}
	b := reg.Walls[wpB]
	if b.Effect != stageEffectParallax {
		t.Fatalf("wpB effect = %v, want parallax", b.Effect)
	}
	if len(b.Layers) != 2 {
		t.Fatalf("wpB layers = %d, want 2", len(b.Layers))
	}
	if filepath.Base(b.Layers[0].Out) != "subject.png" || b.Layers[0].Label != "Subject" {
		t.Fatalf("wpB layer[0] is not the rewritten subject: %+v", b.Layers[0])
	}
	if b.Layers[0].Depth != 0.5 {
		t.Fatalf("wpB subject depth = %v, want 0.5 (from the legacy layer)", b.Layers[0].Depth)
	}
	if filepath.Base(b.Layers[1].Out) != "layer-02.png" {
		t.Fatalf("wpB layer[1] out = %v", b.Layers[1].Out)
	}
	// Scene reduction: the subject (layer:1) is behind the widget; layer 2 is in front.
	if b.Layers[0].Front {
		t.Fatalf("wpB subject should be behind the widgets: %+v", b.Layers[0])
	}
	if !b.Layers[1].Front {
		t.Fatalf("wpB layer 2 should be in front (listed after the widget): %+v", b.Layers[1])
	}

	// Artifact tree: renamed, sources gone.
	if !isFile(stageSubjectOut(wpA)) {
		t.Fatal("depth cutout did not become Stage/a/subject.png")
	}
	if isFile(depthPNG) {
		t.Fatal("old depth cutout was copied, not moved")
	}
	if !isFile(stageSubjectOut(wpB)) || !isFile(stageBackgroundOut(wpB)) ||
		!isFile(filepath.Join(stageWallDir(wpB), "layer-02.png")) {
		t.Fatal("parallax folder did not move whole into Stage/b/")
	}
	if isDir(pbDir) {
		t.Fatal("old parallax folder was copied, not moved")
	}

	// Settings fold: v2 shape, quality fine, front carried, v1 keys gone.
	sb, err := os.ReadFile(filepath.Join(cfg, "stage.json"))
	if err != nil {
		t.Fatalf("stage.json not written: %v", err)
	}
	var settings map[string]any
	if err := json.Unmarshal(sb, &settings); err != nil {
		t.Fatal(err)
	}
	if settings["quality"] != "fine" {
		t.Fatalf("quality = %v, want fine (parallax birefnet+matting)", settings["quality"])
	}
	if _, ok := settings["edge"]; !ok {
		t.Fatal("stage.json missing the v2 edge key")
	}
	for _, gone := range []string{"feather", "lift", "preset", "model"} {
		if _, ok := settings[gone]; ok {
			t.Fatalf("v2 stage.json still carries %q: %s", gone, sb)
		}
	}
	front, _ := settings["front"].([]any)
	if len(front) != 1 || front[0] != "clock" {
		t.Fatalf("front = %v, want [clock] carried from depth.json", settings["front"])
	}

	if !isFile(stageMigrationMarker()) {
		t.Fatal("migration marker not written")
	}
	before, _ := os.ReadFile(stageWallsPath())
	migrateStage()
	after, _ := os.ReadFile(stageWallsPath())
	if string(before) != string(after) {
		t.Fatal("second migration mutated the registry (not a no-op)")
	}
}

// TestStageLegacySettingsFoldV2 pins the retired depth.json + parallax.json fold
// when a stable box has no stage.json: the depth scalar wins over a parallax
// per-layer array, feather -> edge, and the higher model+matting tier wins the
// quality (fine), with the user's shadow 0.85 intact.
func TestStageLegacySettingsFoldV2(t *testing.T) {
	home := stageHome(t)
	cfg := filepath.Join(home, ".config", "ryoku")
	if err := os.MkdirAll(cfg, 0o755); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(cfg, "parallax.json"),
		`{"feather":[0],"lift":[0],"shadow":[0],"shadowAngle":[331],"model":"u2netp"}`)
	writeFile(t, filepath.Join(cfg, "depth.json"),
		`{"feather":0.15,"lift":1,"shadow":0.85,"model":"birefnet-general-lite","alphaMatting":true}`)

	migrateStageSettings()

	b, err := os.ReadFile(filepath.Join(cfg, "stage.json"))
	if err != nil {
		t.Fatal(err)
	}
	var got map[string]any
	if err := json.Unmarshal(b, &got); err != nil {
		t.Fatal(err)
	}
	if got["shadow"] != 0.85 {
		t.Fatalf("shadow = %v, want the depth scalar 0.85", got["shadow"])
	}
	if got["edge"] != 0.15 {
		t.Fatalf("edge = %v, want feather 0.15", got["edge"])
	}
	if got["quality"] != "fine" {
		t.Fatalf("quality = %v, want fine (birefnet+matting)", got["quality"])
	}
	if _, isList := got["shadowAngle"].([]any); isList {
		t.Fatalf("shadowAngle folded as a list: %s", b)
	}
	if _, ok := got["feather"]; ok {
		t.Fatalf("feather not dropped: %s", b)
	}
}
