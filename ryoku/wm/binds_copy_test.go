package wm

import (
	"regexp"
	"testing"
	"unicode"
)

// Keyboard shortcuts names a bind the way the rest of Settings words the same
// thing: "full screen" in two words and no bracketed asides, on top of the
// rules every provider row follows.
var bindOnlyCopy = regexp.MustCompile(`(?i)\bfullscreen\b|\(`)

func checkBindCopy(t *testing.T, where, s string) {
	t.Helper()
	if s == "" {
		return
	}
	if !unicode.IsUpper([]rune(s)[0]) {
		t.Errorf("%s: %q does not start with a capital", where, s)
	}
	for _, re := range []*regexp.Regexp{rawCopy, usSpelling, bindOnlyCopy} {
		if m := re.FindString(s); m != "" {
			t.Errorf("%s: %q shows %q", where, s, m)
		}
	}
}

func TestBindCatalogueCopyIsPlain(t *testing.T) {
	for _, b := range ShippedBinds() {
		checkBindCopy(t, b.ID+" category", b.Category)
		checkBindCopy(t, b.ID+" label", b.Label)
		checkBindCopy(t, b.ID+" hint", b.Hint)
	}
	for _, action := range []string{"exec", "close", "fullscreen", "togglefloating", "workspace",
		"movetoworkspace", "movetoworkspacesilent"} {
		checkBindCopy(t, "custom "+action, CustomBindLabel(action, "2"))
	}
	if got := CustomBindLabel("exec", " "); got != "Run a command" {
		t.Errorf("an empty command row reads %q", got)
	}
}
