package demo

import (
	"encoding/json"
	"slices"
	"time"

	"ryoku-i18n"

	"ryoku-rashin/internal/config"
	"ryoku-rashin/internal/oni/feature"
	"ryoku-rashin/internal/protocol"
)

// FeatureOniHands is no feature: it starts the demo as FeatureOni with
// Hands in a browser on, for the Settings page of Oni Hands.
const FeatureOniHands = "oni-hands"

// FeatureOniHandsNoBrowser is FeatureOniHands on a machine with no
// Chromium-family browser installed, so the Hands browser cannot open.
const FeatureOniHandsNoBrowser = "oni-hands-no-browser"

// FeatureOniHandsPaused is FeatureOniHands with the Hands browser open, its
// first fixture tab authorized and a task paused because another window
// took focus, so Resume has something to go on from.
const FeatureOniHandsPaused = "oni-hands-paused"

// demoHandsBrowser is the Chromium-family program the demo machine has
const demoHandsBrowser = "chromium"

// the step the demo's paused task stopped at
const demoHandsPausedStep = 4

// the id of the demo's step disclosure card
const demoHandsStepCard = feature.HandsStepCardPrefix + "00000000000000d1"

// handsFixtures is what the demo's Hands answers from: a Hands browser that
// opens on its two fixture tabs, one tab authorized at a time, a task that
// starts at its first step and a short ledger told from Epoch, so every run
// answers the same. Nothing drives a page, and only FeatureOniHandsPaused
// starts paused: the demo has no browser. A task's first step asks its
// disclosure on a card, as the daemon does, and the frames it shows go to
// every watcher.
type handsFixtures struct {
	rows []protocol.OniHandsRow
	// browser is the program found, empty when none is
	browser string
	open    bool
	tab     *protocol.OniHandsTab
	state   string
	step    int
	pause   string
	card    *feature.ConsentFrame
	told    []feature.Payload
	// the app and desktop stages' own (oni_hands_app.go)
	handsAppPart
}

// newHandsFixtures is the demo's Hands for its features
func newHandsFixtures(features []string) handsFixtures {
	h := handsFixtures{browser: demoHandsBrowser, handsAppPart: newHandsAppPart(features)}
	h.rows = slices.Clone(demoHandsRows)
	switch {
	case slices.Contains(features, FeatureOniHandsNoBrowser):
		h.browser = ""
	case slices.Contains(features, FeatureOniHandsPaused):
		tab := demoHandsTabs[0]
		h.open, h.tab = true, &tab
		h.state, h.step, h.pause = protocol.OniHandsStatePaused, demoHandsPausedStep, protocol.OniHandsPauseFocus
	case slices.Contains(features, FeatureOniHandsAppPaused):
		h.startPaused()
	}
	return h
}

// demoHandsFeature says whether id is one of the Hands demo switches
func demoHandsFeature(id string) bool {
	_, app := demoHandsAppFeature(id)
	return app || id == FeatureOniHands || id == FeatureOniHandsNoBrowser || id == FeatureOniHandsPaused
}

// the tabs the demo's Hands browser shows
var demoHandsTabs = []protocol.OniHandsTab{
	{ID: "D1A2B3C4", Origin: "http://127.0.0.1:8137", Title: "Contact us"},
	{ID: "E5F6A7B8", Origin: "https://example.org", Title: "Example Domain"},
}

// the demo's Hands ledger, newest first: codes only, never a word typed
var demoHandsRows = []protocol.OniHandsRow{
	{At: Epoch.Add(-time.Minute), Task: "t-2", Step: 3, Tool: "click", Outcome: protocol.OniHandsOutcomeHandoff, Reason: "submits", Ms: 4},
	{At: Epoch.Add(-time.Minute - 5*time.Second), Task: "t-2", Step: 2, Tool: "fill", Outcome: protocol.OniHandsOutcomeDone, Ms: 210},
	{At: Epoch.Add(-time.Minute - 9*time.Second), Task: "t-2", Step: 1, Tool: "fill", Outcome: protocol.OniHandsOutcomeDone, Ms: 180},
	{At: Epoch.Add(-time.Hour), Task: "t-1", Step: 1, Tool: "fill", Outcome: protocol.OniHandsOutcomeRefused, Reason: "private-field", Ms: 2},
}

