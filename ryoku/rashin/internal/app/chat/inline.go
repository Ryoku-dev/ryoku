package chat

import (
	"strings"
	"unicode"
	"unicode/utf8"
)

type inlineStyle uint8

const (
	inlineStrong inlineStyle = 1 << iota
	inlineEmphasis
	inlineCode
)

// url is set on a link
type inlineSpan struct {
	text  string
	style inlineStyle
	url   string
}

// a span while emphasis is being paired, mark is set on a run of * or _ that may still open or close some
type inlineNode struct {
	inlineSpan
	mark    byte
	count   int
	written int
	opens   bool
	closes  bool
}

func parseInline(source string) []inlineSpan {
	nodes := inlineNodes(source)
	pairMarks(nodes)
	return tidySpans(joinNodes(nodes))
}

func inlineNodes(source string) []inlineNode {
	var nodes []inlineNode
	var plain strings.Builder
	flush := func() {
		if plain.Len() > 0 {
			nodes = append(nodes, inlineNode{inlineSpan: inlineSpan{text: plain.String()}})
			plain.Reset()
		}
	}
	link := func(label, url string) {
		flush()
		inner := parseInline(label)
		if len(inner) == 0 {
			inner = []inlineSpan{{text: url}}
		}
		for _, span := range inner {
			span.url = url
			nodes = append(nodes, inlineNode{inlineSpan: span})
		}
	}
	for at := 0; at < len(source); {
		switch character := source[at]; {
		case character == '\\' && at+1 < len(source) && isASCIIPunctuation(source[at+1]):
			plain.WriteByte(source[at+1])
			at += 2
			continue
		case character == '`':
			if code, next, ok := codeSpan(source, at); ok {
				flush()
				nodes = append(nodes, inlineNode{inlineSpan: inlineSpan{text: code, style: inlineCode}})
				at = next
				continue
			}
			run := runLength(source, at)
			plain.WriteString(source[at : at+run])
			at += run
			continue
		case character == '*' || character == '_':
			flush()
			run := runLength(source, at)
			nodes = append(nodes, markNode(source, at, run))
			at += run
			continue
		case character == '[':
			if label, url, next, ok := linkAt(source, at); ok {
				link(label, url)
				at = next
				continue
			}
		case character == '!' && at+1 < len(source) && source[at+1] == '[':
			if label, url, next, ok := linkAt(source, at+1); ok {
				link(label, url)
				at = next
				continue
			}
		case character == '<':
			if url, next, ok := angleLink(source, at); ok {
				link(url, url)
				at = next
				continue
			}
			if next, ok := breakTag(source, at); ok {
				plain.WriteByte('\n')
				at = next
				continue
			}
		case character == 'h' && (at == 0 || !isWordByte(source[at-1])):
			if url := bareLink(source[at:]); url != "" {
				flush()
				nodes = append(nodes, inlineNode{inlineSpan: inlineSpan{text: url, url: url}})
				at += len(url)
				continue
			}
		}
		_, size := utf8.DecodeRuneInString(source[at:])
		plain.WriteString(source[at : at+size])
		at += size
	}
	flush()
	return nodes
}

func runLength(source string, at int) int {
	end := at
	for end < len(source) && source[end] == source[at] {
		end++
	}
	return end - at
}

// a code span closes on the next run of as many backticks as opened it
func codeSpan(source string, at int) (code string, next int, ok bool) {
	ticks := runLength(source, at)
	for search := at + ticks; search < len(source); {
		found := strings.IndexByte(source[search:], '`')
		if found < 0 {
			return "", 0, false
		}
		found += search
		run := runLength(source, found)
		if run != ticks {
			search = found + run
			continue
		}
		code = strings.ReplaceAll(source[at+ticks:found], "\n", " ")
		if len(code) > 1 && code[0] == ' ' && code[len(code)-1] == ' ' && strings.TrimSpace(code) != "" {
			code = code[1 : len(code)-1]
		}
		return code, found + run, true
	}
	return "", 0, false
}

// * marks emphasis wherever it touches a word, text without spaces has no other way to say it
// _ only does at a word's edge, or every snake_case name and XDG_VARIABLE loses its underscores
func markNode(source string, at, run int) inlineNode {
	before, beforeSize := utf8.DecodeLastRuneInString(source[:at])
	after, afterSize := utf8.DecodeRuneInString(source[at+run:])
	beforeSpace := beforeSize == 0 || unicode.IsSpace(before)
	afterSpace := afterSize == 0 || unicode.IsSpace(after)
	node := inlineNode{mark: source[at], count: run, written: run, opens: !afterSpace, closes: !beforeSpace}
	if node.mark == '_' {
		beforeMark := beforeSpace || unicode.IsPunct(before) || unicode.IsSymbol(before)
		afterMark := afterSpace || unicode.IsPunct(after) || unicode.IsSymbol(after)
		node.opens, node.closes = node.opens && beforeMark, node.closes && afterMark
	}
	return node
}

// what is left of a run after pairing stays as written
func pairMarks(nodes []inlineNode) {
	for closer := range nodes {
		if nodes[closer].mark == 0 || !nodes[closer].closes {
			continue
		}
		for nodes[closer].count > 0 {
			opener := openerFor(nodes, closer)
			if opener < 0 {
				break
			}
			used, style := 1, inlineEmphasis
			if nodes[opener].count >= 2 && nodes[closer].count >= 2 {
				used, style = 2, inlineStrong
			}
			for between := opener + 1; between < closer; between++ {
				nodes[between].style |= style
				nodes[between].opens, nodes[between].closes = false, false
			}
			nodes[opener].count -= used
			nodes[closer].count -= used
		}
	}
}

