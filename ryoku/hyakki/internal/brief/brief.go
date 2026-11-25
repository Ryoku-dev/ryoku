// Package brief describes Hyakki, the authorized session and optional machine
// context to the model. A Ryoku vault enriches that context when present; it
// is not a requirement for standalone use.
package brief

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"ryoku-rashin/internal/paths"
	"ryoku-rashin/internal/protocol"
)

const rashin = `Hyakki is the local-first assistant and coding harness this conversation runs in. Its daemon, Oni, starts on demand and stops when idle. Hyakki Settings and the hyakki on/off commands control activation.
- Ask answers questions in the Ask tab or from ` + "`hyakki <question>`" + ` in a terminal. Desktop shortcuts depend on the installation, not on having an account.
- Agents work in authorized folders under the user's approvals. Projects holds Git worktrees, commit graphs, review and tasks. The Yokai is the teams area, and Index shows the indexed folders.
- Providers can be local models, user-supplied services or optional Ryoku Ember hosted inference. Follow the selected routing and disclosure settings; never assume that local work requires Ryoku Account or hosted inference.
- ` + "`hyakki api`" + ` exposes the local router to compatible tools. Its local API credential is not a Ryoku Account session or an Ember key.
- Code search is built in: hyakki search, find, def, outline, references, peek, impact and overview, run inside an indexed folder. ` + "`hyakki index list`" + ` shows the indexes.
- Providers, local models and keys are in Settings > Providers and ` + "`hyakki providers`" + `. ` + "`hyakki status`" + ` reports activation without starting Oni. The settings file is $XDG_CONFIG_HOME/hyakki/hyakki.json.`

const rashinGuide = "The optional Ryoku vault documents the installed desktop; its wiki is not required for Hyakki."

// vaultGuide names the vault at %s and how a model with its own tools uses it.
const vaultGuide = "%s is this machine's map, kept current by Ryoku. Read AGENTS.md there for its rules, and desktop.md before you search the filesystem for where a setting lives. system.md, packages.md, ownership.md and logs.md cover the hardware, the packages, who owns a config file and where each log is, and wiki/ holds the guides. Change desktop state through its owning ryoku command, and run `ryoku owner <path>` before you edit a config file."

// Rashin is the retained internal entry point for Hyakki's product brief;
// vault adds only the optional guide pointer for a session that can read it.
func Rashin(vault bool) string {
	if vault {
		return rashin + "\n" + rashinGuide
	}
	return rashin
}

// VaultGuide is how a model with file tools uses the vault at dir.
func VaultGuide(dir string) string { return fmt.Sprintf(vaultGuide, dir) }

const teams = `A team is a standing group of agents Hyakki keeps for one purpose on this computer: members on models of their own, a lead that hands pieces of each task to the others, routines that give it work on a schedule, a vault that keeps its credentials so no agent ever sees a value, and approvals that wait for the user in the Hyakki Inbox. Teams do not talk to each other: each works only in its own folder.
When the user asks for a team, or wants work done again and again on its own (a weekly digest, watching a repository, looking after this computer), propose one. Start from the closest template: light ones have one member, medium ones two and a routine, advanced ones several members on different models, routines bound to members and credentials. Name every credential the team will need, with the hosts it goes to and why, and Hyakki asks the user for each value in the Hyakki Inbox once the team exists. Nothing is made until the user creates the team from the Hyakki Inbox, and its routines start off until the user turns them on: never tell the user a proposed team or its routines start by themselves.`

// Teams describes how a model proposes an existing team, using its tools when
// available and otherwise using the Hyakki command line.
func Teams(tools bool) string {
	if tools {
		return teams + "\nYour tools: " + ToolTeamsTemplates + " lists the templates, " + ToolTeamsList + " the teams that exist, and " + ToolProposeTeam + " proposes a team."
	}
	return teams + "\n`hyakki teams templates` lists the templates and `hyakki teams list` the teams that exist. `hyakki teams propose <file.json>` proposes a team from a JSON file with team (a template id, name, purpose, members, routines), credentials (name, hosts, why) and reason."
}

// VaultDir is the Ryoku vault of this machine, or "" when it has none yet.
func VaultDir() string {
	for _, dir := range []string{paths.VaultDir(), paths.LegacyVaultDir()} {
		if info, err := os.Stat(filepath.Join(dir, "AGENTS.md")); err == nil && info.Mode().IsRegular() {
			return dir
		}
	}
	return ""
}

// Vault returns the vault's own instructions and its desktop map, the two
// files a model needs before it goes looking, cut to budget bytes.
func Vault(budget int) string {
	root := VaultDir()
	var b strings.Builder
	remaining := budget
	for _, name := range []string{"AGENTS.md", "desktop.md"} {
		if root == "" || remaining <= 0 {
			break
		}
		header := "## " + name + "\n"
		available := remaining - len(header) - 2
		if available <= 0 {
			break
		}
		body, err := readAtMost(filepath.Join(root, name), available)
		if err != nil {
			continue
		}
		body = strings.TrimSpace(body)
		fmt.Fprintf(&b, "%s%s\n\n", header, body)
		remaining -= len(header) + len(body) + 2
	}
	if b.Len() == 0 {
		return "No Ryoku vault is configured on this machine. Hyakki works without it; use authorized read-only tools for live system facts."
	}
	return strings.TrimSpace(b.String())
}

