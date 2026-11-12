package sandbox

import "testing"

// The most specific step wins, as the mounts apply them: a folder inside a
// hidden one is seen again, and a hidden path inside a writable one is not.
func TestHidesFollowsTheMostSpecificStep(t *testing.T) {
	spec := Spec{
		Folder:     "/home/me/.local/state/rashin/teams/one",
		Writable:   []string{"/home/me/Projects"},
		Hidden:     []string{"/home/me/.local/state/rashin", "/home/me/Projects/site/.ssh", "/home/me/Projects/notes"},
		ReadOnly:   []string{"/tmp/build/bin"},
		RuntimeDir: "/run/user/1000",
		Binds:      []Bind{{Source: "/run/user/1000/rashin/agents/a", Target: "/run/user/1000/rashin"}},
	}
	for path, want := range map[string]bool{
		"/home/me/.local/state/rashin/sessions/x/events.jsonl": true,
		"/home/me/.local/state/rashin/teams/one/main.go":       false,
		"/home/me/Projects/site/main.go":                       false,
		"/home/me/Projects/site/.ssh/id_ed25519":               true,
		"/home/me/Projects/site/.ssh":                          true,
		"/home/me/Projects/notes/plan.md":                      true,
		"/home/me/Projects/notes-old/plan.md":                  false,
		"/home/me/Documents/letter.txt":                        false,
		"/tmp/scratch.txt":                                     true,
		"/tmp/build/bin/rashin":                                false,
		"/run/user/1000/bus":                                   true,
		"/run/user/1000/rashin/rashin.sock":                    true,
	} {
		if got := spec.Hides(path); got != want {
			t.Errorf("Hides(%s) = %v, want %v", path, got, want)
		}
	}
}
