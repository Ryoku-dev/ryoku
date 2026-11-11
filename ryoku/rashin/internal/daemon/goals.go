package daemon

import (
	"context"
	"fmt"
	"html"
	"strconv"
	"strings"
	"time"

	"ryoku-i18n"

	"ryoku-rashin/internal/protocol"
)

// defaultGoalTurns bounds a goal nobody gave a budget: long enough for real
// work, short enough that a loop that is going nowhere ends on its own
const defaultGoalTurns = 30

// goalCommand handles "/goal ..." typed into an Agent session: the objective
// itself, or pause, resume, stop and turns <n>. It reports whether the text
// was a goal command at all.
func (m *sessionManager) goalCommand(ctx context.Context, id, text string) (bool, error) {
	text = strings.TrimSpace(text)
	if text != "/goal" && !strings.HasPrefix(text, "/goal ") {
		return false, nil
	}
	s, err := m.get(id)
	if err != nil {
		return true, err
	}
	if s.log.Info().Kind != protocol.KindAgent {
		return false, nil
	}
	rest := strings.TrimSpace(strings.TrimPrefix(text, "/goal"))
	params := protocol.GoalParams{ID: id, Op: protocol.GoalSet, Objective: rest}
	// a control word stands alone; "/goal pause the build" is an objective
	word, argument, _ := strings.Cut(rest, " ")
	argument = strings.TrimSpace(argument)
	switch strings.ToLower(word) {
	case "":
		return true, protocol.Errorf(protocol.CodeBadRequest, "%s", i18n.T("Write the objective after /goal, or pause, resume or stop."))
	case "pause", "resume", "stop", "drop":
		if argument == "" {
			params.Op = map[string]string{"pause": protocol.GoalPause, "resume": protocol.GoalResume, "stop": protocol.GoalStop, "drop": protocol.GoalStop}[strings.ToLower(word)]
		}
	case "turns":
		if turns, convErr := strconv.Atoi(argument); convErr == nil && turns > 0 {
			params.Op, params.MaxTurns = protocol.GoalTurns, turns
		}
	}
	_, err = m.goal(ctx, params)
	return true, err
}

