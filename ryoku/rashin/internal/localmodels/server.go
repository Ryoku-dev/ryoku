package localmodels

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	i18n "ryoku-i18n"
	"ryoku-rashin/internal/childproc"
	"ryoku-rashin/internal/paths"
	"ryoku-rashin/internal/protocol"
)

type server struct {
	cmd       *exec.Cmd
	exited    chan struct{}
	base, key string
	build     int
	loaded    map[string]bool
	loading   map[string]chan struct{}
	presets   map[string]bool
	plans     map[string]protocol.LocalPlan
	choices   map[string]protocol.LocalModelOverride
	mu        sync.Mutex
	presetMu  sync.Mutex
}

func (m *Manager) runtimeInfo(r resolution, s *server, starting, stopping bool) protocol.LocalRuntime {
	state := r.State
	if starting {
		state = protocol.RuntimeStarting
	}
	if s != nil {
		state = protocol.RuntimeReady
	}
	if stopping {
		state = protocol.RuntimeStopping
	}
	info := protocol.LocalRuntime{State: state, Source: r.Bin.Source, Path: r.Bin.Path, Build: r.Bin.Build,
		Version: r.Bin.Version, Backend: r.Bin.Backend, MinBuild: r.Min, Log: paths.LlamaLog(), Offer: offers()}
	if r.State != protocol.RuntimeStopped && s == nil && !stopping {
		if err := runtimeError(r); err != nil {
			info.Error = err.Error()
		}
	}
	if r.State == protocol.RuntimeStopped {
		m.mu.Lock()
		if key, ok := probeKey(r.Bin.Path); ok && key == m.deviceKey {
			info.Devices = append([]protocol.LocalDevice(nil), m.devices...)
		}
		m.mu.Unlock()
	}
	if s != nil {
		info.Address = s.base
		s.mu.Lock()
		for id, ok := range s.loaded {
			if ok {
				info.Loaded = append(info.Loaded, id)
			}
		}
		if len(s.plans) > 0 {
			info.Plans = make(map[string]protocol.LocalPlan, len(s.plans))
			for id, plan := range s.plans {
				info.Plans[id] = plan
			}
		}
		s.mu.Unlock()
		sort.Strings(info.Loaded)
	}
	return info
}

func (m *Manager) Ensure(ctx context.Context, model string) (string, string, error) {
	model = strings.TrimPrefix(model, protocol.ProviderRashinModels+"/")
	e, found := m.entry(model)
	if !found {
		return "", "", protocol.Errorf(protocol.CodeNotFound, "%s", i18n.T("That model is not in Rashin's catalogue."))
	}
	if m.modelOnDisk(e).State != protocol.LocalInstalled {
		return "", "", protocol.Errorf(protocol.CodeUnavailable, "%s", i18n.T("Download this model before asking it a question."))
	}
	m.mu.Lock()
	m.active = e.ID
	m.mu.Unlock()
	s, err := m.ensureServer(ctx, e.minBuild())
	if err != nil {
		return "", "", err
	}
	s.mu.Lock()
	present := s.presets[e.ID]
	s.mu.Unlock()
	if !present {
		if err := m.refreshPresets(ctx, s); err != nil {
			return "", "", err
		}
	}
	if err := m.syncChoice(ctx, s, e); err != nil {
		return "", "", err
	}
	if err := m.ensureLoaded(ctx, s, e); err != nil {
		return "", "", err
	}
	m.mu.Lock()
	m.armIdleLocked()
	m.mu.Unlock()
	return s.base + "/v1", s.key, nil
}

func (m *Manager) Start(ctx context.Context, id string) (protocol.LocalRuntime, error) {
	if id == "" {
		inventory := m.Inventory()
		if len(inventory) == 0 {
			for _, model := range m.Models(ctx).Models {
				if model.State == protocol.LocalInstalled {
					inventory = append(inventory, model)
				}
			}
		}
		if len(inventory) == 0 {
			return protocol.LocalRuntime{}, protocol.Errorf(protocol.CodeUnavailable, "%s", i18n.T("Download a model before starting llama.cpp."))
		}
		id = inventory[0].ID
	}
	release := m.Acquire()
	defer release()
	_, _, err := m.Ensure(ctx, id)
	if err != nil {
		return protocol.LocalRuntime{}, err
	}
	return m.Models(ctx).Runtime, nil
}

