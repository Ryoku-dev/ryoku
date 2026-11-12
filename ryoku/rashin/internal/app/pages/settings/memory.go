package settings

import (
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
	"time"

	"gioui.org/layout"
	"gioui.org/widget"

	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/kit"
	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/palette"
	"ryoku-i18n"

	"ryoku-rashin/internal/app/env"
	"ryoku-rashin/internal/app/look"
	"ryoku-rashin/internal/app/pages/memoryui"
	"ryoku-rashin/internal/app/state"
	"ryoku-rashin/internal/memory/backend"
	"ryoku-rashin/internal/paths"
	"ryoku-rashin/internal/protocol"
)

// the vault credential a Honcho key is kept under when nothing names one yet
const honchoKeyName = "HONCHO_API_KEY"

// toggles and selects save as they change; only Honcho's address and key are typed
type memoryPanel struct {
	open, forget, importing, folder kit.Plaque

	use, learn, sessions, agents, sensitive kit.Toggle
	teams, model, fade                      look.Select

	engine        kit.Segmented
	picked        string
	test, setup   kit.Plaque
	move          kit.Plaque
	tested        string
	testedEngine  string
	address       kit.TextField
	addressFilled string
	saveAddress   kit.Plaque
	key           kit.TextField
	storeKey      kit.Plaque
	replaceKey    kit.Plaque
	replacing     bool

	moving    kit.Confirm
	movingTo  string
	forgetAll memoryui.ForgetAll
	importer  memoryui.Import
}

var engines = []string{protocol.MemoryBuiltin, protocol.MemoryHindsight, protocol.MemoryHoncho}

var engineName = memoryui.EngineName

func engineDetail(engine string) string {
	switch engine {
	case protocol.MemoryHindsight:
		return i18n.Tf("Runs %s on this computer, starting and stopping with Rashin. It uses about 1.2 GB while it runs.", engineName(engine))
	case protocol.MemoryHoncho:
		return i18n.Tf("Talks to a %s server you run. What you remember is sent to that server.", engineName(engine))
	}
	return i18n.T("Plain files Rashin keeps and indexes. Nothing else runs.")
}

func memoryProof(settings protocol.MemorySettings, status *protocol.MemoryStatus) string {
	switch {
	case !settings.Asked:
		return i18n.T("Memory has not been turned on.")
	case !settings.Use:
		return i18n.T("Memory is off. Rashin keeps nothing between conversations.")
	case status == nil:
		return i18n.T("Memory is on.")
	}
	projects, teams := 0, 0
	for _, scope := range status.Scopes {
		switch {
		case strings.HasPrefix(scope.ID, protocol.MemoryProjectPrefix):
			projects++
		case strings.HasPrefix(scope.ID, protocol.MemoryTeamPrefix):
			teams++
		}
	}
	proof := i18n.Tf("Rashin remembers %s things about you, %s projects and %s teams.", look.Count(status.Counts.All), look.Count(projects), look.Count(teams))
	switch waiting := status.Counts.Suggested; {
	case waiting == 1:
		proof += " " + i18n.T("One is waiting for you.")
	case waiting > 1:
		proof += " " + i18n.Tf("%s are waiting for you.", look.CountStart(waiting))
	}
	return proof
}

func (p *Page) memoryBlocks(gtx layout.Context, e *env.Env, blocks []layout.Widget) []layout.Widget {
	th := e.Theme
	m := &p.memory
	snapshot := e.Snapshot()
	settings := snapshot.Settings.Memory
	m.handle(gtx, e, snapshot)
	title := func(gtx layout.Context) layout.Dimensions {
		return m.layoutTitle(gtx, th, memoryProof(settings, snapshot.Memory))
	}
	return spacedBlocks(th, append(blocks, title),
		m.usingGroup(e, settings),
		m.modelGroup(e, gtx.Now, settings, snapshot.Memory),
		m.fadingGroup(e, settings),
		m.engineGroup(e, snapshot),
		m.importGroup(th),
	)
}

