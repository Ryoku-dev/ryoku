package extract

import (
	"strconv"
	"strings"
	"testing"
)

func TestGoExtractorSpans(t *testing.T) {
	source := []byte("package sample\n\n// Item holds a value.\ntype Item struct { Value int }\n\nfunc (i *Item) Read() int {\n return i.Value\n}\n")
	result, err := Extract("sample.go", source)
	if err != nil {
		t.Fatal(err)
	}
	want := map[string][2]int{"Item": {4, 4}, "Item.Read": {6, 8}}
	for name, span := range want {
		found := false
		for _, symbol := range result.Symbols {
			if symbol.Name == name {
				found = true
				if symbol.Start != span[0] || symbol.End != span[1] {
					t.Fatalf("%s span = %d-%d, want %d-%d", name, symbol.Start, symbol.End, span[0], span[1])
				}
			}
		}
		if !found {
			t.Fatalf("missing symbol %s", name)
		}
	}
}
func TestLineExtractorSpans(t *testing.T) {
	tests := []struct {
		name, path, source, symbol string
		start, end                 int
	}{
		{"python", "sample.py", "class Item:\n    def read(self):\n        return 1\n\ndef top():\n    return 2\n", "Item.read", 2, 3},
		{"typescript", "sample.ts", "export class Item {\n  read() {\n    return 1\n  }\n}\nexport const Card = () => {\n return null\n}\n", "Item.read", 2, 4},
		{"qml", "Sample.qml", "import QtQuick\nItem {\n id: root\n property string title: \"x\"\n function open() {\n  return title\n }\n}\n", "open", 5, 7},
		{"bash", "sample.sh", "#!/bin/bash\nread_value() {\n echo value\n}\n", "read_value", 2, 4},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			result, err := Extract(test.path, []byte(test.source))
			if err != nil {
				t.Fatal(err)
			}
			for _, symbol := range result.Symbols {
				if symbol.Name == test.symbol {
					if symbol.Start != test.start || symbol.End != test.end {
						t.Fatalf("span = %d-%d, want %d-%d", symbol.Start, symbol.End, test.start, test.end)
					}
					return
				}
			}
			t.Fatalf("missing %s in %#v", test.symbol, result.Symbols)
		})
	}
}
func TestChunksKeepSmallSymbolWhole(t *testing.T) {
	source := ""
	for line := 1; line <= 100; line++ {
		source += "line\n"
	}
	chunks := Chunks(source, []Symbol{{Name: "whole", Start: 25, End: 80}})
	for _, chunk := range chunks {
		if chunk.Symbol == "whole" {
			if chunk.Start != 25 || chunk.End != 80 {
				t.Fatalf("symbol chunk = %d-%d", chunk.Start, chunk.End)
			}
			return
		}
	}
	t.Fatal("whole symbol chunk missing")
}
func TestRedactSecrets(t *testing.T) {
	source := "token = \"ghp_1234567890123456\"\n-----BEGIN PRIVATE KEY-----\nabc\n-----END PRIVATE KEY-----" // rashin-reviewer:allow private-key
	redacted := Redact(source)
	if redacted == source || contains(redacted, "ghp_") || contains(redacted, "BEGIN PRIVATE") {
		t.Fatalf("secret was not redacted: %s", redacted)
	}
}
func TestRedactPrivateKeyAcrossChunks(t *testing.T) {
	chunks := RedactChunks([]Chunk{{Text: "-----BEGIN PRIVATE KEY-----\nfirst"}, {Text: "second\n-----END PRIVATE KEY-----"}}) // rashin-reviewer:allow private-key
	if contains(chunks[0].Text, "first") || contains(chunks[1].Text, "second") {
		t.Fatalf("private key crossed chunk boundary: %#v", chunks)
	}
}
func contains(value, part string) bool {
	for index := 0; index+len(part) <= len(value); index++ {
		if value[index:index+len(part)] == part {
			return true
		}
	}
	return false
}

func TestRedactReviewFormats(t *testing.T) {
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
		{"jwt", `value eyJhbGciOiJIUzI1NiJ9.cGF5bG9hZA.c2lnbmF0dXJl`, "eyJhbGciOiJIUzI1NiJ9"},
		{"stripe", `sk_live_1234567890abcdef`, "sk_live_1234567890abcdef"},
		{"google", `AIza1234567890abcdef`, "AIza1234567890abcdef"},
		{"github oauth", `gho_1234567890abcdef`, "gho_1234567890abcdef"},
		{"github personal", `ghp_1234567890abcdef`, "ghp_1234567890abcdef"},
		{"github refresh", `ghr_1234567890abcdef`, "ghr_1234567890abcdef"},
		{"github server", `ghs_1234567890abcdef`, "ghs_1234567890abcdef"},
		{"github user", `ghu_1234567890abcdef`, "ghu_1234567890abcdef"},
		{"restricted live", `rk_live_1234567890abcdef`, "rk_live_1234567890abcdef"},
		{"whitespace assignment", `password whitespace-leak`, "whitespace-leak"},
		{"gitlab", `glpat-1234567890abcdef`, "glpat-1234567890abcdef"},
		{"slack app", `xapp-1234567890abcdef`, "xapp-1234567890abcdef"},
		{"npm", `npm_1234567890abcdef`, "npm_1234567890abcdef"},
		{"quoted apostrophe", `password = "it's here"`, "it's here"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			redacted := Redact(test.input)
			if strings.Contains(redacted, test.secret) || !strings.Contains(redacted, "[REDACTED]") {
				t.Fatalf("redaction = %q", redacted)
			}
		})
	}
}

