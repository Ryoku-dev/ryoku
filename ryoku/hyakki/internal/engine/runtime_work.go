package engine

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	"ryoku-rashin/internal/core"
	"ryoku-rashin/internal/engine/exec"
	"ryoku-rashin/internal/protocol"
	"ryoku-rashin/internal/sandbox"
)

func (r *runtimeRunner) command(ctx context.Context, in core.PromptInput, sink core.Sink, spec *sandbox.Spec, environment map[string]string) (core.TurnResult, error) {
	result := core.TurnResult{Stop: protocol.StopEnd, Usage: &protocol.TokenUsage{Unmetered: true}}
	x := r.getExecutor(spec)
	if x == nil {
		return result, errors.New("runtime sandbox executor is unavailable")
	}
	payload, _ := json.Marshal(r.work(in))
	id, err := x.Start(ctx, exec.StartRequest{Argv: r.config.Command, Dir: r.opts.Cwd, Env: environment, Stdin: payload, Keep: runtimeOutputLimit}, nil)
	if err != nil {
		if errors.Is(err, exec.ErrNotFound) {
			return result, errors.New("configured command executable is missing inside the team sandbox")
		}
		return result, errors.New("configured command could not start in the team sandbox")
	}
	released := false
	defer func() {
		if released {
			return
		}
		cleanup, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = x.Kill(cleanup, id)
		_ = x.Release(cleanup, id)
	}()
	exit, err := x.Wait(ctx, id)
	if err != nil {
		if ctx.Err() != nil {
			return result, ctx.Err()
		}
		return result, errors.New("runtime command could not be monitored")
	}
	text := runtimeClip(r.redact(exit.Output), runtimeOutputLimit)
	if exit.Truncated {
		text += "\n[executor output truncated]"
	}
	r.emitText(sink, text)
	if exit.Code != 0 || exit.Killed || exit.TimedOut || exit.Signal != "" || exit.Running {
		return result, fmt.Errorf("runtime command failed (exit %d, signal %q, timed out %t)", exit.Code, exit.Signal, exit.TimedOut)
	}
	if ctx.Err() != nil {
		return result, ctx.Err()
	}
	cleanup, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := x.Kill(cleanup, id); err != nil {
		return result, errors.New("command process group could not be stopped")
	}
	if err := x.Release(cleanup, id); err != nil {
		return result, errors.New("command process could not be released")
	}
	released = true
	if err := x.Close(); err != nil {
		return result, errors.New("command sandbox processes could not be stopped")
	}
	r.mu.Lock()
	r.executor = nil
	r.mu.Unlock()
	// An exit is observed work, unlike a successful spawn. Empty-output
	// commands still report their actual exit, never an invented narrative.
	body := "Configured command exited successfully (exit 0)."
	if text != "" {
		body += "\n\n" + text
	}
	if r.opts.Tools.Team.Dream {
		return result, errors.New("a command cannot finish a Dream: use an ACP manager with governance tools")
	}
	return result, r.report(ctx, "Command task completed", body, nil, false)
}

// runtimeResponse is the complete synchronous webhook report contract. A
// receiver returns HTTP 200 with version=1, matching session/task/turn,
// status="completed", and a non-empty report.body (or quiet=true). HTTP 202,
// waiting, truncation, redirects and all other statuses are NOT completion.
// usage is optional; absent accounting is always Unmetered. There is no poll,
// retry, callback URL or bearer run token in the work envelope.
type runtimeResponse struct {
	Version int    `json:"version"`
	Session string `json:"session"`
	Task    string `json:"task"`
	Turn    int    `json:"turn"`
	Status  string `json:"status"`
	Report  struct {
		Title string   `json:"title"`
		Body  string   `json:"body"`
		Files []string `json:"files,omitempty"`
		Quiet bool     `json:"quiet,omitempty"`
	} `json:"report"`
	Usage *protocol.TokenUsage `json:"usage,omitempty"`
}

func (r *runtimeRunner) webhook(ctx context.Context, in core.PromptInput, sink core.Sink) (core.TurnResult, error) {
	result := core.TurnResult{Stop: protocol.StopEnd, Usage: &protocol.TokenUsage{Unmetered: true}}
	payload, _ := json.Marshal(r.work(in))
	response, err := r.opts.RuntimeHTTP(ctx, payload)
	if err != nil {
		if ctx.Err() != nil {
			return result, ctx.Err()
		}
		return result, errors.New("configured webhook request failed")
	}
	if ctx.Err() != nil {
		return result, ctx.Err()
	}
	if response.Waiting != "" {
		return result, errors.New("webhook is waiting for central approval; no work report received")
	}
	if response.Status != 200 {
		return result, fmt.Errorf("webhook returned HTTP %d, not a completed report; no automatic retry", response.Status)
	}
	if response.Truncated || len(response.Body) > runtimeOutputLimit {
		return result, errors.New("webhook response exceeds the complete 256 KiB report limit")
	}
	var report runtimeResponse
	decoder := json.NewDecoder(strings.NewReader(response.Body))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&report); err != nil {
		return result, errors.New("webhook response is not a versioned runtime report")
	}
	var extra any
	if decoder.Decode(&extra) != io.EOF {
		return result, errors.New("webhook response contains trailing or malformed JSON")
	}
	if report.Version != 1 || report.Session != r.opts.Session.ID || report.Task != r.opts.Tools.Team.Task || report.Turn != in.Turn || report.Status != "completed" {
		return result, errors.New("webhook did not return completion for this exact task turn")
	}
	if report.Usage != nil {
		if report.Usage.Input < 0 || report.Usage.Output < 0 {
			return result, errors.New("webhook supplied invalid token usage")
		}
		result.Usage = report.Usage
	}
	if r.opts.Tools.Team.Dream {
		return result, errors.New("a webhook report cannot finish a Dream; use an ACP manager with governance tools")
	}
	if err := r.report(ctx, report.Report.Title, report.Report.Body, report.Report.Files, report.Report.Quiet); err != nil {
		return result, err
	}
	r.emitText(sink, report.Report.Body)
	return result, nil
}
