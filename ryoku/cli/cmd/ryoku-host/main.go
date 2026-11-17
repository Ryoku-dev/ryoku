package main

import (
	"os"

	"ryoku-cli/internal/host"
	"ryoku-cli/internal/ryokuapps"
)

func main() {
	if len(os.Args) > 1 && os.Args[1] == "apps" {
		os.Exit(ryokuapps.HelperMain(os.Args[2:]))
	}
	os.Exit(host.Default().Execute(os.Args[1:]))
}
