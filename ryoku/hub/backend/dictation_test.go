package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func isolate(t *testing.T) (conf string) {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
	t.Setenv("XDG_DATA_HOME", filepath.Join(home, ".local", "share"))
	t.Setenv("XDG_RUNTIME_DIR", filepath.Join(home, "run"))
	t.Setenv("PATH", t.TempDir())
	return dictationConfPath()
}

func TestDictationWriteConfKeepsCommentsAndUnknownKeys(t *testing.T) {
	conf := isolate(t)
	if err := os.MkdirAll(filepath.Dir(conf), 0o755); err != nil {
		t.Fatal(err)
	}
	original := dictationConfHeader + "\n" +
		"# a note the page must not lose\n" +
		"enabled=true\n" +
		"mode=type\n" +
		"someone_elses_key=keepme\n"
	if err := os.WriteFile(conf, []byte(original), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := dictationWriteConf(map[string]string{"mode": "paste", "silence_ms": "900"}); err != nil {
		t.Fatal(err)
	}
	after := readFileString(conf)

	if !strings.Contains(after, "# a note the page must not lose") {
		t.Errorf("comment lost:\n%s", after)
	}
	if !strings.Contains(after, "someone_elses_key=keepme") {
		t.Errorf("unknown key lost:\n%s", after)
	}
	if !strings.Contains(after, "mode=paste") {
		t.Errorf("changed key not written:\n%s", after)
	}
	if strings.Contains(after, "mode=type") {
		t.Errorf("old value still present:\n%s", after)
	}
	if !strings.Contains(after, "silence_ms=900") {
		t.Errorf("new key not appended:\n%s", after)
	}
	if strings.Count(after, "mode=") != 1 {
		t.Errorf("key duplicated:\n%s", after)
	}

	values := dictationValues()
	if values["mode"] != "paste" || values["silence_ms"] != "900" || values["enabled"] != "true" {
		t.Fatalf("read back %+v", values)
	}
}

func TestDictationSetRejectsBadValues(t *testing.T) {
	isolate(t)

	if err := dictationSet(`{"mode":"sing"}`); err == nil {
		t.Error("an unknown mode was accepted")
	}
	// No GPU backend exists on this test box (the engine is not even installed),
	// so asking for the GPU must be refused rather than quietly running on the CPU.
	if err := dictationSet(`{"device":"gpu"}`); err == nil {
		t.Error("gpu was accepted with no backend installed")
	} else if !strings.Contains(err.Error(), "GPU") {
		t.Errorf("gpu refusal should name the missing backend, got %q", err)
	}
	if err := dictationSet(`{"device":"cpu"}`); err != nil {
		t.Errorf("cpu refused: %v", err)
	}
	if values := dictationValues(); values["device"] != "cpu" {
		t.Errorf("device not persisted: %+v", values)
	}
	if err := dictationSet(`{"device":"quantum"}`); err == nil {
		t.Error("an unknown device was accepted")
	}
}

func TestDictationDownloadVerifiesBeforePublishing(t *testing.T) {
	body := strings.Repeat("parakeet weights", 512)
	sum := sha256.Sum256([]byte(body))
	hash := hex.EncodeToString(sum[:])

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(body))
	}))
	defer server.Close()

	dir := t.TempDir()
	final := filepath.Join(dir, "model.gguf")
	var progress strings.Builder

	if err := dictationDownloadFile(server.URL, hash, int64(len(body)), final, &progress); err != nil {
		t.Fatalf("download: %v", err)
	}
	if _, err := os.Stat(final); err != nil {
		t.Fatalf("model not published: %v", err)
	}
	if _, err := os.Stat(final + ".part"); !os.IsNotExist(err) {
		t.Error("staging file survived a successful install")
	}
	lines := strings.Split(strings.TrimSpace(progress.String()), "\n")
	if len(lines) == 0 || lines[len(lines)-1] != "progress 100" {
		t.Errorf("progress never reached 100: %q", progress.String())
	}

	// A wrong hash must leave nothing at the final name and no staging behind:
	// a half-trusted model is worse than no model.
	badDir := t.TempDir()
	badFinal := filepath.Join(badDir, "model.gguf")
	err := dictationDownloadFile(server.URL, strings.Repeat("0", 64), int64(len(body)), badFinal, &progress)
	if err == nil || !strings.Contains(err.Error(), "checksum mismatch") {
		t.Fatalf("bad checksum accepted: %v", err)
	}
	if _, err := os.Stat(badFinal); !os.IsNotExist(err) {
		t.Error("a model with the wrong hash was published")
	}
	if _, err := os.Stat(badFinal + ".part"); !os.IsNotExist(err) {
		t.Error("the staging file of a failed install was left behind")
	}

	notFound := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.NotFound(w, r)
	}))
	defer notFound.Close()
	missingDir := t.TempDir()
	missingFinal := filepath.Join(missingDir, "model.gguf")
	if err := dictationDownloadFile(notFound.URL, hash, 10, missingFinal, &progress); err == nil {
		t.Error("a 404 was accepted")
	}
	if _, err := os.Stat(missingFinal + ".part"); !os.IsNotExist(err) {
		t.Error("a refused download left a staging file")
	}
}

