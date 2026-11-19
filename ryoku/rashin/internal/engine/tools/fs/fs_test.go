package fs

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	"ryoku-rashin/internal/engine/exec"
	"ryoku-rashin/internal/engine/tool"
	"ryoku-rashin/internal/protocol"
)

type fakeFS struct {
	exec.Executor
	files    map[string][]byte
	links    map[string]string
	dirs     map[string]bool
	versions map[string]int
	grep     exec.GrepResult
	greps    []exec.GrepRequest
	reads    int
	writes   int
}

func newFakeFS(files map[string]string) *fakeFS {
	f := &fakeFS{files: map[string][]byte{}, links: map[string]string{}, dirs: map[string]bool{}, versions: map[string]int{}}
	for p, data := range files {
		f.files[p] = []byte(data)
		f.versions[p] = 1
	}
	return f
}
func (f *fakeFS) Stat(_ context.Context, p string) (exec.FileInfo, error) {
	if p == "/" || p == "/work" || f.dirs[p] {
		return exec.FileInfo{Path: p, Kind: exec.KindDir}, nil
	}
	if link, ok := f.links[p]; ok {
		return exec.FileInfo{Path: p, Kind: exec.KindSymlink, Target: link, Version: fmt.Sprint(f.versions[p])}, nil
	}
	if b, ok := f.files[p]; ok {
		return exec.FileInfo{Path: p, Name: filepath.Base(p), Kind: exec.KindFile, Size: int64(len(b)), Version: fmt.Sprint(f.versions[p])}, nil
	}
	return exec.FileInfo{}, exec.ErrNotFound
}
func (f *fakeFS) Read(ctx context.Context, req exec.ReadRequest) (exec.ReadResult, error) {
	f.reads++
	info, err := f.Stat(ctx, req.Path)
	if err != nil {
		return exec.ReadResult{}, err
	}
	data := f.files[req.Path]
	if req.Limit > 0 && int64(len(data)) > req.Limit {
		return exec.ReadResult{Data: data[:req.Limit], Info: info, Truncated: true}, nil
	}
	return exec.ReadResult{Data: data, Info: info}, nil
}
func (f *fakeFS) Write(ctx context.Context, req exec.WriteRequest) (exec.WriteResult, error) {
	before, err := f.Stat(ctx, req.Path)
	exists := err == nil
	if req.IfAbsent && exists {
		return exec.WriteResult{}, exec.ErrExists
	}
	if req.IfVersion != "" && (!exists || before.Version != req.IfVersion) {
		return exec.WriteResult{}, exec.ErrChanged
	}
	old := append([]byte(nil), f.files[req.Path]...)
	f.files[req.Path] = append([]byte(nil), req.Content...)
	f.versions[req.Path]++
	f.writes++
	info, _ := f.Stat(ctx, req.Path)
	return exec.WriteResult{Existed: exists, Old: old, Info: info}, nil
}
func (f *fakeFS) Remove(ctx context.Context, req exec.RemoveRequest) error {
	info, err := f.Stat(ctx, req.Path)
	if err != nil {
		return err
	}
	if req.IfVersion != "" && req.IfVersion != info.Version {
		return exec.ErrChanged
	}
	delete(f.files, req.Path)
	delete(f.links, req.Path)
	return nil
}
func (f *fakeFS) List(_ context.Context, req exec.ListRequest) (exec.ListResult, error) {
	var entries []exec.FileInfo
	for path, data := range f.files {
		if filepath.Dir(path) == req.Path {
			entries = append(entries, exec.FileInfo{Path: path, Name: filepath.Base(path), Kind: exec.KindFile, Size: int64(len(data))})
		}
	}
	return exec.ListResult{Entries: entries, Total: len(entries)}, nil
}

func (f *fakeFS) Grep(_ context.Context, req exec.GrepRequest) (exec.GrepResult, error) {
	f.greps = append(f.greps, req)
	return f.grep, nil
}

type fakePermissions struct{}

