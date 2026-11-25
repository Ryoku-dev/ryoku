// Package characters holds the characters Hyakki ships, built into the
// program: one folder per id with its profile.json, artwork, persona, lines
// and the license and provenance of each. Only the profile package reads it.
package characters

import "embed"

// FS holds the shipped character folders at its top level.
//
//go:embed oni
var FS embed.FS
