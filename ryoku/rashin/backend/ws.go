package main

import (
	"context"
	"encoding/json"
	"os"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"
)

// ws.go fans one shared hermes ACP session out to every dashboard chat
// socket, and pushes vitals samples. The session is lazy: hermes spawns on
// the first chat client, survives client disconnects, and is restarted on
// the next user message after a death.

type wsOut struct {
	Type       string        `json:"type"`
	State      string        `json:"state,omitempty"`
	Error      string        `json:"error,omitempty"`
	Text       string        `json:"text,omitempty"`
	ID         string        `json:"id,omitempty"`
	Title      string        `json:"title,omitempty"`
	Kind       string        `json:"kind,omitempty"`
	Status     string        `json:"status,omitempty"`
	Input      string        `json:"input,omitempty"`
	Output     string        `json:"output,omitempty"`
	Diffs      []ToolDiff    `json:"diffs,omitempty"`
	Auto       bool          `json:"auto,omitempty"`
	ToolID     string        `json:"toolId,omitempty"`
	RequestID  string        `json:"requestId,omitempty"`
	Options    []PermOption  `json:"options,omitempty"`
	Outcome    string        `json:"outcome,omitempty"`
	Mode       string        `json:"mode,omitempty"`
	StopReason string        `json:"stopReason,omitempty"`
	Models     []ModelInfo   `json:"models,omitempty"`
	Current    string        `json:"current,omitempty"`
	Agent      string        `json:"agent,omitempty"`
	Prowl      string        `json:"prowl,omitempty"`
	Reason     string        `json:"reason,omitempty"`
	Commands   []CommandInfo `json:"commands,omitempty"`
	SessionID  string        `json:"sessionId,omitempty"`
	Size       int           `json:"size,omitempty"`
	Used       int           `json:"used,omitempty"`
	Sessions   []SessionMeta `json:"sessions,omitempty"`
}

type wsIn struct {
	Type      string        `json:"type"`
	Text      string        `json:"text"`
	RequestID string        `json:"requestId"`
	OptionID  string        `json:"optionId"`
	Images    []PromptImage `json:"images"`
	ModelID   string        `json:"modelId"`
	SessionID string        `json:"sessionId"`
	Quick     bool          `json:"quick"`
	Mode      string        `json:"mode"`
}

type chatClient struct {
	ws  *websocket.Conn
	out chan wsOut
}

type chatHub struct {
	// lane says what this hub's conversation is for: its session cwd (the
	// agent's context), whether quick asks land here, and the first-turn
	// preamble. Hubs never share an agent process.
	lane     lane
	mu       sync.Mutex
	conn     *acpConn
	clients  map[*chatClient]bool
	last     wsOut // last state frame, replayed to joiners
	models   wsOut // last models frame, replayed to joiners
	commands wsOut // last commands frame, replayed to joiners
	// transcript is the current session's conversation, replayed to every
	// joiner so a chat begun anywhere (launcher ask, another tab) is already
	// on screen when the dashboard opens.
	transcript []wsOut
	// perms are the approvals still waiting on a person, in arrival order;
	// a joiner gets them after the replay so any surface can answer one.
	perms []wsOut
	// introduced guards the one-time Needle identity preamble: it rides the
	// first non-slash chat turn of a session and only hermes sees it.
	introduced bool
	// askCancel stops the in-flight quick ask (Escape in the launcher);
	// askCancelGen keys it so a finished ask never clears a newer one.
	askCancel    func()
	askGen       uint64
	askCancelGen uint64
	// termCancels keys in-flight terminal asks by request id, so concurrent
	// terminals cancel independently; termHist is the last terminal exchange,
	// the continuation context behind `rashin -c`.
	termCancels map[string]context.CancelFunc
	termHist    []chatMessage
}

// transcriptCap bounds the join replay; older frames just scroll away. Chunks
// and tool updates coalesce, so a frame is a message or a tool call.
const transcriptCap = 400

