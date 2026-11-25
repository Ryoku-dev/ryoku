package textcalls

// Derived from oh-my-pi (MIT), packages/ai/src/dialect/qwen3.ts@c7fff67
// Derived from oh-my-pi (MIT), packages/ai/src/dialect/glm.ts@c7fff67
// Derived from oh-my-pi (MIT), packages/ai/src/dialect/owned-stream.ts@c7fff67

import (
	"html"
	"strings"

	"ryoku-rashin/internal/llm"
)

const (
	tagToolCall    = "<tool_call>"
	tagToolCallEnd = "</tool_call>"
	tagFunction    = "<function="
	tagFunctionEnd = "</function>"
	tagParamEq     = "<parameter="
	tagParamSp     = "<parameter "
	tagParamEnd    = "</parameter>"
	tagPyStart     = "<|tool_call_start|>"
	tagPyEnd       = "<|tool_call_end|>"
	tagThink       = "<think>"
	tagThinkEnd    = "</think>"
	tagResponse    = "<tool_response>"
	tagResponseEnd = "</tool_response>"
	tagPyResponse  = "<|tool_response_start|>"
	tagPyRespEnd   = "<|tool_response_end|>"
	tagArgKey      = "<arg_key>"
	tagArgKeyEnd   = "</arg_key>"
	tagArgValue    = "<arg_value>"
	tagArgValueEnd = "</arg_value>"
)

const (
	spaces = " \t\r\n"
	// maxName bounds a function name; longer text after <function= is
	// prose, not a call.
	maxName = 128
	// maxHeader bounds a <parameter ...> header.
	maxHeader = 160
	// healSpace is how much whitespace may sit between a forgotten closer
	// and the tag that proves it was one (omp glm.ts HEAL_WS_MAX).
	healSpace = 32
	// scanBack is how far a value search steps back on new text: longer
	// than any heal signature still waiting for its end.
	scanBack = 256
	// An unclosed Auto candidate must not buffer an entire answer.
	maxAutoHeld = 64 << 10
)

var (
	// Text outside a call: openers, a model-written result, stray closers.
	outsideTags      = []string{tagToolCall, tagFunction, tagPyStart, tagResponse, tagPyResponse, tagToolCallEnd, tagFunctionEnd, tagPyEnd}
	outsideTagsThink = append(append([]string{}, outsideTags...), tagThink, tagThinkEnd)
	// Ends of a <tool_call> body: its closer, or a new block or result that
	// proves the closer was forgotten.
	blockEnds = []string{tagToolCallEnd, tagToolCall, tagPyStart, tagResponse, tagPyResponse}
	pyEnds    = []string{tagPyEnd, tagPyStart, tagToolCall, tagResponse, tagPyResponse}
	fnFollows = []string{tagToolCallEnd, tagFunction, tagToolCall, tagResponse, tagPyResponse}
	fnTags    = []string{tagParamEq, tagParamSp, tagFunctionEnd, tagToolCallEnd, tagFunction}
	valueTags = []string{tagParamEnd, tagToolCallEnd, tagParamEq, tagParamSp, tagFunctionEnd}
)

type state int

const (
	stOutside   state = iota
	stThink           // inside <think>
	stBlock           // after <tool_call>, before its form is known
	stBody            // a JSON, GLM or Pythonic body, up to its closer
	stFuncHead        // after <function=
	stFunc            // inside a function, between parameters
	stParam           // inside a parameter value
	stAfterFunc       // after </function>
	stPyBare          // a bare [f(...)] reply
	stPass            // an oversized Auto candidate makes the rest visible
	stCut             // after a model-written tool result: nothing more
)

type form int

const (
	formJSON form = iota
	formGLM
	formPy
)

type match int

const (
	matchNo match = iota
	matchMaybe
	matchYes
)

type parsed struct {
	name string
	args object
}

// xcall is the open call of the XML form.
type xcall struct {
	id, name string
	args     object
	key      string
}

type scanner struct {
	auto, bare, thinking bool
	tools                map[string]*shape
	newID                func() string

	buf         string
	st          state
	from        int  // where the next terminator search starts in buf
	quote       byte // quoted JSON/Python text cannot end a call
	escape      bool
	visible     bool // non-space text went out: no bare Pythonic list now
	inBlock     bool // the open function sits in a <tool_call> block
	form        form
	closer      string
	raw         strings.Builder // markup of the open call, for Broken
	call        *xcall
	startID     string // Start already sent for the open body
	ev          []Event
	code        byte // Auto: 0 is prose, 1 is inline code, 2 is fenced code
	inlineTicks int  // closing inline delimiter must match its opening width
	lineIndent  byte // 0-3 leading spaces, 4 after other text on this line
}