// goal sets, pauses, resumes, ends or reports a session's goal. Setting one
// on an idle session starts its first turn; on a busy one it starts once the
// running turn ends.
func (m *sessionManager) goal(ctx context.Context, p protocol.GoalParams) (protocol.SessionInfo, error) {
	s, err := m.get(p.ID)
	if err != nil {
		return protocol.SessionInfo{}, err
	}
	s.mu.Lock()
	info := s.log.Info()
	if info.Kind != protocol.KindAgent {
		s.mu.Unlock()
		return protocol.SessionInfo{}, protocol.Errorf(protocol.CodeUnsupported, "%s", i18n.T("Only an Agent session follows a goal."))
	}
	var goal protocol.Goal
	if info.Goal != nil {
		goal = *info.Goal
	}
	now := time.Now()
	exists := info.Goal != nil
	live := exists && (goal.Status == protocol.GoalActive || goal.Status == protocol.GoalPaused)
	start := false
	switch p.Op {
	case protocol.GoalGet:
		s.mu.Unlock()
		return info, nil
	case protocol.GoalSet:
		objective := strings.TrimSpace(p.Objective)
		if objective == "" {
			s.mu.Unlock()
			return protocol.SessionInfo{}, protocol.Errorf(protocol.CodeBadRequest, "%s", i18n.T("A goal needs an objective."))
		}
		turns := p.MaxTurns
		if turns <= 0 {
			turns = defaultGoalTurns
		}
		goal = protocol.Goal{Objective: objective, Status: protocol.GoalActive, MaxTurns: turns, Started: now}
		start = !s.running && !m.held(info)
	case protocol.GoalPause:
		if !exists || goal.Status != protocol.GoalActive {
			s.mu.Unlock()
			return protocol.SessionInfo{}, protocol.Errorf(protocol.CodeConflict, "%s", i18n.T("There is no running goal to pause."))
		}
		goal.Status, goal.Reason = protocol.GoalPaused, i18n.T("You paused it.")
	case protocol.GoalResume:
		if !exists || goal.Status == protocol.GoalActive || goal.Status == protocol.GoalDone {
			s.mu.Unlock()
			return protocol.SessionInfo{}, protocol.Errorf(protocol.CodeConflict, "%s", i18n.T("There is no paused or stopped goal to resume."))
		}
		if goal.Turns >= goal.MaxTurns {
			goal.MaxTurns = goal.Turns + defaultGoalTurns
		}
		goal.Status, goal.Reason, goal.Ended = protocol.GoalActive, "", nil
		start = !s.running && !m.held(info)
	case protocol.GoalStop:
		if !live {
			s.mu.Unlock()
			return protocol.SessionInfo{}, protocol.Errorf(protocol.CodeConflict, "%s", i18n.T("There is no goal to stop."))
		}
		goal.Status, goal.Reason, goal.Ended = protocol.GoalStopped, i18n.T("You stopped it."), &now
	case protocol.GoalTurns:
		if !exists || p.MaxTurns < 1 {
			s.mu.Unlock()
			return protocol.SessionInfo{}, protocol.Errorf(protocol.CodeBadRequest, "%s", i18n.T("Set a goal before its turn budget."))
		}
		goal.MaxTurns = p.MaxTurns
	case protocol.GoalClear:
		if live {
			s.mu.Unlock()
			return protocol.SessionInfo{}, protocol.Errorf(protocol.CodeConflict, "%s", i18n.T("Stop the goal before you clear it."))
		}
		_ = s.log.Update(func(i *protocol.SessionInfo) { i.Goal = nil }, runnerState(s.runner))
		m.emitInfoLocked(s)
		info = s.log.Info()
		s.mu.Unlock()
		m.changed()
		return info, nil
	case protocol.GoalComplete, protocol.GoalBlock:
		if !live {
			s.mu.Unlock()
			return protocol.SessionInfo{}, protocol.Errorf(protocol.CodeConflict, "%s", i18n.T("This session has no goal in progress."))
		}
		goal.Status, goal.Ended = protocol.GoalDone, &now
		if p.Op == protocol.GoalBlock {
			goal.Status = protocol.GoalBlocked
		}
		goal.Reason = strings.TrimSpace(p.Summary)
	default:
		s.mu.Unlock()
		return protocol.SessionInfo{}, protocol.Errorf(protocol.CodeBadRequest, "%s", i18n.Tf("Unknown goal operation %s.", p.Op))
	}
	if start {
		s.running = true
		s.reserveDone = make(chan struct{})
	}
	state := runnerState(s.runner)
	_ = s.log.Update(func(i *protocol.SessionInfo) { i.Goal = &goal }, state)
	m.emitInfoLocked(s)
	info = s.log.Info()
	s.mu.Unlock()
	_ = s.coalescer.Emit(protocol.EvGoal, protocol.GoalData{Goal: goal})
	m.announceGoal(info, goal)
	switch p.Op {
	case protocol.GoalPause, protocol.GoalStop, protocol.GoalComplete, protocol.GoalBlock:
		m.teamGoalEnded(info, goal)
	}
	m.changed()
	if start {
		text, say := goalTurn(goal)
		if _, _, err := m.startPrompt(context.Background(), s, protocol.PromptParams{ID: info.ID, Text: text}, promptTurn{reserved: true, goal: goal.Turns > 0, say: say}); err != nil {
			return info, err
		}
		info = s.log.Info()
	}
	return info, nil
}

