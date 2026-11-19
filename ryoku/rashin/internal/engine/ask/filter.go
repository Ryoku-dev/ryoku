package ask

import (
	"strings"

	"ryoku-rashin/internal/memory/render"
)

// Filter suppresses Ask-only private markers and fences as the model streams.
type Filter interface {
	Text(string)
	Thought(string)
	Close()
}

type answerFilter struct {
	needs   needsFilter
	visible fenceScrubber
	thought fenceScrubber
	emit    func(string)
	think   func(string)
}

// Filter holds back NEEDS_AGENT and recalled-memory fences as model deltas arrive.
func (m *Mode) Filter(text, thought func(string)) Filter {
	f := &answerFilter{visible: fenceScrubber{lineStart: true}, thought: fenceScrubber{lineStart: true}, emit: text, think: thought}
	f.needs.emit = text
	return f
}

func (f *answerFilter) Text(delta string) {
	f.needs.Write(f.visible.Feed(delta))
}

func (f *answerFilter) Thought(delta string) {
	if value := f.thought.Feed(delta); value != "" {
		f.think(value)
	}
}

func (f *answerFilter) Close() {
	f.needs.Write(f.visible.Flush())
	if value := f.thought.Flush(); value != "" {
		f.think(value)
	}
	f.needs.Close()
}

type needsFilter struct {
	pending  string
	suppress bool
	emit     func(string)
}

func (f *needsFilter) Write(delta string) {
	if f.suppress || delta == "" {
		return
	}
	f.pending += delta
	if index := completeNeedsMarker(f.pending); index >= 0 {
		prefix := strings.TrimRight(f.pending[:index], " \t\n")
		if prefix != "" {
			f.emit(prefix)
		}
		f.pending = ""
		f.suppress = true
		return
	}
	keep := partialNeedsSuffix(f.pending)
	if keep == len(f.pending) {
		return
	}
	f.emit(f.pending[:len(f.pending)-keep])
	f.pending = f.pending[len(f.pending)-keep:]
}

func (f *needsFilter) Close() {
	if !f.suppress && f.pending != "" {
		f.emit(f.pending)
	}
	f.pending = ""
}

func completeNeedsMarker(text string) int {
	from := 0
	for {
		i := strings.Index(text[from:], "NEEDS_AGENT:")
		if i < 0 {
			return -1
		}
		i += from
		line := strings.LastIndex(text[:i], "\n") + 1
		if strings.TrimSpace(text[line:i]) == "" {
			return line
		}
		from = i + 1
	}
}

func partialNeedsSuffix(text string) int {
	const marker = "NEEDS_AGENT:"
	line := strings.LastIndex(text, "\n") + 1
	candidate := text[line:]
	trimmed := strings.TrimLeft(candidate, " \t")
	if trimmed != "" && strings.HasPrefix(marker, trimmed) {
		keep := len(candidate)
		if line > 0 {
			keep++
		}
		return keep
	}
	return 0
}

type fenceScrubber struct {
	inSpan    bool
	held      string
	lineStart bool
}

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

func (s *fenceScrubber) Flush() string {
	tail := s.held
	if s.inSpan {
		tail = ""
	}
	s.held, s.inSpan = "", false
	return tail
}

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

func partialSuffix(buf, tag string) int {
	folded := foldASCII(buf)
	for n := min(len(folded), len(tag)-1); n > 0; n-- {
		if strings.HasPrefix(tag, folded[len(folded)-n:]) {
			return n
		}
	}
	return 0
}

func foldASCII(s string) string {
	b := []byte(s)
	for i, c := range b {
		if 'A' <= c && c <= 'Z' {
			b[i] = c + 'a' - 'A'
		}
	}
	return string(b)
}