// demoHandsPatch refuses a settings patch whose hands stage the daemon
// refuses: the app and desktop stages are not in this build. Keys match as
// the settings decoder matches them.
func demoHandsPatch(patch json.RawMessage) error {
	var p struct {
		Oni *struct {
			Hands *string `json:"hands"`
		} `json:"oni"`
	}
	if json.Unmarshal(patch, &p) != nil || p.Oni == nil || p.Oni.Hands == nil {
		return nil
	}
	return config.CheckOniHands(*p.Oni.Hands)
}

// the demo's Hands calls answer as the daemon will: each needs Hands at
// its stage, and only the app, which is all the demo serves, sends them.
// The app and desktop stages answer from oni_hands_app.go.
func (s *server) callOniHands(method string, raw json.RawMessage) (any, error) {
	h := &s.data.oni.hands
	params, err := feature.DecodeCall(method, raw)
	if err != nil {
		return nil, protocol.Errorf(protocol.CodeBadRequest, "%s", i18n.Tf("The Oni call is invalid: %v.", err))
	}
	oni := protocol.EffectiveOni(s.data.snapshot.Settings)
	id := feature.HandsBrowser
	switch oni.Hands {
	case protocol.OniHandsApp:
		id = feature.HandsApp
	case protocol.OniHandsDesktop:
		id = feature.HandsDesktop
	}
	if err := demoRefusal(oni, id); err != nil {
		return nil, err
	}
	if method == protocol.MOniHandsLog {
		return protocol.OniHandsLogResult{Rows: slices.Clone(s.data.oni.hands.rows)}, nil
	}
	notOpen := func() error {
		return protocol.Errorf(protocol.CodeUnavailable, "%s", i18n.T("The Hands browser is not open. Open it first."))
	}
	p := params.(*protocol.OniHandsParams)
	if id != feature.HandsBrowser {
		return s.callOniHandsApp(oni.Hands, p)
	}
	switch p.Action {
	case protocol.OniHandsWindows, protocol.OniHandsSelect, protocol.OniHandsTasks, protocol.OniHandsClear:
		return nil, protocol.Errorf(protocol.CodeConflict, "%s", i18n.T("Hands in a browser has no windows to pick and no task journal."))
	}
	switch p.Action {
	case protocol.OniHandsOpen:
		if h.browser == "" {
			return nil, protocol.Errorf(protocol.CodeUnavailable, "%s", i18n.T("No Chromium-family browser is installed. Hands does not install one."))
		}
		h.open = true
	case protocol.OniHandsTabs:
		if !h.open {
			return nil, notOpen()
		}
		result := h.result()
		result.Tabs = slices.Clone(demoHandsTabs)
		return result, nil
	case protocol.OniHandsAuthorize:
		if !h.open {
			return nil, notOpen()
		}
		i := slices.IndexFunc(demoHandsTabs, func(t protocol.OniHandsTab) bool { return t.ID == p.Tab })
		if i < 0 {
			return nil, protocol.Errorf(protocol.CodeNotFound, "%s", i18n.T("That tab is not open in the Hands browser."))
		}
		tab := demoHandsTabs[i]
		h.drop()
		h.tab, h.state, h.step, h.pause = &tab, "", 0, ""
	case protocol.OniHandsStart:
		switch {
		case h.tab == nil:
			return nil, protocol.Errorf(protocol.CodeConflict, "%s", i18n.T("Authorize a tab in the Hands browser first."))
		case h.state != "":
			return nil, protocol.Errorf(protocol.CodeConflict, "%s", i18n.T("Hands is working on a task. Stop it first."))
		}
		h.stage, h.state, h.step = "", protocol.OniHandsStateWorking, 1
		route, destination := s.data.oni.route()
		card := feature.ConsentFrame{ID: demoHandsStepCard, Feature: feature.Inspect, Class: feature.ConsentDisclosure,
			Destination: destination, Route: route, ExpiresMs: feature.ConsentTTL.Milliseconds(),
			Question: i18n.Tf("Send a picture of the tab on %s, its visible text and its controls' names for step 1?", h.tab.Origin)}
		h.card = &card
		h.told = append(h.told, h.frame(), card)
	case protocol.OniHandsStop:
		h.drop()
		h.state, h.step, h.pause = "", 0, ""
		h.told = append(h.told, h.frame())
	case protocol.OniHandsResume:
		if h.state != protocol.OniHandsStatePaused {
			return nil, protocol.Errorf(protocol.CodeConflict, "%s", i18n.T("Hands is not paused."))
		}
		h.state, h.pause = protocol.OniHandsStateWorking, ""
		h.told = append(h.told, h.frame())
	case protocol.OniHandsForget:
		if h.open {
			return nil, protocol.Errorf(protocol.CodeConflict, "%s", i18n.T("Close the Hands browser first, then forget it."))
		}
		result := h.result()
		result.Line = i18n.T("The Hands browser is forgotten: its profile, sign-ins and history are deleted.")
		return result, nil
	}
	return h.result(), nil
}

