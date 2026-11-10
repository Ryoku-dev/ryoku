package workspace

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	i18n "ryoku-i18n"
	"ryoku-rashin/internal/core"
	gitpkg "ryoku-rashin/internal/git"
	"ryoku-rashin/internal/llm"
	"ryoku-rashin/internal/paths"
	"ryoku-rashin/internal/protocol"
)

type ownedWorktree struct {
	Path     string               `json:"path"`
	Base     string               `json:"base,omitempty"`
	BaseSHA  string               `json:"baseSha,omitempty"`
	Parent   string               `json:"parent,omitempty"`
	Task     string               `json:"task,omitempty"`
	Setup    protocol.SetupResult `json:"setup"`
	Created  time.Time            `json:"created"`
	Activity time.Time            `json:"activity"`
}

type storedComment struct {
	Comment protocol.ReviewComment `json:"comment"`
	Sent    bool                   `json:"sent,omitempty"`
}

type projectRecord struct {
	ID        string                   `json:"id"`
	Name      string                   `json:"name"`
	Root      string                   `json:"root"`
	Git       bool                     `json:"git"`
	Settings  protocol.ProjectSettings `json:"settings"`
	Created   time.Time                `json:"created"`
	Activity  time.Time                `json:"activity"`
	Worktrees []ownedWorktree          `json:"worktrees,omitempty"`
	Tasks     []protocol.TaskInfo      `json:"tasks,omitempty"`
	Comments  []storedComment          `json:"comments,omitempty"`
}

type diskState struct {
	Projects []projectRecord `json:"projects"`
}

type statusCache struct {
	porcelain [32]byte
	operation string
	baseSHA   string
	base      string
	result    protocol.GitStatusResult
}

type watcher struct {
	file     *os.File
	stop     chan struct{}
	readDone chan struct{}
	done     chan struct{}
	ioMu     sync.Mutex
}
type setupRun struct {
	cancel context.CancelFunc
	done   chan struct{}
}

type Work struct {
	mu               sync.RWMutex
	activityMu       sync.Mutex
	projects         map[string]*projectRecord
	events           core.Broadcaster
	client           llm.Client
	settings         func() protocol.Settings
	sessions         core.SessionControl
	status           map[string]statusCache
	watchRef         map[string]int
	watchers         map[string]*watcher
	setups           map[string]*setupRun
	cleanupLast      map[string]time.Time
	background       context.Context
	cancelBackground context.CancelFunc
	backgroundWG     sync.WaitGroup
	busy             atomic.Int32
	closed           bool
}

func Open(events core.Broadcaster, client llm.Client, settings func() protocol.Settings) (*Work, error) {
	if settings == nil {
		settings = func() protocol.Settings { return protocol.Settings{} }
	}
	background, cancelBackground := context.WithCancel(context.Background())
	w := &Work{
		projects: make(map[string]*projectRecord), events: events, client: client, settings: settings,
		status: make(map[string]statusCache), watchRef: make(map[string]int), watchers: make(map[string]*watcher),
		setups: make(map[string]*setupRun), cleanupLast: make(map[string]time.Time),
		background: background, cancelBackground: cancelBackground,
	}
	data, err := os.ReadFile(paths.Projects())
	if errors.Is(err, fs.ErrNotExist) {
		return w, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read projects: %w", err)
	}
	if len(strings.TrimSpace(string(data))) == 0 {
		return w, nil
	}
	var state diskState
	if err := json.Unmarshal(data, &state); err != nil {
		return nil, fmt.Errorf("parse projects: %w", err)
	}
	for i := range state.Projects {
		p := state.Projects[i]
		w.projects[p.ID] = &p
	}
	return w, nil
}

func (w *Work) Bind(sessions core.SessionControl) {
	w.mu.Lock()
	w.sessions = sessions
	w.mu.Unlock()
}

func (w *Work) Busy() bool { return w.busy.Load() > 0 }

