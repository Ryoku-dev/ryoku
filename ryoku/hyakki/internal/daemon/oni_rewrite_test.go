package daemon

import (
	"context"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"ryoku-rashin/internal/dictate/insert"
	"ryoku-rashin/internal/hands/input"
	"ryoku-rashin/internal/oni/feature"
	"ryoku-rashin/internal/oni/screenask"
	"ryoku-rashin/internal/oni/window"
	"ryoku-rashin/internal/protocol"
	"ryoku-rashin/internal/rewrite/sel"
)

// These tests drive a rewrite in place over the daemon's own socket. The
// focused window is a fake editor: its selection reader, its typing, its
// undo key and the text reader that reads it back are fakes that record
// every argv, so nothing reaches the real desktop, its selection or its
// clipboard.

// rewriteEditor is the text field in the fake desktop's focused window
// ("0x1"): text with a selection, and an undo history that groups all the
// typing with the deletion of the selection, as GTK does, unless perChunk.
type rewriteEditor struct {
	mu                    sync.Mutex
	before, middle, after string
	selected              bool
	perChunk              bool
	deaf                  bool
	history               []string
	argv                  [][]string
	reads                 int
}

func (e *rewriteEditor) load(before, selection, after string) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.before, e.middle, e.after, e.selected, e.history = before, selection, after, true, nil
}

func (e *rewriteEditor) text() string {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.before + e.middle + e.after
}

func (e *rewriteEditor) record(name string, args []string) {
	e.argv = append(e.argv, append([]string{name}, args...))
}

func (e *rewriteEditor) selectRun(_ context.Context, limit int, name string, args ...string) ([]byte, error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.record(name, args)
	if !sel.Allowed(name, args...) {
		return nil, sel.ErrNotAllowed
	}
	if !e.selected {
		return nil, sel.ErrEmpty
	}
	out := []byte(e.middle)
	if len(out) > limit {
		out = out[:limit]
	}
	return out, nil
}

func (e *rewriteEditor) typeRun(_ context.Context, name string, args ...string) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.record(name, args)
	if !insert.Allowed(name, args...) {
		return insert.ErrNotAllowed
	}
	if e.selected {
		e.history = append(e.history, e.middle)
		e.middle, e.selected = "", false
	}
	if e.perChunk {
		e.history = append(e.history, e.middle)
	}
	e.middle += args[len(args)-1]
	return nil
}

func (e *rewriteEditor) keyRun(_ context.Context, name string, args ...string) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.record(name, args)
	if !input.Allowed(name, args...) {
		return input.ErrNotAllowed
	}
	if e.deaf || len(e.history) == 0 {
		return nil
	}
	e.middle, e.history = e.history[len(e.history)-1], e.history[:len(e.history)-1]
	return nil
}

// Words is the text reader: the words the field shows now.
func (e *rewriteEditor) Words(context.Context, []byte) ([]screenask.Word, error) {
	e.mu.Lock()
	e.reads++
	e.mu.Unlock()
	var out []screenask.Word
	for i, w := range strings.Fields(e.text()) {
		out = append(out, screenask.Word{Text: w, Box: feature.Box{X: float64(i%10) / 10, Y: 0.1, W: 0.05, H: 0.05}})
	}
	return out, nil
}

func (e *rewriteEditor) calls() [][]string {
	e.mu.Lock()
	defer e.mu.Unlock()
	return slices.Clone(e.argv)
}

// noClipboard fails if any runner saw a clipboard tool or a paste key, or
// anything but the one selection read, wtype chunks and the undo chord.
func (e *rewriteEditor) noClipboard(t *testing.T) {
	t.Helper()
	for _, c := range e.calls() {
		for _, a := range c {
			low := strings.ToLower(a)
			if strings.Contains(low, "wl-copy") || strings.Contains(low, "clip") || strings.Contains(low, "xsel") {
				t.Fatalf("a clipboard argv ran: %q", c)
			}
		}
		switch {
		case sel.Allowed(c[0], c[1:]...), insert.Allowed(c[0], c[1:]...):
		case slices.Equal(c, []string{"wtype", "-s", input.Settle, "-M", "ctrl", "-k", "z", "-m", "ctrl"}):
		default:
			t.Fatalf("an unexpected argv ran: %q", c)
		}
	}
}

