package workspace

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"strings"
	"time"

	i18n "ryoku-i18n"
	"ryoku-rashin/internal/llm"
	"ryoku-rashin/internal/protocol"
)

func (w *Work) reviewList(path string) protocol.ReviewResult {
	result := protocol.ReviewResult{Worktree: path, Comments: []protocol.ReviewComment{}}
	w.mu.RLock()
	defer w.mu.RUnlock()
	for _, p := range w.projects {
		for _, stored := range p.Comments {
			if samePath(stored.Comment.Worktree, path) {
				result.Comments = append(result.Comments, stored.Comment)
			}
		}
	}
	sort.Slice(result.Comments, func(i, j int) bool { return result.Comments[i].Created.Before(result.Comments[j].Created) })
	return result
}

func (w *Work) reviewAdd(ctx context.Context, params protocol.ReviewAddParams) (result protocol.ReviewComment, err error) {
	p, _, err := w.projectForPath(ctx, params.Worktree)
	if err != nil {
		return protocol.ReviewComment{}, err
	}
	defer w.trackMutation(p.ID, "you", "review.add", params.Worktree, &err)()
	if params.Side != "new" && params.Side != "old" {
		return protocol.ReviewComment{}, badRequest("A review comment side must be new or old.")
	}
	if params.Line <= 0 || strings.TrimSpace(params.Path) == "" || strings.TrimSpace(params.Text) == "" {
		return protocol.ReviewComment{}, badRequest("A review comment needs a path, line and text.")
	}
	comment := protocol.ReviewComment{
		ID: newID("review"), Worktree: params.Worktree, Path: params.Path, Line: params.Line,
		Side: params.Side, Text: params.Text, Created: time.Now(),
	}
	w.mu.Lock()
	if current := w.projects[p.ID]; current != nil {
		current.Comments = append(current.Comments, storedComment{Comment: comment})
		err = w.saveLocked()
	}
	w.mu.Unlock()
	if err != nil {
		return protocol.ReviewComment{}, err
	}
	w.broadcast(protocol.EvtReviewChanged, w.reviewList(params.Worktree))
	return comment, nil
}

func (w *Work) reviewResolve(ctx context.Context, params protocol.ReviewRef) (result protocol.ReviewComment, err error) {
	p, _, err := w.projectForPath(ctx, params.Worktree)
	if err != nil {
		return protocol.ReviewComment{}, err
	}
	defer w.trackMutation(p.ID, "you", "review.resolve", params.Worktree, &err)()
	w.mu.Lock()
	for i := range p.Comments {
		if p.Comments[i].Comment.ID == params.ID && samePath(p.Comments[i].Comment.Worktree, params.Worktree) {
			p.Comments[i].Comment.Resolved = params.Resolved
			comment := p.Comments[i].Comment
			saveErr := w.saveLocked()
			w.mu.Unlock()
			if saveErr != nil {
				return protocol.ReviewComment{}, saveErr
			}
			w.broadcast(protocol.EvtReviewChanged, w.reviewList(params.Worktree))
			return comment, nil
		}
	}
	w.mu.Unlock()
	return protocol.ReviewComment{}, protocol.Errorf(protocol.CodeNotFound, "%s", i18n.T("The review comment does not exist."))
}

func (w *Work) reviewDelete(ctx context.Context, params protocol.ReviewRef) (err error) {
	p, _, err := w.projectForPath(ctx, params.Worktree)
	if err != nil {
		return err
	}
	defer w.trackMutation(p.ID, "you", "review.delete", params.Worktree, &err)()
	w.mu.Lock()
	found := false
	kept := p.Comments[:0]
	for _, item := range p.Comments {
		if item.Comment.ID == params.ID && samePath(item.Comment.Worktree, params.Worktree) {
			found = true
			continue
		}
		kept = append(kept, item)
	}
	if !found {
		w.mu.Unlock()
		return protocol.Errorf(protocol.CodeNotFound, "%s", i18n.T("The review comment does not exist."))
	}
	p.Comments = kept
	err = w.saveLocked()
	w.mu.Unlock()
	if err == nil {
		w.broadcast(protocol.EvtReviewChanged, w.reviewList(params.Worktree))
	}
	return err
}

