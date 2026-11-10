package agent

import (
	"path/filepath"
	"sort"

	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/palette"
	"ryoku-i18n"

	"ryoku-rashin/internal/app/state"
	"ryoku-rashin/internal/protocol"
)

type group int

const (
	groupNeeds group = iota
	groupWorking
	groupDone
	groupIdle
	groupCount
)

func (g group) label() string {
	switch g {
	case groupNeeds:
		return i18n.T("Needs you")
	case groupWorking:
		return i18n.T("Working")
	case groupDone:
		return i18n.T("Done")
	default:
		return i18n.T("Idle")
	}
}

func groupSessions(snapshot state.Snapshot) [groupCount][]protocol.SessionInfo {
	var groups [groupCount][]protocol.SessionInfo
	for _, id := range snapshot.Order {
		info, ok := snapshot.Sessions[id]
		if !ok || info.Kind != protocol.KindAgent {
			continue
		}
		at := groupOf(snapshot, info)
		groups[at] = append(groups[at], info)
	}
	return groups
}

func groupOf(snapshot state.Snapshot, info protocol.SessionInfo) group {
	switch {
	case needsYou(snapshot, info):
		return groupNeeds
	case info.State == protocol.StateWorking:
		return groupWorking
	case info.Unread:
		return groupDone
	default:
		return groupIdle
	}
}

// an open permission, a failed turn, an open decision on its task, or conflicts in its worktree
func needsYou(snapshot state.Snapshot, info protocol.SessionInfo) bool {
	if info.Pending > 0 || info.State == protocol.StateWaiting || info.State == protocol.StateFailed {
		return true
	}
	if info.Task != "" {
		for _, task := range snapshot.Tasks[info.Project] {
			if task.ID != info.Task {
				continue
			}
			if task.Status == protocol.TaskNeedsYou {
				return true
			}
			for _, gate := range task.Gates {
				if gate.Resolution == "" {
					return true
				}
			}
		}
	}
	if worktree, ok := worktreeOf(snapshot, info); ok && worktree.Status != nil {
		return worktree.Status.Conflicted > 0
	}
	return false
}

func sessionTone(snapshot state.Snapshot, info protocol.SessionInfo) palette.Tone {
	switch {
	case info.State == protocol.StateFailed:
		return palette.Failed
	case needsYou(snapshot, info):
		return palette.Attention
	case info.State == protocol.StateWorking:
		return palette.Becoming
	default:
		return palette.Idle
	}
}

func sessionClaim(snapshot state.Snapshot, info protocol.SessionInfo) string {
	switch {
	case info.State == protocol.StateFailed:
		return i18n.T("Failed")
	case needsYou(snapshot, info):
		return i18n.T("Needs you")
	case info.State == protocol.StateWorking:
		return i18n.T("Working")
	default:
		return ""
	}
}

// a session at its project's root works in the main worktree
func worktreeOf(snapshot state.Snapshot, info protocol.SessionInfo) (protocol.WorktreeInfo, bool) {
	for _, worktree := range snapshot.Worktrees[info.Project] {
		if worktree.Path == info.Worktree || (info.Worktree == "" && worktree.Main) {
			return worktree, true
		}
	}
	return protocol.WorktreeInfo{}, false
}

func projectName(snapshot state.Snapshot, id string) string {
	for _, project := range snapshot.Projects {
		if project.ID == id {
			return project.Name
		}
	}
	return ""
}

func harnessName(snapshot state.Snapshot, id string) string {
	for _, harness := range snapshot.Harnesses {
		if harness.ID == id {
			return harness.Name
		}
	}
	return id
}

func sessionFolder(info protocol.SessionInfo) string {
	if info.Worktree != "" {
		return info.Worktree
	}
	return info.Cwd
}

func sessionPlace(snapshot state.Snapshot, info protocol.SessionInfo) string {
	project := projectName(snapshot, info.Project)
	if project == "" {
		return filepath.Base(sessionFolder(info))
	}
	if worktree, ok := worktreeOf(snapshot, info); ok && !worktree.Main {
		return project + " · " + worktree.Name
	}
	return project
}

func sessionMatter(snapshot state.Snapshot, info protocol.SessionInfo) string {
	switch groupOf(snapshot, info) {
	case groupWorking:
		return info.Activity
	case groupNeeds:
		if attached, ok := snapshot.Attached[info.ID]; ok {
			if pending := attached.Model.Pending(); len(pending) > 0 {
				return pending[0].Title
			}
		}
		for _, task := range snapshot.Tasks[info.Project] {
			if task.ID != info.Task {
				continue
			}
			for _, gate := range task.Gates {
				if gate.Resolution == "" {
					return gate.Question
				}
			}
		}
		if info.Error != "" {
			return info.Error
		}
	}
	return info.Preview
}

func installedHarnesses(snapshot state.Snapshot) []protocol.HarnessInfo {
	result := make([]protocol.HarnessInfo, 0, len(snapshot.Harnesses))
	for _, harness := range snapshot.Harnesses {
		if harness.Installed {
			result = append(result, harness)
		}
	}
	sort.SliceStable(result, func(i, j int) bool {
		if result[i].Default != result[j].Default {
			return result[i].Default
		}
		return result[i].Name < result[j].Name
	})
	return result
}

func choiceOptions(choice protocol.ChoiceData, fallback string) (labels, ids []string, selected int) {
	current := choice.Current
	if current == "" {
		current = fallback
	}
	if len(choice.Options) == 0 {
		if current == "" {
			return []string{i18n.T("Default")}, []string{""}, 0
		}
		return []string{current}, []string{current}, 0
	}
	for index, option := range choice.Options {
		labels, ids = append(labels, option.Name), append(ids, option.ID)
		if option.ID == current {
			selected = index
		}
	}
	return labels, ids, selected
}
