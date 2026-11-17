package main

import (
	"bytes"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// A worded card is a v4 record: the v2 payload, then the style, the tagline and the Stand's name,
// each as a u16 big-endian length and its UTF-8 bytes (empty when absent), signed under
// ryoku-card-v4, so no word can be added, swapped, moved between fields or stripped.
func TestWordedRecordSignsV4(t *testing.T) {
	owner, _, device := badgeTestKeys(t)
	id := cardID(badgeDevicePublic(device))
	b, _ := badgeParse(badgeTestCode(t, 1, 4, owner, device, "neur0map"))
	d := codedDesign{cardID: id, name: "stand user", kind: 1, gallerySrc: "hawk", threads: 32}
	look := recordLook{Style: "stand", Tagline: "力と美のために, Café", StandName: "Star Platinum"}
	worded, e := recordSign(d, nil, device, 1000, look)
	if e != nil || worded.V != 4 || worded.Tagline != look.Tagline || worded.StandName != look.StandName {
		t.Fatal(worded, e)
	}
	classic, _ := recordSign(d, nil, device, 1000, recordLook{})
	v2, _ := recordPayload(classic)
	v4, e := recordPayload(worded)
	want := append([]byte(nil), v2...)
	for _, s := range []string{look.Style, look.Tagline, look.StandName} {
		want = append(binary.BigEndian.AppendUint16(want, uint16(len(s))), s...)
	}
	if e != nil || !bytes.Equal(v4, want) {
		t.Fatalf("v4 payload is not the v2 payload and three prefixed fields: %x, %v", v4, e)
	}
	sig, _ := base64.StdEncoding.DecodeString(worded.Sig)
	if !verifyDomain(badgeDevicePublic(device), "ryoku-card-v4", v4, sig) || verifyDomain(badgeDevicePublic(device), "ryoku-card-v3", v4, sig) {
		t.Fatal("v4 record is not signed under ryoku-card-v4")
	}
	if got, e := recordVerify(worded, b); e != nil || got.name != d.name {
		t.Fatal(got, e)
	}
	// words alone, on a classic card, are v4 too; a style alone stays v3 for older binders
	if r, e := recordSign(d, nil, device, 1000, recordLook{Tagline: "Yare yare daze."}); e != nil || r.V != 4 || r.Style != "" {
		t.Fatal(r, e)
	}
	if r, _ := recordSign(d, nil, device, 1000, recordLook{Style: "stand"}); r.V != 3 {
		t.Fatal("a style alone left v3", r)
	}
	tampered := map[string]func(*cardRecord){
		"tagline":  func(r *cardRecord) { r.Tagline = "Muda muda muda." },
		"name":     func(r *cardRecord) { r.StandName = "The World" },
		"swapped":  func(r *cardRecord) { r.Tagline, r.StandName = r.StandName, r.Tagline },
		"moved":    func(r *cardRecord) { r.Tagline, r.StandName = r.Tagline+"Star", " Platinum"[1:] },
		"stripped": func(r *cardRecord) { r.Tagline, r.StandName = "", "" },
		"down":     func(r *cardRecord) { r.V = 3 },
		"style":    func(r *cardRecord) { r.Style = "" },
	}
	for name, change := range tampered {
		r := worded
		change(&r)
		if _, e := recordVerify(r, b); e == nil {
			t.Fatal(name, "record verified")
		}
	}
	for _, words := range []string{"a\u202eb", "a\u200bb", "a\u200db", "<b>", "a & b", " lead", "trail ", "e\u0301", "a\u2028b", "a\x07b", "a\u3164b", strings.Repeat("a", 129)} {
		if _, e := recordSign(d, nil, device, 1000, recordLook{Tagline: words}); e == nil {
			t.Fatalf("%q signed as words", words)
		}
	}
	if _, e := recordSign(d, nil, device, 1000, recordLook{StandName: strings.Repeat("ザ", 65)}); e == nil {
		t.Fatal("an oversized Stand name signed")
	}
	if _, e := recordSign(d, nil, device, 1000, recordLook{Tagline: strings.Repeat("力", 128), StandName: strings.Repeat("ザ", 64)}); e != nil {
		t.Fatal("words at their caps refused", e)
	}
}

// Profile words become record words trimmed, in NFC and cut to their cap; words that still
// break the rule travel as nothing.
func TestCardWordsFromProfile(t *testing.T) {
	for in, want := range map[string]string{
		"  Yare yare daze.  ":           "Yare yare daze.",
		"Cafe\u0301":                    "Café",
		strings.Repeat("力", 200):        strings.Repeat("力", 128),
		strings.Repeat("a", 127) + " b": strings.Repeat("a", 127),
		"a\u202eb":                      "",
		"<b>bold</b>":                   "",
		"":                              "",
	} {
		if got := cardWordsFrom(in, recordTaglineLimit); got != want {
			t.Fatalf("%q -> %q, want %q", in, got, want)
		}
	}
}

// Only a badge holder's card carries words; the stock motto is never sent, a Stand's name only
// with the Stand style, and changing the words publishes again.
func TestRecordSyncSendsWordsOnlyForHolder(t *testing.T) {
	owner, _, _ := badgeTestKeys(t)
	setCardMachine(t)
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	g := newFakeGate(t)
	setProfile := func(profile string) {
		t.Helper()
		if e := atomicWrite(profileConfigPath(), []byte(profile), 0o644); e != nil {
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
	setProfile(`{"style":"stand","text":{"name":"Jotaro","tagline":"Yare yare daze.","standName":"Star Platinum"}}`)
	if r := sync(); r.V != 2 || r.Tagline != "" || r.StandName != "" || g.puts != 1 {
		t.Fatal("a machine without a badge published words", r, g.puts)
	}
	seed, _ := badgeDeviceSeed()
	code := badgeTestCode(t, 1, 4, owner, seed, "neur0map")
	if e := badgeActivate(code); e != nil {
		t.Fatal(e)
	}
	b, _ := badgeParse(code)
	r := sync()
	if r.V != 4 || r.Style != "stand" || r.Tagline != "Yare yare daze." || r.StandName != "Star Platinum" || g.puts != 2 {
		t.Fatal("a holder's words were not published", r, g.puts)
	}
	if _, e := recordVerify(r, b); e != nil {
		t.Fatal(e)
	}
	if sync(); g.puts != 2 {
		t.Fatal("unchanged words were published again")
	}
	setProfile(`{"style":"stand","text":{"name":"Jotaro","tagline":"Ora ora ora!","standName":"Star Platinum"}}`)
	if r := sync(); r.Tagline != "Ora ora ora!" || g.puts != 3 {
		t.Fatal("changed words were not published", r, g.puts)
	}
	// the stock motto, and a Stand's name on a classic card, travel as nothing
	setProfile(`{"style":"stand","text":{"name":"Jotaro","tagline":"For the sake of power and beauty."}}`)
	if r := sync(); r.V != 3 || r.Tagline != "" || r.StandName != "" || g.puts != 4 {
		t.Fatal("the stock motto was sent", r)
	}
	setProfile(`{"style":"classic","text":{"name":"Jotaro","standName":"Star Platinum"}}`)
	if r := sync(); r.V != 2 || r.StandName != "" || g.puts != 5 {
		t.Fatal("a classic card sent a Stand's name", r)
	}
	setProfile(`{"style":"classic","text":{"name":"Jotaro","tagline":"Yare yare daze."}}`)
	if r := sync(); r.V != 4 || r.Style != "" || r.Tagline != "Yare yare daze." || g.puts != 6 {
		t.Fatal("a classic holder's motto was not sent", r)
	}
}

// A friend's words are drawn only when the gate vouches for an active badge whose device signed
// the record; otherwise the card keeps the code's own line and Ryoku names the Stand.
func TestBinderDrawsWordsOnlyThroughItsBadge(t *testing.T) {
	owner, _, device := badgeTestKeys(t)
	setCardMachine(t)
	g := newFakeGate(t)
	id := cardID(badgeDevicePublic(device))
	code := badgeTestCode(t, 1, 4, owner, device, "neur0map")
	d := codedDesign{cardID: id, name: "record name", tagline: "first 20 bytes only", kind: 2, threads: 24}
	look := recordLook{Style: "stand", Tagline: "The whole motto, longer than the card code carries.", StandName: "Hierophant Green"}
	r, e := recordSign(d, nil, device, 1000, look)
	if e != nil {
		t.Fatal(e)
	}
	g.records[r.ID] = r
	g.badges = []string{code}
	entry, e := addCard(sharedCard(t, codedDesign{cardID: id, name: "picture name", kind: 2}, "png"))
	if e != nil || !entry.Verified || entry.Tagline != look.Tagline || entry.StandName != look.StandName || entry.Style != "stand" {
		t.Fatal("a badge-verified record's words were not drawn", entry, e)
	}
	g.badges = nil
	makeDue(t)
	if c := listed(t).Cards[0]; c.Verified || c.Tagline != d.tagline || c.StandName != "" || c.Record != "live" {
		t.Fatal("words were drawn without the gate vouching for a badge", c)
	}
	// a badge another machine holds vouches for that machine's card, not this one
	g.badges = []string{badgeTestCode(t, 1, 4, owner, bytes.Repeat([]byte{9}, 32), "friend")}
	makeDue(t)
	if c := listed(t).Cards[0]; c.Verified || c.Tagline != d.tagline || c.StandName != "" {
		t.Fatal("another device's badge drew the words", c)
	}
	g.badges = []string{code}
	makeDue(t)
	if c := listed(t).Cards[0]; c.Tagline != look.Tagline || c.StandName != look.StandName {
		t.Fatal(c)
	}
	// a cached record whose words were edited after it verified no longer verifies
	cache, _ := teamCacheDir()
	path := filepath.Join(cache, "record-"+entry.ID+".json")
	held, _ := os.ReadFile(path)
	var tampered cardRecord
	json.Unmarshal(held, &tampered)
	tampered.Tagline = "Edited after the fact."
	raw, _ := json.Marshal(tampered)
	os.WriteFile(path, raw, 0o600)
	if c := listed(t).Cards[0]; c.Verified || c.Tagline == tampered.Tagline || c.StandName != "" || c.Record != "" {
		t.Fatal("a cached record with edited words verified", c)
	}
}
