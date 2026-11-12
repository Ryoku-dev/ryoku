package harness

import (
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"ryoku-rashin/internal/paths"
	"ryoku-rashin/internal/protocol"
	"ryoku-rashin/internal/sandbox"
)

// keptEnv is what a team agent keeps of the daemon's environment: where
// programs are, language and terminal, who it runs as, and how the network
// and its certificates are reached. Everything else, API keys and tokens a
// login session exported included, stays outside the sandbox.
var keptEnv = []string{
	"PATH", "LANG", "LANGUAGE", "TERM", "COLORTERM", "TZ", "USER", "LOGNAME", "SHELL", "XDG_RUNTIME_DIR",
	"HTTP_PROXY", "HTTPS_PROXY", "NO_PROXY", "ALL_PROXY", "http_proxy", "https_proxy", "no_proxy", "all_proxy",
	"SSL_CERT_FILE", "SSL_CERT_DIR", "NODE_EXTRA_CA_CERTS", "REQUESTS_CA_BUNDLE",
}

// definitionOf is the definition a harness runs on: its own, or for a custom
// instance the definition whose command it runs, by base name.
func definitionOf(id string, command []string) (definition, bool) {
	for _, def := range definitions {
		if def.id == id {
			return def, true
		}
	}
	if len(command) == 0 {
		return definition{}, false
	}
	base := filepath.Base(command[0])
	for _, def := range definitions {
		for _, candidate := range def.commands {
			if candidate[0] == base {
				return def, true
			}
		}
	}
	return definition{}, false
}

// homeRelative writes paths relative to HOME as the user reads them: "~/.omp".
func homeRelative(relative []string) []string {
	out := make([]string, 0, len(relative))
	for _, rel := range relative {
		out = append(out, "~/"+rel)
	}
	return out
}

// StatePaths are every harness's own state under the user's home: what every
// team sandbox hides, whichever harness it runs.
func StatePaths() []string {
	var out []string
	for _, def := range definitions {
		for _, rel := range def.state {
			out = append(out, filepath.Join(paths.Home(), rel))
		}
	}
	return out
}

// sandboxFor completes a team session's sandbox for harness instance id: it
// runs with HOME and the XDG base dirs at team's own home for it, created
// when missing and its only writable place beside the team's folders, with
// only keptEnv of the daemon's environment beside the instance's own. Every
// other team home stays hidden, and so does the user's harness state
// (StatePaths); the home's guarded paths are read-only. A harness whose
// program lives inside its state sees that program again, read-only.
func sandboxFor(base sandbox.Spec, id, team string, command []string, environment map[string]string) (sandbox.Spec, error) {
	home, err := prepareHome(id, team)
	if err != nil {
		return sandbox.Spec{}, err
	}
	spec := base
	spec.Hidden = append(slices.Clone(base.Hidden), paths.Resolved(paths.HarnessHomes()))
	spec.Writable = append(slices.Clone(base.Writable), home)
	spec.ClearEnv = true
	spec.Env = make(map[string]string)
	for _, name := range keptEnv {
		if value, ok := os.LookupEnv(name); ok {
			spec.Env[name] = value
		}
	}
	for _, entry := range os.Environ() {
		if name, value, ok := strings.Cut(entry, "="); ok && strings.HasPrefix(name, "LC_") {
			spec.Env[name] = value
		}
	}
	maps.Copy(spec.Env, environment)
	maps.Copy(spec.Env, base.Env)
	maps.Copy(spec.Env, homeEnv(home))
	def, _ := definitionOf(id, command)
	spec.ReadOnly = append(slices.Clone(base.ReadOnly), ReadOnlyGuarded(id, team)...)
	for _, rel := range def.programs {
		spec.ReadOnly = append(spec.ReadOnly, paths.Resolved(filepath.Join(paths.Home(), rel)))
	}
	return spec, nil
}

// withSandbox runs the agent inside spec and keeps Rashin from reading,
// writing or running anything on its behalf. The sandbox is tried at every
// start, so an agent whose sandbox cannot run does not start.
func withSandbox(spec sandbox.Spec) runnerOption {
	return func(r *runner) {
		r.sandbox = func(argv []string) ([]string, error) {
			if err := sandbox.Available(); err != nil {
				return nil, err
			}
			return sandbox.Command(spec, argv)
		}
	}
}

// askSandboxFor completes the sandbox an Ask's harness runs in while the Ask
// is read-only. It is the user's own harness as the user runs it, its
// environment and credentials as they are, so only its own state under its
// home (the instance's HOME, else the user's) is made writable, where it
// exists, beside the private /tmp.
func askSandboxFor(base sandbox.Spec, id string, command []string, environment map[string]string) sandbox.Spec {
	home := environment["HOME"]
	if home == "" {
		home = paths.Home()
	}
	spec := base
	spec.Writable = slices.Clone(base.Writable)
	def, _ := definitionOf(id, command)
	for _, rel := range def.state {
		path := paths.Resolved(filepath.Join(home, rel))
		if _, err := os.Stat(path); err == nil {
			spec.Writable = append(spec.Writable, path)
		}
	}
	return spec
}

// withAskSandbox runs an Ask's harness inside spec while the Ask is
// read-only, so what the harness writes or runs in its own process cannot
// change anything either. A mode that moves across Read only restarts the
// harness on its next start, inside the sandbox or out of it.
func withAskSandbox(spec sandbox.Spec) runnerOption {
	return func(r *runner) {
		withSandbox(spec)(r)
		r.sandboxWhen = func() bool { return r.askMode() == protocol.ApprovalsReadOnly }
	}
}
