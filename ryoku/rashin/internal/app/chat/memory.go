package chat

import (
	"image"
	"slices"
	"strconv"
	"strings"

	"gioui.org/layout"
	"gioui.org/op"

	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/kit"
	"ryoku-i18n"

	"ryoku-rashin/internal/app/look"
	"ryoku-rashin/internal/protocol"
)

// what a memory line keeps between frames: the event and its change as last drawn, read again by its popover after the frame
type memoryVisual struct {
	data   protocol.MemoryEventData
	change protocol.MemoryChange
	links  map[string]*kit.Click
	// the conversation it stands in
	session string
	// memories the reader said were no use in this conversation
	dropped map[string]bool
	// its popover was open last frame
	shown bool
}

func (state *modelState) applyMemory(event protocol.SessionEvent, kind protocol.SessionKind) {
	var data protocol.MemoryEventData
	if !decodeEvent(event, &data) || data.Kind == "" {
		return
	}
	row := transcriptRow{kind: rowMemory, key: strconv.FormatInt(event.Seq, 10), seq: event.Seq, memory: &data, turn: state.activeTurn}
	// an answer's memory lines stand under it, so what Ask wrote while it ran waits for the turn to end; an agent's stands where its call did
	if turn := state.activeTurn; turn != nil && kind == protocol.KindAsk && data.Kind != protocol.MemoryNarrowed {
		turn.memory = append(turn.memory, row)
		return
	}
	state.rows = append(state.rows, row)
	state.tellCall(data.Kind)
}

// a memory call waiting for its line, and the turn it was made in
type pendingCall struct {
	call, name string
	turn       *turnState
}

// a call to Rashin's remember, forget or session search, as its title names it
func memoryTool(tool protocol.ToolData) string {
	name, ours := strings.CutPrefix(strings.TrimPrefix(tool.Title, "mcp__"), "rashin_")
	if !ours || (name != "remember" && name != "forget" && name != "session_search") {
		return ""
	}
	return name
}

// a memory call stands until the memory line that tells it arrives in its turn, then gives way to it; a failed call stays, no line tells it
func (state *modelState) tellCall(kind string) {
	turn := state.currentTurn()
	for index, call := range state.memoryTools {
		told := call.name == "session_search" && kind == protocol.MemoryLooked ||
			call.name != "session_search" && (kind == protocol.MemoryUpdated || kind == protocol.MemoryRefused)
		if !told || call.turn != turn {
			continue
		}
		state.memoryTools = append(state.memoryTools[:index:index], state.memoryTools[index+1:]...)
		at, ok := state.tools[call.call]
		if !ok {
			return
		}
		row := &state.rows[at]
		tool := findTool(row.tools, call.call)
		if tool == nil || tool.Status == protocol.ToolFailed {
			return
		}
		row.tools = slices.DeleteFunc(row.tools, func(candidate *protocol.ToolData) bool { return candidate == tool })
		row.tool = nil
		if len(row.tools) > 0 {
			row.tool = row.tools[0]
		}
		delete(state.tools, call.call)
		state.memoryCalls[call.call] = true
		return
	}
}

func (state *viewState) memoryVisual(key string) *memoryVisual {
	visual := state.memory[key]
	if visual == nil {
		visual = &memoryVisual{links: make(map[string]*kit.Click), dropped: make(map[string]bool)}
		state.memory[key] = visual
	}
	return visual
}

func (state *viewState) layoutMemory(gtx layout.Context, th *look.Theme, block blockContext) layout.Dimensions {
	// one view shows every conversation of a page, so a line is known by its conversation and its place in it
	key := "memory:" + block.info.ID + ":" + block.row.key
	visual, memory := state.workVisual(key), state.memoryVisual(key)
	memory.data, memory.session = *block.row.memory, block.info.ID
	memory.change = protocol.MemoryChange{}
	if change := memory.data.Change; change != nil {
		memory.change = *change
		if latest, ok := state.changes[change.ID]; ok {
			memory.change = latest
		}
	}
	line := memoryLine(memory)
	tag := &visual.click.Clickable
	open := state.overlays != nil && state.overlays.IsOpen(tag)
	visual.open = open
	// a popover that closed leaves the keyboard on the line, the composer wants it back
	if memory.shown && !open {
		state.act(Action{Kind: ActRelease})
	}
	memory.shown = open
	dimensions := look.Named(gtx, line.label, func(gtx layout.Context) layout.Dimensions {
		return state.layoutLine(gtx, th, visual, line)
	})
	if !visual.pressed || state.overlays == nil {
		return dimensions
	}
	visual.pressed = false
	if open {
		state.overlays.Close(tag)
		return dimensions
	}
	placement := look.Placement{Gap: 8, Width: 420, Frame: true}
	state.overlays.Open(tag, state.overlays.AnchorOf(tag, dimensions.Size), placement, func(gtx layout.Context) layout.Dimensions {
		state.popover = tag
		return layout.UniformInset(5).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			return state.layoutMemoryPopover(gtx, th, key, memory)
		})
	})
	return dimensions
}

