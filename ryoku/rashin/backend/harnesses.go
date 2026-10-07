package main

// harnesses.go is the cross-harness agent-OS scan behind GET /api/harnesses.
// Hermes has first-class surfaces (skills, memory, sessions); every other
// installed harness keeps its own memories, skills, sessions, and model
// choice in its own directory layout. This file walks each known layout
// read-only and reports what a user should see: counts, names, the active
// model, and where credentials exist (variable names and key presence
// only, never values). Nothing here mutates a harness or reads a secret.

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

type HarnessRouting struct {
	Supported bool   `json:"supported"`
	Injected  bool   `json:"injected"`
	Connected bool   `json:"connected"`
	Active    bool   `json:"active"`
	Pending   bool   `json:"pending"`
	Note      string `json:"note,omitempty"`
}

// Harness is one detected coding agent and everything rashin can see about it.
type Harness struct {
	ID         string         `json:"id"`
	Name       string         `json:"name"`
	Present    bool           `json:"present"`
	Home       string         `json:"home,omitempty"`
	Version    string         `json:"version,omitempty"`
	Model      string         `json:"model,omitempty"`
	Provider   string         `json:"provider,omitempty"`
	Wired      bool           `json:"wired"`
	SkillCount int            `json:"skillCount"`
	Skills     []Skill        `json:"skills,omitempty"`
	Memories   []MemoryNote   `json:"memories"`
	SessionsN  int            `json:"sessions"`
	LastActive string         `json:"lastActive,omitempty"`
	Creds      []CredSource   `json:"creds"`
	Note       string         `json:"note,omitempty"`
	Routing    HarnessRouting `json:"routing"`
}

// MemoryNote is one durable-memory artifact a harness owns.
type MemoryNote struct {
	Name     string    `json:"name"`
	Path     string    `json:"path"`
	Kind     string    `json:"kind"` // file | dir | db
	Bytes    int64     `json:"bytes,omitempty"`
	Entries  int       `json:"entries,omitempty"`
	Modified time.Time `json:"modified,omitempty"`
}

// CredSource records that credentials exist for a harness: the file or
// variable name only. Values are never read or sent.
type CredSource struct {
	Kind  string `json:"kind"` // env | file
	Label string `json:"label"`
}

// harnessDef is the scan layout for one known agent.
type harnessDef struct {
	id   string
	name string
	home func() string
	bin  string // CLI used for --version
}

func harnessDefs() []harnessDef {
	return []harnessDef{
		{id: "hermes", name: "Hermes", home: func() string { return filepath.Join(home(), ".hermes") }, bin: "hermes"},
		{id: "claude", name: "Claude Code", home: func() string { return filepath.Join(home(), ".claude") }, bin: "claude"},
		{id: "codex", name: "Codex CLI", home: func() string { return filepath.Join(home(), ".codex") }, bin: "codex"},
		{id: "opencode", name: "opencode", home: func() string { return filepath.Join(configHome(), "opencode") }, bin: "opencode"},
		{id: "omp", name: "Oh My Pi", home: func() string { return filepath.Join(home(), ".omp", "agent") }, bin: "omp"},
		{id: "gemini", name: "Gemini CLI", home: func() string { return filepath.Join(home(), ".gemini") }, bin: "gemini"},
	}
}

// ScanHarnesses reports every known harness with live counts. Presence is by
// home directory, matching DetectAgents, so the two views never disagree.
func ScanHarnesses() []Harness {
	defs := harnessDefs()
	out := make([]Harness, 0, len(defs))
	var mu sync.Mutex
	var wg sync.WaitGroup
	for _, d := range defs {
		wg.Add(1)
		go func(h harnessDef) {
			defer wg.Done()
			row := scanHarness(h)
			mu.Lock()
			out = append(out, row)
			mu.Unlock()
		}(d)
	}
	wg.Wait()
	sort.Slice(out, func(i, j int) bool {
		if out[i].Present != out[j].Present {
			return out[i].Present
		}
		return out[i].ID < out[j].ID
	})
	return out
}

