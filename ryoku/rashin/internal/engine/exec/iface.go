// Package exec runs every file and process operation of the engine's tools.
// An Executor either works in-process as the user (non-team sessions) or
// forwards each call to a tool host child inside the team task's sandbox
// (`rashin __toolhost`, JSON-RPC over stdio), so tools never touch the
// filesystem or start a process themselves and behave the same in both.
//
// Every request and result type here crosses that JSON-RPC pipe, hence the
// JSON tags: the tool host server decodes them as they are. Paths are
// absolute; a tool resolves a relative path against Env.Cwd before it calls.
package exec

import (
	"context"
	"errors"
	"time"
)

// Executor is one session's way to touch files and run programs. It is safe
// for concurrent use: the engine runs read tools in parallel.
//
// File writes are atomic (a temporary file beside the target renamed into
// place); a replaced file keeps its mode, a new one is 0600 unless Mode says
// otherwise. Old and new contents come back so the tool can build a
// protocol.Diff without a second read.
//
// Processes run in their own process group. Their lifetime is governed by
// StartRequest.Timeout, Kill, Release and Close, never by the context given
// to Start, so a background job outlives the tool call that started it.
type Executor interface {
	// Stat describes one path without following a final symlink's target
	// for Kind (a link reports KindSymlink).
	Stat(ctx context.Context, path string) (FileInfo, error)
	// Read returns bytes of one regular file. ErrIsDir for a directory,
	// ErrNotRegular for a device, socket or fifo.
	Read(ctx context.Context, req ReadRequest) (ReadResult, error)
	// Write creates or replaces one whole file.
	Write(ctx context.Context, req WriteRequest) (WriteResult, error)
	// Edit applies exact text replacements to one file in a single atomic
	// write. Fuzzy matching and anchored (hashline) edits belong to the edit
	// tool, which computes the new content and calls Write with IfVersion.
	Edit(ctx context.Context, req EditRequest) (EditResult, error)
	// Remove deletes one regular file or symlink (never a directory). A
	// move is Write(dest, IfAbsent) then Remove(src, IfVersion).
	Remove(ctx context.Context, req RemoveRequest) error
	// List reads one directory.
	List(ctx context.Context, req ListRequest) (ListResult, error)
	// Glob finds paths under a root whose relative path matches a pattern.
	Glob(ctx context.Context, req GlobRequest) (GlobResult, error)
	// Grep searches file contents under a root.
	Grep(ctx context.Context, req GrepRequest) (GrepResult, error)

	// Start runs a program and returns its process id. Output, stdout and
	// stderr merged in arrival order, is streamed to out (which may be nil)
	// from the executor's own goroutine, in UTF-8 safe chunks, and also
	// retained up to StartRequest.Keep bytes for Wait and Peek.
	Start(ctx context.Context, req StartRequest, out OutputFunc) (string, error)
	// Wait blocks until the process exits or ctx ends. On ctx end it returns
	// ctx.Err() and leaves the process running; the caller decides whether
	// to Kill it. After exit the result stays available until Release.
	Wait(ctx context.Context, id string) (Exit, error)
	// Peek reports a process without waiting: Running, the retained output
	// so far, and the exit once it has one.
	Peek(ctx context.Context, id string) (Exit, error)
	// Kill ends the whole process group: SIGTERM, then SIGKILL after a short
	// grace. Killing an exited process is not an error.
	Kill(ctx context.Context, id string) error
	// Release forgets an exited process and its retained output, killing it
	// first if it still runs.
	Release(ctx context.Context, id string) error
	// Running lists the processes not yet released, so the runner reports
	// Alive while a background job runs.
	Running() []string

	// Close kills every process and, for the sandbox executor, stops the
	// tool host child. The executor may be used again afterwards: the tool
	// host starts on the next call.
	Close() error
}

// OutputFunc receives streamed process output. It must not block for long:
// it runs on the executor's reader goroutine.
type OutputFunc func(Output)

// Output is one streamed chunk of a process's output.
type Output struct {
	ID    string `json:"id"`
	Delta string `json:"delta"`
}

// Kinds of a FileInfo.
const (
	KindFile    = "file"
	KindDir     = "dir"
	KindSymlink = "symlink"
	KindOther   = "other"
)

// FileInfo describes one path.
type FileInfo struct {
	// Path is absolute.
	Path string `json:"path"`
	// Name is the last element of Path.
	Name    string    `json:"name"`
	Kind    string    `json:"kind"`
	Size    int64     `json:"size"`
	Mode    uint32    `json:"mode"`
	ModTime time.Time `json:"modTime"`
	// Target is a symlink's destination as stored.
	Target string `json:"target,omitempty"`
	// Version changes whenever the file's content may have changed (its
	// modification time and size). A write with IfVersion refuses when the
	// file moved on since the version was read.
	Version string `json:"version"`
}

