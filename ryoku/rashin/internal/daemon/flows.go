package daemon

import (
	"context"
	"strings"

	i18n "ryoku-i18n"
	"ryoku-rashin/internal/paths"
	"ryoku-rashin/internal/protocol"
)

func (d *Daemon) ask(ctx context.Context, c *connection, p protocol.AskParams) (protocol.AskResult, error) {
	id := p.Session
	if id == "" {
		origin := p.Origin
		if origin == "" {
			origin = protocol.OriginCLI
		}
		info, err := d.manager.newSession(ctx, protocol.SessionNewParams{Kind: protocol.KindAsk, Model: p.Model, Origin: origin})
		if err != nil {
			return protocol.AskResult{}, err
		}
		id = info.ID
	}
	s, err := d.manager.get(id)
	if err != nil {
		return protocol.AskResult{}, err
	}
	info := s.log.Info()
	if info.Kind != protocol.KindAsk {
		return protocol.AskResult{}, protocol.Errorf(protocol.CodeBadRequest, "%s", i18n.Tf("Session %s is not an Ask session.", id))
	}
	since := info.LastSeq
	if _, err := d.manager.attach(id, since, c); err != nil {
		return protocol.AskResult{}, err
	}
	result, _, err := d.manager.prompt(ctx, protocol.PromptParams{ID: id, Text: p.Question, Images: p.Images, Terminal: p.Terminal}, nil)
	if err != nil {
		return protocol.AskResult{}, err
	}
	return protocol.AskResult{Session: s.log.Info(), Turn: result.Turn, Since: since}, nil
}

func (d *Daemon) handoff(ctx context.Context, p protocol.HandoffParams) (protocol.SessionInfo, error) {
	source, err := d.manager.get(p.ID)
	if err != nil {
		return protocol.SessionInfo{}, err
	}
	if source.log.Info().Kind != protocol.KindAsk {
		return protocol.SessionInfo{}, protocol.Errorf(protocol.CodeBadRequest, "%s", i18n.T("Only an Ask session can be handed off."))
	}
	last, err := d.manager.lastExchange(p.ID)
	if err != nil {
		return protocol.SessionInfo{}, err
	}
	cwd := p.Cwd
	if cwd == "" && p.Project == "" && p.Worktree == "" {
		cwd = paths.Home()
	}
	info, err := d.manager.newSession(ctx, protocol.SessionNewParams{
		Kind: protocol.KindAgent, Harness: p.Harness, Cwd: cwd, Origin: protocol.OriginApp,
		Title: source.log.Info().Title, Project: p.Project, Worktree: p.Worktree,
		Base: p.Base, Branch: p.Branch, Name: p.Name,
	})
	if err != nil {
		return protocol.SessionInfo{}, err
	}
	target, err := d.manager.get(info.ID)
	if err != nil {
		return protocol.SessionInfo{}, err
	}
	target.mu.Lock()
	_ = target.log.Update(func(i *protocol.SessionInfo) { i.Parent = p.ID }, runnerState(target.runner))
	d.manager.emitInfoLocked(target)
	target.mu.Unlock()
	handoff := &protocol.Handoff{From: p.ID, Title: source.log.Info().Title, Question: last.question, Answer: last.answer, Reason: last.reason}
	text := strings.TrimSpace(p.Text)
	if text == "" {
		text = last.question
	}
	// The agent is handed what the question was about as well as the question.
	if p.Run {
		if _, _, err := d.manager.prompt(ctx, protocol.PromptParams{ID: info.ID, Text: text, Images: last.attached}, handoff); err != nil {
			return protocol.SessionInfo{}, err
		}
	} else {
		if err := target.coalescer.Emit(protocol.EvUser, protocol.UserData{Text: text, Images: last.attached, Handoff: handoff}); err != nil {
			return protocol.SessionInfo{}, err
		}
	}
	return target.log.Info(), nil
}
