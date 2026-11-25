// Package config reads and writes rashin.json: defaults, validation, merge
// patches and the one-time translation of Rashin 1's file.
package config

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	i18n "ryoku-i18n"
	"ryoku-rashin/internal/oni/voice/wake"
	"ryoku-rashin/internal/paths"
	"ryoku-rashin/internal/protocol"
	"slices"
	"strings"
	"syscall"
)

// Version is the rashin.json schema this package writes.
const Version = 2

// DefaultAccountServer is Ryoku's own Ryoku Accounts server, where a fresh
// install signs in. Nothing contacts it until the user signs in.
const DefaultAccountServer = "https://ai.ryoku.dev"

// Defaults are the settings of a machine that never changed one. Rashin is off
// until the user turns it on.
func Defaults() protocol.Settings {
	return protocol.Settings{
		Enabled: false,
		Ask:     protocol.AskSettings{Model: protocol.RouteAuto, Habits: true, Approvals: protocol.ApprovalsReadOnly},
		Agent: protocol.AgentSettings{
			Model:           protocol.RouteAuto,
			Approvals:       protocol.ApprovalsReadOnly,
			IdleStopMinutes: 10,
		},
		Daemon: protocol.DaemonSettings{IdleExitMinutes: 5},
		Index:  protocol.IndexSettings{System: true, Semantic: true},
		API:    protocol.APISettings{Port: protocol.DefaultAPIPort},
		Teams:  defaultTeams(),
		Providers: protocol.ProvidersSettings{
			Local:   protocol.DefaultLocalAddresses(),
			Models:  protocol.LocalModelsSettings{IdleMinutes: protocol.DefaultLocalIdleMinutes},
			Account: DefaultAccountServer,
		},
		Memory: defaultMemory(),
		Oni:    protocol.DefaultOniSettings(),
	}
}

// legacyKeys are Rashin 1 keys dropped on the first version 2 write.
var legacyKeys = []string{
	"port", "quick", "chatAgent", "fix", "prowlHarnesses", "keepProwl",
	"approvals", "intro", "habits", "optedOut",
}

// known are the version 2 keys this package owns.
var known = []string{"version", "enabled", "ask", "agent", "daemon", "index", "api", "teams", "providers", "memory", "skills", "features", "oni", "onboarding"}

// Validate normalises s in place: unknown enums fall back to the defaults and
// numbers are clamped to sane ranges.
func Validate(s *protocol.Settings) {
	d := Defaults()
	s.Ask.ResumeMinutes = max(0, min(s.Ask.ResumeMinutes, 120))
	for id := range s.Features {
		known := false
		for _, feature := range protocol.FeatureIDs {
			if id == feature {
				known = true
				break
			}
		}
		if !known {
			delete(s.Features, id)
		}
	}
	s.Ask.Model = strings.TrimSpace(s.Ask.Model)
	if !protocol.ValidModel(s.Ask.Model) {
		s.Ask.Model = d.Ask.Model
	}
	if !protocol.KnownApprovals(s.Ask.Approvals) {
		s.Ask.Approvals = d.Ask.Approvals
	}
	if !protocol.KnownApprovals(s.Agent.Approvals) {
		s.Agent.Approvals = d.Agent.Approvals
	}
	s.Agent.Model = strings.TrimSpace(s.Agent.Model)
	if !protocol.ValidModel(s.Agent.Model) {
		s.Agent.Model = d.Agent.Model
	}
	if !protocol.ValidEngineOptions(s.Agent.Options) {
		s.Agent.Options = nil
	}
	s.Agent.IdleStopMinutes = clamp(s.Agent.IdleStopMinutes, 1, 24*60, d.Agent.IdleStopMinutes)
	s.Daemon.IdleExitMinutes = clamp(s.Daemon.IdleExitMinutes, 1, 24*60, d.Daemon.IdleExitMinutes)
	if !validPort(s.API.Port) {
		s.API.Port = d.API.Port
	}
	validateTeams(&s.Teams, d.Teams)
	validateLocal(&s.Providers, d.Providers)
	validateMemory(&s.Memory, d.Memory)
	s.Skills.Off = cleanIDs(s.Skills.Off)
	validateOni(&s.Oni, d.Oni)
}

