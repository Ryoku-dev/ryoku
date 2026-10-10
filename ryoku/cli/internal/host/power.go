package host

func (a *App) Power(args []string) int {
	if len(args) != 1 {
		return ExitUsage
	}
	switch args[0] {
	case "poweroff", "reboot", "suspend", "hibernate":
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
