package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"

	i18n "ryoku-i18n"
	"ryoku-rashin/internal/client"
	"ryoku-rashin/internal/protocol"
)

type indexOptions struct {
	index  string
	limit  int
	format string
	all    bool
	base   string
}

func init() {
	register(&Command{Name: "index", Group: "index", Usage: "add|rm|list|refresh|status", Summary: i18n.T("Manage source indexes."), Run: runIndex})
	register(&Command{Name: "search", Group: "index", Usage: "[--index id] [--limit n] <query>", Summary: i18n.T("Search indexed source."), Run: runSearch})
	register(&Command{Name: "find", Group: "index", Usage: "[--index id] [--limit n] <name>", Summary: i18n.T("Find a symbol."), Run: runFind})
	register(&Command{Name: "def", Group: "index", Usage: "[--index id] <name>", Summary: i18n.T("Show a symbol definition."), Run: runDef})
	register(&Command{Name: "outline", Group: "index", Usage: "[--index id] <path>", Summary: i18n.T("Show a file outline."), Run: runOutline})
	register(&Command{Name: "references", Group: "index", Usage: "[--index id] [--limit n] <name>", Summary: i18n.T("Find symbol references."), Run: runReferences})
	register(&Command{Name: "refs", Group: "index", Usage: "[--index id] [--limit n] <name>", Summary: i18n.T("Find symbol references."), Run: runReferences})
	register(&Command{Name: "peek", Group: "index", Usage: "[--index id] <file:start[-end]>", Summary: i18n.T("Read indexed source lines."), Run: runPeek})
	register(&Command{Name: "overview", Group: "index", Usage: "[--index id]", Summary: i18n.T("Map an index."), Run: runOverview})
	register(&Command{Name: "impact", Group: "index", Usage: "[--index id] [--all] <path>", Summary: i18n.T("Show files that depend on a path."), Run: runImpact})
	register(&Command{Name: "wip", Group: "index", Usage: "[--index id]", Summary: i18n.T("Show working changes and their impact."), Run: runWip})
	register(&Command{Name: "changed", Group: "index", Usage: "[--index id] [--base ref]", Summary: i18n.T("Show changes against a base ref."), Run: runChanged})
}
func parseIndexOptions(args []string) (indexOptions, []string, error) {
	options := indexOptions{format: defaultOutputFormat()}
	var rest []string
	for index := 0; index < len(args); index++ {
		argument := args[index]
		value := func() (string, error) {
			if index+1 >= len(args) {
				return "", errors.New(i18n.Tf("Option %s needs a value.", argument))
			}
			index++
			return args[index], nil
		}
		switch argument {
		case "--index":
			v, err := value()
			if err != nil {
				return options, nil, err
			}
			options.index = v
		case "--limit":
			v, err := value()
			if err != nil {
				return options, nil, err
			}
			limit, err := strconv.Atoi(v)
			if err != nil || limit < 1 {
				return options, nil, errors.New(i18n.T("Limit must be a positive number."))
			}
			options.limit = limit
		case "--format":
			v, err := value()
			if err != nil {
				return options, nil, err
			}
			if v != "human" && v != "json" && v != "toon" {
				return options, nil, errors.New(i18n.T("Format must be human, json, or toon."))
			}
			options.format = v
		case "--all":
			options.all = true
		case "--base":
			v, err := value()
			if err != nil {
				return options, nil, err
			}
			options.base = v
		default:
			if strings.HasPrefix(argument, "-") {
				return options, nil, errors.New(i18n.Tf("Unknown option: %s", argument))
			}
			rest = append(rest, argument)
		}
	}
	return options, rest, nil
}
func indexRef(options indexOptions) protocol.IndexRef {
	cwd, _ := os.Getwd()
	return protocol.IndexRef{ID: options.index, Cwd: cwd}
}
func callIndex(method string, params, result any) error {
	conn, cancel, err := dial(true)
	if err != nil {
		return err
	}
	defer cancel()
	defer conn.Close()
	return conn.Call(context.Background(), method, params, result)
}
func indexError(err error) int {
	fmt.Fprintln(os.Stderr, err)
	if errors.Is(err, client.ErrDisabled) {
		return 3
	}
	return 1
}
func outputCall(method string, params, result any, format string) int {
	if err := callIndex(method, params, result); err != nil {
		return indexError(err)
	}
	return printFormatted(result, format)
}

