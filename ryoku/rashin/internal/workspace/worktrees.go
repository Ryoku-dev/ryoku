package workspace

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	i18n "ryoku-i18n"
	gitpkg "ryoku-rashin/internal/git"
	"ryoku-rashin/internal/paths"
	"ryoku-rashin/internal/protocol"
)

var slugPart = regexp.MustCompile(`[^a-z0-9]+`)

func slug(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	value = slugPart.ReplaceAllString(value, "-")
	value = strings.Trim(value, "-")
	if value == "" {
		return "work"
	}
	return value
}

func (w *Work) listWorktrees(ctx context.Context, project string, refresh bool) (protocol.WorktreesResult, error) {
	w.mu.RLock()
	p, err := w.projectLocked(project)
	if err != nil {
		w.mu.RUnlock()
		return protocol.WorktreesResult{}, err
	}
	copy := *p
	copy.Worktrees = append([]ownedWorktree(nil), p.Worktrees...)
	sessions := w.sessions
	w.mu.RUnlock()
	if !copy.Git {
		return protocol.WorktreesResult{Project: project, Worktrees: []protocol.WorktreeInfo{}}, nil
	}
	repo, err := gitpkg.Open(ctx, copy.Root)
	if err != nil {
		return protocol.WorktreesResult{}, unavailable("The project repository is unavailable: %v.", err)
	}
	gitWorktrees, err := repo.Worktrees(ctx)
	if err != nil {
		return protocol.WorktreesResult{}, unavailable("The project worktrees cannot be listed: %v.", err)
	}
	base, err := resolveBase(ctx, &copy, repo, gitWorktrees)
	if err != nil {
		return protocol.WorktreesResult{}, err
	}
	owned := make(map[string]ownedWorktree, len(copy.Worktrees))
	for _, record := range copy.Worktrees {
		owned[filepath.Clean(record.Path)] = record
	}
	sessionMap := make(map[string][]string)
	if sessions != nil {
		for _, session := range sessions.List(protocol.KindAgent) {
			if session.Project == project && session.Worktree != "" {
				sessionMap[filepath.Clean(session.Worktree)] = append(sessionMap[filepath.Clean(session.Worktree)], session.ID)
			}
		}
	}
	result := protocol.WorktreesResult{Project: project, Worktrees: make([]protocol.WorktreeInfo, 0, len(gitWorktrees))}
	seen := make(map[string]bool, len(gitWorktrees))
	for _, current := range gitWorktrees {
		seen[filepath.Clean(current.Path)] = true
		item := protocol.WorktreeInfo{
			Path: current.Path, Project: project, Name: filepath.Base(current.Path), Branch: current.Branch,
			Head: current.Head, Detached: current.Detached, Main: current.Main, Locked: current.Locked,
			LockReason: current.LockReason, Prunable: current.Prunable, Missing: current.Missing,
			Sessions: append([]string(nil), sessionMap[filepath.Clean(current.Path)]...), Setup: protocol.SetupNone,
		}
		statusBase := base
		if record, ok := owned[filepath.Clean(current.Path)]; ok {
			item.Owned = true
			item.Base = record.Base
			item.BaseSha = record.BaseSHA
			item.Parent = record.Parent
			item.Task = record.Task
			item.Setup = record.Setup.State
			created, activity := record.Created, record.Activity
			item.Created, item.Activity = &created, &activity
			if record.Base != "" {
				statusBase = record.Base
			}
		}
		if !current.Missing {
			if status, statusErr := w.cachedStatus(ctx, current.Path, statusBase, refresh); statusErr == nil {
				value := status.Status
				item.Status = &value
			}
		}
		result.Worktrees = append(result.Worktrees, item)
	}
	for key, record := range owned {
		if seen[key] {
			continue
		}
		created, activity := record.Created, record.Activity
		setup := record.Setup.State
		if setup == "" {
			setup = protocol.SetupNone
		}
		result.Worktrees = append(result.Worktrees, protocol.WorktreeInfo{
			Path: record.Path, Project: project, Name: filepath.Base(record.Path), Owned: true, Missing: true,
			Base: record.Base, BaseSha: record.BaseSHA, Parent: record.Parent, Task: record.Task,
			Sessions: append([]string(nil), sessionMap[key]...), Setup: setup,
			Created: &created, Activity: &activity,
		})
	}
	return result, nil
}

