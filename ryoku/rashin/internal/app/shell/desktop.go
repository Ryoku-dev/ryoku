package shell

import (
	"context"
	"sync/atomic"

	"ryoku-rashin/internal/app/desktop"
	"ryoku-rashin/internal/app/env"
	"ryoku-rashin/internal/paths"
)

// the work runs off the frame goroutine and comes back through posts
func windowDesktop(ctx context.Context, posts *postQueue) env.Desktop {
	var choosing atomic.Bool
	return env.Desktop{
		Choose: func(request desktop.ChooseRequest, done func([]string, error)) {
			// one chooser at a time, a second press while it's up is ignored
			if !choosing.CompareAndSwap(false, true) {
				return
			}
			go func() {
				chosen, err := desktop.Choose(ctx, request)
				posts.Post(func() {
					choosing.Store(false)
					done(chosen, err)
				})
			}()
		},
		ClipboardImage: func(done func(string)) {
			go func() {
				if path, ok := desktop.ClipboardImage(ctx, paths.PastedDir()); ok {
					posts.Post(func() { done(path) })
				}
			}()
		},
		Launch: func(name string, args []string, done func(error)) {
			go func() {
				err := desktop.Launch(name, args)
				if done != nil {
					posts.Post(func() { done(err) })
				}
			}()
		},
	}
}
