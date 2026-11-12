package settings

import (
	"image"
	"io"
	"strings"

	"gioui.org/io/clipboard"
	"gioui.org/layout"
	"gioui.org/op"

	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/kit"
	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/palette"
	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/tokens"
	"ryoku-i18n"

	"ryoku-rashin/internal/app/chat"
	"ryoku-rashin/internal/app/env"
	"ryoku-rashin/internal/app/look"
	"ryoku-rashin/internal/app/pages/memoryui"
	"ryoku-rashin/internal/app/pages/skillsui"
	"ryoku-rashin/internal/paths"
	"ryoku-rashin/internal/protocol"
)

// each set saves as its toggle flips; a skill's SKILL.md is read in the side panel
type skillsPanel struct {
	list   skillsui.List
	folder kit.Plaque
	sets   map[string]*skillSetControls

	sheet memoryui.Sheet
	// the skill the sheet shows and what the daemon handed back for it
	open    protocol.Skill
	setName string
	content *protocol.SkillContent
	problem string
	scroll  look.Scroller
	prose   chat.Prose
	close   kit.Plaque
}

type skillSetControls struct {
	toggle kit.Toggle
	copies map[string]*kit.Plaque
	rows   map[string]*skillRow
}

type skillRow struct {
	click kit.Click
	hover look.Hover
}

func (k *skillsPanel) controls(set string) *skillSetControls {
	if k.sets == nil {
		k.sets = make(map[string]*skillSetControls)
	}
	controls := k.sets[set]
	if controls == nil {
		controls = &skillSetControls{copies: make(map[string]*kit.Plaque), rows: make(map[string]*skillRow)}
		k.sets[set] = controls
	}
	return controls
}

func (p *Page) skillsBlocks(gtx layout.Context, e *env.Env, blocks []layout.Widget) []layout.Widget {
	th := e.Theme
	k := &p.skills
	settings := e.Snapshot().Settings.Skills
	result, ready := k.list.Sets(e)
	if k.folder.Clicked(gtx) && result.Root != "" {
		e.OpenFolder(result.Root)
	}
	children := []layout.FlexChild{
		layout.Rigid(func(gtx layout.Context) layout.Dimensions { return k.layoutTitle(gtx, th, result.Root) }),
	}
	switch {
	case k.list.Problem() != "":
		children = append(children,
			layout.Rigid(layout.Spacer{Height: th.Sp.S4}.Layout),
			layout.Rigid(look.Paragraph(th, th.T.Detail, th.R.Jasper, i18n.Tf("Rashin could not list its skills: %s", k.list.Problem()))),
		)
	case !ready:
		children = append(children,
			layout.Rigid(layout.Spacer{Height: th.Sp.S4}.Layout),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions { return look.SkeletonRows(gtx, th, 4, true) }),
		)
	}
	blocks = append(blocks, func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
	})
	for _, set := range result.Sets {
		blocks = spacedBlocks(th, blocks, func(gtx layout.Context) layout.Dimensions { return k.layoutSet(gtx, e, settings, set) })
	}
	return blocks
}

func (k *skillsPanel) layoutTitle(gtx layout.Context, th *look.Theme, root string) layout.Dimensions {
	proof := i18n.T("Every model Rashin runs gets the sets that are on: Ask, agent sessions and teams.")
	k.folder.Kind, k.folder.Small, k.folder.Label = kit.PlaqueTonal, true, i18n.T("Show in files")
	words := func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			layout.Rigid(kit.Text(th.Theme, th.T.Section, th.R.Fg, i18n.T("Skills"))),
			layout.Rigid(layout.Spacer{Height: th.Sp.S2}.Layout),
			layout.Rigid(look.Paragraph(th, th.T.Proof, th.R.Fg2, proof)),
		)
	}
	if root == "" {
		return words(gtx)
	}
	folder := func(gtx layout.Context) layout.Dimensions {
		return look.Named(gtx, i18n.T("Show the skills in files"), func(gtx layout.Context) layout.Dimensions { return k.folder.Layout(gtx, th.Theme) })
	}
	if gtx.Constraints.Max.X < gtx.Dp(560) {
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			layout.Rigid(words),
			layout.Rigid(layout.Spacer{Height: th.Sp.S3}.Layout),
			layout.Rigid(folder),
		)
	}
	return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Start}.Layout(gtx,
		layout.Flexed(1, words),
		layout.Rigid(layout.Spacer{Width: th.Sp.S4}.Layout),
		layout.Rigid(folder),
	)
}

