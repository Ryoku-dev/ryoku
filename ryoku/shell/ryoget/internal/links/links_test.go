package links

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func TestMain(m *testing.M) {
	root, err := os.MkdirTemp("", "ryoget-links-test-")
	if err != nil {
		panic(err)
	}
	for _, k := range []string{"XDG_CONFIG_HOME", "XDG_CACHE_HOME", "XDG_DATA_HOME", "XDG_STATE_HOME", "XDG_RUNTIME_DIR"} {
		os.Setenv(k, filepath.Join(root, strings.ToLower(k)))
	}
	code := m.Run()
	os.RemoveAll(root)
	os.Exit(code)
}

// one link in, one link out
func TestCleanOne(t *testing.T) {
	cases := []struct {
		in, want string
		notes    []string
	}{
		// untouched
		{"https://example.com/file.iso", "https://example.com/file.iso", nil},
		{"https://example.com/a?b=1&c=2", "https://example.com/a?b=1&c=2", nil},
		{"http://example.com/plain", "http://example.com/plain", nil},
		{"https://example.com/wiki/Foo_(bar)", "https://example.com/wiki/Foo_(bar)", nil},
		{"https://music.youtube.com/watch?v=abc", "https://music.youtube.com/watch?v=abc", nil},
		{"https://example.com/#/files/7", "https://example.com/#/files/7", nil},
		{"https://example.com/app#!/dl/9", "https://example.com/app#!/dl/9", nil},
		{"https://mega.nz/file/AbC#KeYkEy", "https://mega.nz/file/AbC#KeYkEy", nil},
		// signed: every parameter stays, tracking included
		{"https://bucket.s3.amazonaws.com/x.zip?X-Amz-Algorithm=AWS4&X-Amz-Signature=abc&utm_source=x",
			"https://bucket.s3.amazonaws.com/x.zip?X-Amz-Algorithm=AWS4&X-Amz-Signature=abc&utm_source=x", nil},
		{"https://cdn.test/v.mp4?Expires=1&Signature=s&Key-Pair-Id=k&fbclid=1",
			"https://cdn.test/v.mp4?Expires=1&Signature=s&Key-Pair-Id=k&fbclid=1", nil},
		{"https://cdn.test/a.bin?token=abc&utm_medium=y", "https://cdn.test/a.bin?token=abc&utm_medium=y", nil},
		{"https://cdn.test/a.bin?hdnts=exp=1~hmac=2", "https://cdn.test/a.bin?hdnts=exp=1~hmac=2", nil},
		{"https://cdn.test/a.bin?sig=1&si=2", "https://cdn.test/a.bin?sig=1&si=2", nil},
		{"https://storage.googleapis.com/b/o?X-Goog-Signature=1&gclid=2", "https://storage.googleapis.com/b/o?X-Goog-Signature=1&gclid=2", nil},
		{"https://cdn.test/a.bin?Policy=p&Signature=s#frag", "https://cdn.test/a.bin?Policy=p&Signature=s#frag", nil},
		// tracking
		{"https://example.com/a?utm_source=x&utm_medium=y&id=3", "https://example.com/a?id=3", []string{NoteTracking}},
		{"https://example.com/a?fbclid=abc", "https://example.com/a", []string{NoteTracking}},
		{"https://example.com/a?gclid=1&dclid=2&msclkid=3&yclid=4", "https://example.com/a", []string{NoteTracking}},
		{"https://example.com/a?mc_cid=1&mc_eid=2&keep=1", "https://example.com/a?keep=1", []string{NoteTracking}},
		{"https://example.com/a?_hsenc=1&_hsmi=2", "https://example.com/a", []string{NoteTracking}},
		{"https://www.instagram.com/p/X/?igsh=abc", "https://www.instagram.com/p/X/", []string{NoteTracking}},
		{"https://www.instagram.com/reel/X/?igshid=abc", "https://www.instagram.com/reel/X/", []string{NoteTracking}},
		{"https://open.spotify.com/track/1?si=abc", "https://open.spotify.com/track/1", []string{NoteTracking}},
		{"https://example.com/a?si=keep", "https://example.com/a?si=keep", nil},
		{"https://www.youtube.com/watch?v=ID&feature=share", "https://www.youtube.com/watch?v=ID", []string{NoteTracking}},
		{"https://www.youtube.com/watch?v=ID&feature=related2", "https://www.youtube.com/watch?v=ID&feature=related2", nil},
		{"https://www.youtube.com/watch?v=ID&pp=ygUF", "https://www.youtube.com/watch?v=ID", []string{NoteTracking}},
		{"https://x.com/u/status/1?s=20&t=abc", "https://x.com/u/status/1", []string{NoteTracking}},
		{"https://x.com/search?q=go&t=1", "https://x.com/search?q=go&t=1", nil},
		{"https://example.com/a?ref_src=twsrc&ref_url=x", "https://example.com/a", []string{NoteTracking}},
		// fragments that only mark a place are dropped
		{"https://example.com/a#section", "https://example.com/a", []string{NoteTrimmed}},
		// canonical
		{"https://youtu.be/aqz-KE-bpKQ?si=x", "https://www.youtube.com/watch?v=aqz-KE-bpKQ", []string{NoteCanonical, NoteTracking}},
		{"https://youtu.be/aqz-KE-bpKQ?t=42", "https://www.youtube.com/watch?v=aqz-KE-bpKQ&t=42", []string{NoteCanonical}},
		{"https://youtube.com/watch?v=aqz-KE-bpKQ", "https://www.youtube.com/watch?v=aqz-KE-bpKQ", []string{NoteCanonical}},
		{"https://m.youtube.com/watch?v=ID", "https://www.youtube.com/watch?v=ID", []string{NoteCanonical}},
		{"https://twitter.com/u/status/1", "https://x.com/u/status/1", []string{NoteCanonical}},
		{"https://mobile.twitter.com/u/status/1", "https://x.com/u/status/1", []string{NoteCanonical}},
		{"https://fxtwitter.com/u/status/1", "https://x.com/u/status/1", []string{NoteCanonical}},
		{"https://vxtwitter.com/u/status/1?s=46", "https://x.com/u/status/1", []string{NoteCanonical, NoteTracking}},
		{"https://fixupx.com/u/status/1", "https://x.com/u/status/1", []string{NoteCanonical}},
		{"https://fixvx.com/u/status/1", "https://x.com/u/status/1", []string{NoteCanonical}},
		{"https://m.facebook.com/watch/?v=1", "https://www.facebook.com/watch/?v=1", []string{NoteCanonical}},
		{"https://old.reddit.com/r/x/comments/1/t/", "https://www.reddit.com/r/x/comments/1/t/", []string{NoteCanonical}},
		{"https://new.reddit.com/r/x/", "https://www.reddit.com/r/x/", []string{NoteCanonical}},
		{"https://m.reddit.com/r/x/", "https://www.reddit.com/r/x/", []string{NoteCanonical}},
		{"https://en.m.wikipedia.org/wiki/Go", "https://en.wikipedia.org/wiki/Go", []string{NoteCanonical}},
		{"https://ddinstagram.com/p/X/", "https://www.instagram.com/p/X/", []string{NoteCanonical}},
		{"HTTPS://Example.COM/Path", "https://example.com/Path", nil},
		// redirectors
		{"https://www.google.com/url?sa=t&url=https%3A%2F%2Fexample.com%2Fa.iso&ved=1", "https://example.com/a.iso", []string{NoteUnwrapped}},
		{"https://www.google.co.uk/url?q=https://example.com/b&sa=D", "https://example.com/b", []string{NoteUnwrapped}},
		{"https://l.facebook.com/l.php?u=https%3A%2F%2Fexample.com%2Fc%3Ffbclid%3D1&h=x", "https://example.com/c", []string{NoteUnwrapped, NoteTracking}},
		{"https://www.youtube.com/redirect?event=video_description&q=https%3A%2F%2Fexample.com%2Fd", "https://example.com/d", []string{NoteUnwrapped}},
		{"https://out.reddit.com/t3_x?url=https%3A%2F%2Fexample.com%2Fe&token=1", "https://example.com/e", []string{NoteUnwrapped}},
		{"https://eur01.safelinks.protection.outlook.com/?url=https%3A%2F%2Fexample.com%2Ff&data=1", "https://example.com/f", []string{NoteUnwrapped}},
		{"https://duckduckgo.com/l/?uddg=https%3A%2F%2Fexample.com%2Fg&rut=1", "https://example.com/g", []string{NoteUnwrapped}},
		{"https://steamcommunity.com/linkfilter/?url=https://example.com/h", "https://example.com/h", []string{NoteUnwrapped}},
		{"https://t.umblr.com/redirect?z=https%3A%2F%2Fexample.com%2Fi&t=1", "https://example.com/i", []string{NoteUnwrapped}},
		{"https://www.google.com/url?q=https%3A%2F%2Fyoutu.be%2FID%3Fsi%3Dz", "https://www.youtube.com/watch?v=ID", []string{NoteUnwrapped, NoteCanonical, NoteTracking}},
		{"https://www.google.com/search?q=go", "https://www.google.com/search?q=go", nil},
		// torrents
		{"https://releases.test/os.iso.torrent", "https://releases.test/os.iso.torrent", nil},
	}
	for _, c := range cases {
		r := Parse(c.in)
		if len(r.Links) != 1 {
			t.Errorf("%q: %d links", c.in, len(r.Links))
			continue
		}
		l := r.Links[0]
		if l.URL != c.want {
			t.Errorf("%q:\n got %q\nwant %q", c.in, l.URL, c.want)
		}
		if !slices.Equal(l.Notes, c.notes) && !(len(l.Notes) == 0 && len(c.notes) == 0) {
			t.Errorf("%q: notes %v, want %v", c.in, l.Notes, c.notes)
		}
		if l.Raw != c.in {
			t.Errorf("%q: raw %q", c.in, l.Raw)
		}
	}
	if k := Parse("https://releases.test/os.iso.torrent").Links[0].Kind; k != "torrent" {
		t.Errorf("torrent kind = %q", k)
	}
}

