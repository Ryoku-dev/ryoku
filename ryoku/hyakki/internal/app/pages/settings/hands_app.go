package settings

import (
	"fmt"
	"strings"
	"sync"

	"gioui.org/layout"
	"gioui.org/widget"

	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/kit"
	"ryoku-i18n"

	"ryoku-rashin/internal/app/env"
	"ryoku-rashin/internal/app/look"
	"ryoku-rashin/internal/oni/feature"
	"ryoku-rashin/internal/oni/spec"
	"ryoku-rashin/internal/protocol"
)

// handsAppPanel is the Hands page's part for the app and desktop stages:
// the two stage switches, the live windows listed at the last Choose a
// window with Authorize this window, the task journal with Start again and
// Delete all, and what to know before using either. Window titles are
// shown at the moment of choosing only; nothing here keeps a goal, a title
// or anything typed.
type handsAppPanel struct {
	appStage, deskStage kit.Toggle

	choose, clear kit.Plaque
	authorize     map[string]*kit.Plaque
	again         map[string]*kit.Plaque

	// the windows at the last windows call, whether they were listed, the
	// journal at the last tasks call, whether it is current, and whether
	// something since may have moved it
	windows    []protocol.OniHandsWindow
	listed     bool
	tasks      []protocol.OniHandsTaskRow
	tasksRead  bool
	tasksStale bool

	// the question before deleting the journal
	clearing   kit.Confirm
	confirming bool
}

// forget drops what was read, so the page reads it again
func (a *handsAppPanel) forget() {
	a.windows, a.listed, a.tasks, a.tasksRead, a.tasksStale = nil, false, nil, false, false
}

// heard keeps what an answer listed: the windows of a windows call, the
// journal of a tasks call; a clear empties the journal
func (a *handsAppPanel) heard(action string, windows []protocol.OniHandsWindow, tasks []protocol.OniHandsTaskRow) {
	switch action {
	case protocol.OniHandsWindows:
		a.windows, a.listed = windows, true
	case protocol.OniHandsTasks:
		a.tasks, a.tasksRead, a.tasksStale = tasks, true, false
	case protocol.OniHandsClear:
		a.tasks, a.tasksRead, a.tasksStale = nil, true, false
	}
}

// handsCardFeature says whether a card is one of Hands' handoffs: in the
// browser tab, a live window or the separate desktop
func handsCardFeature(id string) bool {
	return id == feature.HandsBrowser || id == feature.HandsApp || id == feature.HandsDesktop
}

var handsNumbers = sync.OnceValues(func() (spec.Hands, error) {
	loaded, err := spec.Load()
	if err != nil {
		return spec.Hands{}, err
	}
	return loaded.Hands, nil
})

// stageRows are the three stages, one switch each and only the app's to
// flip: turning one on stores its stage, which turns the others off, and
// turning the one that is on off stores off. A switch is on only while its
// own stage is stored, so choosing one never shows another on.
func (h *handsPanel) stageRows(e *env.Env, settings protocol.Settings) []look.Setting {
	th := e.Theme
	oni := protocol.EffectiveOni(settings)
	stored := settings.Oni
	off := ""
	if !oni.Master {
		off = i18n.T("Oni is off, so Hands cannot run.")
	}
	needs := ""
	if !stored.Inspect {
		needs = "\n" + i18n.T("Needs Looking at a window: each step sends a picture, as asking about a window does. Until it is on, Hands does nothing.")
	}
	row := func(stage, name, detail string, toggle *kit.Toggle) look.Setting {
		toggle.Value = stored.Hands == stage
		return look.Setting{Title: name, Detail: detail + needs, Unavailable: off,
			Changed: stored.Hands == stage, Reset: func() { set(e, "oni", map[string]any{"hands": protocol.OniHandsOff}) },
			Control: func(gtx layout.Context) layout.Dimensions {
				return namedSwitch(gtx, th, name, toggle, func(on bool) {
					next := protocol.OniHandsOff
					if on {
						next = stage
					}
					set(e, "oni", map[string]any{"hands": next})
				})
			}}
	}
	return []look.Setting{
		row(protocol.OniHandsBrowser, i18n.T("Hands in a browser"),
			i18n.T("Oni fills fields, picks options, clicks ordinary controls and scrolls. Anything that submits, sends, posts, pays, buys, deletes or signs out it never presses: it points at the control and you press it. Only this app turns Hands on; the command line and voice never can."),
			&h.stage),
		row(protocol.OniHandsApp, i18n.T("Hands in one app window"),
			i18n.T("Hands types, presses safe keys and clicks in one window you authorize, on your own desktop. It types at most 40 characters at a time and checks the window before and after each, never submits, never pastes, and reads back what it typed. Enter is always yours to press."),
			&h.app.appStage),
		row(protocol.OniHandsDesktop, i18n.T("Hands on a separate desktop"),
			i18n.T("Hyakki starts a desktop of its own, apart from yours, and Hands opens apps and works only there. Nothing it does reaches your desktop, and it never falls back to yours.")+"\n"+h.nestLine(stored.Hands),
			&h.app.deskStage),
	}
}

