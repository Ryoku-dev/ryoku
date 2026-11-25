package migration

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"go.etcd.io/bbolt"

	"ryoku-rashin/internal/paths"
)

// RebaseReferences updates only identified filesystem-reference fields in moved
// metadata. The caller owns the rollback snapshot and must restore it on error.
// Every replacement is staged before any is installed, so malformed metadata
// cannot leave an otherwise valid file half-converted.
//
// Inventory: config providers.models.{binary,folders}; models/custom.json
// external directories (model manifests contain relative filenames and hashes);
// sessions/*/meta.json cwd/worktree/queued attachments; events.jsonl user image,
// terminal cwd and tool diff/location pointers (never tool prose);
// app.json last/teams worktree; indexes.json roots; projects.json roots, worktree
// settings and owned worktrees; collections/*.json saved/source/image paths;
// teams/state.json folders/presentation files and tasks/*.json report files;
// cache/index/*/index.db files.source (all record paths/keys are root-relative);
// memory/memory.db files bucket path keys; mcp-servers.json info command and
// exact absolute argv elements; Hindsight pg0 instance.json directory fields.
// The app-owned Hindsight installed marker is invalidated, not its Python code.
// Known Firefox/Zen INI, download preferences and addon metadata, and Chromium
// download preferences are handled below; only proven-rebuildable browser caches
// are invalidated. Other third-party profiles are retained, not claimed portable.
//
// Deliberately untouched: engine.jsonl embeds image bytes and signed reasoning;
// tool input/output and activity/ledger JSONL are historical user/model text;
// rules/skills/decision Markdown, decision queues, prompts.json, voice.json and
// words.txt are authored content;
// runtime.json uses a relative build directory; generated llama presets are
// regenerated on start. gateway.db, vault.db, grievances.db and judge.db have no
// relocatable filesystem references; memory's other buckets contain facts,
// scope IDs, history and vectors. Ciphertext, IDs, signatures, hashes, vectors
// and third-party browser databases must never be rewritten.
func RebaseReferences(moves []Move) error {
	var pending []referenceReplacement
	defer func() {
		for _, replacement := range pending {
			_ = os.Remove(replacement.temporary)
		}
	}()
	seen := make(map[string]bool)
	for _, move := range moves {
		if !filepath.IsAbs(move.Source) || filepath.Clean(move.Source) != move.Source || !filepath.IsAbs(move.Destination) || filepath.Clean(move.Destination) != move.Destination || move.Source == move.Destination {
			return fmt.Errorf("unsafe reference relocation %q -> %q", move.Source, move.Destination)
		}
		err := filepath.WalkDir(move.Destination, func(name string, entry fs.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			if entry.IsDir() {
				if skipReferenceDirectory(name) {
					return filepath.SkipDir
				}
				return nil
			}
			if seen[name] {
				return nil
			}
			seen[name] = true
			fields, kind := referenceSchema(name)
			if kind == "" {
				return nil
			}
			info, err := entry.Info()
			if err != nil {
				return err
			}
			if !info.Mode().IsRegular() {
				return fmt.Errorf("unsafe migration metadata %s: not a regular file", name)
			}
			var temporary string
			if kind == "bolt" || kind == "bolt-memory" {
				temporary, err = stageReferenceDatabase(name, info.Mode(), moves, kind)
			} else {
				var before, after []byte
				before, err = os.ReadFile(name)
				if err == nil {
					switch kind {
					case "jsonl":
						after, err = rebaseEventLines(before, moves)
					case "ini":
						after, err = rebaseProfileINI(name, before, moves)
					case "firefox-prefs":
						after, err = rebaseFirefoxPreferences(before, moves)
					case "firefox-addons":
						after, err = rebaseFirefoxAddons(before, moves)
					case "mcp":
						after, err = rebaseMCPReferences(before, moves)
					case "hindsight-install":
						// Python venv console scripts have absolute shebangs.
						// Installed() trusts this app-owned marker; removing it
						// requests the existing explicit Setup flow, which
						// rebuilds venv without rewriting third-party scripts.
						// No install/process/network action runs in migration.
						pending = append(pending, referenceReplacement{path: name})
						after = before
					case "firefox-startup":
						// XPIStates.loadExtensionState/scanForChanges rebuild
						// missing startup data by scanning all addon locations:
						// https://searchfox.org/mozilla-central/source/toolkit/mozapps/extensions/internal/XPIProvider.sys.mjs
						// ExtensionParent.StartupCache._readData similarly
						// recreates missing startupCache/webext.sc.lz4 lazily:
						// https://searchfox.org/mozilla-central/source/toolkit/components/extensions/ExtensionParent.sys.mjs
						// The enclosing move's full backup retains this cache.
						pending = append(pending, referenceReplacement{path: name})
						after = before
					case "decision":
						after, err = rebaseJSON(before, []string{"project"}, moves)
						if err == nil && !bytes.Equal(before, after) {
							err = errors.New("decision bank project moved: its path-derived bank ID requires an explicit bank migration")
						}
					default:
						after, err = rebaseJSON(before, fields, moves)
					}
				}
				if err == nil && !bytes.Equal(before, after) {
					temporary, err = stageReferenceFile(name, after, info.Mode())
				}
			}
			if err != nil {
				return fmt.Errorf("rebase %s: %w", name, err)
			}
			if temporary != "" {
				pending = append(pending, referenceReplacement{path: name, temporary: temporary})
			}
			return nil
		})
		if err != nil {
			return err
		}
	}
	for _, replacement := range pending {
		if replacement.temporary == "" {
			if err := os.Remove(replacement.path); err != nil {
				return err
			}
		} else if err := os.Rename(replacement.temporary, replacement.path); err != nil {
			return err
		}
		parent, err := os.Open(filepath.Dir(replacement.path))
		if err != nil {
			return err
		}
		err = parent.Sync()
		closeErr := parent.Close()
		if err != nil {
			return err
		}
		if closeErr != nil {
			return closeErr
		}
	}
	return nil
}

