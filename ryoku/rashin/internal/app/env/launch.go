package env

import (
	"image"
	"os"
	"strings"

	"ryoku-i18n"

	"ryoku-rashin/internal/app/desktop"
)

// the sizes a window opened from here takes in front of a floating Rashin
var (
	terminalFront = image.Pt(900, 560)
	filesFront    = image.Pt(1100, 700)
)

// a file or a web address, in whatever the desktop opens it with
func (e *Env) Open(target string) {
	if target == "" {
		return
	}
	request := desktop.LaunchRequest{Name: "xdg-open", Args: []string{target}}
	if strings.Contains(target, "://") {
		// a browser keeps its own window and place, so a floating Rashin only says the page went there
		request.Hidden = func() { e.Notify(i18n.T("Opened in your browser")) }
	} else {
		request.Front = filesFront
	}
	e.launch(request, nil)
}

// the file manager the user chose for Ryoku, at this folder
func (e *Env) OpenFolder(folder string) {
	if folder != "" {
		e.launch(desktop.LaunchRequest{
			Name: "ryoku-app", Args: []string{"files", "--", folder},
			Fallback: []string{"xdg-open", folder}, Front: filesFront,
		}, nil)
	}
}

// a folder goes to the file manager, anything else to what opens it
func (e *Env) OpenPath(path string) {
	if info, err := os.Stat(path); err == nil && info.IsDir() {
		e.OpenFolder(path)
		return
	}
	e.Open(path)
}

func (e *Env) OpenTerminal(folder string) {
	if folder != "" {
		e.terminal("sh", "-c", `cd "$1" && exec "${SHELL:-/bin/sh}"`, "sh", folder)
	}
}

// the command is shown as if typed, or one that takes a minute leaves an empty window that looks broken
// it runs from the home folder, where Ask's own tools run, and a shell stays so the output does
const runScript = `cd; printf '\033[2m$ %s\033[0m\n' "$1"; eval "$1"; status=$?; [ "$status" -eq 0 ] || printf '\033[2m[exit %s]\033[0m\n' "$status"; exec "${SHELL:-/bin/sh}"`

func (e *Env) RunInTerminal(command string) {
	e.terminal("sh", "-c", runScript, "sh", command)
}

func (e *Env) terminal(inner ...string) {
	e.launch(desktop.LaunchRequest{
		Name: "ryoku-app", Args: append([]string{"terminal", "--"}, inner...), Front: terminalFront,
	}, nil)
}

// done may be nil
func (e *Env) Rashin(done func(error), args ...string) {
	e.launch(desktop.LaunchRequest{Name: "rashin", Args: args}, done)
}

func (e *Env) launch(request desktop.LaunchRequest, done func(error)) {
	if e.Desktop.Launch != nil {
		e.Desktop.Launch(request, done)
	}
}
