package teams

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	i18n "ryoku-i18n"

	"ryoku-rashin/internal/paths"
	"ryoku-rashin/internal/protocol"
)

// keepDecided is how many decided approvals state.json keeps; pending ones
// are always kept, so a long history never hides what still waits.
const keepDecided = 500

// state is the teams' persisted state: teams, routines and approval records
// in state.json, and one file per task under tasks/ because reports can be
// long. Every mutation reaches the disk before it returns.
type state struct {
	mu          sync.Mutex
	dir         string
	teamSet     map[string]protocol.Team
	taskSet     map[string]protocol.TeamTask
	routineSet  map[string]protocol.Routine
	approvalSet map[string]approvalRecord

	// spendMu guards spendData, spend.json in memory, loaded on first use. It
	// is apart from mu so counting tokens never waits on a state write.
	spendMu   sync.Mutex
	spendData spendFile

	// expiry is the one timer to the next pending approval's expiry. It lives
	// with the approvals it watches: Service has no field of its own for it.
	expiry *time.Timer
}

// approvalRecord is an approval with the exact operation it would run.
type approvalRecord struct {
	protocol.Approval
	// Action is the exact operation an approval would run, stored daemon-side only
	Action json.RawMessage `json:"action,omitempty"`
}

// stateFile is the shape of state.json; rashin routines wake reads its
// teams and routines without starting the daemon.
type stateFile struct {
	Teams     []protocol.Team    `json:"teams"`
	Routines  []protocol.Routine `json:"routines"`
	Approvals []approvalRecord   `json:"approvals"`
}

// openState loads the state under dir, creating the directory when it is
// missing. A task file that cannot be read is skipped, so one damaged report
// does not take the rest of the board with it.
func openState(dir string) (*state, error) {
	st := &state{
		dir:         dir,
		teamSet:     make(map[string]protocol.Team),
		taskSet:     make(map[string]protocol.TeamTask),
		routineSet:  make(map[string]protocol.Routine),
		approvalSet: make(map[string]approvalRecord),
	}
	if err := paths.EnsureDir(filepath.Join(dir, "tasks")); err != nil {
		return nil, err
	}
	data, err := os.ReadFile(filepath.Join(dir, "state.json"))
	switch {
	case errors.Is(err, fs.ErrNotExist):
	case err != nil:
		return nil, err
	default:
		var file stateFile
		if err := json.Unmarshal(data, &file); err != nil {
			return nil, err
		}
		for _, t := range file.Teams {
			st.teamSet[t.ID] = t
		}
		for _, r := range file.Routines {
			st.routineSet[r.ID] = r
		}
		for _, a := range file.Approvals {
			st.approvalSet[a.ID] = a
		}
	}
	entries, err := os.ReadDir(filepath.Join(dir, "tasks"))
	if err != nil {
		return nil, err
	}
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !strings.HasSuffix(name, ".json") {
			continue
		}
		raw, err := os.ReadFile(filepath.Join(dir, "tasks", name))
		if err != nil {
			continue
		}
		var t protocol.TeamTask
		if json.Unmarshal(raw, &t) != nil || t.ID+".json" != name {
			continue
		}
		st.taskSet[t.ID] = t
	}
	return st, nil
}

// teams lists every team by name.
func (st *state) teams() []protocol.Team {
	st.mu.Lock()
	defer st.mu.Unlock()
	out := make([]protocol.Team, 0, len(st.teamSet))
	for _, t := range st.teamSet {
		out = append(out, cloneTeam(t))
	}
	sort.Slice(out, func(i, j int) bool {
		a, b := strings.ToLower(out[i].Name), strings.ToLower(out[j].Name)
		if a != b {
			return a < b
		}
		return out[i].ID < out[j].ID
	})
	return out
}

func (st *state) team(id string) (protocol.Team, bool) {
	st.mu.Lock()
	defer st.mu.Unlock()
	t, ok := st.teamSet[id]
	return cloneTeam(t), ok
}

// putTeam stores a team. Stats are derived on read, so they are not kept.
func (st *state) putTeam(t protocol.Team) error {
	t = cloneTeam(t)
	t.Stats = protocol.TeamStats{}
	st.mu.Lock()
	defer st.mu.Unlock()
	prev, had := st.teamSet[t.ID]
	st.teamSet[t.ID] = t
	if err := st.saveLocked(); err != nil {
		if had {
			st.teamSet[t.ID] = prev
		} else {
			delete(st.teamSet, t.ID)
		}
		return err
	}
	return nil
}