func scanHarness(d harnessDef) Harness {
	h := Harness{ID: d.id, Name: d.name}
	dir := d.home()
	h.Home = tildeAbbrev(dir)
	if !dirExists(dir) {
		return h
	}
	h.Present = true
	if wired, _ := wireStateFor(d.id); wired {
		h.Wired = true
	}
	h.Version = harnessVersion(d.bin)
	h.Skills, h.SkillCount = scanHarnessSkills(d.id, dir)
	h.Memories = scanHarnessMemories(d.id, dir)
	h.SessionsN, h.LastActive = scanHarnessSessions(d.id, dir)
	h.Model, h.Provider = harnessModelChoice(d.id, dir)
	h.Creds = scanHarnessCreds(d.id, dir)
	return h
}

// wireStateFor answers the same wiring question agents.go answers, without a
// second copy of the rule: it is wired when our pointer block or skill link
// is in place.
func wireStateFor(id string) (bool, bool) {
	for _, a := range DetectAgents() {
		if a.ID == id {
			return a.Wired, a.SkillWired
		}
	}
	if id == "hermes" {
		return fileHasBlock(hermesMemory()), agentSkillWiredAt(filepath.Join(home(), ".hermes", "skills", "ryoku"))
	}
	return false, false
}

// harnessVersion caches `bin --version` per binary; a missing binary is "" once
// and forever within the TTL.
var versionCache struct {
	mu  sync.Mutex
	at  map[string]time.Time
	val map[string]string
}

func harnessVersion(bin string) string {
	if bin == "" {
		return ""
	}
	versionCache.mu.Lock()
	if versionCache.at == nil {
		versionCache.at = map[string]time.Time{}
		versionCache.val = map[string]string{}
	}
	if t, ok := versionCache.at[bin]; ok && time.Since(t) < harnessTTL {
		v := versionCache.val[bin]
		versionCache.mu.Unlock()
		return v
	}
	versionCache.mu.Unlock()
	v := ""
	if out, err := sysProbe(6, bin, "--version"); err == nil {
		v = strings.TrimSpace(firstLine(string(out)))
		if len(v) > 60 {
			v = v[:60]
		}
	}
	versionCache.mu.Lock()
	versionCache.at[bin], versionCache.val[bin] = time.Now(), v
	versionCache.mu.Unlock()
	return v
}

// ---- skills -----------------------------------------------------------------

// skillDirs is where each harness keeps skills, by convention on disk.
func skillDirsFor(id, dir string) []struct{ path, origin string } {
	type sd = struct{ path, origin string }
	var out []sd
	switch id {
	case "hermes":
		out = append(out, sd{filepath.Join(dir, "skills"), ""})
	case "claude":
		out = append(out, sd{filepath.Join(dir, "skills"), "agent"}, sd{filepath.Join(dir, "plugins"), "plugin"})
	case "codex":
		out = append(out, sd{filepath.Join(dir, "skills"), "agent"})
	case "opencode":
		out = append(out, sd{filepath.Join(dir, "skills"), "agent"})
	case "omp":
		out = append(out, sd{filepath.Join(dir, "skills"), "agent"}, sd{filepath.Join(dir, "managed-skills"), "managed"})
	case "gemini":
		out = append(out, sd{filepath.Join(dir, "extensions"), "extension"})
	}
	var keep []sd
	for _, s := range out {
		if dirExists(s.path) {
			keep = append(keep, s)
		}
	}
	return keep
}

