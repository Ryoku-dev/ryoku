package dict

import (
	"encoding/json"
	"ryoku-rashin/internal/grammar/pos"
	"strings"
)

func yes(p *bool) bool { return p != nil && *p }
func first[T any](a, b *T) *T {
	if a != nil {
		return a
	}
	return b
}

type Person uint8

const (
	First Person = iota
	Second
	Third
)

func (p Person) IsFirst() bool  { return p == First }
func (p Person) IsSecond() bool { return p == Second }
func (p Person) IsThird() bool  { return p == Third }
func (p Person) MarshalJSON() ([]byte, error) {
	return json.Marshal([]string{"First", "Second", "Third"}[p])
}
func (p *Person) UnmarshalJSON(b []byte) error {
	var v string
	if err := json.Unmarshal(b, &v); err != nil {
		return err
	}
	for i, n := range []string{"First", "Second", "Third"} {
		if n == v {
			*p = Person(i)
			return nil
		}
	}
	return nil
}

type Degree uint8

const (
	Positive Degree = iota
	Comparative
	Superlative
)

func (p Degree) IsPositive() bool    { return p == Positive }
func (p Degree) IsComparative() bool { return p == Comparative }
func (p Degree) IsSuperlative() bool { return p == Superlative }
func (p Degree) MarshalJSON() ([]byte, error) {
	return json.Marshal([]string{"Positive", "Comparative", "Superlative"}[p])
}
func (p *Degree) UnmarshalJSON(b []byte) error {
	var v string
	if err := json.Unmarshal(b, &v); err != nil {
		return err
	}
	for i, n := range []string{"Positive", "Comparative", "Superlative"} {
		if n == v {
			*p = Degree(i)
			return nil
		}
	}
	return nil
}

type NounData struct {
	IsProper     *bool `json:"is_proper"`
	IsSingular   *bool `json:"is_singular"`
	IsPlural     *bool `json:"is_plural"`
	IsCountable  *bool `json:"is_countable"`
	IsMass       *bool `json:"is_mass"`
	IsPossessive *bool `json:"is_possessive"`
}

func (a NounData) Or(b NounData) NounData {
	a.IsProper = first(a.IsProper, b.IsProper)
	a.IsSingular = first(a.IsSingular, b.IsSingular)
	a.IsPlural = first(a.IsPlural, b.IsPlural)
	a.IsCountable = first(a.IsCountable, b.IsCountable)
	a.IsMass = first(a.IsMass, b.IsMass)
	a.IsPossessive = first(a.IsPossessive, b.IsPossessive)
	return a
}

type PronounData struct {
	IsPersonal   *bool   `json:"is_personal"`
	IsSingular   *bool   `json:"is_singular"`
	IsPlural     *bool   `json:"is_plural"`
	IsPossessive *bool   `json:"is_possessive"`
	IsReflexive  *bool   `json:"is_reflexive"`
	IsSubject    *bool   `json:"is_subject"`
	IsObject     *bool   `json:"is_object"`
	Person       *Person `json:"person"`
}

func (a PronounData) Or(b PronounData) PronounData {
	a.IsPersonal = first(a.IsPersonal, b.IsPersonal)
	a.IsSingular = first(a.IsSingular, b.IsSingular)
	a.IsPlural = first(a.IsPlural, b.IsPlural)
	a.IsPossessive = first(a.IsPossessive, b.IsPossessive)
	a.IsReflexive = first(a.IsReflexive, b.IsReflexive)
	a.IsSubject = first(a.IsSubject, b.IsSubject)
	a.IsObject = first(a.IsObject, b.IsObject)
	a.Person = first(a.Person, b.Person)
	return a
}

type VerbData struct {
	IsLinking   *bool          `json:"is_linking"`
	IsAuxiliary *bool          `json:"is_auxiliary"`
	VerbForms   *VerbFormFlags `json:"verb_form"`
}

func (a VerbData) Or(b VerbData) VerbData {
	a.IsLinking = first(a.IsLinking, b.IsLinking)
	a.IsAuxiliary = first(a.IsAuxiliary, b.IsAuxiliary)
	if a.VerbForms != nil && b.VerbForms != nil {
		a.VerbForms = new(*a.VerbForms | *b.VerbForms)
	} else {
		a.VerbForms = first(a.VerbForms, b.VerbForms)
	}
	return a
}

type AdjectiveData struct {
	Degree *Degree `json:"degree"`
}

func (a AdjectiveData) Or(b AdjectiveData) AdjectiveData {
	a.Degree = first(a.Degree, b.Degree)
	return a
}

type AdverbData struct {
	IsManner    *bool `json:"is_manner"`
	IsFrequency *bool `json:"is_frequency"`
	IsDegree    *bool `json:"is_degree"`
}

func (a AdverbData) Or(b AdverbData) AdverbData {
	a.IsManner = first(a.IsManner, b.IsManner)
	a.IsFrequency = first(a.IsFrequency, b.IsFrequency)
	a.IsDegree = first(a.IsDegree, b.IsDegree)
	return a
}

type ConjunctionData struct {
}

func (a ConjunctionData) Or(b ConjunctionData) ConjunctionData {
	return a
}

