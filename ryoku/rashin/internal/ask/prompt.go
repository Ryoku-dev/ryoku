package ask

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"

	"ryoku-rashin/internal/brief"
	"ryoku-rashin/internal/llm"
	"ryoku-rashin/internal/paths"
	"ryoku-rashin/internal/protocol"
)

const (
	historyBudgetBytes = 12 * 1024 * 4
	vaultBudgetBytes   = 28 * 1024
	habitsTTL          = 10 * time.Minute
)

const askWho = `You are the Needle, Rashin's resident system agent on a Ryoku Linux desktop. The edition and compositor are named in desktop.md and by ryoku wm status. Detect them and never assume.`

const askReads = `This is Ask. You may inspect this machine with the supplied read-only tools, but you never change it. If the request needs a machine change or broader agent work, explain what is known and end with exactly NEEDS_AGENT: <one-line reason>. Run what only reads with system_query and answer from what it prints: never hand the user a command you can run. Use propose only for commands that change something, which the user runs themselves. The app shows a proposed command with a button that runs it and brings you what it prints, so do not repeat it in your answer. When terminal context says buffer is true, always call propose and put every command in that plan.`

// askActs carries one %s: how a change is cleared, which is what tells asking from running freely.
const askActs = `This is Ask, and the user has allowed you to change this machine. Inspect first with the read-only tools. When the user asks for a change, make it yourself: run_command runs one shell command and write_file writes one whole text file. %s Make the smallest change that does what was asked and nothing beside it. Change Ryoku through its own doors: name the Ryoku Settings page, and use the owning ryoku command before a config file. Before you edit a config file, run ` + "`ryoku owner <path>`" + ` with system_query and follow what it says, and never edit a generated or shipped file. A command that needs root cannot run here: put it in propose for the user to run in a terminal, and do not repeat it in your answer, since the app shows it with a button to run it. Never propose a command you can run yourself: run it and answer from what it prints. After a change, check that it took and say in a sentence or two what you changed. When the user only asks a question, answer it and change nothing. If the request is coding work across a project, end with exactly NEEDS_AGENT: <one-line reason>. When terminal context says buffer is true, always call propose and put every command in that plan.`

const (
	askActsAsking = `Each command or write that changes something is shown to the user, who allows or refuses it. A refusal is final: do not try the same change another way.`
	askActsFreely = `They run at once without asking, so read before you write. Anything destructive is still shown to the user first, and a refusal is final.`
)

const askStyle = `Answer desktop how-to questions GUI first. Name the exact Ryoku Settings page, shell picker, or active bar style setting before any command. Cite matching vault wiki pages by page name when they help. Write direct, plain sentence-case prose with no preamble, no follow-up question, and no em dash. Prefer the current vault and live tools over guesses.`

// askIdentity is who Ask is and what it may do in mode, an approvals value.
func askIdentity(mode string) string {
	may := askReads
	switch mode {
	case protocol.ApprovalsAsk:
		may = fmt.Sprintf(askActs, askActsAsking)
	case protocol.ApprovalsAuto:
		may = fmt.Sprintf(askActs, askActsFreely)
	}
	return askWho + "\n\n" + may + "\n\n" + askStyle
}

// systemPrompt is Ask's system prompt for mode; memory is the memory section
// recall gave this turn, "" for none.
func (f *Factory) systemPrompt(settings protocol.Settings, terminal *protocol.TerminalContext, mode, memory string) string {
	var b strings.Builder
	b.WriteString(askIdentity(mode))
	b.WriteString("\n\n# Ryoku vault\n\n")
	b.WriteString(brief.Vault(vaultBudgetBytes))
	b.WriteString("\n\n# Rashin\n\n")
	b.WriteString(brief.Rashin())
	b.WriteString("\n\n# Teams\n\n")
	b.WriteString(brief.Teams(true))
	if settings.Ask.Habits {
		b.WriteString("\n\n# User habits\n\n")
		b.WriteString(f.habits.summary())
	}
	if memory != "" {
		b.WriteString("\n\n")
		b.WriteString(memory)
	}
	if terminal != nil {
		b.WriteString("\n\n# Terminal context\n\n")
		if terminal.Cwd != "" {
			fmt.Fprintf(&b, "- cwd: %s\n", terminal.Cwd)
		}
		if terminal.Shell != "" {
			fmt.Fprintf(&b, "- shell: %s\n", terminal.Shell)
		}
		if terminal.LastCommand != "" {
			fmt.Fprintf(&b, "- last command: %s\n", terminal.LastCommand)
		}
		if terminal.LastStatus != nil {
			fmt.Fprintf(&b, "- last exit status: %d\n", *terminal.LastStatus)
		}
		fmt.Fprintf(&b, "- buffer plan requested: %t\n", terminal.Buffer)
	}
	return b.String()
}

type habitCache struct {
	mu      sync.Mutex
	at      time.Time
	content string
}

func (c *habitCache) summary() string {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.content != "" && time.Since(c.at) < habitsTTL {
		return c.content
	}
	c.content = collectHabits()
	c.at = time.Now()
	return c.content
}

var secretHistory = regexp.MustCompile(`(?i)(password|passwd|secret|token|api[_-]?key|bearer[ =]|authorization|private[_-]?key)`)

