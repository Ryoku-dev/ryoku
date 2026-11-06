package doctor

import (
	"os"
	"strings"

	"ryoku-cli/internal/profile"
	"ryoku-cli/internal/sys"

	i18n "ryoku-i18n"
)

var (
	readBlackArchReceipts = func() ([]profile.SystemReceipt, error) {
		return profile.ReadSystemReceipts(profile.SystemRoot)
	}
	readBlackArchPacmanConf = func() ([]byte, error) {
		return os.ReadFile("/etc/pacman.conf")
	}
	enableBlackArchRepo = func() error {
		return sys.Run("sudo", "-n", "/usr/bin/ryoku-pkg-blackarch", "enable")
	}
)

func receiptsClaimRepo(receipts []profile.SystemReceipt, id string) bool {
	for _, receipt := range receipts {
		for _, repo := range receipt.Repos {
			if repo.ID == id {
				return true
			}
		}
	}
	return false
}

func pacmanConfigHasRepo(contents []byte, id string) bool {
	want := "[" + id + "]"
	for _, line := range strings.Split(string(contents), "\n") {
		if strings.TrimSpace(line) == want {
			return true
		}
	}
	return false
}

func reconcileBlackArchRepo(checkOnly bool) recResult {
	if result, applicable := pacmanHost(i18n.T("BlackArch repository")); !applicable {
		return result
	}

	receipts, err := readBlackArchReceipts()
	if err != nil {
		return warnRes(i18n.T("could not read profile receipts to check the BlackArch repository: %v"), err).
			withFix(i18n.T("run `sudo /usr/bin/ryoku-pkg-blackarch status`, then run `ryoku doctor` again"))
	}
	if !receiptsClaimRepo(receipts, "blackarch") {
		return okRes(i18n.T("no installed profile requires the BlackArch repository"))
	}

	conf, err := readBlackArchPacmanConf()
	if err != nil {
		return warnRes(i18n.T("an installed profile requires BlackArch, but pacman.conf could not be read: %v"), err).
			withFix(i18n.T("sudo /usr/bin/ryoku-pkg-blackarch enable"))
	}
	if pacmanConfigHasRepo(conf, "blackarch") {
		return okRes(i18n.T("installed profile receipts and the BlackArch repository agree"))
	}
	if checkOnly {
		return wouldRes(i18n.T("an installed profile requires BlackArch, but [blackarch] is missing from pacman.conf")).
			withFix(i18n.T("sudo /usr/bin/ryoku-pkg-blackarch enable"))
	}
	if err := enableBlackArchRepo(); err != nil {
		return warnRes(i18n.T("an installed profile requires BlackArch, but the repository could not be restored: %v"), err).
			withFix(i18n.T("sudo /usr/bin/ryoku-pkg-blackarch enable"))
	}
	return fixedRes(i18n.T("restored the BlackArch repository required by an installed profile"))
}
