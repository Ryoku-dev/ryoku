package doctor

import (
	"errors"
	"os"
	"path/filepath"

	"ryoku-cli/internal/sys"

	i18n "ryoku-i18n"
)

var zshrcTemplatePath = "/usr/share/ryoku/zsh/zshrc"

func reconcileZshrc(checkOnly bool) recResult {
	if filepath.Base(accountLoginShell()) != "zsh" {
		return okRes(i18n.T("the account login shell does not need a .zshrc"))
	}

	path := filepath.Join(sys.Home(), ".zshrc")
	if _, err := os.Lstat(path); err == nil {
		return okRes(i18n.T("the zsh login config already exists"))
	} else if !errors.Is(err, os.ErrNotExist) {
		return failRes(i18n.T("could not inspect the zsh login config: %v"), err)
	}

	template, err := os.ReadFile(zshrcTemplatePath)
	if errors.Is(err, os.ErrNotExist) {
		return warnRes(i18n.T("the packaged zsh login template is missing")).
			withFix(i18n.T("ryoku update restores the template, then ryoku doctor seeds ~/.zshrc"))
	}
	if err != nil {
		return failRes(i18n.T("could not read the packaged zsh login template: %v"), err)
	}
	if checkOnly {
		return wouldRes(i18n.T("the zsh account has no .zshrc")).
			withFix(i18n.T("ryoku doctor seeds it from the packaged Ryoku template"))
	}

	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
	if errors.Is(err, os.ErrExist) {
		return okRes(i18n.T("the zsh login config already exists"))
	}
	if err != nil {
		return failRes(i18n.T("could not create the zsh login config: %v"), err)
	}
	if _, err = file.Write(template); err != nil {
		_ = file.Close()
		_ = os.Remove(path)
		return failRes(i18n.T("could not seed the zsh login config: %v"), err)
	}
	if err = file.Close(); err != nil {
		_ = os.Remove(path)
		return failRes(i18n.T("could not finish the zsh login config: %v"), err)
	}
	return fixedRes(i18n.T("seeded the missing zsh login config from the packaged Ryoku template"))
}
