package host

import "strings"

func normalizeRPMEVR(value string) string {
	if strings.HasPrefix(value, "(none):") {
		return "0:" + strings.TrimPrefix(value, "(none):")
	}
	return value
}

func compareRPMEVR(left, right string) int {
	left = normalizeRPMEVR(left)
	right = normalizeRPMEVR(right)
	leftEpoch, leftVersion, leftRelease := splitRPMEVR(left)
	rightEpoch, rightVersion, rightRelease := splitRPMEVR(right)
	if compared := compareRPMNumber(leftEpoch, rightEpoch); compared != 0 {
		return compared
	}
	if compared := rpmvercmp(leftVersion, rightVersion); compared != 0 {
		return compared
	}
	return rpmvercmp(leftRelease, rightRelease)
}

func splitRPMEVR(value string) (epoch, version, release string) {
	epoch = "0"
	if before, after, ok := strings.Cut(value, ":"); ok && isRPMNumber(before) {
		epoch, value = before, after
	}
	if before, after, ok := strings.Cut(value, "-"); ok {
		return epoch, before, after
	}
	return epoch, value, ""
}

func isRPMNumber(value string) bool {
	if value == "" {
		return false
	}
	for index := range len(value) {
		if !isRPMDigit(value[index]) {
			return false
		}
	}
	return true
}

func compareRPMNumber(left, right string) int {
	left = strings.TrimLeft(left, "0")
	right = strings.TrimLeft(right, "0")
	if len(left) < len(right) {
		return -1
	}
	if len(left) > len(right) {
		return 1
	}
	return strings.Compare(left, right)
}

func rpmvercmp(left, right string) int {
	leftIndex, rightIndex := 0, 0
	for leftIndex < len(left) || rightIndex < len(right) {
		for leftIndex < len(left) && !isRPMAlphaNumeric(left[leftIndex]) && left[leftIndex] != '~' && left[leftIndex] != '^' {
			leftIndex++
		}
		for rightIndex < len(right) && !isRPMAlphaNumeric(right[rightIndex]) && right[rightIndex] != '~' && right[rightIndex] != '^' {
			rightIndex++
		}

		leftTilde := leftIndex < len(left) && left[leftIndex] == '~'
		rightTilde := rightIndex < len(right) && right[rightIndex] == '~'
		if leftTilde || rightTilde {
			if !leftTilde {
				return 1
			}
			if !rightTilde {
				return -1
			}
			leftIndex++
			rightIndex++
			continue
		}

		leftCaret := leftIndex < len(left) && left[leftIndex] == '^'
		rightCaret := rightIndex < len(right) && right[rightIndex] == '^'
		if leftCaret || rightCaret {
			if leftIndex == len(left) {
				return -1
			}
			if rightIndex == len(right) {
				return 1
			}
			if !leftCaret {
				return 1
			}
			if !rightCaret {
				return -1
			}
			leftIndex++
			rightIndex++
			continue
		}

		if leftIndex == len(left) || rightIndex == len(right) {
			break
		}

		leftNumeric := isRPMDigit(left[leftIndex])
		rightNumeric := isRPMDigit(right[rightIndex])
		if leftNumeric != rightNumeric {
			if leftNumeric {
				return 1
			}
			return -1
		}

		leftEnd := leftIndex
		rightEnd := rightIndex
		if leftNumeric {
			for leftEnd < len(left) && isRPMDigit(left[leftEnd]) {
				leftEnd++
			}
			for rightEnd < len(right) && isRPMDigit(right[rightEnd]) {
				rightEnd++
			}
			if compared := compareRPMNumber(left[leftIndex:leftEnd], right[rightIndex:rightEnd]); compared != 0 {
				return compared
			}
		} else {
			for leftEnd < len(left) && isRPMAlpha(left[leftEnd]) {
				leftEnd++
			}
			for rightEnd < len(right) && isRPMAlpha(right[rightEnd]) {
				rightEnd++
			}
			if compared := strings.Compare(left[leftIndex:leftEnd], right[rightIndex:rightEnd]); compared != 0 {
				return compared
			}
		}
		leftIndex, rightIndex = leftEnd, rightEnd
	}
	if leftIndex == len(left) && rightIndex == len(right) {
		return 0
	}
	if leftIndex == len(left) {
		return -1
	}
	return 1
}

func isRPMDigit(value byte) bool {
	return value >= '0' && value <= '9'
}

func isRPMAlpha(value byte) bool {
	return value >= 'a' && value <= 'z' || value >= 'A' && value <= 'Z'
}

func isRPMAlphaNumeric(value byte) bool {
	return isRPMAlpha(value) || isRPMDigit(value)
}