func (w *Work) reviewSend(ctx context.Context, params protocol.ReviewSendParams) (result protocol.SessionInfo, err error) {
	p, _, err := w.projectForPath(ctx, params.Worktree)
	if err != nil {
		return protocol.SessionInfo{}, err
	}
	defer w.trackMutation(p.ID, "you", "review.send", params.Worktree, &err)()
	w.mu.RLock()
	sessions := w.sessions
	comments := make([]protocol.ReviewComment, 0)
	for _, item := range p.Comments {
		if samePath(item.Comment.Worktree, params.Worktree) && !item.Comment.Resolved {
			comments = append(comments, item.Comment)
		}
	}
	w.mu.RUnlock()
	if len(comments) == 0 {
		return protocol.SessionInfo{}, conflict("There are no unresolved review comments to send.")
	}
	if sessions == nil {
		return protocol.SessionInfo{}, unavailable("Session control is not available.")
	}
	var selected protocol.SessionInfo
	if params.Session != "" {
		var ok bool
		selected, ok = sessions.Info(params.Session)
		if !ok {
			return protocol.SessionInfo{}, protocol.Errorf(protocol.CodeNotFound, "%s", i18n.T("The selected session does not exist."))
		}
		if !samePath(selected.Worktree, params.Worktree) {
			return protocol.SessionInfo{}, badRequest("The selected session belongs to another worktree.")
		}
	} else {
		for _, candidate := range sessions.List(protocol.KindAgent) {
			if samePath(candidate.Worktree, params.Worktree) && (selected.ID == "" || candidate.Updated.After(selected.Updated)) {
				selected = candidate
			}
		}
	}
	if selected.ID == "" {
		harness := p.Settings.Harness
		if harness == "" {
			harness = w.settings().Agent.Harness
		}
		selected, err = sessions.New(ctx, protocol.SessionNewParams{
			Kind: protocol.KindAgent, Harness: harness, Project: p.ID, Worktree: params.Worktree,
			Title: i18n.T("Review changes"), Origin: protocol.OriginApp,
		})
		if err != nil {
			return protocol.SessionInfo{}, err
		}
	}
	diff, _ := w.gitDiff(ctx, protocol.DiffParams{Worktree: params.Worktree, Mode: protocol.DiffBase})
	var prompt strings.Builder
	prompt.WriteString(i18n.T("Address every unresolved review comment below. Preserve unrelated changes."))
	prompt.WriteString("\n\n")
	for _, comment := range comments {
		fmt.Fprintf(&prompt, "%s:%d (%s)", comment.Path, comment.Line, comment.Side)
		if quoted := quoteDiffLine(diff, comment); quoted != "" {
			fmt.Fprintf(&prompt, "\n> %s", quoted)
		}
		fmt.Fprintf(&prompt, "\n%s\n\n", comment.Text)
	}
	if strings.TrimSpace(params.Note) != "" {
		prompt.WriteString(params.Note)
		prompt.WriteByte('\n')
	}
	if _, err := sessions.Prompt(ctx, protocol.PromptParams{ID: selected.ID, Text: prompt.String()}); err != nil {
		return protocol.SessionInfo{}, err
	}
	w.mu.Lock()
	if current := w.projects[p.ID]; current != nil {
		for i := range current.Comments {
			for _, sent := range comments {
				if current.Comments[i].Comment.ID == sent.ID {
					current.Comments[i].Sent = true
				}
			}
		}
		_ = w.saveLocked()
	}
	w.mu.Unlock()
	w.broadcast(protocol.EvtReviewChanged, w.reviewList(params.Worktree))
	if refreshed, ok := sessions.Info(selected.ID); ok {
		selected = refreshed
	}
	return selected, nil
}

