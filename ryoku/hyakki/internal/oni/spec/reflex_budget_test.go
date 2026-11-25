package spec_test

import (
	"testing"

	"ryoku-rashin/internal/oni/spec"
	"ryoku-rashin/internal/protocol"
)

// The reflex decision budget is the same default the decision route uses,
// so a reflex decision never asks for more time than a Settings one.
func TestReflexDecisionBudgetIsTheRouteDefault(t *testing.T) {
	s, err := spec.Load()
	if err != nil {
		t.Fatal(err)
	}
	if got := s.Reflex.Decisions.BudgetMs; got != protocol.DefaultDecisionBudgetMS {
		t.Fatalf("reflex.decisions.budgetMs = %d, the decision route default is %d", got, protocol.DefaultDecisionBudgetMS)
	}
}
