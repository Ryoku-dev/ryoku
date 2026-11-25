package exec

import (
	"bufio"
	"bytes"
	"context"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"time"
	"unicode/utf8"
)

const (
	// grepFileMax skips files too large to be source a model should read.
	grepFileMax = 16 << 20
	// grepLineMax caps one returned line; tools clip further to their limits.
	grepLineMax = 4096
	// grepContextMax caps GrepRequest.Context.
	grepContextMax = 20
	// braceMax caps the patterns one `{a,b}` expansion produces.
	braceMax = 64
)

func (h *host) Glob(ctx context.Context, req GlobRequest) (GlobResult, error) {
	root, err := walkRoot(req.Root)
	if err != nil {
		return GlobResult{}, err
	}
	pattern, err := compileGlob(req.Pattern)
	if err != nil {
		return GlobResult{}, err
	}
	limit := req.Limit
	if limit <= 0 {
		limit = 1000
	}
	type hit struct {
		path string
		mod  time.Time
	}
	var hits []hit
	err = walk(ctx, root, req.Hidden, func(rel string, entry fs.DirEntry) {
		if entry.IsDir() && !req.Dirs {
			return
		}
		if !pattern.match(rel) {
			return
		}
		found := hit{path: filepath.Join(root.shown, filepath.FromSlash(rel))}
		if fi, err := entry.Info(); err == nil {
			found.mod = fi.ModTime()
		}
		hits = append(hits, found)
	})
	if err != nil {
		return GlobResult{}, err
	}
	slices.SortFunc(hits, func(a, b hit) int {
		if c := b.mod.Compare(a.mod); c != 0 {
			return c
		}
		return strings.Compare(a.path, b.path)
	})
	result := GlobResult{Total: len(hits), Paths: make([]string, 0, min(len(hits), limit))}
	for i, h := range hits {
		if i == limit {
			result.Truncated = true
			break
		}
		result.Paths = append(result.Paths, h.path)
	}
	return result, nil
}

func (h *host) Grep(ctx context.Context, req GrepRequest) (GrepResult, error) {
	p, err := absPath(req.Root)
	if err != nil {
		return GrepResult{}, err
	}
	expr := req.Pattern
	if req.Literal {
		expr = regexp.QuoteMeta(expr)
	}
	if req.IgnoreCase {
		expr = "(?i)" + expr
	}
	re, err := regexp.Compile(expr)
	if err != nil {
		return GrepResult{}, &Error{Code: CodeBadPattern, Message: err.Error()}
	}
	var include *globPattern
	if req.Include != "" {
		if include, err = compileGlob(req.Include); err != nil {
			return GrepResult{}, err
		}
	}
	g := grepper{
		re:      re,
		files:   req.MaxFiles,
		perFile: req.MaxPerFile,
		context: min(max(req.Context, 0), grepContextMax),
	}
	if g.files <= 0 {
		g.files = 100
	}
	if g.perFile <= 0 {
		g.perFile = 20
	}
	fi, err := os.Stat(p)
	if err != nil {
		return GrepResult{}, fsError(p, err)
	}
	if !fi.IsDir() {
		if err := regular(p, fi); err != nil {
			return GrepResult{}, err
		}
		g.search(p)
		return g.result, nil
	}
	root, err := walkRoot(p)
	if err != nil {
		return GrepResult{}, err
	}
	err = walk(ctx, root, req.Hidden, func(rel string, entry fs.DirEntry) {
		if !entry.Type().IsRegular() || (include != nil && !include.match(rel)) {
			return
		}
		g.search(filepath.Join(root.shown, filepath.FromSlash(rel)))
	})
	if err != nil {
		return GrepResult{}, err
	}
	return g.result, nil
}

type grepper struct {
	re                      *regexp.Regexp
	files, perFile, context int
	result                  GrepResult
}

func (g *grepper) search(p string) {
	if fi, err := os.Stat(p); err != nil || fi.Size() > grepFileMax {
		return
	}
	data, err := os.ReadFile(p)
	if err != nil || bytes.IndexByte(data, 0) >= 0 || !utf8.Valid(data) || !g.re.Match(data) {
		return
	}
	lines := strings.Split(string(data), "\n")
	if n := len(lines); n > 0 && lines[n-1] == "" {
		lines = lines[:n-1]
	}
	for i := range lines {
		lines[i] = strings.TrimSuffix(lines[i], "\r")
	}
	if g.result.Files >= g.files {
		// counted, not returned
		if slices.ContainsFunc(lines, g.re.MatchString) {
			g.result.Files++
			g.result.Truncated = true
		}
		return
	}
	found := 0
	for i, line := range lines {
		if !g.re.MatchString(line) {
			continue
		}
		if found == g.perFile {
			g.result.Truncated = true
			break
		}
		found++
		m := Match{Path: p, Line: i + 1, Text: clipLine(line)}
		if g.context > 0 {
			for _, before := range lines[max(0, i-g.context):i] {
				m.Before = append(m.Before, clipLine(before))
			}
			for _, after := range lines[i+1 : min(len(lines), i+1+g.context)] {
				m.After = append(m.After, clipLine(after))
			}
		}
		g.result.Matches = append(g.result.Matches, m)
	}
	if found > 0 {
		g.result.Files++
	}
}

