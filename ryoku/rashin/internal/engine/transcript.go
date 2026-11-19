package engine

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	"ryoku-rashin/internal/engine/tool"
	"ryoku-rashin/internal/llm"
	"ryoku-rashin/internal/paths"
	"ryoku-rashin/internal/protocol"
)

// TranscriptVersion distinguishes engine transcripts from old runner state.
const TranscriptVersion = 1

// TranscriptToolCap bounds one stored tool result.
const TranscriptToolCap = 32 << 10

// Rule reminders and interrupts are user messages that must outlive summaries.
var injectionPrefixes = []string{
	`<system-interrupt reason="rule_violation"`,
	`<system-reminder reason="rule_violation"`,
}

func isInjection(m llm.Message) bool {
	if m.Role != llm.RoleUser {
		return false
	}
	for _, p := range injectionPrefixes {
		if strings.HasPrefix(m.Content, p) {
			return true
		}
	}
	return false
}

// Compaction kinds.
const (
	compactSummary = "summary"
	compactNotes   = "notes"
)

// record is every engine.jsonl line that is not a message. Exactly one
// field is set. Seq numbers count message lines from 0.
type record struct {
	Turn       *int              `json:"turn,omitempty"`
	Compaction *compactionRecord `json:"compaction,omitempty"`
	Prune      *pruneRecord      `json:"prune,omitempty"`
	Revert     *int              `json:"revert,omitempty"`
	Notes      *notesRecord      `json:"notes,omitempty"`
}

func (r *record) empty() bool {
	return r.Turn == nil && r.Compaction == nil && r.Prune == nil && r.Revert == nil && r.Notes == nil
}

// compactionRecord replaces messages before Through with a head message
// (summary and/or notes), keeping the messages in Keep verbatim: the last
// user turn and rule injections.
type compactionRecord struct {
	Kind    string `json:"kind"`
	Summary string `json:"summary,omitempty"`
	Notes   string `json:"notes,omitempty"`
	Through int    `json:"through"`
	Keep    []int  `json:"keep,omitempty"`
}

type pruneRecord struct {
	Stubs []pruneStub `json:"stubs"`
}

type pruneStub struct {
	Seq  int    `json:"seq"`
	Text string `json:"text"`
}

type notesRecord struct {
	Text string `json:"text"`
}

// line is one engine.jsonl line: a message or a record.
type line struct {
	msg *llm.Message
	rec *record
}

func (l line) marshal() ([]byte, error) {
	if l.msg != nil {
		return json.Marshal(l.msg)
	}
	return json.Marshal(l.rec)
}

func parseLine(b []byte) (line, error) {
	var rec record
	if err := json.Unmarshal(b, &rec); err != nil {
		return line{}, err
	}
	if !rec.empty() {
		return line{rec: &rec}, nil
	}
	var m llm.Message
	if err := json.Unmarshal(b, &m); err != nil {
		return line{}, err
	}
	if m.Role == "" {
		return line{}, errors.New("line is neither a message nor a record")
	}
	return line{msg: &m}, nil
}

// revertMark is a "files were reverted" note told before message Seq.
type revertMark struct {
	seq  int
	turn int
}

// Anchor is the provider's count of the previous prompt and reply. Extra
// estimates raw messages appended since that reply, independent of synthetic
// dangling tool results in the model-facing view.
type Anchor struct {
	Tokens int
	Tail   int
	Extra  int
}

// Transcript is one session's model-facing history: every message the
// engine sent or received, plus compaction, pruning, notes and revert
// records. A persisted session writes each change to
// paths.EngineTranscript(id) as it happens; a temporary one, and a
// subagent's, lives in memory only. Messages is the view the next request
// sends. A Transcript serves one runner: the turn goroutine and the tools it
// runs, never two turns at once.
type Transcript struct {
	mu   sync.Mutex
	id   string
	path string // "" in memory

	lines []line
	msgs  []llm.Message
	comp  *compactionRecord
	stubs map[int]string
	marks []revertMark
	notes string
	// turnSeq is the seq of the first message of the latest turn.
	turnSeq int

	anchorTokens int
	anchorSeq    int
	rollover     bool
}

var (
	liveMu sync.Mutex
	// live holds the open transcripts by session id: the in-memory ones of
	// temporary sessions until DropTranscript, and the persisted ones the
	// context tools find through their session.
	live = map[string]*Transcript{}
)

