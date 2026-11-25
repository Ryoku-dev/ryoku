package exec

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"unicode/utf8"
)

// HostOptions configure a host executor.
type HostOptions struct {
	// Env is the environment programs start from; nil reads the process's
	// own environment at each Start.
	Env []string
	// MaxProcesses caps the processes running at once; 0 means 32.
	MaxProcesses int
}

const defaultMaxProcesses = 32

// Host returns an executor that works in this process, as the user. The
// tool host runs one inside the sandbox.
func Host(opts HostOptions) Executor {
	if opts.MaxProcesses <= 0 {
		opts.MaxProcesses = defaultMaxProcesses
	}
	return &host{opts: opts, procs: make(map[string]*process)}
}

type host struct {
	opts HostOptions

	// writeMu serialises writes, edits and removals so one session never
	// loses its own update between a version check and the rename.
	writeMu sync.Mutex

	procMu sync.Mutex
	procs  map[string]*process
	next   int
}

func (h *host) Stat(ctx context.Context, path string) (FileInfo, error) {
	if err := ctx.Err(); err != nil {
		return FileInfo{}, err
	}
	p, err := absPath(path)
	if err != nil {
		return FileInfo{}, err
	}
	fi, err := os.Lstat(p)
	if err != nil {
		return FileInfo{}, fsError(p, err)
	}
	return infoOf(p, fi), nil
}

func (h *host) Read(ctx context.Context, req ReadRequest) (ReadResult, error) {
	if err := ctx.Err(); err != nil {
		return ReadResult{}, err
	}
	p, err := absPath(req.Path)
	if err != nil {
		return ReadResult{}, err
	}
	// a fifo would block the open, so the kind is checked first
	fi, err := os.Stat(p)
	if err != nil {
		return ReadResult{}, fsError(p, err)
	}
	if err := regular(p, fi); err != nil {
		return ReadResult{}, err
	}
	f, err := os.Open(p)
	if err != nil {
		return ReadResult{}, fsError(p, err)
	}
	defer f.Close()
	if fi, err = f.Stat(); err != nil {
		return ReadResult{}, fsError(p, err)
	}
	limit := req.Limit
	if limit <= 0 || limit > MaxRead {
		limit = MaxRead
	}
	offset := max(req.Offset, 0)
	if offset > 0 {
		if _, err := f.Seek(offset, io.SeekStart); err != nil {
			return ReadResult{}, fsError(p, err)
		}
	}
	data, err := io.ReadAll(io.LimitReader(f, limit+1))
	if err != nil {
		return ReadResult{}, fsError(p, err)
	}
	truncated := int64(len(data)) > limit
	if truncated {
		data = data[:limit]
	}
	return ReadResult{
		Data:      data,
		Info:      fileInfo(p, fi),
		Truncated: truncated,
		Binary:    binary(data, offset > 0, truncated),
	}, nil
}

func (h *host) Write(ctx context.Context, req WriteRequest) (WriteResult, error) {
	if err := ctx.Err(); err != nil {
		return WriteResult{}, err
	}
	p, err := absPath(req.Path)
	if err != nil {
		return WriteResult{}, err
	}
	h.writeMu.Lock()
	defer h.writeMu.Unlock()
	target, err := resolveLink(p)
	if err != nil {
		return WriteResult{}, fsError(p, err)
	}
	fi, err := os.Stat(target)
	existed := err == nil
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return WriteResult{}, fsError(p, err)
	}
	if existed {
		if err := regular(p, fi); err != nil {
			return WriteResult{}, err
		}
	}
	if req.IfAbsent {
		if existed {
			return WriteResult{}, &Error{Code: CodeExists, Path: p}
		}
		if _, err := os.Lstat(p); err == nil {
			return WriteResult{}, &Error{Code: CodeExists, Path: p}
		} else if !errors.Is(err, fs.ErrNotExist) {
			return WriteResult{}, fsError(p, err)
		}
	}
	if req.IfVersion != "" && (!existed || version(fi) != req.IfVersion) {
		return WriteResult{}, &Error{Code: CodeChanged, Path: p}
	}
	result := WriteResult{Existed: existed}
	mode := fs.FileMode(0o600)
	if req.Mode != 0 {
		mode = fs.FileMode(req.Mode).Perm()
	}
	if existed {
		mode = fi.Mode().Perm()
		if result.Old, result.OldTruncated, err = readCapped(target); err != nil {
			return WriteResult{}, fsError(p, err)
		}
	}
	dir := filepath.Dir(target)
	if req.MakeDirs {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return WriteResult{}, fsError(dir, err)
		}
	} else if err := isDir(dir); err != nil {
		return WriteResult{}, err
	}
	if err := ctx.Err(); err != nil {
		return WriteResult{}, err
	}
	if err := writeAtomic(target, req.Content, mode); err != nil {
		return WriteResult{}, fsError(p, err)
	}
	after, err := os.Stat(target)
	if err != nil {
		return WriteResult{}, fsError(p, err)
	}
	result.Info = fileInfo(p, after)
	return result, nil
}

