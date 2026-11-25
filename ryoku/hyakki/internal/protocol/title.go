package protocol

import (
	"strings"
	"unicode"
)

// TitleRunes is the most runes a title made from a brief holds.
const TitleRunes = 80

// BriefTitle is the title a team task takes from its brief when it is given
// none: the brief's first line, clipped to TitleRunes by ClipWords. The
// daemon and the app both name a task with it, so the title the app shows
// while it creates one is the one the daemon keeps.
func BriefTitle(brief string) string {
	line, _, _ := strings.Cut(strings.TrimSpace(brief), "\n")
	return ClipWords(strings.TrimSpace(line), TitleRunes)
}

// ClipWords cuts text to at most limit runes. Text that is longer ends at
// the last word boundary that leaves room for a closing "…", which counts
// toward the limit; a first word longer than that is cut inside itself.
func ClipWords(text string, limit int) string {
	runes := []rune(text)
	if len(runes) <= limit {
		return text
	}
	if limit < 1 {
		return ""
	}
	keep := limit - 1
	for cut := keep; cut > 0; cut-- {
		if unicode.IsSpace(runes[cut]) {
			if head := strings.TrimRightFunc(string(runes[:cut]), unicode.IsSpace); head != "" {
				return head + "…"
			}
			break
		}
	}
	return strings.TrimRightFunc(string(runes[:keep]), unicode.IsSpace) + "…"
}