// the set's name and toggle, who made it, what it is for, what it needs, then its skills
func (k *skillsPanel) layoutSet(gtx layout.Context, e *env.Env, settings protocol.SkillsSettings, group protocol.SkillSet) layout.Dimensions {
	th := e.Theme
	controls := k.controls(group.ID)
	controls.toggle.Value = skillsui.On(settings, group.ID)
	byline := i18n.Tf("By %s, %s licence, %d skills.", group.By, group.License, len(group.Skills))
	if len(group.Skills) == 1 {
		byline = i18n.Tf("By %s, %s licence, one skill.", group.By, group.License)
	}
	heading := func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
			layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
				return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
					layout.Rigid(kit.Text(th.Theme, th.T.Title, th.R.Fg, group.Name)),
					layout.Rigid(layout.Spacer{Height: th.Sp.S1}.Layout),
					layout.Rigid(look.Paragraph(th, th.T.Detail, th.R.Fg2, byline)),
				)
			}),
			layout.Rigid(layout.Spacer{Width: th.Sp.S4}.Layout),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return namedSwitch(gtx, th, i18n.Tf("%s for every model", group.Name), &controls.toggle, func(on bool) {
					set(e, "skills", map[string]any{"off": skillsui.Turn(settings.Off, group.ID, on)})
				})
			}),
		)
	}
	children := []layout.FlexChild{
		layout.Rigid(heading),
		layout.Rigid(layout.Spacer{Height: th.Sp.S3}.Layout),
		layout.Rigid(look.Paragraph(th, th.T.Body, th.R.Fg, group.Summary)),
	}
	if len(group.Needs) > 0 {
		children = append(children,
			layout.Rigid(layout.Spacer{Height: th.Sp.S3}.Layout),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions { return k.layoutNeeds(gtx, e, controls, group.Needs) }),
		)
	}
	children = append(children,
		layout.Rigid(layout.Spacer{Height: th.Sp.S3}.Layout),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return foldSlab(gtx, th, len(group.Skills), func(gtx layout.Context, index int) layout.Dimensions {
				skill := group.Skills[index]
				row := controls.rows[skill.ID]
				if row == nil {
					row = new(skillRow)
					controls.rows[skill.ID] = row
				}
				if row.click.Clicked(gtx) {
					k.openSkill(e, skill, group.Name)
				}
				return row.layout(gtx, th, skill, k.sheet.IsOpen() && k.open.ID == skill.ID)
			})
		}),
	)
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
}

// a command this computer has is one line; a missing one says so and offers its install command to copy
func (k *skillsPanel) layoutNeeds(gtx layout.Context, e *env.Env, controls *skillSetControls, needs []protocol.SkillNeed) layout.Dimensions {
	th := e.Theme
	var present []string
	var children []layout.FlexChild
	for _, need := range needs {
		if need.Present {
			present = append(present, need.Command)
			continue
		}
		if len(children) > 0 {
			children = append(children, layout.Rigid(layout.Spacer{Height: th.Sp.S3}.Layout))
		}
		children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions { return k.layoutMissing(gtx, e, controls, need) }))
	}
	if len(present) > 0 {
		if len(children) > 0 {
			children = append(children, layout.Rigid(layout.Spacer{Height: th.Sp.S3}.Layout))
		}
		children = append(children, layout.Rigid(look.Paragraph(th, th.T.Detail, th.R.Fg2, i18n.Tf("Uses %s on this computer.", andList(present)))))
	}
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
}

