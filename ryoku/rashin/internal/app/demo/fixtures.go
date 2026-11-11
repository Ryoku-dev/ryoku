package demo

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"ryoku-i18n"

	"ryoku-rashin/internal/paths"
	"ryoku-rashin/internal/protocol"
)

const (
	demoProjectHyperion = "hyperion"
	demoProjectRyotunes = "ryotunes"
)

// under the real home so the app shortens the demo's folders like real ones
var (
	demoHome             = paths.Home()
	demoWorktrees        = demoHome + "/.local/share/ryoku/worktrees"
	demoHyperionRoot     = demoHome + "/Work/hyperion"
	demoRyotunesRoot     = demoHome + "/Work/ryotunes-gio"
	demoDirtyWorktree    = demoWorktrees + "/hyperion/settings-panel"
	demoConflictWorktree = demoWorktrees + "/hyperion/merge-conflict"
	demoLockedWorktree   = demoWorktrees + "/hyperion/docs-refresh"
)

type fixtures struct {
	snapshot  protocol.Snapshot
	events    map[string][]protocol.SessionEvent
	models    protocol.ModelsResult
	worktrees map[string]protocol.WorktreesResult
	tasks     map[string]protocol.TasksResult
	reviews   map[string]protocol.ReviewResult
	logs      map[string]protocol.GitLogResult
	commits   map[string]protocol.GitCommitDetail
	status    map[string]protocol.GitStatusResult
	diffs     map[string]protocol.DiffResult
	ports     map[string]protocol.PortsResult
	setups    map[string]protocol.SetupResult
	activity  map[string]protocol.ActivityResult
	teams     teamsFixtures
	// how often the demo's API key was replaced
	apiKeys int
}

// the moment the demo data is told from, a headless render starts its clock here so pictures don't depend on the time
var Epoch = time.Date(2026, 11, 10, 10, 45, 0, 0, time.UTC)

