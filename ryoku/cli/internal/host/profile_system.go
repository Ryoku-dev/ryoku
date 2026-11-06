package host

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"syscall"

	"ryoku-cli/internal/profile"
	i18n "ryoku-i18n"
)

var profileRefreshHookPath = "/etc/pacman.d/hooks/profile-refresh.hook"

func (a *App) ensureProfileGroup(id string, receipt *profile.SystemReceipt) error {
	name := profile.GroupName(id)
	_, groups, err := loadAccountDB(a.cfg.PasswdPath, a.cfg.GroupPath)
	if err != nil {
		return err
	}
	receipt.Groups = appendUnique(receipt.Groups, name)
	if groups[name] == nil {
		receipt.GroupsCreated = appendUnique(receipt.GroupsCreated, name)
		if err := profile.WriteSystemReceipt(profileRootPath, receipt); err != nil {
			return err
		}
		if err := a.usersRun(nil, "groupadd", "--system", name); err != nil {
			return fmt.Errorf(i18n.T("Create the profile group: %v"), err)
		}
		return nil
	}
	return profile.WriteSystemReceipt(profileRootPath, receipt)
}

func (a *App) applyProfileSystem(manifest *profile.Manifest, receipt *profile.SystemReceipt) error {
	group := profile.GroupName(manifest.ID)
	if len(manifest.Capabilities) > 0 {
		if err := ensureCapabilityDirectories(manifest.ID, receipt); err != nil {
			return fmt.Errorf(i18n.T("Prepare profile capability directories: %v"), err)
		}
	}
	for _, capability := range manifest.Capabilities {
		source := filepath.Join("/usr/bin", capability.Tool)
		destination := filepath.Join(profile.ProfileBinDir(manifest.ID), capability.Tool)
		if _, err := os.Lstat(destination); err == nil && !systemFileClaimed(receipt.System, destination) {
			return fmt.Errorf(i18n.T("Capability path %s already exists and is not owned by this profile."), destination)
		} else if err != nil && !os.IsNotExist(err) {
			return err
		}
		receipt.System = replaceSystemFile(receipt.System, profile.SystemFile{Kind: "capability", Path: destination})
		if err := profile.WriteSystemReceipt(profileRootPath, receipt); err != nil {
			return err
		}
		if err := a.usersRun(nil, "install", "-D", "-m", "0750", "-g", group, "--", source, destination); err != nil {
			return fmt.Errorf(i18n.T("Install the profile copy of %s: %v"), capability.Tool, err)
		}
		digest, err := hashFile(destination)
		if err != nil {
			return err
		}
		receipt.System = replaceSystemFile(receipt.System, profile.SystemFile{Kind: "capability", Path: destination, SHA256: digest})
		if err := profile.WriteSystemReceipt(profileRootPath, receipt); err != nil {
			return err
		}
		if err := a.usersRun(nil, "setcap", capability.Caps, destination); err != nil {
			return fmt.Errorf(i18n.T("Set capabilities on %s: %v"), capability.Tool, err)
		}
	}
	if err := ensureWordlistsRoot(receipt); err != nil {
		return fmt.Errorf(i18n.T("Prepare wordlists: %v"), err)
	}
	for _, link := range manifest.Wordlists.Links {
		path := filepath.Join(profile.WordlistsRoot, link.Name)
		if _, err := os.Lstat(path); err == nil && !stringSliceContains(receipt.Wordlists, path) {
			return fmt.Errorf(i18n.T("Wordlist path %s already exists and is not owned by this profile."), path)
		} else if err != nil && !os.IsNotExist(err) {
			return err
		}
		if err := ensureWordlistParent(path, receipt); err != nil {
			return fmt.Errorf(i18n.T("Prepare wordlist directories for %s: %v"), link.Name, err)
		}
		receipt.Wordlists = appendUnique(receipt.Wordlists, path)
		receipt.System = replaceSystemFile(receipt.System, profile.SystemFile{Kind: "wordlists", Path: path})
		if err := profile.WriteSystemReceipt(profileRootPath, receipt); err != nil {
			return err
		}
		if err := replaceSymlink(path, link.Target); err != nil {
			return fmt.Errorf(i18n.T("Link wordlist %s: %v"), link.Name, err)
		}
	}
	for _, extract := range manifest.Wordlists.Extract {
		path := filepath.Join(profile.WordlistsRoot, extract.Name)
		if _, err := os.Lstat(path); err == nil && !stringSliceContains(receipt.Wordlists, path) {
			return fmt.Errorf(i18n.T("Wordlist path %s already exists and is not owned by this profile."), path)
		} else if err != nil && !os.IsNotExist(err) {
			return err
		}
		if err := ensureWordlistParent(path, receipt); err != nil {
			return fmt.Errorf(i18n.T("Prepare wordlist directories for %s: %v"), extract.Name, err)
		}
		receipt.Wordlists = appendUnique(receipt.Wordlists, path)
		receipt.System = replaceSystemFile(receipt.System, profile.SystemFile{Kind: "wordlists", Path: path})
		if err := profile.WriteSystemReceipt(profileRootPath, receipt); err != nil {
			return err
		}
		if err := a.extractWordlist(extract.Archive, extract.Member, path); err != nil {
			return fmt.Errorf(i18n.T("Extract wordlist %s: %v"), extract.Name, err)
		}
		digest, err := hashFile(path)
		if err != nil {
			return err
		}
		receipt.System = replaceSystemFile(receipt.System, profile.SystemFile{Kind: "wordlists", Path: path, SHA256: digest})
		if err := profile.WriteSystemReceipt(profileRootPath, receipt); err != nil {
			return err
		}
	}
	if len(manifest.Capabilities) > 0 || len(manifest.Wordlists.Extract) > 0 {
		receipt.System = replaceSystemFile(receipt.System, profile.SystemFile{Kind: "pacman-hook", Path: profileRefreshHookPath})
		if err := profile.WriteSystemReceipt(profileRootPath, receipt); err != nil {
			return err
		}
	}
	if len(manifest.Hosts.Suffixes) > 0 {
		hosts := manifest.Hosts
		receipt.Hosts = &hosts
		if err := profile.WriteSystemReceipt(profileRootPath, receipt); err != nil {
			return err
		}
	}
	return nil
}

