package teams

import (
	"strings"

	"gioui.org/layout"

	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/kit"
	"ryoku-i18n"

	"ryoku-rashin/internal/app/env"
	"ryoku-rashin/internal/app/look"
	appstate "ryoku-rashin/internal/app/state"
	"ryoku-rashin/internal/protocol"
)

// the fields of one member, shared by the member dialog and the rows of a draft
type memberForm struct {
	id           string
	name, title  kit.TextField
	lead         kit.Toggle
	harness      look.Select
	harnessID    string
	model        look.Select
	modelID      string
	modelText    kit.TextField
	instructions look.MultiLine
	// listed says the model was picked from a list the last time the form was drawn, else typed
	listed bool
}

func (f *memberForm) set(member protocol.Member) {
	f.id = member.ID
	f.name.SetText(member.Name)
	f.title.SetText(member.Title)
	f.lead.Value = member.Lead
	f.harnessID = member.Harness
	f.modelID = member.Model
	f.modelText.SetText(member.Model)
	f.instructions.SetText(member.Instructions)
}

func (f *memberForm) member() protocol.Member {
	model := f.modelID
	if !f.listed {
		model = strings.TrimSpace(f.modelText.Text())
	}
	return protocol.Member{
		ID: f.id, Name: strings.TrimSpace(f.name.Text()), Title: strings.TrimSpace(f.title.Text()), Lead: f.lead.Value,
		Harness: f.harnessID, Model: model, Instructions: strings.TrimSpace(f.instructions.Text()),
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

// the harnesses on this computer, after the one Settings starts agents with; one no longer installed stays so it is not lost
func harnessOptions(snapshot appstate.Snapshot, current string) ([]string, []string) {
	ids, labels := []string{""}, []string{i18n.T("The default harness")}
	found := current == ""
	for _, harness := range env.InstalledHarnesses(snapshot) {
		ids, labels = append(ids, harness.ID), append(labels, harness.Name)
		found = found || harness.ID == current
	}
	if !found {
		ids, labels = append(ids, current), append(labels, i18n.Tf("%s, not installed", harnessName(snapshot, current)))
	}
	return ids, labels
}

func (f *memberForm) harnessField(e *env.Env) layout.Widget {
	return func(gtx layout.Context) layout.Dimensions {
		ids, labels := harnessOptions(e.Snapshot(), f.harnessID)
		selected := 0
		for index, id := range ids {
			if id == f.harnessID {
				selected = index
			}
		}
		f.harness.Label = i18n.T("Harness")
		gtx.Constraints.Min.X = gtx.Constraints.Max.X
		dimensions := f.harness.Layout(gtx, e.Theme, e.Overlays, labels, selected)
		if index, changed := f.harness.Changed(); changed && index >= 0 && index < len(ids) && ids[index] != f.harnessID {
			// model ids belong to one harness, so another harness starts from its own default
			f.harnessID, f.modelID = ids[index], ""
			f.modelText.SetText("")
		}
		return dimensions
	}
}

// the models the app has seen a session of this harness offer, newest list first
func knownModels(snapshot appstate.Snapshot, harness string) []protocol.Choice {
	if harness == "" {
		harness = snapshot.Settings.Agent.Harness
	}
	var newest []protocol.Choice
	var at int64 = -1
	for _, attached := range snapshot.Attached {
		if attached.Model == nil || attached.Info.Harness != harness {
			continue
		}
		options := attached.Model.ModelChoice().Options
		if len(options) > 0 && attached.Info.Updated.UnixNano() > at {
			newest, at = options, attached.Info.Updated.UnixNano()
		}
	}
	return newest
}

func (f *memberForm) modelField(e *env.Env) layout.Widget {
	return func(gtx layout.Context) layout.Dimensions {
		th := e.Theme
		known := knownModels(e.Snapshot(), f.harnessID)
		f.listed = len(known) > 0
		gtx.Constraints.Min.X = gtx.Constraints.Max.X
		if !f.listed {
			f.modelText.Label, f.modelText.Mono = i18n.T("Model"), true
			f.modelText.Placeholder = i18n.T("Model id (empty for the harness default)")
			return f.modelText.Layout(gtx, th.Theme)
		}
		ids := []string{""}
		options := []look.SelectOption{{Label: i18n.T("The harness default")}}
		found := f.modelID == ""
		for _, choice := range known {
			ids = append(ids, choice.ID)
			options = append(options, look.SelectOption{Label: choice.Name, Detail: choice.Description})
			found = found || choice.ID == f.modelID
		}
		if !found {
			ids = append(ids, f.modelID)
			options = append(options, look.SelectOption{Label: f.modelID})
		}
		selected := 0
		for index, id := range ids {
			if id == f.modelID {
				selected = index
			}
		}
		f.model.Label = i18n.T("Model")
		dimensions := f.model.LayoutOptions(gtx, th, e.Overlays, options, selected)
		if index, changed := f.model.Changed(); changed && index >= 0 && index < len(ids) {
			f.modelID = ids[index]
		}
		return dimensions
	}
}
