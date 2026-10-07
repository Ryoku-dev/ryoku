package main

import (
	"context"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// Foreign-file pointer blocks are fenced with these markers so wiring is
// idempotent and cleanly reversible.
const (
	pointerBegin = "<!-- ryoku-rashin:begin -->"
	pointerEnd   = "<!-- ryoku-rashin:end -->"
)

// PointerBlock is the fenced note appended to each agent's instructions file,
// telling it the vault exists and to read it first.
const PointerBlock = pointerBegin + "\n" +
	"## Ryoku Rashin system vault\n" +
	"\n" +
	"This machine runs Ryoku (Arch Linux with the Ryoku desktop on Hyprland or niri;\n" +
	"`ryoku wm status` says which). A maintained map of the system lives at\n" +
	"`~/.local/share/ryoku/rashin/`. Before exploring the machine or guessing paths,\n" +
	"read `AGENTS.md` there: it says where every config lives, which binary owns it,\n" +
	"and how to reload it. Write durable notes to `memory/` and dated notes to\n" +
	"`journal/YYYY-MM-DD.md`. For code questions prefer `prowl` (cited code\n" +
	"intelligence, reindexed each run): `prowl search \"<question>\"`, `find`, `def`,\n" +
	"`references`, `outline`, `impact` -- one call instead of grepping. The `ryoku`\n" +
	"agent skill (safety rules, the GUI map, the bar and dock, the window-manager\n" +
	"seam, building the Ryoku way, and the command catalogue) is wired into this\n" +
	"agent's skills directory; read it before customising the desktop, and cite the\n" +
	"vault's `wiki/` pages to someone new. Answer a desktop \"how do I\" question\n" +
	"GUI-first: name the Ryoku Hub page, shell picker, or QS Bar Settings before\n" +
	"naming any command.\n" +
	pointerEnd

// Agent is a detected coding CLI and its vault-pointer wiring state.
type Agent struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Present bool   `json:"present"`
	Wired   bool   `json:"wired"`
	File    string `json:"file"`
	// SkillWired is true when the agent's skills dir carries the ryoku skill
	// symlink. Agents with no skills dir (opencode) report false.
	SkillWired bool `json:"skillWired"`
}

// agentDef describes where an agent lives and where its pointer block goes.
// gate is the directory that must already exist before Wire may create file;
// for most agents it equals the agent home, but opencode wires into an
// on-demand ~/.config/opencode as long as ~/.config exists.
type agentDef struct {
	id   string
	name string
	home func() string // the agent's own config dir; Present when it exists
	gate func() string // the dir that must exist for Wire to proceed
	file func() string // the instructions file the pointer block lands in
	// skill is the symlink path the ryoku skill dir lands at for this agent, or
	// "" for an agent with no skills directory.
	skill func() string
}

func agentDefs() []agentDef {
	return []agentDef{
		{
			id: "claude", name: "Claude Code",
			home:  func() string { return filepath.Join(home(), ".claude") },
			gate:  func() string { return filepath.Join(home(), ".claude") },
			file:  func() string { return filepath.Join(home(), ".claude", "CLAUDE.md") },
			skill: func() string { return filepath.Join(home(), ".claude", "skills", "ryoku") },
		},
		{
			id: "codex", name: "Codex CLI",
			home:  func() string { return filepath.Join(home(), ".codex") },
			gate:  func() string { return filepath.Join(home(), ".codex") },
			file:  func() string { return filepath.Join(home(), ".codex", "AGENTS.md") },
			skill: func() string { return filepath.Join(home(), ".codex", "skills", "ryoku") },
		},
		{
			id: "opencode", name: "opencode",
			home: func() string { return filepath.Join(configHome(), "opencode") },
			gate: func() string { return configHome() },
			file: func() string { return filepath.Join(configHome(), "opencode", "AGENTS.md") },
		},
		{
			id: "omp", name: "Oh My Pi",
			home:  func() string { return filepath.Join(home(), ".omp") },
			gate:  func() string { return filepath.Join(home(), ".omp") },
			file:  func() string { return filepath.Join(home(), ".omp", "agent", "AGENTS.md") },
			skill: func() string { return filepath.Join(home(), ".omp", "agent", "skills", "ryoku") },
		},
	}
}

