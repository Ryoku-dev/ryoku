package logos

import (
	"embed"
	"strings"
	"sync"
)

// masks are written by bin/ryoku-dev-sync-provider-logos from list.txt
//
//go:embed list.txt masks/*.png
var files embed.FS

// a Rashin id to its mark's file name, "" for an id listed without one
var table = sync.OnceValue(func() map[string]string {
	data, err := files.ReadFile("list.txt")
	if err != nil {
		return nil
	}
	result := make(map[string]string)
	for line := range strings.Lines(string(data)) {
		fields := strings.Fields(line)
		if len(fields) != 2 || strings.HasPrefix(fields[0], "#") {
			continue
		}
		if fields[1] == "-" {
			fields[1] = ""
		}
		result[fields[0]] = fields[1]
	}
	return result
})

// the mark drawn for a provider or harness id, "" when there is none
func markFor(id string) string {
	return table()[id]
}
