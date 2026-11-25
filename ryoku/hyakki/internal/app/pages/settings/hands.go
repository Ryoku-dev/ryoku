package settings

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"gioui.org/layout"
	"gioui.org/widget"

	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/kit"
	"ryoku-i18n"

	"ryoku-rashin/internal/app/env"
	"ryoku-rashin/internal/app/look"
	"ryoku-rashin/internal/oni/feature"
	"ryoku-rashin/internal/oni/hands/consent"
	"ryoku-rashin/internal/oni/hands/proto"
	"ryoku-rashin/internal/paths"
	"ryoku-rashin/internal/protocol"
)

// handsPanel is Settings' page for Oni Hands: the app-only switch, the
// Hands browser found and its Open, the tab picker with Authorize, a task
// started toward a goal with Stop and Resume, the step disclosure waiting
// for an answer, what to know before using it with Forget, and the log.
// Where Hands stands is read once per visit while Hands may run, again after
// each press and whenever a Hands frame says it moved; the frames are heard
// on an observer connection of its own, open only while the page is drawn.
// Nothing here keeps a goal, a tab's title or page text.
type handsPanel struct {
	stage kit.Toggle

	open, list, start, stop, resume, erase, again, allow, deny kit.Plaque
	authorize                                                  map[string]*kit.Plaque
	goal                                                       kit.TextField

	// where Hands stood at the last answer, the tabs listed at the last
	// tabs, whether the answer is current, whether any came since Hands
	// turned on (a frame asks again without the page going blank), a call
	// out, why the state could not be read, and the line the last press
	// came to
	status   protocol.OniHandsResult
	tabs     []protocol.OniHandsTab
	read     bool
	known    bool
	readOn   bool
	busy     bool
	problem  string
	note     string
	log      reflexList[protocol.OniHandsRow]
	logStale bool

	// the question before forgetting the Hands browser
	forgetting kit.Confirm
	confirming bool

	// the frames: drawn this frame, the watch, which one it is, when the last
	// failed, and the card waiting: a step's disclosure to answer, or a
	// handoff that is the user's own press
	drawn    bool
	watch    context.CancelFunc
	watchID  int
	failedAt time.Time
	card     *feature.ConsentFrame

	// the app and desktop stages' part of the page (hands_app.go), and the
	// stage the state was read at
	app       handsAppPanel
	readStage string
}

// forget reads Hands again the next time the page shows and closes the watch
func (h *handsPanel) forget() {
	h.read, h.known, h.log.read, h.tabs = false, false, false, nil
	h.app.forget()
	h.stopWatch()
}

// handsOn says whether Hands may run under these settings: Oni on, looking
// at windows, and Hands at a stage, in a browser, in one app window or on
// a separate desktop
func handsOn(settings protocol.Settings) bool {
	oni := protocol.EffectiveOni(settings)
	id := feature.HandsStageFeature(oni.Hands)
	return id != "" && feature.Decide(feature.Request{Feature: id}, oni).Allow
}

// running says whether a task is under way: working, waiting on the user or
// paused
func (h *handsPanel) running() bool {
	switch h.status.State {
	case protocol.OniHandsStateWorking, protocol.OniHandsStateWaiting, protocol.OniHandsStatePaused:
		return true
	}
	return false
}

