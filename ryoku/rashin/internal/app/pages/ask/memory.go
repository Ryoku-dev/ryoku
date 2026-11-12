package ask

import (
	"gioui.org/layout"

	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/kit"
	"ryoku-i18n"

	"ryoku-rashin/internal/app/env"
	"ryoku-rashin/internal/app/look"
	"ryoku-rashin/internal/app/state"
	"ryoku-rashin/internal/protocol"
)

// what a conversation does with memory, picked before it starts and only narrowed after
type memoryChoice struct {
	control look.Select
	lock    look.Tip
	// what the next new conversation starts with, empty for its default
	draft   string
	changed string
}

func memoryOn(snapshot state.Snapshot) bool {
	return snapshot.Memory != nil && snapshot.Settings.Memory.Asked && snapshot.Settings.Memory.Use
}

func memoryValues(temporary bool) []string {
	if temporary {
		return []string{protocol.MemoryUseOnly, protocol.MemoryNone}
	}
	return []string{protocol.MemoryRemember, protocol.MemoryUseOnly, protocol.MemoryNone}
}

func memoryOptions(temporary bool) []look.SelectOption {
	if temporary {
		return []look.SelectOption{
			{Label: i18n.T("Use only"), Detail: i18n.T("Answers may draw on what Rashin knows about you.")},
			{Label: i18n.T("No memory"), Detail: i18n.T("Answers draw on nothing Rashin remembers.")},
		}
	}
	return []look.SelectOption{
		{Label: i18n.T("Remember"), Detail: i18n.T("Answers draw on memory, and Rashin learns from this conversation.")},
		{Label: i18n.T("Use only"), Detail: i18n.T("Answers draw on memory. Rashin learns nothing from it.")},
		{Label: i18n.T("No memory"), Detail: i18n.T("Neither draws on memory nor teaches it. It stays in your history.")},
	}
}

// a temporary conversation starts from use only, whatever was picked for an ordinary one
func memoryValue(value string, temporary bool) string {
	if temporary && value == protocol.MemoryRemember {
		return protocol.MemoryUseOnly
	}
	return value
}

// only taking away is allowed once a conversation has started: remember, then use only, then no memory
func narrows(from, to string) bool {
	rank := map[string]int{protocol.MemoryRemember: 0, protocol.MemoryUseOnly: 1, protocol.MemoryNone: 2}
	return rank[to] > rank[from]
}

func (choice *memoryChoice) layout(gtx layout.Context, e *env.Env, value string, temporary, locked bool, focus any) layout.Dimensions {
	value = memoryValue(value, temporary)
	values, options := memoryValues(temporary), memoryOptions(temporary)
	selected := 0
	for index, candidate := range values {
		if candidate == value {
			selected = index
		}
		if locked && candidate != value && !narrows(value, candidate) {
			options[index].Disabled = true
			options[index].Detail = i18n.T("Set when this conversation started.")
		}
	}
	control := &choice.control
	control.Form, control.Label, control.Icon, control.Return = look.SelectQuiet, i18n.T("Memory"), look.Icon.Memory, focus
	dimensions := layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return control.LayoutOptions(gtx, e.Theme, e.Overlays, options, selected)
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			if !locked {
				return layout.Dimensions{}
			}
			words := i18n.T("Set when this conversation started.")
			return choice.lock.Layout(gtx, e.Theme, e.Overlays, words, func(gtx layout.Context) layout.Dimensions {
				return look.Named(gtx, words, look.IconWidget(look.Icon.Lock, 14, e.Theme.R.Fg3))
			})
		}),
	)
	if index, changed := control.Changed(); changed && index >= 0 && index < len(values) && values[index] != value {
		choice.changed = values[index]
	}
	return dimensions
}

func (choice *memoryChoice) Changed() (string, bool) {
	value := choice.changed
	choice.changed = ""
	return value, value != ""
}

// set records a pick: what the next conversation starts with, or a narrowing of the one open
func (choice *memoryChoice) set(e *env.Env, id, value string) {
	if id == "" {
		choice.draft = value
		return
	}
	e.Call(protocol.MSessionSetMemory, protocol.SetMemoryParams{ID: id, Memory: value}, new(protocol.SessionInfo), e.Report)
}

// the sidebar says it of a conversation memory stays out of
func memoryMark(info protocol.SessionInfo) string {
	switch {
	case info.Temporary:
		return i18n.T("Temporary")
	case info.Memory == protocol.MemoryNone:
		return i18n.T("No memory")
	}
	return ""
}

// the first-run card, once, in place of the things to try, until the reader answers it
type firstRun struct {
	on, later, how kit.Plaque
}

func firstRunShown(snapshot state.Snapshot) bool {
	return snapshot.Memory != nil && !snapshot.Settings.Memory.Asked
}

func (card *firstRun) layout(gtx layout.Context, e *env.Env) layout.Dimensions {
	th := e.Theme
	card.on.Kind, card.on.Label = kit.PlaqueGold, i18n.T("Turn on memory")
	card.later.Kind, card.later.Label = kit.PlaqueQuiet, i18n.T("Not now")
	card.how.Kind, card.how.Label = kit.PlaqueQuiet, i18n.T("How it works")
	if card.on.Clicked(gtx) {
		answerFirstRun(e, true)
	}
	if card.later.Clicked(gtx) {
		answerFirstRun(e, false)
	}
	if card.how.Clicked(gtx) && e.OpenSettings != nil {
		e.OpenSettings("memory")
	}
	paragraph := func(text string) layout.Widget { return look.Paragraph(th, th.T.Body, th.R.Fg2, text) }
	return kit.SlabRows(gtx, th.Theme,
		func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
				layout.Rigid(kit.Text(th.Theme, th.T.Title, th.R.Fg, i18n.T("Rashin can remember"))),
				layout.Rigid(layout.Spacer{Height: th.Sp.S3}.Layout),
				layout.Rigid(paragraph(i18n.T("When a conversation goes quiet, Rashin keeps what will help next time: what you told it about yourself, your projects and how you like answers. Asking about something doesn't make it about you."))),
				layout.Rigid(layout.Spacer{Height: th.Sp.S2}.Layout),
				layout.Rigid(paragraph(i18n.T("It never keeps passwords, keys or tokens, and leaves out health, beliefs and money unless you say otherwise. It doesn't learn from command output, files or web pages."))),
				layout.Rigid(layout.Spacer{Height: th.Sp.S2}.Layout),
				layout.Rigid(paragraph(i18n.T("Everything stays on this computer, in files you can read. Each thing it remembers shows in the conversation, where you can undo it, and on the Memory page, where you can change or forget it."))),
			)
		},
		func(gtx layout.Context) layout.Dimensions {
			return look.Flow{Gap: 8, LineGap: 8}.Layout(gtx, 3, func(gtx layout.Context, index int) layout.Dimensions {
				return [...]*kit.Plaque{&card.on, &card.later, &card.how}[index].Layout(gtx, th.Theme)
			})
		},
	)
}

// turning it on starts with the next conversation, never one held before
func answerFirstRun(e *env.Env, on bool) {
	patch := map[string]any{"asked": true}
	if on {
		patch["use"], patch["learn"] = true, true
	}
	e.SetSettings(map[string]any{"memory": patch}, func(err error) {
		if err != nil {
			e.Report(err)
			return
		}
		if on {
			e.Notify(i18n.T("Memory is on. Rashin starts with your next conversation."))
		}
	})
}
