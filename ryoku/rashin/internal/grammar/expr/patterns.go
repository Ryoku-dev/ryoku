package expr

import (
	"math"
	"ryoku-rashin/internal/grammar/dict"
	"ryoku-rashin/internal/grammar/doc"
	"ryoku-rashin/internal/grammar/pos"
	"ryoku-rashin/internal/grammar/text"
	"slices"
	"sync"
	"unicode"
)

func asciiLower(c rune) rune {
	if c >= 'A' && c <= 'Z' {
		return c + 32
	}
	return c
}
func sameWord(a, b []rune, normalize bool) bool {
	if len(a) != len(b) {
		return false
	}
	for i, c := range a {
		d := b[i]
		if normalize {
			c = text.NormalizedChar(c)
			d = text.NormalizedChar(d)
		}
		if asciiLower(c) != asciiLower(d) {
			return false
		}
	}
	return true
}

// Word matches a predefined word using ASCII-insensitive, exact, or standard case.
type Word struct {
	word []rune
	mode uint8
}

func NewWord(w string) *Word               { return &Word{word: []rune(w)} }
func NewWordFromChars(w []rune) *Word      { return &Word{word: append([]rune(nil), w...)} }
func NewWordFromCharString(w []rune) *Word { return &Word{word: w} }
func NewWordExact(w string) *Word          { return &Word{word: []rune(w), mode: 1} }
func NewWordStandardCase(w string) *Word   { return &Word{word: []rune(w), mode: 2} }
func (w *Word) MatchesToken(t doc.Token, s []rune) bool {
	if !t.Kind.IsWord() || t.Span.End-t.Span.Start != len(w.word) {
		return false
	}
	return w.matchesChars(t.GetCh(s))
}

func (w *Word) matchesChars(chars []rune) bool {
	if w.mode == 1 {
		return slices.Equal(chars, w.word)
	}
	if w.mode == 2 && !standardCase(chars) {
		return false
	}
	return sameWord(chars, w.word, false)
}
func standardCase(chars []rune) bool {
	if len(chars) == 0 {
		return false
	}
	if len(chars) == 1 {
		return true
	}
	rest := chars[1:]
	basis := isLowercase(chars[0])
	if !basis {
		basis = isLowercase(rest[0])
		rest = rest[1:]
	}
	for _, c := range rest {
		if isLowercase(c) != basis {
			return false
		}
	}
	return true
}

func isLowercase(c rune) bool {
	return unicode.IsLower(c) || unicode.In(c, unicode.Other_Lowercase)
}

// WordSet matches any contained word, normalizing punctuation and ASCII case.
// Contains only folds ASCII case; normalization belongs to insertion and matching.
type WordSet struct{ words [][]rune }

func NewWordSet(words ...[]string) *WordSet {
	s := &WordSet{}
	for _, list := range words {
		for _, w := range list {
			s.Add(w)
		}
	}
	return s
}
func (s *WordSet) Add(word string) { s.AddChars([]rune(word)) }
func (s *WordSet) AddChars(chars []rune) {
	if s.ContainsChars(chars) {
		return
	}
	w := make([]rune, len(chars))
	for i, c := range chars {
		w[i] = asciiLower(text.NormalizedChar(c))
	}
	s.words = append(s.words, w)
}
func (s *WordSet) Contains(word string) bool { return s.ContainsChars([]rune(word)) }
func (s *WordSet) ContainsChars(chars []rune) bool {
	for _, w := range s.words {
		if sameWord(chars, w, false) {
			return true
		}
	}
	return false
}
func (s *WordSet) MatchesToken(t doc.Token, src []rune) bool {
	if !t.Kind.IsWord() {
		return false
	}
	chars := t.GetCh(src)
	for _, word := range s.words {
		if sameWord(chars, word, true) {
			return true
		}
	}
	return false
}

type AnyPattern struct{}

func NewAnyPattern() *AnyPattern                       { return &AnyPattern{} }
func (AnyPattern) MatchesToken(doc.Token, []rune) bool { return true }

type WhitespacePattern struct{}

