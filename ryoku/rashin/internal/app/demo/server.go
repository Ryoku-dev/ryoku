// Package demo serves deterministic Rashin fixtures over the real wire protocol.
package demo

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"time"

	"ryoku-i18n"

	"ryoku-rashin/internal/protocol"
)

func Serve(ctx context.Context, socket string) error {
	if err := os.MkdirAll(filepath.Dir(socket), 0o700); err != nil {
		return err
	}
	_ = os.Remove(socket)
	listener, err := net.Listen("unix", socket)
	if err != nil {
		return err
	}
	defer listener.Close()
	defer os.Remove(socket)
	server := &server{data: makeFixtures()}
	go func() { <-ctx.Done(); _ = listener.Close() }()
	for {
		connection, err := listener.Accept()
		if err != nil {
			if ctx.Err() != nil {
				return nil
			}
			return err
		}
		go server.serve(ctx, connection)
	}
}

type server struct {
	mu   sync.Mutex
	data fixtures
	// session events a method wrote, for the client to hear after its answer
	told []protocol.SessionEventData
}

func (s *server) serve(ctx context.Context, connection net.Conn) {
	defer connection.Close()
	decoder := protocol.NewDecoder(connection)
	out := &outbox{encoder: protocol.NewEncoder(connection)}
	for {
		frame, err := decoder.Next()
		if err != nil {
			if !errors.Is(err, io.EOF) && ctx.Err() == nil {
				_ = out.send(protocol.Response{Error: protocol.AsError(err)})
			}
			return
		}
		if !frame.IsRequest() || frame.ID == nil {
			continue
		}
		result, methodErr := s.call(frame.Method, frame.Params)
		response := protocol.Response{ID: *frame.ID}
		if methodErr != nil {
			response.Error = protocol.AsError(methodErr)
		} else if response.Result, methodErr = json.Marshal(result); methodErr != nil {
			response.Error = protocol.AsError(methodErr)
		}
		if out.send(response) != nil {
			return
		}
		if frame.Method == protocol.MSessionPrompt && response.Error == nil {
			s.wakeFirst(out, frame.Params)
			go s.playTurn(ctx, out, frame.Params)
		}
		if frame.Method == protocol.MSessionPermission && response.Error == nil {
			s.resolvePermission(out, frame.Params)
		}
		if frame.Method == protocol.MSessionStart && response.Error == nil {
			s.wake(out, frame.Params)
		}
		if response.Error == nil {
			s.chose(out, frame.Method, frame.Params)
		}
		if response.Error == nil {
			s.announce(out, frame.Method)
			s.tellSessions(out, frame.Method, frame.Params)
		}
	}
}

