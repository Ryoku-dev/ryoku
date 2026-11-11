package teams

import (
	"image"
	"path/filepath"
	"regexp"
	"strings"

	"gioui.org/layout"

	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/kit"
	"ryoku-i18n"

	"ryoku-rashin/internal/app/env"
	"ryoku-rashin/internal/app/look"
	"ryoku-rashin/internal/app/pages/teamsui"
	"ryoku-rashin/internal/paths"
	"ryoku-rashin/internal/protocol"
)

// the template a draft opened without one starts from
const blankTemplate = "blank"

var slugPart = regexp.MustCompile(`[^a-z0-9]+`)

// the folder name the daemon would make of a team's name
func slug(name string) string {
	value := strings.Trim(slugPart.ReplaceAllString(strings.ToLower(strings.TrimSpace(name)), "-"), "-")
	if value == "" {
		return "team"
	}
	return value
}

// a team before it exists: started from a template, every field changeable, previewed in the aside
type draft struct {
	// wanted is the template Select asked for; it fills the form once the templates are here
	wanted   string
	template string
	filled   bool
	busy     bool
	problem  string

	scroll         look.Scroller
	cancel, create kit.Plaque
	starts         templateRows

	name    kit.TextField
	purpose look.MultiLine
	folder  env.PathField
	members []*draftMember
	add     kit.Plaque

	screen, mail   kit.Toggle
	approvals      look.Select
	approvalsValue string
	limit          teamsui.TokenStepper
	// warn is not asked in the draft; it comes from the teams defaults
	warn      int
	more      kit.Disclosure
	moreClick kit.Click
	turns     kit.Stepper
	minutes   kit.Stepper
	notify    look.Select
	// notifyValue is what reaches the desktop, from the teams defaults until changed
	notifyValue string
	routines    []protocol.RoutineDraft
	routineOn   []kit.Toggle
	sentences   map[string]string
}

type draftMember struct {
	form   memberForm
	remove kit.IconPlaque
	tip    look.Tip
	// the engine folds under one line, since most members keep the teams default
	engine      kit.Disclosure
	engineClick kit.Click
}

// a draft already filled from the asked template keeps what was typed into it
func (d *draft) open(template string) {
	if template == "" {
		if d.filled {
			return
		}
		template = blankTemplate
	}
	if d.filled && template == d.template {
		return
	}
	d.wanted, d.filled = template, false
}

func (d *draft) apply(e *env.Env, template protocol.TeamTemplate) {
	d.template, d.filled, d.problem = template.ID, true, ""
	d.name.SetText(template.Name)
	if template.ID == blankTemplate {
		d.name.SetText("")
	}
	d.purpose.SetText(template.Purpose)
	d.folder.SetText("")
	d.members = d.members[:0]
	for _, member := range template.Members {
		row := new(draftMember)
		row.form.set(member)
		d.members = append(d.members, row)
	}
	if len(d.members) == 0 {
		d.members = append(d.members, new(draftMember))
	}
	d.markLead()
	d.screen.Value, d.mail.Value = template.Tools.Screen, template.Tools.Mail
	// what the template leaves open comes from Settings, Teams, so the draft shows what the team will get
	defaults := e.Snapshot().Settings.Teams.Defaults
	d.approvalsValue = template.Approvals
	if !protocol.KnownApprovals(d.approvalsValue) {
		d.approvalsValue = defaults.Approvals
	}
	d.notifyValue = defaults.Notify
	budget := template.Budget
	d.limit.Value = budget.MonthTokens
	if budget.MonthTokens <= 0 {
		d.limit.Value = defaults.Budget.MonthTokens
	}
	d.warn = budget.WarnPercent
	if budget.WarnPercent <= 0 {
		d.warn = defaults.Budget.WarnPercent
	}
	d.turns.Value = budget.TaskTurns
	if budget.TaskTurns <= 0 {
		d.turns.Value = defaults.Budget.TaskTurns
	}
	d.minutes.Value = defaults.Budget.TaskMinutes
	d.routines = append([]protocol.RoutineDraft(nil), template.Routines...)
	d.routineOn = make([]kit.Toggle, len(d.routines))
	d.loadSentences(e)
}

// the first row is the lead, the others its members
func (d *draft) markLead() {
	for index, row := range d.members {
		row.form.lead.Value = index == 0
	}
}

func (d *draft) loadSentences(e *env.Env) {
	if d.sentences == nil {
		d.sentences = make(map[string]string)
	}
	for _, routine := range d.routines {
		schedule := routine.Schedule
		if _, ok := d.sentences[schedule]; ok {
			continue
		}
		var preview protocol.RoutinePreview
		e.Call(protocol.MTeamsRoutinePreview, protocol.RoutinePreviewParams{Schedule: schedule, Count: 1}, &preview, func(err error) {
			if err == nil && preview.Error == "" {
				d.sentences[schedule] = preview.Sentence
			}
		})
	}
}

// the folder the team gets: the one typed, else one named after it under ~/Teams
func (d *draft) folderPath() string {
	if folder := d.folder.Path(); folder != "" {
		return folder
	}
	return filepath.Join(paths.TeamsHome(), slug(d.name.Text()))
}

func (d *draft) crew() []protocol.Member {
	members := make([]protocol.Member, len(d.members))
	for index, row := range d.members {
		members[index] = row.form.member()
		members[index].Lead = index == 0
	}
	return members
}

