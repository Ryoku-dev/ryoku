package skills

// Derived from oh-my-pi (MIT), packages/coding-agent/src/autolearn/managed-skills.ts@c7fff67

import (
	"context"
	"errors"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"unicode"

	i18n "ryoku-i18n"
	"ryoku-rashin/internal/memory/scan"
	"ryoku-rashin/internal/protocol"
	"ryoku-rashin/internal/redact"
)

// MaxManagedBytes caps a managed SKILL.md, frontmatter included, so what a
// model writes stays bounded.
const MaxManagedBytes = 64_000

// Managed skill actions, as manage_skill names them.
const (
	ActionCreate = "create"
	ActionUpdate = "update"
	ActionDelete = "delete"
)

// managedName is the one shape a managed skill's name and folder may have:
// a single path segment that cannot climb out of the managed folder.
var managedName = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{0,63}$`)

// Managed is the skills Rashin writes itself (feature autolearn), one folder
// per skill under its own directory, apart from the built-in sets. Every
// write stays inside that directory, and a built-in skill of the same name
// always wins: such a managed skill is shadowed and reaches no model.
type Managed struct {
	dir     string
	builtin *Library

	// mu runs mutations one at a time, so a create, update and delete of
	// the same skill in one batch of tool calls never interleave.
	mu sync.Mutex
}

// WithManaged attaches the managed skills under dir to the library, so
// Search and Get serve them as "managed/<name>" beside the built-in sets,
// and returns the store. It does no I/O.
func (l *Library) WithManaged(dir string) *Managed {
	m := &Managed{dir: dir, builtin: l}
	l.mu.Lock()
	l.managed = m
	l.mu.Unlock()
	return m
}

// Managed is the attached managed store, nil without one.
func (l *Library) Managed() *Managed {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.managed
}

// Off is the list of sets a skills.* call leaves out under settings: those
// the user turned off, the extra ones (a team's), and the managed skills
// while feature autolearn is off.
func Off(settings protocol.Settings, extra ...string) []string {
	off := append(slices.Clone(settings.Skills.Off), extra...)
	if !settings.Features[protocol.FeatureAutolearn] {
		off = append(off, protocol.ManagedSkillSet)
	}
	return off
}

// SkillName is raw as a managed skill's name: trimmed and lower-cased, then
// held to kebab-case of 1 to 64 characters.
func SkillName(raw string) (string, error) {
	name := strings.ToLower(strings.TrimSpace(raw))
	if !managedName.MatchString(name) {
		return "", protocol.Errorf(protocol.CodeBadRequest, "%s", i18n.Tf("%q is not a skill name: use lowercase letters, digits and hyphens, 1 to 64 characters, starting with a letter or digit.", raw))
	}
	return name, nil
}

// ManagedDescription neutralises a description a model wrote so it cannot
// break out of the skills listing in a system prompt: control and format
// characters, angle brackets and backticks go, runs of tildes shrink to one,
// and it is one line. Applied when written and when read.
func ManagedDescription(raw string) string {
	var b strings.Builder
	tildes := false
	for _, r := range raw {
		switch {
		case unicode.Is(unicode.Cc, r) || unicode.Is(unicode.Cf, r):
			r = ' '
		case r == '<' || r == '>' || r == '`':
			continue
		}
		if r == '~' {
			if tildes {
				continue
			}
			tildes = true
		} else {
			tildes = false
		}
		b.WriteRune(r)
	}
	return strings.Join(strings.Fields(b.String()), " ")
}

// Dir is the folder the managed skills lie in.
func (m *Managed) Dir() string { return m.dir }

// Claimed reports that a built-in skill has the name, so a managed one of
// that name would never reach a model.
func (m *Managed) Claimed(name string) bool {
	if m.builtin == nil {
		return false
	}
	for _, e := range m.builtin.entries() {
		if e.name == name {
			return true
		}
	}
	return false
}

