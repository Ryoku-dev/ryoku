package teams

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	i18n "ryoku-i18n"

	"ryoku-rashin/internal/paths"
	"ryoku-rashin/internal/protocol"
)

// ledgerKeep is how many entries the ledger keeps.
const ledgerKeep = 5000

// ledgerSlack lets the ledger grow past ledgerKeep before it is rewritten,
// so a busy team does not rewrite the whole file on every entry.
const ledgerSlack = ledgerKeep / 10

// ledger is the teams' append-only record of what was done, by whom and
// whether anyone was watching: one protocol.Activity per line.
type ledger struct {
	mu    sync.Mutex
	path  string
	lines int
}

// openLedger opens ledger.jsonl under dir, counting its entries so appends
// know when to trim.
func openLedger(dir string) (*ledger, error) {
	if err := paths.EnsureDir(dir); err != nil {
		return nil, err
	}
	l := &ledger{path: filepath.Join(dir, "ledger.jsonl")}
	data, err := os.ReadFile(l.path)
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return nil, err
	}
	l.lines = bytes.Count(data, []byte{'\n'})
	return l, nil
}

// append writes one entry, rewriting the file to the newest ledgerKeep once
// it has grown past them.
func (l *ledger) append(a protocol.Activity) error {
	line, err := json.Marshal(a)
	if err != nil {
		return err
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	file, err := os.OpenFile(l.path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	_, writeErr := file.Write(append(line, '\n'))
	closeErr := file.Close()
	if writeErr != nil {
		return writeErr
	}
	if closeErr != nil {
		return closeErr
	}
	l.lines++
	if l.lines <= ledgerKeep+ledgerSlack {
		return nil
	}
	return l.trimLocked()
}

func (l *ledger) trimLocked() error {
	data, err := os.ReadFile(l.path)
	if err != nil {
		return err
	}
	lines := strings.Split(strings.TrimSuffix(string(data), "\n"), "\n")
	if len(lines) > ledgerKeep {
		lines = lines[len(lines)-ledgerKeep:]
	}
	if err := writeStateFile(filepath.Dir(l.path), filepath.Base(l.path), []byte(strings.Join(lines, "\n")+"\n")); err != nil {
		return err
	}
	l.lines = len(lines)
	return nil
}

// list pages the ledger newest first. Before is the RFC3339Nano time of the
// last entry the client shows; only older entries follow.
func (l *ledger) list(p protocol.TeamActivityParams) (protocol.ActivityResult, error) {
	limit := p.Limit
	if limit <= 0 {
		limit = 100
	}
	limit = min(limit, ledgerKeep)
	var before time.Time
	if p.Before != "" {
		parsed, err := time.Parse(time.RFC3339Nano, p.Before)
		if err != nil {
			return protocol.ActivityResult{}, protocol.Errorf(protocol.CodeBadRequest, "%s", i18n.T("The activity page marker is not a time."))
		}
		before = parsed
	}
	keep, err := activityFilter(p.Filter)
	if err != nil {
		return protocol.ActivityResult{}, err
	}
	l.mu.Lock()
	data, err := os.ReadFile(l.path)
	l.mu.Unlock()
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return protocol.ActivityResult{}, err
	}
	var entries []protocol.Activity
	scanner := bufio.NewScanner(bytes.NewReader(data))
	scanner.Buffer(make([]byte, 0, 64*1024), 16*1024*1024)
	for scanner.Scan() {
		var a protocol.Activity
		if json.Unmarshal(scanner.Bytes(), &a) != nil {
			continue
		}
		entries = append(entries, a)
	}
	out := make([]protocol.Activity, 0, min(limit, len(entries)))
	for i := len(entries) - 1; i >= 0 && len(out) < limit; i-- {
		a := entries[i]
		if !before.IsZero() && !a.Time.Before(before) {
			continue
		}
		if p.Team != "" && a.Team != p.Team {
			continue
		}
		if keep(a) {
			out = append(out, a)
		}
	}
	return protocol.ActivityResult{Activity: out}, nil
}

// activityFilter turns a teams.activity filter into a test on one entry.
func activityFilter(filter string) (func(protocol.Activity) bool, error) {
	switch filter {
	case "", protocol.ActivityAll:
		return func(protocol.Activity) bool { return true }, nil
	case protocol.ActivityUnwatched:
		return func(a protocol.Activity) bool { return a.Unwatched }, nil
	case protocol.ActivityApprovals:
		return func(a protocol.Activity) bool { return strings.HasPrefix(a.Action, "approval.") }, nil
	case protocol.ActivityCredentials:
		return func(a protocol.Activity) bool {
			return strings.HasPrefix(a.Action, "credential.") || strings.HasPrefix(a.Action, "mail.")
		}, nil
	case protocol.ActivityFailures:
		return func(a protocol.Activity) bool { return !a.OK }, nil
	}
	return nil, protocol.Errorf(protocol.CodeBadRequest, "%s", i18n.Tf("%s is not an activity filter.", filter))
}

// record fills an entry's time and whether anyone was watching, keeps any
// credential value out of its detail, and appends it to the ledger. A ledger
// that cannot be written does not stop the action it records.
func (s *Service) record(a protocol.Activity) {
	a.Time = s.now()
	a.Unwatched = s.nobodyWatching()
	if a.Detail != "" {
		a.Detail = s.redact(a.Detail)
	}
	_ = s.ledger.append(a)
}
