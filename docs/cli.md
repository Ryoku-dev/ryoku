# The `ryoku` command

The user-facing control CLI (`ryoku/cli/`, one Go program). It is the front door
to updates, rollback, status, and the shell; it orchestrates the host's package,
service, and snapshot tools rather than reimplementing them. This reference
explains what each command does, where it is meant to run, and who runs it.

## Two worlds

Every command behaves with respect to one of two ways Ryoku can exist on a
machine. Most confusion about the CLI comes from mixing them up.

- **A packaged install** the normal case. The desktop is installed from the
  `[ryoku]` pacman repo; `ryoku-desktop` ships the base config under
  `/usr/share/ryoku/config`, and there is no git checkout and no build toolchain.
- **A dev checkout** a clone of this repo on a maintainer's machine. There is no
  `/usr/share/ryoku/config`; the desktop is laid down from the checkout by
  `ryoku/shell/deploy.sh` (via `ryoku deploy`). The deploy records the checkout
  root and the commit it laid down under `~/.local/state/ryoku/`, which is how
  the deployed `ryoku` binary later finds the repo again.

`ryoku update` auto-detects which world it is in (a recorded checkout means the
git path, otherwise the pacman path), so the same command is correct on both. A
few commands belong to only one world; that is called out per command below.

## At a glance

|Command|What it does|Where|Who|
|---|---|---|---|
|`update`|Snapshot when supported, then bring the system current and redeploy|both|you|
|`status [--json]`|Version, how far behind, snapshot count when supported|both|you, the Hub|
|`rollback [id]`|List releases; on snapshot-capable hosts, guide a boot-menu restore|both|you|
|`snapshots`|List snapshots when the host provides them|both|you|
|`reload`|Restart the shell and reload Hyprland|both|you|
|`materialize`|Lay the base configs into `~/.config`|packaged install|the updater/installer|
|`reset [path]`|Drop a `user_edits` override, back to the Ryoku default|both|you|
|`deploy`|Build and lay the desktop from a checkout|dev checkout|a maintainer|
|`recovery`|Last resort: reset to main and redeploy|both|you, when broken|
|`doctor`|Run idempotent reconcilers for stateful drift|both|you, the updater|
|`plugin <cmd>`|Install, remove, list, or validate a shell plugin from git|both|you, an agent|

## Everyday commands

These are user-facing and work on any install.

### `ryoku update`

On pacman hosts, the full safe update is wrapped in a best-effort snapper
pre/post pair: an unconfigured snapper never blocks the update, but a failed
step aborts before anything else changes. On hosts without the snapshot stack,
the snapshot step prints one warning and the update continues without invoking
snapper. What the update runs next depends on the world:

- **Dev checkout:** updates through the git channel. It fetches the channel
  branch (`main` for everyone), fast-forwards the checkout when it is sitting
  cleanly on that branch, and redeploys with `deploy.sh`. A feature branch or a
  dirty tree is left to git only the redeploy runs.
- **Packaged install:** the packages the `[ryoku]` repo serves, by name
  (`pacman -Sy`, then `pacman -S --needed ryoku/<pkg>...`), then
  `ryoku materialize`, then a shell reload. It is not a sysupgrade: the base
  system and its kernel come from Arch or CachyOS, and `sudo pacman -Syu` is
  what moves them. The run reports how many packages that lane is holding, and
  `ryoku update --system` runs it too (the full `pacman -Syu`, `yay -Sua`, and
  `flatpak update`) for a box that wants one command.

On a terminal the run is a curated console: a header, one line per step with
its time and anything it found, a live line for the step in flight, a progress
bar, and a closing card. Everything the steps and their tools print (pacman,
git, the builds, the doctor) goes to `~/.local/state/ryoku/update-log.txt`
instead; the console surfaces only `error:`, `warning:` and `note:` lines, a
`.pacnew`, and the doctor's findings. On failure the card shows the error and
the last lines the work printed. When a pre-update snapshot exists, it also
offers that snapshot for rollback.

- `ryoku update -v` (`--verbose`) streams the raw output instead, pacman's own
  progress bars included.
- `ryoku update --gui` is the Hub's path: it starts the run in the background
  under a pseudo terminal and returns. The password sudo needs and any question
  come back through the Hub's Updates page (`--auth` hands it the password on
  stdin), and `--cancel` stops a run: what it quiesced is given back, and a
  package transaction already committing finishes on its own. Ctrl-C on a
  terminal run stops it the same way.

