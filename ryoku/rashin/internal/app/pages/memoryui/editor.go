package memoryui

import (
	"slices"
	"strconv"
	"strings"
	"unicode/utf8"

	"gioui.org/io/key"
	"gioui.org/layout"

	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/kit"
	"ryoku-i18n"

	"ryoku-rashin/internal/app/env"
	"ryoku-rashin/internal/app/look"
	"ryoku-rashin/internal/protocol"
)

// TextProblem is what is wrong with a memory's text before it is sent, or
// "": empty, or longer than the engine keeps.
func TextProblem(text string) string {
	text = strings.TrimSpace(text)
	switch length := utf8.RuneCountInString(text); {
	case length == 0:
		return i18n.T("A memory can't be empty.")
	case length > protocol.MemoryTextMax:
		return i18n.Tf("That's %d characters. Keep it under %d or split it in two.", length, protocol.MemoryTextMax)
	}
	return ""
}

// Editor is the slab a memory unfolds into: its text, where it applies, its
// pin, where it came from and its earlier versions. Opened blank it adds one.
type Editor struct {
	open    bool
	memory  protocol.Memory
	text    look.MultiLine
	scope   look.Select
	scopeID string
	pin     kit.Toggle
	problem string
	busy    bool

	showVersions bool
	versions     kit.Plaque
	restores     []kit.Plaque

	save, cancel, forget kit.Plaque
}

// Open unfolds the editor over memory.
func (ed *Editor) Open(e *env.Env, memory protocol.Memory) {
	ed.reset(memory)
	// the list carries no earlier versions, so they are asked for once
	id := memory.ID
	e.GetMemory(id, func(full protocol.Memory, err error) {
		if err == nil && ed.open && ed.memory.ID == id {
			ed.memory.Versions = full.Versions
		}
	})
}

// OpenNew opens a blank editor that remembers in scope.
func (ed *Editor) OpenNew(scope string) {
	if scope == "" {
		scope = protocol.MemoryYou
	}
	ed.reset(protocol.Memory{Scope: scope})
}

func (ed *Editor) reset(memory protocol.Memory) {
	ed.open, ed.memory, ed.problem, ed.busy, ed.showVersions = true, memory, "", false, false
	ed.text.SetText(memory.Text)
	ed.text.Focus()
	ed.scopeID, ed.pin.Value = memory.Scope, memory.Pinned
}

// Close folds the editor away, dropping what was typed.
func (ed *Editor) Close() { ed.open = false }

// IsOpen says whether the editor is unfolded.
func (ed *Editor) IsOpen() bool { return ed.open }

// ID is the memory being edited, "" for a new one.
func (ed *Editor) ID() string { return ed.memory.ID }

// ScopeOptions are the scopes a memory may apply to, About you first, then
// projects, then teams, with current among them.
func ScopeOptions(scopes []protocol.MemoryScope, current string) []protocol.MemoryScope {
	options := []protocol.MemoryScope{{ID: protocol.MemoryYou, Name: i18n.T("About you")}}
	for _, prefix := range []string{protocol.MemoryProjectPrefix, protocol.MemoryTeamPrefix} {
		for _, scope := range scopes {
			if strings.HasPrefix(scope.ID, prefix) {
				options = append(options, scope)
			}
		}
	}
	if current != "" && !slices.ContainsFunc(options, func(scope protocol.MemoryScope) bool { return scope.ID == current }) {
		options = append(options, protocol.MemoryScope{ID: current, Name: current})
	}
	return options
}

func (ed *Editor) handle(gtx layout.Context, e *env.Env, options []protocol.MemoryScope) {
	for {
		event, ok := gtx.Event(key.Filter{Focus: &ed.text.Editor, Name: key.NameEscape})
		if !ok {
			break
		}
		if pressed, ok := event.(key.Event); ok && pressed.State == key.Press {
			ed.open = false
		}
	}
	if picked, ok := ed.scope.Changed(); ok && picked < len(options) {
		ed.scopeID = options[picked].ID
	}
	if ed.cancel.Clicked(gtx) {
		ed.open = false
	}
	if ed.forget.Clicked(gtx) && ed.memory.ID != "" {
		e.ForgetMemory(ed.memory)
		ed.open = false
	}
	if ed.versions.Clicked(gtx) {
		ed.showVersions = !ed.showVersions
	}
	for index := range ed.restores {
		if ed.restores[index].Clicked(gtx) {
			Restore(e, ed.memory.ID, index+1)
			ed.open = false
		}
	}
	if (ed.save.Clicked(gtx) || ed.text.Submitted(gtx)) && !ed.busy {
		ed.submit(e)
	}
}

