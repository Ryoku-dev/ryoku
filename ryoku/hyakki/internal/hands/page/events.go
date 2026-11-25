package page

import (
	"encoding/json"
	"strings"

	"ryoku-rashin/internal/hands/cdp"
)

// pump turns the browser's notifications for the authorized tab into
// Events, updating what ready checks first. It ends with Close or the
// connection.
func (s *Session) pump() {
	defer close(s.events)
	src := s.conn.Events()
	for {
		select {
		case <-s.done:
			return
		case ev, ok := <-src:
			if !ok {
				s.mu.Lock()
				had := s.session != "" && s.gone == ""
				s.gone = Closed
				s.mu.Unlock()
				if had {
					s.emit(Event{Kind: Closed, Away: true})
				}
				return
			}
			if out, ok := s.translate(ev); ok {
				s.emit(out)
			}
		}
	}
}

func (s *Session) emit(e Event) {
	select {
	case s.events <- e:
	case <-s.done:
	}
}

// translate updates the tab's state for ev and says what, if anything,
// the daemon hears. Notifications about other tabs are ignored, except a
// tab the authorized one opened.
func (s *Session) translate(ev cdp.Event) (Event, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.session == "" {
		return Event{}, false
	}
	mine := ev.Session == s.session
	var p struct {
		TargetID   string     `json:"targetId"`
		SessionID  string     `json:"sessionId"`
		TargetInfo targetInfo `json:"targetInfo"`
		Frame      struct {
			ID       string `json:"id"`
			ParentID string `json:"parentId"`
			URL      string `json:"url"`
		} `json:"frame"`
		Name    string `json:"name"`
		Payload string `json:"payload"`
	}
	_ = json.Unmarshal(ev.Params, &p)
	lose := func(k Kind) (Event, bool) {
		if s.gone != "" {
			return Event{}, false
		}
		s.gone, s.nodes, s.armed = k, nil, armed{}
		return Event{Kind: k, Away: true}, true
	}
	switch ev.Method {
	case "Target.targetDestroyed":
		if ev.Session == "" && p.TargetID == s.scope.TabID {
			return lose(Closed)
		}
	case "Target.detachedFromTarget":
		if ev.Session == "" && p.SessionID == s.session {
			return lose(Closed)
		}
	case "Inspector.detached":
		if mine {
			return lose(Closed)
		}
	case "Target.targetCrashed":
		if ev.Session == "" && p.TargetID == s.scope.TabID {
			return lose(Crashed)
		}
	case "Inspector.targetCrashed":
		if mine {
			return lose(Crashed)
		}
	case "Target.targetCreated":
		if ev.Session == "" && p.TargetInfo.OpenerID == s.scope.TabID && p.TargetInfo.OpenerID != "" {
			return Event{Kind: Popup, Away: true}, true
		}
	case "Page.windowOpen":
		if mine {
			return Event{Kind: Popup, Away: true}, true
		}
	case "Page.downloadWillBegin":
		if mine {
			return Event{Kind: Download, Away: true}, true
		}
	case "Page.javascriptDialogOpening":
		if mine {
			s.dialog = true
			return Event{Kind: Dialog, Away: true}, true
		}
	case "Page.javascriptDialogClosed":
		if mine {
			s.dialog = false
		}
	case "Page.frameNavigated":
		if mine && p.Frame.ParentID == "" {
			s.frame, s.origin, s.world, s.nodes, s.armed = p.Frame.ID, Origin(p.Frame.URL), 0, nil, armed{}
			s.gen++
			return Event{Kind: Navigated, Origin: s.origin, Away: s.origin != s.scope.Origin}, true
		}
	case "Runtime.bindingCalled":
		if !mine || p.Name != cdp.Binding {
			return Event{}, false
		}
		switch {
		case strings.HasPrefix(p.Payload, "pressed:"):
			token := strings.TrimPrefix(p.Payload, "pressed:")
			if s.armed.token == "" || token != s.armed.token {
				return Event{}, false
			}
			c := s.armed.control
			s.armed = armed{}
			return Event{Kind: Pressed, Control: c}, true
		case p.Payload == "hidden" && !s.hidden:
			s.hidden = true
			return Event{Kind: Hidden, Away: true}, true
		case p.Payload == "visible" && s.hidden:
			s.hidden = false
			return Event{Kind: Visible}, true
		}
	}
	return Event{}, false
}
