package demo

import (
	"encoding/json"
	"slices"
	"sort"
	"strings"
	"time"

	"ryoku-i18n"

	"ryoku-rashin/internal/paths"
	"ryoku-rashin/internal/protocol"
)

// the demo's dreams, named so routes and stories can open them
const (
	demoDreamESP32    = "dream-esp32"
	demoDreamReceipts = "dream-receipts"
)

// the options a presentation's Inbox card asks with, as the daemon asks them
func presentOptions() []string { return []string{i18n.T("Accept"), i18n.T("Send notes")} }

// two dreams: one building with a question waiting, one presenting what it built
func addDemoDreams(result *fixtures, now time.Time, settings protocol.Settings) {
	fx := &result.teams
	ago := func(d time.Duration) time.Time { return now.Add(-d) }
	at := func(t time.Time) *time.Time { return &t }
	dream := func(id, name string, started time.Time, members []protocol.Member, state protocol.Dream) protocol.Team {
		state.Manager, state.Task, state.Started = members[0].ID, id+"-run", started
		return protocol.Team{
			ID: id, Name: name, Purpose: state.Statement, Folder: paths.DreamsHome() + "/" + slugName(name), Members: members,
			Approvals: protocol.ApprovalsAuto, Budget: protocol.TeamBudget{WarnPercent: 80}, Notify: protocol.NotifyAll,
			Created: started, Updated: now, Dream: &state,
		}
	}
	manager := func(id, harness, model string) protocol.Member {
		return protocol.Member{ID: id, Name: i18n.T("Manager"), Title: i18n.T("Runs the dream"), Lead: true, Harness: harness, Model: model}
	}

	esp32 := dream(demoDreamESP32, i18n.T("ESP32 Wi-Fi setup page"), ago(4*24*time.Hour), []protocol.Member{
		manager("member-esp32-manager", "claude", "opus"),
		{ID: "member-esp32-firmware", Name: i18n.T("Firmware engineer"), Title: i18n.T("Writes the ESP-IDF firmware"), Harness: "claude", Model: "sonnet"},
		{ID: "member-esp32-page", Name: i18n.T("Page designer"), Title: i18n.T("Builds the setup page"), Harness: "codex", Model: "gpt-5.6"},
		{ID: "member-esp32-tester", Name: i18n.T("Tester"), Title: i18n.T("Tests it in QEMU"), Harness: "claude", Model: "haiku"},
	}, protocol.Dream{
		Statement: i18n.T("Make ESP32 firmware whose Wi-Fi setup page opens when I connect to the board's own network: it lists the networks it can see, I pick one and type the password, and it joins and remembers it. Build it with ESP-IDF and keep the page small enough to live in flash."),
		Brief:     i18n.T("ESP32-S3, ESP-IDF 5.3. A captive portal on the board's access point; the page under 20 KB; the network saved in NVS; back to the portal after three failed joins."),
		Phase:     protocol.DreamBuilding, Hired: 3,
		Summary: i18n.T("The portal opens on a phone and lists the networks around it. Saving a network and the page itself are being built."),
	})
	receipts := dream(demoDreamReceipts, i18n.T("Receipt renamer"), ago(6*24*time.Hour), []protocol.Member{
		manager("member-receipts-manager", "claude", "sonnet"),
		{ID: "member-receipts-developer", Name: i18n.T("Developer"), Title: i18n.T("Writes the tool and its tests"), Harness: "codex", Model: "gpt-5.6"},
	}, protocol.Dream{
		Statement: i18n.T("A small tool that renames the receipts I scan into ~/Documents/receipts by the date and the shop on them, so I can find one by its name."),
		Phase:     protocol.DreamPresenting, Hired: 2, Rounds: 1,
		Summary: i18n.T("receipt-namer is built and tested on your 214 scans. It is ready for you to try."),
		Presentation: &protocol.DreamPresentation{
			Title: i18n.T("receipt-namer renames your scans by date and shop"),
			Body:  receiptsPresentation(),
			Tests: i18n.T("`go test ./...` passes: 38 tests, among them the 12 receipts that read badly. A dry run over all 214 scans renamed 212 and left 2 it could not read, listed in `unread.txt`."),
			Files: []string{
				paths.DreamsHome() + "/receipt-renamer/receipt-namer/README.md",
				paths.DreamsHome() + "/receipt-renamer/receipt-namer/main.go",
				paths.DreamsHome() + "/receipt-renamer/unread.txt",
			},
			At: ago(25 * time.Minute),
		},
	})
	fx.teams = append(fx.teams, esp32, receipts)

	run := func(team protocol.Team, task protocol.TeamTask, preview string, lines ...line) {
		task.Origin = protocol.TaskFromYou
		fx.addTask(settings, result.events, team, task, preview, "", lines...)
	}
	piece := func(team protocol.Team, task protocol.TeamTask, preview, activity string, lines ...line) {
		task.Parent, task.Origin = team.Dream.Task, protocol.TaskFromLead
		fx.addTask(settings, result.events, team, task, preview, activity, lines...)
	}
	waiting := i18n.T("Waiting for the user. End your turn now; Rashin continues you with their answer.")
	run(esp32, protocol.TeamTask{
		ID: esp32.Dream.Task, Title: esp32.Name, Brief: esp32.Dream.Statement, Member: "member-esp32-manager",
		Status: protocol.TaskNeedsYou, Waiting: protocol.WaitingOnYou, Session: "agent-dream-esp32-run",
		Usage: protocol.TokenUsage{Input: 412000, Output: 31000}, Turns: 14,
		Created: esp32.Created, Started: at(esp32.Created), Updated: ago(6 * time.Minute),
	}, i18n.T("Asked you which look the setup page takes."),
		said(i18n.T("The interview settled the board, the size of the page and what happens when a join fails. Hiring three: firmware, the page, and tests.")),
		did("hire-firmware", "hire", protocol.ToolOther, protocol.ToolDone, i18n.T("Firmware engineer"), i18n.T("Hired Firmware engineer on Claude Code, sonnet.")),
		did("hire-page", "hire", protocol.ToolOther, protocol.ToolDone, i18n.T("Page designer"), i18n.T("Hired Page designer on Codex, gpt-5.6.")),
		did("hire-tester", "hire", protocol.ToolOther, protocol.ToolDone, i18n.T("Tester"), i18n.T("Hired Tester on Claude Code, haiku.")),
		did("delegate-portal", "delegate", protocol.ToolOther, protocol.ToolDone, i18n.T("Firmware engineer: Serve the captive portal and scan for networks"), ""),
		did("delegate-page", "delegate", protocol.ToolOther, protocol.ToolDone, i18n.T("Page designer: Build the setup page under 20 KB"), ""),
		did("dream-building", "dream_status", protocol.ToolOther, protocol.ToolDone, i18n.T("building, 45%"), ""),
		did("ask-look", "ask_user", protocol.ToolOther, protocol.ToolDone, i18n.T("Which look should the setup page take?"), waiting),
	)
	piece(esp32, protocol.TeamTask{
		ID: "task-esp32-portal", Title: i18n.T("Serve the captive portal and scan for networks"), Member: "member-esp32-firmware",
		Brief:  i18n.T("Bring up the access point, answer every DNS query with the board so phones open the portal, and serve the scan list as JSON at /networks."),
		Status: protocol.TaskWorking, Session: "agent-dream-esp32-portal", Usage: protocol.TokenUsage{Input: 188000, Output: 14200}, Turns: 6,
		Created: ago(3 * time.Hour), Started: at(ago(3 * time.Hour)), Updated: ago(time.Minute),
	}, i18n.T("Saving the chosen network in NVS."), i18n.T("Writing wifi_store.c"),
		did("idf-build", "idf.py build", protocol.ToolExecute, protocol.ToolDone, "idf.py build", i18n.T("Project build complete.")),
		said(i18n.T("The portal opens on a phone and /networks lists eleven networks. Saving the chosen one next.")),
		did("write-store", "Write main/wifi_store.c", protocol.ToolEdit, protocol.ToolRunning, "main/wifi_store.c", ""),
	)
	piece(esp32, protocol.TeamTask{
		ID: "task-esp32-page", Title: i18n.T("Build the setup page under 20 KB"), Member: "member-esp32-page",
		Brief:  i18n.T("One HTML page with inline CSS and script: the network list from /networks, a password field, and Join. Gzipped under 20 KB."),
		Status: protocol.TaskWorking, Session: "agent-dream-esp32-page", Usage: protocol.TokenUsage{Input: 96000, Output: 8800}, Turns: 3,
		Created: ago(3 * time.Hour), Started: at(ago(2 * time.Hour)), Updated: ago(4 * time.Minute),
	}, i18n.T("The page is 14 KB gzipped."), i18n.T("Measuring the gzipped page"),
		did("page-size", "gzip -9 | wc -c", protocol.ToolExecute, protocol.ToolDone, "gzip -9 < page/index.html | wc -c", "14382"),
	)
	piece(esp32, protocol.TeamTask{
		ID: "task-esp32-bench", Title: i18n.T("Set up the QEMU test bench"), Member: "member-esp32-tester",
		Brief:  i18n.T("Run the firmware in Espressif's QEMU with the network emulated, so the join and reconnect tests run without a board."),
		Status: protocol.TaskDone, Session: "agent-dream-esp32-bench", Usage: protocol.TokenUsage{Input: 52000, Output: 4100}, Turns: 2,
		Report:  &protocol.TeamReport{Title: i18n.T("The QEMU bench runs the firmware"), Body: i18n.T("`qemu-system-xtensa` boots the build with the open-eth network. `pytest-embedded` drives the portal; two smoke tests pass."), At: ago(70 * time.Minute)},
		Created: ago(3 * time.Hour), Started: at(ago(3 * time.Hour)), Ended: at(ago(70 * time.Minute)), Updated: ago(70 * time.Minute),
	}, i18n.T("Two smoke tests pass."), "",
		did("report-bench", "report", protocol.ToolOther, protocol.ToolDone, i18n.T("The QEMU bench runs the firmware"), ""),
	)

	run(receipts, protocol.TeamTask{
		ID: receipts.Dream.Task, Title: receipts.Name, Brief: receipts.Dream.Statement, Member: "member-receipts-manager",
		Status: protocol.TaskNeedsYou, Waiting: protocol.WaitingOnYou, Session: "agent-dream-receipts-run",
		Usage: protocol.TokenUsage{Input: 236000, Output: 19000}, Turns: 11,
		Created: receipts.Created, Started: at(receipts.Created), Updated: ago(25 * time.Minute),
	}, i18n.T("Presented receipt-namer."),
		said(i18n.T("Built and tested on your scans. Presenting it now.")),
		did("present", "present", protocol.ToolOther, protocol.ToolDone, receipts.Dream.Presentation.Title, waiting),
	)
	piece(receipts, protocol.TeamTask{
		ID: "task-receipts-tool", Title: i18n.T("Write receipt-namer and its tests"), Member: "member-receipts-developer",
		Brief:  i18n.T("Read the date and the shop from each scan's text layer, fall back to OCR, and rename to 2026-11-03 Lidl.pdf. A dry run first."),
		Status: protocol.TaskDone, Session: "agent-dream-receipts-tool", Usage: protocol.TokenUsage{Input: 141000, Output: 12700}, Turns: 5,
		Report:  &protocol.TeamReport{Title: i18n.T("receipt-namer is written"), Body: i18n.T("Renames 212 of 214 scans in the dry run. 38 tests pass."), At: ago(2 * time.Hour)},
		Created: ago(26 * time.Hour), Started: at(ago(26 * time.Hour)), Ended: at(ago(2 * time.Hour)), Updated: ago(2 * time.Hour),
	}, i18n.T("212 of 214 renamed in the dry run."), "",
		did("report-tool", "report", protocol.ToolOther, protocol.ToolDone, i18n.T("receipt-namer is written"), ""),
	)

	fx.approvals = append(fx.approvals,
		protocol.Approval{
			ID: "approval-esp32-look", Team: esp32.ID, Task: esp32.Dream.Task, Session: "agent-dream-esp32-run", Member: "member-esp32-manager",
			Kind: protocol.ApprovalAction, Title: i18n.T("Which look should the setup page take?"),
			Detail:  i18n.T("The page has to fit in about 20 KB of flash, so it can carry one look. Pick the one you want to see on your phone."),
			Options: []string{i18n.T("Plain, like a router's page"), i18n.T("Ryoku's dark look"), i18n.T("As small as it can be")},
			Status:  protocol.ApprovalPending, Created: ago(6 * time.Minute), Expires: ago(6 * time.Minute).Add(time.Duration(settings.Teams.ApprovalHours) * time.Hour),
		},
		protocol.Approval{
			ID: "approval-receipts-present", Team: receipts.ID, Task: receipts.Dream.Task, Session: "agent-dream-receipts-run", Member: "member-receipts-manager",
			Kind: protocol.ApprovalAction, Title: i18n.Tf("%s is ready: %s", receipts.Name, receipts.Dream.Presentation.Title),
			Detail: receipts.Dream.Presentation.Body, Options: presentOptions(), Status: protocol.ApprovalPending, Created: ago(25 * time.Minute),
		},
	)
	fx.activity = append([]protocol.Activity{
		{Time: ago(6 * time.Minute), Actor: i18n.T("Manager"), Initiator: protocol.InitiatorAgent, Action: "approval.ask", Target: i18n.T("Which look should the setup page take?"), OK: true, Team: esp32.ID, Task: esp32.Dream.Task},
		{Time: ago(25 * time.Minute), Actor: i18n.T("Manager"), Initiator: protocol.InitiatorAgent, Action: "dream.present", Target: receipts.Dream.Presentation.Title, OK: true, Team: receipts.ID, Task: receipts.Dream.Task},
		{Time: ago(3 * time.Hour), Actor: i18n.T("Manager"), Initiator: protocol.InitiatorAgent, Action: "dream.hire", Target: i18n.T("Tester"), OK: true, Team: esp32.ID},
	}, fx.activity...)
	fx.spend[esp32.ID] = dreamSpend(now, esp32, 690_000, 1)
	fx.spend[receipts.ID] = dreamSpend(now, receipts, 409_000, 2)
	signInDreams(result.snapshot.Harnesses, now, esp32, receipts)
	sort.SliceStable(fx.tasks, func(i, j int) bool { return fx.tasks[i].Created.After(fx.tasks[j].Created) })
}

