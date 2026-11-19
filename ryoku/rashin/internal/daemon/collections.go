package daemon

import (
	"context"

	"ryoku-i18n"
	"ryoku-rashin/internal/collections"
	"ryoku-rashin/internal/protocol"
)

func (d *Daemon) collectionsGate() error {
	if !d.feature("collections") {
		return protocol.Errorf(protocol.CodeForbidden, "%s", i18n.T("Turn on Collections in Settings first."))
	}
	return nil
}

func (d *Daemon) collectionsList(ctx context.Context, c *connection, p protocol.CollectionsListParams) (protocol.CollectionsListResult, error) {
	if err := d.collectionsGate(); err != nil {
		return protocol.CollectionsListResult{}, err
	}
	items, err := collections.New().List()
	return protocol.CollectionsListResult{Collections: items}, err
}

func (d *Daemon) collectionsSave(ctx context.Context, c *connection, p protocol.CollectionSaveParams) (protocol.Collection, error) {
	if err := d.collectionsGate(); err != nil {
		return protocol.Collection{}, err
	}
	return collections.New().Save(p)
}

func (d *Daemon) collectionsAdd(ctx context.Context, c *connection, p protocol.CollectionAddParams) (protocol.CollectionItem, error) {
	if err := d.collectionsGate(); err != nil {
		return protocol.CollectionItem{}, err
	}
	return collections.New().Add(p)
}

func (d *Daemon) collectionsItems(ctx context.Context, c *connection, p protocol.CollectionItemsParams) (protocol.CollectionItemsResult, error) {
	if err := d.collectionsGate(); err != nil {
		return protocol.CollectionItemsResult{}, err
	}
	items, err := collections.New().Items(p.Collection)
	if err != nil {
		return protocol.CollectionItemsResult{}, err
	}
	return collections.Page(items, p)
}

func (d *Daemon) collectionsRemove(ctx context.Context, c *connection, p protocol.CollectionRemoveParams) (protocol.Empty, error) {
	if err := d.collectionsGate(); err != nil {
		return protocol.Empty{}, err
	}
	return protocol.Empty{}, collections.New().Remove(p.Collection, p.ID)
}

func (d *Daemon) collectionsCheck(ctx context.Context, c *connection, p protocol.CollectionCheckParams) (protocol.CollectionItem, error) {
	if err := d.collectionsGate(); err != nil {
		return protocol.CollectionItem{}, err
	}
	return collections.New().Check(ctx, p.Collection, p.ID)
}

func (d *Daemon) collectionsChat(ctx context.Context, c *connection, p protocol.CollectionChatParams) (protocol.AskResult, error) {
	if err := d.collectionsGate(); err != nil {
		return protocol.AskResult{}, err
	}
	items, err := collections.New().Items(p.Collection)
	if err != nil {
		return protocol.AskResult{}, err
	}
	if len(items) == 0 {
		return protocol.AskResult{}, protocol.Errorf(protocol.CodeBadRequest, "%s", i18n.T("Save something in this collection first."))
	}
	text, files := collections.Chat(items, p.Question)
	info, err := d.manager.newSession(ctx, protocol.SessionNewParams{Kind: protocol.KindAsk, Mode: protocol.ApprovalsReadOnly})
	if err != nil {
		return protocol.AskResult{}, err
	}
	if _, err = d.manager.attach(info.ID, info.LastSeq, c); err != nil {
		return protocol.AskResult{}, err
	}
	result, _, err := d.manager.promptOptions(ctx, protocol.PromptParams{ID: info.ID, Text: text, Images: files}, promptTurn{})
	if err != nil {
		return protocol.AskResult{}, err
	}
	return protocol.AskResult{Session: info, Turn: result.Turn, Since: info.LastSeq}, nil
}
