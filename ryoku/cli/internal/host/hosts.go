package host

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/netip"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"ryoku-cli/internal/profile"
	i18n "ryoku-i18n"
)

const (
	hostsBlockStart = "# >>> lab hosts (ryoku-host) >>>"
	hostsBlockEnd   = "# <<< lab hosts <<<"
)

var (
	profileHostsPath = "/etc/hosts"
	profileRootPath  = profile.SystemRoot
)

type labHostEntry struct {
	IP    string   `json:"ip"`
	Names []string `json:"names"`
}

type labHostsResult struct {
	Entries  []labHostEntry `json:"entries"`
	Suffixes []string       `json:"suffixes"`
}

func (a *App) Hosts(args []string) int {
	if len(args) == 0 {
		return a.profileFail(i18n.T("Choose a hosts action."))
	}
	if args[0] == "list" {
		if len(args) != 1 {
			return a.profileFail(i18n.T("hosts list does not accept arguments."))
		}
		return a.hostsList()
	}
	if a.cfg.UID != 0 {
		return a.profileFail(i18n.T("Administrator authentication is required."))
	}
	suffixes, err := appliedHostSuffixes(profileRootPath)
	if err != nil {
		return a.profileFail(fmt.Sprintf(i18n.T("Read applied profiles: %v"), err))
	}
	data, mode, err := readHostsFile(profileHostsPath)
	if err != nil {
		return a.profileFail(fmt.Sprintf(i18n.T("Read lab hosts: %v"), err))
	}
	entries, err := parseManagedHosts(data)
	if err != nil {
		return a.profileFail(err.Error())
	}
	switch args[0] {
	case "add":
		if len(args) < 3 {
			return a.profileFail(i18n.T("Use hosts add <ip> <name>..."))
		}
		if err := addManagedHost(&entries, args[1], args[2:], suffixes); err != nil {
			return a.profileFail(err.Error())
		}
	case "remove":
		if len(args) < 2 {
			return a.profileFail(i18n.T("Use hosts remove <name>..."))
		}
		if err := validateHostNames(args[1:], suffixes); err != nil {
			return a.profileFail(err.Error())
		}
		entries = removeManagedHostNames(entries, args[1:])
	case "clear":
		if len(args) != 1 {
			return a.profileFail(i18n.T("hosts clear does not accept arguments."))
		}
		entries = nil
	default:
		return a.profileFail(i18n.T("Unknown hosts action."))
	}
	out, err := rewriteManagedHosts(data, entries)
	if err != nil {
		return a.profileFail(err.Error())
	}
	if err := writeHostsAtomic(profileHostsPath, out, mode); err != nil {
		return a.profileFail(fmt.Sprintf(i18n.T("Write lab hosts: %v"), err))
	}
	return a.profileJSON(map[string]bool{"ok": true})
}

func (a *App) hostsList() int {
	data, _, err := readHostsFile(profileHostsPath)
	if err != nil {
		return a.profileFail(fmt.Sprintf(i18n.T("Read lab hosts: %v"), err))
	}
	entries, err := parseManagedHosts(data)
	if err != nil {
		return a.profileFail(err.Error())
	}
	suffixes, err := appliedHostSuffixes(profileRootPath)
	if err != nil {
		return a.profileFail(fmt.Sprintf(i18n.T("Read applied profiles: %v"), err))
	}
	return a.profileJSON(labHostsResult{Entries: entries, Suffixes: suffixes})
}

func readHostsFile(path string) ([]byte, os.FileMode, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, 0, err
	}
	info, err := os.Stat(path)
	if err != nil {
		return nil, 0, err
	}
	return data, info.Mode().Perm(), nil
}

func appliedHostSuffixes(root string) ([]string, error) {
	receipts, err := profile.ReadSystemReceipts(root)
	if err != nil {
		return nil, err
	}
	return hostSuffixesFromReceipts(receipts), nil
}

func hostSuffixesFromReceipts(receipts []profile.SystemReceipt) []string {
	set := map[string]bool{}
	for _, receipt := range receipts {
		if receipt.Hosts == nil {
			continue
		}
		for _, suffix := range receipt.Hosts.Suffixes {
			set[suffix] = true
		}
	}
	out := make([]string, 0, len(set))
	for suffix := range set {
		out = append(out, suffix)
	}
	sort.Strings(out)
	return out
}