func (k *skillsPanel) layoutMissing(gtx layout.Context, e *env.Env, controls *skillSetControls, need protocol.SkillNeed) layout.Dimensions {
	th := e.Theme
	sentence := func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Baseline}.Layout(gtx,
			layout.Rigid(kit.Claim(th.Theme, palette.Attention, i18n.T("Missing"))),
			layout.Rigid(layout.Spacer{Width: th.Sp.S3}.Layout),
			layout.Flexed(1, look.Paragraph(th, th.T.Detail, th.R.Fg2, i18n.Tf("Its skills need the %s command, which this computer doesn't have. They say so when they run.", need.Command))),
		)
	}
	if need.Install == "" {
		return sentence(gtx)
	}
	plaque := controls.copies[need.Command]
	if plaque == nil {
		plaque = new(kit.Plaque)
		controls.copies[need.Command] = plaque
	}
	plaque.Kind, plaque.Small, plaque.Icon, plaque.Label = kit.PlaqueTonal, true, look.Icon.Copy, i18n.T("Copy")
	if plaque.Clicked(gtx) {
		gtx.Execute(clipboard.WriteCmd{Type: "text/plain", Data: io.NopCloser(strings.NewReader(need.Install))})
		e.Notify(i18n.T("Copied the install command"))
	}
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(sentence),
		layout.Rigid(layout.Spacer{Height: th.Sp.S2}.Layout),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
				layout.Rigid(look.TokenChip(th, need.Install)),
				layout.Rigid(layout.Spacer{Width: th.Sp.S2}.Layout),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return look.Named(gtx, i18n.Tf("Copy the %s install command", need.Command), func(gtx layout.Context) layout.Dimensions { return plaque.Layout(gtx, th.Theme) })
				}),
			)
		}),
	)
}

func andList(words []string) string {
	if len(words) < 2 {
		return strings.Join(words, "")
	}
	return i18n.Tf("%s and %s", strings.Join(words[:len(words)-1], ", "), words[len(words)-1])
}

// the whole row opens the skill; one its authors start only by name says so in words
func (row *skillRow) layout(gtx layout.Context, th *look.Theme, skill protocol.Skill, shown bool) layout.Dimensions {
	width := gtx.Constraints.Max.X
	inset, pad := gtx.Dp(13), gtx.Dp(10)
	record := op.Record(gtx.Ops)
	content := gtx
	content.Constraints = layout.Constraints{Min: image.Pt(width-2*inset, 0), Max: image.Pt(width-2*inset, gtx.Constraints.Max.Y)}
	offset := op.Offset(image.Pt(inset, pad)).Push(gtx.Ops)
	words := layout.Flex{Axis: layout.Horizontal, Alignment: layout.Start}.Layout(content,
		layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
				layout.Rigid(kit.Text(th.Theme, th.T.Row, th.R.Fg, skill.Name)),
				layout.Rigid(layout.Spacer{Height: th.Sp.S1}.Layout),
				layout.Rigid(look.Paragraph(th, th.T.Detail, th.R.Fg2, skill.Description)),
			)
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			if !skill.Manual {
				return layout.Dimensions{}
			}
			return layout.Inset{Left: th.Sp.S4}.Layout(gtx, kit.Claim(th.Theme, palette.Idle, i18n.T("When you ask")))
		}),
	)
	offset.Pop()
	call := record.Stop()
	size := image.Pt(width, words.Size.Y+2*pad)
	area := gtx
	area.Constraints = layout.Exact(size)
	_, hover := row.hover.Layout(area, th, func(gtx layout.Context) layout.Dimensions {
		return look.Named(gtx, skill.Name, func(gtx layout.Context) layout.Dimensions {
			return row.click.Layout(gtx, func(gtx layout.Context) layout.Dimensions { return layout.Dimensions{Size: size} })
		})
	})
	if shown {
		hover = 1
	}
	if hover > 0 {
		kit.FillCut(gtx.Ops, image.Rectangle{Max: size}, gtx.Dp(th.Cut.Small), palette.WithAlpha(palette.Mix(th.R.Slab, th.R.Fg, tokens.AlphaHoverPlane), hover))
	}
	call.Add(gtx.Ops)
	return layout.Dimensions{Size: size}
}

