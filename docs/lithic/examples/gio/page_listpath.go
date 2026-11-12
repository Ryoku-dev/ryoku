package main

import (
	"gioui.org/io/event"
	"gioui.org/layout"

	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/kit"
	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/theme"
	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/tokens"
)

// listEditorSpecimen is one ListEditor state with the host's items, editor
// fields and per-item toggles.
type listEditorSpecimen struct {
	label    string
	list     kit.ListEditor
	items    []kit.ListItem
	toggles  []kit.Toggle
	name     kit.TextField
	command  kit.TextField
	disabled bool
	ready    bool
}

var listPathSpecimens = newListPathGallery()

type listPathGallery struct {
	lists []*listEditorSpecimen
	paths [7]kit.PathField
}

func startupApps() []kit.ListItem {
	return []kit.ListItem{
		{Title: "Ryotunes", Detail: "ryotunes --background"},
		{Title: "Clipboard history", Detail: "ryoku clipboard watch"},
		{Title: "Nextcloud", Detail: "nextcloud --background"},
	}
}

func newListPathGallery() *listPathGallery {
	gallery := &listPathGallery{}
	add := func(label string, setup func(s *listEditorSpecimen)) {
		specimen := &listEditorSpecimen{label: label, items: startupApps()}
		specimen.list.AddText = "Add a startup app"
		specimen.list.Label = "Startup apps"
		specimen.list.EmptyText = "Nothing starts with the desktop yet."
		setup(specimen)
		specimen.toggles = make([]kit.Toggle, len(specimen.items))
		for index := range specimen.toggles {
			specimen.toggles[index].Value = index != 1
			specimen.toggles[index].Label = ""
		}
		gallery.lists = append(gallery.lists, specimen)
	}
	add("Rest, a toggle per item", func(s *listEditorSpecimen) {})
	add("Edit plaque under the pointer", func(s *listEditorSpecimen) { s.list.Preview = kit.StateHover })
	add("Reorderable, keyboard focus on a handle", func(s *listEditorSpecimen) {
		s.list.Reorderable = true
		s.list.Preview = kit.StateFocus
	})
	add("Reorderable, dragging the first item", func(s *listEditorSpecimen) {
		s.list.Reorderable = true
		s.list.PreviewDrag = 34
	})
	add("Editing an item", func(s *listEditorSpecimen) { s.list.Edit(1) })
	add("Adding, not yet valid", func(s *listEditorSpecimen) { s.list.Add() })
	add("Empty", func(s *listEditorSpecimen) { s.items = nil })
	add("Disabled", func(s *listEditorSpecimen) { s.disabled = true })
	return gallery
}

func (s *listEditorSpecimen) applyEvents() {
	if index, ok := s.list.Opened(); ok {
		s.name.SetText("")
		s.command.SetText("")
		if index >= 0 && index < len(s.items) {
			s.name.SetText(s.items[index].Title)
			s.command.SetText(s.items[index].Detail)
		}
	}
	item := kit.ListItem{Title: s.name.Text(), Detail: s.command.Text()}
	if s.list.Added() {
		s.items = append(s.items, item)
		s.toggles = append(s.toggles, kit.Toggle{Value: true})
	}
	if index, ok := s.list.Edited(); ok && index < len(s.items) {
		s.items[index] = item
	}
	if index, ok := s.list.Removed(); ok && index < len(s.items) {
		s.items = append(s.items[:index], s.items[index+1:]...)
		s.toggles = append(s.toggles[:index], s.toggles[index+1:]...)
	}
	if from, to, ok := s.list.Moved(); ok && from < len(s.items) && to < len(s.items) {
		moved := s.items[from]
		s.items = append(s.items[:from], s.items[from+1:]...)
		s.items = append(s.items[:to], append([]kit.ListItem{moved}, s.items[to:]...)...)
	}
	s.list.Cancelled()
	s.list.EditorValid = s.name.Text() != "" && s.command.Text() != ""
	s.list.EditorFields = []event.Tag{&s.name.Editor, &s.command.Editor}
}