type referenceReplacement struct{ path, temporary string }

func skipReferenceDirectory(name string) bool {
	if name == filepath.Join(paths.DataDir(), "hindsight", "venv") || name == filepath.Join(paths.DataDir(), "hindsight", "python") {
		return true
	}
	rel, err := filepath.Rel(paths.StateDir(), name)
	if err == nil {
		switch strings.Split(rel, string(filepath.Separator))[0] {
		case "tool-homes", "harness-homes", "checkpoints":
			return true
		}
	}
	return false
}

func referenceSchema(name string) ([]string, string) {
	if name == paths.ConfigFile() {
		return []string{"providers.models.binary", "providers.models.folders.*"}, "json"
	}
	if name == paths.AppState() {
		return []string{"last.worktree", "teams.worktree"}, "json"
	}
	if name == paths.IndexRegistry() {
		return []string{"indexes.*.roots.*"}, "json"
	}
	if name == paths.Projects() {
		return []string{"projects.*.root", "projects.*.settings.worktreeRoot", "projects.*.worktrees.*.path", "projects.*.worktrees.*.parent", "projects.*.worktrees.*.setup.path", "projects.*.worktrees.*.setup.script", "projects.*.tasks.*.worktree", "projects.*.comments.*.comment.worktree"}, "json"
	}
	rel, _ := filepath.Rel(paths.SessionsDir(), name)
	parts := strings.Split(filepath.ToSlash(rel), "/")
	if len(parts) == 2 && parts[0] != ".." {
		switch parts[1] {
		case "meta.json":
			return []string{"cwd", "worktree", "queue.*.images.*.path"}, "json"
		case "events.jsonl":
			return nil, "jsonl"
		}
	}
	if name == filepath.Join(paths.ModelsDir(), "custom.json") {
		return []string{"*.external"}, "json"
	}
	if name == filepath.Join(paths.DataDir(), "mcp-servers.json") {
		return nil, "mcp"
	}
	if name == filepath.Join(paths.StateDir(), "memory", "memory.db") {
		return nil, "bolt-memory"
	}
	if name == filepath.Join(paths.DataDir(), "hindsight", "installed") {
		return nil, "hindsight-install"
	}
	// pg0 0.16.1 InstanceInfo stores these two paths separately from
	// credentials and the untouched PostgreSQL cluster itself.
	// https://github.com/vectorize-io/pg0/blob/v0.16.1/src/main.rs
	if name == filepath.Join(paths.StateDir(), "hindsight", "home", ".pg0", "instances", "rashin", "instance.json") {
		return []string{"data_dir", "installation_dir"}, "json"
	}
	rel, _ = filepath.Rel(paths.DecisionsDir(), name)
	parts = strings.Split(filepath.ToSlash(rel), "/")
	if len(parts) == 2 && parts[0] != ".." && parts[1] == "state.json" {
		return nil, "decision"
	}
	rel, _ = filepath.Rel(filepath.Join(paths.DataDir(), "collections"), name)
	if filepath.Dir(rel) == "." && strings.HasSuffix(rel, ".json") {
		return []string{"items.*.path", "items.*.image", "items.*.source"}, "json"
	}
	if name == filepath.Join(paths.TeamsDir(), "state.json") {
		return []string{"teams.*.folder", "teams.*.dream.presentation.files.*", "approvals.*.proposal.team.folder"}, "json"
	}
	rel, _ = filepath.Rel(filepath.Join(paths.TeamsDir(), "tasks"), name)
	if filepath.Dir(rel) == "." && strings.HasSuffix(rel, ".json") {
		return []string{"report.files.*"}, "json"
	}
	rel, _ = filepath.Rel(filepath.Join(paths.CacheDir(), "index"), name)
	parts = strings.Split(filepath.ToSlash(rel), "/")
	if len(parts) == 2 && parts[0] != ".." && parts[1] == "index.db" {
		return nil, "bolt"
	}
	rel, _ = filepath.Rel(paths.ScreenHome(), name)
	firefox := false
	for _, prefix := range []string{".mozilla/firefox/", ".mozilla/zen/", ".zen/", ".config/zen/"} {
		firefox = firefox || strings.HasPrefix(filepath.ToSlash(rel), prefix)
	}
	if firefox {
		switch filepath.Base(name) {
		case "profiles.ini", "installs.ini":
			return nil, "ini"
		case "prefs.js", "user.js":
			return nil, "firefox-prefs"
		case "extensions.json":
			return nil, "firefox-addons"
		case "addonStartup.json.lz4":
			return nil, "firefox-startup"
		}
	}
	for _, prefix := range []string{".mozilla/firefox/", ".mozilla/zen/", ".zen/", ".config/zen/", ".cache/mozilla/firefox/", ".cache/zen/"} {
		if strings.HasPrefix(filepath.ToSlash(rel), prefix) && filepath.Base(name) == "webext.sc.lz4" && filepath.Base(filepath.Dir(name)) == "startupCache" {
			return nil, "firefox-startup"
		}
	}
	for _, prefix := range []string{".config/chromium/", ".config/google-chrome/", ".config/google-chrome-beta/", ".config/google-chrome-unstable/", ".config/BraveSoftware/Brave-Browser/", ".config/microsoft-edge/", ".config/vivaldi/", ".config/opera/"} {
		if strings.HasPrefix(filepath.ToSlash(rel), prefix) && filepath.Base(name) == "Preferences" {
			return []string{"download.default_directory", "savefile.default_directory"}, "json"
		}
	}
	return nil, ""
}

