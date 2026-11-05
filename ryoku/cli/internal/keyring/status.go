package keyring

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	i18n "ryoku-i18n"
)

// KeyringFile is the state of one on-disk keyring, as the Hub renders it.
type KeyringFile struct {
	Name   string `json:"name"`
	Role   string `json:"role"` // "default" or "login"
	Exists bool   `json:"exists"`
	Format string `json:"format"` // encrypted, plaintext, or absent
}

// Status is the full read-only picture: the configured mode and where it came
// from, the system wiring (PAM, autologin, daemon), and the keyring files that
// matter. The Hub decides from this whether a mode switch is blocked.
type Status struct {
	Mode          string        `json:"mode"`
	ModeSource    string        `json:"mode_source"` // "configured" or "inferred"
	PamPresent    bool          `json:"pam_present"`
	PamWired      bool          `json:"pam_wired"`
	PamDuplicates bool          `json:"pam_duplicates"`
	Autologin     bool          `json:"autologin"`
	DaemonAlive   bool          `json:"daemon_alive"`
	Keyrings      []KeyringFile `json:"keyrings"`
	Notes         []string      `json:"notes"`
}

// autologinConfigured scans the SDDM config for a non-empty [Autologin] User.
// Drop-ins under sddm.conf.d win, but any file naming a user counts.
func autologinConfigured(root string) bool {
	files := []string{filepath.Join(root, "sddm.conf")}
	if entries, err := os.ReadDir(filepath.Join(root, "sddm.conf.d")); err == nil {
		for _, e := range entries {
			if !e.IsDir() {
				files = append(files, filepath.Join(root, "sddm.conf.d", e.Name()))
			}
		}
	}
	for _, f := range files {
		if autologinUserSet(f) {
			return true
		}
	}
	return false
}

// autologinUserSet reports whether an SDDM ini file sets User= to a non-empty
// value inside the [Autologin] section.
func autologinUserSet(path string) bool {
	raw, err := os.ReadFile(path)
	if err != nil {
		return false
	}
	inSection := false
	for _, l := range strings.Split(string(raw), "\n") {
		t := strings.TrimSpace(l)
		if strings.HasPrefix(t, "[") && strings.HasSuffix(t, "]") {
			inSection = strings.EqualFold(t, "[Autologin]")
			continue
		}
		if !inSection {
			continue
		}
		k, v, ok := strings.Cut(t, "=")
		if ok && strings.EqualFold(strings.TrimSpace(k), "User") && strings.TrimSpace(v) != "" {
			return true
		}
	}
	return false
}

func sddmConfRoot() string {
	if r := os.Getenv("RYOKU_SDDM_CONF_ROOT"); r != "" {
		return r
	}
	return "/etc"
}

func gatherStatus() Status {
	pamContent, _ := os.ReadFile(pamFilePath())
	pam := pamPresent(string(pamContent))

	mode, configured := readConfig()
	source := "configured"
	if !configured {
		source = "inferred"
		// Ryoku's default is never prompt: an unconfigured box with no PAM wiring
		// defaults to never-ask (a blank, passwordless keyring) rather than ask
		// (which prompts on first use). `ryoku keyring init` records this and seeds
		// the blank keyring at first login. A PAM-wired box means the user chose
		// unlock-on-login, so honour that.
		if pam {
			mode = ModeUnlockOnLogin
		} else {
			mode = ModeNeverAsk
		}
	}

	defName := defaultKeyringName()
	defFmt := probeFormat(keyringFile(defName))
	kfs := []KeyringFile{
		{Name: defName, Role: "default", Exists: defFmt != fmtAbsent, Format: defFmt},
	}
	// login.keyring is the PAM-managed store; list it too unless it is already
	// the default (no need to name the same file twice).
	if defName != "login" {
		lf := probeFormat(keyringFile("login"))
		kfs = append(kfs, KeyringFile{Name: "login", Role: "login", Exists: lf != fmtAbsent, Format: lf})
	}

	st := Status{
		Mode:          mode,
		ModeSource:    source,
		PamPresent:    pam,
		PamWired:      pamWired(string(pamContent)),
		PamDuplicates: pamHasDuplicates(string(pamContent)),
		Autologin:     autologinConfigured(sddmConfRoot()),
		DaemonAlive:   daemonAlive(),
		Keyrings:      kfs,
	}
	st.Notes = statusNotes(st)
	return st
}

// statusNotes spells out the caveats a bare state cannot: whether an encrypted
// keyring will actually unlock, and the autologin conflict.
func statusNotes(st Status) []string {
	var notes []string
	if st.Mode == ModeUnlockOnLogin {
		for _, k := range st.Keyrings {
			if k.Role == "default" && k.Format == fmtEncrypted {
				notes = append(notes, fmt.Sprintf(i18n.T("the %q keyring is password-protected; it will unlock silently at next login only if its password is your login password"), k.Name))
			}
		}
		if st.Autologin {
			notes = append(notes, i18n.T("autologin is configured; unlock-on-login has no login password to reuse, so it cannot unlock silently -- use never-ask under autologin"))
		}
	}
	if st.Mode == ModeNeverAsk {
		for _, k := range st.Keyrings {
			if k.Role == "default" && k.Format == fmtEncrypted {
				notes = append(notes, fmt.Sprintf(i18n.T("the %q keyring is still password-protected; never-ask needs it blank -- convert it (with your current password) or start fresh"), k.Name))
			}
		}
	}
	return notes
}

func runStatus(args []string) error {
	jsonOut := false
	for _, a := range args {
		if a == "--json" {
			jsonOut = true
		} else {
			return fmt.Errorf(i18n.T("usage: ryoku keyring status [--json]"))
		}
	}
	st := gatherStatus()
	if jsonOut {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(st)
	}
	printStatus(st)
	return nil
}

func printStatus(st Status) {
	fmt.Printf(i18n.T("mode:      %s (%s)\n"), st.Mode, st.ModeSource)
	fmt.Printf(i18n.T("pam lines: %s\n"), yesno(st.PamPresent))
	fmt.Printf(i18n.T("autologin: %s\n"), yesno(st.Autologin))
	fmt.Printf(i18n.T("daemon:    %s\n"), map[bool]string{true: i18n.T("running"), false: i18n.T("not running")}[st.DaemonAlive])
	for _, k := range st.Keyrings {
		state := k.Format
		if !k.Exists {
			state = "absent"
		}
		fmt.Printf(i18n.T("keyring %-8s (%s): %s\n"), k.Name, k.Role, state)
	}
	for _, n := range st.Notes {
		fmt.Printf(i18n.T("note: %s\n"), n)
	}
}

func yesno(b bool) string {
	if b {
		return i18n.T("yes")
	}
	return i18n.T("no")
}
