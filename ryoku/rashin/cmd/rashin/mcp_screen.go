package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	i18n "ryoku-i18n"
	"ryoku-rashin/internal/mcp"
	"ryoku-rashin/internal/protocol"
)

const screenAbout = "Your own screen: a hidden 1280x800 desktop that is not the user's, with its own home folder. " +
	"The first screen tool you call asks the user to give you one. Coordinates are pixels of the frames you get back. " +
	"The user can watch it and take it over; while they have it, your actions wait for them, and one that waits long returns saying nothing was done yet, so call it again."

// screenTools let the agent use apps on a screen of its own: look at it,
// point, click, drag, scroll, type, press keys, open programs, and hand it to
// the user for what only they should do, such as signing in
func screenTools(session mcpSession) []mcp.Tool {
	look := map[string]any{"type": "boolean", "description": "Return the screen after the action settles. On by default; turn off to chain quick actions."}
	coordinate := func(description string) map[string]any {
		return map[string]any{"type": "integer", "minimum": 0, "description": description}
	}
	act := func(kind string, build func(arguments map[string]any) (protocol.ScreenAction, error)) func(context.Context, json.RawMessage) mcp.Result {
		return func(ctx context.Context, raw json.RawMessage) mcp.Result {
			var arguments map[string]any
			if err := json.Unmarshal(raw, &arguments); err != nil {
				return mcp.Failure(errors.New(i18n.T("The arguments are not valid JSON for this tool.")))
			}
			action, err := build(arguments)
			if err != nil {
				return mcp.Failure(err)
			}
			action.Kind = kind
			wantLook := true
			if value, ok := arguments["look"].(bool); ok {
				wantLook = value
			}
			result, _ := screenCall(ctx, session, action, wantLook)
			return result
		}
	}
	point := func(arguments map[string]any) (protocol.ScreenAction, error) {
		x, xOK := integer(arguments["x"])
		y, yOK := integer(arguments["y"])
		if !xOK || !yOK {
			return protocol.ScreenAction{}, errors.New(i18n.T("Give x and y."))
		}
		return protocol.ScreenAction{X: x, Y: y}, nil
	}
	return []mcp.Tool{
		{
			Name: "screen_look", Description: screenAbout + " screen_look returns what the screen shows now.",
			Input: schema(nil, map[string]any{}), ReadOnly: true,
			Call: act("look", func(map[string]any) (protocol.ScreenAction, error) { return protocol.ScreenAction{}, nil }),
		},
		{
			Name: "screen_click", Description: "Click on your screen at x,y. button is left, right or middle; double clicks twice.",
			Input: schema([]string{"x", "y"}, map[string]any{"x": coordinate("Pixels from the left."), "y": coordinate("Pixels from the top."), "button": map[string]any{"type": "string", "enum": []string{"left", "right", "middle"}}, "double": map[string]any{"type": "boolean"}, "look": look}),
			Call: func(ctx context.Context, raw json.RawMessage) mcp.Result {
				var arguments struct {
					Button string `json:"button"`
					Double bool   `json:"double"`
				}
				_ = json.Unmarshal(raw, &arguments)
				kind := "click"
				switch {
				case arguments.Double:
					kind = "double"
				case arguments.Button == "right" || arguments.Button == "middle":
					kind = arguments.Button
				}
				return act(kind, point)(ctx, raw)
			},
		},
		{
			Name: "screen_move", Description: "Move the pointer on your screen to x,y, for hover menus and tooltips.",
			Input: schema([]string{"x", "y"}, map[string]any{"x": coordinate("Pixels from the left."), "y": coordinate("Pixels from the top."), "look": look}),
			Call:  act("move", point),
		},
		{
			Name: "screen_drag", Description: "Drag with the left button held from x,y to to_x,to_y on your screen.",
			Input: schema([]string{"x", "y", "to_x", "to_y"}, map[string]any{"x": coordinate("Start, from the left."), "y": coordinate("Start, from the top."), "to_x": coordinate("End, from the left."), "to_y": coordinate("End, from the top."), "look": look}),
			Call: act("drag", func(arguments map[string]any) (protocol.ScreenAction, error) {
				action, err := point(arguments)
				toX, xOK := integer(arguments["to_x"])
				toY, yOK := integer(arguments["to_y"])
				if err != nil || !xOK || !yOK {
					return protocol.ScreenAction{}, errors.New(i18n.T("Give x, y, to_x and to_y."))
				}
				action.ToX, action.ToY = toX, toY
				return action, nil
			}),
		},
		{
			Name: "screen_scroll", Description: "Turn the mouse wheel at x,y on your screen: down notches (negative scrolls up) and right notches (negative scrolls left).",
			Input: schema([]string{"x", "y"}, map[string]any{"x": coordinate("Pixels from the left."), "y": coordinate("Pixels from the top."), "down": map[string]any{"type": "integer"}, "right": map[string]any{"type": "integer"}, "look": look}),
			Call: act("scroll", func(arguments map[string]any) (protocol.ScreenAction, error) {
				action, err := point(arguments)
				if err != nil {
					return action, err
				}
				action.DY, _ = integer(arguments["down"])
				action.DX, _ = integer(arguments["right"])
				if action.DX == 0 && action.DY == 0 {
					action.DY = 3
				}
				return action, nil
			}),
		},
		{
			Name: "screen_type", Description: "Type text into whatever has the keyboard on your screen. Click the field first.",
			Input: schema([]string{"text"}, map[string]any{"text": stringProperty("The text to type."), "look": look}),
			Call: act("type", func(arguments map[string]any) (protocol.ScreenAction, error) {
				text, _ := arguments["text"].(string)
				if text == "" {
					return protocol.ScreenAction{}, errors.New(i18n.Tf("Missing %s.", "text"))
				}
				return protocol.ScreenAction{Text: text}, nil
			}),
		},
		{
			Name: "screen_key", Description: "Press a key or chord on your screen, such as Return, Escape, Tab, ctrl+l, ctrl+shift+t or alt+F4.",
			Input: schema([]string{"keys"}, map[string]any{"keys": stringProperty("The key or chord."), "look": look}),
			Call: act("key", func(arguments map[string]any) (protocol.ScreenAction, error) {
				keys, _ := arguments["keys"].(string)
				if keys == "" {
					return protocol.ScreenAction{}, errors.New(i18n.Tf("Missing %s.", "keys"))
				}
				return protocol.ScreenAction{Keys: keys}, nil
			}),
		},
		{
			Name: "screen_open", Description: "Open a program on your screen with a command line, such as firefox https://example.org or kitty. It starts in your screen's own home folder; the result shows it after a short wait.",
			Input: schema([]string{"command"}, map[string]any{"command": stringProperty("The command line to run."), "look": look}),
			Call: func(ctx context.Context, raw json.RawMessage) mcp.Result {
				var arguments struct {
					Command string `json:"command"`
					Look    *bool  `json:"look"`
				}
				if err := toolArguments(raw, &arguments, "command"); err != nil {
					return mcp.Failure(err)
				}
				if result, done := screenCall(ctx, session, protocol.ScreenAction{Kind: "open", Command: arguments.Command}, false); !done {
					return result
				}
				result, _ := screenCall(ctx, session, protocol.ScreenAction{Kind: "wait", Seconds: 2}, arguments.Look == nil || *arguments.Look)
				return result
			},
		},
		{
			Name: "screen_ask_user", Description: "Hand your screen to the user for something only they should do, such as signing in or solving a captcha. Say exactly what you need; this waits for them to hand the screen back and returns it, or says they are not done yet, and then screen_look goes on waiting.",
			Input: schema([]string{"reason"}, map[string]any{"reason": stringProperty("What you need the user to do on the screen.")}),
			Call: func(ctx context.Context, raw json.RawMessage) mcp.Result {
				var arguments struct {
					Reason string `json:"reason"`
				}
				if err := toolArguments(raw, &arguments, "reason"); err != nil {
					return mcp.Failure(err)
				}
				if session.id == "" {
					return mcp.Failure(errors.New(i18n.T("This server was not started for a Rashin session.")))
				}
				var state protocol.ScreenState
				if err := daemonCall(ctx, protocol.MScreenControl, protocol.ScreenControlParams{ID: session.id, Op: protocol.ScreenAsk, Reason: arguments.Reason}, &state); err != nil {
					return mcp.Failure(err)
				}
				switch {
				case state.State == protocol.ScreenStarting:
					return mcp.Text(screenWaiting[protocol.ScreenWaitConsent])
				case state.Ask != "":
					return mcp.Text("The user has not handed the screen back yet. Call screen_look to wait for them; it returns the screen once they do.")
				}
				result, _ := screenCall(ctx, session, protocol.ScreenAction{Kind: "look"}, true)
				result.Content = append([]mcp.Content{{Type: "text", Text: "The user handed the screen back."}}, result.Content...)
				return result
			},
		},
		{
			Name: "screen_stop", Description: "Close your screen and every program on it once you are done with it.",
			Input: schema(nil, map[string]any{}),
			Call: func(ctx context.Context, _ json.RawMessage) mcp.Result {
				var state protocol.ScreenState
				if err := daemonCall(ctx, protocol.MScreenControl, protocol.ScreenControlParams{ID: session.id, Op: protocol.ScreenStop}, &state); err != nil {
					return mcp.Failure(err)
				}
				return mcp.Text("The screen is closed.")
			},
		},
	}
}