// RawMessage keeps unknown fields, exact integer values and opaque values out of
// the transform. A selector names a field in an inventoried schema, never an
// arbitrary key encountered recursively in user data.
func rebaseJSON(data []byte, fields []string, moves []Move) ([]byte, error) {
	trimmed := bytes.TrimSpace(data)
	root := byte('{')
	if len(fields) > 0 && strings.HasPrefix(fields[0], "*.") {
		root = '['
	}
	if len(trimmed) == 0 || trimmed[0] != root {
		return nil, errors.New("metadata has the wrong JSON root type")
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.UseNumber()
	if err := validateReferenceJSON(decoder); err != nil {
		return nil, err
	}
	if _, err := decoder.Token(); !errors.Is(err, io.EOF) {
		return nil, errors.New("trailing JSON data")
	}
	out := data
	for _, field := range fields {
		var err error
		out, err = rebaseJSONField(out, strings.Split(field, "."), moves)
		if err != nil {
			return nil, fmt.Errorf("field %s: %w", field, err)
		}
	}
	return out, nil
}

func validateReferenceJSON(decoder *json.Decoder) error {
	token, err := decoder.Token()
	if err != nil {
		return err
	}
	delim, compound := token.(json.Delim)
	if !compound {
		return nil
	}
	if delim != '{' && delim != '[' {
		return errors.New("unexpected JSON delimiter")
	}
	keys := make(map[string]bool)
	for decoder.More() {
		if delim == '{' {
			key, err := decoder.Token()
			if err != nil {
				return err
			}
			name, ok := key.(string)
			if !ok || keys[name] {
				return fmt.Errorf("duplicate or invalid JSON key %v", key)
			}
			keys[name] = true
		}
		if err := validateReferenceJSON(decoder); err != nil {
			return err
		}
	}
	_, err = decoder.Token()
	return err
}

func rebaseJSONField(data []byte, field []string, moves []Move) ([]byte, error) {
	if bytes.Equal(bytes.TrimSpace(data), []byte("null")) {
		return data, nil
	}
	if len(field) == 0 {
		var path string
		if err := json.Unmarshal(data, &path); err != nil {
			return nil, errors.New("filesystem reference must be a string")
		}
		if strings.ContainsRune(path, '\x00') {
			return nil, errors.New("filesystem reference contains NUL")
		}
		if filepath.IsAbs(path) && filepath.Clean(path) != path {
			return nil, fmt.Errorf("unclean absolute filesystem reference %q", path)
		}
		if next := RebasePath(path, moves); next != path {
			return json.Marshal(next)
		}
		return data, nil
	}
	if field[0] == "*" {
		var entries []json.RawMessage
		if err := json.Unmarshal(data, &entries); err != nil {
			return nil, err
		}
		changed := false
		for i, entry := range entries {
			next, err := rebaseJSONField(entry, field[1:], moves)
			if err != nil {
				return nil, err
			}
			changed = changed || !bytes.Equal(entry, next)
			entries[i] = next
		}
		if changed {
			return json.Marshal(entries)
		}
		return data, nil
	}
	var object map[string]json.RawMessage
	if err := json.Unmarshal(data, &object); err != nil {
		return nil, err
	}
	value, exists := object[field[0]]
	if !exists {
		return data, nil
	}
	next, err := rebaseJSONField(value, field[1:], moves)
	if err != nil {
		return nil, err
	}
	if bytes.Equal(value, next) {
		return data, nil
	}
	object[field[0]] = next
	return json.Marshal(object)
}

func rebaseEventLines(data []byte, moves []Move) ([]byte, error) {
	lines := bytes.SplitAfter(data, []byte{'\n'})
	for i, line := range lines {
		if len(bytes.TrimSpace(line)) == 0 {
			continue
		}
		if _, err := rebaseJSON(line, nil, moves); err != nil {
			return nil, fmt.Errorf("event line %d: %w", i+1, err)
		}
		var event struct {
			Kind string `json:"kind"`
		}
		if err := json.Unmarshal(line, &event); err != nil {
			return nil, err
		}
		var fields []string
		switch event.Kind {
		case "user":
			fields = []string{"data.images.*.path", "data.terminal.cwd"}
		case "tool":
			fields = []string{"data.diff.path", "data.locations.*.path"}
		default:
			continue
		}
		next, err := rebaseJSON(line, fields, moves)
		if err != nil {
			return nil, fmt.Errorf("event line %d: %w", i+1, err)
		}
		if !bytes.Equal(next, line) && bytes.HasSuffix(line, []byte{'\n'}) {
			next = append(next, '\n')
		}
		lines[i] = next
	}
	return bytes.Join(lines, nil), nil
}

func stageReferenceFile(name string, data []byte, mode fs.FileMode) (string, error) {
	file, err := os.CreateTemp(filepath.Dir(name), ".hyakki-references-*")
	if err != nil {
		return "", err
	}
	temporary := file.Name()
	ok := false
	defer func() {
		_ = file.Close()
		if !ok {
			_ = os.Remove(temporary)
		}
	}()
	if err := file.Chmod(mode); err != nil {
		return "", err
	}
	if _, err := file.Write(data); err != nil {
		return "", err
	}
	if err := file.Sync(); err != nil {
		return "", err
	}
	if err := file.Close(); err != nil {
		return "", err
	}
	ok = true
	return temporary, nil
}

func stageReferenceDatabase(name string, mode fs.FileMode, moves []Move, kind string) (temporary string, err error) {
	// The index store itself treats bbolt panics as corruption. Preserve that
	// contract here, but return the failure for full migration rollback rather
	// than deleting/rebuilding a database during this transaction.
	defer func() {
		if value := recover(); value != nil {
			temporary = ""
			err = fmt.Errorf("damaged migration database: %v", value)
		}
	}()
	return stageReferenceDatabaseChecked(name, mode, moves, kind)
}

func stageReferenceDatabaseChecked(name string, mode fs.FileMode, moves []Move, kind string) (string, error) {
	db, err := bbolt.Open(name, mode.Perm(), &bbolt.Options{ReadOnly: true, Timeout: time.Second})
	if err != nil {
		return "", err
	}
	defer db.Close()
	var changes = make(map[string][]byte)
	newKeys := make(map[string]string)
	destinations := make(map[string]bool)
	err = db.View(func(tx *bbolt.Tx) error {
		var checkErr error
		for err := range tx.Check() {
			if checkErr == nil {
				checkErr = err
			}
		}
		if checkErr != nil {
			return checkErr
		}
		bucket := tx.Bucket([]byte("files"))
		if bucket == nil {
			return errors.New("database files bucket missing")
		}
		return bucket.ForEach(func(key, value []byte) error {
			if value == nil {
				return errors.New("unexpected nested files bucket")
			}
			if kind == "bolt-memory" {
				if _, err := rebaseJSON(value, nil, moves); err != nil {
					return err
				}
				path := string(key)
				if !filepath.IsAbs(path) || filepath.Clean(path) != path || strings.ContainsRune(path, '\x00') {
					return errors.New("unsafe memory file stamp path")
				}
				if next := RebasePath(path, moves); next != path {
					if destinations[next] || bucket.Get([]byte(next)) != nil || bucket.Bucket([]byte(next)) != nil {
						return fmt.Errorf("memory file stamp destination already exists: %s", next)
					}
					destinations[next] = true
					changes[path] = append([]byte(nil), value...)
					newKeys[path] = next
				}
				return nil
			}
			next, err := rebaseJSON(value, []string{"source"}, moves)
			if err != nil {
				return err
			}
			// Path and its keys are relative; changing them would invalidate
			// content-derived IDs, so refuse a nonconforming cache instead.
			var record struct {
				Path string `json:"path"`
			}
			if err := json.Unmarshal(value, &record); err != nil {
				return err
			}
			if RebasePath(record.Path, moves) != record.Path {
				return errors.New("index contains absolute record paths; rebuild with its original application before migrating")
			}
			if !bytes.Equal(next, value) {
				changes[string(key)] = next
			}
			return nil
		})
	})
	if err != nil || len(changes) == 0 {
		closeErr := db.Close()
		if err != nil {
			return "", err
		}
		return "", closeErr
	}
	temporary, err := stageReferenceFile(name, nil, mode)
	if err != nil {
		_ = db.Close()
		return "", err
	}
	ok := false
	defer func() {
		if !ok {
			_ = os.Remove(temporary)
		}
	}()
	err = db.View(func(tx *bbolt.Tx) error { return tx.CopyFile(temporary, mode.Perm()) })
	closeErr := db.Close()
	if err != nil {
		return "", err
	}
	if closeErr != nil {
		return "", closeErr
	}
	copyDB, err := bbolt.Open(temporary, mode.Perm(), &bbolt.Options{Timeout: time.Second})
	if err != nil {
		return "", err
	}
	defer copyDB.Close()
	err = copyDB.Update(func(tx *bbolt.Tx) error {
		bucket := tx.Bucket([]byte("files"))
		for key, value := range changes {
			destination := key
			if next, exists := newKeys[key]; exists {
				destination = next
				if err := bucket.Delete([]byte(key)); err != nil {
					return err
				}
			}
			if err := bucket.Put([]byte(destination), value); err != nil {
				return err
			}
		}
		return nil
	})
	closeErr = copyDB.Close()
	if err != nil {
		return "", err
	}
	if closeErr != nil {
		return "", closeErr
	}
	ok = true
	return temporary, nil
}

// Firefox profile registries are INI, not JSON. Only absolute Path/Default
// pointers change; relative profiles, section IDs and all other lines survive.
func rebaseProfileINI(registry string, data []byte, moves []Move) ([]byte, error) {
	lines := strings.SplitAfter(string(data), "\n")
	section := ""
	seen := make(map[string]bool)
	reverse := make([]Move, len(moves))
	for i, move := range moves {
		reverse[i] = Move{Source: move.Destination, Destination: move.Source}
	}
	originalRegistry := RebasePath(registry, reverse)
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "[") && strings.HasSuffix(trimmed, "]") {
			section = strings.TrimSuffix(strings.TrimPrefix(trimmed, "["), "]")
			continue
		}
		key, value, found := strings.Cut(trimmed, "=")
		key = strings.TrimSpace(key)
		if !found || strings.HasPrefix(key, ";") || strings.HasPrefix(key, "#") {
			continue
		}
		if key != "Path" && key != "Default" {
			continue
		}
		identity := section + "\x00" + key
		if seen[identity] {
			return nil, fmt.Errorf("duplicate Firefox profile pointer %s/%s", section, key)
		}
		seen[identity] = true
		value = strings.TrimSpace(value)
		if strings.ContainsRune(value, '\x00') || (filepath.IsAbs(value) && filepath.Clean(value) != value) {
			return nil, errors.New("unsafe Firefox profile pointer")
		}
		if value == "" {
			return nil, errors.New("empty Firefox profile pointer")
		}
		if !filepath.IsAbs(value) {
			oldTarget := filepath.Clean(filepath.Join(filepath.Dir(originalRegistry), value))
			newTarget := filepath.Clean(filepath.Join(filepath.Dir(registry), value))
			if RebasePath(oldTarget, moves) != newTarget {
				return nil, errors.New("Firefox relative profile pointer would change external target; use an absolute profile pointer before migration")
			}
		}
		if next := RebasePath(value, moves); next != value {
			end := ""
			if strings.HasSuffix(line, "\r\n") {
				end = "\r\n"
			} else if strings.HasSuffix(line, "\n") {
				end = "\n"
			}
			lines[i] = line[:strings.Index(line, "=")+1] + next + end
		}
	}
	return []byte(strings.Join(lines, "")), nil
}

