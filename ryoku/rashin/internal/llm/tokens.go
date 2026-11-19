package llm

// Token estimates for what a provider has not counted yet. They follow omp's
// output budget (oh-my-pi packages/agent/src/output-budget.ts@c7fff67): text
// is a token per four bytes, each message adds its framing, and a sum gets a
// tenth more as margin, so prompt plus max_tokens stays inside the window
// llama.cpp enforces.

// MessageFramingTokens is what one message costs beyond its content: role and
// separators in the chat template.
const MessageFramingTokens = 16

// ImageTokens is what one image is counted as, whatever its size: on the
// high side of what vision encoders spend per picture, so a screenshot turn
// never overflows.
const ImageTokens = 1024

// EstimateText is s at a token per four bytes, rounded up.
func EstimateText(s string) int {
	return (len(s) + 3) / 4
}

// EstimateMessage is one message without margin: its text, reasoning, tool
// calls and images, plus MessageFramingTokens.
func EstimateMessage(m Message) int {
	n := MessageFramingTokens + EstimateText(m.Content) + EstimateText(m.ToolCallID) + EstimateText(m.Name)
	for _, call := range m.ToolCalls {
		n += EstimateText(call.ID) + EstimateText(call.Name) + EstimateText(call.Arguments)
	}
	for _, block := range m.Thinking {
		n += EstimateText(block.Text) + EstimateText(block.Signature) + (len(block.Redacted)+3)/4
	}
	return n + len(m.Images)*ImageTokens
}

// EstimateTool is one tool schema as the request sends it, plus
// MessageFramingTokens.
func EstimateTool(t Tool) int {
	return MessageFramingTokens + EstimateText(t.Name) + EstimateText(t.Description) + (len(t.Parameters)+3)/4
}

// EstimateMessages is messages with the margin.
func EstimateMessages(messages []Message) int {
	n := 0
	for _, m := range messages {
		n += EstimateMessage(m)
	}
	return withMargin(n)
}

// EstimateRequest is the prompt of req with the margin: every message and
// every tool schema.
func EstimateRequest(req Request) int {
	n := 0
	for _, m := range req.Messages {
		n += EstimateMessage(m)
	}
	for _, t := range req.Tools {
		n += EstimateTool(t)
	}
	return withMargin(n)
}

func withMargin(n int) int {
	return n + n/10
}