func (s *server) call(method string, raw json.RawMessage) (any, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if strings.HasPrefix(method, "memory.") {
		return s.callMemory(method, raw)
	}
	switch method {
	case protocol.MHello:
		return protocol.HelloResult{Daemon: "rashind-demo", Version: "2.0.0-demo", Protocol: protocol.Version, Enabled: true}, nil
	case protocol.MSubscribe:
		snapshot := s.data.snapshot
		s.fillTeamsSnapshot(&snapshot)
		snapshot.Memory = s.memoryStatus()
		return snapshot, nil
	case protocol.MStatus:
		return s.data.snapshot.Status, nil
	case protocol.MModelsList:
		return s.data.models, nil
	case protocol.MProvidersList:
		return protocol.ProvidersResult{Providers: s.data.snapshot.Providers}, nil
	case protocol.MHarnessesList:
		return protocol.HarnessesResult{Harnesses: s.data.snapshot.Harnesses}, nil
	case protocol.MHarnessesProbe:
		var params protocol.HarnessProbeParams
		if err := decode(raw, &params); err != nil {
			return nil, err
		}
		return s.probeHarness(params)
	case protocol.MHarnessesTeamsOpen:
		var params protocol.HarnessTeamsOpenParams
		if err := decode(raw, &params); err != nil {
			return nil, err
		}
		return s.openTeamsHome(params)
	case protocol.MHarnessesTeamsDone:
		var params protocol.HarnessTeamsDoneParams
		if err := decode(raw, &params); err != nil {
			return nil, err
		}
		return s.finishTeamsHome(params)
	case protocol.MIndexList:
		return protocol.IndexesResult{Indexes: s.data.snapshot.Indexes}, nil
	case protocol.MSettingsGet:
		return s.data.snapshot.Settings, nil
	case protocol.MProjectsList:
		return protocol.ProjectsResult{Projects: s.data.snapshot.Projects}, nil
	case protocol.MSessionList:
		var params protocol.SessionListParams
		if err := decode(raw, &params); err != nil {
			return nil, err
		}
		list := make([]protocol.SessionInfo, 0)
		for _, info := range s.data.snapshot.Sessions {
			if params.Kind == "" || info.Kind == params.Kind {
				list = append(list, info)
			}
		}
		if params.Limit > 0 && len(list) > params.Limit {
			list = list[:params.Limit]
		}
		return protocol.SessionListResult{Sessions: list}, nil
	case protocol.MSessionAttach:
		var params protocol.SessionAttachParams
		if err := decode(raw, &params); err != nil {
			return nil, err
		}
		if info, ok := s.session(params.ID); ok {
			events := s.data.events[params.ID]
			first := 0
			for first < len(events) && events[first].Seq <= params.Since {
				first++
			}
			return protocol.SessionAttachResult{Info: info, Events: append([]protocol.SessionEvent(nil), events[first:]...)}, nil
		}
		return nil, protocol.Errorf(protocol.CodeNotFound, "%s", i18n.T("The demo session is not available."))
	case protocol.MSessionNew:
		var params protocol.SessionNewParams
		if err := decode(raw, &params); err != nil {
			return nil, err
		}
		info := s.newSession(params.Kind, params.Harness, params.Cwd, params.Project, params.Worktree, params.Branch, params.Task, "")
		info.Temporary, info.Memory = params.Temporary, newMemoryMode(params)
		if params.Title != "" {
			info.Title = params.Title
		}
		// the model the session was opened on, as the daemon keeps it
		if params.Model != "" {
			info.Model = params.Model
		}
		if params.Kind == protocol.KindAsk {
			info.Mode = params.Mode
		}
		s.replaceSession(info)
		return info, nil
	case protocol.MSessionHandoff:
		var params protocol.HandoffParams
		if err := decode(raw, &params); err != nil {
			return nil, err
		}
		info := s.newSession(protocol.KindAgent, params.Harness, params.Cwd, params.Project, params.Worktree, params.Branch, "", params.ID)
		info.Title = i18n.T("Moved from Ask")
		s.replaceSession(info)
		return info, nil
	case protocol.MSessionRename:
		var params protocol.RenameParams
		if err := decode(raw, &params); err != nil {
			return nil, err
		}
		info, ok := s.session(params.ID)
		if !ok {
			return nil, notFound("session")
		}
		info.Title, info.Updated = params.Title, time.Now()
		s.replaceSession(info)
		return info, nil
	case protocol.MSessionRead:
		var params protocol.SessionRef
		if err := decode(raw, &params); err != nil {
			return nil, err
		}
		if info, ok := s.session(params.ID); ok {
			info.Unread = false
			s.replaceSession(info)
		}
		return protocol.Empty{}, nil
	case protocol.MSessionStop, protocol.MSessionCancel:
		var params protocol.SessionRef
		if err := decode(raw, &params); err != nil {
			return nil, err
		}
		if info, ok := s.session(params.ID); ok {
			info.Alive, info.State, info.Activity = false, protocol.StateStopped, ""
			s.replaceSession(info)
		}
		return protocol.Empty{}, nil
	case protocol.MSessionDelete:
		var params protocol.SessionRef
		if err := decode(raw, &params); err != nil {
			return nil, err
		}
		s.deleteSession(params.ID)
		return protocol.Empty{}, nil
	case protocol.MSessionSetModel:
		var params protocol.SetModelParams
		if err := decode(raw, &params); err != nil {
			return nil, err
		}
		if info, ok := s.session(params.ID); ok {
			info.Model = params.Model
			s.replaceSession(info)
		}
		return protocol.Empty{}, nil
	case protocol.MSessionSetMode:
		var params protocol.SetModeParams
		if err := decode(raw, &params); err != nil {
			return nil, err
		}
		if info, ok := s.session(params.ID); ok {
			info.Mode = params.Mode
			s.replaceSession(info)
		}
		return protocol.Empty{}, nil
	case protocol.MSessionSetMemory:
		var params protocol.SetMemoryParams
		if err := decode(raw, &params); err != nil {
			return nil, err
		}
		return s.setMemory(params)
	case protocol.MSessionSetOption, protocol.MSessionStart:
		return protocol.Empty{}, nil
	case protocol.MSessionQueue:
		var params protocol.QueueParams
		if err := decode(raw, &params); err != nil {
			return nil, err
		}
		info, ok := s.session(params.ID)
		if !ok {
			return nil, notFound("session")
		}
		info.Queue = append(info.Queue, protocol.QueuedPrompt{ID: fmt.Sprintf("queue-%d", len(info.Queue)+1), Text: params.Text})
		s.replaceSession(info)
		return info, nil
	case protocol.MSessionUnqueue:
		var params protocol.UnqueueParams
		if err := decode(raw, &params); err != nil {
			return nil, err
		}
		info, ok := s.session(params.ID)
		if !ok {
			return nil, notFound("session")
		}
		queue := info.Queue[:0]
		for _, prompt := range info.Queue {
			if prompt.ID != params.Queue {
				queue = append(queue, prompt)
			}
		}
		info.Queue = queue
		s.replaceSession(info)
		return info, nil
	case protocol.MSessionFork:
		var params protocol.ForkParams
		if err := decode(raw, &params); err != nil {
			return nil, err
		}
		source, ok := s.session(params.ID)
		if !ok {
			return nil, notFound("session")
		}
		info := s.newSession(
			source.Kind,
			source.Harness,
			source.Cwd,
			source.Project,
			source.Worktree,
			source.Branch,
			source.Task,
			source.ID,
		)
		info.Title = source.Title + " copy"
		info = s.forkFrom(source, info)
		s.replaceSession(info)
		return info, nil
	case protocol.MSessionPrompt, protocol.MSessionSteer:
		return protocol.PromptResult{Turn: 2}, nil
	case protocol.MSessionPermission, protocol.MSessionDetach, protocol.MSessionRevertTurn:
		if method == protocol.MSessionRevertTurn {
			return protocol.GitOpResult{OK: true, Output: i18n.T("Files restored to the checkpoint.")}, nil
		}
		return protocol.Empty{}, nil
	case protocol.MSettingsSet:
		var params protocol.SettingsSetParams
		if err := decode(raw, &params); err != nil {
			return nil, err
		}
		paused := s.data.snapshot.Settings.Teams.Paused
		if err := mergeInto(params.Patch, &s.data.snapshot.Settings); err != nil {
			return nil, err
		}
		if err := s.syncLocalProviders(); err != nil {
			return nil, err
		}
		s.syncCustomHarnesses()
		if s.data.snapshot.Settings.Teams.Paused != paused {
			s.holdEveryTeam(!paused)
		}
		return s.data.snapshot.Settings, nil
	case protocol.MGatewayUsage:
		var params protocol.UsageParams
		if err := decode(raw, &params); err != nil {
			return nil, err
		}
		return demoUsage(Epoch, params.Days), nil
	case protocol.MGatewayAPI:
		return s.data.snapshot.Status.API, nil
	case protocol.MGatewayAPIKey:
		var params protocol.APIKeyParams
		if err := decode(raw, &params); err != nil {
			return nil, err
		}
		return s.apiKey(params.Rotate), nil
	case protocol.MProvidersEnable:
		var params protocol.EnableProviderParams
		if err := decode(raw, &params); err != nil {
			return nil, err
		}
		for index := range s.data.snapshot.Providers {
			if s.data.snapshot.Providers[index].ID == params.Provider {
				s.data.snapshot.Providers[index].Enabled = params.Enabled
				return s.data.snapshot.Providers[index], nil
			}
		}
		return protocol.ProviderInfo{}, nil
	case protocol.MProvidersAddKey:
		var params protocol.AddKeyParams
		if err := decode(raw, &params); err != nil {
			return nil, err
		}
		for index := range s.data.snapshot.Providers {
			if s.data.snapshot.Providers[index].ID == params.Provider {
				provider := &s.data.snapshot.Providers[index]
				provider.Keys = append(
					provider.Keys,
					protocol.KeyInfo{
						ID:     fmt.Sprintf("demo-%d", len(provider.Keys)+1),
						Label:  params.Label,
						Status: protocol.KeyUnknown,
						Masked: "••••demo",
					},
				)
				provider.Configured = true
				return *provider, nil
			}
		}
		return protocol.ProviderInfo{}, nil
	case protocol.MProvidersRemoveKey:
		var params protocol.RemoveKeyParams
		if err := decode(raw, &params); err != nil {
			return nil, err
		}
		for index := range s.data.snapshot.Providers {
			if s.data.snapshot.Providers[index].ID == params.Provider {
				provider := &s.data.snapshot.Providers[index]
				keys := provider.Keys[:0]
				for _, key := range provider.Keys {
					if key.ID != params.Key {
						keys = append(keys, key)
					}
				}
				provider.Keys = keys
				return *provider, nil
			}
		}
		return protocol.ProviderInfo{}, nil
	case protocol.MProvidersTest:
		var params protocol.ProviderParams
		if err := decode(raw, &params); err != nil {
			return nil, err
		}
		return s.testProvider(params.Provider), nil
	case protocol.MIndexSearch:
		return protocol.SearchResult{
			Index:    demoProjectHyperion,
			Semantic: true,
			Hits: []protocol.SearchHit{
				{
					Path:    "ryoku/rashin/internal/app/shell/run.go",
					Start:   42,
					End:     78,
					Score:   .94,
					Symbol:  "Run",
					Snippet: i18n.T("Run owns the Rashin client until its window closes."),
				},
				{
					Path:    "docs/rashin.md",
					Start:   591,
					End:     635,
					Score:   .87,
					Symbol:  i18n.T("The app"),
					Snippet: i18n.T("rashin-app is a Gio client drawn with the Lithic library."),
				},
			},
		}, nil
	case protocol.MIndexAdd, protocol.MIndexRefresh:
		if len(s.data.snapshot.Indexes) == 0 {
			return protocol.IndexInfo{}, nil
		}
		return s.data.snapshot.Indexes[0], nil
	case protocol.MIndexRemove:
		return protocol.Empty{}, nil
	case protocol.MProjectsWatch:
		return protocol.Empty{}, nil
	case protocol.MProjectsAdd:
		var params protocol.ProjectAddParams
		if err := decode(raw, &params); err != nil {
			return nil, err
		}
		info := protocol.ProjectInfo{
			ID:       "demo-added",
			Name:     params.Name,
			Root:     params.Path,
			Git:      true,
			BaseRef:  "origin/main",
			Branch:   "main",
			Created:  time.Now(),
			Activity: time.Now(),
		}
		if info.Name == "" {
			info.Name = filepath.Base(params.Path)
		}
		s.data.snapshot.Projects = append([]protocol.ProjectInfo{info}, s.data.snapshot.Projects...)
		return info, nil
	case protocol.MProjectsRemove:
		return protocol.Empty{}, nil
	case protocol.MProjectsSettings:
		var params protocol.ProjectSettingsParams
		if err := decode(raw, &params); err != nil {
			return nil, err
		}
		for index := range s.data.snapshot.Projects {
			if s.data.snapshot.Projects[index].ID == params.Project {
				if err := mergeInto(params.Patch, &s.data.snapshot.Projects[index].Settings); err != nil {
					return nil, err
				}
				return s.data.snapshot.Projects[index], nil
			}
		}
		return nil, notFound("project")
	case protocol.MWorktreesList:
		var params protocol.ProjectRef
		if err := decode(raw, &params); err != nil {
			return nil, err
		}
		return s.data.worktrees[params.Project], nil
	case protocol.MWorktreesCreate:
		var params protocol.WorktreeCreateParams
		if err := decode(raw, &params); err != nil {
			return nil, err
		}
		root := demoWorktrees + "/" + params.Project + "/" + params.Name
		info := protocol.WorktreeInfo{
			Path:    root,
			Project: params.Project,
			Name:    params.Name,
			Branch:  params.Branch,
			Head:    "demo-new",
			Owned:   true,
			Base:    params.Base,
			Setup:   protocol.SetupNone,
			Status:  &protocol.WorktreeStatus{Checked: time.Now()},
		}
		if info.Branch == "" {
			info.Branch = "rashin/" + params.Name
		}
		result := s.data.worktrees[params.Project]
		result.Project = params.Project
		result.Worktrees = append(result.Worktrees, info)
		s.data.worktrees[params.Project] = result
		return info, nil
	case protocol.MWorktreesPreviewRemove:
		var params protocol.WorktreeRef
		if err := decode(raw, &params); err != nil {
			return nil, err
		}
		return protocol.RemovePreview{
			Path:       params.Path,
			Branch:     s.worktree(params.Path).Branch,
			Dirty:      []string{"router.go"},
			Untracked:  []string{"capture.png"},
			NeedsForce: true,
			CanRemove:  true,
		}, nil
	case protocol.MWorktreesRemove:
		var params protocol.WorktreeRemoveParams
		if err := decode(raw, &params); err != nil {
			return nil, err
		}
		for project, result := range s.data.worktrees {
			kept := result.Worktrees[:0]
			for _, worktree := range result.Worktrees {
				if worktree.Path != params.Path {
					kept = append(kept, worktree)
				}
			}
			result.Worktrees = kept
			s.data.worktrees[project] = result
		}
		return protocol.Empty{}, nil
	case protocol.MWorktreesLock:
		var params protocol.WorktreeLockParams
		if err := decode(raw, &params); err != nil {
			return nil, err
		}
		return s.updateWorktree(
			params.Path,
			func(info *protocol.WorktreeInfo) { info.Locked, info.LockReason = params.Lock, params.Reason },
		), nil
	case protocol.MWorktreesMove:
		var params protocol.WorktreeMoveParams
		if err := decode(raw, &params); err != nil {
			return nil, err
		}
		return s.updateWorktree(params.Path, func(info *protocol.WorktreeInfo) { info.Path = params.To }), nil
	case protocol.MWorktreesRepair:
		var params protocol.ProjectRef
		if err := decode(raw, &params); err != nil {
			return nil, err
		}
		return s.data.worktrees[params.Project], nil
	case protocol.MWorktreesPrune:
		return protocol.PruneResult{Pruned: []string{"/tmp/missing-worktree"}}, nil
	case protocol.MWorktreesCleanup:
		var params protocol.CleanupParams
		if err := decode(raw, &params); err != nil {
			return nil, err
		}
		result := protocol.CleanupResult{}
		for _, worktree := range s.data.worktrees[params.Project].Worktrees {
			if worktree.Main {
				continue
			}
			result.Items = append(
				result.Items,
				protocol.CleanupItem{Path: worktree.Path, Removed: false, Reason: i18n.T("Kept because it has local work")},
			)
		}
		return result, nil
	case protocol.MWorktreesSetup:
		var params protocol.WorktreeSetupParams
		if err := decode(raw, &params); err != nil {
			return nil, err
		}
		setup := s.data.setups[params.Path]
		if params.Rerun {
			setup.State, setup.Log, setup.Exit = protocol.SetupRunning, i18n.T("Running setup.\n"), nil
			s.data.setups[params.Path] = setup
			exit := 0
			setup.State, setup.Log, setup.Exit = protocol.SetupDone, i18n.T("Setup complete.\n"), &exit
			s.data.setups[params.Path] = setup
		}
		return setup, nil
	case protocol.MWorktreesTrust:
		var params protocol.TrustParams
		if err := decode(raw, &params); err != nil {
			return nil, err
		}
		setup := s.data.setups[params.Path]
		setup.State, setup.Log = protocol.SetupDone, "Setup complete.\n"
		s.data.setups[params.Path] = setup
		return setup, nil
	case protocol.MWorktreesPorts:
		var params protocol.WorktreeRef
		if err := decode(raw, &params); err != nil {
			return nil, err
		}
		result := s.data.ports[params.Path]
		result.Path = params.Path
		return result, nil
	case protocol.MGitStatus:
		var params protocol.WorktreeRef
		if err := decode(raw, &params); err != nil {
			return nil, err
		}
		return s.data.status[params.Path], nil
	case protocol.MGitLog:
		var params protocol.GitLogParams
		if err := decode(raw, &params); err != nil {
			return nil, err
		}
		result := s.data.logs[params.Project]
		if params.Search != "" {
			filtered := result.Commits[:0]
			query := strings.ToLower(params.Search)
			for _, commit := range result.Commits {
				if strings.Contains(strings.ToLower(commit.Subject+commit.Author+commit.Hash), query) {
					filtered = append(filtered, commit)
				}
			}
			result.Commits = filtered
		}
		return result, nil
	case protocol.MGitShow:
		var params protocol.GitCommitParams
		if err := decode(raw, &params); err != nil {
			return nil, err
		}
		if result, ok := s.data.commits[params.Hash]; ok {
			return result, nil
		}
		return nil, notFound("commit")
	case protocol.MGitDiff:
		var params protocol.DiffParams
		if err := decode(raw, &params); err != nil {
			return nil, err
		}
		return s.data.diffs[params.Mode], nil
	case protocol.MGitStage, protocol.MGitUnstage, protocol.MGitDiscard:
		var params protocol.GitPathsParams
		if err := decode(raw, &params); err != nil {
			return nil, err
		}
		return s.data.status[params.Path], nil
	case protocol.MGitCommit, protocol.MGitPush, protocol.MGitFetch, protocol.MGitPull,
		protocol.MGitBranchCreate, protocol.MGitBranchDelete, protocol.MGitCheckout, protocol.MGitMerge,
		protocol.MGitCherryPick, protocol.MGitRevert, protocol.MGitRebase, protocol.MGitAbort, protocol.MGitContinue:
		return protocol.GitOpResult{OK: true, Output: i18n.T("The demo operation completed.")}, nil
	case protocol.MReviewList:
		var params protocol.WorktreeRef
		if err := decode(raw, &params); err != nil {
			return nil, err
		}
		result := s.data.reviews[params.Path]
		result.Worktree = params.Path
		return result, nil
	case protocol.MReviewAdd:
		var params protocol.ReviewAddParams
		if err := decode(raw, &params); err != nil {
			return nil, err
		}
		comment := protocol.ReviewComment{
			ID:       fmt.Sprintf("review-%d", len(s.data.reviews[params.Worktree].Comments)+1),
			Worktree: params.Worktree,
			Path:     params.Path,
			Line:     params.Line,
			Side:     params.Side,
			Text:     params.Text,
			Created:  time.Now(),
		}
		result := s.data.reviews[params.Worktree]
		result.Worktree = params.Worktree
		result.Comments = append(result.Comments, comment)
		s.data.reviews[params.Worktree] = result
		return comment, nil
	case protocol.MReviewResolve:
		var params protocol.ReviewRef
		if err := decode(raw, &params); err != nil {
			return nil, err
		}
		result := s.data.reviews[params.Worktree]
		for index := range result.Comments {
			if result.Comments[index].ID == params.ID {
				result.Comments[index].Resolved = params.Resolved
				s.data.reviews[params.Worktree] = result
				return result.Comments[index], nil
			}
		}
		return nil, notFound("review comment")
	case protocol.MReviewDelete:
		var params protocol.ReviewRef
		if err := decode(raw, &params); err != nil {
			return nil, err
		}
		result := s.data.reviews[params.Worktree]
		kept := result.Comments[:0]
		for _, comment := range result.Comments {
			if comment.ID != params.ID {
				kept = append(kept, comment)
			}
		}
		result.Comments = kept
		s.data.reviews[params.Worktree] = result
		return protocol.Empty{}, nil
	case protocol.MReviewSend:
		var params protocol.ReviewSendParams
		if err := decode(raw, &params); err != nil {
			return nil, err
		}
		for _, info := range s.data.snapshot.Sessions {
			if info.Worktree == params.Worktree && info.Kind == protocol.KindAgent {
				return info, nil
			}
		}
		return nil, notFound("agent session")
	case protocol.MTasksList:
		var params protocol.TasksParams
		if err := decode(raw, &params); err != nil {
			return nil, err
		}
		return s.data.tasks[params.Project], nil
	case protocol.MTasksCreate:
		var params protocol.TaskCreateParams
		if err := decode(raw, &params); err != nil {
			return nil, err
		}
		task := protocol.TaskInfo{
			ID:      fmt.Sprintf("task-%d", len(s.data.tasks[params.Project].Tasks)+1),
			Project: params.Project,
			Title:   params.Title,
			Goal:    params.Goal,
			Status:  protocol.TaskBacklog,
			Deps:    params.Deps,
			Harness: params.Harness,
			Created: time.Now(),
			Updated: time.Now(),
		}
		result := s.data.tasks[params.Project]
		result.Tasks = append(result.Tasks, task)
		s.data.tasks[params.Project] = result
		return task, nil
	case protocol.MTasksUpdate:
		var params protocol.TaskUpdateParams
		if err := decode(raw, &params); err != nil {
			return nil, err
		}
		for project, result := range s.data.tasks {
			for index := range result.Tasks {
				if result.Tasks[index].ID == params.ID {
					if err := mergeInto(params.Patch, &result.Tasks[index]); err != nil {
						return nil, err
					}
					result.Tasks[index].Updated = time.Now()
					s.data.tasks[project] = result
					return result.Tasks[index], nil
				}
			}
		}
		return nil, notFound("task")
	case protocol.MTasksDelete:
		var params protocol.TaskRef
		if err := decode(raw, &params); err != nil {
			return nil, err
		}
		for project, result := range s.data.tasks {
			kept := result.Tasks[:0]
			for _, task := range result.Tasks {
				if task.ID != params.ID {
					kept = append(kept, task)
				}
			}
			result.Tasks = kept
			s.data.tasks[project] = result
		}
		return protocol.Empty{}, nil
	case protocol.MTasksStart:
		var params protocol.TaskStartParams
		if err := decode(raw, &params); err != nil {
			return nil, err
		}
		task, ok := s.task(params.ID)
		if !ok {
			return nil, notFound("task")
		}
		path := demoWorktrees + "/" + task.Project + "/" + task.ID
		worktree := protocol.WorktreeInfo{
			Path:    path,
			Project: task.Project,
			Name:    task.ID,
			Branch:  "rashin/" + task.ID,
			Head:    "demo-task",
			Owned:   true,
		}
		session := s.newSession(
			protocol.KindAgent,
			params.Harness,
			worktree.Path,
			task.Project,
			worktree.Path,
			worktree.Branch,
			task.ID,
			"",
		)
		task.Status, task.Worktree, task.Sessions = protocol.TaskWorking, worktree.Path, []string{session.ID}
		s.replaceTask(task)
		return protocol.TaskStartResult{Task: task, Worktree: worktree, Session: session}, nil
	case protocol.MTasksGate:
		var params protocol.GateParams
		if err := decode(raw, &params); err != nil {
			return nil, err
		}
		task, ok := s.task(params.Task)
		if !ok {
			return nil, notFound("task")
		}
		task.Gates = append(
			task.Gates,
			protocol.Gate{
				ID:       fmt.Sprintf("gate-%d", len(task.Gates)+1),
				Question: params.Question,
				Options:  params.Options,
				Created:  time.Now(),
			},
		)
		task.Status = protocol.TaskNeedsYou
		s.replaceTask(task)
		return task, nil
	case protocol.MTasksResolveGate:
		var params protocol.GateResolveParams
		if err := decode(raw, &params); err != nil {
			return nil, err
		}
		task, ok := s.task(params.Task)
		if !ok {
			return nil, notFound("task")
		}
		for index := range task.Gates {
			if task.Gates[index].ID == params.Gate {
				task.Gates[index].Resolution = params.Resolution
			}
		}
		task.Status = protocol.TaskWorking
		s.replaceTask(task)
		return task, nil
	case protocol.MTasksPlan:
		return protocol.TaskPlanResult{
			Via: "demo/free",
			Drafts: []protocol.TaskDraft{
				{Title: i18n.T("Map the existing flow"), Goal: i18n.T("Read the current implementation.")},
				{Title: i18n.T("Build the change"), Goal: i18n.T("Implement and review the requested behavior."), Deps: []int{0}},
			},
		}, nil
	case protocol.MActivityList:
		var params protocol.ActivityParams
		if err := decode(raw, &params); err != nil {
			return nil, err
		}
		return s.data.activity[params.Project], nil
	case protocol.MUIOpen:
		return protocol.UIOpenResult{Delivered: true}, nil
	case protocol.MShutdown:
		return protocol.Empty{}, nil
	default:
		if teamsMethod(method) {
			return s.callTeams(method, raw)
		}
		return nil, protocol.Errorf(protocol.CodeUnsupported, "demo method %s is not implemented", method)
	}
}

