package youtube

import (
	"net/url"
	"regexp"
	"strings"
)

var idRe = regexp.MustCompile(`^[A-Za-z0-9_-]{11}$`)

// IsYouTube reports whether host belongs to YouTube (www, m, music, youtu.be,
// youtube-nocookie).
func IsYouTube(host string) bool {
	host = strings.TrimSuffix(strings.ToLower(host), ".")
	for _, d := range []string{"youtube.com", "youtu.be", "youtube-nocookie.com"} {
		if host == d || strings.HasSuffix(host, "."+d) {
			return true
		}
	}
	return false
}

// ParseURL pulls the video id and playlist id out of any YouTube link shape:
// watch, youtu.be, shorts, live, embed, music, playlist. Either may be empty.
func ParseURL(u *url.URL) (video, playlist string) {
	if u == nil || !IsYouTube(u.Hostname()) {
		return "", ""
	}
	q := u.Query()
	playlist = q.Get("list")
	if !validList(playlist) {
		playlist = ""
	}
	segs := strings.FieldsFunc(u.Path, func(r rune) bool { return r == '/' })
	host := strings.ToLower(u.Hostname())
	switch {
	case host == "youtu.be" || strings.HasSuffix(host, ".youtu.be"):
		if len(segs) > 0 {
			video = segs[0]
		}
	case len(segs) == 0:
	case segs[0] == "watch":
		video = q.Get("v")
	case len(segs) > 1 && (segs[0] == "shorts" || segs[0] == "live" || segs[0] == "embed" || segs[0] == "v" || segs[0] == "e"):
		video = segs[1]
	}
	if !idRe.MatchString(video) {
		video = ""
	}
	return video, playlist
}

func validList(id string) bool {
	if len(id) < 2 || len(id) > 64 {
		return false
	}
	for _, r := range id {
		if !(r >= 'A' && r <= 'Z' || r >= 'a' && r <= 'z' || r >= '0' && r <= '9' || r == '_' || r == '-') {
			return false
		}
	}
	return true
}

// WatchURL is the canonical page of a video.
func WatchURL(id string) string { return "https://www.youtube.com/watch?v=" + id }
