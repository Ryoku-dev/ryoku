package daemon

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	i18n "ryoku-i18n"
	"ryoku-rashin/internal/config"
	"ryoku-rashin/internal/harness"
	"ryoku-rashin/internal/protocol"
	"ryoku-rashin/internal/teams"
)

const (
	outboundQueueSize = 64
	writeTimeout      = 5 * time.Second
)

type connection struct {
	daemon     *Daemon
	nc         net.Conn
	dec        *protocol.Decoder
	mu         sync.RWMutex
	kind       string
	subscribed bool
	watches    map[string]int
	closed     bool
	once       sync.Once
	outbound   chan []byte
	done       chan struct{}
	// app is whether the Rashin app opened this connection, read once when
	// the daemon accepted it.
	app bool
	// session is the team session whose own socket this connection came in
	// on; such a connection calls only that session's agent tools. Empty on
	// the main socket.
	session string
}

func newConnection(d *Daemon, nc net.Conn) *connection {
	c := &connection{daemon: d, nc: nc, dec: protocol.NewDecoder(nc), watches: make(map[string]int), outbound: make(chan []byte, outboundQueueSize), done: make(chan struct{})}
	go c.writeLoop()
	return c
}

func (c *connection) serve() {
	defer func() { c.close(); c.daemon.disconnected(c) }()
	first, err := c.dec.Next()
	if err != nil {
		return
	}
	if !first.IsRequest() || first.ID == nil || first.Method != protocol.MHello {
		id := int64(0)
		if first.ID != nil {
			id = *first.ID
		}
		c.respond(id, nil, protocol.Errorf(protocol.CodeBadRequest, "%s", i18n.T("The first request must be hello.")))
		return
	}
	var hello protocol.HelloParams
	if err := protocol.Unmarshal(first.Params, &hello); err != nil {
		c.respond(*first.ID, nil, protocol.Errorf(protocol.CodeBadRequest, "%s", i18n.T("Hello parameters are invalid.")))
		return
	}
	if hello.Version != protocol.Version {
		c.respond(*first.ID, nil, protocol.Errorf(protocol.CodeUnsupported, "%s", i18n.Tf("Protocol version %d is not supported.", hello.Version)))
		return
	}
	if hello.Client != protocol.ClientApp && hello.Client != protocol.ClientBar && hello.Client != protocol.ClientCLI && hello.Client != protocol.ClientHub {
		c.respond(*first.ID, nil, protocol.Errorf(protocol.CodeBadRequest, "%s", i18n.Tf("Client kind %q is not supported.", hello.Client)))
		return
	}
	c.mu.Lock()
	c.kind = hello.Client
	c.mu.Unlock()
	c.respond(*first.ID, protocol.HelloResult{Daemon: "rashind", Version: c.daemon.deps.Version, Protocol: protocol.Version, Enabled: true}, nil)
	for {
		frame, err := c.dec.Next()
		if err != nil {
			if !errors.Is(err, io.EOF) {
				c.daemon.warn("client decode: %v", err)
			}
			return
		}
		if !frame.IsRequest() || frame.ID == nil {
			return
		}
		if _, ok := protocol.Lookup(frame.Method); !ok {
			c.respond(*frame.ID, nil, protocol.Errorf(protocol.CodeUnsupported, "%s", i18n.Tf("Method %s is not supported.", frame.Method)))
			continue
		}
		if frame.Method == protocol.MHello {
			c.respond(*frame.ID, nil, protocol.Errorf(protocol.CodeConflict, "%s", i18n.T("Hello has already completed.")))
			continue
		}
		go c.handle(frame)
	}
}