func (s *scanner) Feed(text string) []Event {
	if text == "" || s.st == stCut {
		return nil
	}
	s.buf += text
	events := s.run(false, false)
	if s.auto && s.st != stOutside && s.st != stThink && s.st != stCut && s.st != stPass && s.raw.Len()+len(s.buf) > maxAutoHeld {
		s.ev = events
		s.text(s.raw.String())
		s.text(s.buf)
		events = s.ev
		s.ev = nil
		s.raw.Reset()
		s.buf = ""
		s.call = nil
		s.startID = ""
		s.st = stPass
	}
	return events
}

func (s *scanner) Flush(cut bool) []Event {
	if s.st == stCut {
		return nil
	}
	return s.run(true, cut)
}

func (s *scanner) run(final, cut bool) []Event {
	s.ev = nil
	for s.step(final, cut) {
	}
	if final {
		s.end()
	}
	ev := s.ev
	s.ev = nil
	return ev
}

// step advances one state; false means it needs more text.
func (s *scanner) step(final, cut bool) bool {
	switch s.st {
	case stOutside:
		return s.outside(final)
	case stThink:
		return s.think(final)
	case stBlock:
		return s.block(final)
	case stBody:
		return s.body(final, cut)
	case stFuncHead:
		return s.funcHead(final)
	case stFunc:
		return s.fn(final)
	case stParam:
		return s.param(final)
	case stAfterFunc:
		return s.afterFunc(final)
	case stPyBare:
		return s.pyBare(final, cut)
	case stPass:
		return s.pass()
	}
	return false
}

func (s *scanner) end() {
	if s.call != nil {
		s.broken(s.raw.String() + s.buf)
		s.call = nil
	}
	s.buf = ""
	s.from = 0
	s.inBlock = false
	s.startID = ""
	if s.st != stCut {
		s.st = stOutside
	}
}

func (s *scanner) outside(final bool) bool {
	if s.auto {
		return s.outsideAuto(final)
	}
	if s.buf == "" {
		return false
	}
	if s.bare && !s.visible {
		rest := strings.TrimLeft(s.buf, spaces)
		if rest == "" {
			if final {
				s.text(s.buf)
				s.buf = ""
			}
			return false
		}
		switch pyLead(rest) {
		case matchYes:
			s.text(s.buf[:len(s.buf)-len(rest)])
			s.buf = rest
			s.raw.Reset()
			s.st = stPyBare
			return true
		case matchMaybe:
			if !final {
				return false
			}
		}
	}
	tags := outsideTags
	if s.thinking {
		tags = outsideTagsThink
	}
	i, tag := firstTag(s.buf, 0, tags)
	if i < 0 {
		hold := 0
		if !final {
			hold = overlap(s.buf, tags)
		}
		s.text(s.buf[:len(s.buf)-hold])
		s.buf = s.buf[len(s.buf)-hold:]
		return false
	}
	s.text(s.buf[:i])
	s.buf = s.buf[i+len(tag):]
	s.raw.Reset()
	s.raw.WriteString(tag)
	s.from = 0
	s.startID = ""
	switch tag {
	case tagToolCall:
		s.inBlock = true
		s.st = stBlock
	case tagFunction:
		s.inBlock = false
		s.st = stFuncHead
	case tagPyStart:
		s.form, s.closer = formPy, tagPyEnd
		s.st = stBody
	case tagThink:
		s.st = stThink
	case tagResponse, tagPyResponse:
		s.ev = append(s.ev, Event{Fabricated: true})
		s.st = stCut
		s.buf = ""
		return false
	}
	return true
}

