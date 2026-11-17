package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

// designWearGate is a stand-in team service: its list, the approved packs it serves, and every
// request body a machine sent it
type designWearGate struct {
	mu     sync.Mutex
	wire   teamWireList
	packs  map[string][]byte
	paths  []string
	bodies [][]byte
}

func (g *designWearGate) set(f func(*teamWireList)) {
	g.mu.Lock()
	defer g.mu.Unlock()
	f(&g.wire)
}

// designWearMachine is an isolated machine holding a badge of role for login "member", with its
// gate; the badge is on the gate's list
func designWearMachine(t *testing.T, role byte) (*designWearGate, badgeInfo, []byte) {
	t.Helper()
	owner, _, _ := badgeTestKeys(t)
	setCardMachine(t)
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	seed, e := badgeDeviceSeed()
	if e != nil {
		t.Fatal(e)
	}
	code := badgeTestCode(t, 1, role, owner, seed, "member")
	b, e := badgeParse(code)
	if e != nil {
		t.Fatal(e)
	}
	gate := &designWearGate{wire: teamWireList{V: 1, Badges: []string{code}, Revoked: []string{}, Designs: []string{}}, packs: map[string][]byte{}}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		gate.mu.Lock()
		defer gate.mu.Unlock()
		gate.paths = append(gate.paths, r.Method+" "+r.URL.Path)
		gate.bodies = append(gate.bodies, body)
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/v1/list":
			json.NewEncoder(w).Encode(gate.wire)
		case r.Method == http.MethodGet && strings.HasPrefix(r.URL.Path, "/v1/packs/"):
			raw, ok := gate.packs[strings.TrimSuffix(strings.TrimPrefix(r.URL.Path, "/v1/packs/"), ".zip")]
			if !ok {
				http.NotFound(w, r)
				return
			}
			w.Write(raw)
		case r.Method == http.MethodPut && strings.HasPrefix(r.URL.Path, "/v1/cards/"):
			w.Write([]byte(`{}`))
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(srv.Close)
	t.Setenv("RYOKU_TEAM_URL", srv.URL)
	if e = badgeActivate(code); e != nil {
		t.Fatal(e)
	}
	return gate, b, owner
}

// designWearFolder writes the sample design, named name, as a design folder
func designWearFolder(t *testing.T, name string) string {
	t.Helper()
	p := designTestPack(t)
	spec := designTestSpec()
	spec.Name = name
	raw, e := json.Marshal(spec)
	if e != nil {
		t.Fatal(e)
	}
	p.files["design.json"] = raw
	dir := t.TempDir()
	for _, file := range p.names {
		if e = os.WriteFile(filepath.Join(dir, file), p.files[file], 0o600); e != nil {
			t.Fatal(e)
		}
	}
	return dir
}

// designWearApprove serves a pack of the sample design named name and approves it for "member"
func designWearApprove(t *testing.T, gate *designWearGate, owner []byte, name string) {
	t.Helper()
	p, e := designReadPack(designWearFolder(t, name))
	if e != nil {
		t.Fatal(e)
	}
	raw, e := designZip(p)
	if e != nil {
		t.Fatal(e)
	}
	sum := sha256.Sum256(raw)
	approval, e := rydSign(1, owner, sum, "member", 1794787200)
	if e != nil {
		t.Fatal(e)
	}
	gate.set(func(w *teamWireList) { w.Designs = append(w.Designs, approval) })
	gate.mu.Lock()
	gate.packs[hex.EncodeToString(sum[:])] = raw
	gate.mu.Unlock()
}

func designWearStatus(t *testing.T) badgeStatusResult {
	t.Helper()
	status, e := badgeStatus(true)
	if e != nil {
		t.Fatal(e)
	}
	return status
}

