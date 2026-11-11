package demo

import (
	"sort"
	"time"

	"ryoku-i18n"

	"ryoku-rashin/internal/paths"
	"ryoku-rashin/internal/protocol"
)

// the demo's teams, named so routes and stories can open them
const (
	demoTeamResearch = "team-research"
	demoTeamMail     = "team-mail"
	demoTeamRepo     = "team-repo"
	demoTeamSystem   = "team-system"
	demoTeamNews     = "team-news"
	demoTeamRelease  = "team-release"
	demoTeamReading  = "team-reading"
)

// everything Teams mode shows, kept apart from the snapshot because the daemon builds that part fresh on each read
type teamsFixtures struct {
	teams     []protocol.Team
	tasks     []protocol.TeamTask
	approvals []protocol.Approval
	routines  []protocol.Routine
	vault     protocol.VaultInfo
	vaultLog  []protocol.VaultEvent
	activity  []protocol.Activity
	spend     map[string]protocol.TeamSpend
	// the sessions that work team tasks, which the daemon keeps out of the snapshot's session list
	sessions []protocol.SessionInfo
	// the exact action each approval would run, kept beside it as the daemon keeps it
	actions map[string]demoAction
	// events a call caused, told to the client once it has its answer
	told []protocol.Event
	// how many things the demo made, so each new id differs
	made int
}

type demoAction struct {
	recipient  string
	credential string
	host       string
	request    protocol.AgentCredentialRequestParams
}

// the moments the routines fired at, found in the computer's time zone the way their schedules are
type teamsClock struct {
	now, morning, previousMorning, evening, weekly, breaking time.Time
}

// the latest hour:minute on a day keep accepts, before now
func lastAt(now time.Time, hour, minute int, keep func(time.Weekday) bool) time.Time {
	at := time.Date(now.Year(), now.Month(), now.Day(), hour, minute, 0, 0, now.Location())
	for !at.Before(now) || !keep(at.Weekday()) {
		at = at.AddDate(0, 0, -1)
	}
	return at
}

func demoTeamsClock(now time.Time) teamsClock {
	local := now.In(time.Local)
	weekdays := func(day time.Weekday) bool { return day != time.Saturday && day != time.Sunday }
	every := func(time.Weekday) bool { return true }
	morning := lastAt(local, 9, 0, weekdays)
	return teamsClock{
		now:             now,
		morning:         morning,
		previousMorning: lastAt(morning, 9, 0, weekdays),
		evening:         lastAt(local, 18, 0, every).AddDate(0, 0, -1),
		weekly:          lastAt(local, 8, 0, func(day time.Weekday) bool { return day == time.Monday }),
		breaking:        local.Add(-5 * time.Hour).Truncate(time.Hour).Add(15 * time.Minute),
	}
}

// the teams settings of someone who set quiet hours, hid one folder of their own and keeps three months of finished tasks
func demoTeamsSettings() protocol.TeamsSettings {
	return protocol.TeamsSettings{
		MaxRunning:   protocol.DefaultTeamsMaxRunning,
		PerTeam:      protocol.DefaultTeamsPerTeam,
		RoutinesWake: true,
		Defaults: protocol.TeamDefaults{
			Approvals: protocol.ApprovalsAsk,
			Budget:    protocol.TeamBudget{MonthTokens: 2_000_000, WarnPercent: 80, TaskTurns: 20, TaskMinutes: 60},
			Notify:    protocol.NotifyAll,
		},
		ApprovalHours: protocol.DefaultApprovalHours,
		QuietHours:    &protocol.QuietHours{From: "22:00", To: "07:30"},
		Hidden:        append(protocol.DefaultTeamsHidden(), "~/Documents/taxes"),
		KeepDays:      90,
	}
}

// the five teams a new one can start from, in the order they are offered
func demoTemplates() []protocol.TeamTemplate {
	return []protocol.TeamTemplate{
		{
			ID: "research", Name: i18n.T("Research desk"),
			Purpose:     i18n.T("Looks things up on the web and in my files, and writes short reports with sources."),
			Description: i18n.T("An editor and a researcher who may use its own screen."),
			Members: []protocol.Member{
				{Name: i18n.T("Editor"), Title: i18n.T("Plans, checks sources, writes"), Lead: true, Instructions: i18n.T("Plans the work, hands searches to the researcher, checks every source and writes the report.")},
				{Name: i18n.T("Researcher"), Title: i18n.T("Searches and reads"), Instructions: i18n.T("Searches, reads pages, opens them on its own screen when a page needs a browser, and notes every source.")},
			},
			Tools: protocol.TeamTools{Screen: true}, Approvals: protocol.ApprovalsAsk,
			Budget: protocol.TeamBudget{MonthTokens: 2_000_000, WarnPercent: 80, TaskTurns: 20},
		},
		{
			ID: "mail", Name: i18n.T("Mail desk"),
			Purpose:     i18n.T("Drafts mail for me and sends it once I approve."),
			Description: i18n.T("A writer that sends nothing without your approval."),
			Members: []protocol.Member{
				{Name: i18n.T("Writer"), Title: i18n.T("Writes and sends mail"), Lead: true, Instructions: i18n.T("Writes clear, short mail in my voice, never sends without approval, and says what it sent.")},
			},
			Tools: protocol.TeamTools{Mail: true}, Approvals: protocol.ApprovalsReadOnly,
			Budget: protocol.TeamBudget{MonthTokens: 1_000_000, WarnPercent: 80, TaskTurns: 12},
		},
		{
			ID: "repo", Name: i18n.T("Repo watch"),
			Purpose:     i18n.T("Reviews open pull requests and issues in my GitHub organisation and tells me what needs me."),
			Description: i18n.T("A reviewer and a triager that read and never comment."),
			Members: []protocol.Member{
				{Name: i18n.T("Reviewer"), Title: i18n.T("Judges what needs a maintainer"), Lead: true, Instructions: i18n.T("Reads pull requests and issues, judges what needs a maintainer, and never comments or merges.")},
				{Name: i18n.T("Triager"), Title: i18n.T("Lists and groups new work"), Instructions: i18n.T("Lists new issues and pull requests, groups duplicates, and labels nothing.")},
			},
			Approvals: protocol.ApprovalsReadOnly,
			Budget:    protocol.TeamBudget{MonthTokens: 3_000_000, WarnPercent: 80, TaskTurns: 30},
			Routines: []protocol.RoutineDraft{{
				Title: i18n.T("Morning review"), Schedule: "0 9 * * 1-5",
				Brief: i18n.T("Review the pull requests and issues opened or updated since the last review, and report which need me and why."),
			}},
		},
		{
			ID: "system", Name: i18n.T("System care"),
			Purpose:     i18n.T("Looks after this computer: disk space, failed services, updates waiting, and tells me what to do."),
			Description: i18n.T("A caretaker that reads and proposes, and never changes anything."),
			Members: []protocol.Member{
				{Name: i18n.T("Caretaker"), Title: i18n.T("Reads the system's state"), Lead: true, Instructions: i18n.T("Reads the system's state with read-only commands, never changes anything, and proposes the exact commands I may run.")},
			},
			Approvals: protocol.ApprovalsReadOnly,
			Budget:    protocol.TeamBudget{MonthTokens: 1_000_000, WarnPercent: 80, TaskTurns: 15},
			Routines: []protocol.RoutineDraft{{
				Title: i18n.T("Evening check"), Schedule: "0 18 * * *",
				Brief: i18n.T("Check disk space, failed services, journal errors since yesterday and pending updates, and report anything I should act on."),
			}},
		},
		{
			ID: "blank", Name: i18n.T("New team"),
			Description: i18n.T("One lead, and nothing else until you add it."),
			Members:     []protocol.Member{{Name: i18n.T("Lead"), Lead: true}},
			Approvals:   protocol.ApprovalsAsk,
			Budget:      protocol.TeamBudget{MonthTokens: 2_000_000, WarnPercent: 80, TaskTurns: 20},
		},
	}
}

