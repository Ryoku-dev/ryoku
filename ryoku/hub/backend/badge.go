package main

import (
	"encoding/base64"
	"errors"
	"flag"
	"fmt"
	"io"
	"strings"
	"time"
)

type badgeView struct {
	Code   string `json:"code,omitempty"`
	ID     string `json:"id"`
	Role   string `json:"role"`
	RoleID byte   `json:"roleId"`
	Login  string `json:"login"`
	Issuer string `json:"issuer"`
	Issued string `json:"issued"`
}

func badgeViewFor(b badgeInfo, withCode bool) *badgeView {
	out := &badgeView{ID: b.ID, Role: b.Role, RoleID: b.RoleID, Login: b.Login, Issuer: b.Issuer, Issued: b.Issued}
	if withCode {
		out.Code = b.Code
	}
	return out
}

type badgeCardDesign struct {
	SHA  string     `json:"sha"`
	Dir  string     `json:"dir"`
	Name string     `json:"name"`
	Spec designSpec `json:"spec"`
}
type badgeDesign struct {
	SHA      string     `json:"sha"`
	Dir      string     `json:"dir"`
	Name     string     `json:"name"`
	Approved bool       `json:"approved"`
	Spec     designSpec `json:"spec"`
}
type badgeStatusResult struct {
	Device    string       `json:"device"`
	CardID    string       `json:"cardId"`
	Request   string       `json:"request"`
	Badge     *badgeView   `json:"badge"`
	Published bool         `json:"published"`
	Owner     bool         `json:"owner"`
	Design    *badgeDesign `json:"design"`
}

func badgeDesignFor(list teamVerifiedList, b badgeInfo) *badgeDesign {
	if b.RoleID < 3 {
		return nil
	}
	approval, ok := teamApproval(list, b.Login)
	if !ok {
		return nil
	}
	pack, e := designFetch(approval.SHA, false)
	if e != nil {
		return nil
	}
	return &badgeDesign{SHA: approval.SHA, Dir: pack.Dir, Name: pack.Design.Name, Approved: true, Spec: pack.Design}
}

