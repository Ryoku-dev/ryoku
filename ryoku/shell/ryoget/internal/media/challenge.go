package media

import (
	"bytes"
	"net/http"
	"path"
	"strings"

	i18n "ryoku-i18n"
)

// interstitials only, ordinary Cloudflare pages also carry challenge-platform/scripts/jsd and cf_chl_rc
var challengeMarkers = [][]byte{
	[]byte("<title>just a moment...</title>"),
	[]byte("window._cf_chl_opt"),
	[]byte("/cdn-cgi/challenge-platform/h/"),
	[]byte("cf-browser-verification"),
	[]byte("<title>attention required! | cloudflare</title>"),
	[]byte("<title>ddos-guard</title>"),
	[]byte("/.well-known/ddos-guard/js-challenge"),
	[]byte("check.ddos-guard.net"),
	[]byte("<title>client challenge</title>"),
	[]byte(`name="js_challenge"`),
}

// an interstitial is a few kilobytes, a big page that merely mentions one is not
const challengeLimit = 2 << 20

// browserCheckHeader reports Cloudflare's own mark for a challenge answer.
// ported from prism-download-manager crates/prism-engine/src/http/fetcher.rs (MIT)
func browserCheckHeader(h http.Header) bool {
	return strings.EqualFold(strings.TrimSpace(h.Get("Cf-Mitigated")), "challenge")
}

// ChallengePage reports Cloudflare's challenge header or a small Cloudflare or DDoS-Guard check page.
func ChallengePage(h http.Header, body []byte) bool {
	if browserCheckHeader(h) {
		return true
	}
	if len(body) == 0 || len(body) > challengeLimit {
		return false
	}
	low := bytes.ToLower(body)
	for _, m := range challengeMarkers {
		if bytes.Contains(low, m) {
			return true
		}
	}
	return false
}

// BrowserCheckError is what a link that answered with a bot check fails with.
func BrowserCheckError() error {
	return BrowserCheck(i18n.T("This site checks for a browser before it hands out anything. Open the link in a browser instead."))
}

// binaryExts names files a page may link to for download.
// ported from prism-download-manager crates/prism-core/src/categories.rs (MIT)
var binaryExts = map[string]bool{
	// archives
	"zip": true, "7z": true, "rar": true, "tar": true, "gz": true, "tgz": true, "bz2": true, "tbz2": true,
	"xz": true, "txz": true, "zst": true, "tzst": true, "lz": true, "lzma": true, "lz4": true, "cab": true,
	// disk images and installers
	"iso": true, "img": true, "exe": true, "msi": true, "dmg": true, "pkg": true, "deb": true, "rpm": true,
	"appimage": true, "apk": true, "flatpakref": true, "flatpak": true, "snap": true, "torrent": true,
	// documents
	"pdf": true, "epub": true, "mobi": true, "djvu": true, "cbz": true, "cbr": true,
	// ROMs and disc dumps
	"gb": true, "gbc": true, "gba": true, "nes": true, "sfc": true, "smc": true, "n64": true, "z64": true,
	"v64": true, "nds": true, "3ds": true, "cia": true, "chd": true, "cue": true, "bin": true, "wbfs": true,
	"rvz": true, "cso": true, "pbp": true,
	// audio and video
	"mp3": true, "m4a": true, "flac": true, "wav": true, "ogg": true, "oga": true, "opus": true, "aac": true,
	"wma": true, "alac": true, "aiff": true, "mp4": true, "m4v": true, "mkv": true, "webm": true, "mov": true,
	"avi": true, "wmv": true, "flv": true, "mpg": true, "mpeg": true,
}

// fileExt returns the lowercase extension of a URL path when it names a downloadable file.
func fileExt(p string) (string, bool) {
	ext := strings.ToLower(strings.TrimPrefix(path.Ext(p), "."))
	return ext, ext != "" && binaryExts[ext]
}
