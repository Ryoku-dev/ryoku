package tts

import (
	"net/url"
	"regexp"
	"strings"
	"unicode"
	"unicode/utf8"
)

// Normalize turns a reply into text a voice can say: fenced code is left
// out, inline code keeps its words, links keep their text, a bare address
// becomes its host, LaTeX becomes words, and markdown markup, HTML tags and
// pictographs go. Headings, list items and table rows each end a sentence.
// English first: the words for math follow the reply's language rule.
func Normalize(text string) string {
	text = strings.ReplaceAll(text, "\r\n", "\n")
	text = dropFences(text)
	text = speakMathSpans(text)
	text = currency.ReplaceAllString(text, "$1 dollars")
	var out []string
	var para []string
	flush := func() {
		if len(para) > 0 {
			out = append(out, terminate(strings.Join(para, " ")))
			para = nil
		}
	}
	for _, line := range strings.Split(text, "\n") {
		line = quote.ReplaceAllString(line, "")
		trimmed := strings.TrimSpace(line)
		switch {
		case trimmed == "" || rule.MatchString(trimmed):
			flush()
		case tableRule.MatchString(trimmed):
		case strings.HasPrefix(trimmed, "|"):
			flush()
			var cells []string
			for c := range strings.SplitSeq(strings.Trim(trimmed, "|"), "|") {
				if c = inline(c); c != "" {
					cells = append(cells, c)
				}
			}
			if len(cells) > 0 {
				out = append(out, terminate(strings.Join(cells, ", ")))
			}
		case heading.MatchString(trimmed) || bullet.MatchString(line):
			flush()
			item := heading.ReplaceAllString(trimmed, "")
			if bullet.MatchString(line) {
				item = task.ReplaceAllString(bullet.ReplaceAllString(line, ""), "")
			}
			if item = inline(item); item != "" {
				out = append(out, terminate(item))
			}
		default:
			if l := inline(line); l != "" {
				para = append(para, l)
			}
		}
	}
	flush()
	return tidy(strings.Join(out, " "))
}

var (
	quote     = regexp.MustCompile(`^\s*(>\s?)+`)
	rule      = regexp.MustCompile(`^([-*_]\s*){3,}$`)
	tableRule = regexp.MustCompile(`^\|?\s*:?-{3,}:?\s*(\|\s*:?-{3,}:?\s*)*\|?$`)
	heading   = regexp.MustCompile(`^#{1,6}\s+`)
	bullet    = regexp.MustCompile(`^\s*([-*+•]|\d{1,3}[.)])\s+`)
	task      = regexp.MustCompile(`^\[[ xX]\]\s+`)
	currency  = regexp.MustCompile(`\$(\d[\d,]*(?:\.\d+)?)`)

	image     = regexp.MustCompile(`!\[([^\]]*)\]\([^)]*\)`)
	link      = regexp.MustCompile(`\[([^\]]+)\]\([^)]*\)`)
	refLink   = regexp.MustCompile(`\[([^\]]+)\]\[[^\]]*\]`)
	footnote  = regexp.MustCompile(`\[\^[^\]]+\]`)
	autolink  = regexp.MustCompile(`<((?:https?|ftp)://[^>\s]+)>`)
	bareURL   = regexp.MustCompile(`\b(?:https?|ftp)://[^\s<>()\[\]]+|\bwww\.[^\s<>()\[\]]+`)
	code      = regexp.MustCompile("`+([^`]+?)`+")
	tag       = regexp.MustCompile(`</?[A-Za-z][^<>]*>`)
	strong    = regexp.MustCompile(`\*\*|__|~~`)
	emphasis  = regexp.MustCompile(`(^|[\s(\[])[*_]([^*_\s](?:[^*_]*[^*_\s])?)[*_]`)
	arrow     = regexp.MustCompile(`\s*(->|=>|→|⇒)\s*`)
	spaces    = regexp.MustCompile(`\s+`)
	spaceStop = regexp.MustCompile(`\s+([.,!?;:])`)
	stops     = regexp.MustCompile(`([.!?])[.!?]+`)
	commaStop = regexp.MustCompile(`,\s*([.!?])`)
)

// dropFences removes fenced code blocks (``` or ~~~), an unclosed fence
// running to the end, and leaves a paragraph break in their place.
func dropFences(text string) string {
	var out []string
	fence := ""
	for _, line := range strings.Split(text, "\n") {
		t := strings.TrimSpace(line)
		switch {
		case fence != "":
			if strings.HasPrefix(t, fence) && strings.Trim(t, fence[:1]) == "" {
				fence = ""
				out = append(out, "")
			}
		case strings.HasPrefix(t, "```") || strings.HasPrefix(t, "~~~"):
			fence = t[:3]
			for strings.HasPrefix(t[len(fence):], fence[:1]) {
				fence += fence[:1]
			}
			out = append(out, "")
		default:
			out = append(out, line)
		}
	}
	return strings.Join(out, "\n")
}

