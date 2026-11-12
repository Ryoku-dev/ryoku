package harness

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"ryoku-i18n"
	"ryoku-rashin/internal/paths"
	"ryoku-rashin/internal/protocol"
)

func (r *runner) handleRequest(message rpcMessage) {
	r.mu.Lock()
	boxed := r.boxed
	r.mu.Unlock()
	if boxed && (strings.HasPrefix(message.Method, "fs/") || strings.HasPrefix(message.Method, "terminal/")) {
		r.respondRequestError(message.ID, errors.New(i18n.T("Rashin does not read, write or run anything for an agent in a sandbox. Use your own tools.")))
		return
	}
	switch message.Method {
	case "session/request_permission":
		r.handlePermissionRequest(message)
	case "fs/read_text_file":
		r.handleReadFile(message)
	case "fs/write_text_file":
		r.handleWriteFile(message)
	case "terminal/create":
		r.handleTerminalCreate(message)
	case "terminal/output":
		r.handleTerminalOutput(message)
	case "terminal/wait_for_exit":
		r.handleTerminalWait(message)
	case "terminal/kill":
		r.handleTerminalKill(message)
	case "terminal/release":
		r.handleTerminalRelease(message)
	default:
		r.mu.Lock()
		conn := r.conn
		r.mu.Unlock()
		if conn != nil {
			_ = conn.respondError(message.ID, -32601, "method not found")
		}
	}
}

func (r *runner) handlePermissionRequest(message rpcMessage) {
	var request struct {
		ToolCall toolCall `json:"toolCall"`
		Options  []struct {
			ID   string `json:"optionId"`
			Name string `json:"name"`
			Kind string `json:"kind"`
		} `json:"options"`
	}
	if err := json.Unmarshal(message.Params, &request); err != nil {
		r.respondRequestError(message.ID, err)
		return
	}
	r.mu.Lock()
	requestID := r.generationIDLocked("permission", rpcID(message.ID))
	r.mu.Unlock()
	options := make([]protocol.PermissionOption, 0, len(request.Options))
	allow := ""
	for _, option := range request.Options {
		kind := normalizePermissionKind(option.Kind)
		options = append(options, protocol.PermissionOption{ID: option.ID, Label: option.Name, Kind: kind})
		if allow == "" && (kind == protocol.OptAllowOnce || kind == protocol.OptAllowAlways) {
			allow = option.ID
		}
	}
	data, diffs := displayTool(request.ToolCall)
	if data.Call == "" {
		data.Call = request.ToolCall.ToolCallID
	}
	r.mu.Lock()
	_, seen := r.tools[data.Call]
	r.mu.Unlock()
	if !seen && data.Call != "" {
		if data.Status == "" {
			data.Status = protocol.ToolPending
		}
		r.emit(protocol.EvTool, data)
		for _, diff := range diffs {
			r.emit(protocol.EvTool, protocol.ToolData{Call: data.Call, Diff: &diff})
		}
		r.mu.Lock()
		r.toolOrder++
		r.tools[data.Call] = trackedTool{kind: data.Kind, status: data.Status, order: r.toolOrder}
		r.mu.Unlock()
	}
	r.mu.Lock()
	cancelled, conn := r.turnCancelled, r.conn
	r.mu.Unlock()
	if cancelled {
		if conn != nil {
			_ = conn.respond(message.ID, cancelledPermission())
		}
		r.emit(protocol.EvPermissionResolved, protocol.PermissionResolvedData{
			Request: requestID, Outcome: "cancelled", By: "user",
		})
		return
	}
	var verdict askVerdict
	var reason string
	var danger bool
	if r.opts.Ask {
		title := ""
		if request.ToolCall.Title != nil {
			title = *request.ToolCall.Title
		}
		verdict, reason, danger = r.askDecides(data.Kind, title, request.ToolCall.RawInput)
		options = askOptions(options, danger)
	} else {
		verdict = askAsks
		if policy := r.approvalPolicy(); policy == protocol.ApprovalsAuto || policy == protocol.ApprovalsReadOnly && isReadOnlyKind(data.Kind) {
			verdict = askAllows
		}
	}
	// the policy answers for the user: a call it allows runs, one it refuses
	// is turned down and the user told why
	answer := allow
	if verdict == askRefuses {
		answer = rejectOption(options)
	}
	if verdict != askAsks && answer != "" {
		r.mu.Lock()
		conn := r.conn
		cancelled := r.turnCancelled
		r.mu.Unlock()
		if cancelled {
			if conn != nil {
				_ = conn.respond(message.ID, cancelledPermission())
			}
			r.emit(protocol.EvPermissionResolved, protocol.PermissionResolvedData{
				Request: requestID, Outcome: "cancelled", By: "user",
			})
			return
		}
		if conn != nil {
			_ = conn.respond(message.ID, selectedPermission(answer))
		}
		if verdict == askRefuses {
			r.askRefused(requestID, data.Call, data.Kind, data.Title, reason)
			return
		}
		r.emit(protocol.EvPermissionResolved, protocol.PermissionResolvedData{
			Request: requestID, Option: answer, Outcome: "selected", By: "policy",
		})
		return
	}
	open := permissionRequest{id: append(json.RawMessage(nil), message.ID...), publicID: requestID, call: data.Call, options: options}
	if r.opts.Ask {
		id := open.id
		open.respond = func(conn *connection, option string) error {
			return conn.respond(id, selectedPermission(r.askAnswer(option, options)))
		}
	}
	r.mu.Lock()
	if r.turnCancelled {
		conn := r.conn
		r.mu.Unlock()
		if conn != nil {
			_ = conn.respond(message.ID, cancelledPermission())
		}
		r.emit(protocol.EvPermissionResolved, protocol.PermissionResolvedData{
			Request: requestID, Outcome: "cancelled", By: "user",
		})
		return
	}
	r.permissions[requestID] = open
	r.mu.Unlock()
	r.emit(protocol.EvPermission, protocol.PermissionData{
		Request: requestID,
		Call:    data.Call,
		Title:   data.Title,
		Kind:    data.Kind,
		Input:   data.Input,
		Options: options,
	})
}

