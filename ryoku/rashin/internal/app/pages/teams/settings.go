package teams

import (
	"image"
	"slices"
	"strings"

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

// the groups of the Settings tab in the order they stand
const (
	sectionIdentity = iota
	sectionCrew
	sectionAccess
	sectionPermissions
	sectionBudget
	sectionNotifications
	sectionExport
	sectionDanger
	sectionCount
)

// how many of its tasks a team may run at once: 0 follows Settings, Teams
var runningValues = [...]int{0, 1, 2, 3, protocol.TeamsPerTeamMax}

// the team's definition as one column of groups with an index beside it; toggles and selects save at once,
// text and numbers wait for their group's save bar
type settingsTab struct {
	scroll  look.Scroller
	index   kit.SectionIndex
	current int
	// the group just picked, until the list has moved to it
	jumped bool
	// the team and its last change the fields were filled from
	filledFrom, filledTeam string

	name         kit.TextField
	purpose      look.MultiLine
	folder       env.PathField
	identityBase identity
	identityBar  saveBar

	members map[string]*memberRow
	add     kit.Plaque
	every   kit.Plaque

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

	limit      teamsui.TokenStepper
	warn       kit.Stepper
	turns      kit.Stepper
	minutes    kit.Stepper
	budgetBase protocol.TeamBudget
	budgetBar  saveBar

	running     look.Select
	runningHeld teamsui.Held[int]

	notify     look.Select
	notifyHeld teamsui.Held[string]

	export, pause, clear, remove kit.Plaque
}

type memberRow struct {
	edit, drop       kit.IconPlaque
	editTip, dropTip look.Tip
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

func settingsLabels() []string {
	return []string{
		i18n.T("Identity"), i18n.T("Crew and engines"), i18n.T("Tools and access"), i18n.T("Permissions"),
		i18n.T("Budget and limits"), i18n.T("Notifications"), i18n.T("Export"), i18n.T("Danger zone"),
	}
}

func (s *settingsTab) jump(section int) {
	s.current, s.jumped = section, true
	s.scroll.ScrollTo(section)
}

// follows the group at the top of the column, unless one was just picked and the list is still on its way
func (s *settingsTab) follow() {
	first := s.scroll.Position.First
	if s.jumped {
		s.jumped = first != s.current
		return
	}
	s.current = min(max(first, 0), sectionCount-1)
}

func (s *settingsTab) choose() {
	if selected, ok := s.index.Selected(); ok && selected < sectionCount {
		s.jump(selected)
	}
}

func (s *settingsTab) identity() identity {
	return identity{name: strings.TrimSpace(s.name.Text()), purpose: strings.TrimSpace(s.purpose.Text()), folder: s.folder.Path()}
}

func (s *settingsTab) setIdentity(team protocol.Team) {
	s.name.SetText(team.Name)
	s.purpose.SetText(team.Purpose)
	// the folder reads as the header writes it, under ~; the daemon takes ~ as home when it saves
	s.folder.SetText(paths.Short(team.Folder))
	s.identityBase = s.identity()
}

func (s *settingsTab) budget() protocol.TeamBudget {
	return protocol.TeamBudget{MonthTokens: s.limit.Value, WarnPercent: s.warn.Value, TaskTurns: s.turns.Value, TaskMinutes: s.minutes.Value}
}

func (s *settingsTab) setBudget(budget protocol.TeamBudget) {
	s.limit.Value, s.warn.Value, s.turns.Value, s.minutes.Value = budget.MonthTokens, budget.WarnPercent, budget.TaskTurns, budget.TaskMinutes
	s.budgetBase = s.budget()
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
	if another || s.identity() == s.identityBase {
		s.setIdentity(team)
	}
	if another || s.budget() == s.budgetBase {
		s.setBudget(team.Budget)
	}
}

func (s *settingsTab) layout(gtx layout.Context, e *env.Env, p *Page, team protocol.Team) layout.Dimensions {
	th := e.Theme
	s.fill(team)
	groups := s.groups(gtx, e, p, team)
	labels := settingsLabels()
	size := gtx.Constraints.Max
	side, column, rail, gap := gtx.Dp(look.ContentInset(e.Class)), gtx.Dp(look.SettingsColumn), gtx.Dp(200), gtx.Dp(34)
	s.follow()
	if size.X >= column+rail+gap+2*side {
		// index and column are centred together so the column stays in the middle of what's left
		left := (size.X - column - rail - gap) / 2
		look.Place(gtx, image.Rect(left, gtx.Dp(34), left+rail, size.Y), func(gtx layout.Context) layout.Dimensions {
			gtx.Constraints.Min = image.Point{}
			return s.index.Layout(gtx, th.Theme, labels, s.current, kit.FrameExpanded)
		})
		s.choose()
		look.Place(gtx, image.Rect(left+rail+gap, 0, size.X, size.Y), func(gtx layout.Context) layout.Dimensions {
			return s.layoutGroups(gtx, e, groups, 0, column)
		})
		return layout.Dimensions{Size: size}
	}
	width := min(column, size.X-2*side)
	left := (size.X - width) / 2
	picker := 0
	if size.X >= gtx.Dp(520) {
		picker = look.Place(gtx, image.Rect(left, gtx.Dp(13), left+width, size.Y), func(gtx layout.Context) layout.Dimensions {
			gtx.Constraints.Min = image.Point{}
			return s.index.Layout(gtx, th.Theme, labels, s.current, kit.FrameMedium)
		}).Size.Y + gtx.Dp(13)
		s.choose()
	}
	look.Place(gtx, image.Rect(0, picker, size.X, size.Y), func(gtx layout.Context) layout.Dimensions {
		return s.layoutGroups(gtx, e, groups, left, width)
	})
	return layout.Dimensions{Size: size}
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
		s.exportSection(gtx, e, team),
		s.dangerSection(gtx, e, p, snapshot, team),
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
		{Title: i18n.T("Name"), Below: true, Control: func(gtx layout.Context) layout.Dimensions { return s.name.Layout(gtx, th.Theme) }},
		{Title: i18n.T("Purpose"), Detail: i18n.T("Every member reads it with every task, so say what the team is for."), Below: true,
			Control: func(gtx layout.Context) layout.Dimensions { return s.purpose.Layout(gtx, th) }},
		// the field's own plaque chooses a folder; the header's folder link opens it
		{Title: i18n.T("Folder"), Below: true,
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
				updateTeam(e, team.ID, patch, done)
			}))
	}
	made := i18n.Tf("Made %s", team.Created.Local().Format("2 Jan 2006"))
	if template, ok := templateByID(p.templates.List(e), team.Template); ok {
		rows = append(rows, look.Setting{Title: i18n.Tf("Made from %s", template.Name), Detail: made})
	} else {
		rows = append(rows, look.Setting{Title: made})
	}
	return func(gtx layout.Context) layout.Dimensions {
		return look.SettingsSection(gtx, th, i18n.T("Identity"), rows)
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
	rows := make([]layout.Widget, 0, len(team.Members)+2)
	for _, member := range team.Members {
		row := s.memberRow(member.ID)
		row.edit.Kind, row.edit.Small, row.edit.Icon = kit.PlaqueQuiet, true, look.Icon.Rename
		row.edit.Description = i18n.Tf("Edit %s", member.Name)
		row.drop.Kind, row.drop.Small, row.drop.Icon = kit.PlaqueQuiet, true, look.Icon.Delete
		row.drop.Description = i18n.Tf("Remove %s", member.Name)
		if row.edit.Clicked(gtx) {
			p.editMember(team, member)
		}
		if row.drop.Clicked(gtx) && len(team.Members) > 1 {
			p.removeMember(team, member)
		}
		rows = append(rows, s.layoutMember(e, snapshot, team, member, row))
	}
	s.add.Kind, s.add.Small, s.add.Icon, s.add.Label = kit.PlaqueTonal, true, look.Icon.Add, i18n.T("Add member")
	full := len(team.Members) >= membersMax
	if s.add.Clicked(gtx) && !full {
		p.addMember(team)
	}
	s.every.Kind, s.every.Small, s.every.Icon, s.every.Label = kit.PlaqueTonal, true, look.Icon.Agents, i18n.T("Set every member's engine")
	if s.every.Clicked(gtx) && len(team.Members) > 0 {
		p.setEveryEngine(team)
	}
	rows = append(rows,
		func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					if full {
						gtx = gtx.Disabled()
					}
					return s.add.Layout(gtx, th.Theme)
				}),
				layout.Rigid(layout.Spacer{Width: th.Sp.S3}.Layout),
				layout.Flexed(1, quiet(th, i18n.Tf("A team has one to %d members, its lead included.", membersMax))),
			)
		},
		func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
				layout.Rigid(func(gtx layout.Context) layout.Dimensions { return s.every.Layout(gtx, th.Theme) }),
				layout.Rigid(layout.Spacer{Width: th.Sp.S3}.Layout),
				layout.Flexed(1, quiet(th, i18n.Tf("Members on the teams default run %s, as Settings, Teams says.", teamsui.DefaultHarnessName(snapshot)))),
			)
		},
	)
	return func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			layout.Rigid(kit.Text(th.Theme, th.T.Section, th.R.Fg, i18n.T("Crew and engines"))),
			layout.Rigid(layout.Spacer{Height: th.Sp.S3}.Layout),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions { return kit.SlabRows(gtx, th.Theme, rows...) }),
		)
	}
}

