package demo

import (
	"slices"
	"time"

	"ryoku-i18n"

	"ryoku-rashin/internal/oni/feature"
	"ryoku-rashin/internal/protocol"
)

// FeatureOniHandsApp starts the demo as FeatureOni with Hands in one app
// window on: no window picked yet, and a task journal holding a task the
// last restart interrupted, so Start again has something to start.
const FeatureOniHandsApp = "oni-hands-app"

// FeatureOniHandsAppPaused is FeatureOniHandsApp with the first fixture
// window authorized and a task in it paused because the person pressed a
// key.
const FeatureOniHandsAppPaused = "oni-hands-app-paused"

// FeatureOniHandsDesktop starts the demo with Hands on a separate desktop
// on, on a machine with weston, niri and grim installed.
const FeatureOniHandsDesktop = "oni-hands-desktop"

// FeatureOniHandsNoNest is FeatureOniHandsDesktop on a machine without
// weston and niri, so the separate desktop cannot run.
const FeatureOniHandsNoNest = "oni-hands-no-nest"

// demoHandsAppFeature says whether id is one of the app or desktop Hands
// demo switches, and which stage it starts at.
func demoHandsAppFeature(id string) (string, bool) {
	switch id {
	case FeatureOniHandsApp, FeatureOniHandsAppPaused:
		return protocol.OniHandsApp, true
	case FeatureOniHandsDesktop, FeatureOniHandsNoNest:
		return protocol.OniHandsDesktop, true
	}
	return "", false
}

// handsAppPart is the demo's Hands outside the browser: the programs the
// separate desktop lacks, the window picked, the running task's scope and
// id, the stage it runs at, and the journal, newest first. It holds no
// goal: a task started again is started with its goal said again.
type handsAppPart struct {
	missing []string
	window  *protocol.OniHandsWindow
	stage   string
	scope   string
	task    string
	tasks   []protocol.OniHandsTaskRow
	// seq numbers the journal rows the demo adds
	seq int
}

// the live windows the demo's desktop shows: one floating, so Hands may
// click in it, and one niri tiled in a column, keys only
var demoHandsWindows = []protocol.OniHandsWindow{
	{ID: "w41", App: "org.gnome.TextEditor", Title: "Notes", Clickable: true},
	{ID: "w42", App: "org.mozilla.firefox", Title: "Example Domain", Clickable: false},
}

// the output the demo's windows are on
const demoHandsOutput = "DP-1"

// the programs a machine without the separate desktop lacks
var demoHandsMissing = []string{"weston", "niri"}

// demoHandsTaskID is the demo's journal id n: 32 hex digits, as the
// journal's own are.
func demoHandsTaskID(n int) string {
	const hex = "0123456789abcdef"
	id := []byte("d0d0d0d0d0d0d0d0d0d0d0d0d0d0d000")
	id[len(id)-2], id[len(id)-1] = hex[(n/16)%16], hex[n%16]
	return string(id)
}

// demoHandsJournal is the journal the app and desktop demos start from,
// newest first: a task in a window the last restart interrupted, one on
// the separate desktop that was done, and one the person stopped. Codes
// only: no goal, title or typed word.
func demoHandsJournal() []protocol.OniHandsTaskRow {
	return []protocol.OniHandsTaskRow{
		{ID: demoHandsTaskID(3), State: "interrupted", Scope: protocol.OniHandsScopeWindow, App: "org.gnome.TextEditor", Origin: "app",
			Step: 3, At: Epoch.Add(-2 * time.Hour), Ended: Epoch.Add(-2*time.Hour + 4*time.Minute), Reason: "restart"},
		{ID: demoHandsTaskID(2), State: "done", Scope: protocol.OniHandsScopeDesktop, Origin: "app",
			Step: 6, At: Epoch.Add(-26 * time.Hour), Ended: Epoch.Add(-26*time.Hour + 3*time.Minute), Reason: "done"},
		{ID: demoHandsTaskID(1), State: "stopped", Scope: protocol.OniHandsScopeWindow, App: "org.mozilla.firefox", Origin: "presenter",
			Step: 2, At: Epoch.Add(-50 * time.Hour), Ended: Epoch.Add(-50*time.Hour + time.Minute), Reason: "stopped"},
	}
}

// newHandsAppPart is the demo's Hands outside the browser for its features
func newHandsAppPart(features []string) handsAppPart {
	a := handsAppPart{seq: 3}
	app := slices.Contains(features, FeatureOniHandsApp) || slices.Contains(features, FeatureOniHandsAppPaused)
	desktop := slices.Contains(features, FeatureOniHandsDesktop) || slices.Contains(features, FeatureOniHandsNoNest)
	if app || desktop {
		a.tasks = demoHandsJournal()
	}
	if slices.Contains(features, FeatureOniHandsNoNest) {
		a.missing = slices.Clone(demoHandsMissing)
	}
	return a
}