func lookupTranscript(id string) *Transcript {
	liveMu.Lock()
	defer liveMu.Unlock()
	return live[id]
}

// OpenTranscript opens the transcript of the session info names: the file of
// a persisted session, the in-memory one of a temporary session (created on
// first use), and registers it for the context tools. It never fails: an
// unreadable file is logged and the session starts empty. resume is the
// runner state of the previous runner (State's value, or a runner state that
// nests it under "transcript"); lost reports that it belonged to history this
// engine cannot replay (a harness session, a missing file), so the caller
// tells the user the conversation starts fresh.
func OpenTranscript(info protocol.SessionInfo, resume json.RawMessage) (t *Transcript, lost bool) {
	state, ours := parseState(resume)
	lost = len(bytes.TrimSpace(resume)) > 0 && string(bytes.TrimSpace(resume)) != "null" && !ours

	liveMu.Lock()
	defer liveMu.Unlock()
	if info.Temporary {
		if t := live[info.ID]; t != nil {
			return t, lost
		}
		t = newTranscript(info.ID, "")
		live[info.ID] = t
		return t, lost
	}
	t = newTranscript(info.ID, paths.EngineTranscript(info.ID))
	lines, err := readLines(t.path)
	if err != nil {
		log.Printf("engine: transcript of %s unreadable, starting fresh: %v", info.ID, err)
	}
	for _, l := range lines {
		t.apply(l)
	}
	if ours && state.Messages > 0 && len(t.msgs) == 0 {
		lost = true
	}
	live[info.ID] = t
	return t, lost
}

// NewMemoryTranscript is an unregistered in-memory transcript, for a
// subagent's run inside its parent's turn.
func NewMemoryTranscript() *Transcript { return newTranscript("", "") }

// DropTranscript forgets a session's open transcript: call it when a
// temporary session ends, so its history is freed.
func DropTranscript(id string) {
	liveMu.Lock()
	defer liveMu.Unlock()
	delete(live, id)
}
func releaseTranscript(id string, tr *Transcript) {
	liveMu.Lock()
	defer liveMu.Unlock()
	if live[id] == tr {
		delete(live, id)
	}
}

func newTranscript(id, path string) *Transcript {
	return &Transcript{id: id, path: path, stubs: map[int]string{}, turnSeq: -1}
}

// transcriptState is what State returns.
type transcriptState struct {
	Version    int    `json:"version"`
	Transcript string `json:"transcript,omitempty"`
	Compacted  int    `json:"compacted,omitempty"`
	Messages   int    `json:"messages"`
}

func parseState(raw json.RawMessage) (transcriptState, bool) {
	raw = bytes.TrimSpace(raw)
	if len(raw) == 0 || raw[0] != '{' {
		return transcriptState{}, false
	}
	var probe map[string]json.RawMessage
	if json.Unmarshal(raw, &probe) != nil {
		return transcriptState{}, false
	}
	// A runner state nests ours under "transcript"; ours names the file there.
	if nested, ok := probe["transcript"]; ok {
		if s, ok := parseState(nested); ok {
			return s, true
		}
	}
	if _, ok := probe["version"]; ok {
		var s transcriptState
		if json.Unmarshal(raw, &s) == nil && s.Version == TranscriptVersion {
			return s, true
		}
	}
	return transcriptState{}, false
}

// State is the transcript's part of the runner state: its format version,
// the file name ("" in memory), the seq the last compaction reached and how
// many messages it holds.
func (t *Transcript) State() json.RawMessage {
	t.mu.Lock()
	defer t.mu.Unlock()
	s := transcriptState{Version: TranscriptVersion, Messages: len(t.msgs)}
	if t.path != "" {
		s.Transcript = paths.EngineTranscriptName
	}
	if t.comp != nil {
		s.Compacted = t.comp.Through
	}
	b, _ := json.Marshal(s)
	return b
}

// BeginTurn marks the start of a user turn, before its user message is
// appended. turn is core.PromptInput.Turn: forks and reverts count turns
// the same way.
func (t *Transcript) BeginTurn(turn int) error {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.add(line{rec: &record{Turn: &turn}})
}

// Append adds messages in order. A tool message is cut at
// TranscriptToolCap. The messages are in the view even when writing them
// failed; the error says the file lags.
func (t *Transcript) Append(msgs ...llm.Message) error {
	t.mu.Lock()
	defer t.mu.Unlock()
	add := make([]line, 0, len(msgs))
	for _, m := range msgs {
		if m.Role == llm.RoleTool && len(m.Content) > TranscriptToolCap {
			const contentRoom = TranscriptToolCap - 128
			m.Content = tool.HeadTail(m.Content, contentRoom/2, contentRoom/2)
		}
		m := m
		add = append(add, line{msg: &m})
	}
	return t.add(add...)
}