// a stored Oni block keeps what is valid; hands keep the browser stage, and
// any other stage, the ones not in this build among them, falls back to off
func validateOni(o *protocol.OniSettings, d protocol.OniSettings) {
	o.Character = strings.TrimSpace(o.Character)
	if !protocol.ValidOniCharacterID(o.Character) {
		o.Character = d.Character
	}
	if o.Disclosure != protocol.OniDisclosureSession && o.Disclosure != protocol.OniDisclosureAlways {
		o.Disclosure = d.Disclosure
	}
	if chord, ok := protocol.NormalizeOniChord(o.Chord); ok {
		o.Chord = chord
	} else {
		o.Chord = d.Chord
	}
	if !slices.Contains(protocol.OniHandsStages, o.Hands) {
		o.Hands = protocol.OniHandsOff
	}
	o.Feeds.Words, _ = protocol.CleanOniFeedWords(o.Feeds.Words)
	if !slices.Contains(protocol.OniSizes, o.Size) {
		o.Size = d.Size
	}
	if !slices.Contains(protocol.OniReduceModes, o.ReduceMotion) {
		o.ReduceMotion = d.ReduceMotion
	}
	validateVoice(&o.VoiceSettings, d.VoiceSettings)
	if !slices.Contains(protocol.OniRewriteStyles, o.Rewrite.Style) {
		o.Rewrite.Style = d.Rewrite.Style
	}
	if !slices.Contains(protocol.OniAssignmentStyles, o.Assignment.Style) {
		o.Assignment.Style = d.Assignment.Style
	}
	if !protocol.ValidOniAssignmentModel(o.Assignment.Model) {
		o.Assignment.Model = d.Assignment.Model
	}
	o.MCP.Tools = protocol.CleanOniMCPTools(o.MCP.Tools)
	var clients []string
	for _, name := range o.MCP.Clients {
		if protocol.ValidOniMCPClientName(name) && !slices.Contains(clients, name) && len(clients) < protocol.OniMCPClientsMax {
			clients = append(clients, name)
		}
	}
	o.MCP.Clients = clients
}

// a stored voice block keeps what is valid; anything else is its default
func validateVoice(v *protocol.OniVoiceSettings, d protocol.OniVoiceSettings) {
	if phrase, ok := wakePhrase(v.Wake.Phrase); ok {
		v.Wake.Phrase = phrase
	} else {
		v.Wake.Phrase = d.Wake.Phrase
	}
	if v.Mic != "" && !protocol.ValidOniMicNode(v.Mic) {
		v.Mic = d.Mic
	}
	if v.STT.Engine != "" && !slices.Contains(protocol.OniSTTEngines, v.STT.Engine) {
		v.STT.Engine = d.STT.Engine
	}
	if v.STT.Model != "" && !protocol.ValidOniSTTModel(v.STT.Model) {
		v.STT.Model = d.STT.Model
	}
	if v.STT.Language != "" && !protocol.ValidOniLanguage(v.STT.Language) {
		v.STT.Language = d.STT.Language
	}
	if v.TTS.Engine != "" && !slices.Contains(protocol.OniTTSEngines, v.TTS.Engine) {
		v.TTS.Engine = d.TTS.Engine
	}
	if !slices.Contains(protocol.OniDictateModes, v.Dictation.Mode) {
		v.Dictation.Mode = d.Dictation.Mode
	}
	if !slices.Contains(protocol.OniPolishRoutes, v.Dictation.Polish) {
		v.Dictation.Polish = d.Dictation.Polish
	}
}

// wakePhrase is phrase as the wake listener compares it, the default for an
// empty one; ok is false for a phrase it could never match.
func wakePhrase(phrase string) (string, bool) {
	if phrase != "" && !protocol.ValidOniText(phrase, protocol.OniWakePhraseMax, false) {
		return "", false
	}
	p, err := wake.ParsePhrase(phrase)
	if err != nil {
		return "", false
	}
	return p.String(), true
}

