package engine

// Derived from oh-my-pi (MIT), packages/coding-agent/src/prompts/system/system-prompt.md@c7fff67
// Derived from oh-my-pi (MIT), packages/coding-agent/src/system-prompt.ts@c7fff67
// Derived from oh-my-pi (MIT), .omp/skills/system-prompts/small-models.md@c7fff67

import (
	"bytes"
	"embed"
	"fmt"
	"io/fs"
	"path/filepath"
	"strings"
	"sync"
	"text/template"

	"ryoku-rashin/internal/brief"
	"ryoku-rashin/internal/engine/tool"
	"ryoku-rashin/internal/paths"
	"ryoku-rashin/internal/protocol"
	"ryoku-rashin/internal/sandbox"
)

// The system prompt is two parts. The prefix is the mode's template for the
// mode on that machine (the tool list, the approvals of an Ask, the vault),
// so llama.cpp reuses its cached tokens across turns. A project session also
// names its folder there: a small model anchors every path on the first one
// it reads, and that costs the cache only between sessions. The
// footer follows it with what belongs to this session: its brief, the
// project's context files, the context providers' blocks. What changes turn
// by turn (date, folder, terminal) rides the user turn instead (Reminder).

//go:embed prompts/compact/*.md prompts/full/*.md
var promptFiles embed.FS

// PromptInput is everything one system prompt is built from. The runner
// fills it once per turn.
type PromptInput struct {
	Profile tool.Profile
	// Template is ModeProfile.Prompt; "" uses the mode's own name.
	Template string
	Session  protocol.SessionInfo
	// Cwd is the session folder; Project the Git top level holding it, ""
	// outside Git. The project's context files are found between the two.
	Cwd, Project string
	Settings     protocol.Settings
	Sandbox      *sandbox.Spec
	// Brief is Options.Brief(): the session section (brief.Agent).
	Brief string
	// Blocks are the context providers' blocks in Deps order.
	Blocks []Block
	// Tools are the names the model is offered this turn (tool.Set.Names).
	Tools []string
	// Habits is the Ask user-habits summary, "" when the setting is off.
	Habits string
	// Role and Context are a subagent's: the agent kind's prompt for the
	// class and the shared context its parent sent with the task.
	Role, Context string
}

// CompactPromptMax is the most bytes a compact template may render to: the
// small-model budget the compact tests hold every mode to.
const CompactPromptMax = 3 << 10

// BuildSystem renders the system prompt as its byte-stable prefix and its
// session footer. The prefix depends only on the template, the class, the
// tools, an Ask's approvals and the machine's vault, never on the turn.
func BuildSystem(in PromptInput) (prefix, footer string) {
	return renderPrefix(in), renderFooter(in)
}

// SystemPrompt is the whole system prompt: prefix, then footer.
func SystemPrompt(in PromptInput) string {
	prefix, footer := BuildSystem(in)
	if footer == "" {
		return prefix
	}
	return prefix + "\n\n" + footer
}

// promptData is what a template sees.
type promptData struct {
	Mode      string
	ToolList  string
	Approvals string
	Rashin    string
	Teams     string
	// VaultDir is "" without a vault, and for a sandboxed team task, which
	// cannot read it.
	VaultDir   string
	VaultGuide string
	// Vault quotes the vault's own files, for a full Ask.
	Vault   string
	Mindmap bool
	Sandbox bool
	// Folder is the session's folder when it is a project rather than the
	// home folder: small models anchor every path they write on it.
	Folder string
	// Memory is what to say of the memory tools, "" when none is offered.
	Memory string
	tools  map[string]bool
}

// Has reports whether any of names is offered this turn.
func (d promptData) Has(names ...string) bool {
	for _, name := range names {
		if d.tools[name] {
			return true
		}
	}
	return false
}

type promptSets struct {
	compact, full *template.Template
}

var (
	promptOnce sync.Once
	prompts    promptSets
)

func loadPrompts() promptSets {
	promptOnce.Do(func() {
		prompts.compact = parsePromptSet("compact")
		prompts.full = parsePromptSet("full")
	})
	return prompts
}

// parsePromptSet panics on a bad template: they are embedded, so the tests
// catch one before it ships.
func parsePromptSet(class string) *template.Template {
	sub, err := fs.Sub(promptFiles, "prompts/"+class)
	if err != nil {
		panic(err)
	}
	return template.Must(template.New(class).Option("missingkey=error").ParseFS(sub, "*.md"))
}

func templateName(in PromptInput) string {
	name := in.Template
	if name == "" {
		name = string(in.Profile.Mode)
	}
	return name + ".md"
}

func renderPrefix(in PromptInput) string {
	sets := loadPrompts()
	set := sets.full
	if in.Profile.Compact() {
		set = sets.compact
	}
	data := prefixData(in)
	var b bytes.Buffer
	if err := set.ExecuteTemplate(&b, templateName(in), data); err != nil {
		// An unknown template falls back to the mode's own, then agent's.
		b.Reset()
		fallback := string(in.Profile.Mode) + ".md"
		if set.Lookup(fallback) == nil {
			fallback = string(tool.ModeAgent) + ".md"
		}
		if err := set.ExecuteTemplate(&b, fallback, data); err != nil {
			panic(fmt.Sprintf("engine: prompt %s: %v", fallback, err))
		}
	}
	return normalizePrompt(b.String())
}