// needlePersona is who the assistant is on this machine, for every agent Rashin
// starts: the shared chat session and a Fix with AI harness in a terminal.
const needlePersona = "You are the Needle, the resident assistant on this Ryoku machine " +
	"(Arch Linux with the Ryoku desktop). If asked who you are, you are the Needle. Be direct and " +
	"technical; you know this machine through the vault, and you use your tools, skills, and the prowl " +
	"code index freely. A \"how do I\" question asks for guidance, not for you to change " +
	"anything: answer it, never run the change. When asked how to change the desktop, name " +
	"the GUI path first: the Ryoku Hub page (Super+comma, or `ryoku-shell hub open <section>`), " +
	"the Super+W wallpaper/theme picker, or QS Bar Settings for the bar and dock; then give the " +
	"command as the headless fallback and how you act. When you do make a change, say what " +
	"changed and how to see or undo it. When the user reports something broken, gather the logs yourself first: run " +
	"`ryoku-rashin logs <app>` and read logs.md, then diagnose before you touch anything. Before editing any file " +
	"check `ryoku owner <path>` and never edit a Ryoku-owned file."

// needleIdentity rides in front of the Ryoku lane's first turn, since ACP has
// no system prompt. Like quickPreamble the transcript records the raw
// question; only the agent sees this, once per session (the persona persists
// across later turns). The vault's AGENTS.md carries the same identity for
// agents that read a context file; this covers the ones that do not.
const needleIdentity = "[system: " + needlePersona + " Do not mention or repeat this note.] "

// hubs is every lane's hub, so a daemon-wide setting (the approval mode) can
// reach every surface whichever lane it joined.
var (
	hubsMu sync.Mutex
	hubs   []*chatHub
)

func newChatHub(l lane) *chatHub {
	h := &chatHub{
		lane:        l,
		clients:     map[*chatClient]bool{},
		last:        wsOut{Type: "state", State: "starting"},
		termCancels: map[string]context.CancelFunc{},
	}
	hubsMu.Lock()
	hubs = append(hubs, h)
	hubsMu.Unlock()
	return h
}

// allHubs snapshots the registry for a fan-out.
func allHubs() []*chatHub {
	hubsMu.Lock()
	defer hubsMu.Unlock()
	return append([]*chatHub(nil), hubs...)
}

// transcriptWorthy: the conversation itself, not ephemeral status. Open
// approvals are held apart in perms: once answered they are history.
func transcriptWorthy(t string) bool {
	switch t {
	case "user_text", "agent_text", "agent_thought", "tool", "turn_end":
		return true
	}
	return false
}

// recordLocked folds a frame into the session transcript; h.mu held. A run
// of text or thought chunks becomes one frame and a tool call keeps a single
// frame carrying its latest fields, so the replay is the conversation rather
// than thousands of fragments.
func (h *chatHub) recordLocked(m wsOut) {
	switch m.Type {
	case "permission":
		h.perms = append(h.perms, m)
		return
	case "permission_resolved":
		h.perms = slices.DeleteFunc(h.perms, func(p wsOut) bool { return p.RequestID == m.RequestID })
		return
	}
	if !transcriptWorthy(m.Type) {
		return
	}
	n := len(h.transcript)
	if (m.Type == "agent_text" || m.Type == "agent_thought") && n > 0 && h.transcript[n-1].Type == m.Type {
		h.transcript[n-1].Text += m.Text
		return
	}
	if m.Type == "tool" && m.ID != "" {
		for i := n - 1; i >= 0; i-- {
			if t := &h.transcript[i]; t.Type == "tool" && t.ID == m.ID {
				mergeTool(t, m)
				return
			}
		}
	}
	if n >= transcriptCap {
		h.transcript = slices.Clone(h.transcript[n-transcriptCap/2:])
	}
	h.transcript = append(h.transcript, m)
}

// mergeTool applies a partial tool update: an empty field means unchanged,
// and an auto-approval stays marked once made.
func mergeTool(t *wsOut, m wsOut) {
	for _, f := range []struct {
		dst *string
		src string
	}{
		{&t.Title, m.Title}, {&t.Kind, m.Kind}, {&t.Status, m.Status},
		{&t.Input, m.Input}, {&t.Output, m.Output},
	} {
		if f.src != "" {
			*f.dst = f.src
		}
	}
	if len(m.Diffs) > 0 {
		t.Diffs = m.Diffs
	}
	t.Auto = t.Auto || m.Auto
}

