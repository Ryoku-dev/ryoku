package settings

import (
	"image"

	"gioui.org/layout"

	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/kit"
	"ryoku-i18n"

	"ryoku-rashin/internal/app/desktop"
	"ryoku-rashin/internal/app/env"
	"ryoku-rashin/internal/app/look"
	"ryoku-rashin/internal/protocol"
)

type featuresPanel struct {
	toggles [20]kit.Toggle
	links   [20]kit.Plaque
	on, off kit.Plaque
	open    kit.Plaque
	theia   kit.Plaque
}

type appFeature struct {
	id, name, detail string
	section          int
	settings         string
}

func appFeatures() [20]appFeature {
	return [20]appFeature{
		{"prompts", i18n.T("Prompt library"), i18n.T("Keep prompts, templates and snippets a slash away in Ask."), sectionAsk, i18n.T("Prompt library settings")},
		{"enhance", i18n.T("Improve my prompt"), i18n.T("Rewrite a draft question so it gets a better answer."), 0, ""},
		{"askwriting", i18n.T("Writing tools in Ask"), i18n.T("Fix, rewrite or change the tone of what you are about to send."), 0, ""},
		{"analyze", i18n.T("Analyze a file"), i18n.T("Add a file to ask about its contents, key numbers and anything odd."), 0, ""},
		{"audio", i18n.T("Audio files"), i18n.T("Attach a recording to get a transcript and summary."), 0, ""},
		{"video", i18n.T("Video links"), i18n.T("Paste a video link to get a summary with key moments."), 0, ""},
		{"history", i18n.T("Star and archive"), i18n.T("Keep favourite conversations on top and tuck old ones away."), sectionAsk, i18n.T("Conversation history settings")},
		{"searchsteps", i18n.T("Search steps"), i18n.T("See what Rashin searched and read while it answers."), 0, ""},
		{"compare", i18n.T("Compare models"), i18n.T("Ask two or three models the same question and read their answers side by side."), 0, ""},
		{"share", i18n.T("Share as a picture"), i18n.T("Turn an answer into a clean picture to send anywhere."), 0, ""},
		{"quote", i18n.T("Ask again about a picture"), i18n.T("Put a picture from a conversation back into your question."), 0, ""},
		{"mindmap", i18n.T("Mind maps"), i18n.T("Explore and save a mind map drawn in an answer."), 0, ""},
		{"artifacts", i18n.T("Artifacts"), i18n.T("Preview pages agents make and let them fix what breaks."), 0, ""},
		{"research", i18n.T("Deep research"), i18n.T("Start a research dream that gathers sources and presents a cited report."), 0, ""},
		{"schedules", i18n.T("Scheduled runs"), i18n.T("Run questions, team tasks or research dreams on a schedule."), 0, ""},
		{"setupcards", i18n.T("Setup cards"), i18n.T("Let agents pause with clear steps when they need you to set something up."), 0, ""},
		{"mcp", i18n.T("MCP servers"), i18n.T("Connect your own tool servers with sign-in and a health check."), sectionAgents, i18n.T("MCP server settings")},
		{"diskcare", i18n.T("Disk care"), i18n.T("See what agents keep on disk and clear it safely."), sectionTeams, i18n.T("Disk care settings")},
		{"profile", i18n.T("About me"), i18n.T("Tell Rashin your name, work and how you like answers."), sectionMemory, i18n.T("About me settings")},
		{"collections", i18n.T("Collections"), i18n.T("Save answers, excerpts and files to chat with a whole collection."), 0, ""},
	}
}

func featuresOn(features map[string]bool) int {
	on := 0
	for _, id := range protocol.FeatureIDs {
		if features[id] {
			on++
		}
	}
	return on
}

