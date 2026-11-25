package dict

import "testing"

func testMD(t *testing.T, w string) DictWordMetadata {
	t.Helper()
	m := Curated().GetWordMetadataStr(w)
	if m == nil {
		t.Fatalf("word %q missing", w)
	}
	return *m
}
func TestMetadataPuppyIsNoun(t *testing.T) {

	if !(testMD(t, "puppy").IsNoun()) {
		t.Fatal("assertion failed")
	}

}

func TestMetadataPrepareIsNotNoun(t *testing.T) {

	if !(!testMD(t, "prepare").IsNoun()) {
		t.Fatal("assertion failed")
	}

}

func TestMetadataParisIsProperNoun(t *testing.T) {

	if !(testMD(t, "Paris").IsProperNoun()) {
		t.Fatal("assertion failed")
	}

}

func TestMetadataPermitIsNonProperNoun(t *testing.T) {

	if !(testMD(t, "lapdog").IsNonProperNoun()) {
		t.Fatal("assertion failed")
	}

}

func TestMetadataHoundIsSingularNoun(t *testing.T) {

	if !(testMD(t, "hound").IsSingularNoun()) {
		t.Fatal("assertion failed")
	}

}

func TestMetadataPoochesIsNonSingularNoun(t *testing.T) {

	if !(testMD(t, "pooches").IsNonSingularNoun()) {
		t.Fatal("assertion failed")
	}

}

func TestMetadataLoyalDoesntPassIsNonSingularNoun(t *testing.T) {

	if !(!testMD(t, "loyal").IsNonSingularNoun()) {
		t.Fatal("assertion failed")
	}

}

func TestMetadataHoundsIsPluralNoun(t *testing.T) {

	if !(testMD(t, "hounds").IsPluralNoun()) {
		t.Fatal("assertion failed")
	}

}

func TestMetadataPoochIsNonPluralNoun(t *testing.T) {

	if !(testMD(t, "pooch").IsNonPluralNoun()) {
		t.Fatal("assertion failed")
	}

}

func TestMetadataFishIsSingularNoun(t *testing.T) {

	if !(testMD(t, "fish").IsSingularNoun()) {
		t.Fatal("assertion failed")
	}

}

func TestMetadataFishIsPluralNoun(t *testing.T) {

	if !(testMD(t, "fish").IsPluralNoun()) {
		t.Fatal("assertion failed")
	}

}

func TestMetadataFishesIsPluralNoun(t *testing.T) {

	if !(testMD(t, "fishes").IsPluralNoun()) {
		t.Fatal("assertion failed")
	}

}

func TestMetadataSheepIsSingularNoun(t *testing.T) {

	if !(testMD(t, "sheep").IsSingularNoun()) {
		t.Fatal("assertion failed")
	}

}

func TestMetadataSheepIsPluralNoun(t *testing.T) {

	if !(testMD(t, "sheep").IsPluralNoun()) {
		t.Fatal("assertion failed")
	}

}

func TestMetadataSheepsIsNotWord(t *testing.T) {
	if Curated().GetWordMetadataStr("sheeps") != nil {
		t.Fatal("expected absent word")
	}
}

func TestMetadataBicepIsSingularNoun(t *testing.T) {

	if !(testMD(t, "bicep").IsSingularNoun()) {
		t.Fatal("assertion failed")
	}

}

func TestMetadataBicepsIsSingularNoun(t *testing.T) {

	if !(testMD(t, "biceps").IsSingularNoun()) {
		t.Fatal("assertion failed")
	}

}

func TestMetadataBicepsIsPluralNoun(t *testing.T) {

	if !(testMD(t, "biceps").IsPluralNoun()) {
		t.Fatal("assertion failed")
	}

}

func TestMetadataAircraftIsSingularNoun(t *testing.T) {

	if !(testMD(t, "aircraft").IsSingularNoun()) {
		t.Fatal("assertion failed")
	}

}

func TestMetadataAircraftIsPluralNoun(t *testing.T) {

	if !(testMD(t, "aircraft").IsPluralNoun()) {
		t.Fatal("assertion failed")
	}

}

