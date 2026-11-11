package demo

import (
	"sort"
	"time"

	"ryoku-i18n"

	"ryoku-rashin/internal/protocol"
)

// a week of what the teams did, from every kind of starter, some of it while no window was open
func demoActivity(clock teamsClock) []protocol.Activity {
	now := clock.now
	ago := func(d time.Duration) time.Time { return now.Add(-d) }
	you, agent, routine, rashin := protocol.InitiatorYou, protocol.InitiatorAgent, protocol.InitiatorRoutine, protocol.InitiatorRashin
	heartbeat, coverage := i18n.T("Compare Paperclip's heartbeat with cron"), i18n.T("Find this week's coverage of Ryoku")
	duplicates, testers := i18n.T("Group the duplicate install failure issues"), i18n.T("Draft the monthly update to the testers")
	morning, evening, weekly, breaking := i18n.T("Morning review"), i18n.T("Evening check"), i18n.T("Weekly digest"), i18n.T("Breaking alerts")
	printShop := i18n.T("Thank the print shop for the merch samples")
	release := i18n.T("Write the release notes for 0.9.4")
	colourPiece := i18n.T("Write a short piece on colour management in Wayland")
	activity := []protocol.Activity{
		{Time: ago(3 * time.Minute), Actor: "you", Action: "task.create", Target: testers, OK: true, Team: demoTeamMail, Task: "task-testers", Initiator: you, Detail: i18n.T("It waits for a free slot.")},
		{Time: ago(13 * time.Minute), Actor: i18n.T("Curator"), Action: "approval.ask", Target: colourPiece, OK: true, Team: demoTeamNews, Task: "task-colour-piece", Initiator: agent, Detail: i18n.T("Which angle should the piece take?")},
		{Time: ago(17 * time.Minute), Actor: "Rashin", Action: "task.start", Target: colourPiece, OK: true, Team: demoTeamNews, Task: "task-colour-piece", Initiator: rashin, Detail: i18n.Tf("%s started work on it with %s, %s.", i18n.T("Curator"), "Claude Code", "sonnet")},
		{Time: ago(18 * time.Minute), Actor: "you", Action: "task.create", Target: colourPiece, OK: true, Team: demoTeamNews, Task: "task-colour-piece", Initiator: you},
		{Time: ago(5 * time.Hour), Actor: "you", Action: "team.create", Target: i18n.T("Reading list"), OK: true, Team: demoTeamReading, Initiator: you},
		{Time: ago(7 * time.Hour), Actor: "Rashin", Action: "task.prune", Target: i18n.T("Research desk"), OK: true, Team: demoTeamResearch, Initiator: rashin, Detail: i18n.Tf("Deleted %d finished tasks and their conversations.", 3)},
		{Time: ago(6 * time.Minute), Actor: i18n.T("Writer"), Action: "approval.ask", Target: i18n.T("Reply to Ana about the Lisbon meetup"), OK: true, Team: demoTeamMail, Task: "task-meetup", Initiator: agent, Detail: i18n.T("Send a mail to ana.ribeiro@example.org")},
		{Time: ago(8 * time.Minute), Actor: "Rashin", Action: "task.start", Target: i18n.T("Reply to Ana about the Lisbon meetup"), OK: true, Team: demoTeamMail, Task: "task-meetup", Initiator: rashin},
		{Time: ago(9 * time.Minute), Actor: i18n.T("Researcher"), Action: "task.report", Target: i18n.T("Collect how cron and systemd timers treat missed runs"), OK: true, Team: demoTeamResearch, Task: "task-heartbeat-cron", Initiator: agent, Detail: i18n.T("How cron and systemd timers treat missed runs")},
		{Time: ago(11 * time.Minute), Actor: i18n.T("Editor"), Action: "approval.ask", Target: coverage, OK: true, Team: demoTeamResearch, Task: "task-coverage", Initiator: agent, Detail: i18n.T("Let Research desk send NEWS_API_KEY to newsapi.org")},
		{Time: ago(14 * time.Minute), Actor: "Rashin", Action: "task.start", Target: coverage, OK: true, Team: demoTeamResearch, Task: "task-coverage", Initiator: rashin},
		{Time: ago(15 * time.Minute), Actor: "you", Action: "task.create", Target: coverage, OK: true, Team: demoTeamResearch, Task: "task-coverage", Initiator: you},
		{Time: ago(22 * time.Minute), Actor: i18n.T("Editor"), Action: "task.delegate", Target: i18n.T("Read how Paperclip's heartbeat works"), OK: true, Team: demoTeamResearch, Task: "task-heartbeat", Initiator: agent, Detail: i18n.T("Handed to the Researcher.")},
		{Time: ago(23 * time.Minute), Actor: i18n.T("Editor"), Action: "task.delegate", Target: i18n.T("Collect how cron and systemd timers treat missed runs"), OK: true, Team: demoTeamResearch, Task: "task-heartbeat", Initiator: agent, Detail: i18n.T("Handed to the Researcher.")},
		{Time: ago(25 * time.Minute), Actor: "Rashin", Action: "task.start", Target: heartbeat, OK: true, Team: demoTeamResearch, Task: "task-heartbeat", Initiator: rashin},
		{Time: ago(26 * time.Minute), Actor: "you", Action: "task.create", Target: heartbeat, OK: true, Team: demoTeamResearch, Task: "task-heartbeat", Initiator: you},
		{Time: ago(35 * time.Minute), Actor: "Rashin", Action: "task.start", Target: duplicates, OK: true, Team: demoTeamRepo, Task: "task-duplicates", Initiator: rashin},
		{Time: ago(36 * time.Minute), Actor: "you", Action: "task.create", Target: duplicates, OK: true, Team: demoTeamRepo, Task: "task-duplicates", Initiator: you},
		{Time: ago(40 * time.Minute), Actor: i18n.T("Curator"), Action: "credential.request", Target: "MASTODON_TOKEN", OK: true, Team: demoTeamNews, Task: "task-mastodon", Initiator: agent, Unwatched: true, Detail: i18n.T("Asked for a token that reads your bookmarks at mastodon.social.")},
		{Time: ago(2 * time.Hour), Actor: "Rashin", Action: "budget.stop", Target: i18n.T("Release notes"), OK: true, Team: demoTeamRelease, Initiator: rashin, Unwatched: true, Detail: i18n.T("It reached its limit of 1,000,000 tokens this month and paused.")},
		{Time: ago(2*time.Hour - time.Second), Actor: "Rashin", Action: "task.stop", Target: release, OK: true, Team: demoTeamRelease, Task: "task-release", Initiator: rashin, Unwatched: true, Detail: i18n.T("The team reached its spending limit.")},
		{Time: ago(2*time.Hour - 2*time.Second), Actor: "Rashin", Action: "approval.ask", Target: i18n.T("Release notes"), OK: true, Team: demoTeamRelease, Initiator: rashin, Unwatched: true, Detail: i18n.T("Release notes reached its spending limit")},
		{Time: ago(3 * time.Hour), Actor: "you", Action: "task.create", Target: release, OK: true, Team: demoTeamRelease, Task: "task-release", Initiator: you},
		{Time: ago(4*time.Hour + 10*time.Minute), Actor: "Rashin", Action: "budget.warn", Target: i18n.T("News digest"), OK: true, Team: demoTeamNews, Initiator: rashin, Unwatched: true, Detail: i18n.T("It used 80 percent of its 1,500,000 tokens this month.")},
		{Time: clock.breaking.Add(3*time.Minute + time.Second), Actor: "Rashin", Action: "routine.off", Target: breaking, Team: demoTeamNews, Task: "task-breaking", Initiator: rashin, Unwatched: true, Detail: i18n.T("Turned off after five failed runs in a row.")},
		{Time: clock.breaking.Add(3 * time.Minute), Actor: i18n.T("Reader"), Action: "task.block", Target: breaking, Team: demoTeamNews, Task: "task-breaking", Initiator: agent, Unwatched: true, Detail: i18n.T("newsapi.org answered 429 Too Many Requests.")},
		{Time: clock.breaking.Add(2 * time.Minute), Actor: i18n.T("Reader"), Action: "credential.use", Target: "NEWS_API_KEY", Team: demoTeamNews, Task: "task-breaking", Initiator: agent, Unwatched: true, Detail: i18n.T("Sent NEWS_API_KEY to newsapi.org; it answered 429.")},
		{Time: clock.breaking, Actor: breaking, Action: "routine.fire", Target: breaking, OK: true, Team: demoTeamNews, Task: "task-breaking", Initiator: routine, Unwatched: true},
		{Time: clock.morning.Add(14 * time.Minute), Actor: i18n.T("Reviewer"), Action: "task.report", Target: morning, OK: true, Team: demoTeamRepo, Task: "task-review-morning", Initiator: agent, Unwatched: true, Detail: i18n.T("Six PRs and three issues need you")},
		{Time: clock.morning.Add(6 * time.Minute), Actor: i18n.T("Reviewer"), Action: "credential.use", Target: "GITHUB_TOKEN", OK: true, Team: demoTeamRepo, Task: "task-review-morning", Initiator: agent, Unwatched: true, Detail: i18n.T("Sent GITHUB_TOKEN to api.github.com for the reviews of #412.")},
		{Time: clock.morning.Add(2 * time.Minute), Actor: i18n.T("Reviewer"), Action: "credential.use", Target: "GITHUB_TOKEN", OK: true, Team: demoTeamRepo, Task: "task-review-morning", Initiator: agent, Unwatched: true, Detail: i18n.T("Sent GITHUB_TOKEN to api.github.com to search open pull requests.")},
		{Time: clock.morning, Actor: morning, Action: "routine.fire", Target: morning, OK: true, Team: demoTeamRepo, Task: "task-review-morning", Initiator: routine, Unwatched: true},
		{Time: clock.weekly.Add(20 * time.Minute), Actor: i18n.T("Curator"), Action: "task.report", Target: weekly, OK: true, Team: demoTeamNews, Task: "task-digest", Initiator: agent, Unwatched: true, Detail: i18n.T("This week in Linux desktops")},
		{Time: clock.weekly.Add(2 * time.Minute), Actor: i18n.T("Curator"), Action: "credential.use", Target: "NEWS_API_KEY", OK: true, Team: demoTeamNews, Task: "task-digest", Initiator: agent, Unwatched: true, Detail: i18n.T("Sent NEWS_API_KEY to newsapi.org.")},
		{Time: clock.weekly, Actor: weekly, Action: "routine.fire", Target: weekly, OK: true, Team: demoTeamNews, Task: "task-digest", Initiator: routine, Unwatched: true},
		{Time: ago(19*time.Hour - 3*time.Minute), Actor: i18n.T("Writer"), Action: "task.report", Target: printShop, OK: true, Team: demoTeamMail, Task: "task-print-shop", Initiator: agent, Detail: i18n.T("Sent the thank-you mail to the print shop")},
		{Time: ago(19*time.Hour - time.Minute), Actor: i18n.T("Writer"), Action: "mail.send", Target: printShop, OK: true, Team: demoTeamMail, Task: "task-print-shop", Initiator: agent, Detail: i18n.T("Sent to orders@riso.example.pt.")},
		{Time: ago(19 * time.Hour), Actor: "you", Action: "approval.approve", Target: i18n.T("Send a mail to orders@riso.example.pt"), OK: true, Team: demoTeamMail, Task: "task-print-shop", Initiator: you, Detail: i18n.T("Approved once.")},
		{Time: ago(20 * time.Hour), Actor: "you", Action: "task.create", Target: printShop, OK: true, Team: demoTeamMail, Task: "task-print-shop", Initiator: you},
		{Time: ago(26 * time.Hour), Actor: "you", Action: "team.pause", Target: i18n.T("System care"), OK: true, Team: demoTeamSystem, Initiator: you, Detail: i18n.T("It starts nothing until you let it work again.")},
		{Time: ago(27 * time.Hour), Actor: i18n.T("Caretaker"), Action: "task.block", Target: i18n.T("Find what fills the disk"), Team: demoTeamSystem, Task: "task-disk", Initiator: agent, Detail: i18n.T("It needs root to read /var/lib/docker.")},
		{Time: ago(29 * time.Hour), Actor: "you", Action: "routine.update", Target: evening, OK: true, Team: demoTeamSystem, Initiator: you, Detail: i18n.T("Turned off.")},
		{Time: clock.previousMorning.Add(31 * time.Minute), Actor: i18n.T("Reviewer"), Action: "task.report", Target: morning, OK: true, Team: demoTeamRepo, Task: "task-review-previous", Initiator: agent, Unwatched: true, Detail: i18n.T("Two PRs need you")},
		{Time: clock.previousMorning.Add(20 * time.Minute), Actor: "you", Action: "approval.deny", Target: i18n.T("Comment on PR #409 asking for a rebase"), OK: true, Team: demoTeamRepo, Task: "task-review-previous", Initiator: you, Detail: i18n.T("Never comment for me. Put it in the report instead.")},
		{Time: clock.previousMorning.Add(8 * time.Minute), Actor: i18n.T("Reviewer"), Action: "approval.ask", Target: i18n.T("Comment on PR #409 asking for a rebase"), OK: true, Team: demoTeamRepo, Task: "task-review-previous", Initiator: agent, Unwatched: true},
		{Time: clock.evening.Add(12 * time.Minute), Actor: i18n.T("Caretaker"), Action: "task.report", Target: evening, OK: true, Team: demoTeamSystem, Task: "task-evening", Initiator: agent, Unwatched: true, Detail: i18n.Tf("Nothing new: %s", i18n.T("Nothing needs you tonight"))},
		{Time: clock.evening, Actor: evening, Action: "routine.fire", Target: evening, OK: true, Team: demoTeamSystem, Task: "task-evening", Initiator: routine, Unwatched: true},
		{Time: ago(4 * 24 * time.Hour), Actor: "Rashin", Action: "approval.expire", Target: i18n.T("Send a mail to press@example.org"), OK: true, Team: demoTeamMail, Initiator: rashin, Unwatched: true, Detail: i18n.T("Nobody answered within a day.")},
		{Time: ago(5 * 24 * time.Hour), Actor: "you", Action: "team.update", Target: i18n.T("Release notes"), OK: true, Team: demoTeamRelease, Initiator: you, Detail: i18n.T("Set its spending limit to 1,000,000 tokens a month.")},
		{Time: ago(6 * 24 * time.Hour), Actor: "you", Action: "routine.create", Target: breaking, OK: true, Team: demoTeamNews, Initiator: you, Detail: i18n.T("Runs every hour at :15.")},
	}
	sort.SliceStable(activity, func(i, j int) bool { return activity[i].Time.After(activity[j].Time) })
	return activity
}