// the section's title and proof, with Open memory and Forget everything beside them and no gold: these toggles save themselves
func (m *memoryPanel) layoutTitle(gtx layout.Context, th *look.Theme, proof string) layout.Dimensions {
	m.open.Kind, m.open.Small, m.open.Label = kit.PlaqueTonal, true, i18n.T("Open memory")
	m.forget.Kind, m.forget.Small, m.forget.Label = kit.PlaqueDangerLine, true, i18n.T("Forget everything")
	words := func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			layout.Rigid(kit.Text(th.Theme, th.T.Section, th.R.Fg, i18n.T("Memory"))),
			layout.Rigid(layout.Spacer{Height: th.Sp.S2}.Layout),
			layout.Rigid(look.Paragraph(th, th.T.Proof, th.R.Fg2, proof)),
		)
	}
	acts := func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
			layout.Rigid(func(gtx layout.Context) layout.Dimensions { return m.open.Layout(gtx, th.Theme) }),
			layout.Rigid(layout.Spacer{Width: th.Sp.S2}.Layout),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions { return m.forget.Layout(gtx, th.Theme) }),
		)
	}
	if gtx.Constraints.Max.X < gtx.Dp(560) {
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			layout.Rigid(words),
			layout.Rigid(layout.Spacer{Height: th.Sp.S3}.Layout),
			layout.Rigid(acts),
		)
	}
	return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Start}.Layout(gtx,
		layout.Flexed(1, words),
		layout.Rigid(layout.Spacer{Width: th.Sp.S4}.Layout),
		layout.Rigid(acts),
	)
}

func (m *memoryPanel) handle(gtx layout.Context, e *env.Env, snapshot state.Snapshot) {
	settings := snapshot.Settings.Memory
	if m.open.Clicked(gtx) {
		e.OpenMemory(env.MemoryPlace{})
	}
	if m.forget.Clicked(gtx) {
		m.forgetAll.Open("")
	}
	if m.importing.Clicked(gtx) {
		m.importer.Open("")
	}
	if m.folder.Clicked(gtx) && snapshot.Memory != nil && snapshot.Memory.Path != "" {
		e.OpenFolder(snapshot.Memory.Path)
	}
	if m.picked == "" {
		m.picked = settings.Engine
	}
	if index, changed := m.engine.Changed(); changed && index < len(engines) {
		m.picked = engines[index]
	}
	if m.test.Clicked(gtx) && !m.test.Busy {
		m.test.Busy = true
		engine := m.picked
		var result protocol.MemoryEngineTest
		e.Call(protocol.MMemoryEngineTest, protocol.MemoryEngineParams{Engine: engine}, &result, func(err error) {
			m.test.Busy, m.testedEngine = false, engine
			switch {
			case err != nil:
				m.tested = err.Error()
			case result.OK:
				m.tested = i18n.Tf("%s answers.", engineName(engine))
			default:
				m.tested = i18n.Tf("%s doesn't answer: %s", engineName(engine), result.Detail)
			}
		})
	}
	if m.setup.Clicked(gtx) {
		e.Call(protocol.MMemoryEngineSetup, protocol.MemoryEngineParams{Engine: m.picked}, new(protocol.Empty), e.Report)
	}
	if m.move.Clicked(gtx) && m.picked != settings.Engine {
		m.movingTo = m.picked
		m.moving.Dialog.Open, m.moving.Dialog.Width = true, 520
	}
	if m.saveAddress.Clicked(gtx) || submitted(gtx, &m.address.Editor) {
		address := strings.TrimSpace(m.address.Text())
		set(e, "memory", map[string]any{"honcho": map[string]any{"url": address}})
		// the key may only go where the address points, so its host follows the address
		if credential, ok := vaultCredential(snapshot.Vault, settings.Honcho.Key); ok && settings.Honcho.Key != "" {
			if hosts := addressHosts(address); len(hosts) > 0 {
				patch, _ := json.Marshal(map[string]any{"hosts": hosts})
				e.Call(protocol.MVaultUpdate, protocol.VaultUpdateParams{ID: credential.ID, Patch: patch}, new(protocol.Credential), e.Report)
			}
		}
	}
	if m.replaceKey.Clicked(gtx) {
		m.replacing = true
	}
	if (m.storeKey.Clicked(gtx) || submitted(gtx, &m.key.Editor)) && strings.TrimSpace(m.key.Text()) != "" && settings.Honcho.URL != "" && !m.storeKey.Busy {
		m.storeHonchoKey(e, settings, snapshot.Vault)
	}
}

