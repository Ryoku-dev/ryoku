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
	register(&Command{Name: "parts", Group: "providers", Usage: "[list|remove <id>]", Summary: "List Hyakki's optional parts, or remove one in the app.", Run: runParts})
}

const partsUsage = "parts [list|remove <id>]"

// partsAction reads the arguments: list, the default, or remove with one
// component id.
func partsAction(args []string) (action, id string, ok bool) {
	switch {
	case len(args) == 0, len(args) == 1 && args[0] == "list":
		return "list", "", true
	case len(args) == 2 && args[0] == "remove" && protocol.ValidComponentID(args[1]):
		return "remove", args[1], true
	default:
		return "", "", false
	}
}

func runParts(args []string) int {
	list := func() (protocol.ComponentsResult, error) {
		var result protocol.ComponentsResult
		conn, cancel, err := dial(true)
		if err != nil {
			return result, err
		}
		defer cancel()
		defer conn.Close()
		err = conn.Call(context.Background(), protocol.MComponents, protocol.Empty{}, &result)
		return result, err
	}
	open := func() int { return runOpen([]string{"--tab", protocol.TabSettings}) }
	return parts(args, os.Stdout, os.Stderr, list, open)
}

// parts lists the parts through list, or, for remove, says why a part cannot
// go now or opens the app through open, where removing one is confirmed:
// only the app may remove a part.
func parts(args []string, out, errs io.Writer, list func() (protocol.ComponentsResult, error), open func() int) int {
	action, id, ok := partsAction(args)
	if !ok {
		fmt.Fprintln(errs, i18n.Tf("Usage: hyakki %s", partsUsage))
		return 2
	}
	result, err := list()
	if err != nil {
		fmt.Fprintln(errs, err)
		return 1
	}
	if action == "list" {
		printParts(out, result)
		return 0
	}
	for _, c := range result.Components {
		if c.ID != id {
			continue
		}
		switch {
		case !c.Removable:
			why := c.Why
			if why == "" {
				why = i18n.T("Hyakki does not remove this part.")
			}
			fmt.Fprintln(errs, why)
			return 1
		case c.InUse != "":
			fmt.Fprintln(errs, c.InUse)
			return 1
		case c.State == protocol.ComponentAbsent:
			fmt.Fprintln(out, i18n.Tf("%s is not on this computer; there is nothing to remove.", c.Title))
			return 0
		}
		if code := open(); code != 0 {
			return code
		}
		fmt.Fprintln(out, i18n.Tf("Confirm removing %s in Hyakki's Settings, under the optional parts.", c.Title))
		return 0
	}
	fmt.Fprintln(errs, i18n.Tf("Hyakki has no optional part called %s.", id))
	return 1
}

// printParts writes one line per part: its id, state, bytes on disk, what
// may happen to it, and its title.
func printParts(w io.Writer, result protocol.ComponentsResult) {
	for _, c := range result.Components {
		fmt.Fprintf(w, "%s\t%s\t%d\t%s\t%s\n", c.ID, c.State, c.Bytes, partVerdict(c), c.Title)
	}
}

// partVerdict is a part's fate in one word: a program the distribution
// provides, one Hyakki keeps, one in use now, or one the app may remove.
func partVerdict(c protocol.Component) string {
	switch {
	case c.Kind == protocol.ComponentProgram:
		return "optional"
	case !c.Removable:
		return "kept"
	case c.InUse != "":
		return "in-use"
	default:
		return "removable"
	}
}
