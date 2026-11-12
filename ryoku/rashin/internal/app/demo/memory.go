package demo

import (
	"encoding/json"
	"fmt"
	"reflect"
	"regexp"
	"slices"
	"sort"
	"strings"
	"time"
	"unicode/utf8"

	"ryoku-i18n"

	"ryoku-rashin/internal/protocol"
)

// the demo's memory scopes and the conversation stage tests read learning from
const (
	demoMemoryHyperion = protocol.MemoryProjectPrefix + "3f9c2a71b0d4"
	demoMemoryRyotunes = protocol.MemoryProjectPrefix + "8e1d05c7a9b2"
	demoMemoryRepo     = protocol.MemoryTeamPrefix + demoTeamRepo
	demoMemoryResearch = protocol.MemoryTeamPrefix + demoTeamResearch
	demoMemorySession  = "ask-20261110-094200-a17c"
)

type memoryFixtures struct {
	ready      bool
	memories   []protocol.Memory
	changes    []memoryChange
	told       []protocol.Event
	settings   protocol.MemorySettings
	cardBuilt  time.Time
	clock      int
	ids        int
	changeIDs  int
	learned    int
	forgot     int
	suppressed []protocol.MemorySuppressParams
}

// undo and redo put each touched memory back the way it was, absent when nil
type memoryChange struct {
	change protocol.MemoryChange
	snaps  []memorySnap
}

type memorySnap struct {
	id     string
	before *protocol.Memory
	after  *protocol.Memory
}

// the demo's chart of the last 30 days, oldest first, busiest on day 14
var (
	demoMemoryLearned = [30]int{0, 1, 0, 0, 2, 0, 1, 0, 0, 0, 1, 0, 0, 7, 1, 0, 0, 2, 0, 0, 1, 0, 3, 0, 0, 1, 0, 2, 0, 1}
	demoMemoryForgot  = [30]int{0, 0, 0, 1, 0, 0, 0, 0, 1, 0, 0, 0, 0, 0, 0, 1, 0, 0, 0, 0, 0, 1, 0, 0, 0, 0, 0, 0, 1, 0}
)

var (
	memorySecret = regexp.MustCompile(`(?i)\bsk-[a-z0-9_-]{8,}|\bghp_[a-z0-9]{10,}|\bgithub_pat_|\bAKIA[0-9A-Z]{12,}|\bpassword\b|\bpasswd\b|-----BEGIN`)
	memoryBullet = regexp.MustCompile(`^(?:[-*+•]|\d+[.)])\s*`)
)

// lazily, so the fixtures only pay for memory once something asks
func (s *server) memory() *memoryFixtures {
	m := &s.data.memory
	if !m.ready {
		*m = makeMemoryFixtures()
		m.settings = s.data.snapshot.Settings.Memory
	}
	return m
}

