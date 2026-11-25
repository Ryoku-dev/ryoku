package daemon

import (
	"os"
	"path/filepath"
	"testing"

	"ryoku-rashin/internal/oni/window"
	"ryoku-rashin/internal/protocol"
)

// These tests exercise the window-closing half, not a browser transport.
// Neither a familiar browser ID nor an entry declaring WebBrowser/http
// becomes a closable feed window merely because the person listed its name.
func TestOniGuardBrowserAppIDIsAnnounceOnly(t *testing.T) {
	for _, tc := range []struct{ name, app, feed, metadata string }{
		{"firefox", "firefox", "firefox", ""},
		{"reverse-domain Firefox", "org.mozilla.firefox", "firefox", ""},
		{"Chromium without catalog entry", "chromium", "chromium", ""},
		{"desktop browser category", "org.example.FeedBrowser", "feedbrowser", "Categories=Network;WebBrowser;\n"},
		{"desktop URL handler", "org.example.FeedBrowser", "feedbrowser", "MimeType=x-scheme-handler/https;\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			if tc.metadata != "" {
				guardBrowserEntry(t, dir, tc.metadata)
			}
			g := serveGuard(t, guardSeam, func(s *protocol.Settings) { s.Oni.Feeds.Words = []string{tc.feed, "tiktok"} })
			g.d.deps.OniHands.AppDirs = []string{dir}
			g.presenting(t)
			cli := dialSocket(t, protocol.ClientCLI)
			guardArm20(t, cli)
			g.desk.await(t, 1, "")
			g.desk.set(window.Snapshot{Outputs: []window.Output{dp1}, Windows: []window.Window{
				{ID: "21", App: guardFeedApp, Title: "For You", Output: "DP-1", Visible: true},
				{ID: "25", App: tc.app, Title: "unrelated personal tabs", Output: "DP-1", Visible: true, Focused: true},
			}})
			remindEventually(t, "both app identities processed", func() bool {
				s := &g.d.oni.guard
				s.mu.Lock()
				defer s.mu.Unlock()
				return len(s.seen) == 2
			})
			s := &g.d.oni.guard
			s.mu.Lock()
			cards, announced := len(s.cards), s.announced
			dedicated := cards == 1 && s.cards[0].app == guardFeedApp
			s.mu.Unlock()
			if !dedicated || announced != 1 {
				t.Fatalf("browser offered a full-window close: cards=%d announced=%d dedicated=%v", cards, announced, dedicated)
			}
			if _, closes := g.closer.seen(); len(closes) != 0 {
				t.Fatalf("closed without app approval: %v", closes)
			}
		})
	}
}

func guardBrowserEntry(t *testing.T, dir, metadata string) {
	t.Helper()
	text := "[Desktop Entry]\nType=Application\nName=Feed Browser\nExec=/usr/bin/true %u\n" + metadata
	if err := os.WriteFile(filepath.Join(dir, "org.example.FeedBrowser.desktop"), []byte(text), 0o600); err != nil {
		t.Fatal(err)
	}
}

// A queued card is not authority after desktop metadata newly identifies the
// exact same app ID as a browser. The close executor must classify again.
func TestOniGuardRechecksBrowserIdentityBeforeWindowClose(t *testing.T) {
	dir := t.TempDir()
	guardBrowserEntry(t, dir, "Categories=Network;\n")
	g := serveGuard(t, guardSeam, func(s *protocol.Settings) { s.Oni.Feeds.Words = []string{"feedbrowser"} })
	g.d.deps.OniHands.AppDirs = []string{dir}
	cli := dialSocket(t, protocol.ClientCLI)
	guardArm20(t, cli)
	g.desk.await(t, 1, "")
	w := window.Window{ID: "25", App: "org.example.FeedBrowser", Title: "personal tabs", Output: "DP-1", Visible: true, Focused: true}
	g.desk.set(window.Snapshot{Outputs: []window.Output{dp1}, Windows: []window.Window{w}})
	card, err := newGuardCard(w, "feedbrowser")
	if err != nil {
		t.Fatal(err)
	}
	guardBrowserEntry(t, dir, "Categories=Network;WebBrowser;\n")
	s := &g.d.oni.guard
	s.mu.Lock()
	gen, provider := s.gen, s.provider
	s.mu.Unlock()
	g.d.guardClose(gen, provider, card)
	if _, closes := g.closer.seen(); len(closes) != 0 {
		t.Fatalf("stale card closed a newly identified browser: %v", closes)
	}
}