// SetUsage anchors later estimates on the provider's count. The raw sequence
// excludes synthetic unanswered-tool stubs, which real results later replace.
func (t *Transcript) SetUsage(u llm.Usage) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if u.Estimated || u.Input <= 0 {
		t.anchorTokens, t.anchorSeq = 0, 0
		return
	}
	t.anchorTokens = u.Input + u.Output
	t.anchorSeq = len(t.msgs)
}

// Anchor is the provider count the next request's prompt builds on.
func (t *Transcript) Anchor() Anchor {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.anchor()
}

func (t *Transcript) anchor() Anchor {
	if t.anchorTokens <= 0 || t.anchorSeq > len(t.msgs) {
		return Anchor{}
	}
	var extra int
	for seq := t.anchorSeq; seq < len(t.msgs); seq++ {
		extra += llm.EstimateMessage(t.message(seq))
	}
	return Anchor{Tokens: t.anchorTokens, Extra: extra + extra/10}
}

// Messages is what the next request sends after the system prompt: the
// compaction head (summary, notes), the messages it kept, then every later
// message with pruned tool results as stubs and revert notes in place.
func (t *Transcript) Messages() []llm.Message {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.view()
}

// Len is len(Messages()).
func (t *Transcript) Len() int {
	t.mu.Lock()
	defer t.mu.Unlock()
	return len(t.view())
}

// Notes is the context notebook, "" when none is saved.
func (t *Transcript) Notes() string {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.notes
}

// SetNotes replaces the context notebook; "" clears it.
func (t *Transcript) SetNotes(text string) error {
	if len(text) > MaxNotesBytes {
		return fmt.Errorf("context notes exceed %d bytes", MaxNotesBytes)
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.add(line{rec: &record{Notes: &notesRecord{Text: text}}})
}

// requestRollover asks for a fresh window once the round ends.
func (t *Transcript) requestRollover() {
	t.mu.Lock()
	t.rollover = true
	t.mu.Unlock()
}

// Rollover starts the fresh window new_context asked for: call it after a
// round's tool results are appended. The window keeps the notes, the last
// user turn and rule injections; everything else stays in the file only. It
// reports whether it rolled over, so the caller rebuilds its messages.
func (t *Transcript) Rollover() bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	if !t.rollover {
		return false
	}
	t.rollover = false
	through := len(t.msgs)
	rec := &compactionRecord{Kind: compactNotes, Notes: t.notes, Through: through, Keep: t.keepBefore(through)}
	if err := t.add(line{rec: &record{Compaction: rec}}); err != nil {
		log.Printf("engine: transcript of %s: %v", t.id, err)
	}
	return true
}

// lastUserSeq is the user message that opened the latest turn, else the
// latest user message that is not a rule injection; -1 when none.
func (t *Transcript) lastUserSeq() int {
	if t.turnSeq >= 0 && t.turnSeq < len(t.msgs) {
		for s := t.turnSeq; s < len(t.msgs); s++ {
			if t.msgs[s].Role == llm.RoleUser && !isInjection(t.msgs[s]) {
				return s
			}
		}
	}
	for s := len(t.msgs) - 1; s >= 0; s-- {
		if t.msgs[s].Role == llm.RoleUser && !isInjection(t.msgs[s]) {
			return s
		}
	}
	return -1
}

// keepBefore preserves the latest turn and every rule injection across a
// context reset, including user steering delivered during the turn.
func (t *Transcript) keepBefore(through int) []int {
	var keep []int
	seen := map[string]bool{}
	for s := 0; s < through && s < len(t.msgs); s++ {
		if isInjection(t.msgs[s]) && !seen[t.msgs[s].Content] {
			seen[t.msgs[s].Content] = true
			keep = append(keep, s)
		}
	}
	if t.turnSeq >= 0 && t.turnSeq < through {
		for s := t.turnSeq; s < through; s++ {
			if t.msgs[s].Role == llm.RoleUser && !isInjection(t.msgs[s]) {
				keep = append(keep, s)
			}
		}
	} else if u := t.lastUserSeq(); u >= 0 && u < through {
		keep = append(keep, u)
	}
	sort.Ints(keep)
	return keep
}

