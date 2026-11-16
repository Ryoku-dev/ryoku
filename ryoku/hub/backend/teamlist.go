package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

type teamWireList struct {
	V       int      `json:"v"`
	Badges  []string `json:"badges"`
	Revoked []string `json:"revoked"`
	Designs []string `json:"designs"`
}
type teamVerifiedList struct {
	V       int          `json:"v"`
	Badges  []badgeInfo  `json:"badges"`
	Revoked []revokeInfo `json:"revoked"`
	Designs []rydInfo    `json:"designs"`
}

func teamVerifyList(raw []byte) (teamVerifiedList, error) {
	var wire teamWireList
	if e := json.Unmarshal(raw, &wire); e != nil {
		return teamVerifiedList{}, e
	}
	if wire.V != 1 {
		return teamVerifiedList{}, errors.New("unknown team list version")
	}
	return teamVerifyWire(wire), nil
}

// teamVerifyWire keeps the codes that verify: badges not revoked, and each team login's newest
// approved design
func teamVerifyWire(wire teamWireList) teamVerifiedList {
	out := teamVerifiedList{V: 1, Badges: []badgeInfo{}, Revoked: []revokeInfo{}, Designs: []rydInfo{}}
	byID := map[string]badgeInfo{}
	for _, text := range wire.Badges {
		b, e := badgeParse(text)
		if e == nil {
			byID[b.ID] = b
		}
	}
	revoked := map[string]bool{}
	for _, text := range wire.Revoked {
		r, e := revokeParse(text)
		if e != nil {
			continue
		}
		b, ok := byID[r.Target]
		if ok && (r.issuer == 1 || r.issuer == b.issuer) {
			revoked[r.Target] = true
			out.Revoked = append(out.Revoked, r)
		}
	}
	teamLogins := map[string]bool{}
	for _, b := range byID {
		if !revoked[b.ID] {
			out.Badges = append(out.Badges, b)
			if b.RoleID >= 3 {
				teamLogins[strings.ToLower(b.Login)] = true
			}
		}
	}
	sort.Slice(out.Badges, func(i, j int) bool { return out.Badges[i].ID < out.Badges[j].ID })
	newest := map[string]rydInfo{}
	order := []string{}
	for _, text := range wire.Designs {
		d, e := rydParse(text)
		key := strings.ToLower(d.Login)
		if e != nil || !teamLogins[key] {
			continue
		}
		old, exists := newest[key]
		if !exists {
			order = append(order, key)
		}
		if !exists || d.approvedAt > old.approvedAt {
			newest[key] = d
		}
	}
	for _, key := range order {
		out.Designs = append(out.Designs, newest[key])
	}
	return out
}
func teamList(refresh bool) (teamVerifiedList, error) {
	dir, e := teamCacheDir()
	if e != nil {
		return teamVerifiedList{}, e
	}
	path := filepath.Join(dir, "list.json")
	if !refresh {
		if st, e := os.Stat(path); e == nil && time.Since(st.ModTime()) < 10*time.Minute {
			raw, e := readBoundedFile(path, 2<<20)
			if e == nil {
				if list, e := teamVerifyList(raw); e == nil {
					return list, nil
				}
			}
		}
	}
	raw, status, e := teamGet("/v1/list", 2<<20)
	if e != nil {
		return teamVerifiedList{}, e
	}
	if status != http.StatusOK {
		return teamVerifiedList{}, fmt.Errorf("team list returned HTTP %d", status)
	}
	list, e := teamVerifyList(raw)
	if e != nil {
		return list, e
	}
	if e = atomicWrite(path, raw, 0600); e != nil {
		return list, e
	}
	return list, nil
}
func teamBadgeForCardID(list teamVerifiedList, id string) (badgeInfo, bool) {
	var best badgeInfo
	ok := false
	for _, b := range list.Badges {
		if b.CardID == id && (!ok || b.RoleID > best.RoleID || (b.RoleID == best.RoleID && b.day > best.day)) {
			best = b
			ok = true
		}
	}
	return best, ok
}
func teamApproval(list teamVerifiedList, login string) (rydInfo, bool) {
	for _, d := range list.Designs {
		if strings.EqualFold(d.Login, login) {
			return d, true
		}
	}
	return rydInfo{}, false
}
func teamPublish(path, code string) bool {
	_, status, e := teamSend(http.MethodPost, path, "text/plain", []byte(code), nil, 4096)
	return e == nil && status == http.StatusOK
}
func teamHolder(list teamVerifiedList, login string) map[string]any {
	roles := []string{}
	ids := []int{}
	cardIDs := []string{}
	seenRole := map[byte]bool{}
	seenCardID := map[string]bool{}
	badges := append([]badgeInfo(nil), list.Badges...)
	sort.Slice(badges, func(i, j int) bool { return badges[i].RoleID > badges[j].RoleID })
	for _, b := range badges {
		if !strings.EqualFold(b.Login, login) {
			continue
		}
		if !seenRole[b.RoleID] {
			roles = append(roles, b.Role)
			ids = append(ids, int(b.RoleID))
			seenRole[b.RoleID] = true
		}
		if !seenCardID[b.CardID] {
			cardIDs = append(cardIDs, b.CardID)
			seenCardID[b.CardID] = true
		}
	}
	sort.Strings(cardIDs)
	return map[string]any{"login": login, "roles": roles, "roleIds": ids, "cardIds": cardIDs}
}