func submitted(gtx layout.Context, editor *widget.Editor) bool {
	editor.SingleLine, editor.Submit = true, true
	done := false
	for {
		event, ok := editor.Update(gtx)
		if !ok {
			return done
		}
		if _, ok := event.(widget.SubmitEvent); ok {
			done = true
		}
	}
}

// the key goes into the vault, settings only ever hold its name
func (m *memoryPanel) storeHonchoKey(e *env.Env, settings protocol.MemorySettings, vault protocol.VaultInfo) {
	name := settings.Honcho.Key
	if name == "" {
		name = honchoKeyName
	}
	value := strings.TrimSpace(m.key.Text())
	hosts := addressHosts(settings.Honcho.URL)
	done := func(err error) {
		m.storeKey.Busy = false
		if err != nil {
			e.Report(err)
			return
		}
		m.key.SetText("")
		m.replacing = false
		set(e, "memory", map[string]any{"honcho": map[string]any{"key": name}})
		e.Notify(i18n.Tf("Stored the %s key in the vault as %s.", engineName(protocol.MemoryHoncho), name))
	}
	m.storeKey.Busy = true
	if credential, ok := vaultCredential(vault, name); ok {
		e.Call(protocol.MVaultUpdate, protocol.VaultUpdateParams{ID: credential.ID, Value: value}, new(protocol.Credential), done)
		return
	}
	params := protocol.VaultAddParams{
		Name: name, Description: i18n.Tf("The key Rashin's memory uses to reach your %s server.", engineName(protocol.MemoryHoncho)),
		Use: protocol.CredentialHeader, Prefix: "Bearer ", Hosts: hosts, Value: value,
	}
	e.Call(protocol.MVaultAdd, params, new(protocol.Credential), done)
}

func addressHosts(address string) []string {
	if parsed, err := url.Parse(address); err == nil && parsed.Hostname() != "" {
		return []string{parsed.Hostname()}
	}
	return nil
}

// whether what is remembered leaves this computer with engine as settings point it, the backends' own rule
func leaves(settings protocol.MemorySettings, engine string) bool {
	switch engine {
	case protocol.MemoryHoncho:
		return settings.Honcho.URL != "" && backend.LeavesMachine(settings.Honcho.URL)
	case protocol.MemoryHindsight:
		return !settings.Hindsight.Local && settings.Hindsight.URL != "" && backend.LeavesMachine(settings.Hindsight.URL)
	}
	return false
}

func vaultCredential(vault protocol.VaultInfo, name string) (protocol.Credential, bool) {
	for _, credential := range vault.Credentials {
		if credential.Name == name {
			return credential, true
		}
	}
	return protocol.Credential{}, false
}

// a control under Use memory, standing back while it is off
func dependent(on bool, control layout.Widget) layout.Widget {
	return func(gtx layout.Context) layout.Dimensions {
		if !on {
			gtx = gtx.Disabled()
		}
		return control(gtx)
	}
}