func makeFixtures() fixtures {
	now := Epoch
	ask := protocol.SessionInfo{
		ID:      "ask-20261110-094200-a17c",
		Kind:    protocol.KindAsk,
		Title:   i18n.T("Why did my build slow down?"),
		Model:   protocol.RouteAuto,
		Origin:  protocol.OriginApp,
		State:   protocol.StateIdle,
		Created: now.Add(-90 * time.Minute),
		Updated: now.Add(-12 * time.Minute),
		Turns:   2,
		LastSeq: 8,
		Preview: i18n.T("The indexer is rebuilding semantic vectors."),
	}
	askRunning := protocol.SessionInfo{
		ID: "ask-20261110-103800-live", Kind: protocol.KindAsk, Title: i18n.T("What is using the build cache?"),
		Model: protocol.RouteFast, Origin: protocol.OriginApp, State: protocol.StateWorking,
		Created: now.Add(-7 * time.Minute), Updated: now.Add(-time.Minute), Turns: 1, LastSeq: 8,
		Preview: i18n.T("Reading cache ownership and recent writes."), Alive: true,
	}
	askYesterday := protocol.SessionInfo{
		ID: "ask-20261109-161200-prev", Kind: protocol.KindAsk, Title: i18n.T("How do I switch to niri?"),
		Model: protocol.RouteAuto, Origin: protocol.OriginApp, State: protocol.StateIdle,
		Created: now.Add(-18 * time.Hour), Updated: now.Add(-17 * time.Hour), Turns: 1, LastSeq: 8,
	}
	askWeek := protocol.SessionInfo{
		ID: "ask-20261107-112500-week", Kind: protocol.KindAsk, Title: i18n.T("Where are provider keys stored?"),
		Model: protocol.RouteSmart, Origin: protocol.OriginApp, State: protocol.StateIdle,
		Created: now.Add(-72 * time.Hour), Updated: now.Add(-70 * time.Hour), Turns: 1, LastSeq: 8,
	}
	askEarlier := protocol.SessionInfo{
		ID: "ask-20261020-090000-old", Kind: protocol.KindAsk, Title: i18n.T("Explain the update channels"),
		Model: protocol.RouteAuto, Origin: protocol.OriginApp, State: protocol.StateIdle,
		Created: now.AddDate(0, 0, -21), Updated: now.AddDate(0, 0, -20), Turns: 1, LastSeq: 8,
	}
	needs := protocol.SessionInfo{
		ID:          "agent-20261110-101500-b82d",
		Kind:        protocol.KindAgent,
		Title:       i18n.T("Add the settings panel"),
		Harness:     "codex",
		Model:       "gpt-5.6",
		Mode:        "default",
		Cwd:         demoDirtyWorktree,
		Origin:      protocol.OriginApp,
		State:       protocol.StateWaiting,
		Created:     now.Add(-55 * time.Minute),
		Updated:     now.Add(-2 * time.Minute),
		Turns:       1,
		LastSeq:     17,
		Preview:     i18n.T("Waiting to edit the settings page."),
		Pending:     1,
		Alive:       true,
		Project:     demoProjectHyperion,
		Worktree:    demoDirtyWorktree,
		Branch:      "rashin/settings-panel",
		Task:        "task-settings",
		Activity:    i18n.T("Waiting for permission to edit settings"),
		Queue:       []protocol.QueuedPrompt{{ID: "queue-1", Text: i18n.T("Then update the Agent settings copy.")}},
		Checkpoints: true,
	}
	working := protocol.SessionInfo{
		ID:          "agent-working",
		Kind:        protocol.KindAgent,
		Title:       i18n.T("Build the commit graph"),
		Harness:     "claude",
		Model:       "sonnet",
		Mode:        "plan",
		Cwd:         demoHyperionRoot,
		Origin:      protocol.OriginApp,
		State:       protocol.StateWorking,
		Created:     now.Add(-40 * time.Minute),
		Updated:     now.Add(-time.Minute),
		Turns:       3,
		LastSeq:     6,
		Preview:     i18n.T("Laying out the octopus merge lanes."),
		Alive:       true,
		Project:     demoProjectHyperion,
		Worktree:    demoHyperionRoot,
		Branch:      "hyperion",
		Task:        "task-graph",
		Activity:    i18n.T("Reading git graph rows"),
		Checkpoints: true,
		Goal:        demoGoal(now),
	}
	done := protocol.SessionInfo{
		ID:       "agent-done",
		Kind:     protocol.KindAgent,
		Title:    i18n.T("Polish provider cards"),
		Harness:  "codex",
		Model:    "gpt-5.6",
		Mode:     "default",
		Cwd:      demoRyotunesRoot,
		Origin:   protocol.OriginApp,
		State:    protocol.StateIdle,
		Created:  now.Add(-3 * time.Hour),
		Updated:  now.Add(-20 * time.Minute),
		Turns:    2,
		LastSeq:  4,
		Preview:  i18n.T("Provider cards are ready for review."),
		Project:  demoProjectRyotunes,
		Worktree: demoRyotunesRoot,
		Branch:   "hyperion",
		Unread:   true,
	}
	idle := protocol.SessionInfo{
		ID:       "agent-idle",
		Kind:     protocol.KindAgent,
		Title:    i18n.T("Explore index cleanup"),
		Harness:  "custom-review",
		Model:    "default",
		Mode:     "default",
		Cwd:      demoLockedWorktree,
		Origin:   protocol.OriginApp,
		State:    protocol.StateStopped,
		Created:  now.Add(-24 * time.Hour),
		Updated:  now.Add(-5 * time.Hour),
		Turns:    1,
		LastSeq:  3,
		Preview:  i18n.T("Ready when you want to continue."),
		Project:  demoProjectHyperion,
		Worktree: demoLockedWorktree,
		Branch:   "rashin/docs-refresh",
	}
	failed := protocol.SessionInfo{
		ID:       "agent-conflict",
		Kind:     protocol.KindAgent,
		Title:    i18n.T("Resolve navigation conflict"),
		Harness:  "codex",
		Model:    "gpt-5.6",
		Mode:     "default",
		Cwd:      demoConflictWorktree,
		Origin:   protocol.OriginApp,
		State:    protocol.StateFailed,
		Created:  now.Add(-2 * time.Hour),
		Updated:  now.Add(-8 * time.Minute),
		Turns:    1,
		LastSeq:  4,
		Preview:  i18n.T("The merge stopped on a conflict."),
		Error:    i18n.T("Merge conflict in router.go"),
		Project:  demoProjectHyperion,
		Worktree: demoConflictWorktree,
		Branch:   "rashin/merge-conflict",
		Task:     "task-conflict",
	}

	settings := protocol.Settings{
		Enabled: true,
		Ask:     protocol.AskSettings{Model: protocol.RouteAuto, Habits: true},
		Agent: protocol.AgentSettings{
			Harness:         "codex",
			Approvals:       protocol.ApprovalsReadOnly,
			IdleStopMinutes: 10,
			Instances: []protocol.HarnessInstance{
				{
					ID:      "custom-review",
					Name:    i18n.T("Review account"),
					Command: []string{"codex-acp", "--profile", "review"},
					Env:     map[string]string{"CODEX_HOME": demoHome + "/.config/codex-review"},
				},
			},
		},
		Daemon: protocol.DaemonSettings{IdleExitMinutes: 5},
		Index:  protocol.IndexSettings{System: true, Semantic: true},
		API:    protocol.APISettings{Enabled: true, Port: protocol.DefaultAPIPort},
	}
	provider := protocol.ProviderInfo{
		ID:         "openrouter",
		Name:       "OpenRouter",
		Tier:       "free",
		Enabled:    true,
		Configured: true,
		Models:     8,
		Available:  6,
		Signup:     "https://openrouter.ai/keys",
		KeyHint:    "sk-or-v1-...",
		Keys:       []protocol.KeyInfo{{ID: "demo", Label: i18n.T("Main"), Status: protocol.KeyHealthy, Masked: "sk-or-••••93af"}},
	}
	keyless := protocol.ProviderInfo{
		ID:         "puter",
		Name:       "Puter",
		Tier:       "renewable",
		Keyless:    true,
		Enabled:    true,
		Configured: true,
		Models:     3,
		Available:  3,
		Signup:     "https://puter.com",
	}
	indexes := []protocol.IndexInfo{
		{
			ID:      demoProjectHyperion,
			Name:    "hyperion",
			Root:    demoHyperionRoot,
			Kind:    protocol.IndexProject,
			State:   protocol.IndexReady,
			Files:   4812,
			Chunks:  29840,
			Symbols: 17604,
			Vectors: 29840,
			Updated: &now,
		},
		{
			ID:      demoProjectRyotunes,
			Name:    "ryotunes-gio",
			Root:    demoRyotunesRoot,
			Kind:    protocol.IndexProject,
			State:   protocol.IndexIndexing,
			Files:   1320,
			Chunks:  6910,
			Symbols: 4102,
			Vectors: 4120,
			Updated: &now,
		},
	}
	projects := []protocol.ProjectInfo{
		{
			ID:        demoProjectHyperion,
			Name:      "hyperion",
			Root:      demoHyperionRoot,
			Git:       true,
			BaseRef:   "origin/hyperion",
			Branch:    "hyperion",
			Worktrees: 4,
			Sessions:  4,
			Tasks:     7,
			Attention: 3,
			Settings: protocol.ProjectSettings{
				BaseRef:      "origin/hyperion",
				WorktreeRoot: demoWorktrees + "/hyperion",
				BranchPrefix: "rashin/",
				Setup:        "go mod download",
				CopyIgnored:  []string{".env"},
				Cleanup:      protocol.CleanupPolicy{Protect: protocol.ProtectAny},
			},
			Created:  now.AddDate(0, -3, 0),
			Activity: now.Add(-time.Minute),
		},
		{
			ID:        demoProjectRyotunes,
			Name:      "ryotunes-gio",
			Root:      demoRyotunesRoot,
			Git:       true,
			BaseRef:   "origin/hyperion",
			Branch:    "hyperion",
			Worktrees: 1,
			Sessions:  1,
			Tasks:     1,
			Settings: protocol.ProjectSettings{
				BaseRef:      "origin/hyperion",
				BranchPrefix: "rashin/",
				Cleanup:      protocol.CleanupPolicy{Protect: protocol.ProtectUncommitted},
			},
			Created:  now.AddDate(0, -2, 0),
			Activity: now.Add(-20 * time.Minute),
		},
	}

	clean := &protocol.WorktreeStatus{Upstream: "origin/hyperion", Ahead: 2, AheadBase: 2, Added: 438, Removed: 91, Checked: now}
	dirtyStatus := &protocol.WorktreeStatus{
		Upstream:   "origin/rashin/settings-panel",
		Ahead:      3,
		Behind:     1,
		AheadBase:  3,
		BehindBase: 1,
		Staged:     2,
		Modified:   3,
		Untracked:  1,
		Added:      286,
		Removed:    43,
		Checked:    now,
	}
	conflictStatus := &protocol.WorktreeStatus{
		AheadBase:  1,
		Modified:   1,
		Conflicted: 2,
		Operation:  protocol.OpMerge,
		Added:      31,
		Removed:    17,
		Checked:    now,
	}
	lockedStatus := &protocol.WorktreeStatus{Upstream: "origin/rashin/docs-refresh", Behind: 2, BehindBase: 2, Checked: now}
	created := now.Add(-48 * time.Hour)
	activityTime := now.Add(-2 * time.Minute)
	hyperionWorktrees := []protocol.WorktreeInfo{
		{
			Path:     demoHyperionRoot,
			Project:  demoProjectHyperion,
			Name:     "hyperion",
			Branch:   "hyperion",
			Head:     "c8main0000000000000000000000000000000000",
			Main:     true,
			Base:     "origin/hyperion",
			BaseSha:  "90base0000000000000000000000000000000000",
			Sessions: []string{working.ID},
			Status:   clean,
			Setup:    protocol.SetupDone,
			Activity: &activityTime,
		},
		{
			Path:     demoDirtyWorktree,
			Project:  demoProjectHyperion,
			Name:     "settings-panel",
			Branch:   "rashin/settings-panel",
			Head:     "b7merge000000000000000000000000000000000",
			Owned:    true,
			Base:     "origin/hyperion",
			BaseSha:  "90base0000000000000000000000000000000000",
			Task:     "task-settings",
			Sessions: []string{needs.ID},
			Status:   dirtyStatus,
			Setup:    protocol.SetupRunning,
			Created:  &created,
			Activity: &activityTime,
		},
		{
			Path:     demoConflictWorktree,
			Project:  demoProjectHyperion,
			Name:     "merge-conflict",
			Branch:   "rashin/merge-conflict",
			Head:     "e6index000000000000000000000000000000000",
			Owned:    true,
			Base:     "origin/hyperion",
			BaseSha:  "90base0000000000000000000000000000000000",
			Task:     "task-conflict",
			Sessions: []string{failed.ID},
			Status:   conflictStatus,
			Setup:    protocol.SetupFailed,
			Created:  &created,
			Activity: &activityTime,
		},
		{
			Path:       demoLockedWorktree,
			Project:    demoProjectHyperion,
			Name:       "docs-refresh",
			Branch:     "rashin/docs-refresh",
			Head:       "f5docs0000000000000000000000000000000000",
			Owned:      true,
			Locked:     true,
			LockReason: i18n.T("Keep for the release notes"),
			Base:       "origin/hyperion",
			BaseSha:    "90base0000000000000000000000000000000000",
			Sessions:   []string{idle.ID},
			Status:     lockedStatus,
			Setup:      protocol.SetupUntrusted,
			Created:    &created,
			Activity:   &activityTime,
		},
	}
	ryotunesWorktrees := []protocol.WorktreeInfo{
		{
			Path:     demoRyotunesRoot,
			Project:  demoProjectRyotunes,
			Name:     "ryotunes-gio",
			Branch:   "hyperion",
			Head:     "r3music",
			Main:     true,
			Base:     "origin/hyperion",
			BaseSha:  "r0base",
			Sessions: []string{done.ID},
			Status:   &protocol.WorktreeStatus{Upstream: "origin/hyperion", Checked: now},
			Setup:    protocol.SetupNone,
			Activity: &activityTime,
		},
	}

	tasks := demoTasks(now)
	commits, refs := demoCommits(now)
	log := protocol.GitLogResult{
		Commits: commits,
		Refs:    refs,
		Wip: []protocol.WipRow{
			{
				Worktree:  demoDirtyWorktree,
				Branch:    "rashin/settings-panel",
				Head:      commits[1].Hash,
				Staged:    2,
				Modified:  3,
				Untracked: 1,
				Agent:     "waiting",
			},
			{
				Worktree:   demoConflictWorktree,
				Branch:     "rashin/merge-conflict",
				Head:       commits[3].Hash,
				Modified:   1,
				Conflicted: 2,
				Agent:      "failed",
			},
			{Worktree: demoHyperionRoot, Branch: "hyperion", Head: commits[0].Hash, Agent: "working"},
		},
	}
	diff := demoDiff()
	review := protocol.ReviewResult{
		Worktree: demoDirtyWorktree,
		Comments: []protocol.ReviewComment{
			{
				ID:       "review-1",
				Worktree: demoDirtyWorktree,
				Path:     "ryoku/rashin/internal/app/shell/router.go",
				Line:     102,
				Side:     "new",
				Text:     i18n.T("Keep the Needs you count as a word and number."),
				Created:  now.Add(-10 * time.Minute),
			},
			{
				ID:       "review-2",
				Worktree: demoDirtyWorktree,
				Path:     "docs/rashin.md",
				Line:     602,
				Side:     "new",
				Text:     i18n.T("This matches the app contract."),
				Resolved: true,
				Created:  now.Add(-8 * time.Minute),
			},
		},
	}

	result := fixtures{
		snapshot: protocol.Snapshot{
			Status: protocol.Status{
				Version:   "2.0.0-demo",
				Enabled:   true,
				Running:   true,
				Clients:   1,
				Sessions:  protocol.SessionCounts{Ask: 1, Agent: 5, Working: 2, Alive: 2},
				Gateway:   protocol.GatewayStatus{Providers: 2, Models: 9, Keyless: 1},
				API:       demoAPI(settings.API),
				Harnesses: 3,
				Indexes:   2,
			},
			Sessions:  []protocol.SessionInfo{working, needs, failed, done, idle, ask},
			Settings:  settings,
			Providers: []protocol.ProviderInfo{provider, keyless},
			Harnesses: []protocol.HarnessInfo{
				{
					ID:        "codex",
					Name:      "Codex",
					Installed: true,
					Version:   "0.62.0",
					Default:   true,
					Capabilities: &protocol.HarnessCapabilities{
						LoadSession: true,
						Images:      true,
						Context:     true,
						Models:      true,
						Modes:       []protocol.Choice{{ID: "default", Name: i18n.T("Default")}, {ID: "plan", Name: i18n.T("Plan")}},
						Started:     []protocol.HarnessFact{{Label: i18n.T("Model"), Value: "gpt-5.6-sol"}, {Label: i18n.T("Thinking"), Value: i18n.T("High")}},
					},
				},
				{
					ID:           "claude",
					Name:         "Claude Code",
					Installed:    true,
					Version:      "2.1.3",
					Capabilities: &protocol.HarnessCapabilities{LoadSession: true, Images: true, Models: true},
					Setup: []protocol.HarnessFact{
						{Label: i18n.T("Model"), Value: "anthropic/claude-opus-5-5"},
						{Label: i18n.Tf("%s model", "smol"), Value: "anthropic/claude-haiku-5"},
					},
				},
				{
					ID:        "custom-review",
					Name:      i18n.T("Review account"),
					Installed: true,
					Custom:    true,
					Command:   []string{"codex-acp", "--profile", "review"},
				},
				{ID: "gemini", Name: "Gemini CLI", Installed: false},
			},
			Indexes:  indexes,
			Projects: projects,
			Tasks:    append(append([]protocol.TaskInfo(nil), tasks[demoProjectHyperion].Tasks...), tasks[demoProjectRyotunes].Tasks...),
		},
		events: make(map[string][]protocol.SessionEvent),
		models: protocol.ModelsResult{
			Routes: []protocol.Choice{
				{ID: protocol.RouteAuto, Name: i18n.T("Auto")},
				{ID: protocol.RouteFast, Name: i18n.T("Fast")},
				{ID: protocol.RouteSmart, Name: i18n.T("Smart")},
			},
			Models: []protocol.ModelInfo{
				{ID: "free", Provider: "openrouter", Name: i18n.T("Free router"), Available: true, Tools: true},
				{ID: "gpt-oss-120b", Provider: "puter", Name: "GPT OSS 120B", Available: true, Tools: true},
			},
		},
		worktrees: map[string]protocol.WorktreesResult{
			demoProjectHyperion: {Project: demoProjectHyperion, Worktrees: hyperionWorktrees},
			demoProjectRyotunes: {Project: demoProjectRyotunes, Worktrees: ryotunesWorktrees},
		},
		tasks:   tasks,
		reviews: map[string]protocol.ReviewResult{demoDirtyWorktree: review, demoConflictWorktree: {Worktree: demoConflictWorktree}},
		logs: map[string]protocol.GitLogResult{
			demoProjectHyperion: log,
			demoProjectRyotunes: {
				Commits: commits[:3],
				Refs: []protocol.GitRef{
					{Name: "hyperion", Kind: protocol.RefBranch, Target: commits[0].Hash, Worktree: demoRyotunesRoot, Current: true},
					{Name: "origin/hyperion", Kind: protocol.RefRemote, Target: commits[0].Hash},
				},
			},
		},
		commits: make(map[string]protocol.GitCommitDetail),
		status:  make(map[string]protocol.GitStatusResult),
		diffs: map[string]protocol.DiffResult{
			protocol.DiffWorking:    diff,
			protocol.DiffStaged:     diff,
			protocol.DiffBase:       diff,
			protocol.DiffCommit:     diff,
			protocol.DiffRange:      diff,
			protocol.DiffCheckpoint: diff,
		},
		ports: map[string]protocol.PortsResult{
			demoDirtyWorktree: {
				Path: demoDirtyWorktree,
				Ports: []protocol.Port{
					{Port: 5173, PID: 43120, Command: "vite", URL: "http://localhost:5173"},
					{Port: 8080, PID: 43144, Command: "go run ./cmd/rashind", URL: "http://localhost:8080"},
				},
			},
		},
		setups: map[string]protocol.SetupResult{
			demoHyperionRoot: {
				Path:    demoHyperionRoot,
				State:   protocol.SetupDone,
				Command: "go mod download",
				Log:     "Dependencies are ready.\n",
			},
			demoDirtyWorktree: {
				Path:    demoDirtyWorktree,
				State:   protocol.SetupRunning,
				Command: "go mod download",
				Log:     "Downloading modules...\n",
			},
			demoConflictWorktree: {
				Path:    demoConflictWorktree,
				State:   protocol.SetupFailed,
				Command: "go mod download",
				Log:     "module download failed\n",
				Exit:    intPointer(1),
			},
			demoLockedWorktree: {Path: demoLockedWorktree, State: protocol.SetupUntrusted, Script: ".rashin/setup", Hash: "d34db33f"},
		},
		activity: map[string]protocol.ActivityResult{
			demoProjectHyperion: {
				Activity: []protocol.Activity{
					{Time: now.Add(-time.Minute), Actor: working.ID, Action: "git.commit", Target: demoHyperionRoot, OK: true},
					{
						Time: now.Add(-8 * time.Minute), Actor: failed.ID, Action: "git.merge", Target: demoConflictWorktree,
						Detail: i18n.T("Stopped on two conflicts"),
					},
					{Time: now.Add(-12 * time.Minute), Actor: "you", Action: "review.send", Target: demoDirtyWorktree, OK: true},
					{
						Time: now.Add(-20 * time.Minute), Actor: "you", Action: "worktree.lock", Target: demoLockedWorktree, OK: true,
						Detail: i18n.T("Keep for the release notes"),
					},
					{Time: now.Add(-47 * time.Minute), Actor: "you", Action: "task.start", Target: "task-settings", OK: true},
					{Time: now.Add(-52 * time.Minute), Actor: "you", Action: "worktree.create", Target: demoDirtyWorktree, OK: true},
					{Time: now.Add(-3 * time.Hour), Actor: "you", Action: "git.fetch", Target: demoHyperionRoot, OK: true},
					{Time: now.Add(-26 * time.Hour), Actor: "you", Action: "project.add", Target: demoHyperionRoot, OK: true},
				},
			},
		},
	}
	result.snapshot.Sessions = append(result.snapshot.Sessions, askRunning, askYesterday, askWeek, askEarlier)
	result.snapshot.Status.Sessions.Ask = 5
	result.snapshot.Status.Sessions.Working = 3
	result.snapshot.Status.Sessions.Alive = 3
	result.snapshot.Providers = append(result.snapshot.Providers, demoProviderCatalogue()...)
	result.snapshot.Status.Gateway = protocol.GatewayStatus{Providers: 6, Models: 25, Keyless: 3}
	for _, commit := range commits {
		result.commits[commit.Hash] = protocol.GitCommitDetail{
			Commit: commit,
			Body:   i18n.T("A deterministic demo commit with files and a diff."),
			Files:  diffFiles(diff),
		}
	}
	for _, worktree := range append(append([]protocol.WorktreeInfo(nil), hyperionWorktrees...), ryotunesWorktrees...) {
		status := protocol.WorktreeStatus{}
		if worktree.Status != nil {
			status = *worktree.Status
		}
		result.status[worktree.Path] = protocol.GitStatusResult{
			Path:   worktree.Path,
			Branch: worktree.Branch,
			Head:   worktree.Head,
			Status: status,
			Files:  diffFiles(diff),
		}
	}
	result.events[ask.ID] = askEvents(ask)
	result.events[askRunning.ID] = askEvents(askRunning)
	result.events[askYesterday.ID] = askEvents(askYesterday)
	result.events[askWeek.ID] = askEvents(askWeek)
	result.events[askEarlier.ID] = askEvents(askEarlier)
	result.events[needs.ID] = agentEvents(needs, true)
	result.events[working.ID] = agentEvents(working, false)
	result.events[working.ID] = append(result.events[working.ID], subagentEvents(int64(len(result.events[working.ID])), now)...)
	result.events[done.ID] = agentEvents(done, false)
	result.events[done.ID] = append(result.events[done.ID], event(int64(len(result.events[done.ID])+1), protocol.EvTurnEnd, protocol.TurnEndData{Turn: 1, Stop: protocol.StopEnd}))
	result.events[idle.ID] = agentEvents(idle, false)
	result.events[failed.ID] = agentEvents(failed, false)
	result.teams = demoTeams(now, result.events)
	return result
}

