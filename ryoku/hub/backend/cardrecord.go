package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"image"
	"image/jpeg"
	"image/png"
	"math"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// the gate's replicas read the badge list again at most every five seconds, so a badge
// published a moment ago may not verify its first record yet
var recordRetryWait = 6 * time.Second

const recordPictureLimit = 400 * 1024

type cardRecord struct {
	V       int    `json:"v"`
	ID      string `json:"id"`
	Updated int64  `json:"updated"`
	Body    string `json:"body"`
	Picture string `json:"picture"`
	Sig     string `json:"sig"`
}

func recordPayload(r cardRecord) ([]byte, error) {
	id, e := hex.DecodeString(r.ID)
	if e != nil || len(id) != 8 || !validCardID(r.ID) || r.V != 2 || r.Updated < 0 {
		return nil, errors.New("invalid record header")
	}
	body, e := base64.StdEncoding.DecodeString(r.Body)
	if e != nil || len(body) > 79 {
		return nil, errors.New("invalid record body")
	}
	d, e := parseDesign(body)
	if e != nil || !bytes.Equal(d.cardID[:], id) {
		return nil, errors.New("record body card id mismatch")
	}
	picture := make([]byte, 32)
	if r.Picture != "" {
		picture, e = hex.DecodeString(r.Picture)
		if e != nil || len(picture) != 32 || hex.EncodeToString(picture) != r.Picture {
			return nil, errors.New("invalid record picture digest")
		}
	}
	out := append([]byte(nil), id...)
	out = binary.BigEndian.AppendUint64(out, uint64(r.Updated))
	out = binary.BigEndian.AppendUint16(out, uint16(len(body)))
	out = append(out, body...)
	out = append(out, picture...)
	return out, nil
}
func recordSign(d codedDesign, picture []byte, seed []byte, updated int64) (cardRecord, error) {
	if len(seed) != 32 || d.cardID != cardID(badgeDevicePublic(seed)) {
		return cardRecord{}, errors.New("record id does not match device")
	}
	r := cardRecord{V: 2, ID: hex.EncodeToString(d.cardID[:]), Updated: updated, Body: base64.StdEncoding.EncodeToString(d.bytes())}
	if len(picture) > 0 {
		sum := sha256.Sum256(picture)
		r.Picture = hex.EncodeToString(sum[:])
	}
	p, e := recordPayload(r)
	if e != nil {
		return r, e
	}
	sig := signDomain(seed, "ryoku-card-v2", p)
	if len(sig) != 64 {
		return r, errors.New("invalid device seed")
	}
	r.Sig = base64.StdEncoding.EncodeToString(sig)
	return r, nil
}
func recordVerify(r cardRecord, b badgeInfo) (codedDesign, error) {
	if r.ID != b.CardID || r.ID != cardIDText(b.device) {
		return codedDesign{}, errors.New("record belongs to another badge")
	}
	payload, e := recordPayload(r)
	if e != nil {
		return codedDesign{}, e
	}
	sig, e := base64.StdEncoding.DecodeString(r.Sig)
	if e != nil || !verifyDomain(b.device, "ryoku-card-v2", payload, sig) {
		return codedDesign{}, errors.New("invalid device signature")
	}
	body, _ := base64.StdEncoding.DecodeString(r.Body)
	return parseDesign(body)
}
func recordPicturePNG(path string) ([]byte, error) {
	raw, e := readPictureFile(path)
	if e != nil {
		return nil, e
	}
	config, e := png.DecodeConfig(bytes.NewReader(raw))
	if e != nil {
		return nil, errors.New("card picture must be PNG")
	}
	if !validPictureSize(config.Width, config.Height) || math.Abs(float64(config.Width)/float64(config.Height)-cardW/cardH) > .02 {
		return nil, errors.New("picture must contain the whole rendered card")
	}
	img, e := decodePicture(raw)
	if e != nil {
		return nil, e
	}
	bounds := img.Bounds()
	sx := float64(bounds.Dx()) / cardW
	sy := float64(bounds.Dy()) / cardH
	w := min(1120, max(1, int(560*sx)))
	h := min(780, max(1, int(390*sy)))
	dst := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		iy := bounds.Min.Y + int((112+float64(y)*390/float64(h))*sy)
		for x := 0; x < w; x++ {
			ix := bounds.Min.X + int((35+float64(x)*560/float64(w))*sx)
			dst.Set(x, y, img.At(ix, iy))
		}
	}
	for q := 90; q >= 10; q -= 5 {
		var out bytes.Buffer
		if e = jpeg.Encode(&out, dst, &jpeg.Options{Quality: q}); e != nil {
			return nil, e
		}
		if out.Len() <= recordPictureLimit {
			return out.Bytes(), nil
		}
	}
	return nil, errors.New("card picture cannot fit within 400 KB")
}
func recordSync(args []string) (bool, error) {
	picturePath := ""
	hardware := []string{}
	for i := 0; i < len(args); i++ {
		if args[i] == "--picture" {
			if picturePath != "" || i+1 == len(args) {
				return false, errors.New("--picture needs one PNG path")
			}
			i++
			picturePath = args[i]
		} else {
			hardware = append(hardware, args[i])
		}
	}
	d, e := cardDesignArgs(hardware)
	if e != nil {
		return false, e
	}
	b, e := badgeActive()
	if e != nil {
		return false, e
	}
	list, e := teamList(false)
	if e != nil {
		return false, e
	}
	valid := false
	for _, known := range list.Badges {
		if known.ID == b.ID {
			valid = true
		}
	}
	if !valid {
		return false, errors.New("badge is not published or has been revoked")
	}
	var picture []byte
	if picturePath != "" {
		picture, e = recordPicturePNG(picturePath)
		if e != nil {
			return false, e
		}
	}
	content := append([]byte(b.ID), d.bytes()...)
	content = append(content, picture...)
	sum := sha256.Sum256(content)
	hash := hex.EncodeToString(sum[:])
	dir, e := badgeDataDir()
	if e != nil {
		return false, e
	}
	// the hash of what was published last and its stamp: a new record is stamped after it, so a
	// clock that stepped back since can't get the record refused as older than the last
	statePath := filepath.Join(dir, "record.sha256")
	state, _ := readBoundedFile(statePath, 128)
	published := strings.Fields(string(state))
	if len(published) > 0 && published[0] == hash {
		return true, nil
	}
	var last int64
	if len(published) > 1 {
		last, _ = strconv.ParseInt(published[1], 10, 64)
	}
	seed, e := badgeDeviceSeed()
	if e != nil {
		return false, e
	}
	defer clear(seed)
	stamp := max(time.Now().Unix(), last+1)
	clocked, waited := false, false
	for {
		record, e := recordSign(d, picture, seed, stamp)
		if e != nil {
			return false, e
		}
		request := struct {
			Record  cardRecord `json:"record"`
			Picture string     `json:"picture,omitempty"`
		}{Record: record}
		if len(picture) > 0 {
			request.Picture = base64.StdEncoding.EncodeToString(picture)
		}
		raw, e := json.Marshal(request)
		if e != nil {
			return false, e
		}
		reply, e := teamExchange(http.MethodPut, "/v1/cards/"+record.ID, "application/json", raw, nil, 4096)
		if e != nil {
			return false, e
		}
		if reply.status == http.StatusOK {
			if e = atomicWrite(statePath, []byte(hash+" "+strconv.FormatInt(stamp, 10)), 0600); e != nil {
				return false, e
			}
			return true, nil
		}
		reason := teamRefusal(reply)
		if reply.status == http.StatusBadRequest {
			switch {
			case reason == "Invalid card id or timestamp" && !clocked && !reply.date.IsZero():
				// this computer's clock is more than ten minutes off: stamp the record in the gate's time
				clocked = true
				stamp = max(reply.date.Unix(), last+1)
				continue
			case reason == "No active badge verifies this record" && !waited:
				waited = true
				time.Sleep(recordRetryWait)
				continue
			}
		}
		return false, fmt.Errorf("the team service refused the card: %s", reason)
	}
}
func recordFetch(b badgeInfo) (cardRecord, codedDesign, []byte, error) {
	raw, status, e := teamGet("/v1/cards/"+b.CardID, 4096)
	if e != nil {
		return cardRecord{}, codedDesign{}, nil, e
	}
	if status != http.StatusOK {
		return cardRecord{}, codedDesign{}, nil, fmt.Errorf("card record returned HTTP %d", status)
	}
	var r cardRecord
	if e = json.Unmarshal(raw, &r); e != nil {
		return r, codedDesign{}, nil, e
	}
	d, e := recordVerify(r, b)
	if e != nil {
		return r, d, nil, e
	}
	dir, e := teamCacheDir()
	if e != nil {
		return r, d, nil, e
	}
	path := filepath.Join(dir, "record-"+b.CardID+".json")
	if old, e := readBoundedFile(path, 4096); e == nil {
		var previous cardRecord
		if json.Unmarshal(old, &previous) == nil {
			if _, e = recordVerify(previous, b); e == nil && (r.Updated < previous.Updated || (r.Updated == previous.Updated && r.Sig != previous.Sig)) {
				return r, d, nil, errors.New("card record replay")
			}
		}
	}
	// the picture is kept beside the record, so recordCached can stand in with both, and an
	// unchanged picture isn't downloaded again on every refresh
	picturePath := filepath.Join(dir, "record-"+b.CardID+".jpg")
	var picture []byte
	if r.Picture != "" {
		if held, e := readBoundedFile(picturePath, recordPictureLimit); e == nil {
			if sum := sha256.Sum256(held); hex.EncodeToString(sum[:]) == r.Picture {
				picture = held
			}
		}
	}
	if r.Picture != "" && picture == nil {
		picture, status, e = teamGet("/v1/cards/"+b.CardID+"/"+r.Picture+".jpg", recordPictureLimit)
		if e != nil {
			return r, d, nil, e
		}
		if status != http.StatusOK {
			return r, d, nil, errors.New("card picture unavailable")
		}
		sum := sha256.Sum256(picture)
		if hex.EncodeToString(sum[:]) != r.Picture {
			return r, d, nil, errors.New("card picture digest mismatch")
		}
		cfg, e := jpeg.DecodeConfig(bytes.NewReader(picture))
		if e != nil || cfg.Width > 1120 || cfg.Height > 780 || cfg.Width < 1 || cfg.Height < 1 {
			return r, d, nil, errors.New("invalid record JPEG")
		}
		if e = atomicWrite(picturePath, picture, 0600); e != nil {
			return r, d, nil, e
		}
	}
	if r.Picture == "" {
		if e = os.Remove(picturePath); e != nil && !os.IsNotExist(e) {
			return r, d, nil, e
		}
	}
	if e = atomicWrite(path, raw, 0600); e != nil {
		return r, d, nil, e
	}
	return r, d, picture, nil
}