// add writes lines to the file (when persisted) and applies them.
func (t *Transcript) add(lines ...line) error {
	var err error
	if t.path != "" {
		err = appendLines(t.path, lines)
	}
	for _, l := range lines {
		t.apply(l)
	}
	return err
}

func (t *Transcript) apply(l line) {
	t.lines = append(t.lines, l)
	if l.msg != nil {
		t.msgs = append(t.msgs, *l.msg)
		return
	}
	r := l.rec
	switch {
	case r.Turn != nil:
		t.turnSeq = len(t.msgs)
	case r.Compaction != nil:
		t.comp = r.Compaction
		t.anchorTokens, t.anchorSeq = 0, 0
	case r.Prune != nil:
		for _, s := range r.Prune.Stubs {
			t.stubs[s.Seq] = s.Text
		}
		t.anchorTokens, t.anchorSeq = 0, 0
	case r.Revert != nil:
		t.marks = append(t.marks, revertMark{seq: len(t.msgs), turn: *r.Revert})
	case r.Notes != nil:
		t.notes = r.Notes.Text
	}
}

// from is the first seq of the tail the view sends whole.
func (t *Transcript) from() int {
	if t.comp == nil {
		return 0
	}
	return t.comp.Through
}

func (t *Transcript) message(seq int) llm.Message {
	m := t.msgs[seq]
	if stub, ok := t.stubs[seq]; ok {
		m.Content = stub
		m.Images = nil
	}
	return m
}

func (t *Transcript) view() []llm.Message {
	var out []llm.Message
	from := t.from()
	if c := t.comp; c != nil {
		if head := compactionHead(c); head != "" {
			out = append(out, llm.Message{Role: llm.RoleUser, Content: head})
		}
		for _, s := range c.Keep {
			if s >= 0 && s < len(t.msgs) {
				out = append(out, t.message(s))
			}
		}
	}
	marks := t.marks
	for s := from; s <= len(t.msgs); s++ {
		for len(marks) > 0 && marks[0].seq <= s {
			if marks[0].seq == s {
				out = append(out, llm.Message{Role: llm.RoleUser, Content: revertNote(marks[0].turn)})
			}
			marks = marks[1:]
		}
		if s < len(t.msgs) {
			out = append(out, t.message(s))
		}
	}
	return mergeUsers(answerDangling(out))
}

// answerDangling gives every tool call a result: a turn cancelled between a
// call and its result would otherwise make every later request invalid.
func answerDangling(ms []llm.Message) []llm.Message {
	var out []llm.Message
	var pending []llm.ToolCall
	flush := func() {
		for _, c := range pending {
			out = append(out, llm.Message{Role: llm.RoleTool, ToolCallID: c.ID, Name: c.Name, Content: "Stopped before it ran.", IsError: true})
		}
		pending = nil
	}
	for _, m := range ms {
		if m.Role == llm.RoleTool {
			for i, c := range pending {
				if c.ID == m.ToolCallID {
					pending = append(pending[:i:i], pending[i+1:]...)
					break
				}
			}
			out = append(out, m)
			continue
		}
		flush()
		out = append(out, m)
		if m.Role == llm.RoleAssistant {
			pending = append([]llm.ToolCall(nil), m.ToolCalls...)
		}
	}
	flush()
	return out
}

// mergeUsers joins adjacent user messages: chat templates that insist on
// alternating roles reject two in a row, and a head, a kept turn and a note
// read as one message anyway.
func mergeUsers(ms []llm.Message) []llm.Message {
	out := ms[:0:0]
	for _, m := range ms {
		if n := len(out); n > 0 && m.Role == llm.RoleUser && out[n-1].Role == llm.RoleUser {
			prev := &out[n-1]
			switch {
			case prev.Content == "":
				prev.Content = m.Content
			case m.Content != "":
				prev.Content += "\n\n" + m.Content
			}
			if len(m.Images) > 0 {
				prev.Images = append(append([]llm.Image(nil), prev.Images...), m.Images...)
			}
			prev.Cache = m.Cache
			continue
		}
		out = append(out, m)
	}
	return out
}

func revertNote(turn int) string {
	return fmt.Sprintf("<system-reminder>The user reverted the files to how they were before turn %d. Read a file again before you change it.</system-reminder>", turn)
}