func (e *rewriteEditor) typed() int {
	n := 0
	for _, c := range e.calls() {
		if insert.Allowed(c[0], c[1:]...) {
			n++
		}
	}
	return n
}

type rewriteRig struct {
	*voiceRig
	ed *rewriteEditor
}

func serveRewrite(t *testing.T, edit func(*protocol.Settings)) *rewriteRig {
	t.Helper()
	ed := &rewriteEditor{}
	ed.load("Hi Sam, ", "can u send me the report by tomorow", " Thanks.")
	r := &rewriteRig{ed: ed}
	r.voiceRig = serveVoice(t, func(s *protocol.Settings) {
		s.Oni.Rewrite.Enabled = true
		if edit != nil {
			edit(s)
		}
	}, nil, func(d *Deps) {
		d.OniRewrite = OniRewriteDeps{Select: ed.selectRun, Keys: ed.keyRun, Wait: -1}
		d.OniVoice.Type = ed.typeRun
		d.OniGuide.OCR = ed
	})
	return r
}

func rewriteCall(t *testing.T, p *socketPeer, params protocol.OniRewriteParams) protocol.OniRewriteResult {
	t.Helper()
	var res protocol.OniRewriteResult
	if err := p.call(protocol.MOniRewrite, params, &res); err != nil {
		t.Fatalf("oni.rewrite %s: %v", params.Action, err)
	}
	return res
}

const rewriteSelected = "can u send me the report by tomorow"

func TestOniRewriteShowsTypesBackAndUndoes(t *testing.T) {
	r := serveRewrite(t, nil)
	surface, _ := r.presenting(t)
	cli := dialSocket(t, protocol.ClientCLI)

	card := rewriteCall(t, cli, protocol.OniRewriteParams{Action: protocol.OniRewriteStart, Style: protocol.OniRewritePolite})
	if card.Status != protocol.OniRewriteConsent || card.Text != rewriteSelected || card.Chars != len(rewriteSelected) ||
		card.Destination != "local" || card.Route == "" || !strings.HasPrefix(card.ID, feature.RewriteCardPrefix) || card.ExpiresMs <= 0 {
		t.Fatalf("card: %+v", card)
	}
	frame := surface.await("the rewrite card", consentFor(feature.Rewrite)).(*feature.ConsentFrame)
	if frame.ID != card.ID || strings.Contains(frame.Question, "report") || frame.Class != feature.ConsentDisclosure {
		t.Fatalf("frame: %+v", frame)
	}
	if len(r.model.sent()) != 0 || r.ed.typed() != 0 {
		t.Fatal("something was sent or typed before the yes")
	}

	r.model.reply("Could you please send me the report by tomorrow?")
	done := rewriteCall(t, cli, protocol.OniRewriteParams{Action: protocol.OniRewriteApply, ID: card.ID})
	if done.Status != protocol.OniRewriteApplied || done.Landed != len("Could you please send me the report by tomorrow?") || done.Rest != "" || done.UndoVerified || done.Text != "" {
		t.Fatalf("applied: %+v", done)
	}
	if got := r.ed.text(); got != "Hi Sam, Could you please send me the report by tomorrow? Thanks." {
		t.Fatalf("field %q", got)
	}
	sent := r.model.sent()
	if len(sent) != 1 || !sent[0].Light || !sent[0].LocalOnly || !strings.Contains(sent[0].Messages[1].Content, "<<<TEXT\n"+rewriteSelected+"\nTEXT>>>") {
		t.Fatalf("request: %+v", sent)
	}

	undone := rewriteCall(t, cli, protocol.OniRewriteParams{Action: protocol.OniRewriteUndo, ID: card.ID})
	if undone.Status != protocol.OniRewriteUndone || !undone.UndoVerified {
		t.Fatalf("undo: %+v", undone)
	}
	if got := r.ed.text(); got != "Hi Sam, "+rewriteSelected+" Thanks." {
		t.Fatalf("field after undo %q", got)
	}
	// the record is gone: a second undo presses nothing
	if again := rewriteCall(t, cli, protocol.OniRewriteParams{Action: protocol.OniRewriteUndo, ID: card.ID}); again.Status != protocol.OniRewriteRefused {
		t.Fatalf("second undo: %+v", again)
	}

	// undo was seen to work in this kind of window, so now it says so
	r.ed.load("", rewriteSelected, "")
	card = rewriteCall(t, cli, protocol.OniRewriteParams{Action: protocol.OniRewriteStart})
	r.model.reply("Can you send me the report by tomorrow?")
	done = rewriteCall(t, cli, protocol.OniRewriteParams{Action: protocol.OniRewriteApply, ID: card.ID})
	if done.Status != protocol.OniRewriteApplied || !done.UndoVerified || done.Style != protocol.OniRewriteGrammar {
		t.Fatalf("second apply: %+v", done)
	}
	r.ed.noClipboard(t)
	if !r.shots.cleared() {
		t.Fatal("a read-back picture was not cleared")
	}
}

