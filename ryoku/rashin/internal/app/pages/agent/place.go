package agent

import (
	"strings"
	"unicode"

	"gioui.org/layout"

	"ryoku-i18n"

	"ryoku-rashin/internal/app/env"
	"ryoku-rashin/internal/app/look"
	"ryoku-rashin/internal/app/state"
	"ryoku-rashin/internal/paths"
	"ryoku-rashin/internal/protocol"
)

const (
	worktreeRoot = iota
	worktreeNew
	worktreeFirst
)

// the board's composer and the full dialog share one, so a choice made in either is the choice in both
type placement struct {
	project, worktree, harness  look.Select
	projectIndex, worktreeIndex int
	pick                        env.HarnessPick
	primed                      bool
}

func (place *placement) prime(e *env.Env) {
	if place.primed {
		return
	}
	place.primed = true
	for _, project := range e.Snapshot().Projects {
		e.LoadWorktrees(project.ID, nil)
	}
}

func (place *placement) clamp(snapshot state.Snapshot) {
	place.projectIndex = min(place.projectIndex, len(snapshot.Projects))
	if place.projectIndex > 0 {
		worktrees := snapshot.Worktrees[snapshot.Projects[place.projectIndex-1].ID]
		place.worktreeIndex = min(place.worktreeIndex, worktreeFirst+len(worktrees)-1)
	}
}

func (place *placement) projectLabels(snapshot state.Snapshot) []string {
	labels := []string{i18n.T("No project")}
	for _, project := range snapshot.Projects {
		labels = append(labels, project.Name)
	}
	return labels
}

func (place *placement) worktreeLabels(snapshot state.Snapshot) []string {
	labels := []string{i18n.T("Project folder"), i18n.T("New worktree")}
	if place.projectIndex == 0 {
		return labels[:1]
	}
	for _, worktree := range snapshot.Worktrees[snapshot.Projects[place.projectIndex-1].ID] {
		if !worktree.Main {
			labels = append(labels, worktree.Name)
		}
	}
	return labels
}

func harnessLabels(harnesses []protocol.HarnessInfo) []string {
	if len(harnesses) == 0 {
		return []string{i18n.T("No harness installed")}
	}
	labels := make([]string, len(harnesses))
	for index, harness := range harnesses {
		labels[index] = harness.Name
	}
	return labels
}

// home is the composer's text: a choice made in its row gives the keyboard back to it
func (place *placement) controls(e *env.Env, home any) []layout.Widget {
	snapshot := e.Snapshot()
	place.prime(e)
	place.clamp(snapshot)
	th := e.Theme
	quiet := func(selection *look.Select, label string) {
		selection.Form, selection.Label, selection.Return = look.SelectQuiet, label, home
	}
	quiet(&place.project, i18n.T("Project"))
	quiet(&place.worktree, i18n.T("Worktree"))
	quiet(&place.harness, i18n.T("Harness"))
	place.project.Icon, place.worktree.Icon, place.harness.Icon = look.Icon.Folder, look.Icon.Branch, look.Icon.Agents
	controls := []layout.Widget{func(gtx layout.Context) layout.Dimensions {
		return place.project.Layout(gtx, th, e.Overlays, place.projectLabels(snapshot), place.projectIndex)
	}}
	if place.projectIndex > 0 {
		controls = append(controls, func(gtx layout.Context) layout.Dimensions {
			return place.worktree.Layout(gtx, th, e.Overlays, place.worktreeLabels(snapshot), place.worktreeIndex)
		})
	}
	harnesses := env.InstalledHarnesses(snapshot)
	controls = append(controls, func(gtx layout.Context) layout.Dimensions {
		return place.harness.Layout(gtx, th, e.Overlays, harnessLabels(harnesses), place.pick.Index(snapshot, harnesses))
	})
	place.update(e, harnesses)
	return controls
}

func (place *placement) update(e *env.Env, harnesses []protocol.HarnessInfo) {
	if index, changed := place.project.Changed(); changed {
		place.projectIndex, place.worktreeIndex = index, worktreeRoot
	}
	if index, changed := place.worktree.Changed(); changed {
		place.worktreeIndex = index
	}
	if index, changed := place.harness.Changed(); changed {
		place.pick.Use(e, harnesses, index)
	}
}

type details struct {
	folder, name, base, branch string
}

func (place *placement) params(snapshot state.Snapshot, more details) (protocol.SessionNewParams, bool) {
	harnesses := env.InstalledHarnesses(snapshot)
	if len(harnesses) == 0 {
		return protocol.SessionNewParams{}, false
	}
	place.clamp(snapshot)
	harness := harnesses[place.pick.Index(snapshot, harnesses)].ID
	params := protocol.SessionNewParams{Kind: protocol.KindAgent, Harness: harness, Origin: protocol.OriginApp}
	if place.projectIndex == 0 {
		params.Cwd = strings.TrimSpace(more.folder)
		if params.Cwd == "" {
			params.Cwd = paths.Home()
		}
		return params, true
	}
	project := snapshot.Projects[place.projectIndex-1]
	params.Project = project.ID
	switch {
	case place.worktreeIndex == worktreeNew:
		params.Worktree, params.Name = "new", strings.TrimSpace(more.name)
		params.Base, params.Branch = strings.TrimSpace(more.base), strings.TrimSpace(more.branch)
	case place.worktreeIndex >= worktreeFirst:
		linked := make([]protocol.WorktreeInfo, 0)
		for _, worktree := range snapshot.Worktrees[project.ID] {
			if !worktree.Main {
				linked = append(linked, worktree)
			}
		}
		if at := place.worktreeIndex - worktreeFirst; at < len(linked) {
			params.Worktree = linked[at].Path
		}
	}
	return params, true
}

// named from what the agent is asked to do so starting from the board needs no second step
func worktreeName(prompt string) string {
	var name strings.Builder
	words := 0
	for _, word := range strings.Fields(strings.ToLower(prompt)) {
		cleaned := strings.Map(func(character rune) rune {
			if unicode.IsLetter(character) || unicode.IsDigit(character) {
				return character
			}
			return -1
		}, word)
		if cleaned == "" {
			continue
		}
		if words > 0 {
			name.WriteByte('-')
		}
		name.WriteString(cleaned)
		if words++; words == 4 {
			break
		}
	}
	return name.String()
}
