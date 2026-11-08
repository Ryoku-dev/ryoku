package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"
)

// askserve.go keeps one-shot answers outside the persistent Agent transcript.
// Prowl handles the fast path and an isolated read-only ACP lane handles
// escalations.
type askSink struct {
	w       http.ResponseWriter
	f       http.Flusher
	started bool
	lastMk  string
}

func (s *askSink) marker(kind, detail string) {
	line := "@" + kind + " " + strings.ReplaceAll(detail, "\n", " ")
	if line == s.lastMk {
		return
	}
	s.lastMk = line
	fmt.Fprintln(s.w, line)
	s.f.Flush()
}

type sessionAskResult struct {
	Text   string
	Via    string
	Denied string
}

func (h *chatHub) handleAsk(w http.ResponseWriter, r *http.Request) {
	if !allowLocalMutation(w, r, false) {
		return
	}
	q := strings.TrimSpace(r.URL.Query().Get("q"))
	if q == "" {
		http.Error(w, "missing q", http.StatusBadRequest)
		return
	}
	f, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "no streaming", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	sink := &askSink{w: w, f: f}
	id := newAskID('a')

	ctx, cancel := context.WithTimeout(context.Background(), quickTimeout+2*time.Minute)
	defer cancel()
	gen := h.setAskCancel(cancel)
	defer h.clearAskCancel(gen)

	cfg := LoadConfig()
	useGateway(5 * time.Second)
	target, targetErr := resolveQuickTarget(cfg)
	if targetErr == nil {
		sink.marker("working", "thinking ("+target.Label+")")
		qctx, qcancel := context.WithTimeout(ctx, quickTimeout)
		answer, model, qerr := quickComplete(qctx, target, q, func(string) {
			if !sink.started {
				sink.started = true
				sink.marker("working", "writing")
			}
		}, func(_ string, title, status string) {
			if status == "in_progress" {
				sink.marker("working", title)
			}
		})
		qcancel()
		switch {
		case qerr == nil:
			viaModel := strings.TrimSpace(model)
			if viaModel == "" {
				viaModel = target.Label
			}
			rec := makeAskRecord(id, "", q, answer, "Prowl · "+viaModel, "")
			recordAsk(rec)
			sink.marker("answer", mustAskJSON(rec))
			return
		case qerr == errNeedsTools:
			sink.marker("working", "needs the read-only agent")
		case ctx.Err() != nil:
			sink.marker("error", "cancelled")
			return
		default:
			sink.marker("working", "fast lane unavailable: "+qerr.Error()+"; waking the agent")
		}
	} else if !errors.Is(targetErr, errAgentRoute) {
		sink.marker("working", "fast lane unavailable: "+targetErr.Error()+"; waking the agent")
	}

	result, ok := h.sessionAsk(ctx, sink, quickPreamble, q)
	if !ok {
		return
	}
	rec := makeAskRecord(id, "", q, result.Text, result.Via, result.Denied)
	recordAsk(rec)
	sink.marker("answer", mustAskJSON(rec))
}

func (h *chatHub) setAskCancel(c context.CancelFunc) uint64 {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.askGen++
	h.askCancel = c
	h.askCancelGen = h.askGen
	return h.askGen
}

func (h *chatHub) clearAskCancel(gen uint64) {
	h.mu.Lock()
	if h.askCancelGen == gen {
		h.askCancel = nil
	}
	h.mu.Unlock()
}

func (h *chatHub) cancelAskTurn() {
	h.mu.Lock()
	cancel := h.askCancel
	h.askCancel = nil
	h.mu.Unlock()
	if cancel != nil {
		cancel()
	}
}

func (h *chatHub) handleAskCancel(w http.ResponseWriter, r *http.Request) {
	if !allowLocalMutation(w, r, false) {
		return
	}
	h.cancelAskTurn()
	w.WriteHeader(http.StatusNoContent)
}

func (h *chatHub) handleAskRecent(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, RecentAsks(12))
}

// sessionAsk serializes the internal lane because one ACP connection owns one
// current session. Every call opens a fresh session and consumes its events
// directly, so none can enter a UI hub transcript.
func (h *chatHub) sessionAsk(ctx context.Context, sink *askSink, preamble, q string) (sessionAskResult, bool) {
	conn, err := h.beginAskSession(ctx)
	if err != nil {
		if _, ok := resolveChatBackend(LoadConfig()); !ok {
			sink.marker("error", "Ask needs an agent harness with an ACP adapter or a connected Prowl provider.")
		} else {
			sink.marker("error", "agent unavailable: "+err.Error())
		}
		return sessionAskResult{}, false
	}
	defer h.finishAskSession(conn)

	drainAskEvents(conn)
	conn.Prompt(h.lane.Intro()+preamble+q, nil)
	var answer strings.Builder
	via := modelEventVia(conn.currentModelsEvent(), quickAgentRoute(LoadConfig()).Sub)
	for {
		select {
		case <-ctx.Done():
			conn.Cancel()
			h.dropAskConn(conn)
			if errors.Is(ctx.Err(), context.Canceled) {
				sink.marker("error", "cancelled")
			} else {
				sink.marker("error", "timed out waiting for the answer")
			}
			return sessionAskResult{}, false
		case ev, open := <-conn.Events():
			if !open {
				h.dropAskConn(conn)
				sink.marker("error", "agent stopped before answering")
				return sessionAskResult{}, false
			}
			switch ev.Type {
			case "state":
				if ev.State == "dead" {
					h.dropAskConn(conn)
					msg := "agent stopped before answering"
					if ev.Err != "" {
						msg += ": " + ev.Err
					}
					sink.marker("error", msg)
					return sessionAskResult{}, false
				}
			case "models":
				via = modelEventVia(ev, via)
			case "agent_thought":
				sink.marker("working", "thinking")
			case "tool":
				if ev.Tool.Status == "pending" || ev.Tool.Status == "in_progress" {
					label := ev.Tool.Title
					if label == "" {
						label = "using a read-only tool"
					}
					sink.marker("working", label)
				}
			case "agent_text":
				answer.WriteString(ev.Text)
				if !sink.started {
					sink.started = true
					sink.marker("working", "writing")
				}
			case "turn_end":
				return sessionAskResult{
					Text: strings.TrimSpace(answer.String()), Via: via,
					Denied: conn.takeAskDenied(),
				}, true
			}
		}
	}
}

