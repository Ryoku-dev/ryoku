package teams

import (
	"encoding/json"
	"fmt"
	"slices"
	"strings"

	"gioui.org/layout"
	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/kit"
	"ryoku-i18n"
	"ryoku-rashin/internal/app/env"
	"ryoku-rashin/internal/app/look"
	"ryoku-rashin/internal/app/pages/teamsui"
	"ryoku-rashin/internal/protocol"
)

// The optional controls preserve nil (inherit) separately from explicit empty policies.
type memberDetails struct {
	seat, capabilities, persona, runtimeModel, endpoint, credential           kit.TextField
	argv, publicEnv, secretEnv, skillIDs, wakeInstructions                    look.MultiLine
	runtimePick, trustPick                                                    look.Select
	runtimeKind, trust                                                        string
	runtimeOn, heartbeatOn, toolsOn, skillsOn, budgetOn, screen, mail, paused kit.Toggle
	wakes                                                                     [4]kit.Toggle
	depth, interval, warn, turns, minutes                                     kit.Stepper
	limit                                                                     teamsui.TokenStepper
}

func configJSON(v any) string { b, _ := json.MarshalIndent(v, "", "  "); return string(b) }
func (x *memberDetails) set(m protocol.Member) {
	x.seat.SetText(m.Seat)
	x.capabilities.SetText(m.Capabilities)
	x.persona.SetText(m.Persona)
	x.depth.Value, x.trust, x.paused.Value = m.DelegationDepth, m.Trust, m.Paused
	x.runtimeOn.Value = m.Runtime != nil
	r := protocol.MemberRuntime{Kind: protocol.MemberRuntimeNative}
	if m.Runtime != nil {
		r = *m.Runtime
	}
	x.runtimeKind = r.Kind
	x.argv.SetText(configJSON(r.Command))
	x.publicEnv.SetText(configJSON(r.Env))
	x.secretEnv.SetText(configJSON(r.SecretEnv))
	x.runtimeModel.SetText(r.Model)
	x.endpoint.SetText(r.URL)
	x.credential.SetText(r.Credential)
	x.heartbeatOn.Value = m.Heartbeat != nil
	h := protocol.MemberHeartbeat{Modes: []string{protocol.HeartbeatTask, protocol.HeartbeatRoutine}}
	if m.Heartbeat != nil {
		h = *m.Heartbeat
	}
	for i, mode := range wakeModes {
		x.wakes[i].Value = slices.Contains(h.Modes, mode)
	}
	x.interval.Value = h.EveryMinutes
	x.wakeInstructions.SetText(h.Instructions)
	x.toolsOn.Value = m.Tools != nil
	x.screen.Value, x.mail.Value = false, false
	if m.Tools != nil {
		x.screen.Value, x.mail.Value = m.Tools.Screen, m.Tools.Mail
	}
	x.skillsOn.Value = m.Skills != nil
	x.skillIDs.SetText("")
	if m.Skills != nil {
		x.skillIDs.SetText(strings.Join(*m.Skills, "\n"))
	}
	x.budgetOn.Value = m.Budget != nil
	b := protocol.TeamBudget{WarnPercent: 80, TaskTurns: 20, TaskMinutes: 30}
	if m.Budget != nil {
		b = *m.Budget
	}
	x.limit.Value, x.warn.Value, x.turns.Value, x.minutes.Value = b.MonthTokens, b.WarnPercent, b.TaskTurns, b.TaskMinutes
}

var wakeModes = []string{protocol.HeartbeatTask, protocol.HeartbeatRoutine, protocol.HeartbeatMessage, protocol.HeartbeatInterval}

func decodeField(text, label string, target any) error {
	if err := json.Unmarshal([]byte(text), target); err != nil {
		return fmt.Errorf("%s: %w", label, err)
	}
	return nil
}

