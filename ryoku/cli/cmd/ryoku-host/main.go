package main

import (
	"os"

	"ryoku-cli/internal/host"
)

func main() {
	os.Exit(host.Default().Execute(os.Args[1:]))
}