func (w *Work) cachedStatus(ctx context.Context, path, base string, refresh bool) (protocol.GitStatusResult, error) {
	porcelain, err := gitpkg.Run(ctx, path, "status", "--porcelain=v2", "-z", "--branch", "--untracked-files=normal")
	if err != nil {
		return protocol.GitStatusResult{}, err
	}
	porcelainHash := sha256.Sum256(porcelain)
	operation := gitpkg.Operation(ctx, path)
	baseSHA := ""
	if base != "" {
		if output, resolveErr := gitpkg.Run(ctx, path, "rev-parse", "--verify", base+"^{commit}"); resolveErr == nil {
			baseSHA = strings.TrimSpace(string(output))
		}
	}
	w.mu.RLock()
	cached, ok := w.status[filepath.Clean(path)]
	w.mu.RUnlock()
	if ok && cached.porcelain == porcelainHash && cached.operation == operation && cached.base == base && cached.baseSHA == baseSHA {
		return cached.result, nil
	}
	result, err := gitpkg.Status(ctx, path, base)
	if err != nil {
		return protocol.GitStatusResult{}, err
	}
	w.mu.Lock()
	w.status[filepath.Clean(path)] = statusCache{
		porcelain: porcelainHash, operation: operation, base: base, baseSHA: baseSHA, result: result,
	}
	w.mu.Unlock()
	_ = refresh
	return result, nil
}

func (w *Work) createWorktree(ctx context.Context, params protocol.WorktreeCreateParams, actor string) (protocol.WorktreeInfo, error) {
	w.mu.RLock()
	p, err := w.projectLocked(params.Project)
	if err != nil {
		w.mu.RUnlock()
		return protocol.WorktreeInfo{}, err
	}
	copy := *p
	copy.Worktrees = append([]ownedWorktree(nil), p.Worktrees...)
	w.mu.RUnlock()
	if !copy.Git {
		return protocol.WorktreeInfo{}, badRequest("Worktrees require a Git project.")
	}
	if strings.TrimSpace(params.Name) == "" {
		return protocol.WorktreeInfo{}, badRequest("A worktree name is required.")
	}
	repo, err := gitpkg.Open(ctx, copy.Root)
	if err != nil {
		return protocol.WorktreeInfo{}, unavailable("The project repository is unavailable: %v.", err)
	}
	listed, err := repo.Worktrees(ctx)
	if err != nil {
		return protocol.WorktreeInfo{}, err
	}
	base := params.Base
	if base == "" {
		base, err = resolveBase(ctx, &copy, repo, listed)
		if err != nil {
			return protocol.WorktreeInfo{}, err
		}
	}
	if _, err := gitpkg.Run(ctx, copy.Root, "rev-parse", "--verify", base+"^{commit}"); err != nil {
		return protocol.WorktreeInfo{}, badRequest("The worktree base cannot be resolved.")
	}
	name := slug(params.Name)
	root := expandHome(copy.Settings.WorktreeRoot)
	if root == "" {
		root = filepath.Join(paths.WorktreesDir(), slug(copy.Name))
	}
	path := filepath.Join(root, name)
	for suffix := 2; pathExists(path); suffix++ {
		path = filepath.Join(root, name+"-"+strconv.Itoa(suffix))
	}
	mode := params.Mode
	if mode == "" {
		mode = protocol.WorktreeNewBranch
	}
	if mode != protocol.WorktreeNewBranch && mode != protocol.WorktreeExistingBranch && mode != protocol.WorktreeDetached {
		return protocol.WorktreeInfo{}, badRequest("The worktree mode is invalid.")
	}
	branch := params.Branch
	if mode == protocol.WorktreeNewBranch && branch == "" {
		prefix := copy.Settings.BranchPrefix
		if prefix == "" {
			prefix = "rashin/"
		}
		branch = prefix + name
		for suffix := 2; refExists(ctx, copy.Root, "refs/heads/"+branch); suffix++ {
			branch = prefix + name + "-" + strconv.Itoa(suffix)
		}
	}
	if mode == protocol.WorktreeExistingBranch && branch == "" {
		return protocol.WorktreeInfo{}, badRequest("An existing branch is required.")
	}
	baseSHABytes, err := gitpkg.Run(ctx, copy.Root, "rev-parse", base+"^{commit}")
	if err != nil {
		return protocol.WorktreeInfo{}, err
	}
	options := gitpkg.AddOptions{Path: path, Mode: mode, Branch: branch, Base: base, Lock: params.Lock}
	if params.Lock {
		options.LockReason = i18n.T("Created by Rashin")
	}
	w.busy.Add(1)
	err = repo.AddWorktree(ctx, options)
	w.busy.Add(-1)
	if err != nil {
		w.recordActivity(copy.ID, actor, "worktree.create", path, false, err.Error())
		return protocol.WorktreeInfo{}, conflict("The worktree could not be created: %v.", err)
	}
	now := time.Now()
	record := ownedWorktree{
		Path: path, Base: base, BaseSHA: strings.TrimSpace(string(baseSHABytes)), Parent: params.Parent,
		Task: params.Task, Setup: protocol.SetupResult{Path: path, State: protocol.SetupNone}, Created: now, Activity: now,
	}
	w.mu.Lock()
	if current := w.projects[copy.ID]; current != nil {
		current.Worktrees = append(current.Worktrees, record)
		current.Activity = now
		err = w.saveLocked()
	}
	w.mu.Unlock()
	if err != nil {
		return protocol.WorktreeInfo{}, err
	}
	mainProject := copy
	mainProject.Root = repo.Root
	if err := w.copyIgnored(ctx, &mainProject, path); err != nil {
		w.recordActivity(copy.ID, actor, "worktree.create", path, false, err.Error())
		return protocol.WorktreeInfo{}, err
	}
	if params.Setup {
		if _, err := w.setupWorktree(ctx, path, ""); err != nil {
			return protocol.WorktreeInfo{}, err
		}
	}
	w.recordActivity(copy.ID, actor, "worktree.create", path, true, "")
	result, err := w.listWorktrees(ctx, copy.ID, true)
	if err != nil {
		return protocol.WorktreeInfo{}, err
	}
	w.broadcast(protocol.EvtWorktreesChanged, result)
	for _, item := range result.Worktrees {
		if samePath(item.Path, path) {
			return item, nil
		}
	}
	return protocol.WorktreeInfo{}, protocol.Errorf(protocol.CodeNotFound, "%s", i18n.T("The new worktree was not found after creation."))
}

