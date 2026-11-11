package protocol

import (
	"strings"
	"testing"
	"unicode/utf8"
)

func TestBriefTitleCutsAtAWordAndSaysSo(t *testing.T) {
	exact := strings.Repeat("abcd ", 15) + "abcde" // 80 runes
	long := "Pick one of these options and write it to choice.txt: alpha, beta or gamma, then report"
	for _, c := range []struct {
		name, brief, want string
	}{
		{"empty", "", ""},
		{"blank lines only", "\n\n  \n", ""},
		{"short first line", "  Rotate the logs.\nKeep a week of them.", "Rotate the logs."},
		{"exactly the limit", exact + "\nmore", exact},
		{"cut at the last word", long, "Pick one of these options and write it to choice.txt: alpha, beta or gamma,…"},
		{"one long word", strings.Repeat("x", 100), strings.Repeat("x", 79) + "…"},
		{"word ending on the limit", strings.Repeat("y", 75) + " " + "abcd" + " tail", strings.Repeat("y", 75) + "…"},
		{"runes, not bytes", strings.Repeat("ü", 81), strings.Repeat("ü", 79) + "…"},
	} {
		got := BriefTitle(c.brief)
		if got != c.want {
			t.Errorf("%s: BriefTitle = %q, want %q", c.name, got, c.want)
		}
		if n := utf8.RuneCountInString(got); n > TitleRunes {
			t.Errorf("%s: the title holds %d runes, more than %d", c.name, n, TitleRunes)
		}
	}
}
