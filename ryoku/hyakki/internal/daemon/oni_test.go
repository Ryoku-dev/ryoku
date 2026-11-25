package daemon

import (
	"bytes"
	"encoding/json"
	"net"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"ryoku-rashin/internal/config"
	"ryoku-rashin/internal/oni/feature"
	"ryoku-rashin/internal/oni/profile"
	"ryoku-rashin/internal/oni/spec"
	"ryoku-rashin/internal/paths"
	"ryoku-rashin/internal/protocol"
)

// oniDaemon is a daemon in temporary XDG dirs with Hyakki and Oni on and a
// mix of Oni features on, so the capabilities hold both answers.
func oniDaemon(t *testing.T) *Daemon {
	t.Helper()
	isolateDaemon(t)
	settings, err := config.Load()
	if err != nil {
		t.Fatal(err)
	}
	settings.Enabled = true
	settings.Oni.Master = true
	settings.Oni.Inspect = true
	settings.Oni.Voice = true
	settings.Oni.Feeds.Guard = true
	if err := config.Save(settings); err != nil {
		t.Fatal(err)
	}
	if settings, err = config.Load(); err != nil {
		t.Fatal(err)
	}
	d := New(refuseRealDesktop(t, Deps{Version: "oni-wire-test"}))
	d.mu.Lock()
	d.settings = settings
	d.mu.Unlock()
	return d
}

// installOniCharacter copies the profile fixture source into the user's
// characters as id, editing its profile.json on the way.
func installOniCharacter(t *testing.T, source, id string, edit func(map[string]any)) {
	t.Helper()
	dir := filepath.Join(paths.CharactersDir(), id)
	if err := os.MkdirAll(paths.CharactersDir(), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.CopyFS(dir, os.DirFS(filepath.Join("..", "oni", "profile", "testdata", source))); err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(dir, "profile.json")
	raw, err := os.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}
	var p map[string]any
	if err := json.Unmarshal(raw, &p); err != nil {
		t.Fatal(err)
	}
	p["id"] = id
	if edit != nil {
		edit(p)
	}
	if raw, err = json.Marshal(p); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(file, raw, 0o600); err != nil {
		t.Fatal(err)
	}
}

// dialOni opens a connection to d over a pipe and says hello as kind; app
// stands for what the daemon's peer check found.
func dialOni(t *testing.T, d *Daemon, kind string, app bool) func(method string, params any) protocol.Frame {
	t.Helper()
	server, client := net.Pipe()
	c := newConnection(d, server)
	c.app = app
	stopped := make(chan struct{})
	go func() { c.serve(); close(stopped) }()
	t.Cleanup(func() {
		client.Close()
		c.close()
		select {
		case <-stopped:
		case <-time.After(time.Second):
			t.Error("connection did not close")
		}
	})
	if err := client.SetDeadline(time.Now().Add(5 * time.Second)); err != nil {
		t.Fatal(err)
	}
	encoder, decoder := protocol.NewEncoder(client), protocol.NewDecoder(client)
	var sequence int64
	call := func(method string, params any) protocol.Frame {
		t.Helper()
		sequence++
		id := sequence
		if err := encoder.Encode(protocol.Frame{ID: &id, Method: method, Params: mustMarshal(t, params)}); err != nil {
			t.Fatal(err)
		}
		for {
			response, err := decoder.Next()
			if err != nil {
				t.Fatal(err)
			}
			if response.ID == nil {
				continue
			}
			if *response.ID != id {
				t.Fatalf("wrong response identity for %s", method)
			}
			return response
		}
	}
	if hello := call(protocol.MHello, protocol.HelloParams{Client: kind, Version: protocol.Version}); hello.Error != nil {
		t.Fatalf("hello failed: %v", hello.Error)
	}
	return call
}

