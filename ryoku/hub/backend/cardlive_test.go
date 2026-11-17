package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"image"
	"maps"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"
	"time"
)

// fakeGate answers like team.ryoku.dev: one batch for many cards, record pictures, and device
// signed publishing and withdrawal
type fakeGate struct {
	t        *testing.T
	now      int64
	records  map[string]cardRecord
	pictures map[string][]byte
	// badges and designs go out with every batch: picking the ones for its cards is the binder's job
	badges  []string
	designs []string
	// extra records go out whatever the binder already holds, as a replaying or hostile gate would
	extra []any
	// block answers the batch in the gate's place when set
	block   func(w http.ResponseWriter)
	batches []map[string]int64
	// holders has, per batch that carried a holder proof, whether the proof verified
	holders []bool
	// holderUsed counts the early refreshes spent today, ten at most, and holderAt the newest proof
	holderUsed, holderAt int64
	puts                 int
	deletes              int
	pictureGets          int
}

func newFakeGate(t *testing.T) *fakeGate {
	g := &fakeGate{t: t, now: time.Now().Unix(), records: map[string]cardRecord{}, pictures: map[string][]byte{}}
	srv := httptest.NewServer(http.HandlerFunc(g.serve))
	t.Cleanup(srv.Close)
	t.Setenv("RYOKU_TEAM_URL", srv.URL)
	return g
}

func (g *fakeGate) serve(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Date", time.Unix(g.now, 0).UTC().Format(http.TimeFormat))
	id, rest, _ := strings.Cut(strings.TrimPrefix(r.URL.Path, "/v1/cards/"), "/")
	switch {
	case r.Method == http.MethodPost && r.URL.Path == "/v1/cards/batch":
		var ask struct {
			Cards  map[string]int64 `json:"cards"`
			Holder *pullHolder      `json:"holder"`
		}
		if e := json.NewDecoder(r.Body).Decode(&ask); e != nil || r.Header.Get("Content-Type") != "application/json" || len(ask.Cards) == 0 || len(ask.Cards) > 200 {
			g.t.Error("malformed batch", e)
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		g.batches = append(g.batches, ask.Cards)
		next := g.now + 900
		reply := map[string]any{}
		if ask.Holder != nil {
			b, e := badgeParse(ask.Holder.Badge)
			payload, _ := pullHolderPayload(b.ID, ask.Holder.At, ask.Cards)
			sig, _ := base64.StdEncoding.DecodeString(ask.Holder.Sig)
			valid := e == nil && verifyDomain(b.device, "ryoku-pull-v2", payload, sig) && ask.Holder.At <= g.now+600 && ask.Holder.At >= g.now-600
			g.holders = append(g.holders, valid)
			if valid {
				if ask.Holder.At > g.holderAt && g.holderUsed < 10 {
					g.holderUsed, g.holderAt, next = g.holderUsed+1, ask.Holder.At, g.now
				}
				reply["holderLeft"] = 10 - g.holderUsed
			}
		}
		if g.block != nil {
			g.block(w)
			return
		}
		records, gone := []any{}, []string{}
		for id, known := range ask.Cards {
			record, ok := g.records[id]
			if !ok {
				gone = append(gone, id)
			} else if record.Updated > known {
				records = append(records, record)
			}
		}
		maps.Copy(reply, map[string]any{"v": 1, "now": g.now, "next": next, "records": append(records, g.extra...),
			"gone": gone, "badges": append([]string{}, g.badges...), "designs": append([]string{}, g.designs...)})
		json.NewEncoder(w).Encode(reply)
	case r.Method == http.MethodGet && strings.HasSuffix(rest, ".jpg"):
		g.pictureGets++
		picture, ok := g.pictures[strings.TrimSuffix(rest, ".jpg")]
		if !ok || g.records[id].Picture+".jpg" != rest {
			http.NotFound(w, r)
			return
		}
		w.Write(picture)
	case r.Method == http.MethodPut && rest == "":
		var put struct {
			Record cardRecord `json:"record"`
		}
		json.NewDecoder(r.Body).Decode(&put)
		if _, e := recordVerifyDevice(put.Record); e != nil || put.Record.ID != id {
			g.t.Error("gate got an unverifiable record", e)
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		g.puts++
		g.records[id] = put.Record
		json.NewEncoder(w).Encode(map[string]string{"id": id})
	case r.Method == http.MethodDelete && rest == "":
		var withdrawal struct {
			ID  string `json:"id"`
			At  int64  `json:"at"`
			Sig string `json:"sig"`
		}
		json.NewDecoder(r.Body).Decode(&withdrawal)
		record, ok := g.records[id]
		if !ok {
			w.WriteHeader(http.StatusNotFound)
			json.NewEncoder(w).Encode(map[string]string{"error": "Card record not found"})
			return
		}
		device, _ := base64.StdEncoding.DecodeString(record.Device)
		payload, e := recordDeletePayload(withdrawal.ID, withdrawal.At)
		sig, _ := base64.StdEncoding.DecodeString(withdrawal.Sig)
		if e != nil || withdrawal.ID != id || r.Header.Get("Content-Type") != "application/json" ||
			!verifyDomain(device, "ryoku-card-delete-v1", payload, sig) || withdrawal.At > g.now+600 || withdrawal.At < g.now-600 {
			g.t.Error("gate got an invalid withdrawal", withdrawal)
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]string{"error": "Invalid card signature"})
			return
		}
		g.deletes++
		delete(g.records, id)
		json.NewEncoder(w).Encode(map[string]string{"id": id})
	default:
		http.NotFound(w, r)
	}
}