// result is where the demo's Hands stand
func (h *handsFixtures) result() protocol.OniHandsResult {
	switch {
	case !h.open:
		return protocol.OniHandsResult{State: protocol.OniHandsStateClosed, Browser: h.browser}
	case h.tab == nil:
		return protocol.OniHandsResult{State: protocol.OniHandsStateOpen, Browser: h.browser}
	case h.state == "":
		return protocol.OniHandsResult{State: protocol.OniHandsStateAuthorized, Browser: h.browser, Origin: h.tab.Origin}
	}
	return protocol.OniHandsResult{State: h.state, Browser: h.browser, Origin: h.tab.Origin, Step: h.step, Max: feature.HandsMaxSteps(), Pause: h.pause}
}

// frame is the hands frame the demo's Hands shows: active while a task runs
func (h *handsFixtures) frame() feature.HandsFrame {
	if h.stage != "" {
		return h.appFrame()
	}
	f := feature.HandsFrame{Stage: protocol.OniHandsBrowser, Reason: feature.ReasonAllowed}
	if h.state == "" {
		return f
	}
	f.Active, f.Scope, f.Phase, f.Step, f.Max, f.Pause = true, protocol.OniHandsScopeBrowser, h.state, h.step, feature.HandsMaxSteps(), h.pause
	return f
}

// drop lets the step's card go unanswered, as a stopped task's does
func (h *handsFixtures) drop() {
	if h.card == nil {
		return
	}
	h.told = append(h.told, feature.ConsentAnswerFrame{ID: h.card.ID, Answer: "no"})
	h.card = nil
}

// answer takes the user's answer to the step's card: yes lets the step go
// on, no ends the task, as a refused disclosure does
func (h *handsFixtures) answer(raw json.RawMessage) (any, error) {
	var params protocol.OniConsentAnswerParams
	if err := decode(raw, &params); err != nil {
		return nil, err
	}
	if params.Answer != "yes" && params.Answer != "no" {
		return nil, protocol.Errorf(protocol.CodeBadRequest, "%s", i18n.T("Answer yes or no."))
	}
	if h.card == nil || h.card.ID != params.ID {
		return nil, protocol.Errorf(protocol.CodeNotFound, "%s", i18n.T("That question is not waiting any more."))
	}
	h.card = nil
	h.told = append(h.told, feature.ConsentAnswerFrame{ID: params.ID, Answer: params.Answer})
	if params.Answer == "no" {
		if h.stage != "" {
			h.endTask("stopped", protocol.OniHandsEndStopped)
			return protocol.OniAskResult{Status: protocol.OniAskDeclined}, nil
		}
		h.state, h.step, h.pause = "", 0, ""
	}
	h.told = append(h.told, h.frame())
	return protocol.OniAskResult{Status: protocol.OniAskDeclined}, nil
}

// opening are the frames a watcher starts from while a task runs
func (h *handsFixtures) opening() []feature.Payload {
	if h.state == "" {
		return nil
	}
	frames := []feature.Payload{h.frame()}
	if h.card != nil {
		frames = append(frames, *h.card)
	}
	return frames
}

// takeFrames are the frames shown since the last take, oldest first
func (h *handsFixtures) takeFrames() []feature.Payload {
	told := h.told
	h.told = nil
	return told
}
