package ask

import (
	"context"

	"ryoku-rashin/internal/core"
	"ryoku-rashin/internal/engine/exec"
	"ryoku-rashin/internal/engine/tools/shell"
	"ryoku-rashin/internal/llm"
)

// Run gives a user-started command and its live terminal to the shared shell lane.
func (m *Mode) Run(ctx context.Context, x exec.Executor, in core.PromptInput, cwd string, sink core.Sink) (llm.Message, bool, error) {
	told, ok, err := shell.UserRun(ctx, x, sink, in.Turn, in.Run, cwd)
	if err != nil || !ok {
		return llm.Message{}, ok, err
	}
	return llm.Message{Role: llm.RoleUser, Content: told}, true, nil
}