func demoProviderCatalogue() []protocol.ProviderInfo {
	providers := []protocol.ProviderInfo{
		{
			ID:         "groq",
			Name:       "Groq",
			Tier:       "free",
			Enabled:    true,
			Configured: true,
			Models:     4,
			Available:  4,
			Signup:     "https://console.groq.com/keys",
			KeyHint:    "gsk_...",
			Keys:       []protocol.KeyInfo{{ID: "groq-demo", Label: i18n.T("Main"), Status: protocol.KeyHealthy, Masked: "gsk_••••19ac"}},
		},
		{
			ID:         "nvidia",
			Name:       "NVIDIA",
			Tier:       "free",
			Enabled:    true,
			Configured: true,
			Models:     7,
			Available:  5,
			Signup:     "https://build.nvidia.com/",
			KeyHint:    "nvapi-...",
			Keys: []protocol.KeyInfo{
				{ID: "nvidia-demo", Label: i18n.T("Main"), Status: protocol.KeyHealthy, Masked: "nvapi-••••4b31"},
			},
		},
		{ID: "ovh", Name: "OVHcloud", Tier: "renewable", Keyless: true, Enabled: true, Configured: true, Models: 3, Available: 3},
		{ID: "kilo", Name: "Kilo", Tier: "renewable", Keyless: true, Enabled: true, Configured: true, Models: 2, Available: 2},
	}
	catalogue := []string{
		"Anthropic", "Cerebras", "Chutes", "Cloudflare", "Cohere", "DeepInfra",
		"DeepSeek", "Fireworks", "GitHub Models", "Google AI Studio", "Hugging Face",
		"Mistral", "Moonshot", "Nebius", "Novita", "OpenAI", "Perplexity", "Qwen",
		"SambaNova", "Together", "xAI", "Zhipu",
	}
	for index, name := range catalogue {
		id := strings.ToLower(strings.NewReplacer(" ", "-", "_", "-").Replace(name))
		providers = append(providers, protocol.ProviderInfo{
			ID: id, Name: name, Tier: "catalogue", Models: 2 + index%5, KeyHint: "key-...",
		})
	}
	return providers
}

