package doctor

import (
	"bytes"
	"io"
	"strings"

	"ryoku-cli/internal/host"
	"ryoku-cli/internal/sys"

	i18n "ryoku-i18n"
)

var (
	turnstileHostKind = func() (host.InitSystem, error) { return host.Default().Init() }
	turnstileCheck    = func() (int, string) {
		var output bytes.Buffer
		app := host.New(host.Config{Stdout: &output, Stderr: io.Discard})
		return app.SessionCheck(), strings.TrimSpace(output.String())
	}
	turnstileEnsureUser = func() int {
		return host.New(host.Config{Stdout: io.Discard, Stderr: io.Discard}).SessionEnsure("--user")
	}
	turnstileEnsureSystem = func() error {
		return sys.Sudo("ryoku-host", "session", "ensure", "--system")
	}
	turnstileFixWrappers = func() int {
		return host.New(host.Config{Stdout: io.Discard, Stderr: io.Discard}).SessionFixWrappers(nil)
	}
)

func reconcileTurnstile(checkOnly bool) recResult {
	init, err := turnstileHostKind()
	if err != nil {
		return warnRes(i18n.T("could not identify the session service manager: %v"), err)
	}
	if init == host.Systemd {
		return okRes(i18n.T("systemd owns the user session bus"))
	}
	code, findings := turnstileCheck()
	if code == host.ExitOK {
		return okRes(i18n.T("Turnstile and the user D-Bus session are configured"))
	}
	fix := "ryoku-host session ensure --user && sudo ryoku-host session ensure --system && ryoku-host session fix-wrappers"
	if checkOnly {
		return wouldRes(i18n.T("Turnstile session setup needs repair: %s"), findings).withFix(fix)
	}
	if code != host.ExitFalse {
		return failRes(i18n.T("could not inspect Turnstile session setup")).withFix(fix)
	}
	if code := turnstileEnsureUser(); code != host.ExitOK {
		return failRes(i18n.T("could not repair the user Turnstile services (exit %d)"), code).withFix(fix)
	}
	if err := turnstileEnsureSystem(); err != nil {
		return failRes(i18n.T("could not repair the system Turnstile services: %v"), err).withFix(fix)
	}
	if code := turnstileFixWrappers(); code != host.ExitOK {
		return failRes(i18n.T("could not remove nested session bus wrappers (exit %d)"), code).withFix(fix)
	}
	code, findings = turnstileCheck()
	if code != host.ExitOK {
		if findings == "" {
			findings = i18n.T("the session check still fails")
		}
		return warnRes(i18n.T("Turnstile repairs landed, but problems remain: %s"), findings).withFix(fix)
	}
	return fixedRes(i18n.T("configured Turnstile, the user D-Bus service, and direct compositor startup"))
}
