package settings

import (
	"io"
	"strings"

	"gioui.org/io/clipboard"
	"gioui.org/layout"

	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/kit"
	"ryoku-i18n"

	"ryoku-rashin/internal/app/env"
	"ryoku-rashin/internal/app/look"
	"ryoku-rashin/internal/dictate"
	"ryoku-rashin/internal/protocol"
)

// oniDictation is dictation: its switch, mode and polish route, the
// dictionary, snippets and per-app styles in the dictation store, and a try
// that keeps what did not reach the window with a Copy
type oniDictation struct {
	enabled      kit.Toggle
	mode, polish look.Select

	// the route question: a hosted polish is chosen only past it
	disclose       kit.Dialog
	disclosing     bool
	discloseYes    kit.Plaque
	discloseCancel kit.Plaque

	personal  dictate.Personal
	loaded    bool
	problem   string
	startOver kit.Plaque

	words    personalList
	snippets personalList
	styles   styleList

	// a dictation started here into its own field, and what it came to
	pad       kit.TextField
	try, stop kit.Plaque
	trying    bool
	busy      bool
	result    *protocol.OniDictateResult
	failed    string
	copyRest  kit.Plaque
	clearRest kit.Plaque
}

// personalList edits two-field entries: a dictionary word or a snippet
type personalList struct {
	list          kit.ListEditor
	first, second kit.TextField
	// the entry the editor opened on, so an edit that renames it removes the old one
	editingKey string
	problem    string
}

// styleList edits the per-app styles
type styleList struct {
	list                kit.ListEditor
	app                 kit.TextField
	mode, casing, end   look.Select
	modeAt, casingAt    int
	endAt               int
	editingKey, problem string
}

// forget reads the store again the next time the section shows
func (d *oniDictation) forget() { d.loaded = false }

func (d *oniDictation) load(e *env.Env) {
	if d.loaded || e.Voice.Dictation == nil {
		return
	}
	d.loaded = true
	personal, err := e.Voice.Dictation.Load()
	if err != nil {
		d.problem = err.Error()
		return
	}
	d.personal, d.problem = personal, ""
}

// keep takes the store's answer to an edit: the whole store on success, the
// refusal for the editor on failure
func (d *oniDictation) keep(personal dictate.Personal, err error, problem *string) bool {
	if err != nil {
		*problem = err.Error()
		return false
	}
	d.personal, *problem = personal, ""
	return true
}

var dictationModes = []string{protocol.OniDictateExact, protocol.OniDictateClean, protocol.OniDictatePolished}

func modeLabels() []string {
	return []string{i18n.T("Exact"), i18n.T("Clean"), i18n.T("Polished")}
}

