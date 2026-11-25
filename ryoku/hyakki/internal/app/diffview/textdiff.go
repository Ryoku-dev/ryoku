package diffview

import (
	"fmt"
	"strings"

	"ryoku-rashin/internal/protocol"
)

type editKind byte

const (
	equal editKind = iota
	insert
	remove
)

type edit struct {
	kind editKind
	text string
}

func fromTexts(path, oldText, newText string) protocol.FileDiff {
	oldLines := textLines(oldText)
	newLines := textLines(newText)
	edits := myers(oldLines, newLines)
	lines := make([]protocol.DiffLine, 0, len(edits))
	oldLine, newLine, added, removed := 1, 1, 0, 0
	for _, change := range edits {
		switch change.kind {
		case equal:
			lines = append(lines, protocol.DiffLine{Kind: protocol.LineContext, Text: change.text, Old: oldLine, New: newLine})
			oldLine++
			newLine++
		case remove:
			lines = append(lines, protocol.DiffLine{Kind: protocol.LineDel, Text: change.text, Old: oldLine})
			oldLine++
			removed++
		case insert:
			lines = append(lines, protocol.DiffLine{Kind: protocol.LineAdd, Text: change.text, New: newLine})
			newLine++
			added++
		}
	}
	status := protocol.ChangeModified
	if len(oldLines) == 0 && len(newLines) > 0 {
		status = protocol.ChangeAdded
	} else if len(newLines) == 0 && len(oldLines) > 0 {
		status = protocol.ChangeDeleted
	}
	file := protocol.FileDiff{Path: path, Status: status, Added: added, Removed: removed}
	if len(lines) != 0 {
		file.Hunks = []protocol.Hunk{{
			Header:   fmt.Sprintf("@@ -%d,%d +%d,%d @@", hunkStart(len(oldLines)), len(oldLines), hunkStart(len(newLines)), len(newLines)),
			OldStart: hunkStart(len(oldLines)), OldLines: len(oldLines),
			NewStart: hunkStart(len(newLines)), NewLines: len(newLines), Lines: lines,
		}}
	}
	return file
}

func hunkStart(lines int) int {
	if lines == 0 {
		return 0
	}
	return 1
}

func textLines(text string) []string {
	if text == "" {
		return nil
	}
	lines := strings.Split(text, "\n")
	if len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	return lines
}

// myers returns one shortest edit script. The trace stores only diagonal
// frontiers, so memory grows with the edit distance rather than an N*M table.
func myers(a, b []string) []edit {
	n, m := len(a), len(b)
	if n == 0 {
		result := make([]edit, len(b))
		for i, line := range b {
			result[i] = edit{kind: insert, text: line}
		}
		return result
	}
	if m == 0 {
		result := make([]edit, len(a))
		for i, line := range a {
			result[i] = edit{kind: remove, text: line}
		}
		return result
	}
	maximum := n + m
	offset := maximum
	frontier := make([]int, 2*maximum+1)
	trace := make([][]int, 0, maximum+1)
	for distance := 0; distance <= maximum; distance++ {
		trace = append(trace, append([]int(nil), frontier...))
		for diagonal := -distance; diagonal <= distance; diagonal += 2 {
			index := offset + diagonal
			var x int
			if diagonal == -distance || (diagonal != distance && frontier[index-1] < frontier[index+1]) {
				x = frontier[index+1]
			} else {
				x = frontier[index-1] + 1
			}
			y := x - diagonal
			for x < n && y < m && a[x] == b[y] {
				x++
				y++
			}
			frontier[index] = x
			if x >= n && y >= m {
				return backtrack(a, b, trace, distance, offset)
			}
		}
	}
	return nil
}

func backtrack(a, b []string, trace [][]int, distance, offset int) []edit {
	x, y := len(a), len(b)
	reversed := make([]edit, 0, x+y)
	for d := distance; d > 0; d-- {
		frontier := trace[d]
		diagonal := x - y
		var previousDiagonal int
		if diagonal == -d || (diagonal != d && frontier[offset+diagonal-1] < frontier[offset+diagonal+1]) {
			previousDiagonal = diagonal + 1
		} else {
			previousDiagonal = diagonal - 1
		}
		previousX := frontier[offset+previousDiagonal]
		previousY := previousX - previousDiagonal
		for x > previousX && y > previousY {
			reversed = append(reversed, edit{kind: equal, text: a[x-1]})
			x--
			y--
		}
		if x == previousX {
			reversed = append(reversed, edit{kind: insert, text: b[y-1]})
			y--
		} else {
			reversed = append(reversed, edit{kind: remove, text: a[x-1]})
			x--
		}
	}
	for x > 0 && y > 0 {
		reversed = append(reversed, edit{kind: equal, text: a[x-1]})
		x--
		y--
	}
	for x > 0 {
		reversed = append(reversed, edit{kind: remove, text: a[x-1]})
		x--
	}
	for y > 0 {
		reversed = append(reversed, edit{kind: insert, text: b[y-1]})
		y--
	}
	for left, right := 0, len(reversed)-1; left < right; left, right = left+1, right-1 {
		reversed[left], reversed[right] = reversed[right], reversed[left]
	}
	return reversed
}
