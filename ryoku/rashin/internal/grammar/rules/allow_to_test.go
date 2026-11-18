package rules

import (
	"ryoku-rashin/internal/grammar/linttest"
	"testing"
)

func TestAllowTo_FlagAllowTo(t *testing.T) {
	linttest.AssertLintCount(t, "Allow to change approval policy during running task # 4394.", NewAllowTo(), 1)
}

func TestAllowTo_FlagAllowingTo(t *testing.T) {
	linttest.AssertLintCount(t, "Allowing to have multiple views with different filtering # 952.", NewAllowTo(), 1)
}

func TestAllowTo_FlagAllowsTo(t *testing.T) {
	linttest.AssertLintCount(t, "It is easily doable for classic IHostBuilder, because its extension allows to pass configure action", NewAllowTo(), 1)
}

func TestAllowTo_DontFlagAllowedTo(t *testing.T) {
	linttest.AssertNoLints(t, "In C and C++ aliasing has to do with what expression types we are allowed to access stored values through.", NewAllowTo())
}

func TestAllowTo_DontFlagAllowPronounTo(t *testing.T) {
	linttest.AssertNoLints(t, "It would be really great to allow me to enter body data using multipart form", NewAllowTo())
}

func TestAllowTo_DontFlagAllowNounTo(t *testing.T) {
	linttest.AssertNoLints(t, "Allows users to export SMART statistics from any connected hard drive", NewAllowTo())
}

func TestAllowTo_DontFlagAllowNpTo(t *testing.T) {
	linttest.AssertNoLints(t, "This vulnerability allows an authenticated attacker to infer data from the database by measuring response times", NewAllowTo())
}
