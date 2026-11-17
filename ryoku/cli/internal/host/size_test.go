package host

import "testing"

func TestParseSize(t *testing.T) {
	cases := map[string]int64{
		"12345":        12345,
		" 0.00 B":      0,
		"512.00 KiB":   512 << 10,
		"12.50 MiB":    12*(1<<20) + (1 << 19),
		"2.00 GiB":     2 << 30,
		"12MB":         12 << 20,
		"1.5GB":        3 << 29,
		"980B":         980,
		"4KB":          4 << 10,
		"1.00 TiB":     1 << 40,
		"7340032 \n":   7340032,
		"3.25 MiB  ":   3*(1<<20) + (1 << 18),
		"100 bytes?":   -1,
		"":             -1,
		"lots":         -1,
		"12 PB":        -1,
		"-1 B":         -1,
		"12.3.4 MiB":   -1,
		"1e3 KiB":      -1,
		"MiB":          -1,
		"0x10 B":       -1,
		"12 MiB extra": -1,
	}
	for in, want := range cases {
		got, ok := parseSize(in)
		if want < 0 {
			if ok {
				t.Errorf("parseSize(%q) = %d, want no size", in, got)
			}
			continue
		}
		if !ok || got != want {
			t.Errorf("parseSize(%q) = %d, %v; want %d", in, got, ok, want)
		}
	}
}

func TestInstalledSizePerManager(t *testing.T) {
	cases := []struct {
		manager string
		command string
		output  string
		want    int64
	}{
		{"pacman", "pacman -Qi -- ryotunes", "Name            : ryotunes\nVersion         : 1:2.0.0-1\nInstalled Size  : 24.00 MiB\nPackager        : Ryoku\n", 24 << 20},
		{"dnf", "rpm -q --qf %{SIZE} -- ryotunes", "25165824", 24 << 20},
		{"xbps", "xbps-query -p installed_size ryotunes", "24MB\n", 24 << 20},
	}
	for _, c := range cases {
		t.Run(c.manager, func(t *testing.T) {
			runner := &fakeRunner{answer: func(command Command) Result {
				if argv(command) != c.command {
					return Result{Code: 1}
				}
				if c.manager == "pacman" && (len(command.Env) != 1 || command.Env[0] != "LC_ALL=C") {
					t.Errorf("pacman -Qi ran without LC_ALL=C: %v", command.Env)
				}
				return Result{Output: c.output}
			}}
			app, _, _ := testApp(runner, map[string]string{"RYOKU_HOST_PKGMGR": c.manager})
			if got, ok := app.InstalledSize("ryotunes"); !ok || got != c.want {
				t.Fatalf("InstalledSize = %d, %v; want %d", got, ok, c.want)
			}
			if _, ok := app.InstalledSize("absent"); ok {
				t.Fatal("an absent package reported a size")
			}
		})
	}
}

// xbps-uhelper cmpver answers through its exit status, never on stdout.
func TestCompareVersionsXBPSExitStatus(t *testing.T) {
	for code, want := range map[int]int{0: 0, 1: 1, 255: -1, 2: 0} {
		runner := &fakeRunner{answer: func(command Command) Result {
			if argv(command) != "xbps-uhelper cmpver 1.0_1 1.1_1" {
				t.Errorf("ran %q", argv(command))
			}
			return Result{Code: code}
		}}
		app, _, _ := testApp(runner, nil)
		if got := app.CompareVersions(XBPS, "1.0_1", "1.1_1"); got != want {
			t.Errorf("exit %d: CompareVersions = %d, want %d", code, got, want)
		}
	}
	app, _, _ := testApp(&fakeRunner{}, nil)
	if got := app.CompareVersions(DNF, "1:2.0.0-1", "1:2.0.0-2"); got >= 0 {
		t.Errorf("DNF compare = %d, want negative", got)
	}
}
