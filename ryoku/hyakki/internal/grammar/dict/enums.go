package dict

import (
	"encoding/json"
	"fmt"
)

// VerbForm identifies morphological forms, independently of tense, aspect and mood.
type VerbForm uint32

const (
	LemmaForm VerbForm = 1 << iota
	PastForm
	SimplePastForm
	PastParticipleForm
	ProgressiveForm
	ThirdPersonSingularPresentForm
)

type Orthography uint16

const (
	Lowercase Orthography = 1 << iota
	Titlecase
	AllCaps
	LowerCamel
	UpperCamel
	Multiword
	Hyphenated
	Apostrophe
	RomanNumerals
)

func (d Dialect) MarshalJSON() ([]byte, error) { return json.Marshal(d.String()) }
func (d *Dialect) UnmarshalJSON(data []byte) error {
	var name string
	if err := json.Unmarshal(data, &name); err != nil {
		return err
	}
	for _, v := range []Dialect{American, Canadian, Australian, British, Indian} {
		if v.String() == name {
			*d = v
			return nil
		}
	}
	return fmt.Errorf("unknown dialect %q", name)
}
