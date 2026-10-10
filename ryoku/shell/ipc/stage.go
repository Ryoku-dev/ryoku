package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync/atomic"
	"syscall"
	"time"
)

// Ryostage v2 (docs/stage.md): the desktop as a stage. One worker, one registry,
// one topic. Depth is the stage with a single still layer in front of the
// widgets; Parallax is the same stack with drift and a recoloured backdrop.
// There is no "mode", no per-layer look knobs and no scene order: a wall is one
// effect and one layer stack, layers[0] is always the cut subject, and every
// idea (edge, shadow, quality, motion) lives once in stage.json. Both effects
// share the ryostage cut-out engine, the per-wall registry, and the artifact
// tree under ~/Pictures/Stage. Generation is slow, so it runs on a coalescing
// worker off the wallpaper hot path; the finished subject is folded into
// ryogami's wallpaper frame under the unchanged `depth` wire for pixel-lock
// (Depth only), and the full editor state rides the `stage` topic QML renders
// from.

type stageEffect string

const (
	stageEffectOff      stageEffect = "off"
	stageEffectDepth    stageEffect = "depth"
	stageEffectParallax stageEffect = "parallax"
)

// stageLayer is one entry in a wall's layer stack. layers[0] is always the cut
// subject; every later entry is a picture the user added. The daemon derives the
// identity keys (out, label) from the filesystem on every reconcile and owns the
// three arrangement flags: enabled (on/off), front (in front of / behind the
// widgets) and depth (near..far drift for Parallax, ignored in Depth). That is
// the whole per-layer schema; the v1 look overrides, offsets, animation and
// audio knobs are gone.
type stageLayer struct {
	Out     string  `json:"out"`
	Label   string  `json:"label"`
	Enabled bool    `json:"enabled"`
	Front   bool    `json:"front"`
	Depth   float64 `json:"depth"`
}

// stageWall is a wallpaper's persisted stage: which effect is on and the layer
// stack with its arrangement flags. Per-wall, because a cut belongs to one image
// and the user's arrangement belongs to it.
type stageWall struct {
	Effect stageEffect  `json:"effect"`
	Layers []stageLayer `json:"layers,omitempty"`
}

// stageWalls is the daemon-owned registry at ~/.local/state/ryoku/stage-walls.json.
// `current` mirrors the wallpaper on screen now so the shell can key its lookup.
type stageWalls struct {
	Current string               `json:"current"`
	Walls   map[string]stageWall `json:"walls"`
}

// stageIndex records what produced a wall's subject.png, so a returning
// wallpaper reuses its cut instantly (mtime) while a quality change re-cuts.
type stageIndex struct {
	Source  string `json:"source"`
	Model   string `json:"model"`
	Matting bool   `json:"matting"`
}

// stageQuality is the model+matting pair a quality tier resolves to.
type stageQuality struct {
	model   string
	matting bool
}

type stageModels struct {
	Draft    string `json:"draft"`
	Standard string `json:"standard"`
	Fine     string `json:"fine"`
}

type stageTarget struct {
	slot   string
	source string
}

// stageWallFrame and stageFrame are the `stage` topic shape QML binds to
// (per-wallpaper keyed so each monitor reads its own entry). busy/stage/percent
// are global to the single worker; everything else is per wall. `rev` is the max
// mtime across the wall's artifacts so the shell can cache-bust every PNG url
// (subject, background and each layer) with one revision.
type stageWallFrame struct {
	Effect     stageEffect  `json:"effect"`
	Subject    string       `json:"subject"`
	Background string       `json:"background"`
	Rev        int64        `json:"rev"`
	Layers     []stageLayer `json:"layers"`
}

type stageFrame struct {
	Current string                    `json:"current"`
	Busy    bool                      `json:"busy"`
	Stage   string                    `json:"stage"`
	Percent int                       `json:"percent"`
	Notice  string                    `json:"notice,omitempty"`
	Walls   map[string]stageWallFrame `json:"walls"`
}

var manualLayerRe = regexp.MustCompile(`^layer-(\d{2,})\.png$`)

// stageCutPID holds the running engine child's real PID (not the daemon's) so
// `stage cancel` can signal its process group; reset to 0 once it exits.
var stageCutPID atomic.Int32

func logStage(what string, err error) {
	if err != nil {
		fmt.Fprintf(os.Stderr, "stage: %s: %v\n", what, err)
		return
	}
	fmt.Fprintf(os.Stderr, "stage: %s\n", what)
}

// --- paths -----------------------------------------------------------------

func stageWallsPath() string { return filepath.Join(stateDir(), "ryoku", "stage-walls.json") }
func stageDir() string       { return filepath.Join(os.Getenv("HOME"), "Pictures", "Stage") }

func stageStem(source string) string {
	base := filepath.Base(source)
	return strings.TrimSuffix(base, filepath.Ext(base))
}

func stageWallDir(source string) string    { return filepath.Join(stageDir(), stageStem(source)) }
func stageSubjectOut(source string) string { return filepath.Join(stageWallDir(source), "subject.png") }
func stageBackgroundOut(source string) string {
	return filepath.Join(stageWallDir(source), "background.png")
}
func stageIndexPath(source string) string { return filepath.Join(stageWallDir(source), ".index.json") }
func stageProgressPath() string           { return filepath.Join(stateDir(), "ryoku", "stage", "progress") }

// --- shared fs helpers -----------------------------------------------------

func fileModTime(p string) int64 {
	if st, err := os.Stat(p); err == nil {
		return st.ModTime().Unix()
	}
	return 0
}

func isDir(p string) bool {
	st, err := os.Stat(p)
	return err == nil && st.IsDir()
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		return err
	}
	return out.Close()
}

// writeJSONFileAtomic writes v as indented JSON via a temp file + rename in the
// same directory, so a crash never leaves a half-written registry or settings.
func writeJSONFileAtomic(path string, v any) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	tmp, err := os.CreateTemp(dir, ".stage-*.tmp")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	if _, err := tmp.Write(b); err != nil {
		tmp.Close()
		os.Remove(tmpName)
		return err
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmpName)
		return err
	}
	if err := os.Chmod(tmpName, 0o644); err != nil {
		os.Remove(tmpName)
		return err
	}
	if err := os.Rename(tmpName, path); err != nil {
		os.Remove(tmpName)
		return err
	}
	return nil
}

// --- registry --------------------------------------------------------------

