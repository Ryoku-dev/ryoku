// Package links pulls download links out of pasted text and cleans them: no
// network, only what the text itself says. It finds plain links, Markdown and
// <angle> links, magnets and bare www. or host/path addresses; trims the
// punctuation a sentence leaves on them; unwraps redirectors; strips tracking
// parameters (never on signed download links); moves known sites to their
// normal address; and marks duplicates that only match once cleaned. Ranges
// like [001-100] and [a-z] expand into the links they stand for.
package links

import (
	"net/url"
	"regexp"
	"slices"
	"sort"
	"strings"
	"unicode/utf8"

	"ryoget/internal/proto"
)

const (
	NoteHTTPS     = "https-added"
	NoteTrimmed   = "trimmed"
	NoteUnwrapped = "unwrapped"
	NoteTracking  = "tracking-removed"
	NoteCanonical = "canonical"
	// one of the links a [1-10] or [a-z] range in the pasted text stands for
	NoteExpanded = "expanded"
)

var (
	markdownRe = regexp.MustCompile(`\[[^\]\n]*\]\(\s*(?:<([^<>\n]+)>|([^()\s<>]+(?:\([^()\s]*\)[^()\s<>]*)*))(?:\s+"[^"\n]*")?\s*\)`)
	// links pasted back to back with only a comma between them
	joinRe   = regexp.MustCompile(`(?i)[,;|]+(?:https?://|www\.|magnet:\?)`)
	magnetRe = regexp.MustCompile(`(?i)\bmagnet:\?[^\s<>"'\x60]+`)
	schemeRe = regexp.MustCompile(`(?i)\bhttps?://[^\s<>"'\x60]+`)
	wwwRe    = regexp.MustCompile(`(?i)\bwww\.[a-z0-9-]+(?:\.[a-z0-9-]+)+(?::\d+)?(?:[/?#][^\s<>"'\x60]*)?`)
	// a bare host is only a link when it carries a path and a familiar top-level domain
	bareRe = regexp.MustCompile(`(?i)\b(?:[a-z0-9](?:[a-z0-9-]*[a-z0-9])?\.)+(?:com|org|net|io|dev|app|be|tv|me|co|uk|de|fr|it|nl|es|se|no|fi|pl|ch|at|cz|ru|jp|kr|cn|in|br|ca|us|eu|au|nz|gg|ly|so|sh|to|xyz|info|edu|gov|moe|cc|fm|ai)(?::\d+)?/[^\s<>"'\x60]*`)
)

type found struct {
	at  int
	raw string
	// no scheme in the text: https:// is added
	bare bool
}

// Parse finds and cleans every link in text, in the order they appear.
func Parse(text string) proto.Links {
	var hits []found
	masked := []byte(text)
	// take records the first non-empty group of each match (the whole match when
	// there are none), cuts it where a second link was glued on, and masks it
	// out so later, looser patterns do not see it again
	take := func(re *regexp.Regexp, bare bool, ok func(start int) bool) {
		for {
			progress := false
			for _, m := range re.FindAllSubmatchIndex(masked, -1) {
				if ok != nil && !ok(m[0]) {
					continue
				}
				s, e := m[0], m[1]
				for g := 1; 2*g < len(m); g++ {
					if m[2*g] >= 0 {
						s, e = m[2*g], m[2*g+1]
						break
					}
				}
				end := m[1]
				if len(m) == 2 {
					if j := joinRe.FindStringIndex(text[s+1 : e]); j != nil {
						e = s + 1 + j[0]
						end = e
					}
				}
				hits = append(hits, found{at: s, raw: text[s:e], bare: bare})
				for i := m[0]; i < end; i++ {
					masked[i] = ' '
				}
				progress = true
			}
			if !progress {
				return
			}
		}
	}
	freeStart := func(start int) bool {
		if start == 0 {
			return true
		}
		r, _ := utf8.DecodeLastRune(masked[:start])
		return !(r == '@' || r == '.' || r == '/' || r == ':' || r == '-' || r == '_' || isWord(r))
	}
	take(markdownRe, false, nil)
	take(magnetRe, false, nil)
	take(schemeRe, false, nil)
	take(wwwRe, true, freeStart)
	take(bareRe, true, freeStart)
	sort.SliceStable(hits, func(i, j int) bool { return hits[i].at < hits[j].at })

	res := proto.Links{Links: []proto.Link{}}
	seen := map[string]bool{}
	add := func(l proto.Link) {
		if seen[l.URL] {
			l.Duplicate = true
			res.Duplicates++
		}
		seen[l.URL] = true
		if slices.ContainsFunc(l.Notes, func(n string) bool { return n != NoteExpanded }) {
			res.Cleaned++
		}
		res.Links = append(res.Links, l)
	}
	for _, h := range hits {
		if !strings.HasPrefix(strings.ToLower(h.raw), "magnet:") {
			if many, ranged := expandPattern(h.raw, MaxExpanded-len(res.Links)); ranged {
				for _, raw := range many {
					l, ok := clean(found{at: h.at, raw: raw, bare: h.bare})
					if !ok {
						continue
					}
					l.Raw = h.raw
					l.Notes = append(l.Notes, NoteExpanded)
					add(l)
				}
				continue
			}
		}
		l, ok := clean(h)
		if !ok {
			continue
		}
		add(l)
	}
	res.Total = len(res.Links)
	return res
}

