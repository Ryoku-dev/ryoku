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

var (
	approvalValues = [...]string{protocol.ApprovalsReadOnly, protocol.ApprovalsAsk, protocol.ApprovalsAuto}
	notifyValues   = [...]string{protocol.NotifyAll, protocol.NotifyNeeds, protocol.NotifyNone}
)

// the team's definition as settings slabs: what it is, where it works, its crew, what it may use and how it asks
type settingsTab struct {
	scroll     look.Scroller
	filledFrom string

	name       kit.TextField
	purpose    look.MultiLine
	saveText   kit.Plaque
	folder     env.PathField
	saveFolder kit.Plaque

	members map[string]*memberRow
	add     kit.Plaque

	screen, mail kit.Toggle
	tools        teamsui.Held[protocol.TeamTools]
	credentials  map[string]*credentialRow
	vault        kit.Plaque

	approvals     look.Select
	approvalsHeld teamsui.Held[string]
	notify        look.Select
	notifyHeld    teamsui.Held[string]

	export, remove kit.Plaque
}

type memberRow struct {
	edit, drop       kit.IconPlaque
	editTip, dropTip look.Tip
}

type credentialRow struct {
	toggle kit.Toggle
	held   teamsui.Held[bool]
}

// the fields start from the team and are filled again whenever the team was changed, here or elsewhere
func (s *settingsTab) fill(team protocol.Team) {
	key := team.ID + "\x00" + team.Updated.String()
	if s.filledFrom == key {
		return
	}
	s.filledFrom = key
	s.name.SetText(team.Name)
	s.purpose.SetText(team.Purpose)
	// the folder reads as the header writes it, under ~; the daemon takes ~ as home when it saves
	s.folder.SetText(paths.Short(team.Folder))
}

func (s *settingsTab) layout(gtx layout.Context, e *env.Env, p *Page, team protocol.Team) layout.Dimensions {
	th := e.Theme
	s.fill(team)
	snapshot := e.Snapshot()
	items := []layout.Widget{
		s.textSection(gtx, e, team),
		s.folderSection(gtx, e, team),
		s.membersSection(gtx, e, p, snapshot, team),
		s.toolsSection(gtx, e, team),
		s.credentialsSection(gtx, e, snapshot, team),
		s.askingSection(gtx, e, team),
		warningSlab(th, snapshot, team),
		s.endSection(gtx, e, p, team),
	}
	return s.scroll.Layout(gtx, th, len(items), func(gtx layout.Context, index int) layout.Dimensions {
		if items[index] == nil {
			return layout.Dimensions{}
		}
		inset := gtx.Dp(look.ContentInset(e.Class))
		full := gtx.Constraints.Max.X
		width := min(max(0, full-2*inset), gtx.Dp(look.SettingsColumn))
		child := gtx
		child.Constraints = layout.Constraints{Min: image.Pt(width, 0), Max: image.Pt(width, gtx.Constraints.Max.Y)}
		top := 0
		if index == 0 {
			top = gtx.Dp(th.Sp.S4)
		}
		stack := op.Offset(image.Pt((full-width)/2, top)).Push(gtx.Ops)
		dimensions := items[index](child)
		stack.Pop()
		return layout.Dimensions{Size: image.Pt(full, top+dimensions.Size.Y+gtx.Dp(th.Sp.S4))}
	})
}

func (s *settingsTab) textSection(gtx layout.Context, e *env.Env, team protocol.Team) layout.Widget {
	th := e.Theme
	name, purpose := strings.TrimSpace(s.name.Text()), strings.TrimSpace(s.purpose.Text())
	changed := name != "" && (name != team.Name || purpose != team.Purpose)
	s.saveText.Kind, s.saveText.Small, s.saveText.Label = kit.PlaqueLine, true, i18n.T("Save")
	if s.saveText.Clicked(gtx) && changed {
		s.saveText.Busy = true
		updateTeam(e, team.ID, map[string]any{"name": name, "purpose": purpose}, func(error) { s.saveText.Busy = false })
	}
	s.purpose.Lines = 3
	rows := []look.Setting{
		{Title: i18n.T("Name"), Below: true, Control: func(gtx layout.Context) layout.Dimensions { return s.name.Layout(gtx, th.Theme) }},
		{Title: i18n.T("Purpose"), Detail: i18n.T("Every member reads it with every task, so say what the team is for."), Below: true,
			Control: func(gtx layout.Context) layout.Dimensions {
				return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
					layout.Rigid(func(gtx layout.Context) layout.Dimensions { return s.purpose.Layout(gtx, th) }),
					layout.Rigid(layout.Spacer{Height: th.Sp.S3}.Layout),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						if !changed {
							gtx = gtx.Disabled()
						}
						return s.saveText.Layout(gtx, th.Theme)
					}),
				)
			}},
	}
	return func(gtx layout.Context) layout.Dimensions {
		return look.SettingsSection(gtx, th, i18n.T("Name and purpose"), rows)
	}
}