// removeTeam removes a team with its tasks and approvals; its routines,
// grants and running sessions belong to the slices that drop them.
func (st *state) removeTeam(id string) error {
	st.mu.Lock()
	defer st.mu.Unlock()
	team, ok := st.teamSet[id]
	if !ok {
		return nil
	}
	approvals := make(map[string]approvalRecord)
	for key, rec := range st.approvalSet {
		if rec.Team == id {
			approvals[key] = rec
			delete(st.approvalSet, key)
		}
	}
	delete(st.teamSet, id)
	if err := st.saveLocked(); err != nil {
		st.teamSet[id] = team
		for key, rec := range approvals {
			st.approvalSet[key] = rec
		}
		return err
	}
	for key, t := range st.taskSet {
		if t.Team != id {
			continue
		}
		if err := os.Remove(st.taskPath(key)); err != nil && !errors.Is(err, fs.ErrNotExist) {
			return err
		}
		delete(st.taskSet, key)
	}
	return nil
}

// tasks lists the tasks keep accepts, newest first; a nil keep takes all.
func (st *state) tasks(keep func(protocol.TeamTask) bool) []protocol.TeamTask {
	st.mu.Lock()
	defer st.mu.Unlock()
	var out []protocol.TeamTask
	for _, t := range st.taskSet {
		if keep == nil || keep(t) {
			out = append(out, cloneTask(t))
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if !out[i].Created.Equal(out[j].Created) {
			return out[i].Created.After(out[j].Created)
		}
		return out[i].ID > out[j].ID
	})
	return out
}

func (st *state) task(id string) (protocol.TeamTask, bool) {
	st.mu.Lock()
	defer st.mu.Unlock()
	t, ok := st.taskSet[id]
	return cloneTask(t), ok
}

func (st *state) putTask(t protocol.TeamTask) error {
	if !safeID(t.ID) {
		return protocol.Errorf(protocol.CodeBadRequest, "%s", i18n.T("The task id is not valid."))
	}
	t = cloneTask(t)
	data, err := json.MarshalIndent(t, "", "  ")
	if err != nil {
		return err
	}
	st.mu.Lock()
	defer st.mu.Unlock()
	if err := writeStateFile(filepath.Join(st.dir, "tasks"), t.ID+".json", append(data, '\n')); err != nil {
		return err
	}
	st.taskSet[t.ID] = t
	return nil
}

func (st *state) removeTask(id string) error {
	st.mu.Lock()
	defer st.mu.Unlock()
	if _, ok := st.taskSet[id]; !ok {
		return nil
	}
	if err := os.Remove(st.taskPath(id)); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	delete(st.taskSet, id)
	return nil
}

// routines lists every routine by team, then title.
func (st *state) routines() []protocol.Routine {
	st.mu.Lock()
	defer st.mu.Unlock()
	out := make([]protocol.Routine, 0, len(st.routineSet))
	for _, r := range st.routineSet {
		out = append(out, r)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Team != out[j].Team {
			return out[i].Team < out[j].Team
		}
		a, b := strings.ToLower(out[i].Title), strings.ToLower(out[j].Title)
		if a != b {
			return a < b
		}
		return out[i].ID < out[j].ID
	})
	return out
}

func (st *state) routine(id string) (protocol.Routine, bool) {
	st.mu.Lock()
	defer st.mu.Unlock()
	r, ok := st.routineSet[id]
	return r, ok
}

func (st *state) putRoutine(r protocol.Routine) error {
	st.mu.Lock()
	defer st.mu.Unlock()
	prev, had := st.routineSet[r.ID]
	st.routineSet[r.ID] = r
	if err := st.saveLocked(); err != nil {
		if had {
			st.routineSet[r.ID] = prev
		} else {
			delete(st.routineSet, r.ID)
		}
		return err
	}
	return nil
}

func (st *state) removeRoutine(id string) error {
	st.mu.Lock()
	defer st.mu.Unlock()
	prev, had := st.routineSet[id]
	if !had {
		return nil
	}
	delete(st.routineSet, id)
	if err := st.saveLocked(); err != nil {
		st.routineSet[id] = prev
		return err
	}
	return nil
}

// approvals lists the records keep accepts, newest first; a nil keep takes
// all.
func (st *state) approvals(keep func(approvalRecord) bool) []approvalRecord {
	st.mu.Lock()
	defer st.mu.Unlock()
	var out []approvalRecord
	for _, rec := range st.approvalSet {
		if keep == nil || keep(rec) {
			out = append(out, cloneApproval(rec))
		}
	}
	sortApprovals(out)
	return out
}

func (st *state) approval(id string) (approvalRecord, bool) {
	st.mu.Lock()
	defer st.mu.Unlock()
	rec, ok := st.approvalSet[id]
	return cloneApproval(rec), ok
}

// putApproval stores a record and drops the oldest decided ones beyond
// keepDecided.
func (st *state) putApproval(rec approvalRecord) error {
	rec = cloneApproval(rec)
	st.mu.Lock()
	defer st.mu.Unlock()
	before := make(map[string]approvalRecord, len(st.approvalSet))
	for key, value := range st.approvalSet {
		before[key] = value
	}
	st.approvalSet[rec.ID] = rec
	var decided []approvalRecord
	for _, value := range st.approvalSet {
		if value.Status != protocol.ApprovalPending {
			decided = append(decided, value)
		}
	}
	if len(decided) > keepDecided {
		sort.Slice(decided, func(i, j int) bool { return decidedAt(decided[i]).After(decidedAt(decided[j])) })
		for _, old := range decided[keepDecided:] {
			delete(st.approvalSet, old.ID)
		}
	}
	if err := st.saveLocked(); err != nil {
		st.approvalSet = before
		return err
	}
	return nil
}