func decode(raw json.RawMessage, target any) error {
	if len(raw) == 0 || string(raw) == "null" {
		return nil
	}
	return json.Unmarshal(raw, target)
}
func notFound(kind string) error {
	return protocol.Errorf(protocol.CodeNotFound, "The demo %s is not available.", kind)
}
func mergeInto(patch json.RawMessage, target any) error {
	current, err := json.Marshal(target)
	if err != nil {
		return err
	}
	var base, update map[string]any
	if err = json.Unmarshal(current, &base); err != nil {
		return err
	}
	if err = json.Unmarshal(patch, &update); err != nil {
		return err
	}
	mergeMap(base, update)
	merged, err := json.Marshal(base)
	if err != nil {
		return err
	}
	// a fresh value, so a key the patch removed from a map does not live on in the old one
	fresh := reflect.New(reflect.TypeOf(target).Elem())
	if err = json.Unmarshal(merged, fresh.Interface()); err != nil {
		return err
	}
	reflect.ValueOf(target).Elem().Set(fresh.Elem())
	return nil
}
func mergeMap(base, update map[string]any) {
	for key, value := range update {
		if value == nil {
			delete(base, key)
			continue
		}
		child, childOK := value.(map[string]any)
		existing, existingOK := base[key].(map[string]any)
		if childOK && existingOK {
			mergeMap(existing, child)
			base[key] = existing
		} else {
			base[key] = value
		}
	}
}
func (s *server) session(id string) (protocol.SessionInfo, bool) {
	for _, info := range s.data.snapshot.Sessions {
		if info.ID == id {
			return info, true
		}
	}
	if index := s.data.teams.sessionIndex(id); index >= 0 {
		return s.data.teams.sessions[index], true
	}
	return protocol.SessionInfo{}, false
}