// a sheet the user shut by its scrim is still marked open, so it is closed first and opens the panel again
func (k *skillsPanel) openSkill(e *env.Env, skill protocol.Skill, setName string) {
	if k.sheet.IsOpen() && !e.PanelOpen {
		k.sheet.Close(e)
	}
	k.sheet.Open(e)
	if k.open.ID == skill.ID && (k.content != nil || k.problem != "") {
		return
	}
	k.open, k.setName, k.content, k.problem = skill, setName, nil, ""
	k.prose, k.scroll = chat.Prose{}, look.Scroller{}
	var content protocol.SkillContent
	e.Call(protocol.MSkillsGet, protocol.SkillParams{ID: skill.ID}, &content, func(err error) {
		// an answer for a skill no longer shown is dropped
		if k.open.ID != skill.ID {
			return
		}
		if err != nil {
			k.problem = err.Error()
			return
		}
		k.content = &content
	})
}

func (p *Page) HasPanel(*env.Env) bool { return p.skills.sheet.IsOpen() }

func (p *Page) Panel(gtx layout.Context, e *env.Env) layout.Dimensions {
	return p.skills.layoutSheet(gtx, e)
}

func (k *skillsPanel) layoutSheet(gtx layout.Context, e *env.Env) layout.Dimensions {
	th := e.Theme
	skill := k.open
	k.close.Kind, k.close.Small, k.close.Label = kit.PlaqueQuiet, true, i18n.T("Close")
	if k.close.Clicked(gtx) {
		k.sheet.Close(e)
	}
	proof := i18n.Tf("From %s. Models reach for it when the work fits.", k.setName)
	if skill.Manual {
		proof = i18n.Tf("From %s. It starts only when you ask for it by name.", k.setName)
	}
	heading := func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Start}.Layout(gtx,
			layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
				return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
					layout.Rigid(look.Paragraph(th, th.T.Title, th.R.Fg, skill.Name)),
					layout.Rigid(layout.Spacer{Height: th.Sp.S1}.Layout),
					layout.Rigid(look.Paragraph(th, th.T.Detail, th.R.Fg2, proof)),
				)
			}),
			layout.Rigid(layout.Spacer{Width: th.Sp.S3}.Layout),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return look.Named(gtx, i18n.Tf("Close %s", skill.Name), func(gtx layout.Context) layout.Dimensions { return k.close.Layout(gtx, th.Theme) })
			}),
		)
	}
	return k.scroll.Layout(gtx, th, 1, func(gtx layout.Context, _ int) layout.Dimensions {
		return layout.UniformInset(th.Sp.S5).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
				layout.Rigid(heading),
				layout.Rigid(layout.Spacer{Height: th.Sp.S4}.Layout),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions { return k.layoutText(gtx, e) }),
			)
		})
	})
}

// SKILL.md read as Markdown the way an answer is, its frontmatter left out as the heading already says it
func (k *skillsPanel) layoutText(gtx layout.Context, e *env.Env) layout.Dimensions {
	th := e.Theme
	switch {
	case k.problem != "":
		return look.Paragraph(th, th.T.Detail, th.R.Jasper, i18n.Tf("Rashin could not read this skill: %s", k.problem))(gtx)
	case k.content == nil:
		return look.SkeletonRows(gtx, th, 6, false)
	}
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return kit.Label{Style: th.T.MonoSm, Color: th.R.Fg3}.Layout(gtx, th.Theme, paths.Short(k.content.Path))
		}),
		layout.Rigid(layout.Spacer{Height: th.Sp.S3}.Layout),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			dimensions := k.prose.Layout(gtx, th, e.Overlays, skillBody(k.content.Text))
			env.HandleChatActions(gtx, e, "", k.prose.Actions())
			return dimensions
		}),
	)
}

func skillBody(text string) string {
	text = strings.ReplaceAll(text, "\r\n", "\n")
	if rest, ok := strings.CutPrefix(text, "---\n"); ok {
		if _, body, found := strings.Cut(rest, "\n---\n"); found {
			return strings.TrimLeft(body, "\n")
		}
	}
	return text
}
