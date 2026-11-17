package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func runRiceCapture(args []string) error {
	force := false
	var positional []string
	for _, arg := range args {
		switch arg {
		case "--help", "-h":
			fmt.Println("usage: ryoku-hub rice capture [--force] <name> [all|layer ...]")
			fmt.Println("Save the current desktop look. Use --force to replace a saved rice.")
			return nil
		case "--force":
			force = true
		default:
			if strings.HasPrefix(strings.TrimSpace(arg), "-") {
				return fmt.Errorf("rice capture: unknown option %q", arg)
			}
			positional = append(positional, arg)
		}
	}
	if len(positional) == 0 {
		return fmt.Errorf("rice capture needs a name")
	}
	name := positional[0]
	slug := slugify(name)
	if slug == "" {
		return fmt.Errorf("a rice needs a name")
	}
	if !force {
		_, err := os.Stat(filepath.Join(ricesDir(), slug, "rice.json"))
		if err == nil {
			return fmt.Errorf("rice %q exists; pass --force to replace it", slug)
		}
		if !os.IsNotExist(err) {
			return fmt.Errorf("check saved rice %q: %w", slug, err)
		}
	}
	r, err := captureRice(name, positional[1:])
	if err != nil {
		return err
	}
	return printJSON(r)
}