// team sessions stay out of the snapshot's list, as the daemon leaves them out of the sessions it lists
func (s *server) replaceSession(info protocol.SessionInfo) {
	if info.Team != "" {
		if index := s.data.teams.sessionIndex(info.ID); index >= 0 {
			s.data.teams.sessions[index] = info
		} else {
			s.data.teams.sessions = append([]protocol.SessionInfo{info}, s.data.teams.sessions...)
		}
		return
	}
	for index := range s.data.snapshot.Sessions {
		if s.data.snapshot.Sessions[index].ID == info.ID {
			s.data.snapshot.Sessions[index] = info
			return
		}
	}
	s.data.snapshot.Sessions = append([]protocol.SessionInfo{info}, s.data.snapshot.Sessions...)
}
func (s *server) deleteSession(id string) {
	kept := s.data.snapshot.Sessions[:0]
	for _, info := range s.data.snapshot.Sessions {
		if info.ID != id {
			kept = append(kept, info)
		}
	}
	s.data.snapshot.Sessions = kept
	if index := s.data.teams.sessionIndex(id); index >= 0 {
		s.data.teams.sessions = append(s.data.teams.sessions[:index:index], s.data.teams.sessions[index+1:]...)
	}
	delete(s.data.events, id)
}
func (s *server) newSession(kind protocol.SessionKind, harness, cwd, project, worktree, branch, task, parent string) protocol.SessionInfo {
	id := fmt.Sprintf("%s-demo-%d", kind, len(s.data.snapshot.Sessions)+1)
	if worktree == "new" {
		worktree = demoWorktrees + "/" + project + "/demo-new"
	}
	if cwd == "" {
		cwd = worktree
	}
	info := protocol.SessionInfo{
		ID:       id,
		Kind:     kind,
		Title:    i18n.T("New session"),
		Harness:  harness,
		Model:    "default",
		Mode:     "default",
		Cwd:      cwd,
		Origin:   protocol.OriginApp,
		State:    protocol.StateIdle,
		Created:  time.Now(),
		Updated:  time.Now(),
		Project:  project,
		Worktree: worktree,
		Branch:   branch,
		Task:     task,
		Parent:   parent,
	}
	s.data.snapshot.Sessions = append([]protocol.SessionInfo{info}, s.data.snapshot.Sessions...)
	s.data.events[id] = []protocol.SessionEvent{event(1, protocol.EvTitle, protocol.TitleData{Title: info.Title})}
	return info
}
func (s *server) syncCustomHarnesses() {
	// what a probe learned about an instance stays with it
	learned := make(map[string]*protocol.HarnessCapabilities)
	builtins := s.data.snapshot.Harnesses[:0]
	for _, harness := range s.data.snapshot.Harnesses {
		if harness.Custom {
			learned[harness.ID] = harness.Capabilities
		} else {
			builtins = append(builtins, harness)
		}
	}
	for _, instance := range s.data.snapshot.Settings.Agent.Instances {
		builtins = append(
			builtins,
			protocol.HarnessInfo{
				ID:           instance.ID,
				Name:         instance.Name,
				Installed:    len(instance.Command) > 0,
				Command:      append([]string(nil), instance.Command...),
				Custom:       true,
				Capabilities: learned[instance.ID],
			},
		)
	}
	s.data.snapshot.Harnesses = builtins
}
func (s *server) worktree(path string) protocol.WorktreeInfo {
	for _, result := range s.data.worktrees {
		for _, worktree := range result.Worktrees {
			if worktree.Path == path {
				return worktree
			}
		}
	}
	return protocol.WorktreeInfo{Path: path}
}
func (s *server) updateWorktree(path string, update func(*protocol.WorktreeInfo)) protocol.WorktreeInfo {
	for project, result := range s.data.worktrees {
		for index := range result.Worktrees {
			if result.Worktrees[index].Path == path {
				update(&result.Worktrees[index])
				s.data.worktrees[project] = result
				return result.Worktrees[index]
			}
		}
	}
	return protocol.WorktreeInfo{Path: path}
}
func (s *server) task(id string) (protocol.TaskInfo, bool) {
	for _, result := range s.data.tasks {
		for _, task := range result.Tasks {
			if task.ID == id {
				return task, true
			}
		}
	}
	return protocol.TaskInfo{}, false
}
func (s *server) replaceTask(task protocol.TaskInfo) {
	result := s.data.tasks[task.Project]
	for index := range result.Tasks {
		if result.Tasks[index].ID == task.ID {
			result.Tasks[index] = task
			s.data.tasks[task.Project] = result
			return
		}
	}
}