// a dream's month: its tokens spread over the days since it started, its finished tasks on the last of them
func dreamSpend(now time.Time, dream protocol.Team, tokens int64, finished int) protocol.TeamSpend {
	spend := protocol.TeamSpend{Team: dream.ID, Month: demoMonth(now), Cap: dream.Budget.MonthTokens, Days: monthDays(now), Tokens: tokens}
	weights := dayWeights(spend.Days, len(dream.ID), false)
	started := dream.Created.In(time.Local).Format("2006-01-02")
	for index := range spend.Days {
		if spend.Days[index].Day < started {
			weights[index] = 0
		}
	}
	for index, day := range spread(tokens, weights) {
		spend.Days[index].Input, spend.Days[index].Output = day*4/5, day-day*4/5
	}
	spend.Days[len(spend.Days)-1].Tasks = finished
	return spend
}

// the demo's dreams signed in for every harness their members run on
func signInDreams(harnesses []protocol.HarnessInfo, now time.Time, dreams ...protocol.Team) {
	at := now.Add(-4 * 24 * time.Hour)
	for index := range harnesses {
		teams := harnesses[index].Teams
		if teams == nil {
			continue
		}
		for _, dream := range dreams {
			if slices.ContainsFunc(dream.Members, func(member protocol.Member) bool { return member.Harness == harnesses[index].ID }) {
				teams.Homes = append(teams.Homes, protocol.TeamHome{Team: dream.ID, Home: demoTeamsHome(harnesses[index].ID, dream.ID), SignedIn: &at})
			}
		}
	}
}