func pathExists(path string) bool {
	_, err := os.Lstat(path)
	return err == nil || !errors.Is(err, os.ErrNotExist)
}

func refExists(ctx context.Context, root, ref string) bool {
	out, err := gitpkg.Run(ctx, root, "rev-parse", "--verify", "--quiet", ref)
	return err == nil && len(bytes.TrimSpace(out)) > 0
}

const copyIgnoredLimit int64 = 256 << 20

type copyBudget struct {
	remaining int64
}

func (b *copyBudget) reserve(size int64) error {
	if size < 0 || size > b.remaining {
		return fmt.Errorf("%s", i18n.T("Ignored files exceed the 256 MiB copy limit."))
	}
	b.remaining -= size
	return nil
}

type copiedDirectory struct {
	path string
	mode os.FileMode
}

func (w *Work) copyIgnored(ctx context.Context, project *projectRecord, target string) error {
	budget := &copyBudget{remaining: copyIgnoredLimit}
	for _, configured := range project.Settings.CopyIgnored {
		relative := filepath.Clean(configured)
		if relative == "." || filepath.IsAbs(relative) || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
			return badRequest("The ignored copy path %s must stay inside the project.", configured)
		}
		ignored, err := gitpkg.Run(ctx, project.Root, "ls-files", "--others", "--ignored", "--exclude-standard", "-z", "--", filepath.ToSlash(relative))
		if err != nil {
			return unavailable("The ignored path %s could not be checked: %v.", configured, err)
		}
		if len(ignored) == 0 {
			continue
		}
		source := filepath.Join(project.Root, relative)
		destination := filepath.Join(target, relative)
		if err := copyPath(source, destination, budget); err != nil && !errors.Is(err, os.ErrNotExist) {
			return unavailable("The ignored path %s could not be copied: %v.", configured, err)
		}
	}
	return nil
}

func copyPath(source, destination string, budget *copyBudget) error {
	info, err := os.Lstat(source)
	if err != nil {
		return err
	}
	if info.Mode()&os.ModeSymlink != 0 {
		target, err := os.Readlink(source)
		if err != nil {
			return err
		}
		if err := os.MkdirAll(filepath.Dir(destination), 0o700); err != nil {
			return err
		}
		return os.Symlink(target, destination)
	}
	if !info.IsDir() {
		if err := budget.reserve(info.Size()); err != nil {
			return err
		}
		return copyFile(source, destination, info.Mode(), info.Size())
	}
	directories := make([]copiedDirectory, 0, 16)
	err = filepath.WalkDir(source, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		relative, err := filepath.Rel(source, path)
		if err != nil {
			return err
		}
		targetPath := filepath.Join(destination, relative)
		info, err := entry.Info()
		if err != nil {
			return err
		}
		if entry.IsDir() {
			if err := os.MkdirAll(targetPath, 0o700); err != nil {
				return err
			}
			directories = append(directories, copiedDirectory{path: targetPath, mode: info.Mode()})
			return nil
		}
		if info.Mode()&os.ModeSymlink != 0 {
			link, err := os.Readlink(path)
			if err != nil {
				return err
			}
			if err := os.MkdirAll(filepath.Dir(targetPath), 0o700); err != nil {
				return err
			}
			return os.Symlink(link, targetPath)
		}
		if err := budget.reserve(info.Size()); err != nil {
			return err
		}
		return copyFile(path, targetPath, info.Mode(), info.Size())
	})
	if err != nil {
		return err
	}
	for i := len(directories) - 1; i >= 0; i-- {
		if err := os.Chmod(directories[i].path, directories[i].mode.Perm()); err != nil {
			return err
		}
	}
	return nil
}

