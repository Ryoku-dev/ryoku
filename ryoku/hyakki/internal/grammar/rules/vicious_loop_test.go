package rules

import (
	"ryoku-rashin/internal/grammar/linttest"
	"testing"
)

func TestViciousLoop_ViciousSingular(t *testing.T) {
	linttest.AssertSuggestionResult(t, "vicious cycle", NewViciousCircle(), "vicious circle")
}

func TestViciousLoop_ViciousPlural(t *testing.T) {
	linttest.AssertSuggestionResult(t, "vicious cycles", NewViciousCircle(), "vicious circles")
}

func TestViciousLoop_ViscousSingular(t *testing.T) {
	linttest.AssertSuggestionResult(t, "viscous cycle", NewViciousCircle(), "vicious circle")
}

func TestViciousLoop_ViscousPlural(t *testing.T) {
	linttest.AssertSuggestionResult(t, "viscous cycles", NewViciousCircle(), "vicious circles")
}

func TestViciousLoop_IgnoreViciousSingular(t *testing.T) {
	linttest.AssertNoLints(t, "vicious circle", NewViciousCircle())
}

func TestViciousLoop_IgnoreVirtuousPlural(t *testing.T) {
	linttest.AssertNoLints(t, "virtuous circles", NewViciousCircle())
}

func TestViciousLoop_FixSingularAndPluralNouns(t *testing.T) {
	linttest.AssertSuggestionResult(t, "The file Vicious Cycle Dataset.ods contains 33 vicious cycles from 13 open source systems studied in our paper.", NewViciousCircle(), "The file Vicious Circle Dataset.ods contains 33 vicious circles from 13 open source systems studied in our paper.")
}

func TestViciousLoop_FixVirtuous(t *testing.T) {
	linttest.AssertSuggestionResult(t, "FlashInfer-Bench is a benchmark suite and production workflow designed to build a virtuous cycle of self-improving AI systems.", NewViciousCircle(), "FlashInfer-Bench is a benchmark suite and production workflow designed to build a virtuous circle of self-improving AI systems.")
}

func TestViciousLoop_FixSingular(t *testing.T) {
	linttest.AssertSuggestionResult(t, "vicious circle", NewViciousCycle(), "vicious cycle")
}

func TestViciousLoop_FixPlural(t *testing.T) {
	linttest.AssertSuggestionResult(t, "virtuous circles", NewViciousCycle(), "virtuous cycles")
}

func TestViciousLoop_FixViscousSingular(t *testing.T) {
	linttest.AssertSuggestionResult(t, "viscous circle", NewViciousCycle(), "vicious cycle")
}

func TestViciousLoop_FixViscousPlural(t *testing.T) {
	linttest.AssertSuggestionResult(t, "viscous circles", NewViciousCycle(), "vicious cycles")
}

func TestViciousLoop_DontFlagSingular(t *testing.T) {
	linttest.AssertNoLints(t, "viscious cycle", NewViciousCycle())
}

func TestViciousLoop_DontFlagPlural(t *testing.T) {
	linttest.AssertNoLints(t, "virtuous cycles", NewViciousCycle())
}

func TestViciousLoop_FixItsAVirtuous(t *testing.T) {
	linttest.AssertSuggestionResult(t, "It's a virtuous circle: if it's interesting to do a project, a person spends a lot of time on it", NewViciousCycle(), "It's a virtuous cycle: if it's interesting to do a project, a person spends a lot of time on it")
}

func TestViciousLoop_FixViscous(t *testing.T) {
	t.Skip("Harper currently misinterprets the words around the ellipses as a hostname")
	linttest.AssertSuggestionResult(t, "However, adding it to $connectionsToTransact causes the tests to stop running...viscous circle.", NewViciousCycle(), "However, adding it to $connectionsToTransact causes the tests to stop running...vicious cycle.")
}

func TestViciousLoop_DontFlagEither(t *testing.T) {
	linttest.AssertNoLints(t, "vicious circle, virtuous cycle, vicious cycles, virtuous circles", NewViciousCircleOrCycle())
}

func TestViciousLoop_FixBothViscous(t *testing.T) {
	linttest.AssertSuggestionResult(t, "viscous circle, viscous cycles", NewViciousCircleOrCycle(), "vicious circle, vicious cycles")
}

func TestViciousLoop_DontFlagCombo(t *testing.T) {
	linttest.AssertNoLints(t, "Instead of a vicious cycle, popularity creates a virtuous circle.", NewViciousCircleOrCycle())
}

func TestViciousLoop_FixItsAViscousCycle(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Its a viscous cycle that started back in 1.13 for a few plugins but is now hurting every single world generation plugin", NewViciousCircleOrCycle(), "Its a vicious cycle that started back in 1.13 for a few plugins but is now hurting every single world generation plugin")
}