func (m *Manager) ensureServer(ctx context.Context, min int) (*server, error) {
	for {
		m.mu.Lock()
		if m.closed {
			m.mu.Unlock()
			return nil, protocol.Errorf(protocol.CodeUnavailable, "%s", i18n.T("Rashin is shutting down."))
		}
		if waiting := m.stopping; waiting != nil {
			m.mu.Unlock()
			select {
			case <-waiting:
				continue
			case <-ctx.Done():
				return nil, ctx.Err()
			}
		}
		if s := m.server; s != nil {
			select {
			case <-s.exited:
				m.server = nil
			default:
				m.mu.Unlock()
				return s, nil
			}
		}
		if waiting := m.starting; waiting != nil {
			m.mu.Unlock()
			select {
			case <-waiting:
				continue
			case <-ctx.Done():
				return nil, ctx.Err()
			}
		}
		wait := make(chan struct{})
		m.starting = wait
		m.mu.Unlock()
		m.notify()
		s, err := m.startServer(ctx, min)
		m.mu.Lock()
		m.starting = nil
		if err == nil && !m.closed {
			m.server = s
			m.armIdleLocked()
		} else if err == nil {
			err = errors.New(i18n.T("Rashin is shutting down."))
		}
		if err == nil {
			m.lastStart = ""
		} else if ctx.Err() == nil {
			m.lastStart = err.Error()
		}
		close(wait)
		m.mu.Unlock()
		if err != nil && s != nil {
			childproc.Terminate(s.cmd, s.exited, 15*time.Second)
		}
		m.notify()
		return s, err
	}
}

func (m *Manager) startServer(ctx context.Context, min int) (*server, error) {
	res := m.resolve(ctx, min)
	if err := runtimeError(res); err != nil {
		return nil, err
	}
	entries := m.entries()
	var ready []Entry
	for _, e := range entries {
		model := m.modelOnDisk(e)
		if model.State == protocol.LocalInstalled && res.Bin.meets(e.minBuild()) {
			ready = append(ready, e)
		}
	}
	if len(ready) == 0 {
		return nil, protocol.Errorf(protocol.CodeUnavailable, "%s", i18n.T("No installed models can run with this llama.cpp build."))
	}
	if err := os.MkdirAll(paths.LlamaStateDir(), 0o700); err != nil {
		return nil, err
	}
	if err := os.MkdirAll(paths.LlamaCache(), 0o700); err != nil {
		return nil, err
	}
	m.mu.Lock()
	m.deviceKey = ""
	m.mu.Unlock()
	devices := m.probeDevices(ctx, res.Bin)
	prepared := m.makePresets(ready, devices)
	presets := renderPresets(prepared)
	if err := writeAtomic(paths.LlamaPresets(), []byte(presets), 0o600); err != nil {
		return nil, err
	}
	secret, err := childproc.NewSecret()
	if err != nil {
		return nil, err
	}
	if err := writeAtomic(paths.LlamaKey(), []byte(secret+"\n"), 0o600); err != nil {
		return nil, err
	}
	port, err := childproc.FreePort()
	if err != nil {
		return nil, err
	}
	log, err := os.OpenFile(paths.LlamaLog(), os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o600)
	if err != nil {
		return nil, err
	}
	defer log.Close()
	args := []string{"--host", "127.0.0.1", "--port", strconv.Itoa(port), "--api-key-file", paths.LlamaKey(),
		"--models-preset", paths.LlamaPresets(), "--models-max", strconv.Itoa(modelsMax(ready, devices)),
		"--no-webui", "--offline", "--jinja"}
	cmd := exec.Command(res.Bin.Path, args...)
	cmd.Dir = filepath.Dir(res.Bin.Path)
	cmd.Env = serverEnv(secret)
	cmd.Stdout, cmd.Stderr = log, log
	cmd.SysProcAttr = childproc.Attr()
	s := &server{cmd: cmd, exited: make(chan struct{}), base: fmt.Sprintf("http://127.0.0.1:%d", port), key: secret, build: res.Bin.Build,
		loaded: make(map[string]bool), loading: make(map[string]chan struct{}), presets: presetIDs(presets),
		plans: make(map[string]protocol.LocalPlan), choices: make(map[string]protocol.LocalModelOverride)}
	for _, p := range prepared {
		s.plans[p.ID] = p.Plan
		s.choices[p.ID] = m.modelChoice(p.ID)
	}
	if err := childproc.LaunchLocked(cmd, s.exited); err != nil {
		return nil, fmt.Errorf("%s: %w", i18n.T("llama.cpp did not start"), err)
	}
	if err := childproc.WaitHealthy(ctx, childproc.Health{URL: s.base + "/health", Bearer: secret, Exited: s.exited, Wait: 30 * time.Second, Period: 250 * time.Millisecond}); err != nil {
		childproc.Terminate(cmd, s.exited, 2*time.Second)
		line := lastLlamaLine()
		if line != "" {
			return nil, protocol.Errorf(protocol.CodeUnavailable, "%s", i18n.Tf("llama.cpp could not start: %s. Check the llama.cpp log in Settings > Providers > Local models.", line))
		}
		return nil, fmt.Errorf("%s: %w", i18n.Tf("llama.cpp did not become ready. Its log is %s.", paths.LlamaLog()), err)
	}
	return s, nil
}

