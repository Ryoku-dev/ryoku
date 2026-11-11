package main

import (
	"flag"
	"fmt"
	"image"
	"image/png"
	"os"
	"sort"
	"strings"
	"time"

	"gioui.org/app"
	"gioui.org/gpu/headless"
	"gioui.org/io/input"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/unit"

	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/anim"
	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/palette"
	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/theme"
)

const (
	galleryWidth  = 1440
	galleryHeight = 900
)

type pageFunc func(layout.Context, *theme.Theme) layout.Dimensions

var pages = map[string]pageFunc{
	"foundation": pageFoundation,
}

func registerPage(name string, page pageFunc) {
	if name == "" || page == nil {
		panic("gio gallery: invalid page registration")
	}
	if _, exists := pages[name]; exists {
		panic("gio gallery: duplicate page " + name)
	}
	pages[name] = page
}

func main() {
	pageName := flag.String("page", "foundation", "page: foundation, controls, lists, or overlays")
	modeName := flag.String("mode", "follow", "palette mode: follow, dark, or light")
	contrastName := flag.String("contrast", "standard", "palette contrast: standard or high")
	tintName := flag.String("tint", "off", "palette tint: off, low, or medium")
	pngPath := flag.String("png", "", "render the complete page to a PNG and exit")
	flag.Parse()

	page, ok := pages[*pageName]
	if !ok {
		fmt.Fprintf(os.Stderr, "gio gallery: page %q is unavailable; registered pages: %s\n", *pageName, strings.Join(pageNames(), ", "))
		os.Exit(2)
	}
	mode, err := parseMode(*modeName)
	if err != nil {
		fmt.Fprintln(os.Stderr, "gio gallery:", err)
		os.Exit(2)
	}
	th := theme.New(palette.FromPalette(palette.Load(mode), palette.ParseLook(*contrastName, *tintName)))
	if *pngPath != "" {
		if err := renderPNG(*pngPath, page, th); err != nil {
			fmt.Fprintln(os.Stderr, "gio gallery:", err)
			os.Exit(1)
		}
		return
	}
	go func() {
		if err := runWindow(page, th); err != nil {
			fmt.Fprintln(os.Stderr, "gio gallery:", err)
			os.Exit(1)
		}
		os.Exit(0)
	}()
	app.Main()
}

func pageNames() []string {
	names := make([]string, 0, len(pages))
	for name := range pages {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

func parseMode(value string) (palette.Mode, error) {
	switch value {
	case "follow":
		return palette.Follow, nil
	case "dark":
		return palette.Dark, nil
	case "light":
		return palette.Light, nil
	default:
		return palette.Follow, fmt.Errorf("invalid --mode %q: use follow, dark, or light", value)
	}
}

func runWindow(page pageFunc, th *theme.Theme) error {
	var window app.Window
	window.Option(
		app.Title("Lithic · Gio gallery"),
		app.Size(unit.Dp(galleryWidth), unit.Dp(galleryHeight)),
		app.MinSize(unit.Dp(760), unit.Dp(520)),
	)
	var operations op.Ops
	for {
		event := window.Event()
		switch event := event.(type) {
		case app.FrameEvent:
			gtx := app.NewContext(&operations, event)
			layoutPage(gtx, page, th)
			event.Frame(&operations)
		case app.DestroyEvent:
			return event.Err
		}
	}
}

func renderPNG(path string, page pageFunc, th *theme.Theme) error {
	anim.SetReduce(true)
	defer anim.SetReduce(false)

	metric := unit.Metric{PxPerDp: 1, PxPerSp: 1}
	var measureOps op.Ops
	var measureRouter input.Router
	measure := layout.Context{
		Ops: &measureOps, Now: time.Now(), Source: measureRouter.Source(), Metric: metric,
		Constraints: layout.Constraints{Min: image.Pt(galleryWidth, 0), Max: image.Pt(galleryWidth, 1<<20)},
	}
	height := max(galleryHeight, layoutPage(measure, page, th).Size.Y)
	window, err := headless.NewWindow(galleryWidth, height)
	if err != nil {
		return fmt.Errorf("create headless renderer: %w", err)
	}
	defer window.Release()

	var operations op.Ops
	var router input.Router
	gtx := layout.Context{
		Ops: &operations, Now: time.Now(), Source: router.Source(), Metric: metric,
		Constraints: layout.Exact(image.Pt(galleryWidth, height)),
	}
	layoutPage(gtx, page, th)
	if err := window.Frame(&operations); err != nil {
		return fmt.Errorf("render frame: %w", err)
	}
	frame := image.NewRGBA(image.Rect(0, 0, galleryWidth, height))
	if err := window.Screenshot(frame); err != nil {
		return fmt.Errorf("capture frame: %w", err)
	}
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

func layoutPage(gtx layout.Context, page pageFunc, th *theme.Theme) layout.Dimensions {
	paint.FillShape(gtx.Ops, th.R.Ground, clip.Rect{Max: gtx.Constraints.Max}.Op())
	gtx.Constraints.Min.Y = 0
	return page(gtx, th)
}
