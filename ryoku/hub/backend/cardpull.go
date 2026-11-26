package main

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"maps"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"time"
)

// the gate answers at most this many cards per request
const pullBatchLimit = 200

// pullClock is this computer's clock; the pull state corrects it by the gate's offset
var pullClock = time.Now

// pullState is the binder's view of the team service: when the gate last answered and when it
// wants to be asked again (both in the gate's time), the badges and designs it vouched for, and
// the cards it no longer holds. A badge holder's binder also keeps how many early refreshes the
// gate said were left on which of its UTC days, and the last proof time it signed, which the
// gate wants to rise with every proof.
type pullState struct {
	Checked    int64    `json:"checked"`
	Next       int64    `json:"next"`
	Paused     string   `json:"paused"`
	Offset     int64    `json:"offset"`
	Badges     []string `json:"badges"`
	Designs    []string `json:"designs"`
	Gone       []string `json:"gone"`
	HolderLeft int      `json:"holderLeft"`
	HolderDay  int64    `json:"holderDay"`
	HolderAt   int64    `json:"holderAt"`
}

type pullReply struct {
	V       int          `json:"v"`
	Now     int64        `json:"now"`
	Next    int64        `json:"next"`
	Records []cardRecord `json:"records"`
	Gone    []string     `json:"gone"`
	Badges  []string     `json:"badges"`
	Designs []string     `json:"designs"`
	// HolderLeft answers a batch that carried a holder proof: the early refreshes left today
	HolderLeft *int `json:"holderLeft"`
}

func pullStatePath() (string, error) {
	dir, e := teamCacheDir()
	if e != nil {
		return "", e
	}
	return filepath.Join(dir, "pull.json"), nil
}

func pullLoad() (pullState, error) {
	path, e := pullStatePath()
	if e != nil {
		return pullState{}, e
	}
	var s pullState
	raw, e := readBoundedFile(path, 2<<20)
	if e != nil && !os.IsNotExist(e) {
		return s, e
	}
	if e != nil || json.Unmarshal(raw, &s) != nil || !slices.Contains([]string{"", "busy", "daily", "offline"}, s.Paused) {
		s = pullState{}
	}
	return s, nil
}

func (s pullState) save() error {
	path, e := pullStatePath()
	if e != nil {
		return e
	}
	raw, e := json.Marshal(s)
	if e != nil {
		return e
	}
	return atomicWrite(path, raw, 0600)
}

// now is the gate's time, read off this computer's clock, so neither its timezone nor its
// drift moves the next check
func (s pullState) now() int64 {
	return pullClock().Unix() + s.Offset
}

func (s pullState) team() teamVerifiedList {
	return teamVerifyWire(teamWireList{V: 1, Badges: s.Badges, Designs: s.Designs})
}

// holderLeft is how many early refreshes the gate last said this badge has left today, or -1
// when it hasn't said since the gate's day turned over
func (s pullState) holderLeft() int {
	if s.HolderDay != s.now()/86400 {
		return -1
	}
	return s.HolderLeft
}

// heard keeps what the gate said of this badge's early refreshes, on the gate's day
func (s *pullState) heard(left *int, now int64) {
	if left != nil && *left >= 0 {
		s.HolderLeft, s.HolderDay = *left, now/86400
	}
}

// binderPull refreshes every card in the binder once the gate's next check time has come.
// early is a badge holder pressing Refresh: while the gate hasn't said their early refreshes for
// the day are used, it asks at once with a proof, whatever the schedule or a pause says, and the
// gate decides; a due check needs no proof and spends none.
func binderPull(ids []string, early bool) (pullState, error) {
	s, e := pullLoad()
	if e != nil || len(ids) == 0 {
		return s, e
	}
	due := s.now() >= s.Next
	early = early && !due && s.holderLeft() != 0
	if !due && !early {
		return s, nil
	}
	var last pullReply
	for start := 0; start < len(ids); start += pullBatchLimit {
		// one early refresh covers the binder: the batches after the first are ordinary checks
		reply, ok, e := s.pull(ids[start:min(len(ids), start+pullBatchLimit)], early && start == 0)
		if e != nil {
			return s, e
		}
		if !ok {
			return s, s.save()
		}
		last = reply
	}
	s.Checked, s.Next, s.Paused = last.Now, last.Next, ""
	// cards removed from the binder since leave the state with this full answer
	s.Badges = slices.DeleteFunc(s.Badges, func(code string) bool {
		b, e := badgeParse(code)
		return e != nil || !slices.Contains(ids, b.CardID)
	})
	s.Gone = slices.DeleteFunc(s.Gone, func(id string) bool { return !slices.Contains(ids, id) })
	return s, s.save()
}