func (p *Page) oniDictationRows(e *env.Env, oni, defaults protocol.OniSettings) []look.Setting {
	th := e.Theme
	d := &p.oni.dictation
	d.load(e)
	dictation, dd := oni.VoiceSettings.Dictation, defaults.VoiceSettings.Dictation
	d.enabled.Value = dictation.Enabled
	off := ""
	if !protocol.EffectiveOni(e.Snapshot().Settings).Master {
		off = i18n.T("Oni is off, so it cannot take dictation.")
	}
	name := i18n.T("Take dictation")
	rows := []look.Setting{
		{Title: name, Detail: i18n.T("Types what you say into the window that had focus when you started, with hyakki oni dictate start and stop. Before every few words Hyakki checks that the window is still there, focused and on the same screen, and stops typing if not."),
			Unavailable: off, Changed: dictation.Enabled != dd.Enabled, Reset: func() { voiceSet(e, map[string]any{"dictation": map[string]any{"enabled": dd.Enabled}}) },
			Control: func(gtx layout.Context) layout.Dimensions {
				return namedSwitch(gtx, th, name, &d.enabled, func(on bool) { voiceSet(e, map[string]any{"dictation": map[string]any{"enabled": on}}) })
			}},
		{Title: i18n.T("Mode"), Detail: i18n.T("Exact types the words as heard. Clean drops filler sounds and turns spoken marks into punctuation. Polished lets a model tidy the Clean text and keeps Clean when it cannot."),
			Changed: dictation.Mode != dd.Mode, Reset: func() { voiceSet(e, map[string]any{"dictation": map[string]any{"mode": dd.Mode}}) },
			Control: func(gtx layout.Context) layout.Dimensions {
				d.mode.Label = i18n.T("Dictation mode")
				gtx.Constraints.Min.X = gtx.Dp(selectWidth)
				dimensions := d.mode.LayoutOptions(gtx, th, e.Overlays, look.Labelled(modeLabels()), indexOf(dictationModes, dictation.Mode))
				if index, ok := d.mode.Changed(); ok && index >= 0 && index < len(dictationModes) {
					voiceSet(e, map[string]any{"dictation": map[string]any{"mode": dictationModes[index]}})
				}
				return dimensions
			}},
		{Title: i18n.T("Polish with"), Detail: i18n.T("Where Polished text is rewritten. On this computer nothing leaves. The Ask route may be a hosted model, and is used only after you agree."),
			Changed: dictation.Polish != dd.Polish, Reset: func() { voiceSet(e, map[string]any{"dictation": map[string]any{"polish": dd.Polish}}) },
			Control: func(gtx layout.Context) layout.Dimensions {
				d.polish.Label = i18n.T("Polish with")
				gtx.Constraints.Min.X = gtx.Dp(selectWidth)
				routes := []string{protocol.OniPolishLocal, protocol.OniPolishRoute}
				options := []look.SelectOption{
					{Label: i18n.T("This computer"), Detail: i18n.T("A local model; nothing leaves")},
					{Label: i18n.T("The Ask route"), Detail: i18n.T("May send the words to a hosted model")},
				}
				dimensions := d.polish.LayoutOptions(gtx, th, e.Overlays, options, indexOf(routes, dictation.Polish))
				if index, ok := d.polish.Changed(); ok && index >= 0 && index < len(routes) && routes[index] != dictation.Polish {
					if routes[index] == protocol.OniPolishRoute {
						d.disclosing, d.disclose.Open, d.disclose.Width = true, true, 560
					} else {
						voiceSet(e, map[string]any{"dictation": map[string]any{"polish": protocol.OniPolishLocal}})
					}
				}
				return dimensions
			}},
	}
	if e.Voice.Dictation == nil {
		return append(rows, d.tryRows(e, dictation)...)
	}
	if d.problem != "" {
		d.startOver.Kind, d.startOver.Small, d.startOver.Label = kit.PlaqueTonal, true, i18n.T("Start over")
		rows = append(rows, look.Setting{Title: i18n.T("The dictation store cannot be read"),
			Detail: d.problem + "\n" + i18n.T("Start over keeps the damaged file beside it and begins an empty store."),
			Control: func(gtx layout.Context) layout.Dimensions {
				if d.startOver.Clicked(gtx) {
					if err := e.Voice.Dictation.Reset(); err != nil {
						d.problem = err.Error()
					} else {
						d.loaded = false
					}
				}
				return d.startOver.Layout(gtx, th.Theme)
			}})
		return append(rows, d.tryRows(e, dictation)...)
	}
	rows = append(rows,
		look.Setting{Title: i18n.T("Dictionary"), Below: true,
			Detail:  i18n.Tf("Words the speech engine gets wrong, typed the way you write them, up to %d. A spoken phrase is at most %d words.", dictate.MaxEntries, dictate.MaxPhraseWords),
			Control: func(gtx layout.Context) layout.Dimensions { return d.layoutWords(gtx, e) }},
		look.Setting{Title: i18n.T("Snippets"), Below: true,
			Detail:  i18n.Tf("Say the trigger alone and the text is typed exactly as kept, never polished, up to %d.", dictate.MaxSnippets),
			Control: func(gtx layout.Context) layout.Dimensions { return d.layoutSnippets(gtx, e) }},
		look.Setting{Title: i18n.T("Per-app styles"), Below: true,
			Detail:  i18n.T("How dictation types into one app: its own mode, casing and ending. An app named with its own mode overrides the mode above, so a terminal can stay Exact."),
			Control: func(gtx layout.Context) layout.Dimensions { return d.layoutStyles(gtx, e) }},
	)
	return append(rows, d.tryRows(e, dictation)...)
}

