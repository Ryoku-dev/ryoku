// Package brief is what every Rashin request is told about where it runs:
// Ryoku, the Ryoku vault and Rashin itself. Ask reads it into its system
// prompt and an agent session hears it ahead of its first prompt, so neither
// starts as a stranger on the machine.
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

const rashin = `Rashin is Ryoku's optional assistant and the system this conversation runs in. One daemon, rashind, does the work: it starts on demand and stops when idle. Ryoku Hub > Rashin turns it on or off.
- Ask answers questions about this machine on free models through Rashin's own router. It opens from the Ask bar (Alt+Space by default), from ` + "`rashin <question>`" + ` in a terminal, and in the Ask tab of the Rashin app (Super+Alt+Space by default).
- The Agents tab of the app runs coding harnesses such as Claude Code, Codex, omp and OpenCode as sessions Rashin owns. Projects holds their Git worktrees, commit graph, review and tasks. Index shows what is indexed.
- Code search is built in: rashin search, find, def, outline, references, peek, impact and overview, run inside an indexed folder. ` + "`rashin index list`" + ` shows the indexes, and a project added to Rashin is indexed by itself.
- Model providers and their keys are in Rashin > Settings > Providers and ` + "`rashin providers`" + `. ` + "`rashin api`" + ` serves the router to other programs, ` + "`rashin status`" + ` says whether Rashin is on, and the settings file is ~/.config/ryoku/rashin.json.
The vault wiki page rashin is the full guide.`

// Rashin describes Rashin to a model that runs inside it.
func Rashin() string { return rashin }

// VaultDir is the Ryoku vault of this machine, or "" when it has none yet.
func VaultDir() string {
	for _, dir := range []string{paths.VaultDir(), paths.LegacyVaultDir()} {
		if info, err := os.Stat(dir); err == nil && info.IsDir() {
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
		body, err := readAtMost(filepath.Join(root, name), remaining)
		if err != nil {
			continue
		}
		fmt.Fprintf(&b, "## %s\n%s\n\n", name, strings.TrimSpace(body))
		remaining -= len(body)
	}
	if b.Len() == 0 {
		return "The Ryoku vault is not available yet. Use read-only tools for live facts."
	}
	return strings.TrimSpace(b.String())
}

// Session is what an agent is told about the session it was started for.
type Session struct {
	Info protocol.SessionInfo
	// Project is the name of the project the session belongs to, if any.
	Project string
	// Index is the index that covers the session's folder, nil when the
	// folder cannot be indexed. One still being built has no file count yet.
	Index *protocol.IndexInfo
	// Semantic says the index also searches by meaning.
	Semantic bool
}

// Agent is the text an external agent hears once, ahead of its first prompt.
// It names the vault instead of quoting it: the agent has its own tools to
// read files, and a long preamble would crowd the user's request.
func Agent(session Session) string {
	info := session.Info
	var b strings.Builder
	b.WriteString("Context from Rashin. The user did not write this part; their request follows it.\n\n")
	b.WriteString("You are working inside Rashin, the assistant built into Ryoku, a Linux desktop built on Arch. Rashin started you as a coding agent, shows your work in its app and puts your permission requests to the user.\n\n")

	b.WriteString("Session\n")
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

	if vault := VaultDir(); vault != "" {
		b.WriteString("\nThe Ryoku vault\n")
		fmt.Fprintf(&b, "%s is this machine's map, kept current by Ryoku. Read AGENTS.md there for its rules, and desktop.md before you search the filesystem for where a setting lives. system.md, packages.md, ownership.md and logs.md cover the hardware, the packages, who owns a config file and where each log is, and wiki/ holds the guides. Change desktop state through its owning ryoku command, and run `ryoku owner <path>` before you edit a config file.\n", vault)
	}

	b.WriteString("\nRashin\n")
	b.WriteString(rashin)
	b.WriteString("\n")

	if index := session.Index; index != nil {
		b.WriteString("\nCode search\n")
		fmt.Fprintf(&b, "Rashin keeps an index of %s", index.Root)
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
		b.WriteString(", and it follows your edits. It reads every source language in the folder, so when another code index or search skill you carry finds little here, that one is likely skipping languages: use this one. Ask it before you grep or open files to find something. Run these from the session folder:\n")
		b.WriteString("- rashin search \"<what you are looking for>\"  ranked passages with file and line\n")
		b.WriteString("- rashin find <name>  a symbol by name\n")
		b.WriteString("- rashin def <name>  a symbol's source\n")
		b.WriteString("- rashin outline <file>  what a file defines\n")
		b.WriteString("- rashin references <name>  where a symbol is used\n")
		b.WriteString("- rashin impact <file>  what depends on a file\n")
		b.WriteString("- rashin peek <file:start-end>  those lines\n")
		b.WriteString("Add --format json for structured output.\n")
	}

	b.WriteString("\nEnd of context from Rashin.")
	return b.String()
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
		return string(body[:limit]) + "\n[trimmed]", nil
	}
	return string(body), nil
}