func quoteDiffLine(diff protocol.DiffResult, comment protocol.ReviewComment) string {
	for _, file := range diff.Files {
		if file.Path != comment.Path && file.OldPath != comment.Path {
			continue
		}
		for _, hunk := range file.Hunks {
			for _, line := range hunk.Lines {
				if comment.Side == "new" && line.New == comment.Line {
					return line.Text
				}
				if comment.Side == "old" && line.Old == comment.Line {
					return line.Text
				}
			}
		}
	}
	return ""
}

func (w *Work) Tasks(_ context.Context) []protocol.TaskInfo {
	w.mu.RLock()
	defer w.mu.RUnlock()
	var result []protocol.TaskInfo
	for _, p := range w.projects {
		result = append(result, p.Tasks...)
	}
	sortTasks(result)
	return result
}

func (w *Work) tasksList(project string) (protocol.TasksResult, error) {
	w.mu.RLock()
	defer w.mu.RUnlock()
	var result []protocol.TaskInfo
	if project != "" {
		p, err := w.projectLocked(project)
		if err != nil {
			return protocol.TasksResult{}, err
		}
		result = append(result, p.Tasks...)
	} else {
		for _, p := range w.projects {
			result = append(result, p.Tasks...)
		}
	}
	sortTasks(result)
	return protocol.TasksResult{Tasks: result}, nil
}

func (w *Work) taskCreate(params protocol.TaskCreateParams) (result protocol.TaskInfo, err error) {
	defer w.trackMutation(params.Project, "you", "task.create", params.Title, &err)()
	if strings.TrimSpace(params.Title) == "" {
		return protocol.TaskInfo{}, badRequest("A task title is required.")
	}
	w.mu.Lock()
	p, err := w.projectLocked(params.Project)
	if err != nil {
		w.mu.Unlock()
		return protocol.TaskInfo{}, err
	}
	if err := validateDeps(p, "", params.Deps); err != nil {
		w.mu.Unlock()
		return protocol.TaskInfo{}, err
	}
	now := time.Now()
	task := protocol.TaskInfo{
		ID: newID("task"), Project: p.ID, Title: strings.TrimSpace(params.Title), Goal: params.Goal,
		Status: protocol.TaskBacklog, Deps: append([]string(nil), params.Deps...), Harness: params.Harness,
		Order: len(p.Tasks), Created: now, Updated: now,
	}
	p.Tasks = append(p.Tasks, task)
	err = w.saveLocked()
	w.mu.Unlock()
	if err != nil {
		return protocol.TaskInfo{}, err
	}
	w.broadcastTasks(p.ID)
	return task, nil
}

func (w *Work) taskUpdate(params protocol.TaskUpdateParams) (result protocol.TaskInfo, err error) {
	projectForActivity := w.taskProjectID(params.ID)
	defer w.trackMutation(projectForActivity, "you", "task.update", params.ID, &err)()
	var fields map[string]json.RawMessage
	if json.Unmarshal(params.Patch, &fields) != nil {
		return protocol.TaskInfo{}, badRequest("The task patch is not valid JSON.")
	}
	allowed := map[string]bool{"title": true, "goal": true, "status": true, "deps": true, "harness": true, "order": true}
	for key := range fields {
		if !allowed[key] {
			return protocol.TaskInfo{}, badRequest("Task field %s cannot be changed.", key)
		}
	}
	w.mu.Lock()
	p, index := w.findTaskLocked(params.ID)
	if p == nil {
		w.mu.Unlock()
		return protocol.TaskInfo{}, protocol.Errorf(protocol.CodeNotFound, "%s", i18n.T("The task does not exist."))
	}
	next, err := mergeJSON(p.Tasks[index], params.Patch)
	if err != nil {
		w.mu.Unlock()
		return protocol.TaskInfo{}, err
	}
	if strings.TrimSpace(next.Title) == "" {
		w.mu.Unlock()
		return protocol.TaskInfo{}, badRequest("A task title is required.")
	}
	if !validTaskStatus(next.Status) {
		w.mu.Unlock()
		return protocol.TaskInfo{}, badRequest("The task status is invalid.")
	}
	if err := validateDeps(p, next.ID, next.Deps); err != nil {
		w.mu.Unlock()
		return protocol.TaskInfo{}, err
	}
	next.ID, next.Project, next.Created = p.Tasks[index].ID, p.Tasks[index].Project, p.Tasks[index].Created
	next.Updated = time.Now()
	p.Tasks[index] = next
	recomputeTaskReadiness(p)
	next = p.Tasks[index]
	project := p.ID
	if err := w.saveLocked(); err != nil {
		w.mu.Unlock()
		return protocol.TaskInfo{}, err
	}
	w.mu.Unlock()
	w.broadcastTasks(project)
	return next, nil
}