func loadStageWalls() stageWalls {
	var w stageWalls
	if b, err := os.ReadFile(stageWallsPath()); err == nil {
		_ = json.Unmarshal(b, &w)
	}
	// Normalize after any read or decode failure so callers can assign into the
	// map without a nil-map panic.
	if w.Walls == nil {
		w.Walls = map[string]stageWall{}
	}
	return w
}

func saveStageWalls(w stageWalls) error {
	if w.Walls == nil {
		w.Walls = map[string]stageWall{}
	}
	return writeJSONFileAtomic(stageWallsPath(), w)
}

func loadStageIndex(source string) stageIndex {
	var idx stageIndex
	if b, err := os.ReadFile(stageIndexPath(source)); err == nil {
		_ = json.Unmarshal(b, &idx)
	}
	return idx
}

func saveStageIndex(source string, idx stageIndex) {
	if err := writeJSONFileAtomic(stageIndexPath(source), idx); err != nil {
		logStage("save index", err)
	}
}

// --- settings --------------------------------------------------------------

// stageConfig reads the shell-owned quality tier and its per-tier model choices
// from stage.json. Everything else in the file is rendered by the shell.
func stageConfig() stageQuality {
	models := defaultStageModels()
	def := stageQualityFor("draft", models)
	dir := ryokuConfigDir()
	if dir == "" {
		return def
	}
	b, err := os.ReadFile(filepath.Join(dir, "stage.json"))
	if err != nil {
		return def
	}
	var m struct {
		Quality string      `json:"quality"`
		Models  stageModels `json:"models"`
	}
	if json.Unmarshal(b, &m) != nil {
		return def
	}
	return stageQualityFor(m.Quality, m.Models)
}

func defaultStageModels() stageModels {
	return stageModels{
		Draft:    "u2netp",
		Standard: "u2netp",
		Fine:     "birefnet-general-lite",
	}
}

// stageQualityFor keeps each tier's matting contract while allowing its model
// to be replaced by a model selected from the engine catalogue.
func stageQualityFor(tier string, models stageModels) stageQuality {
	defaults := defaultStageModels()
	var model string
	var matting bool
	switch tier {
	case "standard":
		model, matting = models.Standard, true
		if model == "" {
			model = defaults.Standard
		}
	case "fine":
		model, matting = models.Fine, true
		if model == "" {
			model = defaults.Fine
		}
	default:
		model = models.Draft
		if model == "" {
			model = defaults.Draft
		}
	}
	return stageQuality{model: model, matting: matting}
}

// --- engine ----------------------------------------------------------------

// stageEngineBin resolves the ryostage helper. RYOKU_STAGE_ENGINE is the test /
// out-of-tree seam; a dev run reaches it under RYOKU_SHELL_DIR; otherwise it is
// on PATH once packaged.
func stageEngineBin() string {
	if bin := os.Getenv("RYOKU_STAGE_ENGINE"); bin != "" {
		return bin
	}
	if dir := os.Getenv("RYOKU_SHELL_DIR"); dir != "" {
		p := filepath.Join(dir, "scripts", "ryostage")
		if isFile(p) {
			return p
		}
	}
	return "ryostage"
}

// stageEngineProbe asks the helper whether it can cut with `model` right now,
// on a deadline so a hung helper never wedges the worker. It returns the
// helper's one-line reason ("runtime missing", "model u2netp missing") so a
// blocked pipeline can name itself instead of dying mute.
func stageEngineProbe(model string) (bool, string) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, stageEngineBin(), "check", model)
	out, err := cmd.CombinedOutput()
	if err == nil {
		return true, ""
	}
	if reason := strings.TrimSpace(string(out)); reason != "" {
		return false, reason
	}
	return false, "engine unavailable"
}

// stageModelsJSON passes the curated catalogue through to the shell.
func stageModelsJSON() string {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	out, err := exec.CommandContext(ctx, stageEngineBin(), "models", "--json").Output()
	if err != nil {
		return "err stage models: " + err.Error()
	}
	return strings.TrimSpace(string(out))
}

// runEngine runs one engine subcommand bounded and cancellable. The child is its
// own process group so `stage cancel` (and a deadline) can signal the whole
// tree (bash + python), not just the leader. stderr is drained so a chatty
// helper cannot block on a full pipe.
func (d *daemon) runEngine(timeout time.Duration, args ...string) error {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, stageEngineBin(), args...)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.Cancel = func() error {
		if cmd.Process != nil {
			return syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
		}
		return nil
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return err
	}
	done := make(chan struct{})
	go func() {
		defer close(done)
		_, _ = io.Copy(io.Discard, stderr)
	}()
	if err := cmd.Start(); err != nil {
		return err
	}
	stageCutPID.Store(int32(cmd.Process.Pid))
	werr := cmd.Wait()
	stageCutPID.Store(0)
	<-done
	return werr
}

// stageCancel kills the running engine child's process group.
func stageCancel() {
	pid := int(stageCutPID.Load())
	if pid <= 0 {
		return
	}
	_ = syscall.Kill(-pid, syscall.SIGKILL)
}

// --- progress --------------------------------------------------------------

func resetStageProgress() {
	p := stageProgressPath()
	_ = os.MkdirAll(filepath.Dir(p), 0o755)
	_ = os.WriteFile(p, nil, 0o644)
}

func writeStageProgress(rec map[string]any) {
	p := stageProgressPath()
	_ = os.MkdirAll(filepath.Dir(p), 0o755)
	b, err := json.Marshal(rec)
	if err != nil {
		return
	}
	f, err := os.OpenFile(p, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return
	}
	defer f.Close()
	_, _ = f.Write(append(b, '\n'))
}

func readLastStagePhase() string {
	b, err := os.ReadFile(stageProgressPath())
	if err != nil {
		return ""
	}
	phase := ""
	for _, line := range strings.Split(string(b), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || !strings.HasPrefix(line, "{") {
			continue
		}
		var rec map[string]any
		if json.Unmarshal([]byte(line), &rec) != nil {
			continue
		}
		if p, ok := rec["phase"].(string); ok {
			phase = p
		}
	}
	return phase
}

// readLastStageError returns the error the last failed engine phase recorded
// ("" when the last phase was not an error), so a blocked cut can name the
// real failure instead of pointing at the log.
func readLastStageError() string {
	b, err := os.ReadFile(stageProgressPath())
	if err != nil {
		return ""
	}
	phase, msg := "", ""
	for _, line := range strings.Split(string(b), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || !strings.HasPrefix(line, "{") {
			continue
		}
		var rec map[string]any
		if json.Unmarshal([]byte(line), &rec) != nil {
			continue
		}
		phase, _ = rec["phase"].(string)
		msg, _ = rec["error"].(string)
	}
	if phase == "error" {
		return msg
	}
	return ""
}