const askIdleTimeout = 10 * time.Minute

var startAskACP = startACP

func (h *chatHub) beginAskSession(ctx context.Context) (*acpConn, error) {
	h.askSerial.Lock()
	if err := ctx.Err(); err != nil {
		h.askSerial.Unlock()
		return nil, err
	}
	h.mu.Lock()
	h.askRunning = true
	if h.askIdle != nil {
		h.askIdle.Stop()
		h.askIdle = nil
	}
	conn := h.conn
	var stale *acpConn
	if conn != nil && (conn.stale() || conn.isClosed()) {
		stale = conn
		h.conn = nil
		conn = nil
	}
	h.mu.Unlock()
	if stale != nil {
		stale.Close()
	}
	if conn == nil {
		if err := prepareLane(h.lane); err != nil {
			h.abortAskSession()
			return nil, err
		}
		var err error
		conn, err = startAskACP(h.lane.Cwd)
		if err != nil {
			h.abortAskSession()
			return nil, err
		}
		conn.mu.Lock()
		conn.askOnly = h.lane.ReadOnly
		conn.mu.Unlock()
		h.mu.Lock()
		h.conn = conn
		h.mu.Unlock()
		if err := conn.Initialize(h.lane.Cwd); err != nil {
			h.dropAskConn(conn)
			h.abortAskSession()
			return nil, err
		}
		return conn, nil
	}
	conn.resetAskDenied()
	if err := conn.NewSession(); err != nil {
		h.dropAskConn(conn)
		h.abortAskSession()
		return nil, err
	}
	return conn, nil
}

func (h *chatHub) abortAskSession() {
	h.mu.Lock()
	h.askRunning = false
	h.mu.Unlock()
	h.askSerial.Unlock()
}

func (h *chatHub) finishAskSession(conn *acpConn) {
	h.mu.Lock()
	h.askRunning = false
	closeNow := false
	keepWarm := false
	if h.conn == conn {
		if h.askReset {
			h.askReset = false
			h.conn = nil
			closeNow = true
		} else {
			keepWarm = true
			h.askIdle = time.AfterFunc(askIdleTimeout, func() {
				h.mu.Lock()
				if h.askRunning || h.conn != conn {
					h.mu.Unlock()
					return
				}
				h.conn = nil
				h.askIdle = nil
				h.mu.Unlock()
				conn.Close()
			})
		}
	}
	h.mu.Unlock()
	if closeNow {
		conn.Close()
	} else if keepWarm {
		drainAskEvents(conn)
	}
	h.askSerial.Unlock()
}

func (h *chatHub) dropAskConn(conn *acpConn) {
	h.mu.Lock()
	if h.conn == conn {
		h.conn = nil
		h.askReset = false
	}
	h.mu.Unlock()
	conn.Close()
}

func drainAskEvents(conn *acpConn) {
	for {
		select {
		case _, open := <-conn.Events():
			if !open {
				return
			}
		default:
			return
		}
	}
}

func modelEventVia(ev AcpEvent, fallback string) string {
	agent := strings.TrimSpace(ev.AgentName)
	model := strings.TrimSpace(ev.CurrentModel)
	for _, offered := range ev.Models {
		if offered.ID == model && strings.TrimSpace(offered.Name) != "" {
			model = strings.TrimSpace(offered.Name)
			break
		}
	}
	if agent == "" || model == "" {
		return fallback
	}
	return agent + " · " + model
}

func makeAskRecord(id, kind, question, text, via, denied string) askRecord {
	text = strings.TrimSpace(text)
	reason := oneLine(strings.TrimSpace(denied), 500)
	needsAgent := reason != ""
	if strings.HasPrefix(text, "NEEDS_AGENT:") {
		reason = oneLine(strings.TrimSpace(strings.TrimPrefix(text, "NEEDS_AGENT:")), 500)
		text = reason
		needsAgent = true
	}
	if text == "" {
		if reason != "" {
			text = reason
		} else {
			text = "(no answer)"
		}
	}
	return askRecord{
		ID: id, At: nowRFC3339(), Kind: kind, Question: question, Answer: text,
		Images: extractImages(text), Actions: extractActions(text), Via: via,
		NeedsAgent: needsAgent, NeedsAgentReason: reason,
	}
}

func mustAskJSON(rec askRecord) string {
	images := rec.Images
	if images == nil {
		images = []string{}
	}
	actions := rec.Actions
	if actions == nil {
		actions = []askAction{}
	}
	b, err := json.Marshal(askAnswer{
		ID: rec.ID, Text: rec.Answer, Images: images, Actions: actions,
		Via: rec.Via, NeedsAgent: rec.NeedsAgent, NeedsAgentReason: rec.NeedsAgentReason,
	})
	if err != nil {
		return `{"id":"` + rec.ID + `","text":"(unreadable answer)"}`
	}
	return string(b)
}