func (h *host) Edit(ctx context.Context, req EditRequest) (EditResult, error) {
	if err := ctx.Err(); err != nil {
		return EditResult{}, err
	}
	p, err := absPath(req.Path)
	if err != nil {
		return EditResult{}, err
	}
	if len(req.Replacements) == 0 {
		return EditResult{}, &Error{Code: CodeNoMatch, Path: p, Message: "no replacements given for " + p}
	}
	h.writeMu.Lock()
	defer h.writeMu.Unlock()
	target, err := resolveLink(p)
	if err != nil {
		return EditResult{}, fsError(p, err)
	}
	fi, err := os.Stat(target)
	if err != nil {
		return EditResult{}, fsError(p, err)
	}
	if err := regular(p, fi); err != nil {
		return EditResult{}, err
	}
	if fi.Size() > MaxRead {
		return EditResult{}, &Error{Code: CodeTooLarge, Path: p}
	}
	if req.IfVersion != "" && version(fi) != req.IfVersion {
		return EditResult{}, &Error{Code: CodeChanged, Path: p}
	}
	old, err := os.ReadFile(target)
	if err != nil {
		return EditResult{}, fsError(p, err)
	}
	if len(old) > MaxRead {
		return EditResult{}, &Error{Code: CodeTooLarge, Path: p}
	}
	text := string(old)
	counts := make([]int, 0, len(req.Replacements))
	for i, r := range req.Replacements {
		which := ""
		if len(req.Replacements) > 1 {
			which = fmt.Sprintf("replacement %d of %d: ", i+1, len(req.Replacements))
		}
		if r.Old == "" {
			return EditResult{}, &Error{Code: CodeNoMatch, Path: p, Message: which + "the old text is empty"}
		}
		n := strings.Count(text, r.Old)
		switch {
		case n == 0:
			return EditResult{}, &Error{Code: CodeNoMatch, Path: p, Message: which + "the old text is not in " + p}
		case n > 1 && !r.All:
			return EditResult{}, &Error{Code: CodeAmbiguous, Path: p, Lines: occurrenceLines(text, r.Old),
				Message: fmt.Sprintf("%sthe old text occurs %d times in %s", which, n, p)}
		}
		if r.All {
			text = strings.ReplaceAll(text, r.Old, r.New)
		} else {
			text = strings.Replace(text, r.Old, r.New, 1)
		}
		counts = append(counts, n)
	}
	if err := ctx.Err(); err != nil {
		return EditResult{}, err
	}
	updated := []byte(text)
	if !bytes.Equal(old, updated) {
		if err := writeAtomic(target, updated, fi.Mode().Perm()); err != nil {
			return EditResult{}, fsError(p, err)
		}
		if fi, err = os.Stat(target); err != nil {
			return EditResult{}, fsError(p, err)
		}
	}
	return EditResult{Old: old, New: updated, Counts: counts, Info: fileInfo(p, fi)}, nil
}

func (h *host) Remove(ctx context.Context, req RemoveRequest) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	p, err := absPath(req.Path)
	if err != nil {
		return err
	}
	h.writeMu.Lock()
	defer h.writeMu.Unlock()
	fi, err := os.Lstat(p)
	if err != nil {
		return fsError(p, err)
	}
	link := fi.Mode()&fs.ModeSymlink != 0
	switch {
	case fi.IsDir():
		return &Error{Code: CodeIsDir, Path: p}
	case !link && !fi.Mode().IsRegular():
		return &Error{Code: CodeNotRegular, Path: p}
	}
	if req.IfVersion != "" && version(fi) != req.IfVersion {
		// Read describes a link's target while Stat describes the link.
		// Either version can be the one the editing tool last observed.
		if !link {
			return &Error{Code: CodeChanged, Path: p}
		}
		target, err := os.Stat(p)
		if err != nil || version(target) != req.IfVersion {
			return &Error{Code: CodeChanged, Path: p}
		}
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := os.Remove(p); err != nil {
		return fsError(p, err)
	}
	return nil
}

