package daemon

import (
	"context"

	"ryoku-i18n"
	"ryoku-rashin/internal/prompts"
	"ryoku-rashin/internal/protocol"
)

func (d *Daemon) promptsAllowed() error {
	if !d.feature("prompts") {
		return protocol.Errorf(protocol.CodeForbidden, "%s", i18n.T("Turn on Prompt library in Settings first"))
	}
	return nil
}

func (d *Daemon) promptsList(ctx context.Context, c *connection, p protocol.PromptsListParams) (protocol.PromptsListResult, error) {
	if err := d.promptsAllowed(); err != nil {
		return protocol.PromptsListResult{}, err
	}
	items, err := prompts.List()
	return protocol.PromptsListResult{Prompts: items}, err
}

func (d *Daemon) promptsSave(ctx context.Context, c *connection, p protocol.PromptSaveParams) (protocol.Prompt, error) {
	if err := d.promptsAllowed(); err != nil {
		return protocol.Prompt{}, err
	}
	item, err := prompts.Save(p)
	if err != nil {
		return protocol.Prompt{}, protocol.Errorf(protocol.CodeBadRequest, "%s", err.Error())
	}
	return item, nil
}

func (d *Daemon) promptsDelete(ctx context.Context, c *connection, p protocol.PromptDeleteParams) (protocol.Empty, error) {
	if err := d.promptsAllowed(); err != nil {
		return protocol.Empty{}, err
	}
	if err := prompts.Delete(p.ID); err != nil {
		return protocol.Empty{}, protocol.Errorf(protocol.CodeBadRequest, "%s", err.Error())
	}
	return protocol.Empty{}, nil
}

func (d *Daemon) promptsExpand(ctx context.Context, c *connection, p protocol.PromptExpandParams) (protocol.PromptExpandResult, error) {
	if err := d.promptsAllowed(); err != nil {
		return protocol.PromptExpandResult{}, err
	}
	text, err := prompts.ExpandID(p)
	if err != nil {
		return protocol.PromptExpandResult{}, protocol.Errorf(protocol.CodeBadRequest, "%s", err.Error())
	}
	return protocol.PromptExpandResult{Text: text}, nil
}