func normalizePermissionKind(kind string) string {
	switch kind {
	case "allow_once", "allow-once":
		return protocol.OptAllowOnce
	case "allow_always", "allow-always":
		return protocol.OptAllowAlways
	case "reject_once", "reject-once":
		return protocol.OptRejectOnce
	case "reject_always", "reject-always":
		return protocol.OptRejectAlways
	default:
		return kind
	}
}

func isReadOnlyKind(kind string) bool {
	switch kind {
	case protocol.ToolRead, protocol.ToolSearch, protocol.ToolFetch, protocol.ToolThink:
		return true
	default:
		return false
	}
}

func (r *runner) handleReadFile(message rpcMessage) {
	var request struct {
		Path  string `json:"path"`
		Line  *int   `json:"line"`
		Limit *int   `json:"limit"`
	}
	if err := json.Unmarshal(message.Params, &request); err != nil {
		r.respondRequestError(message.ID, err)
		return
	}
	target, inside, err := r.resolveSessionPath(request.Path)
	if err != nil {
		r.respondRequestError(message.ID, err)
		return
	}
	read := func(conn *connection) error {
		content, readErr := readTextLines(target, request.Line, request.Limit)
		if readErr != nil {
			return conn.respondError(message.ID, -32603, readErr.Error())
		}
		return conn.respond(message.ID, map[string]string{"content": content})
	}
	// an Ask reads the whole machine, as Ask's own read_file does, but never
	// a credential
	if r.opts.Ask {
		if err := askReadable(target); err != nil {
			r.respondRequestError(message.ID, err)
			return
		}
	} else if policy := r.approvalPolicy(); !inside || policy == protocol.ApprovalsAsk {
		r.requestFilePermission(message, protocol.ToolRead, request.Path, "", read)
		return
	}
	r.mu.Lock()
	conn := r.conn
	r.mu.Unlock()
	if conn != nil {
		_ = read(conn)
	}
}