// resetConversationLocked forgets the transcript and any open approvals, for
// a new or loaded session; h.mu held.
func (h *chatHub) resetConversationLocked() {
	h.transcript = nil
	h.perms = nil
}

func (h *chatHub) broadcast(m wsOut) {
	h.broadcastExcept(m, nil)
}

// broadcastExcept sends to every client but skip (the author of a user_text
// already renders it locally).
func (h *chatHub) broadcastExcept(m wsOut, skip *chatClient) {
	h.mu.Lock()
	if m.Type == "state" {
		h.last = m
	}
	h.recordLocked(m)
	for c := range h.clients {
		if c == skip {
			continue
		}
		select {
		case c.out <- m:
		default: // slow client: drop frame rather than block the hub
		}
	}
	h.mu.Unlock()
}

// ensureConn starts hermes if there is no live session, and replaces one that
// outlived a hermes reconfigure (the process would keep answering with the
// provider and keys it loaded at spawn). Called with h.mu held.
func (h *chatHub) ensureConnLocked() (spawned bool) {
	if h.conn != nil && h.conn.stale() {
		old := h.conn
		h.conn = nil
		go old.Close()
	}
	if h.conn != nil {
		return false
	}
	h.last = wsOut{Type: "state", State: "starting"}
	if err := os.MkdirAll(h.lane.Cwd, 0o700); err != nil {
		h.conn = nil
		h.last = wsOut{Type: "state", State: "dead", Error: "lane dir: " + err.Error()}
		go h.broadcast(h.last)
		return false
	}
	conn, err := startACP(h.lane.Cwd)
	if err != nil {
		h.conn = nil
		h.last = wsOut{Type: "state", State: "dead", Error: err.Error()}
		go h.broadcast(h.last)
		return false
	}
	h.conn = conn
	h.introduced = false
	go func() {
		if err := conn.Initialize(h.lane.Cwd); err != nil {
			h.broadcast(wsOut{Type: "state", State: "dead", Error: err.Error()})
			h.dropConn(conn)
			return
		}
		h.broadcast(wsOut{Type: "state", State: "ready"})
	}()
	go h.pump(conn)
	return true
}

func (h *chatHub) dropConn(c *acpConn) {
	h.mu.Lock()
	if h.conn == c {
		h.conn = nil
	}
	h.mu.Unlock()
	c.Close()
}

// resetConn drops the live session so the next turn respawns it -- used when
// the chat backend changes, so switching agents takes effect immediately.
func (h *chatHub) resetConn() {
	h.mu.Lock()
	old := h.conn
	h.conn = nil
	h.mu.Unlock()
	if old != nil {
		old.Close()
	}
	// Clear the remembered model frame so a joiner (or the live chip) never
	// shows the previous backend's model; the next session re-emits its own.
	h.mu.Lock()
	h.models = wsOut{Type: "models", Current: "", Agent: ""}
	h.mu.Unlock()
	h.broadcast(h.models)
	h.broadcast(wsOut{Type: "state", State: "ready"})
}

