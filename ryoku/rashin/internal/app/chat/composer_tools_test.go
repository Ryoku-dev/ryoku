package chat

import (
	"context"
	"testing"

	"ryoku-rashin/internal/protocol"
)

type composerWritingCall struct {
	method string
	params any
	result any
	done   func(error)
}

type composerWritingFixture struct {
	composer Composer
	features map[string]bool
	calls    []composerWritingCall
	notices  []string
}

func newComposerWritingFixture() *composerWritingFixture {
	f := &composerWritingFixture{features: map[string]bool{"enhance": true, "askwriting": true}}
	state := f.composer.ensureState()
	state.ask = &askComposerState{voiceRead: true, hooks: AskToolsOptions{
		Feature: func(id string) bool { return f.features[id] },
		Call: func(method string, params, result any, done func(error)) {
			f.calls = append(f.calls, composerWritingCall{method: method, params: params, result: result, done: done})
		},
		Notify: func(text string) { f.notices = append(f.notices, text) },
	}}
	state.setText("Original draft")
	return f
}

func (f *composerWritingFixture) start() string {
	state := f.composer.ensureState()
	state.transformDraft("enhance", nil, "Improved")
	return state.ask.request
}

func (f *composerWritingFixture) assertCanceled(t *testing.T, request string) {
	t.Helper()
	state := f.composer.ensureState()
	if state.ask.request != "" || state.ask.busy {
		t.Fatalf("request still active: %q, busy %v", state.ask.request, state.ask.busy)
	}
	if len(f.calls) != 2 || f.calls[1].method != protocol.MWritingCancel {
		t.Fatalf("calls = %#v", f.calls)
	}
	if got := f.calls[1].params.(protocol.WritingCancelParams).Request; got != request || got == "" {
		t.Fatalf("cancel request = %q, want %q", got, request)
	}
}

func TestComposerWritingRequestsAreUniqueAndKeepUndo(t *testing.T) {
	f := newComposerWritingFixture()
	state := f.composer.ensureState()
	first := f.start()
	if first == "" || f.calls[0].params.(protocol.WritingTransformParams).Request != first {
		t.Fatal("transform has no request identity")
	}
	f.calls[0].result.(*protocol.WritingTransformResult).Text = "Improved draft"
	f.calls[0].done(nil)
	if state.editor.Text() != "Improved draft" || !state.ask.undoReady || state.ask.undoBefore != "Original draft" {
		t.Fatal("the transformed draft lost its undo source")
	}
	if state.ask.request != "" || state.ask.busy || len(f.calls) != 1 {
		t.Fatal("successful transform did not finish cleanly")
	}
	state.setText("Hi 😀 teh")
	state.checkDraft()
	check := f.calls[1]
	second := check.params.(protocol.WritingCheckParams).Request
	if check.method != protocol.MWritingCheck || second == "" || second == first {
		t.Fatal("check did not receive a fresh request identity")
	}
	check.result.(*protocol.WritingCheckResult).Issues = []protocol.WritingIssue{{Start: 6, End: 9, Replacements: []string{"the"}}}
	check.done(nil)
	if state.editor.Text() != "Hi 😀 teh" || state.ask.writingResult == nil {
		t.Fatal("checking changed the draft before the fixes were reviewed")
	}
	state.replaceDraft(applyDraftFixes(state.ask.writingDraft, state.ask.writingResult.Issues), "Fixed")
	if state.editor.Text() != "Hi 😀 the" || state.ask.undoBefore != "Hi 😀 teh" {
		t.Fatal("the UTF-16 correction did not keep the original draft")
	}
}