// call sends one oni.hands action; the answer is where Hands stands after it
func (h *handsPanel) call(e *env.Env, params protocol.OniHandsParams) {
	if h.busy {
		return
	}
	h.busy = true
	if params.Action != protocol.OniHandsStatus {
		h.note = ""
	}
	result := new(protocol.OniHandsResult)
	e.Call(protocol.MOniHands, params, result, func(err error) {
		h.busy, h.read = false, true
		if err != nil {
			if params.Action == protocol.OniHandsStatus {
				h.problem = err.Error()
			} else {
				h.note = err.Error()
			}
			return
		}
		h.problem, h.known = "", true
		tabs, windows, tasks := result.Tabs, result.Windows, result.Tasks
		result.Tabs, result.Windows, result.Tasks = nil, nil, nil
		h.status = *result
		h.app.heard(params.Action, windows, tasks)
		if result.Line != "" {
			h.note = result.Line
		}
		switch params.Action {
		case protocol.OniHandsTabs:
			h.tabs = tabs
		case protocol.OniHandsOpen:
			// the tabs are what the user came to open it for
			h.call(e, protocol.OniHandsParams{Action: protocol.OniHandsTabs})
		case protocol.OniHandsStart:
			// the goal is typed words: it is not kept once Hands has it
			h.goal.SetText("")
		case protocol.OniHandsStop, protocol.OniHandsResume, protocol.OniHandsForget:
			h.logStale = true
		}
		switch params.Action {
		case protocol.OniHandsStart, protocol.OniHandsStop, protocol.OniHandsResume, protocol.OniHandsSelect:
			h.app.tasksStale = true
		}
		if h.status.State == protocol.OniHandsStateClosed {
			h.tabs = nil
		}
		if !h.running() {
			h.card = nil
		}
	})
}

func (h *handsPanel) readLog(e *env.Env) {
	l := &h.log
	l.busy, h.logStale = true, false
	result := new(protocol.OniHandsLogResult)
	e.Call(protocol.MOniHandsLog, protocol.Empty{}, result, func(err error) {
		l.busy, l.read = false, true
		if err != nil {
			l.problem = err.Error()
			return
		}
		l.items, l.problem = result.Rows, ""
	})
}

// answer says yes or no to the step's disclosure, once
func (h *handsPanel) answer(e *env.Env, yes bool) {
	if h.card == nil || !strings.HasPrefix(h.card.ID, feature.HandsStepCardPrefix) {
		return
	}
	id := h.card.ID
	h.card = nil
	answer := "no"
	if yes {
		answer = "yes"
	}
	e.Call(protocol.MOniConsentAnswer, protocol.OniConsentAnswerParams{ID: id, Answer: answer}, nil, func(err error) {
		if err != nil {
			h.note = err.Error()
		}
	})
}

// settle runs once a frame, after the page is drawn: the watch runs while
// the page was drawn and Hands may run, and stops the frame it was not
func (h *handsPanel) settle(gtx layout.Context, e *env.Env) {
	want := h.drawn && handsOn(e.Snapshot().Settings)
	h.drawn = false
	switch {
	case !want:
		h.stopWatch()
	case h.watch == nil && e.Post != nil && e.Client != nil && e.Client.Connected() &&
		(h.failedAt.IsZero() || gtx.Now.Sub(h.failedAt) >= watchRetry):
		h.startWatch(gtx, e)
	}
}

func (h *handsPanel) startWatch(gtx layout.Context, e *env.Env) {
	ctx, cancel := context.WithCancel(context.Background())
	h.watchID++
	id, client, post, started := h.watchID, e.Client, e.Post, gtx.Now
	h.watch = cancel
	go func() {
		err := client.WatchOni(ctx, func(raw json.RawMessage) {
			payload, err := feature.DecodeFrame(raw)
			if err != nil {
				return
			}
			post(func() {
				if h.watchID == id && h.watch != nil {
					h.hear(payload)
				}
			})
		})
		cancel()
		post(func() {
			if h.watchID != id {
				return
			}
			h.watch, h.card = nil, nil
			if err != nil || ctx.Err() == nil {
				h.failedAt = started
			}
		})
	}()
}

func (h *handsPanel) stopWatch() {
	if h.watch == nil {
		return
	}
	h.watch()
	h.watch, h.card = nil, nil
	h.watchID++
}

