package memory

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// A memory file holds one fact per bullet, `- <text> ^<id>`, under two
// headings: the facts in use and the ones a newer fact replaced. Everything
// else in the file is the user's and is kept as written.
const (
	headingFacts    = "## Facts"
	headingReplaced = "## Replaced"
	fileIntro       = "Rashin keeps this file. Edit a line to change a memory, delete it to forget it."
)

// Sections of a memory file.
const (
	sectionNone = iota
	sectionFacts
	sectionReplaced
)

var (
	factLine = regexp.MustCompile(`^\s*[-*+]\s+(.*?)\s*$`)
	factID   = regexp.MustCompile(`\s*\^(m[0-9a-f]{7})$`)
)

// mdLine is one line of a memory file; a fact line keeps its id and text
// apart from the raw line.
type mdLine struct {
	raw     string
	fact    bool
	id      string
	text    string
	section int
}

// mdFile is a parsed memory file.
type mdFile struct {
	lines []mdLine
}

// parseMarkdown reads a memory file. Bullets under Facts and Replaced are
// facts, with or without an id; every other line is kept as it is.
func parseMarkdown(data string) *mdFile {
	f := &mdFile{}
	section := sectionNone
	data = strings.ReplaceAll(data, "\r\n", "\n")
	data = strings.TrimSuffix(data, "\n")
	if data == "" {
		return f
	}
	for _, raw := range strings.Split(data, "\n") {
		trimmed := strings.TrimSpace(raw)
		if strings.HasPrefix(trimmed, "#") {
			switch {
			case strings.EqualFold(trimmed, headingFacts):
				section = sectionFacts
			case strings.EqualFold(trimmed, headingReplaced):
				section = sectionReplaced
			default:
				section = sectionNone
			}
			f.lines = append(f.lines, mdLine{raw: raw, section: section})
			continue
		}
		line := mdLine{raw: raw, section: section}
		if match := factLine.FindStringSubmatch(raw); match != nil && section != sectionNone {
			text := match[1]
			if id := factID.FindStringSubmatch(text); id != nil {
				line.id = id[1]
				text = strings.TrimSpace(text[:len(text)-len(id[0])])
			}
			if text != "" {
				line.fact, line.text = true, text
			}
		}
		f.lines = append(f.lines, line)
	}
	return f
}

// newMarkdown is a fresh memory file with its title and the two headings.
func newMarkdown(title string) *mdFile {
	return parseMarkdown("# " + title + "\n\n" + fileIntro + "\n\n" + headingFacts + "\n\n" + headingReplaced + "\n")
}

// String renders the file; fact lines are written from their id and text.
func (f *mdFile) String() string {
	var b strings.Builder
	for _, line := range f.lines {
		if line.fact {
			b.WriteString("- " + line.text)
			if line.id != "" {
				b.WriteString(" ^" + line.id)
			}
		} else {
			b.WriteString(line.raw)
		}
		b.WriteByte('\n')
	}
	return b.String()
}

// facts lists the fact lines in file order.
func (f *mdFile) facts() []mdLine {
	var out []mdLine
	for _, line := range f.lines {
		if line.fact {
			out = append(out, line)
		}
	}
	return out
}

// keep drops every fact line whose id is not in want and every repeat of an
// id; lines without an id go too, unless loose keeps them.
func (f *mdFile) keep(want map[string]bool, loose bool) {
	seen := map[string]bool{}
	out := f.lines[:0]
	for _, line := range f.lines {
		if line.fact && line.id == "" && loose {
			out = append(out, line)
			continue
		}
		if line.fact && (!want[line.id] || seen[line.id]) {
			continue
		}
		if line.fact {
			seen[line.id] = true
		}
		out = append(out, line)
	}
	f.lines = out
}

// set writes a fact line: in place when it is already in the right section,
// otherwise moved to the end of its section.
func (f *mdFile) set(id, text string, section int) {
	for index, line := range f.lines {
		if !line.fact || line.id != id {
			continue
		}
		if line.section == section {
			f.lines[index].text = text
			return
		}
		f.lines = append(f.lines[:index], f.lines[index+1:]...)
		break
	}
	at := f.sectionEnd(section)
	line := mdLine{fact: true, id: id, text: text, section: section}
	f.lines = append(f.lines[:at], append([]mdLine{line}, f.lines[at:]...)...)
}

// sectionEnd is where a new fact of the section goes: after its last fact,
// or after its heading and a blank line. A missing heading is added at the
// end of the file.
func (f *mdFile) sectionEnd(section int) int {
	heading := headingFacts
	if section == sectionReplaced {
		heading = headingReplaced
	}
	start := -1
	for index, line := range f.lines {
		if !line.fact && strings.EqualFold(strings.TrimSpace(line.raw), heading) {
			start = index
			break
		}
	}
	if start < 0 {
		if n := len(f.lines); n > 0 && (f.lines[n-1].fact || strings.TrimSpace(f.lines[n-1].raw) != "") {
			f.lines = append(f.lines, mdLine{section: f.lines[n-1].section})
		}
		f.lines = append(f.lines, mdLine{raw: heading, section: section}, mdLine{section: section})
		return len(f.lines)
	}
	end := start + 1
	for index := start + 1; index < len(f.lines); index++ {
		if isHeading(f.lines[index]) {
			break
		}
		if f.lines[index].fact {
			end = index + 1
		}
	}
	if end > start+1 {
		return end
	}
	// an empty section: keep a blank line after the heading and before the next one
	if end < len(f.lines) && isBlank(f.lines[end]) {
		end++
	} else {
		f.insert(end, mdLine{section: section})
		end++
	}
	if end < len(f.lines) && isHeading(f.lines[end]) {
		f.insert(end, mdLine{section: section})
	}
	return end
}

// insert puts line at index.
func (f *mdFile) insert(index int, line mdLine) {
	f.lines = append(f.lines[:index], append([]mdLine{line}, f.lines[index:]...)...)
}

// isHeading reports a Markdown heading line.
func isHeading(line mdLine) bool {
	return !line.fact && strings.HasPrefix(strings.TrimSpace(line.raw), "#")
}

// isBlank reports an empty non-fact line.
func isBlank(line mdLine) bool {
	return !line.fact && strings.TrimSpace(line.raw) == ""
}

// writeFileAtomic replaces path with data through a temporary file in the
// same folder, owner-only, so a reader never sees half a file.
func writeFileAtomic(path string, data []byte) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(dir, "."+filepath.Base(path)+".*")
	if err != nil {
		return err
	}
	name := tmp.Name()
	defer os.Remove(name)
	if err := tmp.Chmod(0o600); err != nil {
		_ = tmp.Close()
		return err
	}
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(name, path)
}