func (s *scanner) outsideAuto(final bool) bool {
	tags := outsideTags
	if s.thinking {
		tags = outsideTagsThink
	}
	for i := 0; i < len(s.buf); {
		if s.buf[i] == '`' {
			j := i + 1
			for j < len(s.buf) && s.buf[j] == '`' {
				j++
			}
			if j == len(s.buf) && !final {
				s.text(s.buf[:i])
				s.buf = s.buf[i:]
				return false
			}
			width := j - i
			switch {
			case s.code == 2 && s.lineIndent <= 3 && width >= 3 && (j == len(s.buf) || strings.IndexByte(" \t\r\n", s.buf[j]) >= 0):
				s.code = 0
			case s.code == 1 && width == s.inlineTicks:
				s.code = 0
			case s.code == 0 && width >= 3 && s.lineIndent <= 3:
				s.code = 2
			case s.code == 0:
				s.code = 1
				s.inlineTicks = width
			}
			s.lineIndent = 4
			i = j
			continue
		}
		if s.code == 0 && s.buf[i] == '<' {
			for _, tag := range tags {
				if !strings.HasPrefix(s.buf[i:], tag) {
					continue
				}
				switch tag {
				case tagToolCall, tagFunction, tagPyStart, tagThink, tagResponse, tagPyResponse:
					s.text(s.buf[:i])
					s.buf = s.buf[i+len(tag):]
					s.raw.Reset()
					s.raw.WriteString(tag)
					s.from = 0
					s.startID = ""
					switch tag {
					case tagToolCall:
						s.inBlock = true
						s.st = stBlock
					case tagFunction:
						s.inBlock = false
						s.st = stFuncHead
					case tagPyStart:
						s.form, s.closer = formPy, tagPyEnd
						s.st = stBody
					case tagThink:
						s.st = stThink
					default:
						s.ev = append(s.ev, Event{Fabricated: true})
						s.st = stCut
						s.buf = ""
						return false
					}
					return true
				}
			}
			if !final && prefixOfAny(s.buf[i:], tags...) {
				s.text(s.buf[:i])
				s.buf = s.buf[i:]
				return false
			}
		}
		switch s.buf[i] {
		case '\n':
			s.lineIndent = 0
		case ' ':
			if s.lineIndent < 4 {
				s.lineIndent++
			}
		default:
			s.lineIndent = 4
		}
		i++
	}
	s.text(s.buf)
	s.buf = ""
	return false
}

func (s *scanner) pass() bool {
	s.text(s.buf)
	s.buf = ""
	return false
}

func (s *scanner) think(final bool) bool {
	i := strings.Index(s.buf, tagThinkEnd)
	if i < 0 {
		hold := 0
		if !final {
			hold = overlap(s.buf, []string{tagThinkEnd})
		}
		s.reasoning(s.buf[:len(s.buf)-hold])
		s.buf = s.buf[len(s.buf)-hold:]
		if final {
			s.st = stOutside
		}
		return false
	}
	s.reasoning(s.buf[:i])
	s.buf = s.buf[i+len(tagThinkEnd):]
	s.st = stOutside
	return true
}

func (s *scanner) block(final bool) bool {
	j := skipSpace(s.buf, 0)
	rest := s.buf[j:]
	if rest == "" {
		if final {
			s.broken(s.raw.String() + s.buf)
			s.buf = ""
			s.st = stOutside
		}
		return false
	}
	switch {
	case strings.HasPrefix(rest, tagFunction):
		s.raw.WriteString(s.buf[:j+len(tagFunction)])
		s.buf = rest[len(tagFunction):]
		s.st = stFuncHead
		return true
	case strings.HasPrefix(rest, tagToolCallEnd):
		s.buf = rest[len(tagToolCallEnd):]
		s.inBlock = false
		s.st = stOutside
		return true
	case !final && prefixOfAny(rest, tagFunction, tagToolCallEnd):
		return false
	}
	f := formJSON
	switch c := rest[0]; {
	case c == '[':
		switch pyLead(rest) {
		case matchYes:
			f = formPy
		case matchMaybe:
			if !final {
				return false
			}
		}
	case isIdentStart(c):
		f = formGLM
	}
	s.raw.WriteString(s.buf[:j])
	s.buf = rest
	s.form, s.closer = f, tagToolCallEnd
	s.from = 0
	s.quote, s.escape = 0, false
	s.st = stBody
	return true
}

// body waits for the closer of a JSON, GLM or Pythonic body and parses it
// whole; only the name is read early, for Start.
func (s *scanner) body(final, cut bool) bool {
	if !s.auto && s.startID == "" {
		if name := s.bodyName(); name != "" {
			s.startID = s.newID()
			s.start(s.startID, name)
		}
	}
	ends := blockEnds
	if s.closer == tagPyEnd {
		ends = pyEnds
	}
	i, tag := s.bodyEnd(ends)
	if i < 0 {
		if !final {
			return false
		}
		body := s.buf
		s.buf = ""
		s.st = stOutside
		s.inBlock = false
		if cut || s.auto {
			s.broken(s.raw.String() + body)
			s.startID = ""
			return false
		}
		s.endBody(body, false)
		return s.buf != ""
	}
	body := s.buf[:i]
	closed := tag == s.closer
	if closed {
		s.buf = s.buf[i+len(tag):]
	} else {
		s.buf = s.buf[i:]
	}
	s.st = stOutside
	s.inBlock = false
	s.endBody(body, closed)
	return true
}

