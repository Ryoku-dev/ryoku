package engine

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"maps"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"unicode/utf8"

	"ryoku-rashin/internal/core"
	"ryoku-rashin/internal/engine/exec"
	"ryoku-rashin/internal/engine/tool"
	"ryoku-rashin/internal/protocol"
	"ryoku-rashin/internal/sandbox"
	"ryoku-rashin/internal/vault"
)

const runtimeOutputLimit = 256 << 10
const runtimeWireLimit = 1 << 20

// runtimeRunner adapts configured external work to the existing session and
// team lifecycle. It never persists credentials, output, permissions or a PID.
// Each turn owns its processes; ACP resumes its opaque session on the next turn.
type runtimeRunner struct {
	f         *Factory
	opts      core.AgentOptions
	config    protocol.MemberRuntime
	stamp     string
	gate      *gate
	mu        sync.Mutex
	cancel    context.CancelFunc
	done      chan struct{}
	stopping  bool
	stopErr   error
	executor  exec.Executor
	model     string
	session   string
	mode      string
	baseScrub func(string) string
	scrub     func(string) string
}

type runtimeState struct {
	Version       int    `json:"version"`
	Kind          string `json:"kind"`
	Configuration string `json:"configuration"`
	Session       string `json:"session,omitempty"`
	Model         string `json:"model,omitempty"`
}

func (f *Factory) newRuntime(opts core.AgentOptions, config protocol.MemberRuntime) (core.Runner, error) {
	if opts.Tools.Team == nil || opts.Tools.Team.Token == "" || f.deps.Host == nil {
		return nil, errors.New("external runtime requires an authenticated team run")
	}
	switch config.Kind {
	case protocol.MemberRuntimeCLI, protocol.MemberRuntimeCommand:
		if len(config.Command) == 0 || config.Command[0] == "" {
			return nil, errors.New("external runtime requires a configured argv")
		}
		if opts.Sandbox == nil || f.deps.Executor == nil {
			return nil, errors.New("external runtime requires the team sandbox and executor")
		}
	case protocol.MemberRuntimeWebhook:
		if opts.RuntimeHTTP == nil || config.URL == "" {
			return nil, errors.New("webhook runtime requires its configured HTTP boundary")
		}
	default:
		return nil, errors.New("unsupported member runtime")
	}
	if len(config.SecretEnv) > 0 && opts.RuntimeEnvironment == nil {
		return nil, errors.New("runtime vault environment resolver is unavailable")
	}
	if opts.Cwd == "" {
		opts.Cwd = opts.Session.Cwd
	}
	if !filepath.IsAbs(opts.Cwd) {
		return nil, errors.New("runtime working directory must be absolute")
	}
	if opts.Approvals == nil {
		opts.Approvals = func() string { return opts.Session.Mode }
	}
	if len(config.Model) > 512 || strings.ContainsAny(config.Model, "\x00\r\n") {
		return nil, errors.New("invalid configured external model ID")
	}
	config.Command = slices.Clone(config.Command)
	config.Env = maps.Clone(config.Env)
	config.SecretEnv = maps.Clone(config.SecretEnv)
	encoded, _ := json.Marshal(config)
	digest := sha256.Sum256(encoded)
	r := &runtimeRunner{f: f, opts: opts, config: config, stamp: hex.EncodeToString(digest[:]), model: config.Model}
	r.baseScrub = vault.ExactValuesScrubber(map[string]string{opts.Tools.Team.Token: "redacted"})
	r.scrub = r.baseScrub
	r.gate = newGate(r.approvals, false)
	var state runtimeState
	if len(opts.Resume) <= 8192 && json.Unmarshal(opts.Resume, &state) == nil && state.Version == 1 && state.Kind == config.Kind && state.Configuration == r.stamp && runtimeOpaqueID(state.Session) && len(state.Model) <= 512 {
		r.session, r.model = state.Session, state.Model
	}
	return r, nil
}

func runtimeOpaqueID(id string) bool {
	if len(id) > 512 {
		return false
	}
	for _, c := range id {
		if !(c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || strings.ContainsRune("-_.:/", c)) {
			return false
		}
	}
	return true
}

func (r *runtimeRunner) approvals() string {
	r.mu.Lock()
	mode := r.mode
	r.mu.Unlock()
	return protocol.MemberApprovals(r.opts.Approvals(), mode)
}

