package settings

import (
	"maps"
	"slices"
	"time"

	"gioui.org/layout"

	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/kit"
	"ryoku-i18n"

	"ryoku-rashin/internal/app/env"
	"ryoku-rashin/internal/app/look"
	"ryoku-rashin/internal/app/pages/teamsui"
	"ryoku-rashin/internal/app/state"
	"ryoku-rashin/internal/protocol"
)

// what every team shares, each team's own settings live on its page; toggles and selects save as they change, numbers and text wait for their group's save
type teamsPanel struct {
	// the stored values the held fields were last filled from, so an edit stays until it is saved
	filled   bool
	running  teamsRunning
	defaults protocol.TeamDefaults
	asking   teamsAsking

	paused      kit.Toggle
	wake        kit.Toggle
	atOnce      kit.Stepper
	perTeam     kit.Stepper
	saveRunning kit.Plaque

	engine       teamsui.EngineForm
	approvals    look.Select
	notify       look.Select
	limit        teamsui.TokenStepper
	warn         kit.Stepper
	turns        kit.Stepper
	minutes      kit.Stepper
	saveDefaults kit.Plaque

	hours      kit.Stepper
	quiet      kit.Toggle
	from, to   kit.TextField
	saveAsking kit.Plaque

	hidden  teamsui.PathList
	signIns map[string]*teamsui.SignInActions
	memory  teamsui.TeamMemoryForm

	keep look.Select
}

type teamsRunning struct{ maxRunning, perTeam int }

type teamsAsking struct {
	hours int
	quiet protocol.QuietHours
	on    bool
}

// where quiet hours start out before anyone set them
const (
	quietFrom = "22:00"
	quietTo   = "07:00"
)

func (p *Page) layoutTeams(gtx layout.Context, e *env.Env) layout.Dimensions {
	th := e.Theme
	snapshot := e.Snapshot()
	t := &p.teams
	t.fill(snapshot.Settings.Teams)
	groups := []layout.Widget{
		t.runningGroup(e, snapshot.Settings.Teams),
		t.defaultsGroup(e, snapshot),
		t.askingGroup(e, snapshot.Settings.Teams),
		t.signInGroup(e, snapshot, gtx.Now),
		t.sandboxGroup(e, snapshot),
		t.historyGroup(e, snapshot.Settings.Teams),
	}
	children := []layout.FlexChild{
		layout.Rigid(kit.Text(th.Theme, th.T.Section, th.R.Fg, i18n.T("Teams"))),
		layout.Rigid(layout.Spacer{Height: th.Sp.S3}.Layout),
	}
	for index, group := range groups {
		if index > 0 {
			children = append(children, layout.Rigid(layout.Spacer{Height: th.Sp.S5}.Layout))
		}
		children = append(children, layout.Rigid(group))
	}
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
}

// a held field is filled again only when what is stored moved, by a save here or from elsewhere
func (t *teamsPanel) fill(settings protocol.TeamsSettings) {
	if running := (teamsRunning{settings.MaxRunning, settings.PerTeam}); !t.filled || running != t.running {
		t.running = running
		t.atOnce.Value, t.perTeam.Value = running.maxRunning, running.perTeam
	}
	stored := settings.Defaults
	if !t.filled || !sameEngine(stored, t.defaults) || stored.Budget != t.defaults.Budget {
		t.defaults = stored
		t.defaults.Options = maps.Clone(stored.Options)
		t.engine.Set(stored.Harness, stored.Model, maps.Clone(stored.Options))
		budget := stored.Budget
		t.limit.Value, t.warn.Value, t.turns.Value, t.minutes.Value = budget.MonthTokens, budget.WarnPercent, budget.TaskTurns, budget.TaskMinutes
	}
	asking := teamsAsking{hours: settings.ApprovalHours, on: settings.QuietHours != nil}
	if asking.on {
		asking.quiet = *settings.QuietHours
	}
	if !t.filled || asking != t.asking {
		t.asking = asking
		t.hours.Value = asking.hours
		switch {
		case asking.on:
			t.from.SetText(asking.quiet.From)
			t.to.SetText(asking.quiet.To)
		case t.from.Text() == "" && t.to.Text() == "":
			t.from.SetText(quietFrom)
			t.to.SetText(quietTo)
		}
	}
	t.filled = true
}

