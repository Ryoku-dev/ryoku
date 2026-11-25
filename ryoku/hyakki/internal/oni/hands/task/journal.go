package task

import (
	"bytes"
	"cmp"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"time"

	i18n "ryoku-i18n"

	"ryoku-rashin/internal/paths"
	"ryoku-rashin/internal/protocol"
)

// Protocol error reasons.
const (
	// ReasonDamaged is a journal file that cannot be read. It is left as it
	// is until Reset.
	ReasonDamaged = "hands_journal_damaged"
	// ReasonFull is a task refused because QueueMax already wait.
	ReasonFull = "hands_queue_full"
	// ReasonBusy is a start, or a forget, refused while a task runs.
	ReasonBusy = "hands_busy"
	// ReasonNotStartable is a start of a task that is not the next to wait
	// or an interrupted one: an ended task never starts again.
	ReasonNotStartable = "hands_not_startable"
)

// MaxStoreBytes is the largest journal file read. The spec's rows of tasks
// and projects at their bounds still fit.
const MaxStoreBytes = 1 << 20

const storeVersion = 1

func damaged() error {
	return &protocol.Error{
		Code:    protocol.CodeConflict,
		Reason:  ReasonDamaged,
		Message: i18n.T("The Hands task journal could not be read. Reset it to start again; the old file is kept beside the new one."),
	}
}

func full() error {
	return &protocol.Error{
		Code:    protocol.CodeConflict,
		Reason:  ReasonFull,
		Message: i18n.T("Hands already has a task waiting. Stop it or let it finish first."),
	}
}

func busy() error {
	return &protocol.Error{
		Code:    protocol.CodeConflict,
		Reason:  ReasonBusy,
		Message: i18n.T("Hands is working on a task. Stop it first."),
	}
}

func notStartable() error {
	return &protocol.Error{
		Code:    protocol.CodeConflict,
		Reason:  ReasonNotStartable,
		Message: i18n.T("That task cannot start. Start the next waiting task, or one a restart interrupted."),
	}
}

func badRequest(message string) error {
	return protocol.Errorf(protocol.CodeBadRequest, "%s", message)
}

func notFound() error {
	return protocol.Errorf(protocol.CodeNotFound, "%s", i18n.T("That Hands task is not there any more."))
}

// DefaultPath is the journal file in Hyakki's state directory.
func DefaultPath() string { return filepath.Join(paths.StateDir(), "oni-hands-tasks.json") }

// Journal keeps the tasks and the kept projects in one JSON file, written
// whole through a synced temporary file and renamed into place, mode 0600
// in a 0700 directory. With no task and no project there is no file. A file
// that cannot be read is never overwritten: every call reports it damaged
// until Reset moves it aside. Every write keeps at most Rows tasks and Rows
// projects, the oldest ended task and the least recently used project out
// first.
type Journal struct {
	mu   sync.Mutex
	path string
	c    Config
}

// Open opens the journal at path with the spec's numbers; an empty path is
// DefaultPath.
func Open(path string) *Journal { return OpenWith(path, Config{}) }

// OpenWith opens the journal at path with c held to the spec: a number
// past the spec's, or none, is the spec's.
func OpenWith(path string, c Config) *Journal {
	if path == "" {
		path = DefaultPath()
	}
	return &Journal{path: path, c: normal(c)}
}

// Path is the journal's file.
func (j *Journal) Path() string { return j.path }

// DamagedPath is where Reset moves a journal file.
func (j *Journal) DamagedPath() string { return j.path + ".damaged" }

// Enqueue keeps t, queued at now, with its ID, scope, app id and origin
// checked; its state, step, end and reason must be unset. With nothing
// running and nothing waiting it starts at once, by the claim that made
// it, and comes back Working. Otherwise it waits, UpNext, and only a
// button starts it; at QueueMax waiting it is refused.
func (j *Journal) Enqueue(t Task, now time.Time) (Task, error) {
	if t.State != "" && t.State != UpNext || t.Step != 0 || !t.Ended.IsZero() || t.Reason != "" {
		return Task{}, badRequest(i18n.T("A new Hands task starts with no state, step or end."))
	}
	t.State, t.At = UpNext, stamp(now)
	if err := checkTask(t, j.c); err != nil {
		return Task{}, err
	}
	j.mu.Lock()
	defer j.mu.Unlock()
	f, err := j.read()
	if err != nil {
		return Task{}, err
	}
	return j.enqueue(&f, t)
}

