package main

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// A styled card is a v3 record: the v2 payload, then the style as a u16 big-endian length and its
// UTF-8 bytes, signed under its own domain, so the style can't be added, swapped or stripped.
func TestStyledRecordSignsV3(t *testing.T) {
	owner, _, device := badgeTestKeys(t)
	id := cardID(badgeDevicePublic(device))
	b, _ := badgeParse(badgeTestCode(t, 1, 4, owner, device, "neur0map"))
	d := codedDesign{cardID: id, name: "stand user", kind: 1, gallerySrc: "hawk", threads: 32}
	classic, e := recordSign(d, nil, device, 1000, recordLook{})
	if e != nil || classic.V != 2 || classic.Style != "" {
		t.Fatal(classic, e)
	}
	stand, e := recordSign(d, nil, device, 1000, recordLook{Style: "stand"})
	if e != nil || stand.V != 3 || stand.Style != "stand" {
		t.Fatal(stand, e)
	}
	v2, _ := recordPayload(classic)
	v3, e := recordPayload(stand)
	if e != nil || !bytes.Equal(v3, append(append(v2, 0, 5), "stand"...)) {
		t.Fatalf("v3 payload is not the v2 payload, the style's length and the style: %x, %v", v3, e)
	}
	sig, _ := base64.StdEncoding.DecodeString(stand.Sig)
	if !verifyDomain(badgeDevicePublic(device), "ryoku-card-v3", v3, sig) || verifyDomain(badgeDevicePublic(device), "ryoku-card-v2", v3, sig) {
		t.Fatal("v3 record is not signed under ryoku-card-v3")
	}
	if got, e := recordVerify(stand, b); e != nil || got.name != d.name {
		t.Fatal(got, e)
	}
	if _, e := recordVerifyDevice(stand); e != nil {
		t.Fatal(e)
	}
	swapped := stand
	swapped.Style = "other-style"
	stripped := stand
	stripped.Style = ""
	downgraded := stand
	downgraded.V, downgraded.Style = 2, ""
	added := classic
	added.Style = "stand"
	raised := classic
	raised.V, raised.Style = 3, "stand"
	malformed := stand
	malformed.Style = "Stand!"
	for name, r := range map[string]cardRecord{"swapped": swapped, "stripped": stripped, "downgraded": downgraded, "v2 with style": added, "raised": raised, "malformed": malformed} {
		if _, e := recordVerifyDevice(r); e == nil {
			t.Fatal(name, "record verified")
		}
	}
	if _, e := recordSign(d, nil, device, 1000, recordLook{Style: "Stand!"}); e == nil {
		t.Fatal("malformed style signed")
	}
}

// The published card wears the Stand style only when its owner chose it and holds a badge;
// anyone else's goes out a classic v2 record, and changing the style publishes again.
func TestRecordSyncSendsStyleOnlyForHolder(t *testing.T) {
	owner, _, _ := badgeTestKeys(t)
	setCardMachine(t)
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	g := newFakeGate(t)
	setStyle := func(style string) {
		t.Helper()
		if e := atomicWrite(profileConfigPath(), []byte(`{"style":"`+style+`","text":{"name":"Jotaro"}}`), 0o644); e != nil {
			t.Fatal(e)
		}
	}
	sync := func() cardRecord {
		t.Helper()
		if ok, e := recordSync([]string{"--threads", "16"}); !ok || e != nil {
			t.Fatal(ok, e)
		}
		for _, r := range g.records {
			return r
		}
		t.Fatal("nothing published")
		return cardRecord{}
	}
	if e := recordSetOnline(true); e != nil {
		t.Fatal(e)
	}
	setStyle("stand")
	if r := sync(); r.V != 2 || r.Style != "" || g.puts != 1 {
		t.Fatal("a machine without a badge published a style", r, g.puts)
	}
	seed, _ := badgeDeviceSeed()
	code := badgeTestCode(t, 1, 4, owner, seed, "neur0map")
	if e := badgeActivate(code); e != nil {
		t.Fatal(e)
	}
	b, _ := badgeParse(code)
	r := sync()
	if r.V != 3 || r.Style != "stand" || g.puts != 2 {
		t.Fatal("a holder's Stand card was not published styled", r, g.puts)
	}
	if _, e := recordVerify(r, b); e != nil {
		t.Fatal(e)
	}
	if sync(); g.puts != 2 {
		t.Fatal("an unchanged styled card was published again")
	}
	setStyle("classic")
	if r := sync(); r.V != 2 || r.Style != "" || g.puts != 3 {
		t.Fatal("going back to classic did not publish a v2 record", r, g.puts)
	}
}

// A friend's style is drawn only when the gate vouches for an active badge whose device signed
// the record; without one, or for a style this binder doesn't know, the card is drawn classic.
func TestBinderDrawsStyleOnlyThroughItsBadge(t *testing.T) {
	owner, _, device := badgeTestKeys(t)
	setCardMachine(t)
	g := newFakeGate(t)
	id := cardID(badgeDevicePublic(device))
	code := badgeTestCode(t, 1, 4, owner, device, "friend")
	d := codedDesign{cardID: id, name: "record name", kind: 2, threads: 24}
	publish := func(updated int64, style string) cardRecord {
		t.Helper()
		r, e := recordSign(d, nil, device, updated, recordLook{Style: style})
		if e != nil {
			t.Fatal(e)
		}
		g.records[r.ID] = r
		makeDue(t)
		return r
	}
	publish(1000, "stand")
	g.badges = []string{code}
	entry, e := addCard(sharedCard(t, codedDesign{cardID: id, name: "picture name", kind: 2}, "png"))
	if e != nil || !entry.Verified || entry.Style != "stand" || entry.Record != "live" {
		t.Fatal("a badge-verified Stand record was not drawn styled", entry, e)
	}
	g.badges = nil
	makeDue(t)
	if c := listed(t).Cards[0]; c.Verified || c.Style != "" || c.Record != "live" || c.Name != "record name" {
		t.Fatal("a style was drawn without the gate vouching for a badge", c)
	}
	// a badge another machine holds vouches for that machine's card, not this one
	other := bytes.Repeat([]byte{9}, 32)
	g.badges = []string{badgeTestCode(t, 1, 4, owner, other, "friend")}
	makeDue(t)
	if c := listed(t).Cards[0]; c.Verified || c.Style != "" {
		t.Fatal("another device's badge drew the style", c)
	}
	g.badges = []string{code}
	publish(1001, "future-style")
	if c := listed(t).Cards[0]; !c.Verified || c.Style != "" || c.RecordUpdated != 1001 {
		t.Fatal("an unknown style was not drawn classic", c)
	}
	publish(1002, "")
	if c := listed(t).Cards[0]; !c.Verified || c.Style != "" || c.RecordUpdated != 1002 {
		t.Fatal("a v2 record changed", c)
	}
	publish(1003, "stand")
	if c := listed(t).Cards[0]; c.Style != "stand" {
		t.Fatal(c)
	}
	// a cached record whose style was edited after it verified no longer verifies
	cache, _ := teamCacheDir()
	path := filepath.Join(cache, "record-"+entry.ID+".json")
	held, _ := os.ReadFile(path)
	var tampered cardRecord
	json.Unmarshal(held, &tampered)
	for _, style := range []string{"x", "other"} {
		tampered.Style = style
		raw, _ := json.Marshal(tampered)
		os.WriteFile(path, raw, 0o600)
		if c := listed(t).Cards[0]; c.Verified || c.Style != "" || c.Record != "" {
			t.Fatal("a cached record with an edited style verified", style, c)
		}
	}
}
