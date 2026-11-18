package host

func (a *App) Power(args []string) int {
	if len(args) == 0 {
		return ExitUsage
	}
	switch args[0] {
	case "capabilities", "profile", "idle", "lid", "charge-limit", "aspm":
		return commandExit(a.run("ryoku-power", args...))
	case "poweroff", "reboot", "suspend", "hibernate":
		if len(args) != 1 {
			return ExitUsage
		}
	default:
		return ExitUsage
	}
	init, err := a.Init()
	if err != nil {
		return a.failf("%v", err)
	}
	command := "loginctl"
	if init == Systemd {
		command = "systemctl"
	}
	return commandExit(a.run(command, args[0]))
}
