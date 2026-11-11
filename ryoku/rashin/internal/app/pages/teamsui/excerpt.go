package teamsui

import (
	"regexp"
	"strings"
)

var (
	markdownLink  = regexp.MustCompile(`!?\[([^\]]*)\]\([^)]*\)`)
	markdownMarks = regexp.MustCompile("[*_`]+")
)

// the first lines of a Markdown report as plain sentences, for a tablet or a
// card that only previews it
func Excerpt(markdown string, lines int) string {
	var kept []string
	fenced := false
	for _, line := range strings.Split(markdown, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "```") {
			fenced = !fenced
			continue
		}
		if fenced || trimmed == "" || strings.HasPrefix(trimmed, "|") || strings.Trim(trimmed, "-=*_ ") == "" {
			continue
		}
		trimmed = strings.TrimLeft(trimmed, "#>-*+ ")
		trimmed = markdownLink.ReplaceAllString(trimmed, "$1")
		trimmed = markdownMarks.ReplaceAllString(trimmed, "")
		if trimmed = strings.TrimSpace(trimmed); trimmed != "" {
			kept = append(kept, trimmed)
		}
		if len(kept) == lines {
			break
		}
	}
	return strings.Join(kept, " ")
}