func (c *connection) handle(frame protocol.Frame) {
	ctx := context.Background()
	var refused error
	if c.session != "" {
		refused = agentGate(c.session, frame.Method, frame.Params)
		ctx = teams.WithAgentSession(ctx, c.session)
	} else {
		refused = c.daemon.mainGate(c, frame.Method, frame.Params)
	}
	if refused != nil {
		c.respond(*frame.ID, nil, refused)
		return
	}
	if frame.Method == protocol.MSessionAttach {
		p, err := unmarshal[protocol.SessionAttachParams](frame.Params)
		if err != nil {
			c.respond(*frame.ID, nil, err)
			return
		}
		c.daemon.manager.attachQueued(p.ID, p.Since, p.Limit, c, func(result protocol.SessionAttachResult, err error) bool {
			return c.respond(*frame.ID, result, err)
		})
		return
	}
	result, err := c.daemon.dispatch(ctx, c, frame.Method, frame.Params)
	c.respond(*frame.ID, result, err)
}

func (c *connection) respond(id int64, result any, err error) bool {
	response := protocol.Response{ID: id, Error: protocol.AsError(err)}
	if err == nil {
		raw, marshalErr := protocol.Marshal(result)
		if marshalErr != nil {
			response.Error = protocol.AsError(protocol.Errorf(protocol.CodeInternal, "%s", i18n.T("The response could not be encoded.")))
		} else {
			response.Result = raw
		}
	}
	frame, marshalErr := json.Marshal(response)
	if marshalErr != nil || len(frame) > protocol.MaxLine {
		response = protocol.Response{ID: id, Error: protocol.AsError(protocol.Errorf(protocol.CodeInternal, "%s", i18n.T("The response is too large to send.")))}
		frame, marshalErr = json.Marshal(response)
	}
	if marshalErr != nil {
		c.close()
		return false
	}
	return c.enqueue(frame)
}

func (c *connection) sendEvent(name string, data any) bool {
	raw, err := protocol.Marshal(data)
	if err != nil {
		return false
	}
	frame, err := json.Marshal(protocol.Event{Event: name, Data: raw})
	if err != nil || len(frame) > protocol.MaxLine {
		return false
	}
	return c.enqueue(frame)
}

func (c *connection) enqueue(frame []byte) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closed || c.outbound == nil {
		return false
	}
	select {
	case c.outbound <- frame:
		return true
	default:
		go c.close()
		return false
	}
}

func (c *connection) writeLoop() {
	for {
		select {
		case frame := <-c.outbound:
			if err := c.nc.SetWriteDeadline(time.Now().Add(writeTimeout)); err != nil {
				c.close()
				return
			}
			frame = append(frame, '\n')
			for len(frame) > 0 {
				n, err := c.nc.Write(frame)
				if err != nil {
					c.close()
					return
				}
				frame = frame[n:]
			}
		case <-c.done:
			return
		}
	}
}

func (c *connection) close() {
	c.once.Do(func() {
		c.mu.Lock()
		c.closed = true
		if c.done != nil {
			close(c.done)
		}
		c.mu.Unlock()
		if c.nc != nil {
			_ = c.nc.Close()
		}
	})
}

func (d *Daemon) deliverGlobal(name string, data any) {
	d.mu.RLock()
	clients := make([]*connection, 0, len(d.clients))
	for c := range d.clients {
		clients = append(clients, c)
	}
	d.mu.RUnlock()
	for _, c := range clients {
		c.mu.RLock()
		subscribed, kind := c.subscribed, c.kind
		watching := name == protocol.EvtGitChanged && c.watches[gitChangedProject(data)] > 0
		c.mu.RUnlock()
		if name == protocol.EvtGitChanged {
			if !watching {
				continue
			}
		} else if !subscribed || name == protocol.EvtUIFocus && kind != protocol.ClientApp {
			continue
		}
		c.sendEvent(name, data)
	}
	d.stateChanged()
}

func gitChangedProject(data any) string {
	switch changed := data.(type) {
	case protocol.GitChanged:
		return changed.Project
	case *protocol.GitChanged:
		if changed != nil {
			return changed.Project
		}
		return ""
	default:
		raw, err := json.Marshal(data)
		if err != nil {
			return ""
		}
		var decoded protocol.GitChanged
		if json.Unmarshal(raw, &decoded) != nil {
			return ""
		}
		return decoded.Project
	}
}

