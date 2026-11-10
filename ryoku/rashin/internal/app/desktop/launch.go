package desktop

import (
	"context"
	"image"
	"os/exec"
)

// what to start and how its window should arrive
type LaunchRequest struct {
	Name string
	Args []string
	// started instead when Name is not installed
	Fallback []string
	// the size its window takes in front of this app when this app floats, zero leaves the window where the desktop puts it
	Front image.Point
	// called when this app floats and the window is left where the desktop puts it, which may be out of sight
	Hidden func()
}

// returns when the program ends, so the caller reaps it
func Launch(ctx context.Context, request LaunchRequest) error {
	name, args := request.Name, request.Args
	if _, err := exec.LookPath(name); err != nil && len(request.Fallback) > 0 {
		name, args = request.Fallback[0], request.Fallback[1:]
	}
	placed := request.Front != (image.Point{})
	var seen *windows
	if placed || request.Hidden != nil {
		seen = seenWindows(ctx)
	}
	command := exec.Command(name, args...)
	if err := command.Start(); err != nil {
		return err
	}
	switch {
	case seen == nil:
	case placed:
		// not tied to the program: a launcher exits long before the window it asked for shows up
		go seen.front(ctx, request.Front)
	default:
		request.Hidden()
	}
	return command.Wait()
}
