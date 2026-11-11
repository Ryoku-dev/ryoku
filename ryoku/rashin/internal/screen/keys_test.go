package screen

import (
	"strings"
	"testing"
)

func TestChordsBecomeWtypePressesInOrder(t *testing.T) {
	cases := map[string]string{
		"ctrl+shift+t": "-M ctrl -M shift -k t -m shift -m ctrl",
		"Ctrl+L":       "-M ctrl -k l -m ctrl",
		"alt+f4":       "-M alt -k F4 -m alt",
		"super+Enter":  "-M logo -k Return -m logo",
		"esc":          "-k Escape",
		"A":            "-k A",
	}
	for chord, want := range cases {
		args, err := keyArgs(chord)
		if err != nil || strings.Join(args, " ") != want {
			t.Errorf("%s = %v, %v; want %s", chord, args, err, want)
		}
	}
	for _, chord := range []string{"", "ctrl+", "hyper+x"} {
		if args, err := keyArgs(chord); err == nil {
			t.Errorf("%q gave %v; want an error", chord, args)
		}
	}
}
