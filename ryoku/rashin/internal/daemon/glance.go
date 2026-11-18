package daemon

import (
	"context"
	"strings"

	i18n "ryoku-i18n"
	"ryoku-rashin/internal/harness"
	"ryoku-rashin/internal/protocol"
)

func glanceEngine(p protocol.GlanceParams, settings protocol.Settings) (string, error) {
	engine := strings.TrimSpace(p.Engine)
	if engine == "" || engine == "ask" {
		engine = settings.Ask.Harness
	}
	if engine == "providers" {
		engine = ""
	}
	if engine != "" && !harness.Known(engine, settings) {
		return "", protocol.Errorf(protocol.CodeBadRequest, "%s", i18n.Tf("The harness %s is not known to Rashin.", engine))
	}
	if p.Local && engine != "" {
		return "", protocol.Errorf(protocol.CodeLocalOnly, "%s", i18n.T("Local-only captures can use Rashin's providers, not an external harness. Choose Providers and a model on this computer."))
	}
	return engine, nil
}

func validateGlance(p protocol.GlanceParams) error {
	if protocol.GlanceInstruction(p) == "" {
		return protocol.Errorf(protocol.CodeBadRequest, "%s", i18n.T("Choose explain, ask, read or translate."))
	}
	if p.Session == "" && len(p.Images) == 0 && strings.TrimSpace(p.Text) == "" {
		return protocol.Errorf(protocol.CodeBadRequest, "%s", i18n.T("Show Sight an image or some text first."))
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
		engine, err := glanceEngine(p, d.manager.settings())
		if err != nil {
			return protocol.AskResult{}, err
		}
		model := p.Model
		if engine == "" && model == "" {
			model = protocol.RouteFast
		}
		info, err := d.manager.newSession(ctx, protocol.SessionNewParams{Kind: protocol.KindAsk, Origin: protocol.OriginSight, Temporary: true, Harness: engine, Model: model, Mode: protocol.ApprovalsReadOnly})
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
		return protocol.AskResult{}, protocol.Errorf(protocol.CodeBadRequest, "%s", i18n.T("Sight can only continue an Ask conversation."))
	}
	if p.Local && info.Harness != "" {
		return protocol.AskResult{}, protocol.Errorf(protocol.CodeLocalOnly, "%s", i18n.T("This conversation uses an external harness. Start a new capture with Providers for local-only answers."))
	}
	if !c.app && info.Mode != "" && info.Mode != protocol.ApprovalsReadOnly {
		return protocol.AskResult{}, protocol.Errorf(protocol.CodeForbidden, "%s", i18n.T("Sight cannot use a conversation that allows changes to the machine."))
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
	if info.Harness != "" {
		options.say = protocol.GlanceInstruction(p) + "\n\n" + text
	}
	result, _, err := d.manager.promptOptions(ctx, protocol.PromptParams{ID: id, Text: text, Images: p.Images}, options)
	if err != nil {
		return protocol.AskResult{}, err
	}
	return protocol.AskResult{Session: s.log.Info(), Turn: result.Turn, Since: since}, nil
}
