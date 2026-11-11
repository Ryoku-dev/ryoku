package main

import (
	"context"
	"encoding/json"

	"ryoku-rashin/internal/mcp"
	"ryoku-rashin/internal/protocol"
)

// codeTools are the code search commands as tools over the index of the
// session folder
func codeTools(session mcpSession) []mcp.Tool {
	ref := protocol.IndexRef{Cwd: session.cwd}
	limit := integerProperty("Most results to return.")
	return []mcp.Tool{
		{
			Name:        "search",
			Description: "Search Rashin's index of the session folder by words or meaning. Returns ranked passages with file and lines. Use it before grep to find where something is done.",
			Input:       schema([]string{"query"}, map[string]any{"query": stringProperty("What you are looking for, in words or code."), "limit": limit}),
			ReadOnly:    true,
			Call: func(ctx context.Context, raw json.RawMessage) mcp.Result {
				var args struct {
					Query string `json:"query"`
					Limit int    `json:"limit"`
				}
				if err := toolArguments(raw, &args, "query"); err != nil {
					return mcp.Failure(err)
				}
				return toolCall(ctx, protocol.MIndexSearch, protocol.SearchParams{IndexRef: ref, Query: args.Query, Limit: args.Limit}, &protocol.SearchResult{})
			},
		},
		nameTool("find", "Find symbols by name: functions, types, methods, classes, constants. Class::method and Type.method both work.", protocol.MIndexFind, ref, &protocol.SymbolsResult{}),
		nameTool("def", "Read the source of one symbol, with the other places a symbol of that name is declared.", protocol.MIndexDef, ref, &protocol.DefResult{}),
		nameTool("references", "Find where a symbol is used.", protocol.MIndexReferences, ref, &protocol.ReferencesResult{}),
		pathTool("outline", "List what a file defines, with lines.", protocol.MIndexOutline, session, &protocol.SymbolsResult{}),
		pathTool("impact", "List the files that depend on a file.", protocol.MIndexImpact, session, &protocol.Impact{}),
		{
			Name:        "peek",
			Description: "Read lines of an indexed file.",
			Input: schema([]string{"path", "start"}, map[string]any{
				"path":  stringProperty("File path relative to the session folder."),
				"start": integerProperty("First line."),
				"end":   integerProperty("Last line; one screen after start when left out."),
			}),
			ReadOnly: true,
			Call: func(ctx context.Context, raw json.RawMessage) mcp.Result {
				var args struct {
					Path  string `json:"path"`
					Start int    `json:"start"`
					End   int    `json:"end"`
				}
				if err := toolArguments(raw, &args, "path", "start"); err != nil {
					return mcp.Failure(err)
				}
				return toolCall(ctx, protocol.MIndexPeek, protocol.PeekParams{IndexRef: ref, Path: session.path(args.Path), Start: args.Start, End: args.End}, &protocol.PeekResult{})
			},
		},
	}
}

func nameTool(name, description, method string, ref protocol.IndexRef, result any) mcp.Tool {
	return mcp.Tool{
		Name:        name,
		Description: description,
		Input:       schema([]string{"name"}, map[string]any{"name": stringProperty("The symbol name."), "limit": integerProperty("Most results to return.")}),
		ReadOnly:    true,
		Call: func(ctx context.Context, raw json.RawMessage) mcp.Result {
			var args struct {
				Name  string `json:"name"`
				Limit int    `json:"limit"`
			}
			if err := toolArguments(raw, &args, "name"); err != nil {
				return mcp.Failure(err)
			}
			return toolCall(ctx, method, protocol.NameParams{IndexRef: ref, Name: args.Name, Limit: args.Limit}, result)
		},
	}
}

func pathTool(name, description, method string, session mcpSession, result any) mcp.Tool {
	return mcp.Tool{
		Name:        name,
		Description: description,
		Input:       schema([]string{"path"}, map[string]any{"path": stringProperty("File path relative to the session folder.")}),
		ReadOnly:    true,
		Call: func(ctx context.Context, raw json.RawMessage) mcp.Result {
			var args struct {
				Path string `json:"path"`
			}
			if err := toolArguments(raw, &args, "path"); err != nil {
				return mcp.Failure(err)
			}
			return toolCall(ctx, method, protocol.PathParams{IndexRef: protocol.IndexRef{Cwd: session.cwd}, Path: session.path(args.Path)}, result)
		},
	}
}
