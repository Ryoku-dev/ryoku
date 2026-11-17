package updater

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"ryoku-cli/internal/sys"

	i18n "ryoku-i18n"
)

var accountRepo = sys.ResolveRepo

const defaultRiceSlug = "default"

type AccountSeedCopy struct {
	Src, Dst string
}

type AccountSeedPlan struct {
	Copies []AccountSeedCopy
	// consumed by ryoku-session-intro once the shell is up
	QueueRice bool
}

func (p AccountSeedPlan) Empty() bool { return len(p.Copies) == 0 && !p.QueueRice }

type accountSeedState struct {
	root    bool
	files   []AccountSeedCopy
	present map[string]bool
	// shell.json means the account is in use and its look is the user's
	shellStore  bool
	activeRice  bool
	ricePending bool
	defaultRice bool
}

func planAccountSeed(s accountSeedState) AccountSeedPlan {
	var p AccountSeedPlan
	if s.root {
		return p
	}
	for _, f := range s.files {
		if !s.present[f.Dst] {
			p.Copies = append(p.Copies, f)
		}
	}
	p.QueueRice = s.defaultRice && !s.shellStore && !s.activeRice && !s.ricePending
	return p
}

func accountShareRoot() string {
	if v := filepath.Clean(os.Getenv("RYOKU_SHARE_ROOT")); filepath.IsAbs(v) {
		return v
	}
	return "/usr/share/ryoku"
}

func systemRicesDir() string {
	if v := os.Getenv("RYOKU_SYSTEM_RICES_DIR"); v != "" {
		return v
	}
	return filepath.Join(accountShareRoot(), "rices")
}

func defaultRiceMarker() string {
	return filepath.Join(sys.StateDir(), "default-rice-pending")
}

func accountSeedSource(repoRel, shareRel string) string {
	if repo := accountRepo(); repo != "" {
		if p := filepath.Join(repo, repoRel); sys.PathPresent(p) {
			return p
		}
	}
	if p := filepath.Join(accountShareRoot(), shareRel); sys.PathPresent(p) {
		return p
	}
	return ""
}

func dirSeedFiles(src, dst string) []AccountSeedCopy {
	if src == "" {
		return nil
	}
	entries, err := os.ReadDir(src)
	if err != nil {
		return nil
	}
	var out []AccountSeedCopy
	for _, e := range entries {
		if e.Type().IsRegular() {
			out = append(out, AccountSeedCopy{filepath.Join(src, e.Name()), filepath.Join(dst, e.Name())})
		}
	}
	return out
}

// accounts the installer never set up start from /etc/skel; only missing files are laid
func accountSeedNow() accountSeedState {
	home := sys.Home()
	s := accountSeedState{root: os.Geteuid() == 0, present: map[string]bool{}}
	s.files = append(s.files, dirSeedFiles(
		accountSeedSource("ryoku/assets/brand", "brand"),
		filepath.Join(home, ".local", "share", "ryoku", "assets", "brand"))...)
	s.files = append(s.files, dirSeedFiles(
		accountSeedSource("ryoku/assets/ryodecors", "ryodecors"),
		filepath.Join(sys.PicturesDir(), "ryodecors"))...)
	if src := accountSeedSource("ryoku/apps/npm/npmrc", "npmrc"); src != "" {
		s.files = append(s.files, AccountSeedCopy{src, filepath.Join(home, ".npmrc")})
	}
	for _, f := range s.files {
		s.present[f.Dst] = sys.PathPresent(f.Dst)
	}
	ryoku := filepath.Join(sys.ConfigHome(), "ryoku")
	s.shellStore = sys.PathPresent(filepath.Join(ryoku, "shell.json"))
	active, _ := os.ReadFile(filepath.Join(ryoku, "rices", ".active"))
	s.activeRice = strings.TrimSpace(string(active)) != ""
	s.ricePending = sys.PathPresent(defaultRiceMarker())
	s.defaultRice = sys.Exists(filepath.Join(systemRicesDir(), defaultRiceSlug, "rice.json"))
	return s
}

func PlanAccountSeed() AccountSeedPlan { return planAccountSeed(accountSeedNow()) }

func ApplyAccountSeed(p AccountSeedPlan) error {
	for _, c := range p.Copies {
		if sys.PathPresent(c.Dst) {
			continue
		}
		if err := sys.CopyFile(c.Src, c.Dst); err != nil {
			return fmt.Errorf(i18n.T("seed %s: %w"), c.Dst, err)
		}
	}
	if !p.QueueRice {
		return nil
	}
	marker := defaultRiceMarker()
	if err := os.MkdirAll(filepath.Dir(marker), 0o755); err != nil {
		return fmt.Errorf(i18n.T("queue the default rice: %w"), err)
	}
	if err := os.WriteFile(marker, []byte(defaultRiceSlug+"\n"), 0o644); err != nil {
		return fmt.Errorf(i18n.T("queue the default rice: %w"), err)
	}
	return nil
}

// a login must not wait on art files, so a failure only warns
func seedAccount() {
	plan := PlanAccountSeed()
	if plan.Empty() {
		return
	}
	if err := ApplyAccountSeed(plan); err != nil {
		fmt.Fprintf(os.Stderr, i18n.T("could not set up this account's Ryoku files: %v\n"), err)
		return
	}
	if len(plan.Copies) > 0 {
		fmt.Printf(i18n.T("seeded %d Ryoku file(s) into this account (brand art, decor art, npm prefix)\n"), len(plan.Copies))
	}
	if plan.QueueRice {
		fmt.Println(i18n.T("the default rice will be applied when the desktop starts"))
	}
}
