package localmodels

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	i18n "ryoku-i18n"
	"ryoku-rashin/internal/paths"
	"ryoku-rashin/internal/protocol"
)

// manifest marks a model installed. Owned files require checksum verification;
// external files retain their size and mtime instead of a full-file hash.
// Probed holds what the running server reported once the model loaded.
type manifest struct {
	ID       string        `json:"id"`
	Repo     string        `json:"repo"`
	Revision string        `json:"revision"`
	Files    []File        `json:"files"`
	Verified time.Time     `json:"verified"`
	Probed   *probed       `json:"probed,omitempty"`
	Header   *ggufMetadata `json:"header,omitempty"`
}

// probed is what llama-server reported for a loaded model.
type probed struct {
	Context int   `json:"context,omitempty"`
	Params  int64 `json:"params,omitempty"`
	Tools   bool  `json:"tools"`
	Vision  bool  `json:"vision"`
	Audio   bool  `json:"audio"`
}

var (
	repoPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]{0,95}/[A-Za-z0-9][A-Za-z0-9._-]{0,95}$`)
	idPattern   = regexp.MustCompile(`^[a-z0-9][a-z0-9._-]{0,63}$`)
)

// validRepo reports a Hugging Face repo id: owner/name, nothing that walks
// out of the models folder.
func validRepo(repo string) bool {
	return repoPattern.MatchString(repo) && !strings.Contains(repo, "..")
}

// validName reports a file path inside a repo: relative, clean, no
// parent steps, no control characters.
func validName(name string) bool {
	if name == "" || strings.HasPrefix(name, "/") || path.Clean(name) != name || strings.ContainsAny(name, "\\\x00\r\n") {
		return false
	}
	for _, part := range strings.Split(name, "/") {
		if part == ".." || part == "." || part == "" {
			return false
		}
	}
	return true
}

func repoDir(repo string) string {
	return filepath.Join(paths.ModelsDir(), filepath.FromSlash(repo))
}

func filePath(e Entry, name string) string {
	if e.External != "" {
		return filepath.Join(e.External, filepath.FromSlash(name))
	}
	return filepath.Join(repoDir(e.Repo), filepath.FromSlash(name))
}

func manifestPath(e Entry) string {
	return filepath.Join(repoDir(e.Repo), "rashin-"+e.ID+".json")
}

// customPath lists the models added from Hugging Face.
func customPath() string { return filepath.Join(paths.ModelsDir(), "custom.json") }

// readManifest returns the entry's manifest when it matches the entry's
// files and revision; one for other files is not this entry's install.
func readManifest(e Entry) (manifest, bool) {
	data, err := os.ReadFile(manifestPath(e))
	if err != nil {
		return manifest{}, false
	}
	var man manifest
	if json.Unmarshal(data, &man) != nil || man.ID != e.ID || man.Repo != e.Repo || man.Revision != e.Revision || len(man.Files) != len(e.Files) {
		return manifest{}, false
	}
	for i, f := range e.Files {
		if man.Files[i].Name != f.Name || man.Files[i].SHA256 != f.SHA256 || man.Files[i].Size != f.Size || man.Files[i].ModTime != f.ModTime {
			return manifest{}, false
		}
	}
	return man, true
}

func writeManifest(e Entry, man manifest) error {
	data, err := json.MarshalIndent(man, "", "  ")
	if err != nil {
		return err
	}
	return writeAtomic(manifestPath(e), append(data, '\n'), 0o600)
}

// writeAtomic replaces path with data through a synced temporary file.
func writeAtomic(name string, data []byte, perm os.FileMode) error {
	if err := os.MkdirAll(filepath.Dir(name), 0o700); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(name), "."+filepath.Base(name)+".*")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Chmod(perm); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), name)
}

// readCustom is the models the user added, oldest first.
func readCustom() []Entry {
	data, err := os.ReadFile(customPath())
	if err != nil {
		return nil
	}
	var entries []Entry
	if json.Unmarshal(data, &entries) != nil {
		return nil
	}
	out := entries[:0]
	for _, e := range entries {
		if idPattern.MatchString(e.ID) && validRepo(e.Repo) && validEntryFiles(e) {
			out = append(out, e)
		}
	}
	return out
}

func writeCustom(entries []Entry) error {
	if entries == nil {
		entries = []Entry{}
	}
	data, err := json.MarshalIndent(entries, "", "  ")
	if err != nil {
		return err
	}
	return writeAtomic(customPath(), append(data, '\n'), 0o600)
}

func validEntryFiles(e Entry) bool {
	models := 0
	var names []string
	if e.External != "" && (!filepath.IsAbs(e.External) || filepath.Clean(e.External) != e.External || !strings.HasPrefix(e.Repo, "found/") || e.Revision == "") {
		return false
	}
	for _, f := range e.Files {
		if !validName(f.Name) || f.Size <= 0 || (e.External == "" && !sha256Pattern.MatchString(f.SHA256)) || (e.External != "" && (f.SHA256 != "" || f.ModTime <= 0)) {
			return false
		}
		if f.Role == protocol.LocalFileModel {
			models++
			names = append(names, f.Name)
		} else if f.Role != protocol.LocalFileMmproj {
			return false
		}
	}
	if models == 1 {
		return splitPart(names[0]) == 0
	}
	return models > 1 && completeSplitNames(names)
}

// addLocal imports a user-selected GGUF as an owned copy. The content hash is
// calculated while copying; changing or removing the original cannot break
// the installed model. A manifest is written only after the copy is durable.
func (m *Manager) addLocal(ctx context.Context, source string) (protocol.LocalModel, error) {
	if !strings.EqualFold(filepath.Ext(source), ".gguf") {
		return protocol.LocalModel{}, protocol.Errorf(protocol.CodeBadRequest, "%s", i18n.T("Choose a local .gguf file."))
	}
	if strings.HasPrefix(strings.ToLower(filepath.Base(source)), "mmproj") {
		return protocol.LocalModel{}, protocol.Errorf(protocol.CodeBadRequest, "%s", i18n.T("Choose model weights, not a picture reader."))
	}
	names := []string{filepath.Base(source)}
	if splitPart(filepath.Base(source)) > 0 {
		part := splitGGUF.FindStringSubmatch(filepath.Base(source))
		entries, err := os.ReadDir(filepath.Dir(source))
		if err != nil {
			return protocol.LocalModel{}, err
		}
		names = names[:0]
		for _, entry := range entries {
			match := splitGGUF.FindStringSubmatch(entry.Name())
			if match != nil && match[1] == part[1] && match[3] == part[3] {
				names = append(names, entry.Name())
			}
		}
		if !completeSplitNames(names) {
			return protocol.LocalModel{}, protocol.Errorf(protocol.CodeBadRequest, "%s", i18n.T("The GGUF split set is incomplete."))
		}
		sort.Strings(names)
		source = filepath.Join(filepath.Dir(source), names[0])
	}
	var projector string
	if entries, err := os.ReadDir(filepath.Dir(source)); err == nil {
		var candidates []protocol.LocalFile
		for _, entry := range entries {
			if strings.HasPrefix(strings.ToLower(entry.Name()), "mmproj") && strings.EqualFold(filepath.Ext(entry.Name()), ".gguf") {
				candidates = append(candidates, protocol.LocalFile{Name: entry.Name(), Role: protocol.LocalFileMmproj})
			}
		}
		projector = matchingMmproj(candidates, names[0])
		if projector != "" {
			names = append(names, projector)
		}
	}
	input, err := os.Open(source)
	if err != nil {
		return protocol.LocalModel{}, err
	}
	defer input.Close()
	info, err := input.Stat()
	if err != nil {
		return protocol.LocalModel{}, err
	}
	if !info.Mode().IsRegular() || info.Size() <= 0 {
		return protocol.LocalModel{}, protocol.Errorf(protocol.CodeBadRequest, "%s", i18n.T("Choose a regular GGUF file."))
	}
	need := info.Size()
	for _, name := range names[1:] {
		part, err := os.Stat(filepath.Join(filepath.Dir(source), name))
		if err != nil {
			return protocol.LocalModel{}, err
		}
		if !part.Mode().IsRegular() || part.Size() <= 0 {
			return protocol.LocalModel{}, protocol.Errorf(protocol.CodeBadRequest, "%s", i18n.T("Choose regular GGUF files."))
		}
		need += part.Size()
	}
	if err := checkSpace(paths.ModelsDir(), need); err != nil {
		return protocol.LocalModel{}, err
	}
	if err := os.MkdirAll(filepath.Join(paths.ModelsDir(), "local"), 0o700); err != nil {
		return protocol.LocalModel{}, err
	}
	tmp, err := os.CreateTemp(filepath.Join(paths.ModelsDir(), "local"), ".import-*")
	if err != nil {
		return protocol.LocalModel{}, err
	}
	defer os.Remove(tmp.Name())
	defer tmp.Close()
	header, err := io.ReadAll(io.LimitReader(input, ggufHeaderLimit))
	if err != nil {
		return protocol.LocalModel{}, err
	}
	meta, err := parseGGUF(header)
	if err != nil {
		return protocol.LocalModel{}, protocol.Errorf(protocol.CodeBadRequest, "%s", i18n.T("This file has an unreadable GGUF header."))
	}
	if _, err := input.Seek(0, io.SeekStart); err != nil {
		return protocol.LocalModel{}, err
	}
	h := sha256.New()
	n, err := copyCtx(ctx, io.MultiWriter(tmp, h), input, func(int64) {})
	if err != nil {
		return protocol.LocalModel{}, err
	}
	after, err := input.Stat()
	if err != nil {
		return protocol.LocalModel{}, err
	}
	if n != info.Size() || after.Size() != info.Size() || !after.ModTime().Equal(info.ModTime()) {
		return protocol.LocalModel{}, protocol.Errorf(protocol.CodeConflict, "%s", i18n.T("The GGUF file changed during import."))
	}
	if err := tmp.Sync(); err != nil {
		return protocol.LocalModel{}, err
	}
	if err := tmp.Close(); err != nil {
		return protocol.LocalModel{}, err
	}
	sum := hex.EncodeToString(h.Sum(nil))
	staged := []string{tmp.Name()}
	defer func() {
		for _, name := range staged[1:] {
			_ = os.Remove(name)
		}
	}()
	sums := []string{sum}
	files := []File{{Name: filepath.Base(source), Role: protocol.LocalFileModel, Size: n, SHA256: sum}}
	meta.ModelBytes = n
	for _, name := range names[1:] {
		sourcePart, err := os.Open(filepath.Join(filepath.Dir(source), name))
		if err != nil {
			return protocol.LocalModel{}, err
		}
		info, err := sourcePart.Stat()
		if err != nil || !info.Mode().IsRegular() || info.Size() <= 0 {
			_ = sourcePart.Close()
			return protocol.LocalModel{}, protocol.Errorf(protocol.CodeBadRequest, "%s", i18n.T("Choose regular GGUF files."))
		}
		part, err := os.CreateTemp(filepath.Join(paths.ModelsDir(), "local"), ".import-*")
		if err != nil {
			_ = sourcePart.Close()
			return protocol.LocalModel{}, err
		}
		staged = append(staged, part.Name())
		otherHash := sha256.New()
		size, copyErr := copyCtx(ctx, io.MultiWriter(part, otherHash), sourcePart, func(int64) {})
		after, statErr := sourcePart.Stat()
		_ = sourcePart.Close()
		if copyErr != nil {
			_ = part.Close()
			return protocol.LocalModel{}, copyErr
		}
		if statErr != nil || size != info.Size() || after.Size() != info.Size() || !after.ModTime().Equal(info.ModTime()) {
			_ = part.Close()
			return protocol.LocalModel{}, protocol.Errorf(protocol.CodeConflict, "%s", i18n.T("The GGUF file changed during import."))
		}
		if err := part.Sync(); err != nil {
			_ = part.Close()
			return protocol.LocalModel{}, err
		}
		if err := part.Close(); err != nil {
			return protocol.LocalModel{}, err
		}
		digest := hex.EncodeToString(otherHash.Sum(nil))
		role := protocol.LocalFileModel
		if name == projector {
			role = protocol.LocalFileMmproj
			meta.MmprojBytes = size
		} else {
			meta.ModelBytes += size
		}
		files = append(files, File{Name: name, Role: role, Size: size, SHA256: digest})
		sums = append(sums, digest)
	}
	if len(sums) > 1 {
		group := sha256.Sum256([]byte(strings.Join(sums, "\x00")))
		sum = hex.EncodeToString(group[:])
	}
	id := "local-" + sum[:16]
	e := Entry{ID: id, Name: filepath.Base(source), Repo: "local/" + sum[:16], Revision: sum[:40],
		Files: files, Params: meta.Params, Context: meta.Context, Tools: meta.Tools, Class: protocol.LocalClassLight,
		Vision: projector != "", Audio: projector != "", Video: projector != ""}
	if e.Params > protocol.LightParams {
		e.Class = protocol.LocalClassMain
	}
	// ModelBytes includes every part, including those without a header.
	e.License.Name = i18n.T("Local file; check its licence before sharing")
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.closed {
		return protocol.LocalModel{}, protocol.Errorf(protocol.CodeUnavailable, "%s", i18n.T("Hyakki is shutting down."))
	}
	for _, existing := range m.entries() {
		if existing.ID != id {
			continue
		}
		current, _ := onDisk(existing)
		if current.State == protocol.LocalInstalled {
			return current, nil
		}
		if existing.Repo != e.Repo || len(existing.Files) != len(e.Files) {
			return protocol.LocalModel{}, protocol.Errorf(protocol.CodeConflict, "%s", i18n.T("This local model already exists with different files. Remove it before importing again."))
		}
		for i, f := range existing.Files {
			if f.Name != e.Files[i].Name || f.SHA256 != e.Files[i].SHA256 || f.Size != e.Files[i].Size {
				return protocol.LocalModel{}, protocol.Errorf(protocol.CodeConflict, "%s", i18n.T("This local model already exists with different files. Remove it before importing again."))
			}
		}
		for i, f := range existing.Files {
			if err := os.Rename(staged[i], filePath(existing, f.Name)); err != nil {
				return protocol.LocalModel{}, err
			}
			_ = os.Remove(filePath(existing, f.Name) + partSuffix)
		}
		syncDir(repoDir(existing.Repo))
		if err := writeManifest(existing, manifest{ID: existing.ID, Repo: existing.Repo, Revision: existing.Revision,
			Files: existing.Files, Verified: time.Now().UTC(), Header: &meta}); err != nil {
			return protocol.LocalModel{}, err
		}
		return m.modelOnDisk(existing), nil
	}
	if err := os.MkdirAll(repoDir(e.Repo), 0o700); err != nil {
		return protocol.LocalModel{}, err
	}
	for _, f := range e.Files {
		if _, err := os.Lstat(filePath(e, f.Name)); err == nil {
			return protocol.LocalModel{}, protocol.Errorf(protocol.CodeConflict, "%s", i18n.T("A model file already exists at the import destination."))
		} else if !errors.Is(err, fs.ErrNotExist) {
			return protocol.LocalModel{}, err
		}
	}
	var installed []string
	committed := false
	defer func() {
		if !committed {
			for _, name := range installed {
				_ = os.Remove(name)
			}
		}
	}()
	for i, f := range e.Files {
		dest := filePath(e, f.Name)
		if err := os.Rename(staged[i], dest); err != nil {
			return protocol.LocalModel{}, err
		}
		installed = append(installed, dest)
	}
	syncDir(repoDir(e.Repo))
	if err := writeManifest(e, manifest{ID: e.ID, Repo: e.Repo, Revision: e.Revision,
		Files: e.Files, Verified: time.Now().UTC(), Header: &meta}); err != nil {
		return protocol.LocalModel{}, err
	}
	custom := append(readCustom(), e)
	if err := writeCustom(custom); err != nil {
		_ = os.Remove(manifestPath(e))
		return protocol.LocalModel{}, err
	}
	committed = true
	return m.modelOnDisk(e), nil
}

// fileState is how much of one file is on disk: complete when the final
// file has its full size, else the bytes of its part file.
type fileState struct {
	done     int64
	complete bool
	part     bool
}

func statFile(e Entry, f File) fileState {
	if e.External != "" {
		info, err := os.Stat(filePath(e, f.Name))
		if err == nil && info.Mode().IsRegular() && info.Size() == f.Size && info.ModTime().UnixNano() == f.ModTime {
			return fileState{done: f.Size, complete: true}
		}
		return fileState{}
	}
	final := filePath(e, f.Name)
	if info, err := os.Stat(final); err == nil && info.Mode().IsRegular() && info.Size() == f.Size {
		return fileState{done: f.Size, complete: true}
	}
	if info, err := os.Stat(final + partSuffix); err == nil && info.Mode().IsRegular() {
		done := info.Size()
		if done > f.Size {
			done = 0
		}
		return fileState{done: done, part: true}
	}
	return fileState{}
}

// onDisk reads an entry's state from the disk alone: installed (or broken)
// with a manifest, paused with something downloaded, else available.
// adoptable is true when every file is complete but nothing verified them.
func onDisk(e Entry) (m protocol.LocalModel, adoptable bool) {
	m = e.info()
	complete, some := 0, false
	for i, f := range e.Files {
		st := statFile(e, f)
		m.Files[i].Done = st.done
		if st.complete {
			complete++
		}
		if st.done > 0 || st.part {
			some = true
		}
	}
	if e.External != "" {
		if file, ok := e.model(); ok {
			m.Path = filePath(e, file.Name)
		}
		if _, ok := readManifest(e); ok {
			if complete == len(e.Files) {
				m.State = protocol.LocalInstalled
			} else {
				m.State = protocol.LocalBroken
				m.Error = i18n.T("A linked model file was moved or changed. Find it again to use it.")
			}
		}
		return m, false
	}
	if man, ok := readManifest(e); ok {
		if complete == len(e.Files) {
			m.State = protocol.LocalInstalled
		} else {
			m.State = protocol.LocalBroken
		}
		if !e.Catalogue && man.Header == nil && m.State == protocol.LocalInstalled {
			header, _ := localGGUF(e)
			man.Header = &header
			_ = writeManifest(e, man)
		}
		if !e.Catalogue {
			if man.Header != nil {
				m.Params, m.Context, m.Tools = man.Header.Params, man.Header.Context, man.Header.Tools
				m.Warning = modelWarning(*man.Header)
			} else if m.State == protocol.LocalInstalled {
				m.Params, m.Context, m.Tools = 0, 0, false
				m.Warning = modelWarning(ggufMetadata{})
			}
			if m.Params > protocol.LightParams {
				m.Class = protocol.LocalClassMain
			}
		}
		if p := man.Probed; p != nil {
			if p.Context > 0 && (m.Context == 0 || p.Context < m.Context) {
				m.Context = p.Context
			}
			if !e.Catalogue {
				m.Tools = m.Tools && p.Tools
				m.Vision, m.Audio = p.Vision, p.Audio
				if m.Params == 0 && p.Params > 0 {
					m.Params = p.Params
				}
				m.Warning = modelWarning(ggufMetadata{Params: m.Params, Context: m.Context, Tools: m.Tools})
			}
		}
		return m, false
	}
	if complete == len(e.Files) {
		return m, true
	}
	if some {
		m.State = protocol.LocalPaused
	}
	return m, false
}

// removeFiles deletes an entry's files, parts and manifest, keeping a file
// another installed entry still uses.
func removeFiles(e Entry, others []Entry) error {
	if e.External != "" {
		err := os.Remove(manifestPath(e))
		if err != nil && !errors.Is(err, fs.ErrNotExist) {
			return err
		}
		_ = os.Remove(repoDir(e.Repo))
		return nil
	}
	keep := map[string]bool{}
	for _, o := range others {
		if o.ID == e.ID || o.Repo != e.Repo {
			continue
		}
		if _, ok := readManifest(o); !ok {
			continue
		}
		for _, f := range o.Files {
			keep[f.Name] = true
		}
	}
	var first error
	note := func(err error) {
		if err != nil && !errors.Is(err, fs.ErrNotExist) && first == nil {
			first = err
		}
	}
	note(os.Remove(manifestPath(e)))
	for _, f := range e.Files {
		note(os.Remove(filePath(e, f.Name) + partSuffix))
		if !keep[f.Name] {
			note(os.Remove(filePath(e, f.Name)))
		}
	}
	// an emptied repo folder goes too; LICENSE alone does not keep it
	dir := repoDir(e.Repo)
	if entries, err := os.ReadDir(dir); err == nil {
		rest := 0
		for _, entry := range entries {
			if entry.Name() != "LICENSE" {
				rest++
			}
		}
		if rest == 0 {
			_ = os.Remove(filepath.Join(dir, "LICENSE"))
			_ = os.Remove(dir)
			_ = os.Remove(filepath.Dir(dir))
		}
	}
	return first
}

// diskBytes is what the models folder holds.
func diskBytes() int64 {
	var total int64
	_ = filepath.WalkDir(paths.ModelsDir(), func(_ string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.Type().IsRegular() {
			if info, err := d.Info(); err == nil {
				total += info.Size()
			}
		}
		return nil
	})
	return total
}
