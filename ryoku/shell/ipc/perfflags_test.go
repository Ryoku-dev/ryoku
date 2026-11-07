package main

import "testing"

func TestParsePerfFlag(t *testing.T) {
	cases := []struct {
		name string
		body string
		key  string
		want bool
	}{
		{"true", `{"unloadWidgetsWhenCovered":true}`, "unloadWidgetsWhenCovered", true},
		{"false", `{"unloadWidgetsWhenCovered":false}`, "unloadWidgetsWhenCovered", false},
		{"other key", `{"freezeVisualizerWhenIdle":true}`, "unloadWidgetsWhenCovered", false},
		{"absent", `{}`, "unloadOverviewWhenIdle", false},
		{"malformed", `not json`, "x", false},
		{"empty", ``, "x", false},
		{"wrong type", `{"x":"yes"}`, "x", false},
		{"palette key", `{"unloadOverviewWhenIdle":true}`, "unloadOverviewWhenIdle", true},
	}
	for _, c := range cases {
		if got := parsePerfFlag([]byte(c.body), c.key); got != c.want {
			t.Errorf("%s: parsePerfFlag(%q,%q) = %v, want %v", c.name, c.body, c.key, got, c.want)
		}
	}
}
