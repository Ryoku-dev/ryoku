// Package settings is the Settings page: whether Hyakki runs, the providers
// and local models it routes through, how Ask and the agents behave, what is
// indexed and how the app wears the desktop's look.
package settings

import (
	"image"
	"slices"
	"strings"

	"gioui.org/io/key"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/unit"
	"gioui.org/widget"

	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/kit"
	"ryoku-i18n"

	"ryoku-rashin/internal/app/env"
	"ryoku-rashin/internal/app/look"
	"ryoku-rashin/internal/config"
	"ryoku-rashin/internal/protocol"
)

const (
	sectionRashin = iota
	sectionFeatures
	sectionProviders
	sectionAPI
	sectionAsk
	sectionAgents
	sectionTeams
	sectionMemory
	sectionSkills
	sectionIndex
	sectionOni
	// Hands stands under Oni: a page of its own, as Oni is longer than a screen
	sectionHands
	sectionAppearance
	sectionKeys
	sectionOniVoice
	sectionOniScreen
	sectionOniCommands
	sectionOniWriting
	sectionOniPrivacy
	sectionParts
	sectionCount
)

// the least a select takes beside its row's words, so its value never wraps to a second line
const selectWidth unit.Dp = 260

// every control applies as it changes, there's nothing to save
type Page struct {
	header look.PaneHeader
	scroll look.Scroller
	blocks blockList
	// the registration sheet and the folio under the column, the colophon at its foot
	sheet  look.Sheet
	reveal look.Reveal
	// the sections stand in the sidebar's list with a search over them, a picker tops the column while the sidebar can't show them
	sections look.SideList
	find     kit.Search
	index    look.Select
	listed   bool
	current  int
	// the section just picked, until the list has moved to it
	jumped    bool
	jumpBlock int

	// what a machine that never changed a setting has, and what differs from it this frame
	defaults protocol.Settings
	changed  changes

	turnOff  kit.Plaque
	off      kit.Confirm
	offOpen  bool
	log      kit.Plaque
	tour     kit.Plaque
	parts    partsBlock
	privacy  privacyBlock
	features featuresPanel

	providers map[string]*providerRow
	open      string
	// provider ids in the order they were sorted into when Settings opened
	providerOrder []string
	// the one control that brings a provider nobody set up into the list
	addProvider look.Select
	// providers the reader picked from Add a provider, shown until set up
	revealed      map[string]bool
	usage         env.Usage
	limits        env.LimitsWatch
	local         localPanel
	fastDecisions fastDecisionPanel
	api           apiPanel

	askModel env.ModelForm
	// the model the form was last filled from, so a pick stands until the daemon has it
	askFilled  bool
	askStored  string
	askMay     look.Select
	habits     kit.Toggle
	agentModel env.ModelForm
	plan       kit.Toggle
	approvals  look.Select
	idle       kit.Stepper
	notices    kit.Toggle
	teams      teamsPanel
	memory     memoryPanel
	skills     skillsPanel
	prompts    promptsBlock
	mcp        mcpBlock
	disk       diskBlock
	profile    profileBlock
	resume     resumeBlock
	oni        oniPanel
	hands      handsPanel

	system   kit.Toggle
	semantic kit.Toggle

	appearance appearancePanel
}

func New() *Page {
	return &Page{providers: make(map[string]*providerRow), defaults: config.Defaults()}
}

// a name it doesn't know leaves the page where it is
func (p *Page) OpenSection(name string) {
	p.providerOrder = nil
	p.usage.Forget()
	p.limits.Forget()
	p.skills.list.Forget()
	p.reveal.Reset()
	p.oni.forget()
	p.hands.forget()
	p.parts.loaded = false
	p.privacy.loaded = false
	// the older names still lead to their section
	aliases := map[string]string{"about": "rashin", "agent": "agents", "keyboard": "keys"}
	if alias, ok := aliases[name]; ok {
		name = alias
	}
	all := sections()
	if at := slices.IndexFunc(all[:], func(item section) bool { return item.name == name }); at >= 0 {
		p.jump(at)
	}
}

// the question that stands before turning Hyakki off
func (p *Page) AskTurnOff() { p.offOpen, p.off.Dialog.Open, p.off.Danger = true, true, true }

func (p *Page) jump(section int) {
	p.current, p.jumped, p.jumpBlock = section, true, 0
}

func (p *Page) ListShown(shown bool) { p.listed = shown }

