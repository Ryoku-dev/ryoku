package harness

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"syscall"
	"time"

	"ryoku-i18n"
	"ryoku-rashin/internal/attachment"
	"ryoku-rashin/internal/core"
	"ryoku-rashin/internal/protocol"
)

const (
	clientVersion   = "1"
	stderrTailLimit = 8 << 10
	stopGrace       = 2 * time.Second
)

type resumeState struct {
	ACPSession string `json:"acpSession"`
	Load       bool   `json:"loadSession"`
	Generation uint64 `json:"generation,omitempty"`
}

type permissionRequest struct {
	id       json.RawMessage
	publicID string
	call     string
	options  []protocol.PermissionOption
	respond  func(*connection, string) error
	cancel   func(*connection) error
}

type trackedTool struct {
	kind   string
	status string
	order  uint64
}

type runner struct {
	opts core.HarnessOptions
	argv []string

	startMu  sync.Mutex
	promptMu sync.Mutex
	mu       sync.Mutex
	cmd      *exec.Cmd
	stdin    io.WriteCloser
	conn     *connection
	alive    bool
	stopping bool
	waitDone chan struct{}
	sink     core.Sink
	stderr   *tailBuffer

	environment        map[string]string
	reportCapabilities func(protocol.HarnessCapabilities)

	sessionID     string
	loadSession   bool
	loading       bool
	promptImage   bool
	modelOption   string
	modeOption    string
	models        []protocol.Choice
	modes         []protocol.Choice
	options       []protocol.SessionOption
	model         string
	mode          string
	usage         *protocol.TokenUsage
	turnCancelled bool

	permissions map[string]permissionRequest
	tools       map[string]trackedTool
	toolOrder   uint64
	terminals   map[string]*terminal

	generation uint64
	chunkKind  string
	chunkID    string
	chunkSeq   uint64
	synthetic  uint64
}

type runnerOption func(*runner)

func withEnvironment(environment map[string]string) runnerOption {
	return func(r *runner) {
		r.environment = make(map[string]string, len(environment))
		for name, value := range environment {
			r.environment[name] = value
		}
	}
}

func withCapabilityReporter(report func(protocol.HarnessCapabilities)) runnerOption {
	return func(r *runner) {
		r.reportCapabilities = report
	}
}

var _ core.Runner = (*runner)(nil)

func newRunner(opts core.HarnessOptions, argv []string, options ...runnerOption) *runner {
	r := &runner{
		opts:        opts,
		argv:        append([]string(nil), argv...),
		permissions: make(map[string]permissionRequest),
		tools:       make(map[string]trackedTool),
		terminals:   make(map[string]*terminal),
	}
	var resume resumeState
	if json.Unmarshal(opts.Resume, &resume) == nil {
		r.generation = resume.Generation
	}
	for _, option := range options {
		option(r)
	}
	return r
}

func (r *runner) Prompt(ctx context.Context, in core.PromptInput, sink core.Sink) (core.TurnResult, error) {
	r.promptMu.Lock()
	defer r.promptMu.Unlock()
	r.resolveAllPermissions("cancelled", "")
	if err := r.ensureStarted(ctx, sink); err != nil {
		return core.TurnResult{}, err
	}
	r.mu.Lock()
	sessionID := r.sessionID
	images := r.promptImage
	conn := r.conn
	r.usage = nil
	r.turnCancelled = false
	r.mu.Unlock()
	blocks := make([]map[string]any, 0, len(in.Images)+1)
	if in.Text != "" {
		blocks = append(blocks, map[string]any{"type": "text", "text": in.Text})
	}
	for _, attached := range in.Images {
		block, err := attachedBlock(attached, images)
		if err != nil {
			return core.TurnResult{}, err
		}
		blocks = append(blocks, block)
	}
	type promptReply struct {
		result json.RawMessage
		err    error
	}
	reply := make(chan promptReply, 1)
	go func() {
		result, err := conn.request(context.Background(), "session/prompt", map[string]any{
			"sessionId": sessionID,
			"prompt":    blocks,
		})
		reply <- promptReply{result: result, err: err}
	}()
	var responseRaw json.RawMessage
	select {
	case answered := <-reply:
		if answered.err != nil {
			return core.TurnResult{}, r.withAgentError(answered.err)
		}
		responseRaw = answered.result
	case <-ctx.Done():
		r.Cancel()
		timer := time.NewTimer(stopGrace)
		select {
		case answered := <-reply:
			if !timer.Stop() {
				<-timer.C
			}
			if answered.err != nil {
				return core.TurnResult{}, ctx.Err()
			}
			responseRaw = answered.result
		case <-timer.C:
			r.killAgentAfterCancel()
			return core.TurnResult{}, ctx.Err()
		}
	}
	var response struct {
		StopReason string          `json:"stopReason"`
		Usage      json.RawMessage `json:"usage"`
	}
	if err := json.Unmarshal(responseRaw, &response); err != nil {
		return core.TurnResult{}, fmt.Errorf("decode agent prompt result: %w", err)
	}
	r.mu.Lock()
	usage := cloneUsage(r.usage)
	r.mu.Unlock()
	if parsed := parseUsage(response.Usage); parsed != nil {
		usage = parsed
	}
	stop := response.StopReason
	if stop == "" {
		stop = protocol.StopEnd
	}
	return core.TurnResult{Stop: stop, Usage: usage}, nil
}

