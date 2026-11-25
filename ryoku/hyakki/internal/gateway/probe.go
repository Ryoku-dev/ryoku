package gateway

import (
	"context"
	"errors"
	"io"
	"strings"

	i18n "ryoku-i18n"
	"ryoku-rashin/internal/llm"
	"ryoku-rashin/internal/protocol"
)

// Probe checks that a model can answer a streamed function request and
// reports the exact model a route chose. A reply of plain text is not proof
// that a coding model can use Rashin's tools.
func (g *Gateway) Probe(ctx context.Context, p protocol.ModelProbeParams) (protocol.ModelProbeResult, error) {
	result := protocol.ModelProbeResult{}
	modelID := strings.TrimSpace(p.Model)
	if modelID == "" {
		modelID = protocol.RouteAuto
	}
	if !protocol.ValidModel(modelID) {
		result.Message = i18n.T("Choose a valid model or route")
		return result, nil
	}
	for option, value := range p.Options {
		switch option {
		case protocol.OptionEffort:
			if value == "" || value == llm.EffortOff || value == llm.EffortLow || value == llm.EffortMedium || value == llm.EffortHigh {
				continue
			}
		case protocol.OptionPlan:
			if value == protocol.OptionOn || value == protocol.OptionOff {
				continue
			}
		}
		result.Message = i18n.T("This model option is not supported")
		return result, nil
	}
	request := llm.Request{
		Model: modelID, Strict: pinnedRoute(modelID),
		Messages:   []llm.Message{{Role: llm.RoleUser, Content: "Call noop with an empty object. Do not answer in text."}},
		Tools:      []llm.Tool{{Name: "noop", Description: "No operation", Parameters: []byte(`{"type":"object","properties":{}}`)}},
		ToolChoice: llm.ToolChoiceRequired,
		// A one-token limit can end before the function name and arguments
		// arrive. The call stays tiny, but must be complete to prove tool use.
		MaxTokens: 96, Reasoning: p.Options[protocol.OptionEffort],
	}
	stream, err := g.Stream(ctx, request)
	if err != nil {
		result.Message = g.Redact(err.Error())
		return result, nil
	}
	defer stream.Close()
	called := false
	for {
		delta, nextErr := stream.Next()
		for _, call := range delta.ToolCalls {
			if call.Name == "noop" {
				called = true
			}
		}
		if errors.Is(nextErr, io.EOF) {
			break
		}
		if nextErr != nil {
			result.Message = g.Redact(nextErr.Error())
			return result, nil
		}
	}
	final := stream.Final()
	result.Model = protocol.Choice{ID: final.Model, Name: final.Model}
	if !called {
		result.Message = i18n.T("The model answered, but did not call the test tool")
		return result, nil
	}
	providerID, name, found := strings.Cut(final.Model, "/")
	if found {
		if provider, lookupErr := g.findProvider(ctx, providerID, false); lookupErr == nil {
			if model, ok := provider.findModel(name); ok {
				result.Model.Name = model.Name
				if effort := p.Options[protocol.OptionEffort]; effort != "" && !model.Reasoning {
					result.Message = i18n.T("This model does not offer reasoning effort")
					return result, nil
				}
				if model.Reasoning {
					result.Options = []protocol.SessionOption{{
						ID: protocol.OptionEffort, Name: i18n.T("Reasoning effort"),
						Current: p.Options[protocol.OptionEffort],
						Choices: []protocol.Choice{
							{ID: llm.EffortOff, Name: i18n.T("Off")},
							{ID: llm.EffortLow, Name: i18n.T("Low")},
							{ID: llm.EffortMedium, Name: i18n.T("Medium")},
							{ID: llm.EffortHigh, Name: i18n.T("High")},
						},
					}}
				}
			}
		}
	}
	result.OK = true
	result.Message = i18n.T("The model called the test tool")
	return result, nil
}