// hear applies one frame: Hands moved, so where it stands is read again; a
// step's disclosure or a handoff is shown; an answered card goes
func (h *handsPanel) hear(payload feature.Payload) {
	switch f := payload.(type) {
	case *feature.HandsFrame:
		h.read = false
		if !f.Active {
			h.card, h.logStale, h.app.tasksStale = nil, true, true
		}
	case *feature.ConsentFrame:
		step := f.Feature == feature.Inspect && strings.HasPrefix(f.ID, feature.HandsStepCardPrefix)
		handoff := handsCardFeature(f.Feature) && strings.HasPrefix(f.ID, feature.HandsCardPrefix)
		if step || handoff {
			card := *f
			h.card = &card
		}
	case *feature.ConsentAnswerFrame:
		if h.card != nil && h.card.ID == f.ID {
			h.card = nil
		}
	}
}

// overlay is the question before forgetting the Hands browser, or before
// deleting the task journal
func (h *handsPanel) overlay(gtx layout.Context, e *env.Env) bool {
	if h.appOverlay(gtx, e) {
		return true
	}
	if !h.confirming {
		return false
	}
	body := i18n.T("Its profile goes: every sign-in, cookie, saved password and its history. The next time it opens, it is a new browser with nothing signed in.")
	if h.forgetting.Layout(gtx, e.Theme.Theme, i18n.T("Forget the Hands browser?"), body, i18n.T("Forget it"), "") {
		h.call(e, protocol.OniHandsParams{Action: protocol.OniHandsForget})
	}
	if h.forgetting.Closed() {
		h.confirming = false
	}
	return h.forgetting.Dialog.Open
}

// handsBlocks are the Hands page's head and its groups
func (p *Page) handsBlocks(gtx layout.Context, e *env.Env, blocks []layout.Widget) []layout.Widget {
	th := e.Theme
	h := &p.hands
	h.drawn = true
	settings := e.Snapshot().Settings
	on := handsOn(settings)
	stage := settings.Oni.Hands
	if on != h.readOn || stage != h.readStage {
		h.read, h.known, h.readOn, h.log.read, h.tabs, h.card = false, false, on, false, nil, nil
		h.status, h.problem, h.readStage = protocol.OniHandsResult{}, "", stage
		h.app.forget()
	}
	if on && !h.read && !h.busy {
		h.call(e, protocol.OniHandsParams{Action: protocol.OniHandsStatus})
	}
	if h.logStale && h.log.read {
		h.log.read = false
	}
	if h.log.due(on) {
		h.readLog(e)
	}

	head := func(gtx layout.Context) layout.Dimensions { return sectionHead(gtx, th, sectionHands) }
	groups := []layout.Widget{
		group(th, i18n.T("Where Hands works"),
			i18n.T("Hands works in one place at a time: turning one on turns the others off. Every step shows on Oni's pill; Esc, the pill, this page or saying stop ends it. Hands never drives your whole desktop."),
			h.stageRows(e, settings), nil),
	}
	switch stage {
	case protocol.OniHandsApp, protocol.OniHandsDesktop:
		groups = append(groups, h.appGroups(e, stage, on)...)
	default:
		if rows := h.browserRows(e, settings); len(rows) > 0 {
			groups = append(groups, group(th, i18n.T("The Hands browser"),
				i18n.T("Hands works in a browser window of its own, in one tab you authorize, toward a goal you give it."), rows, nil))
		}
		// the tab and the task wait for a browser to work in; until Hands has
		// answered they stand, so the page does not grow under the reader
		if on && h.problem == "" && (!h.known || h.status.Browser != "" || h.status.State != protocol.OniHandsStateClosed) {
			groups = append(groups,
				group(th, i18n.T("Authorize a tab"),
					i18n.T("Hands acts only in the tab you authorize and only on that site. Switching tab, window or site pauses it."), h.tabRows(e), nil),
				group(th, i18n.T("Task"), h.taskCaption(), h.taskRows(e), nil),
			)
		}
		groups = append(groups, group(th, i18n.T("Before you use Hands"), "", h.knowRows(e), nil))
	}
	groups = append(groups,
		group(th, i18n.T("Hands log"),
			i18n.T("What each step came to, kept on this computer. A row names the tool and the rule behind it, never a control's name, page text or anything typed."), h.logRows(e, on), nil),
	)
	return spacedBlocks(th, append(blocks, head), groups...)
}

