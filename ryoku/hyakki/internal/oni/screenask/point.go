// Package screenask holds the read-only screen-ask contracts: the prompts that
// frame one picture of the window Oni is perched on, the parsers that turn a
// model's reply into a point or a spoken answer, the code-side refusal of
// private fields, and how long an answer stays up. It reads text and boxes
// only: it never captures, clicks or types, and a reply that asks for an
// action is refused rather than carried out.
//
// Every box here is in fractions of the picture the model saw (X, Y the
// top-left corner, W, H the size, each 0..1), the same frame a perched
// strand target uses.
package screenask

import (
	"math"
	"regexp"
	"strconv"
	"strings"

	i18n "ryoku-i18n"
	"ryoku-rashin/internal/oni/feature"
	"ryoku-rashin/internal/sensitive"
)

// Point kinds.
const (
	KindElement = "element" // a listed control, by id
	KindBox     = "box"     // a box the model read off the picture
	KindNone    = "none"    // nothing in the picture fits the request
	KindAnswer  = "answer"  // a question about the picture, answered in words instead
)

// Confidence floors and the share of the picture a box may cover.
const (
	FloorElement    = 0.3  // a control from the list
	FloorBox        = 0.4  // a box that a listed control or read text agrees with
	FloorUnverified = 0.5  // a box nothing else agrees with
	SureAt          = 0.6  // at or above this the point is not a best guess
	MaxArea         = 0.25 // a box covering more of the picture is refused
)

// Refusal codes.
const (
	CodeMalformed      = "malformed"
	CodeUnknownControl = "unknown-control"
	CodeLowConfidence  = "low-confidence"
	CodeTooLarge       = "too-large"
	CodeOutside        = "outside"
	CodeSensitive      = "sensitive"
	CodeAction         = "action"
	CodeEmpty          = "empty"
)

// RefusalLine is what Oni says instead of pointing at a private field.
var RefusalLine = i18n.T("That field holds private details, so I am leaving it to you.")

// Size is the picture the model saw, in pixels. It turns a box the model gave
// in pixels into fractions.
type Size struct{ W, H int }

// Control is one listed control in the picture. Verified says the entry came
// from a source that checked it rather than from reading pixels.
type Control struct {
	ID, Role, Name string
	Box            feature.Box
	Verified       bool
}

// Word is one piece of text read in the picture.
type Word struct {
	Text string
	Box  feature.Box
}

// Pointing is a parsed pointing reply. Say is the sentence for the person;
// Box is set for element and box kinds. Verified says a listed control or
// read text agrees with the point; Sure says Conf reached SureAt, and a point
// that is not sure is shown as a best guess. Plan is the preview of later
// steps a guide reply may add: it is shown to the person only and is never
// fed back to the model, which hears of earlier steps through Input.Done.
type Pointing struct {
	Kind     string
	ID       string
	Box      *feature.Box
	Label    string
	Conf     float64
	Sure     bool
	Done     bool
	Note     string
	Say      string
	Verified bool
	Plan     []PlanItem
}

// PlanItem is one later step a guide reply expects: what to do, and the
// control it would use when the reply named one.
type PlanItem struct {
	Text    string
	Control string
}

// Bounds on one guide step. MaxPlan matches the spec's session.planNextMax.
const (
	MaxStepWords = 12 // words in a guide step's sentence
	MaxPlan      = 3  // later steps kept from a [NEXT] tag
	maxPlanText  = 80 // runes of one later step
)

// Refusal is why a reply was not turned into a point or an answer. Line is the
// plain sentence to show or say instead.
type Refusal struct {
	Code   string
	Reason string
	Line   string
}

func (r *Refusal) Error() string { return "screen ask refused (" + r.Code + "): " + r.Reason }

func refuse(code, reason string) *Refusal {
	var line string
	switch code {
	case CodeSensitive:
		line = RefusalLine
	case CodeLowConfidence:
		line = i18n.T("I cannot tell which one you mean, so I am holding back.")
	case CodeAction:
		line = i18n.T("I only look and point here; I will not act on the window.")
	case CodeUnknownControl, CodeOutside, CodeTooLarge:
		line = i18n.T("I could not tie that to a single spot on this window, so I am holding back.")
	case CodeEmpty:
		line = i18n.T("No answer came back. Try asking again.")
	default:
		line = i18n.T("The reply made no sense to me, so I am holding back.")
	}
	return &Refusal{Code: code, Reason: reason, Line: line}
}