// names tell members apart, so each needs one of its own
func (d *draft) markNames(members []protocol.Member) bool {
	valid := true
	seen := make(map[string]bool, len(members))
	for index, member := range members {
		field := &d.members[index].form.name
		field.Error = ""
		if member.Name != "" && seen[member.Name] {
			field.Error = i18n.T("Another member of the team has this name.")
		}
		seen[member.Name] = true
		valid = valid && member.Name != "" && field.Error == ""
	}
	return valid
}

func (d *draft) params() protocol.TeamCreateParams {
	routines := make([]protocol.RoutineDraft, len(d.routines))
	for index, routine := range d.routines {
		routine.Enabled = d.routineOn[index].Value
		routines[index] = routine
	}
	params := protocol.TeamCreateParams{Name: strings.TrimSpace(d.name.Text()), Purpose: strings.TrimSpace(d.purpose.Text())}
	params.Template, params.Folder, params.Members = d.template, d.folderPath(), d.crew()
	params.Tools = protocol.TeamTools{Screen: d.screen.Value, Mail: d.mail.Value}
	params.Approvals, params.Notify, params.Routines = d.approvalsValue, d.notifyValue, routines
	params.Budget = protocol.TeamBudget{MonthTokens: d.limit.Value, WarnPercent: d.warn, TaskTurns: d.turns.Value, TaskMinutes: d.minutes.Value}
	return params
}

func (d *draft) submit(e *env.Env) {
	d.busy, d.problem = true, ""
	var team protocol.Team
	e.Call(protocol.MTeamsCreate, d.params(), &team, func(err error) {
		d.busy = false
		if err != nil {
			d.problem = err.Error()
			return
		}
		// the next draft starts afresh
		d.filled, d.wanted = false, blankTemplate
		openTeam(e, team.ID, env.TeamOverview)
		e.Notify(i18n.Tf("%s is ready. Give it a task.", team.Name))
	})
}

func (d *draft) layout(gtx layout.Context, e *env.Env, p *Page) layout.Dimensions {
	th := e.Theme
	if templates := p.templates.List(e); !d.filled && len(templates) > 0 {
		template, ok := templateByID(templates, d.wanted)
		if !ok {
			template, ok = templateByID(templates, blankTemplate)
		}
		if !ok {
			template = templates[len(templates)-1]
		}
		d.apply(e, template)
	}
	members := d.crew()
	named := d.markNames(members)
	ready := d.filled && !d.busy && named && strings.TrimSpace(d.name.Text()) != ""
	for _, member := range members {
		ready = ready && protocol.ValidEngineModel(member.Model)
	}
	d.cancel.Kind, d.cancel.Small, d.cancel.Label = kit.PlaqueQuiet, true, i18n.T("Cancel")
	d.create.Kind, d.create.Small, d.create.Label = kit.PlaqueGold, true, i18n.T("Create team")
	d.create.Busy = d.busy
	if d.cancel.Clicked(gtx) {
		e.NavigateTeams(env.TeamsPlace{Tab: protocol.TabTeams})
	}
	if d.create.Clicked(gtx) && ready {
		d.submit(e)
	}
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions { return d.layoutHeader(gtx, th, ready) }),
		layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
			inset := look.ContentInset(e.Class)
			return d.scroll.Layout(gtx, th, 1, func(gtx layout.Context, _ int) layout.Dimensions {
				return layout.Inset{Top: th.Sp.S4, Bottom: th.Sp.S5, Left: inset, Right: inset}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					return kit.DraftColumns(gtx, th.Theme,
						func(gtx layout.Context) layout.Dimensions { return d.layoutForm(gtx, e, p) },
						func(gtx layout.Context) layout.Dimensions { return d.layoutAside(gtx, e, members) },
					)
				})
			})
		}),
	)
}

// the draft's header carries its one consequential action, so it holds a gold plaque the pane header has no room for
func (d *draft) layoutHeader(gtx layout.Context, th *look.Theme, ready bool) layout.Dimensions {
	size := image.Pt(gtx.Constraints.Max.X, min(gtx.Dp(look.PaneHeaderHeight), gtx.Constraints.Max.Y))
	row := gtx
	row.Constraints = layout.Constraints{Max: image.Pt(max(0, size.X-gtx.Dp(21)-gtx.Dp(13)), size.Y)}
	look.MiddleLeft(row, gtx.Dp(21), size.Y, func(gtx layout.Context) layout.Dimensions {
		gtx.Constraints.Min.X = gtx.Constraints.Max.X
		return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
			layout.Flexed(1, kit.Text(th.Theme, th.T.Title, th.R.Fg, i18n.T("New team"))),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions { return d.cancel.Layout(gtx, th.Theme) }),
			layout.Rigid(layout.Spacer{Width: th.Sp.S1}.Layout),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				if !ready {
					gtx = gtx.Disabled()
				}
				return d.create.Layout(gtx, th.Theme)
			}),
		)
	})
	if d.scroll.Position.First > 0 || d.scroll.Position.Offset > 0 {
		line := max(1, gtx.Dp(1))
		look.FillRect(gtx, image.Rect(0, size.Y-line, size.X, size.Y), th.R.Vein)
	}
	return layout.Dimensions{Size: size}
}