// browserRows are the browser found with its Open, while Hands in a
// browser may run
func (h *handsPanel) browserRows(e *env.Env, settings protocol.Settings) []look.Setting {
	th := e.Theme
	if settings.Oni.Hands != protocol.OniHandsBrowser || !handsOn(settings) {
		return nil
	}
	h.open.Kind, h.open.Small, h.open.Label, h.open.Busy = kit.PlaqueGold, true, i18n.T("Open"), h.busy
	title, browserDetail := i18n.T("Browser"), i18n.T("Looking for a Chromium-family browser…")
	switch {
	case h.problem != "":
		title, browserDetail = i18n.T("Hands cannot be read"), h.problem
	case !h.known:
	case h.status.Browser == "":
		title = i18n.T("No Chromium-family browser found")
		browserDetail = i18n.T("Install Chromium, Google Chrome, Brave, Vivaldi or Microsoft Edge from your distribution's packages. Firefox, Flatpak and snap builds cannot be driven, and Hyakki never installs a browser.")
	default:
		title = i18n.Tf("Browser: %s", h.status.Browser)
		browserDetail = i18n.T("Hands opens it in a window of its own, with a profile kept apart from your browser. Your own browser and its profile are never driven.")
		if h.status.State != protocol.OniHandsStateClosed {
			browserDetail += "\n" + i18n.T("The Hands browser is open.")
		}
	}
	return []look.Setting{{Title: title, Detail: browserDetail,
		Control: func(gtx layout.Context) layout.Dimensions {
			if h.open.Clicked(gtx) {
				h.call(e, protocol.OniHandsParams{Action: protocol.OniHandsOpen})
			}
			if h.busy || !h.known || h.status.Browser == "" || h.status.State != protocol.OniHandsStateClosed {
				gtx = gtx.Disabled()
			}
			return look.Named(gtx, i18n.T("Open the Hands browser"), func(gtx layout.Context) layout.Dimensions { return h.open.Layout(gtx, th.Theme) })
		}}}
}

// tabRows are the Hands browser's tabs as last listed, each with Authorize;
// a tab shows its title and origin at the moment of choosing only
func (h *handsPanel) tabRows(e *env.Env) []look.Setting {
	th := e.Theme
	if h.status.State == "" || h.status.State == protocol.OniHandsStateClosed {
		return []look.Setting{{Title: i18n.T("The Hands browser is closed"), Detail: i18n.T("Open it above, sign in where you need to, then list its tabs here.")}}
	}
	h.list.Kind, h.list.Small, h.list.Label = kit.PlaqueTonal, true, i18n.T("List tabs")
	authorized := i18n.T("No tab is authorized yet.")
	if h.status.Origin != "" {
		authorized = i18n.Tf("Authorized: %s.", h.status.Origin)
	}
	rows := []look.Setting{{Title: i18n.T("Tabs in the Hands browser"), Detail: authorized,
		Control: func(gtx layout.Context) layout.Dimensions {
			if h.list.Clicked(gtx) {
				h.call(e, protocol.OniHandsParams{Action: protocol.OniHandsTabs})
			}
			if h.busy {
				gtx = gtx.Disabled()
			}
			return look.Named(gtx, i18n.T("List the tabs"), func(gtx layout.Context) layout.Dimensions { return h.list.Layout(gtx, th.Theme) })
		}}}
	listed := make(map[string]bool, len(h.tabs))
	for _, tab := range h.tabs {
		listed[tab.ID] = true
		plaque := plaqueFor(&h.authorize, tab.ID)
		plaque.Kind, plaque.Small, plaque.Label = kit.PlaqueTonal, true, i18n.T("Authorize")
		title := tab.Title
		if title == "" {
			title = i18n.T("Untitled page")
		}
		id, origin := tab.ID, tab.Origin
		rows = append(rows, look.Setting{Title: title, Detail: origin,
			Control: func(gtx layout.Context) layout.Dimensions {
				if plaque.Clicked(gtx) {
					h.call(e, protocol.OniHandsParams{Action: protocol.OniHandsAuthorize, Tab: id})
				}
				if h.busy || h.running() {
					gtx = gtx.Disabled()
				}
				return look.Named(gtx, i18n.Tf("Authorize the tab on %s", origin), func(gtx layout.Context) layout.Dimensions { return plaque.Layout(gtx, th.Theme) })
			}})
	}
	prunePlaques(h.authorize, listed)
	if len(h.tabs) == 0 {
		rows = append(rows, look.Setting{Title: i18n.T("No tab listed"), Detail: i18n.T("List the tabs once the page you want is open in the Hands browser.")})
	}
	return rows
}

