package profile

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"golang.org/x/sys/unix"
)

func Check(home string, m *Manifest, r *AccountReceipt, quick bool) CheckResult {
	result := CheckResult{OK: true, At: time.Now().UTC(), Failures: []Finding{}, Warnings: []Finding{}}
	v := Vars{Home: home, User: filepath.Base(home), ID: m.ID}
	pathValue := ComposePATH(m.PathEntries(v), home)
	packages := selectedPackages(home, m)
	selected := stringSet(packages)
	for _, component := range m.Components {
		if !selected[component.Name] {
			continue
		}
		if filepath.IsAbs(component.Detect) {
			if _, err := os.Stat(component.Detect); err != nil {
				result.fail("path", component.Detect)
			}
		} else if findOnPath(component.Detect, pathValue) == "" {
			result.fail("tool", component.Detect)
		}
	}
	if r == nil {
		result.fail("file", "account receipt is missing")
		return result
	}
	for _, managed := range r.Files {
		path := filepath.Join(home, filepath.FromSlash(managed.Path))
		if digest, err := hashFile(path); err != nil {
			result.fail("file", fmt.Sprintf("%s: %v", managed.Path, err))
		} else if digest != managed.SHA256 {
			result.warn("file", managed.Path+" was changed")
		}
	}
	for _, env := range DesktopEnv(m, v) {
		if got := os.Getenv(env.Key); got != env.Value {
			result.warn("env", fmt.Sprintf("%s is not active in this session", env.Key))
		}
	}
	if _, err := os.Stat(filepath.Join(home, ".config", "ryoku", "rices", m.Rice, "rice.json")); err != nil {
		result.fail("rice", m.Rice)
	}
	for _, link := range m.Wordlists.Links {
		if _, err := os.Stat(filepath.Join(WordlistsRoot, filepath.FromSlash(link.Name))); err != nil {
			result.fail("wordlists", link.Name)
		}
	}
	for _, extract := range m.Wordlists.Extract {
		if _, err := os.Stat(filepath.Join(WordlistsRoot, filepath.FromSlash(extract.Name))); err != nil {
			result.fail("wordlists", extract.Name)
		}
	}
	for _, capability := range m.Capabilities {
		selectedTool := false
		for _, component := range m.Components {
			if selected[component.Name] && filepath.Base(component.Detect) == capability.Tool {
				selectedTool = true
				break
			}
		}
		if !selectedTool {
			continue
		}
		path := filepath.Join(ProfileBinDir(m.ID), capability.Tool)
		buf := make([]byte, 256)
		n, err := unix.Getxattr(path, "security.capability", buf)
		if err != nil || !capabilityXattrMatches(buf[:maxInt(n, 0)], capability.Caps) {
			result.fail("capability", capability.Tool)
		}
	}
	if !quick {
		for _, name := range packages {
			if err := exec.Command("pacman", "-Q", name).Run(); err != nil {
				result.fail("package", name)
			}
		}
	}
	result.OK = len(result.Failures) == 0
	return result
}

func (r *CheckResult) fail(kind, detail string) {
	r.Failures = append(r.Failures, Finding{Kind: kind, Detail: detail})
	r.OK = false
}
func (r *CheckResult) warn(kind, detail string) {
	r.Warnings = append(r.Warnings, Finding{Kind: kind, Detail: detail})
}

func selectedPackages(home string, m *Manifest) []string {
	receipt, err := ReadSystemReceipt(SystemRoot, m.ID, filepath.Base(home))
	if err == nil {
		comps, extras, targetErr := m.Targets(receipt.Selection)
		if targetErr == nil {
			var names []string
			for _, component := range comps {
				names = append(names, component.Name)
			}
			return append(names, extras...)
		}
	}
	var names []string
	for _, component := range m.Components {
		if component.Tier == "core" {
			names = append(names, component.Name)
		}
	}
	return names
}

func findOnPath(name, pathValue string) string {
	if strings.ContainsRune(name, '/') {
		return ""
	}
	for _, dir := range strings.Split(pathValue, ":") {
		path := filepath.Join(dir, name)
		if info, err := os.Stat(path); err == nil && info.Mode().IsRegular() && info.Mode().Perm()&0o111 != 0 {
			return path
		}
	}
	return ""
}

func capabilityXattrMatches(raw []byte, spec string) bool {
	if len(raw) < 20 {
		return false
	}
	magic := binary.LittleEndian.Uint32(raw[:4])
	revision := magic & 0xff000000
	if revision != 0x02000000 && revision != 0x03000000 {
		return false
	}
	names, flags, ok := strings.Cut(spec, "=")
	if !ok {
		return false
	}
	var low, high uint32
	bits := map[string]uint{"cap_net_bind_service": 10, "cap_net_admin": 12, "cap_net_raw": 13}
	for _, name := range strings.Split(names, ",") {
		bit, exists := bits[name]
		if !exists {
			return false
		}
		if bit < 32 {
			low |= 1 << bit
		} else {
			high |= 1 << (bit - 32)
		}
	}
	permittedLow := binary.LittleEndian.Uint32(raw[4:8])
	inheritableLow := binary.LittleEndian.Uint32(raw[8:12])
	permittedHigh := binary.LittleEndian.Uint32(raw[12:16])
	inheritableHigh := binary.LittleEndian.Uint32(raw[16:20])
	if permittedLow != low || permittedHigh != high {
		return false
	}
	if strings.Contains(flags, "i") && (inheritableLow != low || inheritableHigh != high) {
		return false
	}
	if !strings.Contains(flags, "i") && (inheritableLow != 0 || inheritableHigh != 0) {
		return false
	}
	return strings.Contains(flags, "e") == (magic&1 != 0)
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func WriteCheck(home string, c CheckResult) error {
	body, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}
	body = append(body, '\n')
	return writeHomeFile(home, ".local/state/ryoku/profile/check.json", body, 0o600)
}
