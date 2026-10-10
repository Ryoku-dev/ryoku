package host

import "testing"

func TestCapabilitiesJSONForSupportedHosts(t *testing.T) {
	tests := []struct {
		name string
		env  map[string]string
		want string
	}{
		{
			name: "pacman-systemd",
			env:  map[string]string{"RYOKU_HOST_PKGMGR": "pacman", "RYOKU_HOST_INIT": "systemd"},
			want: "{\"packageManager\":\"pacman\",\"init\":\"systemd\",\"snapshots\":{\"supported\":true,\"reason\":\"\"},\"aur\":{\"supported\":true,\"reason\":\"\"}}\n",
		},
		{
			name: "xbps-runit",
			env:  map[string]string{"RYOKU_HOST_PKGMGR": "xbps", "RYOKU_HOST_INIT": "runit"},
			want: "{\"packageManager\":\"xbps\",\"init\":\"runit\",\"snapshots\":{\"supported\":false,\"reason\":\"Snapshots are not available on Void Linux, so updates cannot be rolled back.\"},\"aur\":{\"supported\":false,\"reason\":\"The AUR is an Arch Linux service; Void installs come from XBPS.\"}}\n",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			app, stdout, stderr := testApp(&fakeRunner{}, tc.env)
			if code := app.Execute([]string{"capabilities"}); code != ExitOK {
				t.Fatalf("capabilities exit = %d", code)
			}
			if got := stdout.String(); got != tc.want {
				t.Fatalf("capabilities = %q, want %q", got, tc.want)
			}
			if stderr.Len() != 0 {
				t.Fatalf("stderr = %q", stderr.String())
			}
		})
	}
}

func TestCapabilitiesRejectsArguments(t *testing.T) {
	app, _, _ := testApp(&fakeRunner{}, map[string]string{"RYOKU_HOST_PKGMGR": "pacman", "RYOKU_HOST_INIT": "systemd"})
	if code := app.Execute([]string{"capabilities", "extra"}); code != ExitUsage {
		t.Fatalf("capabilities with argument exit = %d", code)
	}
}
