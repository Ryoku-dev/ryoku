package look

import "strings"

// drops directories from the middle so the file's name and the top of its tree both stay readable
func ElidePath(path string, limit int) string {
	if limit <= 0 || len([]rune(path)) <= limit {
		return path
	}
	parts := strings.Split(path, "/")
	if len(parts) < 3 {
		return path
	}
	head, tail := 1, len(parts)-1
	join := func() string {
		return strings.Join(parts[:head], "/") + "/…/" + strings.Join(parts[tail:], "/")
	}
	for tail-1 > head {
		tail--
		if len([]rune(join())) > limit {
			tail++
			break
		}
	}
	for head+1 < tail {
		head++
		if len([]rune(join())) > limit {
			head--
			break
		}
	}
	if head+1 >= tail && len([]rune(path)) <= limit {
		return path
	}
	return join()
}