// enqueue adds t to f and writes it.
func (j *Journal) enqueue(f *file, t Task) (Task, error) {
	if slices.ContainsFunc(f.Tasks, func(x Task) bool { return x.ID == t.ID }) {
		return Task{}, badRequest(i18n.T("That Hands task is already in the journal."))
	}
	waiting := 0
	running := false
	for _, x := range f.Tasks {
		waiting += b2i(x.State == UpNext)
		running = running || x.State.Active()
	}
	switch {
	case !running && waiting == 0:
		t.State = Working
	case waiting >= j.c.QueueMax:
		return Task{}, full()
	}
	f.Tasks = append(f.Tasks, t)
	return t, j.write(f)
}

// Start starts the task with id at the person's button. By must be
// ByButton: a Press, wake, open mic, the CLI and a harness start nothing.
// Nothing may be running. The task must be the first waiting one, or one
// a restart interrupted; an interrupted task keeps its step count, and
// its goal is the one the person restates. A task that ended in any other
// way, failed included, never starts again.
func (j *Journal) Start(id, by string, now time.Time) (Task, error) {
	if by != ByButton {
		return Task{}, protocol.Errorf(protocol.CodeForbidden, "%s", i18n.T("Only the Start button starts a Hands task."))
	}
	j.mu.Lock()
	defer j.mu.Unlock()
	f, err := j.read()
	if err != nil {
		return Task{}, err
	}
	i := slices.IndexFunc(f.Tasks, func(x Task) bool { return x.ID == id })
	if i < 0 {
		return Task{}, notFound()
	}
	if slices.ContainsFunc(f.Tasks, func(x Task) bool { return x.State.Active() }) {
		return Task{}, busy()
	}
	t := &f.Tasks[i]
	switch t.State {
	case UpNext:
		if first := slices.IndexFunc(f.Tasks, func(x Task) bool { return x.State == UpNext }); first != i {
			return Task{}, notStartable()
		}
	case Interrupted:
		t.Ended, t.Reason = time.Time{}, ""
	default:
		return Task{}, notStartable()
	}
	t.State = Working
	return *t, j.write(&f)
}

// Update moves the running task with id to st, at step, for reason: to
// another running state, or to Done, Failed or Stopped, which end it at
// now. A waiting task may only be Stopped. The step never goes back and
// never passes Turns; the reason is one of the package's reasons or none.
// Only Recover interrupts a task.
func (j *Journal) Update(id string, st State, step int, reason string, now time.Time) (Task, error) {
	if !st.Active() && st != Done && st != Failed && st != Stopped {
		return Task{}, badRequest(i18n.T("A Hands task cannot move to that state."))
	}
	if !validReason(reason) {
		return Task{}, badRequest(i18n.T("That is not a Hands reason."))
	}
	j.mu.Lock()
	defer j.mu.Unlock()
	f, err := j.read()
	if err != nil {
		return Task{}, err
	}
	i := slices.IndexFunc(f.Tasks, func(x Task) bool { return x.ID == id })
	if i < 0 {
		return Task{}, notFound()
	}
	t := &f.Tasks[i]
	switch {
	case t.State.Active():
	case t.State == UpNext && st == Stopped:
	default:
		return Task{}, notStartable()
	}
	if step < t.Step || step > j.c.Turns {
		return Task{}, badRequest(i18n.T("A Hands task's step count only goes forward, within its turns."))
	}
	t.State, t.Step, t.Reason = st, step, reason
	if st.Ended() {
		t.Ended = endAt(*t, now)
	}
	return *t, j.write(&f)
}

