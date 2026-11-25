package localmodels

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	i18n "ryoku-i18n"
	"ryoku-rashin/internal/core"
	"ryoku-rashin/internal/protocol"
)

type job struct {
	cancel context.CancelFunc
	done   chan struct{}
}

// Manager owns the downloaded models and one on-demand llama-server router.
// New does not start a process, open a network connection or create a file.
type Manager struct {
	mu        sync.Mutex
	adoptMu   sync.Mutex
	deviceMu  sync.Mutex
	settings  func() protocol.Settings
	broadcast core.Broadcaster
	change    func()
	client    *http.Client
	hfBase    string
	bins      map[string]binary
	discovery discoveryOptions
	badAdopts map[string]failedAdopt
	failures  map[string]string
	lastStart string
	active    string
	devices   []protocol.LocalDevice
	deviceKey string
	jobs      map[string]*job
	limit     chan struct{}
	server    *server
	starting  chan struct{}
	stopping  chan struct{}
	switching chan struct{}
	idle      *time.Timer
	leases    int
	closed    bool
}

var _ core.Local = (*Manager)(nil)

func New(settings func() protocol.Settings, broadcast core.Broadcaster) *Manager {
	return &Manager{settings: settings, broadcast: broadcast, client: newClient(), hfBase: huggingFace,
		bins: make(map[string]binary), failures: make(map[string]string), badAdopts: make(map[string]failedAdopt), jobs: make(map[string]*job), limit: make(chan struct{}, 2), discovery: defaultDiscoveryOptions()}
}

func (m *Manager) entries() []Entry {
	all := Catalogue()
	return append(all, readCustom()...)
}

func (m *Manager) entry(id string) (Entry, bool) {
	for _, e := range m.entries() {
		if e.ID == id {
			return e, true
		}
	}
	return Entry{}, false
}

// OnChange registers the gateway's provider refresh hook. It is called
// outside the manager lock so the gateway can read Inventory immediately.
func (m *Manager) OnChange(fn func()) {
	m.mu.Lock()
	m.change = fn
	m.mu.Unlock()
}

func (m *Manager) notify() {
	if m.broadcast != nil {
		m.broadcast.Broadcast(protocol.EvtLocalChanged, m.Models(context.Background()))
	}
	m.mu.Lock()
	fn := m.change
	m.mu.Unlock()
	if fn != nil {
		fn()
	}
}

// Inventory reads file metadata and manifests without hashing model weights.
func (m *Manager) Inventory() []protocol.LocalModel {
	var models []protocol.LocalModel
	for _, e := range m.entries() {
		model, _ := onDisk(e)
		if model.State == protocol.LocalInstalled {
			models = append(models, model)
		}
	}
	return models
}

type failedAdopt struct {
	stamp string
	err   error
}

func adoptStamp(e Entry) (string, bool) {
	var b strings.Builder
	b.WriteString(e.Revision)
	for _, f := range e.Files {
		info, err := os.Stat(filePath(e, f.Name))
		if err != nil || !info.Mode().IsRegular() || info.Size() != f.Size {
			return "", false
		}
		fmt.Fprintf(&b, "|%s:%s:%d:%d", f.Name, f.SHA256, info.Size(), info.ModTime().UnixNano())
	}
	return b.String(), true
}

func (m *Manager) modelOnDisk(e Entry) protocol.LocalModel {
	model, adopt := onDisk(e)
	if !adopt {
		return model
	}
	m.adoptMu.Lock()
	defer m.adoptMu.Unlock()
	model, adopt = onDisk(e)
	if !adopt {
		return model
	}
	stamp, ok := adoptStamp(e)
	if !ok {
		return model
	}
	failed := m.badAdopts[e.ID]
	if failed.stamp != stamp || failed.err == nil {
		err := m.verifyExisting(e)
		if err == nil {
			delete(m.badAdopts, e.ID)
			model, _ = onDisk(e)
			return model
		}
		m.badAdopts[e.ID] = failedAdopt{stamp: stamp, err: err}
		failed = m.badAdopts[e.ID]
	}
	model.State = protocol.LocalBroken
	model.Error = failed.err.Error()
	return model
}