func makeMemoryFixtures() memoryFixtures {
	ago := func(d time.Duration) time.Time { return Epoch.Add(-d) }
	at := func(d time.Duration) *time.Time { moment := Epoch.Add(-d); return &moment }
	day := 24 * time.Hour
	buildTitle, niriTitle, keysTitle := i18n.T("Why did my build slow down?"), i18n.T("How do I switch to niri?"), i18n.T("Where are provider keys stored?")
	panelTitle := i18n.T("Add the settings panel")
	coverageTitle, duplicatesTitle := i18n.T("Find this week's coverage of Ryoku"), i18n.T("Group the duplicate install failure issues")
	morningTitle := demoTemplate("repo").Routines[0].Title
	ask := func(session, title string, turn int) protocol.MemorySource {
		return protocol.MemorySource{Kind: protocol.MemoryFromAsk, Session: session, Turn: turn, Title: title, Model: "llama.cpp"}
	}
	agent := protocol.MemorySource{Kind: protocol.MemoryFromAgent, Session: "agent-20261110-101500-b82d", Title: panelTitle, Model: "gpt-5.6"}
	you := protocol.MemorySource{Kind: protocol.MemoryFromYou}
	file := protocol.MemorySource{Kind: protocol.MemoryFromFile}
	chatgpt := protocol.MemorySource{Kind: protocol.MemoryFromImport, From: "ChatGPT"}
	team := func(id, task, title string) protocol.MemorySource {
		return protocol.MemorySource{Kind: protocol.MemoryFromTeam, Team: id, Task: task, Title: title}
	}
	inBuild := &protocol.MemoryRef{Session: demoMemorySession, Kind: protocol.KindAsk, Title: buildTitle}
	inNiri := &protocol.MemoryRef{Session: "ask-20261109-161200-prev", Kind: protocol.KindAsk, Title: niriTitle}
	inKeys := &protocol.MemoryRef{Session: "ask-20261107-112500-week", Kind: protocol.KindAsk, Title: keysTitle}
	inPanel := &protocol.MemoryRef{Session: "agent-20261110-101500-b82d", Kind: protocol.KindAgent, Title: panelTitle}
	fact := func(id, scope, text string, source protocol.MemorySource, age time.Duration, uses int, used *protocol.MemoryRef, usedAgo time.Duration) protocol.Memory {
		memory := protocol.Memory{
			ID: id, Scope: scope, ScopeName: memoryScopeName(scope), Text: text, Status: protocol.MemoryActive,
			Kind: protocol.MemoryDurable, Source: source, Created: ago(age), Updated: ago(age), Uses: uses,
		}
		if used != nil {
			memory.LastUsed, memory.LastUsedIn = at(usedAgo), used
		}
		return memory
	}
	pinned := func(memory protocol.Memory) protocol.Memory {
		memory.Pinned, memory.Kind = true, protocol.MemoryPermanent
		return memory
	}
	versioned := func(memory protocol.Memory, versions ...protocol.MemoryVersion) protocol.Memory {
		memory.Versions, memory.Updated = versions, memory.Created.Add(time.Hour)
		if len(versions) > 0 {
			memory.Created = versions[len(versions)-1].At
		}
		return memory
	}
	suggested := func(memory protocol.Memory) protocol.Memory {
		memory.Status, memory.Uses = protocol.MemorySuggested, 0
		return memory
	}
	replaced := func(memory protocol.Memory, when time.Duration, by, byText string) protocol.Memory {
		memory.Status, memory.Replaced, memory.ReplacedBy, memory.ReplacedByText = protocol.MemoryReplaced, at(when), by, byText
		return memory
	}
	youScope := protocol.MemoryYou
	kernel := fact("m2b9d3f0", youScope, i18n.T("Keeps the T14 on the 6.12 LTS kernel"), ask(demoMemorySession, buildTitle, 0), 6*time.Minute, 0, nil, 0)
	niri := versioned(fact("m4d1f6b8", youScope, i18n.T("Uses niri, not Hyprland, and reaches for the niri IPC before editing config by hand"), ask("ask-20261109-161200-prev", niriTitle, 1), 17*time.Hour, 6, inBuild, 12*time.Minute),
		protocol.MemoryVersion{Text: i18n.T("Uses niri on the laptop and Hyprland on the desktop"), At: ago(9 * day), By: "learning"})
	helix := fact("md2e9b6c", youScope, i18n.T("Uses Helix as their editor with gopls, and opens files from the terminal rather than a file manager"), ask("ask-20261107-112500-week", keysTitle, 1), 70*time.Hour, 4, inPanel, 2*time.Minute)
	memories := []protocol.Memory{
		fact("m1a4c7e2", youScope, i18n.T("Uses fish as their login shell"), ask(demoMemorySession, buildTitle, 0), 6*time.Minute, 0, nil, 0),
		kernel,
		fact("m3e8a1c5", youScope, i18n.T("Prefers answers with the command first"), ask(demoMemorySession, buildTitle, 1), 80*time.Minute, 2, inBuild, 12*time.Minute),
		niri,
		pinned(versioned(fact("m5a7b2d9", youScope, i18n.T("Writes Go most days and some QML for the desktop shell, and reads Go errors faster than prose"), agent, 2*day, 19, inPanel, 2*time.Minute),
			protocol.MemoryVersion{Text: i18n.T("Writes Go and some QML"), At: ago(5 * day), By: "you"},
			protocol.MemoryVersion{Text: i18n.T("Writes Go"), At: ago(26 * day), By: "learning"})),
		fact("m6c3e0f1", youScope, i18n.T("Has a ThinkPad T14 whose Wi-Fi drops after suspend; `sudo modprobe -r iwlwifi && sudo modprobe iwlwifi` brings it back"), you, 12*day, 5, inKeys, 70*time.Hour),
		fact("m7f2a8c3", youScope, i18n.T("Writes in British English and wants answers spelled the same way, colour and organise included"), chatgpt, 27*day, 11, inBuild, 12*time.Minute),
		fact("m8b4d6e0", youScope, i18n.T("Works mostly in the evenings after seven, so long builds and updates are best left for the morning"), ask("ask-20261107-112500-week", keysTitle, 1), 3*day, 3, inNiri, 17*time.Hour),
		pinned(fact("m9e1c5a7", youScope, i18n.T("Keeps the niri config in `~/.config/niri/config.kdl` and edits it by hand, so suggestions should show the exact lines to change"), file, 16*day, 8, inNiri, 17*time.Hour)),
		fact("ma3f7b1d", youScope, i18n.T("Uses Zen as their browser and keeps Firefox only for testing"), chatgpt, 27*day, 1, inKeys, 70*time.Hour),
		fact("mb6d2e8f", youScope, i18n.T("Runs Ryoku on Arch from the hyperion test line and updates with `ryoku update` rather than pacman"), agent, 9*day, 7, inPanel, 2*time.Minute),
		fact("mc8a4f2b", youScope, i18n.T("Keeps dotfiles in a bare git repository at `~/.dotfiles` and commits after every change that works"), you, 21*day, 0, nil, 0),
		helix,
		fact("me5c1a8d", youScope, i18n.T("Turns notification sounds off while working and keeps Do Not Disturb on in the evenings"), agent, 6*day, 2, inNiri, 17*time.Hour),
		fact("mf8e2d4a", youScope, i18n.T("Has a 4K monitor at 1.5 scale beside the laptop panel at 1.25, so sizes need checking on both"), ask("ask-20261109-161200-prev", niriTitle, 1), 17*time.Hour, 3, inBuild, 12*time.Minute),

		fact("mf1b7d3e", demoMemoryHyperion, i18n.T("Build Rashin with `-mod=vendor -tags nox11`"), agent, 4*day, 14, inPanel, 2*time.Minute),
		pinned(fact("m0a6c2f8", demoMemoryHyperion, i18n.T("The working branch is hyperion; nothing is pushed to main by hand"), you, 25*day, 9, inPanel, 2*time.Minute)),
		fact("m1c8e4b0", demoMemoryHyperion, i18n.T("Run tests with `-p 2` so the laptop stays usable"), agent, 8*day, 6, inPanel, 40*time.Minute),
		fact("m2d5f9a1", demoMemoryHyperion, i18n.T("Every user-visible string goes through `i18n.T` with a literal"), file, 19*day, 10, inPanel, 2*time.Minute),
		fact("m3f0b6c4", demoMemoryHyperion, i18n.T("Comments are one lowercase line that says why"), agent, 11*day, 4, inPanel, 3*time.Hour),
		fact("m4a2d8e7", demoMemoryHyperion, i18n.T("The design system is Lithic, set in Geologica and Martian Mono"), ask("ask-20261107-112500-week", keysTitle, 1), 3*day, 2, inPanel, 2*time.Hour),
		fact("m5b9e3f6", demoMemoryHyperion, i18n.T("Demo data lives in `internal/app/demo` and starts from a fixed clock"), protocol.MemorySource{Kind: protocol.MemoryFromAgent, Session: agent.Session, Turn: 1, Title: panelTitle, Model: agent.Model}, 50*time.Minute, 1, inPanel, 2*time.Minute),
		fact("m6e7a1c9", demoMemoryHyperion, i18n.T("Publish runs on every push to unstable-dev and takes about 20 minutes"), agent, 14*day, 0, nil, 0),

		fact("m7b3c8d2", demoMemoryRyotunes, i18n.T("Ryotunes is written in Go with Gio"), file, 29*day, 5, inKeys, 70*time.Hour),
		fact("m8d6f1a5", demoMemoryRyotunes, i18n.T("The volume slider must stay easy to drag precisely"), you, 13*day, 2, inKeys, 70*time.Hour),
		fact("m9a0e7b3", demoMemoryRyotunes, i18n.T("Releases are cut by a bot commit that bumps the version"), agent, 10*day, 1, inKeys, 70*time.Hour),
		fact("ma4c9d6e", demoMemoryRyotunes, i18n.T("YouTube sign-in must survive days offline"), you, 7*day, 0, nil, 0),
		fact("mb1f3e8a", demoMemoryRyotunes, i18n.T("Run `go run ./cmd/ryotunes --demo` to try changes without signing in"), agent, 5*day, 3, inKeys, 70*time.Hour),

		fact("mc5e2a9f", demoMemoryRepo, i18n.T("Install failure reports usually come from an out-of-date mirror list"), team(demoTeamRepo, "task-duplicates", duplicatesTitle), 4*day, 3, nil, 0),
		fact("md7b0c4e", demoMemoryRepo, i18n.T("Label nothing; the maintainer labels issues by hand"), team(demoTeamRepo, "task-review-morning", morningTitle), 15*day, 9, nil, 0),
		fact("me9d4f1a", demoMemoryRepo, i18n.T("The morning review covers pull requests opened since six the evening before"), team(demoTeamRepo, "task-review-morning", morningTitle), 15*day, 9, nil, 0),
		fact("mf3a6b2c", demoMemoryResearch, i18n.T("Coverage of Ryoku is mostly on Reddit and Phoronix"), team(demoTeamResearch, "task-coverage", coverageTitle), 6*day, 2, nil, 0),
		fact("m0c7d9e3", demoMemoryResearch, i18n.T("NEWS_API_KEY is in the vault and is sent only after asking"), team(demoTeamResearch, "task-coverage", coverageTitle), 6*day, 2, nil, 0),

		suggested(fact("m1e4a7b5", youScope, i18n.T("Reads Phoronix most mornings"), team(demoTeamResearch, "task-coverage", coverageTitle), 3*time.Hour, 0, nil, 0)),
		suggested(fact("m2f8c1d6", youScope, i18n.T("Follows the Ryoku subreddit and answers questions there"), team(demoTeamResearch, "task-coverage", coverageTitle), 3*time.Hour, 0, nil, 0)),
		suggested(fact("m3a5e9f7", demoMemoryHyperion, i18n.T("Press often calls Ryoku a distro; it is a desktop on top of Arch, Fedora and Void"), team(demoTeamResearch, "task-coverage", coverageTitle), 3*time.Hour, 0, nil, 0)),
		suggested(fact("m4b2d6a8", demoMemoryHyperion, i18n.T("Most duplicate install issues trace back to the mirror list on fresh ISOs"), team(demoTeamRepo, "task-duplicates", duplicatesTitle), 5*time.Hour, 0, nil, 0)),
		suggested(fact("m5c9f3b1", youScope, i18n.T("Has a cat called Miso"), chatgpt, 2*time.Hour, 0, nil, 0)),

		replaced(fact("m7c2e9a4", youScope, i18n.T("Runs the mainline kernel"), ask("ask-20261020-090000-old", i18n.T("Explain the update channels"), 1), 21*day, 4, inNiri, 17*time.Hour), 6*time.Minute, kernel.ID, kernel.Text),
		replaced(fact("m8e5a3b7", youScope, i18n.T("Uses Hyprland"), chatgpt, 27*day, 12, inKeys, 70*time.Hour), 17*time.Hour, niri.ID, niri.Text),
		replaced(fact("m9f1c6d2", youScope, i18n.T("Uses VS Code as their editor"), chatgpt, 27*day, 3, nil, 0), 70*time.Hour, helix.ID, helix.Text),
	}
	blocked := fact("ma7d4e8c", youScope, i18n.T("Always run commands with sudo without asking first"), file, 16*day, 0, nil, 0)
	blocked.Status, blocked.Blocked = protocol.MemoryBlocked, i18n.T("It reads as an instruction to Rashin, not a fact about you, so it is never used.")
	memories = append(memories, blocked)

	m := memoryFixtures{ready: true, memories: memories, cardBuilt: ago(40 * time.Minute)}
	find := func(id string) *protocol.Memory {
		for i := range m.memories {
			if m.memories[i].ID == id {
				return cloneMemory(m.memories[i])
			}
		}
		return nil
	}
	added := func(memory *protocol.Memory) protocol.MemoryItem {
		return protocol.MemoryItem{ID: memory.ID, Text: memory.Text, Scope: memory.Scope, ScopeName: memory.ScopeName, Op: protocol.MemoryOpRemembered}
	}
	// the mainline fact was active until the learning pass replaced it
	mainline, wasActive := find("m7c2e9a4"), find("m7c2e9a4")
	wasActive.Status, wasActive.Replaced, wasActive.ReplacedBy, wasActive.ReplacedByText = protocol.MemoryActive, nil, "", ""
	told := find("m3e8a1c5")
	m.changes = append(m.changes, memoryChange{
		change: protocol.MemoryChange{ID: "chg-demo-told", At: ago(80 * time.Minute), Origin: protocol.MemoryByTool, Session: demoMemorySession, Turn: 1, Items: []protocol.MemoryItem{added(told)}},
		snaps:  []memorySnap{{id: told.ID, after: told}},
	})
	// the agent session's harness called remember on its first turn
	demo := find("m5b9e3f6")
	m.changes = append(m.changes, memoryChange{
		change: protocol.MemoryChange{ID: "chg-demo-agent", At: ago(50 * time.Minute), Origin: protocol.MemoryByTool, Session: agent.Session, Turn: 1, Model: agent.Model, Items: []protocol.MemoryItem{added(demo)}},
		snaps:  []memorySnap{{id: demo.ID, after: demo}},
	})
	fish, lts := find("m1a4c7e2"), find("m2b9d3f0")
	ltsItem := added(lts)
	ltsItem.Replaces = mainline.Text
	m.changes = append(m.changes, memoryChange{
		change: protocol.MemoryChange{ID: "chg-demo-learned", At: ago(6 * time.Minute), Origin: protocol.MemoryByLearning, Session: demoMemorySession, Model: "llama.cpp", Items: []protocol.MemoryItem{added(fish), ltsItem}},
		snaps:  []memorySnap{{id: fish.ID, after: fish}, {id: lts.ID, after: lts}, {id: mainline.ID, before: wasActive, after: mainline}},
	})
	return m
}