// nestLine says whether the separate desktop can run: what Hyakki found or
// lacks, once Hands at the desktop stage has said so, and otherwise when
// it looks
func (h *handsPanel) nestLine(stage string) string {
	if stage != protocol.OniHandsDesktop || !h.known {
		return i18n.T("It needs weston, niri and grim installed. Hyakki looks for them when this is on and never installs them.")
	}
	if len(h.status.Missing) == 0 {
		return i18n.T("weston, niri and grim found")
	}
	return handsMissingLine(h.status.Missing)
}

// handsMissingLine names the programs the separate desktop lacks
func handsMissingLine(missing []string) string {
	names := missing[0]
	if n := len(missing); n > 1 {
		names = strings.Join(missing[:n-1], ", ") + " " + i18n.T("and") + " " + missing[n-1]
	}
	return i18n.Tf("Needs %s", names)
}

// appGroups are the app or desktop stage's groups: the window to work in
// or the separate desktop, the task, the journal and what to know
func (h *handsPanel) appGroups(e *env.Env, stage string, on bool) []layout.Widget {
	th := e.Theme
	if on && h.read && !h.busy && (!h.app.tasksRead || h.app.tasksStale) {
		h.call(e, protocol.OniHandsParams{Action: protocol.OniHandsTasks})
	}
	var groups []layout.Widget
	if on {
		if stage == protocol.OniHandsApp {
			groups = append(groups, group(th, i18n.T("Pick a window"),
				i18n.T("Hands acts only in the window you authorize. Another window taking focus, its title, place or size changing, or any key you press pauses it."),
				h.windowRows(e), nil))
		} else {
			groups = append(groups, group(th, i18n.T("The separate desktop"),
				i18n.T("Hyakki's own desktop: a headless compositor with niri on it, in a folder of its own, started for a task and stopped after it."),
				h.nestRows(), nil))
		}
		groups = append(groups, group(th, i18n.T("Task"), h.appTaskCaption(stage), h.appTaskRows(e, stage), nil))
	}
	return append(groups,
		group(th, i18n.T("Tasks"),
			i18n.T("Each task's state, where it worked, its step and why it ended, kept on this computer. A row never holds the goal, a window's title, a web address or anything typed."),
			h.journalRows(e, on), nil),
		group(th, i18n.T("Before you use Hands"), "", h.appKnowRows(stage), nil),
	)
}