func TestMetadataAircraftsIsNotWord(t *testing.T) {
	if Curated().GetWordMetadataStr("aircrafts") != nil {
		t.Fatal("expected absent word")
	}
}

func TestMetadataDogApostropheSIsPossessiveNoun(t *testing.T) {

	if !(testMD(t, "dog's").IsPossessiveNoun()) {
		t.Fatal("assertion failed")
	}

}

func TestMetadataDogsIsNonPossessiveNoun(t *testing.T) {

	if !(testMD(t, "dogs").IsNonPossessiveNoun()) {
		t.Fatal("assertion failed")
	}

}

func TestMetadataDogIsCountable(t *testing.T) {

	if !(testMD(t, "dog").IsCountableNoun()) {
		t.Fatal("assertion failed")
	}

}

func TestMetadataDogIsNonMassNoun(t *testing.T) {

	if !(testMD(t, "dog").IsNonMassNoun()) {
		t.Fatal("assertion failed")
	}

}

func TestMetadataFurnitureIsMassNoun(t *testing.T) {

	if !(testMD(t, "furniture").IsMassNoun()) {
		t.Fatal("assertion failed")
	}

}

func TestMetadataFurnitureIsNonCountableNoun(t *testing.T) {

	if !(testMD(t, "furniture").IsNonCountableNoun()) {
		t.Fatal("assertion failed")
	}

}

func TestMetadataEquipmentIsMassNoun(t *testing.T) {

	if !(testMD(t, "equipment").IsMassNoun()) {
		t.Fatal("assertion failed")
	}

}

func TestMetadataEquipmentIsNonCountableNoun(t *testing.T) {

	if !(testMD(t, "equipment").IsNonCountableNoun()) {
		t.Fatal("assertion failed")
	}

}

func TestMetadataBeerIsCountableNoun(t *testing.T) {

	if !(testMD(t, "beer").IsCountableNoun()) {
		t.Fatal("assertion failed")
	}

}

func TestMetadataBeerIsMassNoun(t *testing.T) {

	if !(testMD(t, "beer").IsMassNoun()) {
		t.Fatal("assertion failed")
	}

}

func TestMetadataIIsPronoun(t *testing.T) {

	if !(testMD(t, "I").IsPronoun()) {
		t.Fatal("assertion failed")
	}

}

func TestMetadataIIsPersonalPronoun(t *testing.T) {

	if !(testMD(t, "I").IsPersonalPronoun()) {
		t.Fatal("assertion failed")
	}

}

func TestMetadataIIsSingularPronoun(t *testing.T) {

	if !(testMD(t, "I").IsSingularPronoun()) {
		t.Fatal("assertion failed")
	}

}

func TestMetadataIIsSubjectPronoun(t *testing.T) {

	if !(testMD(t, "I").IsSubjectPronoun()) {
		t.Fatal("assertion failed")
	}

}

func TestMetadataMeIsPronoun(t *testing.T) {

	if !(testMD(t, "me").IsPronoun()) {
		t.Fatal("assertion failed")
	}

}

func TestMetadataMeIsPersonalPronoun(t *testing.T) {

	if !(testMD(t, "me").IsPersonalPronoun()) {
		t.Fatal("assertion failed")
	}

}

func TestMetadataMeIsSingularPronoun(t *testing.T) {

	if !(testMD(t, "me").IsSingularPronoun()) {
		t.Fatal("assertion failed")
	}

}

func TestMetadataMeIsObjectPronoun(t *testing.T) {

	if !(testMD(t, "me").IsObjectPronoun()) {
		t.Fatal("assertion failed")
	}

}

func TestMetadataMyselfIsPronoun(t *testing.T) {

	if !(testMD(t, "myself").IsPronoun()) {
		t.Fatal("assertion failed")
	}

}

func TestMetadataMyselfIsPersonalPronoun(t *testing.T) {

	if !(testMD(t, "myself").IsPersonalPronoun()) {
		t.Fatal("assertion failed")
	}

}

func TestMetadataMyselfIsSingularPronoun(t *testing.T) {

	if !(testMD(t, "myself").IsSingularPronoun()) {
		t.Fatal("assertion failed")
	}

}

