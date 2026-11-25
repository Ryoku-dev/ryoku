package shell

import (
	"fmt"
	"image"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/anim"
	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/design"
	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/palette"
)

// the overrides a per-design run of the suite sets would win over the files these tests write
func clearLookOverrides(t *testing.T) {
	t.Helper()
	for _, name := range []string{"LITHIC_DESIGN", "LITHIC_MODE", "LITHIC_PALETTE_JSON"} {
		t.Setenv(name, "")
	}
}

func writeDesktopFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

// Hyakki wears the look the desktop asks for and follows it while the window
// is open: picking a style, the wallpaper's colours or reduced motion in Ryoku
// Settings reshapes the window without a restart. Hyakki used to keep its own
// Colours, Contrast, Tint and Reduce motion and drew Lithic whatever the
// desktop chose, so Material, Adwaita and Fluent never reached it.
func TestTheWindowFollowsTheDesktopsLook(t *testing.T) {
	clearLookOverrides(t)
	stage := newStage(t, image.Pt(1440, 900))
	theme := stage.runtime.Theme()
	if theme.Design != design.Lithic || theme.Desktop.Mode != palette.Dark || !theme.Desktop.ReduceMotion {
		t.Fatalf("an untouched desktop opened as %v, mode %v, reduced %v; want Lithic, dark and the stage's reduced motion",
			theme.Design, theme.Desktop.Mode, theme.Desktop.ReduceMotion)
	}
	stock, shaper := theme.R, theme.Shaper
	shell := filepath.Join(os.Getenv("XDG_CONFIG_HOME"), "ryoku", "shell.json")

	for _, style := range []design.Style{design.Material, design.Adwaita, design.Fluent, design.Lithic} {
		writeDesktopFile(t, shell, fmt.Sprintf(`{"theme":{"design":%q,"motion":{"reduce":true}}}`, style.String()))
		stage.await("the window wears "+style.String(), func() bool { return stage.runtime.Theme().Design == style })
		if stage.runtime.Theme() != theme || theme.Shaper != shaper {
			t.Fatalf("%v came as a new theme or shaper, so every page's faces loaded again", style)
		}
	}
	// Adwaita sets the sidebar apart, and the window's ground is the side plane
	writeDesktopFile(t, shell, `{"theme":{"design":"adwaita","motion":{"reduce":true}}}`)
	stage.await("the window wears Adwaita again", func() bool { return theme.Design == design.Adwaita })
	if side := theme.Planes().Side; side != theme.R.Stone3 || side == theme.R.Ground {
		t.Fatalf("Adwaita's side plane is %v, want stone3 %v apart from the ground", side, theme.R.Stone3)
	}

	wallpaper := `{"surface":"#0f2418","onSurface":"#d6f2e0","primary":"#3cff8a","primaryContainer":"#1d5c3a","onPrimaryContainer":"#c4ffd8","error":"#ff5a4f"}`
	writeDesktopFile(t, filepath.Join(os.Getenv("XDG_CACHE_HOME"), "ryoku", "colors.json"), wallpaper)
	writeDesktopFile(t, shell, `{"theme":{"colours":"wallpaper","motion":{"reduce":true}}}`)
	stage.await("the wallpaper's colours reach the window", func() bool {
		return theme.Desktop.Colours == palette.Wallpaper && theme.Design == design.Lithic
	})
	if theme.R == stock {
		t.Fatal("following the wallpaper still paints the ryoku.dev stock")
	}

	// the demo has a working agent whose badge legitimately animates when motion is on
	stage.moving = true
	writeDesktopFile(t, shell, `{"theme":{"colours":"ryoku","motion":{"reduce":false}}}`)
	stage.await("the desktop's motion switch reaches the window", func() bool { return !theme.Desktop.ReduceMotion })
	if anim.Reduced() {
		t.Fatal("the desktop turned motion back on and the window still reduces it")
	}
	if theme.R != stock {
		t.Fatalf("back on Ryoku's colours the window paints gold %v, want the stock's %v", theme.R.Gold, stock.Gold)
	}
}

// A PNG is one look at one moment, so a render reads the desktop once and
// watches nothing: no helper runs for it and a change during it is not worn.
func TestARenderReadsTheDesktopOnceAndWatchesNothing(t *testing.T) {
	clearLookOverrides(t)
	root := t.TempDir()
	for name, directory := range map[string]string{
		"HOME": "home", "XDG_CONFIG_HOME": "config", "XDG_STATE_HOME": "state", "XDG_CACHE_HOME": "cache", "XDG_RUNTIME_DIR": "run",
	} {
		t.Setenv(name, filepath.Join(root, directory))
	}
	shell := filepath.Join(root, "config", "ryoku", "shell.json")
	writeDesktopFile(t, shell, `{"theme":{"design":"material"}}`)
	t.Cleanup(func() { anim.SetReduce(false) })
	r, err := startRuntime(Options{Demo: true, PNG: filepath.Join(root, "still.png")})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(r.Close)
	if r.Theme().Design != design.Material {
		t.Fatalf("the render drew %v, want the desktop's Material", r.Theme().Design)
	}

	writeDesktopFile(t, shell, `{"theme":{"design":"fluent"}}`)
	// past the watcher's 150 ms settle, had one been started
	time.Sleep(500 * time.Millisecond)
	r.wearDesktop()
	if r.Theme().Design != design.Material {
		t.Fatalf("a render followed the desktop to %v", r.Theme().Design)
	}
}
