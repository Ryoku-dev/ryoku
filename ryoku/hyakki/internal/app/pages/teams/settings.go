package teams

import (
	"image"
	"slices"
	"strings"
	"time"

	"gioui.org/layout"
	"gioui.org/op"

	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/kit"
	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/palette"
	"ryoku-i18n"

	"ryoku-rashin/internal/app/env"
	"ryoku-rashin/internal/app/look"
	"ryoku-rashin/internal/app/pages/teamsui"
	appstate "ryoku-rashin/internal/app/state"
	"ryoku-rashin/internal/paths"
	"ryoku-rashin/internal/protocol"
)

const (
	sectionIdentity = iota
	sectionCrew
	sectionAccess
	sectionPermissions
	sectionBudget
	sectionNotifications
	sectionMemory
	sectionSkills
	sectionCount
)

// how many of its tasks a team may run at once: 0 follows Settings, Teams
var runningValues = [...]int{0, 1, 2, 3, protocol.TeamsPerTeamMax}

// eight settings groups; export and removal stay at the foot, outside the index
type settingsTab struct {
	scroll look.Scroller
	// the team and its last change the fields were filled from
	filledFrom, filledTeam string
	name                   kit.TextField
	purpose                look.MultiLine
	folder                 env.PathField
	identityBase           identity
	identityBar            saveBar
	identityUpdated        time.Time

	members  map[string]*memberRow
	add      kit.Plaque
	every    kit.Plaque
	graph    reportingDraft
	plan     kit.Toggle
	planHeld teamsui.Held[bool]
	routines kit.Plaque

	screen, mail kit.Toggle
	tools        teamsui.Held[protocol.TeamTools]
	credentials  map[string]*credentialRow
	vault        kit.Plaque
	trusts       map[protocol.CredentialTrust]*kit.Plaque
	mailTrusts   map[protocol.MailTrust]*kit.Plaque
	writable     teamsui.PathList
	hidden       teamsui.PathList

	approvals     look.Select
	approvalsHeld teamsui.Held[string]

	limit         teamsui.TokenStepper
	warn          kit.Stepper
	turns         kit.Stepper
	minutes       kit.Stepper
	budgetBase    protocol.TeamBudget
	budgetBar     saveBar
	budgetUpdated time.Time

	running     look.Select
	runningHeld teamsui.Held[int]

	notify     look.Select
	notifyHeld teamsui.Held[string]

	memory teamsui.TeamMemoryForm
	skills skillsSection

	export, pause, clear, remove kit.Plaque
}

type memberRow struct {
	edit, drop kit.IconPlaque
}

type credentialRow struct {
	toggle kit.Toggle
	held   teamsui.Held[bool]
}

// the text fields of the Identity group, trimmed as they are saved
type identity struct{ name, purpose, folder string }

// the bar a group of text and number fields saves through, there only while they differ from the team;
// shown says it stands this frame, so a second dirty group's save stays tonal under the first one's gold
type saveBar struct {
	discard, save kit.Plaque
	shown         bool
}

func (s *settingsTab) jump(section int) {
	if section >= 0 && section < sectionCount {
		s.scroll.ScrollTo(section)
	}
}

func (s *settingsTab) identity() identity {
	return identity{name: strings.TrimSpace(s.name.Text()), purpose: strings.TrimSpace(s.purpose.Text()), folder: paths.Short(paths.Expand(s.folder.Path()))}
}

func (s *settingsTab) setIdentity(team protocol.Team) {
	s.name.SetText(team.Name)
	s.purpose.SetText(team.Purpose)
	// the folder reads as the header writes it, under ~; the daemon takes ~ as home when it saves
	s.folder.SetText(paths.Short(team.Folder))
	s.identityBase = s.identity()
	s.identityUpdated = team.Updated
}

func (s *settingsTab) budget() protocol.TeamBudget {
	return protocol.TeamBudget{MonthTokens: s.limit.Value, WarnPercent: s.warn.Value, TaskTurns: s.turns.Value, TaskMinutes: s.minutes.Value}
}

func (s *settingsTab) setBudget(team protocol.Team) {
	budget := team.Budget
	s.limit.Value, s.warn.Value, s.turns.Value, s.minutes.Value = budget.MonthTokens, budget.WarnPercent, budget.TaskTurns, budget.TaskMinutes
	s.budgetBase, s.budgetUpdated = s.budget(), team.Updated
}

// a group with unsaved edits keeps them when the team changes elsewhere; one without follows the team
func (s *settingsTab) fill(team protocol.Team) {
	key := team.ID + "\x00" + team.Updated.String()
	if s.filledFrom == key {
		return
	}
	s.filledFrom = key
	another := s.filledTeam != team.ID
	s.filledTeam = team.ID
	if another || s.identity() == s.identityBase || s.identity() == (identity{name: team.Name, purpose: team.Purpose, folder: paths.Short(team.Folder)}) {
		s.setIdentity(team)
	}
	if another || s.budget() == s.budgetBase || s.budget() == team.Budget {
		s.setBudget(team)
	}
}