func openerFor(nodes []inlineNode, closer int) int {
	for opener := closer - 1; opener >= 0; opener-- {
		candidate := nodes[opener]
		if candidate.mark != nodes[closer].mark || !candidate.opens || candidate.count == 0 {
			continue
		}
		// a run that could do either pairs only when the two lengths do not add up to a multiple of three
		either := candidate.closes || nodes[closer].opens
		if either && (candidate.written+nodes[closer].written)%3 == 0 && (candidate.written%3 != 0 || nodes[closer].written%3 != 0) {
			continue
		}
		return opener
	}
	return -1
}

func joinNodes(nodes []inlineNode) []inlineSpan {
	spans := make([]inlineSpan, 0, len(nodes))
	for _, node := range nodes {
		span := node.inlineSpan
		if node.mark != 0 {
			span.text = strings.Repeat(string(node.mark), node.count)
		}
		if span.text == "" {
			continue
		}
		if last := len(spans) - 1; last >= 0 && spans[last].style == span.style && spans[last].url == span.url {
			spans[last].text += span.text
			continue
		}
		spans = append(spans, span)
	}
	return spans
}

// blanks fold to one space as on a page, with none at either end or beside a line break
func tidySpans(spans []inlineSpan) []inlineSpan {
	tidy := spans[:0]
	blank := true
	for _, span := range spans {
		var text strings.Builder
		for _, character := range span.text {
			switch {
			case character == '\n':
				trimmed := strings.TrimRight(text.String(), " ")
				text.Reset()
				text.WriteString(trimmed)
				text.WriteByte('\n')
				blank = true
			case character == ' ' || character == '\t':
				if !blank || span.style&inlineCode != 0 {
					text.WriteByte(' ')
				}
				blank = true
			default:
				text.WriteRune(character)
				blank = false
			}
		}
		if span.text = text.String(); span.text != "" {
			tidy = append(tidy, span)
		}
	}
	for len(tidy) > 0 {
		last := &tidy[len(tidy)-1]
		last.text = strings.TrimRight(last.text, " \n")
		if last.text != "" {
			break
		}
		tidy = tidy[:len(tidy)-1]
	}
	return tidy
}

func linkAt(source string, at int) (label, url string, next int, ok bool) {
	closing, depth := -1, 0
scan:
	for index := at; index < len(source); index++ {
		switch source[index] {
		case '\\':
			index++
		case '`':
			if _, after, ok := codeSpan(source, index); ok {
				index = after - 1
			}
		case '[':
			depth++
		case ']':
			if depth--; depth == 0 {
				closing = index
				break scan
			}
		}
	}
	if closing < 0 || closing+1 >= len(source) || source[closing+1] != '(' {
		return "", "", 0, false
	}
	start := closing + 2
	for start < len(source) && source[start] == ' ' {
		start++
	}
	end, depth := start, 0
	for ; end < len(source); end++ {
		character := source[end]
		if character == ' ' || character == '\n' || (character == ')' && depth == 0) {
			break
		}
		switch character {
		case '(':
			depth++
		case ')':
			depth--
		}
	}
	url = strings.Trim(source[start:end], "<>")
	// a title after the address is not shown
	finish := strings.IndexByte(source[end:], ')')
	if url == "" || finish < 0 || strings.ContainsRune(source[end:end+finish], '\n') {
		return "", "", 0, false
	}
	return source[at+1 : closing], url, end + finish + 1, true
}

func angleLink(source string, at int) (url string, next int, ok bool) {
	end := strings.IndexByte(source[at:], '>')
	if end < 0 {
		return "", 0, false
	}
	url = source[at+1 : at+end]
	if !startsAddress(url) || strings.ContainsAny(url, " \n") {
		return "", 0, false
	}
	return url, at + end + 1, true
}

func breakTag(source string, at int) (next int, ok bool) {
	for _, tag := range []string{"<br>", "<br/>", "<br />"} {
		if len(source)-at >= len(tag) && strings.EqualFold(source[at:at+len(tag)], tag) {
			return at + len(tag), true
		}
	}
	return 0, false
}

func startsAddress(value string) bool {
	for _, scheme := range []string{"https://", "http://", "mailto:"} {
		if len(value) > len(scheme) && strings.HasPrefix(value, scheme) {
			return true
		}
	}
	return false
}

// without the full stop or bracket that only ends the sentence around the address
func bareLink(source string) string {
	if !strings.HasPrefix(source, "https://") && !strings.HasPrefix(source, "http://") {
		return ""
	}
	end := strings.IndexAny(source, " \t\n<>`")
	if end < 0 {
		end = len(source)
	}
	url := source[:end]
	for len(url) > 0 {
		last := url[len(url)-1]
		if strings.IndexByte("?!.,:;*_~'\"", last) >= 0 || (last == ')' && strings.Count(url, ")") > strings.Count(url, "(")) {
			url = url[:len(url)-1]
			continue
		}
		break
	}
	if !startsAddress(url) {
		return ""
	}
	return url
}

func isASCIIPunctuation(character byte) bool {
	return character < utf8.RuneSelf && (unicode.IsPunct(rune(character)) || unicode.IsSymbol(rune(character)))
}

func isWordByte(character byte) bool {
	return character >= utf8.RuneSelf || unicode.IsLetter(rune(character)) || unicode.IsDigit(rune(character))
}
