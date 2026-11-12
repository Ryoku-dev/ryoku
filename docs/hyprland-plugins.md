# Hyprland compositor plugins

A compositor plugin is a `.so` Hyprland dlopens into itself: title bars, glass,
image borders, cursor motion, focus flash, key sounds. Ryoku bundles six as
`[ryoku]` packages, Ryoku Settings manages every one of them (and any the user adds)
on Windows > Window manager, and the machinery below keeps them loading across Hyprland
updates. Shell plugins (the Quickshell widgets under `docs/plugins.md`) are a
different thing: they run in the shell, not the compositor.

## The ABI lock, and the receipt beside every copy

A plugin is compiled against the exact compositor build: Hyprland's commit plus
the major.minor of aquamarine, hyprutils, hyprgraphics, hyprcursor and
hyprlang. Hyprland bakes that string into itself (`hyprctl version -j` reports
it as `abiHash`) and into every plugin (`__hyprland_api_get_client_hash`), and
refuses a plugin whose string differs: `version mismatch, built against: ...,
running compositor: ...`. Arch can bump any of those libraries between two
Ryoku releases, and then a copy that loaded yesterday does not load today.

So every copy Ryoku builds carries a receipt: `<name>.abi` beside `<name>.so`,
the ABI string it was compiled for. The `[ryoku]` packages write it from the
build host's `version.h` (the same formula, in `release/packages/*/PKGBUILD`),
and the local builder writes it from the installed headers. With the receipt,
Ryoku Settings, `settings.lua` and the doctor can all tell a stale copy from a working
one without loading it.

Three places can hold a copy, and `ryoku-hub` resolves them in this order:

| Tier | Path | Written by |
|---|---|---|
| built | `~/.local/lib/hyprland/plugins/<id>.so` (+ `.abi`, `<id>.json` receipt) | `deploy.sh` on a checkout, Rebuild and Add on Ryoku Settings > Windows > Window manager, the doctor |
| package | `/usr/lib/hyprland/plugins/<id>.so` (+ `.abi`) | the `[ryoku]` package |
| hyprpm | `~/.local/share/hyprpm/<repo>/<id>.so` | hyprpm, by the user's own hand; read-only here |

The generated `settings.lua` loads the first copy whose receipt matches the
running compositor (the installed headers when no compositor answers); a copy
with no receipt is taken on trust. When every copy is stale the load is left
out with a comment pointing at Ryoku Settings > Windows > Window manager, so
Hyprland does not refuse it with a notification on every reload.

