package rules

import (
	"ryoku-rashin/internal/grammar/linttest"
	"testing"
)

func TestFellowCoRedundancy_FixCoworker(t *testing.T) {
	linttest.AssertGoodAndBadSuggestions(t, "My fellow coworker discovered a vagueness in the salt docs at least from his perspective using this state module:", NewFellowCoRedundancy(), []string{
		"My coworker discovered a vagueness in the salt docs at least from his perspective using this state module:",
		"My fellow worker discovered a vagueness in the salt docs at least from his perspective using this state module:",
	}, []string{})
}

func TestFellowCoRedundancy_FixCoworkers(t *testing.T) {
	linttest.AssertGoodAndBadSuggestions(t, "Actually, my fellow coworkers use putty, solarputty, snowflake and asbru.", NewFellowCoRedundancy(), []string{
		"Actually, my coworkers use putty, solarputty, snowflake and asbru.",
		"Actually, my fellow workers use putty, solarputty, snowflake and asbru.",
	}, []string{})
}

func TestFellowCoRedundancy_FixCoHyWorker(t *testing.T) {
	linttest.AssertGoodAndBadSuggestions(t, "GitFit is a Git Hook that randomly selects an Exercise and Reps with an option to challenge a fellow co-worker.", NewFellowCoRedundancy(), []string{
		"GitFit is a Git Hook that randomly selects an Exercise and Reps with an option to challenge a co-worker.",
		"GitFit is a Git Hook that randomly selects an Exercise and Reps with an option to challenge a fellow worker.",
	}, []string{})
}

func TestFellowCoRedundancy_FixCoSpWorker(t *testing.T) {
	linttest.AssertGoodAndBadSuggestions(t, "Can you please help me find a book that can inspire a fellow co worker? ", NewFellowCoRedundancy(), []string{
		"Can you please help me find a book that can inspire a co worker? ",
		"Can you please help me find a book that can inspire a fellow worker? ",
	}, []string{})
}

func TestFellowCoRedundancy_FixFellowCoHyWorkers(t *testing.T) {
	linttest.AssertGoodAndBadSuggestions(t, "Enable fellow co-workers with no command line experience to utilize python scripts.", NewFellowCoRedundancy(), []string{
		"Enable co-workers with no command line experience to utilize python scripts.",
		"Enable fellow workers with no command line experience to utilize python scripts.",
	}, []string{})
}

func TestFellowCoRedundancy_FixCoSpWorkers(t *testing.T) {
	linttest.AssertGoodAndBadSuggestions(t, "and if your interests align with 50.1% of your fellow co workers than great", NewFellowCoRedundancy(), []string{
		"and if your interests align with 50.1% of your co workers than great",
		"and if your interests align with 50.1% of your fellow workers than great",
	}, []string{})
}

func TestFellowCoRedundancy_FixCoSpWorkersAllCaps(t *testing.T) {
	linttest.AssertGoodAndBadSuggestions(t, "GOOD MORNING MY FELLOW CO WORKERS.", NewFellowCoRedundancy(), []string{
		"GOOD MORNING MY FELLOW WORKERS.",
		"GOOD MORNING MY CO WORKERS.",
	}, []string{})
}

func TestFellowCoRedundancy_FixFellowCoSpHosts(t *testing.T) {
	linttest.AssertGoodAndBadSuggestions(t, "sliding into the commentary booth with me are my fellow Co hosts", NewFellowCoRedundancy(), []string{
		"sliding into the commentary booth with me are my Co hosts",
		"sliding into the commentary booth with me are my fellow hosts",
	}, []string{})
}

func TestFellowCoRedundancy_FixFellowCoHyHost(t *testing.T) {
	linttest.AssertGoodAndBadSuggestions(t, "James Martell along with wife and fellow co-host Arlene discuss WiFi in the cars", NewFellowCoRedundancy(), []string{
		"James Martell along with wife and co-host Arlene discuss WiFi in the cars",
		"James Martell along with wife and fellow host Arlene discuss WiFi in the cars",
	}, []string{})
}

func TestFellowCoRedundancy_FixFellowCoHyConspirators(t *testing.T) {
	linttest.AssertGoodAndBadSuggestions(t, "it is so you can talk to yourself and your fellow co-conspirators", NewFellowCoRedundancy(), []string{
		"it is so you can talk to yourself and your co-conspirators",
		"it is so you can talk to yourself and your fellow conspirators",
	}, []string{})
}
