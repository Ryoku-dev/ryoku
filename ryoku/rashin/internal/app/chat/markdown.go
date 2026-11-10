package chat

import (
	"strconv"
	"strings"
	"unicode"
)

type markdownKind uint8

const (
	markdownParagraph markdownKind = iota + 1
	markdownHeading
	markdownList
	markdownQuote
	markdownCode
	markdownTable
)

type markdownBlock struct {
	kind     markdownKind
	level    int
	text     string
	language string
	items    []markdownListItem
	head     []string
	rows     [][]string
}

type markdownListItem struct {
	depth   int
	ordered bool
	number  int
	text    string
}

type inlineKind uint8

const (
	inlinePlain inlineKind = iota
	inlineEmphasis
	inlineStrong
	inlineCode
	inlineLink
)

type inlineSpan struct {
	kind inlineKind
	text string
	url  string
}

func parseMarkdown(source string) []markdownBlock {
	lines := strings.Split(strings.ReplaceAll(source, "\r\n", "\n"), "\n")
	blocks := make([]markdownBlock, 0, max(1, len(lines)/2))
	for index := 0; index < len(lines); {
		line := lines[index]
		if strings.TrimSpace(line) == "" {
			index++
			continue
		}
		trimmed := strings.TrimLeftFunc(line, unicode.IsSpace)
		if strings.HasPrefix(trimmed, "```") {
			language := strings.TrimSpace(strings.TrimPrefix(trimmed, "```"))
			index++
			start := index
			for index < len(lines) && !strings.HasPrefix(strings.TrimLeftFunc(lines[index], unicode.IsSpace), "```") {
				index++
			}
			blocks = append(blocks, markdownBlock{kind: markdownCode, language: language, text: strings.Join(lines[start:index], "\n")})
			if index < len(lines) {
				index++
			}
			continue
		}
		if level, title := markdownHeadingLine(trimmed); level > 0 {
			blocks = append(blocks, markdownBlock{kind: markdownHeading, level: level, text: title})
			index++
			continue
		}
		if index+1 < len(lines) && isTableSeparator(lines[index+1]) && strings.Contains(line, "|") {
			header := tableCells(line)
			index += 2
			rows := make([][]string, 0)
			for index < len(lines) && strings.Contains(lines[index], "|") && strings.TrimSpace(lines[index]) != "" {
				rows = append(rows, tableCells(lines[index]))
				index++
			}
			blocks = append(blocks, markdownBlock{kind: markdownTable, head: header, rows: rows})
			continue
		}
		if _, _, _, _, ok := markdownListLine(line); ok {
			items := make([]markdownListItem, 0)
			for index < len(lines) {
				depth, ordered, number, text, matched := markdownListLine(lines[index])
				if !matched {
					break
				}
				items = append(items, markdownListItem{depth: depth, ordered: ordered, number: number, text: text})
				index++
			}
			blocks = append(blocks, markdownBlock{kind: markdownList, items: items})
			continue
		}
		if strings.HasPrefix(trimmed, ">") {
			quoted := make([]string, 0)
			for index < len(lines) {
				part := strings.TrimLeftFunc(lines[index], unicode.IsSpace)
				if !strings.HasPrefix(part, ">") {
					break
				}
				quoted = append(quoted, strings.TrimSpace(strings.TrimPrefix(part, ">")))
				index++
			}
			blocks = append(blocks, markdownBlock{kind: markdownQuote, text: strings.Join(quoted, "\n")})
			continue
		}

		paragraph := []string{strings.TrimSpace(line)}
		index++
		for index < len(lines) && strings.TrimSpace(lines[index]) != "" && !startsMarkdownBlock(lines, index) {
			paragraph = append(paragraph, strings.TrimSpace(lines[index]))
			index++
		}
		blocks = append(blocks, markdownBlock{kind: markdownParagraph, text: strings.Join(paragraph, " ")})
	}
	return blocks
}

func startsMarkdownBlock(lines []string, index int) bool {
	line := strings.TrimLeftFunc(lines[index], unicode.IsSpace)
	if strings.HasPrefix(line, "```") || strings.HasPrefix(line, ">") {
		return true
	}
	if level, _ := markdownHeadingLine(line); level > 0 {
		return true
	}
	if _, _, _, _, ok := markdownListLine(lines[index]); ok {
		return true
	}
	return index+1 < len(lines) && strings.Contains(line, "|") && isTableSeparator(lines[index+1])
}