type DeterminerData struct {
	IsDemonstrative *bool `json:"is_demonstrative"`
	IsPossessive    *bool `json:"is_possessive"`
	IsQuantifier    *bool `json:"is_quantifier"`
}

func (a DeterminerData) Or(b DeterminerData) DeterminerData {
	a.IsDemonstrative = first(a.IsDemonstrative, b.IsDemonstrative)
	a.IsPossessive = first(a.IsPossessive, b.IsPossessive)
	a.IsQuantifier = first(a.IsQuantifier, b.IsQuantifier)
	return a
}

type AffixData struct {
	IsPrefix *bool `json:"is_prefix"`
	IsSuffix *bool `json:"is_suffix"`
}

func (a AffixData) Or(b AffixData) AffixData {
	a.IsPrefix = first(a.IsPrefix, b.IsPrefix)
	a.IsSuffix = first(a.IsSuffix, b.IsSuffix)
	return a
}

type DictWordMetadata struct {
	Noun         *NounData        `json:"noun"`
	Pronoun      *PronounData     `json:"pronoun"`
	Verb         *VerbData        `json:"verb"`
	Adjective    *AdjectiveData   `json:"adjective"`
	Adverb       *AdverbData      `json:"adverb"`
	Conjunction  *ConjunctionData `json:"conjunction"`
	Determiner   *DeterminerData  `json:"determiner"`
	Affix        *AffixData       `json:"affix"`
	Preposition  bool             `json:"preposition"`
	Common       bool             `json:"common"`
	Dialects     DialectFlags     `json:"dialects"`
	OrthInfo     OrthFlags        `json:"orth_info"`
	Swear        *bool            `json:"swear"`
	Abbreviation *bool            `json:"abbreviation"`
	DerivedFrom  DerivedFrom      `json:"derived_from"`
	NpMember     *bool            `json:"np_member"`
	PosTag       *pos.UPOS        `json:"pos_tag"`
}

