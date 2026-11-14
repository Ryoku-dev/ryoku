package main

import (
	"bufio"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// Dictation settings and models, for the Hub's Dictation page.
//
// The engine is `ryoku-dictate` (Ryoku's own, in ryoku/dictation). This file is
// the only writer of ~/.config/ryoku/dictation.conf and the model store: the
// page reads a snapshot, writes one key at a time, and the daemon picks the
// change up live over its socket.

// Path and file helpers shared by the Hub's writers. They lived beside the
// Voxtype backend this replaced.
func configHome() string {
	if b := os.Getenv("XDG_CONFIG_HOME"); b != "" {
		return b
	}
	return filepath.Join(os.Getenv("HOME"), ".config")
}

func dataHome() string {
	if b := os.Getenv("XDG_DATA_HOME"); b != "" {
		return b
	}
	return filepath.Join(os.Getenv("HOME"), ".local", "share")
}

func readFileString(path string) string {
	b, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return string(b)
}

func onPath(bin string) bool {
	_, err := exec.LookPath(bin)
	return err == nil
}

const dictationConfHeader = "# Ryoku dictation. Edited by Settings > Dictation; the daemon reads this file."

type dictationModel struct {
	ID        string `json:"id"`
	Label     string `json:"label"`
	Detail    string `json:"detail"`
	Language  string `json:"language"`
	WER       string `json:"wer"`
	Size      string `json:"size"`
	SizeBytes int64  `json:"sizeBytes"`
	Licence   string `json:"licence"`
	Bundled   bool   `json:"bundled"`
	Installed bool   `json:"installed"`
	Path      string `json:"path"`
	URL       string `json:"-"`
	SHA256    string `json:"-"`
	File      string `json:"-"`
}

type dictationSource struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Default     bool   `json:"default"`
}

type dictationSnapshot struct {
	Enabled     bool   `json:"enabled"`
	Source      string `json:"source"`
	Model       string `json:"model"`
	Device      string `json:"device"`
	Language    string `json:"language"`
	Mode        string `json:"mode"`
	MaxDuration int    `json:"maxDurationSecs"`
	SilenceMs   int    `json:"silenceMs"`
	SilenceDb   int    `json:"silenceDb"`
	TypeDelayMs int    `json:"typeDelayMs"`
	Engine      bool   `json:"engine"`
	// whether the engine package can be installed here, and why not: dictation
	// ships from the Ryoku repository on every edition, so this is a repository
	// question rather than an AUR one
	PackageAvailable bool   `json:"packageAvailable"`
	PackageReason    string `json:"packageReason"`
	GPU              string `json:"gpu"`
	Backends         string `json:"backends"`
	ActiveModel      string `json:"activeModel"`
	// the compute backend the running daemon reports, e.g. "Vulkan0" or "CPU"
	ActiveBackend  string            `json:"activeBackend"`
	Running        bool              `json:"running"`
	State          string            `json:"state"`
	Models         []dictationModel  `json:"models"`
	Sources        []dictationSource `json:"sources"`
	BundledModelID string            `json:"bundledModelId"`
}

func dictationConfPath() string {
	return filepath.Join(configHome(), "ryoku", "dictation.conf")
}

func dictationModelsDir() string {
	return filepath.Join(dataHome(), "ryoku", "dictation", "models")
}

func dictationBundledPath() string {
	return "/usr/share/ryoku/dictation/models/parakeet-tdt_ctc-110m-Q8_0.gguf"
}

