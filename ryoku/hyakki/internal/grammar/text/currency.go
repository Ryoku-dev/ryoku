package text

type Currency uint8

const (
	Dollar Currency = iota
	Cent
	Euro
	Ruble
	Lira
	Pound
	Yen
	Baht
	Won
	Kip
	Rupee
)

func CurrencyFromChar(c rune) (Currency, bool) {
	switch c {
	case '$':
		return Dollar, true
	case '¢':
		return Cent, true
	case '€':
		return Euro, true
	case '₽':
		return Ruble, true
	case '₺':
		return Lira, true
	case '£':
		return Pound, true
	case '¥':
		return Yen, true
	case '฿':
		return Baht, true
	case '₩':
		return Won, true
	case '₭':
		return Kip, true
	case '₹':
		return Rupee, true
	}
	return Dollar, false
}
func (c Currency) ToChar() rune {
	switch c {
	case Dollar:
		return '$'
	case Cent:
		return '¢'
	case Euro:
		return '€'
	case Ruble:
		return '₽'
	case Lira:
		return '₺'
	case Pound:
		return '£'
	case Yen:
		return '¥'
	case Baht:
		return '฿'
	case Won:
		return '₩'
	case Kip:
		return '₭'
	case Rupee:
		return '₹'
	}
	panic("invalid currency")
}
func (c Currency) FormatAmount(amount Number) string {
	symbol := string(c.ToChar())
	value := amount.String()
	switch c {
	case Cent:
		return value + symbol
	case Ruble, Lira, Baht:
		return value + " " + symbol
	case Won:
		return symbol + " " + value
	default:
		return symbol + value
	}
}
func (c Currency) IsDollar() bool { return c == Dollar }
func (c Currency) IsCent() bool   { return c == Cent }
func (c Currency) IsEuro() bool   { return c == Euro }
func (c Currency) IsRuble() bool  { return c == Ruble }
func (c Currency) IsLira() bool   { return c == Lira }
func (c Currency) IsPound() bool  { return c == Pound }
func (c Currency) IsYen() bool    { return c == Yen }
func (c Currency) IsBaht() bool   { return c == Baht }
func (c Currency) IsWon() bool    { return c == Won }
func (c Currency) IsKip() bool    { return c == Kip }
func (c Currency) IsRupee() bool  { return c == Rupee }
