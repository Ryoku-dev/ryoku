package session

import (
	"bytes"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"unicode/utf8"

	"ryoku-rashin/internal/paths"
	"ryoku-rashin/internal/protocol"
	"ryoku-rashin/internal/redact"
)

// ScrubTurn runs scrub over the events written since the last scrubbed turn,
// called once the turn's end is written: the text of each streamed message
// and terminal is merged first, so a secret split across deltas is found
// whole. When scrub changed anything, the log is rewritten in one rename,
// and the session's title and preview are scrubbed with it. It reports
// whether the log changed.
func (l *Log) ScrubTurn(scrub func(string) string) (bool, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	changed, err := l.scrubLocked(l.turnAt, l.turnSeq, -1, scrub)
	if err == nil {
		l.turnAt, l.turnSeq = l.size, l.info.LastSeq
	}
	return changed, err
}

// ScrubLast runs scrub over the newest event alone, such as the user's
// prompt the moment it is written, and rewrites the log as ScrubTurn does
// when it changed anything.
func (l *Log) ScrubLast(scrub func(string) string) (bool, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.info.LastSeq == 0 {
		return false, nil
	}
	return l.scrubLocked(l.lastAt, l.info.LastSeq-1, -1, scrub)
}

// ScrubAll runs scrub over every finished turn of the log, as ScrubTurn
// does over one. The turn after them is left to ScrubTurn, whole: scrubbed
// now, a secret it streams in pieces would lose its first piece and keep
// the rest. A log nothing writes to any more follows it with ScrubTurn.
func (l *Log) ScrubAll(scrub func(string) string) (bool, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.scrubLocked(0, 0, l.turnAt, scrub)
}

// scrubLocked scrubs the events from offset at up to end, or to the end of
// the log when end is negative, the first of them numbered after seq, and
// the title and preview. Every token the scrub finds is learned for the
// rest of the session, whole and as its body after the prefix.
func (l *Log) scrubLocked(at, seq, end int64, scrub func(string) string) (bool, error) {
	if l.closed {
		return false, os.ErrClosed
	}
	if end < 0 {
		end = l.size
	}
	scrub = l.learned.Wrap(scrub)
	changed, err := l.scrubEventsLocked(at, seq, end, scrub)
	if err != nil {
		return false, err
	}
	title, preview := scrub(l.info.Title), scrub(l.info.Preview)
	if title != l.info.Title || preview != l.info.Preview {
		l.info.Title, l.info.Preview = title, preview
		if err := l.persistMetaLocked(); err != nil {
			return changed, err
		}
	}
	return changed, nil
}

// scrubEventsLocked scrubs the events from offset at up to end: in place
// for a temporary log, whose offsets count events, and for a stored one by
// rewriting events.jsonl when scrub changed any.
func (l *Log) scrubEventsLocked(at, seq, end int64, scrub func(string) string) (bool, error) {
	if l.temporary {
		return scrubLearning(l.events[at:end], &l.learned, scrub), nil
	}
	path := filepath.Join(paths.SessionDir(l.info.ID), "events.jsonl")
	events, lengths, _, err := readEvents(path, at, seq+1)
	if err != nil {
		return false, err
	}
	for count, read := 0, at; count < len(events); count++ {
		if read += lengths[count]; read > end {
			events = events[:count]
			break
		}
	}
	changed := scrubLearning(events, &l.learned, scrub)
	if changed {
		if err := l.rewriteLocked(path, at, end, events); err != nil {
			return false, err
		}
	}
	return changed, nil
}

// scrubLearning runs scrub, which learner wraps, over events, and once more
// when the learner learned a token on the way, so a restatement that came
// before the token in these events is scrubbed as well.
func scrubLearning(events []protocol.SessionEvent, learner *redact.Learner, scrub func(string) string) bool {
	before := learner.Len()
	changed := ScrubEvents(events, scrub)
	if learner.Len() > before && ScrubEvents(events, scrub) {
		changed = true
	}
	return changed
}

// rewriteLocked replaces the log's events from byte offset at up to end
// with events and keeps what follows end as it is: the new file is written
// beside the log, synced, and renamed over it, so a crash leaves either log
// whole, and the log appends to the new file from then on.
func (l *Log) rewriteLocked(path string, at, end int64, events []protocol.SessionEvent) error {
	tmp := path + ".scrub"
	out, err := os.OpenFile(tmp, os.O_CREATE|os.O_TRUNC|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		return err
	}
	fail := func(err error) error {
		out.Close()
		os.Remove(tmp)
		return err
	}
	in, err := os.Open(path)
	if err != nil {
		return fail(err)
	}
	defer in.Close()
	if _, err := io.CopyN(out, in, at); err != nil {
		return fail(err)
	}
	size, last := at, at
	for _, ev := range events {
		line, err := json.Marshal(ev)
		if err != nil {
			return fail(err)
		}
		line = append(line, '\n')
		if _, err := out.Write(line); err != nil {
			return fail(err)
		}
		last = size
		size += int64(len(line))
	}
	// what follows end moves by as much as the rewritten events grew or shrank
	moved := size - end
	if _, err := in.Seek(end, io.SeekStart); err != nil {
		return fail(err)
	}
	if _, err := io.CopyN(out, in, l.size-end); err != nil {
		return fail(err)
	}
	if err := out.Sync(); err != nil {
		return fail(err)
	}
	if err := os.Rename(tmp, path); err != nil {
		return fail(err)
	}
	if dir, err := os.Open(filepath.Dir(path)); err == nil {
		_ = dir.Sync()
		dir.Close()
	}
	l.file.Close()
	l.file = out
	if l.lastAt >= end {
		last = l.lastAt + moved
	}
	if l.turnAt >= end {
		l.turnAt += moved
	}
	l.size, l.lastAt = l.size+moved, last
	return nil
}

