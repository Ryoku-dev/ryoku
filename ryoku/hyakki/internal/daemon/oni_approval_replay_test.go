package daemon

import (
	"context"
	"testing"
	"time"

	"ryoku-rashin/internal/oni/feature"
	"ryoku-rashin/internal/oni/presence"
	"ryoku-rashin/internal/protocol"
)

func replayApprovalIDs(t *testing.T, result protocol.OniOpenResult) map[string]feature.ConsentFrame {
	t.Helper()
	cards := make(map[string]feature.ConsentFrame)
	for _, raw := range result.Frames {
		payload, err := feature.DecodeFrame(raw)
		if err != nil {
			t.Fatal(err)
		}
		if card, ok := payload.(*feature.ConsentFrame); ok {
			cards[card.ID] = *card
		}
	}
	return cards
}

func TestOniAppObserverRecoversPendingWritingApprovals(t *testing.T) {
	d := oniDaemon(t)
	initial := gateConn(t, d, protocol.ClientApp, true, "")
	params := mustMarshal(t, protocol.OniOpenParams{Renderer: protocol.OniRendererObserver})
	if _, err := d.oniOpen(initial, params); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { d.oni.mu.Lock(); defer d.oni.mu.Unlock(); d.oniStop() })
	service := d.oni.service.Load()
	cards := []*feature.ConsentFrame{
		feature.GuardCard("guard-pending", "Close the feed you chose?"),
		feature.ForgetCard("forget-pending", "Forget the fact you selected?"),
		feature.RecallCard("recall-pending", "openai/gpt-4.1", "Send the selected recall passages?"),
	}
	for _, card := range cards {
		if err := service.Consent(*card); err != nil {
			t.Fatal(err)
		}
	}
	// No Writing observer existed when the cards were emitted. Both the
	// first visit and a later visit must recover the still-pending cards.
	for range 2 {
		app := gateConn(t, d, protocol.ClientApp, true, "")
		opened, err := d.oniOpen(app, params)
		if err != nil {
			t.Fatal(err)
		}
		got := replayApprovalIDs(t, opened)
		for _, want := range cards {
			card, ok := got[want.ID]
			if !ok || card.Question != want.Question || card.Feature != want.Feature || card.ExpiresMs <= 0 || card.ExpiresMs > want.ExpiresMs {
				t.Errorf("late Writing observer did not recover pending %s: %+v", want.Feature, card)
			}
		}
	}
}

