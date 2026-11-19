package main

import (
	"context"
	"fmt"
	"os"

	i18n "ryoku-i18n"
	"ryoku-rashin/internal/toolhost"
)

func init() {
	register(&Command{Name: "__toolhost", Group: "internal", Run: runToolHost})
}

func runToolHost(args []string) int {
	if len(args) != 0 {
		return usageError("__toolhost")
	}
	if err := toolhost.Serve(context.Background(), os.Stdin, os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, i18n.Tf("Rashin tool host stopped: %v", err))
		return 1
	}
	return 0
}
