package daemon

import (
	"bytes"
	"context"
	"encoding/json"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"ryoku-rashin/internal/core"
	"ryoku-rashin/internal/paths"
	"ryoku-rashin/internal/protocol"
)

func TestGlanceValidation(t *testing.T) {
	for _, p := range []protocol.GlanceParams{
		{Mode: "unknown", Text: "text"},
		{Mode: "explain"},
		{Mode: "ask", Text: "text"},
	} {
		if err := validateGlance(p); protocol.AsError(err) == nil || protocol.AsError(err).Code != protocol.CodeBadRequest {
			t.Fatalf("validation accepted %#v: %v", p, err)
		}
	}
	if err := validateGlance(protocol.GlanceParams{Mode: "translate", Text: "hello"}); err != nil {
		t.Fatal(err)
	}
}

func TestGlanceCreatesTemporarySightSessionAndStreams(t *testing.T) {
	isolateDaemon(t)
	runner := &fakeRunner{prompt: func(_ context.Context, in core.PromptInput, sink core.Sink) (core.TurnResult, error) {
		if in.Origin != protocol.OriginSight || in.Glance == nil || in.Glance.Mode != "explain" || len(in.Images) != 1 {
			t.Errorf("capture input = %#v", in)
		}
		sink.Emit(protocol.EvText, protocol.TextData{Message: "capture", Delta: "a capture"})
		return core.TurnResult{Stop: protocol.StopEnd}, nil
	}}
	settings := func() protocol.Settings { return protocol.Settings{} }
	m := newSessionManager(fakeAsker{runner: runner}, nil, nil, settings, func(string, any) {}, func() {}, nil)
	d := &Daemon{manager: m}
	c := &connection{outbound: make(chan []byte, 64)}
	result, err := d.glance(context.Background(), c, protocol.GlanceParams{Mode: "explain", Images: []protocol.Attachment{{Path: "/capture.png"}}})
	if err != nil {
		t.Fatal(err)
	}
	if !result.Session.Temporary || result.Session.Origin != protocol.OriginSight || result.Session.Model != "" || result.Turn != 1 {
		t.Fatalf("capture result = %#v", result)
	}
	events := glanceTurnEvents(t, c)
	text := false
	for _, event := range events {
		text = text || event.Kind == protocol.EvText
	}
	if !text || events[len(events)-1].Kind != protocol.EvTurnEnd {
		t.Fatalf("capture events = %#v", events)
	}
}

func glanceTurnEvents(t *testing.T, c *connection) []protocol.SessionEventData {
	t.Helper()
	var events []protocol.SessionEventData
	timer := time.NewTimer(3 * time.Second)
	defer timer.Stop()
	for {
		select {
		case raw := <-c.outbound:
			var frame protocol.Frame
			if json.Unmarshal(raw, &frame) != nil || frame.Event != protocol.EvtSessionEvent {
				continue
			}
			var event protocol.SessionEventData
			if json.Unmarshal(frame.Data, &event) != nil {
				t.Fatal("invalid event")
			}
			events = append(events, event)
			if event.Kind == protocol.EvTurnEnd {
				return events
			}
		case <-timer.C:
			t.Fatal("capture turn did not end")
		}
	}
}

func TestGlanceKeepsExplicitModelOrUsesLightRoute(t *testing.T) {
	isolateDaemon(t)
	for _, model := range []string{"provider/model", ""} {
		seen := make(chan core.PromptInput, 1)
		runner := &fakeRunner{prompt: func(_ context.Context, in core.PromptInput, _ core.Sink) (core.TurnResult, error) {
			seen <- in
			return core.TurnResult{Stop: protocol.StopEnd}, nil
		}}
		settings := func() protocol.Settings {
			return protocol.Settings{Ask: protocol.AskSettings{Model: "unrelated-provider/model"}}
		}
		m := newSessionManager(fakeAsker{runner: runner}, nil, nil, settings, func(string, any) {}, func() {}, nil)
		d := &Daemon{manager: m}
		c := &connection{outbound: make(chan []byte, 64)}
		result, err := d.glance(context.Background(), c, protocol.GlanceParams{Mode: "read", Text: "capture text", Model: model})
		if err != nil {
			t.Fatal(err)
		}
		glanceTurnEvents(t, c)
		if result.Session.Model != model || result.Session.Origin != protocol.OriginSight {
			t.Fatalf("capture model = %q, origin = %q", result.Session.Model, result.Session.Origin)
		}
		if input := <-seen; input.Text != "capture text" || input.Glance == nil || input.Glance.Mode != "read" {
			t.Fatalf("capture input = %#v", input)
		}
	}
}

func TestForkKeepsRuntimeImagesButNotTheTemporarySession(t *testing.T) {
	manager, _ := testManager(t, &fakeRunner{})
	runtime := os.Getenv("XDG_RUNTIME_DIR")
	if err := os.MkdirAll(runtime, 0o700); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(runtime, "capture.png")
	var picture bytes.Buffer
	if err := png.Encode(&picture, image.NewRGBA(image.Rect(0, 0, 4, 4))); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, picture.Bytes(), 0o600); err != nil {
		t.Fatal(err)
	}
	info, err := manager.create(protocol.SessionNewParams{Kind: protocol.KindAsk, Origin: protocol.OriginSight, Temporary: true})
	if err != nil {
		t.Fatal(err)
	}
	source, _ := manager.get(info.ID)
	if err := source.coalescer.Emit(protocol.EvUser, protocol.UserData{Text: "look", Images: []protocol.Attachment{{Path: path}}}); err != nil {
		t.Fatal(err)
	}
	info = source.log.Info()
	fork, err := manager.fork(context.Background(), protocol.ForkParams{ID: info.ID, Seq: info.LastSeq})
	if err != nil {
		t.Fatal(err)
	}
	if fork.Temporary {
		t.Fatal("kept copy stayed temporary")
	}
	kept, _ := manager.get(fork.ID)
	events, _ := kept.log.Since(0)
	var user protocol.UserData
	if err := json.Unmarshal(events[0].Data, &user); err != nil {
		t.Fatal(err)
	}
	if len(user.Images) != 1 || !strings.HasPrefix(user.Images[0].Path, paths.AttachmentsDir()+string(filepath.Separator)) {
		t.Fatalf("kept attachments = %#v", user.Images)
	}
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(user.Images[0].Path)
	if err != nil || !bytes.Equal(data, picture.Bytes()) {
		t.Fatalf("kept image did not survive capture removal: %v", err)
	}
	sourceEvents, _ := source.log.Since(0)
	var original protocol.UserData
	_ = json.Unmarshal(sourceEvents[0].Data, &original)
	if original.Images[0].Path != path {
		t.Fatal("keeping rewrote the temporary session")
	}
}