func sameEngine(a, b protocol.TeamDefaults) bool {
	return a.Harness == b.Harness && a.Model == b.Model && maps.Equal(a.Options, b.Options)
}

// a small heading, what the group is about, its slab, and the save for what it holds when it holds anything to save
func teamsGroup(th *look.Theme, title, caption string, rows []look.Setting, save layout.Widget) layout.Widget {
	return func(gtx layout.Context) layout.Dimensions {
		children := []layout.FlexChild{layout.Rigid(kit.Text(th.Theme, th.T.Title, th.R.Fg, title))}
		if caption != "" {
			children = append(children,
				layout.Rigid(layout.Spacer{Height: th.Sp.S1}.Layout),
				layout.Rigid(look.Paragraph(th, th.T.Detail, th.R.Fg2, caption)),
			)
		}
		children = append(children,
			layout.Rigid(layout.Spacer{Height: th.Sp.S2}.Layout),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions { return look.SettingsSlab(gtx, th, rows) }),
		)
		if save != nil {
			children = append(children, layout.Rigid(layout.Spacer{Height: th.Sp.S3}.Layout), layout.Rigid(save))
		}
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
	}
}

// the save under a group, dimmed while nothing in it changed
func groupSave(th *look.Theme, plaque *kit.Plaque, changed bool) layout.Widget {
	plaque.Kind, plaque.Small, plaque.Label = kit.PlaqueTonal, true, i18n.T("Save")
	return func(gtx layout.Context) layout.Dimensions {
		if !changed {
			gtx = gtx.Disabled()
		}
		return plaque.Layout(gtx, th.Theme)
	}
}

func (t *teamsPanel) runningGroup(e *env.Env, settings protocol.TeamsSettings) layout.Widget {
	th := e.Theme
	t.paused.Value, t.wake.Value = settings.Paused, settings.RoutinesWake
	t.atOnce.Min, t.atOnce.Max, t.atOnce.Step = 1, protocol.TeamsMaxRunningMax, 1
	t.perTeam.Min, t.perTeam.Max, t.perTeam.Step = 1, min(protocol.TeamsPerTeamMax, t.atOnce.Value), 1
	t.perTeam.Value = min(t.perTeam.Value, t.perTeam.Max)
	edited := teamsRunning{t.atOnce.Value, t.perTeam.Value}
	changed := edited != t.running
	save := func(gtx layout.Context) layout.Dimensions {
		if t.saveRunning.Clicked(gtx) && changed {
			t.saveRunning.Busy = true
			e.SetSettings(map[string]any{"teams": map[string]any{"maxRunning": edited.maxRunning, "perTeam": edited.perTeam}}, func(err error) {
				t.saveRunning.Busy = false
				if err != nil {
					e.Notify(err.Error())
				}
			})
		}
		return groupSave(th, &t.saveRunning, changed)(gtx)
	}
	rows := []look.Setting{
		{
			Title:  i18n.T("Pause every team"),
			Detail: i18n.T("Nothing starts and no routine fires until you turn this off. Working tasks stop."),
			Control: func(gtx layout.Context) layout.Dimensions {
				return namedSwitch(gtx, th, i18n.T("Pause every team"), &t.paused, func(on bool) {
					set(e, "teams", map[string]any{"paused": on})
				})
			},
		},
		{
			Title:   i18n.T("Tasks at once"),
			Detail:  i18n.T("How many team tasks work at the same time across all teams."),
			Control: func(gtx layout.Context) layout.Dimensions { return t.atOnce.Layout(gtx, th.Theme) },
		},
		{
			Title:   i18n.T("Tasks at once in one team"),
			Detail:  i18n.T("A team can override this in its own settings."),
			Control: func(gtx layout.Context) layout.Dimensions { return t.perTeam.Layout(gtx, th.Theme) },
		},
		{
			Title:  i18n.T("Wake Rashin for routines"),
			Detail: i18n.T("A timer starts Rashin at login and every 30 minutes while a routine is on. Off, routines run only while Rashin is up."),
			Control: func(gtx layout.Context) layout.Dimensions {
				return namedSwitch(gtx, th, i18n.T("Wake Rashin for routines"), &t.wake, func(on bool) {
					set(e, "teams", map[string]any{"routinesWake": on})
				})
			},
		},
	}
	return teamsGroup(th, i18n.T("Running"), "", rows, save)
}