// makeDue brings the binder's next check forward to now
func makeDue(t *testing.T) {
	t.Helper()
	s, e := pullLoad()
	if e != nil {
		t.Fatal(e)
	}
	s.Next = 0
	if e = s.save(); e != nil {
		t.Fatal(e)
	}
}

func binderByID(t *testing.T, view binderView) map[string]binderCard {
	t.Helper()
	out := map[string]binderCard{}
	for _, c := range view.Cards {
		out[c.ID] = c
	}
	return out
}

func TestBadgelessCardGoesOnlineAndWithdraws(t *testing.T) {
	id := setCardMachine(t)
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	g := newFakeGate(t)
	sync := func() bool {
		t.Helper()
		ok, e := recordSync([]string{"--threads", "16"})
		if e != nil {
			t.Fatal(e)
		}
		return ok
	}
	toggle := func(choice string) badgeStatusResult {
		t.Helper()
		var status badgeStatusResult
		out := captureStdout(t, func() {
			if e := badgeCommand([]string{"online", choice}); e != nil {
				t.Fatal(e)
			}
		})
		if e := json.Unmarshal([]byte(out), &status); e != nil {
			t.Fatal(out, e)
		}
		return status
	}
	if status, e := badgeStatus(false); e != nil || status.Online || status.Badge != nil {
		t.Fatal("a machine without a badge started online", status, e)
	}
	if sync() || g.puts != 0 {
		t.Fatal("published before going online")
	}
	status := toggle("on")
	if !status.Online || status.Badge != nil {
		t.Fatal("online on", status)
	}
	if !sync() || g.puts != 1 {
		t.Fatal("an online machine without a badge did not publish", g.puts)
	}
	record := g.records[hex.EncodeToString(id[:])]
	if d, e := recordVerifyDevice(record); e != nil || d.threads != 16 || record.Device != status.Device {
		t.Fatal("published record does not prove itself by its device", record, e)
	}
	if status = toggle("off"); status.Online || g.deletes != 1 || len(g.records) != 0 {
		t.Fatal("turning off left the card online", status, g.deletes)
	}
	out := captureStdout(t, func() {
		if e := badgeCommand([]string{"sync", "--threads", "16"}); e != nil {
			t.Fatal(e)
		}
	})
	if strings.TrimSpace(out) != `{"published":false}` || g.puts != 1 {
		t.Fatal("published while off", out, g.puts)
	}
	// nothing online to take down is no failure
	if status = toggle("off"); status.Online {
		t.Fatal(status)
	}
	toggle("on")
	// the same card publishes again, stamped after the withdrawn record the gate still refuses
	if !sync() || g.puts != 2 || g.records[record.ID].Updated <= record.Updated {
		t.Fatal("going online again did not publish the card again", g.puts, g.records[record.ID].Updated)
	}
}

