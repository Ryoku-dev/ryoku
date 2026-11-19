package daemon

import (
	"context"
	"strings"

	i18n "ryoku-i18n"
	"ryoku-rashin/internal/protocol"
)

func validateGlance(p protocol.GlanceParams) error {
	if protocol.GlanceInstruction(p) == "" {
		return protocol.Errorf(protocol.CodeBadRequest, "%s", i18n.T("Choose a supported Theia action."))
	}
	if p.Session == "" && len(p.Images) == 0 && strings.TrimSpace(p.Text) == "" {
		return protocol.Errorf(protocol.CodeBadRequest, "%s", i18n.T("Show Theia an image or some text first."))
	}
	if p.Mode == "ask" && strings.TrimSpace(p.Question) == "" {
		return protocol.Errorf(protocol.CodeBadRequest, "%s", i18n.T("Ask a question about this capture."))
	}
	return nil
}

func (d *Daemon) glance(ctx context.Context, c *connection, p protocol.GlanceParams) (protocol.AskResult, error) {
	if err := validateGlance(p); err != nil {
		return protocol.AskResult{}, err
	}
	id := p.Session
	if id == "" {
		// with no model picked, a capture is light work: the smallest model
		// that sees images answers it, a local one first
		info, err := d.manager.newSession(ctx, protocol.SessionNewParams{Kind: protocol.KindAsk, Origin: protocol.OriginSight, Temporary: true, Model: p.Model, Mode: protocol.ApprovalsReadOnly})
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
		return protocol.AskResult{}, protocol.Errorf(protocol.CodeBadRequest, "%s", i18n.T("Theia can only continue an Ask conversation."))
	}
	if !c.app && info.Mode != "" && info.Mode != protocol.ApprovalsReadOnly {
		return protocol.AskResult{}, protocol.Errorf(protocol.CodeForbidden, "%s", i18n.T("Theia cannot use a conversation that allows changes to the machine."))
	}
	since := info.LastSeq
	if _, err := d.manager.attach(id, since, c); err != nil {
		return protocol.AskResult{}, err
	}
	text := p.Text
	if p.Mode == "ask" {
		text = strings.TrimSpace(p.Question + "\n\n" + text)
	}
	options := promptTurn{glance: &p}
	result, _, err := d.manager.promptOptions(ctx, protocol.PromptParams{ID: id, Text: text, Images: p.Images}, options)
	if err != nil {
		return protocol.AskResult{}, err
	}
	return protocol.AskResult{Session: s.log.Info(), Turn: result.Turn, Since: since}, nil
}