func (fakePermissions) Allow(context.Context, tool.Request) (tool.Decision, error) {
	return tool.Decision{Allowed: true}, nil
}
func (fakePermissions) Choose(context.Context, tool.Question) (string, error) {
	return "", nil
}

func fakeEnv(f *fakeFS, id string, class tool.Class) tool.Env {
	profile := tool.Profile{Mode: tool.ModeAgent, Class: class, Window: 16384}
	return tool.Env{Session: protocol.SessionInfo{ID: id}, Cwd: "/work", Profile: profile, Limits: profile.Limits(), Exec: f, Permissions: fakePermissions{}}
}
func callArgs(v any) json.RawMessage {
	b, _ := json.Marshal(v)
	return b
}

func TestReadRangesAndAnchor(t *testing.T) {
	fs := newFakeFS(map[string]string{"/work/a.go": "one\ntwo\nthree\nfour\n"})
	env := fakeEnv(fs, t.Name(), tool.ClassFull)
	result := (readTool{}).Run(context.Background(), tool.Call{Args: callArgs(map[string]string{"path": "a.go:2-3"})}, env)
	if result.IsError || !strings.Contains(result.Text, "2:two\n3:three") || strings.Contains(result.Text, "4:four") {
		t.Fatalf("range result = %#v", result)
	}
	if !strings.Contains(result.Text, "[a.go#"+fileTag("one\ntwo\nthree\nfour\n")+"]") {
		t.Fatalf("missing edit anchor: %s", result.Text)
	}
	raw := (readTool{}).Run(context.Background(), tool.Call{Args: callArgs(map[string]string{"path": "a.go:2:raw"})}, env)
	if raw.IsError || raw.Text != "two" {
		t.Fatalf("raw one-line selection = %#v", raw)
	}
}

func TestCompactEditAmbiguity(t *testing.T) {
	fs := newFakeFS(map[string]string{"/work/a.go": "apple\napple\n"})
	env := fakeEnv(fs, t.Name(), tool.ClassCompact)
	call := tool.Call{Args: callArgs(map[string]any{"path": "a.go", "old": "apple", "new": "pear"})}
	result := (editTool{}).Run(context.Background(), call, env)
	if !result.IsError || !strings.Contains(result.Text, "lines 1, 2") || fs.writes != 0 {
		t.Fatalf("ambiguous edit changed file: %#v", result)
	}
	call.Args = callArgs(map[string]any{"path": "a.go", "old": " apple  ", "new": "pear", "all": true})
	result = (editTool{}).Run(context.Background(), call, env)
	if result.IsError || string(fs.files["/work/a.go"]) != "pear\npear\n" || result.Diff == nil {
		t.Fatalf("whitespace-tolerant edit = %#v, file %q", result, fs.files["/work/a.go"])
	}
}

func TestHashlineStaleTagAndOriginalLineNumbers(t *testing.T) {
	fs := newFakeFS(map[string]string{"/work/a.go": "one\ntwo\nthree\nfour\n"})
	env := fakeEnv(fs, t.Name(), tool.ClassFull)
	ctx := context.Background()
	read := (readTool{}).Run(ctx, tool.Call{Args: callArgs(map[string]string{"path": "a.go"})}, env)
	if read.IsError {
		t.Fatal(read.Text)
	}
	tag := fileTag("one\ntwo\nthree\nfour\n")
	patch := "*** Begin Patch\n[a.go#" + tag + "]\nPUT 1.=1:\n+ONE\nCUT 3.=3 @part\nPUT >4 @part\n*** End Patch\n"
	result := (editTool{}).Run(ctx, tool.Call{Args: callArgs(map[string]string{"input": patch})}, env)
	if result.IsError || string(fs.files["/work/a.go"]) != "ONE\ntwo\nfour\nthree\n" || result.Diff == nil {
		t.Fatalf("patch = %#v, file %q", result, fs.files["/work/a.go"])
	}
	result = (editTool{}).Run(ctx, tool.Call{Args: callArgs(map[string]string{"input": patch})}, env)
	if !result.IsError || !strings.Contains(result.Text, "changed between read and edit") || fs.writes != 1 {
		t.Fatalf("stale tag = %#v", result)
	}
}

