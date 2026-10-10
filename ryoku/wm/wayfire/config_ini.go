package main

import "strings"

// The wayfire ini codec. wayfire has no include directive, so apply cannot
// patch a fragment the way niri writes an overlay it includes: it rewrites one
// whole file from whole layers. These types carry a config as sections and keys
// in file order, which is what lets a merge keep the shipped defaults' layout
// and replace only the values later layers set, last layer winning.
//
// Comments are deliberately not modelled. The composed file is machine-owned
// and regenerated, so an inline comment would go stale on the next change; the
// comment home for a hand edit is user.ini, a layer apply reads but never
// writes.

type iniDoc struct {
	sections []iniSection
}

type iniSection struct {
	name string
	keys []iniKey
}

type iniKey struct {
	name  string
	value string
}

// wfStripComment is one line through wf-config's comment pass: truncate at the
// first '#' that is not preceded by a backslash, then unescape every "\#" into
// "#". Both halves matter here — wayfire reads a colour as "\#rrggbbaa" and a
// real comment as plain "# ...", and a seed's inline comment must not leak into
// a value the composer emits again.
func wfStripComment(line string) string {
	cut := len(line)
	for i := 0; i < len(line); i++ {
		if line[i] == '#' && (i == 0 || line[i-1] != '\\') {
			cut = i
			break
		}
	}
	line = line[:cut]
	out := make([]byte, 0, len(line))
	hadEscape := false
	for i := 0; i < len(line); i++ {
		c := line[i]
		if c == '#' && hadEscape && len(out) > 0 {
			out = out[:len(out)-1]
		}
		out = append(out, c)
		hadEscape = c == '\\'
	}
	return string(out)
}

// parseIni reads a wayfire config into ordered sections. Lines before the first
// section, comments and anything without a key and value are skipped. A repeated
// section joins the first one, and a repeated key keeps the last value in the
// first key's slot, which is the pair wayfire itself would read. A value ending
// in a single backslash continues onto the following physical lines (the way
// the shipped plugin list is written); those lines are glued on with single
// spaces, so the composed file carries the same value on one line. A doubled
// trailing backslash is a literal, the way wf-config's join reads it.
func parseIni(b []byte) iniDoc {
	var d iniDoc
	cur := -1
	lines := strings.Split(string(b), "\n")
	for i := 0; i < len(lines); i++ {
		line := strings.TrimSpace(wfStripComment(lines[i]))
		if line == "" || strings.HasPrefix(line, ";") {
			continue
		}
		if strings.HasPrefix(line, "[") {
			if end := strings.Index(line, "]"); end > 0 {
				name := strings.TrimSpace(line[1:end])
				cur = -1
				for j := range d.sections {
					if d.sections[j].name == name {
						cur = j
						break
					}
				}
				if cur < 0 {
					d.sections = append(d.sections, iniSection{name: name})
					cur = len(d.sections) - 1
				}
			}
			continue
		}
		k, v, ok := strings.Cut(line, "=")
		if !ok || cur < 0 {
			continue
		}
		value := strings.TrimSpace(v)
		for strings.HasSuffix(value, `\`) {
			if strings.HasSuffix(value, `\\`) {
				value = value[:len(value)-1]
				break
			}
			if i+1 >= len(lines) {
				value = strings.TrimSpace(value[:len(value)-1])
				break
			}
			i++
			next := strings.TrimSpace(wfStripComment(lines[i]))
			value = strings.TrimSpace(value[:len(value)-1]) + " " + next
		}
		d.setKey(cur, strings.TrimSpace(k), value)
	}
	return d
}

// set records a value under section/key, creating either when absent. An
// existing key keeps its position, so a merged file lists keys in the order the
// first layer introduced them.
func (d *iniDoc) set(section, key, value string) {
	for i := range d.sections {
		if d.sections[i].name == section {
			d.setKey(i, key, value)
			return
		}
	}
	d.sections = append(d.sections, iniSection{name: section})
	d.setKey(len(d.sections)-1, key, value)
}

func (d *iniDoc) setKey(i int, key, value string) {
	sec := &d.sections[i]
	for j := range sec.keys {
		if sec.keys[j].name == key {
			sec.keys[j].value = value
			return
		}
	}
	sec.keys = append(sec.keys, iniKey{name: key, value: value})
}

// get reads back a merged value, so a post-merge adjustment (the plugin list,
// which the store toggles enforce rather than layer) sees the composed result.
func (d iniDoc) get(section, key string) (string, bool) {
	for _, sec := range d.sections {
		if sec.name != section {
			continue
		}
		for _, k := range sec.keys {
			if k.name == key {
				return k.value, true
			}
		}
	}
	return "", false
}

// overlay lays another doc over the receiver, later values winning per key and
// first-seen positions kept, the same rule as mergeIni but mutating in place
// for a builder composing sub-documents.
func (d *iniDoc) overlay(o iniDoc) {
	for _, sec := range o.sections {
		for _, k := range sec.keys {
			d.set(sec.name, k.name, k.value)
		}
	}
}

// mergeIni lays the docs over each other, later docs winning per key. Section
// and key order comes from first sight, so the shipped defaults set the shape of
// the generated file and the store and seeds only override what they name.
func mergeIni(docs ...iniDoc) iniDoc {
	var out iniDoc
	for _, d := range docs {
		for _, sec := range d.sections {
			for _, k := range sec.keys {
				out.set(sec.name, k.name, k.value)
			}
		}
	}
	return out
}

// generatedHeader is the first line of every file apply writes, so the live
// config says it is machine-owned and where a hand edit goes instead.
const generatedHeader = "# Generated by ryoku apply; edit user.ini, monitors.ini\n" +
	"# or keyboard.ini instead of this file.\n"

// iniEscape is one emitted line through the escape rules wf-config's own
// serializer applies: every '#' becomes "\#" (unescaped it would start a
// comment and truncate the value), and a line ending in a backslash gains a
// second one (a lone trailing backslash would glue the next line into this
// value). The generated header lines are comments by design and never pass
// through here.
func iniEscape(line string) string {
	line = strings.ReplaceAll(line, "#", `\#`)
	if strings.HasSuffix(line, `\`) {
		line += `\`
	}
	return line
}

// renderIni writes the doc back as one ini body. Empty sections are dropped
// (they carried nothing but comments, which are not modelled), and every other
// line is derived from data, so two runs over the same layers are byte-equal.
func renderIni(d iniDoc) []byte {
	var b strings.Builder
	b.WriteString(generatedHeader)
	for _, sec := range d.sections {
		if len(sec.keys) == 0 {
			continue
		}
		b.WriteString("\n")
		b.WriteString(iniEscape("[" + sec.name + "]"))
		b.WriteString("\n")
		for _, k := range sec.keys {
			b.WriteString(iniEscape(k.name + " = " + k.value))
			b.WriteString("\n")
		}
	}
	return []byte(b.String())
}
