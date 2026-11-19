// Package learning shows everything Rashin learned and lets its owner remove it.
package learning

import (
	"path/filepath"
	"strings"

	"gioui.org/layout"
	"gioui.org/unit"

	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/kit"
	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/tokens"
	"ryoku-i18n"

	"ryoku-rashin/internal/app/env"
	"ryoku-rashin/internal/app/look"
	"ryoku-rashin/internal/app/state"
	"ryoku-rashin/internal/protocol"
)

const (
	sectionSkills    = "skills"
	sectionDecisions = "decisions"
	sectionRules     = "rules"
	sectionReports   = "reports"
)

var sections = []string{sectionSkills, sectionDecisions, sectionRules, sectionReports}

func sectionTitle(id string) string {
	switch id {
	case sectionSkills:
		return i18n.T("Skills it learned")
	case sectionDecisions:
		return i18n.T("Project decisions")
	case sectionRules:
		return i18n.T("Rules")
	case sectionReports:
		return i18n.T("Tool reports")
	}
	return i18n.T("Learning")
}

func sectionFeature(id string) string {
	switch id {
	case sectionSkills:
		return protocol.FeatureAutolearn
	case sectionDecisions:
		return protocol.FeatureDecisions
	case sectionRules:
		return protocol.FeatureRules
	case sectionReports:
		return protocol.FeatureGrievances
	}
	return ""
}

type Page struct {
	header    look.PaneHeader
	index     look.SideList
	scroll    look.Scroller
	chooser   look.Select
	paper     look.Sheet
	features  [4]kit.Plaque
	rise      look.Reveal
	selected  string
	listed    bool
	loaded    bool
	epoch     int
	changes   state.Learning
	skills    skillsSection
	decisions decisionsSection
	rules     rulesSection
	reports   reportsSection
}

func New() *Page { return &Page{} }

func (p *Page) Open(e *env.Env, section string) {
	if section != "" && !validSection(section) {
		section = ""
	}
	if p.selected != section {
		p.selected = section
		p.scroll.Position = layout.Position{}
		p.rise.Reset()
	}
	p.load(e)
}

func validSection(section string) bool {
	for _, id := range sections {
		if id == section {
			return true
		}
	}
	return false
}

func (p *Page) Sheet(e *env.Env) look.SheetPage {
	page := look.PageFor(protocol.TabLearning)
	if p.selected != "" {
		for index, id := range sections {
			if id == p.selected {
				return look.PartPage(protocol.TabLearning, index+1, sectionTitle(id), "")
			}
		}
	}
	return page
}

func (p *Page) New(*env.Env)         {}
func (p *Page) NewLabel() string     { return "" }
func (p *Page) ListShown(shown bool) { p.listed = shown }

func (p *Page) List(gtx layout.Context, e *env.Env) look.ListDimensions {
	rows := make([]look.SideRow, 0, len(sections)+1)
	rows = append(rows, look.SideRow{ID: "all", Title: i18n.T("All learning")})
	for _, id := range sections {
		rows = append(rows, look.SideRow{ID: id, Title: sectionTitle(id), Depth: 1})
	}
	selected := p.selected
	if selected == "" {
		selected = "all"
	}
	dim := p.index.Layout(gtx, e.Theme, e.Overlays, []look.SideGroup{{Rows: rows}}, selected)
	for _, event := range p.index.Events() {
		if event.Kind == look.SideOpen {
			id := event.ID
			if id == "all" {
				id = ""
			}
			p.Open(e, id)
		}
	}
	return dim
}

func (p *Page) Overlay(layout.Context, *env.Env) bool { return false }

