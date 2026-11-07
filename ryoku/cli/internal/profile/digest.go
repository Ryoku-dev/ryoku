package profile

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"sort"
)

func (p *Plan) Pinned() Pinned {
	var out Pinned
	out.Profile.ID, out.Profile.Version, out.Profile.ManifestSHA256 = p.Profile.ID, p.Profile.Version, p.Profile.ManifestSHA256
	out.Account.Name, out.Account.Role, out.Account.Shell = p.Account.Name, p.Account.Role, p.Account.Shell
	out.Account.Permissions = append([]string{}, p.Account.Permissions...)
	out.Account.Groups = append([]string{}, p.Account.Groups...)
	for _, repo := range p.Repos {
		out.Repos = append(out.Repos, repo.ID)
	}
	for _, pkg := range p.Packages {
		if pkg.Target {
			out.Targets = append(out.Targets, pkg.Name)
		}
	}
	for _, pkg := range p.Remove {
		out.Removals = append(out.Removals, pkg.Name)
	}
	out.StoreItems = append([]StoreItem(nil), p.StoreItems...)
	if p.manifest != nil {
		out.Capabilities = append([]CapabilitySpec{}, p.manifest.Capabilities...)
		out.Wordlists.Links = append([]WordlistLink{}, p.manifest.Wordlists.Links...)
		out.Wordlists.Extract = append([]WordlistExtract{}, p.manifest.Wordlists.Extract...)
		out.Hosts.Suffixes = append([]string{}, p.manifest.Hosts.Suffixes...)
		out.Env = append([]EnvVar{}, p.manifest.Env...)
		out.Path = append([]string{}, p.manifest.Path...)
	}
	out.Files = append([]FileHash{}, p.source.Files...)
	return out
}

func (p Pinned) Digest() string {
	p = normalizePinned(p)
	body, err := canonicalJSON(p)
	if err != nil {
		return ""
	}
	sum := sha256.Sum256(body)
	return hex.EncodeToString(sum[:])
}

func normalizePinned(p Pinned) Pinned {
	p.Account.Permissions = append([]string{}, p.Account.Permissions...)
	p.Account.Groups = append([]string{}, p.Account.Groups...)
	p.Repos = append([]string{}, p.Repos...)
	p.Targets = append([]string{}, p.Targets...)
	p.Removals = append([]string{}, p.Removals...)
	p.StoreItems = append([]StoreItem(nil), p.StoreItems...)
	p.Capabilities = append([]CapabilitySpec{}, p.Capabilities...)
	p.Wordlists.Links = append([]WordlistLink{}, p.Wordlists.Links...)
	p.Wordlists.Extract = append([]WordlistExtract{}, p.Wordlists.Extract...)
	p.Hosts.Suffixes = append([]string{}, p.Hosts.Suffixes...)
	p.Env = append([]EnvVar{}, p.Env...)
	p.Path = append([]string{}, p.Path...)
	p.Files = append([]FileHash{}, p.Files...)
	sort.Strings(p.Account.Permissions)
	sort.Strings(p.Account.Groups)
	sort.Strings(p.Repos)
	sort.Strings(p.Targets)
	sort.Strings(p.Removals)
	sort.Slice(p.StoreItems, func(i, j int) bool {
		if p.StoreItems[i].Category != p.StoreItems[j].Category {
			return p.StoreItems[i].Category < p.StoreItems[j].Category
		}
		return p.StoreItems[i].ID < p.StoreItems[j].ID
	})
	sort.Slice(p.Capabilities, func(i, j int) bool {
		if p.Capabilities[i].Tool == p.Capabilities[j].Tool {
			return p.Capabilities[i].Caps < p.Capabilities[j].Caps
		}
		return p.Capabilities[i].Tool < p.Capabilities[j].Tool
	})
	sort.Slice(p.Wordlists.Links, func(i, j int) bool {
		if p.Wordlists.Links[i].Name == p.Wordlists.Links[j].Name {
			return p.Wordlists.Links[i].Target < p.Wordlists.Links[j].Target
		}
		return p.Wordlists.Links[i].Name < p.Wordlists.Links[j].Name
	})
	sort.Slice(p.Wordlists.Extract, func(i, j int) bool { return p.Wordlists.Extract[i].Name < p.Wordlists.Extract[j].Name })
	sort.Strings(p.Hosts.Suffixes)
	sort.Slice(p.Env, func(i, j int) bool {
		if p.Env[i].Key == p.Env[j].Key {
			return p.Env[i].Value < p.Env[j].Value
		}
		return p.Env[i].Key < p.Env[j].Key
	})
	sort.Slice(p.Files, func(i, j int) bool {
		if p.Files[i].Path == p.Files[j].Path {
			return p.Files[i].SHA256 < p.Files[j].SHA256
		}
		return p.Files[i].Path < p.Files[j].Path
	})
	return p
}

func digestJSON(value any) string {
	body, err := canonicalJSON(value)
	if err != nil {
		return ""
	}
	sum := sha256.Sum256(body)
	return hex.EncodeToString(sum[:])
}

func canonicalJSON(value any) ([]byte, error) {
	body, err := json.Marshal(value)
	if err != nil {
		return nil, err
	}
	dec := json.NewDecoder(bytes.NewReader(body))
	dec.UseNumber()
	var generic any
	if err := dec.Decode(&generic); err != nil {
		return nil, err
	}
	return json.Marshal(generic)
}