// Exists reports a managed skill of that name.
func (m *Managed) Exists(name string) bool {
	info, err := os.Lstat(filepath.Join(m.dir, name, "SKILL.md"))
	return err == nil && info.Mode().IsRegular()
}

// Create writes a new managed skill and returns its SKILL.md. It refuses a
// name a built-in skill claims and one that already exists.
func (m *Managed) Create(name, description, body string) (string, error) {
	return m.write(ActionCreate, name, description, body)
}

// Update replaces an existing managed skill's description and body.
func (m *Managed) Update(name, description, body string) (string, error) {
	return m.write(ActionUpdate, name, description, body)
}

// Save creates the skill, or updates it when it exists.
func (m *Managed) Save(name, description, body string) (path string, created bool, err error) {
	safe, err := SkillName(name)
	if err != nil {
		return "", false, err
	}
	if m.Exists(safe) {
		path, err = m.Update(safe, description, body)
		return path, false, err
	}
	path, err = m.Create(safe, description, body)
	return path, err == nil, err
}

func (m *Managed) write(action, rawName, rawDescription, rawBody string) (string, error) {
	name, err := SkillName(rawName)
	if err != nil {
		return "", err
	}
	if _, unsafe := scan.Strict(rawDescription); unsafe {
		return "", protocol.Errorf(protocol.CodeBadRequest, "%s", i18n.T("The skill description contains unsafe instructions or secrets."))
	}
	if _, unsafe := scan.Strict(rawBody); unsafe {
		return "", protocol.Errorf(protocol.CodeBadRequest, "%s", i18n.T("The skill body contains unsafe instructions or secrets."))
	}
	description := ManagedDescription(redact.Strict(rawDescription))
	body := strings.TrimSpace(redact.Strict(rawBody))
	if description == "" {
		return "", protocol.Errorf(protocol.CodeBadRequest, "%s", i18n.Tf("The skill %s needs a description.", name))
	}
	if body == "" {
		return "", protocol.Errorf(protocol.CodeBadRequest, "%s", i18n.Tf("The skill %s needs a body.", name))
	}
	content := "---\nname: " + name + "\ndescription: " + strconv.Quote(description) + "\n---\n\n" + body + "\n"
	if len(content) > MaxManagedBytes {
		return "", protocol.Errorf(protocol.CodeBadRequest, "%s", i18n.Tf("The skill is %d bytes; the limit is %d. Shorten its body or description.", len(content), MaxManagedBytes))
	}
	if m.Claimed(name) {
		return "", protocol.Errorf(protocol.CodeConflict, "%s", i18n.Tf("A built-in skill is named %s, and a managed skill never overrides one. Choose another name.", name))
	}

	m.mu.Lock()
	defer m.mu.Unlock()
	if err := m.rootSafe(true); err != nil {
		return "", err
	}
	root, err := os.OpenRoot(m.dir)
	if err != nil {
		return "", err
	}
	defer root.Close()
	folder := name
	file := filepath.Join(m.dir, name, "SKILL.md")
	if info, err := root.Lstat(folder); err == nil {
		if info.Mode()&fs.ModeSymlink != 0 || !info.IsDir() {
			return "", refused(name)
		}
	} else if !errors.Is(err, fs.ErrNotExist) {
		return "", err
	}
	if action == ActionCreate {
		if err := root.Mkdir(folder, 0o700); err != nil && !errors.Is(err, fs.ErrExist) {
			return "", err
		}
		if info, err := root.Lstat(folder); err != nil || !info.IsDir() || info.Mode()&fs.ModeSymlink != 0 {
			return "", refused(name)
		}
		// O_EXCL makes the existence check and the create one step, and
		// refuses a SKILL.md that is a link.
		f, err := root.OpenFile(filepath.Join(folder, "SKILL.md"), os.O_WRONLY|os.O_CREATE|os.O_EXCL|syscall.O_NOFOLLOW, 0o600)
		if errors.Is(err, fs.ErrExist) {
			return "", protocol.Errorf(protocol.CodeConflict, "%s", i18n.Tf("The skill %s already exists; update it instead.", name))
		}
		if err != nil {
			return "", err
		}
		return file, finish(f, content)
	}

	info, err := root.Lstat(filepath.Join(folder, "SKILL.md"))
	if errors.Is(err, fs.ErrNotExist) {
		return "", protocol.Errorf(protocol.CodeNotFound, "%s", i18n.Tf("There is no managed skill %s; create it instead.", name))
	}
	if err != nil {
		return "", err
	}
	if err := plainFile(name, info); err != nil {
		return "", err
	}
	// The file is checked again through the open handle, so a swap after
	// the Lstat cannot turn the write into a link or a hard-linked file.
	f, err := root.OpenFile(filepath.Join(folder, "SKILL.md"), os.O_WRONLY|syscall.O_NOFOLLOW, 0)
	if errors.Is(err, syscall.ELOOP) {
		return "", refused(name)
	}
	if err != nil {
		return "", err
	}
	opened, err := f.Stat()
	if err == nil {
		err = plainFile(name, opened)
	}
	if err == nil {
		err = f.Truncate(0)
	}
	if err != nil {
		_ = f.Close()
		return "", err
	}
	return file, finish(f, content)
}

