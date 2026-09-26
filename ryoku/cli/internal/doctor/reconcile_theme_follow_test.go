package doctor

import "testing"

func TestThemeFollowHeal(t *testing.T) {
	cases := []struct {
		name string
		raw  string
		heal bool
		ok   bool
	}{
		{"missing key defaults to following", `{}`, false, true},
		{"already following", `{"followWallpaper":true}`, false, true},
		{"off with no lock is incoherent", `{"followWallpaper":false}`, true, true},
		{"off with the mono placeholder is incoherent", `{"followWallpaper":false,"scheme":"mono"}`, true, true},
		{"off with a locked palette is a choice", `{"followWallpaper":false,"scheme":"greek-noir"}`, false, true},
		{"off with a legacy locked theme is a choice", `{"followWallpaper":false,"theme":"greek-noir"}`, false, true},
		{"dynamic variant names are not locks", `{"followWallpaper":false,"theme":"Wallpaper"}`, true, true},
		{"unparseable document", `[1,2]`, false, false},
	}
	for _, c := range cases {
		heal, ok := themeFollowHeal(c.raw)
		if heal != c.heal || ok != c.ok {
			t.Fatalf("%s: themeFollowHeal(%s) = %v, %v; want %v, %v", c.name, c.raw, heal, ok, c.heal, c.ok)
		}
	}
}

func TestStaticPaletteName(t *testing.T) {
	for _, dynamic := range []string{"", "mono", "Default", "Wallpaper"} {
		if staticPaletteName(dynamic) {
			t.Fatalf("%q must read as dynamic, not a locked palette", dynamic)
		}
	}
	if !staticPaletteName("greek-noir") {
		t.Fatal("a curated scheme name must read as locked")
	}
}
