package doctor

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"ryoku-cli/internal/host"

	i18n "ryoku-i18n"
)

var (
	kernelHeadersModulesDir       = "/usr/lib/modules"
	kernelHeadersReadDir          = os.ReadDir
	kernelHeadersReadFile         = os.ReadFile
	kernelHeadersPackageInstalled = func(name string) bool { return doctorPackageInstalled(name) }
)

func installedKernelHeaderPackages() ([]string, error) {
	entries, err := kernelHeadersReadDir(kernelHeadersModulesDir)
	if err != nil {
		return nil, err
	}
	seen := map[string]bool{}
	var headers []string
	for _, entry := range entries {
		data, err := kernelHeadersReadFile(filepath.Join(kernelHeadersModulesDir, entry.Name(), "pkgbase"))
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return nil, fmt.Errorf("read kernel metadata for %s: %w", entry.Name(), err)
		}
		kernel := strings.TrimSpace(string(data))
		if !strings.HasPrefix(kernel, "linux") || strings.ContainsAny(kernel, " \t\r\n/") ||
			!kernelHeadersPackageInstalled(kernel) {
			continue
		}
		header := kernel + "-headers"
		if !seen[header] {
			seen[header] = true
			headers = append(headers, header)
		}
	}
	sort.Strings(headers)
	return headers, nil
}

func reconcileKernelHeaders(checkOnly bool) recResult {
	if result, ok := pacmanHost(i18n.T("kernel header packages")); !ok {
		return result
	}
	headers, err := installedKernelHeaderPackages()
	if err != nil {
		return warnRes(i18n.T("could not inspect installed kernels: %v"), err)
	}
	if len(headers) == 0 {
		return noteRes(i18n.T("no installed Arch kernel packages were found"))
	}
	missing := make([]string, 0, len(headers))
	for _, name := range headers {
		if !kernelHeadersPackageInstalled(name) {
			missing = append(missing, name)
		}
	}
	if len(missing) == 0 {
		return okRes(i18n.T("every installed kernel has matching headers"))
	}
	advice := doctorInstallAdvice(missing...)
	if checkOnly {
		return wouldRes(i18n.T("matching kernel headers are missing: %s"), strings.Join(missing, ", ")).withFix("%s", advice)
	}
	if doctorPackageInstall(missing...) != host.ExitOK {
		return warnRes(i18n.T("could not install matching kernel headers: %s"), strings.Join(missing, ", ")).withFix("%s", advice)
	}
	return fixedRes(i18n.T("installed matching kernel headers: %s"), strings.Join(missing, ", "))
}