func (s *settingsTab) layout(gtx layout.Context, e *env.Env, p *Page, team protocol.Team) layout.Dimensions {
	s.fill(team)
	groups := s.groups(gtx, e, p, team)
	side := gtx.Dp(look.ContentInset(e.Class))
	column := min(gtx.Dp(look.SettingsColumn), max(0, gtx.Constraints.Max.X-2*side))
	left := (gtx.Constraints.Max.X - column) / 2
	return s.layoutGroups(gtx, e, groups, left, column)
}

// the list takes the pane out to its edge so the wheel works beside the column and the thumb stands at the edge
func (s *settingsTab) layoutGroups(gtx layout.Context, e *env.Env, groups []layout.Widget, inset, width int) layout.Dimensions {
	th := e.Theme
	return s.scroll.Layout(gtx, th, len(groups), func(gtx layout.Context, index int) layout.Dimensions {
		top, bottom := th.Sp.S5, th.Sp.S1
		if index == len(groups)-1 {
			// room under the last group so it can still reach the top
			bottom = th.Sp.S6
		}
		full := gtx.Constraints.Max.X
		gtx.Constraints.Min.X, gtx.Constraints.Max.X = min(gtx.Constraints.Min.X, width), width
		defer op.Offset(image.Pt(inset, 0)).Push(gtx.Ops).Pop()
		dimensions := layout.Inset{Top: top, Bottom: bottom}.Layout(gtx, groups[index])
		return layout.Dimensions{Size: image.Pt(full, dimensions.Size.Y)}
	})
}

func (s *settingsTab) groups(gtx layout.Context, e *env.Env, p *Page, team protocol.Team) []layout.Widget {
	snapshot := e.Snapshot()
	return []layout.Widget{
		s.identitySection(gtx, e, p, team),
		s.crewSection(gtx, e, p, snapshot, team),
		s.accessSection(gtx, e, snapshot, team),
		s.permissionsSection(e, snapshot, team),
		s.budgetSection(gtx, e, snapshot, team),
		s.notifySection(e, snapshot, team),
		s.memorySection(e, team),
		s.skills.layout(e, snapshot, team),
		s.exportSection(gtx, e, team),
		func(gtx layout.Context) layout.Dimensions {
			return uiStack(e, look.SectionHead(e.Theme, look.Head{Title: i18n.T("Routines")}), quiet(e.Theme, i18n.T("Scheduled wakes use the same team grants, reporting graph and member heartbeat policy.")), action(e, &s.routines, i18n.T("Manage team routines"), true, func() { openTeam(e, team.ID, env.TeamRoutines) }))(gtx)
		},
		s.dangerSection(gtx, e, p, snapshot, team),
		func(gtx layout.Context) layout.Dimensions { return p.colophon(gtx, e) },
	}
}

// discard and save are taken before the bar is drawn; save stays off while the fields cannot be saved,
// and is the view's gold unless another group's bar already holds it
func (b *saveBar) setting(gtx layout.Context, th *look.Theme, ready, gold bool, discard func(), save func(done func(error))) look.Setting {
	b.discard.Kind, b.discard.Small, b.discard.Label = kit.PlaqueQuiet, true, i18n.T("Discard")
	b.save.Kind, b.save.Small, b.save.Label = kit.PlaqueTonal, true, i18n.T("Save")
	if gold {
		b.save.Kind = kit.PlaqueGold
	}
	if b.discard.Clicked(gtx) {
		discard()
	}
	if b.save.Clicked(gtx) && ready && !b.save.Busy {
		b.save.Busy = true
		save(func(error) { b.save.Busy = false })
	}
	return look.Setting{Title: i18n.T("Unsaved changes"), Control: func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
			layout.Rigid(func(gtx layout.Context) layout.Dimensions { return b.discard.Layout(gtx, th.Theme) }),
			layout.Rigid(layout.Spacer{Width: th.Sp.S1}.Layout),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				if !ready {
					gtx = gtx.Disabled()
				}
				return b.save.Layout(gtx, th.Theme)
			}),
		)
	}}
}