// finish writes content to a freshly opened or truncated file and closes it.
func finish(f *os.File, content string) error {
	_, err := io.WriteString(f, content)
	if err == nil {
		err = f.Sync()
	}
	if closeErr := f.Close(); err == nil {
		err = closeErr
	}
	return err
}

// plainFile refuses to overwrite anything but a regular file with one link:
// a second hard link may be a file the user wrote elsewhere.
func plainFile(name string, info fs.FileInfo) error {
	if !info.Mode().IsRegular() {
		return refused(name)
	}
	if stat, ok := info.Sys().(*syscall.Stat_t); ok && stat.Nlink > 1 {
		return protocol.Errorf(protocol.CodeConflict, "%s", i18n.Tf("The SKILL.md of %s has more than one hard link; Rashin does not overwrite it.", name))
	}
	return nil
}

// refused is the error for a skill whose folder or file is a link or no
// plain file: writing or deleting it could reach outside the managed folder.
func refused(name string) error {
	return protocol.Errorf(protocol.CodeConflict, "%s", i18n.Tf("The managed skill %s is a link or not a plain file; Rashin does not touch it.", name))
}

// rootSafe checks the managed folder before any path under it is used. A
// link is allowed when it leads to a folder (the user may keep the skills
// elsewhere); a broken link or one to a file has nowhere to write. create
// makes the folder when it is missing.
func (m *Managed) rootSafe(create bool) error {
	info, err := os.Lstat(m.dir)
	switch {
	case errors.Is(err, fs.ErrNotExist):
		if create {
			return os.MkdirAll(m.dir, 0o700)
		}
		return nil
	case err != nil:
		return err
	case info.Mode()&fs.ModeSymlink != 0:
		target, err := os.Stat(m.dir)
		if err != nil || !target.IsDir() {
			return protocol.Errorf(protocol.CodeConflict, "%s", i18n.Tf("The managed skills folder %s is a link that leads to no folder; Rashin does not write there.", m.dir))
		}
		return nil
	case !info.IsDir():
		return protocol.Errorf(protocol.CodeConflict, "%s", i18n.Tf("The managed skills folder %s is not a folder; Rashin does not write there.", m.dir))
	}
	return nil
}