func scanHarnessSkills(id, dir string) ([]Skill, int) {
	bundled := map[string]bool{}
	hub := map[string]bool{}
	if id == "hermes" {
		bundled = bundledSkillNames()
		hub = hubSkillNames()
	}
	var skills []Skill
	for _, sd := range skillDirsFor(id, dir) {
		entries, err := os.ReadDir(sd.path)
		if err != nil {
			continue
		}
		for _, e := range entries {
			name := e.Name()
			if strings.HasPrefix(name, ".") {
				continue
			}
			skillDir := filepath.Join(sd.path, name)
			// skill dirs are often symlinks (wire lays them with ln -s), and
			// DirEntry.IsDir is false for a link, so follow it.
			if st, err := os.Stat(skillDir); err != nil || !st.IsDir() {
				continue
			}
			s := Skill{Name: name, Dir: tildeAbbrev(skillDir)}
			if fm, err := os.ReadFile(filepath.Join(skillDir, "SKILL.md")); err == nil {
				n, desc, ver := parseFrontmatter(string(fm))
				if n != "" {
					s.Name = n
				}
				s.Description = desc
				s.Version = ver
			}
			switch {
			case id == "hermes" && bundled[name]:
				s.Origin = "bundled"
			case id == "hermes" && hub[name]:
				s.Origin = "hub"
			case name == "ryoku":
				s.Origin = "shipped"
			case sd.origin != "":
				s.Origin = sd.origin
			default:
				s.Origin = "agent"
			}
			skills = append(skills, s)
		}
	}
	sort.Slice(skills, func(i, j int) bool {
		if skills[i].Origin != skills[j].Origin {
			return skills[i].Origin < skills[j].Origin
		}
		return skills[i].Name < skills[j].Name
	})
	return skills, len(skills)
}

// ---- memories -----------------------------------------------------------------

func scanHarnessMemories(id, dir string) []MemoryNote {
	var notes []MemoryNote
	file := func(label, p string) {
		if st, err := os.Stat(p); err == nil && !st.IsDir() {
			notes = append(notes, MemoryNote{Name: label, Path: tildeAbbrev(p), Kind: "file",
				Bytes: st.Size(), Modified: st.ModTime()})
		}
	}
	countDir := func(label, p, match string) {
		ents, err := os.ReadDir(p)
		if err != nil {
			return
		}
		n := 0
		var newest time.Time
		for _, e := range ents {
			if strings.HasPrefix(e.Name(), ".") {
				continue
			}
			if match != "" && !strings.HasSuffix(e.Name(), match) {
				continue
			}
			n++
			if fi, err := e.Info(); err == nil && fi.ModTime().After(newest) {
				newest = fi.ModTime()
			}
		}
		if n > 0 {
			notes = append(notes, MemoryNote{Name: label, Path: tildeAbbrev(p), Kind: "dir",
				Entries: n, Modified: newest})
		}
	}
	db := func(label, p string) {
		if st, err := os.Stat(p); err == nil {
			notes = append(notes, MemoryNote{Name: label, Path: tildeAbbrev(p), Kind: "db",
				Bytes: st.Size(), Modified: st.ModTime()})
		}
	}
	switch id {
	case "hermes":
		file("MEMORY.md", filepath.Join(dir, "memories", "MEMORY.md"))
		file("USER.md", filepath.Join(dir, "memories", "USER.md"))
		countDir("memories", filepath.Join(dir, "memories"), ".md")
		db("state.db", filepath.Join(dir, "state.db"))
	case "claude":
		file("CLAUDE.md", filepath.Join(dir, "CLAUDE.md"))
		countDir("history", filepath.Join(dir, "history"), ".jsonl")
	case "codex":
		file("AGENTS.md", filepath.Join(dir, "AGENTS.md"))
		db("memories", filepath.Join(dir, "memories_1.sqlite"))
	case "opencode":
		file("AGENTS.md", filepath.Join(dir, "AGENTS.md"))
	case "omp":
		file("AGENTS.md", filepath.Join(dir, "AGENTS.md"))
		countDir("memories", filepath.Join(dir, "memories"), "")
		db("agent.db", filepath.Join(dir, "agent.db"))
	case "gemini":
		file("settings", filepath.Join(dir, "settings.json"))
	}
	return notes
}

// ---- sessions -----------------------------------------------------------------