// badgeStatus reports this machine's badge state; refresh re-reads the team list instead of trusting a cached copy
// up to ten minutes old, so an owner approval made a moment ago shows on the next look.
func badgeStatus(refresh bool) (badgeStatusResult, error) {
	var out badgeStatusResult
	seed, e := badgeDeviceSeed()
	if e != nil {
		return out, e
	}
	defer clear(seed)
	pub := badgeDevicePublic(seed)
	out.Device = base64.StdEncoding.EncodeToString(pub)
	out.CardID = cardIDText(pub)
	out.Request = requestCode(pub)
	ownerSeed, e := loadIssuerSeed("owner", "")
	defer clear(ownerSeed)
	out.Owner = e == nil
	if b, e := badgeActive(); e == nil {
		out.Badge = badgeViewFor(b, true)
		if list, e := teamList(refresh); e == nil {
			for _, known := range list.Badges {
				if known.ID == b.ID {
					out.Published = true
					out.Design = badgeDesignFor(list, b)
					break
				}
			}
			if !out.Published {
				for _, revoked := range list.Revoked {
					if revoked.Target == b.ID {
						out.Badge = nil
					}
				}
			}
		}
	}
	return out, nil
}
func badgeIssuerFlags(name string, args []string) (*flag.FlagSet, string, string, error) {
	f := flag.NewFlagSet(name, flag.ContinueOnError)
	f.SetOutput(io.Discard)
	issuer := f.String("issuer", "", "")
	key := f.String("key-file", "", "")
	if e := f.Parse(args); e != nil {
		return f, "", "", e
	}
	if badgeIssuerID(*issuer) == 0 {
		return f, "", "", errors.New("--issuer must be owner or mika")
	}
	return f, *issuer, *key, nil
}
func badgeIssue(args []string) error {
	f := flag.NewFlagSet("badge issue", flag.ContinueOnError)
	f.SetOutput(io.Discard)
	issuer := f.String("issuer", "", "")
	key := f.String("key-file", "", "")
	role := f.String("role", "", "")
	login := f.String("login", "", "")
	request := f.String("request", "", "")
	if e := f.Parse(args); e != nil {
		return e
	}
	if f.NArg() != 0 {
		return errors.New("badge issue accepts flags only")
	}
	id := badgeIssuerID(*issuer)
	var roleID byte
	for i, r := range badgeRoles {
		if r == *role {
			roleID = byte(i)
		}
	}
	if !badgeRoleAllowed(id, roleID) {
		return errors.New("issuer cannot issue this role")
	}
	req, e := requestParse(*request)
	if e != nil {
		return e
	}
	seed, e := loadIssuerSeed(*issuer, *key)
	if e != nil {
		return e
	}
	defer clear(seed)
	code, e := badgeSign(id, roleID, seed, req, *login, badgeDay(time.Now()))
	if e != nil {
		return e
	}
	info, e := badgeParse(code)
	if e != nil {
		return e
	}
	return printJSON(map[string]any{"code": code, "id": info.ID, "published": teamPublish("/v1/badges", code)})
}
func badgeRevoke(args []string) error {
	f, issuer, key, e := badgeIssuerFlags("badge revoke", args)
	if e != nil {
		return e
	}
	if f.NArg() != 1 {
		return errors.New("badge revoke needs a badge code or id")
	}
	target := f.Arg(0)
	b, e := badgeParse(target)
	if e != nil {
		list, err := teamList(true)
		if err != nil {
			return err
		}
		found := false
		for _, candidate := range list.Badges {
			if candidate.ID == target {
				b = candidate
				found = true
				break
			}
		}
		if !found {
			return errors.New("badge id not found in verified team list")
		}
	}
	id := badgeIssuerID(issuer)
	if id != 1 && id != b.issuer {
		return errors.New("Mika may only revoke Mika-issued badges")
	}
	seed, e := loadIssuerSeed(issuer, key)
	if e != nil {
		return e
	}
	defer clear(seed)
	code, e := revokeSign(id, seed, b.ID, badgeDay(time.Now()))
	if e != nil {
		return e
	}
	published := teamPublish("/v1/revoke", code)
	if published {
		_, _ = teamList(true)
	}
	return printJSON(map[string]any{"code": code, "published": published})
}
func badgeCommand(args []string) error {
	if len(args) == 0 {
		return errors.New("badge needs status|activate|sync|issue|revoke|inspect|holder|list")
	}
	switch args[0] {
	case "status":
		if len(args) > 2 || (len(args) == 2 && args[1] != "--refresh") {
			return errors.New("badge status takes only --refresh")
		}
		out, e := badgeStatus(len(args) == 2)
		if e != nil {
			return e
		}
		return printJSON(out)
	case "activate":
		if len(args) != 2 {
			return errors.New("badge activate needs one badge code")
		}
		if e := badgeActivate(args[1]); e != nil {
			return e
		}
		b, e := badgeActive()
		if e != nil {
			return e
		}
		if teamPublish("/v1/badges", b.Code) {
			_, _ = teamList(true)
		}
		out, e := badgeStatus(false)
		if e != nil {
			return e
		}
		return printJSON(out)
	case "sync":
		published, e := recordSync(args[1:])
		if e != nil {
			return e
		}
		return printJSON(map[string]bool{"published": published})
	case "issue":
		return badgeIssue(args[1:])
	case "revoke":
		return badgeRevoke(args[1:])
	case "inspect":
		if len(args) != 2 {
			return errors.New("badge inspect needs one code")
		}
		return printJSON(badgeInspect(args[1]))
	case "holder":
		if len(args) != 2 || !badgeLoginPattern.MatchString(args[1]) {
			return errors.New("badge holder needs a GitHub login")
		}
		// Mika asks this before accepting a team design, often minutes after the badge was
		// activated, so it reads the current list rather than a cached copy
		list, e := teamList(true)
		if e != nil {
			return e
		}
		return printJSON(teamHolder(list, args[1]))
	case "list":
		refresh := len(args) == 2 && args[1] == "--refresh"
		if len(args) != 1 && !refresh {
			return errors.New("badge list accepts only --refresh")
		}
		list, e := teamList(refresh)
		if e != nil {
			return e
		}
		return printJSON(list)
	default:
		return fmt.Errorf("unknown badge command: %s", strings.TrimSpace(args[0]))
	}
}
