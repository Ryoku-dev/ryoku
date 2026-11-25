// Package apprules holds the fixed rules for Hands outside a browser tab:
// in one live window the person picked, and on the separate nested desktop
// Hyakki owns. It names which keys Hands may press itself, how typed text is
// judged and cut into chunks, where a click may land, which windows are
// never a target, and how a found file is shown.
//
// It is a leaf of plain functions: it starts no process, reads no file and
// asks no compositor. The caller hands it what it observed (a window, a
// file's mode and first bytes) and gets a verdict back. Package consent turns
// a verdict into the step's class, and the executor refuses anything these
// rules refuse before any input is sent.
//
// The rules are deliberately small. There is no drag and no paste. Hands
// presses a short list of moving and undo keys itself; Enter, and anything
// that closes, deletes or reaches the desktop, is the person's press. Text
// holding a line break is the person's to send, because Hands cannot tell a
// new line from a submit. A program or a script found by a search is never
// run: Hands shows the folder it sits in, and only the folder.
package apprules

import (
	"strings"

	"ryoku-rashin/internal/sensitive"
)

// Kind is what a rule allows. The zero value is KindHandoff, so a step these
// rules did not judge is never run by Hands.
type Kind uint8

const (
	// KindHandoff: the person does it themselves.
	KindHandoff Kind = iota
	// KindSafe: Hands may do it after announcing it.
	KindSafe
	// KindRefused: neither Hands nor a pointer card offers it.
	KindRefused
)

func (k Kind) String() string {
	switch k {
	case KindSafe:
		return "safe"
	case KindRefused:
		return "refused"
	default:
		return "handoff"
	}
}

// Verdict is one rule's answer: Kind, and Why, a short code naming the rule
// (never the text or key that was judged), fit for the activity log.
type Verdict struct {
	Kind Kind
	Why  string
}

// Rule codes a Verdict may carry.
const (
	WhySafe      = "safe"
	WhySubmit    = "submit"     // Enter, or text with a line break
	WhyRiskyKey  = "risky-key"  // a key outside the safe set
	WhyPaste     = "paste"      // there is no clipboard path
	WhySystemKey = "system-key" // a key that reaches the desktop, not the window
	WhyControl   = "control"    // text with a character that would be pressed as a key
	WhySecret    = "secret"     // text shaped like a secret
	WhyEmpty     = "empty"      // nothing to type
)

// SelfApp is Hyakki's own window. Hands never acts on it.
const SelfApp = "dev.ryoku.Hyakki"

// Self says whether app is one of Hyakki's own surfaces.
func Self(app string) bool {
	app = strings.TrimSpace(app)
	return strings.EqualFold(app, SelfApp) || len(app) > len(SelfApp) && strings.EqualFold(app[:len(SelfApp)+1], SelfApp+".")
}

// privateApps name, anywhere inside an app id, a surface that holds or asks
// for a secret: a password manager (sensitive.Place covers those), a
// polkit or askpass prompt, a lock screen.
var privateApps = []string{
	"polkit", "policykit", "askpass", "gcr-prompter", "systemprompter",
	"swaylock", "hyprlock", "gtklock", "waylock", "i3lock", "screenlocker", "lockscreen", "lock-screen",
}

// PrivateApp says whether app is a private surface: a password manager, a
// polkit or askpass prompt, or a lock screen. Hands never acts in one, and
// never opens one.
func PrivateApp(app string) bool {
	app = strings.TrimSpace(app)
	if app == "" {
		return false
	}
	if sensitive.Place(app) {
		return true
	}
	low := strings.ToLower(app)
	for _, name := range privateApps {
		if strings.Contains(low, name) {
			return true
		}
	}
	return false
}
