package chat

import (
	"encoding/json"
	"strconv"
	"time"

	"ryoku-rashin/internal/protocol"
)

type rowKind uint8

const (
	rowUser rowKind = iota + 1
	rowText
	rowThought
	rowTool
	rowPermission
	rowPlan
	rowTerminal
	rowAnswer
	rowError
	rowTurnEnd
)

type transcriptRow struct {
	kind       rowKind
	key        string
	seq        int64
	user       protocol.UserData
	text       string
	tool       *protocol.ToolData
	tools      []*protocol.ToolData
	permission *permissionState
	plan       []protocol.PlanEntry
	terminal   *terminalState
	answer     *protocol.AnswerData
	err        protocol.ErrorData
	turn       *turnState
}

type permissionState struct {
	data     protocol.PermissionData
	resolved *protocol.PermissionResolvedData
}

type terminalState struct {
	data protocol.TerminalData
	text string
}

type turnState struct {
	number int
	start  protocol.TurnStartData
	end    *protocol.TurnEndData
	answer *protocol.AnswerData
	// the text of the turn shows a command the answer offers to run, so the slab that runs it stands there
	offeredInText bool
	planRow       int
	answerRow     int
	footerAdded   bool
	// when the turn started and ended, for how long it worked
	began, ended time.Time
}

type modelState struct {
	lastSeq int64

	rows            []transcriptRow
	messages        map[string]int
	tools           map[string]int
	permissions     map[string]*permissionState
	permissionOrder []string
	terminals       map[string]*terminalState
	terminalOwners  map[string]string
	// children are the calls a subagent made, under the call that runs it
	children   map[string][]*protocol.ToolData
	childByID  map[string]*protocol.ToolData
	turns      map[int]*turnState
	turnOrder  []*turnState
	activeTurn *turnState

	model    protocol.ChoiceData
	mode     protocol.ChoiceData
	options  protocol.OptionsData
	commands protocol.CommandsData
	// counts changes to the three above, so a composer rebuilds its lists only then
	choices uint64
}

func newModelState() *modelState {
	return &modelState{
		messages:       make(map[string]int),
		tools:          make(map[string]int),
		permissions:    make(map[string]*permissionState),
		terminals:      make(map[string]*terminalState),
		terminalOwners: make(map[string]string),
		children:       make(map[string][]*protocol.ToolData),
		childByID:      make(map[string]*protocol.ToolData),
		turns:          make(map[int]*turnState),
	}
}

