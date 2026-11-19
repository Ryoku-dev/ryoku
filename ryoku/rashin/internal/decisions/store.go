package decisions

// Derived from oh-my-pi (MIT), packages/coding-agent/src/sharpshooter/{paths,queue}.ts@c7fff67

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"ryoku-rashin/internal/protocol"
)

// MaxLines is the most lines one decision file may hold.
const MaxLines = 120

// A bank folder holds the three area files, state.json and
// queue/<session>/<time>-<nonce>.json, one queued decision per file so
// extraction never races consolidation over a shared file.
const (
	stateName = "state.json"
	queueName = "queue"
)

// fileName is an area's file in its bank.
func fileName(area string) string { return area + ".md" }

func validArea(area string) bool {
	for _, known := range protocol.DecisionAreas {
		if area == known {
			return true
		}
	}
	return false
}

// note is one decision line of an area file and the topic heading it sits
// under.
type note struct {
	Section string
	Text    string
}

// parseNotes reads an area file: "## Topic" lines open a section, every
// other non-empty line is one decision, its list marker taken off. Notes
// come back grouped by section in the order each section first appears, the
// order renderNotes writes them in.
func parseNotes(content string) []note {
	var notes []note
	section := ""
	for _, line := range strings.Split(content, "\n") {
		line = strings.TrimSpace(line)
		switch {
		case line == "":
		case strings.HasPrefix(line, "#"):
			section = strings.TrimSpace(strings.TrimLeft(line, "#"))
		default:
			if text := bulletText(line); text != "" {
				notes = append(notes, note{Section: section, Text: text})
			}
		}
	}
	return group(notes)
}

func bulletText(line string) string {
	line = strings.TrimSpace(line)
	for _, marker := range []string{"- ", "* ", "+ "} {
		if strings.HasPrefix(line, marker) {
			return strings.TrimSpace(line[len(marker):])
		}
	}
	if line == "-" || line == "*" {
		return ""
	}
	return line
}

func group(notes []note) []note {
	var order []string
	bySection := map[string][]note{}
	for _, n := range notes {
		if _, ok := bySection[n.Section]; !ok {
			order = append(order, n.Section)
		}
		bySection[n.Section] = append(bySection[n.Section], n)
	}
	out := make([]note, 0, len(notes))
	for _, section := range order {
		out = append(out, bySection[section]...)
	}
	return out
}

// renderNotes writes an area file: a "## Topic" heading per section, one
// "- " line per decision, a blank line between sections.
func renderNotes(notes []note) string {
	var b strings.Builder
	notes = group(notes)
	for i, n := range notes {
		if i == 0 || n.Section != notes[i-1].Section {
			if i > 0 {
				b.WriteString("\n")
			}
			if n.Section != "" {
				b.WriteString("## " + n.Section + "\n")
			}
		}
		b.WriteString("- " + n.Text + "\n")
	}
	return b.String()
}

// lineCount counts a file's lines, a final newline closing the last one.
func lineCount(content string) int {
	content = strings.TrimRight(content, "\n")
	if content == "" {
		return 0
	}
	return strings.Count(content, "\n") + 1
}

func (s *Service) readArea(id, area string) string {
	data, err := os.ReadFile(filepath.Join(s.bankDir(id), fileName(area)))
	if err != nil {
		return ""
	}
	return string(data)
}

// writeArea replaces an area file whole; an empty area removes the file.
func (s *Service) writeArea(id, area, content string) error {
	path := filepath.Join(s.bankDir(id), fileName(area))
	if strings.TrimSpace(content) == "" {
		if err := os.Remove(path); err != nil && !errors.Is(err, fs.ErrNotExist) {
			return err
		}
		return nil
	}
	return writeAtomic(path, []byte(content))
}

func writeAtomic(path string, data []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	temp, err := os.CreateTemp(filepath.Dir(path), "."+filepath.Base(path)+".*.tmp")
	if err != nil {
		return err
	}
	name := temp.Name()
	if _, err := temp.Write(data); err != nil {
		temp.Close()
		os.Remove(name)
		return err
	}
	if err := temp.Close(); err != nil {
		os.Remove(name)
		return err
	}
	if err := os.Rename(name, path); err != nil {
		os.Remove(name)
		return err
	}
	return nil
}

// state is a bank's bookkeeping. Project names the folder the bank serves,
// so a listing needs no reverse of the hash.
type state struct {
	V            int        `json:"v"`
	Project      string     `json:"project"`
	Consolidated *time.Time `json:"consolidated,omitempty"`
	Model        string     `json:"model,omitempty"`
	Error        *failure   `json:"error,omitempty"`
}

// failure is the last consolidation that failed and how many failed in a
// row since the last one that worked; the retry waits longer each time.
type failure struct {
	At       time.Time `json:"at"`
	Message  string    `json:"message"`
	Failures int       `json:"failures"`
}

