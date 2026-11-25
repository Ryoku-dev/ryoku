package workspace

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	i18n "ryoku-i18n"
	gitpkg "ryoku-rashin/internal/git"
	"ryoku-rashin/internal/paths"
	"ryoku-rashin/internal/protocol"
)

func defaultProjectSettings() protocol.ProjectSettings {
	return protocol.ProjectSettings{
		BranchPrefix: "rashin/",
		Cleanup:      protocol.CleanupPolicy{Protect: protocol.ProtectUncommitted},
	}
}

func (w *Work) addProject(ctx context.Context, params protocol.ProjectAddParams) (protocol.ProjectInfo, error) {
	if strings.TrimSpace(params.Path) == "" {
		return protocol.ProjectInfo{}, badRequest("A project path is required.")
	}
	absolute, err := filepath.Abs(paths.Expand(params.Path))
	if err != nil {
		return protocol.ProjectInfo{}, badRequest("The project path is invalid.")
	}
	info, err := os.Stat(absolute)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return protocol.ProjectInfo{}, protocol.Errorf(protocol.CodeNotFound, "%s", i18n.T("The project folder does not exist."))
		}
		return protocol.ProjectInfo{}, unavailable("The project folder cannot be read: %v.", err)
	}
	if !info.IsDir() {
		return protocol.ProjectInfo{}, badRequest("The project path must be a folder.")
	}
	if canonical, resolveErr := filepath.EvalSymlinks(absolute); resolveErr == nil {
		absolute = canonical
	}
	isGit := false
	if top, topErr := gitpkg.TopLevel(ctx, absolute); topErr == nil {
		absolute = top
		isGit = true
	}
	id := projectID(absolute)
	w.mu.Lock()
	if existing := w.projects[id]; existing != nil {
		copy := *existing
		w.mu.Unlock()
		return w.projectInfo(ctx, &copy)
	}
	now := time.Now()
	name := strings.TrimSpace(params.Name)
	if name == "" {
		name = filepath.Base(absolute)
	}
	p := &projectRecord{ID: id, Name: name, Root: absolute, Git: isGit, Settings: defaultProjectSettings(), Created: now, Activity: now}
	w.projects[id] = p
	if err := w.saveLocked(); err != nil {
		delete(w.projects, id)
		w.mu.Unlock()
		return protocol.ProjectInfo{}, err
	}
	w.mu.Unlock()
	result, err := w.projectInfo(ctx, p)
	if err != nil {
		return protocol.ProjectInfo{}, err
	}
	w.recordActivity(id, "you", "project.add", absolute, true, "")
	w.broadcastProjects(ctx)
	return result, nil
}

func (w *Work) removeProject(ctx context.Context, id string) error {
	w.mu.Lock()
	if _, ok := w.projects[id]; !ok {
		w.mu.Unlock()
		return protocol.Errorf(protocol.CodeNotFound, "%s", i18n.Tf("No project %s.", id))
	}
	delete(w.projects, id)
	delete(w.status, id)
	current := w.watchers[id]
	delete(w.watchers, id)
	delete(w.watchRef, id)
	err := w.saveLocked()
	w.mu.Unlock()
	if current != nil {
		stopWatcher(current)
	}
	if err != nil {
		return err
	}
	w.broadcastProjects(ctx)
	return nil
}

func (w *Work) updateProjectSettings(ctx context.Context, params protocol.ProjectSettingsParams) (result protocol.ProjectInfo, err error) {
	defer w.trackMutation(params.Project, "you", "project.settings", params.Project, &err)()
	w.mu.Lock()
	p, err := w.projectLocked(params.Project)
	if err != nil {
		w.mu.Unlock()
		return protocol.ProjectInfo{}, err
	}
	next, err := mergeJSON(p.Settings, params.Patch)
	if err != nil {
		w.mu.Unlock()
		return protocol.ProjectInfo{}, err
	}
	if err := validateProjectSettings(ctx, p, next); err != nil {
		w.mu.Unlock()
		return protocol.ProjectInfo{}, err
	}
	p.Settings = next
	p.Activity = time.Now()
	if err := w.saveLocked(); err != nil {
		w.mu.Unlock()
		return protocol.ProjectInfo{}, err
	}
	copy := *p
	w.mu.Unlock()
	result, err = w.projectInfo(ctx, &copy)
	if err == nil {
		w.broadcastProjects(ctx)
	}
	return result, err
}