// a patch naming an Oni value that cannot be used is refused rather than
// quietly replaced, so the client learns which hands are in this build
func checkOni(o protocol.OniSettings) error {
	if err := CheckOniHands(o.Hands); err != nil {
		return err
	}
	if !protocol.ValidOniCharacterID(strings.TrimSpace(o.Character)) {
		return protocol.Errorf(protocol.CodeBadRequest, "%s", i18n.T("The Oni character id is invalid."))
	}
	if o.Disclosure != protocol.OniDisclosureSession && o.Disclosure != protocol.OniDisclosureAlways {
		return protocol.Errorf(protocol.CodeBadRequest, "%s", i18n.T("Oni asks before a picture leaves this computer either once a session or every time."))
	}
	if _, ok := protocol.NormalizeOniChord(o.Chord); !ok {
		return protocol.Errorf(protocol.CodeBadRequest, "%s", i18n.T("The Oni talk chord needs Ctrl, Alt or Super, with at most one other key."))
	}
	if _, ok := protocol.CleanOniFeedWords(o.Feeds.Words); !ok {
		return protocol.Errorf(protocol.CodeBadRequest, "%s", i18n.Tf("Oni takes at most %d feed words of up to %d characters each.", protocol.OniFeedWordsMax, protocol.OniFeedWordMaxRunes))
	}
	if !slices.Contains(protocol.OniSizes, o.Size) {
		return protocol.Errorf(protocol.CodeBadRequest, "%s", i18n.T("Oni's size is small, normal or big."))
	}
	if !slices.Contains(protocol.OniReduceModes, o.ReduceMotion) {
		return protocol.Errorf(protocol.CodeBadRequest, "%s", i18n.T("Oni's reduced motion follows the desktop, or is on or off."))
	}
	if !slices.Contains(protocol.OniRewriteStyles, o.Rewrite.Style) {
		return protocol.Errorf(protocol.CodeBadRequest, "%s", i18n.T("The rewrite style is one of the styles Settings lists."))
	}
	if !slices.Contains(protocol.OniAssignmentStyles, o.Assignment.Style) {
		return protocol.Errorf(protocol.CodeBadRequest, "%s", i18n.T("Assignment mode shows the steps or the answer alone."))
	}
	if !protocol.ValidOniAssignmentModel(o.Assignment.Model) {
		return protocol.Errorf(protocol.CodeBadRequest, "%s", i18n.T("The assignment model is empty or a model the gateway can name."))
	}
	for id := range o.MCP.Tools {
		if !slices.Contains(protocol.OniMCPTools, id) {
			return protocol.Errorf(protocol.CodeBadRequest, "%s", i18n.Tf("The MCP bridge has no tool %q.", id))
		}
	}
	if _, ok := protocol.CleanOniMCPClients(o.MCP.Clients); !ok {
		return protocol.Errorf(protocol.CodeBadRequest, "%s", i18n.Tf("The MCP bridge approves at most %d programs, each named in one line of up to %d characters.", protocol.OniMCPClientsMax, protocol.OniMCPClientNameMax))
	}
	return checkVoice(o.VoiceSettings)
}

// CheckOniHands refuses a hands stage settings.set cannot store: anything
// but off, browser, app or desktop is no stage. Who may store one is the
// gate's: only the app.
func CheckOniHands(stage string) error {
	if slices.Contains(protocol.OniHandsStages, stage) {
		return nil
	}
	return protocol.Errorf(protocol.CodeBadRequest, "%s", i18n.T("Oni's Hands are off, or work in one browser tab, in one window you pick, or on Hyakki's own separate desktop."))
}