// Recover runs once when the daemon starts: every task that was running or
// waiting becomes Interrupted at now, for ReasonRestart, and is returned.
// None resumes; each starts again only through Start from a button, with
// the goal restated, since the journal never kept it.
func (j *Journal) Recover(now time.Time) ([]Task, error) {
	j.mu.Lock()
	defer j.mu.Unlock()
	f, err := j.read()
	if err != nil {
		return nil, err
	}
	var held []Task
	for i := range f.Tasks {
		t := &f.Tasks[i]
		if t.State == UpNext || t.State.Active() {
			t.State, t.Reason, t.Ended = Interrupted, ReasonRestart, endAt(*t, now)
			held = append(held, *t)
		}
	}
	if len(held) == 0 {
		return nil, nil
	}
	return held, j.write(&f)
}

// Purge drops every ended task that ended more than Age before now and
// every project unused for longer, then keeps at most Rows of each. A
// running or waiting task is never purged.
func (j *Journal) Purge(now time.Time) error {
	j.mu.Lock()
	defer j.mu.Unlock()
	f, err := j.read()
	if err != nil {
		return err
	}
	cut := now.Add(-j.c.Age)
	n := len(f.Tasks) + len(f.Projects)
	f.Tasks = slices.DeleteFunc(f.Tasks, func(t Task) bool { return t.State.Ended() && t.Ended.Before(cut) })
	f.Projects = slices.DeleteFunc(f.Projects, func(p Project) bool { return p.Used.Before(cut) })
	j.trim(&f)
	if len(f.Tasks)+len(f.Projects) == n {
		return nil
	}
	return j.write(&f)
}

// List returns every task, the earliest queued first.
func (j *Journal) List() ([]Task, error) {
	j.mu.Lock()
	defer j.mu.Unlock()
	f, err := j.read()
	return f.Tasks, err
}

// Running returns the running task, if there is one.
func (j *Journal) Running() (Task, bool, error) {
	ts, err := j.List()
	if i := slices.IndexFunc(ts, func(t Task) bool { return t.State.Active() }); i >= 0 {
		return ts[i], true, err
	}
	return Task{}, false, err
}

// Forget drops the task with id. A running task is stopped first.
func (j *Journal) Forget(id string) error {
	j.mu.Lock()
	defer j.mu.Unlock()
	f, err := j.read()
	if err != nil {
		return err
	}
	i := slices.IndexFunc(f.Tasks, func(t Task) bool { return t.ID == id })
	if i < 0 {
		return notFound()
	}
	if f.Tasks[i].State.Active() {
		return busy()
	}
	f.Tasks = slices.Delete(f.Tasks, i, i+1)
	return j.write(&f)
}

// ForgetAll drops every task and project and removes the file. A running
// task is stopped first.
func (j *Journal) ForgetAll() error {
	j.mu.Lock()
	defer j.mu.Unlock()
	f, err := j.read()
	if err != nil {
		return err
	}
	if slices.ContainsFunc(f.Tasks, func(t Task) bool { return t.State.Active() }) {
		return busy()
	}
	if err := os.Remove(j.path); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return nil
}

// Reset moves the journal file to DamagedPath, replacing any earlier one,
// and leaves the journal empty.
func (j *Journal) Reset() error {
	j.mu.Lock()
	defer j.mu.Unlock()
	err := os.Rename(j.path, j.DamagedPath())
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	return err
}

// Keep keeps the ended task with taskID as the project projectID at now:
// its scope, app id and step count, and nothing else.
func (j *Journal) Keep(taskID, projectID string, now time.Time) (Project, error) {
	if !validID(projectID) {
		return Project{}, badRequest(i18n.T("That is not a Hands project id."))
	}
	j.mu.Lock()
	defer j.mu.Unlock()
	f, err := j.read()
	if err != nil {
		return Project{}, err
	}
	i := slices.IndexFunc(f.Tasks, func(t Task) bool { return t.ID == taskID })
	if i < 0 {
		return Project{}, notFound()
	}
	t := f.Tasks[i]
	if !t.State.Ended() {
		return Project{}, badRequest(i18n.T("Only a task that ended can be kept."))
	}
	if slices.ContainsFunc(f.Projects, func(p Project) bool { return p.ID == projectID }) {
		return Project{}, badRequest(i18n.T("That Hands project is already kept."))
	}
	at := stamp(now)
	p := Project{ID: projectID, Scope: t.Scope, App: t.App, Steps: t.Step, Kept: at, Used: at}
	f.Projects = append(f.Projects, p)
	return p, j.write(&f)
}