func demoTasks(now time.Time) map[string]protocol.TasksResult {
	gate := protocol.Gate{
		ID:       "gate-layout",
		Question: i18n.T("Which compact layout should ship?"),
		Options:  []string{i18n.T("Bottom bar"), i18n.T("Rail")},
		Created:  now.Add(-time.Hour),
	}
	makeTask := func(id, title, status string, order int) protocol.TaskInfo {
		return protocol.TaskInfo{
			ID:      id,
			Project: demoProjectHyperion,
			Title:   title,
			Goal:    title,
			Status:  status,
			Order:   order,
			Created: now.Add(-24 * time.Hour),
			Updated: now.Add(-time.Duration(order) * time.Minute),
		}
	}
	backlog := makeTask("task-backlog", i18n.T("Document command palette"), protocol.TaskBacklog, 0)
	ready := makeTask("task-ready", i18n.T("Add project search"), protocol.TaskReady, 1)
	ready.Deps = []string{backlog.ID}
	working := makeTask("task-graph", i18n.T("Build the commit graph"), protocol.TaskWorking, 2)
	working.Worktree, working.Sessions, working.Harness = demoHyperionRoot, []string{"agent-working"}, "claude"
	needs := makeTask("task-settings", i18n.T("Add the settings panel"), protocol.TaskNeedsYou, 3)
	needs.Worktree, needs.Sessions, needs.Gates = demoDirtyWorktree, []string{"agent-20261110-101500-b82d"}, []protocol.Gate{gate}
	review := makeTask("task-review", i18n.T("Review provider cards"), protocol.TaskReview, 4)
	review.Products = protocol.TaskProducts{Branch: "rashin/providers", Commits: 2, Files: 8, Added: 210, Removed: 42}
	done := makeTask("task-done", i18n.T("Seed demo sessions"), protocol.TaskDone, 5)
	blocked := makeTask("task-conflict", i18n.T("Resolve navigation conflict"), protocol.TaskBlocked, 6)
	blocked.Deps = []string{needs.ID}
	blocked.Worktree, blocked.Sessions = demoConflictWorktree, []string{"agent-conflict"}
	music := protocol.TaskInfo{
		ID:      "task-music",
		Project: demoProjectRyotunes,
		Title:   i18n.T("Tune compact player"),
		Goal:    i18n.T("Keep the player precise at compact sizes."),
		Status:  protocol.TaskReady,
		Created: now.Add(-8 * time.Hour),
		Updated: now.Add(-20 * time.Minute),
	}
	return map[string]protocol.TasksResult{
		demoProjectHyperion: {Tasks: []protocol.TaskInfo{backlog, ready, working, needs, review, done, blocked}},
		demoProjectRyotunes: {Tasks: []protocol.TaskInfo{music}},
	}
}