// bodyEnd searches only new bytes; a partial tag stays in buf and is
// reconsidered with the next delta. Quoted arguments can contain markup.
func (s *scanner) bodyEnd(tags []string) (int, string) {
	for i := s.from; i < len(s.buf); i++ {
		c := s.buf[i]
		if s.form != formGLM {
			if s.quote != 0 {
				if s.escape {
					s.escape = false
				} else if c == '\\' {
					s.escape = true
				} else if c == s.quote {
					s.quote = 0
				}
				continue
			}
			if c == '"' || c == '\'' {
				s.quote = c
				continue
			}
		}
		if c != '<' {
			continue
		}
		for _, tag := range tags {
			if strings.HasPrefix(s.buf[i:], tag) {
				return i, tag
			}
		}
		if prefixOfAny(s.buf[i:], tags...) {
			s.from = i
			return -1, ""
		}
	}
	s.from = len(s.buf)
	return -1, ""
}

func (s *scanner) bodyName() string {
	switch s.form {
	case formJSON:
		return s.toolName(jsonName(s.buf))
	case formGLM:
		i := firstOf(s.buf, "\n", tagArgKey, tagToolCallEnd)
		if i < 0 {
			return ""
		}
		if name := s.toolName(s.buf[:i]); validName(name) {
			return name
		}
		return ""
	}
	return s.toolName(pyFirstName(s.buf))
}

func (s *scanner) endBody(body string, closed bool) {
	raw := s.raw.String() + body
	if closed {
		raw += s.closer
	}
	var calls []parsed
	var rest string
	switch s.form {
	case formJSON:
		calls, rest = s.jsonBody(body, closed)
	case formGLM:
		if c, ok := s.parseGLM(body); ok {
			calls = []parsed{c}
		} else {
			calls, _ = s.parsePy(body)
		}
	case formPy:
		calls, _ = s.parsePy(body)
	}
	if len(calls) == 0 {
		s.broken(raw)
		s.startID = ""
		return
	}
	if s.auto {
		for _, c := range calls {
			if !s.offered(c.name) {
				s.broken(raw)
				return
			}
		}
	}
	s.emitCalls(calls)
	// Text after a whole JSON call in a block that never closed is the
	// model talking again; scan it as such.
	s.buf = rest + s.buf
}

func (s *scanner) funcHead(final bool) bool {
	j := strings.IndexAny(s.buf, ">\n<")
	if j < 0 || j > maxName {
		if j < 0 && len(s.buf) <= maxName && !final {
			return false
		}
		s.broken(s.raw.String() + s.buf)
		s.buf = ""
		s.st = stOutside
		s.inBlock = false
		return false
	}
	name := s.toolName(s.buf[:j])
	if !validName(name) || s.auto && !s.offered(name) {
		if s.auto {
			end := j
			if s.buf[j] == '>' {
				end++
			}
			s.text(s.raw.String() + s.buf[:end])
			s.buf = s.buf[end:]
		} else {
			s.broken(s.raw.String() + s.buf[:j])
		}
		s.st = stOutside
		s.inBlock = false
		return true
	}
	if s.buf[j] == '>' {
		j++
	}
	s.raw.WriteString(s.buf[:j])
	s.buf = s.buf[j:]
	s.call = &xcall{id: s.newID(), name: name}
	if !s.auto {
		s.start(s.call.id, name)
	}
	s.st = stFunc
	return true
}

