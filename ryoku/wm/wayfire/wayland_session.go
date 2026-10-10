package main

// sessionEntry is the wayland-session .desktop a greeter offers, byte-identical
// to the entry the wayfire package itself ships. DesktopNames included: that is
// what sets XDG_CURRENT_DESKTOP, which is how the seam detects the provider
// when no session handle is exported yet. The installer writes this as a
// fallback inside a chroot, only when the package's entry is missing.
const sessionEntry = `[Desktop Entry]
Name=Wayfire
Exec=wayfire
TryExec=wayfire
Icon=
Type=Application
DesktopNames=Wayfire;wlroots`

// runSession prints the wayland-session entry. It must work with no live
// compositor: the installer writes the file inside a chroot.
func runSession() error {
	_, err := stdout.WriteString(sessionEntry + "\n")
	return err
}