func memoryScopeName(scope string) string {
	switch scope {
	case protocol.MemoryYou:
		return i18n.T("About you")
	case demoMemoryHyperion:
		return demoProjectHyperion
	case demoMemoryRyotunes:
		return demoProjectRyotunes
	case demoMemoryRepo:
		return i18n.T("Repo watch")
	case demoMemoryResearch:
		return i18n.T("Research desk")
	}
	return strings.TrimPrefix(strings.TrimPrefix(scope, protocol.MemoryProjectPrefix), protocol.MemoryTeamPrefix)
}

// a team made in the demo after the fixtures still shows by its name
func (s *server) memoryScopeName(scope string) string {
	if id, ok := strings.CutPrefix(scope, protocol.MemoryTeamPrefix); ok && scope != demoMemoryRepo && scope != demoMemoryResearch {
		if index := s.data.teams.teamIndex(id); index >= 0 {
			return s.data.teams.teams[index].Name
		}
	}
	return memoryScopeName(scope)
}

func cloneMemory(memory protocol.Memory) *protocol.Memory {
	memory.Versions = slices.Clone(memory.Versions)
	return &memory
}

func validMemoryScope(scope string) bool {
	if scope == protocol.MemoryYou {
		return true
	}
	for _, prefix := range []string{protocol.MemoryProjectPrefix, protocol.MemoryTeamPrefix} {
		if rest, ok := strings.CutPrefix(scope, prefix); ok && rest != "" {
			return true
		}
	}
	return false
}

