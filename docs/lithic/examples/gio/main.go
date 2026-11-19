package main

import (
	"flag"
	"fmt"
	"image"
	"image/png"
	"os"
	"path/filepath"
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
	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/design"
	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/palette"
	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/theme"
)

const (
	galleryWidth  = 1440
	galleryHeight = 900
)

type pageFunc func(layout.Context, *theme.Theme) layout.Dimensions

// galleryLook is the look the flags asked for; a page that shows another
// design beside the one in use takes that design's roles from it.
var galleryLook palette.Desktop

// themeFor returns a theme in style over the gallery's look, sharing th's
// shaper: building one loads every face.
func themeFor(th *theme.Theme, style design.Style) *theme.Theme {
	styled := theme.New(galleryLook.RolesFor(style), theme.WithShaper(th.Shaper))
	styled.Design = style
	return styled
}

// readLook resolves the flags without reading the desktop's choice: the
// signature for dark and light, this machine's palette for follow, or the
// Material roles in a file (a colors.json) when one is named.
func readLook(mode palette.Mode, style design.Style, look palette.Look, palettePath string) palette.Desktop {
	if palettePath == "" {
		d := palette.Desktop{Design: style, Mode: mode, Look: look, Palette: palette.Load(mode)}
		d.Roles = d.RolesFor(style)
		return d
	}
	// a named palette is followed whatever the mode, read as a host palette
	// so no file of this machine's desktop takes part
	os.Setenv("LITHIC_MODE", "follow")
	os.Setenv("LITHIC_DESIGN", style.String())
	os.Setenv("LITHIC_PALETTE_JSON", palettePath)
	nowhere := filepath.Join(os.TempDir(), "lithic-gallery-no-desktop")
	d := palette.ReadDesktop(palette.WithDirs(nowhere, nowhere, ""))
	d.Look = look
	d.Roles = d.RolesFor(style)
	return d
}

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
	pageName := flag.String("page", "foundation", "page: foundation, controls, lists, overlays, or designs")
	modeName := flag.String("mode", "dark", "palette mode: dark, light, or follow")
	designName := flag.String("design", "lithic", "design: lithic, material, adwaita, or fluent")
	palettePath := flag.String("palette", "", "follow the Material roles in this JSON file (a colors.json) instead of this machine's")
	contrastName := flag.String("contrast", "standard", "palette contrast: standard or high")
	tintName := flag.String("tint", "off", "palette tint: off, low, or medium")
	pngPath := flag.String("png", "", "render the complete page to a PNG and exit")
	scale := flag.Float64("scale", 1, "pixels per dp of a PNG render, such as 1.75 for a fractional display")
	flag.Parse()

	page, ok := pages[*pageName]
	if !ok {
		fmt.Fprintf(os.Stderr, "gio gallery: page %q is unavailable; registered pages: %s\n", *pageName, strings.Join(pageNames(), ", "))
		os.Exit(2)
	}
	mode, err := palette.ParseMode(*modeName)
	if err != nil {
		fmt.Fprintln(os.Stderr, "gio gallery:", err)
		os.Exit(2)
	}
	style, ok := design.Parse(*designName)
	if !ok {
		fmt.Fprintf(os.Stderr, "gio gallery: design %q is unavailable; designs: lithic, material, adwaita, fluent\n", *designName)
		os.Exit(2)
	}
	galleryLook = readLook(mode, style, palette.ParseLook(*contrastName, *tintName), *palettePath)
	th := theme.New(galleryLook.Roles)
	th.Design = style
	if *pngPath != "" {
		if err := renderPNG(*pngPath, page, th, float32(*scale)); err != nil {
			fmt.Fprintln(os.Stderr, "gio gallery:", err)
			os.Exit(1)
		}
		return
	}
	go func() {
		if err := runWindow(*pageName, page, th); err != nil {
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

func runWindow(name string, page pageFunc, th *theme.Theme) error {
	var window app.Window
	window.Option(
		app.Title("Lithic Gio gallery: "+name),
		app.Size(unit.Dp(galleryWidth), unit.Dp(galleryHeight)),
		app.MinSize(unit.Dp(760), unit.Dp(520)),
	)
	var operations op.Ops
	scroll := layout.List{Axis: layout.Vertical}
	for {
		event := window.Event()
		switch event := event.(type) {
		case app.FrameEvent:
			gtx := app.NewContext(&operations, event)
			paint.FillShape(gtx.Ops, th.Planes().Side, clip.Rect{Max: gtx.Constraints.Max}.Op())
			scroll.Layout(gtx, 1, func(gtx layout.Context, _ int) layout.Dimensions {
				return layoutPage(gtx, page, th)
			})
			event.Frame(&operations)
		case app.DestroyEvent:
			return event.Err
		}
	}
}

func renderPNG(path string, page pageFunc, th *theme.Theme, scale float32) error {
	anim.SetReduce(true)
	defer anim.SetReduce(false)

	if scale <= 0 {
		scale = 1
	}
	metric := unit.Metric{PxPerDp: scale, PxPerSp: scale}
	width := int(galleryWidth*scale + .5)
	var measureOps op.Ops
	var measureRouter input.Router
	measure := layout.Context{
		Ops: &measureOps, Now: time.Now(), Source: measureRouter.Source(), Metric: metric,
		Constraints: layout.Constraints{Min: image.Pt(width, 0), Max: image.Pt(width, 1<<20)},
	}
	height := max(int(galleryHeight*scale+.5), layoutPage(measure, page, th).Size.Y)
	window, err := headless.NewWindow(width, height)
	if err != nil {
		return fmt.Errorf("create headless renderer: %w", err)
	}
	defer window.Release()

	var operations op.Ops
	var router input.Router
	gtx := layout.Context{
		Ops: &operations, Now: time.Now(), Source: router.Source(), Metric: metric,
		Constraints: layout.Exact(image.Pt(width, height)),
	}
	layoutPage(gtx, page, th)
	if err := window.Frame(&operations); err != nil {
		return fmt.Errorf("render frame: %w", err)
	}
	frame := image.NewRGBA(image.Rect(0, 0, width, height))
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

// layoutPage lays the page on the window's ground, the design's side plane.
func layoutPage(gtx layout.Context, page pageFunc, th *theme.Theme) layout.Dimensions {
	paint.FillShape(gtx.Ops, th.Planes().Side, clip.Rect{Max: gtx.Constraints.Max}.Op())
	gtx.Constraints.Min.Y = 0
	return page(gtx, th)
}