func (state *modelState) apply(model *Model, event protocol.SessionEvent) {
	if event.Seq <= state.lastSeq {
		return
	}
	state.lastSeq = event.Seq
	if event.Seq > model.info.LastSeq {
		model.info.LastSeq = event.Seq
	}

	switch event.Kind {
	case protocol.EvUser:
		var data protocol.UserData
		if decodeEvent(event, &data) {
			state.rows = append(state.rows, transcriptRow{kind: rowUser, key: strconv.FormatInt(event.Seq, 10), seq: event.Seq, user: data})
		}
	case protocol.EvText, protocol.EvThought:
		var data protocol.TextData
		if !decodeEvent(event, &data) || data.Message == "" {
			return
		}
		kind := rowText
		if event.Kind == protocol.EvThought {
			kind = rowThought
		}
		key := event.Kind + "\x00" + data.Message
		if index, ok := state.messages[key]; ok {
			state.rows[index].text += data.Delta
			state.rows[index].seq = event.Seq
		} else {
			state.messages[key] = len(state.rows)
			state.rows = append(state.rows, transcriptRow{kind: kind, key: key, seq: event.Seq, text: data.Delta, turn: state.activeTurn})
		}
	case protocol.EvTool:
		state.applyTool(event)
	case protocol.EvPermission:
		var data protocol.PermissionData
		if !decodeEvent(event, &data) || data.Request == "" {
			return
		}
		permission, ok := state.permissions[data.Request]
		if !ok {
			permission = &permissionState{}
			state.permissions[data.Request] = permission
			state.permissionOrder = append(state.permissionOrder, data.Request)
			state.rows = append(state.rows, transcriptRow{kind: rowPermission, key: data.Request, permission: permission})
		}
		permission.data = data
	case protocol.EvPermissionResolved:
		var data protocol.PermissionResolvedData
		if !decodeEvent(event, &data) || data.Request == "" {
			return
		}
		if permission := state.permissions[data.Request]; permission != nil {
			permission.resolved = &data
		}
	case protocol.EvPlan:
		var data protocol.PlanData
		if decodeEvent(event, &data) {
			state.applyPlan(data)
		}
	case protocol.EvTerminal:
		state.applyTerminal(event)
	case protocol.EvTurnStart:
		var data protocol.TurnStartData
		if decodeEvent(event, &data) {
			turn := &turnState{number: data.Turn, start: data, planRow: -1, answerRow: -1, began: event.TS}
			state.turns[data.Turn] = turn
			state.turnOrder = append(state.turnOrder, turn)
			state.activeTurn = turn
		}
	case protocol.EvTurnEnd:
		var data protocol.TurnEndData
		if decodeEvent(event, &data) {
			state.finishTurn(data, event.Seq, event.TS)
		}
	case protocol.EvError:
		var data protocol.ErrorData
		if decodeEvent(event, &data) {
			state.rows = append(state.rows, transcriptRow{kind: rowError, err: data})
		}
	case protocol.EvModel:
		if decodeEvent(event, &state.model) {
			model.info.Model = state.model.Current
			state.choices++
		}
	case protocol.EvMode:
		if decodeEvent(event, &state.mode) {
			model.info.Mode = state.mode.Current
			state.choices++
		}
	case protocol.EvOptions:
		// a fresh value each time: decoding into the old one would keep an option the harness dropped
		var options protocol.OptionsData
		if decodeEvent(event, &options) {
			state.options = options
			state.choices++
		}
	case protocol.EvCommands:
		decodeEvent(event, &state.commands)
	case protocol.EvAnswer:
		var data protocol.AnswerData
		if decodeEvent(event, &data) {
			state.applyAnswer(data)
		}
	case protocol.EvTitle:
		var data protocol.TitleData
		if decodeEvent(event, &data) {
			model.info.Title = data.Title
		}
	}
}

func decodeEvent(event protocol.SessionEvent, target any) bool {
	return len(event.Data) > 0 && json.Unmarshal(event.Data, target) == nil
}

func (state *modelState) currentTurn() *turnState {
	if state.activeTurn != nil {
		return state.activeTurn
	}
	if len(state.turnOrder) == 0 {
		return nil
	}
	return state.turnOrder[len(state.turnOrder)-1]
}

func (state *modelState) applyTool(event protocol.SessionEvent) {
	var incoming protocol.ToolData
	if !decodeEvent(event, &incoming) || incoming.Call == "" {
		return
	}
	if current := state.childByID[incoming.Call]; current != nil {
		mergeTool(current, incoming, event.Data)
		return
	}
	if incoming.Parent != "" {
		child := incoming
		state.childByID[child.Call] = &child
		state.children[child.Parent] = append(state.children[child.Parent], &child)
		return
	}
	if index, ok := state.tools[incoming.Call]; ok {
		current := findTool(state.rows[index].tools, incoming.Call)
		if current == nil {
			current = state.rows[index].tool
		}
		if current == nil {
			return
		}
		previousTerminal := current.Terminal
		mergeTool(current, incoming, event.Data)
		if previousTerminal != current.Terminal && state.terminalOwners[previousTerminal] == incoming.Call {
			delete(state.terminalOwners, previousTerminal)
		}
		if current.Terminal != "" {
			state.terminalOwners[current.Terminal] = incoming.Call
		}
		return
	}
	tool := incoming
	index := len(state.rows)
	if index > 0 && state.rows[index-1].kind == rowTool {
		index--
		row := &state.rows[index]
		row.tools = append(row.tools, &tool)
	} else {
		state.rows = append(state.rows, transcriptRow{kind: rowTool, key: incoming.Call, seq: event.Seq, tool: &tool, tools: []*protocol.ToolData{&tool}})
	}
	state.tools[incoming.Call] = index
	if tool.Terminal != "" {
		state.terminalOwners[tool.Terminal] = tool.Call
	}
}

func findTool(tools []*protocol.ToolData, call string) *protocol.ToolData {
	for _, tool := range tools {
		if tool != nil && tool.Call == call {
			return tool
		}
	}
	return nil
}