func validateProjectSettings(ctx context.Context, p *projectRecord, settings protocol.ProjectSettings) error {
	prefix := settings.BranchPrefix
	if prefix == "" {
		prefix = "rashin/"
	}
	if strings.HasPrefix(prefix, "/") || strings.Contains(prefix, "..") {
		return badRequest("The branch prefix is not a valid Git ref fragment.")
	}
	if p.Git {
		if _, err := gitpkg.Run(ctx, p.Root, "check-ref-format", "refs/heads/"+prefix+"work"); err != nil {
			return badRequest("The branch prefix is not a valid Git ref fragment.")
		}
		if settings.BaseRef != "" {
			if _, err := gitpkg.Run(ctx, p.Root, "rev-parse", "--verify", settings.BaseRef+"^{commit}"); err != nil {
				return badRequest("The base ref cannot be resolved.")
			}
		}
	}
	if settings.WorktreeRoot != "" && !filepath.IsAbs(settings.WorktreeRoot) && settings.WorktreeRoot != "~" && !strings.HasPrefix(settings.WorktreeRoot, "~/") {
		return badRequest("The worktree folder must be absolute or start with ~.")
	}
	if settings.Cleanup.Protect != "" && settings.Cleanup.Protect != protocol.ProtectUncommitted && settings.Cleanup.Protect != protocol.ProtectAny {
		return badRequest("The cleanup protection must be uncommitted or any.")
	}
	if settings.Cleanup.AfterMergeDays < 0 || settings.Cleanup.InactiveDays < 0 {
		return badRequest("Cleanup day counts cannot be negative.")
	}
	return nil
}

func (w *Work) Projects(ctx context.Context) []protocol.ProjectInfo {
	w.mu.RLock()
	records := make([]projectRecord, 0, len(w.projects))
	for _, p := range w.projects {
		records = append(records, *p)
	}
	w.mu.RUnlock()
	result := make([]protocol.ProjectInfo, 0, len(records))
	for i := range records {
		info, err := w.projectInfo(ctx, &records[i])
		if err == nil {
			result = append(result, info)
		}
	}
	sort.Slice(result, func(i, j int) bool { return result[i].Activity.After(result[j].Activity) })
	return result
}

func (w *Work) projectInfo(ctx context.Context, p *projectRecord) (protocol.ProjectInfo, error) {
	result := protocol.ProjectInfo{
		ID: p.ID, Name: p.Name, Root: p.Root, Git: p.Git, Settings: p.Settings,
		Tasks: len(p.Tasks), Created: p.Created, Activity: p.Activity,
	}
	if p.Git {
		repo, err := gitpkg.Open(ctx, p.Root)
		if err != nil {
			return result, err
		}
		worktrees, err := repo.Worktrees(ctx)
		if err != nil {
			return result, err
		}
		result.Worktrees = len(worktrees)
		result.BaseRef, _ = resolveBase(ctx, p, repo, worktrees)
		for _, wt := range worktrees {
			if wt.Main {
				result.Branch = wt.Branch
				break
			}
		}
	}
	w.mu.RLock()
	sessions := w.sessions
	w.mu.RUnlock()
	if sessions != nil {
		for _, session := range sessions.List(protocol.KindAgent) {
			if session.Project != p.ID {
				continue
			}
			result.Sessions++
			if session.State == protocol.StateWaiting || session.State == protocol.StateFailed {
				result.Attention++
			}
		}
	}
	for _, task := range p.Tasks {
		for _, gate := range task.Gates {
			if gate.Resolution == "" {
				result.Attention++
			}
		}
	}
	if p.Git {
		if listed, err := w.listWorktrees(ctx, p.ID, false); err == nil {
			for _, wt := range listed.Worktrees {
				if wt.Status != nil && wt.Status.Conflicted > 0 {
					result.Attention++
				}
			}
		}
	}
	return result, nil
}

func resolveBase(ctx context.Context, p *projectRecord, repo *gitpkg.Repo, worktrees []gitpkg.Worktree) (string, error) {
	if p.Settings.BaseRef != "" {
		if _, err := gitpkg.Run(ctx, p.Root, "rev-parse", "--verify", p.Settings.BaseRef+"^{commit}"); err != nil {
			return "", badRequest("The configured base ref cannot be resolved.")
		}
		return p.Settings.BaseRef, nil
	}
	if out, err := gitpkg.Run(ctx, p.Root, "symbolic-ref", "--quiet", "--short", "refs/remotes/origin/HEAD"); err == nil {
		base := strings.TrimSpace(string(out))
		if base != "" {
			return base, nil
		}
	}
	for _, wt := range worktrees {
		if wt.Main && wt.Branch != "" {
			return wt.Branch, nil
		}
	}
	return "HEAD", nil
}

func (w *Work) listActivity(params protocol.ActivityParams) (protocol.ActivityResult, error) {
	if _, err := w.project(params.Project); err != nil {
		return protocol.ActivityResult{}, err
	}
	limit := params.Limit
	if limit <= 0 || limit > 5000 {
		limit = 100
	}
	data, err := os.ReadFile(paths.ActivityLog(params.Project))
	if errors.Is(err, os.ErrNotExist) {
		return protocol.ActivityResult{Activity: []protocol.Activity{}}, nil
	}
	if err != nil {
		return protocol.ActivityResult{}, err
	}
	lines := strings.Split(strings.TrimSuffix(string(data), "\n"), "\n")
	result := protocol.ActivityResult{Activity: make([]protocol.Activity, 0, limit)}
	for i := len(lines) - 1; i >= 0 && len(result.Activity) < limit; i-- {
		if lines[i] == "" {
			continue
		}
		var entry protocol.Activity
		if json.Unmarshal([]byte(lines[i]), &entry) == nil {
			result.Activity = append(result.Activity, entry)
		}
	}
	return result, nil
}