// DetectAgents reports each known agent's presence and wiring state.
func DetectAgents() []Agent {
	defs := agentDefs()
	out := make([]Agent, 0, len(defs))
	for _, d := range defs {
		out = append(out, Agent{
			ID:         d.id,
			Name:       d.name,
			Present:    dirExists(d.home()),
			Wired:      fileHasBlock(d.file()),
			File:       tildeAbbrev(d.file()),
			SkillWired: agentSkillWired(d),
		})
	}
	return out
}

// Wire upserts the pointer block into an agent's file, creating the file (and
// the parent dir only when the agent's gate dir already exists). It never
// creates an agent's home dir; opencode is the sole exception, where the gate
// is ~/.config and Wire may create ~/.config/opencode beneath it.
func Wire(id string) error {
	d, ok := lookupAgent(id)
	if !ok {
		return os.ErrInvalid
	}
	if !dirExists(d.gate()) {
		return &os.PathError{Op: "wire", Path: d.gate(), Err: os.ErrNotExist}
	}
	file := d.file()
	if err := os.MkdirAll(filepath.Dir(file), 0o755); err != nil {
		return err
	}
	doc := readFileOrEmpty(file)
	if err := atomicWrite(file, []byte(upsertBlock(doc)), 0o644); err != nil {
		return err
	}
	// Wiring an agent also drops the ryoku skill into its skills dir.
	err := wireAgentSkill(d)
	invalidateHarnessCache()
	return err
}

// Unwire removes the pointer block from an agent's file, keeping the file.
func Unwire(id string) error {
	d, ok := lookupAgent(id)
	if !ok {
		return os.ErrInvalid
	}
	file := d.file()
	// The skill symlink is independent of the pointer block; drop it too.
	unwireAgentSkill(d)
	invalidateHarnessCache()
	doc := readFileOrEmpty(file)
	if doc == "" {
		return nil
	}
	return atomicWrite(file, []byte(removeBlock(doc)), 0o644)
}

// WireAll wires every present agent and returns how many it wired.
func WireAll() int {
	n := 0
	for _, d := range agentDefs() {
		if !dirExists(d.home()) {
			continue
		}
		if Wire(d.id) == nil {
			n++
		}
	}
	// Also link the always-created homes (~/.agents, ~/.hermes) and every
	// hermes profile; the per-agent links were made by Wire above.
	_, _ = WireSkill()
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	_ = migrateQuickProviderKeys(ctx)
	_ = routeConnectedHarnesses(ctx)
	wireProwlSkills()
	return n
}

// upsertBlock replaces an existing pointer block or appends one, so calling it
// repeatedly is stable.
func upsertBlock(doc string) string {
	bi := strings.Index(doc, pointerBegin)
	ei := strings.Index(doc, pointerEnd)
	if bi >= 0 && ei > bi {
		before := doc[:bi]
		after := doc[ei+len(pointerEnd):]
		return before + PointerBlock + after
	}
	if strings.TrimSpace(doc) == "" {
		return PointerBlock + "\n"
	}
	return strings.TrimRight(doc, "\n") + "\n\n" + PointerBlock + "\n"
}

// removeBlock deletes the pointer block and collapses the blank lines that
// surrounded it, leaving the rest of the file intact.
func removeBlock(doc string) string {
	bi := strings.Index(doc, pointerBegin)
	ei := strings.Index(doc, pointerEnd)
	if bi < 0 || ei <= bi {
		return doc
	}
	before := strings.TrimRight(doc[:bi], "\n")
	after := strings.TrimLeft(doc[ei+len(pointerEnd):], "\n")
	switch {
	case before == "" && after == "":
		return ""
	case before == "":
		return after
	case after == "":
		return before + "\n"
	default:
		return before + "\n\n" + after
	}
}