func scanHarnessSessions(id, dir string) (int, string) {
	countJSONL := func(root string, depth int) (int, time.Time) {
		total := 0
		var newest time.Time
		filepath.Walk(root, func(p string, fi os.FileInfo, err error) error {
			if err != nil || fi.IsDir() {
				if err == nil && fi.IsDir() && strings.Count(strings.TrimPrefix(p, root), string(os.PathSeparator)) > depth {
					return filepath.SkipDir
				}
				return nil
			}
			if strings.HasSuffix(p, ".jsonl") {
				total++
				if fi.ModTime().After(newest) {
					newest = fi.ModTime()
				}
			}
			return nil
		})
		return total, newest
	}
	var n int
	var newest time.Time
	switch id {
	case "hermes":
		if sess := hermesSessions(500); len(sess) > 0 {
			n = len(sess)
		} else if ents, err := os.ReadDir(filepath.Join(dir, "sessions")); err == nil {
			n = len(ents)
		}
		for _, e := range mustReadDir(filepath.Join(dir, "sessions")) {
			if fi, err := e.Info(); err == nil && fi.ModTime().After(newest) {
				newest = fi.ModTime()
			}
		}
	case "claude":
		n, newest = countJSONL(filepath.Join(dir, "projects"), 2)
	case "codex":
		n, newest = countJSONL(filepath.Join(dir, "sessions"), 4)
	case "omp":
		n, newest = countJSONL(filepath.Join(dir, "sessions"), 3)
	case "gemini":
		n, newest = countJSONL(filepath.Join(dir, "tmp"), 4)
	case "opencode":
		n, newest = countJSONL(filepath.Join(dataHome(), "opencode", "storage"), 4)
	}
	if newest.IsZero() {
		return n, ""
	}
	return n, newest.Format(time.RFC3339)
}

func mustReadDir(p string) []os.DirEntry {
	ents, _ := os.ReadDir(p)
	return ents
}

// ---- model choice ----------------------------------------------------------------

func harnessModelChoice(id, dir string) (model, provider string) {
	switch id {
	case "hermes":
		p, m, ok := hermesModel()
		if ok {
			return m, p
		}
	case "codex":
		b, err := os.ReadFile(filepath.Join(dir, "config.toml"))
		if err == nil {
			for _, line := range strings.Split(string(b), "\n") {
				t := strings.TrimSpace(line)
				if v, ok := strings.CutPrefix(t, "model = "); ok && model == "" {
					model = strings.Trim(v, `"`)
				}
			}
		}
		return model, "openai"
	case "claude":
		var doc struct {
			Model string `json:"model"`
		}
		if b, err := os.ReadFile(filepath.Join(dir, "settings.json")); err == nil && json.Unmarshal(b, &doc) == nil {
			return doc.Model, "anthropic"
		}
	case "omp":
		b, err := os.ReadFile(filepath.Join(dir, "models.yml"))
		if err == nil {
			for _, line := range strings.Split(string(b), "\n") {
				t := strings.TrimSpace(line)
				if v, ok := strings.CutPrefix(t, "model:"); ok && model == "" {
					model = strings.Trim(strings.TrimSpace(v), `"'`)
				}
				if v, ok := strings.CutPrefix(t, "provider:"); ok && provider == "" {
					provider = strings.Trim(strings.TrimSpace(v), `"'`)
				}
			}
		}
	case "gemini":
		var doc struct {
			Model struct {
				Name string `json:"name"`
			} `json:"model"`
		}
		if b, err := os.ReadFile(filepath.Join(dir, "settings.json")); err == nil && json.Unmarshal(b, &doc) == nil {
			return doc.Model.Name, "google"
		}
	}
	return "", ""
}

// ---- credential presence (names only) ------------------------------------------------