const (
	maxReply     = 16 << 10
	maxSay       = 200
	maxNote      = 160
	maxLabel     = 60
	edgeSlack    = 0.02
	verifyIoU    = 0.3
	verifyTokens = 0.5
)

// A tag is [NAME] or [NAME:value]; a value holds no brackets or line breaks.
var tagPattern = regexp.MustCompile(`\[([A-Za-z]+)(?::([^\[\]\r\n]*))?\]`)

// actionTags are the verbs of a hands reply. Screen asks are read-only, so a
// reply carrying one is refused whole.
var actionTags = map[string]bool{"DO": true, "CLICK": true, "TYPE": true, "PRESS": true, "KEY": true, "SCROLL": true, "DRAG": true, "OPEN": true, "RUN": true, "ACT": true}

var replyTags = map[string]bool{"POINT": true, "CONF": true, "DONE": true, "NOTE": true, "ANSWER": true, "NEXT": true}

type tag struct {
	name, value string
	hasValue    bool
}

// scan splits a reply into its tags and the prose around them. Bracketed text
// that is not a reply tag stays prose.
func scan(reply string) (tags []tag, prose string, action string) {
	var b strings.Builder
	last := 0
	for _, m := range tagPattern.FindAllStringSubmatchIndex(reply, -1) {
		name := strings.ToUpper(reply[m[2]:m[3]])
		switch {
		case actionTags[name]:
			if action == "" {
				action = name
			}
		case replyTags[name]:
		default:
			continue
		}
		b.WriteString(reply[last:m[0]])
		b.WriteByte(' ')
		last = m[1]
		t := tag{name: name}
		if m[4] >= 0 {
			t.value, t.hasValue = reply[m[4]:m[5]], true
		}
		tags = append(tags, t)
	}
	b.WriteString(reply[last:])
	return tags, b.String(), action
}

// ParsePoint reads a pointing reply: one sentence, then one [POINT] tag and
// optional [CONF], [DONE], [NOTE] and [NEXT] tags, or [ANSWER] alone. A box
// may come in fractions or, when any value is above 1, in pixels of pic. A
// point at a private field, an unknown control, a box outside the picture or
// over MaxArea of it, a confidence under its floor, or a reply asking for an
// action is refused with a *Refusal. The first [NEXT] tag becomes the Plan.
func ParsePoint(reply string, controls []Control, ocr []Word, pic Size) (Pointing, error) {
	return parsePoint(reply, controls, ocr, pic, false)
}

// ParseStep reads one guide step: ParsePoint's grammar, held to what the
// guide prompt asks for. Unless the reply is [ANSWER], its sentence must be
// there and run to at most MaxStepWords words, and it may carry one [NEXT]
// tag at most. The plan keeps the first MaxPlan later steps; a step naming a
// private field is left out of it.
func ParseStep(reply string, controls []Control, ocr []Word, pic Size) (Pointing, error) {
	p, err := parsePoint(reply, controls, ocr, pic, true)
	if err != nil || p.Kind == KindAnswer {
		return p, err
	}
	_, prose, _ := scan(unfence(reply))
	switch n := len(strings.Fields(clean(prose, false))); {
	case n == 0:
		return Pointing{}, refuse(CodeMalformed, "step has no sentence")
	case n > MaxStepWords:
		return Pointing{}, refuse(CodeMalformed, "step sentence runs to "+strconv.Itoa(n)+" words, over "+strconv.Itoa(MaxStepWords))
	}
	return p, nil
}