func (r *runtimeRunner) Prompt(ctx context.Context, in core.PromptInput, sink core.Sink) (result core.TurnResult, err error) {
	result.Usage = &protocol.TokenUsage{Unmetered: true}
	ctx, cancel := context.WithCancel(ctx)
	r.mu.Lock()
	if r.done != nil || r.stopping {
		r.mu.Unlock()
		cancel()
		result.Stop = protocol.StopError
		return result, errors.New("runtime is already running or stopping")
	}
	done := make(chan struct{})
	r.done, r.cancel, r.stopErr = done, cancel, nil
	r.mu.Unlock()
	r.gate.bind(sink)
	defer func() {
		interrupted := ctx.Err() != nil
		cancel()
		r.gate.cancelAll()
		r.mu.Lock()
		x := r.executor
		r.executor = nil
		r.mu.Unlock()
		var closeErr error
		if x != nil {
			closeErr = x.Close()
		}
		if closeErr != nil && err == nil {
			err = errors.New("runtime sandbox processes could not be stopped")
		}
		r.mu.Lock()
		if closeErr != nil {
			r.stopErr = errors.New("runtime sandbox processes could not be stopped")
		}
		r.scrub = r.baseScrub
		r.done, r.cancel = nil, nil
		close(done)
		r.mu.Unlock()
		if err != nil {
			result.Stop = protocol.StopError
			if interrupted {
				result.Stop = protocol.StopCancelled
			}
		}
	}()
	if err = ctx.Err(); err != nil {
		return result, err
	}
	if len(in.Images) > 0 || in.Run != "" || in.Glance != nil || in.Terminal != nil || in.Handoff != nil {
		return result, errors.New("external runtime requires text-only task input; images, terminal snapshots, handoffs and ad-hoc commands are unsupported")
	}
	if len(in.Text) > runtimeOutputLimit {
		return result, errors.New("runtime task text exceeds 256 KiB")
	}
	// External code cannot be forced to use permission callbacks. Approval is
	// for the whole configured process within its sandbox, not a claim that
	// an agent's self-reported permission mode is an enforcement boundary.
	tier := protocol.TierWrite
	if r.config.Kind != protocol.MemberRuntimeWebhook && r.approvals() == protocol.ApprovalsReadOnly {
		tier = protocol.TierRead
	}
	if err = r.allow(ctx, tier, "Run configured "+r.config.Kind+" runtime within team policy"); err != nil {
		return result, err
	}
	if r.config.Kind == protocol.MemberRuntimeWebhook {
		return r.webhook(ctx, in, sink)
	}
	environment, err := r.environment(ctx)
	if err != nil {
		return result, err
	}
	spec := r.processSpec()
	if r.config.Kind == protocol.MemberRuntimeCLI {
		return r.cli(ctx, in, sink, spec, environment)
	}
	return r.command(ctx, in, sink, spec, environment)
}

func (r *runtimeRunner) allow(ctx context.Context, tier, title string) error {
	if r.approvals() == protocol.ApprovalsReadOnly && tier != protocol.TierRead {
		return errors.New("runtime action is forbidden by read-only policy")
	}
	decision, err := r.gate.Allow(ctx, tool.Request{Title: title, Tier: tier})
	if err != nil {
		return err
	}
	if !decision.Allowed {
		return errors.New("runtime action was refused")
	}
	return nil
}

// processSpec preserves ordinary networking and filesystem hiding. Read-only
// foreign code is offline as well as persistently read-only: it cannot be
// trusted to consult our action gate before making its own network writes.
func (r *runtimeRunner) processSpec() *sandbox.Spec {
	original := r.opts.Sandbox
	spec := *original
	spec.Hidden = slices.Clone(original.Hidden)
	spec.ReadOnly = slices.Clone(original.ReadOnly)
	spec.Writable = slices.Clone(original.Writable)
	spec.Unset = slices.Clone(original.Unset)
	spec.Binds = nil
	spec.Env = maps.Clone(original.Env)
	if spec.Env == nil {
		spec.Env = map[string]string{}
	}
	delete(spec.Env, "HYAKKI_SOCKET")
	spec.Unset = append(spec.Unset, "HYAKKI_SOCKET")
	if r.approvals() == protocol.ApprovalsReadOnly {
		spec.Offline = true
		if spec.Folder != "" {
			spec.ReadOnly = append(spec.ReadOnly, spec.Folder)
		}
		spec.ReadOnly = append(spec.ReadOnly, spec.Writable...)
		spec.Dir, spec.Folder, spec.Writable = r.opts.Cwd, "", nil
		spec.Env["HOME"] = "/tmp"
		spec.Env["XDG_CONFIG_HOME"] = "/tmp/.config"
		spec.Env["XDG_DATA_HOME"] = "/tmp/.local/share"
		spec.Env["XDG_STATE_HOME"] = "/tmp/.local/state"
		spec.Env["XDG_CACHE_HOME"] = "/tmp/.cache"
	}
	return &spec
}

