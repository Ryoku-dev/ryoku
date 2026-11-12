package redact

import (
	"strings"
	"testing"
)

func TestStrictTakesOutEveryFormOfSecret(t *testing.T) {
	tests := []struct{ name, input, secret string }{
		{"json password", `{"password": "hunter2"}`, "hunter2"},
		{"refresh token", `"refresh_token": "refresh-leak"`, "refresh-leak"},
		{"client secret", `"client_secret": "client-leak"`, "client-leak"},
		{"oauth token", `"oauth_token": "oauth-leak"`, "oauth-leak"},
		{"rpc password", `"rpc-password": "rpc-leak"`, "rpc-leak"},
		{"camel auth", `"authToken": "auth-leak"`, "auth-leak"},
		{"hf assignment", `HF_TOKEN=hf_1234567890abcdef`, "hf_1234567890abcdef"},
		{"github assignment", `export GITHUB_TOKEN=github_pat_1234567890abcdef`, "github_pat_1234567890abcdef"},
		{"access token", `access_token: access-leak`, "access-leak"},
		{"api key camel", `apiKey: api-leak`, "api-leak"},
		{"password compound", `my_password = password-leak`, "password-leak"},
		{"passwd", `passwd=passwd-leak`, "passwd-leak"},
		{"fish", `set -gx AWS_SECRET_ACCESS_KEY fish-leak`, "fish-leak"},
		{"url", `source = imaps://me%40mail.com:URLPASSWORD@imap.mail.com`, "URLPASSWORD"},
		{"bearer", `Authorization: Bearer eyJhbGciOiJIUzI1NiJ9.payload.signature`, "eyJhbGciOiJIUzI1NiJ9"},
		{"whitespace assignment", `password whitespace-leak`, "whitespace-leak"},
		{"quoted apostrophe", `password = "it's here"`, "it's here"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			redacted := Strict(test.input)
			if strings.Contains(redacted, test.secret) || !strings.Contains(redacted, Mark) {
				t.Fatalf("redaction = %q", redacted)
			}
		})
	}
}

func TestExactTakesOutTokensByTheirShape(t *testing.T) {
	tests := []struct{ name, input, secret string }{
		{"openai", `the key is sk-proj-1234567890abcdef`, "sk-proj-1234567890abcdef"},
		{"groq", `gsk_1234567890abcdef`, "gsk_1234567890abcdef"},
		{"hugging face", `hf_1234567890abcdef`, "hf_1234567890abcdef"},
		{"jwt", `value eyJhbGciOiJIUzI1NiJ9.cGF5bG9hZA.c2lnbmF0dXJl`, "eyJhbGciOiJIUzI1NiJ9"},
		{"stripe", `sk_live_1234567890abcdef`, "sk_live_1234567890abcdef"},
		{"google", `AIza1234567890abcdef`, "AIza1234567890abcdef"},
		{"aws", `AKIA1234567890ABCDEF`, "AKIA1234567890ABCDEF"},
		{"github oauth", `gho_1234567890abcdef`, "gho_1234567890abcdef"},
		{"github personal", `ghp_1234567890abcdef`, "ghp_1234567890abcdef"},
		{"github fine-grained", `github_pat_1234567890abcdef`, "github_pat_1234567890abcdef"},
		{"github refresh", `ghr_1234567890abcdef`, "ghr_1234567890abcdef"},
		{"github server", `ghs_1234567890abcdef`, "ghs_1234567890abcdef"},
		{"github user", `ghu_1234567890abcdef`, "ghu_1234567890abcdef"},
		{"restricted live", `rk_live_1234567890abcdef`, "rk_live_1234567890abcdef"},
		{"gitlab", `glpat-1234567890abcdef`, "glpat-1234567890abcdef"},
		{"slack bot", `xoxb-1234567890-abcdef`, "xoxb-1234567890-abcdef"},
		{"slack app", `xapp-1234567890abcdef`, "xapp-1234567890abcdef"},
		{"npm", `npm_1234567890abcdef`, "npm_1234567890abcdef"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			redacted := Exact(test.input)
			if strings.Contains(redacted, test.secret) || !strings.Contains(redacted, Mark) {
				t.Fatalf("redaction = %q", redacted)
			}
		})
	}
}

// Exact leaves alone what only sounds like a secret, which Strict takes out,
// and ordinary prose that shares a token's prefix.
func TestExactLeavesWhatOnlyLooksLikeASecret(t *testing.T) {
	for _, text := range []string{
		"password = hunter2",
		"export API_TOKEN=abc-def",
		"Authorization: Bearer abcdef",
		"Read the sk-learn-tutorial-for-beginners page, then ask about the hf_transfer module.",
		"Run go test ./... in ~/Work/app and keep the task-12345 note.",
	} {
		if got := Exact(text); got != text {
			t.Fatalf("Exact(%q) = %q", text, got)
		}
	}
	if got := Strict("password = hunter2"); strings.Contains(got, "hunter2") {
		t.Fatalf("Strict = %q", got)
	}
}

