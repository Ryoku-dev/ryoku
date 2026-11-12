package state

import (
	"encoding/json"
	"sort"
	"sync"

	"ryoku-rashin/internal/app/chat"
	"ryoku-rashin/internal/protocol"
)

// Connection is the app's view of the daemon connection.
type Connection uint8

const (
	Off Connection = iota
	Connecting
	Ready
	Unreachable
)

// Attached is one open transcript.
type Attached struct {
	Info  protocol.SessionInfo
	Model *chat.Model
}

// Snapshot is an immutable view of the store at one revision. Slices and maps
// are copied before publication. Chat models remain store-owned stateful views.
type Snapshot struct {
	Revision uint64
	// Epoch increments each time the daemon connection becomes ready.
	Epoch      int
	Connection Connection
	Status     protocol.Status
	Sessions   map[string]protocol.SessionInfo
	Order      []string
	Settings   protocol.Settings
	Providers  []protocol.ProviderInfo
	Harnesses  []protocol.HarnessInfo
	Indexes    []protocol.IndexInfo
	Projects   []protocol.ProjectInfo
	Worktrees  map[string][]protocol.WorktreeInfo
	Tasks      map[string][]protocol.TaskInfo
	Reviews    map[string][]protocol.ReviewComment
	Progress   map[string]protocol.IndexProgress
	Attached   map[string]Attached
	Focus      protocol.UIOpenParams
	Notices    []protocol.Notice
	// Screens are the agent screens that are up, by session.
	Screens map[string]protocol.ScreenState
	// Teams are by name, TeamTasks by id, Approvals newest first, Routines by
	// team then title; the Vault holds no value.
	Teams     []protocol.Team
	TeamTasks map[string]protocol.TeamTask
	Approvals []protocol.Approval
	Routines  []protocol.Routine
	Vault     protocol.VaultInfo
	// Memory is memory's status, nil when the daemon runs without it;
	// MemoryChanges are the changes seen so far by id, and MemoryRevision
	// moves with every memory.changed, so a page showing memories knows to
	// load them again.
	Memory         *protocol.MemoryStatus
	MemoryChanges  map[string]protocol.MemoryChange
	MemoryRevision uint64
	// MemorySetup is the latest memory.setup progress, kept after its last
	// step until the next setup starts or a snapshot replaces it.
	MemorySetup *protocol.MemorySetupProgress
}

// Store reduces daemon snapshots and events into one app state.
type Store struct {
	mu         sync.RWMutex
	revision   uint64
	epoch      int
	connection Connection
	status     protocol.Status
	sessions   map[string]protocol.SessionInfo
	order      []string
	settings   protocol.Settings
	providers  []protocol.ProviderInfo
	indexes    []protocol.IndexInfo
	harnesses  []protocol.HarnessInfo
	projects   []protocol.ProjectInfo
	worktrees  map[string][]protocol.WorktreeInfo
	tasks      map[string][]protocol.TaskInfo
	reviews    map[string][]protocol.ReviewComment
	progress   map[string]protocol.IndexProgress
	attached   map[string]*chat.Model
	// what a session streamed while the replay that opens it was still on its way
	arriving    map[string]*arrival
	focus       protocol.UIOpenParams
	notices     []protocol.Notice
	screens     map[string]protocol.ScreenState
	teamsState  teamsState
	memoryState memoryState
	gitNext     uint64
	gitChanged  map[string]map[uint64]func()
	invalidate  func()
}

func New(invalidate func()) *Store {
	return &Store{
		sessions:    make(map[string]protocol.SessionInfo),
		progress:    make(map[string]protocol.IndexProgress),
		attached:    make(map[string]*chat.Model),
		arriving:    make(map[string]*arrival),
		worktrees:   make(map[string][]protocol.WorktreeInfo),
		tasks:       make(map[string][]protocol.TaskInfo),
		reviews:     make(map[string][]protocol.ReviewComment),
		gitChanged:  make(map[string]map[uint64]func()),
		teamsState:  newTeamsState(),
		memoryState: newMemoryState(),
		invalidate:  invalidate,
	}
}

func (s *Store) changedLocked() func() {
	s.revision++
	return s.invalidate
}

func (s *Store) SetInvalidate(invalidate func()) {
	s.mu.Lock()
	s.invalidate = invalidate
	s.mu.Unlock()
}

