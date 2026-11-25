package daemon

import (
	"context"
	"strings"
	"testing"
	"time"

	"ryoku-rashin/internal/hands/page"
	"ryoku-rashin/internal/oni/reflex"
	"ryoku-rashin/internal/oni/voice/route"
	"ryoku-rashin/internal/protocol"
)

func TestRewriteVoiceRecognizesExplicitSelectionOnly(t *testing.T) {
	for _, text := range []string{"make this friendlier", "fix the grammar", "turn this into an email", "rewrite selected text"} {
		if _, ok := rewriteVoiceStyle(text); !ok {
			t.Errorf("explicit request missed: %q", text)
		}
	}
	for _, text := range []string{"shorter", "yes", "read my clipboard", "rewrite everything", "send this", "make a friendlier website"} {
		if _, ok := rewriteVoiceStyle(text); ok {
			t.Errorf("implicit selection read: %q", text)
		}
	}
}

func TestGuardVoiceCommandTable(t *testing.T) {
	for _, tc := range []struct {
		text, action, app string
		minutes           int
	}{
		{"keep me off reddit for 20 minutes", protocol.OniFocusArm, "", 20},
		{"block tiktok for 8 hours", protocol.OniFocusArm, "", 480},
		{"block tiktok for 9 hours", protocol.OniFocusArm, "", 481},
		{"block reddit until i close word", protocol.OniFocusArm, "word", 0},
		{"ok let me back", protocol.OniFocusBack, "", 0},
		{"stop guarding", protocol.OniFocusStop, "", 0},
	} {
		p, ok := guardVoiceCommand(tc.text)
		if !ok || p.Action != tc.action || p.Minutes != tc.minutes || p.UntilApp != tc.app {
			t.Errorf("%q: %+v %v", tc.text, p, ok)
		}
	}
	for _, text := range []string{"open reddit", "block reddit", "block reddit forever", "keep me off reddit for -2 minutes"} {
		if _, ok := guardVoiceCommand(text); ok {
			t.Errorf("unexpected guard command %q", text)
		}
	}
}

func TestGuardedOpenBoundaryIncludesRoutineActions(t *testing.T) {
	d := oniDaemon(t)
	d.oni.guard.armed, d.oni.guard.words = true, []string{"instagram"}
	for _, intent := range []reflex.Intent{reflex.OpenApp, reflex.OpenSite, reflex.WebSearch} {
		if line := d.guardActionRefusal(reflex.Action{Intent: intent, Arg: "https://instagram.com/feed"}); !strings.Contains(line, "instagram") {
			t.Errorf("%s passed: %q", intent, line)
		}
		if line := d.guardActionRefusal(reflex.Action{Intent: intent, Arg: "https://example.com"}); line != "" {
			t.Errorf("neutral blocked: %q", line)
		}
	}
	d.oni.guard.armed = false
	if line := d.guardOpenRefusal("open instagram"); line != "" {
		t.Fatal("disarmed guard refused an open")
	}
}

type guardRenamedCatalog struct{}

func (guardRenamedCatalog) Resolve(spoken string) (string, string, float64, bool) {
	if strings.EqualFold(strings.TrimSpace(spoken), "instagram") {
		return "Install app", "install-app", 0.99, true
	}
	return fakeCatalog{names: []string{"Firefox"}}.Resolve(spoken)
}

func TestGuardedOpenOriginalRequestSurvivesCatalogRename(t *testing.T) {
	for _, tc := range []struct {
		name, text string
		armed      bool
		wantArg    string
		blocked    bool
	}{
		{"guarded fuzzy rename", "open instagram", true, "", true},
		{"unrelated open", "open firefox", true, "Firefox", false},
		{"disarmed rename", "open instagram", false, "Install app", false},
		{"mixed unrelated note", "open firefox and remember that instagram is blocked", true, "Firefox", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := serveReflex(t, func(s *protocol.Settings) {
				s.Oni.Hidden = true
				s.Oni.ReflexSettings.Notes = true
			})
			r.d.deps.OniReflex.Catalog = guardRenamedCatalog{}
			r.d.oni.guard.armed, r.d.oni.guard.words = tc.armed, []string{"instagram"}
			r.utter(t, route.Chord, tc.text)
			r.settle(t)
			calls := r.exec.calls()
			if tc.blocked {
				if len(calls) != 0 || !strings.Contains(strings.Join(r.notices(), "\n"), "stay off instagram") {
					t.Fatalf("guarded original request executed: calls=%+v notices=%q", calls, r.notices())
				}
			} else if len(calls) != 1 || calls[0].Intent != reflex.OpenApp || calls[0].Arg != tc.wantArg {
				t.Fatalf("unblocked request changed: calls=%+v notices=%q", calls, r.notices())
			}
			if !tc.blocked && strings.Contains(strings.Join(r.notices(), "\n"), "stay off") {
				t.Fatalf("an unrelated mention was refused: %q", r.notices())
			}
			if tc.name == "mixed unrelated note" && len(r.memory.seen()) != 1 {
				t.Fatalf("the unrelated note was lost: %q", r.memory.seen())
			}
		})
	}
}

