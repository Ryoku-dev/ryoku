package media

import (
	"net/url"
	"regexp"
	"strings"
)

var messageURLs = regexp.MustCompile(`https?://[^\s<>"` + "`" + `]+`)

// URLs returns distinct HTTP links in message order, without prose punctuation.
func URLs(text string) []string {
	var urls []string
	seen := make(map[string]bool)
	for _, raw := range messageURLs.FindAllString(text, -1) {
		raw = strings.TrimRight(raw, ".,;:!?'\"।")
		for _, pair := range [][2]string{{"(", ")"}, {"[", "]"}, {"{", "}"}} {
			for strings.HasSuffix(raw, pair[1]) && strings.Count(raw, pair[1]) > strings.Count(raw, pair[0]) {
				raw = strings.TrimSuffix(raw, pair[1])
			}
		}
		u, err := url.Parse(raw)
		if err == nil && u.Hostname() != "" && u.User == nil && (u.Scheme == "http" || u.Scheme == "https") && !seen[raw] {
			seen[raw] = true
			urls = append(urls, raw)
		}
	}
	return urls
}

// KnownVideoURL identifies the sites whose links should report extraction errors.
func KnownVideoURL(raw string) bool {
	u, err := url.Parse(raw)
	if err != nil {
		return false
	}
	host := strings.ToLower(u.Hostname())
	for _, site := range []string{"youtube.com", "youtu.be", "vimeo.com"} {
		if host == site || strings.HasSuffix(host, "."+site) {
			return true
		}
	}
	return false
}