func clipLine(s string) string {
	if len(s) <= grepLineMax {
		return s
	}
	cut := grepLineMax
	for cut > 0 && !utf8RuneStart(s[cut]) {
		cut--
	}
	return s[:cut]
}

func utf8RuneStart(b byte) bool { return b&0xC0 != 0x80 }

// tree is a walk's root: shown is the path results report, real the
// directory walked (a root reached through a symlink is still searched).
type tree struct{ shown, real string }

func walkRoot(root string) (tree, error) {
	p, err := absPath(root)
	if err != nil {
		return tree{}, err
	}
	real, err := filepath.EvalSymlinks(p)
	if err != nil {
		return tree{}, fsError(p, err)
	}
	fi, err := os.Stat(real)
	if err != nil {
		return tree{}, fsError(p, err)
	}
	if !fi.IsDir() {
		return tree{}, &Error{Code: CodeNotDir, Path: p}
	}
	return tree{shown: p, real: real}, nil
}

// walk visits every entry under root in lexical order with its slash path
// relative to root. It skips SkipDirs, dot entries unless hidden, and what
// git ignores when root is inside a work tree; it never follows a symlinked
// directory. Unreadable directories below root are skipped.
func walk(ctx context.Context, root tree, hidden bool, visit func(rel string, entry fs.DirEntry)) error {
	var descend func(dir, rel string, ig *ignorer) error
	descend = func(dir, rel string, ig *ignorer) error {
		if err := ctx.Err(); err != nil {
			return err
		}
		entries, err := os.ReadDir(dir)
		if err != nil {
			if rel == "" {
				return fsError(root.shown, err)
			}
			return nil
		}
		ig = ig.with(dir)
		for _, entry := range entries {
			name := entry.Name()
			if !hidden && strings.HasPrefix(name, ".") {
				continue
			}
			isDir := entry.IsDir()
			if isDir && slices.Contains(SkipDirs, name) {
				continue
			}
			abs := filepath.Join(dir, name)
			if ig.ignored(abs, isDir) {
				continue
			}
			r := name
			if rel != "" {
				r = rel + "/" + name
			}
			visit(r, entry)
			if isDir {
				if err := descend(abs, r, ig); err != nil {
					return err
				}
			}
		}
		return nil
	}
	return descend(root.real, "", gitIgnorer(root.real))
}

// globPattern is a compiled GlobRequest pattern: each brace alternative
// split into path elements, `**` spanning any number of them.
type globPattern struct{ alts [][]string }

func compileGlob(pattern string) (*globPattern, error) {
	pattern = strings.TrimPrefix(filepath.ToSlash(strings.TrimSpace(pattern)), "./")
	if pattern == "" {
		return nil, &Error{Code: CodeBadPattern, Message: "empty pattern"}
	}
	g := &globPattern{}
	for _, alt := range expandBraces(pattern) {
		anchored := strings.Contains(strings.TrimSuffix(alt, "/"), "/")
		segs, err := globSegments(alt)
		if err != nil {
			return nil, &Error{Code: CodeBadPattern, Message: "bad pattern " + pattern + ": " + err.Error()}
		}
		if len(segs) == 0 {
			continue
		}
		if !anchored {
			segs = append([]string{"**"}, segs...)
		}
		g.alts = append(g.alts, segs)
	}
	if len(g.alts) == 0 {
		return nil, &Error{Code: CodeBadPattern, Message: "empty pattern"}
	}
	return g, nil
}

func globSegments(pattern string) ([]string, error) {
	var segs []string
	for _, seg := range strings.Split(pattern, "/") {
		if seg == "" || seg == "." {
			continue
		}
		if _, err := path.Match(seg, ""); err != nil {
			return nil, err
		}
		segs = append(segs, seg)
	}
	return segs, nil
}

func (g *globPattern) match(rel string) bool {
	parts := strings.Split(rel, "/")
	for _, segs := range g.alts {
		if matchSegments(segs, parts) {
			return true
		}
	}
	return false
}

func matchSegments(pat, parts []string) bool {
	for len(pat) > 0 {
		if pat[0] == "**" {
			for len(pat) > 0 && pat[0] == "**" {
				pat = pat[1:]
			}
			if len(pat) == 0 {
				return true
			}
			for i := range len(parts) + 1 {
				if matchSegments(pat, parts[i:]) {
					return true
				}
			}
			return false
		}
		if len(parts) == 0 {
			return false
		}
		if ok, _ := path.Match(pat[0], parts[0]); !ok {
			return false
		}
		pat, parts = pat[1:], parts[1:]
	}
	return len(parts) == 0
}

