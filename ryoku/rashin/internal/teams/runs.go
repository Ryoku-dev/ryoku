package teams

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"slices"

	i18n "ryoku-i18n"

	"ryoku-rashin/internal/paths"
	"ryoku-rashin/internal/protocol"
)

const routineRunsKeep = 1000

func routineKind(r protocol.Routine) string {
	if r.Kind == "" {
		return protocol.RoutineTask
	}
	return r.Kind
}

func (s *Service) routineAvailable(r protocol.Routine) bool {
	if s.settings().Teams.Paused {
		return false
	}
	if routineKind(r) != protocol.RoutineTask {
		if !s.feature("schedules") || (r.Preset == "research" && !s.feature("research")) {
			return false
		}
		if r.Team == protocol.RoutinePersonal {
			return true
		}
	}
	team, ok := s.state.team(r.Team)
	return ok && !team.Paused
}

func (s *Service) readRoutineRuns() ([]protocol.RoutineRun, error) {
	file, err := os.Open(filepath.Join(s.state.dir, "runs.jsonl"))
	if errors.Is(err, fs.ErrNotExist) {
		return []protocol.RoutineRun{}, nil
	}
	if err != nil {
		return nil, err
	}
	defer file.Close()
	runs := []protocol.RoutineRun{}
	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 4096), 1<<20)
	for scanner.Scan() {
		var run protocol.RoutineRun
		if err := json.Unmarshal(scanner.Bytes(), &run); err != nil {
			return nil, err
		}
		runs = append(runs, run)
	}
	return runs, scanner.Err()
}

func (s *Service) saveRoutineRun(run protocol.RoutineRun) error {
	s.routineRunsMu.Lock()
	defer s.routineRunsMu.Unlock()
	runs, err := s.readRoutineRuns()
	if err != nil {
		return err
	}
	index := slices.IndexFunc(runs, func(r protocol.RoutineRun) bool { return r.ID == run.ID })
	if index < 0 {
		runs = append(runs, run)
	} else {
		// a fast answer may finish before its caller records the start
		if runs[index].Ended != nil {
			return nil
		}
		runs[index] = run
	}
	return s.writeRoutineRuns(runs, run)
}

func (s *Service) writeRoutineRuns(runs []protocol.RoutineRun, changed protocol.RoutineRun) error {
	runs = runs[max(0, len(runs)-routineRunsKeep):]
	var data bytes.Buffer
	encoder := json.NewEncoder(&data)
	for _, entry := range runs {
		if err := encoder.Encode(entry); err != nil {
			return err
		}
	}
	if err := writeStateFile(s.state.dir, "runs.jsonl", data.Bytes()); err != nil {
		return err
	}
	if changed.Kind == protocol.RoutineAsk && changed.Session != "" {
		if changed.Ended != nil {
			delete(s.scheduledQuestions, changed.Session)
		} else {
			if s.scheduledQuestions == nil {
				s.scheduledQuestions = make(map[string]bool)
			}
			s.scheduledQuestions[changed.Session] = true
		}
	}
	return nil
}

func (s *Service) routineRuns(p protocol.RoutineRunsParams) (protocol.RoutineRunsResult, error) {
	out := protocol.RoutineRunsResult{Runs: []protocol.RoutineRun{}}
	if !s.feature("schedules") {
		return out, protocol.Errorf(protocol.CodeForbidden, "%s", i18n.T("Scheduled runs are off in Settings."))
	}
	return s.runHistory(p)
}

func (s *Service) runHistory(p protocol.RoutineRunsParams) (protocol.RoutineRunsResult, error) {
	out := protocol.RoutineRunsResult{Runs: []protocol.RoutineRun{}}
	s.routineRunsMu.Lock()
	runs, err := s.readRoutineRuns()
	s.routineRunsMu.Unlock()
	if err != nil {
		return out, err
	}
	limit := p.Limit
	if limit <= 0 || limit > routineRunsKeep {
		limit = routineRunsKeep
	}
	for index := len(runs) - 1; index >= 0 && len(out.Runs) < limit; index-- {
		if p.Routine == "" || runs[index].Routine == p.Routine {
			if task, ok := s.state.task(runs[index].Task); ok && runs[index].Ended == nil {
				runs[index].Status = task.Status
			}
			out.Runs = append(out.Runs, runs[index])
		}
	}
	return out, nil
}

