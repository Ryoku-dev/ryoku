package extract

import (
	"regexp"
	"sort"
	"strings"
	"unicode/utf8"
)

const (
	ChunkLines          = 40
	MaxWholeSymbolLines = 80
	MaxChunkBytes       = 8 << 10
)

func Chunks(source string, symbols []Symbol) []Chunk {
	lines := strings.Split(source, "\n")
	ordered := append([]Symbol(nil), symbols...)
	sort.SliceStable(ordered, func(i, j int) bool {
		if ordered[i].Start != ordered[j].Start {
			return ordered[i].Start < ordered[j].Start
		}
		return ordered[i].End > ordered[j].End
	})
	var chunks []Chunk
	cursor := 1
	appendRange := func(start, end int, symbol string) {
		if start < 1 {
			start = 1
		}
		if end > len(lines) {
			end = len(lines)
		}
		if end < start {
			return
		}
		chunkStart := start
		var current []string
		currentBytes := 0
		flush := func(chunkEnd int) {
			text := strings.Join(current, "\n")
			if strings.TrimSpace(text) != "" {
				chunks = append(chunks, Chunk{Start: chunkStart, End: chunkEnd, Text: text, Symbol: symbol})
			}
			current = nil
			currentBytes = 0
		}
		for lineNumber := start; lineNumber <= end; lineNumber++ {
			line := lines[lineNumber-1]
			for len(line) > MaxChunkBytes {
				if len(current) > 0 {
					flush(lineNumber - 1)
					chunkStart = lineNumber
				}
				cut := MaxChunkBytes
				for cut > 0 && !utf8.RuneStart(line[cut]) {
					cut--
				}
				if cut == 0 {
					cut = MaxChunkBytes
				}
				part := line[:cut]
				if strings.TrimSpace(part) != "" {
					chunks = append(chunks, Chunk{Start: lineNumber, End: lineNumber, Text: part, Symbol: symbol})
				}
				line = line[cut:]
				chunkStart = lineNumber
			}
			added := len(line)
			if len(current) > 0 {
				added++
			}
			if currentBytes+added > MaxChunkBytes && len(current) > 0 {
				flush(lineNumber - 1)
				chunkStart = lineNumber
			}
			current = append(current, line)
			currentBytes += len(line)
			if len(current) > 1 {
				currentBytes++
			}
		}
		if len(current) > 0 {
			flush(end)
		}
	}
	window := func(start, end int) {
		for start <= end {
			stop := min(end, start+ChunkLines-1)
			appendRange(start, stop, "")
			start = stop + 1
		}
	}
	for _, symbol := range ordered {
		start, end := symbol.Start, symbol.End
		if start < cursor {
			continue
		}
		if start > len(lines) {
			break
		}
		window(cursor, start-1)
		if end < start {
			end = start
		}
		if end > len(lines) {
			end = len(lines)
		}
		if end-start+1 <= MaxWholeSymbolLines {
			appendRange(start, end, symbol.Name)
		} else {
			for at := start; at <= end; at += ChunkLines {
				appendRange(at, min(end, at+ChunkLines-1), symbol.Name)
			}
		}
		cursor = end + 1
	}
	window(cursor, len(lines))
	return chunks
}

var (
	secretName        = `[A-Za-z0-9_.-]*(?:password|passwd|pwd|secret|token|api[-_]?key|apikey|access[-_]?key|auth|credential|private[-_]?key)[A-Za-z0-9_.-]*`
	quotedSecretValue = `(?:"(?:\\.|[^"\\])*"|'(?:\\.|[^'\\])*')`
	secretAssignment  = regexp.MustCompile(`(?i)(^|[\s{,;])(["']?` + secretName + `["']?\s*(?::|=)\s*)(` + quotedSecretValue + `|[^\s,;}]+)`)
	secretWhitespace  = regexp.MustCompile(`(?i)^(\s*(?:export\s+)?["']?` + secretName + `["']?\s+)(` + quotedSecretValue + `|\S+)`)
	fishSecret        = regexp.MustCompile(`(?i)^(\s*set\s+(?:-[A-Za-z]+\s+)*["']?` + secretName + `["']?\s+)(` + quotedSecretValue + `|\S+)`)
	urlCredential     = regexp.MustCompile(`(?i)([A-Za-z][A-Za-z0-9+.-]*://[^\s/:@]+:)[^\s@/]+(@)`)
	bearerToken       = regexp.MustCompile(`(?i)\bBearer\s+\S+`)
	jwtToken          = regexp.MustCompile(`\beyJ[A-Za-z0-9_-]*\.[A-Za-z0-9_-]+\.[A-Za-z0-9_-]+\b`)
	secretToken       = regexp.MustCompile(`(?i)\b(?:sk-[A-Za-z0-9_-]{8,}|gsk_[A-Za-z0-9_-]{8,}|hf_[A-Za-z0-9_-]{8,}|github_pat_[A-Za-z0-9_]{8,}|gh[oprsu]_[A-Za-z0-9]{8,}|glpat-[A-Za-z0-9_-]{8,}|[sr]k_live_[A-Za-z0-9_-]{8,}|AIza[A-Za-z0-9_-]{8,}|xapp-[A-Za-z0-9-]{8,}|npm_[A-Za-z0-9_-]{8,}|AKIA[A-Z0-9]{12,}|xox[a-z]-[A-Za-z0-9-]{8,})`)
	privateKeyBegin   = regexp.MustCompile(`(?i)-----BEGIN [A-Z0-9 ]*PRIVATE KEY(?: BLOCK)?-----`)
	privateKeyEnd     = regexp.MustCompile(`(?i)-----END [A-Z0-9 ]*PRIVATE KEY(?: BLOCK)?-----`)
)

func redactLine(line string) string {
	line = urlCredential.ReplaceAllString(line, "$1[REDACTED]$2")
	line = secretAssignment.ReplaceAllString(line, "$1$2[REDACTED]")
	line = fishSecret.ReplaceAllString(line, "$1[REDACTED]")
	line = secretWhitespace.ReplaceAllString(line, "$1[REDACTED]")
	line = bearerToken.ReplaceAllString(line, "Bearer [REDACTED]")
	line = jwtToken.ReplaceAllString(line, "[REDACTED]")
	return secretToken.ReplaceAllString(line, "[REDACTED]")
}

func redactText(text string, inPrivateKey *bool) string {
	lines := strings.Split(text, "\n")
	for index, line := range lines {
		if privateKeyBegin.MatchString(line) {
			*inPrivateKey = true
		}
		if *inPrivateKey {
			lines[index] = "[REDACTED PRIVATE KEY]"
			if privateKeyEnd.MatchString(line) {
				*inPrivateKey = false
			}
			continue
		}
		lines[index] = redactLine(line)
	}
	return strings.Join(lines, "\n")
}

// RedactSource removes secrets without changing the source's line count.
func RedactSource(text string) string {
	inPrivateKey := false
	return redactText(text, &inPrivateKey)
}

func Redact(text string) string { return RedactSource(text) }

func RedactChunks(chunks []Chunk) []Chunk {
	out := append([]Chunk(nil), chunks...)
	inPrivateKey := false
	for index := range out {
		out[index].Text = redactText(out[index].Text, &inPrivateKey)
	}
	return out
}
