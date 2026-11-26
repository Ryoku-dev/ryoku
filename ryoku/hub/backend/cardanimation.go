package main

import (
	"context"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

const cardAnimationPosterLimit int64 = 8 << 20

func cardAnimationFetch(id string) (cardAnimationPublication, int, error) {
	var p cardAnimationPublication
	if !cardAnimationUUID.MatchString(id) {
		return p, 0, errors.New("invalid publication id")
	}
	raw, status, e := teamGet("/v1/published-designs/"+id, 64<<10)
	if e != nil {
		return p, status, e
	}
	if status != http.StatusOK {
		return p, status, fmt.Errorf("approved card publication unavailable (%d)", status)
	}
	e = cardAnimationJSON(raw, &p)
	return p, status, e
}

// Selection checks the same restricted authority and actual package boundary
// as publishing, without creating a record or enabling live sync.
func cardAnimationSelectVerify(ctx context.Context, gate, publicationID string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	configured, err := teamURL()
	if err != nil {
		return err
	}
	if strings.TrimRight(configured, "/") != strings.TrimRight(gate, "/") {
		return errors.New("publication gate does not match configured gate")
	}
	publication, _, err := cardAnimationFetch(publicationID)
	if err != nil {
		return err
	}
	seed, err := badgeDeviceSeed()
	if err != nil {
		return err
	}
	defer clear(seed)
	raw, err := cardAnimationBase64(publication.Approval.Payload, -1)
	if err != nil {
		return err
	}
	var approval cardAnimationApproval
	if err = cardAnimationJSON(raw, &approval); err != nil {
		return err
	}
	certificate, err := cardAnimationBase64(publication.Certificate.Payload, -1)
	if err != nil {
		return err
	}
	binding := cardRecord{V: 5, ID: cardIDText(badgeDevicePublic(seed)), Device: base64.StdEncoding.EncodeToString(badgeDevicePublic(seed)), Publication: publicationID, CanonicalSHA: approval.CanonicalSHA, Picture: approval.PosterSHA, ApprovalSHA: pictureSHA(raw), CertificateSHA: pictureSHA(certificate)}
	if _, _, err = cardAnimationDownload(binding, publication); err != nil {
		return err
	}
	return ctx.Err()
}
func cardAnimationRecord(d codedDesign, p cardAnimationPublication, seed []byte, at int64) (cardRecord, error) {
	r := cardRecord{V: 5, ID: hex.EncodeToString(d.cardID[:]), Updated: at, Body: base64.StdEncoding.EncodeToString(d.bytes()), Device: base64.StdEncoding.EncodeToString(badgeDevicePublic(seed)), Publication: p.ID}
	// These candidate fields select expected bindings only. The independent key
	// registry and both complete signatures below grant authority.
	raw, e := cardAnimationBase64(p.Approval.Payload, -1)
	if e != nil {
		return r, e
	}
	var a cardAnimationApproval
	if e = cardAnimationJSON(raw, &a); e != nil {
		return r, e
	}
	cert, e := cardAnimationBase64(p.Certificate.Payload, -1)
	if e != nil {
		return r, e
	}
	r.CanonicalSHA = a.CanonicalSHA
	r.Picture = a.PosterSHA
	r.ApprovalSHA = pictureSHA(raw)
	r.CertificateSHA = pictureSHA(cert)
	if _, _, e = cardAnimationProof(p, r); e != nil {
		return r, e
	}
	if r.ID != cardIDText(badgeDevicePublic(seed)) {
		return r, errors.New("publication belongs to another device")
	}
	payload, e := recordPayload(r)
	if e != nil {
		return r, e
	}
	r.Sig = base64.StdEncoding.EncodeToString(signDomain(seed, recordDomain(r), payload))
	return r, nil
}

// A revoked/withdrawn/ineligible publication falls back only to the ordinary
// signed still path. It cannot retain its approved-animation classification.
func cardAnimationSync(d codedDesign, picturePath string) (handled, synced bool, err error) {
	ctx, cancel := context.WithTimeout(context.Background(), 25*time.Second)
	defer cancel()
	gate, id, e := cardApprovedPublication(ctx)
	if e != nil {
		return true, false, e
	}
	if id == "" {
		return false, false, nil
	}
	configured, e := teamURL()
	if e != nil || strings.TrimRight(configured, "/") != strings.TrimRight(gate, "/") {
		return true, false, errors.New("selected publication gate does not match configured gate")
	}
	p, status, e := cardAnimationFetch(id)
	if status == http.StatusForbidden || status == http.StatusNotFound {
		return false, false, nil
	}
	if e != nil {
		return true, false, e
	}
	server, token, e := cardAccountAccess(ctx)
	if e != nil {
		return true, false, e
	}
	if server == "" || token == "" {
		return true, false, errors.New("card account authorization unavailable")
	}
	seed, e := badgeDeviceSeed()
	if e != nil {
		return true, false, e
	}
	defer clear(seed)
	statePath, e := recordStatePath()
	if e != nil {
		return true, false, e
	}
	held, _ := readBoundedFile(statePath, 128)
	fields := strings.Fields(string(held))
	var last int64
	if len(fields) > 1 {
		last, _ = strconv.ParseInt(fields[1], 10, 64)
	}
	r, e := cardAnimationRecord(d, p, seed, max(time.Now().Unix(), last+1))
	if e != nil {
		return true, false, e
	}
	hash := pictureSHA(append(append(d.bytes(), []byte(r.CanonicalSHA)...), []byte(r.ApprovalSHA)...))
	if len(fields) > 0 && fields[0] == hash {
		return true, true, nil
	}
	// Verify and safely materialize the actual certified package on the sender as
	// well as the recipient; selecting an approved UUID never trusts a fresh JPEG.
	if _, _, e = cardAnimationDownload(r, p); e != nil {
		return true, false, e
	}
	raw, e := json.Marshal(struct {
		Record cardRecord `json:"record"`
	}{r})
	if e != nil {
		return true, false, e
	}
	header := make(http.Header)
	header.Set("Authorization", "Bearer "+token)
	reply, e := teamExchange(http.MethodPut, "/v1/cards/"+r.ID, "application/json", raw, header, 4096)
	if e != nil {
		return true, false, e
	}
	if reply.status == http.StatusForbidden || reply.status == http.StatusNotFound {
		return false, false, nil
	}
	if reply.status != http.StatusOK {
		return true, false, fmt.Errorf("animated card publication refused: %s", teamRefusal(reply))
	}
	e = atomicWrite(statePath, []byte(hash+" "+strconv.FormatInt(r.Updated, 10)), 0600)
	return true, e == nil, e
}

func cardAnimationCache(r cardRecord, suffix string) (string, error) {
	if !validCardID(r.ID) || !cardAnimationDigest(r.CanonicalSHA) {
		return "", errors.New("invalid approved cache identity")
	}
	dir, e := teamCacheDir()
	if e != nil {
		return "", e
	}
	return filepath.Join(dir, "animation-"+r.ID+"-"+r.CanonicalSHA+suffix), nil
}
func cardAnimationDownload(r cardRecord, p cardAnimationPublication) ([]byte, []byte, error) {
	if _, _, e := cardAnimationProof(p, r); e != nil {
		return nil, nil, e
	}
	raw, status, e := teamGet("/v1/published-designs/"+r.Publication+"/canonical", designZipLimit)
	if e != nil {
		return nil, nil, e
	}
	if status != http.StatusOK || pictureSHA(raw) != r.CanonicalSHA {
		return nil, nil, errors.New("certified package unavailable or changed")
	}
	poster, status, e := teamGet("/v1/published-designs/"+r.Publication+"/poster", cardAnimationPosterLimit)
	if e != nil {
		return nil, nil, e
	}
	if status != http.StatusOK || pictureSHA(poster) != r.Picture {
		return nil, nil, errors.New("certified poster unavailable or changed")
	}
	if _, e = cardAnimationMaterialize(r, p, raw, poster); e != nil {
		return nil, nil, e
	}
	return raw, poster, nil
}
func cardAnimationStore(r cardRecord) error {
	p, _, e := cardAnimationFetch(r.Publication)
	if e != nil {
		return e
	}
	raw, poster, e := cardAnimationDownload(r, p)
	if e != nil {
		return e
	}
	for suffix, data := range map[string][]byte{".zip": raw, ".png": poster} {
		path, e := cardAnimationCache(r, suffix)
		if e != nil {
			return e
		}
		if e = atomicWrite(path, data, 0600); e != nil {
			return e
		}
	}
	proof, e := json.Marshal(p)
	if e != nil {
		return e
	}
	path, e := cardAnimationCache(r, ".json")
	if e != nil {
		return e
	}
	if e = atomicWrite(path, proof, 0600); e != nil {
		return e
	}
	dir, e := teamCacheDir()
	if e != nil {
		return e
	}
	record, e := json.Marshal(r)
	if e != nil {
		return e
	}
	return atomicWrite(filepath.Join(dir, "record-"+r.ID+".json"), record, 0600)
}
func cardAnimationCached(r cardRecord) (cardAnimationPublication, []byte, []byte, error) {
	var p cardAnimationPublication
	path, e := cardAnimationCache(r, ".json")
	if e != nil {
		return p, nil, nil, e
	}
	proof, e := readBoundedFile(path, 64<<10)
	if e != nil {
		return p, nil, nil, e
	}
	if e = cardAnimationJSON(proof, &p); e != nil {
		return p, nil, nil, e
	}
	if _, _, e = cardAnimationProof(p, r); e != nil {
		return p, nil, nil, e
	}
	path, e = cardAnimationCache(r, ".zip")
	if e != nil {
		return p, nil, nil, e
	}
	raw, e := readBoundedFile(path, designZipLimit)
	if e != nil {
		return p, nil, nil, e
	}
	path, e = cardAnimationCache(r, ".png")
	if e != nil {
		return p, nil, nil, e
	}
	poster, e := readBoundedFile(path, cardAnimationPosterLimit)
	if e != nil {
		return p, nil, nil, e
	}
	if pictureSHA(raw) != r.CanonicalSHA || pictureSHA(poster) != r.Picture {
		return p, nil, nil, errors.New("cached approved media changed")
	}
	return p, raw, poster, nil
}
func cardAnimationMaterialize(r cardRecord, p cardAnimationPublication, raw, poster []byte) (*badgeCardDesign, error) {
	_, certificate, e := cardAnimationProof(p, r)
	if e != nil {
		return nil, e
	}
	if pictureSHA(raw) != r.CanonicalSHA || pictureSHA(poster) != r.Picture {
		return nil, errors.New("approved media digest mismatch")
	}
	for _, file := range certificate.Files {
		if (file.Name == "canonical" && file.Bytes != int64(len(raw))) || (file.Name == "poster" && file.Bytes != int64(len(poster))) {
			return nil, errors.New("certified media length mismatch")
		}
	}
	pack, e := designReadZip(raw, "")
	if e != nil {
		return nil, e
	}
	// Re-inspection is mandatory even for certified packages: only locally
	// canonical decoder output may enter Qt, never downloaded compressed bytes.
	if _, hasDesign := pack.files["design.json"]; !hasDesign {
		_, e = cardAnimationHero(r, pack)
		return nil, e
	}
	pack, e = canonicalDesignPack(pack, true)
	if e != nil {
		return nil, e
	}
	spec, e := designValidate(pack)
	if e != nil {
		return nil, e
	}
	return &badgeCardDesign{SHA: r.CanonicalSHA, Dir: pack.root, Name: spec.Name, Spec: spec}, nil
}
func cardAnimationEnrich(entry *binderCard, r cardRecord) error {
	p, raw, poster, e := cardAnimationCached(r)
	if e != nil {
		return e
	}
	design, e := cardAnimationMaterialize(r, p, raw, poster)
	if e != nil {
		return e
	}
	path, e := recordStorePicture(r.ID, poster)
	if e != nil {
		return e
	}
	entry.Hero = cardHero{Kind: "custom", File: path, Rev: r.Picture[:12]}
	if design == nil {
		pack, e := designReadZip(raw, "")
		if e != nil {
			return e
		}
		hero, e := cardAnimationHero(r, pack)
		if e != nil {
			return e
		}
		hero.File = path
		hero.Rev = r.Picture[:12]
		entry.Hero = hero
	}
	entry.Design = design
	entry.Signed = true
	entry.Publication = r.Publication
	entry.ApprovalState = "current"
	return nil
}

func cardAnimationHero(r cardRecord, pack designPack) (cardHero, error) {
	if len(pack.names) != 1 {
		return cardHero{}, errors.New("approved hero must contain exactly one image")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	media, e := inspectMedia(ctx, pack.files[pack.names[0]], false)
	if e != nil {
		return cardHero{}, e
	}
	if media.Extension != "png" && media.Extension != "webp" && media.Extension != "gif" {
		return cardHero{}, errors.New("unsupported canonical hero format")
	}
	dir, e := filepath.Abs(cardsDir())
	if e != nil {
		return cardHero{}, e
	}
	if e = os.MkdirAll(dir, 0700); e != nil {
		return cardHero{}, e
	}
	digest := pictureSHA(media.Data)
	path := filepath.Join(dir, r.ID+"-approved-"+digest+"."+media.Extension)
	if e = atomicWrite(path, media.Data, 0400); e != nil {
		return cardHero{}, e
	}
	return cardHero{Kind: "custom", MotionFile: path, MotionSHA: digest}, nil
}

// Version 5 rejects unknown transport fields instead of treating unsigned
// extensions as part of the approved record. Legacy still parsing is unchanged.
func (r *cardRecord) UnmarshalJSON(raw []byte) error {
	type wireRecord cardRecord
	var candidate wireRecord
	if err := json.Unmarshal(raw, &candidate); err != nil {
		return err
	}
	if candidate.V == 5 {
		var fields map[string]json.RawMessage
		if err := json.Unmarshal(raw, &fields); err != nil {
			return err
		}
		allowed := []string{"v", "id", "updated", "body", "picture", "sig", "device", "publication", "canonicalSha", "approvalSha", "certificateSha"}
		if len(fields) != len(allowed) {
			return errors.New("invalid v5 record fields")
		}
		for _, key := range allowed {
			if _, present := fields[key]; !present {
				return errors.New("missing v5 record field")
			}
		}
		if err := cardAnimationJSON(raw, &candidate); err != nil {
			return err
		}
	}
	*r = cardRecord(candidate)
	return nil
}
