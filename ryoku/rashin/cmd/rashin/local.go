package main

import (
	"context"
	"fmt"
	"io"
	"os"

	i18n "ryoku-i18n"
	"ryoku-rashin/internal/protocol"
)

func init() {
	register(&Command{Name: "local", Group: "providers", Usage: "[list|pull <id>|rm <id>|runtime install]", Summary: "List or manage models run by Rashin on this computer.", Run: runLocal})
}

func localAction(args []string) (action, id string, ok bool) {
	switch {
	case len(args) == 0, len(args) == 1 && args[0] == "list":
		return "list", "", true
	case len(args) == 2 && (args[0] == "pull" || args[0] == "rm") && args[1] != "":
		return args[0], args[1], true
	case len(args) == 2 && args[0] == "runtime" && args[1] == "install":
		return "runtime", "", true
	default:
		return "", "", false
	}
}

func runLocal(args []string) int {
	action, id, ok := localAction(args)
	if !ok {
		return usageError("local [list|pull <id>|rm <id>|runtime install]")
	}
	if action != "list" {
		if code := runOpen([]string{"--tab", protocol.TabSettings, "--view", "providers"}); code != 0 {
			return code
		}
		switch action {
		case "pull":
			fmt.Fprintln(os.Stdout, i18n.Tf("Confirm the licence and download %s in Rashin's Providers settings.", id))
		case "rm":
			fmt.Fprintln(os.Stdout, i18n.Tf("Remove %s in Rashin's Providers settings.", id))
		case "runtime":
			fmt.Fprintln(os.Stdout, i18n.T("Install llama.cpp in Rashin's Providers settings."))
		}
		return 0
	}
	conn, cancel, err := dial(true)
	if err != nil {
		return report(err)
	}
	defer cancel()
	defer conn.Close()
	var result protocol.LocalResult
	if err := conn.Call(context.Background(), protocol.MLocalModels, protocol.Empty{}, &result); err != nil {
		return report(err)
	}
	printLocal(os.Stdout, result)
	return 0
}

func printLocal(w io.Writer, result protocol.LocalResult) {
	for _, model := range result.Models {
		var size int64
		for _, file := range model.Files {
			size += file.Size
		}
		fmt.Fprintf(w, "%s\t%s\t%d\n", model.ID, model.State, size)
	}
	fmt.Fprintf(w, "runtime\t%s\t%s\n", result.Runtime.State, result.Runtime.Version)
}
