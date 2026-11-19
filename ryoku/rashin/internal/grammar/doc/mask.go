package doc

import (
	"regexp"
	"ryoku-rashin/internal/grammar/dict"
	"ryoku-rashin/internal/grammar/text"
	"slices"
	"sort"
)

// TextMask selects rune spans visible to a parser. Mask names the parser adapter.
type TextMask struct{ allowed []text.Span }
type AllowedSpan struct {
	Span    text.Span
	Content []rune
}
type Masker interface{ CreateMask([]rune) TextMask }

func NewBlankMask() TextMask { return TextMask{} }
func NewTextMask(spans []text.Span) TextMask {
	allowed := slices.Clone(spans)
	sort.SliceStable(allowed, func(i, j int) bool { return allowed[i].Start < allowed[j].Start })
	for i := 1; i < len(allowed); i++ {
		if allowed[i-1].End > allowed[i].Start {
			panic("Masker elements cannot overlap and must be sorted!")
		}
	}
	return TextMask{allowed: allowed}
}
func (m *TextMask) PushAllowed(s text.Span) {
	if n := len(m.allowed); n > 0 {
		last := &m.allowed[n-1]
		if s.Start < last.End {
			panic("Masker elements cannot overlap and must be sorted!")
		}
		if s.Start == last.End {
			last.End = s.End
			return
		}
	}
	m.allowed = append(m.allowed, s)
}
func (m TextMask) IterAllowed(source []rune) []AllowedSpan {
	out := make([]AllowedSpan, len(m.allowed))
	for i, s := range m.allowed {
		out[i] = AllowedSpan{Span: s, Content: s.GetContent(source)}
	}
	return out
}
func (m *TextMask) MergeWhitespaceSep(source []rune) {
	out := m.allowed[:0]
	for _, s := range m.allowed {
		if len(out) > 0 {
			last := &out[len(out)-1]
			if allWhitespace(source[last.End:s.Start]) {
				last.End = s.End
				continue
			}
		}
		out = append(out, s)
	}
	m.allowed = out
}

type RegexMasker struct {
	regex          *regexp.Regexp
	excludeMatches bool
}

func NewRegexMasker(pattern string, exclude bool) (*RegexMasker, bool) {
	r, err := regexp.Compile(pattern)
	if err != nil {
		return nil, false
	}
	return &RegexMasker{regex: r, excludeMatches: exclude}, true
}
func (m *RegexMasker) CreateMask(source []rune) TextMask {
	s := string(source)
	offsets := text.BuildByteToCharMap(s)
	mask := NewBlankMask()
	start := 0
	for _, match := range m.regex.FindAllStringIndex(s, -1) {
		a, b := offsets[match[0]], offsets[match[1]]
		if m.excludeMatches {
			if start < a {
				mask.PushAllowed(text.Span{Start: start, End: a})
			}
			start = b
		} else if a < b {
			mask.PushAllowed(text.Span{Start: a, End: b})
		}
	}
	if m.excludeMatches && start < len(source) {
		mask.PushAllowed(text.Span{Start: start, End: len(source)})
	}
	return mask
}

type Mask struct {
	Masker Masker
	Parser Parser
}

func NewMask(masker Masker, parser Parser) *Mask { return &Mask{Masker: masker, Parser: parser} }
func (p *Mask) Parse(source []rune) []Token {
	mask := p.Masker.CreateMask(source)
	var tokens []Token
	last := -1
	for _, allowed := range mask.IterAllowed(source) {
		if last >= 0 && slices.Contains(source[last:allowed.Span.Start], '\n') {
			tokens = append(tokens, Token{Span: text.Span{Start: last, End: allowed.Span.Start}, Kind: TokenKind{Kind: ParagraphBreak}})
		}
		next := p.Parser.Parse(allowed.Content)
		for i := range next {
			next[i].Span.Start += allowed.Span.Start
			next[i].Span.End += allowed.Span.Start
		}
		tokens = append(tokens, next...)
		last = allowed.Span.End
	}
	return tokens
}
func (p *Mask) ParseStr(source string) []Token { return p.Parse([]rune(source)) }

type CollapseIdentifiers struct {
	inner      Parser
	dictionary dict.Dictionary
}

func NewCollapseIdentifiers(inner Parser, dictionary dict.Dictionary) *CollapseIdentifiers {
	return &CollapseIdentifiers{inner: inner, dictionary: dictionary}
}
func (p *CollapseIdentifiers) Parse(source []rune) []Token {
	tokens := p.inner.Parse(source)
	remove := map[int]bool{}
	for i := 0; i < len(tokens); {
		if !tokens[i].Kind.IsWord() {
			i++
			continue
		}
		end := i + 1
		for end+1 < len(tokens) && tokens[end].Kind.IsCaseSeparator() && tokens[end+1].Kind.IsWord() {
			end += 2
		}
		if end > i+1 {
			span := text.Span{Start: tokens[i].Span.Start, End: tokens[end-1].Span.End}
			if p.dictionary.ContainsWord(span.GetContent(source)) {
				tokens[i] = Token{Span: span, Kind: BlankWord()}
				for j := i + 1; j < end; j++ {
					remove[j] = true
				}
			}
		}
		i = end
	}
	out := tokens[:0]
	for i, t := range tokens {
		if !remove[i] {
			out = append(out, t)
		}
	}
	return out
}
func (p *CollapseIdentifiers) ParseStr(source string) []Token { return p.Parse([]rune(source)) }

type IsolateEnglish struct {
	inner      Parser
	dictionary dict.Dictionary
}

func NewIsolateEnglish(inner Parser, dictionary dict.Dictionary) *IsolateEnglish {
	return &IsolateEnglish{inner: inner, dictionary: dictionary}
}
func (p *IsolateEnglish) Parse(source []rune) []Token {
	tokens := p.inner.Parse(source)
	out := make([]Token, 0, len(tokens))
	for _, chunk := range IterChunks(tokens) {
		if len(chunk) < 4 || IsLikelyEnglish(chunk, source, p.dictionary) {
			out = append(out, chunk...)
		}
	}
	return out
}
func (p *IsolateEnglish) ParseStr(source string) []Token { return p.Parse([]rune(source)) }