func TestGrepOutputLimits(t *testing.T) {
	fs := newFakeFS(map[string]string{"/work": ""})
	fs.grep = exec.GrepResult{Matches: []exec.Match{
		{Path: "/work/a.go", Line: 4, Text: "test"},
		{Path: "/work/b.go", Line: 4, Text: "test"},
		{Path: "/work/c.go", Line: 4, Text: "test"},
		{Path: "/work/d.go", Line: 4, Text: "test"},
	}, Files: 8, Truncated: true}
	env := fakeEnv(fs, t.Name(), tool.ClassFull)
	env.Limits.GrepFiles = 2
	result := (grepTool{}).Run(context.Background(), tool.Call{Args: callArgs(map[string]any{"pattern": "test", "path": "/work", "skip": 3})}, env)
	if result.IsError || fs.greps[0].MaxFiles != 5 || !strings.Contains(result.Text, "skip=4") {
		t.Fatalf("grep paging = %#v, request = %+v", result, fs.greps)
	}
}

func TestSecretReadRefusesBeforeExecutor(t *testing.T) {
	fs := newFakeFS(map[string]string{"/work/.env": "key=value\n"})
	result := (readTool{}).Run(context.Background(), tool.Call{Args: callArgs(map[string]string{"path": ".env"})}, fakeEnv(fs, t.Name(), tool.ClassFull))
	if !result.IsError || !strings.Contains(result.Text, "refused") || fs.reads != 0 {
		t.Fatalf("secret read = %#v", result)
	}
}

func TestSpillReadPagesThroughExecutor(t *testing.T) {
	fs := newFakeFS(nil)
	env := fakeEnv(fs, t.Name(), tool.ClassCompact)
	text := strings.Repeat("line of retained output\n", 350)
	shaped := Shape(env, text)
	if !strings.Contains(shaped, "local://1.log") || fs.writes != 1 {
		t.Fatalf("spill was not written through executor: %s", shaped)
	}
	result := (readTool{}).Run(context.Background(), tool.Call{Args: callArgs(map[string]string{"path": "local://1.log:10-11"})}, env)
	if result.IsError || !strings.Contains(result.Text, "10:line of retained output") || !strings.Contains(result.Text, "11:line of retained output") {
		t.Fatalf("spill page = %#v", result)
	}
}

func TestWriteSecretAndBackupBeforeChange(t *testing.T) {
	fs := newFakeFS(map[string]string{"/work/.env": "SECRET=original\n", "/work/note.txt": "\uFEFFold\nsecond\r\n"})
	env := fakeEnv(fs, t.Name(), tool.ClassCompact)
	ctx := context.Background()
	result := (writeTool{}).Run(ctx, tool.Call{Args: callArgs(map[string]string{"path": ".env", "content": "SECRET=new\n"})}, env)
	if !result.IsError || fs.writes != 0 || string(fs.files["/work/.env"]) != "SECRET=original\n" {
		t.Fatalf("secret file was modified: %#v", result)
	}
	backed := ""
	env.Backup = func(path string, previous []byte) (string, error) {
		if path != "/work/note.txt" {
			t.Fatalf("backup path = %s", path)
		}
		backed = string(previous)
		return "before/note.txt", nil
	}
	result = (writeTool{}).Run(ctx, tool.Call{Args: callArgs(map[string]string{"path": "note.txt", "content": "new\n"})}, env)
	if result.IsError || backed != "\uFEFFold\nsecond\r\n" || string(fs.files["/work/note.txt"]) != "\uFEFFnew\n" || result.Diff == nil {
		t.Fatalf("write and backup = %#v, backed = %q", result, backed)
	}
}

type recordedApproval struct{ requests []tool.Request }

func (p *recordedApproval) Allow(_ context.Context, request tool.Request) (tool.Decision, error) {
	p.requests = append(p.requests, request)
	return tool.Decision{Allowed: true}, nil
}
func (*recordedApproval) Choose(context.Context, tool.Question) (string, error) {
	return "", nil
}

