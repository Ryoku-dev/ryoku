package teams

import (
	"encoding/json"
	"path/filepath"

	"ryoku-rashin/internal/protocol"
)

func (s *Service) taskLineage(id string) (protocol.TeamTaskContext, error) {
	task, exists := s.state.task(id)
	if !exists {
		return protocol.TeamTaskContext{}, protocol.Errorf(protocol.CodeNotFound, "No task %s.", id)
	}
	return lineage(s.state.task, task)
}

func lineage(get func(string) (protocol.TeamTask, bool), task protocol.TeamTask) (protocol.TeamTaskContext, error) {
	context := protocol.TeamTaskContext{Task: task, Goal: task.Goal, Because: []protocol.TaskAncestor{}}
	seen := map[string]bool{task.ID: true}
	current := task
	for current.Parent != "" {
		if seen[current.Parent] {
			return context, protocol.Errorf(protocol.CodeConflict, "Task parentage contains a cycle.")
		}
		parent, exists := get(current.Parent)
		if !exists || parent.Team != task.Team {
			return context, protocol.Errorf(protocol.CodeConflict, "A parent task is missing or belongs to another team.")
		}
		seen[parent.ID] = true
		context.Because = append(context.Because, protocol.TaskAncestor{ID: parent.ID, Title: parent.Title, Member: parent.Member})
		current = parent
	}
	if context.Goal == "" {
		context.Goal = current.Goal
		if context.Goal == "" {
			context.Goal = current.Brief
		}
	}
	return context, nil
}

// Existing tasks predate the goal snapshot. Preserve the currently stored team
// purpose, or their original root brief when no standing purpose was recorded.
func (st *state) migrateTaskGoals() error {
	get := func(id string) (protocol.TeamTask, bool) { task, ok := st.taskSet[id]; return task, ok }
	for id, task := range st.taskSet {
		if task.Goal != "" {
			continue
		}
		if team, exists := st.teamSet[task.Team]; exists && team.Purpose != "" {
			task.Goal = team.Purpose
		} else if context, err := lineage(get, task); err == nil {
			task.Goal = context.Goal
		}
		if task.Goal == "" {
			continue
		}
		data, err := json.MarshalIndent(task, "", "  ")
		if err != nil {
			return err
		}
		if err := st.writer.write(filepath.Join(st.dir, "tasks"), id+".json", append(data, '\n')); err != nil {
			return err
		}
		st.taskSet[id] = task
	}
	return nil
}
