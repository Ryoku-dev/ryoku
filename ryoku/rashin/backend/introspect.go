package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// introspect.go reads Hermes's install (skills, toolsets, memory provider,
// session history) so the dashboard can show what the resident agent knows
// and has grown, all without running hermes itself.

const rashinVersion = "0.2.0"

// ---- skills ----------------------------------------------------------------

type Skill struct {
	Name        string `json:"name"`
	Dir         string `json:"dir"`
	Description string `json:"description"`
	Version     string `json:"version,omitempty"`
	Origin      string `json:"origin"` // bundled | hub | agent
}

type SkillCategory struct {
	Name   string  `json:"name"`
	Skills []Skill `json:"skills"`
}

type ToolFamily struct {
	Family string   `json:"family"`
	Tools  []string `json:"tools"`
}

type SkillsReport struct {
	Counts     map[string]int  `json:"counts"`
	Categories []SkillCategory `json:"categories"`
	Toolbelt   []ToolFamily    `json:"toolbelt"`
}

func hermesSkillsDir() string { return filepath.Join(home(), ".hermes", "skills") }

// bundledSkillNames parses skills/.bundled_manifest (name:md5 per line).
func bundledSkillNames() map[string]bool {
	out := map[string]bool{}
	b, err := os.ReadFile(filepath.Join(hermesSkillsDir(), ".bundled_manifest"))
	if err != nil {
		return out
	}
	for _, line := range strings.Split(string(b), "\n") {
		if name, _, ok := strings.Cut(strings.TrimSpace(line), ":"); ok && name != "" {
			out[name] = true
		}
	}
	return out
}

// hubSkillNames parses skills/.hub/lock.json when the skills hub is in use.
func hubSkillNames() map[string]bool {
	out := map[string]bool{}
	b, err := os.ReadFile(filepath.Join(hermesSkillsDir(), ".hub", "lock.json"))
	if err != nil {
		return out
	}
	var doc map[string]any
	if json.Unmarshal(b, &doc) != nil {
		return out
	}
	for k := range doc {
		out[k] = true
	}
	return out
}

// skillFrontmatter pulls name/description/version from a SKILL.md header.
func skillFrontmatter(path string) (name, desc, version string) {
	b, err := os.ReadFile(path)
	if err != nil {
		return "", "", ""
	}
	return parseFrontmatter(string(b))
}

// parseFrontmatter pulls name/description/version from SKILL.md text. Shared
// by the hermes report and the cross-harness scan.
func parseFrontmatter(s string) (name, desc, version string) {
	if !strings.HasPrefix(s, "---") {
		return "", "", ""
	}
	body := s[3:]
	end := strings.Index(body, "\n---")
	if end < 0 {
		return "", "", ""
	}
	for _, line := range strings.Split(body[:end], "\n") {
		t := strings.TrimSpace(line)
		if v, ok := strings.CutPrefix(t, "name:"); ok && name == "" {
			name = strings.Trim(strings.TrimSpace(v), `"'`)
		}
		if v, ok := strings.CutPrefix(t, "description:"); ok && desc == "" {
			desc = strings.Trim(strings.TrimSpace(v), `"'`)
		}
		if v, ok := strings.CutPrefix(t, "version:"); ok && version == "" {
			version = strings.Trim(strings.TrimSpace(v), `"'`)
		}
	}
	return name, desc, version
}

