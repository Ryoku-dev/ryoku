package harness

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"sync"
	"syscall"
	"unicode/utf8"

	"ryoku-i18n"
	"ryoku-rashin/internal/ask/command"
	"ryoku-rashin/internal/protocol"
)

const terminalHardLimit = 1 << 20

type terminal struct {
	id      string
	command string
	cmd     *exec.Cmd
	limit   int

	mu        sync.Mutex
	output    []byte
	streamed  int
	truncated bool
	done      bool
	exitCode  *int
	signal    string
	wait      chan struct{}
	released  bool
}

type terminalExit struct {
	ExitCode *int   `json:"exitCode"`
	Signal   string `json:"signal,omitempty"`
}

func (r *runner) handleTerminalCreate(message rpcMessage) {
	var request struct {
		Command         string   `json:"command"`
		Args            []string `json:"args"`
		Cwd             string   `json:"cwd"`
		OutputByteLimit *int     `json:"outputByteLimit"`
		Env             []struct {
			Name  string `json:"name"`
			Value string `json:"value"`
		} `json:"env"`
	}
	if err := json.Unmarshal(message.Params, &request); err != nil {
		r.respondRequestError(message.ID, err)
		return
	}
	if request.Command == "" {
		r.respondRequestError(message.ID, errors.New("terminal command is empty"))
		return
	}
	// a Read only Ask runs only what reads, whether or not the agent asked
	// first; in the modes that may act its permission request decided
	if r.opts.Ask && r.askMode() == protocol.ApprovalsReadOnly {
		if line := terminalLine(request.Command, request.Args); !command.ReadOnly(line) {
			reason := command.WhyNotReadOnly(line)
			r.askRefused("", "", protocol.ToolExecute, line, reason)
			r.respondRequestError(message.ID, errors.New(i18n.Tf("This Ask is Read only: Rashin refused to run this, since %s. Tell the user what it would do, or that switching the conversation to Ask first lets you run it.", reason)))
			return
		}
	}
	cwd := request.Cwd
	if cwd == "" {
		cwd = r.opts.Cwd
	}
	limit := terminalHardLimit
	if request.OutputByteLimit != nil && *request.OutputByteLimit >= 0 && *request.OutputByteLimit < limit {
		limit = *request.OutputByteLimit
	}
	cmd := exec.Command(request.Command, request.Args...)
	cmd.Dir = cwd
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	environment := os.Environ()
	for _, item := range request.Env {
		environment = setEnvironment(environment, item.Name, item.Value)
	}
	cmd.Env = environment
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		r.respondRequestError(message.ID, err)
		return
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		r.respondRequestError(message.ID, err)
		return
	}
	r.mu.Lock()
	r.synthetic++
	id := r.generationIDLocked("terminal", fmt.Sprint(r.synthetic))
	r.mu.Unlock()
	term := &terminal{id: id, command: strings.Join(append([]string{request.Command}, request.Args...), " "), cmd: cmd, limit: limit, wait: make(chan struct{})}
	if err := cmd.Start(); err != nil {
		r.respondRequestError(message.ID, err)
		return
	}
	r.mu.Lock()
	r.terminals[id] = term
	r.mu.Unlock()
	r.emit(protocol.EvTerminal, protocol.TerminalData{Terminal: id, Command: term.command})
	var readers sync.WaitGroup
	readers.Add(2)
	go r.streamTerminal(term, stdout, &readers)
	go r.streamTerminal(term, stderr, &readers)
	go func() {
		readers.Wait()
		err := cmd.Wait()
		term.finish(err)
		term.mu.Lock()
		exit := term.exitCode
		term.mu.Unlock()
		r.emit(protocol.EvTerminal, protocol.TerminalData{Terminal: id, Exit: exit, Done: true})
	}()
	r.respondRequest(message.ID, map[string]string{"terminalId": id})
}

func (r *runner) streamTerminal(term *terminal, reader io.Reader, readers *sync.WaitGroup) {
	defer readers.Done()
	buffer := make([]byte, 32<<10)
	for {
		n, err := reader.Read(buffer)
		if n > 0 {
			chunk := append([]byte(nil), buffer[:n]...)
			if delta := term.append(chunk); len(delta) != 0 {
				r.emit(protocol.EvTerminal, protocol.TerminalData{Terminal: term.id, Delta: strings.ToValidUTF8(string(delta), "�")})
			}
		}
		if err != nil {
			return
		}
	}
}

func (t *terminal) append(chunk []byte) []byte {
	t.mu.Lock()
	defer t.mu.Unlock()
	var delta []byte
	if remaining := t.limit - t.streamed; remaining > 0 {
		if len(chunk) < remaining {
			remaining = len(chunk)
		}
		delta = append([]byte(nil), chunk[:remaining]...)
		t.streamed += remaining
	}
	if t.limit == 0 {
		if len(chunk) != 0 {
			t.truncated = true
		}
		return delta
	}
	t.output = append(t.output, chunk...)
	if len(t.output) <= t.limit {
		return delta
	}
	over := len(t.output) - t.limit
	for over < len(t.output) && !utf8.RuneStart(t.output[over]) {
		over++
	}
	t.output = append(t.output[:0], t.output[over:]...)
	t.truncated = true
	return delta
}