func (r *runner) Cancel() {
	r.mu.Lock()
	conn := r.conn
	sessionID := r.sessionID
	r.turnCancelled = true
	r.mu.Unlock()
	if conn != nil && sessionID != "" {
		_ = conn.notify("session/cancel", map[string]string{"sessionId": sessionID})
	}
	r.resolveAllPermissions("cancelled", "user")
}

func (r *runner) Permission(request, option string) error {
	r.mu.Lock()
	open, ok := r.permissions[request]
	if !ok {
		r.mu.Unlock()
		return errors.New(i18n.T("This permission request is no longer open. Try the action again."))
	}
	valid := false
	for _, candidate := range open.options {
		if candidate.ID == option {
			valid = true
			break
		}
	}
	if !valid {
		r.mu.Unlock()
		return errors.New(i18n.T("This permission option is not available. Choose one of the listed options."))
	}
	conn := r.conn
	if conn == nil {
		r.mu.Unlock()
		return errors.New(i18n.T("The agent connection closed. Start the turn again."))
	}
	delete(r.permissions, request)
	sink := r.sink
	r.mu.Unlock()
	if open.respond != nil {
		if err := open.respond(conn, option); err != nil {
			return err
		}
	} else if err := conn.respond(open.id, selectedPermission(option)); err != nil {
		return err
	}
	if sink != nil {
		sink.Emit(protocol.EvPermissionResolved, protocol.PermissionResolvedData{
			Request: request, Option: option, Outcome: "selected", By: "user",
		})
	}
	return nil
}

func (r *runner) SetModel(ctx context.Context, model string, sink core.Sink) error {
	if err := r.ensureStarted(ctx, sink); err != nil {
		return err
	}
	r.mu.Lock()
	sessionID, option := r.sessionID, r.modelOption
	r.mu.Unlock()
	method := "session/set_model"
	params := map[string]any{"sessionId": sessionID, "modelId": model}
	if option != "" {
		method = "session/set_config_option"
		params = map[string]any{"sessionId": sessionID, "configId": option, "value": model}
	}
	result, err := r.conn.request(ctx, method, params)
	if err != nil {
		return r.withAgentError(err)
	}
	if method == "session/set_config_option" && len(result) != 0 {
		var response struct {
			ConfigOptions []configOption `json:"configOptions"`
		}
		if json.Unmarshal(result, &response) == nil {
			r.applyConfigOptions(response.ConfigOptions)
		}
	}
	r.mu.Lock()
	// the agent's own answer may have said so already
	settled := r.model == model
	r.model = model
	choices := append([]protocol.Choice(nil), r.models...)
	r.mu.Unlock()
	if !settled {
		r.emit(protocol.EvModel, protocol.ChoiceData{Current: model, Options: choices})
		r.updateInfo(func(info *protocol.SessionInfo) { info.Model = model })
	}
	return nil
}

