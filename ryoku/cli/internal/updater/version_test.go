package updater

import "testing"

// `ryoku version` falls back to this core on a box without /etc/ryoku-release;
// the monorepo packages carry epoch 1 from 1.0.9 on, and "v1:1.0.9" is wrong.
func TestPackagedVersionPartsDropsTheEpoch(t *testing.T) {
	cases := []struct{ installed, base, sha string }{
		{"1:1.0.9.r6200.gbbbbbbb-1", "1.0.9", "bbbbbbb"},
		{"1.11.4.r6100.gaaaaaaa-1", "1.11.4", "aaaaaaa"},
		{"1:1.1.0.r6300.gccccccc-5", "1.1.0", "ccccccc"},
	}
	for _, c := range cases {
		base, sha := packagedVersionParts(c.installed)
		if base != c.base || sha != c.sha {
			t.Errorf("packagedVersionParts(%q) = %q, %q; want %q, %q", c.installed, base, sha, c.base, c.sha)
		}
	}
}