func demoCommits(now time.Time) ([]protocol.GitCommit, []protocol.GitRef) {
	commits := []protocol.GitCommit{
		{
			Hash: "c8main0000000000000000000000000000000000",
			Parents: []string{
				"b7merge000000000000000000000000000000000",
				"f5docs0000000000000000000000000000000000",
				"e6index000000000000000000000000000000000",
			},
			Author:  "Ren",
			Time:    now.Add(-time.Hour),
			Subject: i18n.T("Merge the Hyperion application wave"),
		},
		{
			Hash:    "b7merge000000000000000000000000000000000",
			Parents: []string{"a4shell000000000000000000000000000000000", "d7agent000000000000000000000000000000000"},
			Author:  "Ren",
			Time:    now.Add(-2 * time.Hour),
			Subject: i18n.T("Merge the agent hub"),
		},
		{
			Hash:    "f5docs0000000000000000000000000000000000",
			Parents: []string{"a4shell000000000000000000000000000000000"},
			Author:  "Aki",
			Time:    now.Add(-3 * time.Hour),
			Subject: i18n.T("Refresh the Work documentation"),
		},
		{
			Hash:    "e6index000000000000000000000000000000000",
			Parents: []string{"a4shell000000000000000000000000000000000"},
			Author:  "Ren",
			Time:    now.Add(-4 * time.Hour),
			Subject: i18n.T("Add project index state"),
		},
		{
			Hash:    "d7agent000000000000000000000000000000000",
			Parents: []string{"a4shell000000000000000000000000000000000"},
			Author:  "Agent",
			Time:    now.Add(-5 * time.Hour),
			Subject: i18n.T("Build the session hub"),
		},
		{
			Hash:    "a4shell000000000000000000000000000000000",
			Parents: []string{"90base0000000000000000000000000000000000"},
			Author:  "Ren",
			Time:    now.Add(-7 * time.Hour),
			Subject: i18n.T("Create the Lithic app shell"),
		},
		{
			Hash:    "90base0000000000000000000000000000000000",
			Parents: []string{fmt.Sprintf("%040x", 1)},
			Author:  "Ren",
			Time:    now.Add(-24 * time.Hour),
			Subject: i18n.T("Start Rashin Work"),
		},
	}
	history := make([]string, 58)
	for index := range history {
		history[index] = fmt.Sprintf("%040x", index+1)
	}
	subjects := []string{
		i18n.T("Refine project discovery"),
		i18n.T("Keep worktree state durable"),
		i18n.T("Teach the index about symbols"),
		i18n.T("Polish the everyday Ask flow"),
		i18n.T("Record agent activity"),
		i18n.T("Tighten provider health checks"),
	}
	for index, hash := range history {
		parents := []string{}
		if index+1 < len(history) {
			parents = append(parents, history[index+1])
		}
		switch index {
		case 8:
			parents = append(parents, history[index+4])
		case 19:
			parents = append(parents, history[index+5])
		case 31:
			parents = append(parents, history[index+3], history[index+6])
		}
		commits = append(commits, protocol.GitCommit{
			Hash: hash, Parents: parents, Author: []string{"Ren", "Aki", "Agent"}[index%3],
			Time: now.Add(-time.Duration(25+index*3) * time.Hour), Subject: subjects[index%len(subjects)],
		})
	}
	refs := []protocol.GitRef{
		{
			Name:     "hyperion",
			Kind:     protocol.RefBranch,
			Target:   commits[0].Hash,
			Worktree: demoHyperionRoot,
			Current:  true,
			Upstream: "origin/hyperion",
		},
		{Name: "rashin/settings-panel", Kind: protocol.RefBranch, Target: commits[1].Hash, Worktree: demoDirtyWorktree},
		{Name: "rashin/merge-conflict", Kind: protocol.RefBranch, Target: commits[3].Hash, Worktree: demoConflictWorktree},
		{Name: "origin/hyperion", Kind: protocol.RefRemote, Target: commits[0].Hash},
		{Name: "origin/main", Kind: protocol.RefRemote, Target: commits[5].Hash},
		{Name: "origin/rashin/index", Kind: protocol.RefRemote, Target: commits[22].Hash},
		{Name: "v2.0.0", Kind: protocol.RefTag, Target: commits[5].Hash},
		{Name: "v1.8.0", Kind: protocol.RefTag, Target: commits[38].Hash},
		{Name: "HEAD", Kind: protocol.RefHead, Target: commits[0].Hash, Current: true},
	}
	return commits, refs
}

