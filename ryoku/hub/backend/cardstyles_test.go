package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestStyleOwnersMatchLoginsWithoutCase(t *testing.T) {
	for _, login := range []string{"neur0map", "Neur0map", "NEUR0MAP"} {
		if !styleOwnedBy("stand", login) || !reflect.DeepEqual(stylesOwnedBy(login), []string{"stand"}) {
			t.Fatal(login, "does not own the Stand style")
		}
	}
	for _, c := range [][2]string{{"stand", "friend"}, {"stand", "neur0map2"}, {"stand", ""}, {"gold", "neur0map"}, {"", "neur0map"}} {
		if styleOwnedBy(c[0], c[1]) {
			t.Fatal(c, "owns the style")
		}
	}
	if got := stylesOwnedBy("friend"); got == nil || len(got) != 0 {
		t.Fatal("a login without a style owns", got)
	}
}

// The gate keeps its own copy of the table; the shared vectors carry this one for its tests to
// compare, so a change here without regenerating them (and changing the gate) fails.
func TestStyleOwnersMatchTheGateVectors(t *testing.T) {
	raw, e := os.ReadFile(filepath.Join("..", "..", "..", "release", "team-badges", "test", "vectors.json"))
	if e != nil {
		t.Fatal(e)
	}
	var vectors []struct {
		Kind   string              `json:"kind"`
		Owners map[string][]string `json:"owners"`
	}
	if e = json.Unmarshal(raw, &vectors); e != nil {
		t.Fatal(e)
	}
	found := 0
	for _, v := range vectors {
		if v.Kind == "styleOwners" {
			found++
			if !reflect.DeepEqual(v.Owners, cardStyleOwners) {
				t.Fatalf("the vectors' style owners %v differ from ryoku-hub's %v: run go test -run TestWriteVectors . -args -write-vectors", v.Owners, cardStyleOwners)
			}
		}
	}
	if found != 1 {
		t.Fatal("the vectors carry", found, "style owner tables")
	}
}

// A badge holder whose login owns no style publishes classic, with their own words, even with
// the Stand style left in their profile; the owner's style goes out, and badge status lists only
// what the login owns.
func TestRecordSyncSendsOnlyAnOwnedStyle(t *testing.T) {
	owner, _, _ := badgeTestKeys(t)
	setCardMachine(t)
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	g := newFakeGate(t)
	if e := atomicWrite(profileConfigPath(), []byte(`{"style":"stand","text":{"name":"Kakyoin","tagline":"Rero rero.","standName":"Hierophant Green"}}`), 0o644); e != nil {
		t.Fatal(e)
	}
	if e := recordSetOnline(true); e != nil {
		t.Fatal(e)
	}
	seed, _ := badgeDeviceSeed()
	sync := func(login string, threads string) cardRecord {
		t.Helper()
		if e := badgeActivate(badgeTestCode(t, 1, 2, owner, seed, login)); e != nil {
			t.Fatal(e)
		}
		if ok, e := recordSync([]string{"--threads", threads}); !ok || e != nil {
			t.Fatal(ok, e)
		}
		for _, r := range g.records {
			return r
		}
		t.Fatal("nothing published")
		return cardRecord{}
	}
	if r := sync("kakyoin", "16"); r.V != 4 || r.Style != "" || r.StandName != "" || r.Tagline != "Rero rero." {
		t.Fatal("a holder who owns no style sent one", r)
	}
	if status, e := badgeStatus(false); e != nil || status.Styles == nil || len(status.Styles) != 0 {
		t.Fatal("badge status offered a style the login doesn't own", status.Styles, e)
	}
	if r := sync("Neur0map", "17"); r.V != 4 || r.Style != "stand" || r.StandName != "Hierophant Green" {
		t.Fatal("the owner's style was not sent", r)
	}
	if status, _ := badgeStatus(false); !reflect.DeepEqual(status.Styles, []string{"stand"}) {
		t.Fatal("badge status left out the owner's style", status.Styles)
	}
}