func TestPrivateKeysGoLineForLine(t *testing.T) {
	tests := []string{
		"-----BEGIN PGP PRIVATE KEY BLOCK-----\nPGP-SECRET\n-----END PGP PRIVATE KEY BLOCK-----\nafter", // rashin-reviewer:allow private-key
		"before\n-----BEGIN OPENSSH PRIVATE KEY-----\nOPENSSH-SECRET\nstill-secret",                     // rashin-reviewer:allow private-key
	}
	for _, source := range tests {
		for _, redacted := range []string{Exact(source), Strict(source), Patterns(source, nil)} {
			if strings.Contains(redacted, "SECRET") || strings.Contains(redacted, "BEGIN") || strings.Count(redacted, "\n") != strings.Count(source, "\n") {
				t.Fatalf("redaction = %q", redacted)
			}
		}
	}
	if got := Exact("-----BEGIN PGP PRIVATE KEY BLOCK-----\nx\n-----END PGP PRIVATE KEY BLOCK-----\nafter"); !strings.HasSuffix(got, "\nafter") { // rashin-reviewer:allow private-key
		t.Fatalf("the text after the block went too: %q", got)
	}
}

// A registered store's values go under its own name first, and Patterns, for
// bulk reads, leaves them to the patterns.
func TestRegisteredValuesAreScrubbedUntilRemoved(t *testing.T) {
	remove := Register(func(text string) string { return strings.ReplaceAll(text, "plain-value-42", "‹MAIL_PASSWORD›") })
	if got := Exact("login with plain-value-42 today"); got != "login with ‹MAIL_PASSWORD› today" {
		t.Fatalf("Exact = %q", got)
	}
	if got := Strict("pass plain-value-42"); got != "pass ‹MAIL_PASSWORD›" {
		t.Fatalf("Strict = %q", got)
	}
	if got := Patterns("pass plain-value-42", nil); got != "pass plain-value-42" {
		t.Fatalf("Patterns = %q", got)
	}
	remove()
	if got := Exact("login with plain-value-42 today"); got != "login with plain-value-42 today" {
		t.Fatalf("a removed store still scrubs: %q", got)
	}
}

// Complete follows every store's readiness, and a removed store no longer
// counts.
func TestCompleteWaitsForEveryStore(t *testing.T) {
	open := false
	remove := RegisterStore(func() func(string) string { return func(text string) string { return text } }, func() bool { return open })
	defer Register(func(text string) string { return text })()
	if Complete() {
		t.Fatal("a closed store counts as complete")
	}
	open = true
	if !Complete() {
		t.Fatal("every store is open, yet not complete")
	}
	open = false
	remove()
	if !Complete() {
		t.Fatal("a removed store still counts")
	}
}

// A snapshot opens each store once, however many texts it scrubs, keeps
// what it opened for its pass, and says whether every store was open.
func TestASnapshotOpensEachStoreOnce(t *testing.T) {
	opens, ready := 0, false
	value := "first-value-123"
	defer RegisterStore(func() func(string) string {
		opens++
		held := value
		return func(text string) string { return strings.ReplaceAll(text, held, "‹MAIL_PASSWORD›") }
	}, func() bool { return ready })()
	snapshot := Snapshot()
	for range 1000 {
		if got := snapshot.Exact("use first-value-123 with ghp_1234567890abcdef"); got != "use ‹MAIL_PASSWORD› with "+Mark {
			t.Fatalf("Exact = %q", got)
		}
	}
	if got := snapshot.Strict("password = first-value-123"); strings.Contains(got, "first-value-123") {
		t.Fatalf("Strict = %q", got)
	}
	if opens != 1 || snapshot.Complete() {
		t.Fatalf("opens = %d, complete = %v; want one open of a locked store", opens, snapshot.Complete())
	}
	value, ready = "second-value-456", true
	if got := snapshot.Exact("second-value-456"); got != "second-value-456" {
		t.Fatalf("a snapshot saw a value added after it: %q", got)
	}
	if again := Snapshot(); !again.Complete() || again.Exact("second-value-456") != "‹MAIL_PASSWORD›" || opens != 2 {
		t.Fatalf("a new snapshot: opens %d, complete %v", opens, again.Complete())
	}
}

// A learner remembers a found token whole and its body after the prefix,
// and scrubs both from later text; a short body, a token-like word with no
// digit and ordinary text teach it nothing.
func TestALearnerScrubsATokensLaterRestatements(t *testing.T) {
	var learner Learner
	scrub := learner.Wrap(Exact)
	body := "1a2B3c4D5e6F7g8H9i0J1k2L3m4N5o6P7q8R"
	if got := scrub("push with ghp_" + body); got != "push with "+Mark {
		t.Fatalf("the token itself: %q", got)
	}
	if got := scrub("the body is " + body + "."); got != "the body is "+Mark+"." {
		t.Fatalf("the body alone: %q", got)
	}
	before := learner.Len()
	for _, text := range []string{"hf_short1234", "sk-learn-tutorial-for-everyone", "nothing secret here"} {
		scrub(text)
	}
	if learner.Len() != before+1 {
		t.Fatalf("the learner holds %d literals after short and wordy tokens, want %d", learner.Len(), before+1)
	}
	if got := scrub("short1234 stays"); got != "short1234 stays" {
		t.Fatalf("a short body was learned: %q", got)
	}
	var other Learner
	if got := other.Wrap(Exact)(body); got != body {
		t.Fatalf("one learner's tokens reached another: %q", got)
	}
}
