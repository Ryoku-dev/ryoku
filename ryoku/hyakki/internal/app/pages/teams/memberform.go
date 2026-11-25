package teams

import (
	"strings"

	"gioui.org/layout"

	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/kit"
	"ryoku-i18n"

	"ryoku-rashin/internal/app/env"
	"ryoku-rashin/internal/app/look"
	"ryoku-rashin/internal/app/pages/teamsui"
	"ryoku-rashin/internal/protocol"
)

// the fields of one member, shared by the member dialog and the rows of a draft
type memberForm struct {
	base         protocol.Member
	name, title  kit.TextField
	rank, spirit kit.TextField
	lead         kit.Toggle
	engine       teamsui.EngineForm
	instructions look.MultiLine
}

func (f *memberForm) set(member protocol.Member) {
	f.base = member
	f.name.SetText(member.Name)
	f.title.SetText(member.Title)
	f.rank.SetText(member.Rank)
	f.spirit.SetText(member.Spirit)
	f.lead.Value = member.Lead
	f.engine.Set(member.Model, member.Options)
	f.instructions.SetText(member.Instructions)
}

func (f *memberForm) member() protocol.Member {
	model, options := f.engine.Value()
	member := f.base
	member.Name, member.Title, member.Lead = strings.TrimSpace(f.name.Text()), strings.TrimSpace(f.title.Text()), f.lead.Value
	member.Model, member.Options, member.Instructions = model, options, strings.TrimSpace(f.instructions.Text())
	member.Rank, member.Spirit = strings.TrimSpace(f.rank.Text()), strings.TrimSpace(f.spirit.Text())
	return member
}

func (f *memberForm) nameField(th *look.Theme) layout.Widget {
	f.name.Label, f.name.Placeholder = i18n.T("Name"), i18n.T("Researcher")
	return func(gtx layout.Context) layout.Dimensions { return f.name.Layout(gtx, th.Theme) }
}

func (f *memberForm) titleField(th *look.Theme) layout.Widget {
	f.title.Label, f.title.Placeholder = i18n.T("Title"), i18n.T("Finds and checks sources")
	return func(gtx layout.Context) layout.Dimensions { return f.title.Layout(gtx, th.Theme) }
}

func (f *memberForm) identityFields(th *look.Theme) layout.Widget {
	f.rank.Label, f.spirit.Label = i18n.T("Rank"), i18n.T("Spirit identity")
	f.rank.Help = i18n.T("Role vocabulary only. Rank grants no tools, permissions or paid model.")
	f.spirit.Help = i18n.T("Catalogue identity or your own name; changing it does not replace your roster.")
	return pair(th,
		func(gtx layout.Context) layout.Dimensions { return f.rank.Layout(gtx, th.Theme) },
		func(gtx layout.Context) layout.Dimensions { return f.spirit.Layout(gtx, th.Theme) },
	)
}

func (f *memberForm) instructionsField(th *look.Theme) layout.Widget {
	f.instructions.Label, f.instructions.Lines = i18n.T("Instructions"), 5
	f.instructions.Placeholder = i18n.T("What this member does and how, in two or three plain sentences.")
	f.instructions.Help = i18n.T("Every task this member works on starts with these.")
	return func(gtx layout.Context) layout.Dimensions { return f.instructions.Layout(gtx, th) }
}

func (f *memberForm) engineField(e *env.Env) layout.Widget {
	return func(gtx layout.Context) layout.Dimensions { return f.engine.Layout(gtx, e) }
}
