# Shima upstream runtime on Arch

Shima retains its existing `iris` identity and embedded iRiS scene. The optional
upstream runtime brings the iRiS, Material ii and Waffle panel families and
Ryoku's visualiser, wallpaper, launcher and settings integrations without
copying iNiR into the Ryoku repository.

The pinned source is iNiR 2.33.0 at commit
`48cdcecb0f6089cfbb7019054602eab28e154b3b`. The upstream archive,
integration patch and final runtime are checked against separate SHA-256
digests. The runtime is built once into a user-owned versioned generation,
not downloaded or patched when the desktop starts.

## Installation

Install or update the Ryoku packages first. The `ryoku-shell` package provides
`ryoku-shima` and `ryoku-shima-install`. The `ryoku-desktop` package provides
the pinned patch and source manifest at
`~/.config/quickshell/shima-overlay` after normal materialization.

Run `ryoku-shima-install install` as your normal user. Internet access is
needed only for the first verified archive download. For a cached archive,
use `ryoku-shima-install install --archive /path/to/archive.tar.gz`.
The archive is always digest-checked. `ryoku-shima-install verify` validates
the installed runtime, including all 2,408 expected runtime files.

The assembled source is in
`~/.local/share/ryoku/shima/generations/<revision>-<patch>`.
The `current` symlink is switched only after the complete build passes
integrity checks. An incomplete download or failed patch never replaces
an existing installation. Running the installer again reuses a verified
generation and does not contact the network.

The upstream installer, packaging files and systemd user units are not
installed. Shima is managed by the existing Ryoku shell daemon, which
starts, supervises and stops its process group.

## Activation and fallback

The complete runtime remains opt-in during Arch-native validation. Set
`RYOKU_SHIMA_UPSTREAM=1` in the **ryoku-shell daemon's** environment and
select the existing Shima barstyle in Ryoku Hub while using Niri. Without
the flag, without Niri, or without the installed runtime, the original
embedded iRiS scene remains available.

The standalone runtime has the same process readiness, switch-away and
crash recovery rules as the tested NixOS port. A failed session returns to
the preceding Ryoku barstyle and does not leave an independent iNiR service.

To revert, remove the opt-in environment flag and use Ryoku's normal
session management to restart the daemon. The original iRiS files and
`shell.json` are not overwritten. Standalone settings use
`~/.config/ryoku/shima/config.json`.

The full iNiR runtime also uses Qt image-format plugins, Kirigami and
KSyntaxHighlighting. Check these packages on Arch before enabling it.
They are not added as mandatory cross-edition dependencies.

## Verification

The repository's Shima source tests verify the pinned archive, installation
in a disposable XDG environment, corrupt-archive rejection, offline
reinstallation, and native fallback. The companion Go tests verify the
supervisor's readiness, switching and crash recovery rules.

The complete iNiR runtime was also exercised in an isolated Niri session:
all three panel families, Ricelin switching, overview wallpaper, family
launchers, switch-away, forced crash recovery and process cleanup.

A clean native Arch graphical session is still required before making the
new runtime the default or declaring the port release-stable. The old iRiS
remains available until that gate passes.
