package profile

import (
	"bytes"
	"io/fs"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"ryoku-rashin/internal/oni/characters"
)

func assetTotal(files map[string][]byte) int {
	n := 0
	for _, b := range files {
		n += len(b)
	}
	return n
}

func TestAssetBytesAreExactlyTheDeclaredFiles(t *testing.T) {
	cases := []struct {
		id   string
		want []string
	}{
		{"testmoth", []string{"moth.svg", "poses/alert.svg", "poses/rest.svg", ProfileFile}},
		{"soft-missing-files", []string{"poses/alert.svg", "poses/rest.svg", ProfileFile}},
		{"soft-no-artwork", []string{"poses/alert.svg", "poses/rest.svg", ProfileFile}},
		{"soft-bare", []string{ProfileFile}},
	}
	for _, c := range cases {
		l, _ := Load(testdata, c.id)
		if !l.Valid {
			t.Fatalf("%s: %+v", c.id, l.Report)
		}
		got := slices.Sorted(maps.Keys(l.Files))
		if !slices.Equal(got, c.want) {
			t.Errorf("%s files = %q, want %q", c.id, got, c.want)
		}
		for name, data := range l.Files {
			disk, err := os.ReadFile(filepath.Join("testdata", c.id, filepath.FromSlash(name)))
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(data, disk) {
				t.Errorf("%s/%s differs from the file on disk", c.id, name)
			}
		}
		if assetTotal(l.Files) > MaxAssetBytes {
			t.Errorf("%s carries %d bytes", c.id, assetTotal(l.Files))
		}
	}
}

func TestShippedOniCarriesItsArtworkBytes(t *testing.T) {
	d := Default()
	got := slices.Sorted(maps.Keys(d.Files))
	if want := []string{ProfileFile, d.Profile.Artwork.Fallback}; !slices.Equal(got, want) {
		t.Fatalf("shipped files = %q, want %q", got, want)
	}
	for name, data := range d.Files {
		shipped, err := fs.ReadFile(characters.FS, ShippedID+"/"+name)
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(data, shipped) {
			t.Errorf("%s differs from the shipped file", name)
		}
	}
	d.Files[d.Profile.Artwork.Fallback][0] ^= 0xff
	if again := Default(); bytes.Equal(again.Files[again.Profile.Artwork.Fallback], d.Files[d.Profile.Artwork.Fallback]) {
		t.Fatal("a caller's change to the bytes reached the next load")
	}
}

func TestRejectedCharactersCarryNoBytes(t *testing.T) {
	entries, err := os.ReadDir("testdata")
	if err != nil {
		t.Fatal(err)
	}
	rejected := 0
	for _, e := range entries {
		l, _ := Load(testdata, e.Name())
		if l.Valid {
			continue
		}
		rejected++
		if l.Files != nil {
			t.Errorf("rejected %s carries files %q", e.Name(), slices.Sorted(maps.Keys(l.Files)))
		}
	}
	if rejected == 0 {
		t.Fatal("no rejected fixture ran")
	}
}

func TestAssetsOverTheBoundRejectTheCharacter(t *testing.T) {
	dir := t.TempDir()
	pieces := MaxAssetBytes/MaxSVGBytes + 1
	poses := map[string]any{}
	files := map[string][]byte{}
	for i := range pieces {
		name := "poses/p" + string(rune('a'+i)) + ".svg"
		files[name] = svgOfSize(MaxSVGBytes - 100)
		if i < 9 {
			poses[[]string{"idle", "ready", "listening", "working", "reading", "thinking", "speaking", "error", "sleeping"}[i]] = name
		}
	}
	outfits := map[string]any{}
	for i := 9; i < pieces; i++ {
		outfits["o"+string(rune('a'+i))] = "poses/p" + string(rune('a'+i)) + ".svg"
	}
	character(t, dir, "heavy", func(p map[string]any) {
		sub(p, "animations")["states"] = poses
		sub(p, "artwork")["outfits"] = outfits
	}, files)
	l, _ := Load(os.DirFS(dir), "heavy")
	if l.Valid || !l.Report.Has(ReasonTooLarge) || l.Files != nil {
		t.Fatalf("heavy: valid=%v errors=%q files=%d", l.Valid, l.Report.Errors, len(l.Files))
	}
}