func (s *Service) readState(id string) state {
	var st state
	data, err := os.ReadFile(filepath.Join(s.bankDir(id), stateName))
	if err != nil || json.Unmarshal(data, &st) != nil || st.V != 1 {
		return state{V: 1}
	}
	return st
}

func (s *Service) writeState(id string, st state) error {
	st.V = 1
	data, err := json.MarshalIndent(st, "", "\t")
	if err != nil {
		return err
	}
	return writeAtomic(filepath.Join(s.bankDir(id), stateName), append(data, '\n'))
}

// delta is one queued decision as extraction admitted it.
type delta struct {
	V        int       `json:"v"`
	Area     string    `json:"area"`
	Rule     string    `json:"rule"`
	Quote    string    `json:"quote"`
	Friction string    `json:"friction"`
	Session  string    `json:"session"`
	At       time.Time `json:"at"`
}

// queued is a delta and the file that holds it, removed once consolidation
// has applied it.
type queued struct {
	delta
	file string
}

func (s *Service) queueDir(id string) string { return filepath.Join(s.bankDir(id), queueName) }

// enqueue writes one delta. The file name starts with the time in base 36,
// padded, so names sort by time; the nonce keeps two in one millisecond
// apart.
func (s *Service) enqueue(id string, d delta) error {
	nonce := make([]byte, 2)
	if _, err := rand.Read(nonce); err != nil {
		return err
	}
	stamp := strconv.FormatInt(d.At.UnixMilli(), 36)
	if len(stamp) < 10 {
		stamp = strings.Repeat("0", 10-len(stamp)) + stamp
	}
	data, err := json.Marshal(d)
	if err != nil {
		return err
	}
	return writeAtomic(filepath.Join(s.queueDir(id), sessionDir(d.Session), stamp+"-"+hex.EncodeToString(nonce)+".json"), data)
}

// sessionDir makes a session id safe as a folder name.
func sessionDir(session string) string {
	var b strings.Builder
	for _, r := range session {
		if r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '_' || r == '-' {
			b.WriteRune(r)
		} else {
			b.WriteByte('-')
		}
	}
	if out := strings.Trim(b.String(), "-"); out != "" {
		return out
	}
	return "session"
}

// listQueue reads every queued delta, sessions ordered by their oldest
// delta and deltas oldest first within a session. Files that do not read
// as a delta are skipped and left in place, so one torn write never blocks
// the queue.
func (s *Service) listQueue(id string) []queued {
	root := s.queueDir(id)
	sessions, err := os.ReadDir(root)
	if err != nil {
		return nil
	}
	var groups [][]queued
	for _, session := range sessions {
		if !session.IsDir() {
			continue
		}
		dir := filepath.Join(root, session.Name())
		entries, err := os.ReadDir(dir)
		if err != nil {
			continue
		}
		var group []queued
		for _, entry := range entries {
			if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
				continue
			}
			file := filepath.Join(dir, entry.Name())
			data, err := os.ReadFile(file)
			if err != nil {
				continue
			}
			var d delta
			if json.Unmarshal(data, &d) != nil || d.V != 1 || strings.TrimSpace(d.Rule) == "" || !validArea(d.Area) {
				continue
			}
			group = append(group, queued{delta: d, file: file})
		}
		if len(group) == 0 {
			continue
		}
		sort.SliceStable(group, func(i, j int) bool { return group[i].At.Before(group[j].At) })
		groups = append(groups, group)
	}
	sort.SliceStable(groups, func(i, j int) bool { return groups[i][0].At.Before(groups[j][0].At) })
	var out []queued
	for _, group := range groups {
		out = append(out, group...)
	}
	return out
}

// queueDepth counts queued deltas without reading them.
func (s *Service) queueDepth(id string) int {
	root := s.queueDir(id)
	sessions, err := os.ReadDir(root)
	if err != nil {
		return 0
	}
	count := 0
	for _, session := range sessions {
		if !session.IsDir() {
			continue
		}
		entries, err := os.ReadDir(filepath.Join(root, session.Name()))
		if err != nil {
			continue
		}
		for _, entry := range entries {
			if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".json") {
				count++
			}
		}
	}
	return count
}

// consume removes applied deltas and the session folders they emptied. A
// file that will not go is delivered again next time, which a rewrite that
// already holds its decision absorbs.
func consume(items []queued) {
	dirs := map[string]bool{}
	for _, item := range items {
		os.Remove(item.file)
		dirs[filepath.Dir(item.file)] = true
	}
	for dir := range dirs {
		if entries, err := os.ReadDir(dir); err == nil && len(entries) == 0 {
			os.Remove(dir)
		}
	}
}