func serverEnv(secret string) []string {
	return append(childEnv(), "LLAMA_API_KEY="+secret)
}

func (m *Manager) modelChoice(id string) protocol.LocalModelOverride {
	settings := m.settings().Providers.Models
	choice := settings.Overrides[id]
	if choice.Device == "" {
		choice.Device = settings.Device
	}
	return choice
}

func (m *Manager) syncChoice(ctx context.Context, s *server, e Entry) error {
	choice := m.modelChoice(e.ID)
	s.mu.Lock()
	previous := s.choices[e.ID]
	s.mu.Unlock()
	if previous == choice {
		return nil
	}
	m.mu.Lock()
	devices := append([]protocol.LocalDevice(nil), m.devices...)
	m.mu.Unlock()
	plan := m.plan(e, hostHardware(devices))
	if err := m.retryPreset(ctx, s, e, plan); err != nil {
		return err
	}
	s.mu.Lock()
	if s.choices == nil {
		s.choices = make(map[string]protocol.LocalModelOverride)
	}
	s.choices[e.ID] = choice
	s.loaded[e.ID] = false
	s.mu.Unlock()
	return nil
}

func (m *Manager) plan(e Entry, h hardwareProfile) protocol.LocalPlan {
	meta := ggufMetadata{Params: e.Params, Context: e.Context, Tools: e.Tools}
	if man, ok := readManifest(e); ok {
		if man.Header == nil || man.Header.Blocks == 0 || man.Header.Architecture == "" {
			m.adoptMu.Lock()
			man, ok = readManifest(e)
			if ok && (man.Header == nil || man.Header.Blocks == 0 || man.Header.Architecture == "") {
				if header, err := localGGUF(e); err == nil {
					for _, file := range e.Files {
						if file.Role == protocol.LocalFileModel {
							header.ModelBytes += file.Size
						} else if file.Role == protocol.LocalFileMmproj {
							header.MmprojBytes += file.Size
						}
					}
					if header.Context == 0 {
						header.Context = e.Context
					}
					if header.Params == 0 {
						header.Params = e.Params
					}
					if e.Catalogue {
						header.Tools = e.Tools
					}
					man.Header = &header
					_ = writeManifest(e, man)
				}
			}
			m.adoptMu.Unlock()
		}
		if man.Header != nil {
			meta = *man.Header
		}
	}
	if meta.Context == 0 {
		meta.Context = e.Context
	}
	if meta.Params == 0 {
		meta.Params = e.Params
	}
	return planModel(e, meta, h, m.modelChoice(e.ID))
}

func (m *Manager) makePresets(entries []Entry, devices []protocol.LocalDevice) []presetModel {
	presets := make([]presetModel, 0, len(entries))
	h := hostHardware(devices)
	for _, e := range entries {
		model, ok := e.model()
		if !ok {
			continue
		}
		plan := m.plan(e, h)
		p := presetModel{ID: e.ID, Model: filePath(e, model.Name), Context: plan.Context,
			Device: plan.Device, Args: e.Args, PlanArgs: planPresetArgs(plan, h.Cores), Plan: plan}
		if mm, ok := e.mmproj(); ok {
			p.Mmproj = filePath(e, mm.Name)
		}
		for _, f := range e.Files {
			if f.Role == protocol.LocalFileModel && f.Name != model.Name {
				p.Shards = append(p.Shards, filePath(e, f.Name))
			}
		}
		presets = append(presets, p)
	}
	return presets
}

func (m *Manager) refreshRunningPresets(ctx context.Context) {
	m.mu.Lock()
	s := m.server
	m.mu.Unlock()
	if s != nil {
		_ = m.refreshPresets(ctx, s)
	}
}

