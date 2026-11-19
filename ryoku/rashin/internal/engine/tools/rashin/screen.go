package rashin

import (
	"context"
	"encoding/base64"
	"errors"

	i18n "ryoku-i18n"
	"ryoku-rashin/internal/engine/tool"
	"ryoku-rashin/internal/llm"
	"ryoku-rashin/internal/protocol"
)

const screenAbout = "Your own screen: a hidden 1280x800 desktop that is not the user's, with its own home folder. " +
	"The first screen tool you call asks the user to give you one. Coordinates are pixels of the frames you get back. " +
	"The user can watch it and take it over; while they have it, your actions wait for them, and one that waits long returns saying nothing was done yet, so call it again."

func screenWaitingMessage(kind string) string {
	switch kind {
	case protocol.ScreenWaitConsent:
		return i18n.T("Nothing was done yet: the user has not answered the request to give you a screen, which waits in their conversation. Call this tool again to keep waiting.")
	case protocol.ScreenWaitUser:
		return i18n.T("Nothing was done yet: the user has your screen, doing what you asked or having taken it over. Call this tool again to keep waiting for them to hand it back.")
	default:
		return i18n.T("Nothing was done yet. Call the screen tool again.")
	}
}

// screenTools let the agent use apps on a screen of its own: look at it,
// point, click, drag, scroll, type, press keys, open programs, and hand it
// to the user for what only they should do, such as signing in. The screen
// service asks the user before the first action, so they never ask here.
func screenTools() []tool.Tool {
	look := boolean("look", "Return the screen after the action settles. On by default; turn off to chain quick actions.")
	coordinate := func(name, description string) field {
		return field{name, map[string]any{"type": "integer", "minimum": 0, "description": description}}
	}
	x, y := coordinate("x", "Pixels from the left."), coordinate("y", "Pixels from the top.")
	enabled := func(env tool.Env) bool { return env.Context.Screen }
	screen := func(name, kind, show, description string, title func() string, schema object, run func(ctx context.Context, args arguments, env tool.Env) tool.Result) tool.Tool {
		return &def{name: name, kind: kind, show: show, title: title, describe: static(description), schema: fixed(schema), enabled: enabled, run: run}
	}
	// act decodes the arguments into an action and does it
	act := func(build func(args arguments) (protocol.ScreenAction, error)) func(context.Context, arguments, tool.Env) tool.Result {
		return func(ctx context.Context, args arguments, env tool.Env) tool.Result {
			action, err := build(args)
			if err != nil {
				return failed(err)
			}
			var options struct {
				Look *bool `json:"look"`
			}
			if err := args.decode(&options); err != nil {
				return failed(err)
			}
			result, _ := screenAct(ctx, env, action, options.Look == nil || *options.Look)
			return result
		}
	}
	type point struct {
		X *int `json:"x"`
		Y *int `json:"y"`
	}
	at := func(args arguments, kind string) (protocol.ScreenAction, error) {
		var p point
		if err := args.decode(&p); err != nil {
			return protocol.ScreenAction{}, err
		}
		if p.X == nil || p.Y == nil {
			return protocol.ScreenAction{}, errors.New(i18n.T("Give x and y."))
		}
		return protocol.ScreenAction{Kind: kind, X: *p.X, Y: *p.Y}, nil
	}
	return []tool.Tool{
		screen(tool.ScreenLook, protocol.ToolRead, "", screenAbout+" screen_look returns what the screen shows now.",
			func() string { return i18n.T("Looking at the screen") }, newObject(nil),
			func(ctx context.Context, _ arguments, env tool.Env) tool.Result {
				result, _ := screenAct(ctx, env, protocol.ScreenAction{Kind: "look"}, true)
				return result
			}),
		screen(tool.ScreenClick, protocol.ToolOther, "", "Click on your screen at x,y. button is left, right or middle; double clicks twice.",
			func() string { return i18n.T("Clicking on the screen") },
			newObject([]string{"x", "y"}, x, y, enum("button", "The mouse button; left unless set.", "left", "right", "middle"), boolean("double", "Click twice."), look),
			act(func(args arguments) (protocol.ScreenAction, error) {
				var options struct {
					Button string `json:"button"`
					Double bool   `json:"double"`
				}
				if err := args.decode(&options); err != nil {
					return protocol.ScreenAction{}, err
				}
				kind := "click"
				switch {
				case options.Double:
					kind = "double"
				case options.Button == "right" || options.Button == "middle":
					kind = options.Button
				}
				return at(args, kind)
			})),
		screen(tool.ScreenMove, protocol.ToolOther, "", "Move the pointer on your screen to x,y, for hover menus and tooltips.",
			func() string { return i18n.T("Pointing on the screen") }, newObject([]string{"x", "y"}, x, y, look),
			act(func(args arguments) (protocol.ScreenAction, error) { return at(args, "move") })),
		screen(tool.ScreenDrag, protocol.ToolOther, "", "Drag with the left button held from x,y to to_x,to_y on your screen.",
			func() string { return i18n.T("Dragging on the screen") },
			newObject([]string{"x", "y", "to_x", "to_y"}, coordinate("x", "Start, from the left."), coordinate("y", "Start, from the top."), coordinate("to_x", "End, from the left."), coordinate("to_y", "End, from the top."), look),
			act(func(args arguments) (protocol.ScreenAction, error) {
				var p struct {
					X   *int `json:"x"`
					Y   *int `json:"y"`
					ToX *int `json:"to_x"`
					ToY *int `json:"to_y"`
				}
				if err := args.decode(&p); err != nil {
					return protocol.ScreenAction{}, err
				}
				if p.X == nil || p.Y == nil || p.ToX == nil || p.ToY == nil {
					return protocol.ScreenAction{}, errors.New(i18n.T("Give x, y, to_x and to_y."))
				}
				return protocol.ScreenAction{Kind: "drag", X: *p.X, Y: *p.Y, ToX: *p.ToX, ToY: *p.ToY}, nil
			})),
		screen(tool.ScreenScroll, protocol.ToolOther, "", "Turn the mouse wheel at x,y on your screen: down notches (negative scrolls up) and right notches (negative scrolls left).",
			func() string { return i18n.T("Scrolling on the screen") },
			newObject([]string{"x", "y"}, x, y, field{"down", map[string]any{"type": "integer", "description": "Notches down; negative scrolls up."}}, field{"right", map[string]any{"type": "integer", "description": "Notches right; negative scrolls left."}}, look),
			act(func(args arguments) (protocol.ScreenAction, error) {
				action, err := at(args, "scroll")
				if err != nil {
					return action, err
				}
				var wheel struct {
					Down  int `json:"down"`
					Right int `json:"right"`
				}
				if err := args.decode(&wheel); err != nil {
					return action, err
				}
				action.DX, action.DY = wheel.Right, wheel.Down
				if action.DX == 0 && action.DY == 0 {
					action.DY = 3
				}
				return action, nil
			})),
		screen(tool.ScreenType, protocol.ToolOther, "", "Type text into whatever has the keyboard on your screen. Click the field first.",
			func() string { return i18n.T("Typing on the screen") }, newObject([]string{"text"}, str("text", "The text to type."), look),
			act(func(args arguments) (protocol.ScreenAction, error) {
				var typed struct {
					Text string `json:"text"`
				}
				err := args.decode(&typed)
				return protocol.ScreenAction{Kind: "type", Text: typed.Text}, err
			})),
		screen(tool.ScreenKey, protocol.ToolOther, "keys", "Press a key or chord on your screen, such as Return, Escape, Tab, ctrl+l, ctrl+shift+t or alt+F4.",
			func() string { return i18n.T("Pressing keys on the screen") }, newObject([]string{"keys"}, str("keys", "The key or chord."), look),
			act(func(args arguments) (protocol.ScreenAction, error) {
				var pressed struct {
					Keys string `json:"keys"`
				}
				err := args.decode(&pressed)
				return protocol.ScreenAction{Kind: "key", Keys: pressed.Keys}, err
			})),
		&def{
			name: tool.ScreenOpen, kind: protocol.ToolExecute, show: "command", enabled: enabled,
			title:    func() string { return i18n.T("Opening a program on the screen") },
			describe: static("Open a program on your screen with a command line, such as firefox https://example.org or kitty. It starts in your screen's own home folder; the result shows it after a short wait."),
			schema:   fixed(newObject([]string{"command"}, str("command", "The command line to run."), look)),
			aliases:  map[string]string{"cmd": "command"},
			run: func(ctx context.Context, args arguments, env tool.Env) tool.Result {
				var opened struct {
					Command string `json:"command"`
					Look    *bool  `json:"look"`
				}
				if err := args.decode(&opened); err != nil {
					return failed(err)
				}
				if result, done := screenAct(ctx, env, protocol.ScreenAction{Kind: "open", Command: opened.Command}, false); !done {
					return result
				}
				result, _ := screenAct(ctx, env, protocol.ScreenAction{Kind: "wait", Seconds: 2}, opened.Look == nil || *opened.Look)
				return result
			},
		},
		screen(tool.ScreenAskUser, protocol.ToolOther, "reason", "Hand your screen to the user for something only they should do, such as signing in or solving a captcha. Say exactly what you need; this waits for them to hand the screen back and returns it, or says they are not done yet, and then screen_look goes on waiting.",
			func() string { return i18n.T("Handing the screen to the user") }, newObject([]string{"reason"}, str("reason", "What you need the user to do on the screen.")),
			func(ctx context.Context, args arguments, env tool.Env) tool.Result {
				var asked struct {
					Reason string `json:"reason"`
				}
				if err := args.decode(&asked); err != nil {
					return failed(err)
				}
				var state protocol.ScreenState
				if err := env.Host.Call(ctx, protocol.MScreenControl, protocol.ScreenControlParams{ID: env.Session.ID, Op: protocol.ScreenAsk, Reason: asked.Reason}, &state); err != nil {
					return failed(err)
				}
				switch {
				case state.State == protocol.ScreenStarting:
					return tool.Result{Text: screenWaitingMessage(protocol.ScreenWaitConsent)}
				case state.Ask != "":
					return tool.Result{Text: i18n.T("The user has not handed the screen back yet. Call screen_look to wait for them; it returns the screen once they do.")}
				}
				result, _ := screenAct(ctx, env, protocol.ScreenAction{Kind: "look"}, true)
				if !result.IsError {
					result.Text = i18n.T("The user handed the screen back.") + "\n" + result.Text
				}
				return result
			}),
		screen(tool.ScreenStop, protocol.ToolOther, "", "Close your screen and every program on it once you are done with it.",
			func() string { return i18n.T("Closing the screen") }, newObject(nil),
			func(ctx context.Context, _ arguments, env tool.Env) tool.Result {
				var state protocol.ScreenState
				if err := env.Host.Call(ctx, protocol.MScreenControl, protocol.ScreenControlParams{ID: env.Session.ID, Op: protocol.ScreenStop}, &state); err != nil {
					return failed(err)
				}
				return tool.Result{Text: i18n.T("The screen is closed.")}
			}),
	}
}

// screenAct does one action on the session's screen and returns the frame
// after it when asked, with what was done in words; done is false when the
// action failed or still waits on the user.
func screenAct(ctx context.Context, env tool.Env, action protocol.ScreenAction, look bool) (tool.Result, bool) {
	var acted protocol.ScreenActResult
	params := protocol.ScreenActParams{ID: env.Session.ID, By: protocol.HolderAgent, Action: action, Look: look}
	if err := env.Host.Call(ctx, protocol.MScreenAct, params, &acted); err != nil {
		return failed(err), false
	}
	if acted.Waiting != "" {
		return tool.Result{Text: screenWaitingMessage(acted.Waiting)}, false
	}
	result := tool.Result{Text: i18n.Tf("%s. Screen %dx%d.", acted.Screen.Action, acted.Screen.Width, acted.Screen.Height)}
	if acted.Image != "" {
		if frame, err := base64.StdEncoding.DecodeString(acted.Image); err == nil {
			result.Images = []llm.Image{{Mime: "image/png", Data: frame}}
		}
	}
	return result, true
}
