package doc

import "ryoku-rashin/internal/grammar/text"

func matchingIndices(tokens []Token, pred func(TokenKind) bool) []int {
	var out []int
	for i, t := range tokens {
		if pred(t.Kind) {
			out = append(out, i)
		}
	}
	return out
}
func matchingTokens(tokens []Token, pred func(TokenKind) bool) []Token {
	var out []Token
	for _, t := range tokens {
		if pred(t.Kind) {
			out = append(out, t)
		}
	}
	return out
}
func firstMatching(tokens []Token, pred func(TokenKind) bool) (*Token, bool) {
	for i := range tokens {
		if pred(tokens[i].Kind) {
			return &tokens[i], true
		}
	}
	return nil, false
}
func lastMatching(tokens []Token, pred func(TokenKind) bool) (*Token, bool) {
	for i := len(tokens) - 1; i >= 0; i-- {
		if pred(tokens[i].Kind) {
			return &tokens[i], true
		}
	}
	return nil, false
}
func lastMatchingIndex(tokens []Token, pred func(TokenKind) bool) (int, bool) {
	for i := len(tokens) - 1; i >= 0; i-- {
		if pred(tokens[i].Kind) {
			return i, true
		}
	}
	return 0, false
}
func Span(tokens []Token) (text.Span, bool) {
	if len(tokens) == 0 {
		return text.Span{}, false
	}
	s := tokens[0].Span
	for _, t := range tokens[1:] {
		if t.Span.Start < s.Start {
			s.Start = t.Span.Start
		}
		if t.Span.End > s.End {
			s.End = t.Span.End
		}
	}
	return s, true
}
func GetCh(tokens []Token, source []rune) ([]rune, bool) {
	s, ok := Span(tokens)
	if !ok {
		return nil, false
	}
	return source[s.Start:s.End], true
}
func GetStr(tokens []Token, source []rune) (string, bool) {
	s, ok := GetCh(tokens, source)
	return string(s), ok
}
func GetRel(tokens []Token, index int) (*Token, bool) {
	if index < 0 {
		index += len(tokens)
	}
	if index < 0 || index >= len(tokens) {
		return nil, false
	}
	return &tokens[index], true
}
func GetRelSlice(tokens []Token, start, end int) ([]Token, bool) {
	if start < 0 {
		start += len(tokens)
	}
	if end < 0 {
		end += len(tokens)
	}
	if start < 0 || end >= len(tokens) || start > end {
		return nil, false
	}
	return tokens[start : end+1], true
}
func FirstSentenceWord(tokens []Token) (*Token, bool) {
	for i := range tokens {
		if tokens[i].Kind.IsWord() {
			return &tokens[i], true
		}
		if tokens[i].Kind.IsUnlintable() {
			return nil, false
		}
	}
	return nil, false
}
func FirstNonWhitespace(tokens []Token) (*Token, bool) {
	return firstMatching(tokens, func(k TokenKind) bool { return !k.IsWhitespace() })
}
func splitInclusive(tokens []Token, pred func(TokenKind) bool) [][]Token {
	return appendSplitInclusive(nil, tokens, pred)
}

func appendSplitInclusive(out [][]Token, tokens []Token, pred func(TokenKind) bool) [][]Token {
	start := 0
	for i := range tokens {
		if pred(tokens[i].Kind) {
			out = append(out, tokens[start:i+1])
			start = i + 1
		}
	}
	if start < len(tokens) {
		out = append(out, tokens[start:])
	}
	return out
}
func IterChunks(tokens []Token) [][]Token { return splitInclusive(tokens, TokenKind.IsChunkTerminator) }
func IterSentences(tokens []Token) [][]Token {
	return splitInclusive(tokens, TokenKind.IsSentenceTerminator)
}

// AppendChunks appends a fresh partition of tokens, reusing the caller's slice
// capacity without retaining boundaries across mutations of the document.
func AppendChunks(out [][]Token, tokens []Token) [][]Token {
	return appendSplitInclusive(out, tokens, TokenKind.IsChunkTerminator)
}

