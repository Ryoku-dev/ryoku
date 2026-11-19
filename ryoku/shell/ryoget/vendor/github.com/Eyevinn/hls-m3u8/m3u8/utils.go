package m3u8

/*
 This file defines some helper functions related to parsing strings.
*/

// isDigit checks if byte is ascii digit
func isDigit(c byte) bool {
	return c >= '0' && c <= '9'
}

// isKeyChar checks if byte would match regex [a-zA-Z0-9_-]
func isKeyChar(c byte) bool {
	return (c >= 'a' && c <= 'z') ||
		(c >= 'A' && c <= 'Z') ||
		(c >= '0' && c <= '9') ||
		c == '_' || c == '-'
}

// decodeAttributes decodes a line containing attributes.
// The values are left as verbatim strings, including quotes if present.
func decodeAttributes(line string) []Attribute {
	if line == "" {
		return []Attribute{}
	}

	attrs := make([]Attribute, 0, 10)

	// go through string as bytes
	i := 0
	n := len(line)

	for i < n {

		// key
		start := i
		for i < n && isKeyChar(line[i]) {
			i++
		}
		if i == start {
			i++ // skip bad char
			continue
		}
		key := string(line[start:i])

		if i >= n-1 || line[i] != '=' {
			if line[i] == '=' {
				attrs = append(attrs, Attribute{Key: key})
			}
			continue // malformed - next key
		}
		i++ // =

		// value
		start = i
		inQuote := line[i] == '"' // Need to include "," if in a quote
		for i < n && (line[i] != ',' || inQuote) {
			if i > start && line[i] == '"' {
				inQuote = false
			}
			i++
		}
		val := string(line[start:i])

		attrs = append(attrs, Attribute{Key: key, Val: val})
		i++ // skip commas
	}

	return attrs
}

// findEndsInNumber finds the last substring that is a number in s.
// if it ends in this number it will return endsInNumber true.
// returned number string includes leading zeroes.
func findEndsInNumber(s string) (numberString string, endsInNumber bool) {
	start := 0
	end := 0
	found := false
	lastIsNum := false
	for i := 0; i < len(s); i++ {
		isDigit := isDigit(s[i])
		if isDigit && !found {
			start = i
			found = true
		}
		if isDigit {
			end = i + 1
		}
		if !isDigit {
			found = false
		}
	}
	if found && end == len(s) {
		lastIsNum = true
	}
	return s[start:end], lastIsNum
}