func receiptsPresentation() string {
	return i18n.T(`receipt-namer reads each scan in ~/Documents/receipts and renames it by **the date and the shop** on it, so ` + "`scan-0412.pdf`" + ` becomes ` + "`2026-11-03 Lidl.pdf`" + `.

## How to use it

1. Try it first: ` + "`receipt-namer --dry-run ~/Documents/receipts`" + ` prints what it would rename.
2. Rename for real: ` + "`receipt-namer ~/Documents/receipts`" + `.
3. Two receipts of one shop on one day get ` + "`(2)`" + ` after the name.

## What it could not read

Two faded scans have no readable date. They stay as they are and are listed in ` + "`unread.txt`" + `.`)
}

// the dream methods the demo answers itself, and what a dream changes in pause and approvals; false leaves the call to callTeams
func (s *server) callDreams(method string, raw json.RawMessage) (any, bool, error) {
	switch method {
	case protocol.MTeamsDreamCreate:
		var p protocol.DreamCreateParams
		if err := decode(raw, &p); err != nil {
			return nil, true, err
		}
		team, err := s.createDream(p)
		return team, true, err
	case protocol.MTeamsDreamAnswer:
		var p protocol.DreamAnswerParams
		if err := decode(raw, &p); err != nil {
			return nil, true, err
		}
		team, err := s.answerDream(p.Team, p.Verdict, strings.TrimSpace(p.Note))
		return team, true, err
	case protocol.MTeamsPause:
		var p protocol.TeamPauseParams
		if err := decode(raw, &p); err != nil || s.dreamIndex(p.ID) < 0 {
			return nil, false, nil
		}
		team, err := s.pauseDream(p)
		return team, true, err
	case protocol.MTeamsApprovalDecide:
		var p protocol.ApprovalDecideParams
		if err := decode(raw, &p); err != nil {
			return nil, false, nil
		}
		index := s.data.teams.approvalIndex(p.ID)
		if index < 0 || !s.presentationApproval(s.data.teams.approvals[index]) {
			return nil, false, nil
		}
		approval := s.data.teams.approvals[index]
		verdict := protocol.DreamNotes
		if p.Decision == protocol.DecideChoose && p.Choice == presentOptions()[0] {
			verdict = protocol.DreamAccepted
		}
		if _, err := s.answerDream(approval.Team, verdict, strings.TrimSpace(p.Note)); err != nil {
			return nil, true, err
		}
		return s.data.teams.approvals[index], true, nil
	}
	return nil, false, nil
}