func ensureWordlistsRoot(receipt *profile.SystemReceipt) error {
	info, err := os.Lstat(profile.WordlistsRoot)
	if err == nil {
		if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
			return fmt.Errorf("%s is not a directory", profile.WordlistsRoot)
		}
		return inheritRecordedDirectory(profile.WordlistsRoot, "wordlists-dir", receipt)
	}
	if !os.IsNotExist(err) {
		return err
	}
	if err := os.MkdirAll(profile.WordlistsRoot, 0o755); err != nil {
		return err
	}
	if err := os.Chmod(profile.WordlistsRoot, 0o755); err != nil {
		return err
	}
	receipt.System = replaceSystemFile(receipt.System, profile.SystemFile{Kind: "wordlists-dir", Path: profile.WordlistsRoot})
	return profile.WriteSystemReceipt(profileRootPath, receipt)
}

func ensureCapabilityDirectories(id string, receipt *profile.SystemReceipt) error {
	bin := profile.ProfileBinDir(id)
	paths := []string{filepath.Dir(filepath.Dir(bin)), filepath.Dir(bin), bin}
	for _, path := range paths {
		info, err := os.Lstat(path)
		if os.IsNotExist(err) {
			if err := os.Mkdir(path, 0o755); err != nil {
				return err
			}
			if err := os.Chmod(path, 0o755); err != nil {
				return err
			}
		} else if err != nil {
			return err
		} else if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
			return fmt.Errorf("%s is not a directory", path)
		}
		receipt.System = replaceSystemFile(receipt.System, profile.SystemFile{Kind: "capability-dir", Path: path})
	}
	return profile.WriteSystemReceipt(profileRootPath, receipt)
}

func ensureWordlistParent(destination string, receipt *profile.SystemReceipt) error {
	parent := filepath.Dir(destination)
	relative, err := filepath.Rel(profile.WordlistsRoot, parent)
	if err != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return fmt.Errorf("wordlist destination escapes %s", profile.WordlistsRoot)
	}
	if relative == "." {
		return nil
	}
	current := profile.WordlistsRoot
	for _, component := range strings.Split(relative, string(filepath.Separator)) {
		current = filepath.Join(current, component)
		info, statErr := os.Lstat(current)
		if statErr == nil {
			if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
				return fmt.Errorf("%s is not a directory", current)
			}
			if err := inheritRecordedDirectory(current, "wordlists-dir", receipt); err != nil {
				return err
			}
			continue
		}
		if !os.IsNotExist(statErr) {
			return statErr
		}
		if err := os.Mkdir(current, 0o755); err != nil {
			return err
		}
		if err := os.Chmod(current, 0o755); err != nil {
			return err
		}
		receipt.System = replaceSystemFile(receipt.System, profile.SystemFile{Kind: "wordlists-dir", Path: current})
		if err := profile.WriteSystemReceipt(profileRootPath, receipt); err != nil {
			return err
		}
	}
	return nil
}