func (s *Store) SetConnection(connection Connection) {
	s.mu.Lock()
	if s.connection == connection {
		s.mu.Unlock()
		return
	}
	s.connection = connection
	if connection == Ready {
		s.epoch++
	}
	invalidate := s.changedLocked()
	s.mu.Unlock()
	if invalidate != nil {
		invalidate()
	}
}

// ApplySnapshot replaces daemon-owned collection state while retaining open
// transcript models, whose event sequence can be ahead of this snapshot.
func (s *Store) ApplySnapshot(snapshot protocol.Snapshot) {
	s.mu.Lock()
	s.status = snapshot.Status
	s.settings = snapshot.Settings
	s.providers = cloneProviders(snapshot.Providers)
	s.harnesses = cloneHarnesses(snapshot.Harnesses)
	s.indexes = cloneIndexes(snapshot.Indexes)
	s.projects = cloneProjects(snapshot.Projects)
	s.tasks = groupTasks(snapshot.Tasks)
	s.teamsState.applySnapshotLocked(snapshot)
	s.memoryState.applySnapshotLocked(snapshot)
	s.screens = make(map[string]protocol.ScreenState, len(snapshot.Screens))
	for _, screen := range snapshot.Screens {
		s.screens[screen.Session] = screen
	}
	s.sessions = make(map[string]protocol.SessionInfo, len(snapshot.Sessions))
	for _, info := range snapshot.Sessions {
		s.sessions[info.ID] = info
		if model := s.attached[info.ID]; model != nil {
			model.SetInfo(info)
		}
	}
	s.rebuildOrderLocked()
	invalidate := s.changedLocked()
	s.mu.Unlock()
	if invalidate != nil {
		invalidate()
	}
}

// Open marks a session as attached and returns the last applied sequence. The
// replay asked for with it is applied by whoever asked, while the events the
// session streams after it are applied by the reader of the connection, and
// the two do not wait for each other. So from here until the replay is in,
// or Opened says it never will be, live events are held: a replay from zero
// that lands late would otherwise wipe everything that arrived before it.
func (s *Store) Open(id string) int64 {
	s.mu.Lock()
	model := s.attached[id]
	if model == nil {
		model = chat.NewModel(s.sessions[id])
		s.attached[id] = model
	}
	if s.arriving[id] == nil {
		s.arriving[id] = new(arrival)
	}
	s.arriving[id].replays++
	seq := model.LastSeq()
	invalidate := s.changedLocked()
	s.mu.Unlock()
	if invalidate != nil {
		invalidate()
	}
	return seq
}

// Opened ends the wait Open began when the replay will not come, as after a
// failed attach, so what the session streams is shown again.
func (s *Store) Opened(id string) {
	s.mu.Lock()
	s.landLocked(id)
	invalidate := s.changedLocked()
	s.mu.Unlock()
	if invalidate != nil {
		invalidate()
	}
}

// arrival is a session being opened: how many replays are still on their way
// and what it streamed in the meantime.
type arrival struct {
	replays int
	held    []protocol.SessionEvent
}

// landLocked counts one replay in. After the last one it applies what was
// held, in the order it arrived; the model drops whatever a replay covered.
func (s *Store) landLocked(id string) {
	arriving := s.arriving[id]
	if arriving == nil {
		return
	}
	if arriving.replays--; arriving.replays > 0 {
		return
	}
	delete(s.arriving, id)
	if model := s.attached[id]; model != nil {
		for _, event := range arriving.held {
			model.Apply(event)
		}
	}
}

func (s *Store) Close(id string) {
	s.mu.Lock()
	delete(s.arriving, id)
	if _, ok := s.attached[id]; !ok {
		s.mu.Unlock()
		return
	}
	delete(s.attached, id)
	invalidate := s.changedLocked()
	s.mu.Unlock()
	if invalidate != nil {
		invalidate()
	}
}

// OpenSessions returns each attached session and its resume sequence.
func (s *Store) OpenSessions() map[string]int64 {
	s.mu.RLock()
	defer s.mu.RUnlock()
	result := make(map[string]int64, len(s.attached))
	for id, model := range s.attached {
		result[id] = model.LastSeq()
	}
	return result
}