func TestOniPendingWritingReplayDoesNotGrantOtherCallersAuthority(t *testing.T) {
	g := serveGuard(t, guardSeam, nil)
	surface, _ := g.presenting(t)
	log := &frameLog{p: surface}
	guardArm20(t, g.dialApp(t))
	g.desk.await(t, 1, "")
	g.desk.set(guardFeeds())
	card := log.until("the live close card", consentFor(feature.FeedGuard)).(*feature.ConsentFrame)
	params := mustMarshal(t, protocol.OniOpenParams{Renderer: protocol.OniRendererObserver})
	for _, who := range []struct {
		name, kind, session string
		verified, observe   bool
	}{
		{"CLI observer", protocol.ClientCLI, "", false, true},
		{"CLI normal connection", protocol.ClientCLI, "", false, false},
		{"forged app", protocol.ClientApp, "", false, true},
		{"team", protocol.ClientCLI, "agent-1", false, true},
		{"app team", protocol.ClientApp, "agent-1", true, true},
	} {
		t.Run(who.name, func(t *testing.T) {
			c := gateConn(t, g.d, who.kind, who.verified, who.session)
			if who.observe {
				// oni.open has a dedicated socket reply path rather than a
				// generic dispatch branch: apply its real gate and handler.
				if err := g.d.mainGate(c, protocol.MOniOpen, params); err == nil {
					result, err := g.d.oniOpen(c, params)
					if err != nil {
						t.Fatal(err)
					}
					if got := replayApprovalIDs(t, result); got[card.ID].ID != "" {
						t.Fatalf("%s received an app-only pending card", who.name)
					}
				}
			}
			answer := mustMarshal(t, protocol.OniConsentAnswerParams{ID: card.ID, Answer: "yes"})
			if _, err := g.d.dispatch(context.Background(), c, protocol.MOniConsentAnswer, answer); !forbidden(err) {
				t.Fatalf("%s could answer a live app-only guard card: %v", who.name, err)
			}
			if g.d.guardCardWaiting() != card.ID {
				t.Fatal("an unauthorized request consumed the pending card")
			}
		})
	}
	appObserver := gateConn(t, g.d, protocol.ClientApp, true, "")
	if err := g.d.mainGate(appObserver, protocol.MOniOpen, params); err != nil {
		t.Fatal(err)
	}
	opened, err := g.d.oniOpen(appObserver, params)
	if err != nil {
		t.Fatal(err)
	}
	if replayApprovalIDs(t, opened)[card.ID].ID == "" {
		t.Fatal("the verified app observer could not read the live pending card")
	}
	if _, err := g.d.dispatch(context.Background(), appObserver, protocol.MOniConsentAnswer, mustMarshal(t, protocol.OniConsentAnswerParams{ID: card.ID, Answer: "yes"})); !forbidden(err) {
		t.Fatalf("a read-only app observer could answer a live pending card: %v", err)
	}
	if g.d.guardCardWaiting() != card.ID {
		t.Fatal("the app observer consumed the pending card")
	}
	if _, closed := g.closer.seen(); len(closed) != 0 {
		t.Fatalf("unauthorized replay readers closed windows: %v", closed)
	}
}

func TestOniWritingReplayKeepsOriginalExpiryAndDropsAnsweredOrOffCards(t *testing.T) {
	d := oniDaemon(t)
	clock := &askClock{now: time.Unix(1_800_000_000, 0)}
	service := presence.New(clock, d.oniEmit)
	d.oni.service.Store(service)
	settings := protocol.EffectiveOni(d.Settings())
	service.Settings(settings)
	t.Cleanup(func() { d.oni.mu.Lock(); defer d.oni.mu.Unlock(); d.oniStop() })
	params := mustMarshal(t, protocol.OniOpenParams{Renderer: protocol.OniRendererObserver})
	open := func() map[string]feature.ConsentFrame {
		t.Helper()
		result, err := d.oniOpen(gateConn(t, d, protocol.ClientApp, true, ""), params)
		if err != nil {
			t.Fatal(err)
		}
		return replayApprovalIDs(t, result)
	}
	card := feature.GuardCard("guard-expiry", "Close the feed you chose?")
	timers := clock.Armed()
	if err := service.Consent(*card); err != nil {
		t.Fatal(err)
	}
	if clock.Armed() != timers {
		t.Fatal("retaining a card added an idle timer")
	}
	clock.Advance(40 * time.Second)
	if got := open()[card.ID]; got.ID == "" || got.ExpiresMs != (80*time.Second).Milliseconds() {
		t.Errorf("late replay reset or lost the original deadline: %+v", got)
	}
	d.guardCardClosed(card.ID, "no")
	if got := open()[card.ID]; got.ID != "" {
		t.Fatal("an answered card was replayed")
	}
	if err := service.Consent(*card); err != nil {
		t.Fatal(err)
	}
	clock.Advance(feature.ConsentTTL)
	if got := open()[card.ID]; got.ID != "" {
		t.Fatal("an expired card was replayed")
	}
	if err := service.Consent(*card); err != nil {
		t.Fatal(err)
	}
	settings.Master = false
	service.Settings(settings)
	settings.Master = true
	service.Settings(settings)
	if got := open()[card.ID]; got.ID != "" {
		t.Fatal("turning Oni back on restored an old permission card")
	}
}