func demoDiff() protocol.DiffResult {
	return protocol.DiffResult{Files: []protocol.FileDiff{
		{
			Path: "ryoku/rashin/internal/app/shell/router.go", Status: protocol.ChangeModified, Added: 5, Removed: 2,
			Hunks: []protocol.Hunk{{
				Header: "@@ -98,6 +98,9 @@", OldStart: 98, OldLines: 6, NewStart: 98, NewLines: 9,
				Lines: []protocol.DiffLine{
					{Kind: protocol.LineContext, Text: "func (r *Router) LayoutRail(...) {", Old: 98, New: 98},
					{Kind: protocol.LineDel, Text: "labels := []string{\"Ask\", \"Agent\"}", Old: 99},
					{Kind: protocol.LineAdd, Text: "labels := []string{\"Ask\", \"Agents\", \"Projects\"}", New: 99},
					{Kind: protocol.LineAdd, Text: "labels[1] = fmt.Sprintf(\"Needs you %d\", count)", New: 100},
					{Kind: protocol.LineContext, Text: "return layout.Flex{...}", Old: 100, New: 101},
				},
			}},
		},
		{
			Path: "docs/rashin.md", Status: protocol.ChangeModified, Added: 3, Removed: 1,
			Hunks: []protocol.Hunk{{
				Header: "@@ -600,3 +600,5 @@", OldStart: 600, OldLines: 3, NewStart: 600, NewLines: 5,
				Lines: []protocol.DiffLine{
					{Kind: protocol.LineContext, Text: "- Layout: rail with Ask as home", Old: 600, New: 600},
					{Kind: protocol.LineDel, Text: "- Agent sessions", Old: 601},
					{Kind: protocol.LineAdd, Text: "- Agents show Needs you, Working, Done and Idle", New: 601},
				},
			}},
		},
	}}
}
func diffFiles(diff protocol.DiffResult) []protocol.FileChange {
	result := make([]protocol.FileChange, len(diff.Files))
	for i, file := range diff.Files {
		result[i] = protocol.FileChange{
			Path:    file.Path,
			OldPath: file.OldPath,
			Status:  file.Status,
			Added:   file.Added,
			Removed: file.Removed,
			Binary:  file.Binary,
		}
	}
	return result
}
func askEvents(info protocol.SessionInfo) []protocol.SessionEvent {
	answer := i18n.T(`# Why the build changed

The **semantic index** is rebuilding after the palette update. The cache remains safe while this runs.

1. The source scan reads changed packages.
2. Symbol extraction refreshes affected files.
3. Vector work follows after the graph settles.

| Stage | State | Files |
| --- | --- | ---: |
| Scan | Done | 4,812 |
| Symbols | Running | 438 |
| Vectors | Waiting | 286 |

> Keep the project open so the incremental watcher can finish.

` + "```sh\nrashin index list\nrashin doctor\n```\n\n" + `CPU use will settle when the project index finishes.`)
	plan := &protocol.CommandPlan{
		Summary: i18n.T("Inspect the index and choose the narrowest repair."),
		Commands: []protocol.PlannedCommand{
			{Command: "rashin index list", Why: i18n.T("Read index progress."), Tier: protocol.TierRead},
			{Command: "rashin index refresh hyperion", Why: i18n.T("Refresh project data."), Tier: protocol.TierWrite},
			{Command: "sudo systemctl restart sddm", Why: i18n.T("Restart the login screen."), Tier: protocol.TierSystem, Terminal: true},
			{Command: "rm -rf ~/.cache/ryoku/rashin/index", Why: i18n.T("Delete cached index data."), Tier: protocol.TierDanger},
		},
	}
	return []protocol.SessionEvent{
		event(1, protocol.EvUser, protocol.UserData{
			Text:   i18n.T("Why did my build slow down?"),
			Images: []protocol.Attachment{{Path: "/tmp/build-profile.png", Mime: "image/png"}},
		}),
		event(2, protocol.EvTurnStart, protocol.TurnStartData{Turn: 1}),
		event(3, protocol.EvThought, protocol.TextData{
			Message: "thought-1", Delta: i18n.T("Comparing recent build stages with index state."),
		}),
		event(4, protocol.EvText, protocol.TextData{Message: "answer-1", Delta: answer}),
		event(5, protocol.EvAnswer, protocol.AnswerData{
			Via: "openrouter/free", NeedsAgent: true,
			Reason: i18n.T("An agent can inspect the build graph if the slowdown continues."),
			Actions: []protocol.Action{
				{Kind: protocol.ActCopy, Label: i18n.T("Copy command"), Value: "rashin index list"},
				{Kind: protocol.ActFile, Label: i18n.T("Open index state"), Value: demoHome + "/.cache/ryoku/rashin/index"},
			},
			Plan: plan,
		}),
		event(6, protocol.EvError, protocol.ErrorData{
			Message: i18n.T("One optional provider timed out. Try again after the index settles."),
			Code:    "provider_timeout",
		}),
		event(7, protocol.EvTurnEnd, protocol.TurnEndData{
			Turn: 1, Stop: protocol.StopEnd, Usage: &protocol.TokenUsage{Input: 2322, Output: 279},
		}),
		event(8, protocol.EvTitle, protocol.TitleData{Title: info.Title}),
	}
}