func (m *Manager) refreshPresets(ctx context.Context, s *server) error {
	s.presetMu.Lock()
	defer s.presetMu.Unlock()
	var entries []Entry
	for _, e := range m.entries() {
		if m.modelOnDisk(e).State == protocol.LocalInstalled && s.build >= e.minBuild() {
			entries = append(entries, e)
		}
	}
	m.mu.Lock()
	devices := append([]protocol.LocalDevice(nil), m.devices...)
	m.mu.Unlock()
	prepared := m.makePresets(entries, devices)
	presets := renderPresets(prepared)
	if err := writeAtomic(paths.LlamaPresets(), []byte(presets), 0o600); err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, s.base+"/models?reload=1", nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+s.key)
	resp, err := (&http.Client{Timeout: 10 * time.Second}).Do(req)
	if err != nil {
		return err
	}
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("llama.cpp preset reload: %s", resp.Status)
	}
	s.mu.Lock()
	s.presets = presetIDs(presets)
	if s.plans == nil {
		s.plans = make(map[string]protocol.LocalPlan)
	}
	if s.choices == nil {
		s.choices = make(map[string]protocol.LocalModelOverride)
	}
	for _, p := range prepared {
		if _, ok := s.plans[p.ID]; !ok {
			s.plans[p.ID] = p.Plan
			s.choices[p.ID] = m.modelChoice(p.ID)
		}
	}
	s.mu.Unlock()
	return nil
}

func retryableLoad(line string) bool {
	line = strings.ToLower(line)
	for _, marker := range []string{"out of memory", "oom", "allocate", "device lost", "vk_error", "cuda error"} {
		if strings.Contains(line, marker) {
			return true
		}
	}
	return false
}

func (m *Manager) retryPreset(ctx context.Context, s *server, e Entry, plan protocol.LocalPlan) error {
	s.presetMu.Lock()
	defer s.presetMu.Unlock()
	m.mu.Lock()
	devices := append([]protocol.LocalDevice(nil), m.devices...)
	m.mu.Unlock()
	var entries []Entry
	for _, item := range m.entries() {
		if m.modelOnDisk(item).State == protocol.LocalInstalled && s.build >= item.minBuild() {
			entries = append(entries, item)
		}
	}
	presets := m.makePresets(entries, devices)
	for i := range presets {
		if presets[i].ID != e.ID {
			continue
		}
		presets[i].Device, presets[i].Context = plan.Device, plan.Context
		presets[i].PlanArgs = planPresetArgs(plan, hostHardware(devices).Cores)
	}
	body, _ := json.Marshal(map[string]string{"model": e.ID})
	if req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.base+"/models/unload", bytes.NewReader(body)); err == nil {
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+s.key)
		if resp, err := (&http.Client{Timeout: 10 * time.Second}).Do(req); err == nil {
			_ = resp.Body.Close()
		}
	}
	if err := writeAtomic(paths.LlamaPresets(), []byte(renderPresets(presets)), 0o600); err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, s.base+"/models?reload=1", nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+s.key)
	resp, err := (&http.Client{Timeout: 10 * time.Second}).Do(req)
	if err != nil {
		return err
	}
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("llama.cpp preset reload: %s", resp.Status)
	}
	s.mu.Lock()
	s.plans[e.ID] = plan
	s.mu.Unlock()
	return nil
}

func (m *Manager) ensureLoaded(ctx context.Context, s *server, e Entry) error {
	for {
		s.mu.Lock()
		if ch := s.loading[e.ID]; ch != nil {
			s.mu.Unlock()
			select {
			case <-ch:
				continue
			case <-ctx.Done():
				return ctx.Err()
			}
		}
		ch := make(chan struct{})
		s.loading[e.ID] = ch
		s.mu.Unlock()
		err := m.loadModel(ctx, s, e)
		if err != nil && ctx.Err() == nil && retryableLoad(lastLlamaLine()) {
			original := err
			s.mu.Lock()
			plan := s.plans[e.ID]
			s.mu.Unlock()
			for attempt := 0; attempt < 2 && err != nil; attempt++ {
				if attempt == 0 {
					plan.Context = max(128, plan.Context/2)
					plan.Retry = i18n.T("The first load ran out of graphics memory; trying a smaller context.")
					plan.Reason = plan.Retry
				} else {
					plan.Device, plan.Kind, plan.DeviceName, plan.GPULayers = deviceNone, protocol.LocalDeviceCPU, i18n.T("Processor"), 0
					plan.Verdict = protocol.LocalVerdictCPU
					plan.Retry = i18n.T("The graphics card could not load the model; trying the processor.")
					plan.Reason = plan.Retry
				}
				if replanErr := m.retryPreset(ctx, s, e, plan); replanErr != nil {
					err = replanErr
					break
				}
				err = m.loadModel(ctx, s, e)
			}
			if err != nil {
				err = fmt.Errorf("%w; %v", original, err)
			}
		}
		s.mu.Lock()
		s.loaded[e.ID] = err == nil
		delete(s.loading, e.ID)
		close(ch)
		s.mu.Unlock()
		m.mu.Lock()
		if err == nil {
			delete(m.failures, e.ID)
		} else if ctx.Err() == nil {
			m.failures[e.ID] = err.Error()
		}
		m.mu.Unlock()
		m.notify()
		return err
	}
}