func runtimeEnvKey(key string) bool {
	if key == "" {
		return false
	}
	for i, c := range key {
		if !(c == '_' || c >= 'A' && c <= 'Z' || c >= 'a' && c <= 'z' || i > 0 && c >= '0' && c <= '9') {
			return false
		}
	}
	// Loader/interpreter configuration can subvert the trusted sandbox scope
	// wrapper before it restricts itself. Auth credentials must not set it.
	return !strings.HasPrefix(key, "LD_") && !strings.HasPrefix(key, "DYLD_") && !strings.HasPrefix(key, "HYAKKI_") && !strings.HasPrefix(key, "RASHIN_") && key != "GCONV_PATH" && key != "LOCPATH" && key != "GLIBC_TUNABLES"
}

func (r *runtimeRunner) environment(ctx context.Context) (map[string]string, error) {
	env := maps.Clone(r.config.Env)
	if env == nil {
		env = map[string]string{}
	}
	for key, value := range env {
		if !runtimeEnvKey(key) || strings.ContainsRune(value, 0) {
			return nil, errors.New("runtime environment contains an unsafe key or value")
		}
	}
	resolved := map[string]string{}
	if r.opts.RuntimeEnvironment != nil {
		var err error
		resolved, err = r.opts.RuntimeEnvironment(ctx)
		if err != nil {
			return nil, errors.New("runtime vault environment could not be resolved")
		}
	}
	if len(r.config.SecretEnv) == 0 {
		return env, nil
	}
	secrets := make(map[string]string, len(r.config.SecretEnv)+1)
	secrets[r.opts.Tools.Team.Token] = "redacted"
	for key := range r.config.SecretEnv {
		value, ok := resolved[key]
		if !ok || value == "" || !runtimeEnvKey(key) || strings.ContainsRune(value, 0) {
			return nil, errors.New("runtime vault environment is incomplete or unsafe")
		}
		env[key] = value
		secrets[value] = "redacted"
	}
	scrub := vault.ExactValuesScrubber(secrets)
	r.mu.Lock()
	r.scrub = scrub
	r.mu.Unlock()
	return env, nil
}

func (r *runtimeRunner) redact(text string) string {
	r.mu.Lock()
	scrub := r.scrub
	r.mu.Unlock()
	return scrub(text)
}

func runtimeClip(text string, limit int) string {
	if len(text) <= limit {
		return strings.ToValidUTF8(text, "�")
	}
	text = text[:limit]
	for !utf8.ValidString(text) && len(text) > 0 {
		text = text[:len(text)-1]
	}
	return text + "\n[output truncated]"
}

func (r *runtimeRunner) getExecutor(spec *sandbox.Spec) exec.Executor {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.executor == nil {
		r.executor = r.f.deps.Executor(r.opts.Session, spec)
	}
	return r.executor
}

func (r *runtimeRunner) report(ctx context.Context, title, body string, files []string, quiet bool) error {
	if strings.TrimSpace(body) == "" && !quiet {
		return errors.New("runtime ended without a work report")
	}
	if len(files) > 128 {
		return errors.New("runtime report contains too many files")
	}
	for _, file := range files {
		if len(file) > 4096 || strings.ContainsRune(file, 0) {
			return errors.New("runtime report contains an invalid file reference")
		}
	}
	cleanFiles := make([]string, len(files))
	for i, file := range files {
		cleanFiles[i] = r.redact(file)
	}
	host := r.f.deps.Host(r.opts.Session)
	if host == nil {
		return errors.New("runtime report host is unavailable")
	}
	err := host.Call(ctx, protocol.MTeamsAgentReport, protocol.AgentReportParams{
		AgentRef: protocol.AgentRef{Session: r.opts.Session.ID, Token: r.opts.Tools.Team.Token},
		Title:    runtimeClip(r.redact(title), 512), Body: runtimeClip(r.redact(body), runtimeOutputLimit), Files: cleanFiles, Quiet: quiet,
	}, nil)
	if err != nil {
		return errors.New("authenticated runtime report was rejected or failed")
	}
	return nil
}