Throughout, it publishes progress to `$XDG_RUNTIME_DIR/ryoku-update.json`:
the steps with their timings, the line the running work last printed, and a
watchdog read of its own process tree every two seconds (output or CPU time
is progress; three minutes of neither is a stall). Only a run that has begun
writes the file, and it names its pid, so the Hub never sits on a run that is
gone.

### `ryoku status [--json]`

A read-only report. It always prints the active config base. On a checkout it
shows the channel, the deployed commit (`installed`), and how many commits behind
the channel you are; on a packaged install it shows the installed `ryoku-desktop`
version, what the `[ryoku]` repo offers, and the count of pending package
updates (via `checkupdates` from `pacman-contrib`), listed as `system:` because
`ryoku update` does not take them. It ends with the snapshot count.

`--json` is the data seam the Hub and the update island read; it is not meant for
humans.

### `ryoku rollback [id]`

With no arguments, this lists published Ryoku releases and, when the host
provides them, snapshots. `ryoku rollback --to <tag>` moves the Ryoku package
set to a published release without a reboot. `ryoku rollback <id>` guides a
whole-system snapshot restore from the boot menu.

Ryoku boots the `@` subvolume directly (`rootflags=subvol=@`), a layout
`snapper rollback` cannot restore because it flips the btrfs default subvolume,
which a pinned `subvol=` ignores. The restore therefore runs from the boot
menu: reboot, boot the snapshot under the Limine Snapshots menu, and run `sudo
limine-snapper-restore` there. It copies the booted snapshot and its matching
kernels on the ESP back onto `@`.

On a host without Ryoku's snapshot stack, bare `rollback` still lists releases
and explains that snapshots are unavailable. Supplying a snapshot id returns
that explanation as an error without invoking snapper.

### `ryoku snapshots`

List the snapper snapshots on supported pacman hosts. On hosts without the
snapshot stack, the command prints the same availability reason and exits with
an error without invoking snapper.

### `ryoku reload`

Restart the shell and reload Hyprland, by handing off to `ryoku-shell reload`.
Use it after changing config that is already in place.

### `ryoku reset [path]`

Drop a user override and go back to the Ryoku default. With a path (relative to
`~/.config`, e.g. `hypr/modules/binds.lua`) it resets that file; with none, and
after a confirm (`-y` skips it), the whole `user_edits` overlay. It removes only
overlay files, not Ryoku Settings' stores. On a packaged box it re-lays the base
at once; on a dev checkout it drops the override and leaves the re-lay to
`ryoku deploy`.

## Production internals

### `ryoku-host`

`ryoku-host` is the internal operating-system seam used by scripts, the Hub,
and the CLI. Package names passed to it use the Arch/Ryoku catalogue; the host
backend translates them for XBPS when needed. The host-facing verbs include:

- `capabilities` prints the detected package manager and init system plus
  snapshot and AUR support as one JSON object.
- `pkg why <name>` explains a catalogue package that deliberately has no Void
  equivalent. A package that can be installed, an unknown name, or any name on
  a pacman host produces no output and returns the false condition exit code.
- `pkg advice <name>...` prints the pacman or XBPS install command for those
  catalogue names.
- `pkg available [--aur] <name>...` checks the native repositories. On pacman
  hosts, `--aur` marks catalogue names as available through Ryoku's AUR tooling;
  on XBPS it uses the same translated repository check as the unflagged form.
- `pkg install [--upgrade] [--aur] [--overwrite GLOB]... <name>...` installs
  packages. `--overwrite` is repeatable and passes through to pacman and AUR
  helpers; XBPS accepts and ignores it because XBPS has no equivalent.
- `power <poweroff|reboot|suspend|hibernate>` runs the action through systemd
  or elogind.
- `time zones` prints the sorted IANA zone list. It uses `timedatectl` under
  systemd and the installed zoneinfo database on other init systems.

### `ryoku materialize`

Lays Ryoku's base configs into `~/.config`, declaratively and override-safe. It
copies every file the package ships under the base config dir (clobbering the
previous Ryoku copy), prunes files a past release shipped but this one dropped
(tracked by a manifest at `~/.local/state/ryoku/materialized`), and never touches
files the package never shipped your own overrides like `hypr/user.lua`,
`hypr/monitors_user.lua`, `kitty/user.conf`, `fish/user.fish` are left alone.

After the base is laid, materialize overlays your edits: every file under
`~/.config/ryoku/user_edits` (mirroring `~/.config`) is copied on top, so a file
there wins at its mirrored path. The overlay is sparse, so a new base file still
lands; a whole-file fork shadows its base copy (visible in the overlay, `ryoku
reset` hands it back). See `docs/updates.md`.