func (s *server) dreamIndex(id string) int {
	index := s.data.teams.teamIndex(id)
	if index < 0 || s.data.teams.teams[index].Dream == nil {
		return -1
	}
	return index
}

// the open card that carries a dream's presentation
func (s *server) presentationApproval(approval protocol.Approval) bool {
	index := s.dreamIndex(approval.Team)
	return index >= 0 && approval.Status == protocol.ApprovalPending && approval.Task == s.data.teams.teams[index].Dream.Task &&
		slices.Equal(approval.Options, presentOptions())
}

// a dream as the daemon starts one: a team of its manager alone, run without confirmations, its standing task started or waiting for a sign-in
func (s *server) createDream(p protocol.DreamCreateParams) (protocol.Team, error) {
	t := &s.data.teams
	statement := strings.TrimSpace(p.Statement)
	if statement == "" {
		return protocol.Team{}, badRequest(i18n.T("Say what you dream of first."))
	}
	if !protocol.ValidEngineModel(p.Model) || !protocol.ValidEngineOptions(p.Options) {
		return protocol.Team{}, badRequest(i18n.T("That model or option cannot be used."))
	}
	name := strings.TrimSpace(p.Name)
	if name == "" {
		name = protocol.BriefTitle(statement)
	}
	now := time.Now()
	manager := protocol.Member{
		ID: t.newID("member"), Name: i18n.T("Manager"), Title: i18n.T("Runs the dream"), Lead: true,
		Harness: p.Harness, Model: p.Model, Options: p.Options,
	}
	team := protocol.Team{
		ID: t.newID("dream"), Name: name, Purpose: statement, Folder: p.Folder, Members: []protocol.Member{manager},
		Approvals: protocol.ApprovalsAuto, Budget: protocol.TeamBudget{MonthTokens: p.MonthTokens, WarnPercent: 80},
		Notify: protocol.NotifyAll, Created: now, Updated: now,
		Dream: &protocol.Dream{Statement: statement, Phase: protocol.DreamInterviewing, Manager: manager.ID, Started: now},
	}
	if team.Folder == "" {
		team.Folder = paths.DreamsHome() + "/" + slugName(name)
	}
	t.spend[team.ID] = protocol.TeamSpend{Team: team.ID, Month: demoMonth(Epoch), Cap: team.Budget.MonthTokens, Days: monthDays(Epoch)}
	s.record(protocol.Activity{Action: "dream.start", Target: team.Name, OK: true, Team: team.ID, Detail: i18n.Tf("Started the dream, working in %s.", paths.Short(team.Folder))})
	team.Dream.Task = s.startDream(team).ID
	t.teams = append(t.teams, team)
	s.emitTeam(team.ID)
	return s.teamWithStats(team), nil
}

