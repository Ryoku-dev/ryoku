package ask

import (
	"context"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	"ryoku-rashin/internal/core"
	"ryoku-rashin/internal/llm"
	"ryoku-rashin/internal/memory/render"
	"ryoku-rashin/internal/protocol"
)

// fakeMemory recalls what it was given for a session that uses memory and
// nothing for one that does not, and records the memory tools' calls.
type fakeMemory struct {
	core.Memory
	recall   core.Recall
	asked    []string
	handled  []string
	callers  []core.MemoryCaller
	answered any
}

func (m *fakeMemory) Recall(_ context.Context, info protocol.SessionInfo, question string) core.Recall {
	m.asked = append(m.asked, question)
	if info.Memory == protocol.MemoryNone {
		return core.Recall{}
	}
	return m.recall
}

func (m *fakeMemory) Handle(_ context.Context, from core.MemoryCaller, method string, _ json.RawMessage) (any, error) {
	m.handled = append(m.handled, method)
	m.callers = append(m.callers, from)
	return m.answered, nil
}

func memoryRunner(t *testing.T, client llm.Client, memory core.Memory, info protocol.SessionInfo) core.Runner {
	t.Helper()
	root := t.TempDir()
	t.Setenv("HOME", root)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(root, "config"))
	t.Setenv("XDG_STATE_HOME", filepath.Join(root, "state"))
	t.Setenv("XDG_DATA_HOME", filepath.Join(root, "data"))
	t.Setenv("XDG_CACHE_HOME", filepath.Join(root, "cache"))
	return New(client, nil, nil, memory).NewAsk(core.AskOptions{
		Session:  info,
		Settings: func() protocol.Settings { return protocol.Settings{Ask: protocol.AskSettings{Model: "auto"}} },
	})
}

func fenced(fact string) string {
	return render.FenceOpen + "\n- " + fact + "\n" + render.FenceClose
}

func toolNames(request llm.Request) map[string]bool {
	names := map[string]bool{}
	for _, tool := range request.Tools {
		names[tool.Name] = true
	}
	return names
}

func eventJSON(t *testing.T, kind string, data any) protocol.SessionEvent {
	t.Helper()
	raw, err := json.Marshal(data)
	if err != nil {
		t.Fatal(err)
	}
	return protocol.SessionEvent{Kind: kind, Data: raw}
}

func TestRecallGoesIntoThePromptAndOnlyThisTurnsMessage(t *testing.T) {
	memory := &fakeMemory{recall: core.Recall{
		System:   "# Memory\n" + render.Card("The user writes Go on a Ryoku laptop."),
		Context:  fenced("The user's shell is fish."),
		Used:     &protocol.MemoryEventData{Kind: protocol.MemoryUsed, Card: true, Used: []protocol.MemoryItem{{ID: "m0000001", Text: "The user's shell is fish."}}},
		Tools:    true,
		Sessions: true,
	}}
	client := &scriptedClient{streams: []*scriptedStream{
		finalStream(llm.Delta{Text: "Fish."}),
		finalStream(llm.Delta{Text: "Helix."}),
	}}
	runner := memoryRunner(t, client, memory, protocol.SessionInfo{ID: "ask-memory", Model: "auto"})
	sink := &recordingSink{}
	if _, err := runner.Prompt(context.Background(), core.PromptInput{Turn: 1, Text: "Which shell do I use?"}, sink); err != nil {
		t.Fatal(err)
	}
	first := client.calls[0]
	if system := first.Messages[0].Content; !strings.Contains(system, "<user_card") || !strings.Contains(system, "Ryoku laptop") {
		t.Fatalf("the card is not in the system prompt:\n%s", system)
	}
	asked := first.Messages[len(first.Messages)-1]
	if asked.Role != llm.RoleUser || !strings.HasPrefix(asked.Content, "Which shell do I use?") || !strings.Contains(asked.Content, fenced("The user's shell is fish.")) {
		t.Fatalf("this turn's message = %q", asked.Content)
	}
	for _, name := range []string{"memory_search", "remember", "forget", "session_search"} {
		if !toolNames(first)[name] {
			t.Fatalf("%s is not offered: %v", name, toolNames(first))
		}
	}
	if len(sink.events) == 0 || sink.events[0].kind != protocol.EvMemory {
		t.Fatalf("the used line does not come before the answer: %+v", sink.events)
	}
	if used := sink.events[0].data.(protocol.MemoryEventData); used.Kind != protocol.MemoryUsed || !used.Card || len(used.Used) != 1 {
		t.Fatalf("used = %+v", used)
	}

	// the next turn is built from the stored events, which hold what the
	// user typed and never the fence
	memory.recall.Context = fenced("The user's editor is Helix.")
	history := []protocol.SessionEvent{
		eventJSON(t, protocol.EvUser, protocol.UserData{Text: "Which shell do I use?"}),
		eventJSON(t, protocol.EvText, protocol.TextData{Message: "ask-1-1", Delta: "Fish."}),
	}
	if _, err := runner.Prompt(context.Background(), core.PromptInput{Turn: 2, Text: "And my editor?", History: history}, &recordingSink{}); err != nil {
		t.Fatal(err)
	}
	second := client.calls[1]
	fences := 0
	for index, message := range second.Messages {
		if strings.Contains(message.Content, render.FenceOpen) {
			fences++
			if index != len(second.Messages)-1 {
				t.Fatalf("message %d of the history carries a fence: %q", index, message.Content)
			}
		}
	}
	if fences != 1 || !strings.Contains(second.Messages[len(second.Messages)-1].Content, "Helix") {
		t.Fatalf("the second turn carries %d fences: %+v", fences, second.Messages)
	}
	if strings.Join(memory.asked, "|") != "Which shell do I use?|And my editor?" {
		t.Fatalf("recall was asked %q", memory.asked)
	}
}

