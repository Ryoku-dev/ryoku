package gateway

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"path"
	"slices"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	i18n "ryoku-i18n"
	"ryoku-rashin/internal/protocol"
)

const (
	// localTTL is how long a local server's model list is trusted before the
	// server is asked again.
	localTTL = 30 * time.Second
	// localAskBudget bounds asking a local server for its models. One that is
	// not running refuses at once; one off the network gives up here.
	localAskBudget = 2 * time.Second
	// ollamaContext is the window Ollama gives a model it was not told
	// otherwise, whatever the model was trained for: it cuts a longer prompt
	// without a word, so the engine must plan for it.
	ollamaContext = 4096
)

// localServers remembers what each local provider's server said about its
// models, and reaches those servers through a client of their own.
type localServers struct {
	client *http.Client
	mu     sync.Mutex
	found  map[string]*localFound
}

// localFound is one local provider's last answer, for the address it was
// asked at.
type localFound struct {
	mu      sync.Mutex
	address string
	at      time.Time
	running bool
	models  []model
}

func newLocalServers() localServers {
	dialer := &net.Dialer{Timeout: localAskBudget, Control: onlyPrivate}
	return localServers{
		client: &http.Client{Transport: &http.Transport{
			// a proxy would carry a request for this computer somewhere else
			Proxy:                 nil,
			DialContext:           dialer.DialContext,
			MaxIdleConns:          8,
			MaxIdleConnsPerHost:   4,
			IdleConnTimeout:       90 * time.Second,
			ResponseHeaderTimeout: localFirstByteBudget,
		}},
		found: make(map[string]*localFound),
	}
}

// onlyPrivate refuses a connection to an address that is neither loopback nor
// on a private network, whatever name led there.
func onlyPrivate(_, address string, _ syscall.RawConn) error {
	host, _, err := net.SplitHostPort(address)
	if err != nil {
		return err
	}
	ip, err := netip.ParseAddr(host)
	if err != nil {
		return err
	}
	if !protocol.PrivateIP(ip) {
		return fmt.Errorf("%s is not on this computer or the local network", host)
	}
	return nil
}

// localProviders builds the local providers: Rashin's own models from what
// is installed, the others from the settings' addresses and what their
// servers answered. With fresh set the servers are asked now; otherwise an
// answer younger than localTTL stands.
func (g *Gateway) localProviders(ctx context.Context, fresh bool) []provider {
	addresses := g.settings().Providers.Local
	defaults := protocol.DefaultLocalAddresses()
	providers := make([]provider, 0, len(protocol.LocalProviders))
	changed := false
	for _, id := range protocol.LocalProviders {
		p := localProvider(id)
		if id == protocol.ProviderRashinModels {
			models, flipped := g.managedModels()
			changed = changed || flipped
			p.Models, p.onDemand = models, len(models) > 0
			providers = append(providers, p)
			continue
		}
		address := addresses[id]
		if address == "" {
			address = defaults[id]
		}
		running, models, flipped := g.localModels(ctx, id, address, fresh)
		changed = changed || flipped
		p.BaseURL, p.running, p.Models = address, running, models
		providers = append(providers, p)
	}
	if changed && g.ctx.Err() == nil {
		g.wg.Add(1)
		go func() {
			defer g.wg.Done()
			g.notifyChange()
		}()
	}
	return providers
}

// localProvider is what a local provider is before its server is asked.
func localProvider(id string) provider {
	p := provider{ID: id, Tier: protocol.TierLocal, Auth: "none", Keyless: true}
	switch id {
	case protocol.ProviderRashinModels:
		p.Name, p.Note = i18n.T("Hyakki models"), i18n.T("Models Hyakki downloads and runs itself. They start when a request needs them and stop when idle.")
	case protocol.ProviderOllamaLocal:
		p.Name, p.Signup, p.Note = "Ollama", "https://ollama.com/download", i18n.T("Install Ollama and run ollama serve.")
	case protocol.ProviderLlamaCpp:
		p.Name, p.Signup, p.Note = "llama.cpp", "https://github.com/ggml-org/llama.cpp", i18n.T("Run llama-server -m with your model file, such as model.gguf.")
	}
	return p
}