// layoutPersonal draws one two-field list over the store's entries
func (d *oniDictation) layoutPersonal(gtx layout.Context, e *env.Env, list *personalList, items []kit.ListItem, keys []string, labels [2]string,
	opened func(index int), save func(first, second string) (dictate.Personal, error), remove func(key string) (dictate.Personal, error)) layout.Dimensions {
	th := e.Theme
	if index, ok := list.list.Opened(); ok {
		opened(index)
	}
	if list.list.Added() || func() bool { _, ok := list.list.Edited(); return ok }() {
		first, second := strings.TrimSpace(list.first.Text()), list.second.Text()
		personal, err := save(first, second)
		if d.keep(personal, err, &list.problem) {
			if list.editingKey != "" && !strings.EqualFold(list.editingKey, first) {
				personal, err := remove(list.editingKey)
				d.keep(personal, err, &list.problem)
			}
		} else {
			// the refusal stays with what was typed, for the user to fix
			list.list.Edit(editorIndex(keys, list.editingKey))
			list.list.Opened()
		}
	}
	if index, ok := list.list.Removed(); ok && index >= 0 && index < len(keys) {
		personal, err := remove(keys[index])
		d.keep(personal, err, &list.problem)
	}
	if list.list.Cancelled() {
		list.problem = ""
	}
	list.list.EditorValid = strings.TrimSpace(list.first.Text()) != "" && strings.TrimSpace(list.second.Text()) != ""
	list.list.EditorFields = list.list.EditorFields[:0]
	list.list.EditorFields = append(list.list.EditorFields, &list.first.Editor)
	list.first.Label, list.second.Label = labels[0], labels[1]
	list.first.Editor.SingleLine = true
	list.second.Error = list.problem
	return list.list.Layout(gtx, th.Theme, items, func(gtx layout.Context, _ int) layout.Dimensions {
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			layout.Rigid(func(gtx layout.Context) layout.Dimensions { return list.first.Layout(gtx, th.Theme) }),
			layout.Rigid(layout.Spacer{Height: th.Sp.S2}.Layout),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions { return list.second.Layout(gtx, th.Theme) }),
		)
	}, nil)
}

// editorIndex is the list index of key, kit.ListEditorNew for a new entry
func editorIndex(keys []string, key string) int {
	if key == "" {
		return kit.ListEditorNew
	}
	for index, each := range keys {
		if strings.EqualFold(each, key) {
			return index
		}
	}
	return kit.ListEditorNew
}

func (d *oniDictation) layoutWords(gtx layout.Context, e *env.Env) layout.Dimensions {
	store := e.Voice.Dictation
	list := &d.words
	list.list.Label, list.list.AddText, list.list.DoneText, list.list.EmptyText = i18n.T("Dictionary"), i18n.T("Add a word"), i18n.T("Add"), i18n.T("No words yet.")
	entries := d.personal.Dictionary
	items, keys := make([]kit.ListItem, len(entries)), make([]string, len(entries))
	for index, entry := range entries {
		items[index] = kit.ListItem{Title: entry.Spoken, Detail: i18n.Tf("Typed as %s", entry.Written)}
		keys[index] = entry.Spoken
	}
	return d.layoutPersonal(gtx, e, list, items, keys, [2]string{i18n.T("You say"), i18n.T("Hyakki types")},
		func(index int) {
			list.problem, list.editingKey = "", ""
			list.first.SetText("")
			list.second.SetText("")
			if index >= 0 && index < len(entries) {
				list.editingKey = entries[index].Spoken
				list.first.SetText(entries[index].Spoken)
				list.second.SetText(entries[index].Written)
			}
		},
		func(first, second string) (dictate.Personal, error) { return store.SetWord(first, second) },
		store.RemoveWord)
}

func (d *oniDictation) layoutSnippets(gtx layout.Context, e *env.Env) layout.Dimensions {
	store := e.Voice.Dictation
	list := &d.snippets
	list.list.Label, list.list.AddText, list.list.DoneText, list.list.EmptyText = i18n.T("Snippets"), i18n.T("Add a snippet"), i18n.T("Add"), i18n.T("No snippets yet.")
	snippets := d.personal.Snippets
	items, keys := make([]kit.ListItem, len(snippets)), make([]string, len(snippets))
	for index, snippet := range snippets {
		first, _, _ := strings.Cut(snippet.Text, "\n")
		items[index] = kit.ListItem{Title: snippet.Trigger, Detail: first}
		keys[index] = snippet.Trigger
	}
	return d.layoutPersonal(gtx, e, list, items, keys, [2]string{i18n.T("You say"), i18n.T("Hyakki types")},
		func(index int) {
			list.problem, list.editingKey = "", ""
			list.first.SetText("")
			list.second.SetText("")
			if index >= 0 && index < len(snippets) {
				list.editingKey = snippets[index].Trigger
				list.first.SetText(snippets[index].Trigger)
				list.second.SetText(snippets[index].Text)
			}
		},
		func(first, second string) (dictate.Personal, error) { return store.SetSnippet(first, second) },
		store.RemoveSnippet)
}