// windowRows are Choose a window and the windows it listed, each with
// Authorize this window; a window shows its title at the moment of
// choosing only, and one niri tiled is keys only
func (h *handsPanel) windowRows(e *env.Env) []look.Setting {
	th := e.Theme
	a := &h.app
	a.choose.Kind, a.choose.Small, a.choose.Label = kit.PlaqueTonal, true, i18n.T("Choose a window")
	authorized := i18n.T("No window is authorized yet.")
	if h.status.App != "" {
		authorized = i18n.Tf("Authorized: a window of %s.", h.status.App)
	}
	rows := []look.Setting{{Title: i18n.T("Windows on your desktop"), Detail: authorized,
		Control: func(gtx layout.Context) layout.Dimensions {
			if a.choose.Clicked(gtx) {
				h.call(e, protocol.OniHandsParams{Action: protocol.OniHandsWindows})
			}
			if h.busy || h.running() {
				gtx = gtx.Disabled()
			}
			return look.Named(gtx, i18n.T("Choose a window"), func(gtx layout.Context) layout.Dimensions { return a.choose.Layout(gtx, th.Theme) })
		}}}
	listed := make(map[string]bool, len(a.windows))
	for _, w := range a.windows {
		listed[w.ID] = true
		plaque := plaqueFor(&a.authorize, w.ID)
		plaque.Kind, plaque.Small, plaque.Label = kit.PlaqueTonal, true, i18n.T("Authorize this window")
		title := w.Title
		if title == "" {
			title = i18n.T("Untitled window")
		}
		detail := title
		if !w.Clickable {
			detail = i18n.T("keys only") + " · " + title + "\n" + i18n.T("The desktop does not say where this window is, so Hands may type and press keys in it but never click.")
		}
		id, app := w.ID, w.App
		rows = append(rows, look.Setting{Title: app, Detail: detail,
			Control: func(gtx layout.Context) layout.Dimensions {
				if plaque.Clicked(gtx) {
					h.call(e, protocol.OniHandsParams{Action: protocol.OniHandsSelect, Window: id})
				}
				if h.busy || h.running() {
					gtx = gtx.Disabled()
				}
				return look.Named(gtx, i18n.Tf("Authorize the window of %s", app), func(gtx layout.Context) layout.Dimensions { return plaque.Layout(gtx, th.Theme) })
			}})
	}
	prunePlaques(a.authorize, listed)
	if a.listed && len(a.windows) == 0 {
		rows = append(rows, look.Setting{Title: i18n.T("No window to work in"), Detail: i18n.T("Hands never works in Hyakki's own windows or in a password manager, a password prompt or the lock screen.")})
	}
	return rows
}

// nestRows say whether the separate desktop can run
func (h *handsPanel) nestRows() []look.Setting {
	switch {
	case h.problem != "":
		return []look.Setting{{Title: i18n.T("Hands cannot be read"), Detail: h.problem}}
	case !h.known:
		return []look.Setting{{Title: i18n.T("Looking for weston, niri and grim…")}}
	case len(h.status.Missing) > 0:
		return []look.Setting{{Title: handsMissingLine(h.status.Missing),
			Detail: i18n.T("Install them from your distribution's packages; Hyakki never installs them. Until then Hands has no desktop to work on, and it never uses yours instead.")}}
	}
	return []look.Setting{{Title: i18n.T("weston, niri and grim found"),
		Detail: i18n.T("The separate desktop starts when a task starts and stops when it ends or sits idle.")}}
}

// appTaskCaption is what a task may take and what each step sends
func (h *handsPanel) appTaskCaption(stage string) string {
	turns := feature.HandsMaxSteps()
	where := i18n.T("a picture of the window and its words")
	if stage == protocol.OniHandsDesktop {
		where = i18n.T("a picture of the separate desktop and its words")
	}
	return i18n.Tf("A task takes at most %d steps. Each step sends %s to the Ask route, behind the same question as asking about a window. Esc stops it at once.", turns, where)
}