// stageChanged ends a running task, as a settings change does in the
// daemon, and lets the picked window go with the stage it was picked at.
func (h *handsFixtures) stageChanged() {
	switch {
	case h.state != "" && h.stage != "":
		h.endTask("stopped", protocol.OniHandsEndStopped)
	case h.state != "":
		h.drop()
		h.state, h.step, h.pause = "", 0, ""
		h.told = append(h.told, h.frame())
	}
	h.window, h.stage = nil, ""
}

// startPaused puts FeatureOniHandsAppPaused's task in place: the first
// window authorized, a task in it paused at its fourth step by a key.
func (h *handsFixtures) startPaused() {
	w := demoHandsWindows[0]
	h.window = &w
	h.seq++
	h.stage, h.scope, h.task = protocol.OniHandsApp, protocol.OniHandsScopeWindow, demoHandsTaskID(h.seq)
	h.state, h.step, h.pause = protocol.OniHandsStatePaused, demoHandsPausedStep, protocol.OniHandsPauseKey
	h.tasks = append([]protocol.OniHandsTaskRow{{ID: h.task, State: "needsyou", Scope: h.scope, App: w.App, Origin: "app",
		Step: h.step, At: Epoch.Add(-3 * time.Minute), Reason: protocol.OniHandsPauseKey}}, h.tasks...)
}

// callOniHandsApp answers oni.hands at the app or desktop stage as the
// daemon does: windows and select at the app stage only, a task in the
// picked window or on the separate desktop, and the journal. The browser's
// own actions are refused here.
func (s *server) callOniHandsApp(stage string, p *protocol.OniHandsParams) (any, error) {
	h := &s.data.oni.hands
	running := h.state != ""
	switch p.Action {
	case protocol.OniHandsOpen, protocol.OniHandsTabs, protocol.OniHandsAuthorize, protocol.OniHandsForget:
		return nil, protocol.Errorf(protocol.CodeConflict, "%s", i18n.T("That is Hands in a browser, which is not the stage Hands is at. Choose it in Settings, under Oni › Hands."))
	case protocol.OniHandsWindows, protocol.OniHandsSelect:
		if stage != protocol.OniHandsApp {
			return nil, protocol.Errorf(protocol.CodeConflict, "%s", i18n.T("Hands picks a window only when it works in one app window."))
		}
		if p.Action == protocol.OniHandsWindows {
			result := h.appResult(stage)
			result.Windows = slices.Clone(demoHandsWindows)
			return result, nil
		}
		if running {
			return nil, protocol.Errorf(protocol.CodeConflict, "%s", i18n.T("Hands is working on a task. Stop it first."))
		}
		i := slices.IndexFunc(demoHandsWindows, func(w protocol.OniHandsWindow) bool { return w.ID == p.Window })
		if i < 0 {
			return nil, protocol.Errorf(protocol.CodeNotFound, "%s", i18n.T("That window is not open any more."))
		}
		w := demoHandsWindows[i]
		h.window = &w
	case protocol.OniHandsStart:
		scope := p.Scope
		if scope == "" {
			scope = protocol.OniHandsScopeWindow
			if stage == protocol.OniHandsDesktop {
				scope = protocol.OniHandsScopeDesktop
			}
		}
		switch {
		case stage == protocol.OniHandsApp && scope != protocol.OniHandsScopeWindow && scope != protocol.OniHandsScopeNested,
			stage == protocol.OniHandsDesktop && scope != protocol.OniHandsScopeDesktop:
			return nil, protocol.Errorf(protocol.CodeForbidden, "%s", i18n.T("Hands works only where its stage lets it."))
		case scope == protocol.OniHandsScopeWindow && h.window == nil:
			return nil, protocol.Errorf(protocol.CodeConflict, "%s", i18n.T("Authorize a window first."))
		case scope != protocol.OniHandsScopeWindow && len(h.missing) > 0:
			return nil, protocol.Errorf(protocol.CodeUnavailable, "%s", i18n.T("The separate desktop cannot run on this computer."))
		}
		var again *protocol.OniHandsTaskRow
		if p.Task != "" {
			i := slices.IndexFunc(h.tasks, func(t protocol.OniHandsTaskRow) bool { return t.ID == p.Task })
			if i < 0 || (h.tasks[i].State != "upnext" && h.tasks[i].State != "interrupted") {
				return nil, protocol.Errorf(protocol.CodeNotFound, "%s", i18n.T("That task cannot be started again."))
			}
			again = &h.tasks[i]
		}
		app := ""
		if scope == protocol.OniHandsScopeWindow {
			app = h.window.App
		}
		if running {
			// a second start waits behind the running task, for a button
			h.seq++
			h.tasks = append([]protocol.OniHandsTaskRow{{ID: demoHandsTaskID(h.seq), State: "upnext", Scope: scope, App: app, Origin: "app", At: Epoch}}, h.tasks...)
			result := h.appResult(stage)
			result.Line = i18n.T("Hands will start it once the running task ends and you press Start.")
			return result, nil
		}
		id := ""
		if again != nil {
			again.State, again.Ended, again.Reason = "working", time.Time{}, ""
			id = again.ID
		} else {
			h.seq++
			id = demoHandsTaskID(h.seq)
			h.tasks = append([]protocol.OniHandsTaskRow{{ID: id, State: "working", Scope: scope, App: app, Origin: "app", Step: 1, At: Epoch}}, h.tasks...)
		}
		h.stage, h.scope, h.task = stage, scope, id
		h.state, h.step, h.pause = protocol.OniHandsStateWorking, 1, ""
		route, destination := s.data.oni.route()
		question := i18n.T("Send a picture of the window you picked and its words for step 1?")
		if scope != protocol.OniHandsScopeWindow {
			question = i18n.T("Send a picture of the separate desktop and its words for step 1?")
		}
		card := feature.ConsentFrame{ID: demoHandsStepCard, Feature: feature.Inspect, Class: feature.ConsentDisclosure,
			Destination: destination, Route: route, ExpiresMs: feature.ConsentTTL.Milliseconds(), Question: question}
		h.card = &card
		h.told = append(h.told, h.frame(), card)
	case protocol.OniHandsStop:
		if running {
			h.endTask("stopped", protocol.OniHandsEndStopped)
		}
	case protocol.OniHandsResume:
		if h.state != protocol.OniHandsStatePaused {
			return nil, protocol.Errorf(protocol.CodeConflict, "%s", i18n.T("Hands is not paused."))
		}
		h.state, h.pause = protocol.OniHandsStateWorking, ""
		h.setRow(h.task, "working", "")
		h.told = append(h.told, h.frame())
	case protocol.OniHandsTasks:
		result := h.appResult(stage)
		result.Tasks = slices.Clone(h.tasks)
		return result, nil
	case protocol.OniHandsClear:
		if running {
			return nil, protocol.Errorf(protocol.CodeConflict, "%s", i18n.T("Hands is working on a task. Stop it first."))
		}
		h.tasks = nil
		result := h.appResult(stage)
		result.Line = i18n.T("The Hands journal is deleted.")
		return result, nil
	}
	return h.appResult(stage), nil
}