func inheritRecordedDirectory(path, kind string, receipt *profile.SystemReceipt) error {
	if systemFileClaimed(receipt.System, path) {
		return nil
	}
	receipts, err := profile.ReadSystemReceipts(profileRootPath)
	if err != nil {
		return err
	}
	for _, other := range receipts {
		for _, file := range other.System {
			if file.Kind == kind && file.Path == path {
				receipt.System = replaceSystemFile(receipt.System, profile.SystemFile{Kind: kind, Path: path})
				return profile.WriteSystemReceipt(profileRootPath, receipt)
			}
		}
	}
	return nil
}

func replaceSymlink(path, target string) error {
	info, err := os.Lstat(path)
	if err == nil {
		if info.Mode()&os.ModeSymlink != 0 {
			current, readErr := os.Readlink(path)
			if readErr == nil && current == target {
				return nil
			}
		}
		if err := os.Remove(path); err != nil {
			return err
		}
	} else if !os.IsNotExist(err) {
		return err
	}
	return os.Symlink(target, path)
}

func (a *App) extractWordlist(archive, member, destination string) error {
	if err := os.MkdirAll(filepath.Dir(destination), 0o755); err != nil {
		return err
	}
	file, err := os.CreateTemp(filepath.Dir(destination), ".wordlist-*")
	if err != nil {
		return err
	}
	temporary := file.Name()
	defer os.Remove(temporary)
	result := a.cfg.Runner.Run(Command{Name: "tar", Args: []string{"-xOf", archive, member}, Stdout: file, Stderr: io.Discard})
	closeErr := file.Close()
	if result.Code != 0 {
		return fmt.Errorf("tar exited with status %d", result.Code)
	}
	if closeErr != nil {
		return closeErr
	}
	if err := os.Chmod(temporary, 0o644); err != nil {
		return err
	}
	return os.Rename(temporary, destination)
}

func (a *App) rewriteProfileRefreshHook(receipts []profile.SystemReceipt) error {
	targets := map[string]bool{}
	needsHook := false
	for _, receipt := range receipts {
		if receipt.Staged == "" {
			continue
		}
		manifest, err := profile.LoadManifest(receipt.Staged)
		if err != nil {
			return fmt.Errorf(i18n.T("Read staged profile %s: %v"), receipt.Profile.ID, err)
		}
		if len(manifest.Capabilities) > 0 || len(manifest.Wordlists.Extract) > 0 {
			needsHook = true
		}
		for _, capability := range manifest.Capabilities {
			if owner := a.packageOwnerName(filepath.Join("/usr/bin", capability.Tool)); owner != "" {
				targets[owner] = true
			}
		}
		for _, extract := range manifest.Wordlists.Extract {
			if owner := a.packageOwnerName(extract.Archive); owner != "" {
				targets[owner] = true
			}
		}
	}
	if !needsHook {
		if err := os.Remove(profileRefreshHookPath); err != nil && !os.IsNotExist(err) {
			return err
		}
		return nil
	}
	packages := make([]string, 0, len(targets))
	for name := range targets {
		packages = append(packages, name)
	}
	sort.Strings(packages)
	if len(packages) == 0 {
		return fmt.Errorf(i18n.T("No package owns a profile tool or wordlist archive."))
	}
	body := "[Trigger]\nOperation = Install\nOperation = Upgrade\nType = Package\nTarget = " + strings.Join(packages, "\nTarget = ") + "\n\n[Action]\nDescription = Refreshing profile tools and wordlists\nWhen = PostTransaction\nExec = /usr/bin/ryoku-host profile refresh\n"
	return writeRootFileAtomic(profileRefreshHookPath, []byte(body), 0o644)
}

func (a *App) packageOwnerName(path string) string {
	result := a.query("pacman", "-Qoq", "--", path)
	if result.Code != 0 {
		return ""
	}
	fields := strings.Fields(result.Output)
	if len(fields) == 0 {
		return ""
	}
	return fields[0]
}