// expandBraces turns `a.{go,md}` into `a.go` and `a.md`. A brace with no
// comma stays literal.
func expandBraces(p string) []string {
	open, depth := -1, 0
	for i := 0; i < len(p); i++ {
		switch p[i] {
		case '\\':
			i++
		case '{':
			if depth == 0 {
				open = i
			}
			depth++
		case '}':
			if depth == 0 {
				continue
			}
			depth--
			if depth != 0 {
				continue
			}
			parts := splitTopComma(p[open+1 : i])
			if len(parts) < 2 {
				open = -1
				continue
			}
			var out []string
			for _, part := range parts {
				for _, alt := range expandBraces(p[:open] + part + p[i+1:]) {
					if len(out) == braceMax {
						return out
					}
					out = append(out, alt)
				}
			}
			return out
		}
	}
	return []string{p}
}

func splitTopComma(s string) []string {
	var parts []string
	depth, from := 0, 0
	for i := 0; i < len(s); i++ {
		switch s[i] {
		case '\\':
			i++
		case '{':
			depth++
		case '}':
			depth--
		case ',':
			if depth == 0 {
				parts = append(parts, s[from:i])
				from = i + 1
			}
		}
	}
	return append(parts, s[from:])
}

// ignorer holds the gitignore rules in force for one directory, outermost
// first, so the last rule that matches decides. Nil ignores nothing.
type ignorer struct{ rules []ignoreRule }

type ignoreRule struct {
	prefix          string // the .gitignore's directory with a trailing slash
	segs            []string
	negate, dirOnly bool
}

// gitIgnorer starts a walk at root: nil outside a git work tree, else the
// work tree's info/exclude and every .gitignore from its top down to root's
// parent (root's own is read as the walk enters it).
func gitIgnorer(root string) *ignorer {
	top := ""
	for dir := root; ; dir = filepath.Dir(dir) {
		if _, err := os.Lstat(filepath.Join(dir, ".git")); err == nil {
			top = dir
			break
		}
		if filepath.Dir(dir) == dir {
			return nil
		}
	}
	ig := &ignorer{}
	ig.rules = append(ig.rules, readIgnore(filepath.Join(top, ".git", "info", "exclude"), top)...)
	if top != root {
		rel, err := filepath.Rel(top, filepath.Dir(root))
		if err != nil {
			return ig
		}
		dir := top
		ig.rules = append(ig.rules, readIgnore(filepath.Join(dir, ".gitignore"), dir)...)
		if rel != "." {
			for _, part := range strings.Split(rel, string(filepath.Separator)) {
				dir = filepath.Join(dir, part)
				ig.rules = append(ig.rules, readIgnore(filepath.Join(dir, ".gitignore"), dir)...)
			}
		}
	}
	return ig
}

// with adds dir's own .gitignore; the parent's rules are left untouched for
// its other children.
func (ig *ignorer) with(dir string) *ignorer {
	if ig == nil {
		return nil
	}
	rules := readIgnore(filepath.Join(dir, ".gitignore"), dir)
	if len(rules) == 0 {
		return ig
	}
	return &ignorer{rules: append(slices.Clip(ig.rules), rules...)}
}

func (ig *ignorer) ignored(abs string, isDir bool) bool {
	if ig == nil {
		return false
	}
	ignored := false
	for _, r := range ig.rules {
		if r.dirOnly && !isDir {
			continue
		}
		rel, ok := strings.CutPrefix(abs, r.prefix)
		if !ok || rel == "" {
			continue
		}
		if matchSegments(r.segs, strings.Split(rel, "/")) {
			ignored = !r.negate
		}
	}
	return ignored
}

func readIgnore(file, dir string) []ignoreRule {
	data, err := os.ReadFile(file)
	if err != nil {
		return nil
	}
	var rules []ignoreRule
	scanner := bufio.NewScanner(bytes.NewReader(data))
	for scanner.Scan() {
		line := strings.TrimSuffix(scanner.Text(), "\r")
		if line == "" || line[0] == '#' {
			continue
		}
		if !strings.HasSuffix(line, `\ `) {
			line = strings.TrimRight(line, " ")
		}
		if line == "" {
			continue
		}
		r := ignoreRule{prefix: strings.TrimSuffix(dir, "/") + "/"}
		if line[0] == '!' {
			r.negate, line = true, line[1:]
		} else if strings.HasPrefix(line, `\!`) || strings.HasPrefix(line, `\#`) {
			line = line[1:]
		}
		if strings.HasSuffix(line, "/") {
			r.dirOnly, line = true, strings.TrimRight(line, "/")
		}
		anchored := strings.Contains(line, "/")
		segs, err := globSegments(line)
		if err != nil || len(segs) == 0 {
			continue
		}
		if !anchored {
			segs = append([]string{"**"}, segs...)
		}
		r.segs = segs
		rules = append(rules, r)
	}
	return rules
}
