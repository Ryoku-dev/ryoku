// Package guard is the focus guard's pure half: which window names a feed
// the user asked to be kept off, and what the guard may do about it. It
// reads a window's app id and title and nothing else; asking, closing and
// announcing are the daemon's.
//
// A window whose app id names a feed (a dedicated app or a web app with its
// own id) may be closed, after a card that names the feed and the app. A
// window that names a feed only in its title is said aloud and never
// closed: in a browser the title is one tab's, and closing the window would
// close every other tab with it.
package guard

import (
	"strings"
	"unicode"
)

// Reach is what the guard may do about one window.
type Reach int

const (
	// None: the window names no feed.
	None Reach = iota
	// Announce: only the title names a feed; the guard says so and
	// closes nothing.
	Announce
	// Close: the app id names a feed; the guard asks before closing it.
	Close
)

// Hit is one window's match: the feed word it names and what the guard may
// do about it.
type Hit struct {
	Feed  string
	Reach Reach
}

// shortFeed is the length under which a feed word names an app only by its
// whole id or the last part of a reverse-domain id, never by any one part:
// "x" names the app "x", not "org.x.Viewer".
const shortFeed = 3

// Match reports which of words, in order, the window app and title name.
// words are clean feed words (protocol.CleanOniFeedWords). An app-id match
// wins over a title match, and the first word that matches is the one
// named.
func Match(app, title string, words []string) Hit {
	appParts := fields(app)
	for _, w := range words {
		if namesApp(appParts, w) {
			return Hit{Feed: w, Reach: Close}
		}
	}
	titleParts := fields(title)
	for _, w := range words {
		if run(titleParts, fields(w)) {
			return Hit{Feed: w, Reach: Announce}
		}
	}
	return Hit{}
}

// namesApp reports whether the app id's parts name the feed word w.
func namesApp(parts []string, w string) bool {
	feed := strings.Join(fields(w), "")
	if feed == "" || len(parts) == 0 {
		return false
	}
	if feed == strings.Join(parts, "") || feed == parts[len(parts)-1] {
		return true
	}
	if len([]rune(feed)) < shortFeed {
		return false
	}
	for _, p := range parts {
		if p == feed {
			return true
		}
	}
	return false
}

// run reports whether want appears in parts as consecutive whole parts.
func run(parts, want []string) bool {
	if len(want) == 0 {
		return false
	}
	for i := 0; i+len(want) <= len(parts); i++ {
		match := true
		for j, w := range want {
			if parts[i+j] != w {
				match = false
				break
			}
		}
		if match {
			return true
		}
	}
	return false
}

// fields lower-cases s and splits it into runs of letters and digits.
func fields(s string) []string {
	return strings.FieldsFunc(strings.ToLower(s), func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsDigit(r)
	})
}
