# Native source distribution

Hyakki's runnable source is the direct `ryoku/hyakki/` module. Lithic and i18n stay at the canonical repository paths `docs/lithic/library/go/` and `ryoku/i18n/`, reached by `../../docs/lithic/library/go` and `../i18n` from the module. The Go module identity remains `ryoku-rashin`. There are four executables: the command line `hyakki`, the daemon `oni`, the Gio application `hyakki-app`, and `oni-surface`, the layer-shell surface that draws Oni on the desktop. `hyakki oni show` and the app start `oni-surface`; it never starts at login on its own. The desktop and Wayland identity is `dev.ryoku.Hyakki`.

This is a native Linux build and staging path, with private package recipes under `release/packages/` for Arch (`arch/PKGBUILD`), Fedora (`fedora/hyakki.spec`), Void (`void/template`) and Nix (`nix/hyakki.nix`). Nothing is published: no AUR, Copr, xbps repository, nixpkgs entry, Flatpak, deb or binary release exists, and none is implied by these files. The research and assessment directories are not distribution inputs.

## Prerequisites

Use Go 1.26 or newer as required by `go.mod`, Python 3.10 or newer, a POSIX shell, a C compiler and pkg-config. The Go dependencies must already be present in `vendor/`, including the contained local replacements. The build sets `GOWORK=off`, `GOTOOLCHAIN=local`, `GOPROXY=off`, `GOSUMDB=off`, `GOENV=off` and `-mod=vendor`, so it does not use a parent workspace, fetch modules or download another toolchain.

The production Gio build includes both Wayland and X11. Its native development dependencies are EGL/OpenGL headers and libraries, Wayland client/cursor/EGL headers and libraries, libxkbcommon, libxkbcommon-x11, Xlib, Xlib-XCB, Xcursor, Xfixes and Vulkan headers. These pkg-config names come from the included Gio source:

```sh
pkg-config --exists egl wayland-egl wayland-client wayland-cursor \
  xkbcommon xkbcommon-x11 x11 x11-xcb xcursor xfixes
```

Install the corresponding development packages for the build host. Runtime Gio requirements include the matching shared libraries, a working EGL/OpenGL driver or Vulkan loader/driver, and a Wayland compositor or X11 server. The CLI and daemon are built without cgo. Python 3 is needed for the optional usage collectors, and runit is needed only if its optional supervisor templates are used. The build and installer do not install any of these system dependencies.

External harnesses, model runners and optional memory services are separate installations. Git-backed project work needs Git, browser sign-in needs the session's URL opener, and filesystem sandboxing needs bubblewrap where that capability is used. The native payload does not bundle those programs or infer that an external provider is configured. Local and user-supplied providers do not require Ryoku Account, and this path does not turn on Ember, live payments or public hosted inference.

## Build and stage

From `ryoku/hyakki/`, with the dependencies and vendor tree already contained:

```sh
HYAKKI_VERSION=dev ./release/build.sh
DESTDIR="$PWD/build/stage" ./release/install.sh
```

`build.sh` compiles all four executables into `build/native/` and regenerates the bundled notice inventory. `HYAKKI_BUILD_DIR` may select another absolute build directory. `HYAKKI_VERSION` is embedded in each command's `main.version` value. `HYAKKI_GIO_TAGS=nox11` is an explicit Wayland-only lab override, not the production default or an equivalent desktop-support result.

`DESTDIR` must be an absolute, empty staging directory, never `/` or the source root. Do not run the installer with sudo against live paths. Root inside a packaging container or fakeroot is allowed. The installed prefix is `/usr`; this script stages that layout and is not a global install command. Installation requires fresh generated notices from the matching source/build and never rewrites source metadata during staging. It never invokes systemctl, sv, a package manager or a desktop-cache updater.

The staged payload contains:

- `/usr/bin/{hyakki,oni,hyakki-app,oni-surface}` and the optional `hyakki-usage`, `claude-usage`, `codex-usage` and `opencode-usage` collectors
- `/usr/lib/systemd/user/oni.{service,socket}`, `oni-routines.{service,timer}` and `hyakki-usage.{service,timer}`
- dormant runit user templates under `/usr/share/hyakki/runit/`, each with a `down` file
- the desktop launcher, AppStream metadata and scalable icon named `dev.ryoku.Hyakki`
- root and upstream notices, GPLv3, Apache 2.0, Material Symbols and font grants, and the full generated third-party inventory under `/usr/share/licenses/hyakki/`
- this guide, the licensing guides and the changelog under `/usr/share/doc/hyakki/`

