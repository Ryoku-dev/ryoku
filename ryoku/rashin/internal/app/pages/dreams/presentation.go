package dreams

import (
	"gioui.org/layout"

	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/kit"
	"ryoku-i18n"

	"ryoku-rashin/internal/app/chat"
	"ryoku-rashin/internal/app/env"
	"ryoku-rashin/internal/app/look"
	"ryoku-rashin/internal/app/state"
	"ryoku-rashin/internal/paths"
	"ryoku-rashin/internal/protocol"
)

// the manager's hand-over, set as the transcript sets an answer, and the CEO's answer to it
type presentationView struct {
	body, tests   chat.Prose
	accept, notes kit.Plaque
	files         []kit.Click
}

func (v *presentationView) section(gtx layout.Context, e *env.Env, p *Page, dream protocol.Team) layout.Widget {
	th := e.Theme
	presentation := dream.Dream.Presentation
	if presentation == nil {
		return nil
	}
	_, open := awaiting(dream)
	v.accept.Kind, v.accept.Label = kit.PlaqueGold, i18n.T("Accept")
	v.notes.Kind, v.notes.Label = kit.PlaqueTonal, i18n.T("Send notes")
	if v.accept.Clicked(gtx) && open && !v.accept.Busy {
		v.accept.Busy = true
		answer(e, dream, protocol.DreamAccepted, "", func(error) { v.accept.Busy = false })
	}
	if v.notes.Clicked(gtx) && open {
		p.notes.open(dream)
	}
	for len(v.files) < len(presentation.Files) {
		v.files = append(v.files, kit.Click{})
	}
	for index, file := range presentation.Files {
		if v.files[index].Clicked(gtx) {
			e.OpenPath(file)
		}
	}
	presented := i18n.Tf("Presented %s", look.Ago(gtx.Now, presentation.At))
	if dream.Dream.Rounds > 1 {
		presented = i18n.Tf("Presented %s, round %d", look.Ago(gtx.Now, presentation.At), dream.Dream.Rounds)
	}
	widgets := []layout.Widget{
		look.Paragraph(th, th.T.Title, th.R.Fg, presentation.Title),
		gap(th.Sp.S1),
		look.Paragraph(th, th.T.Caption, th.R.Fg3, presented),
		gap(th.Sp.S3),
		func(gtx layout.Context) layout.Dimensions {
			dimensions := v.body.Layout(gtx, th, e.Overlays, presentation.Body)
			env.HandleChatActions(gtx, e, "", v.body.Actions())
			return dimensions
		},
	}
	if presentation.Tests != "" {
		widgets = append(widgets,
			gap(th.Sp.S4),
			kit.Text(th.Theme, th.T.Label, th.R.Fg, i18n.T("Tests")),
			gap(th.Sp.S2),
			func(gtx layout.Context) layout.Dimensions {
				dimensions := v.tests.Layout(gtx, th, e.Overlays, presentation.Tests)
				env.HandleChatActions(gtx, e, "", v.tests.Actions())
				return dimensions
			},
		)
	}
	if len(presentation.Files) > 0 {
		widgets = append(widgets, gap(th.Sp.S4), kit.Text(th.Theme, th.T.Label, th.R.Fg, i18n.T("Files")), gap(th.Sp.S2))
		for index, file := range presentation.Files {
			click := &v.files[index]
			widgets = append(widgets, func(gtx layout.Context) layout.Dimensions {
				return look.Link(gtx, th, click, paths.Short(file))
			})
		}
	}
	widgets = append(widgets, gap(th.Sp.S4), builtBy(th, e.Snapshot(), dream))
	widgets = append(widgets, gap(th.Sp.S4), v.verdict(th, dream, open), gap(th.Sp.S5))
	return section(th, i18n.T("Presentation"), func(gtx layout.Context) layout.Dimensions {
		return stack(gtx, widgets...)
	})
}

// who did the work this round and what each really ran on, from the daemon's own records whatever the presentation says
func builtBy(th *look.Theme, snapshot state.Snapshot, dream protocol.Team) layout.Widget {
	list := builders(snapshot, dream)
	widgets := []layout.Widget{
		kit.Text(th.Theme, th.T.Label, th.R.Fg, i18n.T("Who built it")),
		gap(th.Sp.S1),
		look.Paragraph(th, th.T.Caption, th.R.Fg3, i18n.T("From Rashin's records of this round, not from the manager's words.")),
	}
	for _, each := range list {
		widgets = append(widgets, gap(th.Sp.S3), builderRow(th, snapshot, each))
	}
	return func(gtx layout.Context) layout.Dimensions { return stack(gtx, widgets...) }
}

// the name and the role, then one line per engine: the harness by name and the model by the id it was given
func builderRow(th *look.Theme, snapshot state.Snapshot, each builder) layout.Widget {
	role := each.member.Title
	if role == "" && each.member.Lead {
		role = i18n.T("Manager")
	}
	tasks := i18n.T("one task")
	if len(each.tasks) > 1 {
		tasks = i18n.Tf("%d tasks", len(each.tasks))
	}
	children := []layout.FlexChild{
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Baseline}.Layout(gtx,
				layout.Rigid(kit.Text(th.Theme, th.T.Label, th.R.Fg, each.member.Name)),
				layout.Rigid(layout.Spacer{Width: th.Sp.S2}.Layout),
				layout.Flexed(1, look.Paragraph(th, th.T.Detail, th.R.Fg2, look.Clauses(role, tasks))),
			)
		}),
	}
	for _, engine := range each.engines {
		harness := engine.harness
		if info, ok := env.HarnessByID(snapshot.Harnesses, engine.harness); ok {
			harness = info.Name
		}
		model := engine.model
		if model == "" {
			model = i18n.T("its default model")
		}
		children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Baseline}.Layout(gtx,
				layout.Rigid(kit.Text(th.Theme, th.T.Detail, th.R.Fg2, harness)),
				layout.Rigid(layout.Spacer{Width: th.Sp.S2}.Layout),
				layout.Flexed(1, look.Paragraph(th, th.T.Mono, th.R.Fg2, model)),
			)
		}))
	}
	return func(gtx layout.Context) layout.Dimensions {
		gtx.Constraints.Min.X = gtx.Constraints.Max.X
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
	}
}

// Accept and Send notes while it waits, else what the CEO answered
func (v *presentationView) verdict(th *look.Theme, dream protocol.Team, open bool) layout.Widget {
	presentation := dream.Dream.Presentation
	if open {
		return func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
				layout.Rigid(func(gtx layout.Context) layout.Dimensions { return v.accept.Layout(gtx, th.Theme) }),
				layout.Rigid(layout.Spacer{Width: th.Sp.S2}.Layout),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions { return v.notes.Layout(gtx, th.Theme) }),
			)
		}
	}
	switch {
	case presentation.Verdict == protocol.DreamNotes && presentation.Note != "":
		return look.Paragraph(th, th.T.Body, th.R.Fg2, i18n.Tf("You sent notes: %s", presentation.Note))
	case presentation.Verdict == protocol.DreamNotes:
		return look.Paragraph(th, th.T.Body, th.R.Fg2, i18n.T("You sent it back to work."))
	}
	return look.Paragraph(th, th.T.Body, th.R.Fg2, i18n.T("You accepted it."))
}