// AppendSentences is the sentence-boundary counterpart of AppendChunks.
func AppendSentences(out [][]Token, tokens []Token) [][]Token {
	return appendSplitInclusive(out, tokens, TokenKind.IsSentenceTerminator)
}
func IterSentencesMut(tokens []Token) [][]Token { return IterSentences(tokens) }
func IterParagraphs(tokens []Token) [][]Token {
	return splitInclusive(tokens, TokenKind.IsParagraphBreak)
}
func IterHeadings(tokens []Token) [][]Token {
	var out [][]Token
	for _, i := range IterHeadingStartIndices(tokens) {
		j := i
		for j < len(tokens)-1 && !tokens[j].Kind.IsParagraphBreak() {
			j++
		}
		out = append(out, tokens[i:j+1])
	}
	return out
}
func IterLinkingVerbIndices(tokens []Token) []int {
	return matchingIndices(tokens, TokenKind.IsLinkingVerb)
}
func IterLinkingVerbs(tokens []Token) []Token { return matchingTokens(tokens, TokenKind.IsLinkingVerb) }
func FirstAdjective(tokens []Token) (*Token, bool) {
	return firstMatching(tokens, TokenKind.IsAdjective)
}
func LastAdjective(tokens []Token) (*Token, bool) { return lastMatching(tokens, TokenKind.IsAdjective) }
func LastAdjectiveIndex(tokens []Token) (int, bool) {
	return lastMatchingIndex(tokens, TokenKind.IsAdjective)
}
func IterAdjectiveIndices(tokens []Token) []int {
	return matchingIndices(tokens, TokenKind.IsAdjective)
}
func IterAdjectives(tokens []Token) []Token { return matchingTokens(tokens, TokenKind.IsAdjective) }
func FirstApostrophe(tokens []Token) (*Token, bool) {
	return firstMatching(tokens, TokenKind.IsApostrophe)
}
func LastApostrophe(tokens []Token) (*Token, bool) {
	return lastMatching(tokens, TokenKind.IsApostrophe)
}
func LastApostropheIndex(tokens []Token) (int, bool) {
	return lastMatchingIndex(tokens, TokenKind.IsApostrophe)
}
func IterApostropheIndices(tokens []Token) []int {
	return matchingIndices(tokens, TokenKind.IsApostrophe)
}
func IterApostrophes(tokens []Token) []Token    { return matchingTokens(tokens, TokenKind.IsApostrophe) }
func FirstAt(tokens []Token) (*Token, bool)     { return firstMatching(tokens, TokenKind.IsAt) }
func LastAt(tokens []Token) (*Token, bool)      { return lastMatching(tokens, TokenKind.IsAt) }
func LastAtIndex(tokens []Token) (int, bool)    { return lastMatchingIndex(tokens, TokenKind.IsAt) }
func IterAtIndices(tokens []Token) []int        { return matchingIndices(tokens, TokenKind.IsAt) }
func IterAts(tokens []Token) []Token            { return matchingTokens(tokens, TokenKind.IsAt) }
func FirstComma(tokens []Token) (*Token, bool)  { return firstMatching(tokens, TokenKind.IsComma) }
func LastComma(tokens []Token) (*Token, bool)   { return lastMatching(tokens, TokenKind.IsComma) }
func LastCommaIndex(tokens []Token) (int, bool) { return lastMatchingIndex(tokens, TokenKind.IsComma) }
func IterCommaIndices(tokens []Token) []int     { return matchingIndices(tokens, TokenKind.IsComma) }
func IterCommas(tokens []Token) []Token         { return matchingTokens(tokens, TokenKind.IsComma) }
func FirstConjunction(tokens []Token) (*Token, bool) {
	return firstMatching(tokens, TokenKind.IsConjunction)
}
func LastConjunction(tokens []Token) (*Token, bool) {
	return lastMatching(tokens, TokenKind.IsConjunction)
}
func LastConjunctionIndex(tokens []Token) (int, bool) {
	return lastMatchingIndex(tokens, TokenKind.IsConjunction)
}
func IterConjunctionIndices(tokens []Token) []int {
	return matchingIndices(tokens, TokenKind.IsConjunction)
}
func IterConjunctions(tokens []Token) []Token { return matchingTokens(tokens, TokenKind.IsConjunction) }
func FirstChunkTerminator(tokens []Token) (*Token, bool) {
	return firstMatching(tokens, TokenKind.IsChunkTerminator)
}
func LastChunkTerminator(tokens []Token) (*Token, bool) {
	return lastMatching(tokens, TokenKind.IsChunkTerminator)
}
func LastChunkTerminatorIndex(tokens []Token) (int, bool) {
	return lastMatchingIndex(tokens, TokenKind.IsChunkTerminator)
}
func IterChunkTerminatorIndices(tokens []Token) []int {
	return matchingIndices(tokens, TokenKind.IsChunkTerminator)
}
func IterChunkTerminators(tokens []Token) []Token {
	return matchingTokens(tokens, TokenKind.IsChunkTerminator)
}
func FirstCurrency(tokens []Token) (*Token, bool) { return firstMatching(tokens, TokenKind.IsCurrency) }
func LastCurrency(tokens []Token) (*Token, bool)  { return lastMatching(tokens, TokenKind.IsCurrency) }
func LastCurrencyIndex(tokens []Token) (int, bool) {
	return lastMatchingIndex(tokens, TokenKind.IsCurrency)
}
func IterCurrencyIndices(tokens []Token) []int    { return matchingIndices(tokens, TokenKind.IsCurrency) }
func IterCurrencys(tokens []Token) []Token        { return matchingTokens(tokens, TokenKind.IsCurrency) }
func FirstEllipsis(tokens []Token) (*Token, bool) { return firstMatching(tokens, TokenKind.IsEllipsis) }
func LastEllipsis(tokens []Token) (*Token, bool)  { return lastMatching(tokens, TokenKind.IsEllipsis) }
func LastEllipsisIndex(tokens []Token) (int, bool) {
	return lastMatchingIndex(tokens, TokenKind.IsEllipsis)
}
func IterEllipsisIndices(tokens []Token) []int    { return matchingIndices(tokens, TokenKind.IsEllipsis) }
func IterEllipsiss(tokens []Token) []Token        { return matchingTokens(tokens, TokenKind.IsEllipsis) }
func FirstHostname(tokens []Token) (*Token, bool) { return firstMatching(tokens, TokenKind.IsHostname) }
func LastHostname(tokens []Token) (*Token, bool)  { return lastMatching(tokens, TokenKind.IsHostname) }
func LastHostnameIndex(tokens []Token) (int, bool) {
	return lastMatchingIndex(tokens, TokenKind.IsHostname)
}
func IterHostnameIndices(tokens []Token) []int { return matchingIndices(tokens, TokenKind.IsHostname) }
func IterHostnames(tokens []Token) []Token     { return matchingTokens(tokens, TokenKind.IsHostname) }
func FirstLikelyHomograph(tokens []Token) (*Token, bool) {
	return firstMatching(tokens, TokenKind.IsLikelyHomograph)
}
func LastLikelyHomograph(tokens []Token) (*Token, bool) {
	return lastMatching(tokens, TokenKind.IsLikelyHomograph)
}
func LastLikelyHomographIndex(tokens []Token) (int, bool) {
	return lastMatchingIndex(tokens, TokenKind.IsLikelyHomograph)
}
func IterLikelyHomographIndices(tokens []Token) []int {
	return matchingIndices(tokens, TokenKind.IsLikelyHomograph)
}
func IterLikelyHomographs(tokens []Token) []Token {
	return matchingTokens(tokens, TokenKind.IsLikelyHomograph)
}
func FirstNumber(tokens []Token) (*Token, bool) { return firstMatching(tokens, TokenKind.IsNumber) }
func LastNumber(tokens []Token) (*Token, bool)  { return lastMatching(tokens, TokenKind.IsNumber) }
func LastNumberIndex(tokens []Token) (int, bool) {
	return lastMatchingIndex(tokens, TokenKind.IsNumber)
}
func IterNumberIndices(tokens []Token) []int   { return matchingIndices(tokens, TokenKind.IsNumber) }
func IterNumbers(tokens []Token) []Token       { return matchingTokens(tokens, TokenKind.IsNumber) }
func FirstNoun(tokens []Token) (*Token, bool)  { return firstMatching(tokens, TokenKind.IsNoun) }
func LastNoun(tokens []Token) (*Token, bool)   { return lastMatching(tokens, TokenKind.IsNoun) }
func LastNounIndex(tokens []Token) (int, bool) { return lastMatchingIndex(tokens, TokenKind.IsNoun) }
func IterNounIndices(tokens []Token) []int     { return matchingIndices(tokens, TokenKind.IsNoun) }
func IterNouns(tokens []Token) []Token         { return matchingTokens(tokens, TokenKind.IsNoun) }
func FirstParagraphBreak(tokens []Token) (*Token, bool) {
	return firstMatching(tokens, TokenKind.IsParagraphBreak)
}
func LastParagraphBreak(tokens []Token) (*Token, bool) {
	return lastMatching(tokens, TokenKind.IsParagraphBreak)
}
func LastParagraphBreakIndex(tokens []Token) (int, bool) {
	return lastMatchingIndex(tokens, TokenKind.IsParagraphBreak)
}
func IterParagraphBreakIndices(tokens []Token) []int {
	return matchingIndices(tokens, TokenKind.IsParagraphBreak)
}
func IterParagraphBreaks(tokens []Token) []Token {
	return matchingTokens(tokens, TokenKind.IsParagraphBreak)
}
func FirstPipe(tokens []Token) (*Token, bool)  { return firstMatching(tokens, TokenKind.IsPipe) }
func LastPipe(tokens []Token) (*Token, bool)   { return lastMatching(tokens, TokenKind.IsPipe) }
func LastPipeIndex(tokens []Token) (int, bool) { return lastMatchingIndex(tokens, TokenKind.IsPipe) }
func IterPipeIndices(tokens []Token) []int     { return matchingIndices(tokens, TokenKind.IsPipe) }
func IterPipes(tokens []Token) []Token         { return matchingTokens(tokens, TokenKind.IsPipe) }
func FirstPreposition(tokens []Token) (*Token, bool) {
	return firstMatching(tokens, TokenKind.IsPreposition)
}
func LastPreposition(tokens []Token) (*Token, bool) {
	return lastMatching(tokens, TokenKind.IsPreposition)
}
func LastPrepositionIndex(tokens []Token) (int, bool) {
	return lastMatchingIndex(tokens, TokenKind.IsPreposition)
}
func IterPrepositionIndices(tokens []Token) []int {
	return matchingIndices(tokens, TokenKind.IsPreposition)
}
func IterPrepositions(tokens []Token) []Token { return matchingTokens(tokens, TokenKind.IsPreposition) }
func FirstPunctuation(tokens []Token) (*Token, bool) {
	return firstMatching(tokens, TokenKind.IsPunctuation)
}
func LastPunctuation(tokens []Token) (*Token, bool) {
	return lastMatching(tokens, TokenKind.IsPunctuation)
}
func LastPunctuationIndex(tokens []Token) (int, bool) {
	return lastMatchingIndex(tokens, TokenKind.IsPunctuation)
}
func IterPunctuationIndices(tokens []Token) []int {
	return matchingIndices(tokens, TokenKind.IsPunctuation)
}
func IterPunctuations(tokens []Token) []Token   { return matchingTokens(tokens, TokenKind.IsPunctuation) }
func FirstQuote(tokens []Token) (*Token, bool)  { return firstMatching(tokens, TokenKind.IsQuote) }
func LastQuote(tokens []Token) (*Token, bool)   { return lastMatching(tokens, TokenKind.IsQuote) }
func LastQuoteIndex(tokens []Token) (int, bool) { return lastMatchingIndex(tokens, TokenKind.IsQuote) }
func IterQuoteIndices(tokens []Token) []int     { return matchingIndices(tokens, TokenKind.IsQuote) }
func IterQuotes(tokens []Token) []Token         { return matchingTokens(tokens, TokenKind.IsQuote) }
func FirstSentenceTerminator(tokens []Token) (*Token, bool) {
	return firstMatching(tokens, TokenKind.IsSentenceTerminator)
}
func LastSentenceTerminator(tokens []Token) (*Token, bool) {
	return lastMatching(tokens, TokenKind.IsSentenceTerminator)
}
func LastSentenceTerminatorIndex(tokens []Token) (int, bool) {
	return lastMatchingIndex(tokens, TokenKind.IsSentenceTerminator)
}
func IterSentenceTerminatorIndices(tokens []Token) []int {
	return matchingIndices(tokens, TokenKind.IsSentenceTerminator)
}
func IterSentenceTerminators(tokens []Token) []Token {
	return matchingTokens(tokens, TokenKind.IsSentenceTerminator)
}
func FirstSpace(tokens []Token) (*Token, bool)  { return firstMatching(tokens, TokenKind.IsSpace) }
func LastSpace(tokens []Token) (*Token, bool)   { return lastMatching(tokens, TokenKind.IsSpace) }
func LastSpaceIndex(tokens []Token) (int, bool) { return lastMatchingIndex(tokens, TokenKind.IsSpace) }
func IterSpaceIndices(tokens []Token) []int     { return matchingIndices(tokens, TokenKind.IsSpace) }
func IterSpaces(tokens []Token) []Token         { return matchingTokens(tokens, TokenKind.IsSpace) }
func FirstUnlintable(tokens []Token) (*Token, bool) {
	return firstMatching(tokens, TokenKind.IsUnlintable)
}
func LastUnlintable(tokens []Token) (*Token, bool) {
	return lastMatching(tokens, TokenKind.IsUnlintable)
}
func LastUnlintableIndex(tokens []Token) (int, bool) {
	return lastMatchingIndex(tokens, TokenKind.IsUnlintable)
}
func IterUnlintableIndices(tokens []Token) []int {
	return matchingIndices(tokens, TokenKind.IsUnlintable)
}
func IterUnlintables(tokens []Token) []Token      { return matchingTokens(tokens, TokenKind.IsUnlintable) }
func FirstVerb(tokens []Token) (*Token, bool)     { return firstMatching(tokens, TokenKind.IsVerb) }
func LastVerb(tokens []Token) (*Token, bool)      { return lastMatching(tokens, TokenKind.IsVerb) }
func LastVerbIndex(tokens []Token) (int, bool)    { return lastMatchingIndex(tokens, TokenKind.IsVerb) }
func IterVerbIndices(tokens []Token) []int        { return matchingIndices(tokens, TokenKind.IsVerb) }
func IterVerbs(tokens []Token) []Token            { return matchingTokens(tokens, TokenKind.IsVerb) }
func FirstWord(tokens []Token) (*Token, bool)     { return firstMatching(tokens, TokenKind.IsWord) }
func LastWord(tokens []Token) (*Token, bool)      { return lastMatching(tokens, TokenKind.IsWord) }
func LastWordIndex(tokens []Token) (int, bool)    { return lastMatchingIndex(tokens, TokenKind.IsWord) }
func IterWordIndices(tokens []Token) []int        { return matchingIndices(tokens, TokenKind.IsWord) }
func IterWords(tokens []Token) []Token            { return matchingTokens(tokens, TokenKind.IsWord) }
func FirstWordLike(tokens []Token) (*Token, bool) { return firstMatching(tokens, TokenKind.IsWordLike) }
func LastWordLike(tokens []Token) (*Token, bool)  { return lastMatching(tokens, TokenKind.IsWordLike) }
func LastWordLikeIndex(tokens []Token) (int, bool) {
	return lastMatchingIndex(tokens, TokenKind.IsWordLike)
}
func IterWordLikeIndices(tokens []Token) []int { return matchingIndices(tokens, TokenKind.IsWordLike) }
func IterWordLikes(tokens []Token) []Token     { return matchingTokens(tokens, TokenKind.IsWordLike) }
func FirstHeadingStart(tokens []Token) (*Token, bool) {
	return firstMatching(tokens, TokenKind.IsHeadingStart)
}
func LastHeadingStart(tokens []Token) (*Token, bool) {
	return lastMatching(tokens, TokenKind.IsHeadingStart)
}
func LastHeadingStartIndex(tokens []Token) (int, bool) {
	return lastMatchingIndex(tokens, TokenKind.IsHeadingStart)
}
func IterHeadingStartIndices(tokens []Token) []int {
	return matchingIndices(tokens, TokenKind.IsHeadingStart)
}
func IterHeadingStarts(tokens []Token) []Token {
	return matchingTokens(tokens, TokenKind.IsHeadingStart)
}