func TestMetadataMyselfIsReflexivePronoun(t *testing.T) {

	if !(testMD(t, "myself").IsReflexivePronoun()) {
		t.Fatal("assertion failed")
	}

}

func TestMetadataWeIsPronoun(t *testing.T) {

	if !(testMD(t, "we").IsPronoun()) {
		t.Fatal("assertion failed")
	}

}

func TestMetadataWeIsPersonalPronoun(t *testing.T) {

	if !(testMD(t, "we").IsPersonalPronoun()) {
		t.Fatal("assertion failed")
	}

}

func TestMetadataWeIsPluralPronoun(t *testing.T) {

	if !(testMD(t, "we").IsPluralPronoun()) {
		t.Fatal("assertion failed")
	}

}

func TestMetadataWeIsSubjectPronoun(t *testing.T) {

	if !(testMD(t, "we").IsSubjectPronoun()) {
		t.Fatal("assertion failed")
	}

}

func TestMetadataUsIsPronoun(t *testing.T) {

	if !(testMD(t, "us").IsPronoun()) {
		t.Fatal("assertion failed")
	}

}

func TestMetadataUsIsPersonalPronoun(t *testing.T) {

	if !(testMD(t, "us").IsPersonalPronoun()) {
		t.Fatal("assertion failed")
	}

}

func TestMetadataUsIsPluralPronoun(t *testing.T) {

	if !(testMD(t, "us").IsPluralPronoun()) {
		t.Fatal("assertion failed")
	}

}

func TestMetadataUsIsObjectPronoun(t *testing.T) {

	if !(testMD(t, "us").IsObjectPronoun()) {
		t.Fatal("assertion failed")
	}

}

func TestMetadataOurselvesIsPronoun(t *testing.T) {

	if !(testMD(t, "ourselves").IsPronoun()) {
		t.Fatal("assertion failed")
	}

}

func TestMetadataOurselvesIsPersonalPronoun(t *testing.T) {

	if !(testMD(t, "ourselves").IsPersonalPronoun()) {
		t.Fatal("assertion failed")
	}

}

func TestMetadataOurselvesIsPluralPronoun(t *testing.T) {

	if !(testMD(t, "ourselves").IsPluralPronoun()) {
		t.Fatal("assertion failed")
	}

}

func TestMetadataOurselvesIsReflexivePronoun(t *testing.T) {

	if !(testMD(t, "ourselves").IsReflexivePronoun()) {
		t.Fatal("assertion failed")
	}

}

func TestMetadataYouIsPronoun(t *testing.T) {

	if !(testMD(t, "you").IsPronoun()) {
		t.Fatal("assertion failed")
	}

}

func TestMetadataYouIsPersonalPronoun(t *testing.T) {

	if !(testMD(t, "you").IsPersonalPronoun()) {
		t.Fatal("assertion failed")
	}

}

func TestMetadataYouIsSingularPronoun(t *testing.T) {

	if !(testMD(t, "you").IsSingularPronoun()) {
		t.Fatal("assertion failed")
	}

}

func TestMetadataYouIsPluralPronoun(t *testing.T) {

	if !(testMD(t, "you").IsPluralPronoun()) {
		t.Fatal("assertion failed")
	}

}

func TestMetadataYouIsSubjectPronoun(t *testing.T) {

	if !(testMD(t, "you").IsSubjectPronoun()) {
		t.Fatal("assertion failed")
	}

}

func TestMetadataYouIsObjectPronoun(t *testing.T) {

	if !(testMD(t, "you").IsObjectPronoun()) {
		t.Fatal("assertion failed")
	}

}

func TestMetadataYourselfIsPronoun(t *testing.T) {

	if !(testMD(t, "yourself").IsPronoun()) {
		t.Fatal("assertion failed")
	}

}

func TestMetadataYourselfIsPersonalPronoun(t *testing.T) {

	if !(testMD(t, "yourself").IsPersonalPronoun()) {
		t.Fatal("assertion failed")
	}

}

func TestMetadataYourselfIsSingularPronoun(t *testing.T) {

	if !(testMD(t, "yourself").IsSingularPronoun()) {
		t.Fatal("assertion failed")
	}

}

