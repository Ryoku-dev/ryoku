package agent

import (
	"strings"

	"gioui.org/layout"
	"gioui.org/widget"

	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/kit"
	"ryoku-i18n"

	"ryoku-rashin/internal/app/env"
	"ryoku-rashin/internal/app/look"
	"ryoku-rashin/internal/paths"
	"ryoku-rashin/internal/protocol"
)

type createDialog struct {
	place        *placement
	dialog       kit.Dialog
	start        kit.Plaque
	cancel       kit.Plaque
	folder       env.PathField
	name         kit.TextField
	base, branch kit.TextField
}

func (d *createDialog) open(e *env.Env, place *placement) {
	place.prime(e)
	d.place, d.dialog.Open = place, true
	if d.folder.Text() == "" {
		d.folder.SetText(paths.Home())
	}
}

func (d *createDialog) layout(gtx layout.Context, e *env.Env) {
	if d.place == nil {
		return
	}
	th := e.Theme
	snapshot := e.Snapshot()
	d.place.clamp(snapshot)
	ready := len(env.InstalledHarnesses(snapshot)) > 0
	d.cancel.Kind, d.cancel.Label = kit.PlaqueQuiet, i18n.T("Cancel")
	d.start.Kind, d.start.Label = kit.PlaqueGold, i18n.T("Start session")
	if d.cancel.Clicked(gtx) {
		d.dialog.Dismiss(gtx)
	}
	if d.start.Clicked(gtx) && ready {
		more := details{folder: d.folder.Path(), name: d.name.Text(), base: d.base.Text(), branch: d.branch.Text()}
		if params, ok := d.place.params(snapshot, e.AgentModels, more); ok {
			var info protocol.SessionInfo
			e.Call(protocol.MSessionNew, params, &info, func(err error) {
				if err != nil {
					e.Notify(err.Error())
					return
				}
				open(e, info.ID)
			})
		}
		d.dialog.Dismiss(gtx)
	}
	d.dialog.Layout(gtx, th.Theme, i18n.T("Start an agent session"),
		func(gtx layout.Context) layout.Dimensions { return d.body(gtx, e) },
		func(gtx layout.Context) layout.Dimensions { return d.cancel.Layout(gtx, th.Theme) },
		func(gtx layout.Context) layout.Dimensions {
			if !ready {
				gtx = gtx.Disabled()
			}
			return d.start.Layout(gtx, th.Theme)
		},
	)
	if d.dialog.Closed() {
		d.place = nil
	}
}

func (d *createDialog) body(gtx layout.Context, e *env.Env) layout.Dimensions {
	th := e.Theme
	snapshot := e.Snapshot()
	place := d.place
	field := func(selection *look.Select, label string, options []look.SelectOption, selected int) layout.FlexChild {
		selection.Form, selection.Label, selection.Icon, selection.Return = look.SelectField, label, nil, nil
		return layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			gtx.Constraints.Min.X = gtx.Constraints.Max.X
			return selection.LayoutOptions(gtx, th, e.Overlays, options, selected)
		})
	}
	text := func(input *kit.TextField, label, placeholder string) layout.FlexChild {
		input.Label, input.Placeholder = label, placeholder
		return layout.Rigid(func(gtx layout.Context) layout.Dimensions { return input.Layout(gtx, th.Theme) })
	}
	gap := layout.Rigid(layout.Spacer{Height: th.Sp.S3}.Layout)
	children := []layout.FlexChild{field(&place.project, i18n.T("Project"), look.Labelled(place.projectLabels(snapshot)), place.projectIndex), gap}
	if place.projectIndex == 0 {
		d.folder.Title, d.folder.Label, d.folder.Placeholder = i18n.T("Where the agent works"), i18n.T("Folder"), paths.Home()
		children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions { return d.folder.Layout(gtx, e) }), gap)
	} else {
		project := snapshot.Projects[place.projectIndex-1]
		children = append(children, field(&place.worktree, i18n.T("Worktree"), look.Labelled(place.worktreeLabels(snapshot)), place.worktreeIndex), gap)
		if place.worktreeIndex == worktreeNew {
			d.base.Mono, d.branch.Mono = true, true
			children = append(children,
				text(&d.name, i18n.T("Name"), ""), gap,
				text(&d.base, i18n.T("Base"), project.BaseRef), gap,
				text(&d.branch, i18n.T("Branch"), project.Settings.BranchPrefix+strings.TrimSpace(d.name.Text())), gap,
			)
		}
	}
	harnesses := env.InstalledHarnesses(snapshot)
	children = append(children, field(&place.harness, i18n.T("Harness"), harnessOptions(harnesses), place.pick.Index(snapshot, harnesses)))
	if len(harnesses) > 0 {
		children = append(children, gap, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			gtx.Constraints.Min.X = gtx.Constraints.Max.X
			return place.model.layout(gtx, e, place.picked(snapshot, harnesses), look.SelectField, nil)
		}))
		children = append(children, place.model.noteRows(th)...)
	}
	children = append(children,
		layout.Rigid(layout.Spacer{Height: th.Sp.S2}.Layout),
		layout.Rigid(look.Paragraph(th, th.T.Caption, th.R.Fg3, approvalsNote(snapshot, len(harnesses) == 0))),
	)
	place.update(e, harnesses)
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
}