func (s *listEditorSpecimen) Layout(gtx layout.Context, th *theme.Theme) layout.Dimensions {
	if !s.ready {
		s.ready = true
		s.name.Label, s.command.Label, s.command.Mono = "Name", "Command", true
		s.name.Placeholder, s.command.Placeholder = "Ryotunes", "ryotunes --background"
	}
	s.applyEvents()
	if s.disabled {
		gtx = gtx.Disabled()
	}
	return kit.SlabContent(gtx, th, func(gtx layout.Context) layout.Dimensions {
		gtx.Constraints.Min.X = gtx.Constraints.Max.X
		return s.list.Layout(gtx, th, s.items,
			func(gtx layout.Context, index int) layout.Dimensions {
				return vstack(gtx, th.Sp.S3,
					func(gtx layout.Context) layout.Dimensions { return s.name.Layout(gtx, th) },
					func(gtx layout.Context) layout.Dimensions { return s.command.Layout(gtx, th) },
				)
			},
			func(gtx layout.Context, index int) layout.Dimensions {
				if index >= len(s.toggles) {
					return layout.Dimensions{}
				}
				s.toggles[index].Label = ""
				return s.toggles[index].Layout(gtx, th)
			})
	})
}

func (g *listPathGallery) listEditorSection(th *theme.Theme) layout.Widget {
	widgets := make([]layout.Widget, len(g.lists))
	for index, list := range g.lists {
		list := list
		widgets[index] = specimen(th, list.label, func(gtx layout.Context) layout.Dimensions { return list.Layout(gtx, th) })
	}
	return section(th, "ListEditor", func(gtx layout.Context) layout.Dimensions { return grid(gtx, 2, th.Sp.S3, widgets) })
}

func (g *listPathGallery) pathFieldSection(th *theme.Theme) layout.Widget {
	states := [...]struct {
		label, path, error string
		kind               kit.PathKind
		preview            kit.State
		disabled           bool
	}{
		{"Rest", "/home/ops/Pictures/Wallpapers", "", kit.PathFolder, kit.StateRest, false},
		{"Empty folder field", "", "", kit.PathFolder, kit.StateRest, false},
		{"Empty file field", "", "", kit.PathFile, kit.StateRest, false},
		{"Keyboard focus", "/home/ops/Pictures/Wallpapers", "", kit.PathFolder, kit.StateFocus, false},
		{"Choose under the pointer", "/home/ops/Pictures/Wallpapers", "", kit.PathFolder, kit.StateHover, false},
		{"Error", "/home/ops/Pictures/Walpapers", "This folder does not exist", kit.PathFolder, kit.StateRest, false},
		{"Disabled", "/home/ops/Pictures/Wallpapers", "", kit.PathFolder, kit.StateRest, true},
	}
	widgets := make([]layout.Widget, len(states))
	for index := range states {
		index := index
		state := states[index]
		widgets[index] = specimen(th, state.label, func(gtx layout.Context) layout.Dimensions {
			field := &g.paths[index]
			if path, ok := field.Edited(); ok {
				field.Path = path
			}
			for field.Choose.Clicked(gtx) {
			}
			if field.Path == "" && field.Label == "" {
				field.Path = state.path
			}
			field.Label = "Wallpaper folder"
			if state.kind == kit.PathFile {
				field.Label = "Lock screen picture"
			}
			field.Kind, field.Error, field.Preview = state.kind, state.error, state.preview
			gtx.Constraints.Max.X = min(gtx.Constraints.Max.X, gtx.Dp(tokens.SizeSettingsControlMax))
			if state.disabled {
				gtx = gtx.Disabled()
			}
			return field.Layout(gtx, th)
		})
	}
	return section(th, "PathField", func(gtx layout.Context) layout.Dimensions { return grid(gtx, 4, th.Sp.S3, widgets) })
}