func agentEvents(info protocol.SessionInfo, permission bool) []protocol.SessionEvent {
	events := []protocol.SessionEvent{
		event(1, protocol.EvUser, protocol.UserData{
			Text:   i18n.T("Complete this work and keep the app Lithic."),
			Images: []protocol.Attachment{{Path: "/tmp/frame-reference.png", Mime: "image/png"}},
		}),
		event(2, protocol.EvModel, protocol.ChoiceData{
			Current: info.Model,
			Options: []protocol.Choice{{ID: info.Model, Name: info.Model}, {ID: "small", Name: i18n.T("Small")}},
		}),
		event(3, protocol.EvMode, protocol.ChoiceData{
			Current: info.Mode,
			Options: []protocol.Choice{{ID: "default", Name: i18n.T("Default")}, {ID: "plan", Name: i18n.T("Plan")}},
		}),
		event(4, protocol.EvTurnStart, protocol.TurnStartData{Turn: 1}),
		event(5, protocol.EvPlan, protocol.PlanData{Entries: []protocol.PlanEntry{
			{Content: i18n.T("Read the frame contract"), Status: "completed"},
			{Content: i18n.T("Build the shell frame"), Status: "in_progress"},
			{Content: i18n.T("Check every route"), Status: "pending"},
		}}),
		event(6, protocol.EvTool, protocol.ToolData{
			Call: "read-1", Title: i18n.T("Read the app contract"), Kind: protocol.ToolRead,
			Status: protocol.ToolDone, Input: "docs/rashin.md", Output: i18n.T("Found the Work contract."),
		}),
		event(7, protocol.EvTool, protocol.ToolData{
			Call: "read-2", Title: i18n.T("Read the Lithic layout"), Kind: protocol.ToolRead,
			Status: protocol.ToolDone, Input: "docs/lithic/layout.md", Output: i18n.T("Found the conversation window."),
		}),
		event(8, protocol.EvTool, protocol.ToolData{
			Call: "edit-1", Title: i18n.T("Edit shell frame"), Kind: protocol.ToolEdit,
			Status: protocol.ToolRunning, Input: "internal/app/shell/window.go",
		}),
		event(9, protocol.EvTool, protocol.ToolData{
			Call: "test-1", Title: i18n.T("Run frame tests"), Kind: protocol.ToolExecute,
			Status: protocol.ToolFailed, Input: "go test ./internal/app/shell",
			Output: i18n.T("One geometry assertion failed."),
		}),
		event(10, protocol.EvPermission, protocol.PermissionData{
			Request: "permission-answered", Call: "edit-old", Title: i18n.T("Edit the old frame"),
			Kind: protocol.ToolEdit, Input: "internal/app/shell/router.go",
			Options: []protocol.PermissionOption{
				{ID: "allow_once", Label: i18n.T("Allow once"), Kind: protocol.OptAllowOnce},
			},
		}),
		event(11, protocol.EvPermissionResolved, protocol.PermissionResolvedData{
			Request: "permission-answered", Option: "allow_once", Outcome: "selected", By: "user",
		}),
		event(12, protocol.EvText, protocol.TextData{
			Message: "message-1",
			Delta:   i18n.T("The frame now keeps navigation, search and the reading column in one stable composition."),
		}),
		event(13, protocol.EvError, protocol.ErrorData{
			Message: i18n.T("The first render missed one icon. The frame can be rendered again."),
			Code:    "render_icon",
		}),
	}
	if permission {
		events = append(events, event(14, protocol.EvPermission, protocol.PermissionData{
			Request: "permission-1", Call: "edit-2", Title: i18n.T("Edit settings page"),
			Kind: protocol.ToolEdit, Input: "internal/app/pages/settings/page.go",
			Options: []protocol.PermissionOption{
				{ID: "allow_once", Label: i18n.T("Allow once"), Kind: protocol.OptAllowOnce},
				{ID: "allow_always", Label: i18n.T("Always allow"), Kind: protocol.OptAllowAlways},
				{ID: "reject_once", Label: i18n.T("Deny"), Kind: protocol.OptRejectOnce},
			},
		}))
	}
	// last, so every seq above stays what the tests and the stories know it as
	return append(events, event(int64(len(events)+1), protocol.EvOptions, protocol.OptionsData{Options: demoOptions("auto")}))
}

