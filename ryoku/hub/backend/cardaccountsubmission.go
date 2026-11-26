package main

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"time"
)

var cardAccountUUID = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)

type cardAccountFile struct {
	Bytes int    `json:"bytes"`
	Name  string `json:"name"`
	SHA   string `json:"sha"`
}
type cardAccountDraft struct {
	Server             string            `json:"server"`
	Account            string            `json:"accountID"`
	Gate               string            `json:"gate"`
	Card               string            `json:"cardID"`
	Kind               string            `json:"kind"`
	SHA                string            `json:"originalSha"`
	Files              []cardAccountFile `json:"files"`
	Nonce              string            `json:"nonce"`
	At                 int64             `json:"submittedAt"`
	Version            string            `json:"termsVersion"`
	AttestationVersion string            `json:"attestationVersion"`
	Preview            string            `json:"preview"`
	Frames             int               `json:"frames"`
}
type cardAccountJob struct {
	ID      string `json:"id"`
	State   string `json:"state"`
	Gate    string `json:"gate"`
	Account string `json:"accountID"`
	Card    string `json:"cardID"`
	SHA     string `json:"originalSha"`
	Expires int64  `json:"expiresAt"`
	Upload  string `json:"uploadPath"`
}
type cardAccountSelection struct {
	Server string `json:"server"`
	ID     string `json:"id"`
}