// ApplyAttach folds a session.attach replay. A replay from zero is a complete
// reset; a reconnect replay after Since is appended to the retained model.
func (s *Store) ApplyAttach(since int64, result protocol.SessionAttachResult) {
	s.mu.Lock()
	model := s.attached[result.Info.ID]
	if model == nil {
		model = chat.NewModel(result.Info)
		s.attached[result.Info.ID] = model
	}
	if since == 0 {
		model.Reset(result.Info, result.Events)
	} else {
		model.SetInfo(result.Info)
		for _, event := range result.Events {
			model.Apply(event)
		}
	}
	// the live stream starts after the last page, so only then is what was held next in line
	if !result.More {
		s.landLocked(result.Info.ID)
	}
	s.sessions[result.Info.ID] = result.Info
	s.rebuildOrderLocked()
	invalidate := s.changedLocked()
	s.mu.Unlock()
	if invalidate != nil {
		invalidate()
	}
}

// Apply reduces one daemon event.
func (s *Store) Apply(event protocol.Event) error {
	s.mu.Lock()
	var err error
	var listeners []func()
	if handled, teamsErr := s.teamsState.applyLocked(event); handled {
		err = teamsErr
	}
	if handled, memoryErr := s.memoryState.applyLocked(event); handled {
		err = memoryErr
	}
	switch event.Event {
	case protocol.EvtSessionEvent:
		var data protocol.SessionEventData
		if err = json.Unmarshal(event.Data, &data); err == nil {
			if arriving := s.arriving[data.Session]; arriving != nil {
				arriving.held = append(arriving.held, data.SessionEvent)
			} else if model := s.attached[data.Session]; model != nil {
				model.Apply(data.SessionEvent)
			}
		}
	case protocol.EvtSessionInfo:
		var info protocol.SessionInfo
		if err = json.Unmarshal(event.Data, &info); err == nil {
			s.sessions[info.ID] = info
			if model := s.attached[info.ID]; model != nil {
				model.SetInfo(info)
			}
			s.rebuildOrderLocked()
		}
	case protocol.EvtSessionsChanged:
		var change protocol.SessionsChanged
		if err = json.Unmarshal(event.Data, &change); err == nil {
			for _, id := range change.Removed {
				delete(s.sessions, id)
				delete(s.attached, id)
				delete(s.arriving, id)
			}
			s.rebuildOrderLocked()
		}
	case protocol.EvtSettingsChanged:
		err = json.Unmarshal(event.Data, &s.settings)
	case protocol.EvtAPIChanged:
		err = json.Unmarshal(event.Data, &s.status.API)
	case protocol.EvtProvidersChanged:
		var result protocol.ProvidersResult
		if err = json.Unmarshal(event.Data, &result); err == nil {
			s.providers = cloneProviders(result.Providers)
		}
	case protocol.EvtHarnessesChanged:
		var result protocol.HarnessesResult
		if err = json.Unmarshal(event.Data, &result); err == nil {
			s.harnesses = cloneHarnesses(result.Harnesses)
		}
	case protocol.EvtIndexProgress:
		var progress protocol.IndexProgress
		if err = json.Unmarshal(event.Data, &progress); err == nil {
			s.progress[progress.ID] = progress
		}
	case protocol.EvtIndexChanged:
		var info protocol.IndexInfo
		if err = json.Unmarshal(event.Data, &info); err == nil {
			replaced := false
			for index := range s.indexes {
				if s.indexes[index].ID == info.ID {
					s.indexes[index] = info
					replaced = true
					break
				}
			}
			if !replaced {
				s.indexes = append(s.indexes, info)
			}
		}
	case protocol.EvtProjectsChanged:
		var result protocol.ProjectsResult
		if err = json.Unmarshal(event.Data, &result); err == nil {
			s.projects = cloneProjects(result.Projects)
		}
	case protocol.EvtWorktreesChanged:
		var result protocol.WorktreesResult
		if err = json.Unmarshal(event.Data, &result); err == nil {
			s.worktrees[result.Project] = cloneWorktrees(result.Worktrees)
		}
	case protocol.EvtTasksChanged:
		var result protocol.TasksResult
		if err = json.Unmarshal(event.Data, &result); err == nil {
			project := ""
			if len(result.Tasks) > 0 {
				project = result.Tasks[0].Project
			}
			if project != "" {
				s.tasks[project] = cloneTasks(result.Tasks)
			}
		}
	case protocol.EvtReviewChanged:
		var result protocol.ReviewResult
		if err = json.Unmarshal(event.Data, &result); err == nil {
			s.reviews[result.Worktree] = cloneReviews(result.Comments)
		}
	case protocol.EvtGitChanged:
		var changed protocol.GitChanged
		if err = json.Unmarshal(event.Data, &changed); err == nil {
			for _, listener := range s.gitChanged[changed.Project] {
				listeners = append(listeners, listener)
			}
		}
	case protocol.EvtUIFocus:
		err = json.Unmarshal(event.Data, &s.focus)
	case protocol.EvtScreen:
		var screen protocol.ScreenState
		if err = json.Unmarshal(event.Data, &screen); err == nil {
			if s.screens == nil {
				s.screens = make(map[string]protocol.ScreenState)
			}
			if screen.State == protocol.ScreenStopped {
				delete(s.screens, screen.Session)
			} else {
				s.screens[screen.Session] = screen
			}
		}
	case protocol.EvtNotice:
		var notice protocol.Notice
		if err = json.Unmarshal(event.Data, &notice); err == nil {
			s.notices = append(s.notices, notice)
			if len(s.notices) > 8 {
				s.notices = append([]protocol.Notice(nil), s.notices[len(s.notices)-8:]...)
			}
		}
	}
	if err != nil {
		s.mu.Unlock()
		return err
	}
	invalidate := s.changedLocked()
	s.mu.Unlock()
	if invalidate != nil {
		invalidate()
	}
	for _, listener := range listeners {
		listener()
	}
	return nil
}