func (s *scanner) fn(final bool) bool {
	j := skipSpace(s.buf, 0)
	rest := s.buf[j:]
	if rest == "" {
		s.raw.WriteString(s.buf)
		s.buf = ""
		return false
	}
	switch {
	case strings.HasPrefix(rest, tagParamEq) || strings.HasPrefix(rest, tagParamSp):
		k := strings.IndexByte(rest, '>')
		if k < 0 || k > maxHeader {
			if k < 0 && len(rest) <= maxHeader && !final {
				return false
			}
			s.buf = rest
			if s.auto {
				s.broken(s.raw.String() + s.buf)
				s.call = nil
				s.buf = ""
			} else {
				s.endCall()
			}
			s.closeFunc()
			return true
		}
		key, self := paramKey(rest[len("<parameter"):k])
		s.raw.WriteString(s.buf[:j+k+1])
		s.buf = rest[k+1:]
		switch {
		case key == "":
		case self:
			s.setArg(key, "")
		default:
			s.call.key = key
			s.from = 0
			s.st = stParam
		}
		return true
	case strings.HasPrefix(rest, tagFunctionEnd):
		s.raw.WriteString(s.buf[:j+len(tagFunctionEnd)])
		s.buf = rest[len(tagFunctionEnd):]
		s.endCall()
		s.st = stAfterFunc
		return true
	case strings.HasPrefix(rest, tagToolCallEnd):
		s.buf = rest[len(tagToolCallEnd):]
		s.endCall()
		s.closeFunc()
		return true
	case strings.HasPrefix(rest, tagFunction):
		s.buf = rest
		s.endCall()
		s.st = stAfterFunc
		return true
	case !final && prefixOfAny(rest, fnTags...):
		s.raw.WriteString(s.buf[:j])
		s.buf = rest
		return false
	}
	// Prose where a parameter belongs is not proof of a call in Auto.
	if s.auto {
		s.broken(s.raw.String() + s.buf)
		s.buf = ""
		s.call = nil
	} else {
		s.buf = rest
		s.endCall()
	}
	s.closeFunc()
	return true
}

func (s *scanner) closeFunc() {
	s.inBlock = false
	s.st = stOutside
}

func (s *scanner) param(final bool) bool {
	end, next, legit := valueEnd(s.buf, s.from, final)
	if end < 0 {
		if !final {
			s.from = max(0, len(s.buf)-scanBack)
			return false
		}
		end, next = len(s.buf), len(s.buf)
	}
	value := trimValue(s.buf[:end], legit)
	s.raw.WriteString(s.buf[:next])
	s.buf = s.buf[next:]
	s.setArg(s.call.key, value)
	s.call.key = ""
	s.st = stFunc
	return true
}

func (s *scanner) afterFunc(final bool) bool {
	rest := s.buf[skipSpace(s.buf, 0):]
	if rest == "" {
		s.buf = ""
		return false
	}
	switch {
	case strings.HasPrefix(rest, tagToolCallEnd):
		s.buf = rest[len(tagToolCallEnd):]
		s.closeFunc()
		return true
	case strings.HasPrefix(rest, tagFunction):
		s.raw.Reset()
		s.raw.WriteString(tagFunction)
		s.buf = rest[len(tagFunction):]
		s.st = stFuncHead
		return true
	case !final && prefixOfAny(rest, tagToolCallEnd, tagFunction):
		s.buf = rest
		return false
	}
	s.buf = rest
	s.closeFunc()
	return true
}

func (s *scanner) pyBare(final, cut bool) bool {
	end := pyListEnd(s.buf)
	if end < 0 && !final {
		return false
	}
	body := s.buf
	if end >= 0 {
		body, s.buf = s.buf[:end], s.buf[end:]
	} else {
		s.buf = ""
	}
	s.st = stOutside
	s.visible = true
	calls, err := s.parsePy(body)
	switch {
	case err == nil && end >= 0:
		s.emitCalls(calls)
	case end < 0:
		s.broken(body)
	default:
		s.text(body)
	}
	return end >= 0
}

func (s *scanner) setArg(key, raw string) {
	key = html.UnescapeString(key)
	s.call.args.set(key, decodeArg(html.UnescapeString(raw), s.tools[s.call.name].prop(key)))
}

func (s *scanner) endCall() {
	if s.call == nil {
		return
	}
	c := s.call
	s.call = nil
	if s.auto {
		s.start(c.id, c.name)
	}
	s.finish(c.id, c.name, c.args)
}

func (s *scanner) emitCalls(calls []parsed) {
	for k, c := range calls {
		if s.auto && !s.offered(c.name) {
			continue
		}
		id := s.startID
		if k > 0 || id == "" {
			id = s.newID()
			s.start(id, c.name)
		}
		s.finish(id, c.name, c.args)
	}
	s.startID = ""
}

