package term

import "strings"

// SpanKind selects sentence ink without turning the whole sentence into data.
type SpanKind uint8

const (
	SpanText SpanKind = iota
	SpanLiteral
	SpanAttention
)

// Span is one styled part of a sentence.
type Span struct {
	Text string
	Kind SpanKind
}

// Sentence describes composable sentence spans.
type Sentence struct {
	Spans []Span
}

// RenderSentence renders bold sentence text with literal and attention spans.
func RenderSentence(th Theme, sentence Sentence) string {
	parts := make([]string, len(sentence.Spans))
	for i, span := range sentence.Spans {
		switch span.Kind {
		case SpanAttention:
			parts[i] = bold(th.Roles.Amber).Render(span.Text)
		case SpanLiteral:
			parts[i] = ink(th.Roles.Fg).Render(span.Text)
		default:
			parts[i] = bold(th.Roles.Fg).Render(span.Text)
		}
	}
	return strings.Join(parts, "")
}