// inline speaks one line's inline markup.
func inline(s string) string {
	s = image.ReplaceAllString(s, "$1")
	s = link.ReplaceAllString(s, "$1")
	s = refLink.ReplaceAllString(s, "$1")
	s = footnote.ReplaceAllString(s, "")
	s = autolink.ReplaceAllStringFunc(s, func(m string) string { return host(m[1 : len(m)-1]) })
	s = code.ReplaceAllStringFunc(s, func(m string) string { return codeWords(strings.Trim(m, "`")) })
	s = bareURL.ReplaceAllStringFunc(s, host)
	s = tag.ReplaceAllString(s, "")
	s = strong.ReplaceAllString(s, "")
	s = emphasis.ReplaceAllString(s, "$1$2")
	s = arrow.ReplaceAllString(s, " to ")
	s = strings.NewReplacer("&", " and ", "#", "", "*", "", "_", " ", "\\", "", "≈", " about ", "×", " times ").Replace(s)
	s = strings.Map(func(r rune) rune {
		switch {
		case unicode.Is(unicode.So, r), unicode.Is(unicode.Sk, r), unicode.Is(unicode.Co, r),
			unicode.Is(unicode.Variation_Selector, r), r == '\u200d', unicode.IsControl(r):
			return -1
		}
		return r
	}, s)
	return strings.TrimSpace(spaces.ReplaceAllString(s, " "))
}

// host is the site an address points at, without www.
func host(raw string) string {
	trail := ""
	for strings.ContainsAny(raw[len(raw)-1:], ".,;:!?") {
		trail = raw[len(raw)-1:] + trail
		raw = raw[:len(raw)-1]
		if raw == "" {
			return trail
		}
	}
	if strings.HasPrefix(raw, "www.") {
		raw = "http://" + raw
	}
	u, err := url.Parse(raw)
	if err != nil || u.Hostname() == "" {
		return trail
	}
	return strings.TrimPrefix(u.Hostname(), "www.") + trail
}

// codeWords says inline code by its words: separators become spaces and
// call parentheses go. Code longer than a few words is left out.
func codeWords(s string) string {
	s = strings.TrimSuffix(strings.TrimSpace(s), "()")
	s = strings.NewReplacer("::", " ", "->", " ", "_", " ", "/", " ", ".", " dot ", "()", "", "=", " equals ").Replace(s)
	s = strings.TrimSpace(spaces.ReplaceAllString(s, " "))
	if utf8.RuneCountInString(s) > maxCodeRunes {
		return ""
	}
	return s
}

const maxCodeRunes = 40

// terminate ends a block with a full stop unless it already ends a
// sentence.
func terminate(s string) string {
	s = strings.TrimRight(s, " ,;:")
	if s == "" || strings.ContainsAny(s[len(s)-1:], ".!?") {
		return s
	}
	return s + "."
}

func tidy(s string) string {
	s = spaces.ReplaceAllString(s, " ")
	s = spaceStop.ReplaceAllString(s, "$1")
	s = commaStop.ReplaceAllString(s, "$1")
	s = stops.ReplaceAllString(s, "$1")
	return strings.TrimSpace(s)
}

// Sentences splits normalized text into the sentences a Speaker renders
// one at a time. A stop splits when the next word starts with a capital,
// a digit or a quote; an initial or a short title before it does not. A
// sentence longer than maxSentenceRunes splits at its last clause break,
// else its last space, before the cap.
func Sentences(text string) []string {
	var out []string
	rs := []rune(strings.TrimSpace(text))
	start := 0
	for i := 0; i < len(rs); i++ {
		if !strings.ContainsRune(".!?…", rs[i]) {
			continue
		}
		j := i + 1
		for j < len(rs) && strings.ContainsRune(".!?…\"')]’”", rs[j]) {
			j++
		}
		if j < len(rs) && !unicode.IsSpace(rs[j]) {
			continue
		}
		k := j
		for k < len(rs) && unicode.IsSpace(rs[k]) {
			k++
		}
		if k < len(rs) && !(unicode.IsUpper(rs[k]) || unicode.IsDigit(rs[k]) || strings.ContainsRune("\"'“‘(", rs[k])) {
			continue
		}
		if rs[i] == '.' && abbreviation(rs[start:i]) {
			continue
		}
		out = appendSentence(out, string(rs[start:j]))
		start, i = k, k-1
	}
	if start < len(rs) {
		out = appendSentence(out, string(rs[start:]))
	}
	return out
}

const maxSentenceRunes = 300

func appendSentence(out []string, s string) []string {
	for {
		s = strings.TrimSpace(s)
		rs := []rune(s)
		if len(rs) <= maxSentenceRunes {
			if strings.IndexFunc(s, func(r rune) bool { return unicode.IsLetter(r) || unicode.IsDigit(r) }) >= 0 {
				out = append(out, s)
			}
			return out
		}
		head := string(rs[:maxSentenceRunes])
		cut := strings.LastIndexAny(head, ",;:")
		if cut < len(head)/3 {
			cut = strings.LastIndex(head, " ")
		}
		if cut <= 0 {
			cut = len(head) - 1
		}
		out = append(out, strings.TrimSpace(head[:cut+1]))
		s = head[cut+1:] + string(rs[maxSentenceRunes:])
	}
}

// abbreviation reports whether the text before a full stop ends in an
// initial or a short title, so the stop does not end the sentence.
func abbreviation(before []rune) bool {
	w := string(before)
	if i := strings.LastIndexFunc(w, unicode.IsSpace); i >= 0 {
		w = w[i+1:]
	}
	w = strings.ToLower(strings.TrimLeft(w, "(\"'"))
	if utf8.RuneCountInString(w) == 1 && unicode.IsLetter([]rune(w)[0]) {
		return true
	}
	switch w {
	case "mr", "mrs", "ms", "dr", "st", "vs", "e.g", "i.e", "etc", "no", "fig", "approx":
		return true
	}
	return false
}
