package text

type PunctuationKind uint8

const (
	Period PunctuationKind = iota
	Degree
	Ellipsis
	EnDash
	EmDash
	Ampersand
	Bang
	Question
	Colon
	Semicolon
	QuotePunctuation
	Comma
	Hyphen
	OpenSquare
	CloseSquare
	OpenRound
	CloseRound
	OpenCurly
	CloseCurly
	Hash
	Apostrophe
	Percent
	ForwardSlash
	Backslash
	LessThan
	GreaterThan
	Equal
	Star
	Tilde
	At
	Caret
	Plus
	CurrencyPunctuation
	Pipe
	Underscore
	Acute
	OpenSingle
	SinglePrime
	DoublePrime
	Backtick
)

type Punctuation struct {
	Kind     PunctuationKind `json:"kind"`
	Quote    Quote           `json:"quote"`
	Currency Currency        `json:"currency"`
}
type Quote struct {
	TwinLoc *int `json:"twin_loc"`
}

func PunctuationFromChar(c rune) (Punctuation, bool) {
	switch c {
	case 180:
		return Punctuation{Kind: Acute}, true
	case 38:
		return Punctuation{Kind: Ampersand}, true
	case 8217:
		return Punctuation{Kind: Apostrophe}, true
	case 39:
		return Punctuation{Kind: Apostrophe}, true
	case 64:
		return Punctuation{Kind: At}, true
	case 92:
		return Punctuation{Kind: Backslash}, true
	case 33:
		return Punctuation{Kind: Bang}, true
	case 94:
		return Punctuation{Kind: Caret}, true
	case 58:
		return Punctuation{Kind: Colon}, true
	case 44:
		return Punctuation{Kind: Comma}, true
	case 12289:
		return Punctuation{Kind: Comma}, true
	case 65292:
		return Punctuation{Kind: Comma}, true
	case 176:
		return Punctuation{Kind: Degree}, true
	case 8243:
		return Punctuation{Kind: DoublePrime}, true
	case 8211:
		return Punctuation{Kind: EnDash}, true
	case 8212:
		return Punctuation{Kind: EmDash}, true
	case 8230:
		return Punctuation{Kind: Ellipsis}, true
	case 61:
		return Punctuation{Kind: Equal}, true
	case 47:
		return Punctuation{Kind: ForwardSlash}, true
	case 62:
		return Punctuation{Kind: GreaterThan}, true
	case 35:
		return Punctuation{Kind: Hash}, true
	case 45:
		return Punctuation{Kind: Hyphen}, true
	case 60:
		return Punctuation{Kind: LessThan}, true
	case 8216:
		return Punctuation{Kind: OpenSingle}, true
	case 37:
		return Punctuation{Kind: Percent}, true
	case 124:
		return Punctuation{Kind: Pipe}, true
	case 43:
		return Punctuation{Kind: Plus}, true
	case 63:
		return Punctuation{Kind: Question}, true
	case 46:
		return Punctuation{Kind: Period}, true
	case 59:
		return Punctuation{Kind: Semicolon}, true
	case 8242:
		return Punctuation{Kind: SinglePrime}, true
	case 42:
		return Punctuation{Kind: Star}, true
	case 126:
		return Punctuation{Kind: Tilde}, true
	case 95:
		return Punctuation{Kind: Underscore}, true
	case 91:
		return Punctuation{Kind: OpenSquare}, true
	case 93:
		return Punctuation{Kind: CloseSquare}, true
	case 123:
		return Punctuation{Kind: OpenCurly}, true
	case 125:
		return Punctuation{Kind: CloseCurly}, true
	case 40:
		return Punctuation{Kind: OpenRound}, true
	case 41:
		return Punctuation{Kind: CloseRound}, true
	case 96:
		return Punctuation{Kind: Backtick}, true
	}
	if currency, ok := CurrencyFromChar(c); ok {
		return Punctuation{Kind: CurrencyPunctuation, Currency: currency}, true
	}
	return Punctuation{}, false
}
func (p Punctuation) IsDegree() bool       { return p.Kind == Degree }
func (p Punctuation) IsEllipsis() bool     { return p.Kind == Ellipsis }
func (p Punctuation) IsEnDash() bool       { return p.Kind == EnDash }
func (p Punctuation) IsEmDash() bool       { return p.Kind == EmDash }
func (p Punctuation) IsAmpersand() bool    { return p.Kind == Ampersand }
func (p Punctuation) IsPeriod() bool       { return p.Kind == Period }
func (p Punctuation) IsBang() bool         { return p.Kind == Bang }
func (p Punctuation) IsQuestion() bool     { return p.Kind == Question }
func (p Punctuation) IsColon() bool        { return p.Kind == Colon }
func (p Punctuation) IsSemicolon() bool    { return p.Kind == Semicolon }
func (p Punctuation) IsQuote() bool        { return p.Kind == QuotePunctuation }
func (p Punctuation) IsComma() bool        { return p.Kind == Comma }
func (p Punctuation) IsHyphen() bool       { return p.Kind == Hyphen }
func (p Punctuation) IsOpenSquare() bool   { return p.Kind == OpenSquare }
func (p Punctuation) IsCloseSquare() bool  { return p.Kind == CloseSquare }
func (p Punctuation) IsOpenRound() bool    { return p.Kind == OpenRound }
func (p Punctuation) IsCloseRound() bool   { return p.Kind == CloseRound }
func (p Punctuation) IsOpenCurly() bool    { return p.Kind == OpenCurly }
func (p Punctuation) IsCloseCurly() bool   { return p.Kind == CloseCurly }
func (p Punctuation) IsHash() bool         { return p.Kind == Hash }
func (p Punctuation) IsApostrophe() bool   { return p.Kind == Apostrophe }
func (p Punctuation) IsPercent() bool      { return p.Kind == Percent }
func (p Punctuation) IsForwardSlash() bool { return p.Kind == ForwardSlash }
func (p Punctuation) IsBackslash() bool    { return p.Kind == Backslash }
func (p Punctuation) IsLessThan() bool     { return p.Kind == LessThan }
func (p Punctuation) IsGreaterThan() bool  { return p.Kind == GreaterThan }
func (p Punctuation) IsEqual() bool        { return p.Kind == Equal }
func (p Punctuation) IsStar() bool         { return p.Kind == Star }
func (p Punctuation) IsTilde() bool        { return p.Kind == Tilde }
func (p Punctuation) IsAt() bool           { return p.Kind == At }
func (p Punctuation) IsCaret() bool        { return p.Kind == Caret }
func (p Punctuation) IsPlus() bool         { return p.Kind == Plus }
func (p Punctuation) IsCurrency() bool     { return p.Kind == CurrencyPunctuation }
func (p Punctuation) IsPipe() bool         { return p.Kind == Pipe }
func (p Punctuation) IsUnderscore() bool   { return p.Kind == Underscore }
func (p Punctuation) IsAcute() bool        { return p.Kind == Acute }
func (p Punctuation) IsOpenSingle() bool   { return p.Kind == OpenSingle }
func (p Punctuation) IsSinglePrime() bool  { return p.Kind == SinglePrime }
func (p Punctuation) IsDoublePrime() bool  { return p.Kind == DoublePrime }
func (p Punctuation) IsBacktick() bool     { return p.Kind == Backtick }
