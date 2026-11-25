package engine

import (
	"context"
	"fmt"
	"strings"
	"sync"

	i18n "ryoku-i18n"
	"ryoku-rashin/internal/core"
	"ryoku-rashin/internal/engine/tool"
	"ryoku-rashin/internal/protocol"
)

// Permission card option ids, kept from Ask so open cards in a session made
// before the native engine still answer.
const (
	optionAllow  = "allow"
	optionAlways = "always"
	optionRefuse = "refuse"
	optionBuild  = "build"
	optionKeep   = "keep"
)

// refusedToTool is told to the model as final: asking again another way is
// how a model wears a user down.
const refusedToTool = "The user refused this. Do not try the same change another way. Tell them it was not done and ask what they want instead."

// gate is one session's approvals: it decides whether a change runs, asks
// the user with a permission card when it must, and holds the open cards
// until Permission answers them or the turn ends. A subagent shares its
// parent's gate, so allowing the conversation covers its children too.
type gate struct {
	// approvals is the session's value now (protocol.Approvals*).
	approvals func() string
	// ask marks an Ask session: there a command the classifier rates a
	// read never waits, whatever the approvals.
	ask bool

	mu      sync.Mutex
	sink    core.Sink
	allowed bool
	asked   int
	pending map[string]*pendingCard
}

type pendingCard struct {
	answer  chan string
	options []protocol.PermissionOption
	sink    core.Sink
}

func newGate(approvals func() string, ask bool) *gate {
	return &gate{approvals: approvals, ask: ask, pending: map[string]*pendingCard{}}
}
func (g *gate) setApprovals(approvals func() string) {
	g.mu.Lock()
	g.approvals = approvals
	g.mu.Unlock()
}

func (g *gate) approvalMode() string {
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.approvals()
}

// bind sets the sink cards of the running turn go to.
func (g *gate) bind(sink core.Sink) {
	g.mu.Lock()
	g.sink = sink
	g.mu.Unlock()
}

// waits reports whether a change waits for the user. A destructive change
// always does and is never covered by "allow in this conversation"; auto
// runs everything else; a read waits only under Agent's ask, where
// everything asks; otherwise the user's earlier "allow in this
// conversation" decides.
func (g *gate) waits(tier string) bool {
	g.mu.Lock()
	approvals := g.approvals()
	allowed := g.allowed
	g.mu.Unlock()
	switch {
	case tier == protocol.TierDanger:
		return true
	case approvals == protocol.ApprovalsAuto:
		return false
	case tier == protocol.TierRead && (g.ask || approvals == protocol.ApprovalsReadOnly):
		return false
	case approvals == protocol.ApprovalsReadOnly:
		return true
	}
	return !allowed
}

// Allow implements tool.Permissions.
func (g *gate) Allow(ctx context.Context, req tool.Request) (tool.Decision, error) {
	if !g.waits(req.Tier) {
		return tool.Decision{Allowed: true, By: "policy"}, nil
	}
	options := []protocol.PermissionOption{{ID: optionAllow, Label: i18n.T("Allow once"), Kind: protocol.OptAllowOnce}}
	if req.Tier != protocol.TierDanger {
		options = append(options, protocol.PermissionOption{ID: optionAlways, Label: i18n.T("Allow in this conversation"), Kind: protocol.OptAllowAlways})
	}
	options = append(options, protocol.PermissionOption{ID: optionRefuse, Label: i18n.T("Refuse"), Kind: protocol.OptRejectOnce})
	kind := req.Kind
	if kind == "" {
		kind = protocol.ToolOther
	}
	option, err := g.card(ctx, protocol.PermissionData{Call: req.Call.ID, Title: req.Title, Kind: kind, Input: req.Input, Tier: req.Tier, Options: options})
	if err != nil {
		return tool.Decision{Reason: i18n.T("Cancelled before it ran."), By: "user"}, err
	}
	switch option {
	case optionRefuse:
		return tool.Decision{Reason: refusedToTool, By: "user"}, nil
	case optionAlways:
		g.mu.Lock()
		g.allowed = true
		g.mu.Unlock()
	}
	return tool.Decision{Allowed: true, By: "user"}, nil
}

// Choose implements tool.Permissions: the ask tool's question card, the
// suggested option first.
func (g *gate) Choose(ctx context.Context, q tool.Question) (string, error) {
	options := make([]protocol.PermissionOption, 0, len(q.Options))
	for _, choice := range q.Options {
		option := protocol.PermissionOption{ID: choice.ID, Label: choice.Name, Kind: protocol.OptAllowOnce}
		if choice.ID == q.Recommended && q.Recommended != "" {
			option.Label = strings.TrimSpace(option.Label + " " + i18n.T("(recommended)"))
			options = append([]protocol.PermissionOption{option}, options...)
			continue
		}
		options = append(options, option)
	}
	if len(options) == 0 {
		return "", fmt.Errorf("%s", i18n.T("The question has no options to choose from."))
	}
	return g.card(ctx, protocol.PermissionData{Call: q.Call.ID, Title: q.Question, Kind: protocol.PermissionQuestion, Options: options})
}