// binderPullAdded asks for a card just added, whatever the cadence, so it shows its latest
// version straight away; the rest of the binder keeps its own schedule
func binderPullAdded(id string) (pullState, error) {
	s, e := pullLoad()
	if e != nil {
		return s, e
	}
	reply, ok, e := s.pull([]string{id}, false)
	if e != nil {
		return s, e
	}
	if ok {
		if s.Checked == 0 {
			// nothing else was ever checked: the rest of the binder is due at once
			s.Checked, s.Next = reply.Now, reply.Now
		}
		if s.Paused != "" {
			s.Paused, s.Next = "", min(s.Next, reply.Next)
		}
	}
	return s, s.save()
}

// pull asks the gate for the cards in ids that changed since this binder last saw them and folds
// its answer into s; false when the gate didn't answer, with the reason in s.Paused. holder signs
// the request with the badge's device to spend one of its early refreshes.
func (s *pullState) pull(ids []string, holder bool) (pullReply, bool, error) {
	list := s.team()
	known := map[string]int64{}
	for _, id := range ids {
		known[id] = 0
		if r, _, _, e := recordCached(id, list); e == nil {
			known[id] = r.Updated
		}
	}
	request := struct {
		Cards  map[string]int64 `json:"cards"`
		Holder *pullHolder      `json:"holder,omitempty"`
	}{Cards: known}
	if holder {
		// the gate takes each proof once, so its time only ever rises
		at := max(s.now(), s.HolderAt+1)
		if proof, e := pullHolderProof(at, known); e == nil {
			request.Holder, s.HolderAt = &proof, at
		}
	}
	body, e := json.Marshal(request)
	if e != nil {
		return pullReply{}, false, e
	}
	reply, e := teamExchange(http.MethodPost, "/v1/cards/batch", "application/json", body, nil, 2<<20)
	local := pullClock().Unix()
	if e != nil {
		s.Paused = "offline"
		return pullReply{}, false, nil
	}
	if !reply.date.IsZero() {
		s.Offset = reply.date.Unix() - local
	}
	switch reply.status {
	case http.StatusOK:
		var answer pullReply
		if json.Unmarshal(reply.body, &answer) != nil || answer.V != 1 || answer.Now <= 0 || answer.Next < answer.Now {
			break
		}
		s.Offset = answer.Now - local
		s.heard(answer.HolderLeft, answer.Now)
		s.fold(ids, answer, known)
		return answer, true, nil
	case http.StatusTooManyRequests, http.StatusServiceUnavailable:
		var busy struct {
			Next       int64 `json:"next"`
			HolderLeft *int  `json:"holderLeft"`
		}
		if reply.status == http.StatusTooManyRequests && json.Unmarshal(reply.body, &busy) == nil && busy.Next > 0 {
			s.heard(busy.HolderLeft, s.now())
			s.Paused, s.Next = "busy", busy.Next
			return pullReply{}, false, nil
		}
		// Cloudflare's own block once the day's requests run out: nothing answers until the
		// gate's day turns over
		s.Paused, s.Next = "daily", (s.now()/86400+1)*86400
		return pullReply{}, false, nil
	}
	s.Paused = "offline"
	return pullReply{}, false, nil
}

// fold takes the gate's word for the cards it was asked about and keeps what it said of others
func (s *pullState) fold(ids []string, answer pullReply, known map[string]int64) {
	badges := slices.DeleteFunc(slices.Clone(s.Badges), func(code string) bool {
		b, e := badgeParse(code)
		return e != nil || slices.Contains(ids, b.CardID)
	})
	for _, code := range answer.Badges {
		if b, e := badgeParse(code); e == nil && slices.Contains(ids, b.CardID) {
			badges = append(badges, code)
		}
	}
	list := teamVerifyWire(teamWireList{V: 1, Badges: badges, Designs: append(slices.Clone(answer.Designs), s.Designs...)})
	s.Badges, s.Designs = []string{}, []string{}
	for _, b := range list.Badges {
		s.Badges = append(s.Badges, b.Code)
	}
	for _, d := range list.Designs {
		s.Designs = append(s.Designs, d.Code)
	}
	s.Gone = slices.DeleteFunc(slices.Clone(s.Gone), func(id string) bool { return slices.Contains(ids, id) })
	for _, id := range answer.Gone {
		if slices.Contains(ids, id) && !slices.Contains(s.Gone, id) {
			s.Gone = append(s.Gone, id)
		}
	}
	for _, r := range answer.Records {
		if _, asked := known[r.ID]; asked && r.Updated > known[r.ID] {
			// A refused replacement never makes the retained approved copy
			// current. It remains a local historical snapshot and is retried.
			if err := pullStoreRecord(r, list); err != nil && r.V == 5 && !slices.Contains(s.Gone, r.ID) {
				s.Gone = append(s.Gone, r.ID)
			}
		}
	}
}