var (
	styleModes = []string{"", protocol.OniDictateExact, protocol.OniDictateClean, protocol.OniDictatePolished}
	// the first casing and ending are the store's defaults, what an empty field means
	styleCasings = []string{dictate.CasingSentence, dictate.CasingKeep, dictate.CasingLower}
	styleEnds    = []string{dictate.EndKeep, dictate.EndPeriod, dictate.EndNone}
)

func styleDetail(style dictate.Style) string {
	mode, casing, end := i18n.T("the mode above"), i18n.T("sentence case"), i18n.T("the ending as heard")
	if index := indexOf(styleModes, style.Mode); index > 0 {
		mode = modeLabels()[index-1]
	}
	switch style.Casing {
	case dictate.CasingKeep:
		casing = i18n.T("casing as heard")
	case dictate.CasingLower:
		casing = i18n.T("lower case")
	}
	switch style.End {
	case dictate.EndPeriod:
		end = i18n.T("a full stop")
	case dictate.EndNone:
		end = i18n.T("no closing mark")
	}
	return i18n.Tf("%s, %s, %s", mode, casing, end)
}

func (d *oniDictation) layoutStyles(gtx layout.Context, e *env.Env) layout.Dimensions {
	th := e.Theme
	store := e.Voice.Dictation
	list := &d.styles
	list.list.Label, list.list.AddText, list.list.DoneText, list.list.EmptyText = i18n.T("Per-app styles"), i18n.T("Add an app"), i18n.T("Add"), i18n.T("Every app types in the mode above.")
	styles := d.personal.Styles
	items, keys := make([]kit.ListItem, len(styles)), make([]string, len(styles))
	for index, style := range styles {
		items[index] = kit.ListItem{Title: style.App, Detail: styleDetail(style)}
		keys[index] = style.App
	}
	if index, ok := list.list.Opened(); ok {
		list.problem, list.editingKey = "", ""
		list.app.SetText("")
		list.modeAt, list.casingAt, list.endAt = 0, 0, 0
		if index >= 0 && index < len(styles) {
			style := styles[index]
			list.editingKey = style.App
			list.app.SetText(style.App)
			list.modeAt, list.casingAt, list.endAt = indexOf(styleModes, style.Mode), indexOf(styleCasings, style.Casing), indexOf(styleEnds, style.End)
		}
	}
	if list.list.Added() || func() bool { _, ok := list.list.Edited(); return ok }() {
		app := strings.TrimSpace(list.app.Text())
		personal, err := store.SetStyle(dictate.Style{App: app, Mode: styleModes[list.modeAt], Casing: styleCasings[list.casingAt], End: styleEnds[list.endAt]})
		if d.keep(personal, err, &list.problem) {
			if list.editingKey != "" && list.editingKey != app {
				personal, err := store.RemoveStyle(list.editingKey)
				d.keep(personal, err, &list.problem)
			}
		} else {
			list.list.Edit(editorIndex(keys, list.editingKey))
			list.list.Opened()
		}
	}
	if index, ok := list.list.Removed(); ok && index >= 0 && index < len(keys) {
		personal, err := store.RemoveStyle(keys[index])
		d.keep(personal, err, &list.problem)
	}
	if list.list.Cancelled() {
		list.problem = ""
	}
	list.list.EditorValid = strings.TrimSpace(list.app.Text()) != ""
	list.list.EditorFields = append(list.list.EditorFields[:0], &list.app.Editor)
	list.app.Label, list.app.Placeholder, list.app.Mono, list.app.Error = i18n.T("App id"), "org.gnome.Console", true, list.problem
	list.app.Editor.SingleLine = true
	choice := func(gtx layout.Context, selection *look.Select, label string, labels []string, at *int) layout.Dimensions {
		selection.Label = label
		dimensions := selection.LayoutOptions(gtx, th, e.Overlays, look.Labelled(labels), *at)
		if index, ok := selection.Changed(); ok && index >= 0 && index < len(labels) {
			*at = index
		}
		return dimensions
	}
	return list.list.Layout(gtx, th.Theme, items, func(gtx layout.Context, _ int) layout.Dimensions {
		gap := layout.Rigid(layout.Spacer{Height: th.Sp.S2}.Layout)
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			layout.Rigid(func(gtx layout.Context) layout.Dimensions { return list.app.Layout(gtx, th.Theme) }),
			gap,
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return choice(gtx, &list.mode, i18n.T("Mode"), append([]string{i18n.T("The mode above")}, modeLabels()...), &list.modeAt)
			}),
			gap,
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return choice(gtx, &list.casing, i18n.T("Casing"), []string{i18n.T("Sentence case"), i18n.T("As heard"), i18n.T("Lower case")}, &list.casingAt)
			}),
			gap,
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return choice(gtx, &list.end, i18n.T("Ending"), []string{i18n.T("As heard"), i18n.T("A full stop"), i18n.T("No closing mark")}, &list.endAt)
			}),
		)
	}, nil)
}