// Remove deletes a managed skill's folder. A link is refused, so the delete
// never reaches what it leads to.
func (m *Managed) Remove(rawName string) error {
	name, err := SkillName(rawName)
	if err != nil {
		return err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if err := m.rootSafe(false); err != nil {
		return err
	}
	root, err := os.OpenRoot(m.dir)
	if errors.Is(err, fs.ErrNotExist) {
		return protocol.Errorf(protocol.CodeNotFound, "%s", i18n.Tf("There is no managed skill %s.", name))
	}
	if err != nil {
		return err
	}
	defer root.Close()
	info, err := root.Lstat(name)
	if errors.Is(err, fs.ErrNotExist) {
		return protocol.Errorf(protocol.CodeNotFound, "%s", i18n.Tf("There is no managed skill %s.", name))
	}
	if err != nil {
		return err
	}
	if info.Mode()&fs.ModeSymlink != 0 || !info.IsDir() {
		return refused(name)
	}
	return root.RemoveAll(name)
}

// Skills are the managed skills by name. A folder whose name is no skill
// name, a link, a SKILL.md that is not a plain file, is too large or has no
// description is no managed skill: it never reaches a prompt.
func (m *Managed) Skills() []protocol.ManagedSkill {
	if m.rootSafe(false) != nil {
		return nil
	}
	root, err := os.OpenRoot(m.dir)
	if err != nil {
		return nil
	}
	defer root.Close()
	entries, err := fs.ReadDir(root.FS(), ".")
	if err != nil {
		return nil
	}
	values := redact.Snapshot()
	out := []protocol.ManagedSkill{}
	for _, entry := range entries {
		name := entry.Name()
		if !managedName.MatchString(name) || !entry.IsDir() || entry.Type()&fs.ModeSymlink != 0 {
			continue
		}
		file := filepath.Join(m.dir, name, "SKILL.md")
		f, err := root.OpenFile(filepath.Join(name, "SKILL.md"), os.O_RDONLY|syscall.O_NOFOLLOW, 0)
		if err != nil {
			continue
		}
		info, statErr := f.Stat()
		if statErr != nil || !info.Mode().IsRegular() || info.Size() > MaxManagedBytes {
			_ = f.Close()
			continue
		}
		body, readErr := io.ReadAll(io.LimitReader(f, MaxManagedBytes+1))
		closeErr := f.Close()
		if readErr != nil || closeErr != nil || len(body) > MaxManagedBytes {
			continue
		}
		_, content, ok := strings.Cut(strings.ReplaceAll(string(body), "\r\n", "\n"), "\n---\n")
		if !ok || strings.TrimSpace(content) == "" {
			continue
		}
		meta := frontmatter(string(body))
		if authored := meta["name"]; authored != "" && authored != name {
			continue
		}
		if _, unsafe := scan.Strict(meta["description"]); unsafe {
			continue
		}
		if _, unsafe := scan.Strict(content); unsafe {
			continue
		}
		description := ManagedDescription(values.Strict(meta["description"]))
		if description == "" {
			continue
		}
		out = append(out, protocol.ManagedSkill{
			Name: name, Description: description, Path: file, Bytes: info.Size(),
			Updated: info.ModTime(), Shadowed: m.Claimed(name),
		})
	}
	return out
}

// List is core.ManagedSkills' listing for learning.skills.
func (m *Managed) List(context.Context) (protocol.LearningSkillsResult, error) {
	return protocol.LearningSkillsResult{Skills: m.Skills(), Root: m.dir}, nil
}

// Delete is core.ManagedSkills' learning.skill_delete: it removes one
// managed skill and lists those left.
func (m *Managed) Delete(ctx context.Context, name string) (protocol.LearningSkillsResult, error) {
	if err := m.Remove(name); err != nil {
		return protocol.LearningSkillsResult{}, err
	}
	return m.List(ctx)
}

// managedSkill is a managed skill as the skills methods show it.
func managedSkill(s protocol.ManagedSkill) protocol.Skill {
	return protocol.Skill{
		ID: protocol.ManagedSkillSet + "/" + s.Name, Name: s.Name, Set: protocol.ManagedSkillSet,
		Description: s.Description, Path: s.Path,
	}
}

// Visible are the managed skills a model may be given under off: none
// while the managed set is off, never a shadowed one.
func (m *Managed) Visible(off []string) []protocol.ManagedSkill {
	if m == nil || !on(protocol.ManagedSkillSet, off) {
		return nil
	}
	var out []protocol.ManagedSkill
	for _, s := range m.Skills() {
		if !s.Shadowed {
			out = append(out, s)
		}
	}
	return out
}
