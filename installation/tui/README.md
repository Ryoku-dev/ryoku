# installation/tui/

`ryoku-tui` is the Ryoku installer front-end: a full-screen terminal app that
collects the install choices and hands them to the `ryoku-install` backend. It is
a Go program built on **Bubble Tea v2** (`charm.land/bubbletea/v2`), with
`lipgloss/v2` for styling, `harmonica` for the spring animations, `qrterminal`
for the on-screen QR codes, and `sahilm/fuzzy` for the picker filters. There is
no Go toolchain on the ISO; `iso/build.sh` ships the binary prebuilt.

## The four files

- **`main.go` is pure UI.** Screens, layout, the wizard state machine, the
  palette and glyphs, and all the layout math. It never shells out.
- **`system.go` is the only file that touches the machine.** The live lists for
  the pickers (keymaps, locales, time zones, disks, Wi-Fi), hardware detection,
  the small live actions (apply a keymap, join Wi-Fi), and the streamed handoff
  to the backend. Every `exec.Command` lives here.
- **`password.go` hashes the password**, in-process: sha512-crypt (`$6$`) written
  against its specification, so the one step of the wizard that cannot fall back
  needs no tool, no `PATH` and no readable live medium. It used to run
  `openssl passwd -6`, and a live session where that child process could not run
  left a user stuck at the password screen with a perfectly good password.
  `password_test.go` pins it to the published vectors and to the C library's own
  `crypt(3)`, which is what verifies the hash at every later login.
- **`apps.go` is the product-choice table**: the three browsers, the three
  login shells and their package stacks, the keep/remove app rows (packages,
  defaults, and the REQUIRED reason each required row refuses to drop), the
  flattened checklist rows the `apps` step renders, and the summary Review
  shows. Membership follows the release's own tables (`ryokumanifest.Apps()`,
  the feature map in `tests/shell-tool-availability.sh`); hard depends of
  `ryoku-desktop` are deliberately absent because they back required surfaces.

`system.go`'s `installEnv` builds the `RYOKU_*` environment the backend reads;
that contract is documented in `../backend/README.md`.

## The step flow

The wizard (`steps()` in `main.go`) walks these steps in order:

`keyboard -> locale -> timezone -> network -> hardware -> profile -> gpu ->
compositor -> browser -> login-shell -> apps -> diskpick (target disk) ->
disk (strategy) -> partitions (layout) -> hostname -> username -> password ->
encryption -> review`

`system.go` reads `/usr/share/ryoku/variant` before the wizard is built. The
Arch flow picks Hyprland or niri, Firefox, Chromium, or Zen, and Fish, Zsh, or
Bash. The Void flow keeps the same steps and backend environment contract, but
greys out Hyprland with its packaging reason, offers niri, Firefox or Chromium,
and omits AUR-only apps and the snapshot toggle. All three login shells and the
four hardware profiles remain available from the Void package table.

Some steps are conditional: the `gpu` (graphics mode) step matters only on a
hybrid iGPU + dGPU laptop, and the pickers fall back to a small built-in list
when the live tools return nothing, so the wizard renders anywhere. `review` is
the last safe point; nothing is written until it launches the backend, whose
streamed output drives the install screen and its staged rows.

## Safety gates

The TUI refuses to hand a layout to the backend that the backend would reject or
that would destroy data unintentionally:

- **BIOS is a hard block.** `detectHardware` sets `hwBIOS` when `/sys/firmware/efi`
  is absent; the hardware step then will not advance (the backend is UEFI-only).
  The card shows the "disable CSM / enable UEFI" guidance.
- **Secure Boot blocks Review.** `secureBootEnabled` reads the last byte of the
  `SecureBoot` efivar; when it is on, `reviewBlockReason` blocks the install
  (Limine is unsigned). Mirrors the backend's preflight gate.
- **Live-medium exclusion.** `liveDisk` resolves the disk backing the archiso
  boot medium and `excludeDisk` hides it (plus zram / loop / `sr` / eMMC boot
  areas) from the target-disk picker, so the installer never offers to erase the
  stick it booted from.
