//go:build !linux

package palette

import (
	"context"
	"errors"
)

var errNoWatch = errors.New("palette: live watching requires inotify")

// Watch reports that native palette watching is unavailable on this platform.
func Watch(context.Context, func() Mode, func(Palette)) error {
	return errNoWatch
}

// WatchDesktop reports that native watching is unavailable on this platform.
func WatchDesktop(context.Context, func(Desktop), ...Option) error {
	return errNoWatch
}
