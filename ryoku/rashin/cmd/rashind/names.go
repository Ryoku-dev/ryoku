package main

import (
	"context"
	"time"

	"ryoku-rashin/internal/core"
	"ryoku-rashin/internal/paths"
	"ryoku-rashin/internal/protocol"
)

// memoryNames tell memory the Work projects and the teams its scopes are
// named after.
type memoryNames struct {
	work  core.Work
	teams interface {
		Team(id string) (protocol.Team, bool)
	}
}

func (n memoryNames) projects() []protocol.ProjectInfo {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	return n.work.Projects(ctx)
}

func (n memoryNames) Project(id string) (root, name, mode string, ok bool) {
	for _, project := range n.projects() {
		if project.ID == id {
			return project.Root, project.Name, project.Settings.Memory, true
		}
	}
	return "", "", "", false
}

func (n memoryNames) ProjectAt(root string) (id, name, mode string, ok bool) {
	root = paths.Resolved(root)
	for _, project := range n.projects() {
		if paths.Resolved(project.Root) == root {
			return project.ID, project.Name, project.Settings.Memory, true
		}
	}
	return "", "", "", false
}

func (n memoryNames) Team(id string) (protocol.Team, bool) { return n.teams.Team(id) }
