package memoryui

import (
	"io"
	"strings"

	"gioui.org/io/clipboard"
	"gioui.org/layout"

	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/kit"
	"ryoku-i18n"

	"ryoku-rashin/internal/app/env"
	"ryoku-rashin/internal/app/look"
	"ryoku-rashin/internal/protocol"
)

// ForgetAll is the typed confirm that forgets everything, or one scope.
type ForgetAll struct {
	open    bool
	scope   string
	confirm kit.Confirm
}

// Open asks to forget scope, every memory when it is empty.
func (f *ForgetAll) Open(scope string) {
	f.open, f.scope = true, scope
	f.confirm.Dialog.Open, f.confirm.Danger = true, true
	f.confirm.Dialog.Width = 520
}

// Layout draws the dialog while it is open and reports whether it is.
func (f *ForgetAll) Layout(gtx layout.Context, e *env.Env) bool {
	if !f.open {
		return false
	}
	snapshot := e.Snapshot()
	var status protocol.MemoryStatus
	if snapshot.Memory != nil {
		status = *snapshot.Memory
	}
	title, body := forgetWords(status, f.scope, snapshot.Settings.Memory)
	scope, action := f.scope, i18n.T("Forget everything")
	if scope != "" {
		action = i18n.T("Forget these memories")
	}
	if f.confirm.Layout(gtx, e.Theme.Theme, title, body, action, i18n.T("forget")) {
		e.ChangeMemory(protocol.MMemoryForgetAll, protocol.MemoryForgetAllParams{Scope: scope}, func(change protocol.MemoryChange, err error) {
			if err != nil {
				e.Report(err)
				return
			}
			e.Notify(ForgotWords(len(change.Items)))
		})
	}
	if f.confirm.Closed() {
		f.open = false
	}
	return f.confirm.Dialog.Open
}

func forgetWords(status protocol.MemoryStatus, scope string, settings protocol.MemorySettings) (title, body string) {
	counts := status.Counts
	if scope != "" {
		name, count := scope, 0
		for _, known := range status.Scopes {
			if known.ID == scope {
				name, count = known.Name, known.Count
			}
		}
		if scope == protocol.MemoryYou {
			name = i18n.T("you")
		}
		title = i18n.Tf("Forget everything about %s?", name)
		body = i18n.Tf("This forgets all %s memories about %s, with their earlier versions and suggestions. It can't be undone.", look.Count(count), name)
	} else {
		projects, teams := 0, 0
		for _, known := range status.Scopes {
			switch {
			case strings.HasPrefix(known.ID, protocol.MemoryProjectPrefix):
				projects++
			case strings.HasPrefix(known.ID, protocol.MemoryTeamPrefix):
				teams++
			}
		}
		title = i18n.T("Forget everything?")
		body = i18n.Tf("This forgets all %s memories: %s about you, %s in %s projects and %s in %s teams, with their earlier versions, suggestions and the summaries Rashin kept of conversations. It can't be undone. The conversations themselves stay; turn off Learn from conversations too if Rashin shouldn't learn from them again.",
			look.Count(counts.All), look.Count(counts.You), look.Count(counts.Projects), look.Count(projects), look.Count(counts.Teams), look.Count(teams))
	}
	if settings.Engine != protocol.MemoryBuiltin && settings.Engine != "" {
		body += "\n\n" + i18n.Tf("%s keeps its own copy. Forget there too.", EngineName(settings.Engine))
	}
	return title, body
}

// ForgotWords is the placard after count memories were forgotten.
func ForgotWords(count int) string {
	if count == 1 {
		return i18n.T("Forgot one memory.")
	}
	return i18n.Tf("Forgot %s memories.", look.Count(count))
}

// EngineName names what keeps memory: Built in, or the engine's own name.
func EngineName(engine string) string {
	switch engine {
	case protocol.MemoryHindsight:
		return i18n.T("Hindsight")
	case protocol.MemoryHoncho:
		return i18n.T("Honcho")
	}
	return i18n.T("Built in")
}

// the request Rashin hands the reader for the other assistant; original wording
func importRequest() string {
	return i18n.T("List every fact you remember about me and every instruction I gave you about how to answer. Put each on its own line with the date you learned it, if you know it. Use my own words where you can, and leave nothing out.")
}

// Import is the dialog that brings what another assistant remembers in as
// suggestions.
type Import struct {
	open   bool
	scope  string
	dialog kit.Dialog
	pasted look.MultiLine
	where  look.Select
	copy   kit.Plaque

	cancel, run kit.Plaque
	busy        bool
	problem     string
}

// Open shows the dialog with Applies to set to scope, About you when empty.
func (d *Import) Open(scope string) {
	if scope == "" {
		scope = protocol.MemoryYou
	}
	d.open, d.scope, d.problem, d.busy = true, scope, "", false
	d.dialog.Open, d.dialog.Width = true, 520
	d.pasted.SetText("")
}