// --- reuse rules -----------------------------------------------------------

// subjectFresh reports whether a generated artifact is no older than its source,
// so a returning wallpaper reuses it but an edited image regenerates.
func subjectFresh(source, out string) bool {
	ot := fileModTime(out)
	return ot > 0 && ot >= fileModTime(source)
}

func backgroundFresh(source, out string) bool { return subjectFresh(source, out) }

// subjectMatches is the stricter generate-time check: fresh AND cut with the
// requested model+matting, so an enable skips a redundant re-cut but a quality
// change (whose model+matting no longer match the index) re-cuts.
func (d *daemon) subjectMatches(source, out string, q stageQuality) bool {
	if !subjectFresh(source, out) {
		return false
	}
	idx := loadStageIndex(source)
	return idx.Source == source && idx.Model == q.model && idx.Matting == q.matting
}

// --- layers ----------------------------------------------------------------

// newSubjectLayer is the always-present layers[0]: the cut subject, in front of
// the widgets, mid drift. It names the subject.png slot even before the cut
// lands, so the stack invariant (layers[0] is the subject) always holds.
func newSubjectLayer(out string) stageLayer {
	return stageLayer{Out: out, Label: "Subject", Enabled: true, Front: true, Depth: 0.5}
}

// manualLayers lists the user-placed layer-NN.png in a wall's folder, ordered,
// each defaulting to on, behind the widgets, mid drift.
func manualLayers(source string) []stageLayer {
	dir := stageWallDir(source)
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	type found struct {
		idx   int
		layer stageLayer
	}
	var fs []found
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		m := manualLayerRe.FindStringSubmatch(e.Name())
		if m == nil {
			continue
		}
		var idx int
		_, _ = fmt.Sscanf(m[1], "%d", &idx)
		out := filepath.Join(dir, e.Name())
		fs = append(fs, found{idx, stageLayer{
			Out: out, Label: fmt.Sprintf("Layer %d", idx), Enabled: true, Front: false, Depth: 0.5,
		}})
	}
	sort.Slice(fs, func(i, j int) bool { return fs[i].idx < fs[j].idx })
	out := make([]stageLayer, 0, len(fs))
	for _, f := range fs {
		out = append(out, f.layer)
	}
	return out
}

// nextManualIndex is the next free NN for a wall folder's layer-NN.png files.
func nextManualIndex(dir string) int {
	next := 1
	if entries, err := os.ReadDir(dir); err == nil {
		for _, e := range entries {
			m := manualLayerRe.FindStringSubmatch(e.Name())
			if m == nil {
				continue
			}
			var idx int
			_, _ = fmt.Sscanf(m[1], "%d", &idx)
			if idx >= next {
				next = idx + 1
			}
		}
	}
	return next
}

// deriveLayers rebuilds a wall's layer stack from the filesystem (subject.png
// then the ordered layer-NN.png) and carries the user's arrangement flags
// forward from the previous registry entry, matched by output path. Identity is
// always the truth on disk; the flags are the user's and are never dropped.
func (d *daemon) deriveLayers(source string, prev []stageLayer) []stageLayer {
	next := []stageLayer{newSubjectLayer(stageSubjectOut(source))}
	next = append(next, manualLayers(source)...)
	byOut := map[string]stageLayer{}
	for _, p := range prev {
		byOut[p.Out] = p
	}
	for i := range next {
		if p, ok := byOut[next[i].Out]; ok {
			next[i].Enabled = p.Enabled
			next[i].Front = p.Front
			next[i].Depth = p.Depth
		}
	}
	return next
}

// --- worker ----------------------------------------------------------------

func (d *daemon) scheduleStage() {
	select {
	case d.stageSig <- struct{}{}:
	default:
	}
}

func (d *daemon) stageWorker() {
	for range d.stageSig {
		d.reconcileStage(d.stageForce.Swap(false), d.stageGen.Swap(false))
	}
}

// stageTargets are the still wallpapers on screen (default + per-output),
// skipping videos and live-claimed slots, each paired with its ryogami slot.
func (d *daemon) stageTargets() []stageTarget {
	f := d.wallFrame()
	var out []stageTarget
	if p := f.Default.Path; p != "" && !f.Default.Live && !f.Default.Video && !isVideo(p) && isFile(p) {
		out = append(out, stageTarget{"", p})
	}
	for name, e := range f.Outputs {
		if e.Path != "" && !e.Live && !e.Video && !isVideo(e.Path) && isFile(e.Path) {
			out = append(out, stageTarget{name, e.Path})
		}
	}
	return out
}

// reconcileStage resolves every on-screen wallpaper's stage from the registry.
// A plain wake (neither force nor gen) reuses artifacts and never runs the
// engine, so a switch reuses a stage instantly but never auto-generates; gen
// (an enable) reuses when present and only generates what is missing; force (a
// quality change, refresh, or manual re-cut) regenerates. The finished subject
// is folded to ryogami under the `depth` wire for the Depth effect only;
// Parallax renders from the topic and clears the fold so the backdrop covers the
// wallpaper's own subject. A failure leaves the effect off with a logged reason
// and is never fatal.
func (d *daemon) reconcileStage(force, gen bool) {
	wall := d.currentWall()
	reg := loadStageWalls()
	if reg.Current != wall {
		reg.Current = wall
		if err := saveStageWalls(reg); err != nil {
			logStage("save current", err)
		}
	}
	q := stageConfig()
	// Probe the engine only when some wall can actually need a cut, and ask
	// about the model the configured tier uses: a box that installed the fine
	// tier must not read as "engine missing" just because the draft model is
	// absent, and a box that cannot cut must say why.
	probed, available := false, false
	reason := ""
	probe := func() bool {
		if !probed {
			available, reason = stageEngineProbe(q.model)
			probed = true
		}
		return available
	}
	notice := ""
	dirty := false
	subjectPublished := false
	for _, t := range d.stageTargets() {
		e := reg.Walls[t.source]
		effect := e.Effect
		if effect == "" || effect == stageEffectOff {
			continue
		}
		// A wall switched on whose artifacts vanished (a hand-purged cache, a
		// cut that died mid-flight) is retried on any wake: the registry says
		// the user wants the scene, so the missing file is the work.
		missing := !subjectFresh(t.source, stageSubjectOut(t.source))
		if !missing && effect == stageEffectParallax {
			missing = !backgroundFresh(t.source, stageBackgroundOut(t.source))
		}
		genHere := gen || force || missing
		avail := false
		if genHere {
			avail = probe()
		}
		prevBytes, _ := json.Marshal(e)
		subjectExists := d.ensureArtifacts(t.source, effect, q, force, genHere, avail)
		if !subjectExists && notice == "" {
			switch {
			case genHere && !avail:
				notice = reason
			case genHere && avail:
				if cutErr := readLastStageError(); cutErr != "" {
					notice = cutErr
				} else {
					notice = "the last cut failed"
				}
			}
		}
		e.Layers = d.deriveLayers(t.source, e.Layers)
		reg.Walls[t.source] = e
		if newBytes, _ := json.Marshal(e); !bytes.Equal(prevBytes, newBytes) {
			dirty = true
		}
		if effect == stageEffectDepth && subjectExists {
			d.depthPublish(t.slot, t.source, stageSubjectOut(t.source))
			subjectPublished = true
		}
	}
	// No subject folded anywhere: clear the wallpaper-frame overlay (a switch to
	// a parallax/off wall must not leave a stale static subject behind).
	if !subjectPublished {
		d.depthClear()
	}
	if dirty {
		if err := saveStageWalls(reg); err != nil {
			logStage("save walls", err)
		}
	}
	d.stageNotice.Store(notice)
	d.stageBusy.Store(false)
	d.publishStage()
}

