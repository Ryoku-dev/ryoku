package extract

import (
	"sort"
	"strings"
	"unicode/utf8"

	"ryoku-rashin/internal/redact"
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

// RedactChunks removes secrets from a file's chunks in order, so a private
// key block that opens in one chunk is still taken out of the next.
func RedactChunks(chunks []Chunk) []Chunk {
	out := append([]Chunk(nil), chunks...)
	inPrivateKey := false
	for index := range out {
		out[index].Text = redact.Patterns(out[index].Text, &inPrivateKey)
	}
	return out
}
