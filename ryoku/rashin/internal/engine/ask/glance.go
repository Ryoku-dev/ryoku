package ask

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"

	i18n "ryoku-i18n"
	"ryoku-rashin/internal/core"
	"ryoku-rashin/internal/llm"
	"ryoku-rashin/internal/protocol"
	"ryoku-rashin/internal/writing"
)

// Glance answers a Theia capture without entering the agent tool loop.
func (m *Mode) Glance(ctx context.Context, in core.PromptInput, model string, strict bool, sink core.Sink) (core.TurnResult, error) {
	p := protocol.GlanceParams{Mode: "ask", Text: in.Text, Images: in.Images}
	if in.Glance != nil {
		p = *in.Glance
		if p.Model != "" {
			model, strict = p.Model, !protocol.IsRoute(p.Model)
		}
	}
	asked, err := m.Message(in.Text, in.Images)
	if err != nil {
		return core.TurnResult{Stop: protocol.StopError}, err
	}
	var instruction string
	if p.Options["voice"] == "on" {
		instruction = protocol.GlanceInstructionWithVoice(p, writing.VoiceNotes())
	} else {
		instruction = protocol.GlanceInstruction(p)
	}
	messages := []llm.Message{{Role: llm.RoleSystem, Content: instruction}}
	messages = append(messages, m.History(in.History)...)
	messages = append(messages, asked)
	maxTokens := 4096
	switch p.Mode {
	case "learn", "define", "reply", "aicheck":
		maxTokens = 8192
	}
	stream, err := m.client.Stream(ctx, llm.Request{Model: model, Strict: strict, Light: model == "", LocalOnly: p.Local, Messages: messages, MaxTokens: maxTokens})
	if err != nil {
		return core.TurnResult{Stop: protocol.StopError}, modelError(err)
	}
	defer stream.Close()
	message := fmt.Sprintf("glance-%d", in.Turn)
	for {
		delta, err := stream.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			if ctx.Err() != nil {
				return core.TurnResult{Stop: protocol.StopCancelled}, nil
			}
			return core.TurnResult{Stop: protocol.StopError}, modelError(err)
		}
		if delta.Text != "" {
			sink.Emit(protocol.EvText, protocol.TextData{Message: message, Delta: delta.Text})
		}
	}
	final := stream.Final()
	provider := final.Provider
	if provider == "" {
		provider, _, _ = strings.Cut(final.Model, "/")
	}
	local := provider == protocol.ProviderRashinModels || provider == protocol.ProviderOllamaLocal || provider == protocol.ProviderLlamaCpp
	via := viaModel(final)
	sink.Emit(protocol.EvModel, protocol.ChoiceData{Current: via, Local: local, Engine: "providers"})
	sink.Emit(protocol.EvAnswer, protocol.AnswerData{Via: via})
	usage := protocol.TokenUsage{Input: final.Usage.Input, Output: final.Usage.Output, Estimated: final.Usage.Estimated}
	result := core.TurnResult{Stop: finishStop(final.Finish)}
	if usage.Input != 0 || usage.Output != 0 {
		result.Usage = &usage
	}
	return result, nil
}

func modelError(err error) error {
	var refused *llm.NoRoute
	if errors.As(err, &refused) {
		if refused.NoVision {
			return protocol.Errorf(protocol.CodeNoVision, "%s", refused.Reason)
		}
		return protocol.Errorf(protocol.CodeUnavailable, "%s", refused.Reason)
	}
	return protocol.Errorf(protocol.CodeUnavailable, "%s", i18n.Tf("Ask could not start an answer: %v. See which providers can answer in Rashin > Settings > Providers.", err))
}

func finishStop(finish string) string {
	switch finish {
	case llm.FinishLength:
		return protocol.StopMaxTokens
	case llm.FinishFilter:
		return protocol.StopRefusal
	default:
		return protocol.StopEnd
	}
}

func viaModel(final llm.Final) string {
	model := strings.TrimSpace(final.Model)
	provider := strings.TrimSpace(final.Provider)
	if provider == "" || strings.HasPrefix(model, provider+"/") {
		return model
	}
	if model == "" {
		return provider
	}
	return provider + "/" + model
}
