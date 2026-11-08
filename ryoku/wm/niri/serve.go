package main

import (
	"fmt"
	"io"

	wm "ryoku-wm"
)

func runServe(in io.Reader, out io.Writer) error {
	return wm.Serve(in, out, stdout, func(argv []string) error {
		switch argv[0] {
		case "caps":
			return runCaps()
		case "act":
			return runAct(argv[1:])
		}
		return fmt.Errorf("unknown verb %q", argv[0])
	})
}
