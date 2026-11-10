package updater

import (
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"ryoku-cli/internal/sys"
	i18n "ryoku-i18n"
)

type piece struct {
	Key     string    `json:"key"`
	Label   string    `json:"label"`
	State   stepState `json:"state"`
	Began   int64     `json:"began,omitempty"`
	Took    int64     `json:"took,omitempty"`
	Frac    float64   `json:"progress,omitempty"`
	Detail  string    `json:"detail,omitempty"`
	Note    string    `json:"note,omitempty"`
	Typical int64     `json:"typical,omitempty"`
}

// pieceView is a copy of the build-piece state safe for the console to retain.
type pieceView struct {
	Step    string
	Pieces  []piece
	Running int
	Typical int64
	First   bool
}

var pieceLabels = map[string]string{
	"shell":        i18n.T("ryoku-shell, the desktop daemon"),
	"wm":           i18n.T("compositor providers"),
	"livewall":     i18n.T("ryogami-live, the video wallpaper engine"),
	"ryogami":      i18n.T("ryogami, the wallpaper daemon"),
	"hub":          i18n.T("ryoku-hub, the settings backend"),
	"rashin":       i18n.T("rashind, the optional AI daemon"),
	"cli":          i18n.T("the ryoku command"),
	"blobs":        i18n.T("Ryoku.Blobs, the frame renderer"),
	"picker":       i18n.T("Ryoku.Ryogami, the wallpaper picker"),
	"hypr-plugins": i18n.T("Hyprland compositor plugins"),
	"modules":      i18n.T("shell and Hub modules"),
	"config":       i18n.T("configuration files"),
	"desktop":      i18n.T("restarting the desktop"),
}

type pieceHistoryEntry struct {
	Key   string `json:"key"`
	Label string `json:"label"`
	Took  int64  `json:"took"`
	Runs  int    `json:"runs"`
}

type pieceHistoryFile struct {
	Pieces  []pieceHistoryEntry `json:"pieces"`
	Updated string              `json:"updated"`
}

func pieceHistoryPath() string {
	return filepath.Join(sys.StateDir(), "deploy-pieces.json")
}

// History is a hint, never a prerequisite for an update. A missing, old or
// malformed file therefore reads exactly like a first build.
func loadPieceHistory() []pieceHistoryEntry {
	b, err := os.ReadFile(pieceHistoryPath())
	if err != nil {
		return nil
	}
	var h pieceHistoryFile
	if json.Unmarshal(b, &h) != nil {
		return nil
	}
	seen := make(map[string]bool, len(h.Pieces))
	out := make([]pieceHistoryEntry, 0, len(h.Pieces))
	for _, e := range h.Pieces {
		if e.Key == "" || e.Took < 0 || e.Runs < 0 || seen[e.Key] {
			continue
		}
		seen[e.Key] = true
		out = append(out, e)
	}
	return out
}

func writePieceHistory(entries []pieceHistoryEntry) {
	if entries == nil {
		entries = []pieceHistoryEntry{}
	}
	h := pieceHistoryFile{
		Pieces:  entries,
		Updated: time.Now().UTC().Format(time.RFC3339),
	}
	b, err := json.Marshal(h)
	if err != nil {
		return
	}
	path := pieceHistoryPath()
	if os.MkdirAll(filepath.Dir(path), 0o755) != nil {
		return
	}
	tmp := path + ".tmp"
	if os.WriteFile(tmp, b, 0o644) != nil {
		return
	}
	_ = os.Rename(tmp, path)
}

func pieceLabel(key, carried string) string {
	if label, ok := pieceLabels[key]; ok {
		return label
	}
	return carried
}

func (p *publisher) currentPieces() pieceView {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.currentPiecesLocked()
}

func (p *publisher) currentPiecesLocked() pieceView {
	v := pieceView{
		Step:    p.piecesStep,
		Pieces:  make([]piece, len(p.pieces)),
		Running: -1,
		Typical: p.piecesTypical,
		First:   p.firstBuild,
	}
	copy(v.Pieces, p.pieces)
	for i := range v.Pieces {
		if v.Pieces[i].State == stepRunning {
			v.Running = i
			break
		}
	}
	return v
}

func (p *publisher) resetPiecesLocked() {
	p.pieces = nil
	p.piecesStep = ""
	p.piecesTypical = 0
	p.piecesSettled = false
	p.firstBuild = false
	p.pieceFrac = 0
	p.pieceHistory = nil
	p.pieceOrder = nil
	p.pieceNotes = nil
}

func (p *publisher) runningStepLocked() string {
	for _, s := range p.steps {
		if s.State == stepRunning {
			return s.Key
		}
	}
	return ""
}

type pieceNotice struct {
	marker     bool
	changed    bool
	opened     bool
	first      bool
	hasSettled bool
	view       pieceView
	settled    piece
}