// taskCaption is what a task may take and what each step sends
func (h *handsPanel) taskCaption() string {
	return i18n.Tf("A task takes at most %d steps. Each step sends a picture of the tab, its visible text and its controls' names to the Ask route, behind the same question as asking about a window.", feature.HandsMaxSteps())
}

// taskRows are where the task stands with Stop and Resume, the card
// waiting, and the goal field with Start while none runs
func (h *handsPanel) taskRows(e *env.Env) []look.Setting {
	th := e.Theme
	var rows []look.Setting
	if h.running() {
		h.stop.Kind, h.stop.Small, h.stop.Label = kit.PlaqueGold, true, i18n.T("Stop")
		h.resume.Kind, h.resume.Small, h.resume.Label = kit.PlaqueTonal, true, i18n.T("Resume")
		paused := h.status.State == protocol.OniHandsStatePaused
		rows = append(rows, look.Setting{Title: handsStateTitle(h.status), Detail: handsStateDetail(h.status),
			Control: func(gtx layout.Context) layout.Dimensions {
				if h.stop.Clicked(gtx) {
					h.call(e, protocol.OniHandsParams{Action: protocol.OniHandsStop})
				}
				if h.resume.Clicked(gtx) && paused {
					h.call(e, protocol.OniHandsParams{Action: protocol.OniHandsResume})
				}
				children := []layout.FlexChild{}
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
	} else {
		ready := h.status.State == protocol.OniHandsStateAuthorized
		h.goal.Label, h.goal.Placeholder, h.goal.Editor.Submit = i18n.T("Goal"), i18n.T("What should Hands do in this tab?"), true
		h.goal.Editor.SingleLine = true
		h.start.Kind, h.start.Small, h.start.Label, h.start.Busy = kit.PlaqueGold, true, i18n.T("Start"), h.busy
		detail := i18n.Tf("Hands works in %s and stops by itself when it is done.", h.status.Origin)
		if !ready {
			detail = i18n.T("Authorize a tab first. Oni must be on screen to start, so you can see every step.")
		}
		rows = append(rows, look.Setting{Title: i18n.T("Give Hands a goal"), Detail: detail, Below: true,
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
						return look.Named(gtx, i18n.T("Start Hands"), func(gtx layout.Context) layout.Dimensions { return h.start.Layout(gtx, th.Theme) })
					}),
				)
			}})
	}
	if h.note != "" {
		rows = append(rows, look.Setting{Title: i18n.T("Last"), Detail: h.note})
	}
	return rows
}

