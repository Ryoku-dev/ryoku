# installation/tests/

Install tests that prove a real Ryoku install succeeds before a user hits a
broken one. Run by the Install test workflow (after a Build ISO, weekly, and on
demand); see `docs/updates.md` for the delivery contract they guard.

- `container-install.sh [arch|cachyos]` builds the Ryoku packages from the
  checkout, installs `ryoku-desktop`, runs `ryoku materialize` as a throwaway
  user, and asserts the materialized `~/.config` is complete. Fast and hermetic
  (a container, no VM). Catches a config that reaches no install and a package
  whose dependencies do not resolve.

- `install-vm.py --iso <iso>` boots the ISO in QEMU, runs the installer
  unattended against a virtual disk (driving the live root shell over the serial
  console), waits for `@@RYOKU_DONE`, then mounts the installed root and asserts
  the tree (the package, the materialized config, the bootloader, and the
  greeter). `--encrypt` enables LUKS root encryption; `--passphrase` selects the
  passphrase and defaults to the layout-safe value `123`. The encrypted
  verification opens the LUKS volume and checks both `crypttab` and the Limine
  `cryptdevice=` argument.

  `--boot-installed` boots the target disk after verification and passes only
  when `ryoku-test login:` appears on its serial console. For an encrypted
  target it waits 45 seconds, records unlock screendumps, and types the
  passphrase through QEMU's monitor. `--keypad` uses `kp_0` through `kp_9`
  instead of digit-row keys and requires a digits-only passphrase.
  `--wrong-first` submits an incorrect passphrase, records `unlock-wrong.ppm`,
  then retries with the real passphrase. `--prompt-wait` changes the initial
  delay, and `--boot-timeout` changes the 600 second login deadline.

  `--payload-dir <checkout>` packs that checkout's `installation/backend` and
  `system` directories, serves them to the live guest, and overlays both the
  `/usr/share/ryoku` payload and the backend used by `ryoku-install`. This lets
  an ISO exercise installer changes from a working tree without rebuilding the
  image. `--boot-only` just reaches the live shell; `--dry` runs the installer
  in `RYOKU_DRYRUN` mode. Uses KVM when `/dev/kvm` is present, else TCG. Needs
  `qemu`, `edk2-ovmf`, and `python-pexpect`.

- `iso-stage-check.sh` stages the ISO profile twice (`iso/build.sh --stage-only`
  into two throwaway dirs) and diffs the trees, proving the prebuilt binaries and
  the baked payload are byte-reproducible for a fixed commit (see
  `iso/README.md`, "Reproducibility"). It strips the `.payload` provenance stamp
  before diffing, and skips cleanly (exit 0) when `go`/`cmake`/`ninja` are absent
  so CI without the build toolchain stays green.

- `iso-preflight.sh` is the gate a Build ISO run clears BEFORE mkarchiso starts
  (its own job in `build-iso-reusable.yml`, and the same command on a dev box
  before dispatching a build). Root-free and hermetic, ~10s: shell syntax +
  ShellCheck over the installer, package-list checks, the offline-install
  regressions (baked-repo resolution, pacman-hook masking, driver-package
  coverage), the boot-menu fixtures, the installer contract suite, the TUI build
  and unit tests, and the update-delivery check. It exists because these classes
  have shipped: an ISO takes an hour or two and then reaches users.

- `install-dualboot-vm.py --iso <iso>` is the real-Windows dual-boot regression
  gate. It builds a CACHED golden Windows 11 image (a genuine ESP+MSR+C:+WinRE
  layout with C: pre-shrunk 300 GiB, unallocated in the MIDDLE of the disk),
  overlays it, runs the `alongside` install from `<iso>`, then asserts via
  `qemu-nbd` that every pre-existing partition is byte-identical (start/end/
  PARTUUID/typeGUID + edge sha256), the Windows ESP/NTFS filesystems are intact,
  and BOTH boot legs work (Ryoku via serial, Windows chainload via OVMF +
  screendump OCR). Answers "does our installer damage a real Windows disk?" with
  evidence. Golden + Windows ISO are cached under `cache/` (gitignored);
  overlays are per-run and deleted. `--golden-only` builds just the cache;
  `--skip-golden` reuses it. Needs `qemu`, `edk2-ovmf`, `python-pexpect`,
  `tesseract`, `ntfs-3g`, and root (nbd + mounts).