// managedModels are Rashin's own installed models, read from their
// manifests without starting anything, and whether they changed since the
// last read.
func (g *Gateway) managedModels() ([]model, bool) {
	var models []model
	if local := g.managed(); local != nil {
		for _, installed := range local.Inventory() {
			if installed.State != protocol.LocalInstalled {
				continue
			}
			models = append(models, model{
				ID: installed.ID, Name: firstText(installed.Name, installed.ID), Context: installed.Context,
				Tools: installed.Tools, Reasoning: installed.Reasoning, Vision: installed.Vision, Audio: installed.Audio,
				Params: installed.Params,
				Light:  installed.Class == protocol.LocalClassLight || installed.Params <= protocol.LightParams,
			})
		}
	}
	g.local.mu.Lock()
	found := g.local.found[protocol.ProviderRashinModels]
	if found == nil {
		found = &localFound{}
		g.local.found[protocol.ProviderRashinModels] = found
	}
	g.local.mu.Unlock()
	found.mu.Lock()
	defer found.mu.Unlock()
	changed := !found.at.IsZero() && !sameModels(models, found.models)
	found.at, found.models = g.now(), models
	return models, changed
}

// localModels returns a local server's models, asking it when its answer is
// stale or for another address, and reports whether what it says changed
// since it was last asked.
func (g *Gateway) localModels(ctx context.Context, id, address string, fresh bool) (bool, []model, bool) {
	g.local.mu.Lock()
	found := g.local.found[id]
	if found == nil {
		found = &localFound{}
		g.local.found[id] = found
	}
	g.local.mu.Unlock()
	found.mu.Lock()
	defer found.mu.Unlock()
	now := g.now()
	if !fresh && found.address == address && !found.at.IsZero() && now.Sub(found.at) < localTTL {
		return found.running, found.models, false
	}
	running, models := g.askLocal(ctx, id, address)
	changed := !found.at.IsZero() && found.address == address && (running != found.running || !sameModels(models, found.models))
	found.address, found.at, found.running, found.models = address, now, running, models
	return running, models, changed
}

// askLocal reads GET {address}/models, the OpenAI listing Ollama and
// llama-server both serve, then asks each model what it can do. A server
// that does not answer is not running, which is never an error.
func (g *Gateway) askLocal(ctx context.Context, providerID, address string) (bool, []model) {
	ctx, cancel := context.WithTimeout(ctx, localAskBudget)
	defer cancel()
	root := strings.TrimSuffix(strings.TrimRight(address, "/"), "/v1")
	var listing llamaListing
	if providerID == protocol.ProviderOllamaLocal {
		if !g.localJSON(ctx, http.MethodGet, strings.TrimRight(address, "/")+"/models", nil, &listing) {
			return false, nil
		}
	} else if !g.localJSON(ctx, http.MethodGet, root+"/models", nil, &listing) &&
		!g.localJSON(ctx, http.MethodGet, strings.TrimRight(address, "/")+"/models", nil, &listing) {
		return false, nil
	}
	var loaded map[string]int
	if providerID == protocol.ProviderOllamaLocal {
		loaded = g.ollamaLoaded(ctx, root)
	}
	models := make([]model, 0, len(listing.Data))
	for _, entry := range listing.Data {
		id := strings.TrimSpace(entry.ID)
		// an embedding model is listed beside the chat models and answers no chat
		if id == "" || strings.Contains(strings.ToLower(id), "embed") || slices.ContainsFunc(models, func(m model) bool { return m.ID == id }) {
			continue
		}
		entry.ID = id
		m := llamaModel(entry)
		switch providerID {
		case protocol.ProviderOllamaLocal:
			var show ollamaShow
			body, _ := json.Marshal(map[string]string{"model": id})
			if g.localJSON(ctx, http.MethodPost, root+"/api/show", body, &show) {
				applyOllama(&m, show, loaded[id])
			}
		default:
			// a router lists models it has not loaded, and asking one for
			// its props would load it
			props := root + "/props"
			if entry.Status != nil {
				if entry.Status.Value != "loaded" {
					m.Light = m.Params == 0 || m.Params <= protocol.LightParams
					models = append(models, m)
					continue
				}
				props += "?model=" + url.QueryEscape(id) + "&autoload=false"
			}
			var answer llamaProps
			if g.localJSON(ctx, http.MethodGet, props, nil, &answer) {
				applyProps(&m, answer, entry.Architecture != nil)
			}
		}
		m.Light = m.Params == 0 || m.Params <= protocol.LightParams
		models = append(models, m)
	}
	return true, models
}

