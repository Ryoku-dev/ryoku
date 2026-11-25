package expr

import (
	"ryoku-rashin/internal/grammar/doc"
	"ryoku-rashin/internal/grammar/text"
)

// SequenceExpr requires every child expression, optionally capturing only a selected subrange.
type SequenceExpr struct {
	exprs                    []Expr
	captureStart, captureEnd int
	ended                    bool
}

func NewSequenceExpr() *SequenceExpr { return &SequenceExpr{} }
func (s *SequenceExpr) Run(c int, t []doc.Token, src []rune) (text.Span, bool) {
	window := text.Span{Start: c, End: c}
	for i, e := range s.exprs {
		sp, ok := e.Run(c, t, src)
		if !ok {
			return text.Span{}, false
		}
		if sp.Start == sp.End {
			continue
		}
		if sp.End > c {
			c = sp.End
		} else if sp.Start < c {
			c = sp.Start
		}
		if s.captureStart == i {
			window = text.Span{Start: c, End: c}
		} else if i < s.captureStart || (s.ended && i >= s.captureEnd) {
			continue
		}
		window.Start = min(window.Start, sp.Start)
		window.End = max(window.End, sp.End)
	}
	return window, true
}
func (s *SequenceExpr) Then(e any) *SequenceExpr              { s.exprs = append(s.exprs, AsExpr(e)); return s }
func (s *SequenceExpr) ThenBoxed(e Expr) *SequenceExpr        { return s.Then(e) }
func (s *SequenceExpr) ThenOptional(e any) *SequenceExpr      { return s.Then(NewOptional(e)) }
func (s *SequenceExpr) ThenAnyOf(es []Expr) *SequenceExpr     { return s.Then(NewFirstMatchOf(es)) }
func (s *SequenceExpr) ThenLongestOf(es []Expr) *SequenceExpr { return s.Then(NewLongestMatchOf(es)) }
func (s *SequenceExpr) ThenSeq(other *SequenceExpr) *SequenceExpr {
	s.exprs = append(s.exprs, other.exprs...)
	return s
}
func (s *SequenceExpr) ThenWordSet(words []string) *SequenceExpr { return s.Then(NewWordSet(words)) }
func (s *SequenceExpr) TSet(words []string) *SequenceExpr        { return s.ThenWordSet(words) }
func (s *SequenceExpr) ThenWhitespace() *SequenceExpr            { return s.Then(NewWhitespacePattern()) }
func (s *SequenceExpr) TWs() *SequenceExpr                       { return s.ThenWhitespace() }
func (s *SequenceExpr) ThenWhitespaceOrHyphen() *SequenceExpr    { return s.Then(NewSpaceOrHyphen()) }
func (s *SequenceExpr) TWsH() *SequenceExpr                      { return s.ThenWhitespaceOrHyphen() }
func (s *SequenceExpr) ThenOptionalWhitespace() *SequenceExpr {
	return s.ThenOptional(NewWhitespacePattern())
}
func (s *SequenceExpr) TOws() *SequenceExpr                { return s.ThenOptionalWhitespace() }
func (s *SequenceExpr) ThenZeroOrMore(e any) *SequenceExpr { return s.Then(NewRepeating(e, 0)) }
func (s *SequenceExpr) ThenOneOrMore(e any) *SequenceExpr  { return s.Then(NewRepeating(e, 1)) }

// ThenZeroOrMoreSpaced preserves Harper's implementation: one initial match followed by
// zero or more whitespace-separated matches, despite the method's zero-or-more name.
func (s *SequenceExpr) ThenZeroOrMoreSpaced(e any) *SequenceExpr {
	x := AsExpr(e)
	return s.Then(NewSequenceExpr().Then(x).ThenZeroOrMore(NewSequenceExpr().TWs().Then(x)))
}
func (s *SequenceExpr) ThenUnless(e any) *SequenceExpr {
	return s.Then(NewUnlessStep(AsExpr(e), NewAnyPattern()))
}
func (s *SequenceExpr) ThenAnything() *SequenceExpr { return s.Then(NewAnyPattern()) }
func (s *SequenceExpr) TAny() *SequenceExpr         { return s.ThenAnything() }
func (s *SequenceExpr) ThenAnyWord() *SequenceExpr  { return s.ThenKind(doc.TokenKind.IsWord) }
func (s *SequenceExpr) ThenAnyCapitalizationOf(word string) *SequenceExpr {
	return s.Then(NewWord(word))
}
func (s *SequenceExpr) TAco(word string) *SequenceExpr          { return s.ThenAnyCapitalizationOf(word) }
func (s *SequenceExpr) ThenExactWord(word string) *SequenceExpr { return s.Then(NewWordExact(word)) }
func (s *SequenceExpr) ThenWordSeq(words []string) *SequenceExpr {
	for i, w := range words {
		if i > 0 {
			s.TWs()
		}
		s.TAco(w)
	}
	return s
}
func (s *SequenceExpr) ThenFixedPhrase(phrase string) *SequenceExpr {
	return s.Then(NewFixedPhraseFromPhrase(phrase))
}

// ThenWordExcept rejects listed words case-insensitively; non-word tokens pass.
func (s *SequenceExpr) ThenWordExcept(words []string) *SequenceExpr {
	return s.Then(Predicate(func(t doc.Token, src []rune) bool { return !t.Kind.IsWord() || !wordIn(t, src, words) }))
}

type KindPredicate = func(doc.TokenKind) bool

func wordIn(t doc.Token, src []rune, words []string) bool {
	chars := t.GetCh(src)
	for _, w := range words {
		if text.EqStr(chars, w) {
			return true
		}
	}
	return false
}
func anyKind(k doc.TokenKind, ps []KindPredicate) bool {
	for _, p := range ps {
		if p(k) {
			return true
		}
	}
	return false
}

// ThenKind matches a token-kind predicate. Use ThenExactKind to compare an entire kind value.
func (s *SequenceExpr) ThenKind(p KindPredicate) *SequenceExpr {
	return s.Then(Predicate(func(t doc.Token, _ []rune) bool { return p(t.Kind) }))
}
func (s *SequenceExpr) ThenKindWhere(p KindPredicate) *SequenceExpr { return s.ThenKind(p) }
func (s *SequenceExpr) ThenExactKind(k doc.TokenKind) *SequenceExpr {
	return s.ThenKind(func(t doc.TokenKind) bool { return tokenKindsEqual(t, k) })
}

func tokenKindsEqual(a, b doc.TokenKind) bool {
	if a.Kind != b.Kind {
		return false
	}
	switch a.Kind {
	case doc.Word:
		if a.Metadata == nil || b.Metadata == nil {
			return a.Metadata == b.Metadata
		}
		return a.Metadata == b.Metadata || a.Metadata.Equal(*b.Metadata)
	case doc.Punctuation:
		return a.Punctuation == b.Punctuation
	case doc.Number:
		x, y := a.Number, b.Number
		if x.Value != y.Value || x.Radix != y.Radix || x.Precision != y.Precision {
			return false
		}
		if x.Suffix == nil || y.Suffix == nil {
			return x.Suffix == y.Suffix
		}
		return *x.Suffix == *y.Suffix
	case doc.Space, doc.Newline:
		return a.Count == b.Count
	default:
		return true
	}
}
func (s *SequenceExpr) ThenKindExcept(p KindPredicate, ex []string) *SequenceExpr {
	return s.Then(Predicate(func(t doc.Token, src []rune) bool { return p(t.Kind) && !wordIn(t, src, ex) }))
}
func (s *SequenceExpr) ThenKindBoth(a, b KindPredicate) *SequenceExpr {
	return s.ThenKind(func(k doc.TokenKind) bool { return a(k) && b(k) })
}
func (s *SequenceExpr) ThenKindEither(a, b KindPredicate) *SequenceExpr {
	return s.ThenKind(func(k doc.TokenKind) bool { return a(k) || b(k) })
}
func (s *SequenceExpr) ThenKindNeither(a, b KindPredicate) *SequenceExpr {
	return s.ThenKind(func(k doc.TokenKind) bool { return !a(k) && !b(k) })
}
func (s *SequenceExpr) ThenKindIsButIsNot(a, b KindPredicate) *SequenceExpr {
	return s.ThenKind(func(k doc.TokenKind) bool { return a(k) && !b(k) })
}
func (s *SequenceExpr) ThenKindIsButIsNotExcept(a, b KindPredicate, ex []string) *SequenceExpr {
	return s.ThenKindExcept(func(k doc.TokenKind) bool { return a(k) && !b(k) }, ex)
}
func (s *SequenceExpr) ThenKindIsButIsntAnyOf(a KindPredicate, bs []KindPredicate) *SequenceExpr {
	return s.ThenKind(func(k doc.TokenKind) bool { return a(k) && !anyKind(k, bs) })
}
func (s *SequenceExpr) ThenKindIsButIsntAnyOfExcept(a KindPredicate, bs []KindPredicate, ex []string) *SequenceExpr {
	return s.ThenKindExcept(func(k doc.TokenKind) bool { return a(k) && !anyKind(k, bs) }, ex)
}
func (s *SequenceExpr) ThenKindBothButNot(ab [2]KindPredicate, n KindPredicate) *SequenceExpr {
	return s.ThenKind(func(k doc.TokenKind) bool { return ab[0](k) && ab[1](k) && !n(k) })
}
func (s *SequenceExpr) ThenKindAny(ps []KindPredicate) *SequenceExpr {
	return s.ThenKind(func(k doc.TokenKind) bool { return anyKind(k, ps) })
}
func (s *SequenceExpr) ThenKindNoneOf(ps []KindPredicate) *SequenceExpr {
	return s.ThenKind(func(k doc.TokenKind) bool { return !anyKind(k, ps) })
}
func (s *SequenceExpr) ThenKindAnyExcept(ps []KindPredicate, ex []string) *SequenceExpr {
	return s.ThenKindExcept(func(k doc.TokenKind) bool { return anyKind(k, ps) }, ex)
}
func (s *SequenceExpr) ThenKindAnyOrWords(ps []KindPredicate, words []string) *SequenceExpr {
	return s.Then(Predicate(func(t doc.Token, src []rune) bool { return anyKind(t.Kind, ps) || wordIn(t, src, words) }))
}
func (s *SequenceExpr) ThenKindAnyButNot(ps []KindPredicate, n KindPredicate) *SequenceExpr {
	return s.ThenKind(func(k doc.TokenKind) bool { return anyKind(k, ps) && !n(k) })
}
func (s *SequenceExpr) ThenKindAnyButNotExcept(ps []KindPredicate, n KindPredicate, ex []string) *SequenceExpr {
	return s.ThenKindExcept(func(k doc.TokenKind) bool { return anyKind(k, ps) && !n(k) }, ex)
}
func (s *SequenceExpr) ThenRelativePronoun() *SequenceExpr   { return s.Then(NewRelativePronoun()) }
func (s *SequenceExpr) ThenIndefiniteArticle() *SequenceExpr { return s.Then(NewIndefiniteArticle()) }