func checkVoice(v protocol.OniVoiceSettings) error {
	if _, ok := wakePhrase(v.Wake.Phrase); !ok {
		return protocol.Errorf(protocol.CodeBadRequest, "%s", i18n.Tf("The wake phrase is %d to %d words of at least %d letters each, at most %d characters, and does not start with a filler sound.", wake.MinPhraseWords, wake.MaxPhraseWords, wake.MinWordRunes, wake.MaxPhraseRunes))
	}
	if v.Mic != "" && !protocol.ValidOniMicNode(v.Mic) {
		return protocol.Errorf(protocol.CodeBadRequest, "%s", i18n.T("The microphone is not a PipeWire source name."))
	}
	if v.STT.Engine != "" && !slices.Contains(protocol.OniSTTEngines, v.STT.Engine) {
		return protocol.Errorf(protocol.CodeBadRequest, "%s", i18n.T("The speech engine is whisper.cpp or transcribe.cpp."))
	}
	if v.STT.Model != "" && !protocol.ValidOniSTTModel(v.STT.Model) {
		return protocol.Errorf(protocol.CodeBadRequest, "%s", i18n.T("The speech model id is invalid."))
	}
	if v.STT.Language != "" && !protocol.ValidOniLanguage(v.STT.Language) {
		return protocol.Errorf(protocol.CodeBadRequest, "%s", i18n.T("The spoken language is a two or three letter code such as en."))
	}
	if v.TTS.Engine != "" && !slices.Contains(protocol.OniTTSEngines, v.TTS.Engine) {
		return protocol.Errorf(protocol.CodeBadRequest, "%s", i18n.T("Oni's voice is the system voice, recorded clips or Kokoro."))
	}
	if !slices.Contains(protocol.OniDictateModes, v.Dictation.Mode) {
		return protocol.Errorf(protocol.CodeBadRequest, "%s", i18n.T("Dictation types exact, clean or polished text."))
	}
	if !slices.Contains(protocol.OniPolishRoutes, v.Dictation.Polish) {
		return protocol.Errorf(protocol.CodeBadRequest, "%s", i18n.T("A polished dictation is rewritten on this computer or on the Ask route."))
	}
	return nil
}

// below 1024 a user's daemon cannot listen
func validPort(port int) bool { return port >= 1024 && port <= 65535 }

// a patch that names the Ask or Agents model must name one the gateway can
// route; a stored one that does not falls back on load
func checkModels(s protocol.Settings) error {
	if !protocol.ValidModel(strings.TrimSpace(s.Ask.Model)) {
		return protocol.Errorf(protocol.CodeBadRequest, "%s", i18n.T("The Ask model is invalid."))
	}
	if !protocol.ValidModel(strings.TrimSpace(s.Agent.Model)) {
		return protocol.Errorf(protocol.CodeBadRequest, "%s", i18n.T("The Agents model is invalid."))
	}
	if !protocol.ValidEngineOptions(s.Agent.Options) {
		return protocol.Errorf(protocol.CodeBadRequest, "%s", i18n.Tf("The Agents model takes at most %d options, each with an id and a value.", protocol.EngineOptionsMax))
	}
	return nil
}

