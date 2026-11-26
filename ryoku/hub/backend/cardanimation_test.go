package main

import (
	"bytes"
	"crypto/ed25519"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"image"
	"image/color"
	"image/png"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func cardAnimationVector(t *testing.T) cardRecord {
	t.Helper()
	raw, e := os.ReadFile("../../../release/team-badges/test/fixtures/card-v5.json")
	if e != nil {
		t.Fatal(e)
	}
	var vector struct {
		V               int        `json:"v"`
		Domain          string     `json:"domain"`
		Seed            string     `json:"seed"`
		PublicKey       string     `json:"publicKey"`
		Record          cardRecord `json:"record"`
		PayloadHex      string     `json:"payloadHex"`
		SigningInputHex string     `json:"signingInputHex"`
	}
	if e = cardAnimationJSON(raw, &vector); e != nil {
		t.Fatal(e)
	}
	payload, e := recordPayload(vector.Record)
	if e != nil {
		t.Fatal(e)
	}
	if hex.EncodeToString(payload) != vector.PayloadHex {
		t.Fatal("JS and Go v5 payload differ")
	}
	if hex.EncodeToString(append([]byte(vector.Domain+"\x00"), payload...)) != vector.SigningInputHex {
		t.Fatal("signature domain framing differs")
	}
	seed, _ := base64.StdEncoding.DecodeString(vector.Seed)
	if !bytes.Equal(signDomain(seed, vector.Domain, payload), mustAnimation64(t, vector.Record.Sig)) {
		t.Fatal("cross-language signature differs")
	}
	if _, e = recordVerifyDevice(vector.Record); e != nil {
		t.Fatal(e)
	}
	return vector.Record
}
func mustAnimation64(t *testing.T, s string) []byte {
	t.Helper()
	b, e := base64.StdEncoding.DecodeString(s)
	if e != nil {
		t.Fatal(e)
	}
	return b
}
func TestCardAnimationSharedVectorAndEveryBinding(t *testing.T) {
	r := cardAnimationVector(t)
	mutations := map[string]func(*cardRecord){"id": func(x *cardRecord) { x.ID = "0000000000000000" }, "updated": func(x *cardRecord) { x.Updated++ }, "body": func(x *cardRecord) { x.Body = base64.StdEncoding.EncodeToString([]byte("different")) }, "picture": func(x *cardRecord) { x.Picture = strings.Repeat("a", 64) }, "device": func(x *cardRecord) { x.Device = base64.StdEncoding.EncodeToString(bytes.Repeat([]byte{1}, 32)) }, "publication": func(x *cardRecord) { x.Publication = "11234567-89ab-4cde-8fab-0123456789ab" }, "canonical": func(x *cardRecord) { x.CanonicalSHA = strings.Repeat("a", 64) }, "approval": func(x *cardRecord) { x.ApprovalSHA = strings.Repeat("a", 64) }, "certificate": func(x *cardRecord) { x.CertificateSHA = strings.Repeat("a", 64) }, "signature": func(x *cardRecord) { x.Sig = base64.StdEncoding.EncodeToString(make([]byte, 64)) }, "downgrade": func(x *cardRecord) { x.V = 2 }, "style": func(x *cardRecord) { x.Style = "stand" }}
	for name, mutate := range mutations {
		t.Run(name, func(t *testing.T) {
			changed := r
			mutate(&changed)
			if _, e := recordVerifyDevice(changed); e == nil {
				t.Fatal("changed record accepted")
			}
		})
	}
}
func cardAnimationTestEnvelope(t *testing.T, v any, domain string, seed []byte) cardAnimationEnvelope {
	t.Helper()
	raw, e := json.Marshal(v)
	if e != nil {
		t.Fatal(e)
	}
	d := json.NewDecoder(bytes.NewReader(raw))
	d.UseNumber()
	var value any
	if e = d.Decode(&value); e != nil {
		t.Fatal(e)
	}
	raw, e = cardAnimationCanonical(value, 0)
	if e != nil {
		t.Fatal(e)
	}
	return cardAnimationEnvelope{Payload: base64.StdEncoding.EncodeToString(raw), Sig: base64.StdEncoding.EncodeToString(signDomain(seed, domain, raw))}
}
func cardAnimationTestPublication(t *testing.T, pack, poster []byte) (cardAnimationPublication, cardRecord) {
	t.Helper()
	t.Setenv("RYOKU_CARD_AUTHORITY_FILE", filepath.Join("testdata", "card-animation-authorities.json"))
	r := cardAnimationVector(t)
	now := time.Now().Unix()
	r.Updated = now
	c := cardAnimationCertificate{V: 1, KeyID: "candidate-inspector", JobID: r.Publication, LeaseSHA: strings.Repeat("1", 64), ManifestSHA: strings.Repeat("2", 64), OriginalSHA: strings.Repeat("3", 64), CanonicalSHA: pictureSHA(pack), PosterSHA: pictureSHA(poster), PreviewSHA: pictureSHA(poster), ReportSHA: pictureSHA([]byte("report")), PipelineVersion: "card-inspector-1", PolicyVersion: "card-data-1", InspectedAt: now - 1, Moderation: "clear"}
	c.Technical.Archive = "pass"
	c.Technical.Decode = "pass"
	c.Technical.Canonical = "pass"
	c.Technical.Render = "pass"
	c.Files = []cardAnimationFile{{"canonical", c.CanonicalSHA, int64(len(pack))}, {"poster", c.PosterSHA, int64(len(poster))}, {"preview", c.PreviewSHA, int64(len(poster))}, {"report", c.ReportSHA, 6}}
	p := cardAnimationPublication{ID: r.Publication, State: "approved", Certificate: cardAnimationTestEnvelope(t, c, "ryoku-inspect-v1", bytes.Repeat([]byte{2}, 32))}
	a := cardAnimationApproval{V: 3, KeyID: "candidate-owner", JobID: r.Publication, AccountID: "test-account", CardID: r.ID, CanonicalSHA: c.CanonicalSHA, PosterSHA: c.PosterSHA, PreviewSHA: c.PreviewSHA, CertificateSHA: pictureSHA(mustAnimation64(t, p.Certificate.Payload)), ContentReviewSHA: strings.Repeat("4", 64), DecisionID: "review-decision", ReviewerRole: "owner", ApprovedAt: now}
	p.Approval = cardAnimationTestEnvelope(t, a, "ryoku-design-approval-v3", bytes.Repeat([]byte{3}, 32))
	r.CanonicalSHA = a.CanonicalSHA
	r.Picture = a.PosterSHA
	r.CertificateSHA = a.CertificateSHA
	r.ApprovalSHA = pictureSHA(mustAnimation64(t, p.Approval.Payload))
	payload, e := recordPayload(r)
	if e != nil {
		t.Fatal(e)
	}
	seed := make([]byte, 32)
	for i := range seed {
		seed[i] = byte(i)
	}
	r.Sig = base64.StdEncoding.EncodeToString(signDomain(seed, recordDomain(r), payload))
	return p, r
}
func TestCardAnimationRestrictedAuthorityAndCanonicalBoundary(t *testing.T) {
	p, r := cardAnimationTestPublication(t, []byte("bounded test package"), []byte("bounded test poster"))
	if _, _, e := cardAnimationProof(p, r); e != nil {
		t.Fatal(e)
	}
	for _, field := range []string{"role", "card", "canonical", "certificate", "signature", "untrusted"} {
		t.Run(field, func(t *testing.T) {
			changed := p
			var a cardAnimationApproval
			if e := json.Unmarshal(mustAnimation64(t, changed.Approval.Payload), &a); e != nil {
				t.Fatal(e)
			}
			switch field {
			case "role":
				a.ReviewerRole = "mika"
			case "card":
				a.CardID = "0000000000000000"
			case "canonical":
				a.CanonicalSHA = strings.Repeat("a", 64)
			case "certificate":
				a.CertificateSHA = strings.Repeat("a", 64)
			case "untrusted":
				a.KeyID = "uploader-key"
			}
			changed.Approval = cardAnimationTestEnvelope(t, a, "ryoku-design-approval-v3", bytes.Repeat([]byte{3}, 32))
			rr := r
			rr.ApprovalSHA = pictureSHA(mustAnimation64(t, changed.Approval.Payload))
			if field == "signature" {
				changed.Approval.Sig = base64.StdEncoding.EncodeToString(make([]byte, 64))
			}
			if _, _, e := cardAnimationProof(changed, rr); e == nil {
				t.Fatal("unscoped authority accepted")
			}
		})
	}
	t.Run("missing registry", func(t *testing.T) {
		t.Setenv("RYOKU_CARD_AUTHORITY_FILE", filepath.Join(t.TempDir(), "absent"))
		if _, _, e := cardAnimationProof(p, r); e == nil {
			t.Fatal("missing trust accepted")
		}
	})
	t.Run("revoked inspector", func(t *testing.T) {
		raw, e := os.ReadFile("testdata/card-animation-authorities.json")
		if e != nil {
			t.Fatal(e)
		}
		raw = bytes.Replace(raw, []byte(`"revoked":false`), []byte(`"revoked":true`), 1)
		file := filepath.Join(t.TempDir(), "registry.json")
		if e = os.WriteFile(file, raw, 0600); e != nil {
			t.Fatal(e)
		}
		t.Setenv("RYOKU_CARD_AUTHORITY_FILE", file)
		if _, _, e = cardAnimationProof(p, r); e == nil {
			t.Fatal("revoked inspector accepted")
		}
	})
	t.Run("noncanonical duplicate JSON", func(t *testing.T) {
		bad := p
		raw := mustAnimation64(t, p.Approval.Payload)
		raw = append([]byte(`{"v":3,`), raw[1:]...)
		bad.Approval.Payload = base64.StdEncoding.EncodeToString(raw)
		bad.Approval.Sig = base64.StdEncoding.EncodeToString(signDomain(bytes.Repeat([]byte{3}, 32), "ryoku-design-approval-v3", raw))
		if _, _, e := cardAnimationProof(bad, r); e == nil {
			t.Fatal("duplicate signed JSON accepted")
		}
	})
}
func TestCardAnimationRealHeroCacheAndTampering(t *testing.T) {
	setCardMachine(t)
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	animation := realTestWebP(t, 12, 10, 3)
	var b bytes.Buffer
	img := image.NewNRGBA(image.Rect(0, 0, 12, 10))
	img.Set(0, 0, color.NRGBA{R: 255, A: 255})
	if e := png.Encode(&b, img); e != nil {
		t.Fatal(e)
	}
	poster := b.Bytes()
	raw, e := designZip(designPack{names: []string{"hero.webp"}, files: map[string][]byte{"hero.webp": animation}, bytes: int64(len(animation))})
	if e != nil {
		t.Fatal(e)
	}
	p, r := cardAnimationTestPublication(t, raw, poster)
	online := true
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		if !online {
			http.NotFound(w, req)
			return
		}
		switch req.URL.Path {
		case "/v1/published-designs/" + p.ID:
			json.NewEncoder(w).Encode(p)
		case "/v1/published-designs/" + p.ID + "/canonical":
			w.Write(raw)
		case "/v1/published-designs/" + p.ID + "/poster":
			w.Write(poster)
		default:
			http.NotFound(w, req)
		}
	}))
	defer server.Close()
	t.Setenv("RYOKU_TEAM_URL", server.URL)
	if e = pullStoreRecord(r, teamVerifiedList{}); e != nil {
		t.Fatal(e)
	}
	d, e := recordVerifyDevice(r)
	if e != nil {
		t.Fatal(e)
	}
	levels, e := encodeLevels(d)
	if e != nil {
		t.Fatal(e)
	}
	entry := entryForDesign(d, levels)
	state := pullState{Checked: time.Now().Unix(), Next: time.Now().Unix() + 900}
	view := recordEnrichCard(entry, state, teamVerifiedList{})
	if !view.Signed || view.Verified || view.Publication != p.ID || view.Hero.MotionFile == "" || view.ApprovalState != "current" {
		t.Fatalf("actual binder did not expose approved hero: %+v", view)
	}
	stream, e := os.ReadFile(view.Hero.MotionFile)
	if e != nil {
		t.Fatal(e)
	}
	if pictureSHA(stream) != view.Hero.MotionSHA {
		t.Fatal("materialized animation differs")
	}
	facts, e := parseWebP(stream)
	if e != nil {
		t.Fatal(e)
	}
	if len(facts.Frames) != 3 {
		t.Fatal("animation frames lost")
	}
	if _, e = recordVerifyDevice(r); e != nil {
		t.Fatal("whole-stream signature changed during frame inspection", e)
	}
	online = false
	if _, _, e = cardAnimationFetch(p.ID); e == nil {
		t.Fatal("withdrawn server publication downloadable")
	}
	state.Gone = []string{r.ID}
	kept := recordEnrichCard(entry, state, teamVerifiedList{})
	if !kept.Signed || kept.Record != "kept" || kept.ApprovalState != "historical" {
		t.Fatal("local historical copy lost or presented current")
	}
	path, e := cardAnimationCache(r, ".zip")
	if e != nil {
		t.Fatal(e)
	}
	changed := append([]byte(nil), raw...)
	changed[len(changed)/2] ^= 1
	if e = os.WriteFile(path, changed, 0600); e != nil {
		t.Fatal(e)
	}
	refused := recordEnrichCard(entry, state, teamVerifiedList{})
	if refused.Signed || refused.Design != nil || refused.Hero.MotionFile != "" || refused.DesignError == "" {
		t.Fatal("changed cached asset retained approved playback")
	}
}
func TestCardAnimationEnvelopeRejectsBundledPublicKey(t *testing.T) {
	seed := bytes.Repeat([]byte{7}, 32)
	pub := ed25519.NewKeyFromSeed(seed).Public().(ed25519.PublicKey)
	p := map[string]any{"keyId": "uploaded", "publicKey": base64.StdEncoding.EncodeToString(pub)}
	envelope := cardAnimationTestEnvelope(t, p, "ryoku-design-approval-v3", seed)
	var out map[string]any
	if _, _, e := cardAnimationOpen(envelope, "ryoku-design-approval-v3", map[string]cardAnimationAuthority{}, &out); e == nil {
		t.Fatal("bundled public key became trusted")
	}
}
func TestCardAnimationRealDesignMaterialization(t *testing.T) {
	setCardMachine(t)
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	animation := realTestWebP(t, 256, 256, 3)
	spec := designTestSpec()
	spec.Name = "Reviewed motion"
	spec.Badge.Emblem = "emblem.webp"
	spec.Card.Art = []designArt{{Image: "art.webp", Depth: 0.5, Blend: "normal"}}
	spec.Card.Effects = []designEffect{}
	manifest, e := json.Marshal(spec)
	if e != nil {
		t.Fatal(e)
	}
	pack := designPack{names: []string{"art.webp", "design.json", "emblem.webp"}, files: map[string][]byte{"art.webp": animation, "design.json": manifest, "emblem.webp": animation}, bytes: int64(len(animation)*2 + len(manifest))}
	raw, e := designZip(pack)
	if e != nil {
		t.Fatal(e)
	}
	var poster bytes.Buffer
	if e = png.Encode(&poster, image.NewNRGBA(image.Rect(0, 0, 256, 256))); e != nil {
		t.Fatal(e)
	}
	publication, record := cardAnimationTestPublication(t, raw, poster.Bytes())
	design, e := cardAnimationMaterialize(record, publication, raw, poster.Bytes())
	if e != nil {
		t.Fatal(e)
	}
	if design == nil || design.SHA != record.CanonicalSHA || design.Spec.Name != "Reviewed motion" || len(design.Spec.Card.Art) != 1 {
		t.Fatal("approved design did not reach real card model")
	}
	asset, e := os.ReadFile(design.Spec.Card.Art[0].Image)
	if e != nil {
		t.Fatal(e)
	}
	frames, e := parseWebP(asset)
	if e != nil || len(frames.Frames) != 3 {
		t.Fatal("card design lost moving asset", e)
	}
	changed := append([]byte(nil), raw...)
	changed[len(changed)/2] ^= 1
	if _, e = cardAnimationMaterialize(record, publication, changed, poster.Bytes()); e == nil {
		t.Fatal("mutated exact asset accepted")
	}
}