// The catalogue is pinned: a model is a known file with a known hash, not a
// name to look up at runtime. WER is LibriSpeech test-clean at Q8_0 from the
// engine's own WER pipeline, so the numbers are comparable to each other.
func dictationCatalogue() []dictationModel {
	return []dictationModel{
		{
			ID: "parakeet-110m", Label: "Parakeet 110M", Language: "English",
			Detail: "The model Ryoku ships. Small, quick, and accurate enough for everyday dictation.",
			WER:    "2.43%", Size: "135 MB", SizeBytes: 135373280, Licence: "CC-BY-4.0",
			Bundled: true,
			URL:     "https://huggingface.co/handy-computer/parakeet-tdt_ctc-110m-gguf/resolve/main/parakeet-tdt_ctc-110m-Q8_0.gguf",
			SHA256:  "7dd44c74a331d788a4e5f8b16913b3feb29ced22cf5613aad0e0f6cd30516296",
			File:    "parakeet-tdt_ctc-110m-Q8_0.gguf",
		},
		{
			ID: "parakeet-v2", Label: "Parakeet 0.6B", Language: "English",
			Detail: "Markedly more accurate on English, still quick on a CPU. The everyday upgrade.",
			WER:    "1.69%", Size: "730 MB", SizeBytes: 729574912, Licence: "CC-BY-4.0",
			URL:    "https://huggingface.co/handy-computer/parakeet-tdt-0.6b-v2-gguf/resolve/main/parakeet-tdt-0.6b-v2-Q8_0.gguf",
			SHA256: "f0d0e99cebb6d3b83f1f7069b82b5d3c2e39a54545b0da039cb4bafd9c4e5caa",
			File:   "parakeet-tdt-0.6b-v2-Q8_0.gguf",
		},
		{
			ID: "parakeet-ultra", Label: "Parakeet Ultra", Language: "25 European languages",
			Detail: "Detects the language itself. The right choice when you dictate in more than English.",
			WER:    "1.80%", Size: "740 MB", SizeBytes: 740363168, Licence: "CC-BY-4.0",
			URL:    "https://huggingface.co/handy-computer/parakeet-ultra-gguf/resolve/main/parakeet-ultra-Q8_0.gguf",
			SHA256: "007a59761e9258779f189df396b50d12b83bb5b79e66df4b955c230d2f2a0a59",
			File:   "parakeet-ultra-Q8_0.gguf",
		},
		{
			ID: "whisper-turbo", Label: "Whisper Large v3 Turbo", Language: "About 100 languages",
			Detail: "Whisper's wide language coverage for anything outside Parakeet's set.",
			WER:    "2.01%", Size: "886 MB", SizeBytes: 886381728, Licence: "MIT",
			URL:    "https://huggingface.co/handy-computer/whisper-large-v3-turbo-gguf/resolve/main/whisper-large-v3-turbo-Q8_0.gguf",
			SHA256: "257aa7ec9121c22c167d5e2c49e3e76b4ba0e6e27ebc13074eba0a80f8190220",
			File:   "whisper-large-v3-turbo-Q8_0.gguf",
		},
	}
}

func runDictation(args []string) error {
	if len(args) == 0 {
		return errors.New("dictation needs get|set|models|install|remove")
	}
	switch args[0] {
	case "get":
		return dictationGet()
	case "set":
		if len(args) < 2 {
			return errors.New("dictation set needs a JSON argument")
		}
		return dictationSet(args[1])
	case "models":
		return dictationModels()
	case "install":
		if len(args) < 2 {
			return errors.New("dictation install needs a model id")
		}
		return dictationInstall(args[1])
	case "remove":
		if len(args) < 2 {
			return errors.New("dictation remove needs a model id")
		}
		return dictationRemove(args[1])
	default:
		return fmt.Errorf("unknown dictation command: %s", args[0])
	}
}

func dictationValues() map[string]string {
	values := map[string]string{}
	for _, line := range strings.Split(readFileString(dictationConfPath()), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, value, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		values[strings.TrimSpace(key)] = strings.TrimSpace(value)
	}
	return values
}

func dictationInt(values map[string]string, key string, fallback int) int {
	if parsed, err := strconv.Atoi(values[key]); err == nil {
		return parsed
	}
	return fallback
}