// tryRows start and stop one dictation from here, and keep what did not
// reach the window, with a Copy
func (d *oniDictation) tryRows(e *env.Env, dictation protocol.OniDictationSettings) []look.Setting {
	th := e.Theme
	unavailable := ""
	if !dictation.Enabled || !protocol.EffectiveOni(e.Snapshot().Settings).Master {
		unavailable = i18n.T("Turn on Take dictation first.")
	}
	rows := []look.Setting{{Title: i18n.T("Try dictation"), Unavailable: unavailable, Below: true,
		Detail: i18n.T("Start, speak, then stop: Hyakki types into the field below. If this window closes or loses focus first, the text that was not typed stays here."),
		Control: func(gtx layout.Context) layout.Dimensions {
			d.try.Kind, d.try.Small, d.try.Label, d.try.Busy = kit.PlaqueTonal, true, i18n.T("Start dictation"), d.busy && !d.trying
			d.stop.Kind, d.stop.Small, d.stop.Label, d.stop.Busy = kit.PlaqueGold, true, i18n.T("Stop dictation"), d.busy && d.trying
			if d.try.Clicked(gtx) && !d.busy && !d.trying {
				// the daemon types into the window focused at start, so the field takes the keyboard first
				d.pad.Focus(gtx)
				d.call(e, protocol.OniVoiceStart)
			}
			if d.stop.Clicked(gtx) && !d.busy && d.trying {
				d.call(e, protocol.OniVoiceStop)
			}
			d.pad.Label, d.pad.Placeholder = i18n.T("Dictated text"), i18n.T("What you say is typed here")
			return layout.Flex{Alignment: layout.End}.Layout(gtx,
				layout.Flexed(1, func(gtx layout.Context) layout.Dimensions { return d.pad.Layout(gtx, th.Theme) }),
				layout.Rigid(layout.Spacer{Width: th.Sp.S2}.Layout),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					if d.trying {
						return d.stop.Layout(gtx, th.Theme)
					}
					return d.try.Layout(gtx, th.Theme)
				}),
			)
		}}}
	switch {
	case d.failed != "":
		rows = append(rows, look.Setting{Title: i18n.T("Dictation did not run"), Detail: d.failed})
	case d.result != nil && d.result.Rest != "":
		rest := d.result.Rest
		d.copyRest.Kind, d.copyRest.Small, d.copyRest.Label = kit.PlaqueGold, true, i18n.T("Copy")
		d.clearRest.Kind, d.clearRest.Small, d.clearRest.Label = kit.PlaqueQuiet, true, i18n.T("Clear")
		rows = append(rows, look.Setting{Title: i18n.T("This text stayed here"), Below: true,
			Detail: restReason(*d.result),
			Control: func(gtx layout.Context) layout.Dimensions {
				if d.copyRest.Clicked(gtx) {
					gtx.Execute(clipboard.WriteCmd{Type: "text/plain", Data: io.NopCloser(strings.NewReader(rest))})
					if e.Notify != nil {
						e.Notify(i18n.T("Copied the text that stayed here."))
					}
				}
				if d.clearRest.Clicked(gtx) {
					d.result = nil
				}
				return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return look.Named(gtx, i18n.T("Text that stayed here"), look.Paragraph(th, th.T.Body, th.R.Fg, strings.TrimSpace(rest)))
					}),
					layout.Rigid(layout.Spacer{Height: th.Sp.S2}.Layout),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
							layout.Rigid(func(gtx layout.Context) layout.Dimensions { return d.copyRest.Layout(gtx, th.Theme) }),
							layout.Rigid(layout.Spacer{Width: th.Sp.S2}.Layout),
							layout.Rigid(func(gtx layout.Context) layout.Dimensions { return d.clearRest.Layout(gtx, th.Theme) }),
						)
					}),
				)
			}})
	case d.result != nil && d.result.Mode != "":
		rows = append(rows, look.Setting{Title: i18n.T("Dictation typed everything"), Detail: dictatedNote(*d.result)})
	}
	return rows
}