// SkillsReportNow walks ~/.hermes/skills: both <cat>/<skill>/SKILL.md and
// loose <skill>/SKILL.md layouts. Provenance is by name: bundled manifest
// first, hub lock second, anything else is agent-grown.
func SkillsReportNow() SkillsReport {
	rep := SkillsReport{
		Counts:   map[string]int{"bundled": 0, "hub": 0, "agent": 0},
		Toolbelt: toolbeltNow(),
	}
	root := hermesSkillsDir()
	if !dirExists(root) {
		return rep
	}
	bundled, hub := bundledSkillNames(), hubSkillNames()
	cats := map[string][]Skill{}

	addSkill := func(cat, dir, skillMD string) {
		name, desc, version := skillFrontmatter(skillMD)
		if name == "" {
			name = filepath.Base(dir)
		}
		origin := "agent"
		if bundled[name] || bundled[filepath.Base(dir)] {
			origin = "bundled"
		} else if hub[name] || hub[filepath.Base(dir)] {
			origin = "hub"
		}
		rep.Counts[origin]++
		rel, _ := filepath.Rel(root, dir)
		cats[cat] = append(cats[cat], Skill{
			Name: name, Dir: rel, Description: desc, Version: version, Origin: origin,
		})
	}

	_ = filepath.WalkDir(root, func(p string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() && strings.HasPrefix(d.Name(), ".") {
			return filepath.SkipDir
		}
		if d.IsDir() || d.Name() != "SKILL.md" {
			return nil
		}
		dir := filepath.Dir(p)
		rel, rerr := filepath.Rel(root, dir)
		if rerr != nil {
			return nil
		}
		cat := "general"
		if parts := strings.Split(rel, string(filepath.Separator)); len(parts) > 1 {
			cat = parts[0]
		}
		addSkill(cat, dir, p)
		return filepath.SkipDir // one skill per directory; skip nested rescans
	})

	names := make([]string, 0, len(cats))
	for k := range cats {
		names = append(names, k)
	}
	sort.Strings(names)
	for _, n := range names {
		sort.Slice(cats[n], func(i, j int) bool { return cats[n][i].Name < cats[n][j].Name })
		rep.Categories = append(rep.Categories, SkillCategory{Name: n, Skills: cats[n]})
	}
	return rep
}

// toolFamilies maps hermes toolset names into display families.
var toolFamilies = []struct {
	family string
	sets   []string
}{
	{"System & Code", []string{"terminal", "file", "code_execution", "computer_use"}},
	{"Web", []string{"web", "browser"}},
	{"Mind", []string{"memory", "session_search", "skills", "todo", "clarify"}},
	{"Media", []string{"image_gen", "vision", "tts", "voice"}},
	{"Orchestration", []string{"delegation", "cronjob", "gateway"}},
}

// toolbeltNow parses platform_toolsets.cli from config.yaml (block list form).
func toolbeltNow() []ToolFamily {
	b, err := os.ReadFile(hermesConfig())
	if err != nil {
		return nil
	}
	enabled := map[string]bool{}
	lines := strings.Split(string(b), "\n")
	inPlatform, inCLI := false, false
	for _, line := range lines {
		t := strings.TrimSpace(line)
		indent := len(line) - len(strings.TrimLeft(line, " \t"))
		switch {
		case strings.HasPrefix(t, "platform_toolsets:"):
			inPlatform, inCLI = true, false
		case inPlatform && indent == 0 && t != "":
			inPlatform, inCLI = false, false
		case inPlatform && strings.HasPrefix(t, "cli:"):
			inCLI = true
		case inPlatform && inCLI && strings.HasPrefix(t, "- "):
			enabled[strings.TrimSpace(strings.TrimPrefix(t, "- "))] = true
		case inPlatform && inCLI && t != "" && !strings.HasPrefix(t, "- "):
			inCLI = false
		}
	}
	if len(enabled) == 0 {
		return nil
	}
	var out []ToolFamily
	seen := map[string]bool{}
	for _, f := range toolFamilies {
		var tools []string
		for _, s := range f.sets {
			if enabled[s] {
				tools = append(tools, s)
				seen[s] = true
			}
		}
		if len(tools) > 0 {
			out = append(out, ToolFamily{Family: f.family, Tools: tools})
		}
	}
	var rest []string
	for s := range enabled {
		if !seen[s] {
			rest = append(rest, s)
		}
	}
	if len(rest) > 0 {
		sort.Strings(rest)
		out = append(out, ToolFamily{Family: "Other", Tools: rest})
	}
	return out
}

// ---- memory ----------------------------------------------------------------