`hl.plugin.load()` only declares a path: Hyprland loads the declared set after
the whole config pass and then reloads once more. So each plugin's
`hl.config({ plugin = … })` runs behind `ryoku_plugin_loaded(name)`, a lookup
in `hl.get_loaded_plugins()` by the name the plugin reports, which is true on
that second pass (setting a plugin key that is not registered is no Lua error;
it paints Hyprland's "unknown config key" overlay). Each save additionally pushes
the same config through `hyprctl eval` after the reload, so a plugin that has
just come up holds the user's values, and the live preview pushes it on every
edit, so a profile or volume change is heard at once. Hyprland unloads a
plugin it loaded from config once the config stops naming it, but not one the
roster probe loaded by hand, so a save that turns a plugin off unloads it
explicitly as well.

## Plugins in Ryoku Settings

Ryoku Settings > Windows > Window manager (`ryoku-shell hub open
window-manager`) has a Plugins section on Hyprland. Its status comes from
`ryoku-hub desktop plugins list`: source and version (package, built here with
commit and date, hyprpm), whether it is running, and the verdict:

- **Running**: loaded in the compositor.
- **Ready** / **On at start**: installed and matching; loads once it is turned
  on, or at the next login.
- **Needs rebuild**: every copy's receipt disagrees with the compositor, named
  ("built for aquamarine 0.14, running 0.15"). Rebuild fixes it here.
- **Failed to load**: the compositor refused it for another reason, quoted.
- **Not installed**: no copy anywhere.

An enabled plugin that is installed and not stale is loaded by the list call
itself, which is what a reload would do, so the row carries the compositor's
own verdict rather than a guess.

Actions: **Docs** opens the plugin's own documentation; **Rebuild plugins**
builds them here (bundled plugins from their upstream repository, keysounds
from the checkout or the copy `ryoku-keysounds` ships), every stale one or a
single plugin; **Remove a plugin you added** drops one the user added. A box
without a toolchain (`git`, `make`, `c++`, `pkg-config`, `cmake`, the
`hyprland` headers) is told what to install; base-devel and git are in the
shipped set, so a stock Ryoku box builds. Plugins you added get a section of
their own with each one's switch and settings.

A bundled plugin's settings live on the page they belong to, so each row is
written once (`registry/provider-map.js` places them): key sounds on Desktop >
Sounds, focus flash on Appearance > Animations, title bars, glass and image
borders on Appearance > Window style, cursor motion and touch gestures on
Devices > Mouse & touchpad. They are written to `settings.lua` like every
other window-manager setting.

## Adding a plugin from git

**Add a plugin from a repository** (under the Plugins section's More options)
takes any repository with a `hyprpm.toml`, the manifest hyprpm reads. Looking
it up clones it into `~/.cache/ryoku/hypr-plugins-src/`
and lists the plugins it declares (one already bundled is named, not offered);
building it checks out the commit the manifest's `commit_pins` pair with
the installed Hyprland (the default branch's tip when there is no pin, as
hyprpm does), runs the manifest's build steps against the installed headers,
and lays the `.so` with its `.abi` and a receipt under the built tier. The
receipt records the repository, commit, build date and the plugin's settings.

Nothing is enabled behind the user's back: the new plugin loads only once it is
turned on. The build steps are the repository's own, run on
this machine, so the page says to add only repositories you trust. hyprpm
itself is not used: it clones and builds Hyprland's own source for headers and
rewrites the system `hyprland.pc` with sudo, where the Arch `hyprland` package
already ships the headers a plugin build needs.

### Settings, detected

A plugin registers its options as `plugin:<ns>:<key>` string literals, which
survive compilation verbatim, so the builder reads them out of the `.so`. Once
the plugin is loaded, `hyprctl getoption` types each one (bool, int, float,
string, colour) and gives its default; the receipt keeps what it learned. The
page renders a switch for a bool, a stepper for an int, a slider for a float, a
field for anything else, under the key's own namespace, with the raw key and
default in the description. A value the user sets is stored in `hypr.json`
under `plugins.extra.<id>.config` by its full path (`hyprexpo:columns`) and
emitted as nested Lua tables (`plugin = { hyprexpo = { columns = 3 } }`), so a
plugin nobody in Ryoku has seen still gets a native control for every option.

## Key sounds

`ryoku/hyprland/plugins/keysounds/` is the one compositor plugin authored in
this repo: a sound on every key press through libcanberra, from a profile of
samples. `main.cpp` is the plugin; the eleven shipped profiles are recordings
of real switches (Cherry MX blue/brown/black/red, Topre, NK Cream, Holy Panda,
Tealios, the "creamy" lubed linear, two Everglides) cut at build time from the
MIT-licensed Mechvibes packs at a pinned commit, one per line of
`ryoku/keysounds/profiles.txt`, by `ryoku/keysounds/ryoku-keysounds-import`,
the same tool a user runs on any pack from mechvibes.com to get a profile of
their own. `hyprpm.toml` is the recipe the builder runs (`make keysounds.so`),
and the README documents the settings, the profile format and where a user's
own samples go (`~/.local/share/ryoku/keysounds/<name>/`).

The samples and the importer are their own package, `ryoku-keysounds-profiles`,
installed under `/usr/share/ryoku/keysounds/`; the plugin package
(`ryoku-keysounds`) ships only the `.so` and its rebuild source under
`/usr/share/ryoku/hypr-plugins/keysounds/`, and depends on the profiles. niri
has no plugin system, so it reuses the same samples through
`ryoku/keysounds/main.rs`: a small standalone engine (evdev in, libcanberra
out, no crates) that the niri provider writes its settings for, shipped as the
package `ryoku-keysounds-niri`. Both read the one copy of the samples on disk.
The Hyprland package arrives with the Hyprland desktop; niri installs its
engine on demand from Ryoku Settings > Desktop > Sounds.

The plugin looks for a profile in the user's `~/.local/share/ryoku/keysounds/`,
then beside its own `.so` (where a local build lays it), then the packaged
`/usr/share/ryoku/keysounds/`. The user's guide is `ryoku/keysounds/usage.md`.

## Convergence: the doctor, deploy.sh, the packages

- `ryoku doctor` (which `ryoku update` runs) has a `Hyprland plugin builds`
  reconciler: every enabled plugin whose receipts no longer match the installed
  headers is rebuilt through `ryoku-hub desktop plugins rebuild --stale`, so a
  Hyprland bump taken with `ryoku update` is converged before the next login. A
  disabled stale plugin costs nothing and is left to the page. Without a
  toolchain it warns and names the packages to install.
- `deploy.sh` runs the same builder (`--stale --checkout <repo>`) instead of
  makepkg: a checkout rebuilds only what its receipts say is missing or stale,
  and keysounds when its source changed.
- The packages pin to the release (`ryoku-desktop` depends on
  `<plugin>=$pkgver`) and rebuild on every publish against the build host's
  Hyprland. Between publishes an Arch bump leaves them stale; the receipt makes
  that visible and the local rebuild covers it until the next release ships.

## The backend

```
ryoku-hub desktop plugins list                                   the roster, JSON
ryoku-hub desktop plugins rebuild [--all|--stale] [--checkout <dir>] [<id>...]
ryoku-hub desktop plugins add [--inspect] <git-url> [<plugin>...]
ryoku-hub desktop plugins remove <id>
```

`rebuild` and `add` log to stderr and print a JSON summary
(`{"built","skipped","failed"}`) on stdout. The bundled registry, the receipts
and the roster live in `ryoku/hub/backend/hyprplugins.go`; the builder in
`hyprplugins_build.go`; the store fields (`plugins.keysounds`,
`plugins.extra`) and the Lua emission in `hypr.go`.
