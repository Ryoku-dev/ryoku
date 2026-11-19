package ask

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"ryoku-rashin/internal/core"
	"ryoku-rashin/internal/llm"
	"ryoku-rashin/internal/memory/render"
	"ryoku-rashin/internal/protocol"
)

type captureSink struct {
	events []struct {
		kind string
		data any
	}
}

func (s *captureSink) Emit(kind string, data any) {
	s.events = append(s.events, struct {
		kind string
		data any
	}{kind, data})
}
func (s *captureSink) Update(func(*protocol.SessionInfo)) {}

type captureClient struct{ req llm.Request }

func (c *captureClient) Stream(_ context.Context, req llm.Request) (llm.Stream, error) {
	c.req = req
	return &captureStream{}, nil
}

type captureStream struct{ sent bool }

func (s *captureStream) Next() (llm.Delta, error) {
	if s.sent {
		return llm.Delta{}, io.EOF
	}
	s.sent = true
	return llm.Delta{Text: "read text"}, nil
}
func (s *captureStream) Final() llm.Final {
	return llm.Final{Model: "rashin-models/light", Provider: protocol.ProviderRashinModels, Finish: llm.FinishStop}
}
func (s *captureStream) Close() error { return nil }

func attached(t *testing.T, name string, data []byte) protocol.Attachment {
	t.Helper()
	path := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	return protocol.Attachment{Path: path}
}

func event(t *testing.T, kind string, data any) protocol.SessionEvent {
	t.Helper()
	raw, err := json.Marshal(data)
	if err != nil {
		t.Fatal(err)
	}
	return protocol.SessionEvent{Kind: kind, Data: raw}
}

func TestMessageCarriesPicturesAndTextAndRejectsMissingAttachment(t *testing.T) {
	mode := New(nil)
	png := append([]byte("\x89PNG\r\n\x1a\n"), bytes.Repeat([]byte{7}, 40)...)
	picture := attached(t, "screenshot", png)
	log := attached(t, "build.log", []byte("error: linker failed\n"))
	binary := attached(t, "binary", []byte("\x7fELF\x02\x00"))
	msg, err := mode.Message("What failed?", []protocol.Attachment{picture, log, binary})
	if err != nil {
		t.Fatal(err)
	}
	if len(msg.Images) != 1 || !bytes.Equal(msg.Images[0].Data, png) || !strings.Contains(msg.Content, "error: linker failed") || !strings.Contains(msg.Content, binary.Path) || strings.Contains(msg.Content, "ELF") {
		t.Fatalf("attachment message = %#v", msg)
	}
	missing := protocol.Attachment{Path: filepath.Join(t.TempDir(), "missing.png")}
	if _, err := mode.Message("Look", []protocol.Attachment{missing}); err == nil || !strings.Contains(err.Error(), missing.Path) {
		t.Fatalf("missing attachment error = %v", err)
	}
}

func TestLongTextAttachmentIsBounded(t *testing.T) {
	long := attached(t, "journal.log", bytes.Repeat([]byte("a line of the journal\n"), 4000))
	message, err := New(nil).Message("Anything odd?", []protocol.Attachment{long})
	if err != nil {
		t.Fatal(err)
	}
	if len(message.Content) > attachedTextBytes+1024 || !strings.Contains(message.Content, "24576 of its 88000 bytes") {
		t.Fatalf("large attachment sent %d bytes: %.200q", len(message.Content), message.Content)
	}
}

func TestHistoryRestoresPicturesAndNamesMissingOnes(t *testing.T) {
	png := append([]byte("\x89PNG\r\n\x1a\n"), bytes.Repeat([]byte{7}, 40)...)
	picture := attached(t, "present.png", png)
	missing := attached(t, "deleted.png", png)
	if err := os.Remove(missing.Path); err != nil {
		t.Fatal(err)
	}
	events := []protocol.SessionEvent{
		event(t, protocol.EvUser, protocol.UserData{Text: "And this one?", Images: []protocol.Attachment{missing}}),
		event(t, protocol.EvText, protocol.TextData{Message: "a", Delta: "Old picture."}),
		event(t, protocol.EvUser, protocol.UserData{Images: []protocol.Attachment{picture}}),
		event(t, protocol.EvText, protocol.TextData{Message: "b", Delta: "New picture."}),
	}
	history := New(nil).History(events)
	if len(history) != 4 || !strings.Contains(history[0].Content, "can no longer be read") || len(history[0].Images) != 0 || len(history[2].Images) != 1 || !bytes.Equal(history[2].Images[0].Data, png) {
		t.Fatalf("picture replay = %#v", history)
	}
}

