package main

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"os"
	"regexp"
	"strings"
	"time"
)

var cardReviewKeyID = regexp.MustCompile(`^[A-Za-z0-9_-]{1,80}$`)
var cardReviewNonce = regexp.MustCompile(`^[A-Za-z0-9_-]{22,86}$`)
var cardReviewAssetName = regexp.MustCompile(`^[a-z0-9][a-z0-9_-]{0,31}\.(png|webp|jpg|jpeg|gif)$`)

type cardReviewKey struct {
	V             int    `json:"v"`
	Gate          string `json:"gate"`
	AccountServer string `json:"accountServer"`
	KeyID         string `json:"keyId"`
	Seed          []byte `json:"seed"`
}
type cardReviewDecision struct {
	ID             string `json:"id"`
	Verdict        string `json:"verdict"`
	Reason         string `json:"reason"`
	Actor          string `json:"actor"`
	Role           string `json:"role"`
	At             int64  `json:"at"`
	IdempotencyKey string `json:"idempotencyKey"`
}
type cardReviewSubmitted struct {
	Verdict        string `json:"verdict"`
	Reason         string `json:"reason"`
	IdempotencyKey string `json:"idempotencyKey"`
	Model          string `json:"model"`
	Coverage       struct {
		Frames      []int `json:"frames"`
		TotalFrames int   `json:"totalFrames"`
		DurationMS  int64 `json:"durationMs"`
		Composition *bool `json:"composition,omitempty"`
	} `json:"coverage"`
	PolicyVersion  string `json:"policyVersion"`
	ReviewVersion  string `json:"reviewVersion"`
	CanonicalSHA   string `json:"canonicalSha"`
	PreviewSHA     string `json:"previewSha"`
	ReportSHA      string `json:"reportSha"`
	CertificateSHA string `json:"certificateSha"`
}
type cardReviewAttestation struct {
	V              int                 `json:"v"`
	JobID          string              `json:"jobId"`
	Actor          string              `json:"actor"`
	Role           string              `json:"role"`
	At             int64               `json:"at"`
	Submitted      cardReviewSubmitted `json:"submitted"`
	SHA            string              `json:"sha"`
	IdempotencyKey string              `json:"idempotencyKey"`
}
type cardReviewRequest struct {
	Payload       json.RawMessage       `json:"payload"`
	Manifest      cardAnimationEnvelope `json:"manifest"`
	Certificate   cardAnimationEnvelope `json:"certificate"`
	Decision      cardReviewDecision    `json:"decision"`
	ContentReview cardReviewAttestation `json:"contentReview"`
}
type cardReviewDetail struct {
	ID             string                  `json:"id"`
	State          string                  `json:"state"`
	Kind           string                  `json:"kind"`
	AccountID      string                  `json:"accountId"`
	CardID         string                  `json:"cardId"`
	CreatedAt      int64                   `json:"createdAt"`
	CertificateSHA string                  `json:"certificateSha"`
	Certificate    cardAnimationEnvelope   `json:"certificate"`
	Manifest       json.RawMessage         `json:"manifest"`
	Decision       cardReviewDecision      `json:"decision"`
	ContentReviews []cardReviewAttestation `json:"contentReviews"`
	SigningRequest *cardReviewRequest      `json:"signingRequest"`
}
type cardReviewManifest struct {
	V                  int                 `json:"v"`
	CardID             string              `json:"cardId"`
	Device             string              `json:"device"`
	OriginalSHA        string              `json:"originalSha"`
	Files              []cardAnimationFile `json:"files"`
	TermsVersion       string              `json:"termsVersion"`
	AttestationVersion string              `json:"attestationVersion"`
	OwnOrLicensed      bool                `json:"ownOrLicensed"`
	PolicyAccepted     bool                `json:"policyAccepted"`
	ConsentToReview    bool                `json:"consentToReview"`
	Nonce              string              `json:"nonce"`
	SubmittedAt        int64               `json:"submittedAt"`
	Audience           string              `json:"audience"`
	Kind               string              `json:"kind"`
}

