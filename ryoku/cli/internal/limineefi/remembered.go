package limineefi

import (
	"fmt"
	"os"
	"path/filepath"

	"golang.org/x/sys/unix"

	i18n "ryoku-i18n"
)

const EFIVarsDir = "/sys/firmware/efi/efivars"

// limine 12.9.5 moved it from vendor guid 513ee0d0 to 03884334; older menus still hold the old one
const lastBootedEntryGlob = "LimineLastBootedEntry-*"

const fsImmutableFlag = 0x10

func lastBootedEntryPaths(efivarsDir string) ([]string, error) {
	return filepath.Glob(filepath.Join(efivarsDir, lastBootedEntryGlob))
}

// ClearLastBootedEntry removes Limine's remembered menu selection. efivarfs
// commonly marks variables immutable, so callers must run this as root.
func ClearLastBootedEntry(efivarsDir string) error {
	paths, err := lastBootedEntryPaths(efivarsDir)
	if err != nil {
		return err
	}
	for _, path := range paths {
		if err := clearVariable(path); err != nil {
			return err
		}
	}
	return nil
}

func clearVariable(path string) error {
	file, err := os.Open(path)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	flags, ioctlErr := unix.IoctlGetInt(int(file.Fd()), unix.FS_IOC_GETFLAGS)
	if ioctlErr == nil && flags&fsImmutableFlag != 0 {
		ioctlErr = unix.IoctlSetPointerInt(int(file.Fd()), unix.FS_IOC_SETFLAGS, flags&^fsImmutableFlag)
	}
	closeErr := file.Close()
	if ioctlErr != nil {
		return fmt.Errorf(i18n.T("clear Limine remembered-entry attributes: %w"), ioctlErr)
	}
	if closeErr != nil {
		return closeErr
	}
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf(i18n.T("clear Limine remembered entry: %w"), err)
	}
	return nil
}

// ClearLastBootedEntryWithSudo is the regular-user doctor path. chattr performs
// the same immutable-flag removal as ClearLastBootedEntry inside the privileged
// process before rm removes the root-owned EFI variable.
func ClearLastBootedEntryWithSudo(efivarsDir string, sudo func(...string) error) error {
	paths, err := lastBootedEntryPaths(efivarsDir)
	if err != nil {
		return err
	}
	for _, path := range paths {
		if err := sudo("chattr", "-i", path); err != nil {
			return fmt.Errorf(i18n.T("clear Limine remembered-entry attributes: %w"), err)
		}
		if err := sudo("rm", "-f", path); err != nil {
			return fmt.Errorf(i18n.T("clear Limine remembered entry: %w"), err)
		}
	}
	return nil
}