Per-machine generated drop-ins (`hypr/monitors.lua` written by `ryoku-monitor`,
`hypr/gpu.lua` by `ryoku-gpu`) are seeded only when absent and never clobbered or
pruned, so an update refreshes the shipped config without ever resetting your
display layout, GPU pin, or any setting.

The base dir is `/usr/share/ryoku/config`, shipped by the `ryoku-desktop`
package. This command is the packaged-install replacement for `deploy.sh`'s
config copy: it is run for you by `ryoku update` and by the installer's deploy
step, not usually by hand. **On a dev checkout it fails** (`base config dir not
found: /usr/share/ryoku/config`) because that path only exists once the package
is installed; use `ryoku deploy` there instead. To point it at a base tree
yourself, set `RYOKU_CONFIG_BASE`.

### `ryoku owner <path>...`

Answers the question a coding agent (or you) has before touching a config file:
who writes this, and where does my change belong so an update does not overwrite
it. It classifies each path against the same materialize contract an update
follows, so the verdict always matches what actually happens on the next update.
A path can be absolute, `~/...`, or relative to `~/.config`.

Each path prints its class (`overlay`, `forked`, `generated`, `store`, `seed`,
`user-override`, `ryoku`, `user`, `package`, `unmanaged`), who writes it, and the
file to edit instead, if any. `--json` prints the same as an array for a tool to
read; `--map` prints a markdown ownership summary of the whole machine, which is
what the Rashin vault carries so every wired agent knows the rules.

```
$ ryoku owner ~/.config/kitty/kitty.conf
/home/you/.config/kitty/kitty.conf
  class:   ryoku
  writer:  the ryoku-desktop package, through materialize
  advice:  Ryoku ships this and overwrites it on every update. Put your change in ~/.config/kitty/user.conf instead.
  edit:    /home/you/.config/kitty/user.conf
```

## Keyring

### `ryoku keyring`

Chooses how the GNOME keyring unlocks your saved passwords and secrets at
sign-in, so browsers and apps stop prompting. **The default is `never-ask`: out
of the box no app ever prompts.** First login runs `ryoku keyring init`, which
records the mode and seeds a blank, passwordless default keyring; from then on
every libsecret app (browsers, editors, the SSH agent) uses it silently, and it
persists across reboots. Three modes:

- `never-ask` (default) the default keyring is blank (stored in plaintext), so it
  is already unlocked and nothing ever prompts. Works the same under SDDM
  autologin or a password login, and is seeded automatically at first login.
- `unlock-on-login` PAM unlocks (or creates) the login keyring with your login
  password at sign-in; the store stays encrypted at rest and the desktop never
  prompts. Opt in for an encrypted store. The default keyring points at `login`.
- `ask` the store stays locked until an app asks, and gnome-keyring prompts then.

`ryoku keyring init` is the first-login default, run from each compositor's
autostart. It is idempotent: it records the inferred mode and seeds the
never-ask keyring, is a no-op once you have chosen a mode, and never destroys a
pre-existing encrypted keyring (it records the policy and points you at
`set --reset` instead).

`ryoku keyring status [--json]` reports the configured (or, when unset, inferred)
mode, whether `/etc/pam.d/sddm` carries `pam_gnome_keyring`, whether autologin is
configured, whether the keyring daemon is running, and each keyring file's format
(encrypted, plaintext, or absent). `ryoku keyring set <mode>` records the choice
in `~/.config/ryoku/keyring.json`, converges the keyring files over D-Bus, and
escalates the root PAM edit through `pkexec` (`--convert` rekeys an encrypted
keyring, reading passwords on stdin; `--reset` backs the files up and starts
fresh). The Hub's Lockscreen page drives all of this; `ryoku doctor` watches for
drift. `$RYOKU_PAM_FILE` overrides the PAM path for tests.

## Developer-only commands

### `ryoku deploy`

The dev loop, never used on production installs. It builds the Go binaries and
the QML plugin and lays the repo into `~/.config` from a checkout, by running
`ryoku/shell/deploy.sh`. It requires `RYOKU_REPO` to point at the checkout and
errors otherwise. The deploy also records the checkout root and the commit it
laid down under `~/.local/state/ryoku/`, which is what lets `status` and `update`
track the git channel afterwards.

## Emergency command

### `ryoku recovery`

The last resort when the desktop is too broken to update normally: it resets the
checkout to `main` and redeploys, **overwriting configs**. It hands off to
`bin/ryoku-recovery`, preferring the copy in a local checkout and otherwise
fetching the canonical script from the repo over the network, so it still works
when the local build is broken.

