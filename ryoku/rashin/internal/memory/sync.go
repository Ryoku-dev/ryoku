package memory

import (
	"os"
	"path/filepath"
	"sort"

	"ryoku-rashin/internal/memory/scan"
	"ryoku-rashin/internal/protocol"
)

// Sync reads back every memory file that changed since the store last read
// or wrote it. An edited line becomes a new version of its memory, a deleted
// line is forgotten and tombstoned, a new line becomes a memory, and a line
// that reads as an instruction or a secret stays on disk, blocked. What it
// found is one change, kept for takeFileChanges. A file left behind the
// ledger by a failed write is brought up to it first.
func (s *Store) Sync() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.syncLocked()
}

// takeFileChanges hands over the changes hand edits made since the last
// call.
func (s *Store) takeFileChanges() []protocol.MemoryChange {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := s.fileChanges
	s.fileChanges = nil
	return out
}

// syncLocked is Sync with the store locked.
func (s *Store) syncLocked() error {
	s.syncing = true
	defer func() { s.syncing = false }()
	if err := s.writePending(); err != nil {
		return err
	}
	changed := map[string]string{}
	for path := range s.stamps {
		if stamp, ok := stat(path); !ok || stamp != s.stamps[path] {
			changed[path] = s.pathScope(path)
		}
	}
	patterns := []string{filepath.Join(s.cfg.Dir, "you.md"), filepath.Join(s.cfg.Dir, "projects", "*.md"), filepath.Join(s.cfg.TeamDir, "*.md")}
	for _, pattern := range patterns {
		matches, _ := filepath.Glob(pattern)
		for _, path := range matches {
			if _, known := s.stamps[path]; !known {
				changed[path] = s.pathScope(path)
			}
		}
	}
	if len(changed) == 0 {
		return nil
	}
	paths := make([]string, 0, len(changed))
	for path, scope := range changed {
		if scope != "" {
			paths = append(paths, path)
		}
	}
	sort.Strings(paths)
	files := map[string]*mdFile{}
	seen := map[string]bool{}
	for _, path := range paths {
		data, err := os.ReadFile(path)
		if os.IsNotExist(err) {
			files[path] = &mdFile{}
			continue
		}
		if err != nil {
			return err
		}
		files[path] = parseMarkdown(string(data))
		for _, line := range files[path].facts() {
			if line.id != "" {
				seen[line.id] = true
			}
		}
	}
	t := s.begin(Origin{By: protocol.MemoryByFile})
	for _, path := range paths {
		if _, ok := stat(path); !ok {
			if _, err := os.Stat(filepath.Dir(path)); err != nil {
				// the folder itself is gone, an unmounted vault rather than a deleted file
				continue
			}
		}
		t.reconcile(changed[path], files[path], seen)
	}
	change, err := t.commit()
	if err != nil {
		return err
	}
	for _, path := range paths {
		if err := s.recordStamp(path); err != nil {
			return err
		}
	}
	if len(change.Items) > 0 {
		s.fileChanges = append(s.fileChanges, change)
	}
	return nil
}

// reconcile brings the ledger in line with one parsed file of scope. seen
// holds every id on a line in any changed file, so a line moved to another
// file is moved, not forgotten.
func (t *txn) reconcile(scope string, file *mdFile, seen map[string]bool) {
	s := t.s
	here := map[string]bool{}
	for _, line := range file.facts() {
		if s.inflight[line.id] {
			// a change is writing this memory now; the line on disk is from before it
			here[line.id] = true
			continue
		}
		f := s.facts[line.id]
		if line.id == "" || f == nil || here[line.id] || !f.inFile() && f.Scope == scope {
			here[t.fileFact(scope, line)] = true
			continue
		}
		here[line.id] = true
		text := cleanText(line.text)
		changedText := text != f.Text
		status := protocol.MemoryActive
		if line.section == sectionReplaced {
			status = protocol.MemoryReplaced
		}
		blocked := ""
		if th, hit := scan.Strict(text); hit {
			// a replaced line is scanned too, so restoring it never brings an instruction back
			blocked = threatReason(th)
			if status == protocol.MemoryActive {
				status = protocol.MemoryBlocked
			}
		}
		if !changedText && f.Scope == scope && f.Status == status && f.Blocked == blocked {
			continue
		}
		t.touch(f.ID)
		if f.Scope != scope {
			t.dirty[f.Scope] = true
			f.Scope = scope
		}
		if changedText {
			f.Versions = pushVersion(f.Versions, protocol.MemoryVersion{Text: f.Text, At: f.Updated, By: protocol.MemoryByFile})
			f.Text, f.Vector = text, nil
		}
		if status == protocol.MemoryReplaced && f.Status != protocol.MemoryReplaced {
			at := t.now
			f.Replaced = &at
		}
		if status != protocol.MemoryReplaced {
			f.Replaced, f.ReplacedBy = nil, ""
		}
		f.Status, f.Blocked, f.Updated = status, blocked, t.now
		if blocked == "" {
			t.item(f, protocol.MemoryOpEdited, "")
		}
	}
	for _, f := range s.facts {
		if f.Scope == scope && f.inFile() && !here[f.ID] && !seen[f.ID] && !s.inflight[f.ID] {
			t.forget(f, protocol.MemoryOpForgot, true)
		}
	}
}

// fileFact makes a memory of a line written by hand, blocked when it reads
// as an instruction or a secret, and returns its id.
func (t *txn) fileFact(scope string, line mdLine) string {
	s := t.s
	f := &fact{ID: s.newID(), Scope: scope, Text: cleanText(line.text), Status: protocol.MemoryActive, Kind: protocol.MemoryDurable,
		Source: protocol.MemorySource{Kind: protocol.MemoryFromFile}, Explicit: true, Created: t.now, Updated: t.now}
	if line.section == sectionReplaced {
		at := t.now
		f.Status, f.Replaced = protocol.MemoryReplaced, &at
	}
	t.touch(f.ID)
	s.facts[f.ID] = f
	if th, hit := scan.Strict(f.Text); hit {
		f.Blocked = threatReason(th)
		if f.Status == protocol.MemoryActive {
			f.Status = protocol.MemoryBlocked
		}
		return f.ID
	}
	if f.Status == protocol.MemoryActive {
		t.learned++
		t.item(f, protocol.MemoryOpRemembered, "")
	}
	return f.ID
}