func (t *teamsPanel) defaultsGroup(e *env.Env, snapshot state.Snapshot) layout.Widget {
	th := e.Theme
	stored := snapshot.Settings.Teams.Defaults
	t.engine.Agents = true
	t.warn.Min, t.warn.Max, t.warn.Step, t.warn.Unit = 1, 99, 1, i18n.T("percent")
	t.turns.Min, t.turns.Max, t.turns.Step, t.turns.Unit = 1, protocol.TeamTaskTurnsMax, 1, i18n.T("turns")
	t.minutes.Min, t.minutes.Max, t.minutes.Step, t.minutes.Unit = 0, protocol.TeamTaskMinutesMax, 15, i18n.T("min")
	harness, model, options := t.engine.Value()
	budget := protocol.TeamBudget{MonthTokens: t.limit.Value, WarnPercent: t.warn.Value, TaskTurns: t.turns.Value, TaskMinutes: t.minutes.Value}
	edited := protocol.TeamDefaults{Harness: harness, Model: model, Options: options, Budget: budget}
	changed := !sameEngine(edited, t.defaults) || budget != t.defaults.Budget
	save := func(gtx layout.Context) layout.Dimensions {
		if t.saveDefaults.Clicked(gtx) && changed {
			// the patch merges options key by key, so one no longer picked is sent as null to go
			patched := make(map[string]any, len(t.defaults.Options)+len(options))
			for id := range t.defaults.Options {
				patched[id] = nil
			}
			for id, value := range options {
				patched[id] = value
			}
			t.saveDefaults.Busy = true
			defaults := map[string]any{"harness": harness, "model": model, "options": patched, "budget": budget}
			e.SetSettings(map[string]any{"teams": map[string]any{"defaults": defaults}}, func(err error) {
				t.saveDefaults.Busy = false
				if err != nil {
					e.Notify(err.Error())
				}
			})
		}
		return groupSave(th, &t.saveDefaults, changed)(gtx)
	}
	agents := i18n.Tf("Same as Agents (%s)", teamsui.AgentsHarnessName(snapshot))
	rows := []look.Setting{
		{
			Title:   i18n.T("Harness and model"),
			Detail:  i18n.T("What a member left on the teams default runs, in every team. A member with a harness of its own keeps it."),
			Below:   true,
			Control: func(gtx layout.Context) layout.Dimensions { return t.engine.Layout(gtx, e, agents) },
		},
		{
			Title:  i18n.T("Permissions"),
			Detail: i18n.T("What a new team may do before it asks you."),
			Control: func(gtx layout.Context) layout.Dimensions {
				dimensions := teamsui.ApprovalsSelect(gtx, e, &t.approvals, stored.Approvals)
				if index, changed := t.approvals.Changed(); changed && index < len(teamsui.ApprovalValues) {
					set(e, "teams", map[string]any{"defaults": map[string]any{"approvals": teamsui.ApprovalValues[index]}})
				}
				return dimensions
			},
		},
		{
			Title:   i18n.T("Tokens a month"),
			Detail:  i18n.T("At the limit the team pauses and asks you to raise it."),
			Control: func(gtx layout.Context) layout.Dimensions { return t.limit.Layout(gtx, th) },
		},
		{
			Title:   i18n.T("Warn at"),
			Detail:  i18n.T("You are told once a month when the team gets this close."),
			Control: func(gtx layout.Context) layout.Dimensions { return t.warn.Layout(gtx, th.Theme) },
		},
		{
			Title:   i18n.T("Turns per task"),
			Detail:  i18n.T("A task that uses them all stops and says so."),
			Control: func(gtx layout.Context) layout.Dimensions { return t.turns.Layout(gtx, th.Theme) },
		},
		{
			Title:   i18n.T("Minutes per task"),
			Detail:  i18n.T("A task still working then stops and says so. 0 lets it work as long as it needs."),
			Control: func(gtx layout.Context) layout.Dimensions { return t.minutes.Layout(gtx, th.Theme) },
		},
		{
			Title:  i18n.T("Notifications"),
			Detail: i18n.T("What a new team tells you on the desktop."),
			Control: func(gtx layout.Context) layout.Dimensions {
				dimensions := teamsui.NotifySelect(gtx, e, &t.notify, stored.Notify)
				if index, changed := t.notify.Changed(); changed && index < len(teamsui.NotifyValues) {
					set(e, "teams", map[string]any{"defaults": map[string]any{"notify": teamsui.NotifyValues[index]}})
				}
				return dimensions
			},
		},
	}
	rows = append(rows, t.memory.Rows(e, stored.Memory, func(memory protocol.TeamMemory, done func(error)) {
		e.SetSettings(map[string]any{"teams": map[string]any{"defaults": map[string]any{"memory": memory}}}, func(err error) {
			if err != nil {
				e.Notify(err.Error())
			}
			done(err)
		})
	})...)
	caption := i18n.T("A new team copies these. Changing them later changes no team you already have.")
	return teamsGroup(th, i18n.T("New teams start with"), caption, rows, save)
}

