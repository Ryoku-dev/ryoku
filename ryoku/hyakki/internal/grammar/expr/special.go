package expr

import (
	"ryoku-rashin/internal/grammar/dict"
	"ryoku-rashin/internal/grammar/doc"
	"ryoku-rashin/internal/grammar/text"
	"sync"
)

// FixedPhrase preserves punctuation and token order, ignoring word case and number values.
type FixedPhrase struct{ inner *SequenceExpr }

func NewFixedPhraseFromPhrase(phrase string) *FixedPhrase {
	return NewFixedPhraseFromDocument(doc.NewBasicTokenize(phrase, doc.PlainEnglish{}))
}
func NewFixedPhraseFromDocument(d *doc.Document) *FixedPhrase {
	seq := NewSequenceExpr()
	for _, t := range d.GetTokens() {
		switch {
		case t.Kind.IsWord():
			seq.Then(NewWordFromChars(t.GetCh(d.GetSource())))
		case t.Kind.IsSpace() || t.Kind.IsParagraphBreak():
			seq.TWs()
		case t.Kind.IsPunctuation():
			p := t.Kind.Punctuation
			seq.ThenKind(func(k doc.TokenKind) bool { return k.IsPunctuation() && k.Punctuation == p })
		case t.Kind.IsNumber():
			seq.ThenNumber()
		default:
			panic("unsupported token in fixed phrase")
		}
	}
	return &FixedPhrase{seq}
}
func (e *FixedPhrase) Run(c int, t []doc.Token, s []rune) (text.Span, bool) {
	return e.inner.Run(c, t, s)
}

// SimilarToPhrase matches nearby spellings of a phrase, but rejects the exact phrase.
// Construction panics for token types other than words, spaces, and paragraph breaks.
type SimilarToPhrase struct{ phrase, fuzzyPhrase *SequenceExpr }

func NewSimilarToPhraseFromPhrase(phrase string, d uint8) *SimilarToPhrase {
	return NewSimilarToPhraseFromDoc(doc.NewPlainEnglishCurated(phrase), d)
}
func NewSimilarToPhraseFromDoc(document *doc.Document, d uint8) *SimilarToPhrase {
	exact, fuzzy := NewSequenceExpr(), NewSequenceExpr()
	for _, t := range document.GetTokens() {
		switch {
		case t.Kind.IsWord():
			chars := t.GetCh(document.GetSource())
			exact.Then(NewWordFromChars(chars))
			fuzzy.Then(NewWithinEditDistance(chars, d))
		case t.Kind.IsSpace() || t.Kind.IsParagraphBreak():
			exact.TWs()
			fuzzy.TWs()
		default:
			panic("unsupported token in similar phrase")
		}
	}
	return &SimilarToPhrase{exact, fuzzy}
}
func (e *SimilarToPhrase) Run(c int, t []doc.Token, s []rune) (text.Span, bool) {
	if _, ok := e.phrase.Run(c, t, s); ok {
		return text.Span{}, false
	}
	return e.fuzzyPhrase.Run(c, t, s)
}

type SpaceOrHyphen struct{}

func NewSpaceOrHyphen() *SpaceOrHyphen { return &SpaceOrHyphen{} }
func (SpaceOrHyphen) Run(c int, t []doc.Token, s []rune) (text.Span, bool) {
	if sp, ok := NewWhitespacePattern().Run(c, t, s); ok {
		return sp, true
	}
	return Predicate(func(t doc.Token, _ []rune) bool { return t.Kind.IsHyphen() }).Run(c, t, s)
}

type SpelledNumberExpr struct{}

func NewSpelledNumberExpr() *SpelledNumberExpr { return &SpelledNumberExpr{} }

var spelledNumber = sync.OnceValue(func() Expr {
	units := []string{"one", "two", "three", "four", "five", "six", "seven", "eight", "nine"}
	teens := []string{"ten", "eleven", "twelve", "thirteen", "fourteen", "fifteen", "sixteen", "seventeen", "eighteen", "nineteen"}
	tens := []string{"twenty", "thirty", "forty", "fifty", "sixty", "seventy", "eighty", "ninety"}
	words := NewWordSet(units, teens, tens, []string{"zero"})
	compound := NewSequenceExprWordSet(tens).TWsH().ThenWordSet(units)
	return NewLongestMatchOf([]Expr{words, compound})
})

func (SpelledNumberExpr) Run(c int, t []doc.Token, s []rune) (text.Span, bool) {
	return spelledNumber().Run(c, t, s)
}

type DurationExpr struct{}

func NewDurationExpr() *DurationExpr { return &DurationExpr{} }

var duration = sync.OnceValue(func() Expr {
	units := NewWordSet([]string{"second", "seconds", "minute", "minutes", "hour", "hours", "day", "days", "week", "weeks", "month", "months", "year", "years"})
	couple := NewSequenceExprAco("a").TWs().TAco("couple").ThenOptional(NewSequenceExprWhitespace().TAco("of"))
	few := NewSequenceExprOptional(NewSequenceExprAco("a").TWs()).TAco("few")
	return NewSequenceExprLongestOf([]Expr{NewSpelledNumberExpr(), NewSequenceExprNumber(), NewIndefiniteArticle(), couple, few, NewSequenceExpr().ThenQuantifier()}).TWs().Then(units)
})

