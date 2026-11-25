package command

import (
	"path/filepath"
	"strings"
)

func stripShellKeywords(argv []string) []string {
	for len(argv) > 0 {
		first := strings.TrimLeft(argv[0], "({!")
		if first == "" || first == "if" || first == "then" || first == "else" || first == "elif" || first == "do" || first == "while" || first == "until" {
			argv = argv[1:]
			continue
		}
		if first != argv[0] {
			argv[0] = first
		}
		break
	}
	return argv
}

func trimShellGroup(argv []string) []string {
	if len(argv) > 0 {
		last := len(argv) - 1
		argv[last] = strings.TrimRight(argv[last], ")}")
	}
	return argv
}

func nestedShell(argv []string) (string, bool) {
	if len(argv) == 0 {
		return "", false
	}
	switch filepath.Base(argv[0]) {
	case "eval":
		return strings.Join(argv[1:], " "), true
	case "script":
		for i, arg := range argv[1:] {
			if arg == "-c" || arg == "--command" || arg == "-qc" {
				if i+2 < len(argv) {
					return argv[i+2], true
				}
			}
		}
	default:
		if !shellNames[filepath.Base(argv[0])] {
			return "", false
		}
		for i, arg := range argv[1:] {
			if arg == "<<<" && i+2 < len(argv) {
				return argv[i+2], true
			}
			if arg == "-c" || arg == "--command" || strings.HasPrefix(arg, "-") && !strings.HasPrefix(arg, "--") && strings.Contains(arg[1:], "c") {
				if i+2 < len(argv) {
					return argv[i+2], true
				}
				return "", false
			}
		}
	}
	return "", false
}

func mutationPaths(argv []string, name string) []string {
	var paths []string
	operand := false
	for _, a := range argv[1:] {
		if a == "--" {
			operand = true
			continue
		}
		if !operand && strings.HasPrefix(a, "-") {
			continue
		}
		if !operand {
			operand = true
			if name == "chmod" || name == "chown" || name == "chgrp" {
				continue
			}
		}
		paths = append(paths, filepath.Clean(expandHomeRef(a)))
	}
	return paths
}

func findRoots(argv []string) []string {
	var roots []string
	for _, a := range argv[1:] {
		if strings.HasPrefix(a, "-") || a == "!" || a == "(" {
			break
		}
		roots = append(roots, a)
	}
	if len(roots) == 0 {
		return []string{"."}
	}
	return roots
}

func hasFindDelete(argv []string) bool {
	for _, a := range argv[1:] {
		if a == "-delete" {
			return true
		}
	}
	return false
}

func classifyFindRemoval(argv []string) (dangerTier, string) {
	return tierDanger, "find recursively deletes " + strings.Join(findRoots(argv), ", ")
}