func TestMetadataYourselfIsReflexivePronoun(t *testing.T) {

	if !(testMD(t, "yourself").IsReflexivePronoun()) {
		t.Fatal("assertion failed")
	}

}

func TestMetadataHeIsPronoun(t *testing.T) {

	if !(testMD(t, "he").IsPronoun()) {
		t.Fatal("assertion failed")
	}

}

func TestMetadataHeIsPersonalPronoun(t *testing.T) {

	if !(testMD(t, "he").IsPersonalPronoun()) {
		t.Fatal("assertion failed")
	}

}

func TestMetadataHeIsSingularPronoun(t *testing.T) {

	if !(testMD(t, "he").IsSingularPronoun()) {
		t.Fatal("assertion failed")
	}

}

func TestMetadataHeIsSubjectPronoun(t *testing.T) {

	if !(testMD(t, "he").IsSubjectPronoun()) {
		t.Fatal("assertion failed")
	}

}

func TestMetadataHimIsPronoun(t *testing.T) {

	if !(testMD(t, "him").IsPronoun()) {
		t.Fatal("assertion failed")
	}

}

func TestMetadataHimIsPersonalPronoun(t *testing.T) {

	if !(testMD(t, "him").IsPersonalPronoun()) {
		t.Fatal("assertion failed")
	}

}

func TestMetadataHimIsSingularPronoun(t *testing.T) {

	if !(testMD(t, "him").IsSingularPronoun()) {
		t.Fatal("assertion failed")
	}

}

func TestMetadataHimIsObjectPronoun(t *testing.T) {

	if !(testMD(t, "him").IsObjectPronoun()) {
		t.Fatal("assertion failed")
	}

}

func TestMetadataHimselfIsPronoun(t *testing.T) {

	if !(testMD(t, "himself").IsPronoun()) {
		t.Fatal("assertion failed")
	}

}

func TestMetadataHimselfIsPersonalPronoun(t *testing.T) {

	if !(testMD(t, "himself").IsPersonalPronoun()) {
		t.Fatal("assertion failed")
	}

}

func TestMetadataHimselfIsSingularPronoun(t *testing.T) {

	if !(testMD(t, "himself").IsSingularPronoun()) {
		t.Fatal("assertion failed")
	}

}

func TestMetadataHimselfIsReflexivePronoun(t *testing.T) {

	if !(testMD(t, "himself").IsReflexivePronoun()) {
		t.Fatal("assertion failed")
	}

}

func TestMetadataSheIsPronoun(t *testing.T) {

	if !(testMD(t, "she").IsPronoun()) {
		t.Fatal("assertion failed")
	}

}

func TestMetadataSheIsPersonalPronoun(t *testing.T) {

	if !(testMD(t, "she").IsPersonalPronoun()) {
		t.Fatal("assertion failed")
	}

}

func TestMetadataSheIsSingularPronoun(t *testing.T) {

	if !(testMD(t, "she").IsSingularPronoun()) {
		t.Fatal("assertion failed")
	}

}

func TestMetadataSheIsSubjectPronoun(t *testing.T) {

	if !(testMD(t, "she").IsSubjectPronoun()) {
		t.Fatal("assertion failed")
	}

}

func TestMetadataHerIsPronoun(t *testing.T) {

	if !(testMD(t, "her").IsPronoun()) {
		t.Fatal("assertion failed")
	}

}

func TestMetadataHerIsPersonalPronoun(t *testing.T) {

	if !(testMD(t, "her").IsPersonalPronoun()) {
		t.Fatal("assertion failed")
	}

}

func TestMetadataHerIsSingularPronoun(t *testing.T) {

	if !(testMD(t, "her").IsSingularPronoun()) {
		t.Fatal("assertion failed")
	}

}

func TestMetadataHerIsObjectPronoun(t *testing.T) {

	if !(testMD(t, "her").IsObjectPronoun()) {
		t.Fatal("assertion failed")
	}

}

func TestMetadataHerselfIsPronoun(t *testing.T) {

	if !(testMD(t, "herself").IsPronoun()) {
		t.Fatal("assertion failed")
	}

}