func NewWhitespacePattern() *WhitespacePattern { return &WhitespacePattern{} }
func (WhitespacePattern) Matches(t []doc.Token, _ []rune) (int, bool) {
	n := 0
	for _, x := range t {
		if !x.Kind.IsWhitespace() {
			break
		}
		n++
	}
	return n, n > 0
}

type IndefiniteArticle struct{ *WordSet }

func NewIndefiniteArticle() *IndefiniteArticle {
	return &IndefiniteArticle{NewWordSet([]string{"a", "an"})}
}

type InflectionOfBe struct{ *WordSet }

func NewInflectionOfBe() *InflectionOfBe {
	return &InflectionOfBe{NewWordSet([]string{"be", "am", "is", "are", "was", "were", "been", "being"})}
}

type RelativePronoun struct{ *WordSet }

func NewRelativePronoun() *RelativePronoun {
	return &RelativePronoun{NewWordSet([]string{"that", "which", "who", "whom", "whose"})}
}

type PrepositionalPrecederPattern struct{ *WordSet }

func NewPrepositionalPrecederPattern() *PrepositionalPrecederPattern {
	return &PrepositionalPrecederPattern{NewWordSet([]string{"accustomed", "addicted", "adjacent", "allergic", "attached", "attuned", "committed", "connected", "dedicated", "devoted", "immune", "oblivious", "opposed", "partial", "prone", "receptive", "related", "resistant", "sensitive", "subject", "susceptible", "used"})}
}

var prepositionalPreceder = sync.OnceValue(NewPrepositionalPrecederPattern)

func PrepositionalPreceder() *PrepositionalPrecederPattern { return prepositionalPreceder() }

type ModalVerb struct{ *WordSet }

var modals = []string{"can", "can't", "could", "may", "might", "must", "shall", "shan't", "should", "will", "won't", "would", "ought", "dare"}

func modalWords(errors bool) *WordSet {
	w := NewWordSet(modals)
	for _, m := range modals {
		w.Add(m + "n't")
		if errors {
			w.Add(m + "nt")
		}
	}
	w.Add("cannot")
	return w
}

var modalsStandard = sync.OnceValue(func() *WordSet { return modalWords(false) })
var modalsErrors = sync.OnceValue(func() *WordSet { return modalWords(true) })

func NewModalVerb() *ModalVerb                    { return NewModalVerbWithoutCommonErrors() }
func NewModalVerbWithoutCommonErrors() *ModalVerb { return &ModalVerb{modalsStandard()} }
func NewModalVerbWithCommonErrors() *ModalVerb    { return &ModalVerb{modalsErrors()} }

type UPOSSet struct{ AllowedTags []pos.UPOS }

func NewUPOSSet(allowed []pos.UPOS) *UPOSSet { return &UPOSSet{append([]pos.UPOS(nil), allowed...)} }
func (p *UPOSSet) Matches(t []doc.Token, _ []rune) (int, bool) {
	if len(t) == 0 || !t[0].Kind.IsWord() || t[0].Kind.Metadata == nil || t[0].Kind.Metadata.PosTag == nil {
		return 0, false
	}
	for _, tag := range p.AllowedTags {
		if tag == *t[0].Kind.Metadata.PosTag {
			return 1, true
		}
	}
	return 0, false
}

// NominalPhrase uses context-free heuristics. Prefer metadata.NpMember for new rules.
type NominalPhrase struct{}

func NewNominalPhrase() *NominalPhrase { return &NominalPhrase{} }
func (NominalPhrase) Matches(t []doc.Token, _ []rune) (int, bool) {
	for c := 0; c < len(t); {
		k := t[c].Kind
		if (k.IsAdjective() || k.IsDeterminer() || k.IsVerbProgressiveForm()) && c+1 < len(t) && t[c+1].Kind.IsWhitespace() {
			c += 2
			continue
		}
		return c + 1, k.IsNominal()
	}
	return 0, false
}

type ImpliesQuantity struct{}