// ReadTranscript returns every message of a session in order, as the model
// sent and received them (before pruning and compaction views): from memory
// for an open temporary session, else from paths.EngineTranscript(id). A
// session without a transcript returns nil, nil.
func ReadTranscript(id string) ([]llm.Message, error) {
	if t := lookupTranscript(id); t != nil {
		t.mu.Lock()
		defer t.mu.Unlock()
		return append([]llm.Message(nil), t.msgs...), nil
	}
	lines, err := readLines(paths.EngineTranscript(id))
	var out []llm.Message
	for _, l := range lines {
		if l.msg != nil {
			out = append(out, *l.msg)
		}
	}
	return out, err
}

// CopyTranscript copies a session's transcript into a fork, up to the end of
// turn turns (core.PromptInput.Turn numbering). Each side is in memory when
// its session is temporary, a file otherwise. A missing source copies
// nothing; turns <= 0 copies nothing; turns past the last copies all.
func CopyTranscript(from, to protocol.SessionInfo, turns int) error {
	if turns <= 0 {
		return nil
	}
	var lines []line
	if from.Temporary {
		t := lookupTranscript(from.ID)
		if t == nil {
			return nil
		}
		t.mu.Lock()
		lines = append(lines, t.lines...)
		t.mu.Unlock()
	} else {
		var err error
		if lines, err = readLines(paths.EngineTranscript(from.ID)); err != nil {
			return err
		}
	}
	for i, l := range lines {
		if l.rec != nil && l.rec.Turn != nil && *l.rec.Turn > turns {
			lines = lines[:i]
			break
		}
	}
	if len(lines) == 0 {
		return nil
	}
	if to.Temporary {
		t := newTranscript(to.ID, "")
		for _, l := range lines {
			t.apply(l)
		}
		liveMu.Lock()
		live[to.ID] = t
		liveMu.Unlock()
		return nil
	}
	return writeLines(paths.EngineTranscript(to.ID), lines)
}

// NoteRevert records that the user reverted the files to before turn: the
// next request tells the model so. A session without a transcript has
// nothing to tell.
func NoteRevert(info protocol.SessionInfo, turn int) error {
	rec := line{rec: &record{Revert: &turn}}
	if t := lookupTranscript(info.ID); t != nil {
		t.mu.Lock()
		defer t.mu.Unlock()
		return t.add(rec)
	}
	if info.Temporary {
		return nil
	}
	path := paths.EngineTranscript(info.ID)
	if _, err := os.Stat(path); err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil
		}
		return err
	}
	return appendLines(path, []line{rec})
}

// readLines reads a transcript file; a missing file is no lines. Lines that
// do not parse are skipped (a torn last write) and reported.
func readLines(path string) ([]line, error) {
	f, err := os.Open(path)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	defer f.Close()
	var out []line
	var bad int
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 64<<10), 64<<20)
	for sc.Scan() {
		b := bytes.TrimSpace(sc.Bytes())
		if len(b) == 0 {
			continue
		}
		l, err := parseLine(b)
		if err != nil {
			bad++
			continue
		}
		out = append(out, l)
	}
	if err := sc.Err(); err != nil {
		return out, err
	}
	if bad > 0 {
		return out, fmt.Errorf("%s: %d unreadable lines skipped", path, bad)
	}
	return out, nil
}

func encodeLines(lines []line) ([]byte, error) {
	var buf bytes.Buffer
	for _, l := range lines {
		b, err := l.marshal()
		if err != nil {
			return nil, err
		}
		buf.Write(b)
		buf.WriteByte('\n')
	}
	return buf.Bytes(), nil
}

func appendLines(path string, lines []line) error {
	data, err := encodeLines(lines)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	f, err := os.OpenFile(path, os.O_RDWR|os.O_APPEND|os.O_CREATE, 0o600)
	if err != nil {
		return err
	}
	if stat, serr := f.Stat(); serr == nil && stat.Size() > 0 {
		var last [1]byte
		if _, serr = f.ReadAt(last[:], stat.Size()-1); serr == nil && last[0] != '\n' {
			_, err = f.Write([]byte{'\n'})
		}
	}
	if err == nil {
		_, err = f.Write(data)
	}
	if cerr := f.Close(); err == nil {
		err = cerr
	}
	return err
}

func writeLines(path string, lines []line) error {
	data, err := encodeLines(lines)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return err
	}
	if err := os.Rename(tmp, path); err != nil {
		os.Remove(tmp)
		return err
	}
	return nil
}