func isWord(r rune) bool {
	return r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9'
}

type notes []string

func (n *notes) add(s string) {
	for _, x := range *n {
		if x == s {
			return
		}
	}
	*n = append(*n, s)
}

func clean(h found) (proto.Link, bool) {
	var ns notes
	s, trimmed := trim(h.raw)
	if trimmed {
		ns.add(NoteTrimmed)
	}
	if s == "" {
		return proto.Link{}, false
	}
	if strings.HasPrefix(strings.ToLower(s), "magnet:") {
		return proto.Link{Raw: h.raw, URL: s, Kind: "magnet", Notes: append([]string{}, ns...)}, true
	}
	if h.bare {
		s = "https://" + s
		ns.add(NoteHTTPS)
	}
	u, err := url.Parse(s)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return proto.Link{}, false
	}
	u.Scheme = strings.ToLower(u.Scheme)
	u.Host = strings.TrimSuffix(strings.ToLower(u.Host), ".")
	for range 4 {
		target := unwrap(u)
		if target == nil {
			break
		}
		u = target
		ns.add(NoteUnwrapped)
	}
	if !signed(u) {
		if canonical(u) {
			ns.add(NoteCanonical)
		}
		if stripTracking(u) {
			ns.add(NoteTracking)
		}
		if (u.Fragment != "" || u.RawFragment != "") && !fragmentMatters(u) {
			u.Fragment, u.RawFragment = "", ""
			ns.add(NoteTrimmed)
		}
	}
	kind := "url"
	if strings.HasSuffix(strings.ToLower(u.Path), ".torrent") {
		kind = "torrent"
	}
	return proto.Link{Raw: h.raw, URL: u.String(), Kind: kind, Host: u.Hostname(), Notes: append([]string{}, ns...)}, true
}

var closers = map[byte]byte{')': '(', ']': '[', '}': '{', '>': '<'}

// trim drops what a sentence leaves on a link: closing punctuation, quotes,
// and brackets that the link itself never opened.
func trim(s string) (string, bool) {
	orig := s
	for s != "" {
		r, size := utf8.DecodeLastRuneInString(s)
		switch {
		case strings.ContainsRune(".,;:!?'\"*…”’»「」『』。，、！？", r):
			s = s[:len(s)-size]
			continue
		case size == 1 && closers[s[len(s)-1]] != 0:
			c := s[len(s)-1]
			if strings.Count(s, string(closers[c])) < strings.Count(s, string(c)) {
				s = s[:len(s)-1]
				continue
			}
		}
		break
	}
	return s, s != orig
}

func hostIs(host string, domains ...string) bool {
	for _, d := range domains {
		if host == d || strings.HasSuffix(host, "."+d) {
			return true
		}
	}
	return false
}

