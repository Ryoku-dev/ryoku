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
	markdownRule
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
		if isMarkdownRule(trimmed) {
			blocks = append(blocks, markdownBlock{kind: markdownRule})
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
				if matched {
					items = append(items, markdownListItem{depth: depth, ordered: ordered, number: number, text: text})
					index++
					continue
				}
				// an indented line that starts nothing of its own carries on the item above, as a wrapped item is written
				if !listContinuation(lines, index) {
					break
				}
				last := &items[len(items)-1]
				last.text += " " + strings.TrimSpace(lines[index])
				index++
			}
			blocks = append(blocks, markdownBlock{kind: markdownList, items: items})
			continue
		}
		if strings.HasPrefix(trimmed, ">") {
			// lines run on as one paragraph, a bare > starts the next
			var quoted strings.Builder
			gap := ""
			for index < len(lines) {
				part := strings.TrimLeftFunc(lines[index], unicode.IsSpace)
				if !strings.HasPrefix(part, ">") {
					break
				}
				index++
				part = strings.TrimSpace(strings.TrimPrefix(part, ">"))
				if part == "" {
					if quoted.Len() > 0 {
						gap = "\n\n"
					}
					continue
				}
				quoted.WriteString(gap)
				quoted.WriteString(part)
				gap = " "
			}
			blocks = append(blocks, markdownBlock{kind: markdownQuote, text: quoted.String()})
			continue
		}

		var paragraph strings.Builder
		for {
			text, breaks := paragraphLine(lines[index])
			paragraph.WriteString(text)
			index++
			if index >= len(lines) || strings.TrimSpace(lines[index]) == "" || startsMarkdownBlock(lines, index) {
				break
			}
			if breaks {
				paragraph.WriteByte('\n')
			} else {
				paragraph.WriteByte(' ')
			}
		}
		blocks = append(blocks, markdownBlock{kind: markdownParagraph, text: paragraph.String()})
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
	if isMarkdownRule(line) {
		return true
	}
	if _, _, _, _, ok := markdownListLine(lines[index]); ok {
		return true
	}
	return index+1 < len(lines) && strings.Contains(line, "|") && isTableSeparator(lines[index+1])
}

func listContinuation(lines []string, index int) bool {
	line := lines[index]
	if strings.TrimSpace(line) == "" || (line[0] != ' ' && line[0] != '\t') {
		return false
	}
	return !startsMarkdownBlock(lines, index)
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

// a line of three or more of the same mark and nothing else
func isMarkdownRule(line string) bool {
	marks := strings.ReplaceAll(strings.TrimSpace(line), " ", "")
	if len(marks) < 3 || strings.IndexByte("-*_", marks[0]) < 0 {
		return false
	}
	return strings.Trim(marks, marks[:1]) == ""
}

// two spaces or a backslash at the end of a line ask for the break to be kept
func paragraphLine(line string) (text string, breaks bool) {
	text = strings.TrimSpace(line)
	if strings.HasSuffix(text, "\\") {
		return strings.TrimSpace(strings.TrimSuffix(text, "\\")), true
	}
	return text, strings.HasSuffix(line, "  ")
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
