package main

import (
	"bytes"
	"context"
	"crypto/ed25519"
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
	"regexp"
	"strconv"
	"strings"
	"time"
)

// the gate's replicas read the badge list again at most every five seconds, so a badge
// published a moment ago may not verify its first record yet
var recordRetryWait = 6 * time.Second

const recordPictureLimit = 400 * 1024

var recordStyleID = regexp.MustCompile(`^[a-z0-9-]{1,32}$`)

type cardRecord struct {
	V       int    `json:"v"`
	ID      string `json:"id"`
	Updated int64  `json:"updated"`
	Body    string `json:"body"`
	Picture string `json:"picture"`
	Sig     string `json:"sig"`
	// Device is the signing machine's public key: its hash is the card id, so the record
	// proves itself without a badge
	Device string `json:"device,omitempty"`
	// Style is a v3 or v4 record's built-in card style, signed with the rest; a classic card is
	// v2 and carries none, so binders that predate styles keep verifying it
	Style string `json:"style,omitempty"`
	// Tagline and StandName are a v4 record's own words, signed with the rest
	Tagline        string `json:"tagline,omitempty"`
	StandName      string `json:"standName,omitempty"`
	Publication    string `json:"publication,omitempty"`
	CanonicalSHA   string `json:"canonicalSha,omitempty"`
	ApprovalSHA    string `json:"approvalSha,omitempty"`
	CertificateSHA string `json:"certificateSha,omitempty"`
}

func recordDomain(r cardRecord) string {
	return "ryoku-card-v" + strconv.Itoa(r.V)
}

// recordHeader checks what a version may carry: v2 nothing, v3 a known-shaped style, v4 an
// optional style and at least one of the words, each within the gate's rule
func recordHeader(r cardRecord) bool {
	if r.V != 5 && (r.Publication != "" || r.CanonicalSHA != "" || r.ApprovalSHA != "" || r.CertificateSHA != "") {
		return false
	}
	styled := recordStyleID.MatchString(r.Style)
	switch r.V {
	case 2:
		return r.Style == "" && r.Tagline == "" && r.StandName == ""
	case 3:
		return styled && r.Tagline == "" && r.StandName == ""
	case 4:
		return (r.Style == "" || styled) && (r.Tagline != "" || r.StandName != "") &&
			(r.Tagline == "" || cardWordsValid(r.Tagline, recordTaglineLimit)) &&
			(r.StandName == "" || cardWordsValid(r.StandName, recordStandNameLimit))
	case 5:
		return r.Style == "" && r.Tagline == "" && r.StandName == "" &&
			cardAnimationUUID.MatchString(r.Publication) && cardAnimationDigest(r.Picture) &&
			cardAnimationDigest(r.CanonicalSHA) && cardAnimationDigest(r.ApprovalSHA) &&
			cardAnimationDigest(r.CertificateSHA) && r.Device != ""
	}
	return false
}

func appendPrefixed(out []byte, s string) []byte {
	return append(binary.BigEndian.AppendUint16(out, uint16(len(s))), s...)
}