func (m *memoryPanel) usingGroup(e *env.Env, settings protocol.MemorySettings) layout.Widget {
	th := e.Theme
	using := settings.Asked && settings.Use
	m.use.Value, m.learn.Value, m.sessions.Value = using, settings.Learn, settings.Sessions
	m.agents.Value, m.sensitive.Value = settings.Agents, settings.Sensitive
	toggle := func(name string, control *kit.Toggle, key string) layout.Widget {
		return func(gtx layout.Context) layout.Dimensions {
			return namedSwitch(gtx, th, name, control, func(on bool) { set(e, "memory", map[string]any{key: on}) })
		}
	}
	learnDetail := i18n.T("When a conversation goes quiet, Rashin reads what you wrote and its final answers, and remembers what will help later. It never learns from command output, files or web pages.")
	if !using {
		learnDetail = i18n.T("Turn on Use memory first.")
	}
	suggestions := []look.SelectOption{
		{Label: i18n.T("Ask me first"), Detail: i18n.T("They wait under Suggested and in the Inbox.")},
		{Label: i18n.T("Never"), Detail: i18n.T("Teams keep their own memory only.")},
	}
	suggestAt := 0
	if settings.TeamSuggestions == protocol.TeamSuggestNever {
		suggestAt = 1
	}
	rows := []look.Setting{
		{
			Title:  i18n.T("Use memory"),
			Detail: i18n.T("Answers can draw on what Rashin remembers about you, your projects and your teams. Off keeps what it remembers and uses none of it."),
			Control: func(gtx layout.Context) layout.Dimensions {
				return namedSwitch(gtx, th, i18n.T("Use memory"), &m.use, func(on bool) {
					patch := map[string]any{"asked": true, "use": on}
					// the first time it is turned on here it learns as well, as the first-run card would have
					if on && !settings.Asked {
						patch["learn"] = true
					}
					set(e, "memory", patch)
				})
			},
		},
		{Title: i18n.T("Learn from conversations"), Detail: learnDetail, Control: dependent(using, toggle(i18n.T("Learn from conversations"), &m.learn, "learn"))},
		{
			Title:   i18n.T("Search past conversations"),
			Detail:  i18n.T("When you ask about an earlier conversation, Rashin may look through your past asks and sessions, and shows which ones it opened."),
			Control: dependent(using, toggle(i18n.T("Search past conversations"), &m.sessions, "sessions")),
		},
		{
			Title:   i18n.T("Memory in agent sessions"),
			Detail:  i18n.T("Harnesses you start from Rashin can read and remember through Rashin's memory tools. Each remembered thing shows in the session."),
			Control: dependent(using, toggle(i18n.T("Memory in agent sessions"), &m.agents, "agents")),
		},
		{
			Title:  i18n.T("Suggestions from teams"),
			Detail: suggestions[suggestAt].Detail,
			Control: func(gtx layout.Context) layout.Dimensions {
				dimensions := look.Named(gtx, i18n.T("Suggestions from teams"), func(gtx layout.Context) layout.Dimensions {
					return m.teams.LayoutOptions(gtx, th, e.Overlays, suggestions, suggestAt)
				})
				if index, ok := m.teams.Changed(); ok {
					value := protocol.TeamSuggestAsk
					if index == 1 {
						value = protocol.TeamSuggestNever
					}
					set(e, "memory", map[string]any{"teamSuggestions": value})
				}
				return dimensions
			},
		},
		{
			Title:   i18n.T("Keep sensitive topics"),
			Detail:  i18n.T("Health, beliefs, money, relationships and similar topics are left out unless you turn this on. Passwords, keys and tokens are never kept."),
			Control: toggle(i18n.T("Keep sensitive topics"), &m.sensitive, "sensitive"),
		},
	}
	return teamsGroup(th, i18n.T("Using and learning"), "", rows, nil)
}

var learners = []string{protocol.LearnLocal, protocol.LearnLocalFirst, protocol.LearnAny}

func (m *memoryPanel) modelGroup(e *env.Env, now time.Time, settings protocol.MemorySettings, status *protocol.MemoryStatus) layout.Widget {
	th := e.Theme
	options := []look.SelectOption{
		{Label: i18n.T("Local models only"), Detail: i18n.T("Nothing leaves this computer. Learning waits until a local model answers.")},
		{Label: i18n.T("Local first"), Detail: i18n.T("A local model when one answers, otherwise the provider that already answered that conversation.")},
		{Label: i18n.T("Any provider"), Detail: i18n.T("The model Ask uses. Free providers may keep what they are sent.")},
	}
	at := 1
	for index, learner := range learners {
		if learner == settings.Model {
			at = index
		}
	}
	rows := []look.Setting{{
		Title:  i18n.T("Learns with"),
		Detail: options[at].Detail,
		Control: func(gtx layout.Context) layout.Dimensions {
			dimensions := look.Named(gtx, i18n.T("Learns with"), func(gtx layout.Context) layout.Dimensions {
				return m.model.LayoutOptions(gtx, th, e.Overlays, options, at)
			})
			if index, ok := m.model.Changed(); ok && index < len(learners) {
				set(e, "memory", map[string]any{"model": learners[index]})
			}
			return dimensions
		},
	}}
	if status != nil {
		claim, proof, tone := learnerWords(now, status.Learner)
		rows = append(rows, look.Setting{
			Title: i18n.T("Now"), Below: true,
			Control: func(gtx layout.Context) layout.Dimensions {
				return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Baseline}.Layout(gtx,
					layout.Rigid(kit.Claim(th.Theme, tone, claim)),
					layout.Rigid(layout.Spacer{Width: th.Sp.S3}.Layout),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return kit.Sentence(gtx, th.Theme, th.T.Detail, th.R.Fg2, proof)
					}),
				)
			},
		})
	}
	return teamsGroup(th, i18n.T("Learning model"), "", rows, nil)
}

