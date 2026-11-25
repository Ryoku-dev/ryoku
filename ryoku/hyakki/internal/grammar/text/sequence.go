package text

import "unicode/utf8"

// EditDistanceMinAlloc computes Levenshtein distance using reusable row buffers.
// Source lengths above 255 exceed Harper's row-buffer range. Distances and
// target row indices wrap at 256, matching Harper's release-mode byte arithmetic.
func EditDistanceMinAlloc(source, target []rune, previousRow, currentRow *[]uint8) uint8 {
	if len(source) > 255 {
		panic("edit distance source exceeds 255 runes")
	}
	n := len(source) + 1
	resize := func(v []uint8) []uint8 {
		if cap(v) < n {
			return make([]uint8, n)
		}
		return v[:n]
	}
	prev := resize(*previousRow)
	curr := resize(*currentRow)
	for i := range prev {
		prev[i] = uint8(i)
	}
	for j := 1; j <= len(target); j++ {
		curr[0] = uint8(j)
		for i := 1; i < n; i++ {
			cost := uint8(0)
			if source[i-1] != target[j-1] {
				cost = 1
			}
			curr[i] = min(prev[i]+1, curr[i-1]+1, prev[i-1]+cost)
		}
		prev, curr = curr, prev
	}
	*previousRow = prev
	*currentRow = curr
	return prev[n-1]
}
func EditDistance(source, target []rune) uint8 {
	var prev, curr []uint8
	return EditDistanceMinAlloc(source, target, &prev, &curr)
}

// RemoveIndices removes sorted original indices in place and returns the shortened slice.
func RemoveIndices[T any](values []T, toRemove []int) []T {
	out, next := 0, 0
	for i, v := range values {
		if next < len(toRemove) && i == toRemove[next] {
			next++
			continue
		}
		values[out] = v
		out++
	}
	clear(values[out:])
	return values[:out]
}

// BuildByteToCharMap maps every byte, including interior UTF-8 bytes and the end, to its rune offset.
func BuildByteToCharMap(source string) []int {
	out := make([]int, len(source)+1)
	ci := 0
	for bi, c := range source {
		end := bi + utf8.RuneLen(c)
		if c == utf8.RuneError {
			_, size := utf8.DecodeRuneInString(source[bi:])
			end = bi + size
		}
		for i := bi; i < end; i++ {
			out[i] = ci
		}
		ci++
		out[end] = ci
	}
	return out
}