func TestCardAnimationV5UnknownFieldsAndStillCompatibility(t *testing.T) {
	record := cardAnimationVector(t)
	raw, e := json.Marshal(record)
	if e != nil {
		t.Fatal(e)
	}
	raw = append([]byte(`{"unapproved":"extension",`), raw[1:]...)
	var decoded cardRecord
	if e = json.Unmarshal(raw, &decoded); e == nil {
		t.Fatal("unsigned v5 extension accepted")
	}
	for _, v := range []int{2, 3, 4} {
		still := record
		still.V = v
		still.Publication = ""
		still.CanonicalSHA = ""
		still.ApprovalSHA = ""
		still.CertificateSHA = ""
		still.Style = ""
		still.Tagline = ""
		still.StandName = ""
		if v == 3 {
			still.Style = "stand"
		}
		if v == 4 {
			still.Tagline = "A signed still"
		}
		payload, e := recordPayload(still)
		if e != nil {
			t.Fatal(e)
		}
		seed := make([]byte, 32)
		for i := range seed {
			seed[i] = byte(i)
		}
		still.Sig = base64.StdEncoding.EncodeToString(signDomain(seed, recordDomain(still), payload))
		if _, e = recordVerifyDevice(still); e != nil {
			t.Fatalf("v%d still no longer verifies: %v", v, e)
		}
	}
}
func TestCardAnimationRefusedReplacementIsNotCurrent(t *testing.T) {
	setCardMachine(t)
	record := cardAnimationVector(t)
	record.Sig = base64.StdEncoding.EncodeToString(make([]byte, 64))
	state := pullState{}
	state.fold([]string{record.ID}, pullReply{Records: []cardRecord{record}}, map[string]int64{record.ID: record.Updated - 1})
	if len(state.Gone) != 1 || state.Gone[0] != record.ID {
		t.Fatal("refused animated replacement left historical copy current")
	}
}
