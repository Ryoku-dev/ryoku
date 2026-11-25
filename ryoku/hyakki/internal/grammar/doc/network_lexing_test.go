package doc

import (
	"math/rand/v2"
	"testing"
)

var domainParts = []string{"example.com", "example.com", "example.com", "and.subdomains.example.com", "example.com", "example.com", "example", "s.example", "example.org", "example.org", "example.org", "strange.example.com", "example.org", "example.org"}
var localParts = []string{"simple", "very.common", "x", "long.email-address-with-hyphens", "user.name+tag+sorting", "name/surname", "admin", "example", `" "`, `"john..doe"`, "mailhost!username", `"very.(),:;<>[]\".VERY.\"very@\\ \"very\".unusual"`, "user%example.com", "user-", "postmaster", "postmaster", "_test"}

func TestExampleLocalPartsPassValidation(t *testing.T) {
	for _, local := range localParts {
		if !validateLocalPart([]rune(local)) {
			t.Fatal(local)
		}
	}
}
func TestTestManyExampleEmailAddresses(t *testing.T) {
	for _, local := range localParts {
		for _, domain := range domainParts {
			address := []rune(local + "@" + domain)
			found, ok := LexEmailAddress(address)
			if !ok || found.NextIndex != len(address) {
				t.Fatal(string(address), found)
			}
		}
	}
}
func TestDoesNotAllowEmptyDomain(t *testing.T) {
	for _, local := range localParts {
		if found, ok := LexEmailAddress([]rune(local + "@ ")); ok {
			t.Fatal(found)
		}
	}
}
func TestCanParseExampleHostnames(t *testing.T) {
	for _, domain := range domainParts {
		n, ok := LexHostname([]rune(domain))
		if !ok || n != len([]rune(domain)) {
			t.Fatal(domain, n)
		}
	}
}
func TestHyphenCannotOpenHostname(t *testing.T) {
	if n, ok := LexHostname([]rune("-something.com")); ok {
		t.Fatal(n)
	}
}
func assertConsumesPart(t *testing.T, url string, n int) {
	t.Helper()
	found, ok := LexUrl([]rune(url))
	if !ok || found.NextIndex != n {
		t.Fatal(url, found)
	}
}
func TestConsumesGoogle(t *testing.T) { s := "https://google.com"; assertConsumesPart(t, s, len(s)) }
func TestConsumesWikipedia(t *testing.T) {
	s := "https://wikipedia.com"
	assertConsumesPart(t, s, len(s))
}
func TestConsumesYoutube(t *testing.T) { s := "https://youtube.com"; assertConsumesPart(t, s, len(s)) }
func TestConsumesYoutubeNotGarbage(t *testing.T) {
	assertConsumesPart(t, "https://youtube.com aklsjdha", 19)
}
func TestConsumesWithPath(t *testing.T) {
	s := "https://elijahpotter.dev/articles/quantifying_hope_on_a_global_scale"
	assertConsumesPart(t, s, len(s))
}
func TestConsumesIssue142(t *testing.T) {
	s := "https://github.com/nodesource/distributions#debinstall"
	assertConsumesPart(t, s, len(s))
}
func randomRunes(r *rand.Rand, buf []rune) {
	for i := range buf {
		c := r.IntN(0x110000 - 0x800)
		if c >= 0xd800 {
			c += 0x800
		}
		buf[i] = rune(c)
	}
}
func TestEmailSurvivesRandomChars(t *testing.T) {
	r := rand.New(rand.NewPCG(1, 2))
	buf := make([]rune, 128)
	for range 1 << 16 {
		randomRunes(r, buf)
		LexEmailAddress(buf)
	}
}
func TestUrlSurvivesRandomChars(t *testing.T) {
	r := rand.New(rand.NewPCG(3, 4))
	buf := make([]rune, 128)
	for range 1 << 16 {
		randomRunes(r, buf)
		LexUrl(buf)
	}
}