func (p *Page) Sheet(e *env.Env) look.SheetPage {
	page := p.page()
	page.Facts = []string{i18n.Tf("%s sections", kit.Pad2(sectionCount))}
	if count := p.changed.count(); count > 0 {
		page.Facts = append(page.Facts, i18n.Tf("%s changed", kit.Pad2(count)))
	}
	return page
}

func (p *Page) SheetProgress() float32 {
	if separateSection(p.current) {
		return p.scroll.Progress(len(p.blocks.built[p.current]) + 1)
	}
	return p.scroll.Progress(p.blocks.total() + 1)
}

// settings lists its own sections while it is open, the New plaque still belongs to where it was opened from
func (p *Page) List(gtx layout.Context, e *env.Env) look.ListDimensions {
	th := e.Theme
	all := sections()
	p.find.Submit = true
	p.find.Placeholder, p.find.Hint = i18n.T("Find a setting"), "Ctrl F"
	// down from the search walks the sections it left, asked before the editor takes the key
	for {
		event, ok := gtx.Event(key.Filter{Focus: &p.find.Editor, Name: key.NameDownArrow})
		if !ok {
			break
		}
		if pressed, ok := event.(key.Event); ok && pressed.State == key.Press {
			p.sections.Focus(gtx)
		}
	}
	for {
		event, ok := p.find.Editor.Update(gtx)
		if !ok {
			break
		}
		if submitted, ok := event.(widget.SubmitEvent); ok {
			if first := slices.IndexFunc(all[:], func(item section) bool { return item.matches(submitted.Text) }); first >= 0 {
				p.jump(first)
			}
		}
	}
	query := p.find.Text()
	size := gtx.Constraints.Max
	field := look.Place(gtx, image.Rectangle{Max: size}, func(gtx layout.Context) layout.Dimensions {
		gtx.Constraints.Min = image.Point{}
		return p.find.Layout(gtx, th.Theme)
	})
	top := field.Size.Y + gtx.Dp(th.Sp.S3)

	groups := make([]look.SideGroup, 0, 4)
	for _, title := range []string{i18n.T("Hyakki"), i18n.T("Work"), i18n.T("Oni"), i18n.T("Optional")} {
		rows := make([]look.SideRow, 0, sectionCount)
		for _, item := range all {
			if item.group != title || !item.matches(query) {
				continue
			}
			depth := 0
			if item.under >= 0 {
				depth = 1
			}
			rows = append(rows, look.SideRow{ID: item.name, Title: item.label, Icon: item.icon, Depth: depth})
		}
		if len(rows) > 0 {
			groups = append(groups, look.SideGroup{Title: title, Rows: rows})
		}
	}
	p.sections.Empty = i18n.T("No section matches what you typed.")
	list := gtx
	list.Constraints = layout.Exact(image.Pt(size.X, max(0, size.Y-top)))
	offset := op.Offset(image.Pt(0, top)).Push(gtx.Ops)
	dimensions := p.sections.Layout(list, th, e.Overlays, groups, all[p.current].name)
	offset.Pop()
	for _, event := range p.sections.Events() {
		if at := slices.IndexFunc(all[:], func(item section) bool { return item.name == event.ID }); event.Kind == look.SideOpen && at >= 0 {
			p.jump(at)
		}
	}
	dimensions.Selected = dimensions.Selected.Add(image.Pt(0, top))
	dimensions.Size = size
	return dimensions
}

// settings makes nothing new
func (p *Page) New(*env.Env) {}

func (p *Page) NewLabel() string { return "" }

func (p *Page) Overlay(gtx layout.Context, e *env.Env) bool {
	// runs every frame, Settings shown or not: the guide's watch stays open
	// only while its panel was drawn this frame
	p.oni.guide.settle(gtx, e)
	p.oni.writing.approvals.settle(gtx, e)
	p.hands.settle(gtx, e)
	if p.oni.ask.overlay(gtx, e) || p.oni.guide.overlay(gtx, e) || p.oni.voice.overlay(gtx, e) || p.oni.dictation.overlay(gtx, e) || p.oni.reflex.overlay(gtx, e) || p.hands.overlay(gtx, e) || p.local.overlay(gtx, e) || p.api.overlay(gtx, e) || p.memory.overlay(gtx, e) || p.mcp.overlay(gtx, e) || p.disk.overlay(gtx, e) || p.parts.overlay(gtx, e) || p.privacy.overlay(gtx, e) {
		return true
	}
	if !p.offOpen {
		return false
	}
	body := i18n.T("Its daemon stops, every agent stops with it, and nothing runs until you turn it on again. Your sessions and settings stay.")
	if p.off.Layout(gtx, e.Theme.Theme, i18n.T("Turn Hyakki off?"), body, i18n.T("Turn off"), "") {
		e.Hyakki(nil, "off")
	}
	if p.off.Closed() {
		p.offOpen = false
	}
	return p.off.Dialog.Open
}