func (a DictWordMetadata) Or(b DictWordMetadata) DictWordMetadata { a.Merge(b); return a }
func (a *DictWordMetadata) Merge(b DictWordMetadata) *DictWordMetadata {
	if a.Noun == nil {
		a.Noun = b.Noun
	} else if b.Noun != nil && !a.Noun.Equal(*b.Noun) {
		a.Noun = new(a.Noun.Or(*b.Noun))
	}
	if a.Pronoun == nil {
		a.Pronoun = b.Pronoun
	} else if b.Pronoun != nil && !a.Pronoun.Equal(*b.Pronoun) {
		a.Pronoun = new(a.Pronoun.Or(*b.Pronoun))
	}
	if a.Verb == nil {
		a.Verb = b.Verb
	} else if b.Verb != nil && !a.Verb.Equal(*b.Verb) {
		a.Verb = new(a.Verb.Or(*b.Verb))
	}
	if a.Adjective == nil {
		a.Adjective = b.Adjective
	} else if b.Adjective != nil && !a.Adjective.Equal(*b.Adjective) {
		a.Adjective = new(a.Adjective.Or(*b.Adjective))
	}
	if a.Adverb == nil {
		a.Adverb = b.Adverb
	} else if b.Adverb != nil && !a.Adverb.Equal(*b.Adverb) {
		a.Adverb = new(a.Adverb.Or(*b.Adverb))
	}
	if a.Conjunction == nil {
		a.Conjunction = b.Conjunction
	} else if b.Conjunction != nil && !a.Conjunction.Equal(*b.Conjunction) {
		a.Conjunction = new(a.Conjunction.Or(*b.Conjunction))
	}
	if a.Determiner == nil {
		a.Determiner = b.Determiner
	} else if b.Determiner != nil && !a.Determiner.Equal(*b.Determiner) {
		a.Determiner = new(a.Determiner.Or(*b.Determiner))
	}
	if a.Affix == nil {
		a.Affix = b.Affix
	} else if b.Affix != nil && !a.Affix.Equal(*b.Affix) {
		a.Affix = new(a.Affix.Or(*b.Affix))
	}
	a.Preposition = a.Preposition || b.Preposition
	a.Common = a.Common || b.Common
	a.Dialects |= b.Dialects
	a.OrthInfo |= b.OrthInfo
	a.Swear = first(a.Swear, b.Swear)
	a.Abbreviation = first(a.Abbreviation, b.Abbreviation)
	a.NpMember = first(a.NpMember, b.NpMember)
	a.PosTag = first(a.PosTag, b.PosTag)
	a.DerivedFrom = a.DerivedFrom.union(b.DerivedFrom)
	return a
}
func (m DictWordMetadata) IsNoun() bool           { return m.Noun != nil }
func (m DictWordMetadata) IsProperNoun() bool     { return m.Noun != nil && yes(m.Noun.IsProper) }
func (m DictWordMetadata) IsNonProperNoun() bool  { return m.Noun != nil && !yes(m.Noun.IsProper) }
func (m DictWordMetadata) IsPluralNoun() bool     { return m.Noun != nil && yes(m.Noun.IsPlural) }
func (m DictWordMetadata) IsNonPluralNoun() bool  { return m.Noun != nil && !yes(m.Noun.IsPlural) }
func (m DictWordMetadata) IsMassNoun() bool       { return m.Noun != nil && yes(m.Noun.IsMass) }
func (m DictWordMetadata) IsNonMassNoun() bool    { return m.Noun != nil && !yes(m.Noun.IsMass) }
func (m DictWordMetadata) IsPossessiveNoun() bool { return m.Noun != nil && yes(m.Noun.IsPossessive) }
func (m DictWordMetadata) IsNonPossessiveNoun() bool {
	return m.Noun != nil && !yes(m.Noun.IsPossessive)
}
func (m DictWordMetadata) IsPronoun() bool { return m.Pronoun != nil }
func (m DictWordMetadata) IsPersonalPronoun() bool {
	return m.Pronoun != nil && yes(m.Pronoun.IsPersonal)
}
func (m DictWordMetadata) IsNonPersonalPronoun() bool {
	return m.Pronoun != nil && !yes(m.Pronoun.IsPersonal)
}
func (m DictWordMetadata) IsSingularPronoun() bool {
	return m.Pronoun != nil && yes(m.Pronoun.IsSingular)
}
func (m DictWordMetadata) IsNonSingularPronoun() bool {
	return m.Pronoun != nil && !yes(m.Pronoun.IsSingular)
}
func (m DictWordMetadata) IsPluralPronoun() bool { return m.Pronoun != nil && yes(m.Pronoun.IsPlural) }
func (m DictWordMetadata) IsNonPluralPronoun() bool {
	return m.Pronoun != nil && !yes(m.Pronoun.IsPlural)
}
func (m DictWordMetadata) IsPossessivePronoun() bool {
	return m.Pronoun != nil && yes(m.Pronoun.IsPossessive)
}
func (m DictWordMetadata) IsNonPossessivePronoun() bool {
	return m.Pronoun != nil && !yes(m.Pronoun.IsPossessive)
}
func (m DictWordMetadata) IsReflexivePronoun() bool {
	return m.Pronoun != nil && yes(m.Pronoun.IsReflexive)
}
func (m DictWordMetadata) IsNonReflexivePronoun() bool {
	return m.Pronoun != nil && !yes(m.Pronoun.IsReflexive)
}
func (m DictWordMetadata) IsSubjectPronoun() bool {
	return m.Pronoun != nil && yes(m.Pronoun.IsSubject)
}
func (m DictWordMetadata) IsNonSubjectPronoun() bool {
	return m.Pronoun != nil && !yes(m.Pronoun.IsSubject)
}
func (m DictWordMetadata) IsObjectPronoun() bool { return m.Pronoun != nil && yes(m.Pronoun.IsObject) }
func (m DictWordMetadata) IsNonObjectPronoun() bool {
	return m.Pronoun != nil && !yes(m.Pronoun.IsObject)
}
func (m DictWordMetadata) IsVerb() bool             { return m.Verb != nil }
func (m DictWordMetadata) IsLinkingVerb() bool      { return m.Verb != nil && yes(m.Verb.IsLinking) }
func (m DictWordMetadata) IsNonLinkingVerb() bool   { return m.Verb != nil && !yes(m.Verb.IsLinking) }
func (m DictWordMetadata) IsAuxiliaryVerb() bool    { return m.Verb != nil && yes(m.Verb.IsAuxiliary) }
func (m DictWordMetadata) IsNonAuxiliaryVerb() bool { return m.Verb != nil && !yes(m.Verb.IsAuxiliary) }
func (m DictWordMetadata) IsAdjective() bool        { return m.Adjective != nil }
func (m DictWordMetadata) IsAdverb() bool           { return m.Adverb != nil }
func (m DictWordMetadata) IsMannerAdverb() bool     { return m.Adverb != nil && yes(m.Adverb.IsManner) }
func (m DictWordMetadata) IsNonMannerAdverb() bool  { return m.Adverb != nil && !yes(m.Adverb.IsManner) }
func (m DictWordMetadata) IsFrequencyAdverb() bool {
	return m.Adverb != nil && yes(m.Adverb.IsFrequency)
}
func (m DictWordMetadata) IsNonFrequencyAdverb() bool {
	return m.Adverb != nil && !yes(m.Adverb.IsFrequency)
}
func (m DictWordMetadata) IsDegreeAdverb() bool    { return m.Adverb != nil && yes(m.Adverb.IsDegree) }
func (m DictWordMetadata) IsNonDegreeAdverb() bool { return m.Adverb != nil && !yes(m.Adverb.IsDegree) }
func (m DictWordMetadata) IsConjunction() bool     { return m.Conjunction != nil }
func (m DictWordMetadata) IsDeterminer() bool      { return m.Determiner != nil }
func (m DictWordMetadata) IsDemonstrativeDeterminer() bool {
	return m.Determiner != nil && yes(m.Determiner.IsDemonstrative)
}
func (m DictWordMetadata) IsNonDemonstrativeDeterminer() bool {
	return m.Determiner != nil && !yes(m.Determiner.IsDemonstrative)
}
func (m DictWordMetadata) IsPossessiveDeterminer() bool {
	return m.Determiner != nil && yes(m.Determiner.IsPossessive)
}
func (m DictWordMetadata) IsNonPossessiveDeterminer() bool {
	return m.Determiner != nil && !yes(m.Determiner.IsPossessive)
}
func (m DictWordMetadata) IsQuantifierDeterminer() bool {
	return m.Determiner != nil && yes(m.Determiner.IsQuantifier)
}
func (m DictWordMetadata) IsNonQuantifierDeterminer() bool {
	return m.Determiner != nil && !yes(m.Determiner.IsQuantifier)
}
func (m DictWordMetadata) IsAffix() bool          { return m.Affix != nil }
func (m DictWordMetadata) IsPrefixAffix() bool    { return m.Affix != nil && yes(m.Affix.IsPrefix) }
func (m DictWordMetadata) IsNonPrefixAffix() bool { return m.Affix != nil && !yes(m.Affix.IsPrefix) }
func (m DictWordMetadata) IsSuffixAffix() bool    { return m.Affix != nil && yes(m.Affix.IsSuffix) }
func (m DictWordMetadata) IsNonSuffixAffix() bool { return m.Affix != nil && !yes(m.Affix.IsSuffix) }
func (m DictWordMetadata) GetPerson() (Person, bool) {
	if m.Pronoun != nil && m.Pronoun.Person != nil {
		return *m.Pronoun.Person, true
	}
	return 0, false
}
func (m DictWordMetadata) GetDegree() (Degree, bool) {
	if m.Adjective != nil && m.Adjective.Degree != nil {
		return *m.Adjective.Degree, true
	}
	return 0, false
}
func (m DictWordMetadata) IsSingularNoun() bool {
	return m.Noun != nil && (yes(m.Noun.IsSingular) || !yes(m.Noun.IsPlural))
}
func (m DictWordMetadata) IsNonSingularNoun() bool { return m.Noun != nil && !m.IsSingularNoun() }
func (m DictWordMetadata) IsCountableNoun() bool {
	return m.Noun != nil && (yes(m.Noun.IsCountable) || !yes(m.Noun.IsMass))
}
func (m DictWordMetadata) IsNonCountableNoun() bool { return m.Noun != nil && !m.IsCountableNoun() }
func (m DictWordMetadata) IsSingularNounOnly() bool {
	return m.Noun != nil && yes(m.Noun.IsSingular) && !yes(m.Noun.IsPlural)
}
func (m DictWordMetadata) IsPluralNounOnly() bool {
	return m.Noun != nil && !yes(m.Noun.IsSingular) && yes(m.Noun.IsPlural)
}
func (m DictWordMetadata) IsMassNounOnly() bool {
	return m.Noun != nil && !yes(m.Noun.IsCountable) && yes(m.Noun.IsMass)
}
func (m DictWordMetadata) IsNominal() bool { return m.IsNoun() || m.IsPronoun() }
func (m DictWordMetadata) IsSingularNominal() bool {
	return m.IsSingularNoun() || m.IsSingularPronoun()
}
func (m DictWordMetadata) IsPluralNominal() bool { return m.IsPluralNoun() || m.IsPluralPronoun() }
func (m DictWordMetadata) IsPossessiveNominal() bool {
	return m.IsPossessiveNoun() || m.IsPossessiveDeterminer()
}
func (m DictWordMetadata) IsNonSingularNominal() bool {
	return m.IsNonSingularNoun() || m.IsNonSingularPronoun()
}
func (m DictWordMetadata) IsNonPluralNominal() bool {
	return m.IsNonPluralNoun() || m.IsNonPluralPronoun()
}
func (m DictWordMetadata) IsPositiveAdjective() bool {
	return m.Adjective != nil && (m.Adjective.Degree == nil || *m.Adjective.Degree == Positive)
}
func (m DictWordMetadata) IsComparativeAdjective() bool {
	d, ok := m.GetDegree()
	return ok && d == Comparative
}
func (m DictWordMetadata) IsSuperlativeAdjective() bool {
	d, ok := m.GetDegree()
	return ok && d == Superlative
}
func (m DictWordMetadata) IsQuantifier() bool   { return m.IsQuantifierDeterminer() }
func (m DictWordMetadata) IsSwear() bool        { return yes(m.Swear) }
func (m DictWordMetadata) IsAbbreviation() bool { return yes(m.Abbreviation) }
func (m DictWordMetadata) IsLikelyHomograph() bool {
	n := 0
	for _, v := range []bool{m.IsDeterminer(), m.Preposition, m.IsNoun(), m.IsPronoun(), m.IsDeterminer(), m.IsVerb(), m.IsConjunction(), m.IsAdjective(), m.IsAdverb()} {
		if v {
			n++
		}
	}
	return n > 1
}
func (m DictWordMetadata) IsFirstPersonSingularPronoun() bool {
	p, ok := m.GetPerson()
	return ok && p == First && m.IsSingularPronoun()
}
func (m DictWordMetadata) IsFirstPersonPluralPronoun() bool {
	p, ok := m.GetPerson()
	return ok && p == First && m.IsPluralPronoun()
}
func (m DictWordMetadata) IsThirdPersonSingularPronoun() bool {
	p, ok := m.GetPerson()
	return ok && p == Third && m.IsSingularPronoun()
}
func (m DictWordMetadata) IsThirdPersonPluralPronoun() bool {
	p, ok := m.GetPerson()
	return ok && p == Third && m.IsPluralPronoun()
}
func (m DictWordMetadata) IsSecondPersonPronoun() bool {
	p, ok := m.GetPerson()
	return ok && p == Second
}
func (m DictWordMetadata) IsThirdPersonPronoun() bool {
	p, ok := m.GetPerson()
	return ok && p == Third
}
func (m DictWordMetadata) IsVerbLemma() bool {
	return m.Verb != nil && (m.Verb.VerbForms == nil || *m.Verb.VerbForms == 0 || m.Verb.VerbForms.Contains(LEMMA))
}
func (m DictWordMetadata) IsVerbPastForm() bool {
	return m.Verb != nil && m.Verb.VerbForms != nil && m.Verb.VerbForms.Contains(PAST)
}
func (m DictWordMetadata) IsVerbSimplePastForm() bool {
	return m.Verb != nil && m.Verb.VerbForms != nil && m.Verb.VerbForms.Contains(PRETERITE)
}
func (m DictWordMetadata) IsVerbPastParticipleForm() bool {
	return m.Verb != nil && m.Verb.VerbForms != nil && m.Verb.VerbForms.Contains(PAST_PARTICIPLE)
}
func (m DictWordMetadata) IsVerbProgressiveForm() bool {
	return m.Verb != nil && m.Verb.VerbForms != nil && m.Verb.VerbForms.Contains(PROGRESSIVE)
}
func (m DictWordMetadata) IsVerbThirdPersonSingularPresentForm() bool {
	return m.Verb != nil && m.Verb.VerbForms != nil && m.Verb.VerbForms.Contains(THIRD_PERSON_SINGULAR)
}
func (m DictWordMetadata) IsVerbPreteriteAndParticipleForm() bool {
	return m.IsVerbSimplePastForm() && m.IsVerbPastParticipleForm()
}
func (m DictWordMetadata) IsVerbSimplePastOnly() bool {
	return m.IsVerbSimplePastForm() && !m.IsVerbPastForm() && !m.IsVerbPastParticipleForm()
}
func (m DictWordMetadata) IsVerbPastParticipleOnly() bool {
	return m.IsVerbPastParticipleForm() && !m.IsVerbPastForm() && !m.IsVerbSimplePastForm()
}
func (m DictWordMetadata) IsLowercase() bool     { return m.OrthInfo.Contains(LOWERCASE) }
func (m DictWordMetadata) IsTitlecase() bool     { return m.OrthInfo.Contains(TITLECASE) }
func (m DictWordMetadata) IsAllcaps() bool       { return m.OrthInfo.Contains(ALLCAPS) }
func (m DictWordMetadata) IsLowerCamel() bool    { return m.OrthInfo.Contains(LOWER_CAMEL) }
func (m DictWordMetadata) IsUpperCamel() bool    { return m.OrthInfo.Contains(UPPER_CAMEL) }
func (m DictWordMetadata) IsApostrophized() bool { return m.OrthInfo.Contains(APOSTROPHE) }
func (m DictWordMetadata) IsRomanNumerals() bool { return m.OrthInfo.Contains(ROMAN_NUMERALS) }
func (m DictWordMetadata) Difference(b DictWordMetadata) uint32 {
	var n uint32
	for _, q := range []func(DictWordMetadata) bool{DictWordMetadata.IsNoun, DictWordMetadata.IsProperNoun, DictWordMetadata.IsNonProperNoun, DictWordMetadata.IsPluralNoun, DictWordMetadata.IsNonPluralNoun, DictWordMetadata.IsMassNoun, DictWordMetadata.IsNonMassNoun, DictWordMetadata.IsPossessiveNoun, DictWordMetadata.IsNonPossessiveNoun, DictWordMetadata.IsPronoun, DictWordMetadata.IsPersonalPronoun, DictWordMetadata.IsNonPersonalPronoun, DictWordMetadata.IsSingularPronoun, DictWordMetadata.IsNonSingularPronoun, DictWordMetadata.IsPluralPronoun, DictWordMetadata.IsNonPluralPronoun, DictWordMetadata.IsPossessivePronoun, DictWordMetadata.IsNonPossessivePronoun, DictWordMetadata.IsReflexivePronoun, DictWordMetadata.IsNonReflexivePronoun, DictWordMetadata.IsSubjectPronoun, DictWordMetadata.IsNonSubjectPronoun, DictWordMetadata.IsObjectPronoun, DictWordMetadata.IsNonObjectPronoun, DictWordMetadata.IsDeterminer, DictWordMetadata.IsDemonstrativeDeterminer, DictWordMetadata.IsNonDemonstrativeDeterminer, DictWordMetadata.IsPossessiveDeterminer, DictWordMetadata.IsNonPossessiveDeterminer, DictWordMetadata.IsQuantifierDeterminer, DictWordMetadata.IsNonQuantifierDeterminer, DictWordMetadata.IsVerb, DictWordMetadata.IsLinkingVerb, DictWordMetadata.IsNonLinkingVerb, DictWordMetadata.IsAuxiliaryVerb, DictWordMetadata.IsNonAuxiliaryVerb, DictWordMetadata.IsConjunction, DictWordMetadata.IsAdjective, DictWordMetadata.IsAdverb, DictWordMetadata.IsMannerAdverb, DictWordMetadata.IsNonMannerAdverb, DictWordMetadata.IsFrequencyAdverb, DictWordMetadata.IsNonFrequencyAdverb, DictWordMetadata.IsDegreeAdverb, DictWordMetadata.IsNonDegreeAdverb} {
		if q(m) != q(b) {
			n++
		}
	}
	return n
}
func (m DictWordMetadata) InferPosTag() (pos.UPOS, bool) {
	if m.PosTag != nil {
		return *m.PosTag, true
	}
	var tag pos.UPOS
	n := 0
	add := func(ok bool, p pos.UPOS) {
		if ok {
			tag = p
			n++
		}
	}
	add(m.IsProperNoun(), pos.PROPN)
	add(m.IsPronoun(), pos.PRON)
	add(m.IsNoun(), pos.NOUN)
	if m.IsAuxiliaryVerb() {
		add(true, pos.AUX)
	} else {
		add(m.IsVerb(), pos.VERB)
	}
	add(m.IsAdjective(), pos.ADJ)
	add(m.IsAdverb(), pos.ADV)
	add(m.IsConjunction(), pos.CCONJ)
	add(m.IsDeterminer(), pos.DET)
	add(m.Preposition, pos.ADP)
	return tag, n == 1
}
func (m *DictWordMetadata) EnforcePosExclusivity(p pos.UPOS) {
	switch p {
	case pos.NOUN, pos.PROPN, pos.PRON, pos.VERB, pos.AUX, pos.ADJ, pos.ADV, pos.ADP, pos.DET, pos.CCONJ, pos.SCONJ:
	default:
		return
	}
	n, pr, v, a, av, c, d := m.Noun, m.Pronoun, m.Verb, m.Adjective, m.Adverb, m.Conjunction, m.Determiner
	m.Noun = nil
	m.Pronoun = nil
	m.Verb = nil
	m.Adjective = nil
	m.Adverb = nil
	m.Conjunction = nil
	m.Determiner = nil
	m.Affix = nil
	m.Preposition = false
	switch p {
	case pos.NOUN, pos.PROPN:
		if n == nil {
			n = &NounData{}
		} else {
			n = new(*n)
		}
		n.IsProper = new(p == pos.PROPN)
		m.Noun = n
	case pos.PRON:
		if pr == nil {
			pr = &PronounData{}
		}
		m.Pronoun = pr
	case pos.VERB, pos.AUX:
		if v == nil {
			v = &VerbData{}
		} else {
			v = new(*v)
		}
		v.IsAuxiliary = new(p == pos.AUX)
		m.Verb = v
	case pos.ADJ:
		if a == nil {
			a = &AdjectiveData{}
		}
		m.Adjective = a
	case pos.ADV:
		if av == nil {
			av = &AdverbData{}
		}
		m.Adverb = av
	case pos.ADP:
		m.Preposition = true
	case pos.DET:
		m.Determiner = &DeterminerData{}
		_ = d
	case pos.CCONJ, pos.SCONJ:
		if c == nil {
			c = &ConjunctionData{}
		}
		m.Conjunction = c
	}
}

