package main

import (
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"
	"unicode"
)

var badgeLoginPattern = regexp.MustCompile(`^[A-Za-z0-9](?:[A-Za-z0-9-]{0,37}[A-Za-z0-9])?$`)
var badgeProgramPattern = regexp.MustCompile(`^[A-Za-z0-9](?:[A-Za-z0-9 .+-]{0,38}[A-Za-z0-9])?$`)
var badgeRoles = []string{"", "tester", "contributor", "nixos", "core", "founder"}
var badgeDomains = map[byte]string{2: "ryoku-badge-v2", 3: "ryoku-badge-v3"}

// every v2 tester badge was issued in the first tester round, which tested Hyperion
const badgeV2TesterProgram = "Ryoku v2 Hyperion"

type badgeInfo struct {
	Code    string `json:"code"`
	ID      string `json:"id"`
	Role    string `json:"role"`
	RoleID  byte   `json:"roleId"`
	Login   string `json:"login"`
	Issuer  string `json:"issuer"`
	Issued  string `json:"issued"`
	CardID  string `json:"cardId"`
	Device  string `json:"device"`
	Program string `json:"program,omitempty"`
	issuer  byte
	day     uint16
	device  []byte
}
type requestInfo struct {
	CardID string `json:"cardId"`
	Device string `json:"device"`
	device []byte
}
type revokeInfo struct {
	Code   string `json:"code"`
	Issuer string `json:"issuer"`
	Target string `json:"target"`
	Issued string `json:"issued"`
	issuer byte
}
type rydInfo struct {
	Code       string `json:"code"`
	Issuer     string `json:"issuer"`
	SHA        string `json:"sha"`
	Login      string `json:"login"`
	ApprovedAt string `json:"approvedAt"`
	approvedAt uint32
}