func copyFile(source, destination string, mode os.FileMode, size int64) error {
	input, err := os.Open(source)
	if err != nil {
		return err
	}
	defer input.Close()
	if err := os.MkdirAll(filepath.Dir(destination), 0o700); err != nil {
		return err
	}
	output, err := os.OpenFile(destination, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	_, copyErr := io.CopyN(output, input, size)
	closeErr := output.Close()
	if copyErr != nil {
		return copyErr
	}
	if closeErr != nil {
		return closeErr
	}
	return os.Chmod(destination, mode.Perm())
}

func (w *Work) previewRemove(ctx context.Context, path string) (protocol.RemovePreview, error) {
	p, repo, err := w.projectForPath(ctx, path)
	if err != nil {
		return protocol.RemovePreview{}, err
	}
	listed, err := repo.Worktrees(ctx)
	if err != nil {
		return protocol.RemovePreview{}, err
	}
	var selected *gitpkg.Worktree
	for i := range listed {
		if samePath(listed[i].Path, path) {
			selected = &listed[i]
			break
		}
	}
	if selected == nil {
		return protocol.RemovePreview{}, protocol.Errorf(protocol.CodeNotFound, "%s", i18n.T("The worktree does not exist."))
	}
	base, err := resolveBase(ctx, p, repo, listed)
	if err != nil {
		return protocol.RemovePreview{}, err
	}
	w.mu.RLock()
	for _, owned := range p.Worktrees {
		if samePath(owned.Path, path) && owned.Base != "" {
			base = owned.Base
			break
		}
	}
	sessions := w.sessions
	w.mu.RUnlock()
	preview, err := repo.RemovePreview(ctx, *selected, base)
	if err != nil {
		return protocol.RemovePreview{}, err
	}
	if sessions != nil {
		for _, session := range sessions.List(protocol.KindAgent) {
			if samePath(session.Worktree, path) && (session.Alive || session.State == protocol.StateWorking) {
				preview.Running = append(preview.Running, session.ID)
			}
		}
	}
	if len(preview.Running) > 0 {
		preview.CanRemove = false
		preview.BlockReason = i18n.T("A session is running in this worktree.")
	}
	if preview.Main {
		preview.CanRemove = false
		preview.BlockReason = i18n.T("The main worktree cannot be removed.")
	}
	return preview, nil
}

func (w *Work) removeWorktree(ctx context.Context, params protocol.WorktreeRemoveParams, actor string) error {
	preview, err := w.previewRemove(ctx, params.Path)
	if err != nil {
		return err
	}
	if preview.Main {
		return conflict("The main worktree cannot be removed.")
	}
	if len(preview.Running) > 0 {
		return conflict("A session is running in this worktree.")
	}
	if preview.NeedsForce && !params.Force {
		return conflict("The worktree needs force because removal would lose work.")
	}
	p, repo, err := w.projectForPath(ctx, params.Path)
	if err != nil {
		return err
	}
	force := 0
	if params.Force {
		force = 1
		if preview.Locked {
			force = 2
		}
	}
	err = w.withActorMutation(p.ID, actor, "worktree.remove", params.Path, func() error {
		return repo.RemoveWorktree(ctx, params.Path, force)
	})
	if err != nil {
		return conflict("The worktree could not be removed: %v.", err)
	}
	var branchErr error
	if params.DeleteBranch && preview.Branch != "" {
		branchErr = w.withActorMutation(p.ID, actor, "git.branch_delete", preview.Branch, func() error {
			return repo.DeleteBranch(ctx, preview.Branch, params.ForceBranch)
		})
	}
	w.mu.Lock()
	if current := w.projects[p.ID]; current != nil {
		kept := current.Worktrees[:0]
		for _, record := range current.Worktrees {
			if !samePath(record.Path, params.Path) {
				kept = append(kept, record)
			}
		}
		current.Worktrees = kept
		_ = w.saveLocked()
	}
	delete(w.status, filepath.Clean(params.Path))
	w.mu.Unlock()
	if listed, listErr := w.listWorktrees(ctx, p.ID, true); listErr == nil {
		w.broadcast(protocol.EvtWorktreesChanged, listed)
	}
	if branchErr != nil {
		return conflict("The worktree was removed but its branch could not be deleted: %v.", branchErr)
	}
	return nil
}

func (w *Work) withActorMutation(project, actor, action, target string, fn func() error) error {
	w.busy.Add(1)
	defer w.busy.Add(-1)
	err := fn()
	w.recordActivity(project, actor, action, target, err == nil, errorDetail(err))
	return err
}

func (w *Work) lockWorktree(ctx context.Context, params protocol.WorktreeLockParams) (protocol.WorktreeInfo, error) {
	p, repo, err := w.projectForPath(ctx, params.Path)
	if err != nil {
		return protocol.WorktreeInfo{}, err
	}
	if err := w.withMutation(p.ID, "worktree.lock", params.Path, func() error {
		return repo.LockWorktree(ctx, params.Path, params.Lock, params.Reason)
	}); err != nil {
		return protocol.WorktreeInfo{}, conflict("The worktree lock could not be changed: %v.", err)
	}
	return w.findWorktree(ctx, p.ID, params.Path, true)
}

func (w *Work) moveWorktree(ctx context.Context, params protocol.WorktreeMoveParams) (protocol.WorktreeInfo, error) {
	if !filepath.IsAbs(params.To) {
		return protocol.WorktreeInfo{}, badRequest("The new worktree path must be absolute.")
	}
	p, repo, err := w.projectForPath(ctx, params.Path)
	if err != nil {
		return protocol.WorktreeInfo{}, err
	}
	if err := w.withMutation(p.ID, "worktree.move", params.Path, func() error {
		return repo.MoveWorktree(ctx, params.Path, params.To)
	}); err != nil {
		return protocol.WorktreeInfo{}, conflict("The worktree could not be moved: %v.", err)
	}
	w.mu.Lock()
	if current := w.projects[p.ID]; current != nil {
		for i := range current.Worktrees {
			if samePath(current.Worktrees[i].Path, params.Path) {
				current.Worktrees[i].Path = params.To
			}
		}
		for i := range current.Worktrees {
			if samePath(current.Worktrees[i].Parent, params.Path) {
				current.Worktrees[i].Parent = params.To
			}
		}
		for i := range current.Tasks {
			if samePath(current.Tasks[i].Worktree, params.Path) {
				current.Tasks[i].Worktree = params.To
			}
		}
		for i := range current.Comments {
			if samePath(current.Comments[i].Comment.Worktree, params.Path) {
				current.Comments[i].Comment.Worktree = params.To
			}
		}
		_ = w.saveLocked()
	}
	delete(w.status, filepath.Clean(params.Path))
	w.mu.Unlock()
	return w.findWorktree(ctx, p.ID, params.To, true)
}

func (w *Work) findWorktree(ctx context.Context, project, path string, refresh bool) (protocol.WorktreeInfo, error) {
	listed, err := w.listWorktrees(ctx, project, refresh)
	if err != nil {
		return protocol.WorktreeInfo{}, err
	}
	for _, item := range listed.Worktrees {
		if samePath(item.Path, path) {
			w.broadcast(protocol.EvtWorktreesChanged, listed)
			return item, nil
		}
	}
	return protocol.WorktreeInfo{}, protocol.Errorf(protocol.CodeNotFound, "%s", i18n.T("The worktree does not exist."))
}

func (w *Work) repairWorktrees(ctx context.Context, project string) (protocol.WorktreesResult, error) {
	p, err := w.project(project)
	if err != nil {
		return protocol.WorktreesResult{}, err
	}
	repo, err := gitpkg.Open(ctx, p.Root)
	if err != nil {
		return protocol.WorktreesResult{}, err
	}
	moved := w.discoverMovedWorktrees(repo, p)
	err = w.withMutation(project, "worktree.repair", p.Root, func() error {
		if len(moved) == 0 {
			return repo.RepairWorktrees(ctx)
		}
		paths := make([]string, 0, len(moved))
		for _, next := range moved {
			paths = append(paths, next)
		}
		_, runErr := gitpkg.Run(ctx, p.Root, append([]string{"worktree", "repair"}, paths...)...)
		return runErr
	})
	if err != nil {
		return protocol.WorktreesResult{}, err
	}
	if len(moved) > 0 {
		w.mu.Lock()
		if current := w.projects[project]; current != nil {
			for old, next := range moved {
				replaceOwnedPath(current, old, next)
			}
			err = w.saveLocked()
		}
		w.mu.Unlock()
		if err != nil {
			return protocol.WorktreesResult{}, err
		}
	}
	result, err := w.listWorktrees(ctx, project, true)
	if err == nil {
		w.broadcast(protocol.EvtWorktreesChanged, result)
	}
	return result, err
}

func (w *Work) discoverMovedWorktrees(repo *gitpkg.Repo, p *projectRecord) map[string]string {
	w.mu.RLock()
	current := w.projects[p.ID]
	var owned []ownedWorktree
	settings := p.Settings
	if current != nil {
		owned = append(owned, current.Worktrees...)
		settings = current.Settings
	}
	w.mu.RUnlock()
	adminTargets := make(map[string]string)
	entries, _ := os.ReadDir(filepath.Join(repo.CommonDir, "worktrees"))
	for _, entry := range entries {
		content, readErr := os.ReadFile(filepath.Join(repo.CommonDir, "worktrees", entry.Name(), "gitdir"))
		if readErr == nil {
			adminTargets[entry.Name()] = strings.TrimSpace(string(content))
		}
	}
	adminToOld := make(map[string]string)
	for _, item := range owned {
		if _, err := os.Lstat(item.Path); !errors.Is(err, os.ErrNotExist) {
			continue
		}
		for admin, target := range adminTargets {
			if samePath(target, filepath.Join(item.Path, ".git")) {
				adminToOld[admin] = item.Path
			}
		}
	}
	if len(adminToOld) == 0 {
		return nil
	}
	root := expandHome(settings.WorktreeRoot)
	if root == "" {
		root = filepath.Join(paths.WorktreesDir(), slug(p.Name))
	}
	moved := make(map[string]string)
	directories := 0
	_ = filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return nil
		}
		if !entry.IsDir() {
			return nil
		}
		directories++
		if directories > 4096 {
			return fs.SkipAll
		}
		if path != root {
			switch entry.Name() {
			case ".git", "node_modules", "target", "build":
				return fs.SkipDir
			}
		}
		content, readErr := os.ReadFile(filepath.Join(path, ".git"))
		if readErr != nil {
			return nil
		}
		line := strings.TrimSpace(string(content))
		if !strings.HasPrefix(line, "gitdir:") {
			return nil
		}
		admin := strings.TrimSpace(strings.TrimPrefix(line, "gitdir:"))
		if !filepath.IsAbs(admin) {
			admin = filepath.Join(path, admin)
		}
		if old := adminToOld[filepath.Base(filepath.Clean(admin))]; old != "" && !samePath(old, path) {
			moved[old] = path
			return fs.SkipDir
		}
		return nil
	})
	return moved
}

