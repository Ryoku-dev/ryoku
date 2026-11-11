package demo

import (
	"strconv"

	"ryoku-rashin/internal/protocol"
)

// one thing a team agent did in its transcript: a tool call, a message, or the next turn Rashin started for it
type line struct {
	tool *protocol.ToolData
	text string
	// next is what Rashin sent to start another turn toward the goal
	next string
}

func did(call, title, kind, status, input, output string) line {
	return line{tool: &protocol.ToolData{Call: call, Title: title, Kind: kind, Status: status, Input: input, Output: output}}
}

func said(text string) line { return line{text: text} }

func continued(text string) line { return line{next: text} }

// a team task's session as the daemon replays it: the goal Rashin set, the brief it sent, the agent's turns, and the end the task came to
func teamTranscript(task protocol.TeamTask, lines []line) []protocol.SessionEvent {
	var events []protocol.SessionEvent
	seq := int64(0)
	add := func(kind string, data any) {
		seq++
		events = append(events, event(seq, kind, data))
	}
	goal := teamGoal(task)
	set := goal
	set.Status, set.Turns, set.Reason, set.Ended = protocol.GoalActive, 0, "", nil
	add(protocol.EvGoal, protocol.GoalData{Goal: set})
	add(protocol.EvUser, protocol.UserData{Text: goal.Objective})
	turn := 1
	add(protocol.EvTurnStart, protocol.TurnStartData{Turn: turn})
	message := 0
	for _, step := range lines {
		switch {
		case step.tool != nil:
			add(protocol.EvTool, *step.tool)
		case step.next != "":
			add(protocol.EvTurnEnd, protocol.TurnEndData{Turn: turn, Stop: protocol.StopEnd})
			turn++
			add(protocol.EvUser, protocol.UserData{Text: step.next, Goal: true})
			add(protocol.EvTurnStart, protocol.TurnStartData{Turn: turn})
		default:
			message++
			add(protocol.EvText, protocol.TextData{Message: task.ID + "-" + strconv.Itoa(message), Delta: step.text})
		}
	}
	// a working agent is mid-turn, every other task's last turn has ended
	if task.Status == protocol.TaskWorking && task.Waiting == "" {
		return events
	}
	usage := task.Usage
	stop := protocol.StopEnd
	if task.Status == protocol.TaskStopped {
		stop = protocol.StopCancelled
	}
	add(protocol.EvTurnEnd, protocol.TurnEndData{Turn: turn, Stop: stop, Usage: &usage})
	if goal.Status != protocol.GoalActive {
		add(protocol.EvGoal, protocol.GoalData{Goal: goal})
	}
	return events
}

// the goal a task's session follows, ended the way the task ended
func teamGoal(task protocol.TeamTask) protocol.Goal {
	goal := protocol.Goal{Objective: task.Title + "\n\n" + task.Brief, Status: protocol.GoalActive, Turns: task.Turns, MaxTurns: task.MaxTurns, Started: task.Created}
	if task.Started != nil {
		goal.Started = *task.Started
	}
	goal.Ended = task.Ended
	switch task.Status {
	case protocol.TaskDone:
		goal.Status = protocol.GoalDone
		if task.Report != nil {
			goal.Reason = task.Report.Title
		}
	case protocol.TaskBlocked:
		goal.Status, goal.Reason = protocol.GoalBlocked, task.Reason
	case protocol.TaskStopped:
		goal.Status, goal.Reason = protocol.GoalStopped, task.Reason
	}
	return goal
}

// the session that works a task, as the daemon describes it while the task stands where it does
func teamSession(team protocol.Team, task protocol.TeamTask, events []protocol.SessionEvent, preview, activity string) protocol.SessionInfo {
	member, _ := teamMember(team, task.Member)
	info := protocol.SessionInfo{
		ID:        task.Session,
		Kind:      protocol.KindAgent,
		Title:     task.Title,
		Harness:   task.Harness,
		Model:     task.Model,
		Mode:      "default",
		Cwd:       team.Folder,
		Origin:    protocol.OriginTeam,
		Created:   task.Created,
		Preview:   preview,
		Activity:  activity,
		Steps:     lastTurnSteps(events),
		Team:      team.ID,
		TeamTask:  task.ID,
		Member:    member.ID,
		Approvals: team.Approvals,
	}
	if info.Model == "" {
		info.Model = "default"
	}
	if task.Started != nil {
		info.Created = *task.Started
	}
	// a first turn began when the task did, a later one when the task last moved on
	if task.Turns <= 1 && task.Started != nil {
		started := *task.Started
		info.TurnStarted = &started
	}
	if len(events) > 0 {
		info.LastSeq = events[len(events)-1].Seq
	}
	standSession(&info, task)
	return info
}

// a session stands where its task does: at work, held alive between turns, or ended
func standSession(info *protocol.SessionInfo, task protocol.TeamTask) {
	goal := teamGoal(task)
	info.Goal, info.Turns, info.Updated = &goal, task.Turns, task.Updated
	info.State, info.Alive = protocol.StateIdle, false
	switch task.Status {
	case protocol.TaskWorking:
		info.Alive = true
		if task.Waiting == "" {
			info.State = protocol.StateWorking
		}
	case protocol.TaskNeedsYou:
		info.Alive = true
	case protocol.TaskStopped:
		info.State = protocol.StateStopped
	}
	if !info.Alive {
		info.Activity = ""
	}
	switch {
	case info.State != protocol.StateWorking:
		info.TurnStarted = nil
	case info.TurnStarted == nil:
		started := task.Updated
		info.TurnStarted = &started
	}
}

// the tool calls of the last turn, the one at work or the one that ended
func lastTurnSteps(events []protocol.SessionEvent) int {
	steps := 0
	for _, made := range events {
		switch made.Kind {
		case protocol.EvTurnStart:
			steps = 0
		case protocol.EvTool:
			steps++
		}
	}
	return steps
}

// a task starts on its member's engine through the defaults chain, with what was asked for this task alone on top
func startEngine(task *protocol.TeamTask, member protocol.Member, settings protocol.Settings, fallback string) {
	harness, model, options := protocol.ResolveEngine(member, settings, fallback)
	task.Harness = harness
	if task.Model == "" {
		task.Model = model
	}
	if len(task.Options) == 0 {
		task.Options = options
	}
}

// the member a task goes to, the lead when it names none
func teamMember(team protocol.Team, id string) (protocol.Member, bool) {
	for _, member := range team.Members {
		if member.ID == id || id == "" && member.Lead {
			return member, true
		}
	}
	return protocol.Member{}, false
}
