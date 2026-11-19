package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"reflect"
	"strings"
	"testing"
)

func badgeTestProgramCode(t *testing.T, issuer, role byte, seed, device []byte, login, program string) string {
	t.Helper()
	req, e := requestParse(requestCode(badgeDevicePublic(device)))
	if e != nil {
		t.Fatal(e)
	}
	code, e := badgeSign(issuer, role, seed, req, login, program, 318)
	if e != nil {
		t.Fatal(e)
	}
	return code
}

// badgeTestRawProgram signs a v3 badge around any program bytes, as an issuer's key could
func badgeTestRawProgram(seed []byte, issuer, role byte, device []byte, login, program string) string {
	b := []byte{3, issuer, role}
	b = append(b, badgeDevicePublic(device)...)
	b = binary.BigEndian.AppendUint16(b, 318)
	b = append(b, byte(len(login)))
	b = append(b, login...)
	b = append(b, byte(len(program)))
	b = append(b, program...)
	return codeText("RYB", append(b, signDomain(seed, "ryoku-badge-v3", b)...))
}

func TestBadgeV3CarriesSignedProgram(t *testing.T) {
	owner, mika, device := badgeTestKeys(t)
	longest := strings.Repeat("Ryoku v3.0+ ", 3) + "Edge"
	for _, tc := range []struct {
		issuer, role   byte
		seed           []byte
		login, program string
	}{
		{2, 1, mika, "tester", "Ryoku v2 Hyperion"},
		{2, 2, mika, "Ryoku-dev", "a"},
		{1, 4, owner, "neur0map", longest},
	} {
		code := badgeTestProgramCode(t, tc.issuer, tc.role, tc.seed, device, tc.login, tc.program)
		raw, _ := parseCodeText("RYB", code)
		sum := sha256.Sum256(raw)
		b, e := badgeParse(code)
		if e != nil || raw[0] != 3 || len(raw) != 38+len(tc.login)+1+len(tc.program)+64 {
			t.Fatal("v3 layout", len(raw), e)
		}
		if b.Program != tc.program || b.Login != tc.login || b.RoleID != tc.role || b.issuer != tc.issuer || b.ID != hex.EncodeToString(sum[:8]) || !bytes.Equal(b.device, badgeDevicePublic(device)) {
			t.Fatal("v3 fields", b)
		}
		if out := badgeInspect(code); out["valid"] != true || out["program"] != tc.program {
			t.Fatal("inspect", out)
		}
		tampered := append([]byte{}, raw...)
		tampered[len(raw)-65] ^= 1
		if _, e := badgeParse(codeText("RYB", tampered)); e == nil {
			t.Fatal("tampered program accepted")
		}
		payload := raw[:len(raw)-64]
		for name, forged := range map[string][]byte{
			"v2 domain":       append(append([]byte{}, payload...), signDomain(tc.seed, "ryoku-badge-v2", payload)...),
			"version 4":       badgeTestResign(tc.seed, append([]byte{4}, payload[1:]...)),
			"trailing byte":   badgeTestResign(tc.seed, append(append([]byte{}, payload...), 'x')),
			"long length":     badgeTestResign(tc.seed, append(append([]byte{}, payload[:len(payload)-len(tc.program)-1]...), append([]byte{byte(len(tc.program) + 1)}, tc.program...)...)),
			"missing program": badgeTestResign(tc.seed, payload[:len(payload)-len(tc.program)-1]),
		} {
			if _, e := badgeParse(codeText("RYB", forged)); e == nil {
				t.Fatal("forged v3 accepted:", name)
			}
		}
	}
	if _, e := badgeParse(badgeTestRawProgram(mika, 2, 1, device, "tester", "Ryoku v2 Hyperion")); e != nil {
		t.Fatal("raw helper", e)
	}
	req, _ := requestParse(requestCode(badgeDevicePublic(device)))
	for _, program := range []string{" Ryoku", "Ryoku ", "Ryoku  v2", "Ryoku_v2", "-Ryoku", "Ryoku.", "Ryokú", "Ryoku\tv2", strings.Repeat("a", 41)} {
		if _, e := badgeSign(2, 1, mika, req, "tester", program, 318); e == nil {
			t.Fatal("signed bad program", program)
		}
		if _, e := badgeParse(badgeTestRawProgram(mika, 2, 1, device, "tester", program)); e == nil {
			t.Fatal("parsed bad program", program)
		}
	}
	if _, e := badgeSign(2, 4, mika, req, "tester", "Ryoku v2 Hyperion", 318); e == nil {
		t.Fatal("Mika signed a core badge with a program")
	}
}