// ReadRequest reads Limit bytes from Offset; Limit 0 reads up to MaxRead.
type ReadRequest struct {
	Path   string `json:"path"`
	Offset int64  `json:"offset,omitempty"`
	Limit  int64  `json:"limit,omitempty"`
}

// ReadResult is what Read returned. Truncated marks bytes left after the
// returned range; Binary marks a file with a NUL byte or invalid UTF-8 in
// the returned range.
type ReadResult struct {
	Data      []byte   `json:"data"`
	Info      FileInfo `json:"info"`
	Truncated bool     `json:"truncated,omitempty"`
	Binary    bool     `json:"binary,omitempty"`
}

// MaxRead caps one Read, and the old content Write and Edit return.
const MaxRead = 4 << 20

// WriteRequest replaces Path with Content. Mode applies to a new file only
// (0 means 0600). MakeDirs creates missing parent directories (0755).
// IfVersion, when set, refuses with ErrChanged unless the file is still at
// that FileInfo.Version; IfAbsent refuses with ErrExists when the file
// exists.
type WriteRequest struct {
	Path      string `json:"path"`
	Content   []byte `json:"content"`
	Mode      uint32 `json:"mode,omitempty"`
	MakeDirs  bool   `json:"makeDirs,omitempty"`
	IfVersion string `json:"ifVersion,omitempty"`
	IfAbsent  bool   `json:"ifAbsent,omitempty"`
}

// WriteResult reports a write. Old is the previous content (nil for a new
// file, capped at MaxRead with OldTruncated); Info is the file after it.
type WriteResult struct {
	Existed      bool     `json:"existed"`
	Old          []byte   `json:"old,omitempty"`
	OldTruncated bool     `json:"oldTruncated,omitempty"`
	Info         FileInfo `json:"info"`
}

// Replacement swaps Old for New: exactly once, or every occurrence with All.
// Old must not be empty.
type Replacement struct {
	Old string `json:"old"`
	New string `json:"new"`
	All bool   `json:"all,omitempty"`
}

// EditRequest applies Replacements in order, each to the result of the one
// before. Nothing is written when any of them fails: ErrNoMatch when Old is
// absent, ErrAmbiguous (with Error.Lines) when Old occurs more than once and
// All is false.
type EditRequest struct {
	Path         string        `json:"path"`
	Replacements []Replacement `json:"replacements"`
	IfVersion    string        `json:"ifVersion,omitempty"`
}

// EditResult carries both whole contents for the diff and how many
// occurrences each replacement changed.
type EditResult struct {
	Old    []byte   `json:"old"`
	New    []byte   `json:"new"`
	Counts []int    `json:"counts"`
	Info   FileInfo `json:"info"`
}

// RemoveRequest deletes Path: ErrIsDir for a directory, ErrNotFound when it
// is gone, ErrChanged when IfVersion is set and the file moved on since.
type RemoveRequest struct {
	Path      string `json:"path"`
	IfVersion string `json:"ifVersion,omitempty"`
}

// ListRequest lists Path. Hidden includes dot entries; Limit 0 means 1000.
type ListRequest struct {
	Path   string `json:"path"`
	Hidden bool   `json:"hidden,omitempty"`
	Limit  int    `json:"limit,omitempty"`
}

// ListResult holds entries sorted directories first, then by name.
// Truncated marks entries left out by Limit; Total counts them all.
type ListResult struct {
	Entries   []FileInfo `json:"entries"`
	Total     int        `json:"total"`
	Truncated bool       `json:"truncated,omitempty"`
}

// SkipDirs are directory names Glob and Grep never descend into.
var SkipDirs = []string{".git", ".hg", ".svn", "node_modules", "__pycache__", ".venv", ".cache"}

// GlobRequest matches Pattern against paths relative to Root, with `**`
// spanning directories and `*`, `?`, `[...]` within one element. A pattern
// without a slash matches at any depth. Hidden includes dot entries;
// Limit 0 means 1000. Directories match only with Dirs.
type GlobRequest struct {
	Root    string `json:"root"`
	Pattern string `json:"pattern"`
	Hidden  bool   `json:"hidden,omitempty"`
	Dirs    bool   `json:"dirs,omitempty"`
	Limit   int    `json:"limit,omitempty"`
}

// GlobResult holds absolute paths, newest modification first. Truncated
// marks matches left out by Limit; Total counts them all.
type GlobResult struct {
	Paths     []string `json:"paths"`
	Total     int      `json:"total"`
	Truncated bool     `json:"truncated,omitempty"`
}

