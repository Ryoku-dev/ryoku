package daemon

import (
	"context"
	"encoding/json"
	"strings"
	"sync"
	"sync/atomic"

	i18n "ryoku-i18n"
	"ryoku-rashin/internal/oni/feature"
	"ryoku-rashin/internal/oni/presence"
	"ryoku-rashin/internal/oni/profile"
	"ryoku-rashin/internal/oni/spec"
	"ryoku-rashin/internal/paths"
	"ryoku-rashin/internal/protocol"
)

// Oni's calls read the characters from disk when asked and keep nothing
// loaded, so an idle daemon holds no Oni state and still exits. Presence
// runs only while a connection has the oni event open.

// loadOniCharacters is the shipped oni followed by the user's characters,
// each as it loads now.
func loadOniCharacters() []protocol.OniCharacter {
	loaded := profile.List(paths.CharactersDir())
	out := make([]protocol.OniCharacter, 0, len(loaded))
	for _, l := range loaded {
		c := protocol.OniCharacter{
			ID:      l.Profile.ID,
			Name:    l.Profile.Name,
			Shipped: l.Profile.Shipped,
			Valid:   l.Valid,
			Errors:  append([]string(nil), l.Report.Errors...),
		}
		for _, f := range l.Report.Fallbacks {
			c.Fallbacks = append(c.Fallbacks, protocol.OniFallback{Slot: f.Slot, Reason: f.Reason, Used: f.Used})
		}
		out = append(out, c)
	}
	return out
}

// activeOniCharacter is the selected character when it loads, else the
// shipped one.
func activeOniCharacter(selected string, characters []protocol.OniCharacter) string {
	for _, c := range characters {
		if c.ID == selected && c.Valid {
			return c.ID
		}
	}
	return protocol.OniDefaultCharacter
}

func (d *Daemon) oniStatus() protocol.OniStatus {
	settings := d.Settings()
	effective := protocol.EffectiveOni(settings)
	characters := loadOniCharacters()
	return protocol.OniStatus{
		Enabled:      settings.Enabled,
		Master:       settings.Oni.Master,
		Active:       effective.Master,
		Character:    activeOniCharacter(settings.Oni.Character, characters),
		Capabilities: feature.Capabilities(effective),
		Characters:   characters,
	}
}

func (d *Daemon) oniCharacters() protocol.OniCharactersResult {
	characters := loadOniCharacters()
	return protocol.OniCharactersResult{
		Active:     activeOniCharacter(d.Settings().Oni.Character, characters),
		Characters: characters,
	}
}

// oniSelect makes id the current character through the settings, refusing
// one that does not load so the current character stays.
func (d *Daemon) oniSelect(ctx context.Context, id string) (protocol.OniStatus, error) {
	if !protocol.ValidOniCharacterID(id) {
		return protocol.OniStatus{}, protocol.Errorf(protocol.CodeBadRequest, "%s", i18n.T("The Oni character id is invalid."))
	}
	var found *protocol.OniCharacter
	characters := loadOniCharacters()
	for i := range characters {
		if characters[i].ID == id {
			found = &characters[i]
			break
		}
	}
	if found == nil {
		return protocol.OniStatus{}, protocol.Errorf(protocol.CodeNotFound, "%s", i18n.Tf("There is no Oni character %s.", id))
	}
	if !found.Valid {
		return protocol.OniStatus{}, protocol.Errorf(protocol.CodeBadRequest, "%s", i18n.Tf("The Oni character %s does not load: %s.", id, strings.Join(found.Errors, "; ")))
	}
	patch, err := json.Marshal(map[string]any{"oni": map[string]any{"character": id}})
	if err != nil {
		return protocol.OniStatus{}, err
	}
	if _, err := d.applySettings(ctx, patch); err != nil {
		return protocol.OniStatus{}, err
	}
	return d.oniStatus(), nil
}

func oniSpec() (protocol.OniSpecResult, error) {
	s, err := spec.Load()
	if err != nil {
		return protocol.OniSpecResult{}, protocol.Errorf(protocol.CodeInternal, "%s", i18n.Tf("The Oni motion spec does not load: %v.", err))
	}
	raw, err := json.Marshal(s)
	if err != nil {
		return protocol.OniSpecResult{}, protocol.Errorf(protocol.CodeInternal, "%s", i18n.Tf("The Oni motion spec does not load: %v.", err))
	}
	return protocol.OniSpecResult{
		Spec:     raw,
		Features: feature.Features(),
		Frames:   feature.Kinds(),
		Calls:    feature.ReservedCalls(),
	}, nil
}