// XPIDatabase's PROP_JSON_FIELDS lists path/rootURI separately from signedState,
// signedTypes and hashes. Only these filesystem pointers change; XPI bytes and
// signature metadata are never touched.
// https://searchfox.org/mozilla-central/source/toolkit/mozapps/extensions/internal/XPIDatabase.sys.mjs
func rebaseFirefoxAddons(data []byte, moves []Move) ([]byte, error) {
	out, err := rebaseJSON(data, []string{"addons.*.path"}, moves)
	if err != nil {
		return nil, err
	}
	var record map[string]json.RawMessage
	if err := json.Unmarshal(out, &record); err != nil {
		return nil, err
	}
	var addons []map[string]json.RawMessage
	if err := json.Unmarshal(record["addons"], &addons); err != nil {
		return nil, err
	}
	changed := false
	for _, addon := range addons {
		raw, exists := addon["rootURI"]
		if !exists || bytes.Equal(raw, []byte("null")) {
			continue
		}
		var uri string
		if err := json.Unmarshal(raw, &uri); err != nil {
			return nil, err
		}
		next, err := rebaseAddonURI(uri, moves)
		if err != nil {
			return nil, err
		}
		if next != uri {
			addon["rootURI"], err = json.Marshal(next)
			if err != nil {
				return nil, err
			}
			changed = true
		}
	}
	if !changed {
		return out, nil
	}
	record["addons"], err = json.Marshal(addons)
	if err != nil {
		return nil, err
	}
	return json.Marshal(record)
}