// the manager's standing task: at work at once, or blocked until its harness is signed in for the dream
func (s *server) startDream(team protocol.Team) protocol.TeamTask {
	t := &s.data.teams
	manager := team.Members[0]
	now := time.Now()
	task := protocol.TeamTask{
		ID: t.newID("task"), Team: team.ID, Title: team.Name, Brief: team.Dream.Statement, Member: manager.ID,
		Origin: protocol.TaskFromYou, Created: now, Updated: now,
	}
	harnessID, _, _ := protocol.ResolveEngine(manager, s.data.snapshot.Settings, s.registryHarness())
	at := slices.IndexFunc(s.data.snapshot.Harnesses, func(harness protocol.HarnessInfo) bool { return harness.ID == harnessID })
	if at >= 0 && s.data.snapshot.Harnesses[at].Teams != nil {
		harness := s.data.snapshot.Harnesses[at]
		if home, ok := harness.Teams.HomeOf(team.ID); !ok || home.SignedIn == nil {
			task.Status, task.SignIn, task.Ended = protocol.TaskBlocked, harness.ID, &now
			task.Reason = i18n.Tf("%s is not signed in for %s. Sign it in, then resume the dream.", harness.Name, team.Name)
			t.tasks = append([]protocol.TeamTask{task}, t.tasks...)
			s.emitTask(task)
			return task
		}
	}
	task = s.beginTeamTask(team, task)
	t.tasks = append([]protocol.TeamTask{task}, t.tasks...)
	s.emitTask(task)
	return task
}