// Canonicalize typed protocol values through the reader's single canonical implementation.
func cardReviewCanonical(value any) ([]byte, error) {
	raw, e := cardAccountJSON(value)
	if e != nil {
		return nil, e
	}
	return cardReviewCanonicalRaw(raw)
}
func cardReviewCanonicalRaw(raw []byte) ([]byte, error) {
	d := json.NewDecoder(bytes.NewReader(raw))
	d.UseNumber()
	var v any
	if e := d.Decode(&v); e != nil {
		return nil, e
	}
	if e := d.Decode(new(any)); e != io.EOF {
		return nil, errors.New("Trailing review JSON")
	}
	return cardAnimationCanonical(v, 0)
}
func cardReviewEqual(a, b any) bool {
	x, e := cardReviewCanonical(a)
	if e != nil {
		return false
	}
	y, e := cardReviewCanonical(b)
	return e == nil && bytes.Equal(x, y)
}
func cardReviewAuthority(trust cardAnimationRegistry, id string, seed []byte) error {
	key, ok := trust.Approvals[id]
	if !cardReviewKeyID.MatchString(id) || !ok || key.Revoked == nil || *key.Revoked || key.Role != "owner" || len(seed) != ed25519.SeedSize {
		return errors.New("A current registered owner-only approval key is required")
	}
	pub, e := cardAnimationBase64(key.PublicKey, ed25519.PublicKeySize)
	if e != nil || !bytes.Equal(pub, badgeDevicePublic(seed)) {
		return errors.New("Approval seed does not match the trusted owner public key")
	}
	return nil
}
func cardReviewScope(k cardReviewKey, gate, server, id string) error {
	if k.V != 1 || k.Gate != gate || k.AccountServer != server || k.KeyID != id {
		return errors.New("Owner approval key is scoped to another Gate, Account server or key ID")
	}
	return nil
}
func cardReviewGate() (string, error) {
	gate, e := cardAccountGate()
	if e != nil || !strings.HasPrefix(gate, "https://") {
		return "", errors.New("Owner signing requires an explicitly configured HTTPS Gate origin")
	}
	return gate, nil
}

// The key occupies a separate Secret Service slot from Account OAuth and badge
// keys. The slot and record both pin its exact Gate origin and key ID.
func cardReviewImport(ctx context.Context, id string, input io.Reader) (any, error) {
	gate, e := cardReviewGate()
	if e != nil {
		return nil, e
	}
	server, e := cardAccountServer()
	if e != nil {
		return nil, e
	}
	trust, e := cardAnimationTrust()
	if e != nil {
		return nil, e
	}
	raw, e := readBounded(input, 128)
	if e != nil {
		return nil, errors.New("Import expects only a standard-base64 Ed25519 seed32 on stdin")
	}
	defer clear(raw)
	seed, e := cardAnimationBase64(strings.TrimSpace(string(raw)), ed25519.SeedSize)
	if e != nil {
		return nil, errors.New("Import expects only a standard-base64 Ed25519 seed32 on stdin")
	}
	defer clear(seed)
	if e = cardReviewAuthority(trust, id, seed); e != nil {
		return nil, e
	}
	k := cardReviewKey{V: 1, Gate: gate, AccountServer: server, KeyID: id, Seed: seed}
	if e = cardAccountSecret(ctx, gate, "store", "review-owner-key:"+id, k, nil); e != nil {
		return nil, e
	}
	return map[string]any{"state": "owner_key_imported", "gate": gate, "keyId": id}, nil
}