func TestMetadataHerselfIsPersonalPronoun(t *testing.T) {

	if !(testMD(t, "herself").IsPersonalPronoun()) {
		t.Fatal("assertion failed")
	}

}

func TestMetadataHerselfIsSingularPronoun(t *testing.T) {

	if !(testMD(t, "herself").IsSingularPronoun()) {
		t.Fatal("assertion failed")
	}

}

func TestMetadataHerselfIsReflexivePronoun(t *testing.T) {

	if !(testMD(t, "herself").IsReflexivePronoun()) {
		t.Fatal("assertion failed")
	}

}

func TestMetadataItIsPronoun(t *testing.T) {

	if !(testMD(t, "it").IsPronoun()) {
		t.Fatal("assertion failed")
	}

}

func TestMetadataItIsPersonalPronoun(t *testing.T) {

	if !(testMD(t, "it").IsPersonalPronoun()) {
		t.Fatal("assertion failed")
	}

}

func TestMetadataItIsSingularPronoun(t *testing.T) {

	if !(testMD(t, "it").IsSingularPronoun()) {
		t.Fatal("assertion failed")
	}

}

func TestMetadataItIsSubjectPronoun(t *testing.T) {

	if !(testMD(t, "it").IsSubjectPronoun()) {
		t.Fatal("assertion failed")
	}

}

func TestMetadataItIsObjectPronoun(t *testing.T) {

	if !(testMD(t, "it").IsObjectPronoun()) {
		t.Fatal("assertion failed")
	}

}

func TestMetadataItselfIsPronoun(t *testing.T) {

	if !(testMD(t, "itself").IsPronoun()) {
		t.Fatal("assertion failed")
	}

}

func TestMetadataItselfIsPersonalPronoun(t *testing.T) {

	if !(testMD(t, "itself").IsPersonalPronoun()) {
		t.Fatal("assertion failed")
	}

}

func TestMetadataItselfIsSingularPronoun(t *testing.T) {

	if !(testMD(t, "itself").IsSingularPronoun()) {
		t.Fatal("assertion failed")
	}

}

func TestMetadataItselfIsReflexivePronoun(t *testing.T) {

	if !(testMD(t, "itself").IsReflexivePronoun()) {
		t.Fatal("assertion failed")
	}

}

func TestMetadataTheyIsPronoun(t *testing.T) {

	if !(testMD(t, "they").IsPronoun()) {
		t.Fatal("assertion failed")
	}

}

func TestMetadataTheyIsPersonalPronoun(t *testing.T) {

	if !(testMD(t, "they").IsPersonalPronoun()) {
		t.Fatal("assertion failed")
	}

}

func TestMetadataTheyIsPluralPronoun(t *testing.T) {

	if !(testMD(t, "they").IsPluralPronoun()) {
		t.Fatal("assertion failed")
	}

}

func TestMetadataTheyIsSubjectPronoun(t *testing.T) {

	if !(testMD(t, "they").IsSubjectPronoun()) {
		t.Fatal("assertion failed")
	}

}

func TestMetadataThemIsPronoun(t *testing.T) {

	if !(testMD(t, "them").IsPronoun()) {
		t.Fatal("assertion failed")
	}

}

func TestMetadataThemIsPersonalPronoun(t *testing.T) {

	if !(testMD(t, "them").IsPersonalPronoun()) {
		t.Fatal("assertion failed")
	}

}

func TestMetadataThemIsPluralPronoun(t *testing.T) {

	if !(testMD(t, "them").IsPluralPronoun()) {
		t.Fatal("assertion failed")
	}

}

func TestMetadataThemIsObjectPronoun(t *testing.T) {

	if !(testMD(t, "them").IsObjectPronoun()) {
		t.Fatal("assertion failed")
	}

}

func TestMetadataThemselvesIsPronoun(t *testing.T) {

	if !(testMD(t, "themselves").IsPronoun()) {
		t.Fatal("assertion failed")
	}

}

