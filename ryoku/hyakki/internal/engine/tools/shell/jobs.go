package shell

// Derived from oh-my-pi (MIT), packages/coding-agent/src/tools/wait.ts@c7fff67
// and packages/coding-agent/src/prompts/tools/wait.md@c7fff67 (bg_N job ids,
// returning at once with nothing to wait for).

import (
	"context"
	"encoding/json"
	"fmt"
	"slices"
	"strings"
	"sync"
	"time"

	i18n "ryoku-i18n"
	"ryoku-rashin/internal/engine/exec"
	"ryoku-rashin/internal/engine/tool"
	"ryoku-rashin/internal/llm"
	"ryoku-rashin/internal/protocol"
)

const (
	// Wait bounds how long one call blocks, not the process lifetime.
	waitDefault = 30 * time.Second
	waitMax     = 600 * time.Second
)

// job is a bash command left running in the background. The process
// belongs to the executor; the job only names it for the model.
type job struct {
	id      string
	process string
	command string
	started time.Time
	timeout time.Duration
	x       exec.Executor
}

type sessionJobs struct {
	// next keeps ids unique for the session's life, so a stale bg_1 never
	// names a newer job.
	next int
	jobs []*job
}

// jobTable holds every session's background jobs. A job is forgotten once
// wait collects it, or once its executor no longer knows the process
// (released, or closed with its session).
type jobTable struct {
	mu       sync.Mutex
	sessions map[string]*sessionJobs
}

func newJobTable() *jobTable { return &jobTable{sessions: map[string]*sessionJobs{}} }

// add registers a running process as the session's next job.
func (t *jobTable) add(session string, x exec.Executor, process, cmd string, timeout time.Duration, started time.Time) string {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.pruneLocked(session)
	s := t.sessions[session]
	if s == nil {
		s = &sessionJobs{}
		t.sessions[session] = s
	}
	s.next++
	j := &job{id: fmt.Sprintf("bg_%d", s.next), process: process, command: cmd, started: started, timeout: timeout, x: x}
	s.jobs = append(s.jobs, j)
	return j.id
}

// list is the session's jobs, oldest first.
func (t *jobTable) list(session string) []*job {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.pruneLocked(session)
	if s := t.sessions[session]; s != nil {
		return slices.Clone(s.jobs)
	}
	return nil
}

func (t *jobTable) remove(session, id string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if s := t.sessions[session]; s != nil {
		s.jobs = slices.DeleteFunc(s.jobs, func(j *job) bool { return j.id == id })
	}
}

func (t *jobTable) pruneLocked(session string) {
	if s := t.sessions[session]; s != nil {
		s.jobs = slices.DeleteFunc(s.jobs, func(j *job) bool { return !slices.Contains(j.x.Running(), j.process) })
	}
}

type waitTool struct{ jobs *jobTable }

func (waitTool) Name() string    { return tool.Wait }
func (waitTool) Kind() tool.Kind { return protocol.ToolExecute }

// ReadOnly is true: it reads job output, and stopping a job the session
// started is not a change to the user's machine.
func (waitTool) ReadOnly() bool { return true }

func (waitTool) Spec(tool.Profile) llm.Tool {
	return llm.Tool{
		Name: tool.Wait,
		Description: "Read or stop a background job that bash started. " +
			"It blocks until the job ends or timeout seconds pass (default 30, at most 600), then returns its output. " +
			"Without id it waits for the first of your jobs to end. stop true ends the job, or every job without id. " +
			"With no job running it returns at once. Wait only when you need the result and have nothing else to do.",
		Parameters: schema(`{"type":"object","properties":{` +
			`"id":{"type":"string","description":"Job id such as bg_1."},` +
			`"stop":{"type":"boolean"},` +
			`"timeout":{"type":"integer","minimum":1,"maximum":600,"description":"Seconds to block, default 30."}` +
			`},"additionalProperties":false}`),
	}
}

type waitArgs struct {
	id      string
	stop    bool
	timeout time.Duration
}

func decodeWait(raw json.RawMessage) (waitArgs, error) {
	fields, err := decodeFields(raw)
	if err != nil {
		return waitArgs{}, err
	}
	a := waitArgs{
		id:      strings.TrimSpace(stringField(fields, "id", "job", "job_id")),
		stop:    boolField(fields, "stop", "kill"),
		timeout: waitDefault,
	}
	if n, ok := intField(fields, "timeout"); ok && n > 0 {
		a.timeout = min(time.Duration(n)*time.Second, waitMax)
	}
	return a, nil
}

func (waitTool) Present(args json.RawMessage) tool.Presentation {
	a, _ := decodeWait(args)
	switch {
	case a.stop && a.id != "":
		return tool.Presentation{Title: i18n.Tf("Stopping %s", a.id)}
	case a.stop:
		return tool.Presentation{Title: i18n.T("Stopping background jobs")}
	case a.id != "":
		return tool.Presentation{Title: i18n.Tf("Waiting for %s", a.id)}
	}
	return tool.Presentation{Title: i18n.T("Waiting for background jobs")}
}

