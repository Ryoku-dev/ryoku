package daemon

import (
	"context"
	"math"
	"testing"
	"time"

	"ryoku-rashin/internal/oni/feature"
	"ryoku-rashin/internal/oni/screenask"
	"ryoku-rashin/internal/oni/window"
	"ryoku-rashin/internal/protocol"
)

// pickWait sends oni.pick wait on p's connection and hands back what it
// came to once it does.
func pickWait(p *socketPeer) <-chan pickOutcome {
	out := make(chan pickOutcome, 1)
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		var res protocol.OniAskResult
		err := p.conn.Call(ctx, protocol.MOniPick, protocol.OniPickParams{Action: protocol.OniPickWait}, &res)
		out <- pickOutcome{res: res, err: err}
	}()
	return out
}

func waited(t *testing.T, ch <-chan pickOutcome) pickOutcome {
	t.Helper()
	select {
	case o := <-ch:
		return o
	case <-time.After(10 * time.Second):
		t.Fatal("the wait never ended")
		return pickOutcome{}
	}
}

// The connection that armed a pick waits on it and answers its picture's
// consent itself, as oni ask's caller does: it gets the same question, with
// the preview, whether the click comes before or after its wait; a cancel
// or the lapse ends the wait with nothing sent; no one else waits on it.
func TestOniPickArmerWaitsForTheClickAndAnswersItsConsent(t *testing.T) {
	r, surface, _, cli := guideRig(t, false)
	r.tess.set(screenask.Word{Text: "Export", Box: feature.Box{X: 0.2, Y: 0.22, W: 0.1, H: 0.05}})
	other := dialSocket(t, protocol.ClientCLI)
	arm := func() {
		t.Helper()
		r.clock.Advance(2 * time.Second) // past the repeat guard
		var res protocol.OniAskResult
		if err := cli.call(protocol.MOniPick, protocol.OniPickParams{Action: protocol.OniPickArm, Text: "What does this do?"}, &res); err != nil || res.Status != protocol.OniAskArmed {
			t.Fatalf("arm = %+v, %v", res, err)
		}
		surface.await("the armed pick", func(p feature.Payload) bool { f, ok := p.(*feature.PickFrame); return ok && f.Armed })
	}
	point := func() protocol.OniAskResult {
		t.Helper()
		var res protocol.OniAskResult
		if err := surface.call(protocol.MOniPick, protocol.OniPickParams{Action: protocol.OniPickPoint, Output: "DP-1", X: 300, Y: 230}, &res); err != nil || res.Status != protocol.OniAskConsent || res.Consent == nil {
			t.Fatalf("the click = %+v, %v", res, err)
		}
		return res
	}

	// waiting first: the click's question reaches the waiter, which answers
	arm()
	if code, _ := reasonOf(other.call(protocol.MOniPick, protocol.OniPickParams{Action: protocol.OniPickWait}, nil)); code != protocol.CodeConflict {
		t.Fatalf("a connection that armed nothing waited: %s", code)
	}
	wait := pickWait(cli)
	time.Sleep(50 * time.Millisecond)
	shown := point()
	got := waited(t, wait)
	if got.err != nil || got.res.Status != protocol.OniAskConsent || got.res.Consent == nil || got.res.Consent.ID != shown.Consent.ID || len(got.res.Consent.Preview) == 0 || got.res.Consent.Question == "" {
		t.Fatalf("the wait = %+v, %v", got.res, got.err)
	}
	if len(r.model.sent()) != 0 {
		t.Fatal("a picture went before its yes")
	}
	r.model.reply(`{"answer":"It saves a copy as a PDF.","walk":false}`)
	var res protocol.OniAskResult
	if err := cli.call(protocol.MOniConsentAnswer, protocol.OniConsentAnswerParams{ID: got.res.Consent.ID, Answer: "yes"}, &res); err != nil || res.Status != protocol.OniAskAnswered || res.Answer == nil || res.Answer.Text != "It saves a copy as a PDF." {
		t.Fatalf("the waiter's yes = %+v, %v", res, err)
	}
	if code, _ := reasonOf(cli.call(protocol.MOniPick, protocol.OniPickParams{Action: protocol.OniPickWait}, nil)); code != protocol.CodeConflict {
		t.Fatalf("a second wait on a taken pick: %s", code)
	}

	// clicking first: the outcome waits for the waiter
	r.allowRoute()
	r.model.reply(`{"answer":"It prints the file.","walk":false}`)
	arm()
	var direct protocol.OniAskResult
	if err := surface.call(protocol.MOniPick, protocol.OniPickParams{Action: protocol.OniPickPoint, Output: "DP-1", X: 300, Y: 230}, &direct); err != nil || direct.Status != protocol.OniAskAnswered {
		t.Fatalf("the click with the route allowed = %+v, %v", direct, err)
	}
	if got := waited(t, pickWait(cli)); got.err != nil || got.res.Status != protocol.OniAskAnswered || got.res.Answer == nil || got.res.Answer.Text != "It prints the file." {
		t.Fatalf("the late wait = %+v, %v", got.res, got.err)
	}

	// a cancel, from anyone, ends the wait with nothing sent
	arm()
	wait = pickWait(cli)
	time.Sleep(50 * time.Millisecond)
	if err := other.call(protocol.MOniPick, protocol.OniPickParams{Action: protocol.OniPickCancel}, nil); err != nil {
		t.Fatal(err)
	}
	if got := waited(t, wait); got.err != nil || got.res.Status != protocol.OniAskEnded || got.res.Answer != nil {
		t.Fatalf("the cancelled wait = %+v, %v", got.res, got.err)
	}

	// so does the lapse
	arm()
	wait = pickWait(cli)
	time.Sleep(50 * time.Millisecond)
	r.clock.Advance(feature.ConsentTTL)
	if got := waited(t, wait); got.err != nil || got.res.Status != protocol.OniAskEnded {
		t.Fatalf("the lapsed wait = %+v, %v", got.res, got.err)
	}
	surface.await("the lapsed pick's disarm", func(p feature.Payload) bool { f, ok := p.(*feature.PickFrame); return ok && !f.Armed })
}

