//go:build !linux

package palette

import (
	"context"
	"errors"
)

// Watch reports that native palette watching is unavailable on this platform.
func Watch(context.Context, func() Mode, func(Palette)) error {
	return errors.New("palette: live watching requires inotify")
}