func (h *host) List(ctx context.Context, req ListRequest) (ListResult, error) {
	if err := ctx.Err(); err != nil {
		return ListResult{}, err
	}
	p, err := absPath(req.Path)
	if err != nil {
		return ListResult{}, err
	}
	entries, err := os.ReadDir(p)
	if err != nil {
		return ListResult{}, fsError(p, err)
	}
	limit := req.Limit
	if limit <= 0 {
		limit = 1000
	}
	all := make([]FileInfo, 0, len(entries))
	for _, entry := range entries {
		name := entry.Name()
		if !req.Hidden && strings.HasPrefix(name, ".") {
			continue
		}
		fi, err := entry.Info()
		if err != nil {
			continue // gone since the directory was read
		}
		all = append(all, infoOf(filepath.Join(p, name), fi))
	}
	sortEntries(all)
	result := ListResult{Entries: all, Total: len(all)}
	if len(all) > limit {
		result.Entries, result.Truncated = all[:limit], true
	}
	return result, ctx.Err()
}

func sortEntries(entries []FileInfo) {
	dirFirst := func(a, b FileInfo) int {
		ad, bd := a.Kind == KindDir, b.Kind == KindDir
		switch {
		case ad && !bd:
			return -1
		case bd && !ad:
			return 1
		}
		return strings.Compare(a.Name, b.Name)
	}
	slices.SortFunc(entries, dirFirst)
}

// absPath refuses a relative path: the executor has no working directory of
// its own, the tool resolves against its session's.
func absPath(path string) (string, error) {
	if !filepath.IsAbs(path) {
		return "", &Error{Code: CodeDenied, Path: path, Message: "path must be absolute: " + path}
	}
	return filepath.Clean(path), nil
}

func regular(p string, fi fs.FileInfo) error {
	switch {
	case fi.IsDir():
		return &Error{Code: CodeIsDir, Path: p}
	case !fi.Mode().IsRegular():
		return &Error{Code: CodeNotRegular, Path: p}
	}
	return nil
}

func isDir(dir string) error {
	fi, err := os.Stat(dir)
	switch {
	case errors.Is(err, fs.ErrNotExist):
		return &Error{Code: CodeNotFound, Path: dir, Message: "directory does not exist: " + dir}
	case err != nil:
		return fsError(dir, err)
	case !fi.IsDir():
		return &Error{Code: CodeNotDir, Path: dir}
	}
	return nil
}

// fsError maps an OS failure onto the codes a tool explains; anything else
// stays as it is.
func fsError(p string, err error) error {
	if err == nil {
		return nil
	}
	var e *Error
	if errors.As(err, &e) {
		return err
	}
	switch {
	case errors.Is(err, fs.ErrNotExist):
		return &Error{Code: CodeNotFound, Path: p}
	case errors.Is(err, fs.ErrPermission):
		return &Error{Code: CodeDenied, Path: p}
	case errors.Is(err, syscall.EROFS):
		return &Error{Code: CodeDenied, Path: p, Message: p + " is read-only here"}
	case errors.Is(err, syscall.ENOTDIR):
		return &Error{Code: CodeNotDir, Path: p}
	case errors.Is(err, syscall.EISDIR):
		return &Error{Code: CodeIsDir, Path: p}
	case errors.Is(err, syscall.ELOOP):
		return &Error{Code: CodeDenied, Path: p, Message: "too many levels of symbolic links: " + p}
	}
	return err
}

func kindOf(mode fs.FileMode) string {
	switch {
	case mode&fs.ModeSymlink != 0:
		return KindSymlink
	case mode.IsDir():
		return KindDir
	case mode.IsRegular():
		return KindFile
	}
	return KindOther
}

// infoOf describes p from an Lstat, reading a symlink's destination.
func infoOf(p string, fi fs.FileInfo) FileInfo {
	info := fileInfo(p, fi)
	if info.Kind == KindSymlink {
		info.Target, _ = os.Readlink(p)
	}
	return info
}