- **Wipe acknowledgement.** A `whole` strategy on a disk that already holds
  partitions requires the user to type `ERASE` on Review (`wipeStage`); only then
  does `installEnv` emit `RYOKU_WIPE_CONFIRMED=1`, which the backend's
  `ryoku_partition_whole` demands. A blank disk skips the extra step. The
  strategy picker also lists the non-destructive "alongside" first so a quick
  Enter is never a wipe.
- **Offline-first, online fallback.** An Arch ISO is offline-ready when its baked
  repository contains `offline.db`; a Void ISO uses
  `/run/initramfs/live/ryoku/repo` when it contains `x86_64-repodata` and XBPS
  packages. `installEnv` emits the selected path with `RYOKU_ONLINE=0` and
  `RYOKU_OFFLINE_REPO`, using the same backend contract for both. An image
  without a usable repository gates Review on `netOnline()` and emits
  `RYOKU_ONLINE=1`.

## Layout math, and how it mirrors the backend

The constants in `main.go` keep the TUI's free-space gate in lockstep with the
backend, so `Tab` never advances a layout that `ryoku-install` would reject
mid-install:

- `minDiskGiB = 32` -- the target-disk floor; matches preflight's 32 GiB check.
- `minRootGiB = 20` -- the minimum root partition; matches the backend's
  `ryoku_min_root_gib` (`20 + swap`).
- `availRoot()` -- the root size: the free region (alongside) or the disk minus
  kept partitions (whole), less the boot partition. Alongside always reserves
  2 GiB; it is an XBOOTLDR when the existing ESP has at least 8 MiB free and a
  dedicated Ryoku ESP otherwise.
- `swapCeil()` -- caps swap at 64 GiB and always leaves at least `minRootGiB` of
  usable root, so swap can never starve the system partition.
- alongside `partReady` requires `minRootGiB + 2` GiB free, matching the backend.

## Running it off-ISO

Normal launch needs an interactive terminal:

```
cd installation/tui
go run .
```

To eyeball the screens on any machine without a TTY, a live disk, or the ISO,
use the built-in snapshot mode. `main()` dispatches `os.Args[1] == "snapshot"`
to `snapshot()`, which renders a fixed sequence of frames to stdout, each under a
`### <title> ###` header:

```
go run . snapshot
```

It renders the welcome screen (plain and with the social QR), the network step
(connected and offline), hardware (detected and the graceful fallback), the
graphics-mode step, the window-manager picker, browser picker, login-shell
picker, apps keep/remove checklist (defaults and a curated slim install), the
target-disk and partition steps, the password step (with the strength meter),
the done screen, and the install and failure screens. The
install/failure frames use `stepLog` sample output; a live install streams the
real backend output instead. Because the pickers fall back to built-in lists,
the snapshot renders even where the live tools (`localectl`, `lsblk`, ...) return
nothing.

## Tests

Pure-function unit tests, no machine or ISO required:

```
cd installation/tui
go test ./...
```

`partition_test.go` covers the layout math and the safety gates (root carves
swap, the alongside free-space floor, the wipe-confirm plumbing);
`done_test.go` covers the done-screen exit action; `password_test.go` covers the
sha512-crypt hash the whole install hangs on (the published vectors, the block
boundaries, the shape `chpasswd -e` needs, a fresh salt per call, and agreement
with `crypt(3)` where a Perl is around to reach it); `apps_test.go` covers the
product choices (ordered browser and login-shell choices, their defaults,
required rows refusing to toggle, and `installEnv` carrying `RYOKU_BROWSER`,
`RYOKU_LOGIN_SHELL`, and every losing browser, shell stack, and deselected app
in `RYOKU_DROP_PACKAGES`).

The shell package policy is exact. Fish keeps `fish`; Zsh keeps `zsh`,
`zsh-autosuggestions`, `zsh-history-substring-search`,
`zsh-syntax-highlighting`, and `ryoku-oh-my-zsh`; Bash keeps `blesh`. Bash
itself and the shared terminal tools remain installed for all three choices.
