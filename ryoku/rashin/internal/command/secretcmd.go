package command

import (
	"path/filepath"
	"strings"
)

// ReadsSecretAt checks lexical and nested paths against the session directory.
func ReadsSecretAt(cmd, cwd string) (string, bool) {
	return readsSecretDepth(cmd, cwd, 0)
}

func readsSecretDepth(cmd, cwd string, depth int) (string, bool) {
	if depth >= 8 {
		return "uninspectable nested shell input", true
	}
	cmd, reason := shellSource(cmd)
	if reason != "" {
		return reason, true
	}
	segs := splitSegments(cmd)
	for index, seg := range segs {
		fields := shellFields(seg.text)
		argv := stripShellKeywords(stripEnvAssignments(fields))
		if len(argv) == 0 {
			continue
		}
		name := filepath.Base(argv[0])
		if name == "env" && len(unwrap(argv)) == 0 || name == "printenv" || name == "set" && len(argv) == 1 ||
			name == "export" && len(argv) > 1 && argv[1] == "-p" ||
			name == "declare" && len(argv) > 1 && argv[1] == "-p" {
			return "the process environment", true
		}
		if name == "cd" && len(argv) > 1 {
			dir := expandHomeRef(argv[1])
			if !filepath.IsAbs(dir) {
				dir = filepath.Join(cwd, dir)
			}
			cwd = filepath.Clean(dir)
		}
		if index > 0 && seg.pipedInto && shellNames[name] {
			prev := shellFields(segs[index-1].text)
			if len(prev) > 0 && (prev[0] == "echo" || prev[0] == "printf") {
				if marker, bad := readsSecretDepth(strings.Join(prev[1:], " "), cwd, depth+1); bad {
					return marker, true
				}
			}
		}
		if roRecursiveSecretSearchAt(argv, cwd) {
			return "a folder that holds credentials", true
		}
		if name == "find" || name == "fd" {
			if tail := execTail(argv); tail != nil {
				if marker, bad := readsSecretDepth(strings.Join(tail, " "), cwd, depth+1); bad {
					return marker, true
				}
			}
		}
		if nested, ok := nestedShell(argv); ok {
			if marker, bad := readsSecretDepth(nested, cwd, depth+1); bad {
				return marker, true
			}
		}
		for i, field := range fields {
			word := strings.TrimLeft(field, "0123456789<>&")
			if flag, value, found := strings.Cut(word, "="); found && strings.HasPrefix(flag, "-") {
				word = value
			}
			if name == "git" && strings.HasPrefix(word, "HEAD:") {
				word = strings.TrimPrefix(word, "HEAD:")
			}
			directFile := i > 0 && (name == "cat" || name == "head" || name == "tail" ||
				name == "less" || name == "bat" || name == "base64" || name == "sort" || name == "jq")
			if !pathLike(word) && !directFile {
				continue
			}
			if marker, secret := SecretPathAt(word, cwd); secret {
				return marker, true
			}
		}
	}
	return "", false
}
