package harness

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strconv"
	"time"
)

// fakeMCPServer is the stdio MCP server Rashin handed the scripted agent in
// session/new
type fakeMCPServer struct {
	Command string   `json:"command"`
	Args    []string `json:"args"`
	Env     []struct {
		Name  string `json:"name"`
		Value string `json:"value"`
	} `json:"env"`
}

func fakeMCPServers(params json.RawMessage) []fakeMCPServer {
	var request struct {
		MCPServers []fakeMCPServer `json:"mcpServers"`
	}
	_ = json.Unmarshal(params, &request)
	return request.MCPServers
}

// fakeMCPClient is the scripted agent's side of an MCP server: started once,
// handshaken, then called the way an agent calls tools
type fakeMCPClient struct {
	command *exec.Cmd
	stdin   io.WriteCloser
	reader  *bufio.Reader
	next    int
}

func startFakeMCP(server fakeMCPServer) (*fakeMCPClient, error) {
	command := exec.Command(server.Command, server.Args...)
	command.Env = os.Environ()
	for _, variable := range server.Env {
		command.Env = append(command.Env, variable.Name+"="+variable.Value)
	}
	stdin, err := command.StdinPipe()
	if err != nil {
		return nil, err
	}
	stdout, err := command.StdoutPipe()
	if err != nil {
		return nil, err
	}
	if err := command.Start(); err != nil {
		return nil, err
	}
	client := &fakeMCPClient{command: command, stdin: stdin, reader: bufio.NewReader(stdout), next: 1}
	if _, err := client.request("initialize", map[string]any{"protocolVersion": "2025-06-18", "capabilities": map[string]any{}, "clientInfo": map[string]string{"name": "fake", "version": "1"}}); err != nil {
		client.close()
		return nil, err
	}
	encoded, _ := json.Marshal(map[string]any{"jsonrpc": "2.0", "method": "notifications/initialized"})
	_, _ = stdin.Write(append(encoded, '\n'))
	return client, nil
}

func (client *fakeMCPClient) request(method string, params any) (map[string]any, error) {
	client.next++
	encoded, _ := json.Marshal(map[string]any{"jsonrpc": "2.0", "id": client.next, "method": method, "params": params})
	if _, err := client.stdin.Write(append(encoded, '\n')); err != nil {
		return nil, err
	}
	line, err := client.reader.ReadBytes('\n')
	if err != nil {
		return nil, err
	}
	var reply map[string]any
	return reply, json.Unmarshal(line, &reply)
}

// call runs one tool and returns its text, and whether it sent a picture
func (client *fakeMCPClient) call(tool string, arguments map[string]any) (string, bool, error) {
	reply, err := client.request("tools/call", map[string]any{"name": tool, "arguments": arguments})
	if err != nil {
		return "", false, err
	}
	result, _ := reply["result"].(map[string]any)
	content, _ := result["content"].([]any)
	if len(content) == 0 {
		return "", false, fmt.Errorf("no answer: %v", reply)
	}
	text, _ := content[0].(map[string]any)["text"].(string)
	picture := false
	for _, block := range content {
		if kind, _ := block.(map[string]any)["type"].(string); kind == "image" {
			picture = true
		}
	}
	if failed, _ := result["isError"].(bool); failed {
		return text, picture, fmt.Errorf("%s", text)
	}
	return text, picture, nil
}

func (client *fakeMCPClient) close() {
	_ = client.stdin.Close()
	_ = client.command.Wait()
}

// fakeGoalTurn works one turn toward a goal: a step each turn, and on the
// third it checks its work and ends the goal through Rashin's goal tool
func fakeGoalTurn(write func(any), servers []fakeMCPServer, prompts int) {
	call := "goal-step-" + strconv.Itoa(prompts)
	writeUpdate(write, map[string]any{"sessionUpdate": "tool_call", "toolCallId": call, "title": "Step " + strconv.Itoa(prompts) + " of 3", "kind": "execute", "status": "in_progress"})
	time.Sleep(fakeAgentPace())
	writeUpdate(write, map[string]any{"sessionUpdate": "tool_call_update", "toolCallId": call, "status": "completed"})
	if prompts < 3 {
		writeUpdate(write, map[string]any{"sessionUpdate": "agent_message_chunk", "content": map[string]string{"type": "text", "text": "Step " + strconv.Itoa(prompts) + " is done; going on."}})
		return
	}
	answer := "Rashin handed me no tools."
	if len(servers) > 0 {
		client, err := startFakeMCP(servers[0])
		if err == nil {
			answer, _, err = client.call("goal", map[string]any{"op": "complete", "summary": "All three steps are done and checked."})
			client.close()
		}
		if err != nil {
			answer = err.Error()
		}
	}
	writeUpdate(write, map[string]any{"sessionUpdate": "agent_message_chunk", "content": map[string]string{"type": "text", "text": answer}})
}