The native staged payload contains no legacy executable aliases, login autostart entry or global enable symlink. The monorepo adapters reuse this payload; the Void adapter additionally installs dormant Ryoku session-environment bridge templates, and the service manifest marks every Hyakki entry on-demand. The desktop package does not also own those optional templates.

## Runtime paths and activation

Hyakki uses `$XDG_CONFIG_HOME/hyakki/hyakki.json`, and `hyakki/` below the XDG state, cache and data homes. The login-session socket is `$XDG_RUNTIME_DIR/hyakki/oni.sock`; the daemon log is `oni.log` and its runtime lock is `oni.lock`. The usual XDG home defaults apply when the optional home variables are unset. The user session must provide a valid private `XDG_RUNTIME_DIR`.

The normal standalone path is client self-start: opening the CLI or Gio app starts Oni on demand when needed, and Oni exits after its existing idle interval. No systemd or runit service is required, and no Ryoku host helper is used by this distribution's units or templates.

All supplied services are initially disabled. On a systemd desktop, `hyakki on` arms the installed socket and usage timer; Oni runs only when connected, with `Restart=no`. The routine timer follows enabled routines. `hyakki off` stops both timers and their in-flight services. The installer itself activates nothing.

### Optional user runit supervision

Use the default self-start path unless supervision is wanted. Templates are installed as data, outside every runsvdir. To use one, the user copies it into their own writable service directory and adds that directory to their existing user runsvdir. Preserve the included `down` file and export the login environment, including `HOME`, XDG variables and desktop/session variables, to that user supervisor. These are user services, not root services, and they do not depend on Turnstile or `ryoku-host`.

