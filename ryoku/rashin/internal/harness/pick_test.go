package harness

import (
	"context"
	"encoding/json"
	"os"
	"strings"
	"testing"

	"ryoku-rashin/internal/core"
	"ryoku-rashin/internal/protocol"
)

// fakePickModels is the model list the scripted agent offers in pick mode.
func fakePickModels(current string) map[string]any {
	return map[string]any{
		"availableModels": []map[string]string{{"modelId": "big", "name": "Big"}, {"modelId": "small", "name": "Small"}},
		"currentModelId":  current,
	}
}

// fakeSetModel answers session/set_model: a model it offers is taken, any
// other is refused, as a harness refuses one it no longer has.
func fakeSetModel(write func(any), message rpcMessage, chosen map[string]string) {
	var params struct {
		SessionID string `json:"sessionId"`
		ModelID   string `json:"modelId"`
	}
	_ = json.Unmarshal(message.Params, &params)
	if params.SessionID != "fake-session" || (params.ModelID != "big" && params.ModelID != "small") {
		write(map[string]any{"jsonrpc": "2.0", "id": rawID(message.ID), "error": rpcError{Code: -32602, Message: "no such model"}})
		return
	}
	chosen["model"] = params.ModelID
	write(map[string]any{"jsonrpc": "2.0", "id": rawID(message.ID), "result": map[string]any{}})
}

func pickRunner(t *testing.T, model string) (*runner, *testSink, *protocol.HarnessCapabilities) {
	t.Helper()
	t.Setenv("RASHIN_ACP_TEST_MODE", "pick")
	cwd := t.TempDir()
	r := newRunner(core.HarnessOptions{
		Session:   protocol.SessionInfo{ID: "agent-pick", Kind: protocol.KindAgent, Model: model, Cwd: cwd},
		Cwd:       cwd,
		Approvals: func() string { return protocol.ApprovalsAsk },
	}, []string{os.Args[0], "-test.run=^TestACPHelperProcess$"})
	reported := &protocol.HarnessCapabilities{}
	r.reportCapabilities = func(capabilities protocol.HarnessCapabilities) { *reported = capabilities }
	sink := newTestSink()
	t.Cleanup(func() { _ = r.Stop() })
	return r, sink, reported
}

// A session opened with a model is put on it right after session/new, and the
// harness's own default is learned from before the pick.
func TestASessionOpenedWithAModelStartsOnIt(t *testing.T) {
	r, sink, reported := pickRunner(t, "small")
	if _, err := r.Prompt(context.Background(), core.PromptInput{Text: "which model"}, sink); err != nil {
		t.Fatal(err)
	}
	text := sink.waitKind(t, protocol.EvText, 1)[0].data.(protocol.TextData)
	if text.Delta != "model small" {
		t.Fatalf("the agent answered on %q", text.Delta)
	}
	if model := lastOf[protocol.ChoiceData](t, sink, protocol.EvModel); model.Current != "small" || len(model.Options) != 2 {
		t.Fatalf("the model reads %#v", model)
	}
	sink.mu.Lock()
	recorded := sink.info.Model
	sink.mu.Unlock()
	if recorded != "small" {
		t.Fatalf("the session records %q", recorded)
	}
	if reported.DefaultModel != "big" || len(reported.ModelList) != 2 {
		t.Fatalf("learned default %q from %#v", reported.DefaultModel, reported.ModelList)
	}
}

// A model the harness refuses leaves the session on its default with a
// warning; the session still starts.
func TestARefusedPickStartsOnTheDefault(t *testing.T) {
	r, sink, reported := pickRunner(t, "gone")
	if _, err := r.Prompt(context.Background(), core.PromptInput{Text: "which model"}, sink); err != nil {
		t.Fatal(err)
	}
	text := sink.waitKind(t, protocol.EvText, 1)[0].data.(protocol.TextData)
	if text.Delta != "model big" {
		t.Fatalf("the agent answered on %q", text.Delta)
	}
	notice := lastOf[protocol.Notice](t, sink, protocol.EvtNotice)
	if notice.Level != protocol.NoticeWarn || !strings.Contains(notice.Text, "gone") || notice.Session != "agent-pick" {
		t.Fatalf("the notice reads %#v", notice)
	}
	if reported.DefaultModel != "big" {
		t.Fatalf("learned default %q", reported.DefaultModel)
	}
}
