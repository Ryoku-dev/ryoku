package main

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

const cardReviewTestJob = "12345678-1234-1234-1234-123456789abc"
const cardReviewTestActor = "ra_owner123"

func cardReviewTestJSON(t *testing.T, value any) []byte {
	t.Helper()
	raw, e := cardReviewCanonical(value)
	if e != nil {
		t.Fatal(e)
	}
	return raw
}
func cardReviewTestEnvelope(t *testing.T, value any, domain string, seed []byte) cardAnimationEnvelope {
	t.Helper()
	raw := cardReviewTestJSON(t, value)
	return cardAnimationEnvelope{Payload: base64.StdEncoding.EncodeToString(raw), Sig: base64.StdEncoding.EncodeToString(signDomain(seed, domain, raw))}
}
func cardReviewTestFixture(t *testing.T, gate string) (cardReviewDetail, map[string][]byte, cardAnimationRegistry, []byte) {
	t.Helper()
	inspector, owner, device := bytes.Repeat([]byte{41}, 32), bytes.Repeat([]byte{42}, 32), bytes.Repeat([]byte{43}, 32)
	active := false
	trust := cardAnimationRegistry{V: 1, Inspectors: map[string]cardAnimationAuthority{"inspection": {PublicKey: base64.StdEncoding.EncodeToString(badgeDevicePublic(inspector)), Revoked: &active}}, Approvals: map[string]cardAnimationAuthority{"owner-test": {PublicKey: base64.StdEncoding.EncodeToString(badgeDevicePublic(owner)), Revoked: &active, Role: "owner"}}, PipelineVersions: []string{"pipeline-test"}, PolicyVersions: []string{"policy-test"}}
	now := time.Now().Unix() - 10
	objects := map[string][]byte{"canonical": []byte("canonical ZIP"), "poster": []byte("poster PNG"), "preview": []byte("preview GIF")}
	sha := cardAccountHash(objects["canonical"])
	report := map[string]any{"totalFrames": 2, "durationMs": 100, "composition": map[string]any{"v": 1, "canonicalSha": sha, "posterSha": cardAccountHash(objects["poster"]), "previewSha": cardAccountHash(objects["preview"]), "coverage": "poster-and-canonical-text"}}
	objects["report"] = cardReviewTestJSON(t, report)
	m := cardReviewManifest{V: 1, CardID: cardIDText(badgeDevicePublic(device)), Device: base64.StdEncoding.EncodeToString(badgeDevicePublic(device)), OriginalSHA: strings.Repeat("1", 64), Files: []cardAnimationFile{{Name: "hero.png", SHA: strings.Repeat("2", 64), Bytes: 10}}, TermsVersion: "terms-test", AttestationVersion: "terms-test", OwnOrLicensed: true, PolicyAccepted: true, ConsentToReview: true, Nonce: strings.Repeat("a", 32), SubmittedAt: now - 30, Audience: gate, Kind: "hero"}
	manifest := cardReviewTestEnvelope(t, m, "ryoku-submit-v1", device)
	c := cardAnimationCertificate{V: 1, KeyID: "inspection", JobID: cardReviewTestJob, LeaseSHA: strings.Repeat("3", 64), ManifestSHA: cardAccountHash(cardReviewTestJSON(t, m)), OriginalSHA: m.OriginalSHA, CanonicalSHA: sha, PosterSHA: cardAccountHash(objects["poster"]), PreviewSHA: cardAccountHash(objects["preview"]), ReportSHA: cardAccountHash(objects["report"]), PipelineVersion: "pipeline-test", PolicyVersion: "policy-test", InspectedAt: now - 20, Moderation: "unreviewed"}
	c.Technical.Archive = "pass"
	c.Technical.Decode = "pass"
	c.Technical.Canonical = "pass"
	c.Technical.Render = "pass"
	for _, name := range []string{"canonical", "poster", "preview", "report"} {
		c.Files = append(c.Files, cardAnimationFile{Name: name, SHA: cardAccountHash(objects[name]), Bytes: int64(len(objects[name]))})
	}
	cert := cardReviewTestEnvelope(t, c, "ryoku-inspect-v1", inspector)
	cs := cardAccountHash(cardReviewTestJSON(t, c))
	review := cardReviewAttestation{V: 1, JobID: cardReviewTestJob, Actor: cardReviewTestActor, Role: "owner", At: now - 5, IdempotencyKey: "manual-review"}
	review.Submitted.Verdict = "clear"
	review.Submitted.Reason = "All canonical frames manually inspected"
	review.Submitted.IdempotencyKey = review.IdempotencyKey
	review.Submitted.Model = "manual"
	review.Submitted.ReviewVersion = "manual-v1"
	review.Submitted.PolicyVersion = c.PolicyVersion
	review.Submitted.CanonicalSHA = sha
	review.Submitted.PreviewSHA = c.PreviewSHA
	review.Submitted.ReportSHA = c.ReportSHA
	review.Submitted.CertificateSHA = cs
	review.Submitted.Coverage.Frames = []int{0, 1}
	review.Submitted.Coverage.TotalFrames = 2
	review.Submitted.Coverage.DurationMS = 100
	review.SHA = cardAccountHash(cardReviewTestJSON(t, map[string]any{"v": review.V, "jobId": review.JobID, "actor": review.Actor, "role": review.Role, "at": review.At, "submitted": review.Submitted}))
	decision := cardReviewDecision{ID: "abcdef12-1234-1234-1234-123456789abc", Verdict: "approve", Reason: "Manual owner approval intent", Actor: cardReviewTestActor, Role: "owner", At: now, IdempotencyKey: "owner-decision"}
	payload := map[string]any{"v": 3, "jobId": cardReviewTestJob, "accountId": "ra_author123", "cardId": m.CardID, "canonicalSha": sha, "posterSha": c.PosterSHA, "previewSha": c.PreviewSHA, "certificateSha": cs, "contentReviewSha": review.SHA, "decisionId": decision.ID, "reviewerRole": "owner", "approvedAt": decision.At}
	d := cardReviewDetail{ID: cardReviewTestJob, State: "awaiting_signature", Kind: "hero", AccountID: "ra_author123", CardID: m.CardID, CreatedAt: now - 30, CertificateSHA: cs, Certificate: cert, Manifest: cardReviewTestJSON(t, m), Decision: decision, ContentReviews: []cardReviewAttestation{review}, SigningRequest: &cardReviewRequest{Payload: cardReviewTestJSON(t, payload), Manifest: manifest, Certificate: cert, Decision: decision, ContentReview: review}}
	return d, objects, trust, owner
}
func TestCardReviewOwnerAuthorityAndScope(t *testing.T) {
	_, _, trust, seed := cardReviewTestFixture(t, "https://gate.example")
	if e := cardReviewAuthority(trust, "owner-test", seed); e != nil {
		t.Fatal(e)
	}
	for _, name := range []string{"mika", "unknown", "revoked", "substituted-seed"} {
		t.Run(name, func(t *testing.T) {
			_, _, keys, key := cardReviewTestFixture(t, "https://gate.example")
			a := keys.Approvals["owner-test"]
			switch name {
			case "mika":
				a.Role = "mika"
			case "unknown":
				a.Role = "administrator"
			case "revoked":
				v := true
				a.Revoked = &v
			case "substituted-seed":
				key = bytes.Repeat([]byte{9}, 32)
			}
			keys.Approvals["owner-test"] = a
			if cardReviewAuthority(keys, "owner-test", key) == nil {
				t.Fatal("unauthorized key accepted")
			}
		})
	}
	k := cardReviewKey{V: 1, Gate: "https://gate.example", AccountServer: "https://account.example", KeyID: "owner-test"}
	for _, scope := range [][3]string{{"https://other.example", k.AccountServer, k.KeyID}, {k.Gate, "https://other.example", k.KeyID}, {k.Gate, k.AccountServer, "other-key"}} {
		if cardReviewScope(k, scope[0], scope[1], scope[2]) == nil {
			t.Fatal("cross-scope key accepted")
		}
	}
}
func TestCardReviewRejectsSubstitution(t *testing.T) {
	for _, mutation := range []string{"role", "actor", "account", "hash", "decision", "review", "unreviewed", "coverage", "certificate", "audience", "manifest", "report", "technical", "policy", "payload-key", "future"} {
		t.Run(mutation, func(t *testing.T) {
			d, objects, trust, _ := cardReviewTestFixture(t, "https://gate.example")
			sha := cardAccountHash(objects["canonical"])
			actor := cardReviewTestActor
			gate := "https://gate.example"
			switch mutation {
			case "role":
				d.SigningRequest.Decision.Role = "mika"
			case "actor":
				actor = "ra_someone123"
			case "account":
				d.AccountID = "ra_victim123"
			case "hash":
				sha = strings.Repeat("9", 64)
			case "decision":
				d.SigningRequest.Decision.ID = "11111111-1234-1234-1234-123456789abc"
			case "review":
				d.SigningRequest.ContentReview.Submitted.Reason = "changed findings"
			case "unreviewed":
				d.SigningRequest.ContentReview.Submitted.Verdict = "uncertain"
			case "coverage":
				d.SigningRequest.ContentReview.Submitted.Coverage.Frames = []int{0}
			case "certificate":
				d.SigningRequest.Certificate.Sig = base64.StdEncoding.EncodeToString(make([]byte, 64))
			case "audience":
				gate = "https://another-gate.example"
			case "manifest":
				d.SigningRequest.Manifest.Sig = base64.StdEncoding.EncodeToString(make([]byte, 64))
			case "report":
				objects["report"] = []byte(`{"totalFrames":1}`)
			case "technical":
				var c cardAnimationCertificate
				raw, _ := base64.StdEncoding.DecodeString(d.Certificate.Payload)
				json.Unmarshal(raw, &c)
				c.Technical.Render = "fail"
				d.SigningRequest.Certificate = cardReviewTestEnvelope(t, c, "ryoku-inspect-v1", bytes.Repeat([]byte{41}, 32))
				d.Certificate = d.SigningRequest.Certificate
			case "policy":
				trust.PolicyVersions = []string{"retired-policy"}
			case "payload-key":
				var p map[string]any
				json.Unmarshal(d.SigningRequest.Payload, &p)
				p["keyId"] = "owner-test"
				d.SigningRequest.Payload = cardReviewTestJSON(t, p)
			case "future":
				d.SigningRequest.Decision.At = time.Now().Unix() + 3600
			}
			if _, _, e := cardReviewValidate(d, objects["report"], gate, actor, cardReviewTestJob, sha, trust, time.Now().Unix()); e == nil {
				t.Fatal("substitution accepted")
			}
		})
	}
}
func TestCardReviewRestrictedOwnerFinish(t *testing.T) {
	for _, mode := range []string{"approved", "held", "denied", "changed-output", "false-approved"} {
		t.Run(mode, func(t *testing.T) {
			var d cardReviewDetail
			var objects map[string][]byte
			var trust cardAnimationRegistry
			var seed []byte
			signed := false
			server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Header.Get("Authorization") != "Bearer native-oauth" {
					t.Error("native Account credential missing")
					w.WriteHeader(403)
					return
				}
				if mode == "denied" {
					w.WriteHeader(403)
					return
				}
				base := "/v1/review/submissions/" + cardReviewTestJob
				if r.URL.Path == base {
					json.NewEncoder(w).Encode(d)
					return
				}
				if r.URL.Path == base+"/signature" {
					if r.Method != "POST" || r.Header.Get("Content-Type") != "application/json" || r.ContentLength <= 0 {
						t.Error("invalid signature submission framing")
					}
					var body struct {
						Approval cardAnimationEnvelope `json:"approval"`
					}
					json.NewDecoder(r.Body).Decode(&body)
					var a cardAnimationApproval
					if _, key, e := cardAnimationOpen(body.Approval, "ryoku-design-approval-v3", trust.Approvals, &a); e != nil || key.Role != "owner" || a.CanonicalSHA != cardAccountHash(objects["canonical"]) || a.DecisionID != d.Decision.ID {
						t.Errorf("restricted signature invalid: %v", e)
						w.WriteHeader(403)
						return
					}
					signed = true
					state := "awaiting_signature"
					if mode == "approved" {
						d.State = "approved"
						state = "approved"
					}
					if mode == "false-approved" {
						state = "approved"
					}
					json.NewEncoder(w).Encode(map[string]any{"id": d.ID, "state": state, "decision": d.Decision})
					return
				}
				name := strings.TrimPrefix(r.URL.Path, base+"/")
				raw, ok := objects[name]
				if !ok {
					w.WriteHeader(404)
					return
				}
				if mode == "changed-output" && name == "poster" {
					raw = []byte("changed")
				}
				w.Write(raw)
			}))
			defer server.Close()
			d, objects, trust, seed = cardReviewTestFixture(t, server.URL)
			old := cardAccountHTTP
			cardAccountHTTP = server.Client()
			cardAccountHTTP.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
			defer func() { cardAccountHTTP = old }()
			out, e := cardReviewFinish(context.Background(), server.URL, "native-oauth", cardReviewTestActor, cardReviewTestJob, cardAccountHash(objects["canonical"]), "owner-test", seed, trust)
			if mode == "approved" {
				if e != nil || out == nil || !signed {
					t.Fatalf("real restricted finish failed: %v", e)
				}
			} else if e == nil {
				t.Fatal("unapproved Gate state reported as approved")
			}
			if (mode == "denied" || mode == "changed-output") && signed {
				t.Fatal("signed before checking authorization and bytes")
			}
		})
	}
}