func (m *Manager) verifyExisting(e Entry) error {
	for _, f := range e.Files {
		sum, err := hashFile(context.Background(), filePath(e, f.Name), func(int64) {})
		if err != nil {
			return err
		}
		if sum != f.SHA256 {
			return errors.New(i18n.Tf("%s does not match its checksum (expected %s, got %s).", f.Name, f.SHA256, sum))
		}
	}
	header, _ := localGGUF(e)
	return writeManifest(e, manifest{ID: e.ID, Repo: e.Repo, Revision: e.Revision, Files: e.Files, Verified: time.Now().UTC(), Header: &header})
}

func (m *Manager) Models(ctx context.Context) protocol.LocalResult {
	all := m.entries()
	models := make([]protocol.LocalModel, 0, len(all))
	for _, e := range all {
		model := m.modelOnDisk(e)
		m.mu.Lock()
		downloading := m.jobs[e.ID] != nil
		if model.State == protocol.LocalInstalled {
			model.Error = m.failures[e.ID]
		}
		m.mu.Unlock()
		if downloading {
			model.State = protocol.LocalDownloading
		}
		models = append(models, model)
	}
	resolved := m.resolve(ctx)
	if resolved.State == protocol.RuntimeStopped {
		m.probeDevices(ctx, resolved.Bin)
	}
	m.mu.Lock()
	s := m.server
	starting := m.starting != nil
	stopping := m.stopping != nil
	lastStart, active := m.lastStart, m.active
	devices := append([]protocol.LocalDevice(nil), m.devices...)
	m.mu.Unlock()
	h := hostHardware(devices)
	for i, e := range all {
		models[i].Plan = m.plan(e, h)
	}
	res := protocol.LocalResult{Models: models, DiskBytes: diskBytes()}
	res.Runtime = m.runtimeInfo(resolved, s, starting, stopping)
	for i := range res.Models {
		if plan, ok := res.Runtime.Plans[res.Models[i].ID]; ok {
			res.Models[i].Plan = plan
		}
	}
	res.Runtime.ActiveModel = active
	if s == nil && !starting && !stopping && lastStart != "" {
		res.Runtime.State, res.Runtime.Error = protocol.RuntimeFailed, lastStart
	}
	return res
}

// RuntimeState reads the router without probing the runtime or model files.
func (m *Manager) RuntimeState() (bool, string) {
	m.mu.Lock()
	s := m.server
	m.mu.Unlock()
	if s == nil {
		return false, ""
	}
	select {
	case <-s.exited:
		return false, ""
	default:
		return true, s.base
	}
}

// Devices probes the runtime only when the local models UI requests devices.
func (m *Manager) Devices(ctx context.Context) []protocol.LocalDevice {
	res := m.resolve(ctx)
	if res.State != protocol.RuntimeStopped {
		return nil
	}
	return m.probeDevices(ctx, res.Bin)
}

func (m *Manager) probeDevices(ctx context.Context, bin binary) []protocol.LocalDevice {
	key, ok := probeKey(bin.Path)
	if !ok {
		return nil
	}
	m.deviceMu.Lock()
	defer m.deviceMu.Unlock()
	m.mu.Lock()
	if key == m.deviceKey {
		devices := append([]protocol.LocalDevice(nil), m.devices...)
		m.mu.Unlock()
		return devices
	}
	m.mu.Unlock()
	out, err := runQuick(ctx, bin.Path, 5*time.Second, "--list-devices")
	if err != nil || ctx.Err() != nil {
		return nil
	}
	devices := parseDevices(out)
	h := hostHardware(devices)
	for i := range devices {
		devices[i].Kind = h.Devices[i].Kind
	}
	m.mu.Lock()
	m.deviceKey = key
	m.devices = devices
	m.mu.Unlock()
	return append([]protocol.LocalDevice(nil), devices...)
}

