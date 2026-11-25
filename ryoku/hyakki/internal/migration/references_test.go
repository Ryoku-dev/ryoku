package migration_test

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"go.etcd.io/bbolt"

	"ryoku-rashin/internal/collections"
	"ryoku-rashin/internal/config"
	"ryoku-rashin/internal/localmodels"
	"ryoku-rashin/internal/mcpservers"
	"ryoku-rashin/internal/memory/hindsight"
	"ryoku-rashin/internal/migration"
	"ryoku-rashin/internal/paths"
	"ryoku-rashin/internal/protocol"
	"ryoku-rashin/internal/session"
)

func referenceEnvironment(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	for variable, dir := range map[string]string{"HOME": "home", "XDG_CONFIG_HOME": "config", "XDG_STATE_HOME": "state", "XDG_DATA_HOME": "data", "XDG_CACHE_HOME": "cache", "XDG_RUNTIME_DIR": "runtime"} {
		t.Setenv(variable, filepath.Join(root, dir))
	}
	return root
}

func referenceWrite(t *testing.T, name string, data []byte, mode os.FileMode) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(name), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(name, data, mode); err != nil {
		t.Fatal(err)
	}
}

func referenceJSON(t *testing.T, name string, value any) {
	t.Helper()
	data, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	referenceWrite(t, name, data, 0600)
}