func TestWriteThroughSymlinkChecksMissingTargetAndNamesRealDestination(t *testing.T) {
	f := newFakeFS(nil)
	f.links["/work/link"] = "/work/.ssh"
	env := fakeEnv(f, t.Name(), tool.ClassCompact)
	call := tool.Call{Args: callArgs(map[string]string{"path": "link/authorized_keys", "content": "ssh-ed25519 test\n"})}
	result := (writeTool{}).Run(context.Background(), call, env)
	if !result.IsError || f.writes != 0 || !strings.Contains(result.Text, "SSH data") {
		t.Fatalf("secret symlink target was writable: %#v", result)
	}
	f.links["/work/link"] = "/work/real"
	f.dirs["/work/real"] = true
	approved := &recordedApproval{}
	env.Permissions = approved
	call.Args = callArgs(map[string]string{"path": "link/note.txt", "content": "safe\n"})
	result = (writeTool{}).Run(context.Background(), call, env)
	if result.IsError || string(f.files["/work/real/note.txt"]) != "safe\n" ||
		len(approved.requests) != 1 || !strings.Contains(approved.requests[0].Input, "/work/real/note.txt") ||
		approved.requests[0].Diff.Path != "/work/real/note.txt" ||
		!strings.Contains(result.Text, "link/note.txt -> /work/real/note.txt") {
		t.Fatalf("untruthful linked write: result=%#v approvals=%#v", result, approved.requests)
	}
}

func TestAnchoredMoveChecksRealDestinationAndShowsIt(t *testing.T) {
	f := newFakeFS(map[string]string{"/work/source.txt": "one\n"})
	f.links["/work/link"] = "/work/.ssh"
	env := fakeEnv(f, t.Name(), tool.ClassFull)
	ctx := context.Background()
	if read := (readTool{}).Run(ctx, tool.Call{Args: callArgs(map[string]string{"path": "source.txt"})}, env); read.IsError {
		t.Fatal(read.Text)
	}
	tag := fileTag("one\n")
	patch := "*** Begin Patch\n[source.txt#" + tag + "]\nMV link/new.txt\n*** End Patch\n"
	result := (editTool{}).Run(ctx, tool.Call{Args: callArgs(map[string]string{"input": patch})}, env)
	if !result.IsError || f.writes != 0 {
		t.Fatalf("move into secret link passed: %#v", result)
	}
	f.links["/work/link"] = "/work/real"
	f.dirs["/work/real"] = true
	approved := &recordedApproval{}
	env.Permissions = approved
	result = (editTool{}).Run(ctx, tool.Call{Args: callArgs(map[string]string{"input": patch})}, env)
	if result.IsError || string(f.files["/work/real/new.txt"]) != "one\n" || len(approved.requests) != 1 ||
		!strings.Contains(approved.requests[0].Input, "/work/real/new.txt") ||
		!strings.Contains(result.Text, "link/new.txt -> /work/real/new.txt") {
		t.Fatalf("move approval omitted real destination: result=%#v approvals=%#v", result, approved.requests)
	}
}

func TestReadAndGrepResolveRelativeSymlinksInsideSession(t *testing.T) {
	f := newFakeFS(map[string]string{"/work/.ssh/key.txt": "private\n"})
	f.links["/work/alias"] = ".ssh"
	env := fakeEnv(f, t.Name(), tool.ClassFull)
	ctx := context.Background()
	read := (readTool{}).Run(ctx, tool.Call{Args: callArgs(map[string]string{"path": "alias/key.txt"})}, env)
	if !read.IsError || strings.Contains(read.Text, "private") || f.reads != 0 {
		t.Fatalf("read followed secret relative symlink: %#v", read)
	}
	found := (grepTool{}).Run(ctx, tool.Call{Args: callArgs(map[string]string{"path": "alias", "pattern": "private"})}, env)
	if !found.IsError || len(f.greps) != 0 {
		t.Fatalf("grep traversed secret relative symlink: %#v", found)
	}
}