func unmarshal[T any](raw json.RawMessage) (T, error) {
	var value T
	if err := protocol.Unmarshal(raw, &value); err != nil {
		return value, protocol.Errorf(protocol.CodeBadRequest, "%s", i18n.T("Request parameters are invalid."))
	}
	return value, nil
}

func (d *Daemon) dispatch(ctx context.Context, c *connection, method string, raw json.RawMessage) (any, error) {
	switch method {
	case protocol.MStatus:
		return d.status(ctx), nil
	case protocol.MSubscribe:
		snapshot, err := d.snapshot(ctx)
		if err != nil {
			return nil, err
		}
		c.mu.Lock()
		c.subscribed = true
		c.mu.Unlock()
		return snapshot, nil
	case protocol.MShutdown:
		go func() { time.Sleep(10 * time.Millisecond); d.Shutdown() }()
		return protocol.Empty{}, nil
	case protocol.MSessionList:
		p, err := unmarshal[protocol.SessionListParams](raw)
		if err != nil {
			return nil, err
		}
		return d.manager.list(p)
	case protocol.MSessionNew:
		p, err := unmarshal[protocol.SessionNewParams](raw)
		if err != nil {
			return nil, err
		}
		return d.manager.newSession(ctx, p)
	case protocol.MSessionAttach:
		p, err := unmarshal[protocol.SessionAttachParams](raw)
		if err != nil {
			return nil, err
		}
		s, getErr := d.manager.get(p.ID)
		if getErr != nil {
			return nil, getErr
		}
		s.mu.Lock()
		result, attachErr := attachPage(s, p.Since, p.Limit)
		if attachErr == nil && !result.More {
			s.attached[c] = struct{}{}
		}
		s.mu.Unlock()
		return result, attachErr
	case protocol.MSessionDetach:
		p, err := unmarshal[protocol.SessionRef](raw)
		if err != nil {
			return nil, err
		}
		d.manager.detach(p.ID, c)
		return protocol.Empty{}, nil
	case protocol.MSessionPrompt:
		p, err := unmarshal[protocol.PromptParams](raw)
		if err != nil {
			return nil, err
		}
		result, _, err := d.manager.prompt(ctx, p, nil)
		return result, err
	case protocol.MSessionCancel:
		p, err := unmarshal[protocol.SessionRef](raw)
		if err != nil {
			return nil, err
		}
		return protocol.Empty{}, d.manager.cancel(p.ID)
	case protocol.MSessionPermission:
		p, err := unmarshal[protocol.PermissionParams](raw)
		if err != nil {
			return nil, err
		}
		return protocol.Empty{}, d.manager.permission(p)
	case protocol.MSessionSetModel:
		p, err := unmarshal[protocol.SetModelParams](raw)
		if err != nil {
			return nil, err
		}
		return protocol.Empty{}, d.manager.setModel(ctx, p)
	case protocol.MSessionSetMode:
		p, err := unmarshal[protocol.SetModeParams](raw)
		if err != nil {
			return nil, err
		}
		return protocol.Empty{}, d.manager.setMode(ctx, p)
	case protocol.MSessionSetOption:
		p, err := unmarshal[protocol.SetOptionParams](raw)
		if err != nil {
			return nil, err
		}
		return protocol.Empty{}, d.manager.setOption(ctx, p)
	case protocol.MSessionStart:
		p, err := unmarshal[protocol.SessionRef](raw)
		if err != nil {
			return nil, err
		}
		return protocol.Empty{}, d.manager.start(ctx, p.ID)
	case protocol.MSessionRename:
		p, err := unmarshal[protocol.RenameParams](raw)
		if err != nil {
			return nil, err
		}
		return d.manager.rename(p)
	case protocol.MSessionDelete:
		p, err := unmarshal[protocol.SessionRef](raw)
		if err != nil {
			return nil, err
		}
		return protocol.Empty{}, d.manager.delete(p.ID)
	case protocol.MSessionStop:
		p, err := unmarshal[protocol.SessionRef](raw)
		if err != nil {
			return nil, err
		}
		return protocol.Empty{}, d.manager.stop(p.ID)
	case protocol.MSessionHandoff:
		p, err := unmarshal[protocol.HandoffParams](raw)
		if err != nil {
			return nil, err
		}
		return d.handoff(ctx, p)
	case protocol.MSessionQueue:
		p, err := unmarshal[protocol.QueueParams](raw)
		if err != nil {
			return nil, err
		}
		return d.manager.queue(ctx, p)
	case protocol.MSessionGoal:
		p, err := unmarshal[protocol.GoalParams](raw)
		if err != nil {
			return nil, err
		}
		return d.manager.goal(ctx, p)
	case protocol.MScreenList:
		return d.screens.list(), nil
	case protocol.MScreenAct:
		p, err := unmarshal[protocol.ScreenActParams](raw)
		if err != nil {
			return nil, err
		}
		if _, err := d.manager.get(p.ID); err != nil {
			return nil, err
		}
		return d.screens.act(ctx, p)
	case protocol.MScreenControl:
		p, err := unmarshal[protocol.ScreenControlParams](raw)
		if err != nil {
			return nil, err
		}
		return d.screens.control(ctx, p)
	case protocol.MSessionUnqueue:
		p, err := unmarshal[protocol.UnqueueParams](raw)
		if err != nil {
			return nil, err
		}
		return d.manager.unqueue(p)
	case protocol.MSessionSteer:
		p, err := unmarshal[protocol.PromptParams](raw)
		if err != nil {
			return nil, err
		}
		return d.manager.steer(ctx, p)
	case protocol.MSessionFork:
		p, err := unmarshal[protocol.ForkParams](raw)
		if err != nil {
			return nil, err
		}
		return d.manager.fork(ctx, p)
	case protocol.MSessionRead:
		p, err := unmarshal[protocol.SessionRef](raw)
		if err != nil {
			return nil, err
		}
		return protocol.Empty{}, d.manager.read(p.ID)
	case protocol.MSessionRevertTurn:
		p, err := unmarshal[protocol.TurnParams](raw)
		if err != nil {
			return nil, err
		}
		return d.manager.revertTurn(ctx, p)
	case protocol.MAsk:
		p, err := unmarshal[protocol.AskParams](raw)
		if err != nil {
			return nil, err
		}
		return d.ask(ctx, c, p)
	case protocol.MProvidersList:
		providers, err := d.deps.Gateway.Providers(ctx)
		return protocol.ProvidersResult{Providers: providers}, err
	case protocol.MProvidersAddKey:
		p, err := unmarshal[protocol.AddKeyParams](raw)
		if err != nil {
			return nil, err
		}
		out, err := d.deps.Gateway.AddKey(ctx, p)
		if err == nil {
			d.broadcastProviders(ctx)
		}
		return out, err
	case protocol.MProvidersRemoveKey:
		p, err := unmarshal[protocol.RemoveKeyParams](raw)
		if err != nil {
			return nil, err
		}
		out, err := d.deps.Gateway.RemoveKey(ctx, p)
		if err == nil {
			d.broadcastProviders(ctx)
		}
		return out, err
	case protocol.MProvidersTest:
		p, err := unmarshal[protocol.ProviderParams](raw)
		if err != nil {
			return nil, err
		}
		out, err := d.deps.Gateway.Test(ctx, p.Provider)
		if err == nil {
			d.broadcastProviders(ctx)
		}
		return out, err
	case protocol.MProvidersEnable:
		p, err := unmarshal[protocol.EnableProviderParams](raw)
		if err != nil {
			return nil, err
		}
		out, err := d.deps.Gateway.EnableProvider(ctx, p)
		if err == nil {
			d.broadcastProviders(ctx)
		}
		return out, err
	case protocol.MModelsList:
		p, err := unmarshal[protocol.ModelsParams](raw)
		if err != nil {
			return nil, err
		}
		return d.deps.Gateway.Models(ctx, p.Provider)
	case protocol.MGatewayAPI:
		return d.apiInfo(), nil
	case protocol.MGatewayAPIKey:
		p, err := unmarshal[protocol.APIKeyParams](raw)
		if err != nil {
			return nil, err
		}
		key, err := d.deps.Gateway.APIKey(ctx, p.Rotate)
		return protocol.APIKey{Key: key}, err
	case protocol.MGatewayUsage:
		p, err := unmarshal[protocol.UsageParams](raw)
		if err != nil {
			return nil, err
		}
		return d.deps.Gateway.Usage(ctx, p.Days)
	case protocol.MHarnessesList:
		p, err := unmarshal[protocol.HarnessesListParams](raw)
		if err != nil {
			return nil, err
		}
		list, err := d.deps.Harnesses.List(ctx, p.Refresh)
		out := protocol.HarnessesResult{Harnesses: list}
		if err == nil && p.Refresh {
			d.Broadcast(protocol.EvtHarnessesChanged, out)
		}
		return out, err
	case protocol.MHarnessesProbe:
		return d.probeHarness(ctx, c, raw)
	case protocol.MIndexList:
		list, err := d.deps.Indexer.List(ctx)
		return protocol.IndexesResult{Indexes: list}, err
	case protocol.MIndexAdd:
		p, err := unmarshal[protocol.IndexAddParams](raw)
		if err != nil {
			return nil, err
		}
		if err := requireAbsolutePath(p.Path); err != nil {
			return nil, err
		}
		return d.deps.Indexer.Add(ctx, p)
	case protocol.MIndexRemove:
		p, err := unmarshal[protocol.IndexRef](raw)
		if err != nil {
			return nil, err
		}
		return protocol.Empty{}, d.deps.Indexer.Remove(ctx, p)
	case protocol.MIndexRefresh:
		p, err := unmarshal[protocol.IndexRef](raw)
		if err != nil {
			return nil, err
		}
		return d.deps.Indexer.Refresh(ctx, p)
	case protocol.MIndexStatus:
		p, err := unmarshal[protocol.IndexRef](raw)
		if err != nil {
			return nil, err
		}
		return d.deps.Indexer.Status(ctx, p)
	case protocol.MIndexSearch:
		p, err := unmarshal[protocol.SearchParams](raw)
		if err != nil {
			return nil, err
		}
		return d.deps.Indexer.Search(ctx, p)
	case protocol.MIndexFind:
		p, err := unmarshal[protocol.NameParams](raw)
		if err != nil {
			return nil, err
		}
		return d.deps.Indexer.Find(ctx, p)
	case protocol.MIndexDef:
		p, err := unmarshal[protocol.NameParams](raw)
		if err != nil {
			return nil, err
		}
		return d.deps.Indexer.Def(ctx, p)
	case protocol.MIndexOutline:
		p, err := unmarshal[protocol.PathParams](raw)
		if err != nil {
			return nil, err
		}
		return d.deps.Indexer.Outline(ctx, p)
	case protocol.MIndexReferences:
		p, err := unmarshal[protocol.NameParams](raw)
		if err != nil {
			return nil, err
		}
		return d.deps.Indexer.References(ctx, p)
	case protocol.MIndexPeek:
		p, err := unmarshal[protocol.PeekParams](raw)
		if err != nil {
			return nil, err
		}
		return d.deps.Indexer.Peek(ctx, p)
	case protocol.MIndexOverview:
		p, err := unmarshal[protocol.IndexRef](raw)
		if err != nil {
			return nil, err
		}
		return d.deps.Indexer.Overview(ctx, p)
	case protocol.MIndexImpact:
		p, err := unmarshal[protocol.PathParams](raw)
		if err != nil {
			return nil, err
		}
		return d.deps.Indexer.Impact(ctx, p)
	case protocol.MIndexWip:
		p, err := unmarshal[protocol.WipParams](raw)
		if err != nil {
			return nil, err
		}
		return d.deps.Indexer.Wip(ctx, p)
	case protocol.MIndexChanged:
		p, err := unmarshal[protocol.WipParams](raw)
		if err != nil {
			return nil, err
		}
		return d.deps.Indexer.Changed(ctx, p)
	case protocol.MSettingsGet:
		return d.Settings(), nil
	case protocol.MSettingsSet:
		d.settingsMu.Lock()
		defer d.settingsMu.Unlock()
		p, err := unmarshal[protocol.SettingsSetParams](raw)
		if err != nil {
			return nil, err
		}
		if settingsForApp(p.Patch) {
			if err := requireApp(c); err != nil {
				return nil, err
			}
		}
		current := d.Settings()
		next, err := config.Merge(current, p.Patch)
		if err != nil {
			return nil, err
		}
		// an instance removed later leaves the default unknown; only a new
		// choice is checked, so unrelated changes still save
		if chosen := next.Teams.Defaults.Harness; chosen != "" && chosen != current.Teams.Defaults.Harness && !harness.Known(chosen, next) {
			return nil, protocol.Errorf(protocol.CodeBadRequest, "%s", i18n.Tf("No harness %s on this computer.", chosen))
		}
		next, err = config.SaveMerged(next)
		if err != nil {
			return nil, err
		}
		d.mu.Lock()
		d.settings = next
		d.mu.Unlock()
		d.Broadcast(protocol.EvtSettingsChanged, next)
		if d.deps.Teams != nil {
			d.deps.Teams.SettingsChanged(next)
		}
		d.applyAPI()
		d.stateChanged()
		return next, nil
	case protocol.MUIOpen:
		p, err := unmarshal[protocol.UIOpenParams](raw)
		if err != nil {
			return nil, err
		}
		return protocol.UIOpenResult{Delivered: d.focusApps(d.openTarget(p))}, nil
	case protocol.MProjectsAdd:
		if d.deps.Work == nil {
			return nil, protocol.Errorf(protocol.CodeUnavailable, "%s", i18n.T("Work is not available."))
		}
		p, err := unmarshal[protocol.ProjectAddParams](raw)
		if err != nil {
			return nil, err
		}
		if err := requireAbsolutePath(p.Path); err != nil {
			return nil, err
		}
		result, err := d.deps.Work.Handle(ctx, method, raw)
		if err != nil {
			return nil, err
		}
		project, ok := projectInfo(result)
		if !ok {
			return nil, protocol.Errorf(protocol.CodeInternal, "%s", i18n.T("Work returned an invalid project."))
		}
		if d.deps.Indexer != nil {
			_, indexErr := d.deps.Indexer.Add(ctx, protocol.IndexAddParams{Path: project.Root, Name: project.Name})
			if indexErr != nil && !isAlreadyRegistered(indexErr) {
				return nil, indexErr
			}
		}
		return result, nil
	case protocol.MProjectsWatch:
		p, err := unmarshal[protocol.WatchParams](raw)
		if err != nil {
			return nil, err
		}
		if p.Project == "" {
			return nil, protocol.Errorf(protocol.CodeBadRequest, "%s", i18n.T("Project is required."))
		}
		if d.deps.Work == nil {
			return nil, protocol.Errorf(protocol.CodeUnavailable, "%s", i18n.T("Work is not available."))
		}
		c.setWatch(p.Project, p.Watch)
		return protocol.Empty{}, nil
	default:
		if isWorkMethod(method) {
			if d.deps.Work == nil {
				return nil, protocol.Errorf(protocol.CodeUnavailable, "%s", i18n.T("Work is not available."))
			}
			if err := validateWorktreePaths(method, raw); err != nil {
				return nil, err
			}
			return d.deps.Work.Handle(ctx, method, raw)
		}
		if teams.IsMethod(method) {
			if teams.UserOnly(method) {
				if err := requireApp(c); err != nil {
					return nil, err
				}
			}
			if d.deps.Teams == nil {
				return nil, protocol.Errorf(protocol.CodeUnavailable, "%s", i18n.T("Teams are not available."))
			}
			return d.deps.Teams.Handle(ctx, method, raw)
		}
		return nil, protocol.Errorf(protocol.CodeUnsupported, "%s", i18n.Tf("Method %s is not supported.", method))
	}
}

