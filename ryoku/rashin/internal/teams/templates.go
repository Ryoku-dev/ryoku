package teams

import (
	i18n "ryoku-i18n"

	"ryoku-rashin/internal/protocol"
)

// templates are the ready-made teams a new one can start from, in the order
// they are offered. Members leave harness and model empty, so they follow
// the teams defaults; a budget names only the turns per task, the rest
// comes from the teams defaults too.
func templates() []protocol.TeamTemplate {
	return []protocol.TeamTemplate{
		{
			ID:          "research",
			Name:        i18n.T("Research desk"),
			Purpose:     i18n.T("Looks things up on the web and in my files, and writes short reports with sources."),
			Description: i18n.T("An editor and a researcher that can use its own screen."),
			Members: []protocol.Member{
				{
					Name: i18n.T("Editor"), Title: i18n.T("Lead editor"), Lead: true,
					Instructions: i18n.T("You plan the work and hand searches to the researcher. You check every source before it goes into the report, and you write the report."),
				},
				{
					Name: i18n.T("Researcher"), Title: i18n.T("Web and file researcher"),
					Instructions: i18n.T("You search the web and the user's files and read what you find. When a page needs a browser, you open it on your own screen. You note every source you use."),
				},
			},
			Tools:     protocol.TeamTools{Screen: true},
			Approvals: protocol.ApprovalsAsk,
			Budget:    protocol.TeamBudget{TaskTurns: 20},
		},
		{
			ID:          "mail",
			Name:        i18n.T("Mail desk"),
			Purpose:     i18n.T("Drafts mail for me and sends it once I approve."),
			Description: i18n.T("A writer that sends mail only after you approve it."),
			Members: []protocol.Member{
				{
					Name: i18n.T("Writer"), Title: i18n.T("Mail writer"), Lead: true,
					Instructions: i18n.T("You write clear, short mail in the user's voice. You never send a mail without the user's approval, and you say what you sent."),
				},
			},
			Tools:     protocol.TeamTools{Mail: true},
			Approvals: protocol.ApprovalsReadOnly,
			Budget:    protocol.TeamBudget{TaskTurns: 12},
		},
		{
			ID:          "repo",
			Name:        i18n.T("Repo watch"),
			Purpose:     i18n.T("Reviews open pull requests and issues in my GitHub organisation and tells me what needs me."),
			Description: i18n.T("A reviewer and a triager that read and never comment, with a weekday morning review you can turn on."),
			Members: []protocol.Member{
				{
					Name: i18n.T("Reviewer"), Title: i18n.T("Lead reviewer"), Lead: true,
					Instructions: i18n.T("You read pull requests and issues and judge which need a maintainer. You never comment, label or merge anything."),
				},
				{
					Name: i18n.T("Triager"), Title: i18n.T("Issue triager"),
					Instructions: i18n.T("You list new issues and pull requests and group the duplicates. You label nothing and change nothing."),
				},
			},
			Approvals: protocol.ApprovalsReadOnly,
			Budget:    protocol.TeamBudget{TaskTurns: 30},
			Routines: []protocol.RoutineDraft{{
				Title:    i18n.T("Morning review"),
				Brief:    i18n.T("Review the pull requests and issues opened or updated since the last review, and report which need me and why."),
				Schedule: "0 9 * * 1-5",
			}},
		},
		{
			ID:          "system",
			Name:        i18n.T("System care"),
			Purpose:     i18n.T("Looks after this computer: disk space, failed services, updates waiting, and tells me what to do."),
			Description: i18n.T("A caretaker that only reads, with an evening check you can turn on."),
			Members: []protocol.Member{
				{
					Name: i18n.T("Caretaker"), Title: i18n.T("System caretaker"), Lead: true,
					Instructions: i18n.T("You read the system's state with read-only commands and never change anything. When something needs fixing, you propose the exact commands the user may run."),
				},
			},
			Approvals: protocol.ApprovalsReadOnly,
			Budget:    protocol.TeamBudget{TaskTurns: 15},
			Routines: []protocol.RoutineDraft{{
				Title:    i18n.T("Evening check"),
				Brief:    i18n.T("Check disk space, failed services, journal errors since yesterday and pending updates, and report anything I should act on."),
				Schedule: "0 18 * * *",
			}},
		},
		{
			ID:          "blank",
			Name:        i18n.T("New team"),
			Description: i18n.T("One lead and nothing else, for a team you set up yourself."),
			Members: []protocol.Member{
				{Name: i18n.T("Lead"), Title: i18n.T("Team lead"), Lead: true},
			},
			Approvals: protocol.ApprovalsAsk,
			Budget:    protocol.TeamBudget{TaskTurns: 20},
		},
	}
}

// templateByID finds a template; the empty id is no template.
func templateByID(id string) (protocol.TeamTemplate, bool) {
	for _, t := range templates() {
		if t.ID == id {
			return t, true
		}
	}
	return protocol.TeamTemplate{}, false
}
