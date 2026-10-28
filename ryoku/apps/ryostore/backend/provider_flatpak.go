package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strings"
	"time"
	"unicode/utf8"
)

const (
	flatpakCategory    = "flatpak"
	flatpakRemote      = "flathub"
	flathubRepoURL     = "https://dl.flathub.org/repo/"
	flathubAPI         = "https://flathub.org/api/v2"
	flathubPageSize    = 300
	flathubSearchHits  = 30
	flathubQueryMin    = 2
	flathubQueryMax    = 100
	flathubCache       = "flathub-popular.json"
	flatpakTimeout     = 30 * time.Minute
	flatpakUpdateProbe = 20 * time.Second
)

var flatpakIDPattern = regexp.MustCompile(`^[A-Za-z0-9_][A-Za-z0-9_-]*(\.[A-Za-z0-9_-]+){2,}$`)

func validFlatpakID(id string) bool { return len(id) <= 255 && flatpakIDPattern.MatchString(id) }

type flathubHit struct {
	AppID             string   `json:"app_id"`
	Name              string   `json:"name"`
	Summary           string   `json:"summary"`
	Description       string   `json:"description"`
	Type              string   `json:"type"`
	Icon              string   `json:"icon"`
	License           string   `json:"project_license"`
	Developer         string   `json:"developer_name"`
	MainCategories    any      `json:"main_categories"`
	SubCategories     any      `json:"sub_categories"`
	Verified          bool     `json:"verification_verified"`
	Arches            []string `json:"arches"`
	AddedAt           int64    `json:"added_at"`
	InstallsLastMonth int64    `json:"installs_last_month"`
}

type flathubCollection struct {
	Hits []flathubHit `json:"hits"`
}

type installedFlatpak struct {
	ID           string
	Name         string
	Version      string
	Installation string
	Origin       string
}

type flatpakProvider struct {
	api       string
	present   func() bool
	installed func(ctx context.Context) ([]installedFlatpak, error)
	updates   func(ctx context.Context, installations []string) map[string]bool
	run       func(ctx context.Context, args ...string) ([]byte, error)
}

func newFlatpakProvider() flatpakProvider {
	p := flatpakProvider{api: flathubAPI}
	p.present = func() bool { _, err := exec.LookPath("flatpak"); return err == nil }
	p.run = runFlatpak
	p.installed = func(ctx context.Context) ([]installedFlatpak, error) {
		out, err := runFlatpak(ctx, "list", "--app", "--columns=application:full,name:full,version:full,installation:full,origin:full")
		if err != nil {
			return nil, err
		}
		return parseFlatpakList(out), nil
	}
	p.updates = func(ctx context.Context, installations []string) map[string]bool {
		return pendingFlatpakUpdates(ctx, p.run, installations)
	}
	return p
}

func pendingFlatpakUpdates(ctx context.Context, run func(context.Context, ...string) ([]byte, error), installations []string) map[string]bool {
	pending := map[string]bool{}
	for _, installation := range installations {
		ctx, cancel := context.WithTimeout(ctx, flatpakUpdateProbe)
		out, err := run(ctx, "remote-ls", "--updates", "--app", "--columns=application:full,origin:full", installationFlag(installation))
		cancel()
		if err != nil {
			continue
		}
		for _, line := range strings.Split(string(out), "\n") {
			fields := strings.Split(line, "\t")
			if len(fields) == 2 && strings.TrimSpace(fields[1]) == flatpakRemote {
				if id := strings.TrimSpace(fields[0]); validFlatpakID(id) {
					pending[installation+"\x00"+id] = true
				}
			}
		}
	}
	return pending
}

func installationFlag(installation string) string {
	switch installation {
	case "user":
		return "--user"
	case "", "system":
		return "--system"
	}
	return "--installation=" + installation
}

func (flatpakProvider) Category() Category {
	return Category{
		ID:          flatpakCategory,
		Name:        "Apps",
		Group:       "EXTEND",
		Description: "Popular applications from Flathub, installed with Flatpak. Flathub is a third-party source: Ryostore does not review these.",
	}
}