func (s *settingsTab) folderSection(gtx layout.Context, e *env.Env, team protocol.Team) layout.Widget {
	th := e.Theme
	folder := s.folder.Path()
	changed := folder != "" && folder != paths.Short(team.Folder) && folder != team.Folder
	s.saveFolder.Kind, s.saveFolder.Small, s.saveFolder.Label = kit.PlaqueLine, true, i18n.T("Save")
	if s.saveFolder.Clicked(gtx) && changed {
		s.saveFolder.Busy = true
		updateTeam(e, team.ID, map[string]any{"folder": folder}, func(error) { s.saveFolder.Busy = false })
	}
	s.folder.Title = i18n.T("The team's folder")
	// the field's own plaque chooses a folder; the header's folder link opens it
	rows := []look.Setting{{
		Title: i18n.T("Folder"), Below: true,
		Detail: i18n.T("Where the team works and keeps what it makes. Choosing another folder does not move the files."),
		Control: func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
				layout.Flexed(1, func(gtx layout.Context) layout.Dimensions { return s.folder.Layout(gtx, e) }),
				layout.Rigid(layout.Spacer{Width: th.Sp.S2}.Layout),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					if !changed {
						gtx = gtx.Disabled()
					}
					return s.saveFolder.Layout(gtx, th.Theme)
				}),
			)
		},
	}}
	return func(gtx layout.Context) layout.Dimensions {
		return look.SettingsSection(gtx, th, i18n.T("Folder"), rows)
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

func (s *settingsTab) membersSection(gtx layout.Context, e *env.Env, p *Page, snapshot appstate.Snapshot, team protocol.Team) layout.Widget {
	th := e.Theme
	rows := make([]layout.Widget, 0, len(team.Members)+1)
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
	s.add.Kind, s.add.Small, s.add.Icon, s.add.Label = kit.PlaqueLine, true, look.Icon.Add, i18n.T("Add member")
	full := len(team.Members) >= membersMax
	if s.add.Clicked(gtx) && !full {
		p.addMember(team)
	}
	rows = append(rows, func(gtx layout.Context) layout.Dimensions {
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
	})
	return func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			layout.Rigid(kit.Text(th.Theme, th.T.Section, th.R.Fg, i18n.T("Members"))),
			layout.Rigid(layout.Spacer{Height: th.Sp.S3}.Layout),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions { return veinedSlab(gtx, th, rows) }),
		)
	}
}

func (s *settingsTab) layoutMember(e *env.Env, snapshot appstate.Snapshot, team protocol.Team, member protocol.Member, row *memberRow) layout.Widget {
	th := e.Theme
	about := member.Title
	if member.Lead {
		about = i18n.Tf("Leads the team · %s", member.Title)
	}
	runs := i18n.Tf("%s · %s", harnessName(snapshot, member.Harness), modelName(member.Model))
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
					layout.Rigid(kit.Text(th.Theme, th.T.Caption, th.R.Fg3, about)),
					layout.Rigid(kit.Text(th.Theme, th.T.MonoSm, th.R.Fg3, runs)),
				)
			}),
			layout.Rigid(plaque(&row.editTip, &row.edit, false)),
			layout.Rigid(layout.Spacer{Width: th.Sp.S1}.Layout),
			layout.Rigid(plaque(&row.dropTip, &row.drop, len(team.Members) <= 1)),
		)
	}
}