// Continue queues a new task taskID from origin in the project's scope and
// app, as Enqueue does, and marks the project used at now. The person
// restates the goal.
func (j *Journal) Continue(projectID, taskID, origin string, now time.Time) (Task, error) {
	j.mu.Lock()
	defer j.mu.Unlock()
	f, err := j.read()
	if err != nil {
		return Task{}, err
	}
	i := slices.IndexFunc(f.Projects, func(p Project) bool { return p.ID == projectID })
	if i < 0 {
		return Task{}, protocol.Errorf(protocol.CodeNotFound, "%s", i18n.T("That Hands project is not there any more."))
	}
	at := stamp(now)
	p := &f.Projects[i]
	t := Task{ID: taskID, State: UpNext, Scope: p.Scope, App: p.App, Origin: origin, At: at}
	if err := checkTask(t, j.c); err != nil {
		return Task{}, err
	}
	p.Used = maxTime(p.Used, at)
	return j.enqueue(&f, t)
}

// Projects returns every kept project, the most recently used first.
func (j *Journal) Projects() ([]Project, error) {
	j.mu.Lock()
	defer j.mu.Unlock()
	f, err := j.read()
	ps := f.Projects
	slices.SortStableFunc(ps, func(a, b Project) int { return b.Used.Compare(a.Used) })
	return ps, err
}

// ForgetProject drops the project with id.
func (j *Journal) ForgetProject(id string) error {
	j.mu.Lock()
	defer j.mu.Unlock()
	f, err := j.read()
	if err != nil {
		return err
	}
	i := slices.IndexFunc(f.Projects, func(p Project) bool { return p.ID == id })
	if i < 0 {
		return protocol.Errorf(protocol.CodeNotFound, "%s", i18n.T("That Hands project is not there any more."))
	}
	f.Projects = slices.Delete(f.Projects, i, i+1)
	return j.write(&f)
}

type file struct {
	V        int       `json:"v"`
	Tasks    []Task    `json:"tasks"`
	Projects []Project `json:"projects"`
}

// read returns the journal, the earliest queued task first. The shape is
// strict: an unknown field, an unknown state, scope, origin or reason, a
// malformed id or app id, a step past the turns, an end that does not fit
// the state, two running tasks or too many waiting ones make it damaged.
func (j *Journal) read() (file, error) {
	fh, err := os.Open(j.path)
	if errors.Is(err, os.ErrNotExist) {
		return file{}, nil
	}
	if err != nil {
		return file{}, err
	}
	defer fh.Close()
	data, err := io.ReadAll(io.LimitReader(fh, MaxStoreBytes+1))
	if err != nil {
		return file{}, err
	}
	if len(data) > MaxStoreBytes {
		return file{}, damaged()
	}
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	var f file
	l := limits()
	if dec.Decode(&f) != nil || dec.More() || f.V != storeVersion || len(f.Tasks) > l.Rows || len(f.Projects) > l.Rows {
		return file{}, damaged()
	}
	seen := make(map[string]bool, len(f.Tasks))
	running, waiting := 0, 0
	for _, t := range f.Tasks {
		if checkTask(t, l) != nil || seen[t.ID] {
			return file{}, damaged()
		}
		seen[t.ID] = true
		running += b2i(t.State.Active())
		waiting += b2i(t.State == UpNext)
	}
	if running > 1 || waiting > l.QueueMax {
		return file{}, damaged()
	}
	clear(seen)
	for _, p := range f.Projects {
		if checkProject(p, l) != nil || seen[p.ID] {
			return file{}, damaged()
		}
		seen[p.ID] = true
	}
	sortTasks(f.Tasks)
	return f, nil
}

// write trims f to Rows and replaces the file with it, or removes the file
// when nothing is left, so nothing is kept at rest once the journal is
// empty.
func (j *Journal) write(f *file) error {
	j.trim(f)
	if len(f.Tasks) == 0 && len(f.Projects) == 0 {
		if err := os.Remove(j.path); err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}
		return nil
	}
	sortTasks(f.Tasks)
	slices.SortFunc(f.Projects, func(a, b Project) int { return strings.Compare(a.ID, b.ID) })
	out := file{V: storeVersion, Tasks: f.Tasks, Projects: f.Projects}
	if out.Tasks == nil {
		out.Tasks = []Task{}
	}
	if out.Projects == nil {
		out.Projects = []Project{}
	}
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "\t")
	if err := enc.Encode(out); err != nil {
		return err
	}
	if buf.Len() > MaxStoreBytes {
		return badRequest(i18n.T("The Hands task journal is too large to save."))
	}
	return writeAtomic(j.path, buf.Bytes())
}

