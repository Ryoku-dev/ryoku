package main

import (
	"bytes"
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode/utf16"
	"unicode/utf8"
)

var cardAnimationUUID = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)
var cardAnimationKey = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9]{0,63}$`)

func cardAnimationDigest(s string) bool { _, e := designSHA(s); return e == nil }

type cardAnimationEnvelope struct {
	Payload string `json:"payload"`
	Sig     string `json:"sig"`
}
type cardAnimationAuthority struct {
	PublicKey string `json:"publicKey"`
	Revoked   *bool  `json:"revoked"`
	Role      string `json:"role,omitempty"`
}

// This file is provisioned independently by the installation/operator. No
// downloaded object or publication can add keys to this registry.
type cardAnimationRegistry struct {
	V                int                               `json:"v"`
	Inspectors       map[string]cardAnimationAuthority `json:"inspectors"`
	Approvals        map[string]cardAnimationAuthority `json:"approvals"`
	PipelineVersions []string                          `json:"pipelineVersions"`
	PolicyVersions   []string                          `json:"policyVersions"`
}

func cardAnimationTrust() (cardAnimationRegistry, error) {
	path := os.Getenv("RYOKU_CARD_AUTHORITY_FILE")
	if path == "" {
		path = "/usr/share/ryoku/card-authorities.json"
	}
	raw, e := readBoundedFile(path, 64<<10)
	if e != nil {
		return cardAnimationRegistry{}, fmt.Errorf("card signing registry unavailable: %w", e)
	}
	var r cardAnimationRegistry
	e = cardAnimationJSON(raw, &r)
	if e != nil || r.V != 1 || len(r.Inspectors) == 0 || len(r.Approvals) == 0 || len(r.PipelineVersions) == 0 || len(r.PolicyVersions) == 0 {
		return r, errors.New("card signing registry incomplete")
	}
	return r, nil
}
func cardAnimationJSON(raw []byte, out any) error {
	d := json.NewDecoder(bytes.NewReader(raw))
	d.DisallowUnknownFields()
	if e := d.Decode(out); e != nil {
		return e
	}
	if e := d.Decode(new(any)); e != io.EOF {
		return errors.New("trailing protocol JSON")
	}
	return nil
}
func cardAnimationBase64(s string, n int) ([]byte, error) {
	b, e := base64.StdEncoding.DecodeString(s)
	if e != nil || base64.StdEncoding.EncodeToString(b) != s || (n >= 0 && len(b) != n) {
		return nil, errors.New("invalid canonical base64")
	}
	return b, nil
}
func cardAnimationCanonical(v any, depth int) ([]byte, error) {
	if depth > 12 {
		return nil, errors.New("protocol nesting limit")
	}
	switch x := v.(type) {
	case nil:
		return []byte("null"), nil
	case bool:
		if x {
			return []byte("true"), nil
		}
		return []byte("false"), nil
	case json.Number:
		n, e := strconv.ParseInt(string(x), 10, 64)
		if e != nil || n > 9007199254740991 || n < -9007199254740991 || string(x) == "-0" {
			return nil, errors.New("invalid protocol integer")
		}
		return []byte(strconv.FormatInt(n, 10)), nil
	case string:
		if !utf8.ValidString(x) || len(utf16.Encode([]rune(x))) > 4096 {
			return nil, errors.New("invalid protocol Unicode")
		}
		var b bytes.Buffer
		enc := json.NewEncoder(&b)
		enc.SetEscapeHTML(false)
		if e := enc.Encode(x); e != nil {
			return nil, e
		}
		s := strings.TrimSuffix(b.String(), "\n")
		s = strings.ReplaceAll(strings.ReplaceAll(s, `\u2028`, "\u2028"), `\u2029`, "\u2029")
		return []byte(s), nil
	case []any:
		if len(x) > 4096 {
			return nil, errors.New("protocol array limit")
		}
		parts := make([]string, len(x))
		for i, v := range x {
			b, e := cardAnimationCanonical(v, depth+1)
			if e != nil {
				return nil, e
			}
			parts[i] = string(b)
		}
		return []byte("[" + strings.Join(parts, ",") + "]"), nil
	case map[string]any:
		if len(x) > 64 {
			return nil, errors.New("protocol key limit")
		}
		keys := make([]string, 0, len(x))
		for k := range x {
			if !cardAnimationKey.MatchString(k) {
				return nil, errors.New("invalid protocol key")
			}
			keys = append(keys, k)
		}
		sort.Strings(keys)
		parts := make([]string, 0, len(keys))
		for _, k := range keys {
			b, e := cardAnimationCanonical(x[k], depth+1)
			if e != nil {
				return nil, e
			}
			parts = append(parts, strconv.Quote(k)+":"+string(b))
		}
		return []byte("{" + strings.Join(parts, ",") + "}"), nil
	}
	return nil, errors.New("invalid canonical value")
}
func cardAnimationOpen(envelope cardAnimationEnvelope, domain string, keys map[string]cardAnimationAuthority, out any) (string, cardAnimationAuthority, error) {
	var authority cardAnimationAuthority
	raw, e := cardAnimationBase64(envelope.Payload, -1)
	if e != nil || len(raw) > 48<<10 {
		return "", authority, errors.New("invalid signed payload")
	}
	if !utf8.Valid(raw) {
		return "", authority, errors.New("invalid signed UTF-8")
	}
	d := json.NewDecoder(bytes.NewReader(raw))
	d.UseNumber()
	var value any
	if e = d.Decode(&value); e != nil {
		return "", authority, e
	}
	canonical, e := cardAnimationCanonical(value, 0)
	if e != nil || !bytes.Equal(canonical, raw) {
		return "", authority, errors.New("noncanonical signed payload")
	}
	var selector struct {
		KeyID string `json:"keyId"`
	}
	if e = json.Unmarshal(raw, &selector); e != nil {
		return "", authority, e
	}
	authority, ok := keys[selector.KeyID]
	if !ok || authority.Revoked == nil || *authority.Revoked {
		return "", authority, errors.New("untrusted or revoked signing key")
	}
	pub, e := cardAnimationBase64(authority.PublicKey, 32)
	if e != nil {
		return "", authority, e
	}
	sig, e := cardAnimationBase64(envelope.Sig, 64)
	if e != nil || !verifyDomain(ed25519.PublicKey(pub), domain, raw, sig) {
		return "", authority, errors.New("invalid restricted signature")
	}
	if e = cardAnimationJSON(raw, out); e != nil {
		return "", authority, e
	}
	return pictureSHA(raw), authority, nil
}

type cardAnimationApproval struct {
	V                int    `json:"v"`
	KeyID            string `json:"keyId"`
	JobID            string `json:"jobId"`
	AccountID        string `json:"accountId"`
	CardID           string `json:"cardId"`
	CanonicalSHA     string `json:"canonicalSha"`
	PosterSHA        string `json:"posterSha"`
	PreviewSHA       string `json:"previewSha"`
	CertificateSHA   string `json:"certificateSha"`
	ContentReviewSHA string `json:"contentReviewSha"`
	DecisionID       string `json:"decisionId"`
	ReviewerRole     string `json:"reviewerRole"`
	ApprovedAt       int64  `json:"approvedAt"`
}
type cardAnimationFile struct {
	Name  string `json:"name"`
	SHA   string `json:"sha"`
	Bytes int64  `json:"bytes"`
}
type cardAnimationCertificate struct {
	V               int                 `json:"v"`
	KeyID           string              `json:"keyId"`
	JobID           string              `json:"jobId"`
	LeaseSHA        string              `json:"leaseSha"`
	ManifestSHA     string              `json:"manifestSha"`
	OriginalSHA     string              `json:"originalSha"`
	CanonicalSHA    string              `json:"canonicalSha"`
	Files           []cardAnimationFile `json:"files"`
	PosterSHA       string              `json:"posterSha"`
	PreviewSHA      string              `json:"previewSha"`
	ReportSHA       string              `json:"reportSha"`
	PipelineVersion string              `json:"pipelineVersion"`
	PolicyVersion   string              `json:"policyVersion"`
	InspectedAt     int64               `json:"inspectedAt"`
	Technical       struct {
		Archive   string `json:"archive"`
		Decode    string `json:"decode"`
		Canonical string `json:"canonical"`
		Render    string `json:"render"`
	} `json:"technical"`
	Moderation string `json:"moderation"`
}
type cardAnimationPublication struct {
	ID          string                `json:"id"`
	State       string                `json:"state"`
	Approval    cardAnimationEnvelope `json:"approval"`
	Certificate cardAnimationEnvelope `json:"certificate"`
}

func cardAnimationProof(p cardAnimationPublication, r cardRecord) (cardAnimationApproval, cardAnimationCertificate, error) {
	var a cardAnimationApproval
	var c cardAnimationCertificate
	trust, e := cardAnimationTrust()
	if e != nil {
		return a, c, e
	}
	cs, _, e := cardAnimationOpen(p.Certificate, "ryoku-inspect-v1", trust.Inspectors, &c)
	if e != nil {
		return a, c, e
	}
	as, key, e := cardAnimationOpen(p.Approval, "ryoku-design-approval-v3", trust.Approvals, &a)
	if e != nil {
		return a, c, e
	}
	contains := func(values []string, s string) bool {
		for _, v := range values {
			if v == s {
				return true
			}
		}
		return false
	}
	if p.ID != r.Publication || p.State != "approved" || a.V != 3 || a.JobID != p.ID || a.CardID != r.ID || a.AccountID == "" || a.CanonicalSHA != r.CanonicalSHA || a.PosterSHA != r.Picture || as != r.ApprovalSHA || cs != r.CertificateSHA || a.CertificateSHA != cs || !cardAnimationDigest(a.ContentReviewSHA) || a.DecisionID == "" || (a.ReviewerRole != "owner" && a.ReviewerRole != "mika") || key.Role != a.ReviewerRole || a.ApprovedAt < 1 || a.ApprovedAt > time.Now().Unix()+60 {
		return a, c, errors.New("approval does not authorize this card record")
	}
	if c.V != 1 || c.JobID != p.ID || c.CanonicalSHA != a.CanonicalSHA || c.PosterSHA != a.PosterSHA || c.PreviewSHA != a.PreviewSHA || c.InspectedAt < 1 || c.InspectedAt > a.ApprovedAt || !contains(trust.PipelineVersions, c.PipelineVersion) || !contains(trust.PolicyVersions, c.PolicyVersion) || c.Technical.Archive != "pass" || c.Technical.Decode != "pass" || c.Technical.Canonical != "pass" || c.Technical.Render != "pass" {
		return a, c, errors.New("inspection does not authorize this card package")
	}
	expected := map[string]string{"canonical": c.CanonicalSHA, "poster": c.PosterSHA, "preview": c.PreviewSHA, "report": c.ReportSHA}
	if len(c.Files) != 4 {
		return a, c, errors.New("incomplete certified outputs")
	}
	previous := ""
	for _, f := range c.Files {
		if f.Name <= previous || !cardAnimationDigest(f.SHA) || expected[f.Name] != f.SHA || f.Bytes < 1 || f.Bytes > 24<<20 {
			return a, c, errors.New("invalid certified output inventory")
		}
		previous = f.Name
	}
	for _, s := range []string{c.LeaseSHA, c.ManifestSHA, c.OriginalSHA} {
		if !cardAnimationDigest(s) {
			return a, c, errors.New("incomplete inspection provenance")
		}
	}
	return a, c, nil
}
