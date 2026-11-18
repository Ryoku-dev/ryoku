package ask

import (
	"context"
	"testing"

	"ryoku-rashin/internal/core"
	"ryoku-rashin/internal/llm"
	"ryoku-rashin/internal/protocol"
)

func TestGlanceSendsOnlyTheModePromptAndNoTools(t *testing.T) {
	client := &scriptedClient{streams: []*scriptedStream{finalStream(llm.Delta{Text: "read text"})}}
	runner := testRunner(t, client)
	sink := &recordingSink{}
	_, err := runner.Prompt(context.Background(), core.PromptInput{Turn: 1, Text: "capture text", Images: []protocol.Attachment{attached(t, "capture.png", pngHead)}, Glance: &protocol.GlanceParams{Mode: "read", Model: protocol.RouteFast, Local: true}}, sink)
	if err != nil {
		t.Fatal(err)
	}
	request := client.calls[0]
	if len(request.Tools) != 0 || len(request.Messages) != 2 || request.Messages[0].Role != llm.RoleSystem || request.Messages[0].Content != protocol.GlanceRead {
		t.Fatalf("quick request = %#v", request)
	}
	if !request.LocalOnly || request.Model != protocol.RouteFast || request.Messages[1].Content != "capture text" || len(request.Messages[1].Images) != 1 {
		t.Fatalf("quick routing = %#v", request)
	}
	if len(sink.events) != 3 || sink.events[0].kind != protocol.EvText || sink.events[2].kind != protocol.EvAnswer {
		t.Fatalf("quick events = %#v", sink.events)
	}
}

func TestNoVisionKeepsItsReadableErrorAndCode(t *testing.T) {
	err := modelError(&llm.NoRoute{Reason: "None of the providers reads pictures.", NoVision: true})
	wire := protocol.AsError(err)
	if wire.Code != protocol.CodeNoVision || wire.Message != "None of the providers reads pictures." {
		t.Fatalf("error = %#v", wire)
	}
}
