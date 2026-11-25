package doc

import (
	"ryoku-rashin/internal/grammar/dict"
	"ryoku-rashin/internal/grammar/pos"
	"ryoku-rashin/internal/grammar/text"
	"sort"
	"strings"
	"unicode"
)

type Parser interface{ Parse([]rune) []Token }
type PlainEnglish struct{}

func NewPlainEnglishParser() PlainEnglish             { return PlainEnglish{} }
func (PlainEnglish) Parse(source []rune) []Token      { return LexWith(source, LexEnglishToken) }
func (p PlainEnglish) ParseStr(source string) []Token { return p.Parse([]rune(source)) }
func ParseStr(p Parser, source string) []Token        { return p.Parse([]rune(source)) }

type Document struct {
	source []rune
	tokens []Token
}

func NewDocument(source string, p Parser, d dict.Dictionary) *Document {
	return NewFromChars([]rune(source), p, d)
}
func NewCurated(source string, p Parser) *Document { return NewDocument(source, p, dict.Curated()) }
func NewFromChars(source []rune, p Parser, d dict.Dictionary) *Document {
	document := &Document{source: source, tokens: p.Parse(source)}
	document.applyFixups()
	document.annotate(d)
	return document
}
func NewPlainEnglishCuratedChars(source []rune) *Document {
	return NewFromChars(source, PlainEnglish{}, dict.Curated())
}
func NewPlainEnglishCurated(source string) *Document { return NewPlainEnglish(source, dict.Curated()) }
func NewPlainEnglish(source string, d dict.Dictionary) *Document {
	return NewDocument(source, PlainEnglish{}, d)
}
func NewBasicTokenize(source string, p Parser) *Document {
	s := []rune(source)
	d := &Document{source: s, tokens: p.Parse(s)}
	d.applyFixups()
	return d
}
func (d *Document) Tokens() []Token                      { return d.tokens }
func (d *Document) TokensMut() []Token                   { return d.tokens }
func (d *Document) GetTokens() []Token                   { return d.tokens }
func (d *Document) GetSource() []rune                    { return d.source }
func (d *Document) GetFullContent() []rune               { return d.source }
func (d *Document) GetFullString() string                { return string(d.source) }
func (d *Document) GetSpanContent(s text.Span) []rune    { return d.source[s.Start:s.End] }
func (d *Document) GetSpanContentStr(s text.Span) string { return string(d.GetSpanContent(s)) }
func (d *Document) String() string {
	var b strings.Builder
	for _, t := range d.tokens {
		b.WriteString(t.GetStr(d.source))
	}
	return b.String()
}
func (d *Document) GetToken(index int) (*Token, bool) {
	if index < 0 || index >= len(d.tokens) {
		return nil, false
	}
	return &d.tokens[index], true
}
func (d *Document) GetTokenOffset(base, offset int) (*Token, bool) { return d.GetToken(base + offset) }
func (d *Document) GetNextWordFromOffset(base, offset int) (*Token, bool) {
	t, ok := d.GetTokenOffset(base, offset)
	if !ok || !t.Kind.IsWhitespace() {
		return nil, false
	}
	sign := 0
	if offset < 0 {
		sign = -1
	} else if offset > 0 {
		sign = 1
	}
	t, ok = d.GetTokenOffset(base, offset+sign)
	return t, ok && t.Kind.IsWord()
}
func (d *Document) GetTokenAtCharIndex(index int) (*Token, bool) {
	i := sort.Search(len(d.tokens), func(i int) bool { return d.tokens[i].Span.End > index })
	if i == len(d.tokens) || d.tokens[i].Span.Start > index {
		return nil, false
	}
	return &d.tokens[i], true
}
func (d *Document) TokenIndicesIntersecting(s text.Span) []int {
	var out []int
	for i, t := range d.tokens {
		if t.Span.Start < s.End && s.Start < t.Span.End {
			out = append(out, i)
		}
	}
	return out
}
func (d *Document) FatTokensIntersecting(s text.Span) []FatToken {
	var out []FatToken
	for _, i := range d.TokenIndicesIntersecting(s) {
		out = append(out, d.tokens[i].ToFat(d.source))
	}
	return out
}
func (d *Document) FatTokens() []FatToken {
	out := make([]FatToken, len(d.tokens))
	for i, t := range d.tokens {
		out[i] = t.ToFat(d.source)
	}
	return out
}
func (d *Document) FatStringTokens() []FatStringToken {
	out := make([]FatStringToken, len(d.tokens))
	for i, t := range d.tokens {
		out[i] = FatStringToken{Content: t.GetStr(d.source), Kind: t.Kind}
	}
	return out
}
func (d *Document) IterNominalPhrases() [][]Token {
	var out [][]Token
	start := 0
	for i := 0; i <= len(d.tokens); i++ {
		if i < len(d.tokens) && (d.tokens[i].Kind.IsNpMember() || d.tokens[i].Kind.IsWhitespace()) {
			continue
		}
		a, b := start, i
		for a < b && d.tokens[a].Kind.IsWhitespace() {
			a++
		}
		for b > a && d.tokens[b-1].Kind.IsWhitespace() {
			b--
		}
		if a < b {
			out = append(out, d.tokens[a:b])
		}
		start = i + 1
	}
	return out
}
func (d *Document) annotate(dictionary dict.Dictionary) {
	tagger := pos.SharedBrillTagger()
	chunker := pos.SharedBurnChunker()
	for _, sentence := range IterSentences(d.tokens) {
		var words []string
		for _, t := range sentence {
			if !t.Kind.IsWhitespace() {
				words = append(words, t.GetStr(d.source))
			}
		}
		tags := tagger.TagSentence(words)
		flags := chunker.ChunkSentence(words, tags)
		ti := 0
		for i := range sentence {
			t := &sentence[i]
			if t.Kind.IsWord() {
				if m := dictionary.GetWordMetadata(t.GetCh(d.source)); m != nil {
					copy := *m
					copy.PosTag = tags[ti]
					if copy.PosTag == nil {
						if tag, ok := copy.InferPosTag(); ok {
							copy.PosTag = &tag
						}
					}
					copy.NpMember = &flags[ti]
					t.Kind.Metadata = &copy
				}
			}
			if !t.Kind.IsWhitespace() {
				ti++
			}
		}
	}
}
func (d *Document) remove(indices map[int]bool) {
	out := d.tokens[:0]
	for i, t := range d.tokens {
		if !indices[i] {
			out = append(out, t)
		}
	}
	d.tokens = out
}
func (d *Document) condense(start, end int, kind *TokenKind, removed map[int]bool) {
	d.tokens[start].Span.End = d.tokens[end-1].Span.End
	if kind != nil {
		d.tokens[start].Kind = *kind
	}
	for i := start + 1; i < end; i++ {
		removed[i] = true
	}
}
func (d *Document) condenseWhitespace(tag TokenKindTag) {
	remove := map[int]bool{}
	for cursor := 0; cursor < len(d.tokens); cursor++ {
		if d.tokens[cursor].Kind.Kind != tag {
			continue
		}
		start := cursor
		for {
			cursor++
			if cursor >= len(d.tokens) {
				break
			}
			child := d.tokens[cursor]
			if tag == Space && d.tokens[start].Span.End != child.Span.Start {
				break
			}
			if child.Kind.Kind != tag {
				break
			}
			d.tokens[start].Kind.Count += child.Kind.Count
			d.tokens[start].Span.End = child.Span.End
			remove[cursor] = true
			cursor++
		}
	}
	d.remove(remove)
}
func (d *Document) applyFixups() {
	d.condenseWhitespace(Space)
	d.condenseWhitespace(Newline)
	for i := range d.tokens {
		if d.tokens[i].Kind.IsNewline() && d.tokens[i].Kind.Count >= 2 {
			d.tokens[i].Kind = TokenKind{Kind: ParagraphBreak}
		}
	}
	d.condenseInitialisms()
	d.condenseNumbers()
	d.condenseEllipsis()
	d.condenseTruncations()
	d.condenseTLDs()
	d.condenseExtensions()
	d.condenseTldr()
	d.condensePairs(false)
	d.condensePairs(true)
	d.matchQuotes()
}
func (d *Document) condenseInitialisms() {
	if len(d.tokens) < 2 {
		return
	}
	remove := map[int]bool{}
	start := -1
	cursor := 1
	for {
		a, b := d.tokens[cursor-1], d.tokens[cursor]
		if a.Kind.IsWord() && a.Span.End-a.Span.Start == 1 && b.Kind.IsPeriod() {
			if start < 0 {
				start = cursor - 1
			} else {
				remove[cursor-1] = true
			}
			remove[cursor] = true
			cursor++
		} else {
			if start >= 0 {
				d.tokens[start].Span.End = d.tokens[cursor-2].Span.End
			}
			start = -1
		}
		cursor++
		if cursor >= len(d.tokens)-1 {
			break
		}
	}
	d.remove(remove)
}
func (d *Document) condenseNumbers() {
	remove := map[int]bool{}
	for i := 0; i < len(d.tokens)-1; i++ {
		if d.tokens[i].Kind.IsNumber() && d.tokens[i+1].Kind.IsWord() {
			if suffix, ok := text.OrdinalSuffixFromChars(d.tokens[i+1].GetCh(d.source)); ok {
				d.tokens[i].Kind.Number.Suffix = &suffix
				d.condense(i, i+2, nil, remove)
			}
		}
	}
	d.remove(remove)
}
func (d *Document) condenseEllipsis() {
	remove := map[int]bool{}
	for i := 0; i < len(d.tokens); {
		if !d.tokens[i].Kind.IsPeriod() {
			i++
			continue
		}
		j := i + 1
		for j < len(d.tokens) && d.tokens[j].Kind.IsPeriod() {
			j++
		}
		if j-i >= 2 {
			k := TokenKind{Kind: Punctuation, Punctuation: text.Punctuation{Kind: text.Ellipsis}}
			d.condense(i, j, &k, remove)
		}
		i = j
	}
	d.remove(remove)
}
func (d *Document) condenseTruncations() {
	remove := map[int]bool{}
	for i := 0; i < len(d.tokens); i++ {
		if !d.tokens[i].Kind.IsWord() {
			continue
		}
		w := d.tokens[i].GetCh(d.source)
		if i+1 < len(d.tokens) && (text.EqStr(w, "esp") || text.EqStr(w, "etc") || text.EqStr(w, "vs")) && d.tokens[i+1].Kind.IsPeriod() {
			d.condense(i, i+2, nil, remove)
			i++
		} else if text.EqStr(w, "et") && i+3 < len(d.tokens) && d.tokens[i+1].Kind.IsWhitespace() && d.tokens[i+2].Kind.IsWord() && text.EqStr(d.tokens[i+2].GetCh(d.source), "al") && d.tokens[i+3].Kind.IsPeriod() {
			d.condense(i, i+4, nil, remove)
			i += 3
		}
	}
	d.remove(remove)
}

