package pos

import (
	"encoding/json"
	"fmt"
)

// UPOS represents the universal parts of speech from Universal Dependencies.
type UPOS uint8

const (
	ADJ UPOS = iota
	ADP
	ADV
	AUX
	CCONJ
	DET
	INTJ
	NOUN
	NUM
	PART
	PRON
	PROPN
	PUNCT
	SCONJ
	SYM
	VERB
)

var uposNames = [...]string{"ADJ", "ADP", "ADV", "AUX", "CCONJ", "DET", "INTJ", "NOUN", "NUM", "PART", "PRON", "PROPN", "PUNCT", "SCONJ", "SYM", "VERB"}
var uposDescriptions = [...]string{"Adjective", "Adposition", "Adverb", "Auxiliary", "Coordinating conjunction", "Determiner", "Interjection", "Noun", "Numeral", "Particle", "Pronoun", "Proper noun", "Punctuation", "Subordinating conjunction", "Symbol", "Verb"}

func (u UPOS) String() string {
	if int(u) >= len(uposDescriptions) {
		return fmt.Sprintf("UPOS(%d)", u)
	}
	return uposDescriptions[u]
}
func (u UPOS) AsRefStr() string {
	if int(u) >= len(uposNames) {
		return fmt.Sprintf("UPOS(%d)", u)
	}
	return uposNames[u]
}
func (u UPOS) IsNominal() bool { return u == NOUN || u == PROPN || u == PRON }
func ParseUPOS(s string) (UPOS, bool) {
	for i, n := range uposNames {
		if s == n {
			return UPOS(i), true
		}
	}
	return NOUN, false
}
func (u UPOS) MarshalJSON() ([]byte, error) { return json.Marshal(u.AsRefStr()) }
func (u *UPOS) UnmarshalJSON(b []byte) error {
	var s string
	if err := json.Unmarshal(b, &s); err != nil {
		return err
	}
	v, ok := ParseUPOS(s)
	if !ok {
		return fmt.Errorf("invalid UPOS %q", s)
	}
	*u = v
	return nil
}
func (u UPOS) MarshalText() ([]byte, error) { return []byte(u.AsRefStr()), nil }
func (u *UPOS) UnmarshalText(b []byte) error {
	v, ok := ParseUPOS(string(b))
	if !ok {
		return fmt.Errorf("invalid UPOS %q", b)
	}
	*u = v
	return nil
}
func (u UPOS) IsAdj() bool   { return u == ADJ }
func (u UPOS) IsAdp() bool   { return u == ADP }
func (u UPOS) IsAdv() bool   { return u == ADV }
func (u UPOS) IsAux() bool   { return u == AUX }
func (u UPOS) IsCconj() bool { return u == CCONJ }
func (u UPOS) IsDet() bool   { return u == DET }
func (u UPOS) IsIntj() bool  { return u == INTJ }
func (u UPOS) IsNoun() bool  { return u == NOUN }
func (u UPOS) IsNum() bool   { return u == NUM }
func (u UPOS) IsPart() bool  { return u == PART }
func (u UPOS) IsPron() bool  { return u == PRON }
func (u UPOS) IsPropn() bool { return u == PROPN }
func (u UPOS) IsPunct() bool { return u == PUNCT }
func (u UPOS) IsSconj() bool { return u == SCONJ }
func (u UPOS) IsSym() bool   { return u == SYM }
func (u UPOS) IsVerb() bool  { return u == VERB }