// ensureArtifacts brings one wall's artifacts in line with its effect, running
// the engine only when a generation is warranted and the runtime is available,
// and returns whether a fresh subject exists. Any effect but off needs the cut;
// only Parallax needs the inpainted backdrop, made once (or re-made on a re-cut
// or force). A cut failure returns no subject and is logged, never fatal.
func (d *daemon) ensureArtifacts(source string, effect stageEffect, q stageQuality, force, gen, available bool) bool {
	subj := stageSubjectOut(source)
	bg := stageBackgroundOut(source)
	subjectExists := false
	didCut := false

	if force || (gen && !d.subjectMatches(source, subj, q)) {
		if available {
			if err := d.runCut(source, subj, q); err != nil {
				logStage("cut "+stageStem(source), err)
				return false
			}
			saveStageIndex(source, stageIndex{Source: source, Model: q.model, Matting: q.matting})
			subjectExists, didCut = true, true
		} else {
			subjectExists = subjectFresh(source, subj)
		}
	} else {
		subjectExists = subjectFresh(source, subj)
	}

	// The Parallax backdrop: inpaint the subject's hole once, or again when the
	// subject was re-cut or a force is in flight. Never for Depth, and never
	// without a subject to hole out.
	if effect == stageEffectParallax && subjectExists {
		if (didCut || force || !backgroundFresh(source, bg)) && available {
			if !didCut {
				resetStageProgress()
			}
			if err := d.runInpaint(source, subj, bg); err != nil {
				// Inpaint failing is non-fatal: the subject still drifts over the
				// original wallpaper; the daemon logs and the surface falls back
				// to no recoloured backdrop.
				logStage("inpaint "+stageStem(source), err)
			}
		}
	}

	if didCut {
		writeStageProgress(map[string]any{"phase": "done"})
	}
	return subjectExists
}

// engineCut writes a subject/layer cut as an alpha-matted PNG, never leaving a
// partial file on failure.
func (d *daemon) engineCut(source, out string, q stageQuality) error {
	if err := os.MkdirAll(filepath.Dir(out), 0o755); err != nil {
		return err
	}
	args := []string{"cut", source, out, "--model", q.model}
	if q.matting {
		args = append(args, "--matting")
	}
	if err := d.runEngine(5*time.Minute, args...); err != nil {
		return err
	}
	if !isFile(out) {
		return fmt.Errorf("cut png missing")
	}
	return nil
}

// runCut writes the wallpaper's subject, framing the engine call with progress
// and the busy flag for the topic (cleared by reconcileStage).
func (d *daemon) runCut(source, out string, q stageQuality) error {
	resetStageProgress()
	d.stageBusy.Store(true)
	writeStageProgress(map[string]any{"phase": "cut", "model": q.model})
	d.publishStage()
	if err := d.engineCut(source, out, q); err != nil {
		writeStageProgress(map[string]any{"phase": "error", "error": err.Error()})
		return err
	}
	return nil
}

// runInpaint fills the subject's hole with the surrounding colour (the parallax
// backdrop).
func (d *daemon) runInpaint(source, subj, bg string) error {
	d.stageBusy.Store(true)
	writeStageProgress(map[string]any{"phase": "inpaint"})
	d.publishStage()
	if err := d.runEngine(2*time.Minute, "inpaint", source, subj, bg); err != nil {
		writeStageProgress(map[string]any{"phase": "error", "error": err.Error(), "warning": "inpaint failed"})
		return err
	}
	return nil
}

// --- topic -----------------------------------------------------------------

// buildStageFrame assembles the `stage` topic frame: one entry per on-screen
// wallpaper (plus the current), keyed by path, so each monitor reads its own.
func (d *daemon) buildStageFrame() stageFrame {
	reg := loadStageWalls()
	cur := d.currentWall()
	busy := d.stageBusy.Load()
	stageStr, percent := "", 0
	if busy {
		switch readLastStagePhase() {
		case "cut":
			stageStr, percent = "cut", 40
		case "inpaint":
			stageStr, percent = "inpaint", 80
		default:
			stageStr, percent = "", 5
		}
	}
	paths := map[string]bool{}
	if cur != "" {
		paths[cur] = true
	}
	for _, t := range d.stageTargets() {
		paths[t.source] = true
	}
	walls := map[string]stageWallFrame{}
	for p := range paths {
		e := reg.Walls[p]
		effect := e.Effect
		if effect == "" {
			effect = stageEffectOff
		}
		wf := stageWallFrame{Effect: effect, Layers: []stageLayer{}}
		if effect != stageEffectOff {
			layers := d.deriveLayers(p, e.Layers)
			wf.Layers = layers
			rev := int64(0)
			if subj := stageSubjectOut(p); subjectFresh(p, subj) {
				wf.Subject = subj
			}
			if effect == stageEffectParallax {
				if bg := stageBackgroundOut(p); isFile(bg) {
					wf.Background = bg
					if r := fileModTime(bg); r > rev {
						rev = r
					}
				}
			}
			// One revision over every artifact so the shell busts every url at once.
			for _, l := range layers {
				if r := fileModTime(l.Out); r > rev {
					rev = r
				}
			}
			wf.Rev = rev
		}
		walls[p] = wf
	}
	return stageFrame{Current: cur, Busy: busy, Stage: stageStr, Percent: percent, Notice: d.stageNoticeText(), Walls: walls}
}