func (s *Service) finishRoutineRun(run *protocol.RoutineRun, status, detail string) {
	now := s.now()
	run.Ended, run.Status, run.Detail = &now, status, s.redact(detail)
}

func (s *Service) scheduledOpen(routine string) bool {
	result, err := s.routineRuns(protocol.RoutineRunsParams{Routine: routine})
	if err != nil {
		return true
	}
	for _, run := range result.Runs {
		if run.Ended == nil {
			return true
		}
	}
	return false
}

func (s *Service) fireScheduled(ctx context.Context, r protocol.Routine, initiator string) (protocol.TeamTask, error) {
	if !s.feature("schedules") {
		return protocol.TeamTask{}, protocol.Errorf(protocol.CodeForbidden, "%s", i18n.T("Scheduled runs are off in Settings."))
	}
	run := protocol.RoutineRun{ID: newID("run"), Routine: r.ID, Kind: r.Kind, Team: r.Team, Started: s.now(), Status: "starting"}
	if err := s.saveRoutineRun(run); err != nil {
		return protocol.TeamTask{}, err
	}
	var task protocol.TeamTask
	var err error
	switch r.Kind {
	case protocol.RoutineDream:
		var dream protocol.Team
		dream, err = s.dreamCreate(ctx, protocol.DreamCreateParams{
			Statement: r.Brief, Name: r.Title, Preset: r.Preset,
			Folder: filepath.Join(paths.DreamsHome(), slugName(r.Title)+"-"+run.ID),
		})
		if err == nil {
			s.mu.Lock()
			task, _ = s.state.task(dream.Dream.Task)
			task.Routine, task.Origin = r.ID, protocol.TaskFromRoutine
			err = s.state.putTask(task)
			s.mu.Unlock()
			run.Dream, run.Task, run.Session, run.Status = dream.ID, task.ID, task.Session, task.Status
		}
	case protocol.RoutineAsk:
		if s.sessions == nil {
			err = protocol.Errorf(protocol.CodeConflict, "%s", i18n.T("Rashin is not ready to start a question."))
			break
		}
		var session protocol.SessionInfo
		session, err = s.sessions.New(ctx, protocol.SessionNewParams{Kind: protocol.KindAsk, Origin: protocol.OriginTeam})
		if err != nil {
			break
		}
		run.Session, run.Status = session.ID, protocol.TaskWorking
		// keep the session link before Prompt, which may finish immediately
		err = s.saveRoutineRun(run)
		if err == nil && !s.feature("schedules") {
			err = protocol.Errorf(protocol.CodeForbidden, "%s", i18n.T("Scheduled runs were turned off."))
			_ = s.sessions.Stop(ctx, session.ID)
		}
		if err == nil {
			_, err = s.sessions.Prompt(ctx, protocol.PromptParams{ID: session.ID, Text: r.Brief})
		}
		task = protocol.TeamTask{ID: session.ID, Title: r.Title, Routine: r.ID, Session: session.ID, Status: protocol.TaskWorking, Created: run.Started}
	default:
		err = protocol.Errorf(protocol.CodeBadRequest, "%s", i18n.T("Choose a task, dream or question for the routine."))
	}
	if err != nil {
		s.finishRoutineRun(&run, protocol.TaskBlocked, err.Error())
	}
	if saveErr := s.saveRoutineRun(run); saveErr != nil && err == nil {
		err = saveErr
	}
	if r.Kind == protocol.RoutineDream && task.ID != "" {
		s.finishTaskRun(task)
	}
	if err == nil && r.Kind == protocol.RoutineAsk {
		if info, ok := s.sessions.Info(run.Session); ok && info.Turns > 0 && info.State != protocol.StateWorking && info.State != protocol.StateWaiting {
			run.Status = protocol.TaskDone
			if info.Error != "" || info.State == protocol.StateFailed {
				run.Status = protocol.TaskBlocked
			}
		}
	}
	updated, ok := s.clock.change(r.ID, func(routine *protocol.Routine) bool {
		routine.LastRun, routine.LastTask, routine.LastStatus = &run.Started, task.ID, run.Status
		return true
	})
	if ok {
		s.broadcast(protocol.EvtRoutine, s.fillRoutine(updated, s.now()))
	}
	s.record(protocol.Activity{Actor: r.Title, Action: "routine.fire", Target: r.Title, OK: err == nil, Team: r.Team, Task: run.Task, Initiator: initiator, Detail: run.Detail})
	return task, err
}

