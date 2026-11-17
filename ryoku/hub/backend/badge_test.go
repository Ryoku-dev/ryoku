package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"flag"
	"fmt"
	"image"
	"image/jpeg"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
)

var badgeWriteVectors = flag.Bool("write-vectors", false, "write deterministic team gate vectors")

func badgeTestKeys(t *testing.T) ([]byte, []byte, []byte) {
	t.Helper()
	owner := bytes.Repeat([]byte{1}, 32)
	mika := bytes.Repeat([]byte{2}, 32)
	device := bytes.Repeat([]byte{3}, 32)
	old := badgeIssuerKeys
	badgeIssuerKeys = map[byte][]byte{1: badgeDevicePublic(owner), 2: badgeDevicePublic(mika)}
	t.Cleanup(func() { badgeIssuerKeys = old })
	return owner, mika, device
}
func badgeTestCode(t *testing.T, issuer, role byte, seed, device []byte, login string) string {
	t.Helper()
	req, e := requestParse(requestCode(badgeDevicePublic(device)))
	if e != nil {
		t.Fatal(e)
	}
	code, e := badgeSign(issuer, role, seed, req, login, 318)
	if e != nil {
		t.Fatal(e)
	}
	return code
}
func TestBadgeCodesRoundTripAndForgeries(t *testing.T) {
	owner, mika, device := badgeTestKeys(t)
	id := cardID(badgeDevicePublic(device))
	request := requestCode(badgeDevicePublic(device))
	req, e := requestParse(strings.ToLower(strings.ReplaceAll(request, "-", " \n")))
	if e != nil || req.CardID != cardIDText(badgeDevicePublic(device)) {
		t.Fatal(req, e)
	}
	badReq, _ := parseCodeText("RYQ", request)
	badReq[1] ^= 1
	if _, e = requestParse(codeText("RYQ", badReq)); e == nil {
		t.Fatal("bad CRC accepted")
	}
	for issuer, seed := range map[byte][]byte{1: owner, 2: mika} {
		for role := byte(1); role <= 5; role++ {
			code, e := badgeSign(issuer, role, seed, req, "Ryoku-dev", 318)
			if !badgeRoleAllowed(issuer, role) {
				if e == nil {
					t.Fatal("Mika signed team role")
				}
				continue
			}
			b, e := badgeParse(code)
			if e != nil || b.RoleID != role || b.CardID != req.CardID || b.Login != "Ryoku-dev" {
				t.Fatal(b, e)
			}
			raw, _ := parseCodeText("RYB", code)
			for name, text := range map[string]string{"tampered": codeText("RYB", append(append([]byte{}, raw[:10]...), append([]byte{raw[10] ^ 1}, raw[11:]...)...)), "wrong prefix": strings.Replace(code, "RYB", "RYD", 1), "truncated": code[:len(code)-8]} {
				t.Run(name, func(t *testing.T) {
					if _, e := badgeParse(text); e == nil {
						t.Fatal("forgery accepted")
					}
				})
			}
			raw[2] = 4
			payload := raw[:len(raw)-64]
			if issuer == 2 {
				forged := codeText("RYB", append(payload, signDomain(mika, "ryoku-badge-v2", payload)...))
				if _, e := badgeParse(forged); e == nil {
					t.Fatal("Mika core forgery")
				}
			}
			wrong, _ := badgeSign(issuer, role, bytes.Repeat([]byte{9}, 32), req, "Ryoku-dev", 318)
			if _, e := badgeParse(wrong); e == nil {
				t.Fatal("wrong key accepted")
			}
		}
	}
	for _, login := range []string{"", "-bad", "bad-", "a_b", strings.Repeat("a", 40)} {
		if _, e := badgeSign(1, 1, owner, req, login, 0); e == nil {
			t.Fatal("bad login", login)
		}
	}
	sha := sha256.Sum256([]byte("pack"))
	approval, e := rydSign(1, owner, sha, "neur0map", 1794787200)
	if e != nil {
		t.Fatal(e)
	}
	d, e := rydParse(approval)
	if e != nil || d.Login != "neur0map" {
		t.Fatal(d, e)
	}
	code := badgeTestCode(t, 1, 4, owner, device, "neur0map")
	b, _ := badgeParse(code)
	if b.CardID != fmt.Sprintf("%x", id) {
		t.Fatal("badge id not derived from device")
	}
	rev, e := revokeSign(1, owner, b.ID, 319)
	if e != nil {
		t.Fatal(e)
	}
	r, e := revokeParse(rev)
	if e != nil || r.Target != b.ID {
		t.Fatal(r, e)
	}
	for _, text := range []string{code, approval, rev, request} {
		if badgeInspect(text)["valid"] != true {
			t.Fatal("inspect rejected valid code")
		}
	}
	if _, e := parseCodeText("RYB", strings.Repeat("A", 601)); e == nil {
		t.Fatal("unbounded code accepted")
	}
}
func TestBadgeActivationDevice(t *testing.T) {
	owner, _, other := badgeTestKeys(t)
	setCardMachine(t)
	t.Setenv("XDG_DATA_HOME", t.TempDir())
	seed, e := badgeDeviceSeed()
	if e != nil {
		t.Fatal(e)
	}
	again, e := badgeDeviceSeed()
	if e != nil || !bytes.Equal(seed, again) {
		t.Fatal("device key changed", e)
	}
	dir, _ := badgeDataDir()
	for path, want := range map[string]os.FileMode{dir: 0700, filepath.Join(dir, "device.key"): 0600} {
		st, e := os.Stat(path)
		if e != nil || st.Mode().Perm() != want {
			t.Fatal("key permissions", e)
		}
	}
	code := badgeTestCode(t, 1, 4, owner, seed, "neur0map")
	if e = badgeActivate(code); e != nil {
		t.Fatal(e)
	}
	if _, e = badgeActive(); e != nil {
		t.Fatal(e)
	}
	for _, bad := range []string{badgeTestCode(t, 1, 4, owner, other, "neur0map")} {
		if e = badgeActivate(bad); e == nil {
			t.Fatal("activated foreign badge")
		}
	}
	key := filepath.Join(t.TempDir(), "mika.key")
	os.WriteFile(key, []byte(base64.StdEncoding.EncodeToString(bytes.Repeat([]byte{2}, 32))), 0644)
	// a restrictive umask would have masked the mode WriteFile asked for
	os.Chmod(key, 0644)
	if _, e = loadIssuerSeed("mika", key); e == nil {
		t.Fatal("public key file accepted")
	}
	os.Chmod(key, 0600)
	if _, e = loadIssuerSeed("mika", key); e != nil {
		t.Fatal(e)
	}
}
func TestTeamListIgnoresInvalidAndRevoked(t *testing.T) {
	owner, mika, device := badgeTestKeys(t)
	core := badgeTestCode(t, 1, 4, owner, device, "neur0map")
	tester := badgeTestCode(t, 2, 1, mika, device, "tester")
	b, _ := badgeParse(core)
	rev, _ := revokeSign(2, mika, b.ID, 320)
	sha := sha256.Sum256([]byte("old"))
	old, _ := rydSign(1, owner, sha, "neur0map", 1794787200)
	sha = sha256.Sum256([]byte("new"))
	newer, _ := rydSign(1, owner, sha, "neur0map", 1794787201)
	wire := teamWireList{V: 1, Badges: []string{core, tester, "bad"}, Revoked: []string{rev, "bad"}, Designs: []string{newer, old, "bad"}}
	raw, _ := json.Marshal(wire)
	list, e := teamVerifyList(raw)
	if e != nil || len(list.Badges) != 2 || len(list.Designs) != 1 || list.Designs[0].Code != newer {
		t.Fatal(list, e)
	}
	rev, _ = revokeSign(1, owner, b.ID, 320)
	wire.Revoked = append(wire.Revoked, rev)
	raw, _ = json.Marshal(wire)
	list, e = teamVerifyList(raw)
	if e != nil || len(list.Badges) != 1 || len(list.Designs) != 0 {
		t.Fatal(list, e)
	}
}
func TestCardRecordVerifiesByBadgeOrDevice(t *testing.T) {
	owner, _, device := badgeTestKeys(t)
	id := cardID(badgeDevicePublic(device))
	b, _ := badgeParse(badgeTestCode(t, 1, 4, owner, device, "neur0map"))
	d := codedDesign{cardID: id, name: "record name", tagline: "real words", kind: 1, gallerySrc: "hawk", threads: 32}
	record, e := recordSign(d, nil, device, 1000, recordLook{})
	if e != nil || record.Device != base64.StdEncoding.EncodeToString(badgeDevicePublic(device)) {
		t.Fatal(record, e)
	}
	if _, e = recordVerify(record, b); e != nil {
		t.Fatal(e)
	}
	if got, e := recordVerifyDevice(record); e != nil || got.name != d.name {
		t.Fatal(got, e)
	}
	swapped := b
	swapped.device = badgeDevicePublic(bytes.Repeat([]byte{7}, 32))
	if _, e = recordVerify(record, swapped); e == nil {
		t.Fatal("swapped device accepted")
	}
	bad := record
	bad.Updated++
	if _, e = recordVerify(bad, b); e == nil {
		t.Fatal("tampered record accepted")
	}
	if _, e = recordVerifyDevice(bad); e == nil {
		t.Fatal("tampered device record accepted")
	}
	// another machine's key, offered as the device, owns neither the card id nor the badge
	claimed := record
	claimed.Device = base64.StdEncoding.EncodeToString(swapped.device)
	if _, e = recordVerifyDevice(claimed); e == nil {
		t.Fatal("a device that does not hash to the card id was trusted")
	}
	if _, e = recordVerify(claimed, b); e == nil {
		t.Fatal("a record naming another device verified through the badge")
	}
	short := record
	short.Device = base64.StdEncoding.EncodeToString(badgeDevicePublic(device)[:31])
	loose := record
	last := strings.IndexByte("ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789+/", record.Device[42])
	loose.Device = record.Device[:42] + string("ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789+/"[last^1]) + "="
	for _, r := range []cardRecord{short, loose} {
		if _, e = recordVerifyDevice(r); e == nil {
			t.Fatal("malformed device accepted", r.Device)
		}
	}
	// a record from before devices were published proves itself only through a badge
	old := record
	old.Device = ""
	if _, e = recordVerifyFor(old, teamVerifiedList{Badges: []badgeInfo{b}}); e != nil {
		t.Fatal(e)
	}
	if _, e = recordVerifyFor(old, teamVerifiedList{}); e == nil {
		t.Fatal("a record without device verified without a badge")
	}
	if _, e = recordVerifyFor(record, teamVerifiedList{}); e != nil {
		t.Fatal(e)
	}
}
func TestRecordPictureAndSyncSkipsUnchanged(t *testing.T) {
	owner, _, _ := badgeTestKeys(t)
	id := setCardMachine(t)
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	seed, _ := badgeDeviceSeed()
	code := badgeTestCode(t, 1, 4, owner, seed, "neur0map")
	if e := badgeActivate(code); e != nil {
		t.Fatal(e)
	}
	picturePath := sharedCard(t, codedDesign{cardID: id, kind: 2}, "png")
	picture, e := recordPicturePNG(picturePath)
	if e != nil {
		t.Fatal(e)
	}
	cfg, e := jpeg.DecodeConfig(bytes.NewReader(picture))
	if e != nil || len(picture) > recordPictureLimit || cfg.Width > 1120 || cfg.Height > 780 {
		t.Fatal(cfg, e)
	}
	writes := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "PUT" {
			var request struct {
				Record  cardRecord `json:"record"`
				Picture string     `json:"picture"`
			}
			if e := json.NewDecoder(r.Body).Decode(&request); e != nil {
				t.Error(e)
			}
			b, _ := badgeParse(code)
			d, e := recordVerify(request.Record, b)
			if e != nil || d.threads != 16 {
				t.Error(d, e)
			}
			writes++
			w.Write([]byte(`{}`))
			return
		}
		http.NotFound(w, r)
	}))
	defer srv.Close()
	t.Setenv("RYOKU_TEAM_URL", srv.URL)
	// a badge alone does not put the card online: its owner turns it on
	if ok, e := recordSync([]string{"--threads", "16", "--picture", picturePath}); e != nil || ok || writes != 0 {
		t.Fatal("published before being turned on", ok, e, writes)
	}
	if e := recordSetOnline(true); e != nil {
		t.Fatal(e)
	}
	for range 2 {
		ok, e := recordSync([]string{"--threads", "16", "--picture", picturePath})
		if e != nil || !ok {
			t.Fatal(ok, e)
		}
	}
	if writes != 1 {
		t.Fatal("unchanged record uploaded", writes)
	}
}

