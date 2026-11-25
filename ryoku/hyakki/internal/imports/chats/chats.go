// Package chats imports the chat exports other assistants give their users
// (ChatGPT's, Claude's and Gemini's) into Markdown files Hyakki can search.
// It runs only when the user picks the files; nothing watches a folder.
//
// The export is read in place and left there; nothing of it is kept but the
// chats themselves, one Markdown file each, with every secret scrubbed and
// every card, identity or account number masked. An archive whose member
// names leave the archive, or whose members are too big, is refused whole.
package chats

import (
	"archive/zip"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"ryoku-rashin/internal/protocol"
	"ryoku-rashin/internal/redact"
	"ryoku-rashin/internal/sensitive"
)

// Limits on what one import reads and writes. They bound the work an
// export can cause, not what a normal export holds.
const (
	// MaxFile is the most bytes read from one export file, or from one
	// archive member once uncompressed.
	MaxFile = 256 << 20
	// MaxArchive is the most bytes of an archive file.
	MaxArchive = 1 << 30
	// MaxMembers is the most members an archive may list.
	MaxMembers = 200000
	// MaxRatio is the most an archive member may expand over its
	// compressed size.
	MaxRatio = 200
	// MaxChats is the most chats one import keeps.
	MaxChats = 20000
	// MaxChatBytes is the most bytes of one chat's Markdown; the rest is
	// cut.
	MaxChatBytes = 512 << 10
	// MaxTitle is the longest title kept, in runes.
	MaxTitle = 120
)

// Why an export is refused.
var (
	ErrTraversal = errors.New("the archive names a file outside itself")
	ErrTooBig    = errors.New("the export is too big")
	ErrFormat    = errors.New("no ChatGPT, Claude or Gemini chats were found in it")
	ErrRead      = errors.New("the export could not be read")
)

// Turn is one message of a chat.
type Turn struct {
	User bool
	Text string
}

// Chat is one parsed conversation.
type Chat struct {
	Source string
	ID     string
	Title  string
	At     time.Time
	Turns  []Turn
}

// Result is what one import came to.
type Result struct {
	Chats   int
	Skipped int
	Sources map[string]int
}

// Import reads each file, .json or .zip, and writes its chats under root,
// one folder per source. A refused file stops the import with the reason;
// files read before it stay imported.
func Import(root string, files []string) (Result, error) {
	res := Result{Sources: map[string]int{}}
	scrub := redact.Snapshot()
	for _, file := range files {
		chats, err := Read(file)
		if err != nil {
			return res, fmt.Errorf("%s: %w", filepath.Base(file), err)
		}
		for _, c := range chats {
			if res.Chats >= MaxChats {
				res.Skipped++
				continue
			}
			md, ok := Markdown(c, func(s string) string { return sensitive.Mask(scrub.Strict(s)) })
			if !ok {
				res.Skipped++
				continue
			}
			if err := write(root, c, md); err != nil {
				return res, err
			}
			res.Chats++
			res.Sources[c.Source]++
		}
	}
	return res, nil
}

// Read parses one export file.
func Read(file string) ([]Chat, error) {
	info, err := os.Stat(file)
	if err != nil || !info.Mode().IsRegular() {
		return nil, ErrRead
	}
	if strings.EqualFold(filepath.Ext(file), ".zip") {
		if info.Size() > MaxArchive {
			return nil, ErrTooBig
		}
		return readZip(file)
	}
	if info.Size() > MaxFile {
		return nil, ErrTooBig
	}
	b, err := os.ReadFile(file)
	if err != nil {
		return nil, ErrRead
	}
	chats, ok := Parse(b, "")
	if !ok {
		return nil, ErrFormat
	}
	return chats, nil
}

