package host

import "testing"

func TestInstallAdvicePacman(t *testing.T) {
	app, _, _ := testApp(&fakeRunner{}, map[string]string{"RYOKU_HOST_PKGMGR": "pacman"})
	if got := app.InstallAdvice("alpha", "beta"); got != "sudo pacman -S alpha beta" {
		t.Fatalf("advice = %q", got)
	}
}

func TestInstallAdviceXBPS(t *testing.T) {
	table := packageFixture(t)
	app, _, _ := testApp(&fakeRunner{}, map[string]string{"RYOKU_HOST_PKGMGR": "xbps", "RYOKU_HOST_PKG_TABLE": table})
	if got := app.InstallAdvice("combo", "identity"); got != "sudo xbps-install -S void-a void-b identity" {
		t.Fatalf("advice = %q", got)
	}
	if got := app.InstallAdvice("combo", "missing-pkg"); got != "sudo xbps-install -S void-a void-b; missing-pkg is not packaged for this system" {
		t.Fatalf("partial advice = %q", got)
	}
	if got := app.InstallAdvice("repo-pkg", "missing-pkg"); got != "sudo xbps-install -S repo-pkg; missing-pkg is not packaged for this system" {
		t.Fatalf("repo advice = %q", got)
	}
}
