package settings

import (
	"image"

	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/unit"

	"ryoku-rashin/internal/app/env"
	"ryoku-rashin/internal/app/look"
)

// a long section stands in the list as its blocks, laying out all of Teams for the screenful shown is what made scrolling lag
type blockList struct {
	frame   int
	builtAt [sectionCount]int
	built   [sectionCount][]layout.Widget
	// counted when last built, so an index keeps meaning the same block until then
	extra [sectionCount]int
	moved bool
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
	switch section {
	case sectionTeams:
		blocks = p.teamsBlocks(gtx, e, blocks)
	case sectionMemory:
		blocks = p.memoryBlocks(gtx, e, blocks)
	case sectionSkills:
		blocks = p.skillsBlocks(gtx, e, blocks)
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

func (p *Page) layoutWhole(gtx layout.Context, e *env.Env, section int) layout.Dimensions {
	switch section {
	case sectionRashin:
		return p.layoutRashin(gtx, e)
	case sectionProviders:
		return p.layoutProviders(gtx, e)
	case sectionAPI:
		return p.layoutAPI(gtx, e)
	case sectionAsk:
		return p.layoutAsk(gtx, e)
	case sectionAgents:
		return p.layoutAgents(gtx, e)
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
	for each := range section + 1 {
		if b.builtAt[each] == 0 {
			p.sectionBlocks(gtx, e, each)
		}
	}
	sets, _ := p.skills.list.Sets(e)
	b.extra[sectionSkills] = len(sets.Sets)
	position.First = b.first(section) + min(block, b.extra[section])
	b.moved = false
}

// the list takes the pane out to its edge so the wheel works beside the column and the thumb stands at the edge
func (p *Page) layoutSections(gtx layout.Context, e *env.Env, inset, width int) layout.Dimensions {
	th := e.Theme
	p.blocks.frame++
	// the wheel moves the list over the blocks it knew, before any count moves
	p.scroll.UpdateInput(gtx)
	p.prepare(gtx, e)
	dimensions := p.scroll.Layout(gtx, th, p.blocks.total(), func(gtx layout.Context, index int) layout.Dimensions {
		section, block := p.blocks.locate(index)
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
		full := gtx.Constraints.Max.X
		gtx.Constraints.Min.X, gtx.Constraints.Max.X = min(gtx.Constraints.Min.X, width), width
		defer op.Offset(image.Pt(inset, 0)).Push(gtx.Ops).Pop()
		dimensions := layout.Inset{Top: top, Bottom: bottom}.Layout(gtx, blocks[block])
		return layout.Dimensions{Size: image.Pt(full, dimensions.Size.Y)}
	})
	if p.blocks.moved {
		p.blocks.moved = false
		gtx.Execute(op.InvalidateCmd{})
	}
	return dimensions
}