func (d *oniDictation) call(e *env.Env, action string) {
	d.busy, d.failed = true, ""
	result := new(protocol.OniDictateResult)
	e.Call(protocol.MOniDictate, protocol.OniDictateParams{Action: action}, result, func(err error) {
		d.busy = false
		if err != nil {
			d.trying, d.failed = false, err.Error()
			return
		}
		d.trying = action == protocol.OniVoiceStart
		if action == protocol.OniVoiceStart {
			d.result = nil
			return
		}
		d.result = result
	})
}

// restReason says why the rest did not reach the window, and how the words
// were typed
func restReason(result protocol.OniDictateResult) string {
	why := i18n.T("Hyakki stopped typing.")
	switch result.Refused {
	case "closed":
		why = i18n.T("The window closed before all of it was typed.")
	case "unfocused":
		why = i18n.T("Another window took focus before all of it was typed.")
	case "moved-output":
		why = i18n.T("The window moved to another screen before all of it was typed.")
	case "hidden":
		why = i18n.T("The window was hidden before all of it was typed.")
	case "unavailable":
		why = i18n.T("Text cannot be typed here: wtype is missing or the desktop refuses it.")
	}
	return why + " " + dictatedNote(result)
}

func dictatedNote(result protocol.OniDictateResult) string {
	note := i18n.Tf("%d characters reached the window.", result.Landed)
	if result.Unsure > 0 {
		note += " " + i18n.Tf("%d more may have; check the window before pasting.", result.Unsure)
	}
	if result.Fallback != "" {
		note += " " + i18n.Tf("Polishing did not run (%s), so it was typed as Clean.", result.Fallback)
	}
	return note
}

// overlay is the hosted-polish question
func (d *oniDictation) overlay(gtx layout.Context, e *env.Env) bool {
	if !d.disclose.Open && !d.disclosing {
		return false
	}
	th := e.Theme
	d.discloseCancel.Kind, d.discloseCancel.Label = kit.PlaqueQuiet, i18n.T("Cancel")
	d.discloseYes.Kind, d.discloseYes.Label = kit.PlaqueGold, i18n.T("Use the Ask route")
	if d.discloseCancel.Clicked(gtx) {
		d.disclose.Dismiss(gtx)
	}
	if d.discloseYes.Clicked(gtx) && d.disclose.Open {
		voiceSet(e, map[string]any{"dictation": map[string]any{"polish": protocol.OniPolishRoute}})
		d.disclose.Dismiss(gtx)
	}
	route := e.Snapshot().Settings.Ask.Model
	if route == "" {
		route = i18n.T("the Ask model")
	}
	d.disclose.Layout(gtx, th.Theme, i18n.T("Polish on the Ask route?"), func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			layout.Rigid(look.Paragraph(th, th.T.Row, th.R.Fg, i18n.Tf("Polished dictation will be rewritten by %s. If that is a hosted model, the words you dictate in Polished leave this computer each time, as a typed question does.", route))),
			layout.Rigid(layout.Spacer{Height: th.Sp.S3}.Layout),
			layout.Rigid(look.Paragraph(th, th.T.Detail, th.R.Fg2, i18n.T("Text that looks like a password, code or card number, or that is dictated into a password manager, is never sent: it is typed as Clean instead. Exact and Clean never leave this computer."))),
		)
	},
		func(gtx layout.Context) layout.Dimensions { return d.discloseCancel.Layout(gtx, th.Theme) },
		func(gtx layout.Context) layout.Dimensions { return d.discloseYes.Layout(gtx, th.Theme) },
	)
	if d.disclose.Closed() {
		d.disclosing = false
	}
	return d.disclose.Open
}
