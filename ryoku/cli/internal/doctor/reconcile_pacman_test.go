package doctor

import "testing"

func stubPacmanHost(t *testing.T, available bool) {
	t.Helper()
	old := hasPacman
	hasPacman = func() bool { return available }
	t.Cleanup(func() { hasPacman = old })
}

// enableILoveCandy must land the directive inside [options] and nowhere else:
// pacman only honors it there, and a config it cannot parse breaks every
// pacman invocation on the box.
func TestEnableILoveCandy(t *testing.T) {
	const stock = "[options]\nHoldPkg = pacman glibc\n\n[core]\nInclude = /etc/pacman.d/mirrorlist\n"
	for _, c := range []struct {
		name, conf, want string
		changed, ok      bool
	}{
		{
			name:    "inserted under the options header",
			conf:    stock,
			want:    "[options]\nILoveCandy\nHoldPkg = pacman glibc\n\n[core]\nInclude = /etc/pacman.d/mirrorlist\n",
			changed: true, ok: true,
		},
		{
			name:    "commented directive is uncommented in place",
			conf:    "[options]\nHoldPkg = pacman glibc\n#ILoveCandy\n\n[core]\nInclude = /etc/pacman.d/mirrorlist\n",
			want:    "[options]\nHoldPkg = pacman glibc\nILoveCandy\n\n[core]\nInclude = /etc/pacman.d/mirrorlist\n",
			changed: true, ok: true,
		},
		{
			name:    "already active is a no-op",
			conf:    "[options]\nILoveCandy\nHoldPkg = pacman glibc\n",
			want:    "[options]\nILoveCandy\nHoldPkg = pacman glibc\n",
			changed: false, ok: true,
		},
		{
			name:    "a directive in a repo section does not count as active",
			conf:    "[options]\nHoldPkg = pacman glibc\n\n[ryoku]\nILoveCandy\n",
			want:    "[options]\nILoveCandy\nHoldPkg = pacman glibc\n\n[ryoku]\nILoveCandy\n",
			changed: true, ok: true,
		},
		{
			name: "no options section: nothing is guessed",
			conf: "[core]\nInclude = /etc/pacman.d/mirrorlist\n",
			ok:   false,
		},
	} {
		t.Run(c.name, func(t *testing.T) {
			out, changed, ok := enableILoveCandy([]byte(c.conf))
			if ok != c.ok || changed != c.changed {
				t.Fatalf("changed=%v ok=%v, want changed=%v ok=%v", changed, ok, c.changed, c.ok)
			}
			if !ok {
				return
			}
			if string(out) != c.want {
				t.Fatalf("got:\n%q\nwant:\n%q", out, c.want)
			}
		})
	}
}

// Applying the result twice must be stable: the reconciler runs on every
// `ryoku update`, and a second pass may not stack a duplicate directive.
func TestEnableILoveCandyIdempotent(t *testing.T) {
	once, _, ok := enableILoveCandy([]byte("[options]\nHoldPkg = pacman glibc\n"))
	if !ok {
		t.Fatal("first pass did not apply")
	}
	twice, changed, ok := enableILoveCandy(once)
	if !ok || changed {
		t.Fatalf("second pass changed=%v ok=%v, want changed=false ok=true", changed, ok)
	}
	if string(twice) != string(once) {
		t.Fatalf("second pass rewrote the config: %q", twice)
	}
}

func TestPacmanHostUsesHostPackageManager(t *testing.T) {
	t.Setenv("RYOKU_HOST_PKGMGR", "xbps")
	if hasPacman() {
		t.Fatal("xbps host reported pacman")
	}
}

func TestPacmanReconcilersAreNeutralOnOtherPackageManagers(t *testing.T) {
	stubPacmanHost(t, false)
	checks := []struct {
		name string
		run  func() recResult
	}{
		{"boot guard", func() recResult { return reconcileBootGuard(false) }},
		{"channel pin", func() recResult { return reconcileChannelPin(false) }},
		{"pacman progress", func() recResult { return reconcilePacmanCandy(false) }},
		{"multilib", func() recResult { return reconcileMultilibRepo(false) }},
		{"manifest", func() recResult { return reconcileManifest(false) }},
		{"shipped apps", func() recResult { return reconcileShippedApps(false) }},
		{"retired apps", func() recResult { return reconcileRetiredApps(false) }},
	}
	for _, check := range checks {
		t.Run(check.name, func(t *testing.T) {
			if got := check.run(); got.status != recOK && got.status != recNote {
				t.Fatalf("result = %#v, want a neutral result", got)
			}
		})
	}
}