func readTextLines(path string, line, limit *int) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	parts := strings.SplitAfter(string(data), "\n")
	start := 0
	if line != nil && *line > 1 {
		start = *line - 1
	}
	if start >= len(parts) {
		return "", nil
	}
	end := len(parts)
	if limit != nil && *limit >= 0 && start+*limit < end {
		end = start + *limit
	}
	return strings.Join(parts[start:end], ""), nil
}

func (r *runner) handleWriteFile(message rpcMessage) {
	var request struct {
		Path    string `json:"path"`
		Content string `json:"content"`
	}
	if err := json.Unmarshal(message.Params, &request); err != nil {
		r.respondRequestError(message.ID, err)
		return
	}
	target, inside, err := r.resolveSessionPath(request.Path)
	if err != nil {
		r.respondRequestError(message.ID, err)
		return
	}
	if !inside {
		r.respondRequestError(message.ID, errors.New(i18n.T("The agent cannot write outside this session's working directory.")))
		return
	}
	policy := r.approvalPolicy()
	if r.opts.Ask {
		policy = r.askMode()
		if err := askReadable(target); err != nil {
			r.respondRequestError(message.ID, err)
			return
		}
		if policy == protocol.ApprovalsReadOnly {
			r.askRefused("", "", protocol.ToolEdit, request.Path, i18n.T("writing a file changes the machine"))
			r.respondRequestError(message.ID, errors.New(i18n.T("This Ask is Read only: Rashin refused the write. Tell the user what to change, or that switching the conversation to Ask first lets you do it.")))
			return
		}
		r.mu.Lock()
		if r.askAllowed {
			policy = protocol.ApprovalsAuto
		}
		r.mu.Unlock()
	}
	if policy == protocol.ApprovalsReadOnly {
		r.respondRequestError(message.ID, errors.New(i18n.T("This session is read-only. Change approvals before writing files.")))
		return
	}
	call, title := r.editCallForWrite(request.Path)
	write := func(conn *connection) error {
		old, readErr := os.ReadFile(target)
		if readErr != nil && !errors.Is(readErr, os.ErrNotExist) {
			return conn.respondError(message.ID, -32603, readErr.Error())
		}
		if writeErr := writeFileAtomic(target, []byte(request.Content)); writeErr != nil {
			return conn.respondError(message.ID, -32603, writeErr.Error())
		}

		oldText, oldCut := clipRunesWithFlag(string(old), toolDiffRunes)
		newText, newCut := clipRunesWithFlag(request.Content, toolDiffRunes)
		if title != "" {
			r.emit(protocol.EvTool, protocol.ToolData{
				Call: call, Title: title, Kind: protocol.ToolEdit, Status: protocol.ToolDone, Input: request.Path,
			})
		}
		r.emit(protocol.EvTool, protocol.ToolData{Call: call, Diff: &protocol.Diff{
			Path: request.Path, Old: oldText, New: newText, Truncated: oldCut || newCut,
		}})
		return conn.respond(message.ID, map[string]any{})
	}
	if policy == protocol.ApprovalsAsk {
		r.requestFilePermission(message, protocol.ToolEdit, request.Path, call, write)
		return
	}
	r.mu.Lock()
	conn := r.conn
	r.mu.Unlock()
	if conn != nil {
		_ = write(conn)
	}
}

func (r *runner) approvalPolicy() string {
	if r.opts.Approvals == nil {
		return protocol.ApprovalsReadOnly
	}
	return r.opts.Approvals()
}

