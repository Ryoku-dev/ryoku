package ask

import (
	"encoding/json"
	"strings"
	"testing"

	"ryoku-rashin/internal/protocol"
)

func TestMediaTranscriptsRemainInFollowupsWithoutReopeningRecordings(t *testing.T) {
	marshal := func(data any) json.RawMessage {
		t.Helper()
		raw, err := json.Marshal(data)
		if err != nil {
			t.Fatal(err)
		}
		return raw
	}
	user := marshal(protocol.UserData{Text: "Summarize this", Images: []protocol.Attachment{{Path: "/gone/voice.ogg"}}})
	events := []protocol.SessionEvent{
		{Kind: protocol.EvUser, Data: user},
		{Kind: protocol.EvTool, Data: marshal(protocol.ToolData{Call: "audio-1-0", Input: "/gone/voice.ogg", Status: protocol.ToolRunning})},
		{Kind: protocol.EvTool, Data: marshal(protocol.ToolData{Call: "audio-1-0", Status: protocol.ToolDone, Output: "The deadline is Friday."})},
		{Kind: protocol.EvText, Data: marshal(protocol.TextData{Message: "answer", Delta: "The deadline is Friday."})},
	}
	messages := historyMessages(events, 10000)
	if len(messages) != 2 || !strings.Contains(messages[0].Content, "The deadline is Friday.") || strings.Contains(messages[0].Content, "can no longer be read") {
		t.Fatalf("followup lost its transcript: %#v", messages)
	}
	if string(events[0].Data) != string(user) {
		t.Fatal("history conversion changed the stored user event")
	}
}
