package look

import "strconv"

// Grouped writes a count with its thousands set apart, 412,300
func Grouped(value int) string {
	text := strconv.Itoa(value)
	for index := len(text) - 3; index > 0; index -= 3 {
		text = text[:index] + "," + text[index:]
	}
	return text
}