// No downloaded key, browser token, arbitrary JSON input or badge issuer can
// reach this signer. The only payload is reconstructed from an authenticated,
// current Gate request after independently checking its entire authority chain.
func cardReviewValidate(d cardReviewDetail, report []byte, gate, actor, job, sha string, trust cardAnimationRegistry, now int64) (cardAnimationApproval, cardAnimationCertificate, error) {
	var a cardAnimationApproval
	var c cardAnimationCertificate
	refuse := func() (cardAnimationApproval, cardAnimationCertificate, error) {
		return a, c, errors.New("Owner signing request does not match current reviewed authority")
	}
	r := d.SigningRequest
	if r == nil || d.ID != job || d.State != "awaiting_signature" || !cardAnimationUUID.MatchString(job) || !cardAnimationDigest(sha) || !cardAccountIDPattern.MatchString(actor) || !cardAccountIDPattern.MatchString(d.AccountID) || !cardAnimationUUID.MatchString(r.Decision.ID) {
		return refuse()
	}
	// keyId is deliberately absent in the Gate's unsigned request.
	var p map[string]json.RawMessage
	if json.Unmarshal(r.Payload, &p) != nil || len(p) != 12 || p["keyId"] != nil || cardAnimationJSON(r.Payload, &a) != nil {
		return refuse()
	}
	cs, _, e := cardAnimationOpen(r.Certificate, "ryoku-inspect-v1", trust.Inspectors, &c)
	if e != nil || r.Certificate != d.Certificate || cs != d.CertificateSHA {
		return refuse()
	}
	pubRaw, e := cardAnimationBase64(r.Manifest.Payload, -1)
	if e != nil {
		return refuse()
	}
	var selector struct {
		Device string `json:"device"`
	}
	if json.Unmarshal(pubRaw, &selector) != nil {
		return refuse()
	}
	active := false
	var m cardReviewManifest
	ms, _, e := cardAnimationOpen(r.Manifest, "ryoku-submit-v1", map[string]cardAnimationAuthority{"": {PublicKey: selector.Device, Revoked: &active}}, &m)
	if e != nil || ms != c.ManifestSHA || !cardReviewEqual(m, json.RawMessage(d.Manifest)) {
		return refuse()
	}
	pub, e := cardAnimationBase64(m.Device, 32)
	if e != nil || m.V != 1 || m.CardID != cardIDText(pub) || m.CardID != d.CardID || m.Audience != gate || m.Kind != d.Kind || (m.Kind != "design" && m.Kind != "hero") || !m.OwnOrLicensed || !m.PolicyAccepted || !m.ConsentToReview || m.TermsVersion == "" || m.AttestationVersion == "" || m.SubmittedAt < 1 || m.SubmittedAt > c.InspectedAt || m.OriginalSHA != c.OriginalSHA || !cardAnimationDigest(m.OriginalSHA) || len(m.Files) < 1 || len(m.Files) > 24 || !cardReviewNonce.MatchString(m.Nonce) {
		return refuse()
	}
	previous := ""
	totalBytes := int64(0)
	hasDesign := false
	for _, f := range m.Files {
		if f.Name <= previous || !cardAnimationDigest(f.SHA) || f.Bytes < 1 || f.Bytes > 24<<20 {
			return refuse()
		}
		if f.Name == "design.json" {
			hasDesign = true
		} else if !cardReviewAssetName.MatchString(f.Name) {
			return refuse()
		}
		previous = f.Name
		totalBytes += f.Bytes
	}
	if totalBytes > 24<<20 || (m.Kind == "design" && !hasDesign) || (m.Kind == "hero" && (len(m.Files) != 1 || hasDesign)) {
		return refuse()
	}
	dec, review := r.Decision, r.ContentReview
	if !cardReviewEqual(dec, d.Decision) || len(d.ContentReviews) == 0 || !cardReviewEqual(review, d.ContentReviews[len(d.ContentReviews)-1]) || dec.Role != "owner" || dec.Actor != actor || dec.Verdict != "approve" || dec.At < 1 || dec.At > now+60 || dec.Reason == "" || dec.IdempotencyKey == "" || review.V != 1 || review.JobID != job || review.Actor != actor || review.Role != "owner" || review.At < c.InspectedAt || review.At > dec.At || review.IdempotencyKey != review.Submitted.IdempotencyKey {
		return refuse()
	}
	s := review.Submitted
	if s.Verdict != "clear" || s.Model != "manual" || s.ReviewVersion != "manual-v1" || s.Reason == "" || s.IdempotencyKey == "" || s.PolicyVersion != c.PolicyVersion || s.CanonicalSHA != sha || s.PreviewSHA != c.PreviewSHA || s.ReportSHA != c.ReportSHA || s.CertificateSHA != cs || cardAccountHash(report) != c.ReportSHA {
		return refuse()
	}
	attestation := map[string]any{"v": review.V, "jobId": review.JobID, "actor": review.Actor, "role": review.Role, "at": review.At, "submitted": review.Submitted}
	raw, e := cardReviewCanonical(attestation)
	if e != nil || cardAccountHash(raw) != review.SHA {
		return refuse()
	}
	var evidence struct {
		TotalFrames int   `json:"totalFrames"`
		DurationMS  int64 `json:"durationMs"`
		Composition struct {
			V            int    `json:"v"`
			CanonicalSHA string `json:"canonicalSha"`
			PosterSHA    string `json:"posterSha"`
			PreviewSHA   string `json:"previewSha"`
			Coverage     string `json:"coverage"`
		} `json:"composition"`
	}
	if json.Unmarshal(report, &evidence) != nil || evidence.TotalFrames < 1 || evidence.TotalFrames > 4096 || evidence.DurationMS < 0 || s.Coverage.TotalFrames != evidence.TotalFrames || s.Coverage.DurationMS != evidence.DurationMS || len(s.Coverage.Frames) != evidence.TotalFrames || evidence.Composition.V != 1 || evidence.Composition.CanonicalSHA != sha || evidence.Composition.PosterSHA != c.PosterSHA || evidence.Composition.PreviewSHA != c.PreviewSHA || evidence.Composition.Coverage != "poster-and-canonical-text" {
		return refuse()
	}
	for i, frame := range s.Coverage.Frames {
		if frame != i {
			return refuse()
		}
	}
	expected := cardAnimationApproval{V: 3, JobID: job, AccountID: d.AccountID, CardID: m.CardID, CanonicalSHA: sha, PosterSHA: c.PosterSHA, PreviewSHA: c.PreviewSHA, CertificateSHA: cs, ContentReviewSHA: review.SHA, DecisionID: dec.ID, ReviewerRole: "owner", ApprovedAt: dec.At}
	if a != expected {
		return refuse()
	}
	// Independently enforce the shipped reader's certificate policy and inventory.
	contains := func(xs []string, value string) bool {
		for _, x := range xs {
			if x == value {
				return true
			}
		}
		return false
	}
	if c.V != 1 || c.JobID != job || c.CanonicalSHA != sha || c.InspectedAt < 1 || c.InspectedAt > dec.At || !contains(trust.PipelineVersions, c.PipelineVersion) || !contains(trust.PolicyVersions, c.PolicyVersion) || c.Technical.Archive != "pass" || c.Technical.Decode != "pass" || c.Technical.Canonical != "pass" || c.Technical.Render != "pass" || !cardAnimationDigest(c.LeaseSHA) || len(c.Files) != 4 {
		return refuse()
	}
	if c.Moderation != "unreviewed" && c.Moderation != "clear" && c.Moderation != "flagged" && c.Moderation != "unavailable" {
		return refuse()
	}
	expectedFiles := map[string]string{"canonical": sha, "poster": c.PosterSHA, "preview": c.PreviewSHA, "report": c.ReportSHA}
	previous = ""
	for _, f := range c.Files {
		limit := int64(8 << 20)
		if f.Name == "preview" {
			limit = 24 << 20
		}
		if f.Name <= previous || !cardAnimationDigest(f.SHA) || expectedFiles[f.Name] != f.SHA || f.Bytes < 1 || f.Bytes > limit {
			return refuse()
		}
		previous = f.Name
	}
	return a, c, nil
}