func TestOniRewriteManyChunksUndoOnlyAsFarAsTheOriginal(t *testing.T) {
	r := serveRewrite(t, nil)
	r.ed.perChunk = true
	cli := dialSocket(t, protocol.ClientCLI)
	long := "Could you please send me the quarterly report by tomorrow morning? I would really appreciate it, thanks so much."
	card := rewriteCall(t, cli, protocol.OniRewriteParams{Action: protocol.OniRewriteStart, Style: protocol.OniRewriteLonger})
	r.model.reply(long)
	done := rewriteCall(t, cli, protocol.OniRewriteParams{Action: protocol.OniRewriteApply, ID: card.ID})
	chunks := len(insert.Chunks(long, insert.MaxChunk))
	if done.Status != protocol.OniRewriteApplied || r.ed.typed() != chunks || chunks < 3 {
		t.Fatalf("applied %+v in %d chunks", done, r.ed.typed())
	}
	undone := rewriteCall(t, cli, protocol.OniRewriteParams{Action: protocol.OniRewriteUndo, ID: card.ID})
	presses := 0
	for _, c := range r.ed.calls() {
		if slices.Contains(c, "-k") {
			presses++
		}
	}
	// each press shows the typing coming undone, so undo goes on to the
	// original and stops there: one press per chunk and one for the
	// selection's deletion
	if undone.Status != protocol.OniRewriteUndone || presses != chunks+1 {
		t.Fatalf("undo: %+v after %d presses for %d chunks", undone, presses, chunks)
	}
	if got := r.ed.text(); got != "Hi Sam, "+rewriteSelected+" Thanks." {
		t.Fatalf("undone, but the field reads %q", got)
	}
	r.ed.noClipboard(t)
}

func TestOniRewriteUnverifiedUndoKeepsTheOriginalToCopy(t *testing.T) {
	r := serveRewrite(t, nil)
	cli := dialSocket(t, protocol.ClientCLI)
	card := rewriteCall(t, cli, protocol.OniRewriteParams{Action: protocol.OniRewriteStart})
	r.model.reply("Can you send me the report by tomorrow?")
	rewriteCall(t, cli, protocol.OniRewriteParams{Action: protocol.OniRewriteApply, ID: card.ID})
	r.ed.deaf = true
	kept := rewriteCall(t, cli, protocol.OniRewriteParams{Action: protocol.OniRewriteUndo, ID: card.ID})
	if kept.Status != protocol.OniRewriteKept || kept.Text != rewriteSelected || kept.UndoVerified {
		t.Fatalf("undo: %+v", kept)
	}
	presses := 0
	for _, c := range r.ed.calls() {
		if slices.Contains(c, "-k") {
			presses++
		}
	}
	if presses != 1 {
		t.Fatalf("%d presses into a window that showed no change", presses)
	}
	// the next rewrite does not claim Ctrl+Z works here
	r.ed.load("", rewriteSelected, "")
	r.ed.deaf = false
	card = rewriteCall(t, cli, protocol.OniRewriteParams{Action: protocol.OniRewriteStart})
	r.model.reply("Can you send me the report by tomorrow?")
	if done := rewriteCall(t, cli, protocol.OniRewriteParams{Action: protocol.OniRewriteApply, ID: card.ID}); done.UndoVerified {
		t.Fatalf("an unverified window claims undo: %+v", done)
	}
}

