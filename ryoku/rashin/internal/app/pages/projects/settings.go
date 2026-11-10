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

type settingsDialog struct {
	project protocol.ProjectInfo
	dialog  kit.Dialog
	scroll  look.Scroller

	base, root, prefix, setup, copyIgnored kit.TextField
	auto                                   kit.Toggle
	mergeDays, inactiveDays                kit.Stepper
	protect                                look.Select
	protectSelected                        int
	save, cancel                           kit.Plaque
}

func (d *settingsDialog) open(info protocol.ProjectInfo) {
	d.project, d.dialog.Open, d.dialog.Width = info, true, 640
	settings := info.Settings
	d.base.SetText(settings.BaseRef)
	d.root.SetText(settings.WorktreeRoot)
	d.prefix.SetText(settings.BranchPrefix)
	d.setup.SetText(settings.Setup)
	d.copyIgnored.SetText(strings.Join(settings.CopyIgnored, ", "))
	d.auto.Value = settings.Cleanup.Auto
	d.mergeDays.Value, d.inactiveDays.Value = settings.Cleanup.AfterMergeDays, settings.Cleanup.InactiveDays
	d.protectSelected = 0
	if settings.Cleanup.Protect == protocol.ProtectAny {
		d.protectSelected = 1
	}
}

func (d *settingsDialog) layout(gtx layout.Context, e *env.Env, page *pageState) {
	if d.project.ID == "" {
		return
	}
	th := e.Theme
	d.cancel.Kind, d.cancel.Label = kit.PlaqueQuiet, i18n.T("Cancel")
	d.save.Kind, d.save.Label = kit.PlaqueGold, i18n.T("Save")
	if d.cancel.Clicked(gtx) {
		d.dialog.Dismiss(gtx)
	}
	if d.save.Clicked(gtx) {
		d.submit(e, page)
		d.dialog.Dismiss(gtx)
	}
	d.dialog.Layout(gtx, th.Theme, i18n.Tf("%s settings", d.project.Name),
		func(gtx layout.Context) layout.Dimensions {
			// the body scrolls so the dialog fits a short window
			gtx.Constraints.Max.Y = min(gtx.Constraints.Max.Y, gtx.Dp(520))
			gtx.Constraints.Min.Y = 0
			rows := d.rows(e)
			return d.scroll.Layout(gtx, th, len(rows), func(gtx layout.Context, index int) layout.Dimensions {
				return layout.Inset{Bottom: th.Sp.S3, Right: th.Sp.S3}.Layout(gtx, rows[index])
			})
		},
		func(gtx layout.Context) layout.Dimensions { return d.cancel.Layout(gtx, th.Theme) },
		func(gtx layout.Context) layout.Dimensions { return d.save.Layout(gtx, th.Theme) },
	)
	if d.dialog.Closed() {
		d.project = protocol.ProjectInfo{}
	}
}

func (d *settingsDialog) rows(e *env.Env) []layout.Widget {
	th := e.Theme
	field := func(input *kit.TextField, label, help string, mono bool) layout.Widget {
		input.Label, input.Help, input.Mono = label, help, mono
		return func(gtx layout.Context) layout.Dimensions { return input.Layout(gtx, th.Theme) }
	}
	d.auto.Label = i18n.T("Remove finished worktrees by themselves")
	d.mergeDays.Min, d.mergeDays.Max, d.mergeDays.Step, d.mergeDays.Unit = 0, 365, 1, i18n.T("days after its branch merged")
	d.inactiveDays.Min, d.inactiveDays.Max, d.inactiveDays.Step, d.inactiveDays.Unit = 0, 365, 1, i18n.T("days without activity")
	d.protect.Label = i18n.T("Never remove a worktree that has")
	protect := []string{i18n.T("Uncommitted changes"), i18n.T("Any file Git does not track")}
	return []layout.Widget{
		field(&d.base, i18n.T("Base"), i18n.T("New worktrees start from this ref."), true),
		field(&d.root, i18n.T("Worktree folder"), i18n.T("Where new worktrees are made."), true),
		field(&d.prefix, i18n.T("Branch prefix"), i18n.T("Put before a new worktree's name to make its branch."), true),
		field(&d.setup, i18n.T("Setup command"), i18n.T("Run in a new worktree before an agent starts."), true),
		field(&d.copyIgnored, i18n.T("Files copied into new worktrees"), i18n.T("Ignored files a worktree needs, separated by commas."), true),
		func(gtx layout.Context) layout.Dimensions { return d.auto.Layout(gtx, th.Theme) },
		func(gtx layout.Context) layout.Dimensions {
			if !d.auto.Value {
				gtx = gtx.Disabled()
			}
			return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
				layout.Rigid(func(gtx layout.Context) layout.Dimensions { return d.mergeDays.Layout(gtx, th.Theme) }),
				layout.Rigid(layout.Spacer{Height: th.Sp.S2}.Layout),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions { return d.inactiveDays.Layout(gtx, th.Theme) }),
			)
		},
		func(gtx layout.Context) layout.Dimensions {
			gtx.Constraints.Min.X = gtx.Constraints.Max.X
			dimensions := d.protect.Layout(gtx, th, e.Overlays, protect, d.protectSelected)
			if index, changed := d.protect.Changed(); changed {
				d.protectSelected = index
			}
			return dimensions
		},
	}
}

func (d *settingsDialog) submit(e *env.Env, page *pageState) {
	protect := protocol.ProtectUncommitted
	if d.protectSelected == 1 {
		protect = protocol.ProtectAny
	}
	patch := protocol.ProjectSettings{
		BaseRef: d.base.Text(), WorktreeRoot: d.root.Text(), BranchPrefix: d.prefix.Text(),
		Setup: d.setup.Text(), CopyIgnored: splitList(d.copyIgnored.Text()),
		Cleanup: protocol.CleanupPolicy{
			Auto: d.auto.Value, AfterMergeDays: d.mergeDays.Value, InactiveDays: d.inactiveDays.Value, Protect: protect,
		},
	}
	params := protocol.ProjectSettingsParams{Project: d.project.ID, Patch: rawPatch(patch)}
	e.Call(protocol.MProjectsSettings, params, new(protocol.ProjectInfo), page.report())
}
