package main

import (
	"context"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"syscall"
	"time"

	wm "ryoku-wm"
)

type pauseRules struct {
	processEnabled   bool
	processes        []string
	fullscreen       bool
	maximized        bool
	fullWidth        bool
	overviewOnly     bool
	scopeDisplay     bool
	capOverviewState bool
	capColumnFill    bool
}

type wmOutputGeom struct{ width, height int }

type wmWin struct {
	output    string
	workspace string
	width     int
	height    int
}

type wmSnapshot struct {
	outputs          map[string]wmOutputGeom
	activeWorkspaces map[string]bool
	windows          []wmWin
	focusedOutput    string
	overviewOpen     bool
}

// Absorbs the sub-pixel rounding a fractional-scale tile leaves against its output.
const coverTolerance = 8

// Coverage comes from geometry alone: no compositor guarantees a maximised flag.
func windowCoverage(w wmWin, out wmOutputGeom) (fullWidth, maximized, fullscreen bool) {
	if out.width <= 0 || out.height <= 0 {
		return
	}
	fullWidth = w.width*10 >= out.width*9
	coversHeight := w.height+coverTolerance >= out.height
	fullscreen = fullWidth && coversHeight
	maximized = fullWidth && !coversHeight
	return
}

func evaluatePause(r pauseRules, s wmSnapshot, running map[string]bool) (bool, []string) {
	var reasons []string
	if r.processEnabled {
		for _, p := range r.processes {
			if running[p] {
				reasons = append(reasons, "process")
				break
			}
		}
	}
	if r.overviewOnly && r.capOverviewState && !s.overviewOpen {
		reasons = append(reasons, "overview")
	}
	if r.fullscreen || r.maximized || (r.fullWidth && r.capColumnFill) {
		for _, w := range s.windows {
			if len(s.activeWorkspaces) > 0 && !s.activeWorkspaces[w.workspace] {
				continue
			}
			if r.scopeDisplay && s.focusedOutput != "" && w.output != s.focusedOutput {
				continue
			}
			geom, ok := s.outputs[w.output]
			if !ok {
				continue
			}
			fw, mx, fs := windowCoverage(w, geom)
			if r.fullscreen && fs {
				reasons = append(reasons, "fullscreen")
			}
			if r.maximized && mx {
				reasons = append(reasons, "maximized")
			}
			if r.fullWidth && r.capColumnFill && fw {
				reasons = append(reasons, "fullWidth")
			}
		}
	}
	return len(reasons) > 0, dedupReasons(reasons)
}

func dedupReasons(in []string) []string {
	if len(in) == 0 {
		return nil
	}
	seen := map[string]bool{}
	out := in[:0]
	for _, s := range in {
		if !seen[s] {
			seen[s] = true
			out = append(out, s)
		}
	}
	return out
}

var wmCache struct {
	mu            sync.Mutex
	windows       []wmWin
	activeWs      map[string]bool
	focusedOutput string
	overviewOpen  bool
}

var pauseState struct {
	mu          sync.Mutex
	active      bool
	manual      map[string]bool
	applied     bool
	resumeTimer *time.Timer
}

var procCache struct {
	mu  sync.Mutex
	at  time.Time
	set map[string]bool
}

func (d *daemon) currentPauseRules() pauseRules {
	r := pauseRules{
		processEnabled: d.settingBool("playback.processEnabled"),
		processes:      ifaceStrings(d.settingList("playback.processes")),
		fullscreen:     d.settingBool("playback.fullscreen"),
		maximized:      d.settingBool("playback.maximized"),
		fullWidth:      d.settingBool("playback.fullWidthPause"),
		overviewOnly:   d.settingBool("playback.overviewOnly"),
		scopeDisplay:   d.settingString("playback.fullscreenScope") == "display",
	}
	if caps, err := wmClient.Caps(); err == nil {
		r.capOverviewState = caps.Has(wm.CapOverviewState)
		r.capColumnFill = caps.Has(wm.CapColumnFill)
	}
	return r
}

func (d *daemon) pauseSnapshot() wmSnapshot {
	snap := wmSnapshot{outputs: map[string]wmOutputGeom{}, activeWorkspaces: map[string]bool{}}
	for _, o := range outputs.list() {
		snap.outputs[o.Name] = wmOutputGeom{o.Width, o.Height}
	}
	wmCache.mu.Lock()
	snap.windows = append([]wmWin(nil), wmCache.windows...)
	for k, v := range wmCache.activeWs {
		snap.activeWorkspaces[k] = v
	}
	snap.focusedOutput = wmCache.focusedOutput
	snap.overviewOpen = wmCache.overviewOpen
	wmCache.mu.Unlock()
	return snap
}

func (d *daemon) evaluatePauseNow() (bool, []string) {
	r := d.currentPauseRules()
	var running map[string]bool
	if r.processEnabled {
		running = cachedRunningSet()
	}
	return evaluatePause(r, d.pauseSnapshot(), running)
}

