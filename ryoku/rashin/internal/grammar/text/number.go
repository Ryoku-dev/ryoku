package text

import (
	"math"
	"strconv"
	"strings"
)

type Number struct {
	Value     float64        `json:"value"`
	Suffix    *OrdinalSuffix `json:"suffix"`
	Radix     uint32         `json:"radix"`
	Precision int            `json:"precision"`
}

func (n Number) String() string {
	var s string
	if n.Radix == 16 {
		v := uint64(0)
		if n.Value >= float64(^uint64(0)) {
			v = ^uint64(0)
		} else if n.Value > 0 {
			v = uint64(n.Value)
		}
		s = "0x" + strings.ToUpper(strconv.FormatUint(v, 16))
	} else {
		s = strconv.FormatFloat(n.Value, 'f', n.Precision, 64)
		if math.IsInf(n.Value, 1) {
			s = "inf"
		} else if math.IsInf(n.Value, -1) {
			s = "-inf"
		}
	}
	if n.Suffix != nil {
		s += string(n.Suffix.ToChars())
	}
	return s
}

type OrdinalSuffix uint8

const (
	Th OrdinalSuffix = iota
	St
	Nd
	Rd
)

func CorrectSuffixFor(number float64) (OrdinalSuffix, bool) {
	if number < 0 || number-math.Floor(number) > math.Nextafter(1, 2)-1 || number > float64(^uint64(0)) {
		return Th, false
	}
	v := uint64(0)
	if number >= float64(^uint64(0)) {
		v = ^uint64(0)
	} else if number > 0 {
		v = uint64(number)
	}
	if v%100 >= 11 && v%100 <= 13 {
		return Th, true
	}
	switch v % 10 {
	case 1:
		return St, true
	case 2:
		return Nd, true
	case 3:
		return Rd, true
	}
	return Th, true
}
func (s OrdinalSuffix) ToChars() []rune {
	switch s {
	case St:
		return []rune{'s', 't'}
	case Nd:
		return []rune{'n', 'd'}
	case Rd:
		return []rune{'r', 'd'}
	default:
		return []rune{'t', 'h'}
	}
}
func OrdinalSuffixFromChars(chars []rune) (OrdinalSuffix, bool) {
	if len(chars) != 2 {
		return Th, false
	}
	switch string([]rune{asciiLower(chars[0]), asciiLower(chars[1])}) {
	case "th":
		return Th, true
	case "st":
		return St, true
	case "nd":
		return Nd, true
	case "rd":
		return Rd, true
	}
	return Th, false
}
func (s OrdinalSuffix) IsTh() bool { return s == Th }
func (s OrdinalSuffix) IsSt() bool { return s == St }
func (s OrdinalSuffix) IsNd() bool { return s == Nd }
func (s OrdinalSuffix) IsRd() bool { return s == Rd }
