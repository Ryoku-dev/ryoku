// Command lithic-roles prints the Lithic roles this machine resolves, as JSON.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"image/color"
	"os"
	"reflect"
	"strings"

	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/palette"
)

func main() {
	modeName := flag.String("mode", "dark", "dark, light or follow")
	contrastName := flag.String("contrast", "standard", "standard or high")
	tintName := flag.String("tint", "off", "off, low or medium")
	flag.Parse()

	mode, err := palette.ParseMode(*modeName)
	if err != nil {
		fmt.Fprintln(os.Stderr, "lithic-roles:", err)
		os.Exit(2)
	}
	if *contrastName != "standard" && *contrastName != "high" {
		fmt.Fprintf(os.Stderr, "lithic-roles: unknown contrast %q\n", *contrastName)
		os.Exit(2)
	}
	if *tintName != "off" && *tintName != "low" && *tintName != "medium" {
		fmt.Fprintf(os.Stderr, "lithic-roles: unknown tint %q\n", *tintName)
		os.Exit(2)
	}

	loaded := palette.Load(mode)
	roles := palette.FromPalette(loaded, palette.ParseLook(*contrastName, *tintName))
	out := map[string]string{"source": loaded.Source}
	value := reflect.ValueOf(roles)
	for i := range value.NumField() {
		if !value.Type().Field(i).IsExported() {
			continue
		}
		c, isColor := value.Field(i).Interface().(color.NRGBA)
		if !isColor {
			continue
		}
		name := value.Type().Field(i).Name
		out[strings.ToLower(name[:1])+name[1:]] = fmt.Sprintf("#%02x%02x%02x%02x", c.R, c.G, c.B, c.A)
	}

	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(out); err != nil {
		fmt.Fprintln(os.Stderr, "lithic-roles:", err)
		os.Exit(1)
	}
}
