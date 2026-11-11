package harness

import (
	"strconv"
	"strings"
)

// fakeScreenTurn uses Rashin's screen tools the way a computer-use agent
// would: it opens a terminal on its own screen, types a command, hands the
// screen to the user for one step, and says what it saw
func fakeScreenTurn(write func(any), servers []fakeMCPServer) {
	say := func(text string) {
		writeUpdate(write, map[string]any{"sessionUpdate": "agent_message_chunk", "content": map[string]string{"type": "text", "text": text}})
	}
	if len(servers) == 0 {
		say("Rashin handed me no tools.")
		return
	}
	client, err := startFakeMCP(servers[0])
	if err != nil {
		say(err.Error())
		return
	}
	defer client.close()
	steps := []struct {
		title, tool string
		arguments   map[string]any
	}{
		{"Open a terminal on my screen", "screen_open", map[string]any{"command": "kitty -o font_size=13"}},
		{"Click into the terminal", "screen_click", map[string]any{"x": 640, "y": 400}},
		{"Type a command", "screen_type", map[string]any{"text": "echo the agent was here", "look": false}},
		{"Press Return", "screen_key", map[string]any{"keys": "Return"}},
		{"Ask you to type your name", "screen_ask_user", map[string]any{"reason": "Type your name in the terminal and press Enter, then hand the screen back."}},
		{"Look at the screen", "screen_look", map[string]any{}},
	}
	for index, step := range steps {
		call := "screen-" + strconv.Itoa(index)
		writeUpdate(write, map[string]any{"sessionUpdate": "tool_call", "toolCallId": call, "title": step.title, "kind": "other", "status": "in_progress"})
		text, picture, err := client.call(step.tool, step.arguments)
		// a call that waited on the user as long as it may says so, and the
		// agent calls again; after an ask, looking goes on waiting
		tool, arguments := step.tool, step.arguments
		for err == nil && (strings.HasPrefix(text, "Nothing was done yet") || strings.HasPrefix(text, "The user has not handed the screen back yet")) {
			if tool == "screen_ask_user" {
				tool, arguments = "screen_look", map[string]any{}
			}
			text, picture, err = client.call(tool, arguments)
		}
		status := "completed"
		if err != nil {
			status = "failed"
		}
		if picture {
			text += " (with a picture of the screen)"
		}
		writeUpdate(write, map[string]any{"sessionUpdate": "tool_call_update", "toolCallId": call, "status": status, "content": []map[string]any{{"type": "content", "content": map[string]string{"type": "text", "text": text}}}})
		if err != nil {
			say("The screen step failed: " + err.Error())
			return
		}
	}
	say("I used my own screen: a terminal shows my command and what you typed.")
}
