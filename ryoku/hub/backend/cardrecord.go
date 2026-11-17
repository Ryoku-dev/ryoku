package main

import (
	"bytes"
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
	"slices"
	"strconv"
	"strings"
	"time"
)

// the gate's replicas read the badge list again at most every five seconds, so a badge
// published a moment ago may not verify its first record yet
var recordRetryWait = 6 * time.Second

const recordPictureLimit = 400 * 1024

// recordStyles are the built-in card styles a binder knows how to draw; a style id it doesn't
// know yet is drawn classic
var recordStyles = []string{"stand"}

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
	// Style is a v3 record's built-in card style, signed with the rest; a classic card is v2
	// and carries none, so binders that predate styles keep verifying it
	Style string `json:"style,omitempty"`
}

func recordDomain(r cardRecord) string {
	if r.V == 3 {
		return "ryoku-card-v3"
	}
	return "ryoku-card-v2"
}

func recordPayload(r cardRecord) ([]byte, error) {
	id, e := hex.DecodeString(r.ID)
	styled := r.V == 3 && recordStyleID.MatchString(r.Style)
	if e != nil || len(id) != 8 || !validCardID(r.ID) || !styled && (r.V != 2 || r.Style != "") || r.Updated < 0 {
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
	if styled {
		out = binary.BigEndian.AppendUint16(out, uint16(len(r.Style)))
		out = append(out, r.Style...)
	}
	return out, nil
}

// recordSign signs a classic v2 record, or a v3 one when style names a built-in card style
func recordSign(d codedDesign, picture []byte, seed []byte, updated int64, style string) (cardRecord, error) {
	if len(seed) != 32 || d.cardID != cardID(badgeDevicePublic(seed)) {
		return cardRecord{}, errors.New("record id does not match device")
	}
	r := cardRecord{V: 2, ID: hex.EncodeToString(d.cardID[:]), Updated: updated, Body: base64.StdEncoding.EncodeToString(d.bytes()), Style: style,
		Device: base64.StdEncoding.EncodeToString(badgeDevicePublic(seed))}
	if len(picture) > 0 {
		sum := sha256.Sum256(picture)
		r.Picture = hex.EncodeToString(sum[:])
	}
	if style != "" {
		r.V = 3
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

// recordStyle is the built-in style the published card wears: the profile's choice, which only
// a badge holder publishes, so anyone else's card goes out classic
func recordStyle() string {
	raw, e := readProfileFile(profileConfigPath(), maxProfileBytes)
	var p profileModel
	if e != nil || json.Unmarshal(raw, &p) != nil || !slices.Contains(recordStyles, p.Style) || !badgeHolder() {
		return ""
	}
	return p.Style
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
	var picture []byte
	if picturePath != "" {
		picture, e = recordPicturePNG(picturePath)
		if e != nil {
			return false, e
		}
	}
	style := recordStyle()
	material := append(d.bytes(), picture...)
	if style != "" {
		// a classic card hashes as it always did, so updating doesn't republish it
		material = append(append(material, 0), style...)
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
		record, e := recordSign(d, picture, seed, stamp, style)
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
		if reply.status == http.StatusForbidden && reason == "Custom card styles need an active team badge on this machine" && !waited {
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
