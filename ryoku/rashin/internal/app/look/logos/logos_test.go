package logos

import (
	"encoding/json"
	"image"
	"image/color"
	"io/fs"
	"os"
	"strings"
	"testing"

	"ryoku-rashin/internal/protocol"
)

func TestEveryProviderIsListed(t *testing.T) {
	data, err := os.ReadFile("../../../gateway/catalog.json")
	if err != nil {
		t.Fatal(err)
	}
	var catalogue []struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(data, &catalogue); err != nil {
		t.Fatal(err)
	}
	ids := []string{protocol.ProviderOllamaLocal, protocol.ProviderLlamaCpp}
	for _, provider := range catalogue {
		ids = append(ids, provider.ID)
	}
	if len(ids) < 10 {
		t.Fatalf("read only %d providers", len(ids))
	}
	for _, id := range ids {
		if _, ok := table()[id]; !ok {
			t.Errorf("%s is not in list.txt: give it a mark or -", id)
		}
	}
}

func TestEveryMarkHasItsMaskAndNoMaskIsLeftOver(t *testing.T) {
	named := map[string]bool{}
	for id, mark := range table() {
		if mark == "" {
			continue
		}
		named[mark] = true
		if mask := decode(mark); mask == nil || mask.Rect.Dx() != 96 {
			t.Errorf("%s (%s) has no 96 px grey mask", id, mark)
		}
	}
	entries, err := fs.ReadDir(files, "masks")
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		if !named[strings.TrimSuffix(entry.Name(), ".png")] {
			t.Errorf("masks/%s is named by no id", entry.Name())
		}
	}
	for _, id := range []string{"openai", "anthropic", "claude", "codex", "gemini", "google"} {
		if markFor(id) == "" {
			t.Errorf("%s has no mark", id)
		}
	}
}

func TestScalingKeepsCoverageAndTintKeepsInk(t *testing.T) {
	mask := image.NewGray(image.Rect(0, 0, 96, 96))
	// the left half covered: any size keeps half its area covered
	for y := range 96 {
		for x := range 48 {
			mask.Pix[y*mask.Stride+x] = 255
		}
	}
	for _, px := range []int{13, 18, 24, 37, 96, 120} {
		var sum float32
		for _, value := range scale(mask, px) {
			sum += value
		}
		if area := sum / float32(px*px); area < 0.49 || area > 0.51 {
			t.Errorf("%d px covers %.3f, want 0.5", px, area)
		}
	}
	ink := color.NRGBA{R: 10, G: 20, B: 30, A: 200}
	tinted := tint([]float32{0, 0.5, 1, 1}, 2, ink)
	if got := tinted.NRGBAAt(1, 1); got != ink {
		t.Fatalf("a covered pixel is %v, want %v", got, ink)
	}
	if got := tinted.NRGBAAt(0, 0).A; got != 0 {
		t.Fatalf("an empty pixel has alpha %d", got)
	}
	if got := tinted.NRGBAAt(1, 0).A; got != 100 {
		t.Fatalf("a half pixel has alpha %d, want 100", got)
	}
}

func TestImageIsCachedPerInkAndSize(t *testing.T) {
	if _, ok := Image("ovh", 18, color.NRGBA{A: 255}); ok {
		t.Fatal("an id without a mark drew one")
	}
	if _, ok := Image("not-a-provider", 18, color.NRGBA{A: 255}); ok {
		t.Fatal("an unknown id drew a mark")
	}
	if _, ok := Image("openai", 0, color.NRGBA{A: 255}); ok {
		t.Fatal("a zero size drew a mark")
	}
	first, ok := Image("openai", 18, color.NRGBA{R: 200, A: 255})
	if !ok || first.Size() != image.Pt(18, 18) {
		t.Fatalf("openai at 18 px: %v %v", ok, first.Size())
	}
	again, _ := Image("openai", 18, color.NRGBA{R: 200, A: 255})
	other, _ := Image("openai", 18, color.NRGBA{G: 200, A: 255})
	if again != first || other == first {
		t.Fatal("the cache does not hold one image per mark, size and ink")
	}
}
