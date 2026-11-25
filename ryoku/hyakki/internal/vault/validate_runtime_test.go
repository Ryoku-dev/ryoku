package vault

import (
	"bytes"
	"encoding/json"
	"os"
	"reflect"
	"testing"

	"ryoku-rashin/internal/protocol"
)

func TestRuntimeConsentRefusalDoesNotCreateKeyOrChangeStorage(t *testing.T) {
	keys := &memoryKeeper{}
	v, path := openTest(t, keys)
	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	p := protocol.VaultAddParams{Name: "RUNTIME_SECRET", Use: protocol.CredentialRuntime, Value: "a shapeless export value"}
	if _, err := v.Add(p); err == nil {
		t.Fatal("runtime export accepted without affirmative consent")
	}
	after, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(before, after) || keys.created != 0 || len(v.List()) != 0 {
		t.Fatal("refused consent changed sealed storage or created a key")
	}
	p.RuntimeConsent = true
	c, err := v.Add(p)
	if err != nil {
		t.Fatal(err)
	}
	got, err := v.Reveal(c.ID)
	if err != nil || got != p.Value || c.Use != protocol.CredentialRuntime {
		t.Fatal("consented runtime record did not retain its own value and use")
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(raw, []byte(p.Value)) {
		t.Fatal("runtime value persisted unsealed")
	}
}

func TestRuntimeMetadataRefusalAndImmutableUseLeaveValuesUnchanged(t *testing.T) {
	v, path := openTest(t, &memoryKeeper{})
	header := addGitHub(t, v)
	mail, err := v.Add(protocol.VaultAddParams{Name: "MAIL_PASSWORD", Use: protocol.CredentialMail, Value: "shapeless mail password"})
	if err != nil {
		t.Fatal(err)
	}
	runtime, err := v.Add(protocol.VaultAddParams{Name: "PROCESS_SECRET", Use: protocol.CredentialRuntime, RuntimeConsent: true, Value: "shapeless process secret"})
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range []protocol.Credential{header, mail, runtime} {
		for _, use := range []string{protocol.CredentialHeader, protocol.CredentialMail, protocol.CredentialRuntime} {
			before, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			value, err := v.Reveal(c.ID)
			if err != nil {
				t.Fatal(err)
			}
			patch, _ := json.Marshal(map[string]string{"use": use})
			if _, err := v.Update(protocol.VaultUpdateParams{ID: c.ID, Patch: patch, Value: "replacement must not persist"}); err == nil {
				t.Fatal("a Use patch was accepted")
			}
			after, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			current, ok := v.Get(c.ID)
			opened, err := v.Reveal(c.ID)
			if !ok || err != nil || opened != value || !reflect.DeepEqual(c, current) || !bytes.Equal(before, after) {
				t.Fatal("refused conversion changed metadata, version, value or storage")
			}
		}
	}
	for _, field := range []string{"header", "prefix", "hosts", "trusted"} {
		var value any = "X-Key"
		if field == "hosts" {
			value = []string{"example.org"}
		}
		if field == "trusted" {
			value = []protocol.CredentialTrust{{Team: "team-1", Host: "example.org"}}
		}
		patch, _ := json.Marshal(map[string]any{field: value})
		before, _ := os.ReadFile(path)
		if _, err := v.Update(protocol.VaultUpdateParams{ID: runtime.ID, Patch: patch}); err == nil {
			t.Fatalf("runtime accepted %s metadata", field)
		}
		after, _ := os.ReadFile(path)
		if !bytes.Equal(before, after) {
			t.Fatalf("refused %s patch changed storage", field)
		}
	}
	for _, edit := range []func(*protocol.VaultAddParams){
		func(p *protocol.VaultAddParams) { p.Header = " " },
		func(p *protocol.VaultAddParams) { p.Prefix = "Bearer " },
		func(p *protocol.VaultAddParams) { p.Hosts = []string{"example.org"} },
	} {
		p := protocol.VaultAddParams{Name: "BAD_RUNTIME", Use: protocol.CredentialRuntime, RuntimeConsent: true, Value: "must stay absent"}
		edit(&p)
		before, _ := os.ReadFile(path)
		if _, err := v.Add(p); err == nil {
			t.Fatal("runtime accepted host-bound metadata")
		}
		after, _ := os.ReadFile(path)
		if !bytes.Equal(before, after) {
			t.Fatal("refused add changed storage")
		}
	}
}

func TestRuntimeGrantRefusalCannotAlsoRotateItsValue(t *testing.T) {
	v, path := openTest(t, &memoryKeeper{})
	credential, err := v.Add(protocol.VaultAddParams{
		Name: "PROCESS_SECRET", Use: protocol.CredentialRuntime, RuntimeConsent: true,
		Teams: []string{"existing-team"}, Value: "original process secret",
	})
	if err != nil {
		t.Fatal(err)
	}
	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := v.Update(protocol.VaultUpdateParams{
		ID: credential.ID, Patch: json.RawMessage(`{"teams":["existing-team","new-team"]}`),
		Value: "unapproved replacement",
	}); err == nil {
		t.Fatal("a new process-read grant was accepted without fresh consent")
	}
	after, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	current, exists := v.Get(credential.ID)
	value, err := v.Reveal(credential.ID)
	if !exists || err != nil || value != "original process secret" || !reflect.DeepEqual(current, credential) || !bytes.Equal(before, after) {
		t.Fatal("a refused process-read grant changed grants, value, version or sealed storage")
	}
}

func TestRuntimeGrantConsentAllowsNewTeamsButIsNotNeededForRevocation(t *testing.T) {
	v, _ := openTest(t, &memoryKeeper{})
	credential, err := v.Add(protocol.VaultAddParams{
		Name: "PROCESS_SECRET", Use: protocol.CredentialRuntime, RuntimeConsent: true,
		Teams: []string{"existing-team"}, Value: "original process secret",
	})
	if err != nil {
		t.Fatal(err)
	}
	updated, err := v.Update(protocol.VaultUpdateParams{
		ID: credential.ID, Patch: json.RawMessage(`{"teams":["existing-team","new-team"]}`),
		RuntimeConsent: true,
	})
	if err != nil || !reflect.DeepEqual(updated.Teams, []string{"existing-team", "new-team"}) {
		t.Fatalf("consented process-read grant was not saved: teams=%q err=%v", updated.Teams, err)
	}
	updated, err = v.Update(protocol.VaultUpdateParams{
		ID: credential.ID, Patch: json.RawMessage(`{"teams":["new-team"],"description":"Restricted to the remaining team"}`),
		Value: "rotated process secret",
	})
	if err != nil || !reflect.DeepEqual(updated.Teams, []string{"new-team"}) {
		t.Fatalf("revocation required a new process-read grant: teams=%q err=%v", updated.Teams, err)
	}
	if _, err := v.Update(protocol.VaultUpdateParams{
		ID: credential.ID, Patch: json.RawMessage(`{"teams":["new-team","existing-team"]}`),
	}); err == nil {
		t.Fatal("a revoked process-read grant was restored without fresh consent")
	}
	current, _ := v.Get(credential.ID)
	value, err := v.Reveal(credential.ID)
	if err != nil || value != "rotated process secret" || !reflect.DeepEqual(current.Teams, []string{"new-team"}) || current.Version != 2 {
		t.Fatalf("the refused regrant changed the remaining access or rotated value: credential=%+v err=%v", current, err)
	}
}
