package shell

import (
	"fmt"
	"image"
	"image/png"
	"os"
	"sync/atomic"
	"time"

	"gioui.org/gpu/headless"
	"gioui.org/io/input"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/unit"

	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/anim"

	"ryoku-rashin/internal/app/state"
)

func (r *runtime) renderPNG(options Options) error {
	window, err := headless.NewWindow(options.Width, options.Height)
	if err != nil {
		return fmt.Errorf("create headless renderer: %w", err)
	}
	defer window.Release()
	anim.SetReduce(true)
	defer anim.SetReduce(false)

	shell := newShell(r, options)
	var dirty atomic.Bool
	dirty.Store(true)
	remove := r.hub.Add(func() { dirty.Store(true) })
	defer remove()
	metric := unit.Metric{PxPerDp: 1, PxPerSp: 1}
	var operations op.Ops
	var router input.Router
	deadline := time.Now().Add(3 * time.Second)
	settled := time.Time{}
	for time.Now().Before(deadline) {
		if !dirty.Swap(false) {
			if r.store.Snapshot().Connection == state.Ready {
				if settled.IsZero() {
					settled = time.Now()
				} else if time.Since(settled) >= 100*time.Millisecond {
					break
				}
			}
			time.Sleep(10 * time.Millisecond)
			continue
		}
		settled = time.Time{}
		operations.Reset()
		gtx := layout.Context{Ops: &operations, Now: time.Now(), Source: router.Source(), Metric: metric, Constraints: layout.Exact(image.Pt(options.Width, options.Height))}
		shell.posts.Drain()
		shell.Frame(gtx)
		router.Frame(&operations)
		if err := window.Frame(&operations); err != nil {
			return fmt.Errorf("render PNG frame: %w", err)
		}
	}
	frame := image.NewRGBA(image.Rect(0, 0, options.Width, options.Height))
	if err := window.Screenshot(frame); err != nil {
		return fmt.Errorf("capture PNG frame: %w", err)
	}
	file, err := os.Create(options.PNG)
	if err != nil {
		return fmt.Errorf("create PNG: %w", err)
	}
	if err := png.Encode(file, frame); err != nil {
		_ = file.Close()
		return fmt.Errorf("encode PNG: %w", err)
	}
	if err := file.Close(); err != nil {
		return fmt.Errorf("close PNG: %w", err)
	}
	return nil
}