func lookupAgent(id string) (agentDef, bool) {
	for _, d := range agentDefs() {
		if d.id == id {
			return d, true
		}
	}
	return agentDef{}, false
}

// configHome is the XDG config root, derived from ConfigPath so the fallback
// logic lives in one place (paths.go).
func configHome() string {
	return filepath.Dir(filepath.Dir(ConfigPath()))
}

func dirExists(p string) bool {
	fi, err := os.Stat(p)
	return err == nil && fi.IsDir()
}

func fileHasBlock(p string) bool {
	b, err := os.ReadFile(p)
	if err != nil {
		return false
	}
	s := string(b)
	return strings.Contains(s, pointerBegin) && strings.Contains(s, pointerEnd)
}

func readFileOrEmpty(p string) string {
	b, err := os.ReadFile(p)
	if err != nil {
		return ""
	}
	return string(b)
}

// tildeAbbrev renders an absolute home path as ~/... for display.
func tildeAbbrev(p string) string {
	h := home()
	if p == h {
		return "~"
	}
	if strings.HasPrefix(p, h+string(os.PathSeparator)) {
		return "~" + p[len(h):]
	}
	return p
}

// ---- the ryoku agent skill --------------------------------------------------
//
// Wiring also symlinks the shipped `ryoku` skill dir into each agent's skills
// directory, so an agent finds the bar, dock, and plugin guides and the command
// catalogue the same way it finds a hub- or agent-grown skill. Every link
// points at one source dir, resolved in a single order.

// skillRoots is the resolution order for the shipped skill's parent dir: an
// explicit override, the packaged tree, then the dev checkout the last deploy
// recorded. Each root holds a `ryoku` subdir.
func skillRoots() []string {
	var roots []string
	if v := os.Getenv("RYOKU_RASHIN_SKILLS"); v != "" {
		roots = append(roots, v)
	}
	roots = append(roots, "/usr/share/ryoku/skills")
	if repo := recordedCheckout(); repo != "" {
		roots = append(roots, filepath.Join(repo, "ryoku", "rashin", "skills"))
	}
	return roots
}

// skillSourceCandidates lists every `ryoku` skill dir the resolution order could
// name, whether or not it exists. Unwire matches a link's target against these.
func skillSourceCandidates() []string {
	roots := skillRoots()
	out := make([]string, 0, len(roots))
	for _, r := range roots {
		out = append(out, filepath.Clean(filepath.Join(r, "ryoku")))
	}
	return out
}

// skillSourceDir returns the shipped `ryoku` skill dir to link, or "" when no
// resolution root carries a SKILL.md.
func skillSourceDir() string {
	for _, d := range skillSourceCandidates() {
		if fileExists(filepath.Join(d, "SKILL.md")) {
			return d
		}
	}
	return ""
}

// skillLinkTargets is every skills/ryoku link path wire maintains: the
// always-created ~/.agents and ~/.hermes, each present coding agent, and every
// existing hermes profile.
func skillLinkTargets() []string {
	h := home()
	out := []string{
		filepath.Join(h, ".agents", "skills", "ryoku"),
		filepath.Join(h, ".hermes", "skills", "ryoku"),
	}
	for _, d := range agentDefs() {
		if d.skill == nil {
			continue
		}
		if link := d.skill(); link != "" && dirExists(d.home()) {
			out = append(out, link)
		}
	}
	profiles, _ := filepath.Glob(filepath.Join(h, ".hermes", "profiles", "*"))
	for _, p := range profiles {
		if dirExists(p) {
			out = append(out, filepath.Join(p, "skills", "ryoku"))
		}
	}
	return out
}