func (r *runner) resolveSessionPath(path string) (string, bool, error) {
	if !filepath.IsAbs(path) {
		return "", false, errors.New(i18n.T("The agent must use an absolute file path."))
	}
	cwd, err := filepath.EvalSymlinks(r.opts.Cwd)
	if err != nil {
		return "", false, err
	}
	clean := filepath.Clean(path)
	parent, err := filepath.EvalSymlinks(filepath.Dir(clean))
	if err != nil {
		return "", false, err
	}
	target := filepath.Join(parent, filepath.Base(clean))
	if _, err := os.Lstat(clean); err == nil {
		target, err = filepath.EvalSymlinks(clean)
		if err != nil {
			return "", false, err
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return "", false, err
	}
	inside := paths.Within(cwd, parent) && paths.Within(cwd, target)
	return target, inside, nil
}

func (r *runner) requestFilePermission(message rpcMessage, kind, path, call string, action func(*connection) error) {
	options := []protocol.PermissionOption{
		{ID: "allow", Label: i18n.T("Allow once"), Kind: protocol.OptAllowOnce},
		{ID: "reject", Label: i18n.T("Reject"), Kind: protocol.OptRejectOnce},
	}
	r.mu.Lock()
	requestID := r.generationIDLocked("permission", rpcID(message.ID))
	if call == "" {
		r.synthetic++
		call = r.generationIDLocked("fs-"+kind, fmt.Sprint(r.synthetic))
	}
	if r.turnCancelled {
		conn := r.conn
		r.mu.Unlock()
		if conn != nil {
			_ = conn.respondError(message.ID, -32603, i18n.T("The file request was cancelled."))
		}
		return
	}
	r.permissions[requestID] = permissionRequest{
		id:       append(json.RawMessage(nil), message.ID...),
		publicID: requestID,
		call:     call,
		options:  options,
		respond: func(conn *connection, option string) error {
			if option != "allow" {
				return conn.respondError(message.ID, -32603, i18n.T("The file request was denied."))
			}
			return action(conn)
		},
		cancel: func(conn *connection) error {
			if conn == nil {
				return nil
			}
			return conn.respondError(message.ID, -32603, i18n.T("The file request was cancelled."))
		},
	}
	r.mu.Unlock()
	r.emit(protocol.EvPermission, protocol.PermissionData{
		Request: requestID,
		Call:    call,
		Title:   path,
		Kind:    kind,
		Input:   path,
		Options: options,
	})
}

func writeFileAtomic(path string, content []byte) error {
	if _, err := os.Lstat(path); err == nil {
		resolved, resolveErr := filepath.EvalSymlinks(path)
		if resolveErr != nil {
			return resolveErr
		}
		path = resolved
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	dir := filepath.Dir(path)
	mode := os.FileMode(0644)
	if st, err := os.Stat(path); err == nil {
		mode = st.Mode().Perm()
	}
	file, err := os.CreateTemp(dir, ".rashin-write-*")
	if err != nil {
		return err
	}
	temp := file.Name()
	ok := false
	defer func() {
		_ = file.Close()
		if !ok {
			_ = os.Remove(temp)
		}
	}()
	if err := file.Chmod(mode); err != nil {
		return err
	}
	if _, err := file.Write(content); err != nil {
		return err
	}
	if err := file.Sync(); err != nil {
		return err
	}
	if err := file.Close(); err != nil {
		return err
	}
	if err := os.Rename(temp, path); err != nil {
		return err
	}
	ok = true
	return nil
}

func (r *runner) editCallForWrite(path string) (string, string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	best, order := "", uint64(0)
	for call, tool := range r.tools {
		if tool.kind == protocol.ToolEdit && tool.status == protocol.ToolRunning && tool.order > order {
			best, order = call, tool.order
		}
	}
	if best != "" {
		return best, ""
	}
	r.synthetic++
	return r.generationIDLocked("fs-write", fmt.Sprint(r.synthetic)), path
}

func (r *runner) respondRequest(id json.RawMessage, result any) {
	r.mu.Lock()
	conn := r.conn
	r.mu.Unlock()
	if conn != nil {
		_ = conn.respond(id, result)
	}
}

func (r *runner) respondRequestError(id json.RawMessage, err error) {
	r.mu.Lock()
	conn := r.conn
	r.mu.Unlock()
	if conn != nil {
		_ = conn.respondError(id, -32603, err.Error())
	}
}
