// Package code exposes the session's indexed source and Git history as read-only tools.
package code

import (
	"context"
	"encoding/json"
	"path/filepath"
	"strings"

	i18n "ryoku-i18n"
	"ryoku-rashin/internal/engine/tool"
	"ryoku-rashin/internal/llm"
	"ryoku-rashin/internal/protocol"
)

// Tools returns the code intelligence and Git read tools for the engine registry.
func Tools() []tool.Tool {
	return []tool.Tool{
		indexTool{tool.Search, protocol.MIndexSearch, protocol.ToolSearch, "query", "Search indexed source by words or meaning."},
		indexTool{tool.Find, protocol.MIndexFind, protocol.ToolSearch, "name", "Find symbols by name."},
		indexTool{tool.Def, protocol.MIndexDef, protocol.ToolRead, "name", "Read the source of one symbol."},
		indexTool{tool.Outline, protocol.MIndexOutline, protocol.ToolRead, "path", "List symbols defined by a file."},
		indexTool{tool.References, protocol.MIndexReferences, protocol.ToolSearch, "name", "Find uses of a symbol."},
		indexTool{tool.Impact, protocol.MIndexImpact, protocol.ToolSearch, "path", "List files depending on a file."},
		indexTool{tool.Peek, protocol.MIndexPeek, protocol.ToolRead, "path", "Read numbered lines of an indexed file."},
		gitTool{tool.GitStatus, protocol.ToolRead, "Show Git status of this session's worktree."},
		gitTool{tool.GitDiff, protocol.ToolRead, "Read a Git diff and its hunks."},
		gitTool{tool.GitLog, protocol.ToolRead, "Read recent commits in this worktree."},
		gitTool{tool.GitShow, protocol.ToolRead, "Read a commit and its changed files."},
	}
}

type indexTool struct{ name, method, kind, field, description string }

func (t indexTool) Name() string    { return t.name }
func (t indexTool) Kind() tool.Kind { return t.kind }
func (t indexTool) ReadOnly() bool  { return true }
func (t indexTool) Spec(p tool.Profile) llm.Tool {
	props := map[string]any{}
	required := []string{}
	if t.field != "" {
		props[t.field] = map[string]any{"type": "string"}
		required = append(required, t.field)
	}
	if t.field == "name" || t.field == "query" {
		props["limit"] = map[string]any{"type": "integer"}
	}
	if t.name == tool.Peek {
		props["start"] = map[string]any{"type": "integer"}
		props["end"] = map[string]any{"type": "integer"}
		required = append(required, "start")
	}
	if t.name == tool.Search {
		props["mode"] = map[string]any{"type": "string", "enum": []string{protocol.SearchHybrid, protocol.SearchLexical, protocol.SearchSemantic}}
	}
	raw, _ := json.Marshal(map[string]any{"type": "object", "properties": props, "required": required, "additionalProperties": false})
	return llm.Tool{Name: t.name, Description: t.description, Parameters: raw}
}
func (t indexTool) Present(raw json.RawMessage) tool.Presentation {
	var a map[string]any
	_ = json.Unmarshal(raw, &a)
	input, _ := a[t.field].(string)
	input, _ = tool.ClipRunes(input, 200)
	return tool.Presentation{Title: i18n.T("Reading code index"), Input: input}
}
func (t indexTool) Run(ctx context.Context, call tool.Call, env tool.Env) tool.Result {
	if env.Host == nil {
		return tool.Failed(i18n.T("The code index is unavailable."))
	}
	var a struct {
		Query string `json:"query"`
		Name  string `json:"name"`
		Path  string `json:"path"`
		Mode  string `json:"mode"`
		Limit int    `json:"limit"`
		Start int    `json:"start"`
		End   int    `json:"end"`
	}
	if err := json.Unmarshal(call.Args, &a); err != nil {
		return tool.Failed(err.Error())
	}
	ref := protocol.IndexRef{Cwd: env.Cwd}
	if ref.Cwd == "" {
		ref.Cwd = env.Session.Cwd
	}
	if ref.Cwd == "" {
		return tool.Failed(i18n.T("This session has no project folder."))
	}
	var params, result any
	switch t.name {
	case tool.Search:
		if strings.TrimSpace(a.Query) == "" {
			return tool.Failed(i18n.T("A search query is required."))
		}
		params = protocol.SearchParams{IndexRef: ref, Query: a.Query, Limit: a.Limit, Mode: a.Mode}
		result = &protocol.SearchResult{}
	case tool.Find, tool.Def, tool.References:
		if strings.TrimSpace(a.Name) == "" {
			return tool.Failed(i18n.T("A symbol name is required."))
		}
		params = protocol.NameParams{IndexRef: ref, Name: a.Name, Limit: a.Limit}
		switch t.name {
		case tool.Find:
			result = &protocol.SymbolsResult{}
		case tool.Def:
			result = &protocol.DefResult{}
		default:
			result = &protocol.ReferencesResult{}
		}
	case tool.Outline, tool.Impact:
		if strings.TrimSpace(a.Path) == "" {
			return tool.Failed(i18n.T("A file path is required."))
		}
		params = protocol.PathParams{IndexRef: ref, Path: indexPath(ref.Cwd, a.Path)}
		if t.name == tool.Outline {
			result = &protocol.SymbolsResult{}
		} else {
			result = &protocol.Impact{}
		}
	case tool.Peek:
		if strings.TrimSpace(a.Path) == "" || a.Start < 1 {
			return tool.Failed(i18n.T("A file path and first line are required."))
		}
		if a.End > 0 && a.End < a.Start {
			return tool.Failed(i18n.T("The last line must not precede the first."))
		}
		params = protocol.PeekParams{IndexRef: ref, Path: indexPath(ref.Cwd, a.Path), Start: a.Start, End: a.End}
		result = &protocol.PeekResult{}
	}
	if err := env.Host.Call(ctx, t.method, params, result); err != nil {
		return tool.Failed(err.Error())
	}
	return rendered(result, env)
}
func indexPath(root, path string) string {
	if filepath.IsAbs(path) {
		return filepath.Clean(path)
	}
	return filepath.Clean(filepath.Join(root, filepath.FromSlash(path)))
}
func rendered(v any, env tool.Env) tool.Result {
	body, err := json.Marshal(v)
	if err != nil {
		return tool.Failed(err.Error())
	}
	limit := env.Limits
	if limit.Told <= 0 {
		limit = env.Profile.Limits()
	}
	text := string(body)
	if len(text) > limit.Told {
		text = tool.HeadTail(text, limit.Head, limit.Tail)
	}
	return tool.Result{Text: text}
}