func (t *teamsPanel) askingGroup(e *env.Env, settings protocol.TeamsSettings) layout.Widget {
	th := e.Theme
	t.quiet.Value = settings.QuietHours != nil
	t.hours.Min, t.hours.Max, t.hours.Step, t.hours.Unit = 1, protocol.TeamsApprovalHoursMax, 1, i18n.T("hours")
	for _, field := range []*kit.TextField{&t.from, &t.to} {
		field.Editor.SingleLine, field.Editor.MaxLen = true, len("15:04")
	}
	t.from.Placeholder, t.to.Placeholder = quietFrom, quietTo
	edited := teamsAsking{hours: t.hours.Value, on: t.asking.on, quiet: protocol.QuietHours{From: t.from.Text(), To: t.to.Text()}}
	if !edited.on {
		edited.quiet = protocol.QuietHours{}
	}
	changed := edited != t.asking
	save := func(gtx layout.Context) layout.Dimensions {
		if t.saveAsking.Clicked(gtx) && changed {
			patch := map[string]any{"approvalHours": edited.hours}
			if edited.on {
				patch["quietHours"] = edited.quiet
			}
			t.saveAsking.Busy = true
			e.SetSettings(map[string]any{"teams": patch}, func(err error) {
				t.saveAsking.Busy = false
				if err != nil {
					e.Notify(err.Error())
				}
			})
		}
		return groupSave(th, &t.saveAsking, changed)(gtx)
	}
	rows := []look.Setting{
		{
			Title:   i18n.T("An unanswered question counts as no after"),
			Detail:  i18n.T("The agent hears no and goes on without it. A team stopped at its spending limit stays paused until you answer."),
			Control: func(gtx layout.Context) layout.Dimensions { return t.hours.Layout(gtx, th.Theme) },
		},
		{
			Title:  i18n.T("Quiet hours"),
			Detail: i18n.T("No desktop notification between these times. What needs you still waits in the Inbox."),
			Control: func(gtx layout.Context) layout.Dimensions {
				return namedSwitch(gtx, th, i18n.T("Quiet hours"), &t.quiet, func(on bool) {
					if !on {
						set(e, "teams", map[string]any{"quietHours": nil})
						return
					}
					set(e, "teams", map[string]any{"quietHours": quietWindow(t.from.Text(), t.to.Text())})
				})
			},
		},
	}
	if t.asking.on {
		rows = append(rows, look.Setting{
			Title:   i18n.T("From and to"),
			Detail:  i18n.T("Your computer's time, on a 24-hour clock. A window may cross midnight."),
			Control: func(gtx layout.Context) layout.Dimensions { return t.layoutQuietTimes(gtx, th) },
		})
	}
	return teamsGroup(th, i18n.T("Asking and telling"), "", rows, save)
}

// the times typed so far when they read, else the ones quiet hours start out with
func quietWindow(from, to string) protocol.QuietHours {
	start, okFrom := protocol.ParseClock(from)
	end, okTo := protocol.ParseClock(to)
	if !okFrom || !okTo || start == end {
		return protocol.QuietHours{From: quietFrom, To: quietTo}
	}
	return protocol.QuietHours{From: from, To: to}
}

