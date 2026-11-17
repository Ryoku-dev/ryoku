package apprelease

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"ryoku-cli/internal/sys"
)

var checkoutCommands = []string{"ryoport", "ryossh", "ryovm", "ryovm-fetch", "ryovm-mon"}

const checkoutDesktop = "[Desktop Entry]\nType=Application\nName=Ryoport\nGenericName=Fleet Dashboard\nExec=ryoport\nIcon=dev.ryoku.ryoport\nTerminal=false\nCategories=System;\nX-Ryoku-Checkout-Release=true\n"

func checkoutDir(home string) string {
	return filepath.Join(home, ".local/share/ryoku/ryoport-release")
}

func CheckoutRyoportVersion() string {
	return checkoutRyoportVersion(sys.Home())
}

func checkoutRyoportVersion(home string) string {
	dir := checkoutDir(home)
	link, err := os.Readlink(filepath.Join(home, ".local/bin/ryoport"))
	if err != nil || link != filepath.Join(dir, "ryoport") {
		return ""
	}
	info, err := os.Stat(link)
	if err != nil || !info.Mode().IsRegular() || info.Mode()&0o111 == 0 {
		return ""
	}
	version, err := os.ReadFile(filepath.Join(dir, "version"))
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(version))
}

func installCheckoutRyoport(ctx context.Context, home, archive, version string) error {
	dir := checkoutDir(home)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	file, err := os.CreateTemp(dir, ".ryoport-")
	if err != nil {
		return err
	}
	defer os.Remove(file.Name())
	cmd := exec.CommandContext(ctx, "bsdtar", "-xOf", archive, "usr/bin/ryoport")
	cmd.Stdout = file
	cmd.Stderr = os.Stderr
	err = cmd.Run()
	if closeErr := file.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		return fmt.Errorf("extract Ryoport binary: %w", err)
	}
	if err := os.Chmod(file.Name(), 0o755); err != nil {
		return err
	}
	if err := os.Rename(file.Name(), filepath.Join(dir, "ryoport")); err != nil {
		return err
	}
	bin := filepath.Join(home, ".local/bin")
	if err := os.MkdirAll(bin, 0o755); err != nil {
		return err
	}
	for _, name := range checkoutCommands {
		link := filepath.Join(bin, name)
		pending := link + ".ryoku-release"
		_ = os.Remove(pending)
		if err := os.Symlink(filepath.Join(dir, "ryoport"), pending); err != nil {
			return err
		}
		if err := os.Rename(pending, link); err != nil {
			_ = os.Remove(pending)
			return err
		}
	}
	appdir := filepath.Join(sys.Xdg("XDG_DATA_HOME", ".local/share"), "applications")
	if err := os.MkdirAll(appdir, 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(appdir, "ryovm.desktop"), []byte(checkoutDesktop), 0o644); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, "version"), []byte(version+"\n"), 0o644)
}

func RemoveCheckoutRyoport() error {
	return removeCheckoutRyoport(sys.Home(), sys.Xdg("XDG_DATA_HOME", ".local/share"))
}

func removeCheckoutRyoport(home, data string) error {
	dir := checkoutDir(home)
	for _, name := range checkoutCommands {
		path := filepath.Join(home, ".local/bin", name)
		if target, err := os.Readlink(path); err == nil && target == filepath.Join(dir, "ryoport") {
			if err := os.Remove(path); err != nil {
				return err
			}
		}
	}
	desktop := filepath.Join(data, "applications/ryovm.desktop")
	if body, err := os.ReadFile(desktop); err == nil && string(body) == checkoutDesktop {
		if err := os.Remove(desktop); err != nil {
			return err
		}
	}
	return os.RemoveAll(dir)
}