func (s *scanner) finish(id, name string, args object) {
	if sh := s.tools[name]; sh != nil {
		args = coerceObject(args, sh.props, 0)
	}
	s.ev = append(s.ev, Event{Call: &llm.ToolCall{ID: id, Name: name, Arguments: marshal(args)}})
}

func (s *scanner) start(id, name string) {
	s.ev = append(s.ev, Event{Start: &llm.ToolCall{ID: id, Name: name}})
}

func (s *scanner) broken(raw string) {
	if s.auto {
		s.text(raw)
		return
	}
	s.ev = append(s.ev, Event{Broken: raw})
}

func (s *scanner) text(t string) {
	if t == "" {
		return
	}
	if strings.TrimLeft(t, spaces) != "" {
		s.visible = true
	}
	if n := len(s.ev); n > 0 && s.ev[n-1].Text != "" {
		s.ev[n-1].Text += t
		return
	}
	s.ev = append(s.ev, Event{Text: t})
}

func (s *scanner) reasoning(t string) {
	if t == "" {
		return
	}
	if n := len(s.ev); n > 0 && s.ev[n-1].Reasoning != "" {
		s.ev[n-1].Reasoning += t
		return
	}
	s.ev = append(s.ev, Event{Reasoning: t})
}

func (s *scanner) offered(name string) bool {
	_, ok := s.tools[name]
	return ok
}

// toolName cleans a written name and maps a dotted one (functions.read,
// default_api.read) to the offered tool it ends in.
func (s *scanner) toolName(n string) string {
	n = strings.TrimSpace(strings.Trim(strings.TrimSpace(n), "\"'`"))
	if _, ok := s.tools[n]; ok || len(s.tools) == 0 {
		return n
	}
	if i := strings.LastIndexByte(n, '.'); i >= 0 {
		if _, ok := s.tools[n[i+1:]]; ok {
			return n[i+1:]
		}
	}
	return n
}

// valueEnd finds where a parameter value ends: at its </parameter>
// (legit), or where a forgotten closer shows: a new parameter header, a
// </function> followed by the end of the call, or </tool_call>. end is -1
// while that cannot be told yet.
func valueEnd(s string, from int, final bool) (end, next int, legit bool) {
	for i := from; i < len(s); i++ {
		k := strings.IndexByte(s[i:], '<')
		if k < 0 {
			break
		}
		i += k
		rest := s[i:]
		switch {
		case strings.HasPrefix(rest, tagParamEnd):
			return i, i + len(tagParamEnd), true
		case strings.HasPrefix(rest, tagToolCallEnd):
			return i, i, false
		case strings.HasPrefix(rest, tagParamEq) || strings.HasPrefix(rest, tagParamSp):
			switch paramHeader(rest) {
			case matchYes:
				return i, i, false
			case matchMaybe:
				if !final {
					return -1, -1, false
				}
			}
		case strings.HasPrefix(rest, tagFunctionEnd):
			switch follows(rest[len(tagFunctionEnd):], final, fnFollows) {
			case matchYes:
				return i, i, false
			case matchMaybe:
				return -1, -1, false
			}
		case !final && prefixOfAny(rest, valueTags...):
			return -1, -1, false
		}
	}
	return -1, -1, false
}

// paramHeader tells whether rest opens a complete <parameter ...> header.
func paramHeader(rest string) match {
	for i := len("<parameter"); i < len(rest); i++ {
		if i > maxHeader {
			return matchNo
		}
		switch rest[i] {
		case '>':
			return matchYes
		case '<', '\n':
			return matchNo
		}
	}
	return matchMaybe
}

// follows tells whether, after at most healSpace spaces, s starts with one
// of tags; the end of a final stream counts.
func follows(s string, final bool, tags []string) match {
	i := 0
	for i < len(s) && strings.IndexByte(spaces, s[i]) >= 0 {
		i++
		if i > healSpace {
			return matchNo
		}
	}
	rest := s[i:]
	if rest == "" {
		if final {
			return matchYes
		}
		return matchMaybe
	}
	for _, t := range tags {
		if strings.HasPrefix(rest, t) {
			return matchYes
		}
	}
	if !final && prefixOfAny(rest, tags...) {
		return matchMaybe
	}
	return matchNo
}

func paramKey(h string) (key string, self bool) {
	h = strings.TrimSpace(h)
	if strings.HasSuffix(h, "/") {
		self = true
		h = strings.TrimSpace(h[:len(h)-1])
	}
	switch {
	case strings.HasPrefix(h, "="):
		key = h[1:]
	default:
		if i := strings.Index(h, "name="); i >= 0 {
			key = h[i+len("name="):]
		}
	}
	return strings.TrimSpace(strings.Trim(strings.TrimSpace(key), "\"'")), self
}

