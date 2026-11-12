package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"

	i18n "ryoku-i18n"
	"ryoku-rashin/internal/protocol"
)

const memoryUsage = "memory status | list [--scope s] [--status s] [--query q] [--limit n] | search <words> [--scope s] [--limit n] | add <text> [--scope s] [--sensitive] | forget <id>... | card [--rebuild] | sessions <words> [--last 6h|7d|2w|1m] [--limit n] | learn <session>, each with [--format human|json|toon]"

func init() {
	register(&Command{Name: "memory", Group: "memory", Usage: "status|list|search|add|forget|card|sessions|learn [--format human|json|toon]", Summary: i18n.T("Read, search and change what Rashin remembers."), Run: runMemory})
}

// memoryArgs are a memory command's words and flags
type memoryArgs struct {
	words     []string
	flags     map[string]string
	sensitive bool
	rebuild   bool
}

// parseMemoryArgs splits words from flags; every flag but --sensitive and
// --rebuild takes a value
func parseMemoryArgs(args []string) (memoryArgs, bool) {
	parsed := memoryArgs{flags: map[string]string{}}
	for i := 0; i < len(args); i++ {
		switch arg := args[i]; arg {
		case "--sensitive":
			parsed.sensitive = true
		case "--rebuild":
			parsed.rebuild = true
		case "--scope", "--status", "--query", "--limit", "--last", "--format":
			if i+1 == len(args) {
				return parsed, false
			}
			parsed.flags[strings.TrimPrefix(arg, "--")] = args[i+1]
			i++
		default:
			if strings.HasPrefix(arg, "--") {
				return parsed, false
			}
			parsed.words = append(parsed.words, arg)
		}
	}
	return parsed, true
}

func runMemory(args []string) int {
	if len(args) == 0 {
		return usageError(memoryUsage)
	}
	verb := args[0]
	parsed, ok := parseMemoryArgs(args[1:])
	if !ok {
		return usageError(memoryUsage)
	}
	limit := 0
	if value, set := parsed.flags["limit"]; set {
		n, err := strconv.Atoi(value)
		if err != nil || n < 1 {
			return usageError(memoryUsage)
		}
		limit = n
	}
	text := strings.Join(parsed.words, " ")
	format := parsed.flags["format"]
	if format == "" {
		format = defaultOutputFormat()
	}
	switch verb {
	case "status":
		if len(parsed.words) != 0 {
			return usageError(memoryUsage)
		}
		var result protocol.MemoryStatus
		return memoryOutput(protocol.MMemoryStatus, protocol.Empty{}, &result, format, func(w io.Writer) { printMemoryStatus(w, result) })
	case "list":
		if len(parsed.words) != 0 {
			return usageError(memoryUsage)
		}
		var result protocol.MemoryListResult
		params := protocol.MemoryListParams{Scope: parsed.flags["scope"], Status: parsed.flags["status"], Query: parsed.flags["query"], Limit: limit}
		return memoryOutput(protocol.MMemoryList, params, &result, format, func(w io.Writer) { printMemories(w, result.Memories) })
	case "search":
		if text == "" {
			return usageError(memoryUsage)
		}
		var result protocol.MemorySearchResult
		params := protocol.MemorySearchParams{Query: text, Scope: parsed.flags["scope"], Limit: limit}
		return memoryOutput(protocol.MMemorySearch, params, &result, format, func(w io.Writer) { printMemories(w, result.Memories) })
	case "add":
		if text == "" {
			return usageError(memoryUsage)
		}
		scope := parsed.flags["scope"]
		if scope == "" {
			scope = protocol.MemoryYou
		}
		var result protocol.MemoryChange
		params := protocol.MemoryAddParams{Scope: scope, Text: text, Sensitive: parsed.sensitive}
		return memoryOutput(protocol.MMemoryAdd, params, &result, format, func(w io.Writer) { printMemoryChange(w, result) })
	case "forget":
		if len(parsed.words) == 0 {
			return usageError(memoryUsage)
		}
		var result protocol.MemoryChange
		return memoryOutput(protocol.MMemoryForget, protocol.MemoryForgetParams{IDs: parsed.words}, &result, format, func(w io.Writer) { printMemoryChange(w, result) })
	case "card":
		if len(parsed.words) != 0 {
			return usageError(memoryUsage)
		}
		var result protocol.MemoryCard
		return memoryOutput(protocol.MMemoryCard, protocol.MemoryCardParams{Rebuild: parsed.rebuild}, &result, format, func(w io.Writer) { fmt.Fprintln(w, result.Text) })
	case "sessions":
		if text == "" {
			return usageError(memoryUsage)
		}
		var result protocol.MemorySessionsResult
		params := protocol.MemorySessionsParams{Query: text, Last: parsed.flags["last"], Limit: limit}
		return memoryOutput(protocol.MMemorySessions, params, &result, format, func(w io.Writer) { printMemorySessions(w, result) })
	case "learn":
		if len(parsed.words) != 1 {
			return usageError(memoryUsage)
		}
		var result protocol.MemoryChange
		return memoryOutput(protocol.MMemoryLearn, protocol.MemoryLearnParams{Session: parsed.words[0]}, &result, format, func(w io.Writer) { printMemoryChange(w, result) })
	}
	return usageError(memoryUsage)
}