// oniPresence is Oni's presence and the connections that opened the oni
// event. Its frames go to those connections alone, never through the
// broadcast, so drawing Oni never keeps the daemon awake.
type oniPresence struct {
	// mu serialises starting and stopping the service
	mu      sync.Mutex
	service atomic.Pointer[presence.Service]
	// stopPower ends the power-profile watch the service was started with
	stopPower context.CancelFunc

	clientsMu sync.Mutex
	clients   map[*connection]*oniClient
	presenter *connection

	// perch is the window Oni sits on and ask the question about it; both
	// hold nothing while no perch is out
	perch oniPerch
	ask   oniAsk
	// voice is the voice loop; it holds nothing until voice is used
	voice oniVoiceLoop
	// guide is the guided walkthrough and pick the armed select-and-ask;
	// both hold nothing while none runs
	guide oniGuide
	pick  oniPick
	// routine watches quick commands for a routine to offer; it holds
	// nothing until one runs with routines on
	routine oniRoutine
	// reflex is the quick commands' executors, caps and ledger; it holds
	// nothing until one runs with reflex on
	reflex oniReflexState
	// remind is the reminders' one timer and their hold on the wake unit;
	// it holds nothing while reminders are off
	remind oniRemind
	// hands is the Hands browser and its task; it holds nothing until the
	// Hands browser is opened
	hands oniHandsState
	// guard is the focus guard; it holds nothing while disarmed
	guard oniGuard
	// rewrite is the one rewrite card or record; it holds nothing once its
	// card or undo time ends
	rewrite oniRewriteState
	// memory is the voice memory card and the note voice kept last; it
	// holds nothing until a memory command is said
	memory oniMemory
	// recall is the recall card waiting on its answer; it holds nothing
	// while none waits
	recall oniRecall
	// assign is assignment mode's solved views, card and day's ledger; it
	// holds nothing until assignment mode starts
	assign oniAssign
	// notify is the lines the MCP bridge showed in the last minute; it
	// holds at most oniNotifyPerMinute times
	notify oniNotify
}

// oniClient is one connection with the oni event open. Until its oni.open
// reply is on its way, the frames emitted for it are held, so none reaches
// it before the snapshot it starts from.
type oniClient struct {
	renderer string
	holding  bool
	held     [][]byte
}

func (d *Daemon) oniCall(c *connection, method string, raw json.RawMessage) (any, error) {
	params, err := feature.DecodeCall(method, raw)
	if err != nil {
		return nil, protocol.Errorf(protocol.CodeBadRequest, "%s", i18n.Tf("The Oni call is invalid: %v.", err))
	}
	switch p := params.(type) {
	case *protocol.OniPlaceParams:
		return protocol.Empty{}, d.oniPlace(*p)
	case *protocol.OniInteractParams:
		if svc := d.oni.service.Load(); svc != nil {
			svc.Interact(p.Kind)
		}
		return protocol.Empty{}, nil
	case *protocol.OniCharacterAssetsParams:
		return oniAssets(p.ID)
	case *protocol.OniStrandArrivedParams:
		return protocol.Empty{}, d.oniArrived(c, *p)
	case *protocol.OniPerchParams:
		return d.oniPerchCall(c, *p)
	case *protocol.OniDetachParams:
		return protocol.Empty{}, d.oniDetachCall(c, *p)
	case *protocol.OniOverlayParams:
		return protocol.Empty{}, d.oniOverlayCall(c, *p)
	case *protocol.OniAskParams:
		return d.oniAskCall(c, p.Text)
	case *protocol.OniConsentAnswerParams:
		return d.oniConsentAnswer(c, *p)
	case *protocol.OniTalkParams:
		return d.voiceTalk(c, *p)
	case *protocol.OniSpeakParams:
		return d.voiceSpeak(*p)
	case *protocol.OniDictateParams:
		return d.voiceDictate(*p)
	case *protocol.OniVoiceTestParams:
		return d.voiceTest(*p)
	case *protocol.OniGuideParams:
		return d.oniGuideCall(c, *p)
	case *protocol.OniKeepParams:
		if method == protocol.MOniKeep {
			return d.oniKeepCall(c, p.ID)
		}
		return protocol.Empty{}, d.oniSkipCall(c, p.ID)
	case *protocol.OniPickParams:
		return d.oniPickCall(c, *p)
	case *protocol.OniRoutinesParams:
		return d.oniRoutinesCall(*p)
	case *protocol.OniRemindersParams:
		return d.remindersCall(*p)
	case *protocol.OniHandsParams:
		return d.oniHandsCall(c, *p)
	case *protocol.OniFocusParams:
		return d.oniFocusCall(*p)
	case *protocol.OniRewriteParams:
		return d.oniRewriteCall(c, *p)
	case *protocol.OniRecallParams:
		return d.oniRecallCall(c, *p)
	case *protocol.OniRecallImportParams:
		return d.oniRecallImportCall(*p)
	case *protocol.OniAssignmentParams:
		return d.oniAssignmentCall(c, *p)
	}
	if method == protocol.MOniAssignmentLog {
		return d.oniAssignmentLogCall()
	}
	if method == protocol.MOniHandsLog {
		return d.handsLogCall()
	}
	if method == protocol.MOniReflexLog {
		return d.reflexLogCall()
	}
	if method == protocol.MOniInspect {
		return d.oniAskCall(c, "")
	}
	d.oniClose(c)
	return protocol.Empty{}, nil
}