func dictationGet() error {
	values := dictationValues()
	models := dictationCatalogue()
	for i := range models {
		path := filepath.Join(dictationModelsDir(), models[i].File)
		models[i].Path = path
		if models[i].Bundled {
			models[i].Path = dictationBundledPath()
			models[i].Installed = fileExists(models[i].Path)
			continue
		}
		models[i].Installed = fileExists(path)
		if !models[i].Installed {
			models[i].Path = ""
		}
	}

	snapshot := dictationSnapshot{
		Enabled:        values["enabled"] == "true" || values["enabled"] == "1" || values["enabled"] == "yes" || values["enabled"] == "on",
		Source:         values["source"],
		Model:          values["model"],
		Device:         orDefault(values["device"], "auto"),
		Language:       values["language"],
		Mode:           orDefault(values["mode"], "paste"),
		MaxDuration:    dictationInt(values, "max_duration_secs", 120),
		SilenceMs:      dictationInt(values, "silence_ms", 1500),
		SilenceDb:      dictationInt(values, "silence_db", -45),
		TypeDelayMs:    dictationInt(values, "type_delay_ms", 2),
		Engine:         onPath("ryoku-dictate"),
		Models:         models,
		Sources:        dictationSources(),
		BundledModelID: "parakeet-110m",
	}
	snapshot.Backends, snapshot.GPU = dictationBackends()
	snapshot.PackageAvailable, snapshot.PackageReason = dictationPackageAvailability()
	snapshot.ActiveModel, snapshot.State, snapshot.Running, snapshot.ActiveBackend = dictationStatus()
	body, err := json.Marshal(snapshot)
	if err != nil {
		return err
	}
	fmt.Println(string(body))
	return nil
}

func orDefault(value, fallback string) string {
	if value == "" {
		return fallback
	}
	return value
}

// dictationBackends asks the binary which compute backends it was built with and
// which one is actually present. A GPU toggle that cannot light up is worse than
// no toggle, so the page shows exactly this.
func dictationBackends() (backends, gpu string) {
	out, err := exec.Command("ryoku-dictate", "--print-config").Output()
	if err != nil {
		return "cpu", "none"
	}
	for _, line := range strings.Split(string(out), "\n") {
		if strings.HasPrefix(line, "backend=") {
			backends = strings.TrimSpace(strings.TrimPrefix(line, "backend="))
		}
	}
	backends = orDefault(backends, "cpu")
	for _, candidate := range []string{"vulkan", "cuda", "rocm"} {
		if strings.Contains(backends, candidate) {
			return backends, candidate
		}
	}
	return backends, "none"
}

func dictationSources() []dictationSource {
	out, err := exec.Command("pactl", "list", "short", "sources").Output()
	if err != nil {
		return nil
	}
	def, _ := exec.Command("pactl", "get-default-source").Output()
	defaultName := strings.TrimSpace(string(def))
	var sources []dictationSource
	for _, line := range strings.Split(string(out), "\n") {
		fields := strings.Split(line, "\t")
		if len(fields) < 2 {
			continue
		}
		name := fields[1]
		if strings.HasSuffix(name, ".monitor") {
			continue
		}
		description := strings.TrimSpace(strings.Join(fields[2:], " "))
		if description == "" {
			description = name
		}
		sources = append(sources, dictationSource{
			Name:        name,
			Description: description,
			Default:     name == defaultName,
		})
	}
	return sources
}

// dictationPackageAvailability asks the host seam whether the engine can be
// installed here, so the page can say why it cannot rather than offering a
// button that fails.
func dictationPackageAvailability() (bool, string) {
	// "available" is a repository question and "installed" is a local one: a dev
	// box runs the checkout and has the package installed without the [ryoku]
	// repo synced, and either way the page may offer install or remove.
	if exec.Command("ryoku-host", "pkg", "available", "ryoku-dictation").Run() == nil ||
		exec.Command("ryoku-host", "pkg", "installed", "ryoku-dictation").Run() == nil {
		return true, ""
	}
	out, err := exec.Command("ryoku-host", "pkg", "why", "ryoku-dictation").Output()
	if err != nil {
		return false, ""
	}
	reason, _, _ := strings.Cut(strings.TrimSpace(string(out)), "\n")
	return false, strings.TrimSpace(reason)
}

func dictationStatus() (model, state string, running bool, backend string) {
	conn, err := net.DialTimeout("unix", dictationSocketPath(), 300*time.Millisecond)
	if err != nil {
		return "", "stopped", false, ""
	}
	defer conn.Close()
	_ = conn.SetDeadline(time.Now().Add(time.Second))
	if _, err := conn.Write([]byte("status\n")); err != nil {
		return "", "stopped", false, ""
	}
	buffer := make([]byte, 4096)
	n, _ := conn.Read(buffer)
	for _, line := range strings.Split(string(buffer[:n]), "\n") {
		key, value, ok := strings.Cut(strings.TrimSpace(line), " ")
		if !ok {
			continue
		}
		switch key {
		case "model":
			model = value
		case "state":
			state = value
		case "backend":
			backend = value
		}
	}
	return model, orDefault(state, "stopped"), true, backend
}