func validTaskStatus(status string) bool {
	switch status {
	case protocol.TaskBacklog, protocol.TaskReady, protocol.TaskWorking, protocol.TaskNeedsYou, protocol.TaskReview, protocol.TaskDone, protocol.TaskBlocked:
		return true
	default:
		return false
	}
}

func validateDeps(p *projectRecord, self string, deps []string) error {
	known := make(map[string]bool, len(p.Tasks))
	graph := make(map[string][]string, len(p.Tasks))
	for _, task := range p.Tasks {
		known[task.ID] = true
		graph[task.ID] = task.Deps
	}
	for _, dep := range deps {
		if dep == self {
			return badRequest("A task cannot depend on itself.")
		}
		if !known[dep] {
			return badRequest("Task dependency %s does not exist in this project.", dep)
		}
	}
	if self != "" {
		graph[self] = deps
	}
	state := make(map[string]uint8, len(graph))
	var visit func(string) bool
	visit = func(id string) bool {
		if state[id] == 1 {
			return true
		}
		if state[id] == 2 {
			return false
		}
		state[id] = 1
		for _, dep := range graph[id] {
			if visit(dep) {
				return true
			}
		}
		state[id] = 2
		return false
	}
	for id := range graph {
		if visit(id) {
			return badRequest("Task dependencies cannot contain a cycle.")
		}
	}
	return nil
}

func recomputeTaskReadiness(p *projectRecord) {
	now := time.Now()
	for i := range p.Tasks {
		task := &p.Tasks[i]
		if len(task.Deps) == 0 {
			continue
		}
		allDone := true
		for _, depID := range task.Deps {
			_, depIndex := findTaskIn(p, depID)
			if depIndex < 0 || p.Tasks[depIndex].Status != protocol.TaskDone {
				allDone = false
				break
			}
		}
		if allDone && task.Status == protocol.TaskBlocked {
			task.Status = protocol.TaskReady
			task.Updated = now
		} else if !allDone && task.Status == protocol.TaskReady {
			task.Status = protocol.TaskBlocked
			task.Updated = now
		}
	}
}

func (w *Work) findTaskLocked(id string) (*projectRecord, int) {
	for _, p := range w.projects {
		for i := range p.Tasks {
			if p.Tasks[i].ID == id {
				return p, i
			}
		}
	}
	return nil, -1
}

func (w *Work) taskProjectID(id string) string {
	w.mu.RLock()
	defer w.mu.RUnlock()
	if p, _ := w.findTaskLocked(id); p != nil {
		return p.ID
	}
	return ""
}

func (w *Work) taskDelete(id string) (err error) {
	projectForActivity := w.taskProjectID(id)
	defer w.trackMutation(projectForActivity, "you", "task.delete", id, &err)()
	w.mu.Lock()
	p, index := w.findTaskLocked(id)
	if p == nil {
		w.mu.Unlock()
		return protocol.Errorf(protocol.CodeNotFound, "%s", i18n.T("The task does not exist."))
	}
	p.Tasks = append(p.Tasks[:index], p.Tasks[index+1:]...)
	for i := range p.Tasks {
		before := len(p.Tasks[i].Deps)
		deps := p.Tasks[i].Deps[:0]
		for _, dep := range p.Tasks[i].Deps {
			if dep != id {
				deps = append(deps, dep)
			}
		}
		p.Tasks[i].Deps = deps
		if len(deps) != before {
			p.Tasks[i].Updated = time.Now()
		}
	}
	recomputeTaskReadiness(p)
	err = w.saveLocked()
	project := p.ID
	w.mu.Unlock()
	if err == nil {
		w.broadcastTasks(project)
	}
	return err
}