// oniOpenAndReply answers oni.open on c itself. The snapshot is taken and c
// registered under the service's lock, which every frame is emitted under,
// and the frames emitted after it wait until the reply is queued; so c sees
// the reply first and then only frames newer than its snapshot.
func (d *Daemon) oniOpenAndReply(c *connection, id int64, raw json.RawMessage) {
	result, err := d.oniOpen(c, raw)
	if !c.respond(id, result, err) || err != nil {
		if err == nil {
			d.oniClose(c)
		}
		return
	}
	// a waiting approval card goes to a presenter that just opened
	if result.Presenter {
		d.approvalsResend()
	}
	d.oni.clientsMu.Lock()
	defer d.oni.clientsMu.Unlock()
	client := d.oni.clients[c]
	if client == nil || !client.holding {
		return
	}
	for _, frame := range client.held {
		c.sendEvent(protocol.EvtOni, json.RawMessage(frame))
	}
	client.holding, client.held = false, nil
}

// oniOpen registers c, holding its frames, and returns the frames to start
// from. Only a surface may draw Oni, one at a time.
func (d *Daemon) oniOpen(c *connection, raw json.RawMessage) (protocol.OniOpenResult, error) {
	params, err := feature.DecodeCall(protocol.MOniOpen, raw)
	if err != nil {
		return protocol.OniOpenResult{}, protocol.Errorf(protocol.CodeBadRequest, "%s", i18n.Tf("The Oni call is invalid: %v.", err))
	}
	renderer := params.(*protocol.OniOpenParams).Renderer
	c.mu.RLock()
	surface := c.kind == protocol.ClientSurface
	appApprovals := c.app && c.kind == protocol.ClientApp && c.session == "" && renderer == protocol.OniRendererObserver
	c.mu.RUnlock()
	if renderer == protocol.OniRendererNative && !surface {
		return protocol.OniOpenResult{}, protocol.Errorf(protocol.CodeForbidden, "%s", i18n.T("Only a surface process can draw Oni."))
	}
	d.oni.mu.Lock()
	defer d.oni.mu.Unlock()
	svc := d.oniStart()
	result := protocol.OniOpenResult{Frames: []json.RawMessage{}}
	was := false
	svc.Open(appApprovals, func(snapshot []feature.Payload) {
		for _, p := range snapshot {
			frame, encodeErr := feature.EncodeFrame(p)
			if encodeErr != nil {
				err = protocol.Errorf(protocol.CodeInternal, "%s", i18n.Tf("An Oni frame could not be encoded: %v.", encodeErr))
				return
			}
			result.Frames = append(result.Frames, frame)
		}
		d.oni.clientsMu.Lock()
		defer d.oni.clientsMu.Unlock()
		if d.oni.clients == nil {
			d.oni.clients = map[*connection]*oniClient{}
		}
		if d.oni.presenter == c {
			d.oni.presenter, was = nil, true
		}
		switch {
		case renderer == protocol.OniRendererObserver:
			result.Reason = protocol.OniOpenObserver
		case renderer == protocol.OniRendererRyoku:
			result.Reason = protocol.OniOpenUnavailable
		case !protocol.EffectiveOni(d.Settings()).Master:
			result.Reason = protocol.OniOpenOff
		case d.oni.presenter != nil:
			result.Reason = protocol.OniOpenBusy
		default:
			d.oni.presenter = c
			result.Presenter = true
		}
		d.oni.clients[c] = &oniClient{renderer: renderer, holding: true}
	})
	if err != nil {
		return protocol.OniOpenResult{}, err
	}
	// a presenter that opens again starts over, its strand gone
	if was || result.Presenter {
		svc.Presenter(result.Presenter)
	}
	return result, nil
}