func scanHarnessCreds(id, dir string) []CredSource {
	var out []CredSource
	// File-based credential stores: report the file and the variable names it
	// holds; values are never read.
	keyNames := func(p string) []string {
		b, err := os.ReadFile(p)
		if err != nil {
			return nil
		}
		var names []string
		if filepath.Ext(p) == ".json" {
			var doc map[string]json.RawMessage
			if json.Unmarshal(b, &doc) == nil {
				for k := range doc {
					if isCredKey(k) {
						names = append(names, k)
					}
				}
			}
		} else {
			for _, line := range strings.Split(string(b), "\n") {
				t := strings.TrimSpace(line)
				if strings.HasPrefix(t, "#") {
					continue
				}
				if k, _, ok := strings.Cut(t, "="); ok && isCredKey(strings.TrimSpace(k)) {
					names = append(names, strings.TrimSpace(k))
				}
			}
		}
		sort.Strings(names)
		return names
	}
	add := func(p string) {
		for _, k := range keyNames(p) {
			out = append(out, CredSource{Kind: "file", Label: k})
		}
	}
	switch id {
	case "hermes":
		add(filepath.Join(dir, ".env"))
		add(filepath.Join(dir, "auth.json"))
	case "claude":
		add(filepath.Join(dir, ".credentials.json"))
	case "codex":
		add(filepath.Join(dir, "auth.json"))
	case "gemini":
		add(filepath.Join(dir, "oauth_creds.json"))
	}
	// process environment: a provider key exported for any tool counts for
	// whoever uses it
	for _, kv := range os.Environ() {
		k, _, _ := strings.Cut(kv, "=")
		if isCredKey(k) {
			out = append(out, CredSource{Kind: "env", Label: k})
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Kind != out[j].Kind {
			return out[i].Kind < out[j].Kind
		}
		return out[i].Label < out[j].Label
	})
	if len(out) > 24 {
		out = out[:24]
	}
	return dedupeCreds(out)
}

// isCredKey names a provider credential, not any secret-ish variable:
// KITTY_PUBLIC_KEY or STARSHIP_SESSION_KEY are not model access and would
// only add noise to the ledger.
func isCredKey(k string) bool {
	u := strings.ToUpper(k)
	if strings.Contains(u, "API_KEY") || strings.HasSuffix(u, "APIKEY") ||
		strings.Contains(u, "CREDENTIAL") || u == "AUTH_MODE" {
		return true
	}
	return strings.HasSuffix(u, "_ACCESS_TOKEN") || strings.HasSuffix(u, "_REFRESH_TOKEN") ||
		strings.HasSuffix(u, "_PAT") || u == "OPENAI_API_KEY"
}

func dedupeCreds(in []CredSource) []CredSource {
	seen := map[string]bool{}
	var out []CredSource
	for _, c := range in {
		if seen[c.Kind+c.Label] {
			continue
		}
		seen[c.Kind+c.Label] = true
		out = append(out, c)
	}
	return out
}

// agentSkillWiredAt checks a concrete link path for our shipped skill.
func agentSkillWiredAt(link string) bool {
	return isOurSkillLink(link)
}

// ---- caching ---------------------------------------------------------------------

const harnessTTL = 60 * time.Second

var harnessCache struct {
	mu   sync.Mutex
	at   time.Time
	data []Harness
}

// HarnessesNow returns the cached scan; refresh runs inline on TTL expiry so
// dashboard polling execs nothing more than once a minute.
func HarnessesNow() []Harness {
	harnessCache.mu.Lock()
	defer harnessCache.mu.Unlock()
	if harnessCache.data == nil || time.Since(harnessCache.at) >= harnessTTL {
		harnessCache.data = ScanHarnesses()
		harnessCache.at = time.Now()
	}
	return append([]Harness(nil), harnessCache.data...)
}

func invalidateHarnessCache() {
	harnessCache.mu.Lock()
	harnessCache.data = nil
	harnessCache.at = time.Time{}
	harnessCache.mu.Unlock()
}

const geminiRoutingReason = "Gemini CLI has no OpenAI- or Anthropic-compatible endpoint, so it cannot route through Prowl"

func HarnessesWithRouting(ctx context.Context) []Harness {
	rows := HarnessesNow()
	setup, err := gatewayRouting(ctx)
	byID := make(map[string]prowlSetupHarness, len(setup.Harnesses))
	for _, row := range setup.Harnesses {
		byID[row.ID] = row
	}
	cfg := LoadConfig()
	chatID := activeChatAgentID(cfg)
	for i := range rows {
		rows[i].Routing.Connected = cfg.HasProwlHarness(rows[i].ID) || rows[i].ID == chatID
		rows[i].Routing.Pending = rows[i].Routing.Connected
		if rows[i].ID == "gemini" {
			rows[i].Routing.Note = geminiRoutingReason
			continue
		}
		rows[i].Routing.Supported = true
		if err != nil {
			rows[i].Routing.Note = quickGatewayError(err).Error()
			continue
		}
		remote, ok := byID[rows[i].ID]
		if !ok {
			rows[i].Routing.Supported = false
			rows[i].Routing.Note = "Prowl does not support this harness"
			continue
		}
		rows[i].Routing.Injected = remote.Injected
		rows[i].Routing.Active = remote.Active
		rows[i].Routing.Pending = rows[i].Routing.Connected && !remote.Active
		rows[i].Routing.Note = remote.Note
		if rows[i].Routing.Pending && rows[i].Routing.Note == "" {
			rows[i].Routing.Note = setup.Reason
		}
	}
	return rows
}