func (w *Work) taskStart(ctx context.Context, params protocol.TaskStartParams) (result protocol.TaskStartResult, err error) {
	projectForActivity := w.taskProjectID(params.ID)
	defer w.trackMutation(projectForActivity, "you", "task.start", params.ID, &err)()
	w.mu.RLock()
	p, index := w.findTaskLocked(params.ID)
	if p == nil {
		w.mu.RUnlock()
		return protocol.TaskStartResult{}, protocol.Errorf(protocol.CodeNotFound, "%s", i18n.T("The task does not exist."))
	}
	task := p.Tasks[index]
	projectID := p.ID
	blocked := false
	for _, depID := range task.Deps {
		_, depIndex := findTaskIn(p, depID)
		if depIndex < 0 || p.Tasks[depIndex].Status != protocol.TaskDone {
			blocked = true
			break
		}
	}
	sessions := w.sessions
	w.mu.RUnlock()
	if blocked {
		w.mu.Lock()
		if current, currentIndex := w.findTaskLocked(task.ID); current != nil {
			current.Tasks[currentIndex].Status = protocol.TaskBlocked
			current.Tasks[currentIndex].Updated = time.Now()
			recomputeTaskReadiness(current)
			_ = w.saveLocked()
		}
		w.mu.Unlock()
		w.broadcastTasks(projectID)
		return protocol.TaskStartResult{}, conflict("Task dependencies must be done before this task starts.")
	}
	if sessions == nil {
		return protocol.TaskStartResult{}, unavailable("Session control is not available.")
	}
	worktree, err := w.createWorktree(ctx, protocol.WorktreeCreateParams{
		Project: projectID, Name: task.Title, Mode: protocol.WorktreeNewBranch, Base: params.Base, Task: task.ID,
	}, "you")
	if err != nil {
		return protocol.TaskStartResult{}, err
	}
	harness := params.Harness
	if harness == "" {
		harness = task.Harness
	}
	if harness == "" {
		w.mu.RLock()
		current := w.projects[projectID]
		if current != nil {
			harness = current.Settings.Harness
		}
		w.mu.RUnlock()
	}
	if harness == "" {
		harness = w.settings().Agent.Harness
	}
	session, err := sessions.New(ctx, protocol.SessionNewParams{
		Kind: protocol.KindAgent, Harness: harness, Project: projectID, Worktree: worktree.Path,
		Task: task.ID, Title: task.Title, Origin: protocol.OriginApp,
	})
	if err != nil {
		_ = w.removeWorktree(context.Background(), protocol.WorktreeRemoveParams{Path: worktree.Path, Force: true}, "Rashin")
		return protocol.TaskStartResult{}, err
	}
	w.mu.Lock()
	p, index = w.findTaskLocked(task.ID)
	if p != nil {
		p.Tasks[index].Status = protocol.TaskWorking
		p.Tasks[index].Worktree = worktree.Path
		p.Tasks[index].Sessions = append(p.Tasks[index].Sessions, session.ID)
		p.Tasks[index].Harness = harness
		p.Tasks[index].Updated = time.Now()
		recomputeTaskReadiness(p)
		task = p.Tasks[index]
		_ = w.saveLocked()
	}
	w.mu.Unlock()
	if _, err := sessions.Prompt(ctx, protocol.PromptParams{ID: session.ID, Text: task.Goal}); err != nil {
		w.mu.Lock()
		if current, currentIndex := w.findTaskLocked(task.ID); current != nil {
			current.Tasks[currentIndex].Status = protocol.TaskNeedsYou
			current.Tasks[currentIndex].Updated = time.Now()
			recomputeTaskReadiness(current)
			task = current.Tasks[currentIndex]
			_ = w.saveLocked()
		}
		w.mu.Unlock()
		w.broadcastTasks(projectID)
		return protocol.TaskStartResult{}, err
	}
	w.broadcastTasks(projectID)
	return protocol.TaskStartResult{Task: task, Worktree: worktree, Session: session}, nil
}