type section struct {
	// as OpenSection and the routes name it
	name  string
	label string
	icon  *widget.Icon
	// words beside its name that find it, translated as one line
	keywords string
	// the section it stands under in the sidebar, -1 for none
	under int
	group string
}

func sections() [sectionCount]section {
	return [sectionCount]section{
		{"rashin", i18n.T("Hyakki"), look.Icon.Thought, i18n.T("on off turn daemon log about version tour onboarding"), -1, i18n.T("Hyakki")},
		{"features", i18n.T("Features"), look.Icon.Features, i18n.T("optional on off compare research schedules prompts enhance writing analyze audio video share quote history search steps mind maps artifacts collections profile setup cards mcp disk care learn skills decisions rules reports tools learning Theia desktop"), -1, i18n.T("Optional")},
		{"providers", i18n.T("Providers"), look.Icon.Graph, i18n.T("models routing route keys usage spend free paid local ollama llama.cpp llama-server address download gguf hugging face lm studio jan gpt4all cache found folder file choose build binary graphics card vulkan cuda idle light fast decisions classifier Ember"), -1, i18n.T("Hyakki")},
		{"api", i18n.T("Local API"), look.Icon.Link, i18n.T("address port key token exports openai"), -1, i18n.T("Hyakki")},
		{"ask", i18n.T("Ask"), look.Icon.Ask, i18n.T("answers model auto memory habits shell approvals changes resume history minutes pick up prompts library templates snippets variables"), -1, i18n.T("Work")},
		{"agents", i18n.T("Agents"), look.Icon.Agents, i18n.T("model default thinking effort plan first approvals asking notices idle stop mcp servers tools connections"), -1, i18n.T("Work")},
		{"teams", i18n.T("The Yokai"), look.Icon.Teams, i18n.T("teams model default ledger budget spend members sandbox mail hidden secrets quiet hours notifications disk care usage storage clean free space"), -1, i18n.T("Work")},
		{"memory", i18n.T("Memory"), look.Icon.Memory, i18n.T("memory remember forget learn temporary privacy Honcho Hindsight profile about me you name work traits answers brief detailed plain technical warm direct examples step by step"), -1, i18n.T("Work")},
		{"skills", i18n.T("Skills"), look.Icon.Skills, i18n.T("skills built in sets specs plans designs reviews debugging tests teams"), -1, i18n.T("Work")},
		{"index", i18n.T("Index"), look.Icon.Index, i18n.T("search meaning semantic system folders hide code"), -1, i18n.T("Work")},
		{"oni", i18n.T("Oni"), look.Icon.Member, i18n.T("character companion desktop presence picker preview size small big reduce motion still courtesy full screen corner pin reset moth perch"), -1, i18n.T("Oni")},
		{"hands", i18n.T("Hands"), look.Icon.Screen, i18n.T("oni hands browser chromium chrome tab authorize goal task start stop resume profile forget sign in log steps"), sectionOni, i18n.T("Oni")},
		{"appearance", i18n.T("Appearance"), look.Icon.Show, i18n.T("look style design lithic material adwaita fluent theme colours colors wallpaper dark light contrast tint motion gloss zoom desktop Ryoku Settings"), -1, i18n.T("Hyakki")},
		{"keys", i18n.T("Keyboard"), look.Icon.Key, i18n.T("keys shortcuts keybinds"), -1, i18n.T("Hyakki")},
		{"oni-voice", i18n.T("Voice and dictation"), look.Icon.Member, i18n.T("oni voice talk microphone mic test chord speak replies speech model download wake phrase open continuous dictation dictate dictionary snippets styles polish"), sectionOni, i18n.T("Oni")},
		{"oni-screen", i18n.T("Screen and guides"), look.Icon.Screen, i18n.T("oni look picture screen ask consent disclosure focus inspect guide kept walkthroughs replay hints"), sectionOni, i18n.T("Oni")},
		{"oni-commands", i18n.T("Quick commands"), look.Icon.Key, i18n.T("oni reflex commands reminders timers routines log notes voice remember forget"), sectionOni, i18n.T("Oni")},
		{"oni-writing", i18n.T("Writing and memory"), look.Icon.Memory, i18n.T("oni memory recall rewrite assignment focus guard feeds words voice imports vault"), sectionOni, i18n.T("Oni")},
		{"oni-privacy", i18n.T("Privacy and tools"), look.Icon.Link, i18n.T("oni privacy activity clear delete records mcp tools clients approvals optional parts"), sectionOni, i18n.T("Oni")},
		{"parts", i18n.T("Optional parts"), look.Icon.Features, i18n.T("oni parts programs models voice packs remove storage disk size dependencies"), -1, i18n.T("Optional")},
	}
}