// endTask ends the running task: its journal row takes state and reason,
// and the inactive frame says how it ended.
func (h *handsFixtures) endTask(state, end string) {
	h.drop()
	h.setRow(h.task, state, end)
	h.state, h.step, h.pause = "", 0, ""
	h.told = append(h.told, h.frame())
	h.scope, h.task = "", ""
}

// setRow moves the journal row id to state, with reason, ending it when
// the state is an end.
func (h *handsFixtures) setRow(id, state, reason string) {
	i := slices.IndexFunc(h.tasks, func(t protocol.OniHandsTaskRow) bool { return t.ID == id })
	if i < 0 {
		return
	}
	r := &h.tasks[i]
	r.State, r.Reason, r.Step = state, reason, max(r.Step, h.step)
	switch state {
	case "done", "failed", "stopped", "interrupted":
		r.Ended = Epoch
	}
}

// appResult is where the demo's Hands stands at the app or desktop stage
func (h *handsFixtures) appResult(stage string) protocol.OniHandsResult {
	r := protocol.OniHandsResult{Missing: slices.Clone(h.missing)}
	if h.window != nil && stage == protocol.OniHandsApp {
		r.App = h.window.App
	}
	switch {
	case h.state != "":
		r.State, r.Scope, r.Task, r.Step, r.Max, r.Pause = h.state, h.scope, h.task, h.step, feature.HandsMaxSteps(), h.pause
	case stage == protocol.OniHandsApp && h.window == nil:
		r.State = protocol.OniHandsStateOpen
	case stage == protocol.OniHandsDesktop && len(h.missing) > 0:
		r.State = protocol.OniHandsStateClosed
	default:
		r.State = protocol.OniHandsStateAuthorized
	}
	return r
}

// appFrame is the hands frame of a task outside the browser
func (h *handsFixtures) appFrame() feature.HandsFrame {
	f := feature.HandsFrame{Stage: h.stage, Reason: feature.ReasonAllowed}
	if h.state == "" {
		f.End = protocol.OniHandsEndStopped
		return f
	}
	f.Active, f.Scope, f.Phase, f.Step, f.Max, f.Pause = true, h.scope, h.state, h.step, feature.HandsMaxSteps(), h.pause
	if h.scope == protocol.OniHandsScopeWindow && h.window != nil {
		f.App, f.Output = h.window.App, demoHandsOutput
	}
	return f
}