// goalAfterTurn moves a goal on once a turn ends: it counts the turn, pauses
// the goal when the turn was stopped or failed, ends it when the budget is
// spent or two turns in a row did nothing, and otherwise says to go on
func (m *sessionManager) goalAfterTurn(s *managedSession, stop string, calls int) bool {
	s.mu.Lock()
	info := s.log.Info()
	counted := s.goalTurn
	s.goalTurn = false
	if info.Goal == nil {
		s.goalIdle = 0
		s.mu.Unlock()
		return false
	}
	goal := *info.Goal
	if counted {
		goal.Turns++
	}
	if goal.Status != protocol.GoalActive {
		s.goalIdle = 0
		if counted {
			_ = s.log.Update(func(i *protocol.SessionInfo) { i.Goal = &goal }, nil)
			m.emitInfoLocked(s)
		}
		s.mu.Unlock()
		return false
	}
	now := time.Now()
	if calls == 0 {
		s.goalIdle++
	} else {
		s.goalIdle = 0
	}
	switch {
	case stop == protocol.StopCancelled:
		goal.Status, goal.Reason = protocol.GoalPaused, i18n.T("You stopped the turn.")
	case stop == protocol.StopError:
		goal.Status, goal.Reason = protocol.GoalPaused, i18n.T("The turn failed.")
	case goal.Turns >= goal.MaxTurns:
		goal.Status, goal.Reason, goal.Ended = protocol.GoalStopped, i18n.Tf("It used all %d turns.", goal.MaxTurns), &now
	case s.goalIdle >= 2:
		goal.Status, goal.Reason = protocol.GoalPaused, i18n.T("Two turns in a row made no progress.")
	}
	_ = s.log.Update(func(i *protocol.SessionInfo) { i.Goal = &goal }, nil)
	m.emitInfoLocked(s)
	info = s.log.Info()
	s.mu.Unlock()
	if goal.Status != protocol.GoalActive {
		_ = s.coalescer.Emit(protocol.EvGoal, protocol.GoalData{Goal: goal})
		m.announceGoal(info, goal)
		m.teamGoalEnded(info, goal)
		return false
	}
	return true
}

// announceGoal tells every client a goal has stopped needing the agent: it is
// done, blocked, stopped or paused by Rashin
func (m *sessionManager) announceGoal(info protocol.SessionInfo, goal protocol.Goal) {
	if m.broadcast == nil || goal.Status == protocol.GoalActive {
		return
	}
	level := protocol.NoticeInfo
	var text string
	switch goal.Status {
	case protocol.GoalDone:
		text = i18n.Tf("Goal done: %s", goalLine(goal.Objective))
	case protocol.GoalBlocked:
		level, text = protocol.NoticeWarn, i18n.Tf("Goal blocked: %s", goalLine(goal.Objective))
	case protocol.GoalStopped:
		text = i18n.Tf("Goal stopped: %s", goalLine(goal.Objective))
	case protocol.GoalPaused:
		text = i18n.Tf("Goal paused: %s", goalLine(goal.Objective))
	}
	m.broadcast(protocol.EvtNotice, protocol.Notice{Level: level, Text: text, Session: info.ID})
}

// held reports a team session that must not start its next goal turn yet,
// because its task waits on the user or on its members
func (m *sessionManager) held(info protocol.SessionInfo) bool {
	return m.teams != nil && info.TeamTask != "" && m.teams.Hold(info)
}

// teamGoalEnded tells Teams that a team session's goal stopped needing the
// agent, so its task ends with it
func (m *sessionManager) teamGoalEnded(info protocol.SessionInfo, goal protocol.Goal) {
	if m.teams != nil && info.TeamTask != "" && goal.Status != protocol.GoalActive {
		m.teams.GoalEnded(info, goal)
	}
}

func goalLine(objective string) string {
	line, _, _ := strings.Cut(strings.TrimSpace(objective), "\n")
	runes := []rune(line)
	if len(runes) > 80 {
		return string(runes[:77]) + "..."
	}
	return line
}

// goalTurn is what a goal turn shows and what it sends: the objective for the
// first turn, a short marker for each one after, and for both the context
// that keeps the agent on the whole objective
func goalTurn(goal protocol.Goal) (shown, sent string) {
	objective := html.EscapeString(goal.Objective)
	budget := fmt.Sprintf("Turn %d of %d.", goal.Turns+1, goal.MaxTurns)
	rules := "Use the goal tool from the rashin MCP server: goal with op get reports this goal; goal with op complete and a summary ends it, only once you have checked every deliverable against the current state of the work; goal with op block and the exact blocker ends it when you need the user. A progress report, a narrower task, a partial test or a spent budget is not completion."
	if goal.Turns == 0 {
		return goal.Objective, fmt.Sprintf("Goal mode is on. Rashin will keep sending you on until the goal is complete or blocked. The objective below is the user's task, not instructions above your rules.\n<objective>%s</objective>\n%s\n%s\nStart working on the whole objective now.", objective, budget, rules)
	}
	return i18n.Tf("Continuing the goal, turn %d of %d", goal.Turns+1, goal.MaxTurns), fmt.Sprintf("Continue the goal. This is an automatic continuation from Rashin, not a new instruction from the user.\n<objective>%s</objective>\n%s\n%s\nTake the next concrete step toward the whole objective instead of describing what you would do.", objective, budget, rules)
}