func TestBinderPullsEveryCardInOneBatch(t *testing.T) {
	owner, _, _ := badgeTestKeys(t)
	setCardMachine(t)
	g := newFakeGate(t)
	friend := func(n byte) ([]byte, string) {
		seed := bytes.Repeat([]byte{n}, 32)
		id := cardID(badgeDevicePublic(seed))
		return seed, hex.EncodeToString(id[:])
	}
	liveSeed, liveID := friend(11)
	heldSeed, heldID := friend(12)
	_, victimID := friend(13)
	_, goneID := friend(14)
	design := func(id string, words string) codedDesign {
		raw, _ := hex.DecodeString(id)
		return codedDesign{cardID: [8]byte(raw), name: words}
	}
	picture := jpegOf(t, image.NewRGBA(image.Rect(0, 0, 56, 39)), 80)
	liveDesign := design(liveID, "live words")
	liveDesign.kind, liveDesign.threads = 2, 24
	live, _ := recordSign(liveDesign, picture, liveSeed, 1000, recordLook{})
	g.records[liveID], g.pictures[live.Picture] = live, picture
	heldDesign := design(heldID, "badge words")
	heldDesign.kind, heldDesign.gallerySrc = 1, "hawk"
	held, _ := recordSign(heldDesign, nil, heldSeed, 1000, recordLook{})
	g.records[heldID] = held
	g.badges = []string{badgeTestCode(t, 1, 4, owner, heldSeed, "friend")}
	sha := sha256.Sum256([]byte("pack"))
	teamDesign, _ := rydSign(1, owner, sha, "friend", 1794787200)
	strangerDesign, _ := rydSign(1, owner, sha, "stranger", 1794787200)
	g.designs = []string{teamDesign, strangerDesign}
	// someone else's key signs a record for the victim's card id and offers itself as the device
	attacker := bytes.Repeat([]byte{15}, 32)
	forged := cardRecord{V: 2, ID: victimID, Updated: 1000, Body: base64.StdEncoding.EncodeToString(design(victimID, "forged").bytes()),
		Device: base64.StdEncoding.EncodeToString(badgeDevicePublic(attacker))}
	payload, _ := recordPayload(forged)
	forged.Sig = base64.StdEncoding.EncodeToString(signDomain(attacker, "ryoku-card-v2", payload))
	g.records[victimID] = forged

	for _, id := range []string{liveID, heldID, victimID, goneID} {
		entry, e := addCard(sharedCard(t, design(id, "picture name"), "png"))
		if e != nil {
			t.Fatal(e)
		}
		if id == heldID && (!entry.Verified || entry.Name != "badge words" || entry.Record != "live") {
			t.Fatal("an added card did not show its owner's record", entry)
		}
	}
	if len(g.batches) != 4 || len(g.batches[3]) != 1 {
		t.Fatal("adding a card did not ask for that card alone", g.batches)
	}
	g.batches = nil
	view, e := listCards(false)
	if e != nil {
		t.Fatal(e)
	}
	want := map[string]int64{liveID: 1000, heldID: 1000, victimID: 0, goneID: 0}
	if len(g.batches) != 1 || !maps.Equal(g.batches[0], want) {
		t.Fatal("binder did not ask for every card at once with what it holds", g.batches)
	}
	if view.Checked != g.now || view.Next != g.now+900 || view.Paused != "" || g.pictureGets != 1 {
		t.Fatal("pull timing or picture downloads", view, g.pictureGets)
	}
	cards := binderByID(t, view)
	if c := cards[liveID]; c.Record != "live" || c.RecordUpdated != 1000 || c.Name != "live words" || c.HW.Threads != 24 ||
		c.Verified || c.Badge != nil || c.BadgeCheck != "none" || c.Hero.Kind != "custom" || c.Hero.Rev != live.Picture[:12] {
		t.Fatal("device record without a badge", c)
	}
	if c := cards[heldID]; c.Record != "live" || !c.Verified || c.BadgeCheck != "ok" || c.Badge == nil || c.Badge.Login != "friend" || c.Name != "badge words" {
		t.Fatal("badge holder's record", c)
	}
	if c := cards[victimID]; c.Record != "" || c.Name != "picture name" || c.Verified {
		t.Fatal("a record whose device does not own its id was applied", c)
	}
	if c := cards[goneID]; c.Record != "" || c.Name != "picture name" {
		t.Fatal("a card never published gained a record", c)
	}
	if s, _ := pullLoad(); len(s.Designs) != 1 || s.Designs[0] != teamDesign || len(s.Badges) != 1 {
		t.Fatal("the gate's designs were taken without a team badge behind them", s)
	}

	view, _ = listCards(false)
	if len(g.batches) != 1 || view.Checked != g.now {
		t.Fatal("asked the gate before its next check time", g.batches)
	}

	// the live card changes, a replayed older copy of the badge holder's card comes along, and
	// the badge is no longer vouched for
	liveDesign.name = "newer words"
	newer, _ := recordSign(liveDesign, picture, liveSeed, 1001, recordLook{})
	g.records[liveID] = newer
	older, _ := recordSign(design(heldID, "replayed words"), nil, heldSeed, 999, recordLook{})
	g.extra = []any{older}
	g.badges = nil
	makeDue(t)
	cards = binderByID(t, listed(t))
	if c := cards[liveID]; c.Name != "newer words" || c.RecordUpdated != 1001 || g.pictureGets != 1 {
		t.Fatal("the newer record was not applied, or its unchanged picture was downloaded again", c, g.pictureGets)
	}
	if c := cards[heldID]; c.Name != "badge words" || c.RecordUpdated != 1000 || c.Verified || c.BadgeCheck != "none" || c.Record != "live" {
		t.Fatal("an older record replaced a newer one, or a badge outlived the gate's word", c)
	}

	// the gate lets the live card go after three idle months: the binder keeps its last copy
	delete(g.records, liveID)
	g.extra = nil
	makeDue(t)
	cards = binderByID(t, listed(t))
	if c := cards[liveID]; c.Record != "kept" || c.Name != "newer words" || c.RecordUpdated != 1001 || c.Hero.Rev != newer.Picture[:12] {
		t.Fatal("a card the gate let go lost the binder's copy", c)
	}
	if len(g.holders) != 0 {
		t.Fatal("a machine without a badge sent a holder proof", g.holders)
	}
}