func memoryInScope(memory protocol.Memory, scope string) bool {
	switch scope {
	case "":
		return true
	case protocol.MemoryProjects:
		return strings.HasPrefix(memory.Scope, protocol.MemoryProjectPrefix)
	case protocol.MemoryTeams:
		return strings.HasPrefix(memory.Scope, protocol.MemoryTeamPrefix)
	}
	return memory.Scope == scope
}

// a write is stamped just after the demo's clock so it sorts newest and renders the same every run
func (m *memoryFixtures) now() time.Time {
	m.clock++
	return Epoch.Add(time.Duration(m.clock) * time.Second)
}

func (m *memoryFixtures) newID() string {
	for {
		m.ids++
		id := fmt.Sprintf("m%07x", 0xd000000+m.ids)
		if m.index(id) < 0 {
			return id
		}
	}
}

func (m *memoryFixtures) index(id string) int {
	return slices.IndexFunc(m.memories, func(memory protocol.Memory) bool { return memory.ID == id })
}

func (m *memoryFixtures) put(id string, memory *protocol.Memory) {
	index := m.index(id)
	switch {
	case memory == nil && index >= 0:
		m.memories = slices.Delete(m.memories, index, index+1)
	case memory != nil && index >= 0:
		m.memories[index] = *cloneMemory(*memory)
	case memory != nil:
		m.memories = append(m.memories, *cloneMemory(*memory))
	}
}

func (m *memoryFixtures) byID() map[string]protocol.Memory {
	all := make(map[string]protocol.Memory, len(m.memories))
	for _, memory := range m.memories {
		all[memory.ID] = *cloneMemory(memory)
	}
	return all
}

func memoryItem(memory protocol.Memory, op string) protocol.MemoryItem {
	return protocol.MemoryItem{ID: memory.ID, Text: memory.Text, Scope: memory.Scope, ScopeName: memory.ScopeName, Op: op}
}

func memoryBadRequest(message string) error {
	return protocol.Errorf(protocol.CodeBadRequest, "%s", message)
}

func memoryNotFound() error {
	return protocol.Errorf(protocol.CodeNotFound, "%s", i18n.T("That memory is not there any more."))
}

// what the daemon refuses before anything is stored
func checkMemoryText(text string) (string, error) {
	text = strings.TrimSpace(text)
	switch {
	case text == "":
		return "", memoryBadRequest(i18n.T("Say what to remember."))
	case utf8.RuneCountInString(text) > protocol.MemoryTextMax:
		return "", memoryBadRequest(i18n.Tf("A memory can be at most %d characters.", protocol.MemoryTextMax))
	case memorySecret.MatchString(text):
		return "", memoryBadRequest(i18n.T("This looks like a secret, so Rashin won't keep it. Store it in the vault instead."))
	}
	return text, nil
}

func (s *server) tellMemory(change *protocol.MemoryChange) {
	m := s.memory()
	data := protocol.MemoryChanged{Status: *s.memoryStatus(), Change: change}
	if wire, err := json.Marshal(data); err == nil {
		m.told = append(m.told, protocol.Event{Event: protocol.EvtMemoryChanged, Data: wire})
	}
}

// a settings patch that moved memory is heard as memory.changed too, after the method's own events
func (s *server) takeMemoryEvents() []protocol.Event {
	m := s.memory()
	if m.settings != s.data.snapshot.Settings.Memory {
		m.settings = s.data.snapshot.Settings.Memory
		s.tellMemory(nil)
	}
	told := m.told
	m.told = nil
	return told
}

// every write diffs the whole store so undo restores exactly what it touched
func (s *server) commitMemory(change protocol.MemoryChange, before map[string]protocol.Memory) protocol.MemoryChange {
	m := s.memory()
	after := m.byID()
	var snaps []memorySnap
	for id, old := range before {
		current, ok := after[id]
		switch {
		case !ok:
			snaps = append(snaps, memorySnap{id: id, before: cloneMemory(old)})
		case !reflect.DeepEqual(old, current):
			snaps = append(snaps, memorySnap{id: id, before: cloneMemory(old), after: cloneMemory(current)})
		}
	}
	for id, current := range after {
		if _, ok := before[id]; !ok {
			snaps = append(snaps, memorySnap{id: id, after: cloneMemory(current)})
		}
	}
	sort.Slice(snaps, func(i, j int) bool { return snaps[i].id < snaps[j].id })
	m.changeIDs++
	change.ID = fmt.Sprintf("c%d", m.changeIDs)
	if change.Items == nil {
		change.Items = []protocol.MemoryItem{}
	}
	m.changes = append(m.changes, memoryChange{change: change, snaps: snaps})
	s.tellMemory(&change)
	return change
}

func (m *memoryFixtures) record(id string) *memoryChange {
	for i := range m.changes {
		if m.changes[i].change.ID == id {
			return &m.changes[i]
		}
	}
	return nil
}