// a team made from a template, its members given ids and engines in order
func teamFrom(template protocol.TeamTemplate, id, folder string, created time.Time, crew ...protocol.Member) protocol.Team {
	members := append([]protocol.Member(nil), template.Members...)
	for index := range members {
		members[index].ID, members[index].Harness, members[index].Model = crew[index].ID, crew[index].Harness, crew[index].Model
		members[index].Options = crew[index].Options
	}
	return protocol.Team{
		ID: id, Name: template.Name, Purpose: template.Purpose, Template: template.ID,
		Folder: paths.TeamsHome() + "/" + folder, Members: members, Tools: template.Tools,
		Approvals: template.Approvals, Budget: template.Budget, Notify: protocol.NotifyAll,
		Created: created, Updated: created,
	}
}

func demoTeams(now time.Time, settings protocol.Settings, events map[string][]protocol.SessionEvent) teamsFixtures {
	clock := demoTeamsClock(now)
	ago := func(d time.Duration) time.Time { return now.Add(-d) }
	at := func(t time.Time) *time.Time { return &t }
	templates := demoTemplates()

	research := teamFrom(templates[0], demoTeamResearch, "research-desk", now.AddDate(0, 0, -24),
		protocol.Member{ID: "member-editor", Harness: "claude", Model: "sonnet", Options: map[string]string{"effort": "high"}},
		protocol.Member{ID: "member-researcher", Harness: "claude", Model: "haiku"})
	mail := teamFrom(templates[1], demoTeamMail, "mail-desk", now.AddDate(0, 0, -30),
		protocol.Member{ID: "member-writer", Harness: "codex", Model: "gpt-5.6"})
	repo := teamFrom(templates[2], demoTeamRepo, "repo-watch", now.AddDate(0, 0, -40),
		protocol.Member{ID: "member-reviewer", Harness: "codex", Model: "gpt-5.6"},
		protocol.Member{ID: "member-triager", Harness: "claude", Model: "haiku"})
	system := teamFrom(templates[3], demoTeamSystem, "system-care", now.AddDate(0, 0, -18),
		protocol.Member{ID: "member-caretaker", Harness: "claude"})
	system.Paused, system.PauseReason, system.Updated = true, protocol.PausedByYou, ago(26*time.Hour)
	system.Notify = protocol.NotifyNeeds
	news := protocol.Team{
		ID: demoTeamNews, Name: i18n.T("News digest"),
		Purpose: i18n.T("Reads the news on the topics I follow and sends me a short digest every Monday."),
		Folder:  paths.TeamsHome() + "/news-digest",
		Members: []protocol.Member{
			{ID: "member-curator", Name: i18n.T("Curator"), Title: i18n.T("Picks what matters"), Lead: true, Harness: "claude", Model: "sonnet", Instructions: i18n.T("Chooses the five stories worth my time, writes the digest and links every source.")},
			{ID: "member-reader", Name: i18n.T("Reader"), Title: i18n.T("Reads the feeds"), Harness: "gemini", Model: "gemini-3-flash", Instructions: i18n.T("Reads the news API and the feeds, and hands the curator every story on my topics with its link.")},
		},
		Approvals: protocol.ApprovalsAsk, Budget: protocol.TeamBudget{MonthTokens: 1_500_000, WarnPercent: 80, TaskTurns: 20},
		Notify: protocol.NotifyAll, Created: now.AddDate(0, 0, -20), Updated: now.AddDate(0, 0, -3),
	}
	release := protocol.Team{
		ID: demoTeamRelease, Name: i18n.T("Release notes"),
		Purpose: i18n.T("Writes the release notes from the pull requests merged since the last release."),
		Folder:  paths.TeamsHome() + "/release-notes",
		Members: []protocol.Member{
			{ID: "member-scribe", Name: i18n.T("Scribe"), Title: i18n.T("Writes release notes"), Lead: true, Harness: "codex", Model: "gpt-5.6", Instructions: i18n.T("Reads the merged pull requests, groups them the way users notice them, and writes plain notes without internal jargon.")},
		},
		Approvals: protocol.ApprovalsReadOnly, Budget: protocol.TeamBudget{MonthTokens: 1_000_000, WarnPercent: 80, TaskTurns: 25},
		Notify: protocol.NotifyAll, Paused: true, PauseReason: protocol.PausedByBudget,
		Created: now.AddDate(0, 0, -12), Updated: ago(2 * time.Hour),
	}
	// made a few hours ago and never given a task, its one member on the teams default
	reading := protocol.Team{
		ID: demoTeamReading, Name: i18n.T("Reading list"),
		Purpose: i18n.T("Sorts the articles I save to read later by topic and writes one line on each."),
		Folder:  paths.TeamsHome() + "/reading-list",
		Members: []protocol.Member{
			{ID: "member-librarian", Name: i18n.T("Librarian"), Title: i18n.T("Sorts and sums up"), Lead: true, Instructions: i18n.T("Reads each saved article, files it under a topic and writes one plain line on what it says.")},
		},
		Approvals: protocol.ApprovalsReadOnly, Budget: settings.Teams.Defaults.Budget,
		Notify: protocol.NotifyAll, Created: ago(5 * time.Hour), Updated: ago(5 * time.Hour),
	}

	fx := teamsFixtures{
		teams:   []protocol.Team{research, mail, repo, system, news, release, reading},
		spend:   make(map[string]protocol.TeamSpend),
		actions: make(map[string]demoAction),
	}
	// every task is stored with the engine it started on, the session that worked it and that session's transcript
	add := func(team protocol.Team, task protocol.TeamTask, preview, activity string, lines ...line) {
		task.Team, task.MaxTurns = team.ID, team.Budget.TaskTurns
		if task.Started != nil {
			member, _ := teamMember(team, task.Member)
			startEngine(&task, member, settings, "")
		}
		if task.Session != "" {
			task.Sessions = []string{task.Session}
			transcript := teamTranscript(task, lines)
			events[task.Session] = transcript
			fx.sessions = append(fx.sessions, teamSession(team, task, transcript, preview, activity))
		}
		fx.tasks = append(fx.tasks, task)
	}
	ended := i18n.T("Your report is with the user. The task has ended.")
	blocked := i18n.T("The task is blocked. Rashin tells the user what you need.")
	routineBrief := func(routine protocol.RoutineDraft) string {
		return i18n.Tf("%s\n\nThis run is from the routine \"%s\"; when nothing changed since the last run, say so in one line in your report.", routine.Brief, routine.Title)
	}
	morningRoutine, eveningRoutine := templates[2].Routines[0], templates[3].Routines[0]
	weeklyRoutine := protocol.RoutineDraft{Title: i18n.T("Weekly digest"), Schedule: "0 8 * * 1", Brief: i18n.T("Write this week's digest: the five stories on my topics worth my time, one line each with its link.")}
	breakingRoutine := protocol.RoutineDraft{Title: i18n.T("Breaking alerts"), Schedule: "15 * * * *", Member: "member-reader", Brief: i18n.T("Check the technology headlines and tell me only about a story that changes something I use: a security fix, a release, a shutdown.")}

	// Research desk: the lead waits on two pieces, one done and one at work on its own screen
	add(research, protocol.TeamTask{
		ID: "task-heartbeat", Title: i18n.T("Compare Paperclip's heartbeat with cron"),
		Brief:  i18n.T("Paperclip wakes its agents with a heartbeat. Compare that with how cron and systemd timers run jobs: what each does with a missed run and with a run that is still going. Two pages at most, with sources."),
		Status: protocol.TaskWorking, Waiting: protocol.WaitingOnMembers, Member: "member-editor", Origin: protocol.TaskFromYou,
		Session: "agent-team-heartbeat", Usage: protocol.TokenUsage{Input: 41200, Output: 3900}, Turns: 1,
		Created: ago(26 * time.Minute), Started: at(ago(25 * time.Minute)), Updated: ago(22 * time.Minute),
	}, i18n.T("Both pieces are with the Researcher."), "",
		said(i18n.T("I'll split this in two: how Paperclip wakes its agents, and how cron and systemd timers treat runs they missed or that overlap. The Researcher takes both; I'll write the comparison from their notes.")),
		did("delegate-docs", "delegate", protocol.ToolOther, protocol.ToolDone, i18n.T("Researcher: Read how Paperclip's heartbeat works"), i18n.T("Started \"Read how Paperclip's heartbeat works\" for Researcher. End your turn now; Rashin continues you with their report.")),
		did("delegate-cron", "delegate", protocol.ToolOther, protocol.ToolDone, i18n.T("Researcher: Collect how cron and systemd timers treat missed runs"), i18n.T("Started \"Collect how cron and systemd timers treat missed runs\" for Researcher. End your turn now; Rashin continues you with their report.")),
		said(i18n.T("Both pieces are with the Researcher. I'll pick the task up again when they report.")),
	)
	add(research, protocol.TeamTask{
		ID: "task-heartbeat-docs", Title: i18n.T("Read how Paperclip's heartbeat works"),
		Brief:  i18n.T("Open the Paperclip repository on your screen and read how an agent is woken: the timer, assignments and on-demand wakeups, and what happens when a run is still going. Note every file you read."),
		Status: protocol.TaskWorking, Member: "member-researcher", Parent: "task-heartbeat", Origin: protocol.TaskFromLead,
		Session: "agent-team-heartbeat-docs", Usage: protocol.TokenUsage{Input: 58300, Output: 2100}, Turns: 2,
		Created: ago(24 * time.Minute), Started: at(ago(22 * time.Minute)), Updated: ago(time.Minute + 40*time.Second),
	}, i18n.T("A busy agent runs once more, not many times."), i18n.T("Handing in its report"),
		did("screen-open", "screen_open", protocol.ToolOther, protocol.ToolDone, "firefox https://github.com/paperclipai/paperclip", i18n.T("Firefox is open on your screen.")),
		did("screen-look", "screen_look", protocol.ToolOther, protocol.ToolDone, "", i18n.T("The repository page: the README and the docs and server folders.")),
		did("screen-docs", "screen_click", protocol.ToolOther, protocol.ToolDone, "docs/start/core-concepts.md", i18n.T("The core concepts page is open.")),
		said(i18n.T("Agents wake on a timer, on an assignment, on demand or from an automation. The timer is an interval per agent, not a schedule.")),
		continued(i18n.Tf("Continuing the goal, turn %d of %d", 2, research.Budget.TaskTurns)),
		did("screen-code", "screen_open", protocol.ToolOther, protocol.ToolDone, "firefox https://github.com/paperclipai/paperclip/blob/main/server/src/services/heartbeat.ts", i18n.T("heartbeat.ts is open: 4,643 lines.")),
		did("screen-scroll", "screen_scroll", protocol.ToolOther, protocol.ToolDone, "heartbeat.ts", i18n.T("A wakeup that arrives during a run waits in the queue.")),
		said(i18n.T("A wakeup that arrives while a run is going is queued and merged with the others, so a busy agent runs once more, not many times.")),
		did("report-docs", "report", protocol.ToolOther, protocol.ToolRunning, i18n.T("Paperclip wakes agents with a heartbeat"), ""),
	)
	add(research, protocol.TeamTask{
		ID: "task-heartbeat-cron", Title: i18n.T("Collect how cron and systemd timers treat missed runs"),
		Brief:  i18n.T("Find what cron, anacron and systemd timers do with a run missed while the computer was off, and with a run that falls due while the last one still works. Cite the manual pages."),
		Status: protocol.TaskDone, Member: "member-researcher", Parent: "task-heartbeat", Origin: protocol.TaskFromLead,
		Session: "agent-team-heartbeat-cron", Usage: protocol.TokenUsage{Input: 22800, Output: 3100}, Turns: 1,
		Report:  &protocol.TeamReport{Title: i18n.T("How cron and systemd timers treat missed runs"), Body: cronReport(), At: ago(9 * time.Minute)},
		Created: ago(24 * time.Minute), Started: at(ago(23 * time.Minute)), Ended: at(ago(9 * time.Minute)), Updated: ago(9 * time.Minute),
	}, i18n.T("cron skips, anacron catches up by the day, a persistent timer fires once at boot."), "",
		did("man-crontab", "man 5 crontab", protocol.ToolExecute, protocol.ToolDone, "man 5 crontab | col -b", i18n.T("cron runs nothing it missed.")),
		did("man-anacron", "man 8 anacron", protocol.ToolExecute, protocol.ToolDone, "man 8 anacron | col -b", i18n.T("anacron runs daily, weekly and monthly jobs once after boot when their period has passed.")),
		did("man-timer", "man 5 systemd.timer", protocol.ToolExecute, protocol.ToolDone, "man 5 systemd.timer | col -b", i18n.T("Persistent=true fires once at boot for a missed run.")),
		said(i18n.T("cron skips, anacron catches up by the day, and a persistent systemd timer fires once at boot. Neither systemd nor anacron overlaps runs.")),
		did("report-cron", "report", protocol.ToolOther, protocol.ToolDone, i18n.T("How cron and systemd timers treat missed runs"), i18n.T("Your report is with the editor. The task has ended.")),
	)
	add(research, protocol.TeamTask{
		ID: "task-coverage", Title: i18n.T("Find this week's coverage of Ryoku"),
		Brief:  i18n.T("Find what news sites and blogs wrote about Ryoku this week. List each piece with its link and one line on what it says."),
		Status: protocol.TaskNeedsYou, Waiting: protocol.WaitingOnYou, Member: "member-editor", Origin: protocol.TaskFromYou,
		Session: "agent-team-coverage", Usage: protocol.TokenUsage{Input: 9800, Output: 700}, Turns: 1,
		Created: ago(15 * time.Minute), Started: at(ago(14 * time.Minute)), Updated: ago(11 * time.Minute),
	}, i18n.T("The first request with NEWS_API_KEY waits for your answer."), "",
		said(i18n.T("I'll search this week's news for Ryoku through the news API.")),
		did("credentials-coverage", "credentials", protocol.ToolOther, protocol.ToolDone, "", i18n.T("NEWS_API_KEY reads headlines for the news digest and may go to newsapi.org.")),
		did("http-coverage", "http_request", protocol.ToolFetch, protocol.ToolDone, "GET https://newsapi.org/v2/everything?q=Ryoku&sortBy=publishedAt", i18n.T("Waiting for the user: Let Research desk send NEWS_API_KEY to newsapi.org. End your turn now; Rashin continues this task with their answer.")),
		said(i18n.T("The first request with NEWS_API_KEY waits for your answer.")),
	)
	add(research, protocol.TeamTask{
		ID: "task-capture-history", Title: i18n.T("Write a short history of Wayland screen capture"),
		Brief:  i18n.T("How did screen capture on Wayland get to the portal we use today? One page, with dates and links."),
		Status: protocol.TaskDone, Member: "member-editor", Origin: protocol.TaskFromYou, Read: true, Model: "opus",
		Session: "agent-team-capture-history", Usage: protocol.TokenUsage{Input: 63400, Output: 8800}, Turns: 1,
		Report: &protocol.TeamReport{
			Title: i18n.T("A short history of Wayland screen capture"), Body: waylandReport(),
			Files: []string{research.Folder + "/wayland-screen-capture.md"}, At: ago(47*time.Hour + 20*time.Minute),
		},
		Created: ago(48 * time.Hour), Started: at(ago(48*time.Hour - time.Minute)), Ended: at(ago(47*time.Hour + 20*time.Minute)), Updated: ago(47*time.Hour + 20*time.Minute),
	}, i18n.T("Three stages, from no capture to the portal."), "",
		did("fetch-protocol", "Fetch wayland.freedesktop.org", protocol.ToolFetch, protocol.ToolDone, "https://wayland.freedesktop.org/docs/html/", i18n.T("The protocol leaves screen access to the compositor.")),
		did("fetch-portal", "Fetch the ScreenCast portal docs", protocol.ToolFetch, protocol.ToolDone, "https://flatpak.github.io/xdg-desktop-portal/docs/doc-org.freedesktop.portal.ScreenCast.html", i18n.T("The ScreenCast portal hands the app a PipeWire stream.")),
		did("write-history", "Write wayland-screen-capture.md", protocol.ToolEdit, protocol.ToolDone, "wayland-screen-capture.md", ""),
		did("report-history", "report", protocol.ToolOther, protocol.ToolDone, i18n.T("A short history of Wayland screen capture"), ended),
	)

	// Mail desk: a mail waits for approval, one waits for a free slot, one went out yesterday
	add(mail, protocol.TeamTask{
		ID: "task-meetup", Title: i18n.T("Reply to Ana about the Lisbon meetup"),
		Brief:  i18n.T("Ana asked whether I can give a talk at the Lisbon Linux meetup on the 28th. Say yes, twenty minutes on running one desktop on two compositors, and ask about the projector and how long we have the room."),
		Status: protocol.TaskNeedsYou, Waiting: protocol.WaitingOnYou, Member: "member-writer", Origin: protocol.TaskFromYou,
		Session: "agent-team-meetup", Usage: protocol.TokenUsage{Input: 7400, Output: 1300}, Turns: 1,
		Created: ago(9 * time.Minute), Started: at(ago(8 * time.Minute)), Updated: ago(6 * time.Minute),
	}, i18n.T("The mail waits for your approval."), "",
		said(i18n.T("I'll keep it short and in your voice, and send it once you approve.")),
		did("mail-meetup", "send_mail", protocol.ToolOther, protocol.ToolDone, "ana.ribeiro@example.org", i18n.T("Waiting for the user: Send a mail to ana.ribeiro@example.org. End your turn now; Rashin continues you with the outcome.")),
		said(i18n.T("The mail waits for your approval.")),
	)
	add(mail, protocol.TeamTask{
		ID: "task-testers", Title: i18n.T("Draft the monthly update to the testers"),
		Brief:  i18n.T("Write this month's update to the Ryoku testers: what changed in 0.9.4, what we want tested, and how to report. Send it to testers@example.org."),
		Status: protocol.TaskReady, Member: "member-writer", Origin: protocol.TaskFromYou,
		Created: ago(3 * time.Minute), Updated: ago(3 * time.Minute),
	}, "", "")
	add(mail, protocol.TeamTask{
		ID: "task-print-shop", Title: i18n.T("Thank the print shop for the merch samples"),
		Brief:  i18n.T("Thank the print shop for the five merch samples, say the indigo one came out best, and ask for a quote on 200 stickers in that ink."),
		Status: protocol.TaskDone, Member: "member-writer", Origin: protocol.TaskFromYou, Read: true,
		Session: "agent-team-print-shop", Usage: protocol.TokenUsage{Input: 11200, Output: 1900}, Turns: 2,
		Report:  &protocol.TeamReport{Title: i18n.T("Sent the thank-you mail to the print shop"), Body: thanksReport(), At: ago(19*time.Hour - 3*time.Minute)},
		Created: ago(20 * time.Hour), Started: at(ago(20*time.Hour - time.Minute)), Ended: at(ago(19*time.Hour - 3*time.Minute)), Updated: ago(19*time.Hour - 3*time.Minute),
	}, i18n.T("Sent after your approval."), "",
		did("mail-print", "send_mail", protocol.ToolOther, protocol.ToolDone, "orders@riso.example.pt", i18n.T("Waiting for the user: Send a mail to orders@riso.example.pt. End your turn now; Rashin continues you with the outcome.")),
		continued(i18n.T("The user approved: Send a mail to orders@riso.example.pt. It was sent.")),
		did("report-print", "report", protocol.ToolOther, protocol.ToolDone, i18n.T("Sent the thank-you mail to the print shop"), ended),
	)

	// Repo watch: this morning's report waits unread, the triager is at work
	day := clock.morning.Format("2006-01-02")
	since := clock.previousMorning.UTC().Format(time.RFC3339)
	add(repo, protocol.TeamTask{
		ID: "task-review-morning", Title: morningRoutine.Title, Brief: routineBrief(morningRoutine),
		Status: protocol.TaskDone, Member: "member-reviewer", Origin: protocol.TaskFromRoutine, Routine: "routine-morning",
		Session: "agent-team-review-morning", Usage: protocol.TokenUsage{Input: 186000, Output: 9400}, Turns: 2,
		Report: &protocol.TeamReport{
			Title: i18n.T("Six PRs and three issues need you"), Body: reviewReport(day),
			Files: []string{repo.Folder + "/reviews/" + day + ".md"}, At: clock.morning.Add(14 * time.Minute),
		},
		Created: clock.morning, Started: at(clock.morning.Add(time.Minute)), Ended: at(clock.morning.Add(14 * time.Minute)), Updated: clock.morning.Add(14 * time.Minute),
	}, i18n.T("Nine items need a maintainer."), "",
		did("http-pulls", "http_request", protocol.ToolFetch, protocol.ToolDone, "GET https://api.github.com/search/issues?q=org:Ryoku-dev+is:pr+is:open+updated:>="+since, i18n.T("200: 23 pull requests.")),
		did("http-issues", "http_request", protocol.ToolFetch, protocol.ToolDone, "GET https://api.github.com/orgs/Ryoku-dev/issues?filter=all&state=open&since="+since, i18n.T("200: 41 issues.")),
		did("http-reviews", "http_request", protocol.ToolFetch, protocol.ToolDone, "GET https://api.github.com/repos/Ryoku-dev/ryoku/pulls/412/reviews", i18n.T("200: two approvals.")),
		said(i18n.T("Fourteen pull requests wait on their authors. Nine items need a maintainer; I'm writing them up with one line each and why.")),
		did("write-review", "Write reviews/"+day+".md", protocol.ToolEdit, protocol.ToolDone, "reviews/"+day+".md", ""),
		did("report-review", "report", protocol.ToolOther, protocol.ToolDone, i18n.T("Six PRs and three issues need you"), ended),
	)
	add(repo, protocol.TeamTask{
		ID: "task-review-previous", Title: morningRoutine.Title, Brief: routineBrief(morningRoutine),
		Status: protocol.TaskDone, Member: "member-reviewer", Origin: protocol.TaskFromRoutine, Routine: "routine-morning", Read: true,
		Session: "agent-team-review-previous", Usage: protocol.TokenUsage{Input: 121000, Output: 6800}, Turns: 2,
		Report:  &protocol.TeamReport{Title: i18n.T("Two PRs need you"), Body: previousReviewReport(), At: clock.previousMorning.Add(31 * time.Minute)},
		Created: clock.previousMorning, Started: at(clock.previousMorning.Add(time.Minute)), Ended: at(clock.previousMorning.Add(31 * time.Minute)), Updated: clock.previousMorning.Add(31 * time.Minute),
	}, i18n.T("Two PRs need you."), "",
		did("http-pulls-previous", "http_request", protocol.ToolFetch, protocol.ToolDone, "GET https://api.github.com/search/issues?q=org:Ryoku-dev+is:pr+is:open", i18n.T("200: 13 pull requests.")),
		did("ask-rebase", "ask_user", protocol.ToolOther, protocol.ToolDone, i18n.T("Comment on PR #409 asking for a rebase"), i18n.T("Waiting for the user: Comment on PR #409 asking for a rebase. End your turn now; Rashin continues this task with their answer.")),
		continued(i18n.T("The user said no: Comment on PR #409 asking for a rebase. Their note: Never comment for me. Put it in the report instead.")),
		said(i18n.T("Understood. The rebase ask goes in the report.")),
		did("report-previous", "report", protocol.ToolOther, protocol.ToolDone, i18n.T("Two PRs need you"), ended),
	)
	add(repo, protocol.TeamTask{
		ID: "task-duplicates", Title: i18n.T("Group the duplicate install failure issues"),
		Brief:  i18n.T("Several issues report the same installer failure at step 9 of 15. Find them all, group the duplicates under the oldest one, and tell me which are really different."),
		Status: protocol.TaskWorking, Member: "member-triager", Origin: protocol.TaskFromYou,
		Session: "agent-team-duplicates", Usage: protocol.TokenUsage{Input: 32500, Output: 1800}, Turns: 1,
		Created: ago(36 * time.Minute), Started: at(ago(35 * time.Minute)), Updated: ago(2 * time.Minute),
	}, i18n.T("Eight have the same hang; reading the other three."), i18n.T("Reading issue #1184"),
		did("http-search-install", "http_request", protocol.ToolFetch, protocol.ToolDone, "GET https://api.github.com/search/issues?q=org:Ryoku-dev+is:issue+%22step+9+of+15%22", i18n.T("200: 11 issues.")),
		did("http-oldest", "http_request", protocol.ToolFetch, protocol.ToolDone, "GET https://api.github.com/repos/Ryoku-dev/ryoku/issues/1170", i18n.T("200: the oldest report, on 0.9.2.")),
		said(i18n.T("Eleven issues mention step 9 of 15. Eight are the same hang while the session starts; I'm reading the other three.")),
		did("http-1184", "http_request", protocol.ToolFetch, protocol.ToolRunning, "GET https://api.github.com/repos/Ryoku-dev/ryoku/issues/1184/comments", ""),
	)

	// System care, paused by you: a blocked question and the last evening check
	add(system, protocol.TeamTask{
		ID: "task-disk", Title: i18n.T("Find what fills the disk"),
		Brief:  i18n.T("The disk was 92% full this afternoon. Find what grew and tell me what I can safely delete."),
		Status: protocol.TaskBlocked, Member: "member-caretaker", Origin: protocol.TaskFromYou,
		Reason:  i18n.T("I can't read /var/lib/docker without root, and it holds most of the space I could not account for. Run `sudo du -sh /var/lib/docker/*` and send me what it prints, or tell me to leave Docker out."),
		Session: "agent-team-disk", Usage: protocol.TokenUsage{Input: 27600, Output: 2400}, Turns: 1,
		Created: ago(28 * time.Hour), Started: at(ago(28*time.Hour - time.Minute)), Ended: at(ago(27 * time.Hour)), Updated: ago(27 * time.Hour),
	}, i18n.T("Blocked: /var/lib/docker needs root."), "",
		did("df", "df -h /", protocol.ToolExecute, protocol.ToolDone, "df -h /", "/dev/nvme0n1p2  466G  428G   38G  92% /"),
		did("du-var", "du -xh --max-depth=1 /var", protocol.ToolExecute, protocol.ToolDone, "du -xh --max-depth=1 /var | sort -h", "640M\t/var/log\n2.1G\t/var/cache\n61G\t/var/lib"),
		did("du-docker", "du -sh /var/lib/docker", protocol.ToolExecute, protocol.ToolFailed, "du -sh /var/lib/docker", "du: cannot read directory '/var/lib/docker': Permission denied"),
		did("goal-block", "goal", protocol.ToolOther, protocol.ToolDone, "block", blocked),
	)
	add(system, protocol.TeamTask{
		ID: "task-evening", Title: eveningRoutine.Title, Brief: routineBrief(eveningRoutine),
		Status: protocol.TaskDone, Member: "member-caretaker", Origin: protocol.TaskFromRoutine, Routine: "routine-evening", Read: true,
		Session: "agent-team-evening", Usage: protocol.TokenUsage{Input: 38900, Output: 2600}, Turns: 1,
		Report:  &protocol.TeamReport{Title: i18n.T("Nothing needs you tonight"), Body: eveningReport(), At: clock.evening.Add(12 * time.Minute), Quiet: true},
		Created: clock.evening, Started: at(clock.evening.Add(time.Minute)), Ended: at(clock.evening.Add(12 * time.Minute)), Updated: clock.evening.Add(12 * time.Minute),
	}, i18n.T("Nothing needs you tonight."), "",
		did("df-evening", "df -h /", protocol.ToolExecute, protocol.ToolDone, "df -h /", "/dev/nvme0n1p2  466G  284G  182G  61% /"),
		did("failed-units", "systemctl --failed", protocol.ToolExecute, protocol.ToolDone, "systemctl --failed --no-legend", i18n.T("0 loaded units listed.")),
		did("journal", "journalctl -p err", protocol.ToolExecute, protocol.ToolDone, "journalctl -p err --since yesterday --no-pager", "bluetoothd[812]: Failed to connect: Host is down (x3)"),
		did("updates", "checkupdates", protocol.ToolExecute, protocol.ToolDone, "checkupdates | wc -l", "14"),
		did("report-evening", "report", protocol.ToolOther, protocol.ToolDone, i18n.T("Nothing needs you tonight"), ended),
	)

	// News digest, at 85 percent of its limit: Monday's digest, a credential it asked for, a routine that kept failing
	add(news, protocol.TeamTask{
		ID: "task-digest", Title: weeklyRoutine.Title, Brief: routineBrief(weeklyRoutine),
		Status: protocol.TaskDone, Member: "member-curator", Origin: protocol.TaskFromRoutine, Routine: "routine-weekly", Read: true,
		Session: "agent-team-digest", Usage: protocol.TokenUsage{Input: 214000, Output: 12100}, Turns: 1,
		Report:  &protocol.TeamReport{Title: i18n.T("This week in Linux desktops"), Body: digestReport(), At: clock.weekly.Add(20 * time.Minute)},
		Created: clock.weekly, Started: at(clock.weekly.Add(time.Minute)), Ended: at(clock.weekly.Add(20 * time.Minute)), Updated: clock.weekly.Add(20 * time.Minute),
	}, i18n.T("Four stories and two reviews."), "",
		did("http-digest", "http_request", protocol.ToolFetch, protocol.ToolDone, "GET https://newsapi.org/v2/everything?q=linux+desktop&sortBy=popularity", i18n.T("200: 87 articles.")),
		said(i18n.T("Four stories are worth your time this week, and two reviews of Ryoku.")),
		did("report-digest", "report", protocol.ToolOther, protocol.ToolDone, i18n.T("This week in Linux desktops"), ended),
	)
	add(news, protocol.TeamTask{
		ID: "task-mastodon", Title: i18n.T("Add my Mastodon bookmarks to the digest"),
		Brief:  i18n.T("Add the posts I bookmark on Mastodon to the weekly digest, with the best three first."),
		Status: protocol.TaskNeedsYou, Waiting: protocol.WaitingOnYou, Member: "member-curator", Origin: protocol.TaskFromYou,
		Session: "agent-team-mastodon", Usage: protocol.TokenUsage{Input: 6100, Output: 600}, Turns: 1,
		Created: ago(42 * time.Minute), Started: at(ago(41 * time.Minute)), Updated: ago(40 * time.Minute),
	}, i18n.T("Asked you for a Mastodon token."), "",
		did("credentials-mastodon", "credentials", protocol.ToolOther, protocol.ToolDone, "", i18n.T("NEWS_API_KEY may go to newsapi.org. There is no Mastodon credential.")),
		did("request-mastodon", "request_credential", protocol.ToolOther, protocol.ToolDone, "MASTODON_TOKEN, mastodon.social", i18n.T("Waiting for the user: News digest asks for a credential: MASTODON_TOKEN. End your turn now; Rashin continues this task with their answer.")),
		said(i18n.T("I asked you for a Mastodon token that can read your bookmarks.")),
	)
	add(news, protocol.TeamTask{
		ID: "task-breaking", Title: breakingRoutine.Title, Brief: routineBrief(breakingRoutine),
		Status: protocol.TaskBlocked, Member: "member-reader", Origin: protocol.TaskFromRoutine, Routine: "routine-breaking",
		Reason:  i18n.T("newsapi.org answered 429 Too Many Requests: the free plan allows 100 requests a day and this routine used them up by morning. Run it less often or move NEWS_API_KEY to a paid plan."),
		Session: "agent-team-breaking", Usage: protocol.TokenUsage{Input: 4100, Output: 500}, Turns: 1,
		Created: clock.breaking, Started: at(clock.breaking.Add(time.Minute)), Ended: at(clock.breaking.Add(3 * time.Minute)), Updated: clock.breaking.Add(3 * time.Minute),
	}, i18n.T("newsapi.org answered 429."), "",
		did("http-breaking", "http_request", protocol.ToolFetch, protocol.ToolFailed, "GET https://newsapi.org/v2/top-headlines?category=technology", "429 Too Many Requests"),
		did("goal-block-breaking", "goal", protocol.ToolOther, protocol.ToolDone, "block", blocked),
	)
	add(news, protocol.TeamTask{
		ID: "task-colour-piece", Title: i18n.T("Write a short piece on colour management in Wayland"),
		Brief:  i18n.T("Colour management landed in the Wayland protocols this month. Write a short piece for the digest's readers, one page at most, with links."),
		Status: protocol.TaskNeedsYou, Waiting: protocol.WaitingOnYou, Member: "member-curator", Origin: protocol.TaskFromYou,
		Session: "agent-team-colour-piece", Usage: protocol.TokenUsage{Input: 18200, Output: 1400}, Turns: 1,
		Created: ago(18 * time.Minute), Started: at(ago(17 * time.Minute)), Updated: ago(13 * time.Minute),
	}, i18n.T("Asked you which angle the piece takes."), "",
		did("fetch-colour", "Fetch the colour management protocol", protocol.ToolFetch, protocol.ToolDone, "https://wayland.app/protocols/color-management-v1", i18n.T("An app can now say which colour space its pixels are in.")),
		said(i18n.T("One page fits one good angle, and there are three. I'll ask which one you want.")),
		did("ask-angle", "ask_user", protocol.ToolOther, protocol.ToolDone, i18n.T("Which angle should the piece take?"), i18n.T("Waiting for the user: Which angle should the piece take? End your turn now; Rashin continues this task with their answer.")),
		said(i18n.T("I asked you which angle the piece takes.")),
	)

	// Release notes stopped when it reached its spending limit
	add(release, protocol.TeamTask{
		ID: "task-release", Title: i18n.T("Write the release notes for 0.9.4"),
		Brief:  i18n.T("Write the release notes for 0.9.4 from the pull requests merged since 0.9.3. Group them the way users notice them."),
		Status: protocol.TaskStopped, Member: "member-scribe", Origin: protocol.TaskFromYou,
		Reason:  i18n.T("The team reached its spending limit."),
		Session: "agent-team-release", Usage: protocol.TokenUsage{Input: 318000, Output: 22400}, Turns: 9,
		Created: ago(3 * time.Hour), Started: at(ago(3*time.Hour - time.Minute)), Ended: at(ago(2 * time.Hour)), Updated: ago(2 * time.Hour),
	}, i18n.T("Half the groups are written."), "",
		did("fetch-merged", "Fetch merged pull requests", protocol.ToolFetch, protocol.ToolDone, "https://github.com/Ryoku-dev/ryoku/pulls?q=is:merged+milestone:0.9.4", i18n.T("148 merged pull requests.")),
		said(i18n.T("148 pull requests to read. I'm grouping them by what users will notice.")),
		did("edit-notes", "Edit RELEASE-0.9.4.md", protocol.ToolEdit, protocol.ToolDone, "RELEASE-0.9.4.md", ""),
		said(i18n.T("Half the groups are written.")),
	)
	sort.SliceStable(fx.tasks, func(i, j int) bool { return fx.tasks[i].Created.After(fx.tasks[j].Created) })

	fx.approvals = demoApprovals(clock, fx.actions, settings.Teams.ApprovalHours)
	fx.routines = []protocol.Routine{
		{
			ID: "routine-morning", Team: demoTeamRepo, Title: morningRoutine.Title, Brief: morningRoutine.Brief, Schedule: morningRoutine.Schedule,
			Enabled: true, Missed: protocol.MissedSkip, Overlap: protocol.OverlapSkip,
			LastRun: at(clock.morning), LastTask: "task-review-morning", LastStatus: protocol.TaskDone,
			Created: now.AddDate(0, 0, -40), Updated: now.AddDate(0, 0, -6),
		},
		{
			ID: "routine-evening", Team: demoTeamSystem, Title: eveningRoutine.Title, Brief: eveningRoutine.Brief, Schedule: eveningRoutine.Schedule,
			Missed: protocol.MissedOnce, Overlap: protocol.OverlapSkip, QuietStreak: 3,
			LastRun: at(clock.evening), LastTask: "task-evening", LastStatus: protocol.TaskDone,
			Created: now.AddDate(0, 0, -18), Updated: ago(29 * time.Hour),
		},
		{
			ID: "routine-breaking", Team: demoTeamNews, Title: breakingRoutine.Title, Brief: breakingRoutine.Brief, Member: breakingRoutine.Member,
			Schedule: breakingRoutine.Schedule, Missed: protocol.MissedSkip, Overlap: protocol.OverlapSkip, Failures: protocol.RoutineFailuresMax,
			LastRun: at(clock.breaking), LastTask: "task-breaking", LastStatus: protocol.TaskBlocked,
			Created: now.AddDate(0, 0, -6), Updated: clock.breaking.Add(3 * time.Minute),
		},
		{
			ID: "routine-weekly", Team: demoTeamNews, Title: weeklyRoutine.Title, Brief: weeklyRoutine.Brief, Schedule: weeklyRoutine.Schedule,
			Enabled: true, Missed: protocol.MissedOnce, Overlap: protocol.OverlapSkip,
			LastRun: at(clock.weekly), LastTask: "task-digest", LastStatus: protocol.TaskDone,
			Created: now.AddDate(0, 0, -20), Updated: now.AddDate(0, 0, -20),
		},
	}
	fx.vault = demoVault(clock)
	fx.vaultLog = demoVaultLog(clock)
	fx.activity = demoActivity(clock)
	for _, team := range fx.teams {
		fx.spend[team.ID] = demoTeamSpend(clock, team)
	}
	return fx
}