func NewImpliesQuantity() *ImpliesQuantity { return &ImpliesQuantity{} }
func ImpliesPlurality(t doc.Token, s []rune) (bool, bool) {
	k := t.Kind
	if k.IsWord() && k.Metadata != nil {
		if k.Metadata.IsDeterminer() {
			return false, true
		}
		switch string(t.GetCh(s)) {
		case "a", "an":
			return false, true
		case "many":
			return true, true
		}
	} else if k.IsNumber() {
		return math.Abs(math.Abs(k.Number.Value)-1) > 2.220446049250313e-16, true
	}
	return false, false
}
func (ImpliesQuantity) ImpliesPlurality(t doc.Token, s []rune) (bool, bool) {
	return ImpliesPlurality(t, s)
}
func (ImpliesQuantity) MatchesToken(t doc.Token, s []rune) bool {
	_, ok := ImpliesPlurality(t, s)
	return ok
}

// DerivedFrom matches a word or any word whose dictionary metadata derives from it.
type DerivedFrom struct{ wordID dict.CanonicalWordId }

func NewDerivedFrom(id dict.CanonicalWordId) *DerivedFrom { return &DerivedFrom{id} }
func NewDerivedFromStr(w string) *DerivedFrom {
	return NewDerivedFrom(dict.NewCanonicalWordIdFromWordStr(w))
}
func NewDerivedFromChars(w []rune) *DerivedFrom {
	return NewDerivedFrom(dict.NewCanonicalWordIdFromWordChars(w))
}
func (p *DerivedFrom) Matches(t []doc.Token, s []rune) (int, bool) {
	if len(t) == 0 {
		return 0, false
	}
	tok := t[0]
	chars := tok.GetCh(s)
	folded, _ := dict.NewCaseFoldedWordIdFromWordChars(chars)
	exact := dict.NewCanonicalWordIdFromWordChars(chars) == p.wordID || folded == p.wordID.AsCaseFolded()
	if exact || (tok.Kind.IsWord() && tok.Kind.Metadata != nil && tok.Kind.Metadata.DerivedFrom.Contains(p.wordID)) {
		return 1, true
	}
	return 0, false
}

type Invert struct{ inner Pattern }

func NewInvert(p Pattern) *Invert { return &Invert{p} }
func (p *Invert) Matches(t []doc.Token, s []rune) (int, bool) {
	_, ok := p.inner.Matches(t, s)
	return 1, !ok
}

// WithinEditDistance matches words within a Unicode-lowercased Levenshtein distance.
type WithinEditDistance struct {
	word        []rune
	maxDistance uint8
}

func NewWithinEditDistance(w []rune, d uint8) *WithinEditDistance {
	return &WithinEditDistance{text.ToLower(w), d}
}
func NewWithinEditDistanceFromStr(w string, d uint8) *WithinEditDistance {
	return NewWithinEditDistance([]rune(w), d)
}

var distanceBuffers = sync.Pool{New: func() any { return &[2][]uint8{} }}