// Layout draws the dialog while it is open and reports whether it is.
func (d *Import) Layout(gtx layout.Context, e *env.Env) bool {
	if !d.open {
		return false
	}
	th := e.Theme
	snapshot := e.Snapshot()
	var scopes []protocol.MemoryScope
	if snapshot.Memory != nil {
		scopes = snapshot.Memory.Scopes
	}
	options := ScopeOptions(scopes, d.scope)
	if picked, ok := d.where.Changed(); ok && picked < len(options) {
		d.scope = options[picked].ID
	}
	if d.copy.Clicked(gtx) {
		gtx.Execute(clipboard.WriteCmd{Type: "text/plain", Data: io.NopCloser(strings.NewReader(importRequest()))})
		e.Notify(i18n.T("Copied the request."))
	}
	if d.cancel.Clicked(gtx) {
		d.dialog.Dismiss(gtx)
	}
	if d.run.Clicked(gtx) && !d.busy {
		d.submit(e)
	}
	d.copy.Kind, d.copy.Small, d.copy.Label = kit.PlaqueTonal, true, i18n.T("Copy")
	d.cancel.Kind, d.cancel.Label = kit.PlaqueQuiet, i18n.T("Cancel")
	d.run.Kind, d.run.Label, d.run.Busy = kit.PlaqueGold, i18n.T("Import"), d.busy
	d.pasted.Label, d.pasted.Lines, d.pasted.MaxLines = i18n.T("Its answer"), 8, 16
	d.where.Label = i18n.T("Applies to")
	labels := make([]look.SelectOption, len(options))
	selected := 0
	for index, scope := range options {
		labels[index].Label = scope.Name
		if scope.ID == d.scope {
			selected = index
		}
	}
	step := func(text string) layout.FlexChild {
		return layout.Rigid(look.Paragraph(th, th.T.Body, th.R.Fg, text))
	}
	gap := func() layout.FlexChild { return layout.Rigid(layout.Spacer{Height: th.Sp.S3}.Layout) }
	body := func(gtx layout.Context) layout.Dimensions {
		children := []layout.FlexChild{
			step(i18n.T("1. Ask the other assistant to list everything it remembers about you.")),
			gap(),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				gtx.Constraints.Min.X = gtx.Constraints.Max.X
				return kit.SlabContent(gtx, th.Theme, func(gtx layout.Context) layout.Dimensions {
					return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
						layout.Rigid(look.Paragraph(th, th.T.Body, th.R.Fg2, importRequest())),
						layout.Rigid(layout.Spacer{Height: th.Sp.S2}.Layout),
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							return look.Named(gtx, i18n.T("Copy the request"), func(gtx layout.Context) layout.Dimensions { return d.copy.Layout(gtx, th.Theme) })
						}),
					)
				})
			}),
			gap(),
			step(i18n.T("2. Paste its answer here.")),
			gap(),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions { return d.pasted.Layout(gtx, th) }),
		}
		if d.problem != "" {
			children = append(children,
				layout.Rigid(layout.Spacer{Height: th.Sp.S1}.Layout),
				layout.Rigid(look.Paragraph(th, th.T.Detail, th.R.Jasper, d.problem)),
			)
		}
		children = append(children,
			gap(),
			step(i18n.T("3. Rashin splits it into separate memories for you to check. Nothing is used until you keep it.")),
			gap(),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				gtx.Constraints.Max.X = min(gtx.Constraints.Max.X, gtx.Dp(look.SettingsControl))
				return d.where.LayoutOptions(gtx, th, e.Overlays, labels, selected)
			}),
		)
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
	}
	d.dialog.Layout(gtx, th.Theme, i18n.T("Import from another assistant"), body,
		func(gtx layout.Context) layout.Dimensions { return d.cancel.Layout(gtx, th.Theme) },
		func(gtx layout.Context) layout.Dimensions {
			if strings.TrimSpace(d.pasted.Text()) == "" {
				gtx = gtx.Disabled()
			}
			return d.run.Layout(gtx, th.Theme)
		},
	)
	if d.dialog.Closed() {
		d.open = false
	}
	return d.dialog.Open
}

func (d *Import) submit(e *env.Env) {
	text := strings.TrimSpace(d.pasted.Text())
	if text == "" {
		return
	}
	d.busy, d.problem = true, ""
	e.ChangeMemory(protocol.MMemoryImport, protocol.MemoryImportParams{Text: text, Scope: d.scope}, func(change protocol.MemoryChange, err error) {
		d.busy = false
		if err != nil {
			d.problem = err.Error()
			return
		}
		d.open, d.dialog.Open = false, false
		count := len(change.Items)
		text := i18n.Tf("%s suggestions wait in Suggested.", look.CountStart(count))
		if count == 1 {
			text = i18n.T("One suggestion waits in Suggested.")
		}
		if e.NotifyAction == nil || e.OpenMemory == nil {
			e.Notify(text)
			return
		}
		e.NotifyAction(text, i18n.T("Show"), func() { e.OpenMemory(env.MemoryPlace{Filter: env.MemorySuggested}) })
	})
}

// SheetBody lays out a read-only sheet: its title, a proof line, the text it
// shows, a line about it in fg3, and its acts in a row under them.
func SheetBody(gtx layout.Context, th *look.Theme, title, proof, text, note string, acts ...layout.Widget) layout.Dimensions {
	children := []layout.FlexChild{layout.Rigid(look.Paragraph(th, th.T.Title, th.R.Fg, title))}
	if proof != "" {
		children = append(children,
			layout.Rigid(layout.Spacer{Height: th.Sp.S1}.Layout),
			layout.Rigid(look.Paragraph(th, th.T.Detail, th.R.Fg2, proof)),
		)
	}
	children = append(children,
		layout.Rigid(layout.Spacer{Height: th.Sp.S4}.Layout),
		layout.Rigid(look.Paragraph(th, th.T.Body, th.R.Fg, text)),
	)
	if note != "" {
		children = append(children,
			layout.Rigid(layout.Spacer{Height: th.Sp.S4}.Layout),
			layout.Rigid(look.Paragraph(th, th.T.Detail, th.R.Fg3, note)),
		)
	}
	if len(acts) > 0 {
		row := make([]layout.FlexChild, 0, 2*len(acts))
		for index, act := range acts {
			if index > 0 {
				row = append(row, layout.Rigid(layout.Spacer{Width: th.Sp.S2}.Layout))
			}
			row = append(row, layout.Rigid(act))
		}
		children = append(children,
			layout.Rigid(layout.Spacer{Height: th.Sp.S4}.Layout),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx, row...)
			}),
		)
	}
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
}
