package projects

import (
	"image"

	"gioui.org/layout"

	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/kit"
	"ryoku-i18n"

	"ryoku-rashin/internal/app/env"
	"ryoku-rashin/internal/app/look"
	"ryoku-rashin/internal/app/pages/graph"
	"ryoku-rashin/internal/paths"
	"ryoku-rashin/internal/protocol"
)

var projectViews = [...]string{TabOverview, TabGraph, TabChanges, TabTasks, TabActivity}

func (p *pageState) layoutProject(gtx layout.Context, e *env.Env) layout.Dimensions {
	info, _ := p.projectInfo(e)
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions { return p.layoutHeader(gtx, e, info) }),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions { return p.layoutTabs(gtx, e) }),
		layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
			gtx.Constraints.Min = gtx.Constraints.Max
			return p.rise.Layout(gtx, e.Theme, p.project+"\x00"+p.tab, 8, func(gtx layout.Context) layout.Dimensions {
				return p.layoutView(gtx, e)
			})
		}),
	)
}

func (p *pageState) layoutHeader(gtx layout.Context, e *env.Env, info protocol.ProjectInfo) layout.Dimensions {
	content := look.PaneHeaderContent{
		Title:   info.Name,
		Facts:   []look.HeaderFact{{Text: shortPath(info.Root, paths.Home()), Kind: look.FactToken}},
		Actions: []look.HeaderAction{{Icon: look.Icon.Terminal, Description: i18n.T("Open a terminal here")}, {Icon: look.Icon.Folder, Description: i18n.T("Open the folder")}},
		Menu: []kit.MenuItem{
			{Icon: look.Icon.Settings, Label: i18n.T("Project settings")},
			{Divider: true},
			{Icon: look.Icon.Close, Label: i18n.T("Forget project"), Danger: true},
		},
	}
	if info.Attention > 0 {
		content.Claim, content.ClaimTone = i18n.Tf("%d need you", info.Attention), 3
	}
	dimensions := p.header.Layout(gtx, e.Theme, e.Overlays, content)
	if index, ok := p.header.ActionClicked(); ok {
		if index == 0 {
			openTerminal(info.Root)
		} else {
			openFolder(info.Root)
		}
	}
	if item, ok := p.header.MenuClicked(); ok {
		if item == 0 {
			p.settings.open(info)
		} else {
			p.forget.open(info)
		}
	}
	return dimensions
}

func (p *pageState) layoutTabs(gtx layout.Context, e *env.Env) layout.Dimensions {
	th := e.Theme
	p.tabs.Labels = []string{i18n.T("Overview"), i18n.T("Graph"), i18n.T("Changes"), i18n.T("Tasks"), i18n.T("Activity")}
	for index, view := range projectViews {
		if view == p.tab {
			p.tabs.Selected = index
		}
	}
	// a tab label sits 12 inside its box so the strip starts 12 early to put the first label under the title
	dimensions := layout.Inset{Left: 9}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return p.tabs.Layout(gtx, th.Theme)
	})
	if index, changed := p.tabs.Changed(); changed && index < len(projectViews) {
		e.NavigateWork(p.project, projectViews[index], p.worktree)
	}
	line := max(1, gtx.Dp(1))
	width := gtx.Constraints.Max.X
	look.FillRect(gtx, image.Rect(0, dimensions.Size.Y-line, width, dimensions.Size.Y), th.R.Vein)
	return layout.Dimensions{Size: image.Pt(width, dimensions.Size.Y)}
}

func (p *pageState) layoutView(gtx layout.Context, e *env.Env) layout.Dimensions {
	switch p.tab {
	case TabGraph:
		return p.layoutGraph(gtx, e)
	case TabChanges:
		return p.changes.layout(gtx, e, p)
	case TabTasks:
		return p.tasks.layout(gtx, e, p)
	case TabActivity:
		return p.activity.layout(gtx, e, p)
	default:
		return p.overview.layout(gtx, e, p)
	}
}

func (p *pageState) layoutGraph(gtx layout.Context, e *env.Env) layout.Dimensions {
	dimensions := p.graph.Layout(gtx, e, p.project)
	for _, action := range p.graph.Actions() {
		switch action.Kind {
		case graph.ActOpenWorktree:
			e.NavigateWork(action.Project, TabOverview, action.Worktree)
		case graph.ActOpenChanges:
			e.NavigateWork(action.Project, TabChanges, action.Worktree)
		case graph.ActNewSession:
			p.newSession(e, action.Worktree)
		case graph.ActOpenSession:
			e.Navigate(protocol.TabAgent, action.Session)
		}
	}
	return dimensions
}