// WireSkill symlinks the shipped skill dir into every applicable location and
// returns how many links now point at it. ~/.agents and ~/.hermes are created;
// a coding agent's link is skipped when its home is absent. It is a no-op when
// the skill dir cannot be resolved, so an unpackaged, un-deployed box never
// fails on it.
func WireSkill() (int, error) {
	src := skillSourceDir()
	if src == "" {
		return 0, nil
	}
	n := 0
	for _, link := range skillLinkTargets() {
		if err := os.MkdirAll(filepath.Dir(link), 0o755); err != nil {
			return n, err
		}
		wrote, err := symlinkForce(link, src)
		if err != nil {
			return n, err
		}
		if wrote {
			n++
		}
	}
	return n, nil
}

// UnwireSkill removes every skills/ryoku symlink that points at a known skill
// source dir, and nothing else.
func UnwireSkill() {
	for _, link := range skillLinkTargets() {
		removeSkillLinkIfOurs(link)
	}
}

// wireAgentSkill links the skill dir into one agent's skills dir, best effort.
func wireAgentSkill(d agentDef) error {
	if d.skill == nil {
		return nil
	}
	link := d.skill()
	src := skillSourceDir()
	if link == "" || src == "" {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(link), 0o755); err != nil {
		return err
	}
	_, err := symlinkForce(link, src)
	return err
}

// unwireAgentSkill removes one agent's skill symlink when it is ours.
func unwireAgentSkill(d agentDef) {
	if d.skill == nil {
		return
	}
	if link := d.skill(); link != "" {
		removeSkillLinkIfOurs(link)
	}
}

// agentSkillWired reports whether an agent's skills dir carries our skill link.
func agentSkillWired(d agentDef) bool {
	if d.skill == nil {
		return false
	}
	link := d.skill()
	return link != "" && isOurSkillLink(link)
}

// symlinkForce points link at target with `ln -sfn` semantics: an existing
// symlink is replaced, a correct one is left alone, and a real file or foreign
// skill dir is never clobbered. Hermes keeps skills in category folders
// (skills/<category>/<name>), so a real directory without a SKILL.md of its
// own is treated as a category and the link is laid inside it as <dir>/ryoku.
// It reports whether a link now points at target.
func symlinkForce(link, target string) (bool, error) {
	if fi, err := os.Lstat(link); err == nil {
		if fi.Mode()&os.ModeSymlink == 0 {
			if fi.IsDir() && !fileExists(filepath.Join(link, "SKILL.md")) {
				return symlinkForce(filepath.Join(link, "ryoku"), target)
			}
			return false, nil // a real file or foreign skill dir; keep the user's data
		}
		if dest, _ := os.Readlink(link); filepath.Clean(dest) == filepath.Clean(target) {
			return true, nil
		}
		if err := os.Remove(link); err != nil {
			return false, err
		}
	}
	if err := os.Symlink(target, link); err != nil {
		return false, err
	}
	return true, nil
}

// isOurSkillLink reports whether link is a symlink to a ryoku skill dir: one of
// the resolution order's candidates, or any directory carrying our SKILL.md
// (frontmatter `name: ryoku`). The second form matters on a dev box, where the
// links were laid by one checkout's deploy and the repo pointer has since moved
// to another: the agent still finds the skill, so it is wired, and unwire may
// still remove it. A foreign `ryoku` skill without that marker is never ours.
func isOurSkillLink(link string) bool {
	fi, err := os.Lstat(link)
	if err != nil {
		return false
	}
	if fi.Mode()&os.ModeSymlink == 0 {
		// A category folder (a real dir with no SKILL.md of its own) holds the
		// skill as <dir>/ryoku; that nested link is what hermes loads.
		if !fi.IsDir() || fileExists(filepath.Join(link, "SKILL.md")) {
			return false
		}
		return isOurSkillLink(filepath.Join(link, "ryoku"))
	}
	dest, err := os.Readlink(link)
	if err != nil {
		return false
	}
	dest = filepath.Clean(dest)
	for _, c := range skillSourceCandidates() {
		if dest == c {
			return true
		}
	}
	return skillDirIsOurs(dest)
}

