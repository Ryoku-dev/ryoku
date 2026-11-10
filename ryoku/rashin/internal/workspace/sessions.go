package workspace

import (
	"context"
	"encoding/binary"
	"log"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	i18n "ryoku-i18n"
	gitpkg "ryoku-rashin/internal/git"
	"ryoku-rashin/internal/protocol"
)

func (w *Work) PrepareSession(ctx context.Context, params protocol.SessionNewParams) (protocol.SessionNewParams, error) {
	if params.Project == "" {
		return params, nil
	}
	p, err := w.project(params.Project)
	if err != nil {
		return params, err
	}
	if params.Worktree == "" {
		params.Cwd = p.Root
		params.Worktree = p.Root
		if p.Git {
			if status, statusErr := w.gitStatus(ctx, p.Root, false); statusErr == nil {
				params.Branch = status.Branch
			}
		}
		return params, nil
	}
	if params.Worktree == "new" {
		name := params.Name
		if name == "" {
			name = params.Title
		}
		created, err := w.createWorktree(ctx, protocol.WorktreeCreateParams{
			Project: p.ID, Name: name, Mode: protocol.WorktreeNewBranch, Base: params.Base,
			Branch: params.Branch, Task: params.Task,
		}, "you")
		if err != nil {
			return params, err
		}
		params.Cwd, params.Worktree, params.Branch = created.Path, created.Path, created.Branch
		return params, nil
	}
	listed, err := w.listWorktrees(ctx, p.ID, false)
	if err != nil {
		return params, err
	}
	for _, current := range listed.Worktrees {
		if samePath(current.Path, params.Worktree) {
			params.Cwd, params.Worktree, params.Branch = current.Path, current.Path, current.Branch
			return params, nil
		}
	}
	return params, protocol.Errorf(protocol.CodeNotFound, "%s", i18n.T("The selected worktree does not belong to the project."))
}

func sessionWorktreeTop(ctx context.Context, session protocol.SessionInfo) (string, error) {
	path := session.Worktree
	if path == "" {
		path = session.Cwd
	}
	if path == "" {
		return "", os.ErrNotExist
	}
	return gitpkg.TopLevel(ctx, path)
}

func (w *Work) BeforeTurn(ctx context.Context, session protocol.SessionInfo, turn int) (bool, error) {
	top, err := sessionWorktreeTop(ctx, session)
	if err != nil {
		return false, nil
	}
	if _, err := gitpkg.Checkpoint(ctx, top, checkpointRef(session.ID, turn)); err != nil {
		return false, unavailable("The turn checkpoint could not be created: %v.", err)
	}
	return true, nil
}

func (w *Work) AfterTurn(ctx context.Context, session protocol.SessionInfo, turn int) {
	path, err := sessionWorktreeTop(ctx, session)
	if err != nil {
		return
	}
	p, _, err := w.projectForPath(ctx, path)
	if err != nil {
		return
	}
	post := checkpointPostRef(session.ID, turn)
	_, checkpointErr := gitpkg.Checkpoint(ctx, path, post)
	if checkpointErr != nil {
		return
	}
	status, err := w.gitStatus(ctx, path, true)
	if err != nil {
		return
	}
	changed := false
	if diff, diffErr := gitpkg.Diff(ctx, path, gitpkg.DiffOptions{Mode: protocol.DiffRange, From: checkpointRef(session.ID, turn), To: post}); diffErr == nil {
		changed = len(diff.Files) > 0
	}
	now := time.Now()
	w.mu.Lock()
	if current := w.projects[p.ID]; current != nil {
		for i := range current.Worktrees {
			if samePath(current.Worktrees[i].Path, path) {
				current.Worktrees[i].Activity = now
			}
		}
		if session.Task != "" {
			for i := range current.Tasks {
				if current.Tasks[i].ID != session.Task {
					continue
				}
				if changed {
					current.Tasks[i].Status = protocol.TaskReview
				}
				current.Tasks[i].Products = protocol.TaskProducts{
					Branch: status.Branch, Commits: status.Status.AheadBase, Files: len(status.Files),
					Added: status.Status.Added, Removed: status.Status.Removed,
				}
				current.Tasks[i].Updated = now
				break
			}
		}
		recomputeTaskReadiness(current)
		_ = w.saveLocked()
	}
	w.mu.Unlock()
	if session.Task != "" {
		w.broadcastTasks(p.ID)
	}
	w.emitGitChanged(p.ID, path)
	w.maybeAutoCleanup(p.ID)
}