// trimValue drops the one newline the Qwen3-Coder template wraps a value
// in; a healed end also loses the spacing before the tag that ended it.
func trimValue(v string, legit bool) string {
	if strings.HasPrefix(v, "\r\n") {
		v = v[2:]
	} else if strings.HasPrefix(v, "\n") {
		v = v[1:]
	}
	if !legit {
		return strings.TrimRight(v, spaces)
	}
	if strings.HasSuffix(v, "\r\n") {
		return v[:len(v)-2]
	}
	return strings.TrimSuffix(v, "\n")
}

func (s *scanner) jsonBody(body string, closed bool) ([]parsed, string) {
	t := strings.TrimSpace(body)
	v, err := parseRelaxed(t)
	rest := ""
	if err != nil {
		if closed {
			return nil, ""
		}
		end := jsonPrefix(t)
		if end < 0 {
			return nil, ""
		}
		if v, err = parseRelaxed(t[:end]); err != nil {
			return nil, ""
		}
		rest = t[end:]
	}
	var calls []parsed
	switch v := v.(type) {
	case object:
		if c, ok := s.jsonCall(v); ok {
			calls = append(calls, c)
		}
	case []any:
		for _, e := range v {
			if o, ok := e.(object); ok {
				if c, ok := s.jsonCall(o); ok {
					calls = append(calls, c)
					continue
				}
			}
			if s.auto {
				return nil, ""
			}
		}
	}
	if len(calls) == 0 {
		return nil, ""
	}
	return calls, rest
}

// jsonCall reads {"name":..,"arguments":{..}} and the shapes models drift
// to: "parameters" or "args" for the arguments, arguments as a JSON string,
// an OpenAI-style "function" object.
func (s *scanner) jsonCall(o object) (parsed, bool) {
	if f, ok := o.get("function"); ok {
		if fo, ok := f.(object); ok {
			return s.jsonCall(fo)
		}
	}
	var name string
	for _, k := range []string{"name", "function", "tool", "tool_name"} {
		if v, ok := o.get(k); ok {
			if n, ok := v.(string); ok && strings.TrimSpace(n) != "" {
				name = s.toolName(n)
				break
			}
		}
	}
	if name == "" || s.auto && !s.offered(name) {
		return parsed{}, false
	}
	var args any
	for _, k := range []string{"arguments", "parameters", "args", "input"} {
		if v, ok := o.get(k); ok {
			args = v
			break
		}
	}
	if str, ok := args.(string); ok {
		if strings.TrimSpace(str) == "" {
			args = object{}
		} else {
			var err error
			args, err = parseRelaxed(strings.TrimSpace(str))
			if err != nil {
				return parsed{}, false
			}
		}
	}
	if args == nil {
		args = object{}
	}
	a, ok := args.(object)
	return parsed{name: name, args: a}, ok
}

// jsonName reads the name of a JSON call body once its string has closed,
// so Start never carries half a name.
func jsonName(b string) string {
	i := skipSpace(b, 0)
	if i < len(b) && b[i] == '[' {
		i = skipSpace(b, i+1)
	}
	if i >= len(b) || b[i] != '{' {
		return ""
	}
	i = skipSpace(b, i+1)
	if !strings.HasPrefix(b[i:], `"name"`) {
		return ""
	}
	i = skipSpace(b, i+len(`"name"`))
	if i >= len(b) || b[i] != ':' {
		return ""
	}
	i = skipSpace(b, i+1)
	if i >= len(b) || b[i] != '"' {
		return ""
	}
	l := &lexer{src: b, pos: i}
	name, err := l.str('"')
	if err != nil {
		return ""
	}
	return name
}

