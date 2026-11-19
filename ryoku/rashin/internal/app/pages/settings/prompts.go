package settings

import (
	"strings"

	"gioui.org/layout"

	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/kit"
	"ryoku-i18n"

	"ryoku-rashin/internal/app/env"
	"ryoku-rashin/internal/app/look"
	"ryoku-rashin/internal/protocol"
)

type promptActions struct {
	open, copy, remove kit.Plaque
}

type promptsBlock struct {
	items                     []protocol.Prompt
	rows                      map[string]*promptActions
	loaded, loading, editing  bool
	readOnly                  bool
	id, kind, problem         string
	name, text                kit.TextField
	kindSelect                look.Select
	add, refresh, save, close kit.Plaque
	builtins                  foldRow
	builtinsOpen              bool
}

func (p *Page) promptsBlocks(gtx layout.Context, e *env.Env, blocks []layout.Widget) []layout.Widget {
	if !e.Feature("prompts") {
		return blocks
	}
	b, th := &p.prompts, e.Theme
	blocks = spacedBlocks(th, blocks, func(gtx layout.Context) layout.Dimensions {
		if !b.loaded && !b.loading {
			b.load(e)
		}
		b.add.Kind, b.add.Small, b.add.Label, b.add.Icon = kit.PlaqueTonal, true, i18n.T("Add prompt"), look.Icon.Add
		b.refresh.Kind, b.refresh.Small, b.refresh.Label, b.refresh.Busy = kit.PlaqueQuiet, true, i18n.T("Refresh prompts"), b.loading
		if b.add.Clicked(gtx) && !b.save.Busy {
			b.edit(protocol.Prompt{Kind: "prompt"})
		}
		if b.refresh.Clicked(gtx) {
			b.load(e)
		}
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			layout.Rigid(look.SectionHead(th, look.Head{Title: i18n.T("Prompt library")})),
			layout.Rigid(layout.Spacer{Height: th.Sp.S1}.Layout),
			layout.Rigid(look.Paragraph(th, th.T.Detail, th.R.Fg2, i18n.T("Type / in Ask to use a prompt, template or snippet. Built-ins stay unchanged, but you can copy them."))),
			layout.Rigid(layout.Spacer{Height: th.Sp.S2}.Layout),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return look.Flow{Gap: th.Sp.S2, LineGap: th.Sp.S2}.Layout(gtx, 2, func(gtx layout.Context, index int) layout.Dimensions {
					if index == 0 {
						return b.add.Layout(gtx, th.Theme)
					}
					return b.refresh.Layout(gtx, th.Theme)
				})
			}),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				if b.problem == "" {
					return layout.Dimensions{}
				}
				return look.Paragraph(th, th.T.Detail, th.R.Fg2, b.problem)(gtx)
			}),
		)
	})
	if b.editing {
		blocks = spacedBlocks(th, blocks, func(gtx layout.Context) layout.Dimensions { return b.editor(gtx, e) })
	}
	builtins := 0
	for _, item := range b.items {
		if item.BuiltIn {
			builtins++
			continue
		}
		blocks = append(blocks, func(gtx layout.Context) layout.Dimensions {
			return layout.Inset{Top: th.Sp.S3}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				return b.row(gtx, e, item)
			})
		})
	}
	if builtins > 0 {
		blocks = append(blocks, func(gtx layout.Context) layout.Dimensions {
			if b.builtins.toggled(gtx) {
				b.builtinsOpen = !b.builtinsOpen
			}
			return layout.Inset{Top: th.Sp.S3, Bottom: th.Sp.S3}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				return foldSlab(gtx, th, 1, func(gtx layout.Context, _ int) layout.Dimensions {
					return b.builtins.layout(gtx, th, b.builtinsOpen, foldHead{
						Title:  i18n.Tf("Built-in prompts (%d)", builtins),
						Detail: i18n.T("Templates you can view and copy"),
					}, func(gtx layout.Context) layout.Dimensions {
						rows := make([]layout.FlexChild, 0, builtins)
						for _, item := range b.items {
							if item.BuiltIn {
								rows = append(rows, layout.Rigid(func(gtx layout.Context) layout.Dimensions { return b.row(gtx, e, item) }))
							}
						}
						return layout.Flex{Axis: layout.Vertical}.Layout(gtx, rows...)
					})
				})
			})
		})
	}
	return blocks
}

func (b *promptsBlock) load(e *env.Env) {
	if b.loading || !e.Feature("prompts") {
		return
	}
	b.loading = true
	result := new(protocol.PromptsListResult)
	e.Call(protocol.MPromptsList, protocol.PromptsListParams{}, result, func(err error) {
		b.loading, b.loaded = false, true
		if err != nil {
			b.problem = err.Error()
			return
		}
		b.items, b.problem = result.Prompts, ""
		if b.rows == nil {
			b.rows = make(map[string]*promptActions)
		}
	})
}

func (b *promptsBlock) edit(item protocol.Prompt) {
	b.editing, b.readOnly = true, item.BuiltIn
	b.id, b.kind, b.problem = item.ID, item.Kind, ""
	b.name.SetText(item.Name)
	b.text.SetText(item.Text)
}