func (h *chatHub) pump(c *acpConn) {
	for ev := range c.Events() {
		switch ev.Type {
		case "state":
			h.broadcast(wsOut{Type: "state", State: ev.State, Error: ev.Err})
			if ev.State == "dead" {
				h.releasePerms()
				h.dropConn(c)
			}
		case "agent_text":
			h.broadcast(wsOut{Type: "agent_text", Text: ev.Text})
		case "agent_thought":
			h.broadcast(wsOut{Type: "agent_thought", Text: ev.Text})
		case "user_text":
			h.broadcast(wsOut{Type: "user_text", Text: ev.Text})
		case "tool":
			t := ev.Tool
			h.broadcast(wsOut{Type: "tool", ID: t.ID, Title: t.Title, Kind: t.Kind, Status: t.Status,
				Input: t.Input, Output: t.Output, Diffs: t.Diffs, Auto: ev.ToolAuto})
		case "permission":
			h.broadcast(wsOut{Type: "permission", RequestID: ev.RequestID, ToolID: ev.Tool.ID,
				Title: ev.PermTitle, Kind: ev.Tool.Kind, Input: ev.Tool.Input, Options: ev.Options})
		case "permission_resolved":
			h.broadcast(wsOut{Type: "permission_resolved", RequestID: ev.RequestID, Outcome: ev.Outcome})
		case "turn_end":
			h.broadcast(wsOut{Type: "turn_end", StopReason: ev.StopReason})
			h.broadcast(wsOut{Type: "state", State: "ready"})
		case "models":
			m := wsOut{
				Type: "models", Models: ev.Models, Current: ev.CurrentModel, Agent: ev.AgentName,
				Prowl: ev.Prowl, Reason: ev.ProwlReason,
			}
			h.mu.Lock()
			h.models = m
			h.mu.Unlock()
			h.broadcast(m)
		case "commands":
			m := wsOut{Type: "commands", Commands: ev.Commands}
			h.mu.Lock()
			h.commands = m
			h.mu.Unlock()
			h.broadcast(m)
		case "session_info":
			h.broadcast(wsOut{Type: "session_info", SessionID: ev.SessionID, Title: ev.SessionTitle})
		case "usage":
			h.broadcast(wsOut{Type: "usage", Size: ev.UsageSize, Used: ev.UsageUsed})
		case "replay_start":
			h.broadcast(wsOut{Type: "replay_start"})
		case "replay_end":
			h.broadcast(wsOut{Type: "replay_end"})
		}
	}
	h.releasePerms()
	h.dropConn(c)
}

// releasePerms clears approvals a dead agent can no longer receive, on every
// surface.
func (h *chatHub) releasePerms() {
	h.mu.Lock()
	open := h.perms
	h.perms = nil
	h.mu.Unlock()
	for _, p := range open {
		h.broadcast(wsOut{Type: "permission_resolved", RequestID: p.RequestID, Outcome: "cancelled"})
	}
}

