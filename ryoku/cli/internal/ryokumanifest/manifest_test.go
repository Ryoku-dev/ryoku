package ryokumanifest

import (
	"reflect"
	"testing"
)

func TestBuildCarriesCachyOSReplaces(t *testing.T) {
	m := Build(Inputs{
		Base:            "base\nlinux\nlinux-headers\n",
		AUR:             "yay-bin\n",
		CachyOSReplaces: "# the stock kernel\nlinux\n\nyay-bin\n",
	})
	if !reflect.DeepEqual(m.CachyOSReplaces, []string{"linux", "yay-bin"}) {
		t.Fatalf("CachyOSReplaces = %v, want [linux yay-bin]", m.CachyOSReplaces)
	}
	// The lanes themselves stay whole: an Arch box still needs the stock kernel.
	if !reflect.DeepEqual(m.Base, []string{"base", "linux", "linux-headers"}) {
		t.Errorf("Base = %v, want the full base set", m.Base)
	}
}

func TestForCachyOSDropsReplacedNamesFromEveryLane(t *testing.T) {
	m := Manifest{
		Base:            []string{"base", "linux", "linux-headers"},
		Dev:             []string{"go", "linux"},
		AUR:             []string{"paru-bin", "yay-bin"},
		Hardware:        map[string][]string{"intel": {"linux", "thermald"}},
		FirstParty:      []string{"ryoku-desktop"},
		Provisioned:     []string{"kitty"},
		CachyOSReplaces: []string{"linux", "yay-bin"},
	}
	got := m.ForCachyOS()
	if !reflect.DeepEqual(got.Base, []string{"base", "linux-headers"}) {
		t.Errorf("Base = %v, want [base linux-headers]", got.Base)
	}
	if !reflect.DeepEqual(got.Dev, []string{"go"}) {
		t.Errorf("Dev = %v, want [go]", got.Dev)
	}
	if !reflect.DeepEqual(got.AUR, []string{"paru-bin"}) {
		t.Errorf("AUR = %v, want [paru-bin]", got.AUR)
	}
	if !reflect.DeepEqual(got.Hardware["intel"], []string{"thermald"}) {
		t.Errorf("Hardware[intel] = %v, want [thermald]", got.Hardware["intel"])
	}
	if !reflect.DeepEqual(got.FirstParty, m.FirstParty) || !reflect.DeepEqual(got.Provisioned, m.Provisioned) {
		t.Errorf("first-party or provisioned lanes changed: %+v", got)
	}
	// The served manifest is cached and saved as the baseline; filtering must
	// not write through to it.
	if !reflect.DeepEqual(m.Base, []string{"base", "linux", "linux-headers"}) ||
		!reflect.DeepEqual(m.Hardware["intel"], []string{"linux", "thermald"}) {
		t.Errorf("ForCachyOS modified its receiver: %+v", m)
	}
}

func TestForCachyOSKeepsAnOlderManifestUnchanged(t *testing.T) {
	m := Manifest{Base: []string{"base", "linux"}}
	if got := m.ForCachyOS(); !reflect.DeepEqual(got, m) {
		t.Errorf("a manifest without cachyosReplaces changed: %+v", got)
	}
}
