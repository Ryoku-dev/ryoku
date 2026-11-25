package teams

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io/fs"
	"maps"
	"os"
	"path/filepath"
	"slices"
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
	mu              sync.Mutex
	dir             string
	teamSet         map[string]protocol.Team
	taskSet         map[string]protocol.TeamTask
	routineSet      map[string]protocol.Routine
	approvalSet     map[string]approvalRecord
	messageSet      map[string]protocol.TeamMessage
	memberHistories map[string]memberHistory
	// A failed owner-wait closure revokes execution but retains its durable
	// open source. Bound in-process credit at revocation until storage works;
	// restart still closes the retained unknown open wait before resuming.
	waitEnds map[string]time.Time
	// One writer fences every persisted execution/configuration consumer.
	writer stateWriter

	// spendMu guards spendData, spend.json in memory, loaded on first use. It
	// is apart from mu so counting tokens never waits on a state write.
	spendMu   sync.Mutex
	spendData spendFile
	spendErr  error

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
	Teams                []protocol.Team          `json:"teams"`
	Routines             []protocol.Routine       `json:"routines"`
	Approvals            []approvalRecord         `json:"approvals"`
	MemberHistoryVersion int                      `json:"memberHistoryVersion"`
	MemberHistories      map[string]memberHistory `json:"memberHistories"`
}