// ScrubEvents runs scrub over every string the events carry and reports
// whether it changed any. The deltas of one streamed message, or of one
// terminal, are scrubbed as the text they add up to, and what changed goes
// back into the fewest deltas that cover it, so the replayed text reads as
// the scrubbed whole. The events change in place.
func ScrubEvents(events []protocol.SessionEvent, scrub func(string) string) bool {
	type piece struct {
		event int
		data  map[string]any
	}
	decoded := make([]any, len(events))
	dirty := make([]bool, len(events))
	streams := map[string][]piece{}
	var order []string
	for index, ev := range events {
		if len(ev.Data) == 0 {
			continue
		}
		decoder := json.NewDecoder(bytes.NewReader(ev.Data))
		decoder.UseNumber()
		var value any
		if decoder.Decode(&value) != nil {
			continue
		}
		decoded[index] = value
		object, _ := value.(map[string]any)
		if key, ok := streamKey(ev.Kind, object); ok {
			if _, seen := streams[key]; !seen {
				order = append(order, key)
			}
			streams[key] = append(streams[key], piece{index, object})
			delta := object["delta"]
			delete(object, "delta")
			decoded[index], dirty[index] = scrubValue(object, scrub)
			object["delta"] = delta
			continue
		}
		decoded[index], dirty[index] = scrubValue(value, scrub)
	}
	for _, key := range order {
		pieces := streams[key]
		deltas := make([]string, len(pieces))
		for index, p := range pieces {
			deltas[index], _ = p.data["delta"].(string)
		}
		scrubbed, changed := scrubStream(deltas, scrub)
		if !changed {
			continue
		}
		for index, p := range pieces {
			if p.data["delta"] != scrubbed[index] {
				p.data["delta"] = scrubbed[index]
				dirty[p.event] = true
			}
		}
	}
	changed := false
	for index := range events {
		if !dirty[index] {
			continue
		}
		raw, err := json.Marshal(decoded[index])
		if err != nil {
			continue
		}
		events[index].Data = raw
		changed = true
	}
	return changed
}

// streamKey names the stream a text, thought or terminal event's delta
// belongs to.
func streamKey(kind string, object map[string]any) (string, bool) {
	if object == nil {
		return "", false
	}
	if _, ok := object["delta"].(string); !ok {
		return "", false
	}
	switch kind {
	case protocol.EvText, protocol.EvThought:
		message, _ := object["message"].(string)
		return kind + "\x00" + message, true
	case protocol.EvTerminal:
		terminal, _ := object["terminal"].(string)
		return kind + "\x00" + terminal, true
	}
	return "", false
}

// scrubValue scrubs every string in a decoded JSON value, keys aside.
func scrubValue(value any, scrub func(string) string) (any, bool) {
	switch v := value.(type) {
	case string:
		scrubbed := scrub(v)
		return scrubbed, scrubbed != v
	case map[string]any:
		changed := false
		for key, item := range v {
			if scrubbed, ok := scrubValue(item, scrub); ok {
				v[key], changed = scrubbed, true
			}
		}
		return v, changed
	case []any:
		changed := false
		for index, item := range v {
			if scrubbed, ok := scrubValue(item, scrub); ok {
				v[index], changed = scrubbed, true
			}
		}
		return v, changed
	}
	return value, false
}

// scrubStream scrubs the text deltas add up to. The deltas wholly before the
// first change, and wholly after the last, stay as they were; the change
// goes into the first delta it touches and the others it spans are emptied.
func scrubStream(deltas []string, scrub func(string) string) ([]string, bool) {
	var merged bytes.Buffer
	for _, delta := range deltas {
		merged.WriteString(delta)
	}
	before := merged.String()
	after := scrub(before)
	if after == before {
		return deltas, false
	}
	prefix := 0
	for prefix < len(before) && prefix < len(after) && before[prefix] == after[prefix] {
		prefix++
	}
	for prefix > 0 && prefix < len(before) && !utf8.RuneStart(before[prefix]) {
		prefix--
	}
	suffix := 0
	for suffix < len(before)-prefix && suffix < len(after)-prefix && before[len(before)-1-suffix] == after[len(after)-1-suffix] {
		suffix++
	}
	for suffix > 0 && !utf8.RuneStart(before[len(before)-suffix]) {
		suffix--
	}
	end := len(before) - suffix
	first, last := -1, -1
	start := 0
	for index, delta := range deltas {
		stop := start + len(delta)
		if first < 0 && (stop > prefix || index == len(deltas)-1) {
			first = index
		}
		if start < end || index == first {
			last = index
		}
		start = stop
	}
	if last < first {
		last = first
	}
	starts := make([]int, len(deltas)+1)
	for index, delta := range deltas {
		starts[index+1] = starts[index] + len(delta)
	}
	out := append([]string(nil), deltas...)
	out[first] = before[starts[first]:prefix] + after[prefix:len(after)-suffix] + before[end:starts[last+1]]
	for index := first + 1; index <= last; index++ {
		out[index] = ""
	}
	return out, true
}
