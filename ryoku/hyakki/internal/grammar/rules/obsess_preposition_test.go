package rules

import (
	"ryoku-rashin/internal/grammar/linttest"
	"testing"
)

func TestObsessPreposition_FixObsessOn(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Obsess on collecting good answers and you might be precise but irrelevant.", NewObsessPreposition(), "Obsess over collecting good answers and you might be precise but irrelevant.")
}

func TestObsessPreposition_FixObsessingOn(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Obsessing on finding new solutions to old problems with AI.", NewObsessPreposition(), "Obsessing over finding new solutions to old problems with AI.")
}

func TestObsessPreposition_FixObsessingWith(t *testing.T) {
	linttest.AssertSuggestionResult(t, "I spent too long checking my code over and over, obsessing with just what might cause this", NewObsessPreposition(), "I spent too long checking my code over and over, obsessing over just what might cause this")
}

func TestObsessPreposition_FixObsessWith(t *testing.T) {
	linttest.AssertSuggestionResult(t, "And as a programmer I've been taught to obsess with that.", NewObsessPreposition(), "And as a programmer I've been taught to obsess over that.")
}

func TestObsessPreposition_FixObsessesWith(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Every developer obsesses with micro-optimizations must be made to read it over and over again.", NewObsessPreposition(), "Every developer obsesses over micro-optimizations must be made to read it over and over again.")
}

func TestObsessPreposition_FixObsessedOn(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Secondly, if you get obsessed on any idea, then delve in it and don't worry about anything others until you get there.", NewObsessPreposition(), "Secondly, if you get obsessed with any idea, then delve in it and don't worry about anything others until you get there.")
}

func TestObsessPreposition_FixObsessAbout2743(t *testing.T) {
	linttest.AssertLintMessage(t, "but don't obsess about it", NewObsessPreposition(), "Use 'over' instead of 'about'.")
}