func (DurationExpr) Run(c int, t []doc.Token, s []rune) (text.Span, bool) {
	return duration().Run(c, t, s)
}

// TimeUnitExpr includes singular, plural, possessive, and abbreviated time units.
type TimeUnitExpr struct{ pluralsOnly bool }

func NewTimeUnitExpr() *TimeUnitExpr            { return &TimeUnitExpr{} }
func NewTimeUnitExprPluralsOnly() *TimeUnitExpr { return &TimeUnitExpr{true} }

var timeUnits = sync.OnceValue(func() *WordSet {
	w := NewWordSet()
	for _, unit := range []string{"microsecond", "millisecond", "second", "minute", "hour", "day", "week", "month", "year", "decade", "moment", "night", "weekend"} {
		w.Add(unit)
		w.Add(unit + "s")
		w.Add(unit + "'s")
	}
	w.Add("ms")
	return w
})
var pluralTimeUnits = sync.OnceValue(func() *WordSet {
	w := NewWordSet()
	for _, unit := range []string{"microsecond", "millisecond", "second", "minute", "hour", "day", "week", "month", "year", "decade", "moment", "night", "weekend"} {
		w.Add(unit + "s")
	}
	return w
})

func (e TimeUnitExpr) Run(c int, t []doc.Token, s []rune) (text.Span, bool) {
	if e.pluralsOnly {
		return pluralTimeUnits().Run(c, t, s)
	}
	return timeUnits().Run(c, t, s)
}

type PronounBe struct{}

func NewPronounBe() *PronounBe { return &PronounBe{} }

var pronounBe = sync.OnceValue(func() Expr {
	return NewFirstMatchOf([]Expr{NewSequenceExpr().ThenSubjectPronoun().TWs().ThenWordSet([]string{"am", "are", "is", "was", "were"}), NewWordSet([]string{"i'm", "we're", "you're", "he's", "she's", "it's", "they're"})})
})

func (PronounBe) Run(c int, t []doc.Token, s []rune) (text.Span, bool) {
	return pronounBe().Run(c, t, s)
}

// ReflexivePronoun optionally includes common nonstandard forms such as "hisself".
type ReflexivePronoun struct{ includeCommonErrors bool }

func NewReflexivePronoun() *ReflexivePronoun                 { return NewReflexivePronounStandard() }
func NewReflexivePronounStandard() *ReflexivePronoun         { return &ReflexivePronoun{} }
func NewReflexivePronounWithCommonErrors() *ReflexivePronoun { return &ReflexivePronoun{true} }

var badReflexives = sync.OnceValue(func() *WordSet { return NewWordSet([]string{"hisself", "oneselves", "theirself", "theirselves"}) })

func (e ReflexivePronoun) Run(c int, t []doc.Token, s []rune) (text.Span, bool) {
	if sp, ok := Predicate(func(t doc.Token, _ []rune) bool { return t.Kind.IsReflexivePronoun() }).Run(c, t, s); ok {
		return sp, true
	}
	if e.includeCommonErrors {
		return badReflexives().Run(c, t, s)
	}
	return text.Span{}, false
}

// MergeableWords finds adjacent words whose closed and open dictionary forms satisfy a predicate.
// A preceding open compound prevents merging across that compound's boundary.
type MergeableWords struct {
	predicate func(*dict.DictWordMetadata, *dict.DictWordMetadata) bool
}

func NewMergeableWords(p func(*dict.DictWordMetadata, *dict.DictWordMetadata) bool) *MergeableWords {
	return &MergeableWords{p}
}

var mergeablePair = sync.OnceValue(func() Expr {
	return NewSequenceExprAnyWord().TWsH().ThenAnyWord()
})

func (e *MergeableWords) GetMergedWord(a, b doc.Token, s []rune) ([]rune, bool) {
	ac, bc := a.GetCh(s), b.GetCh(s)
	open := make([]rune, 0, len(ac)+len(bc)+1)
	open = append(open, ac...)
	open = append(open, ' ')
	open = append(open, bc...)
	dictionary := dict.Curated()
	metaOpen := dictionary.GetWordMetadata(open)
	closed := append(open[:len(ac)], bc...)
	metaClosed := dictionary.GetWordMetadata(closed)
	if e.predicate(metaClosed, metaOpen) {
		return closed, true
	}
	return nil, false
}
func (e *MergeableWords) Run(c int, t []doc.Token, s []rune) (text.Span, bool) {
	sp, ok := mergeablePair().Run(c, t, s)
	if !ok || sp.End-sp.Start != 3 {
		return text.Span{}, false
	}
	if c >= 2 && t[c-1].Kind.IsWhitespace() && t[c-2].Kind.IsWord() {
		compound := append([]rune(nil), t[c-2].GetCh(s)...)
		compound = append(compound, ' ')
		compound = append(compound, t[c].GetCh(s)...)
		if dict.Curated().GetWordMetadata(compound) != nil {
			return text.Span{}, false
		}
	}
	_, ok = e.GetMergedWord(t[c], t[c+2], s)
	return sp, ok
}