func (s *settingsTab) identitySection(gtx layout.Context, e *env.Env, p *Page, team protocol.Team) layout.Widget {
	th := e.Theme
	now := s.identity()
	if now.folder == team.Folder {
		now.folder = paths.Short(team.Folder)
	}
	saved := identity{name: team.Name, purpose: team.Purpose, folder: paths.Short(team.Folder)}
	s.purpose.Lines = 3
	s.folder.Title = i18n.T("The team's folder")
	rows := []look.Setting{
		{Title: i18n.T("Name"), Below: true, Changed: now.name != saved.name,
			Reset:   func() { s.name.SetText(team.Name) },
			Control: func(gtx layout.Context) layout.Dimensions { return s.name.Layout(gtx, th.Theme) }},
		{Title: i18n.T("Purpose"), Detail: i18n.T("Every member reads it with every task, so say what the team is for."),
			Below: true, Changed: now.purpose != saved.purpose, Reset: func() { s.purpose.SetText(team.Purpose) },
			Control: func(gtx layout.Context) layout.Dimensions { return s.purpose.Layout(gtx, th) }},
		// the field's own plaque chooses a folder; the header's folder link opens it
		{Title: i18n.T("Folder"), Below: true, Changed: now.folder != saved.folder,
			Reset:   func() { s.folder.SetText(saved.folder) },
			Detail:  i18n.T("Where the team works and keeps what it makes. Choosing another folder does not move the files."),
			Control: func(gtx layout.Context) layout.Dimensions { return s.folder.Layout(gtx, e) }},
	}
	s.identityBar.shown = now != saved
	if s.identityBar.shown {
		rows = append(rows, s.identityBar.setting(gtx, th, now.name != "" && now.folder != "", true,
			func() { s.setIdentity(team) },
			func(done func(error)) {
				patch := map[string]any{}
				if now.name != saved.name {
					patch["name"] = now.name
				}
				if now.purpose != saved.purpose {
					patch["purpose"] = now.purpose
				}
				if now.folder != saved.folder {
					patch["folder"] = now.folder
				}
				updateTeam(e, team.ID, patch, done, s.identityUpdated)
			}))
	}
	made := i18n.Tf("Made %s", team.Created.Local().Format("2 Jan 2006"))
	if template, ok := templateByID(p.templates.List(e), team.Template); ok {
		rows = append(rows, look.Setting{Title: i18n.Tf("Made from %s", template.Name), Detail: made})
	} else {
		rows = append(rows, look.Setting{Title: made})
	}
	changed := 0
	if s.identityBar.shown {
		changed++
	}
	if s.budget() != team.Budget {
		changed++
	}
	return func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return look.SpecStrip(gtx, th, []look.Spec{
					{Label: i18n.T("Members"), Value: kit.Pad2(len(team.Members))},
					{Label: i18n.T("Settings"), Value: kit.Pad2(sectionCount)},
					{Label: i18n.T("Changed"), Value: kit.Pad2(changed), Lit: changed > 0},
				})
			}),
			layout.Rigid(layout.Spacer{Height: th.Sp.S5}.Layout),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return look.SettingsSection(gtx, th, look.Head{Title: i18n.T("Identity"), Number: 1, Of: sectionCount}, rows)
			}),
		)
	}
}

func (s *settingsTab) memberRow(id string) *memberRow {
	if s.members == nil {
		s.members = make(map[string]*memberRow)
	}
	row := s.members[id]
	if row == nil {
		row = new(memberRow)
		s.members[id] = row
	}
	return row
}

func (s *settingsTab) crewSection(gtx layout.Context, e *env.Env, p *Page, snapshot appstate.Snapshot, team protocol.Team) layout.Widget {
	th := e.Theme
	rows := make([]look.Setting, 0, len(team.Members)+2)
	rows = append(rows, look.Setting{Title: i18n.T("Structure and reporting"), Below: true, Control: s.graph.layout(e, team)})
	crewReady := !s.graph.dirty() && !s.graph.busy
	if !crewReady {
		rows = append(rows, look.Setting{Title: i18n.T("Reporting draft open"), Detail: i18n.T("Save or deliberately discard the reporting draft before editing, adding or removing spirits. The chart and reporting pickers share that one draft.")})
	}
	for _, member := range team.Members {
		row := s.memberRow(member.ID)
		row.edit.Kind, row.edit.Small, row.edit.Icon = kit.PlaqueQuiet, true, look.Icon.Rename
		row.edit.Description = i18n.Tf("Edit %s", member.Name)
		row.drop.Kind, row.drop.Small, row.drop.Icon = kit.PlaqueQuiet, true, look.Icon.Delete
		row.drop.Description = i18n.Tf("Remove %s", member.Name)
		if row.edit.Clicked(gtx) && crewReady {
			p.editMember(team, member)
		}
		if row.drop.Clicked(gtx) && crewReady && len(team.Members) > 1 {
			p.removeMember(team, member)
		}
		engine := teamsui.MemberEngine(snapshot, member)
		detail := engine.Line()
		if engine.Missing {
			detail = i18n.Tf("%s cannot run now", engine.Short())
		}
		rows = append(rows, look.Setting{
			Title: member.Name, Detail: detail,
			Control: s.memberActions(e, team, member, row),
		})
	}
	s.add.Kind, s.add.Small, s.add.Icon, s.add.Label = kit.PlaqueTonal, true, look.Icon.Add, i18n.T("Add member")
	full := len(team.Members) >= membersMax
	if s.add.Clicked(gtx) && crewReady && !full {
		p.addMember(team)
	}
	s.every.Kind, s.every.Small, s.every.Icon, s.every.Label = kit.PlaqueTonal, true, look.Icon.Agents, i18n.T("Set every member's model")
	if s.every.Clicked(gtx) && crewReady && len(team.Members) > 0 {
		p.setEveryEngine(team)
	}
	rows = append(rows,
		look.Setting{Title: i18n.T("Add member"), Detail: i18n.Tf("A team has one to %d seats, including any marshal or synthesizer.", membersMax),
			Control: func(gtx layout.Context) layout.Dimensions {
				if full || !crewReady {
					gtx = gtx.Disabled()
				}
				return s.add.Layout(gtx, th.Theme)
			}},
		look.Setting{Title: i18n.T("Member models"), Detail: i18n.Tf("Members on the teams default run %s.", teamsui.DefaultEngine(snapshot).Place()),
			Control: func(gtx layout.Context) layout.Dimensions {
				if !crewReady {
					gtx = gtx.Disabled()
				}
				return s.every.Layout(gtx, th.Theme)
			}},
	)
	return func(gtx layout.Context) layout.Dimensions {
		return look.SettingsSection(gtx, th, look.Head{Title: i18n.T("Crew and models"), Number: 2, Of: sectionCount}, rows)
	}
}