func markdownHeadingLine(line string) (int, string) {
	level := 0
	for level < len(line) && level < 6 && line[level] == '#' {
		level++
	}
	if level == 0 || level >= len(line) || line[level] != ' ' {
		return 0, ""
	}
	return level, strings.TrimSpace(line[level:])
}

func markdownListLine(line string) (depth int, ordered bool, number int, text string, ok bool) {
	indent := 0
	for indent < len(line) && (line[indent] == ' ' || line[indent] == '\t') {
		if line[indent] == '\t' {
			depth += 2
		} else {
			depth++
		}
		indent++
	}
	depth /= 2
	rest := line[indent:]
	if len(rest) >= 2 && (rest[0] == '-' || rest[0] == '*' || rest[0] == '+') && rest[1] == ' ' {
		return depth, false, 0, strings.TrimSpace(rest[2:]), true
	}
	digits := 0
	for digits < len(rest) && rest[digits] >= '0' && rest[digits] <= '9' {
		digits++
	}
	if digits == 0 || digits+1 >= len(rest) || rest[digits] != '.' || rest[digits+1] != ' ' {
		return 0, false, 0, "", false
	}
	number, _ = strconv.Atoi(rest[:digits])
	return depth, true, number, strings.TrimSpace(rest[digits+2:]), true
}

func isTableSeparator(line string) bool {
	cells := tableCells(line)
	if len(cells) == 0 {
		return false
	}
	for _, cell := range cells {
		cell = strings.TrimSpace(cell)
		cell = strings.TrimPrefix(cell, ":")
		cell = strings.TrimSuffix(cell, ":")
		if len(cell) < 3 || strings.Trim(cell, "-") != "" {
			return false
		}
	}
	return true
}

func tableCells(line string) []string {
	line = strings.TrimSpace(line)
	line = strings.TrimPrefix(line, "|")
	line = strings.TrimSuffix(line, "|")
	parts := strings.Split(line, "|")
	cells := make([]string, len(parts))
	for index, part := range parts {
		cells[index] = strings.TrimSpace(part)
	}
	return cells
}

func parseInline(source string) []inlineSpan {
	spans := make([]inlineSpan, 0, 4)
	for len(source) > 0 {
		switch {
		case source[0] == '`':
			if end := strings.IndexByte(source[1:], '`'); end >= 0 {
				spans = append(spans, inlineSpan{kind: inlineCode, text: source[1 : end+1]})
				source = source[end+2:]
				continue
			}
		case strings.HasPrefix(source, "**"):
			if end := strings.Index(source[2:], "**"); end >= 0 {
				spans = append(spans, inlineSpan{kind: inlineStrong, text: source[2 : end+2]})
				source = source[end+4:]
				continue
			}
		case source[0] == '*' || source[0] == '_':
			marker := source[0]
			if end := strings.IndexByte(source[1:], marker); end >= 0 {
				spans = append(spans, inlineSpan{kind: inlineEmphasis, text: source[1 : end+1]})
				source = source[end+2:]
				continue
			}
		case source[0] == '[':
			closeLabel := strings.Index(source, "](")
			if closeLabel > 0 {
				closeURL := strings.IndexByte(source[closeLabel+2:], ')')
				if closeURL >= 0 {
					urlEnd := closeLabel + 2 + closeURL
					spans = append(spans, inlineSpan{kind: inlineLink, text: source[1:closeLabel], url: source[closeLabel+2 : urlEnd]})
					source = source[urlEnd+1:]
					continue
				}
			}
		}
		next := nextInlineMarker(source)
		if next == 0 {
			next = 1
		}
		spans = append(spans, inlineSpan{kind: inlinePlain, text: source[:next]})
		source = source[next:]
	}
	return spans
}

func nextInlineMarker(source string) int {
	next := len(source)
	for _, marker := range []byte{'`', '*', '_', '['} {
		if index := strings.IndexByte(source, marker); index >= 0 && index < next {
			next = index
		}
	}
	return next
}