func (r *runner) SetMode(ctx context.Context, mode string, sink core.Sink) error {
	if err := r.ensureStarted(ctx, sink); err != nil {
		return err
	}
	r.mu.Lock()
	sessionID, option := r.sessionID, r.modeOption
	r.mu.Unlock()
	if option != "" {
		return r.setConfigOption(ctx, sessionID, option, mode)
	}
	if _, err := r.conn.request(ctx, "session/set_mode", map[string]any{"sessionId": sessionID, "modeId": mode}); err != nil {
		return r.withAgentError(err)
	}
	r.mu.Lock()
	r.mode = mode
	choices := append([]protocol.Choice(nil), r.modes...)
	r.mu.Unlock()
	r.emit(protocol.EvMode, protocol.ChoiceData{Current: mode, Options: choices})
	r.updateInfo(func(info *protocol.SessionInfo) { info.Mode = mode })
	return nil
}

func (r *runner) SetOption(ctx context.Context, option, value string, sink core.Sink) error {
	if err := r.ensureStarted(ctx, sink); err != nil {
		return err
	}
	r.mu.Lock()
	sessionID := r.sessionID
	known := false
	for _, offered := range r.options {
		if offered.ID == option {
			known = true
			break
		}
	}
	r.mu.Unlock()
	if !known {
		return errors.New(i18n.T("The agent does not offer this option. Pick one from the list."))
	}
	return r.setConfigOption(ctx, sessionID, option, value)
}

// setConfigOption sets one option and takes the agent's answer, its whole
// list again, as the new state. An agent that answers with nothing is taken
// at its word that the value stuck.
func (r *runner) setConfigOption(ctx context.Context, sessionID, option, value string) error {
	result, err := r.conn.request(ctx, "session/set_config_option", map[string]any{"sessionId": sessionID, "configId": option, "value": value})
	if err != nil {
		return r.withAgentError(err)
	}
	var response struct {
		ConfigOptions []configOption `json:"configOptions"`
	}
	if json.Unmarshal(result, &response) == nil && len(response.ConfigOptions) != 0 {
		r.applyConfigOptions(response.ConfigOptions)
		return nil
	}
	r.mu.Lock()
	if option == r.modeOption {
		modes := append([]protocol.Choice(nil), r.modes...)
		r.mode = value
		r.mu.Unlock()
		r.emit(protocol.EvMode, protocol.ChoiceData{Current: value, Options: modes})
		r.updateInfo(func(info *protocol.SessionInfo) { info.Mode = value })
		return nil
	}
	options := make([]protocol.SessionOption, len(r.options))
	copy(options, r.options)
	for index := range options {
		if options[index].ID == option {
			options[index].Current = value
		}
	}
	r.options = options
	r.mu.Unlock()
	r.emit(protocol.EvOptions, protocol.OptionsData{Options: options})
	return nil
}

func (r *runner) Start(ctx context.Context, sink core.Sink) error {
	return r.ensureStarted(ctx, sink)
}

// startedLocked names the model and each option value the agent came up with.
func (r *runner) startedLocked() []protocol.HarnessFact {
	named := func(choices []protocol.Choice, id string) string {
		for _, choice := range choices {
			if choice.ID == id && choice.Name != "" {
				return choice.Name
			}
		}
		return id
	}
	facts := make([]protocol.HarnessFact, 0, len(r.options)+1)
	if r.model != "" {
		facts = append(facts, protocol.HarnessFact{Label: i18n.T("Model"), Value: named(r.models, r.model)})
	}
	for _, option := range r.options {
		if option.Current != "" {
			facts = append(facts, protocol.HarnessFact{Label: option.Name, Value: named(option.Choices, option.Current)})
		}
	}
	return facts
}

func (r *runner) Alive() bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.alive
}

func (r *runner) State() json.RawMessage {
	r.mu.Lock()
	state := resumeState{ACPSession: r.sessionID, Load: r.loadSession, Generation: r.generation}
	if state.ACPSession == "" && len(r.opts.Resume) != 0 {
		_ = json.Unmarshal(r.opts.Resume, &state)
	}
	r.mu.Unlock()
	encoded, _ := json.Marshal(state)
	return encoded
}

func (r *runner) Stop() error {
	r.startMu.Lock()
	defer r.startMu.Unlock()
	r.mu.Lock()
	if r.cmd == nil || r.waitDone == nil {
		r.mu.Unlock()
		r.stopTerminals()
		return nil
	}
	r.stopping = true
	cmd, stdin, done := r.cmd, r.stdin, r.waitDone
	r.mu.Unlock()
	r.Cancel()
	if stdin != nil {
		_ = stdin.Close()
	}
	_ = signalGroup(cmd, syscall.SIGTERM)
	select {
	case <-done:
	case <-time.After(stopGrace):
		_ = signalGroup(cmd, syscall.SIGKILL)
		<-done
	}
	r.stopTerminals()
	return nil
}