// memoryOutput calls a memory method and prints its answer, human through
// human
func memoryOutput(method string, params, result any, format string, human func(io.Writer)) int {
	if err := daemonCall(context.Background(), method, params, result); err != nil {
		return report(err)
	}
	if format == "human" {
		human(os.Stdout)
		return 0
	}
	return printFormatted(result, format)
}

func printMemoryStatus(w io.Writer, status protocol.MemoryStatus) {
	fmt.Fprint(w, i18n.Tf("Engine: %s, %s\n", status.Engine, status.EngineState))
	if status.EngineDetail != "" {
		fmt.Fprintf(w, "  %s\n", status.EngineDetail)
	}
	counts := status.Counts
	fmt.Fprint(w, i18n.Tf("Memories: %d in use (%d about you, %d in projects, %d in teams)\n", counts.All, counts.You, counts.Projects, counts.Teams))
	fmt.Fprint(w, i18n.Tf("Waiting: %d suggested, %d replaced, %d blocked\n", counts.Suggested, counts.Replaced, counts.Blocked))
	fmt.Fprint(w, i18n.Tf("Last 30 days: %d learned, %d forgotten\n", counts.LearnedMonth, counts.ForgotMonth))
	learner := status.Learner
	fmt.Fprint(w, i18n.Tf("Learning: %s", learner.State))
	if learner.Model != "" {
		fmt.Fprint(w, i18n.Tf(" with %s", learner.Model))
	}
	if learner.Waiting > 0 {
		fmt.Fprint(w, i18n.Tf(", %d conversations waiting", learner.Waiting))
	}
	fmt.Fprintln(w)
	if learner.Detail != "" {
		fmt.Fprintf(w, "  %s\n", learner.Detail)
	}
	fmt.Fprint(w, i18n.Tf("Card: %d of %d characters\n", len([]rune(status.Card.Text)), protocol.MemoryCardMax))
	if status.Path != "" {
		fmt.Fprint(w, i18n.Tf("Files: %s\n", status.Path))
	}
}

func printMemories(w io.Writer, memories []protocol.Memory) {
	for _, m := range memories {
		fmt.Fprintf(w, "%s\t%s\t%s\n", m.ID, m.ScopeName, strings.Join(strings.Fields(m.Text), " "))
	}
}

func printMemoryChange(w io.Writer, change protocol.MemoryChange) {
	if len(change.Items) == 0 {
		fmt.Fprintln(w, i18n.T("Nothing changed."))
		return
	}
	for _, item := range change.Items {
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", item.Op, item.ID, item.ScopeName, item.Text)
	}
}

func printMemorySessions(w io.Writer, result protocol.MemorySessionsResult) {
	for index, hit := range result.Sessions {
		if index > 0 {
			fmt.Fprintln(w)
		}
		fmt.Fprintf(w, "%s\t%s\t%s\n", hit.Session, hit.Updated.Local().Format("2006-01-02 15:04"), hit.Title)
		if hit.Summary != "" {
			fmt.Fprintf(w, "  %s\n", hit.Summary)
		}
		for _, line := range hit.Lines {
			fmt.Fprintf(w, "  > %s\n", line)
		}
	}
}