func findTaskIn(p *projectRecord, id string) (*protocol.TaskInfo, int) {
	for i := range p.Tasks {
		if p.Tasks[i].ID == id {
			return &p.Tasks[i], i
		}
	}
	return nil, -1
}

func (w *Work) taskGate(params protocol.GateParams) (result protocol.TaskInfo, err error) {
	projectForActivity := w.taskProjectID(params.Task)
	defer w.trackMutation(projectForActivity, "you", "task.gate.open", params.Task, &err)()
	if strings.TrimSpace(params.Question) == "" || len(params.Options) == 0 {
		return protocol.TaskInfo{}, badRequest("A gate needs a question and options.")
	}
	w.mu.Lock()
	p, index := w.findTaskLocked(params.Task)
	if p == nil {
		w.mu.Unlock()
		return protocol.TaskInfo{}, protocol.Errorf(protocol.CodeNotFound, "%s", i18n.T("The task does not exist."))
	}
	gate := protocol.Gate{ID: newID("gate"), Question: params.Question, Options: append([]string(nil), params.Options...), Created: time.Now()}
	p.Tasks[index].Gates = append(p.Tasks[index].Gates, gate)
	p.Tasks[index].Status = protocol.TaskNeedsYou
	p.Tasks[index].Updated = time.Now()
	recomputeTaskReadiness(p)
	task := p.Tasks[index]
	err = w.saveLocked()
	w.mu.Unlock()
	if err != nil {
		return protocol.TaskInfo{}, err
	}
	w.broadcastTasks(p.ID)
	return task, nil
}

func (w *Work) resolveGate(ctx context.Context, params protocol.GateResolveParams) (result protocol.TaskInfo, err error) {
	projectForActivity := w.taskProjectID(params.Task)
	defer w.trackMutation(projectForActivity, "you", "task.gate.resolve", params.Task, &err)()
	w.mu.RLock()
	p, index := w.findTaskLocked(params.Task)
	if p == nil {
		w.mu.RUnlock()
		return protocol.TaskInfo{}, protocol.Errorf(protocol.CodeNotFound, "%s", i18n.T("The task does not exist."))
	}
	sessionIDs := append([]string(nil), p.Tasks[index].Sessions...)
	sessions := w.sessions
	w.mu.RUnlock()
	var selected protocol.SessionInfo
	if sessions != nil {
		for _, id := range sessionIDs {
			if candidate, ok := sessions.Info(id); ok && (selected.ID == "" || candidate.Updated.After(selected.Updated)) {
				selected = candidate
			}
		}
	}
	w.mu.Lock()
	p, index = w.findTaskLocked(params.Task)
	if p == nil {
		w.mu.Unlock()
		return protocol.TaskInfo{}, protocol.Errorf(protocol.CodeNotFound, "%s", i18n.T("The task does not exist."))
	}
	gateIndex := -1
	for i := range p.Tasks[index].Gates {
		if p.Tasks[index].Gates[i].ID == params.Gate {
			gateIndex = i
			break
		}
	}
	if gateIndex < 0 {
		w.mu.Unlock()
		return protocol.TaskInfo{}, protocol.Errorf(protocol.CodeNotFound, "%s", i18n.T("The decision gate does not exist."))
	}
	gate := p.Tasks[index].Gates[gateIndex]
	valid := false
	for _, option := range gate.Options {
		if option == params.Resolution {
			valid = true
			break
		}
	}
	if !valid {
		w.mu.Unlock()
		return protocol.TaskInfo{}, badRequest("The gate resolution must be one of its options.")
	}
	if gate.Resolution != "" {
		w.mu.Unlock()
		return protocol.TaskInfo{}, conflict("The decision gate is already resolved.")
	}
	p.Tasks[index].Gates[gateIndex].Resolution = params.Resolution
	if selected.ID == "" {
		p.Tasks[index].Status = protocol.TaskReady
	} else {
		p.Tasks[index].Status = protocol.TaskWorking
	}
	p.Tasks[index].Updated = time.Now()
	recomputeTaskReadiness(p)
	task := p.Tasks[index]
	project := p.ID
	if err := w.saveLocked(); err != nil {
		w.mu.Unlock()
		return protocol.TaskInfo{}, err
	}
	w.mu.Unlock()
	if selected.ID != "" {
		text := i18n.Tf("Decision for %s: %s", gate.Question, params.Resolution)
		_, _ = sessions.Prompt(ctx, protocol.PromptParams{ID: selected.ID, Text: text})
	}
	w.broadcastTasks(project)
	return task, nil
}