// stageNoticeText reads the last blocked-cut reason; a daemon that never
// reconciled (or a bare test struct) reads as fine.
func (d *daemon) stageNoticeText() string {
	if v, ok := d.stageNotice.Load().(string); ok {
		return v
	}
	return ""
}

func (d *daemon) publishStage() {
	t := d.topic("stage")
	if t == nil {
		return
	}
	if b, err := json.Marshal(d.buildStageFrame()); err == nil {
		t.publish(b)
	}
}

func (d *daemon) stageStatusJSON() string {
	b, _ := json.Marshal(d.buildStageFrame())
	return string(b)
}

// --- verbs -----------------------------------------------------------------

// restField recovers a trailing argument that may contain spaces from the raw
// command line, splitting into exactly n fields.
func restField(line string, n int) string {
	parts := strings.SplitN(line, " ", n)
	if len(parts) == n {
		return parts[n-1]
	}
	return ""
}

// stageSetEffect records the three-way effect for the current wallpaper and
// schedules a reconcile. Enabling never re-cuts: it reuses a saved cut when one
// exists and only generates what is missing, so switching Depth<->Parallax is
// instant. Off clears the ryogami overlay but keeps the layer arrangement for a
// later re-enable. The write is synchronous so the shell's control reflects at
// once.
func (d *daemon) stageSetEffect(eff stageEffect) {
	wall := d.currentWall()
	if wall == "" {
		return
	}
	reg := loadStageWalls()
	e := reg.Walls[wall]
	e.Effect = eff
	reg.Walls[wall] = e
	reg.Current = wall
	if err := saveStageWalls(reg); err != nil {
		logStage("set-effect save", err)
	}
	if eff == stageEffectOff {
		d.depthClear()
		d.publishStage()
		return
	}
	d.stageGen.Store(true)
	d.scheduleStage()
	d.publishStage()
}

// stageSetLayer applies the three arrangement flags (enabled/front/depth) to one
// layer (0-based, index 0 is the subject). Only those keys are accepted;
// identity (out/label) is the daemon's. The layer stack is re-derived first so a
// set that races an enable still lands on the right slot.
func (d *daemon) stageSetLayer(indexArg, body string) error {
	wall := d.currentWall()
	if wall == "" {
		return fmt.Errorf("no active wallpaper")
	}
	index, err := strconv.Atoi(indexArg)
	if err != nil {
		return fmt.Errorf("bad index: %s", indexArg)
	}
	var patch struct {
		Enabled *bool    `json:"enabled"`
		Front   *bool    `json:"front"`
		Depth   *float64 `json:"depth"`
	}
	if err := json.Unmarshal([]byte(body), &patch); err != nil {
		return err
	}
	reg := loadStageWalls()
	e := reg.Walls[wall]
	e.Layers = d.deriveLayers(wall, e.Layers)
	if index < 0 || index >= len(e.Layers) {
		return fmt.Errorf("layer index out of range: %d", index)
	}
	if patch.Enabled != nil {
		e.Layers[index].Enabled = *patch.Enabled
	}
	if patch.Front != nil {
		e.Layers[index].Front = *patch.Front
	}
	if patch.Depth != nil {
		e.Layers[index].Depth = *patch.Depth
	}
	reg.Walls[wall] = e
	reg.Current = wall
	if err := saveStageWalls(reg); err != nil {
		return err
	}
	d.publishStage()
	return nil
}

// stageAddLayer copies a ready PNG into the wall's folder as the next numbered
// manual layer and re-derives; no re-cut of the auto subject.
func (d *daemon) stageAddLayer(src string) (string, error) {
	wall := d.currentWall()
	if wall == "" {
		return "", fmt.Errorf("no active wallpaper")
	}
	if src == "" {
		return "", fmt.Errorf("empty source path")
	}
	st, err := os.Stat(src)
	if err != nil || st.IsDir() {
		return "", fmt.Errorf("source not a file: %s", src)
	}
	dir := stageWallDir(wall)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	dst := filepath.Join(dir, fmt.Sprintf("layer-%02d.png", nextManualIndex(dir)))
	if err := copyFile(src, dst); err != nil {
		return "", err
	}
	d.scheduleStage()
	return dst, nil
}

// stageCutLayer runs the engine on another picture and adds the alpha-matted
// result as the next manual layer. The cut runs in the background so the verb
// returns at once; progress rides the topic and a reconcile re-derives on done.
func (d *daemon) stageCutLayer(src string) (string, error) {
	wall := d.currentWall()
	if wall == "" {
		return "", fmt.Errorf("no active wallpaper")
	}
	if src == "" {
		return "", fmt.Errorf("empty source path")
	}
	st, err := os.Stat(src)
	if err != nil || st.IsDir() {
		return "", fmt.Errorf("source not a file: %s", src)
	}
	dir := stageWallDir(wall)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	dst := filepath.Join(dir, fmt.Sprintf("layer-%02d.png", nextManualIndex(dir)))
	q := stageConfig()
	go func() {
		d.stageBusy.Store(true)
		resetStageProgress()
		writeStageProgress(map[string]any{"phase": "cut", "model": q.model})
		d.publishStage()
		if err := d.engineCut(src, dst, q); err != nil {
			writeStageProgress(map[string]any{"phase": "error", "error": err.Error()})
			logStage("cut-layer "+stageStem(wall), err)
		} else {
			writeStageProgress(map[string]any{"phase": "done"})
		}
		d.stageBusy.Store(false)
		d.scheduleStage()
	}()
	return dst, nil
}

// stageRemoveLayer deletes a manual layer by its 0-based stack index. Index 0 is
// the subject and can never be removed; the resolved path is guarded to a
// layer-NN.png inside the wall's own folder.
func (d *daemon) stageRemoveLayer(indexArg string) error {
	wall := d.currentWall()
	if wall == "" {
		return fmt.Errorf("no active wallpaper")
	}
	index, err := strconv.Atoi(indexArg)
	if err != nil {
		return fmt.Errorf("bad index: %s", indexArg)
	}
	if index == 0 {
		return fmt.Errorf("cannot remove the subject")
	}
	reg := loadStageWalls()
	e := reg.Walls[wall]
	e.Layers = d.deriveLayers(wall, e.Layers)
	if index < 0 || index >= len(e.Layers) {
		return fmt.Errorf("layer index out of range: %d", index)
	}
	clean := filepath.Clean(e.Layers[index].Out)
	dir := filepath.Clean(stageWallDir(wall))
	if filepath.Dir(clean) != dir || !manualLayerRe.MatchString(filepath.Base(clean)) {
		return fmt.Errorf("not a manual layer: %s", e.Layers[index].Out)
	}
	if err := os.Remove(clean); err != nil {
		return err
	}
	d.scheduleStage()
	return nil
}