type VerbFormFlags uint16

const (
	LEMMA                 VerbFormFlags = 1 << 0
	PAST                  VerbFormFlags = 1 << 1
	PRETERITE             VerbFormFlags = 1 << 2
	PAST_PARTICIPLE       VerbFormFlags = 1 << 3
	PROGRESSIVE           VerbFormFlags = 1 << 4
	THIRD_PERSON_SINGULAR VerbFormFlags = 1 << 5
)

func (f VerbFormFlags) Contains(b VerbFormFlags) bool   { return f&b == b }
func (f VerbFormFlags) Intersects(b VerbFormFlags) bool { return f&b != 0 }
func (f VerbFormFlags) IsEmpty() bool                   { return f == 0 }
func (f VerbFormFlags) Bits() uint16                    { return uint16(f) }
func (f VerbFormFlags) MarshalJSON() ([]byte, error) {
	var names []string
	for i, n := range []string{"LEMMA", "PAST", "PRETERITE", "PAST_PARTICIPLE", "PROGRESSIVE", "THIRD_PERSON_SINGULAR"} {
		if f&(1<<i) != 0 {
			names = append(names, n)
		}
	}
	return json.Marshal(strings.Join(names, " | "))
}
func (f *VerbFormFlags) UnmarshalJSON(b []byte) error {
	var v string
	if err := json.Unmarshal(b, &v); err != nil {
		var n uint16
		if err = json.Unmarshal(b, &n); err != nil {
			return err
		}
		*f = VerbFormFlags(n)
		return nil
	}
	*f = 0
	for _, part := range strings.Split(v, "|") {
		for i, n := range []string{"LEMMA", "PAST", "PRETERITE", "PAST_PARTICIPLE", "PROGRESSIVE", "THIRD_PERSON_SINGULAR"} {
			if strings.TrimSpace(part) == n {
				*f |= 1 << i
			}
		}
	}
	return nil
}

