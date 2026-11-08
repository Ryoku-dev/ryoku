# Fedora edition (experimental)

The shell installer installs Ryoku onto existing mutable Fedora 44 or 45
Workstation, KDE Plasma, Server, and spin systems on `x86_64`, with systemd and
either Hyprland or niri. Atomic and other rpm-ostree systems are refused. Run:

```sh
curl -fsSL https://raw.githubusercontent.com/ryoku-dev/ryoku/main/ryoku-shell-installer/install.sh | bash
```

Ryoku 1.0 publishes stable packages for both supported Fedora releases. The
default command follows that stable channel. Use `bash -s -- --ref
unstable-dev` only when you intend to install the testing channel.

Before sudo or any system change, the installer requires repository metadata
for the selected channel and running Fedora release. If Ryoku does not publish
that release, it stops and prints the supported list from
`https://repo.ryoku.dev/stable/fedora/releases`. The installer uses dnf5 or
dnf, verifies and imports Ryoku's release key, writes the signed RPM repository,
and configures SDDM. It replaces GDM when present; KDE keeps its existing SDDM
greeter configuration by default. The `ryoku-keyring` RPM then owns the key file
that the repository names, so dnf picks up a rotated key on the next update.

Ryoku's repository carries the Fedora-compatible Arch desktop set, including a
Ryoku-built Quickshell 0.3 or newer, GPU Screen Recorder, GlazePKG, the Hyprland
plugins, skwd-paper, Ryotunes, Ryoport, fonts, cursors, controller support, and
optional apps. Package-restricted COPR repositories provide only the Hyprland stack,
starship, lazygit, and yazi packages Fedora 44 and 45 lack. The package
translation maps the desktop's power-profile dependency to `ppd-service`, so
Fedora's tuned-ppd and power-profiles-daemon providers both work and the
installer does not replace one with the other.

Stable Release dispatches the Fedora publisher on each `v*` tag. Release jobs
prepare SRPMs from the shared tree, rebuild and sign the binary RPMs and
`repomd.xml`, install-test both desktops on Fedora 44 and 45, then publish the
tested stable heads and frozen repositories. They also publish the supported
release list at `stable/fedora/releases`. For Fedora release `<N>`, the public
layout below
`https://repo.ryoku.dev/` is:

```text
stable/fedora/releases
stable/fedora/<N>/x86_64
stable/fedora/<N>/releases/<tag>/x86_64
stable/fedora/<N>/releases/index.json
stable/fedora/<N>/channels/testing/x86_64
stable/fedora/<N>/channels/testing/builds/<build>/x86_64
stable/fedora/<N>/channels/testing/index.json
```

The build name replaces `+` with `_` in the frozen directory. Public repository
directories contain signed binary RPMs, `repodata/`, the detached armored
`repomd.xml` signature, and `release.json`; SRPMs are not published.

Specs under `packages/rpm/` either stage the Ryoku tree or build upstream
projects. Upstream URL sources have checksum manifests under `sources/`; locked
Go and Cargo dependencies are vendored for offline rebuilds; verified
`fetch/` hooks supply sources that need custom preparation. A matching trusted
upstream RPM may be reused from the published repository after signature and
dependency checks. Ryoku-tree RPMs always rebuild.

`ryoku update`, `ryoku track`, and `ryoku rollback` use DNF. Update and doctor
say when Ryoku has not published the running Fedora release and leave the
installed Ryoku set in place. Frozen release and testing targets are retained
for package rollback, and the boot guard can downgrade the Ryoku set after a
failed update.

Fedora keeps GRUB, and snapper snapshots are on. `ryoku doctor` gives
`/.snapshots` its own top-level btrfs subvolume and fstab entry, `ryoku update`
takes a pre/post pair, and a libdnf5 actions hook (`system/snapshots/`) takes a
pair around each dnf transaction that `ryoku update` does not run. GRUB lists the
snapshots under **Ryoku snapshots**. Fedora hides the menu after a good boot, so
hold Shift or press Esc while booting to see it. A snapshot boots read-only and
keeps its writes in RAM. `/home` shows as it is now, and writes there are
dropped too. Run `sudo ryoku boot-menu restore` in that boot to make the
snapshot the system again; `ryoku rollback <id>` walks through the steps. Shim,
GRUB and Fedora's kernels stay as Fedora signs them, and the snapshot entries
boot copies of those kernels from `/boot/ryoku/snapshots`. The menu uses the
Ryoku theme (`system/boot/grub/theme/`), which puts the Limine menu's layout on
the Ryoku wallpaper. `ryoku-grub-menu` builds its "Ryoku Mono" fonts onto
`/boot` from SpaceMono when `ryoku-extras` installed it, else from Noto Sans
Mono.
The Ryoku Plymouth splash shows from GRUB to the login screen: `ryoku-desktop`
pulls in the script and label plugins it draws with, and `ryoku-plymouth-apply`
(`system/boot/ryoku-plymouth-apply`) makes it the default and rebuilds the dracut
images on install. Later updates rebuild only while it is still the default.
Wayfire is unavailable because Fedora ships
0.10.1 and Ryoku requires 0.11. AUR-only extras such as Zen Browser, LocalSend,
Voxtype, Looking Glass, and SteamCMD are not shipped in the Fedora lane.

Ryoku leaves NVIDIA GPUs on nouveau. To install NVIDIA's driver, add RPM Fusion
and create a signing key for the driver:

```sh
sudo dnf install \
  https://mirrors.rpmfusion.org/free/fedora/rpmfusion-free-release-$(rpm -E %fedora).noarch.rpm \
  https://mirrors.rpmfusion.org/nonfree/fedora/rpmfusion-nonfree-release-$(rpm -E %fedora).noarch.rpm
sudo dnf install akmods mokutil openssl
sudo kmodgenca -a
```

With Secure Boot on, enroll the key. Set a password, reboot, choose Enroll MOK,
and enter that password:

```sh
sudo mokutil --import /etc/pki/akmods/certs/public_key.der
```

Then install the driver, wait a few minutes for it to build, and reboot:

```sh
sudo dnf install akmod-nvidia
```

`lsmod | grep '^nvidia'` lists the driver when it works. If the shell does not
start, Secure Boot rejected the driver: enroll the key again, or go back to
nouveau with `sudo dnf remove akmod-nvidia xorg-x11-drv-nvidia\*`.
