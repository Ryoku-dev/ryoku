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

type boardStart struct {
	here       map[string]*kit.Plaque
	add, alone kit.Plaque
	// a project that was just added, chosen once the daemon lists it
	added string
}

func (s *boardStart) layoutPlaces(gtx layout.Context, e *env.Env, snapshot state.Snapshot, b *board) layout.Dimensions {
	th := e.Theme
	if s.here == nil {
		s.here = make(map[string]*kit.Plaque)
	}
	rows := make([]look.Setting, 0, len(snapshot.Projects)+3)
	s.alone.Kind, s.alone.Small, s.alone.Label = kit.PlaqueTonal, true, i18n.T("Work here")
	if b.place.projectIndex == 0 {
		s.alone.Kind, s.alone.Label = kit.PlaqueOn, i18n.T("Chosen")
	}
	if s.alone.Clicked(gtx) {
		b.place.projectIndex, b.place.worktreeIndex = 0, worktreeRoot
		b.composer.Focus()
	}
	rows = append(rows, look.Setting{
		Title: i18n.T("No project"), Detail: i18n.T("Work in your home folder without a project or a separate worktree."),
		Control: func(gtx layout.Context) layout.Dimensions { return s.alone.Layout(gtx, th.Theme) },
	})
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
	if b.place.projectIndex > 0 {
		b.place.worktree.Form, b.place.worktree.Label, b.place.worktree.Return = look.SelectField, i18n.T("Worktree"), b.composer.FocusTag()
		b.place.worktree.Icon = look.Icon.Branch
		rows = append(rows, look.Setting{
			Title: i18n.T("Worktree"), Detail: i18n.T("Use the project folder, a separate worktree, or make a new one."),
			Control: func(gtx layout.Context) layout.Dimensions {
				return b.place.worktree.LayoutOptions(gtx, th, e.Overlays, look.Labelled(b.place.worktreeLabels(snapshot)), b.place.worktreeIndex)
			},
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
	dim := look.SettingsSection(gtx, th, look.Head{Title: i18n.T("Where it can work"), Number: 1, Of: 1}, rows)
	b.place.update()
	return dim
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