func TestReasoningNeverShowsTheFence(t *testing.T) {
	memory := &fakeMemory{recall: core.Recall{Context: fenced("The user's shell is fish.")}}
	client := &scriptedClient{streams: []*scriptedStream{finalStream(
		llm.Delta{Reasoning: "The context says:\n<memory-con"},
		llm.Delta{Reasoning: "text>\n- The user's shell is fish.\n</memory-context>\nSo fish."},
		llm.Delta{Text: "You use fish."},
	)}}
	sink := &recordingSink{}
	if _, err := memoryRunner(t, client, memory, protocol.SessionInfo{ID: "ask-thought", Model: "auto"}).Prompt(context.Background(), core.PromptInput{Turn: 1, Text: "Which shell?"}, sink); err != nil {
		t.Fatal(err)
	}
	var thought strings.Builder
	for _, event := range sink.events {
		if event.kind == protocol.EvThought {
			thought.WriteString(event.data.(protocol.TextData).Delta)
		}
	}
	if shown := thought.String(); strings.Contains(shown, "memory-context") || strings.Contains(shown, "shell is fish") || !strings.Contains(shown, "So fish.") {
		t.Fatalf("reasoning shown = %q", shown)
	}
}

func TestAnAnswerNeverShowsTheFence(t *testing.T) {
	memory := &fakeMemory{recall: core.Recall{Context: fenced("The user's shell is fish.")}}
	client := &scriptedClient{streams: []*scriptedStream{finalStream(
		llm.Delta{Text: "Here is what I recall.\n<memory-"},
		llm.Delta{Text: "context>\n- The user's shell is fish.\n</memory-con"},
		llm.Delta{Text: "text>\nYou use fish."},
	)}}
	sink := &recordingSink{}
	if _, err := memoryRunner(t, client, memory, protocol.SessionInfo{ID: "ask-fence", Model: "auto"}).Prompt(context.Background(), core.PromptInput{Turn: 1, Text: "Which shell?"}, sink); err != nil {
		t.Fatal(err)
	}
	var visible strings.Builder
	for _, event := range sink.events {
		if event.kind == protocol.EvText {
			visible.WriteString(event.data.(protocol.TextData).Delta)
		}
	}
	if shown := visible.String(); strings.Contains(shown, "memory-context") || strings.Contains(shown, "shell is fish") || !strings.Contains(shown, "You use fish.") {
		t.Fatalf("shown = %q", shown)
	}
}