// Arming again right after a click, while that click is still being read
// (the lab's repeat-guard run, or a user arming twice), replaces nothing the
// click owns: the first waiter still gets its click's question and answers
// it. The second click, inside the repeat guard, is a double click and ends
// only the second wait.
func TestOniPickClickInFlightKeepsItsWaiterAcrossANewArm(t *testing.T) {
	r, surface, _, cli := guideRig(t, false)
	r.tess.set(screenask.Word{Text: "Export", Box: feature.Box{X: 0.2, Y: 0.22, W: 0.1, H: 0.05}})
	second := dialSocket(t, protocol.ClientCLI)
	arm := func(p *socketPeer) {
		t.Helper()
		var res protocol.OniAskResult
		if err := p.call(protocol.MOniPick, protocol.OniPickParams{Action: protocol.OniPickArm, Text: "What does this do?"}, &res); err != nil || res.Status != protocol.OniAskArmed {
			t.Fatalf("arm = %+v, %v", res, err)
		}
	}
	click := protocol.OniPickParams{Action: protocol.OniPickPoint, Output: "DP-1", X: 300, Y: 230}

	arm(cli)
	first := pickWait(cli)
	time.Sleep(50 * time.Millisecond)
	// the presenter just moved, so the click's picture waits for it to hold
	// still: the second arm lands while the click is read
	if err := surface.call(protocol.MOniOverlay, protocol.OniOverlayParams{Output: "DP-1", Rects: []protocol.OniRect{{X: 600, Y: 40, W: 120, H: 80}}}, nil); err != nil {
		t.Fatal(err)
	}
	clicked := surface.async(protocol.MOniPick, click)
	time.Sleep(60 * time.Millisecond)
	arm(second)
	again := pickWait(second)

	got := waited(t, first)
	if got.err != nil || got.res.Status != protocol.OniAskConsent || got.res.Consent == nil {
		t.Fatalf("a new arm ended the wait of the click being read: %+v, %v", got.res, got.err)
	}
	if c := <-clicked; c.err != nil || c.res.Status != protocol.OniAskConsent {
		t.Fatalf("the click = %+v, %v", c.res, c.err)
	}
	var res protocol.OniAskResult
	if err := surface.call(protocol.MOniPick, click, &res); err != nil || res.Status != protocol.OniAskEnded {
		t.Fatalf("the double click = %+v, %v", res, err)
	}
	if got := waited(t, again); got.err != nil || got.res.Status != protocol.OniAskEnded {
		t.Fatalf("the double click's wait = %+v, %v", got.res, got.err)
	}
	r.model.reply(`{"answer":"It saves a copy as a PDF.","walk":false}`)
	if err := cli.call(protocol.MOniConsentAnswer, protocol.OniConsentAnswerParams{ID: got.res.Consent.ID, Answer: "yes"}, &res); err != nil || res.Status != protocol.OniAskAnswered || res.Answer == nil || res.Answer.Text != "It saves a copy as a PDF." {
		t.Fatalf("the first waiter's yes = %+v, %v", res, err)
	}
	if sent := r.model.sent(); len(sent) != 1 {
		t.Fatalf("%d select requests, want the first click's one", len(sent))
	}
}

// A click on a second output lands on the spot it was made on: the click is
// local to its output and the window's box to the window's output, so a
// window at the same place on either output gives the same spot.
func TestOniPickSpotOnASecondOutputIsTheClickedSpot(t *testing.T) {
	snap := window.Snapshot{Outputs: []window.Output{
		{Name: "A", X: 0, Y: 0, W: 1920, H: 1080, Scale: 1},
		{Name: "B", X: 1920, Y: 0, W: 1920, H: 1080, Scale: 1},
	}}
	local := &window.Box{X: 1120, Y: 440, W: 720, H: 480}
	sw, sh := oniPickSpotW/local.W, oniPickSpotH/local.H
	want := feature.Box{X: 146/local.W - sw/2, Y: 260/local.H - sh/2, W: sw, H: sh}
	for _, out := range []string{"A", "B"} {
		w := window.Window{ID: "w", Output: out, Visible: true, Floating: true, Box: local}
		snap.Windows = []window.Window{w}
		p := protocol.OniPickParams{Action: protocol.OniPickPoint, Output: out, X: 1266, Y: 700}
		if found, ok := oniFind(snap, protocol.OniPerchParams{Output: p.Output, X: p.X, Y: p.Y}); !ok || found.ID != w.ID {
			t.Fatalf("on %s the click finds %+v, %v", out, found, ok)
		}
		got := pickSpot(snap, w, p)
		if math.Abs(got.X-want.X) > 1e-9 || math.Abs(got.Y-want.Y) > 1e-9 || got.W != want.W || got.H != want.H {
			t.Fatalf("on %s the spot is %+v, want %+v", out, got, want)
		}
	}
}
