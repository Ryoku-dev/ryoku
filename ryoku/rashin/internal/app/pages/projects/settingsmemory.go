package projects

import (
	"strings"

	"gioui.org/layout"

	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/kit"
	"ryoku-i18n"

	"ryoku-rashin/internal/app/env"
	"ryoku-rashin/internal/app/look"
	"ryoku-rashin/internal/protocol"
)

var projectMemoryValues = [...]string{protocol.ProjectMemoryShared, protocol.ProjectMemoryKept}

// unlike the rest of the dialog this saves the moment it is picked, as ux asks for project memory
type projectMemory struct {
	mode kit.Segmented
	// a pick shows at once and stays until the daemon agrees
	seen   string
	filled bool
	open   kit.Plaque
}

func (m *projectMemory) layout(gtx layout.Context, e *env.Env, d *settingsDialog) layout.Dimensions {
	th := e.Theme
	info := d.project
	for _, project := range e.Snapshot().Projects {
		if project.ID == info.ID {
			info = project
		}
	}
	stored := info.Settings.Memory
	if !m.filled || stored != m.seen {
		m.seen, m.filled = stored, true
		m.mode.Selected = projectMemoryIndex(stored)
	}
	m.mode.Labels = []string{i18n.T("Shared"), i18n.T("Kept here")}
	scope, count := projectScope(e, info.Name)
	m.open.Kind, m.open.Small, m.open.Label = kit.PlaqueQuiet, true, i18n.T("Open")
	if m.open.Clicked(gtx) && scope != "" {
		d.dialog.Dismiss(gtx)
		e.OpenMemory(env.MemoryPlace{Filter: env.MemoryProjects, Scope: scope})
	}
	detail := i18n.T("Ask and sessions here use what Rashin knows about you, and what it learns here can help elsewhere.")
	if projectMemoryValues[m.mode.Selected] == protocol.ProjectMemoryKept {
		detail = i18n.T("Only this project's memories are used here, and nothing learned here leaves it.")
	}
	held := i18n.Tf("No memories about %s yet.", info.Name)
	if count > 0 {
		held = memoriesAbout(count, info.Name)
	}
	children := []layout.FlexChild{
		layout.Rigid(kit.Text(th.Theme, th.T.Section, th.R.Fg, i18n.T("Memory"))),
		layout.Rigid(layout.Spacer{Height: th.Sp.S2}.Layout),
		layout.Rigid(kit.Text(th.Theme, th.T.Label, th.R.Fg, i18n.T("Project memory"))),
		layout.Rigid(layout.Spacer{Height: th.Sp.S1}.Layout),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			gtx.Constraints.Min.X = gtx.Dp(260)
			dimensions := m.mode.Layout(gtx, th.Theme)
			if index, changed := m.mode.Changed(); changed && index < len(projectMemoryValues) && projectMemoryValues[index] != stored {
				m.save(e, info.ID, projectMemoryValues[index])
			}
			return dimensions
		}),
		layout.Rigid(layout.Spacer{Height: th.Sp.S1}.Layout),
		layout.Rigid(look.Paragraph(th, th.T.Detail, th.R.Fg2, detail)),
		layout.Rigid(look.Paragraph(th, th.T.Detail, th.R.Fg2, i18n.T("Follows the repository, not the folder."))),
		layout.Rigid(layout.Spacer{Height: th.Sp.S2}.Layout),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			children := []layout.FlexChild{layout.Rigid(kit.Text(th.Theme, th.T.Detail, th.R.Fg, held))}
			if scope != "" {
				children = append(children,
					layout.Rigid(layout.Spacer{Width: th.Sp.S2}.Layout),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions { return m.open.Layout(gtx, th.Theme) }),
				)
			}
			return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx, children...)
		}),
	}
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
}

// a refusal puts the segments back on what is stored
func (m *projectMemory) save(e *env.Env, project, value string) {
	params := protocol.ProjectSettingsParams{Project: project, Patch: rawPatch(map[string]string{"memory": value})}
	e.Call(protocol.MProjectsSettings, params, new(protocol.ProjectInfo), func(err error) {
		if err != nil {
			m.mode.Selected = projectMemoryIndex(m.seen)
			e.Notify(err.Error())
		}
	})
}

func projectMemoryIndex(value string) int {
	if value == protocol.ProjectMemoryKept {
		return 1
	}
	return 0
}

// memory names a project's scope after the Work project, so the name finds it
func projectScope(e *env.Env, name string) (string, int) {
	status := e.Snapshot().Memory
	if status == nil {
		return "", 0
	}
	for _, scope := range status.Scopes {
		if strings.HasPrefix(scope.ID, protocol.MemoryProjectPrefix) && scope.Name == name {
			return scope.ID, scope.Count
		}
	}
	return "", 0
}

func memoriesAbout(count int, name string) string {
	if count == 1 {
		return i18n.Tf("1 memory about %s", name)
	}
	return i18n.Tf("%d memories about %s", count, name)
}
