package protocol

import (
	"strings"
	"testing"
	"unicode/utf8"
)

func TestServerTextStripsControlAndFormatCharactersAndCuts(t *testing.T) {
	for in, want := range map[string]string{
		"plain":                                "plain",
		"  two\twords\n and\r\nlines  ":        "two words and lines",
		"\u202eevil\u202c \u2066x\u2069":       "evil x",
		"zero\u200bwidth\ufeffjoin\u2060ed":    "zerowidthjoined",
		"bell\x07 esc\x1b[31m del\x7f nul\x00": "bell esc [31m del nul",
		"line\u2028sep\u2029arator":            "line sep arator",
		"bad \xff\xfe utf8":                    "bad utf8",
		"":                                     "",
	} {
		if got := ServerText(in, ServerTextMax); got != want {
			t.Errorf("ServerText(%q) = %q, want %q", in, got, want)
		}
	}
	long := ServerText(strings.Repeat("é", 1000), ServerTextMax)
	if utf8.RuneCountInString(long) != ServerTextMax || !strings.HasSuffix(long, "…") || !utf8.ValidString(long) {
		t.Fatalf("cut text has %d runes: %q", utf8.RuneCountInString(long), long)
	}
	if got := ServerText("exactly", 7); got != "exactly" {
		t.Fatalf("text at the limit was cut: %q", got)
	}
}

func TestAccountManageOpensOnlyPlainWebAddresses(t *testing.T) {
	for raw, want := range map[string]string{
		"https://accounts.example.com/account": "https://accounts.example.com/account",
		"  https://accounts.example.com/a?b=1": "https://accounts.example.com/a?b=1",
		"http://127.0.0.1:3100/account":        "http://127.0.0.1:3100/account",
		"http://localhost:3100/account":        "http://localhost:3100/account",
		"http://[::1]:3100/account":            "http://[::1]:3100/account",
		"http://accounts.example.com/account":  "",
		"http://foo.localhost/account":         "",
		"https://u:p@accounts.example.com/":    "",
		"https://u@accounts.example.com/":      "",
		"ftp://accounts.example.com/":          "",
		"javascript:alert(1)":                  "",
		"mailto:a@b.example":                   "",
		"https:///account":                     "",
		"/account":                             "",
		"":                                     "",
	} {
		if got := AccountManage(raw); got != want {
			t.Errorf("AccountManage(%q) = %q, want %q", raw, got, want)
		}
	}
}

func TestIsHostedModelMatchesTheProviderPrefixOnly(t *testing.T) {
	for id, want := range map[string]bool{
		"ember/rashin-fast": true, "EMBER/x": true, " ember/x": true,
		"ember": false, "emberfoo/x": false, "openrouter/ember/x": false, "ryoku/x": false, "koe/x": false, "auto": false, "": false,
	} {
		if IsHostedModel(id) != want {
			t.Errorf("IsHostedModel(%q) = %v", id, !want)
		}
	}
}