func (h *chatHub) handle(ctx context.Context, ws *websocket.Conn) {
	cl := &chatClient{ws: ws, out: make(chan wsOut, 1024)}
	h.mu.Lock()
	h.clients[cl] = true
	h.ensureConnLocked()
	greeting := []wsOut{h.last}
	if h.models.Type != "" {
		greeting = append(greeting, h.models)
	}
	if h.commands.Type != "" {
		greeting = append(greeting, h.commands)
	}
	greeting = append(greeting, approvalsFrame())
	// Replay the running conversation so a joiner lands mid-session with the
	// transcript already on screen (this is how a launcher ask is waiting in
	// the dashboard when the user clicks "continue"), then the approvals still
	// waiting, so any surface can answer one. An empty replay still goes out:
	// a client reconnecting to a restarted daemon must drop what it showed.
	replay := append([]wsOut{{Type: "replay_start"}}, h.transcript...)
	replay = append(replay, wsOut{Type: "replay_end"})
	replay = append(replay, h.perms...)
	h.mu.Unlock()

	writerDone := make(chan struct{})
	go func() {
		defer close(writerDone)
		for _, g := range greeting {
			_ = wsjson.Write(ctx, ws, g)
		}
		for _, r := range replay {
			_ = wsjson.Write(ctx, ws, r)
		}
		for m := range cl.out {
			if wsjson.Write(ctx, ws, m) != nil {
				return
			}
		}
	}()

	for {
		var in wsIn
		if wsjson.Read(ctx, ws, &in) != nil {
			break
		}
		if in.Type == "approvals" {
			h.setApprovals(in.Mode)
			continue
		}
		h.mu.Lock()
		spawned := false
		if in.Type == "user" || in.Type == "new" {
			spawned = h.ensureConnLocked()
		}
		conn := h.conn
		h.mu.Unlock()
		if conn == nil {
			continue
		}
		switch in.Type {
		case "user":
			// The author renders its own message; everyone else (and the
			// transcript) gets it as user_text. Quick asks ride a terse
			// preamble that only hermes sees; a lane's first real turn
			// carries the lane's intro, which the chat lane leaves empty.
			h.broadcastExcept(wsOut{Type: "user_text", Text: in.Text}, cl)
			h.broadcast(wsOut{Type: "state", State: "busy"})
			prompt := in.Text
			if in.Quick && h.lane.Quick {
				prompt = quickPreamble + prompt
			} else {
				h.mu.Lock()
				intro := !h.introduced && !strings.HasPrefix(strings.TrimSpace(in.Text), "/")
				if intro {
					h.introduced = true
				}
				h.mu.Unlock()
				if intro {
					prompt = h.lane.Intro() + prompt
				}
			}
			conn.Prompt(prompt, in.Images)
		case "cancel":
			conn.Cancel()
		case "permission":
			if id, err := strconv.ParseInt(in.RequestID, 10, 64); err == nil {
				conn.RespondPermission(id, in.OptionID)
			}
		case "set_model":
			if in.ModelID != "" {
				go func(c *acpConn, id string) {
					if err := c.SetModel(id); err != nil {
						h.mu.Lock()
						st := h.last.State
						h.mu.Unlock()
						h.broadcast(wsOut{Type: "state", State: st, Error: "model switch failed: " + err.Error()})
						return
					}
					saveSessionModel(id)
					h.mu.Lock()
					m := h.models
					m.Current = id
					h.models = m
					h.mu.Unlock()
					h.broadcast(m)
				}(conn, in.ModelID)
			}
		case "history":
			go func(c *acpConn, target *chatClient) {
				// The drawer shows this lane's own conversations: the
				// agent lists every session it stores, whichever cwd
				// opened it, and the lane keeps the ones that are its.
				sessions := slices.DeleteFunc(c.ListSessions(), func(s SessionMeta) bool {
					return !h.lane.ownsSession(s)
				})
				h.mu.Lock()
				if h.clients[target] {
					select {
					case target.out <- wsOut{Type: "history", Sessions: sessions}:
					default:
					}
				}
				h.mu.Unlock()
			}(conn, cl)
		case "load":
			if in.SessionID != "" {
				go func(c *acpConn, id string) {
					h.mu.Lock()
					h.resetConversationLocked() // the ACP replay rebuilds it
					h.introduced = true         // an existing session already introduced itself
					h.mu.Unlock()
					h.broadcast(wsOut{Type: "state", State: "busy"})
					if err := c.LoadSession(id); err != nil {
						h.broadcast(wsOut{Type: "state", State: "ready", Error: "load failed: " + err.Error()})
						return
					}
					h.broadcast(wsOut{Type: "state", State: "ready"})
				}(conn, in.SessionID)
			}
		case "new":
			go func(c *acpConn, fresh bool) {
				h.mu.Lock()
				h.resetConversationLocked()
				h.introduced = false
				h.mu.Unlock()
				h.broadcast(wsOut{Type: "replay_start"})
				h.broadcast(wsOut{Type: "replay_end"})
				// A process spawned for this request opens its own first
				// session in Initialize; asking for another would race it.
				if fresh {
					return
				}
				if err := c.NewSession(); err != nil {
					h.broadcast(wsOut{Type: "state", State: "dead", Error: err.Error()})
					return
				}
				h.broadcast(wsOut{Type: "state", State: "ready"})
			}(conn, spawned)
		}
	}

	h.mu.Lock()
	delete(h.clients, cl)
	close(cl.out)
	h.mu.Unlock()
	<-writerDone
}

// approvalsFrame reports the current approval mode to a surface.
func approvalsFrame() wsOut {
	return wsOut{Type: "approvals", Mode: LoadConfig().ApprovalsMode()}
}

// setApprovals stores the approval mode and tells every surface on every lane
// (the mode is one daemon-wide setting); an unknown mode is ignored rather
// than guessed at.
func (h *chatHub) setApprovals(mode string) {
	if mode != approvalsReadOnly && mode != approvalsAsk && mode != approvalsAuto {
		return
	}
	cfg := LoadConfig()
	cfg.Approvals = mode
	if err := SaveConfig(cfg); err != nil {
		return
	}
	frame := approvalsFrame()
	for _, peer := range allHubs() {
		peer.broadcast(frame)
	}
}

func serveVitalsWS(ctx context.Context, ws *websocket.Conn) {
	t := time.NewTicker(2 * time.Second)
	defer t.Stop()
	send := func() bool {
		b, err := json.Marshal(SampleVitals())
		if err != nil {
			return false
		}
		return ws.Write(ctx, websocket.MessageText, b) == nil
	}
	if !send() {
		return
	}
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			if !send() {
				return
			}
		}
	}
}
