# Ryoku packages

The Ryoku desktop ships as signed pacman packages served from the `[ryoku]`
repository (`Server = https://repo.ryoku.dev/stable/$arch`). Each directory here is one
package; the publish CI builds every `PKGBUILD` and pushes the results to the
repo. Packages publish only from `main` release tags, never from `unstable-dev`.

## The set

- `ryoku-keyring` -- the release signing key, into `/usr/share/pacman/keyrings`.
  Installed first so pacman trusts the repo. (Built from key material beside its
  own PKGBUILD; the only package that is not built from the source tree.)
- `ryoku-shell` -- the shell IPC daemon (Go), to `/usr/bin/ryoku-shell`.
- `ryoku-hub` -- the Hub backend (Go), to `/usr/bin/ryoku-hub`.
  Its card sandbox and account sign-in require `bubblewrap`, the WebP CLI tools
  (`cwebp`, `dwebp`), `ffmpeg` with `libvpx-vp9`, `ffprobe` and `secret-tool`.
- `ryoku` -- the control CLI (update / rollback / snapshots / materialize / ...),
  to `/usr/bin/ryoku`.
- `ryoku-blobs` -- the `Ryoku.Blobs` QML plugin, to
  `/usr/lib/qt6/qml/Ryoku/Blobs`.
- `skwd-paper-bin` -- the pinned upstream skwd-paper scene renderer (Wallpaper
  Engine wallpapers), a hard dependency of `ryogami`, into `/usr/bin`
  and `/usr/lib/skwd-paper`. A fixed-version binary rebuild like `ryoku-cursors`:
  bumped by `pkgver` + `sha256sums` when upstream cuts a release.
- `hypr-dynamic-cursors`, `ryoku-hypr-plugins` (hyprbars + hyprfocus),
  `hyprglass`, `imgborders` -- the optional Hyprland compositor plugins the Hub
  can toggle, into `/usr/lib/hyprland/plugins/`. Each builds from source
  version-matched to the build host's Hyprland (its `prepare()` reads the plugin's
  `hyprpm.toml` and checks out the commit paired with the running Hyprland), so
  they track the shipped compositor with no manual pin bumps. Off until enabled
  in Ryoku Settings.
- `xwayland-satellite` -- the X11 bridge niri spawns for Xwayland apps, built a
  few commits past `v0.8.2` because that release regressed override-redirect
  popup focus (Steam and Wine menus close on sight under niri). A pinned commit,
  not a monorepo build; a later official `0.8.3` sorts above it and takes over.
- `xdg-desktop-portal-hyprland` -- the Hyprland portal pinned ten commits past
  `v1.4.1`: it preserves the screencopy callback across DMA-BUF renegotiation
  and bounds PipeWire stream startup instead of hanging forever. A later
  official `1.4.2` sorts above it and takes over.
- `ryoku-desktop` -- the umbrella. Depends on the packages above plus the user-facing
  desktop runtime, lays the base configuration under `/usr/share/ryoku/config`,
  and installs the helper scripts (`ryoku-cmd-*`, the hardware `ryoku-*`,
  `ryoku-fastfetch`) to `/usr/bin` and the GPU udev rule to
  `/usr/lib/udev/rules.d`.

## Build-from-checkout model

Most of these PKGBUILDs build from the checked-out monorepo, not from release tarballs.
`source=()` is empty; each PKGBUILD derives the repo root as `$startdir/../../..`,
because the CI runs `makepkg` in place inside each package directory within a
full checkout. The Go binaries and QML plugins are built into `$srcdir`, so the
source tree is never modified, and `makepkg --clean` removes `$srcdir` and
`$pkgdir` afterward.

The `gpk`, `skwd-paper-bin`, `hyprland-preview-share-picker`, and
`ryoku-keyring` PKGBUILDs fetch pinned upstream artifacts or source instead of
building from the checkout.

## Shared package payloads