// what the learner is doing now, as a claim and its proof
// the proof's address is literal data, so it is set in mono
func learnerWords(now time.Time, learner protocol.MemoryLearner) (claim string, proof []kit.Span, tone palette.Tone) {
	plain := func(text string) []kit.Span { return []kit.Span{{Text: text}} }
	switch {
	case learner.State == protocol.LearnerWaiting:
		text := learner.Detail
		if text == "" {
			text = i18n.T("No local model answers.")
		}
		if learner.Waiting > 0 {
			text += " " + i18n.Tf("%s conversations to learn from.", look.CountStart(learner.Waiting))
		}
		return i18n.T("Waiting"), plain(text), palette.Attention
	case learner.State == protocol.LearnerOff:
		return i18n.T("Off"), plain(i18n.T("Starts when needed.")), palette.Idle
	case learner.Waiting > 0:
		return i18n.T("Learning"), plain(i18n.Tf("%s conversations waiting.", look.CountStart(learner.Waiting))), palette.Becoming
	}
	proof = plain(learner.Model)
	if learner.Detail != "" {
		proof = []kit.Span{{Text: i18n.Tf("%s at ", learner.Model)}, {Text: learner.Detail, Mono: true}}
	}
	end := "."
	if learner.Last != nil {
		end = ", " + i18n.Tf("learned %s.", look.Ago(now, *learner.Last))
	}
	return i18n.T("Ready"), append(proof, kit.Span{Text: end}), palette.Alive
}

var fades = []int{0, 6, 12, 18}

func (m *memoryPanel) fadingGroup(e *env.Env, settings protocol.MemorySettings) layout.Widget {
	th := e.Theme
	labels := []string{i18n.T("Never"), i18n.T("After 6 months"), i18n.T("After a year"), i18n.T("After 18 months")}
	at := 0
	for index, months := range fades {
		if months == settings.FadeMonths {
			at = index
		}
	}
	rows := []look.Setting{{
		Title:  i18n.T("Forget unused memories"),
		Detail: i18n.T("A memory no answer has used in that time is forgotten. Pinned memories never fade."),
		Control: func(gtx layout.Context) layout.Dimensions {
			dimensions := look.Named(gtx, i18n.T("Forget unused memories"), func(gtx layout.Context) layout.Dimensions {
				return m.fade.Layout(gtx, th, e.Overlays, labels, at)
			})
			if index, ok := m.fade.Changed(); ok && index < len(fades) {
				set(e, "memory", map[string]any{"fadeMonths": fades[index]})
			}
			return dimensions
		},
	}}
	return teamsGroup(th, i18n.T("Fading"), "", rows, nil)
}

func humanBytes(size int64) string {
	switch {
	case size >= 1<<20:
		return fmt.Sprintf("%.1f MB", float64(size)/(1<<20))
	case size >= 1<<10:
		return fmt.Sprintf("%d KB", size>>10)
	}
	return fmt.Sprintf("%d B", size)
}