type DialectFlags uint16

const (
	AMERICAN   DialectFlags = 1 << 0
	CANADIAN   DialectFlags = 1 << 1
	AUSTRALIAN DialectFlags = 1 << 2
	BRITISH    DialectFlags = 1 << 3
	INDIAN     DialectFlags = 1 << 4
)

func (f DialectFlags) Contains(b DialectFlags) bool   { return f&b == b }
func (f DialectFlags) Intersects(b DialectFlags) bool { return f&b != 0 }
func (f DialectFlags) IsEmpty() bool                  { return f == 0 }
func (f DialectFlags) Bits() uint16                   { return uint16(f) }
func (f DialectFlags) MarshalJSON() ([]byte, error) {
	var names []string
	for i, n := range []string{"AMERICAN", "CANADIAN", "AUSTRALIAN", "BRITISH", "INDIAN"} {
		if f&(1<<i) != 0 {
			names = append(names, n)
		}
	}
	return json.Marshal(strings.Join(names, " | "))
}
func (f *DialectFlags) UnmarshalJSON(b []byte) error {
	var v string
	if err := json.Unmarshal(b, &v); err != nil {
		var n uint16
		if err = json.Unmarshal(b, &n); err != nil {
			return err
		}
		*f = DialectFlags(n)
		return nil
	}
	*f = 0
	for _, part := range strings.Split(v, "|") {
		for i, n := range []string{"AMERICAN", "CANADIAN", "AUSTRALIAN", "BRITISH", "INDIAN"} {
			if strings.TrimSpace(part) == n {
				*f |= 1 << i
			}
		}
	}
	return nil
}