func (m *Manager) progress(p protocol.LocalProgress) {
	if m.broadcast != nil {
		m.broadcast.Broadcast(protocol.EvtLocalProgress, p)
	}
}

func (m *Manager) Download(ctx context.Context, id string) error {
	e, ok := m.entry(id)
	if !ok {
		return protocol.Errorf(protocol.CodeNotFound, "%s", i18n.T("That model is not in Hyakki's catalogue."))
	}
	if e.External != "" {
		return protocol.Errorf(protocol.CodeBadRequest, "%s", i18n.T("A linked model cannot be downloaded. Find its original file again."))
	}
	if strings.HasPrefix(e.Repo, "local/") {
		return protocol.Errorf(protocol.CodeBadRequest, "%s", i18n.T("Re-import this local model from its original file."))
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	m.mu.Lock()
	if m.closed {
		m.mu.Unlock()
		return protocol.Errorf(protocol.CodeUnavailable, "%s", i18n.T("Hyakki is shutting down."))
	}
	if m.jobs[id] != nil {
		m.mu.Unlock()
		return nil
	}
	for otherID := range m.jobs {
		if other, ok := m.entry(otherID); ok && other.Repo == e.Repo {
			m.mu.Unlock()
			return protocol.Errorf(protocol.CodeConflict, "%s", i18n.T("Another model from this repository is still downloading."))
		}
	}
	jobCtx, cancel := context.WithCancel(context.Background())
	j := &job{cancel: cancel, done: make(chan struct{})}
	m.jobs[id] = j
	m.mu.Unlock()
	go m.downloadJob(jobCtx, id, e, j)
	m.notify()
	return nil
}

func (m *Manager) downloadJob(ctx context.Context, id string, e Entry, j *job) {
	var err error
	select {
	case m.limit <- struct{}{}:
		defer func() { <-m.limit }()
	case <-ctx.Done():
		err = ctx.Err()
	}
	if err == nil {
		err = m.downloadEntry(ctx, e)
	}
	if err == nil {
		m.refreshRunningPresets(ctx)
	}
	m.mu.Lock()
	delete(m.jobs, id)
	close(j.done)
	m.mu.Unlock()
	phase := protocol.LocalPhaseDone
	if err != nil {
		phase = protocol.LocalPhaseError
		if errors.Is(err, context.Canceled) {
			phase = protocol.LocalPhaseCancelled
		}
	}
	p := protocol.LocalProgress{ID: id, Phase: phase}
	if err != nil && phase != protocol.LocalPhaseCancelled {
		p.Error = err.Error()
	}
	m.progress(p)
	m.notify()
}

func (m *Manager) downloadEntry(ctx context.Context, e Entry) error {
	var remaining int64
	for _, f := range e.Files {
		state := statFile(e, f)
		if !state.complete {
			remaining += f.Size - state.done
		}
	}
	if err := checkSpace(repoDir(e.Repo), remaining); err != nil {
		return err
	}
	for _, f := range e.Files {
		last := time.Time{}
		lastPercent := int64(-1)
		progress := func(phase string, n int64) {
			percent := n * 100 / f.Size
			if n == f.Size || time.Since(last) >= 250*time.Millisecond || percent > lastPercent {
				last, lastPercent = time.Now(), percent
				m.progress(protocol.LocalProgress{ID: e.ID, File: f.Name, Done: n, Total: f.Size, Phase: phase})
			}
		}
		_, err := fetch(ctx, m.client, fetchRequest{URL: e.url(m.hfBase, f.Name), Dest: filePath(e, f.Name), Size: f.Size, SHA256: f.SHA256}, progress)
		if err != nil {
			return err
		}
	}
	if err := m.downloadLicense(ctx, e); err != nil {
		return err
	}
	header, _ := localGGUF(e)
	if header.Params == 0 {
		header.Params = e.Params
	}
	if header.Context == 0 {
		header.Context = e.Context
	}
	if e.Catalogue {
		header.Tools = e.Tools
	}
	return writeManifest(e, manifest{ID: e.ID, Repo: e.Repo, Revision: e.Revision, Files: e.Files, Verified: time.Now().UTC(), Header: &header})
}

func (m *Manager) downloadLicense(ctx context.Context, e Entry) error {
	if _, err := os.Stat(filepath.Join(repoDir(e.Repo), "LICENSE")); err == nil {
		return nil
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, e.url(m.hfBase, "LICENSE"), nil)
	if err != nil {
		return err
	}
	response, err := m.client.Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		// Not all custom repos carry a separate LICENSE file.
		if !e.Catalogue && response.StatusCode == http.StatusNotFound {
			return nil
		}
		return protocol.Errorf(protocol.CodeUnavailable, "%s", i18n.Tf("Could not get the model licence: %s.", response.Status))
	}
	data, err := ioReadLimit(response.Body, 1<<20)
	if err != nil {
		return err
	}
	return writeAtomic(filepath.Join(repoDir(e.Repo), "LICENSE"), data, 0o600)
}

