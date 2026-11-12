package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"
)

// record is the one control surface for the screen recorder: `ryoku-shell record
// start|stop|status|info|pause|resume|settings`, all handed to ryoku-cmd-record
// except settings, which the daemon owns because it is the sole writer of
// recording.json. A start is spawned detached in its own session so the capture
// outlives a daemon reload; the other verbs run synchronously and return the
// reply. A bare `record` toggles: stop when a capture is live, else start a
// fullscreen one.
func (d *daemon) record(args []string) string {
	if len(args) == 0 {
		if recorderRunning() {
			return runRecord([]string{"stop"})
		}
		spawnRecord([]string{"start"})
		return "ok"
	}

	switch args[0] {
	case "stop", "--stop":
		return runRecord([]string{"stop"})
	case "status", "--status":
		return runRecord([]string{"status"})
	case "info", "--info":
		return runRecord([]string{"info"})
	case "pause":
		return runRecord([]string{"pause"})
	case "resume":
		return runRecord([]string{"resume"})
	case "settings":
		return recordSettingsCLI(args[1:])
	case "start", "--fullscreen", "--region", "-r", "--monitor",
		"--with-desktop-audio", "--with-microphone-audio", "--discord":
		// a start: hand the flags straight to the recorder, prefixing the start
		// verb so `record --region ...` and `record start --region ...` behave the
		// same. The tokens are already split by the socket transport and reach the
		// script as a plain argv (no shell), so nothing here can be re-parsed.
		rest := args
		if args[0] == "start" {
			rest = args[1:]
		}
		spawnRecord(append([]string{"start"}, rest...))
		return "ok"
	default:
		return "err record: expected start|stop|status|info|pause|resume|settings"
	}
}

// startRecord wires the daemon call the Screen capture page in Settings uses to read and
// write the capture settings. The daemon is the sole writer of recording.json;
// the backend script and the shell surfaces only read it.
func (d *daemon) startRecord() {
	d.registerCall("record.settings", func(raw json.RawMessage) (any, error) {
		return recordSettingsCall(raw)
	})
}

// recorderRunning reports whether the Ryoku-started GSR capture is live, matching
// the shell surfaces' own detection: gsr-cli answers on the IPC socket only for
// the instance the recorder script launched.
func recorderRunning() bool {
	return exec.Command("gsr-cli", "-ipc", recordSocketPath(), "status").Run() == nil
}

// recordSocketPath is the single-recording GSR control socket, shared with the
// backend script.
func recordSocketPath() string {
	dir := os.Getenv("XDG_RUNTIME_DIR")
	if dir == "" {
		dir = "/tmp"
	}
	return filepath.Join(dir, "ryoku-gsr.sock")
}

// runRecord invokes ryoku-cmd-record and returns its trimmed stdout, or "ok" for
// a silent success. Used for the fast, non-blocking verbs (stop/status/info and
// the pause/resume passthroughs that map to gsr-cli set-paused).
func runRecord(args []string) string {
	out, err := exec.Command("ryoku-cmd-record", args...).Output()
	text := strings.TrimSpace(string(out))
	if err != nil && text == "" {
		return "err record: " + err.Error()
	}
	if text == "" {
		return "ok"
	}
	return text
}

// spawnRecord starts ryoku-cmd-record detached in its own session. The recorder
// backgrounds gpu-screen-recorder and waits on it, so it must not be a supervised
// child: setsid frees it from the daemon's process group so a reload cannot take
// the capture down with it.
func spawnRecord(args []string) {
	cmd := exec.Command("ryoku-cmd-record", args...)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	if err := cmd.Start(); err == nil {
		go func() { _ = cmd.Wait() }()
	}
}

// recordSettingKind is how a recording.json value is typed and validated.
type recordSettingKind int

const (
	recordInt recordSettingKind = iota
	recordBool
	recordEnum
	recordPath
)

// recordSettingSpec is one row of the recording.json contract: its default, JSON
// type, and the bounds or allowed values that make a write legal.
type recordSettingSpec struct {
	key     string
	kind    recordSettingKind
	def     any
	min     int
	max     int
	allowed []string
}

