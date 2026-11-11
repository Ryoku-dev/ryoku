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

func TestCFamilyOutline(t *testing.T) {
	header := `#pragma once
#include "core/log.h"

namespace app {

// Serves commands over a socket.
class [[nodiscard]] Service : public QObject {
  Q_OBJECT
  Q_PROPERTY(int port READ port NOTIFY portChanged)

public:
  using Handler = std::function<std::string(const std::string&)>;
  explicit Service(QObject* parent = nullptr);
  ~Service() override;
  Service(const Service&) = delete;
  Service& operator=(const Service&) = delete;

  /** Starts listening. */
  bool start();
  int port() const noexcept { return port_; }
  void registerHandler(const std::string& name, Handler handler, std::string usage = {});
  static constexpr int kMaxClients = 8;

signals:
  void portChanged();

private:
  struct Entry {
    Handler handler;
  };
  enum class Mode : uint8_t { Idle, Busy };
  friend class Peer;
  int port_ = 0;
};

class Peer;
template <typename T> T clampTo(T value, T low, T high);
inline constexpr int kDefaultPort = 4000;

} // namespace app
`
	source := `#include "service.h"

namespace {
constexpr auto kTimeout = std::chrono::seconds(2);
}

bool Service::start() {
  if (port_ < 0) {
    return false;
  }
  return true;
}

void Service::registerHandler(
    const std::string& name, Handler handler, std::string usage) {
  handlers_[name] = {handler, usage};
}

Service::Service(QObject* parent) : QObject(parent), port_{0}, clients_{} {
  start();
}

static int helper(int value) { return value * 2; }
`
	cSource := `#include "list.h"
#ifdef __cplusplus
extern "C" {
#endif

typedef struct {
  int size;
} list_t;

typedef enum { LIST_OK, LIST_FULL } list_status;

/* Appends one item; the list grows by doubling. */
list_status
list_push (list_t *list, int item)
{
  const char *quote = "}{ not a brace";
  char brace = '}';
  return LIST_OK;
}

#ifdef __cplusplus
}
#endif
`
	type want struct {
		name, kind, parent string
		start, end         int
	}
	tests := []struct {
		path, source string
		language     string
		symbols      []want
		absent       []string
	}{
		{"service.h", header, "cpp", []want{
			{"Service", "class", "", 7, 34},
			{"Service::Handler", "type", "Service", 12, 12},
			{"Service::Service", "method", "Service", 13, 13},
			{"Service::~Service", "method", "Service", 14, 14},
			{"Service::operator=", "method", "Service", 16, 16},
			{"Service::start", "method", "Service", 19, 19},
			{"Service::port", "method", "Service", 20, 20},
			{"Service::registerHandler", "method", "Service", 21, 21},
			{"Service::kMaxClients", "const", "Service", 22, 22},
			{"Service::portChanged", "method", "Service", 25, 25},
			{"Service::Entry", "struct", "Service", 28, 30},
			{"Service::Mode", "enum", "Service", 31, 31},
			{"clampTo", "function", "", 37, 37},
			{"kDefaultPort", "const", "", 38, 38},
		}, []string{"Peer", "Q_OBJECT", "Q_PROPERTY", "port_", "Service::Peer"}},
		{"service.cpp", source, "cpp", []want{
			{"kTimeout", "const", "", 4, 4},
			{"Service::start", "method", "Service", 7, 12},
			{"Service::registerHandler", "method", "Service", 14, 17},
			{"Service::Service", "method", "Service", 19, 21},
			{"helper", "function", "", 23, 23},
		}, []string{"if", "port_", "clients_"}},
		{"list.c", cSource, "c", []want{
			{"list_t", "struct", "", 6, 8},
			{"list_status", "enum", "", 10, 10},
			{"list_push", "function", "", 13, 19},
		}, []string{"extern", "quote", "brace"}},
	}
	for _, test := range tests {
		t.Run(test.path, func(t *testing.T) {
			result, err := Extract(test.path, []byte(test.source))
			if err != nil {
				t.Fatal(err)
			}
			if result.Language != test.language {
				t.Fatalf("language = %q, want %q", result.Language, test.language)
			}
			byName := map[string]Symbol{}
			for _, symbol := range result.Symbols {
				if _, seen := byName[symbol.Name]; !seen {
					byName[symbol.Name] = symbol
				}
			}
			for _, expected := range test.symbols {
				symbol, ok := byName[expected.name]
				if !ok {
					t.Fatalf("missing %s in %v", expected.name, symbolNames(result.Symbols))
				}
				if symbol.Kind != expected.kind || symbol.Parent != expected.parent || symbol.Start != expected.start || symbol.End != expected.end {
					t.Fatalf("%s = %s parent %q %d-%d, want %s parent %q %d-%d", expected.name, symbol.Kind, symbol.Parent, symbol.Start, symbol.End, expected.kind, expected.parent, expected.start, expected.end)
				}
			}
			for _, name := range test.absent {
				if _, ok := byName[name]; ok {
					t.Fatalf("%s should not be a symbol: %v", name, symbolNames(result.Symbols))
				}
			}
		})
	}
	docs := map[string]string{"Service": "Serves commands over a socket.", "Service::start": "Starts listening.", "list_push": "Appends one item; the list grows by doubling."}
	for path, source := range map[string]string{"service.h": header, "list.c": cSource} {
		result, _ := Extract(path, []byte(source))
		for _, symbol := range result.Symbols {
			if doc, ok := docs[symbol.Name]; ok && symbol.Doc != doc {
				t.Fatalf("%s doc = %q, want %q", symbol.Name, symbol.Doc, doc)
			}
		}
	}
}

func symbolNames(symbols []Symbol) []string {
	names := make([]string, len(symbols))
	for index := range symbols {
		names[index] = symbols[index].Name
	}
	return names
}