func (w *Work) SessionChanged(session protocol.SessionInfo) {
	if session.Task == "" {
		return
	}
	w.mu.Lock()
	p, index := w.findTaskLocked(session.Task)
	if p == nil {
		w.mu.Unlock()
		return
	}
	next := ""
	switch session.State {
	case protocol.StateWorking:
		next = protocol.TaskWorking
	case protocol.StateWaiting, protocol.StateFailed:
		next = protocol.TaskNeedsYou
	}
	if next != "" && p.Tasks[index].Status != protocol.TaskDone && p.Tasks[index].Status != protocol.TaskBlocked {
		p.Tasks[index].Status = next
		p.Tasks[index].Updated = time.Now()
		recomputeTaskReadiness(p)
		_ = w.saveLocked()
	}
	project := p.ID
	w.mu.Unlock()
	if next != "" {
		w.broadcastTasks(project)
	}
}

func (w *Work) RevertTurn(ctx context.Context, session protocol.SessionInfo, turn int) (protocol.GitOpResult, error) {
	path, err := sessionWorktreeTop(ctx, session)
	if err != nil {
		return protocol.GitOpResult{}, badRequest("The session has no Git worktree.")
	}
	w.mu.RLock()
	sessions := w.sessions
	w.mu.RUnlock()
	if sessions != nil {
		for _, current := range sessions.List(protocol.KindAgent) {
			currentPath, topErr := sessionWorktreeTop(ctx, current)
			if topErr == nil && samePath(currentPath, path) && current.State == protocol.StateWorking {
				return protocol.GitOpResult{}, conflict("Files cannot be reverted while a session is working in this worktree.")
			}
		}
	}
	p, _, err := w.projectForPath(ctx, path)
	if err != nil {
		return protocol.GitOpResult{}, err
	}
	post := checkpointPostRef(session.ID, turn)
	if _, err := gitpkg.Run(ctx, path, "rev-parse", "--verify", post+"^{commit}"); err != nil {
		return protocol.GitOpResult{}, conflict("The turn cannot be reverted because its completed snapshot is unavailable.")
	}
	current := "refs/rashin/tmp/" + newID("revert")
	if _, err := gitpkg.Checkpoint(ctx, path, current); err != nil {
		return protocol.GitOpResult{}, unavailable("The current worktree snapshot could not be created: %v.", err)
	}
	defer gitpkg.DeleteRefs(ctx, path, current)
	changedOutput, err := gitpkg.Run(ctx, path, "diff", "--name-only", "-z", post, current, "--")
	if err != nil {
		return protocol.GitOpResult{}, unavailable("The worktree could not be checked for newer changes: %v.", err)
	}
	if changed := nulPaths(changedOutput); len(changed) > 0 {
		return protocol.GitOpResult{}, conflict("Files changed after this turn: %s.", strings.Join(changed, ", "))
	}
	err = w.withActorMutation(p.ID, "you", "checkpoint.restore", path, func() error {
		return gitpkg.Restore(ctx, path, checkpointRef(session.ID, turn))
	})
	if err != nil {
		return protocol.GitOpResult{}, conflict("The turn checkpoint could not be restored: %v.", err)
	}
	w.emitGitChanged(p.ID, path)
	return protocol.GitOpResult{OK: true}, nil
}

func nulPaths(data []byte) []string {
	if len(data) == 0 {
		return nil
	}
	parts := strings.Split(strings.TrimSuffix(string(data), "\x00"), "\x00")
	result := parts[:0]
	for _, part := range parts {
		if part != "" {
			result = append(result, part)
		}
	}
	return result
}

