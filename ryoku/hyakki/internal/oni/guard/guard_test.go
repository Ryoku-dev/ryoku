package guard

import "testing"

func TestMatchClosesOnlyByAppIDAndAnnouncesTitles(t *testing.T) {
	words := []string{"reddit", "youtube shorts", "x", "tiktok"}
	cases := []struct {
		name, app, title string
		want             Hit
	}{
		{"dedicated app", "tiktok", "For You", Hit{"tiktok", Close}},
		{"reverse-domain app", "com.reddit.Reddit", "Home", Hit{"reddit", Close}},
		{"app id part", "reddit-desktop", "Home", Hit{"reddit", Close}},
		{"compact multi-word app", "youtube-shorts", "", Hit{"youtube shorts", Close}},
		{"short word, whole id", "X", "Home", Hit{"x", Close}},
		{"short word, last part", "com.example.x", "Home", Hit{"x", Close}},
		{"short word, inner part", "org.x.Viewer", "photo", Hit{}},
		{"browser tab title", "firefox", "Reddit - Dive into anything — Mozilla Firefox", Hit{"reddit", Announce}},
		{"browser multi-word title", "chromium", "YouTube Shorts - Chromium", Hit{"youtube shorts", Announce}},
		{"app id wins over title", "tiktok", "reddit thread", Hit{"tiktok", Close}},
		{"word inside a longer word", "firefox", "redditors unite", Hit{}},
		{"neutral window", "org.gnome.TextEditor", "notes.txt", Hit{}},
		{"nothing to read", "", "", Hit{}},
	}
	for _, c := range cases {
		if got := Match(c.app, c.title, words); got != c.want {
			t.Errorf("%s: Match(%q, %q) = %+v, want %+v", c.name, c.app, c.title, got, c.want)
		}
	}
	if got := Match("tiktok", "tiktok", nil); got != (Hit{}) {
		t.Errorf("no words matched %+v", got)
	}
}

// The first word in the user's order is the one a card names.
func TestMatchNamesTheFirstWord(t *testing.T) {
	if got := Match("chromium", "Reddit and TikTok", []string{"tiktok", "reddit"}); got != (Hit{"tiktok", Announce}) {
		t.Fatalf("got %+v", got)
	}
}