func TestGuardedOpenDoesNotClaimOrdinaryAsk(t *testing.T) {
	r := serveReflex(t, func(s *protocol.Settings) { s.Oni.Hidden = true })
	r.d.oni.guard.armed, r.d.oni.guard.words = true, []string{"instagram"}
	r.utter(t, route.Chord, "why do people use instagram")
	r.settle(t)
	if len(r.exec.calls()) != 0 || len(r.questions()) != 1 || strings.Contains(strings.Join(r.notices(), "\n"), "stay off") {
		t.Fatalf("ordinary Ask was changed: calls=%+v questions=%q notices=%q", r.exec.calls(), r.questions(), r.notices())
	}
}

func TestGuardedOpenDecidedRequestSurvivesCatalogRename(t *testing.T) {
	for _, tc := range []struct {
		name, text string
		blocked    bool
	}{
		{"guarded", "i need the instagram thing now", true},
		{"unrelated", "i need the firefox thing now", false},
		{"mixed neutral target", "i need the firefox thing now with a note about instagram", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := serveReflex(t, func(s *protocol.Settings) {
				s.Oni.Hidden = true
				s.Oni.ReflexSettings.Decisions = true
				s.Providers.Decisions.Route = protocol.DecisionLocal
				s.Providers.Decisions.LocalModel = "fixture"
			})
			r.d.deps.OniReflex.Catalog = guardRenamedCatalog{}
			r.d.oni.guard.armed, r.d.oni.guard.words = true, []string{"instagram"}
			r.decider.answer = func(protocol.FastDecisionParams) protocol.FastDecisionResult {
				return decided(protocol.DecisionLocal, reflex.OpenApp, 0.9)
			}
			r.utter(t, route.Chord, tc.text)
			r.settle(t)
			calls := r.exec.calls()
			if len(r.decider.calls()) != 1 {
				t.Fatal("the decided route was not exercised")
			}
			if tc.blocked {
				if len(calls) != 0 || !strings.Contains(strings.Join(r.notices(), "\n"), "stay off instagram") {
					t.Fatalf("decided guarded request executed: calls=%+v notices=%q", calls, r.notices())
				}
			} else if len(calls) != 1 || calls[0].Intent != reflex.OpenApp || calls[0].Arg != "Firefox" {
				t.Fatalf("unrelated decided request changed: %+v", calls)
			}
			if !tc.blocked && strings.Contains(strings.Join(r.notices(), "\n"), "stay off") {
				t.Fatalf("a non-target mention was refused: %q", r.notices())
			}
		})
	}
}

type guardTestTab struct {
	page.Driver
	closes, opens int
}

func (p *guardTestTab) CloseGuardTab(context.Context, page.Scope, string) error {
	p.closes++
	return nil
}
func (p *guardTestTab) ReopenGuardTab(context.Context, string) error { p.opens++; return nil }

func TestBrowserGuardRejectsExpiredRearmedAndReplacedBrowser(t *testing.T) {
	for _, state := range []string{"disarmed", "rearmed", "replaced", "expired", "live"} {
		t.Run(state, func(t *testing.T) {
			g := serveGuard(t, guardNiri, nil)
			driver := &guardTestTab{}
			hb := &handsBrowser{drv: driver, scope: &page.Scope{TabID: "owned", Origin: "https://feed.test"}}
			handsWait(g.d, func() bool { g.d.oni.hands.browser = hb; return true })
			defer handsWait(g.d, func() bool { g.d.oni.hands.browser = nil; return true })
			s := &g.d.oni.guard
			s.mu.Lock()
			s.armed, s.gen, s.words, s.until = true, 1, []string{"reddit"}, g.clock.Now().Add(time.Minute)
			s.mu.Unlock()
			card := &guardCard{browser: hb, tab: "owned", origin: "https://feed.test", title: "reddit", feed: "reddit"}
			switch state {
			case "disarmed":
				s.armed = false
			case "rearmed":
				s.gen = 2
			case "replaced":
				handsWait(g.d, func() bool { g.d.oni.hands.browser = &handsBrowser{}; return true })
			}
			g.d.guardCloseBrowser(context.Background(), 1, card)
			want := 0
			if state == "live" || state == "expired" {
				want = 1
			}
			if driver.closes != want {
				t.Fatalf("%s closed %d", state, driver.closes)
			}
			if state == "live" || state == "expired" {
				cli := dialSocket(t, protocol.ClientCLI)
				if res := guardFocus(t, cli, protocol.OniFocusParams{Action: protocol.OniFocusStatus}); res.Reopen != 1 {
					t.Fatalf("closed tab is not offered for return: %+v", res)
				}
				if state == "expired" {
					g.clock.Advance(10*time.Minute + time.Nanosecond)
					if res := guardFocus(t, cli, protocol.OniFocusParams{Action: protocol.OniFocusStatus}); res.Reopen != 0 {
						t.Fatalf("expired tab is still offered for return: %+v", res)
					}
				}
				s.mu.Lock()
				s.armed = false
				s.mu.Unlock()
				g.d.guardReopen()
				wantOpens := 1
				if state == "expired" {
					wantOpens = 0
				}
				if driver.opens != wantOpens || hb.scope != nil {
					t.Fatal("reopen count or cleared authorization is wrong")
				}
				if res := guardFocus(t, cli, protocol.OniFocusParams{Action: protocol.OniFocusStatus}); res.Reopen != 0 {
					t.Fatalf("consumed tab is still offered for return: %+v", res)
				}
				g.d.guardReopen()
				if driver.opens != wantOpens {
					t.Fatal("reopened twice")
				}
			}
		})
	}
}