// oniStart starts presence when no connection had it open. The caller holds
// d.oni.mu.
func (d *Daemon) oniStart() *presence.Service {
	if svc := d.oni.service.Load(); svc != nil {
		return svc
	}
	settings := d.Settings()
	svc := presence.New(presence.SystemClock(), d.oniEmit)
	svc.OnDetach(d.oniDetached)
	svc.OnRest(d.oniRest)
	// a guide's step waits on its strand landing, never on a stale one
	svc.OnArrive(d.guideArrived)
	svc.OnCancel(d.guideCancelled)
	svc.Character(oniCharacterFrame(settings.Oni.Character))
	svc.Settings(protocol.EffectiveOni(settings))
	d.voicePresent(svc)
	power, stop := context.WithCancel(context.Background())
	d.oni.stopPower = stop
	go presence.WatchPowerSaver(power, svc.Power)
	for _, info := range d.runningSessions() {
		svc.Activity(oniActivity(info))
	}
	return svc
}

// oniStop stops presence and what it watches, and forgets the desktop, the
// perch and any question waiting. The caller holds d.oni.mu.
func (d *Daemon) oniStop() {
	if d.oni.stopPower != nil {
		d.oni.stopPower()
		d.oni.stopPower = nil
	}
	if svc := d.oni.service.Swap(nil); svc != nil {
		svc.Close()
	}
	d.oniPerchReset()
	d.oniAskReset()
	d.guideUnwind("closed")
	d.pickReset()
	// Hands works only where Oni is drawn: a tab's task stops, a task
	// outside a tab pauses
	d.handsPresenterLost()
}

// oniClose takes c off the frames; the last one to leave stops presence. A
// presenter that leaves cancels the strand target it was drawing.
func (d *Daemon) oniClose(c *connection) {
	d.oni.mu.Lock()
	defer d.oni.mu.Unlock()
	d.oni.clientsMu.Lock()
	_, had := d.oni.clients[c]
	delete(d.oni.clients, c)
	presenter := d.oni.presenter == c
	if presenter {
		d.oni.presenter = nil
	}
	empty := len(d.oni.clients) == 0
	d.oni.clientsMu.Unlock()
	if svc := d.oni.service.Load(); presenter && svc != nil {
		svc.Presenter(false)
	}
	if presenter {
		d.handsPresenterLost()
	}
	if had && empty {
		d.oniStop()
	}
}

// oniStrand sends the strand to t through the running presence and returns
// the seq its presenter reports the landing under. With nothing drawing Oni
// it fails at once.
func (d *Daemon) oniStrand(t feature.StrandTargetFrame) (uint64, error) {
	svc := d.oni.service.Load()
	if svc == nil {
		return 0, presence.ErrNoPresenter
	}
	return svc.Strand(t)
}

// oniArrived takes the report of where the strand landed. Only the
// connection drawing Oni may send it; a stale or repeated seq is answered
// and advances nothing.
func (d *Daemon) oniArrived(c *connection, p protocol.OniStrandArrivedParams) error {
	d.oni.clientsMu.Lock()
	presenter := d.oni.presenter == c
	d.oni.clientsMu.Unlock()
	if !presenter {
		return protocol.Errorf(protocol.CodeForbidden, "%s", i18n.T("Only the surface drawing Oni reports where its strand landed."))
	}
	if svc := d.oni.service.Load(); svc != nil {
		svc.Arrived(p.Seq, p.Skipped)
	}
	return nil
}

// oniEmit sends one frame to the connections that opened the oni event, or
// holds it for one whose oni.open reply is not queued yet. It runs under the
// service's lock, which keeps the frames in order.
func (d *Daemon) oniEmit(p feature.Payload) {
	frame, err := feature.EncodeFrame(p)
	if err != nil {
		d.warn("oni frame: %v", err)
		return
	}
	d.oni.clientsMu.Lock()
	defer d.oni.clientsMu.Unlock()
	for c, client := range d.oni.clients {
		if client.holding {
			client.held = append(client.held, frame)
			continue
		}
		c.sendEvent(protocol.EvtOni, json.RawMessage(frame))
	}
}