func googleHost(host string) bool {
	host = strings.TrimPrefix(host, "www.")
	return host == "google.com" || strings.HasPrefix(host, "google.") || strings.HasPrefix(host, "google.co.")
}

// unwrap returns the link a redirector points at, or nil.
func unwrap(u *url.URL) *url.URL {
	q := u.Query()
	host, p := u.Hostname(), strings.TrimSuffix(u.Path, "/")
	var keys []string
	switch {
	case googleHost(host) && p == "/url":
		keys = []string{"q", "url"}
	case hostIs(host, "l.facebook.com", "lm.facebook.com", "l.messenger.com", "l.instagram.com", "l.threads.net") && (p == "/l.php" || p == ""):
		keys = []string{"u"}
	case hostIs(host, "youtube.com") && p == "/redirect":
		keys = []string{"q"}
	case host == "out.reddit.com":
		keys = []string{"url"}
	case strings.HasSuffix(host, ".safelinks.protection.outlook.com"):
		keys = []string{"url"}
	case hostIs(host, "duckduckgo.com") && (p == "/l" || p == ""):
		keys = []string{"uddg"}
	case host == "steamcommunity.com" && p == "/linkfilter":
		keys = []string{"url", "u"}
	case host == "t.umblr.com" && p == "/redirect":
		keys = []string{"z"}
	case host == "away.vk.com" || (hostIs(host, "vk.com") && p == "/away.php"):
		keys = []string{"to"}
	case hostIs(host, "linkedin.com") && p == "/redir/redirect":
		keys = []string{"url"}
	case host == "slack-redir.net" && p == "/link":
		keys = []string{"url"}
	case host == "exit.sc":
		keys = []string{"url"}
	default:
		return nil
	}
	for _, k := range keys {
		v := q.Get(k)
		if v == "" {
			continue
		}
		t, err := url.Parse(v)
		if err != nil || (t.Scheme != "http" && t.Scheme != "https") || t.Host == "" {
			continue
		}
		t.Scheme = strings.ToLower(t.Scheme)
		t.Host = strings.TrimSuffix(strings.ToLower(t.Host), ".")
		return t
	}
	return nil
}

// signed links keep every byte: their query is the credential.
func signed(u *url.URL) bool {
	for _, kv := range strings.Split(u.RawQuery, "&") {
		k, _, _ := strings.Cut(kv, "=")
		k, _ = url.QueryUnescape(k)
		k = strings.ToLower(k)
		switch {
		case strings.HasPrefix(k, "x-amz-"), strings.HasPrefix(k, "x-goog-"), strings.HasPrefix(k, "x-oss-"):
			return true
		}
		switch k {
		case "token", "sig", "signature", "expires", "policy", "key-pair-id", "hdnts", "hmac",
			"googleaccessid", "se", "sv", "st", "sp", "verify", "auth_key", "md5", "e", "x-expires":
			if k == "se" || k == "sv" || k == "st" || k == "sp" || k == "e" {
				// short names are only a signature alongside one of the long ones
				continue
			}
			return true
		}
	}
	return false
}

func isYouTube(host string) bool {
	return hostIs(host, "youtube.com", "youtu.be", "youtube-nocookie.com")
}

// tracking reports whether a query key carries nothing but attribution here.
func tracking(host, path, key, value string) bool {
	k := strings.ToLower(key)
	if strings.HasPrefix(k, "utm_") {
		return true
	}
	switch k {
	case "fbclid", "gclid", "dclid", "gbraid", "wbraid", "msclkid", "yclid", "twclid", "ttclid", "li_fat_id",
		"mc_cid", "mc_eid", "_hsenc", "_hsmi", "__hssc", "__hstc", "__hsfp", "hsctatracking",
		"igsh", "igshid", "ref_src", "ref_url", "_ga", "_gl", "oly_anon_id", "oly_enc_id",
		"vero_conv", "vero_id", "mkt_tok", "spm", "rdt", "epik", "ncid", "sr_share":
		return true
	case "si":
		return isYouTube(host) || hostIs(host, "spotify.com")
	case "pp":
		return isYouTube(host)
	case "feature":
		return isYouTube(host) && (value == "share" || value == "shared" || value == "youtu.be" || value == "emb_title")
	case "s", "t":
		return host == "x.com" && strings.Contains(path, "/status/")
	}
	return false
}