// cardRow is the card waiting on the task: a step's disclosure the user
// allows or not, or a handoff, which only the user's own press in the page
// answers
func (h *handsPanel) cardRow(e *env.Env) *look.Setting {
	th := e.Theme
	card := h.card
	if card == nil {
		return nil
	}
	switch card.Feature {
	case feature.HandsBrowser:
		return &look.Setting{Title: i18n.T("Your turn"), Detail: card.Question + "\n" + i18n.T("Press it yourself in the Hands browser. Nothing you say or click here presses it.")}
	case feature.HandsApp, feature.HandsDesktop:
		if h.status.Scope == protocol.OniHandsScopeWindow {
			return &look.Setting{Title: i18n.T("Your turn"), Detail: card.Question + "\n" + i18n.T("Press it yourself in the window. Nothing you say or click here presses it.")}
		}
		return &look.Setting{Title: i18n.T("Your turn"), Detail: card.Question + "\n" + i18n.T("Hands never presses it on the separate desktop. Stop the task if you do not want it done.")}
	}
	h.allow.Kind, h.allow.Small, h.allow.Label = kit.PlaqueGold, true, i18n.T("Allow")
	h.deny.Kind, h.deny.Small, h.deny.Label = kit.PlaqueQuiet, true, i18n.T("Not now")
	where := i18n.T("It stays on this computer.")
	if card.Destination != feature.EgressLocal {
		where = i18n.Tf("It goes to %s.", card.Route)
	}
	return &look.Setting{Title: i18n.T("Send this step?"), Detail: card.Question + "\n" + where, Below: true,
		Control: func(gtx layout.Context) layout.Dimensions {
			if h.allow.Clicked(gtx) {
				h.answer(e, true)
			}
			if h.deny.Clicked(gtx) {
				h.answer(e, false)
			}
			return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return look.Named(gtx, i18n.T("Allow this step"), func(gtx layout.Context) layout.Dimensions { return h.allow.Layout(gtx, th.Theme) })
				}),
				layout.Rigid(layout.Spacer{Width: th.Sp.S2}.Layout),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return look.Named(gtx, i18n.T("Do not allow this step"), func(gtx layout.Context) layout.Dimensions { return h.deny.Layout(gtx, th.Theme) })
				}),
			)
		}}
}

// knowRows are the residency, the profile and what a page can do, said
// plainly, and Forget
func (h *handsPanel) knowRows(e *env.Env) []look.Setting {
	th := e.Theme
	h.erase.Kind, h.erase.Small, h.erase.Label = kit.PlaqueQuiet, true, i18n.T("Forget")
	profile := paths.Short(filepath.Join(paths.StateDir(), "hands-browser"))
	open := h.status.State != "" && h.status.State != protocol.OniHandsStateClosed
	forgetDetail := i18n.T("Deletes its profile with every sign-in, cookie and its history. It cannot be brought back.")
	switch {
	case !h.readOn:
		forgetDetail += "\n" + i18n.T("Turn Hands on to forget it: Oni answers for its browser only while Hands is on.")
	case open:
		forgetDetail += "\n" + i18n.T("Close the Hands browser first.")
	}
	return []look.Setting{
		{Title: i18n.T("Oni stays running while the Hands browser is open"),
			Detail: i18n.T("The Hands browser is Oni's own window, so Oni does not rest while it is open, even with no task. Close the window and Oni rests as usual.")},
		{Title: i18n.T("A profile of its own"),
			Detail: i18n.Tf("The Hands browser keeps its own profile in %s, so you sign in once. Its history, cookies and sign-ins stay there, apart from your browser. Nothing you type and no page text goes into Hyakki's own files.", profile)},
		{Title: i18n.T("What a page can do"),
			Detail: i18n.T("A page can influence what Hands types into a field. Hands never submits, sends or pays, reads back what it typed and shows you every step, so check the fields before you press the button yourself.")},
		{Title: i18n.T("Forget the Hands browser"), Detail: forgetDetail,
			Control: func(gtx layout.Context) layout.Dimensions {
				if h.erase.Clicked(gtx) && !open {
					h.confirming, h.forgetting.Dialog.Open, h.forgetting.Danger = true, true, true
				}
				if open || h.busy || !h.known {
					gtx = gtx.Disabled()
				}
				return look.Named(gtx, i18n.T("Forget the Hands browser and its profile"), func(gtx layout.Context) layout.Dimensions { return h.erase.Layout(gtx, th.Theme) })
			}},
	}
}