func (d *daemon) reevaluatePause() {
	want, reasons := d.evaluatePauseNow()
	pauseState.mu.Lock()
	if want {
		if pauseState.resumeTimer != nil {
			pauseState.resumeTimer.Stop()
			pauseState.resumeTimer = nil
		}
		pauseState.active = true
	} else if pauseState.active && pauseState.resumeTimer == nil {
		// Clearing waits out the resume delay so a brief gap does not blink the wallpaper.
		pauseState.resumeTimer = time.AfterFunc(d.resumeDelay(), d.finishResume)
	}
	changed, val := syncAppliedLocked()
	manualAny := len(pauseState.manual) > 0
	pauseState.mu.Unlock()
	if changed {
		d.applyPauseState(val)
	}
	d.broadcastPlaybackStatus(want || manualAny, reasons)
}

func (d *daemon) finishResume() {
	want, _ := d.evaluatePauseNow()
	pauseState.mu.Lock()
	pauseState.resumeTimer = nil
	pauseState.active = want // a condition that returned during the delay keeps it
	changed, val := syncAppliedLocked()
	manualAny := len(pauseState.manual) > 0
	pauseState.mu.Unlock()
	if changed {
		d.applyPauseState(val)
	}
	d.broadcastPlaybackStatus(want || manualAny, nil)
}

// Caller holds pauseState.mu.
func syncAppliedLocked() (changed, value bool) {
	eff := pauseState.active || len(pauseState.manual) > 0
	if eff != pauseState.applied {
		pauseState.applied = eff
		return true, eff
	}
	return false, pauseState.applied
}

func (d *daemon) setManualPause(outputs []string, paused bool) {
	pauseState.mu.Lock()
	if pauseState.manual == nil {
		pauseState.manual = map[string]bool{}
	}
	switch {
	case len(outputs) == 0 || contains(outputs, "*"):
		if paused {
			pauseState.manual = map[string]bool{"*": true}
		} else {
			pauseState.manual = map[string]bool{}
		}
	default:
		for _, o := range outputs {
			if paused {
				pauseState.manual[o] = true
			} else {
				delete(pauseState.manual, o)
			}
		}
	}
	changed, val := syncAppliedLocked()
	active := pauseState.active
	manualAny := len(pauseState.manual) > 0
	pauseState.mu.Unlock()
	if changed {
		d.applyPauseState(val)
	}
	d.broadcastPlaybackStatus(active || manualAny, nil)
}

func (d *daemon) outputPauseState(name string) (paused, manual bool) {
	pauseState.mu.Lock()
	defer pauseState.mu.Unlock()
	manual = pauseState.manual[name] || pauseState.manual["*"]
	paused = pauseState.active || len(pauseState.manual) > 0
	return
}

func (d *daemon) outputGeom(name string) (int, int, bool) {
	for _, o := range outputs.list() {
		if o.Name == name {
			return o.Width, o.Height, true
		}
	}
	return 0, 0, false
}

func (d *daemon) resumeDelay() time.Duration {
	s := d.settingNumber("playback.resumeDelay")
	if s <= 0 {
		return 0
	}
	return time.Duration(s * float64(time.Second))
}

func (d *daemon) applyPauseState(paused bool) {
	if d.video != nil {
		d.video.Pause(paused)
	}
	if d.paper != nil && d.paper.running() {
		_, _ = d.paper.exchange("paper.pause", paperPauseReq{Paused: paused}, false)
	}
	if d.surface != nil {
		d.surface.setPaused(paused)
	}
}

type paperPauseReq struct {
	Paused bool `json:"paused"`
}

func (d *daemon) broadcastPlaybackStatus(paused bool, reasons []string) {
	if reasons == nil {
		reasons = []string{}
	}
	d.broadcast("ryogami.playback.status", map[string]interface{}{
		"paused":  paused,
		"reasons": reasons,
	})
}

// SIGSTOP on the player's process group halts decode at no CPU cost; SIGCONT resumes the same frame.
func (p *videoPlayer) Pause(paused bool) {
	sig := syscall.SIGCONT
	if paused {
		sig = syscall.SIGSTOP
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	for _, lp := range p.procs {
		if lp.cmd.Process != nil {
			_ = syscall.Kill(-lp.cmd.Process.Pid, sig)
		}
	}
}

// Republished without a new revision so the shell pauses the clip in place instead of reloading.
func (w *wallSurface) setPaused(paused bool) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.def.Paused == paused {
		allSame := true
		for _, e := range w.outputs {
			if e.Paused != paused {
				allSame = false
				break
			}
		}
		if allSame {
			return
		}
	}
	w.def.Paused = paused
	for k, e := range w.outputs {
		e.Paused = paused
		w.outputs[k] = e
	}
	w.publishLocked()
}

