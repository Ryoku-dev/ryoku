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
	id           string
	name, title  kit.TextField
	lead         kit.Toggle
	engine       teamsui.EngineForm
	instructions look.MultiLine
}

func (f *memberForm) set(member protocol.Member) {
	f.id = member.ID
	f.name.SetText(member.Name)
	f.title.SetText(member.Title)
	f.lead.Value = member.Lead
	f.engine.Set(member.Harness, member.Model, member.Options)
	f.instructions.SetText(member.Instructions)
}

func (f *memberForm) member() protocol.Member {
	harness, model, options := f.engine.Value()
	return protocol.Member{
		ID: f.id, Name: strings.TrimSpace(f.name.Text()), Title: strings.TrimSpace(f.title.Text()), Lead: f.lead.Value,
		Harness: harness, Model: model, Options: options, Instructions: strings.TrimSpace(f.instructions.Text()),
	}
}

func (f *memberForm) nameField(th *look.Theme) layout.Widget {
	f.name.Label, f.name.Placeholder = i18n.T("Name"), i18n.T("Researcher")
	return func(gtx layout.Context) layout.Dimensions { return f.name.Layout(gtx, th.Theme) }
}

func (f *memberForm) titleField(th *look.Theme) layout.Widget {
	f.title.Label, f.title.Placeholder = i18n.T("Title"), i18n.T("Finds and checks sources")
	return func(gtx layout.Context) layout.Dimensions { return f.title.Layout(gtx, th.Theme) }
}

func (f *memberForm) instructionsField(th *look.Theme) layout.Widget {
	f.instructions.Label, f.instructions.Lines = i18n.T("Instructions"), 5
	f.instructions.Placeholder = i18n.T("What this member does and how, in two or three plain sentences.")
	f.instructions.Help = i18n.T("Every task this member works on starts with these.")
	return func(gtx layout.Context) layout.Dimensions { return f.instructions.Layout(gtx, th) }
}

// the harness select's first option names the harness the teams default resolves to now
func teamsDefaultLabel(e *env.Env) string {
	return i18n.Tf("Teams default (%s)", teamsui.DefaultHarnessName(e.Snapshot()))
}

func (f *memberForm) engineField(e *env.Env) layout.Widget {
	return func(gtx layout.Context) layout.Dimensions { return f.engine.Layout(gtx, e, teamsDefaultLabel(e)) }
}
