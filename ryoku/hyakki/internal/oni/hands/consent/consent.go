// Package consent sorts every step Hands might take in its one authorized
// browser tab into what Hands may do itself, what it hands to the person, and
// what it refuses outright.
//
// There is no step Hands takes after a yes. A safe step (typing into an
// ordinary field, an ordinary click, a scroll, a short wait) is announced and
// run. Anything that sends, pays, deletes, signs out or otherwise cannot be
// taken back, and anything these rules cannot place, is a handoff: Hands
// points at the control and waits, and the person presses it themselves. A
// spoken yes never stands in for that press. Private fields (passwords, card
// details, identity numbers, one-time codes), file pickers, hidden controls
// and controls inside a frame from another site are refused.
//
// Outside a browser tab there is no page to read, only a window and what
// the screen shows. ClassifyApp sorts those steps with the same floor, in
// three layers for private surfaces: the window (Hyakki's own surfaces,
// password managers, polkit and askpass prompts, lock screens, and a title
// naming a private field are refused), what the screen reads at the point
// (a private word there is refused), and the model's claim, which only
// raises. Wayland names no text field, so the window is the floor.
//
// The model may say how risky it thinks a step is. That claim can only move a
// step towards caution, never away from it: these rules are the floor.
package consent

import (
	"strings"
	"unicode"

	"ryoku-rashin/internal/oni/feature"
	"ryoku-rashin/internal/oni/hands/apprules"
	"ryoku-rashin/internal/oni/screenask"
)

// Class is what Hands may do with one step. The zero value is Handoff, so a
// step nobody classified is never run.
type Class uint8

const (
	// Handoff: Hands points at the control and the person presses it.
	Handoff Class = iota
	// Safe: Hands announces the step and runs it.
	Safe
	// Refused: Hands neither runs it nor points at it.
	Refused
)

func (c Class) String() string {
	switch c {
	case Safe:
		return "safe"
	case Refused:
		return "refused"
	default:
		return "handoff"
	}
}

// SpokenMayAnswer says whether speech may approve a step of class c. It never
// may: a safe step needs no approval, a handoff is the person's own press,
// and a refusal has nothing to approve.
func SpokenMayAnswer(Class) bool { return false }

// Reason is a short code for why a step got its class. It names a rule, never
// page text, so it may be kept in the activity log.
type Reason string

const (
	ReasonSafe        Reason = "safe"
	ReasonRiskyLabel  Reason = "risky-label"
	ReasonSubmits     Reason = "submits"
	ReasonPaymentForm Reason = "payment-form"
	ReasonUnreadable  Reason = "unreadable-label"
	ReasonUnknownRole Reason = "unknown-role"
	ReasonUnknownVerb Reason = "unknown-verb"
	ReasonClaim       Reason = "model-claim"
	ReasonPrivate     Reason = "private-field"
	ReasonFileInput   Reason = "file-input"
	ReasonOtherOrigin Reason = "other-origin"
	ReasonHidden      Reason = "hidden-control"
	ReasonSelf        Reason = "self-surface"
	ReasonPrivateApp  Reason = "private-app"
	ReasonSecret      Reason = "secret-text"
	ReasonControlText Reason = "control-text"
	ReasonRiskyKey    Reason = "risky-key"
	ReasonRefusedKey  Reason = "refused-key"
)

// The verbs a step may carry. Anything else is a handoff.
const (
	VerbClick  = "click"
	VerbFill   = "fill"
	VerbSelect = "select"
	VerbScroll = "scroll"
	VerbWait   = "wait"
	// Outside a browser tab: in one live window or on the nested desktop.
	VerbType      = "type"
	VerbPress     = "press"
	VerbOpenApp   = "open_app"
	VerbFindFiles = "find_files"
	VerbShowFile  = "show_file"
)

// How risky the model says a step is.
const (
	ClaimNormal    = "normal"
	ClaimSensitive = "sensitive"
	ClaimCritical  = "critical"
)