func (w waitTool) Run(ctx context.Context, call tool.Call, env tool.Env) tool.Result {
	a, err := decodeWait(call.Args)
	if err != nil {
		return tool.Failed(err.Error())
	}
	session := env.Session.ID
	all := w.jobs.list(session)
	if len(all) == 0 {
		return tool.Result{Text: i18n.T("Nothing to wait for: no background jobs are running.")}
	}
	targets := all
	if a.id != "" {
		i := slices.IndexFunc(all, func(j *job) bool { return j.id == a.id })
		if i < 0 {
			return tool.Failed(i18n.Tf("There is no background job %s. Jobs: %s.", a.id, jobNames(all)))
		}
		targets = all[i : i+1]
	}
	l := limitsOf(env)

	if a.stop {
		var reports []string
		failed := false
		for _, j := range targets {
			cleanup, cancel := stopContext(ctx)
			_ = j.x.Kill(cleanup, j.process)
			exit, err := j.x.Wait(cleanup, j.process)
			cancel()
			failed = failed || err != nil
			reports = append(reports, w.collect(ctx, session, j, exit, err, l))
		}
		result := w.result(session, reports, l)
		result.IsError = failed
		return result
	}

	// jobs that ended while nobody waited are reported first, without waiting
	var reports []string
	failed := false
	for _, j := range targets {
		if exit, err := j.x.Peek(ctx, j.process); err != nil || !exit.Running {
			failed = failed || jobFailed(exit, err)
			reports = append(reports, w.collect(ctx, session, j, exit, err, l))
		}
	}
	if len(reports) > 0 {
		result := w.result(session, reports, l)
		result.IsError = failed
		return result
	}

	waitCtx, cancel := context.WithTimeout(ctx, a.timeout)
	defer cancel()
	type ended struct {
		job  *job
		exit exec.Exit
		err  error
	}
	results := make(chan ended, len(targets))
	for _, j := range targets {
		go func() {
			exit, err := j.x.Wait(waitCtx, j.process)
			results <- ended{j, exit, err}
		}()
	}
	for range targets {
		got := <-results
		if waitCtx.Err() != nil && got.err != nil {
			continue
		}
		cancel()
		result := w.result(session, []string{w.collect(ctx, session, got.job, got.exit, got.err, l)}, l)
		result.IsError = jobFailed(got.exit, got.err)
		return result
	}
	if ctx.Err() != nil {
		return tool.Failed(i18n.T("Stopped waiting."))
	}
	for _, j := range targets {
		exit, _ := j.x.Peek(ctx, j.process)
		report := i18n.Tf("%s is still running after %d s: %s", j.id, seconds(time.Since(j.started)), clipLine(j.command, 200))
		if so := output(exit); so != "" {
			report += "\n" + i18n.T("Last output:") + "\n" + lastLines(so, l.TailLines)
		}
		reports = append(reports, report)
	}
	return tool.Result{Text: shape(strings.Join(reports, "\n\n"), l)}
}

func jobFailed(exit exec.Exit, err error) bool {
	return err != nil || exit.TimedOut || (!exit.Killed && (exit.Signal != "" || exit.Code != 0))
}

// collect reports one ended job and forgets it.
func (w waitTool) collect(ctx context.Context, session string, j *job, exit exec.Exit, err error, l tool.Limits) string {
	defer w.jobs.remove(session, j.id)
	defer release(ctx, j.x, j.process)
	if err != nil {
		return i18n.Tf("%s could not be read: %v", j.id, err)
	}
	took := exit.Elapsed
	if took == 0 {
		took = time.Since(j.started)
	}
	var head string
	switch {
	case exit.TimedOut:
		head = i18n.Tf("%s was stopped at its timeout of %d s: %s", j.id, seconds(j.timeout), clipLine(j.command, 200))
	case exit.Killed:
		head = i18n.Tf("%s was stopped after %d s: %s", j.id, seconds(took), clipLine(j.command, 200))
	case exit.Signal != "":
		head = i18n.Tf("%s was killed by signal %s after %d s: %s", j.id, exit.Signal, seconds(took), clipLine(j.command, 200))
	default:
		head = i18n.Tf("%s ended with exit code %d after %d s: %s", j.id, exit.Code, seconds(took), clipLine(j.command, 200))
	}
	body := output(exit)
	if body == "" {
		body = i18n.T("(no output)")
	}
	return head + "\n" + shape(body, l)
}

// result joins reports and names the jobs still running, so the model
// knows what it may still wait for.
func (w waitTool) result(session string, reports []string, l tool.Limits) tool.Result {
	text := strings.Join(reports, "\n\n")
	if left := w.jobs.list(session); len(left) > 0 {
		text += "\n\n" + i18n.Tf("Still running: %s.", jobNames(left))
	}
	return tool.Result{Text: shape(text, l)}
}

func jobNames(jobs []*job) string {
	names := make([]string, 0, len(jobs))
	for _, j := range jobs {
		names = append(names, j.id+" ("+clipLine(j.command, 60)+")")
	}
	return strings.Join(names, ", ")
}