func requireAbsolutePath(path string) error {
	if path == "" || !filepath.IsAbs(path) {
		return protocol.Errorf(protocol.CodeBadRequest, "%s", i18n.T("A filesystem path must be absolute."))
	}
	return nil
}

func validateWorktreePaths(method string, raw json.RawMessage) error {
	switch method {
	case protocol.MWorktreesPreviewRemove, protocol.MWorktreesPorts:
		p, err := unmarshal[protocol.WorktreeRef](raw)
		if err != nil {
			return err
		}
		return requireAbsolutePath(p.Path)
	case protocol.MWorktreesRemove:
		p, err := unmarshal[protocol.WorktreeRemoveParams](raw)
		if err != nil {
			return err
		}
		return requireAbsolutePath(p.Path)
	case protocol.MWorktreesLock:
		p, err := unmarshal[protocol.WorktreeLockParams](raw)
		if err != nil {
			return err
		}
		return requireAbsolutePath(p.Path)
	case protocol.MWorktreesMove:
		p, err := unmarshal[protocol.WorktreeMoveParams](raw)
		if err != nil {
			return err
		}
		if err := requireAbsolutePath(p.Path); err != nil {
			return err
		}
		return requireAbsolutePath(p.To)
	case protocol.MWorktreesSetup:
		p, err := unmarshal[protocol.WorktreeSetupParams](raw)
		if err != nil {
			return err
		}
		return requireAbsolutePath(p.Path)
	case protocol.MWorktreesTrust:
		p, err := unmarshal[protocol.TrustParams](raw)
		if err != nil {
			return err
		}
		return requireAbsolutePath(p.Path)
	default:
		return nil
	}
}

