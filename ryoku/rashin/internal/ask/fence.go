package ask

import (
	"strings"

	"ryoku-rashin/internal/memory/render"
)

// Portions adapted from NousResearch/hermes-agent agent/memory_context.py
// @ fe6649291e60 (MIT, Copyright (c) 2025 Nous Research): the streaming
// scrubber for memory-context spans split across deltas. Modified by Ryoku:
// ported to Go over bytes with ASCII case folding, using Rashin's fence tags.

// fenceScrubber drops recalled-memory fences, and what lies between their
// tags, from an answer as it streams. A fence opens only at the start of a
// line followed by a line break, so prose that names the tag stays; a tag
// split across deltas is held back until the next one settles it, and a
// fence the stream never closes is dropped whole.
type fenceScrubber struct {
	inSpan bool
	held   string
	// lineStart says what was shown so far ends at the start of a line
	lineStart bool
}

func newFenceScrubber() *fenceScrubber { return &fenceScrubber{lineStart: true} }

// Feed returns the part of text that may be shown now.
func (s *fenceScrubber) Feed(text string) string {
	if text == "" {
		return ""
	}
	buf := s.held + text
	s.held = ""
	var out strings.Builder
	for buf != "" {
		var at, held int
		var tag string
		if s.inSpan {
			tag = render.FenceClose
			at = strings.Index(foldASCII(buf), tag)
			held = partialSuffix(buf, tag)
		} else {
			tag = render.FenceOpen
			at = s.openAt(buf)
			// a whole tag at the end waits for the character that confirms it
			if strings.HasSuffix(foldASCII(buf), tag) && s.endsAtLineStart(buf[:len(buf)-len(tag)]) {
				held = len(tag)
			} else {
				held = partialSuffix(buf, tag)
			}
		}
		if at == -1 {
			if !s.inSpan {
				s.show(&out, buf[:len(buf)-held])
			}
			s.held = buf[len(buf)-held:]
			break
		}
		if !s.inSpan {
			s.show(&out, buf[:at])
		}
		buf = buf[at+len(tag):]
		s.inSpan = !s.inSpan
	}
	return out.String()
}

// Flush returns what was held back once the stream ends; inside a fence it
// is dropped, since a cut answer is better than leaked memory.
func (s *fenceScrubber) Flush() string {
	tail := s.held
	if s.inSpan {
		tail = ""
	}
	s.held, s.inSpan = "", false
	return tail
}

// openAt is where an opening tag starts a fence: at a line start with a line
// break right after it, or -1.
func (s *fenceScrubber) openAt(buf string) int {
	folded := foldASCII(buf)
	from := 0
	for {
		at := strings.Index(folded[from:], render.FenceOpen)
		if at == -1 {
			return -1
		}
		at += from
		after := at + len(render.FenceOpen)
		if s.endsAtLineStart(buf[:at]) && after < len(buf) && (buf[after] == '\n' || buf[after] == '\r') {
			return at
		}
		from = at + 1
	}
}

// endsAtLineStart reports whether showing text leaves the stream at the start
// of a line: only blanks after its last line break, or after what was shown
// before when it has none.
func (s *fenceScrubber) endsAtLineStart(text string) bool {
	cut := strings.LastIndexByte(text, '\n')
	return strings.TrimSpace(text[cut+1:]) == "" && (cut >= 0 || s.lineStart)
}

func (s *fenceScrubber) show(out *strings.Builder, text string) {
	if text == "" {
		return
	}
	out.WriteString(text)
	s.lineStart = s.endsAtLineStart(text)
}

// partialSuffix is the length of the longest end of buf that begins tag.
func partialSuffix(buf, tag string) int {
	folded := foldASCII(buf)
	for n := min(len(folded), len(tag)-1); n > 0; n-- {
		if strings.HasPrefix(tag, folded[len(folded)-n:]) {
			return n
		}
	}
	return 0
}

// foldASCII lowercases ASCII letters only, so byte offsets stay those of s.
func foldASCII(s string) string {
	b := []byte(s)
	for i, c := range b {
		if 'A' <= c && c <= 'Z' {
			b[i] = c + 'a' - 'A'
		}
	}
	return string(b)
}
