package daemon

import (
	"context"
	"reflect"
	"testing"

	"ryoku-rashin/internal/core"
	"ryoku-rashin/internal/protocol"
)

func TestAskMediaOffLeavesAttachmentsAndQuestionUntouched(t *testing.T) {
	d := &Daemon{}
	input := core.PromptInput{Text: "Summarize https://youtu.be/example", Images: []protocol.Attachment{{Path: "/missing/voice.wav"}}}
	got, err := d.prepareAskMedia(context.Background(), protocol.SessionInfo{Kind: protocol.KindAsk}, input, nil)
	if err != nil || !reflect.DeepEqual(got, input) {
		t.Fatalf("disabled media changed the prompt: %#v, %v", got, err)
	}
}

func TestAskMediaNeverInterceptsAgentOrGlanceTurns(t *testing.T) {
	d := &Daemon{settings: protocol.Settings{Features: map[string]bool{"audio": true, "video": true}}}
	for _, test := range []struct {
		name  string
		kind  protocol.SessionKind
		input core.PromptInput
	}{
		{"agent", protocol.KindAgent, core.PromptInput{Text: "https://youtu.be/example"}},
		{"glance", protocol.KindAsk, core.PromptInput{Glance: &protocol.GlanceParams{}, Images: []protocol.Attachment{{Path: "/missing/voice.wav"}}}},
		{"theia", protocol.KindAsk, core.PromptInput{Origin: protocol.OriginSight, Text: "https://youtu.be/example"}},
		{"command", protocol.KindAsk, core.PromptInput{Run: "true", Images: []protocol.Attachment{{Path: "/missing/voice.wav"}}}},
	} {
		t.Run(test.name, func(t *testing.T) {
			got, err := d.prepareAskMedia(context.Background(), protocol.SessionInfo{Kind: test.kind}, test.input, nil)
			if err != nil || !reflect.DeepEqual(got, test.input) {
				t.Fatalf("non-Ask turn intercepted: %#v, %v", got, err)
			}
		})
	}
}
