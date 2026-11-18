package ask

import (
	"context"
	"fmt"
	"io"
	"strings"

	"ryoku-rashin/internal/core"
	"ryoku-rashin/internal/llm"
	"ryoku-rashin/internal/protocol"
)

func (r *runner) glance(ctx context.Context, in core.PromptInput, sink core.Sink, model string) (core.TurnResult, error) {
	p := protocol.GlanceParams{Mode: "ask"}
	if in.Glance != nil {
		p = *in.Glance
		if p.Model != "" {
			model = p.Model
		}
	}
	asked, err := userMessage(in.Text, in.Images)
	if err != nil {
		return core.TurnResult{Stop: protocol.StopError}, err
	}
	messages := []llm.Message{{Role: llm.RoleSystem, Content: protocol.GlanceInstruction(p)}}
	messages = append(messages, historyMessages(in.History, historyBudgetBytes)...)
	messages = append(messages, asked)
	stream, err := r.factory.client.Stream(ctx, llm.Request{Model: model, Strict: pinned(model), LocalOnly: p.Local, Messages: messages, MaxTokens: 4096})
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
	local := provider == protocol.ProviderOllamaLocal || provider == protocol.ProviderLlamaCpp
	sink.Emit(protocol.EvModel, protocol.ChoiceData{Current: via(final), Local: local, Engine: "providers"})
	sink.Emit(protocol.EvAnswer, protocol.AnswerData{Via: via(final)})
	usage := protocol.TokenUsage{Input: final.Usage.Input, Output: final.Usage.Output, Estimated: final.Usage.Estimated}
	return core.TurnResult{Stop: finishStop(final.Finish), Usage: usagePtr(usage)}, nil
}
