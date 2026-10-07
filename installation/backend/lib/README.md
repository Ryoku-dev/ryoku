# installation/backend/lib/

The install steps, sourced by `ryoku-install`. Each file owns one concern and
exposes the functions the orchestrator calls in order. Every destructive or
system command routes through the `run` / `run_sh` / `run_secret` wrappers in
`common.sh`, so `RYOKU_DRYRUN=1` prints the whole plan without touching a disk.

## Per-file reference

The **sentinel** column is the `@@RYOKU_STEP <id>` the TUI watches while the
file's functions run; `-` means the file runs outside a sentinelled stage
(a helper library, a preflight gate, or a post-stage step).

| File | Sentinel | What it does | What it writes | Failure behavior |
|---|---|---|---|---|
| `common.sh` | - | `log`/`step`/`die`, the `run`/`run_sh`/`run_secret` dry-run wrappers, `write_file`/`append_file`/`deploy_dir`, the `part_dev`/`part_num`/`dev_uuid` device helpers, and `ryoku_drop_pkg`/`ryoku_drop_from` (the app, browser, and shell-stack removals every package-set reader filters through). | Nothing of its own; `write_file` writes the caller's target. | Library. `die` aborts non-zero; `run` prints under dry-run. |
| `preflight.sh` | `preflight` | Gates root, UEFI, Secure Boot, disk size and repo payload. Alongside also requires GPT, an identifiable existing ESP and `2 + 20 + swap` GiB free; auto mode shares an ESP with 8 MiB free or selects a dedicated Ryoku ESP. | Nothing. | Fatal `die`; dry-run narrates and returns. |
| `disk.sh` | `partition` | Creates whole-disk ESP + root, or alongside 2 GiB boot + root at explicit free sectors. Alongside boot is EA00 XBOOTLDR in shared mode and EF00 ESP in dedicated mode. | GPT table; sets `ESP_DEV` / `ROOT_PART`. | Fatal; proves new partitions before formatting. |
| `luks.sh` | `partition` | Optional LUKS2 on the root partition; opens it as `/dev/mapper/root`. | LUKS2 header on `ROOT_PART`; sets `ROOT_DEV` / `LUKS_PART`. | Fatal on an unsafe or missing root target. |
| `filesystem.sh` | `filesystems`, `mount` | Formats the new boot FAT and Btrfs root, creates subvolumes, mounts under `/mnt`, and builds optional swap. | Filesystems, mounts and swapfile. | Fatal. |
| `pacstrap.sh` | `pacstrap` | Assembles the package set (base + profile hardware + dev + Broadcom when a `14e4:` device is present), drops what `RYOKU_DROP_PACKAGES` names, waits on the keyring, `pacstrap -K`, writes `fstab`. On failure regenerates the mirrorlist from the next tier and retries once with `--needed`. | The base system into `/mnt`; `/mnt/etc/fstab`. | Fatal `die` only after the `--needed` retry also fails (message lists the tiers tried + the failing mirror URL). |
| `mirrors.sh` | `pacstrap` | Bounded four-tier mirror prep before `pacstrap`: reflector, else the mirror-status API, else the shipped list, plus emergency mirrors appended to every tier; also sets `ParallelDownloads`, `DisableDownloadTimeout`, and `ILoveCandy` in the install-time `pacman.conf`. | The live `/etc/pacman.d/mirrorlist` and `/etc/pacman.conf`, in place. | Best-effort; never aborts. Any failure keeps the shipped list. |
| `chroot.sh` | `configure` | In-target config: locale, keymap, timezone, hostname, a primary user with the selected login shell, sudo, initramfs `HOOKS`, crypttab; owns the pacman-hook masks (snap-pac, `80-limine-efi-deploy`, the mkinitcpio pair) and restores them before we finish. | `/mnt/etc/*` (`locale.conf`, `vconsole.conf`, `hostname`, the user, `sudoers.d`, `mkinitcpio.conf.d/ryoku.conf`, `crypttab`), `/mnt/etc/pacman.d/hooks`. | Fatal for config; refuses to create the user when the selected shell binary is absent from the target. |
| `deploy.sh` | `configure` | Adds `[ryoku]`, copies the live mirrorlist, imports the release key, installs the desktop packages, runs `ryoku materialize`, records `RYOKU_DROP_PACKAGES`, and seeds unpackaged user data plus the first-login default-rice marker. | `/mnt/etc/pacman.conf`, the target keyring, the desktop packages, `~/.config`, the provisioning ledger, `~/.local/state/ryoku/default-rice-pending`, and user-data seeds. | Desktop-package failures and the marker write are fatal. Optional payload sources may be absent; all created user-state paths are handed to the user before login. |
| `network.sh` | `configure`, `preflight` | Carries the live wifi into the target (iwd backend pin + `.nmconnection` keyfiles); and, at preflight, gates DNS and HTTP reach before the disk is touched. | `/mnt/etc/NetworkManager/conf.d` + `system-connections`; the live resolver (heal only). | `ryoku_network` best-effort; `ryoku_ensure_dns`/`ryoku_ensure_mirrors` are fatal at preflight (before the disk), by design. |
| `drivers.sh` | `configure` | Copies each per-vendor GPU driver script into the target and runs it in the chroot; the scripts self-gate on the detected GPU and are idempotent. Offline, it hands them `RYOKU_PACMAN_CONF` (the `[offline]`-only config) so their `pacman -S` resolves. | Driver packages + (via `nvidia.sh`) the NVIDIA early-KMS `MODULES` drop-in. | Fatal wrapper, but the scripts self-gate so running all of them is safe. |
| `bootloader.sh` | `bootloader` (+ post-AUR finalize) | Builds Limine and the kernel menu. Whole/dedicated mode installs directly to Ryoku's ESP; shared mode installs a static ESP hop to the XBOOTLDR. Existing systems are GUID-chainloaded. | `/boot`, optional `/efi/EFI/ryoku`, optional `/etc/ryoku/limine-existing-esp`, NVRAM entry. | Fatal for files/config; NVRAM registration is best-effort. |
| `aur.sh` | - | Bootstraps `yay` (AUR clone, else the GitHub release binary), builds `aur.packages` (minus `RYOKU_DROP_PACKAGES`) as the unprivileged user; points xdg HTTP, HTTPS, HTML and the desktop browser role at `RYOKU_BROWSER`. | The AUR packages; `~/.config/ryoku/desktop.json`; a transient NOPASSWD sudoers drop-in + `resolv.conf`, both removed after. | Best-effort: always returns 0. Offline or a failed build is a warning. |
| `snapshots.sh` | - | Snapper `root` config (written by hand, no `snapperd`), snap-pac registration, the cleanup timer, `limine-snapper-sync`. | `/mnt/etc/snapper/configs/root`, `/etc/conf.d/snapper`, the service enables. | Best-effort; `limine-snapper-sync` enable warns if absent. No `@snapshots` subvol = no-op. |

