package git

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"strings"

	i18n "ryoku-i18n"
)

type commandError struct {
	args   []string
	stderr string
	err    error
}

func (e *commandError) Error() string {
	message := strings.TrimSpace(e.stderr)
	if message == "" {
		message = e.err.Error()
	}
	return i18n.Tf("Git %s: %s", strings.Join(e.args, " "), message)
}

func (e *commandError) Unwrap() error { return e.err }

func run(ctx context.Context, dir string, args ...string) ([]byte, error) {
	return runEnv(ctx, dir, nil, args...)
}

func runEnv(ctx context.Context, dir string, extra []string, args ...string) ([]byte, error) {
	cmdArgs := append([]string{"-C", dir}, args...)
	cmd := exec.CommandContext(ctx, "git", cmdArgs...)
	cmd.Env = gitEnvironment(readOnlyCommand(args), extra)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return nil, ctxErr
		}
		return nil, &commandError{args: args, stderr: stderr.String(), err: err}
	}
	return stdout.Bytes(), nil
}

func runCombined(ctx context.Context, dir string, args ...string) ([]byte, error) {
	return runCapture(ctx, dir, false, args...)
}

func runCapture(ctx context.Context, dir string, readOnly bool, args ...string) ([]byte, error) {
	cmdArgs := append([]string{"-C", dir}, args...)
	cmd := exec.CommandContext(ctx, "git", cmdArgs...)
	cmd.Env = gitEnvironment(readOnly, nil)
	out, err := cmd.CombinedOutput()
	if err != nil {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return out, ctxErr
		}
		return out, &commandError{args: args, stderr: string(out), err: err}
	}
	return out, nil
}

var gitLocalEnvironment = map[string]bool{
	"GIT_ALTERNATE_OBJECT_DIRECTORIES": true,
	"GIT_CONFIG":                       true,
	"GIT_CONFIG_PARAMETERS":            true,
	"GIT_CONFIG_COUNT":                 true,
	"GIT_OBJECT_DIRECTORY":             true,
	"GIT_DIR":                          true,
	"GIT_WORK_TREE":                    true,
	"GIT_IMPLICIT_WORK_TREE":           true,
	"GIT_GRAFT_FILE":                   true,
	"GIT_INDEX_FILE":                   true,
	"GIT_NO_REPLACE_OBJECTS":           true,
	"GIT_REPLACE_REF_BASE":             true,
	"GIT_PREFIX":                       true,
	"GIT_SHALLOW_FILE":                 true,
	"GIT_COMMON_DIR":                   true,
}

func gitEnvironment(readOnly bool, extra []string) []string {
	managed := map[string]bool{
		"LC_ALL": true, "GIT_TERMINAL_PROMPT": true, "GIT_LITERAL_PATHSPECS": true,
		"GIT_OPTIONAL_LOCKS": true, "GIT_EDITOR": true, "GIT_SEQUENCE_EDITOR": true,
	}
	for _, value := range extra {
		if name, _, found := strings.Cut(value, "="); found {
			managed[name] = true
		}
	}
	env := make([]string, 0, len(os.Environ())+len(extra)+5)
	for _, value := range os.Environ() {
		name, _, found := strings.Cut(value, "=")
		if !found || managed[name] || unsafeGitEnvironment(name) {
			continue
		}
		env = append(env, value)
	}
	env = append(env, "LC_ALL=C", "GIT_TERMINAL_PROMPT=0", "GIT_LITERAL_PATHSPECS=1")
	if readOnly {
		env = append(env, "GIT_OPTIONAL_LOCKS=0")
	} else {
		env = append(env, "GIT_EDITOR=true", "GIT_SEQUENCE_EDITOR=true")
	}
	return append(env, extra...)
}

func unsafeGitEnvironment(name string) bool {
	return gitLocalEnvironment[name] ||
		strings.HasPrefix(name, "GIT_CONFIG_KEY_") ||
		strings.HasPrefix(name, "GIT_CONFIG_VALUE_")
}

func readOnlyCommand(args []string) bool {
	if len(args) == 0 {
		return true
	}
	for _, arg := range args {
		if strings.HasPrefix(arg, "-") || arg == "-c" {
			continue
		}
		switch arg {
		case "rev-parse", "status", "diff", "diff-tree", "show", "log", "for-each-ref", "merge-base", "rev-list", "ls-files", "ls-tree", "cat-file", "check-ref-format":
			return true
		case "config":
			return contains(args, "--get") || contains(args, "--get-all")
		case "remote":
			return len(args) == 1
		case "symbolic-ref":
			return !contains(args, "--delete")
		case "worktree":
			return len(args) > 1 && (args[1] == "list" || args[1] == "prune" && (contains(args, "-n") || contains(args, "--dry-run")))
		default:
			return false
		}
	}
	return true
}

func contains(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

func trimOutput(out []byte) string { return strings.TrimSpace(string(out)) }