// parseGLM reads NAME then <arg_key>K</arg_key><arg_value>V</arg_value>
// pairs, healing a forgotten or wrong value closer the way omp's GLM
// scanner does.
func (s *scanner) parseGLM(body string) (parsed, bool) {
	i := firstOf(body, "\n", tagArgKey)
	if i < 0 {
		i = len(body)
	}
	name := s.toolName(body[:i])
	if !validName(name) || s.auto && !s.offered(name) {
		return parsed{}, false
	}
	sh := s.tools[name]
	var args object
	rest := body[i:]
	for {
		k := strings.Index(rest, tagArgKey)
		if k < 0 {
			if strings.TrimSpace(rest) != "" {
				return parsed{}, false
			}
			break
		}
		if strings.TrimSpace(rest[:k]) != "" {
			return parsed{}, false
		}
		rest = rest[k+len(tagArgKey):]
		ke := strings.Index(rest, tagArgKeyEnd)
		if ke < 0 {
			return parsed{}, false
		}
		key := html.UnescapeString(strings.TrimSpace(rest[:ke]))
		if key == "" {
			return parsed{}, false
		}
		rest = strings.TrimLeft(rest[ke+len(tagArgKeyEnd):], spaces)
		if !strings.HasPrefix(rest, tagArgValue) {
			return parsed{}, false
		}
		rest = rest[len(tagArgValue):]
		end, next, legit := glmValueEnd(rest)
		value := rest[:end]
		if !legit {
			value = strings.TrimRight(value, spaces)
		}
		args.set(key, decodeArg(html.UnescapeString(value), sh.prop(key)))
		rest = rest[next:]
	}
	return parsed{name: name, args: args}, true
}

func glmValueEnd(s string) (end, next int, legit bool) {
	for i := 0; i < len(s); i++ {
		k := strings.IndexByte(s[i:], '<')
		if k < 0 {
			break
		}
		i += k
		rest := s[i:]
		switch {
		case strings.HasPrefix(rest, tagArgValueEnd):
			return i, i + len(tagArgValueEnd), true
		case strings.HasPrefix(rest, tagArgKeyEnd):
			after := rest[len(tagArgKeyEnd):]
			if follows(after, true, []string{tagArgKey, tagToolCallEnd}) == matchYes {
				return i, i + len(tagArgKeyEnd), false
			}
			if follows(after, false, []string{tagArgValueEnd}) == matchYes {
				j := strings.Index(after, tagArgValueEnd)
				return i, i + len(tagArgKeyEnd) + j + len(tagArgValueEnd), false
			}
		case strings.HasPrefix(rest, tagArgKey):
			if glmNextPair(rest) {
				return i, i, false
			}
		}
	}
	return len(s), len(s), false
}

// glmNextPair tells whether rest opens a whole next <arg_key>…</arg_key>
// <arg_value> pair: the model started it without closing the value.
func glmNextPair(rest string) bool {
	k := rest[len(tagArgKey):]
	e := strings.IndexAny(k, "<\n")
	if e < 0 || e > maxName || !strings.HasPrefix(k[e:], tagArgKeyEnd) {
		return false
	}
	return follows(k[e+len(tagArgKeyEnd):], false, []string{tagArgValue}) == matchYes
}

func firstTag(s string, from int, tags []string) (int, string) {
	best, tag := -1, ""
	for _, t := range tags {
		if i := strings.Index(s[from:], t); i >= 0 && (best < 0 || from+i < best) {
			best, tag = from+i, t
		}
	}
	return best, tag
}

func firstOf(s string, subs ...string) int {
	best := -1
	for _, t := range subs {
		if i := strings.Index(s, t); i >= 0 && (best < 0 || i < best) {
			best = i
		}
	}
	return best
}

// overlap is the length of the longest tail of s that is a proper prefix
// of a tag: text that may still become markup.
func overlap(s string, tags []string) int {
	best := 0
	for _, t := range tags {
		for k := min(len(s), len(t)-1); k > best; k-- {
			if strings.HasSuffix(s, t[:k]) {
				best = k
				break
			}
		}
	}
	return best
}

// prefixOfAny reports whether s is a proper prefix of one of tags.
func prefixOfAny(s string, tags ...string) bool {
	for _, t := range tags {
		if len(s) < len(t) && strings.HasPrefix(t, s) {
			return true
		}
	}
	return false
}

func longest(tags []string) int {
	n := 0
	for _, t := range tags {
		n = max(n, len(t))
	}
	return n
}

func skipSpace(s string, i int) int {
	for i < len(s) && strings.IndexByte(spaces, s[i]) >= 0 {
		i++
	}
	return i
}

func isIdentStart(c byte) bool { return c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c == '_' }

func validName(n string) bool {
	if n == "" || len(n) > maxName || !isIdentStart(n[0]) {
		return false
	}
	for i := 1; i < len(n); i++ {
		c := n[i]
		if !isIdent(c) && c != '.' && c != '-' && c != ':' || c == '$' {
			return false
		}
	}
	return true
}