func parseManagedHosts(data []byte) ([]labHostEntry, error) {
	_, body, _, found, err := splitHostsBlock(data)
	if err != nil || !found {
		return []labHostEntry{}, err
	}
	entries := []labHostEntry{}
	for _, raw := range strings.Split(string(body), "\n") {
		line := strings.TrimSpace(raw)
		if line == "" {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 2 {
			return nil, fmt.Errorf(i18n.T("The managed hosts block is invalid."))
		}
		addr, err := netip.ParseAddr(fields[0])
		if err != nil || !allowedLabAddress(addr) {
			return nil, fmt.Errorf(i18n.T("The managed hosts block contains an invalid address."))
		}
		seen := map[string]bool{}
		names := make([]string, 0, len(fields)-1)
		for _, name := range fields[1:] {
			if !validDNSName(name) {
				return nil, fmt.Errorf(i18n.T("The managed hosts block contains an invalid name."))
			}
			if !seen[name] {
				seen[name] = true
				names = append(names, name)
			}
		}
		entries = append(entries, labHostEntry{IP: addr.String(), Names: names})
	}
	if len(entries) > 256 {
		return nil, fmt.Errorf(i18n.T("The managed hosts block has more than 256 entries."))
	}
	return entries, nil
}

func splitHostsBlock(data []byte) (before, body, after []byte, found bool, err error) {
	startNeedle := []byte(hostsBlockStart)
	endNeedle := []byte(hostsBlockEnd)
	start := bytes.Index(data, startNeedle)
	end := bytes.Index(data, endNeedle)
	if start < 0 && end < 0 {
		return data, nil, nil, false, nil
	}
	if start < 0 || end < 0 || end < start || bytes.Index(data[start+len(startNeedle):], startNeedle) >= 0 || bytes.Index(data[end+len(endNeedle):], endNeedle) >= 0 {
		return nil, nil, nil, false, fmt.Errorf(i18n.T("The managed hosts block markers are invalid."))
	}
	lineStart := start
	if lineStart > 0 && data[lineStart-1] != '\n' {
		return nil, nil, nil, false, fmt.Errorf(i18n.T("The managed hosts block start is not on its own line."))
	}
	startLineEnd := start + len(startNeedle)
	if startLineEnd < len(data) && data[startLineEnd] == '\r' {
		startLineEnd++
	}
	if startLineEnd < len(data) && data[startLineEnd] == '\n' {
		startLineEnd++
	} else if startLineEnd != len(data) {
		return nil, nil, nil, false, fmt.Errorf(i18n.T("The managed hosts block start is not on its own line."))
	}
	if end > 0 && data[end-1] != '\n' {
		return nil, nil, nil, false, fmt.Errorf(i18n.T("The managed hosts block end is not on its own line."))
	}
	endLineEnd := end + len(endNeedle)
	if endLineEnd < len(data) && data[endLineEnd] == '\r' {
		endLineEnd++
	}
	if endLineEnd < len(data) && data[endLineEnd] == '\n' {
		endLineEnd++
	} else if endLineEnd != len(data) {
		return nil, nil, nil, false, fmt.Errorf(i18n.T("The managed hosts block end is not on its own line."))
	}
	return data[:lineStart], data[startLineEnd:end], data[endLineEnd:], true, nil
}

func rewriteManagedHosts(data []byte, entries []labHostEntry) ([]byte, error) {
	before, _, after, found, err := splitHostsBlock(data)
	if err != nil {
		return nil, err
	}
	if !found {
		before = data
		after = nil
		if len(before) > 0 && before[len(before)-1] != '\n' {
			before = append(append([]byte(nil), before...), '\n')
		}
	}
	var block strings.Builder
	block.WriteString(hostsBlockStart)
	block.WriteByte('\n')
	for _, entry := range entries {
		if len(entry.Names) == 0 {
			continue
		}
		block.WriteString(entry.IP)
		block.WriteByte('\t')
		block.WriteString(strings.Join(entry.Names, " "))
		block.WriteByte('\n')
	}
	block.WriteString(hostsBlockEnd)
	block.WriteByte('\n')
	out := make([]byte, 0, len(before)+block.Len()+len(after))
	out = append(out, before...)
	out = append(out, block.String()...)
	out = append(out, after...)
	return out, nil
}

func allowedLabAddress(addr netip.Addr) bool {
	prefixes := [...]netip.Prefix{
		netip.MustParsePrefix("10.0.0.0/8"),
		netip.MustParsePrefix("172.16.0.0/12"),
		netip.MustParsePrefix("192.168.0.0/16"),
		netip.MustParsePrefix("100.64.0.0/10"),
		netip.MustParsePrefix("fc00::/7"),
	}
	for _, prefix := range prefixes {
		if prefix.Contains(addr) {
			return true
		}
	}
	return false
}

func validDNSName(name string) bool {
	if name == "" || name != strings.ToLower(name) || len(name) > 253 || strings.HasSuffix(name, ".") {
		return false
	}
	labels := strings.Split(name, ".")
	if len(labels) < 2 {
		return false
	}
	for _, label := range labels {
		if label == "" || len(label) > 63 || label[0] == '-' || label[len(label)-1] == '-' {
			return false
		}
		for _, r := range label {
			if r != '-' && (r < 'a' || r > 'z') && (r < '0' || r > '9') {
				return false
			}
		}
	}
	return true
}

func validateHostNames(names, suffixes []string) error {
	allowed := map[string]bool{}
	for _, suffix := range suffixes {
		allowed[suffix] = true
	}
	seen := map[string]bool{}
	for _, name := range names {
		if seen[name] {
			return fmt.Errorf(i18n.T("Host names must not be repeated."))
		}
		seen[name] = true
		if !validDNSName(name) {
			return fmt.Errorf(i18n.T("Host names must be lowercase DNS names."))
		}
		_, suffix, ok := strings.Cut(name, ".")
		if !ok {
			return fmt.Errorf(i18n.T("Host names must end in a suffix declared by an applied profile."))
		}
		parts := strings.Split(name, ".")
		suffix = parts[len(parts)-1]
		if !allowed[suffix] {
			return fmt.Errorf(i18n.T("Host names must end in a suffix declared by an applied profile."))
		}
	}
	return nil
}

func addManagedHost(entries *[]labHostEntry, ip string, names, suffixes []string) error {
	addr, err := netip.ParseAddr(ip)
	if err != nil || !allowedLabAddress(addr) {
		return fmt.Errorf(i18n.T("The address must be private, CGNAT or ULA."))
	}
	if err := validateHostNames(names, suffixes); err != nil {
		return err
	}
	without := removeManagedHostNames(*entries, names)
	canonical := addr.String()
	for index := range without {
		if without[index].IP == canonical {
			without[index].Names = append(without[index].Names, names...)
			*entries = without
			return nil
		}
	}
	if len(without) >= 256 {
		return fmt.Errorf(i18n.T("Lab hosts are limited to 256 entries."))
	}
	*entries = append(without, labHostEntry{IP: canonical, Names: append([]string(nil), names...)})
	return nil
}

func removeManagedHostNames(entries []labHostEntry, names []string) []labHostEntry {
	remove := map[string]bool{}
	for _, name := range names {
		remove[name] = true
	}
	out := make([]labHostEntry, 0, len(entries))
	for _, entry := range entries {
		kept := make([]string, 0, len(entry.Names))
		for _, name := range entry.Names {
			if !remove[name] {
				kept = append(kept, name)
			}
		}
		if len(kept) > 0 {
			entry.Names = kept
			out = append(out, entry)
		}
	}
	return out
}

func pruneManagedHosts(path string, suffixes []string) error {
	data, mode, err := readHostsFile(path)
	if err != nil {
		return err
	}
	entries, err := parseManagedHosts(data)
	if err != nil {
		return err
	}
	allowed := map[string]bool{}
	for _, suffix := range suffixes {
		allowed[suffix] = true
	}
	keptEntries := make([]labHostEntry, 0, len(entries))
	for _, entry := range entries {
		keptNames := make([]string, 0, len(entry.Names))
		for _, name := range entry.Names {
			parts := strings.Split(name, ".")
			if allowed[parts[len(parts)-1]] {
				keptNames = append(keptNames, name)
			}
		}
		if len(keptNames) > 0 {
			entry.Names = keptNames
			keptEntries = append(keptEntries, entry)
		}
	}
	if len(keptEntries) == 0 && len(suffixes) == 0 {
		before, _, after, found, err := splitHostsBlock(data)
		if err != nil {
			return err
		}
		if found {
			out := append(append([]byte(nil), before...), after...)
			return writeHostsAtomic(path, out, mode)
		}
		return nil
	}
	out, err := rewriteManagedHosts(data, keptEntries)
	if err != nil {
		return err
	}
	return writeHostsAtomic(path, out, mode)
}

func writeHostsAtomic(path string, data []byte, mode os.FileMode) error {
	dir := filepath.Dir(path)
	file, err := os.CreateTemp(dir, ".hosts-*")
	if err != nil {
		return err
	}
	temp := file.Name()
	defer os.Remove(temp)
	if err := file.Chmod(mode); err != nil {
		file.Close()
		return err
	}
	if _, err := file.Write(data); err != nil {
		file.Close()
		return err
	}
	if err := file.Sync(); err != nil {
		file.Close()
		return err
	}
	if err := file.Close(); err != nil {
		return err
	}
	return os.Rename(temp, path)
}

func (a *App) profileJSON(value any) int {
	if err := json.NewEncoder(a.cfg.Stdout).Encode(value); err != nil {
		return a.failf("%v", err)
	}
	return ExitOK
}

func (a *App) profileFail(message string) int {
	_ = json.NewEncoder(a.cfg.Stdout).Encode(map[string]string{"error": message})
	return ExitFailure
}