func referenceRead(t *testing.T, name string) []byte {
	t.Helper()
	data, err := os.ReadFile(name)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func referenceMove(t *testing.T, source, destination string) migration.Move {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(destination), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(source, destination); err != nil {
		t.Fatal(err)
	}
	return migration.Move{Source: source, Destination: destination}
}

func TestRebaseReferencesKeepsLocalModelsAndStoredAttachmentsUsable(t *testing.T) {
	root := referenceEnvironment(t)
	oldData := filepath.Join(root, "legacy-data")
	oldState := filepath.Join(root, "legacy-state")
	oldModels := filepath.Join(oldData, "models")
	oldAttachments := filepath.Join(oldData, "attachments")
	oldRuntime := filepath.Join(oldData, "llama.cpp")
	modelBytes := []byte("fixture weights retained without reparsing or hashing by migration")
	modelFile := localmodels.File{Name: "fixture.gguf", Role: protocol.LocalFileModel, Size: int64(len(modelBytes))}
	sum := sha256.Sum256(modelBytes)
	modelFile.SHA256 = hex.EncodeToString(sum[:])
	entry := localmodels.Entry{ID: "fixture", Repo: "Fixture/model", Revision: "pinned", Files: []localmodels.File{modelFile}}
	referenceWrite(t, filepath.Join(oldModels, "Fixture", "model", modelFile.Name), modelBytes, 0600)
	referenceJSON(t, filepath.Join(oldModels, "custom.json"), []localmodels.Entry{entry})
	manifest := map[string]any{"id": entry.ID, "repo": entry.Repo, "revision": entry.Revision, "files": entry.Files, "header": map[string]any{}, "unknown": json.RawMessage(`{"number":900719925474099312345}`)}
	manifestPath := filepath.Join(oldModels, "Fixture", "model", "rashin-fixture.json")
	referenceJSON(t, manifestPath, manifest)
	manifestBefore := referenceRead(t, manifestPath)
	binary := filepath.Join(oldRuntime, "llama-b12521", "llama-server")
	referenceWrite(t, binary, []byte("fixture executable bytes"), 0751)
	runtimeBytes := []byte(`{"build":12521,"backend":"cpu","dir":"llama-b12521","sha256":"signed-archive-hash"}`)
	referenceWrite(t, filepath.Join(oldRuntime, "runtime.json"), runtimeBytes, 0600)
	image := filepath.Join(oldAttachments, "capture.png")
	imageBytes := []byte("stored image fixture")
	referenceWrite(t, image, imageBytes, 0640)
	settings := config.Defaults()
	settings.Providers.Models.Binary = binary
	settings.Providers.Models.Folders = []string{oldModels, filepath.Join(root, "external-models"), paths.VaultDir(), filepath.Join(paths.DataHome(), "ryoku", "worktrees")}
	settings.Ask.Model = protocol.ProviderRashinModels + "/fixture"
	settingsData, err := json.Marshal(settings)
	if err != nil {
		t.Fatal(err)
	}
	var settingsRecord map[string]json.RawMessage
	if err := json.Unmarshal(settingsData, &settingsRecord); err != nil {
		t.Fatal(err)
	}
	settingsRecord["version"] = json.RawMessage(fmt.Sprint(config.Version))
	referenceJSON(t, paths.ConfigFile(), settingsRecord)
	id := "ask-20261121-010203-abcd"
	prose := "Keep the literal old path in user prose: " + image
	meta := protocol.SessionInfo{ID: id, Kind: protocol.KindAsk, Title: prose, Cwd: oldAttachments, Worktree: filepath.Join(paths.DataHome(), "ryoku", "worktrees", "original"), LastSeq: 1}
	referenceJSON(t, filepath.Join(oldState, "sessions", id, "meta.json"), meta)
	eventData, err := json.Marshal(protocol.UserData{Text: prose, Images: []protocol.Attachment{{Path: image, Mime: "image/png"}}})
	if err != nil {
		t.Fatal(err)
	}
	event, err := json.Marshal(protocol.SessionEvent{Seq: 1, Kind: protocol.EvUser, TS: time.Unix(123, 0).UTC(), Data: eventData})
	if err != nil {
		t.Fatal(err)
	}
	referenceWrite(t, filepath.Join(oldState, "sessions", id, "events.jsonl"), append(event, '\n'), 0640)
	engine := []byte(`{"Role":"assistant","Content":"` + image + `","Thinking":[{"Signature":"do-not-change","Redacted":"AQID"}]}` + "\n")
	referenceWrite(t, filepath.Join(oldState, "sessions", id, "engine.jsonl"), engine, 0600)
	collectionOld := filepath.Join(oldData, "collections")
	referenceJSON(t, filepath.Join(collectionOld, "saved.json"), map[string]any{"collection": protocol.Collection{ID: "saved", Name: "Saved"}, "items": []protocol.CollectionItem{{ID: "item", Path: image, Image: image, Source: image, Text: prose}}})
	moves := []migration.Move{
		referenceMove(t, oldModels, paths.ModelsDir()),
		referenceMove(t, oldRuntime, paths.LlamaRuntimeDir()),
		referenceMove(t, oldAttachments, paths.AttachmentsDir()),
		referenceMove(t, collectionOld, filepath.Join(paths.DataDir(), "collections")),
		referenceMove(t, oldState, paths.StateDir()),
		{Source: filepath.Join(root, "legacy-config.json"), Destination: paths.ConfigFile()},
	}
	if err := migration.RebaseReferences(moves); err != nil {
		t.Fatal(err)
	}
	loaded, err := config.Load()
	if err != nil {
		t.Fatal(err)
	}
	wantBinary := filepath.Join(paths.LlamaRuntimeDir(), "llama-b12521", "llama-server")
	if loaded.Ask.Model != protocol.ProviderRashinModels+"/fixture" || loaded.Providers.Models.Binary != wantBinary || loaded.Providers.Models.Folders[0] != paths.ModelsDir() {
		t.Fatalf("local model selection or executable not restored: %+v", loaded)
	}
	for i := 1; i < len(settings.Providers.Models.Folders); i++ {
		if loaded.Providers.Models.Folders[i] != settings.Providers.Models.Folders[i] {
			t.Fatalf("external folder %d changed", i)
		}
	}
	binaryInfo, err := os.Stat(loaded.Providers.Models.Binary)
	if err != nil || binaryInfo.Mode().Perm() != 0751 {
		t.Fatalf("executable no longer usable: %v, %v", binaryInfo, err)
	}
	inventory := localmodels.New(func() protocol.Settings { return loaded }, nil).Inventory()
	found := false
	for _, model := range inventory {
		if model.ID == entry.ID && model.State == protocol.LocalInstalled {
			found = true
		}
	}
	if !found {
		t.Fatalf("installed model lost from real consumer inventory: %+v", inventory)
	}
	if !bytes.Equal(referenceRead(t, filepath.Join(paths.ModelsDir(), "Fixture", "model", "rashin-fixture.json")), manifestBefore) {
		t.Fatal("relative model manifest/hash/unknown precision changed")
	}
	if !bytes.Equal(referenceRead(t, filepath.Join(paths.LlamaRuntimeDir(), "runtime.json")), runtimeBytes) {
		t.Fatal("relative runtime metadata changed")
	}
	info, err := session.ReadInfo(id)
	if err != nil || info.Cwd != paths.AttachmentsDir() || info.Title != prose || info.Worktree != meta.Worktree {
		t.Fatalf("session metadata: %+v, %v", info, err)
	}
	events, err := session.Events(id)
	if err != nil || len(events) != 1 {
		t.Fatalf("stored events: %+v, %v", events, err)
	}
	var user protocol.UserData
	if err := json.Unmarshal(events[0].Data, &user); err != nil {
		t.Fatal(err)
	}
	if user.Text != prose || len(user.Images) != 1 || !bytes.Equal(referenceRead(t, user.Images[0].Path), imageBytes) {
		t.Fatalf("attachment no longer usable: %+v", user)
	}
	items, err := collections.New().Items("saved")
	if err != nil {
		t.Fatal(err)
	}
	_, attachments := collections.Chat(items, "Summarize")
	if len(attachments) != 1 || !bytes.Equal(referenceRead(t, attachments[0].Path), imageBytes) || items[0].Text != prose {
		t.Fatalf("saved attachment consumer: %+v", attachments)
	}
	if !bytes.Equal(referenceRead(t, paths.EngineTranscript(id)), engine) {
		t.Fatal("signed model history was rewritten")
	}
	eventsInfo, err := os.Stat(filepath.Join(paths.SessionDir(id), "events.jsonl"))
	if err != nil || eventsInfo.Mode().Perm() != 0640 {
		t.Fatal("event file permissions changed")
	}
}

func TestRebaseReferencesPreservesUnknownFieldsIntegersAndOpaqueStores(t *testing.T) {
	root := referenceEnvironment(t)
	old := filepath.Join(root, "old-state")
	modelRoot := filepath.Join(root, "old-models")
	data := []byte(fmt.Sprintf(`{"last":{"worktree":%q},"unknown":{"number":900719925474099312345,"sealed":"AQIDBA==","text":%q}}`, modelRoot, modelRoot))
	referenceWrite(t, filepath.Join(old, "app.json"), data, 0640)
	dbName := filepath.Join(old, "gateway.db")
	referenceWrite(t, filepath.Join(old, "vault.db"), []byte("sealed credential bytes deliberately not opened"), 0600)
	if err := os.MkdirAll(old, 0700); err != nil {
		t.Fatal(err)
	}
	db, err := bbolt.Open(dbName, 0600, nil)
	if err != nil {
		t.Fatal(err)
	}
	err = db.Update(func(tx *bbolt.Tx) error {
		b, err := tx.CreateBucket([]byte("keys"))
		if err != nil {
			return err
		}
		if err := b.Put([]byte("id-stable"), []byte(`{"sealed":"AQIDBA==","id":"stable","count":900719925474099312345}`)); err != nil {
			return err
		}
		account, err := tx.CreateBucket([]byte("account"))
		if err != nil {
			return err
		}
		return account.Put([]byte("tokens"), []byte{0, 3, 255, 127, 42, 31})
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	dbBefore := referenceRead(t, dbName)
	move := referenceMove(t, old, paths.StateDir())
	if err := os.MkdirAll(paths.ModelsDir(), 0700); err != nil {
		t.Fatal(err)
	}
	if err := migration.RebaseReferences([]migration.Move{move, {Source: modelRoot, Destination: paths.ModelsDir()}}); err != nil {
		t.Fatal(err)
	}
	var got map[string]json.RawMessage
	if err := json.Unmarshal(referenceRead(t, paths.AppState()), &got); err != nil {
		t.Fatal(err)
	}
	var unknown map[string]json.RawMessage
	if err := json.Unmarshal(got["unknown"], &unknown); err != nil {
		t.Fatal(err)
	}
	if string(unknown["number"]) != "900719925474099312345" || string(unknown["sealed"]) != `"AQIDBA=="` {
		t.Fatalf("opaque data lost: %s", got["unknown"])
	}
	var prose string
	if err := json.Unmarshal(unknown["text"], &prose); err != nil || prose != modelRoot {
		t.Fatal("unknown prose changed")
	}
	if !bytes.Equal(referenceRead(t, paths.GatewayDB()), dbBefore) {
		t.Fatal("encrypted gateway database bytes changed")
	}
	if string(referenceRead(t, paths.CredentialVault())) != "sealed credential bytes deliberately not opened" {
		t.Fatal("credential vault changed")
	}
}

func TestRebaseReferencesRebasesOnlyIndexSourceNotKeysHashesOrVectors(t *testing.T) {
	root := referenceEnvironment(t)
	oldCache := filepath.Join(root, "old-cache")
	oldModels := filepath.Join(root, "old-models")
	name := filepath.Join(oldCache, "index", "fixture", "index.db")
	if err := os.MkdirAll(filepath.Dir(name), 0700); err != nil {
		t.Fatal(err)
	}
	db, err := bbolt.Open(name, 0640, nil)
	if err != nil {
		t.Fatal(err)
	}
	record := []byte(fmt.Sprintf(`{"path":"model.go","source":%q,"hash":"unchanged-hash","size":9007199254740993,"unknown":{"signature":"sealed"}}`, filepath.Join(oldModels, "model.go")))
	err = db.Update(func(tx *bbolt.Tx) error {
		files, err := tx.CreateBucket([]byte("files"))
		if err != nil {
			return err
		}
		if err := files.Put([]byte("model.go"), record); err != nil {
			return err
		}
		vectors, err := tx.CreateBucket([]byte("vectors"))
		if err != nil {
			return err
		}
		return vectors.Put([]byte("content-derived-ID"), []byte{0, 1, 255, 127})
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(paths.ModelsDir(), 0700); err != nil {
		t.Fatal(err)
	}
	moves := []migration.Move{referenceMove(t, oldCache, paths.CacheDir()), {Source: oldModels, Destination: paths.ModelsDir()}}
	if err := migration.RebaseReferences(moves); err != nil {
		t.Fatal(err)
	}
	db, err = bbolt.Open(filepath.Join(paths.IndexDir("fixture"), "index.db"), 0640, &bbolt.Options{ReadOnly: true})
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err := db.View(func(tx *bbolt.Tx) error {
		var got map[string]json.RawMessage
		if err := json.Unmarshal(tx.Bucket([]byte("files")).Get([]byte("model.go")), &got); err != nil {
			return err
		}
		var source string
		if err := json.Unmarshal(got["source"], &source); err != nil {
			return err
		}
		if source != filepath.Join(paths.ModelsDir(), "model.go") || string(got["size"]) != "9007199254740993" || string(got["hash"]) != `"unchanged-hash"` {
			t.Fatalf("index source record: %+v", got)
		}
		if !bytes.Equal(tx.Bucket([]byte("vectors")).Get([]byte("content-derived-ID")), []byte{0, 1, 255, 127}) {
			t.Fatal("vector ID or bytes changed")
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
}

func TestRebaseReferencesFailsBeforeCommittingMalformedOrUnsafeMetadata(t *testing.T) {
	for _, bad := range []string{`{"cwd":`, `{"cwd":"/safe","cwd":"/other"}`, `{"cwd":42}`, `null`, `{"cwd":"/unsafe/../model"}`, `{"cwd":"/unsafe\u0000model"}`} {
		t.Run(bad, func(t *testing.T) {
			root := referenceEnvironment(t)
			old := filepath.Join(root, "old-state")
			good := []byte(fmt.Sprintf(`{"last":{"worktree":%q}}`, old))
			referenceWrite(t, filepath.Join(old, "app.json"), good, 0600)
			referenceWrite(t, filepath.Join(old, "sessions", "fixture", "meta.json"), []byte(bad), 0600)
			move := referenceMove(t, old, paths.StateDir())
			if err := migration.RebaseReferences([]migration.Move{move}); err == nil {
				t.Fatal("unsafe metadata falsely succeeded")
			}
			if !bytes.Equal(referenceRead(t, paths.AppState()), good) {
				t.Fatal("staged valid metadata committed before validation completed")
			}
		})
	}
}

func TestRebaseReferencesDoesNotTouchExistingUnmovedConsumers(t *testing.T) {
	root := referenceEnvironment(t)
	old := filepath.Join(root, "old-models")
	prose := []byte(fmt.Sprintf(`{"providers":{"models":{"binary":%q}}}`, filepath.Join(old, "llama-server")))
	referenceWrite(t, paths.ConfigFile(), prose, 0600)
	if err := os.MkdirAll(paths.ModelsDir(), 0700); err != nil {
		t.Fatal(err)
	}
	if err := migration.RebaseReferences([]migration.Move{{Source: old, Destination: paths.ModelsDir()}}); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(referenceRead(t, paths.ConfigFile()), prose) {
		t.Fatal("existing destination not in move map was mutated")
	}
}

func TestRebaseReferencesMigratesFirefoxPointersAndInvalidatesOnlyDerivedCache(t *testing.T) {
	root := referenceEnvironment(t)
	old := filepath.Join(root, "old-screen-home")
	profile := filepath.Join(old, ".mozilla", "firefox", "fixture.default")
	xpi := filepath.Join(profile, "extensions", "fixture@example.xpi")
	xpiBytes := []byte("signed XPI bytes")
	referenceWrite(t, xpi, xpiBytes, 0600)
	referenceWrite(t, filepath.Join(profile, "cookies.sqlite"), []byte("opaque browser database"), 0600)
	referenceWrite(t, filepath.Join(profile, "startupCache", "webext.sc.lz4"), []byte("derived webextension cache"), 0600)
	referenceWrite(t, filepath.Join(profile, "prefs.js"), []byte(fmt.Sprintf("user_pref(\"browser.download.dir\", %q);\nuser_pref(\"authored.prose\", %q);\n", filepath.Join(old, "Downloads"), old)), 0600)
	referenceWrite(t, filepath.Join(profile, "logins.json"), []byte(`{"encryptedPassword":"AQIDBA=="}`), 0600)
	referenceWrite(t, filepath.Join(profile, "addonStartup.json.lz4"), []byte("opaque derived compressed cache"), 0600)
	referenceWrite(t, filepath.Join(old, ".mozilla", "firefox", "profiles.ini"), []byte("[Profile0]\nName=fixture\nIsRelative=0\nPath="+profile+"\n[Profile1]\nIsRelative=1\nPath=relative.default\n"), 0640)
	referenceJSON(t, filepath.Join(profile, "extensions.json"), map[string]any{"schemaVersion": 37, "addons": []map[string]any{{"id": "fixture@example", "path": xpi, "rootURI": "jar:file://" + xpi + "!/", "signedState": 2, "signedTypes": []int{1}, "unknown": json.RawMessage(`900719925474099312345`)}, {"id": "external@example", "path": "/external/addon.xpi", "rootURI": "jar:file:///external/addon.xpi!/"}}})
	pref := filepath.Join(old, ".config", "chromium", "Default", "Preferences")
	referenceJSON(t, pref, map[string]any{"download": map[string]any{"default_directory": filepath.Join(old, "Downloads")}, "unknown": old})
	unrelated := []byte(`{"addons":[{"path":"authored user text"}]}`)
	referenceWrite(t, filepath.Join(old, "Documents", "extensions.json"), unrelated, 0600)
	move := referenceMove(t, old, paths.ScreenHome())
	if err := migration.RebaseReferences([]migration.Move{move}); err != nil {
		t.Fatal(err)
	}
	newProfile := filepath.Join(paths.ScreenHome(), ".mozilla", "firefox", "fixture.default")
	if !bytes.Equal(referenceRead(t, filepath.Join(newProfile, "extensions", "fixture@example.xpi")), xpiBytes) {
		t.Fatal("signed extension was changed")
	}
	if string(referenceRead(t, filepath.Join(newProfile, "cookies.sqlite"))) != "opaque browser database" || string(referenceRead(t, filepath.Join(newProfile, "logins.json"))) != `{"encryptedPassword":"AQIDBA=="}` {
		t.Fatal("browser credentials changed")
	}
	if _, err := os.Stat(filepath.Join(newProfile, "addonStartup.json.lz4")); !os.IsNotExist(err) {
		t.Fatalf("derived startup cache not invalidated: %v", err)
	}
	if _, err := os.Stat(filepath.Join(newProfile, "startupCache", "webext.sc.lz4")); !os.IsNotExist(err) {
		t.Fatalf("webextension startup cache not invalidated: %v", err)
	}
	prefs := referenceRead(t, filepath.Join(newProfile, "prefs.js"))
	if !bytes.Contains(prefs, []byte(fmt.Sprintf("user_pref(\"browser.download.dir\", %q);", filepath.Join(paths.ScreenHome(), "Downloads")))) || !bytes.Contains(prefs, []byte(fmt.Sprintf("user_pref(\"authored.prose\", %q);", old))) {
		t.Fatalf("Firefox preferences: %s", prefs)
	}
	var chromium struct {
		Download struct {
			Directory string `json:"default_directory"`
		} `json:"download"`
		Unknown string `json:"unknown"`
	}
	if err := json.Unmarshal(referenceRead(t, filepath.Join(paths.ScreenHome(), ".config", "chromium", "Default", "Preferences")), &chromium); err != nil {
		t.Fatal(err)
	}
	if chromium.Download.Directory != filepath.Join(paths.ScreenHome(), "Downloads") || chromium.Unknown != old {
		t.Fatalf("Chromium preferences: %+v", chromium)
	}
	var addons struct {
		Addons []struct {
			Path        string          `json:"path"`
			RootURI     string          `json:"rootURI"`
			SignedState int             `json:"signedState"`
			Unknown     json.RawMessage `json:"unknown"`
		} `json:"addons"`
	}
	if err := json.Unmarshal(referenceRead(t, filepath.Join(newProfile, "extensions.json")), &addons); err != nil {
		t.Fatal(err)
	}
	wantXPI := filepath.Join(newProfile, "extensions", "fixture@example.xpi")
	if addons.Addons[0].Path != wantXPI || addons.Addons[0].RootURI != "jar:file://"+wantXPI+"!/" || addons.Addons[0].SignedState != 2 || string(addons.Addons[0].Unknown) != "900719925474099312345" || addons.Addons[1].Path != "/external/addon.xpi" {
		t.Fatalf("addon metadata: %+v", addons)
	}
	if !bytes.Contains(referenceRead(t, filepath.Join(paths.ScreenHome(), ".mozilla", "firefox", "profiles.ini")), []byte("Path="+newProfile+"\n")) {
		t.Fatal("absolute profile pointer not rebased")
	}
	if !bytes.Equal(referenceRead(t, filepath.Join(paths.ScreenHome(), "Documents", "extensions.json")), unrelated) {
		t.Fatal("unrecognized user file was interpreted as browser metadata")
	}
}

func TestEnsureKeepsInternalAndExternalLinkedModelsInstalled(t *testing.T) {
	root := referenceEnvironment(t)
	oldModels := filepath.Join(paths.LegacyRashinData(), "models")
	var entries []localmodels.Entry
	for _, fixture := range []struct{ id, directory string }{
		{"found-internal", filepath.Join(oldModels, "linked")},
		{"found-external", filepath.Join(root, "external-models")},
	} {
		name := filepath.Join(fixture.directory, "fixture.gguf")
		referenceWrite(t, name, []byte("linked weights"), 0600)
		stamp := time.Unix(1_700_000_000, 123456789)
		if err := os.Chtimes(name, stamp, stamp); err != nil {
			t.Fatal(err)
		}
		info, err := os.Stat(name)
		if err != nil {
			t.Fatal(err)
		}
		entry := localmodels.Entry{ID: fixture.id, Repo: "found/" + fixture.id, Revision: "file-stamp", External: fixture.directory, Files: []localmodels.File{{Name: "fixture.gguf", Role: protocol.LocalFileModel, Size: info.Size(), ModTime: info.ModTime().UnixNano()}}}
		entries = append(entries, entry)
		referenceJSON(t, filepath.Join(oldModels, filepath.FromSlash(entry.Repo), "rashin-"+entry.ID+".json"), map[string]any{"id": entry.ID, "repo": entry.Repo, "revision": entry.Revision, "files": entry.Files})
	}
	referenceJSON(t, filepath.Join(oldModels, "custom.json"), entries)
	if err := migration.Ensure(); err != nil {
		t.Fatal(err)
	}
	inventory := localmodels.New(config.Defaults, nil).Inventory()
	seen := make(map[string]string)
	for _, model := range inventory {
		seen[model.ID] = model.Path
	}
	if seen["found-internal"] != filepath.Join(paths.ModelsDir(), "linked", "fixture.gguf") || seen["found-external"] != filepath.Join(root, "external-models", "fixture.gguf") {
		t.Fatalf("linked model consumer inventory lost paths: %+v", inventory)
	}
	for _, path := range seen {
		if _, err := os.Stat(path); err != nil {
			t.Fatal(err)
		}
	}
}

func TestRebaseReferencesMigratesMemoryStampKeysAndMCPExecutableOnly(t *testing.T) {
	root := referenceEnvironment(t)
	oldState := filepath.Join(root, "old-state")
	oldData := filepath.Join(root, "old-data")
	legacyMemory := filepath.Join(oldState, "memory", "teams", "team.md")
	externalMemory := filepath.Join(root, "shared-vault", "you.md")
	referenceWrite(t, legacyMemory, []byte("User-authored memory remains literal: "+oldState), 0600)
	database := filepath.Join(oldState, "memory", "memory.db")
	db, err := bbolt.Open(database, 0640, nil)
	if err != nil {
		t.Fatal(err)
	}
	stamp := []byte(`{"mod":9007199254740993123,"size":123,"unknown":"literal"}`)
	fact := []byte(`{"id":"stable","scope":"team:fixture","text":"do not rewrite history","vector":"AQIDBA=="}`)
	err = db.Update(func(tx *bbolt.Tx) error {
		files, err := tx.CreateBucket([]byte("files"))
		if err != nil {
			return err
		}
		if err := files.Put([]byte(legacyMemory), stamp); err != nil {
			return err
		}
		if err := files.Put([]byte(externalMemory), stamp); err != nil {
			return err
		}
		facts, err := tx.CreateBucket([]byte("facts"))
		if err != nil {
			return err
		}
		return facts.Put([]byte("stable"), fact)
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	executable := filepath.Join(oldData, "models", "mcp-server")
	referenceWrite(t, executable, []byte("MCP executable bytes"), 0700)
	serverPath := filepath.Join(oldData, "mcp-servers.json")
	referenceJSON(t, serverPath, []map[string]any{{"info": protocol.MCPServerInfo{ID: "stdio-fixture", Name: "Fixture", Type: "stdio", Command: executable, Args: []string{"literal user argument " + oldData, executable, "--external=/outside/unchanged"}}, "headers": map[string]string{"Authorization": "credential-id"}, "token": "sealed-token-id", "unknown": json.RawMessage(`900719925474099312345`)}})
	moves := []migration.Move{referenceMove(t, oldState, paths.StateDir()), referenceMove(t, oldData, paths.DataDir())}
	if err := migration.RebaseReferences(moves); err != nil {
		t.Fatal(err)
	}
	db, err = bbolt.Open(filepath.Join(paths.StateDir(), "memory", "memory.db"), 0640, &bbolt.Options{ReadOnly: true})
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err := db.View(func(tx *bbolt.Tx) error {
		files := tx.Bucket([]byte("files"))
		if files.Get([]byte(legacyMemory)) != nil || !bytes.Equal(files.Get([]byte(filepath.Join(paths.StateDir(), "memory", "teams", "team.md"))), stamp) || !bytes.Equal(files.Get([]byte(externalMemory)), stamp) {
			t.Fatal("memory stamp references or bytes changed incorrectly")
		}
		if !bytes.Equal(tx.Bucket([]byte("facts")).Get([]byte("stable")), fact) {
			t.Fatal("memory facts/IDs/vectors changed")
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	store, err := mcpservers.Open(filepath.Join(paths.DataDir(), "mcp-servers.json"), nil)
	if err != nil {
		t.Fatal(err)
	}
	servers := store.List()
	if len(servers) != 1 || servers[0].Command != filepath.Join(paths.ModelsDir(), "mcp-server") || servers[0].Args[0] != "literal user argument "+oldData || servers[0].Args[1] != filepath.Join(paths.ModelsDir(), "mcp-server") || servers[0].Args[2] != "--external=/outside/unchanged" {
		t.Fatalf("MCP consumer command or opaque arguments: %+v", servers)
	}
	if !bytes.Contains(referenceRead(t, filepath.Join(paths.DataDir(), "mcp-servers.json")), []byte(`"token":"sealed-token-id"`)) {
		t.Fatal("MCP token reference changed")
	}
}

func TestRebaseReferencesInvalidatesHindsightRuntimeWithoutChangingPythonModelsOrDatabase(t *testing.T) {
	root := referenceEnvironment(t)
	oldData := filepath.Join(root, "old-hindsight")
	oldState := filepath.Join(root, "old-state")
	oldCache := filepath.Join(root, "old-cache")
	script := []byte("#!" + filepath.Join(oldData, "venv", "bin", "python") + "\n# third-party console entrypoint\n")
	referenceWrite(t, filepath.Join(oldData, "venv", "bin", "hindsight-api"), script, 0750)
	referenceWrite(t, filepath.Join(oldData, "installed"), []byte("app-owned runtime lock marker"), 0600)
	cluster := []byte{0, 255, 127, 1, 42, 32}
	instance := filepath.Join(oldState, "hindsight", "home", ".pg0", "instances", "rashin")
	referenceWrite(t, filepath.Join(instance, "data", "base", "1234", "5678"), cluster, 0600)
	referenceJSON(t, filepath.Join(instance, "instance.json"), map[string]any{"data_dir": filepath.Join(instance, "data"), "installation_dir": filepath.Join(oldState, "hindsight", "home", ".pg0", "installation"), "username": "rashin", "password": "unchanged-db-secret", "version": "18.1.0", "unknown": json.RawMessage(`900719925474099312345`)})
	referenceWrite(t, filepath.Join(oldCache, "hindsight", "hf", "model.bin"), []byte("cached embedding model"), 0600)
	moves := []migration.Move{referenceMove(t, oldData, filepath.Join(paths.DataDir(), "hindsight")), referenceMove(t, oldState, paths.StateDir()), referenceMove(t, oldCache, paths.CacheDir())}
	if err := migration.RebaseReferences(moves); err != nil {
		t.Fatal(err)
	}
	layout := hindsight.Layout{Data: filepath.Join(paths.DataDir(), "hindsight"), State: filepath.Join(paths.StateDir(), "hindsight"), Cache: filepath.Join(paths.CacheDir(), "hindsight")}
	if layout.Installed() {
		t.Fatal("nonrelocatable Python environment falsely reported installed")
	}
	if _, err := os.Stat(filepath.Join(layout.Data, "installed")); !os.IsNotExist(err) {
		t.Fatalf("runtime completion marker retained: %v", err)
	}
	if !bytes.Equal(referenceRead(t, filepath.Join(layout.Data, "venv", "bin", "hindsight-api")), script) {
		t.Fatal("third-party Python scripts were rewritten")
	}
	newInstance := filepath.Join(layout.State, "home", ".pg0", "instances", "rashin")
	if !bytes.Equal(referenceRead(t, filepath.Join(newInstance, "data", "base", "1234", "5678")), cluster) {
		t.Fatal("PostgreSQL cluster bytes changed")
	}
	if string(referenceRead(t, filepath.Join(layout.Cache, "hf", "model.bin"))) != "cached embedding model" {
		t.Fatal("cached model changed")
	}
	var info map[string]json.RawMessage
	if err := json.Unmarshal(referenceRead(t, filepath.Join(newInstance, "instance.json")), &info); err != nil {
		t.Fatal(err)
	}
	var dataDir, password string
	if err := json.Unmarshal(info["data_dir"], &dataDir); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(info["password"], &password); err != nil {
		t.Fatal(err)
	}
	if dataDir != filepath.Join(newInstance, "data") || password != "unchanged-db-secret" || string(info["unknown"]) != "900719925474099312345" {
		t.Fatalf("pg0 instance path/opaque metadata: %+v", info)
	}
}

func TestRebaseReferencesRefusesEmbeddedMCPPathBeforeInvalidatingRuntime(t *testing.T) {
	root := referenceEnvironment(t)
	oldData := filepath.Join(root, "old-data")
	marker := []byte("must survive failed migration")
	referenceWrite(t, filepath.Join(oldData, "hindsight", "installed"), marker, 0600)
	referenceJSON(t, filepath.Join(oldData, "mcp-servers.json"), []map[string]any{{"info": protocol.MCPServerInfo{Type: "stdio", Command: "node", Args: []string{"--config=" + filepath.Join(oldData, "models", "config.json")}}}})
	move := referenceMove(t, oldData, paths.DataDir())
	if err := migration.RebaseReferences([]migration.Move{move}); err == nil {
		t.Fatal("embedded path argument falsely migrated")
	}
	if !bytes.Equal(referenceRead(t, filepath.Join(paths.DataDir(), "hindsight", "installed")), marker) {
		t.Fatal("runtime invalidation committed despite migration failure")
	}
}

func TestRebaseReferencesPreservesToolProseWhileUpdatingNavigation(t *testing.T) {
	root := referenceEnvironment(t)
	old := filepath.Join(root, "old-state")
	id := "agent-20261121-010203-abcd"
	path := filepath.Join(old, "generated.txt")
	prose := "Never rewrite history containing " + path
	referenceJSON(t, filepath.Join(old, "sessions", id, "meta.json"), protocol.SessionInfo{ID: id, Kind: protocol.KindAgent, LastSeq: 1})
	payload, err := json.Marshal(protocol.ToolData{Call: "stable-call", Input: prose, Output: prose, Diff: &protocol.Diff{Path: path, Old: prose, New: prose}, Locations: []protocol.Location{{Path: path, Line: 42}}})
	if err != nil {
		t.Fatal(err)
	}
	event, err := json.Marshal(protocol.SessionEvent{Seq: 1, Kind: protocol.EvTool, Data: payload})
	if err != nil {
		t.Fatal(err)
	}
	referenceWrite(t, filepath.Join(old, "sessions", id, "events.jsonl"), append(event, '\n'), 0600)
	move := referenceMove(t, old, paths.StateDir())
	if err := migration.RebaseReferences([]migration.Move{move}); err != nil {
		t.Fatal(err)
	}
	events, err := session.Events(id)
	if err != nil || len(events) != 1 {
		t.Fatalf("tool event consumer: %v %v", events, err)
	}
	var tool protocol.ToolData
	if err := json.Unmarshal(events[0].Data, &tool); err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(paths.StateDir(), "generated.txt")
	if tool.Diff.Path != want || tool.Locations[0].Path != want || tool.Diff.Old != prose || tool.Diff.New != prose || tool.Input != prose || tool.Output != prose || tool.Call != "stable-call" {
		t.Fatalf("typed tool reference/prose: %+v", tool)
	}
}

func TestRebaseReferencesRejectsCorruptJSONLAndDatabaseWithoutCommitting(t *testing.T) {
	for _, format := range []string{"jsonl", "bbolt"} {
		t.Run(format, func(t *testing.T) {
			root := referenceEnvironment(t)
			oldState := filepath.Join(root, "old-state")
			oldCache := filepath.Join(root, "old-cache")
			before := []byte(fmt.Sprintf(`{"last":{"worktree":%q}}`, oldState))
			referenceWrite(t, filepath.Join(oldState, "app.json"), before, 0600)
			var moves []migration.Move
			if format == "jsonl" {
				referenceWrite(t, filepath.Join(oldState, "sessions", "fixture", "events.jsonl"), []byte("{\"kind\":\"user\",\"data\":{\"images\":[\n"), 0600)
			} else {
				referenceWrite(t, filepath.Join(oldCache, "index", "fixture", "index.db"), make([]byte, 8192), 0600)
				moves = append(moves, referenceMove(t, oldCache, paths.CacheDir()))
			}
			moves = append(moves, referenceMove(t, oldState, paths.StateDir()))
			if err := migration.RebaseReferences(moves); err == nil {
				t.Fatal("corrupt persisted format falsely succeeded")
			}
			if !bytes.Equal(referenceRead(t, paths.AppState()), before) {
				t.Fatal("valid metadata committed alongside malformed metadata")
			}
		})
	}
}

func TestRebaseReferencesRefusesAmbiguousMemoryStampCollision(t *testing.T) {
	root := referenceEnvironment(t)
	oldState := filepath.Join(root, "old-state")
	name := filepath.Join(oldState, "memory", "memory.db")
	if err := os.MkdirAll(filepath.Dir(name), 0700); err != nil {
		t.Fatal(err)
	}
	db, err := bbolt.Open(name, 0600, nil)
	if err != nil {
		t.Fatal(err)
	}
	err = db.Update(func(tx *bbolt.Tx) error {
		files, err := tx.CreateBucket([]byte("files"))
		if err != nil {
			return err
		}
		for _, path := range []string{filepath.Join(oldState, "memory", "teams", "one.md"), filepath.Join(paths.StateDir(), "memory", "teams", "one.md")} {
			if err := files.Put([]byte(path), []byte(`{"mod":123,"size":456}`)); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	before := referenceRead(t, name)
	move := referenceMove(t, oldState, paths.StateDir())
	if err := migration.RebaseReferences([]migration.Move{move}); err == nil {
		t.Fatal("colliding memory stamp keys falsely succeeded")
	}
	if !bytes.Equal(referenceRead(t, filepath.Join(paths.StateDir(), "memory", "memory.db")), before) {
		t.Fatal("memory database changed on collision")
	}
}

func TestRebaseReferencesRefusesMovingPathDerivedDecisionIdentity(t *testing.T) {
	root := referenceEnvironment(t)
	old := filepath.Join(root, "old-decisions")
	referenceJSON(t, filepath.Join(old, "existing-bank-id", "state.json"), map[string]any{"v": 1, "project": old})
	move := referenceMove(t, old, paths.DecisionsDir())
	before := referenceRead(t, filepath.Join(paths.DecisionsDir(), "existing-bank-id", "state.json"))
	if err := migration.RebaseReferences([]migration.Move{move}); err == nil {
		t.Fatal("path-derived decision bank identity silently orphaned")
	}
	if !bytes.Equal(referenceRead(t, filepath.Join(paths.DecisionsDir(), "existing-bank-id", "state.json")), before) {
		t.Fatal("decision metadata changed despite unresolved bank identity")
	}
}
