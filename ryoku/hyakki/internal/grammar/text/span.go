package text

import (
	"fmt"
	"iter"
)

// Span is an end-exclusive window in a rune or token sequence.
type Span struct {
	Start int `json:"start"`
	End   int `json:"end"`
}

var Zero = Span{}

func NewSpan(start, end int) Span {
	if start > end {
		panic(fmt.Sprintf("%d > %d", start, end))
	}
	return Span{start, end}
}
func NewSpanWithLen(start, length int) Span { return Span{start, start + length} }
func EmptySpan(pos int) Span                { return Span{pos, pos} }
func (s Span) Len() int                     { return s.End - s.Start }
func (s Span) IsEmpty() bool                { return s.Len() == 0 }
func (s Span) Contains(idx int) bool        { return s.Start <= idx && idx < s.End }
func (s Span) OverlapsWith(other Span) bool { return s.Start < other.End && other.Start < s.End }

// TryGetContent returns an empty slice even when an empty span's position is outside source.
func TryGetContent[T any](s Span, source []T) ([]T, bool) {
	if s.IsEmpty() {
		return source[:0], true
	}
	if s.Start < 0 || s.End < s.Start || s.End > len(source) {
		return nil, false
	}
	return source[s.Start:s.End], true
}
func GetContent[T any](s Span, source []T) []T {
	v, ok := TryGetContent(s, source)
	if !ok {
		panic("Failed to get content for span.")
	}
	return v
}
func (s Span) TryGetContent(source []rune) ([]rune, bool) { return TryGetContent(s, source) }
func (s Span) GetContent(source []rune) []rune            { return GetContent(s, source) }
func (s Span) GetContentString(source []rune) string      { return string(s.GetContent(source)) }
func (s *Span) ExpandToInclude(target int) {
	if target < s.Start {
		s.Start = target
	} else if target >= s.End {
		s.End = target + 1
	}
}
func (s *Span) SetLen(length int)      { s.End = s.Start + length }
func (s Span) WithLen(length int) Span { s.SetLen(length); return s }
func (s *Span) PushBy(by int)          { s.Start += by; s.End += by }
func (s *Span) PullBy(by int) {
	if by > s.Start {
		panic("span underflow")
	}
	s.Start -= by
	s.End -= by
}
func (s Span) PushedBy(by int) Span { s.PushBy(by); return s }
func (s Span) PulledBy(by int) (Span, bool) {
	if by > s.Start {
		return Span{}, false
	}
	s.PullBy(by)
	return s, true
}

// ToCharSpan converts a token range using the source tokens' character spans.
func (s Span) ToCharSpan(sourceTokenSpans []Span) Span {
	if s.IsEmpty() {
		return Zero
	}
	v := GetContent(s, sourceTokenSpans)
	return NewSpan(v[0].Start, v[len(v)-1].End)
}
func (s Span) Indices() iter.Seq[int] {
	return func(yield func(int) bool) {
		for i := s.Start; i < s.End; i++ {
			if !yield(i) {
				return
			}
		}
	}
}