func TestDictationRemoveRefusesTheBundledModel(t *testing.T) {
	isolate(t)
	if err := dictationRemove("parakeet-110m"); err == nil {
		t.Error("the bundled model was removed from under the package")
	} else if !strings.Contains(err.Error(), "package") {
		t.Errorf("removal refusal should explain why, got %q", err)
	}
	if err := dictationRemove("no-such-model"); err == nil {
		t.Error("an unknown model id was accepted")
	}
}

func TestDictationGetReportsTheEngine(t *testing.T) {
	conf := isolate(t)
	if err := os.MkdirAll(filepath.Dir(conf), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(conf, []byte("enabled=false\ndevice=cpu\nmode=type\nlanguage=de\nsilence_ms=0\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	// A page reload must show what the file says even with no daemon running,
	// and must say the engine is missing rather than pretending otherwise.
	stdout := os.Stdout
	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stdout = writer
	getErr := dictationGet()
	_ = writer.Close()
	os.Stdout = stdout
	if getErr != nil {
		t.Fatalf("get: %v", getErr)
	}
	captured, err := io.ReadAll(reader)
	if err != nil {
		t.Fatal(err)
	}

	var snapshot dictationSnapshot
	if err := json.Unmarshal(captured, &snapshot); err != nil {
		t.Fatalf("get printed %q: %v", captured, err)
	}
	if snapshot.Enabled {
		t.Error("enabled=false was not reported")
	}
	if snapshot.Device != "cpu" || snapshot.Mode != "type" || snapshot.Language != "de" {
		t.Errorf("settings not reported: %+v", snapshot)
	}
	if snapshot.SilenceMs != 0 {
		t.Errorf("silence_ms=0 read back as %d", snapshot.SilenceMs)
	}
	if snapshot.Engine {
		t.Error("the engine was reported installed with nothing on PATH")
	}
	if snapshot.BundledModelID == "" {
		t.Error("no bundled model named")
	}
	if len(snapshot.Models) != 4 {
		t.Errorf("catalogue has %d entries, want 4", len(snapshot.Models))
	}
}

func TestDictationRequiresExplicitOptIn(t *testing.T) {
	isolate(t)
	snapshot := func() dictationSnapshot {
		t.Helper()
		out := captureStdout(t, func() {
			if err := dictationGet(); err != nil {
				t.Fatal(err)
			}
		})
		var state dictationSnapshot
		if err := json.Unmarshal([]byte(out), &state); err != nil {
			t.Fatal(err)
		}
		return state
	}
	if snapshot().Enabled {
		t.Fatal("a new user must not have voice typing enabled")
	}
	for _, step := range []struct {
		update  string
		enabled bool
	}{
		{`{"mode":"type"}`, false},
		{`{"enabled":true}`, true},
		{`{"device":"cpu"}`, true},
		{`{"enabled":false}`, false},
	} {
		captureStdout(t, func() {
			if err := dictationSet(step.update); err != nil {
				t.Fatal(err)
			}
		})
		if got := snapshot().Enabled; got != step.enabled {
			t.Fatalf("after %s: enabled=%t, want %t", step.update, got, step.enabled)
		}
	}
}