Monorepo packages keep their build and install logic in
`release/packages/<pkg>/payload.sh`. The PKGBUILD calls that payload with
`systemd`, the matching `void/packages/srcpkgs/<pkg>/template` calls it with
`runit`, and Fedora's adapter calls it with `systemd` before applying its small
RPM-specific changes. Void does not ship Hyprland, so the parity test stages
that shared payload's runit path directly. Fedora's Hyprland adapter changes
only the portal picker's command name.

When changing one of these packages, update `payload.sh` first. Change a native
package recipe only for package-manager metadata or a documented adapter. Do
not copy build or install commands into a wrapper. `tests/payload-parity.sh`
stages all shared payloads with fake prebuilt binaries, exercises the Arch,
Void, and Fedora wrappers, and compares normalized path, mode, and SHA-256
manifests. Its single allow-list covers only init, boot, package-manager, and
other edition adapters named in the test.

makedepends across the set include `go` (the Ryoku Go tools),
`cmake ninja qt6-shadertools qt6-declarative` (ryoku-blobs), Gio's Wayland,
xkbcommon, EGL and Vulkan headers, and `rust` + `git`
(hyprland-preview-share-picker, asusctl), on top of the assumed `base-devel`.

## Hub runtime dependencies

| Edition | Sandbox | WebP CLI | Media CLI | Secret Service CLI |
| --- | --- | --- | --- | --- |
| Arch / CachyOS | `bubblewrap` | `libwebp-utils` | `ffmpeg` | `libsecret` |
| Fedora 44 / 45 | `bubblewrap` | `libwebp-tools` | `ffmpeg-free` | `libsecret` |
| Void | `bubblewrap` | `libwebp-tools` | `ffmpeg6` | `libsecret` |
| Nix | `bubblewrap` | `libwebp` | `ffmpeg` | `libsecret` |

These are hard dependencies of `ryoku-hub`, and the shared package
set carries them for source installs too. Nix adds their executable outputs to
the Hub wrapper's `PATH`. No service is added or enabled. Account storage still
needs the session's existing Secret Service provider and an unlocked collection.

The library package alone does not supply `cwebp` and `dwebp` on
[Arch](https://archlinux.org/packages/extra/x86_64/libwebp-utils/files/),
[Fedora](https://packages.fedoraproject.org/pkgs/libwebp/libwebp-tools/) or
[Void](https://github.com/void-linux/void-packages/blob/master/srcpkgs/libwebp/template).
The [Nix derivation](https://github.com/NixOS/nixpkgs/blob/56c02bc00adcf003215cc4bd996d6efaf4cff188/pkgs/by-name/li/libwebp/package.nix)
keeps the tools in `libwebp`. Package metadata is not an installation or sandbox
proof: check the packed Hub, all six executables, VP9 encoding,
and real sandboxed imports on each supported edition and compositor.

No public card-authority registry is installed by these recipes.
`/usr/share/ryoku/card-authorities.json` remains absent until approved public
keys are supplied, so online animation verification fails closed. Never fill
it with test keys or enable a fallback.

## Configs and materialize

`ryoku-desktop` installs the base configs to `/usr/share/ryoku/config`, mirroring
the `~/.config` layout (for example `hypr/hyprland.lua`, `quickshell/...`,
`quickshell/hub/...`, `fish/config.fish`, `qt6ct/qt6ct.conf`, `starship.toml`). The
Hyprland tree includes `scripts/`, so after materialize the `ryoku-cmd-*` and
`*.sh` helpers also sit at `~/.config/hypr/scripts`, where the shell invokes them
by absolute path.

`ryoku materialize` (run as the user, on login or by hand) copies that tree into
`~/.config`: it clobbers the files it owns and prunes files dropped from a
release, but never touches user files such as `hypr/user.lua` or
`fish/user.fish`. Package install runs as root and cannot write a user's home, so
the `ryoku-desktop` install scriptlet only prints the materialize hint.