// a dream blocked only on this sign-in goes on by itself, its blocked attempt taken back as the daemon does
func (s *server) continueSignedInDream(id, harness string) {
	t := &s.data.teams
	index := s.dreamIndex(id)
	if index < 0 {
		return
	}
	dream := t.teams[index]
	at := t.taskIndex(dream.Dream.Task)
	if at < 0 || dream.Paused || t.tasks[at].Status != protocol.TaskBlocked || t.tasks[at].SignIn != harness {
		return
	}
	task := t.tasks[at]
	task.SignIn, task.Reason, task.Ended = "", "", nil
	t.tasks[at] = s.beginTeamTask(dream, task)
	s.emitTask(t.tasks[at])
}

// a dream resumed starts its manager again when the standing task is not running
func (s *server) pauseDream(p protocol.TeamPauseParams) (protocol.Team, error) {
	t := &s.data.teams
	team, err := s.pauseTeam(p)
	if err != nil || p.Paused {
		return team, err
	}
	index := s.dreamIndex(p.ID)
	dream := t.teams[index]
	if dream.Dream.Done != nil || dream.Dream.Phase == protocol.DreamPresenting {
		return s.teamWithStats(dream), nil
	}
	if task := t.taskIndex(dream.Dream.Task); task >= 0 && taskOpen(t.tasks[task]) {
		return s.teamWithStats(dream), nil
	}
	state := *dream.Dream
	state.Task = s.startDream(dream).ID
	t.teams[index].Dream = &state
	s.record(protocol.Activity{Action: "dream.resume", Target: dream.Name, OK: true, Team: dream.ID})
	s.emitTeam(dream.ID)
	return s.teamWithStats(t.teams[index]), nil
}