func fileInfo(p string, fi fs.FileInfo) FileInfo {
	return FileInfo{
		Path:    p,
		Name:    filepath.Base(p),
		Kind:    kindOf(fi.Mode()),
		Size:    fi.Size(),
		Mode:    uint32(fi.Mode().Perm()),
		ModTime: fi.ModTime(),
		Version: version(fi),
	}
}

func version(fi fs.FileInfo) string {
	v := strconv.FormatInt(fi.ModTime().UnixNano(), 36) + "." + strconv.FormatInt(fi.Size(), 36)
	if stat, ok := fi.Sys().(*syscall.Stat_t); ok {
		v += "." + strconv.FormatUint(stat.Dev, 36) + "." + strconv.FormatUint(stat.Ino, 36)
		v += "." + strconv.FormatInt(stat.Ctim.Sec, 36) + "." + strconv.FormatInt(stat.Ctim.Nsec, 36)
	}
	return v
}

// binary reports a NUL byte or invalid UTF-8, forgiving a rune cut by the
// range's start or end.
func binary(data []byte, cutStart, cutEnd bool) bool {
	if bytes.IndexByte(data, 0) >= 0 {
		return true
	}
	start, end := 0, len(data)
	if cutStart {
		for start < end && start < utf8.UTFMax-1 && !utf8.RuneStart(data[start]) {
			start++
		}
	}
	if cutEnd {
		end = completeRunes(data[start:]) + start
	}
	return !utf8.Valid(data[start:end])
}

// completeRunes is the length of data without an incomplete rune at its end.
func completeRunes(data []byte) int {
	for i := len(data) - 1; i >= 0 && i >= len(data)-utf8.UTFMax; i-- {
		if utf8.RuneStart(data[i]) {
			if !utf8.FullRune(data[i:]) {
				return i
			}
			break
		}
	}
	return len(data)
}

// resolveLink follows p while it is a symlink, so a write goes to the
// link's target and the link stays; a dangling link names the file to
// create.
func resolveLink(p string) (string, error) {
	for range 40 {
		fi, err := os.Lstat(p)
		if errors.Is(err, fs.ErrNotExist) {
			return p, nil
		}
		if err != nil {
			return "", err
		}
		if fi.Mode()&fs.ModeSymlink == 0 {
			return p, nil
		}
		dest, err := os.Readlink(p)
		if err != nil {
			return "", err
		}
		if !filepath.IsAbs(dest) {
			dest = filepath.Join(filepath.Dir(p), dest)
		}
		p = filepath.Clean(dest)
	}
	return "", syscall.ELOOP
}

func readCapped(p string) ([]byte, bool, error) {
	f, err := os.Open(p)
	if err != nil {
		return nil, false, err
	}
	defer f.Close()
	data, err := io.ReadAll(io.LimitReader(f, MaxRead+1))
	if err != nil {
		return nil, false, err
	}
	if len(data) > MaxRead {
		return data[:MaxRead], true, nil
	}
	return data, false, nil
}

// writeAtomic writes a temporary file beside path and renames it into place.
func writeAtomic(path string, content []byte, mode fs.FileMode) error {
	file, err := os.CreateTemp(filepath.Dir(path), ".rashin-write-*")
	if err != nil {
		return err
	}
	temp := file.Name()
	ok := false
	defer func() {
		_ = file.Close()
		if !ok {
			_ = os.Remove(temp)
		}
	}()
	if _, err := file.Write(content); err != nil {
		return err
	}
	if err := file.Chmod(mode); err != nil {
		return err
	}
	if err := file.Sync(); err != nil {
		return err
	}
	if err := file.Close(); err != nil {
		return err
	}
	if err := os.Rename(temp, path); err != nil {
		return err
	}
	ok = true
	return nil
}

// occurrenceLines lists the 1-based line of each occurrence of s, at most 50.
func occurrenceLines(text, s string) []int {
	var lines []int
	line, from := 1, 0
	for len(lines) < 50 {
		i := strings.Index(text[from:], s)
		if i < 0 {
			break
		}
		line += strings.Count(text[from:from+i], "\n")
		lines = append(lines, line)
		line += strings.Count(s, "\n")
		from += i + len(s)
	}
	return lines
}
