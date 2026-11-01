package securitykey

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"ryoku-cli/internal/sys"
	i18n "ryoku-i18n"
)

type Credential struct {
	ID    string `json:"id"`
	Label string `json:"label"`
}

type Capabilities struct {
	Compatible       bool `json:"compatible"`
	ResidentKey      bool `json:"residentKey"`
	ClientPIN        bool `json:"clientPin"`
	UserPresence     bool `json:"userPresence"`
	UserVerification bool `json:"userVerification"`
	AlwaysUV         bool `json:"alwaysUv"`
}

type Status struct {
	Supported        bool         `json:"supported"`
	DevicePresent    bool         `json:"devicePresent"`
	DeviceName       string       `json:"deviceName"`
	Capabilities     Capabilities `json:"capabilities"`
	Enrolled         bool         `json:"enrolled"`
	Credentials      int          `json:"credentials"`
	CredentialIDs    []Credential `json:"credentialIds"`
	Sudo             bool         `json:"sudo"`
	Polkit           bool         `json:"polkit"`
	Login            bool         `json:"login"`
	Lock             bool         `json:"lock"`
	LockSupported    bool         `json:"lockSupported"`
	LockReason       string       `json:"lockReason,omitempty"`
	LockPIN          bool         `json:"lockPin"`
	AuthMode         string       `json:"authMode"`
	TouchRequired    bool         `json:"touchRequired"`
	PinVerification  bool         `json:"pinVerification"`
	UserVerification bool         `json:"userVerification"`
}

func gatherStatus() Status {
	a, _ := loadAuthFile()
	p := readPolicy()
	present, name, caps := probeDevice()
	st := Status{
		Supported:        sys.Has("pamu2fcfg") || fakeFIDO(),
		DevicePresent:    present,
		DeviceName:       name,
		Capabilities:     caps,
		Enrolled:         len(a.creds) > 0,
		Credentials:      len(a.creds),
		Sudo:             pamEnabled(TargetSudo),
		Polkit:           pamEnabled(TargetPolkit),
		Login:            pamEnabled(TargetLogin),
		Lock:             lockEnabled(),
		LockSupported:    lockUnsupportedReason(p) == "",
		LockReason:       lockUnsupportedReason(p),
		LockPIN:          lockAsksPIN(p, a.creds),
		AuthMode:         p.Mode,
		TouchRequired:    p.TouchRequired,
		PinVerification:  p.PinVerification,
		UserVerification: p.UserVerification,
	}
	for i := range a.creds {
		st.CredentialIDs = append(st.CredentialIDs, Credential{ID: fmt.Sprintf("%d", i+1), Label: fmt.Sprintf(i18n.T("Security key %d"), i+1)})
	}
	return st
}

func probeDevice() (bool, string, Capabilities) {
	if fakeFIDO() {
		return true, "Fake YubiKey 5 NFC", Capabilities{Compatible: true, ResidentKey: true, ClientPIN: true, UserPresence: true, UserVerification: true}
	}
	for _, probe := range []struct {
		name string
		args []string
	}{
		{name: "systemd-cryptenroll", args: []string{"--fido2-device=list"}},
		{name: "fido2-token", args: []string{"-L"}},
	} {
		if !sys.Has(probe.name) {
			continue
		}
		out, err := exec.Command(probe.name, probe.args...).CombinedOutput()
		if err != nil {
			continue
		}
		if name, caps, ok := parseProbeOutput(string(out)); ok {
			return true, name, caps
		}
	}
	return false, "", Capabilities{}
}

func parseProbeOutput(out string) (string, Capabilities, bool) {
	for _, line := range strings.Split(strings.TrimSpace(out), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || probeNoise(line) {
			continue
		}
		return line, parseCapabilities(line), true
	}
	return "", Capabilities{}, false
}

func parseCapabilities(line string) Capabilities {
	fields := strings.Fields(line)
	if len(fields) < 7 {
		return Capabilities{}
	}
	vals := fields[len(fields)-6:]
	return Capabilities{
		Compatible:       parseCapability(vals[0]),
		ResidentKey:      parseCapability(vals[1]),
		ClientPIN:        parseCapability(vals[2]),
		UserPresence:     parseCapability(vals[3]),
		UserVerification: parseCapability(vals[4]),
		AlwaysUV:         parseCapability(vals[5]),
	}
}

func parseCapability(v string) bool {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "✓", "yes", "true", "1", "y", "on":
		return true
	default:
		return false
	}
}

func probeNoise(line string) bool {
	low := strings.ToLower(strings.TrimSpace(line))
	if low == "" {
		return true
	}
	if strings.Contains(low, "no fido2 devices found") || strings.Contains(low, "no fido devices found") || strings.Contains(low, "no devices found") {
		return true
	}
	if strings.HasPrefix(low, "path ") && strings.Contains(low, "manufacturer") && strings.Contains(low, "product") {
		return true
	}
	if strings.HasPrefix(low, "path\t") && strings.Contains(low, "manufacturer") && strings.Contains(low, "product") {
		return true
	}
	return false
}

func runStatus(args []string) error {
	jsonOut := false
	for _, a := range args {
		if a == "--json" {
			jsonOut = true
		} else {
			return fmt.Errorf(i18n.T("usage: ryoku security-key status [--json]"))
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
	fmt.Printf(i18n.T("supported:   %s\n"), yesno(st.Supported))
	fmt.Printf(i18n.T("device:      %s\n"), map[bool]string{true: st.DeviceName, false: i18n.T("not detected")}[st.DevicePresent])
	fmt.Printf(i18n.T("enrolled:    %s (%d credential%s)\n"), yesno(st.Enrolled), st.Credentials, plural(st.Credentials))
	fmt.Printf(i18n.T("mode:        %s\n"), st.AuthMode)
	fmt.Printf(i18n.T("touch:       %s\n"), yesno(st.TouchRequired))
	fmt.Printf(i18n.T("pin verify:  %s\n"), yesno(st.PinVerification))
	fmt.Printf(i18n.T("user verify: %s\n"), yesno(st.UserVerification))
	fmt.Printf(i18n.T("sudo:        %s\n"), yesno(st.Sudo))
	fmt.Printf(i18n.T("polkit:      %s\n"), yesno(st.Polkit))
	fmt.Printf(i18n.T("login:       %s\n"), yesno(st.Login))
	fmt.Printf(i18n.T("lockscreen:  %s\n"), lockLine(st))
}

func lockLine(st Status) string {
	if !st.LockSupported {
		return fmt.Sprintf(i18n.T("unavailable (%s)"), st.LockReason)
	}
	if !st.Lock {
		return i18n.T("no")
	}
	switch {
	case st.LockPIN && st.TouchRequired:
		return i18n.T("yes (PIN, then touch; the PIN needs a lock skin that shows key prompts, such as Orbital)")
	case st.LockPIN:
		return i18n.T("yes (PIN; the PIN needs a lock skin that shows key prompts, such as Orbital)")
	case st.TouchRequired:
		return i18n.T("yes (touch)")
	}
	return i18n.T("yes")
}

func yesno(b bool) string {
	if b {
		return i18n.T("yes")
	}
	return i18n.T("no")
}

func plural(n int) string {
	if n == 1 {
		return ""
	}
	return "s"
}