func urls(t *testing.T, text string) []string {
	t.Helper()
	var out []string
	for _, l := range Parse(text).Links {
		if !l.Duplicate {
			out = append(out, l.URL)
		}
	}
	return out
}

func TestFindInText(t *testing.T) {
	cases := []struct {
		name, in string
		want     []string
	}{
		{"one per line", "https://a.test/1\nhttps://a.test/2\r\nhttps://a.test/3", []string{"https://a.test/1", "https://a.test/2", "https://a.test/3"}},
		{"commas and spaces", "https://a.test/1, https://a.test/2,https://a.test/3 https://a.test/4", []string{"https://a.test/1", "https://a.test/2", "https://a.test/3", "https://a.test/4"}},
		{"chat message", "hey grab https://a.test/x.iso (the new one), also www.b.test/y.zip. and this: youtu.be/abc!",
			[]string{"https://a.test/x.iso", "https://www.b.test/y.zip", "https://www.youtube.com/watch?v=abc"}},
		{"markdown", "see [the iso](https://a.test/x.iso) and [docs](<https://a.test/d (1).pdf>)", []string{"https://a.test/x.iso", "https://a.test/d%20%281%29.pdf"}},
		{"markdown text is a link", "[https://a.test/x](https://a.test/x)", []string{"https://a.test/x"}},
		{"angle brackets", "<https://a.test/a.iso?utm_source=x>", []string{"https://a.test/a.iso"}},
		{"quoted", `"https://a.test/q" and 'https://a.test/r'`, []string{"https://a.test/q", "https://a.test/r"}},
		{"parens around", "(https://a.test/p)", []string{"https://a.test/p"}},
		{"wiki parens kept", "read https://en.wikipedia.org/wiki/Go_(language).", []string{"https://en.wikipedia.org/wiki/Go_(language)"}},
		{"bare host with path", "get github.com/u/repo/releases/x.tar.gz now", []string{"https://github.com/u/repo/releases/x.tar.gz"}},
		{"no false positives", "version 1.2.3 is out, mail me at a@b.com, file.txt or e.g. this", nil},
		{"email with path-like tail", "write to bob@example.com/x", nil},
		{"unicode punctuation", "链接：https://a.test/z。", []string{"https://a.test/z"}},
		{"magnet", "magnet:?xt=urn:btih:ABCDEF&dn=Ubuntu&tr=udp%3A%2F%2Ftracker.test%3A1337&tr=udp://t2.test:80",
			[]string{"magnet:?xt=urn:btih:ABCDEF&dn=Ubuntu&tr=udp%3A%2F%2Ftracker.test%3A1337&tr=udp://t2.test:80"}},
		{"empty", "", nil},
	}
	for _, c := range cases {
		if got := urls(t, c.in); !slices.Equal(got, c.want) {
			t.Errorf("%s:\n got %q\nwant %q", c.name, got, c.want)
		}
	}
}