// logRows are the ledger, newest first: when, which task and step, the
// tool, what came of it and the rule behind it. No row holds words.
func (h *handsPanel) logRows(e *env.Env, on bool) []look.Setting {
	th := e.Theme
	l := &h.log
	if !on {
		return []look.Setting{{Title: i18n.T("Nothing to list"), Detail: i18n.T("The log is read while Hands is on.")}}
	}
	h.again.Kind, h.again.Small, h.again.Label = kit.PlaqueQuiet, true, i18n.T("Read again")
	rows := []look.Setting{{Title: i18n.T("What Hands did"), Detail: i18n.T("The newest first."),
		Control: func(gtx layout.Context) layout.Dimensions {
			if h.again.Clicked(gtx) && !l.busy {
				h.readLog(e)
			}
			if l.busy {
				gtx = gtx.Disabled()
			}
			return look.Named(gtx, i18n.T("Read the Hands log again"), func(gtx layout.Context) layout.Dimensions { return h.again.Layout(gtx, th.Theme) })
		}}}
	switch {
	case l.problem != "":
		return append(rows, look.Setting{Title: i18n.T("The log cannot be read"), Detail: l.problem})
	case !l.read:
		return rows
	case len(l.items) == 0:
		return append(rows, look.Setting{Title: i18n.T("Hands has not acted yet")})
	}
	for _, row := range l.items {
		outcome := handsOutcomeLabel(row.Outcome)
		if row.Reason != "" {
			outcome = i18n.Tf("%s: %s", outcome, handsReasonLabel(row.Reason))
		}
		detail := fmt.Sprintf("%s · %s · %s · %s · %s", row.At.Local().Format("Mon 15:04:05"), i18n.Tf("task %s", row.Task),
			i18n.Tf("step %d", row.Step), outcome, i18n.Tf("%d ms", row.Ms))
		rows = append(rows, look.Setting{Title: handsToolLabel(row.Tool), Detail: detail})
	}
	return rows
}

// handsStateTitle says where a running task stands
func handsStateTitle(s protocol.OniHandsResult) string {
	switch s.State {
	case protocol.OniHandsStateWaiting:
		return i18n.Tf("Waiting for you · step %d of %d", s.Step, s.Max)
	case protocol.OniHandsStatePaused:
		return i18n.Tf("Paused · step %d of %d", s.Step, s.Max)
	}
	return i18n.Tf("Working · step %d of %d", s.Step, s.Max)
}

// handsStateDetail says where it works and, paused, why
func handsStateDetail(s protocol.OniHandsResult) string {
	where := i18n.Tf("In %s.", s.Origin)
	switch s.State {
	case protocol.OniHandsStateWaiting:
		return where + " " + i18n.T("Hands points at a control only you press.")
	case protocol.OniHandsStatePaused:
		return where + " " + handsPauseLabel(s.Pause) + " " + i18n.T("Resume here or on Oni's pill when you are back.")
	}
	return where
}

