package rules

import (
	"ryoku-rashin/internal/grammar/linttest"
	"testing"
)

func TestOxfordCommasNoOxfordComma_Fruits(t *testing.T) {
	linttest.AssertLintCount(t, "An apple, a banana, and a pear", NewNoOxfordComma(), 1)
}

func TestOxfordCommasNoOxfordComma_People(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Nancy, Steve, and Carl are going to the coffee shop.", NewNoOxfordComma(), "Nancy, Steve and Carl are going to the coffee shop.")
}

func TestOxfordCommasNoOxfordComma_Places(t *testing.T) {
	linttest.AssertSuggestionResult(t, "I've always wanted to visit Paris, Tokyo, and Rome.", NewNoOxfordComma(), "I've always wanted to visit Paris, Tokyo and Rome.")
}

func TestOxfordCommasNoOxfordComma_Foods(t *testing.T) {
	linttest.AssertSuggestionResult(t, "My favorite foods are pizza, sushi, tacos, and burgers.", NewNoOxfordComma(), "My favorite foods are pizza, sushi, tacos and burgers.")
}

func TestOxfordCommasNoOxfordComma_AllowsCleanMusic(t *testing.T) {
	linttest.AssertLintCount(t, "I enjoy listening to pop music, rock, hip-hop, electronic dance and classical music.", NewNoOxfordComma(), 0)
}

func TestOxfordCommasNoOxfordComma_AllowsCleanNations(t *testing.T) {
	linttest.AssertLintCount(t, "The team consists of players from different countries: France, Germany, Italy and Spain.", NewNoOxfordComma(), 0)
}

func TestOxfordCommasNoOxfordComma_OrWriting(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Harper can be a lifesaver when writing technical documents, emails, or other formal forms of communication.", NewNoOxfordComma(), "Harper can be a lifesaver when writing technical documents, emails or other formal forms of communication.")
}

func TestOxfordCommasNoOxfordComma_Sports(t *testing.T) {
	linttest.AssertSuggestionResult(t, "They enjoy playing soccer, basketball, or tennis.", NewNoOxfordComma(), "They enjoy playing soccer, basketball or tennis.")
}

func TestOxfordCommasNoOxfordComma_NorVegetables(t *testing.T) {
	linttest.AssertSuggestionResult(t, "I like carrots, kale, nor broccoli.", NewNoOxfordComma(), "I like carrots, kale nor broccoli.")
}

func TestOxfordCommasNoOxfordComma_FixTheSecondSentence(t *testing.T) {
	linttest.AssertSuggestionResult(t, "One, two and three. But four, five, and six.", NewNoOxfordComma(), "One, two and three. But four, five and six.")
}

func TestOxfordCommasNoOxfordComma_AllowEgConjunction(t *testing.T) {
	linttest.AssertLintCount(t, "Memory is used for different tasks in embedded systems, e.g., storage or communication with sensors and actuators.", NewNoOxfordComma(), 0)
}
