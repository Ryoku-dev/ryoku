package teams

import (
	"slices"
	"strings"
	"unicode/utf8"

	"ryoku-rashin/internal/config"
	"ryoku-rashin/internal/memberconfig"
	"ryoku-rashin/internal/protocol"
)

func normaliseMemberConfig(team *protocol.Team, _ protocol.Settings) error {
	if team.Structure != "" && team.Purpose == "" {
		return protocol.Errorf(protocol.CodeBadRequest, "A Yokai structure needs a goal.")
	}
	for i := range team.Members {
		member := &team.Members[i]
		member.Rank = strings.TrimSpace(member.Rank)
		member.Spirit = strings.TrimSpace(member.Spirit)
		member.Seat = strings.TrimSpace(member.Seat)
		member.Capabilities = strings.TrimSpace(member.Capabilities)
		member.Trust = strings.TrimSpace(member.Trust)
		member.Persona = strings.TrimSpace(member.Persona)
		for _, text := range []string{member.Rank, member.Spirit, member.Seat, member.Persona} {
			if utf8.RuneCountInString(text) > protocol.EngineModelMax || strings.ContainsRune(text, 0) {
				return protocol.Errorf(protocol.CodeBadRequest, "A seat identity is too long or contains a zero byte.")
			}
		}
		if utf8.RuneCountInString(member.Capabilities) > protocol.MemberCapabilitiesMaxRunes {
			return protocol.Errorf(protocol.CodeBadRequest, "A capabilities description has at most %d characters.", protocol.MemberCapabilitiesMaxRunes)
		}
		if member.Trust != "" && !protocol.KnownApprovals(member.Trust) {
			return protocol.Errorf(protocol.CodeBadRequest, "Member trust must be read-only, ask, or auto.")
		}
		if member.Tools != nil && (member.Tools.Screen && !team.Tools.Screen || member.Tools.Mail && !team.Tools.Mail) {
			return protocol.Errorf(protocol.CodeForbidden, "A member's tools cannot exceed its team's grants.")
		}
		if member.Skills != nil {
			ids := make([]string, len(*member.Skills))
			for j, id := range *member.Skills {
				ids[j] = strings.TrimSpace(id)
				if ids[j] == "" || len(ids[j]) > 200 || strings.ContainsRune(ids[j], 0) {
					return protocol.Errorf(protocol.CodeBadRequest, "A skill grant needs a valid skill-set ID.")
				}
			}
			slices.Sort(ids)
			ids = slices.Compact(ids)
			member.Skills = &ids
		}
		if member.Budget != nil {
			if err := config.CheckBudget(*member.Budget); err != nil {
				return err
			}
		}
		if err := checkMemberRuntime(member.Runtime); err != nil {
			return err
		}
		if err := checkMemberHeartbeat(member.Heartbeat); err != nil {
			return err
		}
		if !member.Paused {
			member.PauseReason = ""
		} else if member.PauseReason == "" {
			member.PauseReason = protocol.PausedByYou
		} else if member.PauseReason != protocol.PausedByYou && member.PauseReason != protocol.PausedByBudget {
			return protocol.Errorf(protocol.CodeBadRequest, "A member pause reason must be you or budget.")
		}
		if member.Revision == 0 {
			member.Revision = 1
		}
		if err := memberconfig.CheckBounds(*member); err != nil {
			return err
		}
	}
	return nil
}