// what waits on the user, a choice among them, and two answers they gave
func demoApprovals(clock teamsClock, actions map[string]demoAction, hours int) []protocol.Approval {
	now := clock.now
	ago := func(d time.Duration) time.Time { return now.Add(-d) }
	at := func(t time.Time) *time.Time { return &t }
	approval := func(a protocol.Approval) protocol.Approval {
		// a team stopped at its limit stays paused until you answer, so its card never runs out
		if a.Kind != protocol.ApprovalBudget {
			a.Expires = a.Created.Add(time.Duration(hours) * time.Hour)
		}
		if a.Status == "" {
			a.Status = protocol.ApprovalPending
		}
		return a
	}
	actions["approval-meetup"] = demoAction{recipient: "ana.ribeiro@example.org"}
	actions["approval-newsapi"] = demoAction{credential: "credential-news", host: "newsapi.org"}
	actions["approval-mastodon"] = demoAction{request: protocol.AgentCredentialRequestParams{
		Name: "MASTODON_TOKEN", Reason: i18n.T("To read the posts you bookmark on Mastodon and add the best ones to the weekly digest."), Hosts: []string{"mastodon.social"},
	}}
	actions["approval-print-shop"] = demoAction{recipient: "orders@riso.example.pt"}
	approvals := []protocol.Approval{
		approval(protocol.Approval{
			ID: "approval-meetup", Team: demoTeamMail, Task: "task-meetup", Session: "agent-team-meetup", Member: "member-writer",
			Kind: protocol.ApprovalMail, Title: i18n.T("Send a mail to ana.ribeiro@example.org"),
			Detail: i18n.T("To: ana.ribeiro@example.org\nSubject: Re: A talk at the Lisbon Linux meetup\n\nHi Ana,\n\nThank you for asking me. Yes, I'd love to give a talk on the 28th: twenty minutes on running one desktop on two compositors, then questions.\n\nTwo things before I plan the slides: does the room have a projector with HDMI, and how long do we have it before the first talk starts? I'll bring the demo laptop.\n\nSee you in Lisbon,\nSam"),
			Always: i18n.T("Always let Mail desk mail ana.ribeiro@example.org"), Created: ago(6 * time.Minute),
		}),
		approval(protocol.Approval{
			ID: "approval-newsapi", Team: demoTeamResearch, Task: "task-coverage", Session: "agent-team-coverage", Member: "member-editor",
			Kind: protocol.ApprovalCredential, Title: i18n.T("Let Research desk send NEWS_API_KEY to newsapi.org"),
			Detail: i18n.Tf("GET %s\nHeaders: %s", "https://newsapi.org/v2/everything?q=Ryoku&sortBy=publishedAt", "Accept, X-Api-Key"),
			Always: i18n.T("Always let Research desk send NEWS_API_KEY to newsapi.org"), Created: ago(11 * time.Minute),
		}),
		approval(protocol.Approval{
			ID: "approval-angle", Team: demoTeamNews, Task: "task-colour-piece", Session: "agent-team-colour-piece", Member: "member-curator",
			Kind: protocol.ApprovalAction, Title: i18n.T("Which angle should the piece take?"),
			Detail: i18n.T("One page fits one angle. The other two get a line each at the end."),
			Options: []string{
				i18n.T("What changes for people who edit photos"), i18n.T("How the compositors got there"), i18n.T("What app developers have to do"),
			},
			Created: ago(13 * time.Minute),
		}),
		approval(protocol.Approval{
			ID: "approval-mastodon", Team: demoTeamNews, Task: "task-mastodon", Session: "agent-team-mastodon", Member: "member-curator",
			Kind: protocol.ApprovalRequest, Title: i18n.T("News digest asks for a credential: MASTODON_TOKEN"),
			Detail:  i18n.Tf("%s\nWhere it would go: %s", actions["approval-mastodon"].request.Reason, "mastodon.social"),
			Created: ago(40 * time.Minute), Unwatched: true,
		}),
		approval(protocol.Approval{
			ID: "approval-budget", Team: demoTeamRelease, Member: "member-scribe",
			Kind: protocol.ApprovalBudget, Title: i18n.T("Release notes reached its spending limit"),
			Detail:  i18n.Tf("%s of %s tokens this month.", "1,000,000", "1,000,000"),
			Created: ago(2 * time.Hour), Unwatched: true,
		}),
		approval(protocol.Approval{
			ID: "approval-print-shop", Team: demoTeamMail, Task: "task-print-shop", Session: "agent-team-print-shop", Member: "member-writer",
			Kind: protocol.ApprovalMail, Title: i18n.T("Send a mail to orders@riso.example.pt"),
			Detail: i18n.T("To: orders@riso.example.pt\nSubject: Thank you for the samples\n\nHello,\n\nThank you for the five samples, they arrived safely. The indigo print came out best by far.\n\nCould you send me a quote for 200 stickers, 6 by 6 cm, in that indigo ink, by the end of the month?\n\nBest,\nSam"),
			Always: i18n.T("Always let Mail desk mail orders@riso.example.pt"), Status: protocol.ApprovalApproved, Decision: protocol.DecideOnce,
			Result: i18n.T("Sent to orders@riso.example.pt."), Created: ago(20*time.Hour - 2*time.Minute), Decided: at(ago(19 * time.Hour)),
		}),
		approval(protocol.Approval{
			ID: "approval-rebase", Team: demoTeamRepo, Task: "task-review-previous", Session: "agent-team-review-previous", Member: "member-reviewer",
			Kind: protocol.ApprovalAction, Title: i18n.T("Comment on PR #409 asking for a rebase"),
			Detail: i18n.T("Comment on github.com/Ryoku-dev/ryoku/pull/409:\n\nThanks for this! It is 31 commits behind unstable-dev now; could you rebase it so we can merge?"),
			Status: protocol.ApprovalDenied, Decision: protocol.DecideDeny, Note: i18n.T("Never comment for me. Put it in the report instead."),
			Created: clock.previousMorning.Add(8 * time.Minute), Decided: at(clock.previousMorning.Add(20 * time.Minute)), Unwatched: true,
		}),
	}
	sort.SliceStable(approvals, func(i, j int) bool { return approvals[i].Created.After(approvals[j].Created) })
	return approvals
}