func badgeTestResign(seed, payload []byte) []byte {
	domain := "ryoku-badge-v3"
	if payload[0] == 2 {
		domain = "ryoku-badge-v2"
	}
	return append(append([]byte{}, payload...), signDomain(seed, domain, payload)...)
}

func TestBadgeV2BytesAndProgramUnchanged(t *testing.T) {
	owner, mika, device := badgeTestKeys(t)
	payload := append([]byte{2, 1, 4}, badgeDevicePublic(device)...)
	payload = append(binary.BigEndian.AppendUint16(payload, 318), 8)
	payload = append(payload, "neur0map"...)
	want := codeText("RYB", append(payload, signDomain(owner, "ryoku-badge-v2", payload)...))
	if got := badgeTestCode(t, 1, 4, owner, device, "neur0map"); got != want {
		t.Fatal("v2 bytes changed", got, want)
	}
	if b, _ := badgeParse(want); b.Program != "" {
		t.Fatal("core v2 badge gained a program", b.Program)
	}
	if out := badgeInspect(want); out["program"] != nil {
		t.Fatal("inspect named a program", out)
	}
	tester := badgeTestCode(t, 2, 1, mika, device, "tester")
	b, e := badgeParse(tester)
	if e != nil || b.Program != "Ryoku v2 Hyperion" || badgeInspect(tester)["program"] != "Ryoku v2 Hyperion" {
		t.Fatal("v2 tester program", b, e)
	}
	contributor, _ := badgeParse(badgeTestCode(t, 2, 2, mika, device, "Ryoku-dev"))
	if contributor.Program != "" {
		t.Fatal("v2 contributor gained a program")
	}
	// a v2 tester relabelled v3 has no program byte, so it never passes as one
	raw, _ := parseCodeText("RYB", tester)
	if _, e := badgeParse(codeText("RYB", badgeTestResign(mika, append([]byte{3}, raw[1:len(raw)-64]...)))); e == nil {
		t.Fatal("v2 relabelled v3 accepted")
	}
}

func TestBadgeV3WorksWhereV2Does(t *testing.T) {
	_, mika, _ := badgeTestKeys(t)
	setCardMachine(t)
	seed, e := badgeDeviceSeed()
	if e != nil {
		t.Fatal(e)
	}
	code := badgeTestProgramCode(t, 2, 1, mika, seed, "tester", "Ryoku v2 Hyperion")
	if e = badgeActivate(code); e != nil {
		t.Fatal(e)
	}
	b, e := badgeActive()
	if e != nil || b.Program != "Ryoku v2 Hyperion" || b.Code != code {
		t.Fatal("activation", b, e)
	}
	view, _ := json.Marshal(badgeViewFor(b, false))
	if !strings.Contains(string(view), `"program":"Ryoku v2 Hyperion"`) {
		t.Fatal("view", string(view))
	}
	raw, _ := json.Marshal(teamWireList{V: 1, Badges: []string{code}})
	list, e := teamVerifyList(raw)
	if e != nil || len(list.Badges) != 1 || list.Badges[0].Program != "Ryoku v2 Hyperion" {
		t.Fatal("team list", list, e)
	}
	if found, ok := teamBadgeForCardID(list, b.CardID); !ok || found.ID != b.ID {
		t.Fatal("card id lookup", found, ok)
	}
	if holder := teamHolder(list, "tester"); !reflect.DeepEqual(holder["roles"], []string{"tester"}) || !reflect.DeepEqual(holder["cardIds"], []string{b.CardID}) {
		t.Fatal("holder", holder)
	}
	d := codedDesign{cardID: cardID(b.device), name: "tester", kind: 1, gallerySrc: "hawk"}
	record, e := recordSign(d, nil, seed, 1000, recordLook{})
	if e != nil {
		t.Fatal(e)
	}
	if _, e = recordVerify(record, b); e != nil {
		t.Fatal("record through a v3 badge", e)
	}
	rev, _ := revokeSign(2, mika, b.ID, 320)
	raw, _ = json.Marshal(teamWireList{V: 1, Badges: []string{code}, Revoked: []string{rev}})
	if list, e = teamVerifyList(raw); e != nil || len(list.Badges) != 0 {
		t.Fatal("revocation", list, e)
	}
}