func runFlatpak(ctx context.Context, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, "flatpak", args...)
	cmd.Env = append(os.Environ(), "LC_ALL=C")
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out
	err := cmd.Run()
	return out.Bytes(), err
}

func parseFlatpakList(out []byte) []installedFlatpak {
	var apps []installedFlatpak
	for _, line := range strings.Split(string(out), "\n") {
		f := strings.Split(line, "\t")
		if len(f) < 5 || !validFlatpakID(strings.TrimSpace(f[0])) {
			continue
		}
		apps = append(apps, installedFlatpak{
			ID:           strings.TrimSpace(f[0]),
			Name:         strings.TrimSpace(f[1]),
			Version:      strings.TrimSpace(f[2]),
			Installation: strings.TrimSpace(f[3]),
			Origin:       strings.TrimSpace(f[4]),
		})
	}
	return apps
}

func (p flatpakProvider) Load(ctx context.Context, refresh bool) ([]Item, SourceState, error) {
	hits, state, err := p.popular(ctx, refresh)
	var installed []installedFlatpak
	if p.present() && p.installed != nil {
		installed, _ = p.installed(ctx)
	}
	if err != nil && len(installed) == 0 {
		return nil, state, err
	}

	remoteReasons := p.remoteReasons(ctx, installed)
	owned := ownedFlathub(installed, remoteReasons)
	_, unavailable := newFlatpakInstallation(remoteReasons)
	if !p.present() {
		unavailable = "Flatpak is not installed on this system."
	}
	updatable := map[string]bool{}
	if p.present() && p.updates != nil && len(owned) > 0 {
		updatable = p.updates(ctx, installationsOf(owned))
	}
	items := flatpakItems(hits, owned, unavailable, updatable)
	seen := map[string]bool{}
	for _, item := range items {
		seen[item.ID] = true
	}
	ids := make([]string, 0, len(owned))
	for id := range owned {
		if !seen[id] {
			ids = append(ids, id)
		}
	}
	sort.Strings(ids)
	for _, id := range ids {
		item := flatpakInstalledItem(owned[id])
		item.UpdateAvailable = updatable[owned[id].Installation+"\x00"+id]
		items = append(items, item)
	}
	return items, state, nil
}

// When an ID exists in both installations, keep the user's existing copy.
func ownedFlathub(installed []installedFlatpak, remoteReasons map[string]string) map[string]installedFlatpak {
	owned := map[string]installedFlatpak{}
	for _, app := range installed {
		if app.Origin == flatpakRemote && remoteReasons[app.Installation] == "" &&
			(owned[app.ID].ID == "" || app.Installation == "user") {
			owned[app.ID] = app
		}
	}
	return owned
}

func (p flatpakProvider) remoteReasons(ctx context.Context, installed []installedFlatpak) map[string]string {
	reasons := map[string]string{}
	if !p.present() {
		return reasons
	}
	reasons["system"] = p.flathubRemoteReason(ctx, "system")
	if reasons["system"] != "" {
		reasons["user"] = p.flathubRemoteReason(ctx, "user")
	}
	for _, app := range installed {
		if app.Origin == flatpakRemote {
			if _, checked := reasons[app.Installation]; !checked {
				reasons[app.Installation] = p.flathubRemoteReason(ctx, app.Installation)
			}
		}
	}
	return reasons
}

func (p flatpakProvider) holdingFlathub(ctx context.Context, apps []installedFlatpak, id string) (installedFlatpak, string, bool) {
	var selected installedFlatpak
	for _, app := range apps {
		if app.ID == id && app.Origin == flatpakRemote && (selected.ID == "" || app.Installation == "user") {
			selected = app
		}
	}
	if selected.ID == "" {
		return installedFlatpak{}, "", false
	}
	reason := p.flathubRemoteReason(ctx, selected.Installation)
	if reason == "" {
		return selected, "", true
	}
	for _, app := range apps {
		if app.ID == id && app.Origin == flatpakRemote && app.Installation != selected.Installation &&
			p.flathubRemoteReason(ctx, app.Installation) == "" {
			return app, "", true
		}
	}
	return installedFlatpak{}, reason, true
}