func isWorkMethod(method string) bool {
	return strings.HasPrefix(method, "projects.") ||
		strings.HasPrefix(method, "worktrees.") ||
		strings.HasPrefix(method, "git.") ||
		strings.HasPrefix(method, "review.") ||
		strings.HasPrefix(method, "tasks.") ||
		method == protocol.MActivityList
}

func projectInfo(result any) (protocol.ProjectInfo, bool) {
	switch project := result.(type) {
	case protocol.ProjectInfo:
		return project, true
	case *protocol.ProjectInfo:
		if project != nil {
			return *project, true
		}
	}
	return protocol.ProjectInfo{}, false
}

func isAlreadyRegistered(err error) bool {
	var methodErr *protocol.Error
	return errors.As(err, &methodErr) && methodErr.Code == protocol.CodeConflict
}

func (c *connection) setWatch(project string, watch bool) {
	c.mu.Lock()
	if c.closed {
		c.mu.Unlock()
		return
	}
	if c.watches == nil {
		c.watches = make(map[string]int)
	}
	count := c.watches[project]
	changed := false
	if watch {
		c.watches[project] = count + 1
		changed = count == 0
	} else if count > 1 {
		c.watches[project] = count - 1
	} else if count == 1 {
		delete(c.watches, project)
		changed = true
	}
	if changed {
		c.daemon.deps.Work.Watch(project, watch)
	}
	c.mu.Unlock()
}

