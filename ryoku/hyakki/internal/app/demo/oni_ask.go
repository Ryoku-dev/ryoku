package demo

import (
	"bytes"
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/jpeg"
	"slices"
	"strings"
	"time"

	"ryoku-i18n"

	"ryoku-rashin/internal/oni/feature"
	"ryoku-rashin/internal/oni/screenask"
	"ryoku-rashin/internal/protocol"
)

// FeatureOniLocal is no feature either: with it the demo's screen asks are
// answered by a model on this computer instead of a hosted one.
const FeatureOniLocal = "oni-local"

// The demo's screen-ask routes, one on this computer and one hosted.
const (
	demoOniLocalRoute  = "rashin-models/gemma-3-4b-it"
	demoOniHostedRoute = "openrouter/qwen/qwen2.5-vl-72b-instruct"
	// how long a question about a picture waits for its answer, as the daemon's
	demoOniConsentTTL = 120 * time.Second
	demoOniOutput     = "DP-1"
	demoOniOutputW    = 2560
)

// a desktop window the demo knows; a nil box is one the desktop does not place
type demoWindow struct {
	app, title string
	box        *protocol.OniRect
}

// the demo desktop: a floating notes window Oni can perch on, a tiled
// terminal the desktop gives no position for, and nothing else
var demoWindows = map[string]demoWindow{
	"21": {app: "org.example.Notes", title: "Packing list", box: &protocol.OniRect{X: 640, Y: 220, W: 960, H: 620}},
	"22": {app: "org.example.Terminal", title: "make", box: nil},
}

// the window the demo starts perched on when Oni is on
const demoPerchedWindow = "21"

// oniFixtures is what the demo's Oni remembers between calls: where it is
// perched, the pictures waiting for an answer and what became of the ones
// answered, which routes were allowed for the session, and the guide, its
// keep offer, the kept guides and the armed pick.
type oniFixtures struct {
	local   bool
	perch   *protocol.OniPerchResult
	seq     uint64
	asked   int
	pending map[string]demoConsent
	// what became of each answered or expired picture: sent or dropped
	fates   map[string]string
	allowed map[string]bool
	// the demo's own clock, so a test can let a question expire
	now func() time.Time
	// the voice loop the voice calls drive
	voice voiceFixtures
	// the guide, keep and pick calls' state
	guides guideFixtures
	// the reminders, routines and ledger the reflex calls read
	reflex reflexFixtures
	// the Hands browser, its tab and task the Hands calls drive
	hands handsFixtures
	// the rewrite card, guard and assignment views the Phase 16 calls drive
	phase16 phase16Fixtures
	// the optional parts, activity, bridge programs and pill lines the
	// Phase 17 calls read
	phase17 phase17Fixtures
	// the demo computer is under the local decision hardware gate
	smallComputer bool
}

// demoConsent is a picture waiting for its yes: about a typed question, a
// guide's step (goal and step set) or a picked spot (spot set, in fractions
// of the window).
type demoConsent struct {
	consent  protocol.OniConsent
	question string
	asked    time.Time
	goal     string
	step     int
	spot     *protocol.OniRect
}

const (
	demoFateSent    = "sent"
	demoFateDropped = "dropped"
)

func newOniFixtures(on, local bool) oniFixtures {
	o := oniFixtures{local: local, pending: map[string]demoConsent{}, fates: map[string]string{}, allowed: map[string]bool{}, now: time.Now, guides: newGuideFixtures(), reflex: newReflexFixtures(), phase17: newPhase17Fixtures()}
	o.phase16.rows = slices.Clone(demoAssignmentRows)
	if on {
		perch, _ := o.perchOn(demoPerchedWindow)
		o.perch = &perch
	}
	return o
}

func (o *oniFixtures) perchOn(id string) (protocol.OniPerchResult, error) {
	window, ok := demoWindows[id]
	if !ok {
		return protocol.OniPerchResult{}, &protocol.Error{Code: protocol.CodeNotFound, Reason: feature.PerchClosed,
			Message: i18n.Tf("There is no window %s on the desktop; it may have closed.", id)}
	}
	if window.box == nil {
		return protocol.OniPerchResult{}, &protocol.Error{Code: protocol.CodeUnavailable, Reason: feature.PerchNoGeometry,
			Message: i18n.Tf("The desktop does not say where %s is, so Oni cannot perch on it. A floating window works.", window.title)}
	}
	o.seq++
	box := *window.box
	corner := "top-left"
	if box.X+box.W/2 >= demoOniOutputW/2 {
		corner = "top-right"
	}
	return protocol.OniPerchResult{Seq: o.seq, Window: id, App: window.app, Title: window.title, Output: demoOniOutput, Corner: corner, Box: &box}, nil
}

