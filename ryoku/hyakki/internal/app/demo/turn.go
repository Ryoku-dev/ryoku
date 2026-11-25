package demo

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	"ryoku-i18n"

	"ryoku-rashin/internal/protocol"
)

// one writer per client, the request loop's answers and a playing turn's events both go through it
type outbox struct {
	mu      sync.Mutex
	encoder *protocol.Encoder
}

func (out *outbox) send(value any) error {
	out.mu.Lock()
	defer out.mu.Unlock()
	return out.encoder.Encode(value)
}

type turnStep struct {
	after time.Duration
	kind  string
	data  any
}

// the demo has no model so the answer is the same whatever is asked, it's here to show a turn move
func (s *server) playTurn(ctx context.Context, out *outbox, raw json.RawMessage) {
	var params protocol.PromptParams
	if decode(raw, &params) != nil || params.ID == "" {
		return
	}
	s.mu.Lock()
	info, known := s.session(params.ID)
	seq := int64(len(s.data.events[params.ID]))
	s.mu.Unlock()
	if !known {
		return
	}
	// named after its first question like a real session
	title := ""
	if info.Title == "" || info.Title == i18n.T("New session") {
		title = firstWords(params.Text, 6)
	}
	s.setState(out, info.ID, protocol.StateWorking, title)
	for _, step := range turnSteps(params, seq) {
		select {
		case <-ctx.Done():
			return
		case <-time.After(step.after):
		}
		seq++
		data, _ := json.Marshal(step.data)
		played := protocol.SessionEvent{Seq: seq, TS: time.Now(), Kind: step.kind, Data: data}
		s.mu.Lock()
		s.data.events[info.ID] = append(s.data.events[info.ID], played)
		s.mu.Unlock()
		wire, _ := json.Marshal(protocol.SessionEventData{Session: info.ID, SessionEvent: played})
		if out.send(protocol.Event{Event: protocol.EvtSessionEvent, Data: wire}) != nil {
			return
		}
	}
	s.setState(out, info.ID, protocol.StateIdle, "")
}

func (s *server) setState(out *outbox, id string, state protocol.SessionState, title string) {
	s.mu.Lock()
	info, known := s.session(id)
	if known {
		info.State, info.Alive, info.Updated = state, state == protocol.StateWorking, time.Now()
		if title != "" {
			info.Title = title
		}
		s.replaceSession(info)
	}
	s.mu.Unlock()
	if !known {
		return
	}
	wire, _ := json.Marshal(info)
	_ = out.send(protocol.Event{Event: protocol.EvtSessionInfo, Data: wire})
}

func turnSteps(params protocol.PromptParams, seq int64) []turnStep {
	// ids carry the sequence they start at so a second turn doesn't write into the first one's messages and calls
	id := func(name string) string { return fmt.Sprintf("%s-%d", name, seq) }
	if params.Run != "" {
		return runSteps(params.Run, id("run"), id("answer"))
	}
	usage, folders := id("usage"), id("folders")
	steps := []turnStep{
		{0, protocol.EvUser, protocol.UserData{Text: params.Text, Images: params.Images}},
		{0, protocol.EvTurnStart, protocol.TurnStartData{Turn: 1}},
		{300 * time.Millisecond, protocol.EvThought, protocol.TextData{
			Message: id("thought"), Delta: i18n.T("Reading how the disk is used before answering."),
		}},
		{350 * time.Millisecond, protocol.EvTool, protocol.ToolData{
			Call: usage, Title: i18n.T("Read disk usage"), Kind: protocol.ToolExecute, Status: protocol.ToolRunning, Input: "df -h /",
		}},
		{650 * time.Millisecond, protocol.EvTool, protocol.ToolData{
			Call: usage, Title: i18n.T("Read disk usage"), Kind: protocol.ToolExecute, Status: protocol.ToolDone, Input: "df -h /",
			Output: "/dev/nvme0n1p2  476G  412G   40G  92% /",
		}},
		{150 * time.Millisecond, protocol.EvTool, protocol.ToolData{
			Call: folders, Title: i18n.T("Find the largest folders"), Kind: protocol.ToolSearch, Status: protocol.ToolRunning, Input: "~/.cache",
		}},
		{700 * time.Millisecond, protocol.EvTool, protocol.ToolData{
			Call: folders, Title: i18n.T("Find the largest folders"), Kind: protocol.ToolSearch, Status: protocol.ToolDone, Input: "~/.cache",
			Output: "38G  ~/.cache/go-build\n21G  ~/.cache/yay\n9G   ~/.cache/ryoku",
		}},
	}
	answer := i18n.T("Your disk is **92% full**, and most of what can go is cache.\n\n" +
		"1. The Go build cache holds 38 GB. `go clean -cache` empties it.\n" +
		"2. Package build folders hold 21 GB.\n" +
		"3. The rest is spread thin.\n\n" +
		"Nothing here needs you to delete your own files.")
	message := id("answer")
	for index, piece := range strings.SplitAfter(answer, " ") {
		wait := 40 * time.Millisecond
		if index == 0 {
			wait = 250 * time.Millisecond
		}
		steps = append(steps, turnStep{wait, protocol.EvText, protocol.TextData{Message: message, Delta: piece}})
	}
	return append(steps,
		turnStep{120 * time.Millisecond, protocol.EvAnswer, protocol.AnswerData{Via: "groq/llama-3.3-70b"}},
		turnStep{0, protocol.EvTurnEnd, protocol.TurnEndData{
			Turn: 1, Stop: protocol.StopEnd, Usage: &protocol.TokenUsage{Input: 1840, Output: 96},
		}},
	)
}

// a command the reader ran: its output comes in pieces, then the answer made from it
func runSteps(command, run, message string) []turnStep {
	exit := 0
	return []turnStep{
		{0, protocol.EvUser, protocol.UserData{Run: command}},
		{0, protocol.EvTurnStart, protocol.TurnStartData{Turn: 1}},
		{0, protocol.EvTool, protocol.ToolData{
			Call: run, Title: i18n.T("Running"), Kind: protocol.ToolExecute, Status: protocol.ToolRunning, Terminal: run,
		}},
		{200 * time.Millisecond, protocol.EvTerminal, protocol.TerminalData{Terminal: run, Delta: "system   indexed  4,812 files\n"}},
		{300 * time.Millisecond, protocol.EvTerminal, protocol.TerminalData{Terminal: run, Delta: "hyperion indexing 438 of 5,790 files\n"}},
		{100 * time.Millisecond, protocol.EvTerminal, protocol.TerminalData{Terminal: run, Exit: &exit, Done: true}},
		{0, protocol.EvTool, protocol.ToolData{Call: run, Title: i18n.Tf("Ran in %d s", 1), Status: protocol.ToolDone}},
		{250 * time.Millisecond, protocol.EvText, protocol.TextData{Message: message, Delta: i18n.T("The hyperion index is still being built: 438 of 5,790 files are in.")}},
		{120 * time.Millisecond, protocol.EvAnswer, protocol.AnswerData{Via: "groq/llama-3.3-70b"}},
		{0, protocol.EvTurnEnd, protocol.TurnEndData{Turn: 1, Stop: protocol.StopEnd, Usage: &protocol.TokenUsage{Input: 640, Output: 22}}},
	}
}

func firstWords(text string, count int) string {
	words := strings.Fields(text)
	if len(words) > count {
		words = words[:count]
	}
	return strings.Join(words, " ")
}