func (x *memberDetails) apply(m *protocol.Member) error {
	m.Seat, m.Capabilities, m.Persona = strings.TrimSpace(x.seat.Text()), strings.TrimSpace(x.capabilities.Text()), strings.TrimSpace(x.persona.Text())
	wasPaused := m.Paused
	m.DelegationDepth, m.Trust, m.Paused = x.depth.Value, x.trust, x.paused.Value
	if m.Paused != wasPaused {
		m.PauseReason = ""
	}
	if m.Paused && m.PauseReason == "" {
		m.PauseReason = protocol.PausedByYou
	}
	if !m.Paused {
		m.PauseReason = ""
	}
	m.Runtime = nil
	if x.runtimeOn.Value {
		r := &protocol.MemberRuntime{Kind: x.runtimeKind, Model: strings.TrimSpace(x.runtimeModel.Text()), URL: strings.TrimSpace(x.endpoint.Text()), Credential: strings.TrimSpace(x.credential.Text())}
		if err := decodeField(x.argv.Text(), i18n.T("Argument vector"), &r.Command); err != nil {
			return err
		}
		if err := decodeField(x.publicEnv.Text(), i18n.T("Public environment"), &r.Env); err != nil {
			return err
		}
		if err := decodeField(x.secretEnv.Text(), i18n.T("Vault references"), &r.SecretEnv); err != nil {
			return err
		}
		m.Runtime = r
	}
	m.Heartbeat = nil
	if x.heartbeatOn.Value {
		h := &protocol.MemberHeartbeat{Modes: []string{}, EveryMinutes: x.interval.Value, Instructions: strings.TrimSpace(x.wakeInstructions.Text())}
		for i, mode := range wakeModes {
			if x.wakes[i].Value {
				h.Modes = append(h.Modes, mode)
			}
		}
		m.Heartbeat = h
	}
	m.Tools = nil
	if x.toolsOn.Value {
		m.Tools = &protocol.TeamTools{Screen: x.screen.Value, Mail: x.mail.Value}
	}
	m.Skills = nil
	if x.skillsOn.Value {
		ids := []string{}
		for _, id := range strings.Split(x.skillIDs.Text(), "\n") {
			if id = strings.TrimSpace(id); id != "" {
				ids = append(ids, id)
			}
		}
		m.Skills = &ids
	}
	m.Budget = nil
	if x.budgetOn.Value {
		m.Budget = &protocol.TeamBudget{MonthTokens: x.limit.Value, WarnPercent: x.warn.Value, TaskTurns: x.turns.Value, TaskMinutes: x.minutes.Value}
	}
	return nil
}

func toggleField(e *env.Env, t *kit.Toggle, label string) layout.Widget {
	t.Label = label
	return func(gtx layout.Context) layout.Dimensions { return t.Layout(gtx, e.Theme.Theme) }
}

func multiField(e *env.Env, f *look.MultiLine, label, help string) layout.Widget {
	f.Label, f.Help, f.Lines = label, help, 4
	return func(gtx layout.Context) layout.Dimensions { return f.Layout(gtx, e.Theme) }
}

func (x *memberDetails) runtimeRows(e *env.Env) []layout.Widget {
	x.interval.Min, x.interval.Max, x.interval.Step, x.interval.Unit = 0, 31*24*60, 1, i18n.T("minutes")
	rows := []layout.Widget{toggleField(e, &x.runtimeOn, i18n.T("Explicit runtime configuration"))}
	if x.runtimeOn.Value {
		rows = append(rows, choose(e, &x.runtimePick, i18n.T("Runtime mode"), []string{protocol.MemberRuntimeNative, protocol.MemberRuntimeCLI, protocol.MemberRuntimeCommand, protocol.MemberRuntimeWebhook}, []string{i18n.T("Native"), i18n.T("ACP CLI"), i18n.T("Command"), i18n.T("Webhook")}, &x.runtimeKind),
			multiField(e, &x.argv, i18n.T("Argument vector (JSON array)"), i18n.T("One executable and its arguments; never a shell fragment. Native/webhook use null or [].")),
			field(e, &x.runtimeModel, i18n.T("Runtime model ID (CLI only)")), field(e, &x.endpoint, i18n.T("Webhook endpoint")),
			multiField(e, &x.publicEnv, i18n.T("Public environment (JSON object)"), i18n.T("Not for API keys. Actual secrets belong in the vault; native/webhook use null or {}.")),
			multiField(e, &x.secretEnv, i18n.T("Environment vault references (JSON object)"), i18n.T("Map environment names to existing vault entry names, never secret values. Process-readable consent is still required.")),
			field(e, &x.credential, i18n.T("Webhook credential vault reference")))
	}
	rows = append(rows, toggleField(e, &x.heartbeatOn, i18n.T("Explicit heartbeat / wake policy")))
	if x.heartbeatOn.Value {
		labels := []string{i18n.T("Wake for tasks"), i18n.T("Wake for routines"), i18n.T("Wake for messages"), i18n.T("Wake on interval")}
		for i := range x.wakes {
			rows = append(rows, toggleField(e, &x.wakes[i], labels[i]))
		}
		rows = append(rows, labelled(e.Theme, i18n.T("Wake interval (zero unless interval is enabled)"), func(gtx layout.Context) layout.Dimensions { return x.interval.Layout(gtx, e.Theme.Theme) }), multiField(e, &x.wakeInstructions, i18n.T("Heartbeat instructions"), i18n.T("No selected modes parks automatic wakes. Inherited policy allows task and routine wakes.")))
	}
	return rows
}