func (s *server) memoryStatus() *protocol.MemoryStatus {
	m := s.memory()
	settings := s.data.snapshot.Settings.Memory
	engine := settings.Engine
	if engine == "" {
		engine = protocol.MemoryBuiltin
	}
	caps := protocol.MemoryCaps{Edit: true, Replace: true, Pin: true, Versions: true, ForgetSession: true}
	switch engine {
	case protocol.MemoryHoncho:
		caps.Edit, caps.LeavesMachine = false, true
	case protocol.MemoryHindsight:
		caps.ForgetSession = false
	}
	var counts protocol.MemoryCounts
	active := map[string]int{}
	var bytes int64 = 32000
	for _, memory := range m.memories {
		bytes += int64(len(memory.Text) + 90)
		switch memory.Status {
		case protocol.MemorySuggested:
			counts.Suggested++
			continue
		case protocol.MemoryReplaced:
			counts.Replaced++
			continue
		case protocol.MemoryBlocked:
			counts.Blocked++
			continue
		}
		counts.All++
		active[memory.Scope]++
		switch {
		case memory.Scope == protocol.MemoryYou:
			counts.You++
		case strings.HasPrefix(memory.Scope, protocol.MemoryProjectPrefix):
			counts.Projects++
		case strings.HasPrefix(memory.Scope, protocol.MemoryTeamPrefix):
			counts.Teams++
		}
	}
	scopes := make([]protocol.MemoryScope, 0, len(active))
	for scope, count := range active {
		scopes = append(scopes, protocol.MemoryScope{ID: scope, Name: s.memoryScopeName(scope), Count: count})
	}
	// about you first, then projects, then teams, each by name
	rank := func(scope string) int {
		switch {
		case scope == protocol.MemoryYou:
			return 0
		case strings.HasPrefix(scope, protocol.MemoryProjectPrefix):
			return 1
		}
		return 2
	}
	sort.Slice(scopes, func(i, j int) bool {
		if rank(scopes[i].ID) != rank(scopes[j].ID) {
			return rank(scopes[i].ID) < rank(scopes[j].ID)
		}
		return scopes[i].Name < scopes[j].Name
	})
	days := make([]protocol.MemoryDay, 30)
	rise := 0
	for i := range days {
		days[i] = protocol.MemoryDay{Day: Epoch.AddDate(0, 0, i-29).Format("2006-01-02"), Learned: demoMemoryLearned[i], Forgot: demoMemoryForgot[i]}
		if i == 29 {
			days[i].Learned += m.learned
			days[i].Forgot += m.forgot
		}
		counts.LearnedMonth += days[i].Learned
		counts.ForgotMonth += days[i].Forgot
		rise += days[i].Learned - days[i].Forgot
		days[i].InUse = rise
	}
	// the chart ends on what is in use now, so it rises to the counts beside it
	for i := range days {
		days[i].InUse = max(0, counts.All-rise+days[i].InUse)
	}
	learner := protocol.MemoryLearner{State: protocol.LearnerReady, Model: "llama.cpp", Local: true, Last: new(Epoch.Add(-6 * time.Minute)), Detail: "127.0.0.1:8080"}
	if !settings.Learn {
		learner.State = protocol.LearnerOff
	}
	return &protocol.MemoryStatus{
		Engine: engine, EngineState: protocol.EngineReady, Caps: caps, Counts: counts, Scopes: scopes, Days: days,
		Learner: learner, Card: s.memoryCard(), Path: demoHome + "/Vault/memory/rashin", Bytes: bytes,
	}
}

// pinned facts first, then oldest first, as many whole sentences as fit
func (s *server) memoryCard() protocol.MemoryCard {
	m := s.memory()
	var facts []protocol.Memory
	for _, memory := range m.memories {
		if memory.Status == protocol.MemoryActive && memory.Scope == protocol.MemoryYou {
			facts = append(facts, memory)
		}
	}
	sort.SliceStable(facts, func(i, j int) bool {
		if facts[i].Pinned != facts[j].Pinned {
			return facts[i].Pinned
		}
		return facts[i].Created.Before(facts[j].Created)
	})
	var card strings.Builder
	from := 0
	for _, fact := range facts {
		sentence := strings.TrimRight(strings.ReplaceAll(fact.Text, "`", ""), ".") + "."
		if card.Len() > 0 {
			sentence = " " + sentence
		}
		if utf8.RuneCountInString(card.String())+utf8.RuneCountInString(sentence) > protocol.MemoryCardMax {
			break
		}
		card.WriteString(sentence)
		from++
	}
	return protocol.MemoryCard{Text: card.String(), Built: m.cardBuilt, From: from, Limit: protocol.MemoryCardMax}
}

func memoryParams[T any](raw json.RawMessage) (T, error) {
	var params T
	err := decode(raw, &params)
	return params, err
}