func replaceOwnedPath(p *projectRecord, old, next string) {
	for i := range p.Worktrees {
		if samePath(p.Worktrees[i].Path, old) {
			p.Worktrees[i].Path = next
		}
		if samePath(p.Worktrees[i].Parent, old) {
			p.Worktrees[i].Parent = next
		}
	}
	for i := range p.Tasks {
		if samePath(p.Tasks[i].Worktree, old) {
			p.Tasks[i].Worktree = next
		}
	}
	for i := range p.Comments {
		if samePath(p.Comments[i].Comment.Worktree, old) {
			p.Comments[i].Comment.Worktree = next
		}
	}
}

func (w *Work) pruneWorktrees(ctx context.Context, params protocol.PruneParams) (protocol.PruneResult, error) {
	p, err := w.project(params.Project)
	if err != nil {
		return protocol.PruneResult{}, err
	}
	repo, err := gitpkg.Open(ctx, p.Root)
	if err != nil {
		return protocol.PruneResult{}, err
	}
	w.mu.RLock()
	current := w.projects[p.ID]
	var owned []ownedWorktree
	if current != nil {
		owned = append(owned, current.Worktrees...)
	}
	w.mu.RUnlock()
	listed, _ := repo.Worktrees(ctx)
	pruned := make([]string, 0)
	for _, item := range owned {
		missing := false
		if _, statErr := os.Lstat(item.Path); errors.Is(statErr, os.ErrNotExist) {
			missing = true
		}
		for _, gitItem := range listed {
			if samePath(gitItem.Path, item.Path) && (gitItem.Missing || gitItem.Prunable) {
				missing = true
			}
		}
		if missing {
			pruned = appendUniquePath(pruned, item.Path)
		}
	}
	err = w.withMutation(p.ID, "worktree.prune", p.Root, func() error {
		reported, pruneErr := repo.PruneWorktrees(ctx, params.DryRun)
		for _, path := range reported {
			pruned = appendUniquePath(pruned, path)
		}
		return pruneErr
	})
	if err != nil {
		return protocol.PruneResult{}, err
	}
	if !params.DryRun {
		w.mu.Lock()
		if current := w.projects[p.ID]; current != nil {
			kept := current.Worktrees[:0]
			for _, item := range current.Worktrees {
				remove := false
				for _, path := range pruned {
					if samePath(path, item.Path) {
						remove = true
						break
					}
				}
				if !remove {
					kept = append(kept, item)
				}
			}
			current.Worktrees = kept
			err = w.saveLocked()
		}
		w.mu.Unlock()
		if err != nil {
			return protocol.PruneResult{}, err
		}
	}
	if result, listErr := w.listWorktrees(ctx, p.ID, true); listErr == nil {
		w.broadcast(protocol.EvtWorktreesChanged, result)
	}
	return protocol.PruneResult{Pruned: pruned}, nil
}