## Self-healing

### `ryoku doctor`

Runs the convergent reconcilers: idempotent checks (and, where it is safe, fixes)
for stateful drift the package and config layers cannot express. Each reconciler
reports `ok` when the machine matches the desired state, otherwise it converges,
proposes the exact fix, or flags it for a human. A healthy machine is quiet;
`--check` (or `-n`) shows the full list without changing anything, and `--json`
emits the findings as a machine-readable array (name, status, detail, remedy)
for a GUI. `ryoku update` runs `ryoku doctor` itself, so healing is seamless and
a finding never aborts the update.

Current reconcilers (in `ryoku/cli/internal/doctor/`): swap kept out of snapshots,
snapper config consistency, stale pacman lock, the ryoku package channel + keyring,
desktop session components, the keyring unlock policy (how the GNOME keyring
unlocks at sign-in; see `ryoku keyring`), Hyprland config integrity (revalidates and repairs the
generated monitors.lua/gpu.lua drop-ins so a corrupt one cannot strand the desktop
backlight (catches a missing interface, missing brightnessctl, or a hybrid-GPU
firmware-only backlight), the pacman progress bar (seeds Ryoku's `ILoveCandy`
default into `/etc/pacman.conf` once, so deleting the line sticks), pending
`.pacnew` config, and orphaned packages.
Reconcilers retire once every supported install has run them, so the set stays
small rather than growing like an ordered migration list.

**When doctor cannot fix something** (or finds an unknown problem), it writes a
single shareable text report and points you to it. Generate one any time with
`ryoku doctor --report [file]`: it bundles the findings with system state (btrfs
usage and device errors, `/proc/swaps`, failed units, recent journal errors,
pacman state, the ryoku channel state, session env, and hardware: backlight
devices, GPU drivers, kernel cmdline, recent display-driver log) into one `.txt`
the maintainers can read. It contains no passwords or keys.

**When you want it to reason, not just match rules**, `ryoku doctor --explain`
sends that same report to a cloud model and prints its root-cause analysis and fix
steps. This is the long tail the reconcilers cannot pre-encode: it reasons over
the evidence the way a human would, then tells you what to change (including
hardware/BIOS causes it cannot touch). It is strictly **advisory and read-only**,
it never runs anything, so a wrong answer can only mislead. It is opt-in and uses
**your own key**: nothing is sent unless you set one. Defaults target Groq (free,
fast); OpenRouter's free models work by overriding the URL and model. See the
environment variables below.

## Plugins

### `ryoku plugin`

Install, remove, list, and validate shell plugins: small widgets a user drops
into the desktop (see `docs/plugins.md`). Ryostore stays the curated front;
`ryoku plugin add` is the open door for any git repo.

- `ryoku plugin add <git-url> [--bar] [--yes]` clones the repo to a staging dir,
  validates its `manifest.json`, and, only if it holds together, installs it with
  `ryostore install plugins <id> --from <dir>`. Going through the store's install
  transaction (rather than a plain copy) is what lets the shell load it: the
  transaction writes the receipt, the content-hashed view, and the journal that
  `discover.sh` requires, into `~/.local/share/ryoku/plugins/<id>` and
  `~/.local/state/ryoku/store`. Validation refuses a manifest whose id is
  malformed, is a reserved built-in widget id, or is already installed, whose
  entry points are absolute, escape the tree with `..`, or do not exist, whose
  hosts are not a subset of `framePopout`, `desktopWidget`, `topbarGlyph`, or
  whose tree contains a symlink. A plugin runs unsandboxed inside your shell, so
  the command prints a warning and asks for confirmation (`--yes` skips the
  prompt; a non-interactive run without it declines). `--bar` enables it on the
  bar. It never executes anything from the plugin.
  `ryoku plugin add https://github.com/you/weather-card --bar`
- `ryoku plugin remove <id>` uninstalls through `ryostore remove plugins <id>`
  (dropping the receipt, view, and files together) and clears the plugin's
  placement so the shell stops loading it. `ryoku plugin remove weather-card`
- `ryoku plugin list [--json]` lists installed plugins merged with their
  placement; each row carries a `source` of `store` (a receipt-owned install) or
  `dev` (a `RYOSTORE_PLUGINS_DIR` override, which shadows the store). `--json` is
  the data seam for agents and Rashin. `ryoku plugin list --json`
- `ryoku plugin validate <dir>` runs the same manifest checks against a local
  tree without installing it, for plugin authors. `ryoku plugin validate .`

## The bar and dock CLI

The bar layout is data: `shell.json` holds `qsbar.layout` as three ordered lists
of stable widget ids (`left`, `center`, `right`), where built-in widgets and
installed store plugins are entries of the same list. The daemon owns
`shell.json`, so these verbs live on `ryoku-shell`, not `ryoku`. Every widget id
comes from one catalogue file the shell, the settings panel and this CLI share,
and every mutation is validated against it (an unknown id, an unknown setting
key, or an option that is not in the list is an error, never a silent write) and
written through the settings store. A script, a keybind, or an agent drives the
bar without touching JSON.

### `ryoku-shell bar`

- `bar list [--json]` every widget with its section, index, and shown flag.
  `ryoku-shell bar list --json`
- `bar catalog [--json]` the built-in catalogue merged with installed
  bar-capable plugins and their settings schema. `ryoku-shell bar catalog --json`
- `bar move <id> --section <left|center|right> [--index N | --before <id> |
  --after <id>]` reorder a widget within a lane or move it across lanes.
  `ryoku-shell bar move clock --section right --after media`
- `bar show <id>` / `bar hide <id>` toggle a widget's visibility (a built-in
  keeps its place and stops rendering; a plugin is enabled or disabled on the
  bar). `ryoku-shell bar hide battery`