func (o *oniFixtures) route() (route, destination string) {
	if o.local {
		return demoOniLocalRoute, feature.EgressLocal
	}
	return demoOniHostedRoute, "hosted"
}

func (s *server) callOniAsk(method string, raw json.RawMessage) (any, error) {
	o := &s.data.oni
	settings := protocol.EffectiveOni(s.data.snapshot.Settings)
	oniOff := &protocol.Error{Code: protocol.CodeDisabled, Reason: feature.PerchOniOff, Message: i18n.T("Oni is off. Turn it on in Hyakki's Settings first.")}
	// Oni turned off lets go of its window and of every picture waiting
	if !settings.Master {
		o.perch = nil
		o.dropAll()
	}
	switch method {
	case protocol.MOniPerch:
		var params protocol.OniPerchParams
		if err := decode(raw, &params); err != nil {
			return nil, err
		}
		if !settings.Master {
			return nil, oniOff
		}
		if params.Window == "" {
			return nil, protocol.Errorf(protocol.CodeBadRequest, "%s", i18n.T("Name the window Oni should perch on."))
		}
		perch, err := o.perchOn(params.Window)
		if err != nil {
			return nil, err
		}
		if !params.Hover {
			// a new perch drops whatever still waited about the old one
			o.perch = &perch
			o.dropAll()
		}
		return perch, nil
	case protocol.MOniDetach:
		o.perch = nil
		o.dropAll()
		return protocol.Empty{}, nil
	case protocol.MOniInspect, protocol.MOniAsk:
		var params protocol.OniAskParams
		if err := decode(raw, &params); err != nil {
			return nil, err
		}
		text := strings.TrimSpace(params.Text)
		if method == protocol.MOniAsk && (text == "" || !protocol.ValidOniText(text, 2000, true)) {
			return nil, protocol.Errorf(protocol.CodeBadRequest, "%s", i18n.T("Ask Oni something about the window, in at most 2000 characters."))
		}
		switch {
		case !settings.Master:
			return nil, oniOff
		case !settings.Inspect:
			return nil, protocol.Errorf(protocol.CodeDisabled, "%s", i18n.T("Looking at windows is off. Turn it on in Settings, under Oni."))
		case o.perch == nil:
			return nil, protocol.Errorf(protocol.CodeUnavailable, "%s", i18n.T("Oni is not perched on a window. Drag it onto one first."))
		}
		route, destination := o.route()
		if settings.Disclosure != protocol.OniDisclosureAlways && o.allowed[route] {
			answer := demoOniAnswer(text, route, destination)
			return protocol.OniAskResult{Status: protocol.OniAskAnswered, Answer: &answer}, nil
		}
		return o.ask(o.perch.Window, demoConsent{question: text}), nil
	case protocol.MOniConsentAnswer:
		var params protocol.OniConsentAnswerParams
		if err := decode(raw, &params); err != nil {
			return nil, err
		}
		if params.Answer != "yes" && params.Answer != "no" {
			return nil, protocol.Errorf(protocol.CodeBadRequest, "%s", i18n.T("Answer yes or no."))
		}
		pending, ok := o.pending[params.ID]
		if ok && o.now().Sub(pending.asked) >= demoOniConsentTTL {
			delete(o.pending, params.ID)
			o.fates[params.ID], ok = demoFateDropped, false
		}
		if !ok {
			// the daemon says only not_found; the demo also says what became of the picture, for tests
			message := i18n.T("That question is not waiting any more.")
			switch o.fates[params.ID] {
			case demoFateSent:
				message = i18n.T("That picture was already sent.")
			case demoFateDropped:
				message = i18n.T("That picture was dropped; nothing was sent.")
			}
			return nil, protocol.Errorf(protocol.CodeNotFound, "%s", message)
		}
		delete(o.pending, params.ID)
		if params.Answer == "no" {
			o.fates[params.ID] = demoFateDropped
			// a guide whose picture is refused ends there
			if pending.step > 0 {
				o.dropGuide()
			}
			return protocol.OniAskResult{Status: protocol.OniAskDeclined}, nil
		}
		o.fates[params.ID] = demoFateSent
		o.allowed[pending.consent.Route] = true
		return o.answered(pending), nil
	}
	return nil, notFound(method)
}