func (h *handsPanel) appTaskRows(e *env.Env, stage string) []look.Setting {
	th := e.Theme
	var rows []look.Setting
	running := h.running()
	if running {
		h.stop.Kind, h.stop.Small, h.stop.Label = kit.PlaqueGold, true, i18n.T("Stop")
		h.resume.Kind, h.resume.Small, h.resume.Label = kit.PlaqueTonal, true, i18n.T("Resume")
		paused := h.status.State == protocol.OniHandsStatePaused
		rows = append(rows, look.Setting{Title: handsStateTitle(h.status), Detail: handsAppStateDetail(h.status),
			Control: func(gtx layout.Context) layout.Dimensions {
				if h.stop.Clicked(gtx) {
					h.call(e, protocol.OniHandsParams{Action: protocol.OniHandsStop})
				}
				if h.resume.Clicked(gtx) && paused {
					h.call(e, protocol.OniHandsParams{Action: protocol.OniHandsResume})
				}
				var children []layout.FlexChild
				if paused {
					children = append(children,
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							if h.busy {
								gtx = gtx.Disabled()
							}
							return look.Named(gtx, i18n.T("Resume Hands"), func(gtx layout.Context) layout.Dimensions { return h.resume.Layout(gtx, th.Theme) })
						}),
						layout.Rigid(layout.Spacer{Width: th.Sp.S2}.Layout))
				}
				children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return look.Named(gtx, i18n.T("Stop Hands"), func(gtx layout.Context) layout.Dimensions { return h.stop.Layout(gtx, th.Theme) })
				}))
				return layout.Flex{Alignment: layout.Middle}.Layout(gtx, children...)
			}})
		if card := h.cardRow(e); card != nil {
			rows = append(rows, *card)
		}
	}
	ready := h.status.State == protocol.OniHandsStateAuthorized || running
	placeholder, detail := i18n.T("What should Hands do in this window?"), i18n.Tf("Hands works in the window of %s and stops by itself when it is done.", h.status.App)
	if stage == protocol.OniHandsDesktop {
		placeholder, detail = i18n.T("What should Hands do on the separate desktop?"), i18n.T("Hands starts the separate desktop, works there and stops by itself when it is done.")
	}
	switch {
	case ready:
	case stage == protocol.OniHandsApp:
		detail = i18n.T("Authorize a window first. Oni must be on screen to start, so you can see every step.")
	default:
		detail = i18n.T("The separate desktop cannot run on this computer yet.")
	}
	h.goal.Label, h.goal.Placeholder, h.goal.Editor.Submit = i18n.T("Goal"), placeholder, true
	h.goal.Editor.SingleLine = true
	h.start.Kind, h.start.Small, h.start.Busy = kit.PlaqueGold, true, h.busy
	var title, startName string
	if running {
		title, startName = i18n.T("Queue another task"), i18n.T("Queue a Hands task")
		h.start.Label = i18n.T("Add to queue")
		detail = i18n.T("Queued tasks never start by themselves. Enter the goal again when you start one.")
	} else {
		title, startName = i18n.T("Give Hands a goal"), i18n.T("Start Hands")
		h.start.Label = i18n.T("Start")
	}
	rows = append(rows, look.Setting{Title: title, Detail: detail, Below: true,
		Control: func(gtx layout.Context) layout.Dimensions {
			submitted := false
			for {
				event, ok := h.goal.Editor.Update(gtx)
				if !ok {
					break
				}
				if _, ok := event.(widget.SubmitEvent); ok {
					submitted = true
				}
			}
			goal := strings.TrimSpace(h.goal.Text())
			if (h.start.Clicked(gtx) || submitted) && ready && goal != "" && !h.busy {
				h.call(e, protocol.OniHandsParams{Action: protocol.OniHandsStart, Goal: goal})
			}
			if !ready {
				gtx = gtx.Disabled()
			}
			return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
				layout.Rigid(func(gtx layout.Context) layout.Dimensions { return h.goal.Layout(gtx, th.Theme) }),
				layout.Rigid(layout.Spacer{Height: th.Sp.S2}.Layout),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					if goal == "" {
						gtx = gtx.Disabled()
					}
					return look.Named(gtx, startName, func(gtx layout.Context) layout.Dimensions { return h.start.Layout(gtx, th.Theme) })
				}),
			)
		}})
	if h.note != "" {
		rows = append(rows, look.Setting{Title: i18n.T("Last"), Detail: h.note})
	}
	return rows
}

// handsAppStateDetail says where a running task works and, paused, why
func handsAppStateDetail(s protocol.OniHandsResult) string {
	where := i18n.T("On the separate desktop.")
	if s.Scope == protocol.OniHandsScopeWindow {
		where = i18n.Tf("In the window of %s.", s.App)
	}
	switch s.State {
	case protocol.OniHandsStateWaiting:
		return where + " " + i18n.T("Hands points at something only you press.")
	case protocol.OniHandsStatePaused:
		return where + " " + handsPauseLabel(s.Pause) + " " + i18n.T("Resume here or on Oni's pill when you are back.")
	}
	return where
}

