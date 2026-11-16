package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"image/jpeg"
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
// the cards it no longer holds
type pullState struct {
	Checked int64    `json:"checked"`
	Next    int64    `json:"next"`
	Paused  string   `json:"paused"`
	Offset  int64    `json:"offset"`
	Badges  []string `json:"badges"`
	Designs []string `json:"designs"`
	Gone    []string `json:"gone"`
}

type pullReply struct {
	V       int          `json:"v"`
	Now     int64        `json:"now"`
	Next    int64        `json:"next"`
	Records []cardRecord `json:"records"`
	Gone    []string     `json:"gone"`
	Badges  []string     `json:"badges"`
	Designs []string     `json:"designs"`
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

// binderPull refreshes every card in the binder once the gate's next check time has come. Badge
// holders check on every look, but wait out a pause the gate asked for like everyone else.
func binderPull(ids []string) (pullState, error) {
	s, e := pullLoad()
	if e != nil || len(ids) == 0 {
		return s, e
	}
	_, holderErr := badgeActive()
	if s.now() < s.Next && (holderErr != nil || s.Paused == "busy" || s.Paused == "daily") {
		return s, nil
	}
	var last pullReply
	for start := 0; start < len(ids); start += pullBatchLimit {
		reply, ok, e := s.pull(ids[start:min(len(ids), start+pullBatchLimit)])
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
	reply, ok, e := s.pull([]string{id})
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
// its answer into s; false when the gate didn't answer, with the reason in s.Paused
func (s *pullState) pull(ids []string) (pullReply, bool, error) {
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
	if holder, e := pullHolderProof(s.now()); e == nil {
		request.Holder = &holder
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
		s.fold(ids, answer, known)
		return answer, true, nil
	case http.StatusTooManyRequests, http.StatusServiceUnavailable:
		var busy struct {
			Next int64 `json:"next"`
		}
		if reply.status == http.StatusTooManyRequests && json.Unmarshal(reply.body, &busy) == nil && busy.Next > 0 {
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
			// a record that fails here leaves the copy held, and is asked for again next time
			_ = pullStoreRecord(r, list)
		}
	}
}

// pullStoreRecord keeps a verified record and its picture beside each other in the team cache,
// so recordCached can stand in with both, and an unchanged picture isn't downloaded again
func pullStoreRecord(r cardRecord, list teamVerifiedList) error {
	if _, e := recordVerifyFor(r, list); e != nil {
		return e
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
		cfg, e := jpeg.DecodeConfig(bytes.NewReader(picture))
		if e != nil || cfg.Width > 1120 || cfg.Height > 780 || cfg.Width < 1 || cfg.Height < 1 {
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

// pullHolder proves the binder belongs to a badge holder, whom the gate lets check without its cadence
type pullHolder struct {
	Badge string `json:"badge"`
	At    int64  `json:"at"`
	Sig   string `json:"sig"`
}

func pullHolderPayload(badgeID string, at int64) ([]byte, error) {
	raw, e := hex.DecodeString(badgeID)
	if e != nil || len(raw) != 8 || at < 0 {
		return nil, errors.New("invalid holder proof")
	}
	return binary.BigEndian.AppendUint64(raw, uint64(at)), nil
}

// pullHolderProof signs at, the gate's time as this binder reckons it, with the device of the
// active badge; it fails on a machine without one
func pullHolderProof(at int64) (pullHolder, error) {
	b, e := badgeActive()
	if e != nil {
		return pullHolder{}, e
	}
	payload, e := pullHolderPayload(b.ID, at)
	if e != nil {
		return pullHolder{}, e
	}
	seed, e := badgeDeviceSeed()
	if e != nil {
		return pullHolder{}, e
	}
	defer clear(seed)
	return pullHolder{Badge: b.Code, At: at, Sig: base64.StdEncoding.EncodeToString(signDomain(seed, "ryoku-pull-v1", payload))}, nil
}