When the session runs runit instead of systemd (runit is process 1, or `/run/runit` exists, and `/run/systemd/system` does not), `hyakki on` and `hyakki off` change the on/off setting as usual, then print the exact steps and change no service. The service folder they name is `$SVDIR` when set, else `~/.config/service` (Turnstile's), else `~/service`, whichever exists first; with none, the steps use `"$SVDIR"` and say to set it. `hyakki on` prints, for that folder, the copy of whichever of `oni-routines` and `hyakki-usage` is missing there and `sv up` for both. `hyakki off` prints `sv down` and, for removal, `rm -r` for each Hyakki service present there. When `ryoku-host` is installed, the steps are its `ryoku-host svc --user enable|disable --now` lines instead, since Ryoku supervises the same services.

`oni/run` launches `/usr/bin/oni` with a private umask and logs to `$XDG_STATE_HOME/hyakki/oni.log`. Start it with `sv once /path/to/user/service/oni` when supervision is wanted. Its finish script requests down after every exit, including the daemon's clean idle exit, so runsv does not undo idle shutdown with a restart loop. Leave it down afterward; clients can still use the normal self-start path.

The optional scheduler templates run `hyakki service routines` or `hyakki service usage`. They retain the one-minute/30-minute routine schedule and 45-second/five-minute usage schedule, with an inotify watch instead of polling for activation changes. Turning Hyakki off stops the worker and its current child, and the finish script parks the supervisor down. Users explicitly arm these standalone services with `sv up` and rearm them after turning Hyakki back on; they are not automatically armed outside the Ryoku service bridge. Routine wake still checks activation and enabled work before starting Oni.

Manual refresh uses `hyakki service usage --once`, without the scheduler's initial delay. It shares an exclusive job lock with scheduled refreshes, does not start Oni, and runs no provider collectors while Hyakki is off. Turning Hyakki off also terminates and reaps an in-flight refresh.

The collectors preserve upstream provider names and provider-owned credential locations. They write only their result caches beneath `$XDG_CACHE_HOME/hyakki/`: `claude-usage.json`, `codex-usage.json`, `codex-usage-activity.json` and `opencode-usage.json`. Codex's existing explicit cache-file overrides remain available. `HYAKKI_USAGE_BIN_DIR` overrides where the dispatcher finds its three helpers. Claude and Codex collectors may query their matching authenticated provider; running the optional collector is not an inference request or a new account requirement.

## Package recipes

Every recipe builds from the corresponding-source archive (see [Source archive](#source-archive)) with `release/build.sh`, stages with `release/install.sh` and runs `release/check.sh` on the staged tree, so each package carries exactly the payload above. None enables, starts or links a unit, and none touches user data.

| Recipe | Archive location | Hard dependencies | Optional programs |
| --- | --- | --- | --- |
| `release/packages/arch/PKGBUILD` and `hyakki.install` | `hyakki-$pkgver.tar.gz` beside the PKGBUILD | Gio's X11, Wayland, EGL and Vulkan libraries, `python`, `git`, `xdg-utils`, `bubblewrap`, `hicolor-icon-theme` | `optdepends` |
| `release/packages/fedora/hyakki.spec` | `Source0` in `SOURCES/` | The same libraries by their Fedora names | `Suggests`, each with a comment |
| `release/packages/void/template` and `INSTALL.msg` | `files/hyakki-${version}.tar.gz` beside the template | The same libraries by their Void names | xbps has no optional dependencies; `INSTALL.msg` lists them |
| `release/packages/nix/hyakki.nix` | the `src` argument | The same libraries from nixpkgs; `bubblewrap`, `git` and `xdg-utils` on the daemon's `PATH` | The `optional` argument names ids from `release/optional.json`; their programs propagate into the user environment |

The recipes carry no archive digest, because no release is published; a published recipe must pin the release archive's SHA-256. The Nix derivation never wraps any of the four native executables. Runtime tools propagate into the user environment; the Ryoku Nix bundle also includes them explicitly. The app and surface remain physical trusted siblings of Oni.

### Optional programs

`release/optional.json` is the one list of optional programs. Each is off by default, used only once the setting that names it is turned on, and removed with the distribution's package manager, not by Hyakki. `release/check.py` fails when the PKGBUILD's `optdepends`, the spec's `Suggests`, Void's `INSTALL.msg` or this table disagree with it, when a program has no licence row, or when a named setting does not exist. Licences are in [oni-licensing.md](oni-licensing.md) and [licensing.md](licensing.md).

| Id | Programs | Used for | Arch | Fedora | Void | Nix |
| --- | --- | --- | --- | --- | --- | --- |
| `chromium` | `chromium` | The browser Hands drives in its own profile; any Chromium-family browser on `PATH` also works | `chromium` | `chromium` | `chromium` | `chromium` |
| `weston` | `weston` | The headless parent of the separate desktop Hands runs apps on | `weston` | `weston` | `weston` | `weston` |
| `niri` | `niri` | The separate desktop Hands runs apps on, and the maximize quick command | `niri` | `niri` | `niri` | `niri` |
| `grim` | `grim` | Screenshots without Ryoku's capture tool, and Hands looking at its desktop | `grim` | `grim` | `grim` | `grim` |
| `wtype` | `wtype` | Typing dictation, rewrites and Hands keys | `wtype` | `wtype` | `wtype` | `wtype` |
| `wl-clipboard` | `wl-paste` | The highlighted text for a rewrite in place, and a pasted picture in the app | `wl-clipboard` | `wl-clipboard` | `wl-clipboard` | `wl-clipboard` |
| `tesseract` | `tesseract`, `tesseract-ocr` | Reading words on screen for guides and select-and-ask; the second name is Void's executable | `tesseract`, `tesseract-data-eng` | `tesseract`, `tesseract-langpack-eng` | `tesseract-ocr`, `tesseract-ocr-eng` | `tesseract` |
| `espeak-ng` | `espeak-ng` | Oni's system voice | `espeak-ng` | `espeak-ng` | `espeak-ng` | `espeak-ng` |
| `pipewire` | `pw-cat` | The microphone and Oni's voice and sounds | `pipewire` | `pipewire-utils` | `pipewire` | `pipewire` |
| `pulseaudio-utils` | `parec`, `paplay`, `pactl` | The microphone list, volume commands and the PulseAudio fallback | `libpulse` | `pulseaudio-utils` | `pulseaudio-utils` | `pulseaudio` |
| `whisper-cpp` | `whisper-server` | Speech with the downloaded Whisper model | `whisper-cpp` | no matching executable package nominated | no package nominated | `whisper-cpp` |
| `ryoku-dictation` | `ryoku-dictate` | Speech with the default Parakeet model | Ryoku's repository only | Ryoku's repository only | Ryoku's repository only | not packaged |
| `sherpa-onnx` | `sherpa-onnx-offline-tts` | The optional Kokoro voice pack | no package nominated | no package nominated | no package nominated | `sherpa-onnx` |
| `uv` | `uv` | Setting up Hindsight, the optional memory engine, when asked | `uv` | `uv` | `uv` | `uv` |
| `llama-cpp` | `llama-server` | Running downloaded models; Hyakki can fetch llama.cpp instead | `llama-cpp` | `llama-cpp` | no package nominated | `llama-cpp` |

The Arch names were checked against the Arch `core` and `extra` sync databases on 2026-11-23. The previous private Nix evaluation resolved every nominated id against nixpkgs 26.11; this is attribute evaluation, not package-build or executable proof. The pinned source used for the resumed review is nixpkgs revision `56c02bc00adcf003215cc4bd996d6efaf4cff188`, including `pkgs/by-name/wh/whisper-cpp/package.nix` and `pkgs/by-name/sh/sherpa-onnx/package.nix`.

The Fedora names were read from Fedora's package catalogue on 2026-11-23: [Chromium](https://packages.fedoraproject.org/pkgs/chromium/chromium/), [Weston](https://packages.fedoraproject.org/pkgs/weston/weston/), [niri](https://packages.fedoraproject.org/pkgs/niri/niri/), [grim](https://packages.fedoraproject.org/pkgs/grim/grim/), [wtype](https://packages.fedoraproject.org/pkgs/wtype/wtype/), [wl-clipboard](https://packages.fedoraproject.org/pkgs/wl-clipboard/wl-clipboard/), [Tesseract](https://packages.fedoraproject.org/pkgs/tesseract/tesseract/), [English data](https://packages.fedoraproject.org/pkgs/tesseract-tessdata/tesseract-langpack-eng/fedora-44.html), [eSpeak NG](https://packages.fedoraproject.org/pkgs/espeak-ng/espeak-ng/), [PipeWire utilities](https://packages.fedoraproject.org/pkgs/pipewire/pipewire-utils/fedora-44.html), [PulseAudio utilities](https://packages.fedoraproject.org/pkgs/pulseaudio/pulseaudio-utils/), [uv](https://packages.fedoraproject.org/pkgs/uv/uv/) and [llama.cpp](https://packages.fedoraproject.org/pkgs/llama-cpp/llama-cpp/fedora-44.html). The last two detailed file lists confirm `pw-cat` and `llama-server`. Fedora's [whisper-cpp 1.8.1-2.fc44 file list](https://packages.fedoraproject.org/pkgs/whisper-cpp/whisper-cpp/fedora-44.html) contains libraries but no `whisper-server`, so the package is deliberately not suggested despite its matching name.

Void's nominated names were read from the official `void-packages` templates on the same date: [chromium](https://github.com/void-linux/void-packages/blob/master/srcpkgs/chromium/template), [weston](https://github.com/void-linux/void-packages/blob/master/srcpkgs/weston/template), [niri](https://github.com/void-linux/void-packages/blob/master/srcpkgs/niri/template), [grim](https://github.com/void-linux/void-packages/blob/master/srcpkgs/grim/template), [wtype](https://github.com/void-linux/void-packages/blob/master/srcpkgs/wtype/template), [wl-clipboard](https://github.com/void-linux/void-packages/blob/master/srcpkgs/wl-clipboard/template), [tesseract-ocr and its English subpackage](https://github.com/void-linux/void-packages/blob/master/srcpkgs/tesseract-ocr/template), [espeak-ng](https://github.com/void-linux/void-packages/blob/master/srcpkgs/espeak-ng/template), [pipewire](https://github.com/void-linux/void-packages/blob/master/srcpkgs/pipewire/template), [pulseaudio-utils](https://github.com/void-linux/void-packages/blob/master/srcpkgs/pulseaudio/template) and [uv](https://github.com/void-linux/void-packages/blob/master/srcpkgs/uv/template). Void renames the OCR binary to `tesseract-ocr`; Hyakki accepts that name only when `tesseract` is absent. No package nominated means no verified package is selected here, not that an upstream project or a third-party repository does not exist.

These are source and metadata checks, not installed-package acceptance. The private packaged-box procedure below must still build each recipe, resolve its required dependencies and exercise the resulting artifacts. Fedora's `libX11-devel` supplies both `x11` and `x11-xcb` pkg-config files, and Void's `libxkbcommon-devel` depends on its X11 split; those development requirements are not optional features. Each recipe requires Go 1.26 or newer. Nix includes the Vulkan loader as well as the headers.

Models and voice packs are not packages. Speech models are downloaded by the app after consent and the Kokoro pack is a folder the user adds; both are removed in the app or by deleting their folder, as [oni-licensing.md](oni-licensing.md) describes. There is no meta-package.

### Update and removal

An update replaces the binaries, units and templates and leaves user data, settings and enabled user units alone. Before an upgrade or downgrade, close the app and run `hyakki off` as each user, then follow any runit steps it prints. Do not replace binaries under a running daemon. Package hooks cannot safely stop every user's session and never try. `hyakki migrate` runs on the command line's first start after an update, not from a package hook.

Keep the previous native package, its matching source and a private backup of the Hyakki XDG directories and credential store before updating. Roll back using `pacman -U OLD.pkg.tar.zst`, `dnf downgrade ./OLD.rpm`, or `xdowngrade OLD.xbps` on Void. A Nix profile can use `nix-env --profile PROFILE --rollback`. These commands replace the program, not user data. If the newer program has changed a data format, restore the matching backup while Hyakki is off before starting the older version; an old binary alone is not a data rollback. Re-enable only the features and user services you want after checking the installed version.

Removal never touches `~/.config/hyakki`, the XDG state, cache and data folders or the credential vault. Before removing the package, run `hyakki off` as each user and follow its systemd or runit cleanup steps, including any copied runit service folders. The Arch and Fedora pre-removal hooks print a reminder, but no hook invokes a service manager, disables global units or edits another user's links. A package cannot promise to remove user-created service links it does not own. Optional programs are removed separately through the package manager, and no Hyakki recipe depends on them. Nix users remove an optional closure by rebuilding with that id omitted from `optional`; it is not a Hyakki feature switch or an automatic store garbage collection.

### Private packaged-box proof

`release/package-proof.py` installs real package artifacts, upgrades them, removes one nominated optional program, rolls back, removes Hyakki and reinstalls it. It does not build packages, inspect source text as a substitute for installation, pull an image or install missing dependencies. Run it only after the source freeze and native package builds. Both artifacts must be genuine builds with distinct version strings, not renamed copies of one package.

Use approved, already prepared x86-64 Linux images: an Arch image with pacman, a Fedora 44 image with rpm, a Void glibc image with xbps and xbps-rindex, and a Nix image with a writable local store and nix-env. Each needs Python 3.10+, strace and ldd. The native images must already have every hard runtime dependency from their recipe and a removable `wtype` package installed, but no Hyakki, user session, daemon or service supervisor running. Do not mount the host's home, display, D-Bus, devices, Docker socket or Nix store into the box. The Nix image must have a usable local store, and the supplied exports must contain the complete old and new closures. Default Nix builds use `optional = []`; proving the separately selected optional closures still needs their own feature runs.

The lifecycle driver requires a local immutable Docker image id, regular input artifacts no larger than 1 GiB each, and a new private evidence directory. It records the actual image size, artifact sizes and Docker storage free bytes; `--minimum-free-mib` is an optional operator-selected reserve, not a guessed universal image requirement. It fixes network access off, refuses pulls, limits each box to 3 GiB RAM with no swap, 2 CPUs, 512 PIDs, a 1 GiB temporary filesystem and 900 seconds by default (maximum 1800). Its OOM score is 1000; `--cpuset-cpus` can select the two CPUs. SYS_PTRACE is added only inside the disposable PID namespace for the real package transaction trace. Run boxes sequentially on the development machine.

`release/package-build.py` prepares an Arch, Fedora or Void dependency image from the hard dependencies in the frozen archive's actual recipe, then builds that same recipe in a separate network-disabled box. Preparation alone needs `--prepare`, network access and an already pulled base image. It records the installed package inventory, source hash, immutable input image, actual container writable-layer size and committed prepared-image id. The build uses a new writable `/work` bind for source, compiler cache and real package artifacts; the archive and driver are read-only. Arch runs makepkg as its unprivileged builder, Fedora runs rpmbuild, and Void runs xbps-src from official `void-packages` revision `988619731769a568ed4a48f6c688c3be23a4f165` with a prepared masterdir. Void's private chroot builder alone needs SYS_ADMIN and an unconfined seccomp profile inside its disposable container, not `--privileged` or host namespaces.

`--void-tree PATH` keeps the pinned prepared Void tree and masterdir outside the image layer. Preparation mounts that private tree writable; builds read it without changing it and work in a separate copy. The builder removes only copied download archives and the redundant initial source extraction, not installed dependencies or package indexes. `--prepare-network NAME` selects an operator-owned preparation network; native builds still run with networking off. Allow space for Docker's image commit as well as its running container: commit can temporarily hold both layers.

The official amd64 manifests were inspected without pulling on 2026-11-23:

| Base | Immutable amd64 manifest or local image id | Measured size |
| --- | --- | --- |
| `archlinux:base-devel` | `archlinux:base-devel@sha256:97119fd9e21219b02b3c14ba256c9d3c0d031d07fc26c836c41ed793beeb5b74` | 309262694 bytes of compressed image layers |
| `fedora:44` | `fedora:44@sha256:b189269dfee735c54ebcd3b4e43ff2dc0d4a4b07855468970217c9df57968eb0` | 69801374 bytes of compressed image layers |
| Existing Void glibc base | local `sha256:7de4630b6d3e0a76e9e68771837c15fc179e320de292b06e29c7292756d29583` | Docker reported 98 MB locally; dependencies not prepared |
| `nixos/nix:latest` | `nixos/nix@sha256:9b6a776642088021e9251e7cdb47988747008178ca2f7398cb3e1b365bde3372` | Manifest inspected only; no prepared closure-size measurement |

Compressed layer bytes are not expanded image or dependency-closure sizes. Do not infer that the matrix fits the development machine's roughly 6 GB free root filesystem from these numbers. Prepare one image at a time and use the runner's measured `container-size.json` and Docker image inspection before continuing. The measured `/dev/shm` mount is executable tmpfs, 21.5 GB total and 19.6 GB available at review time, so a private source/cache/artifact workspace there is viable without filling root. Its memory and per-user quota still apply, and tmpfs writes can count toward the box's 3 GiB limit. Image pulls, dependency layers, the Void masterdir image and a Nix store still use Docker storage; a `/dev/shm` work bind is not a replacement Docker data root or proof that a full Nix build closure fits.

After the source freeze, set `SOURCE` to the approved archive and prepare just the selected distribution. These commands deliberately separate the explicit pull from the runner, which never pulls:

```sh
ARCH_BASE=archlinux:base-devel@sha256:97119fd9e21219b02b3c14ba256c9d3c0d031d07fc26c836c41ed793beeb5b74
FEDORA_BASE=fedora:44@sha256:b189269dfee735c54ebcd3b4e43ff2dc0d4a4b07855468970217c9df57968eb0
# Choose one, only after reviewing available Docker storage
docker pull "$ARCH_BASE"
BASE_IMAGE=$(docker image inspect --format '{{.Id}}' "$ARCH_BASE")
python3 release/package-build.py --distro arch --prepare --image "$BASE_IMAGE" \
  --source "$SOURCE" --work /dev/shm/hyakki-arch-prep \
  --output /path/to/private-evidence/arch-prep
PREPARED_IMAGE=$(python3 -c 'import json,sys; print(json.load(open(sys.argv[1]))["prepared_image"])' \
  /path/to/private-evidence/arch-prep/result.json)
python3 release/package-build.py --distro arch --image "$PREPARED_IMAGE" \
  --source "$SOURCE" --work /dev/shm/hyakki-arch-build \
  --output /path/to/private-evidence/arch-build

docker pull "$FEDORA_BASE"
BASE_IMAGE=$(docker image inspect --format '{{.Id}}' "$FEDORA_BASE")
python3 release/package-build.py --distro fedora --prepare --image "$BASE_IMAGE" \
  --source "$SOURCE" --work /dev/shm/hyakki-fedora-prep \
  --output /path/to/private-evidence/fedora-prep
PREPARED_IMAGE=$(python3 -c 'import json,sys; print(json.load(open(sys.argv[1]))["prepared_image"])' \
  /path/to/private-evidence/fedora-prep/result.json)
python3 release/package-build.py --distro fedora --image "$PREPARED_IMAGE" \
  --source "$SOURCE" --work /dev/shm/hyakki-fedora-build \
  --output /path/to/private-evidence/fedora-build

BASE_IMAGE=sha256:7de4630b6d3e0a76e9e68771837c15fc179e320de292b06e29c7292756d29583
python3 release/package-build.py --distro void --prepare --image "$BASE_IMAGE" \
  --source "$SOURCE" --work /dev/shm/hyakki-void-prep \
  --output /path/to/private-evidence/void-prep
PREPARED_IMAGE=$(python3 -c 'import json,sys; print(json.load(open(sys.argv[1]))["prepared_image"])' \
  /path/to/private-evidence/void-prep/result.json)
python3 release/package-build.py --distro void --image "$PREPARED_IMAGE" \
  --source "$SOURCE" --work /dev/shm/hyakki-void-build \
  --output /path/to/private-evidence/void-build

# Nix, using pinned local nixpkgs and a separately cached build closure
nix-build --option substitute false --option builders '' --no-out-link \
  --expr '(import /path/to/pinned/nixpkgs {}).callPackage ./release/packages/nix/hyakki.nix {
    src = /path/to/hyakki-0.1.0.tar.gz; optional = []; }'
```

All work and evidence paths must be new, and each action is bounded to one hour, 3 GiB RAM, no swap, 2 CPUs and 512 PIDs. The runner leaves its named work directory and artifact for review but removes only its own container; it never removes another image, cache or workspace. Use a fresh path for another artifact. The source archive's version and recipe version must agree.

For the first private lifecycle proof, two independently compiled snapshots of the same reviewed product code may differ only in package/build version metadata, such as 0.1.0 and 0.1.1. Change the three native recipe version fields only in a private extracted source copy outside the checkout, create its matching archive with `release/native.py source --version`, and build each actual recipe with its real version flag. The same prepared dependency image can build both when dependencies are unchanged. Record both source and artifact hashes and label the result metadata-only private upgrade/rollback coverage, not migration from an older released product. Renaming one package is not an upgrade proof. A real older-data migration or production downgrade still needs the genuine older source/package and matching data backup.

With `SOURCE` naming the new frozen source archive, `OLD` and `NEW` naming real native packages, `OLD_VERSION` and `NEW_VERSION` their embedded version strings, and `IMAGE` the prepared image's `sha256:...` id, run the matching command:

```sh
python3 release/package-proof.py --distro arch --image "$IMAGE" \
  --old "$OLD" --new "$NEW" --source "$SOURCE" \
  --old-version "$OLD_VERSION" --new-version "$NEW_VERSION" \
  --output /path/to/private-evidence/arch

python3 release/package-proof.py --distro fedora --image "$IMAGE" \
  --old "$OLD" --new "$NEW" --source "$SOURCE" \
  --old-version "$OLD_VERSION" --new-version "$NEW_VERSION" \
  --output /path/to/private-evidence/fedora

python3 release/package-proof.py --distro void --image "$IMAGE" \
  --old "$OLD" --new "$NEW" --source "$SOURCE" \
  --old-version "$OLD_VERSION" --new-version "$NEW_VERSION" \
  --old-package-version "$OLD_XBPS_VERSION_REVISION" \
  --new-package-version "$NEW_XBPS_VERSION_REVISION" \
  --output /path/to/private-evidence/void

# Export each complete Nix closure from its build machine, not just the output
nix-store --export $(nix-store --query --requisites "$OLD_STORE") > old-closure.nar
nix-store --export $(nix-store --query --requisites "$NEW_STORE") > new-closure.nar
python3 release/package-proof.py --distro nix --image "$IMAGE" \
  --old old-closure.nar --new new-closure.nar --source "$SOURCE" \
  --old-store "$OLD_STORE" --new-store "$NEW_STORE" \
  --old-version "$OLD_VERSION" --new-version "$NEW_VERSION" \
  --output /path/to/private-evidence/nix
```

The driver retains the artifact/source hashes, immutable image inspection, real package-manager inventory, exec traces and step results in its private output. It checks installed executables and runtime linkage, compares the new installed grants byte-for-byte with the frozen source, verifies the old grants on rollback, preserves XDG user-data sentinels and a user-owned unit link, and checks dormant runit markers and units. Traced transactions must not launch Hyakki, enable or start a unit, or bring a runit service up. No application process or socket may survive a step. The sole application invocations are bounded `--version` calls for the CLI, daemon and app; no surface, microphone, screen capture or account is opened. Boxes are removed on success, failure or timeout.

This closes only the exercised package lifecycle rows. It does not prove a graphical first run, migration of a real user's older data, actual systemd/runit session activation, every optional engine, Void's OCR executable or Wayland/X11 behavior. Those need the separate frozen-binary, nested-session and real-engine gates. A source assertion or a successful container install is not their substitute.


### Flatpak

There is no Flatpak, and none is planned for this release, because:

- The daemon gives app-only settings and calls to a peer whose running binary is `/usr/bin/hyakki-app` or the `hyakki-app` beside the daemon (`internal/daemon/peer.go`). Inside a sandbox the daemon and app paths are the sandbox's, and a host daemon cannot verify them.
- Teams run agents under `bubblewrap`, which a Flatpak sandbox cannot nest without host privileges.
- Hands refuses Flatpak and snap browser launchers, and its separate desktop starts `weston`, `niri` and `grim` from the host.
- Dictation, rewrites and Hands type through `wtype` and read through `wl-paste`, and the voice loop runs `pw-cat`; Flatpak would need portal replacements for each, which do not exist in Hyakki.

A `.deb` is not supported either.

## Source archive

Create the corresponding source only after reconciling the canonical shared sources with `vendor/` and reviewing the staged source-file set. Maintainers update the versioned manifest from the Git index, not from an unrestricted working-tree traversal:

```sh
python3 release/native.py manifest
SOURCE_DATE_EPOCH=0 ./release/source-dist.sh \
  --version dev --output "$PWD/build/hyakki-dev-source.tar.gz"
```

`release/SOURCES` lists the approved paths. Missing or unexpected files beneath the distributed trees stop packaging, including ignored files; they are never silently added to the archive. Regenerate that list only after reviewing and staging new source files. Building an extracted source archive does not need Git.

The archive preserves the canonical repository layout under `hyakki-VERSION/`: `ryoku/hyakki/`, `docs/lithic/library/go/`, `ryoku/i18n/` and the repository `LICENSE` and `NOTICE`, plus the explicitly generated third-party notices. Public build, install and licensing guides are retained in `ryoku/hyakki/release/docs/`. Run the module's build/install tools from `ryoku/hyakki/` after extraction; recipes do the same. It includes vendored and local module sources and embedded assets with their grants. Raw research, assessment, private plans, Git metadata and Python bytecode are excluded. Output must be outside the archived source directories, and symlinks are rejected. Ordering, ownership, executable modes and timestamps are normalized; identical inputs and `SOURCE_DATE_EPOCH` produce identical archive bytes. Existing outputs are never overwritten.

The archive also includes the canonical native adapter modules
`ryoku/wm/oniwindow/` and `ryoku/wm/reflex/`, including their tests, fixtures and
licenses. Run `go test ./...` separately in both adapter directories as well
as the native main module; dependency-module tests are not discovered by the
main module's `./...`. Prebuilt `window.test` runs with `ryoku/wm/oniwindow`
as its working directory, not a generated vendor mirror.

No module cache or sibling checkout is part of the build contract. When modifying dependencies, regenerate vendor from the canonical shared sources before producing another source archive. The source gate rejects any byte drift in their vendored files; it never overwrites newer shared code. The distributor remains responsible for supplying any further corresponding source required by their modified binaries.

### Retaining the native Gio modification

The pinned Gio v0.10.3 source has a native Wayland activation modification so opening an existing Hyakki window does not require a Ryoku window-manager helper. `release/patches/gio-native-activation.patch` contains the complete change and generated protocol grant. After deliberately regenerating vendor from the contained modules, apply that patch from the source root before rebuilding:

```sh
GOWORK=off GOTOOLCHAIN=local go mod vendor
(cd ../.. && patch --batch --fuzz=0 -p1 < ryoku/hyakki/release/patches/gio-native-activation.patch)
```

Do not regenerate vendor as part of a normal contained build. Vendoring also restores an unused upstream `openapi-generator-cli.jar`; preserve it privately outside the distribution inputs rather than adding it to `release/SOURCES`. The strict source-input gate rejects unexpected files.

The activation bindings were generated with wayland-scanner 1.26.0 from `wayland-protocols/staging/xdg-activation/xdg-activation-v1.xml`, SHA-256 `d8418be2d5738d50aff788bef1c7574f33f26659aa045447ff2ef9b78c58fe01`. The header retains the pre-1.20 libwayland marshalling API used by the pinned Gio source; regenerating the bindings must retain that compatibility adjustment. `vendor/gioui.org/app/LICENSE.xdg-activation` preserves the authors' protocol grant and is included in the bundled notice inventory. Applying the patch to pristine pinned source has been checked to reproduce all seven changed or added files byte-for-byte.

Activation remains subject to compositor policy. A missing, expired or insufficiently authorized token does not grant focus-stealing rights.

## Owner verification

Run these only after all concurrent source edits have converged, in an isolated build environment, not against live units:

```sh
python3 release/native.py check-source
python3 release/native.py notices
./release/check.sh
HYAKKI_VERSION=dev ./release/build.sh
DESTDIR="$PWD/build/stage" ./release/install.sh
./release/check.sh "$PWD/build/stage"
```

The stage directory must be absent or empty before installation. `check.sh` checks shell and Python syntax, deterministic notice freshness, XML and desktop identities, and, when a stage is supplied, the payload and dormant-service contract. It never executes the application or starts a unit. Full application Go tests are a separate owner gate.

For reproducible corresponding-source verification, create two different output archives with the same `SOURCE_DATE_EPOCH`, compare them with `cmp`, extract one into a temporary directory and run its `release/build.sh` there with networking disabled and empty module/workspace caches. Run `desktop-file-validate` on the staged desktop file and `appstreamcli validate --no-net` on the staged metainfo when those validators are installed. A successful build does not replace actual Wayland and X11 runtime proof on the intended distributions.