// title is a section's name where it stands alone, a sub-page's with the
// section it stands under: "Oni › Hands"
func (item section) title() string {
	if item.under < 0 {
		return item.label
	}
	return look.Crumb(sections()[item.under].label, item.label)
}

// every word typed has to start a word of the section's name or keywords
func (item section) matches(query string) bool {
	words := strings.Fields(strings.ToLower(item.label + " " + item.keywords))
	for _, typed := range strings.Fields(strings.ToLower(query)) {
		if !slices.ContainsFunc(words, func(word string) bool { return strings.HasPrefix(word, typed) }) {
			return false
		}
	}
	return true
}

func (p *Page) Content(gtx layout.Context, e *env.Env) layout.Dimensions {
	th := e.Theme
	for {
		event, ok := gtx.Event(key.Filter{Name: "F", Required: key.ModShortcut})
		if !ok {
			break
		}
		if pressed, ok := event.(key.Event); ok && pressed.State == key.Press && p.listed {
			p.find.Focus(gtx)
		}
	}
	p.changed = changesOf(e.Snapshot().Settings, p.defaults, e)
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			scrolled := p.scroll.Position.First > 0 || p.scroll.Position.Offset > 0
			return p.header.Layout(gtx, th, e.Overlays, look.PaneHeaderContent{Title: i18n.T("Settings"), Scrolled: scrolled})
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			specs := []look.Spec{
				{Label: i18n.T("Sections"), Value: kit.Pad2(sectionCount)},
				{Label: i18n.T("Changed"), Value: kit.Pad2(p.changed.count()), Lit: p.changed.count() > 0},
			}
			return layout.Inset{Top: th.Sp.S2, Left: th.Sp.S5}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				return look.SpecStrip(gtx, th, specs)
			})
		}),
		layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
			return p.sheet.Layout(gtx, th, p.page(), func(gtx layout.Context) layout.Dimensions { return p.layoutBody(gtx, e) })
		}),
	)
}

// the section being read as a part of Settings, which numbers the folio and the colophon
func (p *Page) page() look.SheetPage {
	return look.PartPage(protocol.TabSettings, p.current+1, sections()[p.current].title(), "")
}

func (p *Page) layoutBody(gtx layout.Context, e *env.Env) layout.Dimensions {
	th := e.Theme
	size := gtx.Constraints.Max
	side, column := gtx.Dp(th.Sp.S5), gtx.Dp(look.SettingsColumn)
	if p.oni.toHands {
		p.oni.toHands = false
		p.jump(sectionHands)
	}
	if p.oni.navigate {
		p.oni.navigate = false
		p.jump(p.oni.destination)
	}
	p.follow()
	width := min(column, size.X-2*side)
	left := (size.X - width) / 2
	top := 0
	// the sections stand in the sidebar while it is open, otherwise a picker tops the column
	if !p.listed {
		all := sections()
		labels := make([]string, 0, sectionCount)
		for _, item := range all {
			labels = append(labels, item.title())
		}
		gap := gtx.Dp(th.Sp.S3)
		top = look.Place(gtx, image.Rect(left, gap, left+width, size.Y), func(gtx layout.Context) layout.Dimensions {
			gtx.Constraints.Min = image.Pt(gtx.Constraints.Max.X, 0)
			return p.index.Layout(gtx, th, e.Overlays, labels, p.current)
		}).Size.Y + gap
		p.choose()
	}
	look.Place(gtx, image.Rect(0, top, size.X, size.Y), func(gtx layout.Context) layout.Dimensions {
		return p.layoutSections(gtx, e, left, width)
	})
	return layout.Dimensions{Size: size}
}

// follows the section at the top of the column, unless one was just picked and the list is still on its way
func (p *Page) follow() {
	if p.jumped || separateSection(p.current) {
		return
	}
	first, _ := p.blocks.locate(max(p.scroll.Position.First, 0))
	p.current = documentSection(first)
}

func (p *Page) choose() {
	if selected, ok := p.index.Changed(); ok && selected < sectionCount {
		p.jump(selected)
	}
}

func set(e *env.Env, section string, values map[string]any) {
	e.SetSettings(map[string]any{section: values}, func(err error) {
		if err != nil {
			e.Notify(err.Error())
		}
	})
}
