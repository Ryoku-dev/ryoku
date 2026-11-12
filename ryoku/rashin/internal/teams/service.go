// Package teams is Rashin's control plane for teams of agents. A team has a
// purpose on this computer, a folder, a lead and members, the tools and
// credentials it may use, routines it runs by itself and a monthly spending
// limit. Its tasks run as ordinary Agent sessions and end in reports; what
// needs the user waits as an approval, and the vault keeps the credentials
// teams use without ever showing them to an agent.
package teams

import (
	"context"
	"encoding/json"
	"strings"
	"sync"
	"time"

	i18n "ryoku-i18n"

	"ryoku-rashin/internal/config"
	"ryoku-rashin/internal/core"
	registry "ryoku-rashin/internal/harness"
	"ryoku-rashin/internal/paths"
	"ryoku-rashin/internal/protocol"
	"ryoku-rashin/internal/skills"
	"ryoku-rashin/internal/vault"
)

// Service implements core.Teams. Its parts live in their own files: the
// persisted state (store.go), the runs of tasks as sessions (runner.go), the
// engine they run on (engine.go), their time limits (minutes.go), the
// routine clock (schedule.go), the ledger and spend (ledger.go, spend.go),
// approvals (approvals.go), pruning (prune.go), the settings it follows
// (settings.go) and the vault methods (credentials.go).
type Service struct {
	mu       sync.Mutex
	events   core.Broadcaster
	settings func() protocol.Settings
	sessions core.TeamSessions
	now      func() time.Time
	// defaultHarness is the registry's default harness for these settings,
	// the last step of the engine chain
	defaultHarness func(protocol.Settings) string
	// signedIn reports whether a team is signed in to a harness
	signedIn func(harness, team string) (time.Time, bool)
	// teamsSettings are the teams settings last seen, so a change is told
	// from what it changed
	teamsSettings protocol.TeamsSettings

	state  *state
	ledger *ledger
	runs   map[string]*run
	clock  *scheduler
	vault  *vault.Vault
	// memory keeps what teams remember; nil when the daemon runs without it
	memory core.Memory
	// skills are the built-in skill sets its members are given; nil when the
	// daemon runs without them
	skills *skills.Library

	// effects carry out an approval's action once the user decides, by kind
	effects map[string]approvalEffect
	// proposing keeps two proposals from both passing the open limit
	proposing sync.Mutex

	background context.Context
	cancel     context.CancelFunc
}

// approvalEffect carries out what an approval asked for once the user
// decided, and returns what the waiting agent is told.
type approvalEffect func(ctx context.Context, rec *approvalRecord, p protocol.ApprovalDecideParams) (string, error)

// caller is the team task a call from an agent's MCP server belongs to.
type caller struct {
	Team    string
	Task    string
	Member  string
	Session string
}

// Open loads the teams' state and opens the vault. A vault that cannot open,
// because there is no login keyring, leaves teams working without it.
func Open(events core.Broadcaster, settings func() protocol.Settings) (*Service, error) {
	return open(paths.TeamsDir(), vault.Open(paths.CredentialVault()), setWakeUnits, events, settings)
}

// open builds the service on the state under dir, with the vault it uses and
// wake, which turns the unit that wakes Rashin for its routines on or off.
func open(dir string, v *vault.Vault, wake func(on bool), events core.Broadcaster, settings func() protocol.Settings) (*Service, error) {
	if settings == nil {
		settings = config.Defaults
	}
	background, cancel := context.WithCancel(context.Background())
	s := &Service{
		events: events, settings: settings, now: time.Now, vault: v, defaultHarness: registry.DefaultID, signedIn: registry.SignedIn,
		runs: make(map[string]*run), background: background, cancel: cancel,
	}
	var err error
	if s.state, err = openState(dir); err != nil {
		cancel()
		_ = v.Close()
		return nil, err
	}
	if s.ledger, err = openLedger(dir); err != nil {
		cancel()
		_ = v.Close()
		return nil, err
	}
	s.clock = newScheduler(s, wake)
	s.effects = map[string]approvalEffect{
		protocol.ApprovalAction:     s.effectAsked,
		protocol.ApprovalBudget:     s.effectBudget,
		protocol.ApprovalMail:       s.effectMail,
		protocol.ApprovalCredential: s.effectCredential,
		protocol.ApprovalRequest:    s.effectRequest,
		protocol.ApprovalTeam:       s.effectTeam,
	}
	return s, nil
}

