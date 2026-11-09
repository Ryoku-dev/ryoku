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
	modeName := flag.String("mode", "follow", "follow, dark or light")
	flag.Parse()

	modes := map[string]palette.Mode{"follow": palette.Follow, "dark": palette.Dark, "light": palette.Light}
	mode, ok := modes[*modeName]
	if !ok {
		fmt.Fprintf(os.Stderr, "lithic-roles: unknown mode %q\n", *modeName)
		os.Exit(2)
	}

	loaded := palette.Load(mode)
	roles := palette.FromPalette(loaded)
	out := map[string]string{"source": loaded.Source}
	value := reflect.ValueOf(roles)
	for i := range value.NumField() {
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