func TestCardReviewImportAndCommandBoundaries(t *testing.T) {
	_, _, trust, seed := cardReviewTestFixture(t, "https://gate.example")
	path := filepath.Join(t.TempDir(), "authorities.json")
	if e := os.WriteFile(path, cardReviewTestJSON(t, trust), 0600); e != nil {
		t.Fatal(e)
	}
	t.Setenv("RYOKU_CARD_AUTHORITY_FILE", path)
	t.Setenv("RYOKU_TEAM_URL", "https://gate.example")
	t.Setenv("RYOKU_ACCOUNTS_URL", "https://account.example")
	// None of these may reach a keyring store; only a matching owner key can.
	for _, candidate := range []struct{ id, input string }{
		{"owner-test", "invalid"},
		{"unregistered", base64.StdEncoding.EncodeToString(seed)},
		{"owner-test", base64.StdEncoding.EncodeToString(bytes.Repeat([]byte{7}, 32))},
		{"owner-test", strings.Repeat("x", 129)},
	} {
		if _, e := cardReviewImport(context.Background(), candidate.id, strings.NewReader(candidate.input)); e == nil {
			t.Fatal("unsafe seed import accepted")
		}
	}
	t.Setenv("RYOKU_TEAM_URL", "http://127.0.0.1:4444")
	if _, e := cardReviewImport(context.Background(), "owner-test", strings.NewReader(base64.StdEncoding.EncodeToString(seed))); e == nil {
		t.Fatal("non-HTTPS key scope accepted")
	}
	for _, args := range [][]string{
		{"sign-json", "{}"},
		{"sign", cardReviewTestJob, strings.Repeat("1", 64), "--key-file", "seed"},
		{"sign", cardReviewTestJob, strings.Repeat("1", 64), "--key-id", "owner-test", "extra"},
		{"sign", "../different", strings.Repeat("1", 64), "--key-id", "owner-test"},
		{"sign", cardReviewTestJob, "changed", "--key-id", "owner-test"},
	} {
		if _, e := cardAccountReview(context.Background(), args); e == nil {
			t.Fatal("generic or unscoped signing command accepted")
		}
	}
}