func (s *settingsTab) memberActions(e *env.Env, team protocol.Team, member protocol.Member, row *memberRow) layout.Widget {
	th := e.Theme
	return func(gtx layout.Context) layout.Dimensions {
		if s.graph.dirty() || s.graph.busy {
			gtx = gtx.Disabled()
		}
		return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
			layout.Rigid(teamsui.Mark(th, member.Name, 32, member.Lead)),
			layout.Rigid(layout.Spacer{Width: th.Sp.S2}.Layout),
			layout.Flexed(1, kit.Text(th.Theme, th.T.Detail, th.R.Fg2, member.Title)),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions { return row.edit.Layout(gtx, th.Theme) }),
			layout.Rigid(layout.Spacer{Width: th.Sp.S1}.Layout),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				if len(team.Members) <= 1 {
					gtx = gtx.Disabled()
				}
				return row.drop.Layout(gtx, th.Theme)
			}),
		)
	}
}

func (s *settingsTab) accessSection(gtx layout.Context, e *env.Env, snapshot appstate.Snapshot, team protocol.Team) layout.Widget {
	th := e.Theme
	rows := s.toolRows(e, team)
	rows = append(rows, s.credentialRows(gtx, e, snapshot, team)...)
	rows = append(rows, s.mailTrustRows(gtx, e, snapshot, team)...)
	rows = append(rows, s.accessRows(gtx, e, team)...)
	return func(gtx layout.Context) layout.Dimensions {
		return look.SettingsSection(gtx, th, look.Head{Title: i18n.T("Tools and access"), Number: 3, Of: sectionCount}, rows)
	}
}

func (s *settingsTab) toolRows(e *env.Env, team protocol.Team) []look.Setting {
	th := e.Theme
	tools := s.tools.Show(team.Tools)
	s.screen.Value, s.mail.Value = tools.Screen, tools.Mail
	toggle := func(name string, control *kit.Toggle) layout.Widget {
		return func(gtx layout.Context) layout.Dimensions {
			dimensions := look.Named(gtx, name, func(gtx layout.Context) layout.Dimensions { return control.Layout(gtx, th.Theme) })
			if control.Changed() {
				wanted := protocol.TeamTools{Screen: s.screen.Value, Mail: s.mail.Value}
				s.tools.Hold(wanted)
				// both are sent, a merge patch would keep a tool that is turned off and so left out
				updateTeam(e, team.ID, map[string]any{"tools": map[string]bool{"screen": wanted.Screen, "mail": wanted.Mail}}, s.tools.Drop)
			}
			return dimensions
		}
	}
	return []look.Setting{
		{Title: i18n.T("Its own screen"), Detail: i18n.T("Opens apps on a hidden screen you can watch. It does not ask each time."), Control: toggle(i18n.T("Its own screen"), &s.screen)},
		{Title: i18n.T("Mail"), Detail: i18n.T("Writes mail; every mail asks you first unless you trust its recipient."), Control: toggle(i18n.T("Mail"), &s.mail)},
	}
}

func (s *settingsTab) credentialRow(id string) *credentialRow {
	if s.credentials == nil {
		s.credentials = make(map[string]*credentialRow)
	}
	row := s.credentials[id]
	if row == nil {
		row = new(credentialRow)
		s.credentials[id] = row
	}
	return row
}

func (s *settingsTab) credentialRows(gtx layout.Context, e *env.Env, snapshot appstate.Snapshot, team protocol.Team) []look.Setting {
	th := e.Theme
	s.vault.Kind, s.vault.Small, s.vault.Label = kit.PlaqueTonal, true, i18n.T("Open the vault")
	if s.vault.Clicked(gtx) {
		e.NavigateTeams(env.TeamsPlace{Tab: protocol.TabVault})
	}
	vaultPlaque := func(gtx layout.Context) layout.Dimensions { return s.vault.Layout(gtx, th.Theme) }
	vault := snapshot.Vault
	switch {
	case vault.State != protocol.VaultOpen:
		reason := vault.Reason
		if reason == "" {
			reason = i18n.T("The vault is closed, so the team can use no credential now.")
		}
		return []look.Setting{{Title: i18n.T("The vault is closed"), Detail: reason, Control: vaultPlaque}}
	case len(vault.Credentials) == 0:
		return []look.Setting{{Title: i18n.T("No credential yet"), Detail: i18n.T("Add one in the vault, then let this team use it here."), Control: vaultPlaque}}
	}
	var rows []look.Setting
	disclosureTitle := i18n.T("Agents never see these values")
	disclosure := i18n.T("Hyakki adds a credential to a request for the team, only to the hosts it may go to.")
	for _, credential := range vault.Credentials {
		if credential.Use == protocol.CredentialRuntime {
			disclosureTitle = i18n.T("Mediated requests and runtime exports differ")
			disclosure = i18n.T("For HTTP and mail, Hyakki uses the value without giving it to the agent. Runtime grants explicitly let a configured CLI or command read the secret: no host allowlist or first-use host prompt. Revoking a grant stops future exports, but cannot erase copies already read.")
		}
		rows = append(rows, s.credentialSetting(e, team, credential))
		for _, trust := range credential.Trusted {
			if trust.Team == team.ID {
				rows = append(rows, s.trustSetting(gtx, e, credential, trust))
			}
		}
	}
	return append(rows, look.Setting{
		Title: disclosureTitle, Control: vaultPlaque,
		Detail: disclosure,
	})
}