func TestOniRewriteRefusesBeforeReadingOrSending(t *testing.T) {
	for _, c := range []struct {
		name   string
		edit   func(*protocol.Settings)
		window func(*window.Window)
		sel    string
		want   string
		reads  bool
	}{
		{name: "off", edit: func(s *protocol.Settings) { s.Oni.Rewrite.Enabled = false }, want: protocol.OniRewriteRefusedOff},
		{name: "oni off", edit: func(s *protocol.Settings) { s.Oni.Master = false }, want: protocol.OniRewriteRefusedOff},
		{name: "terminal", window: func(w *window.Window) { w.App = "foot" }, want: protocol.OniRewriteRefusedTerminal},
		{name: "hyakki", window: func(w *window.Window) { w.App = "dev.ryoku.Hyakki" }, want: protocol.OniRewriteRefusedSelf},
		{name: "password manager", window: func(w *window.Window) { w.App = "org.keepassxc.KeePassXC" }, want: protocol.OniRewriteRefusedPrivate},
		{name: "password field", window: func(w *window.Window) { w.Title = "Change your password" }, want: protocol.OniRewriteRefusedPrivate},
		{name: "no focus", window: func(w *window.Window) { w.Focused = false }, want: protocol.OniRewriteRefusedWindow},
		{name: "nothing selected", sel: "\x00none", want: protocol.OniRewriteRefusedNoText, reads: true},
		{name: "white space", sel: "   ", want: protocol.OniRewriteRefusedNoText, reads: true},
		{name: "too long", sel: strings.Repeat("word ", 1700), want: protocol.OniRewriteRefusedTooLong, reads: true},
		{name: "a card number", sel: "my card is 4111 1111 1111 1111", want: protocol.OniRewriteRefusedSecret, reads: true},
		{name: "a token", sel: "key sk-abcdefghijklmnopqrstuvwxyz0123", want: protocol.OniRewriteRefusedSecret, reads: true},
	} {
		t.Run(c.name, func(t *testing.T) {
			r := serveRewrite(t, c.edit)
			if c.window != nil {
				snap := desktopWindows()
				c.window(&snap.Windows[0])
				r.desk.set(snap)
			}
			switch c.sel {
			case "":
			case "\x00none":
				r.ed.selected = false
			default:
				r.ed.load("", c.sel, "")
			}
			cli := dialSocket(t, protocol.ClientCLI)
			res := rewriteCall(t, cli, protocol.OniRewriteParams{Action: protocol.OniRewriteStart})
			if res.Status != protocol.OniRewriteRefused || res.Refused != c.want || res.Text != "" || res.ID != "" {
				t.Fatalf("got %+v, want %s", res, c.want)
			}
			if read := len(r.ed.calls()) > 0; read != c.reads {
				t.Fatalf("selection read %v, want %v: %q", read, c.reads, r.ed.calls())
			}
			if len(r.model.sent()) != 0 || r.ed.typed() != 0 || r.d.rewriteCardWaiting() != "" {
				t.Fatal("something was sent, typed or held")
			}
			r.ed.noClipboard(t)
		})
	}
}