func clamp(v, lo, hi, def int) int {
	if v == 0 {
		return def
	}
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

// Load reads hyakki.json. A missing file yields the defaults (Hyakki off). A
// Rashin 1 file is translated in memory; Save writes the translation.
func Load() (protocol.Settings, error) {
	raw, err := readRaw()
	if err != nil {
		return Defaults(), err
	}
	return fromRaw(raw), nil
}

func readRaw() (map[string]json.RawMessage, error) {
	b, err := os.ReadFile(paths.ConfigFile())
	if errors.Is(err, fs.ErrNotExist) {
		return map[string]json.RawMessage{}, nil
	}
	if err != nil {
		return map[string]json.RawMessage{}, err
	}
	raw := map[string]json.RawMessage{}
	if len(bytes.TrimSpace(b)) == 0 {
		return raw, nil
	}
	if err := json.Unmarshal(b, &raw); err != nil {
		return map[string]json.RawMessage{}, fmt.Errorf("read %s: %w", paths.ConfigFile(), err)
	}
	if raw == nil {
		return nil, fmt.Errorf("read %s: settings must be an object", paths.ConfigFile())
	}
	return raw, nil
}

func fromRaw(raw map[string]json.RawMessage) protocol.Settings {
	var version int
	_ = json.Unmarshal(raw["version"], &version)
	if version < Version && len(raw) > 0 {
		return MigrateV1(raw)
	}
	s := Defaults()
	s.Enabled = enabledFromRaw(raw, false)
	decodeInto(raw["ask"], &s.Ask)
	decodeInto(raw["agent"], &s.Agent)
	decodeInto(raw["daemon"], &s.Daemon)
	decodeInto(raw["index"], &s.Index)
	decodeInto(raw["api"], &s.API)
	decodeInto(raw["teams"], &s.Teams)
	decodeInto(raw["providers"], &s.Providers)
	decodeInto(raw["memory"], &s.Memory)
	decodeInto(raw["skills"], &s.Skills)
	decodeInto(raw["features"], &s.Features)
	decodeInto(raw["oni"], &s.Oni)
	decodeInto(raw["onboarding"], &s.Onboarding)
	Validate(&s)
	return s
}

func decodeInto(b json.RawMessage, v any) {
	if len(b) > 0 {
		_ = json.Unmarshal(b, v)
	}
}

func enabledFromRaw(raw map[string]json.RawMessage, legacy bool) bool {
	var enabled bool
	_ = json.Unmarshal(raw["enabled"], &enabled)
	if !enabled || !legacy {
		return enabled
	}
	var optedOut bool
	_ = json.Unmarshal(raw["optedOut"], &optedOut)
	return !optedOut
}

// MigrateV1 translates a Rashin 1 file. Rashin stays on only when it was on and
// the user had not opted out; approvals carry over. Rashin 1's chatAgent named
// an external harness, which Rashin no longer runs, so it is dropped.
func MigrateV1(raw map[string]json.RawMessage) protocol.Settings {
	s := Defaults()
	s.Enabled = enabledFromRaw(raw, true)
	var approvals string
	_ = json.Unmarshal(raw["approvals"], &approvals)
	s.Agent.Approvals = approvals
	var habits struct {
		History *bool `json:"history"`
	}
	_ = json.Unmarshal(raw["habits"], &habits)
	if habits.History != nil {
		s.Ask.Habits = *habits.History
	}
	Validate(&s)
	return s
}

// Save writes s atomically, keeping keys this package does not know and
// dropping Rashin 1 keys.
func Save(s protocol.Settings) error {
	return withConfigLock(func() error { return saveUnlocked(s) })
}

// SaveMerged writes settings.set while preserving the on-disk enabled switch.
func SaveMerged(s protocol.Settings) (protocol.Settings, error) {
	err := withConfigLock(func() error {
		raw, err := readRaw()
		if err != nil {
			return err
		}
		current := fromRaw(raw)
		s.Enabled = current.Enabled
		return saveUnlocked(s)
	})
	return s, err
}

func saveUnlocked(s protocol.Settings) error {
	Validate(&s)
	raw, err := readRaw()
	if err != nil {
		raw = map[string]json.RawMessage{}
	}
	for _, k := range legacyKeys {
		delete(raw, k)
	}
	set := func(k string, v any) error {
		b, err := json.Marshal(v)
		if err != nil {
			return err
		}
		raw[k] = b
		return nil
	}
	for k, v := range map[string]any{
		"version":    Version,
		"enabled":    s.Enabled,
		"ask":        s.Ask,
		"agent":      s.Agent,
		"daemon":     s.Daemon,
		"index":      s.Index,
		"api":        s.API,
		"teams":      s.Teams,
		"providers":  s.Providers,
		"memory":     s.Memory,
		"skills":     s.Skills,
		"features":   s.Features,
		"oni":        s.Oni,
		"onboarding": s.Onboarding,
	} {
		if err := set(k, v); err != nil {
			return err
		}
	}
	b, err := marshalOrdered(raw)
	if err != nil {
		return err
	}
	return writeAtomic(paths.ConfigFile(), b)
}

func withConfigLock(fn func() error) error {
	if err := paths.EnsureDir(paths.RuntimeDir()); err != nil {
		return err
	}
	lock, err := os.OpenFile(paths.ConfigLock(), os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return err
	}
	defer lock.Close()
	if err := syscall.Flock(int(lock.Fd()), syscall.LOCK_EX); err != nil {
		return err
	}
	defer syscall.Flock(int(lock.Fd()), syscall.LOCK_UN)
	return fn()
}

// marshalOrdered writes the owned keys first, in schema order, then the rest
// sorted, so the file reads the same way every time.
func marshalOrdered(raw map[string]json.RawMessage) ([]byte, error) {
	var buf bytes.Buffer
	buf.WriteString("{\n")
	keys := append([]string{}, known...)
	var rest []string
	for k := range raw {
		if !contains(known, k) {
			rest = append(rest, k)
		}
	}
	sortStrings(rest)
	keys = append(keys, rest...)
	first := true
	for _, k := range keys {
		v, ok := raw[k]
		if !ok {
			continue
		}
		if !first {
			buf.WriteString(",\n")
		}
		first = false
		kb, _ := json.Marshal(k)
		var vb bytes.Buffer
		if err := json.Compact(&vb, v); err != nil {
			return nil, err
		}
		buf.WriteString("  ")
		buf.Write(kb)
		buf.WriteString(": ")
		buf.Write(vb.Bytes())
	}
	buf.WriteString("\n}\n")
	return buf.Bytes(), nil
}

func contains(list []string, s string) bool {
	for _, v := range list {
		if v == s {
			return true
		}
	}
	return false
}

func sortStrings(s []string) {
	for i := 1; i < len(s); i++ {
		for j := i; j > 0 && s[j] < s[j-1]; j-- {
			s[j], s[j-1] = s[j-1], s[j]
		}
	}
}

func writeAtomic(path string, b []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".rashin.json.*")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	if _, err := tmp.Write(b); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Chmod(0o644); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), path)
}