func dictationSocketPath() string {
	runtime := os.Getenv("XDG_RUNTIME_DIR")
	if runtime == "" {
		runtime = os.TempDir()
	}
	return filepath.Join(runtime, "ryoku-dictate.sock")
}

// dictationNotify pushes a changed key to a running daemon. A stopped daemon
// reads the file when it next starts, so a failure here is not an error.
func dictationNotify(line string) {
	conn, err := net.DialTimeout("unix", dictationSocketPath(), 300*time.Millisecond)
	if err != nil {
		return
	}
	defer conn.Close()
	_ = conn.SetDeadline(time.Now().Add(time.Second))
	if _, err := conn.Write([]byte(line + "\n")); err != nil {
		return
	}
	reader := bufio.NewScanner(conn)
	for reader.Scan() {
		if reply := reader.Text(); reply == "ok" || strings.HasPrefix(reply, "error ") {
			return
		}
	}
}

func dictationSet(raw string) error {
	var update struct {
		Enabled     *bool   `json:"enabled"`
		Source      *string `json:"source"`
		Model       *string `json:"model"`
		Device      *string `json:"device"`
		Language    *string `json:"language"`
		Mode        *string `json:"mode"`
		MaxDuration *int    `json:"maxDurationSecs"`
		SilenceMs   *int    `json:"silenceMs"`
		SilenceDb   *int    `json:"silenceDb"`
		TypeDelayMs *int    `json:"typeDelayMs"`
	}
	if err := json.Unmarshal([]byte(raw), &update); err != nil {
		return err
	}

	changes := map[string]string{}
	if update.Enabled != nil {
		changes["enabled"] = strconv.FormatBool(*update.Enabled)
	}
	if update.Source != nil {
		changes["source"] = *update.Source
	}
	if update.Model != nil {
		changes["model"] = *update.Model
	}
	if update.Device != nil {
		if *update.Device != "auto" && *update.Device != "cpu" && *update.Device != "gpu" {
			return errors.New("device must be auto, cpu or gpu")
		}
		if *update.Device == "gpu" {
			if _, gpu := dictationBackends(); gpu == "none" {
				return errors.New("no GPU backend is installed; install ryoku-dictation-vulkan first")
			}
		}
		changes["device"] = *update.Device
	}
	if update.Language != nil {
		changes["language"] = *update.Language
	}
	if update.Mode != nil {
		if *update.Mode != "paste" && *update.Mode != "type" {
			return errors.New("mode must be paste or type")
		}
		changes["mode"] = *update.Mode
	}
	if update.MaxDuration != nil {
		changes["max_duration_secs"] = strconv.Itoa(*update.MaxDuration)
	}
	if update.SilenceMs != nil {
		changes["silence_ms"] = strconv.Itoa(*update.SilenceMs)
	}
	if update.SilenceDb != nil {
		changes["silence_db"] = strconv.Itoa(*update.SilenceDb)
	}
	if update.TypeDelayMs != nil {
		changes["type_delay_ms"] = strconv.Itoa(*update.TypeDelayMs)
	}
	if err := dictationWriteConf(changes); err != nil {
		return err
	}

	live := []string{"enabled", "device", "mode", "language", "model"}
	for _, key := range live {
		if value, ok := changes[key]; ok {
			dictationNotify("set " + key + " " + orDefault(value, "auto"))
		}
	}
	if update.Enabled != nil {
		_ = daemonCall("dictation.status", nil, nil)
	}
	return dictationGet()
}

