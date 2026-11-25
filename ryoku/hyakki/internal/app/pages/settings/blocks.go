package settings

import (
	"image"

	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/unit"

	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/kit"
	"ryoku-i18n"

	"ryoku-rashin/internal/app/env"
	"ryoku-rashin/internal/app/look"
)

// how many blocks rise as Settings opens, the Hub's first five
const revealed = 5

// a long section stands in the list as its blocks, laying out all of Teams for the screenful shown is what made scrolling lag
type blockList struct {
	frame   int
	builtAt [sectionCount]int
	built   [sectionCount][]layout.Widget
	// counted when last built, so an index keeps meaning the same block until then
	extra     [sectionCount]int
	moved     bool
	featureAt map[string]int
	// how many blocks this frame has drawn, which orders the ones that rise
	drawn int
}

func (b *blockList) total() int {
	total := sectionCount
	for _, extra := range b.extra {
		total += extra
	}
	return total
}

func (b *blockList) first(section int) int {
	index := section
	for _, extra := range b.extra[:section] {
		index += extra
	}
	return index
}

func (b *blockList) locate(index int) (section, block int) {
	for section := range sectionCount {
		count := 1 + b.extra[section]
		if index < count {
			return section, index
		}
		index -= count
	}
	return sectionCount - 1, b.extra[sectionCount-1]
}

func (p *Page) sectionBlocks(gtx layout.Context, e *env.Env, section int) []layout.Widget {
	b := &p.blocks
	if b.builtAt[section] == b.frame {
		return b.built[section]
	}
	blocks := b.built[section][:0]
	if b.featureAt == nil {
		b.featureAt = make(map[string]int)
	}
	switch section {
	case sectionFeatures:
		blocks = p.featuresBlocks(gtx, e, blocks)
	case sectionProviders:
		blocks = p.providerBlocks(gtx, e, blocks)
	case sectionAsk:
		blocks = append(blocks, func(gtx layout.Context) layout.Dimensions { return p.layoutAsk(gtx, e) })
		if e.Feature("history") {
			b.featureAt["history"] = len(blocks)
			blocks = p.resumeBlocks(gtx, e, blocks)
		}
		if e.Feature("prompts") {
			b.featureAt["prompts"] = len(blocks)
			blocks = p.promptsBlocks(gtx, e, blocks)
		}
	case sectionAgents:
		blocks = append(blocks, func(gtx layout.Context) layout.Dimensions { return p.layoutAgents(gtx, e) })
		if e.Feature("mcp") {
			b.featureAt["mcp"] = len(blocks)
			blocks = p.mcpBlocks(gtx, e, blocks)
		}
	case sectionTeams:
		blocks = p.teamsBlocks(gtx, e, blocks)
		if e.Feature("diskcare") {
			b.featureAt["diskcare"] = len(blocks)
			blocks = p.diskBlocks(gtx, e, blocks)
		}
	case sectionMemory:
		blocks = p.memoryBlocks(gtx, e, blocks)
		if e.Feature("profile") {
			b.featureAt["profile"] = len(blocks)
			blocks = p.profileBlocks(gtx, e, blocks)
		}
	case sectionSkills:
		blocks = p.skillsBlocks(gtx, e, blocks)
	case sectionOni:
		blocks = p.oniBlocks(gtx, e, blocks)
	case sectionHands:
		blocks = p.handsBlocks(gtx, e, blocks)
	case sectionOniVoice, sectionOniScreen, sectionOniCommands, sectionOniWriting, sectionOniPrivacy, sectionParts:
		blocks = p.oniSubpageBlocks(gtx, e, section, blocks)
	default:
		blocks = append(blocks, func(gtx layout.Context) layout.Dimensions { return p.layoutWhole(gtx, e, section) })
	}
	if extra := len(blocks) - 1; extra != b.extra[section] {
		b.extra[section], b.moved = extra, true
	}
	b.built[section], b.builtAt[section] = blocks, b.frame
	return blocks
}

// the gap between the groups of a section
func spacedBlocks(th *look.Theme, blocks []layout.Widget, groups ...layout.Widget) []layout.Widget {
	for _, group := range groups {
		blocks = append(blocks, func(gtx layout.Context) layout.Dimensions {
			return layout.Inset{Top: th.Sp.S5}.Layout(gtx, group)
		})
	}
	return blocks
}

// one of the sections by its name and its place among them, as the sidebar lists them
func ordinal(section int) look.Head {
	return look.Head{Title: sections()[section].title(), Number: section + 1, Of: sectionCount}
}

// the ruled head a section opens under, for a section whose blocks are its own
func sectionHead(gtx layout.Context, th *look.Theme, section int) layout.Dimensions {
	return look.SectionHead(th, ordinal(section))(gtx)
}