func runIndex(args []string) int {
	if len(args) == 0 {
		return usageError("index add|rm|list|refresh|status")
	}
	verb, args := args[0], args[1:]
	switch verb {
	case "add":
		name := ""
		var filtered []string
		for index := 0; index < len(args); index++ {
			if args[index] == "--name" {
				if index+1 >= len(args) {
					return report(errors.New(i18n.T("Option --name needs a value.")))
				}
				index++
				name = args[index]
			} else {
				filtered = append(filtered, args[index])
			}
		}
		options, rest, err := parseIndexOptions(filtered)
		if err != nil {
			return report(err)
		}
		if len(rest) != 1 {
			return usageError("index add <path> [--name name]")
		}
		path, err := absoluteUserPath(rest[0])
		if err != nil {
			return report(err)
		}
		var result protocol.IndexInfo
		return outputCall(protocol.MIndexAdd, protocol.IndexAddParams{Path: path, Name: name}, &result, options.format)
	case "rm", "remove":
		options, rest, err := parseIndexOptions(args)
		if err != nil {
			return report(err)
		}
		if len(rest) != 1 {
			return usageError("index rm <id>")
		}
		if err := callIndex(protocol.MIndexRemove, protocol.IndexRef{ID: rest[0]}, &protocol.Empty{}); err != nil {
			return indexError(err)
		}
		_ = options
		return 0
	case "list":
		options, rest, err := parseIndexOptions(args)
		if err != nil {
			return report(err)
		}
		if len(rest) != 0 {
			return usageError("index list")
		}
		var result protocol.IndexesResult
		return outputCall(protocol.MIndexList, protocol.Empty{}, &result, options.format)
	case "refresh", "status":
		options, rest, err := parseIndexOptions(args)
		if err != nil {
			return report(err)
		}
		if len(rest) > 1 {
			return usageError("index " + verb + " [id]")
		}
		ref := indexRef(options)
		if len(rest) == 1 {
			ref.ID = rest[0]
		}
		var result protocol.IndexInfo
		method := protocol.MIndexRefresh
		if verb == "status" {
			method = protocol.MIndexStatus
		}
		return outputCall(method, ref, &result, options.format)
	default:
		return usageError("index add|rm|list|refresh|status")
	}
}
func runSearch(args []string) int {
	options, rest, err := parseIndexOptions(args)
	if err != nil {
		return report(err)
	}
	query := strings.TrimSpace(strings.Join(rest, " "))
	if query == "" {
		return usageError("search [options] <query>")
	}
	var result protocol.SearchResult
	params := protocol.SearchParams{IndexRef: indexRef(options), Query: query, Limit: options.limit}
	return outputCall(protocol.MIndexSearch, params, &result, options.format)
}
func runFind(args []string) int {
	options, rest, err := parseIndexOptions(args)
	if err != nil {
		return report(err)
	}
	if len(rest) != 1 {
		return usageError("find [options] <name>")
	}
	var result protocol.SymbolsResult
	return outputCall(protocol.MIndexFind, protocol.NameParams{IndexRef: indexRef(options), Name: rest[0], Limit: options.limit}, &result, options.format)
}
func runDef(args []string) int {
	options, rest, err := parseIndexOptions(args)
	if err != nil {
		return report(err)
	}
	if len(rest) != 1 {
		return usageError("def [options] <name>")
	}
	var result protocol.DefResult
	return outputCall(protocol.MIndexDef, protocol.NameParams{IndexRef: indexRef(options), Name: rest[0]}, &result, options.format)
}
func runOutline(args []string) int {
	options, rest, err := parseIndexOptions(args)
	if err != nil {
		return report(err)
	}
	if len(rest) != 1 {
		return usageError("outline [options] <path>")
	}
	path, err := absoluteUserPath(rest[0])
	if err != nil {
		return report(err)
	}
	var result protocol.SymbolsResult
	return outputCall(protocol.MIndexOutline, protocol.PathParams{IndexRef: indexRef(options), Path: path}, &result, options.format)
}
func runReferences(args []string) int {
	options, rest, err := parseIndexOptions(args)
	if err != nil {
		return report(err)
	}
	if len(rest) != 1 {
		return usageError("references [options] <name>")
	}
	var result protocol.ReferencesResult
	return outputCall(protocol.MIndexReferences, protocol.NameParams{IndexRef: indexRef(options), Name: rest[0], Limit: options.limit}, &result, options.format)
}
func runPeek(args []string) int {
	options, rest, err := parseIndexOptions(args)
	if err != nil {
		return report(err)
	}
	if len(rest) != 1 {
		return usageError("peek [options] <file:start[-end]>")
	}
	path, start, end, err := parsePeek(rest[0])
	if err != nil {
		return report(err)
	}
	path, err = absoluteUserPath(path)
	if err != nil {
		return report(err)
	}
	var result protocol.PeekResult
	return outputCall(protocol.MIndexPeek, protocol.PeekParams{IndexRef: indexRef(options), Path: path, Start: start, End: end}, &result, options.format)
}
func parsePeek(value string) (string, int, int, error) {
	colon := strings.LastIndexByte(value, ':')
	if colon < 1 {
		return "", 0, 0, errors.New(i18n.T("Peek needs file:start or file:start-end."))
	}
	pathPart, rangePart := value[:colon], value[colon+1:]
	startText, endText, hasEnd := strings.Cut(rangePart, "-")
	start, err := strconv.Atoi(startText)
	if err != nil || start < 1 {
		return "", 0, 0, errors.New(i18n.T("Peek line numbers must be positive."))
	}
	end := start + 199
	if hasEnd {
		end, err = strconv.Atoi(endText)
		if err != nil || end < start {
			return "", 0, 0, errors.New(i18n.T("Peek end line must not come before its start."))
		}
	}
	return pathPart, start, end, nil
}
func runOverview(args []string) int {
	options, rest, err := parseIndexOptions(args)
	if err != nil {
		return report(err)
	}
	if len(rest) != 0 {
		return usageError("overview [options]")
	}
	var result protocol.Overview
	return outputCall(protocol.MIndexOverview, indexRef(options), &result, options.format)
}
func runImpact(args []string) int {
	options, rest, err := parseIndexOptions(args)
	if err != nil {
		return report(err)
	}
	if len(rest) != 1 {
		return usageError("impact [options] <path>")
	}
	path, err := absoluteUserPath(rest[0])
	if err != nil {
		return report(err)
	}
	var result protocol.Impact
	return outputCall(protocol.MIndexImpact, protocol.PathParams{IndexRef: indexRef(options), Path: path, All: options.all}, &result, options.format)
}
func runWip(args []string) int {
	options, rest, err := parseIndexOptions(args)
	if err != nil {
		return report(err)
	}
	if len(rest) != 0 {
		return usageError("wip [options]")
	}
	var result protocol.WipResult
	return outputCall(protocol.MIndexWip, protocol.WipParams{IndexRef: indexRef(options)}, &result, options.format)
}
func runChanged(args []string) int {
	options, rest, err := parseIndexOptions(args)
	if err != nil {
		return report(err)
	}
	if len(rest) != 0 {
		return usageError("changed [options]")
	}
	var result protocol.WipResult
	return outputCall(protocol.MIndexChanged, protocol.WipParams{IndexRef: indexRef(options), Base: options.base}, &result, options.format)
}