func (s *Service) recoverRoutineHistory() {
	if !s.feature("schedules") {
		return
	}
	s.routineRunsMu.Lock()
	runs, err := s.readRoutineRuns()
	for _, run := range runs {
		if run.Kind == protocol.RoutineAsk && run.Session != "" && run.Ended == nil {
			if s.scheduledQuestions == nil {
				s.scheduledQuestions = make(map[string]bool)
			}
			s.scheduledQuestions[run.Session] = true
		}
	}
	s.routineRunsMu.Unlock()
	if err != nil {
		s.record(protocol.Activity{Actor: "Rashin", Action: "routine.record", OK: false, Detail: err.Error()})
		return
	}
	for _, run := range runs {
		if run.Ended != nil {
			continue
		}
		if run.Task != "" {
			if task, ok := s.state.task(run.Task); ok {
				s.finishTaskRun(task)
				continue
			}
		}
		if run.Kind == protocol.RoutineAsk && s.sessions != nil {
			if info, ok := s.sessions.Info(run.Session); ok {
				if info.State == protocol.StateWorking || info.State == protocol.StateWaiting {
					continue
				}
				s.finishRoutineRun(&run, protocol.TaskStopped, i18n.T("The question ended before Rashin restarted."))
				if info.State == protocol.StateIdle && info.Turns > 0 {
					run.Status, run.Detail = protocol.TaskDone, info.Preview
				}
			} else {
				s.finishRoutineRun(&run, protocol.TaskStopped, i18n.T("The question is no longer available."))
			}
		} else {
			s.finishRoutineRun(&run, protocol.TaskStopped, i18n.T("Rashin restarted before this run could start."))
		}
		if run.Kind == protocol.RoutineAsk {
			if _, err := s.finishQuestionRun(run); err != nil {
				s.record(protocol.Activity{Actor: "Rashin", Action: "routine.record", OK: false, Detail: err.Error()})
			}
		} else if err := s.saveRoutineRun(run); err != nil {
			s.record(protocol.Activity{Actor: "Rashin", Action: "routine.record", OK: false, Detail: err.Error()})
		}
	}
}

func (s *Service) finishTaskRun(task protocol.TeamTask) {
	if !s.feature("schedules") || (task.Status != protocol.TaskDone && task.Status != protocol.TaskStopped && task.Status != protocol.TaskBlocked) {
		return
	}
	result, err := s.routineRuns(protocol.RoutineRunsParams{Routine: task.Routine})
	if err != nil {
		return
	}
	for _, run := range result.Runs {
		if run.Task != task.ID || run.Ended != nil {
			continue
		}
		run.Session, run.Tokens = task.Session, int64(task.Usage.Input+task.Usage.Output)
		detail := task.Reason
		if task.Report != nil {
			detail = task.Report.Title
		}
		s.finishRoutineRun(&run, task.Status, detail)
		if task.Ended != nil {
			run.Ended = task.Ended
		}
		if err := s.saveRoutineRun(run); err != nil {
			s.record(protocol.Activity{Actor: "Rashin", Action: "routine.record", Target: task.Title, OK: false, Detail: err.Error(), Task: task.ID})
		}
	}
}