var commonTLDs = strings.Fields("ai app blog co com dev edu gov info io me mil net org shop tech uk us xyz jp de fr br it ru es pl ca au cn in nl eu ch id at kr cz mx be tv se tr tw al ua ir vn cl sk ly cc to no fi pt dk ar hu tk gr il news ro my biz ie za nz sg ee th pe bg hk rs lt link ph club si site mobi by cat wiki la ga xxx cf hr ng jobs online kz ug gq ae is lv pro fm tips ms sa int")

func (d *Document) extensionContext(i int) bool {
	left, lok := d.GetToken(i - 2)
	right, rok := d.GetToken(i + 1)
	return (!lok || left.Kind.IsWhitespace()) && (!rok || right.Kind.IsWhitespace()) || lok && rok && left.Kind.IsOpenRound() && right.Kind.IsCloseRound()
}
func (d *Document) condenseTLDs() {
	remove := map[int]bool{}
	for i := 1; i < len(d.tokens); i++ {
		if !d.tokens[i-1].Kind.IsPeriod() || !d.tokens[i].Kind.IsWord() || !d.extensionContext(i) {
			continue
		}
		w := d.tokens[i].GetCh(d.source)
		for _, tld := range commonTLDs {
			if text.EqStr(w, tld) {
				k := TokenKind{Kind: Unlintable}
				d.condense(i-1, i+1, &k, remove)
				break
			}
		}
	}
	d.remove(remove)
}
func (d *Document) condenseExtensions() {
	if len(d.tokens) < 2 {
		return
	}
	remove := map[int]bool{}
	cursor, start := 1, -1
	for {
		a, b := d.tokens[cursor-1], d.tokens[cursor]
		w := b.GetCh(d.source)
		lower, upper := true, true
		for _, c := range w {
			lower = lower && c >= 'a' && c <= 'z'
			upper = upper && c >= 'A' && c <= 'Z'
		}
		if a.Kind.IsPeriod() && b.Kind.IsWord() && len(w) <= 3 && d.extensionContext(cursor) && (lower || upper) {
			if start < 0 {
				start = cursor - 1
				d.tokens[start].Kind = TokenKind{Kind: Unlintable}
			} else {
				remove[cursor-1] = true
			}
			remove[cursor] = true
			cursor++
		} else {
			if start >= 0 {
				d.tokens[start].Span.End = d.tokens[cursor-2].Span.End
			}
			start = -1
		}
		cursor++
		if cursor >= len(d.tokens) {
			break
		}
	}
	d.remove(remove)
}
func (d *Document) condenseTldr() {
	remove := map[int]bool{}
	for i := 2; i < len(d.tokens); i++ {
		a, b, c := d.tokens[i-2], d.tokens[i-1], d.tokens[i]
		if a.Kind.IsWord() && text.EqStr(a.GetCh(d.source), "tl") && b.Kind.IsSemicolon() && c.Kind.IsWord() && (text.EqStr(c.GetCh(d.source), "dr") || text.EqStr(c.GetCh(d.source), "drs")) {
			d.condense(i-2, i+1, nil, remove)
		}
	}
	d.remove(remove)
}
func (d *Document) condensePairs(slash bool) {
	pairs := "bb bw gt kr qa rb rd rr sp"
	if slash {
		pairs = "ac bw co dc dl io jk na rc sn yn yo"
	}
	remove := map[int]bool{}
	for i := 2; i < len(d.tokens); i++ {
		a, b, c := d.tokens[i-2], d.tokens[i-1], d.tokens[i]
		if !a.Kind.IsWord() || !c.Kind.IsWord() || a.Span.End-a.Span.Start != 1 || c.Span.End-c.Span.Start != 1 || !(slash && b.Kind.IsSlash() || !slash && b.Kind.IsAmpersand()) {
			continue
		}
		pair := strings.ToLower(a.GetStr(d.source) + c.GetStr(d.source))
		for _, valid := range strings.Fields(pairs) {
			if pair == valid {
				d.condense(i-2, i+1, nil, remove)
				break
			}
		}
	}
	d.remove(remove)
}
func (d *Document) matchQuotes() {
	indices := append([]int{0}, IterParagraphBreakIndices(d.tokens)...)
	indices = append(indices, len(d.tokens))
	for pi := 0; pi < len(indices)-1; pi++ {
		start, end := indices[pi], indices[pi+1]
		pg := d.tokens[start:end]
		quotes := IterQuoteIndices(pg)
		var opens []int
		for _, i := range quotes {
			_, wordBefore := firstMatching(pg[:i], TokenKind.IsWordLike)
			open := i == 0 || !wordBefore
			if i > 0 {
				k := pg[i-1].Kind
				open = open || k.IsWhitespace() || k.IsLessThan() || k.IsOpenRound() || k.IsOpenSquare() || k.isPunct(text.OpenCurly) || k.isPunct(text.EmDash) || k.isPunct(text.EnDash) || k.IsApostrophe() || k.IsOpenSingle()
			}
			if open {
				opens = append(opens, i)
			}
		}
		for oi := len(opens) - 1; oi >= 0; oi-- {
			o := opens[oi]
			for _, c := range quotes {
				if c <= o {
					continue
				}
				if pg[c].Kind.Punctuation.Quote.TwinLoc == nil {
					a, b := start+c, start+o
					pg[o].Kind.Punctuation.Quote.TwinLoc = &a
					pg[c].Kind.Punctuation.Quote.TwinLoc = &b
				}
				break
			}
		}
	}
}

func IsDocLikelyEnglish(d *Document, dictionary dict.Dictionary) bool {
	return IsLikelyEnglish(d.tokens, d.source, dictionary)
}
func IsLikelyEnglish(tokens []Token, source []rune, dictionary dict.Dictionary) bool {
	total, valid, punctuation, unlintable := 0, 0, 0, 0
	for _, t := range tokens {
		switch t.Kind.Kind {
		case Word:
			total++
			if dictionary.ContainsWord(t.GetCh(source)) {
				valid++
			}
		case Punctuation:
			punctuation++
		case Unlintable:
			unlintable++
		}
	}
	return !(total <= 7 && total-valid > 0 || unlintable > valid || float32(punctuation)*1.25 > float32(valid) || float64(valid)/float64(total) < 0.7)
}
func allWhitespace(chars []rune) bool {
	for _, c := range chars {
		if !unicode.IsSpace(c) {
			return false
		}
	}
	return true
}
