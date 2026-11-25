package rules

import (
	"ryoku-rashin/internal/grammar/linttest"
	"testing"
)

func TestFewUnitsOfTimeAgo_FixFewMinutesAgo(t *testing.T) {
	t.Skip("Ignored upstream: Needs ^ zero-width anchor that matches the start of a chunk")
	linttest.AssertSuggestionResult(t, "Few minutes ago", NewFewUnitsOfTimeAgo(), "A few minutes ago")
}

func TestFewUnitsOfTimeAgo_DontFlagAFewMinutesAgo(t *testing.T) {
	linttest.AssertLintCount(t, "A few minutes ago", NewFewUnitsOfTimeAgo(), 0)
}

func TestFewUnitsOfTimeAgo_FixDoneFewMinutesAgo(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Done few minutes ago", NewFewUnitsOfTimeAgo(), "Done a few minutes ago")
}

func TestFewUnitsOfTimeAgo_DontFlagDoneAFewMinutesAgo(t *testing.T) {
	linttest.AssertLintCount(t, "Done a few minutes ago", NewFewUnitsOfTimeAgo(), 0)
}

func TestFewUnitsOfTimeAgo_FixAfterSpace(t *testing.T) {
	t.Skip("Ignored upstream: Needs ^ zero-width anchor that matches the start of a chunk")
	linttest.AssertSuggestionResult(t, " Few minutes ago.", NewFewUnitsOfTimeAgo(), " A few minutes ago.")
}

func TestFewUnitsOfTimeAgo_Fix2ndSentence(t *testing.T) {
	t.Skip("Ignored upstream: Needs ^ zero-width anchor that matches the start of a chunk")
	linttest.AssertSuggestionResult(t, "Hello World. Few minutes ago I bought your planet.", NewFewUnitsOfTimeAgo(), "Hello World. A few minutes ago I bought your planet.")
}

func TestFewUnitsOfTimeAgo_FixDays(t *testing.T) {
	linttest.AssertSuggestionResult(t, "My jupyter kernel always says restarting and never ever runs i ran into the problem few days ago before it was fine dont know what happened", NewFewUnitsOfTimeAgo(), "My jupyter kernel always says restarting and never ever runs i ran into the problem a few days ago before it was fine dont know what happened")
}

func TestFewUnitsOfTimeAgo_FixDecades(t *testing.T) {
	linttest.AssertSuggestionResult(t, "This is very old piece of software I wrote few decades ago.", NewFewUnitsOfTimeAgo(), "This is very old piece of software I wrote a few decades ago.")
}

func TestFewUnitsOfTimeAgo_FixHours(t *testing.T) {
	linttest.AssertSuggestionResult(t, "I just updated my index file few hours ago and there's this error.", NewFewUnitsOfTimeAgo(), "I just updated my index file a few hours ago and there's this error.")
}

func TestFewUnitsOfTimeAgo_FixMinutes(t *testing.T) {
	linttest.AssertSuggestionResult(t, "mysql installed few minutes ago somehow , ubuntu bash thinks its not installed.", NewFewUnitsOfTimeAgo(), "mysql installed a few minutes ago somehow , ubuntu bash thinks its not installed.")
}

func TestFewUnitsOfTimeAgo_FixMonths(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Hello, I was working with D455 few months ago, and everything was working fine.", NewFewUnitsOfTimeAgo(), "Hello, I was working with D455 a few months ago, and everything was working fine.")
}

func TestFewUnitsOfTimeAgo_FixMs(t *testing.T) {
	linttest.AssertSuggestionResult(t, "So I not sure, by getting old signal (get from few ms ago), will it affected my result badly?", NewFewUnitsOfTimeAgo(), "So I not sure, by getting old signal (get from a few ms ago), will it affected my result badly?")
}

func TestFewUnitsOfTimeAgo_FixSeconds(t *testing.T) {
	linttest.AssertSuggestionResult(t, "I have submitted the same issue few seconds ago.", NewFewUnitsOfTimeAgo(), "I have submitted the same issue a few seconds ago.")
}