// Every badge role may have an approved design: tester and contributor as well as the team
func TestBadgeDesignForEveryRole(t *testing.T) {
	for role := byte(1); role <= 5; role++ {
		t.Run(badgeRoles[role], func(t *testing.T) {
			gate, b, owner := designWearMachine(t, role)
			designWearApprove(t, gate, owner, "Approved")
			list, e := teamList(true)
			if e != nil || len(list.Designs) != 1 {
				t.Fatal("approval dropped for role", role, list.Designs, e)
			}
			design := badgeDesignFor(list, b)
			if design == nil || !design.Approved || design.Worn || design.Name != "Approved" {
				t.Fatal("no approved design for role", role, design)
			}
			status := designWearStatus(t)
			if status.Badge == nil || status.Badge.RoleID != role || status.Design == nil || status.Design.Name != "Approved" {
				t.Fatal(status)
			}
			// a revoked badge takes its login's design with it
			rev, _ := revokeSign(1, owner, b.ID, 320)
			gate.set(func(w *teamWireList) { w.Revoked = append(w.Revoked, rev) })
			if list, _ = teamList(true); len(list.Designs) != 0 {
				t.Fatal("design kept after revocation", list.Designs)
			}
			if status = designWearStatus(t); status.Badge != nil || status.Design != nil {
				t.Fatal("revoked badge kept its design", status)
			}
		})
	}
}

// A badge holder wears a checked design privately: it needs this machine's own active badge and a
// pack that passes every check, is checked again on each read, and stops with the badge
func TestDesignWearNeedsOwnBadgeAndValidPack(t *testing.T) {
	owner, _, _ := badgeTestKeys(t)
	setCardMachine(t)
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	folder := designWearFolder(t, "Worn")
	if e := designWear(folder); !errors.Is(e, errWearNeedsBadge) {
		t.Fatal("worn without a badge", e)
	}
	// a badge made for another machine does not activate here, so it cannot be worn under either
	other := badgeTestCode(t, 1, 1, owner, bytes.Repeat([]byte{7}, 32), "member")
	if e := badgeActivate(other); e == nil {
		t.Fatal("activated another machine's badge")
	}
	if e := designWear(folder); !errors.Is(e, errWearNeedsBadge) {
		t.Fatal("worn with another machine's badge", e)
	}
	if _, e := os.Lstat(designWornDir()); !os.IsNotExist(e) {
		t.Fatal("kept a design without a badge", e)
	}

	gate, b, owner := designWearMachine(t, 1)
	bad := designWearFolder(t, "Bad")
	os.WriteFile(filepath.Join(bad, "design.json"), []byte(`{"version":2}`), 0o600)
	scripted := designWearFolder(t, "Scripted")
	os.WriteFile(filepath.Join(scripted, "card.qml"), []byte("Item {}"), 0o600)
	linked := designWearFolder(t, "Linked")
	os.Remove(filepath.Join(linked, "art.webp"))
	os.Symlink(filepath.Join(folder, "art.webp"), filepath.Join(linked, "art.webp"))
	for _, refused := range []string{bad, scripted, linked} {
		if e := designWear(refused); e == nil {
			t.Fatal("wore a pack that fails the checks", refused)
		}
		if _, e := os.Lstat(designWornDir()); !os.IsNotExist(e) {
			t.Fatal("kept a pack that fails the checks", refused)
		}
	}

	if e := designWear(folder); e != nil {
		t.Fatal(e)
	}
	status := designWearStatus(t)
	if status.Design != nil || status.Worn == nil || !status.Worn.Worn || status.Worn.Approved || status.Worn.Name != "Worn" ||
		status.Worn.Dir != designWornDir() || status.Worn.Spec.Badge.Emblem != filepath.Join(designWornDir(), "emblem.webp") {
		t.Fatal(status.Worn, status.Design)
	}
	// a zip wears the same and replaces the folder worn before
	p, _ := designReadPack(designWearFolder(t, "Zipped"))
	raw, _ := designZip(p)
	zipPath := filepath.Join(t.TempDir(), "design.zip")
	os.WriteFile(zipPath, raw, 0o600)
	if e := designWear(zipPath); e != nil {
		t.Fatal(e)
	}
	if status = designWearStatus(t); status.Worn == nil || status.Worn.Name != "Zipped" {
		t.Fatal(status.Worn)
	}
	leftovers, _ := filepath.Glob(filepath.Join(profileHeroDir(), ".design-*"))
	if len(leftovers) != 0 {
		t.Fatal("left temporary folders", leftovers)
	}

	// the copy is checked again on every read: anything added to it stops it being drawn
	os.WriteFile(filepath.Join(designWornDir(), "card.qml"), []byte("Item {}"), 0o600)
	if status = designWearStatus(t); status.Worn != nil || status.WornError == "" {
		t.Fatal("a tampered design was drawn", status.Worn)
	}
	os.Remove(filepath.Join(designWornDir(), "card.qml"))
	// and a link in its place is refused, then removed without touching what it points at
	os.RemoveAll(designWornDir())
	os.Symlink(folder, designWornDir())
	if status = designWearStatus(t); status.Worn != nil || status.WornError == "" {
		t.Fatal("followed a linked design", status.Worn)
	}
	if e := designUnwear(); e != nil {
		t.Fatal(e)
	}
	if _, e := os.Lstat(designWornDir()); !os.IsNotExist(e) {
		t.Fatal("unwear kept the link", e)
	}
	if _, e := os.Stat(filepath.Join(folder, "design.json")); e != nil {
		t.Fatal("unwear followed the link", e)
	}

	// Stop wearing removes it
	if e := designWear(folder); e != nil {
		t.Fatal(e)
	}
	if e := designUnwear(); e != nil {
		t.Fatal(e)
	}
	if status = designWearStatus(t); status.Worn != nil {
		t.Fatal("still worn after unwear", status.Worn)
	}

	// losing the badge stops wearing it
	if e := designWear(folder); e != nil {
		t.Fatal(e)
	}
	rev, _ := revokeSign(1, owner, b.ID, 320)
	gate.set(func(w *teamWireList) { w.Revoked = append(w.Revoked, rev) })
	if status = designWearStatus(t); status.Badge != nil || status.Worn != nil {
		t.Fatal("a revoked badge still wears its design", status)
	}
	if _, e := teamList(true); e != nil {
		t.Fatal(e)
	}
	if e := designWear(folder); !errors.Is(e, errWearNeedsBadge) {
		t.Fatal("wore a design with a revoked badge", e)
	}
}