// StartCapture leaves earlier children required but excludes them from the returned window.
func (s *SequenceExpr) StartCapture() *SequenceExpr { s.captureStart = len(s.exprs); return s }

// EndCapture leaves subsequent children required but excludes them from the returned window.
func (s *SequenceExpr) EndCapture() *SequenceExpr {
	s.captureEnd = len(s.exprs)
	s.ended = true
	return s
}
func (s *SequenceExpr) ThenOov() *SequenceExpr { return s.ThenKind(doc.TokenKind.IsOov) }
func (s *SequenceExpr) ThenOptionalOov() *SequenceExpr {
	return s.ThenOptional(Predicate(func(t doc.Token, _ []rune) bool { return t.Kind.IsOov() }))
}
func (s *SequenceExpr) ThenOneOrMoreOovs() *SequenceExpr {
	return s.ThenOneOrMore(Predicate(func(t doc.Token, _ []rune) bool { return t.Kind.IsOov() }))
}
func (s *SequenceExpr) ThenAnythingButOov() *SequenceExpr {
	return s.ThenKind(func(k doc.TokenKind) bool { return !k.IsOov() })
}
func (s *SequenceExpr) ThenSwear() *SequenceExpr { return s.ThenKind(doc.TokenKind.IsSwear) }
func (s *SequenceExpr) ThenOptionalSwear() *SequenceExpr {
	return s.ThenOptional(Predicate(func(t doc.Token, _ []rune) bool { return t.Kind.IsSwear() }))
}
func (s *SequenceExpr) ThenOneOrMoreSwears() *SequenceExpr {
	return s.ThenOneOrMore(Predicate(func(t doc.Token, _ []rune) bool { return t.Kind.IsSwear() }))
}
func (s *SequenceExpr) ThenAnythingButSwear() *SequenceExpr {
	return s.ThenKind(func(k doc.TokenKind) bool { return !k.IsSwear() })
}
func (s *SequenceExpr) ThenNominal() *SequenceExpr { return s.ThenKind(doc.TokenKind.IsNominal) }
func (s *SequenceExpr) ThenOptionalNominal() *SequenceExpr {
	return s.ThenOptional(Predicate(func(t doc.Token, _ []rune) bool { return t.Kind.IsNominal() }))
}
func (s *SequenceExpr) ThenOneOrMoreNominals() *SequenceExpr {
	return s.ThenOneOrMore(Predicate(func(t doc.Token, _ []rune) bool { return t.Kind.IsNominal() }))
}
func (s *SequenceExpr) ThenAnythingButNominal() *SequenceExpr {
	return s.ThenKind(func(k doc.TokenKind) bool { return !k.IsNominal() })
}
func (s *SequenceExpr) ThenPluralNominal() *SequenceExpr {
	return s.ThenKind(doc.TokenKind.IsPluralNominal)
}
func (s *SequenceExpr) ThenOptionalPluralNominal() *SequenceExpr {
	return s.ThenOptional(Predicate(func(t doc.Token, _ []rune) bool { return t.Kind.IsPluralNominal() }))
}
func (s *SequenceExpr) ThenOneOrMorePluralNominals() *SequenceExpr {
	return s.ThenOneOrMore(Predicate(func(t doc.Token, _ []rune) bool { return t.Kind.IsPluralNominal() }))
}
func (s *SequenceExpr) ThenAnythingButPluralNominal() *SequenceExpr {
	return s.ThenKind(func(k doc.TokenKind) bool { return !k.IsPluralNominal() })
}
func (s *SequenceExpr) ThenNonPluralNominal() *SequenceExpr {
	return s.ThenKind(doc.TokenKind.IsNonPluralNominal)
}
func (s *SequenceExpr) ThenOptionalNonPluralNominal() *SequenceExpr {
	return s.ThenOptional(Predicate(func(t doc.Token, _ []rune) bool { return t.Kind.IsNonPluralNominal() }))
}
func (s *SequenceExpr) ThenOneOrMoreNonPluralNominals() *SequenceExpr {
	return s.ThenOneOrMore(Predicate(func(t doc.Token, _ []rune) bool { return t.Kind.IsNonPluralNominal() }))
}
func (s *SequenceExpr) ThenAnythingButNonPluralNominal() *SequenceExpr {
	return s.ThenKind(func(k doc.TokenKind) bool { return !k.IsNonPluralNominal() })
}
func (s *SequenceExpr) ThenPossessiveNominal() *SequenceExpr {
	return s.ThenKind(doc.TokenKind.IsPossessiveNominal)
}
func (s *SequenceExpr) ThenOptionalPossessiveNominal() *SequenceExpr {
	return s.ThenOptional(Predicate(func(t doc.Token, _ []rune) bool { return t.Kind.IsPossessiveNominal() }))
}
func (s *SequenceExpr) ThenOneOrMorePossessiveNominals() *SequenceExpr {
	return s.ThenOneOrMore(Predicate(func(t doc.Token, _ []rune) bool { return t.Kind.IsPossessiveNominal() }))
}
func (s *SequenceExpr) ThenAnythingButPossessiveNominal() *SequenceExpr {
	return s.ThenKind(func(k doc.TokenKind) bool { return !k.IsPossessiveNominal() })
}
func (s *SequenceExpr) ThenNoun() *SequenceExpr { return s.ThenKind(doc.TokenKind.IsNoun) }
func (s *SequenceExpr) ThenOptionalNoun() *SequenceExpr {
	return s.ThenOptional(Predicate(func(t doc.Token, _ []rune) bool { return t.Kind.IsNoun() }))
}
func (s *SequenceExpr) ThenOneOrMoreNouns() *SequenceExpr {
	return s.ThenOneOrMore(Predicate(func(t doc.Token, _ []rune) bool { return t.Kind.IsNoun() }))
}
func (s *SequenceExpr) ThenAnythingButNoun() *SequenceExpr {
	return s.ThenKind(func(k doc.TokenKind) bool { return !k.IsNoun() })
}
func (s *SequenceExpr) ThenProperNoun() *SequenceExpr { return s.ThenKind(doc.TokenKind.IsProperNoun) }
func (s *SequenceExpr) ThenOptionalProperNoun() *SequenceExpr {
	return s.ThenOptional(Predicate(func(t doc.Token, _ []rune) bool { return t.Kind.IsProperNoun() }))
}
func (s *SequenceExpr) ThenOneOrMoreProperNouns() *SequenceExpr {
	return s.ThenOneOrMore(Predicate(func(t doc.Token, _ []rune) bool { return t.Kind.IsProperNoun() }))
}
func (s *SequenceExpr) ThenAnythingButProperNoun() *SequenceExpr {
	return s.ThenKind(func(k doc.TokenKind) bool { return !k.IsProperNoun() })
}
func (s *SequenceExpr) ThenSingularNoun() *SequenceExpr {
	return s.ThenKind(doc.TokenKind.IsSingularNoun)
}
func (s *SequenceExpr) ThenOptionalSingularNoun() *SequenceExpr {
	return s.ThenOptional(Predicate(func(t doc.Token, _ []rune) bool { return t.Kind.IsSingularNoun() }))
}
func (s *SequenceExpr) ThenOneOrMoreSingularNouns() *SequenceExpr {
	return s.ThenOneOrMore(Predicate(func(t doc.Token, _ []rune) bool { return t.Kind.IsSingularNoun() }))
}
func (s *SequenceExpr) ThenAnythingButSingularNoun() *SequenceExpr {
	return s.ThenKind(func(k doc.TokenKind) bool { return !k.IsSingularNoun() })
}
func (s *SequenceExpr) ThenPluralNoun() *SequenceExpr { return s.ThenKind(doc.TokenKind.IsPluralNoun) }
func (s *SequenceExpr) ThenOptionalPluralNoun() *SequenceExpr {
	return s.ThenOptional(Predicate(func(t doc.Token, _ []rune) bool { return t.Kind.IsPluralNoun() }))
}
func (s *SequenceExpr) ThenOneOrMorePluralNouns() *SequenceExpr {
	return s.ThenOneOrMore(Predicate(func(t doc.Token, _ []rune) bool { return t.Kind.IsPluralNoun() }))
}
func (s *SequenceExpr) ThenAnythingButPluralNoun() *SequenceExpr {
	return s.ThenKind(func(k doc.TokenKind) bool { return !k.IsPluralNoun() })
}
func (s *SequenceExpr) ThenSingularNounOnly() *SequenceExpr {
	return s.ThenKind(doc.TokenKind.IsSingularNounOnly)
}
func (s *SequenceExpr) ThenOptionalSingularNounOnly() *SequenceExpr {
	return s.ThenOptional(Predicate(func(t doc.Token, _ []rune) bool { return t.Kind.IsSingularNounOnly() }))
}
func (s *SequenceExpr) ThenOneOrMoreSingularNounOnlys() *SequenceExpr {
	return s.ThenOneOrMore(Predicate(func(t doc.Token, _ []rune) bool { return t.Kind.IsSingularNounOnly() }))
}
func (s *SequenceExpr) ThenAnythingButSingularNounOnly() *SequenceExpr {
	return s.ThenKind(func(k doc.TokenKind) bool { return !k.IsSingularNounOnly() })
}
func (s *SequenceExpr) ThenPluralNounOnly() *SequenceExpr {
	return s.ThenKind(doc.TokenKind.IsPluralNounOnly)
}
func (s *SequenceExpr) ThenOptionalPluralNounOnly() *SequenceExpr {
	return s.ThenOptional(Predicate(func(t doc.Token, _ []rune) bool { return t.Kind.IsPluralNounOnly() }))
}
func (s *SequenceExpr) ThenOneOrMorePluralNounOnlys() *SequenceExpr {
	return s.ThenOneOrMore(Predicate(func(t doc.Token, _ []rune) bool { return t.Kind.IsPluralNounOnly() }))
}
func (s *SequenceExpr) ThenAnythingButPluralNounOnly() *SequenceExpr {
	return s.ThenKind(func(k doc.TokenKind) bool { return !k.IsPluralNounOnly() })
}
func (s *SequenceExpr) ThenMassNounOnly() *SequenceExpr {
	return s.ThenKind(doc.TokenKind.IsMassNounOnly)
}
func (s *SequenceExpr) ThenOptionalMassNounOnly() *SequenceExpr {
	return s.ThenOptional(Predicate(func(t doc.Token, _ []rune) bool { return t.Kind.IsMassNounOnly() }))
}
func (s *SequenceExpr) ThenOneOrMoreMassNounOnlys() *SequenceExpr {
	return s.ThenOneOrMore(Predicate(func(t doc.Token, _ []rune) bool { return t.Kind.IsMassNounOnly() }))
}
func (s *SequenceExpr) ThenAnythingButMassNounOnly() *SequenceExpr {
	return s.ThenKind(func(k doc.TokenKind) bool { return !k.IsMassNounOnly() })
}
func (s *SequenceExpr) ThenPronoun() *SequenceExpr { return s.ThenKind(doc.TokenKind.IsPronoun) }
func (s *SequenceExpr) ThenOptionalPronoun() *SequenceExpr {
	return s.ThenOptional(Predicate(func(t doc.Token, _ []rune) bool { return t.Kind.IsPronoun() }))
}
func (s *SequenceExpr) ThenOneOrMorePronouns() *SequenceExpr {
	return s.ThenOneOrMore(Predicate(func(t doc.Token, _ []rune) bool { return t.Kind.IsPronoun() }))
}
func (s *SequenceExpr) ThenAnythingButPronoun() *SequenceExpr {
	return s.ThenKind(func(k doc.TokenKind) bool { return !k.IsPronoun() })
}
func (s *SequenceExpr) ThenPersonalPronoun() *SequenceExpr {
	return s.ThenKind(doc.TokenKind.IsPersonalPronoun)
}
func (s *SequenceExpr) ThenOptionalPersonalPronoun() *SequenceExpr {
	return s.ThenOptional(Predicate(func(t doc.Token, _ []rune) bool { return t.Kind.IsPersonalPronoun() }))
}
func (s *SequenceExpr) ThenOneOrMorePersonalPronouns() *SequenceExpr {
	return s.ThenOneOrMore(Predicate(func(t doc.Token, _ []rune) bool { return t.Kind.IsPersonalPronoun() }))
}
func (s *SequenceExpr) ThenAnythingButPersonalPronoun() *SequenceExpr {
	return s.ThenKind(func(k doc.TokenKind) bool { return !k.IsPersonalPronoun() })
}
func (s *SequenceExpr) ThenFirstPersonSingularPronoun() *SequenceExpr {
	return s.ThenKind(doc.TokenKind.IsFirstPersonSingularPronoun)
}
func (s *SequenceExpr) ThenOptionalFirstPersonSingularPronoun() *SequenceExpr {
	return s.ThenOptional(Predicate(func(t doc.Token, _ []rune) bool { return t.Kind.IsFirstPersonSingularPronoun() }))
}
func (s *SequenceExpr) ThenOneOrMoreFirstPersonSingularPronouns() *SequenceExpr {
	return s.ThenOneOrMore(Predicate(func(t doc.Token, _ []rune) bool { return t.Kind.IsFirstPersonSingularPronoun() }))
}
func (s *SequenceExpr) ThenAnythingButFirstPersonSingularPronoun() *SequenceExpr {
	return s.ThenKind(func(k doc.TokenKind) bool { return !k.IsFirstPersonSingularPronoun() })
}
func (s *SequenceExpr) ThenFirstPersonPluralPronoun() *SequenceExpr {
	return s.ThenKind(doc.TokenKind.IsFirstPersonPluralPronoun)
}
func (s *SequenceExpr) ThenOptionalFirstPersonPluralPronoun() *SequenceExpr {
	return s.ThenOptional(Predicate(func(t doc.Token, _ []rune) bool { return t.Kind.IsFirstPersonPluralPronoun() }))
}
func (s *SequenceExpr) ThenOneOrMoreFirstPersonPluralPronouns() *SequenceExpr {
	return s.ThenOneOrMore(Predicate(func(t doc.Token, _ []rune) bool { return t.Kind.IsFirstPersonPluralPronoun() }))
}
func (s *SequenceExpr) ThenAnythingButFirstPersonPluralPronoun() *SequenceExpr {
	return s.ThenKind(func(k doc.TokenKind) bool { return !k.IsFirstPersonPluralPronoun() })
}
func (s *SequenceExpr) ThenSecondPersonPronoun() *SequenceExpr {
	return s.ThenKind(doc.TokenKind.IsSecondPersonPronoun)
}
func (s *SequenceExpr) ThenOptionalSecondPersonPronoun() *SequenceExpr {
	return s.ThenOptional(Predicate(func(t doc.Token, _ []rune) bool { return t.Kind.IsSecondPersonPronoun() }))
}
func (s *SequenceExpr) ThenOneOrMoreSecondPersonPronouns() *SequenceExpr {
	return s.ThenOneOrMore(Predicate(func(t doc.Token, _ []rune) bool { return t.Kind.IsSecondPersonPronoun() }))
}
func (s *SequenceExpr) ThenAnythingButSecondPersonPronoun() *SequenceExpr {
	return s.ThenKind(func(k doc.TokenKind) bool { return !k.IsSecondPersonPronoun() })
}
func (s *SequenceExpr) ThenThirdPersonPronoun() *SequenceExpr {
	return s.ThenKind(doc.TokenKind.IsThirdPersonPronoun)
}
func (s *SequenceExpr) ThenOptionalThirdPersonPronoun() *SequenceExpr {
	return s.ThenOptional(Predicate(func(t doc.Token, _ []rune) bool { return t.Kind.IsThirdPersonPronoun() }))
}
func (s *SequenceExpr) ThenOneOrMoreThirdPersonPronouns() *SequenceExpr {
	return s.ThenOneOrMore(Predicate(func(t doc.Token, _ []rune) bool { return t.Kind.IsThirdPersonPronoun() }))
}
func (s *SequenceExpr) ThenAnythingButThirdPersonPronoun() *SequenceExpr {
	return s.ThenKind(func(k doc.TokenKind) bool { return !k.IsThirdPersonPronoun() })
}
func (s *SequenceExpr) ThenThirdPersonSingularPronoun() *SequenceExpr {
	return s.ThenKind(doc.TokenKind.IsThirdPersonSingularPronoun)
}
func (s *SequenceExpr) ThenOptionalThirdPersonSingularPronoun() *SequenceExpr {
	return s.ThenOptional(Predicate(func(t doc.Token, _ []rune) bool { return t.Kind.IsThirdPersonSingularPronoun() }))
}
func (s *SequenceExpr) ThenOneOrMoreThirdPersonSingularPronouns() *SequenceExpr {
	return s.ThenOneOrMore(Predicate(func(t doc.Token, _ []rune) bool { return t.Kind.IsThirdPersonSingularPronoun() }))
}
func (s *SequenceExpr) ThenAnythingButThirdPersonSingularPronoun() *SequenceExpr {
	return s.ThenKind(func(k doc.TokenKind) bool { return !k.IsThirdPersonSingularPronoun() })
}
func (s *SequenceExpr) ThenThirdPersonPluralPronoun() *SequenceExpr {
	return s.ThenKind(doc.TokenKind.IsThirdPersonPluralPronoun)
}
func (s *SequenceExpr) ThenOptionalThirdPersonPluralPronoun() *SequenceExpr {
	return s.ThenOptional(Predicate(func(t doc.Token, _ []rune) bool { return t.Kind.IsThirdPersonPluralPronoun() }))
}
func (s *SequenceExpr) ThenOneOrMoreThirdPersonPluralPronouns() *SequenceExpr {
	return s.ThenOneOrMore(Predicate(func(t doc.Token, _ []rune) bool { return t.Kind.IsThirdPersonPluralPronoun() }))
}
func (s *SequenceExpr) ThenAnythingButThirdPersonPluralPronoun() *SequenceExpr {
	return s.ThenKind(func(k doc.TokenKind) bool { return !k.IsThirdPersonPluralPronoun() })
}
func (s *SequenceExpr) ThenSubjectPronoun() *SequenceExpr {
	return s.ThenKind(doc.TokenKind.IsSubjectPronoun)
}
func (s *SequenceExpr) ThenOptionalSubjectPronoun() *SequenceExpr {
	return s.ThenOptional(Predicate(func(t doc.Token, _ []rune) bool { return t.Kind.IsSubjectPronoun() }))
}
func (s *SequenceExpr) ThenOneOrMoreSubjectPronouns() *SequenceExpr {
	return s.ThenOneOrMore(Predicate(func(t doc.Token, _ []rune) bool { return t.Kind.IsSubjectPronoun() }))
}
func (s *SequenceExpr) ThenAnythingButSubjectPronoun() *SequenceExpr {
	return s.ThenKind(func(k doc.TokenKind) bool { return !k.IsSubjectPronoun() })
}
func (s *SequenceExpr) ThenObjectPronoun() *SequenceExpr {
	return s.ThenKind(doc.TokenKind.IsObjectPronoun)
}
func (s *SequenceExpr) ThenOptionalObjectPronoun() *SequenceExpr {
	return s.ThenOptional(Predicate(func(t doc.Token, _ []rune) bool { return t.Kind.IsObjectPronoun() }))
}
func (s *SequenceExpr) ThenOneOrMoreObjectPronouns() *SequenceExpr {
	return s.ThenOneOrMore(Predicate(func(t doc.Token, _ []rune) bool { return t.Kind.IsObjectPronoun() }))
}
func (s *SequenceExpr) ThenAnythingButObjectPronoun() *SequenceExpr {
	return s.ThenKind(func(k doc.TokenKind) bool { return !k.IsObjectPronoun() })
}
func (s *SequenceExpr) ThenVerb() *SequenceExpr { return s.ThenKind(doc.TokenKind.IsVerb) }
func (s *SequenceExpr) ThenOptionalVerb() *SequenceExpr {
	return s.ThenOptional(Predicate(func(t doc.Token, _ []rune) bool { return t.Kind.IsVerb() }))
}
func (s *SequenceExpr) ThenOneOrMoreVerbs() *SequenceExpr {
	return s.ThenOneOrMore(Predicate(func(t doc.Token, _ []rune) bool { return t.Kind.IsVerb() }))
}
func (s *SequenceExpr) ThenAnythingButVerb() *SequenceExpr {
	return s.ThenKind(func(k doc.TokenKind) bool { return !k.IsVerb() })
}
func (s *SequenceExpr) ThenAuxiliaryVerb() *SequenceExpr {
	return s.ThenKind(doc.TokenKind.IsAuxiliaryVerb)
}
func (s *SequenceExpr) ThenOptionalAuxiliaryVerb() *SequenceExpr {
	return s.ThenOptional(Predicate(func(t doc.Token, _ []rune) bool { return t.Kind.IsAuxiliaryVerb() }))
}
func (s *SequenceExpr) ThenOneOrMoreAuxiliaryVerbs() *SequenceExpr {
	return s.ThenOneOrMore(Predicate(func(t doc.Token, _ []rune) bool { return t.Kind.IsAuxiliaryVerb() }))
}
func (s *SequenceExpr) ThenAnythingButAuxiliaryVerb() *SequenceExpr {
	return s.ThenKind(func(k doc.TokenKind) bool { return !k.IsAuxiliaryVerb() })
}
func (s *SequenceExpr) ThenLinkingVerb() *SequenceExpr {
	return s.ThenKind(doc.TokenKind.IsLinkingVerb)
}
func (s *SequenceExpr) ThenOptionalLinkingVerb() *SequenceExpr {
	return s.ThenOptional(Predicate(func(t doc.Token, _ []rune) bool { return t.Kind.IsLinkingVerb() }))
}
func (s *SequenceExpr) ThenOneOrMoreLinkingVerbs() *SequenceExpr {
	return s.ThenOneOrMore(Predicate(func(t doc.Token, _ []rune) bool { return t.Kind.IsLinkingVerb() }))
}
func (s *SequenceExpr) ThenAnythingButLinkingVerb() *SequenceExpr {
	return s.ThenKind(func(k doc.TokenKind) bool { return !k.IsLinkingVerb() })
}
func (s *SequenceExpr) ThenVerbLemma() *SequenceExpr { return s.ThenKind(doc.TokenKind.IsVerbLemma) }
func (s *SequenceExpr) ThenOptionalVerbLemma() *SequenceExpr {
	return s.ThenOptional(Predicate(func(t doc.Token, _ []rune) bool { return t.Kind.IsVerbLemma() }))
}
func (s *SequenceExpr) ThenOneOrMoreVerbLemmas() *SequenceExpr {
	return s.ThenOneOrMore(Predicate(func(t doc.Token, _ []rune) bool { return t.Kind.IsVerbLemma() }))
}
func (s *SequenceExpr) ThenAnythingButVerbLemma() *SequenceExpr {
	return s.ThenKind(func(k doc.TokenKind) bool { return !k.IsVerbLemma() })
}
func (s *SequenceExpr) ThenVerbSimplePastForm() *SequenceExpr {
	return s.ThenKind(doc.TokenKind.IsVerbSimplePastForm)
}
func (s *SequenceExpr) ThenOptionalVerbSimplePastForm() *SequenceExpr {
	return s.ThenOptional(Predicate(func(t doc.Token, _ []rune) bool { return t.Kind.IsVerbSimplePastForm() }))
}
func (s *SequenceExpr) ThenOneOrMoreVerbSimplePastForms() *SequenceExpr {
	return s.ThenOneOrMore(Predicate(func(t doc.Token, _ []rune) bool { return t.Kind.IsVerbSimplePastForm() }))
}
func (s *SequenceExpr) ThenAnythingButVerbSimplePastForm() *SequenceExpr {
	return s.ThenKind(func(k doc.TokenKind) bool { return !k.IsVerbSimplePastForm() })
}
func (s *SequenceExpr) ThenVerbPastParticipleForm() *SequenceExpr {
	return s.ThenKind(doc.TokenKind.IsVerbPastParticipleForm)
}
func (s *SequenceExpr) ThenOptionalVerbPastParticipleForm() *SequenceExpr {
	return s.ThenOptional(Predicate(func(t doc.Token, _ []rune) bool { return t.Kind.IsVerbPastParticipleForm() }))
}
func (s *SequenceExpr) ThenOneOrMoreVerbPastParticipleForms() *SequenceExpr {
	return s.ThenOneOrMore(Predicate(func(t doc.Token, _ []rune) bool { return t.Kind.IsVerbPastParticipleForm() }))
}
func (s *SequenceExpr) ThenAnythingButVerbPastParticipleForm() *SequenceExpr {
	return s.ThenKind(func(k doc.TokenKind) bool { return !k.IsVerbPastParticipleForm() })
}
func (s *SequenceExpr) ThenVerbProgressiveForm() *SequenceExpr {
	return s.ThenKind(doc.TokenKind.IsVerbProgressiveForm)
}
func (s *SequenceExpr) ThenOptionalVerbProgressiveForm() *SequenceExpr {
	return s.ThenOptional(Predicate(func(t doc.Token, _ []rune) bool { return t.Kind.IsVerbProgressiveForm() }))
}
func (s *SequenceExpr) ThenOneOrMoreVerbProgressiveForms() *SequenceExpr {
	return s.ThenOneOrMore(Predicate(func(t doc.Token, _ []rune) bool { return t.Kind.IsVerbProgressiveForm() }))
}
func (s *SequenceExpr) ThenAnythingButVerbProgressiveForm() *SequenceExpr {
	return s.ThenKind(func(k doc.TokenKind) bool { return !k.IsVerbProgressiveForm() })
}
func (s *SequenceExpr) ThenVerbThirdPersonSingularPresentForm() *SequenceExpr {
	return s.ThenKind(doc.TokenKind.IsVerbThirdPersonSingularPresentForm)
}
func (s *SequenceExpr) ThenOptionalVerbThirdPersonSingularPresentForm() *SequenceExpr {
	return s.ThenOptional(Predicate(func(t doc.Token, _ []rune) bool { return t.Kind.IsVerbThirdPersonSingularPresentForm() }))
}
func (s *SequenceExpr) ThenOneOrMoreVerbThirdPersonSingularPresentForms() *SequenceExpr {
	return s.ThenOneOrMore(Predicate(func(t doc.Token, _ []rune) bool { return t.Kind.IsVerbThirdPersonSingularPresentForm() }))
}
func (s *SequenceExpr) ThenAnythingButVerbThirdPersonSingularPresentForm() *SequenceExpr {
	return s.ThenKind(func(k doc.TokenKind) bool { return !k.IsVerbThirdPersonSingularPresentForm() })
}
func (s *SequenceExpr) ThenAdjective() *SequenceExpr { return s.ThenKind(doc.TokenKind.IsAdjective) }
func (s *SequenceExpr) ThenOptionalAdjective() *SequenceExpr {
	return s.ThenOptional(Predicate(func(t doc.Token, _ []rune) bool { return t.Kind.IsAdjective() }))
}
func (s *SequenceExpr) ThenOneOrMoreAdjectives() *SequenceExpr {
	return s.ThenOneOrMore(Predicate(func(t doc.Token, _ []rune) bool { return t.Kind.IsAdjective() }))
}
func (s *SequenceExpr) ThenAnythingButAdjective() *SequenceExpr {
	return s.ThenKind(func(k doc.TokenKind) bool { return !k.IsAdjective() })
}
func (s *SequenceExpr) ThenPositiveAdjective() *SequenceExpr {
	return s.ThenKind(doc.TokenKind.IsPositiveAdjective)
}
func (s *SequenceExpr) ThenOptionalPositiveAdjective() *SequenceExpr {
	return s.ThenOptional(Predicate(func(t doc.Token, _ []rune) bool { return t.Kind.IsPositiveAdjective() }))
}
func (s *SequenceExpr) ThenOneOrMorePositiveAdjectives() *SequenceExpr {
	return s.ThenOneOrMore(Predicate(func(t doc.Token, _ []rune) bool { return t.Kind.IsPositiveAdjective() }))
}
func (s *SequenceExpr) ThenAnythingButPositiveAdjective() *SequenceExpr {
	return s.ThenKind(func(k doc.TokenKind) bool { return !k.IsPositiveAdjective() })
}
func (s *SequenceExpr) ThenComparativeAdjective() *SequenceExpr {
	return s.ThenKind(doc.TokenKind.IsComparativeAdjective)
}
func (s *SequenceExpr) ThenOptionalComparativeAdjective() *SequenceExpr {
	return s.ThenOptional(Predicate(func(t doc.Token, _ []rune) bool { return t.Kind.IsComparativeAdjective() }))
}
func (s *SequenceExpr) ThenOneOrMoreComparativeAdjectives() *SequenceExpr {
	return s.ThenOneOrMore(Predicate(func(t doc.Token, _ []rune) bool { return t.Kind.IsComparativeAdjective() }))
}
func (s *SequenceExpr) ThenAnythingButComparativeAdjective() *SequenceExpr {
	return s.ThenKind(func(k doc.TokenKind) bool { return !k.IsComparativeAdjective() })
}
func (s *SequenceExpr) ThenSuperlativeAdjective() *SequenceExpr {
	return s.ThenKind(doc.TokenKind.IsSuperlativeAdjective)
}
func (s *SequenceExpr) ThenOptionalSuperlativeAdjective() *SequenceExpr {
	return s.ThenOptional(Predicate(func(t doc.Token, _ []rune) bool { return t.Kind.IsSuperlativeAdjective() }))
}
func (s *SequenceExpr) ThenOneOrMoreSuperlativeAdjectives() *SequenceExpr {
	return s.ThenOneOrMore(Predicate(func(t doc.Token, _ []rune) bool { return t.Kind.IsSuperlativeAdjective() }))
}
func (s *SequenceExpr) ThenAnythingButSuperlativeAdjective() *SequenceExpr {
	return s.ThenKind(func(k doc.TokenKind) bool { return !k.IsSuperlativeAdjective() })
}
func (s *SequenceExpr) ThenAdverb() *SequenceExpr { return s.ThenKind(doc.TokenKind.IsAdverb) }
func (s *SequenceExpr) ThenOptionalAdverb() *SequenceExpr {
	return s.ThenOptional(Predicate(func(t doc.Token, _ []rune) bool { return t.Kind.IsAdverb() }))
}
func (s *SequenceExpr) ThenOneOrMoreAdverbs() *SequenceExpr {
	return s.ThenOneOrMore(Predicate(func(t doc.Token, _ []rune) bool { return t.Kind.IsAdverb() }))
}
func (s *SequenceExpr) ThenAnythingButAdverb() *SequenceExpr {
	return s.ThenKind(func(k doc.TokenKind) bool { return !k.IsAdverb() })
}
func (s *SequenceExpr) ThenFrequencyAdverb() *SequenceExpr {
	return s.ThenKind(doc.TokenKind.IsFrequencyAdverb)
}
func (s *SequenceExpr) ThenOptionalFrequencyAdverb() *SequenceExpr {
	return s.ThenOptional(Predicate(func(t doc.Token, _ []rune) bool { return t.Kind.IsFrequencyAdverb() }))
}
func (s *SequenceExpr) ThenOneOrMoreFrequencyAdverbs() *SequenceExpr {
	return s.ThenOneOrMore(Predicate(func(t doc.Token, _ []rune) bool { return t.Kind.IsFrequencyAdverb() }))
}
func (s *SequenceExpr) ThenAnythingButFrequencyAdverb() *SequenceExpr {
	return s.ThenKind(func(k doc.TokenKind) bool { return !k.IsFrequencyAdverb() })
}
func (s *SequenceExpr) ThenDegreeAdverb() *SequenceExpr {
	return s.ThenKind(doc.TokenKind.IsDegreeAdverb)
}
func (s *SequenceExpr) ThenOptionalDegreeAdverb() *SequenceExpr {
	return s.ThenOptional(Predicate(func(t doc.Token, _ []rune) bool { return t.Kind.IsDegreeAdverb() }))
}
func (s *SequenceExpr) ThenOneOrMoreDegreeAdverbs() *SequenceExpr {
	return s.ThenOneOrMore(Predicate(func(t doc.Token, _ []rune) bool { return t.Kind.IsDegreeAdverb() }))
}
func (s *SequenceExpr) ThenAnythingButDegreeAdverb() *SequenceExpr {
	return s.ThenKind(func(k doc.TokenKind) bool { return !k.IsDegreeAdverb() })
}
func (s *SequenceExpr) ThenDeterminer() *SequenceExpr { return s.ThenKind(doc.TokenKind.IsDeterminer) }
func (s *SequenceExpr) ThenOptionalDeterminer() *SequenceExpr {
	return s.ThenOptional(Predicate(func(t doc.Token, _ []rune) bool { return t.Kind.IsDeterminer() }))
}
func (s *SequenceExpr) ThenOneOrMoreDeterminers() *SequenceExpr {
	return s.ThenOneOrMore(Predicate(func(t doc.Token, _ []rune) bool { return t.Kind.IsDeterminer() }))
}
func (s *SequenceExpr) ThenAnythingButDeterminer() *SequenceExpr {
	return s.ThenKind(func(k doc.TokenKind) bool { return !k.IsDeterminer() })
}
func (s *SequenceExpr) ThenDemonstrativeDeterminer() *SequenceExpr {
	return s.ThenKind(doc.TokenKind.IsDemonstrativeDeterminer)
}
func (s *SequenceExpr) ThenOptionalDemonstrativeDeterminer() *SequenceExpr {
	return s.ThenOptional(Predicate(func(t doc.Token, _ []rune) bool { return t.Kind.IsDemonstrativeDeterminer() }))
}
func (s *SequenceExpr) ThenOneOrMoreDemonstrativeDeterminers() *SequenceExpr {
	return s.ThenOneOrMore(Predicate(func(t doc.Token, _ []rune) bool { return t.Kind.IsDemonstrativeDeterminer() }))
}
func (s *SequenceExpr) ThenAnythingButDemonstrativeDeterminer() *SequenceExpr {
	return s.ThenKind(func(k doc.TokenKind) bool { return !k.IsDemonstrativeDeterminer() })
}
func (s *SequenceExpr) ThenPossessiveDeterminer() *SequenceExpr {
	return s.ThenKind(doc.TokenKind.IsPossessiveDeterminer)
}
func (s *SequenceExpr) ThenOptionalPossessiveDeterminer() *SequenceExpr {
	return s.ThenOptional(Predicate(func(t doc.Token, _ []rune) bool { return t.Kind.IsPossessiveDeterminer() }))
}
func (s *SequenceExpr) ThenOneOrMorePossessiveDeterminers() *SequenceExpr {
	return s.ThenOneOrMore(Predicate(func(t doc.Token, _ []rune) bool { return t.Kind.IsPossessiveDeterminer() }))
}
func (s *SequenceExpr) ThenAnythingButPossessiveDeterminer() *SequenceExpr {
	return s.ThenKind(func(k doc.TokenKind) bool { return !k.IsPossessiveDeterminer() })
}
func (s *SequenceExpr) ThenQuantifier() *SequenceExpr { return s.ThenKind(doc.TokenKind.IsQuantifier) }
func (s *SequenceExpr) ThenOptionalQuantifier() *SequenceExpr {
	return s.ThenOptional(Predicate(func(t doc.Token, _ []rune) bool { return t.Kind.IsQuantifier() }))
}
func (s *SequenceExpr) ThenOneOrMoreQuantifiers() *SequenceExpr {
	return s.ThenOneOrMore(Predicate(func(t doc.Token, _ []rune) bool { return t.Kind.IsQuantifier() }))
}
func (s *SequenceExpr) ThenAnythingButQuantifier() *SequenceExpr {
	return s.ThenKind(func(k doc.TokenKind) bool { return !k.IsQuantifier() })
}
func (s *SequenceExpr) ThenNonQuantifierDeterminer() *SequenceExpr {
	return s.ThenKind(doc.TokenKind.IsNonQuantifierDeterminer)
}
func (s *SequenceExpr) ThenOptionalNonQuantifierDeterminer() *SequenceExpr {
	return s.ThenOptional(Predicate(func(t doc.Token, _ []rune) bool { return t.Kind.IsNonQuantifierDeterminer() }))
}
func (s *SequenceExpr) ThenOneOrMoreNonQuantifierDeterminers() *SequenceExpr {
	return s.ThenOneOrMore(Predicate(func(t doc.Token, _ []rune) bool { return t.Kind.IsNonQuantifierDeterminer() }))
}
func (s *SequenceExpr) ThenAnythingButNonQuantifierDeterminer() *SequenceExpr {
	return s.ThenKind(func(k doc.TokenKind) bool { return !k.IsNonQuantifierDeterminer() })
}
func (s *SequenceExpr) ThenNonDemonstrativeDeterminer() *SequenceExpr {
	return s.ThenKind(doc.TokenKind.IsNonDemonstrativeDeterminer)
}
func (s *SequenceExpr) ThenOptionalNonDemonstrativeDeterminer() *SequenceExpr {
	return s.ThenOptional(Predicate(func(t doc.Token, _ []rune) bool { return t.Kind.IsNonDemonstrativeDeterminer() }))
}
func (s *SequenceExpr) ThenOneOrMoreNonDemonstrativeDeterminers() *SequenceExpr {
	return s.ThenOneOrMore(Predicate(func(t doc.Token, _ []rune) bool { return t.Kind.IsNonDemonstrativeDeterminer() }))
}
func (s *SequenceExpr) ThenAnythingButNonDemonstrativeDeterminer() *SequenceExpr {
	return s.ThenKind(func(k doc.TokenKind) bool { return !k.IsNonDemonstrativeDeterminer() })
}
func (s *SequenceExpr) ThenConjunction() *SequenceExpr {
	return s.ThenKind(doc.TokenKind.IsConjunction)
}
func (s *SequenceExpr) ThenOptionalConjunction() *SequenceExpr {
	return s.ThenOptional(Predicate(func(t doc.Token, _ []rune) bool { return t.Kind.IsConjunction() }))
}
func (s *SequenceExpr) ThenOneOrMoreConjunctions() *SequenceExpr {
	return s.ThenOneOrMore(Predicate(func(t doc.Token, _ []rune) bool { return t.Kind.IsConjunction() }))
}
func (s *SequenceExpr) ThenAnythingButConjunction() *SequenceExpr {
	return s.ThenKind(func(k doc.TokenKind) bool { return !k.IsConjunction() })
}
func (s *SequenceExpr) ThenPreposition() *SequenceExpr {
	return s.ThenKind(doc.TokenKind.IsPreposition)
}
func (s *SequenceExpr) ThenOptionalPreposition() *SequenceExpr {
	return s.ThenOptional(Predicate(func(t doc.Token, _ []rune) bool { return t.Kind.IsPreposition() }))
}
func (s *SequenceExpr) ThenOneOrMorePrepositions() *SequenceExpr {
	return s.ThenOneOrMore(Predicate(func(t doc.Token, _ []rune) bool { return t.Kind.IsPreposition() }))
}
func (s *SequenceExpr) ThenAnythingButPreposition() *SequenceExpr {
	return s.ThenKind(func(k doc.TokenKind) bool { return !k.IsPreposition() })
}
func (s *SequenceExpr) ThenNumber() *SequenceExpr { return s.ThenKind(doc.TokenKind.IsNumber) }
func (s *SequenceExpr) ThenOptionalNumber() *SequenceExpr {
	return s.ThenOptional(Predicate(func(t doc.Token, _ []rune) bool { return t.Kind.IsNumber() }))
}
func (s *SequenceExpr) ThenOneOrMoreNumbers() *SequenceExpr {
	return s.ThenOneOrMore(Predicate(func(t doc.Token, _ []rune) bool { return t.Kind.IsNumber() }))
}
func (s *SequenceExpr) ThenAnythingButNumber() *SequenceExpr {
	return s.ThenKind(func(k doc.TokenKind) bool { return !k.IsNumber() })
}
func (s *SequenceExpr) ThenCardinalNumber() *SequenceExpr {
	return s.ThenKind(doc.TokenKind.IsCardinalNumber)
}
func (s *SequenceExpr) ThenOptionalCardinalNumber() *SequenceExpr {
	return s.ThenOptional(Predicate(func(t doc.Token, _ []rune) bool { return t.Kind.IsCardinalNumber() }))
}
func (s *SequenceExpr) ThenOneOrMoreCardinalNumbers() *SequenceExpr {
	return s.ThenOneOrMore(Predicate(func(t doc.Token, _ []rune) bool { return t.Kind.IsCardinalNumber() }))
}
func (s *SequenceExpr) ThenAnythingButCardinalNumber() *SequenceExpr {
	return s.ThenKind(func(k doc.TokenKind) bool { return !k.IsCardinalNumber() })
}
func (s *SequenceExpr) ThenOrdinalNumber() *SequenceExpr {
	return s.ThenKind(doc.TokenKind.IsOrdinalNumber)
}
func (s *SequenceExpr) ThenOptionalOrdinalNumber() *SequenceExpr {
	return s.ThenOptional(Predicate(func(t doc.Token, _ []rune) bool { return t.Kind.IsOrdinalNumber() }))
}
func (s *SequenceExpr) ThenOneOrMoreOrdinalNumbers() *SequenceExpr {
	return s.ThenOneOrMore(Predicate(func(t doc.Token, _ []rune) bool { return t.Kind.IsOrdinalNumber() }))
}
func (s *SequenceExpr) ThenAnythingButOrdinalNumber() *SequenceExpr {
	return s.ThenKind(func(k doc.TokenKind) bool { return !k.IsOrdinalNumber() })
}
func (s *SequenceExpr) ThenPunctuation() *SequenceExpr {
	return s.ThenKind(doc.TokenKind.IsPunctuation)
}
func (s *SequenceExpr) ThenOptionalPunctuation() *SequenceExpr {
	return s.ThenOptional(Predicate(func(t doc.Token, _ []rune) bool { return t.Kind.IsPunctuation() }))
}
func (s *SequenceExpr) ThenOneOrMorePunctuations() *SequenceExpr {
	return s.ThenOneOrMore(Predicate(func(t doc.Token, _ []rune) bool { return t.Kind.IsPunctuation() }))
}
func (s *SequenceExpr) ThenAnythingButPunctuation() *SequenceExpr {
	return s.ThenKind(func(k doc.TokenKind) bool { return !k.IsPunctuation() })
}
func (s *SequenceExpr) ThenApostrophe() *SequenceExpr { return s.ThenKind(doc.TokenKind.IsApostrophe) }
func (s *SequenceExpr) ThenOptionalApostrophe() *SequenceExpr {
	return s.ThenOptional(Predicate(func(t doc.Token, _ []rune) bool { return t.Kind.IsApostrophe() }))
}
func (s *SequenceExpr) ThenOneOrMoreApostrophes() *SequenceExpr {
	return s.ThenOneOrMore(Predicate(func(t doc.Token, _ []rune) bool { return t.Kind.IsApostrophe() }))
}
func (s *SequenceExpr) ThenAnythingButApostrophe() *SequenceExpr {
	return s.ThenKind(func(k doc.TokenKind) bool { return !k.IsApostrophe() })
}
func (s *SequenceExpr) ThenComma() *SequenceExpr { return s.ThenKind(doc.TokenKind.IsComma) }
func (s *SequenceExpr) ThenOptionalComma() *SequenceExpr {
	return s.ThenOptional(Predicate(func(t doc.Token, _ []rune) bool { return t.Kind.IsComma() }))
}
func (s *SequenceExpr) ThenOneOrMoreCommas() *SequenceExpr {
	return s.ThenOneOrMore(Predicate(func(t doc.Token, _ []rune) bool { return t.Kind.IsComma() }))
}
func (s *SequenceExpr) ThenAnythingButComma() *SequenceExpr {
	return s.ThenKind(func(k doc.TokenKind) bool { return !k.IsComma() })
}
func (s *SequenceExpr) ThenHyphen() *SequenceExpr { return s.ThenKind(doc.TokenKind.IsHyphen) }
func (s *SequenceExpr) ThenOptionalHyphen() *SequenceExpr {
	return s.ThenOptional(Predicate(func(t doc.Token, _ []rune) bool { return t.Kind.IsHyphen() }))
}
func (s *SequenceExpr) ThenOneOrMoreHyphens() *SequenceExpr {
	return s.ThenOneOrMore(Predicate(func(t doc.Token, _ []rune) bool { return t.Kind.IsHyphen() }))
}
func (s *SequenceExpr) ThenAnythingButHyphen() *SequenceExpr {
	return s.ThenKind(func(k doc.TokenKind) bool { return !k.IsHyphen() })
}
func (s *SequenceExpr) ThenPeriod() *SequenceExpr { return s.ThenKind(doc.TokenKind.IsPeriod) }
func (s *SequenceExpr) ThenOptionalPeriod() *SequenceExpr {
	return s.ThenOptional(Predicate(func(t doc.Token, _ []rune) bool { return t.Kind.IsPeriod() }))
}
func (s *SequenceExpr) ThenOneOrMorePeriods() *SequenceExpr {
	return s.ThenOneOrMore(Predicate(func(t doc.Token, _ []rune) bool { return t.Kind.IsPeriod() }))
}
func (s *SequenceExpr) ThenAnythingButPeriod() *SequenceExpr {
	return s.ThenKind(func(k doc.TokenKind) bool { return !k.IsPeriod() })
}
func (s *SequenceExpr) ThenSemicolon() *SequenceExpr { return s.ThenKind(doc.TokenKind.IsSemicolon) }
func (s *SequenceExpr) ThenOptionalSemicolon() *SequenceExpr {
	return s.ThenOptional(Predicate(func(t doc.Token, _ []rune) bool { return t.Kind.IsSemicolon() }))
}
func (s *SequenceExpr) ThenOneOrMoreSemicolons() *SequenceExpr {
	return s.ThenOneOrMore(Predicate(func(t doc.Token, _ []rune) bool { return t.Kind.IsSemicolon() }))
}
func (s *SequenceExpr) ThenAnythingButSemicolon() *SequenceExpr {
	return s.ThenKind(func(k doc.TokenKind) bool { return !k.IsSemicolon() })
}
func (s *SequenceExpr) ThenAcute() *SequenceExpr { return s.ThenKind(doc.TokenKind.IsAcute) }
func (s *SequenceExpr) ThenOptionalAcute() *SequenceExpr {
	return s.ThenOptional(Predicate(func(t doc.Token, _ []rune) bool { return t.Kind.IsAcute() }))
}
func (s *SequenceExpr) ThenOneOrMoreAcutes() *SequenceExpr {
	return s.ThenOneOrMore(Predicate(func(t doc.Token, _ []rune) bool { return t.Kind.IsAcute() }))
}
func (s *SequenceExpr) ThenAnythingButAcute() *SequenceExpr {
	return s.ThenKind(func(k doc.TokenKind) bool { return !k.IsAcute() })
}
func (s *SequenceExpr) ThenQuote() *SequenceExpr { return s.ThenKind(doc.TokenKind.IsQuote) }
func (s *SequenceExpr) ThenOptionalQuote() *SequenceExpr {
	return s.ThenOptional(Predicate(func(t doc.Token, _ []rune) bool { return t.Kind.IsQuote() }))
}
func (s *SequenceExpr) ThenOneOrMoreQuotes() *SequenceExpr {
	return s.ThenOneOrMore(Predicate(func(t doc.Token, _ []rune) bool { return t.Kind.IsQuote() }))
}
func (s *SequenceExpr) ThenAnythingButQuote() *SequenceExpr {
	return s.ThenKind(func(k doc.TokenKind) bool { return !k.IsQuote() })
}
func (s *SequenceExpr) ThenBackslash() *SequenceExpr { return s.ThenKind(doc.TokenKind.IsBackslash) }
func (s *SequenceExpr) ThenOptionalBackslash() *SequenceExpr {
	return s.ThenOptional(Predicate(func(t doc.Token, _ []rune) bool { return t.Kind.IsBackslash() }))
}
func (s *SequenceExpr) ThenOneOrMoreBackslashs() *SequenceExpr {
	return s.ThenOneOrMore(Predicate(func(t doc.Token, _ []rune) bool { return t.Kind.IsBackslash() }))
}
func (s *SequenceExpr) ThenAnythingButBackslash() *SequenceExpr {
	return s.ThenKind(func(k doc.TokenKind) bool { return !k.IsBackslash() })
}
func (s *SequenceExpr) ThenSlash() *SequenceExpr { return s.ThenKind(doc.TokenKind.IsSlash) }
func (s *SequenceExpr) ThenOptionalSlash() *SequenceExpr {
	return s.ThenOptional(Predicate(func(t doc.Token, _ []rune) bool { return t.Kind.IsSlash() }))
}
func (s *SequenceExpr) ThenOneOrMoreSlashs() *SequenceExpr {
	return s.ThenOneOrMore(Predicate(func(t doc.Token, _ []rune) bool { return t.Kind.IsSlash() }))
}
func (s *SequenceExpr) ThenAnythingButSlash() *SequenceExpr {
	return s.ThenKind(func(k doc.TokenKind) bool { return !k.IsSlash() })
}
func (s *SequenceExpr) ThenPercent() *SequenceExpr { return s.ThenKind(doc.TokenKind.IsPercent) }
func (s *SequenceExpr) ThenOptionalPercent() *SequenceExpr {
	return s.ThenOptional(Predicate(func(t doc.Token, _ []rune) bool { return t.Kind.IsPercent() }))
}
func (s *SequenceExpr) ThenOneOrMorePercents() *SequenceExpr {
	return s.ThenOneOrMore(Predicate(func(t doc.Token, _ []rune) bool { return t.Kind.IsPercent() }))
}
func (s *SequenceExpr) ThenAnythingButPercent() *SequenceExpr {
	return s.ThenKind(func(k doc.TokenKind) bool { return !k.IsPercent() })
}
func (s *SequenceExpr) ThenDegree() *SequenceExpr { return s.ThenKind(doc.TokenKind.IsDegree) }
func (s *SequenceExpr) ThenOptionalDegree() *SequenceExpr {
	return s.ThenOptional(Predicate(func(t doc.Token, _ []rune) bool { return t.Kind.IsDegree() }))
}
func (s *SequenceExpr) ThenOneOrMoreDegrees() *SequenceExpr {
	return s.ThenOneOrMore(Predicate(func(t doc.Token, _ []rune) bool { return t.Kind.IsDegree() }))
}
func (s *SequenceExpr) ThenAnythingButDegree() *SequenceExpr {
	return s.ThenKind(func(k doc.TokenKind) bool { return !k.IsDegree() })
}
func (s *SequenceExpr) ThenOpenSingle() *SequenceExpr { return s.ThenKind(doc.TokenKind.IsOpenSingle) }
func (s *SequenceExpr) ThenOptionalOpenSingle() *SequenceExpr {
	return s.ThenOptional(Predicate(func(t doc.Token, _ []rune) bool { return t.Kind.IsOpenSingle() }))
}
func (s *SequenceExpr) ThenOneOrMoreOpenSingles() *SequenceExpr {
	return s.ThenOneOrMore(Predicate(func(t doc.Token, _ []rune) bool { return t.Kind.IsOpenSingle() }))
}
func (s *SequenceExpr) ThenAnythingButOpenSingle() *SequenceExpr {
	return s.ThenKind(func(k doc.TokenKind) bool { return !k.IsOpenSingle() })
}
func (s *SequenceExpr) ThenSinglePrime() *SequenceExpr {
	return s.ThenKind(doc.TokenKind.IsSinglePrime)
}
func (s *SequenceExpr) ThenOptionalSinglePrime() *SequenceExpr {
	return s.ThenOptional(Predicate(func(t doc.Token, _ []rune) bool { return t.Kind.IsSinglePrime() }))
}
func (s *SequenceExpr) ThenOneOrMoreSinglePrimes() *SequenceExpr {
	return s.ThenOneOrMore(Predicate(func(t doc.Token, _ []rune) bool { return t.Kind.IsSinglePrime() }))
}
func (s *SequenceExpr) ThenAnythingButSinglePrime() *SequenceExpr {
	return s.ThenKind(func(k doc.TokenKind) bool { return !k.IsSinglePrime() })
}
func (s *SequenceExpr) ThenDoublePrime() *SequenceExpr {
	return s.ThenKind(doc.TokenKind.IsDoublePrime)
}
func (s *SequenceExpr) ThenOptionalDoublePrime() *SequenceExpr {
	return s.ThenOptional(Predicate(func(t doc.Token, _ []rune) bool { return t.Kind.IsDoublePrime() }))
}
func (s *SequenceExpr) ThenOneOrMoreDoublePrimes() *SequenceExpr {
	return s.ThenOneOrMore(Predicate(func(t doc.Token, _ []rune) bool { return t.Kind.IsDoublePrime() }))
}
func (s *SequenceExpr) ThenAnythingButDoublePrime() *SequenceExpr {
	return s.ThenKind(func(k doc.TokenKind) bool { return !k.IsDoublePrime() })
}
func (s *SequenceExpr) ThenBacktick() *SequenceExpr { return s.ThenKind(doc.TokenKind.IsBacktick) }
func (s *SequenceExpr) ThenOptionalBacktick() *SequenceExpr {
	return s.ThenOptional(Predicate(func(t doc.Token, _ []rune) bool { return t.Kind.IsBacktick() }))
}
func (s *SequenceExpr) ThenOneOrMoreBackticks() *SequenceExpr {
	return s.ThenOneOrMore(Predicate(func(t doc.Token, _ []rune) bool { return t.Kind.IsBacktick() }))
}
func (s *SequenceExpr) ThenAnythingButBacktick() *SequenceExpr {
	return s.ThenKind(func(k doc.TokenKind) bool { return !k.IsBacktick() })
}
func (s *SequenceExpr) ThenPlus() *SequenceExpr { return s.ThenKind(doc.TokenKind.IsPlus) }
func (s *SequenceExpr) ThenOptionalPlus() *SequenceExpr {
	return s.ThenOptional(Predicate(func(t doc.Token, _ []rune) bool { return t.Kind.IsPlus() }))
}
func (s *SequenceExpr) ThenOneOrMorePluss() *SequenceExpr {
	return s.ThenOneOrMore(Predicate(func(t doc.Token, _ []rune) bool { return t.Kind.IsPlus() }))
}
func (s *SequenceExpr) ThenAnythingButPlus() *SequenceExpr {
	return s.ThenKind(func(k doc.TokenKind) bool { return !k.IsPlus() })
}
func (s *SequenceExpr) ThenCaseSeparator() *SequenceExpr {
	return s.ThenKind(doc.TokenKind.IsCaseSeparator)
}
func (s *SequenceExpr) ThenOptionalCaseSeparator() *SequenceExpr {
	return s.ThenOptional(Predicate(func(t doc.Token, _ []rune) bool { return t.Kind.IsCaseSeparator() }))
}
func (s *SequenceExpr) ThenOneOrMoreCaseSeparators() *SequenceExpr {
	return s.ThenOneOrMore(Predicate(func(t doc.Token, _ []rune) bool { return t.Kind.IsCaseSeparator() }))
}
func (s *SequenceExpr) ThenAnythingButCaseSeparator() *SequenceExpr {
	return s.ThenKind(func(k doc.TokenKind) bool { return !k.IsCaseSeparator() })
}
func (s *SequenceExpr) ThenLikelyHomograph() *SequenceExpr {
	return s.ThenKind(doc.TokenKind.IsLikelyHomograph)
}
func (s *SequenceExpr) ThenOptionalLikelyHomograph() *SequenceExpr {
	return s.ThenOptional(Predicate(func(t doc.Token, _ []rune) bool { return t.Kind.IsLikelyHomograph() }))
}
func (s *SequenceExpr) ThenOneOrMoreLikelyHomographs() *SequenceExpr {
	return s.ThenOneOrMore(Predicate(func(t doc.Token, _ []rune) bool { return t.Kind.IsLikelyHomograph() }))
}
func (s *SequenceExpr) ThenAnythingButLikelyHomograph() *SequenceExpr {
	return s.ThenKind(func(k doc.TokenKind) bool { return !k.IsLikelyHomograph() })
}
func (s *SequenceExpr) ThenSentenceTerminator() *SequenceExpr {
	return s.ThenKind(doc.TokenKind.IsSentenceTerminator)
}
func (s *SequenceExpr) ThenOptionalSentenceTerminator() *SequenceExpr {
	return s.ThenOptional(Predicate(func(t doc.Token, _ []rune) bool { return t.Kind.IsSentenceTerminator() }))
}
func (s *SequenceExpr) ThenOneOrMoreSentenceTerminators() *SequenceExpr {
	return s.ThenOneOrMore(Predicate(func(t doc.Token, _ []rune) bool { return t.Kind.IsSentenceTerminator() }))
}
func (s *SequenceExpr) ThenAnythingButSentenceTerminator() *SequenceExpr {
	return s.ThenKind(func(k doc.TokenKind) bool { return !k.IsSentenceTerminator() })
}
func NewSequenceExprWith(e any) *SequenceExpr { return NewSequenceExpr().Then(e) }
func NewSequenceExprFrom(e any) *SequenceExpr { return NewSequenceExpr().Then(e) }
func NewSequenceExprAnything() *SequenceExpr  { return NewSequenceExpr().ThenAnything() }
func NewSequenceExprAnyCapitalizationOf(word string) *SequenceExpr {
	return NewSequenceExpr().ThenAnyCapitalizationOf(word)
}
func NewSequenceExprAco(word string) *SequenceExpr { return NewSequenceExpr().TAco(word) }
func NewSequenceExprWordSet(words []string) *SequenceExpr {
	return NewSequenceExpr().ThenWordSet(words)
}
func NewSequenceExprAnyWord() *SequenceExpr       { return NewSequenceExpr().ThenAnyWord() }
func NewSequenceExprNumber() *SequenceExpr        { return NewSequenceExpr().ThenNumber() }
func NewSequenceExprOptional(e any) *SequenceExpr { return NewSequenceExpr().ThenOptional(e) }
func NewSequenceExprWordSeq(words []string) *SequenceExpr {
	return NewSequenceExpr().ThenWordSeq(words)
}
func NewSequenceExprFixedPhrase(phrase string) *SequenceExpr {
	return NewSequenceExpr().ThenFixedPhrase(phrase)
}
func NewSequenceExprAnyOf(es []Expr) *SequenceExpr     { return NewSequenceExpr().ThenAnyOf(es) }
func NewSequenceExprLongestOf(es []Expr) *SequenceExpr { return NewSequenceExpr().ThenLongestOf(es) }
func NewSequenceExprWhitespace() *SequenceExpr         { return NewSequenceExpr().ThenWhitespace() }
func NewSequenceExprUnless(e any) *SequenceExpr        { return NewSequenceExpr().ThenUnless(e) }
