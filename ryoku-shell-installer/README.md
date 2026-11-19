# ryoku-shell

Install the Ryoku desktop on an existing machine, without the ISO.

Arch-based hosts install signed packages from `[ryoku]`. Void and Fedora are
experimental editions in 1.0. Void installs the same desktop package set from
the signed Ryoku XBPS repository, with niri, runit, elogind and Turnstile.
Fedora installs the packaged desktop from the signed Ryoku RPM repository and
restricted dependency COPRs. Debian remains the source-build path: its package
manager supplies the runtime and build dependencies, and the installer fetches
the pinned fonts and cursor theme its archive does not carry.

The Void edition is niri-only because Hyprland needs GCC 15 and Void ships GCC
14. Zen Browser, LocalSend, Voxtype, asusctl, adw-gtk-theme, and GPU
passthrough are not available. A conversion keeps the machine's existing
bootloader and snapshot stack; the shell installer provisions neither.
Snapshot rollback works when the existing root is btrfs with snapper set up,
and `ryoku doctor` converges the snapper config the same way it does on Arch.
XBPS has no transaction hooks, so use `ryoku update --system` for base updates
to keep the snapshot safety net.

## Fedora (experimental)

The public installer supports mutable x86_64 Fedora 44 and 45 Workstation,
KDE Plasma, Server, and spin systems with systemd and either dnf5 or dnf. It
refuses rpm-ostree editions. Before sudo or any system change, it checks that
the selected Ryoku channel publishes repository metadata for the running
release. `RYOKU_RPM_REPO` skips that check for local repository tests.
Ryoku 1.0 publishes stable packages for Fedora 44 and 45. The default `main`
ref installs that stable channel. `--ref unstable-dev` selects testing instead.

The installer imports the verified Ryoku release key, writes
`/etc/yum.repos.d/ryoku.repo`, adds the package-restricted COPRs from the
release payload, and installs the selected compositor variant. The payload ref
selects the RPM channel: `main` uses stable, `unstable-dev` uses testing, and a
release tag uses that frozen release.

The session conversion follows the Fedora edition:

- Workstation disables GDM, enables SDDM with the Ryoku greeter, leaves GNOME
  in the session list, and keeps the GNOME login keyring unlocking through
  SDDM.
- KDE Plasma keeps SDDM. Its current greeter theme stays selected unless the
  Ryoku greeter toggle is turned on; that writes an administrator drop-in that
  sorts after KDE's, and a managed block in `/etc/sddm.conf` only when an
  active theme there would otherwise win.
- Server and other spins enable SDDM and the graphical target. Bluetooth is
  enabled when its service is installed and was not already enabled.

Keyboard and monitor layouts are carried from GNOME, KDE, or existing
compositor configuration into Ryoku's settings, so they appear in the Hub.
The login screen, console, and boot prompt then follow the desktop keyboard
layout.

Fedora stays on the nouveau NVIDIA driver. The conversion sets up snapshots on
a btrfs root and has no Fedora ISO.

```bash
curl -fsSL https://raw.githubusercontent.com/ryoku-dev/ryoku/main/ryoku-shell-installer/install.sh | bash
```

For an unattended install, the same catalogue used by the ISO installer is
available through flags:

```bash
curl -fsSL .../install.sh | bash -s -- --yes --compositor niri --browser chromium --shell zsh --file-manager dolphin --rashin=false --without ryotunes,ryoport
```

`--compositor` accepts the variants shipped by the host edition. `--browser`
accepts `firefox` (recommended and the default), `chromium`, `zen`, or `none`.
`--shell` accepts `fish` (recommended), `zsh`, or `bash`; without it the
installer keeps a supported current shell. `--file-manager` accepts `nautilus`
(recommended), `dolphin`, or `thunar`; without it an installed one is kept.
`--rashin=false` leaves out the AI agent. `--without` takes comma-separated
component IDs such as `ryotunes,ryoport`. Plain Arch also accepts
`--cachyos-layer`, which adds the CachyOS kernel and tuning while keeping the
Arch kernel as a fallback.

The matching environment variables are `RYOKU_BROWSER`, `RYOKU_LOGIN_SHELL`,
`RYOKU_FILE_MANAGER`, `RYOKU_RASHIN=0|1`, `RYOKU_WITHOUT`, and
`RYOKU_CACHYOS_LAYER=0|1`. Package names are translated through the host
edition table. Choices unavailable on that edition are rejected before sudo.
Zen and Arch-only AUR components remain unavailable on Void and Fedora.

Preview without changing anything:

```bash
curl -fsSL .../install.sh | bash -s -- --dry-run
```

Remove the Ryoku desktop again:

```bash
ryoku-shell-install --uninstall        # or: ... | bash -s -- --uninstall
```

## What it does

`install.sh` is a dumb bootstrap: it verifies the machine is a supported
x86_64 family (pacman, apt-get, xbps, dnf5 or dnf), downloads the prebuilt
`ryoku-shell-install` binary (checksummed) from this directory, and hands it the
real terminal. On Fedora it refuses rpm-ostree and installs curl and git first
when either is missing.
Everything else is the binary, a Bubble Tea TUI using Ryoku's paper-and-ink
terminal language:

1. **Scan** the machine: distro, GPU, Secure Boot state, display manager,
   network stack, installed desktops (GNOME/KDE/Cinnamon/Xfce), rival
   quickshell shells (Noctalia, DankMaterialShell, Caelestia, iNiR), known
   Hyprland rices (ML4W, HyDE, JaKooLit, end-4, Caelestia), conflicting user
   daemons (dunst/mako/waybar/swww/…), a plain Hyprland, niri or sway setup
   to migrate from, an Omarchy install to retire (repo + mirror pin),
   keyboard layout, btrfs, and an interrupted previous run to resume.
2. **Plan** the compositor, browser, login shell, file manager, Rashin, and
   apps from the shared installer catalogue, then review the migration toggles.
   Existing supported shells and file managers are kept. Required edition
   components stay visible but cannot be turned off.
3. **Install**, streamed step by step. Arch retires legacy repositories, adds
   `[ryoku]`, installs signed desktop packages and handles AUR extras. Void
   seeds the XBPS signing key, configures the matching Ryoku channel, resolves
   the Void package lanes and installs the signed desktop packages. Fedora
   verifies and imports the RPM release key, writes the channel and restricted
   COPR repository files, then installs the signed desktop and selected
   compositor packages with the translated choices. Debian installs distro
   dependencies, fetches the pinned fonts and cursor, then builds the desktop
   from the payload. Every path backs up existing configs, wires SDDM/qylock
   and networking, materializes the selected compositor config, applies the
   application roles, runs `ryoku doctor`, and verifies the result.

Afterwards Arch, Void and Fedora machines update from their signed Ryoku
package repositories. Debian source builds can rerun the installer at a newer
payload ref; `ryoku doctor` remains the common health and repair path.

The browser and file-manager choices set their XDG defaults and the
`desktop.apps.browser` and `desktop.apps.files` roles. Packages omitted by any
choice are recorded in `~/.local/state/ryoku/provisioned`, unless they were
already installed before the conversion. That keeps `ryoku doctor` and
`ryoku update` from adding an intentionally omitted package later without
removing software the user already had.

Migration policy: rival shells are uninstalled (toggle), conflicting daemons
are disabled but never uninstalled, the old display manager is disabled (not
removed), desktop environments are never uninstalled and stay selectable at
the login screen, niri and sway stay installed as fallback sessions, and
every config the install touches is saved to
`~/.local/state/ryoku/shell-install/backup-<ts>/` first, `restore.sh`
included. Monitor layout and keyboard intent are salvaged with compositor
configs first: Hyprland (`monitor=`/`monitorv2`/`input`, includes and `$vars`
followed) beats niri beats sway, then KDE (`kxkbrc`,
`kwinoutputconfig.json`), then GNOME (gsettings input-sources,
`monitors.xml`), then `localectl`.

Safety gates: Arch, Debian and Fedora require systemd. Fedora must be a mutable
installation; rpm-ostree systems are refused. Void requires its native runit
boot and the x86_64 glibc package set. With Secure Boot enforcing, the NVIDIA
toggle is forced off and locked on Arch because unsigned DKMS modules are
rejected; sign with sbctl or disable Secure Boot, then re-run. Fedora does not
install the proprietary NVIDIA driver in this release.
Manjaro requires a typed acknowledgement in the TUI and is refused under
`--yes` unless `RYOKU_ALLOW_MANJARO=1` is set.

Lifecycle: an interrupted run records its completed steps in
`~/.local/state/ryoku/shell-install-state.json`; the next run offers a
resume toggle (automatic with `--yes`) that skips finished steps and
continues the same backup. `--uninstall` removes signed Ryoku packages where
present, drops the Arch repository stanza, the Void XBPS override, or Fedora's
Ryoku and restricted COPR repository files and RPM signing key, then walks the
backup chain newest to oldest, running each `restore.sh` with confirmation.
Session packages such as SDDM, PipeWire and NetworkManager are left installed.

## Development

```bash
go build -trimpath -o ryoku-shell-install .   # rebuild
sha256sum ryoku-shell-install > ryoku-shell-install.sha256
go test ./...
```

The binary and its checksum are committed (same convention as
`installation/tui/ryoku-tui`) so `install.sh` can fetch them from
raw.githubusercontent.com with no release infrastructure. Test a branch with:

```bash
curl -fsSL https://raw.githubusercontent.com/ryoku-dev/ryoku/<branch>/ryoku-shell-installer/install.sh \
  | RYOKU_SHELL_REF=<branch> bash
```

The ref also picks the package channel: `unstable-dev` points the Arch, Void and
Fedora repositories at testing, a Fedora release-tag payload uses that frozen
RPM release, and any other ref uses stable. The choice is recorded the way
`ryoku track` records it. Rerunning from another ref moves the Ryoku channel and
redoes the payload and package steps, so a half-finished install can be finished
on unstable.

`--payload /path/to/checkout` (or `RYOKU_SHELL_PAYLOAD`) skips the payload
clone and uses a local repo, for iterating without pushing.

### Test overrides

- `RYOKU_XBPS_REPO`: repository URL or absolute path written for a Void
  installer test.
- `RYOKU_XBPS_KEYRING_DIR`: directory of `*.plist` trust keys seeded before
  the first repository sync.
- `RYOKU_RPM_REPO`: repository URL or absolute path written into
  `/etc/yum.repos.d/ryoku.repo` for a Fedora installer test.
- `RYOKU_RPM_KEY_URL`: alternative HTTPS release-key URL; the key must still
  contain exactly the Ryoku release certificate and fingerprint.
- `RYOKU_DNF_REPO_CONFIG`: alternative path for the Fedora `ryoku.repo` file
  in installer tests.