// stageClear removes the current wall's generated artifacts (subject, backdrop,
// index), keeps manual layers, and records the wall as off: the registry is the
// user's intent, and a cleared cut-out is an intent to stop showing the scene
// (an on-wall with purged files is the ghost state the self-heal would
// immediately re-cut). The reconcile then clears the overlay and re-derives.
func (d *daemon) stageClear() {
	wall := d.currentWall()
	if wall == "" {
		return
	}
	dir := stageWallDir(wall)
	for _, n := range []string{"subject.png", "background.png", ".index.json"} {
		_ = os.Remove(filepath.Join(dir, n))
	}
	reg := loadStageWalls()
	e := reg.Walls[wall]
	e.Effect = stageEffectOff
	reg.Walls[wall] = e
	if err := saveStageWalls(reg); err != nil {
		logStage("clear save", err)
	}
	d.publishStage()
	d.scheduleStage()
}

// --- migration -------------------------------------------------------------

// The v1 Ryostage (the merged knob-pile that landed and was rejected) is folded
// into v2 once, gated by a marker. The registry loses mode/scene and every
// per-layer look knob; stage.json loses feather/lift/preset and the motion
// sub-knobs. Read the v1 shapes raw so the retired fields survive long enough to
// reduce into the v2 flags before being dropped.

type stageWallV1 struct {
	Effect string           `json:"effect"`
	Mode   string           `json:"mode"`
	Scene  []string         `json:"scene"`
	Layers []map[string]any `json:"layers"`
}

type stageWallsV1 struct {
	Current string                 `json:"current"`
	Walls   map[string]stageWallV1 `json:"walls"`
}

func stageMigrationMarker() string {
	return filepath.Join(stateDir(), "ryoku", "migrations", "ryostage-v2")
}

// migrateStage folds the v1 Ryostage state into v2 once, gated by the marker.
// The registry fold must persist before the marker is written, so a failed run
// retries on the next start and a second start is a no-op.
func migrateStage() {
	marker := stageMigrationMarker()
	if isFile(marker) {
		return
	}
	if err := migrateStageRegistry(); err != nil {
		logStage("migrate registry", err)
		return
	}
	migrateStageSettings()
	if err := os.MkdirAll(filepath.Dir(marker), 0o755); err != nil {
		logStage("migrate marker dir", err)
		return
	}
	if err := os.WriteFile(marker, []byte("ryostage v2 migration complete\n"), 0o644); err != nil {
		logStage("migrate marker", err)
	}
}

// migrateStageRegistry converges the pre-v1 (retired Depth/Parallax) state and a
// v1 Ryostage registry onto one v2 stage-walls.json. The legacy fold runs first
// for walls the v1 registry has not already claimed -- a stable box that skipped
// v1 gets its whole Depth/Parallax arrangement, while a testing box's own v1
// entries win -- moving artifacts by rename; then every v1-shaped entry is
// normalised to v2. A missing v1 registry (a stable box) is just an empty base.
func migrateStageRegistry() error {
	var raw stageWallsV1
	if b, err := os.ReadFile(stageWallsPath()); err == nil {
		_ = json.Unmarshal(b, &raw)
	}
	claimed := map[string]bool{}
	for path := range raw.Walls {
		claimed[path] = true
	}
	out := stageWalls{Current: raw.Current, Walls: map[string]stageWall{}}
	migrateLegacyRegistry(&out, claimed)
	for path, w := range raw.Walls {
		out.Walls[path] = foldStageWallV1(path, w)
	}
	return saveStageWalls(out)
}

// Legacy on-disk formats (retired Depth and Parallax), read once to fold to v2.
type legacyDepthWalls struct {
	Walls map[string]bool `json:"walls"`
}

type legacyParallaxLayer struct {
	Out   string  `json:"out"`
	Rev   int64   `json:"rev"`
	Label string  `json:"label"`
	Depth float32 `json:"depth"`
	Area  float32 `json:"area"`
}

type legacyParallaxWall struct {
	Enabled bool     `json:"enabled"`
	Mode    string   `json:"mode"`
	Scene   []string `json:"scene"`
}

type legacyParallaxWalls struct {
	Walls  map[string]legacyParallaxWall    `json:"walls"`
	Layers map[string][]legacyParallaxLayer `json:"layers"`
}

func legacyDepthWallsPath() string { return filepath.Join(stateDir(), "ryoku", "depth-walls.json") }
func legacyDepthDir() string       { return filepath.Join(os.Getenv("HOME"), "Pictures", "Depth") }
func legacyParallaxDir() string    { return filepath.Join(os.Getenv("HOME"), "Pictures", "Parallax") }
func legacyLayersPath() string     { return filepath.Join(legacyParallaxDir(), "layers.pz") }

func loadLegacyDepthWalls() map[string]bool {
	b, err := os.ReadFile(legacyDepthWallsPath())
	if err != nil {
		return nil
	}
	var dw legacyDepthWalls
	if json.Unmarshal(b, &dw) != nil {
		return nil
	}
	return dw.Walls
}

func loadLegacyParallaxWalls() legacyParallaxWalls {
	var lp legacyParallaxWalls
	if b, err := os.ReadFile(legacyLayersPath()); err == nil {
		_ = json.Unmarshal(b, &lp)
	}
	return lp
}

// migrateLegacyRegistry folds the retired Depth (depth-walls.json + Pictures/
// Depth) and Parallax (layers.pz + Pictures/Parallax) state into v2 entries for
// walls the v1 registry has not claimed, moving each wall's artifacts by rename.
// Parallax is the richer effect and wins a wall enabled in both, so its folders
// move first (creating Stage/<stem>/), then the depth PNGs land beside them.
func migrateLegacyRegistry(out *stageWalls, claimed map[string]bool) {
	depthWalls := loadLegacyDepthWalls()
	lp := loadLegacyParallaxWalls()

	for path, w := range lp.Walls {
		if !w.Enabled || claimed[path] {
			continue
		}
		layers := migrateParallaxFolder(path, w, lp.Layers[path])
		e := out.Walls[path]
		e.Effect = stageEffectParallax
		if len(layers) > 0 {
			e.Layers = layers
		}
		out.Walls[path] = e
	}
	for path, on := range depthWalls {
		if !on || claimed[path] {
			continue
		}
		migrateDepthPNG(path)
		e := out.Walls[path]
		if e.Effect == "" || e.Effect == stageEffectOff {
			e.Effect = stageEffectDepth
		}
		if len(e.Layers) == 0 {
			e.Layers = []stageLayer{newSubjectLayer(stageSubjectOut(path))}
		}
		out.Walls[path] = e
	}
}

