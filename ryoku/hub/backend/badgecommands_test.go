package main

import (
	"bytes"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"testing"
)

func TestBadgeCommandsPublishInspectHolderRevoke(t *testing.T) {
	owner, _, _ := badgeTestKeys(t)
	setCardMachine(t)
	t.Setenv("XDG_DATA_HOME", t.TempDir())
	t.Setenv("PATH", t.TempDir())
	seed, e := badgeDeviceSeed()
	if e != nil {
		t.Fatal(e)
	}
	request := requestCode(badgeDevicePublic(seed))
	key := filepath.Join(t.TempDir(), "owner.key")
	if e = os.WriteFile(key, []byte(base64.StdEncoding.EncodeToString(owner)), 0600); e != nil {
		t.Fatal(e)
	}
	wire := teamWireList{V: 1, Badges: []string{}, Revoked: []string{}, Designs: []string{}}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v1/list":
			json.NewEncoder(w).Encode(wire)
		case "/v1/badges":
			raw, _ := io.ReadAll(r.Body)
			b, e := badgeParse(string(raw))
			if e != nil {
				t.Error(e)
				w.WriteHeader(400)
				return
			}
			wire.Badges = append(wire.Badges, b.Code)
			json.NewEncoder(w).Encode(map[string]string{"id": b.ID})
		case "/v1/revoke":
			raw, _ := io.ReadAll(r.Body)
			wire.Revoked = append(wire.Revoked, string(raw))
			w.Write([]byte(`{}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()
	t.Setenv("RYOKU_TEAM_URL", server.URL)
	out := captureStdout(t, func() {
		if e = badgeCommand([]string{"issue", "--issuer", "owner", "--key-file", key, "--role", "core", "--login", "neur0map", "--request", request}); e != nil {
			t.Fatal(e)
		}
	})
	var issued struct {
		Code      string `json:"code"`
		ID        string `json:"id"`
		Published bool   `json:"published"`
	}
	if e = json.Unmarshal([]byte(out), &issued); e != nil || !issued.Published {
		t.Fatal(out, e)
	}
	out = captureStdout(t, func() {
		if e = badgeCommand([]string{"activate", issued.Code}); e != nil {
			t.Fatal(e)
		}
	})
	var status badgeStatusResult
	if e = json.Unmarshal([]byte(out), &status); e != nil || status.Badge == nil || !status.Published || status.Owner || status.CardID != cardIDText(badgeDevicePublic(seed)) {
		t.Fatal(out, e)
	}
	for _, command := range [][]string{{"status"}, {"inspect", issued.Code}, {"list", "--refresh"}, {"holder", "neur0map"}} {
		out = captureStdout(t, func() {
			if e = badgeCommand(command); e != nil {
				t.Fatal(e)
			}
		})
		if !json.Valid([]byte(out)) {
			t.Fatal("non-JSON output", out)
		}
		if command[0] == "holder" && !bytes.Contains([]byte(out), []byte(`"roleIds":[4]`)) {
			t.Fatal("role IDs not numeric array", out)
		}
	}
	if result := badgeInspect("RYB-!!!"); result["kind"] != "badge" || result["valid"] != false {
		t.Fatal(result)
	}
	out = captureStdout(t, func() {
		if e = badgeCommand([]string{"revoke", "--issuer", "owner", "--key-file", key, issued.ID}); e != nil {
			t.Fatal(e)
		}
	})
	if !bytes.Contains([]byte(out), []byte(`"published":true`)) {
		t.Fatal(out)
	}
	status, e = badgeStatus(false)
	if e != nil || status.Badge != nil || status.Published {
		t.Fatal(status, e)
	}
}

// The gate's answer may carry badges for other cards; only the badge whose card id matches
// vouches for the record, however high another badge's role.
func TestRecordSelectsOnlyMatchingCardID(t *testing.T) {
	owner, _, device := badgeTestKeys(t)
	setCardMachine(t)
	g := newFakeGate(t)
	id := cardID(badgeDevicePublic(device))
	other := bytes.Repeat([]byte{8}, 32)
	g.badges = []string{badgeTestCode(t, 1, 5, owner, other, "other"), badgeTestCode(t, 1, 4, owner, device, "neur0map")}
	record, e := recordSign(codedDesign{cardID: id, name: "real"}, nil, device, 1000, "")
	if e != nil {
		t.Fatal(e)
	}
	g.records[record.ID] = record
	entry, e := addCard(sharedCard(t, codedDesign{cardID: id, name: "fake"}, "png"))
	if e != nil || !entry.Verified || entry.Name != "real" || entry.Badge.Login != "neur0map" || len(g.batches) != 1 {
		t.Fatal(entry, e, g.batches)
	}
	if s, _ := pullLoad(); len(s.Badges) != 1 {
		t.Fatal("kept a badge for a card not in the binder", s.Badges)
	}
}

func TestBadgeV2LayoutsAndInspectShapes(t *testing.T) {
	owner, _, device := badgeTestKeys(t)
	request := requestCode(badgeDevicePublic(device))
	badge := badgeTestCode(t, 1, 4, owner, device, "neur0map")
	b, _ := badgeParse(badge)
	revoke, _ := revokeSign(1, owner, b.ID, 319)
	var sha [32]byte
	approval, _ := rydSign(1, owner, sha, "neur0map", 1794787201)
	tests := []struct {
		prefix, text, domain string
		length               int
		keys                 []string
	}{
		{"RYQ", request, "", 35, []string{"kind", "valid", "cardId", "device"}},
		{"RYB", badge, "ryoku-badge-v2", 110, []string{"kind", "valid", "id", "role", "roleId", "login", "issuer", "issued", "cardId", "device"}},
		{"RYX", revoke, "ryoku-revoke-v2", 76, []string{"kind", "valid", "issuer", "target", "issued"}},
		{"RYD", approval, "ryoku-design-v2", 111, []string{"kind", "valid", "issuer", "sha", "login", "approvedAt"}},
	}
	for _, tc := range tests {
		t.Run(tc.prefix, func(t *testing.T) {
			raw, e := parseCodeText(tc.prefix, tc.text)
			if e != nil || len(raw) != tc.length || raw[0] != 2 {
				t.Fatal("wrong v2 layout", len(raw), e)
			}
			fields := badgeInspect(tc.text)
			keys := make([]string, 0, len(fields))
			for key := range fields {
				keys = append(keys, key)
			}
			sort.Strings(keys)
			sort.Strings(tc.keys)
			if !reflect.DeepEqual(keys, tc.keys) || fields["valid"] != true {
				t.Fatal("inspect shape", fields)
			}
			raw[0] = 1
			if tc.prefix == "RYQ" {
				binary.BigEndian.PutUint16(raw[33:], crc16(raw[:33]))
			} else {
				payload := raw[:len(raw)-64]
				copy(raw[len(raw)-64:], signDomain(owner, tc.domain, payload))
			}
			if badgeInspect(codeText(tc.prefix, raw))["valid"] != false {
				t.Fatal("signed legacy version accepted")
			}
		})
	}
	parsed, e := rydParse(approval)
	if e != nil || parsed.ApprovedAt != "2026-11-16T00:00:01Z" || parsed.approvedAt != 1794787201 {
		t.Fatal("approval timestamp", parsed, e)
	}
	raw, _ := parseCodeText("RYB", badge)
	if !bytes.Equal(raw[3:35], badgeDevicePublic(device)) || binary.BigEndian.Uint16(raw[35:37]) != 318 || raw[37] != 8 {
		t.Fatal("badge field offsets")
	}
	raw, _ = parseCodeText("RYD", approval)
	if binary.BigEndian.Uint32(raw[34:38]) != 1794787201 || raw[38] != 8 {
		t.Fatal("approval field offsets")
	}
	holder := teamHolder(teamVerifiedList{Badges: []badgeInfo{b, b}}, "neur0map")
	if !reflect.DeepEqual(holder["cardIds"], []string{b.CardID}) || len(holder) != 4 {
		t.Fatal("holder shape or dedupe", holder)
	}
}

func TestRecordCannotClaimAnotherDeviceCardID(t *testing.T) {
	owner, _, device := badgeTestKeys(t)
	other := bytes.Repeat([]byte{8}, 32)
	b, _ := badgeParse(badgeTestCode(t, 1, 4, owner, device, "victim"))
	attacker, _ := badgeParse(badgeTestCode(t, 1, 1, owner, other, "attacker"))
	d := codedDesign{cardID: cardID(b.device), name: "victim"}
	record, e := recordSign(d, nil, device, 1000, "")
	if e != nil {
		t.Fatal(e)
	}
	if _, e := recordSign(d, nil, other, 1000, ""); e == nil {
		t.Fatal("signed another device's namespace")
	}
	payload, _ := recordPayload(record)
	record.Sig = base64.StdEncoding.EncodeToString(signDomain(other, "ryoku-card-v2", payload))
	for _, badge := range []badgeInfo{b, attacker} {
		if _, e := recordVerify(record, badge); e == nil {
			t.Fatal("foreign device record verified")
		}
	}
	record, _ = recordSign(d, nil, device, 1000, "")
	body, _ := base64.StdEncoding.DecodeString(record.Body)
	body[6] ^= 1
	record.Body = base64.StdEncoding.EncodeToString(body)
	if _, e := recordPayload(record); e == nil {
		t.Fatal("body id mismatch accepted")
	}
}

func TestKnownHolderRecordFailuresRemainUnverified(t *testing.T) {
	owner, _, device := badgeTestKeys(t)
	b, _ := badgeParse(badgeTestCode(t, 1, 4, owner, device, "neur0map"))
	design := codedDesign{cardID: cardID(b.device), name: "forged words"}
	foreign := bytes.Repeat([]byte{8}, 32)
	// another machine signs the badge holder's card and offers its own key as the device
	signedElsewhere := cardRecord{V: 2, ID: b.CardID, Updated: 1000, Body: base64.StdEncoding.EncodeToString(design.bytes()),
		Device: base64.StdEncoding.EncodeToString(badgeDevicePublic(foreign))}
	payload, _ := recordPayload(signedElsewhere)
	signedElsewhere.Sig = base64.StdEncoding.EncodeToString(signDomain(foreign, "ryoku-card-v2", payload))
	for name, record := range map[string]any{
		"missing":        nil,
		"old version":    map[string]any{"v": 1, "id": b.CardID, "updated": 1000},
		"wrong id":       map[string]any{"v": 2, "id": "wrong", "updated": 1000},
		"foreign device": signedElsewhere,
	} {
		t.Run(name, func(t *testing.T) {
			setCardMachine(t)
			g := newFakeGate(t)
			g.badges = []string{b.Code}
			if record != nil {
				g.extra = []any{record}
			}
			got, e := addCard(sharedCard(t, codedDesign{cardID: cardID(b.device), name: "picture facts"}, "png"))
			if e != nil || got.BadgeCheck != "unverified" || got.Verified || got.Badge != nil || got.Design != nil || got.Name != "picture facts" || got.Record != "" {
				t.Fatal("untrusted picture gained badge", got, e)
			}
		})
	}
}