func (p *Page) Content(gtx layout.Context, e *env.Env) layout.Dimensions {
	p.load(e)
	th := e.Theme
	parts := []string{}
	for _, id := range sections {
		if p.selected == "" || p.selected == id {
			parts = append(parts, id)
		}
	}
	items := make([]layout.Widget, 0, len(parts)+3)
	if !p.listed {
		items = append(items, func(gtx layout.Context) layout.Dimensions {
			labels := []string{i18n.T("All learning")}
			for _, section := range sections {
				labels = append(labels, sectionTitle(section))
			}
			selected := 0
			for n, section := range sections {
				if section == p.selected {
					selected = n + 1
				}
			}
			if next, changed := p.chooser.Changed(); changed && next >= 0 && next <= len(sections) {
				section := ""
				if next > 0 {
					section = sections[next-1]
				}
				p.Open(e, section)
			}
			p.chooser.Label = i18n.T("Learning section")
			return p.chooser.Layout(gtx, th, e.Overlays, labels, selected)
		})
	}
	if p.selected == "" {
		items = append(items, func(gtx layout.Context) layout.Dimensions {
			return layout.Inset{Top: th.Sp.S5, Bottom: th.Sp.S3}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				return look.Monument(gtx, th, i18n.T("What Rashin has learned"), i18n.T("Skills, decisions and rules stay on this computer. Review or remove anything it kept."))
			})
		})
		items = append(items, func(gtx layout.Context) layout.Dimensions {
			count := 0
			for _, id := range sections {
				if e.Feature(sectionFeature(id)) {
					count++
				}
			}
			return look.SpecStrip(gtx, th, []look.Spec{{Label: i18n.T("Sections"), Value: kit.Pad2(len(sections))}, {Label: i18n.T("On"), Value: kit.Pad2(count), Lit: true}})
		})
	} else {
		items = append(items, func(gtx layout.Context) layout.Dimensions {
			return layout.Spacer{Height: th.Sp.S4}.Layout(gtx)
		})
	}
	for index, id := range parts {
		id, index := id, index
		items = append(items, func(gtx layout.Context) layout.Dimensions {
			return p.rise.Group(gtx, th, index, func(gtx layout.Context) layout.Dimensions { return p.section(gtx, e, id) })
		})
	}
	items = append(items, func(gtx layout.Context) layout.Dimensions { return p.paper.Colophon(gtx, th, p.Sheet(e)) })
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return p.header.Layout(gtx, th, e.Overlays, look.PaneHeaderContent{Title: i18n.T("Learning"), Scrolled: p.scroll.Position.First > 0 || p.scroll.Position.Offset > 0})
		}),
		layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
			return p.paper.Layout(gtx, th, p.Sheet(e), func(gtx layout.Context) layout.Dimensions {
				return p.scroll.Layout(gtx, th, len(items), func(gtx layout.Context, index int) layout.Dimensions {
					return look.Centred(gtx, e.Class, unit.Dp(tokens.SizeSettingsMediumMax), items[index])
				})
			})
		}),
	)
}

func (p *Page) section(gtx layout.Context, e *env.Env, id string) layout.Dimensions {
	th := e.Theme
	n := 0
	for index, entry := range sections {
		if entry == id {
			n = index + 1
		}
	}
	return look.Section(gtx, th, look.Head{Title: sectionTitle(id), Number: n, Of: len(sections)}, func(gtx layout.Context) layout.Dimensions {
		content := []layout.FlexChild{}
		if !e.Feature(sectionFeature(id)) {
			content = append(content, layout.Rigid(func(gtx layout.Context) layout.Dimensions { return p.off(gtx, e, id) }))
		}
		content = append(content, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			switch id {
			case sectionSkills:
				return p.skills.layout(gtx, e)
			case sectionDecisions:
				return p.decisions.layout(gtx, e)
			case sectionRules:
				return p.rules.layout(gtx, e)
			case sectionReports:
				return p.reports.layout(gtx, e)
			}
			return layout.Dimensions{}
		}))
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx, content...)
	})
}

func (p *Page) off(gtx layout.Context, e *env.Env, id string) layout.Dimensions {
	detail := ""
	switch id {
	case sectionSkills:
		detail = i18n.T("Rashin is not learning skills from its work. Anything learned earlier is still here to review or remove.")
	case sectionDecisions:
		detail = i18n.T("Rashin is not learning project decisions. Earlier decisions remain here until you remove them.")
	case sectionRules:
		detail = i18n.T("Rules are off. Existing rules can still be read, edited or removed.")
	case sectionReports:
		detail = i18n.T("Rashin is not collecting tool reports. Earlier reports remain here until you clear them.")
	}
	return layout.Inset{Bottom: e.Theme.Sp.S4}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return look.SettingsSlab(gtx, e.Theme, []look.Setting{{Title: i18n.T("This feature is off"), Detail: detail, Control: func(gtx layout.Context) layout.Dimensions {
			var index int
			for n, section := range sections {
				if section == id {
					index = n
					break
				}
			}
			button := &p.features[index]
			if button.Clicked(gtx) {
				e.OpenSettings("features")
			}
			button.Kind, button.Small, button.Label = kit.PlaqueTonal, true, i18n.T("Open Features")
			return button.Layout(gtx, e.Theme.Theme)
		}}})
	})
}

func (p *Page) load(e *env.Env) {
	snapshot := e.Snapshot()
	changes := e.Store.Learning()
	if !p.loaded || p.epoch != snapshot.Epoch || changes.Skills != p.changes.Skills {
		p.skills.load(e)
	}
	if !p.loaded || p.epoch != snapshot.Epoch || changes.Decisions != p.changes.Decisions {
		p.decisions.load(e)
	}
	if !p.loaded || p.epoch != snapshot.Epoch || changes.Rules != p.changes.Rules {
		p.rules.load(e)
	}
	if !p.loaded || p.epoch != snapshot.Epoch || changes.Grievances != p.changes.Grievances {
		p.reports.load(e)
	}
	p.loaded, p.epoch, p.changes = true, snapshot.Epoch, changes
}

func projectName(path string) string {
	name := filepath.Base(strings.TrimRight(path, "/"))
	if name == "." || name == "/" {
		return path
	}
	return name
}
