package fs

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"

	i18n "ryoku-i18n"
	"ryoku-rashin/internal/command"
	"ryoku-rashin/internal/engine/exec"
	"ryoku-rashin/internal/engine/tool"
	"ryoku-rashin/internal/llm"
	"ryoku-rashin/internal/protocol"
)

// Derived from oh-my-pi (MIT), packages/coding-agent/src/tools/read.ts@c7fff67

// imageTypes are the extensions read returns as images for vision models.
var imageTypes = map[string]bool{".png": true, ".jpg": true, ".jpeg": true, ".gif": true, ".webp": true, ".bmp": true}

type readTool struct{}

func (readTool) Name() string    { return tool.Read }
func (readTool) Kind() tool.Kind { return protocol.ToolRead }
func (readTool) ReadOnly() bool  { return true }
func (readTool) Parallel() bool  { return true }
func (readTool) Spec(p tool.Profile) llm.Tool {
	if p.Compact() {
		return spec(tool.Read,
			"Read a file or list a folder. Add :20-60 to the path for lines 20 to 60. Output lines start with their line number.",
			`{"type":"object","properties":{"path":{"type":"string","description":"File or folder path, with :A-B for lines."}},"required":["path"]}`)
	}
	return spec(tool.Read,
		"Read a text file, an image or a folder. Text comes back as `N:text` rows under a `[PATH#TAG]` header; copy that header into edit. "+
			"Select lines in the path: `file:50-80`, `file:50` (from 50), `file:50+20`, `file:-30` (last 30), `file:1-20,90-99`, and add `:raw` for the text without numbers. "+
			"A folder lists its entries two levels deep. Long output from other tools is kept as `local://N.log`; read it with the same selectors.",
		`{"type":"object","properties":{"path":{"type":"string","description":"File, folder or local://N.log; line selectors go after a colon."}},"required":["path"]}`)
}

func (readTool) Present(raw json.RawMessage) tool.Presentation {
	a, _ := decode(raw)
	p := a.str(pathKeys...)
	return tool.Presentation{Title: i18n.Tf("Reading %s", filepath.Base(p)), Input: p, Locations: presentLocation(p)}
}

func presentLocation(p string) []protocol.Location {
	if p == "" || strings.HasPrefix(p, spillScheme) {
		return nil
	}
	path, sel, _ := splitSelector(stripHeaderPath(p))
	loc := protocol.Location{Path: path}
	if len(sel.Ranges) > 0 {
		loc.Line = sel.Ranges[0].From
	}
	return []protocol.Location{loc}
}

func (readTool) Run(ctx context.Context, call tool.Call, env tool.Env) tool.Result {
	a, err := decode(call.Args)
	if err != nil {
		return tool.Failed(err.Error())
	}
	p := strings.TrimSpace(stripHeaderPath(a.str(pathKeys...)))
	if p == "" {
		return tool.Failed("path is empty. Give a file or folder path, for example {\"path\":\"README.md\"}.")
	}
	if strings.HasPrefix(p, spillScheme) {
		ref, sel, err := splitSelector(p)
		if err != nil {
			return tool.Failed(err.Error())
		}
		text, err := spilled(env, ref)
		if err != nil {
			return tool.Failed(err.Error())
		}
		out, err := render(env, lines(text), sel, ref, view{Numbers: !sel.Raw})
		if err != nil {
			return tool.Failed(err.Error())
		}
		return tool.Result{Text: out}
	}
	abs, sel, err := locate(ctx, env, p)
	if err != nil {
		return tool.Failed(err.Error())
	}
	if sel.empty() {
		if from, ok := a.integer("offset", "start", "line", "from"); ok {
			r := lineRange{From: max(from, 1)}
			if n, ok := a.integer("limit", "count"); ok && n > 0 {
				r.To = r.From + n - 1
			} else if end, ok := a.integer("end", "to"); ok && end >= r.From {
				r.To = end
			}
			sel.Ranges = []lineRange{r}
		} else if n, ok := a.integer("limit", "count"); ok && n > 0 {
			sel.Ranges = []lineRange{{From: 1, To: n}}
		}
	}
	return readPath(ctx, env, abs, sel, view{Numbers: !sel.Raw, Tags: !env.Profile.Compact() && !sel.Raw})
}

// locate resolves a read path, taking a trailing selector off only when the
// whole string is not itself an existing file.
func locate(ctx context.Context, env tool.Env, p string) (string, selection, error) {
	abs := resolveFor(env, p, env.Cwd)
	if !strings.Contains(filepath.Base(p), ":") {
		return abs, selection{}, nil
	}
	if _, err := env.Exec.Stat(ctx, abs); err == nil {
		return abs, selection{}, nil
	}
	path, sel, err := splitSelector(p)
	if err != nil {
		return "", sel, err
	}
	return resolveFor(env, path, env.Cwd), sel, nil
}

// view says how text rows are printed.
type view struct {
	Numbers bool // `N:text`
	Tags    bool // hashline header above the rows
	Ask     bool // Ask's read_file: plain rows, Ask's wording
}