// the vault's own record: uses, an ask, a refusal, a failure and the values it was given
func demoVaultLog(clock teamsClock) []protocol.VaultEvent {
	now := clock.now
	ago := func(d time.Duration) time.Time { return now.Add(-d) }
	log := []protocol.VaultEvent{
		{Time: ago(11 * time.Minute), Credential: "credential-news", Name: "NEWS_API_KEY", Team: demoTeamResearch, Task: "task-coverage", Host: "newsapi.org", Outcome: protocol.VaultAsked, Detail: i18n.T("The first use on this host waits for you.")},
		{Time: clock.morning.Add(6 * time.Minute), Credential: "credential-github", Name: "GITHUB_TOKEN", Team: demoTeamRepo, Task: "task-review-morning", Host: "api.github.com", Outcome: protocol.VaultUsed, Detail: "GET /repos/Ryoku-dev/ryoku/pulls/412/reviews", Unwatched: true},
		{Time: clock.morning.Add(3 * time.Minute), Credential: "credential-github", Name: "GITHUB_TOKEN", Team: demoTeamRepo, Task: "task-review-morning", Host: "api.github.com", Outcome: protocol.VaultUsed, Detail: "GET /orgs/Ryoku-dev/issues", Unwatched: true},
		{Time: clock.morning.Add(2 * time.Minute), Credential: "credential-github", Name: "GITHUB_TOKEN", Team: demoTeamRepo, Task: "task-review-morning", Host: "api.github.com", Outcome: protocol.VaultUsed, Detail: "GET /search/issues", Unwatched: true},
		{Time: clock.breaking.Add(2 * time.Minute), Credential: "credential-news", Name: "NEWS_API_KEY", Team: demoTeamNews, Task: "task-breaking", Host: "newsapi.org", Outcome: protocol.VaultFailed, Detail: i18n.T("newsapi.org answered 429 Too Many Requests."), Unwatched: true},
		{Time: clock.weekly.Add(2 * time.Minute), Credential: "credential-news", Name: "NEWS_API_KEY", Team: demoTeamNews, Task: "task-digest", Host: "newsapi.org", Outcome: protocol.VaultUsed, Detail: "GET /v2/everything", Unwatched: true},
		{Time: ago(19*time.Hour - time.Minute), Credential: "credential-smtp", Name: "SMTP_PASSWORD", Team: demoTeamMail, Task: "task-print-shop", Host: "smtp.fastmail.com", Outcome: protocol.VaultUsed, Detail: i18n.T("Sent one mail to orders@riso.example.pt.")},
		{Time: clock.previousMorning.Add(2 * time.Minute), Credential: "credential-github", Name: "GITHUB_TOKEN", Team: demoTeamRepo, Task: "task-review-previous", Host: "api.github.com", Outcome: protocol.VaultUsed, Detail: "GET /search/issues", Unwatched: true},
		{Time: ago(47*time.Hour + 50*time.Minute), Credential: "credential-news", Name: "NEWS_API_KEY", Team: demoTeamResearch, Task: "task-capture-history", Host: "api.gdeltproject.org", Outcome: protocol.VaultRefused, Detail: i18n.T("NEWS_API_KEY may not go to api.gdeltproject.org.")},
		{Time: ago(4 * 24 * time.Hour), Credential: "credential-smtp", Name: "SMTP_PASSWORD", Team: demoTeamMail, Host: "smtp.fastmail.com", Outcome: protocol.VaultDenied, Detail: i18n.T("A mail to press@example.org waited a day without an answer."), Unwatched: true},
		{Time: now.AddDate(0, 0, -9), Credential: "credential-github", Name: "GITHUB_TOKEN", Outcome: protocol.VaultRotated, Detail: i18n.T("You replaced the value; the old one stays until the next replace.")},
		{Time: now.AddDate(0, 0, -20), Credential: "credential-news", Name: "NEWS_API_KEY", Outcome: protocol.VaultAdded, Detail: i18n.T("You added it for News digest.")},
	}
	sort.SliceStable(log, func(i, j int) bool { return log[i].Time.After(log[j].Time) })
	return log
}