func (r *runner) killAgentAfterCancel() {
	r.startMu.Lock()
	defer r.startMu.Unlock()
	r.mu.Lock()
	if r.cmd == nil || r.waitDone == nil {
		r.mu.Unlock()
		return
	}
	r.stopping = true
	cmd, stdin, done := r.cmd, r.stdin, r.waitDone
	r.mu.Unlock()
	if stdin != nil {
		_ = stdin.Close()
	}
	_ = signalGroup(cmd, syscall.SIGKILL)
	<-done
}

func (r *runner) ensureStarted(ctx context.Context, sink core.Sink) error {
	r.startMu.Lock()
	defer r.startMu.Unlock()
	r.mu.Lock()
	r.sink = sink
	if r.alive && r.conn != nil && !r.conn.Closed() {
		r.mu.Unlock()
		return nil
	}
	staleCmd, staleStdin, staleDone := r.cmd, r.stdin, r.waitDone
	if staleCmd != nil && staleDone != nil {
		r.stopping = true
	}
	r.mu.Unlock()
	if staleCmd != nil && staleDone != nil {
		_ = r.stopStartedProcess(staleCmd, staleStdin, staleDone)
	}
	r.mu.Lock()
	r.stopping = false
	r.mu.Unlock()
	if len(r.argv) == 0 {
		return errors.New(i18n.T("The agent command is empty. Choose an installed harness."))
	}
	cmd := exec.Command(r.argv[0], r.argv[1:]...)
	cmd.Dir = r.opts.Cwd
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.WaitDelay = stopGrace
	if len(r.environment) != 0 {
		cmd.Env = os.Environ()
		for name, value := range r.environment {
			cmd.Env = setEnvironment(cmd.Env, name, value)
		}
	}
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return err
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}
	stderr := newTailBuffer(stderrTailLimit)
	cmd.Stderr = stderr
	if err := cmd.Start(); err != nil {
		return err
	}
	conn := newConnection(stdin)
	conn.onRequest = r.handleRequest
	conn.onNotify = r.handleNotification
	conn.onClose = func(err error) { r.connectionClosed(cmd, conn, err) }
	done := make(chan struct{})
	r.mu.Lock()
	r.cmd = cmd
	r.stdin = stdin
	r.conn = conn
	r.alive = true
	r.waitDone = done
	r.stderr = stderr
	r.generation++
	r.chunkKind = ""
	r.chunkID = ""
	r.chunkSeq = 0
	r.synthetic = 0
	r.tools = make(map[string]trackedTool)
	r.toolOrder = 0
	r.mu.Unlock()
	r.updateInfo(func(info *protocol.SessionInfo) { info.Alive = true })
	conn.start(stdout)
	go func() {
		err := cmd.Wait()
		r.processExited(cmd, err)
		close(done)
	}()
	if err := r.initialize(ctx); err != nil {
		_ = r.stopStartedProcess(cmd, stdin, done)
		return err
	}
	return nil
}