type OrthFlags uint16

const (
	LOWERCASE      OrthFlags = 1 << 0
	TITLECASE      OrthFlags = 1 << 1
	ALLCAPS        OrthFlags = 1 << 2
	LOWER_CAMEL    OrthFlags = 1 << 3
	UPPER_CAMEL    OrthFlags = 1 << 4
	MULTIWORD      OrthFlags = 1 << 5
	HYPHENATED     OrthFlags = 1 << 6
	APOSTROPHE     OrthFlags = 1 << 7
	ROMAN_NUMERALS OrthFlags = 1 << 8
)

func (f OrthFlags) Contains(b OrthFlags) bool   { return f&b == b }
func (f OrthFlags) Intersects(b OrthFlags) bool { return f&b != 0 }
func (f OrthFlags) IsEmpty() bool               { return f == 0 }
func (f OrthFlags) Bits() uint16                { return uint16(f) }
func (f OrthFlags) MarshalJSON() ([]byte, error) {
	var names []string
	for i, n := range []string{"LOWERCASE", "TITLECASE", "ALLCAPS", "LOWER_CAMEL", "UPPER_CAMEL", "MULTIWORD", "HYPHENATED", "APOSTROPHE", "ROMAN_NUMERALS"} {
		if f&(1<<i) != 0 {
			names = append(names, n)
		}
	}
	return json.Marshal(strings.Join(names, " | "))
}
func (f *OrthFlags) UnmarshalJSON(b []byte) error {
	var v string
	if err := json.Unmarshal(b, &v); err != nil {
		var n uint16
		if err = json.Unmarshal(b, &n); err != nil {
			return err
		}
		*f = OrthFlags(n)
		return nil
	}
	*f = 0
	for _, part := range strings.Split(v, "|") {
		for i, n := range []string{"LOWERCASE", "TITLECASE", "ALLCAPS", "LOWER_CAMEL", "UPPER_CAMEL", "MULTIWORD", "HYPHENATED", "APOSTROPHE", "ROMAN_NUMERALS"} {
			if strings.TrimSpace(part) == n {
				*f |= 1 << i
			}
		}
	}
	return nil
}