func TestFewUnitsOfTimeAgo_FixWeekends(t *testing.T) {
	linttest.AssertSuggestionResult(t, "This challenge is a Python jail escape and lucky for me our team had just done one few weekends ago so I was fairly familiar with the tricks to break out.", NewFewUnitsOfTimeAgo(), "This challenge is a Python jail escape and lucky for me our team had just done one a few weekends ago so I was fairly familiar with the tricks to break out.")
}

func TestFewUnitsOfTimeAgo_FixWeeks(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Terraform cloud crashes on plan (same configuration worked few weeks ago)", NewFewUnitsOfTimeAgo(), "Terraform cloud crashes on plan (same configuration worked a few weeks ago)")
}

func TestFewUnitsOfTimeAgo_FixYears(t *testing.T) {
	linttest.AssertSuggestionResult(t, "sandbox-exec was deprecated on MacOS few years ago", NewFewUnitsOfTimeAgo(), "sandbox-exec was deprecated on MacOS a few years ago")
}

func TestFewUnitsOfTimeAgo_DontFlagCenturies(t *testing.T) {
	linttest.AssertLintCount(t, "Would have been useful a few centuries ago.", NewFewUnitsOfTimeAgo(), 0)
}

func TestFewUnitsOfTimeAgo_DontFlagDays(t *testing.T) {
	linttest.AssertLintCount(t, "A few days ago, I upgraded ComfyUI to the latest version, then the prompt node can't upload prompt list text file in Ubuntu", NewFewUnitsOfTimeAgo(), 0)
}

func TestFewUnitsOfTimeAgo_DontFlagDecades(t *testing.T) {
	linttest.AssertLintCount(t, "With your QA background you may have heard of the IBM black team of testers back a few decades ago.", NewFewUnitsOfTimeAgo(), 0)
}

func TestFewUnitsOfTimeAgo_DontFlagHours(t *testing.T) {
	linttest.AssertLintCount(t, "It was working well and we could see the installation page a few hours ago.", NewFewUnitsOfTimeAgo(), 0)
}

func TestFewUnitsOfTimeAgo_DontFlagMilliseconds(t *testing.T) {
	linttest.AssertLintCount(t, "It is actually the true motor angle observed a few milliseconds ago (pd latency).", NewFewUnitsOfTimeAgo(), 0)
}

func TestFewUnitsOfTimeAgo_DontFlagMinutes(t *testing.T) {
	linttest.AssertLintCount(t, "Example from DoD The following was circulated a few minutes ago on an IDESG/NSTIC list", NewFewUnitsOfTimeAgo(), 0)
}

func TestFewUnitsOfTimeAgo_DontFlagMoments(t *testing.T) {
	linttest.AssertLintCount(t, "Our microservices started failing a few moments ago when creating new...", NewFewUnitsOfTimeAgo(), 0)
}

func TestFewUnitsOfTimeAgo_DontFlagMonths(t *testing.T) {
	linttest.AssertLintCount(t, "A few months ago there was an mixed reality project.", NewFewUnitsOfTimeAgo(), 0)
}

func TestFewUnitsOfTimeAgo_DontFlagNights(t *testing.T) {
	linttest.AssertLintCount(t, "As an example, a few nights ago I was working on my laptop and stuff that had been working stopped working.", NewFewUnitsOfTimeAgo(), 0)
}

func TestFewUnitsOfTimeAgo_DontFlagSeconds(t *testing.T) {
	linttest.AssertLintCount(t, "0 - 45 seconds, a few seconds ago.", NewFewUnitsOfTimeAgo(), 0)
}

func TestFewUnitsOfTimeAgo_DontFlagWeeks(t *testing.T) {
	linttest.AssertLintCount(t, "It was all working perfectly till a few weeks ago.", NewFewUnitsOfTimeAgo(), 0)
}

func TestFewUnitsOfTimeAgo_DontFlagYears(t *testing.T) {
	linttest.AssertLintCount(t, "Hello, I've been an intensive user of your dada2 pipeline until a few years ago.", NewFewUnitsOfTimeAgo(), 0)
}

func TestFewUnitsOfTimeAgo_DontFlagDecade(t *testing.T) {
	linttest.AssertLintCount(t, "With your QA background you may have heard of the IBM black team of testers back a few decade ago.", NewFewUnitsOfTimeAgo(), 0)
}