// managedWindow reads the loaded model's real allocation. The inventory
// advertises what its weights support, not what llama-server actually got.
// Ask the named router model without letting /props load a different one.
func (g *Gateway) managedWindow(ctx context.Context, baseURL, bearer, modelID string) int {
	ctx, cancel := context.WithTimeout(ctx, localAskBudget)
	defer cancel()
	root := strings.TrimSuffix(strings.TrimRight(baseURL, "/"), "/v1")
	query := url.Values{"model": {modelID}, "autoload": {"false"}}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, root+"/props?"+query.Encode(), nil)
	if err != nil {
		return 0
	}
	request.Header.Set("Accept", "application/json")
	if bearer != "" {
		request.Header.Set("Authorization", "Bearer "+bearer)
	}
	response, err := g.local.client.Do(request)
	if err != nil {
		return 0
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return 0
	}
	var props llamaProps
	if json.NewDecoder(io.LimitReader(response.Body, 4<<20)).Decode(&props) != nil {
		return 0
	}
	return props.contextWindow()
}

// localJSON asks a local server for JSON and reads it into out; false is a
// server that did not answer, refused, or said something else.
func (g *Gateway) localJSON(ctx context.Context, method, endpoint string, body []byte, out any) bool {
	var reader io.Reader
	if body != nil {
		reader = bytes.NewReader(body)
	}
	request, err := http.NewRequestWithContext(ctx, method, endpoint, reader)
	if err != nil {
		return false
	}
	request.Header.Set("Accept", "application/json")
	if body != nil {
		request.Header.Set("Content-Type", "application/json")
	}
	response, err := g.local.client.Do(request)
	if err != nil {
		return false
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return false
	}
	return json.NewDecoder(io.LimitReader(response.Body, 4<<20)).Decode(out) == nil
}

// llamaListing is GET /v1/models as llama-server and Ollama answer it.
// llama-server adds what the model reads (architecture, from the projector,
// even for a model not loaded) and its size and window (meta, null while it
// loads); a router adds each model's status.
type llamaListing struct {
	Data []llamaEntry `json:"data"`
}

type llamaEntry struct {
	ID           string `json:"id"`
	Architecture *struct {
		InputModalities []string `json:"input_modalities"`
	} `json:"architecture"`
	Meta *struct {
		NCtx      int   `json:"n_ctx"`
		NCtxTrain int   `json:"n_ctx_train"`
		NParams   int64 `json:"n_params"`
	} `json:"meta"`
	Status *struct {
		Value string `json:"value"`
	} `json:"status"`
}

// llamaProps is GET /props: the window the server actually runs, and what
// the model's chat template can do.
type llamaProps struct {
	DefaultGenerationSettings struct {
		NCtx int `json:"n_ctx"`
	} `json:"default_generation_settings"`
	NCtx      int `json:"n_ctx"`
	ModelInfo struct {
		NCtx int `json:"n_ctx"`
	} `json:"model_info"`
	ChatTemplateCaps map[string]json.RawMessage `json:"chat_template_caps"`
	Modalities       struct {
		Vision bool `json:"vision"`
		Audio  bool `json:"audio"`
	} `json:"modalities"`
}

func (props llamaProps) contextWindow() int {
	if props.DefaultGenerationSettings.NCtx > 0 {
		return props.DefaultGenerationSettings.NCtx
	}
	if props.NCtx > 0 {
		return props.NCtx
	}
	if props.ModelInfo.NCtx > 0 {
		return props.ModelInfo.NCtx
	}
	return 0
}

// llamaModel reads one listed model. Tools are assumed until the server's
// props say otherwise: llama-server renders tools with --jinja, on by
// default.
func llamaModel(entry llamaEntry) model {
	m := model{ID: entry.ID, Name: localModelName(entry.ID), Tools: true}
	if entry.Architecture != nil {
		m.Vision = slices.Contains(entry.Architecture.InputModalities, "image")
		m.Audio = slices.Contains(entry.Architecture.InputModalities, "audio")
	}
	if entry.Meta != nil {
		m.Params = entry.Meta.NParams
		m.Context = entry.Meta.NCtx
		if m.Context == 0 {
			m.Context = entry.Meta.NCtxTrain
		}
	}
	return m
}

// applyProps takes the window the server runs, which may be smaller than the
// model's, and whether its template calls tools. A build too old to say
// keeps tools on; vision comes from props only when the listing had no
// architecture.
func applyProps(m *model, props llamaProps, listed bool) {
	if n := props.contextWindow(); n > 0 {
		m.Context = n
	}
	if len(props.ChatTemplateCaps) > 0 {
		var tools bool
		if raw, ok := props.ChatTemplateCaps["supports_tool_calls"]; ok && json.Unmarshal(raw, &tools) == nil {
			m.Tools = tools
		}
		var levels []string
		if raw, ok := props.ChatTemplateCaps["reasoning_effort_levels"]; ok && json.Unmarshal(raw, &levels) == nil && len(levels) > 0 {
			m.Reasoning = true
		}
	}
	if !listed {
		m.Vision = m.Vision || props.Modalities.Vision
		m.Audio = m.Audio || props.Modalities.Audio
	}
}