// Subject is the control a step acts on, as the page describes it. Every
// string is page text: it is only matched against, never obeyed.
type Subject struct {
	Role         string // accessibility role, such as button, link or textbox
	Name         string // accessible name: the control's label
	InputType    string // the input's type attribute, "" when it is not an input
	Autocomplete string // the autocomplete attribute, as written
	Option       string // select: the label of the option being chosen
	Submits      bool   // pressing it submits a form
	PaymentForm  bool   // it sits in a form that takes payment
	OtherOrigin  bool   // it sits in a frame from another origin
	Hidden       bool   // it is not drawn on the page
}

// Classify sorts one step: verb on s, with the model's claim of how risky it
// is. The claim can only raise the result: a safe step claimed sensitive or
// critical, or carrying a claim these rules do not know, becomes a handoff.
func Classify(verb string, s Subject, claim string) (Class, Reason) {
	c, r := byRules(verb, s)
	if c == Safe && claim != ClaimNormal {
		return Handoff, ReasonClaim
	}
	return c, r
}

func byRules(verb string, s Subject) (Class, Reason) {
	switch verb {
	case VerbScroll, VerbWait:
		return Safe, ReasonSafe
	case VerbClick, VerbFill, VerbSelect:
	default:
		return Handoff, ReasonUnknownVerb
	}
	role := strings.ToLower(strings.TrimSpace(s.Role))
	kind := strings.ToLower(strings.TrimSpace(s.InputType))
	switch {
	case s.Hidden || kind == "hidden":
		return Refused, ReasonHidden
	case s.OtherOrigin:
		return Refused, ReasonOtherOrigin
	case kind == "file":
		return Refused, ReasonFileInput
	case private(s.Name, kind, s.Autocomplete):
		return Refused, ReasonPrivate
	}
	if !readable(s.Name) {
		return Handoff, ReasonUnreadable
	}
	switch verb {
	case VerbFill:
		// A field's label says what goes in it, not what typing does: typing
		// into a field called "Message" sends nothing, and the send stays the
		// person's. Ordinary fields of a payment form (a delivery address)
		// may be typed into; its card fields were refused above.
		if !typable[role] || !typableInput[kind] {
			return Handoff, ReasonUnknownRole
		}
		return Safe, ReasonSafe
	case VerbSelect:
		if !choosable[role] {
			return Handoff, ReasonUnknownRole
		}
		if risky(s.Name) {
			return Handoff, ReasonRiskyLabel
		}
		if !readable(s.Option) {
			return Handoff, ReasonUnreadable
		}
		if risky(s.Option) {
			return Handoff, ReasonRiskyLabel
		}
	default: // click
		if !clickable[role] {
			return Handoff, ReasonUnknownRole
		}
		if risky(s.Name) {
			return Handoff, ReasonRiskyLabel
		}
		if s.Submits || pressesForm[kind] {
			return Handoff, ReasonSubmits
		}
	}
	if s.PaymentForm {
		return Handoff, ReasonPaymentForm
	}
	return Safe, ReasonSafe
}

// Roles a click may land on by itself.
var clickable = set("button", "link", "tab", "menuitem", "menuitemcheckbox", "menuitemradio", "checkbox",
	"radio", "switch", "option", "treeitem", "textbox", "searchbox", "combobox", "spinbutton", "listitem",
	"row", "cell", "gridcell", "columnheader", "rowheader", "heading", "img", "image", "disclosuretriangle")

// Roles that take typed text, and the input types an ordinary text field has.
var (
	typable      = set("textbox", "searchbox", "combobox", "spinbutton")
	typableInput = set("", "text", "search", "email", "url", "tel", "number", "date", "time", "datetime-local", "month", "week")
	choosable    = set("combobox", "listbox", "menulistpopup", "radiogroup")
	pressesForm  = set("submit", "image", "reset")
)

