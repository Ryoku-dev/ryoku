package daemon

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"ryoku-rashin/internal/config"
	"ryoku-rashin/internal/core"
	"ryoku-rashin/internal/protocol"
)

type comparisonAsker struct {
	gates     map[string]chan struct{}
	started   chan string
	cancelled chan string
}

func (a comparisonAsker) NewAsk(options core.AskOptions) core.Runner {
	model := options.Session.Model
	return &fakeRunner{prompt: func(ctx context.Context, in core.PromptInput, sink core.Sink) (core.TurnResult, error) {
		sink.Emit(protocol.EvText, protocol.TextData{Message: "answer", Delta: "answer from " + model})
		if gate := a.gates[model]; gate != nil {
			a.started <- model
			select {
			case <-gate:
			case <-ctx.Done():
				a.cancelled <- model
				return core.TurnResult{Stop: protocol.StopCancelled}, ctx.Err()
			}
		}
		return core.TurnResult{Stop: protocol.StopEnd}, nil
	}}
}

func comparisonDaemon(t *testing.T) (*Daemon, protocol.SessionInfo, comparisonAsker) {
	t.Helper()
	isolateDaemon(t)
	settings := config.Defaults()
	settings.Features = map[string]bool{"compare": true}
	settings.Ask.Model = protocol.RouteAuto
	asker := comparisonAsker{gates: map[string]chan struct{}{"first": make(chan struct{}), "second": make(chan struct{})}, started: make(chan string, 2), cancelled: make(chan string, 2)}
	d := New(Deps{Asker: asker})
	d.settings = settings
	manager := d.manager
	info, err := manager.create(protocol.SessionNewParams{Kind: protocol.KindAsk, Model: "original", Title: "One question", Memory: protocol.MemoryNone})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		manager.mu.Lock()
		ids := make([]string, 0, len(manager.items))
		for id := range manager.items {
			ids = append(ids, id)
		}
		manager.mu.Unlock()
		for _, id := range ids {
			_ = manager.delete(id)
		}
	})
	finishTurn(t, manager, info.ID, "What should I measure?")
	return d, info, asker
}

func comparisonLastEnd(t *testing.T, manager *sessionManager, id string) int64 {
	t.Helper()
	events, err := manager.Events(id)
	if err != nil {
		t.Fatal(err)
	}
	var seq int64
	for _, event := range events {
		if event.Kind == protocol.EvTurnEnd {
			seq = event.Seq
		}
	}
	return seq
}