// ollamaShow is POST /api/show.
type ollamaShow struct {
	Capabilities []string       `json:"capabilities"`
	ModelInfo    map[string]any `json:"model_info"`
	Details      struct {
		ParameterSize string `json:"parameter_size"`
	} `json:"details"`
	Parameters string `json:"parameters"`
}

// applyOllama takes what Ollama says of a model. Its window is the one the
// loaded model runs with, else num_ctx from the model file, else Ollama's
// default; never more than the model was trained for.
func applyOllama(m *model, show ollamaShow, loaded int) {
	m.Vision = slices.Contains(show.Capabilities, "vision")
	m.Tools = slices.Contains(show.Capabilities, "tools")
	m.Reasoning = slices.Contains(show.Capabilities, "thinking")
	m.Audio = slices.Contains(show.Capabilities, "audio")
	arch, _ := show.ModelInfo["general.architecture"].(string)
	trained := int(number(show.ModelInfo[arch+".context_length"]))
	m.Params = int64(number(show.ModelInfo["general.parameter_count"]))
	if m.Params == 0 {
		m.Params = parameterSize(show.Details.ParameterSize)
	}
	window := loaded
	if window == 0 {
		window = numCtx(show.Parameters)
	}
	if window == 0 {
		window = ollamaContext
	}
	if trained > 0 && trained < window {
		window = trained
	}
	m.Context = window
}

// ollamaLoaded is the window of each model Ollama has loaded now, by name.
func (g *Gateway) ollamaLoaded(ctx context.Context, root string) map[string]int {
	var ps struct {
		Models []struct {
			Name          string `json:"name"`
			Model         string `json:"model"`
			ContextLength int    `json:"context_length"`
		} `json:"models"`
	}
	if !g.localJSON(ctx, http.MethodGet, root+"/api/ps", nil, &ps) {
		return nil
	}
	loaded := make(map[string]int, len(ps.Models))
	for _, m := range ps.Models {
		if m.ContextLength > 0 {
			loaded[m.Name] = m.ContextLength
			loaded[m.Model] = m.ContextLength
		}
	}
	return loaded
}

func number(value any) float64 {
	switch typed := value.(type) {
	case float64:
		return typed
	case json.Number:
		n, _ := typed.Float64()
		return n
	}
	return 0
}

// numCtx reads num_ctx from Ollama's parameters, one "name value" a line.
func numCtx(parameters string) int {
	for _, line := range strings.Split(parameters, "\n") {
		fields := strings.Fields(line)
		if len(fields) == 2 && fields[0] == "num_ctx" {
			if n, err := strconv.Atoi(fields[1]); err == nil && n > 0 {
				return n
			}
		}
	}
	return 0
}

// parameterSize reads Ollama's "2B", "7.6B" or "270M".
func parameterSize(size string) int64 {
	size = strings.ToUpper(strings.TrimSpace(size))
	scale := 1.0
	switch {
	case strings.HasSuffix(size, "B"):
		scale = 1e9
	case strings.HasSuffix(size, "M"):
		scale = 1e6
	case strings.HasSuffix(size, "K"):
		scale = 1e3
	default:
		return 0
	}
	n, err := strconv.ParseFloat(size[:len(size)-1], 64)
	if err != nil || n <= 0 {
		return 0
	}
	return int64(n * scale)
}

// localModelName reads a model's id as a name: llama-server may give the
// model file's path, Ollama a tag that is usually latest.
func localModelName(id string) string {
	name := strings.TrimSuffix(path.Base(id), ".gguf")
	return strings.TrimSuffix(name, ":latest")
}

func sameModels(a, b []model) bool {
	return slices.EqualFunc(a, b, func(left, right model) bool {
		return left.ID == right.ID && left.Name == right.Name && left.Vision == right.Vision &&
			left.Tools == right.Tools && left.Audio == right.Audio && left.Light == right.Light &&
			left.Reasoning == right.Reasoning && left.Context == right.Context && left.Params == right.Params &&
			left.ToolDialect == right.ToolDialect && left.ReplayReasoning == right.ReplayReasoning
	})
}