func cardAccountHash(raw []byte) string { s := sha256.Sum256(raw); return hex.EncodeToString(s[:]) }
func cardAccountGate() (string, error) {
	raw, e := teamURL()
	if e != nil {
		return "", e
	}
	return cardAccountOrigin(raw)
}
func cardApprovedPublication(ctx context.Context) (string, string, error) {
	if e := ctx.Err(); e != nil {
		return "", "", e
	}
	gate, e := cardAccountGate()
	if e != nil {
		return "", "", e
	}
	var selected cardAccountSelection
	e = cardAccountRead("publication.json", &selected)
	if os.IsNotExist(e) {
		return gate, "", nil
	}
	if e != nil {
		return "", "", e
	}
	if selected.Server != gate || !cardAccountUUID.MatchString(selected.ID) {
		return "", "", errors.New("Approved publication selection does not match this Gate")
	}
	return gate, selected.ID, nil
}
func cardAccountGateRequest(ctx context.Context, gate, token, method, path, kind string, raw []byte, out any) error {
	_, e := cardAccountRequest(ctx, method, gate+path, token, kind, raw, out)
	return e
}
func cardAccountBound(ctx context.Context) (server, token, account, card, version, attestationVersion string, err error) {
	server, token, err = cardAccountAccess(ctx)
	if err != nil {
		return
	}
	var identity cardAccountIdentity
	err = cardAccountAPI(ctx, server, token, "GET", "/v1/account", nil, &identity)
	if err != nil {
		return
	}
	account = identity.Account.ID
	var consent cardAccountConsent
	err = cardAccountAPI(ctx, server, token, "GET", "/v1/consent", nil, &consent)
	if err != nil {
		return
	}
	if !consent.cardsReady() {
		err = errors.New("Accept current Account terms and separate optional online-card consent on the Account website first")
		return
	}
	version = consent.Account.Policy.Version
	attestationVersion = consent.Cards.Policy.Version
	seed, e := badgeDeviceSeed()
	if e != nil {
		err = e
		return
	}
	pub := badgeDevicePublic(seed)
	clear(seed)
	card = cardIDText(pub)
	var devices struct {
		Devices []struct {
			Card    string `json:"cardID"`
			Key     string `json:"publicKey"`
			Revoked *int64 `json:"revokedAt"`
		} `json:"devices"`
	}
	err = cardAccountAPI(ctx, server, token, "GET", "/v1/card-devices", nil, &devices)
	if err != nil {
		return
	}
	bound := false
	for _, d := range devices.Devices {
		bound = bound || (d.Card == card && d.Key == base64.StdEncoding.EncodeToString(pub) && d.Revoked == nil)
	}
	if !bound {
		err = errors.New("Link this device to the signed-in Ryoku Account first")
	}
	return
}
func cardAccountPrepareSubmission(ctx context.Context, kind, path string) (any, error) {
	var previous cardAccountJob
	if e := cardAccountRead("submission.json", &previous); e == nil && previous.State == "uploading" {
		return nil, errors.New("Retry or cancel the reserved upload before preparing another archive")
	} else if e != nil && !os.IsNotExist(e) {
		return nil, e
	}
	server, _, account, card, version, attestationVersion, e := cardAccountBound(ctx)
	if e != nil {
		return nil, e
	}
	gate, e := cardAccountGate()
	if e != nil {
		return nil, e
	}
	var raw []byte
	var original designPack
	preview := ""
	frames := 0
	if kind == "design" {
		st, err := os.Lstat(path)
		if err != nil {
			return nil, err
		}
		original, err = designReadPack(path)
		if err != nil {
			return nil, err
		}
		canonical, err := canonicalDesignPack(original, true)
		if err != nil {
			return nil, err
		}
		preview = canonical.root
		// upload the user's ZIP unchanged, not the local preview pack
		if st.IsDir() {
			raw, err = designZip(original)
		} else {
			raw, err = readBoundedFile(path, designZipLimit)
			if err == nil {
				var snapshot designPack
				snapshot, err = designReadZip(raw, "")
				if err == nil && !cardAccountSamePack(snapshot, original) {
					err = errors.New("Design changed during inspection")
				}
			}
		}
		if err != nil {
			return nil, err
		}
	} else if kind == "hero" {
		source, err := readBoundedFile(path, 8<<20)
		if err != nil {
			return nil, err
		}
		inspected, err := inspectMedia(ctx, source, false)
		if err != nil {
			return nil, err
		}
		frames = inspected.Frames
		// the independent inspector needs the original media, not our preview
		extension := filepath.Ext(path)
		if !regexp.MustCompile(`^\.(png|jpg|jpeg|gif|webp)$`).MatchString(extension) {
			return nil, errors.New("Choose a supported PNG, JPEG, GIF or WebP image")
		}
		name := "hero" + extension
		original = designPack{files: map[string][]byte{name: source}, names: []string{name}, bytes: int64(len(source))}
		raw, err = designZip(original)
		if err != nil {
			return nil, err
		}
		dir, err := cardAccountDir()
		if err != nil {
			return nil, err
		}
		preview = filepath.Join(dir, "submission-preview."+inspected.Extension)
		if err = cardAccountWriteBytes(filepath.Base(preview), inspected.Data); err != nil {
			return nil, err
		}
	} else {
		return nil, errors.New("Submission kind must be design or hero")
	}
	if e = cardAccountOriginalInventory(raw, original); e != nil {
		return nil, e
	}
	nonce := make([]byte, 32)
	if _, e = rand.Read(nonce); e != nil {
		return nil, e
	}
	draft := cardAccountDraft{Server: server, Account: account, Gate: gate, Card: card, Kind: kind, SHA: cardAccountHash(raw), Nonce: base64.RawURLEncoding.EncodeToString(nonce), At: time.Now().Unix(), Version: version, AttestationVersion: attestationVersion, Preview: preview, Frames: frames}
	sort.Strings(original.names)
	for _, name := range original.names {
		data := original.files[name]
		draft.Files = append(draft.Files, cardAccountFile{len(data), name, cardAccountHash(data)})
	}
	dir, e := cardAccountDir()
	if e != nil {
		return nil, e
	}
	f, e := os.CreateTemp(dir, ".submission-")
	if e != nil {
		return nil, e
	}
	defer os.Remove(f.Name())
	if e = f.Chmod(0600); e == nil {
		_, e = f.Write(raw)
	}
	ce := f.Close()
	if e == nil {
		e = ce
	}
	if e != nil {
		return nil, e
	}
	if e = os.Rename(f.Name(), filepath.Join(dir, "submission.zip")); e != nil {
		return nil, e
	}
	if e = cardAccountWrite("draft.json", draft); e != nil {
		return nil, e
	}
	return draft, nil
}
func cardAccountSamePack(a, b designPack) bool {
	if len(a.files) != len(b.files) {
		return false
	}
	for name, data := range a.files {
		if !bytes.Equal(data, b.files[name]) {
			return false
		}
	}
	return true
}
func cardAccountOriginalInventory(raw []byte, p designPack) error {
	z, e := zip.NewReader(bytes.NewReader(raw), int64(len(raw)))
	if e != nil {
		return e
	}
	if len(z.File) != len(p.files) {
		return errors.New("Archive inventory changed")
	}
	seen := map[string]bool{}
	for _, f := range z.File {
		want, ok := p.files[f.Name]
		if !ok || seen[f.Name] || !f.Mode().IsRegular() || (f.Method != zip.Store && f.Method != zip.Deflate) {
			return errors.New("Archive inventory mismatch")
		}
		seen[f.Name] = true
		r, e := f.Open()
		if e != nil {
			return e
		}
		got, e := readBounded(r, int64(len(want)))
		ce := r.Close()
		if e != nil {
			return e
		}
		if ce != nil {
			return ce
		}
		if !bytes.Equal(got, want) {
			return errors.New("Archive bytes changed during inspection")
		}
	}
	return nil
}
func cardAccountSubmissionManifest(d cardAccountDraft, pub []byte, checklist map[string]bool, now time.Time) ([]byte, error) {
	if len(checklist) != 3 || !checklist["ownOrLicensed"] || !checklist["policyAccepted"] || !checklist["consentToReview"] {
		return nil, errors.New("Confirm rights, current policy and review separately")
	}
	nonce, e := base64.RawURLEncoding.DecodeString(d.Nonce)
	if e != nil || len(nonce) != 32 || base64.RawURLEncoding.EncodeToString(nonce) != d.Nonce {
		return nil, errors.New("Invalid submission nonce")
	}
	if !cardAccountPolicyVersionPattern.MatchString(d.Version) || !cardAccountPolicyVersionPattern.MatchString(d.AttestationVersion) || !cardAccountDigestPattern.MatchString(d.SHA) || (d.Kind != "design" && d.Kind != "hero") {
		return nil, errors.New("Invalid submission policy, digest or kind")
	}
	if d.Card != cardIDText(pub) || d.At > now.Unix()+30 || now.Unix()-d.At > 600 || d.Nonce == "" || len(d.Files) == 0 {
		return nil, errors.New("Submission preparation expired or device changed; inspect it again")
	}
	// protocol keys and strings are ASCII, with sorted map keys
	return cardAccountJSON(map[string]any{"v": 1, "cardId": d.Card, "device": base64.StdEncoding.EncodeToString(pub), "originalSha": d.SHA, "files": d.Files, "termsVersion": d.Version, "attestationVersion": d.AttestationVersion, "ownOrLicensed": true, "policyAccepted": true, "consentToReview": true, "nonce": d.Nonce, "submittedAt": d.At, "audience": d.Gate, "kind": d.Kind})
}
func cardAccountSubmit(ctx context.Context, expectedNonce, expectedSHA string, checklist map[string]bool) (any, error) {
	server, token, account, card, version, attestationVersion, e := cardAccountBound(ctx)
	if e != nil {
		return nil, e
	}
	gate, e := cardAccountGate()
	if e != nil {
		return nil, e
	}
	var d cardAccountDraft
	if e = cardAccountRead("draft.json", &d); e != nil {
		return nil, e
	}
	if d.Server != server || d.Gate != gate || d.Account != account || d.Card != card || d.Version != version || d.AttestationVersion != attestationVersion {
		return nil, errors.New("Submission account, device, Gate or consent changed; inspect it again")
	}
	if d.Nonce != expectedNonce || d.SHA != expectedSHA {
		return nil, errors.New("The displayed submission changed; inspect and confirm it again")
	}
	dir, e := cardAccountDir()
	if e != nil {
		return nil, e
	}
	raw, e := readBoundedFile(filepath.Join(dir, "submission.zip"), designZipLimit)
	if e != nil {
		return nil, e
	}
	if cardAccountHash(raw) != d.SHA {
		return nil, errors.New("Prepared archive changed; inspect it again")
	}
	if e = cardAccountCheckDraftArchive(raw, d); e != nil {
		return nil, e
	}
	seed, e := badgeDeviceSeed()
	if e != nil {
		return nil, e
	}
	defer clear(seed)
	payload, e := cardAccountSubmissionManifest(d, badgeDevicePublic(seed), checklist, time.Now())
	if e != nil {
		return nil, e
	}
	body, e := cardAccountJSON(map[string]any{"manifest": map[string]string{"payload": base64.StdEncoding.EncodeToString(payload), "sig": base64.StdEncoding.EncodeToString(signDomain(seed, "ryoku-submit-v1", payload))}})
	if e != nil {
		return nil, e
	}
	if e = cardAccountWriteBytes("submission.manifest.json", body); e != nil {
		return nil, e
	}
	var job cardAccountJob
	if e = cardAccountGateRequest(ctx, gate, token, "POST", "/v1/submissions", "application/json", body, &job); e != nil {
		return nil, e
	}
	if !cardAccountUUID.MatchString(job.ID) || job.Upload != "/v1/submissions/"+job.ID+"/archive" {
		return nil, errors.New("Gate returned an invalid reservation")
	}
	job.Gate = gate
	job.Account = account
	job.Card = card
	job.SHA = d.SHA
	if e = cardAccountWrite("submission.json", job); e != nil {
		return nil, e
	}
	if job.State != "uploading" {
		return job, nil
	}
	return cardAccountUpload(ctx, job, raw, token)
}
func cardAccountUpload(ctx context.Context, job cardAccountJob, raw []byte, token string) (any, error) {
	if cardAccountHash(raw) != job.SHA {
		return nil, errors.New("Reserved archive changed")
	}
	var reply struct {
		ID    string `json:"id"`
		State string `json:"state"`
	}
	if e := cardAccountGateRequest(ctx, job.Gate, token, "PUT", "/v1/submissions/"+job.ID+"/archive", "application/zip", raw, &reply); e != nil {
		return nil, fmt.Errorf("Job %s was reserved, but upload did not complete; use Retry upload or Cancel: %w", job.ID, e)
	}
	if reply.ID != "" && reply.ID != job.ID {
		return nil, errors.New("Gate upload job mismatch")
	}
	job.State = reply.State
	if e := cardAccountWrite("submission.json", job); e != nil {
		return nil, e
	}
	return job, nil
}
func cardAccountSubmission(ctx context.Context, args []string) (any, error) {
	unlock, lockErr := cardAccountNamedLock(ctx, "submission.lock")
	if lockErr != nil {
		return nil, lockErr
	}
	defer unlock()
	if len(args) == 0 {
		return nil, errors.New("submission needs prepare|submit|upload|status|cancel|select|clear")
	}
	if args[0] == "local" {
		var job cardAccountJob
		e := cardAccountRead("submission.json", &job)
		return job, e
	}
	if args[0] == "prepare" {
		if len(args) != 3 {
			return nil, errors.New("prepare needs design|hero and a local path")
		}
		return cardAccountPrepareSubmission(ctx, args[1], args[2])
	}
	if args[0] == "submit" {
		if len(args) != 6 || args[3] != "rights" || args[4] != "policy" || args[5] != "review" {
			return nil, errors.New("Submission requires displayed nonce, ZIP digest and explicit rights policy review confirmations")
		}
		return cardAccountSubmit(ctx, args[1], args[2], map[string]bool{"ownOrLicensed": true, "policyAccepted": true, "consentToReview": true})
	}
	if args[0] == "clear" {
		e := cardAccountRemove("publication.json")
		return map[string]bool{"cleared": e == nil}, e
	}
	server, token, e := cardAccountAccess(ctx)
	if e != nil {
		return nil, e
	}
	var identity cardAccountIdentity
	if e = cardAccountAPI(ctx, server, token, "GET", "/v1/account", nil, &identity); e != nil {
		return nil, e
	}
	gate, e := cardAccountGate()
	if e != nil {
		return nil, e
	}
	var job cardAccountJob
	if e = cardAccountRead("submission.json", &job); e != nil {
		return nil, e
	}
	if !cardAccountUUID.MatchString(job.ID) || job.Gate != gate || job.Account != identity.Account.ID {
		return nil, errors.New("Submission belongs to another Gate or Account")
	}
	if len(args) != 2 || args[1] != job.ID {
		return nil, errors.New("The displayed job changed; refresh its status before acting")
	}
	path := "/v1/submissions/" + job.ID
	switch args[0] {
	case "upload":
		dir, e := cardAccountDir()
		if e != nil {
			return nil, e
		}
		raw, e := readBoundedFile(filepath.Join(dir, "submission.zip"), designZipLimit)
		if e != nil {
			return nil, e
		}
		return cardAccountUpload(ctx, job, raw, token)
	case "status", "select":
		var current struct {
			ID      string `json:"id"`
			State   string `json:"state"`
			Account string `json:"accountId"`
			Card    string `json:"cardId"`
		}
		if e = cardAccountGateRequest(ctx, gate, token, "GET", path, "", nil, &current); e != nil {
			return nil, e
		}
		if current.ID != job.ID || current.Account != identity.Account.ID || current.Card != job.Card {
			return nil, errors.New("Gate job identity mismatch")
		}
		job.State = current.State
		if e = cardAccountWrite("submission.json", job); e != nil {
			return nil, e
		}
		if args[0] == "select" {
			if current.State != "approved" {
				return nil, errors.New("Only a genuinely approved publication can be selected")
			}
			if e = cardAnimationSelectVerify(ctx, gate, job.ID); e != nil {
				return nil, e
			}
			if e = cardAccountWrite("publication.json", cardAccountSelection{gate, job.ID}); e != nil {
				return nil, e
			}
		}
		return job, nil
	case "cancel":
		var result any
		if e = cardAccountGateRequest(ctx, gate, token, http.MethodDelete, path, "", nil, &result); e != nil {
			return nil, e
		}
		job.State = "cancelled"
		if e = cardAccountWrite("submission.json", job); e != nil {
			return nil, e
		}
		return job, nil
	default:
		return nil, errors.New("Unknown submission action")
	}
}