// GrepRequest searches files under Root (or Root itself when it is a file)
// for Pattern, an RE2 regular expression unless Literal. Include, a glob as
// in GlobRequest, limits which files are searched. Binary files are skipped.
// MaxFiles and MaxPerFile cap the matches returned (0 means 100 and 20);
// Context adds that many lines before and after each match.
type GrepRequest struct {
	Root       string `json:"root"`
	Pattern    string `json:"pattern"`
	Literal    bool   `json:"literal,omitempty"`
	IgnoreCase bool   `json:"ignoreCase,omitempty"`
	Include    string `json:"include,omitempty"`
	Hidden     bool   `json:"hidden,omitempty"`
	MaxFiles   int    `json:"maxFiles,omitempty"`
	MaxPerFile int    `json:"maxPerFile,omitempty"`
	Context    int    `json:"context,omitempty"`
}

// Match is one matching line. Before and After are its context lines.
type Match struct {
	Path   string   `json:"path"`
	Line   int      `json:"line"`
	Text   string   `json:"text"`
	Before []string `json:"before,omitempty"`
	After  []string `json:"after,omitempty"`
}

// GrepResult lists matches grouped by file in search order. Files counts
// every file that matched, Truncated marks matches left out by the caps.
type GrepResult struct {
	Matches   []Match `json:"matches"`
	Files     int     `json:"files"`
	Truncated bool    `json:"truncated,omitempty"`
}

// StartRequest runs Argv (Argv[0] looked up on the executor's PATH) in Dir
// with Env added to the executor's environment. Timeout 0 means no limit;
// past it the process group is killed and Exit.TimedOut set. Keep caps the
// retained output (0 means 1 MiB): when output exceeds it the head is kept
// and the rest dropped until the last Keep/4 bytes, which are kept as the
// tail. Stdin, when set, is written and then closed; otherwise stdin is
// /dev/null.
type StartRequest struct {
	Argv    []string          `json:"argv"`
	Dir     string            `json:"dir"`
	Env     map[string]string `json:"env,omitempty"`
	Timeout time.Duration     `json:"timeout,omitempty"`
	Keep    int               `json:"keep,omitempty"`
	Stdin   []byte            `json:"stdin,omitempty"`
}

// Exit is a process's state. Code is the exit code (-1 when killed by a
// signal, then Signal names it). Output is the retained output; Truncated
// marks bytes dropped from its middle.
type Exit struct {
	ID        string        `json:"id"`
	Running   bool          `json:"running,omitempty"`
	Code      int           `json:"code"`
	Signal    string        `json:"signal,omitempty"`
	TimedOut  bool          `json:"timedOut,omitempty"`
	Killed    bool          `json:"killed,omitempty"`
	Output    string        `json:"output"`
	Truncated bool          `json:"truncated,omitempty"`
	Elapsed   time.Duration `json:"elapsed"`
}

// Error codes, stable across the tool host pipe.
const (
	CodeNotFound    = "not_found"
	CodeIsDir       = "is_dir"
	CodeNotDir      = "not_dir"
	CodeNotRegular  = "not_regular"
	CodeDenied      = "denied"
	CodeExists      = "exists"
	CodeChanged     = "changed"
	CodeNoMatch     = "no_match"
	CodeAmbiguous   = "ambiguous"
	CodeTooLarge    = "too_large"
	CodeBadPattern  = "bad_pattern"
	CodeNoProcess   = "no_process"
	CodeUnavailable = "unavailable"
)

// Error is an executor failure a tool can explain to the model. The tool
// host sends it as JSON-RPC error data so it survives the pipe; Is compares
// codes, so errors.Is(err, ErrNotFound) works on both sides.
type Error struct {
	Code    string `json:"code"`
	Path    string `json:"path,omitempty"`
	Message string `json:"message,omitempty"`
	// Lines are 1-based line numbers of every occurrence, for CodeAmbiguous.
	Lines []int `json:"lines,omitempty"`
}

func (e *Error) Error() string {
	switch {
	case e.Message != "":
		return e.Message
	case e.Path != "":
		return e.Code + ": " + e.Path
	default:
		return e.Code
	}
}

// Is matches any *Error with the same code.
func (e *Error) Is(target error) bool {
	var t *Error
	return errors.As(target, &t) && t.Code == e.Code
}

// Sentinels for errors.Is.
var (
	ErrNotFound    = &Error{Code: CodeNotFound}
	ErrIsDir       = &Error{Code: CodeIsDir}
	ErrNotDir      = &Error{Code: CodeNotDir}
	ErrNotRegular  = &Error{Code: CodeNotRegular}
	ErrDenied      = &Error{Code: CodeDenied}
	ErrExists      = &Error{Code: CodeExists}
	ErrChanged     = &Error{Code: CodeChanged}
	ErrNoMatch     = &Error{Code: CodeNoMatch}
	ErrAmbiguous   = &Error{Code: CodeAmbiguous}
	ErrTooLarge    = &Error{Code: CodeTooLarge}
	ErrBadPattern  = &Error{Code: CodeBadPattern}
	ErrNoProcess   = &Error{Code: CodeNoProcess}
	ErrUnavailable = &Error{Code: CodeUnavailable}
)