func (c *connection) dropWatches() {
	if c.daemon.deps.Work == nil {
		return
	}
	c.mu.Lock()
	projects := make([]string, 0, len(c.watches))
	for project, count := range c.watches {
		if count > 0 {
			projects = append(projects, project)
		}
	}
	c.watches = make(map[string]int)
	c.mu.Unlock()
	for _, project := range projects {
		c.daemon.deps.Work.Watch(project, false)
	}
}

func (d *Daemon) status(ctx context.Context) protocol.Status {
	d.mu.RLock()
	clients := len(d.clients)
	settings := d.settings
	d.mu.RUnlock()
	status := protocol.Status{
		Version: d.deps.Version, Enabled: settings.Enabled, Running: true, PID: os.Getpid(),
		UptimeSec: int64(time.Since(d.started).Seconds()), Clients: clients, Sessions: d.manager.counts(),
		Gateway: d.deps.Gateway.Status(ctx), API: d.apiInfo(),
	}
	if list, err := d.deps.Harnesses.List(ctx, false); err == nil {
		for _, h := range list {
			if h.Installed {
				status.Harnesses++
			}
		}
	}
	if list, err := d.deps.Indexer.List(ctx); err == nil {
		status.Indexes = len(list)
	}
	return status
}

func (d *Daemon) snapshot(ctx context.Context) (protocol.Snapshot, error) {
	sessions, err := d.manager.recent(50)
	if err != nil {
		return protocol.Snapshot{}, err
	}
	providers, err := d.deps.Gateway.Providers(ctx)
	if err != nil {
		return protocol.Snapshot{}, err
	}
	harnesses, err := d.deps.Harnesses.List(ctx, false)
	if err != nil {
		return protocol.Snapshot{}, err
	}
	indexes, err := d.deps.Indexer.List(ctx)
	if err != nil {
		return protocol.Snapshot{}, err
	}
	snapshot := protocol.Snapshot{Status: d.status(ctx), Sessions: sessions, Settings: d.Settings(), Providers: providers, Harnesses: harnesses, Indexes: indexes, Screens: d.screens.list().Screens}
	if d.deps.Work != nil {
		snapshot.Projects = d.deps.Work.Projects(ctx)
		snapshot.Tasks = d.deps.Work.Tasks(ctx)
	}
	if d.deps.Teams != nil {
		d.deps.Teams.Snapshot(ctx, &snapshot)
	}
	return snapshot, nil
}