func TestOniRewriteTypesNothingWhenItShouldNot(t *testing.T) {
	cases := []struct {
		name, style, reply string
		before             func(*rewriteRig)
		status, refused    string
		text               string
	}{
		{name: "same text", style: protocol.OniRewriteGrammar, reply: rewriteSelected, status: protocol.OniRewriteSame},
		{name: "facts added", style: protocol.OniRewriteFriendly, reply: "Could you send me the report by 5pm tomorrow?", status: protocol.OniRewriteRefused, refused: protocol.OniRewriteRefusedFacts},
		{name: "two paragraphs", style: protocol.OniRewriteBullets, reply: "- send me the report\n- by tomorrow", status: protocol.OniRewriteCopy, text: "- send me the report\n- by tomorrow"},
		{name: "model failed", style: protocol.OniRewriteGrammar, reply: "", status: protocol.OniRewriteRefused, refused: protocol.OniRewriteRefusedModel},
		{name: "selection changed", style: protocol.OniRewriteGrammar, reply: "Can you send me the report by tomorrow?",
			before: func(r *rewriteRig) { r.ed.load("", "something else entirely", "") }, status: protocol.OniRewriteRefused, refused: protocol.OniRewriteRefusedChanged},
		{name: "focus moved", style: protocol.OniRewriteGrammar, reply: "Can you send me the report by tomorrow?",
			before: func(r *rewriteRig) {
				snap := desktopWindows()
				snap.Windows[0].Focused = false
				snap.Windows[3].Focused = true
				r.desk.set(snap)
			}, status: protocol.OniRewriteRefused, refused: protocol.OniRewriteRefusedWindow},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			r := serveRewrite(t, nil)
			cli := dialSocket(t, protocol.ClientCLI)
			card := rewriteCall(t, cli, protocol.OniRewriteParams{Action: protocol.OniRewriteStart, Style: c.style})
			if c.before != nil {
				c.before(r)
			}
			field := r.ed.text()
			r.model.reply(c.reply)
			res := rewriteCall(t, cli, protocol.OniRewriteParams{Action: protocol.OniRewriteApply, ID: card.ID})
			if res.Status != c.status || res.Refused != c.refused || res.Text != c.text || res.Landed != 0 {
				t.Fatalf("got %+v", res)
			}
			if r.ed.typed() != 0 || r.ed.text() != field {
				t.Fatalf("typed %d chunks, field %q", r.ed.typed(), r.ed.text())
			}
			if c.status == protocol.OniRewriteCopy {
				// the result waits to be copied, by the caller only
				if again := rewriteCall(t, cli, protocol.OniRewriteParams{Action: protocol.OniRewriteApply, ID: card.ID}); again.Status != protocol.OniRewriteCopy || again.Text != c.text {
					t.Fatalf("copy again: %+v", again)
				}
				other := dialSocket(t, protocol.ClientCLI)
				if again := rewriteCall(t, other, protocol.OniRewriteParams{Action: protocol.OniRewriteApply, ID: card.ID}); again.Text != "" {
					t.Fatalf("another caller read the result: %+v", again)
				}
				r.clock.Advance(rewriteLimits().Undo())
				if again := rewriteCall(t, cli, protocol.OniRewriteParams{Action: protocol.OniRewriteApply, ID: card.ID}); again.Text != "" || again.Status != protocol.OniRewriteRefused {
					t.Fatalf("the result outlived its time: %+v", again)
				}
			}
			r.ed.noClipboard(t)
		})
	}
}