func (s *server) callMemory(method string, raw json.RawMessage) (any, error) {
	switch method {
	case protocol.MMemoryStatus:
		return *s.memoryStatus(), nil
	case protocol.MMemoryList:
		params, err := memoryParams[protocol.MemoryListParams](raw)
		if err != nil {
			return nil, err
		}
		return s.listMemory(params), nil
	case protocol.MMemoryGet:
		params, err := memoryParams[protocol.MemoryRefParams](raw)
		if err != nil {
			return nil, err
		}
		m := s.memory()
		if index := m.index(params.ID); index >= 0 {
			return *cloneMemory(m.memories[index]), nil
		}
		return nil, memoryNotFound()
	case protocol.MMemorySearch:
		params, err := memoryParams[protocol.MemorySearchParams](raw)
		if err != nil {
			return nil, err
		}
		return s.searchMemory(params), nil
	case protocol.MMemoryAdd:
		params, err := memoryParams[protocol.MemoryAddParams](raw)
		if err != nil {
			return nil, err
		}
		return s.addMemory(params)
	case protocol.MMemoryEdit:
		params, err := memoryParams[protocol.MemoryEditParams](raw)
		if err != nil {
			return nil, err
		}
		return s.editMemory(params)
	case protocol.MMemoryForget:
		params, err := memoryParams[protocol.MemoryForgetParams](raw)
		if err != nil {
			return nil, err
		}
		return s.forgetMemory(params.IDs, params.Session, "")
	case protocol.MMemoryForgetAll:
		params, err := memoryParams[protocol.MemoryForgetAllParams](raw)
		if err != nil {
			return nil, err
		}
		var ids []string
		for _, memory := range s.memory().memories {
			if memoryInScope(memory, params.Scope) {
				ids = append(ids, memory.ID)
			}
		}
		return s.forgetMemory(ids, "", params.Scope)
	case protocol.MMemoryRestore:
		params, err := memoryParams[protocol.MemoryRestoreParams](raw)
		if err != nil {
			return nil, err
		}
		return s.restoreMemory(params)
	case protocol.MMemoryKeep, protocol.MMemoryDismiss:
		params, err := memoryParams[protocol.MemoryIDsParams](raw)
		if err != nil {
			return nil, err
		}
		return s.decideMemory(params.IDs, method == protocol.MMemoryKeep)
	case protocol.MMemoryUndo, protocol.MMemoryRedo:
		params, err := memoryParams[protocol.MemoryChangeParams](raw)
		if err != nil {
			return nil, err
		}
		return s.undoMemory(params.Change, method == protocol.MMemoryUndo)
	case protocol.MMemoryChanges:
		params, err := memoryParams[protocol.MemoryChangesParams](raw)
		if err != nil {
			return nil, err
		}
		m := s.memory()
		result := protocol.MemoryChangesResult{Changes: []protocol.MemoryChange{}}
		for _, id := range params.IDs {
			if change := m.record(id); change != nil {
				result.Changes = append(result.Changes, change.change)
			}
		}
		return result, nil
	case protocol.MMemoryCard:
		params, err := memoryParams[protocol.MemoryCardParams](raw)
		if err != nil {
			return nil, err
		}
		if params.Rebuild {
			m := s.memory()
			m.cardBuilt = m.now()
			s.tellMemory(nil)
		}
		return s.memoryCard(), nil
	case protocol.MMemoryImport:
		params, err := memoryParams[protocol.MemoryImportParams](raw)
		if err != nil {
			return nil, err
		}
		return s.importMemory(params)
	case protocol.MMemorySessions:
		params, err := memoryParams[protocol.MemorySessionsParams](raw)
		if err != nil {
			return nil, err
		}
		return s.memorySessions(params), nil
	case protocol.MMemorySuppress:
		params, err := memoryParams[protocol.MemorySuppressParams](raw)
		if err != nil {
			return nil, err
		}
		m := s.memory()
		if m.index(params.ID) < 0 {
			return nil, memoryNotFound()
		}
		m.suppressed = append(m.suppressed, params)
		return protocol.Empty{}, nil
	case protocol.MMemoryLearn:
		params, err := memoryParams[protocol.MemoryLearnParams](raw)
		if err != nil {
			return nil, err
		}
		// nothing new in the demo's conversations, so the pass learns nothing
		m := s.memory()
		return s.commitMemory(protocol.MemoryChange{At: m.now(), Origin: protocol.MemoryByLearning, Session: params.Session, Model: "llama.cpp"}, m.byID()), nil
	case protocol.MMemoryEngineTest:
		params, err := memoryParams[protocol.MemoryEngineParams](raw)
		if err != nil {
			return nil, err
		}
		return s.testMemoryEngine(params.Engine)
	case protocol.MMemoryEngineSetup:
		params, err := memoryParams[protocol.MemoryEngineParams](raw)
		if err != nil {
			return nil, err
		}
		return s.setupMemoryEngine(params.Engine)
	case protocol.MMemoryEngineMove:
		params, err := memoryParams[protocol.MemoryEngineParams](raw)
		if err != nil {
			return nil, err
		}
		if !knownMemoryEngine(params.Engine) {
			return nil, memoryBadRequest(i18n.T("That memory engine is not one Rashin knows."))
		}
		m := s.memory()
		s.data.snapshot.Settings.Memory.Engine = params.Engine
		m.settings = s.data.snapshot.Settings.Memory
		// the daemon says the settings moved too, or the app keeps offering the old engine
		if wire, err := json.Marshal(s.data.snapshot.Settings); err == nil {
			m.told = append(m.told, protocol.Event{Event: protocol.EvtSettingsChanged, Data: wire})
		}
		s.tellMemory(nil)
		return *s.memoryStatus(), nil
	}
	return nil, protocol.Errorf(protocol.CodeUnsupported, "demo method %s is not implemented", method)
}

func knownMemoryEngine(engine string) bool {
	return engine == protocol.MemoryBuiltin || engine == protocol.MemoryHindsight || engine == protocol.MemoryHoncho
}

func (s *server) listMemory(params protocol.MemoryListParams) protocol.MemoryListResult {
	status := params.Status
	if status == "" {
		status = protocol.MemoryActive
	}
	query := strings.ToLower(strings.TrimSpace(params.Query))
	matched := []protocol.Memory{}
	for _, memory := range s.memory().memories {
		if memory.Status != status || !memoryInScope(memory, params.Scope) {
			continue
		}
		if params.Session != "" && memory.Source.Session != params.Session {
			continue
		}
		if query != "" && !strings.Contains(strings.ToLower(memory.Text+"\n"+memory.ScopeName+"\n"+memory.Source.Title), query) {
			continue
		}
		matched = append(matched, *cloneMemory(memory))
	}
	sort.SliceStable(matched, func(i, j int) bool {
		a, b := matched[i], matched[j]
		switch params.Sort {
		case protocol.MemorySortOldest:
			if !a.Created.Equal(b.Created) {
				return a.Created.Before(b.Created)
			}
		case protocol.MemorySortUsed:
			if a.Uses != b.Uses {
				return a.Uses > b.Uses
			}
		case protocol.MemorySortUnused:
			if a.Uses != b.Uses {
				return a.Uses < b.Uses
			}
			if !a.Created.Equal(b.Created) {
				return a.Created.Before(b.Created)
			}
		}
		if !a.Created.Equal(b.Created) {
			return a.Created.After(b.Created)
		}
		return a.ID < b.ID
	})
	total := len(matched)
	offset := min(max(params.Offset, 0), total)
	matched = matched[offset:]
	if params.Limit > 0 && len(matched) > params.Limit {
		matched = matched[:params.Limit]
	}
	return protocol.MemoryListResult{Memories: matched, Total: total}
}