func (d *Daemon) broadcastProviders(ctx context.Context) {
	if p, err := d.deps.Gateway.Providers(ctx); err == nil {
		d.Broadcast(protocol.EvtProvidersChanged, protocol.ProvidersResult{Providers: p})
	}
}

// openTarget completes a request that names only a session with the tab that
// session lives in. The caller means "show it", and the daemon is the one that
// knows where: the app may not have heard of the session yet, and guessed Ask.
func (d *Daemon) openTarget(p protocol.UIOpenParams) protocol.UIOpenParams {
	if p.Tab != "" || p.Session == "" {
		return p
	}
	if info, ok := d.manager.Info(p.Session); ok {
		p.Tab = protocol.TabAsk
		if info.Kind == protocol.KindAgent {
			p.Tab = protocol.TabAgent
		}
	}
	return p
}

func (d *Daemon) focusApps(p protocol.UIOpenParams) bool {
	d.mu.RLock()
	clients := make([]*connection, 0, len(d.clients))
	for c := range d.clients {
		clients = append(clients, c)
	}
	d.mu.RUnlock()
	delivered := false
	for _, c := range clients {
		c.mu.RLock()
		app := c.kind == protocol.ClientApp && c.subscribed
		c.mu.RUnlock()
		if app && c.sendEvent(protocol.EvtUIFocus, p) {
			delivered = true
		}
	}
	return delivered
}