func parsePoint(reply string, controls []Control, ocr []Word, pic Size, step bool) (Pointing, error) {
	if len(reply) > maxReply {
		return Pointing{}, refuse(CodeMalformed, "reply is too long")
	}
	tags, prose, action := scan(unfence(reply))
	if action != "" {
		return Pointing{}, refuse(CodeAction, "reply asks for the action "+action)
	}
	var (
		point, conf, note, next *tag
		answer, done            bool
	)
	for i := range tags {
		t := &tags[i]
		switch t.name {
		case "POINT":
			if point != nil {
				return Pointing{}, refuse(CodeMalformed, "more than one point")
			}
			point = t
		case "CONF":
			if conf != nil {
				return Pointing{}, refuse(CodeMalformed, "more than one confidence")
			}
			conf = t
		case "NOTE":
			note = t
		case "NEXT":
			if next != nil && step {
				return Pointing{}, refuse(CodeMalformed, "more than one plan")
			}
			if next == nil {
				next = t
			}
		case "DONE":
			done = true
		case "ANSWER":
			answer = true
		}
	}
	p := Pointing{Done: done, Say: cleanText(prose, maxSay)}
	if note != nil {
		p.Note = cleanText(note.value, maxNote)
	}
	if next != nil {
		p.Plan = parsePlan(next.value)
	}
	switch {
	case answer && point != nil:
		return Pointing{}, refuse(CodeMalformed, "reply both points and asks for an answer")
	case answer:
		return Pointing{Kind: KindAnswer}, nil
	case point == nil:
		return Pointing{}, refuse(CodeMalformed, "reply has no point")
	case !point.hasValue || strings.TrimSpace(point.value) == "":
		return Pointing{}, refuse(CodeMalformed, "point is empty")
	}
	if conf != nil {
		c, ok := parseConf(conf.value)
		if !ok {
			return Pointing{}, refuse(CodeMalformed, "confidence "+strconv.Quote(conf.value)+" is not a number from 0 to 1")
		}
		p.Conf = c
	}
	value := strings.TrimSpace(point.value)
	if strings.EqualFold(value, "none") {
		p.Kind, p.Conf = KindNone, 0
		return p, nil
	}
	floor := FloorElement
	if head, label, isBox := strings.Cut(value, ":"); isBox || strings.Contains(value, ",") {
		box, err := parseBox(head, pic)
		if err != nil {
			return Pointing{}, err
		}
		p.Kind, p.Box, p.Label = KindBox, &box, cleanText(label, maxLabel)
		p.Verified = agrees(box, p.Label, controls, ocr)
		floor = FloorUnverified
		if p.Verified {
			floor = FloorBox
		}
		if why, private := Sensitive(p.Label, ocr, box); private {
			return Pointing{}, refuse(CodeSensitive, why)
		}
		if box.W*box.H > MaxArea {
			return Pointing{}, refuse(CodeTooLarge, "box covers more than a quarter of the picture")
		}
	} else {
		c, ok := findControl(controls, value)
		if !ok {
			return Pointing{}, refuse(CodeUnknownControl, "no listed control has id "+strconv.Quote(cleanText(value, maxLabel)))
		}
		box := c.Box
		p.Kind, p.ID, p.Box, p.Label, p.Verified = KindElement, c.ID, &box, cleanText(c.Name, maxLabel), c.Verified
		if why, private := Sensitive(c.Role+" "+c.Name, ocr, box); private {
			return Pointing{}, refuse(CodeSensitive, why)
		}
	}
	if conf == nil {
		return Pointing{}, refuse(CodeLowConfidence, "reply gave no confidence")
	}
	if p.Conf < floor {
		return Pointing{}, refuse(CodeLowConfidence, "confidence "+strconv.FormatFloat(p.Conf, 'f', 2, 64)+" is under the floor "+strconv.FormatFloat(floor, 'f', 1, 64))
	}
	p.Sure = p.Conf >= SureAt
	return p, nil
}

// parsePlan reads a [NEXT] value: later steps split on |, each naming in
// trailing parentheses the control it would use. Hidden runes and markdown
// are stripped, empty steps and steps naming a private field are left out,
// and at most MaxPlan are kept.
func parsePlan(value string) []PlanItem {
	var plan []PlanItem
	for _, part := range strings.Split(value, "|") {
		text, control := strings.TrimSpace(part), ""
		if strings.HasSuffix(text, ")") {
			if i := strings.LastIndexByte(text, '('); i >= 0 {
				text, control = text[:i], text[i+1:len(text)-1]
			}
		}
		item := PlanItem{Text: cleanText(text, maxPlanText), Control: cleanText(control, maxLabel)}
		if item.Text == "" || sensitive.Label(item.Text+" "+item.Control) != "" {
			continue
		}
		if plan = append(plan, item); len(plan) == MaxPlan {
			break
		}
	}
	return plan
}