func TestMetadataThemselvesIsPersonalPronoun(t *testing.T) {

	if !(testMD(t, "themselves").IsPersonalPronoun()) {
		t.Fatal("assertion failed")
	}

}

func TestMetadataThemselvesIsPluralPronoun(t *testing.T) {

	if !(testMD(t, "themselves").IsPluralPronoun()) {
		t.Fatal("assertion failed")
	}

}

func TestMetadataThemselvesIsReflexivePronoun(t *testing.T) {

	if !(testMD(t, "themselves").IsReflexivePronoun()) {
		t.Fatal("assertion failed")
	}

}

func TestMetadataMineIsPronoun(t *testing.T) {

	if !(testMD(t, "mine").IsPronoun()) {
		t.Fatal("assertion failed")
	}

}

func TestMetadataOursIsPronoun(t *testing.T) {

	if !(testMD(t, "ours").IsPronoun()) {
		t.Fatal("assertion failed")
	}

}

func TestMetadataYoursIsPronoun(t *testing.T) {

	if !(testMD(t, "yours").IsPronoun()) {
		t.Fatal("assertion failed")
	}

}

func TestMetadataHisIsPronoun(t *testing.T) {

	if !(testMD(t, "his").IsPronoun()) {
		t.Fatal("assertion failed")
	}

}

func TestMetadataHersIsPronoun(t *testing.T) {

	if !(testMD(t, "hers").IsPronoun()) {
		t.Fatal("assertion failed")
	}

}

func TestMetadataItsIsPronoun(t *testing.T) {

	if !(testMD(t, "its").IsPronoun()) {
		t.Fatal("assertion failed")
	}

}

func TestMetadataTheirsIsPronoun(t *testing.T) {

	if !(testMD(t, "theirs").IsPronoun()) {
		t.Fatal("assertion failed")
	}

}

func TestMetadataArchaicPronouns(t *testing.T) {

	if !(testMD(t, "thou").IsPronoun()) {
		t.Fatal("assertion failed")
	}
	if !(testMD(t, "thee").IsPronoun()) {
		t.Fatal("assertion failed")
	}
	if !(testMD(t, "thyself").IsPronoun()) {
		t.Fatal("assertion failed")
	}
	if !(testMD(t, "thine").IsPronoun()) {
		t.Fatal("assertion failed")
	}

}

func TestMetadataGenericPronouns(t *testing.T) {

	if !(testMD(t, "one").IsPronoun()) {
		t.Fatal("assertion failed")
	}
	if !(testMD(t, "oneself").IsPronoun()) {
		t.Fatal("assertion failed")
	}

}

func TestMetadataRelativeAndInterrogativePronouns(t *testing.T) {

	if !(testMD(t, "who").IsPronoun()) {
		t.Fatal("assertion failed")
	}
	if !(testMD(t, "whom").IsPronoun()) {
		t.Fatal("assertion failed")
	}
	if !(testMD(t, "whose").IsPronoun()) {
		t.Fatal("assertion failed")
	}
	if !(testMD(t, "which").IsPronoun()) {
		t.Fatal("assertion failed")
	}
	if !(testMD(t, "what").IsPronoun()) {
		t.Fatal("assertion failed")
	}

}

func TestMetadataNonstandardPronouns(t *testing.T) {
	t.Skip("not in dictionary")

	if !(testMD(t, "themself").Pronoun != nil) {
		t.Fatal("assertion failed")
	}
	if !(testMD(t, "y'all'").Pronoun != nil) {
		t.Fatal("assertion failed")
	}

}

func TestMetadataMyIsPossessiveNominal(t *testing.T) {

	if !(testMD(t, "my").IsPossessiveNominal()) {
		t.Fatal("assertion failed")
	}

}

func TestMetadataMineIsNotPossessiveNominal(t *testing.T) {

	if !(!testMD(t, "mine").IsPossessiveNominal()) {
		t.Fatal("assertion failed")
	}

}

func TestMetadataFredsIsPossessiveNominal(t *testing.T) {

	if !(testMD(t, "Fred's").IsPossessiveNominal()) {
		t.Fatal("assertion failed")
	}

}