type MemoryReport struct {
	Provider struct {
		Kind          string `json:"kind"`
		ObsidianVault string `json:"obsidianVault,omitempty"`
	} `json:"provider"`
	Files struct {
		MemoryMd    bool  `json:"memoryMd"`
		MemoryBytes int64 `json:"memoryBytes"`
		UserMd      bool  `json:"userMd"`
	} `json:"files"`
	// Learned is the growth layer: what the agent has accumulated on its own,
	// beyond the maps Ryoku generates. It fills as hermes runs.
	Learned struct {
		MemoryEntries int      `json:"memoryEntries"`
		UserFacts     int      `json:"userFacts"`
		AgentSkills   int      `json:"agentSkills"`
		VaultNotes    int      `json:"vaultNotes"`
		JournalDays   int      `json:"journalDays"`
		Preview       []string `json:"preview,omitempty"`
	} `json:"learned"`
	Graph        GraphModel     `json:"graph"`
	Heatmap      []HeatmapDay   `json:"heatmap"`
	Sessions     []SessionEntry `json:"sessions"`
	SessionsNote string         `json:"sessionsNote,omitempty"`
}

// memoryEntries splits a hermes memory file into its curated entries. The
// agent separates entries with a bare section sign line; the rashin pointer
// fence does not count as learned content.
func memoryEntries(raw string) []string {
	// Drop the rashin pointer block: it is wiring, not learned memory.
	for {
		bi := strings.Index(raw, "<!-- ryoku-rashin:begin -->")
		ei := strings.Index(raw, "<!-- ryoku-rashin:end -->")
		if bi < 0 || ei <= bi {
			break
		}
		raw = raw[:bi] + raw[ei+len("<!-- ryoku-rashin:end -->"):]
	}
	var out []string
	for _, part := range strings.Split(raw, "\n\u00a7\n") {
		if s := strings.TrimSpace(part); s != "" {
			out = append(out, s)
		}
	}
	return out
}

// entryLabel reduces a memory entry to a short node label.
func entryLabel(entry string) string {
	line := firstLine(entry)
	line = strings.TrimLeft(line, "#-* \t")
	if len(line) > 42 {
		line = line[:42]
	}
	return line
}

type GraphNode struct {
	ID    string `json:"id"`
	Label string `json:"label"`
	Group string `json:"group"`
	Size  int64  `json:"size"`
}

type GraphLink struct {
	Source string `json:"source"`
	Target string `json:"target"`
}

type GraphModel struct {
	Nodes []GraphNode `json:"nodes"`
	Links []GraphLink `json:"links"`
}

type HeatmapDay struct {
	Date  string `json:"date"`
	Count int    `json:"count"`
}

type SessionEntry struct {
	ID        string `json:"id"`
	Title     string `json:"title"`
	Model     string `json:"model"`
	Messages  int    `json:"messages"`
	StartedAt string `json:"startedAt"`
	Source    string `json:"source"`
}

// memoryProviderKind reads memory.provider from config.yaml; builtin when unset.
func memoryProviderKind() (kind, obsidianVault string) {
	kind = "builtin"
	b, err := os.ReadFile(hermesConfig())
	if err != nil {
		return kind, ""
	}
	lines := strings.Split(string(b), "\n")
	inMemory := false
	for _, line := range lines {
		t := strings.TrimSpace(line)
		indent := len(line) - len(strings.TrimLeft(line, " \t"))
		switch {
		case strings.HasPrefix(t, "memory:") && indent == 0:
			inMemory = true
		case inMemory && indent == 0 && t != "":
			inMemory = false
		case inMemory:
			if v, ok := strings.CutPrefix(t, "provider:"); ok {
				if p := strings.Trim(strings.TrimSpace(v), `"'`); p != "" && !strings.HasPrefix(p, "#") {
					kind = p
				}
			}
		}
	}
	if v := os.Getenv("OBSIDIAN_VAULT_PATH"); v != "" {
		obsidianVault = v
	} else if env, err := os.ReadFile(filepath.Join(home(), ".hermes", ".env")); err == nil {
		for _, line := range strings.Split(string(env), "\n") {
			if v, ok := strings.CutPrefix(strings.TrimSpace(line), "OBSIDIAN_VAULT_PATH="); ok {
				obsidianVault = strings.Trim(v, `"'`)
			}
		}
	}
	return kind, obsidianVault
}