// handsPauseLabel says why a task paused
func handsPauseLabel(pause string) string {
	switch pause {
	case protocol.OniHandsPauseFocus:
		return i18n.T("Another window took focus.")
	case protocol.OniHandsPauseWindow:
		return i18n.T("The Hands browser's window could not be told apart.")
	case protocol.OniHandsPauseNavigation:
		return i18n.T("The tab went to another site.")
	case protocol.OniHandsPauseHidden:
		return i18n.T("The tab went out of view.")
	case protocol.OniHandsPauseCovered:
		return i18n.T("Oni could not be seen working.")
	case protocol.OniHandsPauseDialog:
		return i18n.T("The page opened a dialog.")
	case protocol.OniHandsPausePopup:
		return i18n.T("The page opened a popup.")
	case protocol.OniHandsPauseDownload:
		return i18n.T("The page started a download.")
	case protocol.OniHandsPauseOutput:
		return i18n.T("The window went to another screen, or your screens changed.")
	case protocol.OniHandsPauseTitle:
		return i18n.T("The window's title changed.")
	case protocol.OniHandsPauseGeometry:
		return i18n.T("The window moved or changed size.")
	case protocol.OniHandsPauseSelf:
		return i18n.T("Hyakki's own window took focus.")
	case protocol.OniHandsPausePrivate:
		return i18n.T("A private window, such as a password manager or a password prompt, came in front.")
	case protocol.OniHandsPauseKey:
		return i18n.T("You pressed a key.")
	case protocol.OniHandsPausePresenter:
		return i18n.T("Oni was not on screen to show the work.")
	}
	return i18n.Tf("Paused (%s).", pause)
}

// handsToolLabel names a step's tool in the user's terms
func handsToolLabel(tool string) string {
	switch tool {
	case proto.ToolClick:
		return i18n.T("Click a control")
	case proto.ToolFill:
		return i18n.T("Fill a field")
	case proto.ToolSelect:
		return i18n.T("Pick an option")
	case proto.ToolScroll:
		return i18n.T("Scroll")
	case proto.ToolWait:
		return i18n.T("Wait for the page")
	case proto.ToolDone:
		return i18n.T("Finish")
	case proto.ToolAskUser:
		return i18n.T("Ask you")
	case proto.ToolType:
		return i18n.T("Type")
	case proto.ToolPress:
		return i18n.T("Press a key")
	case proto.ToolOpenApp:
		return i18n.T("Open an app")
	case proto.ToolFindFiles:
		return i18n.T("Find files")
	case proto.ToolShowFile:
		return i18n.T("Show a file's folder")
	}
	return tool
}

// handsOutcomeLabel says what became of a step
func handsOutcomeLabel(outcome string) string {
	switch outcome {
	case protocol.OniHandsOutcomeDone:
		return i18n.T("Done")
	case protocol.OniHandsOutcomeHandoff:
		return i18n.T("Handed to you")
	case protocol.OniHandsOutcomeRefused:
		return i18n.T("Refused")
	case protocol.OniHandsOutcomeFailed:
		return i18n.T("Failed")
	}
	return outcome
}

// handsReasonLabel names the rule behind a handoff, refusal or failure;
// a code it does not know is shown as the code, which names a rule only
func handsReasonLabel(reason string) string {
	switch consent.Reason(reason) {
	case consent.ReasonSubmits:
		return i18n.T("it submits")
	case consent.ReasonRiskyLabel:
		return i18n.T("it sends, pays, deletes or signs out")
	case consent.ReasonPaymentForm:
		return i18n.T("a payment form")
	case consent.ReasonUnreadable:
		return i18n.T("its label could not be read")
	case consent.ReasonUnknownRole, consent.ReasonUnknownVerb:
		return i18n.T("Hands could not tell what it does")
	case consent.ReasonClaim:
		return i18n.T("the model's claim was not trusted")
	case consent.ReasonPrivate:
		return i18n.T("a private field")
	case consent.ReasonFileInput:
		return i18n.T("a file picker")
	case consent.ReasonOtherOrigin:
		return i18n.T("another site")
	case consent.ReasonHidden:
		return i18n.T("a hidden control")
	case consent.ReasonSecret:
		return i18n.T("it looked like a secret")
	case consent.ReasonSelf:
		return i18n.T("one of Hyakki's own windows")
	case consent.ReasonPrivateApp:
		return i18n.T("a private app")
	case consent.ReasonControlText:
		return i18n.T("a control character")
	case consent.ReasonRiskyKey:
		return i18n.T("a key that may submit or close")
	case consent.ReasonRefusedKey:
		return i18n.T("a key Hands never presses")
	}
	return reason
}
