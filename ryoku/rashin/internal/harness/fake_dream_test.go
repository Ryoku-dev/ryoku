package harness

import (
	"encoding/json"
	"os"
	"strconv"
	"strings"
)

// fakeDreamTurn plays one turn of a dream with Rashin's tools. The manager,
// whose tools the daemon marked as a dream's, first asks the CEO one round
// of questions with its plan; on the answer it hires one member and hands
// it the page, on notes it hands the page out again, and once a report is in
// it moves to testing and presents. Any other member writes the page into
// its folder and reports.
func fakeDreamTurn(write func(any), servers []fakeMCPServer, prompts int, params json.RawMessage) {
	if len(servers) == 0 {
		fakeSay(write, "Rashin handed me no tools.")
		return
	}
	client, err := startFakeMCP(servers[0])
	if err != nil {
		fakeSay(write, err.Error())
		return
	}
	defer client.close()
	step := func(tool string, arguments map[string]any) {
		call := "dream-" + tool + "-" + strconv.Itoa(prompts)
		writeUpdate(write, map[string]any{"sessionUpdate": "tool_call", "toolCallId": call, "title": tool, "kind": "other", "status": "in_progress"})
		answer, _, err := client.call(tool, arguments)
		if err != nil {
			answer = "failed: " + err.Error()
		}
		writeUpdate(write, map[string]any{"sessionUpdate": "tool_call_update", "toolCallId": call, "status": "completed"})
		fakeSay(write, tool+": "+answer)
	}
	if !fakeServerEnv(servers[0], "RASHIN_TEAM_DREAM", "1") {
		page := "<!doctype html><title>ESP32 setup</title><h1>Join your network</h1>\n"
		if err := os.WriteFile("index.html", []byte(page), 0o600); err != nil {
			fakeSay(write, err.Error())
		}
		step("report", map[string]any{"title": "The setup page", "body": "index.html is written: a page asking for the network name and password.", "files": []string{"index.html"}})
		return
	}
	text := fakePromptText(params)
	switch {
	case prompts == 1:
		step("dream_status", map[string]any{"phase": "interviewing", "summary": "Asking how the page should look."})
		step("ask_user", map[string]any{"title": "How should the setup page look?", "question": "Plan: Ada writes one page asking for the Wi-Fi name and password, a second member checks it. Should it list nearby networks, and light or dark?"})
	case strings.Contains(text, "finished the pieces"):
		step("dream_status", map[string]any{"phase": "testing", "summary": "The page is in; checking it."})
		step("present", map[string]any{"title": "ESP32 setup page, turn " + strconv.Itoa(prompts), "body": "Join ESP32-Setup and the page asks for your network.", "tests": "Opened index.html; both fields show.", "files": []string{"index.html"}})
	default:
		if !strings.Contains(text, "sent notes") {
			step("hire", map[string]any{"name": "Ada", "title": "Web engineer", "instructions": "You own the setup page the ESP32 serves.", "why": "The page is a piece one member can own."})
		}
		step("delegate", map[string]any{"member": "Ada", "title": "Setup page", "brief": "Write index.html: a page asking for the Wi-Fi name and password, and report."})
		step("dream_status", map[string]any{"phase": "building", "summary": "Ada writes the setup page."})
	}
}

// fakePromptText is the text a session/prompt carries
func fakePromptText(params json.RawMessage) string {
	var request struct {
		Prompt []struct {
			Text string `json:"text"`
		} `json:"prompt"`
	}
	_ = json.Unmarshal(params, &request)
	var b strings.Builder
	for _, block := range request.Prompt {
		b.WriteString(block.Text)
	}
	return b.String()
}

// fakeServerEnv reports whether Rashin started the MCP server with name set
// to value
func fakeServerEnv(server fakeMCPServer, name, value string) bool {
	for _, variable := range server.Env {
		if variable.Name == name && strings.TrimSpace(variable.Value) == value {
			return true
		}
	}
	return false
}

func fakeSay(write func(any), text string) {
	writeUpdate(write, map[string]any{"sessionUpdate": "agent_message_chunk", "content": map[string]string{"type": "text", "text": text + "\n"}})
}
