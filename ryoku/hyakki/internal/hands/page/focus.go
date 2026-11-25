package page

import (
	"context"
	"time"
)

// guardClosedTab never accepts an address supplied by a caller. It is the
// address re-read from this session's authorized target immediately before close.
type guardClosedTab struct {
	id, address string
	at          time.Time
}

// CloseGuardTab closes only this pipe session's currently authorized tab,
// after checking its target, origin and exact displayed title again. It retains
// one safe web address for a single, short-lived undo; file tabs are not closed.
func (s *Session) CloseGuardTab(ctx context.Context, scope Scope, title string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if scope.TabID == "" || s.scope != scope || s.gone != "" {
		return ErrNotAuthorized
	}
	var r struct {
		TargetInfos []targetInfo `json:"targetInfos"`
	}
	if err := s.conn.Call(ctx, "", "Target.getTargets", nil, &r); err != nil {
		return err
	}
	for _, t := range r.TargetInfos {
		if t.TargetID != scope.TabID {
			continue
		}
		if t.Type != "page" || Origin(t.URL) != scope.Origin || clean(t.Title, s.lim.MaxNameRunes) != title || !webAddress(t.URL) {
			return ErrStale
		}
		var closed struct {
			Success bool `json:"success"`
		}
		if err := s.conn.Call(ctx, "", "Target.closeTarget", map[string]any{"targetId": t.TargetID}, &closed); err != nil {
			return err
		}
		if !closed.Success {
			return ErrAway
		}
		s.closedTab = &guardClosedTab{id: t.TargetID, address: t.URL, at: time.Now()}
		s.scope, s.nodes, s.armed = Scope{}, nil, armed{}
		s.gone = Closed
		return nil
	}
	return ErrUnknownTab
}

func webAddress(raw string) bool {
	o := Origin(raw)
	return len(raw) <= 8192 && (len(o) > 7 && o[:7] == "http://" || len(o) > 8 && o[:8] == "https://")
}

// ReopenGuardTab opens only the last target actually closed above, once and
// within ten minutes. The new tab is deliberately NOT authorized.
func (s *Session) ReopenGuardTab(ctx context.Context, closedID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	c := s.closedTab
	if c == nil || c.id != closedID || time.Since(c.at) < 0 || time.Since(c.at) > 10*time.Minute {
		return ErrUnknownTab
	}
	if err := s.conn.Call(ctx, "", "Target.createTarget", map[string]any{"url": c.address}, nil); err != nil {
		return err
	}
	s.closedTab = nil
	return nil
}