func comparisonWait(t *testing.T, manager *sessionManager, id string) {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		session, err := manager.get(id)
		if err != nil {
			t.Fatal(err)
		}
		session.mu.Lock()
		busy := session.running || session.finishing || session.starting
		session.mu.Unlock()
		if !busy {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatal("comparison answer never finished")
}

func comparisonText(t *testing.T, manager *sessionManager, id string) string {
	t.Helper()
	collector := newCollector()
	replay, err := manager.attach(id, 0, collector)
	if err != nil {
		t.Fatal(err)
	}
	defer manager.detach(id, collector)
	var text strings.Builder
	for _, event := range replay.Events {
		if event.Kind == protocol.EvText {
			var delta protocol.TextData
			if err := json.Unmarshal(event.Data, &delta); err != nil {
				t.Fatal(err)
			}
			text.WriteString(delta.Delta)
		}
	}
	return text.String()
}

func TestComparisonLanesStreamIndependentlyAndKeepOneConversation(t *testing.T) {
	d, source, asker := comparisonDaemon(t)
	seq := comparisonLastEnd(t, d.manager, source.ID)
	result, err := d.compareStart(context.Background(), nil, protocol.CompareStartParams{ID: source.ID, Seq: seq, Models: []string{"first", "second"}})
	if err != nil {
		t.Fatal(err)
	}
	for range 2 {
		select {
		case <-asker.started:
		case <-time.After(3 * time.Second):
			t.Fatal("one comparison held up the other model")
		}
	}
	close(asker.gates["first"])
	comparisonWait(t, d.manager, result.Lanes[1].Session)
	if got := comparisonText(t, d.manager, result.Lanes[1].Session); got != "answer from first" {
		t.Fatalf("first lane mixed in another model's answer: %q", got)
	}
	other, err := d.manager.get(result.Lanes[2].Session)
	if err != nil {
		t.Fatal(err)
	}
	other.mu.Lock()
	stillAnswering := other.running
	other.mu.Unlock()
	if !stillAnswering {
		t.Fatal("finishing the first model also stopped the second")
	}
	if got := comparisonText(t, d.manager, source.ID); got != "answer from original" {
		t.Fatalf("comparing rewrote the source answer: %q", got)
	}
	kept, err := d.compareKeep(context.Background(), nil, protocol.CompareKeepParams{ID: result.ID, Session: result.Lanes[1].Session})
	if err != nil {
		t.Fatal(err)
	}
	if kept.Temporary || kept.Model != "first" || comparisonText(t, d.manager, kept.ID) != "answer from first" {
		t.Fatalf("kept conversation lost the selected answer: %+v", kept)
	}
	if _, exists := d.manager.Info(source.ID); exists {
		t.Fatal("keeping the answer left the replaced conversation in history")
	}
	for _, lane := range result.Lanes {
		if _, exists := d.manager.Info(lane.Session); exists {
			t.Fatalf("keeping one answer left temporary lane %s", lane.Session)
		}
	}
	if _, _, err := d.manager.prompt(context.Background(), protocol.PromptParams{ID: kept.ID, Text: "And next?"}, nil); err != nil {
		t.Fatalf("the kept lane could not continue: %v", err)
	}
}

func TestKeepingComparisonDoesNotDeleteLaterQuestions(t *testing.T) {
	d, source, asker := comparisonDaemon(t)
	seq := comparisonLastEnd(t, d.manager, source.ID)
	result, err := d.compareStart(context.Background(), nil, protocol.CompareStartParams{ID: source.ID, Seq: seq, Models: []string{"first"}})
	if err != nil {
		t.Fatal(err)
	}
	close(asker.gates["first"])
	comparisonWait(t, d.manager, result.Lanes[1].Session)
	finishTurn(t, d.manager, source.ID, "Do not lose this later question")
	if _, err := d.compareKeep(context.Background(), nil, protocol.CompareKeepParams{ID: result.ID, Session: result.Lanes[1].Session}); err != nil {
		t.Fatal(err)
	}
	if _, exists := d.manager.Info(source.ID); !exists {
		t.Fatal("keeping an earlier comparison deleted a newer question")
	}
}

func TestComparisonEndsWithItsSourceOrFeature(t *testing.T) {
	for _, featureOff := range []bool{false, true} {
		name := "source deleted"
		if featureOff {
			name = "feature disabled"
		}
		t.Run(name, func(t *testing.T) {
			d, source, asker := comparisonDaemon(t)
			result, err := d.compareStart(context.Background(), nil, protocol.CompareStartParams{
				ID: source.ID, Seq: comparisonLastEnd(t, d.manager, source.ID), Models: []string{"first", "second"},
			})
			if err != nil {
				t.Fatal(err)
			}
			for range 2 {
				select {
				case <-asker.started:
				case <-time.After(3 * time.Second):
					t.Fatal("comparison worker did not start")
				}
			}
			if featureOff {
				if _, err := d.applySettings(context.Background(), json.RawMessage(`{"features":{"compare":false}}`)); err != nil {
					t.Fatal(err)
				}
			} else if err := d.manager.delete(source.ID); err != nil {
				t.Fatal(err)
			}
			cancelled := make(map[string]bool)
			for range 2 {
				select {
				case model := <-asker.cancelled:
					cancelled[model] = true
				case <-time.After(3 * time.Second):
					t.Fatal("ending the comparison left provider work running")
				}
			}
			if !cancelled["first"] || !cancelled["second"] {
				t.Fatalf("not every provider stopped: %v", cancelled)
			}
			for _, lane := range result.Lanes {
				if _, exists := d.manager.Info(lane.Session); exists {
					t.Fatalf("ending the comparison kept orphaned transcript %s", lane.Session)
				}
			}
			d.comparisons.mu.Lock()
			remaining := len(d.comparisons.groups)
			d.comparisons.mu.Unlock()
			if remaining != 0 {
				t.Fatal("ending the comparison kept its registry entry")
			}
			if featureOff {
				if comparisonText(t, d.manager, source.ID) != "answer from original" {
					t.Fatal("turning comparison off lost the source answer")
				}
			}
		})
	}
}
