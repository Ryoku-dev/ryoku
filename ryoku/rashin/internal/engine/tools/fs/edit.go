package fs

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"

	i18n "ryoku-i18n"
	"ryoku-rashin/internal/engine/exec"
	"ryoku-rashin/internal/engine/tool"
	"ryoku-rashin/internal/llm"
	"ryoku-rashin/internal/protocol"
)

// Derived from oh-my-pi (MIT), crates/edit/src/modes/hashline/mod.rs and crates/edit/src/modes/replace.rs@c7fff67

type editTool struct{}

func (editTool) Name() string    { return tool.Edit }
func (editTool) Kind() tool.Kind { return protocol.ToolEdit }
func (editTool) ReadOnly() bool  { return false }
func (editTool) Spec(p tool.Profile) llm.Tool {
	if p.Compact() {
		return spec(tool.Edit, "Replace text in one file. Read it first; old must identify one place unless all is true.",
			`{"type":"object","properties":{"path":{"type":"string"},"old":{"type":"string"},"new":{"type":"string"},"all":{"type":"boolean"}},"required":["path","old","new"]}`)
	}
	return spec(tool.Edit,
		"Edit files by anchored patch. Read each file for its [PATH#TAG], then send *** Begin Patch, a [PATH#TAG] header and PUT/CUT/REM/MV ops, and *** End Patch. Line numbers refer to the original tagged file. Copy the new tag from the reply for another edit. Use explicit ranges for blocks.",
		`{"type":"object","properties":{"input":{"type":"string","description":"Hashline patch with [PATH#TAG] section headers and PUT/CUT/REM/MV operations."}},"required":["input"]}`)
}
func (editTool) Present(raw json.RawMessage) tool.Presentation {
	a, _ := decode(raw)
	if path := a.str(pathKeys...); path != "" {
		return tool.Presentation{Title: i18n.Tf("Editing %s", filepath.Base(path)), Input: path, Locations: []protocol.Location{{Path: path}}}
	}
	return tool.Presentation{Title: i18n.T("Editing files"), Input: tool.HeadTail(a.str("input"), 1200, 300)}
}
func (editTool) Run(ctx context.Context, call tool.Call, env tool.Env) tool.Result {
	a, err := decode(call.Args)
	if err != nil {
		return tool.Failed(err.Error())
	}
	if env.Profile.Compact() {
		return compactEdit(ctx, call, env, a)
	}
	return anchoredEdit(ctx, call, env, a.str("input", "patch"))
}

func compactEdit(ctx context.Context, call tool.Call, env tool.Env, a args) tool.Result {
	p := strings.TrimSpace(stripHeaderPath(a.str(pathKeys...)))
	if p == "" || !a.has("old", "old_string") || !a.has("new", "new_string") {
		return tool.Failed("edit needs path, old and new. Give the original text in old and its replacement in new (empty new deletes it).")
	}
	abs := resolveFor(env, p, env.Cwd)
	shown := display(abs, env.Cwd)
	file, err := readEditable(ctx, env, abs)
	if err != nil {
		return tool.Failed(describe(err, shown))
	}
	changed, err := findReplace(file.Text.Text, a.str("old", "old_string"), a.str("new", "new_string"), a.boolean("all", "replace_all"), shown)
	if err != nil {
		return tool.Failed(err.Error())
	}
	if changed.Text == file.Text.Text {
		return tool.Failed("the replacement leaves the file unchanged. Check old and new, then retry.")
	}
	diff := makeDiff(file.Path, file.Text.Text, changed.Text)
	if result, ok := allow(ctx, env, call, i18n.Tf("Editing %s", filepath.Base(abs)), abs, diff); !ok {
		return result
	}
	kept, err := backup(env, file.Path, file.Text.Raw)
	if err != nil {
		return tool.Failed(i18n.Tf("Could not keep a copy before changing %s: %v", shown, err))
	}
	if _, err = env.Exec.Write(ctx, exec.WriteRequest{Path: file.Path, Content: file.Text.join(changed.Text), IfVersion: file.Info.Version}); err != nil {
		return tool.Failed(describe(err, shown))
	}
	message := fmt.Sprintf("Edited %s at line %d (%d %s).", shown, changed.Line, changed.Count, plural(changed.Count, "place", "places"))
	if changed.How != "" {
		message += " Matched " + changed.How + "."
	}
	if kept != "" {
		message += " Previous content: " + kept
	}
	return tool.Result{Text: message, Shown: i18n.Tf("Edited %s.", shown), Diff: diff, Locations: []protocol.Location{{Path: file.Path, Line: changed.Line}}}
}

type editable struct {
	Path string
	Info exec.FileInfo
	Text textFile
}

func readEditable(ctx context.Context, env tool.Env, abs string) (editable, error) {
	if err := secret(abs); err != nil {
		return editable{}, err
	}
	real, info, exists, err := target(ctx, env.Exec, abs)
	if err != nil {
		return editable{}, err
	}
	if !exists {
		return editable{}, fmt.Errorf("%s does not exist. Use write to create it", abs)
	}
	if info.Kind != exec.KindFile {
		return editable{}, fmt.Errorf("%s is not a regular file", abs)
	}
	res, err := env.Exec.Read(ctx, exec.ReadRequest{Path: real, Limit: exec.MaxRead})
	if err != nil {
		return editable{}, err
	}
	if res.Truncated || res.Info.Size > exec.MaxRead {
		return editable{}, fmt.Errorf("%s is larger than %s. Narrow the change or use bash", abs, size(exec.MaxRead))
	}
	if res.Binary || !isText(res.Data) {
		return editable{}, fmt.Errorf("%s is binary, and edit only changes text", abs)
	}
	return editable{Path: real, Info: res.Info, Text: splitText(res.Data)}, nil
}