## Wave-2/3 helpers, by owning file

- **reclaim** -- `disk.sh` `ryoku_reclaim_leftovers`: before measuring free space,
  `alongside` deletes any *unmounted* partitions labeled exactly `ryoku`/`ryokuboot`
  from a prior failed run, in one atomic `sgdisk -d`, so retries never stack.
- **secureboot** -- `preflight.sh` `ryoku_secureboot_enabled`: reads the last byte
  of the `SecureBoot` efivar; preflight dies when it is on (Limine is unsigned)
  unless `RYOKU_ALLOW_SECUREBOOT=1`.
- **clock-skew** -- `network.sh` `ryoku_fix_clock_skew`: when a mirror probe fails,
  reads the server clock over an unverified `curl` and, if the system clock is off
  by more than a day, sets it from the `Date` header and re-probes (best-effort).
- **vmd** -- `bootloader.sh` `ryoku_boot_vmd`: when the live kernel has the Intel
  VMD module loaded (`/sys/module/vmd`), writes `MODULES+=(vmd)` into the target
  initramfs so the installed system can still find its NVMe at boot.
- **resume** -- `bootloader.sh` `ryoku_cmdline` and `chroot.sh` `ryoku_cfg_initramfs`:
  with a swapfile, adds `resume=`/`resume_offset=` to the cmdline and the `resume`
  hook after `encrypt` in `HOOKS`, so hibernation works.

## Trap and failure protocol

`ryoku-install` sets an EXIT trap that acts only on a non-zero status (a clean
finish is untouched, and `@@RYOKU_DONE` still prints only on success). On failure
it names the stage from `RYOKU_STAGE` (which `step` records), restores the masked
snap-pac hooks, flushes to disk, and -- unlike the clean path -- **leaves `/mnt`
mounted** so the partial tree and `/var/log/ryoku-install.log` can be inspected.