// the vault, open, with the three credentials the teams use and the mail account
func demoVault(clock teamsClock) protocol.VaultInfo {
	now := clock.now
	at := func(t time.Time) *time.Time { return &t }
	return protocol.VaultInfo{
		State: protocol.VaultOpen, Keeper: i18n.T("your login keyring"),
		Credentials: []protocol.Credential{
			{
				ID: "credential-github", Name: "GITHUB_TOKEN", Description: i18n.T("Reads pull requests and issues in the Ryoku-dev organisation."),
				Use: protocol.CredentialHeader, Header: "Authorization", Prefix: "Bearer ", Hosts: []string{"api.github.com"},
				Teams: []string{demoTeamRepo}, Trusted: []protocol.CredentialTrust{{Team: demoTeamRepo, Host: "api.github.com"}},
				Fingerprint: "3f9a1c07", Version: 2, Created: now.AddDate(0, 0, -40), Rotated: at(now.AddDate(0, 0, -9)),
				LastUsed: at(clock.morning.Add(6 * time.Minute)), Uses: 214,
			},
			{
				ID: "credential-news", Name: "NEWS_API_KEY", Description: i18n.T("Reads headlines for the news digest."),
				Use: protocol.CredentialHeader, Header: "X-Api-Key", Hosts: []string{"newsapi.org"},
				Teams: []string{demoTeamNews, demoTeamResearch}, Trusted: []protocol.CredentialTrust{{Team: demoTeamNews, Host: "newsapi.org"}},
				Fingerprint: "c4d70a5e", Version: 1, Created: now.AddDate(0, 0, -20),
				LastUsed: at(clock.weekly.Add(2 * time.Minute)), Uses: 96,
			},
			{
				ID: "credential-smtp", Name: "SMTP_PASSWORD", Description: i18n.T("The password of the desk@example.org mail account."),
				Use: protocol.CredentialMail, Fingerprint: "b21e6d94", Version: 1, Created: now.AddDate(0, 0, -30),
				LastUsed: at(now.Add(-19*time.Hour + time.Minute)), Uses: 12,
			},
		},
		Mail: &protocol.MailAccount{
			Address: "desk@example.org", Name: "Ryoku desk", Host: "smtp.example.org", Port: 465, Security: protocol.MailTLS,
			Username: "desk@example.org", Credential: "credential-smtp",
			Trusted: []protocol.MailTrust{{Team: demoTeamMail, Recipient: "testers@example.org"}},
		},
	}
}