// screenWaiting tells the agent why its action was not done yet; the call
// returned before its harness would give up on it
var screenWaiting = map[string]string{
	protocol.ScreenWaitConsent: "Nothing was done yet: the user has not answered the request to give you a screen, which waits in their conversation. Call this tool again to keep waiting.",
	protocol.ScreenWaitUser:    "Nothing was done yet: the user has your screen, doing what you asked or having taken it over. Call this tool again to keep waiting for them to hand it back.",
}

// screenCall acts on the session's screen and returns the frame after it
// when asked, with what was done in words; done is false when the action
// failed or still waits on the user
func screenCall(ctx context.Context, session mcpSession, action protocol.ScreenAction, look bool) (result mcp.Result, done bool) {
	if session.id == "" {
		return mcp.Failure(errors.New(i18n.T("This server was not started for a Rashin session."))), false
	}
	var acted protocol.ScreenActResult
	if err := daemonCall(ctx, protocol.MScreenAct, protocol.ScreenActParams{ID: session.id, By: protocol.HolderAgent, Action: action, Look: look}, &acted); err != nil {
		return mcp.Failure(err), false
	}
	if acted.Waiting != "" {
		return mcp.Text(screenWaiting[acted.Waiting]), false
	}
	text := fmt.Sprintf("%s. Screen %dx%d.", acted.Screen.Action, acted.Screen.Width, acted.Screen.Height)
	content := []mcp.Content{{Type: "text", Text: text}}
	if acted.Image != "" {
		content = append(content, mcp.Content{Type: "image", Data: acted.Image, MimeType: "image/png"})
	}
	return mcp.Result{Content: content}, true
}

func daemonCall(ctx context.Context, method string, params, result any) error {
	conn, cancel, err := dial(true)
	if err != nil {
		return err
	}
	defer cancel()
	defer conn.Close()
	return conn.Call(ctx, method, params, result)
}

// integer reads a JSON number that is a whole number
func integer(value any) (int, bool) {
	number, ok := value.(float64)
	if !ok || number != float64(int(number)) {
		return 0, false
	}
	return int(number), true
}
