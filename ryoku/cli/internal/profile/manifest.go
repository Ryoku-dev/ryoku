package profile

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	wm "ryoku-wm"
)

var profileIDPattern = regexp.MustCompile(`^[a-z][a-z0-9-]{1,30}$`)
var semverPattern = regexp.MustCompile(`^(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)(?:-[0-9A-Za-z-]+(?:\.[0-9A-Za-z-]+)*)?(?:\+[0-9A-Za-z-]+(?:\.[0-9A-Za-z-]+)*)?$`)
var packagePattern = regexp.MustCompile(`^[a-z0-9@._+][a-z0-9@._+-]*$`)
var detectPattern = regexp.MustCompile(`^[A-Za-z0-9._+-]+$`)
var envKeyPattern = regexp.MustCompile(`^[A-Z_][A-Z0-9_]*$`)

func LoadManifest(dir string) (*Manifest, error) {
	f, err := os.Open(filepath.Join(dir, "profile.json"))
	if err != nil {
		return nil, fmt.Errorf("profile.json: %w", err)
	}
	defer f.Close()
	var m Manifest
	dec := json.NewDecoder(f)
	dec.DisallowUnknownFields()
	if err := dec.Decode(&m); err != nil {
		return nil, fmt.Errorf("profile.json: %w", err)
	}
	var extra any
	if err := dec.Decode(&extra); err != io.EOF {
		return nil, fmt.Errorf("profile.json: expected one JSON object")
	}
	if err := m.Validate(dir); err != nil {
		return nil, err
	}
	return &m, nil
}