func (m *memoryPanel) engineGroup(e *env.Env, snapshot state.Snapshot) layout.Widget {
	th := e.Theme
	settings := snapshot.Settings.Memory
	var status protocol.MemoryStatus
	if snapshot.Memory != nil {
		status = *snapshot.Memory
	}
	m.engine.Labels = []string{engineName(protocol.MemoryBuiltin), engineName(protocol.MemoryHindsight), engineName(protocol.MemoryHoncho)}
	for index, engine := range engines {
		if engine == m.picked {
			m.engine.Selected = index
		}
	}
	m.folder.Kind, m.folder.Small, m.folder.Label = kit.PlaqueQuiet, true, i18n.T("Show in files")
	m.test.Kind, m.test.Small, m.test.Label = kit.PlaqueTonal, true, i18n.T("Test")
	m.setup.Kind, m.setup.Small, m.setup.Label = kit.PlaqueTonal, true, i18n.T("Set up on this computer")
	m.move.Kind, m.move.Small, m.move.Label = kit.PlaqueTonal, true, i18n.Tf("Move memory to %s", engineName(m.picked))
	picked := m.picked
	rows := []layout.Widget{
		func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
						layout.Rigid(look.Paragraph(th, th.T.Row, th.R.Fg, i18n.T("Kept by"))),
						layout.Rigid(layout.Spacer{Width: th.Sp.S3}.Layout),
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							if !leaves(settings, picked) {
								return layout.Dimensions{}
							}
							return kit.Claim(th.Theme, palette.Attention, i18n.T("Leaves this computer"))(gtx)
						}),
					)
				}),
				layout.Rigid(layout.Spacer{Height: th.Sp.S2}.Layout),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					gtx.Constraints.Min.X = min(gtx.Constraints.Max.X, gtx.Dp(360))
					return look.Named(gtx, i18n.T("Kept by"), func(gtx layout.Context) layout.Dimensions { return m.engine.Layout(gtx, th.Theme) })
				}),
				layout.Rigid(layout.Spacer{Height: th.Sp.S2}.Layout),
				layout.Rigid(look.Paragraph(th, th.T.Detail, th.R.Fg2, engineDetail(picked))),
			)
		},
		func(gtx layout.Context) layout.Dimensions { return m.layoutEngineFacts(gtx, e, settings, status) },
	}
	if picked == protocol.MemoryHoncho {
		rows = append(rows, func(gtx layout.Context) layout.Dimensions { return m.layoutHoncho(gtx, e, snapshot) })
	}
	if setup := m.setupRow(e, snapshot, picked); setup != nil {
		rows = append(rows, setup)
	}
	rows = append(rows, func(gtx layout.Context) layout.Dimensions { return m.layoutEngineActs(gtx, th, settings, picked) })
	return func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			layout.Rigid(kit.Text(th.Theme, th.T.Title, th.R.Fg, i18n.T("Engine"))),
			layout.Rigid(layout.Spacer{Height: th.Sp.S2}.Layout),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions { return kit.SlabRows(gtx, th.Theme, rows...) }),
		)
	}
}

func (m *memoryPanel) layoutEngineFacts(gtx layout.Context, e *env.Env, settings protocol.MemorySettings, status protocol.MemoryStatus) layout.Dimensions {
	th := e.Theme
	path := status.Path
	if path == "" {
		path = i18n.T("not made yet")
	}
	rows := []look.FactRow{
		{Label: i18n.T("Kept in"), Widget: func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
				layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
					return kit.Label{Style: th.T.Mono, Color: th.R.Fg, MaxLines: 1}.Layout(gtx, th.Theme, paths.Short(path))
				}),
				layout.Rigid(layout.Spacer{Width: th.Sp.S2}.Layout),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions { return m.folder.Layout(gtx, th.Theme) }),
			)
		}},
		{Label: i18n.T("Size"), Value: i18n.Tf("%d memories, %s", status.Counts.All, humanBytes(status.Bytes))},
	}
	if settings.Engine == m.picked && status.EngineDetail != "" {
		rows = append(rows, look.FactRow{Label: i18n.T("State"), Value: status.EngineDetail})
	}
	return look.Facts(gtx, th, rows)
}