func TestComposerWritingCancelsOnDraftAndSurfaceAbandonment(t *testing.T) {
	cases := map[string]func(*composerWritingFixture){
		"replacement": func(f *composerWritingFixture) { f.composer.SetText("Another draft") },
		"typing": func(f *composerWritingFixture) {
			state := f.composer.ensureState()
			state.editor.SetText("Typed words")
			state.cancelChangedWriting()
		},
		"switching": func(f *composerWritingFixture) { f.composer.SetDraft(Draft{Text: "Original draft"}) },
		"hiding":    func(f *composerWritingFixture) { f.composer.CancelAskTools() },
	}
	for name, abandon := range cases {
		t.Run(name, func(t *testing.T) {
			f := newComposerWritingFixture()
			request := f.start()
			epoch := f.composer.ensureState().ask.epoch
			abandon(f)
			f.assertCanceled(t, request)
			if f.composer.ensureState().ask.epoch <= epoch {
				t.Fatal("abandonment kept the old epoch")
			}
			before := f.composer.Draft().Text
			f.calls[0].result.(*protocol.WritingTransformResult).Text = "Late replacement"
			f.calls[0].done(nil)
			if f.composer.Draft().Text != before {
				t.Fatal("a canceled request replaced the current draft")
			}
		})
	}
}

func TestComposerWritingCancelsWhenFeatureTurnsOff(t *testing.T) {
	f := newComposerWritingFixture()
	request := f.start()
	f.features["enhance"] = false
	state := f.composer.ensureState()
	f.composer.AskTools(new(ComposerOptions), state.ask.hooks)
	f.assertCanceled(t, request)
}

func TestComposerWritingTimeoutCancelsDaemonRequest(t *testing.T) {
	f := newComposerWritingFixture()
	request := f.start()
	f.calls[0].done(context.DeadlineExceeded)
	f.assertCanceled(t, request)
	if len(f.notices) != 1 || f.notices[0] != context.DeadlineExceeded.Error() {
		t.Fatalf("timeout notice = %#v", f.notices)
	}
}

func TestComposerStaleWritingCallbackKeepsNewRequest(t *testing.T) {
	f := newComposerWritingFixture()
	first := f.start()
	f.composer.CancelAskTools()
	f.composer.SetText("New draft")
	second := f.start()
	if second == "" || second == first {
		t.Fatal("the second request reused an identity")
	}
	f.calls[0].result.(*protocol.WritingTransformResult).Text = "Old answer"
	f.calls[0].done(nil)
	state := f.composer.ensureState()
	if state.ask.request != second || !state.ask.busy || state.editor.Text() != "New draft" {
		t.Fatal("the stale callback changed the new request")
	}
	f.calls[2].result.(*protocol.WritingTransformResult).Text = "New answer"
	f.calls[2].done(nil)
	if state.ask.busy || state.ask.request != "" || state.editor.Text() != "New answer" {
		t.Fatal("the current callback was not applied")
	}
}

func TestComposerToolsOffDoNoWork(t *testing.T) {
	var composer Composer
	calls := 0
	composer.AskTools(new(ComposerOptions), AskToolsOptions{
		Feature: func(string) bool { return false },
		Call:    func(string, any, any, func(error)) { calls++ },
	})
	if composer.state.ask != nil || calls != 0 {
		t.Fatal("disabled tools allocated feature state or started a call")
	}
}

func TestComposerDictionaryAddsExactUTF16WordAndRechecks(t *testing.T) {
	f := newComposerWritingFixture()
	state := f.composer.ensureState()
	state.setText("😀 中文 Ryoku")
	state.ask.writingDraft = state.editor.Text()
	state.addDraftWord(protocol.WritingIssue{Start: 6, End: 11, Category: "spelling"})
	if len(f.calls) != 1 || f.calls[0].method != protocol.MWritingWordsAdd || f.calls[0].params.(protocol.WritingWordParams).Word != "Ryoku" {
		t.Fatalf("dictionary call=%#v", f.calls)
	}
	f.calls[0].done(nil)
	if len(f.calls) != 2 || f.calls[1].method != protocol.MWritingCheck {
		t.Fatalf("dictionary did not recheck: %#v", f.calls)
	}
}
