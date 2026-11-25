//go:build !linux

package surface

import "errors"

var errNoWayland = errors.New("oni-surface needs Wayland on Linux")

type noDisplay struct{ backend }

func (noDisplay) required() error { return errNoWayland }
func (noDisplay) close()          {}

func connectWayland(sink) (noDisplay, error) { return noDisplay{}, errNoWayland }

// ListGlobals is unavailable off Linux.
func ListGlobals() (Globals, error) { return Globals{}, errNoWayland }