func findControl(controls []Control, id string) (Control, bool) {
	for _, c := range controls {
		if c.ID == id {
			return c, true
		}
	}
	return Control{}, false
}

// parseConf reads 0..1, or a percentage written with %.
func parseConf(s string) (float64, bool) {
	s = strings.TrimSpace(s)
	scale := 1.0
	if t, ok := strings.CutSuffix(s, "%"); ok {
		s, scale = strings.TrimSpace(t), 100
	}
	v, err := strconv.ParseFloat(s, 64)
	if err != nil || math.IsNaN(v) || math.IsInf(v, 0) {
		return 0, false
	}
	v /= scale
	return v, v >= 0 && v <= 1
}

// parseBox reads x,y,w,h. All four at most 1 are fractions; any above 1 makes
// them pixels of pic.
func parseBox(s string, pic Size) (feature.Box, error) {
	parts := strings.Split(s, ",")
	if len(parts) != 4 {
		return feature.Box{}, refuse(CodeMalformed, "box needs x,y,w,h")
	}
	var v [4]float64
	pixels := false
	for i, part := range parts {
		f, err := strconv.ParseFloat(strings.TrimSpace(part), 64)
		if err != nil || math.IsNaN(f) || math.IsInf(f, 0) {
			return feature.Box{}, refuse(CodeMalformed, "box value "+strconv.Quote(cleanText(part, 20))+" is not a number")
		}
		v[i] = f
		pixels = pixels || f > 1
	}
	if pixels {
		if pic.W <= 0 || pic.H <= 0 {
			return feature.Box{}, refuse(CodeMalformed, "box is in pixels but the picture size is unknown")
		}
		w, h := float64(pic.W), float64(pic.H)
		v = [4]float64{v[0] / w, v[1] / h, v[2] / w, v[3] / h}
	}
	b := feature.Box{X: v[0], Y: v[1], W: v[2], H: v[3]}
	if b.W <= 0 || b.H <= 0 {
		return feature.Box{}, refuse(CodeMalformed, "box has no size")
	}
	if b.X < -edgeSlack || b.Y < -edgeSlack || b.X+b.W > 1+edgeSlack || b.Y+b.H > 1+edgeSlack {
		return feature.Box{}, refuse(CodeOutside, "box reaches outside the picture")
	}
	// trim the slack so the box lies inside the picture
	x0, y0 := max(b.X, 0), max(b.Y, 0)
	x1, y1 := min(b.X+b.W, 1), min(b.Y+b.H, 1)
	return feature.Box{X: x0, Y: y0, W: x1 - x0, H: y1 - y0}, nil
}

// agrees says whether a listed control or the text read inside the box backs
// up a box point: a control overlapping it whose name shares most of the
// label's words, or words inside it that do.
func agrees(box feature.Box, label string, controls []Control, ocr []Word) bool {
	want := tokens(label)
	if len(want) == 0 {
		return false
	}
	for _, c := range controls {
		if iou(box, c.Box) >= verifyIoU && overlap(want, tokens(c.Name)) >= verifyTokens {
			return true
		}
	}
	var inside []string
	for _, w := range ocr {
		if contains(box, center(w.Box)) {
			inside = append(inside, tokens(w.Text)...)
		}
	}
	return overlap(want, inside) >= verifyTokens
}

// overlap is the share of want found in got.
func overlap(want, got []string) float64 {
	if len(want) == 0 {
		return 0
	}
	have := make(map[string]bool, len(got))
	for _, g := range got {
		have[g] = true
	}
	n := 0
	for _, w := range want {
		if have[w] {
			n++
		}
	}
	return float64(n) / float64(len(want))
}

func iou(a, b feature.Box) float64 {
	ix := min(a.X+a.W, b.X+b.W) - max(a.X, b.X)
	iy := min(a.Y+a.H, b.Y+b.H) - max(a.Y, b.Y)
	if ix <= 0 || iy <= 0 {
		return 0
	}
	in := ix * iy
	return in / (a.W*a.H + b.W*b.H - in)
}

type pt struct{ x, y float64 }

func center(b feature.Box) pt { return pt{b.X + b.W/2, b.Y + b.H/2} }

func contains(b feature.Box, p pt) bool {
	return p.x >= b.X && p.x <= b.X+b.W && p.y >= b.Y && p.y <= b.Y+b.H
}