func (t *teamsPanel) layoutQuietTimes(gtx layout.Context, th *look.Theme) layout.Dimensions {
	field := func(text *kit.TextField, name string) layout.FlexChild {
		return layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			gtx.Constraints.Min.X, gtx.Constraints.Max.X = gtx.Dp(84), gtx.Dp(84)
			return look.Named(gtx, name, func(gtx layout.Context) layout.Dimensions { return text.Layout(gtx, th.Theme) })
		})
	}
	return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
		field(&t.from, i18n.T("Quiet from")),
		layout.Rigid(layout.Spacer{Width: th.Sp.S2}.Layout),
		layout.Rigid(kit.Text(th.Theme, th.T.Caption, th.R.Fg3, i18n.T("to"))),
		layout.Rigid(layout.Spacer{Width: th.Sp.S2}.Layout),
		field(&t.to, i18n.T("Quiet until")),
	)
}

func (t *teamsPanel) sandboxGroup(e *env.Env, snapshot state.Snapshot) layout.Widget {
	th := e.Theme
	harnessState := harnessStatePaths(snapshot.Harnesses)
	always := append(protocol.TeamsAlwaysHiddenPaths(), harnessState...)
	// an old saved list repeats what is now always hidden; it is dropped from what shows and what saves
	hidden := slices.DeleteFunc(slices.Clone(snapshot.Settings.Teams.Hidden), func(path string) bool { return slices.Contains(always, path) })
	t.hidden.Title, t.hidden.Placeholder = i18n.T("Choose a folder to hide"), "~/Private"
	rows := []look.Setting{
		{
			Title:   i18n.T("Always hidden"),
			Detail:  i18n.T("Where keys, sign-ins and private state live. Every team agent and every Check sees these as empty, whatever the list below says."),
			Below:   true,
			Control: func(gtx layout.Context) layout.Dimensions { return layoutAlwaysHidden(gtx, th, harnessState) },
		},
		{
			Title:  i18n.T("Also hidden from every team"),
			Detail: i18n.T("Your own folders and files agents see as empty, beside what is always hidden."),
			Below:  true,
			Control: func(gtx layout.Context) layout.Dimensions {
				dimensions := t.hidden.Layout(gtx, e, hidden)
				if list, changed := t.hidden.Changed(); changed {
					// the list is only the user's own, so an empty one is written as empty
					if list == nil {
						list = []string{}
					}
					set(e, "teams", map[string]any{"hidden": list})
				}
				return dimensions
			},
		},
		{
			Title:   i18n.T("Network"),
			Detail:  i18n.T("Harnesses reach their model online, so an agent reaches what you can."),
			Control: kit.Text(th.Theme, th.T.Label, th.R.Fg2, i18n.T("Shared")),
		},
	}
	return teamsGroup(th, i18n.T("Sandbox"), "", rows, nil)
}

// every harness's own state, which every team sandbox hides as well, each path once
func harnessStatePaths(harnesses []protocol.HarnessInfo) []string {
	var paths []string
	for _, harness := range harnesses {
		for _, path := range harness.State {
			if !slices.Contains(paths, path) {
				paths = append(paths, path)
			}
		}
	}
	return paths
}

// one line per kind of secret with the paths that hold it, then every harness's own state; none of it can be taken off
func layoutAlwaysHidden(gtx layout.Context, th *look.Theme, harnessState []string) layout.Dimensions {
	var children []layout.FlexChild
	line := func(label string, paths []string) {
		if len(children) > 0 {
			children = append(children, layout.Rigid(layout.Spacer{Height: th.Sp.S2}.Layout))
		}
		children = append(children,
			layout.Rigid(look.Paragraph(th, th.T.Label, th.R.Fg, label)),
			layout.Rigid(layout.Spacer{Height: th.Sp.S1}.Layout),
			// a path never breaks inside itself, the line breaks between them
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return look.Flow{Gap: 13, LineGap: 2}.Layout(gtx, len(paths), func(gtx layout.Context, index int) layout.Dimensions {
					return kit.RawText(th.Theme, th.T.MonoSm, th.R.Fg2, paths[index])(gtx)
				})
			}),
		)
	}
	for _, group := range protocol.TeamsAlwaysHidden() {
		line(hiddenKindName(group.Kind), group.Paths)
	}
	line(i18n.T("Every harness's own state"), harnessState)
	gtx.Constraints.Min.X = gtx.Constraints.Max.X
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
}

