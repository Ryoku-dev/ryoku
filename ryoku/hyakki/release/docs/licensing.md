# Licensing the native distribution

The root `LICENSE` is the verbatim GNU General Public License version 3. Ryoku's retained `NOTICE` identifies GPL version 3 without an or-later grant, so the combined Hyakki binaries linking its code are described as `GPL-3.0-only`.

The Gio application links the contained Lithic and i18n modules. The CLI and daemon also use i18n. A binary combining these GPLv3-only modules with the permissively licensed components cannot be distributed as Apache-only. The AppStream `project_license` therefore describes the combined application as GPL-3.0-only. This describes the combination, not a replacement grant for each included source file.

`LICENSE.apache-2.0` and the complete imported `NOTICE.upstream` remain unchanged. Existing third-party copyrights, grants and attributions still apply, including Prowl, Hindsight, the bundled embedding model, Codex, mem0, Letta, Letta Code, built-in skill sets, Material Symbols and fonts. External tools and services are not bundled by the native build and retain their own distributions and terms. No Nixi implementation or assets, and no Nūs marks or artwork, are part of this distribution.

Oni's guide, voice and consent patterns are adapted from Nūs Companion by Pranav Desu, Apache-2.0; the root `NOTICE` carries that attribution and the trademark statement. Oni's shipped character files are original and declared with their license, provenance and digest in `release/oni-assets.json`. [oni-licensing.md](oni-licensing.md) records the component matrix, the artwork and voice decisions, and the checks that keep the declarations matched to the tree. Its voice and dictation table lists every speech engine, model, voice and program Oni can run or download; none is bundled, and the root `NOTICE` carries their credits and the CC BY 4.0 attribution for NVIDIA Parakeet.

The newly added native packaging scripts carry `SPDX-License-Identifier: GPL-3.0-only`. AppStream metadata declares its own metadata license separately from the combined application's license. No contributor identity or permission to use another project's marks is implied by these files.

## Deterministic notices

`python3 release/native.py notices` collects retained legal files from `vendor/`, embedded source under `internal/` and the canonical shared modules at `../../docs/lithic/library/go/`, `../i18n/`, `../wm/oniwindow/` and `../wm/reflex/`, then writes `release/notices/THIRD_PARTY_NOTICES`. It includes the full original `NOTICE.upstream`, the Apache grant and the Go standard library notice. The input paths and their SHA-256 digests are recorded in sorted order. The inventory deliberately includes all vendored-source notices, even when that source is not linked into each executable.

Each vendored module must have a retained license in its vendored subtree or its canonical local-module source ancestry. Missing module grants fail generation instead of being guessed. Subdirectories' copyright and asset notices are included as well. The Go notice is frozen from the Go 1.26.0 source LICENSE at <https://raw.githubusercontent.com/golang/go/go1.26.0/LICENSE>, so generation does not depend on whether a distribution's installed Go toolchain includes that file.

`python3 release/native.py notices --check` compares the exact expected bytes and fails if the output is missing or stale. It neither contacts providers nor accesses a module cache. Regenerate after dependency, embedded-asset or notice changes, and include the generated file in the corresponding-source archive and binary payload.

## Installed grants

The native installer stages these in `/usr/share/licenses/hyakki/`:

| File | Contents |
| --- | --- |
| `LICENSE` | GPL version 3 |
| `NOTICE` | Hyakki's combination and distribution notice |
| `NOTICE.upstream` | Complete unchanged imported `NOTICE.upstream` |
| `LICENSE.apache-2.0` | Unchanged imported Apache 2.0 text |
| `THIRD_PARTY_NOTICES` | Full deterministic vendor, embedded-source and local-module inventory |
| `LICENSE.material-symbols` | Material Symbols Apache 2.0 grant |
| `LICENSE.geologica` | Geologica SIL Open Font License |
| `LICENSE.martian-mono` | Martian Mono SIL Open Font License |

The original module and asset grants remain in their corresponding-source locations. Package maintainers should include this entire installed license directory, not reduce the distribution to the `project_license` field or the root GPL text.

## Optional programs

The packages declare optional programs only as optional dependencies, listed with their reasons in `release/optional.json` and [native-distribution.md](native-distribution.md). None is bundled, linked or started at install. Oni's and Hands' programs and their licences are in [oni-licensing.md](oni-licensing.md). The two that serve Hyakki itself are:

| Program | License | Hyakki's use | Delivery |
| --- | --- | --- | --- |
| uv (`uv`) | MIT OR Apache-2.0 | Sets up Hindsight, the optional memory engine, only when the user asks | The user's distribution; not bundled |
| llama.cpp (`llama-server`) | MIT | Runs downloaded models on this computer; without it Hyakki can fetch llama.cpp when the user asks | The user's distribution or Hyakki's on-request download; not bundled |

## Corresponding source

`release/source-dist.sh` includes the runnable application, canonical module replacements, vendor sources, embedded assets, build and staging scripts, instructions and retained licenses. It does not include research downloads, assessment copies, prebuilt executables or a developer's module cache. Distributors must pair their binaries with the matching source and build information under the applicable license terms; creating an archive alone is not evidence that a particular binary was built from it.

See [native-distribution.md](native-distribution.md) for the exact commands and the distinction between source/staging checks and real desktop runtime verification.
