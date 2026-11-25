package screen

import (
	"errors"
	"path/filepath"
	"strings"
)

// The ways a command line can fail to become an argv. Their text never
// repeats the line.
var (
	errEmpty    = errors.New("the command line names no program")
	errQuote    = errors.New("the command line leaves a quote open")
	errShell    = errors.New("the command line needs a shell (pipes, redirects, ;, &, $, backticks, subshells or variable assignments), and programs on the screen start without one: name one program and its arguments")
	errShellRun = errors.New("programs on the screen start without a shell, so a shell cannot be the program: name the program itself")
	errControl  = errors.New("the command line holds a control character")
)

// shells are programs that would read a command line of their own, which
// Open refuses to start, so no shell runs on a screen at all.
var shells = map[string]bool{
	"sh": true, "bash": true, "dash": true, "zsh": true, "fish": true, "ksh": true, "mksh": true,
	"csh": true, "tcsh": true, "ash": true, "busybox": true, "nu": true, "elvish": true, "xonsh": true,
	"env": true, "exec": true, "eval": true, "xargs": true, "nohup": true, "setsid": true,
}

// Argv splits a launcher-style command line into a program and its
// arguments without a shell: words split on spaces and tabs, single quotes
// keep everything literal, double quotes keep everything but \" and \\,
// a backslash outside quotes keeps the next character, and an unquoted ~ or
// ~/ starting a word is home. Anything only a shell would act on (an
// operator, a substitution, a variable assignment, a comment) is refused
// rather than passed through, and so is a shell as the program.
func Argv(command, home string) ([]string, error) {
	var (
		argv    []string
		word    strings.Builder
		inWord  bool
		quote   rune
		escaped bool
	)
	flush := func() {
		if inWord {
			argv = append(argv, word.String())
		}
		word.Reset()
		inWord = false
	}
	runes := []rune(command)
	for i, r := range runes {
		if r < 0x20 && r != '\t' || r == 0x7f {
			return nil, errControl
		}
		switch {
		case escaped:
			word.WriteRune(r)
			escaped = false
		case quote == '\'':
			if r == '\'' {
				quote = 0
			} else {
				word.WriteRune(r)
			}
		case quote == '"':
			switch {
			case r == '"':
				quote = 0
			case r == '$' || r == '`':
				return nil, errShell
			case r == '\\' && i+1 < len(runes) && (runes[i+1] == '"' || runes[i+1] == '\\'):
				escaped = true
			default:
				word.WriteRune(r)
			}
		case r == ' ' || r == '\t':
			flush()
		case r == '\'' || r == '"':
			quote, inWord = r, true
		case r == '\\':
			escaped, inWord = true, true
		case strings.ContainsRune("|&;<>()$`", r):
			return nil, errShell
		case r == '#' && !inWord:
			return nil, errShell
		case r == '~' && !inWord && (i+1 == len(runes) || runes[i+1] == '/' || runes[i+1] == ' ' || runes[i+1] == '\t'):
			word.WriteString(home)
			inWord = true
		default:
			word.WriteRune(r)
			inWord = true
		}
	}
	if quote != 0 || escaped {
		return nil, errQuote
	}
	flush()
	if len(argv) == 0 || argv[0] == "" {
		return nil, errEmpty
	}
	if name, _, ok := strings.Cut(argv[0], "="); ok && !strings.Contains(name, "/") {
		return nil, errShell
	}
	if shells[filepath.Base(argv[0])] {
		return nil, errShellRun
	}
	return argv, nil
}