// the engine line wraps rather than cuts, so a long model id reads whole
func (s *settingsTab) layoutMember(e *env.Env, snapshot appstate.Snapshot, team protocol.Team, member protocol.Member, row *memberRow) layout.Widget {
	th := e.Theme
	about := member.Title
	if member.Lead {
		about = i18n.Tf("Leads the team · %s", member.Title)
	}
	engine := teamsui.MemberEngine(snapshot, member)
	ink := th.R.Fg2
	if engine.Missing {
		ink = th.R.Jasper
	} else if engine.Version != "" {
		engine.Harness = i18n.Tf("%s %s", engine.Harness, engine.Version)
	}
	plaque := func(tip *look.Tip, button *kit.IconPlaque, disabled bool) layout.Widget {
		return func(gtx layout.Context) layout.Dimensions {
			if disabled {
				gtx = gtx.Disabled()
			}
			return tip.Layout(gtx, th, e.Overlays, button.Description, func(gtx layout.Context) layout.Dimensions {
				return button.Layout(gtx, th.Theme)
			})
		}
	}
	return func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
			layout.Rigid(teamsui.Mark(th, member.Name, 32, member.Lead)),
			layout.Rigid(layout.Spacer{Width: th.Sp.S3}.Layout),
			layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
				return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
					layout.Rigid(kit.Text(th.Theme, th.T.Row, th.R.Fg, member.Name)),
					layout.Rigid(kit.Text(th.Theme, th.T.Detail, th.R.Fg2, about)),
					layout.Rigid(look.Paragraph(th, th.T.Detail, ink, engine.Line())),
				)
			}),
			layout.Rigid(plaque(&row.editTip, &row.edit, false)),
			layout.Rigid(layout.Spacer{Width: th.Sp.S1}.Layout),
			layout.Rigid(plaque(&row.dropTip, &row.drop, len(team.Members) <= 1)),
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
		return look.SettingsSection(gtx, th, i18n.T("Tools and access"), rows)
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
	for _, credential := range vault.Credentials {
		rows = append(rows, s.credentialSetting(e, team, credential))
		for _, trust := range credential.Trusted {
			if trust.Team == team.ID {
				rows = append(rows, s.trustSetting(gtx, e, credential, trust))
			}
		}
	}
	return append(rows, look.Setting{
		Title: i18n.T("Agents never see these values"), Control: vaultPlaque,
		Detail: i18n.T("Rashin adds a credential to a request for the team, only to the hosts it may go to."),
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
	name := i18n.Tf("May use %s", credential.Name)
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
				e.Call(protocol.MVaultUpdate, protocol.VaultUpdateParams{ID: credential.ID, Patch: patch}, new(protocol.Credential), func(err error) {
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
	}
	warning := warningSlab(th, snapshot, team)
	return func(gtx layout.Context) layout.Dimensions {
		children := []layout.FlexChild{layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return look.SettingsSection(gtx, th, i18n.T("Permissions"), rows)
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
			Control: func(gtx layout.Context) layout.Dimensions { return s.limit.Layout(gtx, th) }},
		{Title: i18n.T("Warn at"), Detail: i18n.T("You are told once a month when the team gets this close."),
			Control: func(gtx layout.Context) layout.Dimensions { return s.warn.Layout(gtx, th.Theme) }},
		{Title: i18n.T("Turns per task"), Detail: i18n.T("A task that uses them all stops and says so."),
			Control: func(gtx layout.Context) layout.Dimensions { return s.turns.Layout(gtx, th.Theme) }},
		{Title: i18n.T("Minutes per task"), Detail: minutes,
			Control: func(gtx layout.Context) layout.Dimensions { return s.minutes.Layout(gtx, th.Theme) }},
		s.runningSetting(e, snapshot, team),
	}
	if budget := s.budget(); budget != team.Budget {
		rows = append(rows, s.budgetBar.setting(gtx, th, true, !s.identityBar.shown,
			func() { s.setBudget(team.Budget) },
			func(done func(error)) { updateTeam(e, team.ID, map[string]any{"budget": budget}, done) }))
	}
	return func(gtx layout.Context) layout.Dimensions {
		return look.SettingsSection(gtx, th, i18n.T("Budget and limits"), rows)
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
		return i18n.T("Only its questions reach the desktop while no Rashin window is open; reports wait in the Inbox.")
	case protocol.NotifyNone:
		return i18n.T("Nothing reaches the desktop; everything waits in the Inbox.")
	}
	return i18n.T("Every report and every question reaches the desktop while no Rashin window is open.")
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
		return look.SettingsSection(gtx, th, i18n.T("Notifications"), rows)
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
		return look.SettingsSection(gtx, th, i18n.T("Export"), rows)
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
		return look.SettingsSection(gtx, th, i18n.T("Danger zone"), rows)
	}
}