// Honcho's address is typed here; its key is pasted once and kept in the vault, never shown
func (m *memoryPanel) layoutHoncho(gtx layout.Context, e *env.Env, snapshot state.Snapshot) layout.Dimensions {
	th := e.Theme
	honcho := snapshot.Settings.Memory.Honcho
	if honcho.URL != m.addressFilled {
		m.addressFilled = honcho.URL
		m.address.SetText(honcho.URL)
	}
	m.address.Label, m.address.Placeholder, m.address.Mono = i18n.T("Address"), "http://127.0.0.1:8000", true
	m.saveAddress.Kind, m.saveAddress.Small, m.saveAddress.Label = kit.PlaqueTonal, true, i18n.T("Save address")
	m.key.Label, m.key.Secret = i18n.T("Key"), true
	m.key.Help = i18n.T("Rashin keeps it in the vault and only ever names it.")
	if honcho.URL == "" {
		m.key.Help = i18n.T("Save the address first: the key may only go there.")
	}
	m.storeKey.Kind, m.storeKey.Small, m.storeKey.Label = kit.PlaqueTonal, true, i18n.T("Keep in the vault")
	m.replaceKey.Kind, m.replaceKey.Small, m.replaceKey.Label = kit.PlaqueQuiet, true, i18n.T("Replace")
	_, stored := vaultCredential(snapshot.Vault, honcho.Key)
	stored = stored && honcho.Key != ""
	field := func(control *kit.TextField, act *kit.Plaque, changed bool) layout.Widget {
		return func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Axis: layout.Horizontal, Alignment: layout.End}.Layout(gtx,
				layout.Flexed(1, func(gtx layout.Context) layout.Dimensions { return control.Layout(gtx, th.Theme) }),
				layout.Rigid(layout.Spacer{Width: th.Sp.S2}.Layout),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					if !changed {
						gtx = gtx.Disabled()
					}
					return act.Layout(gtx, th.Theme)
				}),
			)
		}
	}
	key := field(&m.key, &m.storeKey, strings.TrimSpace(m.key.Text()) != "" && honcho.URL != "")
	if stored && !m.replacing {
		key = func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Baseline}.Layout(gtx,
				layout.Rigid(look.Paragraph(th, th.T.Detail, th.R.Fg2, i18n.T("Key from the vault:"))),
				layout.Rigid(layout.Spacer{Width: th.Sp.S1}.Layout),
				layout.Flexed(1, kit.Text(th.Theme, th.T.Mono, th.R.Fg, honcho.Key)),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions { return m.replaceKey.Layout(gtx, th.Theme) }),
			)
		}
	}
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(field(&m.address, &m.saveAddress, strings.TrimSpace(m.address.Text()) != honcho.URL)),
		layout.Rigid(layout.Spacer{Height: th.Sp.S3}.Layout),
		layout.Rigid(key),
	)
}

// Hindsight on this computer has to be set up once; its progress arrives step by step
func (m *memoryPanel) setupRow(e *env.Env, snapshot state.Snapshot, picked string) layout.Widget {
	th := e.Theme
	progress := snapshot.MemorySetup
	if progress != nil && progress.Engine != picked {
		progress = nil
	}
	settings := snapshot.Settings.Memory
	// only a local engine that still waits for its setup offers it; a finished one never does again
	waits := picked == protocol.MemoryHindsight && settings.Hindsight.Local && snapshot.Memory != nil &&
		snapshot.Memory.Engine == picked && snapshot.Memory.EngineState == protocol.EngineSetup
	if progress == nil && !waits {
		return nil
	}
	return func(gtx layout.Context) layout.Dimensions {
		switch {
		case progress == nil:
			return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
				layout.Flexed(1, look.Paragraph(th, th.T.Detail, th.R.Fg2, i18n.Tf("%s runs from its own folder. Setting it up fetches it and its models once.", engineName(picked)))),
				layout.Rigid(layout.Spacer{Width: th.Sp.S3}.Layout),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions { return m.setup.Layout(gtx, th.Theme) }),
			)
		case progress.Done && progress.Error != "":
			return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
				layout.Flexed(1, look.Paragraph(th, th.T.Detail, th.R.Jasper, i18n.Tf("Setting up failed: %s", progress.Error))),
				layout.Rigid(layout.Spacer{Width: th.Sp.S3}.Layout),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions { return m.setup.Layout(gtx, th.Theme) }),
			)
		case progress.Done:
			return look.Paragraph(th, th.T.Detail, th.R.Fg2, i18n.Tf("%s is set up on this computer.", engineName(progress.Engine)))(gtx)
		}
		fraction := 0.0
		if progress.Parts > 0 {
			fraction = float64(progress.Part) / float64(progress.Parts)
		}
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			layout.Rigid(look.Paragraph(th, th.T.Detail, th.R.Fg2, i18n.Tf("Setting up, step %d of %d: %s", progress.Part, progress.Parts, progress.Step))),
			layout.Rigid(layout.Spacer{Height: th.Sp.S2}.Layout),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return kit.BladeProgress{Fraction: fraction, Active: true}.Layout(gtx, th.Theme)
			}),
		)
	}
}