// the small head of a group inside a section, and what the group is about when that needs saying
func groupHead(th *look.Theme, title, caption string) layout.Widget {
	return func(gtx layout.Context) layout.Dimensions {
		children := []layout.FlexChild{layout.Rigid(look.SectionHead(th, look.Head{Title: title}))}
		if caption != "" {
			children = append(children,
				layout.Rigid(layout.Spacer{Height: th.Sp.S1}.Layout),
				layout.Rigid(look.Paragraph(th, th.T.Detail, th.R.Fg2, caption)),
			)
		}
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
	}
}

// a group's head, its slab of rows, and the save for what it holds when it holds anything to save
func group(th *look.Theme, title, caption string, rows []look.Setting, save layout.Widget) layout.Widget {
	return func(gtx layout.Context) layout.Dimensions {
		children := []layout.FlexChild{
			layout.Rigid(groupHead(th, title, caption)),
			layout.Rigid(layout.Spacer{Height: th.Sp.S2}.Layout),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions { return look.SettingsSlab(gtx, th, rows) }),
		}
		if save != nil {
			children = append(children, layout.Rigid(layout.Spacer{Height: th.Sp.S3}.Layout), layout.Rigid(save))
		}
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
	}
}

// the save under a group, dimmed while nothing in it changed
func groupSave(th *look.Theme, plaque *kit.Plaque, changed bool) layout.Widget {
	plaque.Kind, plaque.Small, plaque.Label = kit.PlaqueTonal, true, i18n.T("Save")
	return func(gtx layout.Context) layout.Dimensions {
		if !changed {
			gtx = gtx.Disabled()
		}
		return plaque.Layout(gtx, th.Theme)
	}
}

// read right after it is drawn: by the next frame the setting has overwritten the flip
func namedSwitch(gtx layout.Context, th *look.Theme, name string, toggle *kit.Toggle, flipped func(on bool)) layout.Dimensions {
	dimensions := look.Named(gtx, name, func(gtx layout.Context) layout.Dimensions { return toggle.Layout(gtx, th.Theme) })
	if toggle.Changed() && flipped != nil {
		flipped(toggle.Value)
	}
	return dimensions
}

func (p *Page) layoutWhole(gtx layout.Context, e *env.Env, section int) layout.Dimensions {
	switch section {
	case sectionRashin:
		return p.layoutRashin(gtx, e)
	case sectionAPI:
		return p.layoutAPI(gtx, e)
	case sectionIndex:
		return p.layoutIndex(gtx, e)
	case sectionAppearance:
		return p.layoutAppearance(gtx, e)
	default:
		return p.layoutKeys(gtx, e)
	}
}

// teams and memory count once built, skills by its sets which arrive later, so the list stays on the block it showed when a count moves
func (p *Page) prepare(gtx layout.Context, e *env.Env) {
	b, position := &p.blocks, &p.scroll.Position
	section, block := b.locate(max(position.First, 0))
	section = documentSection(section)
	if p.jumped {
		section, block = p.current, p.jumpBlock
	}
	// A flattened list index must know the structural blocks before its
	// destination before Gio measures backwards around that destination.
	// Otherwise materializing an earlier section moves the index underneath
	// the jump. Child pages are separate lists and are never prepared here.
	for each := range section + 1 {
		if separateSection(each) {
			continue
		}
		if p.jumped || b.builtAt[each] == 0 {
			p.sectionBlocks(gtx, e, each)
		}
	}
	// Once requested, Skills' asynchronous result can add blocks even while
	// only its loading head was in view. Refresh the count before anchoring,
	// without starting a Skills request on unrelated pages.
	if b.builtAt[sectionSkills] != 0 {
		sets, _ := p.skills.list.Sets(e)
		b.extra[sectionSkills] = len(sets.Sets)
	}
	if p.jumped {
		p.scroll.ScrollTo(b.first(section) + p.jumpBlock)
	} else {
		position.First = b.first(section) + min(block, b.extra[section])
	}
	b.moved = false
}