// armExpiry replaces the expiry timer with one that runs fire at at.
func (st *state) armExpiry(at time.Time, now time.Time, fire func()) {
	st.mu.Lock()
	defer st.mu.Unlock()
	if st.expiry != nil {
		st.expiry.Stop()
	}
	st.expiry = time.AfterFunc(max(at.Sub(now), 0), fire)
}

// disarmExpiry stops the expiry timer while nothing is pending.
func (st *state) disarmExpiry() {
	st.mu.Lock()
	defer st.mu.Unlock()
	if st.expiry != nil {
		st.expiry.Stop()
		st.expiry = nil
	}
}

func (st *state) saveLocked() error {
	file := stateFile{
		Teams:     make([]protocol.Team, 0, len(st.teamSet)),
		Routines:  make([]protocol.Routine, 0, len(st.routineSet)),
		Approvals: make([]approvalRecord, 0, len(st.approvalSet)),
	}
	for _, t := range st.teamSet {
		file.Teams = append(file.Teams, t)
	}
	for _, r := range st.routineSet {
		file.Routines = append(file.Routines, r)
	}
	for _, rec := range st.approvalSet {
		file.Approvals = append(file.Approvals, rec)
	}
	sort.Slice(file.Teams, func(i, j int) bool { return file.Teams[i].ID < file.Teams[j].ID })
	sort.Slice(file.Routines, func(i, j int) bool { return file.Routines[i].ID < file.Routines[j].ID })
	sortApprovals(file.Approvals)
	data, err := json.MarshalIndent(file, "", "  ")
	if err != nil {
		return err
	}
	return writeStateFile(st.dir, "state.json", append(data, '\n'))
}

func (st *state) taskPath(id string) string {
	return filepath.Join(st.dir, "tasks", id+".json")
}

// writeStateFile replaces dir/name so a crash leaves the old file or the new
// one, never half of either.
func writeStateFile(dir, name string, data []byte) error {
	if err := paths.EnsureDir(dir); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(dir, "."+name+"-*.tmp")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	ok := false
	defer func() {
		_ = tmp.Close()
		if !ok {
			_ = os.Remove(tmpName)
		}
	}()
	if err := tmp.Chmod(0o600); err != nil {
		return err
	}
	if _, err := tmp.Write(data); err != nil {
		return err
	}
	if err := tmp.Sync(); err != nil {
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Rename(tmpName, filepath.Join(dir, name)); err != nil {
		return err
	}
	directory, err := os.Open(dir)
	if err != nil {
		return err
	}
	syncErr := directory.Sync()
	closeErr := directory.Close()
	if syncErr != nil {
		return syncErr
	}
	if closeErr != nil {
		return closeErr
	}
	ok = true
	return nil
}

// newID makes an id such as "task-6f1c2a9e".
func newID(prefix string) string {
	var raw [4]byte
	if _, err := rand.Read(raw[:]); err != nil {
		now := time.Now().UnixNano()
		for i := range raw {
			raw[i] = byte(now >> (i * 8))
		}
	}
	return prefix + "-" + hex.EncodeToString(raw[:])
}

// safeID keeps a task id usable as a file name inside tasks/.
func safeID(id string) bool {
	return id != "" && !strings.HasPrefix(id, ".") && !strings.ContainsAny(id, `/\`)
}

func sortApprovals(list []approvalRecord) {
	sort.Slice(list, func(i, j int) bool {
		if !list[i].Created.Equal(list[j].Created) {
			return list[i].Created.After(list[j].Created)
		}
		return list[i].ID > list[j].ID
	})
}

func decidedAt(rec approvalRecord) time.Time {
	if rec.Decided != nil {
		return *rec.Decided
	}
	return rec.Created
}

// cloneTeam copies what a caller could change through a shared slice or map.
func cloneTeam(t protocol.Team) protocol.Team {
	t.Members = append([]protocol.Member(nil), t.Members...)
	if t.Stats.Doing != nil {
		doing := make(map[string]string, len(t.Stats.Doing))
		for key, value := range t.Stats.Doing {
			doing[key] = value
		}
		t.Stats.Doing = doing
	}
	return t
}

func cloneTask(t protocol.TeamTask) protocol.TeamTask {
	t.Sessions = append([]string(nil), t.Sessions...)
	if t.Report != nil {
		report := *t.Report
		report.Files = append([]string(nil), report.Files...)
		t.Report = &report
	}
	return t
}

func cloneApproval(rec approvalRecord) approvalRecord {
	rec.Action = append(json.RawMessage(nil), rec.Action...)
	return rec
}