func prefixData(in PromptInput) promptData {
	tools := make(map[string]bool, len(in.Tools))
	for _, name := range in.Tools {
		tools[name] = true
	}
	sandbox := in.Session.Team != "" || in.Profile.Mode == tool.ModeTeam || in.Profile.Mode == tool.ModeDreamManager
	data := promptData{
		Mode:      string(in.Profile.Mode),
		ToolList:  strings.Join(in.Tools, ", "),
		Approvals: in.Session.Mode,
		Teams:     brief.Teams(tools[tool.ProposeTeam]),
		Mindmap:   in.Settings.Features["mindmap"],
		Sandbox:   sandbox,
		tools:     tools,
	}
	if in.Cwd != "" && (in.Project != "" || filepath.Clean(in.Cwd) != filepath.Clean(paths.Home())) {
		data.Folder = in.Cwd
	}
	if !sandbox {
		data.VaultDir = brief.VaultDir()
	}
	if data.VaultDir != "" {
		data.VaultGuide = brief.VaultGuide(data.VaultDir)
	}
	data.Rashin = brief.Rashin(data.VaultDir != "")
	if tools[tool.MemorySearch] {
		data.Memory = brief.MemoryInstructions(tools[tool.SessionSearch])
	}
	if !in.Profile.Compact() && (in.Profile.Mode == tool.ModeAskRead || in.Profile.Mode == tool.ModeAskAct) {
		data.Vault = brief.Vault(vaultBudget(in.Profile))
	}
	return data
}

// vaultBudget is the bytes a full Ask quotes of the vault: 28 KiB, or an
// eighth of a smaller window.
func vaultBudget(p tool.Profile) int {
	return min(28<<10, promptWindowBytes(p)/8)
}

// promptWindowBytes is the profile's window in bytes, at four bytes a token.
func promptWindowBytes(p tool.Profile) int {
	window := p.Window
	if window <= 0 {
		window = tool.DefaultWindow
	}
	return window * 4
}

func renderFooter(in PromptInput) string {
	compact := in.Profile.Compact()
	var parts []string
	add := func(heading, body string) {
		if body = strings.TrimSpace(body); body == "" {
			return
		}
		if heading != "" {
			body = "# " + heading + "\n" + body
		}
		parts = append(parts, body)
	}
	add("Your role", in.Role)
	add("Context from the lead agent", in.Context)
	add("", in.Brief)
	if projectRulesFor(in.Profile.Mode, compact) {
		add("Project rules", projectContext(in.Cwd, in.Project, projectBudget(in.Profile), in.Sandbox))
	}
	add("User habits", in.Habits)
	remaining := blocksBudget(in.Profile)
	for _, block := range in.Blocks {
		text := strings.TrimSpace(block.Text)
		if text == "" {
			continue
		}
		if block.Budget > 0 {
			text = promptClip(text, block.Budget)
		}
		if len(text) > remaining {
			// Earlier blocks win: a late block gets what is left when that
			// is still worth reading, else it is dropped.
			if remaining < minBlockBytes {
				continue
			}
			text = promptClip(text, remaining)
		}
		remaining -= len(text)
		add(blockHeading(block.Name), text)
	}
	return normalizePrompt(strings.Join(parts, "\n\n"))
}

// minBlockBytes is the least of a block worth sending once the budget runs
// short.
const minBlockBytes = 512

// projectRulesFor says which modes are told the project's context files:
// every mode that works a folder, not Ask (its folder is wherever the user
// happened to be) and not a compact subagent, whose assignment says enough.
func projectRulesFor(mode tool.Mode, compact bool) bool {
	switch mode {
	case tool.ModeAskRead, tool.ModeAskAct:
		return false
	case tool.ModeSubagent:
		return !compact
	}
	return true
}

// projectBudget caps the project's context files: 2 KiB for a compact
// model, else 24 KiB or a quarter of a smaller window.
func projectBudget(p tool.Profile) int {
	if p.Compact() {
		return 2 << 10
	}
	return min(24<<10, promptWindowBytes(p)/4)
}

// blocksBudget caps the context providers' blocks together: a quarter of
// the window, at most 12 KiB for a compact model and 96 KiB otherwise.
func blocksBudget(p tool.Profile) int {
	limit := 96 << 10
	if p.Compact() {
		limit = 12 << 10
	}
	return min(limit, promptWindowBytes(p)/4)
}

func blockHeading(name string) string {
	switch name {
	case BlockSkills:
		return "Skills"
	case BlockDecisions:
		return "Project decisions"
	case BlockRules:
		return "Rules"
	case BlockMemory:
		return "Memory"
	case "":
		return ""
	}
	return strings.ToUpper(name[:1]) + name[1:]
}

// promptClip cuts s to at most max bytes, at the last line break inside the
// limit when there is one, and marks the cut.
func promptClip(s string, max int) string {
	if len(s) <= max {
		return s
	}
	const mark = "\n[trimmed]"
	keep := max - len(mark)
	if keep <= 0 {
		return ""
	}
	cut := s[:keep]
	if i := strings.LastIndexByte(cut, '\n'); i > keep/2 {
		cut = cut[:i]
	} else {
		cut = strings.ToValidUTF8(cut, "")
	}
	return strings.TrimRight(cut, " \t\n") + mark
}

// normalizePrompt trims trailing spaces from every line, keeps at most one
// blank line between paragraphs and drops blank lines at both ends, so a
// template's own spacing never shows in what the model reads. Fenced code
// keeps its blank lines.
func normalizePrompt(s string) string {
	lines := strings.Split(s, "\n")
	out := make([]string, 0, len(lines))
	fenced, blank := false, false
	for _, line := range lines {
		line = strings.TrimRight(line, " \t\r")
		if strings.HasPrefix(strings.TrimSpace(line), "```") {
			fenced = !fenced
		}
		if line == "" && !fenced {
			if blank || len(out) == 0 {
				continue
			}
			blank = true
			out = append(out, line)
			continue
		}
		blank = false
		out = append(out, line)
	}
	for len(out) > 0 && out[len(out)-1] == "" {
		out = out[:len(out)-1]
	}
	return strings.Join(out, "\n")
}