func TestMetadataFredIsNotPossessiveNominal(t *testing.T) {

	if !(!testMD(t, "Fred").IsPossessiveNominal()) {
		t.Fatal("assertion failed")
	}

}

func TestMetadataDogsIsPossessiveNominal(t *testing.T) {

	if !(testMD(t, "dog's").IsPossessiveNominal()) {
		t.Fatal("assertion failed")
	}

}

func TestMetadataMicrosoftsIsPossessiveNominal(t *testing.T) {

	if !(testMD(t, "Microsoft's").IsPossessiveNominal()) {
		t.Fatal("assertion failed")
	}

}

func TestMetadataBigIsPositive(t *testing.T) {
	t.Skip("not marked yet because it might not be reliable")

	if degree, ok := testMD(t, "big").GetDegree(); !ok || degree != Positive {
		t.Fatal("degree mismatch")
	}

}

func TestMetadataBiggerIsComparative(t *testing.T) {

	if degree, ok := testMD(t, "bigger").GetDegree(); !ok || degree != Comparative {
		t.Fatal("degree mismatch")
	}

}

func TestMetadataBiggestIsSuperlative(t *testing.T) {

	if degree, ok := testMD(t, "biggest").GetDegree(); !ok || degree != Superlative {
		t.Fatal("degree mismatch")
	}

}

func TestMetadataBiglyIsNotAnAdjectiveFormWeTrack(t *testing.T) {
	if Curated().GetWordMetadataStr("bigly") != nil {
		t.Fatal("expected absent word")
	}
}

func TestMetadataBiggerIsComparativeAdjective(t *testing.T) {

	if !(testMD(t, "bigger").IsComparativeAdjective()) {
		t.Fatal("assertion failed")
	}

}

func TestMetadataBiggestIsSuperlativeAdjective(t *testing.T) {

	if !(testMD(t, "biggest").IsSuperlativeAdjective()) {
		t.Fatal("assertion failed")
	}

}

func TestMetadataTheIsDeterminer(t *testing.T) {

	if !(testMD(t, "the").IsDeterminer()) {
		t.Fatal("assertion failed")
	}

}

func TestMetadataThisIsDemonstrativeDeterminer(t *testing.T) {

	if !(testMD(t, "this").IsDemonstrativeDeterminer()) {
		t.Fatal("assertion failed")
	}

}

func TestMetadataYourIsPossessiveDeterminer(t *testing.T) {

	if !(testMD(t, "your").IsPossessiveDeterminer()) {
		t.Fatal("assertion failed")
	}

}

func TestMetadataEveryIsQuantifier(t *testing.T) {

	if !(testMD(t, "every").IsQuantifier()) {
		t.Fatal("assertion failed")
	}

}

func TestMetadataTheIsntQuantifier(t *testing.T) {

	if !(!testMD(t, "the").IsQuantifier()) {
		t.Fatal("assertion failed")
	}

}

func TestMetadataRootEquipmentIsMassNoun(t *testing.T) {

	if !(testMD(t, "equipment").IsMassNoun()) {
		t.Fatal("assertion failed")
	}

}

func TestMetadataRootEquipmentIsNonCountableNoun(t *testing.T) {

	if !(testMD(t, "equipment").IsNonCountableNoun()) {
		t.Fatal("assertion failed")
	}

}

func TestMetadataEquipmentIsntCountableNoun(t *testing.T) {

	if !(!testMD(t, "equipment").IsCountableNoun()) {
		t.Fatal("assertion failed")
	}

}

func TestMetadataInfrastructureIsMassNounOnly(t *testing.T) {

	if !(testMD(t, "infrastructure").IsMassNounOnly()) {
		t.Fatal("assertion failed")
	}

}

func TestMetadataBeerIsNotMassNounOnly(t *testing.T) {

	if !(!testMD(t, "beer").IsMassNounOnly()) {
		t.Fatal("assertion failed")
	}

}

func TestMetadataSheepIsNotSingularOnly(t *testing.T) {

	if !(!testMD(t, "sheep").IsSingularNounOnly()) {
		t.Fatal("assertion failed")
	}

}

