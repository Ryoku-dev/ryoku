package daemon

import (
	"context"
	"os"
	"strings"

	i18n "ryoku-i18n"
	"ryoku-rashin/internal/hands/nest"
	"ryoku-rashin/internal/oni/hands/loop"
	"ryoku-rashin/internal/oni/hands/task"
	"ryoku-rashin/internal/protocol"
)

// The task journal keeps each task outside a tab: its state, scope, app
// id, step and times, never its goal, a title or a typed word. One task
// runs; others wait for their own Start button. When the daemon starts,
// every task that was running or waiting becomes interrupted and none
// resumes: the person picks again and presses Start with the goal said
// again. Startup starts no input program and no nested desktop.

// handsJournal is the task journal, opened on first use.
func (d *Daemon) handsJournal() (*task.Journal, error) {
	h := &d.oni.hands
	h.journalMu.Lock()
	defer h.journalMu.Unlock()
	if h.journal == nil {
		h.journal = task.Open(d.handsJournalPath())
	}
	return h.journal, nil
}

// handsBoot runs once as the daemon starts: tasks a dead daemon left
// running or waiting are marked interrupted, old rows are purged, and a
// nested desktop a killed daemon left behind is swept away. Nothing is
// started.
func (d *Daemon) handsBoot() {
	if _, err := os.Stat(d.handsJournalPath()); err == nil {
		j, _ := d.handsJournal()
		now := d.handsClock().Now()
		if held, err := j.Recover(now); err != nil {
			d.warn("oni hands journal: %v", err)
		} else if len(held) > 0 {
			d.warn("oni hands: %d task(s) interrupted by a restart; each waits for its Start button", len(held))
		}
		if err := j.Purge(now); err != nil {
			d.warn("oni hands journal: %v", err)
		}
	}
	if d.deps.OniHands.Nest != nil {
		return
	}
	if dir := handsNestDir(os.Getenv); dirExists(dir) {
		if err := nest.Sweep(dir); err != nil {
			d.warn("oni hands desktop: %v", err)
		}
	}
}

func dirExists(dir string) bool {
	info, err := os.Lstat(dir)
	return err == nil && info.IsDir()
}

// handsTasksCall lists the journal, the earliest queued first.
func (d *Daemon) handsTasksCall() (protocol.OniHandsResult, error) {
	j, err := d.handsJournal()
	if err != nil {
		return protocol.OniHandsResult{}, err
	}
	rows, err := j.List()
	if err != nil {
		return protocol.OniHandsResult{}, err
	}
	res := handsWait(d, d.handsResult)
	res.Tasks = make([]protocol.OniHandsTaskRow, 0, len(rows))
	for _, t := range rows {
		res.Tasks = append(res.Tasks, protocol.OniHandsTaskRow{
			ID: t.ID, State: string(t.State), Scope: t.Scope, App: t.App, Origin: t.Origin,
			Step: t.Step, At: t.At, Ended: t.Ended, Reason: t.Reason,
		})
	}
	return res, nil
}

// handsClearCall stops the running task and deletes every journal row and
// kept project.
func (d *Daemon) handsClearCall() (protocol.OniHandsResult, error) {
	handsWait(d, func() struct{} {
		if run := d.oni.hands.run; run != nil && run.ex != nil {
			d.handsStopRun(run, handsByApp)
		}
		return struct{}{}
	})
	j, err := d.handsJournal()
	if err != nil {
		return protocol.OniHandsResult{}, err
	}
	if err := j.ForgetAll(); err != nil {
		return protocol.OniHandsResult{}, err
	}
	res := handsWait(d, d.handsResult)
	res.Tasks = []protocol.OniHandsTaskRow{}
	res.Line = i18n.T("Hands' tasks are forgotten.")
	return res, nil
}

// handsJournalFrame keeps the running task's row in step with its frame:
// working, or needing the person while a card waits or a pause holds. It
// writes only when the state, step or reason changes. On the queue.
func (d *Daemon) handsJournalFrame(run *handsRun, phase string, step int, pause string) {
	if run.task == "" {
		return
	}
	st := task.Working
	reason := ""
	switch phase {
	case protocol.OniHandsStateWaiting:
		st = task.NeedsYou
	case protocol.OniHandsStatePaused:
		st, reason = task.NeedsYou, pause
	}
	step = run.stepBase + step
	if st == run.state && step == run.written && reason == run.reason {
		return
	}
	j, err := d.handsJournal()
	if err != nil {
		return
	}
	if _, err := j.Update(run.task, st, step, reason, d.handsClock().Now()); err != nil {
		d.warn("oni hands journal: %v", err)
		return
	}
	run.state, run.written, run.reason = st, step, reason
}

// handsJournalEnd ends the running task's row: done, stopped or failed,
// with the loop's word for how it ended. A failed task is never retried.
// On the queue.
func (d *Daemon) handsJournalEnd(run *handsRun, e loop.End, step int) {
	if run.task == "" {
		return
	}
	st := task.Failed
	switch e.Reason {
	case loop.EndDone, loop.EndAsked, loop.EndAnswered:
		st = task.Done
	case loop.EndStopped:
		st = task.Stopped
	}
	j, err := d.handsJournal()
	if err != nil {
		return
	}
	if _, err := j.Update(run.task, st, max(run.stepBase+step, run.written), e.Reason, d.handsClock().Now()); err != nil {
		d.warn("oni hands journal: %v", err)
	}
	run.task = ""
}

// handsGranted starts Hyakki's own loop toward goal for a team member a
// Yokai harness hosts, once the person allowed its request in the app or
// on the pill: in scope nested, or in the live window named window. The
// harness supplies the goal only; every step is Hands' own, under the same
// rules. A harness never reaches the desktop scope, a task never queues for
// it, and live is asked before every action: once the approval no longer
// holds the task stops. It answers the journal task's id.
func (d *Daemon) handsGranted(ctx context.Context, goal, scope, window string, live func() bool) (string, error) {
	if live == nil || !live() {
		return "", protocol.Errorf(protocol.CodeForbidden, "%s", i18n.T("That approval no longer holds."))
	}
	switch scope {
	case protocol.OniHandsScopeNested:
		if window != "" {
			return "", protocol.Errorf(protocol.CodeBadRequest, "%s", i18n.T("A task on the separate desktop names no window."))
		}
	case protocol.OniHandsScopeWindow:
		if !protocol.ValidOniHandsWindowID(window) {
			return "", protocol.Errorf(protocol.CodeBadRequest, "%s", i18n.T("A task in a window names the window by its id."))
		}
	default:
		return "", protocol.Errorf(protocol.CodeForbidden, "%s", i18n.T("A team's request may use Hands only on the separate desktop or in one window you picked."))
	}
	goal = strings.TrimSpace(goal)
	if goal == "" || !protocol.ValidOniText(goal, protocol.OniHandsGoalMax, false) {
		return "", protocol.Errorf(protocol.CodeBadRequest, "%s", i18n.Tf("A goal is 1 to %d characters on one line.", protocol.OniHandsGoalMax))
	}
	if ctx.Err() != nil {
		return "", ctx.Err()
	}
	if scope == protocol.OniHandsScopeWindow {
		if protocol.EffectiveOni(d.Settings()).Hands != protocol.OniHandsApp {
			return "", protocol.Errorf(protocol.CodeForbidden, "%s", handsLineNoStage)
		}
		if _, err := d.handsSelect(window); err != nil {
			return "", err
		}
	}
	res, err := d.handsBeginApp(handsAppStart{scope: scope, goal: goal, origin: task.OriginHarness, grant: live})
	if err != nil {
		return "", err
	}
	return res.Task, nil
}