func rebaseAddonURI(uri string, moves []Move) (string, error) {
	original, prefix, suffix := uri, "", ""
	if strings.HasPrefix(uri, "jar:file:") {
		var found bool
		uri, suffix, found = strings.Cut(strings.TrimPrefix(uri, "jar:"), "!/")
		if !found {
			return "", errors.New("malformed Firefox addon jar URI")
		}
		prefix, suffix = "jar:", "!/"+suffix
	}
	if !strings.HasPrefix(uri, "file:") {
		return original, nil
	}
	parsed, err := url.Parse(uri)
	if err != nil {
		return "", err
	}
	if parsed.Host != "" && parsed.Host != "localhost" {
		return "", errors.New("nonlocal Firefox addon file URI")
	}
	if parsed.RawQuery != "" || parsed.Fragment != "" || strings.ContainsRune(parsed.Path, '\x00') || !filepath.IsAbs(parsed.Path) || filepath.Clean(parsed.Path) != strings.TrimSuffix(parsed.Path, "/") {
		return "", errors.New("unsafe Firefox addon file URI")
	}
	path := strings.TrimSuffix(parsed.Path, "/")
	next := RebasePath(path, moves)
	if next == path {
		return original, nil
	}
	if strings.HasSuffix(parsed.Path, "/") {
		next += "/"
	}
	parsed.Path, parsed.RawPath = next, ""
	return prefix + parsed.String() + suffix, nil
}

