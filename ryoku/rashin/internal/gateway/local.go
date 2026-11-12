package gateway

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/netip"
	"path"
	"slices"
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
			ResponseHeaderTimeout: firstByteBudget,
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

// localProviders builds the local providers from the settings' addresses and
// what their servers answered. With fresh set the servers are asked now;
// otherwise an answer younger than localTTL stands.
func (g *Gateway) localProviders(ctx context.Context, fresh bool) []provider {
	addresses := g.settings().Providers.Local
	defaults := protocol.DefaultLocalAddresses()
	providers := make([]provider, 0, len(protocol.LocalProviders))
	changed := false
	for _, id := range protocol.LocalProviders {
		address := addresses[id]
		if address == "" {
			address = defaults[id]
		}
		running, models, flipped := g.localModels(ctx, id, address, fresh)
		changed = changed || flipped
		p := localProvider(id)
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
	case protocol.ProviderOllamaLocal:
		p.Name, p.Signup, p.Note = "Ollama", "https://ollama.com/download", i18n.T("Install Ollama and run ollama serve.")
	case protocol.ProviderLlamaCpp:
		p.Name, p.Signup, p.Note = "llama.cpp", "https://github.com/ggml-org/llama.cpp", i18n.T("Run llama-server -m with your model file, such as model.gguf.")
	}
	return p
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
	running, models := g.askLocal(ctx, address)
	changed := !found.at.IsZero() && found.address == address && (running != found.running || !sameModels(models, found.models))
	found.address, found.at, found.running, found.models = address, now, running, models
	return running, models, changed
}

// askLocal reads GET {address}/models, the OpenAI listing Ollama and
// llama-server both serve. A server that does not answer is not running,
// which is never an error.
func (g *Gateway) askLocal(ctx context.Context, address string) (bool, []model) {
	ctx, cancel := context.WithTimeout(ctx, localAskBudget)
	defer cancel()
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, strings.TrimRight(address, "/")+"/models", nil)
	if err != nil {
		return false, nil
	}
	request.Header.Set("Accept", "application/json")
	response, err := g.local.client.Do(request)
	if err != nil {
		return false, nil
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return false, nil
	}
	var listing struct {
		Data []struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	if err := json.NewDecoder(io.LimitReader(response.Body, 1<<20)).Decode(&listing); err != nil {
		return false, nil
	}
	models := make([]model, 0, len(listing.Data))
	for _, entry := range listing.Data {
		id := strings.TrimSpace(entry.ID)
		// an embedding model is listed beside the chat models and answers no chat
		if id == "" || strings.Contains(strings.ToLower(id), "embed") || slices.ContainsFunc(models, func(m model) bool { return m.ID == id }) {
			continue
		}
		// tools are assumed: a model that refuses them cools down like any other
		models = append(models, model{ID: id, Name: localModelName(id), Tools: true})
	}
	return true, models
}

// localModelName reads a model's id as a name: llama-server may give the
// model file's path, Ollama a tag that is usually latest.
func localModelName(id string) string {
	name := strings.TrimSuffix(path.Base(id), ".gguf")
	return strings.TrimSuffix(name, ":latest")
}

func sameModels(a, b []model) bool {
	return slices.EqualFunc(a, b, func(left, right model) bool { return left.ID == right.ID })
}