func (r *runner) initialize(ctx context.Context) error {
	result, err := r.conn.request(ctx, "initialize", map[string]any{
		"protocolVersion": acpProtocolVersion,
		"clientCapabilities": map[string]any{
			"fs":       map[string]bool{"readTextFile": true, "writeTextFile": true},
			"terminal": true,
		},
		"clientInfo": map[string]string{"name": "rashin", "version": clientVersion},
	})
	if err != nil {
		return r.withAgentError(err)
	}
	var initialized initResult
	if err := json.Unmarshal(result, &initialized); err != nil {
		return fmt.Errorf("decode agent initialization: %w", err)
	}
	if initialized.ProtocolVersion != acpProtocolVersion {
		return errors.New(i18n.Tf("The agent selected unsupported ACP version %d. Update the harness and try again.", initialized.ProtocolVersion))
	}
	r.mu.Lock()
	previousSession, previousLoad := r.sessionID, r.loadSession
	r.loadSession = initialized.AgentCapabilities.LoadSession
	r.promptImage = initialized.AgentCapabilities.PromptCapabilities.Image
	r.mu.Unlock()
	var resume resumeState
	_ = json.Unmarshal(r.opts.Resume, &resume)
	if previousSession != "" {
		resume = resumeState{ACPSession: previousSession, Load: previousLoad, Generation: r.generation}
	}
	method := "session/new"
	params := map[string]any{"cwd": r.opts.Cwd, "mcpServers": []any{}}
	if resume.ACPSession != "" && resume.Load && initialized.AgentCapabilities.LoadSession {
		method = "session/load"
		params["sessionId"] = resume.ACPSession
		r.mu.Lock()
		r.loading = true
		r.mu.Unlock()
	}
	opened, err := r.conn.request(ctx, method, params)
	r.mu.Lock()
	r.loading = false
	r.mu.Unlock()
	if err != nil {
		return r.withAgentError(err)
	}
	if resume.ACPSession != "" && method == "session/new" {
		r.emit(protocol.EvtNotice, protocol.Notice{
			Level:   protocol.NoticeWarn,
			Text:    i18n.T("The harness cannot resume this agent session. A new agent session was started."),
			Session: r.opts.Session.ID,
		})
	}
	var session sessionResult
	if err := json.Unmarshal(opened, &session); err != nil {
		return fmt.Errorf("decode agent session: %w", err)
	}
	if session.SessionID == "" && method == "session/load" {
		session.SessionID = resume.ACPSession
	}
	if session.SessionID == "" {
		return errors.New(i18n.T("The agent did not return a session id. Restart the harness and try again."))
	}
	r.mu.Lock()
	r.sessionID = session.SessionID
	r.mu.Unlock()
	r.applySessionState(session)
	r.mu.Lock()
	capabilities := protocol.HarnessCapabilities{
		LoadSession: initialized.AgentCapabilities.LoadSession,
		Images:      initialized.AgentCapabilities.PromptCapabilities.Image,
		Audio:       initialized.AgentCapabilities.PromptCapabilities.Audio,
		Context:     initialized.AgentCapabilities.PromptCapabilities.EmbeddedContext,
		Modes:       append([]protocol.Choice(nil), r.modes...),
		Models:      session.Models != nil || r.modelOption != "",
		Started:     r.startedLocked(),
	}
	report := r.reportCapabilities
	r.mu.Unlock()
	if report != nil {
		report(capabilities)
	}
	return nil
}

func (r *runner) processExited(cmd *exec.Cmd, err error) {
	_ = signalGroup(cmd, syscall.SIGTERM)
	r.mu.Lock()
	if r.cmd != cmd {
		r.mu.Unlock()
		return
	}
	wasStopping := r.stopping
	r.alive = false
	sink := r.sink
	stderr := ""
	if r.stderr != nil {
		stderr = r.stderr.String()
	}
	r.mu.Unlock()
	r.resolveAllPermissions("cancelled", "")
	if sink != nil {
		sink.Update(func(info *protocol.SessionInfo) { info.Alive = false })
	}
	if wasStopping {
		return
	}
	message := i18n.T("The agent process exited. Check the harness logs and start the turn again.")
	if stderr != "" {
		message += " " + stderr
	} else if err != nil {
		message += " " + err.Error()
	}
	if sink != nil {
		sink.Emit(protocol.EvError, protocol.ErrorData{Message: message, Code: "agent_exit"})
		sink.Update(func(info *protocol.SessionInfo) {
			info.Alive = false
			info.Error = message
		})
	}
}

func (r *runner) connectionClosed(cmd *exec.Cmd, conn *connection, _ error) {
	r.mu.Lock()
	if r.cmd != cmd || r.conn != conn {
		r.mu.Unlock()
		return
	}
	r.alive = false
	sink := r.sink
	r.mu.Unlock()
	_ = signalGroup(cmd, syscall.SIGTERM)
	if sink != nil {
		sink.Update(func(info *protocol.SessionInfo) { info.Alive = false })
	}
}

func (r *runner) stopStartedProcess(cmd *exec.Cmd, stdin ioCloser, done <-chan struct{}) error {
	r.mu.Lock()
	r.stopping = true
	r.mu.Unlock()
	_ = stdin.Close()
	_ = signalGroup(cmd, syscall.SIGTERM)
	select {
	case <-done:
		return nil
	case <-time.After(stopGrace):
		_ = signalGroup(cmd, syscall.SIGKILL)
		<-done
		return nil
	}
}

