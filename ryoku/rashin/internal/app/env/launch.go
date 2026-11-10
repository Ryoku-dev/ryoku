package env

// a file, a folder or a web address
func (e *Env) Open(target string) {
	if target != "" {
		e.launch(nil, "xdg-open", target)
	}
}

func (e *Env) OpenTerminal(folder string) {
	if folder != "" {
		e.launch(nil, "ryoku-app", "terminal", "--", "sh", "-c", `cd "$1" && exec "${SHELL:-/bin/sh}"`, "sh", folder)
	}
}

// leaves a shell behind the command so its output stays
func (e *Env) RunInTerminal(command string) {
	e.launch(nil, "ryoku-app", "terminal", "--", "sh", "-c", command+`; exec "$SHELL"`)
}

// done may be nil
func (e *Env) Rashin(done func(error), args ...string) { e.launch(done, "rashin", args...) }

func (e *Env) launch(done func(error), name string, args ...string) {
	if e.Desktop.Launch != nil {
		e.Desktop.Launch(name, args, done)
	}
}