// SetEnabled flips the on/off switch and saves.
func SetEnabled(on bool) error {
	return withConfigLock(func() error {
		raw, err := readRaw()
		if err != nil {
			return err
		}
		s := fromRaw(raw)
		s.Enabled = on
		return saveUnlocked(s)
	})
}

// EnableIfUnset applies deferred consent without replacing a saved choice.
// Activation shares the config lock so a later off cannot be followed by
// stale service arming. activate must not write configuration.
func EnableIfUnset(activate func() error) (bool, error) {
	enabled := false
	err := withConfigLock(func() error {
		raw, err := readRaw()
		if err != nil {
			return err
		}
		if len(raw) != 0 {
			var version int
			_ = json.Unmarshal(raw["version"], &version)
			enabled = enabledFromRaw(raw, version < Version)
		} else {
			s := Defaults()
			s.Enabled = true
			if err := saveUnlocked(s); err != nil {
				return err
			}
			enabled = true
		}
		if !enabled {
			return nil
		}
		return activate()
	})
	return enabled, err
}

// Merge applies a JSON merge patch (RFC 7396) to cur and validates the result.
// Enabled cannot be changed this way.
func Merge(cur protocol.Settings, patch json.RawMessage) (protocol.Settings, error) {
	base, err := json.Marshal(cur)
	if err != nil {
		return cur, err
	}
	var doc, p any
	if err := json.Unmarshal(base, &doc); err != nil {
		return cur, err
	}
	if err := json.Unmarshal(patch, &p); err != nil {
		return cur, protocol.Errorf(protocol.CodeBadRequest, "%s", i18n.Tf("The settings patch is not JSON: %v.", err))
	}
	merged, err := json.Marshal(mergePatch(doc, p))
	if err != nil {
		return cur, err
	}
	next := Defaults()
	if err := json.Unmarshal(merged, &next); err != nil {
		return cur, protocol.Errorf(protocol.CodeBadRequest, "%s", i18n.Tf("The settings patch has the wrong shape: %v.", err))
	}
	next.Enabled = cur.Enabled
	if err := validatePatch(next, cur); err != nil {
		return cur, err
	}
	Validate(&next)
	return next, nil
}

func validatePatch(s, cur protocol.Settings) error {
	if err := checkModels(s); err != nil {
		return err
	}
	if !validPort(s.API.Port) {
		return protocol.Errorf(protocol.CodeBadRequest, "%s", i18n.T("The API port must be between 1024 and 65535."))
	}
	if err := checkLocal(s.Providers, cur.Providers); err != nil {
		return err
	}
	if err := checkMemory(s.Memory); err != nil {
		return err
	}
	if err := checkOni(s.Oni); err != nil {
		return err
	}
	return checkTeams(s.Teams)
}

func mergePatch(target, patch any) any {
	pm, ok := patch.(map[string]any)
	if !ok {
		return patch
	}
	tm, ok := target.(map[string]any)
	if !ok {
		tm = map[string]any{}
	}
	for k, v := range pm {
		if v == nil {
			delete(tm, k)
			continue
		}
		tm[k] = mergePatch(tm[k], v)
	}
	return tm
}
