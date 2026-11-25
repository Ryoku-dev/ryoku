package ask

import (
	"bufio"
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

	"ryoku-rashin/internal/paths"
)

const habitsTTL = 10 * time.Minute

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
