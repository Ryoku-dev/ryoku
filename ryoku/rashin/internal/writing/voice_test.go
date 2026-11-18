package writing

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"ryoku-rashin/internal/paths"
	"ryoku-rashin/internal/protocol"
)

func TestVoiceStoreRoundTrip(t *testing.T) {
	t.Setenv("XDG_DATA_HOME", t.TempDir())
	voice, err := VoiceGet()
	if err != nil || voice != (protocol.WritingVoice{}) {
		t.Fatalf("missing voice=%#v, error=%v", voice, err)
	}
	samples := "Café plans look good 🙂. I'll send them tomorrow."
	notes := "Plain words, short sentences."
	written, err := VoiceSet(protocol.WritingVoiceSetParams{Samples: &samples, Notes: &notes})
	if err != nil {
		t.Fatal(err)
	}
	got, err := VoiceGet()
	if err != nil || got != written || got.Samples != samples || got.Notes != notes || got.Updated == "" {
		t.Fatalf("got=%#v, want=%#v, error=%v", got, written, err)
	}
	if VoiceNotes() != notes {
		t.Fatalf("voice notes=%q", VoiceNotes())
	}
	info, err := os.Stat(filepath.Join(paths.DataDir(), "voice.json"))
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0600 {
		t.Fatalf("voice permission=%o", info.Mode().Perm())
	}
	empty := ""
	cleared, err := VoiceSet(protocol.WritingVoiceSetParams{Notes: &empty})
	if err != nil || cleared.Samples != samples || cleared.Notes != "" || VoiceNotes() != "" {
		t.Fatalf("partial update=%#v, error=%v", cleared, err)
	}
}

func TestVoiceLearnBoundedAndUsedInRewrite(t *testing.T) {
	t.Setenv("XDG_DATA_HOME", t.TempDir())
	samples := "A quick update, the draft is ready. Let me know what you think."
	if _, err := VoiceSet(protocol.WritingVoiceSetParams{Samples: &samples}); err != nil {
		t.Fatal(err)
	}
	client := &testClient{reply: strings.Repeat("é", 650)}
	voice, err := VoiceLearn(context.Background(), client, protocol.WritingVoiceLearnParams{Local: true})
	if err != nil || len([]rune(voice.Notes)) != 600 || !client.request.LocalOnly || client.calls != 1 {
		t.Fatalf("voice=%#v, error=%v", voice, err)
	}
	instruction, err := transformInstruction(protocol.WritingTransformParams{Action: "rewrite", Options: map[string]string{"voice": "on"}})
	if err != nil || !strings.Contains(instruction, voice.Notes) {
		t.Fatalf("voice missing from rewrite instruction: %s, %v", instruction, err)
	}
}

func TestVoiceRejectsLongNotes(t *testing.T) {
	t.Setenv("XDG_DATA_HOME", t.TempDir())
	notes := strings.Repeat("x", 601)
	if _, err := VoiceSet(protocol.WritingVoiceSetParams{Notes: &notes}); err == nil {
		t.Fatal("accepted notes beyond 600 characters")
	}
	client := &testClient{}
	if _, err := VoiceLearn(context.Background(), client, protocol.WritingVoiceLearnParams{}); err == nil || client.calls != 0 {
		t.Fatalf("learning without samples: error=%v, calls=%d", err, client.calls)
	}
}

func TestCancelledVoiceLearnDoesNotSave(t *testing.T) {
	t.Setenv("XDG_DATA_HOME", t.TempDir())
	samples, notes := "The draft is ready, let me know what needs work.", "Keep these saved notes."
	before, err := VoiceSet(protocol.WritingVoiceSetParams{Samples: &samples, Notes: &notes})
	if err != nil {
		t.Fatal(err)
	}
	stream := &testStream{block: true, closed: make(chan struct{})}
	client := &testClient{stream: stream, started: make(chan struct{})}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() {
		<-client.started
		cancel()
	}()
	if _, err := VoiceLearn(ctx, client, protocol.WritingVoiceLearnParams{Request: "voice-cancelled"}); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancelled learning error=%v", err)
	}
	after, err := VoiceGet()
	if err != nil || after != before {
		t.Fatalf("cancelled learn changed voice: before=%#v, after=%#v, error=%v", before, after, err)
	}
	select {
	case <-stream.closed:
	default:
		t.Fatal("cancelled learn left its model stream open")
	}
}

func TestCancelledVoiceStoreDoesNotCommit(t *testing.T) {
	path := filepath.Join(t.TempDir(), "voice.json")
	samples, notes := "Existing sample.", "Existing notes."
	before, err := setVoice(context.Background(), path, protocol.WritingVoiceSetParams{Samples: &samples, Notes: &notes})
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	replacement := "Should not be saved."
	if _, err := setVoice(ctx, path, protocol.WritingVoiceSetParams{Notes: &replacement}); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancelled store error=%v", err)
	}
	after, err := readVoice(path)
	if err != nil || after != before {
		t.Fatalf("cancelled store changed voice: before=%#v, after=%#v, error=%v", before, after, err)
	}
}