func (t *terminal) finish(err error) {
	t.mu.Lock()
	if exitErr, ok := err.(*exec.ExitError); ok {
		if status, ok := exitErr.Sys().(syscall.WaitStatus); ok {
			if status.Signaled() {
				t.signal = status.Signal().String()
			} else {
				code := status.ExitStatus()
				t.exitCode = &code
			}
		}
	} else if err == nil {
		code := 0
		t.exitCode = &code
	}
	t.done = true
	close(t.wait)
	t.mu.Unlock()
}

func (r *runner) handleTerminalOutput(message rpcMessage) {
	term, ok := r.requestTerminal(message)
	if !ok {
		return
	}
	term.mu.Lock()
	response := struct {
		Output     string        `json:"output"`
		Truncated  bool          `json:"truncated"`
		ExitStatus *terminalExit `json:"exitStatus"`
	}{Output: strings.ToValidUTF8(string(term.output), "�"), Truncated: term.truncated}
	if term.done {
		response.ExitStatus = &terminalExit{ExitCode: term.exitCode, Signal: term.signal}
	}
	term.mu.Unlock()
	r.respondRequest(message.ID, response)
}

func (r *runner) handleTerminalWait(message rpcMessage) {
	term, ok := r.requestTerminal(message)
	if !ok {
		return
	}
	<-term.wait
	term.mu.Lock()
	response := terminalExit{ExitCode: term.exitCode, Signal: term.signal}
	term.mu.Unlock()
	r.respondRequest(message.ID, response)
}

func (r *runner) handleTerminalKill(message rpcMessage) {
	term, ok := r.requestTerminal(message)
	if !ok {
		return
	}
	_ = signalGroup(term.cmd, syscall.SIGKILL)
	r.respondRequest(message.ID, map[string]any{})
}

func (r *runner) handleTerminalRelease(message rpcMessage) {
	var request struct {
		TerminalID string `json:"terminalId"`
	}
	if err := json.Unmarshal(message.Params, &request); err != nil {
		r.respondRequestError(message.ID, err)
		return
	}
	r.mu.Lock()
	term := r.terminals[request.TerminalID]
	delete(r.terminals, request.TerminalID)
	r.mu.Unlock()
	if term == nil {
		r.respondRequestError(message.ID, errors.New("terminal was not found"))
		return
	}
	term.mu.Lock()
	term.released = true
	done := term.done
	term.mu.Unlock()
	if !done {
		_ = signalGroup(term.cmd, syscall.SIGKILL)
	}
	r.respondRequest(message.ID, map[string]any{})
}

func (r *runner) requestTerminal(message rpcMessage) (*terminal, bool) {
	var request struct {
		TerminalID string `json:"terminalId"`
	}
	if err := json.Unmarshal(message.Params, &request); err != nil {
		r.respondRequestError(message.ID, err)
		return nil, false
	}
	r.mu.Lock()
	term := r.terminals[request.TerminalID]
	r.mu.Unlock()
	if term == nil {
		r.respondRequestError(message.ID, errors.New("terminal was not found"))
		return nil, false
	}
	return term, true
}

func (r *runner) stopTerminals() {
	r.mu.Lock()
	terminals := make([]*terminal, 0, len(r.terminals))
	for id, term := range r.terminals {
		terminals = append(terminals, term)
		delete(r.terminals, id)
	}
	r.mu.Unlock()
	for _, term := range terminals {
		term.mu.Lock()
		done := term.done
		term.released = true
		term.mu.Unlock()
		if !done {
			_ = signalGroup(term.cmd, syscall.SIGKILL)
		}
	}
}

func setEnvironment(environment []string, name, value string) []string {
	prefix := name + "="
	for i, item := range environment {
		if strings.HasPrefix(item, prefix) {
			environment[i] = prefix + value
			return environment
		}
	}
	return append(environment, prefix+value)
}

type tailBuffer struct {
	mu    sync.Mutex
	bytes []byte
	limit int
}

func newTailBuffer(limit int) *tailBuffer {
	return &tailBuffer{limit: limit}
}

func (b *tailBuffer) Write(data []byte) (int, error) {
	b.mu.Lock()
	b.bytes = append(b.bytes, data...)
	if len(b.bytes) > b.limit {
		b.bytes = append(b.bytes[:0], b.bytes[len(b.bytes)-b.limit:]...)
	}
	b.mu.Unlock()
	return len(data), nil
}

func (b *tailBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return strings.TrimSpace(string(bytes.ToValidUTF8(b.bytes, []byte("�"))))
}