func (a *App) pruneProfileSystem(manifest *profile.Manifest, receipt *profile.SystemReceipt) error {
	desired := map[string]bool{}
	for _, capability := range manifest.Capabilities {
		desired[filepath.Join(profile.ProfileBinDir(manifest.ID), capability.Tool)] = true
	}
	for _, link := range manifest.Wordlists.Links {
		desired[filepath.Join(profile.WordlistsRoot, link.Name)] = true
	}
	for _, extract := range manifest.Wordlists.Extract {
		desired[filepath.Join(profile.WordlistsRoot, extract.Name)] = true
	}
	for _, file := range receipt.System {
		if strings.HasSuffix(file.Kind, "-dir") && hasDesiredChild(file.Path, desired) {
			desired[file.Path] = true
		}
	}
	if len(manifest.Capabilities) > 0 || len(manifest.Wordlists.Extract) > 0 {
		desired[profileRefreshHookPath] = true
	}
	receipts, err := profile.ReadSystemReceipts(profileRootPath)
	if err != nil {
		return err
	}
	claimed := map[string]bool{}
	for _, other := range receipts {
		if other.Account.Name == receipt.Account.Name && other.Profile.ID == receipt.Profile.ID {
			continue
		}
		for _, file := range other.System {
			claimed[file.Path] = true
		}
		for _, path := range other.Wordlists {
			claimed[path] = true
		}
	}
	system := receipt.System[:0]
	remove := make([]profile.SystemFile, 0)
	for _, file := range receipt.System {
		if desired[file.Path] {
			system = append(system, file)
			continue
		}
		if !claimed[file.Path] {
			remove = append(remove, file)
		}
	}
	if err := removeRecordedSystemFiles(remove); err != nil {
		return err
	}
	receipt.System = system
	wordlists := receipt.Wordlists[:0]
	for _, path := range receipt.Wordlists {
		if desired[path] {
			wordlists = append(wordlists, path)
		}
	}
	receipt.Wordlists = wordlists
	if len(manifest.Hosts.Suffixes) == 0 {
		receipt.Hosts = nil
	}
	return nil
}

func hasDesiredChild(directory string, desired map[string]bool) bool {
	prefix := filepath.Clean(directory) + string(filepath.Separator)
	for path := range desired {
		if strings.HasPrefix(filepath.Clean(path), prefix) {
			return true
		}
	}
	return false
}

func removeRecordedSystemFiles(files []profile.SystemFile) error {
	sort.SliceStable(files, func(i, j int) bool {
		leftDepth := strings.Count(filepath.Clean(files[i].Path), string(filepath.Separator))
		rightDepth := strings.Count(filepath.Clean(files[j].Path), string(filepath.Separator))
		if leftDepth != rightDepth {
			return leftDepth > rightDepth
		}
		return !strings.HasSuffix(files[i].Kind, "-dir") && strings.HasSuffix(files[j].Kind, "-dir")
	})
	for _, file := range files {
		err := os.Remove(file.Path)
		if err == nil || os.IsNotExist(err) {
			continue
		}
		if strings.HasSuffix(file.Kind, "-dir") && (errors.Is(err, syscall.ENOTEMPTY) || errors.Is(err, syscall.EEXIST)) {
			continue
		}
		return err
	}
	return nil
}

func removeEmptyDirectory(path string) error {
	err := os.Remove(path)
	if err == nil || os.IsNotExist(err) || errors.Is(err, syscall.ENOTEMPTY) || errors.Is(err, syscall.EEXIST) {
		return nil
	}
	return err
}

func writeRootFileAtomic(path string, data []byte, mode os.FileMode) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	file, err := os.CreateTemp(filepath.Dir(path), ".profile-*")
	if err != nil {
		return err
	}
	temporary := file.Name()
	defer os.Remove(temporary)
	if err := file.Chmod(mode); err != nil {
		file.Close()
		return err
	}
	if _, err := file.Write(data); err != nil {
		file.Close()
		return err
	}
	if err := file.Sync(); err != nil {
		file.Close()
		return err
	}
	if err := file.Close(); err != nil {
		return err
	}
	return os.Rename(temporary, path)
}

func replaceSystemFile(files []profile.SystemFile, value profile.SystemFile) []profile.SystemFile {
	for index := range files {
		if files[index].Kind == value.Kind && files[index].Path == value.Path {
			files[index] = value
			return files
		}
	}
	return append(files, value)
}

func appendUnique(values []string, value string) []string {
	for _, current := range values {
		if current == value {
			return values
		}
	}
	return append(values, value)
}

func stringSliceContains(values []string, value string) bool {
	for _, current := range values {
		if current == value {
			return true
		}
	}
	return false
}

func systemFileClaimed(files []profile.SystemFile, path string) bool {
	for _, file := range files {
		if file.Path == path {
			return true
		}
	}
	return false
}
