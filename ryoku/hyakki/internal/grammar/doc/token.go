package doc

import (
	"ryoku-rashin/internal/grammar/dict"
	"ryoku-rashin/internal/grammar/pos"
	"ryoku-rashin/internal/grammar/text"
)

type TokenKindTag uint8

const (
	Unlintable TokenKindTag = iota
	Word
	Punctuation
	Decade
	Number
	Space
	Newline
	EmailAddress
	Url
	Hostname
	ParagraphBreak
	Regexish
	HeadingStart
)

type TokenKind struct {
	Kind        TokenKindTag
	Metadata    *dict.DictWordMetadata
	Punctuation text.Punctuation
	Number      text.Number
	Count       int
}

func BlankWord() TokenKind                                      { return TokenKind{Kind: Word} }
func (k TokenKind) WithDefaultData() TokenKind                  { return TokenKind{Kind: k.Kind} }
func (k TokenKind) MatchesVariantOf(other TokenKind) bool       { return k.Kind == other.Kind }
func (k TokenKind) AsWord() (*dict.DictWordMetadata, bool)      { return k.Metadata, k.Kind == Word }
func (k *TokenKind) AsMutWord() (**dict.DictWordMetadata, bool) { return &k.Metadata, k.Kind == Word }
func (k TokenKind) AsNumber() (text.Number, bool)               { return k.Number, k.Kind == Number }
func (k *TokenKind) AsMutNumber() (*text.Number, bool)          { return &k.Number, k.Kind == Number }
func (k TokenKind) AsPunctuation() (text.Punctuation, bool) {
	return k.Punctuation, k.Kind == Punctuation
}
func (k *TokenKind) AsMutPunctuation() (*text.Punctuation, bool) {
	return &k.Punctuation, k.Kind == Punctuation
}
func (k TokenKind) AsSpace() (int, bool)             { return k.Count, k.Kind == Space }
func (k *TokenKind) AsMutSpace() (*int, bool)        { return &k.Count, k.Kind == Space }
func (k TokenKind) AsNewline() (int, bool)           { return k.Count, k.Kind == Newline }
func (k *TokenKind) AsMutNewline() (*int, bool)      { return &k.Count, k.Kind == Newline }
func (k TokenKind) AsQuote() (text.Quote, bool)      { return k.Punctuation.Quote, k.IsQuote() }
func (k *TokenKind) AsMutQuote() (*text.Quote, bool) { return &k.Punctuation.Quote, k.IsQuote() }
func (k TokenKind) IsOov() bool                      { return k.IsWord() && k.Metadata == nil }
func (k TokenKind) IsCommonWord() bool               { return !k.IsWord() || k.Metadata == nil || k.Metadata.Common }
func (k TokenKind) IsNpMember() bool {
	return k.IsWord() && k.Metadata != nil && k.Metadata.NpMember != nil && *k.Metadata.NpMember
}
func (k TokenKind) IsPreposition() bool {
	return k.IsWord() && k.Metadata != nil && k.Metadata.Preposition
}
func (k TokenKind) IsUpos(p pos.UPOS) bool {
	return k.IsWord() && k.Metadata != nil && k.Metadata.PosTag != nil && *k.Metadata.PosTag == p
}
func (k TokenKind) GetPronounPerson() (dict.Person, bool) {
	if k.IsWord() && k.Metadata != nil {
		return k.Metadata.GetPerson()
	}
	return 0, false
}
func (k TokenKind) IsCardinalNumber() bool { return k.IsNumber() && k.Number.Suffix == nil }
func (k TokenKind) IsOrdinalNumber() bool  { return k.IsNumber() && k.Number.Suffix != nil }
func (k TokenKind) IsWordLike() bool {
	return k.IsWord() || k.IsEmailAddress() || k.IsHostname() || k.IsDecade() || k.IsNumber()
}
func (k TokenKind) IsSentenceTerminator() bool {
	if k.Kind == ParagraphBreak {
		return true
	}
	if k.Kind != Punctuation {
		return false
	}
	switch k.Punctuation.Kind {
	case text.Period, text.Bang, text.Question:
		return true
	}
	return false
}
func (k TokenKind) IsChunkTerminator() bool {
	if k.Kind == ParagraphBreak {
		return true
	}
	if k.Kind != Punctuation {
		return false
	}
	switch k.Punctuation.Kind {
	case text.Period, text.Bang, text.Question, text.Comma, text.Semicolon, text.QuotePunctuation, text.Colon:
		return true
	}
	return false
}
func (k TokenKind) IsCaseSeparator() bool { return k.IsHyphen() || k.isPunct(text.Underscore) }
func (k TokenKind) IsWhitespace() bool    { return k.IsSpace() || k.IsNewline() }
func (k TokenKind) isPunct(p text.PunctuationKind) bool {
	return k.Kind == Punctuation && k.Punctuation.Kind == p
}