func memoryLine(memory *memoryVisual) workLine {
	data, change := memory.data, memory.change
	line := workLine{icon: look.Icon.Memory, pops: true}
	switch data.Kind {
	case protocol.MemoryUpdated:
		line.label, line.prose = i18n.T("Memory updated"), changeWords(change)
		if change.Undone {
			line.label, line.prose = i18n.T("Memory unchanged"), i18n.T("you undid this")
		}
		line.pops = len(change.Items) > 0
	case protocol.MemoryUsed:
		line.label = usedWords(len(data.Used))
		line.pops = len(data.Used) > 0
		switch {
		// an engine that did not answer in time says so itself, in the daemon's words
		case len(data.Used) == 0 && data.Text != "":
			line.label = data.Text
		case len(data.Used) == 0 && data.Card:
			line.label, line.pops = i18n.T("Used the card about you"), true
		}
	case protocol.MemoryLooked:
		line.icon, line.label = look.Icon.Ask, lookedWords(len(data.Sessions))
		line.pops = len(data.Sessions) > 0
	case protocol.MemoryRefused:
		line.label, line.prose = i18n.T("Memory unchanged"), refusedWords(data.Reason)
	case protocol.MemoryNarrowed:
		line.label, line.pops = narrowedWords(data.Memory), false
	default:
		line.label, line.pops = i18n.T("Memory updated"), false
	}
	return line
}

// counted by what each memory went through: "remembered 1, replaced 1"
func changeWords(change protocol.MemoryChange) string {
	counts := map[string]int{}
	replacing := 0
	for _, item := range change.Items {
		counts[item.Op]++
		if item.Replaces != "" {
			replacing++
		}
	}
	replaced := counts[protocol.MemoryOpReplaced]
	if replaced == 0 {
		replaced = replacing
	}
	var parts []string
	if count := counts[protocol.MemoryOpRemembered]; count > 0 {
		parts = append(parts, i18n.Tf("remembered %d", count))
	}
	if replaced > 0 {
		parts = append(parts, i18n.Tf("replaced %d", replaced))
	}
	if count := counts[protocol.MemoryOpEdited]; count > 0 {
		parts = append(parts, i18n.Tf("changed %d", count))
	}
	if count := counts[protocol.MemoryOpForgot]; count > 0 {
		parts = append(parts, i18n.Tf("forgot %d", count))
	}
	if count := counts[protocol.MemoryOpSuggested]; count > 0 {
		parts = append(parts, i18n.Tf("%d suggested to you", count))
	}
	return strings.Join(parts, ", ")
}

func usedWords(count int) string {
	if count == 1 {
		return i18n.T("Used 1 memory")
	}
	return i18n.Tf("Used %d memories", count)
}

func lookedWords(count int) string {
	if count == 1 {
		return i18n.T("Looked at 1 past conversation")
	}
	return i18n.Tf("Looked at %d past conversations", count)
}

func refusedWords(reason string) string {
	switch reason {
	case protocol.MemoryRefusedSecret:
		return i18n.T("looked like a secret")
	case protocol.MemoryRefusedSensitive:
		return i18n.T("sensitive topic left out")
	case protocol.MemoryRefusedInstruction:
		return i18n.T("read like an instruction")
	}
	return i18n.T("memory is off")
}

func narrowedWords(mode string) string {
	if mode == protocol.MemoryNone {
		return i18n.T("Rashin won't use memory or learn from this conversation")
	}
	return i18n.T("Rashin won't learn from this conversation")
}