func (b *promptsBlock) row(gtx layout.Context, e *env.Env, item protocol.Prompt) layout.Dimensions {
	th := e.Theme
	if b.rows == nil {
		b.rows = make(map[string]*promptActions)
	}
	row := b.rows[item.ID]
	if row == nil {
		row = new(promptActions)
		b.rows[item.ID] = row
	}
	row.open.Kind, row.open.Small, row.open.Label = kit.PlaqueTonal, true, i18n.T("Edit")
	if item.BuiltIn {
		row.open.Label = i18n.T("View")
	}
	row.copy.Kind, row.copy.Small, row.copy.Label, row.copy.Icon = kit.PlaqueTonal, true, i18n.T("Copy"), look.Icon.Copy
	row.remove.Kind, row.remove.Small, row.remove.Label = kit.PlaqueQuiet, true, i18n.T("Delete")
	if row.open.Clicked(gtx) && !b.save.Busy {
		b.edit(item)
	}
	if row.copy.Clicked(gtx) && !b.save.Busy {
		copied := item
		copied.ID, copied.BuiltIn = "", false
		copied.Name = i18n.Tf("%s copy", item.Name)
		b.edit(copied)
	}
	if !item.BuiltIn && row.remove.Clicked(gtx) && !b.save.Busy {
		row.remove.Busy = true
		e.Call(protocol.MPromptsDelete, protocol.PromptDeleteParams{ID: item.ID}, new(protocol.Empty), func(err error) {
			row.remove.Busy = false
			if err != nil {
				b.problem = err.Error()
				return
			}
			if b.id == item.ID {
				b.editing = false
			}
			delete(b.rows, item.ID)
			b.load(e)
		})
	}
	controls := func(gtx layout.Context) layout.Dimensions {
		if b.save.Busy {
			gtx = gtx.Disabled()
		}
		count := 3
		if item.BuiltIn {
			count = 2
		}
		return look.Flow{Gap: 8, LineGap: 8}.Layout(gtx, count, func(gtx layout.Context, index int) layout.Dimensions {
			switch index {
			case 0:
				return row.open.Layout(gtx, th.Theme)
			case 1:
				return row.copy.Layout(gtx, th.Theme)
			default:
				return row.remove.Layout(gtx, th.Theme)
			}
		})
	}
	if item.BuiltIn {
		kind := i18n.T("Prompt")
		switch item.Kind {
		case "template":
			kind = i18n.T("Template")
		case "snippet":
			kind = i18n.T("Snippet")
		}
		return layout.Inset{Top: 4, Bottom: 4}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
				layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
					return kit.Label{Style: th.T.Detail, Color: th.R.Fg, MaxLines: 1}.Layout(gtx, th.Theme, item.Name+" · "+kind)
				}),
				layout.Rigid(layout.Spacer{Width: th.Sp.S3}.Layout),
				layout.Rigid(controls),
			)
		})
	}
	line, _, _ := strings.Cut(item.Text, "\n")
	return look.SettingsSlab(gtx, th, []look.Setting{{Title: item.Name, Detail: item.Kind + " · " + line, Control: controls}})
}

func (b *promptsBlock) editor(gtx layout.Context, e *env.Env) layout.Dimensions {
	th := e.Theme
	b.name.Label, b.name.Editor.SingleLine, b.name.Editor.MaxLen = i18n.T("Prompt name"), true, 100
	b.text.Label = i18n.T("Prompt text")
	b.text.Help = i18n.T("Use {{content}} or {{selection}} for your draft or clipboard, {{lang}} for your language, {{date}} for today, and {{name}} for a field you fill in.")
	b.kindSelect.Label = i18n.T("Prompt kind")
	kinds := []string{"prompt", "template", "snippet"}
	selected := 0
	for index, kind := range kinds {
		if kind == b.kind {
			selected = index
		}
	}
	b.save.Kind, b.save.Small, b.save.Label = kit.PlaqueTonal, true, i18n.T("Save prompt")
	b.close.Kind, b.close.Small, b.close.Label = kit.PlaqueQuiet, true, i18n.T("Close prompt editor")
	if !b.readOnly && b.save.Clicked(gtx) {
		b.save.Busy = true
		result := new(protocol.Prompt)
		e.Call(protocol.MPromptsSave, protocol.PromptSaveParams{ID: b.id, Name: b.name.Text(), Kind: b.kind, Text: b.text.Text()}, result, func(err error) {
			b.save.Busy = false
			if err != nil {
				b.problem = err.Error()
				return
			}
			b.id, b.problem = result.ID, ""
			b.load(e)
			e.Notify(i18n.T("Prompt saved"))
		})
	}
	if b.close.Clicked(gtx) {
		b.editing = false
	}
	field := func(control *kit.TextField) layout.Widget {
		return func(gtx layout.Context) layout.Dimensions {
			if b.readOnly || b.save.Busy {
				gtx = gtx.Disabled()
			}
			gtx.Constraints.Max.Y = min(gtx.Constraints.Max.Y, gtx.Dp(240))
			return control.Layout(gtx, th.Theme)
		}
	}
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(field(&b.name)),
		layout.Rigid(layout.Spacer{Height: th.Sp.S2}.Layout),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			if b.readOnly || b.save.Busy {
				gtx = gtx.Disabled()
			}
			dims := b.kindSelect.Layout(gtx, th, e.Overlays, []string{i18n.T("Prompt"), i18n.T("Template"), i18n.T("Snippet")}, selected)
			if index, changed := b.kindSelect.Changed(); changed && index >= 0 && index < len(kinds) {
				b.kind = kinds[index]
			}
			return dims
		}),
		layout.Rigid(layout.Spacer{Height: th.Sp.S2}.Layout),
		layout.Rigid(field(&b.text)),
		layout.Rigid(layout.Spacer{Height: th.Sp.S2}.Layout),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					if b.readOnly {
						gtx = gtx.Disabled()
					}
					return b.save.Layout(gtx, th.Theme)
				}),
				layout.Rigid(layout.Spacer{Width: th.Sp.S2}.Layout),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions { return b.close.Layout(gtx, th.Theme) }),
			)
		}),
	)
}