// pullStoreRecord keeps a verified record and its picture beside each other in the team cache,
// so recordCached can stand in with both, and an unchanged picture isn't downloaded again
func pullStoreRecord(r cardRecord, list teamVerifiedList) error {
	if _, e := recordVerifyFor(r, list); e != nil {
		return e
	}
	if r.V == 5 {
		return cardAnimationStore(r)
	}
	dir, e := teamCacheDir()
	if e != nil {
		return e
	}
	picturePath := filepath.Join(dir, "record-"+r.ID+".jpg")
	if r.Picture == "" {
		if e = os.Remove(picturePath); e != nil && !os.IsNotExist(e) {
			return e
		}
	} else if held, e := readBoundedFile(picturePath, recordPictureLimit); e != nil || pictureSHA(held) != r.Picture {
		picture, status, e := teamGet("/v1/cards/"+r.ID+"/"+r.Picture+".jpg", recordPictureLimit)
		if e != nil {
			return e
		}
		if status != http.StatusOK {
			return errors.New("card picture unavailable")
		}
		if pictureSHA(picture) != r.Picture {
			return errors.New("card picture digest mismatch")
		}
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		media, e := inspectMedia(ctx, picture, false)
		cancel()
		if e != nil || media.Extension != "jpg" || media.Width > 1120 || media.Height > 780 || media.Width < 1 || media.Height < 1 {
			return errors.New("invalid record JPEG")
		}
		if e = atomicWrite(picturePath, picture, 0600); e != nil {
			return e
		}
	}
	raw, e := json.Marshal(r)
	if e != nil {
		return e
	}
	return atomicWrite(filepath.Join(dir, "record-"+r.ID+".json"), raw, 0600)
}

// recordCached is the last record this binder verified for the card, with its picture, checked
// again against what the gate last said of the card's badge
func recordCached(id string, list teamVerifiedList) (cardRecord, codedDesign, []byte, error) {
	if !validCardID(id) {
		return cardRecord{}, codedDesign{}, nil, errors.New("invalid card id")
	}
	dir, e := teamCacheDir()
	if e != nil {
		return cardRecord{}, codedDesign{}, nil, e
	}
	raw, e := readBoundedFile(filepath.Join(dir, "record-"+id+".json"), 4096)
	if e != nil {
		return cardRecord{}, codedDesign{}, nil, e
	}
	var r cardRecord
	if e = json.Unmarshal(raw, &r); e != nil {
		return r, codedDesign{}, nil, e
	}
	if r.ID != id {
		return r, codedDesign{}, nil, errors.New("cached record belongs to another card")
	}
	d, e := recordVerifyFor(r, list)
	if e != nil || r.Picture == "" {
		return r, d, nil, e
	}
	if r.V == 5 {
		_, _, poster, err := cardAnimationCached(r)
		return r, d, poster, err
	}
	picture, e := readBoundedFile(filepath.Join(dir, "record-"+id+".jpg"), recordPictureLimit)
	if e != nil {
		return r, d, nil, e
	}
	if pictureSHA(picture) != r.Picture {
		return r, d, nil, errors.New("cached card picture digest mismatch")
	}
	return r, d, picture, nil
}

func pictureSHA(picture []byte) string {
	sum := sha256.Sum256(picture)
	return hex.EncodeToString(sum[:])
}

// pullHolder proves a badge holder asked for this very batch, spending one of the badge's early
// refreshes for the day
type pullHolder struct {
	Badge string `json:"badge"`
	At    int64  `json:"at"`
	Sig   string `json:"sig"`
}

// pullHolderPayload is what a holder proof signs under ryoku-pull-v2: the badge id, the proof's
// time as a u64, and SHA-256 over the batch's cards in id order, each id's 8 bytes then the
// version held as a u64, so a proof speaks for one request and nothing else
func pullHolderPayload(badgeID string, at int64, cards map[string]int64) ([]byte, error) {
	raw, e := hex.DecodeString(badgeID)
	if e != nil || len(raw) != 8 || at < 0 {
		return nil, errors.New("invalid holder proof")
	}
	batch := []byte{}
	for _, id := range slices.Sorted(maps.Keys(cards)) {
		card, e := hex.DecodeString(id)
		if e != nil || len(card) != 8 || !validCardID(id) || cards[id] < 0 {
			return nil, errors.New("invalid holder proof")
		}
		batch = binary.BigEndian.AppendUint64(append(batch, card...), uint64(cards[id]))
	}
	sum := sha256.Sum256(batch)
	return append(binary.BigEndian.AppendUint64(raw, uint64(at)), sum[:]...), nil
}

// pullHolderProof signs the batch at at, the gate's time as this binder reckons it, with the
// device of the active badge; it fails on a machine without one
func pullHolderProof(at int64, cards map[string]int64) (pullHolder, error) {
	b, e := badgeActive()
	if e != nil {
		return pullHolder{}, e
	}
	payload, e := pullHolderPayload(b.ID, at, cards)
	if e != nil {
		return pullHolder{}, e
	}
	seed, e := badgeDeviceSeed()
	if e != nil {
		return pullHolder{}, e
	}
	defer clear(seed)
	return pullHolder{Badge: b.Code, At: at, Sig: base64.StdEncoding.EncodeToString(signDomain(seed, "ryoku-pull-v2", payload))}, nil
}