// A computer whose clock is hours off, and a first record that reaches a gate replica which has
// not read the new badge yet, still publish; records stay ordered after the gate's clock; any
// other refusal is reported with the gate's reason.
func TestRecordSyncRecoversFromGateRefusals(t *testing.T) {
	owner, _, _ := badgeTestKeys(t)
	setCardMachine(t)
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	seed, _ := badgeDeviceSeed()
	code := badgeTestCode(t, 1, 4, owner, seed, "neur0map")
	if e := badgeActivate(code); e != nil {
		t.Fatal(e)
	}
	b, _ := badgeParse(code)
	wait := recordRetryWait
	recordRetryWait = 0
	t.Cleanup(func() { recordRetryWait = wait })
	gateNow := time.Now().Add(3 * time.Hour).Unix()
	var stored int64
	unseen, refuse := 1, ""
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Date", time.Unix(gateNow, 0).UTC().Format(http.TimeFormat))
		var request struct {
			Record cardRecord `json:"record"`
		}
		json.NewDecoder(r.Body).Decode(&request)
		if _, e := recordVerify(request.Record, b); e != nil {
			t.Error(e)
		}
		fail := func(reason string) {
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]string{"error": reason})
		}
		switch updated := request.Record.Updated; {
		case refuse != "":
			fail(refuse)
		case unseen > 0:
			unseen--
			fail("No active badge verifies this record")
		case updated > gateNow+600 || updated < gateNow-600:
			fail("Invalid card id or timestamp")
		case updated <= stored:
			fail("Card record is not newer")
		default:
			stored = updated
			w.Write([]byte(`{}`))
		}
	}))
	defer srv.Close()
	t.Setenv("RYOKU_TEAM_URL", srv.URL)
	if e := recordSetOnline(true); e != nil {
		t.Fatal(e)
	}
	for threads := 8; threads <= 10; threads++ {
		if ok, e := recordSync([]string{"--threads", strconv.Itoa(threads)}); !ok || e != nil {
			t.Fatal(threads, ok, e)
		}
	}
	if stored != gateNow+2 {
		t.Fatalf("records stamped %d, want the gate's clock %d plus two", stored, gateNow)
	}
	// the clock now agrees with the gate, but the last record went out while it ran five
	// minutes fast: the next one is still stamped after it
	gateNow = time.Now().Unix()
	stored = gateNow + 300
	dir, _ := badgeDataDir()
	if e := atomicWrite(filepath.Join(dir, "record.sha256"), []byte("published "+strconv.FormatInt(stored, 10)), 0o600); e != nil {
		t.Fatal(e)
	}
	if ok, e := recordSync([]string{"--threads", "11"}); !ok || e != nil || stored <= gateNow+300 {
		t.Fatal("a clock that stepped back got its record refused", ok, e)
	}
	refuse = "Picture hash mismatch"
	if _, e := recordSync([]string{"--threads", "12"}); e == nil || !strings.Contains(e.Error(), refuse) {
		t.Fatal("refusal without the gate's reason", e)
	}
}