// Bind hands the service the session manager and starts what runs by itself:
// queued tasks, the routine clock and the daily pruning. Tasks a restart
// left while every team is paused stop, and team homes every team on a
// harness once shared become one team's own first.
func (s *Service) Bind(sessions core.TeamSessions) {
	settings := s.settings()
	s.migrateHomes()
	s.mu.Lock()
	s.sessions = sessions
	s.teamsSettings = settings.Teams
	s.mu.Unlock()
	if settings.Teams.Paused {
		for _, team := range s.state.teams() {
			s.stopTeamTasks(team.ID, i18n.T("Every team was paused."))
		}
	}
	s.recoverRuns()
	s.clock.start()
	s.pruneDaily()
	s.pump()
}

// Handle serves one teams.* or vault.* method.
func (s *Service) Handle(ctx context.Context, method string, raw json.RawMessage) (any, error) {
	switch method {
	case protocol.MTeamsList:
		return withoutDreams(s.listTeams()), nil
	case protocol.MTeamsTemplates:
		return protocol.TeamTemplatesResult{Templates: templates()}, nil
	case protocol.MTeamsCreate:
		return call(raw, func(p protocol.TeamCreateParams) (any, error) { return s.createTeam(ctx, p) })
	case protocol.MTeamsPropose:
		return call(raw, func(p protocol.TeamProposeParams) (any, error) { return s.proposeTeam(ctx, p) })
	case protocol.MTeamsUpdate:
		return call(raw, func(p protocol.TeamUpdateParams) (any, error) { return s.updateTeam(ctx, p) })
	case protocol.MTeamsDelete:
		return call(raw, func(p protocol.TeamRef) (any, error) { return protocol.Empty{}, s.deleteTeam(ctx, p.ID) })
	case protocol.MTeamsPause:
		return call(raw, func(p protocol.TeamPauseParams) (any, error) { return s.pauseTeam(ctx, p) })
	case protocol.MTeamsExport:
		return call(raw, func(p protocol.TeamRef) (any, error) { return s.exportTeam(p.ID) })
	case protocol.MTeamsImport:
		return call(raw, func(p protocol.TeamImportParams) (any, error) { return s.importTeam(ctx, p) })
	case protocol.MTeamsTasks:
		return call(raw, func(p protocol.TeamTasksParams) (any, error) { return s.listTasks(p), nil })
	case protocol.MTeamsTaskCreate:
		return call(raw, func(p protocol.TeamTaskCreateParams) (any, error) { return s.createTask(ctx, p) })
	case protocol.MTeamsTaskStop:
		return call(raw, func(p protocol.TeamTaskRef) (any, error) { return s.stopTask(ctx, p.ID, i18n.T("You stopped it.")) })
	case protocol.MTeamsTaskRetry:
		return call(raw, func(p protocol.TeamTaskRetryParams) (any, error) { return s.retryTask(ctx, p) })
	case protocol.MTeamsTaskRead:
		return call(raw, func(p protocol.TeamTaskRef) (any, error) { return s.readTask(p.ID) })
	case protocol.MTeamsTaskDelete:
		return call(raw, func(p protocol.TeamTaskRef) (any, error) { return protocol.Empty{}, s.deleteTask(ctx, p.ID) })
	case protocol.MTeamsRoutines:
		return call(raw, func(p protocol.TeamRef) (any, error) { return s.listRoutines(p.ID), nil })
	case protocol.MTeamsRoutineCreate:
		return call(raw, func(p protocol.RoutineCreateParams) (any, error) { return s.createRoutine(ctx, p) })
	case protocol.MTeamsRoutineUpdate:
		return call(raw, func(p protocol.RoutineUpdateParams) (any, error) { return s.updateRoutine(ctx, p) })
	case protocol.MTeamsRoutineDelete:
		return call(raw, func(p protocol.RoutineRef) (any, error) { return protocol.Empty{}, s.deleteRoutine(p.ID) })
	case protocol.MTeamsRoutineRun:
		return call(raw, func(p protocol.RoutineRef) (any, error) { return s.runRoutine(ctx, p.ID, protocol.InitiatorYou) })
	case protocol.MTeamsRoutinePreview:
		return call(raw, func(p protocol.RoutinePreviewParams) (any, error) { return previewSchedule(p, s.now()), nil })
	case protocol.MTeamsApprovals:
		return call(raw, func(p protocol.ApprovalsParams) (any, error) { return s.listApprovals(p), nil })
	case protocol.MTeamsApprovalDecide:
		return call(raw, func(p protocol.ApprovalDecideParams) (any, error) { return s.decideApproval(ctx, p) })
	case protocol.MTeamsActivity:
		return call(raw, func(p protocol.TeamActivityParams) (any, error) { return s.ledger.list(p) })
	case protocol.MTeamsSpend:
		return call(raw, func(p protocol.TeamSpendParams) (any, error) { return s.spend(p) })
	case protocol.MTeamsPrune:
		return call(raw, func(p protocol.TeamPruneParams) (any, error) { return s.prune(ctx, p) })
	case protocol.MTeamsDreamCreate:
		return call(raw, func(p protocol.DreamCreateParams) (any, error) { return s.dreamCreate(ctx, p) })
	case protocol.MTeamsDreamAnswer:
		return call(raw, func(p protocol.DreamAnswerParams) (any, error) { return s.dreamAnswer(ctx, p) })

	case protocol.MTeamsAgentReport:
		return call(raw, func(p protocol.AgentReportParams) (any, error) { return s.agentReport(ctx, p) })
	case protocol.MTeamsAgentDelegate:
		return call(raw, func(p protocol.AgentDelegateParams) (any, error) { return s.agentDelegate(ctx, p) })
	case protocol.MTeamsAgentStatus:
		return call(raw, func(p protocol.AgentRef) (any, error) { return s.agentStatus(ctx, p) })
	case protocol.MTeamsAgentAsk:
		return call(raw, func(p protocol.AgentAskParams) (any, error) { return s.agentAsk(ctx, p) })
	case protocol.MTeamsAgentHTTP:
		return call(raw, func(p protocol.AgentHTTPParams) (any, error) { return s.agentHTTP(ctx, p) })
	case protocol.MTeamsAgentMail:
		return call(raw, func(p protocol.AgentMailParams) (any, error) { return s.agentMail(ctx, p) })
	case protocol.MTeamsAgentRequest:
		return call(raw, func(p protocol.AgentCredentialRequestParams) (any, error) { return s.agentRequest(ctx, p) })
	case protocol.MTeamsAgentCredentials:
		return call(raw, func(p protocol.AgentRef) (any, error) { return s.agentCredentials(ctx, p) })
	case protocol.MTeamsAgentHire:
		return call(raw, func(p protocol.AgentHireParams) (any, error) { return s.agentHire(ctx, p) })
	case protocol.MTeamsAgentRelease:
		return call(raw, func(p protocol.AgentReleaseParams) (any, error) { return s.agentRelease(ctx, p) })
	case protocol.MTeamsAgentDream:
		return call(raw, func(p protocol.AgentDreamParams) (any, error) { return s.agentDream(ctx, p) })
	case protocol.MTeamsAgentPresent:
		return call(raw, func(p protocol.AgentPresentParams) (any, error) { return s.agentPresent(ctx, p) })

	case protocol.MVaultInfo:
		return s.vaultInfo(), nil
	case protocol.MVaultAdd:
		return call(raw, func(p protocol.VaultAddParams) (any, error) { return s.vaultAdd(ctx, p) })
	case protocol.MVaultUpdate:
		return call(raw, func(p protocol.VaultUpdateParams) (any, error) { return s.vaultUpdate(ctx, p) })
	case protocol.MVaultRemove:
		return call(raw, func(p protocol.VaultRef) (any, error) { return protocol.Empty{}, s.vaultRemove(ctx, p.ID) })
	case protocol.MVaultEvents:
		return call(raw, func(p protocol.VaultEventsParams) (any, error) { return s.vaultEvents(p) })
	case protocol.MVaultMail:
		return call(raw, func(p protocol.MailAccountParams) (any, error) { return s.vaultMail(ctx, p) })
	case protocol.MVaultMailTest:
		return s.vaultMailTest(ctx), nil
	}
	return nil, protocol.Errorf(protocol.CodeUnsupported, "%s", i18n.Tf("%s is not a Teams method.", method))
}

