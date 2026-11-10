package shell

import (
	"fmt"
	"image"
	"image/color"
	"image/draw"
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
	if options.Script == "" {
		anim.SetReduce(true)
		defer anim.SetReduce(false)
	}

	shell := newShell(r, options)
	var dirty atomic.Bool
	dirty.Store(true)
	remove := r.hub.Add(func() { dirty.Store(true) })
	defer remove()
	scale := float32(shell.preferences.ZoomPercent) / 100
	metric := unit.Metric{PxPerDp: scale, PxPerSp: scale}
	var operations op.Ops
	var router input.Router

	// again means the last frame asked for one more right away, like a list settling at its end
	again := false
	render := func(now time.Time) error {
		operations.Reset()
		gtx := layout.Context{
			Ops: &operations, Now: now, Source: router.Source(), Metric: metric,
			Constraints: layout.Exact(image.Pt(options.Width, options.Height)),
		}
		shell.posts.Drain()
		shell.Frame(gtx)
		router.Frame(&operations)
		at, wake := router.WakeupTime()
		again = wake && !at.After(now)
		if err := window.Frame(&operations); err != nil {
			return fmt.Errorf("render PNG frame: %w", err)
		}
		return nil
	}

	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if dirty.Swap(false) || r.store.Snapshot().Connection != state.Ready {
			if err := render(storyboardEpoch); err != nil {
				return err
			}
		}
		if r.store.Snapshot().Connection == state.Ready {
			break
		}
		time.Sleep(5 * time.Millisecond)
	}

	// pages load after their first draw so this redraws until the answers stop, with a few extra frames for anything that asks
	settle := func(now time.Time) error {
		granted := 0
		for quiet := 0; quiet < 8; {
			time.Sleep(5 * time.Millisecond)
			redraw := dirty.Swap(false)
			if again && granted < 6 {
				redraw, granted = true, granted+1
			}
			if !redraw {
				quiet++
				continue
			}
			quiet = 0
			if err := render(now); err != nil {
				return err
			}
		}
		return render(now)
	}
	if options.Script == "" {
		if err := settle(storyboardEpoch); err != nil {
			return err
		}
		frame, err := screenshot(window, options.Width, options.Height)
		if err != nil {
			return err
		}
		return writePNG(options.PNG, frame)
	}
	if err := settle(storyboardEpoch); err != nil {
		return err
	}

	events, err := readStoryboard(options.Script)
	if err != nil {
		return err
	}
	shots := append([]int(nil), options.Shots...)
	if len(shots) == 0 {
		at := 0
		if len(events) > 0 {
			at = events[len(events)-1].at
		}
		shots = []int{at}
	}
	// stories run in real time so what the demo plays by the clock lands in the frames it would on screen
	started := time.Now()
	frameAt := func(at int) error {
		elapsed := time.Duration(at) * time.Millisecond
		if rest := elapsed - time.Since(started); rest > 0 {
			time.Sleep(rest)
		}
		now := storyboardEpoch.Add(elapsed)
		if err := render(now); err != nil || !again {
			return err
		}
		return render(now)
	}
	frames := make([]*image.RGBA, 0, len(shots))
	eventIndex := 0
	for _, shot := range shots {
		for eventIndex < len(events) && events[eventIndex].at <= shot {
			event := events[eventIndex]
			if err := applyStoryboardEvent(event, &router, shell, scale); err != nil {
				return fmt.Errorf("storyboard at %d ms: %w", event.at, err)
			}
			if err := frameAt(event.at); err != nil {
				return err
			}
			eventIndex++
		}
		if err := frameAt(shot); err != nil {
			return err
		}
		frame, err := screenshot(window, options.Width, options.Height)
		if err != nil {
			return err
		}
		frames = append(frames, frame)
	}
	return writePNG(options.PNG, contactSheet(frames, shots))
}

func screenshot(window *headless.Window, width, height int) (*image.RGBA, error) {
	frame := image.NewRGBA(image.Rect(0, 0, width, height))
	if err := window.Screenshot(frame); err != nil {
		return nil, fmt.Errorf("capture PNG frame: %w", err)
	}
	return frame, nil
}

func writePNG(path string, frame image.Image) error {
	file, err := os.Create(path)
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

func contactSheet(frames []*image.RGBA, shots []int) *image.RGBA {
	if len(frames) == 0 {
		return image.NewRGBA(image.Rect(0, 0, 1, 1))
	}
	width, height := frames[0].Bounds().Dx(), frames[0].Bounds().Dy()
	canvas := image.NewRGBA(image.Rect(0, 0, width*len(frames), height+18))
	draw.Draw(canvas, canvas.Bounds(), image.NewUniform(color.NRGBA{A: 0xff}), image.Point{}, draw.Src)
	for index, frame := range frames {
		destination := image.Rect(index*width, 0, (index+1)*width, height)
		draw.Draw(canvas, destination, frame, frame.Bounds().Min, draw.Src)
		drawTinyText(canvas, index*width+8, height+5, fmt.Sprintf("%d ms", shots[index]), color.NRGBA{R: 0xe8, G: 0xe1, B: 0xd5, A: 0xff})
	}
	return canvas
}

var tinyGlyphs = map[rune][5]uint8{
	'0': {7, 5, 5, 5, 7}, '1': {2, 6, 2, 2, 7}, '2': {7, 1, 7, 4, 7},
	'3': {7, 1, 7, 1, 7}, '4': {5, 5, 7, 1, 1}, '5': {7, 4, 7, 1, 7},
	'6': {7, 4, 7, 5, 7}, '7': {7, 1, 1, 1, 1}, '8': {7, 5, 7, 5, 7},
	'9': {7, 5, 7, 1, 7}, 'm': {0, 7, 7, 5, 5}, 's': {0, 7, 4, 1, 7}, ' ': {},
}

func drawTinyText(frame *image.RGBA, x, y int, text string, ink color.NRGBA) {
	for _, character := range text {
		glyph := tinyGlyphs[character]
		for row, bits := range glyph {
			for column := range 3 {
				if bits&(1<<uint(2-column)) != 0 {
					frame.Set(x+column, y+row, ink)
				}
			}
		}
		x += 4
	}
}
