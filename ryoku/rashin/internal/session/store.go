package session

import (
	"bufio"
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"

	"ryoku-rashin/internal/paths"
	"ryoku-rashin/internal/protocol"
)

const askRetention = 200

type metadata struct {
	protocol.SessionInfo
	Runner json.RawMessage `json:"runner,omitempty"`
}

// Log is one durable session event stream.
type Log struct {
	mu     sync.Mutex
	info   protocol.SessionInfo
	runner json.RawMessage
	file   *os.File
}

// NewID returns a chronologically sortable session id.
func NewID(kind protocol.SessionKind, at time.Time) (string, error) {
	var suffix [2]byte
	if _, err := rand.Read(suffix[:]); err != nil {
		return "", err
	}
	return fmt.Sprintf("%s-%s-%s-%s", kind, at.Local().Format("20060102"), at.Local().Format("150405"), hex.EncodeToString(suffix[:])), nil
}

// Create creates a session and enforces Ask retention.
func Create(info protocol.SessionInfo, runner json.RawMessage) (*Log, error) {
	if info.Created.IsZero() {
		info.Created = time.Now()
	}
	if info.ID == "" {
		id, err := NewID(info.Kind, info.Created)
		if err != nil {
			return nil, err
		}
		info.ID = id
	}
	if info.Updated.IsZero() {
		info.Updated = info.Created
	}
	dir := paths.SessionDir(info.ID)
	if err := paths.EnsureDir(dir); err != nil {
		return nil, err
	}
	f, err := os.OpenFile(filepath.Join(dir, "events.jsonl"), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		return nil, err
	}
	l := &Log{info: info, runner: append(json.RawMessage(nil), runner...), file: f}
	if err := l.persistMetaLocked(); err != nil {
		f.Close()
		return nil, err
	}
	if info.Kind == protocol.KindAsk {
		if err := retainAsk(info.ID); err != nil {
			f.Close()
			return nil, err
		}
	}
	return l, nil
}

// Open opens an existing session and repairs a truncated final event.
func Open(id string) (*Log, error) {
	if id == "" || filepath.Base(id) != id {
		return nil, os.ErrNotExist
	}
	m, err := readMeta(id)
	if err != nil {
		return nil, err
	}
	events, validBytes, err := loadEvents(filepath.Join(paths.SessionDir(id), "events.jsonl"))
	if err != nil {
		return nil, err
	}
	if len(events) > 0 {
		m.LastSeq = events[len(events)-1].Seq
	} else {
		m.LastSeq = 0
	}
	f, err := os.OpenFile(filepath.Join(paths.SessionDir(id), "events.jsonl"), os.O_CREATE|os.O_RDWR|os.O_APPEND, 0o600)
	if err != nil {
		return nil, err
	}
	if stat, statErr := f.Stat(); statErr == nil && stat.Size() != validBytes {
		if err := f.Truncate(validBytes); err != nil {
			f.Close()
			return nil, err
		}
	}
	l := &Log{info: m.SessionInfo, runner: append(json.RawMessage(nil), m.Runner...), file: f}
	if err := l.persistMetaLocked(); err != nil {
		f.Close()
		return nil, err
	}
	return l, nil
}

func readMeta(id string) (metadata, error) {
	var m metadata
	b, err := os.ReadFile(filepath.Join(paths.SessionDir(id), "meta.json"))
	if err != nil {
		return m, err
	}
	if err := json.Unmarshal(b, &m); err != nil {
		return m, err
	}
	return m, nil
}

func loadEvents(path string) ([]protocol.SessionEvent, int64, error) {
	f, err := os.Open(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, 0, nil
	}
	if err != nil {
		return nil, 0, err
	}
	defer f.Close()
	r := bufio.NewReader(f)
	var out []protocol.SessionEvent
	var valid int64
	for {
		line, readErr := r.ReadBytes('\n')
		if len(line) == 0 && readErr == io.EOF {
			break
		}
		var ev protocol.SessionEvent
		if json.Unmarshal(bytesTrimSpace(line), &ev) != nil || ev.Seq != int64(len(out)+1) {
			if readErr == io.EOF {
				break
			}
			peek, _ := r.Peek(1)
			if len(peek) == 0 {
				break
			}
			return nil, valid, fmt.Errorf("session event %d is corrupt", len(out)+1)
		}
		out = append(out, ev)
		valid += int64(len(line))
		if readErr == io.EOF {
			break
		}
	}
	return out, valid, nil
}

func bytesTrimSpace(b []byte) []byte {
	return bytes.TrimSpace(b)
}