// readPath reads one resolved path: a folder lists, an image attaches, text
// is rendered under the selection.
func readPath(ctx context.Context, env tool.Env, abs string, sel selection, v view) tool.Result {
	shown := display(abs, env.Cwd)
	if err := secret(abs); err != nil {
		return tool.Failed(err.Error())
	}
	real, info, exists, err := target(ctx, env.Exec, abs)
	if err != nil {
		return tool.Failed(describe(err, shown))
	}
	if !exists {
		return tool.Failed(fmt.Sprintf("%s does not exist. Check the path with ls or find_files.", shown))
	}
	switch info.Kind {
	case exec.KindDir:
		depth := 2
		if env.Profile.Compact() {
			depth = 1
		}
		out, err := listing(ctx, env, real, shown, depth, false)
		if err != nil {
			return tool.Failed(describe(err, shown))
		}
		return tool.Result{Text: Shape(env, out)}
	case exec.KindOther:
		return tool.Failed(fmt.Sprintf("%s is not a regular file (a device, socket or pipe).", shown))
	}
	if imageTypes[strings.ToLower(filepath.Ext(real))] && !sel.Raw {
		return readImage(ctx, env, real, shown, info)
	}
	res, err := env.Exec.Read(ctx, exec.ReadRequest{Path: real, Limit: exec.MaxRead})
	if err != nil {
		return tool.Failed(describe(err, shown))
	}
	if res.Binary {
		return tool.Failed(fmt.Sprintf("%s is a binary file (%s); read shows text only.", shown, size(res.Info.Size)))
	}
	f := splitText(res.Data)
	if res.Truncated && sel.Last > 0 {
		return tool.Failed(fmt.Sprintf("%s is larger than %s; read cannot find its final lines. Narrow with grep or bash.", shown, size(exec.MaxRead)))
	}
	rows := lines(f.Text)
	opts := v
	head := ""
	if v.Tags {
		if res.Truncated {
			opts.Tags = false
		} else {
			tag := fileTag(f.Text)
			state(env.Session.ID).issue(real, tag, f.Text)
			head = header(shown, tag)
		}
	}
	out, err := render(env, rows, sel, shown, opts)
	if err != nil {
		return tool.Failed(err.Error())
	}
	if head != "" {
		out = head + "\n" + out
	}
	if res.Truncated {
		out += fmt.Sprintf("\n[%s is %s; read shows its first %s. Find later lines with grep.]", shown, size(res.Info.Size), size(exec.MaxRead))
	}
	r := tool.Result{Text: out, Locations: []protocol.Location{{Path: real, Line: firstLine(sel)}}}
	return r
}

func firstLine(sel selection) int {
	if len(sel.Ranges) > 0 {
		return sel.Ranges[0].From
	}
	return 0
}

func readImage(ctx context.Context, env tool.Env, abs, shown string, info exec.FileInfo) tool.Result {
	if info.Size > exec.MaxRead {
		return tool.Failed(fmt.Sprintf("%s is %s; read attaches images up to %s.", shown, size(info.Size), size(exec.MaxRead)))
	}
	res, err := env.Exec.Read(ctx, exec.ReadRequest{Path: abs, Limit: exec.MaxRead})
	if err != nil {
		return tool.Failed(describe(err, shown))
	}
	if res.Truncated {
		return tool.Failed(fmt.Sprintf("%s is larger than %s; read attaches smaller images only.", shown, size(exec.MaxRead)))
	}
	mime := http.DetectContentType(res.Data)
	if !strings.HasPrefix(mime, "image/") {
		return tool.Failed(fmt.Sprintf("%s does not hold an image it can attach (%s).", shown, mime))
	}
	return tool.Result{
		Text:      fmt.Sprintf("Image %s (%s, %s) is attached to this result.", shown, mime, size(int64(len(res.Data)))),
		Shown:     i18n.Tf("Image %s", shown),
		Images:    []llm.Image{{Mime: mime, Data: res.Data}},
		Locations: []protocol.Location{{Path: abs}},
	}
}

