package exec

import (
	"encoding/json"
	"errors"

	"ryoku-rashin/internal/rpcpipe"
)

// Tool host methods: the sandbox executor calls them, `rashin __toolhost`
// (internal/toolhost) answers them with a host executor. Params and results
// are this package's request and result types.
const (
	MethodStat    = "fs.stat"    // PathParams → FileInfo
	MethodRead    = "fs.read"    // ReadRequest → ReadResult
	MethodWrite   = "fs.write"   // WriteRequest → WriteResult
	MethodEdit    = "fs.edit"    // EditRequest → EditResult
	MethodRemove  = "fs.remove"  // RemoveRequest → null
	MethodList    = "fs.list"    // ListRequest → ListResult
	MethodGlob    = "fs.glob"    // GlobRequest → GlobResult
	MethodGrep    = "fs.grep"    // GrepRequest → GrepResult
	MethodStart   = "exec.start" // StartRequest → StartResult
	MethodWait    = "exec.wait"  // ProcessParams → Exit
	MethodPeek    = "exec.peek"  // ProcessParams → Exit
	MethodKill    = "exec.kill"  // ProcessParams → null
	MethodRelease = "exec.release"

	// NotifyOutput carries one Output from the tool host.
	NotifyOutput = "exec.output"
	NotifyExit   = "exec.exit"
	// NotifyReady releases process output after the start result has reached
	// the client and its callback has been installed.
	NotifyReady = "exec.ready"
)

// WireErrorCode is the JSON-RPC error code whose data is an *Error.
const WireErrorCode = -32000

// PathParams names one path.
type PathParams struct {
	Path string `json:"path"`
}

// ProcessParams names one process.
type ProcessParams struct {
	ID string `json:"id"`
}

// StartResult is the started process's id.
type StartResult struct {
	ID string `json:"id"`
}

// WireError turns an executor failure into the JSON-RPC error the tool host
// answers with, keeping an *Error whole in its data.
func WireError(err error) error {
	if err == nil {
		return nil
	}
	var e *Error
	if errors.As(err, &e) {
		data, _ := json.Marshal(e)
		return &rpcpipe.Error{Code: WireErrorCode, Message: e.Error(), Data: data}
	}
	return &rpcpipe.Error{Code: rpcpipe.CodeInternal, Message: err.Error()}
}

// Alive reports whether e holds something running: the sandbox executor's
// tool host, or any process not yet released.
func Alive(e Executor) bool {
	if a, ok := e.(interface{ Alive() bool }); ok {
		return a.Alive()
	}
	return len(e.Running()) > 0
}