// skillDirIsOurs: the directory holds a SKILL.md whose frontmatter names the
// ryoku skill. Only the head of the file is read.
func skillDirIsOurs(dir string) bool {
	f, err := os.Open(filepath.Join(dir, "SKILL.md"))
	if err != nil {
		return false
	}
	defer f.Close()
	head := make([]byte, 512)
	n, _ := f.Read(head)
	for _, line := range strings.Split(string(head[:n]), "\n") {
		if strings.TrimSpace(line) == "name: ryoku" {
			return true
		}
	}
	return false
}

// removeSkillLinkIfOurs deletes link only when it is one of our skill symlinks.
func removeSkillLinkIfOurs(link string) {
	if fi, err := os.Lstat(link); err == nil && fi.IsDir() && fi.Mode()&os.ModeSymlink == 0 &&
		!fileExists(filepath.Join(link, "SKILL.md")) {
		link = filepath.Join(link, "ryoku") // the category layout keeps the link inside
	}
	if isOurSkillLink(link) {
		_ = os.Remove(link)
	}
}

// ---- prowl skills -----------------------------------------------------------

func prowlSkillClients() []string {
	var clients []string
	for _, agent := range DetectAgents() {
		if agent.Present {
			clients = append(clients, agent.ID)
		}
	}
	if _, ok := FindHermes(); ok {
		clients = append(clients, "hermes")
	}
	return clients
}

func installProwlSkills(ctx context.Context, clients []string) error {
	if len(clients) == 0 {
		return nil
	}
	return prowlGatewayJSON(ctx, http.MethodPost, "/api/setup/skills",
		map[string]any{"clients": clients}, nil)
}

func wireProwlSkills() {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	_ = migrateQuickProviderKeys(ctx)
	_ = installProwlSkills(ctx, prowlSkillClients())
}

func connectHarness(ctx context.Context, id string) (bool, string, error) {
	if id == "gemini" {
		return false, "", errors.New(geminiRoutingReason)
	}
	if err := wireRashinHarness(id); err != nil {
		return false, "", err
	}
	skills := ""
	if setup, err := gatewayRouting(ctx); err == nil {
		for _, row := range setup.Harnesses {
			if row.ID == id {
				skills = row.Skills
				break
			}
		}
	}
	pending, reason, err := routeProwlHarness(ctx, id)
	if err != nil {
		return false, "", err
	}
	cfg := LoadConfig()
	if cfg.AddProwlHarness(id) {
		if err := SaveConfig(cfg); err != nil {
			return false, "", err
		}
	}
	if skills != "unsupported" {
		if err := installProwlSkills(ctx, []string{id}); err != nil {
			return false, "", err
		}
	}
	return pending, reason, nil
}

func disconnectHarness(ctx context.Context, id string) error {
	if err := prowlGatewayJSON(ctx, http.MethodDelete, "/api/setup/harnesses/"+id, nil, nil); err != nil {
		return err
	}
	cfg := LoadConfig()
	if cfg.RemoveProwlHarness(id) {
		if err := SaveConfig(cfg); err != nil {
			return err
		}
	}
	return unwireRashinHarness(id)
}

func wireRashinHarness(id string) error {
	if id == "hermes" {
		err := WireHermesMemory()
		invalidateHarnessCache()
		return err
	}
	if _, ok := lookupAgent(id); ok {
		return Wire(id)
	}
	for _, harness := range harnessDefs() {
		if harness.id == id {
			return nil
		}
	}
	return os.ErrInvalid
}

func unwireRashinHarness(id string) error {
	if id == "hermes" {
		file := hermesMemory()
		removeSkillLinkIfOurs(filepath.Join(home(), ".hermes", "skills", "ryoku"))
		invalidateHarnessCache()
		doc := readFileOrEmpty(file)
		if doc == "" {
			return nil
		}
		return atomicWrite(file, []byte(removeBlock(doc)), 0o644)
	}
	if _, ok := lookupAgent(id); ok {
		return Unwire(id)
	}
	for _, harness := range harnessDefs() {
		if harness.id == id {
			return nil
		}
	}
	return os.ErrInvalid
}