func (state *viewState) layoutMemoryPopover(gtx layout.Context, th *look.Theme, key string, memory *memoryVisual) layout.Dimensions {
	switch memory.data.Kind {
	case protocol.MemoryUsed:
		return state.layoutUsedPopover(gtx, th, key, memory)
	case protocol.MemoryLooked:
		return state.layoutLookedPopover(gtx, th, memory)
	case protocol.MemoryRefused:
		return state.layoutRefusedPopover(gtx, th, key, memory)
	}
	return state.layoutChangePopover(gtx, th, key, memory)
}

// the rows of a change: a memory that replaced another tells it under its own row, so the one it replaced has none
func shownItems(change protocol.MemoryChange) []protocol.MemoryItem {
	told := map[string]bool{}
	for _, item := range change.Items {
		if item.Replaces != "" {
			told[item.Replaces] = true
		}
	}
	items := make([]protocol.MemoryItem, 0, len(change.Items))
	for _, item := range change.Items {
		if item.Op == protocol.MemoryOpReplaced && told[item.Text] {
			continue
		}
		items = append(items, item)
	}
	return items
}

func changeTitle(items []protocol.MemoryItem) string {
	forgot, suggested := 0, 0
	for _, item := range items {
		switch item.Op {
		case protocol.MemoryOpForgot:
			forgot++
		case protocol.MemoryOpSuggested:
			suggested++
		}
	}
	switch {
	case forgot == len(items):
		return i18n.T("Rashin forgot")
	case suggested == len(items):
		return i18n.T("Suggested to you")
	}
	return i18n.T("Rashin remembered")
}

func scopeWords(scope, name string) string {
	if name != "" {
		return name
	}
	if scope == protocol.MemoryYou || scope == "" {
		return i18n.T("About you")
	}
	return scope
}

func (state *viewState) layoutChangePopover(gtx layout.Context, th *look.Theme, key string, memory *memoryVisual) layout.Dimensions {
	change := memory.change
	items := shownItems(change)
	rows := make([]layout.Widget, 0, len(items))
	first, undoable := "", false
	for index, item := range items {
		caption := scopeWords(item.Scope, item.ScopeName)
		if item.Replaces != "" {
			caption = i18n.Tf("Replaces \"%s\"", state.plainMemory(item.Replaces))
		}
		forgotten := item.Op == protocol.MemoryOpForgot
		if first == "" && !forgotten {
			first = item.ID
		}
		plaques := key + ":" + strconv.Itoa(index)
		var trailing []layout.Widget
		switch {
		case item.Op == protocol.MemoryOpSuggested:
			trailing = []layout.Widget{
				state.memoryPlaque(th, plaques+":keep", kit.PlaqueLine, i18n.T("Keep"), Action{Kind: ActMemoryKeep, Value: item.ID}, nil),
				state.memoryPlaque(th, plaques+":dismiss", kit.PlaqueQuiet, i18n.T("Dismiss"), Action{Kind: ActMemoryDismiss, Value: item.ID}, nil),
			}
		case change.Undone:
			undoable = true
			trailing = []layout.Widget{kit.Text(th.Theme, th.T.Caption, th.R.Fg3, i18n.T("Undone"))}
		default:
			undoable = true
			if !forgotten {
				trailing = []layout.Widget{state.memoryPlaque(th, plaques+":edit", kit.PlaqueQuiet, i18n.T("Edit"), Action{Kind: ActMemoryEdit, Value: item.ID}, nil)}
			}
		}
		rows = append(rows, state.memoryRow(th, item.Text, caption, trailing...))
	}
	var foot []layout.Widget
	// the daemon undoes a change whole, so the one plaque that does it stands for every row
	switch {
	case undoable && change.Undone:
		foot = append(foot, state.memoryPlaque(th, key+":redo", kit.PlaqueQuiet, i18n.T("Redo"), Action{Kind: ActMemoryRedo, Value: change.ID}, nil))
	case undoable:
		foot = append(foot, state.memoryPlaque(th, key+":undo", kit.PlaqueQuiet, i18n.T("Undo"), Action{Kind: ActMemoryUndo, Value: change.ID}, nil))
	}
	if first != "" && !change.Undone {
		foot = append(foot, state.memoryPlaque(th, key+":open", kit.PlaqueQuiet, i18n.T("Open memory"), Action{Kind: ActMemoryEdit, Value: first}, nil))
	}
	if change.Origin == protocol.MemoryByLearning && change.Model != "" {
		foot = append(foot, kit.Text(th.Theme, th.T.Caption, th.R.Fg3, i18n.Tf("Learned with %s.", change.Model)))
	}
	return popoverColumn(gtx, th, changeTitle(items), rows, foot)
}