func TestOniRewriteCardAnswersAndLapses(t *testing.T) {
	r := serveRewrite(t, nil)
	surface, _ := r.presenting(t)
	cli := dialSocket(t, protocol.ClientCLI)
	app := r.dialApp(t)

	// no, through the consent answer, sends nothing
	card := rewriteCall(t, cli, protocol.OniRewriteParams{Action: protocol.OniRewriteStart})
	var ask protocol.OniAskResult
	if err := app.call(protocol.MOniConsentAnswer, protocol.OniConsentAnswerParams{ID: card.ID, Answer: "no"}, &ask); err != nil || ask.Status != protocol.OniAskDeclined {
		t.Fatalf("no: %+v %v", ask, err)
	}
	if len(r.model.sent()) != 0 || r.d.rewriteCardWaiting() != "" {
		t.Fatal("a declined card sent or stayed")
	}

	// the presenter cannot answer or start one
	card = rewriteCall(t, cli, protocol.OniRewriteParams{Action: protocol.OniRewriteStart})
	if err := surface.call(protocol.MOniConsentAnswer, protocol.OniConsentAnswerParams{ID: card.ID, Answer: "yes"}, nil); err == nil {
		t.Fatal("the presenter answered the rewrite card")
	}
	if err := surface.call(protocol.MOniRewrite, protocol.OniRewriteParams{Action: protocol.OniRewriteApply, ID: card.ID}, nil); err == nil {
		t.Fatal("the presenter applied a rewrite")
	}
	// another CLI connection cannot apply it
	other := dialSocket(t, protocol.ClientCLI)
	if res := rewriteCall(t, other, protocol.OniRewriteParams{Action: protocol.OniRewriteApply, ID: card.ID}); res.Status != protocol.OniRewriteRefused {
		t.Fatalf("another caller applied: %+v", res)
	}

	// yes, through the consent answer from the app, types it
	r.model.reply("Can you send me the report by tomorrow?")
	if err := app.call(protocol.MOniConsentAnswer, protocol.OniConsentAnswerParams{ID: card.ID, Answer: "yes"}, &ask); err != nil || ask.Status != protocol.OniAskAnswered {
		t.Fatalf("yes: %+v %v", ask, err)
	}
	if got := r.ed.text(); got != "Hi Sam, Can you send me the report by tomorrow? Thanks." {
		t.Fatalf("field %q", got)
	}
	// the undo time lapses and the original is dropped
	r.clock.Advance(rewriteLimits().Undo())
	if res := rewriteCall(t, cli, protocol.OniRewriteParams{Action: protocol.OniRewriteUndo, ID: card.ID}); res.Status != protocol.OniRewriteRefused {
		t.Fatalf("undo after its time: %+v", res)
	}

	// a card left alone lapses
	r.ed.load("", rewriteSelected, "")
	card = rewriteCall(t, cli, protocol.OniRewriteParams{Action: protocol.OniRewriteStart})
	r.clock.Advance(feature.ConsentTTL)
	if res := rewriteCall(t, cli, protocol.OniRewriteParams{Action: protocol.OniRewriteApply, ID: card.ID}); res.Status != protocol.OniRewriteRefused || res.Refused != protocol.OniRewriteRefusedUnknownID {
		t.Fatalf("a lapsed card applied: %+v", res)
	}
	// cancel takes a card down
	card = rewriteCall(t, cli, protocol.OniRewriteParams{Action: protocol.OniRewriteStart})
	if res := rewriteCall(t, cli, protocol.OniRewriteParams{Action: protocol.OniRewriteCancel, ID: card.ID}); res.Status != protocol.OniRewriteDeclined {
		t.Fatalf("cancel: %+v", res)
	}
	if len(r.model.sent()) != 1 {
		t.Fatalf("%d requests; only the yes sends", len(r.model.sent()))
	}
	r.ed.noClipboard(t)
}

// The record holds nothing once it ends: the texts are zeroed.
func TestOniRewriteZeroesTheOriginal(t *testing.T) {
	r := serveRewrite(t, nil)
	cli := dialSocket(t, protocol.ClientCLI)
	card := rewriteCall(t, cli, protocol.OniRewriteParams{Action: protocol.OniRewriteStart})
	r.model.reply("Can you send me the report by tomorrow?")
	rewriteCall(t, cli, protocol.OniRewriteParams{Action: protocol.OniRewriteApply, ID: card.ID})
	s := &r.d.oni.rewrite
	s.mu.Lock()
	rec := s.rec
	s.mu.Unlock()
	if rec == nil || string(rec.original) != rewriteSelected {
		t.Fatal("the original is not kept for undo")
	}
	original, result := rec.original, rec.result
	r.clock.Advance(rewriteLimits().Undo())
	deadline := time.Now().Add(2 * time.Second)
	for {
		s.mu.Lock()
		gone := s.rec == nil
		s.mu.Unlock()
		if gone {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("the record outlived its time")
		}
		time.Sleep(5 * time.Millisecond)
	}
	for _, b := range append(slices.Clone(original), result...) {
		if b != 0 {
			t.Fatal("a text was not zeroed")
		}
	}
}