func (s *settingsTab) credentialSetting(e *env.Env, team protocol.Team, credential protocol.Credential) look.Setting {
	th := e.Theme
	row := s.credentialRow(credential.ID)
	row.toggle.Value = row.held.Show(slices.Contains(credential.Teams, team.ID))
	where := strings.Join(credential.Hosts, ", ")
	if credential.Use == protocol.CredentialMail {
		where = i18n.T("mail password")
	}
	if credential.Use == protocol.CredentialRuntime {
		where = i18n.T("runtime export; trusted process can read it without host restrictions")
	}
	name := i18n.Tf("May use %s", credential.Name)
	if credential.Use == protocol.CredentialRuntime {
		name = i18n.Tf("Allow trusted runtime process to read %s", credential.Name)
	}
	return look.Setting{
		Title: name, Detail: credential.Description,
		Control: func(gtx layout.Context) layout.Dimensions {
			dimensions := layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
				layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
					return kit.Label{Style: th.T.Detail, Color: th.R.Fg2, MaxLines: 2}.Layout(gtx, th.Theme, where)
				}),
				layout.Rigid(layout.Spacer{Width: th.Sp.S3}.Layout),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return look.Named(gtx, name, func(gtx layout.Context) layout.Dimensions { return row.toggle.Layout(gtx, th.Theme) })
				}),
			)
			if row.toggle.Changed() {
				allowed := slices.DeleteFunc(slices.Clone(credential.Teams), func(id string) bool { return id == team.ID })
				if row.toggle.Value {
					allowed = append(allowed, team.ID)
				}
				row.held.Hold(row.toggle.Value)
				patch := rawJSON(map[string]any{"teams": allowed})
				e.Call(protocol.MVaultUpdate, protocol.VaultUpdateParams{ID: credential.ID, Patch: patch, RuntimeConsent: credential.Use == protocol.CredentialRuntime && row.toggle.Value}, new(protocol.Credential), func(err error) {
					row.held.Drop(err)
					e.Report(err)
				})
			}
			return dimensions
		},
	}
}

// a host this team sends the credential to without asking; revoking it makes the next use there ask again
func (s *settingsTab) trustSetting(gtx layout.Context, e *env.Env, credential protocol.Credential, trust protocol.CredentialTrust) look.Setting {
	th := e.Theme
	if s.trusts == nil {
		s.trusts = make(map[protocol.CredentialTrust]*kit.Plaque)
	}
	revoke := s.trusts[trust]
	if revoke == nil {
		revoke = new(kit.Plaque)
		s.trusts[trust] = revoke
	}
	revoke.Kind, revoke.Small, revoke.Label = kit.PlaqueTonal, true, i18n.T("Revoke")
	if revoke.Clicked(gtx) && !revoke.Busy {
		rest := slices.DeleteFunc(slices.Clone(credential.Trusted), func(each protocol.CredentialTrust) bool { return each == trust })
		revoke.Busy = true
		patch := rawJSON(map[string]any{"trusted": rest})
		e.Call(protocol.MVaultUpdate, protocol.VaultUpdateParams{ID: credential.ID, Patch: patch}, new(protocol.Credential), func(err error) {
			revoke.Busy = false
			e.Report(err)
		})
	}
	return look.Setting{
		Title:   i18n.Tf("Sends %s to %s without asking", credential.Name, trust.Host),
		Detail:  i18n.T("Revoke it and the next use there asks you first."),
		Control: func(gtx layout.Context) layout.Dimensions { return revoke.Layout(gtx, th.Theme) },
	}
}

// recipients this team mails without asking; the account is sent whole with the one recipient left out
func (s *settingsTab) mailTrustRows(gtx layout.Context, e *env.Env, snapshot appstate.Snapshot, team protocol.Team) []look.Setting {
	th := e.Theme
	account := snapshot.Vault.Mail
	if account == nil || snapshot.Vault.State != protocol.VaultOpen {
		return nil
	}
	if s.mailTrusts == nil {
		s.mailTrusts = make(map[protocol.MailTrust]*kit.Plaque)
	}
	var rows []look.Setting
	for _, trust := range account.Trusted {
		if trust.Team != team.ID {
			continue
		}
		revoke := s.mailTrusts[trust]
		if revoke == nil {
			revoke = new(kit.Plaque)
			s.mailTrusts[trust] = revoke
		}
		revoke.Kind, revoke.Small, revoke.Label = kit.PlaqueTonal, true, i18n.T("Revoke")
		if revoke.Clicked(gtx) && !revoke.Busy {
			changed := *account
			changed.Trusted = slices.DeleteFunc(slices.Clone(account.Trusted), func(each protocol.MailTrust) bool { return each == trust })
			revoke.Busy = true
			e.Call(protocol.MVaultMail, protocol.MailAccountParams{Account: &changed}, nil, func(err error) {
				revoke.Busy = false
				e.Report(err)
			})
		}
		rows = append(rows, look.Setting{
			Title:   i18n.Tf("Mails %s without asking", trust.Recipient),
			Detail:  i18n.T("Revoke it and the next mail to this recipient asks you first."),
			Control: func(gtx layout.Context) layout.Dimensions { return revoke.Layout(gtx, th.Theme) },
		})
	}
	return rows
}

