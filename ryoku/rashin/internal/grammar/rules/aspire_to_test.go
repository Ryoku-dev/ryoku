package rules

import (
	"ryoku-rashin/internal/grammar/linttest"
	"testing"
)

func TestAspireTo_TestAspireTo(t *testing.T) {
	linttest.AssertSuggestionResult(t, "aspire for", NewAspireTo(), "aspire to")
}

func TestAspireTo_IgnoreAfterUsing(t *testing.T) {
	linttest.AssertNoLints(t, "I am developing an application on ASP.NET Core 8 using Aspire for infrastructure management.", NewAspireTo())
}

func TestAspireTo_IgnoreAfterAllCapsNet(t *testing.T) {
	linttest.AssertNoLints(t, "This repositry contains the integrations with .NET Aspire for AWS.", NewAspireTo())
}

func TestAspireTo_IgnoreAfterAllCapsNetNoDot(t *testing.T) {
	linttest.AssertNoLints(t, "This repositry contains the integrations with NET Aspire for AWS.", NewAspireTo())
}

func TestAspireTo_DontIgnoreAfterLowercaseNet(t *testing.T) {
	linttest.AssertSuggestionResult(t, "my net aspires for catch a bug", NewAspireTo(), "my net aspires to catch a bug")
}

func TestAspireTo_FlagAtStartOfDocWhenCapitalized(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Aspire for greatness, even at the start of a sentence.", NewAspireTo(), "Aspire to greatness, even at the start of a sentence.")
}

func TestAspireTo_FlagAtTheStartOfASentenceMidDocument(t *testing.T) {
	linttest.AssertSuggestionResult(t, "This is a sentence. Aspire for greatness, even at the start of a mid-document sentence.", NewAspireTo(), "This is a sentence. Aspire to greatness, even at the start of a mid-document sentence.")
}

func TestAspireTo_DontFlagCapitalizedMidDocument(t *testing.T) {
	linttest.AssertNoLints(t, "I reccumend Aspire for your next project.", NewAspireTo())
}