func (m *Manager) loadModel(ctx context.Context, s *server, e Entry) error {
	ctx, cancel := context.WithTimeout(ctx, loadBudget(e))
	defer cancel()
	st, err := m.modelStatus(ctx, s, e.ID)
	if err == nil && st.Value == "loaded" {
		m.probeModel(ctx, s, e)
		return nil
	}
	if err == nil && st.Failed {
		return loadFailure(e.Name)
	}
	if err == nil && st.Value != "loading" {
		body, _ := json.Marshal(map[string]string{"model": e.ID})
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.base+"/models/load", bytes.NewReader(body))
		if err != nil {
			return err
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+s.key)
		resp, err := (&http.Client{Timeout: 20 * time.Second}).Do(req)
		if err != nil {
			return err
		}
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
			_ = resp.Body.Close()
			if resp.StatusCode != http.StatusBadRequest || !strings.Contains(strings.ToLower(string(body)), "already running") {
				return loadFailure(e.Name)
			}
		} else {
			_ = resp.Body.Close()
		}
	}
	ticker := time.NewTicker(400 * time.Millisecond)
	defer ticker.Stop()
	for {
		status, err := m.modelStatus(ctx, s, e.ID)
		if err == nil {
			if status.Value == "loaded" {
				m.probeModel(ctx, s, e)
				return nil
			}
			if status.Failed {
				return loadFailure(e.Name)
			}
		}
		select {
		case <-ticker.C:
		case <-s.exited:
			return loadFailure(e.Name)
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}

func loadFailure(name string) error {
	line := lastLlamaLine()
	if line == "" {
		return protocol.Errorf(protocol.CodeUnavailable, "%s", i18n.Tf("llama.cpp failed to load %s. Check its log in Settings > Providers > Local models or choose Auto.", name))
	}
	return protocol.Errorf(protocol.CodeUnavailable, "%s", i18n.Tf("llama.cpp failed to load %s: %s. Check its log in Settings > Providers > Local models or choose Auto.", name, line))
}

func lastLlamaLine() string {
	file, err := os.Open(paths.LlamaLog())
	if err != nil {
		return ""
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		return ""
	}
	if info.Size() > 8192 {
		if _, err := file.Seek(-8192, io.SeekEnd); err != nil {
			return ""
		}
	}
	data, err := io.ReadAll(io.LimitReader(file, 8192))
	if err != nil {
		return ""
	}
	var last string
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		// terminal controls and unbounded model-authored lines do not enter a notice
		line = strings.Map(func(r rune) rune {
			if r < ' ' || r == 127 {
				return -1
			}
			return r
		}, line)
		if len(line) > 400 {
			line = line[:400]
		}
		last = line
	}
	return last
}

func loadBudget(e Entry) time.Duration {
	wait := time.Duration(e.size()/(1<<30)+1) * 20 * time.Second
	if wait < time.Minute {
		wait = time.Minute
	}
	return wait
}

type status struct {
	Value  string `json:"value"`
	Failed bool   `json:"failed"`
}

