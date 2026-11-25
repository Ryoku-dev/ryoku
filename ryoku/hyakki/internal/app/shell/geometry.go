package shell

import "ryoku-rashin/internal/app/look"

// docks only while the content keeps its minimum width, otherwise it's raised as a sheet
func panelDockEligible(widthDP float32, class look.Class, folded bool) bool {
	var sidebar float32
	switch class {
	case look.Expanded:
		sidebar = float32(look.SidebarExpanded)
		if folded {
			sidebar = float32(look.SidebarRail)
		}
	case look.Medium:
		sidebar = float32(look.SidebarRail)
	default:
		return false
	}
	return widthDP-sidebar-float32(look.Panel)-2*float32(look.Margin)-2*float32(look.Gap) >= float32(look.ContentMinWithPanel)
}
