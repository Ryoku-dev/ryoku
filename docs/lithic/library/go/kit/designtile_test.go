package kit

import "testing"

func TestDesignGalleryColumns(t *testing.T) {
	// minimum catalogueTileWidthMin 200, gap space s2 8
	cases := []struct {
		width, minimum, gap, want int
	}{
		{0, 200, 8, 1},
		{150, 200, 8, 1},
		{200, 200, 8, 1},
		{407, 200, 8, 1},
		{408, 200, 8, 2},
		{616, 200, 8, 3},
		{1000, 200, 8, 4},
		{1032, 200, 8, 5},
		{1330, 200, 8, 6},
		{500, 0, 8, 1},
	}
	for _, test := range cases {
		if got := designGalleryColumns(test.width, test.minimum, test.gap); got != test.want {
			t.Errorf("designGalleryColumns(%d, %d, %d) = %d, want %d", test.width, test.minimum, test.gap, got, test.want)
		}
	}
}