// render prints rows under a selection within the profile's caps. Without a
// selection it shows the first Limits.ReadLines rows; every cut ends with a
// line naming the selector that continues.
func render(env tool.Env, rows []string, sel selection, shown string, v view) (string, error) {
	lim := env.Limits
	if lim.Told <= 0 {
		lim = env.Profile.Limits()
	}
	total := len(rows)
	if total == 0 {
		if v.Ask {
			return "", nil
		}
		return "(empty file)", nil
	}
	ranges := append([]lineRange(nil), sel.Ranges...)
	if sel.Last > 0 {
		ranges = mergeRanges(append(ranges, lineRange{From: max(total-sel.Last+1, 1), To: total}))
	}
	if len(ranges) == 0 {
		ranges = []lineRange{{From: 1}}
	}
	for i := range ranges {
		if ranges[i].From > total {
			if v.Ask {
				return "", errors.New(i18n.T("The requested line range is outside the file."))
			}
			return "", fmt.Errorf("line %d is past the end of %s, which has %d lines. Use %s:1-%d", ranges[i].From, shown, total, shown, total)
		}
		if ranges[i].To == 0 {
			ranges[i].To = min(total, ranges[i].From+lim.ReadLines-1)
		}
		ranges[i].To = min(ranges[i].To, total)
	}
	budget := lim.Told - 256
	var b strings.Builder
	shownFrom, shownTo := 0, 0
	stopped := 0
	var cut []int
	for ri, r := range ranges {
		if ri > 0 && v.Numbers {
			b.WriteString("…\n")
		}
		for n := r.From; n <= r.To; n++ {
			row := rows[n-1]
			if !sel.Raw {
				var clipped bool
				row, clipped = clipLine(row, lim.LineBytes)
				if clipped {
					cut = append(cut, n)
				}
			}
			line := row
			if v.Numbers {
				line = strconv.Itoa(n) + ":" + row
			}
			if b.Len()+len(line)+1 > budget && b.Len() > 0 {
				stopped = n
				break
			}
			b.WriteString(line)
			b.WriteByte('\n')
			if shownFrom == 0 {
				shownFrom = n
			}
			shownTo = n
		}
		if stopped > 0 {
			break
		}
	}
	out := strings.TrimSuffix(b.String(), "\n")
	lastAsked := ranges[len(ranges)-1].To
	switch {
	case stopped > 0:
		out += fmt.Sprintf("\n[Showing lines %d-%d of %d (size limit). Use %s:%d- to continue]", shownFrom, shownTo, total, shown, stopped)
	case len(sel.Ranges) == 0 && sel.Last == 0 && lastAsked < total:
		out += fmt.Sprintf("\n[Showing lines %d-%d of %d. Use %s:%d- to continue]", shownFrom, shownTo, total, shown, lastAsked+1)
	case len(sel.Ranges) > 0 && sel.Ranges[len(sel.Ranges)-1].To == 0 && lastAsked < total:
		out += fmt.Sprintf("\n[Showing lines %d-%d of %d. Use %s:%d- to continue]", shownFrom, shownTo, total, shown, lastAsked+1)
	}
	if len(cut) > 0 && !v.Ask {
		out += fmt.Sprintf("\n[Some lines cut at %d bytes. Read %s:%d:raw for line %d whole]", lim.LineBytes, shown, cut[0], cut[0])
	}
	return out, nil
}

// size is a byte count for a sentence.
func size(n int64) string {
	switch {
	case n >= 1<<20:
		return fmt.Sprintf("%.1f MiB", float64(n)/(1<<20))
	case n >= 1<<10:
		return fmt.Sprintf("%.1f KiB", float64(n)/(1<<10))
	}
	return fmt.Sprintf("%d bytes", n)
}

// readFileTool is Ask's read_file, kept under its name and schema.
type readFileTool struct{}

func (readFileTool) Name() string    { return tool.ReadFile }
func (readFileTool) Kind() tool.Kind { return protocol.ToolRead }
func (readFileTool) ReadOnly() bool  { return true }
func (readFileTool) Parallel() bool  { return true }
func (readFileTool) Spec(tool.Profile) llm.Tool {
	return spec(tool.ReadFile, "Read a bounded range from a non-secret local text file.",
		`{"type":"object","properties":{"path":{"type":"string"},"start":{"type":"integer","minimum":1},"end":{"type":"integer","minimum":1}},"required":["path"],"additionalProperties":false}`)
}

func (readFileTool) Present(raw json.RawMessage) tool.Presentation {
	a, _ := decode(raw)
	p := a.str(pathKeys...)
	return tool.Presentation{Title: i18n.Tf("Reading %s", p), Input: p, Locations: presentLocation(p)}
}

func (readFileTool) Run(ctx context.Context, call tool.Call, env tool.Env) tool.Result {
	a, err := decode(call.Args)
	if err != nil {
		return tool.Failed(err.Error())
	}
	p := strings.TrimSpace(a.str(pathKeys...))
	if p == "" {
		return tool.Failed(i18n.T("The file path is empty."))
	}
	abs := resolveFor(env, p, env.Cwd)
	if marker, blocked := command.SecretPathAt(abs, env.Cwd); blocked {
		return tool.Failed(i18n.Tf("Refused to read %s because it contains %s.", abs, marker))
	}
	var sel selection
	start, _ := a.integer("start")
	end, _ := a.integer("end")
	if start > 0 || end > 0 {
		r := lineRange{From: max(start, 1), To: end}
		if r.To > 0 && r.To < r.From {
			return tool.Failed(i18n.T("The requested line range is outside the file."))
		}
		sel.Ranges = []lineRange{r}
	}
	real, info, exists, err := target(ctx, env.Exec, abs)
	if err != nil {
		return tool.Failed(describe(err, abs))
	}
	if !exists {
		return tool.Failed(fmt.Sprintf("%s does not exist.", abs))
	}
	if info.Kind != exec.KindFile {
		return tool.Failed(i18n.T("Refused to read a non-regular file."))
	}
	return readPath(ctx, env, real, sel, view{Ask: true})
}