// Firefox writes user_pref calls, not JSON files. Only the two established
// directory preferences are interpreted; every other line remains literal.
func rebaseFirefoxPreferences(data []byte, moves []Move) ([]byte, error) {
	lines := strings.SplitAfter(string(data), "\n")
	seen := make(map[string]bool)
	for i, line := range lines {
		text := strings.TrimSpace(line)
		if !strings.HasPrefix(text, "user_pref(") {
			continue
		}
		body := strings.TrimPrefix(text, "user_pref(")
		key, _, found := strings.Cut(body, ",")
		if !found {
			continue
		}
		var name string
		if json.Unmarshal([]byte(strings.TrimSpace(key)), &name) != nil {
			continue
		}
		if name != "browser.download.dir" && name != "browser.download.lastDir" {
			continue
		}
		if seen[name] {
			return nil, fmt.Errorf("duplicate Firefox directory preference %s", name)
		}
		seen[name] = true
		if !strings.HasSuffix(body, ");") {
			return nil, fmt.Errorf("malformed Firefox directory preference %s", name)
		}
		var args []json.RawMessage
		if err := json.Unmarshal([]byte("["+strings.TrimSuffix(body, ");")+"]"), &args); err != nil {
			return nil, err
		}
		if len(args) != 2 {
			return nil, errors.New("invalid Firefox directory preference arguments")
		}
		next, err := rebaseJSONField(args[1], nil, moves)
		if err != nil {
			return nil, err
		}
		if bytes.Equal(args[1], next) {
			continue
		}
		ending := ""
		if strings.HasSuffix(line, "\r\n") {
			ending = "\r\n"
		} else if strings.HasSuffix(line, "\n") {
			ending = "\n"
		}
		lines[i] = "user_pref(" + string(args[0]) + ", " + string(next) + ");" + ending
	}
	return []byte(strings.Join(lines, "")), nil
}

func rebaseMCPReferences(data []byte, moves []Move) ([]byte, error) {
	out, err := rebaseJSON(data, []string{"*.info.command", "*.info.args.*"}, moves)
	if err != nil {
		return nil, err
	}
	var servers []struct {
		Info struct {
			Args []string `json:"args"`
		} `json:"info"`
	}
	if err := json.Unmarshal(data, &servers); err != nil {
		return nil, err
	}
	for _, server := range servers {
		for _, arg := range server.Info.Args {
			// An embedded flag value is not an independent argv path. Its
			// syntax belongs to the external executable; require the user
			// to express it as a separate path argument instead of guessing.
			if strings.HasPrefix(arg, "-") {
				_, value, found := strings.Cut(arg, "=")
				if found && RebasePath(value, moves) != value {
					return nil, errors.New("MCP flag embeds a relocated path: use separate flag/path arguments before migration")
				}
			}
		}
	}
	return out, nil
}
