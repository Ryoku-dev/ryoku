package harness

import (
	"context"
	"maps"
	"net/url"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"ryoku-i18n"
	"ryoku-rashin/internal/config"
	"ryoku-rashin/internal/core"
	"ryoku-rashin/internal/paths"
	"ryoku-rashin/internal/protocol"
	"ryoku-rashin/internal/sandbox"
)

// probeTimeout bounds a whole probe, from starting the harness to picking its
// last option.
var probeTimeout = 30 * time.Second

// Probe starts a harness once the way a team task would, sandboxed in a
// folder of its own under the cache, opens a session without a prompt, picks
// the model and options given and stops it. What the harness reports is
// recorded as for any start. One probe runs per harness at a time.
func (r *Registry) Probe(ctx context.Context, params protocol.HarnessProbeParams, base sandbox.Spec) (protocol.HarnessProbeResult, error) {
	settings, err := config.Load()
	if err != nil {
		return protocol.HarnessProbeResult{}, err
	}
	id := strings.TrimSpace(params.ID)
	if !Known(id, settings) {
		return protocol.HarnessProbeResult{}, protocol.Errorf(protocol.CodeBadRequest, "%s", i18n.Tf("No harness %s on this computer.", id))
	}
	if !protocol.ValidEngineModel(params.Model) {
		return protocol.HarnessProbeResult{}, protocol.Errorf(protocol.CodeBadRequest, "%s", i18n.Tf("A model id has at most %d characters and no spaces.", protocol.EngineModelMax))
	}
	if !protocol.ValidEngineOptions(params.Options) {
		return protocol.HarnessProbeResult{}, protocol.Errorf(protocol.CodeBadRequest, "%s", i18n.Tf("A harness takes at most %d options, each with an id and a value.", protocol.EngineOptionsMax))
	}
	if !r.claimProbe(id) {
		return protocol.HarnessProbeResult{}, protocol.Errorf(protocol.CodeConflict, "%s", i18n.T("This harness is being checked already. Try again when it finishes."))
	}
	defer r.releaseProbe(id)
	ok, message := r.probe(ctx, id, params, base)
	result := protocol.HarnessProbeResult{OK: ok, Message: message}
	if info, found := r.info(ctx, id); found {
		result.Harness = info
	}
	return result, nil
}

func (r *Registry) claimProbe(id string) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.probing[id] {
		return false
	}
	r.probing[id] = true
	return true
}

func (r *Registry) releaseProbe(id string) {
	r.mu.Lock()
	delete(r.probing, id)
	r.mu.Unlock()
}

// probe runs the harness and says how it went, in the harness's own words
// when it refused.
func (r *Registry) probe(ctx context.Context, id string, params protocol.HarnessProbeParams, base sandbox.Spec) (bool, string) {
	info, _ := r.info(ctx, id)
	name := info.Name
	if name == "" {
		name = id
	}
	if !info.Installed {
		return false, i18n.Tf("%s is not installed.", name)
	}
	dir := probeDir(id)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return false, err.Error()
	}
	defer os.RemoveAll(dir)
	ctx, cancel := context.WithTimeout(ctx, probeTimeout)
	defer cancel()
	spec := base
	spec.Folder = paths.Resolved(dir)
	agent, err := r.NewAgent(ctx, core.HarnessOptions{
		Session:   protocol.SessionInfo{Kind: protocol.KindAgent, Harness: id, Cwd: dir, Origin: protocol.OriginTeam},
		Harness:   id,
		Cwd:       dir,
		Approvals: func() string { return protocol.ApprovalsReadOnly },
		Sandbox:   &spec,
	})
	if err != nil {
		return false, err.Error()
	}
	defer agent.Stop()
	sink := probeSink{}
	failed := func(err error) string {
		if ctx.Err() != nil {
			return i18n.Tf("%s did not answer within %d seconds.", name, int(probeTimeout/time.Second))
		}
		return err.Error()
	}
	if err := agent.Start(ctx, sink); err != nil {
		return false, failed(err)
	}
	learned, _ := r.info(ctx, id)
	catalog := protocol.HarnessCapabilities{}
	if learned.Capabilities != nil {
		catalog = *learned.Capabilities
	}
	var took []string
	if params.Model != "" {
		model := choiceName(catalog.ModelList, params.Model)
		if err := agent.SetModel(ctx, params.Model, sink); err != nil {
			return false, i18n.Tf("%s did not take the model %s. %s", name, model, failed(err))
		}
		took = append(took, model)
	}
	for _, option := range slices.Sorted(maps.Keys(params.Options)) {
		value := params.Options[option]
		picked := optionPick(catalog.Options, option, value)
		if err := agent.SetOption(ctx, option, value, sink); err != nil {
			return false, i18n.Tf("%s did not take %s. %s", name, picked, failed(err))
		}
		took = append(took, picked)
	}
	if len(took) != 0 {
		return true, i18n.Tf("%s started in the sandbox and took %s.", name, strings.Join(took, ", "))
	}
	if catalog.DefaultModel != "" {
		return true, i18n.Tf("%s started in the sandbox on %s.", name, choiceName(catalog.ModelList, catalog.DefaultModel))
	}
	return true, i18n.Tf("%s started in the sandbox.", name)
}

// info is one harness as List shows it.
func (r *Registry) info(ctx context.Context, id string) (protocol.HarnessInfo, bool) {
	list, err := r.List(ctx, false)
	if err != nil {
		return protocol.HarnessInfo{}, false
	}
	for _, info := range list {
		if info.ID == id {
			return info, true
		}
	}
	return protocol.HarnessInfo{}, false
}

// probeDir is the folder a probe of id runs in. An instance id is the user's
// to choose, so it is escaped to stay one name inside the probe folder.
func probeDir(id string) string {
	name := url.PathEscape(id)
	if name == "." || name == ".." {
		name = strings.ReplaceAll(name, ".", "%2E")
	}
	return filepath.Join(paths.CacheDir(), "probe", name)
}

func choiceName(choices []protocol.Choice, id string) string {
	for _, choice := range choices {
		if choice.ID == id && choice.Name != "" {
			return choice.Name
		}
	}
	return id
}

// optionPick reads a picked option as the app shows it: "effort High".
func optionPick(options []protocol.SessionOption, id, value string) string {
	for _, option := range options {
		if option.ID == id {
			return strings.ToLower(option.Name) + " " + choiceName(option.Choices, value)
		}
	}
	return id + " " + value
}

// probeSink takes what a probed harness says and keeps none of it: a probe
// has no session to show it in, and its result is read from the catalog.
type probeSink struct{}

func (probeSink) Emit(string, any) {}

func (probeSink) Update(func(*protocol.SessionInfo)) {}