func (p *publisher) handlePieceMarkerLocked(line string) pieceNotice {
	if !strings.HasPrefix(line, "::piece:: ") {
		return pieceNotice{}
	}
	n := pieceNotice{marker: true}
	body := strings.TrimPrefix(line, "::piece:: ")
	switch {
	case strings.HasPrefix(body, "begin "):
		rest := strings.TrimPrefix(body, "begin ")
		space := strings.IndexByte(rest, ' ')
		if space <= 0 || space == len(rest)-1 {
			return n
		}
		key, label := rest[:space], strings.TrimSpace(rest[space+1:])
		if !validPieceKey(key) || label == "" {
			return n
		}
		return p.beginPieceLocked(key, label)
	case strings.HasPrefix(body, "end "):
		rest := strings.TrimPrefix(body, "end ")
		key, status, ok := strings.Cut(rest, " ")
		status = strings.TrimSpace(status)
		if !ok || status == "" || strings.ContainsAny(status, " \t") || !validPieceKey(key) {
			return n
		}
		var state stepState
		switch status {
		case string(stepDone):
			state = stepDone
		case string(stepSkipped):
			state = stepSkipped
		case string(stepFailed):
			state = stepFailed
		default:
			return n
		}
		if pc, ok := p.settleNamedPieceLocked(key, state); ok {
			n.changed = true
			n.hasSettled = true
			n.settled = pc
		}
		return n
	case body == "note" || strings.HasPrefix(body, "note "):
		text := strings.TrimSpace(strings.TrimPrefix(body, "note"))
		if text != "" && p.noteRunningPieceLocked(text) {
			n.changed = true
		}
		return n
	default:
		return n
	}
}

func validPieceKey(key string) bool {
	if key == "" {
		return false
	}
	for _, r := range key {
		if (r < 'a' || r > 'z') && (r < '0' || r > '9') && r != '-' {
			return false
		}
	}
	return true
}

func (p *publisher) beginPieceLocked(key, carried string) pieceNotice {
	n := pieceNotice{marker: true}
	step := p.runningStepLocked()
	if step == "" {
		return n
	}
	if p.piecesStep != "" && p.piecesStep != step {
		p.resetPiecesLocked()
	}
	if p.piecesStep == "" {
		p.piecesStep = step
		p.pieceHistory = loadPieceHistory()
		p.firstBuild = len(p.pieceHistory) == 0
		p.pieceNotes = make(map[string][]string)
		for _, h := range p.pieceHistory {
			label := pieceLabel(h.Key, h.Label)
			p.pieces = append(p.pieces, piece{
				Key: h.Key, Label: label, State: stepPending, Typical: h.Took,
			})
			p.piecesTypical += h.Took
		}
		n.opened, n.first = true, p.firstBuild
	}
	if pc, ok := p.settleRunningPieceLocked(stepDone); ok {
		n.hasSettled = true
		n.settled = pc
	}
	idx := p.pieceIndexLocked(key)
	if idx < 0 {
		p.pieces = append(p.pieces, piece{Key: key, State: stepPending})
		idx = len(p.pieces) - 1
	}
	pc := &p.pieces[idx]
	pc.Label = pieceLabel(key, carried)
	pc.State = stepRunning
	pc.Began = nowMs()
	pc.Took = 0
	pc.Frac = 0
	pc.Detail = ""
	pc.Note = ""
	delete(p.pieceNotes, key)
	if !containsString(p.pieceOrder, key) {
		p.pieceOrder = append(p.pieceOrder, key)
	}
	n.changed = true
	if n.opened {
		n.view = p.currentPiecesLocked()
	}
	return n
}

func (p *publisher) pieceIndexLocked(key string) int {
	for i := range p.pieces {
		if p.pieces[i].Key == key {
			return i
		}
	}
	return -1
}

func (p *publisher) runningPieceIndexLocked() int {
	for i := range p.pieces {
		if p.pieces[i].State == stepRunning {
			return i
		}
	}
	return -1
}

func (p *publisher) settleNamedPieceLocked(key string, state stepState) (piece, bool) {
	i := p.runningPieceIndexLocked()
	if i < 0 || p.pieces[i].Key != key {
		return piece{}, false
	}
	return p.settlePieceLocked(i, state), true
}

func (p *publisher) settleRunningPieceLocked(state stepState) (piece, bool) {
	i := p.runningPieceIndexLocked()
	if i < 0 {
		return piece{}, false
	}
	return p.settlePieceLocked(i, state), true
}

func (p *publisher) settlePieceLocked(i int, state stepState) piece {
	pc := &p.pieces[i]
	if pc.Began > 0 {
		pc.Took = max(int64(0), nowMs()-pc.Began)
	}
	pc.State = state
	return *pc
}

func (p *publisher) noteRunningPieceLocked(text string) bool {
	i := p.runningPieceIndexLocked()
	if i < 0 {
		return false
	}
	key := p.pieces[i].Key
	notes := p.pieceNotes[key]
	if containsString(notes, text) || len(notes) >= 2 {
		return false
	}
	notes = append(notes, text)
	p.pieceNotes[key] = notes
	p.pieces[i].Note = strings.Join(notes, " · ")
	return true
}

func containsString(items []string, item string) bool {
	for _, candidate := range items {
		if candidate == item {
			return true
		}
	}
	return false
}

