package grievances

import (
	"os"
	"os/user"
	"regexp"
	"sync"
)

// redactor holds the patterns for this user, built once.
type redactor struct {
	home *regexp.Regexp
	name *regexp.Regexp
	mail *regexp.Regexp
}

var (
	redactOnce sync.Once
	redactWith redactor
)

// Redact takes the user's identity out of a report: the home folder becomes
// "~", and the login name becomes "<user>" where it names the user (a home
// path the first rewrite missed, "~name", a reverse-DNS label, name@host).
// Elsewhere the name stays, since logins are often ordinary words. root
// names no one, so only its home is rewritten.
func Redact(text string) string {
	redactOnce.Do(func() { redactWith = newRedactor() })
	r := redactWith
	if r.home != nil {
		text = r.home.ReplaceAllString(text, "~$1")
	}
	if r.name != nil {
		text = r.name.ReplaceAllString(text, "${1}<user>${2}")
		text = r.mail.ReplaceAllString(text, "${1}<user>@")
	}
	return text
}

func newRedactor() redactor {
	var r redactor
	if home, err := os.UserHomeDir(); err == nil && len(home) > 1 {
		r.home = regexp.MustCompile(`(?i)` + regexp.QuoteMeta(home) + `([^\pL\pN_.\-]|\.[^\pL\pN]|$)`)
	}
	name := os.Getenv("USER")
	if u, err := user.Current(); err == nil && u.Username != "" {
		name = u.Username
	}
	if name == "" || name == "root" {
		return r
	}
	quoted := regexp.QuoteMeta(name)
	r.name = regexp.MustCompile(`(?i)((?:^|[^\pL\pN])(?:users|home)[/\\-]|~|(?:^|[^\pL\pN])(?:com|org|net|io|dev)\.)` + quoted + `([^\pL\pN]|$)`)
	r.mail = regexp.MustCompile(`(?i)(^|[^\pL\pN])` + quoted + `@`)
	return r
}