func TestRedactPrivateKeyFormsAndLineCount(t *testing.T) {
	tests := []string{
		"-----BEGIN PGP PRIVATE KEY BLOCK-----\nPGP-SECRET\n-----END PGP PRIVATE KEY BLOCK-----\nafter", // rashin-reviewer:allow private-key
		"before\n-----BEGIN OPENSSH PRIVATE KEY-----\nOPENSSH-SECRET\nstill-secret",                     // rashin-reviewer:allow private-key
	}
	for _, source := range tests {
		redacted := RedactSource(source)
		if strings.Contains(redacted, "SECRET") || strings.Count(redacted, "\n") != strings.Count(source, "\n") {
			t.Fatalf("redaction = %q", redacted)
		}
	}
}

func TestPythonTrailingImportIsSafe(t *testing.T) {
	for _, source := range []string{"import os,\n", "import os, , sys\n"} {
		result, err := Extract("sample.py", []byte(source))
		if err != nil {
			t.Fatal(err)
		}
		if result.Language != "python" {
			t.Fatalf("language = %q", result.Language)
		}
	}
}

func TestChunksCapLongLines(t *testing.T) {
	chunks := Chunks(strings.Repeat("x", MaxChunkBytes*2+17), nil)
	if len(chunks) != 3 {
		t.Fatalf("chunks = %d", len(chunks))
	}
	for _, chunk := range chunks {
		if len(chunk.Text) > MaxChunkBytes {
			t.Fatalf("chunk bytes = %d", len(chunk.Text))
		}
	}
}

func TestGoGroupedDeclarationSpansAndSignatures(t *testing.T) {
	source := []byte("package sample\n\ntype (\n Alpha struct{}\n Beta interface{}\n)\nvar (\n First = 1\n Second = 2\n)\n")
	result, err := Extract("sample.go", source)
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]struct {
		line      int
		signature string
	}{"Alpha": {4, "Alpha struct{}"}, "Beta": {5, "Beta interface{}"}, "First": {8, "First = 1"}, "Second": {9, "Second = 2"}}
	for _, symbol := range result.Symbols {
		if expected, ok := want[symbol.Name]; ok {
			if symbol.Start != expected.line || symbol.Signature != expected.signature {
				t.Fatalf("%s = %d %q", symbol.Name, symbol.Start, symbol.Signature)
			}
			delete(want, symbol.Name)
		}
	}
	if len(want) != 0 {
		t.Fatalf("missing declarations: %#v", want)
	}
}

func TestJSONSymbolsRespectDepthSizeAndCap(t *testing.T) {
	small, err := Extract("small.json", []byte("{\n  \"top\": 1,\n  \"nested\": {\n    \"child\": 2\n  }\n}\n"))
	if err != nil {
		t.Fatal(err)
	}
	if names := symbolNames(small.Symbols); strings.Join(names, ",") != "top,nested,child" {
		t.Fatalf("small JSON symbols = %v", names)
	}
	largeSource := "{\n  \"top\": 1,\n  \"nested\": {\n    \"child\": 2\n  },\n  \"padding\": \"" + strings.Repeat("x", smallStructuredBytes) + "\"\n}\n"
	large, err := Extract("large.json", []byte(largeSource))
	if err != nil {
		t.Fatal(err)
	}
	if names := symbolNames(large.Symbols); strings.Join(names, ",") != "top,nested,padding" {
		t.Fatalf("large JSON symbols = %v", names)
	}
	var cappedSource strings.Builder
	cappedSource.WriteString("{\n")
	for index := range maxStructuredSymbols + 20 {
		cappedSource.WriteString("\"key")
		cappedSource.WriteString(strconv.Itoa(index))
		cappedSource.WriteString("\": 1,\n")
	}
	cappedSource.WriteString("\"last\": 1\n}\n")
	capped, err := Extract("capped.json", []byte(cappedSource.String()))
	if err != nil {
		t.Fatal(err)
	}
	if len(capped.Symbols) != maxStructuredSymbols {
		t.Fatalf("capped JSON symbols = %d", len(capped.Symbols))
	}
}

func symbolNames(symbols []Symbol) []string {
	names := make([]string, len(symbols))
	for index := range symbols {
		names[index] = symbols[index].Name
	}
	return names
}