func oniResult[T any](t *testing.T, f protocol.Frame) T {
	t.Helper()
	var v T
	if f.Error != nil {
		t.Fatalf("call failed: %v", f.Error)
	}
	if err := json.Unmarshal(f.Result, &v); err != nil {
		t.Fatal(err)
	}
	return v
}

func capabilitiesBytes(t *testing.T, f protocol.Frame) json.RawMessage {
	t.Helper()
	return oniResult[struct {
		Capabilities json.RawMessage `json:"capabilities"`
	}](t, f).Capabilities
}

func decideEveryFeature(s protocol.Settings) []feature.Decision {
	effective := protocol.EffectiveOni(s)
	var out []feature.Decision
	for _, f := range feature.Features() {
		out = append(out, feature.Decide(feature.Request{Feature: f.ID}, effective))
	}
	return out
}

func oniCharacterByID(characters []protocol.OniCharacter, id string) (protocol.OniCharacter, bool) {
	for _, c := range characters {
		if c.ID == id {
			return c, true
		}
	}
	return protocol.OniCharacter{}, false
}

func errorCode(f protocol.Frame) string {
	if f.Error == nil {
		return ""
	}
	return f.Error.Code
}

func TestOniSecondCharacterChangesPresentationOnly(t *testing.T) {
	d := oniDaemon(t)
	installOniCharacter(t, "testmoth", "testmoth", nil)
	call := dialOni(t, d, protocol.ClientApp, true)

	beforeFrame := call(protocol.MOniStatus, protocol.Empty{})
	before := oniResult[protocol.OniStatus](t, beforeFrame)
	beforeDecisions := decideEveryFeature(d.Settings())
	if before.Character != protocol.OniDefaultCharacter || !before.Enabled || !before.Master || !before.Active {
		t.Fatalf("status before selecting = %+v", before)
	}
	if moth, ok := oniCharacterByID(before.Characters, "testmoth"); !ok || !moth.Valid || moth.Shipped || len(moth.Fallbacks) != 0 || len(moth.Errors) != 0 {
		t.Fatalf("testmoth listed as %+v (present %v)", moth, ok)
	}

	selectedFrame := call(protocol.MOniCharacterSelect, protocol.OniCharacterSelectParams{ID: "testmoth"})
	selected := oniResult[protocol.OniStatus](t, selectedFrame)
	if selected.Character != "testmoth" {
		t.Fatalf("selected character = %q", selected.Character)
	}
	if stored, err := config.Load(); err != nil || stored.Oni.Character != "testmoth" {
		t.Fatalf("stored character = %q, %v", stored.Oni.Character, err)
	}
	statusFrame := call(protocol.MOniStatus, protocol.Empty{})
	if got := oniResult[protocol.OniStatus](t, statusFrame); got.Character != "testmoth" {
		t.Fatalf("status after selecting names %q", got.Character)
	}

	for _, f := range []protocol.Frame{selectedFrame, statusFrame} {
		if !bytes.Equal(capabilitiesBytes(t, beforeFrame), capabilitiesBytes(t, f)) {
			t.Fatalf("capabilities changed with the character:\n%s\n%s", capabilitiesBytes(t, beforeFrame), capabilitiesBytes(t, f))
		}
	}
	afterDecisions := decideEveryFeature(d.Settings())
	if !reflect.DeepEqual(beforeDecisions, afterDecisions) {
		t.Fatalf("decisions changed with the character: %+v, %+v", beforeDecisions, afterDecisions)
	}
	allowed, refused := 0, 0
	for i, c := range selected.Capabilities.Features {
		if c.Allow != afterDecisions[i].Allow || c.Reason != afterDecisions[i].Reason {
			t.Fatalf("status capability %s = %+v, policy decides %+v", c.ID, c, afterDecisions[i])
		}
		if c.Allow {
			allowed++
		} else {
			refused++
		}
	}
	if allowed == 0 || refused == 0 || len(afterDecisions) != len(feature.Features()) {
		t.Fatalf("the settings must exercise both answers: %d allowed, %d refused", allowed, refused)
	}

	presentation := selected
	presentation.Character = before.Character
	if !reflect.DeepEqual(before, presentation) {
		t.Fatalf("more than the character changed:\n%+v\n%+v", before, selected)
	}

	reset := oniResult[protocol.OniStatus](t, call(protocol.MOniCharacterReset, protocol.Empty{}))
	if reset.Character != protocol.OniDefaultCharacter || !reflect.DeepEqual(reset, before) {
		t.Fatalf("reset status = %+v, want %+v", reset, before)
	}
	if stored, err := config.Load(); err != nil || stored.Oni.Character != protocol.OniDefaultCharacter {
		t.Fatalf("stored character after reset = %q, %v", stored.Oni.Character, err)
	}
}

