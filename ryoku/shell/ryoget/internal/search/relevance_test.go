package search

import (
	"context"
	"sync"
	"testing"
)

func TestRelevanceGate(t *testing.T) {
	const obs = "wayland screen sharing obs"
	cases := []struct {
		name    string
		query   string
		gate    gate
		title   string
		snippet string
		pass    bool
	}{
		// noise the live run showed in All for "wayland screen sharing obs"
		{"crate without a term", obs, gateStrict, "micstudio", "Studio-grade mic processing", false},
		{"mdn one word", obs, gateStrict, "Cross-Origin Resource Sharing (CORS)", "CORS is an HTTP-header based mechanism", false},
		{"npm one word", obs, gateStrict, "is-wayland", "Check if the current Linux session is using Wayland", false},
		{"wikipedia off topic", obs, gateLenient, "Qt (software)", "Object-oriented framework for software development application for sharing a keyboard", false},
		{"flathub one word", obs, gateMedium, "OBS Studio", "Live stream and record videos", false},
		// what should stay
		{"repo name carries it", obs, gateMedium, "dennistdk/omarchy-screen-sharing-indicator", "Omarchy/Hyprland plugin", true},
		{"repo description carries it", obs, gateMedium, "fmarzuki/pointer", "on-screen annotation overlay for screen sharing. Works with OBS on GNOME Wayland", true},
		{"question title", obs, gateMedium, "How to hide a window from screen capture/screen sharing on Ubuntu (X11/Wayland)?", "", true},
		{"wiki page in snippet", obs, gateLenient, "V4l2loopback", "Screen casting: how to implement screen sharing by streaming", true},
		{"wiki title on topic", obs, gateLenient, "Wayland (protocol)", "Display system intended to replace X11", true},
		{"acronym spelled out", obs, gateLenient, "Open Broadcaster Software", "Open Broadcaster Software (OBS) is an open-source cross-platform", true},
		{"acronym needs three letters", "go", gateStrict, "Getting Online", "", false},
		// stemming, compounds and short words
		{"stems meet", "screen sharing", gateStrict, "Share your screen", "", true},
		{"longer word", "nvidia flicker", gateStrict, "Flickering with NVIDIA drivers", "", true},
		{"compound", "wayland", gateStrict, "Xwayland", "", true},
		{"short word", "go json", gateStrict, "json-iterator for Go", "", true},
		{"stopwords ignored", "how to use the screen capture api", gateStrict, "Using the Screen Capture API", "", true},
		{"single term package", "wayland", gateStrict, "is-wayland", "", true},
		{"single term miss", "wayland", gateStrict, "hyfetch", "neofetch with wayland support", false},
		{"answers always show", obs, gateNone, "anything", "", true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			q := newQuery(c.query, "en")
			it := Item{Title: c.title, Snippet: c.snippet}
			it.tcov, it.rel = q.relevance(it)
			if got := c.gate.pass(it); got != c.pass {
				t.Errorf("pass = %v, want %v (title %.2f, rel %.2f)", got, c.pass, it.tcov, it.rel)
			}
		})
	}
}

func obsFixtures(t *testing.T) *fixtures {
	f := newFixtures(t)
	f.set("crates.io/api/v1/crates", route{file: "obs_crates.json"})
	f.set("developer.mozilla.org/api/v1/search", route{file: "obs_mdn.json"})
	f.set("registry.npmjs.org/-/v1/search", route{file: "obs_npm.json"})
	f.set("en.wikipedia.org/w/rest.php/v1/search/page", route{file: "obs_wikipedia.json"})
	f.set("wiki.archlinux.org/api.php", route{file: "obs_archwiki.json"})
	f.set("api.github.com/search/repositories", route{file: "obs_github.json"})
	f.set("api.stackexchange.com/2.3/search/excerpts", route{file: "obs_stackoverflow.json"})
	f.set("hn.algolia.com/api/v1/search", route{file: "obs_hn.json"})
	f.set("flathub.org/api/v2/search", route{file: "obs_flathub.json"})
	f.set("api.keenable.ai/v1/search/public", route{body: `{"results":[{"title":"How to Enable Wayland Screen Sharing in OBS Studio","url":"https://example.com/obs-wayland","snippet":"Turn on **PipeWire** screen capture for OBS on a Wayland session."}]}`})
	return f
}

func collect(e *Service, q, scope string) map[string]doneMsg {
	return collectMode(e, q, scope, "full")
}

func collectMode(e *Service, q, scope, mode string) map[string]doneMsg {
	var mu sync.Mutex
	out := map[string]doneMsg{}
	e.query(context.Background(), 1, q, scope, mode, "en-US", func(v any) {
		if d, ok := v.(doneMsg); ok {
			mu.Lock()
			out[d.Source] = d
			mu.Unlock()
		}
	})
	return out
}

func titles(d doneMsg) []string {
	var out []string
	for _, it := range d.Items {
		out = append(out, it.Title)
	}
	return out
}

func TestAllScopeDropsNoiseFromRealResults(t *testing.T) {
	e := testService(t, obsFixtures(t), "os-release-arch")
	got := collect(e, "wayland screen sharing obs", "all")
	for _, src := range []string{"crates", "mdn", "npm", "wikipedia", "flathub"} {
		if d := got[src]; len(d.Items) != 0 || d.Rank != 0 || !d.More {
			t.Errorf("%s: want an empty section that still flags more, got %v rank=%d more=%v", src, titles(d), d.Rank, d.More)
		}
	}
	if d := got["github"]; len(d.Items) != 2 {
		t.Errorf("github kept %v", titles(d))
	}
	for src, d := range got {
		t.Logf("%-13s rank=%d more=%-5v %q", src, d.Rank, d.More, titles(d))
	}
	if d := got["stackoverflow"]; len(d.Items) == 0 {
		t.Errorf("stackoverflow kept %v", titles(d))
	}
	wiki := titles(got["archwiki"])
	if len(wiki) == 0 || wiki[0] != "Open Broadcaster Software" {
		t.Errorf("ArchWiki should lead with the OBS page, got %v", wiki)
	}
	for _, title := range wiki {
		if title == "DaVinci Resolve" {
			t.Errorf("off-topic wiki page kept: %v", wiki)
		}
	}
	if d := got["web"]; len(d.Items) == 0 || d.Rank != 6 {
		t.Errorf("the web lane leads whenever it has rows: rank=%d items=%d", d.Rank, len(d.Items))
	}
	for src, d := range got {
		if src != "web" && d.Rank >= 6 {
			t.Errorf("%s outranks the web section", src)
		}
	}

	// the source's own view keeps everything it returned
	full := collect(e, "wayland screen sharing obs", "crates")
	if n := len(full["crates"].Items); n != 10 {
		t.Errorf("crates view shows %d rows, want all 10", n)
	}
	scope := collect(e, "wayland screen sharing obs", "code")
	if n := len(scope["mdn"].Items); n != 10 {
		t.Errorf("code scope shows %d MDN rows, want all 10", n)
	}
}

func TestSectionRank(t *testing.T) {
	strong := []Item{{rel: 1}, {rel: 0.9}}
	weak := []Item{{rel: 0.3}, {rel: 0.25}, {rel: 0.25}}
	close1 := []Item{{rel: 0.8}}
	close2 := []Item{{rel: 0.77}}
	if sectionRank(strong) <= sectionRank(weak) {
		t.Error("a section of exact matches must outrank weak ones")
	}
	if sectionRank(close1) != sectionRank(close2) {
		t.Error("near-equal sections keep their fixed order")
	}
	if sectionRank(nil) != 0 {
		t.Error("an empty section ranks last")
	}
}
