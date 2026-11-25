package memory

import (
	"context"
	"encoding/json"
	"strings"

	i18n "ryoku-i18n"

	"ryoku-rashin/internal/core"
	"ryoku-rashin/internal/protocol"
)

// Bounds on what a program on the MCP bridge sends and reads.
const (
	bridgeSearchMax = 10
	bridgeNoteMax   = 500
)

// handleBridge serves a program on the MCP bridge (from.Bridge): a search of
// what is known about the user, its sensitive memories hidden, and a note
// suggested about the user, which waits for the user to keep or dismiss it
// and is never kept as a fact. Nothing else, whatever the daemon's gate let
// through.
func (e *Engine) handleBridge(ctx context.Context, from core.MemoryCaller, method string, params json.RawMessage) (any, error) {
	if method != protocol.MMemorySearch && method != protocol.MMemoryImport {
		return nil, errAppOnly()
	}
	if !e.on() {
		return nil, protocol.Errorf(protocol.CodeDisabled, "%s", i18n.T("Memory is off."))
	}
	if method == protocol.MMemorySearch {
		p, err := decode[protocol.MemorySearchParams](params)
		if err != nil {
			return nil, err
		}
		limit := p.Limit
		if limit <= 0 || limit > bridgeSearchMax {
			limit = bridgeSearchMax
		}
		res, err := e.searchScopes(ctx, protocol.SessionInfo{}, p.Query, []string{protocol.MemoryYou}, limit)
		for index := range res.Memories {
			if res.Memories[index].Sensitive {
				res.Memories[index].Text = i18n.T("(a sensitive note, hidden)")
			}
		}
		return res, err
	}
	p, err := decode[protocol.MemoryImportParams](params)
	if err != nil {
		return nil, err
	}
	return e.bridgeSuggest(ctx, from, p.Text)
}

// bridgeSuggest keeps text as one suggestion about the user from the
// program from.Bridge names, after the refusals every write gets. A note
// the user asked to forget, or one already known or suggested, suggests
// nothing.
func (e *Engine) bridgeSuggest(ctx context.Context, from core.MemoryCaller, text string) (protocol.MemoryChange, error) {
	text = strings.Join(strings.Fields(text), " ")
	if text == "" || len([]rune(text)) > bridgeNoteMax {
		return protocol.MemoryChange{}, protocol.Errorf(protocol.CodeBadRequest, "%s", i18n.Tf("A suggested note is one statement of at most %d characters.", bridgeNoteMax))
	}
	if err := e.screen(from, "", text, false); err != nil {
		return protocol.MemoryChange{}, err
	}
	if factProblem(text) != "" {
		return protocol.MemoryChange{}, protocol.Errorf(protocol.CodeBadRequest, "%s", i18n.T("That does not read as a note about the user."))
	}
	st, err := e.store()
	if err != nil {
		return protocol.MemoryChange{}, err
	}
	if gone, err := st.Tombstoned(ctx, text, protocol.MemoryYou); err != nil || gone {
		return protocol.MemoryChange{}, err
	}
	if _, dup, err := st.Duplicate(ctx, text, protocol.MemoryYou); err != nil || dup {
		return protocol.MemoryChange{}, err
	}
	source := protocol.MemorySource{Kind: protocol.MemoryFromImport, From: from.Bridge}
	change, err := st.Apply(Origin{By: protocol.MemoryByImport}, []Op{{Kind: OpSuggest, Text: text, Scope: protocol.MemoryYou, Retain: protocol.MemoryDurable, Source: source}})
	if err != nil {
		if refused, ok := errRefused(err); ok {
			return change, protocol.Errorf(protocol.CodeBadRequest, "%s", refused.Detail)
		}
		return change, err
	}
	if len(change.Items) > 0 {
		e.changed(change)
	}
	return change, nil
}
