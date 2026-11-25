package rules

import (
	"ryoku-rashin/internal/grammar/linttest"
	"testing"
)

func TestByExtension_And(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Well it's mostly arcane magic, as TTYs (and by extent PTYs) are an ancient technology used in teletypes", NewByExtension(), "Well it's mostly arcane magic, as TTYs (and by extension PTYs) are an ancient technology used in teletypes")
}

func TestByExtension_AndComma(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Since pushState does nothing to inform the browser--and, by extent, the screen reader--that new content is present", NewByExtension(), "Since pushState does nothing to inform the browser--and, by extension, the screen reader--that new content is present")
}

func TestByExtension_So(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Discord activities are only available on the Discord desktop client, so by extent the Stremio presence only works on desktop.", NewByExtension(), "Discord activities are only available on the Discord desktop client, so by extension the Stremio presence only works on desktop.")
}

func TestByExtension_Thus(t *testing.T) {
	linttest.AssertSuggestionResult(t, "This means it can emulate every other Turing complete language and thus by extent Java", NewByExtension(), "This means it can emulate every other Turing complete language and thus by extension Java")
}

func TestByExtension_WhichComma(t *testing.T) {
	linttest.AssertSuggestionResult(t, "which, by extent, includes 100% match with your OS file paths handling.", NewByExtension(), "which, by extension, includes 100% match with your OS file paths handling.")
}

func TestByExtension_DontFlagRasterContext(t *testing.T) {
	linttest.AssertNoLints(t, "Mask raster by extent in Python using rasterio", NewByExtension())
}

func TestByExtension_DontFlagFactUnitContext(t *testing.T) {
	linttest.AssertNoLints(t, "Group facts by extent. The unit is the extent, not the fact.", NewByExtension())
}

func TestByExtension_DontFlagGeometryTileContext(t *testing.T) {
	linttest.AssertNoLints(t, "Geometries that extend past the tile's area as defined by extent are often used as a buffer for rendering features that overlap multiple adjacent tiles.", NewByExtension())
}
