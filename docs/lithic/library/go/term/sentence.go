package term

import "strings"

// SpanKind selects sentence ink without turning the whole sentence into data.
type SpanKind uint8

const (
	SpanText SpanKind = iota
	SpanLiteral
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

// RenderSentence renders bold sentence text with literal spans in plain ink.
func RenderSentence(th Theme, sentence Sentence) string {
	parts := make([]string, len(sentence.Spans))
	for i, span := range sentence.Spans {
		if span.Kind == SpanLiteral {
			parts[i] = ink(th.Roles.Fg).Render(span.Text)
			continue
		}
		parts[i] = bold(th.Roles.Fg).Render(span.Text)
	}
	return strings.Join(parts, "")
}