func appendUniquePath(paths []string, candidate string) []string {
	for _, current := range paths {
		if samePath(current, candidate) {
			return paths
		}
	}
	return append(paths, candidate)
}

func (w *Work) cleanupWorktrees(ctx context.Context, params protocol.CleanupParams) (protocol.CleanupResult, error) {
	return w.cleanupWorktreesAs(ctx, params, "you")
}

func (w *Work) cleanupWorktreesAs(ctx context.Context, params protocol.CleanupParams, actor string) (protocol.CleanupResult, error) {
	w.mu.RLock()
	current, ok := w.projects[params.Project]
	if !ok {
		w.mu.RUnlock()
		return protocol.CleanupResult{}, protocol.Errorf(protocol.CodeNotFound, "%s", i18n.Tf("No project %s.", params.Project))
	}
	project := *current
	project.Worktrees = append([]ownedWorktree(nil), current.Worktrees...)
	w.mu.RUnlock()
	p := &project
	w.busy.Add(1)
	defer w.busy.Add(-1)
	result := protocol.CleanupResult{}
	for _, owned := range p.Worktrees {
		item := protocol.CleanupItem{Path: owned.Path}
		preview, previewErr := w.previewRemove(ctx, owned.Path)
		if previewErr != nil {
			item.Reason = i18n.Tf("Kept because its state could not be inspected: %v.", previewErr)
			result.Items = append(result.Items, item)
			continue
		}
		if len(preview.Running) > 0 {
			item.Reason = i18n.T("Kept because a session is running.")
		} else if preview.Locked {
			item.Reason = i18n.T("Kept because the worktree is locked.")
		} else if preview.Unmerged || len(preview.Unpushed) > 0 {
			item.Reason = i18n.T("Kept because it has commits not safely integrated with the base.")
		} else if len(preview.Dirty) > 0 || (p.Settings.Cleanup.Protect == protocol.ProtectAny && len(preview.Untracked) > 0) {
			item.Reason = i18n.T("Kept because it has protected local changes.")
		} else {
			ageDays := int(time.Since(owned.Activity).Hours() / 24)
			merged := false
			if preview.Branch != "" && owned.Base != "" {
				_, mergedErr := gitpkg.Run(ctx, p.Root, "merge-base", "--is-ancestor", preview.Branch, owned.Base)
				merged = mergedErr == nil
			}
			eligible := false
			if merged && p.Settings.Cleanup.AfterMergeDays > 0 && ageDays >= p.Settings.Cleanup.AfterMergeDays {
				eligible = true
				item.Reason = i18n.T("Eligible because its branch was merged and the retention period passed.")
			} else if p.Settings.Cleanup.InactiveDays > 0 && ageDays >= p.Settings.Cleanup.InactiveDays {
				eligible = true
				item.Reason = i18n.T("Eligible because it was inactive past the retention period.")
			} else {
				item.Reason = i18n.T("Kept because no cleanup rule applies yet.")
			}
			if eligible && !params.DryRun {
				removeErr := w.removeWorktree(ctx, protocol.WorktreeRemoveParams{Path: owned.Path, Force: len(preview.Untracked) > 0}, actor)
				if removeErr != nil {
					item.Reason = i18n.Tf("Kept because removal failed: %v.", removeErr)
				} else {
					item.Removed = true
				}
			}
		}
		result.Items = append(result.Items, item)
	}
	w.recordActivity(p.ID, actor, "worktree.cleanup", p.Root, true, "")
	return result, nil
}