func cardReviewFetch(ctx context.Context, gate, token, method, path string, body []byte, limit int) ([]byte, error) {
	req, e := http.NewRequestWithContext(ctx, method, gate+path, bytes.NewReader(body))
	if e != nil {
		return nil, e
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/json")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	res, e := cardAccountHTTP.Do(req)
	if e != nil {
		return nil, errors.New("Owner review Gate could not be reached")
	}
	defer res.Body.Close()
	if res.StatusCode != 200 {
		return nil, errors.New("Gate refused owner review authorization or exact publication state")
	}
	raw, e := readBounded(res.Body, int64(limit))
	if e != nil {
		return nil, errors.New("Review response exceeds its bound")
	}
	return raw, nil
}
func cardReviewFinish(ctx context.Context, gate, token, actor, job, sha, id string, seed []byte, trust cardAnimationRegistry) (any, error) {
	if e := cardReviewAuthority(trust, id, seed); e != nil {
		return nil, e
	}
	if !cardAnimationUUID.MatchString(job) || !cardAnimationDigest(sha) {
		return nil, errors.New("Signing needs the exact job UUID and canonical SHA-256")
	}
	base := "/v1/review/submissions/" + job
	raw, e := cardReviewFetch(ctx, gate, token, "GET", base, nil, 9<<20)
	if e != nil {
		return nil, e
	}
	var d cardReviewDetail
	if json.Unmarshal(raw, &d) != nil {
		return nil, errors.New("Invalid Gate review detail")
	}
	report, e := cardReviewFetch(ctx, gate, token, "GET", base+"/report", nil, 8<<20)
	if e != nil {
		return nil, e
	}
	a, c, e := cardReviewValidate(d, report, gate, actor, job, sha, trust, time.Now().Unix())
	if e != nil {
		return nil, e
	}
	for _, f := range c.Files {
		if f.Name == "report" {
			if int64(len(report)) != f.Bytes {
				return nil, errors.New("Certified report size changed")
			}
			continue
		}
		object, err := cardReviewFetch(ctx, gate, token, "GET", base+"/"+f.Name, nil, int(f.Bytes))
		if err != nil {
			return nil, err
		}
		if int64(len(object)) != f.Bytes || cardAccountHash(object) != f.SHA {
			return nil, errors.New("Certified publication bytes changed")
		}
	}
	a.KeyID = id
	payload, e := cardReviewCanonical(a)
	if e != nil {
		return nil, e
	}
	envelope := cardAnimationEnvelope{Payload: base64.StdEncoding.EncodeToString(payload), Sig: base64.StdEncoding.EncodeToString(signDomain(seed, "ryoku-design-approval-v3", payload))}
	body, e := cardAccountJSON(map[string]any{"approval": envelope})
	if e != nil {
		return nil, e
	}
	raw, e = cardReviewFetch(ctx, gate, token, "POST", base+"/signature", body, 64<<10)
	if e != nil {
		return nil, e
	}
	var result struct {
		ID       string             `json:"id"`
		State    string             `json:"state"`
		Decision cardReviewDecision `json:"decision"`
	}
	if cardAnimationJSON(raw, &result) != nil || result.ID != job || result.State != "approved" || !cardReviewEqual(result.Decision, d.Decision) {
		return nil, errors.New("Gate did not store the exact owner approval; publication is not confirmed")
	}
	// Read back the actual stored state rather than trusting a locally made signature.
	raw, e = cardReviewFetch(ctx, gate, token, "GET", base, nil, 9<<20)
	if e != nil {
		return nil, e
	}
	var stored cardReviewDetail
	if json.Unmarshal(raw, &stored) != nil || stored.ID != job || stored.State != "approved" || stored.CertificateSHA != a.CertificateSHA || !cardReviewEqual(stored.Decision, d.Decision) || stored.SigningRequest == nil || !bytes.Equal(stored.SigningRequest.Payload, d.SigningRequest.Payload) {
		return nil, errors.New("Stored Gate approval could not be confirmed")
	}
	return map[string]any{"id": job, "state": "approved", "canonicalSha": sha, "keyId": id}, nil
}
func cardAccountReview(ctx context.Context, args []string) (any, error) {
	if len(args) == 3 && args[0] == "import-key" && args[1] == "--key-id" {
		return cardReviewImport(ctx, args[2], os.Stdin)
	}
	if len(args) != 5 || args[0] != "sign" || args[3] != "--key-id" || !cardReviewKeyID.MatchString(args[4]) || !cardAnimationUUID.MatchString(args[1]) || !cardAnimationDigest(args[2]) {
		return nil, errors.New("review needs import-key --key-id ID (seed32 base64 on stdin), or sign JOB_UUID CANONICAL_SHA --key-id ID")
	}
	gate, e := cardReviewGate()
	if e != nil {
		return nil, e
	}
	server, token, e := cardAccountAccess(ctx)
	if e != nil {
		return nil, e
	}
	var identity cardAccountIdentity
	if e = cardAccountAPI(ctx, server, token, "GET", "/v1/account", nil, &identity); e != nil {
		return nil, e
	}
	var k cardReviewKey
	if e = cardAccountSecret(ctx, gate, "lookup", "review-owner-key:"+args[4], nil, &k); e != nil {
		return nil, errors.New("Import the registered owner approval key into unlocked Secret Service first")
	}
	defer clear(k.Seed)
	if e = cardReviewScope(k, gate, server, args[4]); e != nil {
		return nil, e
	}
	trust, e := cardAnimationTrust()
	if e != nil {
		return nil, e
	}
	return cardReviewFinish(ctx, gate, token, identity.Account.ID, args[1], args[2], args[4], k.Seed, trust)
}