// approvePlan puts a finished plan to the user: Start building or Keep
// planning. It reports whether to build.
func (g *gate) approvePlan(ctx context.Context, plan string) (bool, error) {
	options := []protocol.PermissionOption{
		{ID: optionBuild, Label: i18n.T("Start building"), Kind: protocol.OptAllowOnce},
		{ID: optionKeep, Label: i18n.T("Keep planning"), Kind: protocol.OptRejectOnce},
	}
	option, err := g.card(ctx, protocol.PermissionData{Title: i18n.T("The plan is ready"), Kind: protocol.PermissionPlan, Input: plan, Options: options})
	if err != nil {
		return false, err
	}
	return option == optionBuild, nil
}

// card shows one permission card and waits for the answer. A turn that ends
// first resolves the card as cancelled and returns ctx's error.
func (g *gate) card(ctx context.Context, data protocol.PermissionData) (string, error) {
	answer := make(chan string, 1)
	g.mu.Lock()
	g.asked++
	prefix := data.Call
	if prefix == "" {
		prefix = data.Kind
	}
	data.Request = fmt.Sprintf("%s-ask-%d", prefix, g.asked)
	sink := g.sink
	g.pending[data.Request] = &pendingCard{answer: answer, options: data.Options, sink: sink}
	g.mu.Unlock()
	if sink != nil {
		sink.Emit(protocol.EvPermission, data)
	}
	select {
	case option := <-answer:
		return option, nil
	case <-ctx.Done():
		g.drop(data.Request)
		return "", ctx.Err()
	}
}

// drop resolves one open card as cancelled.
func (g *gate) drop(request string) {
	g.mu.Lock()
	open, ok := g.pending[request]
	delete(g.pending, request)
	g.mu.Unlock()
	if ok && open.sink != nil {
		open.sink.Emit(protocol.EvPermissionResolved, protocol.PermissionResolvedData{Request: request, Outcome: "cancelled", By: "user"})
	}
}

// cancelAll resolves every open card as cancelled: Cancel and Stop.
func (g *gate) cancelAll() {
	g.mu.Lock()
	requests := make([]string, 0, len(g.pending))
	for request := range g.pending {
		requests = append(requests, request)
	}
	g.mu.Unlock()
	for _, request := range requests {
		g.drop(request)
	}
}

// resolve answers an open card: Runner.Permission.
func (g *gate) resolve(request, option string) error {
	g.mu.Lock()
	open, ok := g.pending[request]
	if !ok {
		g.mu.Unlock()
		return protocol.Errorf(protocol.CodeNotFound, "%s", i18n.T("This request is no longer open. Ask again."))
	}
	offered := false
	for _, o := range open.options {
		if o.ID == option {
			offered = true
		}
	}
	if !offered {
		g.mu.Unlock()
		return protocol.Errorf(protocol.CodeBadRequest, "%s", i18n.T("This answer is not one of the choices. Pick one of those shown."))
	}
	delete(g.pending, request)
	g.mu.Unlock()
	if open.sink != nil {
		open.sink.Emit(protocol.EvPermissionResolved, protocol.PermissionResolvedData{Request: request, Option: option, Outcome: "selected", By: "user"})
	}
	open.answer <- option
	return nil
}

// forget takes back "allow in this conversation": the session went
// read-only.
func (g *gate) forget() {
	g.mu.Lock()
	g.allowed = false
	g.mu.Unlock()
}

// allowedAll reports the conversation-wide allowance, kept in State.
func (g *gate) allowedAll() bool {
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.allowed
}

func (g *gate) setAllowed(on bool) {
	g.mu.Lock()
	g.allowed = on
	g.mu.Unlock()
}

// planText renders a plan for the approval card, one line per entry.
func planText(plan *protocol.PlanData) string {
	if plan == nil {
		return ""
	}
	var b strings.Builder
	for _, e := range plan.Entries {
		mark := " "
		switch e.Status {
		case "completed":
			mark = "x"
		case "in_progress":
			mark = "~"
		}
		fmt.Fprintf(&b, "- [%s] %s\n", mark, strings.TrimSpace(e.Content))
	}
	return strings.TrimRight(b.String(), "\n")
}

var _ tool.Permissions = (*gate)(nil)