func TestFewUnitsOfTimeAgo_DontFlagHour(t *testing.T) {
	linttest.AssertLintCount(t, "It was working well and we could see the installation page a few hour ago.", NewFewUnitsOfTimeAgo(), 0)
}

func TestFewUnitsOfTimeAgo_DontFlagMillennia(t *testing.T) {
	linttest.AssertLintCount(t, "A few millennia ago, there was a civilization here", NewFewUnitsOfTimeAgo(), 0)
}

func TestFewUnitsOfTimeAgo_DontFlagMinute(t *testing.T) {
	linttest.AssertLintCount(t, "Example from DoD The following was circulated a few minute ago on an IDESG/NSTIC list", NewFewUnitsOfTimeAgo(), 0)
}

func TestFewUnitsOfTimeAgo_DontFlagMoment(t *testing.T) {
	linttest.AssertLintCount(t, "No problem should be in the updated version pushed a few moment ago will be live in beta in about 10 min.", NewFewUnitsOfTimeAgo(), 0)
}

func TestFewUnitsOfTimeAgo_DontFlagMonth(t *testing.T) {
	linttest.AssertLintCount(t, "I noticed the same thing a few month ago.", NewFewUnitsOfTimeAgo(), 0)
}

func TestFewUnitsOfTimeAgo_DontFlagSecond(t *testing.T) {
	linttest.AssertLintCount(t, "Bug it doesnt even answer me rn, like a few second ago he did", NewFewUnitsOfTimeAgo(), 0)
}

func TestFewUnitsOfTimeAgo_DontFlagWeek(t *testing.T) {
	linttest.AssertLintCount(t, "A few week ago, when logging in the usual way", NewFewUnitsOfTimeAgo(), 0)
}

func TestFewUnitsOfTimeAgo_DontFlagYear(t *testing.T) {
	linttest.AssertLintCount(t, "Hello, I've been an intensive user of your dada2 pipeline until a few year ago.", NewFewUnitsOfTimeAgo(), 0)
}

func TestFewUnitsOfTimeAgo_DontFlagDaysApos(t *testing.T) {
	linttest.AssertLintCount(t, "And finally it got released a few day's ago.", NewFewUnitsOfTimeAgo(), 0)
}

func TestFewUnitsOfTimeAgo_DontFlagMonthsApos(t *testing.T) {
	linttest.AssertLintCount(t, "I had thought that since I had done this process a few month's ago the database could just be updated.", NewFewUnitsOfTimeAgo(), 0)
}

func TestFewUnitsOfTimeAgo_DontFlagWeeksApos(t *testing.T) {
	linttest.AssertLintCount(t, "A few week's ago, I was alerted in Webmin that Webmin was eligible to upgrade to 1.880 which I did through Webmin.", NewFewUnitsOfTimeAgo(), 0)
}

func TestFewUnitsOfTimeAgo_DontFlagYearsApos(t *testing.T) {
	linttest.AssertLintCount(t, "A few year's ago a spammer registered an unused base url", NewFewUnitsOfTimeAgo(), 0)
}

func TestFewUnitsOfTimeAgo_FixDay(t *testing.T) {
	linttest.AssertSuggestionResult(t, "That worked few day ago with the same setting.", NewFewUnitsOfTimeAgo(), "That worked a few day ago with the same setting.")
}

func TestFewUnitsOfTimeAgo_FixDecade(t *testing.T) {
	t.Skip("Ignored upstream: Needs ^ zero-width anchor that matches the start of a chunk")
	linttest.AssertSuggestionResult(t, "few decade ago, African Americans weren't allowed to swim in public", NewFewUnitsOfTimeAgo(), "a few decade ago, African Americans weren't allowed to swim in public")
}

func TestFewUnitsOfTimeAgo_FixMinute(t *testing.T) {
	linttest.AssertSuggestionResult(t, "All works fine, but few minute ago the device stop responding from web", NewFewUnitsOfTimeAgo(), "All works fine, but a few minute ago the device stop responding from web")
}

func TestFewUnitsOfTimeAgo_FixWeekend(t *testing.T) {
	linttest.AssertSuggestionResult(t, "I have done this few weekend ago.", NewFewUnitsOfTimeAgo(), "I have done this a few weekend ago.")
}