// what a demo harness offers beside its model and mode
func demoOptions(thinking string) []protocol.SessionOption {
	return []protocol.SessionOption{{
		ID: "thinking", Name: i18n.T("Thinking"), Category: "thought_level", Current: thinking,
		Choices: []protocol.Choice{
			{ID: "off", Name: i18n.T("Off")}, {ID: "auto", Name: i18n.T("Auto"), Description: i18n.T("Decides for each prompt")},
			{ID: "high", Name: i18n.T("High")},
		},
	}}
}

// more models than fit a menu, as a real harness has, so the list has to be searched
func demoModels() []protocol.Choice {
	names := []string{"Opus", "Sonnet", "Haiku", "Sol", "Luna", "Terra", "Flash", "Pro", "Ultra", "Mini", "Nano", "Max"}
	models := make([]protocol.Choice, len(names))
	for index, name := range names {
		provider := "anthropic"
		if index >= 3 {
			provider = "openai"
		}
		if index >= 6 {
			provider = "google"
		}
		models[index] = protocol.Choice{ID: provider + "/" + strings.ToLower(name), Name: name}
	}
	return models
}
func event(seq int64, kind string, data any) protocol.SessionEvent {
	raw, _ := json.Marshal(data)
	return protocol.SessionEvent{Seq: seq, TS: time.Date(2026, 11, 10, 10, 45, int(seq), 0, time.Local), Kind: kind, Data: raw}
}
func intPointer(value int) *int { return &value }