// best first by how many of the words a memory holds
func (s *server) searchMemory(params protocol.MemorySearchParams) protocol.MemorySearchResult {
	words := strings.Fields(strings.ToLower(params.Query))
	type hit struct {
		memory protocol.Memory
		score  int
	}
	var hits []hit
	for _, memory := range s.memory().memories {
		if memory.Status != protocol.MemoryActive || !memoryInScope(memory, params.Scope) {
			continue
		}
		text := strings.ToLower(memory.Text + "\n" + memory.ScopeName + "\n" + memory.Source.Title)
		score := 0
		for _, word := range words {
			if strings.Contains(text, word) {
				score++
			}
		}
		if score > 0 || len(words) == 0 {
			hits = append(hits, hit{memory: *cloneMemory(memory), score: score})
		}
	}
	sort.SliceStable(hits, func(i, j int) bool {
		if hits[i].score != hits[j].score {
			return hits[i].score > hits[j].score
		}
		return hits[i].memory.Created.After(hits[j].memory.Created)
	})
	result := protocol.MemorySearchResult{Memories: []protocol.Memory{}}
	for _, hit := range hits {
		if params.Limit > 0 && len(result.Memories) == params.Limit {
			break
		}
		result.Memories = append(result.Memories, hit.memory)
	}
	return result
}

func (s *server) addMemory(params protocol.MemoryAddParams) (protocol.MemoryChange, error) {
	scope := params.Scope
	if scope == "" {
		scope = protocol.MemoryYou
	}
	if !validMemoryScope(scope) {
		return protocol.MemoryChange{}, memoryBadRequest(i18n.T("Pick where to remember it."))
	}
	text, err := checkMemoryText(params.Text)
	if err != nil {
		return protocol.MemoryChange{}, err
	}
	m := s.memory()
	before, now := m.byID(), m.now()
	memory := protocol.Memory{
		ID: m.newID(), Scope: scope, ScopeName: s.memoryScopeName(scope), Text: text, Status: protocol.MemoryActive,
		Kind: protocol.MemoryDurable, Pinned: params.Pinned, Sensitive: params.Sensitive,
		Source: protocol.MemorySource{Kind: protocol.MemoryFromYou, Session: params.Session}, Created: now, Updated: now,
	}
	m.memories = append(m.memories, memory)
	m.learned++
	return s.commitMemory(protocol.MemoryChange{At: now, Origin: protocol.MemoryByYou, Session: params.Session, Items: []protocol.MemoryItem{memoryItem(memory, protocol.MemoryOpRemembered)}}, before), nil
}

func (s *server) editMemory(params protocol.MemoryEditParams) (protocol.MemoryChange, error) {
	m := s.memory()
	index := m.index(params.ID)
	if index < 0 {
		return protocol.MemoryChange{}, memoryNotFound()
	}
	before, now := m.byID(), m.now()
	memory := cloneMemory(m.memories[index])
	op := ""
	if params.Text != nil {
		text, err := checkMemoryText(*params.Text)
		if err != nil {
			return protocol.MemoryChange{}, err
		}
		if text != memory.Text {
			if !s.memoryStatus().Caps.Edit {
				return protocol.MemoryChange{}, protocol.Errorf(protocol.CodeUnsupported, "%s", i18n.T("This memory engine can't change a memory's words; forget it and remember the new ones."))
			}
			memory.Versions = append([]protocol.MemoryVersion{{Text: memory.Text, At: memory.Updated, By: "you"}}, memory.Versions...)
			memory.Versions = memory.Versions[:min(len(memory.Versions), 3)]
			memory.Text, op = text, protocol.MemoryOpEdited
		}
	}
	if params.Scope != nil && *params.Scope != memory.Scope {
		if !validMemoryScope(*params.Scope) {
			return protocol.MemoryChange{}, memoryBadRequest(i18n.T("Pick where to remember it."))
		}
		memory.Scope, memory.ScopeName, op = *params.Scope, s.memoryScopeName(*params.Scope), protocol.MemoryOpEdited
	}
	if params.Pinned != nil && *params.Pinned != memory.Pinned {
		memory.Pinned = *params.Pinned
		if op == "" {
			op = protocol.MemoryOpUnpinned
			if memory.Pinned {
				op = protocol.MemoryOpPinned
			}
		}
	}
	if op == "" {
		op = protocol.MemoryOpEdited
	}
	memory.Updated = now
	m.memories[index] = *memory
	return s.commitMemory(protocol.MemoryChange{At: now, Origin: protocol.MemoryByYou, Items: []protocol.MemoryItem{memoryItem(*memory, op)}}, before), nil
}

func (s *server) forgetMemory(ids []string, session, scope string) (protocol.MemoryChange, error) {
	m := s.memory()
	before, now := m.byID(), m.now()
	items := []protocol.MemoryItem{}
	for _, id := range ids {
		index := m.index(id)
		if index < 0 {
			continue
		}
		memory := m.memories[index]
		if memory.Status == protocol.MemoryActive {
			m.forgot++
		}
		items = append(items, memoryItem(memory, protocol.MemoryOpForgot))
		m.memories = slices.Delete(m.memories, index, index+1)
	}
	if len(items) == 0 && scope == "" && len(ids) > 0 {
		return protocol.MemoryChange{}, memoryNotFound()
	}
	return s.commitMemory(protocol.MemoryChange{At: now, Origin: protocol.MemoryByYou, Session: session, Items: items}, before), nil
}

func (s *server) restoreMemory(params protocol.MemoryRestoreParams) (protocol.MemoryChange, error) {
	m := s.memory()
	index := m.index(params.ID)
	if index < 0 {
		return protocol.MemoryChange{}, memoryNotFound()
	}
	before, now := m.byID(), m.now()
	memory := cloneMemory(m.memories[index])
	item := memoryItem(*memory, protocol.MemoryOpRestored)
	switch {
	case params.Version == 0:
		if memory.Status != protocol.MemoryReplaced {
			return protocol.MemoryChange{}, protocol.Errorf(protocol.CodeConflict, "%s", i18n.T("Only a replaced memory can be brought back."))
		}
		memory.Status, memory.Replaced, memory.ReplacedBy, memory.ReplacedByText = protocol.MemoryActive, nil, "", ""
	case params.Version > 0 && params.Version <= len(memory.Versions):
		earlier := memory.Versions[params.Version-1]
		rest := slices.Delete(slices.Clone(memory.Versions), params.Version-1, params.Version)
		memory.Versions = append([]protocol.MemoryVersion{{Text: memory.Text, At: memory.Updated, By: "you"}}, rest...)
		memory.Versions = memory.Versions[:min(len(memory.Versions), 3)]
		item.Replaces, memory.Text = memory.Text, earlier.Text
		item.Text = memory.Text
	default:
		return protocol.MemoryChange{}, memoryBadRequest(i18n.T("That earlier version is not there any more."))
	}
	memory.Updated = now
	m.memories[index] = *memory
	return s.commitMemory(protocol.MemoryChange{At: now, Origin: protocol.MemoryByYou, Items: []protocol.MemoryItem{item}}, before), nil
}