func (s *Store) Snapshot() Snapshot {
	s.mu.RLock()
	defer s.mu.RUnlock()
	result := Snapshot{
		Revision:   s.revision,
		Epoch:      s.epoch,
		Connection: s.connection,
		Status:     s.status,
		Sessions:   make(map[string]protocol.SessionInfo, len(s.sessions)),
		Order:      append([]string(nil), s.order...),
		Settings:   s.settings,
		Providers:  cloneProviders(s.providers),
		Harnesses:  cloneHarnesses(s.harnesses),
		Indexes:    cloneIndexes(s.indexes),
		Projects:   cloneProjects(s.projects),
		Worktrees:  cloneWorktreeMap(s.worktrees),
		Tasks:      cloneTaskMap(s.tasks),
		Reviews:    cloneReviewMap(s.reviews),
		Progress:   make(map[string]protocol.IndexProgress, len(s.progress)),
		Attached:   make(map[string]Attached, len(s.attached)),
		Focus:      s.focus,
		Notices:    append([]protocol.Notice(nil), s.notices...),
		Screens:    make(map[string]protocol.ScreenState, len(s.screens)),
	}
	for id, info := range s.sessions {
		result.Sessions[id] = info
	}
	for id, screen := range s.screens {
		result.Screens[id] = screen
	}
	s.teamsState.snapshotLocked(&result)
	s.memoryState.snapshotLocked(&result)
	for id, progress := range s.progress {
		result.Progress[id] = progress
	}
	for id, model := range s.attached {
		result.Attached[id] = Attached{Info: model.Info(), Model: model}
	}
	return result
}

// SetWorktrees stores a worktrees.list result so pages can render it before a
// later worktrees.changed event.
func (s *Store) SetWorktrees(result protocol.WorktreesResult) {
	s.mu.Lock()
	s.worktrees[result.Project] = cloneWorktrees(result.Worktrees)
	invalidate := s.changedLocked()
	s.mu.Unlock()
	if invalidate != nil {
		invalidate()
	}
}

// SetReviews stores a review.list result.
func (s *Store) SetReviews(result protocol.ReviewResult) {
	s.mu.Lock()
	s.reviews[result.Worktree] = cloneReviews(result.Comments)
	invalidate := s.changedLocked()
	s.mu.Unlock()
	if invalidate != nil {
		invalidate()
	}
}

// SetTasks stores a tasks.list result for its requested project.
func (s *Store) SetTasks(project string, result protocol.TasksResult) {
	s.mu.Lock()
	s.tasks[project] = cloneTasks(result.Tasks)
	invalidate := s.changedLocked()
	s.mu.Unlock()
	if invalidate != nil {
		invalidate()
	}
}

// OnGitChanged registers a project-scoped callback. The returned function is
// idempotent and removes it.
func (s *Store) OnGitChanged(project string, callback func()) func() {
	if project == "" || callback == nil {
		return func() {}
	}
	s.mu.Lock()
	s.gitNext++
	id := s.gitNext
	if s.gitChanged[project] == nil {
		s.gitChanged[project] = make(map[uint64]func())
	}
	s.gitChanged[project][id] = callback
	s.mu.Unlock()
	var once sync.Once
	return func() {
		once.Do(func() {
			s.mu.Lock()
			delete(s.gitChanged[project], id)
			if len(s.gitChanged[project]) == 0 {
				delete(s.gitChanged, project)
			}
			s.mu.Unlock()
		})
	}
}