func (d *daemon) watchWindowState() {
	kinds := []wm.FrameKind{wm.FrameWindows, wm.FrameWorkspaces, wm.FrameFocus, wm.FrameOverview}
	for {
		_ = wmClient.WatchKinds(context.Background(), kinds, func(f wm.Frame) {
			wmCache.mu.Lock()
			switch f.Kind {
			case wm.FrameWindows:
				wins := make([]wmWin, 0, len(f.Windows))
				for _, w := range f.Windows {
					wins = append(wins, wmWin{output: w.Output, workspace: w.Workspace, width: w.Width, height: w.Height})
				}
				wmCache.windows = wins
			case wm.FrameWorkspaces:
				aw := map[string]bool{}
				for _, ws := range f.Workspaces {
					if ws.Active {
						aw[ws.ID] = true
					}
				}
				wmCache.activeWs = aw
			case wm.FrameFocus:
				wmCache.focusedOutput = f.FocusedOutput
			case wm.FrameOverview:
				wmCache.overviewOpen = f.OverviewOpen
			}
			wmCache.mu.Unlock()
			if f.Kind == wm.FrameWorkspaces {
				d.workspaces.updateActive(f.Workspaces)
			}
			d.reevaluatePause()
		})
		time.Sleep(500 * time.Millisecond)
	}
}

func (d *daemon) pollProcesses() {
	for {
		time.Sleep(2 * time.Second)
		if d.settingBool("playback.processEnabled") {
			d.reevaluatePause()
		}
	}
}

func cachedRunningSet() map[string]bool {
	procCache.mu.Lock()
	defer procCache.mu.Unlock()
	if procCache.set != nil && time.Since(procCache.at) < time.Second {
		return procCache.set
	}
	procCache.set = runningProcSet()
	procCache.at = time.Now()
	return procCache.set
}

// runningProcSet is every process's comm and executable basename. A listed name
// longer than the kernel's 15-char comm still matches through the exe basename.
func runningProcSet() map[string]bool {
	set := map[string]bool{}
	ents, err := os.ReadDir("/proc")
	if err != nil {
		return set
	}
	for _, e := range ents {
		if !isPid(e.Name()) {
			continue
		}
		if comm := readTrimmed(filepath.Join("/proc", e.Name(), "comm")); comm != "" {
			set[comm] = true
		}
		if exe, err := os.Readlink(filepath.Join("/proc", e.Name(), "exe")); err == nil {
			set[filepath.Base(exe)] = true
		}
	}
	return set
}

func sortedRunningProcesses() []string {
	set := runningProcSet()
	out := make([]string, 0, len(set))
	for name := range set {
		out = append(out, name)
	}
	sort.Strings(out)
	return out
}

func isPid(s string) bool {
	for _, c := range s {
		if c < '0' || c > '9' {
			return false
		}
	}
	return s != ""
}

func ifaceStrings(v []interface{}) []string {
	out := make([]string, 0, len(v))
	for _, e := range v {
		if s, ok := e.(string); ok && s != "" {
			out = append(out, s)
		}
	}
	return out
}

func stringsIface(v []string) []interface{} {
	out := make([]interface{}, 0, len(v))
	for _, s := range v {
		out = append(out, s)
	}
	return out
}

func strIn(list []string, s string) bool {
	for _, e := range list {
		if e == s {
			return true
		}
	}
	return false
}

func init() {
	onStart(func(d *daemon) {
		go d.watchWindowState()
		go d.pollProcesses()
		d.reevaluatePause()
	})
	watchSetting("playback.", func(d *daemon, _ string, _ interface{}) { d.reevaluatePause() })
	watchSetting("paper.idlePauseSeconds", func(d *daemon, _ string, _ interface{}) { d.reevaluatePause() })
	afterApply(func(d *daemon, _ applyEvent) { d.reevaluatePause() })

	registerAction("ChooseRunningProcess", func(d *daemon, args map[string]interface{}) (interface{}, error) {
		if add, ok := args["add"].(string); ok && add != "" {
			list := ifaceStrings(d.settingList("playback.processes"))
			if !strIn(list, add) {
				list = append(list, add)
				if err := d.setSetting("playback.processes", stringsIface(list)); err != nil {
					return nil, err
				}
			}
		}
		if rm, ok := args["remove"].(string); ok && rm != "" {
			list := ifaceStrings(d.settingList("playback.processes"))
			kept := make([]string, 0, len(list))
			for _, p := range list {
				if p != rm {
					kept = append(kept, p)
				}
			}
			if err := d.setSetting("playback.processes", stringsIface(kept)); err != nil {
				return nil, err
			}
		}
		return map[string]interface{}{
			"running":   sortedRunningProcesses(),
			"processes": ifaceStrings(d.settingList("playback.processes")),
		}, nil
	})
}