// migrateParallaxFolder moves ~/Pictures/Parallax/<stem>/ whole into the Stage
// tree, renames the auto subject (old layer-01.png) to subject.png, and returns
// the wall's v2 layer stack with out paths rewritten onto the new folder and
// front derived from the retired scene order. On a conflict the source is left
// in place; the stack is still rebuilt so the registry references resolve.
func migrateParallaxFolder(path string, w legacyParallaxWall, layers []legacyParallaxLayer) []stageLayer {
	stem := stageStem(path)
	src := filepath.Join(legacyParallaxDir(), stem)
	dst := stageWallDir(path)
	if isDir(src) && !isDir(dst) {
		if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
			logStage("migrate parallax mkdir "+stem, err)
		} else if err := os.Rename(src, dst); err != nil {
			logStage("migrate move parallax "+stem, err)
		}
	}
	auto := w.Mode == "" || w.Mode == "auto"
	if auto {
		l1 := filepath.Join(dst, "layer-01.png")
		sp := filepath.Join(dst, "subject.png")
		if isFile(l1) && !isFile(sp) {
			if err := os.Rename(l1, sp); err != nil {
				logStage("migrate rename subject "+stem, err)
			}
		}
	}
	front := sceneFront(w.Scene, len(layers))
	v2 := make([]stageLayer, 0, len(layers)+1)
	hasSubject := false
	for i, ol := range layers {
		base := filepath.Base(ol.Out)
		if auto && base == "layer-01.png" {
			base = "subject.png"
		}
		nl := stageLayer{
			Out:     filepath.Join(dst, base),
			Label:   ol.Label,
			Enabled: true,
			Depth:   float64(ol.Depth),
		}
		if base == "subject.png" {
			nl.Label = "Subject"
			hasSubject = true
		} else if nl.Label == "" {
			nl.Label = fmt.Sprintf("Layer %d", i+1)
		}
		if front != nil && i < len(front) {
			nl.Front = front[i]
		}
		if nl.Depth == 0 {
			nl.Depth = 0.5
		}
		v2 = append(v2, nl)
	}
	if !hasSubject {
		v2 = append([]stageLayer{newSubjectLayer(stageSubjectOut(path))}, v2...)
	}
	return v2
}

// migrateDepthPNG renames ~/Pictures/Depth/<stem>-depth.png to the wall's
// subject.png, unless a subject is already present (a wall enabled in both
// effects keeps the parallax subject).
func migrateDepthPNG(path string) {
	stem := stageStem(path)
	src := filepath.Join(legacyDepthDir(), stem+"-depth.png")
	if !isFile(src) {
		return
	}
	dst := stageSubjectOut(path)
	if isFile(dst) {
		logStage("migrate depth "+stem+": subject already present, left in place", nil)
		return
	}
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		logStage("migrate depth mkdir "+stem, err)
		return
	}
	if err := os.Rename(src, dst); err != nil {
		logStage("migrate move depth "+stem, err)
	}
}

// foldStageWallV1 reduces one v1 wall to the v2 shape.
func foldStageWallV1(path string, w stageWallV1) stageWall {
	eff := stageEffect(w.Effect)
	if w.Effect == "subject" {
		eff = stageEffectDepth
	}
	switch eff {
	case stageEffectDepth, stageEffectParallax:
	default:
		eff = stageEffectOff
	}
	var front []bool
	if len(w.Scene) > 0 {
		front = sceneFront(w.Scene, len(w.Layers))
	}
	layers := make([]stageLayer, 0, len(w.Layers)+1)
	hasSubject := false
	for i, l := range w.Layers {
		nl := stageLayer{
			Out:     asString(l["out"]),
			Label:   asString(l["label"]),
			Enabled: asBool(l["enabled"], true),
			Depth:   layerDepthV1(l),
		}
		if nl.Label == "" {
			nl.Label = fmt.Sprintf("Layer %d", i+1)
		}
		if front != nil {
			nl.Front = front[i]
		} else if b, ok := l["front"].(bool); ok {
			nl.Front = b
		}
		if filepath.Base(nl.Out) == "subject.png" {
			hasSubject = true
		}
		layers = append(layers, nl)
	}
	// A v1 manual wall lists only layer-NN.png; v2 always leads with the subject.
	if !hasSubject && eff != stageEffectOff {
		layers = append([]stageLayer{newSubjectLayer(stageSubjectOut(path))}, layers...)
	}
	return stageWall{Effect: eff, Layers: layers}
}

// sceneFront reduces a v1 scene order (back-to-front tokens: wallpaper, layer:N
// 1-based, widget:*, visualizer) to each layer's front flag: a layer token that
// appears after any widget:* token sits in front of the widgets.
func sceneFront(scene []string, n int) []bool {
	front := make([]bool, n)
	firstWidget := -1
	for i, tok := range scene {
		if strings.HasPrefix(tok, "widget:") {
			firstWidget = i
			break
		}
	}
	if firstWidget < 0 {
		return front
	}
	for i, tok := range scene {
		if i <= firstWidget || !strings.HasPrefix(tok, "layer:") {
			continue
		}
		idx, err := strconv.Atoi(strings.TrimPrefix(tok, "layer:"))
		if err != nil {
			continue
		}
		idx-- // 1-based token -> 0-based layer
		if idx >= 0 && idx < n {
			front[idx] = true
		}
	}
	return front
}

// layerDepthV1 reads a v1 layer's drift knob: the v1 `depthFactor` becomes the
// v2 `depth`; a registry already carrying `depth` keeps it; otherwise the mid
// default.
func layerDepthV1(l map[string]any) float64 {
	if v, ok := l["depthFactor"].(float64); ok {
		return v
	}
	if v, ok := l["depth"].(float64); ok {
		return v
	}
	return 0.5
}

func asString(v any) string {
	s, _ := v.(string)
	return s
}

func asBool(v any, def bool) bool {
	if b, ok := v.(bool); ok {
		return b
	}
	return def
}

