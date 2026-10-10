package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// Voxtype dictation backend for Ryoku Settings' Dictation page. Voxtype owns a
// large config; Ryoku writes a minimal, schema-valid ~/.config/voxtype/config.toml
// with the built-in hotkey off (the shell owns Super+` and drives `voxtype
// record`), pins the Whisper engine + model, and manages the user service.
// Models download and remove in-process, so the Hub never shells out to a
// terminal. Only engines that work without a root binary swap are offered:
// local Whisper and Whisper through an OpenAI-compatible API.
//
//	ryoku-hub voxtype get             presets + current state as JSON
//	ryoku-hub voxtype set <json>      apply a preset (writes config, manages service)
//	ryoku-hub voxtype ensure          seed a default config + service (autostart)
//	ryoku-hub voxtype download <key>  download a preset's model (streams progress)
//	ryoku-hub voxtype rmmodel <key>   delete a preset's downloaded model

// voxtypePreset is one dictation option the page offers. Lower-case fields map
// it to config.toml and the model store; exported ones drive the UI.
type voxtypePreset struct {
	Key       string `json:"key"`
	Label     string `json:"label"`
	Provider  string `json:"provider"`
	Detail    string `json:"detail"`
	Size      string `json:"size"`
	SizeBytes int64  `json:"sizeBytes"`
	Group     string `json:"group"`
	Language  string `json:"language"`
	Speed     string `json:"speed"`
	Cloud     bool   `json:"cloud"`   // sends audio off-device via a remote API
	KeyKind   string `json:"keyKind"` // "openai" or ""
	Present   bool   `json:"present"` // local model on disk (get only; cloud = true)

	model string // whisper model name (local) or remote_model (cloud)
	lang  string // whisper language
}

// The curated set. Every entry uses the default Whisper binary, so selecting one
// never needs sudo or a binary swap. Parakeet and the ONNX engines are omitted:
// switching to them rewrites the root-owned /usr/bin/voxtype symlink, which the
// Hub cannot do without a terminal sudo prompt.
func voxtypePresets() []voxtypePreset {
	return []voxtypePreset{
		{Key: "whisper-tiny-en", Label: "Tiny English", Provider: "Whisper", Detail: "English only. The smallest and fastest local model.", Size: "39 MB", SizeBytes: 39_000_000, Group: "english", Language: "English", Speed: "fastest", model: "tiny.en", lang: "en"},
		{Key: "whisper-fast", Label: "Base English", Provider: "Whisper", Detail: "English only. A quick everyday model with a little more accuracy.", Size: "142 MB", SizeBytes: 142_000_000, Group: "english", Language: "English", Speed: "fast", model: "base.en", lang: "en"},
		{Key: "whisper-small-en", Label: "Small English", Provider: "Whisper", Detail: "English only. The best balance of speed and accuracy for English dictation.", Size: "466 MB", SizeBytes: 466_000_000, Group: "english", Language: "English", Speed: "balanced", model: "small.en", lang: "en"},
		{Key: "whisper-base", Label: "Base Multilingual", Provider: "Whisper", Detail: "A lightweight local model that detects any of Whisper's 99+ languages.", Size: "142 MB", SizeBytes: 142_000_000, Group: "multilingual", Language: "99+ languages", Speed: "fast", model: "base", lang: "auto"},
		{Key: "whisper-medium", Label: "Medium Multilingual", Provider: "Whisper", Detail: "Higher accuracy across 99+ languages, with a larger download and slower response.", Size: "1.5 GB", SizeBytes: 1_500_000_000, Group: "multilingual", Language: "99+ languages", Speed: "quality", model: "medium", lang: "auto"},
		{Key: "whisper-accurate", Label: "Large Turbo", Provider: "Whisper", Detail: "Fast, high-accuracy multilingual dictation for more capable hardware.", Size: "1.6 GB", SizeBytes: 1_600_000_000, Group: "multilingual", Language: "99+ languages", Speed: "fast + accurate", model: "large-v3-turbo", lang: "auto"},
		{Key: "openai", Label: "OpenAI API", Provider: "OpenAI", Detail: "Cloud Whisper through OpenAI's API. Needs a key; audio leaves your machine.", Size: "cloud", Group: "cloud", Language: "99+ languages", Speed: "cloud", Cloud: true, KeyKind: "openai", model: "whisper-1", lang: "en"},
	}
}

func presetByKey(k string) (voxtypePreset, bool) {
	for _, p := range voxtypePresets() {
		if p.Key == k {
			return p, true
		}
	}
	return voxtypePreset{}, false
}

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

func voxtypeConfigPath() string { return filepath.Join(configHome(), "voxtype", "config.toml") }
func voxtypeUnitPath() string {
	return filepath.Join(configHome(), "systemd", "user", "voxtype.service")
}

