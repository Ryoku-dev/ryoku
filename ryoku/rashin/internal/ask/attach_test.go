package ask

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"ryoku-rashin/internal/core"
	"ryoku-rashin/internal/llm"
	"ryoku-rashin/internal/protocol"
)

var pngHead = append([]byte("\x89PNG\r\n\x1a\n"), bytes.Repeat([]byte{7}, 40)...)

func attached(t *testing.T, name string, data []byte) protocol.Attachment {
	t.Helper()
	path := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	return protocol.Attachment{Path: path}
}

func lastUser(t *testing.T, request llm.Request) llm.Message {
	t.Helper()
	last := request.Messages[len(request.Messages)-1]
	if last.Role != llm.RoleUser {
		t.Fatalf("the last message sent is from %s", last.Role)
	}
	return last
}

// The composer attaches and the transcript shows the attachment under the
// question. Unless the model is handed it too, the answer is about nothing.
func TestAQuestionCarriesWhatWasAttachedToIt(t *testing.T) {
	client := &scriptedClient{streams: []*scriptedStream{finalStream(llm.Delta{Text: "It says the linker failed."})}}
	picture, log := attached(t, "screenshot", pngHead), attached(t, "build.log", []byte("error: linker failed\n"))
	program := attached(t, "tool", []byte("\x7fELF\x02\x01\x00\x00"))
	input := core.PromptInput{Turn: 1, Text: "What went wrong?", Images: []protocol.Attachment{picture, log, program}}
	if _, err := testRunner(t, client).Prompt(context.Background(), input, &recordingSink{}); err != nil {
		t.Fatal(err)
	}
	asked := lastUser(t, client.calls[0])
	if len(asked.Images) != 1 || asked.Images[0].Mime != "image/png" || !bytes.Equal(asked.Images[0].Data, pngHead) {
		t.Fatalf("the model was handed %d pictures: %+v", len(asked.Images), asked.Images)
	}
	if !strings.HasPrefix(asked.Content, "What went wrong?") || strings.Contains(asked.Content, picture.Path) {
		t.Fatalf("the question reads %q", asked.Content)
	}
	if !strings.Contains(asked.Content, log.Path) || !strings.Contains(asked.Content, "error: linker failed") {
		t.Fatalf("the log's words did not go with the question: %q", asked.Content)
	}
	if !strings.Contains(asked.Content, program.Path) || strings.Contains(asked.Content, "ELF") {
		t.Fatalf("a file that is not text should be named and no more: %q", asked.Content)
	}
}

// A whole log would spend a free provider's minute of tokens before the
// question is read, so a long one goes in part and says so.
func TestALongTextFileGoesInPartAndSaysSo(t *testing.T) {
	client := &scriptedClient{streams: []*scriptedStream{finalStream(llm.Delta{Text: "ok"})}}
	long := attached(t, "journal.log", bytes.Repeat([]byte("a line of the journal\n"), 4000))
	input := core.PromptInput{Turn: 1, Text: "Anything odd?", Images: []protocol.Attachment{long}}
	if _, err := testRunner(t, client).Prompt(context.Background(), input, &recordingSink{}); err != nil {
		t.Fatal(err)
	}
	asked := lastUser(t, client.calls[0])
	if len(asked.Content) > attachedTextBytes+1024 || !strings.Contains(asked.Content, "24576 of its 88000 bytes") {
		t.Fatalf("a long log went as %d bytes: %.200q", len(asked.Content), asked.Content)
	}
}

func TestAnAttachmentThatCannotBeReadStopsTheQuestion(t *testing.T) {
	client := &scriptedClient{}
	gone := protocol.Attachment{Path: filepath.Join(t.TempDir(), "gone.png")}
	_, err := testRunner(t, client).Prompt(context.Background(), core.PromptInput{Turn: 1, Text: "Look", Images: []protocol.Attachment{gone}}, &recordingSink{})
	if err == nil || !strings.Contains(err.Error(), gone.Path) || len(client.calls) != 0 {
		t.Fatalf("a question with a missing attachment went on: %v after %d calls", err, len(client.calls))
	}
}

// A follow-up about a picture needs the picture again, and a picture deleted
// since must not end the conversation.
func TestAFollowUpStillHasThePictureItIsAbout(t *testing.T) {
	picture := attached(t, "screenshot.png", pngHead)
	deleted := attached(t, "old.png", pngHead)
	if err := os.Remove(deleted.Path); err != nil {
		t.Fatal(err)
	}
	history := []protocol.SessionEvent{
		historyEvent(t, 1, protocol.EvUser, protocol.UserData{Text: "And this one?", Images: []protocol.Attachment{deleted}}),
		historyEvent(t, 2, protocol.EvText, protocol.TextData{Message: "a", Delta: "A terminal."}),
		historyEvent(t, 3, protocol.EvUser, protocol.UserData{Images: []protocol.Attachment{picture}}),
		historyEvent(t, 4, protocol.EvText, protocol.TextData{Message: "b", Delta: "A build error."}),
	}
	client := &scriptedClient{streams: []*scriptedStream{finalStream(llm.Delta{Text: "Line two."})}}
	input := core.PromptInput{Turn: 3, Text: "Which line?", History: history}
	if _, err := testRunner(t, client).Prompt(context.Background(), input, &recordingSink{}); err != nil {
		t.Fatal(err)
	}
	var users []llm.Message
	for _, message := range client.calls[0].Messages {
		if message.Role == llm.RoleUser {
			users = append(users, message)
		}
	}
	if len(users) != 3 {
		t.Fatalf("the model was sent %d of the reader's 3 turns", len(users))
	}
	if len(users[0].Images) != 0 || !strings.Contains(users[0].Content, "can no longer be read") {
		t.Fatalf("the turn whose picture is gone reads %q with %d pictures", users[0].Content, len(users[0].Images))
	}
	if len(users[1].Images) != 1 || !bytes.Equal(users[1].Images[0].Data, pngHead) {
		t.Fatalf("the turn that was only a picture went back with %d pictures", len(users[1].Images))
	}
}

type refusingClient struct{ reason string }

func (c refusingClient) Stream(context.Context, llm.Request) (llm.Stream, error) {
	return nil, &llm.NoRoute{Reason: c.reason}
}

// The router words its own refusals, with what to do about each. Wrapped in
// Ask's general failure they said to add a key twice.
func TestTheRoutersRefusalIsSaidAsItIs(t *testing.T) {
	const reason = "None of the providers in use reads pictures. These can, with a key added: Sighted Cloud."
	_, err := testRunner(t, refusingClient{reason}).Prompt(context.Background(), core.PromptInput{Turn: 1, Text: "Look"}, &recordingSink{})
	if err == nil || !strings.Contains(err.Error(), reason) || strings.Contains(err.Error(), "Ask could not start") {
		t.Fatalf("the refusal reached the reader as %v", err)
	}
}

func historyEvent(t *testing.T, seq int64, kind string, data any) protocol.SessionEvent {
	t.Helper()
	raw, err := json.Marshal(data)
	if err != nil {
		t.Fatal(err)
	}
	return protocol.SessionEvent{Seq: seq, Kind: kind, Data: raw}
}