// Append persists one event. Turn endings are synced before returning.
func (l *Log) Append(kind string, data any) (protocol.SessionEvent, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	raw, err := json.Marshal(data)
	if err != nil {
		return protocol.SessionEvent{}, err
	}
	if bytes.Equal(raw, []byte("null")) || bytes.Equal(raw, []byte("{}")) {
		raw = nil
	}
	ev := protocol.SessionEvent{Seq: l.info.LastSeq + 1, TS: time.Now(), Kind: kind, Data: raw}
	line, err := json.Marshal(ev)
	if err != nil {
		return protocol.SessionEvent{}, err
	}
	line = append(line, '\n')
	if _, err := l.file.Write(line); err != nil {
		return protocol.SessionEvent{}, err
	}
	l.info.LastSeq = ev.Seq
	l.info.Updated = ev.TS
	if kind == protocol.EvTurnEnd {
		if err := l.file.Sync(); err != nil {
			return protocol.SessionEvent{}, err
		}
	}
	if err := l.persistMetaLocked(); err != nil {
		return protocol.SessionEvent{}, err
	}
	return ev, nil
}

// AppendCopy preserves event identity so a fork replays exactly the selected history.
func (l *Log) AppendCopy(ev protocol.SessionEvent) error {
	l.mu.Lock()
	defer l.mu.Unlock()
	if ev.Seq != l.info.LastSeq+1 {
		return fmt.Errorf("copied session event sequence %d follows %d", ev.Seq, l.info.LastSeq)
	}
	line, err := json.Marshal(ev)
	if err != nil {
		return err
	}
	line = append(line, '\n')
	if _, err := l.file.Write(line); err != nil {
		return err
	}
	l.info.LastSeq = ev.Seq
	l.info.Updated = ev.TS
	if ev.Kind == protocol.EvTurnEnd {
		if err := l.file.Sync(); err != nil {
			return err
		}
	}
	return l.persistMetaLocked()
}

// Since returns events with seq greater than seq.
func (l *Log) Since(seq int64) ([]protocol.SessionEvent, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	events, _, err := loadEvents(filepath.Join(paths.SessionDir(l.info.ID), "events.jsonl"))
	if err != nil {
		return nil, err
	}
	i := sort.Search(len(events), func(i int) bool { return events[i].Seq > seq })
	return append([]protocol.SessionEvent(nil), events[i:]...), nil
}

func (l *Log) Info() protocol.SessionInfo {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.info
}

func (l *Log) Runner() json.RawMessage {
	l.mu.Lock()
	defer l.mu.Unlock()
	return append(json.RawMessage(nil), l.runner...)
}

// Update changes metadata atomically.
func (l *Log) Update(fn func(*protocol.SessionInfo), runner json.RawMessage) error {
	l.mu.Lock()
	defer l.mu.Unlock()
	fn(&l.info)
	l.info.Updated = time.Now()
	if runner != nil {
		l.runner = append(l.runner[:0], runner...)
	}
	return l.persistMetaLocked()
}

func (l *Log) persistMetaLocked() error {
	b, err := json.MarshalIndent(metadata{SessionInfo: l.info, Runner: l.runner}, "", "  ")
	if err != nil {
		return err
	}
	b = append(b, '\n')
	path := filepath.Join(paths.SessionDir(l.info.ID), "meta.json")
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, b, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

func (l *Log) Close() error {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.file == nil {
		return nil
	}
	if err := l.file.Sync(); err != nil {
		return err
	}
	err := l.file.Close()
	l.file = nil
	return err
}

// List returns metadata newest first.
func List(kind protocol.SessionKind) ([]protocol.SessionInfo, error) {
	entries, err := os.ReadDir(paths.SessionsDir())
	if errors.Is(err, os.ErrNotExist) {
		return []protocol.SessionInfo{}, nil
	}
	if err != nil {
		return nil, err
	}
	out := make([]protocol.SessionInfo, 0, len(entries))
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		m, err := readMeta(entry.Name())
		if err != nil || kind != "" && m.Kind != kind {
			continue
		}
		out = append(out, m.SessionInfo)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Created.Equal(out[j].Created) {
			return out[i].ID > out[j].ID
		}
		return out[i].Created.After(out[j].Created)
	})
	return out, nil
}

// Delete removes a session directory.
func Delete(id string) error {
	if id == "" || filepath.Base(id) != id {
		return os.ErrNotExist
	}
	return os.RemoveAll(paths.SessionDir(id))
}

func retainAsk(keep string) error {
	all, err := List(protocol.KindAsk)
	if err != nil {
		return err
	}
	for i := askRetention; i < len(all); i++ {
		if all[i].ID != keep {
			if err := Delete(all[i].ID); err != nil {
				return err
			}
		}
	}
	return nil
}
