package agent

import (
	"gioui.org/layout"

	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/kit"
	"ryoku-i18n"

	"ryoku-rashin/internal/app/env"
	"ryoku-rashin/internal/app/look"
	"ryoku-rashin/internal/app/state"
	"ryoku-rashin/internal/paths"
	"ryoku-rashin/internal/protocol"
)

// what the board offers while it has no session: the way to a harness, then the places an agent can work in
type boardStart struct {
	install env.HarnessInstall
	here    map[string]*kit.Plaque
	add     kit.Plaque
	// a project that was just added, chosen once the daemon lists it
	added string
}

func (s *boardStart) layoutInstall(gtx layout.Context, e *env.Env, snapshot state.Snapshot) layout.Dimensions {
	th := e.Theme
	rows := s.install.Rows(gtx, e, snapshot)
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
				layout.Flexed(1, kit.Text(th.Theme, th.T.Title, th.R.Fg, i18n.T("Install a harness first"))),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions { return s.install.Again(gtx, e) }),
			)
		}),
		layout.Rigid(layout.Spacer{Height: th.Sp.S1}.Layout),
		layout.Rigid(look.Paragraph(th, th.T.Detail, th.R.Fg2,
			i18n.T("An agent runs through a harness: a coding agent you install, which Rashin starts, watches and asks you for. These are the ones it knows."))),
		layout.Rigid(layout.Spacer{Height: th.Sp.S3}.Layout),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions { return look.SettingsSlab(gtx, th, rows) }),
	)
}

func (s *boardStart) layoutPlaces(gtx layout.Context, e *env.Env, snapshot state.Snapshot, b *board) layout.Dimensions {
	th := e.Theme
	if s.here == nil {
		s.here = make(map[string]*kit.Plaque)
	}
	rows := make([]look.Setting, 0, len(snapshot.Projects)+1)
	for index, project := range snapshot.Projects {
		if project.ID == s.added {
			s.added = ""
			b.place.projectIndex, b.place.worktreeIndex = index+1, worktreeRoot
		}
		here := s.here[project.ID]
		if here == nil {
			here = new(kit.Plaque)
			s.here[project.ID] = here
		}
		chosen := b.place.projectIndex == index+1
		here.Kind, here.Small, here.Label = kit.PlaqueTonal, true, i18n.T("Work here")
		if chosen {
			here.Kind, here.Label = kit.PlaqueOn, i18n.T("Chosen")
		}
		if here.Clicked(gtx) {
			b.place.projectIndex, b.place.worktreeIndex = index+1, worktreeRoot
			b.composer.Focus()
		}
		rows = append(rows, look.Setting{
			Title: project.Name, Detail: paths.Short(project.Root),
			Control: func(gtx layout.Context) layout.Dimensions { return here.Layout(gtx, th.Theme) },
		})
	}
	s.add.Kind, s.add.Small, s.add.Label, s.add.Icon = kit.PlaqueTonal, true, i18n.T("Add project"), look.Icon.Add
	if s.add.Clicked(gtx) {
		s.addProject(e, b)
	}
	rows = append(rows, look.Setting{
		Title:   i18n.T("Another folder"),
		Detail:  i18n.T("As a project it gives each session its own worktree, so two agents never write to the same files."),
		Control: func(gtx layout.Context) layout.Dimensions { return s.add.Layout(gtx, th.Theme) },
	})
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(kit.Text(th.Theme, th.T.Title, th.R.Fg, i18n.T("Where it can work"))),
		layout.Rigid(layout.Spacer{Height: th.Sp.S2}.Layout),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions { return look.SettingsSlab(gtx, th, rows) }),
	)
}

func (s *boardStart) addProject(e *env.Env, b *board) {
	e.ChooseFolder(i18n.T("Add a project"), paths.Home(), func(folder string) {
		var added protocol.ProjectInfo
		e.Call(protocol.MProjectsAdd, protocol.ProjectAddParams{Path: folder}, &added, func(err error) {
			if err != nil {
				e.Notify(err.Error())
				return
			}
			s.added = added.ID
			b.composer.Focus()
		})
	}, func() {
		// without a chooser the Projects page has the field to type a folder into
		e.Navigate(protocol.TabProjects, "")
	})
}
