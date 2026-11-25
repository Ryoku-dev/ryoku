package discipline

import (
	"strings"
	"unicode"

	"ryoku-rashin/internal/oni/window"
)

// Self says whether w is one of Hyakki's own windows: its app id is one of
// self, compared without case. An empty entry in self names nothing.
func Self(w window.Window, self []string) bool {
	if w.App == "" {
		return false
	}
	for _, id := range self {
		if id != "" && strings.EqualFold(w.App, id) {
			return true
		}
	}
	return false
}

// Private says whether w is a window Hands never acts on or alongside: a
// password manager or secret store, a polkit agent or other credential
// prompt, or a lock screen. Its app id is matched with case, spaces and
// punctuation removed, so "org.keepassxc.KeePassXC" and "Proton Pass" both
// match; its title is matched for the words a credential prompt shows.
// Both are only matched against, never obeyed.
func Private(w window.Window) bool {
	app := fold(w.App)
	for _, word := range privateApps {
		if strings.Contains(app, word) {
			return true
		}
	}
	title := fold(w.Title)
	for _, word := range privateTitles {
		if strings.Contains(title, word) {
			return true
		}
	}
	return false
}

// privateApps are words that mark an app id as private wherever they appear
// inside it, written as fold leaves them.
var privateApps = []string{
	// password managers, secret stores and one-time code apps
	"password", "passphrase", "keepass", "bitwarden", "vaultwarden", "lastpass", "dashlane",
	"enpass", "nordpass", "protonpass", "qtpass", "gopass", "kwallet", "seahorse", "keyring",
	"secrets", "authenticator",
	// polkit agents and other credential prompts
	"polkit", "policykit", "authenticationagent", "pinentry", "askpass", "gcrprompter",
	"soteria",
	// lock screens
	"hyprlock", "swaylock", "gtklock", "waylock", "i3lock", "securelock", "screenlocker",
	"lockscreen", "sessionlock",
}

// privateTitles are words a credential prompt or a secret store shows in its
// title, written as fold leaves them.
var privateTitles = []string{
	"password", "passphrase", "passcode", "authenticat", "keyring", "unlock",
}

// fold lowers s and keeps only its letters and digits.
func fold(s string) string {
	var b strings.Builder
	for _, r := range s {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			b.WriteRune(unicode.ToLower(r))
		}
	}
	return b.String()
}