// recordCached is the last record this binder verified for b, with its picture. It stands in
// when the gate can't hand over a fresh one (a dropped connection, the rate limit, a copy from
// before the owner's latest change) and is checked against the badge again, so only the badge
// leaving the team list drops it
func recordCached(b badgeInfo) (cardRecord, codedDesign, []byte, error) {
	dir, e := teamCacheDir()
	if e != nil {
		return cardRecord{}, codedDesign{}, nil, e
	}
	raw, e := readBoundedFile(filepath.Join(dir, "record-"+b.CardID+".json"), 4096)
	if e != nil {
		return cardRecord{}, codedDesign{}, nil, e
	}
	var r cardRecord
	if e = json.Unmarshal(raw, &r); e != nil {
		return r, codedDesign{}, nil, e
	}
	d, e := recordVerify(r, b)
	if e != nil || r.Picture == "" {
		return r, d, nil, e
	}
	picture, e := readBoundedFile(filepath.Join(dir, "record-"+b.CardID+".jpg"), recordPictureLimit)
	if e != nil {
		return r, d, nil, e
	}
	if sum := sha256.Sum256(picture); hex.EncodeToString(sum[:]) != r.Picture {
		return r, d, nil, errors.New("cached card picture digest mismatch")
	}
	return r, d, picture, nil
}
func recordStorePicture(id string, raw []byte) (string, error) {
	if !validCardID(id) {
		return "", errors.New("invalid card id")
	}
	img, e := decodePicture(raw)
	if e != nil {
		return "", e
	}
	var out bytes.Buffer
	if e = png.Encode(&out, img); e != nil {
		return "", e
	}
	path, e := filepath.Abs(filepath.Join(cardsDir(), id+"-record.png"))
	if e != nil {
		return "", e
	}
	if e = os.MkdirAll(cardsDir(), 0755); e != nil {
		return "", e
	}
	return path, atomicWrite(path, out.Bytes(), 0644)
}