// whisper models land in ~/.local/share/voxtype/models as ggml-<model>.bin.
func modelFilePath(p voxtypePreset) string {
	return filepath.Join(dataHome(), "voxtype", "models", "ggml-"+p.model+".bin")
}

func runVoxtype(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("voxtype needs get|set|ensure|download|rmmodel")
	}
	switch args[0] {
	case "get":
		return voxtypeGet()
	case "set":
		if len(args) < 2 {
			return fmt.Errorf("voxtype set needs a JSON argument")
		}
		return voxtypeSet(args[1])
	case "ensure":
		return voxtypeEnsure()
	case "download":
		if len(args) < 2 {
			return fmt.Errorf("voxtype download needs a preset key")
		}
		return voxtypeDownload(args[1])
	case "rmmodel":
		if len(args) < 2 {
			return fmt.Errorf("voxtype rmmodel needs a preset key")
		}
		return voxtypeRmModel(args[1])
	default:
		return fmt.Errorf("unknown voxtype command: %s", args[0])
	}
}

func voxtypeGet() error {
	text := readFileString(voxtypeConfigPath())
	presets := voxtypePresets()
	for i := range presets {
		presets[i].Present = presets[i].Cloud || fileExists(modelFilePath(presets[i]))
	}
	out := map[string]any{
		"installed":    onPath("voxtype"),
		"selected":     selectedPreset(text),
		"enabled":      voxtypeServiceEnabled(),
		"openaiKeySet": extractConfigString(text, "remote_api_key") != "" || os.Getenv("VOXTYPE_WHISPER_API_KEY") != "",
		"presets":      presets,
	}
	b, err := json.Marshal(out)
	if err != nil {
		return err
	}
	os.Stdout.Write(b)
	fmt.Println()
	return nil
}

type voxtypeSetReq struct {
	Preset    string `json:"preset"`
	Enabled   bool   `json:"enabled"`
	OpenAIKey string `json:"openaiKey"` // empty keeps the stored key
}

// voxtypeSet writes a valid config.toml for the chosen preset and brings the
// service to the requested state. All presets use the Whisper binary, so there
// is no engine/binary swap.
func voxtypeSet(arg string) error {
	var req voxtypeSetReq
	if err := json.Unmarshal([]byte(arg), &req); err != nil {
		return fmt.Errorf("bad JSON: %w", err)
	}
	p, ok := presetByKey(req.Preset)
	if !ok {
		return fmt.Errorf("unknown preset: %s", req.Preset)
	}
	openaiKey := req.OpenAIKey
	if openaiKey == "" {
		openaiKey = extractConfigString(readFileString(voxtypeConfigPath()), "remote_api_key")
	}
	if err := atomicWrite(voxtypeConfigPath(), []byte(buildVoxtypeConfig(p, openaiKey)), 0o600); err != nil {
		return err
	}
	if req.Enabled {
		ensureVoxtypeUnit()
		_ = userctl("enable", "voxtype.service")
		_ = userctl("restart", "voxtype.service")
	} else {
		_ = userctl("disable", "--now", "voxtype.service")
	}
	fmt.Println("ok")
	return nil
}

// voxtypeEnsure seeds a default config and the user service on first run. It is
// idempotent so the session startup can call it every login, and never forces
// the service on when the user turned dictation off in the Hub.
func voxtypeEnsure() error {
	if !onPath("voxtype") {
		return nil
	}
	if _, err := os.Stat(voxtypeConfigPath()); err != nil {
		p, _ := presetByKey("whisper-fast")
		if err := atomicWrite(voxtypeConfigPath(), []byte(buildVoxtypeConfig(p, "")), 0o600); err != nil {
			return err
		}
	}
	ensureVoxtypeUnit()
	if voxtypeServiceEnabled() {
		_ = userctl("start", "voxtype.service")
	}
	return nil
}

// voxtypeDownload fetches a preset's Whisper model, streaming voxtype's progress
// to stdout so the Hub can show it. Blocks until the download finishes.
func voxtypeDownload(key string) error {
	p, ok := presetByKey(key)
	if !ok {
		return fmt.Errorf("unknown preset: %s", key)
	}
	if p.Cloud || p.model == "" {
		return fmt.Errorf("%s has no downloadable model", key)
	}
	cmd := exec.Command("voxtype", "setup", "--download", "--model", p.model, "--no-post-install")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return err
	}
	if !fileExists(modelFilePath(p)) {
		return fmt.Errorf("voxtype did not create %s", modelFilePath(p))
	}
	return nil
}

// voxtypeRmModel deletes a preset's downloaded model file.
func voxtypeRmModel(key string) error {
	p, ok := presetByKey(key)
	if !ok {
		return fmt.Errorf("unknown preset: %s", key)
	}
	if p.Cloud {
		return fmt.Errorf("%s has no local model", key)
	}
	if err := os.Remove(modelFilePath(p)); err != nil && !os.IsNotExist(err) {
		return err
	}
	fmt.Println("ok")
	return nil
}