type cappedBuffer struct {
	data []byte
	max  int
}

func (b *cappedBuffer) Write(data []byte) (int, error) {
	remaining := b.max - len(b.data)
	if remaining > 0 {
		if len(data) < remaining {
			remaining = len(data)
		}
		b.data = append(b.data, data[:remaining]...)
	}
	return len(data), nil
}

func (w *Work) setupWorktree(ctx context.Context, path, trustedHash string) (protocol.SetupResult, error) {
	return w.setupWorktreeMode(ctx, path, trustedHash, false)
}

func (w *Work) setupWorktreeMode(ctx context.Context, path, trustedHash string, rerun bool) (protocol.SetupResult, error) {
	p, _, err := w.projectForPath(ctx, path)
	if err != nil {
		return protocol.SetupResult{}, err
	}
	w.mu.RLock()
	current := w.projects[p.ID]
	if current == nil {
		w.mu.RUnlock()
		return protocol.SetupResult{}, protocol.Errorf(protocol.CodeNotFound, "%s", i18n.Tf("No project %s.", p.ID))
	}
	project := *current
	project.Worktrees = append([]ownedWorktree(nil), current.Worktrees...)
	project.Settings.CopyIgnored = append([]string(nil), current.Settings.CopyIgnored...)
	project.Settings.TrustedScripts = append([]string(nil), current.Settings.TrustedScripts...)
	w.mu.RUnlock()
	if !rerun {
		for _, owned := range project.Worktrees {
			if samePath(owned.Path, path) && owned.Setup.State != protocol.SetupNone {
				return owned.Setup, nil
			}
		}
	}
	p = &project
	command := p.Settings.Setup
	var verifiedScript []byte
	result := protocol.SetupResult{Path: path, State: protocol.SetupNone}
	if command != "" {
		result.Command = command
	} else {
		script := filepath.Join(path, ".rashin", "setup")
		content, readErr := os.ReadFile(script)
		if errors.Is(readErr, os.ErrNotExist) {
			w.storeSetup(p.ID, path, result)
			return result, nil
		}
		if readErr != nil {
			return result, unavailable("The setup script cannot be read: %v.", readErr)
		}
		hashBytes := sha256.Sum256(content)
		hash := hex.EncodeToString(hashBytes[:])
		result.Script, result.Hash = script, hash
		trusted := trustedHash == hash
		if !trusted {
			for _, candidate := range p.Settings.TrustedScripts {
				if candidate == hash {
					trusted = true
					break
				}
			}
		}
		if !trusted {
			result.State = protocol.SetupUntrusted
			w.storeSetup(p.ID, path, result)
			return result, nil
		}
		verifiedScript = append([]byte(nil), content...)
		command = "sh -s"
		result.Command = command
	}
	runCtx, cancel := context.WithTimeout(ctx, 10*time.Minute)
	run := &setupRun{cancel: cancel, done: make(chan struct{})}
	key := filepath.Clean(path)
	w.mu.Lock()
	if w.closed {
		w.mu.Unlock()
		cancel()
		close(run.done)
		return result, unavailable("Work is closing.")
	}
	previous := w.setups[key]
	w.setups[key] = run
	w.mu.Unlock()
	if previous != nil {
		previous.cancel()
		<-previous.done
	}
	defer func() {
		cancel()
		w.mu.Lock()
		if w.setups[key] == run {
			delete(w.setups, key)
		}
		w.mu.Unlock()
		close(run.done)
	}()
	result.State = protocol.SetupRunning
	w.storeSetup(p.ID, path, result)
	w.busy.Add(1)
	defer w.busy.Add(-1)
	cmd := exec.CommandContext(runCtx, "sh", "-c", command)
	cmd.Dir = path
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.WaitDelay = 250 * time.Millisecond
	if verifiedScript != nil {
		cmd.Stdin = bytes.NewReader(verifiedScript)
	}
	var cancelMu sync.Mutex
	var killTimer *time.Timer
	cmd.Cancel = func() error {
		cancelMu.Lock()
		defer cancelMu.Unlock()
		if cmd.Process == nil {
			return os.ErrProcessDone
		}
		err := syscall.Kill(-cmd.Process.Pid, syscall.SIGTERM)
		killTimer = time.AfterFunc(250*time.Millisecond, func() {
			_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
		})
		if errors.Is(err, syscall.ESRCH) {
			return os.ErrProcessDone
		}
		return err
	}
	output := &cappedBuffer{max: 256 << 10}
	cmd.Stdout, cmd.Stderr = output, output
	runErr := cmd.Run()
	cancelMu.Lock()
	if killTimer != nil {
		killTimer.Stop()
	}
	if cmd.Process != nil {
		_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
	}
	cancelMu.Unlock()
	if errors.Is(runErr, exec.ErrWaitDelay) {
		runErr = nil
	}
	result.Log = string(output.data)
	exit := 0
	if runErr != nil {
		result.State = protocol.SetupFailed
		exit = -1
		var exitErr *exec.ExitError
		if errors.As(runErr, &exitErr) {
			exit = exitErr.ExitCode()
		}
	} else {
		result.State = protocol.SetupDone
	}
	result.Exit = &exit
	w.storeSetup(p.ID, path, result)
	if runErr != nil {
		return result, unavailable("Worktree setup failed with exit code %d.", exit)
	}
	return result, nil
}

