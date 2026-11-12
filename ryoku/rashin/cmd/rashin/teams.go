package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"

	i18n "ryoku-i18n"
	"ryoku-rashin/internal/protocol"
)

func init() {
	register(&Command{Name: "teams", Group: "teams", Usage: "[list|templates] [--format human|json|toon] | propose <file.json>", Summary: i18n.T("List teams and their templates, or propose a team for the user to approve."), Run: runTeams})
}

func runTeams(args []string) int {
	verb := "list"
	if len(args) > 0 && !strings.HasPrefix(args[0], "--") {
		verb, args = args[0], args[1:]
	}
	switch verb {
	case "list", "templates":
		format := defaultOutputFormat()
		switch {
		case len(args) == 2 && args[0] == "--format":
			format = args[1]
		case len(args) != 0:
			return usageError("teams [list|templates] [--format human|json|toon]")
		}
		if verb == "templates" {
			var result protocol.TeamTemplatesResult
			return teamsOutput(protocol.MTeamsTemplates, &result, format, func(w io.Writer) { printTemplates(w, result) })
		}
		var result protocol.TeamsResult
		return teamsOutput(protocol.MTeamsList, &result, format, func(w io.Writer) { printTeams(w, result) })
	case "propose":
		if len(args) != 1 {
			return usageError("teams propose <file.json>")
		}
		return proposeTeam(args[0])
	case "home-done":
		if len(args) != 2 {
			return usageError("teams home-done <token> <status>")
		}
		return finishTeamsHome(args[0], args[1])
	default:
		return usageError("teams [list|templates] [--format human|json|toon] | propose <file.json>")
	}
}

// finishTeamsHome tells the daemon a sign-in or setup terminal ended, with the
// token Rashin handed it and how its command exited, and prints the answer.
// The terminal Rashin opens runs it; nobody types it.
func finishTeamsHome(token, status string) int {
	code, err := strconv.Atoi(status)
	if err != nil {
		return usageError("teams home-done <token> <status>")
	}
	var result protocol.HarnessTeamsDoneResult
	if err := daemonCall(context.Background(), protocol.MHarnessesTeamsDone, protocol.HarnessTeamsDoneParams{Token: token, Status: code}, &result); err != nil {
		return report(err)
	}
	fmt.Fprintln(os.Stdout, result.Text)
	if !result.SignedIn {
		return 1
	}
	return 0
}

// teamsOutput calls a teams method that only reads and prints its answer,
// human through human
func teamsOutput(method string, result any, format string, human func(io.Writer)) int {
	if err := daemonCall(context.Background(), method, protocol.Empty{}, result); err != nil {
		return report(err)
	}
	if format == "human" {
		human(os.Stdout)
		return 0
	}
	return printFormatted(result, format)
}

func printTeams(w io.Writer, result protocol.TeamsResult) {
	for _, team := range result.Teams {
		state := i18n.T("working")
		if team.Paused {
			state = i18n.T("paused")
		}
		fmt.Fprintf(w, "%s\t%s\t%s\n", team.ID, team.Name, state)
		if team.Purpose != "" {
			fmt.Fprintf(w, "  %s\n", team.Purpose)
		}
	}
}

func printTemplates(w io.Writer, result protocol.TeamTemplatesResult) {
	for index, template := range result.Templates {
		if index > 0 {
			fmt.Fprintln(w)
		}
		fmt.Fprintf(w, "%s\t%s\t%s\n", template.ID, template.Name, template.Level)
		fmt.Fprintf(w, "  %s\n", template.Description)
		for _, member := range template.Members {
			role := i18n.T("member")
			if member.Lead {
				role = i18n.T("lead")
			}
			fmt.Fprintf(w, "  %s: %s, %s\n", role, member.Name, member.Title)
		}
		for _, routine := range template.Routines {
			fmt.Fprint(w, i18n.Tf("  routine: %s, %s\n", routine.Title, routine.Schedule))
		}
		for _, need := range template.Credentials {
			fmt.Fprint(w, i18n.Tf("  credential: %s for %s\n", need.Name, strings.Join(need.Hosts, ", ")))
		}
	}
}

// proposeTeam reads a team proposal from path, or standard input for "-",
// and hands it to the user's Inbox
func proposeTeam(path string) int {
	var data []byte
	var err error
	if path == "-" {
		data, err = io.ReadAll(os.Stdin)
	} else {
		data, err = os.ReadFile(path)
	}
	if err != nil {
		return report(err)
	}
	var proposal protocol.TeamProposeParams
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&proposal); err != nil {
		return report(fmt.Errorf("%s", i18n.Tf("%s is not a team proposal: %v", path, err)))
	}
	var approval protocol.Approval
	if err := daemonCall(context.Background(), protocol.MTeamsPropose, proposal, &approval); err != nil {
		return report(err)
	}
	fmt.Fprintln(os.Stdout, i18n.Tf("Proposed %s. Approve it or say no in the Rashin Inbox.", approval.Proposal.Team.Name))
	return 0
}