// oniPlace saves where Oni stands, through the running presence so its
// renderers follow, or straight to the file when nothing has Oni open.
func (d *Daemon) oniPlace(p protocol.OniPlaceParams) error {
	d.oni.mu.Lock()
	defer d.oni.mu.Unlock()
	svc := d.oni.service.Load()
	if svc == nil {
		svc = presence.New(presence.SystemClock(), nil)
		defer svc.Close()
	}
	if err := svc.Place(p); err != nil {
		return protocol.Errorf(protocol.CodeInternal, "%s", i18n.Tf("Oni's place could not be saved: %v.", err))
	}
	return nil
}

func (d *Daemon) oniSettingsChanged(current, next protocol.Settings) {
	d.voiceSettings(next)
	d.routineSettings(next)
	d.reflexSettings(next)
	d.remindSettings(next)
	d.handsSettings(current, next)
	d.guardSettings(current, next)
	d.memorySettings(next)
	d.recallSettings(next)
	d.assignSettings(next)
	if d.oni.service.Load() == nil {
		return
	}
	d.oni.mu.Lock()
	defer d.oni.mu.Unlock()
	svc := d.oni.service.Load()
	if svc == nil {
		return
	}
	if current.Oni.Character != next.Oni.Character {
		svc.Character(oniCharacterFrame(next.Oni.Character))
	}
	was, now := protocol.EffectiveOni(current), protocol.EffectiveOni(next)
	svc.Settings(now)
	// a question asked under other rules is not answered under these
	if feature.Decide(feature.Request{Feature: feature.Inspect}, now) != feature.Decide(feature.Request{Feature: feature.Inspect}, was) || was.Disclosure != now.Disclosure {
		d.oniAskReset()
		d.pickDisarm(0)
	}
	// a guide started under other rules does not go on under these
	for _, id := range []string{feature.Guide, feature.GuideKeep} {
		if feature.Decide(feature.Request{Feature: id}, now) != feature.Decide(feature.Request{Feature: id}, was) || was.Disclosure != now.Disclosure {
			d.guideUnwind("settings")
			break
		}
	}
	// a hint out goes when hints are turned off
	if hintsOn(was) && !hintsOn(now) {
		d.hintsOff()
	}
}

// oniSessionInfo passes a session's new state to presence while it runs.
func (d *Daemon) oniSessionInfo(data any) {
	svc := d.oni.service.Load()
	if svc == nil {
		return
	}
	switch info := data.(type) {
	case protocol.SessionInfo:
		svc.Activity(oniActivity(info))
	case *protocol.SessionInfo:
		if info != nil {
			svc.Activity(oniActivity(*info))
		}
	}
}

func oniActivity(info protocol.SessionInfo) presence.Activity {
	return presence.Activity{Session: info.ID, State: info.State, Tool: info.Activity != ""}
}

// runningSessions are the sessions with a turn running or waiting on the
// user.
func (d *Daemon) runningSessions() []protocol.SessionInfo {
	m := d.manager
	m.mu.Lock()
	items := make([]*managedSession, 0, len(m.items))
	for _, s := range m.items {
		items = append(items, s)
	}
	m.mu.Unlock()
	var out []protocol.SessionInfo
	for _, s := range items {
		if info := s.log.Info(); info.State == protocol.StateWorking || info.State == protocol.StateWaiting {
			out = append(out, info)
		}
	}
	return out
}

// oniCharacterFrame names the character in use, the shipped one when the
// selected one does not load, with the slots that fell back.
func oniCharacterFrame(selected string) feature.CharacterFrame {
	characters := loadOniCharacters()
	active := activeOniCharacter(selected, characters)
	for _, c := range characters {
		if c.ID == active {
			return feature.CharacterFrame{ID: c.ID, Name: c.Name, Fallbacks: c.Fallbacks}
		}
	}
	shipped := profile.Default()
	return feature.CharacterFrame{ID: shipped.Profile.ID, Name: shipped.Profile.Name}
}

// oniAssets is a valid character's profile.json and declared SVGs as the
// loader validated them.
func oniAssets(id string) (protocol.OniCharacterAssets, error) {
	for _, l := range profile.List(paths.CharactersDir()) {
		if l.Profile.ID != id {
			continue
		}
		if !l.Valid {
			return protocol.OniCharacterAssets{}, protocol.Errorf(protocol.CodeBadRequest, "%s", i18n.Tf("The Oni character %s does not load: %s.", id, strings.Join(l.Report.Errors, "; ")))
		}
		return protocol.OniCharacterAssets{ID: id, Files: l.Files}, nil
	}
	return protocol.OniCharacterAssets{}, protocol.Errorf(protocol.CodeNotFound, "%s", i18n.Tf("There is no Oni character %s.", id))
}