type renameDialog struct {
	id         string
	dialog     kit.Dialog
	field      kit.TextField
	do, cancel kit.Plaque
}

func (d *renameDialog) open(info protocol.SessionInfo) {
	d.id, d.dialog.Open = info.ID, true
	d.field.SetText(info.Title)
	d.field.Editor.Submit = true
}

func (d *renameDialog) layout(gtx layout.Context, e *env.Env) {
	if d.id == "" {
		return
	}
	submitted := false
	for {
		event, ok := d.field.Editor.Update(gtx)
		if !ok {
			break
		}
		if _, ok := event.(widget.SubmitEvent); ok {
			submitted = true
		}
	}
	d.field.Label = i18n.T("Name")
	d.do.Kind, d.do.Label = kit.PlaqueGold, i18n.T("Rename")
	d.cancel.Kind, d.cancel.Label = kit.PlaqueQuiet, i18n.T("Cancel")
	title := strings.TrimSpace(d.field.Text())
	if (d.do.Clicked(gtx) || submitted) && title != "" {
		renameSession(e, d.id, title)
		d.dialog.Dismiss(gtx)
	}
	if d.cancel.Clicked(gtx) {
		d.dialog.Dismiss(gtx)
	}
	d.dialog.Layout(gtx, e.Theme.Theme, i18n.T("Rename session"),
		func(gtx layout.Context) layout.Dimensions { return d.field.Layout(gtx, e.Theme.Theme) },
		func(gtx layout.Context) layout.Dimensions { return d.cancel.Layout(gtx, e.Theme.Theme) },
		func(gtx layout.Context) layout.Dimensions { return d.do.Layout(gtx, e.Theme.Theme) },
	)
	if d.dialog.Closed() {
		d.id = ""
	}
}

type removeDialog struct {
	id      string
	confirm kit.Confirm
}

func (d *removeDialog) open(id string) {
	d.id, d.confirm.Dialog.Open, d.confirm.Danger = id, true, true
}

func (d *removeDialog) layout(gtx layout.Context, e *env.Env, page *Page) {
	if d.id == "" {
		return
	}
	title := sessionTitle(e.Snapshot().Sessions[d.id])
	body := i18n.Tf("“%s” and its transcript are removed. Its worktree and files stay.", title)
	if d.confirm.Layout(gtx, e.Theme.Theme, i18n.T("Delete this session?"), body, i18n.T("Delete"), "") {
		id := d.id
		e.Call(protocol.MSessionDelete, protocol.SessionRef{ID: id}, new(protocol.Empty), func(err error) {
			if err != nil {
				e.Notify(err.Error())
				return
			}
			if page.selected == id {
				open(e, "")
			}
		})
	}
	if d.confirm.Closed() {
		d.id = ""
	}
}

type revertDialog struct {
	id      string
	turn    int
	confirm kit.Confirm
}

func (d *revertDialog) open(id string, turn int) {
	d.id, d.turn, d.confirm.Dialog.Open, d.confirm.Danger = id, turn, true, true
}

func (d *revertDialog) layout(gtx layout.Context, e *env.Env) {
	if d.id == "" {
		return
	}
	body := i18n.T("The worktree's files return to how they were before this turn. The conversation stays as it is.")
	if d.confirm.Layout(gtx, e.Theme.Theme, i18n.T("Revert files to before this turn?"), body, i18n.T("Revert files"), "") {
		params := protocol.TurnParams{ID: d.id, Turn: d.turn}
		e.Call(protocol.MSessionRevertTurn, params, new(protocol.GitOpResult), e.Report)
	}
	if d.confirm.Closed() {
		d.id = ""
	}
}