func set(words ...string) map[string]bool {
	m := make(map[string]bool, len(words))
	for _, w := range words {
		m[w] = true
	}
	return m
}

// private says whether a field holds something nobody but the person types:
// its label names one (the same words the read-only asks refuse), its type
// hides what is typed, or its autocomplete hint names a secret or a card.
func private(name, kind, autocomplete string) bool {
	if kind == "password" {
		return true
	}
	if _, refused := screenask.Sensitive(name, nil, feature.Box{}); refused {
		return true
	}
	for _, hint := range strings.Fields(strings.ToLower(autocomplete)) {
		if strings.HasPrefix(hint, "cc-") || strings.HasSuffix(hint, "password") || hint == "one-time-code" {
			return true
		}
	}
	return false
}

// readable says whether a label is one these rules can judge: it has at least
// one word, and every letter in it is plain Latin. A label with no words (an
// icon, a price, an arrow) or in a script the word lists do not cover cannot
// be judged, so the step is the person's.
func readable(label string) bool {
	word := false
	for _, r := range label {
		if !unicode.IsLetter(r) {
			continue
		}
		if r > unicode.MaxASCII {
			return false
		}
		word = true
	}
	return word
}

// risky says whether a label names a step that cannot be taken back.
func risky(label string) bool {
	words := strings.FieldsFunc(strings.ToLower(label), func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsDigit(r)
	})
	for _, w := range words {
		if riskyWords[w] {
			return true
		}
		for _, stem := range riskyStems {
			if strings.Contains(w, stem) {
				return true
			}
		}
	}
	joined := " " + strings.Join(words, " ") + " "
	for _, p := range riskyPhrases {
		if strings.Contains(joined, " "+p+" ") {
			return true
		}
	}
	return false
}

// riskyWords are whole words naming a step whose effect leaves the page or
// cannot be taken back: sending and publishing, paying and ordering,
// deleting, leaving an account, agreeing to something. A handful of the same
// verbs in other languages written in plain Latin letters sit at the end;
// labels in other scripts are already unreadable.
var riskyWords = set(
	// sending and publishing
	"send", "submit", "post", "publish", "share", "reply", "respond", "comment", "repost", "forward",
	"invite", "apply", "upload", "download", "install", "like", "follow", "vote",
	// paying and ordering
	"pay", "buy", "purchase", "checkout", "order", "donate", "tip", "subscribe", "upgrade", "renew",
	"transfer", "withdraw", "deposit", "bid", "book", "reserve", "rent", "charge", "billing",
	// deleting and undoing
	"delete", "remove", "erase", "discard", "clear", "trash", "destroy", "wipe", "reset", "revoke",
	"cancel", "uninstall", "deactivate", "disable", "terminate", "leave", "block", "report",
	"unfollow", "unfriend", "archive",
	// leaving an account
	"logout", "signout", "logoff", "quit", "exit", "unsubscribe", "unregister",
	// agreeing and creating
	"confirm", "accept", "agree", "allow", "authorize", "authorise", "approve", "grant", "consent",
	"register", "signup", "join", "enroll", "enrol", "verify",
	// the same verbs in other languages, plain Latin letters only
	"envoyer", "enviar", "senden", "absenden", "inviare", "invia", "verzenden", "versturen", "verstuur",
	"pagar", "payer", "bezahlen", "zahlen", "betalen", "pagare", "kaufen", "comprar", "acheter", "comprare",
	"kopen", "bestellen", "commander", "eliminar", "borrar", "supprimer", "effacer", "loeschen", "entfernen",
	"verwijderen", "elimina", "cancella", "publicar", "publier", "pubblica", "abmelden", "deconnexion",
	"abbestellen", "confirmar", "confirmer", "bestaetigen", "conferma", "aceptar", "accepter", "akzeptieren",
	"valider", "enregistrer",
)

