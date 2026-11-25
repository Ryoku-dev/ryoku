package rules

import (
	"ryoku-rashin/internal/grammar/linttest"
	"testing"
)

func TestBackInTheDay_DetectsGemUpdateCase(t *testing.T) {
	linttest.AssertSuggestionResult(t, "... has been resolved through a gem update back in the days", NewBackInTheDay(), "... has been resolved through a gem update back in the day")
}

func TestBackInTheDay_DetectsInstallCase(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Back in the days we're used to install it directly from ...", NewBackInTheDay(), "Back in the day we're used to install it directly from ...")
}

func TestBackInTheDay_DetectsComposerJsonCase(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Back in the days there was only composer.json and ...", NewBackInTheDay(), "Back in the day there was only composer.json and ...")
}

func TestBackInTheDay_DetectsVersionReleaseCase(t *testing.T) {
	linttest.AssertSuggestionResult(t, "... should have been released back in the days in a version 11", NewBackInTheDay(), "... should have been released back in the day in a version 11")
}

func TestBackInTheDay_AvoidsFalsePositiveSpringfox(t *testing.T) {
	linttest.AssertLintCount(t, "Back in the days of SpringFox, there were several requests to ...", NewBackInTheDay(), 0)
}

func TestBackInTheDay_AvoidsFalsePositiveIe(t *testing.T) {
	linttest.AssertLintCount(t, "Back in the days of IE, Powershell used to ...", NewBackInTheDay(), 0)
}

func TestBackInTheDay_AvoidsFalsePositiveCodeUsage(t *testing.T) {
	linttest.AssertLintCount(t, "Back in the days when I had 100% of my code in ...", NewBackInTheDay(), 0)
}

func TestBackInTheDay_CatchesUppercase(t *testing.T) {
	linttest.AssertLintCount(t, "Back in the days, we went for a walk.", NewBackInTheDay(), 1)
}

func TestBackInTheDay_CatchesLowercase(t *testing.T) {
	linttest.AssertLintCount(t, "We used to go for walks back in the days.", NewBackInTheDay(), 1)
}

func TestBackInTheDay_DoesntCatchFalsePositiveOf(t *testing.T) {
	linttest.AssertLintCount(t, "Back in the days of CRTs, computers were expensive.", NewBackInTheDay(), 0)
}

func TestBackInTheDay_DoesntCatchFalsePositiveWhen(t *testing.T) {
	linttest.AssertLintCount(t, "Back in the days when videogame arcades were popular.", NewBackInTheDay(), 0)
}

func TestBackInTheDay_CatchesCommaWhen(t *testing.T) {
	linttest.AssertLintCount(t, "Back in the days, when we were children, we played outside.", NewBackInTheDay(), 1)
}

func TestBackInTheDay_DoesntCatchFalsePositiveBefore(t *testing.T) {
	linttest.AssertLintCount(t, "Back in the days before laptops we had \"luggables\".", NewBackInTheDay(), 0)
}

func TestBackInTheDay_CatchesCommaBefore(t *testing.T) {
	linttest.AssertLintCount(t, "Back in the days, before laptops.", NewBackInTheDay(), 1)
}

func TestBackInTheDay_DoesntCatchQualifiedDays(t *testing.T) {
	linttest.AssertLintCount(t, "Back in the old days we did this by hand.", NewBackInTheDay(), 0)
}