// Once a friend's card verified, a gate that fails for a moment leaves it verified with the
// record and picture last seen; a cached record that no longer verifies leaves it unverified.
// Without the badge the card stays live through its device, while a record from before devices
// were published falls back to the shared picture.
func TestBinderKeepsTheLastVerifiedRecord(t *testing.T) {
	owner, _, device := badgeTestKeys(t)
	setCardMachine(t)
	g := newFakeGate(t)
	id := cardID(badgeDevicePublic(device))
	code := badgeTestCode(t, 1, 4, owner, device, "friend")
	g.badges = []string{code}
	picture := jpegOf(t, image.NewRGBA(image.Rect(0, 0, 56, 39)), 80)
	d := codedDesign{cardID: id, name: "record name", kind: 2, threads: 24}
	record, e := recordSign(d, picture, device, 1000, recordLook{})
	if e != nil {
		t.Fatal(e)
	}
	g.records[record.ID], g.pictures[record.Picture] = record, picture
	verified := func(c binderCard) bool {
		return c.Verified && c.BadgeCheck == "ok" && c.Badge != nil && c.Name == "record name" && c.HW.Threads == 24 && c.Record == "live" &&
			c.Hero.Kind == "custom" && c.Hero.Rev == record.Picture[:12] && strings.HasSuffix(c.Hero.File, "-record.png")
	}
	entry, e := addCard(sharedCard(t, codedDesign{cardID: id, name: "picture name", kind: 2}, "png"))
	if e != nil || !verified(entry) {
		t.Fatal("record not applied", entry, e)
	}
	makeDue(t)
	if view := listed(t); len(view.Cards) != 1 || !verified(view.Cards[0]) || g.pictureGets != 1 {
		t.Fatal("an unchanged picture was downloaded again", g.pictureGets, view)
	}
	g.block = func(w http.ResponseWriter) { w.WriteHeader(http.StatusInternalServerError) }
	makeDue(t)
	if view := listed(t); view.Paused != "offline" || len(view.Cards) != 1 || !verified(view.Cards[0]) {
		t.Fatal("a failing gate dropped the verified card", view)
	}
	cache, _ := teamCacheDir()
	path := filepath.Join(cache, "record-"+record.ID+".json")
	kept, _ := os.ReadFile(path)
	tampered := record
	tampered.Updated++
	raw, _ := json.Marshal(tampered)
	os.WriteFile(path, raw, 0o600)
	if c := listed(t).Cards[0]; c.Verified || c.BadgeCheck != "unverified" || c.Name != "picture name" || c.Record != "" {
		t.Fatal("a tampered cached record verified", c)
	}
	os.WriteFile(path, kept, 0o600)
	g.block, g.badges = nil, nil
	makeDue(t)
	if c := listed(t).Cards[0]; c.Verified || c.BadgeCheck != "none" || c.Badge != nil || c.Record != "live" || c.Name != "record name" {
		t.Fatal("a badge gone from the gate's answer kept its card verified, or the device record was dropped", c)
	}
	old, _ := recordSign(d, picture, device, 1001, recordLook{})
	old.Device = ""
	g.records[old.ID], g.badges = old, []string{code}
	makeDue(t)
	if c := listed(t).Cards[0]; !verified(c) || c.RecordUpdated != 1001 {
		t.Fatal("a record without its device did not verify through the badge", c)
	}
	g.badges = nil
	makeDue(t)
	if c := listed(t).Cards[0]; c.Verified || c.Record != "" || c.Name != "picture name" {
		t.Fatal("a record without its device stayed trusted without the badge", c)
	}
}
func TestTeamHTTPBoundariesAndCache(t *testing.T) {
	setCardMachine(t)
	for _, url := range []string{"http://example.com", "https://u:p@example.com", "https://example.com/path", "file:///tmp/a"} {
		t.Setenv("RYOKU_TEAM_URL", url)
		if _, e := teamURL(); e == nil {
			t.Fatal("unsafe URL", url)
		}
	}
	requests := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		if r.URL.Path == "/large" {
			w.Write(bytes.Repeat([]byte{'a'}, 65))
			return
		}
		w.Write([]byte(`{"v":1,"badges":[],"revoked":[],"designs":[]}`))
	}))
	defer srv.Close()
	t.Setenv("RYOKU_TEAM_URL", srv.URL)
	if _, _, e := teamGet("/large", 64); e == nil {
		t.Fatal("unbounded response")
	}
	for range 2 {
		if _, e := teamList(false); e != nil {
			t.Fatal(e)
		}
	}
	if requests != 2 {
		t.Fatal("cache not used", requests)
	}
}
func TestWriteVectors(t *testing.T) {
	if !*badgeWriteVectors {
		t.Skip("pass -write-vectors to generate cross-language vectors")
	}
	owner, mika, device := badgeTestKeys(t)
	id := cardID(badgeDevicePublic(device))
	request := requestCode(badgeDevicePublic(device))
	code := badgeTestCode(t, 1, 4, owner, device, "neur0map")
	badge, _ := badgeParse(code)
	revoke, _ := revokeSign(1, owner, badge.ID, 319)
	sha := sha256.Sum256([]byte("deterministic pack"))
	design, _ := rydSign(1, owner, sha, "neur0map", 1794787200)
	vectors := []map[string]any{}
	add := func(kind string, valid bool, fields map[string]any) {
		v := map[string]any{"kind": kind, "valid": valid, "ownerKey": base64.StdEncoding.EncodeToString(badgeDevicePublic(owner)), "mikaKey": base64.StdEncoding.EncodeToString(badgeDevicePublic(mika))}
		for k, x := range fields {
			v[k] = x
		}
		vectors = append(vectors, v)
	}
	for _, kind := range []string{"request", "badge", "revoke", "design"} {
		text := map[string]string{"request": request, "badge": code, "revoke": revoke, "design": design}[kind]
		add(kind, true, map[string]any{"text": text})
		add(kind, false, map[string]any{"text": text[:len(text)-8]})
		legacy, _ := parseCodeText(map[string]string{"request": "RYQ", "badge": "RYB", "revoke": "RYX", "design": "RYD"}[kind], text)
		legacy[0] = 1
		add(kind, false, map[string]any{"text": codeText(map[string]string{"request": "RYQ", "badge": "RYB", "revoke": "RYX", "design": "RYD"}[kind], legacy)})
		raw, _ := parseCodeText(map[string]string{"request": "RYQ", "badge": "RYB", "revoke": "RYX", "design": "RYD"}[kind], text)
		raw[2] ^= 1
		add(kind, false, map[string]any{"text": codeText(map[string]string{"request": "RYQ", "badge": "RYB", "revoke": "RYX", "design": "RYD"}[kind], raw)})
	}
	add("badge", true, map[string]any{"text": code, "id": badge.ID, "cardId": badge.CardID, "role": 4, "issuer": 1, "login": "neur0map", "day": 318})
	add("request", true, map[string]any{"text": request, "cardId": badge.CardID})
	add("design", true, map[string]any{"text": design, "approvedAt": 1794787200})
	raw, _ := parseCodeText("RYB", code)
	raw[1] = 2
	payload := raw[:len(raw)-64]
	add("badge", false, map[string]any{"text": codeText("RYB", append(payload, signDomain(mika, "ryoku-badge-v2", payload)...))})
	add("badge", false, map[string]any{"text": strings.Replace(code, "RYB", "RYD", 1)})
	d := codedDesign{cardID: id, name: "neur0map", tagline: "test vector", kind: 1, gallerySrc: "hawk"}
	record, _ := recordSign(d, nil, device, 1794787200, recordLook{})
	add("card", true, map[string]any{"record": record, "deviceKey": base64.StdEncoding.EncodeToString(badgeDevicePublic(device))})
	bad := record
	bad.Updated++
	add("card", false, map[string]any{"record": bad, "deviceKey": base64.StdEncoding.EncodeToString(badgeDevicePublic(device))})
	bad = record
	bad.V = 1
	add("card", false, map[string]any{"record": bad, "deviceKey": base64.StdEncoding.EncodeToString(badgeDevicePublic(device))})
	bad = record
	bad.ID = cardIDText(badgeDevicePublic(owner))
	add("card", false, map[string]any{"record": bad, "deviceKey": base64.StdEncoding.EncodeToString(badgeDevicePublic(device))})
	add("card", false, map[string]any{"record": record, "deviceKey": base64.StdEncoding.EncodeToString(badgeDevicePublic(owner))})
	// device records prove themselves: the device must hash to the id and sign the record
	add("deviceCard", true, map[string]any{"record": record})
	for _, key := range []string{"", base64.StdEncoding.EncodeToString(badgeDevicePublic(owner)), base64.StdEncoding.EncodeToString(badgeDevicePublic(device)[:31])} {
		bad = record
		bad.Device = key
		add("deviceCard", false, map[string]any{"record": bad})
	}
	bad = record
	bad.Updated++
	add("deviceCard", false, map[string]any{"record": bad})
	// another key claims the card id and signs for it
	bad = record
	bad.Device = base64.StdEncoding.EncodeToString(badgeDevicePublic(owner))
	claimed, _ := recordPayload(bad)
	bad.Sig = base64.StdEncoding.EncodeToString(signDomain(owner, "ryoku-card-v2", claimed))
	add("deviceCard", false, map[string]any{"record": bad})
	// a styled and a worded record, which only a badge's device key vouches for
	deviceKey := base64.StdEncoding.EncodeToString(badgeDevicePublic(device))
	styled, _ := recordSign(d, nil, device, 1794787200, recordLook{Style: "stand"})
	worded, _ := recordSign(d, nil, device, 1794787200, recordLook{Style: "stand", Tagline: "力と美のために, Café 😀", StandName: "Star Platinum"})
	wordsOnly, _ := recordSign(d, nil, device, 1794787200, recordLook{Tagline: "Yare yare daze."})
	for _, r := range []cardRecord{styled, worded, wordsOnly} {
		add("card", true, map[string]any{"record": r, "deviceKey": deviceKey})
		add("deviceCard", false, map[string]any{"record": r})
	}
	bad = worded
	bad.Tagline = "Muda muda muda."
	add("card", false, map[string]any{"record": bad, "deviceKey": deviceKey})
	bad = worded
	bad.Tagline, bad.StandName = worded.StandName, worded.Tagline
	add("card", false, map[string]any{"record": bad, "deviceKey": deviceKey})
	at := int64(1794787200)
	withdrawal, _ := recordDeletePayload(record.ID, at)
	deleteSig := base64.StdEncoding.EncodeToString(signDomain(device, "ryoku-card-delete-v1", withdrawal))
	add("cardDelete", true, map[string]any{"id": record.ID, "at": at, "sig": deleteSig, "deviceKey": deviceKey, "now": at + 600})
	add("cardDelete", false, map[string]any{"id": record.ID, "at": at, "sig": deleteSig, "deviceKey": deviceKey, "now": at + 601})
	add("cardDelete", false, map[string]any{"id": record.ID, "at": at + 1, "sig": deleteSig, "deviceKey": deviceKey, "now": at})
	add("cardDelete", false, map[string]any{"id": record.ID, "at": at, "sig": deleteSig, "deviceKey": base64.StdEncoding.EncodeToString(badgeDevicePublic(owner)), "now": at})
	batch := map[string]int64{record.ID: 1794787000, "0000000000000001": 0}
	holderPayload, _ := pullHolderPayload(badge.ID, at, batch)
	holder := func(seed []byte) map[string]any {
		return map[string]any{"badge": code, "at": at, "sig": base64.StdEncoding.EncodeToString(signDomain(seed, "ryoku-pull-v2", holderPayload))}
	}
	add("pullHolder", true, map[string]any{"holder": holder(device), "cards": batch, "now": at + 600})
	add("pullHolder", false, map[string]any{"holder": holder(device), "cards": map[string]int64{record.ID: 1794787000}, "now": at})
	add("pullHolder", false, map[string]any{"holder": holder(owner), "cards": batch, "now": at})
	add("pullHolder", false, map[string]any{"holder": holder(device), "cards": batch, "now": at + 601})
	flipped := holder(device)
	flippedSig, _ := base64.StdEncoding.DecodeString(flipped["sig"].(string))
	flippedSig[0] ^= 1
	flipped["sig"] = base64.StdEncoding.EncodeToString(flippedSig)
	add("pullHolder", false, map[string]any{"holder": flipped, "cards": batch, "now": at})
	now := int64(1794787200)
	path := "/v1/pending/" + fmt.Sprintf("%x", sha)
	method := "PUT"
	sig := signDomain(owner, "ryoku-upload-v1", []byte(fmt.Sprintf("%s\n%s\n%d\n%x", method, path, now, sha)))
	header := fmt.Sprintf("1.%d.%s", now, base64.StdEncoding.EncodeToString(sig))
	for _, valid := range []bool{true, false} {
		clock := now
		if !valid {
			clock += 301
		}
		add("upload", valid, map[string]any{"header": header, "method": method, "path": path, "sha": fmt.Sprintf("%x", sha), "now": clock})
	}
	add("upload", false, map[string]any{"header": header, "method": "POST", "path": path, "sha": fmt.Sprintf("%x", sha), "now": now})
	rawJSON, e := json.MarshalIndent(vectors, "", "  ")
	if e != nil {
		t.Fatal(e)
	}
	path = filepath.Join("..", "..", "..", "release", "team-badges", "test", "vectors.json")
	if e = os.WriteFile(path, append(rawJSON, '\n'), 0644); e != nil {
		t.Fatal(e)
	}
	t.Logf("wrote %d vectors", len(vectors))
}