// both lists go whole, as a merge patch replaces a list rather than merging it
func (s *settingsTab) accessRows(gtx layout.Context, e *env.Env, team protocol.Team) []look.Setting {
	send := func(access protocol.TeamAccess) {
		updateTeam(e, team.ID, map[string]any{"access": map[string]any{"writable": access.Writable, "hidden": access.Hidden}}, func(error) {})
	}
	if list, changed := s.writable.Changed(); changed {
		send(protocol.TeamAccess{Writable: list, Hidden: team.Access.Hidden})
	}
	if list, changed := s.hidden.Changed(); changed {
		send(protocol.TeamAccess{Writable: team.Access.Writable, Hidden: list})
	}
	s.writable.Title, s.writable.Placeholder = i18n.T("A folder this team may change"), i18n.T("~/Work/project")
	s.hidden.Title, s.hidden.Placeholder = i18n.T("A path hidden from this team"), i18n.T("~/Documents/private")
	return []look.Setting{
		{Title: i18n.T("Folders it may change"), Below: true,
			Detail:  i18n.T("Besides its own folder. It still cannot see other teams' folders."),
			Control: func(gtx layout.Context) layout.Dimensions { return s.writable.Layout(gtx, e, team.Access.Writable) }},
		{Title: i18n.T("Also hidden from this team"), Below: true,
			Detail:  i18n.T("Its agents see these as empty, on top of what Settings, Teams hides from every team."),
			Control: func(gtx layout.Context) layout.Dimensions { return s.hidden.Layout(gtx, e, team.Access.Hidden) }},
	}
}

func (s *settingsTab) permissionsSection(e *env.Env, snapshot appstate.Snapshot, team protocol.Team) layout.Widget {
	th := e.Theme
	approvals := s.approvalsHeld.Show(team.Approvals)
	rows := []look.Setting{
		{Title: i18n.T("Permissions"), Detail: approvalsSentence(approvals), Control: func(gtx layout.Context) layout.Dimensions {
			dimensions := teamsui.ApprovalsSelect(gtx, e, &s.approvals, approvals)
			values := teamsui.ApprovalValues
			if index, changed := s.approvals.Changed(); changed && index >= 0 && index < len(values) && values[index] != approvals {
				s.approvalsHeld.Hold(values[index])
				updateTeam(e, team.ID, map[string]any{"approvals": values[index]}, s.approvalsHeld.Drop)
			}
			return dimensions
		}},
		{Title: i18n.T("Sandbox"), Detail: i18n.T("It runs in a sandbox: it can change only its folder and the folders above.")},
		{Title: i18n.T("Strategy approval"), Detail: i18n.T("Review proposed plans before execution; this does not replace tool approvals."), Control: func(gtx layout.Context) layout.Dimensions {
			s.plan.Value = s.planHeld.Show(team.PlanApproval)
			s.plan.Label = i18n.T("Approve plans before execution")
			dimensions := s.plan.Layout(gtx, th.Theme)
			if s.plan.Changed() {
				s.planHeld.Hold(s.plan.Value)
				updateTeam(e, team.ID, map[string]any{"planApproval": s.plan.Value}, s.planHeld.Drop)
			}
			return dimensions
		}},
	}
	warning := warningSlab(th, snapshot, team)
	return func(gtx layout.Context) layout.Dimensions {
		children := []layout.FlexChild{layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return look.SettingsSection(gtx, th, look.Head{Title: i18n.T("Permissions"), Number: 4, Of: sectionCount}, rows)
		})}
		if warning != nil {
			children = append(children, layout.Rigid(layout.Spacer{Height: th.Sp.S3}.Layout), layout.Rigid(warning))
		}
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
	}
}

// a team that holds a credential, reads what others wrote and can send could be talked into sending what it holds
func warningSlab(th *look.Theme, snapshot appstate.Snapshot, team protocol.Team) layout.Widget {
	holds := false
	for _, credential := range snapshot.Vault.Credentials {
		holds = holds || slices.Contains(credential.Teams, team.ID)
	}
	reads := team.Tools.Screen || team.Tools.Mail
	if !holds || !reads || !team.Tools.Mail {
		return nil
	}
	return func(gtx layout.Context) layout.Dimensions {
		return teamsui.NotchedSlab(gtx, th, palette.Attention, th.Sp.S4, func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Start}.Layout(gtx,
				layout.Rigid(look.IconWidget(look.Icon.Warning, 20, th.R.Amber)),
				layout.Rigid(layout.Spacer{Width: th.Sp.S3}.Layout),
				layout.Flexed(1, look.Paragraph(th, th.T.Body, th.R.Fg, i18n.T("This team reads untrusted text and can send it out. Keep its credentials narrow."))),
			)
		})
	}
}

