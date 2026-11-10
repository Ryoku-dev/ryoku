package vault

import (
	"fmt"

	i18n "ryoku-i18n"
)

func Run(args []string) error {
	command := "status"
	if len(args) > 0 {
		command = args[0]
	}
	switch command {
	case "status":
		if len(args) > 1 {
			return fmt.Errorf(i18n.T("usage: ryoku vault status"))
		}
		status := Inspect()
		fmt.Printf(i18n.T("Path: %s\n"), status.Path)
		fmt.Printf(i18n.T("Exists: %t\n"), status.Exists)
		fmt.Printf(i18n.T("Current: %t\n"), status.Current)
		fmt.Printf(i18n.T("Wired: %t\n"), status.Wired)
		fmt.Printf(i18n.T("Wiring opted out: %t\n"), status.OptedOut)
		fmt.Printf(i18n.T("Rashin 1 vault pending migration: %t\n"), status.LegacyPresent)
		return nil
	case "refresh":
		if len(args) > 1 {
			return fmt.Errorf(i18n.T("usage: ryoku vault refresh"))
		}
		return Refresh()
	case "wire":
		if len(args) > 1 {
			return fmt.Errorf(i18n.T("usage: ryoku vault wire"))
		}
		return Wire()
	case "unwire":
		if len(args) > 1 {
			return fmt.Errorf(i18n.T("usage: ryoku vault unwire"))
		}
		return Unwire()
	case "path":
		if len(args) > 1 {
			return fmt.Errorf(i18n.T("usage: ryoku vault path"))
		}
		fmt.Println(Path())
		return nil
	case "migrate":
		if len(args) > 1 {
			return fmt.Errorf(i18n.T("usage: ryoku vault migrate"))
		}
		return Migrate()
	case "repo-index":
		if len(args) != 3 {
			return fmt.Errorf(i18n.T("usage: ryoku vault repo-index <root> <out>"))
		}
		return RepoIndex(args[1], args[2])
	case "-h", "--help", "help":
		fmt.Print(i18n.T("Usage: ryoku vault [status|refresh|wire|unwire|path|migrate|repo-index <root> <out>]\n"))
		return nil
	default:
		return fmt.Errorf(i18n.T("unknown vault command: %s"), command)
	}
}
