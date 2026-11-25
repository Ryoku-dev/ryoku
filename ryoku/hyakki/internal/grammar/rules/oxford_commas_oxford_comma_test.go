package rules

import (
	"ryoku-rashin/internal/grammar/linttest"
	"testing"
)

func TestOxfordCommasOxfordComma_Fruits(t *testing.T) {
	linttest.AssertLintCount(t, "An apple, a banana and a pear walk into a bar.", NewOxfordComma(), 1)
}

func TestOxfordCommasOxfordComma_People(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Nancy, Steve and Carl are going to the coffee shop.", NewOxfordComma(), "Nancy, Steve, and Carl are going to the coffee shop.")
}

func TestOxfordCommasOxfordComma_Places(t *testing.T) {
	linttest.AssertSuggestionResult(t, "I've always wanted to visit Paris, Tokyo and Rome.", NewOxfordComma(), "I've always wanted to visit Paris, Tokyo, and Rome.")
}

func TestOxfordCommasOxfordComma_Foods(t *testing.T) {
	linttest.AssertSuggestionResult(t, "My favorite foods are pizza, sushi, tacos and burgers.", NewOxfordComma(), "My favorite foods are pizza, sushi, tacos, and burgers.")
}

func TestOxfordCommasOxfordComma_AllowsCleanMusic(t *testing.T) {
	linttest.AssertLintCount(t, "I enjoy listening to pop music, rock, hip-hop, electronic dance, and classical music.", NewOxfordComma(), 0)
}

func TestOxfordCommasOxfordComma_AllowsCleanNations(t *testing.T) {
	linttest.AssertLintCount(t, "The team consists of players from different countries: France, Germany, Italy, and Spain.", NewOxfordComma(), 0)
}

func TestOxfordCommasOxfordComma_OrWriting(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Harper can be a lifesaver when writing technical documents, emails or other formal forms of communication.", NewOxfordComma(), "Harper can be a lifesaver when writing technical documents, emails, or other formal forms of communication.")
}

func TestOxfordCommasOxfordComma_Sports(t *testing.T) {
	linttest.AssertSuggestionResult(t, "They enjoy playing soccer, basketball or tennis.", NewOxfordComma(), "They enjoy playing soccer, basketball, or tennis.")
}

func TestOxfordCommasOxfordComma_NorVegetables(t *testing.T) {
	linttest.AssertSuggestionResult(t, "I like carrots, kale nor broccoli.", NewOxfordComma(), "I like carrots, kale, nor broccoli.")
}

func TestOxfordCommasOxfordComma_AllowNonListTransportation(t *testing.T) {
	linttest.AssertLintCount(t, "In transportation, autonomous vehicles and smart traffic management systems promise to reduce accidents and optimize travel routes.", NewOxfordComma(), 0)
}

func TestOxfordCommasOxfordComma_AllowPill(t *testing.T) {
	linttest.AssertLintCount(t, "Develop a pill that causes partial amnesia, affecting relationships and identity.", NewOxfordComma(), 0)
}

func TestOxfordCommasOxfordComma_AllowAtFirst(t *testing.T) {
	linttest.AssertLintCount(t, "In the heart of a bustling city, Sarah finds herself trapped in an endless cycle of the same day. Each morning, she awakens to find the date unchanged, her life on repeat. At first, confusion and frustration cloud her thoughts, but soon she notices something peculiar—each day has tiny differences, subtle changes that hint at a larger pattern.", NewOxfordComma(), 0)
}

func TestOxfordCommasOxfordComma_AllowStandoff(t *testing.T) {
	linttest.AssertLintCount(t, "In a tense standoff, Alex and his reflection engage in a battle of wills.", NewOxfordComma(), 0)
}

func TestOxfordCommasOxfordComma_FixTheFirstSentence(t *testing.T) {
	linttest.AssertSuggestionResult(t, "One, two and three. But four, five, and six.", NewOxfordComma(), "One, two, and three. But four, five, and six.")
}

func TestOxfordCommasOxfordComma_AllowEgConjunction(t *testing.T) {
	linttest.AssertLintCount(t, "Memory is used for different tasks in embedded systems, e.g., storage or communication with sensors and actuators.", NewOxfordComma(), 0)
}

func TestOxfordCommasOxfordComma_EgWithThreeItemsRequiresOxfordComma(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Tasks in embedded systems, e.g., storage, sensing or communication.", NewOxfordComma(), "Tasks in embedded systems, e.g., storage, sensing, or communication.")
}