// stageMotionAmount reduces the v1 motion sub-knobs to the single v2 word:
// mouse:false is Subtle (motion off the cursor), a high sensitivity is Strong,
// otherwise Normal.
func stageMotionAmount(motion map[string]any) string {
	if mouse, ok := motion["mouse"].(bool); ok && !mouse {
		return "subtle"
	}
	if sens, ok := motion["sensitivity"].(float64); ok && sens >= 1.5 {
		return "strong"
	}
	return "normal"
}

// migrateStageSettings folds the shell-owned stage.json to v2. An existing v1
// stage.json (one still carrying feather/lift/preset or the motion sub-knobs) is
// rewritten in place; an already-v2 file is left alone. When no stage.json
// exists, the retired depth.json + parallax.json are folded instead (a stable
// box that skipped v1), deriving the quality tier from their model+matting. The
// file is GUI-owned, so nothing is created when there is nothing to fold.
func migrateStageSettings() {
	dir := ryokuConfigDir()
	if dir == "" {
		return
	}
	path := filepath.Join(dir, "stage.json")
	if b, err := os.ReadFile(path); err == nil {
		var m map[string]any
		if json.Unmarshal(b, &m) != nil {
			return
		}
		motion, _ := m["motion"].(map[string]any)
		if !stageSettingsIsV1(m, motion) {
			return // already v2
		}
		if err := writeJSONFileAtomic(path, settingsToV2(m)); err != nil {
			logStage("migrate settings", err)
		}
		return
	}
	legacy := loadLegacyStageSettings(dir)
	if legacy == nil {
		return
	}
	if err := writeJSONFileAtomic(path, settingsToV2(legacy)); err != nil {
		logStage("migrate settings", err)
	}
}

// settingsToV2 maps a v1 or merged-legacy settings map onto the v2 stage.json
// shape: quality from the tier string when present else the legacy model+matting
// pair; the three model slots get their defaults; feather -> edge; the motion
// sub-knobs become one amount word; lift/preset are dropped.
func settingsToV2(m map[string]any) map[string]any {
	motion, _ := m["motion"].(map[string]any)
	quality := firstString(m["quality"], "")
	if quality != "draft" && quality != "standard" && quality != "fine" {
		model, _ := m["model"].(string)
		matting, _ := m["alphaMatting"].(bool)
		if model != "" || matting {
			if model == "" {
				model = "u2netp"
			}
			quality = qualityTierForModel(model, matting)
		} else {
			quality = "draft"
		}
	}
	return map[string]any{
		"quality":     quality,
		"models":      defaultStageModels(),
		"edge":        firstNumber(m["feather"], 0.15),
		"shadow":      firstNumber(m["shadow"], 0),
		"shadowAngle": firstNumber(m["shadowAngle"], 90),
		"motion": map[string]any{
			"amount": stageMotionAmount(motion),
			"idle":   firstString(mapValue(motion, "idle"), "none"),
			"music":  asBool(mapValue(motion, "music"), false),
		},
		"front": firstList(m["front"]),
	}
}

// loadLegacyStageSettings merges the retired depth.json and parallax.json into
// one settings map for the v2 fold: parallax first then depth so the depth
// scalars win, per-layer arrays are skipped (they never land in a scalar slot),
// and the higher model+matting tier is what the user paid the download for.
func loadLegacyStageSettings(dir string) map[string]any {
	scalarProto := map[string]any{
		"feather": 0.0, "shadow": 0.0, "shadowAngle": 0.0,
		"front": []any{}, "motion": map[string]any{},
	}
	result := map[string]any{}
	found := false
	bestTier := ""
	for _, name := range []string{"parallax.json", "depth.json"} {
		b, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			continue
		}
		var mm map[string]any
		if json.Unmarshal(b, &mm) != nil {
			continue
		}
		found = true
		for k, proto := range scalarProto {
			if v, ok := mm[k]; ok && sameJSONKind(v, proto) {
				result[k] = v
			}
		}
		model, _ := mm["model"].(string)
		_, hasMatting := mm["alphaMatting"]
		if model != "" || hasMatting {
			matting, _ := mm["alphaMatting"].(bool)
			if model == "" {
				model = "u2netp"
			}
			if tier := qualityTierForModel(model, matting); qualityRank(tier) > qualityRank(bestTier) {
				bestTier = tier
			}
		}
	}
	if !found {
		return nil
	}
	if bestTier != "" {
		result["quality"] = bestTier
	}
	return result
}

func qualityRank(t string) int {
	switch t {
	case "fine":
		return 2
	case "standard":
		return 1
	}
	return 0
}

// qualityTierForModel maps a legacy model + matting pair to a v2 quality tier.
func qualityTierForModel(model string, matting bool) string {
	switch {
	case model == "birefnet-general-lite":
		return "fine"
	case matting:
		return "standard"
	default:
		return "draft"
	}
}

// sameJSONKind reports whether a legacy value has the shape of the scalar slot it
// would fill, so a per-layer array never lands where a scalar is expected.
func sameJSONKind(v, def any) bool {
	switch def.(type) {
	case float64, int:
		_, ok := v.(float64)
		return ok
	case bool:
		_, ok := v.(bool)
		return ok
	case string:
		_, ok := v.(string)
		return ok
	case []any, []string:
		_, ok := v.([]any)
		return ok
	case map[string]any:
		_, ok := v.(map[string]any)
		return ok
	}
	return false
}

// stageSettingsIsV1 reports whether a stage.json still carries any v1-only key,
// so an already-v2 file is left untouched.
func stageSettingsIsV1(m, motion map[string]any) bool {
	for _, k := range []string{"feather", "lift", "preset"} {
		if _, ok := m[k]; ok {
			return true
		}
	}
	for _, k := range []string{"mouse", "sensitivity", "range", "wallpaper"} {
		if _, ok := motion[k]; ok {
			return true
		}
	}
	return false
}

func mapValue(m map[string]any, k string) any {
	if m == nil {
		return nil
	}
	return m[k]
}

func firstString(v any, def string) string {
	if s, ok := v.(string); ok && s != "" {
		return s
	}
	return def
}

func firstNumber(v any, def float64) float64 {
	if n, ok := v.(float64); ok {
		return n
	}
	return def
}

func firstList(v any) []any {
	if l, ok := v.([]any); ok {
		return l
	}
	return []any{}
}

// startStage registers the topic, runs the one-time migration, publishes the
// first frame, and starts the coalescing worker.
func (d *daemon) startStage() {
	d.registerTopic("stage")
	migrateStage()
	d.publishStage()
	go d.stageWorker()
}