- `bar set <id> <key> <value>` set one of a widget's own settings, validated
  against the catalogue (built-in) or the plugin manifest.
  `ryoku-shell bar set launcher launcherLogoText arch`
- `bar position top|bottom` move the bar to the top or bottom edge.
  `ryoku-shell bar position bottom`
- `bar form full|fit|dock|notch|islands` set the shell shape the bar takes.
  `ryoku-shell bar form islands`
- `bar defaults` restore the shipped layout and visibility, leaving presentation
  keys (separators, density, per-widget colour) untouched. `ryoku-shell bar defaults`
- `bar settings [route|close]` open QS Bar Settings on the active monitor,
  optionally on a named route (`bars`, `layout`, `widgets`, `dock`, `community`); `close`
  puts it away. `ryoku-shell bar settings layout`

### `ryoku-shell dock`

The dock is a first-class shell surface on the edge opposite the bar; its state
is the top-level `dock` object in `shell.json`.

- `dock show` / `dock hide` turn the dock on or off. `ryoku-shell dock show`
- `dock edge auto|top|bottom|left|right` anchor the dock (`auto` tracks the edge
  opposite the bar). `ryoku-shell dock edge bottom`
- `dock autohide on|off` keep the dock as a peek strip until hovered.
  `ryoku-shell dock autohide on`
- `dock pin <app-id>` / `dock unpin <app-id>` add or remove a pinned app.
  `ryoku-shell dock pin org.kitty`
- `dock list [--json]` report the dock's enabled, edge, autohide, and pinned
  state. `ryoku-shell dock list --json`

## Environment and state

Overrides (mostly for `deploy` and tests):

- `RYOKU_REPO` the checkout root. Required by `deploy`; also lets `status`,
  `update`, and `recovery` find the repo explicitly.
- `RYOKU_CONFIG_BASE` overrides the `materialize` base dir
  (default `/usr/share/ryoku/config`).
- `RYOKU_CHANNEL` overrides the update-channel branch (default `main`; used by
  tests).

AI reasoning for `doctor --explain` (opt-in; the report is sent only when a key is
set). Any OpenAI-compatible endpoint works:

- `RYOKU_AI_KEY` your provider key, or write it to `~/.config/ryoku/ai-key`.
- `RYOKU_AI_URL` the API base (default `https://api.groq.com/openai/v1`; for
  OpenRouter use `https://openrouter.ai/api/v1`).
- `RYOKU_AI_MODEL` the model (Groq default `llama-3.3-70b-versatile`; an
  OpenRouter free model is e.g. `meta-llama/llama-3.3-70b-instruct:free`).

State the CLI keeps under `$XDG_STATE_HOME/ryoku` (default `~/.local/state/ryoku`):

- `repo` the checkout root the last deploy recorded.
- `deployed` the commit the last deploy laid down; the baseline `status`
  measures the channel against.
- `materialized` the manifest of Ryoku-owned files, so the next `materialize`
  can prune cleanly.
- `doctor-report.txt` the latest diagnostic report `doctor` wrote (also the
  default target of `ryoku doctor --report`).

Runtime: `$XDG_RUNTIME_DIR/ryoku-update.json` is the update's progress file,
written by `update` and drawn by the Hub's Updates page.