func TestMetadataSheepIsNotPluralOnly(t *testing.T) {

	if !(!testMD(t, "sheep").IsPluralNounOnly()) {
		t.Fatal("assertion failed")
	}

}

func TestMetadataOxIsSingularOnly(t *testing.T) {

	if !(testMD(t, "ox").IsSingularNounOnly()) {
		t.Fatal("assertion failed")
	}

}

func TestMetadataOxenIsPluralOnly(t *testing.T) {

	if !(testMD(t, "oxen").IsPluralNounOnly()) {
		t.Fatal("assertion failed")
	}

}

func TestMetadataLemmaWalk(t *testing.T) {

	m := testMD(t, "walk")
	if !(m.IsVerbLemma()) {
		t.Fatal("assertion failed")
	}

}

func TestMetadataLemmaFix(t *testing.T) {

	m := testMD(t, "fix")
	if !(m.IsVerbLemma()) {
		t.Fatal("assertion failed")
	}

}

func TestMetadataProgressiveWalking(t *testing.T) {

	m := testMD(t, "walking")
	if !(m.IsVerbProgressiveForm()) {
		t.Fatal("assertion failed")
	}

}

func TestMetadataPastWalked(t *testing.T) {

	m := testMD(t, "walked")
	if !(m.IsVerbPastForm()) {
		t.Fatal("assertion failed")
	}

}

func TestMetadataRegularPastThought(t *testing.T) {

	m := testMD(t, "thought")
	if !(m.IsVerbPreteriteAndParticipleForm()) {
		t.Fatal("assertion failed")
	}

}

func TestMetadataSimplePastAte(t *testing.T) {

	m := testMD(t, "ate")
	if !(m.IsVerbSimplePastForm()) {
		t.Fatal("assertion failed")
	}

}

func TestMetadataPastParticipleEaten(t *testing.T) {

	m := testMD(t, "eaten")
	if !(m.IsVerbPastParticipleForm()) {
		t.Fatal("assertion failed")
	}

}

func TestMetadataAteIsSimplePastOnly(t *testing.T) {

	m := testMD(t, "ate")
	if !(m.IsVerbSimplePastOnly()) {
		t.Fatal("assertion failed")
	}
	if !(!m.IsVerbPastParticipleOnly()) {
		t.Fatal("assertion failed")
	}

}

func TestMetadataEatenIsPastParticipleOnly(t *testing.T) {

	m := testMD(t, "eaten")
	if !(m.IsVerbPastParticipleOnly()) {
		t.Fatal("assertion failed")
	}
	if !(!m.IsVerbSimplePastOnly()) {
		t.Fatal("assertion failed")
	}

}

func TestMetadataThoughtIsNeitherPastFormOnly(t *testing.T) {

	m := testMD(t, "thought")
	if !(!m.IsVerbSimplePastOnly()) {
		t.Fatal("assertion failed")
	}
	if !(!m.IsVerbPastParticipleOnly()) {
		t.Fatal("assertion failed")
	}

}

func TestMetadataSharedPastFormsAreNeitherPastFormOnly(t *testing.T) {

	m := testMD(t, "thought")
	if !(!m.IsVerbSimplePastOnly()) {
		t.Fatal("assertion failed")
	}
	if !(!m.IsVerbPastParticipleOnly()) {
		t.Fatal("assertion failed")
	}
	if !(m.IsVerbPreteriteAndParticipleForm()) {
		t.Fatal("assertion failed")
	}

}

func TestMetadataDistinctPastFormsAreNotRegularPast(t *testing.T) {

	if !(!testMD(t, "ate").IsVerbPreteriteAndParticipleForm()) {
		t.Fatal("assertion failed")
	}
	if !(!testMD(t, "eaten").IsVerbPreteriteAndParticipleForm()) {
		t.Fatal("assertion failed")
	}
	if !(!testMD(t, "walked").IsVerbPreteriteAndParticipleForm()) {
		t.Fatal("assertion failed")
	}

}

func TestMetadataThirdPersSingWalks(t *testing.T) {

	m := testMD(t, "walks")
	if !(m.IsVerbThirdPersonSingularPresentForm()) {
		t.Fatal("assertion failed")
	}

}