// dictationWriteConf edits the settings file in place: a changed key keeps its
// line, a new key is appended, and every comment and unknown key survives.
func dictationWriteConf(changes map[string]string) error {
	path := dictationConfPath()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	seen := map[string]bool{}
	var lines []string
	existing := readFileString(path)
	if strings.TrimSpace(existing) == "" {
		lines = append(lines, dictationConfHeader)
	} else {
		lines = strings.Split(strings.TrimSuffix(existing, "\n"), "\n")
	}
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		key, _, ok := strings.Cut(trimmed, "=")
		if !ok {
			continue
		}
		key = strings.TrimSpace(key)
		if value, changing := changes[key]; changing {
			lines[i] = key + "=" + value
			seen[key] = true
		}
	}
	for key, value := range changes {
		if !seen[key] {
			lines = append(lines, key+"="+value)
		}
	}
	body := strings.Join(lines, "\n") + "\n"
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, []byte(body), 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

func dictationModels() error {
	models := dictationCatalogue()
	for i := range models {
		path := filepath.Join(dictationModelsDir(), models[i].File)
		if models[i].Bundled {
			models[i].Installed = fileExists(dictationBundledPath())
			models[i].Path = dictationBundledPath()
			continue
		}
		models[i].Installed = fileExists(path)
		if models[i].Installed {
			models[i].Path = path
		}
	}
	body, err := json.Marshal(models)
	if err != nil {
		return err
	}
	fmt.Println(string(body))
	return nil
}

func dictationInstall(id string) error {
	var chosen *dictationModel
	for _, model := range dictationCatalogue() {
		if model.ID == id {
			copy := model
			chosen = &copy
			break
		}
	}
	if chosen == nil {
		return fmt.Errorf("unknown model: %s", id)
	}
	if chosen.Bundled {
		return errors.New("the bundled model ships with ryoku-dictation and is always installed")
	}

	dir := dictationModelsDir()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	final := filepath.Join(dir, chosen.File)
	if err := dictationDownloadFile(chosen.URL, chosen.SHA256, chosen.SizeBytes, final, os.Stdout); err != nil {
		return err
	}
	// The model the user just installed is the one they mean to use.
	if err := dictationWriteConf(map[string]string{"model": final}); err != nil {
		return err
	}
	dictationNotify("set model " + final)
	fmt.Println("installed")
	return nil
}

// dictationDownloadFile fetches a model into a staging name, checks its hash and
// only then renames it into place, so a cancelled or corrupt transfer can never
// become the model the daemon loads. Percent lines go to progress for the page.
func dictationDownloadFile(url, want string, size int64, final string, progress io.Writer) error {
	staging := final + ".part"
	response, err := http.Get(url)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return fmt.Errorf("download failed: %s", response.Status)
	}
	file, err := os.Create(staging)
	if err != nil {
		return err
	}
	hasher := sha256.New()
	written := int64(0)
	buffer := make([]byte, 256*1024)
	lastPercent := -1
	for {
		n, readErr := response.Body.Read(buffer)
		if n > 0 {
			if _, err := file.Write(buffer[:n]); err != nil {
				file.Close()
				os.Remove(staging)
				return err
			}
			hasher.Write(buffer[:n])
			written += int64(n)
			if size > 0 {
				percent := int(written * 100 / size)
				if percent != lastPercent {
					lastPercent = percent
					fmt.Fprintf(progress, "progress %d\n", percent)
				}
			}
		}
		if readErr == io.EOF {
			break
		}
		if readErr != nil {
			file.Close()
			os.Remove(staging)
			return readErr
		}
	}
	if err := file.Close(); err != nil {
		os.Remove(staging)
		return err
	}
	sum := hex.EncodeToString(hasher.Sum(nil))
	if sum != want {
		os.Remove(staging)
		return fmt.Errorf("checksum mismatch: downloaded %s", sum)
	}
	return os.Rename(staging, final)
}

func dictationRemove(id string) error {
	var chosen *dictationModel
	for _, model := range dictationCatalogue() {
		if model.ID == id {
			copy := model
			chosen = &copy
			break
		}
	}
	if chosen == nil {
		return fmt.Errorf("unknown model: %s", id)
	}
	if chosen.Bundled {
		return errors.New("the bundled model belongs to the ryoku-dictation package")
	}
	path := filepath.Join(dictationModelsDir(), chosen.File)
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return err
	}
	if dictationValues()["model"] == path {
		if err := dictationWriteConf(map[string]string{"model": ""}); err != nil {
			return err
		}
		dictationNotify("set model ")
	}
	fmt.Println("removed")
	return nil
}