// The command fixtures model a display-local selected field, not a Wayland
// server. They exercise the real restricted process constructors; Phase 16
// separately proves the same read, replacement and undo against GTK.
func TestOniRewriteToolConstructorsReadAndUndoOnNamedDisplay(t *testing.T) {
	bin, displayDir, daemonDir := t.TempDir(), t.TempDir(), t.TempDir()
	scripts := map[string]string{
		"wl-paste": `#!/bin/sh
set -eu
[ "$#" = 4 ] && [ "$1" = --primary ] && [ "$2" = --no-newline ] &&
    [ "$3" = --type ] && [ "$4" = 'text/plain;charset=utf-8' ]
value=
IFS= read -r value < "$XDG_RUNTIME_DIR/$WAYLAND_DISPLAY.selected" || [ -n "$value" ]
printf '%s' "$value"
`,
		"wtype": `#!/bin/sh
set -eu
[ "$1" = -s ]
shift 2
[ "$*" = '-M ctrl -k z -m ctrl' ]
value=
IFS= read -r value < "$XDG_RUNTIME_DIR/$WAYLAND_DISPLAY.original" || [ -n "$value" ]
printf '%s' "$value" > "$XDG_RUNTIME_DIR/$WAYLAND_DISPLAY.selected"
`,
	}
	for name, body := range scripts {
		if err := os.WriteFile(filepath.Join(bin, name), []byte(body), 0o700); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("PATH", bin)
	const selected, rewritten = "selected private words", "rewritten private words"
	field := filepath.Join(displayDir, "wayland-private.selected")
	if err := os.WriteFile(filepath.Join(displayDir, "wayland-private.original"), []byte(selected), 0o600); err != nil {
		t.Fatal(err)
	}
	for _, absolute := range []bool{false, true} {
		t.Run(map[bool]string{false: "bare", true: "absolute"}[absolute], func(t *testing.T) {
			d := oniDaemon(t)
			d.deps.OniRewrite.Select, d.deps.OniRewrite.Keys = nil, nil
			runtime, display := displayDir, "wayland-private"
			if absolute {
				runtime, display = daemonDir, filepath.Join(displayDir, display)
			}
			t.Setenv("XDG_RUNTIME_DIR", runtime)
			t.Setenv("WAYLAND_DISPLAY", display)
			if err := os.WriteFile(field, []byte(selected), 0o600); err != nil {
				t.Fatal(err)
			}
			got, err := sel.Read(context.Background(), d.rewriteSelector(), 100)
			if err != nil || got != selected {
				t.Fatalf("selection on named display: %q, %v", got, err)
			}
			if err := os.WriteFile(field, []byte(rewritten), 0o600); err != nil {
				t.Fatal(err)
			}
			args, err := input.KeyArgs("ctrl+z")
			if err != nil {
				t.Fatal(err)
			}
			args = append([]string{"-s", input.Settle}, args...)
			if err := d.rewriteKeys()(context.Background(), input.Tool, args...); err != nil {
				t.Fatalf("undo on named display: %v", err)
			}
			if got, err := sel.Read(context.Background(), d.rewriteSelector(), 100); err != nil || got != selected {
				t.Fatalf("undo did not restore the same display's field: %q, %v", got, err)
			}
			if os.Getenv("XDG_RUNTIME_DIR") != runtime || os.Getenv("WAYLAND_DISPLAY") != display {
				t.Fatal("per-tool construction changed the daemon's environment")
			}
		})
	}
}

func TestOniRewriteToolConstructorsRefuseMalformedDisplayBeforeStarting(t *testing.T) {
	bin, runtime := t.TempDir(), t.TempDir()
	started := filepath.Join(bin, "started")
	for _, tool := range []string{"wl-paste", "wtype"} {
		if err := os.WriteFile(filepath.Join(bin, tool), []byte("#!/bin/sh\nprintf started > '"+started+"'\n"), 0o700); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("PATH", bin)
	t.Setenv("XDG_RUNTIME_DIR", runtime)
	for _, display := range []string{"", "../wayland-1", "nested/wayland-1", "/tmp/../wayland-1", "/tmp/./wayland-1", "/tmp//wayland-1", "/tmp/wayland-1/", "/tmp/wayland-\n1"} {
		t.Run(strings.ReplaceAll(display, "/", "_"), func(t *testing.T) {
			t.Setenv("WAYLAND_DISPLAY", display)
			d := oniDaemon(t)
			d.deps.OniRewrite.Select, d.deps.OniRewrite.Keys = nil, nil
			if out, err := d.rewriteSelector()(context.Background(), 100, sel.Tool, sel.Args...); err == nil || len(out) != 0 {
				t.Fatalf("malformed display read a selection: %q, %v", out, err)
			}
			args, err := input.KeyArgs("ctrl+z")
			if err != nil {
				t.Fatal(err)
			}
			args = append([]string{"-s", input.Settle}, args...)
			if err := d.rewriteKeys()(context.Background(), input.Tool, args...); err == nil {
				t.Fatal("malformed display pressed an undo key")
			}
			if _, err := os.Stat(started); !os.IsNotExist(err) {
				t.Fatalf("a tool started for a malformed display: %v", err)
			}
		})
	}
}