// openState tolerates a damaged task file without losing the rest of the
// board. Old team state is rewritten without removed external-engine fields.
func openState(dir string) (*state, error) {
	st := &state{
		dir:         dir,
		writer:      stateWriter{root: dir},
		teamSet:     make(map[string]protocol.Team),
		taskSet:     make(map[string]protocol.TeamTask),
		routineSet:  make(map[string]protocol.Routine),
		approvalSet: make(map[string]approvalRecord),
		messageSet:  make(map[string]protocol.TeamMessage),
	}
	if err := refusePendingTransactions(dir); err != nil {
		return nil, err
	}
	if err := paths.EnsureDir(filepath.Join(dir, "tasks")); err != nil {
		return nil, err
	}
	reportingChanged := false
	var file stateFile
	data, err := os.ReadFile(filepath.Join(dir, "state.json"))
	switch {
	case errors.Is(err, fs.ErrNotExist):
	case err != nil:
		return nil, err
	default:
		if err := json.Unmarshal(data, &file); err != nil {
			return nil, err
		}
		for _, t := range file.Teams {
			if t.Reporting == "" {
				if err := normaliseReporting(&t, nil); err != nil {
					return nil, err
				}
				reportingChanged = true
			} else if err := protocol.ValidateReporting(t); err != nil {
				return nil, err
			}
			for i := range t.Members {
				if t.Members[i].Revision == 0 {
					if file.MemberHistoryVersion != 0 {
						return nil, errors.New("zero canonical member revision in versioned history")
					}
					t.Members[i].Revision = 1
					reportingChanged = true
				}
			}
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
		if formerTask(&t, raw) {
			data, err := json.MarshalIndent(t, "", "  ")
			if err != nil {
				return nil, err
			}
			if err := st.writer.write(filepath.Join(dir, "tasks"), name, append(data, '\n')); err != nil {
				return nil, err
			}
		}
		st.taskSet[t.ID] = cloneTask(t)
	}
	forgot := st.forgetHarnesses()
	if err := st.loadMemberHistory(file.MemberHistoryVersion, file.MemberHistories); err != nil {
		return nil, err
	}
	if forgot || reportingChanged || len(data) > 0 && file.MemberHistoryVersion == 0 || bytes.Contains(data, []byte(`"harness"`)) || bytes.Contains(data, []byte(`"repeats"`)) {
		st.mu.Lock()
		err = st.saveLocked()
		st.mu.Unlock()
		if err != nil {
			return nil, err
		}
	}
	if err := st.migrateTaskGoals(); err != nil {
		return nil, err
	}
	if err := st.loadMessages(); err != nil {
		return nil, err
	}
	return st, nil
}

// formerApprovalRestore is the kind of a notice a former harness left in
// the Inbox; it asked nothing and no longer happens.
const formerApprovalRestore = "restore"

// forgetHarnesses clears what a former external harness left in the state:
// a member's model id only a harness took (one protocol.ValidEngineModel
// refuses), in teams and in proposed teams, and its notices. The members
// then follow the teams defaults. It reports whether it changed anything.
func (st *state) forgetHarnesses() bool {
	changed := false
	clearModels := func(members []protocol.Member) {
		for i, m := range members {
			if !protocol.ValidEngineModel(m.Model) {
				members[i].Model = ""
				changed = true
			}
		}
	}
	for _, t := range st.teamSet {
		clearModels(t.Members)
	}
	for id, rec := range st.approvalSet {
		if rec.Kind == formerApprovalRestore || rec.Status == protocol.ApprovalPending && cardOf(rec) == "stall" {
			delete(st.approvalSet, id)
			changed = true
			continue
		}
		if rec.Proposal != nil {
			clearModels(rec.Proposal.Team.Members)
		}
	}
	return changed
}

// formerTask clears what a former external harness left on a task read from
// raw: a model id only a harness took, and the harness a blocked task waited
// on (its former signIn field), which keeps it blocked with a reason the
// user can retry from. It reports whether it changed the task.
func formerTask(t *protocol.TeamTask, raw []byte) bool {
	var former struct {
		SignIn string `json:"signIn"`
	}
	_ = json.Unmarshal(raw, &former)
	changed := false
	if !protocol.ValidEngineModel(t.Model) {
		t.Model = ""
		changed = true
	}
	if bytes.Contains(raw, []byte(`"harness"`)) || bytes.Contains(raw, []byte(`"signIn"`)) {
		changed = true
	}
	if former.SignIn != "" && t.Status == protocol.TaskBlocked {
		t.Reason = i18n.T("It waited on a step Hyakki no longer needs. Retry it to run it on its model.")
	}
	return changed
}

// teams lists every team by name.
func (st *state) teams() []protocol.Team {
	st.mu.Lock()
	defer st.mu.Unlock()
	if st.writer.available() != nil {
		return nil
	}
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
	if st.writer.available() != nil {
		return protocol.Team{}, false
	}
	t, ok := st.teamSet[id]
	return cloneTeam(t), ok
}

// putTeam stores a team. Stats are derived on read, so they are not kept.
func (st *state) putTeam(t protocol.Team) error {
	t = cloneTeam(t)
	t.Stats = protocol.TeamStats{}
	st.mu.Lock()
	defer st.mu.Unlock()
	if err := st.writer.available(); err != nil {
		return err
	}
	prev, had := st.teamSet[t.ID]
	previousHistory := st.memberHistories[t.ID]
	history, err := nextMemberHistory(prev, &t, previousHistory)
	if err != nil {
		return err
	}
	if st.memberHistories == nil {
		st.memberHistories = make(map[string]memberHistory)
	}
	st.memberHistories[t.ID] = history
	st.teamSet[t.ID] = t
	if err := st.saveLocked(); err != nil {
		if previousHistory == nil {
			delete(st.memberHistories, t.ID)
		} else {
			st.memberHistories[t.ID] = previousHistory
		}
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
	if err := st.writer.available(); err != nil {
		return err
	}
	team, ok := st.teamSet[id]
	history := st.memberHistories[id]
	approvals := make(map[string]approvalRecord)
	for key, rec := range st.approvalSet {
		if rec.Team == id {
			approvals[key] = rec
			delete(st.approvalSet, key)
		}
	}
	delete(st.teamSet, id)
	delete(st.memberHistories, id)
	if err := st.saveLocked(); err != nil {
		if history != nil {
			st.memberHistories[id] = history
		}
		if ok {
			st.teamSet[id] = team
		}
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
	return st.removeMessagesLocked(func(m protocol.TeamMessage) bool { return m.Team == id })
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
	st.mu.Lock()
	defer st.mu.Unlock()
	if old, exists := st.taskSet[t.ID]; exists {
		if old.Started != nil && t.Started != nil && old.Started.Equal(*t.Started) {
			appendClosedWaits(&t, old.PermissionWaits)
		} else if old.Started != nil {
			if err := st.preserveAncestorWaitsLocked(old); err != nil {
				return err
			}
		}
	}
	return st.writeTaskLocked(t)
}

// writeTaskLocked shares the normal fail-closed writer with history folds.
func (st *state) writeTaskLocked(t protocol.TeamTask) error {
	if old, exists := st.taskSet[t.ID]; t.Ended != nil && (!exists || old.Ended == nil) {
		// Freeze the execution's complete earned union before it becomes a
		// terminal source. Descendant recovery, pruning and deletion must
		// never retroactively change an ended attempt's active budget.
		active := t
		active.Ended = nil
		appendClosedWaits(&t, st.ownerWaitSpansLocked(active, *t.Ended))
	}
	data, err := json.MarshalIndent(t, "", "  ")
	if err != nil {
		return err
	}
	if err := st.writer.write(filepath.Join(st.dir, "tasks"), t.ID+".json", append(data, '\n')); err != nil {
		if old, exists := st.taskSet[t.ID]; exists && taskHasOpenOwnerWait(old) {
			cutoff := t.Updated
			if cutoff.Before(old.Updated) {
				cutoff = old.Updated
			}
			if st.waitEnds == nil {
				st.waitEnds = make(map[string]time.Time)
			}
			if at, bounded := st.waitEnds[t.ID]; !bounded || cutoff.Before(at) {
				st.waitEnds[t.ID] = cutoff
			}
		}
		return err
	}
	st.taskSet[t.ID] = t
	if t.Ended != nil || !taskHasOpenOwnerWait(t) {
		delete(st.waitEnds, t.ID)
	}
	return nil
}

func (st *state) removeTask(id string) error {
	st.mu.Lock()
	defer st.mu.Unlock()
	if err := st.writer.available(); err != nil {
		return err
	}
	if _, ok := st.taskSet[id]; !ok {
		return st.removeMessagesLocked(func(m protocol.TeamMessage) bool { return m.Task == id })
	}
	if st.hasChildrenLocked(id) {
		return protocol.Errorf(protocol.CodeConflict, "This task still has child tasks.")
	}
	if err := st.preserveAncestorWaitsLocked(st.taskSet[id]); err != nil {
		return err
	}
	if err := os.Remove(st.taskPath(id)); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	delete(st.taskSet, id)
	return st.removeMessagesLocked(func(m protocol.TeamMessage) bool { return m.Task == id })
}

func (st *state) removeTaskConfined(id string, root *os.Root, eligible func(protocol.TeamTask) bool) (bool, error) {
	st.mu.Lock()
	defer st.mu.Unlock()
	if err := st.writer.available(); err != nil {
		return false, err
	}
	task, ok := st.taskSet[id]
	if !ok {
		return false, nil
	}
	if root == nil || !safeID(id) || !eligible(task) {
		return false, protocol.Errorf(protocol.CodeConflict, "%s", i18n.T("This task is still in use or inside the history window."))
	}
	if st.hasChildrenLocked(id) {
		return false, protocol.Errorf(protocol.CodeConflict, "This task still has child tasks.")
	}
	if err := st.preserveAncestorWaitsLocked(task); err != nil {
		return false, err
	}
	if err := root.Remove(id + ".json"); err != nil {
		return false, err
	}
	delete(st.taskSet, id)
	return true, st.removeMessagesLocked(func(m protocol.TeamMessage) bool { return m.Task == id })
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
			// A bounded approval list must not erase already-earned wait time.
			// The session check prevents an old decision crediting a retry.
			if task, exists := st.taskSet[old.Task]; exists && old.Session == task.Session && old.Decided != nil {
				until := *old.Decided
				if task.Ended != nil && task.Ended.Before(until) {
					until = *task.Ended
				}
				appendClosedWaits(&task, []protocol.TaskPermissionWait{{From: old.Created, Until: until}})
				if err := st.writeTaskLocked(task); err != nil {
					st.approvalSet = before
					return err
				}
			}
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
	if err := st.writer.available(); err != nil {
		return err
	}
	file := stateFile{
		Teams:                make([]protocol.Team, 0, len(st.teamSet)),
		Routines:             make([]protocol.Routine, 0, len(st.routineSet)),
		Approvals:            make([]approvalRecord, 0, len(st.approvalSet)),
		MemberHistoryVersion: 1,
		MemberHistories:      st.memberHistories,
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
	return st.writer.write(st.dir, "state.json", append(data, '\n'))
}

func (st *state) taskPath(id string) string {
	return filepath.Join(st.dir, "tasks", id+".json")
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

// cloneTeam copies what a caller could change through a shared slice, map or
// dream.
func cloneTeam(t protocol.Team) protocol.Team {
	t.Members = append([]protocol.Member(nil), t.Members...)
	for i := range t.Members {
		t.Members[i].Options = maps.Clone(t.Members[i].Options)
		member := &t.Members[i]
		if member.Tools != nil {
			tools := *member.Tools
			member.Tools = &tools
		}
		if member.Skills != nil {
			skills := slices.Clone(*member.Skills)
			member.Skills = &skills
		}
		if member.Runtime != nil {
			runtime := *member.Runtime
			runtime.Command = slices.Clone(runtime.Command)
			runtime.Env = maps.Clone(runtime.Env)
			runtime.SecretEnv = maps.Clone(runtime.SecretEnv)
			member.Runtime = &runtime
		}
		if member.Heartbeat != nil {
			heartbeat := *member.Heartbeat
			heartbeat.Modes = slices.Clone(heartbeat.Modes)
			member.Heartbeat = &heartbeat
		}
		if member.Budget != nil {
			budget := *member.Budget
			member.Budget = &budget
		}
	}
	t.Access.Writable = slices.Clone(t.Access.Writable)
	t.Access.Hidden = slices.Clone(t.Access.Hidden)
	t.SkillsOff = slices.Clone(t.SkillsOff)
	if t.Stats.Doing != nil {
		doing := make(map[string]string, len(t.Stats.Doing))
		for key, value := range t.Stats.Doing {
			doing[key] = value
		}
		t.Stats.Doing = doing
	}
	if t.Dream != nil {
		dream := cloneDream(*t.Dream)
		t.Dream = &dream
	}
	return t
}

func cloneTask(t protocol.TeamTask) protocol.TeamTask {
	t.Sessions = append([]string(nil), t.Sessions...)
	t.PermissionWaits = slices.Clone(t.PermissionWaits)
	t.Options = maps.Clone(t.Options)
	if t.Plan != nil {
		plan := *t.Plan
		plan.Steps = slices.Clone(plan.Steps)
		t.Plan = &plan
	}
	if t.Report != nil {
		report := *t.Report
		report.Files = append([]string(nil), report.Files...)
		t.Report = &report
	}
	return t
}

func cloneApproval(rec approvalRecord) approvalRecord {
	rec.Action = append(json.RawMessage(nil), rec.Action...)
	rec.Options = slices.Clone(rec.Options)
	rec.Steps = slices.Clone(rec.Steps)
	return rec
}