func (s *server) decideMemory(ids []string, keep bool) (protocol.MemoryChange, error) {
	m := s.memory()
	before, now := m.byID(), m.now()
	items := []protocol.MemoryItem{}
	for _, id := range ids {
		index := m.index(id)
		if index < 0 || m.memories[index].Status != protocol.MemorySuggested {
			continue
		}
		if keep {
			m.memories[index].Status, m.memories[index].Updated = protocol.MemoryActive, now
			m.learned++
			items = append(items, memoryItem(m.memories[index], protocol.MemoryOpKept))
			continue
		}
		items = append(items, memoryItem(m.memories[index], protocol.MemoryOpDismissed))
		m.memories = slices.Delete(m.memories, index, index+1)
	}
	if len(items) == 0 {
		return protocol.MemoryChange{}, memoryNotFound()
	}
	return s.commitMemory(protocol.MemoryChange{At: now, Origin: protocol.MemoryByYou, Items: items}, before), nil
}

func (s *server) undoMemory(id string, undo bool) (protocol.MemoryChange, error) {
	m := s.memory()
	change := m.record(id)
	if change == nil {
		return protocol.MemoryChange{}, protocol.Errorf(protocol.CodeNotFound, "%s", i18n.T("That change is not there any more."))
	}
	if change.change.Undone == undo {
		return change.change, nil
	}
	for _, snap := range change.snaps {
		if undo {
			m.put(snap.id, snap.before)
		} else {
			m.put(snap.id, snap.after)
		}
	}
	change.change.Undone = undo
	result := change.change
	s.tellMemory(&result)
	return result, nil
}

func (s *server) importMemory(params protocol.MemoryImportParams) (protocol.MemoryChange, error) {
	scope := params.Scope
	if scope == "" {
		scope = protocol.MemoryYou
	}
	if !validMemoryScope(scope) {
		return protocol.MemoryChange{}, memoryBadRequest(i18n.T("Pick where to remember it."))
	}
	from := params.From
	if from == "" {
		from = i18n.T("another assistant")
	}
	m := s.memory()
	before, now := m.byID(), m.now()
	items := []protocol.MemoryItem{}
	for line := range strings.Lines(params.Text) {
		text := strings.TrimSpace(memoryBullet.ReplaceAllString(strings.TrimSpace(line), ""))
		if text == "" || memorySecret.MatchString(text) {
			continue
		}
		if runes := []rune(text); len(runes) > protocol.MemoryTextMax {
			text = strings.TrimSpace(string(runes[:protocol.MemoryTextMax]))
		}
		memory := protocol.Memory{
			ID: m.newID(), Scope: scope, ScopeName: s.memoryScopeName(scope), Text: text, Status: protocol.MemorySuggested,
			Kind: protocol.MemoryDurable, Source: protocol.MemorySource{Kind: protocol.MemoryFromImport, From: from}, Created: now, Updated: now,
		}
		m.memories = append(m.memories, memory)
		items = append(items, memoryItem(memory, protocol.MemoryOpSuggested))
	}
	if len(items) == 0 {
		return protocol.MemoryChange{}, memoryBadRequest(i18n.T("Nothing in that text reads as something to remember."))
	}
	return s.commitMemory(protocol.MemoryChange{At: now, Origin: protocol.MemoryByImport, Items: items}, before), nil
}

func (s *server) memorySessions(params protocol.MemorySessionsParams) protocol.MemorySessionsResult {
	hits := []protocol.MemorySessionHit{
		{Session: "ask-20261109-161200-prev", Summary: i18n.T("Moved from Hyprland to niri; the keybinds carried over and the Wi-Fi fix after suspend still applies."),
			Lines: []string{i18n.T("Switch with `ryoku compositor niri`, then log out and pick niri."), i18n.T("Your Hyprland binds are kept for when you switch back.")}},
		{Session: "ask-20261107-112500-week", Summary: i18n.T("Provider keys live in the Ryoku vault, never in settings files."),
			Lines: []string{i18n.T("Keys are kept in the vault and only their names travel in settings.")}},
	}
	result := protocol.MemorySessionsResult{Sessions: []protocol.MemorySessionHit{}}
	for _, hit := range hits {
		if params.Limit > 0 && len(result.Sessions) == params.Limit {
			break
		}
		if info, ok := s.session(hit.Session); ok {
			hit.Kind, hit.Title, hit.Updated = info.Kind, info.Title, info.Updated
			result.Sessions = append(result.Sessions, hit)
		}
	}
	return result
}

func (s *server) testMemoryEngine(engine string) (protocol.MemoryEngineTest, error) {
	switch engine {
	case protocol.MemoryBuiltin, protocol.MemoryHindsight:
		return protocol.MemoryEngineTest{OK: true}, nil
	case protocol.MemoryHoncho:
		if s.data.snapshot.Settings.Memory.Honcho.URL != "" {
			return protocol.MemoryEngineTest{OK: true}, nil
		}
		return protocol.MemoryEngineTest{Detail: "connection refused"}, nil
	}
	return protocol.MemoryEngineTest{}, memoryBadRequest(i18n.T("That memory engine is not one Rashin knows."))
}

// the steps arrive after the response, the way a real setup reports as it goes
func (s *server) setupMemoryEngine(engine string) (protocol.Empty, error) {
	if !knownMemoryEngine(engine) {
		return protocol.Empty{}, memoryBadRequest(i18n.T("That memory engine is not one Rashin knows."))
	}
	m := s.memory()
	steps := []string{i18n.T("Downloading"), i18n.T("Unpacking"), i18n.T("Starting"), i18n.T("Checking it answers")}
	for i, step := range steps {
		progress := protocol.MemorySetupProgress{Engine: engine, Step: step, Part: i + 1, Parts: len(steps), Done: i == len(steps)-1}
		if wire, err := json.Marshal(progress); err == nil {
			m.told = append(m.told, protocol.Event{Event: protocol.EvtMemorySetup, Data: wire})
		}
	}
	return protocol.Empty{}, nil
}
