package host

import "path/filepath"

func (a *App) Session(args []string) int {
	if len(args) == 0 || args[0] != "start" {
		return ExitUsage
	}
	extra := args[1:]
	init, err := a.Init()
	if err != nil {
		return a.failf("%v", err)
	}
	if init == Runit {
		lib := a.getenv("RYOKU_INIT_LIB")
		if lib == "" {
			lib = a.cfg.InitLibDir
		}
		result := a.cfg.Runner.Run(Command{Name: filepath.Join(lib, "session-start"), Replace: true, Stdin: a.cfg.Stdin, Stdout: a.cfg.Stdout, Stderr: a.cfg.Stderr})
		return commandExit(result)
	}
	// These first steps deliberately continue after failure. The session's
	// original shell chain only gated portal restart on power cutover.
	_ = a.run("dbus-update-activation-environment", "--systemd", "--all")
	_ = a.run("systemctl", "--user", "daemon-reload")
	_ = a.run("ryoku-reload-cover", "begin", "boot")
	if result := a.run("ryoku-power-cutover", "session-start-logged"); result.Code != 0 {
		return ExitFailure
	}
	units := []string{"--user", "try-restart", "xdg-desktop-portal.service"}
	units = append(units, extra...)
	return commandExit(a.run("systemctl", units...))
}