func (w *Work) Watch(project string, on bool) {
	if on {
		w.mu.Lock()
		if w.closed {
			w.mu.Unlock()
			return
		}
		w.watchRef[project]++
		alreadyWatching := w.watchRef[project] > 1 || w.watchers[project] != nil
		currentProject := w.projects[project]
		root, isGit := "", false
		if currentProject != nil {
			root, isGit = currentProject.Root, currentProject.Git
		}
		w.mu.Unlock()
		w.maybeAutoCleanup(project)
		if alreadyWatching || !isGit {
			return
		}
		current, err := w.startWatcher(project, root)
		if err != nil {
			return
		}
		w.mu.Lock()
		if w.closed || w.watchRef[project] == 0 {
			w.mu.Unlock()
			stopWatcher(current)
			return
		}
		w.watchers[project] = current
		w.mu.Unlock()
		return
	}
	w.mu.Lock()
	if w.watchRef[project] > 0 {
		w.watchRef[project]--
	}
	if w.watchRef[project] > 0 {
		w.mu.Unlock()
		return
	}
	delete(w.watchRef, project)
	current := w.watchers[project]
	delete(w.watchers, project)
	w.mu.Unlock()
	if current != nil {
		stopWatcher(current)
	}
}

func (w *Work) maybeAutoCleanup(project string) {
	now := time.Now()
	w.mu.Lock()
	p := w.projects[project]
	if w.closed || p == nil || !p.Settings.Cleanup.Auto || now.Sub(w.cleanupLast[project]) < time.Hour {
		w.mu.Unlock()
		return
	}
	w.cleanupLast[project] = now
	ctx := w.background
	w.backgroundWG.Add(1)
	w.mu.Unlock()
	go func() {
		defer w.backgroundWG.Done()
		_, _ = w.cleanupWorktreesAs(ctx, protocol.CleanupParams{Project: project}, "Rashin")
	}()
}