func shellQuote(value string) string { return "'" + strings.ReplaceAll(value, "'", "'\\''") + "'" }

func (w *Work) storeSetup(project, path string, result protocol.SetupResult) {
	w.mu.Lock()
	if p := w.projects[project]; p != nil {
		for i := range p.Worktrees {
			if samePath(p.Worktrees[i].Path, path) {
				p.Worktrees[i].Setup = result
				p.Worktrees[i].Activity = time.Now()
				break
			}
		}
		_ = w.saveLocked()
	}
	w.mu.Unlock()
}

func (w *Work) trustSetup(ctx context.Context, params protocol.TrustParams) (protocol.SetupResult, error) {
	p, _, err := w.projectForPath(ctx, params.Path)
	if err != nil {
		return protocol.SetupResult{}, err
	}
	script := filepath.Join(params.Path, ".rashin", "setup")
	content, err := os.ReadFile(script)
	if err != nil {
		return protocol.SetupResult{}, protocol.Errorf(protocol.CodeNotFound, "%s", i18n.T("The repository setup script does not exist."))
	}
	sum := sha256.Sum256(content)
	actual := hex.EncodeToString(sum[:])
	if params.Hash == "" || params.Hash != actual {
		return protocol.SetupResult{}, conflict("The setup script changed. Review its new hash before trusting it.")
	}
	projectID := p.ID
	w.mu.Lock()
	current := w.projects[projectID]
	if current == nil {
		w.mu.Unlock()
		return protocol.SetupResult{}, protocol.Errorf(protocol.CodeNotFound, "%s", i18n.T("The project does not exist."))
	}
	found := false
	for _, hash := range current.Settings.TrustedScripts {
		if hash == actual {
			found = true
			break
		}
	}
	if !found {
		current.Settings.TrustedScripts = append(current.Settings.TrustedScripts, actual)
	}
	saveErr := w.saveLocked()
	w.mu.Unlock()
	if saveErr != nil {
		return protocol.SetupResult{}, saveErr
	}
	return w.setupWorktreeMode(ctx, params.Path, actual, true)
}

func (w *Work) ports(ctx context.Context, path string) (protocol.PortsResult, error) {
	if _, _, err := w.projectForPath(ctx, path); err != nil {
		return protocol.PortsResult{}, err
	}
	ports, err := gitpkg.Ports(path)
	if err != nil {
		return protocol.PortsResult{}, unavailable("Listening ports could not be read: %v.", err)
	}
	sort.Slice(ports, func(i, j int) bool { return ports[i].Port < ports[j].Port })
	return protocol.PortsResult{Path: path, Ports: ports}, nil
}
