# Installing on real hardware

VMs install clean. Metal is where installers die, and the failures cluster into
a handful of classes that a live-USB installer has to handle or explain. This is
the durable home of that research: per class, the symptom a user sees, the cause,
what the Ryoku installer now does automatically, and what the user still has to
do in firmware or Windows when automation cannot reach it.

The backend that implements the automatic half lives in
`installation/backend/lib/`; the front-end gates in `installation/tui/`.

## Contents

- [Intel VMD / RST hides the NVMe](#intel-vmd--rst-hides-the-nvme)
- [Secure Boot vs unsigned Limine](#secure-boot-vs-unsigned-limine)
- [NVIDIA black or garbled screen](#nvidia-black-or-garbled-screen)
- [Dual-boot (Windows or another Linux)](#dual-boot-windows-or-another-linux)
- [Broadcom Wi-Fi](#broadcom-wi-fi)
- [ASUS Aura keyboard lighting](#asus-aura-keyboard-lighting)
- [Xbox controller will not pair over Bluetooth](#xbox-controller-will-not-pair-over-bluetooth)
- [Bluetooth adapter is detected but never comes up](#bluetooth-adapter-is-detected-but-never-comes-up)
- [Xbox Wireless Dongle is not supported](#xbox-wireless-dongle-is-not-supported)
- [A game freezes when you switch workspace](#a-game-freezes-when-you-switch-workspace)
- [Only 2.4 GHz networks appear, or 5 GHz will not connect](#only-24-ghz-networks-appear-or-5-ghz-will-not-connect)
- [RTC clock skew vs pacman signatures](#rtc-clock-skew-vs-pacman-signatures)
- [NVRAM-readonly firmware](#nvram-readonly-firmware)
- [Ventoy and other loop-mounted media](#ventoy-and-other-loop-mounted-media)
- [Slow or flaky USB media](#slow-or-flaky-usb-media)

## Intel VMD / RST hides the NVMe

**Symptom.** The target-disk step is empty, or the install completes and then
drops to an emergency shell on first reboot because it cannot find its own root.
Common on Intel laptops.

**Cause.** Intel Volume Management Device (the "VMD" / RST mode in firmware)
hides the NVMe behind a controller the kernel cannot see without the `vmd`
module. If the module is needed on the live system, it is needed in the installed
initramfs too, or the target loses its own disk at boot.

**What the installer does.** When the live kernel has VMD loaded
(`/sys/module/vmd`), `bootloader.sh` writes `MODULES+=(vmd)` into the target
initramfs before it is built, so both the UKI and the `mkinitcpio -P` paths bake
it in. If the disk list is empty, the TUI's disk hint names VMD as the likely
cause instead of a generic "no disk".

**What the user must do.** If the live installer itself sees no disk, the live
kernel did not load `vmd`. On a Ryoku-only machine, switch the firmware storage
mode from RST/VMD to AHCI and retry. On a machine that also runs Windows, do not
switch to AHCI (Windows installed under VMD will fail to boot): instead leave VMD
on and boot the installer so the `vmd` module loads and gets carried into the
target automatically.

## Secure Boot vs unsigned Limine

**Symptom.** The installer refuses to start with a "Secure Boot is enabled"
message; or, if that gate is bypassed, the installed system dies at a security
violation on first boot.

**Cause.** Limine ships unsigned. Firmware enforcing Secure Boot will not run an
unsigned bootloader.

**What the installer does.** The TUI blocks the Review screen when
`secureBootEnabled()` reports Secure Boot on, and the backend's preflight dies
before touching the disk, both with "disable Secure Boot in firmware setup"
guidance. `RYOKU_ALLOW_SECUREBOOT=1` overrides it for a user who has enrolled
their own keys.

**What the user must do.** Enter firmware setup, disable Secure Boot, reboot the
installer. Set the override only if you have enrolled your own keys and know
Limine will pass.

## NVIDIA black or garbled screen

**Symptom.** The screen goes black or garbled at boot, live or installed, on some
NVIDIA and hybrid laptops.

**Cause.** Kernel mode-setting and the NVIDIA modeset handoff. Some panels need
mode-setting off to reach the installer at all; the installed NVIDIA system needs
it on.

**What the installer does.** The live ISO's default entry boots with KMS on, and
ships a "safe graphics" entry that adds `nomodeset` for machines that come up
black. The installed `amd-nvidia` profile gets `nvidia_drm.modeset=1` on its
kernel cmdline (`ryoku_cmdline`), and the NVIDIA early-KMS `MODULES` drop-in is
written only when the driver module actually built.

**What the user must do.** If the live screen is black or garbled, pick the
**safe graphics (nomodeset)** boot entry. The installed system is configured for
NVIDIA already; if it comes up black, boot the Limine fallback and add `nomodeset`
to the cmdline for that boot to get in and investigate.

## Dual-boot (Windows or another Linux)

**Symptom.** An alongside install fails partway through (out of space); or not
enough free space to start; or the other OS will not boot after the install.

**Cause.** OEM ESPs are often only 96-260 MiB. Ryoku needs 8 MiB of free
headroom to share one safely; a nearly full Windows ESP must not be resized,
moved, or cleaned by guessing at Microsoft files.

**Not Windows-only.** The strategy keeps any existing OS and uses its EFI
loader for the Limine menu when one is available. The TUI names Windows only
when it finds NTFS.

**What the installer does.** Alongside always creates a 2 GiB FAT32 boot
partition plus the Btrfs root in the chosen free region. The boot mode is
automatic:

- With at least 8 MiB free on the existing ESP, the new FAT partition is an
  XBOOTLDR. The existing ESP is backed up, then Ryoku writes only
  `/EFI/ryoku/*` there as a static hop to the XBOOTLDR.
- With less than 8 MiB free, the new FAT partition is a dedicated Ryoku ESP.
  Loader, menu, kernels and initramfs stay on it; the existing ESP is never
  mounted read-write or modified.

Before either mode touches the disk, two gates protect the neighbouring OS. A
hibernated or Fast-Startup-dirty Windows volume stops the install outright
(override with `RYOKU_ALLOW_DIRTY_NTFS=1`): writing a partition table under a
dirty NTFS volume is what invites Windows Startup Repair to rewrite that table
on its next boot, taking the fresh Ryoku partitions with it. And in shared mode
the existing ESP is checked with `fsck.fat -a` before Ryoku writes a single byte
into it; a volume the repair cannot clear stops the install with the option to
switch to `RYOKU_ESP_MODE=dedicated` instead.

Both modes register the Ryoku NVRAM entry against the partition that holds its
loader. Windows or another Linux remains in its existing firmware entry and is
also added to the Limine menu by its ESP partition GUID when a chainloadable EFI
binary is found; managed entries are restored after menu updates. Verified
failed-run partitions labeled `ryoku` or `ryokuboot`
still require the TUI's typed `ERASE` acknowledgement before reclamation.

The free-space requirement remains `2 + 20 + swap` GiB: 2 GiB boot plus the
20 GiB root floor.

**What the user must do.**

- **Make room first.** Either leave unallocated space, or let the installer carve
  it: the layout step can shrink one selected partition (NTFS, ext2/3/4, btrfs)
  in place. For Windows, shrinking from within Windows first (Disk Management ->
  Shrink Volume) is still the safest route; for a Linux neighbour, unmount it and
  let the carve step take the space.
- **BitLocker.** Suspend BitLocker in Windows before you install. Two things trip
  it: changing the partition table can force a recovery-key prompt on the next
  Windows boot, and chainloading through Limine changes the measured boot path,
  so a BitLocker-on machine prompts for the recovery key on every chainloaded
  boot until BitLocker is suspended and re-sealed (Windows re-seals on its own
  next boot). Suspend it before installing, or skip the chainload and boot
  Windows from the firmware boot menu, whose loader path is unchanged and never
  prompts.
- **Fast Startup.** Disable Windows Fast Startup (hybrid shutdown). It leaves the
  disk in a hibernated state that can lock filesystems and confuse dual-boot.
- **Boot order after install.** The installer registers its NVRAM entry and sets
  it first for the next boot. If the firmware resets the order (or ignores NVRAM,
  see below), enter firmware setup and put the Ryoku / Limine entry first.
- **If the Limine chainload boot-loops.** Some firmware will not chainload
  Windows cleanly and loops back to the menu. The Windows loader and its own
  NVRAM entry are untouched, so pick **Windows Boot Manager** directly from the
  firmware boot menu (usually F12 / F9 / Esc at power-on) to boot Windows; use
  that as the everyday Windows path if the chainload never settles.
- **After a Windows feature update.** A major update can reset boot order or
  drop the Ryoku NVRAM entry. The fallback loader remains on the volume selected
  during install. Re-register the matching path (`lsblk -o NAME,SIZE,PARTLABEL`
  shows the partition number):

  ```
  # shared mode: the existing Windows ESP
  efibootmgr --create --disk /dev/nvme0n1 --part 1 \
    --loader '\EFI\ryoku\BOOTX64.EFI' --label 'Ryoku'

  # dedicated mode: the new 2 GiB ryokuboot ESP
  efibootmgr --create --disk /dev/nvme0n1 --part N \
    --loader '\EFI\limine\limine_x64.efi' --label 'Ryoku'
  ```
- **Avoid in-place Windows reinstalls on dedicated mode.** Microsoft setup does
  not officially support two ESPs on one disk and may choose the Ryoku ESP or
  reshuffle boot entries. Expect to re-register Ryoku after a reinstall.

## Broadcom Wi-Fi

**Symptom.** No Wi-Fi on a machine with a Broadcom BCM43xx card, in the live
environment or after install.

**Cause.** The in-kernel `b43`/`brcmsmac` drivers often cannot associate; these
cards need the out-of-tree `broadcom-wl` driver.

**What the installer does.** The live ISO ships `broadcom-wl`, and the backend
adds `broadcom-wl` to the `pacstrap` set for the target when a Broadcom PCI device
(vendor `14e4:`) is present.

**What the user must do.** Usually nothing. If a card still will not associate,
check `rfkill` (a hardware switch or soft block). If it associates but only
2.4 GHz networks appear, see [Only 2.4 GHz networks appear, or 5 GHz will not
connect](#only-24-ghz-networks-appear-or-5-ghz-will-not-connect).

## ASUS Aura keyboard lighting

**Symptom.** Appearance > Lighting shows only an OpenRGB motherboard or N-KEY
controller, and wallpaper palette changes do not recolour the built-in keyboard.

**Cause.** OpenRGB can identify the laptop's USB controller without exposing the
firmware-backed lighting interface consistently across keyboard generations.
`asusd` provides that controller through a stable D-Bus API instead.

**What the installer does.** The live environment detects the ASUS laptop family
or its `asus-nb-wmi` keyboard device, then installs the signed `asusctl` package
only on a match. Its udev rule starts `asusd`; Ryoku Settings prefers the native
Aura keyboard while keeping OpenRGB for other RGB hardware. Offline installs
carry the same package.

**What the user must do.** Usually nothing. An existing TLP installation
conflicts with `asusctl`, so conversion leaves TLP in place and `ryoku doctor`
reports the choice instead of removing a power stack silently.

## Xbox controller will not pair over Bluetooth

**Symptom.** An Xbox One or Series pad never finishes pairing, or it connects and
then drops within seconds, over and over. The same pad works fine on a USB cable.

**Cause.** These pads' L2CAP handshake does not survive the kernel's Enhanced
Retransmission Mode, which is on by default (`disable_ertm=N`). It looks like a
broken controller or a bad adapter, and it is the most common "my Xbox pad will
not connect on Linux" report. Separately, `xpad`, the in-kernel Xbox driver,
handles these pads only over a cable and does not speak Bluetooth at all.

**What Ryoku does.** `ryoku-desktop` ships
`/usr/lib/modprobe.d/99-ryoku-controller.conf`, which sets `disable_ertm=1` as a
`modprobe.d` drop-in, so it applies on first module load and survives a kernel
change. `xpadneo-dkms` provides the Bluetooth driver and is in `base.packages`,
shipped from `[ryoku]` so `ryoku update` keeps it current.

**What the user must do.** Nothing on a fresh install. On a machine that predates
this, `ryoku update` installs both; the ERTM change needs a reboot, or
`modprobe -r btusb && modprobe btusb` to take effect without one. A pad paired
while ERTM was on should be removed and re-paired.

## Bluetooth headphones connect, then disconnect a few seconds later

**Symptom.** A headset or earbuds pair and connect, then drop within a few
seconds, reconnect on their own, and drop again, over and over. On a cable or on
another machine the same device is fine. Often worse right after the audio has
been briefly idle.

**Cause.** `btusb`, the USB Bluetooth driver, enables USB autosuspend by default
(`enable_autosuspend=Y`). On the combo Wi-Fi+Bluetooth controllers most laptops
ship -- Intel AX2xx, Realtek RTL8761, MediaTek MT7921 -- the controller
runtime-suspends during the brief idle gaps in an A2DP/HFP stream, and the resume
races the still-open link, so the headset reads a dropped connection. It looks
like a broken headset or a flaky pairing, and it is the most common "my Bluetooth
headphones keep disconnecting on Linux" report on these chipsets.

**What Ryoku does.** `ryoku-desktop` ships
`/usr/lib/modprobe.d/99-ryoku-bt-autosuspend.conf`, which sets `options btusb
enable_autosuspend=0` as a `modprobe.d` drop-in, so it applies on the first
`btusb` load and survives a kernel change. The cost is a few milliwatts of idle
radio power a powered-on adapter carrying audio was not saving anyway.

**What the user must do.** Nothing on a fresh install. On a machine that predates
this, `ryoku update` installs it; the change needs a reboot, or
`modprobe -r btusb && modprobe btusb` to take effect without one.

## Bluetooth adapter is detected but never comes up

**Symptom.** `bluetoothctl` shows no controller, or the adapter appears and never
powers on. `dmesg` names a missing file, for example
`bluetooth hci0: BCM: Patch brcm/BCM20702A1-0a5c-21e8.hcd not found`. Common on
cheap USB dongles and on older laptops with BCM43142 or BCM20702.

**Cause.** Broadcom's USB Bluetooth parts are not self-initialising: `btbcm`
uploads a patchram blob matching the device's USB vendor:product id, and only
then does the adapter work. Arch's `linux-firmware` default set covers nearly
every adapter (`-intel`, `-realtek`, `-mediatek`, `-atheros`, `-broadcom`), but
`linux-firmware-broadcom` ships no `.hcd` files at all, so a Broadcom device has
nothing to load and reads as dead hardware.

**What Ryoku does.** `broadcom-bt-firmware` is in `base.packages`, shipped from
`[ryoku]`, and adds the 114 missing `.hcd` blobs. It shares no file with
`linux-firmware-broadcom`, so the two coexist.

**What the user must do.** Nothing on a fresh install. On an older machine,
`ryoku update` installs it; replug the adapter or reboot afterwards.

## Xbox Wireless Dongle is not supported

**Symptom.** The Xbox Wireless Dongle (the small USB adapter that pairs Xbox pads
without Bluetooth) does nothing. Pads still work over Bluetooth and over USB.

**Cause.** The dongle speaks Microsoft's proprietary Game Input Protocol over
2.4 GHz, not Bluetooth, so neither `xpad` nor `xpadneo` can drive it. Only the
`xone` driver can, and supporting it is a deliberate non-goal:

- `xone` ships `blacklist xpad` and `blacklist mt76x2u`. Blacklisting `xpad`
  takes wired Xbox pads and every XInput off-brand with it, and upstream's own
  README confirms "installing `xone` will disable the `xpad` kernel driver",
  recommending a third driver fork (`xpad-noone`) to get them back.
  Blacklisting `mt76x2u` breaks MediaTek USB Wi-Fi adapters, which have nothing
  to do with controllers.
- The dongle's firmware is extracted from Microsoft's Windows driver package and
  is Microsoft-licensed, so it cannot be redistributed in the signed `[ryoku]`
  repo the way a GPL driver can. Upstream fetches it on the user's own machine
  behind a Terms-of-Use prompt, precisely because of this.

Trading working wired pads, off-brand pads and USB Wi-Fi for one accessory is the
wrong default, so Ryoku ships neither the driver nor the firmware.

**What the user must do.** Use Bluetooth or a USB cable, both of which work out
of the box. To use the dongle anyway, install `xone-dkms` and
`xone-dongle-firmware` from the AUR and accept the blacklists; `xpad-noone`
restores the other pads.

## A game freezes when you switch workspace

**Symptom.** Switch away from the workspace a game is on and it stops dead: still
loading and it never finishes, already in a match and it locks up. Audio often
keeps playing, the mouse and keyboard do nothing, and a multiplayer session
disconnects or times out.

**Cause.** Not a crash, and not the game's fault. A Wayland client draws in
response to `wl_surface.frame` callbacks, and a surface that is not visible
receives none, so a render loop that waits on them is never scheduled again. The
game has not paused; it has stopped being asked to draw. Audio continues because
it runs on its own thread, and multiplayer drops because the netcode is usually
pumped from the same loop as rendering. It hits XWayland and Proton titles
hardest, and upstream Hyprland tracks it in several forms.

**What Ryoku does.** Game Mode adds Hyprland's `render_unfocused` rule, scoped to
fullscreen windows, so a fullscreen game keeps rendering while it is off-screen
and its loop (and its connection) stays alive. Fullscreen rather than a class list
means it covers any launcher, Steam or Lutris or Heroic or a bare binary; and
fullscreen rather than every window means the desktop behind it is not also
rendered off-screen, which would spend the GPU the game needs.

It is deliberately not on all the time. Rendering a window nobody is looking at
costs real power and heat, so it lasts exactly as long as the toggle.

**What the user must do.** Turn Game Mode on before playing (quick settings, the
Gaming tile). If a title still freezes, run it nested in gamescope, which Ryoku
already ships: put `gamescope -f -- %command%` in its Steam launch options. The
compositor then only ever sees gamescope's surface, and gamescope keeps driving
the game. Some titles also behave differently between their own fullscreen option
and the compositor's, so it is worth trying borderless or windowed with Hyprland
doing the fullscreen.

## Only 2.4 GHz networks appear, or 5 GHz will not connect

**Symptom.** The Wi-Fi list shows only 2.4 GHz networks, or a known 5 GHz (or
6 GHz) network never appears or refuses to connect, even though the card
supports those bands.

**Cause.** Four things gate the higher bands, and any one of them alone produces
the symptom:

- *No regulatory domain.* The kernel gates which channels a radio may use on the
  country's regulatory domain. With none set it stays on the worldwide default
  `00`, which disables or marks no-IR most 5 GHz channels, so those networks
  never appear.
- *No band chosen.* A dual-band network advertises one SSID on both bands.
  Joining it by name lands on whichever radio answers first, almost always the
  2.4 GHz one, with no way to ask for the 5 GHz radio.
- *iwd's band ranking.* iwd is the default NetworkManager backend; left
  unconfigured it neither prefers 5 GHz nor takes a country hint, so it can
  settle on the slower band even when both are in range.
- *WPA3-SAE.* Many 5 GHz and 6 GHz networks are WPA3-only. A client that offers
  only WPA2 key management cannot finish the handshake, so the join fails.

**What the installer does.** It sets the regulatory domain from geolocation,
falling back to the country in the system locale, and writes iwd's `main.conf`
so iwd prefers 5 GHz and carries the country hint. `ryoku doctor` re-checks the
domain on every update and heals a box that has drifted back to `00`.

**What the user must do.**

- **Fix the regulatory domain.** If only 2.4 GHz networks appear, run
  `ryoku doctor`; it sets the domain from your locale where it can. If it cannot
  infer your country, set it by hand with `ryoku-wifi-regdom set <CC>` (for
  example `ryoku-wifi-regdom set US`); `ryoku-wifi-regdom status` prints the
  domain in force.
- **Pick the band.** The shell's Wi-Fi list and the Hub's Connections page now
  show each band of a dual-band network as its own entry. Pick the 5 GHz one and
  the saved profile is locked to that band, so it never drops back to 2.4 GHz.
- **WPA3 networks.** The shell now joins WPA3-SAE networks with the right key
  management. If one still refuses, open Ryoku Settings -> Connections and switch
  the Wi-Fi backend from iwd to wpa_supplicant, which handles a few WPA3 edge
  cases iwd does not.

## RTC clock skew vs pacman signatures

**Symptom.** The install fails with TLS "certificate is not yet valid" / "expired"
errors, or pacman signature-verification failures. Common on a laptop with a dead
CMOS battery.

**Cause.** A wildly wrong system clock makes TLS reject every certificate and
breaks pacman's signature checks.

**What the installer does.** When a mirror probe fails, `network.sh`
(`ryoku_fix_clock_skew`) reads the mirror's own clock over an unverified
connection (so the skewed cert cannot block it) and, if the system clock is off
by more than a day, sets it from the HTTP `Date` header and re-probes once. This
is best-effort and never aborts the install on its own; the live ISO also runs
`systemd-timesyncd`.

**What the user must do.** If it still fails (no network to read a `Date` header
from), set the correct date and time in firmware setup, or run
`timedatectl set-time` on the live shell, then retry. Replace the CMOS battery to
make the fix stick across power-off.

## NVRAM-readonly firmware

**Symptom.** The install completes but the machine boots straight into Windows or
the firmware boot menu; the Ryoku entry never appears or does not persist.

**Cause.** Some firmware ignores or will not persist `efibootmgr` NVRAM writes.

**What the installer does.** `bootloader.sh` writes the Limine binary to a
tool-managed path plus the removable-media fallback `EFI/BOOT/BOOTX64.EFI`, so
the disk is bootable even with no working NVRAM entry. Which ESP and path depends
on the strategy: whole-disk uses `EFI/limine/limine_x64.efi` on the Ryoku ESP;
alongside uses `EFI/ryoku/BOOTX64.EFI` on the shared Windows ESP. The
`efibootmgr` registration is best-effort on top of the fallback.

**What the user must do.** In firmware setup, add a boot entry pointing at the
Limine loader on that ESP (`\EFI\limine\limine_x64.efi` for a whole-disk install,
`\EFI\ryoku\BOOTX64.EFI` for alongside), or move the Ryoku entry to the top of
the boot order. On firmware that only boots the removable path, the fallback
loader already covers it; just select the disk.

## Ventoy and other loop-mounted media

**Symptom.** "No space left" or squashfs failures during live boot when booted
from Ventoy.

**Cause.** Ventoy loop-mounts the ISO and injects its own boot shim, which breaks
archiso's squashfs discovery (the UUID search that finds the airootfs and the
`cow_spacesize` overlay it sets up).

**What the installer does.** Nothing can repair a Ventoy boot from inside the
image. The ISO does raise the copy-on-write overlay to 1 GiB to reduce the
"no space" class, but Ventoy is unsupported.

**What the user must do.** Write the ISO raw with `dd` (or Rufus in DD mode on
Windows) to a dedicated stick, and verify it against the `SHA256SUMS` the build
writes next to the ISO.

```
dd if=ryoku-<date>-x86_64.iso of=/dev/sdX bs=4M status=progress oflag=sync
```

## Slow or flaky USB media

**Symptom.** The install is painfully slow, or the stick is flaky and the live
system falls over when it is jostled or pulled mid-install.

**Cause.** The live system reads the squashfs from the stick throughout the
install, so a slow or unreliable stick drags on or dies.

**What the installer does.** The ISO ships a "copy to RAM (copytoram)" boot entry
that loads the whole image into RAM before boot, so the install runs entirely
from memory and no longer depends on the stick.

**What the user must do.** Pick the **copy to RAM (copytoram)** boot entry. It is
slow to start (it reads the whole image once) but resilient afterwards, and
tolerates a removed or flaky drive. It needs enough RAM to hold the image.