// The worn design never leaves the machine: the published record and everything sent to the team
// service carry none of it
func TestDesignWearNeverPublished(t *testing.T) {
	gate, _, _ := designWearMachine(t, 2)
	folder := designWearFolder(t, "Private")
	if e := designWear(folder); e != nil {
		t.Fatal(e)
	}
	if e := recordSetOnline(true); e != nil {
		t.Fatal(e)
	}
	if ok, e := recordSync([]string{"--threads", "16"}); e != nil || !ok {
		t.Fatal(ok, e)
	}
	emblem, _ := os.ReadFile(filepath.Join(folder, "emblem.webp"))
	design, _ := os.ReadFile(filepath.Join(folder, "design.json"))
	gate.mu.Lock()
	defer gate.mu.Unlock()
	put := false
	for i, path := range gate.paths {
		if strings.Contains(path, "/v1/pending") || strings.Contains(path, "/v1/designs") {
			t.Fatal("the worn design was sent", path)
		}
		body := gate.bodies[i]
		if strings.HasPrefix(path, "PUT /v1/cards/") {
			put = true
		}
		if bytes.Contains(body, []byte("Private")) || bytes.Contains(body, []byte(designWornDir())) ||
			bytes.Contains(body, design) || bytes.Contains(body, emblem) {
			t.Fatal("the worn design reached the team service in", path)
		}
	}
	if !put {
		t.Fatal("no record was published", gate.paths)
	}
}

// An approved design is drawn before the private one and drops it; the owner's Stand style is the
// card's own and is left to the card (TradingCard draws Stand over any design)
func TestApprovedDesignOutranksWornDesign(t *testing.T) {
	gate, _, owner := designWearMachine(t, 1)
	if e := designWear(designWearFolder(t, "Worn")); e != nil {
		t.Fatal(e)
	}
	if status := designWearStatus(t); status.Worn == nil || status.Design != nil {
		t.Fatal(status)
	}
	designWearApprove(t, gate, owner, "Approved")
	status := designWearStatus(t)
	if status.Design == nil || status.Design.Name != "Approved" || status.Worn != nil {
		t.Fatal("approved design did not take over", status.Design, status.Worn)
	}
	if _, e := os.Lstat(designWornDir()); !os.IsNotExist(e) {
		t.Fatal("the private copy stayed after approval", e)
	}
	if e := designWear(designWearFolder(t, "Again")); e == nil {
		t.Fatal("wore a design over an approved one")
	}
}