func newFlatpakInstallation(reasons map[string]string) (string, string) {
	if reasons["system"] == "" {
		return "system", ""
	}
	if reason, checked := reasons["user"]; checked && reason == "" {
		return "user", ""
	}
	return "", reasons["system"]
}

func (p flatpakProvider) flathubRemoteReason(ctx context.Context, installation string) string {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	out, err := p.run(ctx, "remotes", "--columns=name,url", installationFlag(installation))
	if err != nil {
		return "Could not check the " + installation + " Flathub remote."
	}
	for _, line := range strings.Split(string(out), "\n") {
		fields := strings.Split(line, "\t")
		if len(fields) != 2 || strings.TrimSpace(fields[0]) != flatpakRemote {
			continue
		}
		if remoteURL := strings.TrimSpace(fields[1]); remoteURL == flathubRepoURL || remoteURL == strings.TrimSuffix(flathubRepoURL, "/") {
			return ""
		}
		return "The " + installation + " flathub remote does not point to the official Flathub repository."
	}
	if installation == "system" {
		return "The system Flathub remote is missing. Run ryoku doctor to set it up."
	}
	return "The " + installation + " Flathub remote is missing."
}

func flatpakItems(hits []flathubHit, owned map[string]installedFlatpak, unavailable string, updatable map[string]bool) []Item {
	arch := flathubArch()
	items := make([]Item, 0, len(hits))
	seen := map[string]bool{}
	for _, hit := range hits {
		if !validFlatpakID(hit.AppID) || seen[hit.AppID] || hit.Type != "desktop-application" || !hasArch(hit.Arches, arch) {
			continue
		}
		seen[hit.AppID] = true
		item := flatpakItem(hit, owned[hit.AppID], unavailable)
		item.UpdateAvailable = item.Installed && updatable[owned[hit.AppID].Installation+"\x00"+hit.AppID]
		items = append(items, item)
	}
	return items
}

func installationsOf(owned map[string]installedFlatpak) []string {
	set := map[string]bool{}
	for _, app := range owned {
		set[app.Installation] = true
	}
	names := make([]string, 0, len(set))
	for name := range set {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

func flathubArch() string {
	switch runtime.GOARCH {
	case "amd64":
		return "x86_64"
	case "arm64":
		return "aarch64"
	}
	return runtime.GOARCH
}

func hasArch(arches []string, arch string) bool {
	for _, a := range arches {
		if a == arch {
			return true
		}
	}
	return false
}

func (p flatpakProvider) popular(ctx context.Context, refresh bool) ([]flathubHit, SourceState, error) {
	file := filepath.Join(extrasCacheDir(), flathubCache)
	if !refresh {
		if info, err := os.Stat(file); err == nil && time.Since(info.ModTime()) < 6*time.Hour {
			if hits, err := decodeFlathub(mustRead(file)); err == nil {
				return hits, SourceState{}, nil
			}
		}
	}
	raw, err := p.fetchPopular(ctx)
	if err == nil {
		if hits, derr := decodeFlathub(raw); derr == nil {
			_ = atomicWrite(file, raw, 0o644)
			return hits, SourceState{}, nil
		} else {
			err = derr
		}
	}
	if archived := mustRead(file); len(archived) > 0 {
		if hits, derr := decodeFlathub(archived); derr == nil {
			state := SourceState{Offline: true}
			if info, serr := os.Stat(file); serr == nil {
				state.CachedAt = info.ModTime().UTC().Format(time.RFC3339)
			}
			return hits, state, nil
		}
	}
	return nil, SourceState{}, fmt.Errorf("flathub: %w", err)
}

func mustRead(path string) []byte {
	f, err := os.Open(path)
	if err != nil {
		return nil
	}
	defer f.Close()
	b, err := io.ReadAll(io.LimitReader(f, maxBody+1))
	if err != nil || len(b) > maxBody {
		return nil
	}
	return b
}

func decodeFlathub(raw []byte) ([]flathubHit, error) {
	var c flathubCollection
	if err := json.Unmarshal(raw, &c); err != nil {
		return nil, err
	}
	if len(c.Hits) == 0 {
		return nil, errors.New("empty collection")
	}
	return c.Hits, nil
}

func (p flatpakProvider) fetchPopular(ctx context.Context) ([]byte, error) {
	return p.flathub(ctx, http.MethodGet, fmt.Sprintf("/collection/popular?page=1&per_page=%d", flathubPageSize), nil)
}

func (p flatpakProvider) flathub(ctx context.Context, method, path string, payload any) ([]byte, error) {
	base, err := url.Parse(p.api)
	if err != nil {
		return nil, err
	}
	endpoint := strings.TrimRight(p.api, "/") + path
	ctx, cancel := context.WithTimeout(ctx, cacheTimeout)
	defer cancel()
	var body io.Reader
	if payload != nil {
		encoded, err := json.Marshal(payload)
		if err != nil {
			return nil, err
		}
		body = bytes.NewReader(encoded)
	}
	req, err := http.NewRequestWithContext(ctx, method, endpoint, body)
	if err != nil {
		return nil, err
	}
	if payload != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	client := http.Client{CheckRedirect: func(req *http.Request, via []*http.Request) error {
		if len(via) >= 10 || req.URL.Scheme != base.Scheme || req.URL.Host != base.Host {
			return errors.New("Flathub redirected outside its API origin")
		}
		return nil
	}}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%s: %s", endpoint, resp.Status)
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, maxBody+1))
	if err != nil {
		return nil, err
	}
	if len(data) > maxBody {
		return nil, fmt.Errorf("%s: response over %d bytes", endpoint, maxBody)
	}
	return data, nil
}

