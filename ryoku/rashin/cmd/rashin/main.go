package main

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	i18n "ryoku-i18n"
	"ryoku-rashin/internal/client"
	"ryoku-rashin/internal/protocol"
)

var version = "dev"

func main() { os.Exit(run(os.Args[1:])) }

func run(args []string) int {
	i18n.Use("")
	if len(args) > 0 && args[0] == "__is-command" {
		return runIsCommand(args[1:])
	}
	if len(args) == 0 || args[0] == "help" || args[0] == "-h" || args[0] == "--help" {
		printHelp()
		return 0
	}
	if command, ok := lookup(args[0]); ok {
		return command.Run(args[1:])
	}
	if isBareAsk(args) {
		command, _ := lookup("ask")
		return command.Run(args)
	}
	fmt.Fprintln(os.Stderr, i18n.Tf("Unknown command: %s", args[0]))
	return 2
}

func isBareAsk(args []string) bool {
	if len(args) == 0 || strings.HasPrefix(args[0], "-") {
		return false
	}
	_, command := lookup(args[0])
	return !command
}

func runIsCommand(args []string) int {
	if len(args) != 1 {
		return 1
	}
	if strings.HasPrefix(args[0], "-") {
		return 0
	}
	if _, ok := lookup(args[0]); ok {
		return 0
	}
	return 1
}

func printHelp() {
	fmt.Fprintln(os.Stdout, i18n.T("Usage: rashin <command> [options]"))
	fmt.Fprintln(os.Stdout)
	group := ""
	for _, command := range sortedCommands() {
		if command.Group != group {
			group = command.Group
			fmt.Fprintln(os.Stdout, i18n.T(strings.ToUpper(group[:1])+group[1:]+":"))
		}
		usage := command.Name
		if command.Usage != "" {
			usage += " " + command.Usage
		}
		fmt.Fprintf(os.Stdout, "  %-30s %s\n", usage, i18n.T(command.Summary))
	}
}

func dial(start bool) (*client.Conn, context.CancelFunc, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	conn, err := client.Dial(ctx, client.Options{Client: protocol.ClientCLI, Start: start})
	if err != nil {
		cancel()
		return nil, func() {}, err
	}
	return conn, cancel, nil
}

func report(err error) int {
	fmt.Fprintln(os.Stderr, err)
	return 1
}