// recordPayload is what a record signs: the card bytes (v2); then the style, its byte length as
// a big-endian u16 first (v3); then, in v4, the style, the tagline and the Stand's name, each
// so prefixed and empty when absent
func recordPayload(r cardRecord) ([]byte, error) {
	id, e := hex.DecodeString(r.ID)
	if e != nil || len(id) != 8 || !validCardID(r.ID) || !recordHeader(r) || r.Updated < 0 {
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
	if r.V == 3 || r.V == 4 {
		out = appendPrefixed(out, r.Style)
	}
	if r.V == 4 {
		out = appendPrefixed(appendPrefixed(out, r.Tagline), r.StandName)
	}
	if r.V == 5 {
		out = appendPrefixed(appendPrefixed(out, r.Device), r.Publication)
		for _, digest := range []string{r.CanonicalSHA, r.ApprovalSHA, r.CertificateSHA} {
			raw, _ := hex.DecodeString(digest)
			out = append(out, raw...)
		}
	}
	return out, nil
}

// recordSign signs the card in the version its look needs: classic v2, a style alone v3, words v4
func recordSign(d codedDesign, picture []byte, seed []byte, updated int64, look recordLook) (cardRecord, error) {
	if len(seed) != 32 || d.cardID != cardID(badgeDevicePublic(seed)) {
		return cardRecord{}, errors.New("record id does not match device")
	}
	r := cardRecord{V: look.version(), ID: hex.EncodeToString(d.cardID[:]), Updated: updated, Body: base64.StdEncoding.EncodeToString(d.bytes()),
		Style: look.Style, Tagline: look.Tagline, StandName: look.StandName, Device: base64.StdEncoding.EncodeToString(badgeDevicePublic(seed))}
	if len(picture) > 0 {
		sum := sha256.Sum256(picture)
		r.Picture = hex.EncodeToString(sum[:])
	}
	p, e := recordPayload(r)
	if e != nil {
		return r, e
	}
	sig := signDomain(seed, recordDomain(r), p)
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
	if r.Device != "" && r.Device != base64.StdEncoding.EncodeToString(b.device) {
		return codedDesign{}, errors.New("record device differs from its badge")
	}
	return recordVerifyKey(r, b.device)
}

// recordVerifyDevice trusts a record through its own device key, which must hash to its card id
func recordVerifyDevice(r cardRecord) (codedDesign, error) {
	device, e := base64.StdEncoding.DecodeString(r.Device)
	if e != nil || len(device) != ed25519.PublicKeySize || base64.StdEncoding.EncodeToString(device) != r.Device || r.ID != cardIDText(device) {
		return codedDesign{}, errors.New("record device does not own its card id")
	}
	return recordVerifyKey(r, device)
}

// recordVerifyFor verifies through the card's badge when it has one, else through the record's device
func recordVerifyFor(r cardRecord, list teamVerifiedList) (codedDesign, error) {
	if b, ok := teamBadgeForCardID(list, r.ID); ok {
		return recordVerify(r, b)
	}
	if r.Device == "" {
		return codedDesign{}, errors.New("no badge verifies this record")
	}
	return recordVerifyDevice(r)
}
func recordVerifyKey(r cardRecord, device []byte) (codedDesign, error) {
	payload, e := recordPayload(r)
	if e != nil {
		return codedDesign{}, e
	}
	sig, e := base64.StdEncoding.DecodeString(r.Sig)
	if e != nil || !verifyDomain(device, recordDomain(r), payload, sig) {
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
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	media, e := inspectMedia(ctx, raw, false)
	cancel()
	if e != nil || media.Extension != "png" {
		return nil, errors.New("card picture must be a safely decoded PNG")
	}
	if !validPictureSize(media.Width, media.Height) || math.Abs(float64(media.Width)/float64(media.Height)-cardW/cardH) > .02 {
		return nil, errors.New("picture must contain the whole rendered card")
	}
	// Only the sandbox's freshly encoded poster reaches the host PNG decoder.
	img, e := png.Decode(bytes.NewReader(media.Poster))
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
	if online, e := recordOnline(); e != nil || !online {
		return false, e
	}
	if handled, synced, err := cardAnimationSync(d, picturePath); handled {
		return synced, err
	}
	var picture []byte
	if picturePath != "" {
		picture, e = recordPicturePNG(picturePath)
		if e != nil {
			return false, e
		}
	}
	look := recordLookFor()
	material := append(d.bytes(), picture...)
	if look.Style != "" {
		// a classic card hashes as it always did, so updating doesn't republish it
		material = append(append(material, 0), look.Style...)
	}
	if look.words() {
		// the style is [a-z0-9-], so the 1 can't be part of it, and each word is length-prefixed
		material = appendPrefixed(appendPrefixed(append(material, 1), look.Tagline), look.StandName)
	}
	sum := sha256.Sum256(material)
	hash := hex.EncodeToString(sum[:])
	statePath, e := recordStatePath()
	if e != nil {
		return false, e
	}
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
		record, e := recordSign(d, picture, seed, stamp, look)
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
		if reply.status == http.StatusForbidden && (reason == "Custom card styles need an active team badge on this machine" || reason == "Card words need an active team badge on this machine") && !waited {
			// a badge activated a moment ago may not have reached the replica that answered
			waited = true
			time.Sleep(recordRetryWait)
			continue
		}
		return false, fmt.Errorf("the team service refused the card: %s", reason)
	}
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