func (s *Service) RoutineSessionEnded(info protocol.SessionInfo, end protocol.TurnEndData) {
	if info.Kind != protocol.KindAsk {
		return
	}
	s.routineRunsMu.Lock()
	tracked := s.scheduledQuestions[info.ID]
	s.routineRunsMu.Unlock()
	if !tracked {
		return
	}
	result, err := s.runHistory(protocol.RoutineRunsParams{})
	if err != nil {
		return
	}
	for _, run := range result.Runs {
		if run.Session != info.ID || run.Kind != protocol.RoutineAsk || run.Ended != nil {
			continue
		}
		status := protocol.TaskDone
		if info.Error != "" || info.State == protocol.StateFailed {
			status = protocol.TaskBlocked
		} else if end.Stop == "cancelled" || end.Stop == "canceled" || end.Stop == "stopped" {
			status = protocol.TaskStopped
		}
		s.finishRoutineRun(&run, status, info.Preview)
		if end.Usage != nil {
			run.Tokens = int64(end.Usage.Input + end.Usage.Output)
		}
		won, err := s.finishQuestionRun(run)
		if err != nil {
			s.record(protocol.Activity{Actor: "Rashin", Action: "routine.record", Target: info.Title, OK: false, Detail: err.Error()})
			return
		}
		if !won {
			continue
		}
		// share the existing failure and quiet-streak policy without making a team task
		s.routineTaskEnded(protocol.TeamTask{ID: info.ID, Routine: run.Routine, Status: status})
		title := i18n.T("Your scheduled question is ready")
		if status != protocol.TaskDone {
			title = i18n.T("Your scheduled question did not finish")
		}
		if quiet := s.settings().Teams.QuietHours; quiet == nil || !quiet.Covers(s.now()) {
			s.startWorker(func() { s.post(notification{Title: title, Body: info.Preview}, "ask/"+info.ID) })
		}
	}
}

func (s *Service) finishQuestionRun(run protocol.RoutineRun) (bool, error) {
	s.routineRunsMu.Lock()
	defer s.routineRunsMu.Unlock()
	runs, err := s.readRoutineRuns()
	if err != nil {
		return false, err
	}
	for index, prior := range runs {
		if prior.ID == run.ID {
			if prior.Ended != nil {
				return false, nil
			}
			runs[index] = run
			return true, s.writeRoutineRuns(runs, run)
		}
	}
	return false, nil
}

func (s *Service) stopScheduledQuestions() {
	result, err := s.runHistory(protocol.RoutineRunsParams{})
	if err != nil {
		s.record(protocol.Activity{Actor: "Rashin", Action: "routine.record", OK: false, Detail: err.Error()})
		return
	}
	for _, run := range result.Runs {
		if run.Kind != protocol.RoutineAsk || run.Ended != nil {
			continue
		}
		s.finishRoutineRun(&run, protocol.TaskStopped, i18n.T("Scheduled runs were turned off."))
		won, err := s.finishQuestionRun(run)
		if err != nil {
			s.record(protocol.Activity{Actor: "Rashin", Action: "routine.record", OK: false, Detail: err.Error()})
			continue
		}
		if !won {
			continue
		}
		if s.sessions != nil && run.Session != "" {
			_ = s.sessions.Cancel(s.background, run.Session)
			_ = s.sessions.Stop(s.background, run.Session)
		}
		if routine, ok := s.clock.change(run.Routine, func(r *protocol.Routine) bool {
			if r.LastTask != run.Session {
				return false
			}
			r.LastStatus = protocol.TaskStopped
			return true
		}); ok {
			s.broadcast(protocol.EvtRoutine, s.fillRoutine(routine, s.now()))
		}
	}
}