func TestOniRefusesACharacterClaimingCapabilities(t *testing.T) {
	d := oniDaemon(t)
	installOniCharacter(t, "testmoth", "testmoth", nil)
	installOniCharacter(t, "broken-capabilities", "hostile", nil)
	call := dialOni(t, d, protocol.ClientApp, true)

	listed := oniResult[protocol.OniCharactersResult](t, call(protocol.MOniCharacters, protocol.Empty{}))
	hostile, ok := oniCharacterByID(listed.Characters, "hostile")
	if !ok || hostile.Valid || len(hostile.Errors) == 0 {
		t.Fatalf("hostile listed as %+v (present %v)", hostile, ok)
	}
	if !(profile.Report{Errors: hostile.Errors}).Has(profile.ReasonAuthorityKey) {
		t.Fatalf("hostile rejected for %q, want %s", hostile.Errors, profile.ReasonAuthorityKey)
	}

	selectedFrame := call(protocol.MOniCharacterSelect, protocol.OniCharacterSelectParams{ID: "testmoth"})
	selected := oniResult[protocol.OniStatus](t, selectedFrame)
	refusal := call(protocol.MOniCharacterSelect, protocol.OniCharacterSelectParams{ID: "hostile"})
	if errorCode(refusal) != protocol.CodeBadRequest || !strings.Contains(refusal.Error.Message, profile.ReasonAuthorityKey) {
		t.Fatalf("selecting hostile answered %+v", refusal.Error)
	}
	if missing := call(protocol.MOniCharacterSelect, protocol.OniCharacterSelectParams{ID: "nobody"}); errorCode(missing) != protocol.CodeNotFound {
		t.Fatalf("selecting a missing character answered %+v", missing.Error)
	}
	if bad := call(protocol.MOniCharacterSelect, protocol.OniCharacterSelectParams{ID: "../oni"}); errorCode(bad) != protocol.CodeBadRequest {
		t.Fatalf("selecting a malformed id answered %+v", bad.Error)
	}

	afterFrame := call(protocol.MOniStatus, protocol.Empty{})
	after := oniResult[protocol.OniStatus](t, afterFrame)
	if after.Character != "testmoth" || !reflect.DeepEqual(after, selected) {
		t.Fatalf("a refused selection changed the status:\n%+v\n%+v", selected, after)
	}
	if !bytes.Equal(capabilitiesBytes(t, selectedFrame), capabilitiesBytes(t, afterFrame)) {
		t.Fatal("a refused selection changed the capabilities")
	}
	if stored, err := config.Load(); err != nil || stored.Oni.Character != "testmoth" {
		t.Fatalf("stored character after refusal = %q, %v", stored.Oni.Character, err)
	}
}