func readZip(file string) ([]Chat, error) {
	z, err := zip.OpenReader(file)
	if err != nil {
		return nil, ErrRead
	}
	defer z.Close()
	if len(z.File) > MaxMembers {
		return nil, ErrTooBig
	}
	// every name is checked before anything is read
	var wanted []*zip.File
	for _, f := range z.File {
		if !inside(f.Name) {
			return nil, ErrTraversal
		}
		if f.UncompressedSize64 > MaxFile || f.CompressedSize64 > 0 && f.UncompressedSize64/f.CompressedSize64 > MaxRatio {
			return nil, ErrTooBig
		}
		if wantedMember(f.Name) {
			wanted = append(wanted, f)
		}
	}
	var out []Chat
	found := false
	for _, f := range wanted {
		b, err := readMember(f)
		if err != nil {
			return nil, err
		}
		chats, ok := Parse(b, f.Name)
		if ok {
			found = true
			out = append(out, chats...)
		}
	}
	if !found {
		return nil, ErrFormat
	}
	return out, nil
}

// inside reports whether an archive member's name stays in the archive.
func inside(name string) bool {
	if name == "" || strings.ContainsRune(name, '\\') || strings.ContainsRune(name, 0) || path.IsAbs(name) {
		return false
	}
	for _, part := range strings.Split(name, "/") {
		if part == ".." {
			return false
		}
	}
	return true
}

// wantedMember is a member an export keeps its chats in.
func wantedMember(name string) bool {
	base := strings.ToLower(path.Base(name))
	if base == "conversations.json" {
		return true
	}
	return strings.HasSuffix(base, ".json") && strings.Contains(strings.ToLower(name), "gemini")
}

// readMember reads one member, never more than MaxFile bytes whatever its
// header says.
func readMember(f *zip.File) ([]byte, error) {
	r, err := f.Open()
	if err != nil {
		return nil, ErrRead
	}
	defer r.Close()
	b, err := io.ReadAll(io.LimitReader(r, MaxFile+1))
	if err != nil {
		return nil, ErrRead
	}
	if len(b) > MaxFile {
		return nil, ErrTooBig
	}
	return b, nil
}

// Parse reads the chats in one export JSON file, whichever assistant made
// it. name is the archive member's name, "" for a file of its own.
func Parse(b []byte, name string) ([]Chat, bool) {
	var probe []map[string]json.RawMessage
	if err := json.Unmarshal(b, &probe); err != nil || len(probe) == 0 {
		return nil, false
	}
	first := probe[0]
	switch {
	case first["mapping"] != nil:
		return parseChatGPT(b)
	case first["chat_messages"] != nil:
		return parseClaude(b)
	case first["header"] != nil || first["products"] != nil:
		return parseGemini(b)
	}
	return nil, false
}

type chatgptConversation struct {
	ID          string                 `json:"id"`
	Title       string                 `json:"title"`
	CreateTime  float64                `json:"create_time"`
	CurrentNode string                 `json:"current_node"`
	Mapping     map[string]chatgptNode `json:"mapping"`
}

type chatgptNode struct {
	Parent  string `json:"parent"`
	Message *struct {
		Author struct {
			Role string `json:"role"`
		} `json:"author"`
		Content struct {
			ContentType string            `json:"content_type"`
			Parts       []json.RawMessage `json:"parts"`
		} `json:"content"`
	} `json:"message"`
}

func parseChatGPT(b []byte) ([]Chat, bool) {
	var convs []chatgptConversation
	if json.Unmarshal(b, &convs) != nil {
		return nil, false
	}
	var out []Chat
	for _, c := range convs {
		chat := Chat{Source: protocol.OniImportChatGPT, ID: c.ID, Title: c.Title, At: unix(c.CreateTime)}
		// walk the branch the user ended on, back to its root
		var turns []Turn
		seen := map[string]bool{}
		for id := c.CurrentNode; id != "" && !seen[id]; id = c.Mapping[id].Parent {
			seen[id] = true
			n, ok := c.Mapping[id]
			if !ok {
				break
			}
			if n.Message == nil || n.Message.Content.ContentType != "text" {
				continue
			}
			role := n.Message.Author.Role
			if role != "user" && role != "assistant" {
				continue
			}
			var parts []string
			for _, p := range n.Message.Content.Parts {
				var s string
				if json.Unmarshal(p, &s) == nil && strings.TrimSpace(s) != "" {
					parts = append(parts, s)
				}
			}
			if len(parts) > 0 {
				turns = append(turns, Turn{User: role == "user", Text: strings.Join(parts, "\n")})
			}
		}
		for i, j := 0, len(turns)-1; i < j; i, j = i+1, j-1 {
			turns[i], turns[j] = turns[j], turns[i]
		}
		chat.Turns = turns
		out = append(out, chat)
	}
	return out, len(out) > 0
}