// buildMemoryGraph turns the vault into nodes and reference links: markdown
// files are nodes, a mention of another vault file's name is an edge.
func buildMemoryGraph() GraphModel {
	g := GraphModel{}
	files, err := VaultTree()
	if err != nil {
		return g
	}
	group := func(rel string) string {
		switch {
		case rel == "AGENTS.md" || rel == "CLAUDE.md":
			return "hub"
		case strings.HasPrefix(rel, "memory/"):
			return "memory"
		case strings.HasPrefix(rel, "journal/"):
			return "journal"
		default:
			return "generated"
		}
	}
	var nodes []GraphNode
	byBase := map[string]string{} // base name -> id
	for _, f := range files {
		if !strings.HasSuffix(f.Path, ".md") || f.Path == "CLAUDE.md" {
			continue
		}
		id := f.Path
		nodes = append(nodes, GraphNode{
			ID:    id,
			Label: strings.TrimSuffix(filepath.Base(id), ".md"),
			Group: group(id),
			Size:  f.Size,
		})
		byBase[filepath.Base(id)] = id
	}
	// Hermes memory joins the graph as its own node when present, and every
	// curated entry inside it becomes a small "learned" satellite, so the web
	// visibly grows as the agent accumulates knowledge about the user.
	memPath := hermesMemory()
	if fi, err := os.Stat(memPath); err == nil {
		nodes = append(nodes, GraphNode{ID: "hermes:MEMORY.md", Label: "hermes MEMORY", Group: "hermes", Size: fi.Size()})
		byBase["MEMORY.md"] = "hermes:MEMORY.md"
	}
	var learnedLinks []GraphLink
	if raw, err := os.ReadFile(memPath); err == nil {
		for i, entry := range memoryEntries(string(raw)) {
			if i >= 40 {
				break
			}
			id := fmt.Sprintf("learned:%d", i)
			nodes = append(nodes, GraphNode{
				ID: id, Label: entryLabel(entry), Group: "learned", Size: int64(len(entry)),
			})
			learnedLinks = append(learnedLinks, GraphLink{Source: "hermes:MEMORY.md", Target: id})
		}
	}
	// Agent-grown skills orbit the graph too: they are the other visible
	// artifact of the agent teaching itself.
	skillsRep := SkillsReportNow()
	agentSkillCount := 0
	for _, cat := range skillsRep.Categories {
		for _, s := range cat.Skills {
			if s.Origin != "agent" {
				continue
			}
			if agentSkillCount >= 20 {
				break
			}
			id := "skill:" + s.Dir
			nodes = append(nodes, GraphNode{ID: id, Label: s.Name, Group: "skill", Size: 400})
			learnedLinks = append(learnedLinks, GraphLink{Source: "hermes:MEMORY.md", Target: id})
			agentSkillCount++
		}
	}

	var links []GraphLink
	seen := map[string]bool{}
	addLink := func(a, b string) {
		if a == b {
			return
		}
		key := a + "->" + b
		if !seen[key] {
			seen[key] = true
			links = append(links, GraphLink{Source: a, Target: b})
		}
	}
	for _, n := range nodes {
		var content []byte
		if n.ID == "hermes:MEMORY.md" {
			content, _ = os.ReadFile(memPath)
		} else {
			content, _ = ReadVaultFile(n.ID)
		}
		if len(content) == 0 || len(content) > 512*1024 {
			continue
		}
		text := string(content)
		for base, id := range byBase {
			if id == n.ID {
				continue
			}
			if strings.Contains(text, base) {
				addLink(n.ID, id)
			}
		}
	}
	g.Nodes, g.Links = nodes, append(links, learnedLinks...)
	return g
}

// buildHeatmap counts activity per day over the trailing 26 weeks: journal
// files by name date, sessions by started_at.
func buildHeatmap(sessions []SessionEntry) []HeatmapDay {
	counts := map[string]int{}
	if entries, err := os.ReadDir(filepath.Join(VaultDir(), "journal")); err == nil {
		for _, e := range entries {
			name := strings.TrimSuffix(e.Name(), ".md")
			if _, err := time.Parse("2006-01-02", name); err == nil {
				counts[name]++
			}
		}
	}
	for _, s := range sessions {
		if t, err := time.Parse(time.RFC3339, s.StartedAt); err == nil {
			counts[t.Format("2006-01-02")]++
		}
	}
	days := 26 * 7
	out := make([]HeatmapDay, 0, days)
	today := time.Now()
	for i := days - 1; i >= 0; i-- {
		d := today.AddDate(0, 0, -i).Format("2006-01-02")
		out = append(out, HeatmapDay{Date: d, Count: counts[d]})
	}
	return out
}