func (m *Manager) Cancel(id string) error {
	m.mu.Lock()
	j := m.jobs[id]
	m.mu.Unlock()
	if j == nil {
		return protocol.Errorf(protocol.CodeNotFound, "%s", i18n.T("That model has no download in progress."))
	}
	j.cancel()
	m.notify()
	return nil
}

func (m *Manager) Remove(ctx context.Context, id string) (protocol.LocalResult, error) {
	e, ok := m.entry(id)
	if !ok {
		return protocol.LocalResult{}, protocol.Errorf(protocol.CodeNotFound, "%s", i18n.T("That model is not in Hyakki's catalogue."))
	}
	m.mu.Lock()
	j := m.jobs[id]
	m.mu.Unlock()
	if j != nil {
		j.cancel()
		select {
		case <-j.done:
		case <-ctx.Done():
			return protocol.LocalResult{}, ctx.Err()
		}
	}
	finishSwap, err := m.beginRuntimeSwap(ctx)
	if err != nil {
		return protocol.LocalResult{}, err
	}
	defer finishSwap()
	m.Stop()
	m.mu.Lock()
	all := m.entries()
	if err := removeFiles(e, all); err != nil {
		m.mu.Unlock()
		return protocol.LocalResult{}, err
	}
	if !e.Catalogue {
		custom := readCustom()
		kept := custom[:0]
		for _, item := range custom {
			if item.ID != id {
				kept = append(kept, item)
			}
		}
		if err := writeCustom(kept); err != nil {
			m.mu.Unlock()
			return protocol.LocalResult{}, err
		}
	}
	m.mu.Unlock()
	result := m.Models(ctx)
	m.notify()
	return result, nil
}