func (state *viewState) layoutUsedPopover(gtx layout.Context, th *look.Theme, key string, memory *memoryVisual) layout.Dimensions {
	rows := make([]layout.Widget, 0, len(memory.data.Used))
	for index, item := range memory.data.Used {
		plaques := key + ":" + strconv.Itoa(index)
		trailing := []layout.Widget{kit.Text(th.Theme, th.T.Caption, th.R.Fg3, i18n.T("Not used here again"))}
		if !memory.dropped[item.ID] {
			id := item.ID
			trailing = []layout.Widget{
				state.memoryPlaque(th, plaques+":drop", kit.PlaqueQuiet, i18n.T("Not useful here"), Action{Kind: ActMemorySuppress, Value: id}, func() { memory.dropped[id] = true }),
				state.memoryPlaque(th, plaques+":edit", kit.PlaqueQuiet, i18n.T("Edit"), Action{Kind: ActMemoryEdit, Value: id}, nil),
			}
		}
		rows = append(rows, state.memoryRow(th, item.Text, scopeWords(item.Scope, item.ScopeName), trailing...))
	}
	var foot []layout.Widget
	switch {
	case memory.data.Card && len(rows) == 0:
		rows = append(rows, look.Paragraph(th, th.T.Body, th.R.Fg2, i18n.T("The card about you was in the prompt. No single memory was drawn on.")))
		foot = append(foot, state.memoryPlaque(th, key+":card", kit.PlaqueQuiet, i18n.T("What Ask sees"), Action{Kind: ActMemoryCard}, nil))
	case memory.data.Card:
		foot = append(foot, kit.Text(th.Theme, th.T.Caption, th.R.Fg3, i18n.T("Rashin also had the card about you.")))
	}
	return popoverColumn(gtx, th, i18n.T("This answer used"), rows, foot)
}

func (state *viewState) layoutLookedPopover(gtx layout.Context, th *look.Theme, memory *memoryVisual) layout.Dimensions {
	rows := make([]layout.Widget, 0, len(memory.data.Sessions))
	for _, ref := range memory.data.Sessions {
		click := memory.links[ref.Session]
		if click == nil {
			click = new(kit.Click)
			memory.links[ref.Session] = click
		}
		title := ref.Title
		if title == "" {
			title = i18n.T("Untitled conversation")
		}
		rows = append(rows, func(gtx layout.Context) layout.Dimensions {
			if click.Clicked(gtx) {
				action := Action{Kind: ActOpenSession, Value: ref.Session, Option: string(ref.Kind)}
				state.act(action)
				state.leavePopover(action)
				gtx.Execute(op.InvalidateCmd{})
			}
			return look.Link(gtx, th, click, title)
		})
	}
	return popoverColumn(gtx, th, i18n.T("This answer read"), rows, nil)
}

func (state *viewState) layoutRefusedPopover(gtx layout.Context, th *look.Theme, key string, memory *memoryVisual) layout.Dimensions {
	data := memory.data
	var rows, foot []layout.Widget
	// a secret is never shown again, not even here
	if data.Text != "" && data.Reason != protocol.MemoryRefusedSecret {
		rows = append(rows, state.memoryRow(th, data.Text, ""))
	}
	switch data.Reason {
	case protocol.MemoryRefusedSensitive:
		// a refusal offers once in a conversation: either answer takes both away
		session := memory.session
		answered := func() { state.offered[session] = true }
		rows = append(rows, look.Paragraph(th, th.T.Body, th.R.Fg2, i18n.T("It is about a sensitive topic, and Rashin leaves those out unless you say otherwise.")))
		if !state.offered[session] {
			foot = append(foot,
				state.memoryPlaque(th, key+":sensitive", kit.PlaqueLine, i18n.T("Turn on sensitive topics"), Action{Kind: ActMemorySettings}, answered),
				state.memoryPlaque(th, key+":once", kit.PlaqueQuiet, i18n.T("Remember this once"), Action{Kind: ActRememberOnce, Value: data.Text}, answered),
			)
		}
	case protocol.MemoryRefusedSecret:
		rows = append(rows, look.Paragraph(th, th.T.Body, th.R.Fg2, i18n.T("Rashin doesn't keep passwords, keys or tokens. Store it in the vault.")))
		foot = append(foot, state.memoryPlaque(th, key+":vault", kit.PlaqueLine, i18n.T("Open the vault"), Action{Kind: ActOpenVault}, nil))
	case protocol.MemoryRefusedInstruction:
		rows = append(rows, look.Paragraph(th, th.T.Body, th.R.Fg2, i18n.T("Rashin keeps facts about you, never instructions to follow.")))
	default:
		rows = append(rows, look.Paragraph(th, th.T.Body, th.R.Fg2, i18n.T("Memory is off, so nothing was kept.")))
	}
	return popoverColumn(gtx, th, i18n.T("Rashin left this out"), rows, foot)
}