func (r *runtimeRunner) emitText(sink core.Sink, text string) {
	if sink != nil && text != "" {
		sink.Emit(protocol.EvText, protocol.TextData{Message: "runtime-result", Delta: runtimeClip(r.redact(text), runtimeOutputLimit)})
	}
}

func (r *runtimeRunner) Cancel() {
	r.mu.Lock()
	cancel := r.cancel
	r.mu.Unlock()
	if cancel != nil {
		cancel()
	}
	r.gate.cancelAll()
}
func (r *runtimeRunner) Permission(request, option string) error {
	return r.gate.resolve(request, option)
}
func (r *runtimeRunner) Alive() bool { r.mu.Lock(); defer r.mu.Unlock(); return r.done != nil }
func (r *runtimeRunner) Stop() error {
	r.mu.Lock()
	r.stopping = true
	cancel, done := r.cancel, r.done
	r.mu.Unlock()
	if cancel != nil {
		cancel()
	}
	r.gate.cancelAll()
	if done != nil {
		<-done
	}
	r.mu.Lock()
	x, stopErr := r.executor, r.stopErr
	r.executor = nil
	r.mu.Unlock()
	if x != nil {
		if err := x.Close(); err != nil {
			stopErr = errors.New("runtime sandbox processes could not be stopped")
		}
	}
	r.mu.Lock()
	r.stopping = false
	r.mu.Unlock()
	return stopErr
}
func (r *runtimeRunner) State() json.RawMessage {
	r.mu.Lock()
	defer r.mu.Unlock()
	session, model := r.session, r.model
	if r.scrub(session) != session {
		session = ""
	}
	if r.scrub(model) != model {
		model = ""
	}
	encoded, _ := json.Marshal(runtimeState{Version: 1, Kind: r.config.Kind, Configuration: r.stamp, Session: session, Model: model})
	return encoded
}
func (r *runtimeRunner) SetModel(ctx context.Context, model string, sink core.Sink) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if r.config.Kind != protocol.MemberRuntimeCLI {
		return errors.New("this runtime has no selectable model")
	}
	if len(model) == 0 || len(model) > 512 || strings.ContainsAny(model, "\x00\r\n") {
		return errors.New("invalid external model ID")
	}
	r.mu.Lock()
	if r.done != nil {
		r.mu.Unlock()
		return errors.New("change external model between turns")
	}
	r.model = model
	r.mu.Unlock()
	if sink != nil {
		sink.Update(func(info *protocol.SessionInfo) { info.Model = model })
	}
	return nil
}
func (r *runtimeRunner) SetMode(ctx context.Context, mode string, sink core.Sink) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if !protocol.KnownApprovals(mode) || protocol.MemberApprovals(r.opts.Approvals(), mode) != mode {
		return errors.New("runtime mode cannot widen team policy")
	}
	r.mu.Lock()
	r.mode = mode
	cancel := r.cancel
	r.mu.Unlock()
	r.gate.forget()
	// Rebuild the sandbox at the narrower posture, never leave an existing
	// writable child alive while the interface advertises read-only.
	if cancel != nil {
		cancel()
	}
	if sink != nil {
		sink.Update(func(info *protocol.SessionInfo) { info.Mode = mode })
	}
	return nil
}
func (r *runtimeRunner) SetOption(context.Context, string, string, core.Sink) error {
	return errors.New("external runtime does not support native engine options")
}

// runtimeWork is a versioned stdin/HTTP work envelope, not an authority grant.
// The run token remains exclusively in report(), on the trusted side.
type runtimeWork struct {
	Version int    `json:"version"`
	Kind    string `json:"kind"`
	Session string `json:"session"`
	Team    string `json:"team"`
	Task    string `json:"task"`
	Member  string `json:"member"`
	Turn    int    `json:"turn"`
	Text    string `json:"text"`
}

func (r *runtimeRunner) work(in core.PromptInput) runtimeWork {
	t := r.opts.Tools.Team
	return runtimeWork{Version: 1, Kind: "task", Session: r.opts.Session.ID, Team: t.Team, Task: t.Task, Member: t.Member, Turn: in.Turn, Text: r.redact(in.Text)}
}

var _ core.Runner = (*runtimeRunner)(nil)