func (w *Work) startWatcher(project, root string) (*watcher, error) {
	repo, err := gitpkg.Open(context.Background(), root)
	if err != nil {
		return nil, err
	}
	fd, err := syscall.InotifyInit1(syscall.IN_CLOEXEC | syscall.IN_NONBLOCK)
	if err != nil {
		return nil, err
	}
	file := os.NewFile(uintptr(fd), "rashin-inotify-"+project)
	if file == nil {
		_ = syscall.Close(fd)
		return nil, os.ErrInvalid
	}
	current := &watcher{file: file, stop: make(chan struct{}), readDone: make(chan struct{}), done: make(chan struct{})}
	const watchLimit = 8192
	mask := uint32(syscall.IN_MODIFY | syscall.IN_CLOSE_WRITE | syscall.IN_MOVED_TO | syscall.IN_MOVED_FROM | syscall.IN_CREATE | syscall.IN_DELETE | syscall.IN_ATTRIB | syscall.IN_DELETE_SELF | syscall.IN_MOVE_SELF)
	watched := make(map[string]int)
	byDescriptor := make(map[int]string)
	capLogged := false
	addPath := func(path string) {
		path = filepath.Clean(path)
		if _, exists := watched[path]; exists {
			return
		}
		if _, statErr := os.Lstat(path); statErr != nil {
			return
		}
		if len(watched) >= watchLimit {
			if !capLogged {
				log.Print(i18n.Tf("Rashin workspace reached the %d-watch limit for project %s.", watchLimit, project))
				capLogged = true
			}
			return
		}
		descriptor, addErr := syscall.InotifyAddWatch(fd, path, mask)
		if addErr != nil {
			return
		}
		watched[path] = descriptor
		byDescriptor[descriptor] = path
	}
	skipDirectory := func(name string) bool {
		switch name {
		case ".git", "node_modules", "target", "build", ".cache", "dist", "vendor":
			return true
		default:
			return false
		}
	}
	var addTree func(string)
	addTree = func(treeRoot string) {
		_ = filepath.WalkDir(treeRoot, func(path string, entry os.DirEntry, walkErr error) error {
			if walkErr != nil {
				return nil
			}
			if !entry.IsDir() {
				return nil
			}
			if path != treeRoot && skipDirectory(entry.Name()) {
				return filepath.SkipDir
			}
			addPath(path)
			if len(watched) >= watchLimit {
				return filepath.SkipDir
			}
			return nil
		})
	}
	refs := filepath.Join(repo.CommonDir, "refs")
	admin := filepath.Join(repo.CommonDir, "worktrees")
	refreshWatches := func() {
		addPath(repo.CommonDir)
		addPath(filepath.Join(repo.CommonDir, "HEAD"))
		addPath(filepath.Join(repo.CommonDir, "index"))
		_ = filepath.WalkDir(refs, func(path string, entry os.DirEntry, walkErr error) error {
			if walkErr == nil && entry.IsDir() {
				addPath(path)
			}
			return nil
		})
		entries, _ := os.ReadDir(admin)
		for _, entry := range entries {
			if !entry.IsDir() {
				continue
			}
			addPath(filepath.Join(admin, entry.Name(), "HEAD"))
			addPath(filepath.Join(admin, entry.Name(), "index"))
		}
		if worktrees, listErr := repo.Worktrees(context.Background()); listErr == nil {
			for _, item := range worktrees {
				if !item.Missing {
					addTree(item.Path)
				}
			}
		}
	}
	refreshWatches()
	changes := make(chan struct{}, 1)
	go func() {
		defer close(current.readDone)
		defer close(changes)
		buffer := make([]byte, 64<<10)
		for {
			count, readErr := file.Read(buffer)
			if readErr != nil {
				return
			}
			current.ioMu.Lock()
			select {
			case <-current.stop:
				current.ioMu.Unlock()
				return
			default:
			}
			for offset := 0; offset+syscall.SizeofInotifyEvent <= count; {
				record := buffer[offset:]
				descriptor := int(int32(binary.LittleEndian.Uint32(record[0:4])))
				eventMask := binary.LittleEndian.Uint32(record[4:8])
				nameLength := int(binary.LittleEndian.Uint32(record[12:16]))
				recordSize := syscall.SizeofInotifyEvent + nameLength
				if offset+recordSize > count {
					break
				}
				base := byDescriptor[descriptor]
				if eventMask&syscall.IN_IGNORED != 0 {
					delete(watched, base)
					delete(byDescriptor, descriptor)
				}
				if eventMask&syscall.IN_ISDIR != 0 && eventMask&(syscall.IN_CREATE|syscall.IN_MOVED_TO) != 0 && nameLength > 0 {
					name := strings.TrimRight(string(record[syscall.SizeofInotifyEvent:recordSize]), "\x00")
					if name != "" && !skipDirectory(name) {
						addTree(filepath.Join(base, name))
					}
				}
				offset += recordSize
			}
			current.ioMu.Unlock()
			select {
			case changes <- struct{}{}:
			default:
			}
		}
	}()
	go func() {
		defer func() { <-current.readDone; close(current.done) }()
		var timer *time.Timer
		var timerC <-chan time.Time
		for {
			select {
			case <-current.stop:
				if timer != nil {
					timer.Stop()
				}
				return
			case _, ok := <-changes:
				if !ok {
					if timer != nil {
						timer.Stop()
					}
					return
				}
				if timer == nil {
					timer = time.NewTimer(300 * time.Millisecond)
				} else {
					if !timer.Stop() {
						select {
						case <-timer.C:
						default:
						}
					}
					timer.Reset(300 * time.Millisecond)
				}
				timerC = timer.C
			case <-timerC:
				timerC = nil
				current.ioMu.Lock()
				select {
				case <-current.stop:
					current.ioMu.Unlock()
					return
				default:
				}
				refreshWatches()
				current.ioMu.Unlock()
				w.broadcast(protocol.EvtGitChanged, protocol.GitChanged{Project: project})
			}
		}
	}()
	return current, nil
}

func stopWatcher(current *watcher) {
	close(current.stop)
	current.ioMu.Lock()
	_ = current.file.Close()
	current.ioMu.Unlock()
	<-current.done
}