type ioCloser interface {
	Close() error
}

func signalGroup(cmd *exec.Cmd, signal syscall.Signal) error {
	if cmd == nil || cmd.Process == nil {
		return nil
	}
	return syscall.Kill(-cmd.Process.Pid, signal)
}

func (r *runner) withAgentError(err error) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.stderr == nil || r.stderr.String() == "" {
		return err
	}
	return fmt.Errorf("%w: %s", err, r.stderr.String())
}

func (r *runner) emit(kind string, data any) {
	r.mu.Lock()
	sink := r.sink
	r.mu.Unlock()
	if sink != nil {
		sink.Emit(kind, data)
	}
}

func (r *runner) updateInfo(change func(*protocol.SessionInfo)) {
	r.mu.Lock()
	sink := r.sink
	r.mu.Unlock()
	if sink != nil {
		sink.Update(change)
	}
}

func selectedPermission(option string) map[string]any {
	return map[string]any{"outcome": map[string]any{"outcome": "selected", "optionId": option}}
}

func cancelledPermission() map[string]any {
	return map[string]any{"outcome": map[string]any{"outcome": "cancelled"}}
}

func (r *runner) resolveAllPermissions(outcome, by string) {
	r.mu.Lock()
	open := make([]permissionRequest, 0, len(r.permissions))
	for key, permission := range r.permissions {
		open = append(open, permission)
		delete(r.permissions, key)
	}
	conn := r.conn
	sink := r.sink
	r.mu.Unlock()
	for _, permission := range open {
		if permission.cancel != nil {
			_ = permission.cancel(conn)
		} else if conn != nil {
			_ = conn.respond(permission.id, cancelledPermission())
		}
		if sink != nil {
			sink.Emit(protocol.EvPermissionResolved, protocol.PermissionResolvedData{
				Request: permission.publicID, Outcome: outcome, By: by,
			})
		}
	}
}

// attachedBlock is one attachment as a prompt block. A picture goes as image
// data to an agent that takes pictures. Anything else, and a picture for an
// agent that takes none or one too large to send whole, goes as a link to
// the file, which every agent takes and opens with its own tools.
func attachedBlock(attached protocol.Attachment, images bool) (map[string]any, error) {
	unreadable := func(err error) error {
		return fmt.Errorf("%s %w", i18n.Tf("Could not read the attachment %s. Check that the file still exists.", attached.Path), err)
	}
	if images {
		image, ok, err := attachment.ReadImage(attached.Path)
		switch {
		case errors.Is(err, attachment.ErrTooLarge):
		case err != nil:
			return nil, unreadable(err)
		case ok:
			return map[string]any{"type": "image", "data": base64.StdEncoding.EncodeToString(image.Data), "mimeType": image.Mime}, nil
		}
	}
	absolute, err := filepath.Abs(attached.Path)
	if err == nil {
		_, err = os.Stat(absolute)
	}
	if err != nil {
		return nil, unreadable(err)
	}
	link := map[string]any{"type": "resource_link", "uri": (&url.URL{Scheme: "file", Path: absolute}).String(), "name": filepath.Base(absolute)}
	if attached.Mime != "" {
		link["mimeType"] = attached.Mime
	}
	return link, nil
}

func cloneUsage(usage *protocol.TokenUsage) *protocol.TokenUsage {
	if usage == nil {
		return nil
	}
	copy := *usage
	return &copy
}

func parseUsage(raw json.RawMessage) *protocol.TokenUsage {
	if len(raw) == 0 || string(raw) == "null" {
		return nil
	}
	var usage struct {
		Input        int `json:"input"`
		Output       int `json:"output"`
		InputTokens  int `json:"inputTokens"`
		OutputTokens int `json:"outputTokens"`
		TotalTokens  int `json:"totalTokens"`
	}
	if json.Unmarshal(raw, &usage) != nil {
		return nil
	}
	if usage.Input == 0 {
		usage.Input = usage.InputTokens
	}
	if usage.Output == 0 {
		usage.Output = usage.OutputTokens
	}
	if usage.Input == 0 && usage.Output == 0 && usage.TotalTokens > 0 {
		usage.Input = usage.TotalTokens
	}
	if usage.Input == 0 && usage.Output == 0 {
		return nil
	}
	return &protocol.TokenUsage{Input: usage.Input, Output: usage.Output}
}