func (m *Manifest) Validate(dir string) error {
	if m.Schema != API {
		return fieldError("schema", "must be 1")
	}
	if !profileIDPattern.MatchString(m.ID) {
		return fieldError("id", "must match ^[a-z][a-z0-9-]{1,30}$")
	}
	if !validSemver(m.Version) {
		return fieldError("version", "must be semantic versioning")
	}
	if strings.TrimSpace(m.Name) == "" {
		return fieldError("name", "must not be empty")
	}
	if m.Requires.ProfileAPI != API {
		return fieldError("requires.profileApi", "must be 1")
	}
	for i, manager := range m.Requires.PackageManagers {
		if !contains([]string{"pacman", "dnf", "xbps"}, manager) {
			return fieldError(fmt.Sprintf("requires.packageManagers[%d]", i), "must be pacman, dnf, or xbps")
		}
	}
	knownCaps := map[string]bool{}
	for _, cap := range wm.All() {
		knownCaps[string(cap)] = true
	}
	for i, cap := range m.Requires.Capabilities {
		if !knownCaps[cap] {
			return fieldError(fmt.Sprintf("requires.capabilities[%d]", i), "is not a ryoku-wm capability")
		}
	}
	if m.Account.Role != "standard" {
		return fieldError("account.role", "must be standard")
	}
	if strings.TrimSpace(m.Account.SuggestedName) == "" {
		return fieldError("account.suggestedName", "must not be empty")
	}
	if strings.TrimSpace(m.Account.FullName) == "" {
		return fieldError("account.fullName", "must not be empty")
	}
	for i, permission := range m.Account.Permissions {
		if permission == "admin" || !contains([]string{"devices", "audio", "virtualization", "containers"}, permission) {
			return fieldError(fmt.Sprintf("account.permissions[%d]", i), "is not an allowed standard-user permission")
		}
	}
	for i, group := range m.Account.Groups {
		if !contains(AllowedGroups, group) {
			return fieldError(fmt.Sprintf("account.groups[%d]", i), "is not allowed")
		}
	}
	if !filepath.IsAbs(m.Account.Shell) {
		return fieldError("account.shell", "must be absolute")
	}
	for i, repo := range m.Repos {
		if !contains(AllowedRepos, repo) {
			return fieldError(fmt.Sprintf("repos[%d]", i), "is not allowed")
		}
	}
	components := map[string]Component{}
	binaries := map[string]bool{}
	for i, c := range m.Components {
		field := fmt.Sprintf("components[%d]", i)
		if c.Type != "package" {
			return fieldError(field+".type", "must be package")
		}
		if !packagePattern.MatchString(c.Name) {
			return fieldError(field+".name", "is not a valid pacman package name")
		}
		if _, exists := components[c.Name]; exists {
			return fieldError(field+".name", "must be unique")
		}
		components[c.Name] = c
		if strings.TrimSpace(c.Detect) == "" {
			return fieldError(field+".detect", "must not be empty")
		}
		if filepath.IsAbs(c.Detect) {
			if filepath.Clean(c.Detect) != c.Detect {
				return fieldError(field+".detect", "absolute path must be clean")
			}
		} else if !detectPattern.MatchString(c.Detect) {
			return fieldError(field+".detect", "must be a command name or absolute path")
		}
		binaries[c.Detect] = true
		binaries[filepath.Base(c.Detect)] = true
		if c.Tier != "core" && c.Tier != "optional" {
			return fieldError(field+".tier", "must be core or optional")
		}
		if strings.TrimSpace(c.Group) == "" {
			return fieldError(field+".group", "must not be empty")
		}
		if strings.TrimSpace(c.Summary) == "" {
			return fieldError(field+".summary", "must not be empty")
		}
		for j, id := range c.DesktopEntries {
			if !strings.HasSuffix(id, ".desktop") {
				return fieldError(fmt.Sprintf("%s.desktopEntries[%d]", field, j), "must end in .desktop")
			}
		}
	}
	for i, c := range m.Capabilities {
		want, ok := AllowedCapabilities[c.Tool]
		if !ok {
			return fieldError(fmt.Sprintf("capabilities[%d].tool", i), "is not allowed")
		}
		if c.Caps != want {
			return fieldError(fmt.Sprintf("capabilities[%d].caps", i), "does not match the allowed capability set")
		}
		if !binaries[c.Tool] {
			return fieldError(fmt.Sprintf("capabilities[%d].tool", i), "must be a component binary")
		}
	}
	for i, link := range m.Wordlists.Links {
		if err := validateRelativeName(link.Name); err != nil {
			return fieldError(fmt.Sprintf("wordlists.links[%d].name", i), err.Error())
		}
		if !underShare(link.Target) {
			return fieldError(fmt.Sprintf("wordlists.links[%d].target", i), "must be absolute under /usr/share")
		}
	}
	for i, extract := range m.Wordlists.Extract {
		if err := validateRelativeName(extract.Name); err != nil {
			return fieldError(fmt.Sprintf("wordlists.extract[%d].name", i), err.Error())
		}
		if !underShare(extract.Archive) {
			return fieldError(fmt.Sprintf("wordlists.extract[%d].archive", i), "must be absolute under /usr/share")
		}
		if extract.Member == "" || filepath.Base(extract.Member) != extract.Member || extract.Member == "." || extract.Member == ".." {
			return fieldError(fmt.Sprintf("wordlists.extract[%d].member", i), "must be a plain file name")
		}
	}
	for i, suffix := range m.Hosts.Suffixes {
		if !contains(AllowedHostSuffixes, suffix) {
			return fieldError(fmt.Sprintf("hosts.suffixes[%d]", i), "is not allowed")
		}
	}
	for i, env := range m.Env {
		field := fmt.Sprintf("env[%d]", i)
		if !envKeyPattern.MatchString(env.Key) {
			return fieldError(field+".key", "is invalid")
		}
		if env.Key == "PATH" || env.Key == "HOME" || env.Key == "USER" || env.Key == "XDG_RUNTIME_DIR" || strings.HasPrefix(env.Key, "LD_") {
			return fieldError(field+".key", "is reserved")
		}
		if strings.ContainsAny(env.Value, "\r\n") {
			return fieldError(field+".value", "must be one line")
		}
	}
	for i, entry := range m.Path {
		expanded := expandTokens(entry, Vars{Home: "/home/user", User: "user", ID: m.ID})
		if strings.Contains(entry, ":") || hasDotDot(entry) || !filepath.IsAbs(expanded) || strings.Contains(expanded, "@") {
			return fieldError(fmt.Sprintf("path[%d]", i), "must expand to an absolute path without ':' or '..'")
		}
	}
	homeDir := filepath.Join(dir, "home")
	for _, set := range []struct {
		name  string
		paths []string
	}{{"expand", m.Expand}, {"seedOnce", m.SeedOnce}} {
		for i, rel := range set.paths {
			if err := safeRelative(rel); err != nil {
				return fieldError(fmt.Sprintf("%s[%d]", set.name, i), err.Error())
			}
			st, err := os.Stat(filepath.Join(homeDir, filepath.FromSlash(rel)))
			if err != nil || !st.Mode().IsRegular() {
				return fieldError(fmt.Sprintf("%s[%d]", set.name, i), "must name an existing file under home")
			}
		}
	}
	for i, rel := range m.UserData {
		if err := safeRelative(rel); err != nil {
			return fieldError(fmt.Sprintf("userData[%d]", i), err.Error())
		}
		if st, err := os.Stat(filepath.Join(homeDir, filepath.FromSlash(rel))); err == nil && !st.IsDir() {
			return fieldError(fmt.Sprintf("userData[%d]", i), "must name a directory")
		}
	}
	if m.Rice == "" || strings.HasPrefix(m.Rice, ".") || strings.ContainsAny(m.Rice, `/\`) {
		return fieldError("rice", "must be a valid rice slug")
	}
	if st, err := os.Stat(filepath.Join(homeDir, ".config", "ryoku", "rices", m.Rice, "rice.json")); err != nil || !st.Mode().IsRegular() {
		return fieldError("rice", "must name home/.config/ryoku/rices/<rice>/rice.json")
	}
	if len(m.FirstRun.Command) == 0 || strings.TrimSpace(m.FirstRun.Command[0]) == "" {
		return fieldError("firstRun.command", "must contain an executable")
	}
	for i, arg := range m.FirstRun.Command {
		if strings.ContainsRune(arg, '\x00') {
			return fieldError(fmt.Sprintf("firstRun.command[%d]", i), "contains NUL")
		}
	}
	if strings.ContainsAny(m.FirstRun.Message, "\r\n") {
		return fieldError("firstRun.message", "must be one line")
	}
	return nil
}

func fieldError(field, detail string) error { return fmt.Errorf("%s: %s", field, detail) }
func contains(values []string, value string) bool {
	for _, item := range values {
		if item == value {
			return true
		}
	}
	return false
}
func validSemver(value string) bool {
	if !semverPattern.MatchString(value) {
		return false
	}
	coreAndPre := strings.SplitN(value, "+", 2)[0]
	_, pre, hasPre := strings.Cut(coreAndPre, "-")
	if !hasPre {
		return true
	}
	for _, identifier := range strings.Split(pre, ".") {
		numeric := true
		for _, r := range identifier {
			if r < '0' || r > '9' {
				numeric = false
				break
			}
		}
		if numeric && len(identifier) > 1 && identifier[0] == '0' {
			return false
		}
	}
	return true
}
func underShare(path string) bool {
	return filepath.IsAbs(path) && (path == "/usr/share" || strings.HasPrefix(filepath.Clean(path), "/usr/share/"))
}
func hasDotDot(path string) bool {
	for _, part := range strings.Split(filepath.ToSlash(path), "/") {
		if part == ".." {
			return true
		}
	}
	return false
}
func safeRelative(path string) error {
	if path == "" || strings.ContainsRune(path, '\x00') || filepath.IsAbs(path) || hasDotDot(path) || filepath.Clean(path) == "." {
		return fmt.Errorf("must be a relative path without '..'")
	}
	return nil
}
func validateRelativeName(path string) error { return safeRelative(path) }