// A badge holder's Refresh asks the gate at once with a proof bound to its batch, ignoring the
// next check time and any pause, until the gate says the day's early refreshes are used; then
// the binder keeps the schedule like anyone's until the gate's UTC day turns over. Opening the
// binder never spends one.
func TestBadgeHolderRefreshSpendsTheGatesEarlyRefreshes(t *testing.T) {
	owner, _, _ := badgeTestKeys(t)
	setCardMachine(t)
	g := newFakeGate(t)
	g.now = time.Date(2026, 11, 17, 9, 0, 0, 0, time.UTC).Unix()
	clock := pullClock
	pullClock = func() time.Time { return time.Unix(g.now, 0) }
	t.Cleanup(func() { pullClock = clock })
	seed, e := badgeDeviceSeed()
	if e != nil {
		t.Fatal(e)
	}
	if e = badgeActivate(badgeTestCode(t, 1, 4, owner, seed, "neur0map")); e != nil {
		t.Fatal(e)
	}
	friend := bytes.Repeat([]byte{31}, 32)
	id := cardID(badgeDevicePublic(friend))
	record, _ := recordSign(codedDesign{cardID: id, name: "record words"}, nil, friend, 1000, recordLook{})
	g.records[record.ID] = record
	if _, e = addCard(sharedCard(t, codedDesign{cardID: id, name: "picture name"}, "png")); e != nil {
		t.Fatal(e)
	}
	if view := listed(t); !view.Holder || view.HolderLeft != -1 || view.Next != g.now+900 || len(g.holders) != 0 {
		t.Fatal("adding a card or opening the binder spent an early refresh", view, g.holders)
	}
	if listed(t); len(g.batches) != 2 {
		t.Fatal("opening the binder again asked before the next check", len(g.batches))
	}
	midnight := time.Date(2026, 11, 18, 0, 0, 0, 0, time.UTC).Unix()
	for i := range 10 {
		g.now++
		if i == 3 {
			// a pause the gate asked for earlier holds no early refresh back
			s, _ := pullLoad()
			s.Paused, s.Next = "busy", g.now+10000
			s.save()
		}
		view := refreshed(t)
		if view.Paused != "" || view.Next != g.now || view.Checked != g.now || view.HolderLeft != 9-i || view.HolderReset != midnight || view.Cards[0].Name != "record words" {
			t.Fatal("early refresh", i+1, view)
		}
		// the early refresh left the binder due, so the next look checks without spending one
		listed(t)
	}
	if len(g.holders) != 10 || slices.Contains(g.holders, false) || g.holderUsed != 10 {
		t.Fatal("holder proofs", g.holders, g.holderUsed)
	}
	batches := len(g.batches)
	g.now++
	if view := refreshed(t); view.HolderLeft != 0 || len(g.batches) != batches || view.Next <= g.now {
		t.Fatal("a used-up holder's refresh asked before the schedule", view, len(g.batches)-batches)
	}
	// the gate's day turns over: the due check spends nothing and says nothing of them, then
	// Refresh asks early again and hears the ten are back
	g.now, g.holderUsed = midnight+30, 0
	if view := refreshed(t); view.HolderLeft != -1 || len(g.batches) != batches+1 || len(g.holders) != 10 {
		t.Fatal("a due check after midnight", view)
	}
	g.now++
	if view := refreshed(t); len(g.batches) != batches+2 || view.HolderLeft != 9 || view.Next != g.now {
		t.Fatal("the next UTC day brought no early refreshes back", view)
	}
	// a refusal from the gate itself is what pauses a holder, and the next Refresh asks again
	g.block = func(w http.ResponseWriter) {
		w.WriteHeader(http.StatusTooManyRequests)
		json.NewEncoder(w).Encode(map[string]any{"error": "Rate limit exceeded", "next": g.now + 60, "holderLeft": 8})
	}
	g.now++
	if view := refreshed(t); view.Paused != "busy" || view.HolderLeft != 8 {
		t.Fatal("busy gate", view)
	}
	g.now++
	if refreshed(t); len(g.batches) != batches+4 {
		t.Fatal("a holder waited out a pause instead of asking", len(g.batches)-batches)
	}
}