func (m *memoryPanel) layoutEngineActs(gtx layout.Context, th *look.Theme, settings protocol.MemorySettings, picked string) layout.Dimensions {
	children := []layout.FlexChild{
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return look.Named(gtx, i18n.Tf("Test %s", engineName(picked)), func(gtx layout.Context) layout.Dimensions { return m.test.Layout(gtx, th.Theme) })
		}),
	}
	if picked != settings.Engine {
		children = append(children,
			layout.Rigid(layout.Spacer{Width: th.Sp.S2}.Layout),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions { return m.move.Layout(gtx, th.Theme) }),
		)
	}
	if m.tested != "" && m.testedEngine == picked {
		children = append(children,
			layout.Rigid(layout.Spacer{Width: th.Sp.S3}.Layout),
			layout.Flexed(1, look.Paragraph(th, th.T.Detail, th.R.Fg2, m.tested)),
		)
	}
	return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx, children...)
}

func (m *memoryPanel) importGroup(th *look.Theme) layout.Widget {
	m.importing.Kind, m.importing.Small, m.importing.Label = kit.PlaqueTonal, true, i18n.T("Import")
	return teamsGroup(th, i18n.T("Bring in"), "", []look.Setting{{
		Title:  i18n.T("Import from another assistant"),
		Detail: i18n.T("Paste what another assistant says it remembers about you. Each thing waits under Suggested until you keep it."),
		Control: func(gtx layout.Context) layout.Dimensions {
			return look.Named(gtx, i18n.T("Import from another assistant"), func(gtx layout.Context) layout.Dimensions { return m.importing.Layout(gtx, th.Theme) })
		},
	}}, nil)
}

// the section's dialogs: forgetting everything, importing, and moving to another engine
func (m *memoryPanel) overlay(gtx layout.Context, e *env.Env) bool {
	open := m.forgetAll.Layout(gtx, e)
	open = m.importer.Layout(gtx, e) || open
	if m.movingTo == "" {
		return open
	}
	snapshot := e.Snapshot()
	count := 0
	if snapshot.Memory != nil {
		count = snapshot.Memory.Counts.All
	}
	settings := snapshot.Settings.Memory
	name, from := engineName(m.movingTo), engineName(settings.Engine)
	var body string
	switch {
	case m.movingTo == protocol.MemoryBuiltin:
		body = i18n.Tf("Rashin copies your %s memories back into its own files and uses them from now on. %s keeps its copy until you forget it there.", look.Count(count), from)
	case settings.Engine == protocol.MemoryBuiltin || settings.Engine == "":
		body = i18n.Tf("Rashin copies your %s memories to %s and uses it from now on. The built-in files stay until you forget them.", look.Count(count), name)
	default:
		body = i18n.Tf("Rashin copies your %s memories to %s and uses it from now on. %s keeps its copy until you forget it there.", look.Count(count), name, from)
	}
	if leaves(settings, m.movingTo) {
		address := settings.Honcho.URL
		if m.movingTo == protocol.MemoryHindsight {
			address = settings.Hindsight.URL
		}
		if parsed, err := url.Parse(address); err == nil && parsed.Host != "" {
			address = parsed.Host
		}
		body += " " + i18n.Tf("What you remember is sent to %s.", address)
	}
	if m.moving.Layout(gtx, e.Theme.Theme, i18n.Tf("Move memory to %s?", name), body, i18n.T("Move"), "") {
		engine := m.movingTo
		var status protocol.MemoryStatus
		e.Call(protocol.MMemoryEngineMove, protocol.MemoryEngineParams{Engine: engine}, &status, func(err error) {
			if err != nil {
				e.Report(err)
				return
			}
			e.Notify(i18n.Tf("Memory is kept by %s now.", engineName(engine)))
		})
	}
	if m.moving.Closed() {
		m.movingTo = ""
	}
	return m.moving.Dialog.Open || open
}