// ask holds the picture of the window named windowID for its yes and
// returns the question
func (o *oniFixtures) ask(windowID string, pending demoConsent) protocol.OniAskResult {
	route, destination := o.route()
	window := demoWindows[windowID]
	o.asked++
	id := fmt.Sprintf("oni-consent-%d", o.asked)
	question := i18n.Tf("Send this picture of “%s” to %s?", window.title, route)
	if destination == feature.EgressLocal {
		question = i18n.Tf("Show this picture of “%s” to %s on this computer?", window.title, route)
	}
	pending.consent = protocol.OniConsent{
		ID: id, Destination: destination, Route: route, Path: "toplevel", ScopeExact: true,
		Disclosure: i18n.T("Only this window's own pixels, taken through the desktop's window capture. Nothing else on the screen is read."),
		Question:   question, Preview: demoOniPreview(), Width: int(window.box.W), Height: int(window.box.H),
		ExpiresMs: demoOniConsentTTL.Milliseconds(),
	}
	pending.asked = o.now()
	o.pending[id] = pending
	consent := pending.consent
	return protocol.OniAskResult{Status: protocol.OniAskConsent, Consent: &consent}
}

// answered is what came back once a picture was sent
func (o *oniFixtures) answered(pending demoConsent) protocol.OniAskResult {
	route, destination := pending.consent.Route, pending.consent.Destination
	switch {
	case pending.step > 0:
		return o.guideStep(pending.goal, pending.step, false, route, destination)
	case pending.spot != nil:
		return o.selected(*pending.spot, pending.question, route, destination)
	}
	answer := demoOniAnswer(pending.question, route, destination)
	return protocol.OniAskResult{Status: protocol.OniAskAnswered, Answer: &answer}
}

// letting go drops every picture still waiting for an answer, the guide,
// its keep offer and an armed pick
func (o *oniFixtures) dropAll() {
	for id := range o.pending {
		delete(o.pending, id)
		o.fates[id] = demoFateDropped
	}
	o.guides.drop()
}

// demoOniAnswer is what the demo's model says about the packing list
func demoOniAnswer(question, route, destination string) protocol.OniAnswer {
	answer := protocol.OniAnswer{Route: route, Destination: destination, Path: "toplevel"}
	lower := strings.ToLower(question)
	switch {
	case strings.Contains(lower, "password"):
		answer.Mode, answer.Refused = "point", "sensitive"
		answer.Text = screenask.RefusalLine
	case strings.Contains(lower, "where") || strings.Contains(lower, "which"):
		answer.Mode, answer.Seq = "point", 1
		answer.Point = &protocol.OniRect{X: .08, Y: .42, W: .4, H: .07}
		answer.Text = i18n.T("The rain jacket is the fourth line, already ticked.")
	case question == "":
		answer.Mode = "describe"
		answer.Text = i18n.T("A notes window with a packing list of nine items, three of them ticked.")
	default:
		answer.Mode = "ask"
		answer.Text = i18n.T("It is a packing list with nine items. Three are ticked: passport, charger and the rain jacket.")
	}
	return answer
}

// demoOniPreview is the thumbnail of the notes window, drawn the same every time
func demoOniPreview() []byte {
	const w, h = 480, 310
	picture := image.NewRGBA(image.Rect(0, 0, w, h))
	fill := func(r image.Rectangle, c color.RGBA) {
		draw.Draw(picture, r, &image.Uniform{c}, image.Point{}, draw.Src)
	}
	fill(picture.Bounds(), color.RGBA{0xF4, 0xF1, 0xEA, 0xFF})
	fill(image.Rect(0, 0, w, 22), color.RGBA{0x2B, 0x2A, 0x33, 0xFF})
	fill(image.Rect(200, 8, 280, 14), color.RGBA{0xC8, 0xC4, 0xBC, 0xFF})
	fill(image.Rect(24, 40, 200, 54), color.RGBA{0x3A, 0x36, 0x30, 0xFF})
	ink, tick, box := color.RGBA{0x8A, 0x84, 0x7A, 0xFF}, color.RGBA{0xE0, 0x6A, 0x2C, 0xFF}, color.RGBA{0xB8, 0xB2, 0xA8, 0xFF}
	widths := [...]int{140, 96, 170, 120, 88, 150, 110, 132, 74}
	for row, width := range widths {
		y := 76 + row*24
		mark := box
		if row == 0 || row == 2 || row == 3 {
			mark = tick
		}
		fill(image.Rect(24, y, 36, y+12), mark)
		fill(image.Rect(48, y+3, 48+width, y+9), ink)
	}
	var out bytes.Buffer
	_ = jpeg.Encode(&out, picture, &jpeg.Options{Quality: 80})
	return out.Bytes()
}
