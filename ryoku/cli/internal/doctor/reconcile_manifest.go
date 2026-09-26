package doctor

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"ryoku-cli/internal/ryokumanifest"
	"ryoku-cli/internal/sys"
	"ryoku-cli/internal/updater"

	i18n "ryoku-i18n"
)

// ---- reconciler: converge to the release's control manifest ------------------
//
// The channel serves manifest.json: every package the release is made of, by
// lane. This reconciler moves the box toward the served manifest and away from
// the baseline it last converged to, which is what makes "add a package to a set
// and it reaches every box on the next update" true without a hard depend, and
// what makes two boxes provably the same machine.
//
// The rules, in the order they are decided:
//
//   - A name the manifest wants and this box lacks is installed, unless it was
//     present when the baseline was saved -- then the user deleted it and it
//     stays deleted. This also self-heals a box that never received a name the
//     release has always carried.
//   - A name the release RETIRED is reported, never uninstalled. A box keeps
//     what it has; removing it is the user's call.
//   - On the FIRST reconcile nothing installs: with no baseline there is no way
//     to tell a never-delivered name from a user removal. The run records the
//     baseline and reports the gap.
//   - The provisioned apps are reconcileShippedApps' deliver-once lane; this
//     reconciler moves base/dev/first-party/AUR and never double-installs them.
//   - The user's own additions (explicit, named by no lane) are left alone and
//     shown by `ryoku verify` so two machines can be compared.
//
// It is best-effort and never fails an update over a package: a box with no
// mirror, or no network, reports what did not land and moves on.

var (
	// installManifestPkgs is one bounded transaction for the whole added set.
	installManifestPkgs = func(pkgs []string) {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
		defer cancel()
		args := append([]string{"pacman", "-S", "--needed", "--noconfirm"}, pkgs...)
		_ = exec.CommandContext(ctx, "sudo", args...).Run()
	}
	// installManifestAUR arrives through the AUR helper, best-effort: the set is
	// online-only by contract, so a box without a helper or a network just
	// reports the miss, exactly as the installer's post-install AUR step does.
	installManifestAUR = func(pkgs []string) {
		if !sys.Has("yay") {
			return
		}
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Minute)
		defer cancel()
		args := append([]string{"yay", "-S", "--needed", "--noconfirm", "--answerupgrade", "all"}, pkgs...)
		_ = exec.CommandContext(ctx, "sudo", args...).Run()
	}
)

func reconcileManifest(checkOnly bool) recResult {
	if !hasPacman() {
		return okRes(i18n.T("not a pacman box; the manifest is the installer's business"))
	}
	served, err := updater.FetchManifest()
	if err != nil {
		// A checkout, a private mirror, or an offline first boot: nothing to
		// converge to yet, which is not a fault.
		return okRes(i18n.T("no release manifest to converge to (%v)"), err)
	}
	applied := updater.LoadApplied()
	installed, err := updater.PacmanInstalled()
	if err != nil {
		return warnRes(i18n.T("could not read the installed set: %v"), err)
	}
	var prev *ryokumanifest.Manifest
	var prevPresent map[string]bool
	if applied != nil {
		prev = &applied.Manifest
		prevPresent = applied.PresentSet()
	}
	plan := updater.PlanManifest(prev, prevPresent, &served, installed)

	if checkOnly {
		return wouldRes("%s", planSummary(&plan)).
			withFix(i18n.T("run `ryoku doctor` (or `ryoku update`) to apply"))
	}

	// The provisioned lane (the deliver-once apps) is reconcileShippedApps'
	// business; this reconciler moves the base/dev/first-party/AUR lanes and
	// never touches the ledger, so "the user deleted this" stays legible for
	// every package through the presence baseline rather than a flooded ledger.
	var landed, missed []string
	if len(plan.Install) > 0 {
		installManifestPkgs(plan.Install)
		for _, pkg := range plan.Install {
			if sys.PkgInstalled(pkg) {
				landed = append(landed, pkg)
			} else {
				missed = append(missed, pkg)
			}
		}
	}
	var aurMissed []string
	if len(plan.AUR) > 0 {
		installManifestAUR(plan.AUR)
		for _, pkg := range plan.AUR {
			if !sys.PkgInstalled(pkg) {
				aurMissed = append(aurMissed, pkg)
			}
		}
	}

	// Save the baseline only when everything that should have landed did: a
	// partial run reconciles again next update rather than recording an added
	// name as delivered when it is not. The presence set is sticky over the
	// previous baseline, so a user removal recorded once is never re-read as
	// "never delivered" and reinstalled on a later run.
	if len(missed) == 0 && len(aurMissed) == 0 {
		if err := updater.SaveApplied(served, updater.StickyPresent(applied, served, installed)); err != nil {
			return warnRes(i18n.T("converged, but could not save the manifest: %v"), err)
		}
	}

	switch {
	case len(missed) > 0:
		return warnRes(i18n.T("manifest: %s did not land"), strings.Join(missed, ", ")).
			withFix("sudo pacman -Sy && sudo pacman -S %s", strings.Join(missed, " "))
	case len(aurMissed) > 0:
		return noteRes(i18n.T("manifest: %s await the AUR helper or a network"), strings.Join(aurMissed, ", ")).
			withFix("yay -S %s", strings.Join(aurMissed, " "))
	case len(landed) > 0:
		return fixedRes(i18n.T("delivered %s from the %s manifest"),
			strings.Join(landed, ", "), served.Release)
	case len(plan.Retired) > 0:
		return noteRes(i18n.T("%s retired by %s stay installed (delete them by hand to reclaim)"),
			strings.Join(plan.Retired, ", "), served.Release)
	case applied == nil:
		return okRes(i18n.T("baseline recorded against %s"), served.Release)
	default:
		return okRes(i18n.T("the box matches the %s manifest"), served.Release)
	}
}