// rows on one stone1 slab with a vein between them, as the settings page sets them
func veinedSlab(gtx layout.Context, th *look.Theme, rows []layout.Widget) layout.Dimensions {
	return kit.Slab(gtx, th.R.Stone1, th.Cut.Mid, func(gtx layout.Context) layout.Dimensions {
		gtx.Constraints.Min.X = gtx.Constraints.Max.X
		return layout.Inset{Left: th.Sp.S4, Right: th.Sp.S4, Top: th.Sp.S2, Bottom: th.Sp.S2}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			children := make([]layout.FlexChild, 0, 2*len(rows))
			for index, row := range rows {
				if index > 0 {
					children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						size := image.Pt(gtx.Constraints.Max.X, max(1, gtx.Dp(1)))
						look.FillRect(gtx, image.Rectangle{Max: size}, th.R.Vein)
						return layout.Dimensions{Size: size}
					}))
				}
				children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					gtx.Constraints.Min.X = gtx.Constraints.Max.X
					return layout.Inset{Top: th.Sp.S3, Bottom: th.Sp.S3}.Layout(gtx, row)
				}))
			}
			return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
		})
	})
}

func (s *settingsTab) toolsSection(gtx layout.Context, e *env.Env, team protocol.Team) layout.Widget {
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
	rows := []look.Setting{
		{Title: i18n.T("Its own screen"), Detail: i18n.T("Opens apps on a hidden screen you can watch. It does not ask each time."), Control: toggle(i18n.T("Its own screen"), &s.screen)},
		{Title: i18n.T("Mail"), Detail: i18n.T("Writes mail; every mail asks you first unless you trust its recipient."), Control: toggle(i18n.T("Mail"), &s.mail)},
	}
	return func(gtx layout.Context) layout.Dimensions {
		return look.SettingsSection(gtx, th, i18n.T("Tools"), rows)
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

func (s *settingsTab) credentialsSection(gtx layout.Context, e *env.Env, snapshot appstate.Snapshot, team protocol.Team) layout.Widget {
	th := e.Theme
	s.vault.Kind, s.vault.Small, s.vault.Label = kit.PlaqueLine, true, i18n.T("Open the vault")
	if s.vault.Clicked(gtx) {
		e.NavigateTeams(env.TeamsPlace{Tab: protocol.TabVault})
	}
	vaultPlaque := func(gtx layout.Context) layout.Dimensions { return s.vault.Layout(gtx, th.Theme) }
	vault := snapshot.Vault
	var rows []look.Setting
	switch {
	case vault.State != protocol.VaultOpen:
		reason := vault.Reason
		if reason == "" {
			reason = i18n.T("The vault is closed, so the team can use no credential now.")
		}
		rows = append(rows, look.Setting{Title: i18n.T("The vault is closed"), Detail: reason, Control: vaultPlaque})
	case len(vault.Credentials) == 0:
		rows = append(rows, look.Setting{Title: i18n.T("No credential yet"), Detail: i18n.T("Add one in the vault, then let this team use it here."), Control: vaultPlaque})
	default:
		for _, credential := range vault.Credentials {
			rows = append(rows, s.credentialSetting(e, team, credential))
		}
		rows = append(rows, look.Setting{
			Title: i18n.T("Agents never see these values"), Control: vaultPlaque,
			Detail: i18n.T("Rashin adds a credential to a request for the team, only to the hosts it may go to."),
		})
	}
	return func(gtx layout.Context) layout.Dimensions {
		return look.SettingsSection(gtx, th, i18n.T("Credentials"), rows)
	}
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
					return kit.Label{Style: th.T.MonoSm, Color: th.R.Fg3, MaxLines: 2}.Layout(gtx, th.Theme, where)
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

func (s *settingsTab) askingSection(gtx layout.Context, e *env.Env, team protocol.Team) layout.Widget {
	th := e.Theme
	approvals := s.approvalsHeld.Show(team.Approvals)
	notify := s.notifyHeld.Show(team.Notify)
	rows := []look.Setting{
		{Title: i18n.T("Permissions"), Detail: approvalsSentence(approvals), Control: func(gtx layout.Context) layout.Dimensions {
			dimensions := approvalsSelect(gtx, e, &s.approvals, approvals)
			if index, changed := s.approvals.Changed(); changed && index >= 0 && index < len(approvalValues) && approvalValues[index] != approvals {
				s.approvalsHeld.Hold(approvalValues[index])
				updateTeam(e, team.ID, map[string]any{"approvals": approvalValues[index]}, s.approvalsHeld.Drop)
			}
			return dimensions
		}},
		{Title: i18n.T("Notifications"), Detail: i18n.T("What reaches the desktop while no Rashin window is open."), Control: func(gtx layout.Context) layout.Dimensions {
			dimensions := notifySelect(gtx, e, &s.notify, notify)
			if index, changed := s.notify.Changed(); changed && index >= 0 && index < len(notifyValues) && notifyValues[index] != notify {
				s.notifyHeld.Hold(notifyValues[index])
				updateTeam(e, team.ID, map[string]any{"notify": notifyValues[index]}, s.notifyHeld.Drop)
			}
			return dimensions
		}},
	}
	return func(gtx layout.Context) layout.Dimensions {
		return look.SettingsSection(gtx, th, i18n.T("Asking and telling"), rows)
	}
}

// the value that stops the asking reads in amber with an open lock, as it does for Agents
func approvalsSelect(gtx layout.Context, e *env.Env, control *look.Select, value string) layout.Dimensions {
	selected := max(0, slices.Index(approvalValues[:], value))
	control.Form, control.Label = look.SelectQuiet, i18n.T("Permissions")
	control.Icon, control.Tone = look.Icon.Lock, palette.Idle
	if value == protocol.ApprovalsAuto {
		control.Icon, control.Tone = look.Icon.Unlock, palette.Attention
	}
	options := []look.SelectOption{
		{Label: i18n.T("Read only"), Detail: i18n.T("Reads and reports. It changes no file.")},
		{Label: i18n.T("Ask before changes"), Detail: i18n.T("Reads by itself and asks you before it changes anything.")},
		{Label: i18n.T("Act without asking"), Detail: i18n.T("Changes files and runs commands without asking you.")},
	}
	return control.LayoutOptions(gtx, e.Theme, e.Overlays, options, selected)
}

func notifySelect(gtx layout.Context, e *env.Env, control *look.Select, value string) layout.Dimensions {
	selected := max(0, slices.Index(notifyValues[:], value))
	control.Form, control.Label = look.SelectQuiet, i18n.T("Notifications")
	options := []string{i18n.T("Reports and when it needs me"), i18n.T("Only when it needs me"), i18n.T("Nothing")}
	return control.Layout(gtx, e.Theme, e.Overlays, options, selected)
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
		gtx.Constraints.Min.X = gtx.Constraints.Max.X
		return kit.Slab(gtx, th.R.Field(palette.Attention), th.Cut.Mid, func(gtx layout.Context) layout.Dimensions {
			gtx.Constraints.Min.X = gtx.Constraints.Max.X
			return layout.UniformInset(th.Sp.S4).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Start}.Layout(gtx,
					layout.Rigid(look.IconWidget(look.Icon.Warning, 20, th.R.Amber)),
					layout.Rigid(layout.Spacer{Width: th.Sp.S3}.Layout),
					layout.Flexed(1, look.Paragraph(th, th.T.Body, th.R.Fg, i18n.T("This team reads untrusted text and can send it out. Keep its credentials narrow."))),
				)
			})
		})
	}
}