type Token struct {
	Span text.Span
	Kind TokenKind
}

func NewToken(s text.Span, k TokenKind) Token { return Token{Span: s, Kind: k} }
func (t Token) GetCh(source []rune) []rune    { return source[t.Span.Start:t.Span.End] }
func (t Token) GetStr(source []rune) string   { return string(t.GetCh(source)) }
func (t Token) ToFat(source []rune) FatToken {
	return FatToken{Content: append([]rune(nil), t.GetCh(source)...), Kind: t.Kind}
}

type FatToken struct {
	Content []rune
	Kind    TokenKind
}
type FatStringToken struct {
	Content string
	Kind    TokenKind
}

func (t FatToken) ToFatString() FatStringToken {
	return FatStringToken{Content: string(t.Content), Kind: t.Kind}
}
func (t FatStringToken) ToFat() FatToken   { return FatToken{Content: []rune(t.Content), Kind: t.Kind} }
func (k TokenKind) IsWord() bool           { return k.Kind == Word }
func (k TokenKind) IsPunctuation() bool    { return k.Kind == Punctuation }
func (k TokenKind) IsDecade() bool         { return k.Kind == Decade }
func (k TokenKind) IsNumber() bool         { return k.Kind == Number }
func (k TokenKind) IsSpace() bool          { return k.Kind == Space }
func (k TokenKind) IsNewline() bool        { return k.Kind == Newline }
func (k TokenKind) IsEmailAddress() bool   { return k.Kind == EmailAddress }
func (k TokenKind) IsUrl() bool            { return k.Kind == Url }
func (k TokenKind) IsHostname() bool       { return k.Kind == Hostname }
func (k TokenKind) IsUnlintable() bool     { return k.Kind == Unlintable }
func (k TokenKind) IsParagraphBreak() bool { return k.Kind == ParagraphBreak }
func (k TokenKind) IsRegexish() bool       { return k.Kind == Regexish }
func (k TokenKind) IsHeadingStart() bool   { return k.Kind == HeadingStart }
func (k TokenKind) IsNominal() bool        { return k.IsWord() && k.Metadata != nil && k.Metadata.IsNominal() }
func (k TokenKind) IsNoun() bool           { return k.IsWord() && k.Metadata != nil && k.Metadata.IsNoun() }
func (k TokenKind) IsPronoun() bool        { return k.IsWord() && k.Metadata != nil && k.Metadata.IsPronoun() }
func (k TokenKind) IsProperNoun() bool {
	return k.IsWord() && k.Metadata != nil && k.Metadata.IsProperNoun()
}
func (k TokenKind) IsSingularNominal() bool {
	return k.IsWord() && k.Metadata != nil && k.Metadata.IsSingularNominal()
}
func (k TokenKind) IsPluralNominal() bool {
	return k.IsWord() && k.Metadata != nil && k.Metadata.IsPluralNominal()
}
func (k TokenKind) IsPossessiveNominal() bool {
	return k.IsWord() && k.Metadata != nil && k.Metadata.IsPossessiveNominal()
}
func (k TokenKind) IsNonPluralNominal() bool {
	return k.IsWord() && k.Metadata != nil && k.Metadata.IsNonPluralNominal()
}
func (k TokenKind) IsSingularNoun() bool {
	return k.IsWord() && k.Metadata != nil && k.Metadata.IsSingularNoun()
}
func (k TokenKind) IsSingularNounOnly() bool {
	return k.IsWord() && k.Metadata != nil && k.Metadata.IsSingularNounOnly()
}
func (k TokenKind) IsPluralNoun() bool {
	return k.IsWord() && k.Metadata != nil && k.Metadata.IsPluralNoun()
}
func (k TokenKind) IsPluralNounOnly() bool {
	return k.IsWord() && k.Metadata != nil && k.Metadata.IsPluralNounOnly()
}
func (k TokenKind) IsNonPluralNoun() bool {
	return k.IsWord() && k.Metadata != nil && k.Metadata.IsNonPluralNoun()
}
func (k TokenKind) IsNonPossessiveNoun() bool {
	return k.IsWord() && k.Metadata != nil && k.Metadata.IsNonPossessiveNoun()
}
func (k TokenKind) IsCountableNoun() bool {
	return k.IsWord() && k.Metadata != nil && k.Metadata.IsCountableNoun()
}
func (k TokenKind) IsNonCountableNoun() bool {
	return k.IsWord() && k.Metadata != nil && k.Metadata.IsNonCountableNoun()
}
func (k TokenKind) IsMassNoun() bool {
	return k.IsWord() && k.Metadata != nil && k.Metadata.IsMassNoun()
}
func (k TokenKind) IsMassNounOnly() bool {
	return k.IsWord() && k.Metadata != nil && k.Metadata.IsMassNounOnly()
}
func (k TokenKind) IsNonMassNoun() bool {
	return k.IsWord() && k.Metadata != nil && k.Metadata.IsNonMassNoun()
}
func (k TokenKind) IsSingularPronoun() bool {
	return k.IsWord() && k.Metadata != nil && k.Metadata.IsSingularPronoun()
}
func (k TokenKind) IsPluralPronoun() bool {
	return k.IsWord() && k.Metadata != nil && k.Metadata.IsPluralPronoun()
}
func (k TokenKind) IsNonPluralPronoun() bool {
	return k.IsWord() && k.Metadata != nil && k.Metadata.IsNonPluralPronoun()
}
func (k TokenKind) IsReflexivePronoun() bool {
	return k.IsWord() && k.Metadata != nil && k.Metadata.IsReflexivePronoun()
}
func (k TokenKind) IsPersonalPronoun() bool {
	return k.IsWord() && k.Metadata != nil && k.Metadata.IsPersonalPronoun()
}
func (k TokenKind) IsFirstPersonSingularPronoun() bool {
	return k.IsWord() && k.Metadata != nil && k.Metadata.IsFirstPersonSingularPronoun()
}
func (k TokenKind) IsFirstPersonPluralPronoun() bool {
	return k.IsWord() && k.Metadata != nil && k.Metadata.IsFirstPersonPluralPronoun()
}
func (k TokenKind) IsSecondPersonPronoun() bool {
	return k.IsWord() && k.Metadata != nil && k.Metadata.IsSecondPersonPronoun()
}
func (k TokenKind) IsThirdPersonPronoun() bool {
	return k.IsWord() && k.Metadata != nil && k.Metadata.IsThirdPersonPronoun()
}
func (k TokenKind) IsThirdPersonSingularPronoun() bool {
	return k.IsWord() && k.Metadata != nil && k.Metadata.IsThirdPersonSingularPronoun()
}
func (k TokenKind) IsThirdPersonPluralPronoun() bool {
	return k.IsWord() && k.Metadata != nil && k.Metadata.IsThirdPersonPluralPronoun()
}
func (k TokenKind) IsSubjectPronoun() bool {
	return k.IsWord() && k.Metadata != nil && k.Metadata.IsSubjectPronoun()
}
func (k TokenKind) IsObjectPronoun() bool {
	return k.IsWord() && k.Metadata != nil && k.Metadata.IsObjectPronoun()
}
func (k TokenKind) IsPossessiveNoun() bool {
	return k.IsWord() && k.Metadata != nil && k.Metadata.IsPossessiveNoun()
}
func (k TokenKind) IsPossessivePronoun() bool {
	return k.IsWord() && k.Metadata != nil && k.Metadata.IsPossessivePronoun()
}
func (k TokenKind) IsVerb() bool { return k.IsWord() && k.Metadata != nil && k.Metadata.IsVerb() }
func (k TokenKind) IsAuxiliaryVerb() bool {
	return k.IsWord() && k.Metadata != nil && k.Metadata.IsAuxiliaryVerb()
}
func (k TokenKind) IsLinkingVerb() bool {
	return k.IsWord() && k.Metadata != nil && k.Metadata.IsLinkingVerb()
}
func (k TokenKind) IsVerbLemma() bool {
	return k.IsWord() && k.Metadata != nil && k.Metadata.IsVerbLemma()
}
func (k TokenKind) IsVerbPastForm() bool {
	return k.IsWord() && k.Metadata != nil && k.Metadata.IsVerbPastForm()
}
func (k TokenKind) IsVerbPreteriteAndParticipleForm() bool {
	return k.IsWord() && k.Metadata != nil && k.Metadata.IsVerbPreteriteAndParticipleForm()
}
func (k TokenKind) IsVerbSimplePastForm() bool {
	return k.IsWord() && k.Metadata != nil && k.Metadata.IsVerbSimplePastForm()
}
func (k TokenKind) IsVerbPastParticipleForm() bool {
	return k.IsWord() && k.Metadata != nil && k.Metadata.IsVerbPastParticipleForm()
}
func (k TokenKind) IsVerbSimplePastOnly() bool {
	return k.IsWord() && k.Metadata != nil && k.Metadata.IsVerbSimplePastOnly()
}
func (k TokenKind) IsVerbPastParticipleOnly() bool {
	return k.IsWord() && k.Metadata != nil && k.Metadata.IsVerbPastParticipleOnly()
}
func (k TokenKind) IsVerbProgressiveForm() bool {
	return k.IsWord() && k.Metadata != nil && k.Metadata.IsVerbProgressiveForm()
}
func (k TokenKind) IsVerbThirdPersonSingularPresentForm() bool {
	return k.IsWord() && k.Metadata != nil && k.Metadata.IsVerbThirdPersonSingularPresentForm()
}
func (k TokenKind) IsAdjective() bool {
	return k.IsWord() && k.Metadata != nil && k.Metadata.IsAdjective()
}
func (k TokenKind) IsComparativeAdjective() bool {
	return k.IsWord() && k.Metadata != nil && k.Metadata.IsComparativeAdjective()
}
func (k TokenKind) IsSuperlativeAdjective() bool {
	return k.IsWord() && k.Metadata != nil && k.Metadata.IsSuperlativeAdjective()
}
func (k TokenKind) IsPositiveAdjective() bool {
	return k.IsWord() && k.Metadata != nil && k.Metadata.IsPositiveAdjective()
}
func (k TokenKind) IsAdverb() bool { return k.IsWord() && k.Metadata != nil && k.Metadata.IsAdverb() }
func (k TokenKind) IsMannerAdverb() bool {
	return k.IsWord() && k.Metadata != nil && k.Metadata.IsMannerAdverb()
}
func (k TokenKind) IsFrequencyAdverb() bool {
	return k.IsWord() && k.Metadata != nil && k.Metadata.IsFrequencyAdverb()
}
func (k TokenKind) IsDegreeAdverb() bool {
	return k.IsWord() && k.Metadata != nil && k.Metadata.IsDegreeAdverb()
}
func (k TokenKind) IsDeterminer() bool {
	return k.IsWord() && k.Metadata != nil && k.Metadata.IsDeterminer()
}
func (k TokenKind) IsDemonstrativeDeterminer() bool {
	return k.IsWord() && k.Metadata != nil && k.Metadata.IsDemonstrativeDeterminer()
}
func (k TokenKind) IsPossessiveDeterminer() bool {
	return k.IsWord() && k.Metadata != nil && k.Metadata.IsPossessiveDeterminer()
}
func (k TokenKind) IsQuantifier() bool {
	return k.IsWord() && k.Metadata != nil && k.Metadata.IsQuantifier()
}
func (k TokenKind) IsNonQuantifierDeterminer() bool {
	return k.IsWord() && k.Metadata != nil && k.Metadata.IsNonQuantifierDeterminer()
}
func (k TokenKind) IsNonDemonstrativeDeterminer() bool {
	return k.IsWord() && k.Metadata != nil && k.Metadata.IsNonDemonstrativeDeterminer()
}
func (k TokenKind) IsConjunction() bool {
	return k.IsWord() && k.Metadata != nil && k.Metadata.IsConjunction()
}
func (k TokenKind) IsSwear() bool { return k.IsWord() && k.Metadata != nil && k.Metadata.IsSwear() }
func (k TokenKind) IsAbbreviation() bool {
	return k.IsWord() && k.Metadata != nil && k.Metadata.IsAbbreviation()
}
func (k TokenKind) IsLikelyHomograph() bool {
	return k.IsWord() && k.Metadata != nil && k.Metadata.IsLikelyHomograph()
}
func (k TokenKind) IsLowercase() bool {
	return k.IsWord() && k.Metadata != nil && k.Metadata.IsLowercase()
}
func (k TokenKind) IsTitlecase() bool {
	return k.IsWord() && k.Metadata != nil && k.Metadata.IsTitlecase()
}
func (k TokenKind) IsAllcaps() bool { return k.IsWord() && k.Metadata != nil && k.Metadata.IsAllcaps() }
func (k TokenKind) IsLowerCamel() bool {
	return k.IsWord() && k.Metadata != nil && k.Metadata.IsLowerCamel()
}
func (k TokenKind) IsUpperCamel() bool {
	return k.IsWord() && k.Metadata != nil && k.Metadata.IsUpperCamel()
}
func (k TokenKind) IsApostrophized() bool {
	return k.IsWord() && k.Metadata != nil && k.Metadata.IsApostrophized()
}
func (k TokenKind) IsRomanNumerals() bool {
	return k.IsWord() && k.Metadata != nil && k.Metadata.IsRomanNumerals()
}
func (k TokenKind) IsOpenSquare() bool  { return k.isPunct(text.OpenSquare) }
func (k TokenKind) IsCloseSquare() bool { return k.isPunct(text.CloseSquare) }
func (k TokenKind) IsLessThan() bool    { return k.isPunct(text.LessThan) }
func (k TokenKind) IsGreaterThan() bool { return k.isPunct(text.GreaterThan) }
func (k TokenKind) IsOpenRound() bool   { return k.isPunct(text.OpenRound) }
func (k TokenKind) IsCloseRound() bool  { return k.isPunct(text.CloseRound) }
func (k TokenKind) IsPipe() bool        { return k.isPunct(text.Pipe) }
func (k TokenKind) IsCurrency() bool    { return k.isPunct(text.CurrencyPunctuation) }
func (k TokenKind) IsEllipsis() bool    { return k.isPunct(text.Ellipsis) }
func (k TokenKind) IsHyphen() bool      { return k.isPunct(text.Hyphen) }
func (k TokenKind) IsPlus() bool        { return k.isPunct(text.Plus) }
func (k TokenKind) IsQuote() bool       { return k.isPunct(text.QuotePunctuation) }
func (k TokenKind) IsApostrophe() bool  { return k.isPunct(text.Apostrophe) }
func (k TokenKind) IsPeriod() bool      { return k.isPunct(text.Period) }
func (k TokenKind) IsAt() bool          { return k.isPunct(text.At) }
func (k TokenKind) IsComma() bool       { return k.isPunct(text.Comma) }
func (k TokenKind) IsSemicolon() bool   { return k.isPunct(text.Semicolon) }
func (k TokenKind) IsAcute() bool       { return k.isPunct(text.Acute) }
func (k TokenKind) IsAmpersand() bool   { return k.isPunct(text.Ampersand) }
func (k TokenKind) IsBackslash() bool   { return k.isPunct(text.Backslash) }
func (k TokenKind) IsSlash() bool       { return k.isPunct(text.ForwardSlash) }
func (k TokenKind) IsPercent() bool     { return k.isPunct(text.Percent) }
func (k TokenKind) IsDegree() bool      { return k.isPunct(text.Degree) }
func (k TokenKind) IsOpenSingle() bool  { return k.isPunct(text.OpenSingle) }
func (k TokenKind) IsSinglePrime() bool { return k.isPunct(text.SinglePrime) }
func (k TokenKind) IsDoublePrime() bool { return k.isPunct(text.DoublePrime) }
func (k TokenKind) IsBacktick() bool    { return k.isPunct(text.Backtick) }
