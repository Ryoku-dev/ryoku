package rules

import (
	"ryoku-rashin/internal/grammar/linttest"
	"testing"
)

func TestExpandPeople_FixSomePeople(t *testing.T) {
	linttest.AssertSuggestionResult(t, "some ppl told my this problem from (ImGui_ImplWin32_WndProcHandler) and u need to add (WM_SIZE) but i don't know what should i do now :(", NewExpandPeople(), "some people told my this problem from (ImGui_ImplWin32_WndProcHandler) and u need to add (WM_SIZE) but i don't know what should i do now :(")
}

func TestExpandPeople_FixAllPeople(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Hi all, maybe all ppl with some experience on R, would know is not easy to debug or work with a language where there is no checks on types", NewExpandPeople(), "Hi all, maybe all people with some experience on R, would know is not easy to debug or work with a language where there is no checks on types")
}

func TestExpandPeople_DontFlagProtectedProcessLight(t *testing.T) {
	linttest.AssertNoLints(t, "Processes started as an Anti Malware 'Protected Process-Light' (PPL) are restricted in what they can do, can only load signed code, but cannot be debugged", NewExpandPeople())
}

func TestExpandPeople_DontFlagPairedPointLifting(t *testing.T) {
	linttest.AssertNoLints(t, "In this work, we present an alternative lightweight strategy called Paired-Point Lifting (PPL) for constructing 3D line clouds.", NewExpandPeople())
}
