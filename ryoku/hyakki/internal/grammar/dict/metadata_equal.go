package dict

import "slices"

func equalOptional[T comparable](a, b *T) bool { return a == b || (a != nil && b != nil && *a == *b) }
func (a NounData) Equal(b NounData) bool {
	return equalOptional(a.IsProper, b.IsProper) && equalOptional(a.IsSingular, b.IsSingular) && equalOptional(a.IsPlural, b.IsPlural) && equalOptional(a.IsCountable, b.IsCountable) && equalOptional(a.IsMass, b.IsMass) && equalOptional(a.IsPossessive, b.IsPossessive)
}
func equalNoun(a, b *NounData) bool { return a == b || (a != nil && b != nil && a.Equal(*b)) }
func (a PronounData) Equal(b PronounData) bool {
	return equalOptional(a.IsPersonal, b.IsPersonal) && equalOptional(a.IsSingular, b.IsSingular) && equalOptional(a.IsPlural, b.IsPlural) && equalOptional(a.IsPossessive, b.IsPossessive) && equalOptional(a.IsReflexive, b.IsReflexive) && equalOptional(a.IsSubject, b.IsSubject) && equalOptional(a.IsObject, b.IsObject) && equalOptional(a.Person, b.Person)
}
func equalPronoun(a, b *PronounData) bool { return a == b || (a != nil && b != nil && a.Equal(*b)) }
func (a VerbData) Equal(b VerbData) bool {
	return equalOptional(a.IsLinking, b.IsLinking) && equalOptional(a.IsAuxiliary, b.IsAuxiliary) && equalOptional(a.VerbForms, b.VerbForms)
}
func equalVerb(a, b *VerbData) bool                { return a == b || (a != nil && b != nil && a.Equal(*b)) }
func (a AdjectiveData) Equal(b AdjectiveData) bool { return equalOptional(a.Degree, b.Degree) }
func equalAdjective(a, b *AdjectiveData) bool      { return a == b || (a != nil && b != nil && a.Equal(*b)) }
func (a AdverbData) Equal(b AdverbData) bool {
	return equalOptional(a.IsManner, b.IsManner) && equalOptional(a.IsFrequency, b.IsFrequency) && equalOptional(a.IsDegree, b.IsDegree)
}
func equalAdverb(a, b *AdverbData) bool                { return a == b || (a != nil && b != nil && a.Equal(*b)) }
func (a ConjunctionData) Equal(b ConjunctionData) bool { return true }
func equalConjunction(a, b *ConjunctionData) bool {
	return a == b || (a != nil && b != nil && a.Equal(*b))
}
func (a DeterminerData) Equal(b DeterminerData) bool {
	return equalOptional(a.IsDemonstrative, b.IsDemonstrative) && equalOptional(a.IsPossessive, b.IsPossessive) && equalOptional(a.IsQuantifier, b.IsQuantifier)
}
func equalDeterminer(a, b *DeterminerData) bool {
	return a == b || (a != nil && b != nil && a.Equal(*b))
}
func (a AffixData) Equal(b AffixData) bool {
	return equalOptional(a.IsPrefix, b.IsPrefix) && equalOptional(a.IsSuffix, b.IsSuffix)
}
func equalAffix(a, b *AffixData) bool { return a == b || (a != nil && b != nil && a.Equal(*b)) }
func metadataEqual(a, b DictWordMetadata) bool {
	return equalNoun(a.Noun, b.Noun) && equalPronoun(a.Pronoun, b.Pronoun) && equalVerb(a.Verb, b.Verb) && equalAdjective(a.Adjective, b.Adjective) && equalAdverb(a.Adverb, b.Adverb) && equalConjunction(a.Conjunction, b.Conjunction) && equalDeterminer(a.Determiner, b.Determiner) && equalAffix(a.Affix, b.Affix) && a.Preposition == b.Preposition && equalOptional(a.Swear, b.Swear) && equalOptional(a.Abbreviation, b.Abbreviation) && a.Dialects == b.Dialects && a.OrthInfo == b.OrthInfo && a.Common == b.Common && slices.Equal(a.DerivedFrom, b.DerivedFrom) && equalOptional(a.NpMember, b.NpMember) && equalOptional(a.PosTag, b.PosTag)
}
