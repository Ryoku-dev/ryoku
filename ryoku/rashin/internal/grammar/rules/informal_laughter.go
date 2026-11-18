package rules

func informalLaughterIsInformalLaughter(chars []rune) bool {
	if len(chars) < 2 {
		return false
	}
	for i, c := range chars {
		if i%2 == 0 {
			if c != 'h' && c != 'H' {
				return false
			}
		} else if c != 'a' && c != 'A' {
			return false
		}
	}
	return true
}