func (s *settingsTab) budgetSection(gtx layout.Context, e *env.Env, snapshot appstate.Snapshot, team protocol.Team) layout.Widget {
	th := e.Theme
	s.warn.Min, s.warn.Max, s.warn.Step, s.warn.Unit = 1, 99, 1, i18n.T("percent")
	s.turns.Min, s.turns.Max, s.turns.Step, s.turns.Unit = 1, protocol.TeamTaskTurnsMax, 1, i18n.T("turns")
	s.minutes.Min, s.minutes.Max, s.minutes.Step, s.minutes.Unit = 0, protocol.TeamTaskMinutesMax, 15, i18n.T("minutes")
	minutes := i18n.T("A task that runs past it stops and says so.")
	if s.minutes.Value == 0 {
		minutes = i18n.T("No time limit: a task works until it reports or runs out of turns.")
	}
	rows := []look.Setting{
		{Title: i18n.T("Tokens a month"), Detail: i18n.T("At the limit the team pauses and asks you to raise it."),
			Changed: s.limit.Value != team.Budget.MonthTokens, Reset: func() { s.limit.Value = team.Budget.MonthTokens },
			Control: func(gtx layout.Context) layout.Dimensions { return s.limit.Layout(gtx, th) }},
		{Title: i18n.T("Warn at"), Detail: i18n.T("You are told once a month when the team gets this close."),
			Changed: s.warn.Value != team.Budget.WarnPercent, Reset: func() { s.warn.Value = team.Budget.WarnPercent },
			Control: func(gtx layout.Context) layout.Dimensions { return s.warn.Layout(gtx, th.Theme) }},
		{Title: i18n.T("Turns per task"), Detail: i18n.T("A task that uses them all stops and says so."),
			Changed: s.turns.Value != team.Budget.TaskTurns, Reset: func() { s.turns.Value = team.Budget.TaskTurns },
			Control: func(gtx layout.Context) layout.Dimensions { return s.turns.Layout(gtx, th.Theme) }},
		{Title: i18n.T("Minutes per task"), Detail: minutes,
			Changed: s.minutes.Value != team.Budget.TaskMinutes, Reset: func() { s.minutes.Value = team.Budget.TaskMinutes },
			Control: func(gtx layout.Context) layout.Dimensions { return s.minutes.Layout(gtx, th.Theme) }},
		s.runningSetting(e, snapshot, team),
	}
	if budget := s.budget(); budget != team.Budget {
		rows = append(rows, s.budgetBar.setting(gtx, th, true, !s.identityBar.shown,
			func() { s.setBudget(team) },
			func(done func(error)) {
				updateTeam(e, team.ID, map[string]any{"budget": budget}, done, s.budgetUpdated)
			}))
	}
	return func(gtx layout.Context) layout.Dimensions {
		return look.SettingsSection(gtx, th, look.Head{Title: i18n.T("Budget and limits"), Number: 5, Of: sectionCount}, rows)
	}
}

// 0 follows Settings, Teams, and says what that is now
func (s *settingsTab) runningSetting(e *env.Env, snapshot appstate.Snapshot, team protocol.Team) look.Setting {
	settings := snapshot.Settings.Teams
	running := s.runningHeld.Show(team.Running)
	labels := make([]string, len(runningValues))
	for index, value := range runningValues {
		labels[index] = i18n.Tf("%d at once", value)
	}
	labels[0] = i18n.Tf("Same as every team (%d)", settings.PerTeam)
	labels[1] = i18n.T("One at a time")
	return look.Setting{
		Title:  i18n.T("Tasks at once in this team"),
		Detail: i18n.Tf("More at once finishes sooner and spends faster. Every team together runs at most %d.", settings.MaxRunning),
		Control: func(gtx layout.Context) layout.Dimensions {
			s.running.Form, s.running.Label = look.SelectQuiet, i18n.T("Tasks at once in this team")
			dimensions := s.running.Layout(gtx, e.Theme, e.Overlays, labels, max(0, slices.Index(runningValues[:], running)))
			if index, changed := s.running.Changed(); changed && index >= 0 && index < len(runningValues) && runningValues[index] != running {
				s.runningHeld.Hold(runningValues[index])
				updateTeam(e, team.ID, map[string]any{"running": runningValues[index]}, s.runningHeld.Drop)
			}
			return dimensions
		},
	}
}

func notifySentence(value string) string {
	switch value {
	case protocol.NotifyNeeds:
		return i18n.T("Only its questions reach the desktop while no Hyakki window is open; reports wait in the Inbox.")
	case protocol.NotifyNone:
		return i18n.T("Nothing reaches the desktop; everything waits in the Inbox.")
	}
	return i18n.T("Every report and every question reaches the desktop while no Hyakki window is open.")
}

