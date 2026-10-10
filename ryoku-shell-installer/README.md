# ryoku-shell

Install the Ryoku desktop on an existing machine, without the ISO.

Arch-based hosts install signed packages from `[ryoku]`. Void installs the same
desktop package set from the signed Ryoku XBPS repository, with niri, runit,
elogind and Turnstile. Fedora installs the packaged desktop from the signed
Ryoku RPM repository and restricted dependency COPRs. Debian remains the
source-build path: its package manager supplies the runtime and build
dependencies, and the installer fetches the pinned fonts and cursor theme its
archive does not carry.

The Void edition is niri-only because Hyprland needs GCC 15 and Void ships GCC
14. Zen Browser, LocalSend, Voxtype, asusctl, Ryotunes, and a few small extras
are not available. A conversion keeps the machine's existing bootloader and
snapshot stack; the shell installer provisions neither. Snapshot rollback works
when the existing root is btrfs with snapper set up, and `ryoku doctor`
converges the snapper config the same way it does on Arch. XBPS has no
transaction hooks, so use `ryoku update --system` for base updates to keep the
snapshot safety net.

## Fedora

Fedora Workstation and Server use the same public installer command. The
installer supports mutable x86_64 Fedora systems with systemd and either dnf5
or dnf. rpm-ostree editions are refused. It imports the verified Ryoku release
key, writes `/etc/yum.repos.d/ryoku.repo`, adds only the package-restricted
COPRs listed by the release payload, and installs `ryoku`, `ryoku-desktop` and
the selected Hyprland or niri variant. Fedora has no Ryoku ISO.

The payload ref selects the RPM channel: `main` uses stable, `unstable-dev`
uses testing, and a release tag uses that frozen release. Fedora updates,
channel moves and rollback then use `ryoku update`, `ryoku track` and
`ryoku rollback` like the other packaged editions. NVIDIA stays on nouveau in
this release.

```bash
curl -fsSL https://ryoku.dev/install.sh | bash
```

Headless / unattended:

```bash
curl -fsSL .../install.sh | bash -s -- --yes
```

Choose a browser and login shell in the interactive plan, or pass the same
choices to an unattended install:

```bash
curl -fsSL .../install.sh | bash -s -- --yes --browser chromium --shell zsh
```

`--browser` accepts `firefox` (recommended and the default), `chromium`, or
`zen`. `--shell` accepts `fish` (recommended), `zsh`, or `bash`. With no shell
flag, the installer keeps the account's current login shell when it is one of
those three, and otherwise chooses Fish. `RYOKU_BROWSER` and
`RYOKU_LOGIN_SHELL` provide the flag defaults for scripted runs.

The package picks are translated for the host: Firefox, Chromium and the chosen
Fish, Zsh or Bash stack are read from the distro package table. A choice marked
unavailable there is skipped with a note. Arch builds Zen in the AUR step; Zen
is not available on Void or Fedora. Bash itself and the shared terminal tools
(`starship`, `fastfetch`, `zoxide`, `fzf`, `eza`, `bat`, and `mise`) stay
available with every choice.

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
2. **Plan** the browser and login shell, then review the migration toggles.
   Firefox and Fish are recommended. On Arch, Zen locks the AUR step on because
   its package is built there. Only the chosen browser and chosen shell stack
   are added to the package transaction.
3. **Install**, streamed step by step. Arch retires legacy repositories, adds
   `[ryoku]`, installs signed desktop packages and handles AUR extras. Void
   seeds the XBPS signing key, configures the matching Ryoku channel, resolves
   the Void package lanes and installs the signed desktop packages. Fedora
   verifies and imports the RPM release key, writes the channel and restricted
   COPR repository files, then installs the signed desktop and selected
   compositor packages with the translated browser and shell choices. Debian
   installs distro dependencies, fetches the pinned fonts and cursor, then
   builds the desktop from the payload. Every path backs up existing configs,
   wires SDDM/qylock and networking, materializes the selected compositor
   config, applies browser and login-shell choices, runs `ryoku doctor`, and
   verifies the result.

Afterwards Arch, Void and Fedora machines update from their signed Ryoku
package repositories. Debian source builds can rerun the installer at a newer
payload ref; `ryoku doctor` remains the common health and repair path.

The browser choice sets the HTTP, HTTPS and HTML XDG defaults and the desktop's
`desktop.apps.browser` role. Packages omitted by either choice are recorded in
`~/.local/state/ryoku/provisioned`, unless they were already installed before
the conversion. That keeps `ryoku doctor` and `ryoku update` from adding an
intentionally omitted package later, without removing software the user already
had.

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
