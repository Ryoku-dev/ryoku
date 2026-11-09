package term

import "unicode/utf8"

func editText(value, key string, limit int) (string, bool) {
	runes := []rune(value)
	switch key {
	case "backspace", "ctrl+h":
		if len(runes) == 0 {
			return value, false
		}
		return string(runes[:len(runes)-1]), true
	case "ctrl+u":
		return "", value != ""
	case "space":
		key = " "
	}
	if utf8.RuneCountInString(key) != 1 {
		return value, false
	}
	if limit > 0 && len(runes) >= limit {
		return value, false
	}
	return value + key, true
}