func (p *publisher) observePieceOutputLocked(line string) {
	i := p.runningPieceIndexLocked()
	if i < 0 {
		return
	}
	pc := &p.pieces[i]
	pc.Detail = strings.TrimSpace(line)
	if m := fracRe.FindStringSubmatch(line); m != nil {
		x, xerr := parseFractionPart(m[1])
		y, yerr := parseFractionPart(m[2])
		if xerr == nil && yerr == nil && y > 0 && x <= y {
			pc.Frac = max(pc.Frac, float64(x)/float64(y))
		}
	}
}

func (p *publisher) settlePiecesForStepLocked(step string, state stepState) (piece, bool) {
	if p.piecesStep == "" || p.piecesStep != step || p.piecesSettled {
		return piece{}, false
	}
	p.piecesSettled = true
	pieceState := stepDone
	switch state {
	case stepFailed:
		pieceState = stepFailed
	case stepSkipped:
		pieceState = stepSkipped
	}
	settled, ok := p.settleRunningPieceLocked(pieceState)
	kept := p.pieces[:0]
	for _, pc := range p.pieces {
		if pc.State != stepPending {
			kept = append(kept, pc)
		}
	}
	p.pieces = kept
	p.updatePieceHistoryLocked()
	return settled, ok
}

func (p *publisher) updatePieceHistoryLocked() {
	old := make(map[string]pieceHistoryEntry, len(p.pieceHistory))
	for _, e := range p.pieceHistory {
		old[e.Key] = e
	}
	byKey := make(map[string]piece, len(p.pieces))
	for _, pc := range p.pieces {
		byKey[pc.Key] = pc
	}
	out := make([]pieceHistoryEntry, 0, len(p.pieceOrder)+len(p.pieceHistory))
	seen := make(map[string]bool, cap(out))
	for _, key := range p.pieceOrder {
		pc, exists := byKey[key]
		if !exists {
			continue
		}
		e, had := old[key]
		if pc.State == stepDone {
			if !had || e.Runs == 0 {
				e.Took = pc.Took
			} else {
				e.Took = int64(math.Round(0.6*float64(e.Took) + 0.4*float64(pc.Took)))
			}
			e.Key, e.Label, e.Runs = pc.Key, pc.Label, e.Runs+1
		} else if !had {
			continue
		}
		out = append(out, e)
		seen[key] = true
	}
	for _, e := range p.pieceHistory {
		if !seen[e.Key] {
			out = append(out, e)
		}
	}
	writePieceHistory(out)
	p.pieceHistory = out
}

func parseFractionPart(s string) (int, error) {
	var n int
	for _, r := range s {
		if r < '0' || r > '9' {
			return 0, os.ErrInvalid
		}
		n = n*10 + int(r-'0')
	}
	return n, nil
}

// pieceStepFraction estimates just the running step's share. Its remembered
// high-water mark prevents a new compiler's first line from moving the run
// backwards.
func (p *publisher) pieceStepFraction() float64 {
	if len(p.pieces) == 0 {
		return 0
	}
	var frac float64
	if p.piecesTypical > 0 {
		median := medianPieceTypical(p.pieces)
		var progressed float64
		for _, pc := range p.pieces {
			typical := pc.Typical
			if typical <= 0 {
				typical = median
			}
			if typical <= 0 {
				continue
			}
			switch pc.State {
			case stepDone, stepSkipped, stepFailed:
				progressed += float64(typical)
			case stepRunning:
				share := pc.Frac
				if pc.Began > 0 {
					elapsed := float64(max(int64(0), nowMs()-pc.Began)) / float64(typical)
					share = max(share, min(0.95, elapsed))
				}
				progressed += float64(typical) * share
			}
		}
		frac = progressed / float64(p.piecesTypical)
	} else {
		settled := 0
		runningShare := 0.0
		for _, pc := range p.pieces {
			switch pc.State {
			case stepDone, stepSkipped, stepFailed:
				settled++
			case stepRunning:
				runningShare = pc.Frac
				if runningShare == 0 {
					runningShare = 0.5
				}
			}
		}
		remainingGuess := max(2, 12-settled)
		frac = (float64(settled) + runningShare) / float64(settled+1+remainingGuess)
	}
	frac = max(0.02, min(0.97, frac))
	p.pieceFrac = min(0.97, max(p.pieceFrac, frac))
	return p.pieceFrac
}

func medianPieceTypical(pieces []piece) int64 {
	known := make([]int64, 0, len(pieces))
	for _, pc := range pieces {
		if pc.Typical > 0 {
			known = append(known, pc.Typical)
		}
	}
	if len(known) == 0 {
		return 0
	}
	sort.Slice(known, func(i, j int) bool { return known[i] < known[j] })
	mid := len(known) / 2
	if len(known)%2 == 1 {
		return known[mid]
	}
	return (known[mid-1] + known[mid]) / 2
}

// firstBuildExplainer is what a box hears the first time it builds from
// source: the console prints it under the open header, the log keeps it.
func firstBuildExplainer() string {
	return i18n.T("First build on this box: every piece compiles from scratch, which takes a few minutes. Later updates reuse the build caches and take a fraction of that.")
}