func (x *memberDetails) policyRows(e *env.Env, team protocol.Team) []layout.Widget {
	th := e.Theme
	rows := []layout.Widget{quiet(th, i18n.T("These policies only narrow the team's grants. Rank, spirit and persona never grant authority.")), choose(e, &x.trustPick, i18n.T("Member approval posture"), []string{"", protocol.ApprovalsReadOnly, protocol.ApprovalsAsk, protocol.ApprovalsAuto}, []string{i18n.T("Inherit team"), i18n.T("Read only"), i18n.T("Ask"), i18n.T("Automatic within team limits")}, &x.trust), toggleField(e, &x.toolsOn, i18n.T("Narrow team tools"))}
	if x.toolsOn.Value {
		rows = append(rows, toggleField(e, &x.screen, i18n.T("Allow own screen (requires team grant)")), toggleField(e, &x.mail, i18n.T("Allow mail (requires team grant)")))
	}
	rows = append(rows, toggleField(e, &x.skillsOn, i18n.T("Narrow skills to an allowlist")))
	if x.skillsOn.Value {
		rows = append(rows, multiField(e, &x.skillIDs, i18n.T("Allowed skill-set IDs (one per line)"), i18n.T("Empty permits none. Team/global disabled skills remain disabled.")))
	}
	rows = append(rows, toggleField(e, &x.budgetOn, i18n.T("Explicit member budget")))
	if x.budgetOn.Value {
		x.warn.Min, x.warn.Max, x.warn.Step, x.warn.Unit = 1, 99, 1, i18n.T("percent")
		x.turns.Min, x.turns.Max, x.turns.Step, x.turns.Unit = 1, protocol.TeamTaskTurnsMax, 1, i18n.T("turns")
		x.minutes.Min, x.minutes.Max, x.minutes.Step, x.minutes.Unit = 0, protocol.TeamTaskMinutesMax, 1, i18n.T("minutes")
		rows = append(rows, labelled(th, i18n.T("Member monthly tokens"), func(gtx layout.Context) layout.Dimensions { return x.limit.Layout(gtx, th) }), labelled(th, i18n.T("Warn at"), func(gtx layout.Context) layout.Dimensions { return x.warn.Layout(gtx, th.Theme) }), labelled(th, i18n.T("Member turns per task"), func(gtx layout.Context) layout.Dimensions { return x.turns.Layout(gtx, th.Theme) }), labelled(th, i18n.T("Member minutes per task"), func(gtx layout.Context) layout.Dimensions { return x.minutes.Layout(gtx, th.Theme) }))
	}
	return append(rows, toggleField(e, &x.paused, i18n.T("Pause this spirit")), field(e, &x.persona, i18n.T("Persona identity (voice / art profile)")), quiet(th, i18n.T("Persona is presentation, not permission. Budget-paused spirits still require a sufficient approved limit before work resumes.")))
}
