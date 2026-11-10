package harness

import (
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
	"unicode/utf8"

	"ryoku-rashin/internal/protocol"
)

const (
	toolOutputRunes = 6000
	toolDiffRunes   = 4000
	toolInputRunes  = 400
)

type toolCall struct {
	SessionUpdate string          `json:"sessionUpdate"`
	ToolCallID    string          `json:"toolCallId"`
	Title         *string         `json:"title"`
	Kind          *string         `json:"kind"`
	Status        *string         `json:"status"`
	RawInput      json.RawMessage `json:"rawInput"`
	RawOutput     json.RawMessage `json:"rawOutput"`
	Content       json.RawMessage `json:"content"`
	Locations     *[]struct {
		Path string `json:"path"`
		Line int    `json:"line"`
	} `json:"locations"`
}

type toolContent struct {
	Type    string `json:"type"`
	Content struct {
		Type string `json:"type"`
		Text string `json:"text"`
	} `json:"content"`
	Path       string  `json:"path"`
	OldText    *string `json:"oldText"`
	NewText    string  `json:"newText"`
	TerminalID string  `json:"terminalId"`
}

func (r *runner) handleNotification(method string, params json.RawMessage) {
	if method != "session/update" {
		return
	}
	var envelope struct {
		SessionID string          `json:"sessionId"`
		Update    json.RawMessage `json:"update"`
	}
	if json.Unmarshal(params, &envelope) != nil {
		return
	}
	var head struct {
		SessionUpdate string `json:"sessionUpdate"`
	}
	if json.Unmarshal(envelope.Update, &head) != nil {
		return
	}
	replayed := false
	r.mu.Lock()
	if r.loading {
		switch head.SessionUpdate {
		case "agent_message_chunk", "agent_thought_chunk", "tool_call", "tool_call_update", "plan", "available_commands_update":
			replayed = true
		}
	}
	r.mu.Unlock()
	if replayed {
		return
	}
	switch head.SessionUpdate {
	case "agent_message_chunk", "agent_thought_chunk":
		r.handleChunk(head.SessionUpdate, envelope.Update)
	case "tool_call", "tool_call_update":
		r.handleTool(envelope.Update)
	case "plan":
		var update struct {
			Entries []protocol.PlanEntry `json:"entries"`
		}
		if json.Unmarshal(envelope.Update, &update) == nil {
			r.breakChunkRun()
			r.emit(protocol.EvPlan, protocol.PlanData{Entries: update.Entries})
		}
	case "available_commands_update":
		r.handleCommands(envelope.Update)
	case "current_mode_update":
		var update struct {
			Current string `json:"currentModeId"`
		}
		if json.Unmarshal(envelope.Update, &update) == nil {
			r.mu.Lock()
			r.mode = update.Current
			choices := append([]protocol.Choice(nil), r.modes...)
			r.mu.Unlock()
			r.breakChunkRun()
			r.emit(protocol.EvMode, protocol.ChoiceData{Current: update.Current, Options: choices})
			r.updateInfo(func(info *protocol.SessionInfo) { info.Mode = update.Current })
		}
	case "config_option_update":
		var update struct {
			ConfigOptions []configOption `json:"configOptions"`
		}
		if json.Unmarshal(envelope.Update, &update) == nil {
			r.breakChunkRun()
			r.applyConfigOptions(update.ConfigOptions)
		}
	case "models":
		var update struct {
			Models *modelState `json:"models"`
			modelState
		}
		if json.Unmarshal(envelope.Update, &update) == nil {
			models := update.Models
			if models == nil {
				models = &update.modelState
			}
			choices := make([]protocol.Choice, 0, len(models.Available))
			for _, model := range models.Available {
				choices = append(choices, protocol.Choice{ID: model.ModelID, Name: model.Name, Description: model.Description})
			}
			r.mu.Lock()
			r.models, r.model = choices, models.Current
			r.mu.Unlock()
			r.breakChunkRun()
			r.emit(protocol.EvModel, protocol.ChoiceData{Current: models.Current, Options: choices})
			r.updateInfo(func(info *protocol.SessionInfo) { info.Model = models.Current })
		}
	case "current_model_update", "model_update":
		var update struct {
			Current string `json:"currentModelId"`
			Model   string `json:"modelId"`
		}
		if json.Unmarshal(envelope.Update, &update) == nil {
			if update.Current == "" {
				update.Current = update.Model
			}
			r.mu.Lock()
			r.model = update.Current
			choices := append([]protocol.Choice(nil), r.models...)
			r.mu.Unlock()
			r.breakChunkRun()
			r.emit(protocol.EvModel, protocol.ChoiceData{Current: update.Current, Options: choices})
			r.updateInfo(func(info *protocol.SessionInfo) { info.Model = update.Current })
		}
	case "session_info_update":
		var update struct {
			Title json.RawMessage `json:"title"`
		}
		if json.Unmarshal(envelope.Update, &update) == nil && len(update.Title) != 0 {
			var title string
			if string(update.Title) != "null" && json.Unmarshal(update.Title, &title) != nil {
				break
			}
			r.breakChunkRun()
			r.emit(protocol.EvTitle, protocol.TitleData{Title: title})
			r.updateInfo(func(info *protocol.SessionInfo) { info.Title = title })
		}
	case "usage_update":
		var update struct {
			Used int `json:"used"`
		}
		if json.Unmarshal(envelope.Update, &update) == nil {
			r.mu.Lock()
			r.usage = &protocol.TokenUsage{Input: update.Used}
			r.mu.Unlock()
			r.breakChunkRun()
		}
	}
}