func TestMemoryToolsOnlyWhenRecallOffersThem(t *testing.T) {
	memory := &fakeMemory{}
	client := &scriptedClient{streams: []*scriptedStream{finalStream(llm.Delta{Text: "Hi."})}}
	sink := &recordingSink{}
	if _, err := memoryRunner(t, client, memory, protocol.SessionInfo{ID: "ask-off", Model: "auto"}).Prompt(context.Background(), core.PromptInput{Turn: 1, Text: "Hello there"}, sink); err != nil {
		t.Fatal(err)
	}
	request := client.calls[0]
	for _, name := range []string{"memory_search", "remember", "forget", "session_search"} {
		if toolNames(request)[name] {
			t.Fatalf("%s offered while memory gives nothing", name)
		}
	}
	if strings.Contains(request.Messages[0].Content, "# Memory") {
		t.Fatal("the system prompt has a memory section while memory gives nothing")
	}
}

func TestATemporaryConversationWithoutMemoryGetsNothing(t *testing.T) {
	memory := &fakeMemory{recall: core.Recall{System: "# Memory\nrules", Context: fenced("a fact"), Used: &protocol.MemoryEventData{Kind: protocol.MemoryUsed}, Tools: true, Sessions: true}}
	client := &scriptedClient{streams: []*scriptedStream{finalStream(llm.Delta{Text: "Hi."})}}
	sink := &recordingSink{}
	info := protocol.SessionInfo{ID: "ask-temporary", Model: "auto", Temporary: true, Memory: protocol.MemoryNone}
	if _, err := memoryRunner(t, client, memory, info).Prompt(context.Background(), core.PromptInput{Turn: 1, Text: "What do you know of me?"}, sink); err != nil {
		t.Fatal(err)
	}
	request := client.calls[0]
	if len(toolNames(request)) != len(askTools(protocol.ApprovalsReadOnly)) {
		t.Fatalf("tools = %v", toolNames(request))
	}
	for _, message := range request.Messages {
		if strings.Contains(message.Content, "# Memory") || strings.Contains(message.Content, render.FenceOpen) {
			t.Fatalf("a message carries memory: %q", message.Content)
		}
	}
	for _, event := range sink.events {
		if event.kind == protocol.EvMemory {
			t.Fatalf("a memory line in a conversation without memory: %+v", event)
		}
	}
}

func TestRememberRunsAsTheConversationAndShowsNoToolRow(t *testing.T) {
	memory := &fakeMemory{
		recall:   core.Recall{Tools: true},
		answered: protocol.MemoryChange{Items: []protocol.MemoryItem{{ID: "m0000002", Text: "The user prefers tabs.", Op: protocol.MemoryOpRemembered}}},
	}
	call := llm.ToolCall{ID: "call-1", Name: "remember", Arguments: `{"text":"The user prefers tabs.","scope":"you"}`}
	client := &scriptedClient{streams: []*scriptedStream{
		finalStream(llm.Delta{ToolCalls: []llm.ToolCall{call}}),
		finalStream(llm.Delta{Text: "I will remember that."}),
	}}
	sink := &recordingSink{}
	if _, err := memoryRunner(t, client, memory, protocol.SessionInfo{ID: "ask-remember", Model: "auto"}).Prompt(context.Background(), core.PromptInput{Turn: 1, Text: "Remember that I prefer tabs."}, sink); err != nil {
		t.Fatal(err)
	}
	if len(memory.handled) != 1 || memory.handled[0] != protocol.MMemoryAdd || memory.callers[0] != (core.MemoryCaller{Session: "ask-remember"}) {
		t.Fatalf("memory heard %v from %+v", memory.handled, memory.callers)
	}
	for _, event := range sink.events {
		if event.kind == protocol.EvTool {
			t.Fatalf("remember showed as a tool: %+v", event)
		}
	}
	answer := client.calls[1].Messages[len(client.calls[1].Messages)-1]
	if answer.Role != llm.RoleTool || !strings.Contains(answer.Content, "Remembered: The user prefers tabs.") {
		t.Fatalf("the model was told %q", answer.Content)
	}
}
