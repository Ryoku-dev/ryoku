package rules

import (
	"ryoku-rashin/internal/grammar/linttest"
	"testing"
)

func TestAmInTheMorning_FlagAt4amInTheMorning(t *testing.T) {
	linttest.AssertLintCount(t, "At 4am in the morning", NewAmInTheMorning(), 1)
}

func TestAmInTheMorning_FixAt4amInTheMorning(t *testing.T) {
	linttest.AssertSuggestionResult(t, "At 4am in the morning", NewAmInTheMorning(), "At 4am")
}

func TestAmInTheMorning_FlagAt4AmInTheMorning(t *testing.T) {
	linttest.AssertLintCount(t, "At 4 am in the morning", NewAmInTheMorning(), 1)
}

func TestAmInTheMorning_FixAt4AmInTheMorning(t *testing.T) {
	linttest.AssertSuggestionResult(t, "At 4 am in the morning", NewAmInTheMorning(), "At 4 am")
}

func TestAmInTheMorning_FlagAt4amInTheMorningCaps(t *testing.T) {
	linttest.AssertLintCount(t, "At 4AM in the morning", NewAmInTheMorning(), 1)
}

func TestAmInTheMorning_FixAt4amInTheMorningCaps(t *testing.T) {
	linttest.AssertSuggestionResult(t, "At 4AM in the morning", NewAmInTheMorning(), "At 4AM")
}

func TestAmInTheMorning_FlagAt4AmInTheMorningCaps(t *testing.T) {
	linttest.AssertLintCount(t, "At 4 AM in the morning", NewAmInTheMorning(), 1)
}

func TestAmInTheMorning_FixAt4AmInTheMorningCaps(t *testing.T) {
	linttest.AssertSuggestionResult(t, "At 4 AM in the morning", NewAmInTheMorning(), "At 4 AM")
}

func TestAmInTheMorning_At4ADotMDotInTheMorning(t *testing.T) {
	linttest.AssertLintCount(t, "At 4 a.m. in the morning", NewAmInTheMorning(), 1)
}

func TestAmInTheMorning_FixAt4ADotMDotInTheMorning(t *testing.T) {
	linttest.AssertSuggestionResult(t, "At 4 a.m. in the morning", NewAmInTheMorning(), "At 4 a.m.")
}

func TestAmInTheMorning_FixRealWorld1AmInTheMorning(t *testing.T) {
	linttest.AssertSuggestionResult(t, "I wrote this whole program as a joke, at 1 AM in the morning. Nothing else to say.", NewAmInTheMorning(), "I wrote this whole program as a joke, at 1 AM. Nothing else to say.")
	linttest.AssertSuggestionResult(t, "I wrote this whole program as a joke, at 1 AM in the morning. Nothing else to say.", NewAmInTheMorning(), "I wrote this whole program as a joke, at 1 in the morning. Nothing else to say.")
}

func TestAmInTheMorning_FixRealWorld3amInTheMorning(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Luckily I was at home, but it was not fun at 3am in the morning.", NewAmInTheMorning(), "Luckily I was at home, but it was not fun at 3am.")
	linttest.AssertSuggestionResult(t, "Luckily I was at home, but it was not fun at 3am in the morning.", NewAmInTheMorning(), "Luckily I was at home, but it was not fun at 3 in the morning.")
}

func TestAmInTheMorning_FixRealWorld3amAtNight(t *testing.T) {
	linttest.AssertSuggestionResult(t, "If I want to run my script or some cron job at 3am at night, it seems to be not possible after macOS is in sleep mode.", NewAmInTheMorning(), "If I want to run my script or some cron job at 3am, it seems to be not possible after macOS is in sleep mode.")
	linttest.AssertSuggestionResult(t, "If I want to run my script or some cron job at 3am at night, it seems to be not possible after macOS is in sleep mode.", NewAmInTheMorning(), "If I want to run my script or some cron job at 3 at night, it seems to be not possible after macOS is in sleep mode.")
}

func TestAmInTheMorning_FixRealWorld9pmAtNight(t *testing.T) {
	linttest.AssertSuggestionResult(t, "The servers stop at 9PM at night and starts again at 9AM.", NewAmInTheMorning(), "The servers stop at 9PM and starts again at 9AM.")
	linttest.AssertSuggestionResult(t, "The servers stop at 9PM at night and starts again at 9AM.", NewAmInTheMorning(), "The servers stop at 9 at night and starts again at 9AM.")
}

func TestAmInTheMorning_FixRealWorld330AmInTheMorning(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Hello I can't believe my neighbor had the nerve to knock on my door at 3:30 AM in the morning.", NewAmInTheMorning(), "Hello I can't believe my neighbor had the nerve to knock on my door at 3:30 AM.")
	linttest.AssertSuggestionResult(t, "Hello I can't believe my neighbor had the nerve to knock on my door at 3:30 AM in the morning.", NewAmInTheMorning(), "Hello I can't believe my neighbor had the nerve to knock on my door at 3:30 in the morning.")
}

func TestAmInTheMorning_FixRealWorld5PmInTheAfternoonCapsDots(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Style issues get a blue marker: It's 5 P.M. in the afternoon.", NewAmInTheMorning(), "Style issues get a blue marker: It's 5 P.M..")
	linttest.AssertSuggestionResult(t, "Style issues get a blue marker: It's 5 P.M. in the afternoon.", NewAmInTheMorning(), "Style issues get a blue marker: It's 5 in the afternoon.")
}

func TestAmInTheMorning_FixRealWorld5PmInTheAfternoonCaps(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Its a impressively versatile tool if youd like to tell a colleague from over sea's about at 5 PM in the afternoon on Monday, 27 May 2007.", NewAmInTheMorning(), "Its a impressively versatile tool if youd like to tell a colleague from over sea's about at 5 PM on Monday, 27 May 2007.")
	linttest.AssertSuggestionResult(t, "Its a impressively versatile tool if youd like to tell a colleague from over sea's about at 5 PM in the afternoon on Monday, 27 May 2007.", NewAmInTheMorning(), "Its a impressively versatile tool if youd like to tell a colleague from over sea's about at 5 in the afternoon on Monday, 27 May 2007.")
}

func TestAmInTheMorning_FixRealWorld6PmInTheEvening(t *testing.T) {
	linttest.AssertSuggestionResult(t, "I am in China and it is six pm in the evening.", NewAmInTheMorning(), "I am in China and it is six pm.")
	linttest.AssertSuggestionResult(t, "I am in China and it is six pm in the evening.", NewAmInTheMorning(), "I am in China and it is six in the evening.")
}

func TestAmInTheMorning_FixRealWorld4AmInTheMorning(t *testing.T) {
	linttest.AssertSuggestionResult(t, "On the second application, we normally have the 503 between 1am and 4 am in the morning, almost every day.", NewAmInTheMorning(), "On the second application, we normally have the 503 between 1am and 4 am, almost every day.")
	linttest.AssertSuggestionResult(t, "On the second application, we normally have the 503 between 1am and 4 am in the morning, almost every day.", NewAmInTheMorning(), "On the second application, we normally have the 503 between 1am and 4 in the morning, almost every day.")
}