func TestEditsKeepEachOriginalEndingAndExactBackup(t *testing.T) {
	original := "\uFEFFone\ntwo\r\nthree\nfour\r\n"
	f := newFakeFS(map[string]string{"/work/mixed.txt": original})
	env := fakeEnv(f, t.Name(), tool.ClassCompact)
	var backed []byte
	env.Backup = func(_ string, previous []byte) (string, error) {
		backed = append([]byte(nil), previous...)
		return "before/mixed.txt", nil
	}
	result := (editTool{}).Run(context.Background(), tool.Call{Args: callArgs(map[string]string{
		"path": "mixed.txt", "old": "one", "new": "ONE",
	})}, env)
	if result.IsError || string(f.files["/work/mixed.txt"]) != "\uFEFFONE\ntwo\r\nthree\nfour\r\n" || string(backed) != original {
		t.Fatalf("edit lost original endings: result=%#v file=%q backup=%q", result, f.files["/work/mixed.txt"], backed)
	}
	env.Profile.Class = tool.ClassFull
	env.Session.ID = t.Name() + "-anchor"
	read := (readTool{}).Run(context.Background(), tool.Call{Args: callArgs(map[string]string{"path": "mixed.txt"})}, env)
	if read.IsError {
		t.Fatal(read.Text)
	}
	tag := fileTag("ONE\ntwo\nthree\nfour\n")
	patch := "*** Begin Patch\n[mixed.txt#" + tag + "]\nPUT 2.=2:\n+TWO\n*** End Patch\n"
	result = (editTool{}).Run(context.Background(), tool.Call{Args: callArgs(map[string]string{"input": patch})}, env)
	if result.IsError || string(f.files["/work/mixed.txt"]) != "\uFEFFONE\nTWO\r\nthree\nfour\r\n" ||
		string(backed) != "\uFEFFONE\ntwo\r\nthree\nfour\r\n" {
		t.Fatalf("anchored edit changed unrelated endings: result=%#v file=%q backup=%q", result, f.files["/work/mixed.txt"], backed)
	}
	if got := string(splitText([]byte(original)).join("inserted\none\ntwo\nthree\nfour\n")); got != "\uFEFFinserted\none\ntwo\r\nthree\nfour\r\n" {
		t.Fatalf("insertion rewrote an existing line ending: %q", got)
	}
}

func TestRemoveSymlinkLeavesItsTargetUntouched(t *testing.T) {
	f := newFakeFS(map[string]string{"/work/target.txt": "one\n"})
	f.links["/work/link.txt"] = "target.txt"
	f.versions["/work/link.txt"] = 1
	env := fakeEnv(f, t.Name(), tool.ClassFull)
	read := (readTool{}).Run(context.Background(), tool.Call{Args: callArgs(map[string]string{"path": "link.txt"})}, env)
	if read.IsError {
		t.Fatal(read.Text)
	}
	tag := fileTag("one\n")
	patch := "*** Begin Patch\n[link.txt#" + tag + "]\nREM\n*** End Patch\n"
	result := (editTool{}).Run(context.Background(), tool.Call{Args: callArgs(map[string]string{"input": patch})}, env)
	if result.IsError || string(f.files["/work/target.txt"]) != "one\n" || f.links["/work/link.txt"] != "" {
		t.Fatalf("REM removed symlink target or left the link: result=%#v file=%q", result, f.files["/work/target.txt"])
	}
}

func TestFuzzyEditRejectsUnrelatedLine(t *testing.T) {
	var old, near []string
	for i := range 20 {
		old = append(old, fmt.Sprintf("item %02d: consistent", i))
		near = append(near, fmt.Sprintf("item %02d: consistent", i))
	}
	near[9] = "unrelated replacement line"
	if r, err := findReplace(strings.Join(near, "\n"), strings.Join(old, "\n"), "replacement", false, "a.go"); err == nil {
		t.Fatalf("fuzzy match replaced an unrelated line: %#v", r)
	}
	if r, err := findReplace("a single line with a one-character typo", "a single line with a one-character typ0", "replacement", false, "a.go"); err == nil {
		t.Fatalf("fuzzy match replaced one-char typo: %#v", r)
	}
}
