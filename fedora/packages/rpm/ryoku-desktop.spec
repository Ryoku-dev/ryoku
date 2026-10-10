Name:           ryoku-desktop
Version:        0.1
Release:        1%{?dist}
Summary:        ryoku-desktop desktop component
License:        GPL-3.0-or-later
URL:            https://ryoku.dev
Source0:        ryoku-%{version}.tar.gz
%global debug_package %{nil}
BuildRequires:  bash
BuildRequires:  coreutils
BuildRequires:  findutils
BuildRequires:  gtk3-devel
BuildRequires:  webkit2gtk4.1-devel
BuildRequires:  golang >= 1.26.4
BuildRequires:  cmake
BuildRequires:  ninja-build
BuildRequires:  gcc-c++
BuildRequires:  qt6-qtbase-devel
BuildRequires:  qt6-qtdeclarative-devel
Requires:       ryoku = %{version}-%{release}
Requires:       ryoku-shell = %{version}-%{release}
Requires:       ryogami = %{version}-%{release}
Requires:       ryoku-hub = %{version}-%{release}
Requires:       ryoku-rashin = %{version}-%{release}
Requires:       ryoku-blobs = %{version}-%{release}
Requires:       ryoku-palette-bridge = %{version}-%{release}
Requires:       ryoku-extras = %{version}-%{release}
Requires:       sddm-theme-ryoku = %{version}-%{release}
Requires:       ryoku-desktop-compositor
Requires:       quickshell
Requires:       sddm
Requires:       weston
Requires:       kitty
Requires:       fish
Requires:       starship
Requires:       fastfetch
Requires:       adw-gtk3-theme
Requires:       papirus-icon-theme
Requires:       gnome-keyring
Requires:       gnome-keyring-pam
Requires:       libsecret
Requires:       polkit
Requires:       xdg-desktop-portal
Requires:       xdg-desktop-portal-gtk
Requires:       xdg-user-dirs
Requires:       qt6-qtwayland
Requires:       qt5-qtwayland
Requires:       qt6-qt5compat
Requires:       qt6-qtsvg
Requires:       qt6-qtimageformats
Requires:       qt6-qtmultimedia
Requires:       kf6-syntax-highlighting
Requires:       pipewire
Requires:       pipewire-pulseaudio
Requires:       NetworkManager
Requires:       NetworkManager-wifi
Requires:       pciutils
Requires:       usbutils
Requires:       kmod
Requires:       dracut
Requires:       grubby
Requires:       grub2-tools
Requires:       btrfs-progs
Requires:       snapper
Requires:       wireless-regdb
Requires:       wireplumber
Requires:       brightnessctl
Requires:       playerctl
Requires:       wl-clipboard
Requires:       cliphist
Requires:       grim
Requires:       slurp
Requires:       cava
Requires:       jq
Requires:       hypridle
Requires:       dbus-tools
Requires:       dnf
Requires:       rpm
Requires:       sudo
Requires:       ImageMagick
Requires:       curl
Requires:       python3
Requires:       rsms-inter-fonts
Requires:       google-noto-sans-fonts
Requires:       google-noto-sans-cjk-fonts
Requires:       google-noto-emoji-fonts
Requires:       jetbrains-mono-fonts
Provides:       ryostore
Provides:       ryovm
Provides:       ryoku-ui = %{version}-%{release}
Obsoletes:      ryostore < 0.2
Obsoletes:      ryovm < 0.2
Obsoletes:      ryoku-ui < 0.2

Requires:       qt6ct
Requires:       zsh
Requires:       zsh-autosuggestions
Requires:       zsh-syntax-highlighting
Requires:       mpv
Requires:       mpv-mpris
Requires:       yt-dlp
Requires:       pulseaudio-utils
Requires:       pamixer
Requires:       zenity
Requires:       iw
Requires:       nftables
Requires:       bluez
Requires:       pam-u2f
Requires:       rtkit
Requires:       tesseract
Requires:       tesseract-langpack-eng
Requires:       zbar
Requires:       wf-recorder
Requires:       hyprsunset
Requires:       wtype
Requires:       libqalculate
Requires:       upower
Requires:       power-profiles-daemon
Requires:       ddcutil
Requires:       uv
Recommends:     quickemu
Recommends:     spice-gtk-tools

%description
ryoku-desktop, built from the shared Ryoku source and package payload recipe.

%prep
%setup -q -n ryoku-%{version}

%build
export RYOKU_PKGVER=%{version}
bash fedora/packages/rpm/stage-package.sh %{name} "$PWD/stage" %{_libdir}

