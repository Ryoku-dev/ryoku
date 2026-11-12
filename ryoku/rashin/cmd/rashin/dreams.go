package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"

	i18n "ryoku-i18n"
	"ryoku-rashin/internal/protocol"
)

func init() {
	register(&Command{Name: "dreams", Group: "teams", Usage: "[list] [--format human|json|toon] | start <statement> [--name n] [--folder dir] [--harness id] [--model id] [--tokens n]", Summary: i18n.T("List dreams, or start one."), Run: runDreams})
}

func runDreams(args []string) int {
	verb := "list"
	if len(args) > 0 && !strings.HasPrefix(args[0], "--") {
		verb, args = args[0], args[1:]
	}
	switch verb {
	case "list":
		format := defaultOutputFormat()
		switch {
		case len(args) == 2 && args[0] == "--format":
			format = args[1]
		case len(args) != 0:
			return usageError("dreams [list] [--format human|json|toon]")
		}
		return listDreams(format)
	case "start":
		params, err := parseDreamStart(args)
		if err != nil {
			return usageError("dreams start <statement> [--name n] [--folder dir] [--harness id] [--model id] [--tokens n]")
		}
		return startDream(params)
	default:
		return usageError("dreams [list] [--format human|json|toon] | start <statement> [...]")
	}
}

// listDreams prints the dreams from the snapshot, as teams.list leaves them
// out
func listDreams(format string) int {
	var snapshot protocol.Snapshot
	if err := daemonCall(context.Background(), protocol.MSubscribe, protocol.Empty{}, &snapshot); err != nil {
		return report(err)
	}
	result := protocol.TeamsResult{Teams: []protocol.Team{}}
	for _, team := range snapshot.Teams {
		if team.Dream != nil {
			result.Teams = append(result.Teams, team)
		}
	}
	if format == "human" {
		printDreams(os.Stdout, result)
		return 0
	}
	return printFormatted(result, format)
}

func printDreams(w io.Writer, result protocol.TeamsResult) {
	for _, team := range result.Teams {
		state := team.Dream.Phase
		if team.Paused {
			state = i18n.T("paused")
		}
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", team.ID, team.Name, state, i18n.Tf("%d members", len(team.Members)))
		if summary := team.Dream.Summary; summary != "" {
			fmt.Fprintf(w, "  %s\n", summary)
		}
	}
}

// parseDreamStart reads the statement and the flags of dreams start
func parseDreamStart(args []string) (protocol.DreamCreateParams, error) {
	var p protocol.DreamCreateParams
	var words []string
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if !strings.HasPrefix(arg, "--") {
			words = append(words, arg)
			continue
		}
		if i+1 >= len(args) {
			return p, errors.New(arg)
		}
		value := args[i+1]
		i++
		switch arg {
		case "--name":
			p.Name = value
		case "--folder":
			path, err := absoluteUserPath(value)
			if err != nil {
				return p, err
			}
			p.Folder = path
		case "--harness":
			p.Harness = value
		case "--model":
			p.Model = value
		case "--tokens":
			tokens, err := strconv.ParseInt(value, 10, 64)
			if err != nil || tokens < 0 {
				return p, errors.New(arg)
			}
			p.MonthTokens = tokens
		default:
			return p, errors.New(arg)
		}
	}
	p.Statement = strings.Join(words, " ")
	if strings.TrimSpace(p.Statement) == "" {
		return p, errors.New("statement")
	}
	return p, nil
}

// startDream starts a dream; only the app may, so a refusal opens the app's
// New dream dialog for the user to start it there
func startDream(p protocol.DreamCreateParams) int {
	var team protocol.Team
	err := daemonCall(context.Background(), protocol.MTeamsDreamCreate, p, &team)
	var refused *protocol.Error
	if errors.As(err, &refused) && refused.Code == protocol.CodeForbidden {
		fmt.Fprintln(os.Stdout, i18n.T("Finish starting the dream in Rashin."))
		return runOpen([]string{"--route", protocol.TabDreams + "/new"})
	}
	if err != nil {
		return report(err)
	}
	fmt.Fprintln(os.Stdout, i18n.Tf("Started the dream %s (%s), working in %s.", team.Name, team.ID, team.Folder))
	return 0
}
