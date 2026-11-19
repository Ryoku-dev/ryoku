package fs

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"regexp"
	"strings"

	i18n "ryoku-i18n"
	"ryoku-rashin/internal/engine/exec"
	"ryoku-rashin/internal/engine/tool"
	"ryoku-rashin/internal/llm"
	"ryoku-rashin/internal/protocol"
)

// Derived from oh-my-pi (MIT), packages/coding-agent/src/tools/write.ts@c7fff67

const (
	// Ask writes configuration and notes, not data files.
	askWriteLimit   = 256 * 1024
	askReplaceLimit = 512 * 1024
)

// pagingNote matches the note read ends a cut view with: content ending in
// it is a partial view, and writing it back would drop the rest.
var pagingNote = regexp.MustCompile(`\[Showing lines \d+-\d+ of \d+[^\]]*\]\s*$`)

type writeTool struct{}

func (writeTool) Name() string    { return tool.Write }
func (writeTool) Kind() tool.Kind { return protocol.ToolEdit }
func (writeTool) ReadOnly() bool  { return false }
func (writeTool) Spec(p tool.Profile) llm.Tool {
	if p.Compact() {
		return spec(tool.Write, "Create a new file or replace a whole file with content.",
			`{"type":"object","properties":{"path":{"type":"string"},"content":{"type":"string","description":"The complete file text."}},"required":["path","content"]}`)
	}
	return spec(tool.Write,
		"Create a file, or replace a whole file, with content. Missing folders are created. Change part of an existing file with edit instead. The reply carries the file's new [PATH#TAG] header.",
		`{"type":"object","properties":{"path":{"type":"string"},"content":{"type":"string","description":"The complete file text."}},"required":["path","content"]}`)
}

func (writeTool) Present(raw json.RawMessage) tool.Presentation {
	a, _ := decode(raw)
	p := stripHeaderPath(a.str(pathKeys...))
	return tool.Presentation{Title: i18n.Tf("Writing %s", filepath.Base(p)), Input: p, Locations: []protocol.Location{{Path: p}}}
}

func (w writeTool) Run(ctx context.Context, call tool.Call, env tool.Env) tool.Result {
	a, err := decode(call.Args)
	if err != nil {
		return tool.Failed(err.Error())
	}
	p := strings.TrimSpace(stripHeaderPath(a.str(pathKeys...)))
	if p == "" {
		return tool.Failed("path is empty. Give the file to write, for example {\"path\":\"notes.md\",\"content\":\"...\"}.")
	}
	if !a.has("content", "text", "contents") {
		return tool.Failed("content is missing. Send the complete file text in content.")
	}
	abs := resolveFor(env, p, env.Cwd)
	return writeFile(ctx, call, env, abs, a.str("content", "text", "contents"), writeOptions{
		Title: w.Present(call.Args).Title,
		Tags:  !env.Profile.Compact(),
	})
}

type writeOptions struct {
	Title string
	Tags  bool
	Ask   bool // Ask's write_file: its limits, its wording, no folder creation, private new files
}