func (s *settingsTab) notifySection(e *env.Env, snapshot appstate.Snapshot, team protocol.Team) layout.Widget {
	th := e.Theme
	notify := s.notifyHeld.Show(team.Notify)
	rows := []look.Setting{
		{Title: i18n.T("Notifications"), Detail: notifySentence(notify), Control: func(gtx layout.Context) layout.Dimensions {
			dimensions := teamsui.NotifySelect(gtx, e, &s.notify, notify)
			values := teamsui.NotifyValues
			if index, changed := s.notify.Changed(); changed && index >= 0 && index < len(values) && values[index] != notify {
				s.notifyHeld.Hold(values[index])
				updateTeam(e, team.ID, map[string]any{"notify": values[index]}, s.notifyHeld.Drop)
			}
			return dimensions
		}},
	}
	if hours := snapshot.Settings.Teams.QuietHours; hours != nil && notify != protocol.NotifyNone {
		rows = append(rows, look.Setting{
			Title:  i18n.T("Quiet hours from Settings apply"),
			Detail: i18n.Tf("Nothing reaches the desktop from %s to %s; what needs you waits in the Inbox.", hours.From, hours.To),
		})
	}
	return func(gtx layout.Context) layout.Dimensions {
		return look.SettingsSection(gtx, th, look.Head{Title: i18n.T("Notifications"), Number: 6, Of: sectionCount}, rows)
	}
}

func (s *settingsTab) memorySection(e *env.Env, team protocol.Team) layout.Widget {
	th := e.Theme
	rows := s.memory.Rows(e, team.TeamMemory(), func(memory protocol.TeamMemory, done func(error)) {
		updateTeam(e, team.ID, map[string]any{"memory": memory}, done)
	})
	return func(gtx layout.Context) layout.Dimensions {
		return look.SettingsSection(gtx, th, look.Head{Title: i18n.T("Memory"), Number: 7, Of: sectionCount}, rows)
	}
}

func (s *settingsTab) exportSection(gtx layout.Context, e *env.Env, team protocol.Team) layout.Widget {
	th := e.Theme
	s.export.Kind, s.export.Small, s.export.Icon, s.export.Label = kit.PlaqueTonal, true, look.Icon.Export, i18n.T("Export team")
	if s.export.Clicked(gtx) {
		exportTeam(e, team)
	}
	rows := []look.Setting{{
		Title: i18n.T("Export team"), Detail: i18n.T("Writes the team into its folder as a file you can import elsewhere, without its history or any credential value."),
		Control: func(gtx layout.Context) layout.Dimensions { return s.export.Layout(gtx, th.Theme) },
	}}
	return func(gtx layout.Context) layout.Dimensions {
		return look.SettingsSection(gtx, th, look.Head{Title: i18n.T("Export")}, rows)
	}
}

// the top-level tasks of a team that ended, which a clear deletes with their pieces
func finishedTasks(snapshot appstate.Snapshot, team string) int {
	count := 0
	for _, task := range snapshot.TeamTasks {
		if task.Team == team && task.Parent == "" && teamsui.Finished(task) {
			count++
		}
	}
	return count
}

func (s *settingsTab) dangerSection(gtx layout.Context, e *env.Env, p *Page, snapshot appstate.Snapshot, team protocol.Team) layout.Widget {
	th := e.Theme
	finished := finishedTasks(snapshot, team.ID)
	s.pause.Kind, s.pause.Small, s.pause.Icon, s.pause.Label = kit.PlaqueTonal, true, look.Icon.Pause, i18n.T("Pause")
	pause := i18n.T("Nothing new starts and its working tasks stop until you resume it.")
	switch {
	case team.Paused && team.PauseReason == protocol.PausedByBudget:
		s.pause.Icon, s.pause.Label = look.Icon.Play, i18n.T("Resume")
		pause = i18n.Tf("It stopped at its limit. Resuming raises the limit to %s tokens a month.", teamsui.Tokens(raisedLimit(team)))
	case team.Paused:
		s.pause.Icon, s.pause.Label = look.Icon.Play, i18n.T("Resume")
		pause = i18n.T("It is paused. Resuming lets it start tasks and routines again.")
	}
	s.clear.Kind, s.clear.Small, s.clear.Icon, s.clear.Label = kit.PlaqueDangerLine, true, look.Icon.Delete, i18n.T("Clear")
	s.remove.Kind, s.remove.Small, s.remove.Icon, s.remove.Label = kit.PlaqueDangerLine, true, look.Icon.Delete, i18n.T("Delete team")
	if s.pause.Clicked(gtx) {
		togglePause(e, team)
	}
	if s.clear.Clicked(gtx) && finished > 0 {
		p.remove.openClear(team, finished)
	}
	if s.remove.Clicked(gtx) {
		p.remove.open(team)
	}
	clearing := i18n.T("No finished task to clear.")
	if finished > 0 {
		clearing = i18n.Tf("%s finished. They go with their conversations; what they made stays in the folder.", tasksWord(finished))
	}
	rows := []look.Setting{
		{Title: s.pause.Label, Detail: pause, Control: func(gtx layout.Context) layout.Dimensions { return s.pause.Layout(gtx, th.Theme) }},
		{Title: i18n.T("Clear finished tasks"), Detail: clearing, Control: func(gtx layout.Context) layout.Dimensions {
			if finished == 0 {
				gtx = gtx.Disabled()
			}
			return s.clear.Layout(gtx, th.Theme)
		}},
		{Title: i18n.T("Delete team"), Detail: i18n.T("Its tasks, routines and grants go. Its folder and what is in it stay."),
			Control: func(gtx layout.Context) layout.Dimensions { return s.remove.Layout(gtx, th.Theme) }},
	}
	return func(gtx layout.Context) layout.Dimensions {
		return look.SettingsSection(gtx, th, look.Head{Title: i18n.T("Danger zone")}, rows)
	}
}
