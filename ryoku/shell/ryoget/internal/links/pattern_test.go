package links

import (
	"slices"
	"strconv"
	"strings"
	"testing"
)

func TestExpandPattern(t *testing.T) {
	for _, c := range []struct {
		in   string
		want []string
	}{
		{"http://h/file[001-003].zip", []string{"http://h/file001.zip", "http://h/file002.zip", "http://h/file003.zip"}},
		{"http://h/p[8-10].jpg", []string{"http://h/p8.jpg", "http://h/p9.jpg", "http://h/p10.jpg"}},
		{"http://h/p[1-7:3]", []string{"http://h/p1", "http://h/p4", "http://h/p7"}},
		{"http://h/p[3-1]", []string{"http://h/p3", "http://h/p2", "http://h/p1"}},
		{"http://h/[a-c].txt", []string{"http://h/a.txt", "http://h/b.txt", "http://h/c.txt"}},
		{"http://h/[Z-X]", []string{"http://h/Z", "http://h/Y", "http://h/X"}},
		{"http://h/[a-b]/[1-2]", []string{"http://h/a/1", "http://h/a/2", "http://h/b/1", "http://h/b/2"}},
		{"http://h/[x]/[1-2]", []string{"http://h/[x]/1", "http://h/[x]/2"}},
	} {
		got, ranged := expandPattern(c.in, MaxExpanded)
		if !ranged || !slices.Equal(got, c.want) {
			t.Errorf("%s: %v (ranged %v), want %v", c.in, got, ranged, c.want)
		}
	}
	for _, in := range []string{"http://[::1]:8080/x", "http://h/a[0]", "http://h/[a-Z]", "http://h/[1-5:0]", "http://h/[1-2:x]", "http://h/[ab-cd]", "http://h/open[1-2", "http://h/plain"} {
		if got, ranged := expandPattern(in, MaxExpanded); ranged || got != nil {
			t.Errorf("%s expanded to %v", in, got)
		}
	}
	got, _ := expandPattern("http://h/[1-999999999999]/[a-z]", 30)
	if len(got) != 30 || got[0] != "http://h/1/a" || got[29] != "http://h/2/d" {
		t.Fatalf("capped = %d items, %v .. %v", len(got), got[0], got[len(got)-1])
	}
}

func TestParseExpandsRanges(t *testing.T) {
	res := Parse("grab https://files.example/disk[01-03].img and https://files.example/a.zip")
	var urls []string
	for _, l := range res.Links {
		urls = append(urls, l.URL)
	}
	want := []string{"https://files.example/disk01.img", "https://files.example/disk02.img", "https://files.example/disk03.img", "https://files.example/a.zip"}
	if !slices.Equal(urls, want) || res.Total != 4 || res.Cleaned != 0 {
		t.Fatalf("links = %v, %+v", urls, res)
	}
	if l := res.Links[1]; l.Raw != "https://files.example/disk[01-03].img" || !slices.Contains(l.Notes, NoteExpanded) {
		t.Fatalf("expanded link = %+v", l)
	}
	// a range in a bare address still gains https and the cleaning notes
	res = Parse("www.example.com/ep[1-2].mp4?utm_source=x")
	if res.Total != 2 || res.Links[1].URL != "https://www.example.com/ep2.mp4" || res.Cleaned != 2 ||
		!slices.Contains(res.Links[0].Notes, NoteHTTPS) || !slices.Contains(res.Links[0].Notes, NoteTracking) {
		t.Fatalf("bare = %+v", res)
	}
}

func TestParseCapsExpansion(t *testing.T) {
	var b strings.Builder
	for i := range 10 {
		b.WriteString("https://plain.example/" + strconv.Itoa(i) + " ")
	}
	b.WriteString("https://h.example/f[0001-9999].bin https://h.example/[a-z]")
	res := Parse(b.String())
	if res.Total != MaxExpanded || res.Links[MaxExpanded-1].URL != "https://h.example/f0490.bin" {
		t.Fatalf("total = %d, last %+v", res.Total, res.Links[len(res.Links)-1])
	}
	for _, l := range res.Links {
		if strings.Contains(l.URL, "[") {
			t.Fatalf("an unexpanded range slipped through: %s", l.URL)
		}
	}
}