// trim keeps at most Rows tasks, dropping the earliest ended first, and at
// most Rows projects, dropping the least recently used first. Running and
// waiting tasks are never more than QueueMax+1, fewer than Rows.
func (j *Journal) trim(f *file) {
	for len(f.Tasks) > j.c.Rows {
		i := -1
		for k, t := range f.Tasks {
			if t.State.Ended() && (i < 0 || endedBefore(t, f.Tasks[i])) {
				i = k
			}
		}
		if i < 0 {
			break
		}
		f.Tasks = slices.Delete(f.Tasks, i, i+1)
	}
	for len(f.Projects) > j.c.Rows {
		i := 0
		for k, p := range f.Projects {
			if p.Used.Before(f.Projects[i].Used) || p.Used.Equal(f.Projects[i].Used) && p.ID < f.Projects[i].ID {
				i = k
			}
		}
		f.Projects = slices.Delete(f.Projects, i, i+1)
	}
}

func endedBefore(a, b Task) bool {
	if c := a.Ended.Compare(b.Ended); c != 0 {
		return c < 0
	}
	return a.ID < b.ID
}

func sortTasks(ts []Task) {
	slices.SortStableFunc(ts, func(a, b Task) int {
		return cmp.Or(a.At.Compare(b.At), strings.Compare(a.ID, b.ID))
	})
}

// checkTask checks t against the bounds in c.
func checkTask(t Task, c Config) error {
	switch {
	case !validID(t.ID):
		return badRequest(i18n.T("That is not a Hands task id."))
	case !t.State.known():
		return badRequest(i18n.T("That is not a Hands task state."))
	case !validApp(t.App):
		return badRequest(i18n.T("A Hands task keeps an app id and nothing else about the app."))
	case !validScope(t.Scope, t.App):
		return badRequest(i18n.T("That is not a Hands scope."))
	case !slices.Contains(origins, t.Origin):
		return badRequest(i18n.T("That is not where a Hands task can come from."))
	case t.Step < 0 || t.Step > c.Turns:
		return badRequest(i18n.T("A Hands task's step count is out of range."))
	case !validReason(t.Reason):
		return badRequest(i18n.T("That is not a Hands reason."))
	case t.At.IsZero() || t.Ended.IsZero() == t.State.Ended() || !t.Ended.IsZero() && t.Ended.Before(t.At):
		return badRequest(i18n.T("A Hands task's times do not fit its state."))
	}
	return nil
}

// checkProject checks p against the bounds in c.
func checkProject(p Project, c Config) error {
	switch {
	case !validID(p.ID) || !validApp(p.App) || !validScope(p.Scope, p.App):
		return badRequest(i18n.T("That is not a Hands project."))
	case p.Steps < 0 || p.Steps > c.Turns:
		return badRequest(i18n.T("A Hands project's step count is out of range."))
	case p.Kept.IsZero() || p.Used.Before(p.Kept):
		return badRequest(i18n.T("A Hands project's times do not fit."))
	}
	return nil
}

// stamp is t in UTC to the millisecond, as the file keeps it.
func stamp(t time.Time) time.Time { return t.UTC().Truncate(time.Millisecond) }

// endAt is when t ends at now: never before it was queued, even when the
// clock was set back in between.
func endAt(t Task, now time.Time) time.Time { return maxTime(stamp(now), t.At) }

func maxTime(a, b time.Time) time.Time {
	if a.Before(b) {
		return b
	}
	return a
}

func b2i(b bool) int {
	if b {
		return 1
	}
	return 0
}

// writeAtomic replaces path with data through a synced temporary file.
func writeAtomic(path string, data []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), "."+filepath.Base(path)+".*")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	if err := tmp.Chmod(0o600); err != nil {
		tmp.Close()
		return err
	}
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), path)
}
