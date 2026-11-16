package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"flag"
	"fmt"
	"image/jpeg"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
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
func TestCardRecordVerifyReplayAndBinderAuthority(t *testing.T) {
	owner, _, device := badgeTestKeys(t)
	setCardMachine(t)
	t.Setenv("XDG_DATA_HOME", t.TempDir())
	id := cardID(badgeDevicePublic(device))
	code := badgeTestCode(t, 1, 4, owner, device, "neur0map")
	b, _ := badgeParse(code)
	d := codedDesign{cardID: id, name: "record name", tagline: "real words", kind: 1, gallerySrc: "hawk", threads: 32}
	record, e := recordSign(d, nil, device, 1000)
	if e != nil {
		t.Fatal(e)
	}
	if _, e = recordVerify(record, b); e != nil {
		t.Fatal(e)
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
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v1/list":
			json.NewEncoder(w).Encode(teamWireList{V: 1, Badges: []string{code}})
		case "/v1/cards/" + b.CardID:
			json.NewEncoder(w).Encode(record)
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(srv.Close)
	t.Setenv("RYOKU_TEAM_URL", srv.URL)
	if _, _, _, e = recordFetch(b); e != nil {
		t.Fatal(e)
	}
	record, _ = recordSign(d, nil, device, 999)
	if _, _, _, e = recordFetch(b); e == nil {
		t.Fatal("record replay accepted")
	}
	record, _ = recordSign(d, nil, device, 1001)
	imageDesign := codedDesign{cardID: id, name: "forged image", tagline: "fake"}
	entry, e := addCard(sharedCard(t, imageDesign, "png"))
	if e != nil || !entry.Verified || entry.Name != d.name || entry.Tagline != d.tagline || entry.Hero.Source != "hawk.png" || entry.HW.Threads != 32 {
		t.Fatal("record did not override image", entry, e)
	}
	cards, e := listCards()
	if e != nil || len(cards) != 1 || cards[0].Name != d.name {
		t.Fatal(cards, e)
	}
	srv.Close()
	cache, _ := teamCacheDir()
	os.Remove(filepath.Join(cache, "list.json"))
	cards, e = listCards()
	if e != nil || len(cards) != 1 || cards[0].Verified || cards[0].Badge != nil || cards[0].BadgeCheck != "offline" || cards[0].Name != imageDesign.name {
		t.Fatal("offline path", cards, e)
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
		if r.URL.Path == "/v1/list" {
			json.NewEncoder(w).Encode(teamWireList{V: 1, Badges: []string{code}})
			return
		}
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
	record, _ := recordSign(d, nil, device, 1794787200)
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