// this month of a team's tokens: by day, by member, by routine and by the engine each member ran on, adding up to what its meter shows
func demoTeamSpend(clock teamsClock, team protocol.Team) protocol.TeamSpend {
	type share struct {
		id, name string
		tokens   int64
		tasks    int
	}
	var members, routines []share
	estimated := false
	switch team.ID {
	case demoTeamResearch:
		members = []share{{"member-editor", i18n.T("Editor"), 431_900, 6}, {"member-researcher", i18n.T("Researcher"), 310_400, 9}}
	case demoTeamMail:
		members = []share{{"member-writer", i18n.T("Writer"), 118_400, 8}}
	case demoTeamRepo:
		members = []share{{"member-reviewer", i18n.T("Reviewer"), 902_000, 8}, {"member-triager", i18n.T("Triager"), 382_000, 5}}
		routines = []share{{"routine-morning", i18n.T("Morning review"), 1_051_000, 7}}
	case demoTeamSystem:
		members = []share{{"member-caretaker", i18n.T("Caretaker"), 96_500, 6}}
		routines = []share{{"routine-evening", i18n.T("Evening check"), 61_200, 5}}
	case demoTeamNews:
		members = []share{{"member-curator", i18n.T("Curator"), 803_000, 4}, {"member-reader", i18n.T("Reader"), 472_000, 31}}
		routines = []share{{"routine-weekly", i18n.T("Weekly digest"), 418_000, 2}, {"routine-breaking", i18n.T("Breaking alerts"), 377_000, 31}}
		estimated = true
	case demoTeamRelease:
		members = []share{{"member-scribe", i18n.T("Scribe"), 1_000_000, 3}}
	}
	spend := protocol.TeamSpend{Team: team.ID, Month: demoMonth(clock.now), Cap: team.Budget.MonthTokens, Estimated: estimated}
	for _, member := range members {
		spend.Tokens += member.tokens
		spend.Members = append(spend.Members, protocol.NamedSpend{ID: member.id, Name: member.name, Tokens: member.tokens, Tasks: member.tasks})
		if found, ok := teamMember(team, member.id); ok {
			engine := protocol.NamedSpend{ID: found.Harness + protocol.EngineSpendSeparator + found.Model, Tokens: member.tokens, Tasks: member.tasks}
			spend.Engines = addEngines(spend.Engines, []protocol.NamedSpend{engine})
		}
	}
	for _, routine := range routines {
		spend.Routines = append(spend.Routines, protocol.NamedSpend{ID: routine.id, Name: routine.name, Tokens: routine.tokens, Tasks: routine.tasks})
	}
	spend.Days = spreadMonth(clock.now, spend.Tokens, len(team.ID))
	return spend
}

// the demo's month, in the computer's time zone as spend is counted
func demoMonth(now time.Time) string { return now.In(time.Local).Format("2006-01") }

// a month's tokens over the days so far, heavier on weekdays, adding up to total exactly
func spreadMonth(now time.Time, total int64, seed int) []protocol.DaySpend {
	local := now.In(time.Local)
	first := time.Date(local.Year(), local.Month(), 1, 0, 0, 0, 0, time.Local)
	weights := make([]int64, local.Day())
	var sum int64
	for index := range weights {
		step := (index + 1) * seed
		weights[index] = int64(2 + step%5)
		if weekday := first.AddDate(0, 0, index).Weekday(); weekday == time.Saturday || weekday == time.Sunday {
			weights[index] = 1
		}
		sum += weights[index]
	}
	days := make([]protocol.DaySpend, len(weights))
	var given int64
	for index, weight := range weights {
		tokens := total * weight / sum
		if index == len(weights)-1 {
			tokens = total - given
		}
		given += tokens
		input := tokens * 4 / 5
		days[index] = protocol.DaySpend{Day: first.AddDate(0, 0, index).Format("2006-01-02"), Input: input, Output: tokens - input}
	}
	return days
}
