package media

import (
	"fmt"
	"html"
	"math"
	"regexp"
	"slices"
	"strconv"
	"strings"
)

var captionTags = regexp.MustCompile(`<[^>]*>`)

// VTTText turns subtitle cues into timestamped text without rolling repetitions.
func VTTText(vtt string) string {
	lines := strings.Split(strings.ReplaceAll(strings.TrimPrefix(vtt, "\ufeff"), "\r\n", "\n"), "\n")
	var out strings.Builder
	var previous []string
	for i := 0; i < len(lines); i++ {
		timing := strings.SplitN(lines[i], " --> ", 2)
		if len(timing) != 2 {
			continue
		}
		seconds, ok := cueSeconds(strings.TrimSpace(timing[0]))
		if !ok {
			continue
		}
		var cue strings.Builder
		for i++; i < len(lines) && strings.TrimSpace(lines[i]) != ""; i++ {
			cue.WriteByte(' ')
			cue.WriteString(lines[i])
		}
		words := strings.Fields(html.UnescapeString(captionTags.ReplaceAllString(cue.String(), "")))
		if len(words) == 0 {
			continue
		}
		overlap := len(previous)
		if overlap > len(words) {
			overlap = len(words)
		}
		for overlap > 0 {
			if slices.Equal(previous[len(previous)-overlap:], words[:overlap]) {
				break
			}
			overlap--
		}
		if overlap < len(words) {
			fmt.Fprintf(&out, "[%s] %s\n", timestamp(seconds), strings.Join(words[overlap:], " "))
		}
		previous = words
	}
	return strings.TrimSpace(out.String())
}

func cueSeconds(s string) (int, bool) {
	parts := strings.Split(s, ":")
	if len(parts) != 2 && len(parts) != 3 {
		return 0, false
	}
	total := 0.0
	for i, part := range parts {
		n, err := strconv.ParseFloat(part, 64)
		if err != nil || math.IsNaN(n) || math.IsInf(n, 0) || n < 0 || (i > 0 && n >= 60) {
			return 0, false
		}
		total = total*60 + n
	}
	return int(total), true
}

func timestamp(seconds int) string {
	return fmt.Sprintf("%02d:%02d", seconds/60, seconds%60)
}

// LimitTranscript caps model input at 60,000 characters and says when it was cut.
func LimitTranscript(text string) string {
	const limit = 60000
	if len(text) <= limit {
		return text
	}
	count := 0
	for offset := range text {
		if count == limit {
			return text[:offset] + "\n[Transcript shortened after 60,000 characters.]"
		}
		count++
	}
	return text
}