func (s *settingsTab) endSection(gtx layout.Context, e *env.Env, p *Page, team protocol.Team) layout.Widget {
	th := e.Theme
	s.export.Kind, s.export.Small, s.export.Icon, s.export.Label = kit.PlaqueLine, true, look.Icon.Export, i18n.T("Export team")
	s.remove.Kind, s.remove.Small, s.remove.Icon, s.remove.Label = kit.PlaqueDangerLine, true, look.Icon.Delete, i18n.T("Delete team")
	if s.export.Clicked(gtx) {
		exportTeam(e, team)
	}
	if s.remove.Clicked(gtx) {
		p.remove.open(team)
	}
	rows := []look.Setting{
		{Title: i18n.T("Export team"), Detail: i18n.T("Writes the team into its folder as a file you can import elsewhere, without its history or any credential value."),
			Control: func(gtx layout.Context) layout.Dimensions { return s.export.Layout(gtx, th.Theme) }},
		{Title: i18n.T("Delete team"), Detail: i18n.T("Its tasks, routines and grants go. Its folder and what is in it stay."),
			Control: func(gtx layout.Context) layout.Dimensions { return s.remove.Layout(gtx, th.Theme) }},
	}
	return func(gtx layout.Context) layout.Dimensions {
		return look.SettingsSection(gtx, th, i18n.T("Export and delete"), rows)
	}
}