func hiddenKindName(kind string) string {
	switch kind {
	case protocol.HiddenKeys:
		return i18n.T("Keys and password stores")
	case protocol.HiddenGit:
		return i18n.T("Git and code hosts")
	case protocol.HiddenCloud:
		return i18n.T("Cloud CLIs")
	case protocol.HiddenPackages:
		return i18n.T("Package registry tokens")
	case protocol.HiddenBrowsers:
		return i18n.T("Browsers")
	case protocol.HiddenMail:
		return i18n.T("Mail")
	case protocol.HiddenChat:
		return i18n.T("Chat apps")
	case protocol.HiddenHistories:
		return i18n.T("Shell histories")
	case protocol.HiddenRyoku:
		return i18n.T("Rashin and Ryoku private state")
	}
	return kind
}

// each team runs each harness in a home of its own, signed in apart from the other teams and from the user's own setup
func (t *teamsPanel) signInGroup(e *env.Env, snapshot state.Snapshot, now time.Time) layout.Widget {
	th := e.Theme
	harnesses := teamsui.TeamsHarnesses(snapshot)
	if t.signIns == nil {
		t.signIns = make(map[string]*teamsui.SignInActions)
	}
	rows := make([]look.Setting, 0, max(1, len(harnesses)))
	for _, harness := range harnesses {
		var waiting []protocol.Team
		signed := 0
		for _, team := range teamsui.HarnessTeams(snapshot, harness.ID) {
			if teamsui.SignedIn(harness, team.ID) {
				signed++
				continue
			}
			waiting = append(waiting, team)
		}
		row := look.Setting{Title: harness.Name, Detail: signInSummary(signed, len(waiting))}
		// only the teams that still wait are listed, a signed in team signs in again from its own settings
		if len(waiting) > 0 {
			row.Below = true
			row.Control = func(gtx layout.Context) layout.Dimensions {
				children := make([]layout.FlexChild, 0, 2*len(waiting))
				for index, team := range waiting {
					actions := t.signIns[harness.ID+"\x00"+team.ID]
					if actions == nil {
						actions = new(teamsui.SignInActions)
						t.signIns[harness.ID+"\x00"+team.ID] = actions
					}
					if index > 0 {
						children = append(children, layout.Rigid(layout.Spacer{Height: th.Sp.S3}.Layout))
					}
					children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return actions.Row(gtx, e, harness, team, team.Name, now)
					}))
				}
				return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
			}
		}
		rows = append(rows, row)
	}
	if len(harnesses) == 0 {
		rows = append(rows, look.Setting{Title: i18n.T("No team runs on an installed harness yet.")})
	}
	caption := i18n.T("Each team runs each harness in a home of its own, signed in apart from the other teams and from you. Your own harness folders stay hidden from them.")
	return teamsGroup(th, i18n.T("Sign-in for Teams"), caption, rows, nil)
}

// "3 teams signed in, 1 not"
func signInSummary(signed, waiting int) string {
	in := i18n.Tf("%d teams signed in", signed)
	if signed == 1 {
		in = i18n.T("1 team signed in")
	}
	switch {
	case waiting == 0:
		return in
	case signed > 0:
		return i18n.Tf("%s, %d not", in, waiting)
	case waiting == 1:
		return i18n.T("1 team not signed in")
	}
	return i18n.Tf("%d teams not signed in", waiting)
}

func (t *teamsPanel) historyGroup(e *env.Env, settings protocol.TeamsSettings) layout.Widget {
	th := e.Theme
	labels := []string{i18n.T("Forever"), i18n.T("30 days"), i18n.T("90 days"), i18n.T("1 year")}
	selected := max(0, slices.Index(protocol.TeamsKeepDays, settings.KeepDays))
	rows := []look.Setting{
		{
			Title:  i18n.T("Keep finished tasks"),
			Detail: i18n.T("Older finished tasks go with their conversations. What they made stays in the team's folder."),
			Control: func(gtx layout.Context) layout.Dimensions {
				gtx.Constraints.Min.X = gtx.Dp(180)
				dimensions := t.keep.Layout(gtx, th, e.Overlays, labels, selected)
				if index, changed := t.keep.Changed(); changed && index < len(protocol.TeamsKeepDays) {
					set(e, "teams", map[string]any{"keepDays": protocol.TeamsKeepDays[index]})
				}
				return dimensions
			},
		},
		{Title: i18n.T("The activity ledger keeps the newest 5,000 entries.")},
	}
	return teamsGroup(th, i18n.T("History"), "", rows, nil)
}
