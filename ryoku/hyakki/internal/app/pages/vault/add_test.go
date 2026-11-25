package vault

import (
	"testing"

	"ryoku-rashin/internal/protocol"
)

func TestRuntimeAddNeedsConsentAndClearsItBetweenDialogs(t *testing.T) {
	var d addDialog
	d.open(protocol.CredentialHeader, nil)
	d.how = sentRuntime
	d.name.SetText("RUNTIME_SECRET")
	d.value.SetText("a shapeless process value")
	d.header.SetText("X-Old-Header")
	d.prefix.SetText("Bearer ")
	d.hosts.SetText("previous.example.org")
	p := d.params(nil)
	if d.problemWith(p) == "" {
		t.Fatal("runtime form is submittable without explicit consent")
	}
	if p.Header != "" || p.Prefix != "" || len(p.Hosts) != 0 {
		t.Fatal("runtime selection carried hidden host-bound form metadata")
	}
	d.consent.Value = true
	p = d.params(nil)
	if d.problemWith(p) != "" {
		t.Fatal("the consented runtime form remained blocked")
	}
	d.clear()
	if d.consent.Value || d.value.Text() != "" {
		t.Fatal("closing retained consent or plaintext")
	}
	d.open(protocol.CredentialHeader, nil)
	d.how = sentRuntime
	if d.consent.Value {
		t.Fatal("reopening silently reused consent")
	}
}