func (p *Page) featuresBlocks(gtx layout.Context, e *env.Env, blocks []layout.Widget) []layout.Widget {
	th := e.Theme
	b := &p.features
	features := e.Snapshot().Settings.Features
	on := featuresOn(features)
	blocks = append(blocks, func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return look.Monument(gtx, th, i18n.T("Features"), i18n.T("Choose what belongs in your Rashin. Features that are off stay out of the way."))
			}),
			layout.Rigid(layout.Spacer{Height: th.Sp.S3}.Layout),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return look.Paragraph(th, th.T.Row, th.R.Fg2, i18n.Tf("%d of %d on", on, len(protocol.FeatureIDs)))(gtx)
			}),
			layout.Rigid(layout.Spacer{Height: th.Sp.S2}.Layout),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return look.Flow{Gap: 8, LineGap: 8}.Layout(gtx, 2, func(gtx layout.Context, index int) layout.Dimensions {
					button, label, value := &b.on, i18n.T("Turn all on"), true
					if index == 1 {
						button, label, value = &b.off, i18n.T("Turn all off"), false
					}
					button.Kind, button.Small, button.Label = kit.PlaqueTonal, true, label
					if button.Clicked(gtx) {
						patch := make(map[string]any, len(b.toggles))
						for _, id := range protocol.FeatureIDs {
							patch[id] = value
						}
						set(e, "features", patch)
					}
					return button.Layout(gtx, th.Theme)
				})
			}),
		)
	})
	all := appFeatures()
	groups := [...]struct {
		title      string
		start, end int
	}{
		{i18n.T("Ask"), 0, 8},
		{i18n.T("Answers"), 8, 13},
		{i18n.T("Agents and Teams"), 13, 18},
		{i18n.T("Memory and Index"), 18, 20},
	}
	for _, group := range groups {
		rows := make([]look.Setting, 0, group.end-group.start)
		for index := group.start; index < group.end; index++ {
			feature := all[index]
			rows = append(rows, look.Setting{
				Title: feature.name, Detail: feature.detail,
				Control: func(gtx layout.Context) layout.Dimensions {
					return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							if !features[feature.id] || feature.settings == "" {
								return layout.Dimensions{}
							}
							button := &b.links[index]
							button.Kind, button.Small, button.Label = kit.PlaqueQuiet, true, feature.settings
							if button.Clicked(gtx) {
								p.sectionBlocks(gtx, e, feature.section)
								p.jump(feature.section)
								p.jumpBlock = p.blocks.featureAt[feature.id]
							}
							return layout.Inset{Right: th.Sp.S2}.Layout(gtx, func(gtx layout.Context) layout.Dimensions { return button.Layout(gtx, th.Theme) })
						}),
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							toggle := &b.toggles[index]
							toggle.Value = features[feature.id]
							return namedSwitch(gtx, th, feature.name, toggle, func(on bool) { set(e, "features", map[string]any{feature.id: on}) })
						}),
					)
				},
			})
		}
		blocks = spacedBlocks(th, blocks, func(gtx layout.Context) layout.Dimensions {
			return look.SettingsSection(gtx, th, group.title, rows)
		})
	}
	return spacedBlocks(th, blocks, func(gtx layout.Context) layout.Dimensions {
		b.theia.Kind, b.theia.Small, b.theia.Label = kit.PlaqueTonal, true, i18n.T("Open Theia settings")
		if b.theia.Clicked(gtx) && e.Desktop.Launch != nil {
			e.Desktop.Launch(desktop.LaunchRequest{Name: "ryoku-shell", Args: []string{"hub", "open", "rashin/theia-on"}, Front: image.Pt(1100, 820)}, func(err error) {
				if err != nil {
					e.Notify(err.Error())
				}
			})
		}
		return look.SettingsSlab(gtx, th, []look.Setting{{
			Title: i18n.T("Theia on your desktop"), Detail: i18n.T("Choose the tools that appear when you point at the screen or highlight text."),
			Control: func(gtx layout.Context) layout.Dimensions { return b.theia.Layout(gtx, th.Theme) },
		}})
	})
}

func (p *Page) featuresOff(gtx layout.Context, e *env.Env) layout.Dimensions {
	if featuresOn(e.Snapshot().Settings.Features) != 0 {
		return layout.Dimensions{}
	}
	th, button := e.Theme, &p.features.open
	button.Kind, button.Small, button.Label, button.Icon = kit.PlaqueQuiet, true, i18n.T("Open Features"), look.Icon.Forward
	if button.Clicked(gtx) {
		p.jump(sectionFeatures)
	}
	return layout.Inset{Top: th.Sp.S3}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
			layout.Flexed(1, look.Paragraph(th, th.T.Detail, th.R.Fg2, i18n.Tf("%d optional features are off", len(protocol.FeatureIDs)))),
			layout.Rigid(layout.Spacer{Width: th.Sp.S3}.Layout),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions { return button.Layout(gtx, th.Theme) }),
		)
	})
}