func (ed *Editor) submit(e *env.Env) {
	text := strings.TrimSpace(ed.text.Text())
	if ed.problem = TextProblem(text); ed.problem != "" {
		return
	}
	done := func(said string) func(protocol.MemoryChange, error) {
		return func(change protocol.MemoryChange, err error) {
			ed.busy = false
			if err != nil {
				ed.problem = err.Error()
				return
			}
			ed.open = false
			e.NotifyChange(said, change)
		}
	}
	if ed.memory.ID == "" {
		ed.busy = true
		e.ChangeMemory(protocol.MMemoryAdd, protocol.MemoryAddParams{Scope: ed.scopeID, Text: text, Pinned: ed.pin.Value}, done(i18n.T("Remembered.")))
		return
	}
	params := protocol.MemoryEditParams{ID: ed.memory.ID}
	if text != ed.memory.Text {
		params.Text = &text
	}
	if ed.scopeID != ed.memory.Scope {
		params.Scope = &ed.scopeID
	}
	if ed.pin.Value != ed.memory.Pinned {
		pinned := ed.pin.Value
		params.Pinned = &pinned
	}
	if params.Text == nil && params.Scope == nil && params.Pinned == nil {
		ed.open = false
		return
	}
	ed.busy = true
	e.ChangeMemory(protocol.MMemoryEdit, params, done(i18n.T("Memory updated.")))
}

// Layout draws the editor as a slab the width it is given.
func (ed *Editor) Layout(gtx layout.Context, e *env.Env, scopes []protocol.MemoryScope, caps protocol.MemoryCaps) layout.Dimensions {
	th := e.Theme
	options := ScopeOptions(scopes, ed.scopeID)
	ed.handle(gtx, e, options)
	if !ed.open {
		return layout.Dimensions{}
	}
	ed.text.Label = i18n.T("Memory")
	ed.text.Help = i18n.T("Write it as a fact, for example \"Prefers fish as the login shell\".")
	ed.text.Lines, ed.text.MaxLines = 2, 4
	ed.scope.Label = i18n.T("Applies to")
	labels := make([]look.SelectOption, len(options))
	selected := 0
	for index, scope := range options {
		labels[index].Label = scope.Name
		if scope.ID == ed.scopeID {
			selected = index
		}
	}
	gtx.Constraints.Min.X = gtx.Constraints.Max.X
	return kit.SlabContent(gtx, th.Theme, func(gtx layout.Context) layout.Dimensions {
		gtx.Constraints.Min.X = gtx.Constraints.Max.X
		children := []layout.FlexChild{
			layout.Rigid(func(gtx layout.Context) layout.Dimensions { return ed.text.Layout(gtx, th) }),
		}
		if ed.problem != "" {
			children = append(children,
				layout.Rigid(layout.Spacer{Height: th.Sp.S1}.Layout),
				layout.Rigid(look.Paragraph(th, th.T.Detail, th.R.Jasper, ed.problem)),
			)
		}
		children = append(children,
			layout.Rigid(layout.Spacer{Height: th.Sp.S4}.Layout),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				gtx.Constraints.Max.X = min(gtx.Constraints.Max.X, gtx.Dp(look.SettingsControl))
				return ed.scope.LayoutOptions(gtx, th, e.Overlays, labels, selected)
			}),
			layout.Rigid(layout.Spacer{Height: th.Sp.S4}.Layout),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions { return ed.layoutPin(gtx, th, caps) }),
		)
		if ed.memory.ID != "" {
			children = append(children,
				layout.Rigid(layout.Spacer{Height: th.Sp.S4}.Layout),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions { return ed.layoutFacts(gtx, e, caps) }),
			)
			if ed.showVersions && caps.Versions {
				children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions { return ed.layoutVersions(gtx, th) }))
			}
		}
		children = append(children,
			layout.Rigid(layout.Spacer{Height: th.Sp.S4}.Layout),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions { return ed.layoutActs(gtx, th) }),
		)
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
	})
}

func (ed *Editor) layoutPin(gtx layout.Context, th *look.Theme, caps protocol.MemoryCaps) layout.Dimensions {
	detail := i18n.T("Pinned memories are never replaced or faded. A newer fact is suggested to you instead.")
	if !caps.Pin {
		detail = i18n.T("This engine can't pin memories.")
	}
	toggle := func(gtx layout.Context) layout.Dimensions {
		if !caps.Pin {
			gtx = gtx.Disabled()
		}
		return look.Named(gtx, i18n.T("Pin this memory"), func(gtx layout.Context) layout.Dimensions { return ed.pin.Layout(gtx, th.Theme) })
	}
	return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
		layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
				layout.Rigid(look.Paragraph(th, th.T.Row, th.R.Fg, i18n.T("Pin"))),
				layout.Rigid(layout.Spacer{Height: th.Sp.S1}.Layout),
				layout.Rigid(look.Paragraph(th, th.T.Detail, th.R.Fg2, detail)),
			)
		}),
		layout.Rigid(layout.Spacer{Width: th.Sp.S4}.Layout),
		layout.Rigid(toggle),
	)
}

