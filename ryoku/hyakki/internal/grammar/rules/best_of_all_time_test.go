package rules

import (
	"ryoku-rashin/internal/grammar/linttest"
	"testing"
)

func TestBestOfAllTime_DontFlagListOfAllTimes(t *testing.T) {
	linttest.AssertLintCount(t, "Provides a formatted list of all times that SDO was non-nominal", NewBestOfAllTime(), 0)
}

func TestBestOfAllTime_FixAfterBest(t *testing.T) {
	linttest.AssertSuggestionResult(t, "And also in the best IDE of all times Visual Studio", NewBestOfAllTime(), "And also in the best IDE of all time Visual Studio")
}

func TestBestOfAllTime_FixAfterGreatest(t *testing.T) {
	linttest.AssertSuggestionResult(t, "This app shows you why Sachin Tendulkar is the greatest cricket of all times, by using interactive stories.", NewBestOfAllTime(), "This app shows you why Sachin Tendulkar is the greatest cricket of all time, by using interactive stories.")
}

func TestBestOfAllTime_FixAfterBiggest(t *testing.T) {
	linttest.AssertSuggestionResult(t, "THIS IS THE BIGGEST QUESTIONS OF ALL TIMES...", NewBestOfAllTime(), "THIS IS THE BIGGEST QUESTIONS OF ALL TIME...")
}

func TestBestOfAllTime_FixAfterMostInfluential(t *testing.T) {
	linttest.AssertSuggestionResult(t, "It is an open source project that aggregates multiple lists of \"the best/most influential games of all times\"", NewBestOfAllTime(), "It is an open source project that aggregates multiple lists of \"the best/most influential games of all time\"")
}

func TestBestOfAllTime_DontFlagSumOfAllTimes(t *testing.T) {
	linttest.AssertLintCount(t, "The original TotalTime seems not be the sum of all times", NewBestOfAllTime(), 0)
}

func TestBestOfAllTime_DontFlagHistoryStacksOfAllTimes(t *testing.T) {
	linttest.AssertLintCount(t, "Didn't this imply all history stacks of all times, which itself implied all those saved.", NewBestOfAllTime(), 0)
}

func TestBestOfAllTime_FixAfterFavorite(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Red Dead Redemption 2 is my nr 1 favorite game of all times", NewBestOfAllTime(), "Red Dead Redemption 2 is my nr 1 favorite game of all time")
}

func TestBestOfAllTime_FixAfterFavourite(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Just made this website to show you my favourite movies of all times.", NewBestOfAllTime(), "Just made this website to show you my favourite movies of all time.")
}

func TestBestOfAllTime_FixTopOutOfVocabulary(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Can I Play the Top 10 Basslines of All Times?", NewBestOfAllTime(), "Can I Play the Top 10 Basslines of All Time?")
}

func TestBestOfAllTime_FixCompoundNoun(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Is he the best bass guitarist of all times?", NewBestOfAllTime(), "Is he the best bass guitarist of all time?")
}

func TestBestOfAllTime_FixContainingCommas(t *testing.T) {
	linttest.AssertSuggestionResult(t, "I am the biggest, best, and most humble of all times", NewBestOfAllTime(), "I am the biggest, best, and most humble of all time")
}