func TestChatMessageWithThreeLinks(t *testing.T) {
	r := Parse("Morning! The talk is at https://youtu.be/aqz-KE-bpKQ?si=Zx, slides on www.example.org/slides.pdf; " +
		"and the dataset (https://data.test/set.tar.gz?utm_campaign=news) if you want it.")
	if r.Total != 3 || r.Cleaned != 3 || r.Duplicates != 0 {
		t.Fatalf("counts = %d/%d/%d", r.Total, r.Cleaned, r.Duplicates)
	}
	want := []struct {
		url   string
		notes []string
	}{
		{"https://www.youtube.com/watch?v=aqz-KE-bpKQ", []string{NoteTrimmed, NoteCanonical, NoteTracking}},
		{"https://www.example.org/slides.pdf", []string{NoteTrimmed, NoteHTTPS}},
		{"https://data.test/set.tar.gz", []string{NoteTrimmed, NoteTracking}},
	}
	for i, w := range want {
		l := r.Links[i]
		if l.URL != w.url || !slices.Equal(l.Notes, w.notes) {
			t.Errorf("link %d = %q %v, want %q %v", i, l.URL, l.Notes, w.url, w.notes)
		}
	}
	if r.Links[1].Host != "www.example.org" || r.Links[0].Kind != "url" {
		t.Errorf("fields = %+v", r.Links)
	}
}