// stripTracking drops tracking parameters, keeping the rest in their order and encoding.
func stripTracking(u *url.URL) bool {
	if u.RawQuery == "" {
		return false
	}
	var kept []string
	changed := false
	for _, kv := range strings.Split(u.RawQuery, "&") {
		if kv == "" {
			changed = true
			continue
		}
		k, v, _ := strings.Cut(kv, "=")
		dk, _ := url.QueryUnescape(k)
		dv, _ := url.QueryUnescape(v)
		if tracking(u.Hostname(), u.Path, dk, dv) {
			changed = true
			continue
		}
		kept = append(kept, kv)
	}
	u.RawQuery = strings.Join(kept, "&")
	u.ForceQuery = false
	return changed
}

var hostMap = map[string]string{
	"youtube.com":          "www.youtube.com",
	"m.youtube.com":        "www.youtube.com",
	"mobile.youtube.com":   "www.youtube.com",
	"twitter.com":          "x.com",
	"www.twitter.com":      "x.com",
	"m.twitter.com":        "x.com",
	"mobile.twitter.com":   "x.com",
	"www.x.com":            "x.com",
	"m.x.com":              "x.com",
	"mobile.x.com":         "x.com",
	"fxtwitter.com":        "x.com",
	"www.fxtwitter.com":    "x.com",
	"vxtwitter.com":        "x.com",
	"www.vxtwitter.com":    "x.com",
	"fixupx.com":           "x.com",
	"fixvx.com":            "x.com",
	"m.facebook.com":       "www.facebook.com",
	"mobile.facebook.com":  "www.facebook.com",
	"facebook.com":         "www.facebook.com",
	"reddit.com":           "www.reddit.com",
	"old.reddit.com":       "www.reddit.com",
	"new.reddit.com":       "www.reddit.com",
	"m.reddit.com":         "www.reddit.com",
	"ddinstagram.com":      "www.instagram.com",
	"www.ddinstagram.com":  "www.instagram.com",
	"instagram.com":        "www.instagram.com",
	"m.wikipedia.org":      "wikipedia.org",
	"music.youtube.com":    "music.youtube.com",
	"www.youtu.be":         "youtu.be",
	"youtube-nocookie.com": "www.youtube.com",
}

// canonical moves a link to the address the site itself uses.
func canonical(u *url.URL) bool {
	host := u.Hostname()
	changed := false
	if to, ok := hostMap[host]; ok && to != host {
		host = to
		changed = true
	} else if strings.HasSuffix(host, ".m.wikipedia.org") {
		host = strings.TrimSuffix(host, ".m.wikipedia.org") + ".wikipedia.org"
		changed = true
	}
	if host == "youtu.be" {
		id := strings.Trim(u.Path, "/")
		if id != "" && !strings.Contains(id, "/") {
			host = "www.youtube.com"
			u.Path, u.RawPath = "/watch", ""
			rest := u.RawQuery
			u.RawQuery = "v=" + url.QueryEscape(id)
			if rest != "" {
				u.RawQuery += "&" + rest
			}
			changed = true
		}
	}
	if !changed {
		return false
	}
	if port := u.Port(); port != "" {
		host += ":" + port
	}
	u.Host = host
	return true
}

// fragmentMatters keeps fragments that are part of the address: hash-routed
// pages and share links whose decryption key lives after the #.
func fragmentMatters(u *url.URL) bool {
	f := u.EscapedFragment()
	if strings.HasPrefix(f, "/") || strings.HasPrefix(f, "!") {
		return true
	}
	return hostIs(u.Hostname(), "mega.nz", "mega.io", "mega.co.nz", "drive.proton.me", "wormhole.app",
		"send.vis.ee", "send.firefox.com", "pixeldrain.com", "files.catbox.moe")
}