func (w *Work) taskPlan(ctx context.Context, params protocol.TaskPlanParams) (protocol.TaskPlanResult, error) {
	if _, err := w.project(params.Project); err != nil {
		return protocol.TaskPlanResult{}, err
	}
	if w.client == nil {
		return protocol.TaskPlanResult{}, unavailable("The planning model is not available.")
	}
	request := llm.Request{Model: w.settings().Ask.Model, MaxTokens: 1800, Messages: []llm.Message{
		{Role: llm.RoleSystem, Content: "Split the project goal into concrete independent coding tasks. Return strict JSON only with this shape: {\"tasks\":[{\"title\":\"...\",\"goal\":\"...\",\"deps\":[0]}]}. Dependency indexes must refer to earlier tasks. Return at most 12 tasks."},
		{Role: llm.RoleUser, Content: params.Goal},
	}}
	stream, err := w.client.Stream(ctx, request)
	if err != nil {
		return protocol.TaskPlanResult{}, unavailable("Task planning could not start: %v.", err)
	}
	defer stream.Close()
	var text strings.Builder
	for {
		delta, nextErr := stream.Next()
		if nextErr == io.EOF {
			break
		}
		if nextErr != nil {
			return protocol.TaskPlanResult{}, unavailable("Task planning failed: %v.", nextErr)
		}
		text.WriteString(delta.Text)
	}
	document, ok := decodeTaskPlan(text.String())
	if !ok {
		return protocol.TaskPlanResult{}, unavailable("The planning model did not return a task list.")
	}
	if len(document.Tasks) > 12 {
		document.Tasks = document.Tasks[:12]
	}
	for i := range document.Tasks {
		if strings.TrimSpace(document.Tasks[i].Title) == "" {
			return protocol.TaskPlanResult{}, unavailable("The planning model returned a task without a title.")
		}
		kept := document.Tasks[i].Deps[:0]
		for _, dep := range document.Tasks[i].Deps {
			if dep >= 0 && dep < i {
				kept = append(kept, dep)
			}
		}
		document.Tasks[i].Deps = kept
	}
	final := stream.Final()
	via := final.Model
	if final.Provider != "" && !strings.Contains(via, "/") {
		via = final.Provider + "/" + via
	}
	return protocol.TaskPlanResult{Drafts: document.Tasks, Via: via}, nil
}

type taskPlanDocument struct {
	Tasks []protocol.TaskDraft `json:"tasks"`
}

func decodeTaskPlan(text string) (taskPlanDocument, bool) {
	for offset := 0; offset < len(text); {
		next := strings.IndexByte(text[offset:], '{')
		if next < 0 {
			break
		}
		offset += next
		var document taskPlanDocument
		decoder := json.NewDecoder(strings.NewReader(text[offset:]))
		if decoder.Decode(&document) == nil && len(document.Tasks) > 0 {
			return document, true
		}
		offset++
	}
	return taskPlanDocument{}, false
}