type claudeConversation struct {
	UUID      string `json:"uuid"`
	Name      string `json:"name"`
	CreatedAt string `json:"created_at"`
	Messages  []struct {
		Sender  string `json:"sender"`
		Text    string `json:"text"`
		Content []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"content"`
	} `json:"chat_messages"`
}

func parseClaude(b []byte) ([]Chat, bool) {
	var convs []claudeConversation
	if json.Unmarshal(b, &convs) != nil {
		return nil, false
	}
	var out []Chat
	for _, c := range convs {
		chat := Chat{Source: protocol.OniImportClaude, ID: c.UUID, Title: c.Name, At: stamp(c.CreatedAt)}
		for _, m := range c.Messages {
			if m.Sender != "human" && m.Sender != "assistant" {
				continue
			}
			text := m.Text
			if strings.TrimSpace(text) == "" {
				var parts []string
				for _, p := range m.Content {
					if p.Type == "text" && strings.TrimSpace(p.Text) != "" {
						parts = append(parts, p.Text)
					}
				}
				text = strings.Join(parts, "\n")
			}
			if strings.TrimSpace(text) != "" {
				chat.Turns = append(chat.Turns, Turn{User: m.Sender == "human", Text: text})
			}
		}
		out = append(out, chat)
	}
	return out, len(out) > 0
}

// geminiActivity is one entry of Google's activity export for Gemini Apps:
// a prompt, in the title after "Prompted", and the reply as HTML.
type geminiActivity struct {
	Header   string   `json:"header"`
	Title    string   `json:"title"`
	Time     string   `json:"time"`
	Products []string `json:"products"`
	Items    []struct {
		HTML string `json:"html"`
	} `json:"safeHtmlItem"`
}

func parseGemini(b []byte) ([]Chat, bool) {
	var acts []geminiActivity
	if json.Unmarshal(b, &acts) != nil {
		return nil, false
	}
	var out []Chat
	for _, a := range acts {
		if !strings.Contains(a.Header, "Gemini") && !containsGemini(a.Products) {
			continue
		}
		prompt, ok := strings.CutPrefix(a.Title, "Prompted ")
		if !ok || strings.TrimSpace(prompt) == "" {
			continue
		}
		chat := Chat{Source: protocol.OniImportGemini, ID: a.Time + "\x00" + prompt, Title: prompt, At: stamp(a.Time)}
		chat.Turns = append(chat.Turns, Turn{User: true, Text: prompt})
		var reply []string
		for _, item := range a.Items {
			if t := htmlText(item.HTML); t != "" {
				reply = append(reply, t)
			}
		}
		if len(reply) > 0 {
			chat.Turns = append(chat.Turns, Turn{Text: strings.Join(reply, "\n")})
		}
		out = append(out, chat)
	}
	return out, len(out) > 0
}

func containsGemini(products []string) bool {
	for _, p := range products {
		if strings.Contains(p, "Gemini") {
			return true
		}
	}
	return false
}

var (
	tags   = regexp.MustCompile(`(?s)<[^>]*>`)
	breaks = regexp.MustCompile(`(?i)<\s*(?:br|/p|/li|/h\d|/div)\s*/?>`)
)

// htmlText is the text of an HTML fragment: line breaks kept, tags and
// entities gone.
func htmlText(s string) string {
	s = breaks.ReplaceAllString(s, "\n")
	s = tags.ReplaceAllString(s, "")
	r := strings.NewReplacer("&amp;", "&", "&lt;", "<", "&gt;", ">", "&quot;", `"`, "&#39;", "'", "&nbsp;", " ")
	return strings.TrimSpace(r.Replace(s))
}

func unix(f float64) time.Time {
	if f <= 0 {
		return time.Time{}
	}
	sec := int64(f)
	return time.Unix(sec, int64((f-float64(sec))*1e9)).UTC()
}

func stamp(s string) time.Time {
	t, err := time.Parse(time.RFC3339Nano, s)
	if err != nil {
		return time.Time{}
	}
	return t.UTC()
}

// SourceName is how a source is named in a chat's header.
func SourceName(source string) string {
	switch source {
	case protocol.OniImportChatGPT:
		return "ChatGPT"
	case protocol.OniImportClaude:
		return "Claude"
	case protocol.OniImportGemini:
		return "Gemini"
	}
	return ""
}

// Header lines of every imported chat: what Recall reads back.
const (
	HeaderSource = "Source: "
	HeaderDate   = "Date: "
)

// Markdown is c as one Markdown file: a title line, the source and date,
// then each turn, every text passed through scrub first. It reports false
// for a chat with no turns.
func Markdown(c Chat, scrub func(string) string) ([]byte, bool) {
	if len(c.Turns) == 0 {
		return nil, false
	}
	var b bytes.Buffer
	title := oneLine(scrub(c.Title), MaxTitle)
	if title == "" {
		title = "Untitled chat"
	}
	fmt.Fprintf(&b, "# %s\n\n%s%s\n", title, HeaderSource, SourceName(c.Source))
	if !c.At.IsZero() {
		fmt.Fprintf(&b, "%s%s\n", HeaderDate, c.At.UTC().Format(time.DateOnly))
	}
	for _, t := range c.Turns {
		who := "Assistant"
		if t.User {
			who = "You"
		}
		text := strings.TrimSpace(clean(scrub(t.Text)))
		if text == "" {
			continue
		}
		entry := "\n**" + who + ":** " + text + "\n"
		if b.Len()+len(entry) > MaxChatBytes {
			b.WriteString("\n(The rest of this chat was cut.)\n")
			break
		}
		b.WriteString(entry)
	}
	return b.Bytes(), true
}

// Head is the title, source and date of an imported chat file's text.
func Head(text string) (title, source string, at time.Time) {
	for i, line := range strings.SplitN(text, "\n", 6) {
		switch {
		case i == 0 && strings.HasPrefix(line, "# "):
			title = strings.TrimPrefix(line, "# ")
		case strings.HasPrefix(line, HeaderSource):
			source = strings.TrimPrefix(line, HeaderSource)
		case strings.HasPrefix(line, HeaderDate):
			at, _ = time.Parse(time.DateOnly, strings.TrimPrefix(line, HeaderDate))
		}
	}
	return title, source, at
}

func oneLine(s string, n int) string {
	s = strings.Join(strings.Fields(clean(s)), " ")
	if utf8.RuneCountInString(s) > n {
		s = string([]rune(s)[:n])
	}
	return s
}

// clean drops control and format characters, keeping line breaks.
func clean(s string) string {
	return strings.Map(func(r rune) rune {
		if r == '\n' || r == '\t' {
			return r
		}
		if unicode.IsControl(r) || unicode.Is(unicode.Cf, r) {
			return -1
		}
		return r
	}, s)
}

// write keeps md as the chat's one file under root/source, 0600 in 0700,
// named for the chat so a second import of it replaces the first.
func write(root string, c Chat, md []byte) error {
	dir := filepath.Join(root, c.Source)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	key := c.ID
	if key == "" {
		key = c.Title + "\x00" + c.At.String()
	}
	sum := sha256.Sum256([]byte(c.Source + "\x00" + key))
	name := filepath.Join(dir, hex.EncodeToString(sum[:12])+".md")
	f, err := os.CreateTemp(dir, ".import-*")
	if err != nil {
		return err
	}
	tmp := f.Name()
	_, err = f.Write(md)
	if err == nil {
		err = f.Chmod(0o600)
	}
	if err == nil {
		err = f.Sync()
	}
	if cerr := f.Close(); err == nil {
		err = cerr
	}
	if err == nil {
		err = os.Rename(tmp, name)
	}
	if err != nil {
		_ = os.Remove(tmp)
	}
	return err
}

// List is how many chats each source gave, by source.
func (r Result) List() []protocol.OniRecallImported {
	var out []protocol.OniRecallImported
	for from, n := range r.Sources {
		out = append(out, protocol.OniRecallImported{From: from, Chats: n})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].From < out[j].From })
	return out
}
