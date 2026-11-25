package doc

import "ryoku-rashin/internal/grammar/text"

func (d *Document) FirstAdjective() (*Token, bool)          { return FirstAdjective(d.tokens) }
func (d *Document) LastAdjective() (*Token, bool)           { return LastAdjective(d.tokens) }
func (d *Document) LastAdjectiveIndex() (int, bool)         { return LastAdjectiveIndex(d.tokens) }
func (d *Document) IterAdjectiveIndices() []int             { return IterAdjectiveIndices(d.tokens) }
func (d *Document) IterAdjectives() []Token                 { return IterAdjectives(d.tokens) }
func (d *Document) FirstApostrophe() (*Token, bool)         { return FirstApostrophe(d.tokens) }
func (d *Document) LastApostrophe() (*Token, bool)          { return LastApostrophe(d.tokens) }
func (d *Document) LastApostropheIndex() (int, bool)        { return LastApostropheIndex(d.tokens) }
func (d *Document) IterApostropheIndices() []int            { return IterApostropheIndices(d.tokens) }
func (d *Document) IterApostrophes() []Token                { return IterApostrophes(d.tokens) }
func (d *Document) FirstAt() (*Token, bool)                 { return FirstAt(d.tokens) }
func (d *Document) LastAt() (*Token, bool)                  { return LastAt(d.tokens) }
func (d *Document) LastAtIndex() (int, bool)                { return LastAtIndex(d.tokens) }
func (d *Document) IterAtIndices() []int                    { return IterAtIndices(d.tokens) }
func (d *Document) IterAts() []Token                        { return IterAts(d.tokens) }
func (d *Document) FirstComma() (*Token, bool)              { return FirstComma(d.tokens) }
func (d *Document) LastComma() (*Token, bool)               { return LastComma(d.tokens) }
func (d *Document) LastCommaIndex() (int, bool)             { return LastCommaIndex(d.tokens) }
func (d *Document) IterCommaIndices() []int                 { return IterCommaIndices(d.tokens) }
func (d *Document) IterCommas() []Token                     { return IterCommas(d.tokens) }
func (d *Document) FirstConjunction() (*Token, bool)        { return FirstConjunction(d.tokens) }
func (d *Document) LastConjunction() (*Token, bool)         { return LastConjunction(d.tokens) }
func (d *Document) LastConjunctionIndex() (int, bool)       { return LastConjunctionIndex(d.tokens) }
func (d *Document) IterConjunctionIndices() []int           { return IterConjunctionIndices(d.tokens) }
func (d *Document) IterConjunctions() []Token               { return IterConjunctions(d.tokens) }
func (d *Document) FirstChunkTerminator() (*Token, bool)    { return FirstChunkTerminator(d.tokens) }
func (d *Document) LastChunkTerminator() (*Token, bool)     { return LastChunkTerminator(d.tokens) }
func (d *Document) LastChunkTerminatorIndex() (int, bool)   { return LastChunkTerminatorIndex(d.tokens) }
func (d *Document) IterChunkTerminatorIndices() []int       { return IterChunkTerminatorIndices(d.tokens) }
func (d *Document) IterChunkTerminators() []Token           { return IterChunkTerminators(d.tokens) }
func (d *Document) FirstCurrency() (*Token, bool)           { return FirstCurrency(d.tokens) }
func (d *Document) LastCurrency() (*Token, bool)            { return LastCurrency(d.tokens) }
func (d *Document) LastCurrencyIndex() (int, bool)          { return LastCurrencyIndex(d.tokens) }
func (d *Document) IterCurrencyIndices() []int              { return IterCurrencyIndices(d.tokens) }
func (d *Document) IterCurrencys() []Token                  { return IterCurrencys(d.tokens) }
func (d *Document) FirstEllipsis() (*Token, bool)           { return FirstEllipsis(d.tokens) }
func (d *Document) LastEllipsis() (*Token, bool)            { return LastEllipsis(d.tokens) }
func (d *Document) LastEllipsisIndex() (int, bool)          { return LastEllipsisIndex(d.tokens) }
func (d *Document) IterEllipsisIndices() []int              { return IterEllipsisIndices(d.tokens) }
func (d *Document) IterEllipsiss() []Token                  { return IterEllipsiss(d.tokens) }
func (d *Document) FirstHostname() (*Token, bool)           { return FirstHostname(d.tokens) }
func (d *Document) LastHostname() (*Token, bool)            { return LastHostname(d.tokens) }
func (d *Document) LastHostnameIndex() (int, bool)          { return LastHostnameIndex(d.tokens) }
func (d *Document) IterHostnameIndices() []int              { return IterHostnameIndices(d.tokens) }
func (d *Document) IterHostnames() []Token                  { return IterHostnames(d.tokens) }
func (d *Document) FirstLikelyHomograph() (*Token, bool)    { return FirstLikelyHomograph(d.tokens) }
func (d *Document) LastLikelyHomograph() (*Token, bool)     { return LastLikelyHomograph(d.tokens) }
func (d *Document) LastLikelyHomographIndex() (int, bool)   { return LastLikelyHomographIndex(d.tokens) }
func (d *Document) IterLikelyHomographIndices() []int       { return IterLikelyHomographIndices(d.tokens) }
func (d *Document) IterLikelyHomographs() []Token           { return IterLikelyHomographs(d.tokens) }
func (d *Document) FirstNumber() (*Token, bool)             { return FirstNumber(d.tokens) }
func (d *Document) LastNumber() (*Token, bool)              { return LastNumber(d.tokens) }
func (d *Document) LastNumberIndex() (int, bool)            { return LastNumberIndex(d.tokens) }
func (d *Document) IterNumberIndices() []int                { return IterNumberIndices(d.tokens) }
func (d *Document) IterNumbers() []Token                    { return IterNumbers(d.tokens) }
func (d *Document) FirstNoun() (*Token, bool)               { return FirstNoun(d.tokens) }
func (d *Document) LastNoun() (*Token, bool)                { return LastNoun(d.tokens) }
func (d *Document) LastNounIndex() (int, bool)              { return LastNounIndex(d.tokens) }
func (d *Document) IterNounIndices() []int                  { return IterNounIndices(d.tokens) }
func (d *Document) IterNouns() []Token                      { return IterNouns(d.tokens) }
func (d *Document) FirstParagraphBreak() (*Token, bool)     { return FirstParagraphBreak(d.tokens) }
func (d *Document) LastParagraphBreak() (*Token, bool)      { return LastParagraphBreak(d.tokens) }
func (d *Document) LastParagraphBreakIndex() (int, bool)    { return LastParagraphBreakIndex(d.tokens) }
func (d *Document) IterParagraphBreakIndices() []int        { return IterParagraphBreakIndices(d.tokens) }
func (d *Document) IterParagraphBreaks() []Token            { return IterParagraphBreaks(d.tokens) }
func (d *Document) FirstPipe() (*Token, bool)               { return FirstPipe(d.tokens) }
func (d *Document) LastPipe() (*Token, bool)                { return LastPipe(d.tokens) }
func (d *Document) LastPipeIndex() (int, bool)              { return LastPipeIndex(d.tokens) }
func (d *Document) IterPipeIndices() []int                  { return IterPipeIndices(d.tokens) }
func (d *Document) IterPipes() []Token                      { return IterPipes(d.tokens) }
func (d *Document) FirstPreposition() (*Token, bool)        { return FirstPreposition(d.tokens) }
func (d *Document) LastPreposition() (*Token, bool)         { return LastPreposition(d.tokens) }
func (d *Document) LastPrepositionIndex() (int, bool)       { return LastPrepositionIndex(d.tokens) }
func (d *Document) IterPrepositionIndices() []int           { return IterPrepositionIndices(d.tokens) }
func (d *Document) IterPrepositions() []Token               { return IterPrepositions(d.tokens) }
func (d *Document) FirstPunctuation() (*Token, bool)        { return FirstPunctuation(d.tokens) }
func (d *Document) LastPunctuation() (*Token, bool)         { return LastPunctuation(d.tokens) }
func (d *Document) LastPunctuationIndex() (int, bool)       { return LastPunctuationIndex(d.tokens) }
func (d *Document) IterPunctuationIndices() []int           { return IterPunctuationIndices(d.tokens) }
func (d *Document) IterPunctuations() []Token               { return IterPunctuations(d.tokens) }
func (d *Document) FirstQuote() (*Token, bool)              { return FirstQuote(d.tokens) }
func (d *Document) LastQuote() (*Token, bool)               { return LastQuote(d.tokens) }
func (d *Document) LastQuoteIndex() (int, bool)             { return LastQuoteIndex(d.tokens) }
func (d *Document) IterQuoteIndices() []int                 { return IterQuoteIndices(d.tokens) }
func (d *Document) IterQuotes() []Token                     { return IterQuotes(d.tokens) }
func (d *Document) FirstSentenceTerminator() (*Token, bool) { return FirstSentenceTerminator(d.tokens) }
func (d *Document) LastSentenceTerminator() (*Token, bool)  { return LastSentenceTerminator(d.tokens) }
func (d *Document) LastSentenceTerminatorIndex() (int, bool) {
	return LastSentenceTerminatorIndex(d.tokens)
}
func (d *Document) IterSentenceTerminatorIndices() []int {
	return IterSentenceTerminatorIndices(d.tokens)
}
func (d *Document) IterSentenceTerminators() []Token    { return IterSentenceTerminators(d.tokens) }
func (d *Document) FirstSpace() (*Token, bool)          { return FirstSpace(d.tokens) }
func (d *Document) LastSpace() (*Token, bool)           { return LastSpace(d.tokens) }
func (d *Document) LastSpaceIndex() (int, bool)         { return LastSpaceIndex(d.tokens) }
func (d *Document) IterSpaceIndices() []int             { return IterSpaceIndices(d.tokens) }
func (d *Document) IterSpaces() []Token                 { return IterSpaces(d.tokens) }
func (d *Document) FirstUnlintable() (*Token, bool)     { return FirstUnlintable(d.tokens) }
func (d *Document) LastUnlintable() (*Token, bool)      { return LastUnlintable(d.tokens) }
func (d *Document) LastUnlintableIndex() (int, bool)    { return LastUnlintableIndex(d.tokens) }
func (d *Document) IterUnlintableIndices() []int        { return IterUnlintableIndices(d.tokens) }
func (d *Document) IterUnlintables() []Token            { return IterUnlintables(d.tokens) }
func (d *Document) FirstVerb() (*Token, bool)           { return FirstVerb(d.tokens) }
func (d *Document) LastVerb() (*Token, bool)            { return LastVerb(d.tokens) }
func (d *Document) LastVerbIndex() (int, bool)          { return LastVerbIndex(d.tokens) }
func (d *Document) IterVerbIndices() []int              { return IterVerbIndices(d.tokens) }
func (d *Document) IterVerbs() []Token                  { return IterVerbs(d.tokens) }
func (d *Document) FirstWord() (*Token, bool)           { return FirstWord(d.tokens) }
func (d *Document) LastWord() (*Token, bool)            { return LastWord(d.tokens) }
func (d *Document) LastWordIndex() (int, bool)          { return LastWordIndex(d.tokens) }
func (d *Document) IterWordIndices() []int              { return IterWordIndices(d.tokens) }
func (d *Document) IterWords() []Token                  { return IterWords(d.tokens) }
func (d *Document) FirstWordLike() (*Token, bool)       { return FirstWordLike(d.tokens) }
func (d *Document) LastWordLike() (*Token, bool)        { return LastWordLike(d.tokens) }
func (d *Document) LastWordLikeIndex() (int, bool)      { return LastWordLikeIndex(d.tokens) }
func (d *Document) IterWordLikeIndices() []int          { return IterWordLikeIndices(d.tokens) }
func (d *Document) IterWordLikes() []Token              { return IterWordLikes(d.tokens) }
func (d *Document) FirstHeadingStart() (*Token, bool)   { return FirstHeadingStart(d.tokens) }
func (d *Document) LastHeadingStart() (*Token, bool)    { return LastHeadingStart(d.tokens) }
func (d *Document) LastHeadingStartIndex() (int, bool)  { return LastHeadingStartIndex(d.tokens) }
func (d *Document) IterHeadingStartIndices() []int      { return IterHeadingStartIndices(d.tokens) }
func (d *Document) IterHeadingStarts() []Token          { return IterHeadingStarts(d.tokens) }
func (d *Document) IterChunks() [][]Token               { return IterChunks(d.tokens) }
func (d *Document) IterSentences() [][]Token            { return IterSentences(d.tokens) }
func (d *Document) IterSentencesMut() [][]Token         { return IterSentencesMut(d.tokens) }
func (d *Document) IterParagraphs() [][]Token           { return IterParagraphs(d.tokens) }
func (d *Document) IterHeadings() [][]Token             { return IterHeadings(d.tokens) }
func (d *Document) FirstSentenceWord() (*Token, bool)   { return FirstSentenceWord(d.tokens) }
func (d *Document) FirstNonWhitespace() (*Token, bool)  { return FirstNonWhitespace(d.tokens) }
func (d *Document) Span() (text.Span, bool)             { return Span(d.tokens) }
func (d *Document) GetCh(source []rune) ([]rune, bool)  { return GetCh(d.tokens, source) }
func (d *Document) GetStr(source []rune) (string, bool) { return GetStr(d.tokens, source) }
func (d *Document) IterLinkingVerbIndices() []int       { return IterLinkingVerbIndices(d.tokens) }
func (d *Document) IterLinkingVerbs() []Token           { return IterLinkingVerbs(d.tokens) }