func chatHarnessRouting(ctx context.Context, id string) chatAgentRouting {
	setup, err := gatewayRouting(ctx)
	if err != nil {
		return chatAgentRouting{}
	}
	for _, row := range setup.Harnesses {
		if row.ID != id {
			continue
		}
		if row.Active {
			return chatAgentRouting{Active: true}
		}
		if !row.Injected {
			return chatAgentRouting{}
		}
		reason := row.Note
		if reason == "" {
			reason = setup.Reason
		}
		if reason == "" {
			reason = "Prowl has no provider connected; open Prowl > Providers in Rashin"
		}
		return chatAgentRouting{Pending: true, Reason: reason}
	}
	return chatAgentRouting{}
}

type harnessRoutePending struct {
	ID     string `json:"id"`
	Reason string `json:"reason"`
}

type harnessRouteResult struct {
	Routed  []string              `json:"routed"`
	Pending []harnessRoutePending `json:"pending"`
}

func routeConnectedHarnesses(ctx context.Context) harnessRouteResult {
	result := harnessRouteResult{Routed: []string{}, Pending: []harnessRoutePending{}}
	setup, err := gatewayRouting(ctx)
	if err != nil {
		return result
	}
	remote := make(map[string]prowlSetupHarness, len(setup.Harnesses))
	for _, row := range setup.Harnesses {
		remote[row.ID] = row
	}
	cfg := LoadConfig()
	chatID := activeChatAgentID(cfg)
	route := func(id string) {
		if id == "" || id == "gemini" {
			return
		}
		prowlRow, ok := remote[id]
		if !ok || prowlRow.Active {
			return
		}
		pending, reason, routeErr := routeProwlHarness(ctx, id)
		if routeErr != nil {
			result.Pending = append(result.Pending, harnessRoutePending{ID: id, Reason: routeErr.Error()})
			return
		}
		if id == chatID && cfg.AddProwlHarness(id) {
			if saveErr := SaveConfig(cfg); saveErr != nil {
				result.Pending = append(result.Pending, harnessRoutePending{ID: id, Reason: saveErr.Error()})
				return
			}
		}
		if pending {
			result.Pending = append(result.Pending, harnessRoutePending{ID: id, Reason: reason})
		} else {
			result.Routed = append(result.Routed, id)
		}
	}
	for _, id := range cfg.ProwlHarnesses {
		route(id)
	}
	if !cfg.HasProwlHarness(chatID) {
		route(chatID)
	}
	for _, id := range result.Routed {
		if chatAgentInUse(id, cfg) {
			resetChatLanes(cfg)
			break
		}
	}
	return result
}

func routeProwlHarness(ctx context.Context, id string) (bool, string, error) {
	var routed struct {
		Harness prowlSetupHarness `json:"harness"`
	}
	path := "/api/setup/harnesses/" + id
	err := prowlGatewayJSON(ctx, http.MethodPost, path, map[string]bool{"activate": true}, &routed)
	if err == nil {
		return false, "", nil
	}
	var gatewayErr *prowlGatewayError
	if !errors.As(err, &gatewayErr) || gatewayErr.Status != http.StatusConflict || gatewayErr.Code != "not_routable" {
		return false, "", err
	}
	if injectErr := prowlGatewayJSON(ctx, http.MethodPost, path, map[string]bool{"activate": false}, &routed); injectErr != nil {
		return false, "", injectErr
	}
	reason := gatewayErr.Message
	if reason == "" {
		reason = "Prowl has no provider connected; open Prowl > Providers in Rashin"
	}
	return true, reason, nil
}
