package session

import (
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"

	"ryoku-rashin/internal/paths"
	"ryoku-rashin/internal/protocol"
	"ryoku-rashin/internal/redact"
)

// ErrMoved is a stored session whose log or metadata changed between a
// PrepareScrub and its Commit; the scrub is prepared again later.
var ErrMoved = errors.New("the session changed while it was scrubbed")

// StoredScrub is the scrub of a session nothing has open, prepared in full
// beside its files without holding anything, so the caller holds its own
// lock only for the check and the renames of Commit.
type StoredScrub struct {
	events, meta       string
	eventsTmp, metaTmp string
	eventsSeen         fs.FileInfo
	metaSeen           fs.FileInfo
}

// PrepareScrub reads a stored session's whole log and metadata and runs
// scrub over every event, the deltas of each stream merged first, and over
// the title and preview. When scrub changed nothing it returns nil; else
// the scrubbed files wait beside the session's own until Commit or
// Discard. A truncated final event is left out, as Open would leave it. A
// session that is gone gives an error that is fs.ErrNotExist.
func PrepareScrub(id string, scrub func(string) string) (*StoredScrub, error) {
	if id == "" || filepath.Base(id) != id {
		return nil, fs.ErrNotExist
	}
	dir := paths.SessionDir(id)
	p := &StoredScrub{events: filepath.Join(dir, "events.jsonl"), meta: filepath.Join(dir, "meta.json")}
	var err error
	// seen before reading, so a write after the read is a change Commit sees
	if p.eventsSeen, err = os.Stat(p.events); err != nil {
		return nil, err
	}
	if p.metaSeen, err = os.Stat(p.meta); err != nil {
		return nil, err
	}
	m, err := readMeta(id)
	if err != nil {
		return nil, err
	}
	events, _, valid, err := readEvents(p.events, 0, 1)
	if err != nil {
		return nil, err
	}
	// the tokens it finds are learned for this session's other events
	var learned redact.Learner
	scrub = learned.Wrap(scrub)
	changed := scrubLearning(events, &learned, scrub)
	title, preview := scrub(m.Title), scrub(m.Preview)
	metaChanged := title != m.Title || preview != m.Preview
	if !changed && !metaChanged && valid == p.eventsSeen.Size() {
		return nil, nil
	}
	if changed || valid != p.eventsSeen.Size() {
		p.eventsTmp = p.events + ".scrub"
		if err := writeEvents(p.eventsTmp, events); err != nil {
			p.Discard()
			return nil, err
		}
	}
	if metaChanged {
		m.Title, m.Preview = title, preview
		raw, err := json.MarshalIndent(m, "", "  ")
		if err != nil {
			p.Discard()
			return nil, err
		}
		p.metaTmp = p.meta + ".scrub"
		if err := writeSynced(p.metaTmp, append(raw, '\n')); err != nil {
			p.Discard()
			return nil, err
		}
	}
	return p, nil
}

// Commit puts the scrubbed files in place, each in one rename, if neither
// of the session's files changed since PrepareScrub read them; else it
// returns ErrMoved and changes nothing. The caller holds whatever keeps the
// session from being opened meanwhile.
func (p *StoredScrub) Commit() error {
	for _, file := range []struct {
		path string
		seen fs.FileInfo
	}{{p.events, p.eventsSeen}, {p.meta, p.metaSeen}} {
		now, err := os.Stat(file.path)
		if err != nil {
			return err
		}
		if !os.SameFile(now, file.seen) || now.Size() != file.seen.Size() || !now.ModTime().Equal(file.seen.ModTime()) {
			return ErrMoved
		}
	}
	if p.eventsTmp != "" {
		if err := os.Rename(p.eventsTmp, p.events); err != nil {
			return err
		}
		p.eventsTmp = ""
	}
	if p.metaTmp != "" {
		if err := os.Rename(p.metaTmp, p.meta); err != nil {
			return err
		}
		p.metaTmp = ""
	}
	if dir, err := os.Open(filepath.Dir(p.events)); err == nil {
		_ = dir.Sync()
		dir.Close()
	}
	return nil
}

// Discard removes what PrepareScrub wrote and Commit did not put in place.
func (p *StoredScrub) Discard() {
	for _, tmp := range []string{p.eventsTmp, p.metaTmp} {
		if tmp != "" {
			_ = os.Remove(tmp)
		}
	}
	p.eventsTmp, p.metaTmp = "", ""
}

// writeEvents writes events one per line to path, synced.
func writeEvents(path string, events []protocol.SessionEvent) error {
	out, err := os.OpenFile(path, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	encoder := json.NewEncoder(out)
	encoder.SetEscapeHTML(true)
	for _, ev := range events {
		if err := encoder.Encode(ev); err != nil {
			out.Close()
			return err
		}
	}
	if err := out.Sync(); err != nil {
		out.Close()
		return err
	}
	return out.Close()
}

// writeSynced writes data to path and syncs it.
func writeSynced(path string, data []byte) error {
	out, err := os.OpenFile(path, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	if _, err := out.Write(data); err != nil {
		out.Close()
		return err
	}
	if err := out.Sync(); err != nil {
		out.Close()
		return err
	}
	return out.Close()
}