func TestOniListsTheExactFallbackOfAMissingAnimation(t *testing.T) {
	d := oniDaemon(t)
	installOniCharacter(t, "testmoth", "partial", func(p map[string]any) {
		delete(p["animations"].(map[string]any)["states"].(map[string]any), "listening")
	})
	call := dialOni(t, d, protocol.ClientApp, true)

	listed := oniResult[protocol.OniCharactersResult](t, call(protocol.MOniCharacters, protocol.Empty{}))
	if len(listed.Characters) != 2 || listed.Characters[0].ID != protocol.OniDefaultCharacter || !listed.Characters[0].Shipped || !listed.Characters[0].Valid {
		t.Fatalf("characters = %+v", listed.Characters)
	}
	if listed.Active != protocol.OniDefaultCharacter {
		t.Fatalf("active = %q", listed.Active)
	}
	partial := listed.Characters[1]
	want := []protocol.OniFallback{{Slot: "animations.states.listening", Reason: profile.FallbackMissing, Used: profile.UsedStaticPose}}
	if partial.ID != "partial" || !partial.Valid || len(partial.Errors) != 0 || !reflect.DeepEqual(partial.Fallbacks, want) {
		t.Fatalf("partial listed as %+v, want fallbacks %+v", partial, want)
	}
	if status := oniResult[protocol.OniStatus](t, call(protocol.MOniCharacterSelect, protocol.OniCharacterSelectParams{ID: "partial"})); status.Character != "partial" {
		t.Fatalf("a character with a fallback was not selected: %q", status.Character)
	}
}

func TestOniSelectionIsTheAppsAloneOnTheWire(t *testing.T) {
	d := oniDaemon(t)
	installOniCharacter(t, "testmoth", "testmoth", nil)
	cli := dialOni(t, d, protocol.ClientCLI, false)

	before := oniResult[protocol.OniStatus](t, cli(protocol.MOniStatus, protocol.Empty{}))
	if f := cli(protocol.MOniCharacterSelect, protocol.OniCharacterSelectParams{ID: "testmoth"}); errorCode(f) != protocol.CodeForbidden {
		t.Fatalf("a CLI client selected a character: %+v", f.Error)
	}
	app := dialOni(t, d, protocol.ClientApp, true)
	oniResult[protocol.OniStatus](t, app(protocol.MOniCharacterSelect, protocol.OniCharacterSelectParams{ID: "testmoth"}))
	if f := cli(protocol.MOniCharacterReset, protocol.Empty{}); errorCode(f) != protocol.CodeForbidden {
		t.Fatalf("a CLI client reset the character: %+v", f.Error)
	}
	after := oniResult[protocol.OniStatus](t, cli(protocol.MOniStatus, protocol.Empty{}))
	if before.Character != protocol.OniDefaultCharacter || after.Character != "testmoth" {
		t.Fatalf("characters = %q then %q", before.Character, after.Character)
	}
	if stored, err := config.Load(); err != nil || stored.Oni.Character != "testmoth" {
		t.Fatalf("stored character = %q, %v", stored.Oni.Character, err)
	}
}

func TestOniSpecServesTheShippedSpecAndContract(t *testing.T) {
	d := oniDaemon(t)
	call := dialOni(t, d, protocol.ClientCLI, false)
	result := oniResult[protocol.OniSpecResult](t, call(protocol.MOniSpec, protocol.Empty{}))
	served, err := spec.Parse(result.Spec)
	if err != nil {
		t.Fatalf("the served spec does not parse strictly: %v", err)
	}
	shipped, err := spec.Load()
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(served, shipped) {
		t.Fatal("the served spec differs from the shipped one")
	}
	if !reflect.DeepEqual(result.Features, feature.Features()) || !reflect.DeepEqual(result.Frames, feature.Kinds()) || !reflect.DeepEqual(result.Calls, feature.ReservedCalls()) {
		t.Fatalf("contract = %+v", result)
	}
	for _, name := range result.Calls {
		if _, ok := protocol.Lookup(name); ok {
			t.Fatalf("reserved call %s is registered", name)
		}
		if f := call(name, protocol.Empty{}); errorCode(f) != protocol.CodeUnsupported {
			t.Fatalf("reserved call %s answered %+v", name, f.Error)
		}
	}
}
