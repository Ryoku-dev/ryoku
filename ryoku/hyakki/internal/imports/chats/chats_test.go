package chats

import (
	"archive/zip"
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

const chatgptExport = `[{"id":"c-1","title":"Eigenvalues help","create_time":1709546400.5,"current_node":"n3",
 "mapping":{
  "root":{"parent":"","message":null},
  "n1":{"parent":"root","message":{"author":{"role":"system"},"content":{"content_type":"text","parts":["You are helpful"]}}},
  "n2":{"parent":"n1","message":{"author":{"role":"user"},"content":{"content_type":"text","parts":["What is an eigenvalue? My card is 4111 1111 1111 1111"]}}},
  "n2b":{"parent":"n1","message":{"author":{"role":"user"},"content":{"content_type":"text","parts":["an abandoned branch"]}}},
  "n3":{"parent":"n2","message":{"author":{"role":"assistant"},"content":{"content_type":"text","parts":["A root of the characteristic polynomial."]}}}
 }}]`

const claudeExport = `[{"uuid":"u-1","name":"Sourdough","created_at":"2026-02-01T09:00:00Z","chat_messages":[
 {"sender":"human","text":"How long to proof sourdough? key sk-ant-api03-AAAAbbbbCCCCddddEEEEffffGGGG"},
 {"sender":"assistant","text":"","content":[{"type":"text","text":"Usually 4 to 12 hours."}]}]}]`

const geminiExport = `[{"header":"Gemini Apps","title":"Prompted plan a trip to Kyoto","time":"2026-03-05T10:00:00.000Z","products":["Gemini Apps"],
 "safeHtmlItem":[{"html":"<p>Day one: Fushimi Inari &amp; Gion.</p><p>Day two: Arashiyama.</p>"}]},
 {"header":"Search","title":"Searched for cats","time":"2026-03-05T10:00:00.000Z","products":["Search"]}]`

func zipOf(t *testing.T, members map[string]string) string {
	t.Helper()
	var b bytes.Buffer
	w := zip.NewWriter(&b)
	for name, body := range members {
		f, err := w.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := f.Write([]byte(body)); err != nil {
			t.Fatal(err)
		}
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	p := filepath.Join(t.TempDir(), "export.zip")
	if err := os.WriteFile(p, b.Bytes(), 0o600); err != nil {
		t.Fatal(err)
	}
	return p
}

func fileOf(t *testing.T, name, body string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(p, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestImportEachSourceWithProvenanceScrubbed(t *testing.T) {
	root := filepath.Join(t.TempDir(), "imports", "chats")
	files := []string{
		zipOf(t, map[string]string{"conversations.json": chatgptExport, "chat.html": "<html></html>"}),
		zipOf(t, map[string]string{"data/conversations.json": claudeExport, "users.json": "[]"}),
		zipOf(t, map[string]string{"Takeout/My Activity/Gemini Apps/MyActivity.json": geminiExport}),
	}
	res, err := Import(root, files)
	if err != nil {
		t.Fatal(err)
	}
	if res.Chats != 3 || res.Skipped != 0 || len(res.List()) != 3 || res.List()[0].From != "chatgpt" {
		t.Fatalf("result %+v", res)
	}
	read := func(source string) string {
		t.Helper()
		entries, err := os.ReadDir(filepath.Join(root, source))
		if err != nil || len(entries) != 1 {
			t.Fatalf("%s: %v %v", source, entries, err)
		}
		p := filepath.Join(root, source, entries[0].Name())
		info, _ := os.Stat(p)
		dir, _ := os.Stat(filepath.Join(root, source))
		if info.Mode().Perm() != 0o600 || dir.Mode().Perm() != 0o700 {
			t.Fatalf("modes %v %v", info.Mode(), dir.Mode())
		}
		b, _ := os.ReadFile(p)
		return string(b)
	}
	gpt := read("chatgpt")
	if title, source, at := Head(gpt); title != "Eigenvalues help" || source != "ChatGPT" || !at.Equal(time.Date(2024, 3, 4, 0, 0, 0, 0, time.UTC)) {
		t.Fatalf("head %q %q %v", title, source, at)
	}
	if strings.Contains(gpt, "4111 1111 1111 1111") || !strings.Contains(gpt, "1111") || strings.Contains(gpt, "abandoned") || strings.Contains(gpt, "You are helpful") {
		t.Fatalf("chatgpt: %s", gpt)
	}
	if !strings.Contains(gpt, "**You:** What is an eigenvalue?") || !strings.Contains(gpt, "**Assistant:** A root") {
		t.Fatalf("turn order: %s", gpt)
	}
	claude := read("claude")
	if strings.Contains(claude, "sk-ant-api03") || !strings.Contains(claude, "Usually 4 to 12 hours.") || !strings.Contains(claude, "Source: Claude") {
		t.Fatalf("claude: %s", claude)
	}
	gemini := read("gemini")
	if !strings.Contains(gemini, "# plan a trip to Kyoto") || !strings.Contains(gemini, "Fushimi Inari & Gion.") || strings.Contains(gemini, "<p>") || strings.Contains(gemini, "cats") {
		t.Fatalf("gemini: %s", gemini)
	}
	// importing again replaces, never adds
	if _, err := Import(root, files[:1]); err != nil {
		t.Fatal(err)
	}
	read("chatgpt")
	// nothing of the export is kept beside the chats
	var all []string
	_ = filepath.WalkDir(root, func(p string, d os.DirEntry, _ error) error {
		if !d.IsDir() {
			all = append(all, filepath.Base(p))
		}
		return nil
	})
	for _, name := range all {
		if !strings.HasSuffix(name, ".md") || strings.HasPrefix(name, ".") {
			t.Fatalf("kept %s", name)
		}
	}
}

func TestPlainJSONExport(t *testing.T) {
	chats, err := Read(fileOf(t, "conversations.json", claudeExport))
	if err != nil || len(chats) != 1 || chats[0].Title != "Sourdough" {
		t.Fatalf("%+v %v", chats, err)
	}
}

func TestRefusals(t *testing.T) {
	for name, c := range map[string]struct {
		file string
		want error
	}{
		"traversal":      {zipOf(t, map[string]string{"../../.bashrc": "x", "conversations.json": claudeExport}), ErrTraversal},
		"absolute":       {zipOf(t, map[string]string{"/etc/passwd": "x"}), ErrTraversal},
		"bomb":           {zipOf(t, map[string]string{"conversations.json": "[" + strings.Repeat(" ", 4<<20) + "]"}), ErrTooBig},
		"no chats":       {zipOf(t, map[string]string{"conversations.json": `[{"x":1}]`}), ErrFormat},
		"malformed json": {fileOf(t, "conversations.json", `[{"uuid":`), ErrFormat},
		"not a zip":      {fileOf(t, "export.zip", "not a zip"), ErrRead},
		"missing":        {filepath.Join(t.TempDir(), "gone.json"), ErrRead},
	} {
		if _, err := Read(c.file); !errors.Is(err, c.want) {
			t.Errorf("%s: %v", name, err)
		}
	}
	root := t.TempDir()
	if _, err := Import(root, []string{zipOf(t, map[string]string{"../x.json": "[]"})}); !errors.Is(err, ErrTraversal) {
		t.Fatalf("import: %v", err)
	}
	if entries, _ := os.ReadDir(root); len(entries) != 0 {
		t.Fatal("a refused archive wrote something")
	}
}

func TestMarkdownCutsALongChat(t *testing.T) {
	c := Chat{Source: "claude", Title: "long", Turns: []Turn{{User: true, Text: strings.Repeat("a", MaxChatBytes)}, {Text: "b"}}}
	md, ok := Markdown(c, func(s string) string { return s })
	if !ok || len(md) > MaxChatBytes+64 || !strings.Contains(string(md), "was cut") {
		t.Fatalf("%d %v", len(md), ok)
	}
	if _, ok := Markdown(Chat{Title: "empty"}, func(s string) string { return s }); ok {
		t.Fatal("an empty chat")
	}
}