func badgeIssuerName(id byte) string {
	if id == 1 {
		return "owner"
	}
	if id == 2 {
		return "mika"
	}
	return ""
}
func badgeIssuerID(name string) byte {
	if name == "owner" {
		return 1
	}
	if name == "mika" {
		return 2
	}
	return 0
}
func badgeRoleAllowed(issuer, role byte) bool {
	return role >= 1 && role <= 5 && (issuer == 1 || (issuer == 2 && role <= 2))
}
func requestCode(pub []byte) string {
	b := append([]byte{2}, pub...)
	b = binary.BigEndian.AppendUint16(b, crc16(b))
	return codeText("RYQ", b)
}
func requestParse(text string) (requestInfo, error) {
	b, e := parseCodeText("RYQ", text)
	if e != nil {
		return requestInfo{}, e
	}
	if len(b) != 35 || b[0] != 2 || binary.BigEndian.Uint16(b[33:]) != crc16(b[:33]) {
		return requestInfo{}, errors.New("invalid badge request")
	}
	return requestInfo{CardID: cardIDText(b[1:33]), Device: base64.StdEncoding.EncodeToString(b[1:33]), device: b[1:33]}, nil
}
func badgeProgramValid(program string) bool {
	return badgeProgramPattern.MatchString(program) && !strings.Contains(program, "  ")
}
func badgeSign(issuer, role byte, seed []byte, req requestInfo, login, program string, day uint16) (string, error) {
	if !badgeRoleAllowed(issuer, role) || !badgeLoginPattern.MatchString(login) || len(req.device) != 32 || len(seed) != 32 || (program != "" && !badgeProgramValid(program)) {
		return "", errors.New("invalid badge fields or issuer role")
	}
	version := byte(2)
	if program != "" {
		version = 3
	}
	b := []byte{version, issuer, role}
	b = append(b, req.device...)
	b = binary.BigEndian.AppendUint16(b, day)
	b = append(b, byte(len(login)))
	b = append(b, login...)
	if program != "" {
		b = append(b, byte(len(program)))
		b = append(b, program...)
	}
	return codeText("RYB", append(b, signDomain(seed, badgeDomains[version], b)...)), nil
}
func badgeParse(text string) (badgeInfo, error) {
	b, e := parseCodeText("RYB", text)
	if e != nil {
		return badgeInfo{}, e
	}
	if len(b) < 103 || !badgeRoleAllowed(b[1], b[2]) {
		return badgeInfo{}, errors.New("invalid badge fields")
	}
	domain, known := badgeDomains[b[0]]
	body, loginEnd := b[:len(b)-64], 38+int(b[37])
	if !known || loginEnd > len(body) || !badgeLoginPattern.Match(b[38:loginEnd]) {
		return badgeInfo{}, errors.New("invalid badge fields")
	}
	program := ""
	if b[0] == 2 {
		if loginEnd != len(body) {
			return badgeInfo{}, errors.New("invalid badge fields")
		}
		if b[2] == 1 {
			program = badgeV2TesterProgram
		}
	} else {
		if loginEnd >= len(body) || loginEnd+1+int(body[loginEnd]) != len(body) || !badgeProgramValid(string(body[loginEnd+1:])) {
			return badgeInfo{}, errors.New("invalid badge fields")
		}
		program = string(body[loginEnd+1:])
	}
	pub, ok := issuerPublic(b[1])
	if !ok || !verifyDomain(pub, domain, body, b[len(b)-64:]) {
		return badgeInfo{}, errors.New("invalid badge signature")
	}
	sum := sha256.Sum256(b)
	day := binary.BigEndian.Uint16(b[35:37])
	return badgeInfo{Code: codeText("RYB", b), ID: hex.EncodeToString(sum[:8]), Role: badgeRoles[b[2]], RoleID: b[2], Login: string(b[38:loginEnd]), Issuer: badgeIssuerName(b[1]), Issued: badgeDate(day), CardID: cardIDText(b[3:35]), Device: base64.StdEncoding.EncodeToString(b[3:35]), Program: program, issuer: b[1], day: day, device: b[3:35]}, nil
}
func revokeSign(issuer byte, seed []byte, id string, day uint16) (string, error) {
	target, e := hex.DecodeString(id)
	if e != nil || len(target) != 8 || len(seed) != ed25519.SeedSize || badgeIssuerName(issuer) == "" {
		return "", errors.New("invalid revocation fields")
	}
	b := append([]byte{2, issuer}, target...)
	b = binary.BigEndian.AppendUint16(b, day)
	return codeText("RYX", append(b, signDomain(seed, "ryoku-revoke-v2", b)...)), nil
}
func revokeParse(text string) (revokeInfo, error) {
	b, e := parseCodeText("RYX", text)
	if e != nil {
		return revokeInfo{}, e
	}
	if len(b) != 76 || b[0] != 2 {
		return revokeInfo{}, errors.New("invalid revocation")
	}
	pub, ok := issuerPublic(b[1])
	if !ok || !verifyDomain(pub, "ryoku-revoke-v2", b[:12], b[12:]) {
		return revokeInfo{}, errors.New("invalid revocation signature")
	}
	return revokeInfo{Code: codeText("RYX", b), Issuer: badgeIssuerName(b[1]), Target: hex.EncodeToString(b[2:10]), Issued: badgeDate(binary.BigEndian.Uint16(b[10:12])), issuer: b[1]}, nil
}
func rydSign(issuer byte, seed []byte, sha [32]byte, login string, approvedAt uint32) (string, error) {
	if badgeIssuerName(issuer) == "" || len(seed) != 32 || !badgeLoginPattern.MatchString(login) {
		return "", errors.New("invalid approval fields")
	}
	b := append([]byte{2, issuer}, sha[:]...)
	b = binary.BigEndian.AppendUint32(b, approvedAt)
	b = append(b, byte(len(login)))
	b = append(b, login...)
	return codeText("RYD", append(b, signDomain(seed, "ryoku-design-v2", b)...)), nil
}
func rydParse(text string) (rydInfo, error) {
	b, e := parseCodeText("RYD", text)
	if e != nil {
		return rydInfo{}, e
	}
	if len(b) < 104 || b[0] != 2 || int(b[38])+103 != len(b) || !badgeLoginPattern.Match(b[39:len(b)-64]) {
		return rydInfo{}, errors.New("invalid approval fields")
	}
	pub, ok := issuerPublic(b[1])
	if !ok || !verifyDomain(pub, "ryoku-design-v2", b[:len(b)-64], b[len(b)-64:]) {
		return rydInfo{}, errors.New("invalid approval signature")
	}
	approvedAt := binary.BigEndian.Uint32(b[34:38])
	return rydInfo{Code: codeText("RYD", b), Issuer: badgeIssuerName(b[1]), SHA: hex.EncodeToString(b[2:34]), Login: string(b[39 : len(b)-64]), ApprovedAt: time.Unix(int64(approvedAt), 0).UTC().Format(time.RFC3339), approvedAt: approvedAt}, nil
}
func badgeInspect(text string) map[string]any {
	out := map[string]any{"kind": "unknown", "valid": false}
	var v any
	var e error
	for _, kind := range []struct{ p, n string }{{"RYQ", "request"}, {"RYB", "badge"}, {"RYX", "revoke"}, {"RYD", "design"}} {
		if !strings.HasPrefix(strings.Map(func(r rune) rune {
			if r == '-' || unicode.IsSpace(r) {
				return -1
			}
			return unicode.ToUpper(r)
		}, text), kind.p) {
			continue
		}
		out["kind"] = kind.n
		switch kind.p {
		case "RYQ":
			v, e = requestParse(text)
		case "RYB":
			v, e = badgeParse(text)
		case "RYX":
			v, e = revokeParse(text)
		case "RYD":
			v, e = rydParse(text)
		}
		if e != nil {
			out["error"] = e.Error()
			return out
		}
		out["valid"] = true
		switch x := v.(type) {
		case requestInfo:
			out["cardId"] = x.CardID
			out["device"] = x.Device
		case badgeInfo:
			out["id"] = x.ID
			out["role"] = x.Role
			out["roleId"] = x.RoleID
			out["login"] = x.Login
			out["issuer"] = x.Issuer
			out["issued"] = x.Issued
			out["cardId"] = x.CardID
			out["device"] = x.Device
			if x.Program != "" {
				out["program"] = x.Program
			}
		case revokeInfo:
			out["issuer"] = x.Issuer
			out["target"] = x.Target
			out["issued"] = x.Issued
		case rydInfo:
			out["issuer"] = x.Issuer
			out["sha"] = x.SHA
			out["login"] = x.Login
			out["approvedAt"] = x.ApprovedAt
		}
		return out
	}
	out["error"] = fmt.Sprint("unrecognised code prefix")
	return out
}