func TestHabitsAreOptionalAndOmitSecretHistory(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("SHELL", "/bin/fish")
	t.Setenv("VISUAL", "")
	t.Setenv("EDITOR", "vim")
	if err := os.WriteFile(filepath.Join(home, ".bash_history"), []byte("git status\ncat ~/.ssh/id_ed25519\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	mode := New(nil)
	if mode.Habits(protocol.Settings{}) != "" {
		t.Fatal("habits were offered while off")
	}
	settings := protocol.Settings{}
	settings.Ask.Habits = true
	habits := mode.Habits(settings)
	if !strings.Contains(habits, "shell: fish") || !strings.Contains(habits, "editor: vim") || !strings.Contains(habits, "git (1)") || strings.Contains(habits, "id_ed25519") {
		t.Fatalf("habits leaked or omitted data: %q", habits)
	}
}

func TestHistoryKeepsMediaTranscriptWithoutReopeningRecording(t *testing.T) {
	user := event(t, protocol.EvUser, protocol.UserData{Text: "Summarize this", Images: []protocol.Attachment{{Path: "/gone/voice.ogg"}}})
	events := []protocol.SessionEvent{
		user,
		event(t, protocol.EvTool, protocol.ToolData{Call: "audio-1-0", Input: "/gone/voice.ogg", Status: protocol.ToolRunning}),
		event(t, protocol.EvTool, protocol.ToolData{Call: "audio-1-0", Status: protocol.ToolDone, Output: "The deadline is Friday."}),
		event(t, protocol.EvText, protocol.TextData{Message: "answer", Delta: "The deadline is Friday."}),
	}
	messages := New(nil).History(events)
	if len(messages) != 2 || !strings.Contains(messages[0].Content, "The deadline is Friday.") || strings.Contains(messages[0].Content, "can no longer be read") || string(events[0].Data) != string(user.Data) {
		t.Fatalf("replayed history = %#v", messages)
	}
}

func TestFilterHoldsBackSplitAgentMarkerAndMemoryFences(t *testing.T) {
	mode := New(nil)
	var text, thought strings.Builder
	filter := mode.Filter(func(s string) { text.WriteString(s) }, func(s string) { thought.WriteString(s) })
	filter.Text("Here is the answer.\n" + render.FenceOpen[:5])
	filter.Text(render.FenceOpen[5:] + "\nprivate memory\n" + render.FenceClose + "\n")
	filter.Text("NEEDS_AG")
	filter.Text("ENT: coding work")
	filter.Thought("reasoning\n" + render.FenceOpen + "\nsecret")
	filter.Close()
	if strings.Contains(text.String(), "private memory") || strings.Contains(text.String(), "NEEDS_AGENT") || strings.Contains(thought.String(), "secret") || !strings.Contains(text.String(), "Here is the answer.") {
		t.Fatalf("visible text %q; thought %q", text.String(), thought.String())
	}
	answer := mode.Answer("Here is the answer.\nNEEDS_AGENT: coding work", text.String(), nil, "test/model")
	if !answer.NeedsAgent || answer.Reason != "coding work" {
		t.Fatalf("answer = %#v", answer)
	}
}

func TestAnswerOffersOnlyUsefulActions(t *testing.T) {
	file := filepath.Join(t.TempDir(), "note.txt")
	if err := os.WriteFile(file, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	answer := New(nil).Answer("", "See "+file+", open https://example.com and run `echo hi`, pick #aabbcc; not /dev/null", nil, "")
	found := map[string]bool{}
	for _, action := range answer.Actions {
		if action.Value == "/dev/null" {
			t.Fatal("device offered as a file")
		}
		found[action.Kind] = true
	}
	for _, kind := range []string{protocol.ActFile, protocol.ActURL, protocol.ActCmd, protocol.ActColor} {
		if !found[kind] {
			t.Fatalf("missing action %s: %#v", kind, answer.Actions)
		}
	}
}

func TestGlanceUsesLightModelWithoutPinAndKeepsRequestedModel(t *testing.T) {
	client := &captureClient{}
	mode := New(client)
	sink := &captureSink{}
	in := core.PromptInput{Turn: 1, Text: "Capture text", Glance: &protocol.GlanceParams{Mode: "read", Local: true}}
	if _, err := mode.Glance(context.Background(), in, "", false, sink); err != nil {
		t.Fatal(err)
	}
	if !client.req.Light || client.req.Model != "" || !client.req.LocalOnly || len(client.req.Tools) != 0 || len(client.req.Messages) != 2 || client.req.Messages[0].Content != protocol.GlanceRead {
		t.Fatalf("light glance request = %#v", client.req)
	}
	if len(sink.events) != 3 || sink.events[0].kind != protocol.EvText || sink.events[2].kind != protocol.EvAnswer {
		t.Fatalf("glance events = %#v", sink.events)
	}
	in.Glance.Model = "rashin-models/light"
	if _, err := mode.Glance(context.Background(), in, "", false, sink); err != nil {
		t.Fatal(err)
	}
	if client.req.Light || !client.req.Strict || client.req.Model != "rashin-models/light" {
		t.Fatalf("pinned glance request = %#v", client.req)
	}
}