func TestDuplicatesAfterCleaning(t *testing.T) {
	r := Parse("see https://youtu.be/aqz-KE-bpKQ?si=x and https://youtube.com/watch?v=aqz-KE-bpKQ\n<https://example.com/a.iso?utm_source=x> https://example.com/a.iso")
	if r.Total != 4 || r.Duplicates != 2 || r.Cleaned != 3 {
		t.Fatalf("counts total=%d cleaned=%d dup=%d", r.Total, r.Cleaned, r.Duplicates)
	}
	dups := []bool{false, true, false, true}
	for i, l := range r.Links {
		if l.Duplicate != dups[i] {
			t.Errorf("link %d duplicate=%v", i, l.Duplicate)
		}
	}
	if r.Links[1].Raw != "https://youtube.com/watch?v=aqz-KE-bpKQ" {
		t.Errorf("raw = %q", r.Links[1].Raw)
	}
}

func TestMagnetUntouched(t *testing.T) {
	m := "magnet:?xt=urn:btih:0123456789abcdef0123456789abcdef01234567&dn=Big+Buck+Bunny&tr=udp%3A%2F%2Fexplodie.org%3A6969&tr=wss%3A%2F%2Ftracker.btorrent.xyz&utm_source=x"
	r := Parse("here: " + m + " enjoy")
	if len(r.Links) != 1 {
		t.Fatalf("links = %+v", r.Links)
	}
	l := r.Links[0]
	if l.URL != m || l.Kind != "magnet" || len(l.Notes) != 0 || l.Host != "" {
		t.Fatalf("magnet = %+v", l)
	}
}

func TestResultShape(t *testing.T) {
	r := Parse("nothing here")
	if r.Links == nil || r.Total != 0 {
		t.Fatalf("empty result = %+v", r)
	}
	for _, l := range Parse("https://a.test/x").Links {
		if l.Notes == nil {
			t.Fatal("notes must be an empty list, not null")
		}
	}
}
