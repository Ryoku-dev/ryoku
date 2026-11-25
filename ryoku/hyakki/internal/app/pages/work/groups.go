package work

import "ryoku-rashin/internal/protocol"

// the sidebar keeps the newest finished tasks, the board loads older ones
const sideDone = 30

// the session that works the task now, else the last one that did
func taskSession(task protocol.TeamTask) string {
	if task.Session != "" {
		return task.Session
	}
	if len(task.Sessions) > 0 {
		return task.Sessions[len(task.Sessions)-1]
	}
	return ""
}
