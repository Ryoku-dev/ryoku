package page

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"ryoku-rashin/internal/hands/cdp"
)

type focusConn struct {
	*fake
	closed, opened int
}

func (f *focusConn) Call(ctx context.Context, session, method string, params, out any) error {
	if method != "Target.closeTarget" && method != "Target.createTarget" {
		return f.fake.Call(ctx, session, method, params, out)
	}
	if !cdp.Allowed(method, params) {
		return cdp.ErrRefused
	}
	if session != "" {
		f.t.Fatal("tab management must use our browser pipe session")
	}
	if method == "Target.closeTarget" {
		f.closed++
		b := []byte(`{"success":true}`)
		return json.Unmarshal(b, out)
	}
	f.opened++
	return nil
}

func focusOpen(t *testing.T, f *focusConn) *Session {
	t.Helper()
	s, err := Open(context.Background(), f, lim)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = s.Close() })
	return s
}

func TestFocusGuardClosesOnlyAuthorizedUnchangedTab(t *testing.T) {
	for _, change := range []string{"none", "title", "origin", "target", "unauthorized", "file"} {
		t.Run(change, func(t *testing.T) {
			f := &focusConn{fake: newFake(t)}
			s := focusOpen(t, f)
			scope, err := s.Authorize(context.Background(), "T1")
			if err != nil {
				t.Fatal(err)
			}
			title := clean(f.targets[0].Title, s.lim.MaxNameRunes)
			switch change {
			case "title":
				f.targets[0].Title = "Neutral"
			case "origin":
				f.targets[0].URL = "https://changed.test/feed"
			case "target":
				scope.TabID = "T2"
			case "unauthorized":
				s.scope = Scope{}
			case "file":
				scope.Origin = "file://"
				s.scope = scope
				f.targets[0].URL = "file:///tmp/feed.html"
			}
			err = s.CloseGuardTab(context.Background(), scope, title)
			if change != "none" {
				if err == nil || f.closed != 0 {
					t.Fatalf("changed target closed: %v %d", err, f.closed)
				}
				if err := s.ReopenGuardTab(context.Background(), "T1"); !errors.Is(err, ErrUnknownTab) {
					t.Fatalf("unclosed target reopened: %v", err)
				}
				return
			}
			if err != nil || f.closed != 1 {
				t.Fatalf("close: %v %d", err, f.closed)
			}
			if err := s.ReopenGuardTab(context.Background(), "T2"); !errors.Is(err, ErrUnknownTab) {
				t.Fatalf("arbitrary target reopened: %v", err)
			}
			if err := s.ReopenGuardTab(context.Background(), "T1"); err != nil || f.opened != 1 {
				t.Fatalf("reopen: %v %d", err, f.opened)
			}
			if s.scope.TabID != "" {
				t.Fatal("reopened target was silently authorized")
			}
			if err := s.ReopenGuardTab(context.Background(), "T1"); !errors.Is(err, ErrUnknownTab) {
				t.Fatal("reopen replayed")
			}
		})
	}
}

func TestFocusGuardReopenExpires(t *testing.T) {
	f := &focusConn{fake: newFake(t)}
	s := focusOpen(t, f)
	s.closedTab = &guardClosedTab{id: "T1", address: "https://feed.test/x", at: time.Now().Add(-11 * time.Minute)}
	if err := s.ReopenGuardTab(context.Background(), "T1"); !errors.Is(err, ErrUnknownTab) || f.opened != 0 {
		t.Fatalf("expired reopen: %v", err)
	}
}

func TestFocusGuardCDPShapesStayNarrow(t *testing.T) {
	for _, c := range []struct {
		method string
		p      map[string]any
		want   bool
	}{
		{"Target.closeTarget", map[string]any{"targetId": "T1"}, true},
		{"Target.closeTarget", map[string]any{"targetId": "T1", "browserContextId": "foreign"}, false},
		{"Target.createTarget", map[string]any{"url": "https://feed.test/x"}, true},
		{"Target.createTarget", map[string]any{"url": "file:///tmp/x"}, false},
		{"Target.createTarget", map[string]any{"url": "javascript:alert(1)"}, false},
		{"Target.createTarget", map[string]any{"url": "https://u:p@feed.test"}, false},
		{"Target.createTarget", map[string]any{"url": "https://feed.test", "newWindow": true}, false},
	} {
		if got := cdp.Allowed(c.method, c.p); got != c.want {
			t.Errorf("%s %v: %v", c.method, c.p, got)
		}
	}
}