// IsMethod reports whether a method belongs to Teams.
func IsMethod(method string) bool {
	return strings.HasPrefix(method, "teams.") || strings.HasPrefix(method, "vault.")
}

// Snapshot fills the Teams part of the subscribe snapshot.
func (s *Service) Snapshot(_ context.Context, into *protocol.Snapshot) {
	into.Teams = s.listTeams().Teams
	into.TeamTasks = s.snapshotTasks()
	into.Approvals = s.snapshotApprovals()
	into.Routines = s.listRoutines("").Routines
	into.Vault = s.vaultSnapshot()
}

// Armed reports routines that are on, so the daemon stays up to fire them.
func (s *Service) Armed() bool { return s.clock.armed() }

// Close stops the clock and the work the service started; running sessions
// belong to the session manager, which stops them itself.
func (s *Service) Close() error {
	s.clock.stop()
	s.cancel()
	return s.vault.Close()
}

// broadcast sends a Teams event to every subscribed client.
func (s *Service) broadcast(event string, data any) {
	if s.events != nil {
		s.events.Broadcast(event, data)
	}
}

// call decodes a method's params and runs it.
func call[T any](raw json.RawMessage, run func(T) (any, error)) (any, error) {
	var params T
	if len(raw) != 0 && string(raw) != "null" {
		if err := json.Unmarshal(raw, &params); err != nil {
			return nil, protocol.Errorf(protocol.CodeBadRequest, "%s", i18n.T("The request's parameters are not valid."))
		}
	}
	return run(params)
}