type Dialect uint8

const (
	American   Dialect = 1
	Canadian   Dialect = 2
	Australian Dialect = 4
	British    Dialect = 8
	Indian     Dialect = 16
)

func (d Dialect) String() string {
	switch d {
	case American:
		return "American"
	case Canadian:
		return "Canadian"
	case Australian:
		return "Australian"
	case British:
		return "British"
	case Indian:
		return "Indian"
	}
	return ""
}
func TryFromAbbr(s string) (Dialect, bool) {
	switch s {
	case "US":
		return American, true
	case "CA":
		return Canadian, true
	case "AU":
		return Australian, true
	case "GB":
		return British, true
	case "IN":
		return Indian, true
	}
	return 0, false
}
func TryFromBCP47(s string) (Dialect, bool) {
	if !strings.HasPrefix(s, "en-") {
		return 0, false
	}
	return TryFromAbbr(s[3:])
}
func DialectFromFlags(f DialectFlags) (Dialect, bool) {
	return Dialect(f), f != 0 && f&(f-1) == 0 && f <= INDIAN
}
func DialectFlagsFromDialect(d Dialect) DialectFlags {
	if _, ok := DialectFromFlags(DialectFlags(d)); !ok {
		panic("undefined dialect")
	}
	return DialectFlags(d)
}
func (f DialectFlags) IsDialectEnabled(d Dialect) bool       { return f == 0 || f.IsDialectEnabledStrict(d) }
func (f DialectFlags) IsDialectEnabledStrict(d Dialect) bool { return f&DialectFlags(d) != 0 }
func (m DictWordMetadata) Equal(other DictWordMetadata) bool { return metadataEqual(m, other) }
func (m DictWordMetadata) Clone() DictWordMetadata {
	if m.Noun != nil {
		m.Noun = new(*m.Noun)
		if m.Noun.IsProper != nil {
			m.Noun.IsProper = new(*m.Noun.IsProper)
		}
		if m.Noun.IsSingular != nil {
			m.Noun.IsSingular = new(*m.Noun.IsSingular)
		}
		if m.Noun.IsPlural != nil {
			m.Noun.IsPlural = new(*m.Noun.IsPlural)
		}
		if m.Noun.IsCountable != nil {
			m.Noun.IsCountable = new(*m.Noun.IsCountable)
		}
		if m.Noun.IsMass != nil {
			m.Noun.IsMass = new(*m.Noun.IsMass)
		}
		if m.Noun.IsPossessive != nil {
			m.Noun.IsPossessive = new(*m.Noun.IsPossessive)
		}
	}
	if m.Pronoun != nil {
		m.Pronoun = new(*m.Pronoun)
		if m.Pronoun.IsPersonal != nil {
			m.Pronoun.IsPersonal = new(*m.Pronoun.IsPersonal)
		}
		if m.Pronoun.IsSingular != nil {
			m.Pronoun.IsSingular = new(*m.Pronoun.IsSingular)
		}
		if m.Pronoun.IsPlural != nil {
			m.Pronoun.IsPlural = new(*m.Pronoun.IsPlural)
		}
		if m.Pronoun.IsPossessive != nil {
			m.Pronoun.IsPossessive = new(*m.Pronoun.IsPossessive)
		}
		if m.Pronoun.IsReflexive != nil {
			m.Pronoun.IsReflexive = new(*m.Pronoun.IsReflexive)
		}
		if m.Pronoun.IsSubject != nil {
			m.Pronoun.IsSubject = new(*m.Pronoun.IsSubject)
		}
		if m.Pronoun.IsObject != nil {
			m.Pronoun.IsObject = new(*m.Pronoun.IsObject)
		}
		if m.Pronoun.Person != nil {
			m.Pronoun.Person = new(*m.Pronoun.Person)
		}
	}
	if m.Verb != nil {
		m.Verb = new(*m.Verb)
		if m.Verb.IsLinking != nil {
			m.Verb.IsLinking = new(*m.Verb.IsLinking)
		}
		if m.Verb.IsAuxiliary != nil {
			m.Verb.IsAuxiliary = new(*m.Verb.IsAuxiliary)
		}
		if m.Verb.VerbForms != nil {
			m.Verb.VerbForms = new(*m.Verb.VerbForms)
		}
	}
	if m.Adjective != nil {
		m.Adjective = new(*m.Adjective)
		if m.Adjective.Degree != nil {
			m.Adjective.Degree = new(*m.Adjective.Degree)
		}
	}
	if m.Adverb != nil {
		m.Adverb = new(*m.Adverb)
		if m.Adverb.IsManner != nil {
			m.Adverb.IsManner = new(*m.Adverb.IsManner)
		}
		if m.Adverb.IsFrequency != nil {
			m.Adverb.IsFrequency = new(*m.Adverb.IsFrequency)
		}
		if m.Adverb.IsDegree != nil {
			m.Adverb.IsDegree = new(*m.Adverb.IsDegree)
		}
	}
	if m.Conjunction != nil {
		m.Conjunction = new(*m.Conjunction)
	}
	if m.Determiner != nil {
		m.Determiner = new(*m.Determiner)
		if m.Determiner.IsDemonstrative != nil {
			m.Determiner.IsDemonstrative = new(*m.Determiner.IsDemonstrative)
		}
		if m.Determiner.IsPossessive != nil {
			m.Determiner.IsPossessive = new(*m.Determiner.IsPossessive)
		}
		if m.Determiner.IsQuantifier != nil {
			m.Determiner.IsQuantifier = new(*m.Determiner.IsQuantifier)
		}
	}
	if m.Affix != nil {
		m.Affix = new(*m.Affix)
		if m.Affix.IsPrefix != nil {
			m.Affix.IsPrefix = new(*m.Affix.IsPrefix)
		}
		if m.Affix.IsSuffix != nil {
			m.Affix.IsSuffix = new(*m.Affix.IsSuffix)
		}
	}
	if m.Swear != nil {
		m.Swear = new(*m.Swear)
	}
	if m.Abbreviation != nil {
		m.Abbreviation = new(*m.Abbreviation)
	}
	if m.NpMember != nil {
		m.NpMember = new(*m.NpMember)
	}
	if m.PosTag != nil {
		m.PosTag = new(*m.PosTag)
	}
	m.DerivedFrom = append(DerivedFrom(nil), m.DerivedFrom...)
	return m
}