// writeFile is write and write_file: checks, the approval with the diff, a
// backup of what was there, then one atomic write.
func writeFile(ctx context.Context, call tool.Call, env tool.Env, abs, content string, o writeOptions) tool.Result {
	shown := display(abs, env.Cwd)
	refuse := func(model, user string) tool.Result {
		r := tool.Failed(model)
		if user != "" {
			r.Shown = user
		}
		return r
	}
	if err := secret(abs); err != nil {
		return refuse(err.Error(), "")
	}
	note := ""
	if stripped, ok := stripRowNumbers(content); ok {
		content = stripped
		note = " The N: line prefixes copied from read were removed."
	}
	if o.Ask && len(content) > askWriteLimit {
		return refuse(i18n.T("The new content is over 256 KiB. Ask writes configuration and notes, not data files."), "")
	}
	if len(content) > exec.MaxRead {
		return refuse(fmt.Sprintf("content is %s; write takes at most %s. Write a smaller file or generate it with bash.", size(int64(len(content))), size(exec.MaxRead)), "")
	}
	if !isText([]byte(content)) {
		return refuse(i18n.T("The new content is not text."), "")
	}
	if pagingNote.MatchString(content) {
		return refuse(fmt.Sprintf("content ends with read's paging note, so it is a partial view of %s. Read the rest and write the complete file, or use edit.", shown), "")
	}
	real, info, existed, err := target(ctx, env.Exec, abs)
	if err != nil {
		return refuse(describe(err, shown), "")
	}
	if err := secret(real); err != nil {
		return refuse(err.Error(), "")
	}
	var old textFile
	if existed {
		if info.Kind != exec.KindFile {
			return refuse(i18n.T("Refused to write over something that is not a regular file."), "")
		}
		if o.Ask && info.Size > askReplaceLimit {
			return refuse(i18n.T("The file is over 512 KiB, too large to replace whole. Change it with a command instead."), "")
		}
		res, err := env.Exec.Read(ctx, exec.ReadRequest{Path: real, Limit: exec.MaxRead})
		if err != nil {
			return refuse(describe(err, shown), "")
		}
		if res.Truncated {
			return refuse(fmt.Sprintf("%s is %s, too large to replace whole. Change it with edit or bash.", shown, size(res.Info.Size)), "")
		}
		if res.Binary {
			if o.Ask {
				return refuse(i18n.T("The file is not text, so Ask will not replace it."), "")
			}
			return refuse(fmt.Sprintf("%s is a binary file; write replaces text files only.", shown), "")
		}
		info = res.Info
		old = splitText(res.Data)
	} else if o.Ask {
		parent, err := env.Exec.Stat(ctx, filepath.Dir(real))
		if err != nil || parent.Kind != exec.KindDir {
			return refuse(i18n.Tf("The folder %s does not exist. Create it first with run_command.", filepath.Dir(real)), "")
		}
	}
	newText := strings.ReplaceAll(content, "\r\n", "\n")
	data := []byte(content)
	if existed {
		// keep the file's own line ends and byte order mark
		data = old.join(newText)
		if old.Text == newText {
			msg := i18n.T("The file already says exactly this. Nothing was written.")
			r := tool.Result{Text: msg}
			if o.Tags {
				tag := fileTag(newText)
				state(env.Session.ID).issue(real, tag, newText)
				r.Text = "The file already says exactly this. Nothing was written.\n" + header(shown, tag)
				r.Shown = msg
			}
			return r
		}
	}
	diff := makeDiff(real, old.Text, newText)
	if r, ok := allow(ctx, env, call, o.Title, targetName(abs, real, env.Cwd), diff); !ok {
		return r
	}
	kept := ""
	if existed {
		kept, err = backup(env, real, old.Raw)
		if err != nil {
			msg := i18n.Tf("Could not keep a copy of the file before changing it, so it was left alone: %v", err)
			return refuse(msg, "")
		}
	}
	req := exec.WriteRequest{Path: real, Content: data, MakeDirs: !o.Ask}
	if existed {
		req.IfVersion = info.Version
	} else {
		req.IfAbsent = true
		if !o.Ask {
			req.Mode = 0o644
			if strings.HasPrefix(content, "#!") && !strings.HasPrefix(content, "#![") {
				req.Mode = 0o755
			}
		}
	}
	if _, err := env.Exec.Write(ctx, req); err != nil {
		return refuse(describe(err, shown), "")
	}
	name := targetName(abs, real, env.Cwd)
	r := tool.Result{Diff: diff, Locations: []protocol.Location{{Path: real}}}
	if o.Ask {
		switch {
		case !existed:
			r.Text = i18n.Tf("Created %s.", name)
		case kept != "":
			r.Text = i18n.Tf("Wrote %s. The version before this change is kept at %s.", name, kept)
		default:
			r.Text = i18n.Tf("Wrote %s.", name)
		}
		return r
	}
	n := countLines(newText)
	verb, userVerb := "Wrote", i18n.Tf("Wrote %s.", name)
	if !existed {
		verb, userVerb = "Created", i18n.Tf("Created %s.", name)
	}
	r.Text = fmt.Sprintf("%s %s (%d %s).%s", verb, name, n, plural(n, "line", "lines"), note)
	if o.Tags {
		tag := fileTag(newText)
		state(env.Session.ID).issue(real, tag, newText)
		r.Text += "\n" + header(shown, tag)
	}
	r.Shown = userVerb
	if kept != "" {
		r.Shown = i18n.Tf("Wrote %s. The version before this change is kept at %s.", name, kept)
	}
	return r
}

// writeFileTool is Ask's write_file, kept under its name and schema.
type writeFileTool struct{}

func (writeFileTool) Name() string    { return tool.WriteFile }
func (writeFileTool) Kind() tool.Kind { return protocol.ToolEdit }
func (writeFileTool) ReadOnly() bool  { return false }
func (writeFileTool) Spec(tool.Profile) llm.Tool {
	return spec(tool.WriteFile, "Create or replace one whole text file the user owns. Read the file first, then send its complete new content.",
		`{"type":"object","properties":{"path":{"type":"string"},"content":{"type":"string"},"why":{"type":"string","description":"One short line the user reads before allowing it."}},"required":["path","content","why"],"additionalProperties":false}`)
}

func (writeFileTool) Present(raw json.RawMessage) tool.Presentation {
	a, _ := decode(raw)
	title := i18n.T("Writing a file")
	if why := strings.TrimSpace(a.str("why")); why != "" {
		title, _ = tool.ClipRunes(why, 80)
	}
	p := a.str(pathKeys...)
	return tool.Presentation{Title: title, Input: p, Locations: []protocol.Location{{Path: p}}}
}

func (w writeFileTool) Run(ctx context.Context, call tool.Call, env tool.Env) tool.Result {
	a, err := decode(call.Args)
	if err != nil {
		return tool.Failed(err.Error())
	}
	p := strings.TrimSpace(a.str(pathKeys...))
	if p == "" {
		return tool.Failed(i18n.T("The file path is empty."))
	}
	return writeFile(ctx, call, env, resolveFor(env, p, env.Cwd), a.str("content"), writeOptions{Title: w.Present(call.Args).Title, Ask: true})
}