// hermesSessionHome mirrors Hermes's profile selection for the session store.
// A concrete profile HERMES_HOME wins; a root follows its sticky active profile.
func hermesSessionHome() string {
	root := strings.TrimSpace(os.Getenv("HERMES_HOME"))
	if root == "" {
		root = filepath.Join(home(), ".hermes")
	} else if strings.HasPrefix(root, "~/") {
		root = filepath.Join(home(), root[2:])
	}
	root = filepath.Clean(root)
	if filepath.Base(filepath.Dir(root)) == "profiles" {
		return root
	}
	active, err := os.ReadFile(filepath.Join(root, "active_profile"))
	if err != nil {
		return root
	}
	name := strings.TrimSpace(string(active))
	if name == "" || name == "default" || !validHermesProfileName(name) {
		return root
	}
	profile := filepath.Join(root, "profiles", name)
	if dirExists(profile) {
		return profile
	}
	return root
}

func validHermesProfileName(name string) bool {
	if len(name) == 0 || len(name) > 64 {
		return false
	}
	for i, r := range name {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || (i > 0 && (r == '_' || r == '-')) {
			continue
		}
		return false
	}
	return true
}

func hermesStateQuery(query string) ([]byte, string) {
	db := filepath.Join(hermesSessionHome(), "state.db")
	if !fileExists(db) {
		return nil, ""
	}
	bin, err := exec.LookPath("sqlite3")
	if err != nil {
		return nil, "session metadata unavailable: sqlite3 is not installed"
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	out, err := exec.CommandContext(ctx, bin, "-readonly", "-json", "file:"+db+"?mode=ro", query).Output()
	if err != nil || len(out) == 0 {
		return nil, ""
	}
	return out, ""
}

// hermesHistorySessions reads the same ACP rows that can be reopened, using
// the dedicated cwd column preferred by current Hermes and the legacy
// model_config value as a fallback.
func hermesHistorySessions(limit int) []SessionMeta {
	if limit <= 0 {
		return nil
	}
	q := "SELECT id, COALESCE(title,'') AS title, " +
		"COALESCE(NULLIF(cwd,''), CASE WHEN json_valid(model_config) THEN json_extract(model_config,'$.cwd') END, '') AS cwd, " +
		"COALESCE(last_activity_at, ended_at, started_at) AS updated_at " +
		"FROM sessions WHERE source = 'acp' AND archived IS NOT 1 AND message_count > 0 " +
		"ORDER BY updated_at DESC LIMIT " + itoa(limit)
	out, _ := hermesStateQuery(q)
	var rows []struct {
		ID        string  `json:"id"`
		Title     string  `json:"title"`
		Cwd       string  `json:"cwd"`
		UpdatedAt float64 `json:"updated_at"`
	}
	if json.Unmarshal(out, &rows) != nil {
		return nil
	}
	sessions := make([]SessionMeta, 0, len(rows))
	for _, r := range rows {
		updated := ""
		if r.UpdatedAt > 0 {
			updated = time.Unix(int64(r.UpdatedAt), 0).Format(time.RFC3339)
		}
		sessions = append(sessions, SessionMeta{
			ID: r.ID, Title: cleanTitle(r.Title), Cwd: r.Cwd, UpdatedAt: updated,
		})
	}
	return sessions
}

// hermesSessions reads session metadata from Hermes's active state.db without
// disturbing its WAL handle.
func hermesSessions(limit int) []SessionEntry {
	entries, _ := hermesSessionReport(limit)
	return entries
}

func hermesSessionReport(limit int) ([]SessionEntry, string) {
	q := "SELECT id, COALESCE(title,'') AS title, COALESCE(model,'') AS model, message_count, started_at, source FROM sessions WHERE archived IS NOT 1 ORDER BY started_at DESC LIMIT " + itoa(limit)
	out, note := hermesStateQuery(q)
	if note != "" || len(out) == 0 {
		return nil, note
	}
	var rows []struct {
		ID       string  `json:"id"`
		Title    string  `json:"title"`
		Model    string  `json:"model"`
		Messages int     `json:"message_count"`
		Started  float64 `json:"started_at"`
		Source   string  `json:"source"`
	}
	if json.Unmarshal(out, &rows) != nil {
		return nil, ""
	}
	entries := make([]SessionEntry, 0, len(rows))
	for _, r := range rows {
		entries = append(entries, SessionEntry{
			ID: r.ID, Title: r.Title, Model: r.Model, Messages: r.Messages,
			StartedAt: time.Unix(int64(r.Started), 0).Format(time.RFC3339),
			Source:    r.Source,
		})
	}
	return entries, ""
}

func MemoryReportNow() MemoryReport {
	var rep MemoryReport
	rep.Provider.Kind, rep.Provider.ObsidianVault = memoryProviderKind()
	if fi, err := os.Stat(hermesMemory()); err == nil {
		rep.Files.MemoryMd, rep.Files.MemoryBytes = true, fi.Size()
	}
	userMd := filepath.Join(home(), ".hermes", "memories", "USER.md")
	rep.Files.UserMd = fileExists(userMd)

	// The growth ledger: curated memory entries, user-profile facts, skills
	// the agent authored, and its own notes in the vault.
	if raw, err := os.ReadFile(hermesMemory()); err == nil {
		entries := memoryEntries(string(raw))
		rep.Learned.MemoryEntries = len(entries)
		for i, e := range entries {
			if i >= 12 {
				break
			}
			rep.Learned.Preview = append(rep.Learned.Preview, entryLabel(e))
		}
	}
	if raw, err := os.ReadFile(userMd); err == nil {
		rep.Learned.UserFacts = len(memoryEntries(string(raw)))
	}
	rep.Learned.AgentSkills = SkillsReportNow().Counts["agent"]
	if entries, err := os.ReadDir(filepath.Join(VaultDir(), "memory")); err == nil {
		for _, e := range entries {
			if !e.IsDir() {
				rep.Learned.VaultNotes++
			}
		}
	}
	if entries, err := os.ReadDir(filepath.Join(VaultDir(), "journal")); err == nil {
		for _, e := range entries {
			if !e.IsDir() {
				rep.Learned.JournalDays++
			}
		}
	}

	rep.Sessions, rep.SessionsNote = hermesSessionReport(50)
	rep.Graph = buildMemoryGraph()
	rep.Heatmap = buildHeatmap(rep.Sessions)
	return rep
}

// ---- about -----------------------------------------------------------------

type AboutReport struct {
	Version string `json:"version"`
	Port    int    `json:"port"`
	Vault   string `json:"vault"`
	Hermes  struct {
		Installed  bool   `json:"installed"`
		Version    string `json:"version"`
		Configured bool   `json:"configured"`
		Provider   string `json:"provider"`
		Model      string `json:"model"`
	} `json:"hermes"`
	Prowl struct {
		Installed bool `json:"installed"`
	} `json:"prowl"`
}

func AboutReportNow(cfg Config) AboutReport {
	var rep AboutReport
	rep.Version = rashinVersion
	rep.Port = cfg.Port
	rep.Vault = VaultDir()
	hs := HermesStatus()
	rep.Hermes.Installed = hs.Installed
	rep.Hermes.Version = hs.Version
	rep.Hermes.Configured = hs.Configured
	rep.Hermes.Provider, rep.Hermes.Model, _ = hermesModel()
	_, rep.Prowl.Installed = findProwl()
	return rep
}

func fileExists(p string) bool {
	fi, err := os.Stat(p)
	return err == nil && !fi.IsDir()
}

func itoa(n int) string {
	if n <= 0 {
		return "50"
	}
	b := [20]byte{}
	i := len(b)
	for n > 0 {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
	}
	return string(b[i:])
}