func collectHabits() string {
	var b strings.Builder
	shell := filepath.Base(os.Getenv("SHELL"))
	if shell == "." || shell == "" {
		shell = "unknown"
	}
	editor := os.Getenv("VISUAL")
	if editor == "" {
		editor = os.Getenv("EDITOR")
	}
	if editor == "" {
		editor = "not set"
	}
	fmt.Fprintf(&b, "- shell: %s\n- editor: %s\n", shell, editor)
	var tools []string
	for _, name := range []string{"eza", "zoxide", "fd", "rg", "bat", "btop"} {
		if _, err := exec.LookPath(name); err == nil {
			tools = append(tools, name)
		}
	}
	if len(tools) > 0 {
		fmt.Fprintf(&b, "- installed modern tools: %s\n", strings.Join(tools, ", "))
	}
	commands := topHistoryCommands(15)
	if len(commands) > 0 {
		fmt.Fprintf(&b, "- frequent commands: %s\n", strings.Join(commands, ", "))
	}
	return strings.TrimSpace(b.String())
}

func topHistoryCommands(limit int) []string {
	home := paths.Home()
	files := []struct {
		path string
		kind string
	}{
		{filepath.Join(home, ".local", "share", "fish", "fish_history"), "fish"},
		{filepath.Join(home, ".bash_history"), "line"},
		{filepath.Join(home, ".zsh_history"), "zsh"},
	}
	counts := make(map[string]int)
	for _, source := range files {
		for _, line := range historyTail(source.path, 256*1024) {
			line = strings.TrimSpace(line)
			switch source.kind {
			case "fish":
				if !strings.HasPrefix(line, "- cmd:") {
					continue
				}
				line = strings.TrimSpace(strings.TrimPrefix(line, "- cmd:"))
			case "zsh":
				if strings.HasPrefix(line, ": ") {
					if i := strings.Index(line, ";"); i >= 0 {
						line = line[i+1:]
					}
				}
			}
			if secretHistory.MatchString(line) {
				continue
			}
			fields := strings.Fields(line)
			if len(fields) == 0 {
				continue
			}
			name := filepath.Base(fields[0])
			if name == "sudo" || name == "doas" || name == "run0" {
				if len(fields) < 2 {
					continue
				}
				name = filepath.Base(fields[1])
			}
			counts[name]++
		}
	}
	type count struct {
		name string
		n    int
	}
	ordered := make([]count, 0, len(counts))
	for name, n := range counts {
		ordered = append(ordered, count{name: name, n: n})
	}
	sort.Slice(ordered, func(i, j int) bool {
		if ordered[i].n == ordered[j].n {
			return ordered[i].name < ordered[j].name
		}
		return ordered[i].n > ordered[j].n
	})
	if len(ordered) > limit {
		ordered = ordered[:limit]
	}
	out := make([]string, len(ordered))
	for i, item := range ordered {
		out[i] = fmt.Sprintf("%s (%d)", item.name, item.n)
	}
	return out
}

func historyTail(path string, limit int64) []string {
	file, err := os.Open(path)
	if err != nil {
		return nil
	}
	defer file.Close()
	if stat, err := file.Stat(); err == nil && stat.Size() > limit {
		_, _ = file.Seek(stat.Size()-limit, io.SeekStart)
	}
	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 4096), 256*1024)
	var lines []string
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}
	return lines
}

func historyMessages(events []protocol.SessionEvent, budget int) []llm.Message {
	var messages []llm.Message
	asked := make(map[int]protocol.UserData)
	assistantByID := make(map[string]int)
	for _, event := range events {
		switch event.Kind {
		case protocol.EvUser:
			var data protocol.UserData
			if jsonUnmarshal(event.Data, &data) != nil {
				continue
			}
			if data.Run != "" {
				// what it printed is in the answer that follows it
				messages = append(messages, llm.Message{Role: llm.RoleUser, Content: "I ran this command myself: " + data.Run})
			} else if strings.TrimSpace(data.Text) != "" || len(data.Images) > 0 {
				asked[len(messages)] = data
				messages = append(messages, llm.Message{Role: llm.RoleUser, Content: data.Text})
			}
		case protocol.EvText:
			var data protocol.TextData
			if jsonUnmarshal(event.Data, &data) != nil || data.Delta == "" {
				continue
			}
			if i, ok := assistantByID[data.Message]; ok {
				messages[i].Content += data.Delta
				continue
			}
			assistantByID[data.Message] = len(messages)
			messages = append(messages, llm.Message{Role: llm.RoleAssistant, Content: data.Delta})
		}
	}
	used := 0
	start := len(messages)
	for i := len(messages) - 1; i >= 0; i-- {
		size := len(messages[i].Content)
		if used+size > budget && start < len(messages) {
			break
		}
		used += size
		start = i
	}
	// newest first, so what was just talked about is what still fits
	imageRoom, textRoom := historyImageBytes, historyTextBytes
	for i := len(messages) - 1; i >= start; i-- {
		if data, ok := asked[i]; ok && len(data.Images) > 0 {
			messages[i] = earlierMessage(data, &imageRoom, &textRoom)
		}
	}
	return messages[start:]
}

func jsonUnmarshal(data []byte, value any) error {
	if len(data) == 0 {
		return io.ErrUnexpectedEOF
	}
	return json.Unmarshal(data, value)
}
