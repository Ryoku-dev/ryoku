package spec_test

import (
	"testing"

	"ryoku-rashin/internal/oni/feature"
	"ryoku-rashin/internal/oni/spec"
)

// A Hands consent card lives exactly as long as every other consent card,
// so the card the character draws and the policy's expiry never disagree.
func TestHandsConsentLifetimeIsTheCardLifetime(t *testing.T) {
	s, err := spec.Load()
	if err != nil {
		t.Fatal(err)
	}
	if got := s.Hands.ConsentTTL(); got != feature.ConsentTTL {
		t.Fatalf("hands.consentTtlS = %v, the consent card lifetime is %v", got, feature.ConsentTTL)
	}
}