func (ed *Editor) layoutFacts(gtx layout.Context, e *env.Env, caps protocol.MemoryCaps) layout.Dimensions {
	th, memory := e.Theme, ed.memory
	parts := SourceOf(gtx.Now, memory)
	rows := []look.FactRow{{Label: i18n.T("Learned"), Value: parts.String()}}
	if by := LearnedBy(memory.Source); by != "" {
		rows = append(rows, look.FactRow{Label: i18n.T("Learned by"), Value: by})
	}
	used := i18n.T("Not yet")
	if memory.LastUsed != nil {
		used = Day(gtx.Now, *memory.LastUsed)
		if memory.LastUsedIn != nil && memory.LastUsedIn.Title != "" {
			used = i18n.Tf("%s in %s", used, memory.LastUsedIn.Title)
		}
	}
	rows = append(rows, look.FactRow{Label: i18n.T("Last used"), Value: used})
	if !memory.Updated.IsZero() && memory.Updated.After(memory.Created) {
		rows = append(rows, look.FactRow{Label: i18n.T("Edited"), Value: Day(gtx.Now, memory.Updated)})
	}
	versions := look.FactRow{Label: i18n.T("Earlier versions"), Value: "-"}
	if caps.Versions && len(memory.Versions) > 0 {
		label := i18n.T("Show")
		if ed.showVersions {
			label = i18n.T("Hide")
		}
		ed.versions.Kind, ed.versions.Small, ed.versions.Label = kit.PlaqueQuiet, true, label
		count := len(memory.Versions)
		versions.Widget = func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
				layout.Rigid(kit.RawText(th.Theme, th.T.Body, th.R.Fg, strconv.Itoa(count))),
				layout.Rigid(layout.Spacer{Width: th.Sp.S3}.Layout),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return look.Named(gtx, i18n.T("Show earlier versions"), func(gtx layout.Context) layout.Dimensions { return ed.versions.Layout(gtx, th.Theme) })
				}),
			)
		}
	}
	rows = append(rows, versions)
	return look.Facts(gtx, th, rows)
}

// LearnedBy names what learned a memory: Ask or an agent, on which model.
func LearnedBy(source protocol.MemorySource) string {
	who := ""
	switch source.Kind {
	case protocol.MemoryFromAsk:
		who = i18n.T("Ask")
	case protocol.MemoryFromAgent:
		who = i18n.T("An agent")
	case protocol.MemoryFromTeam:
		who = i18n.T("A team")
	default:
		return source.Model
	}
	if source.Model == "" {
		return who
	}
	return i18n.Tf("%s on %s", who, source.Model)
}

func (ed *Editor) layoutVersions(gtx layout.Context, th *look.Theme) layout.Dimensions {
	versions := ed.memory.Versions
	if len(ed.restores) != len(versions) {
		ed.restores = make([]kit.Plaque, len(versions))
	}
	children := make([]layout.FlexChild, 0, 2*len(versions))
	for index, version := range versions {
		restore := &ed.restores[index]
		restore.Kind, restore.Small, restore.Label = kit.PlaqueQuiet, true, i18n.T("Restore")
		children = append(children,
			layout.Rigid(layout.Spacer{Height: th.Sp.S2}.Layout),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
					layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
						return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
							layout.Rigid(func(gtx layout.Context) layout.Dimensions { return FactText(gtx, th, th.R.Fg2, version.Text) }),
							layout.Rigid(kit.RawText(th.Theme, th.T.Caption, th.R.Fg3, Day(gtx.Now, version.At))),
						)
					}),
					layout.Rigid(layout.Spacer{Width: th.Sp.S3}.Layout),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return look.Named(gtx, i18n.Tf("Restore \"%s\"", env.PlainMemory(version.Text)), func(gtx layout.Context) layout.Dimensions { return restore.Layout(gtx, th.Theme) })
					}),
				)
			}),
		)
	}
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
}

func (ed *Editor) layoutActs(gtx layout.Context, th *look.Theme) layout.Dimensions {
	ed.save.Kind, ed.save.Label, ed.save.Busy = kit.PlaqueGold, i18n.T("Save"), ed.busy
	ed.cancel.Kind, ed.cancel.Label = kit.PlaqueQuiet, i18n.T("Cancel")
	ed.forget.Kind, ed.forget.Label = kit.PlaqueDangerLine, i18n.T("Forget")
	children := []layout.FlexChild{
		layout.Rigid(func(gtx layout.Context) layout.Dimensions { return ed.save.Layout(gtx, th.Theme) }),
		layout.Rigid(layout.Spacer{Width: th.Sp.S2}.Layout),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions { return ed.cancel.Layout(gtx, th.Theme) }),
		layout.Flexed(1, layout.Spacer{}.Layout),
	}
	if ed.memory.ID != "" {
		children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return look.Named(gtx, i18n.T("Forget this memory"), func(gtx layout.Context) layout.Dimensions { return ed.forget.Layout(gtx, th.Theme) })
		}))
	}
	return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx, children...)
}