func (p *WithinEditDistance) MatchesToken(t doc.Token, s []rune) bool {
	if !t.Kind.IsWord() {
		return false
	}
	buf := distanceBuffers.Get().(*[2][]uint8)
	distance := text.EditDistanceMinAlloc(text.ToLower(t.GetCh(s)), p.word, &buf[0], &buf[1])
	distanceBuffers.Put(buf)
	return distance <= p.maxDistance
}
func (p *Word) Matches(t []doc.Token, s []rune) (int, bool)           { return singleMatches(p, t, s) }
func (p *WordSet) Matches(t []doc.Token, s []rune) (int, bool)        { return singleMatches(p, t, s) }
func (p AnyPattern) Matches(t []doc.Token, s []rune) (int, bool)      { return singleMatches(p, t, s) }
func (p ImpliesQuantity) Matches(t []doc.Token, s []rune) (int, bool) { return singleMatches(p, t, s) }
func (p *WithinEditDistance) Matches(t []doc.Token, s []rune) (int, bool) {
	return singleMatches(p, t, s)
}
func (p *Word) Step(t []doc.Token, c int, s []rune) (int, bool) {
	if c >= 0 && c < len(t) && p.MatchesToken(t[c], s) {
		return 1, true
	}
	return 0, false
}
func (p *Word) Run(c int, t []doc.Token, s []rune) (text.Span, bool) {
	// Keep the common word path concrete: the blanket conversions would otherwise
	// dispatch through Expr, Step, Pattern, and SingleTokenPattern for each attempt.
	if c < 0 || c >= len(t) {
		return text.Span{}, false
	}
	token := &t[c]
	if token.Kind.Kind != doc.Word || token.Span.End-token.Span.Start != len(p.word) {
		return text.Span{}, false
	}
	if p.matchesChars(s[token.Span.Start:token.Span.End]) {
		return text.Span{Start: c, End: c + 1}, true
	}
	return text.Span{}, false
}
func (p *WordSet) Step(t []doc.Token, c int, s []rune) (int, bool) {
	if c >= 0 && c < len(t) && p.MatchesToken(t[c], s) {
		return 1, true
	}
	return 0, false
}
func (p *WordSet) Run(c int, t []doc.Token, s []rune) (text.Span, bool) {
	if c >= 0 && c < len(t) && p.MatchesToken(t[c], s) {
		return text.Span{Start: c, End: c + 1}, true
	}
	return text.Span{}, false
}
func (p AnyPattern) Step(t []doc.Token, c int, s []rune) (int, bool) {
	if c >= 0 && c < len(t) {
		return 1, true
	}
	return 0, false
}
func (p AnyPattern) Run(c int, t []doc.Token, s []rune) (text.Span, bool) {
	if c >= 0 && c < len(t) {
		return text.Span{Start: c, End: c + 1}, true
	}
	return text.Span{}, false
}
func (p WhitespacePattern) Step(t []doc.Token, c int, s []rune) (int, bool) {
	if c < 0 || c > len(t) {
		return 0, false
	}
	return p.Matches(t[c:], s)
}
func (p WhitespacePattern) Run(c int, t []doc.Token, s []rune) (text.Span, bool) {
	n, ok := p.Step(t, c, s)
	if !ok {
		return text.Span{}, false
	}
	return text.Span{Start: c, End: c + n}, true
}
func (p *UPOSSet) Step(t []doc.Token, c int, s []rune) (int, bool) {
	if c < 0 || c > len(t) {
		return 0, false
	}
	return p.Matches(t[c:], s)
}
func (p *UPOSSet) Run(c int, t []doc.Token, s []rune) (text.Span, bool) {
	n, ok := p.Step(t, c, s)
	if !ok {
		return text.Span{}, false
	}
	return text.Span{Start: c, End: c + n}, true
}
func (p NominalPhrase) Step(t []doc.Token, c int, s []rune) (int, bool) {
	return patternStep(p, t, c, s)
}
func (p NominalPhrase) Run(c int, t []doc.Token, s []rune) (text.Span, bool) {
	return stepRun(p, c, t, s)
}
func (p ImpliesQuantity) Step(t []doc.Token, c int, s []rune) (int, bool) {
	return patternStep(p, t, c, s)
}
func (p ImpliesQuantity) Run(c int, t []doc.Token, s []rune) (text.Span, bool) {
	return stepRun(p, c, t, s)
}
func (p *DerivedFrom) Step(t []doc.Token, c int, s []rune) (int, bool) {
	return patternStep(p, t, c, s)
}
func (p *DerivedFrom) Run(c int, t []doc.Token, s []rune) (text.Span, bool) {
	return stepRun(p, c, t, s)
}
func (p *Invert) Step(t []doc.Token, c int, s []rune) (int, bool)      { return patternStep(p, t, c, s) }
func (p *Invert) Run(c int, t []doc.Token, s []rune) (text.Span, bool) { return stepRun(p, c, t, s) }
func (p *WithinEditDistance) Step(t []doc.Token, c int, s []rune) (int, bool) {
	return patternStep(p, t, c, s)
}
func (p *WithinEditDistance) Run(c int, t []doc.Token, s []rune) (text.Span, bool) {
	return stepRun(p, c, t, s)
}
