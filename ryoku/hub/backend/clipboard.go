package main

import (
	"encoding/json"
	"fmt"
	"os"
)

// clipboard.go is the Hub's window onto the shell daemon's clipboard storage:
// what the history occupies and the prune that drops everything the user has not
// starred. Both come from the daemon, which owns the entries and their files --
// the Hub never walks a directory to guess a size or delete a file behind the
// daemon's back, so a size it shows and a prune it runs cannot disagree with the
// history the panel is drawing.
//
//	ryoku-hub clipboard stats   print {items,starred,bytes,textBytes,...} as JSON
//	ryoku-hub clipboard prune   drop every unstarred entry, then print the same,
//	                            or null when only that report failed
type clipboardStats struct {
	Items      int   `json:"items"`
	Starred    int   `json:"starred"`
	Bytes      int64 `json:"bytes"`
	TextBytes  int64 `json:"textBytes"`
	ImageBytes int64 `json:"imageBytes"`
	OtherBytes int64 `json:"otherBytes"`
	// LastPrune is when the automatic weekly sweep last ran, unix seconds; 0 when
	// it never has.
	LastPrune int64 `json:"lastPrune,omitempty"`
}

func runClipboard(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("clipboard needs stats|prune")
	}
	switch args[0] {
	case "stats":
		return printClipboardStats()
	case "prune":
		// clear is the prune: the daemon drops every unstarred entry and its
		// backing files, and keeps the starred ones.
		if err := daemonCall("clipboard.clear", nil, nil); err != nil {
			return err
		}
		// The prune is done; a report that fails after it must not turn it into a
		// failure the page offers to retry. null leaves the page without a number.
		if err := printClipboardStats(); err != nil {
			fmt.Fprintf(os.Stderr, "ryoku-hub: history cleared, but its size could not be read: %v\n", err)
			fmt.Println("null")
		}
		return nil
	default:
		return fmt.Errorf("clipboard: unknown action %q", args[0])
	}
}

// printClipboardStats asks the daemon for the storage report and prints it for
// the page. A daemon that is not answering is an error, never a zeroed report: a
// confident "0 B used" while the history holds a gigabyte is worse than no
// number at all.
func printClipboardStats() error {
	var st clipboardStats
	if err := daemonCall("clipboard.stats", nil, &st); err != nil {
		return err
	}
	b, err := json.Marshal(st)
	if err != nil {
		return err
	}
	os.Stdout.Write(b)
	fmt.Println()
	return nil
}