func cardAccountCheckDraftArchive(raw []byte, d cardAccountDraft) error {
	z, e := zip.NewReader(bytes.NewReader(raw), int64(len(raw)))
	if e != nil {
		return e
	}
	if len(z.File) != len(d.Files) || len(z.File) > 24 {
		return errors.New("Signed inventory does not match the archive")
	}
	expected := map[string]cardAccountFile{}
	last := ""
	var total int
	for _, f := range d.Files {
		if f.Name <= last || f.Bytes <= 0 || f.Bytes > int(designTotalLimit) {
			return errors.New("Invalid signed inventory")
		}
		last = f.Name
		expected[f.Name] = f
		total += f.Bytes
	}
	if int64(total) > designTotalLimit {
		return errors.New("Signed inventory exceeds expanded size limit")
	}
	seen := map[string]bool{}
	for _, f := range z.File {
		want, ok := expected[f.Name]
		if !ok || seen[f.Name] || !f.Mode().IsRegular() || f.UncompressedSize64 != uint64(want.Bytes) || (f.Method != zip.Store && f.Method != zip.Deflate) {
			return errors.New("Signed inventory entry mismatch")
		}
		seen[f.Name] = true
		r, e := f.Open()
		if e != nil {
			return e
		}
		data, e := readBounded(r, int64(want.Bytes))
		closeError := r.Close()
		if e != nil {
			return e
		}
		if closeError != nil {
			return closeError
		}
		if len(data) != want.Bytes || cardAccountHash(data) != want.SHA {
			return errors.New("Signed inventory digest mismatch")
		}
	}
	return nil
}