func (m *Manager) modelStatus(ctx context.Context, s *server, id string) (status, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, s.base+"/models", nil)
	if err != nil {
		return status{}, err
	}
	req.Header.Set("Authorization", "Bearer "+s.key)
	resp, err := (&http.Client{Timeout: 3 * time.Second}).Do(req)
	if err != nil {
		return status{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return status{}, fmt.Errorf("/models: %s", resp.Status)
	}
	var list struct {
		Data []struct {
			ID     string `json:"id"`
			Status status `json:"status"`
		} `json:"data"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&list); err != nil {
		return status{}, err
	}
	for _, item := range list.Data {
		if item.ID == id {
			return item.Status, nil
		}
	}
	return status{}, fmt.Errorf("%s: %s", i18n.T("llama.cpp has no preset for model"), id)
}

func (m *Manager) probeModel(ctx context.Context, s *server, e Entry) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, s.base+"/props?model="+e.ID+"&autoload=false", nil)
	if err != nil {
		return
	}
	req.Header.Set("Authorization", "Bearer "+s.key)
	resp, err := (&http.Client{Timeout: 3 * time.Second}).Do(req)
	if err != nil {
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return
	}
	var props struct {
		Default struct {
			Context int `json:"n_ctx"`
		} `json:"default_generation_settings"`
		Context   int `json:"n_ctx"`
		ModelInfo struct {
			Context   int  `json:"n_ctx"`
			GPULayers *int `json:"n_gpu_layers"`
			Offloaded *int `json:"n_gpu_layers_offloaded"`
		} `json:"model_info"`
		GPULayers *int `json:"n_gpu_layers"`
		Caps      *struct {
			Tools bool `json:"supports_tool_calls"`
		} `json:"chat_template_caps"`
		Modalities struct {
			Vision bool `json:"vision"`
			Audio  bool `json:"audio"`
		} `json:"modalities"`
	}
	if json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&props) != nil {
		return
	}
	actual := props.Default.Context
	if actual == 0 {
		actual = props.Context
	}
	if actual == 0 {
		actual = props.ModelInfo.Context
	}
	if actual > 0 || props.ModelInfo.Offloaded != nil || props.ModelInfo.GPULayers != nil || props.GPULayers != nil {
		s.mu.Lock()
		plan := s.plans[e.ID]
		plan.ActualContext = actual
		if props.ModelInfo.Offloaded != nil {
			plan.GPULayers = *props.ModelInfo.Offloaded
		} else if props.ModelInfo.GPULayers != nil {
			plan.GPULayers = *props.ModelInfo.GPULayers
		} else if props.GPULayers != nil {
			plan.GPULayers = *props.GPULayers
		}
		if s.plans == nil {
			s.plans = make(map[string]protocol.LocalPlan)
		}
		s.plans[e.ID] = plan
		s.mu.Unlock()
	}
	man, ok := readManifest(e)
	if !ok {
		return
	}
	p := &probed{Context: actual, Tools: props.Caps == nil || props.Caps.Tools, Vision: props.Modalities.Vision, Audio: props.Modalities.Audio}
	if !e.Catalogue {
		if req, err := http.NewRequestWithContext(ctx, http.MethodGet, s.base+"/v1/models", nil); err == nil {
			req.Header.Set("Authorization", "Bearer "+s.key)
			if response, err := (&http.Client{Timeout: 3 * time.Second}).Do(req); err == nil {
				var models struct {
					Data []struct {
						ID   string `json:"id"`
						Meta struct {
							Params int64 `json:"n_params"`
						} `json:"meta"`
					} `json:"data"`
				}
				if response.StatusCode == http.StatusOK && json.NewDecoder(io.LimitReader(response.Body, 1<<20)).Decode(&models) == nil {
					for _, model := range models.Data {
						if model.ID == e.ID {
							p.Params = model.Meta.Params
							break
						}
					}
				}
				_ = response.Body.Close()
			}
		}
	}
	man.Probed = p
	_ = writeManifest(e, man)
}

func (m *Manager) Stop() {
	m.stopServer(false)
}

func (m *Manager) stopServer(onlyIdle bool) {
	for {
		m.mu.Lock()
		if onlyIdle && (m.leases != 0 || m.starting != nil || m.stopping != nil) {
			m.mu.Unlock()
			return
		}
		if m.idle != nil {
			m.idle.Stop()
			m.idle = nil
		}
		if ch := m.starting; ch != nil {
			m.mu.Unlock()
			<-ch
			continue
		}
		if ch := m.stopping; ch != nil {
			m.mu.Unlock()
			<-ch
			continue
		}
		s := m.server
		if s == nil {
			m.mu.Unlock()
			return
		}
		m.server = nil
		ch := make(chan struct{})
		m.stopping = ch
		m.mu.Unlock()
		m.notify()
		childproc.Terminate(s.cmd, s.exited, 2*time.Second)
		// the router may spawn workers that outlive its leader
		if s.cmd != nil && s.cmd.Process != nil {
			_ = syscall.Kill(-s.cmd.Process.Pid, syscall.SIGKILL)
		}
		_ = os.Remove(paths.LlamaKey())
		m.mu.Lock()
		m.stopping = nil
		m.active = ""
		m.lastStart = ""
		close(ch)
		m.mu.Unlock()
		m.notify()
		return
	}
}