func flathubIcon(raw string) string {
	u, err := url.Parse(raw)
	if err != nil || u.Scheme != "https" || u.Host != "dl.flathub.org" || u.User != nil {
		return ""
	}
	return raw
}

func flatpakItem(hit flathubHit, owned installedFlatpak, unavailable string) Item {
	verification := "unverified"
	if hit.Verified {
		verification = "verified"
	}
	item := Item{
		ID:                 hit.AppID,
		Category:           flatpakCategory,
		Name:               firstNonEmpty(hit.Name, owned.Name, hit.AppID),
		Summary:            strings.TrimSpace(hit.Summary),
		Description:        strings.Join(strings.Fields(hit.Description), " "),
		Art:                flathubIcon(hit.Icon),
		Author:             hit.Developer,
		Upstream:           "https://flathub.org/apps/" + hit.AppID,
		License:            hit.License,
		Kind:               "flatpak",
		VerificationStatus: verification,
		Tags:               flathubTags(hit),
		Installed:          owned.ID != "",
		InstalledVersion:   owned.Version,
		Metadata: map[string]any{
			"provider":          "Flathub",
			"installsLastMonth": hit.InstallsLastMonth,
		},
	}
	if hit.AddedAt > 0 {
		item.ListedAt = time.Unix(hit.AddedAt, 0).UTC().Format(time.RFC3339)
	}
	if owned.Installation != "" {
		item.Metadata["installation"] = owned.Installation
	}
	if owned.ID == "" {
		markFlatpakUsable(&item, unavailable)
	}
	return item
}

func flatpakInstalledItem(app installedFlatpak) Item {
	item := Item{
		ID:                 app.ID,
		Category:           flatpakCategory,
		Name:               firstNonEmpty(app.Name, app.ID),
		Upstream:           "https://flathub.org/apps/" + app.ID,
		Kind:               "flatpak",
		VerificationStatus: "unknown",
		Installed:          true,
		InstalledVersion:   app.Version,
		Metadata:           map[string]any{"provider": "Flathub", "installation": app.Installation},
	}
	return item
}

func markFlatpakUsable(item *Item, reason string) {
	if reason == "" {
		return
	}
	item.Unavailable = true
	item.UnavailableEdition = true
	item.UnavailableReason = reason
}

