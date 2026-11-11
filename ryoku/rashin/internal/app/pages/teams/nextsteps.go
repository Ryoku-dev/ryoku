package teams

import (
	"slices"

	"gioui.org/layout"

	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/kit"
	"ryoku-i18n"

	"ryoku-rashin/internal/app/env"
	"ryoku-rashin/internal/app/look"
	"ryoku-rashin/internal/app/pages/teamsui"
	appstate "ryoku-rashin/internal/app/state"
	"ryoku-rashin/internal/protocol"
)

// a small card: the most blocking few, the rest follow as these are taken
const stepsShown = 4

// what each next step does when its one action is taken
type stepKind uint8

const (
	stepMemberHarness stepKind = iota
	stepDefaultHarness
	stepCredential
	stepMail
	stepFirstTask
	stepRoutine
)

// one thing the team lacks to work, said in a line, with the one action that gives it
type nextStep struct {
	// key names the step for as long as it stands, so hiding it keeps it hidden
	key, line, action string
	kind              stepKind
	// the routine to turn on, or the task a credential request waits in
	subject protocol.Routine
	task    string
}

type stepRow struct {
	act  kit.Plaque
	hide kit.IconPlaque
	tip  look.Tip
}

// the Overview's card of next steps, built from the snapshot alone
type nextSteps struct {
	rows map[string]*stepRow
}

func (n *nextSteps) row(key string) *stepRow {
	if n.rows == nil {
		n.rows = make(map[string]*stepRow)
	}
	row := n.rows[key]
	if row == nil {
		row = new(stepRow)
		n.rows[key] = row
	}
	return row
}

// the card, or nil when the team lacks nothing so the column leaves it out
func (n *nextSteps) layout(gtx layout.Context, e *env.Env, snapshot appstate.Snapshot, team protocol.Team) layout.Widget {
	th := e.Theme
	steps := teamSteps(snapshot, team, e.HiddenSteps[team.ID])
	// rows of steps that went go with them
	for key := range n.rows {
		if !slices.ContainsFunc(steps, func(step nextStep) bool { return step.key == key }) {
			delete(n.rows, key)
		}
	}
	if len(steps) == 0 {
		return nil
	}
	rows := make([]layout.Widget, len(steps))
	for index, step := range steps {
		row := n.row(step.key)
		row.act.Kind, row.act.Small, row.act.Label = kit.PlaqueLine, true, step.action
		row.hide.Kind, row.hide.Small, row.hide.Icon = kit.PlaqueQuiet, true, look.Icon.Close
		row.hide.Description = i18n.T("Hide this step")
		if row.act.Clicked(gtx) {
			takeStep(e, team, step)
		}
		if row.hide.Clicked(gtx) {
			e.HideStep(team.ID, step.key)
		}
		rows[index] = func(gtx layout.Context) layout.Dimensions {
			gtx.Constraints.Min.X = gtx.Constraints.Max.X
			return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
				layout.Flexed(1, look.Paragraph(th, th.T.Body, th.R.Fg, step.line)),
				layout.Rigid(layout.Spacer{Width: th.Sp.S3}.Layout),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions { return row.act.Layout(gtx, th.Theme) }),
				layout.Rigid(layout.Spacer{Width: th.Sp.S1}.Layout),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return row.tip.Layout(gtx, th, e.Overlays, row.hide.Description, func(gtx layout.Context) layout.Dimensions {
						return row.hide.Layout(gtx, th.Theme)
					})
				}),
			)
		}
	}
	return section(th, i18n.T("Next steps"), func(gtx layout.Context) layout.Dimensions {
		return kit.Slab(gtx, th.R.Stone1, th.Cut.Mid, func(gtx layout.Context) layout.Dimensions {
			gtx.Constraints.Min.X = gtx.Constraints.Max.X
			return layout.UniformInset(th.Sp.S3).Layout(gtx, stack(th, rows))
		})
	})
}

// what the team lacks, most blocking first: an engine that cannot start, a credential or a mail account
// its work waits on, a first task, then routines that are off; at most stepsShown, less what the user hid
func teamSteps(snapshot appstate.Snapshot, team protocol.Team, hidden []string) []nextStep {
	var steps []nextStep
	add := func(step nextStep) {
		if len(steps) < stepsShown && !slices.Contains(hidden, step.key) {
			steps = append(steps, step)
		}
	}
	// before the harness list arrives every engine would read as missing
	if len(snapshot.Harnesses) > 0 {
		defaultMissing := false
		for _, member := range team.Members {
			if !teamsui.MemberEngine(snapshot, member).Missing {
				continue
			}
			if member.Harness == "" {
				defaultMissing = true
				continue
			}
			add(nextStep{key: "harness:" + member.ID + ":" + member.Harness, kind: stepMemberHarness, line: i18n.Tf("%s's harness is not installed.", member.Name), action: i18n.T("Change it")})
		}
		if defaultMissing {
			add(nextStep{key: "harness:default", kind: stepDefaultHarness, line: i18n.T("The harness its members run on by default is not installed."), action: i18n.T("Open Settings")})
		}
	}
	for _, approval := range snapshot.Approvals {
		if approval.Team == team.ID && approval.Kind == protocol.ApprovalRequest && approval.Status == protocol.ApprovalPending {
			add(nextStep{key: "request:" + approval.ID, kind: stepCredential, line: approval.Title, action: i18n.T("Add it"), task: approval.Task})
		}
	}
	if team.Tools.Mail && snapshot.Vault.Mail == nil {
		add(nextStep{key: "mail", kind: stepMail, line: i18n.T("Mail is on but no mail account is set."), action: i18n.T("Add one")})
	}
	if neverRan(snapshot, team) {
		add(nextStep{key: "first", kind: stepFirstTask, line: i18n.T("This team has never run."), action: i18n.T("Give it a task")})
	}
	for _, routine := range snapshot.Routines {
		if routine.Team == team.ID && !routine.Enabled {
			add(nextStep{key: "routine:" + routine.ID, kind: stepRoutine, line: i18n.Tf("%s is off.", routine.Title), action: i18n.T("Turn it on"), subject: routine})
		}
	}
	return steps
}

// the team keeps no task, none is loaded here, and it never reported
func neverRan(snapshot appstate.Snapshot, team protocol.Team) bool {
	if team.Stats.Tasks > 0 || team.Stats.LastReport != nil {
		return false
	}
	for _, task := range snapshot.TeamTasks {
		if task.Team == team.ID {
			return false
		}
	}
	return true
}

func takeStep(e *env.Env, team protocol.Team, step nextStep) {
	switch step.kind {
	case stepMemberHarness:
		openTeam(e, team.ID, env.TeamSettings)
	case stepDefaultHarness:
		e.OpenSettings("teams")
	case stepCredential:
		// the value is typed into the request's own card, on its task or in the Inbox
		if step.task != "" {
			teamsui.OpenTask(e, step.task)
			return
		}
		e.NavigateTeams(env.TeamsPlace{Tab: protocol.TabInbox})
	case stepMail:
		e.NavigateTeams(env.TeamsPlace{Tab: protocol.TabVault})
	case stepFirstTask:
		e.NewTeamTask(team.ID)
	case stepRoutine:
		teamsui.SetRoutineEnabled(e, e.Snapshot(), step.subject, true, e.Report)
	}
}