func (w *Work) Close() error {
	w.mu.Lock()
	if w.closed {
		w.mu.Unlock()
		return nil
	}
	w.closed = true
	watchers := make([]*watcher, 0, len(w.watchers))
	for _, current := range w.watchers {
		watchers = append(watchers, current)
	}
	setups := make([]*setupRun, 0, len(w.setups))
	for _, run := range w.setups {
		setups = append(setups, run)
	}
	w.watchers = make(map[string]*watcher)
	w.watchRef = make(map[string]int)
	cancelBackground := w.cancelBackground
	w.mu.Unlock()
	cancelBackground()
	w.backgroundWG.Wait()
	for _, run := range setups {
		run.cancel()
	}
	for _, run := range setups {
		<-run.done
	}
	for _, current := range watchers {
		stopWatcher(current)
	}
	return nil
}

func (w *Work) saveLocked() error {
	state := diskState{Projects: make([]projectRecord, 0, len(w.projects))}
	for _, p := range w.projects {
		state.Projects = append(state.Projects, *p)
	}
	sort.Slice(state.Projects, func(i, j int) bool { return state.Projects[i].ID < state.Projects[j].ID })
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	dir := filepath.Dir(paths.Projects())
	if err := paths.EnsureDir(dir); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(dir, ".projects-*.tmp")
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
	if err := os.Rename(tmpName, paths.Projects()); err != nil {
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

func projectID(root string) string {
	sum := sha256.Sum256([]byte(root))
	return hex.EncodeToString(sum[:6])
}

func newID(prefix string) string {
	var raw [6]byte
	if _, err := rand.Read(raw[:]); err != nil {
		now := time.Now().UnixNano()
		for i := range raw {
			raw[i] = byte(now >> (i * 8))
		}
	}
	return prefix + "-" + hex.EncodeToString(raw[:])
}

func (w *Work) projectLocked(id string) (*projectRecord, error) {
	p, ok := w.projects[id]
	if !ok {
		return nil, protocol.Errorf(protocol.CodeNotFound, "%s", i18n.Tf("No project %s.", id))
	}
	return p, nil
}

func (w *Work) project(id string) (*projectRecord, error) {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return w.projectLocked(id)
}

func badRequest(format string, args ...any) error {
	return protocol.Errorf(protocol.CodeBadRequest, "%s", i18n.Tf(format, args...))
}

func conflict(format string, args ...any) error {
	return protocol.Errorf(protocol.CodeConflict, "%s", i18n.Tf(format, args...))
}

func unavailable(format string, args ...any) error {
	return protocol.Errorf(protocol.CodeUnavailable, "%s", i18n.Tf(format, args...))
}

func decode[T any](raw json.RawMessage) (T, error) {
	var value T
	if err := protocol.Unmarshal(raw, &value); err != nil {
		return value, protocol.Errorf(protocol.CodeBadRequest, "%s", i18n.T("Request parameters are invalid."))
	}
	return value, nil
}

func (w *Work) broadcast(event string, data any) {
	if w.events != nil {
		w.events.Broadcast(event, data)
	}
}

func (w *Work) broadcastProjects(ctx context.Context) {
	w.broadcast(protocol.EvtProjectsChanged, protocol.ProjectsResult{Projects: w.Projects(ctx)})
}

func (w *Work) broadcastTasks(project string) {
	w.mu.RLock()
	p := w.projects[project]
	var tasks []protocol.TaskInfo
	if p != nil {
		tasks = append(tasks, p.Tasks...)
	}
	w.mu.RUnlock()
	sortTasks(tasks)
	w.broadcast(protocol.EvtTasksChanged, protocol.TasksResult{Tasks: tasks})
}

func (w *Work) trackMutation(project, actor, action, target string, result *error) func() {
	return func() {
		if project == "" {
			return
		}
		err := *result
		w.recordActivity(project, actor, action, target, err == nil, errorDetail(err))
	}
}

func (w *Work) recordActivity(project, actor, action, target string, ok bool, detail string) {
	entry := protocol.Activity{Time: time.Now(), Actor: actor, Action: action, Target: target, OK: ok, Detail: detail}
	w.mu.Lock()
	if p := w.projects[project]; p != nil {
		p.Activity = entry.Time
		_ = w.saveLocked()
	}
	w.mu.Unlock()
	w.activityMu.Lock()
	defer w.activityMu.Unlock()
	path := paths.ActivityLog(project)
	if err := paths.EnsureDir(filepath.Dir(path)); err != nil {
		return
	}
	line, err := json.Marshal(entry)
	if err != nil {
		return
	}
	file, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	if err == nil {
		_, _ = file.Write(append(line, '\n'))
		_ = file.Close()
	}
	w.trimActivity(path)
}

func (w *Work) trimActivity(path string) {
	data, err := os.ReadFile(path)
	if err != nil {
		return
	}
	lines := strings.Split(strings.TrimSuffix(string(data), "\n"), "\n")
	if len(lines) <= 5000 {
		return
	}
	out := strings.Join(lines[len(lines)-5000:], "\n") + "\n"
	tmp := path + ".tmp"
	if os.WriteFile(tmp, []byte(out), 0o600) == nil {
		_ = os.Rename(tmp, path)
	}
}

func (w *Work) withMutation(project, action, target string, fn func() error) error {
	w.busy.Add(1)
	defer w.busy.Add(-1)
	err := fn()
	w.recordActivity(project, "you", action, target, err == nil, errorDetail(err))
	return err
}

func errorDetail(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}

func mergeJSON[T any](current T, patch json.RawMessage) (T, error) {
	var zero T
	base, err := json.Marshal(current)
	if err != nil {
		return zero, err
	}
	var target, change any
	if err := json.Unmarshal(base, &target); err != nil {
		return zero, err
	}
	if err := json.Unmarshal(patch, &change); err != nil {
		return zero, badRequest("The patch is not valid JSON: %v.", err)
	}
	merged, err := json.Marshal(applyMergePatch(target, change))
	if err != nil {
		return zero, err
	}
	var result T
	if err := json.Unmarshal(merged, &result); err != nil {
		return zero, badRequest("The patch has the wrong shape: %v.", err)
	}
	return result, nil
}

func applyMergePatch(target, patch any) any {
	change, object := patch.(map[string]any)
	if !object {
		return patch
	}
	base, ok := target.(map[string]any)
	if !ok {
		base = make(map[string]any)
	}
	for key, value := range change {
		if value == nil {
			delete(base, key)
		} else {
			base[key] = applyMergePatch(base[key], value)
		}
	}
	return base
}

func (w *Work) projectForPath(ctx context.Context, path string) (*projectRecord, *gitpkg.Repo, error) {
	absolute, err := filepath.Abs(path)
	if err != nil {
		return nil, nil, badRequest("The worktree path is invalid.")
	}
	repo, err := gitpkg.Open(ctx, absolute)
	if err != nil {
		return nil, nil, unavailable("The folder is not an available Git worktree: %v.", err)
	}
	w.mu.RLock()
	candidates := make([]*projectRecord, 0, len(w.projects))
	for _, p := range w.projects {
		if p.Git {
			candidates = append(candidates, p)
		}
	}
	w.mu.RUnlock()
	for _, p := range candidates {
		if samePath(p.Root, repo.Root) {
			return p, repo, nil
		}
		if registered, openErr := gitpkg.Open(ctx, p.Root); openErr == nil && samePath(registered.CommonDir, repo.CommonDir) {
			return p, repo, nil
		}
	}
	return nil, nil, protocol.Errorf(protocol.CodeNotFound, "%s", i18n.T("The worktree does not belong to a registered project."))
}

func samePath(a, b string) bool {
	cleanA, errA := filepath.EvalSymlinks(filepath.Clean(a))
	cleanB, errB := filepath.EvalSymlinks(filepath.Clean(b))
	if errA == nil {
		a = cleanA
	}
	if errB == nil {
		b = cleanB
	}
	return filepath.Clean(a) == filepath.Clean(b)
}

func sortTasks(tasks []protocol.TaskInfo) {
	sort.SliceStable(tasks, func(i, j int) bool {
		if tasks[i].Order != tasks[j].Order {
			return tasks[i].Order < tasks[j].Order
		}
		return tasks[i].Created.Before(tasks[j].Created)
	})
}