// Without a badge, Refresh keeps the gate's schedule and never sends a proof.
func TestRefreshKeepsTheScheduleWithoutABadge(t *testing.T) {
	setCardMachine(t)
	g := newFakeGate(t)
	friend := bytes.Repeat([]byte{32}, 32)
	id := cardID(badgeDevicePublic(friend))
	record, _ := recordSign(codedDesign{cardID: id, name: "record words"}, nil, friend, 1000, recordLook{})
	g.records[record.ID] = record
	if _, e := addCard(sharedCard(t, codedDesign{cardID: id, name: "picture name"}, "png")); e != nil {
		t.Fatal(e)
	}
	listed(t)
	for range 3 {
		if view := refreshed(t); view.Holder || view.HolderLeft != -1 || view.Next != g.now+900 {
			t.Fatal(view)
		}
	}
	if len(g.batches) != 2 || len(g.holders) != 0 {
		t.Fatal("a badge-less Refresh asked early", len(g.batches), g.holders)
	}
}

func refreshed(t *testing.T) binderView {
	t.Helper()
	view, e := listCards(true)
	if e != nil {
		t.Fatal(e)
	}
	return view
}

func TestBinderPullTimingComesFromTheGate(t *testing.T) {
	setCardMachine(t)
	g := newFakeGate(t)
	seed := bytes.Repeat([]byte{21}, 32)
	id := cardID(badgeDevicePublic(seed))
	record, _ := recordSign(codedDesign{cardID: id, name: "record words"}, nil, seed, 1000, recordLook{})
	g.records[record.ID] = record
	g.now = time.Date(2026, 11, 16, 20, 30, 0, 0, time.UTC).Unix()
	skew := -2 * time.Hour
	clock := pullClock
	pullClock = func() time.Time { return time.Unix(g.now, 0).Add(skew) }
	t.Cleanup(func() { pullClock = clock })
	if _, e := addCard(sharedCard(t, codedDesign{cardID: id, name: "picture name"}, "png")); e != nil {
		t.Fatal(e)
	}
	if view := listed(t); view.Next != g.now+900 || len(g.batches) != 2 {
		t.Fatal(view, g.batches)
	}

	// this computer's clock jumps three hours ahead and the day's requests have run out: the
	// pause lasts until the gate's midnight, not this computer's
	skew = 3 * time.Hour
	g.now = time.Date(2026, 11, 16, 22, 30, 0, 0, time.UTC).Unix()
	g.block = func(w http.ResponseWriter) {
		w.WriteHeader(http.StatusServiceUnavailable)
		w.Write([]byte("<html>error 1027</html>"))
	}
	view := listed(t)
	midnight := time.Date(2026, 11, 17, 0, 0, 0, 0, time.UTC).Unix()
	if view.Paused != "daily" || view.Next != midnight || len(view.Cards) != 1 || view.Cards[0].Name != "record words" {
		t.Fatal("daily pause", view)
	}
	g.now += 3600
	if view = listed(t); len(g.batches) != 3 || view.Paused != "daily" {
		t.Fatal("asked again during the daily pause", g.batches)
	}

	g.now = midnight + 5
	g.block = func(w http.ResponseWriter) {
		w.Header().Set("Retry-After", "60")
		w.WriteHeader(http.StatusTooManyRequests)
		json.NewEncoder(w).Encode(map[string]any{"error": "Rate limit exceeded", "next": g.now + 60})
	}
	if view = listed(t); view.Paused != "busy" || view.Next != midnight+65 {
		t.Fatal("busy gate", view)
	}
	g.now += 61
	g.block = func(w http.ResponseWriter) { w.WriteHeader(http.StatusTooManyRequests) }
	if view = listed(t); view.Paused != "daily" || view.Next != midnight+86400 {
		t.Fatal("a block without the gate's answer", view)
	}

	g.now = midnight + 86400 + 1
	t.Setenv("RYOKU_TEAM_URL", "http://127.0.0.1:1")
	if view = listed(t); view.Paused != "offline" || view.Next != midnight+86400 || view.Cards[0].Name != "record words" || view.Cards[0].Record != "live" {
		t.Fatal("offline binder", view)
	}
}

func listed(t *testing.T) binderView {
	t.Helper()
	view, e := listCards(false)
	if e != nil {
		t.Fatal(e)
	}
	return view
}