// what takes the reader to another place closes the popover first, or it would stand over that place
func (state *viewState) leavePopover(action Action) {
	switch action.Kind {
	case ActMemoryEdit, ActMemoryCard, ActMemorySettings, ActOpenVault, ActOpenSession:
		if state.overlays != nil && state.popover != nil {
			state.overlays.Close(state.popover)
		}
	}
}

// a plaque in a popover raises its action the frame it is pressed
func (state *viewState) memoryPlaque(th *look.Theme, key string, kind kit.PlaqueKind, label string, action Action, then func()) layout.Widget {
	return func(gtx layout.Context) layout.Dimensions {
		plaque := state.plaque(key)
		plaque.Kind, plaque.Small, plaque.Label = kind, true, label
		if plaque.Clicked(gtx) {
			if then != nil {
				then()
			}
			state.act(action)
			state.leavePopover(action)
			gtx.Execute(op.InvalidateCmd{})
		}
		return plaque.Layout(gtx, th.Theme)
	}
}

func popoverColumn(gtx layout.Context, th *look.Theme, title string, rows, foot []layout.Widget) layout.Dimensions {
	gtx.Constraints.Min.X = gtx.Constraints.Max.X
	children := []layout.FlexChild{layout.Rigid(kit.Text(th.Theme, th.T.Title, th.R.Fg, title))}
	for _, row := range rows {
		children = append(children, layout.Rigid(layout.Spacer{Height: th.Sp.S3}.Layout), layout.Rigid(row))
	}
	if len(foot) > 0 {
		children = append(children, layout.Rigid(layout.Spacer{Height: th.Sp.S4}.Layout), layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return memoryActions(gtx, th, foot)
		}))
	}
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
}

// a fact is set the way the Memory page sets it, its code spans in mono and never their marks
func (state *viewState) memoryFact(th *look.Theme, text string) layout.Widget {
	return func(gtx layout.Context) layout.Dimensions {
		if state.memoryText.Fact == nil {
			return look.Paragraph(th, th.T.Body, th.R.Fg, state.plainMemory(text))(gtx)
		}
		return state.memoryText.Fact(gtx, th, th.R.Fg, text)
	}
}

func (state *viewState) plainMemory(text string) string {
	if state.memoryText.Plain == nil {
		return text
	}
	return state.memoryText.Plain(text)
}

func (state *viewState) memoryRow(th *look.Theme, text, caption string, trailing ...layout.Widget) layout.Widget {
	return func(gtx layout.Context) layout.Dimensions {
		gtx.Constraints.Min.X = gtx.Constraints.Max.X
		words := func(gtx layout.Context) layout.Dimensions {
			children := []layout.FlexChild{layout.Rigid(state.memoryFact(th, text))}
			if caption != "" {
				children = append(children, layout.Rigid(layout.Spacer{Height: 2}.Layout), layout.Rigid(look.Paragraph(th, th.T.Caption, th.R.Fg3, caption)))
			}
			return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
		}
		if len(trailing) == 0 {
			return words(gtx)
		}
		return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Start}.Layout(gtx,
			layout.Flexed(1, words),
			layout.Rigid(layout.Spacer{Width: th.Sp.S3}.Layout),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions { return memoryActions(gtx, th, trailing) }),
		)
	}
}

func memoryActions(gtx layout.Context, th *look.Theme, actions []layout.Widget) layout.Dimensions {
	children := make([]layout.FlexChild, 0, len(actions)*2)
	for index, action := range actions {
		if index > 0 {
			children = append(children, layout.Rigid(layout.Spacer{Width: th.Sp.S1}.Layout))
		}
		children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			gtx.Constraints.Min = image.Point{}
			return action(gtx)
		}))
	}
	return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx, children...)
}