// buildVoxtypeConfig renders a minimal, schema-valid config.toml for the preset,
// matching Voxtype's own default layout (as shipped by Omarchy): built-in hotkey
// off, type mode, media paused while recording. A preset marker lets the Hub read
// the selection back. No modifier-suppress submap is needed: Super+` is a tap, so
// the keys are released well before Voxtype types.
func buildVoxtypeConfig(p voxtypePreset, openaiKey string) string {
	var b strings.Builder
	b.WriteString("# Ryoku dictation config for Voxtype. Managed by Ryoku Settings > Dictation\n")
	b.WriteString("# (ryoku-hub voxtype); hand edits are overwritten when you change it there.\n")
	b.WriteString("# The shell owns Super+` and the pill mic-wave, so the built-in hotkey is off.\n")
	fmt.Fprintf(&b, "# ryoku-preset: %s\n\n", p.Key)
	b.WriteString("state_file = \"auto\"\n\n")
	b.WriteString("[hotkey]\nenabled = false\n\n")
	b.WriteString("[audio]\ndevice = \"default\"\nsample_rate = 16000\nmax_duration_secs = 60\npause_media = true\n\n")
	b.WriteString("[whisper]\n")
	if p.Cloud {
		b.WriteString("mode = \"remote\"\n")
		b.WriteString("model = \"base.en\"\n")
		b.WriteString("language = \"en\"\n")
		b.WriteString("translate = false\n")
		b.WriteString("remote_endpoint = \"https://api.openai.com/v1\"\n")
		fmt.Fprintf(&b, "remote_model = %q\n", p.model)
		if openaiKey != "" {
			fmt.Fprintf(&b, "remote_api_key = %q\n", openaiKey)
		}
	} else {
		fmt.Fprintf(&b, "model = %q\n", p.model)
		fmt.Fprintf(&b, "language = %q\n", p.lang)
		b.WriteString("translate = false\n")
	}
	b.WriteString("\n[output]\nmode = \"type\"\nfallback_to_clipboard = true\ntype_delay_ms = 1\n\n")
	b.WriteString("[output.notification]\non_recording_start = false\non_recording_stop = false\non_transcription = false\n\n")
	// the pill's mic wave is the indicator; keep Voxtype's own OSD off (Ryoku does
	// not ship gtk4-layer-shell, which the OSD would need).
	b.WriteString("[osd]\nenabled = false\n")
	return b.String()
}

// --- small helpers -------------------------------------------------------

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

func selectedPreset(configText string) string {
	for _, ln := range strings.Split(configText, "\n") {
		ln = strings.TrimSpace(ln)
		if strings.HasPrefix(ln, "# ryoku-preset:") {
			return strings.TrimSpace(strings.TrimPrefix(ln, "# ryoku-preset:"))
		}
	}
	return "whisper-fast"
}

// extractConfigString pulls a simple `key = "value"` string out of the config.
func extractConfigString(configText, key string) string {
	for _, ln := range strings.Split(configText, "\n") {
		t := strings.TrimSpace(ln)
		if strings.HasPrefix(t, key+" =") || strings.HasPrefix(t, key+"=") {
			v := strings.TrimSpace(t[strings.Index(t, "=")+1:])
			return strings.Trim(v, "\"")
		}
	}
	return ""
}

func userctl(args ...string) error {
	return exec.Command("systemctl", append([]string{"--user"}, args...)...).Run()
}

func voxtypeServiceEnabled() bool {
	return userctl("is-enabled", "--quiet", "voxtype.service") == nil
}

// ensureVoxtypeUnit installs the user service (voxtype setup systemd writes and
// enables the unit) and adds a resilience drop-in: StartLimitIntervalSec=0 so a
// crash-loop (the mic getting killed over and over) can't trip systemd's restart
// rate-limit and leave dictation dead until the next login.
func ensureVoxtypeUnit() {
	if !onPath("voxtype") {
		return
	}
	if !fileExists(voxtypeUnitPath()) {
		_ = exec.Command("voxtype", "setup", "systemd").Run()
	}
	dropin := filepath.Join(configHome(), "systemd", "user", "voxtype.service.d", "ryoku-resilient.conf")
	want := "# Ryoku: never give up restarting the daemon; a mic-capture crash-loop\n" +
		"# must not permanently disable dictation.\n" +
		"[Unit]\nStartLimitIntervalSec=0\n\n[Service]\nRestart=always\nRestartSec=2\n"
	if readFileString(dropin) != want && atomicWrite(dropin, []byte(want), 0o644) == nil {
		_ = userctl("daemon-reload")
	}
}
