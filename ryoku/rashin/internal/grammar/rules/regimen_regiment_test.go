package rules

import (
	"ryoku-rashin/internal/grammar/linttest"
	"testing"
)

func TestRegimenRegiment_FixChemotherapeuticRegimentsCaps(t *testing.T) {
	linttest.AssertSuggestionResult(t, "ANALYSIS OF CHEMOTHERAPEUTIC REGIMENTS FOR THE MANAGEMENT OF PTLD.", NewRegimenRegiment(), "ANALYSIS OF CHEMOTHERAPEUTIC REGIMENS FOR THE MANAGEMENT OF PTLD.")
}

func TestRegimenRegiment_FixChemotherapyRegiments(t *testing.T) {
	linttest.AssertSuggestionResult(t, "ICER was counted to summarize the cost-effectiveness of two chemotherapy regiments (Taxane versus FAC).", NewRegimenRegiment(), "ICER was counted to summarize the cost-effectiveness of two chemotherapy regimens (Taxane versus FAC).")
}

func TestRegimenRegiment_FixChemotherapyRegiment(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Comparison of four chemotherapy regiment for advanced nonsmall cell lung cancer.", NewRegimenRegiment(), "Comparison of four chemotherapy regimen for advanced nonsmall cell lung cancer.")
}

func TestRegimenRegiment_FixDoseRegiment(t *testing.T) {
	linttest.AssertSuggestionResult(t, "P110-4 Low compared to high dose regiment of induction chemotherapy in nasopharingeal cancer: a systematic review meta-analysis.", NewRegimenRegiment(), "P110-4 Low compared to high dose regimen of induction chemotherapy in nasopharingeal cancer: a systematic review meta-analysis.")
}

func TestRegimenRegiment_FixDosingRegiment(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Dr Lisette Rozeman speaks with ecancer at ESMO 2018 in Munich about the study into a more tolerable dosing regiment of the combination of ...", NewRegimenRegiment(), "Dr Lisette Rozeman speaks with ecancer at ESMO 2018 in Munich about the study into a more tolerable dosing regimen of the combination of ...")
}

func TestRegimenRegiment_FixDrugRegiment(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Three drug regiment (Irinotecan, Carboplatin and Etoposide) as a front line treatment in extensive disease small-cell lung cancer: phase II trial.", NewRegimenRegiment(), "Three drug regimen (Irinotecan, Carboplatin and Etoposide) as a front line treatment in extensive disease small-cell lung cancer: phase II trial.")
}

func TestRegimenRegiment_FixTherapeuticRegiment(t *testing.T) {
	linttest.AssertSuggestionResult(t, "A Triple Therapeutic Regiment Consisted of Colchicine, Thalidomide and Total Glucosides of Paeony Is Effective and Well‐Tolerated for", NewRegimenRegiment(), "A Triple Therapeutic Regimen Consisted of Colchicine, Thalidomide and Total Glucosides of Paeony Is Effective and Well‐Tolerated for")
}

func TestRegimenRegiment_DontFlagArtilleryRegiments(t *testing.T) {
	linttest.AssertNoLints(t, "Artillery Regiments with Minden associations (see below) also wear red roses.", NewRegimenRegiment())
}

func TestRegimenRegiment_DontFlagCavalryRegiments(t *testing.T) {
	linttest.AssertNoLints(t, "Army—Cavalry Regiments For India—Question", NewRegimenRegiment())
}