// Session is what an agent is told about the session it runs in.
type Session struct {
	Info protocol.SessionInfo
	// Project is the name of the project the session belongs to, if any.
	Project string
	// Index is the index that covers the session's folder, nil when the
	// folder cannot be indexed. One still being built has no file count yet.
	Index *protocol.IndexInfo
	// Semantic says the index also searches by meaning.
	Semantic bool
	// Screen says the session is offered a screen of its own.
	Screen bool
	// Team is what an agent working a team task is told about its team, its
	// role and how the team works; empty for any other session.
	Team string
	// Writable are the folders a team agent may change besides its own.
	Writable []string
}

// Agent is the session part of an agent's system prompt (engine Options
// Brief): the folder it works in and what it can reach there, the code index
// over it, its screen and, for a team task, its team. What every session is
// told (Ryoku, the vault, Rashin, teams) and what the context providers add
// (skills, decisions, rules, memory) are the prompt builder's, not this.
func Agent(session Session) string {
	info := session.Info
	var b strings.Builder
	b.WriteString("# Session\n")
	fmt.Fprintf(&b, "- Folder: %s\n", info.Cwd)
	if session.Project != "" {
		fmt.Fprintf(&b, "- Project: %s\n", session.Project)
	}
	if info.Branch != "" {
		fmt.Fprintf(&b, "- Branch: %s\n", info.Branch)
	}
	if info.Worktree != "" && info.Worktree != info.Cwd {
		fmt.Fprintf(&b, "- Worktree: %s\n", info.Worktree)
	}
	if info.Team != "" {
		b.WriteString("- Reach: you run in a sandbox. You can change only this folder")
		if len(session.Writable) > 0 {
			fmt.Fprintf(&b, " and %s", strings.Join(session.Writable, ", "))
		}
		b.WriteString("; the rest of this computer is read-only, while Hyakki's state, the Ryoku vault, the credential vault, other teams' folders and the paths Settings > Teams hides look empty, and code search reaches only the indexes inside the folders you can change.\n")
	}
	if index := session.Index; index != nil {
		fmt.Fprintf(&b, "- Code index: Hyakki keeps an index of %s", index.Root)
		if index.Files > 0 {
			fmt.Fprintf(&b, " (%d files", index.Files)
			if len(index.Languages) > 0 {
				fmt.Fprintf(&b, ": %s", languageSummary(index.Languages))
			}
			b.WriteString(")")
		}
		if session.Semantic {
			b.WriteString(" with search by meaning built in")
		}
		b.WriteString(", and it follows your edits. It reads every source language in the folder.\n")
	}
	if session.Screen {
		b.WriteString("- Screen: when the work needs a graphical app, such as a web page to check or a program with no command line, the screen tools give you a hidden desktop of your own to open apps on, look at, click and type into. The first use asks the user; they can watch it and take it over, and screen_ask_user hands it to them for sign-ins.\n")
	}
	if team := strings.TrimSpace(session.Team); team != "" {
		b.WriteString("\n")
		b.WriteString(team)
		b.WriteString("\n")
	}
	return strings.TrimRight(b.String(), "\n")
}

// languageNames are how the brief names an index's languages
var languageNames = map[string]string{
	"bash": "shell", "c": "C", "cpp": "C++", "css": "CSS", "fish": "fish", "go": "Go", "gomod": "go.mod",
	"ini": "INI", "javascript": "JavaScript", "json": "JSON", "kdl": "KDL", "lua": "Lua", "markdown": "Markdown",
	"python": "Python", "qml": "QML", "rust": "Rust", "toml": "TOML", "typescript": "TypeScript", "yaml": "YAML",
}

// languageSummary lists the most common languages with their file counts
func languageSummary(counts []protocol.Count) string {
	const shown = 6
	var parts []string
	for i, count := range counts {
		if i == shown {
			parts = append(parts, fmt.Sprintf("%d more languages", len(counts)-shown))
			break
		}
		name := languageNames[count.Name]
		if name == "" {
			name = count.Name
		}
		parts = append(parts, fmt.Sprintf("%d %s", count.Count, name))
	}
	return strings.Join(parts, ", ")
}

func readAtMost(path string, limit int) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer file.Close()
	body, err := io.ReadAll(io.LimitReader(file, int64(limit)+1))
	if err != nil {
		return "", err
	}
	if len(body) > limit {
		const marker = "\n[trimmed]"
		if limit <= len(marker) {
			return string(body[:limit]), nil
		}
		return strings.TrimRight(strings.ToValidUTF8(string(body[:limit-len(marker)]), ""), " \t\n") + marker, nil
	}
	return string(body), nil
}
