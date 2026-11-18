package rules

import (
	"ryoku-rashin/internal/grammar/linttest"
	"testing"
)

func TestDenyOffer_FixDenyTheOffer(t *testing.T) {
	linttest.AssertSuggestionResult(t, "I did not want to accept or deny the offer without floating to other maintainers", NewDenyOffer(), "I did not want to accept or decline the offer without floating to other maintainers")
}

func TestDenyOffer_DenyYourOffer(t *testing.T) {
	linttest.AssertSuggestionResult(t, "I understand, why you'd want this, but must deny your offer to expand my participation in your project quite so dramatically at this time.", NewDenyOffer(), "I understand, why you'd want this, but must reject your offer to expand my participation in your project quite so dramatically at this time.")
}

func TestDenyOffer_DeniedTheirOffer(t *testing.T) {
	linttest.AssertSuggestionResult(t, "So I denied their offer of employment and went back to finish my masters degree", NewDenyOffer(), "So I declined their offer of employment and went back to finish my masters degree")
}

func TestDenyOffer_DyingOffers(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Seemingly denying an offer is translated as accepting it", NewDenyOffer(), "Seemingly rejecting an offer is translated as accepting it")
}

func TestDenyOffer_DenyOffersAndDenyingOffers(t *testing.T) {
	linttest.AssertSuggestionResult(t, "For details about how a Marketplace administrator can deny offers in the Back Office, see Approving or denying offers.", NewDenyOffer(), "For details about how a Marketplace administrator can decline offers in the Back Office, see Approving or declining offers.")
}