%install
cp -a stage/. %{buildroot}/
find %{buildroot} \( -type f -o -type l \) -print0 | python3 -c 'import os,sys; root=os.fsencode(sys.argv[1]); [print("\"" + os.fsdecode(p[len(root):]).replace("\\", "\\\\").replace("\"", "\\\"") + "\"") for p in sys.stdin.buffer.read().split(bytes([0])) if p]' %{buildroot} > rpm-files

%pre
# Block sleep before the old lid, idle and shell owners are replaced. An
# upgrade runs the installed helper, which also records the live sessions; the
# first release that ships it holds the same durable guard directly.
[ -d "${RYOKU_SYSTEMD_RUNTIME_DIR:-/run/systemd/system}" ] || exit 0
systemd-detect-virt --quiet --chroot && exit 0
helper="${RYOKU_POWER_CUTOVER_HELPER:-/usr/bin/ryoku-power-cutover}"
[ -x "$helper" ] && exec "$helper" prepare-package
systemctl is-active --quiet ryoku-power-cutover-guard.service && exit 0
systemctl reset-failed ryoku-power-cutover-guard.service >/dev/null 2>&1 || :
systemd-run --quiet --collect --unit=ryoku-power-cutover-guard.service \
  --property=Type=exec --property=TimeoutStopSec=5s \
  /usr/bin/systemd-inhibit --what=sleep --mode=block \
  --who=ryoku-package-cutover \
  --why="keep sessions awake while packaged suspend owners are replaced" \
  /usr/bin/sleep infinity

%post
systemctl --global enable ryoku-bootstrap.service ryoku-bluetooth-reset.service >/dev/null 2>&1 || :
systemctl daemon-reload >/dev/null 2>&1 || :
systemctl enable ryoku-wifi-regdom.service >/dev/null 2>&1 || :
/usr/bin/ryoku-bluetooth-tune >/dev/null 2>&1 || :
# boot_success is set by the shell once the desktop proves itself, so a boot
# that never gets there shows the GRUB menu; Fedora's timer would set it for
# any login.
systemctl --global disable grub-boot-success.timer >/dev/null 2>&1 || :
systemctl enable ryoku-snapshot-restored.service ryoku-snapshot-menu.path >/dev/null 2>&1 || :

%posttrans
# Theme GRUB and render the snapshot submenu's snippet into grub.cfg (the
# installer does this itself, after Anaconda writes the bootloader config).
# On a running system, list the snapshots already on disk in the background,
# since a kernel image is built per kernel version they need.
/usr/bin/ryoku-grub-menu install >/dev/null 2>&1 || :
# Adopt the new sleep and lid policy in every live Hyprland or niri session,
# then release the guard %pre took. A failure keeps sleep blocked until
# `ryoku update` retries or the box reboots.
[ -d /run/systemd/system ] || exit 0
systemd-detect-virt --quiet --chroot && exit 0
systemctl start --no-block ryoku-snapshot-menu.path ryoku-snapshot-menu.service >/dev/null 2>&1 || :
[ -x /usr/bin/ryoku-power-cutover ] || exit 0
/usr/bin/ryoku-power-cutover package

%preun
# Removal: hand the boot flag back to Fedora's timer, drop the console
# entries while their plugin is still on disk and the GRUB theme and snapshot
# menu while their helper is, then keep a copy of the
# executor in /run so %postun can still hand the live sessions back to
# logind's default policy.
[ "$1" -eq 0 ] || exit 0
systemctl --global enable grub-boot-success.timer >/dev/null 2>&1 || :
/usr/lib/kernel/install.d/95-ryoku-console.install purge >/dev/null 2>&1 || :
systemctl disable ryoku-snapshot-restored.service ryoku-snapshot-menu.path >/dev/null 2>&1 || :
/usr/bin/ryoku-grub-menu purge >/dev/null 2>&1 || :
[ -d /run/systemd/system ] || exit 0
systemd-detect-virt --quiet --chroot && exit 0
systemctl stop ryoku-snapshot-menu.path >/dev/null 2>&1 || :
[ -x /usr/bin/ryoku-power-cutover ] || exit 0
/usr/bin/ryoku-power-cutover prepare-package

%postun
[ "$1" -eq 0 ] || exit 0
# the theme and the snapshot snippet are gone; drop them from grub.cfg too.
grub2-mkconfig -o /boot/grub2/grub.cfg >/dev/null 2>&1 || :
[ -x /run/ryoku-power-cutover ] || exit 0
/run/ryoku-power-cutover package

%files -f rpm-files
