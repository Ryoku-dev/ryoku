package rules

import (
	"ryoku-rashin/internal/grammar/linttest"
	"testing"
)

func TestExpandMemoryShorthands_DetectsBytes(t *testing.T) {
	linttest.AssertSuggestionResult(t, "5 B", NewExpandMemoryShorthands(), "5 bytes")
}

func TestExpandMemoryShorthands_DetectsKilobytes(t *testing.T) {
	linttest.AssertSuggestionResult(t, "10 kB", NewExpandMemoryShorthands(), "10 kilobytes")
}

func TestExpandMemoryShorthands_DetectsMegabytes(t *testing.T) {
	linttest.AssertSuggestionResult(t, "30 MB", NewExpandMemoryShorthands(), "30 megabytes")
}

func TestExpandMemoryShorthands_DetectsGigabytes(t *testing.T) {
	linttest.AssertSuggestionResult(t, "16 GB", NewExpandMemoryShorthands(), "16 gigabytes")
}

func TestExpandMemoryShorthands_DetectsTerabytes(t *testing.T) {
	linttest.AssertSuggestionResult(t, "2 TB", NewExpandMemoryShorthands(), "2 terabytes")
}

func TestExpandMemoryShorthands_DetectsKibibytes(t *testing.T) {
	linttest.AssertSuggestionResult(t, "1024 KiB", NewExpandMemoryShorthands(), "1024 kibibytes")
}

func TestExpandMemoryShorthands_DetectsMebibytes(t *testing.T) {
	linttest.AssertSuggestionResult(t, "2048 MiB", NewExpandMemoryShorthands(), "2048 mebibytes")
}

func TestExpandMemoryShorthands_DetectsGibibytes(t *testing.T) {
	linttest.AssertSuggestionResult(t, "4 GiB", NewExpandMemoryShorthands(), "4 gibibytes")
}

func TestExpandMemoryShorthands_DetectsTebibytes(t *testing.T) {
	linttest.AssertSuggestionResult(t, "8 TiB", NewExpandMemoryShorthands(), "8 tebibytes")
}

func TestExpandMemoryShorthands_DetectsPetabytes(t *testing.T) {
	linttest.AssertSuggestionResult(t, "1 PB", NewExpandMemoryShorthands(), "1 petabyte")
}

func TestExpandMemoryShorthands_DetectsExabytes(t *testing.T) {
	linttest.AssertSuggestionResult(t, "1 EB", NewExpandMemoryShorthands(), "1 exabyte")
}

func TestExpandMemoryShorthands_DetectsZettabytes(t *testing.T) {
	linttest.AssertSuggestionResult(t, "1 ZB", NewExpandMemoryShorthands(), "1 zettabyte")
}

func TestExpandMemoryShorthands_DetectsYottabytes(t *testing.T) {
	linttest.AssertSuggestionResult(t, "1 YB", NewExpandMemoryShorthands(), "1 yottabyte")
}

func TestExpandMemoryShorthands_DetectsQuettabytes(t *testing.T) {
	linttest.AssertSuggestionResult(t, "1 QB", NewExpandMemoryShorthands(), "1 quettabyte")
}

func TestExpandMemoryShorthands_DetectsPebibytes(t *testing.T) {
	linttest.AssertSuggestionResult(t, "1 PiB", NewExpandMemoryShorthands(), "1 pebibyte")
}

func TestExpandMemoryShorthands_DetectsExbibytes(t *testing.T) {
	linttest.AssertSuggestionResult(t, "1 EiB", NewExpandMemoryShorthands(), "1 exbibyte")
}

func TestExpandMemoryShorthands_DetectsZebibytes(t *testing.T) {
	linttest.AssertSuggestionResult(t, "1 ZiB", NewExpandMemoryShorthands(), "1 zebibyte")
}

func TestExpandMemoryShorthands_DetectsYobibytes(t *testing.T) {
	linttest.AssertSuggestionResult(t, "1 YiB", NewExpandMemoryShorthands(), "1 yobibyte")
}

func TestExpandMemoryShorthands_DetectsRobibytes(t *testing.T) {
	linttest.AssertSuggestionResult(t, "1 RiB", NewExpandMemoryShorthands(), "1 robibyte")
}

func TestExpandMemoryShorthands_DetectsQuebibytes(t *testing.T) {
	linttest.AssertSuggestionResult(t, "1 QiB", NewExpandMemoryShorthands(), "1 quebibyte")
}

func TestExpandMemoryShorthands_HandlesPunctuation(t *testing.T) {
	linttest.AssertSuggestionResult(t, "8 GB.", NewExpandMemoryShorthands(), "8 gigabytes.")
}

func TestExpandMemoryShorthands_HandlesAdjacentNumber(t *testing.T) {
	linttest.AssertSuggestionResult(t, "16GB", NewExpandMemoryShorthands(), "16 gigabytes")
}

func TestExpandMemoryShorthands_HandlesHyphenSeparated(t *testing.T) {
	linttest.AssertSuggestionResult(t, "32-GB", NewExpandMemoryShorthands(), "32-gigabytes")
}

func TestExpandMemoryShorthands_DoesntHandleWrongKbCases(t *testing.T) {
	linttest.AssertLintCount(t, "48kb and 64 KB were common in the 8-bit era.", NewExpandMemoryShorthands(), 0)
}

func TestExpandMemoryShorthands_DontFlagIsABPlus3715(t *testing.T) {
	linttest.AssertNoLints(t, "Is a B+ a 3.3 GPA?", NewExpandMemoryShorthands())
}