// the list takes the pane out to its edge so the wheel works beside the column and the thumb stands at the edge
func (p *Page) layoutSections(gtx layout.Context, e *env.Env, inset, width int) layout.Dimensions {
	th := e.Theme
	p.blocks.frame++
	p.blocks.drawn = 0
	// the wheel moves the list over the blocks it knew, before any count moves
	if separateSection(p.current) {
		return p.layoutSubpage(gtx, e, inset, width)
	}
	p.scroll.UpdateInput(gtx)
	anchorSection, anchorBlock := p.blocks.locate(max(p.scroll.Position.First, 0))
	anchorOffset := p.scroll.Position.Offset
	if p.jumped {
		anchorSection, anchorBlock, anchorOffset = p.current, p.jumpBlock, 0
	}
	p.prepare(gtx, e)
	total := p.blocks.total()
	// the colophon closes the sheet as the row after the last section
	dimensions := p.scroll.Layout(gtx, th, total+1, func(gtx layout.Context, index int) layout.Dimensions {
		full := gtx.Constraints.Max.X
		gtx.Constraints.Min.X, gtx.Constraints.Max.X = min(gtx.Constraints.Min.X, width), width
		defer op.Offset(image.Pt(inset, 0)).Push(gtx.Ops).Pop()
		if index >= total {
			foot := p.sheet.Colophon(gtx, th, p.page())
			return layout.Dimensions{Size: image.Pt(full, foot.Size.Y)}
		}
		section, block := p.blocks.locate(index)
		if separateSection(section) {
			return layout.Dimensions{}
		}
		blocks := p.sectionBlocks(gtx, e, section)
		if block >= len(blocks) {
			// a section came out a different length than counted, the next frame has it right
			return layout.Dimensions{}
		}
		var top, bottom unit.Dp
		if block == 0 {
			top = th.Sp.S5
		}
		if block == len(blocks)-1 {
			bottom = th.Sp.S1
			if section == sectionCount-1 {
				// room under the last section so it can still reach the top
				bottom = th.Sp.S6
			}
		}
		order := p.blocks.drawn
		p.blocks.drawn++
		widget := blocks[block]
		if order < revealed {
			// only the blocks Settings opens on rise, one after the other; a later scroll moves nothing
			widget = func(gtx layout.Context) layout.Dimensions { return p.reveal.Group(gtx, th, order, blocks[block]) }
		}
		// a section longer than the viewport keeps its true height, so its later controls remain scrollable
		child := gtx
		child.Constraints.Max.Y = 1 << 20
		dimensions := layout.Inset{Top: top, Bottom: bottom}.Layout(child, widget)
		return layout.Dimensions{Size: image.Pt(full, dimensions.Size.Y)}
	})
	if p.blocks.moved {
		p.scroll.AnchorTo(p.blocks.first(anchorSection)+min(anchorBlock, p.blocks.extra[anchorSection]), -anchorOffset)
		p.blocks.moved = false
		gtx.Execute(op.InvalidateCmd{})
	} else if p.jumped {
		p.jumped = false
		gtx.Execute(op.InvalidateCmd{})
	}
	return dimensions
}

// Oni's children and optional parts are destinations, not more of the
// top-level scrolling document. Their controls never spill into another
// page or load while somebody is reading unrelated Settings.
func separateSection(section int) bool {
	switch section {
	case sectionHands, sectionOniVoice, sectionOniScreen, sectionOniCommands, sectionOniWriting, sectionOniPrivacy, sectionParts:
		return true
	}
	return false
}

func (p *Page) layoutSubpage(gtx layout.Context, e *env.Env, inset, width int) layout.Dimensions {
	th := e.Theme
	p.scroll.UpdateInput(gtx)
	blocks := p.sectionBlocks(gtx, e, p.current)
	if p.jumped {
		p.scroll.ScrollTo(min(p.jumpBlock, len(blocks)-1))
	}
	dimensions := p.scroll.Layout(gtx, th, len(blocks)+1, func(gtx layout.Context, index int) layout.Dimensions {
		full := gtx.Constraints.Max.X
		gtx.Constraints.Min.X, gtx.Constraints.Max.X = min(gtx.Constraints.Min.X, width), width
		defer op.Offset(image.Pt(inset, 0)).Push(gtx.Ops).Pop()
		if index == len(blocks) {
			foot := p.sheet.Colophon(gtx, th, p.page())
			return layout.Dimensions{Size: image.Pt(full, foot.Size.Y)}
		}
		child := gtx
		child.Constraints.Max.Y = 1 << 20
		draw := blocks[index]
		if p.blocks.drawn < revealed {
			order := p.blocks.drawn
			draw = func(gtx layout.Context) layout.Dimensions { return p.reveal.Group(gtx, th, order, blocks[index]) }
		}
		p.blocks.drawn++
		dims := layout.Inset{Top: th.Sp.S2, Bottom: th.Sp.S1}.Layout(child, draw)
		return layout.Dimensions{Size: image.Pt(full, dims.Size.Y)}
	})
	p.blocks.moved = false
	if p.jumped {
		p.jumped = false
		gtx.Execute(op.InvalidateCmd{})
	}
	return dimensions
}

func documentSection(section int) int {
	for section < sectionCount-1 && separateSection(section) {
		section++
	}
	if separateSection(section) {
		return sectionKeys
	}
	return section
}