func (r *runner) handleChunk(kind string, raw json.RawMessage) {
	var update struct {
		MessageID string `json:"messageId"`
		Content   struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"content"`
	}
	if json.Unmarshal(raw, &update) != nil || update.Content.Type != "text" || update.Content.Text == "" {
		return
	}
	eventKind := protocol.EvText
	if kind == "agent_thought_chunk" {
		eventKind = protocol.EvThought
	}
	r.mu.Lock()
	if r.chunkKind != eventKind || (update.MessageID != "" && r.chunkID != r.generationIDLocked(eventKind, update.MessageID)) {
		r.chunkSeq++
		r.chunkKind = eventKind
		if update.MessageID != "" {
			r.chunkID = r.generationIDLocked(eventKind, update.MessageID)
		} else {
			r.chunkID = r.generationIDLocked(eventKind, fmt.Sprint(r.chunkSeq))
		}
	}
	messageID := r.chunkID
	r.mu.Unlock()
	r.emit(eventKind, protocol.TextData{Message: messageID, Delta: update.Content.Text})
}

func (r *runner) generationIDLocked(kind, raw string) string {
	return fmt.Sprintf("%d-%s-%s", r.generation, kind, raw)
}

func (r *runner) breakChunkRun() {
	r.mu.Lock()
	r.chunkKind = ""
	r.chunkID = ""
	r.mu.Unlock()
}

func (r *runner) handleTool(raw json.RawMessage) {
	var call toolCall
	if json.Unmarshal(raw, &call) != nil || call.ToolCallID == "" {
		return
	}
	r.breakChunkRun()
	data, diffs := displayTool(call)
	r.mu.Lock()
	tracked := r.tools[call.ToolCallID]
	if call.Kind != nil {
		tracked.kind = normalizeToolKind(*call.Kind)
	}
	if call.Status != nil {
		tracked.status = normalizeToolStatus(*call.Status)
	} else if call.SessionUpdate == "tool_call" && tracked.status == "" {
		tracked.status = protocol.ToolPending
		data.Status = protocol.ToolPending
	}
	if tracked.order == 0 {
		r.toolOrder++
		tracked.order = r.toolOrder
	}
	r.tools[call.ToolCallID] = tracked
	r.mu.Unlock()
	r.emit(protocol.EvTool, data)
	for _, diff := range diffs {
		r.emit(protocol.EvTool, protocol.ToolData{Call: call.ToolCallID, Diff: &diff})
	}
}

func displayTool(call toolCall) (protocol.ToolData, []protocol.Diff) {
	data := protocol.ToolData{Call: call.ToolCallID}
	if call.Title != nil {
		data.Title = *call.Title
	}
	if call.Kind != nil {
		data.Kind = normalizeToolKind(*call.Kind)
	}
	if call.Status != nil {
		data.Status = normalizeToolStatus(*call.Status)
	}
	data.Input = clipRunes(strings.Join(strings.Fields(toolInput(call.RawInput)), " "), toolInputRunes)
	if call.Locations != nil {
		for _, location := range *call.Locations {
			data.Locations = append(data.Locations, protocol.Location{Path: location.Path, Line: location.Line})
		}
	}
	var blocks []toolContent
	_ = json.Unmarshal(call.Content, &blocks)
	var texts []string
	var diffs []protocol.Diff
	for _, block := range blocks {
		switch block.Type {
		case "content":
			if block.Content.Type == "text" && block.Content.Text != "" {
				texts = append(texts, block.Content.Text)
			}
		case "diff":
			oldText := ""
			if block.OldText != nil {
				oldText = *block.OldText
			}
			oldClipped, oldCut := clipRunesWithFlag(oldText, toolDiffRunes)
			newClipped, newCut := clipRunesWithFlag(block.NewText, toolDiffRunes)
			diffs = append(diffs, protocol.Diff{Path: block.Path, Old: oldClipped, New: newClipped, Truncated: oldCut || newCut})
		case "terminal":
			data.Terminal = block.TerminalID
		}
	}
	output := rawOutputText(call.RawOutput)
	if output == "" {
		if len(texts) > 0 && data.Input != "" && strings.TrimSpace(texts[0]) == "$ "+data.Input {
			texts = texts[1:]
		}
		output = strings.Join(texts, "\n")
	}
	data.Output = clipRunes(strings.TrimRight(output, "\n"), toolOutputRunes)
	return data, diffs
}

func (r *runner) handleCommands(raw json.RawMessage) {
	var update struct {
		Available []struct {
			Name        string `json:"name"`
			Description string `json:"description"`
			Input       *struct {
				Hint string `json:"hint"`
			} `json:"input"`
		} `json:"availableCommands"`
	}
	if json.Unmarshal(raw, &update) != nil {
		return
	}
	commands := make([]protocol.Command, 0, len(update.Available))
	for _, command := range update.Available {
		hint := ""
		if command.Input != nil {
			hint = command.Input.Hint
		}
		commands = append(commands, protocol.Command{Name: command.Name, Description: command.Description, Hint: hint})
	}
	r.breakChunkRun()
	r.emit(protocol.EvCommands, protocol.CommandsData{Commands: commands})
}

func (r *runner) applySessionState(session sessionResult) {
	if session.Models != nil {
		models := make([]protocol.Choice, 0, len(session.Models.Available))
		for _, model := range session.Models.Available {
			models = append(models, protocol.Choice{ID: model.ModelID, Name: model.Name, Description: model.Description})
		}
		r.setModels("", models, session.Models.Current)
	}
	if session.Modes != nil {
		modes := make([]protocol.Choice, 0, len(session.Modes.Available))
		for _, mode := range session.Modes.Available {
			modes = append(modes, protocol.Choice{ID: mode.ID, Name: mode.Name, Description: mode.Description})
		}
		r.setModes("", modes, session.Modes.Current)
	}
	r.applyConfigOptions(session.ConfigOptions)
}

// applyConfigOptions takes the agent's whole option list, which it sends at
// the start and again whenever one changes. The model and the mode keep their
// own events; every other select becomes a session option. A mode the agent
// also listed the older way is the same choice twice, so the list's copy is
// left out.
func (r *runner) applyConfigOptions(options []configOption) {
	others := make([]protocol.SessionOption, 0, len(options))
	for _, option := range options {
		if option.Type != "select" || option.ID == "" {
			continue
		}
		choices := make([]protocol.Choice, 0, len(option.Options))
		for _, choice := range option.Options {
			description := choice.Description
			if description == choice.Value {
				description = ""
			}
			choices = append(choices, protocol.Choice{ID: choice.Value, Name: choice.Name, Description: description})
		}
		current := string(option.CurrentValue)
		switch option.Category {
		case "model":
			r.setModels(option.ID, choices, current)
		case "mode":
			r.mu.Lock()
			listed := r.modeOption == "" && len(r.modes) != 0
			r.mu.Unlock()
			if !listed {
				r.setModes(option.ID, choices, current)
			}
		default:
			name := option.Name
			if name == "" {
				name = option.ID
			}
			others = append(others, protocol.SessionOption{ID: option.ID, Name: name, Category: option.Category, Current: current, Choices: choices})
		}
	}
	r.mu.Lock()
	changed := !reflect.DeepEqual(r.options, others)
	r.options = others
	r.mu.Unlock()
	if changed {
		r.emit(protocol.EvOptions, protocol.OptionsData{Options: others})
	}
}

// setModels records the agent's models and says so once: an agent repeats its
// whole option list on every change, and a model list runs to hundreds.
func (r *runner) setModels(option string, models []protocol.Choice, current string) {
	r.mu.Lock()
	changed := r.model != current || !reflect.DeepEqual(r.models, models)
	r.modelOption, r.models, r.model = option, models, current
	r.mu.Unlock()
	if changed {
		r.emit(protocol.EvModel, protocol.ChoiceData{Current: current, Options: models})
		r.updateInfo(func(info *protocol.SessionInfo) { info.Model = current })
	}
}

func (r *runner) setModes(option string, modes []protocol.Choice, current string) {
	r.mu.Lock()
	changed := r.mode != current || !reflect.DeepEqual(r.modes, modes)
	r.modeOption, r.modes, r.mode = option, modes, current
	r.mu.Unlock()
	if changed {
		r.emit(protocol.EvMode, protocol.ChoiceData{Current: current, Options: modes})
		r.updateInfo(func(info *protocol.SessionInfo) { info.Mode = current })
	}
}

func normalizeToolKind(kind string) string {
	switch kind {
	case protocol.ToolRead, protocol.ToolEdit, protocol.ToolDelete, protocol.ToolMove,
		protocol.ToolSearch, protocol.ToolExecute, protocol.ToolThink, protocol.ToolFetch:
		return kind
	default:
		return protocol.ToolOther
	}
}

func normalizeToolStatus(status string) string {
	switch status {
	case "pending":
		return protocol.ToolPending
	case "in_progress", "running":
		return protocol.ToolRunning
	case "completed", "done":
		return protocol.ToolDone
	case "failed":
		return protocol.ToolFailed
	default:
		return status
	}
}

func toolInput(raw json.RawMessage) string {
	var values map[string]json.RawMessage
	if json.Unmarshal(raw, &values) != nil {
		return ""
	}
	for _, key := range []string{"command", "path", "pattern", "file_path", "filePath", "query", "url"} {
		value, ok := values[key]
		if !ok {
			continue
		}
		var text string
		if json.Unmarshal(value, &text) == nil && text != "" {
			return text
		}
		var list []string
		if json.Unmarshal(value, &list) == nil && len(list) != 0 {
			return strings.Join(list, " ")
		}
	}
	return ""
}

func rawOutputText(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}
	var text string
	if json.Unmarshal(raw, &text) == nil {
		return text
	}
	var output struct {
		Output  string `json:"output"`
		Stdout  string `json:"stdout"`
		Content []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"content"`
	}
	if json.Unmarshal(raw, &output) != nil {
		return ""
	}
	var parts []string
	for _, block := range output.Content {
		if block.Type == "text" && block.Text != "" {
			parts = append(parts, block.Text)
		}
	}
	if len(parts) != 0 {
		return strings.Join(parts, "\n")
	}
	if output.Output != "" {
		return output.Output
	}
	return output.Stdout
}

func clipRunes(text string, limit int) string {
	clipped, _ := clipRunesWithFlag(text, limit)
	return clipped
}

func clipRunesWithFlag(text string, limit int) (string, bool) {
	if utf8.RuneCountInString(text) <= limit {
		return text, false
	}
	runes := []rune(text)
	return string(runes[:limit]), true
}