// journalRows are the task journal, newest first, with Start again on a
// task waiting or one a restart interrupted, and Delete all. A row holds
// codes and numbers only.
func (h *handsPanel) journalRows(e *env.Env, on bool) []look.Setting {
	th := e.Theme
	a := &h.app
	if !on {
		return []look.Setting{{Title: i18n.T("Nothing to list"), Detail: i18n.T("The journal is read while Hands is on.")}}
	}
	a.clear.Kind, a.clear.Small, a.clear.Label = kit.PlaqueQuiet, true, i18n.T("Delete all")
	rows := []look.Setting{{Title: i18n.T("Every task"),
		Detail: h.retentionLine(),
		Control: func(gtx layout.Context) layout.Dimensions {
			if a.clear.Clicked(gtx) && !h.running() {
				a.confirming, a.clearing.Dialog.Open, a.clearing.Danger = true, true, true
			}
			if h.busy || h.running() || len(a.tasks) == 0 {
				gtx = gtx.Disabled()
			}
			return look.Named(gtx, i18n.T("Delete every task in the journal"), func(gtx layout.Context) layout.Dimensions { return a.clear.Layout(gtx, th.Theme) })
		}}}
	switch {
	case !a.tasksRead:
		return rows
	case len(a.tasks) == 0:
		return append(rows, look.Setting{Title: i18n.T("No task yet")})
	}
	shown := make(map[string]bool, len(a.tasks))
	for _, t := range a.tasks {
		detail := fmt.Sprintf("%s · %s · %s · %s", handsScopeLabel(t.Scope, t.App), i18n.Tf("step %d", t.Step),
			handsOriginLabel(t.Origin), t.At.Local().Format("Mon 15:04"))
		if t.Reason != "" {
			detail += " · " + handsTaskReasonLabel(t.Reason)
		}
		row := look.Setting{Title: handsTaskStateLabel(t.State), Detail: detail}
		if t.State == "interrupted" || t.State == "upnext" {
			shown[t.ID] = true
			plaque := plaqueFor(&a.again, t.ID)
			plaque.Kind, plaque.Small, plaque.Label = kit.PlaqueTonal, true, i18n.T("Start again")
			id, scope := t.ID, t.Scope
			row.Detail += "\n" + i18n.T("Say the goal again in the goal field above, then Start again. Nothing starts by itself.")
			row.Control = func(gtx layout.Context) layout.Dimensions {
				goal := strings.TrimSpace(h.goal.Text())
				if plaque.Clicked(gtx) && goal != "" && !h.busy && !h.running() {
					h.call(e, protocol.OniHandsParams{Action: protocol.OniHandsStart, Task: id, Scope: scope, Goal: goal})
				}
				if h.busy || h.running() || goal == "" {
					gtx = gtx.Disabled()
				}
				return look.Named(gtx, i18n.T("Start this task again"), func(gtx layout.Context) layout.Dimensions { return plaque.Layout(gtx, th.Theme) })
			}
		}
		rows = append(rows, row)
	}
	prunePlaques(a.again, shown)
	return rows
}

// retentionLine says how long the journal keeps a task
func (h *handsPanel) retentionLine() string {
	n, err := handsNumbers()
	if err != nil {
		return i18n.T("Newest first.")
	}
	return i18n.Tf("Newest first. A task is kept %d days, at most %d of them.", n.Retention.Days, n.Retention.Rows)
}