func checkMemberRuntime(runtime *protocol.MemberRuntime) error {
	if runtime == nil {
		return nil
	}
	if runtime.Kind == "" {
		runtime.Kind = protocol.MemberRuntimeNative
	}
	switch runtime.Kind {
	case protocol.MemberRuntimeNative:
		if len(runtime.Command) != 0 || runtime.URL != "" || runtime.Model != "" || len(runtime.Env) != 0 || len(runtime.SecretEnv) != 0 || runtime.Credential != "" {
			return protocol.Errorf(protocol.CodeBadRequest, "Native runtime uses the member's engine, not command or webhook configuration.")
		}
	case protocol.MemberRuntimeCLI, protocol.MemberRuntimeCommand:
		if len(runtime.Command) == 0 || len(runtime.Command) > 64 || strings.TrimSpace(runtime.Command[0]) == "" || runtime.URL != "" || runtime.Credential != "" {
			return protocol.Errorf(protocol.CodeBadRequest, "A local runtime needs an argv of at most 64 arguments and no webhook credentials.")
		}
		for _, argument := range runtime.Command {
			if len(argument) > 16384 || strings.ContainsRune(argument, 0) {
				return protocol.Errorf(protocol.CodeBadRequest, "A runtime argument is too long or contains a zero byte.")
			}
		}
		if runtime.Kind == protocol.MemberRuntimeCommand && runtime.Model != "" {
			return protocol.Errorf(protocol.CodeBadRequest, "A command configures its model through its argv, not a CLI-agent model ID.")
		}
	case protocol.MemberRuntimeWebhook:
		if _, err := checkRuntimeEndpoint(runtime.URL); err != nil {
			return err
		}
		if len(runtime.Command) != 0 || runtime.Model != "" || len(runtime.Env) != 0 || len(runtime.SecretEnv) != 0 {
			return protocol.Errorf(protocol.CodeBadRequest, "A webhook does not run a local command or inherit local environment variables.")
		}
	default:
		return protocol.Errorf(protocol.CodeBadRequest, "Runtime must be native, cli, command, or webhook.")
	}
	if utf8.RuneCountInString(runtime.Model) > protocol.EngineModelMax || strings.ContainsRune(runtime.Model, 0) {
		return protocol.Errorf(protocol.CodeBadRequest, "The CLI-agent model ID is too long or contains a zero byte.")
	}
	if len(runtime.Env)+len(runtime.SecretEnv) > 64 {
		return protocol.Errorf(protocol.CodeBadRequest, "A runtime has at most 64 environment entries.")
	}
	for key, value := range runtime.Env {
		if !runtimeEnvName(key) || strings.ContainsRune(value, 0) || len(value) > 16384 {
			return protocol.Errorf(protocol.CodeBadRequest, "A runtime environment entry is invalid.")
		}
		if _, secret := runtime.SecretEnv[key]; secret {
			return protocol.Errorf(protocol.CodeBadRequest, "An environment entry cannot be both public and a vault reference.")
		}
	}
	for key, name := range runtime.SecretEnv {
		if !runtimeEnvName(key) || strings.TrimSpace(name) == "" || len(name) > 200 || strings.ContainsRune(name, 0) {
			return protocol.Errorf(protocol.CodeBadRequest, "A secret environment entry must name a vault credential.")
		}
	}
	return nil
}

func runtimeEnvName(name string) bool {
	if name == "" || len(name) > 200 {
		return false
	}
	for i, c := range name {
		if c != '_' && !(c >= 'a' && c <= 'z') && !(c >= 'A' && c <= 'Z') && !(i > 0 && c >= '0' && c <= '9') {
			return false
		}
	}
	return true
}

func checkMemberHeartbeat(heartbeat *protocol.MemberHeartbeat) error {
	if heartbeat == nil {
		return nil
	}
	seen := make(map[string]bool, len(heartbeat.Modes))
	for _, mode := range heartbeat.Modes {
		if mode != protocol.HeartbeatTask && mode != protocol.HeartbeatRoutine && mode != protocol.HeartbeatMessage && mode != protocol.HeartbeatInterval || seen[mode] {
			return protocol.Errorf(protocol.CodeBadRequest, "Heartbeat modes must be distinct task, routine, message, or interval values.")
		}
		seen[mode] = true
	}
	if seen[protocol.HeartbeatInterval] {
		if heartbeat.EveryMinutes < 1 || heartbeat.EveryMinutes > 31*24*60 {
			return protocol.Errorf(protocol.CodeBadRequest, "An interval wake must be between one minute and 31 days.")
		}
	} else if heartbeat.EveryMinutes != 0 {
		return protocol.Errorf(protocol.CodeBadRequest, "A wake interval requires interval mode.")
	}
	return nil
}

func memberWakeAllowed(member protocol.Member, mode string) bool {
	if member.Heartbeat == nil {
		return mode == protocol.HeartbeatTask || mode == protocol.HeartbeatRoutine
	}
	return slices.Contains(member.Heartbeat.Modes, mode)
}