// riskyStems mark a risky word inside a longer one ("deleteall", "payments").
var riskyStems = []string{"delete", "remove", "purchase", "payment", "checkout", "logout", "signout", "subscribe", "submit"}

// riskyPhrases are word sequences whose words are harmless alone.
var riskyPhrases = []string{
	"sign out", "log out", "log off", "sign off", "check out", "place order", "opt out", "opt in",
	"sign up", "create account", "close account", "add card", "save card", "buy now", "go live", "make live",
}

// AppSubject is what one step outside a browser tab acts on, as observed.
// Every string is window or screen text, or the model's argument: it is only
// matched against, never obeyed.
type AppSubject struct {
	App   string // app id of the window acted in; on the nested desktop, the focused one
	Title string // that window's title
	Near  string // click or scroll: the words the screen shows at the point
	Text  string // type: the text to type
	Keys  string // press: the chord
	Name  string // open_app: the launcher entry's name
	// Private is the executor's own finding that the target is a private
	// field or surface (the OCR words beside the focus name one).
	Private bool
}

// ClassifyApp sorts one step taken in a live window or on the nested
// desktop, with the model's claim. As with Classify, the claim only raises,
// and a step these rules cannot place is a handoff.
func ClassifyApp(verb string, s AppSubject, claim string) (Class, Reason) {
	c, r := appRules(verb, s)
	if c == Safe && claim != ClaimNormal {
		return Handoff, ReasonClaim
	}
	return c, r
}

func appRules(verb string, s AppSubject) (Class, Reason) {
	switch verb {
	case VerbClick, VerbScroll, VerbWait, VerbType, VerbPress, VerbOpenApp, VerbFindFiles, VerbShowFile:
	default:
		return Handoff, ReasonUnknownVerb
	}
	switch {
	case apprules.Self(s.App):
		return Refused, ReasonSelf
	case apprules.PrivateApp(s.App):
		return Refused, ReasonPrivateApp
	case s.Private || private(s.Title, "", ""):
		return Refused, ReasonPrivate
	}
	switch verb {
	case VerbWait, VerbFindFiles, VerbShowFile:
		// a wait presses nothing; a search reads names in the chosen
		// folders; showing a file opens only the folder it sits in
		return Safe, ReasonSafe
	case VerbScroll:
		if private(s.Near, "", "") {
			return Refused, ReasonPrivate
		}
		return Safe, ReasonSafe
	case VerbType:
		return byVerdict(apprules.Text(s.Text))
	case VerbPress:
		_, v := apprules.PressChord(s.Keys)
		return byVerdict(v)
	case VerbOpenApp:
		switch {
		case apprules.Self(s.Name):
			return Refused, ReasonSelf
		case apprules.PrivateApp(s.Name):
			return Refused, ReasonPrivateApp
		case !readable(s.Name):
			return Handoff, ReasonUnreadable
		case risky(s.Name):
			return Handoff, ReasonRiskyLabel
		}
		return Safe, ReasonSafe
	}
	// click: on what the screen reads at the point
	switch {
	case private(s.Near, "", ""):
		return Refused, ReasonPrivate
	case !readable(s.Near):
		return Handoff, ReasonUnreadable
	case risky(s.Near):
		return Handoff, ReasonRiskyLabel
	}
	return Safe, ReasonSafe
}

// byVerdict turns an apprules verdict into a class and a reason.
func byVerdict(v apprules.Verdict) (Class, Reason) {
	switch v.Kind {
	case apprules.KindSafe:
		return Safe, ReasonSafe
	case apprules.KindRefused:
		switch v.Why {
		case apprules.WhySecret:
			return Refused, ReasonSecret
		case apprules.WhyControl, apprules.WhyEmpty:
			return Refused, ReasonControlText
		}
		return Refused, ReasonRefusedKey
	}
	if v.Why == apprules.WhySubmit {
		return Handoff, ReasonSubmits
	}
	return Handoff, ReasonRiskyKey
}
