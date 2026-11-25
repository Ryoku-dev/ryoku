package memoryui

import "ryoku-rashin/internal/app/env"

// Sheet is a read-only sheet in the side panel. Closing it puts the panel
// back as it stood before the sheet opened.
type Sheet struct {
	open, opened bool
}

// Open shows the sheet, opening the side panel when it was shut.
func (s *Sheet) Open(e *env.Env) {
	if s.open {
		return
	}
	s.open, s.opened = true, !e.PanelOpen
	if s.opened {
		e.TogglePanel()
	}
}

// Close hides the sheet, shutting the side panel only if the sheet opened it.
func (s *Sheet) Close(e *env.Env) {
	if !s.open {
		return
	}
	if s.opened && e.PanelOpen {
		e.TogglePanel()
	}
	s.open, s.opened = false, false
}

// IsOpen says whether the sheet is shown.
func (s *Sheet) IsOpen() bool { return s.open }