func (m *Manager) Search(ctx context.Context, input string) (protocol.LocalSearchResult, error) {
	if filepath.IsAbs(input) {
		if !strings.EqualFold(filepath.Ext(input), ".gguf") {
			return protocol.LocalSearchResult{}, protocol.Errorf(protocol.CodeBadRequest, "%s", i18n.T("Choose a local .gguf file."))
		}
		f, err := os.Open(input)
		if err != nil {
			return protocol.LocalSearchResult{}, err
		}
		defer f.Close()
		info, err := f.Stat()
		if err != nil || !info.Mode().IsRegular() {
			return protocol.LocalSearchResult{}, protocol.Errorf(protocol.CodeBadRequest, "%s", i18n.T("Choose a regular GGUF file."))
		}
		header, err := io.ReadAll(io.LimitReader(f, ggufHeaderLimit))
		if err != nil {
			return protocol.LocalSearchResult{}, err
		}
		meta, err := parseGGUF(header)
		if err != nil {
			return protocol.LocalSearchResult{}, protocol.Errorf(protocol.CodeBadRequest, "%s", i18n.T("This file has an unreadable GGUF header."))
		}
		meta.ModelBytes = info.Size()
		e := Entry{Files: []File{{Name: filepath.Base(input), Role: protocol.LocalFileModel, Size: info.Size()}}, Params: meta.Params, Context: meta.Context, Tools: meta.Tools}
		plan := planModel(e, meta, hostHardware(m.Devices(ctx)), protocol.LocalModelOverride{})
		name := filepath.Base(input)
		return protocol.LocalSearchResult{Repo: input, Path: input, Recommended: name,
			Files: []protocol.LocalFile{{Name: name, Role: protocol.LocalFileModel, Size: info.Size(),
				Params: meta.Params, Context: meta.Context, Tools: meta.Tools, Warning: modelWarning(meta),
				Plan: plan, Verdict: plan.Verdict, Recommended: true}}}, nil
	}
	return m.searchHF(ctx, input)
}

func (m *Manager) Add(ctx context.Context, p protocol.LocalAddParams) (protocol.LocalModel, error) {
	if filepath.IsAbs(p.Repo) {
		model, err := m.addLocal(ctx, p.Repo)
		if err == nil {
			m.refreshRunningPresets(ctx)
			m.notify()
		}
		return model, err
	}
	result, err := m.Search(ctx, p.Repo)
	if err != nil {
		return protocol.LocalModel{}, err
	}
	if p.Revision != "" && result.Revision != p.Revision {
		return protocol.LocalModel{}, protocol.Errorf(protocol.CodeConflict, "%s", i18n.T("This repository changed since the model preview. Find its files again before downloading."))
	}
	if result.Gated {
		return protocol.LocalModel{}, protocol.Errorf(protocol.CodeUnavailable, "%s", i18n.T("This repository requires Hugging Face access, which Hyakki does not store."))
	}
	selected := p.File
	if selected == "" {
		selected = result.Path
	}
	if selected == "" {
		selected = result.Recommended
	}
	parts := matchGGUFFiles(result.Files, selected)
	if len(parts) == 0 {
		return protocol.LocalModel{}, protocol.Errorf(protocol.CodeBadRequest, "%s", i18n.T("Choose a complete GGUF model from this repository."))
	}
	projector := p.Mmproj
	if projector == "" {
		projector = matchingMmproj(result.Files, selected)
	}
	e := Entry{Repo: result.Repo, Revision: result.Revision, Name: parts[0].Name,
		Class: protocol.LocalClassLight}
	for _, file := range parts {
		e.Files = append(e.Files, File{Name: file.Name, Role: file.Role, Size: file.Size, SHA256: file.SHA256})
	}
	e.Params, e.Context, e.Tools = parts[0].Params, parts[0].Context, parts[0].Tools
	if e.Params > protocol.LightParams {
		e.Class = protocol.LocalClassMain
	}
	if projector != "" {
		if projector == selected {
			return protocol.LocalModel{}, protocol.Errorf(protocol.CodeBadRequest, "%s", i18n.T("Pick a different picture reader from the model file."))
		}
		found := false
		for _, file := range result.Files {
			if file.Name == projector && file.Role == protocol.LocalFileMmproj {
				e.Files = append(e.Files, File{Name: file.Name, Role: file.Role, Size: file.Size, SHA256: file.SHA256})
				found = true
				break
			}
		}
		if !found {
			return protocol.LocalModel{}, protocol.Errorf(protocol.CodeBadRequest, "%s", i18n.T("Choose files from the selected repository."))
		}
	}
	e.License.Name = result.License
	if e.License.Name == "" {
		e.License.Name = i18n.T("See the model repository for its licence")
	}
	e.License.URL = m.hfBase + "/" + e.Repo + "/blob/" + e.Revision + "/LICENSE"
	e.Vision, e.Audio, e.Video = projector != "", projector != "", projector != ""
	hash := sha256.Sum256([]byte(e.Repo + "\x00" + e.Revision + "\x00" + e.Name + "\x00" + projector))
	e.ID = "hf-" + hex.EncodeToString(hash[:8])
	m.mu.Lock()
	for _, existing := range m.entries() {
		if existing.ID == e.ID {
			m.mu.Unlock()
			return m.modelOnDisk(existing), nil
		}
		if existing.Repo == e.Repo && existing.Revision != e.Revision {
			m.mu.Unlock()
			return protocol.LocalModel{}, protocol.Errorf(protocol.CodeConflict, "%s", i18n.T("A model from this repository is pinned to another revision. Remove it before adding this one."))
		}
	}
	custom := append(readCustom(), e)
	err = writeCustom(custom)
	m.mu.Unlock()
	if err != nil {
		return protocol.LocalModel{}, err
	}
	if err := m.Download(ctx, e.ID); err != nil {
		return protocol.LocalModel{}, err
	}
	m.notify()
	model := e.info()
	model.State = protocol.LocalDownloading
	return model, nil
}