func mergeTool(current *protocol.ToolData, incoming protocol.ToolData, raw json.RawMessage) {
	var fields map[string]json.RawMessage
	if current == nil || json.Unmarshal(raw, &fields) != nil {
		return
	}
	if _, ok := fields["title"]; ok {
		current.Title = incoming.Title
	}
	if _, ok := fields["kind"]; ok {
		current.Kind = incoming.Kind
	}
	if _, ok := fields["status"]; ok {
		current.Status = incoming.Status
	}
	if _, ok := fields["input"]; ok {
		current.Input = incoming.Input
	}
	if _, ok := fields["output"]; ok {
		current.Output = incoming.Output
	}
	if _, ok := fields["diff"]; ok {
		current.Diff = incoming.Diff
	}
	if _, ok := fields["terminal"]; ok {
		current.Terminal = incoming.Terminal
	}
	if _, ok := fields["locations"]; ok {
		current.Locations = incoming.Locations
	}
	if _, ok := fields["agents"]; ok {
		current.Agents = incoming.Agents
	}
}

func (state *modelState) applyPlan(data protocol.PlanData) {
	turn := state.currentTurn()
	if turn != nil && turn.planRow >= 0 {
		state.rows[turn.planRow].plan = append(state.rows[turn.planRow].plan[:0], data.Entries...)
		return
	}
	index := len(state.rows)
	entries := append([]protocol.PlanEntry(nil), data.Entries...)
	state.rows = append(state.rows, transcriptRow{kind: rowPlan, plan: entries, turn: turn})
	if turn != nil {
		turn.planRow = index
	}
}

func (state *modelState) applyTerminal(event protocol.SessionEvent) {
	var data protocol.TerminalData
	if !decodeEvent(event, &data) || data.Terminal == "" {
		return
	}
	terminal, ok := state.terminals[data.Terminal]
	if !ok {
		terminal = &terminalState{data: data}
		state.terminals[data.Terminal] = terminal
		state.rows = append(state.rows, transcriptRow{kind: rowTerminal, key: data.Terminal, terminal: terminal})
	}
	if data.Command != "" {
		terminal.data.Command = data.Command
	}
	terminal.text += data.Delta
	if data.Exit != nil {
		exit := *data.Exit
		terminal.data.Exit = &exit
	}
	if data.Done {
		terminal.data.Done = true
	}
}

func (state *modelState) applyAnswer(data protocol.AnswerData) {
	turn := state.currentTurn()
	if turn != nil {
		copy := data
		turn.answer = &copy
		turn.offeredInText = state.textRestates(turn, copy.Plan)
		if turn.answerRow >= 0 {
			state.rows[turn.answerRow].answer = turn.answer
			return
		}
		turn.answerRow = len(state.rows)
		state.rows = append(state.rows, transcriptRow{kind: rowAnswer, answer: turn.answer, turn: turn})
		return
	}
	copy := data
	state.rows = append(state.rows, transcriptRow{kind: rowAnswer, answer: &copy})
}

func (state *modelState) textRestates(turn *turnState, plan *protocol.CommandPlan) bool {
	if plan == nil {
		return false
	}
	for index := range state.rows {
		if row := &state.rows[index]; row.kind == rowText && row.turn == turn {
			for _, block := range parseMarkdown(row.text) {
				if block.kind == markdownCode && restates(block.text, plan) {
					return true
				}
			}
		}
	}
	return false
}

func (state *modelState) finishTurn(data protocol.TurnEndData, seq int64, at time.Time) {
	turn := state.turns[data.Turn]
	if turn == nil {
		turn = &turnState{number: data.Turn, start: protocol.TurnStartData{Turn: data.Turn}, planRow: -1, answerRow: -1}
		state.turns[data.Turn] = turn
		state.turnOrder = append(state.turnOrder, turn)
	}
	copy := data
	turn.end, turn.ended = &copy, at
	if state.activeTurn == turn {
		state.activeTurn = nil
	}
	if !turn.footerAdded {
		turn.footerAdded = true
		state.rows = append(state.rows, transcriptRow{kind: rowTurnEnd, seq: seq, turn: turn})
	}
}

func (state *modelState) pendingPermissions() []protocol.PermissionData {
	pending := make([]protocol.PermissionData, 0, len(state.permissionOrder))
	for _, request := range state.permissionOrder {
		permission := state.permissions[request]
		if permission != nil && permission.resolved == nil {
			data := permission.data
			data.Options = append([]protocol.PermissionOption(nil), data.Options...)
			pending = append(pending, data)
		}
	}
	return pending
}
