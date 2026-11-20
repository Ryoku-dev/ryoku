package main

import (
	"embed"
)

//go:embed schema.json
var schemaFS embed.FS

// runSchema prints Umbriel's exclusive settings rows. Only rows with a real
// writer or action are listed; the Hub renders them through the shared renderer.
func runSchema() error {
	body, err := schemaFS.ReadFile("schema.json")
	if err != nil {
		return err
	}
	_, err = stdout.Write(body)
	return err
}