// appKnowRows are what to know before Hands works outside a browser: a
// restart holds every task, a key pauses it, what it keeps and what it
// never does
func (h *handsPanel) appKnowRows(stage string) []look.Setting {
	rows := []look.Setting{
		{Title: i18n.T("Nothing goes on after a restart"),
			Detail: i18n.T("If Hyakki stops while a task runs, the task is held as interrupted: no window stays authorized and the separate desktop stops with it. Nothing goes on until you pick again, say the goal again and press Start.")},
		{Title: i18n.T("Executables are never run"),
			Detail: i18n.T("Hands shows a program or script in its folder and leaves it there. It opens apps only from your installed apps.")},
	}
	if stage == protocol.OniHandsApp {
		rows = append([]look.Setting{
			{Title: i18n.T("Any key you press pauses it"),
				Detail: i18n.T("While a task runs in your window, Hyakki notices that a key went down, never which key, and pauses. Esc stops it.")},
			{Title: i18n.T("Your pointer moves when it clicks"),
				Detail: i18n.T("A click moves your own pointer to the spot. Hands clicks only in a window your desktop places, after checking nothing covers the spot, and never drags.")},
			{Title: i18n.T("What a window can do"),
				Detail: i18n.T("A window's text can influence what Hands types. Hands never submits, never pastes, reads back what it typed and stops when it cannot read it, so check it before you press Enter yourself.")},
		}, rows...)
	} else {
		rows = append([]look.Setting{
			{Title: i18n.T("The separate desktop is not yours"),
				Detail: i18n.T("Apps opened there start without your desktop's windows and do not see them. Oni stays running while it is up, and it stops when the task ends or sits idle.")},
		}, rows...)
	}
	return rows
}

// appOverlay is the question before deleting the journal
func (h *handsPanel) appOverlay(gtx layout.Context, e *env.Env) bool {
	a := &h.app
	if !a.confirming {
		return false
	}
	body := i18n.T("Every task in the journal goes, with nothing kept. It cannot be brought back.")
	if a.clearing.Layout(gtx, e.Theme.Theme, i18n.T("Delete every task?"), body, i18n.T("Delete them"), "") {
		h.call(e, protocol.OniHandsParams{Action: protocol.OniHandsClear})
	}
	if a.clearing.Closed() {
		a.confirming = false
	}
	return a.clearing.Dialog.Open
}

// handsScopeLabel says where a task worked: a window by its app's id, or
// the separate desktop
func handsScopeLabel(scope, app string) string {
	switch scope {
	case protocol.OniHandsScopeWindow:
		if app != "" {
			return i18n.Tf("in %s", app)
		}
		return i18n.T("in a window")
	case protocol.OniHandsScopeNested, protocol.OniHandsScopeDesktop:
		return i18n.T("on the separate desktop")
	case protocol.OniHandsScopeBrowser:
		return i18n.T("in the Hands browser")
	}
	return scope
}

// handsTaskStateLabel names a journal row's state
func handsTaskStateLabel(state string) string {
	switch state {
	case "upnext":
		return i18n.T("Up next")
	case "working", "checking":
		return i18n.T("Working")
	case "needsyou":
		return i18n.T("Needs you")
	case "done":
		return i18n.T("Done")
	case "failed":
		return i18n.T("Failed")
	case "stopped":
		return i18n.T("Stopped")
	case "interrupted":
		return i18n.T("Interrupted")
	}
	return state
}

// handsOriginLabel says who started a task
func handsOriginLabel(origin string) string {
	switch origin {
	case "app":
		return i18n.T("from this app")
	case "presenter":
		return i18n.T("from Oni's pill")
	case "voice":
		return i18n.T("by voice")
	case "harness":
		return i18n.T("for a team member")
	}
	return origin
}

// handsTaskReasonLabel says why a task ended or paused: a pause's rule,
// a restart, or how it ended
func handsTaskReasonLabel(reason string) string {
	switch reason {
	case "restart":
		return i18n.T("Hyakki restarted")
	case protocol.OniHandsEndDone:
		return i18n.T("goal reached")
	case protocol.OniHandsEndStopped:
		return i18n.T("stopped")
	case protocol.OniHandsEndBudget:
		return i18n.T("out of steps")
	case protocol.OniHandsEndUnread:
		return i18n.T("what it typed could not be read back")
	case protocol.OniHandsEndClosed:
		return i18n.T("the window closed")
	case protocol.OniHandsEndAway:
		return i18n.T("not resumed in time")
	}
	label := handsPauseLabel(reason)
	return strings.TrimSuffix(label, ".")
}