func flathubTags(hit flathubHit) []string {
	var tags []string
	add := func(v any) {
		switch t := v.(type) {
		case string:
			if t != "" {
				tags = append(tags, strings.ToLower(t))
			}
		case []any:
			for _, e := range t {
				if s, ok := e.(string); ok && s != "" {
					tags = append(tags, strings.ToLower(s))
				}
			}
		}
	}
	add(hit.MainCategories)
	add(hit.SubCategories)
	return tags
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return strings.TrimSpace(v)
		}
	}
	return ""
}

func (p flatpakProvider) Install(ctx context.Context, id string) error {
	if !validFlatpakID(id) {
		return fmt.Errorf("invalid flatpak id %q", id)
	}
	if !p.present() {
		return errors.New("flatpak is not installed")
	}
	ctx, cancel := context.WithTimeout(ctx, flatpakTimeout)
	defer cancel()
	reportProgressNow(ctx, "apply", 0, 0)
	apps, err := p.installed(ctx)
	if err != nil {
		return flatpakError("list installed applications", nil, err)
	}
	if app, reason, found := p.holdingFlathub(ctx, apps, id); found {
		if reason != "" {
			return errors.New(reason)
		}
		if out, err := p.run(ctx, "update", "--noninteractive", "--assumeyes", installationFlag(app.Installation), id); err != nil {
			return flatpakError("update "+id, out, err)
		}
		return nil
	}
	installation, reason := newFlatpakInstallation(p.remoteReasons(ctx, nil))
	if reason != "" {
		return errors.New(reason)
	}
	if out, err := p.run(ctx, "install", installationFlag(installation), "--noninteractive", "--assumeyes", flatpakRemote, id); err != nil {
		return flatpakError("install "+id, out, err)
	}
	return nil
}

func (p flatpakProvider) Remove(ctx context.Context, id string) error {
	if !validFlatpakID(id) {
		return fmt.Errorf("invalid flatpak id %q", id)
	}
	if !p.present() {
		return errors.New("flatpak is not installed")
	}
	ctx, cancel := context.WithTimeout(ctx, flatpakTimeout)
	defer cancel()
	apps, err := p.installed(ctx)
	if err != nil {
		return flatpakError("list installed applications", nil, err)
	}
	app, reason, found := p.holdingFlathub(ctx, apps, id)
	if !found {
		return fmt.Errorf("%s is not installed from Flathub", id)
	}
	if reason != "" {
		return errors.New(reason)
	}
	if out, err := p.run(ctx, "uninstall", "--noninteractive", "--assumeyes", installationFlag(app.Installation), id); err != nil {
		return flatpakError("remove "+id, out, err)
	}
	return nil
}

func flatpakError(action string, out []byte, err error) error {
	detail := lastLine(out)
	if detail == "" {
		return fmt.Errorf("could not %s: %w", action, err)
	}
	return fmt.Errorf("could not %s: %s", action, detail)
}

func lastLine(out []byte) string {
	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	return strings.TrimSpace(lines[len(lines)-1])
}

func (p flatpakProvider) Search(ctx context.Context, query string) ([]Item, error) {
	query = strings.Join(strings.Fields(query), " ")
	if r := []rune(query); len(r) > flathubQueryMax {
		query = string(r[:flathubQueryMax])
	}
	if utf8.RuneCountInString(query) < flathubQueryMin {
		return []Item{}, nil
	}
	raw, err := p.flathub(ctx, http.MethodPost, "/search", map[string]any{
		"query": query, "page": 1, "hits_per_page": flathubSearchHits,
	})
	if err != nil {
		return nil, fmt.Errorf("flathub search: %w", err)
	}
	var found flathubCollection
	if err := json.Unmarshal(raw, &found); err != nil {
		return nil, fmt.Errorf("flathub search: %w", err)
	}
	unavailable := ""
	var installed []installedFlatpak
	if p.present() && p.installed != nil {
		installed, _ = p.installed(ctx)
	}
	reasons := p.remoteReasons(ctx, installed)
	_, unavailable = newFlatpakInstallation(reasons)
	if !p.present() {
		unavailable = "Flatpak is not installed on this system."
	}
	return flatpakItems(found.Hits, ownedFlathub(installed, reasons), unavailable, nil), nil
}