// recordSettingSpecs is the authoritative key set. The Screen capture page and
// the backend capture script read these keys; only the daemon writes them, so
// the validation here is the single gate every write passes through.
var recordSettingSpecs = []recordSettingSpec{
	{key: "fps", kind: recordInt, def: 60, min: 1, max: 360},
	{key: "framerateMode", kind: recordEnum, def: "cfr", allowed: []string{"cfr", "vfr"}},
	{key: "quality", kind: recordEnum, def: "very_high", allowed: []string{"medium", "high", "very_high", "ultra"}},
	{key: "bitrateMode", kind: recordEnum, def: "quality", allowed: []string{"quality", "cbr"}},
	{key: "bitrate", kind: recordInt, def: 20000, min: 500, max: 200000},
	{key: "codec", kind: recordEnum, def: "h264", allowed: []string{"h264", "hevc", "av1"}},
	{key: "encoder", kind: recordEnum, def: "gpu", allowed: []string{"gpu", "cpu"}},
	{key: "cursor", kind: recordBool, def: true},
	{key: "container", kind: recordEnum, def: "mp4", allowed: []string{"mp4", "mkv", "webm"}},
	{key: "audioCodec", kind: recordEnum, def: "opus", allowed: []string{"opus", "aac"}},
	{key: "audioBitrate", kind: recordInt, def: 0, min: 0, max: 1024},
	{key: "colorRange", kind: recordEnum, def: "limited", allowed: []string{"limited", "full"}},
	{key: "keyint", kind: recordInt, def: 2, min: 1, max: 600},
	{key: "maxResolution", kind: recordEnum, def: "native", allowed: []string{"native", "1080p", "1440p", "2160p"}},
	{key: "directory", kind: recordPath, def: ""},
}

// recordSettingsMu serializes writes so the two entry points (the CLI verb and
// the Hub's socket call) can never interleave a read-modify-write.
var recordSettingsMu sync.Mutex

func recordSpecFor(key string) (recordSettingSpec, bool) {
	for _, s := range recordSettingSpecs {
		if s.key == key {
			return s, true
		}
	}
	return recordSettingSpec{}, false
}

func recordingConfigPath() string {
	dir := os.Getenv("XDG_CONFIG_HOME")
	if dir == "" {
		if home, err := os.UserHomeDir(); err == nil {
			dir = filepath.Join(home, ".config")
		}
	}
	return filepath.Join(dir, "ryoku", "recording.json")
}

// mergedRecordingSettings returns the full key set: the defaults overlaid with
// any valid values from recording.json. Malformed or unknown entries in the file
// are ignored so a hand-edited file can never surface an out-of-contract value.
func mergedRecordingSettings() map[string]any {
	merged := make(map[string]any, len(recordSettingSpecs))
	for _, s := range recordSettingSpecs {
		merged[s.key] = s.def
	}
	var file map[string]json.RawMessage
	if b, err := os.ReadFile(recordingConfigPath()); err == nil {
		_ = json.Unmarshal(b, &file)
	}
	for _, s := range recordSettingSpecs {
		raw, ok := file[s.key]
		if !ok {
			continue
		}
		if v, err := validateRecordingValue(s, raw); err == nil {
			merged[s.key] = v
		}
	}
	return merged
}

// validateRecordingValue checks one raw JSON value against a spec, returning the
// typed value to store. The JSON type must match the contract: ints stay ints (a
// fractional or non-numeric value is rejected), bools stay bools, enums must be
// one of the allowed strings.
func validateRecordingValue(s recordSettingSpec, raw json.RawMessage) (any, error) {
	switch s.kind {
	case recordInt:
		var n json.Number
		if err := json.Unmarshal(raw, &n); err != nil {
			return nil, fmt.Errorf("%s must be an integer", s.key)
		}
		i, err := strconv.Atoi(n.String())
		if err != nil {
			return nil, fmt.Errorf("%s must be an integer", s.key)
		}
		if i < s.min || i > s.max {
			return nil, fmt.Errorf("%s must be between %d and %d", s.key, s.min, s.max)
		}
		return i, nil
	case recordBool:
		var b bool
		if err := json.Unmarshal(raw, &b); err != nil {
			return nil, fmt.Errorf("%s must be true or false", s.key)
		}
		return b, nil
	case recordEnum:
		var str string
		if err := json.Unmarshal(raw, &str); err != nil {
			return nil, fmt.Errorf("%s must be a string", s.key)
		}
		for _, a := range s.allowed {
			if str == a {
				return str, nil
			}
		}
		return nil, fmt.Errorf("%s must be one of %s", s.key, strings.Join(s.allowed, ", "))
	default: // recordPath
		var str string
		if err := json.Unmarshal(raw, &str); err != nil {
			return nil, fmt.Errorf("%s must be a string", s.key)
		}
		return str, nil
	}
}