// accepting ends the dream and its standing task; notes send the manager back to work with them
func (s *server) answerDream(id, verdict, note string) (protocol.Team, error) {
	t := &s.data.teams
	index := s.dreamIndex(id)
	if index < 0 {
		return protocol.Team{}, notFound("dream")
	}
	team := t.teams[index]
	state := *team.Dream
	if state.Presentation == nil || state.Presentation.Verdict != "" || state.Done != nil {
		return protocol.Team{}, badRequest(i18n.T("This dream has nothing waiting for your answer."))
	}
	if verdict != protocol.DreamAccepted && verdict != protocol.DreamNotes {
		return protocol.Team{}, badRequest(i18n.T("Accept the dream or send notes."))
	}
	now := time.Now()
	presentation := *state.Presentation
	presentation.Verdict, presentation.Note = verdict, note
	state.Presentation = &presentation
	choice := presentOptions()[1]
	if verdict == protocol.DreamAccepted {
		choice = presentOptions()[0]
		state.Phase, state.Done = protocol.DreamDone, &now
		s.record(protocol.Activity{Action: "dream.accept", Target: team.Name, OK: true, Team: team.ID, Detail: i18n.Tf("You accepted %s", presentation.Title)})
	} else {
		state.Phase = protocol.DreamBuilding
		s.record(protocol.Activity{Action: "dream.notes", Target: team.Name, OK: true, Team: team.ID, Detail: note})
	}
	team.Dream, team.Updated = &state, now
	t.teams[index] = team
	for approval := range t.approvals {
		if s.presentationApproval(t.approvals[approval]) {
			t.approvals[approval].Choice = choice
			s.settleApproval(approval, protocol.DecideChoose, note)
		}
	}
	if task := t.taskIndex(state.Task); task >= 0 {
		if verdict == protocol.DreamAccepted {
			done := t.tasks[task]
			done.Status, done.Waiting, done.Ended, done.Updated = protocol.TaskDone, "", &now, now
			t.tasks[task] = done
			s.settleTeamSession(done)
			s.emitTask(done)
		} else {
			text := i18n.T("The user sent notes on your presentation. Work them in, test again and present again.")
			if note != "" {
				text += "\n" + i18n.Tf("Their notes: %s", note)
			}
			s.releaseTeamTask(state.Task, text)
		}
	}
	s.emitTeam(team.ID)
	return s.teamWithStats(team), nil
}
