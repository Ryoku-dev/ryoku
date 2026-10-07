package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

// askserve.go runs quick asks INSIDE the daemon (POST /api/ask): fast lane
// first, session lane as escalation, both recorded in the hub transcript so
// the conversation is already in the Super+S chat when the user continues there.
// The response streams the same marker lines the ask CLI used to emit:
// @working / @perm / @answer / @error.
type askSink struct {
	w       http.ResponseWriter
	f       http.Flusher
	answer  strings.Builder
	started bool
	lastMk  string
	// permJSON: the terminal lane's consumer answers permissions itself, so
	// its @perm payload carries {id, title, options} instead of a bare title.
	permJSON bool
}

func (s *askSink) marker(kind, detail string) {
	line := "@" + kind + " " + strings.ReplaceAll(detail, "\n", " ")
	if line == s.lastMk {
		return // a stream of identical thinking markers is noise
	}
	s.lastMk = line
	fmt.Fprintln(s.w, line)
	s.f.Flush()
}

// handleAsk is the /api/ask HTTP handler on the running daemon. The turn runs
// on a background context, not the request's, so closing the launcher (to
// continue in the Super+S chat) never aborts the work; only an explicit
// /api/ask/cancel or the timeout stops it. The CLI is a viewer.
func (h *chatHub) handleAsk(w http.ResponseWriter, r *http.Request) {
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

	// Background context: survives the launcher closing. Cancelable by the
	// user (Escape -> /api/ask/cancel) and bounded by a timeout.
	ctx, cancel := context.WithTimeout(context.Background(), quickTimeout+2*time.Minute)
	defer cancel()
	gen := h.setAskCancel(cancel)
	defer h.clearAskCancel(gen)

	// The question enters the shared transcript immediately, so a chat
	// opened mid-ask already shows it.
	h.broadcast(wsOut{Type: "user_text", Text: q})
	h.broadcast(wsOut{Type: "state", State: "busy"})

	if target, err := resolveQuickTarget(LoadConfig()); err == nil {
		sink.marker("working", "thinking ("+target.Label+")")
		qctx, qcancel := context.WithTimeout(ctx, quickTimeout)
		answer, qerr := quickComplete(qctx, target, q, func(delta string) {
			if !sink.started {
				sink.started = true
				sink.marker("working", "writing")
			}
			h.broadcast(wsOut{Type: "agent_text", Text: delta})
		}, func(id, title, status string) {
			if status == "in_progress" {
				sink.marker("working", title)
			}
			h.broadcast(wsOut{Type: "tool", ID: id, Title: title, Kind: "quick", Status: status})
		})
		qcancel()
		switch {
		case qerr == nil:
			h.broadcast(wsOut{Type: "turn_end", StopReason: "end_turn"})
			h.broadcast(wsOut{Type: "state", State: "ready"})
			recordAsk(askRecord{At: nowRFC3339(), Question: q, Answer: answer,
				Images: extractImages(answer), Actions: extractActions(answer)})
			sink.marker("answer", mustAskJSON(answer))
			return
		case qerr == errNeedsTools:
			sink.marker("working", "needs tools, waking the full agent")
		case ctx.Err() != nil:
			h.broadcast(wsOut{Type: "state", State: "ready"})
			sink.marker("error", "cancelled")
			return
		default:
			sink.marker("working", "fast lane unavailable: "+qerr.Error()+"; waking the full agent")
		}
	} else {
		sink.marker("working", "fast lane unavailable: "+err.Error()+"; waking the full agent")
	}

	if ans, ok := h.sessionAsk(ctx, sink, quickPreamble, q); ok {
		recordAsk(askRecord{At: nowRFC3339(), Kind: "quick", Question: q, Answer: ans,
			Images: extractImages(ans), Actions: extractActions(ans)})
		sink.marker("answer", mustAskJSON(ans))
	}
}

// setAskCancel registers the current ask's cancel func, keyed by a generation
// so a finished ask never clears a newer one. One user, one launcher.
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

func (h *chatHub) handleAskCancel(w http.ResponseWriter, r *http.Request) {
	h.mu.Lock()
	c := h.askCancel
	h.askCancel = nil
	conn := h.conn
	h.mu.Unlock()
	if c != nil {
		c()
		if conn != nil {
			conn.Cancel() // interrupt a session-lane turn too
		}
	}
	w.WriteHeader(http.StatusNoContent)
}

// handleAskRecent serves the resume list as JSON.
func (h *chatHub) handleAskRecent(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, RecentAsks(12))
}

// sessionAsk runs the question through the real hermes session, translating
// hub frames into markers until the turn ends. It joins as an internal client
// so every chat surface sees the same stream. The answer comes back for the
// caller to record and emit; error paths emit @error here and report false.
func (h *chatHub) sessionAsk(ctx context.Context, sink *askSink, preamble, q string) (string, bool) {
	cl := &chatClient{out: make(chan wsOut, 256)}
	h.mu.Lock()
	h.clients[cl] = true
	h.ensureConnLocked()
	conn := h.conn
	h.mu.Unlock()
	defer func() {
		h.mu.Lock()
		if h.clients[cl] {
			delete(h.clients, cl)
			close(cl.out)
		}
		h.mu.Unlock()
	}()
	if conn == nil {
		h.broadcast(wsOut{Type: "state", State: "ready"})
		sink.marker("error", "hermes is unavailable; run setup from Ryoku Settings")
		return "", false
	}
	// The user_text is already in the transcript; hand hermes the terse-mode
	// prompt directly.
	conn.Prompt(preamble+q, nil)

	var answer strings.Builder
	for {
		select {
		case <-ctx.Done():
			sink.marker("error", "timed out waiting for the answer")
			return "", false
		case f, ok := <-cl.out:
			if !ok {
				sink.marker("error", "daemon shutting down")
				return "", false
			}
			switch f.Type {
			case "state":
				if f.State == "dead" {
					msg := "hermes died mid-answer"
					if f.Error != "" {
						msg += ": " + f.Error
					}
					sink.marker("error", msg)
					return "", false
				}
			case "agent_thought":
				sink.marker("working", "thinking")
			case "tool":
				if f.Status == "pending" || f.Status == "in_progress" {
					label := f.Title
					if label == "" {
						label = "running a tool"
					}
					sink.marker("working", label)
				}
			case "agent_text":
				answer.WriteString(f.Text)
				if !sink.started {
					sink.started = true
					sink.marker("working", "writing")
				}
			case "permission":
				if sink.permJSON {
					b, err := json.Marshal(map[string]any{
						"id": f.RequestID, "title": f.Title, "options": f.Options,
					})
					if err == nil {
						sink.marker("perm", string(b))
						break
					}
				}
				sink.marker("perm", f.Title)
			case "turn_end":
				return strings.TrimSpace(answer.String()), true
			}
		}
	}
}

func mustAskJSON(text string) string {
	if text == "" {
		text = "(no answer)"
	}
	b, err := json.Marshal(askAnswer{Text: text, Images: extractImages(text), Actions: extractActions(text)})
	if err != nil {
		return `{"text":"(unreadable answer)"}`
	}
	return string(b)
}