// writeRecordingSettings validates every provided key and, only if all pass,
// writes the whole merged key set back to recording.json atomically. An unknown
// key or a value that fails validation rejects the entire write so the file never
// lands half-applied.
func writeRecordingSettings(updates map[string]json.RawMessage) error {
	validated := make(map[string]any, len(updates))
	for key, raw := range updates {
		spec, ok := recordSpecFor(key)
		if !ok {
			return fmt.Errorf("unknown setting: %s", key)
		}
		v, err := validateRecordingValue(spec, raw)
		if err != nil {
			return err
		}
		validated[key] = v
	}

	recordSettingsMu.Lock()
	defer recordSettingsMu.Unlock()

	merged := mergedRecordingSettings()
	for key, v := range validated {
		merged[key] = v
	}

	body, err := json.MarshalIndent(merged, "", "  ")
	if err != nil {
		return err
	}
	body = append(body, '\n')

	path := recordingConfigPath()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, body, 0o644); err != nil {
		return err
	}
	if err := os.Rename(tmp, path); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	return nil
}

// recordSettingsCall answers the Hub's record.settings socket call. With no
// settings map it reads and returns the merged key set; with a settings map it
// validates and writes the whole map, replying ok or an error.
func recordSettingsCall(raw json.RawMessage) (any, error) {
	var payload struct {
		Settings map[string]json.RawMessage `json:"settings"`
	}
	if len(raw) > 0 {
		if err := json.Unmarshal(raw, &payload); err != nil {
			return nil, err
		}
	}
	if payload.Settings == nil {
		return mergedRecordingSettings(), nil
	}
	if err := writeRecordingSettings(payload.Settings); err != nil {
		return nil, err
	}
	return nil, nil
}

// recordSettingsCLI backs `ryoku-shell record settings`: no arguments prints the
// merged settings as JSON; `settings KEY VALUE` validates and writes that one
// key.
func recordSettingsCLI(args []string) string {
	if len(args) == 0 {
		b, err := json.Marshal(mergedRecordingSettings())
		if err != nil {
			return "err record: " + err.Error()
		}
		return string(b)
	}
	if len(args) != 2 {
		return "err record: settings expects KEY VALUE"
	}
	key, token := args[0], args[1]
	spec, ok := recordSpecFor(key)
	if !ok {
		return "err record: unknown setting: " + key
	}
	raw, err := recordCLIValue(spec, token)
	if err != nil {
		return "err record: " + err.Error()
	}
	if err := writeRecordingSettings(map[string]json.RawMessage{key: raw}); err != nil {
		return "err record: " + err.Error()
	}
	return "ok"
}

// recordCLIValue turns a command-line token into the JSON shape the setting's
// type demands, so the shared validator sees the same value the socket path
// does.
func recordCLIValue(s recordSettingSpec, token string) (json.RawMessage, error) {
	switch s.kind {
	case recordInt:
		n, err := strconv.Atoi(token)
		if err != nil {
			return nil, fmt.Errorf("%s must be an integer", s.key)
		}
		return json.RawMessage(strconv.Itoa(n)), nil
	case recordBool:
		b, err := strconv.ParseBool(token)
		if err != nil {
			return nil, fmt.Errorf("%s must be true or false", s.key)
		}
		return json.RawMessage(strconv.FormatBool(b)), nil
	default:
		b, err := json.Marshal(token)
		if err != nil {
			return nil, err
		}
		return json.RawMessage(b), nil
	}
}