// planSummary renders a check-only plan as one line, so `ryoku doctor --check`
// says what a real run would do without doing it.
func planSummary(p *updater.Plan) string {
	var parts []string
	if len(p.Install) > 0 {
		parts = append(parts, fmt.Sprintf("install %s", strings.Join(p.Install, ", ")))
	}
	if len(p.AUR) > 0 {
		parts = append(parts, fmt.Sprintf("aur %s", strings.Join(p.AUR, ", ")))
	}
	if len(p.UserGone) > 0 {
		parts = append(parts, fmt.Sprintf("%d user-removed stay removed", len(p.UserGone)))
	}
	if len(p.Retired) > 0 {
		parts = append(parts, fmt.Sprintf("report %d retired", len(p.Retired)))
	}
	if len(p.Untracked) > 0 {
		parts = append(parts, fmt.Sprintf("baseline: %d manifest names not installed", len(p.Untracked)))
	}
	if len(parts) == 0 {
		return "already converged to the served manifest"
	}
	return strings.Join(parts, "; ")
}

// manifestVerify is `ryoku verify`'s core: the box-vs-manifest diff, reported
// not applied. It lives here so the CLI command and the reconciler read the
// same three-way rule.
func manifestVerify() (updater.VerifyReport, *ryokumanifest.Manifest, error) {
	served, err := updater.FetchManifest()
	if err != nil {
		return updater.VerifyReport{}, nil, err
	}
	installed, err := updater.PacmanInstalled()
	if err != nil {
		return updater.VerifyReport{}, nil, err
	}
	explicit, err := updater.PacmanExplicit()
	if err != nil {
		return updater.VerifyReport{}, nil, err
	}
	applied := updater.LoadApplied()
	r := updater.Verify(served, installed, explicit, applied.PresentSet())
	if applied == nil {
		r.NotSaved = true
	}
	return r, &served, nil
}

// Verify answers `ryoku verify`: is this box the machine the channel's release
// describes? It reports the box-vs-manifest diff -- what the release never
// delivered, what the user removed, what the user added -- and changes nothing,
// so two machines can be compared line by line. --json emits the same report
// for scripts and the Hub.
func Verify(args []string) error {
	asJSON := false
	for _, a := range args {
		switch a {
		case "--json":
			asJSON = true
		case "-h", "--help":
			fmt.Println(i18n.T("Usage: ryoku verify [--json]\n\n  Report whether this box matches the release its channel serves."))
			return nil
		default:
			return fmt.Errorf(i18n.T("unknown argument: %s (try --help)"), a)
		}
	}
	r, _, err := manifestVerify()
	if err != nil {
		return err
	}
	if asJSON {
		b, err := json.MarshalIndent(r, "", "  ")
		if err != nil {
			return err
		}
		fmt.Println(string(b))
		return nil
	}
	rel := sys.ReadRelease()
	head := r.Release
	if rel.Name != "" {
		head = rel.Name + " " + head
	}
	fmt.Printf("  %s %s\n", sys.Bold(i18n.T("release")), sys.Dim(i18n.Tf("%s · %s · %s", head, shortCommit(r.Commit), r.Version)))
	drift := false
	if len(r.Missing) > 0 {
		drift = true
		fmt.Printf("  %s %s\n", sys.Amber("!"), i18n.Tf("not delivered (%d)", len(r.Missing)))
		names(r.Missing)
	}
	if len(r.UserGone) > 0 {
		fmt.Printf("  %s %s\n", sys.Dim("·"), i18n.Tf("kept removed (%d)", len(r.UserGone)))
		names(r.UserGone)
	}
	if len(r.Extra) > 0 {
		fmt.Printf("  %s %s\n", sys.Dim("·"), i18n.Tf("added by you (%d)", len(r.Extra)))
		names(r.Extra)
	}
	switch {
	case r.NotSaved:
		fmt.Printf("  %s %s\n", sys.Amber("›"), i18n.T("never reconciled against a manifest"))
		fmt.Println("      " + sys.Brand("↳ "+i18n.T("run `ryoku update` once to record the baseline")))
	case !drift && len(r.UserGone) == 0 && len(r.Extra) == 0:
		fmt.Println("  " + sys.Green("✓") + " " + i18n.T("this box is the machine the release describes"))
	case !drift:
		fmt.Println("  " + sys.Green("✓") + " " + i18n.T("nothing the release wants is missing"))
	default:
		fmt.Println("      " + sys.Brand("↳ "+i18n.T("run `ryoku update` to deliver what is missing")))
	}
	return nil
}

// names prints a package list as wrapped, dimmed continuation rows under its
// section glyph, the way doctor prints a finding's detail. A long list is
// capped so the report stays a summary; `--json` carries the whole set.
func names(pkgs []string) {
	shown := pkgs
	if len(pkgs) > 12 {
		shown = pkgs[:12]
	}
	line := strings.Join(shown, ", ")
	if extra := len(pkgs) - len(shown); extra > 0 {
		line += i18n.Tf(", and %d more", extra)
	}
	fmt.Println(sys.Dim(sys.Wrap(line, sys.TermWidth(), "      ")))
}

func shortCommit(sha string) string {
	if len(sha) > 8 {
		return sha[:8]
	}
	return sha
}