func (s *Store) rebuildOrderLocked() {
	s.order = s.order[:0]
	for id := range s.sessions {
		s.order = append(s.order, id)
	}
	sort.SliceStable(s.order, func(i, j int) bool {
		left, right := s.sessions[s.order[i]], s.sessions[s.order[j]]
		if left.Updated.Equal(right.Updated) {
			return left.ID > right.ID
		}
		return left.Updated.After(right.Updated)
	})
}

func cloneProviders(input []protocol.ProviderInfo) []protocol.ProviderInfo {
	result := append([]protocol.ProviderInfo(nil), input...)
	for index := range result {
		result[index].Keys = append([]protocol.KeyInfo(nil), input[index].Keys...)
	}
	return result
}

func cloneIndexes(input []protocol.IndexInfo) []protocol.IndexInfo {
	result := append([]protocol.IndexInfo(nil), input...)
	for index := range result {
		result[index].Roots = append([]string(nil), input[index].Roots...)
	}
	return result
}

func cloneHarnesses(input []protocol.HarnessInfo) []protocol.HarnessInfo {
	result := append([]protocol.HarnessInfo(nil), input...)
	for index := range result {
		result[index].Command = append([]string(nil), input[index].Command...)
		if input[index].Capabilities != nil {
			capabilities := *input[index].Capabilities
			capabilities.Modes = append([]protocol.Choice(nil), input[index].Capabilities.Modes...)
			result[index].Capabilities = &capabilities
		}
	}
	return result
}

func cloneProjects(input []protocol.ProjectInfo) []protocol.ProjectInfo {
	result := append([]protocol.ProjectInfo(nil), input...)
	for index := range result {
		result[index].Settings.CopyIgnored = append([]string(nil), input[index].Settings.CopyIgnored...)
		result[index].Settings.TrustedScripts = append([]string(nil), input[index].Settings.TrustedScripts...)
	}
	return result
}

func cloneWorktrees(input []protocol.WorktreeInfo) []protocol.WorktreeInfo {
	result := append([]protocol.WorktreeInfo(nil), input...)
	for index := range result {
		result[index].Sessions = append([]string(nil), input[index].Sessions...)
		if input[index].Status != nil {
			status := *input[index].Status
			result[index].Status = &status
		}
	}
	return result
}

func cloneTasks(input []protocol.TaskInfo) []protocol.TaskInfo {
	result := append([]protocol.TaskInfo(nil), input...)
	for index := range result {
		result[index].Deps = append([]string(nil), input[index].Deps...)
		result[index].Gates = append([]protocol.Gate(nil), input[index].Gates...)
		for gate := range result[index].Gates {
			result[index].Gates[gate].Options = append([]string(nil), input[index].Gates[gate].Options...)
		}
		result[index].Sessions = append([]string(nil), input[index].Sessions...)
	}
	return result
}

func cloneReviews(input []protocol.ReviewComment) []protocol.ReviewComment {
	return append([]protocol.ReviewComment(nil), input...)
}

func groupTasks(input []protocol.TaskInfo) map[string][]protocol.TaskInfo {
	result := make(map[string][]protocol.TaskInfo)
	for _, task := range input {
		result[task.Project] = append(result[task.Project], task)
	}
	return result
}

func cloneWorktreeMap(input map[string][]protocol.WorktreeInfo) map[string][]protocol.WorktreeInfo {
	result := make(map[string][]protocol.WorktreeInfo, len(input))
	for project, worktrees := range input {
		result[project] = cloneWorktrees(worktrees)
	}
	return result
}

func cloneTaskMap(input map[string][]protocol.TaskInfo) map[string][]protocol.TaskInfo {
	result := make(map[string][]protocol.TaskInfo, len(input))
	for project, tasks := range input {
		result[project] = cloneTasks(tasks)
	}
	return result
}

func cloneReviewMap(input map[string][]protocol.ReviewComment) map[string][]protocol.ReviewComment {
	result := make(map[string][]protocol.ReviewComment, len(input))
	for worktree, comments := range input {
		result[worktree] = cloneReviews(comments)
	}
	return result
}