func (m *Manager) Busy() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.jobs) != 0 || m.starting != nil || m.leases > 0
}

// Acquire keeps the router alive until its release function is called, even
// while a long streamed completion is in progress. Gateway wraps each attempt.
func (m *Manager) Acquire() func() {
	for {
		m.mu.Lock()
		if ch := m.switching; ch != nil {
			m.mu.Unlock()
			<-ch
			continue
		}
		m.leases++
		if m.idle != nil {
			m.idle.Stop()
			m.idle = nil
		}
		m.mu.Unlock()
		break
	}
	var once sync.Once
	return func() { once.Do(func() { m.mu.Lock(); m.leases--; m.armIdleLocked(); m.mu.Unlock() }) }
}

func (m *Manager) beginRuntimeSwap(ctx context.Context) (func(), error) {
	ticker := time.NewTicker(50 * time.Millisecond)
	defer ticker.Stop()
	for {
		m.mu.Lock()
		if m.leases == 0 && m.starting == nil && m.switching == nil {
			ch := make(chan struct{})
			m.switching = ch
			m.mu.Unlock()
			return func() { m.mu.Lock(); m.switching = nil; close(ch); m.mu.Unlock() }, nil
		}
		m.mu.Unlock()
		select {
		case <-ticker.C:
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
}

func (m *Manager) Close() error {
	m.mu.Lock()
	m.closed = true
	jobs := make([]*job, 0, len(m.jobs))
	for _, j := range m.jobs {
		jobs = append(jobs, j)
		j.cancel()
	}
	m.mu.Unlock()
	for _, j := range jobs {
		<-j.done
	}
	m.Stop()
	m.client.CloseIdleConnections()
	return nil
}

func (m *Manager) idleMinutes() int {
	n := m.settings().Providers.Models.IdleMinutes
	if n < 1 {
		n = protocol.DefaultLocalIdleMinutes
	}
	if n > protocol.LocalIdleMinutesMax {
		n = protocol.LocalIdleMinutesMax
	}
	return n
}

func (m *Manager) armIdleLocked() {
	if m.idle != nil {
		m.idle.Stop()
		m.idle = nil
	}
	if m.server == nil || m.leases > 0 || m.starting != nil {
		return
	}
	m.idle = time.AfterFunc(time.Duration(m.idleMinutes())*time.Minute, m.stopIfIdle)
}

func (m *Manager) stopIfIdle() {
	m.stopServer(true)
}

func ioReadLimit(r io.Reader, max int64) ([]byte, error) {
	data, err := io.ReadAll(io.LimitReader(r, max+1))
	if err == nil && int64(len(data)) > max {
		err = errors.New(i18n.T("The licence file is too large."))
	}
	return data, err
}
